package cmd

import (
	"context"
	"fmt"
	"strings"

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
	var savePath string
	if cfg != nil && len(cfg.KiroConfig.TokenFiles) > 0 {
		savePath = cfg.KiroConfig.TokenFiles[0].Path
	}
	if savePath == "" {
		savePath = kiro.DefaultTokenPath()
	}

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
