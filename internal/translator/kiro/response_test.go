package kiro

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
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
	text, calls := ParseResponse(body)
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

	text, calls := ParseResponse([]byte(stream))
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

func TestBuildOpenAIChatCompletionPayload(t *testing.T) {
	payload, err := BuildOpenAIChatCompletionPayload("claude-sonnet-4-5", "hi", []OpenAIToolCall{
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
	chunks := BuildStreamingChunks("chatcmpl_test", "claude", time.Now().Unix(), "hello", []OpenAIToolCall{
		{ID: "call-1", Name: "lookup", Arguments: `{"foo":1}`},
	})
	if len(chunks) < 3 {
		t.Fatalf("expected multiple streaming chunks, got %d", len(chunks))
	}
}
