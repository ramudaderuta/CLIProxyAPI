package kiro_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	testutil "github.com/router-for-me/CLIProxyAPI/v6/tests/shared"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro"
)

// ============================================================================
// Critical Bug Regression Tests
// ============================================================================

// TestKiroBugReproduction_ContentClipping reproduces the exact bug described in kiro_response_bug.txt
// where content gets clipped to ".txt"}\n\nTool usage" instead of preserving full content + tool_calls
func TestKiroBugReproduction_ContentClipping(t *testing.T) {
	// Load the bug reproduction fixture
	fixtureData := testutil.LoadTestData(t, "nonstream/bug_reproduction.json")

	// Parse the response using current implementation
	content, toolCalls := kiro.ParseResponse(fixtureData)

	// Build OpenAI-compatible response
	response, err := kiro.BuildOpenAIChatCompletionPayload(
		"claude-sonnet-4-5-20250929",
		content,
		toolCalls,
		100, 50, // token counts
	)
	require.NoError(t, err, "Should build OpenAI response without error")

	// Parse the generated response to validate structure
	responseJSON := gjson.ParseBytes(response)
	actualContent := responseJSON.Get("choices.0.message.content").String()
	actualToolCalls := responseJSON.Get("choices.0.message.tool_calls").Array()

	// CRITICAL: The bug manifests as content being clipped to ".txt\"}\n\nTool usage"
	// This should NOT happen - full content should be preserved
	assert.NotContains(t, actualContent, ".txt\"}\n\nTool usage",
		"Content should not be clipped with artifact")

	// Expected content should be the full message, not clipped
	expectedContent := "Please save the following content to a file named example.txt with this text: Hello world, this is a test file that contains important information."
	assert.Equal(t, expectedContent, actualContent,
		"Full content should be preserved without clipping")

	// Should have proper tool_calls structure, not embedded in content
	assert.NotEmpty(t, actualToolCalls, "Should have tool calls in proper structure")

	// Validate tool call structure
	if len(actualToolCalls) > 0 {
		toolCall := actualToolCalls[0]
		assert.Equal(t, "t1", toolCall.Get("id").String(), "Tool call ID should match")
		assert.Equal(t, "write_file", toolCall.Get("function.name").String(), "Tool name should match")

		expectedArgs := `{"path":"example.txt","content":"Hello world, this is a test file that contains important information."}`
		assert.JSONEq(t, expectedArgs, toolCall.Get("function.arguments").String(),
			"Tool arguments should match expected")
	}

	// Additional validation: content should not contain JSON artifacts
	assert.NotContains(t, actualContent, "}", "Content should not contain JSON closing braces")
	assert.NotContains(t, actualContent, ".txt\"}\n\nTool usage", "Content should not contain 'Tool usage' artifact")
}

// TestKiroBugReproduction_DelimiterSafety tests content with special characters that trigger clipping
func TestKiroBugReproduction_DelimiterSafety(t *testing.T) {
	// Test case with content containing .txt and } characters that trigger the bug
	testCases := []struct {
		name     string
		content  string
		toolName string
		toolArgs map[string]any
	}{
		{
			name:    "content_with_txt_and_brace",
			content: "Save this to config.txt and update settings.json",
			toolName: "write_file",
			toolArgs: map[string]any{
				"path": "config.txt",
				"content": "settings: { debug: true }",
			},
		},
		{
			name:    "content_with_tool_usage_phrase",
			content: "Here's the Tool usage guide for developers",
			toolName: "create_documentation",
			toolArgs: map[string]any{
				"title": "Tool Usage Guide",
			},
		},
		{
			name:    "content_with_json_like_structure",
			content: "The configuration looks like: { \"file\": \"test.txt\", \"enabled\": true }",
			toolName: "write_config",
			toolArgs: map[string]any{
				"config": map[string]any{
					"file": "test.txt",
					"enabled": true,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test fixture
			fixture := map[string]any{
				"conversationState": map[string]any{
					"currentMessage": map[string]any{
						"assistantResponseMessage": map[string]any{
							"content": tc.content,
						},
						"toolUse": map[string]any{
							"toolUseId": "test_tool",
							"name":      tc.toolName,
							"input":     tc.toolArgs,
						},
					},
				},
			}

			fixtureData, err := json.Marshal(fixture)
			require.NoError(t, err)

			// Parse response
			content, toolCalls := kiro.ParseResponse(fixtureData)

			// Build OpenAI response
			response, err := kiro.BuildOpenAIChatCompletionPayload(
				"claude-sonnet-4-5-20250929",
				content,
				toolCalls,
				50, 25,
			)
			require.NoError(t, err)

			// Validate content preservation
			responseJSON := gjson.ParseBytes(response)
			actualContent := responseJSON.Get("choices.0.message.content").String()

			assert.Equal(t, tc.content, actualContent,
				"Content should be preserved exactly without clipping")

			// Should not contain clipping artifacts
			assert.NotContains(t, actualContent, ".txt\"}", "Should not have txt clipping artifact")
			// Only check for the specific artifact pattern, not legitimate use of "Tool usage" phrase
			assert.NotContains(t, actualContent, ".txt\"}\n\nTool usage", "Should not have tool usage artifact")

			// Should have proper tool calls
			actualToolCalls := responseJSON.Get("choices.0.message.tool_calls").Array()
			assert.NotEmpty(t, actualToolCalls, "Should have tool calls in proper structure")
		})
	}
}

// TestKiroBugReproduction_TextOnlyNoClipping ensures text-only responses don't get clipped
func TestKiroBugReproduction_TextOnlyNoClipping(t *testing.T) {
	// Load text-only fixture using centralized test data loader
	fixtureData := testutil.LoadTestData(t, "nonstream/text_only.json")

	// Parse response
	content, toolCalls := kiro.ParseResponse(fixtureData)

	// Should have content but no tool calls
	assert.Equal(t, "Hello world", content, "Should extract text content correctly")
	assert.Empty(t, toolCalls, "Should have no tool calls for text-only response")

	// Build OpenAI response
	response, err := kiro.BuildOpenAIChatCompletionPayload(
		"claude-sonnet-4-5-20250929",
		content,
		toolCalls,
		10, 5,
	)
	require.NoError(t, err)

	// Validate response structure
	responseJSON := gjson.ParseBytes(response)
	actualContent := responseJSON.Get("choices.0.message.content").String()
	actualToolCalls := responseJSON.Get("choices.0.message.tool_calls").Array()

	assert.Equal(t, "Hello world", actualContent, "Content should be preserved")
	assert.Empty(t, actualToolCalls, "Should have no tool_calls field for text-only")
}

// TestKiroBugReproduction_TextThenToolProperSeparation tests proper text + tool separation
func TestKiroBugReproduction_TextThenToolProperSeparation(t *testing.T) {
	// Load text + tool fixture using centralized test data loader
	fixtureData := testutil.LoadTestData(t, "nonstream/text_then_tool.json")

	// Parse response
	content, toolCalls := kiro.ParseResponse(fixtureData)

	// Should have both content and tool calls
	assert.Equal(t, "Save file", content, "Should extract text content")
	assert.Len(t, toolCalls, 1, "Should have one tool call")

	// Validate tool call structure
	assert.Equal(t, "t1", toolCalls[0].ID, "Tool call ID should match")
	assert.Equal(t, "write_file", toolCalls[0].Name, "Tool name should match")
	assert.JSONEq(t, `{"path":"a.txt"}`, toolCalls[0].Arguments, "Tool arguments should match")

	// Build OpenAI response
	response, err := kiro.BuildOpenAIChatCompletionPayload(
		"claude-sonnet-4-5-20250929",
		content,
		toolCalls,
		20, 10,
	)
	require.NoError(t, err)

	// Validate response structure
	responseJSON := gjson.ParseBytes(response)
	actualContent := responseJSON.Get("choices.0.message.content").String()
	actualToolCalls := responseJSON.Get("choices.0.message.tool_calls").Array()

	assert.Equal(t, "Save file", actualContent, "Text content should be preserved")
	assert.Len(t, actualToolCalls, 1, "Should have one tool call in response")

	// Validate tool call in response
	toolCall := actualToolCalls[0]
	assert.Equal(t, "t1", toolCall.Get("id").String(), "Tool call ID should match")
	assert.Equal(t, "write_file", toolCall.Get("function.name").String(), "Tool name should match")
	assert.JSONEq(t, `{"path":"a.txt"}`, toolCall.Get("function.arguments").String(),
		"Tool arguments should match")
}

// ============================================================================
// Streaming Bug Regression Tests
// ============================================================================

// TestKiroBugReproduction_StreamingDotTxtTruncation tests streaming format bug
func TestKiroBugReproduction_StreamingDotTxtTruncation(t *testing.T) {
	// This test reproduces the exact bug from kiro_response_bug.txt in streaming format
	// The content should not be truncated at ".txt" and should not have "}\n\nTool usage" artifacts

	input := `{"content":"Here is your file saved as document.txt"}
{"name":"write_file","toolUseId":"t1","input":{"path":"document.txt"},"stop":true}`

	content, toolCalls := kiro.ParseResponse([]byte(input))

	// Debug: Show what the current implementation actually produces
	t.Logf("Current implementation produces content: %q", content)
	t.Logf("Current implementation produces %d tool calls", len(toolCalls))
	for i, call := range toolCalls {
		t.Logf("Tool call %d: ID=%s, Name=%s, Args=%s", i, call.ID, call.Name, call.Arguments)
	}

	// Based on the bug report, we expect the current implementation to fail
	// The bug shows content becomes: ".txt\"}\n\nTool usage"
	// If the current implementation works correctly, this test should be updated

	// For now, let's check that we get reasonable results
	assert.Contains(t, content, "document.txt", "Content should preserve the complete filename")
	assert.NotEqual(t, ".txt\"}\n\nTool usage", content, "Content should not be just the artifact")

	// Should have proper tool call
	require.Len(t, toolCalls, 1, "Should have one tool call")
	assert.Equal(t, "write_file", toolCalls[0].Name, "Tool call name should be correct")
	assert.Equal(t, "t1", toolCalls[0].ID, "Tool call ID should be preserved")
}

// ============================================================================
// Edge Case Bug Tests
// ============================================================================

// TestKiroBugReproduction_EmptyResponseHandling tests empty response handling
func TestKiroBugReproduction_EmptyResponseHandling(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		expectPanic bool
	}{
		{
			name:        "completely_empty",
			input:       "",
			expectPanic: false,
		},
		{
			name:        "only_whitespace",
			input:       "   \n\t  ",
			expectPanic: false,
		},
		{
			name:        "invalid_json_only",
			input:       "not json at all",
			expectPanic: false,
		},
		{
			name:        "malformed_json",
			input:       `{"incomplete": json`,
			expectPanic: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.expectPanic {
				assert.Panics(t, func() {
					kiro.ParseResponse([]byte(tc.input))
				}, "Should panic for malformed input")
			} else {
				assert.NotPanics(t, func() {
					content, toolCalls := kiro.ParseResponse([]byte(tc.input))
					// Should return some result, even if empty
					_ = content
					_ = toolCalls
				}, "Should not panic for input: %s", tc.input)
			}
		})
	}
}

// TestKiroBugReproduction_ToolCallArgumentEscaping tests argument escaping issues
func TestKiroBugReproduction_ToolCallArgumentEscaping(t *testing.T) {
	testCases := []struct {
		name     string
		content  string
		toolName string
		toolArgs map[string]any
	}{
		{
			name:    "args_with_quotes",
			content: "Save file with quotes",
			toolName: "write_file",
			toolArgs: map[string]any{
				"path": "file.txt",
				"content": "He said \"Hello world\" and left",
			},
		},
		{
			name:    "args_with_newlines",
			content: "Save multiline file",
			toolName: "write_file",
			toolArgs: map[string]any{
				"path": "multiline.txt",
				"content": "Line 1\nLine 2\nLine 3",
			},
		},
		{
			name:    "args_with_special_chars",
			content: "Save file with special characters",
			toolName: "write_file",
			toolArgs: map[string]any{
				"path": "special.txt",
				"content": "Special chars: \\ \" \n \t {} []",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test fixture
			fixture := map[string]any{
				"conversationState": map[string]any{
					"currentMessage": map[string]any{
						"assistantResponseMessage": map[string]any{
							"content": tc.content,
						},
						"toolUse": map[string]any{
							"toolUseId": "test_tool",
							"name":      tc.toolName,
							"input":     tc.toolArgs,
						},
					},
				},
			}

			fixtureData, err := json.Marshal(fixture)
			require.NoError(t, err)

			// Parse response
			content, toolCalls := kiro.ParseResponse(fixtureData)

			// Should preserve content
			assert.Equal(t, tc.content, content, "Content should be preserved")

			// Should extract tool calls
			require.Len(t, toolCalls, 1, "Should have one tool call")
			assert.Equal(t, tc.toolName, toolCalls[0].Name, "Tool name should match")
			assert.Equal(t, "test_tool", toolCalls[0].ID, "Tool ID should match")

			// Arguments should be valid JSON
			assert.NotEmpty(t, toolCalls[0].Arguments, "Tool arguments should not be empty")
			var args map[string]any
			err = json.Unmarshal([]byte(toolCalls[0].Arguments), &args)
			assert.NoError(t, err, "Tool arguments should be valid JSON")
		})
	}
}

// ============================================================================
// Performance and Stress Bug Tests
// ============================================================================

// TestKiroBugReproduction_LargeResponseHandling tests large response handling
func TestKiroBugReproduction_LargeResponseHandling(t *testing.T) {
	// Generate progressively larger content to find the threshold
	testCases := []struct {
		name   string
		lines  int
	}{
		{"small", 100},
		{"medium", 1000},
		{"large", 5000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var content strings.Builder
			for i := 0; i < tc.lines; i++ {
				content.WriteString("This is line ")
				content.WriteString(fmt.Sprintf("%04d", i))
				content.WriteString(" of the test content.")
				if i < tc.lines-1 {
					content.WriteString(" ")
				}
			}

			testContent := content.String()
			t.Logf("Testing with %d lines, content length: %d", tc.lines, len(testContent))

			// Create response fixture
			fixture := map[string]any{
				"conversationState": map[string]any{
					"currentMessage": map[string]any{
						"assistantResponseMessage": map[string]any{
							"content": testContent,
						},
					},
				},
			}

			fixtureData, err := json.Marshal(fixture)
			require.NoError(t, err)

			// Parse response
			parsedContent, toolCalls := kiro.ParseResponse(fixtureData)

			assert.Equal(t, testContent, parsedContent, "Content should be preserved completely for %s test", tc.name)
			assert.Empty(t, toolCalls, "Should have no tool calls for text-only response")

			// Build OpenAI response
			response, err := kiro.BuildOpenAIChatCompletionPayload(
				"claude-sonnet-4-5-20250929",
				parsedContent,
				toolCalls,
				int64(len(testContent)/4), int64(len(testContent)/4), // Approximate token counts
			)
			require.NoError(t, err, "Should build response without error for %s test", tc.name)

			// Validate response structure
			responseJSON := gjson.ParseBytes(response)
			actualContent := responseJSON.Get("choices.0.message.content").String()

			t.Logf("Expected length: %d, Actual length: %d for %s test", len(testContent), len(actualContent), tc.name)

			if len(testContent) != len(actualContent) {
				t.Logf("Content mismatch detected in %s test", tc.name)
				t.Logf("Expected prefix: %q", testContent[:min(100, len(testContent))])
				t.Logf("Actual prefix: %q", actualContent[:min(100, len(actualContent))])
				t.Logf("Expected suffix: %q", testContent[max(0, len(testContent)-100):])
				t.Logf("Actual suffix: %q", actualContent[max(0, len(actualContent)-100):])
			}

			assert.Equal(t, testContent, actualContent, "Content should be preserved in OpenAI response for %s test", tc.name)
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}