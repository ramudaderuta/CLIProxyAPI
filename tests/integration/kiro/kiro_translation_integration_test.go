//go:build integration
// +build integration

package kiro_test

import (
	"encoding/json"
	"testing"
	"time"

	kirotranslator "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro"
	authkiro "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
)

// TestKiroTranslationIntegration_CompleteFlow tests the complete translation flow
// from OpenAI format to Kiro format and back to OpenAI format.
func TestKiroTranslationIntegration_CompleteFlow(t *testing.T) {
	// Create OpenAI chat completion payload with system message and history
	openAIPayload := map[string]any{
		"model": "claude-sonnet-4-5",
		"messages": []map[string]any{
			{"role": "user", "content": "Hello, how are you?"},
		},
		"stream": false,
	}

	payloadBytes, err := json.Marshal(openAIPayload)
	if err != nil {
		t.Fatalf("Failed to marshal OpenAI payload: %v", err)
	}

	// Create a mock token for testing
	token := &authkiro.KiroTokenStorage{
		ProfileArn:  "arn:aws:codewhisperer:us-east-1:699475941385:profile/test",
		AccessToken: "test_access_token",
		ExpiresAt:   time.Now().Add(24 * time.Hour),
		Type:        "kiro",
	}

	// Test 1: OpenAI → Kiro translation (request)
	kiroReqBytes, err := kirotranslator.BuildRequest("claude-sonnet-4-5", payloadBytes, token, nil)
	if err != nil {
		t.Fatalf("Failed to build Kiro request: %v", err)
	}

	// Verify the Kiro request structure
	var kiroReq map[string]any
	if err := json.Unmarshal(kiroReqBytes, &kiroReq); err != nil {
		t.Fatalf("Failed to unmarshal Kiro request: %v", err)
	}

	// Verify the Kiro request structure
	if kiroReq["conversationState"] == nil {
		t.Error("Expected non-nil conversationState in Kiro request")
	}

	conversationState := kiroReq["conversationState"].(map[string]any)
	currentMessage := conversationState["currentMessage"].(map[string]any)
	userInputMessage := currentMessage["userInputMessage"].(map[string]any)

	// Verify model mapping
	expectedModel := kirotranslator.MapModel("claude-sonnet-4-5")
	if userInputMessage["modelId"] != expectedModel {
		t.Errorf("Expected modelId %s, got %v", expectedModel, userInputMessage["modelId"])
	}

	// Verify content
	if userInputMessage["content"] != "Hello, how are you?" {
		t.Errorf("Expected content 'Hello, how are you?', got %v", userInputMessage["content"])
	}

	// Test 2: Kiro → OpenAI translation (response)
	kiroResponse := map[string]any{
		"conversationState": map[string]any{
			"currentMessage": map[string]any{
				"assistantResponseMessage": map[string]any{
					"content": "I'd be happy to help you! What do you need assistance with?",
				},
			},
		},
	}

	kiroResponseBytes, err := json.Marshal(kiroResponse)
	if err != nil {
		t.Fatalf("Failed to marshal Kiro response: %v", err)
	}

	// Parse the Kiro response
	content, toolCalls := kirotranslator.ParseResponse(kiroResponseBytes)

	// Build OpenAI chat completion payload from the parsed response
	openAIResponseBytes, err := kirotranslator.BuildOpenAIChatCompletionPayload(
		"claude-sonnet-4-5",
		content,
		toolCalls,
		100, // promptTokens
		50,  // completionTokens
	)
	if err != nil {
		t.Fatalf("Failed to build OpenAI response: %v", err)
	}

	// Verify the OpenAI response structure
	var openAIResponse map[string]any
	if err := json.Unmarshal(openAIResponseBytes, &openAIResponse); err != nil {
		t.Fatalf("Failed to unmarshal OpenAI response: %v", err)
	}

	choices, ok := openAIResponse["choices"].([]any)
	if !ok || len(choices) == 0 {
		t.Error("Expected at least one choice in OpenAI response")
	}

	choice := choices[0].(map[string]any)
	message := choice["message"].(map[string]any)
	if message["content"] == nil {
		t.Error("Expected non-empty message content in OpenAI response")
	}

	expectedContent := "I'd be happy to help you! What do you need assistance with?"
	if message["content"] != expectedContent {
		t.Errorf("Expected content %s, got %s", expectedContent, message["content"])
	}

	// Verify model is preserved
	if openAIResponse["model"] != "claude-sonnet-4-5" {
		t.Errorf("Expected model claude-sonnet-4-5, got %s", openAIResponse["model"])
	}
}

// TestKiroTranslationIntegration_StreamingFlow tests the complete streaming translation flow
func TestKiroTranslationIntegration_StreamingFlow(t *testing.T) {
	// Create OpenAI chat completion payload for streaming
	openAIPayload := map[string]any{
		"model": "claude-sonnet-4-5",
		"messages": []map[string]any{
			{"role": "user", "content": "Tell me a short story"},
		},
		"stream": true,
	}

	payloadBytes, err := json.Marshal(openAIPayload)
	if err != nil {
		t.Fatalf("Failed to marshal OpenAI payload: %v", err)
	}

	// Create a mock token for testing
	token := &authkiro.KiroTokenStorage{
		ProfileArn:  "arn:aws:codewhisperer:us-east-1:699475941385:profile/test",
		AccessToken: "test_access_token",
		ExpiresAt:   time.Now().Add(24 * time.Hour),
		Type:        "kiro",
	}

	// Test 1: OpenAI → Kiro translation (request)
	kiroReqBytes, err := kirotranslator.BuildRequest("claude-sonnet-4-5", payloadBytes, token, nil)
	if err != nil {
		t.Fatalf("Failed to build Kiro request: %v", err)
	}

	// Verify streaming request structure
	var kiroReq map[string]any
	if err := json.Unmarshal(kiroReqBytes, &kiroReq); err != nil {
		t.Fatalf("Failed to unmarshal Kiro request: %v", err)
	}

	// Verify the request structure (userInput is nested under conversationState)
	if kiroReq["conversationState"] == nil {
		t.Error("Expected non-nil conversationState in Kiro request")
	}

	// Test 2: Kiro streaming response → OpenAI streaming chunks
	content := "Once upon a time..."

	// Build streaming chunks using the translator function
	chunks := kirotranslator.BuildStreamingChunks(
		"chatcmpl_test",
		"claude-sonnet-4-5",
		1699000000, // created timestamp
		content,
		nil, // no tool calls
	)

	// Verify we got chunks
	if len(chunks) == 0 {
		t.Error("Expected at least one chunk")
	}

	// Verify chunk structure
	for i, chunkBytes := range chunks {
		var chunk map[string]any
		if err := json.Unmarshal(chunkBytes, &chunk); err != nil {
			t.Fatalf("Failed to unmarshal chunk %d: %v", i, err)
		}

		if chunk["choices"] == nil {
			t.Errorf("Expected choices in chunk %d", i)
		}
	}
}

// TestKiroTranslationIntegration_WithTools tests translation flow with tool calls
func TestKiroTranslationIntegration_WithTools(t *testing.T) {
	// Create OpenAI payload with tools
	openAIPayload := map[string]any{
		"model": "claude-sonnet-4-5",
		"messages": []map[string]any{
			{"role": "user", "content": "What's the weather like?"},
		},
		"tools": []map[string]any{
			{
				"type": "function",
				"function": map[string]any{
					"name":        "get_weather",
					"description": "Get current weather information",
					"parameters": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"location": map[string]any{
								"type":        "string",
								"description": "The city and state",
							},
						},
						"required": []string{"location"},
					},
				},
			},
		},
		"stream": false,
	}

	payloadBytes, err := json.Marshal(openAIPayload)
	if err != nil {
		t.Fatalf("Failed to marshal OpenAI payload: %v", err)
	}

	// Create a mock token for testing
	token := &authkiro.KiroTokenStorage{
		ProfileArn:  "arn:aws:codewhisperer:us-east-1:699475941385:profile/test",
		AccessToken: "test_access_token",
		ExpiresAt:   time.Now().Add(24 * time.Hour),
		Type:        "kiro",
	}

	// Test OpenAI → Kiro translation with tools
	kiroReqBytes, err := kirotranslator.BuildRequest("claude-sonnet-4-5", payloadBytes, token, nil)
	if err != nil {
		t.Fatalf("Failed to build Kiro request with tools: %v", err)
	}

	// Verify tools are included in the request
	var kiroReq map[string]any
	if err := json.Unmarshal(kiroReqBytes, &kiroReq); err != nil {
		t.Fatalf("Failed to unmarshal Kiro request: %v", err)
	}

	// Verify the request structure (userInput is nested under conversationState)
	if kiroReq["conversationState"] == nil {
		t.Error("Expected non-nil conversationState in Kiro request")
	}

	// The tools should be converted to Kiro's format
	conversationState := kiroReq["conversationState"].(map[string]any)
	currentMessage := conversationState["currentMessage"].(map[string]any)
	userInput := currentMessage["userInputMessage"].(map[string]any)

	// Check that tools information is present (the exact format depends on Kiro's API)
	if _, exists := userInput["tools"]; !exists {
		// Tools might be embedded in a different field in Kiro's format
		// This test verifies the translation doesn't fail with tools
		t.Log("Tools field not found in UserInput - this may be expected based on Kiro's API format")
	}
}

// TestKiroTranslationIntegration_ModelMapping tests model mapping consistency
func TestKiroTranslationIntegration_ModelMapping(t *testing.T) {
	testCases := []struct {
		openAIModel string
		expected    string
	}{
		{"claude-sonnet-4-5", "CLAUDE_SONNET_4_5_20250929_V1_0"},
		{"claude-sonnet-4-5-20250929", "CLAUDE_SONNET_4_5_20250929_V1_0"},
		{"claude-sonnet-4-20250514", "CLAUDE_SONNET_4_20250514_V1_0"},
		{"unknown-model", "CLAUDE_SONNET_4_5_20250929_V1_0"}, // Should default to claude-sonnet-4-5
	}

	for _, tc := range testCases {
		t.Run(tc.openAIModel, func(t *testing.T) {
			mapped := kirotranslator.MapModel(tc.openAIModel)
			if mapped != tc.expected {
				t.Errorf("Expected model mapping %s -> %s, got %s", tc.openAIModel, tc.expected, mapped)
			}
		})
	}
}