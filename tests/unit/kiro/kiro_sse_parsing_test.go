package kiro_test

import (
	"strings"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro"
	"github.com/stretchr/testify/assert"
)

// TestParseResponseFromEventStream_SSEParsing tests basic SSE parsing failures
// This test will fail because the current implementation has issues with SSE parsing
func TestParseResponseFromEventStream_SSEParsing(t *testing.T) {
	// Basic SSE parsing test - this will fail due to implementation issues
	t.Run("Basic_SSE_Parsing", func(t *testing.T) {
		stream := strings.Join([]string{
			`data: {"content":"Line 1"}`,
			`data: {"content":"Line 2"}`,
			`data: {"content":"Line 3"}`,
		}, "\n")

		content, calls := kiro.ParseResponse([]byte(stream))

		// This assertion will fail because the current implementation doesn't properly parse SSE streams
		expectedContent := "Line 1Line 2Line 3"
		assert.Equal(t, expectedContent, content, "SSE stream should be properly parsed and concatenated")

		// Verify no tool calls were parsed
		assert.Empty(t, calls, "No tool calls should be parsed from this stream")
	})

	// Test with tool calls in SSE format - this will fail due to implementation issues
	t.Run("SSE_With_Tool_Calls", func(t *testing.T) {
		stream := strings.Join([]string{
			`data: {"content":"Processing request"}`,
			`data: {"name":"lookup","toolUseId":"call-1","input":{"foo":"bar"}}`,
			`data: {"name":"lookup","toolUseId":"call-1","input":{"baz":1},"stop":true}`,
		}, "\n")

		content, calls := kiro.ParseResponse([]byte(stream))

		// This assertion will fail because the current implementation doesn't properly parse SSE streams with tool calls
		assert.Contains(t, content, "Processing request", "Content should be parsed from SSE stream")

		// This assertion will fail because tool calls are not properly parsed
		assert.Len(t, calls, 1, "Should parse one tool call from SSE stream")

		if len(calls) > 0 {
			// These assertions will fail due to improper tool call parsing
			assert.Equal(t, "lookup", calls[0].Name, "Tool call name should be parsed correctly")
			assert.Equal(t, "call-1", calls[0].ID, "Tool call ID should be parsed correctly")
			assert.Contains(t, calls[0].Arguments, "foo", "Tool call arguments should be parsed correctly")
			assert.Contains(t, calls[0].Arguments, "baz", "Tool call arguments should be parsed correctly")
		}
	})

	// Test with control delimiters in SSE - this will fail due to implementation issues
	t.Run("SSE_With_Control_Delimiters", func(t *testing.T) {
		raw := strings.Join([]string{
			`:message-typeevent{"content":"I don"}`,
			"\v:message-typeevent{\"content\":\"'t have access\"}",
			"\v:message-typeevent{\"content\":\" to data.\"}",
			"\v:metering-event{\"unit\":\"credit\",\"usage\":0.01}",
		}, "")

		content, calls := kiro.ParseResponse([]byte(raw))

		// This assertion will fail because control delimiters are not properly handled
		expectedContent := "I don't have access to data."
		assert.Equal(t, expectedContent, content, "Control delimiters should be properly handled in SSE parsing")

		// This assertion will fail if control characters cause issues
		assert.Empty(t, calls, "No tool calls should be parsed from control delimiter stream")
	})

	// Test with malformed SSE - this will fail due to error handling issues
	t.Run("Malformed_SSE", func(t *testing.T) {
		malformedStream := strings.Join([]string{
			`{"content":"Line 1"}`, // Missing "data: " prefix
			`data: {"content":"Line 2"}`,
			`{"name":"lookup","toolUseId":"call-1","input":{"foo":"bar"}}`, // Missing "data: " prefix
		}, "\n")

		content, _ := kiro.ParseResponse([]byte(malformedStream))

		// This assertion will fail because the current implementation may not handle malformed SSE correctly
		// We expect at least the properly formatted lines to be parsed
		assert.Contains(t, content, "Line 2", "Properly formatted SSE lines should still be parsed")

		// This assertion might fail if the entire parsing fails due to malformed input
		assert.GreaterOrEqual(t, len(content), 5, "Content should not be empty when partially valid SSE is present")
	})

	// Test with empty data lines - this will fail due to implementation issues
	t.Run("SSE_With_Empty_Data_Lines", func(t *testing.T) {
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

		// This assertion might fail if empty lines cause incorrect parsing
		assert.Empty(t, calls, "No tool calls should be parsed")
	})
}