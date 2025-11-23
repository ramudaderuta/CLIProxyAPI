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
	Config    config.KiroTokenFile
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

// LoadTokens loads all configured token files.
// If auto-discover is enabled (default) or token-files is empty, it will scan auth-dir for kiro-*.json files.
// If tokens are found, Kiro will be automatically enabled even if not explicitly configured.
func (tm *TokenManager) LoadTokens(ctx context.Context) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Check if Kiro is explicitly disabled AND no token files provided
	if tm.cfg != nil && !tm.cfg.KiroConfig.Enabled && len(tm.cfg.KiroConfig.TokenFiles) == 0 {
		log.Debugf("Kiro explicitly disabled in configuration")
		return nil
	}

	// Auto-discover is enabled by default unless token files are explicitly provided
	shouldAutoDiscover := true
	if tm.cfg != nil && len(tm.cfg.KiroConfig.TokenFiles) > 0 {
		// If token files are explicitly provided, don't auto-discover
		shouldAutoDiscover = false
	}

	tm.tokens = make([]*TokenEntry, 0)

	if shouldAutoDiscover {
		// Auto-discover kiro-*.json files from auth-dir
		discoveredFiles, err := tm.discoverTokenFiles()
		if err != nil {
			log.Debugf("Failed to auto-discover Kiro token files: %v", err)
			// Don't return error, just skip auto-discovery
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
				Storage: storage,
				Config: config.KiroTokenFile{
					Path:   filePath,
					Region: storage.Region,
					Label:  label,
				},
				LastUsed:  time.Time{},
				FailCount: 0,
				Disabled:  false,
			}

			tm.tokens = append(tm.tokens, entry)
			log.Infof("Auto-discovered Kiro token from %s (label: %s)", filePath, label)
		}
	} else if tm.cfg != nil && len(tm.cfg.KiroConfig.TokenFiles) > 0 {
		// Load explicitly configured token files
		for _, tokenFileCfg := range tm.cfg.KiroConfig.TokenFiles {
			storage, err := LoadTokenFromFile(tokenFileCfg.Path)
			if err != nil {
				log.Warnf("Failed to load token from %s: %v", tokenFileCfg.Path, err)
				continue
			}

			entry := &TokenEntry{
				Storage:   storage,
				Config:    tokenFileCfg,
				LastUsed:  time.Time{},
				FailCount: 0,
				Disabled:  false,
			}

			tm.tokens = append(tm.tokens, entry)
			log.Infof("Loaded Kiro token from %s (label: %s)", tokenFileCfg.Path, tokenFileCfg.Label)
		}
	}

	if len(tm.tokens) == 0 {
		// Not an error if no tokens found - Kiro is optional
		log.Debugf("No Kiro tokens loaded (auto-discover: %v)", shouldAutoDiscover)
		return nil
	}

	log.Infof("Loaded %d Kiro token(s) for rotation", len(tm.tokens))
	return nil
}

// GetNextToken returns the next available token using round-robin selection.
// It automatically validates and refreshes tokens as needed.
func (tm *TokenManager) GetNextToken(ctx context.Context) (*KiroTokenStorage, error) {
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
			log.Warnf("Token validation failed for %s: %v", entry.Config.Label, err)
			entry.FailCount++

			// Disable token after 3 consecutive failures
			if entry.FailCount >= 3 {
				log.Errorf("Disabling token %s after %d failures", entry.Config.Label, entry.FailCount)
				entry.Disabled = true
			}

			tm.currentIndex = (tm.currentIndex + 1) % len(tm.tokens)
			continue
		}

		// Update entry with refreshed token if needed
		if refreshed {
			entry.Storage = validToken
			// Save refreshed token back to file
			if err := validToken.SaveTokenToFile(entry.Config.Path); err != nil {
				log.Warnf("Failed to save refreshed token to %s: %v", entry.Config.Path, err)
			} else {
				log.Infof("Refreshed and saved token for %s", entry.Config.Label)
			}
		}

		// Reset fail count on success
		entry.FailCount = 0
		entry.LastUsed = time.Now()

		// Move to next token for round-robin
		tm.currentIndex = (tm.currentIndex + 1) % len(tm.tokens)

		return validToken, nil
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
			log.Infof("Re-enabling token %s after failure reset", entry.Config.Label)
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
			"label":       entry.Config.Label,
			"path":        entry.Config.Path,
			"region":      entry.Config.Region,
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
		authDir = filepath.Join(home, ".kiro")
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
