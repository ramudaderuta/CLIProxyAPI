package claude

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/google/uuid"
	authkiro "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro/helpers"
	"github.com/tidwall/gjson"
)

const (
	chatTrigger              = "MANUAL"
	origin                   = "AI_EDITOR"
	maxToolDescriptionLength = 256
)

type toolContextEntry struct {
	Name        string
	Description string
	Hash        string
	Length      int
}

// mergeAdjacentSameRole collapses consecutive messages with the same role into a single message.
// This mirrors the defensive fix from AIClient-2-API (commit 1e724c2) to reduce fragmented
// transcripts emitted by some clients.
func mergeAdjacentSameRole(messages []gjson.Result) []gjson.Result {
	merged := make([]gjson.Result, 0, len(messages))
	for _, msg := range messages {
		if len(merged) == 0 {
			merged = append(merged, msg)
			continue
		}

		prev := merged[len(merged)-1]
		prevRole := strings.ToLower(strings.TrimSpace(prev.Get("role").String()))
		currRole := strings.ToLower(strings.TrimSpace(msg.Get("role").String()))
		if prevRole != "assistant" && prevRole != "user" {
			merged = append(merged, msg)
			continue
		}
		if prevRole != currRole {
			merged = append(merged, msg)
			continue
		}

		prevMap, ok1 := prev.Value().(map[string]any)
		currMap, ok2 := msg.Value().(map[string]any)
		if !ok1 || !ok2 {
			merged = append(merged, msg)
			continue
		}

		if hasToolResult(prevMap["content"]) || hasToolResult(currMap["content"]) {
			merged = append(merged, msg)
			continue
		}

		prevMap = mergeMessageContent(prevMap, currMap)
		bytes, err := json.Marshal(prevMap)
		if err != nil {
			merged = append(merged, msg)
			continue
		}
		merged[len(merged)-1] = gjson.ParseBytes(bytes)
	}
	return merged
}

func mergeMessageContent(prev, curr map[string]any) map[string]any {
	prevContent := toContentArray(prev["content"])
	currContent := toContentArray(curr["content"])
	prev["content"] = append(prevContent, currContent...)
	return prev
}

func hasToolResult(raw any) bool {
	content := toContentArray(raw)
	for _, item := range content {
		if obj, ok := item.(map[string]any); ok {
			typ := strings.ToLower(strings.TrimSpace(fmt.Sprint(obj["type"])))
			if typ == "tool_result" {
				return true
			}
		}
	}
	return false
}

// toContentArray normalizes a content field into []any of structured blocks.
func toContentArray(raw any) []any {
	switch v := raw.(type) {
	case []any:
		return v
	case string:
		return []any{map[string]any{"type": "text", "text": v}}
	default:
		return []any{map[string]any{"type": "text", "text": fmt.Sprint(v)}}
	}
}

func isToolResultOnlyMessage(msg gjson.Result) bool {
	content := msg.Get("content")
	if !content.Exists() {
		return false
	}
	if content.Type == gjson.String {
		return strings.TrimSpace(content.String()) == ""
	}
	if !content.IsArray() {
		return false
	}
	hasUserFacing := false
	hasToolResult := false
	content.ForEach(func(_, part gjson.Result) bool {
		typ := strings.ToLower(strings.TrimSpace(part.Get("type").String()))
		switch typ {
		case "text", "input_text", "output_text":
			if strings.TrimSpace(part.Get("text").String()) != "" {
				hasUserFacing = true
			}
		case "tool_result":
			hasToolResult = true
		case "image", "input_image":
			hasUserFacing = true
		default:
			if part.Get("text").Exists() && strings.TrimSpace(part.Get("text").String()) != "" {
				hasUserFacing = true
			}
		}
		return true
	})
	return hasToolResult && !hasUserFacing
}

// BuildRequest converts an OpenAI-compatible chat payload into Kiro's conversation request format.
func BuildRequest(model string, payload []byte, token *authkiro.KiroTokenStorage, metadata map[string]any) ([]byte, error) {
	if token == nil {
		return nil, fmt.Errorf("kiro translator: token storage missing")
	}
	root := gjson.ParseBytes(payload)
	messages := root.Get("messages")
	if !messages.Exists() || !messages.IsArray() {
		return nil, fmt.Errorf("kiro translator: messages array is required")
	}
	messageArray := mergeAdjacentSameRole(messages.Array())

	// Defensive fix mirroring AIClient-2-API commit 1e724c2: drop a trailing assistant
	// message whose only content is a single "{" text block (buggy npm Claude Code client
	// sometimes emits this). This prevents Kiro from receiving a malformed current turn.
	if len(messageArray) > 0 {
		last := messageArray[len(messageArray)-1]
		if strings.EqualFold(last.Get("role").String(), "assistant") {
			content := last.Get("content")
			if content.IsArray() && len(content.Array()) == 1 {
				first := content.Array()[0]
				typ := strings.ToLower(strings.TrimSpace(first.Get("type").String()))
				text := strings.TrimSpace(first.Get("text").String())
				if typ == "text" && text == "{" {
					messageArray = messageArray[:len(messageArray)-1]
				}
			}
		}
	}
	if len(messageArray) == 0 {
		return nil, fmt.Errorf("kiro translator: messages array is required")
	}

	systemPrompt := extractSystemPrompt(root.Get("system"))
	tools := root.Get("tools")
	toolDefinitions, toolContextEntries := buildToolSpecifications(tools)
	toolChoiceMeta, toolChoiceDirective := buildToolChoiceMetadata(root.Get("tool_choice"))
	planModeMeta, planDirective := buildPlanModeMetadata(messages, tools)

	if block := buildToolContextBlock(toolContextEntries); block != "" {
		systemPrompt = combineContent(systemPrompt, block)
	}
	if toolChoiceDirective != "" {
		systemPrompt = combineContent(systemPrompt, toolChoiceDirective)
	}
	if planDirective != "" {
		systemPrompt = combineContent(systemPrompt, planDirective)
	}

	// If no explicit tools are provided but the transcript contains tool_use blocks,
	// synthesize minimal tool specifications so Kiro accepts referenced tools.
	if len(toolDefinitions) == 0 {
		if inferred := collectToolUseNames(messages); len(inferred) > 0 {
			toolDefinitions = buildSyntheticToolSpecifications(inferred)
		}
	}

	kiroModel := helpers.MapModel(model)

	history := make([]map[string]any, 0, len(messageArray))
	startIndex := 0

	if systemPrompt != "" {
		first := messageArray[0]
		firstIsUser := strings.EqualFold(first.Get("role").String(), "user")
		if firstIsUser && len(messageArray) > 1 {
			// Preserve tool events; merge system prompt into first user content
			text, toolResults, toolUses, images := extractUserMessage(first)
			content := combineContent(systemPrompt, text)
			history = append(history, wrapUserMessage(content, kiroModel, toolResults, toolUses, images, nil))
			startIndex = 1
		} else {
			history = append(history, wrapUserMessage(systemPrompt, kiroModel, nil, nil, nil, nil))
		}
	}

	currentIndex := len(messageArray) - 1

	// First move trailing user tool_result-only messages out of the active turn.
	// This ensures Kiro never receives a tool_result as the current user entry.
	movedToolResultHistory := make([]map[string]any, 0)
	for currentIndex >= startIndex {
		msg := messageArray[currentIndex]
		role := strings.ToLower(strings.TrimSpace(msg.Get("role").String()))
		if role != "user" {
			break
		}
		if !isToolResultOnlyMessage(msg) {
			break
		}
		text, toolResults, toolUses, images := extractUserMessage(msg)
		movedToolResultHistory = append([]map[string]any{
			wrapUserMessage(text, kiroModel, toolResults, toolUses, images, nil),
		}, movedToolResultHistory...)
		currentIndex--
	}
	hadTrailingUserToolResults := len(movedToolResultHistory) > 0

	// Collect trailing assistant tool_uses (messages after the final user turn)
	// and move them to the upcoming current user turn. Always retain a text-only
	// assistant placeholder in history.
	pendingToolUses := make([]map[string]any, 0)
	trailingAssistantTexts := make([]map[string]any, 0)
	if !hadTrailingUserToolResults {
		for currentIndex >= 0 {
			msg := messageArray[currentIndex]
			role := strings.ToLower(strings.TrimSpace(msg.Get("role").String()))
			if role != "assistant" {
				break
			}
			text, toolUses := extractAssistantMessage(msg)
			if len(toolUses) > 0 {
				pendingToolUses = append(toolUses, pendingToolUses...)
			}
			trailingAssistantTexts = append([]map[string]any{wrapAssistantMessage(text, nil)}, trailingAssistantTexts...)
			currentIndex--
		}
	}
	if currentIndex < 0 {
		return nil, fmt.Errorf("kiro translator: no user turn found to forward to kiro")
	}

	syntheticCurrent := hadTrailingUserToolResults || currentIndex < startIndex

	// Build history preserving tool events and media
	historyLimit := currentIndex
	if syntheticCurrent {
		historyLimit++
	}
	if historyLimit < startIndex {
		historyLimit = startIndex
	}
	if historyLimit > len(messageArray) {
		historyLimit = len(messageArray)
	}
	for i := startIndex; i < historyLimit; i++ {
		msg := messageArray[i]
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

	if !syntheticCurrent {
		if currentIndex < 0 {
			syntheticCurrent = true
		} else {
			currentMsgCheck := messageArray[currentIndex]
			currentRole := strings.ToLower(strings.TrimSpace(currentMsgCheck.Get("role").String()))
			if currentRole != "user" {
				switch currentRole {
				case "assistant":
					text, toolUses := extractAssistantMessage(currentMsgCheck)
					history = append(history, wrapAssistantMessage(text, toolUses))
				case "system", "tool":
					text, toolResults, toolUses, images := extractUserMessage(currentMsgCheck)
					history = append(history, wrapUserMessage(text, kiroModel, toolResults, toolUses, images, nil))
				}
				syntheticCurrent = true
			}
		}
	}

	// Place moved tool_result entries before assistant placeholders
	// so that each tool_result is immediately followed by a non-tool assistant turn.
	if len(movedToolResultHistory) > 0 {
		history = append(history, movedToolResultHistory...)
	}
	if len(trailingAssistantTexts) > 0 {
		history = append(history, trailingAssistantTexts...)
	}

	var (
		text        string
		toolResults []map[string]any
		toolUses    []map[string]any
		images      []map[string]any
	)

	if syntheticCurrent {
		text = "."
	} else {
		current := messageArray[currentIndex]
		text, toolResults, toolUses, images = extractUserMessage(current)
		if strings.TrimSpace(text) == "" && len(toolResults) > 0 {
			if collapsed := collapseToolResultContent(toolResults); collapsed != "" {
				text = collapsed
			}
		}
		if strings.TrimSpace(text) == "" {
			text = "."
		}
	}
	// Attach any collected trailing assistant tool_uses to the current user turn
	// to avoid leaving orphan assistant tool_use messages without a corresponding tool_result in history.
	if len(pendingToolUses) > 0 {
		toolUses = append(pendingToolUses, toolUses...)
	}
	context := map[string]any{}
	if len(toolResults) > 0 {
		context["toolResults"] = toolResults
	}
	if len(toolDefinitions) > 0 {
		context["tools"] = toolDefinitions
	}
	if manifest := buildToolContextManifest(toolContextEntries); len(manifest) > 0 {
		context["toolContextManifest"] = manifest
	}
	if toolChoiceMeta != nil {
		context["claudeToolChoice"] = toolChoiceMeta
	}
	if planModeMeta != nil {
		context["planMode"] = planModeMeta
	}
	if len(context) == 0 {
		context = nil
	}

	currentPayload := map[string]any{
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
	// Kiro is stricter about non-empty message content. If this user entry only
	// carries structured toolResults and no visible text, inject a minimal placeholder.
	if strings.TrimSpace(content) == "" && len(toolResults) > 0 {
		content = "."
	}
	payload := map[string]any{
		"userInputMessage": map[string]any{
			"content": content,
			"modelId": model,
			"origin":  origin,
		},
	}
	if len(toolResults) > 0 {
		context := map[string]any{
			"toolResults": toolResults,
		}
		if len(tools) > 0 {
			context["tools"] = tools
		}
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
	if strings.TrimSpace(content) == "" {
		// Kiro requires assistant content to be non-empty.
		// If the assistant turn only contains tool_use blocks (no text),
		// insert a minimal placeholder so the message is accepted.
		content = "."
	}
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
				toolUseId := helpers.SanitizeToolCallID(helpers.FirstString(
					part.Get("tool_use_id").String(),
					part.Get("tool_useId").String(),
				))
				// Always create tool result entry, even with empty content
				toolResults = append(toolResults, map[string]any{
					"content":   []map[string]string{{"text": resultContent}},
					"status":    helpers.FirstString(part.Get("status").String(), "success"),
					"toolUseId": toolUseId,
				})
			case "tool_use":
				toolUses = append(toolUses, map[string]any{
					"name":      part.Get("name").String(),
					"toolUseId": helpers.SanitizeToolCallID(helpers.FirstString(part.Get("id").String(), part.Get("tool_use_id").String())),
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
	return sanitizeAnthropicText(strings.Join(textParts, "\n")), toolResults, toolUses, images
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
					"toolUseId": helpers.SanitizeToolCallID(helpers.FirstString(part.Get("id").String(), part.Get("tool_use_id").String())),
					"input":     parseJSONSafely(part.Get("input"), part.Get("arguments")),
				})
			}
			return true
		})
	} else if content.Exists() {
		textParts = append(textParts, content.String())
	}
	return sanitizeAnthropicText(strings.Join(textParts, "\n")), toolUses
}

func buildToolSpecifications(tools gjson.Result) ([]map[string]any, []toolContextEntry) {
	if !tools.Exists() || !tools.IsArray() {
		return nil, nil
	}
	specs := make([]map[string]any, 0, len(tools.Array()))
	contexts := make([]toolContextEntry, 0, len(tools.Array()))
	tools.ForEach(func(_, tool gjson.Result) bool {
		var name, description string
		var schema map[string]any

		// Handle OpenAI format: {"type": "function", "function": {...}}
		if strings.ToLower(tool.Get("type").String()) == "function" {
			function := tool.Get("function")
			if !function.Exists() {
				return true
			}
			name = function.Get("name").String()
			description = function.Get("description").String()
			schemaRaw := parseJSONSafely(function.Get("parameters"), gjson.Result{})
			if schemaRaw != nil {
				if schemaMap, ok := schemaRaw.(map[string]any); ok {
					schema = schemaMap
				}
			}
		} else {
			// Handle Anthropic/Claude format: {"name": "...", "description": "...", "input_schema": {...}}
			name = tool.Get("name").String()
			description = tool.Get("description").String()
			schemaRaw := parseJSONSafely(tool.Get("input_schema"), gjson.Result{})
			if schemaRaw != nil {
				if schemaMap, ok := schemaRaw.(map[string]any); ok {
					schema = schemaMap
				}
			}
		}

		name = sanitizeTextContent(name)
		shortDescription, fullDescription, truncated := sanitizeToolDescription(name, description)
		if truncated {
			hash, length := hashToolDescription(fullDescription)
			contexts = append(contexts, toolContextEntry{
				Name:        name,
				Description: fullDescription,
				Hash:        hash,
				Length:      length,
			})
		}

		if name == "" {
			return true
		}

		if schema == nil {
			schema = map[string]any{}
		}

		entry := map[string]any{
			"toolSpecification": map[string]any{
				"name":        name,
				"description": shortDescription,
				"inputSchema": map[string]any{"json": schema},
			},
		}
		specs = append(specs, entry)
		return true
	})
	return specs, contexts
}

// collectToolUseNames scans messages for assistant tool_use entries and returns
// a sorted unique list of tool names referenced.
func collectToolUseNames(messages gjson.Result) []string {
	if !messages.Exists() || !messages.IsArray() {
		return nil
	}
	seen := make(map[string]struct{})
	names := make([]string, 0)
	messages.ForEach(func(_, msg gjson.Result) bool {
		content := msg.Get("content")
		if content.IsArray() {
			content.ForEach(func(_, part gjson.Result) bool {
				if strings.EqualFold(strings.TrimSpace(part.Get("type").String()), "tool_use") {
					name := strings.TrimSpace(part.Get("name").String())
					if name != "" {
						key := strings.ToLower(name)
						if _, ok := seen[key]; !ok {
							seen[key] = struct{}{}
							names = append(names, name)
						}
					}
				}
				return true
			})
		}
		return true
	})
	sort.Slice(names, func(i, j int) bool { return strings.ToLower(names[i]) < strings.ToLower(names[j]) })
	return names
}

// buildSyntheticToolSpecifications constructs minimal tool specifications for the given tool names.
// It uses a generic object schema and a short placeholder description.
func buildSyntheticToolSpecifications(toolNames []string) []map[string]any {
	if len(toolNames) == 0 {
		return nil
	}
	specs := make([]map[string]any, 0, len(toolNames))
	for _, name := range toolNames {
		cleanName := sanitizeTextContent(name)
		if cleanName == "" {
			continue
		}
		desc := fmt.Sprintf("Tool %s", cleanName)
		desc, _, _ = sanitizeToolDescription(cleanName, desc)
		entry := map[string]any{
			"toolSpecification": map[string]any{
				"name":        cleanName,
				"description": desc,
				"inputSchema": map[string]any{"json": map[string]any{"type": "object"}},
			},
		}
		specs = append(specs, entry)
	}
	return specs
}

func sanitizeToolDescription(name, desc string) (string, string, bool) {
	desc = sanitizeTextContent(desc)
	desc = stripAngleBracketBlocks(desc)
	desc = collapseSpaces(desc)
	if desc == "" {
		desc = fmt.Sprintf("Tool %s", name)
	}
	full := desc
	truncated := false
	runes := []rune(desc)
	if len(runes) > maxToolDescriptionLength {
		desc = string(runes[:maxToolDescriptionLength])
		truncated = true
	}
	return desc, full, truncated
}

func hashToolDescription(desc string) (string, int) {
	sum := sha256.Sum256([]byte(desc))
	return hex.EncodeToString(sum[:8]), len([]rune(desc))
}

var angleBracketPattern = regexp.MustCompile(`(?s)<[^>]+>`)

func stripAngleBracketBlocks(text string) string {
	return angleBracketPattern.ReplaceAllString(text, "")
}

func collapseSpaces(text string) string {
	fields := strings.Fields(text)
	return strings.Join(fields, " ")
}

func normalizeToolKey(name string) string {
	key := strings.ToLower(name)
	key = strings.ReplaceAll(key, " ", "")
	key = strings.ReplaceAll(key, "-", "")
	key = strings.ReplaceAll(key, "_", "")
	return key
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

func extractSystemPrompt(system gjson.Result) string {
	if !system.Exists() {
		return ""
	}

	switch {
	case system.Type == gjson.String:
		return strings.TrimSpace(system.String())
	case system.IsArray():
		parts := make([]string, 0, len(system.Array()))
		system.ForEach(func(_, part gjson.Result) bool {
			if text := extractSystemPrompt(part); text != "" {
				parts = append(parts, text)
			}
			return true
		})
		return sanitizeAnthropicText(strings.TrimSpace(strings.Join(parts, "\n\n")))
	case system.IsObject():
		if text := strings.TrimSpace(system.Get("text").String()); text != "" {
			return sanitizeAnthropicText(text)
		}
		if content := system.Get("content"); content.Exists() {
			if nested := strings.TrimSpace(extractNestedContent(content)); nested != "" {
				return sanitizeAnthropicText(nested)
			}
		}
		return sanitizeAnthropicText(strings.TrimSpace(system.String()))
	default:
		return sanitizeAnthropicText(strings.TrimSpace(system.String()))
	}
}

func combineContent(parts ...string) string {
	acc := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := sanitizeAnthropicText(part); trimmed != "" {
			acc = append(acc, trimmed)
		}
	}
	return strings.Join(acc, "\n\n")
}

// safeParseJSON robustly parses JSON-ish strings that may contain truncated escape sequences.
// It handles dangling backslashes, incomplete Unicode escapes, and other malformed JSON.
// On failure, it returns the original string rather than failing hard.
func safeParseJSON(raw string) any {
	if raw == "" {
		return ""
	}
	cleaned := raw

	// Handle dangling backslash (e.g. truncated JSON)
	if strings.HasSuffix(cleaned, `\`) && !strings.HasSuffix(cleaned, `\\`) {
		cleaned = cleaned[:len(cleaned)-1]
	} else if strings.HasSuffix(cleaned, `\u`) ||
		strings.HasSuffix(cleaned, `\u0`) ||
		strings.HasSuffix(cleaned, `\u00`) {
		// Remove incomplete Unicode escape at the end
		idx := strings.LastIndex(cleaned, `\u`)
		if idx >= 0 {
			cleaned = cleaned[:idx]
		}
	}

	var out any
	if err := json.Unmarshal([]byte(cleaned), &out); err != nil {
		// On failure, return original string rather than failing hard
		return raw
	}
	if out == nil {
		return map[string]any{}
	}
	return out
}

func parseJSONSafely(primary, fallback gjson.Result) any {
	if primary.Exists() && primary.Raw != "" {
		return safeParseJSON(primary.Raw)
	}
	if fallback.Exists() && fallback.Raw != "" {
		return safeParseJSON(fallback.Raw)
	}
	return nil
}

func sanitizeTextContent(text string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	var builder strings.Builder
	builder.Grow(len(text))
	for _, r := range text {
		switch {
		case r == '\r':
			continue
		case r == '\n', r == '\t':
			builder.WriteRune(r)
		case unicode.IsControl(r):
			continue
		default:
			builder.WriteRune(r)
		}
	}
	return strings.TrimSpace(builder.String())
}

// sanitizeAnthropicText strips Anthropic protocol markers and control noise, then applies
// the generic text sanitizer. Specifically, it removes "<system-reminder>" tokens/lines.
func sanitizeAnthropicText(text string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	// Remove Anthropic protocol reminder tags
	text = stripSystemReminders(text)
	// Finally, collapse control characters/newlines via the generic sanitizer
	return sanitizeTextContent(text)
}

func collapseToolResultContent(results []map[string]any) string {
	parts := make([]string, 0, len(results))
	for _, result := range results {
		switch content := result["content"].(type) {
		case []map[string]string:
			for _, entry := range content {
				if text := strings.TrimSpace(entry["text"]); text != "" {
					parts = append(parts, text)
				}
			}
		case []any:
			for _, raw := range content {
				if entry, ok := raw.(map[string]any); ok {
					if text, _ := entry["text"].(string); strings.TrimSpace(text) != "" {
						parts = append(parts, strings.TrimSpace(text))
					}
				}
			}
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

// stripSystemReminders removes literal "<system-reminder>" tag lines and standalone tokens.
func stripSystemReminders(text string) string {
	if text == "" {
		return ""
	}
	// Remove lines that were just the tag (common in Claude Code transcripts)
	lines := strings.Split(text, "\n")
	kept := lines[:0]
	for _, ln := range lines {
		trim := strings.TrimSpace(ln)
		if strings.EqualFold(trim, "<system-reminder>") {
			continue
		}
		kept = append(kept, ln)
	}
	return strings.Join(kept, "\n")
}

func buildToolContextBlock(entries []toolContextEntry) string {
	if len(entries) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("Tool reference manifest (hash -> tool). Use ToolContextLookup(<hash>) to fetch the preserved description on demand without bloating the system prompt:")
	for _, entry := range entries {
		builder.WriteString("\n- ")
		builder.WriteString(entry.Name)
		builder.WriteString(" [")
		builder.WriteString(entry.Hash)
		builder.WriteString(", ")
		builder.WriteString(strconv.Itoa(entry.Length))
		builder.WriteString(" chars]")
	}
	return sanitizeTextContent(builder.String())
}

func buildToolContextManifest(entries []toolContextEntry) []map[string]any {
	if len(entries) == 0 {
		return nil
	}
	manifest := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		manifest = append(manifest, map[string]any{
			"name":        entry.Name,
			"hash":        entry.Hash,
			"length":      entry.Length,
			"description": entry.Description,
		})
	}
	return manifest
}

func buildToolChoiceMetadata(choice gjson.Result) (map[string]any, string) {
	if !choice.Exists() {
		return nil, ""
	}

	mode := ""
	name := ""

	switch {
	case choice.Type == gjson.String:
		mode = normalizeToolChoiceMode(choice.String())
	case choice.IsObject():
		mode = normalizeToolChoiceMode(choice.Get("type").String())
		if strings.EqualFold(mode, "tool") {
			name = choice.Get("name").String()
			if name == "" {
				name = choice.Get("function.name").String()
			}
		}
		if mode == "" {
			if fn := choice.Get("function.name").String(); fn != "" {
				mode = "tool"
				name = fn
			}
		}
	default:
		return nil, ""
	}

	if mode == "" || strings.EqualFold(mode, "auto") {
		return nil, ""
	}

	meta := map[string]any{"mode": mode}
	if trimmed := strings.TrimSpace(name); trimmed != "" {
		meta["name"] = trimmed
	}
	return meta, buildToolChoiceDirective(mode, name)
}

func normalizeToolChoiceMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "auto":
		return ""
	case "any", "required":
		return "required"
	case "none":
		return "none"
	case "tool", "function":
		return "tool"
	default:
		return value
	}
}

func buildToolChoiceDirective(mode, name string) string {
	switch mode {
	case "none":
		return "Tool directive: respond directly without invoking any tools for this turn."
	case "required":
		return "Tool directive: you must use at least one available tool before concluding your response."
	case "tool":
		if strings.TrimSpace(name) == "" {
			return ""
		}
		return fmt.Sprintf("Tool directive: you must call the tool %q before responding to the user.", name)
	default:
		return ""
	}
}

func buildPlanModeMetadata(messages, tools gjson.Result) (map[string]any, string) {
	available := detectPlanTools(tools)
	tracker := newPlanModeTracker(available)

	if messages.Exists() && messages.IsArray() {
		messages.ForEach(func(_, msg gjson.Result) bool {
			tracker.observeMessage(msg)
			return true
		})
	}
	return tracker.export()
}

type planTransition struct {
	ToolUseID string
	Name      string
	Action    string
}

type planModeTracker struct {
	available    []string
	pending      map[string]planTransition
	pendingOrder []string
	pendingEnter map[string]struct{}
	lastAction   string
	lastTool     string
	seen         bool
}

func newPlanModeTracker(available []string) *planModeTracker {
	return &planModeTracker{
		available:    available,
		pending:      make(map[string]planTransition),
		pendingOrder: make([]string, 0, 4),
		pendingEnter: make(map[string]struct{}),
	}
}

func (p *planModeTracker) observeMessage(msg gjson.Result) {
	content := msg.Get("content")
	if content.IsArray() {
		content.ForEach(func(_, part gjson.Result) bool {
			p.observePart(part)
			return true
		})
	} else if content.Exists() {
		p.observePart(content)
	}
}

func (p *planModeTracker) observePart(part gjson.Result) {
	partType := strings.ToLower(strings.TrimSpace(part.Get("type").String()))
	switch partType {
	case "tool_use":
		p.handleToolUse(part)
	case "tool_result":
		p.handleToolResult(part)
	}
}

func (p *planModeTracker) handleToolUse(part gjson.Result) {
	name := strings.TrimSpace(part.Get("name").String())
	action := classifyPlanAction(name)
	if action == "" {
		return
	}
	id := helpers.SanitizeToolCallID(helpers.FirstString(part.Get("id").String(), part.Get("tool_use_id").String()))
	if id == "" {
		return
	}
	p.pending[id] = planTransition{
		ToolUseID: id,
		Name:      name,
		Action:    action,
	}
	p.pendingOrder = append(p.pendingOrder, id)
	if action == "enter" {
		p.pendingEnter[id] = struct{}{}
	}
	p.lastAction = action
	p.lastTool = name
	p.seen = true
}

func (p *planModeTracker) handleToolResult(part gjson.Result) {
	id := helpers.SanitizeToolCallID(helpers.FirstString(part.Get("tool_use_id").String(), part.Get("tool_useId").String()))
	if id == "" {
		return
	}
	if trans, ok := p.pending[id]; ok {
		delete(p.pending, id)
		if trans.Action == "enter" {
			delete(p.pendingEnter, id)
		}
		p.seen = true
	}
}

func (p *planModeTracker) export() (map[string]any, string) {
	if len(p.available) == 0 && !p.seen {
		return nil, ""
	}

	meta := map[string]any{
		"active": len(p.pendingEnter) > 0,
	}
	if len(p.available) > 0 {
		// Only include when non-empty to avoid sending null arrays upstream
		meta["available"] = p.available
	}
	if len(p.pending) > 0 {
		pending := make([]map[string]string, 0, len(p.pending))
		for _, id := range p.pendingOrder {
			if trans, ok := p.pending[id]; ok {
				pending = append(pending, map[string]string{
					"toolUseId": trans.ToolUseID,
					"name":      trans.Name,
					"action":    trans.Action,
				})
			}
		}
		if len(pending) > 0 {
			meta["pending"] = pending
		}
	}
	if p.lastAction != "" {
		meta["lastTransition"] = p.lastAction
		meta["lastTool"] = p.lastTool
	}
	return meta, p.buildDirective()
}

func (p *planModeTracker) buildDirective() string {
	if len(p.pendingEnter) > 0 {
		ids := make([]string, 0, len(p.pendingEnter))
		for id := range p.pendingEnter {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		return fmt.Sprintf("Plan directive: Task/plan agents (%s) are running—wait for their tool results before concluding.", strings.Join(ids, ", "))
	}

	exitPending := make([]string, 0)
	for _, trans := range p.pending {
		if trans.Action == "exit" {
			exitPending = append(exitPending, trans.ToolUseID)
		}
	}
	if len(exitPending) > 0 {
		sort.Strings(exitPending)
		return fmt.Sprintf("Plan directive: ExitPlanMode requested via %s; acknowledge the exit once the tool returns.", strings.Join(exitPending, ", "))
	}

	if p.lastAction == "exit" {
		return "Plan directive: Plan helpers have exited; return to direct responses until a new Task agent is launched."
	}

	if len(p.available) > 0 {
		return fmt.Sprintf("Plan directive: Plan helpers available (%s). Launch Task agents when multi-step orchestration is needed.", strings.Join(p.available, ", "))
	}
	return ""
}

func detectPlanTools(tools gjson.Result) []string {
	if !tools.Exists() || !tools.IsArray() {
		return nil
	}
	seen := make(map[string]struct{})
	names := make([]string, 0, len(tools.Array()))
	tools.ForEach(func(_, tool gjson.Result) bool {
		name := strings.TrimSpace(tool.Get("name").String())
		if name == "" {
			name = strings.TrimSpace(tool.Get("function.name").String())
		}
		if classifyPlanAction(name) == "" {
			return true
		}
		key := strings.ToLower(name)
		if _, exists := seen[key]; exists {
			return true
		}
		seen[key] = struct{}{}
		names = append(names, name)
		return true
	})
	sort.Strings(names)
	return names
}

func classifyPlanAction(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "task", "plan", "launchplanmode", "launchplanagent":
		return "enter"
	case "exitplanmode", "exitplan", "exitplanagent":
		return "exit"
	default:
		return ""
	}
}
