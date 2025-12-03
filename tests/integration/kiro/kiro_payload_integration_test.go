//go:build integration && kiro_live && !windows
// +build integration,kiro_live,!windows

package kiro_test

// This test performs end-to-end integration testing for the Kiro provider,
// verifying that it correctly translates between OpenAI and Anthropic (Claude) formats.
//
// PREREQUISITES (live Kiro call, opt-in via build tags):
//   - Build tags: integration, kiro_live
//   - A valid kiro-auth-token.json file must exist in ~/.cli-proxy-api/
//   - The test uses the real token for authentication
//
// TIMEOUT & RETRY CONFIGURATION:
//   - HTTP request timeout: 10 seconds per attempt
//   - Kiro executor retry attempts: up to 3 (primary → flattened → minimal)
//   - Maximum total request time: 30 seconds (10s × 3 attempts)
//   - Test must complete within: 30 seconds
//
// RUNNING TESTS:
//
// 1. Run All Tests (OpenAI + Anthropic formats)
//    ------------------------------------------
//    go test -tags='integration kiro_live' -v ./tests/integration/kiro -run TestKiroPayloads
//
//    Expected duration: 10-30 seconds
//
//    This runs:
//    - OpenAI Chat Completions (3 test cases)
//      * openai_format_simple
//      * openai_format
//      * openai_format_with_tools
//    - Anthropic Messages (3 test cases)
//      * claude_format
//      * claude_format_tool_call_no_result
//      * claude_format_simple
//
// 2. Run Only OpenAI Format Tests
//    -----------------------------
//    go test -tags='integration kiro_live' -v ./tests/integration/kiro -run TestKiroPayloads/OpenAI_Chat_Completions
//
// 3. Run Only Anthropic/Claude Format Tests
//    ---------------------------------------
//    go test -tags='integration kiro_live' -v ./tests/integration/kiro -run TestKiroPayloads/Anthropic_Messages
//
// 4. Run Specific Test Cases
//    ------------------------
//    Single test case:
//    go test -tags='integration kiro_live' -v ./tests/integration/kiro -run TestKiroPayloads/OpenAI_Chat_Completions/openai_format_simple
//
//    Specific Claude test:
//    go test -tags='integration kiro_live' -v ./tests/integration/kiro -run TestKiroPayloads/Anthropic_Messages/claude_format_simple
//
//    Pattern matching (all simple format tests):
//    go test -tags=integration -v ./tests/integration/kiro -run '.*/.*simple'
//
// 5. Skip Integration Tests (for unit test runs)
//    --------------------------------------------
//    go test -short -v ./tests/integration/kiro -run TestKiroPayloads
//
//    or simply:
//    go test -short ./...
//
// LOGGING:
//   - Logs are written to logs/kiro_payload_debug_YYYYMMDD_HHMMSS.log
//   - Each test run creates a new timestamped log file
//   - Logs include detailed request/response debug information
//   - Log files are automatically closed when test completes
//   - The test also shows the log filename in the output
//
// TEST DATA:
//   Test payloads are loaded from tests/testdata/:
//   - openai_format_simple.json
//   - openai_format.json
//   - openai_format_with_tools.json
//   - claude_format.json
//   - claude_format_tool_call_no_result.json
//   - claude_format_simple.json
//
// VALIDATION:
//   - OpenAI format tests strictly verify: choices, choices.0.message
//   - Anthropic format tests strictly verify: content, role
//   - Any format mismatch will cause the test to FAIL

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	internalconfig "github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	sdkconfig "github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"gopkg.in/natefinch/lumberjack.v2"
	"gopkg.in/yaml.v3"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/logging"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
)

// TestKiroPayloads performs an end-to-end integration test for Kiro.
// It starts the server with built-in config and runs payload tests.
func TestKiroPayloads(t *testing.T) {
	const modelName = "claude-sonnet-4-5" // must exist in registry.GetKiroModels
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 1. Locate project root
	rootDir, err := findProjectRoot()
	require.NoError(t, err, "Failed to find project root")

	// 2. Start the server with built-in config (no external config.yaml needed)
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
				// Strictly verify OpenAI format response from /v1/chat/completions
				assert.True(t, gjson.Get(resp, "choices").Exists(), "Response must be OpenAI format with 'choices' field. Body: %s", resp)
				assert.True(t, gjson.Get(resp, "choices.0.message").Exists(), "Missing 'choices.0.message'. Body: %s", resp)
			})
		}
	})

	t.Run("Anthropic Messages", func(t *testing.T) {
		files := []string{
			"claude_format",
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

// findProjectRoot locates the project root directory by looking for go.mod
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

func startServer(t *testing.T, rootDir string) (*exec.Cmd, int, func()) {
	// Create logs directory in project root if it doesn't exist
	logsDir := filepath.Join(rootDir, "logs")
	if _, err := os.Stat(logsDir); os.IsNotExist(err) {
		os.MkdirAll(logsDir, 0755)
	}

	// Generate timestamped log filename
	timestamp := time.Now().Format("20060102_150405") // YYYYMMDD_HHMMSS
	logFilename := filepath.Join(logsDir, fmt.Sprintf("kiro_payload_debug_%s.log", timestamp))

	// Configure logger to write to the timestamped file
	fileLogger := &lumberjack.Logger{
		Filename:   logFilename,
		MaxSize:    100, // megabytes
		MaxBackups: 0,
		MaxAge:     0,
		Compress:   false,
	}

	// Set up logging.BaseLogger
	logging.SetupBaseLogger()
	logrus.SetOutput(fileLogger)
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetReportCaller(true)
	logrus.SetFormatter(&logging.LogFormatter{})

	// Create built-in config (no need for external config.yaml)
	cfg := &internalconfig.Config{
		SDKConfig: sdkconfig.SDKConfig{
			APIKeys: []string{"test-api-key"},
		},
		Port:                             8318, // Use a different port for testing to avoid conflict
		Debug:                            true,
		LoggingToFile:                    true,
		UsageStatisticsEnabled:           false,
		RequestRetry:                     3,
		MaxRetryInterval:                 30,
		AuthDir:                          "~/.cli-proxy-api", // Use real auth dir with real token
		AmpRestrictManagementToLocalhost: true,
	}

	t.Logf("Log file: %s", logFilename)

	// Write the config to a temp file in /tmp directory (not in project)
	newConfigContent, err := yaml.Marshal(cfg)
	tempConfigPath := filepath.Join("/tmp", fmt.Sprintf("config_test_temp_%d.yaml", time.Now().UnixNano()))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(tempConfigPath, newConfigContent, 0644))

	// Run server directly with go run (no need to build)
	cmd := exec.Command("go", "run", "./cmd/server/main.go", "--config", tempConfigPath)
	cmd.Dir = rootDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Put the server in its own process group so we can kill both the go-run wrapper
	// and the compiled server it spawns. Otherwise the child keeps stdout open and
	// `go test` hits ErrWaitDelay after the tests finish.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	err = cmd.Start()
	require.NoError(t, err, "Failed to start server")

	cleanup := func() {
		// Kill the entire process group to ensure the spawned server exits too.
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		_ = cmd.Wait()
		_ = os.Remove(tempConfigPath)
		_ = fileLogger.Close()
	}

	return cmd, cfg.Port, cleanup
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
	// Force non-streaming responses for integration assertions that expect JSON.
	payload["stream"] = false

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

	// Log request payload (pretty-printed when possible) and headers to the debug log file.
	prettyPayload := payload
	if len(payload) > 0 {
		var buf bytes.Buffer
		if err := json.Indent(&buf, payload, "", "  "); err == nil {
			prettyPayload = buf.Bytes()
		}
	}

	logrus.Infof("kiro integration: sending request url=%s headers=%v payload=%s", url, req.Header, string(prettyPayload))

	// Timeout reduced from 30s to 10s to prevent long hangs
	client := &http.Client{Timeout: 10 * time.Second}

	startTime := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(startTime)

	if err != nil {
		t.Logf("Request failed after %v: %v", duration, err)
		logrus.WithError(err).Errorf("kiro integration: request failed url=%s duration=%v", url, duration)
		require.NoError(t, err, "Request failed after %v", duration)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	bodySize := len(body)
	t.Logf("Request completed in %v, status=%d, body_size=%d bytes", duration, resp.StatusCode, bodySize)
	logrus.Infof("kiro integration: response status=%d duration=%v body_size=%d body=%s", resp.StatusCode, duration, bodySize, string(body))

	if resp.StatusCode != 200 {
		t.Logf("Non-200 response: %s", string(body))
	}

	return string(body), resp.StatusCode
}
