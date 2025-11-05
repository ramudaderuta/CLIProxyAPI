package kiro_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro"
	"github.com/stretchr/testify/assert"
)

// TestSpecialCharacterPreservation_EdgeCase_ApostropheAtEnd tests that trailing apostrophes are not truncated
// This test will fail because the current implementation truncates trailing apostrophes
func TestSpecialCharacterPreservation_EdgeCase_ApostropheAtEnd(t *testing.T) {
	// Test case: Apostrophe at end of text should be preserved
	input := `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "That's all folks'"}}}}`
	expected := "That's all folks'"

	content, _ := kiro.ParseResponse([]byte(input))

	// This assertion will fail because trailing apostrophes are currently being truncated
	assert.Equal(t, expected, content, "Trailing apostrophe should be preserved")

	// Additional verification that the apostrophe is actually missing
	if !strings.HasSuffix(content, "'") && strings.HasSuffix(expected, "'") {
		t.Errorf("Trailing apostrophe was incorrectly truncated. Expected: %q, Got: %q", expected, content)
	}
}

// TestParseResponseFromEventStream_Failure tests basic SSE parsing failures
// This test will fail because the current implementation has issues with SSE parsing
func TestParseResponseFromEventStream_Failure(t *testing.T) {
	// Test case: Basic SSE parsing with multiple data lines
	stream := strings.Join([]string{
		`data: {"content":"Hello"}`,
		`data: {"content":" World"}`,
		`data: {"content":"!"}`,
	}, "\n")

	content, calls := kiro.ParseResponse([]byte(stream))

	// This assertion will fail because the current implementation doesn't properly parse SSE streams
	expectedContent := "Hello World!"
	assert.Equal(t, expectedContent, content, "SSE stream should be properly parsed and concatenated")

	// Verify no tool calls were parsed
	assert.Empty(t, calls, "No tool calls should be parsed from this stream")

	// Additional check to verify the content is correctly aggregated
	if len(content) < len("Hello World!") {
		t.Errorf("Content was not properly aggregated from SSE stream. Expected at least %d characters, got %d",
			len("Hello World!"), len(content))
	}
}

// TestKiroParseResponse_InvalidJSON_CriticalIssues tests JSON error handling
// This test will fail because the current implementation doesn't properly handle invalid JSON
func TestKiroParseResponse_InvalidJSON_CriticalIssues(t *testing.T) {
	// Test case: Invalid JSON should be handled gracefully
	invalidJSON := []string{
		`{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "Hello World"}}}`, // Missing closing brace
		`{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "Hello World"}}}}}`, // Extra closing brace
		`{"conversationState": {"currentMessage": {"assistantResponseMessage": {content: "Hello World"}}}}`, // Missing quotes
		`{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "Hello World"}}}}{`, // Trailing character
		``, // Empty string
		`{`, // Incomplete JSON
		`{"conversationState": {`, // Incomplete nested JSON
	}

	for i, invalid := range invalidJSON {
		t.Run("InvalidJSON_"+string(rune(i+'0')), func(t *testing.T) {
			// This test will fail because the current implementation may panic or not handle errors gracefully
			content, calls := kiro.ParseResponse([]byte(invalid))

			// We should not get a panic or unhandled error
			// Content should be empty or a default value when JSON is invalid
			// Tool calls should be empty when JSON is invalid

			// This assertion might fail if the implementation panics or returns unexpected values
			assert.NotNil(t, content, "Content should not be nil even for invalid JSON")

			// This assertion might fail if the implementation doesn't handle invalid JSON properly
			assert.Empty(t, calls, "Tool calls should be empty for invalid JSON")

			// If we get here without a panic, that's already a good sign
			// But we might still want to verify the error handling behavior
		})
	}
}

// TestParseResponseFromEventStream_MalformedSSE tests handling of malformed SSE data
// This test will fail because the current implementation doesn't properly handle malformed SSE
func TestParseResponseFromEventStream_MalformedSSE(t *testing.T) {
	// Test case: Malformed SSE with missing data prefix
	malformedStream := strings.Join([]string{
		`{"content":"Line 1"}`, // Missing "data: " prefix
		`data: {"content":"Line 2"}`,
		`{"name":"lookup","toolUseId":"call-1","input":{"foo":"bar"}}`, // Missing "data: " prefix
	}, "\n")

	content, calls := kiro.ParseResponse([]byte(malformedStream))

	// This assertion will fail because the current implementation may not handle malformed SSE correctly
	// We expect at least the properly formatted lines to be parsed
	assert.Contains(t, content, "Line 2", "Properly formatted SSE lines should still be parsed")

	// This might fail if the entire parsing fails due to malformed input
	assert.GreaterOrEqual(t, len(content), 5, "Content should not be empty when partially valid SSE is present")

	// This might fail if tool calls from malformed lines are incorrectly parsed
	assert.LessOrEqual(t, len(calls), 1, "At most one tool call should be parsed")
}

// TestParseResponseFromEventStream_EmptyData tests handling of empty data lines
// This test will fail because the current implementation might not handle empty data correctly
func TestParseResponseFromEventStream_EmptyData(t *testing.T) {
	// Test case: SSE with empty data lines
	emptyDataStream := strings.Join([]string{
		`data: {"content":"Start"}`,
		`data: `, // Empty data line
		`data: {"content":"End"}`,
		``, // Empty line
		`data: {"content":"."}`,
	}, "\n")

	content, calls := kiro.ParseResponse([]byte(emptyDataStream))

	// This assertion will fail if empty data lines cause parsing issues
	expectedContent := "StartEnd."
	assert.Equal(t, expectedContent, content, "Empty data lines should be skipped")

	// This might fail if empty lines cause incorrect parsing
	assert.Empty(t, calls, "No tool calls should be parsed")

	// Additional verification
	if len(content) != len(expectedContent) {
		t.Errorf("Content length mismatch. Expected: %d, Got: %d", len(expectedContent), len(content))
	}
}

// TestKiroParseResponse_JSONWithSpecialCharacters tests handling of JSON with special characters
// This test will fail because the current implementation might not properly handle special characters
func TestKiroParseResponse_JSONWithSpecialCharacters(t *testing.T) {
	// Test case: JSON with various special characters that might cause parsing issues
	specialCharJSON := `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "Special chars: \n\t\r\"'\\/"}}}}`

	content, calls := kiro.ParseResponse([]byte(specialCharJSON))

	// This assertion will fail if special characters are not properly handled
	assert.Contains(t, content, "Special chars:", "Content should contain the special characters text")

	// This might fail if escape sequences cause issues
	assert.NotEmpty(t, content, "Content should not be empty")

	// This might fail if parsing breaks due to special characters
	assert.Empty(t, calls, "No tool calls should be parsed")

	// Try to validate that the result is still valid for JSON marshaling
	testStruct := map[string]interface{}{
		"test_content": content,
	}

	// This might fail if content contains invalid UTF-8 or other issues
	_, err := json.Marshal(testStruct)
	assert.NoError(t, err, "Content should be JSON marshalable even with special characters")
}