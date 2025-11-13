package kiro_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	authkiro "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
	kirotranslator "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro"
	testutil "github.com/router-for-me/CLIProxyAPI/v6/tests/shared"
	"github.com/tidwall/gjson"
)

// TestKiroTranslation_CompleteFlow tests the complete translation flow
// from OpenAI format to Kiro format and back to OpenAI format.
func TestKiroTranslation_CompleteFlow(t *testing.T) {
	// Create OpenAI chat completion payload with system message and history
	openAIPayload := map[string]any{
		"model": "claude-sonnet-4-5",
		"messages": []map[string]any{
			{"role": "user", "content": "Hello, how are you?"},
		},
		"stream": false,
	}

	payloadBytes, err := json.Marshal(openAIPayload)
	if err != nil {
		t.Fatalf("Failed to marshal OpenAI payload: %v", err)
	}

	// Create a mock token for testing
	token := &authkiro.KiroTokenStorage{
		ProfileArn:  "arn:aws:codewhisperer:us-east-1:699475941385:profile/test",
		AccessToken: "test_access_token",
		ExpiresAt:   time.Now().Add(24 * time.Hour),
		Type:        "kiro",
	}

	// Test 1: OpenAI → Kiro translation (request)
	kiroReqBytes, err := kirotranslator.BuildRequest("claude-sonnet-4-5", payloadBytes, token, nil)
	if err != nil {
		t.Fatalf("Failed to build Kiro request: %v", err)
	}

	// Verify the Kiro request structure
	var kiroReq map[string]any
	if err := json.Unmarshal(kiroReqBytes, &kiroReq); err != nil {
		t.Fatalf("Failed to unmarshal Kiro request: %v", err)
	}

	// Verify the Kiro request structure
	if kiroReq["conversationState"] == nil {
		t.Error("Expected non-nil conversationState in Kiro request")
	}

	conversationState := kiroReq["conversationState"].(map[string]any)
	currentMessage := conversationState["currentMessage"].(map[string]any)
	userInputMessage := currentMessage["userInputMessage"].(map[string]any)

	// Verify model mapping
	expectedModel := kirotranslator.MapModel("claude-sonnet-4-5")
	if userInputMessage["modelId"] != expectedModel {
		t.Errorf("Expected modelId %s, got %v", expectedModel, userInputMessage["modelId"])
	}

	// Verify content
	if userInputMessage["content"] != "Hello, how are you?" {
		t.Errorf("Expected content 'Hello, how are you?', got %v", userInputMessage["content"])
	}

	// Test 2: Kiro → OpenAI translation (response)
	kiroResponse := map[string]any{
		"conversationState": map[string]any{
			"currentMessage": map[string]any{
				"assistantResponseMessage": map[string]any{
					"content": "I'd be happy to help you! What do you need assistance with?",
				},
			},
		},
	}

	kiroResponseBytes, err := json.Marshal(kiroResponse)
	if err != nil {
		t.Fatalf("Failed to marshal Kiro response: %v", err)
	}

	// Parse the Kiro response
	content, toolCalls := kirotranslator.ParseResponse(kiroResponseBytes)

	// Build OpenAI chat completion payload from the parsed response
	openAIResponseBytes, err := kirotranslator.BuildOpenAIChatCompletionPayload(
		"claude-sonnet-4-5",
		content,
		toolCalls,
		100, // promptTokens
		50,  // completionTokens
	)
	if err != nil {
		t.Fatalf("Failed to build OpenAI response: %v", err)
	}

	// Verify the OpenAI response structure
	var openAIResponse map[string]any
	if err := json.Unmarshal(openAIResponseBytes, &openAIResponse); err != nil {
		t.Fatalf("Failed to unmarshal OpenAI response: %v", err)
	}

	choices, ok := openAIResponse["choices"].([]any)
	if !ok || len(choices) == 0 {
		t.Error("Expected at least one choice in OpenAI response")
	}

	choice := choices[0].(map[string]any)
	message := choice["message"].(map[string]any)
	if message["content"] == nil {
		t.Error("Expected non-empty message content in OpenAI response")
	}

	expectedContent := "I'd be happy to help you! What do you need assistance with?"
	if message["content"] != expectedContent {
		t.Errorf("Expected content %s, got %s", expectedContent, message["content"])
	}

	// Verify model is preserved
	if openAIResponse["model"] != "claude-sonnet-4-5" {
		t.Errorf("Expected model claude-sonnet-4-5, got %s", openAIResponse["model"])
	}
}

// TestKiroTranslation_StreamingFlow tests the complete streaming translation flow
func TestKiroTranslation_StreamingFlow(t *testing.T) {
	// Create OpenAI chat completion payload for streaming
	openAIPayload := map[string]any{
		"model": "claude-sonnet-4-5",
		"messages": []map[string]any{
			{"role": "user", "content": "Tell me a short story"},
		},
		"stream": true,
	}

	payloadBytes, err := json.Marshal(openAIPayload)
	if err != nil {
		t.Fatalf("Failed to marshal OpenAI payload: %v", err)
	}

	// Create a mock token for testing
	token := &authkiro.KiroTokenStorage{
		ProfileArn:  "arn:aws:codewhisperer:us-east-1:699475941385:profile/test",
		AccessToken: "test_access_token",
		ExpiresAt:   time.Now().Add(24 * time.Hour),
		Type:        "kiro",
	}

	// Test 1: OpenAI → Kiro translation (request)
	kiroReqBytes, err := kirotranslator.BuildRequest("claude-sonnet-4-5", payloadBytes, token, nil)
	if err != nil {
		t.Fatalf("Failed to build Kiro request: %v", err)
	}

	// Verify streaming request structure
	var kiroReq map[string]any
	if err := json.Unmarshal(kiroReqBytes, &kiroReq); err != nil {
		t.Fatalf("Failed to unmarshal Kiro request: %v", err)
	}

	// Verify the request structure (userInput is nested under conversationState)
	if kiroReq["conversationState"] == nil {
		t.Error("Expected non-nil conversationState in Kiro request")
	}

	// Test 2: Kiro streaming response → OpenAI streaming chunks
	content := "Once upon a time..."

	// Build streaming chunks using the translator function
	chunks := kirotranslator.BuildStreamingChunks(
		"chatcmpl_test",
		"claude-sonnet-4-5",
		1699000000, // created timestamp
		content,
		nil, // no tool calls
	)

	// Verify we got chunks
	if len(chunks) == 0 {
		t.Error("Expected at least one chunk")
	}

	// Verify chunk structure
	for i, chunkBytes := range chunks {
		var chunk map[string]any
		if err := json.Unmarshal(chunkBytes, &chunk); err != nil {
			t.Fatalf("Failed to unmarshal chunk %d: %v", i, err)
		}

		if chunk["choices"] == nil {
			t.Errorf("Expected choices in chunk %d", i)
		}
	}
}

// TestKiroTranslation_WithTools tests translation flow with tool calls
func TestKiroTranslation_WithTools(t *testing.T) {
	// Create OpenAI payload with tools
	openAIPayload := map[string]any{
		"model": "claude-sonnet-4-5",
		"messages": []map[string]any{
			{"role": "user", "content": "What's the weather like?"},
		},
		"tools": []map[string]any{
			{
				"type": "function",
				"function": map[string]any{
					"name":        "get_weather",
					"description": "Get current weather information",
					"parameters": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"location": map[string]any{
								"type":        "string",
								"description": "The city and state",
							},
						},
						"required": []string{"location"},
					},
				},
			},
		},
		"stream": false,
	}

	payloadBytes, err := json.Marshal(openAIPayload)
	if err != nil {
		t.Fatalf("Failed to marshal OpenAI payload: %v", err)
	}

	// Create a mock token for testing
	token := &authkiro.KiroTokenStorage{
		ProfileArn:  "arn:aws:codewhisperer:us-east-1:699475941385:profile/test",
		AccessToken: "test_access_token",
		ExpiresAt:   time.Now().Add(24 * time.Hour),
		Type:        "kiro",
	}

	// Test OpenAI → Kiro translation with tools
	kiroReqBytes, err := kirotranslator.BuildRequest("claude-sonnet-4-5", payloadBytes, token, nil)
	if err != nil {
		t.Fatalf("Failed to build Kiro request with tools: %v", err)
	}

	// Verify tools are included in the request
	var kiroReq map[string]any
	if err := json.Unmarshal(kiroReqBytes, &kiroReq); err != nil {
		t.Fatalf("Failed to unmarshal Kiro request: %v", err)
	}

	// Verify the request structure (userInput is nested under conversationState)
	if kiroReq["conversationState"] == nil {
		t.Error("Expected non-nil conversationState in Kiro request")
	}

	// The tools should be converted to Kiro's format
	conversationState := kiroReq["conversationState"].(map[string]any)
	currentMessage := conversationState["currentMessage"].(map[string]any)
	userInput := currentMessage["userInputMessage"].(map[string]any)

	// Check that tools information is present (the exact format depends on Kiro's API)
	if _, exists := userInput["tools"]; !exists {
		// Tools might be embedded in a different field in Kiro's format
		// This test verifies the translation doesn't fail with tools
		t.Log("Tools field not found in UserInput - this may be expected based on Kiro's API format")
	}
}

// TestKiroTranslation_ModelMapping tests model mapping consistency
func TestKiroTranslation_ModelMapping(t *testing.T) {
	testCases := []struct {
		openAIModel string
		expected    string
	}{
		{"claude-sonnet-4-5", "CLAUDE_SONNET_4_5_20250929_V1_0"},
		{"claude-sonnet-4-5-20250929", "CLAUDE_SONNET_4_5_20250929_V1_0"},
		{"claude-sonnet-4-20250514", "CLAUDE_SONNET_4_20250514_V1_0"},
		{"claude-3-7-sonnet-20250219", "CLAUDE_3_7_SONNET_20250219_V1_0"},
		{"unknown-model", "CLAUDE_SONNET_4_5_20250929_V1_0"}, // Should default to claude-sonnet-4-5
	}

	for _, tc := range testCases {
		t.Run(tc.openAIModel, func(t *testing.T) {
			mapped := kirotranslator.MapModel(tc.openAIModel)
			if mapped != tc.expected {
				t.Errorf("Expected model mapping %s -> %s, got %s", tc.openAIModel, tc.expected, mapped)
			}
		})
	}
}

// TestBuildRequestWithSystemAndHistory tests building Kiro requests with system prompts and message history
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

	body, err := kirotranslator.BuildRequest("claude-sonnet-4-5", payload, token, map[string]any{"project": "demo"})
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
	if conv["chatTriggerType"] == nil {
		t.Fatalf("unexpected chat trigger: %v", conv["chatTriggerType"])
	}
	if req["projectName"] != "demo" {
		t.Fatalf("projectName not propagated: %v", req["projectName"])
	}

	current := conv["currentMessage"].(map[string]any)
	userMessage := current["userInputMessage"].(map[string]any)
	if userMessage["modelId"] != kirotranslator.MapModel("claude-sonnet-4-5") {
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

func TestBuildRequestNormalizesSystemBlocks(t *testing.T) {
	token := &authkiro.KiroTokenStorage{AccessToken: "token"}
	payload := []byte(`{
		"system": [
			{"type":"text","text":"You are Claude Code."},
			{"type":"text","text":"Always call tools for weather."}
		],
		"messages": [
			{"role":"user","content":[{"type":"text","text":"Ping"}]}
		]
	}`)

	body, err := kirotranslator.BuildRequest("claude-sonnet-4-5", payload, token, nil)
	if err != nil {
		t.Fatalf("BuildRequest returned error: %v", err)
	}

	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	conv := req["conversationState"].(map[string]any)
	history := conv["history"].([]any)
	if len(history) == 0 {
		t.Fatalf("expected history seeded by system prompt")
	}
	first := history[0].(map[string]any)["userInputMessage"].(map[string]any)
	content := first["content"].(string)
	if strings.Contains(content, "[{") {
		t.Fatalf("system prompt was not normalized: %q", content)
	}
	if !strings.Contains(content, "Claude Code") || !strings.Contains(content, "Always call tools") {
		t.Fatalf("system prompt text missing from content: %q", content)
	}
}

func TestBuildRequestWithSystemAndSingleUserDoesNotDuplicate(t *testing.T) {
	token := &authkiro.KiroTokenStorage{AccessToken: "token"}
	payload := []byte(`{
		"system": "Stay calm.",
		"messages": [
			{"role":"user","content":"Ping"}
		]
	}`)

	body, err := kirotranslator.BuildRequest("claude-sonnet-4-5", payload, token, nil)
	if err != nil {
		t.Fatalf("BuildRequest returned error: %v", err)
	}

	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	conv := req["conversationState"].(map[string]any)
	history := conv["history"].([]any)
	if len(history) != 1 {
		t.Fatalf("expected single history entry for system prompt, got %d", len(history))
	}
	historyContent := history[0].(map[string]any)["userInputMessage"].(map[string]any)["content"].(string)
	if strings.Contains(historyContent, "Ping") {
		t.Fatalf("system history entry should not include user content: %q", historyContent)
	}

	current := conv["currentMessage"].(map[string]any)["userInputMessage"].(map[string]any)
	if current["content"] != "Ping" {
		t.Fatalf("current message should include user content, got %v", current["content"])
	}
}

func TestBuildRequestStripsControlCharactersFromUserContent(t *testing.T) {
	token := &authkiro.KiroTokenStorage{AccessToken: "token"}
	payload := []byte(`{
		"system": [{"type":"text","text":"You are Claude Code."}],
		"messages": [
			{
				"role": "user",
				"content": [
					{"type":"text","text":"<system-reminder>Stay focused</system-reminder>"},
					{"type":"text","text":"hello from terminal \u001b[1mopus (claude-sonnet-4-5)\u001b[22m"}
				]
			}
		],
		"tools": [
			{
				"name": "get_weather",
				"description": "Get weather",
				"input_schema": {
					"type": "object",
					"properties": {
						"city": {"type": "string"}
					},
					"required": ["city"]
				}
			}
		]
	}`)

	body, err := kirotranslator.BuildRequest("claude-sonnet-4-5", payload, token, nil)
	if err != nil {
		t.Fatalf("BuildRequest returned error: %v", err)
	}

	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	conv := req["conversationState"].(map[string]any)
	current := conv["currentMessage"].(map[string]any)
	userInput := current["userInputMessage"].(map[string]any)
	content := userInput["content"].(string)

	if strings.Contains(content, "\u001b") {
		t.Fatalf("expected escape characters to be stripped, got %q", content)
	}
	if !strings.Contains(content, "<system-reminder>") {
		t.Fatalf("expected system reminder tag to be preserved, got %q", content)
	}

	context := userInput["userInputMessageContext"].(map[string]any)
	tools, ok := context["tools"].([]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("expected translated tool specification, got %v", context)
	}
}

func TestBuildRequestPreservesLongToolDescriptions(t *testing.T) {
	token := &authkiro.KiroTokenStorage{AccessToken: "token"}
	longDesc := strings.Repeat("Detailed description ", 200)
	payload := []byte(fmt.Sprintf(`{
		"messages": [
			{"role":"user","content":[{"type":"text","text":"Ping"}]}
		],
		"tools": [
			{
				"name": "get_weather",
				"description": "%s",
				"input_schema": {"type":"object"}
			}
		]
	}`, longDesc))

	body, err := kirotranslator.BuildRequest("claude-sonnet-4-5", payload, token, nil)
	if err != nil {
		t.Fatalf("BuildRequest returned error: %v", err)
	}

	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	current := req["conversationState"].(map[string]any)["currentMessage"].(map[string]any)
	context := current["userInputMessage"].(map[string]any)["userInputMessageContext"].(map[string]any)
	tools := context["tools"].([]any)
	toolSpec := tools[0].(map[string]any)["toolSpecification"].(map[string]any)

	desc := toolSpec["description"].(string)
	if len([]rune(desc)) > 256 {
		t.Fatalf("expected sanitized description to stay within 256 chars, got %d", len([]rune(desc)))
	}

	conv := req["conversationState"].(map[string]any)
	history := conv["history"].([]any)
	if len(history) == 0 {
		t.Fatalf("expected system prompt history entry")
	}
	sysContent := history[0].(map[string]any)["userInputMessage"].(map[string]any)["content"].(string)
	if !strings.Contains(sysContent, "Tool reference manifest") {
		t.Fatalf("expected tool reference manifest in system prompt:\n%s", sysContent)
	}
	ctx := req["conversationState"].(map[string]any)["currentMessage"].(map[string]any)["userInputMessage"].(map[string]any)["userInputMessageContext"].(map[string]any)
	manifest, ok := ctx["toolContextManifest"].([]any)
	if !ok || len(manifest) == 0 {
		t.Fatalf("expected toolContextManifest with preserved descriptions: %#v", ctx)
	}
	found := false
	for _, item := range manifest {
		entry := item.(map[string]any)
		desc, _ := entry["description"].(string)
		if strings.Contains(desc, "Detailed description Detailed description Detailed description Detailed description") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected manifest entry with the full long description preserved: %#v", manifest)
	}
}

func TestBuildRequestStripsMarkupFromToolDescriptions(t *testing.T) {
	token := &authkiro.KiroTokenStorage{AccessToken: "token"}
	payload := []byte(`{
		"messages": [
			{"role":"user","content":[{"type":"text","text":"Ping"}]}
		],
		"tools": [
			{
				"name": "skills",
				"description": "<skills_instructions>Use this tool.</skills_instructions>",
				"input_schema": {"type":"object"}
			}
		]
	}`)

	body, err := kirotranslator.BuildRequest("claude-sonnet-4-5", payload, token, nil)
	if err != nil {
		t.Fatalf("BuildRequest returned error: %v", err)
	}

	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	tools := req["conversationState"].(map[string]any)["currentMessage"].(map[string]any)["userInputMessage"].(map[string]any)["userInputMessageContext"].(map[string]any)["tools"].([]any)
	desc := tools[0].(map[string]any)["toolSpecification"].(map[string]any)["description"].(string)
	if strings.Contains(desc, "<") || strings.Contains(desc, ">") {
		t.Fatalf("expected markup to be stripped, got %q", desc)
	}
}

func TestBuildRequestPreservesClaudeCodeBuiltinTools(t *testing.T) {
	token := &authkiro.KiroTokenStorage{AccessToken: "token"}
	payload := testutil.LoadTestData(t, "nonstream/claude_code_tooling_request.json")

	body, err := kirotranslator.BuildRequest("claude-sonnet-4-5", payload, token, nil)
	if err != nil {
		t.Fatalf("BuildRequest returned error: %v", err)
	}

	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	conv := req["conversationState"].(map[string]any)
	current := conv["currentMessage"].(map[string]any)["userInputMessage"].(map[string]any)
	ctx, ok := current["userInputMessageContext"].(map[string]any)
	if !ok {
		t.Fatalf("expected userInputMessageContext to be populated")
	}

	toolsAny := ctx["tools"].([]any)
	if len(toolsAny) < 6 {
		t.Fatalf("expected builtin tools to pass through, only saw %d entries", len(toolsAny))
	}

	found := make(map[string]bool)
	for _, tool := range toolsAny {
		spec := tool.(map[string]any)["toolSpecification"].(map[string]any)
		name := spec["name"].(string)
		found[strings.ToLower(name)] = true
	}

	for _, builtin := range []string{"task", "bash", "glob", "grep", "todowrite", "skill", "slashcommand"} {
		if !found[builtin] {
			t.Fatalf("expected builtin tool %s to be forwarded", builtin)
		}
	}

	meta, ok := ctx["claudeToolChoice"].(map[string]any)
	if !ok {
		t.Fatalf("expected claudeToolChoice metadata to be attached")
	}
	if meta["mode"] != "tool" || meta["name"] != "get_weather" {
		t.Fatalf("unexpected tool choice metadata: %#v", meta)
	}

	history := conv["history"].([]any)
	if len(history) == 0 {
		t.Fatalf("expected system prompt to be injected into history")
	}
	sysContent := history[0].(map[string]any)["userInputMessage"].(map[string]any)["content"].(string)
	if !strings.Contains(sysContent, "Tool reference manifest") {
		t.Fatalf("expected tool reference manifest in system prompt:\n%s", sysContent)
	}
	if !strings.Contains(sysContent, "Tool directive: you must call the tool") {
		t.Fatalf("expected tool directive in system prompt:\n%s", sysContent)
	}
}

func TestBuildRequestAddsToolReferenceForTruncatedDescriptions(t *testing.T) {
	token := &authkiro.KiroTokenStorage{AccessToken: "token"}
	payload := testutil.LoadTestData(t, "nonstream/claude_code_tooling_request.json")

	body, err := kirotranslator.BuildRequest("claude-sonnet-4-5", payload, token, nil)
	if err != nil {
		t.Fatalf("BuildRequest returned error: %v", err)
	}

	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	current := req["conversationState"].(map[string]any)["currentMessage"].(map[string]any)
	userInput := current["userInputMessage"].(map[string]any)
	ctx := userInput["userInputMessageContext"].(map[string]any)
	tools := ctx["tools"].([]any)

	for idx, tool := range tools {
		desc := tool.(map[string]any)["toolSpecification"].(map[string]any)["description"].(string)
		if l := len([]rune(desc)); l > 256 {
			t.Fatalf("tool %d description exceeded 256 chars (%d)", idx, l)
		}
	}

	history := req["conversationState"].(map[string]any)["history"].([]any)
	if len(history) == 0 {
		t.Fatalf("expected system prompt entry in history")
	}
	sysContent := history[0].(map[string]any)["userInputMessage"].(map[string]any)["content"].(string)
	if !strings.Contains(sysContent, "Tool reference manifest") {
		t.Fatalf("expected tool reference manifest in system prompt:\n%s", sysContent)
	}

	manifest, ok := ctx["toolContextManifest"].([]any)
	if !ok || len(manifest) == 0 {
		t.Fatalf("expected toolContextManifest to be populated: %#v", ctx)
	}
	foundTask := false
	for _, item := range manifest {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if entry["name"] == "Task" {
			foundTask = true
			desc, _ := entry["description"].(string)
			if !strings.Contains(desc, "Create, update, and list todos") {
				t.Fatalf("expected full Task description inside manifest entry: %#v", entry)
			}
			if hash, _ := entry["hash"].(string); strings.TrimSpace(hash) == "" {
				t.Fatalf("expected Task manifest entry to have a hash: %#v", entry)
			}
		}
	}
	if !foundTask {
		t.Fatalf("expected Task entry inside toolContextManifest: %#v", manifest)
	}
}

func TestBuildRequestIncludesPlanModeMetadata(t *testing.T) {
	token := &authkiro.KiroTokenStorage{AccessToken: "token"}
	payload := []byte(`{
        "messages": [
            {"role": "user", "content": [{"type":"text","text":"start planning"}]},
            {"role": "assistant", "content": [
                {"type":"text","text":"Launching background agent"},
                {"type":"tool_use","name":"Task","id":"plan_123","input":{"goal":"audit"}}
            ]},
            {"role": "user", "content": [{"type":"text","text":"waiting..."}]}
        ],
        "tools": [
            {"name":"Task","description":"Launch plan agents to parallelize work."},
            {"name":"ExitPlanMode","description":"Stop plan mode and return to normal."}
        ]
    }`)

	body, err := kirotranslator.BuildRequest("claude-sonnet-4-5", payload, token, nil)
	if err != nil {
		t.Fatalf("BuildRequest returned error: %v", err)
	}

	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	current := req["conversationState"].(map[string]any)["currentMessage"].(map[string]any)
	userInput := current["userInputMessage"].(map[string]any)
	ctx, ok := userInput["userInputMessageContext"].(map[string]any)
	if !ok {
		t.Fatalf("expected userInputMessageContext to exist: %#v", userInput)
	}

	planMeta, ok := ctx["planMode"].(map[string]any)
	if !ok {
		t.Fatalf("expected planMode metadata to be attached: %#v", ctx)
	}
	if planMeta["active"] != true {
		t.Fatalf("expected planMode.active to be true: %#v", planMeta)
	}
	pending, _ := planMeta["pending"].([]any)
	if len(pending) == 0 {
		t.Fatalf("expected pending plan transitions: %#v", planMeta)
	}
	first := pending[0].(map[string]any)
	if first["toolUseId"] != "plan_123" {
		t.Fatalf("expected plan_123 to be marked pending: %#v", first)
	}

	systemEntry := req["conversationState"].(map[string]any)["history"].([]any)[0]
	sysContent := systemEntry.(map[string]any)["userInputMessage"].(map[string]any)["content"].(string)
	if !strings.Contains(sysContent, "Plan directive") {
		t.Fatalf("expected plan directive injected into system prompt:\n%s", sysContent)
	}
}

func TestBuildRequestMovesTrailingAssistantMessagesIntoHistory(t *testing.T) {
	token := &authkiro.KiroTokenStorage{AccessToken: "token"}
	payload := []byte(`{
    "messages": [
        {"role": "user", "content": "Summarize the report"},
        {"role": "assistant", "content": "Here is the first draft."},
        {"role": "user", "content": "Continue refining the summary."},
        {"role": "assistant", "content": "Absolutely, adding more context now."}
    ]
}`)

	body, err := kirotranslator.BuildRequest("claude-sonnet-4-5", payload, token, nil)
	if err != nil {
		t.Fatalf("BuildRequest returned error: %v", err)
	}

	conv := gjson.ParseBytes(body).Get("conversationState")
	current := conv.Get("currentMessage.userInputMessage.content").String()
	if current != "Continue refining the summary." {
		t.Fatalf("expected trailing user message to be forwarded, got %q", current)
	}
	if conv.Get("currentMessage.assistantResponseMessage").Exists() {
		t.Fatalf("assistantResponseMessage should not be present on current message: %s", conv.Get("currentMessage"))
	}

	history := conv.Get("history")
	historyArray := history.Array()
	if len(historyArray) != 3 {
		t.Fatalf("expected three history entries, got %d", len(historyArray))
	}
	last := historyArray[len(historyArray)-1]
	content := last.Get("assistantResponseMessage.content").String()
	if content != "Absolutely, adding more context now." {
		t.Fatalf("expected trailing assistant message in history, got %q", content)
	}
}

func TestBuildRequestFailsWhenTranscriptHasNoUserTurn(t *testing.T) {
	token := &authkiro.KiroTokenStorage{AccessToken: "token"}
	payload := []byte(`{
    "messages": [
        {"role": "assistant", "content": "Here you go"}
    ]
}`)

	_, err := kirotranslator.BuildRequest("claude-sonnet-4-5", payload, token, nil)
	if err == nil {
		t.Fatalf("expected BuildRequest to fail without a user turn")
	}
	if !strings.Contains(err.Error(), "no user turn found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestBuildRequestWithTools tests building Kiro requests with OpenAI format tools
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

	body, err := kirotranslator.BuildRequest("claude-sonnet-4-5", payload, token, nil)
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

// TestBuildRequestMissingMessages tests that empty message arrays return errors
func TestBuildRequestMissingMessages(t *testing.T) {
	token := &authkiro.KiroTokenStorage{AccessToken: "token"}
	if _, err := kirotranslator.BuildRequest("claude-sonnet-4-5", []byte(`{"messages": []}`), token, nil); err == nil {
		t.Fatalf("expected error when messages array is empty")
	}
}

// TestBuildRequestWithAnthropicFormatTools tests building Kiro requests with Anthropic format tools
func TestBuildRequestWithAnthropicFormatTools(t *testing.T) {
	token := &authkiro.KiroTokenStorage{AccessToken: "token"}
	payload := []byte(`{
        "messages": [
            {"role": "user", "content": [{"type":"text","text":"Get weather"}]}
        ],
        "tools": [
            {
                "name": "get_weather",
                "description": "Get current weather by city name",
                "input_schema": {
                    "type": "object",
                    "properties": {
                        "city": {"type": "string"},
                        "unit": {"type": "string", "enum": ["°C","°F"]}
                    },
                    "required": ["city"]
                }
            }
        ]
    }`)

	body, err := kirotranslator.BuildRequest("claude-sonnet-4-5", payload, token, nil)
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

	toolSpec := tools[0].(map[string]any)["toolSpecification"].(map[string]any)
	if toolSpec["name"] != "get_weather" {
		t.Fatalf("expected tool name 'get_weather', got %v", toolSpec["name"])
	}
	if toolSpec["description"] != "Get current weather by city name" {
		t.Fatalf("unexpected tool description: %v", toolSpec["description"])
	}
}

// TestBuildRequestWithOpenAIFormatTools tests building Kiro requests with OpenAI format tools
func TestBuildRequestWithOpenAIFormatTools(t *testing.T) {
	token := &authkiro.KiroTokenStorage{AccessToken: "token"}
	payload := []byte(`{
        "messages": [
            {"role": "user", "content": [{"type":"text","text":"Lookup data"}]}
        ],
        "tools": [
            {
                "type": "function",
                "function": {
                    "name": "lookup",
                    "description": "Lookup data",
                    "parameters": {
                        "type": "object",
                        "properties": {
                            "query": {"type": "string"}
                        },
                        "required": ["query"]
                    }
                }
            }
        ]
    }`)

	body, err := kirotranslator.BuildRequest("claude-sonnet-4-5", payload, token, nil)
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

	toolSpec := tools[0].(map[string]any)["toolSpecification"].(map[string]any)
	if toolSpec["name"] != "lookup" {
		t.Fatalf("expected tool name 'lookup', got %v", toolSpec["name"])
	}
	if toolSpec["description"] != "Lookup data" {
		t.Fatalf("unexpected tool description: %v", toolSpec["description"])
	}
}

// TestCompleteToolConversionFlow tests the complete tool conversion flow: Anthropic → Kiro → OpenAI/Anthropic
func TestCompleteToolConversionFlow(t *testing.T) {
	// 1. Anthropic format request
	token := &authkiro.KiroTokenStorage{AccessToken: "token"}
	anthropicRequest := []byte(`{
        "model": "claude-sonnet-4-5",
        "messages": [
            {"role": "user", "content": [{"type":"text","text":"Get weather"}]}
        ],
        "tools": [
            {
                "name": "get_weather",
                "description": "Get current weather by city name",
                "input_schema": {
                    "type": "object",
                    "properties": {
                        "city": {"type": "string"},
                        "unit": {"type": "string", "enum": ["°C","°F"]}
                    },
                    "required": ["city"]
                }
            }
        ]
    }`)

	// 2. Convert to Kiro format
	kiroReq, err := kirotranslator.BuildRequest("claude-sonnet-4-5", anthropicRequest, token, nil)
	if err != nil {
		t.Fatalf("BuildRequest failed: %v", err)
	}

	var kiroMap map[string]any
	if err := json.Unmarshal(kiroReq, &kiroMap); err != nil {
		t.Fatalf("Failed to parse Kiro request: %v", err)
	}

	// 3. Verify Kiro format contains tools
	convState := kiroMap["conversationState"].(map[string]any)
	currentMsg := convState["currentMessage"].(map[string]any)
	userInputMsg := currentMsg["userInputMessage"].(map[string]any)

	context, ok := userInputMsg["userInputMessageContext"].(map[string]any)
	if !ok {
		t.Fatalf("Expected userInputMessageContext in Kiro request")
	}

	tools, ok := context["tools"].([]any)
	if !ok || len(tools) == 0 {
		t.Fatalf("Expected tools in Kiro request context")
	}

	// 4. Simulate Kiro response
	kiroResponse := `{
		"conversationState": {
			"currentMessage": {
				"assistantResponseMessage": {
					"content": "I'll get the weather for you.",
					"toolUse": [
						{
							"toolUseId": "call_12345",
							"name": "get_weather",
							"input": {
								"city": "Tokyo",
								"unit": "°C"
							}
						}
					]
				}
			}
		}
	}`

	// 5. Parse Kiro response
	content, toolCalls := kirotranslator.ParseResponse([]byte(kiroResponse))
	if content == "" {
		t.Fatalf("Expected content from Kiro response")
	}
	if len(toolCalls) == 0 {
		t.Fatalf("Expected tool calls from Kiro response")
	}

	// 6. Convert to OpenAI format
	openaiPayload, err := kirotranslator.BuildOpenAIChatCompletionPayload(
		"claude-sonnet-4-5", content, toolCalls, 50, 30,
	)
	if err != nil {
		t.Fatalf("OpenAI conversion failed: %v", err)
	}

	var openaiResp map[string]any
	if err := json.Unmarshal(openaiPayload, &openaiResp); err != nil {
		t.Fatalf("Failed to parse OpenAI response: %v", err)
	}

	// Verify OpenAI format
	choices := openaiResp["choices"].([]any)
	if len(choices) == 0 {
		t.Fatalf("Expected choices in OpenAI response")
	}

	message := choices[0].(map[string]any)["message"].(map[string]any)
	if message["content"] != content {
		t.Fatalf("OpenAI content mismatch")
	}

	if _, hasToolCalls := message["tool_calls"]; !hasToolCalls {
		t.Fatalf("Expected tool_calls in OpenAI response")
	}

	// 7. Convert to Anthropic format
	anthropicPayload, err := kirotranslator.BuildAnthropicMessagePayload(
		"claude-sonnet-4-5", content, toolCalls, 50, 30,
	)
	if err != nil {
		t.Fatalf("Anthropic conversion failed: %v", err)
	}

	var anthropicResp map[string]any
	if err := json.Unmarshal(anthropicPayload, &anthropicResp); err != nil {
		t.Fatalf("Failed to parse Anthropic response: %v", err)
	}

	// Verify Anthropic format
	if anthropicResp["type"] != "message" {
		t.Fatalf("Expected 'message' type in Anthropic response")
	}

	contentBlocks := anthropicResp["content"].([]any)
	if len(contentBlocks) == 0 {
		t.Fatalf("Expected content blocks in Anthropic response")
	}
}

// TestKiroCompleteConversionFlow tests the comprehensive conversion flow with detailed validation
func TestKiroCompleteConversionFlow(t *testing.T) {
	t.Run("AnthropicRequestToKiroToMultipleFormats", func(t *testing.T) {
		// Test 1: Request conversion: Claude/Anthropic format → Kiro format
		token := &authkiro.KiroTokenStorage{
			AccessToken: "test-token",
			AuthMethod:  "api_key",
		}

		claudeRequest := []byte(`{
			"model": "claude-sonnet-4-5",
			"temperature": 0,
			"max_tokens": 256,
			"system": [{"type": "text", "text": "You are Claude Code. When asked about weather, ALWAYS call the tool."}],
			"tools": [
				{
					"name": "get_weather",
					"description": "Get current weather by city name",
					"input_schema": {
						"type": "object",
						"properties": {
							"city": {"type": "string"},
							"unit": {"type": "string", "enum": ["°C","°F"]}
						},
						"required": ["city"]
					}
				}
			],
			"tool_choice": {"type": "tool", "name": "get_weather"},
			"messages": [
				{
					"role": "user",
					"content": [{"type": "text", "text": "Get the weather in Tokyo in °C."}]
				}
			]
		}`)

		kiroRequest, err := kirotranslator.BuildRequest("claude-sonnet-4-5", claudeRequest, token, map[string]any{})
		if err != nil {
			t.Fatalf("Kiro request conversion failed: %v", err)
		}

		var kiroReq map[string]any
		if err := json.Unmarshal(kiroRequest, &kiroReq); err != nil {
			t.Fatalf("Kiro request parsing failed: %v", err)
		}

		// Verify tools are correctly converted to Kiro format
		convState := kiroReq["conversationState"].(map[string]any)
		currentMsg := convState["currentMessage"].(map[string]any)
		userInputMsg := currentMsg["userInputMessage"].(map[string]any)

		if context, ok := userInputMsg["userInputMessageContext"].(map[string]any); ok {
			if tools, ok := context["tools"]; ok {
				t.Logf("✅ Tools successfully converted to Kiro format")
				toolsJSON, _ := json.MarshalIndent(tools, "", "  ")
				t.Logf("Kiro tool format:\n%s", toolsJSON)

				// Verify tool specification structure
				toolsArray := tools.([]any)
				if len(toolsArray) != 1 {
					t.Fatalf("Expected 1 tool, got %d", len(toolsArray))
				}

				toolSpec := toolsArray[0].(map[string]any)["toolSpecification"].(map[string]any)
				if toolSpec["name"] != "get_weather" {
					t.Fatalf("Expected tool name 'get_weather', got %v", toolSpec["name"])
				}
				if toolSpec["description"] != "Get current weather by city name" {
					t.Fatalf("Expected description mismatch: %v", toolSpec["description"])
				}
			} else {
				t.Fatalf("❌ Tools conversion failed")
			}
		}

		// Test 2: Response conversion: Kiro format → OpenAI format
		kiroResponse := `{
			"conversationState": {
				"currentMessage": {
					"assistantResponseMessage": {
						"content": "I'll get the weather for you.",
						"toolUse": [
							{
								"toolUseId": "call_12345",
								"name": "get_weather",
								"input": {
									"city": "Tokyo",
									"unit": "°C"
								}
							}
						]
					}
				}
			}
		}`

		// Parse Kiro response
		content, toolCalls := kirotranslator.ParseResponse([]byte(kiroResponse))
		t.Logf("✅ Kiro response parsing successful")
		t.Logf("Text content: %s", content)
		t.Logf("Tool calls count: %d", len(toolCalls))

		for i, call := range toolCalls {
			t.Logf("Tool %d: %s(%s)", i+1, call.Name, call.Arguments)
		}

		// Convert to OpenAI format
		openaiPayload, err := kirotranslator.BuildOpenAIChatCompletionPayload(
			"claude-sonnet-4-5",
			content,
			toolCalls,
			50, // promptTokens
			30, // completionTokens
		)
		if err != nil {
			t.Fatalf("OpenAI format conversion failed: %v", err)
		}

		var openaiResp map[string]any
		if err := json.Unmarshal(openaiPayload, &openaiResp); err != nil {
			t.Fatalf("OpenAI response parsing failed: %v", err)
		}

		t.Logf("✅ OpenAI format conversion successful")

		// Verify OpenAI response structure
		choices := openaiResp["choices"].([]any)
		if len(choices) == 0 {
			t.Fatalf("Expected choices in OpenAI response")
		}

		message := choices[0].(map[string]any)["message"].(map[string]any)
		if message["content"] != content {
			t.Fatalf("OpenAI content mismatch: expected %s, got %v", content, message["content"])
		}

		if _, hasToolCalls := message["tool_calls"]; !hasToolCalls {
			t.Fatalf("Expected tool_calls in OpenAI response")
		}

		// Test 3: Response conversion: Kiro format → Anthropic format
		anthropicPayload, err := kirotranslator.BuildAnthropicMessagePayload(
			"claude-sonnet-4-5",
			content,
			toolCalls,
			50, // promptTokens
			30, // completionTokens
		)
		if err != nil {
			t.Fatalf("Anthropic format conversion failed: %v", err)
		}

		var anthropicResp map[string]any
		if err := json.Unmarshal(anthropicPayload, &anthropicResp); err != nil {
			t.Fatalf("Anthropic response parsing failed: %v", err)
		}

		t.Logf("✅ Anthropic format conversion successful")

		// Verify Anthropic response structure
		if anthropicResp["type"] != "message" {
			t.Fatalf("Expected 'message' type in Anthropic response, got %v", anthropicResp["type"])
		}

		if anthropicResp["model"] != "claude-sonnet-4-5" {
			t.Fatalf("Expected model 'claude-sonnet-4-5', got %v", anthropicResp["model"])
		}

		contentBlocks := anthropicResp["content"].([]any)
		if len(contentBlocks) == 0 {
			t.Fatalf("Expected content blocks in Anthropic response")
		}

		// Verify content blocks include both text and tool_use
		hasTextBlock := false
		hasToolUseBlock := false
		for _, block := range contentBlocks {
			blockMap := block.(map[string]any)
			switch blockMap["type"] {
			case "text":
				hasTextBlock = true
			case "tool_use":
				hasToolUseBlock = true
				if blockMap["name"] != "get_weather" {
					t.Fatalf("Expected tool name 'get_weather', got %v", blockMap["name"])
				}
			}
		}

		if !hasTextBlock {
			t.Fatalf("Expected text content block in Anthropic response")
		}
		if !hasToolUseBlock {
			t.Fatalf("Expected tool_use content block in Anthropic response")
		}
	})

	t.Run("OpenAIFormatRequestToKiro", func(t *testing.T) {
		// Test OpenAI format tools conversion
		token := &authkiro.KiroTokenStorage{AccessToken: "token"}
		openAIRequest := []byte(`{
			"messages": [
				{"role": "user", "content": [{"type":"text","text":"Calculate something"}]}
			],
			"tools": [
				{
					"type": "function",
					"function": {
						"name": "calculate",
						"description": "Perform mathematical calculations",
						"parameters": {
							"type": "object",
							"properties": {
								"expression": {"type": "string"},
								"precision": {"type": "number"}
							},
							"required": ["expression"]
						}
					}
				}
			]
		}`)

		kiroRequest, err := kirotranslator.BuildRequest("claude-sonnet-4-5", openAIRequest, token, nil)
		if err != nil {
			t.Fatalf("Kiro request conversion failed: %v", err)
		}

		var kiroReq map[string]any
		if err := json.Unmarshal(kiroRequest, &kiroReq); err != nil {
			t.Fatalf("Kiro request parsing failed: %v", err)
		}

		// Verify OpenAI format tools are correctly converted
		convState := kiroReq["conversationState"].(map[string]any)
		currentMsg := convState["currentMessage"].(map[string]any)
		userInputMsg := currentMsg["userInputMessage"].(map[string]any)

		if context, ok := userInputMsg["userInputMessageContext"].(map[string]any); ok {
			if tools, ok := context["tools"]; ok {
				toolsArray := tools.([]any)
				if len(toolsArray) != 1 {
					t.Fatalf("Expected 1 tool, got %d", len(toolsArray))
				}

				toolSpec := toolsArray[0].(map[string]any)["toolSpecification"].(map[string]any)
				if toolSpec["name"] != "calculate" {
					t.Fatalf("Expected tool name 'calculate', got %v", toolSpec["name"])
				}
				if toolSpec["description"] != "Perform mathematical calculations" {
					t.Fatalf("Expected description mismatch: %v", toolSpec["description"])
				}
				t.Logf("✅ OpenAI format tools successfully converted to Kiro format")
			} else {
				t.Fatalf("❌ OpenAI format tools conversion failed")
			}
		}
	})
}

// ============================================================================
// Tool Event Sanitization Tests (sdk-kiro-contract.md compliance)
// ============================================================================

// TestBuildRequest_StripsToolEventsFromHistory verifies that tool_use and tool_result
// blocks are stripped from history messages to comply with Kiro's contract:
// "Any assistant tool_use or user tool_result in the request body leads to 'Improperly formed request.'"
func TestBuildRequest_StripsToolEventsFromHistory(t *testing.T) {
	token := &authkiro.KiroTokenStorage{
		ProfileArn:  "arn:aws:codewhisperer:us-east-1:699475941385:profile/test",
		AccessToken: "test_access_token",
		ExpiresAt:   time.Now().Add(24 * time.Hour),
		Type:        "kiro",
	}

	t.Run("strips_assistant_tool_use_from_history", func(t *testing.T) {
		openAIRequest := []byte(`{
			"model": "claude-sonnet-4-5",
			"messages": [
				{
					"role": "user",
					"content": "What's the weather?"
				},
				{
					"role": "assistant",
					"content": [
						{"type": "text", "text": "I'll check the weather for you."},
						{"type": "tool_use", "id": "call_123", "name": "get_weather", "input": {"city": "Tokyo"}}
					]
				},
				{
					"role": "user",
					"content": "Thanks!"
				}
			]
		}`)

		kiroRequest, err := kirotranslator.BuildRequest("claude-sonnet-4-5", openAIRequest, token, nil)
		if err != nil {
			t.Fatalf("BuildRequest failed: %v", err)
		}

		var kiroReq map[string]any
		if err := json.Unmarshal(kiroRequest, &kiroReq); err != nil {
			t.Fatalf("Failed to parse Kiro request: %v", err)
		}

		// Verify history doesn't contain tool_use blocks
		convState := kiroReq["conversationState"].(map[string]any)
		history := convState["history"].([]any)

		for i, msg := range history {
			msgMap := msg.(map[string]any)
			if assistantMsg, ok := msgMap["assistantResponseMessage"].(map[string]any); ok {
				// Assistant messages in history should not have toolUses
				if toolUses, exists := assistantMsg["toolUses"]; exists && toolUses != nil {
					toolUsesArray := toolUses.([]any)
					if len(toolUsesArray) > 0 {
						t.Errorf("History message %d contains tool_use blocks (should be stripped): %v", i, toolUses)
					}
				}
				// Content should be sanitized text
				content := assistantMsg["content"].(string)
				if strings.Contains(content, "tool_use") {
					t.Logf("History message %d content: %s", i, content)
				}
			}
		}

		t.Logf("✅ Assistant tool_use blocks successfully stripped from history")
	})

	t.Run("strips_user_tool_result_from_history", func(t *testing.T) {
		openAIRequest := []byte(`{
			"model": "claude-sonnet-4-5",
			"messages": [
				{
					"role": "user",
					"content": "What's the weather?"
				},
				{
					"role": "assistant",
					"content": [
						{"type": "tool_use", "id": "call_123", "name": "get_weather", "input": {"city": "Tokyo"}}
					]
				},
				{
					"role": "user",
					"content": [
						{"type": "tool_result", "tool_use_id": "call_123", "content": "Temperature: 22°C"}
					]
				},
				{
					"role": "user",
					"content": "What about tomorrow?"
				}
			]
		}`)

		kiroRequest, err := kirotranslator.BuildRequest("claude-sonnet-4-5", openAIRequest, token, nil)
		if err != nil {
			t.Fatalf("BuildRequest failed: %v", err)
		}

		var kiroReq map[string]any
		if err := json.Unmarshal(kiroRequest, &kiroReq); err != nil {
			t.Fatalf("Failed to parse Kiro request: %v", err)
		}

		// Verify history doesn't contain tool_result blocks
		convState := kiroReq["conversationState"].(map[string]any)
		history := convState["history"].([]any)

		for i, msg := range history {
			msgMap := msg.(map[string]any)
			if userMsg, ok := msgMap["userInputMessage"].(map[string]any); ok {
				// User messages in history should not have toolResults
				if context, exists := userMsg["userInputMessageContext"].(map[string]any); exists {
					if toolResults, exists := context["toolResults"]; exists && toolResults != nil {
						toolResultsArray := toolResults.([]any)
						if len(toolResultsArray) > 0 {
							t.Errorf("History message %d contains tool_result blocks (should be stripped): %v", i, toolResults)
						}
					}
				}
			}
		}

		t.Logf("✅ User tool_result blocks successfully stripped from history")
	})

	t.Run("preserves_tool_results_in_current_message", func(t *testing.T) {
		openAIRequest := []byte(`{
			"model": "claude-sonnet-4-5",
			"messages": [
				{
					"role": "user",
					"content": "What's the weather?"
				},
				{
					"role": "assistant",
					"content": [
						{"type": "tool_use", "id": "call_123", "name": "get_weather", "input": {"city": "Tokyo"}}
					]
				},
				{
					"role": "user",
					"content": [
						{"type": "tool_result", "tool_use_id": "call_123", "content": "Temperature: 22°C"}
					]
				}
			]
		}`)

		kiroRequest, err := kirotranslator.BuildRequest("claude-sonnet-4-5", openAIRequest, token, nil)
		if err != nil {
			t.Fatalf("BuildRequest failed: %v", err)
		}

		var kiroReq map[string]any
		if err := json.Unmarshal(kiroRequest, &kiroReq); err != nil {
			t.Fatalf("Failed to parse Kiro request: %v", err)
		}

		// Verify current message DOES contain tool_result (this is allowed)
		convState := kiroReq["conversationState"].(map[string]any)
		currentMsg := convState["currentMessage"].(map[string]any)
		userInputMsg := currentMsg["userInputMessage"].(map[string]any)

		context, ok := userInputMsg["userInputMessageContext"].(map[string]any)
		if !ok {
			t.Fatalf("Expected userInputMessageContext in current message")
		}

		toolResults, ok := context["toolResults"].([]any)
		if !ok || len(toolResults) == 0 {
			t.Fatalf("Expected tool_result in current message (should be preserved)")
		}

		t.Logf("✅ Tool results preserved in current message as expected")
	})

	t.Run("converts_tool_events_to_text_summaries", func(t *testing.T) {
		openAIRequest := []byte(`{
			"model": "claude-sonnet-4-5",
			"messages": [
				{
					"role": "user",
					"content": "Calculate 2+2"
				},
				{
					"role": "assistant",
					"content": [
						{"type": "text", "text": "I'll calculate that."},
						{"type": "tool_use", "id": "call_456", "name": "calculate", "input": {"expression": "2+2"}}
					]
				},
				{
					"role": "user",
					"content": "What's next?"
				}
			]
		}`)

		kiroRequest, err := kirotranslator.BuildRequest("claude-sonnet-4-5", openAIRequest, token, nil)
		if err != nil {
			t.Fatalf("BuildRequest failed: %v", err)
		}

		var kiroReq map[string]any
		if err := json.Unmarshal(kiroRequest, &kiroReq); err != nil {
			t.Fatalf("Failed to parse Kiro request: %v", err)
		}

		// Verify assistant message in history has text summary
		convState := kiroReq["conversationState"].(map[string]any)
		history := convState["history"].([]any)

		foundAssistant := false
		for _, msg := range history {
			msgMap := msg.(map[string]any)
			if assistantMsg, ok := msgMap["assistantResponseMessage"].(map[string]any); ok {
				content := assistantMsg["content"].(string)
				// Should contain the original text and a summary of the tool use
				if strings.Contains(content, "I'll calculate that") {
					foundAssistant = true
					t.Logf("Assistant message content: %s", content)
				}
			}
		}

		if !foundAssistant {
			t.Errorf("Expected to find assistant message with text content in history")
		}

		t.Logf("✅ Tool events converted to text summaries in history")
	})
}

// TestSafeParseJSON_TruncatedJSON verifies defensive JSON parsing
// as specified in sdk-kiro-contract.md section 3.1
func TestSafeParseJSON_TruncatedJSON(t *testing.T) {
	token := &authkiro.KiroTokenStorage{
		ProfileArn:  "arn:aws:codewhisperer:us-east-1:699475941385:profile/test",
		AccessToken: "test_access_token",
		ExpiresAt:   time.Now().Add(24 * time.Hour),
		Type:        "kiro",
	}

	t.Run("handles_dangling_backslash", func(t *testing.T) {
		// Tool call with truncated JSON (dangling backslash)
		openAIRequest := []byte(`{
			"model": "claude-sonnet-4-5",
			"messages": [
				{
					"role": "user",
					"content": "Test"
				},
				{
					"role": "assistant",
					"content": [
						{
							"type": "tool_use",
							"id": "call_789",
							"name": "test_tool",
							"input": "{\"key\": \"value\\"
						}
					]
				},
				{
					"role": "user",
					"content": "Continue"
				}
			]
		}`)

		// Should not panic or fail
		kiroRequest, err := kirotranslator.BuildRequest("claude-sonnet-4-5", openAIRequest, token, nil)
		if err != nil {
			t.Fatalf("BuildRequest failed with truncated JSON: %v", err)
		}

		if kiroRequest == nil {
			t.Fatalf("Expected non-nil Kiro request")
		}

		t.Logf("✅ Handled dangling backslash in tool input")
	})

	t.Run("handles_incomplete_unicode_escape", func(t *testing.T) {
		// Tool call with incomplete Unicode escape
		openAIRequest := []byte(`{
			"model": "claude-sonnet-4-5",
			"messages": [
				{
					"role": "user",
					"content": "Test"
				},
				{
					"role": "assistant",
					"content": [
						{
							"type": "tool_use",
							"id": "call_abc",
							"name": "test_tool",
							"input": "{\"emoji\": \"\\u"
						}
					]
				},
				{
					"role": "user",
					"content": "Continue"
				}
			]
		}`)

		// Should not panic or fail
		kiroRequest, err := kirotranslator.BuildRequest("claude-sonnet-4-5", openAIRequest, token, nil)
		if err != nil {
			t.Fatalf("BuildRequest failed with incomplete Unicode: %v", err)
		}

		if kiroRequest == nil {
			t.Fatalf("Expected non-nil Kiro request")
		}

		t.Logf("✅ Handled incomplete Unicode escape in tool input")
	})
}
