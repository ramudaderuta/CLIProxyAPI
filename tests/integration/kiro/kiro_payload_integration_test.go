package kiro_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"gopkg.in/yaml.v3"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
)

// TestKiroPayloads performs an end-to-end integration test for Kiro.
// It checks if Kiro tokens exist, starts the server, and runs payload tests.
func TestKiroPayloads(t *testing.T) {
	const modelName = "claude-sonnet-4-5" // must exist in registry.GetKiroModels
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 1. Locate and parse config.yaml
	rootDir, err := findProjectRoot()
	require.NoError(t, err, "Failed to find project root")
	configPath := filepath.Join(rootDir, "config.yaml")

	cfg, err := config.LoadConfig(configPath)
	require.NoError(t, err, "Failed to load config.yaml")

	// 2. Check for Kiro tokens
	authDir := expandPath(cfg.AuthDir)
	if !hasKiroTokens(authDir) {
		t.Skipf("No Kiro tokens found in %s. Skipping Kiro integration tests.", authDir)
	}

	// 3. Start the server
	_, port, cleanup := startServer(t, rootDir)
	defer cleanup()

	// Wait for server to be ready
	baseURL := fmt.Sprintf("http://localhost:%d", port)
	require.NoError(t, waitForServer(baseURL), "Server failed to start")

	// Ensure Kiro models are registered in the global registry so provider lookup succeeds
	registry.GetGlobalRegistry().RegisterClient("integration-kiro", "kiro", registry.GetKiroModels())

	t.Logf("Server started at %s", baseURL)

	// 4. Run Payload Tests
	apiKey := "test-api-key" // Matches config.yaml default
	testDataDir := filepath.Join(rootDir, "tests/testdata")

	t.Run("OpenAI Chat Completions", func(t *testing.T) {
		files := []string{
			"openai_format_simple",
			"openai_format",
			"openai_format_with_tools",
		}

		for _, file := range files {
			t.Run(file, func(t *testing.T) {
				payload := loadAndModifyPayload(t, testDataDir, file, modelName)
				resp, code := sendRequest(t, baseURL+"/v1/chat/completions", apiKey, payload)

				assert.Equal(t, 200, code, "Expected HTTP 200. Body: %s", resp)
				assert.True(t, gjson.Get(resp, "id").Exists(), "Missing 'id'. Body: %s", resp)
				assert.True(t, gjson.Get(resp, "choices").Exists(), "Missing 'choices'. Body: %s", resp)
				assert.True(t, gjson.Get(resp, "choices.0.message").Exists(), "Missing 'message'. Body: %s", resp)
			})
		}
	})

	t.Run("Anthropic Messages", func(t *testing.T) {
		files := []string{
			"claude_format_with_tools",
			"claude_format_tool_call_no_result",
			"claude_format_simple",
		}

		for _, file := range files {
			t.Run(file, func(t *testing.T) {
				payload := loadAndModifyPayload(t, testDataDir, file, modelName)
				headers := map[string]string{"anthropic-version": "2023-06-01"}
				resp, code := sendRequestWithHeaders(t, baseURL+"/v1/messages", apiKey, payload, headers)

				assert.Equal(t, 200, code, "Expected HTTP 200. Body: %s", resp)
				assert.True(t, gjson.Get(resp, "id").Exists(), "Missing 'id'. Body: %s", resp)
				assert.True(t, gjson.Get(resp, "content").Exists(), "Missing 'content'. Body: %s", resp)
				assert.True(t, gjson.Get(resp, "role").Exists(), "Missing 'role'. Body: %s", resp)
			})
		}
	})
}

// Helpers

func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

func hasKiroTokens(authDir string) bool {
	entries, err := os.ReadDir(authDir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.Contains(entry.Name(), "kiro") { // Simplified check
			return true
		}
	}
	// Also check for any json files that might contain kiro tokens if naming convention differs
	// But for now, let's assume if the directory is not empty, we might have tokens.
	// Actually, let's look for *token* files.
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			return true
		}
	}
	return false
}

func startServer(t *testing.T, rootDir string) (*exec.Cmd, int, func()) {
	// Build the server first to ensure we run the latest code
	buildCmd := exec.Command("go", "build", "-o", "cli-proxy-api-test", "./cmd/server")
	buildCmd.Dir = rootDir
	out, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build server: %s", string(out))

	serverBin := filepath.Join(rootDir, "cli-proxy-api-test")
	port := 8318 // Use a different port for testing to avoid conflict
	tempAuthDir := t.TempDir()

	// Create a temporary config for the test server
	// We need to copy the real config but change the port
	// For simplicity, we'll pass the port via env var if supported, or just rely on config.
	// Since config.yaml is hardcoded to 8317, we might conflict if main server is running.
	// Let's try to run with the existing config and hope 8317 is free, or use a temp config.
	// Better: Create a temp config file.

	origConfigPath := filepath.Join(rootDir, "config.yaml")
	cfg, err := config.LoadConfig(origConfigPath)
	require.NoError(t, err)

	// Ensure the test server binds to the expected port regardless of existing config values.
	cfg.Port = port
	// Enable verbose server logging during integration runs for easier debugging.
	cfg.Debug = true
	// Copy kiro token into an isolated temp auth dir and inject missing type field to satisfy watcher.
	if authDir := strings.TrimSpace(cfg.AuthDir); authDir != "" {
		sourcePath := filepath.Join(expandPath(authDir), "kiro-auth-token.json")
		tokenBytes, readErr := os.ReadFile(sourcePath)
		require.NoError(t, readErr, "failed to read kiro token file: %s", sourcePath)
		var token map[string]any
		require.NoError(t, json.Unmarshal(tokenBytes, &token))
		token["type"] = "kiro"
		destPath := filepath.Join(tempAuthDir, "kiro-auth-token.json")
		patched, marshalErr := json.Marshal(token)
		require.NoError(t, marshalErr)
		require.NoError(t, os.WriteFile(destPath, patched, 0o600))
		cfg.AuthDir = tempAuthDir
		cfg.KiroTokenFiles = nil // rely on auth-dir scanning so watcher registers kiro auth
	}

	newConfigContent, err := yaml.Marshal(cfg)
	tempConfigPath := filepath.Join(rootDir, "config_test_temp.yaml")
	require.NoError(t, err)

	err = os.WriteFile(tempConfigPath, newConfigContent, 0644)
	require.NoError(t, err)

	cmd := exec.Command(serverBin, "--config", tempConfigPath)
	cmd.Dir = rootDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Start()
	require.NoError(t, err, "Failed to start server")

	cleanup := func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		_ = os.Remove(serverBin)
		_ = os.Remove(tempConfigPath)
	}

	return cmd, port, cleanup
}

func waitForServer(baseURL string) error {
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/health") // Assuming /health exists, or just check connection
		if err == nil {
			resp.Body.Close()
			return nil
		}
		// Try root if health doesn't exist or just tcp dial
		conn, err := http.Get(baseURL)
		if err == nil {
			conn.Body.Close()
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for server")
}

func loadAndModifyPayload(t *testing.T, dir, filename, model string) []byte {
	path := filepath.Join(dir, filename+".json")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	// Use sjson to modify model
	// Note: sjson.Set returns string, need to convert back to bytes
	// Or just use a struct map
	var payload map[string]interface{}
	err = json.Unmarshal(data, &payload)
	require.NoError(t, err)

	payload["model"] = model

	modData, err := json.Marshal(payload)
	require.NoError(t, err)
	return modData
}

func sendRequest(t *testing.T, url, apiKey string, payload []byte) (string, int) {
	return sendRequestWithHeaders(t, url, apiKey, payload, nil)
}

func sendRequestWithHeaders(t *testing.T, url, apiKey string, payload []byte, extraHeaders map[string]string) (string, int) {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return string(body), resp.StatusCode
}
