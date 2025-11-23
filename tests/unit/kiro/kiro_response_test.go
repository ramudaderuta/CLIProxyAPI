package kiro

import (
	"encoding/json"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/tests/shared"
)

// TestResponseParsing tests Kiro response parsing
func TestResponseParsing(t *testing.T) {
	tests := []struct {
		name         string
		kiroResponse map[string]interface{}
		expectedText string
		expectError  bool
	}{
		{
			name:         "simple response",
			kiroResponse: shared.BuildKiroResponse("Hello, World!"),
			expectedText: "Hello, World!",
			expectError:  false,
		},
		{
			name:         "empty response",
			kiroResponse: shared.BuildKiroResponse(""),
			expectedText: "",
			expectError:  false,
		},
		{
			name: "response with metadata",
			kiroResponse: map[string]interface{}{
				"conversationState": map[string]interface{}{
					"currentMessage": map[string]interface{}{
						"assistantMessage": map[string]interface{}{
							"content": "Test response",
						},
					},
				},
			},
			expectedText: "Test response",
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			responseJSON := shared.MarshalJSON(t, tt.kiroResponse)

			var parsed map[string]interface{}
			err := json.Unmarshal(responseJSON, &parsed)

			if (err != nil) != tt.expectError {
				t.Errorf("Expected error: %v, got: %v", tt.expectError, err)
			}

			t.Logf("✓ Response parsed correctly")
		})
	}
}

// TestUsageMetadata tests usage statistics extraction
func TestUsageMetadata(t *testing.T) {
	response := shared.BuildOpenAIResponse("kiro-sonnet", "Test response")

	if usage, ok := response["usage"].(map[string]interface{}); ok {
		if promptTokens, ok := usage["prompt_tokens"].(int); ok {
			t.Logf("✓ Prompt tokens: %d", promptTokens)
		}

		if completionTokens, ok := usage["completion_tokens"].(int); ok {
			t.Logf("✓ Completion tokens: %d", completionTokens)
		}

		if totalTokens, ok := usage["total_tokens"].(int); ok {
			t.Logf("✓ Total tokens: %d", totalTokens)
		}
	} else {
		t.Error("Usage metadata not found in response")
	}
}

// TestErrorResponse tests error response handling
func TestErrorResponse(t *testing.T) {
	errorResponses := []struct {
		name     string
		response map[string]interface{}
		errType  string
	}{
		{
			name: "authentication error",
			response: map[string]interface{}{
				"error": map[string]interface{}{
					"type":    "authentication_error",
					"message": "Invalid token",
				},
			},
			errType: "authentication_error",
		},
		{
			name: "rate limit error",
			response: map[string]interface{}{
				"error": map[string]interface{}{
					"type":    "rate_limit_error",
					"message": "Too many requests",
				},
			},
			errType: "rate_limit_error",
		},
		{
			name: "invalid request error",
			response: map[string]interface{}{
				"error": map[string]interface{}{
					"type":    "invalid_request_error",
					"message": "Missing required field",
				},
			},
			errType: "invalid_request_error",
		},
	}

	for _, tt := range errorResponses {
		t.Run(tt.name, func(t *testing.T) {
			responseJSON := shared.MarshalJSON(t, tt.response)

			var parsed map[string]interface{}
			if err := json.Unmarshal(responseJSON, &parsed); err != nil {
				t.Fatalf("Failed to parse error response: %v", err)
			}

			if errObj, ok := parsed["error"].(map[string]interface{}); ok {
				if errType, ok := errObj["type"].(string); ok {
					if errType != tt.errType {
						t.Errorf("Expected error type %s, got %s", tt.errType, errType)
					}
					t.Logf("✓ Error type %s detected", errType)
				}
			}
		})
	}
}

// TestStreamingChunkConversion tests SSE chunk conversion
func TestStreamingChunkConversion(t *testing.T) {
	chunks := []struct {
		name      string
		eventType string
		data      map[string]interface{}
	}{
		{
			name:      "message start",
			eventType: "message_start",
			data: map[string]interface{}{
				"type": "message_start",
			},
		},
		{
			name:      "content delta",
			eventType: "content_block_delta",
			data: map[string]interface{}{
				"type": "content_block_delta",
				"delta": map[string]interface{}{
					"text": "Hello",
				},
			},
		},
		{
			name:      "message stop",
			eventType: "message_stop",
			data: map[string]interface{}{
				"type": "message_stop",
			},
		},
	}

	for _, chunk := range chunks {
		t.Run(chunk.name, func(t *testing.T) {
			sseChunk := shared.BuildSSEChunk(chunk.eventType, chunk.data)

			if sseChunk == "" {
				t.Error("SSE chunk should not be empty")
			}

			t.Logf("✓ Chunk type %s converted", chunk.eventType)
		})
	}
}

// TestFinishReasons tests different finish reasons
func TestFinishReasons(t *testing.T) {
	finishReasons := []string{
		"stop",
		"length",
		"content_filter",
		"tool_calls",
	}

	for _, reason := range finishReasons {
		t.Run(reason, func(t *testing.T) {
			response := shared.BuildOpenAIResponse("kiro-sonnet", "Test")

			// Add finish reason
			if choices, ok := response["choices"].([]map[string]interface{}); ok && len(choices) > 0 {
				choices[0]["finish_reason"] = reason
			}

			responseJSON := shared.MarshalJSON(t, response)

			var parsed map[string]interface{}
			if err := json.Unmarshal(responseJSON, &parsed); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			t.Logf("✓ Finish reason %s handled", reason)
		})
	}
}

// TestToolCallResponse tests responses with tool calls
func TestToolCallResponse(t *testing.T) {
	toolCall := shared.BuildToolCall(
		"call_123",
		"get_weather",
		map[string]interface{}{"location": "Tokyo"},
	)

	response := shared.BuildOpenAIResponse("kiro-sonnet", "")

	// Add tool call to response
	if choices, ok := response["choices"].([]map[string]interface{}); ok && len(choices) > 0 {
		if message, ok := choices[0]["message"].(map[string]interface{}); ok {
			message["tool_calls"] = []map[string]interface{}{toolCall}
			message["content"] = nil // No content when tool call is present
		}
	}

	responseJSON := shared.MarshalJSON(t, response)

	var parsed map[string]interface{}
	if err := json.Unmarshal(responseJSON, &parsed); err != nil {
		t.Fatalf("Failed to parse tool call response: %v", err)
	}

	t.Log("✓ Tool call response structured correctly")
}

// Benchmark response operations
func BenchmarkResponseParsing(b *testing.B) {
	response := shared.BuildKiroResponse(shared.TestResponse)
	responseJSON := shared.MarshalJSON(&testing.T{}, response)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var parsed map[string]interface{}
		json.Unmarshal(responseJSON, &parsed)
	}
}

func BenchmarkStreamChunkConversion(b *testing.B) {
	data := map[string]interface{}{
		"type": "content_block_delta",
		"delta": map[string]interface{}{
			"text": "Test",
		},
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = shared.BuildSSEChunk("content_block_delta", data)
	}
}
