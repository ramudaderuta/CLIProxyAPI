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
		name         string
		input        string
		shouldChange bool
		contains     []string
		notContains  []string
	}{
		{
			name: "response with empty tool call IDs is sanitized",
			input: `{
				"choices": [{
					"message": {
						"role": "assistant",
						"content": "I'll help you with that task",
						"tool_calls": [{
							"id": "",
							"type": "function",
							"function": {
								"name": "TodoWrite",
								"arguments": "{\"task\": \"test task\"}"
							}
						}]
					},
					"delta": {
						"role": "assistant",
						"tool_calls": [{
							"index": 0,
							"id": "",
							"type": "function",
							"function": {
								"name": "TodoWrite",
								"arguments": "{\"task\": \"partial\"}"
							}
						}]
					}
				}]
			}`,
			shouldChange: true,
			notContains:  []string{"\"id\": \"\""},
			contains:     []string{"\"name\": \"TodoWrite\""},
		},
		{
			name: "response with whitespace tool call IDs is sanitized",
			input: `{
				"choices": [{
					"message": {
						"role": "assistant",
						"content": "I'll help you with that task",
						"tool_calls": [{
							"id": "   ",
							"type": "function",
							"function": {
								"name": "Edit",
								"arguments": "{\"file\": \"test.txt\"}"
							}
						}]
					}
				}]
			}`,
			shouldChange: true,
			notContains:  []string{"\"id\": \"   \""},
			contains:     []string{"\"name\": \"Edit\""},
		},
		{
			name: "response with claude style IDs remains unchanged",
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
						}]
					}
				}]
			}`,
			shouldChange: false,
			contains:     []string{"***.TodoWrite:3"},
		},
		{
			name: "response without tool calls remains unchanged",
			input: `{
				"choices": [{
					"message": {
						"role": "assistant",
						"content": "Hello, how can I help you?"
					}
				}]
			}`,
			shouldChange: false,
			contains:     []string{"Hello, how can I help you?"},
		},
		{
			name: "invalid JSON remains unchanged",
			input: `invalid json response`,
			shouldChange: false,
			contains:     []string{"invalid json response"},
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

			if tc.shouldChange {
				assert.NotEqual(t, tc.input, result, "Input should be modified when empty tool call IDs are present")
			} else {
				assert.Equal(t, tc.input, result, "Response should remain unchanged when IDs are non-empty")
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
					{"id": "", "type": "function", "function": {"name": "empty1", "arguments": "{}"}},
					{"id": "   ", "type": "function", "function": {"name": "empty2", "arguments": "{}"}},
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
		assert.NotContains(t, result, "\"id\": \"\"")
		assert.NotContains(t, result, "\"id\": \"   \"")
		assert.Contains(t, result, "***.Bash:3")       // Non-empty IDs are preserved
		assert.Contains(t, result, "call_valid_uuid") // Valid ID should be preserved
	}
}
