package kiro

import (
	"os"
	"path/filepath"
)

// DefaultTokenPath returns the default path for the Kiro token file.
// It follows the same pattern as kiro-cli: ~/.kiro/auth.json
func DefaultTokenPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "./kiro-token.json"
	}
	return filepath.Join(homeDir, ".kiro", "auth.json")
}
