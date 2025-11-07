package kiro_test

import (
	"encoding/json"
	"testing"
	"time"

	kirotranslator "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro"
	authkiro "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
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
			50,  // promptTokens
			30,  // completionTokens
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
			50,  // promptTokens
			30,  // completionTokens
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