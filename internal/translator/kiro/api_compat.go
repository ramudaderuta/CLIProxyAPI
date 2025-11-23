// Package kiro provides API compatibility wrappers for backward compatibility.
// These functions provide the contract-documented API while delegating to actual implementations.
package kiro

import (
	"encoding/json"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
	chat_completions "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro/openai/chat-completions"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro/openai/responses"
	"github.com/tidwall/gjson"
)

// OpenAIToolCall represents a tool call in OpenAI format.
type OpenAIToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function map[string]interface{} `json:"function"`
}

// BuildRequest converts an OpenAI-compatible chat payload into Kiro's conversation request format.
// This is a compatibility wrapper that delegates to ConvertOpenAIRequestToKiro.
//
// Parameters:
//   - model: The Kiro model name
//   - payload: The raw OpenAI request JSON
//   - token: Kiro token storage (currently unused, kept for API compatibility)
//   - metadata: Additional metadata (currently unused)
//
// Returns:
//   - []byte: The transformed request data in Kiro conversationState format
//   - error: Always returns nil (errors are handled internally)
func BuildRequest(
	model string,
	payload []byte,
	token *kiro.KiroTokenStorage,
	metadata map[string]any,
) ([]byte, error) {
	// Delegate to actual implementation
	// Note: stream parameter set to false for non-streaming requests
	result := chat_completions.ConvertOpenAIRequestToKiro(model, payload, false)
	return result, nil
}

// ParseResponse extracts assistant text and tool calls from a Kiro upstream payload.
// This is a compatibility wrapper that provides the documented contract API.
//
// Parameters:
//   - data: The raw Kiro response data (JSON or SSE)
//
// Returns:
//   - string: The assistant's text response
//   - []OpenAIToolCall: List of tool calls if any
func ParseResponse(data []byte) (string, []OpenAIToolCall) {
	// First, convert Kiro response to OpenAI format
	// Assuming model name doesn't affect parsing, use empty string
	openAIResp := responses.ConvertKiroResponseToOpenAI(data, "", false)

	// Parse the OpenAI response to extract text and tool calls
	parsed := gjson.ParseBytes(openAIResp)

	// Extract content from first choice
	content := parsed.Get("choices.0.message.content").String()

	// Extract tool calls if present
	var toolCalls []OpenAIToolCall
	toolCallsArray := parsed.Get("choices.0.message.tool_calls").Array()

	if len(toolCallsArray) > 0 {
		for _, tc := range toolCallsArray {
			var toolCall OpenAIToolCall
			// Parse each tool call
			if err := json.Unmarshal([]byte(tc.Raw), &toolCall); err == nil {
				toolCalls = append(toolCalls, toolCall)
			}
		}
	}

	return content, toolCalls
}
