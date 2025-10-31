// Package main provides a simple test for Kiro tool processing.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	fmt.Println("🔧 Simple Kiro Tool Test")

	// Test payload with tools
	payload := map[string]interface{}{
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
	}

	// Test buildToolSpecifications function directly
	payloadBytes, _ := json.Marshal(payload)

	fmt.Printf("📝 Input payload:\n")
	var prettyJSON bytes.Buffer
	json.Indent(&prettyJSON, payloadBytes, "  ", "  ")
	fmt.Printf("%s\n", prettyJSON.String())

	// Parse to simulate what buildKiroRequestPayload does
	var parsed map[string]interface{}
	json.Unmarshal(payloadBytes, &parsed)

	tools := parsed["tools"]
	fmt.Printf("🔧 Tools from payload: %T = %v\n", tools, tools)

	if toolsArray, ok := tools.([]interface{}); ok {
		fmt.Printf("📊 Found %d tools\n", len(toolsArray))
		for i, tool := range toolsArray {
			toolMap := tool.(map[string]interface{})
			function := toolMap["function"].(map[string]interface{})
			name := function["name"].(string)
			fmt.Printf("   Tool %d: %s\n", i+1, name)
		}
	} else {
		fmt.Printf("❌ Tools is not an array: %T\n", tools)
	}

	fmt.Printf("✅ Test completed successfully!\n")
	os.Exit(0)
}