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

// OAuth endpoints for Kiro device code flow via AWS SSO OIDC
const (
	// RegisterClientEndpoint is the AWS SSO OIDC endpoint for registering public clients
	RegisterClientEndpoint = "https://oidc.us-east-1.amazonaws.com/client/register"

	// DeviceAuthEndpoint is the AWS SSO OIDC endpoint for requesting device codes
	// Kiro uses AWS IAM Identity Center (SSO) for authentication
	DeviceAuthEndpoint = "https://oidc.us-east-1.amazonaws.com/device_authorization"

	// TokenEndpoint is the AWS SSO OIDC endpoint for exchanging device codes for tokens or refreshing tokens
	TokenEndpoint = "https://oidc.us-east-1.amazonaws.com/token"

	// DefaultStartURL is the default AWS access portal URL for Builder ID
	DefaultStartURL = "https://view.awsapps.com/start"

	// DefaultPollInterval is the default interval between polling attempts (in seconds)
	DefaultPollInterval = 5

	// MaxPollAttempts is the maximum number of polling attempts before giving up
	MaxPollAttempts = 120 // 10 minutes with 5-second interval

	// ClientCacheDuration is how long we consider a registered client valid (80 days, before 90-day expiry)
	ClientCacheDuration = 80 * 24 * time.Hour
)

// RegisteredClient represents a registered OIDC client.
type RegisteredClient struct {
	ClientID              string    `json:"clientId"`
	ClientSecret          string    `json:"clientSecret"`
	ClientIDIssuedAt      int64     `json:"clientIdIssuedAt"`
	ClientSecretExpiresAt int64     `json:"clientSecretExpiresAt"`
	AuthorizationEndpoint string    `json:"authorizationEndpoint,omitempty"`
	TokenEndpoint         string    `json:"tokenEndpoint,omitempty"`
	RegisteredAt          time.Time `json:"registeredAt"`
}

// IsExpired checks if the client registration has expired or will expire soon.
func (c *RegisteredClient) IsExpired() bool {
	if c.ClientSecretExpiresAt == 0 {
		// If no expiration set, check against our cache duration
		return time.Since(c.RegisteredAt) > ClientCacheDuration
	}
	// Check if expires within next 10 days
	expiryTime := time.Unix(c.ClientSecretExpiresAt, 0)
	return time.Until(expiryTime) < 10*24*time.Hour
}

// DeviceCodeFlow handles the OAuth device code flow for Kiro authentication.
type DeviceCodeFlow struct {
	client       *http.Client
	clientID     string
	clientSecret string
	startURL     string
	scopes       []string
	pollInterval time.Duration
}

// DeviceCodeResponse represents the response from the device authorization endpoint.
type DeviceCodeResponse struct {
	DeviceCode              string `json:"deviceCode"`
	UserCode                string `json:"userCode"`
	VerificationURI         string `json:"verificationUri"`
	VerificationURIComplete string `json:"verificationUriComplete"`
	ExpiresIn               int    `json:"expiresIn"`
	Interval                int    `json:"interval"`
}

// TokenResponse represents the response from the token endpoint.
// AWS SSO OIDC uses camelCase for field names, not snake_case.
type TokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	TokenType    string `json:"tokenType"`
	ExpiresIn    int    `json:"expiresIn"`
	Scope        string `json:"scope,omitempty"`
	Error        string `json:"error,omitempty"`
	ErrorDesc    string `json:"error_description,omitempty"`
}

// maskToken masks a token for safe logging.
func maskToken(token string) string {
	if token == "" {
		return "<empty>"
	}
	if len(token) < 8 {
		return "***"
	}
	return token[:4] + "..." + token[len(token)-4:]
}

// NewDeviceCodeFlow creates a new device code flow handler.
//
// Parameters:
//   - cfg: Configuration for proxy settings
//   - client: Registered OIDC client information
//
// Returns:
//   - *DeviceCodeFlow: The device code flow handler
func NewDeviceCodeFlow(cfg *config.Config, client *RegisteredClient) *DeviceCodeFlow {
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Configure proxy if set in config
	if cfg != nil && cfg.ProxyURL != "" {
		// Proxy configuration would be set here if needed
		log.Debugf("Using proxy: %s", cfg.ProxyURL)
	}

	return &DeviceCodeFlow{
		client:       httpClient,
		clientID:     client.ClientID,
		clientSecret: client.ClientSecret,
		startURL:     DefaultStartURL,
		scopes:       []string{}, // Scopes are handled by StartDeviceAuthorization
		pollInterval: DefaultPollInterval * time.Second,
	}
}

// RegisterClient registers a new public OIDC client with AWS SSO.
//
// Parameters:
//   - ctx: Context for the request
//   - httpClient: HTTP client to use for the request
//
// Returns:
//   - *RegisteredClient: The registered client information
//   - error: An error if registration fails, nil otherwise
func RegisterClient(ctx context.Context, httpClient *http.Client) (*RegisteredClient, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	// Build request payload
	// issuerUrl is required for proper consent screen display
	payload := map[string]interface{}{
		"clientName": "Kiro CLI",
		"clientType": "public",
		"issuerUrl":  "https://codewhisperer.aws",
		"grantTypes": []string{
			"urn:ietf:params:oauth:grant-type:device_code",
			"refresh_token",
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, NewAuthError("RegisterClient", err, "failed to marshal request")
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", RegisterClientEndpoint, bytes.NewReader(jsonData))
	if err != nil {
		return nil, NewAuthError("RegisterClient", err, "failed to create request")
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Kiro-CLI")
	req.Header.Set("x-amz-user-agent", "Kiro-CLI")

	// Execute request
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, NewAuthError("RegisterClient", err, "failed to send request")
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, NewAuthError("RegisterClient", err, "failed to read response")
	}

	// Check status code
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, NewAuthError("RegisterClient", fmt.Errorf("status %d: %s", resp.StatusCode, string(body)), "request failed")
	}

	// Parse response
	var clientResp RegisteredClient
	if err = json.Unmarshal(body, &clientResp); err != nil {
		return nil, NewAuthError("RegisterClient", err, "failed to parse response")
	}

	clientResp.RegisteredAt = time.Now()

	log.Infof("Successfully registered OIDC client: %s", clientResp.ClientID)
	return &clientResp, nil
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
	// Build request payload for AWS SSO OIDC StartDeviceAuthorization
	payload := map[string]interface{}{
		"clientId":     f.clientID,
		"clientSecret": f.clientSecret,
		"startUrl":     f.startURL,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, NewAuthError("StartDeviceFlow", err, "failed to marshal request")
	}

	log.Debugf("StartDeviceFlow: Endpoint=%s, RequestBody=%s", DeviceAuthEndpoint, string(jsonData))

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", DeviceAuthEndpoint, bytes.NewReader(jsonData))
	if err != nil {
		return nil, NewAuthError("StartDeviceFlow", err, "failed to create request")
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Kiro-CLI")
	req.Header.Set("x-amz-user-agent", "Kiro-CLI")

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
	req.Header.Set("User-Agent", "Kiro-CLI")
	req.Header.Set("x-amz-user-agent", "Kiro-CLI")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, NewAuthError("requestToken", err, "failed to send request")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, NewAuthError("requestToken", err, "failed to read response")
	}

	// Log response for debugging (only at DEBUG level)
	log.Debugf("requestToken: HTTP Status=%d, Response body: %s", resp.StatusCode, string(body))

	var tokenResp TokenResponse
	if err = json.Unmarshal(body, &tokenResp); err != nil {
		log.Errorf("requestToken: Failed to parse JSON: %v", err)
		return nil, NewAuthError("requestToken", err, "failed to parse response")
	}

	log.Debugf("requestToken: Parsed - AccessToken=%s, RefreshToken=%s, TokenType=%s, ExpiresIn=%d, Error=%s",
		maskToken(tokenResp.AccessToken), maskToken(tokenResp.RefreshToken),
		tokenResp.TokenType, tokenResp.ExpiresIn, tokenResp.Error)

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
	// Note: refreshToken may be null/empty on initial device code authorization
	if tokenResp.AccessToken == "" {
		return nil, NewAuthError("requestToken", fmt.Errorf("missing access token in response"), "invalid response")
	}

	// Validate ExpiresIn value to catch potential API errors
	if tokenResp.ExpiresIn <= 0 {
		return nil, NewAuthError("requestToken",
			fmt.Errorf("invalid expiresIn: %d", tokenResp.ExpiresIn),
			"invalid expiration time from server")
	}

	if tokenResp.ExpiresIn < 60 {
		log.Warnf("Token expiration time is very short: %d seconds", tokenResp.ExpiresIn)
	}

	// Calculate expiration time using UTC for consistency
	// Subtract a small amount (2 seconds) to account for network latency
	// between receiving the response and saving the token
	now := time.Now().UTC()
	expiresAt := now.Add(time.Duration(tokenResp.ExpiresIn) * time.Second).Add(-2 * time.Second)

	log.Infof("Token expiration calculated: now=%s, expiresIn=%ds, expiresAt=%s",
		now.Format(time.RFC3339),
		tokenResp.ExpiresIn,
		expiresAt.Format(time.RFC3339))

	// Build token storage
	storage := &KiroTokenStorage{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken, // May be empty
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
	req.Header.Set("User-Agent", "Kiro-CLI")
	req.Header.Set("x-amz-user-agent", "Kiro-CLI")

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

	// Validate ExpiresIn value
	if tokenResp.ExpiresIn <= 0 {
		return nil, NewAuthError("RefreshToken",
			fmt.Errorf("invalid expiresIn: %d", tokenResp.ExpiresIn),
			"invalid expiration time from server")
	}

	if tokenResp.ExpiresIn < 60 {
		log.Warnf("Refreshed token expiration time is very short: %d seconds", tokenResp.ExpiresIn)
	}

	// Calculate expiration time using UTC for consistency
	// Subtract 2 seconds to account for network latency
	now := time.Now().UTC()
	expiresAt := now.Add(time.Duration(tokenResp.ExpiresIn) * time.Second).Add(-2 * time.Second)

	log.Infof("Refreshed token expiration calculated: now=%s, expiresIn=%ds, expiresAt=%s",
		now.Format(time.RFC3339),
		tokenResp.ExpiresIn,
		expiresAt.Format(time.RFC3339))

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
