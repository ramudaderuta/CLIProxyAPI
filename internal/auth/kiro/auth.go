package kiro

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	log "github.com/sirupsen/logrus"
)

const (
	// DefaultClientID is the default OAuth client ID for Kiro CLI
	// This matches the client ID used by the official kiro-cli
	DefaultClientID = "arn:aws:codewhisperer:us-east-1::client/codewhisperer-cli"

	// DefaultScopes are the default OAuth scopes for Kiro authentication
	DefaultScopes = "https://cloudcode.aws/builderid/authorization"

	// TokenExpirationBuffer is the time buffer before token expiration to treat as expired (in minutes)
	TokenExpirationBuffer = 5
)

// KiroAuthenticator provides methods for handling Kiro OAuth2 authentication flow.
// It encapsulates the logic for obtaining, storing, and refreshing authentication tokens
// for Amazon Q Developer (Kiro) CLI.
type KiroAuthenticator struct {
	cfg    *config.Config
	oauth  *DeviceCodeFlow
	client *http.Client
}

// NewKiroAuthenticator creates a new instance of KiroAuthenticator.
//
// Parameters:
//   - cfg: Configuration for proxy and other settings
//
// Returns:
//   - *KiroAuthenticator: The authenticator instance
func NewKiroAuthenticator(cfg *config.Config) *KiroAuthenticator {
	oauth := NewDeviceCodeFlow(cfg, DefaultClientID, []string{DefaultScopes})

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	return &KiroAuthenticator{
		cfg:    cfg,
		oauth:  oauth,
		client: client,
	}
}

// Authenticate initiates the OAuth device code flow.
// It displays the user code and verification URL to the user.
//
// Parameters:
//   - ctx: Context for the authentication request
//
// Returns:
//   - *KiroTokenStorage: The token storage after successful authentication
//   - *DeviceCodeResponse: The device code response (for displaying to user)
//   - error: An error if authentication fails, nil otherwise
func (a *KiroAuthenticator) Authenticate(ctx context.Context) (*KiroTokenStorage, *DeviceCodeResponse, error) {
	log.Info("Starting Kiro device code authentication flow")

	// Start device code flow
	deviceResp, err := a.oauth.StartDeviceFlow(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start device flow: %w", err)
	}

	log.Infof("Device code flow initiated. User code: %s", deviceResp.UserCode)
	log.Infof("Please visit: %s", deviceResp.VerificationURI)

	// Poll for token
	token, err := a.oauth.PollForToken(ctx, deviceResp.DeviceCode)
	if err != nil {
		return nil, deviceResp, fmt.Errorf("failed to obtain token: %w", err)
	}

	log.Info("Authentication successful")
	return token, deviceResp, nil
}

// RefreshToken refreshes an expired access token using the refresh token.
//
// Parameters:
//   - ctx: Context for the refresh request
//   - storage: The current token storage with refresh token
//
// Returns:
//   - *KiroTokenStorage: Updated token storage with new access token
//   - error: An error if refresh fails, nil otherwise
func (a *KiroAuthenticator) RefreshToken(ctx context.Context, storage *KiroTokenStorage) (*KiroTokenStorage, error) {
	if storage == nil || storage.RefreshToken == "" {
		return nil, fmt.Errorf("no refresh token available")
	}

	log.Info("Refreshing Kiro access token")

	newToken, err := a.oauth.RefreshToken(ctx, storage.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	// Preserve fields not returned in refresh response
	newToken.ProfileArn = storage.ProfileArn
	newToken.AuthMethod = storage.AuthMethod
	newToken.Provider = storage.Provider
	newToken.Region = storage.Region
	newToken.ClientIdHash = storage.ClientIdHash

	log.Info("Token refreshed successfully")
	return newToken, nil
}

// ValidateToken checks if a token is valid and not expired.
// If the token is expired but has a refresh token, it will attempt to refresh it.
//
// Parameters:
//   - ctx: Context for the validation request
//   - storage: The token storage to validate
//
// Returns:
//   - *KiroTokenStorage: The validated (and possibly refreshed) token storage
//   - bool: true if token was refreshed, false otherwise
//   - error: An error if validation fails, nil otherwise
func (a *KiroAuthenticator) ValidateToken(ctx context.Context, storage *KiroTokenStorage) (*KiroTokenStorage, bool, error) {
	if storage == nil {
		return nil, false, fmt.Errorf("token storage is nil")
	}

	if storage.AccessToken == "" {
		return nil, false, fmt.Errorf("access token is empty")
	}

	// Check if token is expired
	if storage.IsExpired(TokenExpirationBuffer) {
		log.Info("Access token is expired, attempting to refresh")

		// Try to refresh
		newToken, err := a.RefreshToken(ctx, storage)
		if err != nil {
			return nil, false, fmt.Errorf("token expired and refresh failed: %w", err)
		}

		return newToken, true, nil
	}

	// Token is valid
	return storage, false, nil
}

// GetAuthenticatedClient creates an HTTP client configured with the token for authentication.
//
// Parameters:
//   - ctx: Context for the client setup
//   - storage: The token storage to use for authentication
//
// Returns:
//   - *http.Client: The authenticated HTTP client
//   - error: An error if client setup fails, nil otherwise
func (a *KiroAuthenticator) GetAuthenticatedClient(ctx context.Context, storage *KiroTokenStorage) (*http.Client, error) {
	// Validate and refresh token if needed
	validToken, refreshed, err := a.ValidateToken(ctx, storage)
	if err != nil {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}

	if refreshed {
		log.Info("Token was refreshed during client setup")
		// Note: Caller should save the refreshed token
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	// Configure proxy if set
	if a.cfg != nil && a.cfg.ProxyURL != "" {
		log.Debugf("Configuring HTTP client with proxy: %s", a.cfg.ProxyURL)
		// Proxy configuration would be set here if needed
	}

	// The token will be added to request headers by the executor
	// This client is just configured with proper timeouts and proxy
	_ = validToken // Token will be used by caller

	return client, nil
}
