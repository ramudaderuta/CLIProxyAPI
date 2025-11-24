// Package chat_completions provides request translation functionality for OpenAI to Kiro API compatibility.
// It converts OpenAI Chat Completions requests into Kiro's conversationState format using gjson/sjson.
package chat_completions

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	authkiro "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	// Maximum length for tool descriptions before truncation
	maxToolDescriptionLength = 500

	// Tool specification hash prefix
	toolHashPrefix = "tool_"
)

// ConvertOpenAIRequestToKiro converts an OpenAI Chat Completions request (raw JSON)
// into Kiro's conversationState format. All JSON construction uses sjson and lookups use gjson.
//
// Parameters:
//   - modelName: The name of the model to use for the request
//   - inputRawJSON: The raw JSON request data from the OpenAI API
//   - token: The Kiro token storage (unused in minimal version)
//   - metadata: Additional metadata (unused in minimal version)
//
// Returns:
//   - []byte: The transformed request data in Kiro conversationState format
//   - error: An error if the conversion fails
func ConvertOpenAIRequestToKiro(modelName string, inputRawJSON []byte, token *authkiro.KiroTokenStorage, metadata map[string]any) ([]byte, error) {
	// Parse input JSON
	inputJSON := gjson.ParseBytes(inputRawJSON)

	// Initialize conversation state
	convState := `{}`

	// Extract system prompt from messages
	systemPrompt := extractSystemPrompt(inputJSON.Get("messages"))
	if systemPrompt != "" {
		convState, _ = sjson.Set(convState, "systemPrompt", systemPrompt)
	}

	// Build tool specifications from tools array
	tools := inputJSON.Get("tools")
	if tools.Exists() && tools.IsArray() {
		toolSpecs := buildToolSpecifications(tools)
		if len(toolSpecs) > 0 {
			convState, _ = sjson.SetRaw(convState, "tools", string(toolSpecs))
		}
	}

	// Build conversation history from messages (all but the last user message)
	messages := inputJSON.Get("messages").Array()
	history, currentMessage := buildHistoryAndCurrentMessage(messages)

	if len(history) > 0 {
		historyJSON, _ := json.Marshal(history)
		convState, _ = sjson.SetRaw(convState, "history", string(historyJSON))
	}

	// Set current message
	if currentMessage != nil {
		currentJSON, _ := json.Marshal(currentMessage)
		convState, _ = sjson.SetRaw(convState, "currentMessage", string(currentJSON))
	}

	// Set model name
	convState, _ = sjson.Set(convState, "model", modelName)

	// Set streaming flag
	// Add additional parameters
	if temp := inputJSON.Get("temperature"); temp.Exists() {
		convState, _ = sjson.Set(convState, "temperature", temp.Float())
	}
	if maxTokens := inputJSON.Get("max_tokens"); maxTokens.Exists() {
		convState, _ = sjson.Set(convState, "maxTokens", maxTokens.Int())
	}
	if topP := inputJSON.Get("top_p"); topP.Exists() {
		convState, _ = sjson.Set(convState, "topP", topP.Float())
	}

	// Build the final Kiro request
	result, err := json.Marshal(map[string]interface{}{
		"conversationState": json.RawMessage(convState),
	})
	if err != nil {
		return nil, err
	}

	log.Debugf("Converted OpenAI request to Kiro format")

	return result, nil
}

// extractSystemPrompt extracts the system prompt from messages array
func extractSystemPrompt(messages gjson.Result) string {
	if !messages.Exists() || !messages.IsArray() {
		return ""
	}

	for _, msg := range messages.Array() {
		role := msg.Get("role").String()
		if role == "system" {
			content := msg.Get("content")
			if content.IsArray() {
				// Handle multimodal content
				var textParts []string
				for _, part := range content.Array() {
					if part.Get("type").String() == "text" {
						textParts = append(textParts, part.Get("text").String())
					}
				}
				return strings.Join(textParts, "\n")
			}
			return content.String()
		}
	}
	return ""
}

// buildToolSpecifications builds Kiro tool specifications from OpenAI tools array
func buildToolSpecifications(tools gjson.Result) []byte {
	if !tools.Exists() || !tools.IsArray() {
		return nil
	}

	var specs []map[string]interface{}

	for _, tool := range tools.Array() {
		toolType := tool.Get("type").String()
		if toolType != "function" {
			continue
		}

		function := tool.Get("function")
		name := function.Get("name").String()
		description := function.Get("description").String()

		// Truncate description if too long
		if len(description) > maxToolDescriptionLength {
			hash := generateToolHash(name, description)
			description = description[:maxToolDescriptionLength] + "... [truncated:" + hash + "]"
		}

		params := function.Get("parameters")

		spec := map[string]interface{}{
			"name":        name,
			"description": description,
		}

		if params.Exists() {
			// Convert parameters to Kiro format
			var paramsMap map[string]interface{}
			if err := json.Unmarshal([]byte(params.Raw), &paramsMap); err == nil {
				spec["inputSchema"] = paramsMap
			}
		}

		specs = append(specs, spec)
	}

	if len(specs) == 0 {
		return nil
	}

	result, _ := json.Marshal(specs)
	return result
}

// generateToolHash creates a deterministic hash for tool signature
func generateToolHash(name, description string) string {
	content := name + ":" + description
	hash := sha256.Sum256([]byte(content))
	return toolHashPrefix + hex.EncodeToString(hash[:])[:12]
}

// buildHistoryAndCurrentMessage separates messages into history and current message
func buildHistoryAndCurrentMessage(messages []gjson.Result) ([]map[string]interface{}, map[string]interface{}) {
	if len(messages) == 0 {
		return nil, nil
	}

	var history []map[string]interface{}
	var currentMessage map[string]interface{}

	// Find last user message index
	lastUserIdx := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Get("role").String() == "user" {
			lastUserIdx = i
			break
		}
	}

	if lastUserIdx == -1 {
		// No user message found, treat all as history
		for _, msg := range messages {
			if converted := convertMessage(msg); converted != nil {
				history = append(history, converted)
			}
		}
		return history, nil
	}

	// Build history from messages before last user message
	for i := 0; i < lastUserIdx; i++ {
		msg := messages[i]
		role := msg.Get("role").String()

		// Skip system messages in history (already extracted)
		if role == "system" {
			continue
		}

		if converted := convertMessage(msg); converted != nil {
			history = append(history, converted)
		}
	}

	// Set current message as the last user message
	currentMessage = convertMessage(messages[lastUserIdx])

	// If there are messages after the last user message, append them to history
	// This handles cases where tool results come after the user message
	for i := lastUserIdx + 1; i < len(messages); i++ {
		if converted := convertMessage(messages[i]); converted != nil {
			history = append(history, converted)
		}
	}

	return history, currentMessage
}

// convertMessage converts an OpenAI message to Kiro format
func convertMessage(msg gjson.Result) map[string]interface{} {
	role := msg.Get("role").String()
	content := msg.Get("content")

	result := map[string]interface{}{
		"role": role,
	}

	// Handle different content types
	if content.IsArray() {
		// Multimodal content
		var parts []map[string]interface{}
		for _, part := range content.Array() {
			partType := part.Get("type").String()

			switch partType {
			case "text":
				parts = append(parts, map[string]interface{}{
					"type": "text",
					"text": part.Get("text").String(),
				})
			case "image_url":
				imageURL := part.Get("image_url.url").String()
				parts = append(parts, map[string]interface{}{
					"type":     "image",
					"imageUrl": imageURL,
				})
			}
		}
		result["content"] = parts
	} else {
		// Simple text content
		result["content"] = content.String()
	}

	// Handle tool calls (for assistant messages)
	if role == "assistant" {
		toolCalls := msg.Get("tool_calls")
		if toolCalls.Exists() && toolCalls.IsArray() {
			var calls []map[string]interface{}
			for _, tc := range toolCalls.Array() {
				tcType := tc.Get("type").String()
				if tcType == "function" {
					callID := tc.Get("id").String()
					// Sanitize empty IDs
					if callID == "" {
						callID = "call_" + uuid.New().String()[:12]
					}

					calls = append(calls, map[string]interface{}{
						"id":   callID,
						"type": "function",
						"function": map[string]interface{}{
							"name":      tc.Get("function.name").String(),
							"arguments": tc.Get("function.arguments").String(),
						},
					})
				}
			}
			if len(calls) > 0 {
				result["toolCalls"] = calls
			}
		}
	}

	// Handle tool results (for tool messages)
	if role == "tool" {
		toolCallID := msg.Get("tool_call_id").String()
		if toolCallID != "" {
			result["toolCallId"] = toolCallID
			result["toolResult"] = content.String()
		}
	}

	return result
}
