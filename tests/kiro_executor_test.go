// Package executor provides runtime execution capabilities for various AI service providers.
// This file contains unit tests for the Kiro executor functionality.
package executor

import (
	"encoding/json"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

func TestNewKiroExecutor(t *testing.T) {
	cfg := &config.Config{
		Port:    8080,
		AuthDir: "/tmp/auth",
	}

	executor := NewKiroExecutor(cfg)

	if executor == nil {
		t.Fatal("Expected non-nil KiroExecutor")
	}

	if executor.cfg != cfg {
		t.Error("Expected executor config to match provided config")
	}

	if executor.auth == nil {
		t.Error("Expected non-nil auth instance")
	}
}

func TestKiroExecutor_Identifier(t *testing.T) {
	cfg := &config.Config{}
	executor := NewKiroExecutor(cfg)

	identifier := executor.Identifier()
	expected := "kiro"

	if identifier != expected {
		t.Errorf("Expected identifier %s, got %s", expected, identifier)
	}
}

func TestKiroExecutor_PrepareRequest(t *testing.T) {
	cfg := &config.Config{}
	executor := NewKiroExecutor(cfg)

	// Test PrepareRequest (should be a no-op and return nil)
	err := executor.PrepareRequest(nil, nil)
	if err != nil {
		t.Errorf("Expected PrepareRequest to return nil, got %v", err)
	}
}

func TestMapKiroModel(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected string
	}{
		{
			name:     "Claude Sonnet 4.5",
			model:    "claude-sonnet-4-5",
			expected: "CLAUDE_SONNET_4_5_20250929_V1_0",
		},
		{
			name:     "Claude Sonnet 4.5 with date",
			model:    "claude-sonnet-4-5-20250929",
			expected: "CLAUDE_SONNET_4_5_20250929_V1_0",
		},
		{
			name:     "Claude Sonnet 4",
			model:    "claude-sonnet-4-20250514",
			expected: "CLAUDE_SONNET_4_20250514_V1_0",
		},
		{
			name:     "Claude 3.7 Sonnet",
			model:    "claude-3-7-sonnet-20250219",
			expected: "CLAUDE_3_7_SONNET_20250219_V1_0",
		},
		{
			name:     "Amazon Q Claude 4",
			model:    "amazonq-claude-sonnet-4-20250514",
			expected: "CLAUDE_SONNET_4_20250514_V1_0",
		},
		{
			name:     "Unknown model",
			model:    "unknown-model",
			expected: "CLAUDE_SONNET_4_5_20250929_V1_0", // Should default to claude-sonnet-4-5
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if result := mapKiroModel(tt.model); result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestKiroExecutor_ExtractRegionFromARN(t *testing.T) {
	executor := NewKiroExecutor(&config.Config{})
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
			result := executor.extractRegionFromARN(tt.arn)
			if result != tt.expected {
				t.Errorf("Expected region %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestKiroModelMapping(t *testing.T) {
	// Test that all expected models are in the mapping
	expectedModels := []string{
		"claude-sonnet-4-5",
		"claude-sonnet-4-5-20250929",
		"claude-sonnet-4-20250514",
		"claude-3-7-sonnet-20250219",
		"amazonq-claude-sonnet-4-20250514",
		"amazonq-claude-3-7-sonnet-20250219",
	}

	for _, model := range expectedModels {
		t.Run(model+" mapping exists", func(t *testing.T) {
			if _, exists := KiroModelMapping[model]; !exists {
				t.Errorf("Expected model %s to exist in KiroModelMapping", model)
			}
		})
	}

	// Test that all mapped values are non-empty
	for model, kiroModel := range KiroModelMapping {
		t.Run(model+" has valid mapping", func(t *testing.T) {
			if kiroModel == "" {
				t.Errorf("Expected non-empty Kiro model for %s", model)
			}
		})
	}
}

func TestEstimateCompletionTokens(t *testing.T) {
	text := "Hello world"
	toolCalls := []openAIToolCall{
		{Name: "test", Arguments: `{"foo":"bar"}`},
	}
	tokens := estimateCompletionTokens(text, toolCalls)
	if tokens <= 0 {
		t.Fatalf("expected positive token count, got %d", tokens)
	}
}

func TestBuildKiroRequestPayload(t *testing.T) {
	payload := []byte(`{"messages":[{"role":"user","content":"Hello"},{"role":"assistant","content":"Hi there"}]}`)
	ts := &kiro.KiroTokenStorage{
		ProfileArn:  "arn:aws:codewhisperer:us-east-1:123456789012:profile/Test",
		AuthMethod:  "social",
		AccessToken: "token",
	}
	body, err := buildKiroRequestPayload("claude-sonnet-4-5", payload, ts, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if parsed["profileArn"] != ts.ProfileArn {
		t.Fatalf("expected profileArn %s, got %v", ts.ProfileArn, parsed["profileArn"])
	}
	conv, ok := parsed["conversationState"].(map[string]any)
	if !ok {
		t.Fatalf("conversationState missing")
	}
	if conv["chatTriggerType"] != kiroChatTrigger {
		t.Fatalf("unexpected chat trigger: %v", conv["chatTriggerType"])
	}
}

func TestBuildKiroRequestPayload_ToolsSchemaDefault(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload []byte
	}{
		{
			name: "MissingParametersDefaultsToEmptyObject",
			payload: []byte(`{
				"messages":[{"role":"user","content":"Hola"}],
				"tools":[{"type":"function","function":{"name":"lookup","description":"Lookup something"}}]
			}`),
		},
		{
			name: "ExplicitEmptyParameters",
			payload: []byte(`{
				"messages":[{"role":"user","content":"Hola"}],
				"tools":[{"type":"function","function":{"name":"lookup","description":"Lookup something","parameters":{}}}]
			}`),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ts := &kiro.KiroTokenStorage{
				ProfileArn:  "arn:aws:codewhisperer:us-east-1:123456789012:profile/Test",
				AuthMethod:  "social",
				AccessToken: "token",
			}

			body, err := buildKiroRequestPayload("claude-sonnet-4-5", tc.payload, ts, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var parsed map[string]any
			if err := json.Unmarshal(body, &parsed); err != nil {
				t.Fatalf("invalid json: %v", err)
			}

			conv, ok := parsed["conversationState"].(map[string]any)
			if !ok {
				t.Fatalf("conversationState missing")
			}
			current, ok := conv["currentMessage"].(map[string]any)
			if !ok {
				t.Fatalf("currentMessage missing")
			}
			user, ok := current["userInputMessage"].(map[string]any)
			if !ok {
				t.Fatalf("userInputMessage missing")
			}
			ctx, ok := user["userInputMessageContext"].(map[string]any)
			if !ok {
				t.Fatalf("userInputMessageContext missing")
			}
			tools, ok := ctx["tools"].([]any)
			if !ok || len(tools) == 0 {
				t.Fatalf("tools context missing")
			}
			firstTool, ok := tools[0].(map[string]any)
			if !ok {
				t.Fatalf("invalid tool entry")
			}
			spec, ok := firstTool["toolSpecification"].(map[string]any)
			if !ok {
				t.Fatalf("toolSpecification missing")
			}
			inputSchema, ok := spec["inputSchema"].(map[string]any)
			if !ok {
				t.Fatalf("inputSchema missing")
			}
			jsonSchema, ok := inputSchema["json"].(map[string]any)
			if !ok {
				t.Fatalf("json schema should be an object, got %T", inputSchema["json"])
			}
			if jsonSchema == nil {
				t.Fatal("json schema should not be nil")
			}
		})
	}
}
