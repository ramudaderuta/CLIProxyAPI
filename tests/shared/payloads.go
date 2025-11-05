package testutil

import (
	"encoding/json"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro"
)

// BuildAnthropicMessageRequest creates a standard Anthropic Messages API request
func BuildAnthropicMessageRequest(model string, messages []map[string]interface{}, tools []map[string]interface{}, maxTokens int, temperature float64) map[string]interface{} {
	request := map[string]interface{}{
		"model":       model,
		"max_tokens":  maxTokens,
		"messages":    messages,
		"temperature": temperature,
	}

	if len(tools) > 0 {
		request["tools"] = tools
	}

	return request
}

// BuildOpenAIMessageRequest creates a standard OpenAI ChatCompletion request
func BuildOpenAIMessageRequest(model string, messages []map[string]interface{}, tools []map[string]interface{}, maxTokens int, temperature float64) map[string]interface{} {
	request := map[string]interface{}{
		"model":       model,
		"max_tokens":  maxTokens,
		"messages":    messages,
		"temperature": temperature,
	}

	if len(tools) > 0 {
		request["tools"] = tools
		request["tool_choice"] = "auto"
	}

	return request
}

// BuildKiroRequest creates a Kiro-specific request payload
func BuildKiroRequest(model string, messages []map[string]interface{}, tools []map[string]interface{}, stream bool) map[string]interface{} {
	request := map[string]interface{}{
		"model":    model,
		"messages": messages,
		"stream":   stream,
	}

	if len(tools) > 0 {
		request["tools"] = tools
	}

	return request
}

// BuildAnthropicTool creates an Anthropic-compatible tool definition
func BuildAnthropicTool(name, description string, inputSchema map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"name":        name,
		"description": description,
		"input_schema": inputSchema,
	}
}

// BuildOpenAITool creates an OpenAI-compatible tool definition
func BuildOpenAITool(name, description string, parameters map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"type":        "function",
		"function": map[string]interface{}{
			"name":        name,
			"description": description,
			"parameters":  parameters,
		},
	}
}

// BuildTextMessage creates a simple text message
func BuildTextMessage(role, content string) map[string]interface{} {
	return map[string]interface{}{
		"role":    role,
		"content": content,
	}
}

// BuildToolCallMessage creates a message with tool calls
func BuildToolCallMessage(role string, toolCalls []kiro.OpenAIToolCall) map[string]interface{} {
	openAIToolCalls := make([]map[string]interface{}, len(toolCalls))

	for i, call := range toolCalls {
		openAIToolCalls[i] = map[string]interface{}{
			"id":   call.ID,
			"type": "function",
			"function": map[string]interface{}{
				"name":      call.Name,
				"arguments": call.Arguments,
			},
		}
	}

	return map[string]interface{}{
		"role":       role,
		"content":    nil,
		"tool_calls": openAIToolCalls,
	}
}

// BuildToolResultMessage creates a message with tool results
func BuildToolResultMessage(toolCallID, content string) map[string]interface{} {
	return map[string]interface{}{
		"role":       "tool",
		"tool_call_id": toolCallID,
		"content":    content,
	}
}

// BuildSystemMessage creates a system message
func BuildSystemMessage(content string) map[string]interface{} {
	return map[string]interface{}{
		"role":    "system",
		"content": content,
	}
}

// MockAnthropicResponse creates a mock Anthropic API response for testing
func MockAnthropicResponse(id, model string, content []map[string]interface{}, stopReason string, usage map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"id":           id,
		"type":         "message",
		"role":         "assistant",
		"model":        model,
		"content":      content,
		"stop_reason":  stopReason,
		"stop_sequence": nil,
		"usage":        usage,
	}
}

// MockOpenAIResponse creates a mock OpenAI API response for testing
func MockOpenAIResponse(id, model string, message map[string]interface{}, finishReason string, usage map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"id":      id,
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": message,
				"finish_reason": finishReason,
			},
		},
		"usage": usage,
	}
}

// MockStreamingChunk creates a mock streaming chunk for testing
func MockStreamingChunk(content string, toolCalls []kiro.OpenAIToolCall, finishReason string) []byte {
	chunk := map[string]interface{}{
		"id":      "chatcmpl-test",
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   "test-model",
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"delta": map[string]interface{}{
					"content": content,
				},
				"finish_reason": finishReason,
			},
		},
	}

	if len(toolCalls) > 0 {
		delta := chunk["choices"].([]map[string]interface{})[0]["delta"].(map[string]interface{})
		openAIToolCalls := make([]map[string]interface{}, len(toolCalls))

		for i, call := range toolCalls {
			openAIToolCalls[i] = map[string]interface{}{
				"id":   call.ID,
				"type": "function",
				"function": map[string]interface{}{
					"name":      call.Name,
					"arguments": call.Arguments,
				},
			}
		}

		delta["tool_calls"] = openAIToolCalls
	}

	data, _ := json.Marshal(chunk)
	return data
}

// MockSSEEvent creates a mock Server-Sent Event for testing
func MockSSEEvent(eventType string, data interface{}) []byte {
	dataBytes, _ := json.Marshal(data)
	return []byte("event: " + eventType + "\ndata: " + string(dataBytes) + "\n\n")
}

// BuildUsage creates a usage object for API responses
func BuildUsage(inputTokens, outputTokens int64) map[string]interface{} {
	return map[string]interface{}{
		"input_tokens":  inputTokens,
		"output_tokens": outputTokens,
		"total_tokens":  inputTokens + outputTokens,
	}
}

// BuildContentBlock creates a content block for Anthropic responses
func BuildContentBlock(blockType string, content interface{}) map[string]interface{} {
	block := map[string]interface{}{
		"type": blockType,
	}

	switch blockType {
	case "text":
		block["text"] = content
	case "tool_use":
		if toolUse, ok := content.(map[string]interface{}); ok {
			block["id"] = toolUse["id"]
			block["name"] = toolUse["name"]
			block["input"] = toolUse["input"]
		}
	}

	return block
}

// BuildToolUseContent creates a tool_use content block
func BuildToolUseContent(id, name string, input interface{}) map[string]interface{} {
	return map[string]interface{}{
		"type":  "tool_use",
		"id":    id,
		"name":  name,
		"input": input,
	}
}

// BuildTextContent creates a text content block
func BuildTextContent(text string) map[string]interface{} {
	return map[string]interface{}{
		"type": "text",
		"text": text,
	}
}