package kiro

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	log "github.com/sirupsen/logrus"
)

// OAuth endpoints for Kiro device code flow
const (
	// DeviceAuthEndpoint is the endpoint for requesting device codes
	DeviceAuthEndpoint = "https://codewhisperer.us-east-1.amazonaws.com/device_authorization"

	// TokenEndpoint is the endpoint for exchanging device codes for tokens or refreshing tokens
	TokenEndpoint = "https://codewhisperer.us-east-1.amazonaws.com/token"

	// DefaultPollInterval is the default interval between polling attempts (in seconds)
	DefaultPollInterval = 5

	// MaxPollAttempts is the maximum number of polling attempts before giving up
	MaxPollAttempts = 120 // 10 minutes with 5-second interval
)

// DeviceCodeFlow handles the OAuth device code flow for Kiro authentication.
type DeviceCodeFlow struct {
	client       *http.Client
	clientID     string
	scopes       []string
	pollInterval time.Duration
}

// DeviceCodeResponse represents the response from the device authorization endpoint.
type DeviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// TokenResponse represents the response from the token endpoint.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	Error        string `json:"error,omitempty"`
	ErrorDesc    string `json:"error_description,omitempty"`
}

// NewDeviceCodeFlow creates a new device code flow handler.
//
// Parameters:
//   - cfg: Configuration for proxy settings
//   - clientID: OAuth client ID for Kiro
//   - scopes: OAuth scopes to request
//
// Returns:
//   - *DeviceCodeFlow: The device code flow handler
func NewDeviceCodeFlow(cfg *config.Config, clientID string, scopes []string) *DeviceCodeFlow {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Configure proxy if set in config
	if cfg != nil && cfg.ProxyURL != "" {
		// Proxy configuration would be set here if needed
		log.Debugf("Using proxy: %s", cfg.ProxyURL)
	}

	return &DeviceCodeFlow{
		client:       client,
		clientID:     clientID,
		scopes:       scopes,
		pollInterval: DefaultPollInterval * time.Second,
	}
}

// StartDeviceFlow initiates the device code flow by requesting a device code.
//
// Parameters:
//   - ctx: Context for the request
//
// Returns:
//   - *DeviceCodeResponse: The device code response with user code and verification URI
//   - error: An error if the request fails, nil otherwise
func (f *DeviceCodeFlow) StartDeviceFlow(ctx context.Context) (*DeviceCodeResponse, error) {
	// Build request payload
	payload := map[string]interface{}{
		"client_id": f.clientID,
		"scope":     "https://cloudcode.aws/builderid/authorization",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, NewAuthError("StartDeviceFlow", err, "failed to marshal request")
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", DeviceAuthEndpoint, bytes.NewReader(jsonData))
	if err != nil {
		return nil, NewAuthError("StartDeviceFlow", err, "failed to create request")
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Execute request
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, NewAuthError("StartDeviceFlow", err, "failed to send request")
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, NewAuthError("StartDeviceFlow", err, "failed to read response")
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, NewAuthError("StartDeviceFlow", fmt.Errorf("status %d: %s", resp.StatusCode, string(body)), "request failed")
	}

	// Parse response
	var deviceResp DeviceCodeResponse
	if err = json.Unmarshal(body, &deviceResp); err != nil {
		return nil, NewAuthError("StartDeviceFlow", err, "failed to parse response")
	}

	// Set poll interval from response if provided
	if deviceResp.Interval > 0 {
		f.pollInterval = time.Duration(deviceResp.Interval) * time.Second
	}

	log.Infof("Device code flow started: user code=%s, expires in %d seconds", deviceResp.UserCode, deviceResp.ExpiresIn)
	return &deviceResp, nil
}

// PollForToken polls the token endpoint until the user authorizes the device.
// It implements exponential backoff for slow_down responses.
//
// Parameters:
//   - ctx: Context for the request
//   - deviceCode: The device code from StartDeviceFlow
//
// Returns:
//   - *KiroTokenStorage: The token storage with access and refresh tokens
//   - error: An error if polling fails or times out, nil otherwise
func (f *DeviceCodeFlow) PollForToken(ctx context.Context, deviceCode string) (*KiroTokenStorage, error) {
	attempt := 0
	pollInterval := f.pollInterval
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, NewAuthError("PollForToken", ctx.Err(), "context cancelled")
		case <-ticker.C:
			attempt++
			if attempt > MaxPollAttempts {
				return nil, NewAuthError("PollForToken", ErrDeviceCodeExpired, "exceeded maximum poll attempts")
			}

			log.Debugf("Polling for token (attempt %d/%d)...", attempt, MaxPollAttempts)

			// Attempt to get token
			token, err := f.requestToken(ctx, deviceCode)
			if err != nil {
				if errors.Is(err, ErrAuthorizationPending) {
					// Continue polling
					log.Debug("Authorization pending, continuing to poll...")
					continue
				}
				if errors.Is(err, ErrSlowDown) {
					// Increase poll interval
					pollInterval = pollInterval + (5 * time.Second)
					ticker.Reset(pollInterval)
					log.Debugf("Slow down requested, increasing poll interval to %v", pollInterval)
					continue
				}
				// Other errors are fatal
				return nil, err
			}

			// Success!
			return token, nil
		}
	}
}

// requestToken attempts to exchange the device code for tokens.
func (f *DeviceCodeFlow) requestToken(ctx context.Context, deviceCode string) (*KiroTokenStorage, error) {
	payload := map[string]interface{}{
		"client_id":   f.clientID,
		"device_code": deviceCode,
		"grant_type":  "urn:ietf:params:oauth:grant-type:device_code",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, NewAuthError("requestToken", err, "failed to marshal request")
	}

	req, err := http.NewRequestWithContext(ctx, "POST", TokenEndpoint, bytes.NewReader(jsonData))
	if err != nil {
		return nil, NewAuthError("requestToken", err, "failed to create request")
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, NewAuthError("requestToken", err, "failed to send request")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, NewAuthError("requestToken", err, "failed to read response")
	}

	var tokenResp TokenResponse
	if err = json.Unmarshal(body, &tokenResp); err != nil {
		return nil, NewAuthError("requestToken", err, "failed to parse response")
	}

	// Check for OAuth errors
	if tokenResp.Error != "" {
		switch tokenResp.Error {
		case "authorization_pending":
			return nil, ErrAuthorizationPending
		case "slow_down":
			return nil, ErrSlowDown
		case "expired_token":
			return nil, ErrDeviceCodeExpired
		case "access_denied":
			return nil, ErrAccessDenied
		default:
			return nil, NewAuthError("requestToken", fmt.Errorf("%s: %s", tokenResp.Error, tokenResp.ErrorDesc), "OAuth error")
		}
	}

	// Validate token response
	if tokenResp.AccessToken == "" || tokenResp.RefreshToken == "" {
		return nil, NewAuthError("requestToken", fmt.Errorf("missing tokens in response"), "invalid response")
	}

	// Build token storage
	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	storage := &KiroTokenStorage{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    expiresAt,
		AuthMethod:   "IdC", // Device code flow uses IdC method
		Provider:     "BuilderId",
	}

	log.Info("Successfully obtained access token via device code flow")
	return storage, nil
}

// RefreshToken refreshes an access token using a refresh token.
//
// Parameters:
//   - ctx: Context for the request
//   - refreshToken: The refresh token
//
// Returns:
//   - *KiroTokenStorage: Updated token storage with new access token
//   - error: An error if refresh fails, nil otherwise
func (f *DeviceCodeFlow) RefreshToken(ctx context.Context, refreshToken string) (*KiroTokenStorage, error) {
	payload := map[string]interface{}{
		"client_id":     f.clientID,
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, NewAuthError("RefreshToken", err, "failed to marshal request")
	}

	req, err := http.NewRequestWithContext(ctx, "POST", TokenEndpoint, bytes.NewReader(jsonData))
	if err != nil {
		return nil, NewAuthError("RefreshToken", err, "failed to create request")
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, NewAuthError("RefreshToken", err, "failed to send request")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, NewAuthError("RefreshToken", err, "failed to read response")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, NewAuthError("RefreshToken", fmt.Errorf("status %d: %s", resp.StatusCode, string(body)), "refresh failed")
	}

	var tokenResp TokenResponse
	if err = json.Unmarshal(body, &tokenResp); err != nil {
		return nil, NewAuthError("RefreshToken", err, "failed to parse response")
	}

	if tokenResp.Error != "" {
		return nil, NewAuthError("RefreshToken", fmt.Errorf("%s: %s", tokenResp.Error, tokenResp.ErrorDesc), "OAuth error")
	}

	if tokenResp.AccessToken == "" {
		return nil, NewAuthError("RefreshToken", ErrInvalidRefreshToken, "no access token in response")
	}

	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	storage := &KiroTokenStorage{
		AccessToken: tokenResp.AccessToken,
		ExpiresAt:   expiresAt,
	}

	// Preserve refresh token if not provided in response
	if tokenResp.RefreshToken != "" {
		storage.RefreshToken = tokenResp.RefreshToken
	} else {
		storage.RefreshToken = refreshToken
	}

	log.Info("Successfully refreshed access token")
	return storage, nil
}
