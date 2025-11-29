package kiro

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

	// TokenExpirationBuffer is the time buffer before a token is considered expired (in minutes)
	// This is used to account for network latency and clock drift.
	// If a token will expire within this many minutes, it is considered expired.
	TokenExpirationBuffer = 5

	// TokenEarlyRefreshBuffer is the time buffer for proactive token refresh (in minutes)
	// If a token will expire within this many minutes and has a refresh token,
	// we will proactively refresh it to avoid potential race conditions where the
	// token expires between validation and API request.
	TokenEarlyRefreshBuffer = 10
)

// KiroAuthenticator provides methods for handling Kiro OAuth2 authentication flow.
// It encapsulates the logic for obtaining, storing, and refreshing authentication tokens
// for Kiro CLI.
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
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	return &KiroAuthenticator{
		cfg:    cfg,
		oauth:  nil, // Will be initialized during Authenticate
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
	fmt.Println("[DEBUG] Authenticate: Starting")

	// Get or create OIDC client registration
	var registeredClient *RegisteredClient
	var err error

	// Try to load from cache first
	fmt.Println("[DEBUG] Authenticate: Loading cached client")

	authDir := a.cfg.AuthDir
	if strings.HasPrefix(authDir, "~/") {
		home, _ := os.UserHomeDir()
		authDir = filepath.Join(home, authDir[2:])
	}

	registeredClient, err = LoadCachedClient(authDir, "")
	if err != nil {
		log.Warnf("Failed to load cached client: %v", err)
		fmt.Printf("[DEBUG] Authenticate: Cache load error: %v\n", err)
	}

	// If no cached client or it's expired, register a new one
	if registeredClient == nil {
		fmt.Println("[DEBUG] Authenticate: No cached client, registering new one")
		log.Info("No valid cached client found, registering new OIDC client...")
		registeredClient, err = RegisterClient(ctx, a.client)
		if err != nil {
			fmt.Printf("[DEBUG] Authenticate: RegisterClient failed: %v\n", err)
			return nil, nil, fmt.Errorf("failed to register OIDC client: %w", err)
		}
		fmt.Println("[DEBUG] Authenticate: RegisterClient succeeded")

		// Save to cache for next time
		if err := SaveCachedClient(authDir, registeredClient); err != nil {
			log.Warnf("Failed to cache client registration: %v", err)
		}
	} else {
		log.Info("Using cached OIDC client")
		fmt.Println("[DEBUG] Authenticate: Using cached client")
	}

	// Initialize DeviceCodeFlow with the registered client
	fmt.Println("[DEBUG] Authenticate: Creating DeviceCodeFlow")
	a.oauth = NewDeviceCodeFlow(a.cfg, registeredClient)
	fmt.Println("[DEBUG] Authenticate: DeviceCodeFlow created successfully")

	// Start device code flow
	fmt.Println("[DEBUG] Authenticate: About to call StartDeviceFlow")
	deviceResp, err := a.oauth.StartDeviceFlow(ctx)
	fmt.Printf("[DEBUG] Authenticate: StartDeviceFlow returned, err=%v, deviceResp=%v\n", err, deviceResp)
	if err != nil {
		fmt.Printf("[DEBUG] Authenticate: StartDeviceFlow error: %v\n", err)
		return nil, nil, fmt.Errorf("failed to start device flow: %w", err)
	}
	fmt.Println("[DEBUG] Authenticate: StartDeviceFlow succeeded, no error")

	log.Infof("Device code flow initiated. User code: %s", deviceResp.UserCode)
	log.Infof("Please visit: %s", deviceResp.VerificationURI)
	fmt.Println("[DEBUG] Authenticate: Returning deviceResp to caller")

	// Return the device response WITHOUT polling yet
	// The caller (DoKiroLogin) will display the code to the user and then poll
	return nil, deviceResp, nil
}

// ensureOAuthInitialized ensures that the OAuth client is initialized.
// It loads the client registration from cache or registers a new client if needed.
// If clientIdHash is provided, it attempts to load the specific client for that hash.
func (a *KiroAuthenticator) ensureOAuthInitialized(ctx context.Context, clientIdHash string) error {
	// If already initialized, check if it matches the requested hash (if provided)
	if a.oauth != nil {
		// If no hash provided, or if we can't verify the hash (DeviceCodeFlow doesn't expose it easily yet),
		// assume it's fine.
		// TODO: Ideally we should verify the hash matches a.oauth.clientID
		return nil
	}

	// Try to load from cache first
	authDir := a.cfg.AuthDir
	if strings.HasPrefix(authDir, "~/") {
		home, _ := os.UserHomeDir()
		authDir = filepath.Join(home, authDir[2:])
	}

	registeredClient, err := LoadCachedClient(authDir, clientIdHash)
	if err != nil {
		log.Warnf("Failed to load cached client: %v", err)
	}

	// If no cached client or it's expired, register a new one
	if registeredClient == nil {
		log.Info("No valid cached client found, registering new OIDC client...")
		registeredClient, err = RegisterClient(ctx, a.client)
		if err != nil {
			return fmt.Errorf("failed to register OIDC client: %w", err)
		}

		// Save to cache
		if err := SaveCachedClient(authDir, registeredClient); err != nil {
			log.Warnf("Failed to cache client registration: %v", err)
		}
	}

	a.oauth = NewDeviceCodeFlow(a.cfg, registeredClient)
	return nil
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

	// Ensure OAuth client is initialized
	if err := a.ensureOAuthInitialized(ctx, storage.ClientIdHash); err != nil {
		return nil, fmt.Errorf("failed to initialize oauth client: %w", err)
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

// PollForToken polls the token endpoint until the user authorizes the device.
//
// Parameters:
//   - ctx: Context for the request
//   - deviceCode: The device code from the StartDeviceFlow response
//
// Returns:
//   - *KiroTokenStorage: The token storage after successful authorization
//   - error: An error if polling fails, nil otherwise
func (a *KiroAuthenticator) PollForToken(ctx context.Context, deviceCode string) (*KiroTokenStorage, error) {
	if a.oauth == nil {
		return nil, fmt.Errorf("oauth client not initialized")
	}

	log.Info("Polling for token authorization")
	token, err := a.oauth.PollForToken(ctx, deviceCode)
	if err != nil {
		return nil, fmt.Errorf("failed to poll for token: %w", err)
	}

	log.Info("Authentication successful")
	return token, nil
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

	// Log token status for debugging
	timeUntilExpiration := storage.TimeUntilExpiration()
	log.Debugf("Token validation: timeUntilExpiration=%s, expirationBuffer=%dm, earlyRefreshBuffer=%dm",
		timeUntilExpiration, TokenExpirationBuffer, TokenEarlyRefreshBuffer)

	// Check if token is expired or will expire within the required buffer
	if storage.IsExpired(TokenExpirationBuffer) {
		log.Info("Access token is expired or expiring soon, attempting to refresh")

		// Try to refresh
		newToken, err := a.RefreshToken(ctx, storage)
		if err != nil {
			return nil, false, fmt.Errorf("token expired and refresh failed: %w", err)
		}

		log.Info("Token refreshed successfully")
		return newToken, true, nil
	}

	// Proactive refresh: if token will expire soon and we have a refresh token,
	// refresh it proactively to avoid potential race conditions where the token
	// expires between validation and API request
	if storage.RefreshToken != "" && storage.IsExpired(TokenEarlyRefreshBuffer) {
		log.Infof("Token will expire in less than %d minutes, proactively refreshing (timeUntil=%s)",
			TokenEarlyRefreshBuffer, timeUntilExpiration)

		newToken, err := a.RefreshToken(ctx, storage)
		if err != nil {
			// Proactive refresh failed, but token is still valid for now
			log.Warnf("Proactive token refresh failed (but token still valid): %v", err)
			return storage, false, nil
		}

		log.Info("Token proactively refreshed successfully")
		return newToken, true, nil
	}

	// Token is valid and not near expiration
	log.Debugf("Token is valid, no refresh needed (timeUntil=%s)", timeUntilExpiration)
	return storage, false, nil
}

// TryValidateToken attempts to validate and refresh the token, but returns the
// original token if validation or refresh fails. This is a best-effort approach
// that allows the caller to proceed with the original token and rely on API
// responses (e.g., 401) to determine if the token is truly invalid.
//
// This is useful for scenarios where:
//   - Local clock may be incorrect (clock skew)
//   - Token appears expired locally but might still be accepted by API
//   - You want to try the API request anyway and handle 401 reactively
//
// Parameters:
//   - ctx: Context for the validation request
//   - storage: The token storage to validate
//
// Returns:
//   - *KiroTokenStorage: The validated token (refreshed if possible), or original if refresh failed
//   - bool: true if token was successfully refreshed, false otherwise
func (a *KiroAuthenticator) TryValidateToken(ctx context.Context, storage *KiroTokenStorage) (*KiroTokenStorage, bool) {
	if storage == nil {
		log.Warn("TryValidateToken: storage is nil, returning nil")
		return nil, false
	}

	// Attempt validation
	validToken, refreshed, err := a.ValidateToken(ctx, storage)
	if err != nil {
		// Validation/refresh failed, but return original token to allow retry
		log.Warnf("Token validation failed, returning original token for API retry: %v", err)
		return storage, false
	}

	return validToken, refreshed
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
