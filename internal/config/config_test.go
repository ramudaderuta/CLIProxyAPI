package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
)

func TestNormalizeKiroTokenFiles(t *testing.T) {
	cfg := &Config{
		KiroTokenFiles: []KiroTokenFile{
			{TokenFilePath: " token-a.json ", Region: "us-east-1"},
			{TokenFilePath: "token-a.json", Region: "us-east-1"},
			{TokenFilePath: "token-b.json", Region: ""},
		},
	}

	cfg.NormalizeKiroTokenFiles()

	if len(cfg.KiroTokenFiles) != 2 {
		t.Fatalf("expected 2 normalized entries, got %d", len(cfg.KiroTokenFiles))
	}
	for _, entry := range cfg.KiroTokenFiles {
		if strings.TrimSpace(entry.TokenFilePath) == "" {
			t.Fatalf("unexpected empty token file path in normalized list: %+v", entry)
		}
		if entry.Region == "" {
			t.Fatalf("expected default region to be applied, entry=%+v", entry)
		}
	}
}

func TestValidateKiroTokenFiles_Success(t *testing.T) {
	tempDir := t.TempDir()
	tokenPath := filepath.Join(tempDir, "kiro.json")
	token := &kiro.KiroTokenStorage{
		AccessToken:  "access",
		RefreshToken: "refresh",
		ProfileArn:   "arn:aws:codewhisperer:us-east-1:123456789012:profile/test",
		ExpiresAt:    time.Now().Add(2 * time.Hour),
		AuthMethod:   "social",
		Provider:     "GitHub",
	}
	if err := token.SaveTokenToFile(tokenPath); err != nil {
		t.Fatalf("failed to write token: %v", err)
	}

	cfg := &Config{
		AuthDir: tempDir,
		KiroTokenFiles: []KiroTokenFile{
			{TokenFilePath: filepath.Base(tokenPath)},
		},
	}
	cfg.NormalizeKiroTokenFiles()
	if err := cfg.ValidateKiroTokenFiles(); err != nil {
		t.Fatalf("validate success: %v", err)
	}
}

func TestValidateKiroTokenFiles_MissingFile(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &Config{
		AuthDir: tempDir,
		KiroTokenFiles: []KiroTokenFile{
			{TokenFilePath: "missing.json"},
		},
	}
	cfg.NormalizeKiroTokenFiles()
	err := cfg.ValidateKiroTokenFiles()
	if err == nil {
		t.Fatalf("expected error for missing token file")
	}
}

func TestValidateKiroTokenFiles_InvalidToken(t *testing.T) {
	tempDir := t.TempDir()
	tokenPath := filepath.Join(tempDir, "invalid.json")
	if err := os.WriteFile(tokenPath, []byte(`{"accessToken":"only-access"}`), 0o600); err != nil {
		t.Fatalf("failed to write token: %v", err)
	}

	cfg := &Config{
		AuthDir: tempDir,
		KiroTokenFiles: []KiroTokenFile{
			{TokenFilePath: filepath.Base(tokenPath)},
		},
	}
	cfg.NormalizeKiroTokenFiles()
	err := cfg.ValidateKiroTokenFiles()
	if err == nil || !strings.Contains(err.Error(), "refreshToken") {
		t.Fatalf("expected refreshToken error, got %v", err)
	}
}

func TestValidateKiroTokenFiles_NativeTokenEnhancement(t *testing.T) {
	tempDir := t.TempDir()
	expires := time.Now().Add(90 * time.Minute).Format(time.RFC3339)
	tokenPath := filepath.Join(tempDir, "native.json")
	content := `{
		"accessToken": "abc",
		"refreshToken": "def",
		"profileArn": "arn:aws:codewhisperer:us-east-1:123456789012:profile/native",
		"expiresAt": "` + expires + `",
		"authMethod": "social"
	}`
	if err := os.WriteFile(tokenPath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write token: %v", err)
	}

	cfg := &Config{
		AuthDir: tempDir,
		KiroTokenFiles: []KiroTokenFile{
			{TokenFilePath: filepath.Base(tokenPath)},
		},
	}
	cfg.NormalizeKiroTokenFiles()
	if err := cfg.ValidateKiroTokenFiles(); err != nil {
		t.Fatalf("validate native token: %v", err)
	}

	token, err := kiro.LoadTokenFromFile(tokenPath)
	if err != nil {
		t.Fatalf("load token: %v", err)
	}
	if token.Type != "kiro" {
		t.Fatalf("expected token type 'kiro', got %q", token.Type)
	}
}
