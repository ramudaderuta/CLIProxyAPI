package kiro

import (
	"strings"

	"github.com/tidwall/gjson"
)

// ConvertKiroStreamToAnthropic maps legacy Kiro SSE payloads into Anthropic-compatible SSE chunks.
// Returns nil when the payload does not look like an SSE stream so the caller can fall back.
func ConvertKiroStreamToAnthropic(raw []byte, model string, promptTokens, completionTokens int64) [][]byte {
	normalized := NormalizeKiroStreamPayload(raw)
	frames := splitKiroFrames(string(normalized))
	if len(frames) == 0 {
		return nil
	}

	builder := newAnthropicLegacyStreamBuilder(model, promptTokens, completionTokens)
	for _, frame := range frames {
		builder.consume(frame)
	}
	return builder.finalize()
}

type kiroStreamFrame struct {
	event string
	data  string
}

func splitKiroFrames(raw string) []kiroStreamFrame {
	lines := strings.Split(raw, "\n")
	frames := make([]kiroStreamFrame, 0, len(lines)/2)

	var current kiroStreamFrame
	var dataBuilder strings.Builder

	reset := func() {
		if dataBuilder.Len() == 0 {
			current = kiroStreamFrame{}
			return
		}
		current.data = strings.TrimSpace(dataBuilder.String())
		if current.data != "" {
			frames = append(frames, current)
		}
		current = kiroStreamFrame{}
		dataBuilder.Reset()
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			reset()
		case strings.HasPrefix(trimmed, "event:"):
			current.event = strings.TrimSpace(strings.TrimPrefix(trimmed, "event:"))
		case strings.HasPrefix(trimmed, "data:"):
			payload := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
			if payload != "" {
				if dataBuilder.Len() > 0 {
					dataBuilder.WriteByte('\n')
				}
				dataBuilder.WriteString(payload)
			}
		case strings.HasPrefix(trimmed, ":"):
			// comment / keep-alive – ignore
		default:
			// legacy payload without prefixes – treat as data
			if dataBuilder.Len() > 0 {
				dataBuilder.WriteByte('\n')
			}
			dataBuilder.WriteString(trimmed)
		}
	}
	reset()
	return frames
}

type anthropicLegacyStreamBuilder struct {
	model            string
	promptTokens     int64
	completionTokens int64

	events         [][]byte
	messageStarted bool

	nextBlockIndex int
	textIndex      int
	textStarted    bool
	textStopped    bool

	toolIndexes   map[string]int
	toolStopped   map[string]bool
	toolNames     map[string]string
	toolFragments map[string]*strings.Builder

	followupPrompt bool
	seenPayload    bool
	stopReason     string
}

func newAnthropicLegacyStreamBuilder(model string, promptTokens, completionTokens int64) *anthropicLegacyStreamBuilder {
	return &anthropicLegacyStreamBuilder{
		model:            model,
		promptTokens:     promptTokens,
		completionTokens: completionTokens,
		events:           make([][]byte, 0, 16),
		textIndex:        -1,
		toolIndexes:      make(map[string]int),
		toolStopped:      make(map[string]bool),
		toolNames:        make(map[string]string),
		toolFragments:    make(map[string]*strings.Builder),
	}
}

func (b *anthropicLegacyStreamBuilder) consume(frame kiroStreamFrame) {
	payload := strings.TrimSpace(frame.data)
	if payload == "" {
		return
	}

	if !gjson.Valid(payload) {
		if text := sanitizeStreamingTextChunk(payload); text != "" {
			b.appendText(text)
		}
		return
	}

	node := gjson.Parse(payload)

	if isMeteringPayload(node) {
		return
	}

	if isContextUsagePayload(node) {
		return
	}

	if sr := node.Get("delta.stop_reason"); sr.Exists() && sr.String() != "" {
		b.stopReason = sr.String()
	}
	if sr := node.Get("stop_reason"); sr.Exists() && sr.String() != "" {
		b.stopReason = sr.String()
	}
	if sr := node.Get("delta.stopReason"); sr.Exists() && sr.String() != "" {
		b.stopReason = sr.String()
	}
	if node.Get("followupPrompt").Bool() || node.Get("delta.followup_prompt").Bool() {
		b.followupPrompt = true
	}

	eventType := strings.ToLower(strings.TrimSpace(node.Get("type").String()))
	switch eventType {
	case "message_delta", "message_stop":
		return
	}

	// Legacy text chunks
	if value := node.Get("content"); value.Exists() {
		if text := sanitizeStreamingTextChunk(value.String()); text != "" {
			b.appendText(text)
		}
		if node.Get("followupPrompt").Bool() {
			b.followupPrompt = true
		}
		return
	}

	// Legacy tool chunks
	if name := strings.TrimSpace(node.Get("name").String()); name != "" {
		id := SanitizeToolCallID(firstString(node.Get("toolUseId").String(), node.Get("tool_use_id").String()))
		if id == "" {
			return
		}
		b.appendToolDelta(id, name, node)
		if node.Get("stop").Bool() {
			b.stopTool(id)
		}
		return
	}

	// Fallback: treat anything else as plain text
	if text := sanitizeStreamingTextChunk(node.String()); text != "" {
		b.appendText(text)
	}
}

func (b *anthropicLegacyStreamBuilder) ensureMessageStart() {
	if b.messageStarted {
		return
	}
	payload := buildMessageStartEvent(b.model)
	b.events = append(b.events, buildSSEEvent("message_start", payload))
	b.messageStarted = true
}

func (b *anthropicLegacyStreamBuilder) ensureTextBlock() {
	if b.textStarted {
		return
	}
	b.ensureMessageStart()
	b.textIndex = b.nextBlockIndex
	b.nextBlockIndex++
	block := buildContentBlockStartEvent(b.textIndex)
	b.events = append(b.events, buildSSEEvent("content_block_start", block))
	b.textStarted = true
}

func (b *anthropicLegacyStreamBuilder) appendText(text string) {
	if text == "" {
		return
	}
	b.seenPayload = true
	b.ensureTextBlock()
	payload := map[string]any{
		"type":  "content_block_delta",
		"index": b.textIndex,
		"delta": map[string]any{
			"type": "text_delta",
			"text": text,
		},
	}
	b.events = append(b.events, buildSSEEvent("content_block_delta", payload))
}

func (b *anthropicLegacyStreamBuilder) appendToolDelta(id, name string, node gjson.Result) {
	b.ensureToolBlock(id, name)
	partial := extractToolPartial(node.Get("input"))
	if partial == "" {
		partial = strings.TrimSpace(node.Get("partial_json").String())
	}
	if partial == "" {
		return
	}

	b.seenPayload = true
	buf := b.ensureToolFragment(id)
	buf.WriteString(partial)
}

func (b *anthropicLegacyStreamBuilder) ensureToolBlock(id, name string) int {
	if idx, ok := b.toolIndexes[id]; ok {
		return idx
	}
	b.ensureMessageStart()
	index := b.nextBlockIndex
	b.nextBlockIndex++
	b.toolIndexes[id] = index
	b.toolNames[id] = name

	payload := map[string]any{
		"type":  "content_block_start",
		"index": index,
		"content_block": map[string]any{
			"type":  "tool_use",
			"id":    id,
			"name":  name,
			"input": map[string]any{},
		},
	}
	b.events = append(b.events, buildSSEEvent("content_block_start", payload))
	return index
}

func (b *anthropicLegacyStreamBuilder) ensureToolFragment(id string) *strings.Builder {
	if buf, ok := b.toolFragments[id]; ok {
		return buf
	}
	builder := &strings.Builder{}
	b.toolFragments[id] = builder
	return builder
}

func (b *anthropicLegacyStreamBuilder) stopTool(id string) {
	if idx, ok := b.toolIndexes[id]; ok && !b.toolStopped[id] {
		if buf, ok := b.toolFragments[id]; ok && buf.Len() > 0 {
			partialJSON := normalizeArguments(buf.String())
			if partialJSON == "" {
				partialJSON = buf.String()
			}
			payload := map[string]any{
				"type":  "content_block_delta",
				"index": idx,
				"delta": map[string]any{
					"type":         "input_json_delta",
					"partial_json": partialJSON,
				},
			}
			b.events = append(b.events, buildSSEEvent("content_block_delta", payload))
		}
		payload := map[string]any{
			"type":  "content_block_stop",
			"index": idx,
		}
		b.events = append(b.events, buildSSEEvent("content_block_stop", payload))
		b.toolStopped[id] = true
		delete(b.toolFragments, id)
	}
}

func (b *anthropicLegacyStreamBuilder) stopTextBlock() {
	if b.textStarted && !b.textStopped {
		payload := map[string]any{
			"type":  "content_block_stop",
			"index": b.textIndex,
		}
		b.events = append(b.events, buildSSEEvent("content_block_stop", payload))
		b.textStopped = true
	}
}

func (b *anthropicLegacyStreamBuilder) finalize() [][]byte {
	if !b.seenPayload {
		return nil
	}

	b.stopTextBlock()
	for id := range b.toolIndexes {
		b.stopTool(id)
	}

	stopReason := ""
	if b.stopReason != "" {
		stopReason = b.stopReason
	} else if len(b.toolIndexes) > 0 {
		stopReason = "tool_use"
	}
	delta := buildMessageDeltaEvent(stopReason, b.promptTokens, b.completionTokens)
	if b.followupPrompt {
		if deltaMap, ok := delta["delta"].(map[string]any); ok {
			deltaMap["followup_prompt"] = true
			deltaMap["stop_reason"] = "followup"
		}
	}
	b.events = append(b.events, buildSSEEvent("message_delta", delta))
	b.events = append(b.events, buildSSEEvent("message_stop", buildMessageStopEvent()))
	return b.events
}

func sanitizeStreamingTextChunk(text string) string {
	return sanitizeAssistantTextWithOptions(text, assistantTextSanitizeOptions{
		allowBlank:         true,
		collapseWhitespace: false,
		trimResult:         false,
		dropEmptyLines:     false,
	})
}

func extractToolPartial(input gjson.Result) string {
	if !input.Exists() {
		return ""
	}
	if input.Type == gjson.String {
		return input.String()
	}
	if input.Raw != "" {
		trimmed := strings.TrimSpace(input.Raw)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
