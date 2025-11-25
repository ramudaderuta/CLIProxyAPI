// Package chat_completions provides request translation functionality for OpenAI to Kiro API compatibility.
// It converts OpenAI Chat Completions requests into Kiro's conversationState format using gjson/sjson.
package chat_completions

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
	authkiro "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
)

const (
	// Maximum length for tool descriptions before truncation
	maxToolDescriptionLength = 256

	// Tool specification hash prefix
	toolHashPrefix = "tool_"
)

type kiroTurn struct {
	role        string
	content     string
	toolUses    []map[string]any
	toolResults []map[string]any
}

// ConvertOpenAIRequestToKiro converts an OpenAI Chat Completions request (raw JSON)
// into Kiro's conversationState format. All JSON construction uses sjson and lookups use gjson.
//
// Parameters:
//   - modelName: The name of the model to use for the request
//   - inputRawJSON: The raw JSON request data from the OpenAI API
//   - token: The Kiro token storage (unused in minimal version)
//   - metadata: Additional metadata (unused in minimal version)
//
// Returns:
//   - []byte: The transformed request data in Kiro conversationState format
//   - error: An error if the conversion fails
func ConvertOpenAIRequestToKiro(modelName string, inputRawJSON []byte, token *authkiro.KiroTokenStorage, metadata map[string]any) ([]byte, error) {
	// Parse input JSON
	inputJSON := gjson.ParseBytes(inputRawJSON)

	// Extract system prompt - check both Anthropic format (top-level array) and OpenAI format (in messages)
	systemPrompt := extractAnthropicSystemPrompt(inputJSON.Get("system"))
	if systemPrompt == "" {
		// Fallback to OpenAI format (system message in messages array)
		systemPrompt = extractSystemPrompt(inputJSON.Get("messages"))
	}

	// Build tool specifications from tools array
	tools := inputJSON.Get("tools")
	toolSpecs, toolManifest := buildToolSpecifications(tools)

	// Build conversation history from messages (all but the last user message)
	rawMessages := inputJSON.Get("messages")
	if !rawMessages.Exists() || !rawMessages.IsArray() || len(rawMessages.Array()) == 0 {
		rawMessages = inputJSON.Get("input")
	}
	messages := rawMessages.Array()
	history, currentMessage, err := buildHistoryAndCurrentMessage(messages, systemPrompt, toolSpecs, toolManifest)
	if err != nil {
		return nil, err
	}

	convState := map[string]any{
		"chatTriggerType": "MANUAL",
		"model":           modelName,
	}

	if len(history) > 0 {
		convState["history"] = history
	}

	if currentMessage != nil {
		convState["currentMessage"] = currentMessage
	}

	// Attach system prompt to customization prompts for additional compatibility
	if systemPrompt != "" {
		convState["customizationArn"] = ""
		convState["customSystemPrompts"] = []map[string]any{
			{
				"text": map[string]string{"text": systemPrompt},
			},
		}
	}

	if len(toolSpecs) > 0 {
		convState["tools"] = toolSpecs
	}

	if temp := inputJSON.Get("temperature"); temp.Exists() {
		convState["temperature"] = temp.Float()
	}
	if maxTokens := inputJSON.Get("max_tokens"); maxTokens.Exists() {
		convState["maxTokens"] = maxTokens.Int()
	}
	if topP := inputJSON.Get("top_p"); topP.Exists() {
		convState["topP"] = topP.Float()
	}

	// Build final payload with optional metadata from token/metadata
	body := map[string]any{
		"conversationState": convState,
	}

	if token != nil && token.ProfileArn != "" {
		body["profileArn"] = token.ProfileArn
	}
	if projectName := extractProjectName(metadata); projectName != "" {
		body["projectName"] = projectName
	}

	// Build the final Kiro request
	result, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	log.Debugf("Converted OpenAI request to Kiro format")

	return result, nil
}

// extractSystemPrompt extracts the system prompt from messages array
func extractSystemPrompt(messages gjson.Result) string {
	if !messages.Exists() || !messages.IsArray() {
		return ""
	}

	for _, msg := range messages.Array() {
		role := msg.Get("role").String()
		if role == "system" {
			content := msg.Get("content")
			if content.IsArray() {
				// Handle multimodal content
				var textParts []string
				for _, part := range content.Array() {
					if part.Get("type").String() == "text" {
						textParts = append(textParts, part.Get("text").String())
					}
				}
				return strings.Join(textParts, "\n")
			}
			return content.String()
		}
	}
	return ""
}

// extractAnthropicSystemPrompt extracts the system prompt from Anthropic format (top-level system array)
func extractAnthropicSystemPrompt(system gjson.Result) string {
	if !system.Exists() {
		return ""
	}

	// Handle Anthropic format: system is an array of objects with type="text" and text field
	if system.IsArray() {
		var textParts []string
		for _, part := range system.Array() {
			if part.Get("type").String() == "text" {
				text := part.Get("text").String()
				if text != "" {
					textParts = append(textParts, text)
				}
			}
		}
		return strings.Join(textParts, "\n")
	}

	// Handle simple string format (fallback)
	if system.Type == gjson.String {
		return system.String()
	}

	return ""
}

// buildToolSpecifications builds Kiro tool specifications and the corresponding manifest hash list.
func buildToolSpecifications(tools gjson.Result) ([]map[string]any, []map[string]any) {
	if !tools.Exists() || !tools.IsArray() {
		return nil, nil
	}

	var specs []map[string]any
	var manifest []map[string]any

	for _, tool := range tools.Array() {
		toolType := tool.Get("type").String()
		if toolType != "function" {
			continue
		}

		function := tool.Get("function")
		name := function.Get("name").String()
		description := function.Get("description").String()

		// Truncate description if too long
		var descriptionHash string
		if len(description) > maxToolDescriptionLength {
			descriptionHash = generateToolHash(name, description)
			description = description[:maxToolDescriptionLength] + "... [truncated:" + descriptionHash + "]"
		}

		params := function.Get("parameters")

		spec := map[string]any{
			"name":        name,
			"description": description,
		}

		if params.Exists() {
			// Convert parameters to Kiro format
			var paramsMap map[string]any
			if err := json.Unmarshal([]byte(params.Raw), &paramsMap); err == nil {
				spec["inputSchema"] = map[string]any{
					"json": paramsMap,
				}
			}
		}

		specs = append(specs, spec)

		if descriptionHash != "" {
			manifest = append(manifest, map[string]any{
				"name":            name,
				"descriptionHash": descriptionHash,
			})
		}
	}

	if len(specs) == 0 {
		return nil, nil
	}

	return specs, manifest
}

// generateToolHash creates a deterministic hash for tool signature
func generateToolHash(name, description string) string {
	content := name + ":" + description
	hash := sha256.Sum256([]byte(content))
	return toolHashPrefix + hex.EncodeToString(hash[:])[:12]
}

// buildHistoryAndCurrentMessage separates messages into history and current message with Kiro schema.
func buildHistoryAndCurrentMessage(messages []gjson.Result, systemPrompt string, tools []map[string]any, manifest []map[string]any) ([]map[string]any, map[string]any, error) {
	if len(messages) == 0 {
		return nil, nil, errors.New("no messages provided")
	}

	var history []map[string]any
	var currentMessage map[string]any

	// Find last user message index
	lastUserIdx := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Get("role").String() == "user" {
			lastUserIdx = i
			break
		}
	}

	if lastUserIdx == -1 {
		// No user message found, treat all as history
		for _, msg := range messages {
			if converted := convertMessage(msg, tools, manifest, systemPrompt != ""); converted != nil {
				history = append(history, converted)
			}
		}
		return history, nil, nil
	}

	// Build history from messages before last user message
	systemPending := systemPrompt
	for i := 0; i < lastUserIdx; i++ {
		msg := messages[i]
		role := msg.Get("role").String()

		// Skip system messages in history (already extracted)
		if role == "system" {
			continue
		}

		if converted := convertMessage(msg, tools, manifest, systemPending != ""); converted != nil {
			// Prepend system prompt to the first user message
			if systemPending != "" && role == "user" {
				if userMsg, ok := converted["userInputMessage"].(map[string]any); ok {
					content, _ := userMsg["content"].(string)
					if content == "" {
						content = systemPending
					} else {
						content = systemPending + "\n\n" + content
					}
					userMsg["content"] = content
					systemPending = ""
				}
			}
			history = append(history, converted)
		}
	}

	// Set current message as the last user message
	currentMessage = convertMessage(messages[lastUserIdx], tools, manifest, systemPending != "")
	if systemPending != "" {
		if userMsg, ok := currentMessage["userInputMessage"].(map[string]any); ok {
			content, _ := userMsg["content"].(string)
			if content == "" {
				content = systemPrompt
			} else {
				content = systemPrompt + "\n\n" + content
			}
			userMsg["content"] = content
		}
	}
	// Ensure tools/context manifest attached to current message context
	if currentMessage != nil {
		if userMsg, ok := currentMessage["userInputMessage"].(map[string]any); ok {
			ctx, _ := userMsg["userInputMessageContext"].(map[string]any)
			if ctx == nil {
				ctx = map[string]any{}
				userMsg["userInputMessageContext"] = ctx
			}
			if len(tools) > 0 {
				ctx["tools"] = tools
			}
			if len(manifest) > 0 {
				ctx["toolContextManifest"] = manifest
			}
		}
	}

	// If there are messages after the last user message, append them to history
	for i := lastUserIdx + 1; i < len(messages); i++ {
		if converted := convertMessage(messages[i], tools, manifest, false); converted != nil {
			history = append(history, converted)
		}
	}

	return history, currentMessage, nil
}

// convertMessage converts an OpenAI message to Kiro format.
func convertMessage(msg gjson.Result, tools []map[string]any, manifest []map[string]any, applyFallbackContent bool) map[string]any {
	role := msg.Get("role").String()
	turn := parseTurn(msg)

	// Ensure we always send non-empty content
	if turn.content == "" && (len(turn.toolUses) > 0 || len(turn.toolResults) > 0 || applyFallbackContent) {
		turn.content = "."
	}

	switch role {
	case "assistant":
		if len(turn.toolUses) == 0 && turn.content == "" {
			return nil
		}

		assistant := map[string]any{
			"assistantResponseMessage": map[string]any{
				"content": turn.content,
			},
			"role":    "assistant",
			"content": turn.content,
		}

		if len(turn.toolUses) > 0 {
			assistantMsg := assistant["assistantResponseMessage"].(map[string]any)
			assistantMsg["toolUses"] = turn.toolUses
		}
		return assistant

	case "tool":
		ctx := map[string]any{}
		if len(turn.toolResults) > 0 {
			ctx["toolResults"] = turn.toolResults
		}
		user := map[string]any{
			"userInputMessage": map[string]any{
				"content":                 turn.content,
				"userInputMessageContext": ctx,
			},
			"role":    "user",
			"content": turn.content,
		}
		return user

	default:
		ctx := map[string]any{}
		if len(turn.toolResults) > 0 {
			ctx["toolResults"] = turn.toolResults
		}
		if len(tools) > 0 {
			ctx["tools"] = tools
		}
		if len(manifest) > 0 {
			ctx["toolContextManifest"] = manifest
		}
		user := map[string]any{
			"userInputMessage": map[string]any{
				"content":                 turn.content,
				"userInputMessageContext": ctx,
			},
			"role":    "user",
			"content": turn.content,
		}
		return user
	}
}

// parseTurn extracts text/tool uses/results from a message.
func parseTurn(msg gjson.Result) kiroTurn {
	role := msg.Get("role").String()
	turn := kiroTurn{role: role}

	content := msg.Get("content")
	switch {
	case content.IsArray():
		for _, part := range content.Array() {
			partType := part.Get("type").String()
			switch partType {
			case "text":
				if text := part.Get("text").String(); text != "" {
					turn.content += text
				}
			case "tool_use":
				turn.toolUses = append(turn.toolUses, buildToolUse(part))
			case "tool_result":
				if result := buildToolResult(part); result != nil {
					turn.toolResults = append(turn.toolResults, result)
				}
			case "image_url":
				// Preserve a lightweight marker for image content
				if url := part.Get("image_url.url").String(); url != "" {
					if turn.content != "" {
						turn.content += "\n"
					}
					turn.content += "[image:" + url + "]"
				}
			}
		}
	default:
		turn.content = content.String()
	}

	// OpenAI tool_calls on assistant role
	if role == "assistant" {
		toolCalls := msg.Get("tool_calls")
		if toolCalls.Exists() && toolCalls.IsArray() {
			for _, tc := range toolCalls.Array() {
				if tc.Get("type").String() != "function" {
					continue
				}
				input := tc.Get("function.arguments").String()
				var args map[string]any
				if input != "" {
					_ = json.Unmarshal([]byte(input), &args)
				}
				turn.toolUses = append(turn.toolUses, map[string]any{
					"toolUseId": sanitizeToolCallID(tc.Get("id").String()),
					"name":      tc.Get("function.name").String(),
					"input":     args,
				})
			}
		}
	}

	// Tool role content becomes tool result entries
	if role == "tool" {
		if result := buildToolResult(msg); result != nil {
			turn.toolResults = append(turn.toolResults, result)
		}
	}

	return turn
}

func buildToolUse(part gjson.Result) map[string]any {
	input := map[string]any{}
	if raw := part.Get("input").Raw; raw != "" {
		_ = json.Unmarshal([]byte(raw), &input)
	}
	toolUseID := sanitizeToolCallID(part.Get("id").String())
	if toolUseID == "" {
		toolUseID = sanitizeToolCallID(part.Get("tool_call_id").String())
	}
	return map[string]any{
		"toolUseId": toolUseID,
		"name":      part.Get("name").String(),
		"input":     input,
	}
}

func buildToolResult(part gjson.Result) map[string]any {
	toolUseID := part.Get("tool_call_id").String()
	if toolUseID == "" {
		toolUseID = part.Get("tool_use_id").String()
	}
	if toolUseID == "" {
		return nil
	}

	text := part.Get("content").String()
	if text == "" && part.Get("text").Exists() {
		text = part.Get("text").String()
	}

	return map[string]any{
		"toolUseId": toolUseID,
		"status":    "success",
		"content": []map[string]any{
			{
				"text": text,
				"type": "text",
			},
		},
	}
}

func sanitizeToolCallID(id string) string {
	id = strings.TrimSpace(id)
	if id != "" {
		return id
	}
	return "call_" + uuid.New().String()[:12]
}

func extractProjectName(metadata map[string]any) string {
	if metadata == nil {
		return ""
	}
	if project, ok := metadata["projectName"].(string); ok {
		return project
	}
	return ""
}
