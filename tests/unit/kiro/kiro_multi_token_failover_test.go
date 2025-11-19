package kiro_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/runtime/executor"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	"github.com/stretchr/testify/require"
)

func TestKiroExecutor_ConfiguredTokens_RoundRobinWithFailover(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	tokenA := writeRotatingTokenFile(t, tempDir, "token-a.json", "token-A")
	tokenB := writeRotatingTokenFile(t, tempDir, "token-b.json", "token-B")
	tokenC := writeRotatingTokenFile(t, tempDir, "token-c.json", "token-C")

	cfg := &config.Config{
		AuthDir: tempDir,
		KiroTokenFiles: []config.KiroTokenFile{
			{TokenFilePath: tokenA},
			{TokenFilePath: tokenB},
			{TokenFilePath: tokenC},
		},
	}
	cfg.NormalizeKiroTokenFiles()
	require.NoError(t, cfg.ValidateKiroTokenFiles())

	exec := executor.NewKiroExecutor(cfg)
	auth := &cliproxyauth.Auth{
		ID:         "multi-token",
		Provider:   "kiro",
		Metadata:   map[string]any{},
		Attributes: map[string]string{},
	}

	ctx := context.Background()

	refreshed, err := exec.Refresh(ctx, auth)
	require.NoError(t, err)
	require.Equal(t, "token-A", refreshed.Metadata["accessToken"], "first refresh should use the first configured token")

	require.NoError(t, os.Remove(tokenB))

	refreshed, err = exec.Refresh(ctx, refreshed)
	require.NoError(t, err)
	require.Equal(t, "token-C", refreshed.Metadata["accessToken"], "second refresh should skip the missing token and advance to the next entry")

	refreshed, err = exec.Refresh(ctx, refreshed)
	require.NoError(t, err)
	require.Equal(t, "token-A", refreshed.Metadata["accessToken"], "round-robin should wrap back to the surviving token after failover")
}

func writeRotatingTokenFile(t *testing.T, dir, name, accessToken string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	token := map[string]any{
		"accessToken":  accessToken,
		"refreshToken": accessToken + "-refresh",
		"profileArn":   "arn:aws:codewhisperer:us-east-1:123456789012:profile/rotate",
		"expiresAt":    time.Now().Add(30 * time.Minute).UTC().Format(time.RFC3339),
		"authMethod":   "social",
		"provider":     "Github",
		"type":         "kiro",
	}

	data, err := json.Marshal(token)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(path, data, 0600))
	return path
}
