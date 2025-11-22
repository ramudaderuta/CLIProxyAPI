// Package shared provides request/response builders for tests
package shared

import (
	"encoding/json"
	"testing"
)

// OpenAI Request Builders

// BuildOpenAIRequest creates a basic OpenAI chat completion request
func BuildOpenAIRequest(model string, messages []map[string]interface{}, streaming bool) map[string]interface{} {
	req := map[string]interface{}{
		"model":    model,
		"messages": messages,
	}

	if streaming {
		req["stream"] = true
	}

	return req
}

// BuildSimpleMessage creates a simple message
func BuildSimpleMessage(role, content string) map[string]interface{} {
	return map[string]interface{}{
		"role":    role,
		"content": content,
	}
}

// BuildToolMessage creates a tool/function call message
func BuildToolMessage(role string, toolCalls []map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"role":       role,
		"tool_calls": toolCalls,
	}
}

// BuildToolCall creates a tool call
func BuildToolCall(id, name string, arguments map[string]interface{}) map[string]interface{} {
	argsJSON, _ := json.Marshal(arguments)

	return map[string]interface{}{
		"id":   id,
		"type": "function",
		"function": map[string]interface{}{
			"name":      name,
			"arguments": string(argsJSON),
		},
	}
}

// BuildToolDefinition creates a tool definition
func BuildToolDefinition(name, description string, parameters map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        name,
			"description": description,
			"parameters":  parameters,
		},
	}
}

// Kiro Request Builders

// BuildKiroRequest creates a basic Kiro conversation state request
func BuildKiroRequest(currentMessage string, history []map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"conversationState": map[string]interface{}{
			"currentMessage": map[string]interface{}{
				"userInputMessage": map[string]interface{}{
					"content":                 currentMessage,
					"userInputMessageContext": map[string]interface{}{},
				},
			},
			"chatTriggerType": "MANUAL",
			"history":         history,
		},
	}
}

// BuildKiroHistoryItem creates a Kiro history item
func BuildKiroHistoryItem(utteranceType, message string) map[string]interface{} {
	return map[string]interface{}{
		"utteranceType": utteranceType,
		"message":       message,
	}
}

// Response Builders

// BuildOpenAIResponse creates a basic OpenAI response
func BuildOpenAIResponse(model, content string) map[string]interface{} {
	return map[string]interface{}{
		"id":      "chatcmpl-test123",
		"object":  "chat.completion",
		"created": 1700000000,
		"model":   model,
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": content,
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]interface{}{
			"prompt_tokens":     10,
			"completion_tokens": 20,
			"total_tokens":      30,
		},
	}
}

// BuildKiroResponse creates a basic Kiro response
func BuildKiroResponse(content string) map[string]interface{} {
	return map[string]interface{}{
		"conversationState": map[string]interface{}{
			"currentMessage": map[string]interface{}{
				"assistantMessage": map[string]interface{}{
					"content": content,
				},
			},
		},
	}
}

// BuildSSEChunk creates an SSE chunk for streaming
func BuildSSEChunk(eventType string, data map[string]interface{}) string {
	dataJSON, _ := json.Marshal(data)

	if eventType != "" {
		return "event: " + eventType + "\ndata: " + string(dataJSON) + "\n\n"
	}
	return "data: " + string(dataJSON) + "\n\n"
}

// MarshalJSON is a helper to marshal data for tests
func MarshalJSON(t *testing.T, v interface{}) []byte {
	t.Helper()

	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}

	return data
}

// MarshalJSONIndent marshals JSON with indentation for readability
func MarshalJSONIndent(t *testing.T, v interface{}) []byte {
	t.Helper()

	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}

	return data
}
