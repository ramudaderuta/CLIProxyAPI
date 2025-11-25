package kiro

import (
	"os"
	"path/filepath"
)

// DefaultTokenPath returns the default path for the Kiro token file.
// It defaults to ~/.cli-proxy-api/kiro-token.json
func DefaultTokenPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "./kiro-token.json"
	}
	return filepath.Join(homeDir, ".cli-proxy-api", "kiro-token.json")
}
