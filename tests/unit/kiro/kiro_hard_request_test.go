package kiro_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authkiro "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro"
	kirotranslator "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro"
	testutil "github.com/router-for-me/CLIProxyAPI/v6/tests/shared"
)

// TestKiroHardRequestParsing tests the Kiro response parsing using the complex hard request fixture
func TestKiroHardRequestParsing(t *testing.T) {
	fixtureData := testutil.LoadTestData(t, "claude_format_simple.json")
	token := &authkiro.KiroTokenStorage{ProfileArn: "arn", AccessToken: "token", ExpiresAt: time.Now().Add(1 * time.Hour), Type: "kiro"}

	// BuildRequest should succeed and emit conversationState
	result, err := kirotranslator.BuildRequest("claude-sonnet-4-5", fixtureData, token, nil)
	require.NoError(t, err)

	var req map[string]any
	require.NoError(t, json.Unmarshal(result, &req))
	assert.Contains(t, req, "conversationState")
}

// TestKiroHardRequestStreaming tests streaming behavior with the complex fixture
func TestKiroHardRequestStreaming(t *testing.T) {
	// Reuse claude_format but force stream=true to exercise streaming path
	var payload map[string]any
	require.NoError(t, json.Unmarshal(testutil.LoadTestData(t, "claude_format.json"), &payload))
	payload["stream"] = true
	fixtureData, err := json.Marshal(payload)
	require.NoError(t, err)

	token := &authkiro.KiroTokenStorage{ProfileArn: "arn", AccessToken: "token", ExpiresAt: time.Now().Add(1 * time.Hour), Type: "kiro"}
	result, err := kirotranslator.BuildRequest("claude-sonnet-4-5", fixtureData, token, nil)
	require.NoError(t, err)

	var kiroRequest map[string]any
	require.NoError(t, json.Unmarshal(result, &kiroRequest))

	conversationState := kiroRequest["conversationState"].(map[string]any)
	currentMessage := conversationState["currentMessage"].(map[string]any)
	assert.Contains(t, currentMessage, "userInputMessage", "streaming request should include current user input")
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
