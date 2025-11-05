package kiro_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseResponseFromJSON(t *testing.T) {
	body := []byte(`{
        "conversationState": {
            "currentMessage": {
                "assistantResponseMessage": {
                    "content": "Hello!"
                }
            }
        }
    }`)
	text, calls := kiro.ParseResponse(body)
	if text != "Hello!" {
		t.Fatalf("unexpected text: %q", text)
	}
	if len(calls) != 0 {
		t.Fatalf("expected no tool calls, got %d", len(calls))
	}
}

func TestParseResponseFromEventStream(t *testing.T) {
	stream := strings.Join([]string{
		`data: {"content":"Line 1"}`,
		`data: {"content":"Line 2"}`,
		`data: {"name":"lookup","toolUseId":"call-1","input":{"foo":"bar"}}`,
		`data: {"name":"lookup","toolUseId":"call-1","input":{"baz":1},"stop":true}`,
	}, "\n")

	text, calls := kiro.ParseResponse([]byte(stream))
	if !strings.Contains(text, "Line 1") || !strings.Contains(text, "Line 2") {
		t.Fatalf("unexpected aggregated text: %q", text)
	}
	if len(calls) != 1 {
		t.Fatalf("expected a single tool call, got %d", len(calls))
	}
	if calls[0].Name != "lookup" {
		t.Fatalf("unexpected tool call name: %s", calls[0].Name)
	}
	if !strings.Contains(calls[0].Arguments, "foo") || !strings.Contains(calls[0].Arguments, "baz") {
		t.Fatalf("tool call arguments missing merged content: %s", calls[0].Arguments)
	}
}

func TestParseResponseFromEventStreamWithControlDelimiters(t *testing.T) {
	raw := strings.Join([]string{
		`:message-typeevent{"content":"I don"}`,
		"\v:message-typeevent{\"content\":\"'t have access\"}",
		"\v:message-typeevent{\"content\":\" to data.\"}",
		"\v:metering-event{\"unit\":\"credit\",\"usage\":0.01}",
	}, "")

	text, calls := kiro.ParseResponse([]byte(raw))
	require.Equal(t, "I don't have access to data.", text)
	require.Empty(t, calls)
}

func TestParseResponseFromAnthropicStyleStream(t *testing.T) {
	stream := strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"content":[{"type":"text","text":""}]}}`,
		"",
		"event: content_block_start",
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		"",
		"event: content_block_delta",
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"I don"}}`,
		"",
		"event: content_block_delta",
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"'t have access"}}`,
		"",
		"event: content_block_delta",
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" to data."}}`,
		"",
		"event: content_block_stop",
		`data: {"type":"content_block_stop","index":0}`,
		"",
		"event: message_stop",
		`data: {"type":"message_stop"}`,
	}, "\n")

	text, calls := kiro.ParseResponse([]byte(stream))
	require.Equal(t, "I don't have access to data.", text)
	require.Empty(t, calls)
}

func TestParseResponseFromAnthropicToolStream(t *testing.T) {
	stream := strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"content":[{"type":"text","text":""}]}}`,
		"",
		"event: content_block_start",
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":"Let me check that."}}`,
		"",
		"event: content_block_start",
		`data: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_1","name":"get_weather","input":{}}}`,
		"",
		"event: content_block_delta",
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"I'm fetching the forecast."}}`,
		"",
		"event: content_block_delta",
		`data: {"type":"content_block_delta","index":1,"delta":{"type":"tool_use_delta","partial_json":"{\"location\": \"Tokyo\""}}`,
		"",
		"event: content_block_delta",
		`data: {"type":"content_block_delta","index":1,"delta":{"type":"tool_use_delta","partial_json":", \"unit\": \"°C\"}"}}`,
		"",
		"event: content_block_stop",
		`data: {"type":"content_block_stop","index":1}`,
		"",
		"event: message_stop",
		`data: {"type":"message_stop"}`,
	}, "\n")

	text, calls := kiro.ParseResponse([]byte(stream))
	require.Contains(t, text, "Let me check that.")
	require.Contains(t, text, "I'm fetching the forecast.")
	require.Len(t, calls, 1)
	require.Equal(t, "toolu_1", calls[0].ID)
	require.Equal(t, "get_weather", calls[0].Name)
	require.JSONEq(t, `{"location":"Tokyo","unit":"°C"}`, calls[0].Arguments)
}

func TestParseResponseFromAnthropicJSONMessage(t *testing.T) {
	body := []byte(`{
        "id": "msg_test",
        "type": "message",
        "role": "assistant",
        "content": [
            {"type": "text", "text": "I don"},
            {"type": "text", "text": "'t have access to data."}
        ],
        "stop_reason": "end_turn"
    }`)

	text, calls := kiro.ParseResponse(body)
	require.Equal(t, "I don't have access to data.", text)
	require.Empty(t, calls)
}

func TestParseResponseFromAnthropicJSONWithToolUse(t *testing.T) {
	body := []byte(`{
        "id": "msg_tool",
        "type": "message",
        "role": "assistant",
        "content": [
            {"type": "text", "text": "Sure, calling the weather tool."},
            {"type": "tool_use", "id": "toolu_2", "name": "get_weather", "input": {"city": "Tokyo", "unit": "°C"}}
        ],
        "stop_reason": "tool_use"
    }`)

	text, calls := kiro.ParseResponse(body)
	require.Contains(t, text, "Sure, calling the weather tool.")
	require.Len(t, calls, 1)
	require.Equal(t, "toolu_2", calls[0].ID)
	require.Equal(t, "get_weather", calls[0].Name)
	require.JSONEq(t, `{"city":"Tokyo","unit":"°C"}`, calls[0].Arguments)
}

func TestBuildOpenAIChatCompletionPayload(t *testing.T) {
	payload, err := kiro.BuildOpenAIChatCompletionPayload("claude-sonnet-4-5", "hi", []kiro.OpenAIToolCall{
		{ID: "call-1", Name: "lookup", Arguments: `{"foo":"bar"}`},
	}, 10, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(payload, &out); err != nil {
		t.Fatalf("payload not JSON: %v", err)
	}
	usage := out["usage"].(map[string]any)
	if usage["total_tokens"].(float64) != 30 {
		t.Fatalf("usage total mismatch: %+v", usage)
	}
	choices := out["choices"].([]any)
	message := choices[0].(map[string]any)["message"].(map[string]any)
	toolCalls := message["tool_calls"].([]any)
	if len(toolCalls) != 1 {
		t.Fatalf("expected tool call in response")
	}
}

func TestBuildStreamingChunks(t *testing.T) {
	chunks := kiro.BuildStreamingChunks("chatcmpl_test", "claude", time.Now().Unix(), "hello", []kiro.OpenAIToolCall{
		{ID: "call-1", Name: "lookup", Arguments: `{"foo":1}`},
	})
	if len(chunks) < 3 {
		t.Fatalf("expected multiple streaming chunks, got %d", len(chunks))
	}
}

// FAILING TESTS FOR kiro.BuildAnthropicMessagePayload FUNCTION
// These tests will fail because the kiro.BuildAnthropicMessagePayload function does not exist yet

func TestBuildAnthropicMessagePayload_BasicTextResponse(t *testing.T) {
	// Test basic text response format conversion
	model := "claude-sonnet-4-5"
	content := "Hello, this is a basic text response"
	toolCalls := []kiro.OpenAIToolCall{}
	promptTokens := int64(25)
	completionTokens := int64(15)

	payload, err := kiro.BuildAnthropicMessagePayload(model, content, toolCalls, promptTokens, completionTokens)

	// This should fail because kiro.BuildAnthropicMessagePayload doesn't exist
	require.NoError(t, err, "kiro.BuildAnthropicMessagePayload should not return an error")
	require.NotNil(t, payload, "Payload should not be nil")

	var result map[string]interface{}
	err = json.Unmarshal(payload, &result)
	require.NoError(t, err, "Payload should be valid JSON")

	// Verify Anthropic Messages API format
	assert.Equal(t, "message", result["type"], "Response type should be 'message'")
	assert.Equal(t, model, result["model"], "Model should match")

	// Check content structure
	contentBlock, ok := result["content"].([]interface{})
	require.True(t, ok, "Content should be an array")
	require.Len(t, contentBlock, 1, "Should have one content block")

	firstBlock := contentBlock[0].(map[string]interface{})
	assert.Equal(t, "text", firstBlock["type"], "Content block type should be 'text'")
	assert.Equal(t, content, firstBlock["text"], "Text content should match")

	// Check usage
	usage, ok := result["usage"].(map[string]interface{})
	require.True(t, ok, "Usage should be present")
	assert.Equal(t, float64(promptTokens), usage["input_tokens"], "Input tokens should match")
	assert.Equal(t, float64(completionTokens), usage["output_tokens"], "Output tokens should match")

	// Check stop reason
	assert.Equal(t, "end_turn", result["stop_reason"], "Stop reason should be 'end_turn'")
	assert.Nil(t, result["stop_sequence"], "Stop sequence should be null per Anthropic spec")
}

func TestBuildAnthropicMessagePayload_ToolUseResponse(t *testing.T) {
	// Test tool use response format conversion
	model := "claude-sonnet-4-5"
	content := "I'll help you get the weather"
	toolCalls := []kiro.OpenAIToolCall{
		{
			ID:        "call_1",
			Name:      "get_weather",
			Arguments: `{"location": "Seattle"}`,
		},
	}
	promptTokens := int64(50)
	completionTokens := int64(30)

	payload, err := kiro.BuildAnthropicMessagePayload(model, content, toolCalls, promptTokens, completionTokens)

	// This should fail because kiro.BuildAnthropicMessagePayload doesn't exist
	require.NoError(t, err, "kiro.BuildAnthropicMessagePayload should not return an error")
	require.NotNil(t, payload, "Payload should not be nil")

	var result map[string]interface{}
	err = json.Unmarshal(payload, &result)
	require.NoError(t, err, "Payload should be valid JSON")

	// Verify Anthropic Messages API format
	assert.Equal(t, "message", result["type"], "Response type should be 'message'")
	assert.Equal(t, model, result["model"], "Model should match")

	// Check content structure - should have text + tool_use blocks
	contentBlock, ok := result["content"].([]interface{})
	require.True(t, ok, "Content should be an array")
	require.Len(t, contentBlock, 2, "Should have two content blocks (text + tool_use)")

	// First block should be text
	textBlock := contentBlock[0].(map[string]interface{})
	assert.Equal(t, "text", textBlock["type"], "First block type should be 'text'")
	assert.Equal(t, content, textBlock["text"], "Text content should match")

	// Second block should be tool_use
	toolBlock := contentBlock[1].(map[string]interface{})
	assert.Equal(t, "tool_use", toolBlock["type"], "Second block type should be 'tool_use'")
	assert.Equal(t, "get_weather", toolBlock["name"], "Tool name should match")
	assert.Equal(t, "call_1", toolBlock["id"], "Tool ID should match")

	// Check tool input
	input, ok := toolBlock["input"].(map[string]interface{})
	require.True(t, ok, "Tool input should be an object")
	assert.Equal(t, "Seattle", input["location"], "Location should match")

	// Check usage
	usage, ok := result["usage"].(map[string]interface{})
	require.True(t, ok, "Usage should be present")
	assert.Equal(t, float64(promptTokens), usage["input_tokens"], "Input tokens should match")
	assert.Equal(t, float64(completionTokens), usage["output_tokens"], "Output tokens should match")

	// Check stop reason
	assert.Equal(t, "tool_use", result["stop_reason"], "Stop reason should be 'tool_use'")
}

func TestBuildAnthropicMessagePayload_MultipleToolUseResponses(t *testing.T) {
	// Test multiple tool use response format conversion
	model := "claude-sonnet-4-5"
	content := "I'll help you get the weather and search for information"
	toolCalls := []kiro.OpenAIToolCall{
		{
			ID:        "call_1",
			Name:      "get_weather",
			Arguments: `{"location": "Seattle"}`,
		},
		{
			ID:        "call_2",
			Name:      "search_web",
			Arguments: `{"query": "latest AI news"}`,
		},
	}
	promptTokens := int64(80)
	completionTokens := int64(45)

	payload, err := kiro.BuildAnthropicMessagePayload(model, content, toolCalls, promptTokens, completionTokens)

	// This should fail because kiro.BuildAnthropicMessagePayload doesn't exist
	require.NoError(t, err, "kiro.BuildAnthropicMessagePayload should not return an error")
	require.NotNil(t, payload, "Payload should not be nil")

	var result map[string]interface{}
	err = json.Unmarshal(payload, &result)
	require.NoError(t, err, "Payload should be valid JSON")

	// Verify Anthropic Messages API format
	assert.Equal(t, "message", result["type"], "Response type should be 'message'")
	assert.Equal(t, model, result["model"], "Model should match")

	// Check content structure - should have text + 2 tool_use blocks
	contentBlock, ok := result["content"].([]interface{})
	require.True(t, ok, "Content should be an array")
	require.Len(t, contentBlock, 3, "Should have three content blocks (text + 2 tool_use)")

	// First block should be text
	textBlock := contentBlock[0].(map[string]interface{})
	assert.Equal(t, "text", textBlock["type"], "First block type should be 'text'")
	assert.Equal(t, content, textBlock["text"], "Text content should match")

	// Second block should be first tool_use
	toolBlock1 := contentBlock[1].(map[string]interface{})
	assert.Equal(t, "tool_use", toolBlock1["type"], "Second block type should be 'tool_use'")
	assert.Equal(t, "get_weather", toolBlock1["name"], "Tool name should match")
	assert.Equal(t, "call_1", toolBlock1["id"], "Tool ID should match")

	// Third block should be second tool_use
	toolBlock2 := contentBlock[2].(map[string]interface{})
	assert.Equal(t, "tool_use", toolBlock2["type"], "Third block type should be 'tool_use'")
	assert.Equal(t, "search_web", toolBlock2["name"], "Tool name should match")
	assert.Equal(t, "call_2", toolBlock2["id"], "Tool ID should match")

	// Check stop reason
	assert.Equal(t, "tool_use", result["stop_reason"], "Stop reason should be 'tool_use'")
}

func TestBuildAnthropicMessagePayload_MixedContentResponses(t *testing.T) {
	// Test mixed content responses with text and tools
	model := "claude-sonnet-4-5"
	content := "I'll help you with that task"
	toolCalls := []kiro.OpenAIToolCall{
		{
			ID:        "call_1",
			Name:      "calculate",
			Arguments: `{"expression": "2 + 2"}`,
		},
	}
	promptTokens := int64(30)
	completionTokens := int64(20)

	payload, err := kiro.BuildAnthropicMessagePayload(model, content, toolCalls, promptTokens, completionTokens)

	// This should fail because kiro.BuildAnthropicMessagePayload doesn't exist
	require.NoError(t, err, "kiro.BuildAnthropicMessagePayload should not return an error")
	require.NotNil(t, payload, "Payload should not be nil")

	var result map[string]interface{}
	err = json.Unmarshal(payload, &result)
	require.NoError(t, err, "Payload should be valid JSON")

	// Verify mixed content structure
	contentBlock, ok := result["content"].([]interface{})
	require.True(t, ok, "Content should be an array")
	require.Len(t, contentBlock, 2, "Should have two content blocks")

	// Verify both text and tool_use blocks are present
	blockTypes := make([]string, 2)
	for i, block := range contentBlock {
		blockMap := block.(map[string]interface{})
		blockTypes[i] = blockMap["type"].(string)
	}

	assert.Contains(t, blockTypes, "text", "Should contain a text block")
	assert.Contains(t, blockTypes, "tool_use", "Should contain a tool_use block")

	// Verify content integrity
	textBlock := contentBlock[0].(map[string]interface{})
	if textBlock["type"] == "text" {
		assert.Equal(t, content, textBlock["text"], "Text content should match")
	}
}

func TestBuildAnthropicMessagePayload_StopReasonMapping(t *testing.T) {
	// Test stop reason mapping from OpenAI to Anthropic format
	testCases := []struct {
		name               string
		content            string
		toolCalls          []kiro.OpenAIToolCall
		expectedStopReason string
	}{
		{
			name:               "end_turn",
			content:            "Hello world",
			toolCalls:          []kiro.OpenAIToolCall{},
			expectedStopReason: "end_turn",
		},
		{
			name:    "tool_use",
			content: "I'll use a tool",
			toolCalls: []kiro.OpenAIToolCall{
				{ID: "call_1", Name: "test", Arguments: "{}"},
			},
			expectedStopReason: "tool_use",
		},
		{
			name:               "max_tokens",
			content:            "This is a long response that should be cut off due to max tokens",
			toolCalls:          []kiro.OpenAIToolCall{},
			expectedStopReason: "max_tokens",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := kiro.BuildAnthropicMessagePayload("claude-sonnet-4-5", tc.content, tc.toolCalls, 10, 20)

			// This should fail because kiro.BuildAnthropicMessagePayload doesn't exist
			require.NoError(t, err, "kiro.BuildAnthropicMessagePayload should not return an error")
			require.NotNil(t, payload, "Payload should not be nil")

			var result map[string]interface{}
			err = json.Unmarshal(payload, &result)
			require.NoError(t, err, "Payload should be valid JSON")

			assert.Equal(t, tc.expectedStopReason, result["stop_reason"], "Stop reason should match expected value")
		})
	}
}

func TestBuildAnthropicMessagePayload_UsageTokenMapping(t *testing.T) {
	// Test usage token mapping
	model := "claude-sonnet-4-5"
	content := "Test response"
	toolCalls := []kiro.OpenAIToolCall{}
	promptTokens := int64(100)
	completionTokens := int64(50)

	payload, err := kiro.BuildAnthropicMessagePayload(model, content, toolCalls, promptTokens, completionTokens)

	// This should fail because kiro.BuildAnthropicMessagePayload doesn't exist
	require.NoError(t, err, "kiro.BuildAnthropicMessagePayload should not return an error")
	require.NotNil(t, payload, "Payload should not be nil")

	var result map[string]interface{}
	err = json.Unmarshal(payload, &result)
	require.NoError(t, err, "Payload should be valid JSON")

	// Check usage structure
	usage, ok := result["usage"].(map[string]interface{})
	require.True(t, ok, "Usage should be present")

	assert.Equal(t, float64(promptTokens), usage["input_tokens"], "Input tokens should match")
	assert.Equal(t, float64(completionTokens), usage["output_tokens"], "Output tokens should match")

	// Check total tokens calculation
	expectedTotal := promptTokens + completionTokens
	assert.Equal(t, float64(expectedTotal), usage["total_tokens"], "Total tokens should be sum of input and output")
}

func TestBuildAnthropicMessagePayload_ErrorHandlingMalformedResponses(t *testing.T) {
	// Test error handling for malformed responses
	testCases := []struct {
		name             string
		model            string
		content          string
		toolCalls        []kiro.OpenAIToolCall
		promptTokens     int64
		completionTokens int64
		expectedError    string
	}{
		{
			name:             "empty_model",
			model:            "",
			content:          "test",
			toolCalls:        []kiro.OpenAIToolCall{},
			promptTokens:     10,
			completionTokens: 5,
			expectedError:    "model cannot be empty",
		},
		{
			name:             "negative_tokens",
			model:            "claude-sonnet-4-5",
			content:          "test",
			toolCalls:        []kiro.OpenAIToolCall{},
			promptTokens:     -1,
			completionTokens: 5,
			expectedError:    "token count cannot be negative",
		},
		{
			name:             "valid_tool_call_with_empty_id",
			model:            "claude-sonnet-4-5",
			content:          "test",
			toolCalls:        []kiro.OpenAIToolCall{{ID: "", Name: "test", Arguments: "{}"}},
			promptTokens:     10,
			completionTokens: 5,
			expectedError:    "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := kiro.BuildAnthropicMessagePayload(tc.model, tc.content, tc.toolCalls, tc.promptTokens, tc.completionTokens)

			// This should fail because kiro.BuildAnthropicMessagePayload doesn't exist
			// When implemented, it should return an error for malformed inputs
			if tc.expectedError != "" {
				require.Error(t, err, "Expected error for malformed input")
				assert.Contains(t, err.Error(), tc.expectedError, "Error message should contain expected text")
				assert.Nil(t, payload, "Payload should be nil for error case")
			} else {
				require.NoError(t, err, "Should not return error for valid input")
				assert.NotNil(t, payload, "Payload should not be nil for valid input")
			}
		})
	}
}

func TestBuildAnthropicMessagePayload_StreamingResponseFormat(t *testing.T) {
	// Test streaming response format
	model := "claude-sonnet-4-5"
	content := "This is a streaming response"
	toolCalls := []kiro.OpenAIToolCall{
		{
			ID:        "call_1",
			Name:      "get_weather",
			Arguments: `{"location": "Seattle"}`,
		},
	}
	promptTokens := int64(40)
	completionTokens := int64(25)

	payload, err := kiro.BuildAnthropicMessagePayload(model, content, toolCalls, promptTokens, completionTokens)

	// This should fail because kiro.BuildAnthropicMessagePayload doesn't exist
	require.NoError(t, err, "kiro.BuildAnthropicMessagePayload should not return an error")
	require.NotNil(t, payload, "Payload should not be nil")

	var result map[string]interface{}
	err = json.Unmarshal(payload, &result)
	require.NoError(t, err, "Payload should be valid JSON")

	// Verify streaming-specific format requirements
	assert.Equal(t, "message", result["type"], "Response type should be 'message'")
	assert.Contains(t, result, "id", "Should contain message ID")
	assert.Contains(t, result, "usage", "Should contain usage information")

	// Check that content is properly structured for streaming
	contentBlock, ok := result["content"].([]interface{})
	require.True(t, ok, "Content should be an array")
	assert.Greater(t, len(contentBlock), 0, "Should have at least one content block")

	// Verify streaming response has proper structure for incremental updates
	for _, block := range contentBlock {
		blockMap := block.(map[string]interface{})
		blockType := blockMap["type"].(string)
		assert.Contains(t, []string{"text", "tool_use"}, blockType, "Block type should be valid")

		if blockType == "text" {
			assert.Contains(t, blockMap, "text", "Text block should have text field")
		} else if blockType == "tool_use" {
			assert.Contains(t, blockMap, "id", "Tool use block should have id")
			assert.Contains(t, blockMap, "name", "Tool use block should have name")
			assert.Contains(t, blockMap, "input", "Tool use block should have input")
		}
	}
}

// FAILING TESTS FOR EDGE CASES AND ERROR SCENARIOS

func TestBuildAnthropicMessagePayload_EdgeCases(t *testing.T) {
	// Test edge cases for kiro.BuildAnthropicMessagePayload
	testCases := []struct {
		name             string
		model            string
		content          string
		toolCalls        []kiro.OpenAIToolCall
		promptTokens     int64
		completionTokens int64
	}{
		{
			name:             "empty_content",
			model:            "claude-sonnet-4-5",
			content:          "",
			toolCalls:        []kiro.OpenAIToolCall{},
			promptTokens:     10,
			completionTokens: 0,
		},
		{
			name:             "whitespace_content",
			model:            "claude-sonnet-4-5",
			content:          "   ",
			toolCalls:        []kiro.OpenAIToolCall{},
			promptTokens:     10,
			completionTokens: 0,
		},
		{
			name:             "zero_tokens",
			model:            "claude-sonnet-4-5",
			content:          "test",
			toolCalls:        []kiro.OpenAIToolCall{},
			promptTokens:     0,
			completionTokens: 0,
		},
		{
			name:             "large_token_count",
			model:            "claude-sonnet-4-5",
			content:          "test",
			toolCalls:        []kiro.OpenAIToolCall{},
			promptTokens:     1000000,
			completionTokens: 500000,
		},
		{
			name:             "special_characters_content",
			model:            "claude-sonnet-4-5",
			content:          "Hello 🌍 World! \n\t Test",
			toolCalls:        []kiro.OpenAIToolCall{},
			promptTokens:     20,
			completionTokens: 10,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := kiro.BuildAnthropicMessagePayload(tc.model, tc.content, tc.toolCalls, tc.promptTokens, tc.completionTokens)

			// This should fail because kiro.BuildAnthropicMessagePayload doesn't exist
			require.NoError(t, err, "kiro.BuildAnthropicMessagePayload should not return an error")
			require.NotNil(t, payload, "Payload should not be nil")

			var result map[string]interface{}
			err = json.Unmarshal(payload, &result)
			require.NoError(t, err, "Payload should be valid JSON")

			// Basic validation that should pass for all edge cases
			assert.Equal(t, "message", result["type"], "Response type should be 'message'")
			assert.Equal(t, tc.model, result["model"], "Model should match")
			assert.Contains(t, result, "content", "Should contain content")
			assert.Contains(t, result, "usage", "Should contain usage")
		})
	}
}

func TestBuildAnthropicMessagePayload_ToolCallEdgeCases(t *testing.T) {
	// Test edge cases for tool calls in kiro.BuildAnthropicMessagePayload
	testCases := []struct {
		name      string
		toolCalls []kiro.OpenAIToolCall
	}{
		{
			name: "empty_tool_call_id",
			toolCalls: []kiro.OpenAIToolCall{
				{ID: "", Name: "test", Arguments: "{}"},
			},
		},
		{
			name: "empty_tool_call_name",
			toolCalls: []kiro.OpenAIToolCall{
				{ID: "call_1", Name: "", Arguments: "{}"},
			},
		},
		{
			name: "empty_tool_call_arguments",
			toolCalls: []kiro.OpenAIToolCall{
				{ID: "call_1", Name: "test", Arguments: ""},
			},
		},
		{
			name: "malformed_tool_call_arguments",
			toolCalls: []kiro.OpenAIToolCall{
				{ID: "call_1", Name: "test", Arguments: "{ invalid json }"},
			},
		},
		{
			name: "null_tool_call_arguments",
			toolCalls: []kiro.OpenAIToolCall{
				{ID: "call_1", Name: "test", Arguments: "null"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := kiro.BuildAnthropicMessagePayload("claude-sonnet-4-5", "test", tc.toolCalls, 10, 5)

			// This should fail because kiro.BuildAnthropicMessagePayload doesn't exist
			// When implemented, it should handle edge cases gracefully
			require.NoError(t, err, "kiro.BuildAnthropicMessagePayload should handle edge cases gracefully")
			require.NotNil(t, payload, "Payload should not be nil")

			var result map[string]interface{}
			err = json.Unmarshal(payload, &result)
			require.NoError(t, err, "Payload should be valid JSON")

			// Basic validation
			assert.Equal(t, "message", result["type"], "Response type should be 'message'")
			assert.Contains(t, result, "content", "Should contain content")
		})
	}
}
