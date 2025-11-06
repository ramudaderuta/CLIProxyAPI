package kiro_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro"
	"github.com/stretchr/testify/assert"
)

// TestKiroSSEBufferLimit tests that SSE parsing handles large thinking blocks
// that would exceed the default bufio.Scanner buffer limit (64KB)
func TestKiroSSEBufferLimit(t *testing.T) {
	t.Run("Large thinking delta exceeds 64KB buffer", func(t *testing.T) {
		// Create a thinking block that exceeds 64KB (default bufio.Scanner limit)
		largeThinking := generateLargeThinkingContent(70000) // 70KB

		// Build SSE stream with the large thinking content
		sseData := buildLargeSSEStream(largeThinking)

		// Parse the stream - this should not truncate
		content, toolCalls := kiro.ParseResponse([]byte(sseData))

		// Verify parsing succeeded without truncation
		assert.NotEmpty(t, content, "Content should not be empty")
		assert.Empty(t, toolCalls, "Should have no tool calls")

		// Verify no truncation artifacts
		assert.NotContains(t, content, "bufio.Scanner: token too long", "Should not contain scanner error")
		assert.NotContains(t, content, `incomplete`, "Should not contain incomplete JSON")
	})

	t.Run("Multiple large deltas in sequence", func(t *testing.T) {
		// Test multiple large thinking deltas that together exceed limits
		chunks := []string{
			generateLargeThinkingContent(50000), // 50KB
			generateLargeThinkingContent(45000), // 45KB
			generateLargeThinkingContent(30000), // 30KB
		}

		sseData := buildSSEStreamWithMultipleLargeDeltas(chunks)

		content, toolCalls := kiro.ParseResponse([]byte(sseData))

		assert.NotEmpty(t, content, "Content should not be empty")
		assert.Empty(t, toolCalls, "Should have no tool calls")
	})

	t.Run("SSE event boundary handling", func(t *testing.T) {
		// Test that SSE event boundaries are properly handled
		// even with large data lines
		sseData := `data:{"type":"content_block_delta","delta":{"type":"thinking_delta","thinking":"` +
			strings.Repeat("A", 70000) + // 70KB of 'A's
			`"}}`

		content, toolCalls := kiro.ParseResponse([]byte(sseData))

		assert.NotEmpty(t, content, "Content should not be empty")
		assert.Empty(t, toolCalls, "Should have no tool calls")
	})

	t.Run("Mixed content with large thinking blocks", func(t *testing.T) {
		actualResponse := "Here is my final answer to your question."
		largeThinking := generateLargeThinkingContent(80000) // 80KB

		sseData := buildMixedSSEStreamWithLargeThinking(largeThinking, actualResponse)

		content, toolCalls := kiro.ParseResponse([]byte(sseData))

		// Should preserve the actual response
		assert.Contains(t, content, actualResponse, "Actual response should be preserved")
		assert.Empty(t, toolCalls, "Should have no tool calls")
	})
}

// TestKiroSSEEventAssembly tests that SSE events are properly assembled
// from multiple data lines according to SSE specification
func TestKiroSSEEventAssembly(t *testing.T) {
	t.Run("Multi-line SSE event assembly", func(t *testing.T) {
		// According to SSE spec, events are separated by blank lines
		// and data lines within an event should be concatenated
		sseData := `data:{"type":"content_block_start","content_block":{"type":"thinking"}}
` +
			`data:{"type":"content_block_delta","delta":{"type":"thinking_delta","thinking":"Part 1"}}
` +
			`data:{"type":"content_block_delta","delta":{"type":"thinking_delta","thinking":"Part 2"}}
` +
			`
` + // Event boundary
			`data:{"type":"content_block_stop"}
`

		content, toolCalls := kiro.ParseResponse([]byte(sseData))

		assert.NotEmpty(t, content, "Content should not be empty")
		assert.Empty(t, toolCalls, "Should have no tool calls")
	})
}

// Helper functions

func generateLargeThinkingContent(size int) string {
	var builder strings.Builder
	baseText := "This is detailed thinking content that demonstrates the AI's reasoning process. "

	for builder.Len() < size {
		builder.WriteString(baseText)
	}

	result := builder.String()
	if len(result) > size {
		result = result[:size]
	}
	return result
}

func buildLargeSSEStream(thinkingContent string) string {
	var builder strings.Builder

	// message_start
	builder.WriteString(`{"type":"message_start","message":{"id":"msg_123","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4-5","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":50,"output_tokens":0}}}`)
	builder.WriteString("\n")

	// content_block_start for thinking
	builder.WriteString(`{"type":"content_block_start","index":0,"content_block":{"type":"thinking","id":"thinking_1"}}`)
	builder.WriteString("\n")

	// Split large thinking content into multiple deltas to simulate streaming
	chunkSize := 16384 // 16KB chunks
	remaining := thinkingContent

	for len(remaining) > 0 {
		end := len(remaining)
		if end > chunkSize {
			end = chunkSize
		}

		chunk := remaining[:end]
		remaining = remaining[end:]

		// Escape the chunk for JSON
		escapedChunk := strings.ReplaceAll(chunk, `\`, `\\`)
		escapedChunk = strings.ReplaceAll(escapedChunk, `"`, `\"`)
		escapedChunk = strings.ReplaceAll(escapedChunk, "\n", `\n`)
		escapedChunk = strings.ReplaceAll(escapedChunk, "\t", `\t`)

		builder.WriteString(fmt.Sprintf(`{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"%s"}}`, escapedChunk))
		builder.WriteString("\n")
	}

	// content_block_stop for thinking
	builder.WriteString(`{"type":"content_block_stop","index":0}`)
	builder.WriteString("\n")

	// message_stop
	builder.WriteString(`{"type":"message_stop"}`)
	builder.WriteString("\n")

	return builder.String()
}

func buildSSEStreamWithMultipleLargeDeltas(chunks []string) string {
	var builder strings.Builder

	// message_start
	builder.WriteString(`{"type":"message_start","message":{"id":"msg_123","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4-5","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":50,"output_tokens":0}}}`)
	builder.WriteString("\n")

	// content_block_start for thinking
	builder.WriteString(`{"type":"content_block_start","index":0,"content_block":{"type":"thinking","id":"thinking_1"}}`)
	builder.WriteString("\n")

	// Add each chunk as a separate delta
	for _, chunk := range chunks {
		escapedChunk := strings.ReplaceAll(chunk, `\`, `\\`)
		escapedChunk = strings.ReplaceAll(escapedChunk, `"`, `\"`)
		escapedChunk = strings.ReplaceAll(escapedChunk, "\n", `\n`)
		escapedChunk = strings.ReplaceAll(escapedChunk, "\t", `\t`)

		builder.WriteString(fmt.Sprintf(`{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"%s"}}`, escapedChunk))
		builder.WriteString("\n")
	}

	// content_block_stop for thinking
	builder.WriteString(`{"type":"content_block_stop","index":0}`)
	builder.WriteString("\n")

	// message_stop
	builder.WriteString(`{"type":"message_stop"}`)
	builder.WriteString("\n")

	return builder.String()
}

func buildMixedSSEStreamWithLargeThinking(thinkingContent, actualResponse string) string {
	var builder strings.Builder

	// message_start
	builder.WriteString(`{"type":"message_start","message":{"id":"msg_123","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4-5","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":50,"output_tokens":0}}}`)
	builder.WriteString("\n")

	// content_block_start for thinking
	builder.WriteString(`{"type":"content_block_start","index":0,"content_block":{"type":"thinking","id":"thinking_1"}}`)
	builder.WriteString("\n")

	// Add large thinking content in chunks
	chunkSize := 16384
	remaining := thinkingContent

	for len(remaining) > 0 {
		end := len(remaining)
		if end > chunkSize {
			end = chunkSize
		}

		chunk := remaining[:end]
		remaining = remaining[end:]

		escapedChunk := strings.ReplaceAll(chunk, `\`, `\\`)
		escapedChunk = strings.ReplaceAll(escapedChunk, `"`, `\"`)
		escapedChunk = strings.ReplaceAll(escapedChunk, "\n", `\n`)

		builder.WriteString(fmt.Sprintf(`{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"%s"}}`, escapedChunk))
		builder.WriteString("\n")
	}

	// content_block_stop for thinking
	builder.WriteString(`{"type":"content_block_stop","index":0}`)
	builder.WriteString("\n")

	// content_block_start for actual text
	builder.WriteString(`{"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}`)
	builder.WriteString("\n")

	// content_block_delta for actual text
	escapedResponse := strings.ReplaceAll(actualResponse, `\`, `\\`)
	escapedResponse = strings.ReplaceAll(escapedResponse, `"`, `\"`)
	builder.WriteString(fmt.Sprintf(`{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"%s"}}`, escapedResponse))
	builder.WriteString("\n")

	// content_block_stop for text
	builder.WriteString(`{"type":"content_block_stop","index":1}`)
	builder.WriteString("\n")

	// message_stop
	builder.WriteString(`{"type":"message_stop"}`)
	builder.WriteString("\n")

	return builder.String()
}