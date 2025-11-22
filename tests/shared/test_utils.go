// Package shared provides Kiro test fixtures and utilities
package shared

import (
	"context"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

// KiroTestFixtures provides common test fixtures for Kiro tests
type KiroTestFixtures struct {
	Cfg           *config.Config
	TokenStorage  *kiro.KiroTokenStorage
	Authenticator *kiro.KiroAuthenticator
	Ctx           context.Context
	T             *testing.T
}

// NewKiroTestFixtures creates a new test fixtures instance
func NewKiroTestFixtures(t *testing.T) *KiroTestFixtures {
	t.Helper()

	cfg := &config.Config{
		AuthDir: TempDir(t, "kiro-test-auth-*"),
		KiroConfig: config.KiroConfig{
			TokenFiles: []config.KiroTokenFile{
				{
					Path:   TempFile(t, "kiro-token-*.json").Name(),
					Region: "us-east-1",
					Label:  "test",
				},
			},
		},
	}

	tokenStorage := &kiro.KiroTokenStorage{
		AccessToken:  "test_access_token_" + RandomString(32),
		RefreshToken: "test_refresh_token_" + RandomString(32),
		ProfileArn:   "arn:aws:codewhisperer:us-east-1:123456789:profile/test",
		ExpiresAt:    time.Now().Add(1 * time.Hour),
		AuthMethod:   "IdC",
		Provider:     "BuilderId",
		Region:       "us-east-1",
	}

	return &KiroTestFixtures{
		Cfg:           cfg,
		TokenStorage:  tokenStorage,
		Authenticator: kiro.NewKiroAuthenticator(cfg),
		Ctx:           context.Background(),
		T:             t,
	}
}

// WithExpiredToken returns a copy of fixtures with an expired token
func (ktf *KiroTestFixtures) WithExpiredToken() *KiroTestFixtures {
	ktf.T.Helper()

	expired := *ktf
	expiredStorage := *ktf.TokenStorage
	expiredStorage.ExpiresAt = time.Now().Add(-1 * time.Hour)
	expired.TokenStorage = &expiredStorage

	return &expired
}

// WithCustomConfig returns a copy with a custom config
func (ktf *KiroTestFixtures) WithCustomConfig(cfg *config.Config) *KiroTestFixtures {
	ktf.T.Helper()

	custom := *ktf
	custom.Cfg = cfg
	custom.Authenticator = kiro.NewKiroAuthenticator(cfg)

	return &custom
}

// SaveToken saves the token to the configured path
func (ktf *KiroTestFixtures) SaveToken() {
	ktf.T.Helper()

	if len(ktf.Cfg.KiroConfig.TokenFiles) == 0 {
		ktf.T.Fatal("No token file configured")
	}

	path := ktf.Cfg.KiroConfig.TokenFiles[0].Path
	if err := ktf.TokenStorage.SaveTokenToFile(path); err != nil {
		ktf.T.Fatalf("Failed to save token: %v", err)
	}
}

// LoadToken loads the token from the configured path
func (ktf *KiroTestFixtures) LoadToken() *kiro.KiroTokenStorage {
	ktf.T.Helper()

	if len(ktf.Cfg.KiroConfig.TokenFiles) == 0 {
		ktf.T.Fatal("No token file configured")
	}

	path := ktf.Cfg.KiroConfig.TokenFiles[0].Path
	storage, err := kiro.LoadTokenFromFile(path)
	if err != nil {
		ktf.T.Fatalf("Failed to load token: %v", err)
	}

	return storage
}

// Cleanup cleans up test fixtures
func (ktf *KiroTestFixtures) Cleanup() {
	// Context cleanup is handled by testing.T.Cleanup
}

// Test Data Constants

const (
	// TestModel is a test model name
	TestModel = "kiro-sonnet"

	// TestPrompt is a simple test prompt
	TestPrompt = "Write a hello world program"

	// TestResponse is a simple test response
	TestResponse = "Here's a hello world program in Python:\n\nprint('Hello, World!')"
)

// Common Test Messages

var (
	// SimpleMessages is a basic conversation
	SimpleMessages = []map[string]interface{}{
		BuildSimpleMessage("user", TestPrompt),
	}

	// MultiTurnMessages is a multi-turn conversation
	MultiTurnMessages = []map[string]interface{}{
		BuildSimpleMessage("system", "You are a helpful assistant"),
		BuildSimpleMessage("user", "Hello"),
		BuildSimpleMessage("assistant", "Hi! How can I help you?"),
		BuildSimpleMessage("user", TestPrompt),
	}
)
