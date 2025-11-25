package kiro_test

import (
	"context"
	"encoding/json"
	"testing"

	authkiro "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro/claude"
	chat_completions "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro/openai/chat-completions"
	kiroresponses "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro/openai/responses"
	_ "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/openai/openai/responses"
	"github.com/router-for-me/CLIProxyAPI/v6/tests/shared"
	"github.com/tidwall/gjson"
)

// TestRequestTranslation tests ConvertOpenAIRequestToKiro with real payload files
func TestRequestTranslation(t *testing.T) {
	// Create a mock token for testing
	mockToken := &authkiro.KiroTokenStorage{
		AccessToken: "test-token",
	}

	tests := []struct {
		name           string
		filename       string
		expectedModel  string
		expectHistory  bool
		expectMessages bool
		expectTools    bool
		notes          string
		format         string // "openai" or "anthropic"
	}{
		// OpenAI format tests
		{
			name:           "openai_format_simple - basic OpenAI request",
			filename:       "openai_format_simple",
			expectedModel:  "gpt-4",
			expectHistory:  false,
			expectMessages: true,
			expectTools:    false,
			notes:          "Simple OpenAI format without tools",
			format:         "openai",
		},
		{
			name:           "openai_format - OpenAI with function tools",
			filename:       "openai_format",
			expectedModel:  "gpt-4",
			expectHistory:  false,
			expectMessages: true,
			expectTools:    true,
			notes:          "OpenAI format with type=function wrapped tools",
			format:         "openai",
		},
		{
			name:           "openai_format_with_tools - OpenAI with tool calls and results",
			filename:       "openai_format_with_tools",
			expectedModel:  "gpt-4",
			expectHistory:  true,
			expectMessages: true,
			expectTools:    true,
			notes:          "OpenAI format with tool_calls in assistant message and tool results",
			format:         "openai",
		},
		// Anthropic/Kiro format tests
		{
			name:           "orignal - Anthropic format with tools and history",
			filename:       "orignal",
			expectedModel:  "claude-sonnet-4-5",
			expectHistory:  true,
			expectMessages: true,
			expectTools:    false,
			notes:          "Uses Anthropic/Kiro format directly, tools already in native format",
			format:         "anthropic",
		},
		{
			name:           "orignal_tool_call - Anthropic request with tool calls",
			filename:       "orignal_tool_call",
			expectedModel:  "claude-sonnet-4-5",
			expectHistory:  true,
			expectMessages: true,
			expectTools:    false,
			notes:          "Contains tool_use in messages",
			format:         "anthropic",
		},
		{
			name:           "orignal_tool_call_no_result - Anthropic tool call without result",
			filename:       "orignal_tool_call_no_result",
			expectedModel:  "claude-sonnet-4-5",
			expectHistory:  true,
			expectMessages: true,
			expectTools:    false,
			notes:          "Has tool calls but no tool results",
			format:         "anthropic",
		},
		{
			name:           "orignal_tool_call_no_tools - Anthropic no tools defined",
			filename:       "orignal_tool_call_no_tools",
			expectedModel:  "claude-sonnet-4-5",
			expectHistory:  true,
			expectMessages: true,
			expectTools:    false,
			notes:          "No tools array in request",
			format:         "anthropic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Load the OpenAI request payload
			payload := shared.LoadNonStreamRequest(t, tt.filename)

			// Convert to Kiro format
			kiroRequest, err := chat_completions.ConvertOpenAIRequestToKiro(
				tt.expectedModel,
				payload,
				mockToken,
				nil,
			)
			if err != nil {
				t.Fatalf("ConvertOpenAIRequestToKiro failed: %v", err)
			}

			// Parse the converted request
			kiroJSON := gjson.ParseBytes(kiroRequest)

			// Verify conversationState exists
			convState := kiroJSON.Get("conversationState")
			if !convState.Exists() {
				t.Fatal("conversationState not found in converted request")
			}

			// Verify tools conversion for OpenAI format
			if tt.format == "openai" && tt.expectTools {
				tools := convState.Get("tools")
				if !tools.Exists() {
					t.Error("expected tools in conversationState after conversion, but not found")
				} else if !tools.IsArray() {
					t.Error("tools should be an array")
				} else if len(tools.Array()) == 0 {
					t.Error("tools array is empty after conversion")
				} else {
					// Verify tools were converted from OpenAI format to Kiro format
					// OpenAI: {type: "function", function: {name, description, parameters}}
					// Kiro: {name, description, inputSchema}
					firstTool := tools.Array()[0]
					if !firstTool.Get("name").Exists() {
						t.Error("converted tool missing 'name' field")
					}
					if !firstTool.Get("description").Exists() {
						t.Error("converted tool missing 'description' field")
					}
					if !firstTool.Get("inputSchema").Exists() {
						t.Error("converted tool missing 'inputSchema' field (should be converted from 'parameters')")
					}
				}
			}

			// Note for Anthropic format: already in native Kiro format
			if tt.format == "anthropic" {
				// These test files use Kiro/Anthropic native format, not OpenAI format.
				// The tools in the files have {name, description, input_schema} directly,
				// while OpenAI format wraps them in {type: "function", function: {...}}.
				// So we don't expect tools to be converted - they're already in the right format.
			}

			// Verify history or currentMessage exists
			if tt.expectMessages {
				hasHistory := convState.Get("history").Exists()
				hasCurrentMsg := convState.Get("currentMessage").Exists()

				if !hasHistory && !hasCurrentMsg {
					t.Error("expected either history or currentMessage, found neither")
				}
			}

			// Verify the request is valid JSON
			var validated map[string]interface{}
			if err := json.Unmarshal(kiroRequest, &validated); err != nil {
				t.Errorf("converted request is not valid JSON: %v", err)
			}
		})
	}
}

// TestResponseConversion tests ConvertKiroResponseToOpenAI
func TestResponseConversion(t *testing.T) {
	testCases := []struct {
		name            string
		kiroResponse    string
		model           string
		expectContent   bool
		expectToolCalls bool
	}{
		{
			name: "simple_text_response",
			kiroResponse: `{
				"conversationState": {
					"currentMessage": {
						"content": "Hello, how can I help you?"
					}
				}
			}`,
			model:         "kiro-sonnet",
			expectContent: true,
		},
		{
			name: "response_with_thinking",
			kiroResponse: `{
				"conversationState": {
					"currentMessage": {
						"content": "<thinking>Internal thought</thinking>Actual response"
					}
				}
			}`,
			model:         "kiro-haiku",
			expectContent: true,
		},
		{
			name: "empty_response",
			kiroResponse: `{
				"conversationState": {
					"currentMessage": {
						"content": ""
					}
				}
			}`,
			model:         "kiro-sonnet",
			expectContent: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Convert Kiro response to OpenAI format
			openAIResp := chat_completions.ConvertKiroResponseToOpenAI(
				[]byte(tc.kiroResponse),
				tc.model,
				false,
			)

			// Parse the response
			respJSON := gjson.ParseBytes(openAIResp)

			// Verify OpenAI response structure
			if !respJSON.Get("id").Exists() {
				t.Error("response missing 'id' field")
			}

			if respJSON.Get("object").String() != "chat.completion" {
				t.Errorf("expected object=chat.completion, got %q", respJSON.Get("object").String())
			}

			if respJSON.Get("model").String() != tc.model {
				t.Errorf("expected model=%q, got %q", tc.model, respJSON.Get("model").String())
			}

			// Verify choices array
			choices := respJSON.Get("choices")
			if !choices.Exists() || !choices.IsArray() {
				t.Fatal("choices array not found or invalid")
			}

			if len(choices.Array()) != 1 {
				t.Errorf("expected 1 choice, got %d", len(choices.Array()))
			}

			// Verify message content
			if tc.expectContent {
				content := respJSON.Get("choices.0.message.content")
				if !content.Exists() {
					t.Error("expected content in message, but not found")
				}

				// Verify thinking content is filtered
				if tc.name == "response_with_thinking" {
					if content.String() != "Actual response" {
						t.Errorf("thinking content not filtered: %q", content.String())
					}
				}
			}
		})
	}
}

// TestFullRoundTrip tests conversion from OpenAI -> Kiro -> OpenAI
func TestFullRoundTrip(t *testing.T) {
	mockToken := &authkiro.KiroTokenStorage{
		AccessToken: "test-token",
	}

	// Load a real payload
	payload := shared.LoadNonStreamRequest(t, "orignal")

	// Convert to Kiro format
	kiroRequest, err := chat_completions.ConvertOpenAIRequestToKiro(
		"claude-sonnet-4-5",
		payload,
		mockToken,
		nil,
	)
	if err != nil {
		t.Fatalf("request conversion failed: %v", err)
	}

	// Verify Kiro request structure
	kiroJSON := gjson.ParseBytes(kiroRequest)
	if !kiroJSON.Get("conversationState").Exists() {
		t.Fatal("Kiro request missing conversationState")
	}

	// Simulate a Kiro response
	mockKiroResponse := `{
		"conversationState": {
			"currentMessage": {
				"content": "I can help with that task."
			}
		}
	}`

	// Convert response back to OpenAI format
	openAIResp := chat_completions.ConvertKiroResponseToOpenAI(
		[]byte(mockKiroResponse),
		"claude-sonnet-4-5",
		false,
	)

	// Verify OpenAI response
	respJSON := gjson.ParseBytes(openAIResp)
	if respJSON.Get("choices.0.message.content").String() != "I can help with that task." {
		t.Errorf("unexpected response content: %q", respJSON.Get("choices.0.message.content").String())
	}

	if respJSON.Get("choices.0.finish_reason").String() != "stop" {
		t.Errorf("unexpected finish_reason: %q", respJSON.Get("choices.0.finish_reason").String())
	}
}

// TestPayloadPreservation ensures important fields are preserved during translation
func TestPayloadPreservation(t *testing.T) {
	mockToken := &authkiro.KiroTokenStorage{
		AccessToken: "test-token",
	}

	payloads := []string{
		"orignal",
		"orignal_tool_call",
		"orignal_tool_call_no_result",
		"orignal_tool_call_no_tools",
	}

	for _, filename := range payloads {
		t.Run(filename, func(t *testing.T) {
			payload := shared.LoadNonStreamRequest(t, filename)
			originalJSON := gjson.ParseBytes(payload)

			// Extract key fields from original
			originalModel := originalJSON.Get("model").String()
			originalMaxTokens := originalJSON.Get("max_tokens").Int()
			originalTemp := originalJSON.Get("temperature").Float()

			// Convert to Kiro
			kiroRequest, err := chat_completions.ConvertOpenAIRequestToKiro(
				originalModel,
				payload,
				mockToken,
				nil,
			)
			if err != nil {
				t.Fatalf("conversion failed: %v", err)
			}

			kiroJSON := gjson.ParseBytes(kiroRequest)
			convState := kiroJSON.Get("conversationState")

			// Verify key fields are preserved
			if convState.Get("model").String() != originalModel {
				t.Errorf("model not preserved: expected %q, got %q",
					originalModel, convState.Get("model").String())
			}

			if originalMaxTokens > 0 {
				if convState.Get("maxTokens").Int() != originalMaxTokens {
					t.Errorf("maxTokens not preserved: expected %d, got %d",
						originalMaxTokens, convState.Get("maxTokens").Int())
				}
			}

			if originalTemp > 0 {
				kiroTemp := convState.Get("temperature").Float()
				if kiroTemp != originalTemp {
					t.Errorf("temperature not preserved: expected %.2f, got %.2f",
						originalTemp, kiroTemp)
				}
			}
		})
	}
}

// TestKiroResponseMultiFormatNonStream verifies Kiro responses translate correctly for
// OpenAI Chat, Claude Messages, and OpenAI Responses callers.
func TestKiroResponseMultiFormatNonStream(t *testing.T) {
	kiroResp := []byte(`{
		"conversationState": {
			"currentMessage": {
				"assistantResponseMessage": {
					"content": "Weather looks clear.",
					"toolUses": [{
						"toolUseId": "call_weather",
						"name": "get_weather",
						"input": {"location": "Paris"}
					}]
				}
			}
		}
	}`)

	// OpenAI Chat format
	openAIResp := chat_completions.ConvertKiroResponseToOpenAI(kiroResp, "kiro-sonnet", false)
	parsed := gjson.ParseBytes(openAIResp)
	if parsed.Get("choices.0.message.content").String() != "Weather looks clear." {
		t.Fatalf("unexpected OpenAI content: %s", parsed.Get("choices.0.message.content").String())
	}
	if parsed.Get("choices.0.message.tool_calls.0.function.name").String() != "get_weather" {
		t.Fatalf("tool call not preserved in OpenAI response")
	}

	// Claude Messages format
	claudeResp := claude.ConvertKiroResponseToClaude(kiroResp, "kiro-sonnet", false)
	claudeJSON := gjson.ParseBytes(claudeResp)
	if claudeJSON.Get("type").String() != "message" {
		t.Fatalf("expected Claude type=message, got %s", claudeJSON.Get("type").String())
	}
	if claudeJSON.Get("content.#").Int() == 0 {
		t.Fatalf("Claude content should not be empty")
	}
	if claudeJSON.Get("content.1.type").String() != "tool_use" {
		t.Fatalf("expected tool_use block in Claude response")
	}

	// OpenAI Responses format
	origReq := []byte(`{"model":"kiro-sonnet","input":[{"role":"user","content":"Hi"}]}`)
	reqRaw := []byte(`{"conversationState":{}}`)
	var param any
	oaiResponses := kiroresponses.ConvertKiroResponseToOpenAIResponsesNonStream(
		context.Background(),
		"kiro-sonnet",
		origReq,
		reqRaw,
		kiroResp,
		&param,
	)
	oaiRespJSON := gjson.Parse(oaiResponses)
	if oaiRespJSON.Get("object").String() != "response" {
		t.Fatalf("expected object=response, got %s", oaiRespJSON.Get("object").String())
	}
	if oaiRespJSON.Get("output.0.content.0.text").String() == "" {
		t.Fatalf("OpenAI Responses output text missing")
	}
}

// TestKiroStreamingMultiFormat ensures streaming chunks translate for OpenAI Chat and Claude.
func TestKiroStreamingMultiFormat(t *testing.T) {
	kiroChunk := []byte(`{"type":"content_block_delta","delta":{"text":"Hello"}}`)

	openAIChunk := chat_completions.ConvertKiroStreamChunkToOpenAI(kiroChunk, "kiro-sonnet")
	if gjson.GetBytes(openAIChunk, "choices.0.delta.content").String() != "Hello" {
		t.Fatalf("OpenAI delta missing text")
	}

	claudeChunk := claude.ConvertKiroStreamChunkToClaude(kiroChunk, "kiro-sonnet")
	if gjson.GetBytes(claudeChunk, "type").String() != "content_block_delta" {
		t.Fatalf("expected Claude content_block_delta chunk")
	}
	if gjson.GetBytes(claudeChunk, "delta.text").String() != "Hello" {
		t.Fatalf("Claude delta text mismatch")
	}
	if gjson.GetBytes(claudeChunk, "choices").Exists() {
		t.Fatalf("Claude chunk should not contain OpenAI choices")
	}
}

// TestOpenAIResponsesRequestFallback ensures OpenAI Responses input is translated to Kiro.
func TestOpenAIResponsesRequestFallback(t *testing.T) {
	payload := []byte(`{
		"model":"kiro-sonnet",
		"input":[{"role":"user","content":"Fallback?"}],
		"response_format":"v1-responses"
	}`)

	req := kiroresponses.ConvertOpenAIResponsesRequestToKiro("kiro-sonnet", payload, false)
	jsonParsed := gjson.ParseBytes(req)
	if !jsonParsed.Get("conversationState.currentMessage").Exists() {
		t.Fatalf("conversationState.currentMessage missing in fallback conversion")
	}
	content := jsonParsed.Get("conversationState.currentMessage.userInputMessage.content").String()
	if content == "" {
		content = jsonParsed.Get("conversationState.currentMessage.content").String()
	}
	if content == "" {
		t.Fatalf("user content should be preserved in fallback conversion")
	}
}
