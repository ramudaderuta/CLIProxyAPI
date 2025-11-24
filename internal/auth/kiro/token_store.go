// Package kiro provides authentication and token management functionality
// for Kiro CLI. It handles OAuth2 device code flow
// authentication, token storage, and refresh for both GitHub OAuth and AWS
// Builder ID authentication methods.
package kiro

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/misc"
	log "github.com/sirupsen/logrus"
)

// KiroTokenStorage stores OAuth2 token information for Kiro CLI authentication.
// It maintains compatibility with the kiro-cli token storage format, supporting
// both social OAuth (GitHub) and IdC (AWS Builder ID) authentication methods.
type KiroTokenStorage struct {
	// AccessToken is the OAuth access token used to authenticate API requests.
	AccessToken string `json:"accessToken"`

	// RefreshToken is the OAuth refresh token used to obtain new access tokens.
	RefreshToken string `json:"refreshToken"`

	// ProfileArn is the AWS profile ARN associated with this token.
	ProfileArn string `json:"profileArn"`

	// ExpiresAt is the timestamp when the access token expires.
	ExpiresAt time.Time `json:"expiresAt"`

	// AuthMethod indicates the authentication method: "social" or "IdC".
	AuthMethod string `json:"authMethod"`

	// Provider indicates the OAuth provider: "Github" or "BuilderId".
	Provider string `json:"provider"`

	// Region is the AWS region for the Kiro API endpoint (optional).
	Region string `json:"region,omitempty"`

	// ClientIdHash is the hashed client ID (for IdC auth, optional).
	ClientIdHash string `json:"clientIdHash,omitempty"`
}

// IsExpired checks if the access token has expired.
// It uses a buffer time to treat tokens as expired slightly before their
// actual expiration time to account for clock skew and request latency.
//
// Parameters:
//   - bufferMinutes: Number of minutes before expiration to treat as expired
//
// Returns:
//   - bool: true if the token is expired or will expire within the buffer period
func (ts *KiroTokenStorage) IsExpired(bufferMinutes int) bool {
	if ts.ExpiresAt.IsZero() {
		// If no expiration is set, assume the token is valid.
		log.Debug("Token expiration time is zero, treating as not expired")
		return false
	}

	buffer := time.Duration(bufferMinutes) * time.Minute
	now := time.Now()
	expirationThreshold := now.Add(buffer)
	isExpired := expirationThreshold.After(ts.ExpiresAt)

	// Add diagnostic logging to help troubleshoot timing issues
	if isExpired {
		timeUntilExpiration := time.Until(ts.ExpiresAt)
		log.Infof("Token is expired or will expire soon: now=%s, expiresAt=%s, buffer=%dm, timeUntil=%s",
			now.Format(time.RFC3339),
			ts.ExpiresAt.Format(time.RFC3339),
			bufferMinutes,
			timeUntilExpiration)
	} else {
		// Log at debug level when token is still valid
		timeUntilExpiration := time.Until(ts.ExpiresAt)
		log.Debugf("Token is valid: timeUntilExpiration=%s, buffer=%dm", timeUntilExpiration, bufferMinutes)
	}

	return isExpired
}

// TimeUntilExpiration returns the duration until the token expires.
// If the token has no expiration set, returns a very large duration.
// If the token is already expired, returns a negative duration.
//
// Returns:
//   - time.Duration: Time until expiration (negative if already expired)
func (ts *KiroTokenStorage) TimeUntilExpiration() time.Duration {
	if ts.ExpiresAt.IsZero() {
		// Return a large duration to indicate "no expiration"
		return time.Hour * 24 * 365 // 1 year
	}
	return time.Until(ts.ExpiresAt)
}

// IsActuallyExpired checks if the token is truly expired without any buffer time.
// This is useful for debugging to distinguish between buffer-based expiration
// and actual expiration.
//
// Returns:
//   - bool: true if the token is actually expired (past expiration time)
func (ts *KiroTokenStorage) IsActuallyExpired() bool {
	if ts.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(ts.ExpiresAt)
}

// SaveTokenToFile serializes the Kiro token storage to a JSON file.
// This method creates the necessary directory structure and writes the token
// data in JSON format to the specified file path for persistent storage.
// The file is created with 0700 permissions for security.
//
// Parameters:
//   - authFilePath: The full path where the token file should be saved
//
// Returns:
//   - error: An error if the operation fails, nil otherwise
func (ts *KiroTokenStorage) SaveTokenToFile(authFilePath string) error {
	misc.LogSavingCredentials(authFilePath)

	// Create parent directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(authFilePath), 0700); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	// Create the file with restrictive permissions
	f, err := os.OpenFile(authFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create token file: %w", err)
	}
	defer func() {
		if errClose := f.Close(); errClose != nil {
			log.Errorf("failed to close file: %v", errClose)
		}
	}()

	// Write token data as JSON
	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	if err = encoder.Encode(ts); err != nil {
		return fmt.Errorf("failed to write token to file: %w", err)
	}

	log.Infof("Kiro token saved successfully to %s", authFilePath)
	return nil
}

// LoadTokenFromFile reads and deserializes a Kiro token from a JSON file.
// It validates the token structure after loading.
//
// Parameters:
//   - authFilePath: The full path to the token file
//
// Returns:
//   - *KiroTokenStorage: The loaded token storage
//   - error: An error if the operation fails, nil otherwise
func LoadTokenFromFile(authFilePath string) (*KiroTokenStorage, error) {
	data, err := os.ReadFile(authFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read token file: %w", err)
	}

	var storage KiroTokenStorage
	if err := json.Unmarshal(data, &storage); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token: %w", err)
	}

	// Set default region if not present
	if storage.Region == "" {
		storage.Region = "us-east-1" // Default to us-east-1
		log.Debugf("No region found in token file, using default: us-east-1")
	}

	// Validate required fields
	if storage.AccessToken == "" {
		return nil, fmt.Errorf("invalid token: missing accessToken")
	}
	if storage.RefreshToken == "" {
		return nil, fmt.Errorf("invalid token: missing refreshToken")
	}
	if storage.ProfileArn == "" {
		return nil, fmt.Errorf("invalid token: missing profileArn")
	}

	return &storage, nil
}
