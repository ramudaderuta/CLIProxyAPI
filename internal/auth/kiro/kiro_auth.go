// Package kiro provides authentication and token management functionality
// for Kiro AI services. It handles OAuth authentication flows, including
// token storage, refresh, and API client configuration for Kiro services.
package kiro

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/auth"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	log "github.com/sirupsen/logrus"
)

const (
	// Kiro OAuth endpoints
	kiroRefreshURL     = "https://prod.%s.auth.desktop.kiro.dev/refreshToken"
	kiroRefreshIDCURL  = "https://oidc.%s.amazonaws.com/token"
	kiroBaseURL        = "https://codewhisperer.%s.amazonaws.com/generateAssistantResponse"
	kiroAmazonQURL     = "https://codewhisperer.%s.amazonaws.com/SendMessageStreaming"

	// Authentication methods
	authMethodSocial = "social"

	// Default region
	defaultRegion = "us-east-1"

	// User agent strings
	kiroUserAgent = "KiroIDE"
)

// KiroAuth provides methods for handling Kiro authentication flows.
// It encapsulates the logic for managing authentication tokens,
// refreshing tokens, and configuring HTTP clients for Kiro API calls.
type KiroAuth struct{}

// NewKiroAuth creates a new instance of KiroAuth.
func NewKiroAuth() *KiroAuth {
	return &KiroAuth{}
}

// GetAuthenticatedClient configures and returns an HTTP client ready for making authenticated API calls.
// It manages token loading, refresh, and proxy configuration for Kiro services.
//
// Parameters:
//   - ctx: The context for the HTTP client
//   - ts: The Kiro token storage containing authentication tokens
//   - cfg: The configuration containing proxy settings
//
// Returns:
//   - *http.Client: An HTTP client configured with authentication
//   - error: An error if the client configuration fails, nil otherwise
func (k *KiroAuth) GetAuthenticatedClient(ctx context.Context, ts *KiroTokenStorage, cfg *config.Config) (*http.Client, error) {
	// Check if token needs refresh
	if ts.IsExpired() {
		log.Info("[Kiro Auth] Token is expired or near expiry, refreshing...")
		if err := k.refreshToken(ts, cfg); err != nil {
			return nil, fmt.Errorf("failed to refresh token: %w", err)
		}
	}

	// Create HTTP client with transport configuration
	client := &http.Client{
		Timeout: 120 * time.Second, // 2 minutes timeout
	}

	// Configure proxy if specified
	if cfg.ProxyURL != "" {
		if proxyURL, err := url.Parse(cfg.ProxyURL); err == nil {
			client.Transport = &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
			}
		} else {
			log.Warnf("[Kiro Auth] Invalid proxy URL: %s", cfg.ProxyURL)
		}
	}

	return client, nil
}

// refreshToken refreshes the access token using the refresh token.
// It handles both social and IDC authentication methods.
//
// Parameters:
//   - ts: The Kiro token storage containing the refresh token
//   - cfg: The configuration containing settings
//
// Returns:
//   - error: An error if the refresh fails, nil otherwise
func (k *KiroAuth) refreshToken(ts *KiroTokenStorage, cfg *config.Config) error {
	if ts.RefreshToken == "" {
		return fmt.Errorf("no refresh token available")
	}

	// Determine region from profile ARN or use default
	region := k.extractRegionFromARN(ts.ProfileArn)
	if region == "" {
		region = defaultRegion
	}

	// Prepare refresh request
	var refreshURL string
	var requestBody map[string]interface{}

	if ts.AuthMethod == authMethodSocial {
		refreshURL = fmt.Sprintf(kiroRefreshURL, region)
		requestBody = map[string]interface{}{
			"refreshToken": ts.RefreshToken,
		}
	} else {
		refreshURL = fmt.Sprintf(kiroRefreshIDCURL, region)
		requestBody = map[string]interface{}{
			"refreshToken": ts.RefreshToken,
			"grantType":    "refresh_token",
		}
	}

	// Marshal request body
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal refresh request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(context.Background(), "POST", refreshURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create refresh request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", kiroUserAgent)

	// Configure proxy for refresh request if needed
	client := &http.Client{Timeout: 30 * time.Second}
	if cfg.ProxyURL != "" {
		if proxyURL, err := url.Parse(cfg.ProxyURL); err == nil {
			client.Transport = &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
			}
		}
	}

	// Execute refresh request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute refresh request: %w", err)
	}
	defer func() {
		if errClose := resp.Body.Close(); errClose != nil {
			log.Errorf("[Kiro Auth] Failed to close response body: %v", errClose)
		}
	}()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("token refresh failed with status %d", resp.StatusCode)
	}

	// Parse refresh response
	var refreshResponse struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		ProfileArn   string `json:"profileArn"`
		ExpiresIn    int64  `json:"expiresIn"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&refreshResponse); err != nil {
		return fmt.Errorf("failed to decode refresh response: %w", err)
	}

	if refreshResponse.AccessToken == "" {
		return fmt.Errorf("invalid refresh response: missing accessToken")
	}

	// Update token storage with new values
	ts.AccessToken = refreshResponse.AccessToken
	if refreshResponse.RefreshToken != "" {
		ts.RefreshToken = refreshResponse.RefreshToken
	}
	if refreshResponse.ProfileArn != "" {
		ts.ProfileArn = refreshResponse.ProfileArn
	}

	// Calculate new expiration time
	expiresIn := refreshResponse.ExpiresIn
	if expiresIn == 0 {
		expiresIn = 3600 // Default to 1 hour if not specified
	}
	ts.ExpiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)

	log.Info("[Kiro Auth] Token refreshed successfully")
	return nil
}

// LoadTokenFromFile loads Kiro authentication from a file and returns a TokenStorage implementation.
// This method provides compatibility with the existing auth system.
//
// Parameters:
//   - authFilePath: The path to the Kiro authentication token file
//
// Returns:
//   - auth.TokenStorage: An implementation of the TokenStorage interface
//   - error: An error if loading fails, nil otherwise
func LoadKiroTokenFromFile(authFilePath string) (auth.TokenStorage, error) {
	token, err := LoadTokenFromFile(authFilePath)
	if err != nil {
		return nil, err
	}
	return token, nil
}

// extractRegionFromARN extracts the AWS region from a profile ARN.
//
// Parameters:
//   - arn: The AWS profile ARN
//
// Returns:
//   - string: The extracted region, or empty string if not found
func (k *KiroAuth) extractRegionFromARN(arn string) string {
	// ARN format: arn:aws:codewhisperer:us-east-1:123456789012:profile/PROFILE_NAME
	// We need to extract the region part (us-east-1 in this example)
	if len(arn) < 30 { // Minimum length for a valid ARN with region
		return ""
	}

	// Split by colon and look for the region part
	parts := bytes.Split([]byte(arn), []byte(":"))
	if len(parts) < 5 {
		return ""
	}

	// Region is the 4th part (index 3) in a standard ARN
	if len(parts) > 3 {
		region := string(parts[3])
		if len(region) >= 9 && region[:9] == "us-east-1" {
			return region
		}
		// Check for other AWS region patterns
		if len(region) >= 9 && (region[:2] == "us" || region[:2] == "eu" || region[:2] == "ap") {
			return region
		}
	}

	return ""
}

// ValidateToken checks if the current token is valid and not expired.
//
// Parameters:
//   - ts: The Kiro token storage to validate
//
// Returns:
//   - bool: True if the token is valid, false otherwise
func (k *KiroAuth) ValidateToken(ts *KiroTokenStorage) bool {
	if ts == nil || ts.AccessToken == "" {
		return false
	}

	// Check if token is expired
	if ts.IsExpired() {
		return false
	}

	// Additional validation can be added here
	return true
}