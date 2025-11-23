package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	log "github.com/sirupsen/logrus"
)

// DoKiroLogin triggers the Kiro OAuth device code flow.
// It initiates the OAuth authentication process for Kiro services
// and saves the authentication tokens to the configured token file.
//
// Parameters:
//   - cfg: The application configuration
//   - options: Login options including browser behavior and prompts (currently unused for device flow)
func DoKiroLogin(cfg *config.Config, options *LoginOptions) {
	if options == nil {
		options = &LoginOptions{}
	}

	fmt.Println("[DEBUG] DoKiroLogin: Function called")
	log.Info("Starting Kiro CLI authentication flow...")

	// Create authenticator
	fmt.Println("[DEBUG] DoKiroLogin: Creating authenticator")
	authenticator := kiro.NewKiroAuthenticator(cfg)

	// Start authentication flow
	fmt.Println("[DEBUG] DoKiroLogin: Starting authentication flow")
	ctx := context.Background()
	tokenStorage, deviceResp, err := authenticator.Authenticate(ctx)
	if err != nil {
		fmt.Printf("❌ Kiro authentication failed: %v\n", err)
		log.Errorf("Authentication failed: %v", err)
		return
	}

	// Display user code and verification URL during device flow
	if deviceResp != nil {
		fmt.Println("\n" + strings.Repeat("=", 60))
		fmt.Println("  Kiro CLI - Device Code Authentication")
		fmt.Println(strings.Repeat("=", 60))
		fmt.Printf("\n📱 User Code: %s\n", deviceResp.UserCode)
		fmt.Printf("🌐 Verification URL: %s\n", deviceResp.VerificationURI)
		if deviceResp.VerificationURIComplete != "" {
			fmt.Printf("\n🔗 Or visit (auto-fills code): %s\n", deviceResp.VerificationURIComplete)
		}
		fmt.Printf("\n⏱️  Expires in %d seconds\n", deviceResp.ExpiresIn)
		fmt.Println("\n" + strings.Repeat("=", 60))
		fmt.Println("\n⏳ Waiting for authorization...")

		// Now poll for the token after user sees the code
		fmt.Println("[DEBUG] DoKiroLogin: Calling PollForToken")
		tokenStorage, err = authenticator.PollForToken(ctx, deviceResp.DeviceCode)
		if err != nil {
			fmt.Printf("❌ Kiro authentication failed while polling: %v\n", err)
			log.Errorf("Token polling failed: %v", err)
			return
		}
		fmt.Println("[DEBUG] DoKiroLogin: PollForToken succeeded")
	}

	// Determine save path from config or use default
	// Determine save path
	authDir := cfg.AuthDir
	if authDir == "" {
		// Fallback to default directory if not configured
		home, err := os.UserHomeDir()
		if err != nil {
			log.Errorf("Failed to get user home directory: %v", err)
			return
		}
		authDir = filepath.Join(home, ".cli-proxy-api")
	}

	// Ensure directory exists
	if err := os.MkdirAll(authDir, 0700); err != nil {
		log.Errorf("Failed to create auth directory: %v", err)
		return
	}

	// Generate dynamic filename: kiro-BuilderId-<timestamp>.json
	filename := fmt.Sprintf("kiro-BuilderId-%d.json", time.Now().Unix())
	var savePath = filepath.Join(authDir, filename)

	// Save token to file
	if err := tokenStorage.SaveTokenToFile(savePath); err != nil {
		fmt.Printf("❌ Failed to save token: %v\n", err)
		log.Errorf("Failed to save token: %v", err)
		return
	}

	// Success message
	fmt.Println("\n✅ Authentication successful!")
	fmt.Printf("📁 Token saved to: %s\n", savePath)
	fmt.Printf("🔑 Profile ARN: %s\n", tokenStorage.ProfileArn)
	fmt.Printf("📍 Region: %s\n", tokenStorage.Region)
	fmt.Printf("🔐 Auth Method: %s (%s)\n", tokenStorage.AuthMethod, tokenStorage.Provider)
	fmt.Println("\nYou can now use Kiro models in your configuration!")
}
