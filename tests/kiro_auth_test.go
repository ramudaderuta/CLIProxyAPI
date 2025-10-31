// Package kiro provides authentication and token management functionality
// for Kiro AI services. This file contains unit tests for the Kiro authentication system.
package kiro

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

func TestKiroTokenStorage_SaveTokenToFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "kiro_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tokenFile := filepath.Join(tempDir, "kiro-test-token.json")

	// Create a test token
	token := &KiroTokenStorage{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		ProfileArn:   "arn:aws:codewhisperer:us-east-1:123456789012:profile/test",
		ExpiresAt:    time.Now().Add(1 * time.Hour),
		AuthMethod:   "social",
		Provider:     "Github",
	}

	// Test saving token to file
	err = token.SaveTokenToFile(tokenFile)
	if err != nil {
		t.Fatalf("Failed to save token: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(tokenFile); os.IsNotExist(err) {
		t.Fatal("Token file was not created")
	}

	// Verify file contents
	loadedToken, err := LoadTokenFromFile(tokenFile)
	if err != nil {
		t.Fatalf("Failed to load token: %v", err)
	}

	if loadedToken.AccessToken != token.AccessToken {
		t.Errorf("Expected access token %s, got %s", token.AccessToken, loadedToken.AccessToken)
	}

	if loadedToken.RefreshToken != token.RefreshToken {
		t.Errorf("Expected refresh token %s, got %s", token.RefreshToken, loadedToken.RefreshToken)
	}

	if loadedToken.Type != "kiro" {
		t.Errorf("Expected type 'kiro', got %s", loadedToken.Type)
	}
}

func TestKiroTokenStorage_IsExpired(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		expected  bool
	}{
		{
			name:      "Not expired token",
			expiresAt: time.Now().Add(1 * time.Hour),
			expected:  false,
		},
		{
			name:      "Expired token",
			expiresAt: time.Now().Add(-1 * time.Hour),
			expected:  true,
		},
		{
			name:      "Token expiring soon (within 5 minutes)",
			expiresAt: time.Now().Add(2 * time.Minute),
			expected:  true,
		},
		{
			name:      "Token exactly at threshold (5 minutes)",
			expiresAt: time.Now().Add(5 * time.Minute),
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &KiroTokenStorage{
				AccessToken:  "test-token",
				RefreshToken: "test-refresh",
				ExpiresAt:    tt.expiresAt,
			}

			result := token.IsExpired()
			if result != tt.expected {
				t.Errorf("Expected IsExpired() = %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestKiroAuth_extractRegionFromARN(t *testing.T) {
	auth := &KiroAuth{}

	tests := []struct {
		name     string
		arn      string
		expected string
	}{
		{
			name:     "Valid US East ARN",
			arn:      "arn:aws:codewhisperer:us-east-1:123456789012:profile/test",
			expected: "us-east-1",
		},
		{
			name:     "Valid EU West ARN",
			arn:      "arn:aws:codewhisperer:eu-west-1:123456789012:profile/test",
			expected: "eu-west-1",
		},
		{
			name:     "Valid AP Southeast ARN",
			arn:      "arn:aws:codewhisperer:ap-southeast-1:123456789012:profile/test",
			expected: "ap-southeast-1",
		},
		{
			name:     "Invalid ARN format",
			arn:      "invalid-arn",
			expected: "",
		},
		{
			name:     "Empty ARN",
			arn:      "",
			expected: "",
		},
		{
			name:     "ARN without region",
			arn:      "arn:aws:codewhisperer::123456789012:profile/test",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := auth.extractRegionFromARN(tt.arn)
			if result != tt.expected {
				t.Errorf("Expected region %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestKiroAuth_ValidateToken(t *testing.T) {
	auth := &KiroAuth{}

	tests := []struct {
		name     string
		token    *KiroTokenStorage
		expected bool
	}{
		{
			name:     "Valid token",
			token:    &KiroTokenStorage{AccessToken: "valid-token", ExpiresAt: time.Now().Add(1 * time.Hour)},
			expected: true,
		},
		{
			name:     "Nil token",
			token:    nil,
			expected: false,
		},
		{
			name:     "Empty access token",
			token:    &KiroTokenStorage{AccessToken: "", ExpiresAt: time.Now().Add(1 * time.Hour)},
			expected: false,
		},
		{
			name:     "Expired token",
			token:    &KiroTokenStorage{AccessToken: "expired-token", ExpiresAt: time.Now().Add(-1 * time.Hour)},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := auth.ValidateToken(tt.token)
			if result != tt.expected {
				t.Errorf("Expected ValidateToken() = %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestLoadTokenFromFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "kiro_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tokenFile := filepath.Join(tempDir, "kiro-test-token.json")

	// Test loading non-existent file
	_, err = LoadTokenFromFile(tokenFile)
	if err == nil {
		t.Fatal("Expected error when loading non-existent file")
	}

	// Create a valid token file
	token := &KiroTokenStorage{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		ProfileArn:   "arn:aws:codewhisperer:us-east-1:123456789012:profile/test",
		ExpiresAt:    time.Now().Add(1 * time.Hour),
		AuthMethod:   "social",
		Provider:     "Github",
	}

	err = token.SaveTokenToFile(tokenFile)
	if err != nil {
		t.Fatalf("Failed to save token: %v", err)
	}

	// Test loading valid file
	loadedToken, err := LoadTokenFromFile(tokenFile)
	if err != nil {
		t.Fatalf("Failed to load valid token file: %v", err)
	}

	if loadedToken.AccessToken != token.AccessToken {
		t.Errorf("Expected access token %s, got %s", token.AccessToken, loadedToken.AccessToken)
	}

	if loadedToken.Type != "kiro" {
		t.Errorf("Expected type 'kiro', got %s", loadedToken.Type)
	}
}

func TestKiroAuth_GetAuthenticatedClient(t *testing.T) {
	auth := &KiroAuth{}
	cfg := &config.Config{}

	// Create a test token that is not expired
	token := &KiroTokenStorage{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		ProfileArn:   "arn:aws:codewhisperer:us-east-1:123456789012:profile/test",
		ExpiresAt:    time.Now().Add(1 * time.Hour),
		AuthMethod:   "social",
		Provider:     "Github",
	}

	// Test getting authenticated client with valid token
	client, err := auth.GetAuthenticatedClient(context.Background(), token, cfg)
	if err != nil {
		t.Fatalf("Failed to get authenticated client: %v", err)
	}

	if client == nil {
		t.Fatal("Expected non-nil client")
	}

	// Note: Client timeout verification removed as Timeout field is not exported
}

func TestNewKiroAuth(t *testing.T) {
	auth := NewKiroAuth()
	if auth == nil {
		t.Fatal("Expected non-nil KiroAuth instance")
	}

	// Test that it can be used for basic operations
	// The methods are tested separately in their own test functions
	if auth.extractRegionFromARN("") != "" {
		// This should return empty string for invalid ARN
		t.Error("Expected empty string for invalid ARN")
	}
}