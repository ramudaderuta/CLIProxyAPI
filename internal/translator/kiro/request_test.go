package kiro

import (
	"encoding/json"
	"testing"

	authkiro "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
)

func TestBuildRequestWithSystemAndHistory(t *testing.T) {
	token := &authkiro.KiroTokenStorage{
		ProfileArn:  "arn:aws:codewhisperer:us-east-1:123456789012:profile/test",
		AuthMethod:  "social",
		AccessToken: "token",
	}
	payload := []byte(`{
        "system": "You are helpful.",
        "messages": [
            {"role": "user", "content": "Hello"},
            {"role": "assistant", "content": "Hi!"},
            {"role": "user", "content": "How are you?"}
        ]
    }`)

	body, err := BuildRequest("claude-sonnet-4-5", payload, token, map[string]any{"project": "demo"})
	if err != nil {
		t.Fatalf("BuildRequest returned error: %v", err)
	}

	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	if _, ok := req["profileArn"]; !ok {
		t.Fatalf("expected profileArn to be included for social auth")
	}

	conv, ok := req["conversationState"].(map[string]any)
	if !ok {
		t.Fatalf("conversationState missing or invalid: %v", req)
	}
	if conv["chatTriggerType"] != chatTrigger {
		t.Fatalf("unexpected chat trigger: %v", conv["chatTriggerType"])
	}
	if req["projectName"] != "demo" {
		t.Fatalf("projectName not propagated: %v", req["projectName"])
	}

	current := conv["currentMessage"].(map[string]any)
	userMessage := current["userInputMessage"].(map[string]any)
	if userMessage["modelId"] != MapModel("claude-sonnet-4-5") {
		t.Fatalf("model mapping incorrect: %v", userMessage["modelId"])
	}
	if userMessage["content"] != "How are you?" {
		t.Fatalf("current message content mismatch: %v", userMessage["content"])
	}

	history := conv["history"].([]any)
	if len(history) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(history))
	}
	first := history[0].(map[string]any)["userInputMessage"].(map[string]any)
	if first["content"].(string) == "" {
		t.Fatalf("system prompt should seed history content")
	}
}

func TestBuildRequestWithTools(t *testing.T) {
	token := &authkiro.KiroTokenStorage{AccessToken: "token"}
	payload := []byte(`{
        "messages": [
            {"role": "user", "content": [{"type":"text","text":"run"}]}
        ],
        "tools": [
            {
                "type": "function",
                "function": {
                    "name": "lookup",
                    "description": "Lookup data",
                    "parameters": {"type":"object"}
                }
            }
        ]
    }`)

	body, err := BuildRequest("claude-sonnet-4-5", payload, token, nil)
	if err != nil {
		t.Fatalf("BuildRequest returned error: %v", err)
	}

	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	conv := req["conversationState"].(map[string]any)
	current := conv["currentMessage"].(map[string]any)
	userMessage := current["userInputMessage"].(map[string]any)
	context, ok := userMessage["userInputMessageContext"].(map[string]any)
	if !ok {
		t.Fatalf("expected userInputMessageContext to be populated")
	}
	tools, ok := context["tools"].([]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("expected tool specification to be included: %v", context)
	}
}

func TestBuildRequestMissingMessages(t *testing.T) {
	token := &authkiro.KiroTokenStorage{AccessToken: "token"}
	if _, err := BuildRequest("claude-sonnet-4-5", []byte(`{"messages": []}`), token, nil); err == nil {
		t.Fatalf("expected error when messages array is empty")
	}
}

func TestMapModel(t *testing.T) {
	if MapModel("unknown") != MapModel("claude-sonnet-4-5") {
		t.Fatalf("unexpected fallback mapping")
	}
	if MapModel("claude-3-7-sonnet-20250219") != "CLAUDE_3_7_SONNET_20250219_V1_0" {
		t.Fatalf("mapping failed")
	}
}
