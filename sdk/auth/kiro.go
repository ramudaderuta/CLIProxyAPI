package auth

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	log "github.com/sirupsen/logrus"
)

// KiroAuthenticator implements token-based authentication for Kiro AI services.
// Unlike other providers, Kiro does not support online OAuth flow and requires
// importing a pre-existing kiro-auth-token.json file.
type KiroAuthenticator struct{}

// NewKiroAuthenticator constructs a Kiro authenticator.
func NewKiroAuthenticator() *KiroAuthenticator {
	return &KiroAuthenticator{}
}

// Provider returns the provider identifier for Kiro.
func (a *KiroAuthenticator) Provider() string {
	return "kiro"
}

// RefreshLead returns the refresh lead time for Kiro tokens.
// Kiro tokens are refreshed 5 minutes before expiration.
func (a *KiroAuthenticator) RefreshLead() *time.Duration {
	d := 5 * time.Minute
	return &d
}

// Login handles Kiro authentication by importing a token file.
// Since Kiro doesn't support online OAuth, this method prompts the user
// to provide the path to their kiro-auth-token.json file.
func (a *KiroAuthenticator) Login(ctx context.Context, cfg *config.Config, opts *LoginOptions) (*coreauth.Auth, error) {
	if cfg == nil {
		return nil, fmt.Errorf("cliproxy auth: configuration is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if opts == nil {
		opts = &LoginOptions{}
	}

	// Determine the token file path
	var tokenFilePath string
	if opts.Metadata != nil && opts.Metadata["token_file"] != "" {
		tokenFilePath = opts.Metadata["token_file"]
	} else if opts.Prompt != nil {
		// Prompt user for token file path
		path, err := opts.Prompt("Enter path to kiro-auth-token.json file: ")
		if err != nil {
			return nil, fmt.Errorf("failed to get token file path: %w", err)
		}
		tokenFilePath = path
	} else {
		// Default to looking for kiro-auth-token.json in the current directory or auth directory
		defaultPaths := []string{
			"kiro-auth-token.json",
			filepath.Join(cfg.AuthDir, "kiro-auth-token.json"),
		}

		for _, path := range defaultPaths {
			if _, err := os.Stat(path); err == nil {
				tokenFilePath = path
				break
			}
		}

		if tokenFilePath == "" {
			return nil, fmt.Errorf("kiro-auth-token.json file not found. Please provide the path to your Kiro authentication token file")
		}
	}

	// Load the token file
	tokenStorage, err := kiro.LoadTokenFromFile(tokenFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load Kiro token file %s: %w", tokenFilePath, err)
	}

	// Validate the token
	kiroAuth := kiro.NewKiroAuth()
	if !kiroAuth.ValidateToken(tokenStorage) {
		return nil, fmt.Errorf("invalid or expired Kiro token in %s", tokenFilePath)
	}

	// Generate a filename for storage
	fileName := "kiro-auth-token.json"
	if tokenStorage.ProfileArn != "" {
		// Extract a meaningful name from the profile ARN if available
		if idx := filepath.Base(tokenStorage.ProfileArn); idx != "" {
			fileName = fmt.Sprintf("kiro-%s.json", idx)
		}
	}

	metadata := map[string]any{
		"profileArn": tokenStorage.ProfileArn,
		"authMethod": tokenStorage.AuthMethod,
		"provider":   tokenStorage.Provider,
	}

	log.Infof("Kiro authentication successful with profile: %s", tokenStorage.ProfileArn)
	fmt.Println("Kiro authentication successful")

	return &coreauth.Auth{
		ID:       fileName,
		Provider: a.Provider(),
		FileName: fileName,
		Storage:  tokenStorage,
		Metadata: metadata,
	}, nil
}