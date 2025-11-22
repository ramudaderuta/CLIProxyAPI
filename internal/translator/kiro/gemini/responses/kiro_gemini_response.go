package responses

import (
	"encoding/json"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro/openai/responses"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
)

// ConvertKiroResponseToGemini converts a Kiro response to Gemini generateContent format.
func ConvertKiroResponseToGemini(kiroResponse []byte, model string, streaming bool) []byte {
	// First convert to OpenAI format (easier to work with)
	openAIResponse := responses.ConvertKiroResponseToOpenAI(kiroResponse, model, streaming)

	// Parse OpenAI response
	messageContent := gjson.GetBytes(openAIResponse, "choices.0.message.content").String()
	finishReason := gjson.GetBytes(openAIResponse, "choices.0.finish_reason").String()
	toolCalls := gjson.GetBytes(openAIResponse, "choices.0.message.tool_calls").Raw

	// Parse usage
	inputTokens := gjson.GetBytes(openAIResponse, "usage.prompt_tokens").Int()
	outputTokens := gjson.GetBytes(openAIResponse, "usage.completion_tokens").Int()

	// Build Gemini response
	geminiResponse := map[string]interface{}{
		"candidates": []map[string]interface{}{
			{
				"content": map[string]interface{}{
					"parts": []map[string]interface{}{},
					"role":  "model",
				},
				"finishReason": convertFinishReasonToGemini(finishReason),
			},
		},
		"usageMetadata": map[string]interface{}{
			"promptTokenCount":     inputTokens,
			"candidatesTokenCount": outputTokens,
			"totalTokenCount":      inputTokens + outputTokens,
		},
	}

	// Build parts array
	parts := []map[string]interface{}{}

	if messageContent != "" {
		parts = append(parts, map[string]interface{}{
			"text": messageContent,
		})
	}

	// Add function calls if present
	if toolCalls != "" && toolCalls != "null" {
		var tools []interface{}
		if err := json.Unmarshal([]byte(toolCalls), &tools); err == nil {
			for _, tool := range tools {
				toolMap := tool.(map[string]interface{})
				if function, ok := toolMap["function"].(map[string]interface{}); ok {
					// Parse arguments
					var args map[string]interface{}
					if argsStr, ok := function["arguments"].(string); ok {
						json.Unmarshal([]byte(argsStr), &args)
					}

					parts = append(parts, map[string]interface{}{
						"functionCall": map[string]interface{}{
							"name": function["name"],
							"args": args,
						},
					})
				}
			}
		}
	}

	// Set parts in candidate
	geminiResponse["candidates"].([]map[string]interface{})[0]["content"].(map[string]interface{})["parts"] = parts

	// Convert to JSON
	result, err := json.Marshal(geminiResponse)
	if err != nil {
		log.Errorf("Failed to marshal Gemini response: %v", err)
		return kiroResponse
	}

	return result
}

// ConvertKiroStreamChunkToGemini converts a Kiro SSE stream chunk to Gemini streaming format
func ConvertKiroStreamChunkToGemini(chunkData []byte, model string) []byte {
	// First convert to OpenAI format
	openAIChunk := responses.ConvertKiroStreamChunkToOpenAI(chunkData, model)

	// Parse OpenAI chunk
	deltaContent := gjson.GetBytes(openAIChunk, "choices.0.delta.content").String()
	finishReason := gjson.GetBytes(openAIChunk, "choices.0.finish_reason").String()

	// Build Gemini chunk
	parts := []map[string]interface{}{}
	if deltaContent != "" {
		parts = append(parts, map[string]interface{}{
			"text": deltaContent,
		})
	}

	geminiChunk := map[string]interface{}{
		"candidates": []map[string]interface{}{
			{
				"content": map[string]interface{}{
					"parts": parts,
					"role":  "model",
				},
			},
		},
	}

	if finishReason != "" {
		geminiChunk["candidates"].([]map[string]interface{})[0]["finishReason"] = convertFinishReasonToGemini(finishReason)
	}

	result, _ := json.Marshal(geminiChunk)
	return result
}

// convertFinishReasonToGemini converts OpenAI finish_reason to Gemini finishReason
func convertFinishReasonToGemini(openAIReason string) string {
	switch openAIReason {
	case "stop":
		return "STOP"
	case "length":
		return "MAX_TOKENS"
	case "tool_calls", "function_call":
		return "STOP" // Gemini doesn't have a specific finish reason for function calls
	case "content_filter":
		return "SAFETY"
	default:
		return "STOP"
	}
}
