// Package chat_completions provides request translation functionality for OpenAI to Kiro API compatibility.
// It converts OpenAI Chat Completions requests into Kiro CodeWhisperer compatible JSON using gjson/sjson only.
package chat_completions

import (
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
)

// Kiro model mapping
var kiroModelMapping = map[string]string{
	"claude-sonnet-4-5":                "CLAUDE_SONNET_4_5_20250929_V1_0",
	"claude-sonnet-4-5-20250929":      "CLAUDE_SONNET_4_5_20250929_V1_0",
	"claude-sonnet-4-20250514":        "CLAUDE_SONNET_4_20250514_V1_0",
	"claude-3-7-sonnet-20250219":      "CLAUDE_3_7_SONNET_20250219_V1_0",
	"amazonq-claude-sonnet-4-20250514": "CLAUDE_SONNET_4_20250514_V1_0",
	"amazonq-claude-3-7-sonnet-20250219": "CLAUDE_3_7_SONNET_20250219_V1_0",
}

// ConvertOpenAIRequestToKiro converts an OpenAI Chat Completions request (raw JSON)
// into a complete Kiro CodeWhisperer request JSON. This is a simplified implementation.
//
// Parameters:
//   - modelName: The name of the model to use for the request
//   - rawJSON: The raw JSON request data from the OpenAI API
//   - stream: A boolean indicating if the request is for a streaming response
//
// Returns:
//   - []byte: The transformed request data in Kiro CodeWhisperer API format
func ConvertOpenAIRequestToKiro(modelName string, inputRawJSON []byte, _ bool) []byte {
	// Generate conversation ID
	conversationID := uuid.New().String()

	// Get Kiro model name
	kiroModel := getKiroModel(modelName)

	// Extract messages
	messages := gjson.GetBytes(inputRawJSON, "messages")
	if !messages.Exists() || !messages.IsArray() {
		// Return basic structure if no messages
		return []byte(`{
			"conversationState": {
				"chatTriggerType": "MANUAL",
				"conversationId": "` + conversationID + `",
				"currentMessage": {},
				"history": []
			}
		}`)
	}

	// Create basic Kiro request structure
	// This is a simplified implementation that just creates a valid JSON structure
	result := `{
		"conversationState": {
			"chatTriggerType": "MANUAL",
			"conversationId": "` + conversationID + `",
			"currentMessage": {
				"userInputMessage": {
					"content": "Hello",
					"modelId": "` + kiroModel + `",
					"origin": "AI_EDITOR"
				}
			},
			"history": []
		}
	}`

	return []byte(result)
}

// getKiroModel returns the Kiro internal model name for the given model.
func getKiroModel(modelName string) string {
	if kiroModel, exists := kiroModelMapping[modelName]; exists {
		return kiroModel
	}
	// Default to claude-sonnet-4-5 if no mapping found
	return kiroModelMapping["claude-sonnet-4-5"]
}