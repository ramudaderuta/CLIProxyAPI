package kiro

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
)

// LoadCachedClient loads a cached OIDC client from disk.
// It first attempts to load from the provided auth directory, then falls back to
// auto-discovery by scanning the provided auth directory for JSON files containing clientId.
//
// Parameters:
//   - authDir: The directory to scan for client files
//
// Returns:
//   - *RegisteredClient: The cached client, or nil if not found or expired
//   - error: An error if loading fails (not including file not found)
func LoadCachedClient(authDir string) (*RegisteredClient, error) {
	if authDir == "" {
		// Fallback to default if no authDir provided (though it should be)
		authDir = filepath.Dir(DefaultTokenPath())
	}

	// 1. Try standard oidc_client.json in authDir
	standardPath := filepath.Join(authDir, "oidc_client.json")
	client, err := loadClientFromPath(standardPath)
	if err == nil && client != nil {
		return client, nil
	}
	if err != nil && !os.IsNotExist(err) {
		log.Warnf("Failed to load client from path %s: %v", standardPath, err)
	}

	// 2. Fallback: Auto-discover client files in auth directory
	log.Info("Standard oidc_client.json not found, scanning auth directory for client files...")

	client, err = discoverClientInDirectory(authDir)
	if err != nil {
		return nil, err
	}
	if client != nil {
		log.Infof("Auto-discovered OIDC client from auth directory: %s", authDir)
		return client, nil
	}

	// No client found
	return nil, nil
}

// loadClientFromPath loads a client from a specific file path.
func loadClientFromPath(path string) (*RegisteredClient, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var client RegisteredClient
	if err := json.Unmarshal(data, &client); err != nil {
		return nil, NewAuthError("loadClientFromPath", err, "failed to parse client file")
	}

	// Check if expired
	if client.IsExpired() {
		log.Debugf("Client at %s is expired", path)
		return nil, nil // Expired cache
	}

	log.Debugf("Loaded valid client from %s", path)
	return &client, nil
}

// discoverClientInDirectory scans a directory for JSON files containing clientId.
func discoverClientInDirectory(dir string) (*RegisteredClient, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Directory doesn't exist
		}
		return nil, NewAuthError("discoverClientInDirectory", err, "failed to read auth directory")
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only check .json files
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		filePath := filepath.Join(dir, entry.Name())

		// Try to read and parse as potential client file
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue // Skip files we can't read
		}

		// Quick check: does it contain "clientId" field?
		var raw map[string]interface{}
		if err := json.Unmarshal(data, &raw); err != nil {
			continue // Not valid JSON
		}

		if _, hasClientId := raw["clientId"]; !hasClientId {
			continue // Not a client file
		}

		// Try to unmarshal as RegisteredClient
		var client RegisteredClient
		if err := json.Unmarshal(data, &client); err != nil {
			log.Debugf("File %s has clientId but failed to parse as RegisteredClient: %v", entry.Name(), err)
			continue
		}

		// Validate client ID is not empty
		if client.ClientID == "" {
			continue
		}

		// Note: We intentionally skip IsExpired() check for auto-discovered files.
		// If the user has a token file that matches this client, we should try to use it
		// even if our local cache policy says it's "expired". The API is the ultimate source of truth.

		// Found a valid client!
		log.Infof("Auto-discovered valid OIDC client from file: %s", entry.Name())
		return &client, nil
	}

	// No valid client found
	return nil, nil
}

// SaveCachedClient saves an OIDC client to disk cache.
//
// Parameters:
//   - authDir: The directory to save the client file to
//   - client: The client registration to save
//
// Returns:
//   - error: An error if saving fails
func SaveCachedClient(authDir string, client *RegisteredClient) error {
	if authDir == "" {
		authDir = filepath.Dir(DefaultTokenPath())
	}
	path := filepath.Join(authDir, "oidc_client.json")

	data, err := json.MarshalIndent(client, "", "  ")
	if err != nil {
		return NewAuthError("SaveCachedClient", err, "failed to marshal client")
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return NewAuthError("SaveCachedClient", err, "failed to create directory")
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return NewAuthError("SaveCachedClient", err, "failed to write cache file")
	}

	return nil
}
