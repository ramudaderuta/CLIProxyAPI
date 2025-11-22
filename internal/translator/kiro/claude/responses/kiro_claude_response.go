package responses

import (
	"encoding/json"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro/openai/responses"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
)

// ConvertKiroResponseToClaude converts a Kiro response to Claude Messages API format.
// Since Kiro responses are similar to OpenAI, we reuse the OpenAI converter and adjust the format.
func ConvertKiroResponseToClaude(kiroResponse []byte, model string, streaming bool) []byte {
	// First convert to OpenAI format
	openAIResponse := responses.ConvertKiroResponseToOpenAI(kiroResponse, model, streaming)

	// Parse OpenAI response
	id := gjson.GetBytes(openAIResponse, "id").String()
	finishReason := gjson.GetBytes(openAIResponse, "choices.0.finish_reason").String()
	messageContent := gjson.GetBytes(openAIResponse, "choices.0.message.content").String()
	toolCalls := gjson.GetBytes(openAIResponse, "choices.0.message.tool_calls").Raw

	// Parse usage
	inputTokens := gjson.GetBytes(openAIResponse, "usage.prompt_tokens").Int()
	outputTokens := gjson.GetBytes(openAIResponse, "usage.completion_tokens").Int()

	// Build Claude response
	claudeResponse := map[string]interface{}{
		"id":          id,
		"type":        "message",
		"role":        "assistant",
		"model":       model,
		"stop_reason": convertFinishReason(finishReason),
	}

	// Build content array
	content := []map[string]interface{}{}

	if messageContent != "" {
		content = append(content, map[string]interface{}{
			"type": "text",
			"text": messageContent,
		})
	}

	// Add tool calls if present
	if toolCalls != "" && toolCalls != "null" {
		var tools []interface{}
		if err := json.Unmarshal([]byte(toolCalls), &tools); err == nil {
			for _, tool := range tools {
				toolMap := tool.(map[string]interface{})
				if function, ok := toolMap["function"].(map[string]interface{}); ok {
					content = append(content, map[string]interface{}{
						"type":  "tool_use",
						"id":    toolMap["id"],
						"name":  function["name"],
						"input": function["arguments"],
					})
				}
			}
		}
	}

	claudeResponse["content"] = content

	// Add usage
	claudeResponse["usage"] = map[string]interface{}{
		"input_tokens":  inputTokens,
		"output_tokens": outputTokens,
	}

	// Convert to JSON
	result, err := json.Marshal(claudeResponse)
	if err != nil {
		log.Errorf("Failed to marshal Claude response: %v", err)
		return kiroResponse
	}

	return result
}

// ConvertKiroStreamChunkToClaude converts a Kiro SSE stream chunk to Claude format
func ConvertKiroStreamChunkToClaude(chunkData []byte, model string) []byte {
	// First convert to OpenAI format
	openAIChunk := responses.ConvertKiroStreamChunkToOpenAI(chunkData, model)

	// For streaming, Claude uses similar SSE format but with different event names
	// Convert the OpenAI chunk to Claude format
	deltaContent := gjson.GetBytes(openAIChunk, "choices.0.delta.content").String()

	if deltaContent != "" {
		claudeChunk := map[string]interface{}{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]interface{}{
				"type": "text_delta",
				"text": deltaContent,
			},
		}

		result, _ := json.Marshal(claudeChunk)
		return result
	}

	// For other events, return the OpenAI format (close enough)
	return openAIChunk
}

// convertFinishReason converts OpenAI finish_reason to Claude stop_reason
func convertFinishReason(openAIReason string) string {
	switch strings.ToLower(openAIReason) {
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "tool_calls", "function_call":
		return "tool_use"
	case "content_filter":
		return "stop_sequence"
	default:
		return "end_turn"
	}
}
