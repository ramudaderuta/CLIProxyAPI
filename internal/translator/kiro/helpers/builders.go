package helpers

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Utility helpers shared across Kiro translators/stream mappers.

func FirstString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func SanitizeToolCallID(id string) string {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return fmt.Sprintf("call_%s", uuid.NewString())
	}
	return trimmed
}

func ValidateToolCallID(id string) bool {
	return strings.TrimSpace(id) != ""
}

func NormalizeArguments(args string) string {
	args = strings.TrimSpace(args)
	if args == "" {
		return ""
	}

	// Try parsing as JSON first
	var jsonObj map[string]interface{}
	if err := json.Unmarshal([]byte(args), &jsonObj); err == nil {
		// Re-marshal to normalize formatting
		if normalized, err := json.Marshal(jsonObj); err == nil {
			return string(normalized)
		}
	}

	// If not valid JSON, wrap as a string value
	wrapped := map[string]string{"value": args}
	if normalized, err := json.Marshal(wrapped); err == nil {
		return string(normalized)
	}

	return args
}

// SSE builder helpers
func BuildSSEEvent(eventType string, payload map[string]any) []byte {
	jsonBytes, _ := json.Marshal(payload)
	return []byte(fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, string(jsonBytes)))
}

func BuildMessageStartEvent(model string) map[string]any {
	return map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id":            fmt.Sprintf("msg_%s", uuid.NewString()),
			"type":          "message",
			"role":          "assistant",
			"content":       []map[string]any{},
			"model":         model,
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage": map[string]any{
				"input_tokens":  0,
				"output_tokens": 0,
			},
		},
	}
}

func BuildContentBlockStartEvent(index int) map[string]any {
	return map[string]any{
		"type":  "content_block_start",
		"index": index,
		"content_block": map[string]any{
			"type": "text",
			"text": "",
		},
	}
}

func BuildContentBlockDeltaEvent(index int, content string) map[string]any {
	return map[string]any{
		"type":  "content_block_delta",
		"index": index,
		"delta": map[string]any{
			"type": "text_delta",
			"text": content,
		},
	}
}

func BuildContentBlockStopEvent(index int) map[string]any {
	return map[string]any{
		"type":  "content_block_stop",
		"index": index,
	}
}

func BuildMessageDeltaEvent(stopReason string, inputTokens, outputTokens int64) map[string]any {
	if strings.TrimSpace(stopReason) == "" {
		stopReason = "end_turn"
	}
	usage := map[string]any{
		"input_tokens":  inputTokens,
		"output_tokens": outputTokens,
	}
	return map[string]any{
		"type": "message_delta",
		"delta": map[string]any{
			"stop_reason":   stopReason,
			"stop_sequence": nil,
		},
		"usage": usage,
	}
}

func BuildMessageStopEvent() map[string]any {
	return map[string]any{
		"type": "message_stop",
	}
}
