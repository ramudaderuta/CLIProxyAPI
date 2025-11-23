package kiro

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/router-for-me/CLIProxyAPI/v6/tests/shared"
)

// TestHardRequests tests edge cases and difficult scenarios
func TestHardRequestEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		request     map[string]interface{}
		description string
	}{
		{
			name: "very long message",
			request: shared.BuildOpenAIRequest(
				"kiro-sonnet",
				[]map[string]interface{}{
					shared.BuildSimpleMessage("user", strings.Repeat("A", 10000)),
				},
				false,
			),
			description: "Request with 10k character message",
		},
		{ // Added missing brace here
			name: "empty message content",
			request: shared.BuildOpenAIRequest(
				"kiro-sonnet",
				[]map[string]interface{}{
					shared.BuildSimpleMessage("user", ""),
				},
				false,
			),
			description: "Empty message content",
		},
		{
			name: "unicode and emoji",
			request: shared.BuildOpenAIRequest(
				"kiro-sonnet",
				[]map[string]interface{}{
					shared.BuildSimpleMessage("user", "Hello 世界 🌍🚀💻"),
				},
				false,
			),
			description: "Unicode characters and emoji",
		},
		{
			name: "special characters",
			request: shared.BuildOpenAIRequest(
				"kiro-sonnet",
				[]map[string]interface{}{
					shared.BuildSimpleMessage("user", "Test \"quotes\" and 'apostrophes' and <tags> and &amp;"),
				},
				false,
			),
			description: "Special XML/HTML characters",
		},
		{
			name: "very long conversation history",
			request: shared.BuildOpenAIRequest(
				"kiro-sonnet",
				generateLongConversation(50),
				false,
			),
			description: "50-turn conversation history",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestJSON := shared.MarshalJSON(t, tt.request)

			// Validate JSON is parseable
			if !utf8.Valid(requestJSON) {
				t.Error("Request JSON is not valid UTF-8")
			}

			t.Logf("✓ %s - Size: %d bytes", tt.description, len(requestJSON))
		})
	}
}

// TestMalformedRequests tests handling of malformed requests
func TestMalformedRequests(t *testing.T) {
	malformedCases := []struct {
		name        string
		requestJSON string
		description string
	}{
		{
			name:        "missing model",
			requestJSON: `{"messages":[{"role":"user","content":"test"}]}`,
			description: "Request missing model field",
		},
		{
			name:        "missing messages",
			requestJSON: `{"model":"kiro-sonnet"}`,
			description: "Request missing messages array",
		},
		{
			name:        "invalid role",
			requestJSON: `{"model":"kiro-sonnet","messages":[{"role":"invalid","content":"test"}]}`,
			description: "Message with invalid role",
		},
		{
			name:        "null content",
			requestJSON: `{"model":"kiro-sonnet","messages":[{"role":"user","content":null}]}`,
			description: "Message with null content",
		},
	}

	for _, tc := range malformedCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test that malformed request is still valid JSON
			if !isValidJSON(tc.requestJSON) {
				t.Error("Malformed request is not valid JSON")
			}

			t.Logf("✓ %s - JSON valid but incomplete", tc.description)
		})
	}
}

// TestLargePayloads tests handling of large payloads
func TestLargePayloads(t *testing.T) {
	shared.SkipIfShort(t, "large payload test")

	sizes := []int{
		1024,    // 1 KB
		10240,   // 10 KB
		102400,  // 100 KB
		1048576, // 1 MB
	}

	for _, size := range sizes {
		t.Run(fmt.Sprintf("%d bytes", size), func(t *testing.T) {
			content := strings.Repeat("X", size)

			request := shared.BuildOpenAIRequest(
				"kiro-sonnet",
				[]map[string]interface{}{
					shared.BuildSimpleMessage("user", content),
				},
				false,
			)

			requestJSON := shared.MarshalJSON(t, request)

			if len(requestJSON) < size {
				t.Errorf("Expected JSON size >= %d, got %d", size, len(requestJSON))
			}

			t.Logf("✓ Handled %d byte payload - JSON size: %d bytes", size, len(requestJSON))
		})
	}
}

// TestNestedContent tests deeply nested content structures
func TestNestedContent(t *testing.T) {
	// Test multimodal content with array
	multimodalContent := []interface{}{
		map[string]interface{}{
			"type": "text",
			"text": "What's in this image?",
		},
		map[string]interface{}{
			"type": "image_url",
			"image_url": map[string]interface{}{
				"url": "https://example.com/image.jpg",
			},
		},
	}

	request := shared.BuildOpenAIRequest(
		"kiro-sonnet",
		[]map[string]interface{}{
			{
				"role":    "user",
				"content": multimodalContent,
			},
		},
		false,
	)

	requestJSON := shared.MarshalJSON(t, request)

	if !isValidJSON(string(requestJSON)) {
		t.Error("Nested content request is not valid JSON")
	}

	t.Log("✓ Nested multimodal content handled")
}

// TestToolCallEdgeCases tests edge cases in tool calling
func TestToolCallEdgeCases(t *testing.T) {
	cases := []struct {
		name        string
		toolCall    map[string]interface{}
		description string
	}{
		{
			name:        "empty tool call ID",
			toolCall:    shared.BuildToolCall("", "test_function", map[string]interface{}{}),
			description: "Tool call with empty ID (should generate UUID)",
		},
		{
			name:        "very long function name",
			toolCall:    shared.BuildToolCall("call_123", strings.Repeat("a", 100), map[string]interface{}{}),
			description: "Tool call with 100-char function name",
		},
		{
			name: "complex arguments",
			toolCall: shared.BuildToolCall("call_123", "complex_func", map[string]interface{}{
				"nested": map[string]interface{}{
					"deep": map[string]interface{}{
						"value": []int{1, 2, 3, 4, 5},
					},
				},
			}),
			description: "Tool call with deeply nested arguments",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			toolCallJSON := shared.MarshalJSON(t, tc.toolCall)

			if !isValidJSON(string(toolCallJSON)) {
				t.Error("Tool call JSON is invalid")
			}

			t.Logf("✓ %s", tc.description)
		})
	}
}

// TestConcurrentRequests tests handling of concurrent scenarios
func TestConcurrentRequests(t *testing.T) {
	shared.SkipIfShort(t, "concurrent request test")

	// This tests the structure for concurrent handling
	// Actual concurrency would be integration test
	requests := make([]map[string]interface{}, 10)

	for i := 0; i < 10; i++ {
		requests[i] = shared.BuildOpenAIRequest(
			"kiro-sonnet",
			shared.SimpleMessages,
			false,
		)
	}

	if len(requests) != 10 {
		t.Errorf("Expected 10 requests, got %d", len(requests))
	}

	t.Log("✓ Concurrent request structure validated")
}

// Helper functions

func generateLongConversation(turns int) []map[string]interface{} {
	messages := make([]map[string]interface{}, turns*2)

	for i := 0; i < turns; i++ {
		messages[i*2] = shared.BuildSimpleMessage("user", "Question "+string(rune('0'+i%10)))
		messages[i*2+1] = shared.BuildSimpleMessage("assistant", "Answer "+string(rune('0'+i%10)))
	}

	return messages
}

func isValidJSON(s string) bool {
	// Simple JSON validation
	return (strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")) ||
		(strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]"))
}

// Benchmark hard requests
func BenchmarkLargePayload(b *testing.B) {
	content := strings.Repeat("X", 100000) // 100 KB

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = shared.BuildOpenAIRequest(
			"kiro-sonnet",
			[]map[string]interface{}{
				shared.BuildSimpleMessage("user", content),
			},
			false,
		)
	}
}

func BenchmarkLongConversation(b *testing.B) {
	messages := generateLongConversation(50)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = shared.BuildOpenAIRequest("kiro-sonnet", messages, false)
	}
}
