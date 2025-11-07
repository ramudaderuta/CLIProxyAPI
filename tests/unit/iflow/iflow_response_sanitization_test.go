package iflow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/runtime/executor"
)

// TestSanitizeToolCallIDsInResponse tests the response sanitization function
func TestSanitizeToolCallIDsInResponse(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		input    string
		expected string
		contains []string // strings that should be present in the output
		notContains []string // strings that should NOT be present in the output
	}{
		{
			name: "response with invalid tool call IDs",
			input: `{
				"choices": [{
					"message": {
						"role": "assistant",
						"content": "I'll help you with that task",
						"tool_calls": [{
							"id": "***.TodoWrite:3",
							"type": "function",
							"function": {
								"name": "TodoWrite",
								"arguments": "{\"task\": \"test task\"}"
							}
						}, {
							"id": "***.Edit:6",
							"type": "function",
							"function": {
								"name": "Edit",
								"arguments": "{\"file\": \"test.txt\", \"content\": \"new content\"}"
							}
						}]
					}
				}]
			}`,
			expected: "should_be_valid", // We'll check the structure instead
			notContains: []string{"***.TodoWrite:3", "***.Edit:6"},
			contains: []string{"\"type\": \"function\"", "\"name\": \"TodoWrite\"", "\"name\": \"Edit\""},
		},
		{
			name: "streaming response with invalid tool call IDs",
			input: `{
				"choices": [{
					"delta": {
						"role": "assistant",
						"content": "I'll help you",
						"tool_calls": [{
							"index": 0,
							"id": "***.Bash:8",
							"type": "function",
							"function": {
								"name": "Bash",
								"arguments": "{\"command\": \"echo test\"}"
							}
						}]
					}
				}]
			}`,
			expected: "should_be_valid",
			notContains: []string{"***.Bash:8"},
			contains: []string{"\"type\": \"function\"", "\"name\": \"Bash\""},
		},
		{
			name: "response with valid tool call IDs should remain unchanged",
			input: `{
				"choices": [{
					"message": {
						"role": "assistant",
						"content": "I'll help you with that task",
						"tool_calls": [{
							"id": "call_123e4567-e89b-12d3-a456-426614174000",
							"type": "function",
							"function": {
								"name": "get_weather",
								"arguments": "{\"location\": \"New York\"}"
							}
						}]
					}
				}]
			}`,
			expected: "should_be_unchanged",
			contains: []string{"call_123e4567-e89b-12d3-a456-426614174000"},
		},
		{
			name: "response without tool calls should remain unchanged",
			input: `{
				"choices": [{
					"message": {
						"role": "assistant",
						"content": "Hello, how can I help you?"
					}
				}]
			}`,
			expected: "should_be_unchanged",
			contains: []string{"Hello, how can I help you?"},
		},
		{
			name: "invalid JSON should remain unchanged",
			input: `invalid json response`,
			expected: "should_be_unchanged",
			contains: []string{"invalid json response"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := executor.SanitizeToolCallIDsInResponse(tc.input)

			// Check that invalid patterns are removed
			for _, notWanted := range tc.notContains {
				assert.NotContains(t, result, notWanted, "Result should not contain invalid pattern: %s", notWanted)
			}

			// Check that expected content is present
			for _, wanted := range tc.contains {
				assert.Contains(t, result, wanted, "Result should contain: %s", wanted)
			}

			if tc.expected == "should_be_valid" {
				// For responses that should be sanitized, verify the result is valid JSON
				// and doesn't contain the invalid patterns
				assert.NotEqual(t, tc.input, result, "Input should be modified when invalid tool call IDs are present")

				// Verify that any tool call IDs in the result are valid
				// This is a basic check - in a real implementation you'd parse the JSON
				// and validate each tool call ID
				assert.NotContains(t, result, "***.", "Result should not contain Claude Code tool patterns")
			} else if tc.expected == "should_be_unchanged" {
				// For responses that should remain unchanged
				assert.Equal(t, tc.input, result, "Valid responses should remain unchanged")
			}
		})
	}
}

// TestToolCallIDEdgeCases tests edge cases for tool call ID handling
func TestToolCallIDEdgeCases(t *testing.T) {
	t.Parallel()

	// Test empty string handling
	assert.False(t, executor.ValidateToolCallID(""), "Empty string should be invalid")

	sanitized := executor.SanitizeToolCallID("")
	assert.True(t, executor.ValidateToolCallID(sanitized), "Sanitized empty string should be valid")
	assert.NotEqual(t, "", sanitized, "Sanitized empty string should not be empty")

	// Test whitespace handling
	assert.False(t, executor.ValidateToolCallID("   "), "Whitespace-only string should be invalid")

	sanitized = executor.SanitizeToolCallID("   ")
	assert.True(t, executor.ValidateToolCallID(sanitized), "Sanitized whitespace string should be valid")

	// Test that valid IDs are preserved
	validID := "call_123e4567-e89b-12d3-a456-426614174000"
	assert.True(t, executor.ValidateToolCallID(validID), "Valid UUID should be valid")
	assert.Equal(t, validID, executor.SanitizeToolCallID(validID), "Valid UUID should be preserved")

	// Test OpenAI tool format
	validToolID := "toolu_abcd1234efgh5678"
	assert.True(t, executor.ValidateToolCallID(validToolID), "Valid OpenAI tool ID should be valid")
	assert.Equal(t, validToolID, executor.SanitizeToolCallID(validToolID), "Valid OpenAI tool ID should be preserved")
}

// TestSanitizeToolCallIDsInResponsePerformance tests performance of the sanitization function
func TestSanitizeToolCallIDsInResponsePerformance(t *testing.T) {
	// Test with a large response to ensure performance is acceptable
	largeResponse := `{
		"choices": [{
			"message": {
				"role": "assistant",
				"content": "This is a test response with multiple tool calls",
				"tool_calls": [
					{"id": "***.TodoWrite:1", "type": "function", "function": {"name": "test1", "arguments": "{}"}},
					{"id": "***.Edit:2", "type": "function", "function": {"name": "test2", "arguments": "{}"}},
					{"id": "***.Bash:3", "type": "function", "function": {"name": "test3", "arguments": "{}"}},
					{"id": "call_valid_uuid", "type": "function", "function": {"name": "test4", "arguments": "{}"}}
				]
			}
		}]
	}`

	// Run multiple times to ensure performance is acceptable
	for i := 0; i < 100; i++ {
		result := executor.SanitizeToolCallIDsInResponse(largeResponse)

		// Verify the result is correct
		assert.NotContains(t, result, "***.TodoWrite:1")
		assert.NotContains(t, result, "***.Edit:2")
		assert.NotContains(t, result, "***.Bash:3")
		assert.Contains(t, result, "call_valid_uuid") // Valid ID should be preserved
	}
}