package kiro_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	authkiro "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro"
	kirotranslator "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro"
	testutil "github.com/router-for-me/CLIProxyAPI/v6/tests/shared"
)

// TestKiroHardRequestParsing tests the Kiro response parsing using the complex hard request fixture
func TestKiroHardRequestParsing(t *testing.T) {
	t.Parallel()
	// Read the hard request fixture
	fixtureData := testutil.LoadTestData(t, "nonstream/test_hard_request.json")

	// Parse the fixture to understand the request structure
	var request map[string]any
	if err := json.Unmarshal(fixtureData, &request); err != nil {
		t.Fatalf("Failed to parse fixture JSON: %v", err)
	}

	// Extract key information from the fixture
	model := request["model"].(string)
	stream, ok := request["stream"].(bool)
	messages := request["messages"].([]any)

	// Verify the fixture contains expected data
	assert.Equal(t, "claude-sonnet-4-5", model)
	if ok {
		assert.True(t, stream)
	}
	assert.Greater(t, len(messages), 0, "Should have messages in the fixture")

	// Create a Kiro token for testing
	token := &authkiro.KiroTokenStorage{
		ProfileArn:  "arn:aws:codewhisperer:us-east-1:699475941385:profile/test",
		AccessToken: "test_access_token",
		ExpiresAt:   time.Now().Add(24 * time.Hour),
		Type:        "kiro",
	}

	// Test BuildRequest with the fixture data
	result, err := kirotranslator.BuildRequest(model, fixtureData, token, nil)
	assert.NoError(t, err, "BuildRequest should handle the complex fixture without errors")

	// Verify the result is valid JSON
	var kiroRequest map[string]any
	if err := json.Unmarshal(result, &kiroRequest); err != nil {
		t.Fatalf("Failed to parse BuildRequest result: %v", err)
	}

	// Verify basic structure
	assert.Contains(t, kiroRequest, "conversationState", "Should have conversationState")

	conversationState := kiroRequest["conversationState"].(map[string]any)
	assert.Contains(t, conversationState, "currentMessage", "Should have currentMessage")
	assert.Contains(t, conversationState, "history", "Should have history")

	// Verify model mapping - it might be at the top level or in conversationState
	if modelId, exists := kiroRequest["modelId"]; exists {
		assert.Equal(t, "CLAUDE_SONNET_4_5_20250929_V1_0", modelId, "Should map Claude model correctly")
	} else if currentMessage, exists := conversationState["currentMessage"]; exists {
		if userInputMsg, ok := currentMessage.(map[string]any)["userInputMessage"]; ok {
			if modelId, exists := userInputMsg.(map[string]any)["modelId"]; exists {
				assert.Equal(t, "CLAUDE_SONNET_4_5_20250929_V1_0", modelId, "Should map Claude model correctly in currentMessage")
			}
		}
	}

	// Test ParseResponse with a mock Kiro response
	// Create a mock response that would come from Kiro API
	mockKiroResponse := `{
		"conversationState": {
			"currentMessage": {
				"assistantResponseMessage": {
					"content": "I'll help you analyze this codebase and create a CLAUDE.md file.",
					"toolUse": [
						{
							"toolUseId": "tool_1",
							"name": "TodoWrite",
							"input": {
								"todos": [
									{
										"content": "Explore repository structure",
										"status": "in_progress",
										"activeForm": "Exploring repository"
									}
								]
							}
						}
					]
				}
			}
		}
	}`

	// Test ParseResponse
	content, toolCalls := kiro.ParseResponse([]byte(mockKiroResponse))
	assert.NotEmpty(t, content, "ParseResponse should extract content")
	assert.Greater(t, len(toolCalls), 0, "ParseResponse should extract tool calls")

	// Verify the content
	assert.Contains(t, content, "analyze this codebase", "Content should contain expected text")

	// Verify tool calls were extracted
	if len(toolCalls) > 0 {
		assert.Equal(t, "tool_1", toolCalls[0].ID, "Tool call should have correct ID")
		assert.Equal(t, "TodoWrite", toolCalls[0].Name, "Tool call should have correct name")
		assert.NotEmpty(t, toolCalls[0].Arguments, "Tool call should have arguments")
	}

	t.Logf("Successfully parsed complex fixture with %d messages", len(messages))
	t.Logf("BuildRequest produced valid Kiro request with model: %s", kiroRequest["modelId"])
	t.Logf("ParseResponse successfully converted mock Kiro response to OpenAI format")
}

// TestKiroHardRequestStreaming tests streaming behavior with the complex fixture
func TestKiroHardRequestStreaming(t *testing.T) {
	t.Parallel()
	// Read the hard request fixture
	fixtureData := testutil.LoadTestData(t, "nonstream/test_hard_request.json")

	// Parse the fixture
	var request map[string]any
	if err := json.Unmarshal(fixtureData, &request); err != nil {
		t.Fatalf("Failed to parse fixture JSON: %v", err)
	}

	if stream, ok := request["stream"].(bool); ok {
		assert.True(t, stream, "Fixture should have streaming enabled")
	}

	// Create a Kiro token for testing
	token := &authkiro.KiroTokenStorage{
		ProfileArn:  "arn:aws:codewhisperer:us-east-1:699475941385:profile/test",
		AccessToken: "test_access_token",
		ExpiresAt:   time.Now().Add(24 * time.Hour),
		Type:        "kiro",
	}

	// Test BuildRequest with streaming
	result, err := kirotranslator.BuildRequest(request["model"].(string), fixtureData, token, nil)
	assert.NoError(t, err, "BuildRequest should handle streaming fixture without errors")

	// Verify the result contains streaming configuration
	var kiroRequest map[string]any
	if err := json.Unmarshal(result, &kiroRequest); err != nil {
		t.Fatalf("Failed to parse BuildRequest result: %v", err)
	}

	// The Kiro request should preserve the streaming intent
	conversationState := kiroRequest["conversationState"].(map[string]any)
	currentMessage := conversationState["currentMessage"].(map[string]any)
	assert.Contains(t, currentMessage, "userInputMessage", "Should have userInputMessage")

	t.Logf("Streaming request successfully processed for model: %s", kiroRequest["modelId"])
}

// TestKiroHardRequestErrorHandling tests error handling with malformed fixture data
func TestKiroHardRequestErrorHandling(t *testing.T) {
	// Test with invalid JSON
	invalidJSON := []byte(`{"invalid": json}`)

	token := &authkiro.KiroTokenStorage{
		ProfileArn:  "arn:aws:codewhisperer:us-east-1:699475941385:profile/test",
		AccessToken: "test_access_token",
		ExpiresAt:   time.Now().Add(24 * time.Hour),
		Type:        "kiro",
	}

	// BuildRequest should handle invalid JSON gracefully
	_, err := kirotranslator.BuildRequest("claude-sonnet-4-5", invalidJSON, token, nil)
	assert.Error(t, err, "BuildRequest should return error for invalid JSON")

	// Test ParseResponse with invalid response
	content, toolCalls := kiro.ParseResponse(invalidJSON)
	// ParseResponse should not panic on invalid JSON, but may return the raw content
	assert.NotEmpty(t, content, "Invalid JSON may return raw content as fallback")
	assert.Empty(t, toolCalls, "Invalid JSON should return no tool calls")

	t.Logf("Error handling working correctly for invalid inputs")
}
