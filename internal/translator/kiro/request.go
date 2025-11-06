package kiro

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	authkiro "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
	"github.com/tidwall/gjson"
)

const (
	chatTrigger = "MANUAL"
	origin      = "AI_EDITOR"
)

// BuildRequest converts an OpenAI-compatible chat payload into Kiro's conversation request format.
func BuildRequest(model string, payload []byte, token *authkiro.KiroTokenStorage, metadata map[string]any) ([]byte, error) {
	if token == nil {
		return nil, fmt.Errorf("kiro translator: token storage missing")
	}
	root := gjson.ParseBytes(payload)
	messages := root.Get("messages")
	if !messages.Exists() || !messages.IsArray() || len(messages.Array()) == 0 {
		return nil, fmt.Errorf("kiro translator: messages array is required")
	}

	systemPrompt := strings.TrimSpace(root.Get("system").String())
	tools := root.Get("tools")
	kiroModel := MapModel(model)

	history := make([]map[string]any, 0, len(messages.Array()))
	startIndex := 0

	if systemPrompt != "" {
		first := messages.Array()[0]
		if strings.EqualFold(first.Get("role").String(), "user") {
			text, _, _, _ := extractUserMessage(first)
			content := combineContent(systemPrompt, text)
			history = append(history, wrapUserMessage(content, kiroModel, nil, nil, nil, nil))
			startIndex = 1
		} else {
			history = append(history, wrapUserMessage(systemPrompt, kiroModel, nil, nil, nil, nil))
		}
	}

	for i := startIndex; i < len(messages.Array())-1; i++ {
		msg := messages.Array()[i]
		role := strings.ToLower(strings.TrimSpace(msg.Get("role").String()))
		switch role {
		case "assistant":
			text, toolUses := extractAssistantMessage(msg)
			history = append(history, wrapAssistantMessage(text, toolUses))
		case "user", "system", "tool":
			text, toolResults, toolUses, images := extractUserMessage(msg)
			history = append(history, wrapUserMessage(text, kiroModel, toolResults, toolUses, images, nil))
		}
	}

	current := messages.Array()[len(messages.Array())-1]
	currentRole := strings.ToLower(strings.TrimSpace(current.Get("role").String()))
	var currentPayload map[string]any
	if currentRole == "assistant" {
		text, toolUses := extractAssistantMessage(current)
		currentPayload = map[string]any{
			"assistantResponseMessage": map[string]any{
				"content":  text,
				"toolUses": toolUses,
			},
		}
	} else {
		text, toolResults, toolUses, images := extractUserMessage(current)
		context := map[string]any{}
		if len(toolResults) > 0 {
			context["toolResults"] = toolResults
		}
		if toolDefinitions := buildToolSpecifications(tools); len(toolDefinitions) > 0 {
			context["tools"] = toolDefinitions
		}
		if len(context) == 0 {
			context = nil
		}

		currentPayload = map[string]any{
			"userInputMessage": map[string]any{
				"content": text,
				"modelId": kiroModel,
				"origin":  origin,
			},
		}
		if len(images) > 0 {
			currentPayload["userInputMessage"].(map[string]any)["images"] = images
		}
		if context != nil {
			currentPayload["userInputMessage"].(map[string]any)["userInputMessageContext"] = context
		}
		if len(toolUses) > 0 {
			currentPayload["userInputMessage"].(map[string]any)["toolUses"] = toolUses
		}
	}

	request := map[string]any{
		"conversationState": map[string]any{
			"chatTriggerType": chatTrigger,
			"conversationId":  uuid.NewString(),
			"currentMessage":  currentPayload,
			"history":         history,
		},
	}
	if strings.EqualFold(token.AuthMethod, "social") && token.ProfileArn != "" {
		request["profileArn"] = token.ProfileArn
	}
	if project, ok := metadata["project"].(string); ok && project != "" {
		request["projectName"] = project
	}

	return json.Marshal(request)
}

func wrapUserMessage(content, model string, toolResults, toolUses, images []map[string]any, tools []map[string]any) map[string]any {
	payload := map[string]any{
		"userInputMessage": map[string]any{
			"content": content,
			"modelId": model,
			"origin":  origin,
		},
	}
	context := map[string]any{}
	if len(toolResults) > 0 {
		context["toolResults"] = toolResults
	}
	if len(tools) > 0 {
		context["tools"] = tools
	}
	if len(context) > 0 {
		payload["userInputMessage"].(map[string]any)["userInputMessageContext"] = context
	}
	if len(images) > 0 {
		payload["userInputMessage"].(map[string]any)["images"] = images
	}
	if len(toolUses) > 0 {
		payload["userInputMessage"].(map[string]any)["toolUses"] = toolUses
	}
	return payload
}

func wrapAssistantMessage(content string, toolUses []map[string]any) map[string]any {
	payload := map[string]any{
		"assistantResponseMessage": map[string]any{
			"content": content,
		},
	}
	if len(toolUses) > 0 {
		payload["assistantResponseMessage"].(map[string]any)["toolUses"] = toolUses
	}
	return payload
}

func extractUserMessage(msg gjson.Result) (string, []map[string]any, []map[string]any, []map[string]any) {
	content := msg.Get("content")
	textParts := make([]string, 0, 4)
	toolResults := make([]map[string]any, 0)
	toolUses := make([]map[string]any, 0)
	images := make([]map[string]any, 0)

	if content.Type == gjson.String {
		textParts = append(textParts, content.String())
	} else if content.IsArray() {
		content.ForEach(func(_, part gjson.Result) bool {
			switch strings.ToLower(part.Get("type").String()) {
			case "text", "input_text", "output_text":
				textParts = append(textParts, part.Get("text").String())
			case "tool_result":
				resultContent := extractNestedContent(part.Get("content"))
				// Remove incorrect fallback to non-existent "text" field
				// Tool results use content field, not text field
				toolUseId := SanitizeToolCallID(firstString(
					part.Get("tool_use_id").String(),
					part.Get("tool_useId").String(),
				))
				// Always create tool result entry, even with empty content
				toolResults = append(toolResults, map[string]any{
					"content": []map[string]string{{"text": resultContent}},
					"status":  firstString(part.Get("status").String(), "success"),
					"toolUseId": toolUseId,
				})
			case "tool_use":
				toolUses = append(toolUses, map[string]any{
					"name":      part.Get("name").String(),
					"toolUseId": SanitizeToolCallID(firstString(part.Get("id").String(), part.Get("tool_use_id").String())),
					"input":     parseJSONSafely(part.Get("input"), part.Get("arguments")),
				})
			case "image", "input_image":
				if img := buildImagePart(part); img != nil {
					images = append(images, img)
				}
			}
			return true
		})
	} else if content.Exists() {
		textParts = append(textParts, content.String())
	}
	return strings.TrimSpace(strings.Join(textParts, "\n")), toolResults, toolUses, images
}

func extractAssistantMessage(msg gjson.Result) (string, []map[string]any) {
	content := msg.Get("content")
	textParts := make([]string, 0, 4)
	toolUses := make([]map[string]any, 0)

	if content.Type == gjson.String {
		textParts = append(textParts, content.String())
	} else if content.IsArray() {
		content.ForEach(func(_, part gjson.Result) bool {
			switch strings.ToLower(part.Get("type").String()) {
			case "text", "output_text":
				textParts = append(textParts, part.Get("text").String())
			case "tool_use":
				toolUses = append(toolUses, map[string]any{
					"name":      part.Get("name").String(),
					"toolUseId": SanitizeToolCallID(firstString(part.Get("id").String(), part.Get("tool_use_id").String())),
					"input":     parseJSONSafely(part.Get("input"), part.Get("arguments")),
				})
			}
			return true
		})
	} else if content.Exists() {
		textParts = append(textParts, content.String())
	}
	return strings.TrimSpace(strings.Join(textParts, "\n")), toolUses
}

func buildToolSpecifications(tools gjson.Result) []map[string]any {
	if !tools.Exists() || !tools.IsArray() {
		return nil
	}
	specs := make([]map[string]any, 0, len(tools.Array()))
	tools.ForEach(func(_, tool gjson.Result) bool {
		if strings.ToLower(tool.Get("type").String()) != "function" {
			return true
		}
		function := tool.Get("function")
		if !function.Exists() {
			return true
		}
		schema := parseJSONSafely(function.Get("parameters"), gjson.Result{})
		if schema == nil {
			schema = map[string]any{}
		}
		entry := map[string]any{
			"toolSpecification": map[string]any{
				"name":        function.Get("name").String(),
				"description": function.Get("description").String(),
				"inputSchema": map[string]any{"json": schema},
			},
		}
		specs = append(specs, entry)
		return true
	})
	return specs
}

func buildImagePart(part gjson.Result) map[string]any {
	if source := part.Get("source"); source.Exists() {
		mediaType := source.Get("media_type").String()
		format := ""
		if idx := strings.Index(mediaType, "/"); idx != -1 && idx+1 < len(mediaType) {
			format = mediaType[idx+1:]
		}
		data := source.Get("data").String()
		if format == "" || data == "" {
			return nil
		}
		return map[string]any{
			"format": format,
			"source": map[string]any{"bytes": data},
		}
	}
	return nil
}

func extractNestedContent(value gjson.Result) string {
	if !value.Exists() {
		return ""
	}
	if value.Type == gjson.String {
		return value.String()
	}
	if value.IsArray() {
		parts := make([]string, 0, len(value.Array()))
		value.ForEach(func(_, part gjson.Result) bool {
			if part.Type == gjson.String {
				parts = append(parts, part.String())
			} else if part.Get("text").Exists() {
				parts = append(parts, part.Get("text").String())
			}
			return true
		})
		return strings.Join(parts, "")
	}
	return value.String()
}

func combineContent(parts ...string) string {
	acc := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			acc = append(acc, trimmed)
		}
	}
	return strings.Join(acc, "\n\n")
}

func parseJSONSafely(primary, fallback gjson.Result) any {
	if primary.Exists() && primary.Raw != "" {
		var obj any
		if err := json.Unmarshal([]byte(primary.Raw), &obj); err == nil {
			return obj
		}
	}
	if fallback.Exists() && fallback.Raw != "" {
		var obj any
		if err := json.Unmarshal([]byte(fallback.Raw), &obj); err == nil {
			return obj
		}
	}
	return nil
}

func firstString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// ValidateToolCallID checks if a tool_call_id is in a valid format
// Valid formats should not contain colons or triple-asterisk patterns
func ValidateToolCallID(id string) bool {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return false
	}
	// Reject IDs with colons (like "***.TodoWrite:3")
	if strings.Contains(trimmed, ":") {
		return false
	}
	// Reject IDs with triple-asterisk patterns
	if strings.Contains(trimmed, "***") {
		return false
	}
	return true
}

// SanitizeToolCallID ensures a tool_call_id is valid
// If invalid, generates a new valid UUID; otherwise returns the original
func SanitizeToolCallID(id string) string {
	if ValidateToolCallID(id) {
		return id
	}
	// Generate a new valid UUID for invalid IDs
	return "call_" + uuid.New().String()
}
