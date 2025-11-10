package kiro_test

import (
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"

	kirotranslator "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestKiroExecutor_SSEFormatting_SimpleText tests that streaming responses are properly formatted as SSE events
func TestKiroExecutor_SSEFormatting_SimpleText(t *testing.T) {
	t.Parallel()
	chunks := kirotranslator.BuildAnthropicStreamingChunks("test-id", "claude-sonnet-4-5", 1234567890, "Hello, world!", []kirotranslator.OpenAIToolCall{}, 25, 5)

	require.Greater(t, len(chunks), 0, "Should produce chunks")

	events := parseSSEChunks(t, chunks)
	require.Equal(t, []string{
		"message_start",
		"content_block_start",
		"content_block_delta",
		"content_block_stop",
		"message_delta",
		"message_stop",
	}, eventNames(events))

	textDelta := events[2].Payload["delta"].(map[string]any)
	assert.Equal(t, "text_delta", textDelta["type"])
	assert.Equal(t, "Hello, world!", textDelta["text"])

	delta := events[4].Payload["delta"].(map[string]any)
	assert.Equal(t, "end_turn", delta["stop_reason"])
	assert.Nil(t, delta["stop_sequence"])

	usage := events[4].Payload["usage"].(map[string]any)
	assert.Equal(t, float64(25), usage["input_tokens"])
	assert.Equal(t, float64(5), usage["output_tokens"])
}

// TestKiroExecutor_SSEFormatting_WithToolCalls tests SSE formatting for responses with tool calls
func TestKiroExecutor_SSEFormatting_WithToolCalls(t *testing.T) {
	toolCalls := []kirotranslator.OpenAIToolCall{
		{
			ID:        "call_123",
			Name:      "test_function",
			Arguments: "{\"param\": \"value\"}",
		},
	}

	chunks := kirotranslator.BuildAnthropicStreamingChunks("test-id", "claude-sonnet-4-5", 1234567890, "", toolCalls, 30, 11)

	require.Greater(t, len(chunks), 0, "Should produce chunks for tool calls")

	events := parseSSEChunks(t, chunks)
	require.Equal(t, []string{
		"message_start",
		"content_block_start",
		"content_block_delta",
		"content_block_stop",
		"message_delta",
		"message_stop",
	}, eventNames(events))

	toolStart := events[1].Payload["content_block"].(map[string]any)
	assert.Equal(t, "tool_use", toolStart["type"])
	assert.Equal(t, "test_function", toolStart["name"])
	assert.Equal(t, "call_123", toolStart["id"])

	toolDelta := events[2].Payload["delta"].(map[string]any)
	assert.Equal(t, "input_json_delta", toolDelta["type"])
	assert.Equal(t, `{"param":"value"}`, toolDelta["partial_json"])

	delta := events[4].Payload["delta"].(map[string]any)
	assert.Equal(t, "tool_use", delta["stop_reason"])
	assert.Nil(t, delta["stop_sequence"])
}

// TestKiroExecutor_SSEFormatting_EmptyContent tests SSE formatting for empty responses
func TestKiroExecutor_SSEFormatting_EmptyContent(t *testing.T) {
	chunks := kirotranslator.BuildAnthropicStreamingChunks("test-id", "claude-sonnet-4-5", 1234567890, "", []kirotranslator.OpenAIToolCall{}, 20, 0)

	require.Greater(t, len(chunks), 0, "Should produce chunks even for empty content")

	events := parseSSEChunks(t, chunks)
	require.Equal(t, []string{
		"message_start",
		"message_delta",
		"message_stop",
	}, eventNames(events))

	delta := events[1].Payload["delta"].(map[string]any)
	assert.Equal(t, "end_turn", delta["stop_reason"])

	usage := events[1].Payload["usage"].(map[string]any)
	assert.Equal(t, float64(20), usage["input_tokens"])
	assert.Equal(t, float64(0), usage["output_tokens"])
}

// TestKiroExecutor_VerifySSEFormatRequirement verifies what the SSE format should look like
func TestKiroExecutor_VerifySSEFormatRequirement(t *testing.T) {
	// This test documents the EXPECTED SSE format for reference
	// This test should PASS as it's documenting the requirement

	expectedSSEFormat := `event: message_start
data: {"type":"message_start","message":{"id":"msg_test123","type":"message","role":"assistant","model":"claude-sonnet-4-5","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":0,"output_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello!"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"input_tokens":12,"output_tokens":3}}

event: message_stop
data: {"type":"message_stop"}`

	// Verify this is proper SSE format
	lines := strings.Split(expectedSSEFormat, "\n")

	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue // Skip empty lines
		}

		if strings.HasPrefix(line, "event: ") {
			// Event line should be followed by data line
			assert.Less(t, i+1, len(lines), "Event line should not be last line")
			nextLine := lines[i+1]
			assert.True(t, strings.HasPrefix(nextLine, "data: "), "Event line should be followed by data line")
		} else if strings.HasPrefix(line, "data: ") {
			// Data line should be valid JSON
			jsonPart := strings.TrimPrefix(line, "data: ")
			var jsonData any
			err := json.Unmarshal([]byte(jsonPart), &jsonData)
			assert.NoError(t, err, "Data line should contain valid JSON: %s", jsonPart)
		}
	}

	t.Log("Expected SSE format documented correctly")
}

func TestConvertKiroStreamToAnthropic_LegacyPayload(t *testing.T) {
	raw := strings.Join([]string{
		"event: content_block_delta",
		`data: {"content":"content-type\u0007\u0000\u0010application/json"}`,
		"",
		"event: content_block_delta",
		`data: {"content":"I'll get the latest weather update."}`,
		"",
		"event: message_stop",
		`data: {"type":"message_stop"}`,
	}, "\n")

	chunks := kirotranslator.ConvertKiroStreamToAnthropic([]byte(raw), "claude-sonnet-4-5", 10, 5)
	require.NotEmpty(t, chunks, "legacy SSE payload should be converted into Anthropic chunks")

	events := parseSSEChunks(t, chunks)
	texts := make([]string, 0, len(events))
	for _, ev := range events {
		if ev.Event != "content_block_delta" {
			continue
		}
		if delta, ok := ev.Payload["delta"].(map[string]any); ok && delta["type"] == "text_delta" {
			texts = append(texts, delta["text"].(string))
		}
	}
	require.NotEmpty(t, texts, "expected at least one text delta")
	assert.Equal(t, "I'll get the latest weather update.", texts[len(texts)-1])
	for _, text := range texts {
		assert.NotContains(t, text, "content-type", "protocol noise should be stripped")
	}
}

func TestConvertKiroStreamToAnthropic_ToolChunks(t *testing.T) {
	raw := strings.Join([]string{
		"event: content_block_delta",
		`data: {"name":"get_weather","toolUseId":"call_1","input":"{\"city\""}`,
		"",
		"event: content_block_delta",
		`data: {"name":"get_weather","toolUseId":"call_1","input":": \"Tokyo\"}"}`,
		"",
		"event: content_block_delta",
		`data: {"name":"get_weather","toolUseId":"call_1","stop":true}`,
		"",
	}, "\n")

	chunks := kirotranslator.ConvertKiroStreamToAnthropic([]byte(raw), "claude-sonnet-4-5", 12, 3)
	require.NotEmpty(t, chunks, "tool SSE payload should be converted")

	events := parseSSEChunks(t, chunks)
	found := false
	for _, ev := range events {
		if ev.Event != "content_block_delta" {
			continue
		}
		delta, ok := ev.Payload["delta"].(map[string]any)
		if !ok || delta["type"] != "input_json_delta" {
			continue
		}
		actualJSON := canonicalJSON(delta["partial_json"].(string))
		assert.Equal(t, `{"city":"Tokyo"}`, actualJSON)
		found = true
	}
	assert.True(t, found, "expected tool delta event")
}

func TestNormalizeKiroStreamPayload_NoOpJSON(t *testing.T) {
	raw := []byte(`{"hello":"world"}`)
	out := kirotranslator.NormalizeKiroStreamPayload(raw)
	assert.Equal(t, string(raw), string(out))
}

func TestNormalizeKiroStreamPayload_DecodesEventStream(t *testing.T) {
	payload := strings.Join([]string{
		":event-type toolUseEvent",
		":content-type application/json",
		":message-type event",
		`{"name":"ping","toolUseId":"call_ping","input":"{\"ok\":true}"}`,
	}, "\n")
	raw := buildEventStreamChunk(payload + "\n")

	decoded := kirotranslator.NormalizeKiroStreamPayload(raw)
	assert.Equal(t, `{"name":"ping","toolUseId":"call_ping","input":"{\"ok\":true}"}`, string(decoded))
}

func TestNormalizeKiroStreamPayload_StripsMeteringEvents(t *testing.T) {
	payload := strings.Join([]string{
		":event-type meteringEvent",
		":content-type application/json",
		":message-type event",
		`{"unit":"credit","unitPlural":"credits","usage":0.12}`,
	}, "\n")
	raw := buildEventStreamChunk(payload + "\n")

	decoded := kirotranslator.NormalizeKiroStreamPayload(raw)
	assert.Equal(t, "", string(decoded))
}

func TestConvertKiroStreamToAnthropic_EventStreamPayload(t *testing.T) {
	toolStart := strings.Join([]string{
		":event-type toolUseEvent",
		":content-type application/json",
		":message-type event",
		`{"name":"get_weather","toolUseId":"call_42","input":"{\"city\""}`,
	}, "\n")
	toolMid := strings.Join([]string{
		":event-type toolUseEvent",
		":content-type application/json",
		":message-type event",
		`{"name":"get_weather","toolUseId":"call_42","input":": \"Seattle\"}"}`,
	}, "\n")
	toolStop := strings.Join([]string{
		":event-type toolUseEvent",
		":content-type application/json",
		":message-type event",
		`{"name":"get_weather","toolUseId":"call_42","stop":true}`,
	}, "\n")
	raw := append(buildEventStreamChunk(toolStart+"\n"), buildEventStreamChunk(toolMid+"\n")...)
	raw = append(raw, buildEventStreamChunk(toolStop+"\n")...)

	chunks := kirotranslator.ConvertKiroStreamToAnthropic(raw, "claude-sonnet-4-5", 0, 0)
	require.NotEmpty(t, chunks, "AWS event stream payload should be converted")

	events := parseSSEChunks(t, chunks)
	foundTool := false
	for _, ev := range events {
		if ev.Event != "content_block_delta" {
			continue
		}
		delta, ok := ev.Payload["delta"].(map[string]any)
		if !ok {
			continue
		}
		if delta["type"] == "input_json_delta" {
			assert.Equal(t, `{"city":"Seattle"}`, canonicalJSON(delta["partial_json"].(string)))
			foundTool = true
		}
	}
	assert.True(t, foundTool, "expected tool delta after decoding event stream")
}

func TestConvertKiroStreamToAnthropic_IgnoresMeteringEvents(t *testing.T) {
	raw := strings.Join([]string{
		`data: {"content":"Working..."}`,
		"",
		`data: {"unit":"credit","unitPlural":"credits","usage":0.02}`,
		"",
		`data: {"name":"get_weather","toolUseId":"call_meter","input":"{\"city\":\"Paris\"}"}`,
		"",
		`data: {"name":"get_weather","toolUseId":"call_meter","stop":true}`,
	}, "\n")

	chunks := kirotranslator.ConvertKiroStreamToAnthropic([]byte(raw), "claude-sonnet-4-5", 0, 0)
	events := parseSSEChunks(t, chunks)
	for _, ev := range events {
		if ev.Event != "content_block_delta" {
			continue
		}
		if delta, ok := ev.Payload["delta"].(map[string]any); ok && delta["type"] == "text_delta" {
			assert.NotContains(t, delta["text"], "credits")
			assert.NotContains(t, delta["text"], "usage")
		}
	}
}

// TestKiroExecutor_StreamingChunkOrder ensures text is emitted as a single block following reference ordering.
func TestKiroExecutor_StreamingChunkOrder(t *testing.T) {
	content := "Hello! How are you today?"
	chunks := kirotranslator.BuildAnthropicStreamingChunks("test-id", "claude-sonnet-4-5", 1234567890, content, []kirotranslator.OpenAIToolCall{}, 18, 7)

	events := parseSSEChunks(t, chunks)
	require.Equal(t, []string{
		"message_start",
		"content_block_start",
		"content_block_delta",
		"content_block_stop",
		"message_delta",
		"message_stop",
	}, eventNames(events))

	textDelta := events[2].Payload["delta"].(map[string]any)
	assert.Equal(t, content, textDelta["text"])
}

func TestBuildAnthropicStreamingChunksMatchReference(t *testing.T) {
	t.Parallel()
	model := "claude-sonnet-4-5"

	cases := []struct {
		name             string
		content          string
		toolCalls        []kirotranslator.OpenAIToolCall
		promptTokens     int64
		completionTokens int64
	}{
		{
			name:             "plain_text",
			content:          "All set.",
			promptTokens:     12,
			completionTokens: 3,
		},
		{
			name:    "single_tool_with_text",
			content: "Calling weather.",
			toolCalls: []kirotranslator.OpenAIToolCall{{
				ID:        "toolu_weather",
				Name:      "get_weather",
				Arguments: `{"city":"Tokyo","unit":"°C"}`,
			}},
			promptTokens:     25,
			completionTokens: 8,
		},
		{
			name:    "multiple_tools_and_text",
			content: "Plan finished.",
			toolCalls: []kirotranslator.OpenAIToolCall{
				{ID: "toolu_plan", Name: "Task", Arguments: `{"goal":"audit","subagent_type":"plan"}`},
				{ID: "toolu_exit", Name: "ExitPlanMode", Arguments: `{}`},
			},
			promptTokens:     40,
			completionTokens: 15,
		},
		{
			name:             "no_tool_empty",
			content:          "",
			promptTokens:     5,
			completionTokens: 0,
		},
		{
			name: "tool_only",
			toolCalls: []kirotranslator.OpenAIToolCall{{
				ID:        "toolu_logs",
				Name:      "FetchLogs",
				Arguments: `{"since":"1h"}`,
			}},
			promptTokens:     10,
			completionTokens: 4,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			actualChunks := kirotranslator.BuildAnthropicStreamingChunks("chatcmpl_ref", model, 0, tc.content, tc.toolCalls, tc.promptTokens, tc.completionTokens)
			actual := normalizeEvents(parseSSEChunks(t, actualChunks))
			expected := normalizeEvents(buildReferenceEvents(model, tc.content, tc.toolCalls, tc.promptTokens, tc.completionTokens))
			compareEventSequences(t, expected, actual)
		})
	}
}

func TestConvertKiroStreamToAnthropic_LongArgumentsMerged(t *testing.T) {
	raw := strings.Join([]string{
		`data: {"name":"search","toolUseId":"call_merge","input":"{"}`,
		"",
		`data: {"name":"search","toolUseId":"call_merge","input":"\"query\": \"status\""}`,
		"",
		`data: {"name":"search","toolUseId":"call_merge","input":", \"limit\": "}`,
		"",
		`data: {"name":"search","toolUseId":"call_merge","input":"5"}`,
		"",
		`data: {"name":"search","toolUseId":"call_merge","input":"}"}`,
		"",
		`data: {"name":"search","toolUseId":"call_merge","stop":true}`,
	}, "\n")

	chunks := kirotranslator.ConvertKiroStreamToAnthropic([]byte(raw), "claude-sonnet-4-5", 0, 0)
	events := parseSSEChunks(t, chunks)
	require.Equal(t, "content_block_delta", events[2].Event)

	toolDelta := events[2].Payload["delta"].(map[string]any)
	actualJSON := canonicalJSON(toolDelta["partial_json"].(string))
	assert.Equal(t, `{"limit":5,"query":"status"}`, actualJSON)
}

func TestConvertKiroStreamToAnthropic_CrossChunkSpacePreservation_SpaceAtEndOfChunk(t *testing.T) {
	raw := strings.Join([]string{
		`data: {"content":"I'll use "}`,
		"",
		`data: {"content":"TDD workflows"}`,
	}, "\n")

	chunks := kirotranslator.ConvertKiroStreamToAnthropic([]byte(raw), "claude-sonnet-4-5", 0, 0)
	require.NotEmpty(t, chunks, "expected SSE chunks from converted Kiro stream")

	var builder strings.Builder
	for _, ev := range parseSSEChunks(t, chunks) {
		if ev.Event != "content_block_delta" {
			continue
		}
		delta, ok := ev.Payload["delta"].(map[string]any)
		if !ok || delta["type"] != "text_delta" {
			continue
		}
		builder.WriteString(delta["text"].(string))
	}

	combined := builder.String()
	assert.Contains(t, combined, "I'll use TDD workflows")
	assert.NotContains(t, combined, "I'll useTDD")
}

func TestConvertKiroStreamToAnthropic_CrossChunkSpacePreservation_SpaceOnlyChunk(t *testing.T) {
	// Ensure that a space at a chunk boundary is preserved and not stripped by
	// Kiro stream normalization / conversion logic.
	// This matches patterns seen in real Kiro outputs like "I'll useTDD".

	raw := strings.Join([]string{
		`data: {"content":"I'll use"}`,
		"",
		// Space-only content chunk that must be preserved
		`data: {"content":" "}`,
		"",
		`data: {"content":"TDD workflows"}`,
	}, "\n")

	chunks := kirotranslator.ConvertKiroStreamToAnthropic([]byte(raw), "claude-sonnet-4-5", 0, 0)
	require.NotEmpty(t, chunks, "expected SSE chunks from converted Kiro stream")

	// Collect all text_delta fragments into a single string.
	var builder strings.Builder
	for _, ev := range parseSSEChunks(t, chunks) {
		if ev.Event != "content_block_delta" {
			continue
		}
		delta, ok := ev.Payload["delta"].(map[string]any)
		if !ok || delta["type"] != "text_delta" {
			continue
		}
		builder.WriteString(delta["text"].(string))
	}

	combined := builder.String()
	assert.Contains(t, combined, "I'll use TDD workflows")
	assert.NotContains(t, combined, "I'll useTDD", "space between chunks must not be dropped")
}

func TestConvertKiroStreamToAnthropic_CrossChunkSpacePreservation_SpaceAtStartOfNextChunk(t *testing.T) {
	raw := strings.Join([]string{
		`data: {"content":"I'll use"}`,
		"",
		`data: {"content":" TDD workflows"}`,
	}, "\n")

	chunks := kirotranslator.ConvertKiroStreamToAnthropic([]byte(raw), "claude-sonnet-4-5", 0, 0)
	require.NotEmpty(t, chunks, "expected SSE chunks from converted Kiro stream")

	var builder strings.Builder
	for _, ev := range parseSSEChunks(t, chunks) {
		if ev.Event != "content_block_delta" {
			continue
		}
		delta, ok := ev.Payload["delta"].(map[string]any)
		if !ok || delta["type"] != "text_delta" {
			continue
		}
		builder.WriteString(delta["text"].(string))
	}

	combined := builder.String()
	assert.Contains(t, combined, "I'll use TDD workflows")
	assert.NotContains(t, combined, "I'll useTDD")
}

func TestConvertKiroStreamToAnthropic_FollowupPromptFlag(t *testing.T) {
	raw := strings.Join([]string{
		`data: {"content":"Need more info","followupPrompt":true}`,
	}, "\n")

	chunks := kirotranslator.ConvertKiroStreamToAnthropic([]byte(raw), "claude-sonnet-4-5", 0, 2)
	events := parseSSEChunks(t, chunks)
	require.Equal(t, "message_delta", events[len(events)-2].Event)

	delta := events[len(events)-2].Payload["delta"].(map[string]any)
	assert.Equal(t, true, delta["followup_prompt"])
	assert.Equal(t, "followup", delta["stop_reason"])
}

func TestConvertKiroStreamToAnthropic_StopReasonOverrides(t *testing.T) {
	t.Run("canceled", func(t *testing.T) {
		raw := strings.Join([]string{
			`event: content_block_delta`,
			`data: {"content":"Partial response"}`,
			"",
			`event: message_delta`,
			`data: {"delta":{"stop_reason":"canceled"}}`,
		}, "\n")

		chunks := kirotranslator.ConvertKiroStreamToAnthropic([]byte(raw), "claude-sonnet-4-5", 0, 0)
		events := parseSSEChunks(t, chunks)
		require.Equal(t, "message_delta", events[len(events)-2].Event)
		delta := events[len(events)-2].Payload["delta"].(map[string]any)
		assert.Equal(t, "canceled", delta["stop_reason"])
	})

	t.Run("max_tokens", func(t *testing.T) {
		raw := strings.Join([]string{
			`event: content_block_delta`,
			`data: {"content":"Streaming..."}`,
			"",
			`event: message_delta`,
			`data: {"delta":{"stop_reason":"max_tokens"}}`,
		}, "\n")

		chunks := kirotranslator.ConvertKiroStreamToAnthropic([]byte(raw), "claude-sonnet-4-5", 0, 0)
		events := parseSSEChunks(t, chunks)
		delta := events[len(events)-2].Payload["delta"].(map[string]any)
		assert.Equal(t, "max_tokens", delta["stop_reason"])
	})

	t.Run("fallback", func(t *testing.T) {
		raw := strings.Join([]string{
			`data: {"content":"Done."}`,
		}, "\n")
		chunks := kirotranslator.ConvertKiroStreamToAnthropic([]byte(raw), "claude-sonnet-4-5", 0, 1)
		events := parseSSEChunks(t, chunks)
		delta := events[len(events)-2].Payload["delta"].(map[string]any)
		assert.Equal(t, "end_turn", delta["stop_reason"])
	})
}

// Helpers --------------------------------------------------------------------

type parsedEvent struct {
	Event   string
	Payload map[string]any
}

func buildEventStreamChunk(payload string) []byte {
	data := []byte(payload)
	totalLen := 12 + len(data) // prelude (8) + payload + CRC (4)
	buf := make([]byte, totalLen)
	binary.BigEndian.PutUint32(buf[0:4], uint32(totalLen))
	// header length stays zero, trailing 4 CRC bytes already zeroed
	copy(buf[8:], data)
	return buf
}

func parseSSEChunks(t *testing.T, chunks [][]byte) []parsedEvent {
	t.Helper()
	events := make([]parsedEvent, 0, len(chunks))
	for _, chunk := range chunks {
		text := strings.TrimSpace(string(chunk))
		if text == "" {
			continue
		}
		lines := strings.SplitN(text, "\n", 2)
		if len(lines) < 2 {
			continue
		}
		eventType := strings.TrimSpace(strings.TrimPrefix(lines[0], "event:"))
		dataLine := strings.TrimSpace(strings.TrimPrefix(lines[1], "data:"))
		if dataLine == "" {
			events = append(events, parsedEvent{Event: eventType, Payload: map[string]any{}})
			continue
		}
		var payload map[string]any
		require.NoError(t, json.Unmarshal([]byte(dataLine), &payload))
		events = append(events, parsedEvent{Event: eventType, Payload: payload})
	}
	return events
}

func eventNames(events []parsedEvent) []string {
	names := make([]string, len(events))
	for i, ev := range events {
		names[i] = ev.Event
	}
	return names
}

func buildReferenceEvents(model, content string, toolCalls []kirotranslator.OpenAIToolCall, promptTokens, completionTokens int64) []parsedEvent {
	events := []parsedEvent{
		{
			Event: "message_start",
			Payload: map[string]any{
				"type": "message_start",
				"message": map[string]any{
					"id":            "ref_message",
					"type":          "message",
					"role":          "assistant",
					"model":         model,
					"content":       []any{},
					"stop_reason":   nil,
					"stop_sequence": nil,
					"usage": map[string]any{
						"input_tokens":  float64(0),
						"output_tokens": float64(0),
					},
				},
			},
		},
	}

	for idx, call := range toolCalls {
		index := float64(idx)
		events = append(events, parsedEvent{
			Event: "content_block_start",
			Payload: map[string]any{
				"type":  "content_block_start",
				"index": index,
				"content_block": map[string]any{
					"type":  "tool_use",
					"id":    call.ID,
					"name":  call.Name,
					"input": map[string]any{},
				},
			},
		})
		events = append(events, parsedEvent{
			Event: "content_block_delta",
			Payload: map[string]any{
				"type":  "content_block_delta",
				"index": index,
				"delta": map[string]any{
					"type":         "input_json_delta",
					"partial_json": canonicalJSON(call.Arguments),
				},
			},
		})
		events = append(events, parsedEvent{
			Event: "content_block_stop",
			Payload: map[string]any{
				"type":  "content_block_stop",
				"index": index,
			},
		})
	}

	if strings.TrimSpace(content) != "" {
		index := float64(len(toolCalls))
		events = append(events, parsedEvent{
			Event: "content_block_start",
			Payload: map[string]any{
				"type":  "content_block_start",
				"index": index,
				"content_block": map[string]any{
					"type": "text",
					"text": "",
				},
			},
		})
		events = append(events, parsedEvent{
			Event: "content_block_delta",
			Payload: map[string]any{
				"type":  "content_block_delta",
				"index": index,
				"delta": map[string]any{
					"type": "text_delta",
					"text": content,
				},
			},
		})
		events = append(events, parsedEvent{
			Event: "content_block_stop",
			Payload: map[string]any{
				"type":  "content_block_stop",
				"index": index,
			},
		})
	}

	stopReason := "end_turn"
	if len(toolCalls) > 0 {
		stopReason = "tool_use"
	} else if strings.TrimSpace(content) == "" {
		stopReason = "end_turn"
	}

	events = append(events, parsedEvent{
		Event: "message_delta",
		Payload: map[string]any{
			"type": "message_delta",
			"delta": map[string]any{
				"stop_reason":   stopReason,
				"stop_sequence": nil,
			},
			"usage": map[string]any{
				"input_tokens":  float64(promptTokens),
				"output_tokens": float64(completionTokens),
			},
		},
	})
	events = append(events, parsedEvent{
		Event:   "message_stop",
		Payload: map[string]any{"type": "message_stop"},
	})
	return events
}

func compareEventSequences(t *testing.T, expected, actual []parsedEvent) {
	t.Helper()
	require.Equal(t, len(expected), len(actual))
	for i := range expected {
		require.Equal(t, expected[i].Event, actual[i].Event, "event mismatch at position %d", i)
		require.Equal(t, expected[i].Payload, actual[i].Payload, "payload mismatch at position %d", i)
	}
}

func normalizeEvents(events []parsedEvent) []parsedEvent {
	normalized := make([]parsedEvent, len(events))
	for i, ev := range events {
		normalized[i] = normalizeEvent(ev)
	}
	return normalized
}

func normalizeEvent(ev parsedEvent) parsedEvent {
	payload := deepCopy(ev.Payload).(map[string]any)
	switch ev.Event {
	case "message_start":
		if msg, ok := payload["message"].(map[string]any); ok {
			delete(msg, "id")
		}
	case "content_block_delta":
		if delta, ok := payload["delta"].(map[string]any); ok {
			if delta["type"] == "input_json_delta" {
				if partial, ok := delta["partial_json"].(string); ok {
					delta["partial_json"] = canonicalJSON(partial)
				}
			}
		}
	}
	return parsedEvent{Event: ev.Event, Payload: payload}
}

func canonicalJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var obj any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return raw
	}
	data, err := json.Marshal(obj)
	if err != nil {
		return raw
	}
	return string(data)
}

func deepCopy(value any) any {
	switch v := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(v))
		for key, val := range v {
			result[key] = deepCopy(val)
		}
		return result
	case []any:
		result := make([]any, len(v))
		for i, val := range v {
			result[i] = deepCopy(val)
		}
		return result
	default:
		return v
	}
}
