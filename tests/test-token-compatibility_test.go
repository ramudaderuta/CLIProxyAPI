package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
)

func TestActualTokenFiles_NativeTokenCompatibility(t *testing.T) {
	// Test the native token file mentioned in the task
	nativeTokenPath := "/home/build/code/CLIProxyAPI/tmp/kiro-test/kiro-auth-token.json"

	// Verify the file exists
	if _, err := os.Stat(nativeTokenPath); os.IsNotExist(err) {
		t.Skipf("Native token file not found at %s", nativeTokenPath)
	}

	// Load the token
	token, err := kiro.LoadTokenFromFile(nativeTokenPath)
	if err != nil {
		t.Fatalf("Failed to load native token: %v", err)
	}

	// Verify token properties
	if token.ProfileArn == "" {
		t.Fatalf("Expected ProfileArn to be set")
	}
	if token.AccessToken == "" {
		t.Fatalf("Expected AccessToken to be set")
	}
	if token.RefreshToken == "" {
		t.Fatalf("Expected RefreshToken to be set")
	}
	if token.AuthMethod == "" {
		t.Fatalf("Expected AuthMethod to be set")
	}

	// Verify the token type is automatically enhanced
	if token.Type != "kiro" {
		t.Fatalf("Expected token type to be 'kiro', got '%s'", token.Type)
	}

	t.Logf("Native token loaded successfully: ProfileARN=%s, Type=%s, AuthMethod=%s",
		token.ProfileArn, token.Type, token.AuthMethod)
}

func TestActualTokenFiles_EnhancedTokenCompatibility(t *testing.T) {
	// Test the enhanced token file mentioned in the task
	enhancedTokenPath := "/home/build/.cli-proxy-api/kiro-auth-token.json"

	// Verify the file exists
	if _, err := os.Stat(enhancedTokenPath); os.IsNotExist(err) {
		t.Skipf("Enhanced token file not found at %s", enhancedTokenPath)
	}

	// Load the token
	token, err := kiro.LoadTokenFromFile(enhancedTokenPath)
	if err != nil {
		t.Fatalf("Failed to load enhanced token: %v", err)
	}

	// Verify token properties
	if token.ProfileArn == "" {
		t.Fatalf("Expected ProfileArn to be set")
	}
	if token.AccessToken == "" {
		t.Fatalf("Expected AccessToken to be set")
	}
	if token.RefreshToken == "" {
		t.Fatalf("Expected RefreshToken to be set")
	}
	if token.AuthMethod == "" {
		t.Fatalf("Expected AuthMethod to be set")
	}

	// Verify the token type is preserved
	if token.Type != "kiro" {
		t.Fatalf("Expected token type to be 'kiro', got '%s'", token.Type)
	}

	t.Logf("Enhanced token loaded successfully: ProfileARN=%s, Type=%s, AuthMethod=%s",
		token.ProfileArn, token.Type, token.AuthMethod)
}

func TestActualTokenFiles_ConfiguredTokenFile(t *testing.T) {
	// Test configuration with explicit token file path
	nativeTokenPath := "/home/build/code/CLIProxyAPI/tmp/kiro-test/kiro-auth-token.json"

	// Verify the file exists
	if _, err := os.Stat(nativeTokenPath); os.IsNotExist(err) {
		t.Skipf("Native token file not found at %s", nativeTokenPath)
	}

	// Create temporary config
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	configContent := `
port: 8317
auth-dir: "` + tempDir + `"
kiro-token-file:
  - token-file-path: "` + nativeTokenPath + `"
    region: "us-east-1"
    label: "test-native"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Load configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify configuration
	if len(cfg.KiroTokenFiles) != 1 {
		t.Fatalf("Expected 1 Kiro token file, got %d", len(cfg.KiroTokenFiles))
	}

	tokenFile := cfg.KiroTokenFiles[0]
	if tokenFile.TokenFilePath != nativeTokenPath {
		t.Fatalf("Expected token file path '%s', got '%s'", nativeTokenPath, tokenFile.TokenFilePath)
	}
	if tokenFile.Region != "us-east-1" {
		t.Fatalf("Expected region 'us-east-1', got '%s'", tokenFile.Region)
	}
	if tokenFile.Label != "test-native" {
		t.Fatalf("Expected label 'test-native', got '%s'", tokenFile.Label)
	}

	// Verify path resolution
	resolvedPath, err := tokenFile.ResolvePath(cfg.AuthDir)
	if err != nil {
		t.Fatalf("Failed to resolve path: %v", err)
	}
	if resolvedPath != nativeTokenPath {
		t.Fatalf("Expected resolved path '%s', got '%s'", nativeTokenPath, resolvedPath)
	}

	// Validate token files
	if err := cfg.ValidateKiroTokenFiles(); err != nil {
		t.Fatalf("Token file validation failed: %v", err)
	}

	t.Logf("Configured token file test passed: Path=%s, Region=%s, Label=%s",
		tokenFile.TokenFilePath, tokenFile.Region, tokenFile.Label)
}