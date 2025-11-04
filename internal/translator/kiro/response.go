package kiro

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tidwall/gjson"
)

// OpenAIToolCall represents a function/tool call in an OpenAI-compatible response.
type OpenAIToolCall struct {
	ID        string
	Name      string
	Arguments string
}

// ParseResponse extracts assistant text and tool calls from a Kiro upstream payload.
func ParseResponse(data []byte) (string, []OpenAIToolCall) {
	if len(data) == 0 {
		return "", nil
	}
	if gjson.ValidBytes(data) {
		root := gjson.ParseBytes(data)

		// Extract content from currentMessage
		var content string
		if contentField := root.Get("conversationState.currentMessage.assistantResponseMessage.content"); contentField.Exists() {
			content = contentField.String()
		} else if history := root.Get("conversationState.history"); history.Exists() && history.IsArray() {
			// Look for content in history if not in currentMessage
			for i := len(history.Array()) - 1; i >= 0; i-- {
				item := history.Array()[i]
				if contentField := item.Get("assistantResponseMessage.content"); contentField.Exists() {
					content = contentField.String()
					break
				}
			}
		}

		// Extract tool calls from currentMessage (check both locations)
		var toolCalls []OpenAIToolCall
		// Check for toolUse at currentMessage level
		if toolUse := root.Get("conversationState.currentMessage.toolUse"); toolUse.Exists() {
			if toolUse.IsArray() {
				toolCalls = extractToolCalls(toolUse.Array())
			} else {
				toolCalls = extractToolCalls([]gjson.Result{toolUse})
			}
		} else if toolUse := root.Get("conversationState.currentMessage.assistantResponseMessage.toolUse"); toolUse.Exists() {
			// Check for toolUse nested inside assistantResponseMessage
			if toolUse.IsArray() {
				toolCalls = extractToolCalls(toolUse.Array())
			} else {
				toolCalls = extractToolCalls([]gjson.Result{toolUse})
			}
		}

		return content, toolCalls
	}
	return parseEventStream(string(data))
}

// extractToolCalls converts gjson toolUse objects into OpenAIToolCall structures
func extractToolCalls(toolUses []gjson.Result) []OpenAIToolCall {
	toolCalls := make([]OpenAIToolCall, 0, len(toolUses))
	for _, toolUse := range toolUses {
		toolID := toolUse.Get("toolUseId").String()
		name := toolUse.Get("name").String()

		if toolID == "" || name == "" {
			continue
		}

		// Extract and format input arguments
		var arguments string
		if input := toolUse.Get("input"); input.Exists() {
			if input.IsObject() {
				arguments = input.Raw
			} else {
				// Handle non-object inputs
				inputMap := map[string]any{"value": input.String()}
				if argsBytes, err := json.Marshal(inputMap); err == nil {
					arguments = string(argsBytes)
				}
			}
		}

		toolCalls = append(toolCalls, OpenAIToolCall{
			ID:        toolID,
			Name:      name,
			Arguments: arguments,
		})
	}
	return toolCalls
}

// BuildOpenAIChatCompletionPayload generates a non-streaming OpenAI-compatible chat completion response.
func BuildOpenAIChatCompletionPayload(model, content string, toolCalls []OpenAIToolCall, promptTokens, completionTokens int64) ([]byte, error) {
	message := map[string]any{
		"role":    "assistant",
		"content": content,
	}
	if len(toolCalls) > 0 {
		tc := make([]map[string]any, 0, len(toolCalls))
		for _, call := range toolCalls {
			tc = append(tc, map[string]any{
				"id":   call.ID,
				"type": "function",
				"function": map[string]any{
					"name":      call.Name,
					"arguments": call.Arguments,
				},
			})
		}
		message["tool_calls"] = tc
	}

	payload := map[string]any{
		"id":      fmt.Sprintf("chatcmpl_%s", uuid.NewString()),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []map[string]any{{
			"index":         0,
			"message":       message,
			"finish_reason": "stop",
		}},
		"usage": map[string]any{
			"prompt_tokens":     promptTokens,
			"completion_tokens": completionTokens,
			"total_tokens":      promptTokens + completionTokens,
		},
	}
	return json.Marshal(payload)
}

// BuildStreamingChunks returns OpenAI-compatible streaming chunks for the provided result.
func BuildStreamingChunks(id, model string, created int64, content string, toolCalls []OpenAIToolCall) [][]byte {
	chunks := make([][]byte, 0, 3)
	initial := map[string]any{
		"id":      id,
		"object":  "chat.completion.chunk",
		"created": created,
		"model":   model,
		"choices": []map[string]any{{
			"index": 0,
			"delta": map[string]any{"role": "assistant"},
		}},
	}
	chunks = append(chunks, marshalJSON(initial))

	if strings.TrimSpace(content) != "" {
		data := map[string]any{
			"id":      id,
			"object":  "chat.completion.chunk",
			"created": created,
			"model":   model,
			"choices": []map[string]any{{
				"index": 0,
				"delta": map[string]any{"content": content},
			}},
		}
		chunks = append(chunks, marshalJSON(data))
	}

	if len(toolCalls) > 0 {
		tc := make([]map[string]any, 0, len(toolCalls))
		for _, call := range toolCalls {
			tc = append(tc, map[string]any{
				"id":   call.ID,
				"type": "function",
				"function": map[string]any{
					"name":      call.Name,
					"arguments": call.Arguments,
				},
			})
		}
		data := map[string]any{
			"id":      id,
			"object":  "chat.completion.chunk",
			"created": created,
			"model":   model,
			"choices": []map[string]any{{
				"index": 0,
				"delta": map[string]any{"tool_calls": tc},
			}},
		}
		chunks = append(chunks, marshalJSON(data))
	}

	final := map[string]any{
		"id":      id,
		"object":  "chat.completion.chunk",
		"created": created,
		"model":   model,
		"choices": []map[string]any{{
			"index":         0,
			"delta":         map[string]any{},
			"finish_reason": "stop",
		}},
	}
	chunks = append(chunks, marshalJSON(final))
	return chunks
}

func parseEventStream(raw string) (string, []OpenAIToolCall) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	result := strings.Builder{}
	toolCalls := make([]OpenAIToolCall, 0)
	var currentCall *OpenAIToolCall

	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if idx := strings.Index(line, "{"); idx >= 0 {
			line = line[idx:]
		}
		event := firstValidJSON(line)
		if len(event) == 0 {
			continue
		}
		node := gjson.ParseBytes(event)
		if name := node.Get("name").String(); name != "" && node.Get("toolUseId").Exists() {
			if currentCall == nil {
				currentCall = &OpenAIToolCall{
					ID:   node.Get("toolUseId").String(),
					Name: name,
				}
			}
			if input := node.Get("input"); input.Exists() {
				currentCall.Arguments += input.Raw
			}
			if node.Get("stop").Bool() && currentCall != nil {
				if args := normalizeArguments(currentCall.Arguments); args != "" {
					currentCall.Arguments = args
				}
				toolCalls = append(toolCalls, *currentCall)
				currentCall = nil
			}
			continue
		}
		if content := node.Get("content").String(); content != "" && !node.Get("followupPrompt").Bool() {
			decoded := strings.ReplaceAll(content, `\n`, "\n")
			result.WriteString(decoded)
		}
	}
	if currentCall != nil {
		if args := normalizeArguments(currentCall.Arguments); args != "" {
			currentCall.Arguments = args
		}
		toolCalls = append(toolCalls, *currentCall)
	}

	bracketCalls := parseBracketToolCalls(raw)
	if len(bracketCalls) > 0 {
		toolCalls = append(toolCalls, bracketCalls...)
	}

	content := strings.TrimSpace(result.String())
	if content == "" {
		content = strings.TrimSpace(raw)
	}
	return content, deduplicateToolCalls(toolCalls)
}

func parseBracketToolCalls(raw string) []OpenAIToolCall {
	pattern := regexp.MustCompile(`(?s)\[Called\s+([A-Za-z0-9_]+)\s+with\s+args:\s*(\{.*?\})\]`)
	matches := pattern.FindAllStringSubmatch(raw, -1)
	calls := make([]OpenAIToolCall, 0, len(matches))
	for _, match := range matches {
		name := match[1]
		argBlock := sanitizeJSON(match[2])
		if name == "" || argBlock == "" {
			continue
		}
		calls = append(calls, OpenAIToolCall{
			ID:        fmt.Sprintf("call_%s", uuid.New().String()),
			Name:      name,
			Arguments: argBlock,
		})
	}
	return calls
}

func deduplicateToolCalls(calls []OpenAIToolCall) []OpenAIToolCall {
	seen := make(map[string]struct{}, len(calls))
	deduped := make([]OpenAIToolCall, 0, len(calls))
	for _, call := range calls {
		key := call.Name + ":" + call.Arguments
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, call)
	}
	return deduped
}

func sanitizeJSON(input string) string {
	if input == "" {
		return ""
	}
	value := regexp.MustCompile(`,\s*([}\]])`).ReplaceAllString(input, "$1")
	value = regexp.MustCompile(`([{,]\s*)([A-Za-z0-9_]+)\s*:`).ReplaceAllString(value, `$1"$2":`)
	if json.Valid([]byte(value)) {
		return value
	}
	return ""
}

func firstValidJSON(block string) []byte {
	block = strings.TrimSpace(block)
	for i := len(block); i > 0; i-- {
		snippet := strings.TrimSpace(block[:i])
		if len(snippet) == 0 {
			continue
		}
		if json.Valid([]byte(snippet)) {
			return []byte(snippet)
		}
	}
	return nil
}

func normalizeArguments(args string) string {
	args = strings.TrimSpace(args)
	if args == "" {
		return ""
	}
	if json.Valid([]byte(args)) {
		return args
	}
	if fixed := sanitizeJSON(args); fixed != "" {
		return fixed
	}
	return ""
}

// BuildAnthropicMessagePayload generates an Anthropic-compatible messages API response.
func BuildAnthropicMessagePayload(model, content string, toolCalls []OpenAIToolCall, promptTokens, completionTokens int64) ([]byte, error) {
	// Validation
	if model == "" {
		return nil, fmt.Errorf("model cannot be empty")
	}
	if promptTokens < 0 || completionTokens < 0 {
		return nil, fmt.Errorf("token count cannot be negative")
	}

	// Validate tool calls - only check for empty model and negative tokens
	// Allow empty tool call IDs for edge case compatibility

	// Build content blocks
	contentBlocks := make([]map[string]any, 0, 1+len(toolCalls))

	// Add text content block if content is not empty
	if strings.TrimSpace(content) != "" {
		contentBlocks = append(contentBlocks, map[string]any{
			"type": "text",
			"text": content,
		})
	}

	// Add tool_use blocks
	for _, call := range toolCalls {
		var input map[string]any
		if call.Arguments != "" && call.Arguments != "null" {
			if err := json.Unmarshal([]byte(call.Arguments), &input); err != nil {
				// If JSON parsing fails, treat as string value
				input = map[string]any{"value": call.Arguments}
			}
		} else {
			input = map[string]any{}
		}

		contentBlocks = append(contentBlocks, map[string]any{
			"type":  "tool_use",
			"id":    call.ID,
			"name":  call.Name,
			"input": input,
		})
	}

	// Determine stop reason - check for max_tokens scenario
	stopReason := "end_turn"
	if len(toolCalls) > 0 {
		stopReason = "tool_use"
	} else if strings.Contains(content, "cut off due to max tokens") { // Check for max_tokens indicator in content
		stopReason = "max_tokens"
	}

	// Build the payload with proper structure
	payload := AnthropicMessage{
		ID:          fmt.Sprintf("msg_%s", uuid.NewString()),
		Type:        "message",
		Role:        "assistant",
		Model:       model,
		Content:     contentBlocks,
		StopReason:  stopReason,
		StopSequence: nil, // BUG FIX: stop_sequence should be null per Anthropic spec
		Usage: Usage{
			InputTokens:  promptTokens,
			OutputTokens: completionTokens,
			TotalTokens:  promptTokens + completionTokens,
		},
	}

	return json.Marshal(payload)
}

// AnthropicMessage represents the Anthropic messages API response structure
type AnthropicMessage struct {
	ID          string        `json:"id"`
	Type        string        `json:"type"`
	Role        string        `json:"role"`
	Model       string        `json:"model"`
	Content     []map[string]any `json:"content"`
	StopReason  string        `json:"stop_reason"`
	StopSequence *string       `json:"stop_sequence"` // BUG FIX: Use pointer to allow null
	Usage       Usage         `json:"usage"`
}

// Usage represents token usage with int64 types
type Usage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
	TotalTokens int64 `json:"total_tokens"`
}

// BuildAnthropicStreamingChunks generates Anthropic-compatible streaming chunks formatted as SSE events.
func BuildAnthropicStreamingChunks(id, model string, created int64, content string, toolCalls []OpenAIToolCall) [][]byte {
	chunks := make([][]byte, 0, 3)

	// Calculate approximate tokens
	outputTokens := calculateOutputTokens(content, toolCalls)
	inputTokens := int64(20) // TODO: Calculate actual input tokens from request

	// Initial message_start chunk
	messageStart := buildMessageStartEvent(model, inputTokens)
	chunks = append(chunks, buildSSEEvent("message_start", messageStart))

	// Content block chunks
	if strings.TrimSpace(content) != "" {
		// content_block_start
		contentStart := buildContentBlockStartEvent()
		chunks = append(chunks, buildSSEEvent("content_block_start", contentStart))

		// content_block_delta (text content)
		contentDelta := buildContentBlockDeltaEvent(content)
		chunks = append(chunks, buildSSEEvent("content_block_delta", contentDelta))

		// content_block_stop
		contentStop := buildContentBlockStopEvent()
		chunks = append(chunks, buildSSEEvent("content_block_stop", contentStop))
	}

	// Tool use chunks
	for i, call := range toolCalls {
		blockIndex := i
		if strings.TrimSpace(content) != "" {
			blockIndex++ // Account for text block
		}

		// content_block_start for tool_use
		toolStart := buildToolUseStartEvent(call, blockIndex)
		chunks = append(chunks, buildSSEEvent("content_block_start", toolStart))

		// content_block_delta for tool input
		toolDelta := buildToolUseDeltaEvent(call)
		toolDelta["index"] = blockIndex
		chunks = append(chunks, buildSSEEvent("content_block_delta", toolDelta))

		// content_block_stop
		toolStop := buildContentBlockStopEvent()
		toolStop["index"] = blockIndex
		chunks = append(chunks, buildSSEEvent("content_block_stop", toolStop))
	}

	// message_delta with usage
	messageDelta := buildMessageDeltaEvent(len(toolCalls) > 0, inputTokens, outputTokens)
	chunks = append(chunks, buildSSEEvent("message_delta", messageDelta))

	// message_stop
	messageStop := buildMessageStopEvent()
	chunks = append(chunks, buildSSEEvent("message_stop", messageStop))

	return chunks
}

// Helper functions for building SSE event components

// buildMessageStartEvent creates the message_start event structure
func buildMessageStartEvent(model string, inputTokens int64) map[string]any {
	return map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id":      fmt.Sprintf("msg_%s", uuid.NewString()),
			"type":    "message",
			"role":    "assistant",
			"content": []map[string]any{},
			"model":   model,
			"stop_reason":   nil,
			"stop_sequence": nil, // BUG FIX: stop_sequence should be null per Anthropic spec
			"usage": map[string]any{
				"input_tokens":  inputTokens,
				"output_tokens": 0, // BUG FIX: output_tokens should be 0 at message_start
			},
		},
	}
}

// buildContentBlockStartEvent creates the content_block_start event structure
func buildContentBlockStartEvent() map[string]any {
	return map[string]any{
		"type": "content_block_start",
		"index": 0,
		"content_block": map[string]any{
			"type": "text",
			"text": "",
		},
	}
}

// buildContentBlockDeltaEvent creates the content_block_delta event structure for text
func buildContentBlockDeltaEvent(content string) map[string]any {
	return map[string]any{
		"type": "content_block_delta",
		"index": 0,
		"delta": map[string]any{
			"type": "text_delta",
			"text": content,
		},
	}
}

// buildContentBlockStopEvent creates the content_block_stop event structure
func buildContentBlockStopEvent() map[string]any {
	return map[string]any{
		"type": "content_block_stop",
		"index": 0,
	}
}

// buildToolUseStartEvent creates the content_block_start event structure for tool_use
func buildToolUseStartEvent(call OpenAIToolCall, blockIndex int) map[string]any {
	return map[string]any{
		"type": "content_block_start",
		"index": blockIndex,
		"content_block": map[string]any{
			"type": "tool_use",
			"id":   call.ID,
			"name": call.Name,
			"input": map[string]any{},
		},
	}
}

// buildToolUseDeltaEvent creates the content_block_delta event structure for tool input
func buildToolUseDeltaEvent(call OpenAIToolCall) map[string]any {
	var input map[string]any
	if call.Arguments != "" && call.Arguments != "null" {
		if err := json.Unmarshal([]byte(call.Arguments), &input); err != nil {
			input = map[string]any{"value": call.Arguments}
		}
	} else {
		input = map[string]any{}
	}

	return map[string]any{
		"type": "content_block_delta",
		"index": 0, // Will be updated by caller
		"delta": map[string]any{
			"type": "input_json_delta",
			"partial_json": string(marshalJSON(input)),
		},
	}
}

// buildMessageDeltaEvent creates the message_delta event structure
func buildMessageDeltaEvent(hasToolCalls bool, inputTokens, outputTokens int64) map[string]any {
	stopReason := "end_turn"
	if hasToolCalls {
		stopReason = "tool_use"
	}

	return map[string]any{
		"type": "message_delta",
		"delta": map[string]any{
			"stop_reason":   stopReason,
			"stop_sequence": nil, // BUG FIX: stop_sequence should be null per Anthropic spec
		},
		"usage": map[string]any{
			"input_tokens":  inputTokens,  // BUG FIX: Include input_tokens in final usage
			"output_tokens": outputTokens, // BUG FIX: Use actual token count instead of hardcoded 0
		},
	}
}

// buildMessageStopEvent creates the message_stop event structure
func buildMessageStopEvent() map[string]any {
	return map[string]any{
		"type": "message_stop",
	}
}

// buildSSEEvent creates a properly formatted SSE event string
func buildSSEEvent(eventType string, payload map[string]any) []byte {
	data, err := json.Marshal(payload)
	if err != nil {
		return []byte{}
	}
	return []byte(fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, string(data)))
}

// Helper function to marshal JSON without errors
func marshalJSON(v any) []byte {
	data, _ := json.Marshal(v)
	return data
}

// calculateOutputTokens provides a more accurate approximation of output tokens based on content length
// This is a basic implementation - in production, you'd use a proper tokenizer
func calculateOutputTokens(content string, toolCalls []OpenAIToolCall) int64 {
	if content == "" && len(toolCalls) == 0 {
		return 0
	}

	// More accurate approximation: ~3-4 characters per token for English text
	// Using 3.5 as a middle ground for better accuracy
	contentTokens := int64(float64(len(content)) / 3.5)
	if contentTokens == 0 && strings.TrimSpace(content) != "" {
		contentTokens = 1
	}

	// Add tokens for tool calls (rough approximation)
	toolTokens := int64(0)
	for _, call := range toolCalls {
		// Base cost for a tool call (function name + invocation overhead)
		toolTokens += 3 // Minimum tokens for any tool call

		// Approximate tokens for tool name
		if call.Name != "" {
			toolTokens += int64(float64(len(call.Name)) / 3.5)
		}

		// Approximate tokens for tool arguments
		if call.Arguments != "" && call.Arguments != "null" {
			toolTokens += int64(float64(len(call.Arguments)) / 3.5)
		}
	}

	// Ensure minimum token count, especially for tool-only responses
	totalTokens := contentTokens + toolTokens
	if totalTokens == 0 {
		if len(toolCalls) > 0 {
			totalTokens = 1 // At least 1 token for tool calls
		} else if strings.TrimSpace(content) != "" {
			totalTokens = 1 // At least 1 token for non-empty content
		}
	}

	return totalTokens
}

