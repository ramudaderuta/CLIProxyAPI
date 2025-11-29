package kiro

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	log "github.com/sirupsen/logrus"
)

// TokenManager manages multiple Kiro tokens for rotation and failover.
// It provides round-robin token selection and automatic failover when tokens fail.
type TokenManager struct {
	cfg           *config.Config
	tokens        []*TokenEntry
	currentIndex  int
	mu            sync.RWMutex
	authenticator *KiroAuthenticator
}

// TokenEntry represents a single token with its configuration and status.
type TokenEntry struct {
	Storage   *KiroTokenStorage
	Path      string
	Region    string
	Label     string
	LastUsed  time.Time
	FailCount int
	Disabled  bool
}

// NewTokenManager creates a new token manager from configuration.
func NewTokenManager(cfg *config.Config) *TokenManager {
	return &TokenManager{
		cfg:           cfg,
		tokens:        make([]*TokenEntry, 0),
		currentIndex:  0,
		authenticator: NewKiroAuthenticator(cfg),
	}
}

// LoadTokens loads all token files using auto-discovery from auth-dir.
// It scans auth-dir for kiro-*.json files and loads them automatically.
func (tm *TokenManager) LoadTokens(ctx context.Context) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.tokens = make([]*TokenEntry, 0)

	// Auto-discover kiro-*.json files from auth-dir
	discoveredFiles, err := tm.discoverTokenFiles()
	if err != nil {
		log.Debugf("Failed to auto-discover Kiro token files: %v", err)
		// Not an error if no tokens found - Kiro is optional
		return nil
	}

	// Load discovered files
	for _, filePath := range discoveredFiles {
		storage, err := LoadTokenFromFile(filePath)
		if err != nil {
			log.Warnf("Failed to load token from %s: %v", filePath, err)
			continue
		}

		// Extract label from filename (kiro-xxx.json -> xxx)
		label := extractLabelFromPath(filePath)

		entry := &TokenEntry{
			Storage:   storage,
			Path:      filePath,
			Region:    storage.Region,
			Label:     label,
			LastUsed:  time.Time{},
			FailCount: 0,
			Disabled:  false,
		}

		tm.tokens = append(tm.tokens, entry)
		log.Infof("Auto-discovered Kiro token from %s (label: %s)", filePath, label)
	}

	if len(tm.tokens) == 0 {
		// Not an error if no tokens found - Kiro is optional
		log.Debugf("No Kiro tokens loaded from auto-discovery")
		return nil
	}

	log.Infof("Loaded %d Kiro token(s) for rotation", len(tm.tokens))
	return nil
}

// GetNextToken returns the next available token entry using round-robin selection.
// It automatically validates and refreshes tokens as needed.
func (tm *TokenManager) GetNextToken(ctx context.Context) (*TokenEntry, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if len(tm.tokens) == 0 {
		return nil, fmt.Errorf("no tokens available")
	}

	// Try up to len(tokens) times to find a working token
	attempts := len(tm.tokens)
	for i := 0; i < attempts; i++ {
		entry := tm.tokens[tm.currentIndex]

		// Skip disabled tokens
		if entry.Disabled {
			tm.currentIndex = (tm.currentIndex + 1) % len(tm.tokens)
			continue
		}

		// Validate and refresh if needed
		validToken, refreshed, err := tm.authenticator.ValidateToken(ctx, entry.Storage)
		if err != nil {
			log.Warnf("Token validation failed for %s: %v", entry.Label, err)
			entry.FailCount++

			// Disable token after 3 consecutive failures
			if entry.FailCount >= 3 {
				log.Errorf("Disabling token %s after %d failures", entry.Label, entry.FailCount)
				entry.Disabled = true
			}

			tm.currentIndex = (tm.currentIndex + 1) % len(tm.tokens)
			continue
		}

		// Update entry with refreshed token if needed
		if refreshed {
			entry.Storage = validToken
			// Save refreshed token back to file
			if err := validToken.SaveTokenToFile(entry.Path); err != nil {
				log.Warnf("Failed to save refreshed token to %s: %v", entry.Path, err)
			} else {
				log.Infof("Refreshed and saved token for %s", entry.Label)
			}
		}

		// Reset fail count on success
		entry.FailCount = 0
		entry.LastUsed = time.Now()

		// Move to next token for round-robin
		tm.currentIndex = (tm.currentIndex + 1) % len(tm.tokens)

		return entry, nil
	}

	return nil, fmt.Errorf("all tokens failed validation")
}

// GetTokenCount returns the number of loaded tokens.
func (tm *TokenManager) GetTokenCount() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return len(tm.tokens)
}

// GetActiveTokenCount returns the number of active (non-disabled) tokens.
func (tm *TokenManager) GetActiveTokenCount() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	count := 0
	for _, entry := range tm.tokens {
		if !entry.Disabled {
			count++
		}
	}
	return count
}

// ResetFailures resets failure counts for all tokens.
// This can be called periodically to re-enable tokens that may have had temporary issues.
func (tm *TokenManager) ResetFailures() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	for _, entry := range tm.tokens {
		if entry.Disabled && entry.FailCount >= 3 {
			log.Infof("Re-enabling token %s after failure reset", entry.Label)
			entry.Disabled = false
			entry.FailCount = 0
		}
	}
}

// GetTokenStats returns statistics about token usage.
func (tm *TokenManager) GetTokenStats() map[string]interface{} {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["total_tokens"] = len(tm.tokens)
	stats["active_tokens"] = 0
	stats["disabled_tokens"] = 0

	tokenDetails := make([]map[string]interface{}, 0, len(tm.tokens))
	for _, entry := range tm.tokens {
		if entry.Disabled {
			stats["disabled_tokens"] = stats["disabled_tokens"].(int) + 1
		} else {
			stats["active_tokens"] = stats["active_tokens"].(int) + 1
		}

		detail := map[string]interface{}{
			"label":       entry.Label,
			"path":        entry.Path,
			"region":      entry.Region,
			"last_used":   entry.LastUsed,
			"fail_count":  entry.FailCount,
			"disabled":    entry.Disabled,
			"profile_arn": entry.Storage.ProfileArn,
		}
		tokenDetails = append(tokenDetails, detail)
	}

	stats["tokens"] = tokenDetails
	return stats
}

// discoverTokenFiles scans auth-dir for kiro-*.json files
func (tm *TokenManager) discoverTokenFiles() ([]string, error) {
	authDir := tm.cfg.AuthDir
	if authDir == "" {
		// Use default ~/.kiro directory
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		authDir = filepath.Join(home, ".cli-proxy-api")
	}

	// Expand ~ if present
	if strings.HasPrefix(authDir, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		authDir = filepath.Join(home, authDir[2:])
	}

	// Check if directory exists
	if _, err := os.Stat(authDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("auth directory does not exist: %s", authDir)
	}

	// Find all kiro-*.json files
	pattern := filepath.Join(authDir, "kiro-*.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to glob pattern %s: %w", pattern, err)
	}

	log.Infof("Auto-discovered %d Kiro token file(s) in %s", len(matches), authDir)
	return matches, nil
}

// extractLabelFromPath extracts a label from filename
// kiro-xxx.json -> xxx, auth.json -> default
func extractLabelFromPath(path string) string {
	base := filepath.Base(path)

	// Remove .json extension
	name := strings.TrimSuffix(base, ".json")

	// If it starts with kiro-, extract the suffix
	if strings.HasPrefix(name, "kiro-") {
		return name[5:] // Remove "kiro-" prefix
	}

	// If it's auth.json, use "default"
	if name == "auth" {
		return "default"
	}

	return name
}
