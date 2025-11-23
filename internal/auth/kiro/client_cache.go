package kiro

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// clientCachePath returns the path to the cached client file.
func clientCachePath() string {
	return filepath.Join(DefaultTokenPath(), "..", "oidc_client.json")
}

// LoadCachedClient loads a cached OIDC client from disk.
//
// Returns:
//   - *RegisteredClient: The cached client, or nil if not found or expired
//   - error: An error if loading fails (not including file not found)
func LoadCachedClient() (*RegisteredClient, error) {
	path := clientCachePath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Not an error, just no cache
		}
		return nil, NewAuthError("LoadCachedClient", err, "failed to read cache file")
	}

	var client RegisteredClient
	if err := json.Unmarshal(data, &client); err != nil {
		return nil, NewAuthError("LoadCachedClient", err, "failed to parse cache")
	}

	// Check if expired
	if client.IsExpired() {
		return nil, nil // Expired cache
	}

	return &client, nil
}

// SaveCachedClient saves an OIDC client to disk cache.
//
// Parameters:
//   - client: The client to cache
//
// Returns:
//   - error: An error if saving fails
func SaveCachedClient(client *RegisteredClient) error {
	path := clientCachePath()

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return NewAuthError("SaveCachedClient", err, "failed to create cache directory")
	}

	data, err := json.MarshalIndent(client, "", "  ")
	if err != nil {
		return NewAuthError("SaveCachedClient", err, "failed to marshal client")
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return NewAuthError("SaveCachedClient", err, "failed to write cache file")
	}

	return nil
}
