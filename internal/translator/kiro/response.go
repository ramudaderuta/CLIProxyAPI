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
		if content := root.Get("conversationState.currentMessage.assistantResponseMessage.content"); content.Exists() {
			return content.String(), nil
		}
		if history := root.Get("conversationState.history"); history.Exists() && history.IsArray() {
			for i := len(history.Array()) - 1; i >= 0; i-- {
				item := history.Array()[i]
				if content := item.Get("assistantResponseMessage.content"); content.Exists() {
					return content.String(), nil
				}
			}
		}
	}
	return parseEventStream(string(data))
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
	chunks = append(chunks, marshalStreamChunk(initial))

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
		chunks = append(chunks, marshalStreamChunk(data))
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
		chunks = append(chunks, marshalStreamChunk(data))
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
	chunks = append(chunks, marshalStreamChunk(final))
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

func marshalStreamChunk(payload map[string]any) []byte {
	data, _ := json.Marshal(payload)
	return data
}
