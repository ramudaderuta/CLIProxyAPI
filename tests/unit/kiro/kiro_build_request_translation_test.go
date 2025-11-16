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
	found := make(map[string]bool)
	for _, tool := range toolsAny {
		spec := tool.(map[string]any)["toolSpecification"].(map[string]any)
		name := spec["name"].(string)
		found[strings.ToLower(name)] = true
	}

	expected := []string{"task", "bash", "glob", "grep", "todowrite", "skill", "slashcommand", "write", "get_weather"}
	for _, builtin := range expected {
		if !found[builtin] {
			t.Fatalf("expected builtin tool %s to be forwarded; saw %v", builtin, found)
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
	if manifest := ctx["toolContextManifest"]; manifest != nil {
		if !strings.Contains(sysContent, "Tool reference manifest") {
			t.Fatalf("expected tool reference manifest in system prompt when manifest exists:\n%s", sysContent)
		}
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
			if !strings.Contains(desc, "Launch background plan executors") {
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

func TestBuildRequestHydratesCurrentUserForAssistantToolUseFollowedByToolResult(t *testing.T) {
	token := &authkiro.KiroTokenStorage{AccessToken: "token"}
	payload := testutil.LoadTestData(t, "streaming/orignal.json")

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
	content := strings.TrimSpace(current["content"].(string))
	if content != "." {
		t.Fatalf("expected placeholder '.' for current user content, got %q", content)
	}
	ctx, ok := current["userInputMessageContext"].(map[string]any)
	if ok {
		if _, exists := ctx["toolResults"]; exists {
			t.Fatalf("did not expect toolResults attached to synthetic current turn: %#v", ctx)
		}
	}
	if _, exists := current["toolUses"]; exists {
		t.Fatalf("did not expect tool_uses attached to synthetic current turn: %#v", current)
	}

	history := conv["history"].([]any)
	if len(history) < 2 {
		t.Fatalf("expected assistant tool_use and user tool_result in history: %#v", history)
	}

	last := history[len(history)-1].(map[string]any)
	userEntry, ok := last["userInputMessage"].(map[string]any)
	if !ok {
		t.Fatalf("expected final history entry to carry the tool_result user message: %#v", last)
	}
	historyCtx, ok := userEntry["userInputMessageContext"].(map[string]any)
	if !ok {
		t.Fatalf("expected tool_result context in history entry: %#v", userEntry)
	}
	results, ok := historyCtx["toolResults"].([]any)
	if !ok || len(results) == 0 {
		t.Fatalf("expected preserved tool_result in history: %#v", historyCtx)
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

func TestBuildRequest_PreservesToolEventsInHistory(t *testing.T) {
	token := &authkiro.KiroTokenStorage{
		ProfileArn:  "arn:aws:codewhisperer:us-east-1:699475941385:profile/test",
		AccessToken: "test_access_token",
		ExpiresAt:   time.Now().Add(24 * time.Hour),
		Type:        "kiro",
	}

	t.Run("preserves_assistant_tool_use_in_history", func(t *testing.T) {
		openAIRequest := []byte(`{
            "model": "claude-sonnet-4-5",
            "messages": [
                {"role": "user", "content": "What's the weather?"},
                {"role": "assistant", "content": [
                    {"type": "text", "text": "I'll check the weather for you."},
                    {"type": "tool_use", "id": "call_123", "name": "get_weather", "input": {"city": "Tokyo"}}
                ]},
                {"role": "user", "content": "Thanks!"}
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

		convState := kiroReq["conversationState"].(map[string]any)
		history := convState["history"].([]any)

		hasToolUses := false
		for _, msg := range history {
			msgMap := msg.(map[string]any)
			if assistantMsg, ok := msgMap["assistantResponseMessage"].(map[string]any); ok {
				if _, exists := assistantMsg["toolUses"]; exists {
					hasToolUses = true
				}
			}
		}
		if !hasToolUses {
			t.Fatalf("expected assistant toolUses preserved in history")
		}
	})

	t.Run("preserves_user_tool_result_in_history", func(t *testing.T) {
		openAIRequest := []byte(`{
            "model": "claude-sonnet-4-5",
            "messages": [
                {"role": "user", "content": "What's the weather?"},
                {"role": "assistant", "content": [
                    {"type": "tool_use", "id": "call_123", "name": "get_weather", "input": {"city": "Tokyo"}}
                ]},
                {"role": "user", "content": [
                    {"type": "tool_result", "tool_use_id": "call_123", "content": "Temperature: 22°C"}
                ]},
                {"role": "user", "content": "What about tomorrow?"}
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

		convState := kiroReq["conversationState"].(map[string]any)
		history := convState["history"].([]any)

		found := false
		for _, msg := range history {
			msgMap := msg.(map[string]any)
			if userMsg, ok := msgMap["userInputMessage"].(map[string]any); ok {
				if context, exists := userMsg["userInputMessageContext"].(map[string]any); exists {
					if toolResults, exists := context["toolResults"]; exists && toolResults != nil {
						if len(toolResults.([]any)) > 0 {
							found = true
						}
					}
				}
			}
		}
		if !found {
			t.Fatalf("expected tool_result blocks preserved in history")
		}
	})

	t.Run("assistant_tool_use_kept_structured_in_history", func(t *testing.T) {
		openAIRequest := []byte(`{
            "model": "claude-sonnet-4-5",
            "messages": [
                {"role": "user", "content": "Calculate 2+2"},
                {"role": "assistant", "content": [
                    {"type": "tool_use", "id": "call_456", "name": "calculate", "input": {"expression": "2+2"}}
                ]},
                {"role": "user", "content": "What's next?"}
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

		convState := kiroReq["conversationState"].(map[string]any)
		history := convState["history"].([]any)

		found := false
		placeholderOK := false
		for _, msg := range history {
			msgMap := msg.(map[string]any)
			if assistantMsg, ok := msgMap["assistantResponseMessage"].(map[string]any); ok {
				if _, ok := assistantMsg["toolUses"]; ok {
					found = true
					if content, _ := assistantMsg["content"].(string); strings.TrimSpace(content) != "" {
						placeholderOK = true
					}
				}
			}
		}
		if !found {
			t.Errorf("Expected structured toolUses on assistant history message")
		}
		if !placeholderOK {
			t.Errorf("Expected non-empty assistant content (placeholder) when only tool_use is present")
		}
	})
}

func TestBuildRequest_EnsuresNonEmptyFinalUserContent(t *testing.T) {
	token := &authkiro.KiroTokenStorage{AccessToken: "token"}
	cases := []struct {
		name                       string
		fixture                    string
		expectToolResultsInCurrent bool
		expectToolResultsInHistory bool
		expectedContent            string
	}{
		{"tool_result_last", "nonstream/claude_request_todowrite_bad.json", false, true, "."},
		{"whitespace_last", "nonstream/claude_request_todowrite_bad2.json", false, false, "."},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := testutil.LoadTestData(t, tc.fixture)

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
			user := current["userInputMessage"].(map[string]any)

			content := strings.TrimSpace(user["content"].(string))
			if content == "" {
				t.Fatalf("expected non-empty current user content for final turn")
			}
			if tc.expectedContent != "" && content != tc.expectedContent {
				t.Fatalf("expected current user content to be %q, got %q", tc.expectedContent, content)
			}

			ctx, ok := user["userInputMessageContext"].(map[string]any)
			if tc.expectToolResultsInCurrent {
				if !ok {
					t.Fatalf("expected userInputMessageContext with toolResults")
				}
				if tr, ok := ctx["toolResults"].([]any); !ok || len(tr) == 0 {
					t.Fatalf("expected structured toolResults in current message context: %+v", ctx)
				}
			} else if ok {
				if _, exists := ctx["toolResults"]; exists {
					t.Fatalf("did not expect toolResults in current message context: %+v", ctx)
				}
			}

			if tc.expectToolResultsInHistory {
				history := conv["history"].([]any)
				found := false
				for _, msg := range history {
					msgMap := msg.(map[string]any)
					userMsg, ok := msgMap["userInputMessage"].(map[string]any)
					if !ok {
						continue
					}
					if ctx, ok := userMsg["userInputMessageContext"].(map[string]any); ok {
						if tr, ok := ctx["toolResults"].([]any); ok && len(tr) > 0 {
							found = true
							break
						}
					}
				}
				if !found {
					t.Fatalf("expected toolResults preserved in history for fixture %s", tc.fixture)
				}
			}
		})
	}
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

		t.Logf("Handled dangling backslash in tool input")
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

		t.Logf("Handled incomplete Unicode escape in tool input")
	})
}
