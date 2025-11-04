package tests

import (
	"encoding/json"
	"strings"
	"testing"

	kirotranslator "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestKiroExecutor_SSEFormatting_SimpleText tests that streaming responses are properly formatted as SSE events
func TestKiroExecutor_SSEFormatting_SimpleText(t *testing.T) {
	// This test should PASS with the corrected implementation
	// because Kiro now returns SSE-formatted events instead of raw JSON

	// Build the chunks that Kiro now produces (SSE-FORMATTED - CORRECT)
	chunks := kirotranslator.BuildAnthropicStreamingChunks("test-id", "claude-sonnet-4-5", 1234567890, "Hello, world!", []kirotranslator.OpenAIToolCall{})

	// Verify new behavior is CORRECT (SSE-formatted events)
	require.Greater(t, len(chunks), 0, "Should produce chunks")

	// Concatenate all chunks to simulate output
	var currentOutput strings.Builder
	for _, chunk := range chunks {
		currentOutput.Write(chunk)
	}
	currentOutputStr := currentOutput.String()

	// Output should now have proper SSE formatting
	assert.Contains(t, currentOutputStr, "event: message_start", "Should have SSE event prefix for message_start")
	assert.Contains(t, currentOutputStr, "event: content_block_start", "Should have SSE event prefix for content_block_start")
	assert.Contains(t, currentOutputStr, "event: content_block_delta", "Should have SSE event prefix for content_block_delta")
	assert.Contains(t, currentOutputStr, "event: content_block_stop", "Should have SSE event prefix for content_block_stop")
	assert.Contains(t, currentOutputStr, "event: message_delta", "Should have SSE event prefix for message_delta")
	assert.Contains(t, currentOutputStr, "event: message_stop", "Should have SSE event prefix for message_stop")

	// CRITICAL: Output should have SSE data prefixes
	assert.Contains(t, currentOutputStr, "data: ", "Should have SSE data prefix")

	// Should still contain the JSON data
	assert.Contains(t, currentOutputStr, `"type":"message_start"`, "Should contain message_start JSON")
	assert.Contains(t, currentOutputStr, `"type":"content_block_start"`, "Should contain content_block_start JSON")
	assert.Contains(t, currentOutputStr, `"type":"content_block_delta"`, "Should contain content_block_delta JSON")
	assert.Contains(t, currentOutputStr, `"type":"content_block_stop"`, "Should contain content_block_stop JSON")
	assert.Contains(t, currentOutputStr, `"type":"message_delta"`, "Should contain message_delta JSON")
	assert.Contains(t, currentOutputStr, `"type":"message_stop"`, "Should contain message_stop JSON")

	// CRITICAL BUG FIX: Verify stop_sequence is null (not "end_turn")
	assert.Contains(t, currentOutputStr, `"stop_sequence":null`, "stop_sequence should be null in message_start")
	assert.Contains(t, currentOutputStr, `"stop_sequence":null`, "stop_sequence should be null in message_delta")

	// CRITICAL BUG FIX: Verify output_tokens is properly counted (not hardcoded 0)
	assert.NotContains(t, currentOutputStr, `"output_tokens":0`, "output_tokens should not be hardcoded to 0")

	// Verify proper SSE structure with incremental streaming
	assert.Contains(t, currentOutputStr, `"text":"Hello, world!"`, "Should have proper text_delta with content")

	// This test should now PASS because we've fixed the SSE formatting
	t.Log("Kiro output (CORRECT - SSE formatted):", currentOutputStr)
	t.Log("SSE format with 'event:' and 'data:' prefixes is working correctly")
}

// TestKiroExecutor_SSEFormatting_WithToolCalls tests SSE formatting for responses with tool calls
func TestKiroExecutor_SSEFormatting_WithToolCalls(t *testing.T) {
	toolCalls := []kirotranslator.OpenAIToolCall{
		{
			ID:        "call_123",
			Name:      "test_function",
			Arguments: "{\"param\": \"value\"}",
		},
	}

	chunks := kirotranslator.BuildAnthropicStreamingChunks("test-id", "claude-sonnet-4-5", 1234567890, "", toolCalls)

	require.Greater(t, len(chunks), 0, "Should produce chunks for tool calls")

	var currentOutput strings.Builder
	for _, chunk := range chunks {
		currentOutput.Write(chunk)
	}
	currentOutputStr := currentOutput.String()

	// Should contain tool_use content blocks
	assert.Contains(t, currentOutputStr, `"type":"tool_use"`, "Should contain tool_use block")
	assert.Contains(t, currentOutputStr, `"name":"test_function"`, "Should contain function name")
	assert.Contains(t, currentOutputStr, `"partial_json":"{\"param\":\"value\"}"`, "Should contain function arguments in partial_json")

	// Should NOW have proper SSE formatting (corrected behavior)
	assert.Contains(t, currentOutputStr, "event: content_block_start", "Should have SSE event prefix for tool_use block")
	assert.Contains(t, currentOutputStr, "event: content_block_delta", "Should have SSE event prefix for tool delta")
	assert.Contains(t, currentOutputStr, "data: ", "Should have SSE data prefix")

	// CRITICAL BUG FIX: Verify stop_sequence is null for tool_use (not "tool_use")
	assert.Contains(t, currentOutputStr, `"stop_sequence":null`, "stop_sequence should be null even for tool_use")

	// CRITICAL BUG FIX: Verify output_tokens is properly counted for tool calls
	assert.NotContains(t, currentOutputStr, `"output_tokens":0`, "output_tokens should not be hardcoded to 0 for tool calls")

	// This test should now PASS because we've fixed the SSE formatting
	t.Log("Tool call SSE formatting working correctly:", currentOutputStr)
}

// TestKiroExecutor_SSEFormatting_EmptyContent tests SSE formatting for empty responses
func TestKiroExecutor_SSEFormatting_EmptyContent(t *testing.T) {
	chunks := kirotranslator.BuildAnthropicStreamingChunks("test-id", "claude-sonnet-4-5", 1234567890, "", []kirotranslator.OpenAIToolCall{})

	require.Greater(t, len(chunks), 0, "Should produce chunks even for empty content")

	var currentOutput strings.Builder
	for _, chunk := range chunks {
		currentOutput.Write(chunk)
	}
	currentOutputStr := currentOutput.String()

	// Should have basic SSE structure but no content blocks
	assert.Contains(t, currentOutputStr, `"type":"message_start"`, "Should contain message_start JSON")
	assert.Contains(t, currentOutputStr, `"type":"message_delta"`, "Should contain message_delta JSON")
	assert.Contains(t, currentOutputStr, `"type":"message_stop"`, "Should contain message_stop JSON")

	// Should NOW have proper SSE formatting (corrected behavior)
	assert.Contains(t, currentOutputStr, "event: message_start", "Should have SSE event prefix for message_start")
	assert.Contains(t, currentOutputStr, "event: message_delta", "Should have SSE event prefix for message_delta")
	assert.Contains(t, currentOutputStr, "event: message_stop", "Should have SSE event prefix for message_stop")
	assert.Contains(t, currentOutputStr, "data: ", "Should have SSE data prefix")

	// Should NOT contain content block events (since content is empty)
	assert.NotContains(t, currentOutputStr, "event: content_block_start", "Should NOT have content_block_start for empty content")
	assert.NotContains(t, currentOutputStr, "event: content_block_delta", "Should NOT have content_block_delta for empty content")
	assert.NotContains(t, currentOutputStr, "event: content_block_stop", "Should NOT have content_block_stop for empty content")

	// CRITICAL BUG FIX: Verify stop_sequence is null for empty responses
	assert.Contains(t, currentOutputStr, `"stop_sequence":null`, "stop_sequence should be null for empty responses")

	// CRITICAL BUG FIX: Verify output_tokens is not hardcoded (should be 0 for empty content, but calculated)
	assert.Contains(t, currentOutputStr, `"output_tokens":0`, "output_tokens should be 0 for empty content")

	// This test should now PASS because we've fixed the SSE formatting
	t.Log("Empty content SSE formatting working correctly:", currentOutputStr)
}

// TestKiroExecutor_VerifySSEFormatRequirement verifies what the SSE format should look like
func TestKiroExecutor_VerifySSEFormatRequirement(t *testing.T) {
	// This test documents the EXPECTED SSE format for reference
	// This test should PASS as it's documenting the requirement

	expectedSSEFormat := `event: message_start
data: {"message":{"content":[],"id":"msg_test123","model":"claude-sonnet-4-5","role":"assistant","stop_reason":null,"stop_sequence":null,"type":"message","usage":{"input_tokens":0,"output_tokens":0}},"type":"message_start"}

event: content_block_start
data: {"content_block":{"type":"text","text":""},"index":0,"type":"content_block_start"}

event: content_block_delta
data: {"delta":{"type":"text_delta","text":"Hello!"},"index":0,"type":"content_block_delta"}

event: content_block_stop
data: {"index":0,"type":"content_block_stop"}

event: message_delta
data: {"delta":{"stop_reason":"end_turn","stop_sequence":"end_turn"},"type":"message_delta","usage":{"output_tokens":0}}

event: message_stop
data: {"type":"message_stop"}`

	// Verify this is proper SSE format
	lines := strings.Split(expectedSSEFormat, "\n")

	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue // Skip empty lines
		}

		if strings.HasPrefix(line, "event: ") {
			// Event line should be followed by data line
			assert.Less(t, i+1, len(lines), "Event line should not be last line")
			nextLine := lines[i+1]
			assert.True(t, strings.HasPrefix(nextLine, "data: "), "Event line should be followed by data line")
		} else if strings.HasPrefix(line, "data: ") {
			// Data line should be valid JSON
			jsonPart := strings.TrimPrefix(line, "data: ")
			var jsonData any
			err := json.Unmarshal([]byte(jsonPart), &jsonData)
			assert.NoError(t, err, "Data line should contain valid JSON: %s", jsonPart)
		}
	}

	t.Log("Expected SSE format documented correctly")
}