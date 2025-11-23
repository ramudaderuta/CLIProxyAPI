package kiro_test

import (
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/runtime/executor"
)

// TestKiroPackagesCompile is a smoke test to ensure all Kiro packages compile
func TestKiroPackagesCompile(t *testing.T) {
	t.Run("AuthPackage", func(t *testing.T) {
		cfg := &config.Config{}
		auth := kiro.NewKiroAuthenticator(cfg)
		if auth == nil {
			t.Fatal("KiroAuthenticator should not be nil")
		}
	})

	t.Run("ExecutorPackage", func(t *testing.T) {
		cfg := &config.Config{}
		exec := executor.NewKiroExecutor(cfg)
		if exec == nil {
			t.Fatal("KiroExecutor should not be nil")
		}
		if exec.Identifier() != "kiro" {
			t.Errorf("Expected identifier 'kiro', got '%s'", exec.Identifier())
		}
	})

	t.Run("ModelRegistry", func(t *testing.T) {
		models := registry.GetKiroModels()
		if len(models) == 0 {
			t.Fatal("GetKiroModels should return at least one model")
		}

		expectedModels := map[string]bool{
			"kiro-sonnet": false,
			"kiro-opus":   false,
			"kiro-haiku":  false,
		}

		for _, model := range models {
			if _, exists := expectedModels[model.ID]; exists {
				expectedModels[model.ID] = true
			}
		}

		for modelID, found := range expectedModels {
			if !found {
				t.Errorf("Expected model '%s' not found in registry", modelID)
			}
		}
	})
}

// TestTokenStorage tests basic token storage functionality
func TestTokenStorage(t *testing.T) {
	// Create a future expiration time
	futureTime := time.Now().Add(24 * time.Hour)

	token := &kiro.KiroTokenStorage{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		ProfileArn:   "arn:aws:codewhisperer:us-east-1:123456789:profile/test",
		ExpiresAt:    futureTime,
		AuthMethod:   "IdC",
		Provider:     "BuilderId",
		Region:       "us-east-1",
	}

	if token.IsExpired(0) {
		t.Error("Token with future expiration should not be expired")
	}
}

// TestDefaultTokenPath tests the default token path function
func TestDefaultTokenPath(t *testing.T) {
	path := kiro.DefaultTokenPath()
	if path == "" {
		t.Error("DefaultTokenPath should not return empty string")
	}
	// Should end with .kiro/auth.json or be a fallback path
	if path != "./kiro-token.json" && len(path) < 10 {
		t.Errorf("Unexpected default token path: %s", path)
	}
}
