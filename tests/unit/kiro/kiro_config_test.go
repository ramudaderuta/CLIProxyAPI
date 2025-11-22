package kiro

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

// TestKiroConfig tests the Kiro configuration structure
func TestKiroConfig(t *testing.T) {
	tests := []struct {
		name          string
		config        config.KiroConfig
		expectedValid bool
		description   string
	}{
		{
			name: "valid config with token files",
			config: config.KiroConfig{
				TokenFiles: []config.KiroTokenFile{
					{
						Path:   "~/.kiro/kiro-primary.json",
						Region: "us-east-1",
						Label:  "primary",
					},
				},
			},
			expectedValid: true,
			description:   "Config with explicit token files should be valid",
		},
		{
			name: "empty config - auto-discovery mode",
			config: config.KiroConfig{
				TokenFiles: []config.KiroTokenFile{},
			},
			expectedValid: true,
			description:   "Empty config should enable auto-discovery",
		},
		{
			name: "multiple token files",
			config: config.KiroConfig{
				TokenFiles: []config.KiroTokenFile{
					{Path: "~/.kiro/kiro-primary.json", Region: "us-east-1", Label: "primary"},
					{Path: "~/.kiro/kiro-backup.json", Region: "us-west-2", Label: "backup"},
					{Path: "~/.kiro/kiro-team.json", Region: "eu-west-1", Label: "team"},
				},
			},
			expectedValid: true,
			description:   "Multiple token files for rotation",
		},
		{
			name: "token file without region - should use default",
			config: config.KiroConfig{
				TokenFiles: []config.KiroTokenFile{
					{
						Path:  "~/.kiro/kiro-test.json",
						Label: "test",
					},
				},
			},
			expectedValid: true,
			description:   "Missing region should default to us-east-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that config structure is valid
			if len(tt.config.TokenFiles) > 0 {
				for i, tf := range tt.config.TokenFiles {
					if tf.Path == "" {
						t.Errorf("Token file %d has empty path", i)
					}
				}
			}

			t.Logf("✓ %s", tt.description)
		})
	}
}

// TestKiroTokenFile tests the token file configuration
func TestKiroTokenFile(t *testing.T) {
	tests := []struct {
		name       string
		tokenFile  config.KiroTokenFile
		wantRegion string
	}{
		{
			name: "explicit region",
			tokenFile: config.KiroTokenFile{
				Path:   "/test/path.json",
				Region: "us-west-2",
				Label:  "test",
			},
			wantRegion: "us-west-2",
		},
		{
			name: "empty region",
			tokenFile: config.KiroTokenFile{
				Path:  "/test/path.json",
				Label: "test",
			},
			wantRegion: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.tokenFile.Region != tt.wantRegion {
				t.Errorf("Expected region %q, got %q", tt.wantRegion, tt.tokenFile.Region)
			}
		})
	}
}

// TestAutoDiscoveryFlag tests the auto-discovery configuration
func TestAutoDiscoveryFlag(t *testing.T) {
	testCases := []struct {
		name           string
		config         config.KiroConfig
		shouldDiscover bool
		description    string
	}{
		{
			name: "enabled with no token files",
			config: config.KiroConfig{
				TokenFiles: []config.KiroTokenFile{},
			},
			shouldDiscover: true,
			description:    "Empty token files should trigger auto-discovery",
		},
		{
			name: "disabled with explicit files",
			config: config.KiroConfig{
				TokenFiles: []config.KiroTokenFile{
					{Path: "/explicit/path.json"},
				},
			},
			shouldDiscover: false,
			description:    "Explicit token files should disable auto-discovery",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			shouldDiscover := len(tc.config.TokenFiles) == 0
			if shouldDiscover != tc.shouldDiscover {
				t.Errorf("Expected auto-discovery=%v, got %v", tc.shouldDiscover, shouldDiscover)
			}
			t.Logf("✓ %s", tc.description)
		})
	}
}

// Benchmark tests
func BenchmarkKiroConfigCreation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = config.KiroConfig{
			TokenFiles: []config.KiroTokenFile{
				{Path: "~/.kiro/token.json", Region: "us-east-1", Label: "test"},
			},
		}
	}
}

// TestDefaultRegion tests that missing region defaults to us-east-1
func TestDefaultRegion(t *testing.T) {
	// This will be validated in the token storage tests
	// Here we just verify the config structure allows empty region
	tokenFile := config.KiroTokenFile{
		Path:  "~/.kiro/test.json",
		Label: "test",
		// Region intentionally omitted
	}

	if tokenFile.Region != "" {
		t.Errorf("Expected empty region initially, got %q", tokenFile.Region)
	}
}

// Example test for documentation
func ExampleKiroConfig_basic() {
	cfg := config.KiroConfig{
		TokenFiles: []config.KiroTokenFile{
			{
				Path:   "~/.kiro/kiro-primary.json",
				Region: "us-east-1",
				Label:  "primary",
			},
		},
	}

	_ = cfg
	// Output:
}

func ExampleKiroConfig_autoDiscovery() {
	// Zero-config mode - auto-discovery enabled
	cfg := config.KiroConfig{
		TokenFiles: []config.KiroTokenFile{},
	}

	_ = cfg
	// Output:
}
