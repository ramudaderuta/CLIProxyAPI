// Package kiro provides authentication and token management functionality
// for Kiro AI services. It handles OAuth token storage, serialization,
// and retrieval for maintaining authenticated sessions with the Kiro API.
package kiro

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	data, err := os.ReadFile(authFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open token file: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("failed to decode token file: file is empty")
	}

	var token KiroTokenStorage
	if err = json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("failed to decode token file: %w", err)
	}

	var raw map[string]any
	if err = json.Unmarshal(data, &raw); err == nil {
		token.AccessToken = coalesceString(token.AccessToken, raw, "accessToken", "access_token")
		token.RefreshToken = coalesceString(token.RefreshToken, raw, "refreshToken", "refresh_token")
		token.ProfileArn = coalesceString(token.ProfileArn, raw, "profileArn", "profile_arn")
		token.AuthMethod = coalesceString(token.AuthMethod, raw, "authMethod", "auth_method")
		token.Provider = coalesceString(token.Provider, raw, "provider")
		if token.ExpiresAt.IsZero() {
			if ts, ok := coalesceTime(raw, "expiresAt", "expires_at"); ok {
				token.ExpiresAt = ts
			}
		}
		if token.Type == "" {
			token.Type = coalesceString("", raw, "type")
		}
	}

	if strings.TrimSpace(token.Type) == "" {
		token.Type = "kiro"
		log.Infof("[Kiro Auth] token file %s missing type; enhancing in memory", authFilePath)
	} else if !strings.EqualFold(token.Type, "kiro") {
		log.Warnf("[Kiro Auth] token file %s has unexpected type %q; overriding to \"kiro\"", authFilePath, token.Type)
		token.Type = "kiro"
	}

	if err := validateKiroToken(&token); err != nil {
		return nil, fmt.Errorf("invalid kiro token file %s: %w", authFilePath, err)
	}

	return &token, nil
}

// IsExpired checks if the access token has expired.
// Returns true if the token is expired or will expire within 5 minutes.
func (ts *KiroTokenStorage) IsExpired() bool {
	// Consider token expired if it expires within 5 minutes to provide buffer
	return time.Until(ts.ExpiresAt) < 5*time.Minute
}

func coalesceString(current string, raw map[string]any, keys ...string) string {
	if strings.TrimSpace(current) != "" {
		return current
	}
	for _, key := range keys {
		if value, ok := raw[key]; ok {
			switch typed := value.(type) {
			case string:
				if trimmed := strings.TrimSpace(typed); trimmed != "" {
					return trimmed
				}
			}
		}
	}
	return current
}

func coalesceTime(raw map[string]any, keys ...string) (time.Time, bool) {
	for _, key := range keys {
		value, ok := raw[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case string:
			trimmed := strings.TrimSpace(typed)
			if trimmed == "" {
				continue
			}
			if ts, err := time.Parse(time.RFC3339, trimmed); err == nil {
				return ts, true
			}
			if unix, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
				return time.Unix(unix, 0), true
			}
		case json.Number:
			if val, err := typed.Int64(); err == nil {
				return time.Unix(val, 0), true
			}
		case float64:
			return time.Unix(int64(typed), 0), true
		case int64:
			return time.Unix(typed, 0), true
		}
	}
	return time.Time{}, false
}

func validateKiroToken(token *KiroTokenStorage) error {
	if token == nil {
		return fmt.Errorf("token payload is empty")
	}
	if strings.TrimSpace(token.AccessToken) == "" {
		return fmt.Errorf("accessToken is required")
	}
	if strings.TrimSpace(token.RefreshToken) == "" {
		return fmt.Errorf("refreshToken is required")
	}
	if token.ExpiresAt.IsZero() {
		return fmt.Errorf("expiresAt is required")
	}
	if token.IsExpired() {
		return fmt.Errorf("token is expired")
	}
	return nil
}
