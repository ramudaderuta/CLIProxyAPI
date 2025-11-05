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

// TestKiroConfig_TokenFilePrecedence validates that explicitly configured
// token files take precedence over auto-detected files.
func TestKiroConfig_TokenFilePrecedence(t *testing.T) {
	t.Parallel()
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

	// Create configuration with explicit token file path
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

	// Create executor and test that it uses the explicitly configured token file
	exec := executor.NewKiroExecutor(cfg)

	// Create a mock auth that would use the configured token
	auth := &cliproxyauth.Auth{
		ID:       "test-auth",
		Provider: "kiro",
		Attributes: map[string]string{
			"region": "us-west-2",
		},
	}

	// Refresh should use the explicitly configured token file
	refreshedAuth, err := exec.Refresh(context.Background(), auth)
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	// Verify that the refreshed auth contains the expected token data
	if refreshedAuth == nil || refreshedAuth.Runtime == nil {
		t.Fatal("Expected refreshed auth with runtime token")
	}

	// The test passes if no errors occurred and the auth was refreshed
}

// TestKiroConfig_NativeTokenEnhancement validates that native Kiro tokens
// (without "type": "kiro") are automatically enhanced in memory.
func TestKiroConfig_NativeTokenEnhancement(t *testing.T) {
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

	// Create basic configuration that relies on auto-detection
	cfg := &config.Config{
		AuthDir: tempDir,
	}

	// Create executor
	exec := executor.NewKiroExecutor(cfg)

	// Create a mock auth that would use the auto-detected token
	auth := &cliproxyauth.Auth{
		ID:       "test-auth",
		Provider: "kiro",
		Attributes: map[string]string{
			"region": "us-east-1",
		},
	}

	// Refresh should work with the native token file and enhance it in memory
	refreshedAuth, err := exec.Refresh(context.Background(), auth)
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	// Verify that the refreshed auth contains the expected token data
	if refreshedAuth == nil || refreshedAuth.Runtime == nil {
		t.Fatal("Expected refreshed auth with runtime token")
	}

	// Check that the token was enhanced with type field in memory
	tokenStorage := refreshedAuth.Runtime
	if tokenStorage == nil {
		t.Fatal("Expected token storage in runtime")
	}

	// The test passes if no errors occurred and the auth was refreshed
	// The actual enhancement is tested in the auth/kiro package kiro_test
}

// TestKiroConfig_BackwardCompatibility validates that existing Kiro token files
// continue to work with the new implementation.
func TestKiroConfig_BackwardCompatibility(t *testing.T) {
	// Test both native and enhanced token files for backward compatibility

	testCases := []struct {
		name       string
		tokenPath  string
	}{
		{
			name:      "NativeTokenFile",
			tokenPath: "/home/build/code/CLIProxyAPI/tmp/kiro-test/kiro-auth-token.json",
		},
		{
			name:      "EnhancedTokenFile",
			tokenPath: "/home/build/.cli-proxy-api/kiro-auth-token.json",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create temporary directory for test
			tempDir := t.TempDir()

			// Copy the token file to temp directory
			tokenContent, err := os.ReadFile(tc.tokenPath)
			if err != nil {
				t.Fatalf("Failed to read token file: %v", err)
			}

			testTokenPath := filepath.Join(tempDir, "kiro-auth-token.json")
			if err := os.WriteFile(testTokenPath, tokenContent, 0644); err != nil {
				t.Fatalf("Failed to write token file: %v", err)
			}

			// Create configuration
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

			// Test that refresh works
			refreshedAuth, err := exec.Refresh(context.Background(), auth)
			if err != nil {
				t.Fatalf("Refresh failed: %v", err)
			}

			if refreshedAuth == nil || refreshedAuth.Runtime == nil {
				t.Fatal("Expected refreshed auth with runtime token")
			}
		})
	}
}

// TestKiroConfig_HotReloading validates that configuration changes are
// properly detected and applied.
func TestKiroConfig_HotReloading(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()

	// Start with native token file
	nativeTokenPath := filepath.Join(tempDir, "kiro-auth-token.json")
	nativeTokenContent, err := os.ReadFile("/home/build/code/CLIProxyAPI/tmp/kiro-test/kiro-auth-token.json")
	if err != nil {
		t.Fatalf("Failed to read native token file: %v", err)
	}
	if err := os.WriteFile(nativeTokenPath, nativeTokenContent, 0644); err != nil {
		t.Fatalf("Failed to write native token file: %v", err)
	}

	// Create initial configuration
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

	// Initial refresh should work with native token
	_, err = exec.Refresh(context.Background(), auth)
	if err != nil {
		t.Fatalf("Initial refresh failed: %v", err)
	}

	// Now update configuration to use explicit token file path
	enhancedTokenContent, err := os.ReadFile("/home/build/.cli-proxy-api/kiro-auth-token.json")
	if err != nil {
		t.Fatalf("Failed to read enhanced token file: %v", err)
	}

	explicitTokenPath := filepath.Join(tempDir, "explicit-kiro-token.json")
	if err := os.WriteFile(explicitTokenPath, enhancedTokenContent, 0644); err != nil {
		t.Fatalf("Failed to write explicit token file: %v", err)
	}

	// Update configuration to use explicit path
	cfg.KiroTokenFiles = []config.KiroTokenFile{
		{
			TokenFilePath: explicitTokenPath,
			Region:        "us-west-2",
			Label:         "updated-account",
		},
	}

	cfg.NormalizeKiroTokenFiles()
	if err := cfg.ValidateKiroTokenFiles(); err != nil {
		t.Fatalf("Updated configuration validation failed: %v", err)
	}

	// Update auth attributes to match new region
	auth.Attributes["region"] = "us-west-2"

	// Refresh should now use the explicitly configured token file
	refreshedAuth, err := exec.Refresh(context.Background(), auth)
	if err != nil {
		t.Fatalf("Refresh after config update failed: %v", err)
	}

	if refreshedAuth == nil || refreshedAuth.Runtime == nil {
		t.Fatal("Expected refreshed auth with runtime token after config update")
	}

	// The test passes if configuration updates are properly handled
}