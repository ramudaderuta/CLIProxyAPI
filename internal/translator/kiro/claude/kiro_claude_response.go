package claude

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro/helpers"
	"github.com/tidwall/gjson"
)

// JSONProcessor provides a unified interface for JSON parsing operations
type JSONProcessor interface {
	IsValidJSON(data []byte) bool
	ParseJSON(data []byte) gjson.Result
	ExtractJSONObjects(line string) []string
	SanitizeJSON(input string) string
	NormalizeArguments(args string) string
}

// KiroJSONProcessor implements JSONProcessor with gjson-based parsing
type KiroJSONProcessor struct{}

// NewJSONProcessor creates a new JSONProcessor instance
func NewJSONProcessor() JSONProcessor {
	return &KiroJSONProcessor{}
}

// IsValidJSON checks if the provided data is valid JSON
func (p *KiroJSONProcessor) IsValidJSON(data []byte) bool {
	return gjson.ValidBytes(data)
}

// ParseJSON parses the provided data and returns a gjson.Result
func (p *KiroJSONProcessor) ParseJSON(data []byte) gjson.Result {
	return gjson.ParseBytes(data)
}

// ExtractJSONObjects extracts all valid JSON objects from a string
func (p *KiroJSONProcessor) ExtractJSONObjects(line string) []string {
	return extractJSONFromLine(line)
}

// SanitizeJSON sanitizes malformed JSON strings
func (p *KiroJSONProcessor) SanitizeJSON(input string) string {
	return sanitizeJSON(input)
}

// NormalizeArguments normalizes tool call arguments
func (p *KiroJSONProcessor) NormalizeArguments(args string) string {
	return normalizeArguments(args)
}

// ContentExtractor defines interface for extracting content from JSON structures
type ContentExtractor interface {
	ExtractTextFromContent(result gjson.Result) string
	ExtractToolCallsFromContent(result gjson.Result) []OpenAIToolCall
}

// KiroContentExtractor implements ContentExtractor for Kiro responses
type KiroContentExtractor struct{}

// NewContentExtractor creates a new ContentExtractor instance
func NewContentExtractor() ContentExtractor {
	return &KiroContentExtractor{}
}

// ExtractTextFromContent extracts text content from nested JSON structures
func (e *KiroContentExtractor) ExtractTextFromContent(result gjson.Result) string {
	return collectTextFromContent(result)
}

// ExtractToolCallsFromContent extracts tool calls from nested JSON structures
func (e *KiroContentExtractor) ExtractToolCallsFromContent(result gjson.Result) []OpenAIToolCall {
	return extractToolCallsFromContent(result)
}

// ResponseParser defines the main parsing interface
type ResponseParser interface {
	ParseResponse(data []byte) (string, []OpenAIToolCall)
}

// KiroResponseParser implements ResponseParser with dependency injection
type KiroResponseParser struct {
	jsonProcessor    JSONProcessor
	contentExtractor ContentExtractor
}

// NewResponseParser creates a new ResponseParser with injected dependencies
func NewResponseParser(processor JSONProcessor, extractor ContentExtractor) ResponseParser {
	if processor == nil {
		processor = NewJSONProcessor()
	}
	if extractor == nil {
		extractor = NewContentExtractor()
	}
	return &KiroResponseParser{
		jsonProcessor:    processor,
		contentExtractor: extractor,
	}
}

// ParseResponse extracts assistant text and tool calls from a Kiro upstream payload.
func (p *KiroResponseParser) ParseResponse(data []byte) (string, []OpenAIToolCall) {
	if len(data) == 0 {
		return "", nil
	}

	// Try to parse as JSON first
	if p.jsonProcessor.IsValidJSON(data) {
		return p.parseJSONResponse(data)
	}

	// If not valid JSON, try to parse as SSE stream
	content, toolCalls := parseEventStream(string(data))

	// If SSE parsing also returns empty content and we have non-empty input,
	// return the input as-is (for plain text fallback)
	if content == "" && len(data) > 0 {
		content = p.handleMalformedInput(data)
	}

	return content, toolCalls
}

// parseJSONResponse handles parsing of valid JSON responses
func (p *KiroResponseParser) parseJSONResponse(data []byte) (string, []OpenAIToolCall) {
	root := p.jsonProcessor.ParseJSON(data)

	// Extract content using strategy pattern
	content := p.extractContentFromConversationState(root)

	// If no content found, try fallback strategies
	if strings.TrimSpace(content) == "" {
		content = p.extractContentWithFallbacks(root)
	}

	// Extract tool calls
	toolCalls := p.extractAllToolCalls(root)

	cleanContent := helpers.SanitizeAssistantText(strings.TrimSpace(content))
	return cleanContent, deduplicateToolCalls(toolCalls)
}

// extractContentFromConversationState extracts content from conversation state
func (p *KiroResponseParser) extractContentFromConversationState(root gjson.Result) string {
	var content string

	// Try currentMessage first
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

	return content
}

// extractContentWithFallbacks tries multiple fallback strategies for content extraction
func (p *KiroResponseParser) extractContentWithFallbacks(root gjson.Result) string {
	// Try Anthropic-style message bodies in order of preference
	fallbackPaths := []string{"content", "message.content", "message"}

	for _, path := range fallbackPaths {
		if content := p.contentExtractor.ExtractTextFromContent(root.Get(path)); content != "" {
			return content
		}
	}

	return ""
}

// extractAllToolCalls extracts tool calls from all possible locations
func (p *KiroResponseParser) extractAllToolCalls(root gjson.Result) []OpenAIToolCall {
	var toolCalls []OpenAIToolCall

	// Extract from conversationState
	toolCalls = append(toolCalls, p.extractToolCallsFromConversationState(root)...)

	// Extract from content structures
	contentPaths := []string{"content", "message.content", "message"}
	for _, path := range contentPaths {
		toolCalls = append(toolCalls, p.contentExtractor.ExtractToolCallsFromContent(root.Get(path))...)
	}

	return toolCalls
}

// extractToolCallsFromConversationState extracts tool calls from conversation state
func (p *KiroResponseParser) extractToolCallsFromConversationState(root gjson.Result) []OpenAIToolCall {
	var toolCalls []OpenAIToolCall

	// Check for toolUse at currentMessage level
	toolUsePaths := []string{
		"conversationState.currentMessage.toolUse",
		"conversationState.currentMessage.assistantResponseMessage.toolUse",
	}

	for _, path := range toolUsePaths {
		if toolUse := root.Get(path); toolUse.Exists() {
			if toolUse.IsArray() {
				toolCalls = append(toolCalls, extractToolCalls(toolUse.Array())...)
			} else {
				toolCalls = append(toolCalls, extractToolCalls([]gjson.Result{toolUse})...)
			}
			break // Found tool calls, no need to check other paths
		}
	}

	return toolCalls
}

// handleMalformedInput processes input that isn't valid JSON
func (p *KiroResponseParser) handleMalformedInput(data []byte) string {
	inputStr := string(data)

	// Check if the input looks like plain text (no JSON-like structures)
	if !strings.Contains(inputStr, "{") && !strings.Contains(inputStr, "data:") {
		return strings.TrimSpace(inputStr)
	}

	// For malformed JSON-like input, try to extract any plain text content
	return extractPlainTextFromMalformedInput(inputStr)
}

// OpenAIToolCall represents a function/tool call in an OpenAI-compatible response.
type OpenAIToolCall struct {
	ID        string
	Name      string
	Arguments string
}

// ParseResponse extracts assistant text and tool calls from a Kiro upstream payload.
// This function maintains backward compatibility by using default implementations.
func ParseResponse(data []byte) (string, []OpenAIToolCall) {
	parser := NewResponseParser(nil, nil)
	return parser.ParseResponse(data)
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

func collectTextFromContent(result gjson.Result) string {
	if !result.Exists() {
		return ""
	}

	var builder strings.Builder
	var visit func(gjson.Result)

	visit = func(node gjson.Result) {
		if !node.Exists() {
			return
		}
		if node.IsArray() {
			for _, item := range node.Array() {
				visit(item)
			}
			return
		}
		if node.IsObject() {
			if text := node.Get("text"); text.Exists() {
				visit(text)
			}
			if nested := node.Get("content"); nested.Exists() {
				visit(nested)
			}
			return
		}

		value := node.String()
		if value == "" {
			return
		}
		builder.WriteString(strings.ReplaceAll(value, `\n`, "\n"))
	}

	visit(result)
	return builder.String()
}

func extractToolCallsFromContent(result gjson.Result) []OpenAIToolCall {
	if !result.Exists() {
		return nil
	}

	calls := make([]OpenAIToolCall, 0)

	var visit func(gjson.Result)
	visit = func(node gjson.Result) {
		if !node.Exists() {
			return
		}

		if node.IsArray() {
			for _, item := range node.Array() {
				visit(item)
			}
			return
		}

		if node.IsObject() {
			if node.Get("type").String() == "tool_use" {
				id := node.Get("id").String()
				if id == "" {
					id = node.Get("toolUseId").String()
				}
				name := node.Get("name").String()

				var arguments string
				if input := node.Get("input"); input.Exists() {
					raw := strings.TrimSpace(input.Raw)
					if raw != "" && raw != "null" && raw != "{}" {
						if normalized := normalizeArguments(raw); normalized != "" {
							arguments = normalized
						} else {
							arguments = raw
						}
					}
				}

				calls = append(calls, OpenAIToolCall{
					ID:        id,
					Name:      name,
					Arguments: arguments,
				})
				return
			}

			if nested := node.Get("content"); nested.Exists() {
				visit(nested)
			}
			return
		}
	}

	visit(result)
	return calls
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
	// Parse SSE stream properly, handling large JSON objects and thinking blocks
	return parseSSEStreamWithThinkingSupport(raw)
}

// parseSSEStreamWithThinkingSupport handles SSE streams with proper buffer management
// and thinking block support to prevent truncation

// toolAccumulator represents a tool call being accumulated across multiple SSE events
type toolAccumulator struct {
	call      OpenAIToolCall
	fragments strings.Builder
	hasStream bool
	finalized bool
}

// extractPlainTextFromMalformedInput extracts plain text from malformed JSON-like input
func extractPlainTextFromMalformedInput(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}

	// Remove surrounding braces if present
	if strings.HasPrefix(input, "{") && strings.HasSuffix(input, "}") {
		input = strings.TrimSpace(input[1 : len(input)-1])
	}

	// Split by common JSON separators and take the most text-like part
	parts := strings.FieldsFunc(input, func(r rune) bool {
		return r == ':' || r == ',' || r == '[' || r == ']' || r == '{' || r == '}'
	})

	// Find the longest part that looks like text
	var bestPart string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		// Remove quotes if present
		if strings.HasPrefix(part, `"`) && strings.HasSuffix(part, `"`) {
			part = strings.TrimSpace(strings.Trim(part, `"`))
		}
		if len(part) > len(bestPart) && len(part) > 1 {
			bestPart = part
		}
	}

	if bestPart != "" {
		return bestPart
	}

	// Fallback: return the cleaned input
	return input
}

// extractJSONFromLine extracts all JSON objects from a line that may contain
// multiple JSON objects concatenated with control characters
func extractJSONFromLine(line string) []string {
	var jsonObjects []string
	start := strings.Index(line, "{")

	for start != -1 {
		// Look for the matching closing brace, handling unescaped quotes
		braceCount := 0
		inString := false
		escapeNext := false

		for i := start; i < len(line); i++ {
			char := rune(line[i])
			if escapeNext {
				escapeNext = false
				continue
			}

			switch char {
			case '\\':
				if inString {
					escapeNext = true
				}
			case '"':
				inString = !inString
			case '{':
				if !inString {
					braceCount++
				}
			case '}':
				if !inString {
					braceCount--
					if braceCount == 0 {
						candidate := line[start : i+1]
						if gjson.Valid(candidate) {
							jsonObjects = append(jsonObjects, candidate)
						}
						// Look for next JSON object
						start = strings.Index(line[i+1:], "{")
						if start != -1 {
							start = i + 1 + start
						}
						break
					}
				}
			}
		}

		if start == -1 || start >= len(line) {
			break
		}

		// Prevent infinite loops by advancing start if we didn't find a complete JSON
		if braceCount > 0 {
			break // Incomplete JSON, exit loop
		}
	}

	return jsonObjects
}

// SSEEventProcessor defines interface for processing SSE events
type SSEEventProcessor interface {
	ProcessEvent(eventType string, node gjson.Result, context *SSEProcessingContext)
}

// SSEProcessingContext holds the state and accumulators for SSE processing
type SSEProcessingContext struct {
	TextBuilder   strings.Builder
	ToolOrder     []string
	ToolByID      map[string]*toolAccumulator
	ToolIndex     map[int]string
	JSONProcessor JSONProcessor
}

// NewSSEProcessingContext creates a new SSE processing context
func NewSSEProcessingContext(jsonProcessor JSONProcessor) *SSEProcessingContext {
	if jsonProcessor == nil {
		jsonProcessor = NewJSONProcessor()
	}
	return &SSEProcessingContext{
		ToolOrder:     make([]string, 0),
		ToolByID:      make(map[string]*toolAccumulator),
		ToolIndex:     make(map[int]string),
		JSONProcessor: jsonProcessor,
	}
}

// KiroSSEEventProcessor implements SSEEventProcessor for Kiro-style SSE streams
type KiroSSEEventProcessor struct{}

// NewSSEEventProcessor creates a new SSEEventProcessor
func NewSSEEventProcessor() SSEEventProcessor {
	return &KiroSSEEventProcessor{}
}

// ProcessEvent processes a single SSE event based on its type
func (p *KiroSSEEventProcessor) ProcessEvent(eventType string, node gjson.Result, context *SSEProcessingContext) {
	switch eventType {
	case "content_block_start":
		p.handleContentBlockStart(node, context)
	case "content_block_delta":
		p.handleContentBlockDelta(node, context)
	case "content_block_stop":
		p.handleContentBlockStop(node, context)
	case "message_start":
		p.appendTextFromNode(node.Get("message"), context)
	case "message_delta":
		p.appendTextFromNode(node.Get("delta"), context)
	case "message":
		p.appendTextFromNode(node.Get("content"), context)
		p.appendTextFromNode(node.Get("message"), context)
	default:
		p.handleDefaultEvent(node, context)
	}
}

// handleContentBlockStart processes content_block_start events
func (p *KiroSSEEventProcessor) handleContentBlockStart(node gjson.Result, context *SSEProcessingContext) {
	block := node.Get("content_block")
	if block.Exists() && block.Get("type").String() == "tool_use" {
		p.handleToolUseStart(block, node, context)
	} else {
		p.appendTextFromNode(block, context)
	}
}

// handleToolUseStart processes tool_use start events
func (p *KiroSSEEventProcessor) handleToolUseStart(block, node gjson.Result, context *SSEProcessingContext) {
	id := p.extractToolID(block)
	name := block.Get("name").String()

	if acc := context.ensureAccumulator(id, name); acc != nil {
		if input := block.Get("input"); input.Exists() {
			rawInput := strings.TrimSpace(input.Raw)
			if rawInput != "" && rawInput != "null" && rawInput != "{}" {
				acc.call.Arguments = rawInput
			}
		}
		if idxVal := node.Get("index"); idxVal.Exists() {
			context.ToolIndex[int(idxVal.Int())] = id
		}
	}
}

// extractToolID extracts tool ID from various possible fields
func (p *KiroSSEEventProcessor) extractToolID(block gjson.Result) string {
	if id := block.Get("id").String(); id != "" {
		return id
	}
	return block.Get("toolUseId").String()
}

// handleContentBlockDelta processes content_block_delta events
func (p *KiroSSEEventProcessor) handleContentBlockDelta(node gjson.Result, context *SSEProcessingContext) {
	delta := node.Get("delta")
	deltaType := delta.Get("type").String()

	switch deltaType {
	case "text_delta":
		if text := delta.Get("text"); text.Exists() {
			p.appendTextFromNode(text, context)
		}
	case "input_json_delta":
		p.handleInputJSONDelta(node, context)
	default:
		// Generic delta handling for backward compatibility
		p.appendTextFromNode(delta, context)
		if partial := node.Get("delta.partial_json").String(); partial != "" {
			p.handlePartialJSON(node, partial, context)
		}
	}
}

// handleInputJSONDelta processes input_json_delta events
func (p *KiroSSEEventProcessor) handleInputJSONDelta(node gjson.Result, context *SSEProcessingContext) {
	if partial := node.Get("delta.partial_json").String(); partial != "" {
		p.handlePartialJSON(node, partial, context)
	}
}

// handlePartialJSON processes partial JSON data
func (p *KiroSSEEventProcessor) handlePartialJSON(node gjson.Result, partial string, context *SSEProcessingContext) {
	idxVal := int(node.Get("index").Int())
	if id := context.ToolIndex[idxVal]; id != "" {
		if acc := context.ensureAccumulator(id, ""); acc != nil {
			acc.fragments.WriteString(partial)
			acc.hasStream = true
		}
	}
}

// handleContentBlockStop processes content_block_stop events
func (p *KiroSSEEventProcessor) handleContentBlockStop(node gjson.Result, context *SSEProcessingContext) {
	idxVal := int(node.Get("index").Int())
	if id := context.ToolIndex[idxVal]; id != "" {
		context.finalizeToolCall(id)
	}
}

// handleDefaultEvent handles unknown event types
func (p *KiroSSEEventProcessor) handleDefaultEvent(node gjson.Result, context *SSEProcessingContext) {
	if node.Get("followupPrompt").Bool() {
		return
	}

	if name := node.Get("name").String(); name != "" {
		p.handleLegacyToolCall(node, context)
	} else {
		p.appendTextFromNode(node.Get("content"), context)
		p.appendTextFromNode(node.Get("delta"), context)
		p.appendTextFromNode(node.Get("message"), context)
	}
}

// handleLegacyToolCall processes legacy tool call events
func (p *KiroSSEEventProcessor) handleLegacyToolCall(node gjson.Result, context *SSEProcessingContext) {
	name := node.Get("name").String()
	id := p.extractLegacyToolID(node)

	if acc := context.ensureAccumulator(id, name); acc != nil {
		if input := node.Get("input"); input.Exists() {
			appendLegacyToolInput(acc, input)
		}
		if node.Get("stop").Bool() {
			context.finalizeToolCall(id)
		}
	}
}

func appendLegacyToolInput(acc *toolAccumulator, input gjson.Result) {
	rawInput := strings.TrimSpace(input.Raw)
	if rawInput == "" || rawInput == "null" {
		return
	}

	switch {
	case input.IsObject() || input.IsArray():
		if acc.call.Arguments != "" {
			acc.call.Arguments = mergeJSONArguments(acc.call.Arguments, rawInput)
		} else {
			acc.call.Arguments = rawInput
		}
	case input.Type == gjson.String:
		fallthrough
	default:
		chunk := input.String()
		if chunk == "" {
			chunk = strings.Trim(rawInput, `"`)
		}
		if chunk == "" {
			return
		}
		acc.fragments.WriteString(chunk)
		acc.hasStream = true
	}
}

// extractLegacyToolID extracts tool ID from legacy event formats
func (p *KiroSSEEventProcessor) extractLegacyToolID(node gjson.Result) string {
	if id := node.Get("toolUseId").String(); id != "" {
		return id
	}
	return node.Get("tool_use_id").String()
}

// appendTextFromNode extracts and appends text content from a JSON node
func (p *KiroSSEEventProcessor) appendTextFromNode(value gjson.Result, context *SSEProcessingContext) {
	if !value.Exists() {
		return
	}

	var visit func(gjson.Result)
	visit = func(v gjson.Result) {
		if !v.Exists() {
			return
		}
		if v.IsArray() {
			for _, item := range v.Array() {
				visit(item)
			}
			return
		}
		if v.IsObject() {
			if text := v.Get("text"); text.Exists() {
				visit(text)
			}
			if nested := v.Get("content"); nested.Exists() {
				visit(nested)
			}
			return
		}
		text := v.String()
		if text == "" {
			return
		}
		// Filter out "Thinking" content at the SSE parsing level
		if text == "Thinking" {
			return // Skip thinking content
		}
		decoded := strings.ReplaceAll(text, `\n`, "\n")
		context.TextBuilder.WriteString(decoded)
	}

	visit(value)
}

// ensureAccumulator ensures a tool accumulator exists for the given ID
func (c *SSEProcessingContext) ensureAccumulator(id, name string) *toolAccumulator {
	if id == "" {
		return nil
	}
	if acc, ok := c.ToolByID[id]; ok {
		if acc.call.Name == "" && name != "" {
			acc.call.Name = name
		}
		return acc
	}
	acc := &toolAccumulator{call: OpenAIToolCall{ID: id, Name: name}}
	c.ToolByID[id] = acc
	c.ToolOrder = append(c.ToolOrder, id)
	return acc
}

// finalizeToolCall finalizes a tool call with proper argument normalization
func (c *SSEProcessingContext) finalizeToolCall(id string) {
	if id == "" {
		return
	}
	acc, ok := c.ToolByID[id]
	if !ok || acc.finalized {
		return
	}

	// Use accumulated fragments if available, otherwise use existing arguments
	if acc.hasStream && acc.fragments.Len() > 0 {
		// Use the accumulated fragments as the arguments
		acc.call.Arguments = acc.fragments.String()
	}

	// Normalize the arguments if they exist
	if acc.call.Arguments != "" {
		acc.call.Arguments = c.JSONProcessor.NormalizeArguments(acc.call.Arguments)
	}
	acc.finalized = true
}

// SSEStreamParser defines interface for parsing SSE streams
type SSEStreamParser interface {
	ParseStream(raw string) (string, []OpenAIToolCall)
}

// KiroSSEStreamParser implements SSEStreamParser for Kiro-style streams
type KiroSSEStreamParser struct {
	eventProcessor SSEEventProcessor
	jsonProcessor  JSONProcessor
}

// NewSSEStreamParser creates a new SSEStreamParser with injected dependencies
func NewSSEStreamParser(eventProcessor SSEEventProcessor, jsonProcessor JSONProcessor) SSEStreamParser {
	if eventProcessor == nil {
		eventProcessor = NewSSEEventProcessor()
	}
	if jsonProcessor == nil {
		jsonProcessor = NewJSONProcessor()
	}
	return &KiroSSEStreamParser{
		eventProcessor: eventProcessor,
		jsonProcessor:  jsonProcessor,
	}
}

// ParseStream parses an SSE stream and returns content and tool calls
func (p *KiroSSEStreamParser) ParseStream(raw string) (string, []OpenAIToolCall) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}

	// Create processing context
	context := NewSSEProcessingContext(p.jsonProcessor)

	// Parse SSE lines
	p.parseSSELines(raw, context)

	// Finalize all pending tool calls
	p.finalizeAllToolCalls(context)

	// Extract results
	content := helpers.SanitizeAssistantText(strings.TrimSpace(context.TextBuilder.String()))
	toolCalls := p.extractToolCalls(context)

	// Parse bracket-style tool calls as fallback
	bracketCalls := parseBracketToolCalls(raw)
	if len(bracketCalls) > 0 {
		toolCalls = append(toolCalls, bracketCalls...)
	}

	return content, deduplicateToolCalls(toolCalls)
}

// parseSSELines processes individual SSE lines
func (p *KiroSSEStreamParser) parseSSELines(raw string, context *SSEProcessingContext) {
	lines := strings.Split(raw, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Clean SSE prefix
		cleanLine := p.cleanSSELine(line)
		if cleanLine == "" {
			continue
		}

		// Process the line
		p.processSSELine(cleanLine, context)
	}
}

// cleanSSELine removes SSE prefixes from a line
func (p *KiroSSEStreamParser) cleanSSELine(line string) string {
	// Handle SSE "data: " prefix
	if strings.HasPrefix(line, "data: ") {
		line = strings.TrimPrefix(line, "data: ")
		return strings.TrimSpace(line)
	}
	if strings.HasPrefix(line, "data:") {
		line = strings.TrimPrefix(line, "data:")
		return strings.TrimSpace(line)
	}
	return line
}

// processSSELine processes a single SSE line
func (p *KiroSSEStreamParser) processSSELine(line string, context *SSEProcessingContext) {
	// Handle malformed data - extract JSON from malformed lines
	if !p.jsonProcessor.IsValidJSON([]byte(line)) {
		p.processMalformedLine(line, context)
		return
	}

	// Process valid JSON line
	node := p.jsonProcessor.ParseJSON([]byte(line))
	if helpers.IsContextUsagePayload(node) {
		return
	}
	eventType := node.Get("type").String()
	p.eventProcessor.ProcessEvent(eventType, node, context)
}

// processMalformedLine handles lines that contain malformed JSON
func (p *KiroSSEStreamParser) processMalformedLine(line string, context *SSEProcessingContext) {
	jsonObjects := p.jsonProcessor.ExtractJSONObjects(line)
	for _, jsonObj := range jsonObjects {
		if p.jsonProcessor.IsValidJSON([]byte(jsonObj)) {
			node := p.jsonProcessor.ParseJSON([]byte(jsonObj))
			if helpers.IsContextUsagePayload(node) {
				continue
			}
			eventType := node.Get("type").String()
			p.eventProcessor.ProcessEvent(eventType, node, context)
		}
	}
}

// finalizeAllToolCalls finalizes all accumulated tool calls
func (p *KiroSSEStreamParser) finalizeAllToolCalls(context *SSEProcessingContext) {
	for _, id := range context.ToolOrder {
		context.finalizeToolCall(id)
	}
}

// extractToolCalls extracts finalized tool calls from context
func (p *KiroSSEStreamParser) extractToolCalls(context *SSEProcessingContext) []OpenAIToolCall {
	toolCalls := make([]OpenAIToolCall, 0, len(context.ToolOrder))
	for _, id := range context.ToolOrder {
		if acc, ok := context.ToolByID[id]; ok && acc.finalized {
			toolCalls = append(toolCalls, acc.call)
		}
	}
	return toolCalls
}

// parseSSEStreamWithThinkingSupport parses SSE streams with thinking support
// This function maintains backward compatibility by using default implementations
func parseSSEStreamWithThinkingSupport(raw string) (string, []OpenAIToolCall) {
	parser := NewSSEStreamParser(nil, nil)
	return parser.ParseStream(raw)
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
	if block == "" {
		return nil
	}

	// Try to find complete JSON objects/arrays by tracking structure
	braceCount := 0
	inString := false
	escapeNext := false

	for i, char := range block {
		if escapeNext {
			escapeNext = false
			continue
		}

		switch char {
		case '\\':
			if inString {
				escapeNext = true
			}
		case '"':
			inString = !inString
		case '{', '[':
			if !inString {
				braceCount++
			}
		case '}', ']':
			if !inString {
				braceCount--
				if braceCount == 0 {
					// Found complete JSON structure
					candidate := strings.TrimSpace(block[:i+1])
					if json.Valid([]byte(candidate)) {
						return []byte(candidate)
					}
				}
			}
		}
	}

	// Fallback: try original approach for edge cases, but preserve more content
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

// decodeToolArguments attempts to parse tool call arguments into a JSON object.
// Some upstream providers double-encode the payload, so we progressively try to
// parse, normalize, and unquote the raw value until it becomes a valid object.
func decodeToolArguments(raw string) (map[string]any, bool) {
	candidate := strings.TrimSpace(raw)
	if candidate == "" || candidate == "null" {
		return map[string]any{}, true
	}

	for i := 0; i < 3; i++ {
		if input, ok := tryUnmarshalToolInput(candidate); ok {
			return input, true
		}

		if normalized := normalizeArguments(candidate); normalized != "" && normalized != candidate {
			if input, ok := tryUnmarshalToolInput(normalized); ok {
				return input, true
			}
		}

		unquoted, err := strconv.Unquote(candidate)
		if err != nil {
			break
		}
		candidate = strings.TrimSpace(unquoted)
	}

	return nil, false
}

func tryUnmarshalToolInput(data string) (map[string]any, bool) {
	var input map[string]any
	if err := json.Unmarshal([]byte(data), &input); err != nil {
		return nil, false
	}
	return input, true
}

func buildToolLeadIn(toolCalls []OpenAIToolCall) string {
	if len(toolCalls) == 0 {
		return ""
	}
	names := make([]string, 0, len(toolCalls))
	seen := make(map[string]struct{}, len(toolCalls))
	for _, call := range toolCalls {
		name := strings.TrimSpace(call.Name)
		if name == "" {
			name = "the requested tool"
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	if len(names) == 1 {
		return fmt.Sprintf("Calling %s to handle this request.", names[0])
	}
	return fmt.Sprintf("Calling tools %s to handle this request.", strings.Join(names, ", "))
}

// mergeJSONArguments merges two JSON objects, with the second one taking precedence for overlapping keys
func mergeJSONArguments(existing, new string) string {
	existing = strings.TrimSpace(existing)
	new = strings.TrimSpace(new)

	if existing == "" {
		return new
	}
	if new == "" {
		return existing
	}

	// Try to parse both as JSON objects
	var existingObj, newObj map[string]interface{}

	if err := json.Unmarshal([]byte(existing), &existingObj); err != nil {
		// If existing is not valid JSON, return the new one
		return new
	}

	if err := json.Unmarshal([]byte(new), &newObj); err != nil {
		// If new is not valid JSON, return the existing one
		return existing
	}

	// Merge the objects - new values override existing ones
	for key, value := range newObj {
		existingObj[key] = value
	}

	// Marshal back to JSON
	result, err := json.Marshal(existingObj)
	if err != nil {
		return existing // Fallback to existing if marshaling fails
	}

	return string(result)
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
	content = helpers.SanitizeAssistantText(strings.TrimSpace(content))
	if content == "" && len(toolCalls) > 0 {
		content = buildToolLeadIn(toolCalls)
	}

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
		input, ok := decodeToolArguments(call.Arguments)
		if !ok {
			input = map[string]any{"value": call.Arguments}
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
		ID:           fmt.Sprintf("msg_%s", uuid.NewString()),
		Type:         "message",
		Role:         "assistant",
		Model:        model,
		Content:      contentBlocks,
		StopReason:   stopReason,
		StopSequence: nil, // BUG FIX: stop_sequence should be null per Anthropic spec
		Usage: Usage{
			InputTokens:  promptTokens,
			OutputTokens: completionTokens,
		},
	}

	return json.Marshal(payload)
}

// AnthropicMessage represents the Anthropic messages API response structure
type AnthropicMessage struct {
	ID           string           `json:"id"`
	Type         string           `json:"type"`
	Role         string           `json:"role"`
	Model        string           `json:"model"`
	Content      []map[string]any `json:"content"`
	StopReason   string           `json:"stop_reason"`
	StopSequence *string          `json:"stop_sequence"` // BUG FIX: Use pointer to allow null
	Usage        Usage            `json:"usage"`
}

// Usage represents token usage with int64 types
type Usage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
}

// BuildAnthropicStreamingChunks generates Anthropic-compatible streaming chunks formatted as SSE events.
func BuildAnthropicStreamingChunks(id, model string, created int64, content string, toolCalls []OpenAIToolCall, promptTokens, completionTokens int64) [][]byte {
	chunks := make([][]byte, 0, 6)

	outputTokens := completionTokens
	inputTokens := promptTokens

	messageStart := helpers.BuildMessageStartEvent(model)
	chunks = append(chunks, helpers.BuildSSEEvent("message_start", messageStart))

	trimmed := strings.TrimSpace(content)

	for idx, call := range toolCalls {
		toolStart := buildToolUseStartEvent(call, idx)
		chunks = append(chunks, helpers.BuildSSEEvent("content_block_start", toolStart))

		toolDelta := buildToolUseDeltaEvent(call, idx)
		chunks = append(chunks, helpers.BuildSSEEvent("content_block_delta", toolDelta))

		toolStop := helpers.BuildContentBlockStopEvent(idx)
		chunks = append(chunks, helpers.BuildSSEEvent("content_block_stop", toolStop))
	}

	if trimmed != "" {
		textIndex := len(toolCalls)
		contentStart := helpers.BuildContentBlockStartEvent(textIndex)
		chunks = append(chunks, helpers.BuildSSEEvent("content_block_start", contentStart))

		contentDelta := helpers.BuildContentBlockDeltaEvent(textIndex, trimmed)
		chunks = append(chunks, helpers.BuildSSEEvent("content_block_delta", contentDelta))

		contentStop := helpers.BuildContentBlockStopEvent(textIndex)
		chunks = append(chunks, helpers.BuildSSEEvent("content_block_stop", contentStop))
	}

	stopReason := ""
	if len(toolCalls) > 0 {
		stopReason = "tool_use"
	} else if trimmed != "" {
		stopReason = "end_turn"
	}

	messageDelta := helpers.BuildMessageDeltaEvent(stopReason, inputTokens, outputTokens)
	chunks = append(chunks, helpers.BuildSSEEvent("message_delta", messageDelta))
	chunks = append(chunks, helpers.BuildSSEEvent("message_stop", helpers.BuildMessageStopEvent()))

	return chunks
}

// buildToolUseStartEvent creates the content_block_start event structure for tool_use
func buildToolUseStartEvent(call OpenAIToolCall, blockIndex int) map[string]any {
	return map[string]any{
		"type":  "content_block_start",
		"index": blockIndex,
		"content_block": map[string]any{
			"type":  "tool_use",
			"id":    call.ID,
			"name":  call.Name,
			"input": map[string]any{},
		},
	}
}

// buildToolUseDeltaEvent creates the content_block_delta event structure for tool input
func buildToolUseDeltaEvent(call OpenAIToolCall, index int) map[string]any {
	input, ok := decodeToolArguments(call.Arguments)
	if !ok {
		input = map[string]any{"value": call.Arguments}
	}

	return map[string]any{
		"type":  "content_block_delta",
		"index": index,
		"delta": map[string]any{
			"type":         "input_json_delta",
			"partial_json": string(marshalJSON(input)),
		},
	}
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
