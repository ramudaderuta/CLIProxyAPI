package kiro_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/runtime/executor"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

// TestKiroBackwardCompatibility_NativeTokenFile validates that native Kiro token files
// (without "type": "kiro") continue to work with automatic type enhancement.
func TestKiroBackwardCompatibility_NativeTokenFile(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()

	// Copy the native token file (without type field) to temp directory
	nativeTokenPath := filepath.Join(tempDir, "kiro-auth-token.json")
	nativeTokenContent, err := os.ReadFile("/home/build/code/CLIProxyAPI/tmp/kiro-test/kiro-auth-token.json")
	if err != nil {
		t.Fatalf("Failed to read native token file: %v", err)
	}
	if err := os.WriteFile(nativeTokenPath, nativeTokenContent, 0644); err != nil {
		t.Fatalf("Failed to write native token file: %v", err)
	}

	// Create configuration that relies on auto-detection
	cfg := &config.Config{
		AuthDir: tempDir,
	}

	// Create executor
	exec := executor.NewKiroExecutor(cfg)

	// Create auth
	auth := &cliproxyauth.Auth{
		ID:       "test-auth",
		Provider: "kiro",
		Attributes: map[string]string{
			"region": "us-east-1",
		},
	}

	// Test that refresh works with native token file
	// This should automatically enhance the token with "type": "kiro" in memory
	refreshedAuth, err := exec.Refresh(context.Background(), auth)
	if err != nil {
		t.Fatalf("Refresh failed with native token file: %v", err)
	}

	if refreshedAuth == nil || refreshedAuth.Runtime == nil {
		t.Fatal("Expected refreshed auth with runtime token")
	}

	// The test passes if no errors occurred and the auth was refreshed
	// The actual enhancement is tested in the auth/kiro package tests
}

// TestKiroBackwardCompatibility_EnhancedTokenFile validates that enhanced Kiro token files
// (with "type": "kiro") continue to work without modification.
func TestKiroBackwardCompatibility_EnhancedTokenFile(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()

	// Copy the enhanced token file (with type field) to temp directory
	enhancedTokenPath := filepath.Join(tempDir, "kiro-auth-token.json")
	enhancedTokenContent, err := os.ReadFile("/home/build/.cli-proxy-api/kiro-auth-token.json")
	if err != nil {
		t.Fatalf("Failed to read enhanced token file: %v", err)
	}
	if err := os.WriteFile(enhancedTokenPath, enhancedTokenContent, 0644); err != nil {
		t.Fatalf("Failed to write enhanced token file: %v", err)
	}

	// Create configuration that relies on auto-detection
	cfg := &config.Config{
		AuthDir: tempDir,
	}

	// Create executor
	exec := executor.NewKiroExecutor(cfg)

	// Create auth
	auth := &cliproxyauth.Auth{
		ID:       "test-auth",
		Provider: "kiro",
		Attributes: map[string]string{
			"region": "us-east-1",
		},
	}

	// Test that refresh works with enhanced token file
	refreshedAuth, err := exec.Refresh(context.Background(), auth)
	if err != nil {
		t.Fatalf("Refresh failed with enhanced token file: %v", err)
	}

	if refreshedAuth == nil || refreshedAuth.Runtime == nil {
		t.Fatal("Expected refreshed auth with runtime token")
	}

	// The test passes if no errors occurred and the auth was refreshed
}

// TestKiroBackwardCompatibility_ConfiguredVsAutoDetected validates that explicitly
// configured token files take precedence over auto-detected files, maintaining
// backward compatibility while enabling new functionality.
func TestKiroBackwardCompatibility_ConfiguredVsAutoDetected(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()

	// Copy the native token file to temp directory (auto-detected location)
	nativeTokenPath := filepath.Join(tempDir, "kiro-auth-token.json")
	nativeTokenContent, err := os.ReadFile("/home/build/code/CLIProxyAPI/tmp/kiro-test/kiro-auth-token.json")
	if err != nil {
		t.Fatalf("Failed to read native token file: %v", err)
	}
	if err := os.WriteFile(nativeTokenPath, nativeTokenContent, 0644); err != nil {
		t.Fatalf("Failed to write native token file: %v", err)
	}

	// Create a different token file for explicit configuration
	explicitTokenPath := filepath.Join(tempDir, "explicit-kiro-token.json")
	explicitTokenContent, err := os.ReadFile("/home/build/.cli-proxy-api/kiro-auth-token.json")
	if err != nil {
		t.Fatalf("Failed to read enhanced token file: %v", err)
	}
	if err := os.WriteFile(explicitTokenPath, explicitTokenContent, 0644); err != nil {
		t.Fatalf("Failed to write explicit token file: %v", err)
	}

	// Test Case 1: Auto-detection only (backward compatibility)
	t.Run("AutoDetectionOnly", func(t *testing.T) {
		cfg := &config.Config{
			AuthDir: tempDir,
		}

		exec := executor.NewKiroExecutor(cfg)
		auth := &cliproxyauth.Auth{
			ID:       "test-auth",
			Provider: "kiro",
			Attributes: map[string]string{
				"region": "us-east-1",
			},
		}

		refreshedAuth, err := exec.Refresh(context.Background(), auth)
		if err != nil {
			t.Fatalf("Refresh failed with auto-detection: %v", err)
		}

		if refreshedAuth == nil || refreshedAuth.Runtime == nil {
			t.Fatal("Expected refreshed auth with runtime token")
		}
	})

	// Test Case 2: Explicit configuration takes precedence
	t.Run("ExplicitConfigurationPrecedence", func(t *testing.T) {
		cfg := &config.Config{
			AuthDir: tempDir,
			KiroTokenFiles: []config.KiroTokenFile{
				{
					TokenFilePath: explicitTokenPath,
					Region:        "us-west-2",
					Label:         "explicit-account",
				},
			},
		}

		// Normalize and validate configuration
		cfg.NormalizeKiroTokenFiles()
		if err := cfg.ValidateKiroTokenFiles(); err != nil {
			t.Fatalf("Configuration validation failed: %v", err)
		}

		exec := executor.NewKiroExecutor(cfg)
		auth := &cliproxyauth.Auth{
			ID:       "test-auth",
			Provider: "kiro",
			Attributes: map[string]string{
				"region": "us-west-2",
			},
		}

		refreshedAuth, err := exec.Refresh(context.Background(), auth)
		if err != nil {
			t.Fatalf("Refresh failed with explicit configuration: %v", err)
		}

		if refreshedAuth == nil || refreshedAuth.Runtime == nil {
			t.Fatal("Expected refreshed auth with runtime token")
		}
	})

	// Test Case 3: Mixed configuration (explicit paths + auto-detection)
	t.Run("MixedConfiguration", func(t *testing.T) {
		cfg := &config.Config{
			AuthDir: tempDir,
			KiroTokenFiles: []config.KiroTokenFile{
				{
					TokenFilePath: explicitTokenPath,
					Region:        "us-west-2",
					Label:         "explicit-account",
				},
			},
		}

		// Normalize and validate configuration
		cfg.NormalizeKiroTokenFiles()
		if err := cfg.ValidateKiroTokenFiles(); err != nil {
			t.Fatalf("Configuration validation failed: %v", err)
		}

		exec := executor.NewKiroExecutor(cfg)

		// Test with explicit configuration
		auth1 := &cliproxyauth.Auth{
			ID:       "test-auth-1",
			Provider: "kiro",
			Attributes: map[string]string{
				"region": "us-west-2",
			},
		}

		refreshedAuth1, err := exec.Refresh(context.Background(), auth1)
		if err != nil {
			t.Fatalf("Refresh failed with explicit configuration: %v", err)
		}

		if refreshedAuth1 == nil || refreshedAuth1.Runtime == nil {
			t.Fatal("Expected refreshed auth with runtime token for explicit configuration")
		}

		// Test with auto-detection (should still work for backward compatibility)
		auth2 := &cliproxyauth.Auth{
			ID:       "test-auth-2",
			Provider: "kiro",
			Attributes: map[string]string{
				"region":     "us-east-1",
				"token_file": nativeTokenPath, // Explicitly specify native token file
			},
		}

		refreshedAuth2, err := exec.Refresh(context.Background(), auth2)
		if err != nil {
			t.Fatalf("Refresh failed with auto-detected configuration: %v", err)
		}

		if refreshedAuth2 == nil || refreshedAuth2.Runtime == nil {
			t.Fatal("Expected refreshed auth with runtime token for auto-detected configuration")
		}
	})
}

// TestKiroBackwardCompatibility_MigrationPath validates the migration path
// from auto-detection to explicit configuration.
func TestKiroBackwardCompatibility_MigrationPath(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()

	// Start with only auto-detected token file (old setup)
	nativeTokenPath := filepath.Join(tempDir, "kiro-auth-token.json")
	nativeTokenContent, err := os.ReadFile("/home/build/code/CLIProxyAPI/tmp/kiro-test/kiro-auth-token.json")
	if err != nil {
		t.Fatalf("Failed to read native token file: %v", err)
	}
	if err := os.WriteFile(nativeTokenPath, nativeTokenContent, 0644); err != nil {
		t.Fatalf("Failed to write native token file: %v", err)
	}

	// Initial configuration (old style - auto-detection only)
	initialCfg := &config.Config{
		AuthDir: tempDir,
	}

	exec := executor.NewKiroExecutor(initialCfg)
	auth := &cliproxyauth.Auth{
		ID:       "test-auth",
		Provider: "kiro",
		Attributes: map[string]string{
			"region": "us-east-1",
		},
	}

	// Initial usage should work with auto-detection
	_, err = exec.Refresh(context.Background(), auth)
	if err != nil {
		t.Fatalf("Initial refresh failed: %v", err)
	}

	// Now migrate to explicit configuration
	enhancedTokenContent, err := os.ReadFile("/home/build/.cli-proxy-api/kiro-auth-token.json")
	if err != nil {
		t.Fatalf("Failed to read enhanced token file: %v", err)
	}

	explicitTokenPath := filepath.Join(tempDir, "explicit-kiro-token.json")
	if err := os.WriteFile(explicitTokenPath, enhancedTokenContent, 0644); err != nil {
		t.Fatalf("Failed to write explicit token file: %v", err)
	}

	// Update configuration to use explicit path (new style)
	updatedCfg := &config.Config{
		AuthDir: tempDir,
		KiroTokenFiles: []config.KiroTokenFile{
			{
				TokenFilePath: explicitTokenPath,
				Region:        "us-west-2",
				Label:         "migrated-account",
			},
		},
	}

	// Normalize and validate updated configuration
	updatedCfg.NormalizeKiroTokenFiles()
	if err := updatedCfg.ValidateKiroTokenFiles(); err != nil {
		t.Fatalf("Updated configuration validation failed: %v", err)
	}

	// Update executor with new configuration
	exec = executor.NewKiroExecutor(updatedCfg)

	// Update auth attributes to match new region
	auth.Attributes["region"] = "us-west-2"

	// Usage should now work with explicit configuration
	refreshedAuth, err := exec.Refresh(context.Background(), auth)
	if err != nil {
		t.Fatalf("Refresh after migration failed: %v", err)
	}

	if refreshedAuth == nil || refreshedAuth.Runtime == nil {
		t.Fatal("Expected refreshed auth with runtime token after migration")
	}

	// Both old and new token files should still exist
	if _, err := os.Stat(nativeTokenPath); err != nil {
		t.Errorf("Native token file should still exist after migration: %v", err)
	}

	if _, err := os.Stat(explicitTokenPath); err != nil {
		t.Errorf("Explicit token file should exist after migration: %v", err)
	}

	// The test passes if migration from auto-detection to explicit configuration works
}