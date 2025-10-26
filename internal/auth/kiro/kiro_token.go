// Package kiro provides authentication and token management functionality
// for Kiro AI services. It handles OAuth token storage, serialization,
// and retrieval for maintaining authenticated sessions with the Kiro API.
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

// KiroTokenStorage stores OAuth token information for Kiro API authentication.
// It maintains compatibility with the existing auth system while adding Kiro-specific fields
// for managing access tokens, refresh tokens, and profile information.
type KiroTokenStorage struct {
	// AccessToken is the OAuth access token for API requests.
	AccessToken string `json:"accessToken"`

	// RefreshToken is used to obtain new access tokens when they expire.
	RefreshToken string `json:"refreshToken"`

	// ProfileArn contains the AWS profile ARN for the authenticated user.
	ProfileArn string `json:"profileArn"`

	// ExpiresAt indicates when the access token expires.
	ExpiresAt time.Time `json:"expiresAt"`

	// AuthMethod indicates the authentication method used (e.g., "social").
	AuthMethod string `json:"authMethod"`

	// Provider indicates the OAuth provider used (e.g., "Github").
	Provider string `json:"provider"`

	// Type indicates the authentication provider type, always "kiro" for this storage.
	Type string `json:"type"`
}

// SaveTokenToFile serializes the Kiro token storage to a JSON file.
// This method creates the necessary directory structure and writes the token
// data in JSON format to the specified file path for persistent storage.
//
// Parameters:
//   - authFilePath: The full path where the token file should be saved
//
// Returns:
//   - error: An error if the operation fails, nil otherwise
func (ts *KiroTokenStorage) SaveTokenToFile(authFilePath string) error {
	misc.LogSavingCredentials(authFilePath)
	ts.Type = "kiro"
	if err := os.MkdirAll(filepath.Dir(authFilePath), 0700); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	f, err := os.Create(authFilePath)
	if err != nil {
		return fmt.Errorf("failed to create token file: %w", err)
	}
	defer func() {
		if errClose := f.Close(); errClose != nil {
			log.Errorf("failed to close file: %v", errClose)
		}
	}()

	if err = json.NewEncoder(f).Encode(ts); err != nil {
		return fmt.Errorf("failed to write token to file: %w", err)
	}
	return nil
}

// LoadTokenFromFile loads Kiro token storage from a JSON file.
// This method reads and deserializes the token data from the specified file path.
//
// Parameters:
//   - authFilePath: The full path to the token file to load
//
// Returns:
//   - *KiroTokenStorage: The loaded token storage
//   - error: An error if the operation fails, nil otherwise
func LoadTokenFromFile(authFilePath string) (*KiroTokenStorage, error) {
	f, err := os.Open(authFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open token file: %w", err)
	}
	defer func() {
		if errClose := f.Close(); errClose != nil {
			log.Errorf("failed to close file: %v", errClose)
		}
	}()

	var token KiroTokenStorage
	if err = json.NewDecoder(f).Decode(&token); err != nil {
		return nil, fmt.Errorf("failed to decode token file: %w", err)
	}

	// Parse the expiresAt field from string if needed
	if token.ExpiresAt.IsZero() {
		// Try to read the file again as raw JSON to handle string format
		var rawToken map[string]interface{}
		f.Seek(0, 0)
		if err = json.NewDecoder(f).Decode(&rawToken); err == nil {
			if expiresStr, ok := rawToken["expiresAt"].(string); ok {
				if parsed, parseErr := time.Parse(time.RFC3339, expiresStr); parseErr == nil {
					token.ExpiresAt = parsed
				}
			}
		}
	}

	token.Type = "kiro"
	return &token, nil
}

// IsExpired checks if the access token has expired.
// Returns true if the token is expired or will expire within 5 minutes.
func (ts *KiroTokenStorage) IsExpired() bool {
	// Consider token expired if it expires within 5 minutes to provide buffer
	return time.Until(ts.ExpiresAt) < 5*time.Minute
}