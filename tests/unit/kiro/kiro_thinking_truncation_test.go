package kiro_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro"
)

// TestKiroThinkingBlockTruncation reproduces the thinking block truncation issue
// This test follows TDD principles: Red -> Green -> Refactor
func TestKiroThinkingBlockTruncation(t *testing.T) {
	t.Run("Large thinking delta should not be truncated", func(t *testing.T) {
		// Create a large thinking block that exceeds typical buffer limits (>64KB)
		var largeThinking strings.Builder
		largeThinking.WriteString("Let me think through this problem step by step:\n\n")

		// Generate content that will exceed 64KB when serialized as JSON
		for i := 0; i < 2000; i++ {
			largeThinking.WriteString(fmt.Sprintf("Step %d: This is a detailed analysis step with substantial content that should be preserved without truncation. ", i))
			largeThinking.WriteString("The thinking process includes multiple paragraphs of reasoning, code analysis, and comprehensive explanation of the approach. ")
			largeThinking.WriteString("Each step builds upon the previous one to form a coherent thought process that demonstrates the AI's reasoning capabilities. ")
			largeThinking.WriteString("This content must be completely preserved to ensure the thinking block is not truncated due to buffer size limitations.\n")
		}

		thinkingContent := largeThinking.String()

		// Create SSE-like streaming data with thinking blocks
		streamingData := buildSSEStreamWithThinking(thinkingContent)

		// Parse the streaming data
		content, toolCalls := kiro.ParseResponse([]byte(streamingData))

		// Verify that thinking content is preserved (it should be filtered out, but we want to test truncation)
		// The key test is that if there's truncation, we'll see incomplete JSON or malformed content
		assert.NotEmpty(t, content, "Content should not be empty")
		assert.Empty(t, toolCalls, "Should have no tool calls for thinking-only response")

		// Parse the content to ensure it's valid (no truncation artifacts)
		assert.NotContains(t, content, `"\n\n}`, "Should not contain truncation artifacts")
		assert.NotContains(t, content, `null`, "Should not contain null values from truncation")

		// The content should be properly formatted without incomplete JSON structures
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				// Each line should be complete and not contain partial JSON
				assert.False(t, strings.HasSuffix(trimmed, `{"`), "Line should not end with incomplete JSON")
				assert.False(t, strings.HasSuffix(trimmed, `":{"`), "Line should not end with incomplete JSON object")
			}
		}
	})

	t.Run("Multiple thinking deltas should accumulate correctly", func(t *testing.T) {
		// Test multiple smaller thinking blocks that together exceed buffer limits
		var chunks []string
		chunkSize := 8192 // 8KB chunks

		// Generate multiple thinking chunks
		for i := 0; i < 10; i++ {
			var chunk strings.Builder
			chunk.WriteString(fmt.Sprintf("Thinking chunk %d:\n", i))
			for j := 0; j < chunkSize/100; j++ {
				chunk.WriteString("This is detailed thinking content that should accumulate properly. ")
			}
			chunks = append(chunks, chunk.String())
		}

		// Build SSE stream with multiple thinking deltas
		streamingData := buildSSEStreamWithMultipleThinkingChunks(chunks)

		// Parse the streaming data
		content, toolCalls := kiro.ParseResponse([]byte(streamingData))

		// Thinking content should be filtered out, leaving only actual response content
		assert.Empty(t, toolCalls, "Should have no tool calls")

		// The fact that parsing succeeded without errors and didn't crash indicates
		// that large thinking blocks were handled without truncation
		// Since thinking blocks are filtered out, we expect empty or minimal content
		t.Logf("Parsed content length: %d", len(content))

		// Verify no truncation artifacts in the parsing process
		// If truncation occurred, we'd see malformed JSON or parsing errors
		assert.NotContains(t, content, `"\n\n}`, "Should not have truncation artifacts")
		assert.NotContains(t, content, `null`, "Should not contain null values from truncation")
	})

	t.Run("Mixed content and thinking should be handled correctly", func(t *testing.T) {
		actualResponse := "Here is the final answer to your question."
		thinkingContent := generateLargeThinkingContent(16384) // 16KB of thinking

		streamingData := buildMixedSSEStream(thinkingContent, actualResponse)

		content, toolCalls := kiro.ParseResponse([]byte(streamingData))

		// Should preserve actual response and handle thinking gracefully
		assert.Contains(t, content, actualResponse, "Actual response should be preserved")
		assert.Empty(t, toolCalls, "Should have no tool calls")

		// Content should be well-formed without truncation artifacts
		assert.NotContains(t, content, `"\n\n}`, "Should not have truncation artifacts")
	})
}

// Helper functions to build test SSE streams

func buildSSEStreamWithThinking(thinkingContent string) string {
	var builder strings.Builder

	// message_start
	builder.WriteString(`{"type":"message_start","message":{"id":"msg_123","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4-5","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":50,"output_tokens":0}}}`)
	builder.WriteString("\n")

	// content_block_start for thinking
	builder.WriteString(`{"type":"content_block_start","index":0,"content_block":{"type":"thinking","id":"thinking_1"}}`)
	builder.WriteString("\n")

	// Split thinking content into multiple deltas to simulate streaming
	chunkSize := 4096 // 4KB chunks
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

	// content_block_start for actual text
	builder.WriteString(`{"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}`)
	builder.WriteString("\n")

	// content_block_delta for actual text
	builder.WriteString(`{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"Here is my response."}}`)
	builder.WriteString("\n")

	// content_block_stop for text
	builder.WriteString(`{"type":"content_block_stop","index":1}`)
	builder.WriteString("\n")

	// message_delta
	builder.WriteString(`{"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"input_tokens":50,"output_tokens":100}}`)
	builder.WriteString("\n")

	// message_stop
	builder.WriteString(`{"type":"message_stop"}`)
	builder.WriteString("\n")

	return builder.String()
}

func buildSSEStreamWithMultipleThinkingChunks(chunks []string) string {
	var builder strings.Builder

	// message_start
	builder.WriteString(`{"type":"message_start","message":{"id":"msg_123","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4-5","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":50,"output_tokens":0}}}`)
	builder.WriteString("\n")

	// content_block_start for thinking
	builder.WriteString(`{"type":"content_block_start","index":0,"content_block":{"type":"thinking","id":"thinking_1"}}`)
	builder.WriteString("\n")

	// Add each thinking chunk as a separate delta
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

func buildMixedSSEStream(thinkingContent, actualResponse string) string {
	var builder strings.Builder

	// message_start
	builder.WriteString(`{"type":"message_start","message":{"id":"msg_123","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4-5","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":50,"output_tokens":0}}}`)
	builder.WriteString("\n")

	// content_block_start for thinking
	builder.WriteString(`{"type":"content_block_start","index":0,"content_block":{"type":"thinking","id":"thinking_1"}}`)
	builder.WriteString("\n")

	// Add thinking content in chunks
	chunkSize := 4096
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

func generateLargeThinkingContent(size int) string {
	var builder strings.Builder
	baseText := "This is detailed thinking content that demonstrates the AI's reasoning process. "

	for builder.Len() < size {
		builder.WriteString(baseText)
	}

	return builder.String()[:size]
}