package kiro_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/runtime/executor"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FAILING TESTS FOR KiroExecutor FORMAT DETECTION
// These tests will fail because format detection logic doesn't exist yet

func TestKiroExecutor_FormatDetection_OpenAIFormat(t *testing.T) {
	t.Parallel()
	// Test format detection for OpenAI format requests
	cfg := &config.Config{}
	exec := executor.NewKiroExecutor(cfg)

	// Create a test request with OpenAI format
	req := cliproxyexecutor.Request{
		Model: "claude-sonnet-4-5",
		Payload: []byte(`{
			"messages": [
				{"role": "user", "content": "Hello"}
			]
		}`),
	}

	// This should fail because format detection logic doesn't exist
	format := exec.DetectRequestFormat(req)
	assert.Equal(t, "openai", format, "Should detect OpenAI format")
}

func TestKiroExecutor_FormatDetection_AnthropicFormat(t *testing.T) {
	t.Parallel()
	// Test format detection for Anthropic format requests
	cfg := &config.Config{}
	exec := executor.NewKiroExecutor(cfg)

	// Create a test request with Anthropic format
	req := cliproxyexecutor.Request{
		Model: "claude-sonnet-4-5",
		Payload: []byte(`{
			"max_tokens": 1000,
			"messages": [
				{"role": "user", "content": "Hello"}
			]
		}`),
	}

	// This should fail because format detection logic doesn't exist
	format := exec.DetectRequestFormat(req)
	assert.Equal(t, "anthropic", format, "Should detect Anthropic format")
}

func TestKiroExecutor_FormatDetection_InvalidFormat(t *testing.T) {
	t.Parallel()
	// Test format detection for invalid format requests
	cfg := &config.Config{}
	exec := executor.NewKiroExecutor(cfg)

	// Create a test request with invalid format
	req := cliproxyexecutor.Request{
		Model:   "claude-sonnet-4-5",
		Payload: []byte(`{ "invalid": "format" }`),
	}

	// This should fail because format detection logic doesn't exist
	format := exec.DetectRequestFormat(req)
	assert.Equal(t, "unknown", format, "Should detect unknown format")
}

func TestKiroExecutor_FormatDetection_EdgeCases(t *testing.T) {
	t.Parallel()
	// Test format detection for edge cases
	cfg := &config.Config{}
	exec := executor.NewKiroExecutor(cfg)

	testCases := []struct {
		name     string
		payload  []byte
		expected string
	}{
		{
			name:     "empty_payload",
			payload:  []byte{},
			expected: "unknown",
		},
		{
			name:     "invalid_json",
			payload:  []byte("{ invalid json }"),
			expected: "unknown",
		},
		{
			name:     "openai_with_tools",
			payload:  []byte(`{"messages": [{"role": "user", "content": "Hello"}], "tools": []}`),
			expected: "openai",
		},
		{
			name:     "anthropic_with_system",
			payload:  []byte(`{"max_tokens": 1000, "system": "You are helpful", "messages": [{"role": "user", "content": "Hello"}]}`),
			expected: "anthropic",
		},
		{
			name:     "anthropic_with_streaming",
			payload:  []byte(`{"max_tokens": 1000, "stream": true, "messages": [{"role": "user", "content": "Hello"}]}`),
			expected: "anthropic",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := cliproxyexecutor.Request{
				Model:   "claude-sonnet-4-5",
				Payload: tc.payload,
			}

			// This should fail because format detection logic doesn't exist
			format := exec.DetectRequestFormat(req)
			assert.Equal(t, tc.expected, format, "Should detect expected format")
		})
	}
}

func TestKiroExecutor_CountTokensIncludesSystemPrompt(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	exec := executor.NewKiroExecutor(cfg)
	ctx := context.Background()

	withSystem := cliproxyexecutor.Request{
		Model: "claude-sonnet-4-5",
		Payload: []byte(`{
			"system": [{"type":"text","text":"You are Claude Code. Always call tools."}],
			"messages": [{"role":"user","content":[{"type":"text","text":"Ping"}]}]
		}`),
	}
	withoutSystem := cliproxyexecutor.Request{
		Model: "claude-sonnet-4-5",
		Payload: []byte(`{
			"messages": [{"role":"user","content":[{"type":"text","text":"Ping"}]}]
		}`),
	}

	respWithSystem, err := exec.CountTokens(ctx, nil, withSystem, cliproxyexecutor.Options{})
	require.NoError(t, err)
	respWithoutSystem, err := exec.CountTokens(ctx, nil, withoutSystem, cliproxyexecutor.Options{})
	require.NoError(t, err)

	withUsage := extractPromptTokens(t, respWithSystem.Payload)
	withoutUsage := extractPromptTokens(t, respWithoutSystem.Payload)
	require.Greater(t, withUsage, withoutUsage, "system prompts should increase prompt token count")
}

func extractPromptTokens(t *testing.T, payload []byte) int64 {
	t.Helper()
	var body struct {
		Usage struct {
			PromptTokens int64 `json:"prompt_tokens"`
		} `json:"usage"`
	}
	require.NoError(t, json.Unmarshal(payload, &body))
	return body.Usage.PromptTokens
}
