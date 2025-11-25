package claude

import (
	"encoding/json"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro/helpers"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// ConvertClaudeRequestToKiro converts a Claude Messages API request to Kiro conversationState format.
// Claude Messages API is similar to OpenAI but with some differences in structure.
func ConvertClaudeRequestToKiro(model string, claudeRequest []byte, streaming bool) []byte {
	// Parse Claude request
	system := gjson.GetBytes(claudeRequest, "system").String()
	messagesJSON := gjson.GetBytes(claudeRequest, "messages").Raw
	maxTokens := gjson.GetBytes(claudeRequest, "max_tokens").Int()
	temperature := gjson.GetBytes(claudeRequest, "temperature").Float()

	// Parse tools if present
	toolsJSON := gjson.GetBytes(claudeRequest, "tools").Raw

	// Build Kiro conversation state
	kiroRequest := map[string]interface{}{
		"conversationState": map[string]interface{}{
			"currentMessage": map[string]interface{}{
				"userInputMessage": map[string]interface{}{},
			},
			"chatTriggerType": "MANUAL",
		},
	}

	// Add system prompt if present
	if system != "" {
		systemPrompts := []map[string]interface{}{
			{
				"text": map[string]string{
					"text": system,
				},
			},
		}
		kiroState := kiroRequest["conversationState"].(map[string]interface{})
		kiroState["customizationArn"] = ""
		kiroState["customSystemPrompts"] = systemPrompts
	}

	// Process messages
	var messages []interface{}
	if err := json.Unmarshal([]byte(messagesJSON), &messages); err == nil {
		var history []map[string]interface{}
		var currentUserInputMessage map[string]interface{}

		for i, msg := range messages {
			msgMap := msg.(map[string]interface{})
			role := helpers.SafeGetString(msgMap, "role")

			// Extract content (can be string or array in Claude)
			var contentText string
			if content, ok := msgMap["content"].(string); ok {
				contentText = content
			} else if contentArr, ok := msgMap["content"].([]interface{}); ok {
				// Handle array of content blocks
				for _, block := range contentArr {
					if blockMap, ok := block.(map[string]interface{}); ok {
						if blockMap["type"] == "text" {
							if text, ok := blockMap["text"].(string); ok {
								contentText += text
							}
						}
						// Handle image_url for multimodal
						if blockMap["type"] == "image" {
							if source, ok := blockMap["source"].(map[string]interface{}); ok {
								if imageType, ok := source["type"].(string); ok && imageType == "base64" {
									// Kiro supports base64 images
									contentText += "\n[Image content]"
								}
							}
						}
					}
				}
			}

			// Last user message becomes currentMessage, rest go to history
			isLastUserMessage := (i == len(messages)-1 && role == "user")

			if isLastUserMessage {
				currentUserInputMessage = map[string]interface{}{
					"userInputMessage": map[string]interface{}{
						"content":                 contentText,
						"userInputMessageContext": map[string]interface{}{},
					},
				}
			} else {
				// Add to history
				var utteranceType string
				if role == "user" {
					utteranceType = "HUMAN"
				} else {
					utteranceType = "AI"
				}

				historyItem := map[string]interface{}{
					"utteranceType": utteranceType,
					"message":       contentText,
				}
				history = append(history, historyItem)
			}
		}

		// Set current message
		if currentUserInputMessage != nil {
			kiroState := kiroRequest["conversationState"].(map[string]interface{})
			kiroState["currentMessage"] = currentUserInputMessage
		}

		// Set history
		if len(history) > 0 {
			kiroState := kiroRequest["conversationState"].(map[string]interface{})
			kiroState["history"] = history
		}
	}

	// Process tools (convert Claude tool format to Kiro)
	if toolsJSON != "" && toolsJSON != "null" {
		var tools []interface{}
		if err := json.Unmarshal([]byte(toolsJSON), &tools); err == nil {
			var kiroTools []map[string]interface{}

			for _, tool := range tools {
				toolMap := tool.(map[string]interface{})
				if toolMap["type"] == "function" {
					if funcMap, ok := toolMap["function"].(map[string]interface{}); ok {
						name := helpers.SafeGetString(funcMap, "name")
						description := helpers.SafeGetString(funcMap, "description")

						// Truncate description  and add hash
						truncDesc := helpers.TruncateString(description, 500, "...")

						kiroTool := map[string]interface{}{
							"name":        name,
							"description": truncDesc,
						}

						// Add parameters if present
						if params, ok := funcMap["parameters"]; ok {
							kiroTool["parameters"] = params
						}

						kiroTools = append(kiroTools, kiroTool)
					}
				}
			}

			if len(kiroTools) > 0 {
				kiroState := kiroRequest["conversationState"].(map[string]interface{})
				kiroState["tools"] = kiroTools
			}
		}
	}

	// Add generation config
	if maxTokens > 0 || temperature > 0 {
		generationConfig := map[string]interface{}{}
		if maxTokens > 0 {
			generationConfig["maxTokens"] = maxTokens
		}
		if temperature > 0 {
			generationConfig["temperature"] = temperature
		}
		kiroState := kiroRequest["conversationState"].(map[string]interface{})
		kiroState["generationConfig"] = generationConfig
	}

	// Convert to JSON
	result, err := json.Marshal(kiroRequest)
	if err != nil {
		log.Errorf("Failed to marshal Kiro request: %v", err)
		return claudeRequest // Return original on error
	}

	// Set streaming flag if needed
	if streaming {
		result, _ = sjson.SetBytes(result, "stream", true)
	}

	return result
}
