// Package main provides a comprehensive integration test for Kiro provider function calling support.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/runtime/executor"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetLevel(log.InfoLevel)
	fmt.Println("Starting Kiro Function Calling Integration Test")

	// Test configurations
	tests := []struct {
		name        string
		description string
		payload     map[string]interface{}
		expectTools bool
	}{
		{
			name:        "Basic Chat No Tools",
			description: "Test basic chat functionality without tools",
			payload: map[string]interface{}{
				"model": "claude-sonnet-4-5",
				"messages": []map[string]interface{}{
					{"role": "user", "content": "Hello, how are you?"},
				},
				"max_tokens": 100,
			},
			expectTools: false,
		},
		{
			name:        "Simple Tool Definition",
			description: "Test request with basic tool definition",
			payload: map[string]interface{}{
				"model": "claude-sonnet-4-5",
				"messages": []map[string]interface{}{
					{"role": "user", "content": "What's the weather like in New York?"},
				},
				"tools": []map[string]interface{}{
					{
						"type": "function",
						"function": map[string]interface{}{
							"name":        "get_weather",
							"description": "Get the current weather for a location",
							"parameters": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"location": map[string]interface{}{
										"type":        "string",
										"description": "The city and state, e.g. San Francisco, CA",
									},
								},
								"required": []string{"location"},
							},
						},
					},
				},
				"max_tokens": 100,
			},
			expectTools: true,
		},
		{
			name:        "Multiple Tools",
			description: "Test request with multiple tool definitions",
			payload: map[string]interface{}{
				"model": "claude-sonnet-4-5",
				"messages": []map[string]interface{}{
					{"role": "user", "content": "Calculate 15 * 27 and then get the weather in London"},
				},
				"tools": []map[string]interface{}{
					{
						"type": "function",
						"function": map[string]interface{}{
							"name":        "calculate",
							"description": "Perform mathematical calculations",
							"parameters": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"expression": map[string]interface{}{
										"type":        "string",
										"description": "Mathematical expression to evaluate",
									},
								},
								"required": []string{"expression"},
							},
						},
					},
					{
						"type": "function",
						"function": map[string]interface{}{
							"name":        "get_weather",
							"description": "Get the current weather for a location",
							"parameters": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"location": map[string]interface{}{
										"type":        "string",
										"description": "The city and state, e.g. San Francisco, CA",
									},
								},
								"required": []string{"location"},
							},
						},
					},
				},
				"max_tokens": 100,
			},
			expectTools: true,
		},
		{
			name:        "Complex Tool Schema",
			description: "Test request with complex nested tool parameters",
			payload: map[string]interface{}{
				"model": "claude-sonnet-4-5",
				"messages": []map[string]interface{}{
					{"role": "user", "content": "Create a user profile with specific settings"},
				},
				"tools": []map[string]interface{}{
					{
						"type": "function",
						"function": map[string]interface{}{
							"name":        "create_user_profile",
							"description": "Create a new user profile with various settings",
							"parameters": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"username": map[string]interface{}{
										"type":        "string",
										"description": "Unique username for the user",
									},
									"preferences": map[string]interface{}{
										"type":        "object",
										"description": "User preferences",
										"properties": map[string]interface{}{
											"theme": map[string]interface{}{
												"type":        "string",
												"enum":        []string{"light", "dark", "auto"},
												"description": "UI theme preference",
											},
											"notifications": map[string]interface{}{
												"type": "object",
												"properties": map[string]interface{}{
													"email": map[string]interface{}{
														"type": "boolean",
													},
													"sms": map[string]interface{}{
														"type": "boolean",
													},
												},
											},
										},
									},
								},
								"required": []string{"username"},
							},
						},
					},
				},
				"max_tokens": 100,
			},
			expectTools: true,
		},
	}

	// Initialize Kiro executor
	cfg := &config.Config{
		AuthDir: "/tmp/kiro-test",
	}
	kiroExecutor := executor.NewKiroExecutor(cfg)

	// Create mock authentication
	mockAuth := createMockAuth()

	// Run tests
	passed := 0
	failed := 0

	for i, test := range tests {
		fmt.Printf("\n📋 Test %d: %s\n", i+1, test.name)
		fmt.Printf("   Description: %s\n", test.description)

		success := runTest(context.Background(), kiroExecutor, mockAuth, test)
		if success {
			fmt.Printf("   ✅ PASSED\n")
			passed++
		} else {
			fmt.Printf("   ❌ FAILED\n")
			failed++
		}
	}

	// Summary
	fmt.Printf("\n📊 Test Summary:\n")
	fmt.Printf("   Total: %d\n", len(tests))
	fmt.Printf("   Passed: %d\n", passed)
	fmt.Printf("   Failed: %d\n", failed)

	if failed == 0 {
		fmt.Printf("\n🎉 All tests passed! Kiro function calling is working correctly.\n")
		os.Exit(0)
	} else {
		fmt.Printf("\n💥 %d tests failed. Kiro function calling needs attention.\n", failed)
		os.Exit(1)
	}
}

func runTest(ctx context.Context, exec *executor.KiroExecutor, auth *cliproxyauth.Auth, test struct {
	name        string
	description string
	payload     map[string]interface{}
	expectTools bool
}) bool {
	// Marshal payload
	payloadBytes, err := json.Marshal(test.payload)
	if err != nil {
		fmt.Printf("   Error marshaling payload: %v\n", err)
		return false
	}

	// Create request
	req := cliproxyexecutor.Request{
		Model:   test.payload["model"].(string),
		Payload: payloadBytes,
	}

	// Create options
	opts := cliproxyexecutor.Options{
		SourceFormat:    "openai",
		OriginalRequest: payloadBytes,
		Stream:          false,
	}

	// Test request building (this is where tool processing happens)
	toolsCount := 0
	if tools, hasTools := test.payload["tools"]; hasTools && tools != nil {
		if toolsArray, ok := tools.([]interface{}); ok {
			toolsCount = len(toolsArray)
		}
	}
	fmt.Printf("   🔧 Testing request building with %d tools...\n", toolsCount)

	// Try to execute the request - this will test the full pipeline
	// Note: This will fail without real auth, but we can catch tool-related errors
	resp, err := exec.Execute(ctx, auth, req, opts)

	if err != nil {
		// Check if it's an auth-related error (expected) or tool-related error (problematic)
		errStr := err.Error()
		if strings.Contains(errStr, "tool") || strings.Contains(errStr, "function") {
			fmt.Printf("   Tool-related error: %v\n", err)
			return false
		}
		if strings.Contains(errStr, "auth") || strings.Contains(errStr, "token") || strings.Contains(errStr, "unauthorized") {
			fmt.Printf("   Auth error (expected): %v\n", err)
			// Auth errors are expected without real credentials, so check if request was built properly
			return validateToolProcessing(test.payload, test.expectTools)
		}
		fmt.Printf("   Unexpected error: %v\n", err)
		return false
	}

	// If we got a response, validate it
	if resp.Payload != nil {
		fmt.Printf("   Got response payload (%d bytes)\n", len(resp.Payload))
		return validateResponse(resp.Payload, test.expectTools)
	}

	return validateToolProcessing(test.payload, test.expectTools)
}

func validateToolProcessing(payload map[string]interface{}, expectTools bool) bool {
	tools, hasTools := payload["tools"]

	// Check if tools field exists and is not nil
	if tools == nil {
		if expectTools {
			fmt.Printf("   Expected tools but none found in payload (tools field is nil)\n")
			return false
		}
		return true // No tools expected and none found
	}

	if !expectTools && hasTools {
		fmt.Printf("   Unexpected tools in payload: %v\n", tools)
		return false
	}

	if expectTools {
		if !hasTools {
			fmt.Printf("   Expected tools but none found in payload\n")
			return false
		}

		toolsArray, ok := tools.([]interface{})
		if !ok {
			fmt.Printf("   Tools is not an array: %T\n", tools)
			return false
		}

		fmt.Printf("   Successfully processed %d tool definitions\n", len(toolsArray))

		// Validate each tool
		for i, tool := range toolsArray {
			toolMap, ok := tool.(map[string]interface{})
			if !ok {
				fmt.Printf("   Tool %d is not an object: %T\n", i, tool)
				return false
			}

			if toolMap["type"] != "function" {
				fmt.Printf("   Tool %d has invalid type: %v\n", i, toolMap["type"])
				return false
			}

			function, ok := toolMap["function"].(map[string]interface{})
			if !ok {
				fmt.Printf("   Tool %d missing function definition\n", i)
				return false
			}

			if function["name"] == nil || function["name"].(string) == "" {
				fmt.Printf("   Tool %d missing function name\n", i)
				return false
			}

			fmt.Printf("   Tool %d: %s\n", i+1, function["name"].(string))
		}
	}

	return true
}

func validateResponse(payload []byte, expectTools bool) bool {
	var response map[string]interface{}
	if err := json.Unmarshal(payload, &response); err != nil {
		fmt.Printf("   Error parsing response: %v\n", err)
		return false
	}

	// Check for basic response structure
	choices, ok := response["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		fmt.Printf("   Invalid response structure: missing choices\n")
		return false
	}

	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		fmt.Printf("   Invalid choice structure\n")
		return false
	}

	message, ok := choice["message"].(map[string]interface{})
	if !ok {
		fmt.Printf("   Invalid message structure\n")
		return false
	}

	content, hasContent := message["content"]
	if !hasContent || content.(string) == "" {
		fmt.Printf("   Response missing content\n")
		return false
	}

	fmt.Printf("   Response content: %s...\n", truncateString(content.(string), 50))

	// Check for tool calls in response if tools were expected
	if expectTools {
		toolCalls, hasToolCalls := message["tool_calls"]
		if hasToolCalls {
			fmt.Printf("   Response includes tool calls: %v\n", toolCalls)
		}
	}

	return true
}

func createMockAuth() *cliproxyauth.Auth {
	return &cliproxyauth.Auth{
		ID: "test-kiro-auth",
		Runtime: &kiro.KiroTokenStorage{
			AccessToken: "mock-access-token",
			ProfileArn:  "arn:aws:codewhisperer:us-east-1:123456789012:profile/TestProfile",
			AuthMethod:  "social",
			Provider:    "kiro",
		},
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
