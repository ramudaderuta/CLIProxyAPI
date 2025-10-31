// Package main provides a focused test for Kiro tool processing functionality.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
)

func main() {
	fmt.Println("🔧 Testing Kiro Tool Processing Functionality")

	tests := []struct {
		name        string
		description string
		payload     map[string]interface{}
		expectTools bool
		toolCount   int
	}{
		{
			name:        "No Tools",
			description: "Test payload without tools",
			payload: map[string]interface{}{
				"model": "claude-sonnet-4-5",
				"messages": []map[string]interface{}{
					{"role": "user", "content": "Hello"},
				},
			},
			expectTools: false,
			toolCount:   0,
		},
		{
			name:        "Single Tool",
			description: "Test payload with one tool",
			payload: map[string]interface{}{
				"model": "claude-sonnet-4-5",
				"messages": []map[string]interface{}{
					{"role": "user", "content": "What's the weather?"},
				},
				"tools": []map[string]interface{}{
					{
						"type": "function",
						"function": map[string]interface{}{
							"name":        "get_weather",
							"description": "Get weather information",
							"parameters": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"location": map[string]interface{}{
										"type":        "string",
										"description": "Location to get weather for",
									},
								},
								"required": []string{"location"},
							},
						},
					},
				},
			},
			expectTools: true,
			toolCount:   1,
		},
		{
			name:        "Multiple Tools",
			description: "Test payload with multiple tools",
			payload: map[string]interface{}{
				"model": "claude-sonnet-4-5",
				"messages": []map[string]interface{}{
					{"role": "user", "content": "Calculate and get weather"},
				},
				"tools": []map[string]interface{}{
					{
						"type": "function",
						"function": map[string]interface{}{
							"name":        "calculate",
							"description": "Perform calculations",
							"parameters": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"expression": map[string]interface{}{
										"type":        "string",
										"description": "Mathematical expression",
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
							"description": "Get weather information",
							"parameters": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"location": map[string]interface{}{
										"type":        "string",
										"description": "Location to get weather for",
									},
								},
								"required": []string{"location"},
							},
						},
					},
				},
			},
			expectTools: true,
			toolCount:   2,
		},
	}

	// Create mock token storage
	mockToken := &kiro.KiroTokenStorage{
		AccessToken: "mock-token",
		ProfileArn:  "arn:aws:codewhisperer:us-east-1:123456789012:profile/Test",
		AuthMethod:  "social",
		Provider:    "kiro",
	}

	passed := 0
	failed := 0

	for i, test := range tests {
		fmt.Printf("\n📋 Test %d: %s\n", i+1, test.name)
		fmt.Printf("   Description: %s\n", test.description)

		success := testToolProcessing(test.payload, mockToken, test.expectTools, test.toolCount)
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
		fmt.Printf("\n🎉 All tool processing tests passed!\n")
		os.Exit(0)
	} else {
		fmt.Printf("\n💥 %d tests failed. Tool processing needs attention.\n", failed)
		os.Exit(1)
	}
}

func testToolProcessing(payload map[string]interface{}, token *kiro.KiroTokenStorage, expectTools bool, expectedToolCount int) bool {
	// Marshal the payload to JSON
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		fmt.Printf("   Error marshaling payload: %v\n", err)
		return false
	}

	fmt.Printf("   📝 Input payload has %d tools\n", getToolCount(payload))

	// Call the actual buildKiroRequestPayload function from the executor
	result, err := executor.BuildKiroRequestPayload("claude-sonnet-4-5", payloadBytes, token, nil)
	if err != nil {
		fmt.Printf("   Error building Kiro request: %v\n", err)
		return false
	}

	// Parse the result to check tool processing
	var kiroRequest map[string]interface{}
	if err := json.Unmarshal(result, &kiroRequest); err != nil {
		fmt.Printf("   Error parsing Kiro request: %v\n", err)
		return false
	}

	// Navigate through the Kiro request structure
	convState, ok := kiroRequest["conversationState"].(map[string]interface{})
	if !ok {
		fmt.Printf("   Missing conversationState in Kiro request\n")
		return false
	}

	currentMessage, ok := convState["currentMessage"].(map[string]interface{})
	if !ok {
		fmt.Printf("   Missing currentMessage in conversationState\n")
		return false
	}

	userInputMessage, ok := currentMessage["userInputMessage"].(map[string]interface{})
	if !ok {
		fmt.Printf("   Missing userInputMessage in currentMessage\n")
		return false
	}

	// Check for tools in userInputMessageContext
	context, hasContext := userInputMessage["userInputMessageContext"].(map[string]interface{})
	var tools []interface{}
	var hasTools bool

	if hasContext {
		toolsInterface, toolsFound := context["tools"]
		if toolsFound {
			tools, hasTools = toolsInterface.([]interface{})
			if !hasTools {
				fmt.Printf("   Tools field exists but is not an array: %T\n", toolsInterface)
				return false
			}
		}
	}

	// Validate tool processing
	if expectTools {
		if !hasContext {
			fmt.Printf("   Expected tools but no userInputMessageContext found\n")
			return false
		}
		if !hasTools {
			fmt.Printf("   Expected tools but none found in context\n")
			return false
		}
		if len(tools) != expectedToolCount {
			fmt.Printf("   Expected %d tools but found %d\n", expectedToolCount, len(tools))
			return false
		}

		fmt.Printf("   ✅ Successfully processed %d tools\n", len(tools))

		// Validate each tool structure
		for i, tool := range tools {
			toolMap, ok := tool.(map[string]interface{})
			if !ok {
				fmt.Printf("   Tool %d is not an object: %T\n", i, tool)
				return false
			}

			toolSpec, ok := toolMap["toolSpecification"].(map[string]interface{})
			if !ok {
				fmt.Printf("   Tool %d missing toolSpecification\n", i)
				return false
			}

			name, ok := toolSpec["name"].(string)
			if !ok || name == "" {
				fmt.Printf("   Tool %d missing or invalid name\n", i)
				return false
			}

			description, _ := toolSpec["description"].(string)
			inputSchema, hasInputSchema := toolSpec["inputSchema"].(map[string]interface{})
			if !hasInputSchema {
				fmt.Printf("   Tool %d missing inputSchema\n", i)
				return false
			}

			fmt.Printf("   Tool %d: %s - %s\n", i+1, name, description)
		}
	} else {
		if hasContext && hasTools {
			fmt.Printf("   Unexpected tools found in context: %d\n", len(tools))
			return false
		}
		fmt.Printf("   ✅ Correctly handled payload without tools\n")
	}

	return true
}

func getToolCount(payload map[string]interface{}) int {
	tools, hasTools := payload["tools"]
	if !hasTools || tools == nil {
		return 0
	}
	toolsArray, ok := tools.([]interface{})
	if !ok {
		return 0
	}
	return len(toolsArray)
}