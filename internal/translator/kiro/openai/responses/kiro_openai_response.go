// Package responses provides response translation functionality from Kiro to OpenAI format.
// It converts Kiro conversationState responses into OpenAI Chat Completions format.
package responses

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

var (
	// Regex to match thinking content blocks
	thinkingRegex = regexp.MustCompile(`(?is)<thinking>.*?</thinking>`)
)

// ConvertKiroResponseToOpenAI converts a Kiro conversationState response to OpenAI format.
//
// Parameters:
//   - kiroResponse: The raw JSON response from Kiro API
//   - model: The model name to include in the response
//   - stream: Whether this is a streaming response
//
// Returns:
//   - []byte: The transformed response in OpenAI Chat Completions format
func ConvertKiroResponseToOpenAI(kiroResponse []byte, model string, stream bool) []byte {
	kiroJSON := gjson.ParseBytes(kiroResponse)

	// Extract the assistant message from conversationState
	message := kiroJSON.Get("conversationState.currentMessage")
	if !message.Exists() {
		// Fallback: try to get response directly
		message = kiroJSON.Get("response")
	}

	// Build OpenAI response
	openAIResp := buildOpenAIResponse(message, model)

	// Extract usage information
	usage := extractUsage(kiroJSON)
	if usage != nil {
		usageJSON, _ := json.Marshal(usage)
		openAIResp, _ = sjson.SetRaw(openAIResp, "usage", string(usageJSON))
	}

	return []byte(openAIResp)
}

// buildOpenAIResponse builds an OpenAI-compatible response from Kiro message
func buildOpenAIResponse(message gjson.Result, model string) string {
	resp := `{}`

	// Set ID
	resp, _ = sjson.Set(resp, "id", "chatcmpl-"+uuid.New().String()[:12])

	// Set object type
	resp, _ = sjson.Set(resp, "object", "chat.completion")

	// Set created timestamp
	resp, _ = sjson.Set(resp, "created", time.Now().Unix())

	// Set model
	resp, _ = sjson.Set(resp, "model", model)

	// Build choice
	choice := buildChoice(message)
	choiceJSON, _ := json.Marshal(choice)
	resp, _ = sjson.SetRaw(resp, "choices.0", string(choiceJSON))

	return resp
}

// buildChoice builds a single choice object from Kiro message
func buildChoice(message gjson.Result) map[string]interface{} {
	content := message.Get("content").String()

	// Filter thinking content
	content = FilterThinkingContent(content)

	choice := map[string]interface{}{
		"index": 0,
		"message": map[string]interface{}{
			"role":    "assistant",
			"content": content,
		},
		"finish_reason": "stop",
	}

	// Handle tool calls
	toolCalls := message.Get("toolCalls")
	if toolCalls.Exists() && toolCalls.IsArray() {
		var calls []map[string]interface{}
		for _, tc := range toolCalls.Array() {
			callID := tc.Get("id").String()
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

		if len(calls) > 0 {
			choice["message"].(map[string]interface{})["tool_calls"] = calls
			choice["finish_reason"] = "tool_calls"
		}
	}

	return choice
}

// extractUsage extracts usage information from Kiro response
func extractUsage(kiroJSON gjson.Result) map[string]interface{} {
	// Try multiple paths for usage data
	usage := kiroJSON.Get("usage")
	if !usage.Exists() {
		usage = kiroJSON.Get("conversationState.usage")
	}
	if !usage.Exists() {
		usage = kiroJSON.Get("metadata.usage")
	}

	if !usage.Exists() {
		return nil
	}

	promptTokens := usage.Get("promptTokens").Int()
	completionTokens := usage.Get("completionTokens").Int()
	totalTokens := usage.Get("totalTokens").Int()

	// If total not provided, calculate it
	if totalTokens == 0 && (promptTokens > 0 || completionTokens > 0) {
		totalTokens = promptTokens + completionTokens
	}

	return map[string]interface{}{
		"prompt_tokens":     promptTokens,
		"completion_tokens": completionTokens,
		"total_tokens":      totalTokens,
	}
}

// FilterThinkingContent removes <thinking>...</thinking> blocks from content
func FilterThinkingContent(content string) string {
	if content == "" {
		return content
	}

	// Remove thinking tags and content
	filtered := thinkingRegex.ReplaceAllString(content, "")

	// Clean up extra whitespace
	filtered = strings.TrimSpace(filtered)

	// Collapse multiple newlines
	filtered = regexp.MustCompile(`\n{3,}`).ReplaceAllString(filtered, "\n\n")

	return filtered
}

// ConvertKiroStreamChunkToOpenAI converts a Kiro SSE stream chunk to OpenAI format
//
// Parameters:
//   - chunkData: The raw SSE chunk data
//   - model: The model name
//
// Returns:
//   - []byte: The transformed chunk in OpenAI streaming format
func ConvertKiroStreamChunkToOpenAI(chunkData []byte, model string) []byte {
	chunkJSON := gjson.ParseBytes(chunkData)

	// Determine event type
	eventType := chunkJSON.Get("type").String()

	switch eventType {
	case "messageStart", "message_start":
		return buildStreamStartChunk(model)

	case "contentBlockStart", "content_block_start":
		return buildContentBlockStartChunk(model)

	case "contentBlockDelta", "content_block_delta":
		deltaText := chunkJSON.Get("delta.text").String()
		if deltaText == "" {
			deltaText = chunkJSON.Get("delta.content").String()
		}
		// Filter thinking content from delta
		deltaText = FilterThinkingContent(deltaText)
		return buildContentDeltaChunk(model, deltaText)

	case "contentBlockStop", "content_block_stop":
		return buildContentBlockStopChunk(model)

	case "messageDelta", "message_delta":
		return buildMessageDeltaChunk(model, chunkJSON)

	case "messageStop", "message_stop":
		return buildMessageStopChunk(model)

	default:
		log.Debugf("Unknown Kiro stream event type: %s", eventType)
		return nil
	}
}

// buildStreamStartChunk creates the initial streaming chunk
func buildStreamStartChunk(model string) []byte {
	chunk := map[string]interface{}{
		"id":      "chatcmpl-" + uuid.New().String()[:12],
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"delta": map[string]interface{}{
					"role": "assistant",
				},
				"finish_reason": nil,
			},
		},
	}

	result, _ := json.Marshal(chunk)
	return result
}

// buildContentBlockStartChunk creates a content block start chunk
func buildContentBlockStartChunk(model string) []byte {
	chunk := map[string]interface{}{
		"id":      "chatcmpl-" + uuid.New().String()[:12],
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"delta": map[string]interface{}{
					"content": "",
				},
				"finish_reason": nil,
			},
		},
	}

	result, _ := json.Marshal(chunk)
	return result
}

// buildContentDeltaChunk creates a content delta chunk
func buildContentDeltaChunk(model string, deltaText string) []byte {
	if deltaText == "" {
		return nil
	}

	chunk := map[string]interface{}{
		"id":      "chatcmpl-" + uuid.New().String()[:12],
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"delta": map[string]interface{}{
					"content": deltaText,
				},
				"finish_reason": nil,
			},
		},
	}

	result, _ := json.Marshal(chunk)
	return result
}

// buildContentBlockStopChunk creates a content block stop chunk
func buildContentBlockStopChunk(model string) []byte {
	chunk := map[string]interface{}{
		"id":      "chatcmpl-" + uuid.New().String()[:12],
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []map[string]interface{}{
			{
				"index":         0,
				"delta":         map[string]interface{}{},
				"finish_reason": nil,
			},
		},
	}

	result, _ := json.Marshal(chunk)
	return result
}

// buildMessageDeltaChunk creates a message delta chunk with usage
func buildMessageDeltaChunk(model string, chunkJSON gjson.Result) []byte {
	chunk := map[string]interface{}{
		"id":      "chatcmpl-" + uuid.New().String()[:12],
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []map[string]interface{}{
			{
				"index":         0,
				"delta":         map[string]interface{}{},
				"finish_reason": nil,
			},
		},
	}

	// Add usage if present
	usage := chunkJSON.Get("usage")
	if usage.Exists() {
		chunk["usage"] = map[string]interface{}{
			"prompt_tokens":     usage.Get("promptTokens").Int(),
			"completion_tokens": usage.Get("completionTokens").Int(),
			"total_tokens":      usage.Get("totalTokens").Int(),
		}
	}

	result, _ := json.Marshal(chunk)
	return result
}

// buildMessageStopChunk creates the final message stop chunk
func buildMessageStopChunk(model string) []byte {
	chunk := map[string]interface{}{
		"id":      "chatcmpl-" + uuid.New().String()[:12],
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []map[string]interface{}{
			{
				"index":         0,
				"delta":         map[string]interface{}{},
				"finish_reason": "stop",
			},
		},
	}

	result, _ := json.Marshal(chunk)
	return result
}
