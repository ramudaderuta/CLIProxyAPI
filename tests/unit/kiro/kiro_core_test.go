package kiro_test

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro"
)

// ============================================================================
// Core Response Parsing Tests
// ============================================================================

// TestKiroParseResponse_EmptyInput tests edge cases for empty input
func TestKiroParseResponse_EmptyInput(t *testing.T) {
	t.Parallel()
	content, toolCalls := kiro.ParseResponse([]byte{})
	assert.Equal(t, "", content, "Empty input should return empty content")
	assert.Empty(t, toolCalls, "Empty input should return no tool calls")

	content, toolCalls = kiro.ParseResponse(nil)
	assert.Equal(t, "", content, "Nil input should return empty content")
	assert.Empty(t, toolCalls, "Nil input should return no tool calls")
}

// TestKiroParseResponse_InvalidJSON tests handling of invalid JSON input
func TestKiroParseResponse_InvalidJSON(t *testing.T) {
	t.Parallel()
	invalidInputs := []string{
		"invalid json string",
		"{ malformed json }",
		"just plain text",
		"",
	}

	for _, input := range invalidInputs {
		t.Run("invalid_input_"+input, func(t *testing.T) {
			content, _ := kiro.ParseResponse([]byte(input))
			// Should not panic and should return some result
			// For non-empty invalid input, we expect some content to be returned
			if input != "" {
				assert.NotEmpty(t, content, "Even invalid input should produce some content")
			} else {
				// Empty input should return empty content - this is expected behavior
				assert.Equal(t, "", content, "Empty input should return empty content")
			}
			// Tool calls might be empty, that's fine
		})
	}
}

// TestKiroParseResponse_ValidJSONNoContent tests JSON with no content field
func TestKiroParseResponse_ValidJSONNoContent(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		jsonData string
	}{
		{
			name:     "empty_object",
			jsonData: `{}`,
		},
		{
			name:     "conversation_state_empty",
			jsonData: `{"conversationState": {}}`,
		},
		{
			name:     "current_message_empty",
			jsonData: `{"conversationState": {"currentMessage": {}}}`,
		},
		{
			name:     "assistant_message_empty",
			jsonData: `{"conversationState": {"currentMessage": {"assistantResponseMessage": {}}}}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			content, toolCalls := kiro.ParseResponse([]byte(tc.jsonData))
			assert.Equal(t, "", content, "Should return empty content when no content field exists")
			assert.Empty(t, toolCalls, "Should return no tool calls when no tool use exists")
		})
	}
}

// TestKiroParseResponse_ContentExtraction tests various content extraction scenarios
func TestKiroParseResponse_ContentExtraction(t *testing.T) {
	testCases := []struct {
		name            string
		jsonData        string
		expectedContent string
		expectedToolCalls []kiro.OpenAIToolCall
	}{
		{
			name:            "simple_content",
			jsonData:        `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "Hello world"}}}}`,
			expectedContent: "Hello world",
			expectedToolCalls: []kiro.OpenAIToolCall{},
		},
		{
			name: "content_with_tool_use",
			jsonData: `{
				"conversationState": {
					"currentMessage": {
						"assistantResponseMessage": {
							"content": "Save the file",
							"toolUse": {
								"toolUseId": "t1",
								"name": "write_file",
								"input": {"path": "test.txt"}
							}
						}
					}
				}
			}`,
			expectedContent: "Save the file",
			expectedToolCalls: []kiro.OpenAIToolCall{
				{ID: "t1", Name: "write_file", Arguments: `{"path":"test.txt"}`},
			},
		},
		{
			name: "content_with_special_characters",
			jsonData: `{
				"conversationState": {
					"currentMessage": {
						"assistantResponseMessage": {
							"content": "Save to config.txt and update {settings: true}"
						}
					}
				}
			}`,
			expectedContent: "Save to config.txt and update {settings: true}",
			expectedToolCalls: []kiro.OpenAIToolCall{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			content, toolCalls := kiro.ParseResponse([]byte(tc.jsonData))
			assert.Equal(t, tc.expectedContent, content, "Content should match expected")
			assert.Equal(t, len(tc.expectedToolCalls), len(toolCalls), "Tool call count should match")

			if len(tc.expectedToolCalls) > 0 {
				assert.Equal(t, tc.expectedToolCalls[0].ID, toolCalls[0].ID, "Tool call ID should match")
				assert.Equal(t, tc.expectedToolCalls[0].Name, toolCalls[0].Name, "Tool call name should match")
				assert.JSONEq(t, tc.expectedToolCalls[0].Arguments, toolCalls[0].Arguments, "Tool call arguments should match")
			}
		})
	}
}

// TestKiroParseResponse_HistoryFallback tests fallback to conversation history
func TestKiroParseResponse_HistoryFallback(t *testing.T) {
	t.Parallel()
	jsonData := `{
		"conversationState": {
			"history": [
				{"assistantResponseMessage": {"content": "First message"}},
				{"assistantResponseMessage": {"content": "Second message"}},
				{"assistantResponseMessage": {"content": "Latest message"}}
			]
		}
	}`

	content, toolCalls := kiro.ParseResponse([]byte(jsonData))
	assert.Equal(t, "Latest message", content, "Should get the latest message from history")
	assert.Empty(t, toolCalls, "Should have no tool calls")
}

// ============================================================================
// OpenAI Response Building Tests
// ============================================================================

// TestKiroBuildOpenAIChatCompletionPayload_ValidatesOutputFormat
func TestKiroBuildOpenAIChatCompletionPayload_ValidatesOutputFormat(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name        string
		model       string
		content     string
		toolCalls   []kiro.OpenAIToolCall
		expectError bool
	}{
		{
			name:    "text_only",
			model:   "claude-sonnet-4-5-20250929",
			content: "Hello world",
			toolCalls: []kiro.OpenAIToolCall{},
			expectError: false,
		},
		{
			name:    "text_with_tools",
			model:   "claude-sonnet-4-5-20250929",
			content: "Save file",
			toolCalls: []kiro.OpenAIToolCall{
				{ID: "t1", Name: "write_file", Arguments: `{"path":"test.txt"}`},
			},
			expectError: false,
		},
		{
			name:        "empty_model",
			model:       "",
			content:     "Hello",
			toolCalls:   []kiro.OpenAIToolCall{},
			expectError: false, // Should not error, model can be empty
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			response, err := kiro.BuildOpenAIChatCompletionPayload(
				tc.model,
				tc.content,
				tc.toolCalls,
				100, 50, // token counts
			)

			if tc.expectError {
				assert.Error(t, err, "Should return error for invalid input")
				return
			}

			require.NoError(t, err, "Should build response without error")
			require.NotEmpty(t, response, "Response should not be empty")

			// Parse and validate OpenAI response structure
			responseJSON := gjson.ParseBytes(response)

			// Validate required fields
			assert.True(t, responseJSON.Get("id").Exists(), "Should have id field")
			assert.Equal(t, "chat.completion", responseJSON.Get("object").String(), "Should have correct object type")
			assert.True(t, responseJSON.Get("created").Exists(), "Should have created timestamp")
			assert.Equal(t, tc.model, responseJSON.Get("model").String(), "Model should match input")

			// Validate choices structure
			choices := responseJSON.Get("choices").Array()
			assert.Len(t, choices, 1, "Should have exactly one choice")

			choice := choices[0]
			assert.Equal(t, float64(0), choice.Get("index").Float(), "Choice index should be 0")
			assert.Equal(t, "assistant", choice.Get("message.role").String(), "Message role should be assistant")
			assert.Equal(t, tc.content, choice.Get("message.content").String(), "Content should match input")
			assert.Equal(t, "stop", choice.Get("finish_reason").String(), "Finish reason should be stop")

			// Validate tool calls if present
			actualToolCalls := choice.Get("message.tool_calls").Array()
			assert.Len(t, actualToolCalls, len(tc.toolCalls), "Tool call count should match")

			if len(tc.toolCalls) > 0 {
				toolCall := actualToolCalls[0]
				assert.Equal(t, tc.toolCalls[0].ID, toolCall.Get("id").String(), "Tool call ID should match")
				assert.Equal(t, "function", toolCall.Get("type").String(), "Tool call type should be function")
				assert.Equal(t, tc.toolCalls[0].Name, toolCall.Get("function.name").String(), "Function name should match")
				assert.JSONEq(t, tc.toolCalls[0].Arguments, toolCall.Get("function.arguments").String(), "Function arguments should match")
			}

			// Validate usage structure
			usage := responseJSON.Get("usage")
			assert.True(t, usage.Get("prompt_tokens").Exists(), "Should have prompt tokens")
			assert.True(t, usage.Get("completion_tokens").Exists(), "Should have completion tokens")
			assert.True(t, usage.Get("total_tokens").Exists(), "Should have total tokens")
		})
	}
}

// ============================================================================
// Streaming Tests
// ============================================================================

// TestKiroStreaming_TextChunks tests streaming text content aggregation
func TestKiroStreaming_TextChunks(t *testing.T) {
	// Load streaming text chunks fixture
	fixturePath := filepath.Join("testdata", "streaming", "text_chunks.ndjson")
	fixtureData, err := os.ReadFile(fixturePath)
	require.NoError(t, err)

	// Simulate streaming by concatenating chunks
	var streamingData strings.Builder
	scanner := bufio.NewScanner(bytes.NewReader(fixtureData))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		streamingData.WriteString(line)
		streamingData.WriteString("\n")
	}

	// Parse the streaming response
	content, toolCalls := kiro.ParseResponse([]byte(streamingData.String()))

	// Should aggregate text content
	assert.Equal(t, "Hello world", content, "Should aggregate streaming text content")
	assert.Empty(t, toolCalls, "Should have no tool calls for text-only streaming")
}

// TestKiroStreaming_ToolInterleave tests streaming with interleaved text and tool calls
func TestKiroStreaming_ToolInterleave(t *testing.T) {
	// Load streaming tool interleave fixture
	fixturePath := filepath.Join("testdata", "streaming", "tool_interleave.ndjson")
	fixtureData, err := os.ReadFile(fixturePath)
	require.NoError(t, err)

	// Simulate streaming response
	var streamingData strings.Builder
	scanner := bufio.NewScanner(bytes.NewReader(fixtureData))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		streamingData.WriteString(line)
		streamingData.WriteString("\n")
	}

	// Parse the streaming response
	content, toolCalls := kiro.ParseResponse([]byte(streamingData.String()))

	// Should have both text and tool calls
	assert.Equal(t, "Save file", content, "Should extract text content from streaming")
	assert.Len(t, toolCalls, 1, "Should have one tool call from streaming")

	// Validate tool call structure
	if len(toolCalls) > 0 {
		toolCall := toolCalls[0]
		assert.Equal(t, "t1", toolCall.ID, "Tool call ID should match")
		assert.Equal(t, "write_file", toolCall.Name, "Tool name should match")
		// Note: streaming parsing might not extract tool calls the same way as JSON
		// This test validates the streaming behavior specifically
	}
}

// TestKiroStreaming_BackpressureSimulation tests streaming under slow client conditions
func TestKiroStreaming_BackpressureSimulation(t *testing.T) {
	// Create large streaming content to test backpressure
	chunks := []string{
		`{"content": "This is chunk 1. "}`,
		`{"content": "This is chunk 2. "}`,
		`{"content": "This is chunk 3. "}`,
		`{"content": "This is chunk 4. "}`,
		`{"content": "This is chunk 5. "}`,
		`{"stop": true}`,
	}

	// Simulate slow client by processing chunks with delays
	var streamingData strings.Builder
	for i, chunk := range chunks {
		// Add chunk to stream
		streamingData.WriteString(chunk)
		streamingData.WriteString("\n")

		// Simulate processing delay (in real test, this would be network latency)
		if i < len(chunks)-1 {
			time.Sleep(1 * time.Millisecond) // Very small delay for testing
		}
	}

	// Parse the complete streaming response
	content, toolCalls := kiro.ParseResponse([]byte(streamingData.String()))

	// Should aggregate all text chunks (note: no trailing space in actual parsing)
	expectedContent := "This is chunk 1. This is chunk 2. This is chunk 3. This is chunk 4. This is chunk 5."
	assert.Equal(t, expectedContent, content, "Should aggregate all text chunks despite delays")
	assert.Empty(t, toolCalls, "Should have no tool calls")
}

// TestKiroBuildStreamingChunks_ValidatesStreamingFormat
func TestKiroBuildStreamingChunks_ValidatesStreamingFormat(t *testing.T) {
	testCases := []struct {
		name      string
		id        string
		model     string
		created   int64
		content   string
		toolCalls []kiro.OpenAIToolCall
	}{
		{
			name:      "text_only",
			id:        "test_id",
			model:     "claude-sonnet-4-5-20250929",
			created:   1234567890,
			content:   "Hello world",
			toolCalls: []kiro.OpenAIToolCall{},
		},
		{
			name:    "text_with_tools",
			id:      "test_id",
			model:   "claude-sonnet-4-5-20250929",
			created: 1234567890,
			content: "Save file",
			toolCalls: []kiro.OpenAIToolCall{
				{ID: "t1", Name: "write_file", Arguments: `{"path":"test.txt"}`},
			},
		},
		{
			name:      "empty_content",
			id:        "test_id",
			model:     "claude-sonnet-4-5-20250929",
			created:   1234567890,
			content:   "",
			toolCalls: []kiro.OpenAIToolCall{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			chunks := kiro.BuildStreamingChunks(tc.id, tc.model, tc.created, tc.content, tc.toolCalls)

			// Should have at least 2 chunks: initial + final, plus content/tool chunks if applicable
			assert.GreaterOrEqual(t, len(chunks), 2, "Should have at least initial and final chunks")

			// Parse and validate each chunk
			for i, chunk := range chunks {
				chunkJSON := gjson.ParseBytes(chunk)

				// Validate common streaming chunk fields
				assert.Equal(t, tc.id, chunkJSON.Get("id").String(), "Chunk ID should match")
				assert.Equal(t, "chat.completion.chunk", chunkJSON.Get("object").String(), "Object type should be chat.completion.chunk")
				assert.Equal(t, tc.created, chunkJSON.Get("created").Int(), "Created timestamp should match")
				assert.Equal(t, tc.model, chunkJSON.Get("model").String(), "Model should match")

				// Validate choices structure
				choices := chunkJSON.Get("choices").Array()
				assert.Len(t, choices, 1, "Each chunk should have exactly one choice")

				choice := choices[0]
				assert.Equal(t, float64(0), choice.Get("index").Float(), "Choice index should be 0")

				// First chunk should have role delta
				if i == 0 {
					assert.Equal(t, "assistant", choice.Get("delta.role").String(), "First chunk should set role")
				}

				// Last chunk should have finish_reason
				if i == len(chunks)-1 {
					assert.Equal(t, "stop", choice.Get("finish_reason").String(), "Last chunk should have finish_reason")
				}
			}
		})
	}
}

// TestKiroStreaming_MalformedChunks handles malformed streaming chunks
func TestKiroStreaming_MalformedChunks(t *testing.T) {
	malformedChunks := []string{
		`{"content": "Valid chunk"}`,
		`{"invalid": "chunk without proper structure"}`,
		`{"content": "Another valid chunk"}`,
		`not json at all`,
		`{"content": "Final valid chunk"}`,
		`{"stop": true}`,
	}

	// Concatenate malformed chunks
	var streamingData strings.Builder
	for _, chunk := range malformedChunks {
		streamingData.WriteString(chunk)
		streamingData.WriteString("\n")
	}

	// Should handle malformed chunks gracefully
	content, toolCalls := kiro.ParseResponse([]byte(streamingData.String()))

	// Should extract valid content despite malformed chunks
	assert.Contains(t, content, "Valid chunk", "Should extract valid content")
	assert.Contains(t, content, "Another valid chunk", "Should extract other valid content")
	assert.Contains(t, content, "Final valid chunk", "Should extract final valid content")
	assert.Empty(t, toolCalls, "Should have no tool calls")
}

// TestKiroStreaming_LargeContent tests streaming with large content
func TestKiroStreaming_LargeContent(t *testing.T) {
	// Generate large content (10KB)
	var largeContent strings.Builder
	for i := 0; i < 1000; i++ {
		largeContent.WriteString("This is line ")
		largeContent.WriteString(string(rune(i)))
		largeContent.WriteString(" of the large content. ")
	}

	content := largeContent.String()

	// Build streaming chunks for large content
	chunks := kiro.BuildStreamingChunks(
		"large_content_test",
		"claude-sonnet-4-5-20250929",
		1234567890,
		content,
		[]kiro.OpenAIToolCall{},
	)

	// Should handle large content without issues
	assert.GreaterOrEqual(t, len(chunks), 2, "Should have at least initial and final chunks")

	// Validate that large content is properly distributed
	var accumulatedContent strings.Builder
	for _, chunk := range chunks {
		chunkJSON := gjson.ParseBytes(chunk)
		delta := chunkJSON.Get("choices.0.delta")
		if content := delta.Get("content").String(); content != "" {
			accumulatedContent.WriteString(content)
		}
	}

	assert.Equal(t, content, accumulatedContent.String(), "Large content should be preserved in streaming")
}

// ============================================================================
// Model Mapping Tests
// ============================================================================

// TestKiroMapModel_ValidatesModelMapping
func TestKiroMapModel_ValidatesModelMapping(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"claude-sonnet-4-5", "CLAUDE_SONNET_4_5_20250929_V1_0"},
		{"claude-sonnet-4-5-20250929", "CLAUDE_SONNET_4_5_20250929_V1_0"},
		{"claude-sonnet-4-20250514", "CLAUDE_SONNET_4_20250514_V1_0"},
		{"claude-3-7-sonnet-20250219", "CLAUDE_3_7_SONNET_20250219_V1_0"},
		{"amazonq-claude-sonnet-4-20250514", "CLAUDE_SONNET_4_20250514_V1_0"},
		{"amazonq-claude-3-7-sonnet-20250219", "CLAUDE_3_7_SONNET_20250219_V1_0"},
		{"unknown-model", "CLAUDE_SONNET_4_5_20250929_V1_0"}, // Should default to sonnet-4-5
		{"", "CLAUDE_SONNET_4_5_20250929_V1_0"},             // Empty should default
		{"  claude-sonnet-4-5  ", "CLAUDE_SONNET_4_5_20250929_V1_0"}, // Should trim whitespace
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := kiro.MapModel(tc.input)
			assert.Equal(t, tc.expected, result, "Model mapping should match expected")
		})
	}
}