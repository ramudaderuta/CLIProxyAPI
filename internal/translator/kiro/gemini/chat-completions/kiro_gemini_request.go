package chatcompletions

import (
	"encoding/json"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro/helpers"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// ConvertGeminiRequestToKiro converts a Gemini generateContent request to Kiro conversationState format.
func ConvertGeminiRequestToKiro(model string, geminiRequest []byte, streaming bool) []byte {
	// Parse Gemini request
	contentsJSON := gjson.GetBytes(geminiRequest, "contents").Raw
	systemInstructionJSON := gjson.GetBytes(geminiRequest, "systemInstruction").Raw
	toolsJSON := gjson.GetBytes(geminiRequest, "tools").Raw

	// Parse generation config
	maxOutputTokens := gjson.GetBytes(geminiRequest, "generationConfig.maxOutputTokens").Int()
	temperature := gjson.GetBytes(geminiRequest, "generationConfig.temperature").Float()

	// Build Kiro conversation state
	kiroRequest := map[string]interface{}{
		"conversationState": map[string]interface{}{
			"currentMessage": map[string]interface{}{
				"userInputMessage": map[string]interface{}{},
			},
			"chatTriggerType": "MANUAL",
		},
	}

	// Add system instruction if present
	if systemInstructionJSON != "" && systemInstructionJSON != "null" {
		systemText := gjson.Get(systemInstructionJSON, "parts.0.text").String()
		if systemText != "" {
			systemPrompts := []map[string]interface{}{
				{
					"text": map[string]string{
						"text": systemText,
					},
				},
			}
			kiroState := kiroRequest["conversationState"].(map[string]interface{})
			kiroState["customizationArn"] = ""
			kiroState["customSystemPrompts"] = systemPrompts
		}
	}

	// Process contents (Gemini's messages)
	var contents []interface{}
	if err := json.Unmarshal([]byte(contentsJSON), &contents); err == nil {
		var history []map[string]interface{}
		var currentUserInputMessage map[string]interface{}

		for i, content := range contents {
			contentMap := content.(map[string]interface{})
			role := helpers.SafeGetString(contentMap, "role")

			// Extract text from parts
			var contentText string
			if parts, ok := contentMap["parts"].([]interface{}); ok {
				for _, part := range parts {
					if partMap, ok := part.(map[string]interface{}); ok {
						if text, ok := partMap["text"].(string); ok {
							contentText += text
						}
						// Handle inline_data for images
						if inlineData, ok := partMap["inline_data"].(map[string]interface{}); ok {
							contentText += "\n[Image content]"
							_ = inlineData // For multimodal support
						}
					}
				}
			}

			// Determine if this is the last user message
			isLastUserMessage := (i == len(contents)-1 && role == "user")

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

	// Process tools (convert Gemini function declarations to Kiro)
	if toolsJSON != "" && toolsJSON != "null" {
		var toolsArray []interface{}
		if err := json.Unmarshal([]byte(toolsJSON), &toolsArray); err == nil {
			var kiroTools []map[string]interface{}

			for _, toolItem := range toolsArray {
				toolMap := toolItem.(map[string]interface{})
				if functionDeclarations, ok := toolMap["function_declarations"].([]interface{}); ok {
					for _, funcDecl := range functionDeclarations {
						funcMap := funcDecl.(map[string]interface{})
						name := helpers.SafeGetString(funcMap, "name")
						description := helpers.SafeGetString(funcMap, "description")

						// Truncate description
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
	if maxOutputTokens > 0 || temperature > 0 {
		generationConfig := map[string]interface{}{}
		if maxOutputTokens > 0 {
			generationConfig["maxTokens"] = maxOutputTokens
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
		return geminiRequest // Return original on error
	}

	// Set streaming flag if needed
	if streaming {
		result, _ = sjson.SetBytes(result, "stream", true)
	}

	return result
}
