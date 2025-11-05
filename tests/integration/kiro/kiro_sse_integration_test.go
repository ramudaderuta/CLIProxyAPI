//go:build integration
// +build integration

package kiro_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/runtime/executor"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/router-for-me/CLIProxyAPI/v6/tests/shared"
)

// TestKiroExecutor_Integration_SSEFormatting tests the complete SSE formatting in the executor
func TestKiroExecutor_Integration_SSEFormatting(t *testing.T) {
	fixtures := testutil.NewKiroTestFixtures()
	cfg := &config.Config{}
	exec := executor.NewKiroExecutor(cfg)
	auth := fixtures.NewTestAuth(nil, map[string]string{"region": "ap-southeast-1"})

	// Mock Kiro API response with SSE format
	kiroSSEResponse := `event: content_block_delta
data: {"content":"Hello from Kiro","followupPrompt":false}

event: content_block_delta
data: {"name":"test_function","toolUseId":"call_123","input":{"param":"value"},"stop":true}

event: content_block_delta
data: {"content":"Response complete","followupPrompt":false}

event: message_stop
data: {"type":"message_stop"}
`

	rt := testutil.RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte(kiroSSEResponse))),
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		}, nil
	})

	ctx := fixtures.WithRoundTripper(context.Background(), rt)
	req := cliproxyexecutor.Request{
		Model:   "claude-sonnet-4-5",
		Payload: fixtures.AnthropicChatPayload(t, []map[string]any{{"role": "user", "content": "Hello!"}}, nil),
	}

	stream, err := exec.ExecuteStream(ctx, auth, req, cliproxyexecutor.Options{})
	require.NoError(t, err, "ExecuteStream should not return error")

	var chunks []cliproxyexecutor.StreamChunk
	for chunk := range stream {
		if chunk.Err != nil {
			t.Fatalf("received chunk error: %v", chunk.Err)
		}
		chunks = append(chunks, chunk)
	}

	require.Greater(t, len(chunks), 0, "Should receive streaming chunks")

	// Concatenate all chunks to form complete SSE response
	var fullResponse strings.Builder
	for _, chunk := range chunks {
		fullResponse.Write(chunk.Payload)
	}
	responseStr := fullResponse.String()

	// Verify SSE format
	lines := strings.Split(responseStr, "\n")
	hasEventLines := false
	hasDataLines := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "event: ") {
			hasEventLines = true
			assert.NotEmpty(t, strings.TrimPrefix(line, "event: "), "Event type should not be empty")
		} else if strings.HasPrefix(line, "data: ") {
			hasDataLines = true
			jsonPart := strings.TrimPrefix(line, "data: ")
			var jsonData any
			assert.NoError(t, json.Unmarshal([]byte(jsonPart), &jsonData), "Data should be valid JSON: %s", jsonPart)
		}
	}

	assert.True(t, hasEventLines, "Response should contain event lines")
	assert.True(t, hasDataLines, "Response should contain data lines")

	// Verify proper SSE event structure
	assert.Contains(t, responseStr, "event: message_start", "Should contain message_start event")
	assert.Contains(t, responseStr, "data: {\"message\":", "Should contain message_start data")
	assert.Contains(t, responseStr, "event: message_stop", "Should contain message_stop event")

	// CRITICAL BUG FIXES: Verify stop_sequence is null per Anthropic spec
	assert.Contains(t, responseStr, `"stop_sequence":null`, "stop_sequence should be null per Anthropic spec")
	assert.NotContains(t, responseStr, `"stop_sequence":"end_turn"`, "stop_sequence should not be 'end_turn'")
	assert.NotContains(t, responseStr, `"stop_sequence":"tool_use"`, "stop_sequence should not be 'tool_use'")

	// CRITICAL BUG FIXES: Verify output_tokens is calculated (not hardcoded 0)
	assert.Regexp(t, `"output_tokens":[1-9][0-9]*`, responseStr, "output_tokens should be calculated, not 0")
}

// TestKiroExecutor_Integration_SSEFormatConsistency tests that SSE format is consistent with iflow provider
func TestKiroExecutor_Integration_SSEFormatConsistency(t *testing.T) {
	fixtures := testutil.NewKiroTestFixtures()
	cfg := &config.Config{}
	exec := executor.NewKiroExecutor(cfg)
	auth := fixtures.NewTestAuth(nil, nil)

	// Mock a simple response
	kiroResponse := `data: {"content":"Simple response"}
`

	rt := testutil.RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte(kiroResponse))),
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		}, nil
	})

	ctx := fixtures.WithRoundTripper(context.Background(), rt)
	req := cliproxyexecutor.Request{
		Model:   "claude-sonnet-4-5",
		Payload: fixtures.AnthropicChatPayload(t, []map[string]any{{"role": "user", "content": "Test"}}, nil),
	}

	stream, err := exec.ExecuteStream(ctx, auth, req, cliproxyexecutor.Options{})
	require.NoError(t, err)

	// Collect all chunks
	var allChunks []byte
	for chunk := range stream {
		if chunk.Err != nil {
			t.Fatalf("received chunk error: %v", chunk.Err)
		}
		allChunks = append(allChunks, chunk.Payload...)
	}

	responseStr := string(allChunks)

	// Verify SSE format consistency
	eventDataPairs := parseSSEEvents(responseStr)
	require.Greater(t, len(eventDataPairs), 0, "Should have at least one SSE event")

	for eventType, data := range eventDataPairs {
		// Each event should have valid JSON data
		assert.NotEmpty(t, eventType, "Event type should not be empty")
		assert.NotEmpty(t, data, "Event data should not be empty")

		var jsonData any
		assert.NoError(t, json.Unmarshal([]byte(data), &jsonData), "Event data should be valid JSON")
	}

	// Check for proper SSE structure
	assert.True(t, strings.Contains(responseStr, "event:"), "Should contain event lines")
	assert.True(t, strings.Contains(responseStr, "data:"), "Should contain data lines")
	assert.True(t, strings.Contains(responseStr, "\n\n"), "Should contain proper SSE line endings")
}

// TestKiroExecutor_Integration_SSEPerformance tests performance of SSE formatting
func TestKiroExecutor_Integration_SSEPerformance(t *testing.T) {
	fixtures := testutil.NewKiroTestFixtures()
	cfg := &config.Config{}
	exec := executor.NewKiroExecutor(cfg)
	auth := fixtures.NewTestAuth(nil, nil)

	// Mock response with moderate content
	kiroResponse := `data: {"content":"This is a test response with some content to process"}
`

	rt := testutil.RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte(kiroResponse))),
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		}, nil
	})

	ctx := fixtures.WithRoundTripper(context.Background(), rt)
	req := cliproxyexecutor.Request{
		Model:   "claude-sonnet-4-5",
		Payload: fixtures.OpenAIChatPayload(t, []map[string]any{{"role": "user", "content": "Performance test"}}, nil),
	}

	// Measure SSE formatting performance
	start := time.Now()
	stream, err := exec.ExecuteStream(ctx, auth, req, cliproxyexecutor.Options{})
	require.NoError(t, err)

	// Collect all chunks
	var chunkCount int
	for chunk := range stream {
		if chunk.Err != nil {
			t.Fatalf("received chunk error: %v", chunk.Err)
		}
		chunkCount++
	}
	duration := time.Since(start)

	// Performance assertions
	assert.Less(t, duration, 100*time.Millisecond, "SSE formatting should be fast (<100ms)")
	assert.Greater(t, chunkCount, 0, "Should receive chunks")
	assert.Less(t, chunkCount, 20, "Should not produce excessive number of chunks")

	t.Logf("SSE formatting: %d chunks in %v (%.2fms per chunk)",
		chunkCount, duration, float64(duration.Nanoseconds())/float64(chunkCount)/1e6)
}

// parseSSEEvents parses SSE response into event->data mapping
func parseSSEEvents(sseResponse string) map[string]string {
	events := make(map[string]string)
	lines := strings.Split(sseResponse, "\n")

	var currentEvent string
	var currentData strings.Builder

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "event: ") {
			// Save previous event if exists
			if currentEvent != "" && currentData.Len() > 0 {
				events[currentEvent] = currentData.String()
			}
			// Start new event
			currentEvent = strings.TrimPrefix(line, "event: ")
			currentData.Reset()
		} else if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if currentData.Len() > 0 {
				currentData.WriteString(" ")
			}
			currentData.WriteString(data)
		} else if line == "" {
			// End of event
			if currentEvent != "" && currentData.Len() > 0 {
				events[currentEvent] = currentData.String()
			}
			currentEvent = ""
			currentData.Reset()
		}
	}

	// Handle last event if no trailing newline
	if currentEvent != "" && currentData.Len() > 0 {
		events[currentEvent] = currentData.String()
	}

	return events
}

// TestKiroExecutor_Integration_IncrementalStreaming validates proper incremental streaming behavior
func TestKiroExecutor_Integration_IncrementalStreaming(t *testing.T) {
	fixtures := testutil.NewKiroTestFixtures()
	cfg := &config.Config{}
	exec := executor.NewKiroExecutor(cfg)
	auth := fixtures.NewTestAuth(nil, nil)

	// Test with longer content to verify incremental streaming
	longContent := "This is a longer response that should be streamed properly with multiple characters to test the incremental streaming functionality and ensure content is not truncated."

	rt := testutil.RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte(`data: {"content":"` + longContent + `"}`))),
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		}, nil
	})

	ctx := fixtures.WithRoundTripper(context.Background(), rt)
	req := cliproxyexecutor.Request{
		Model:   "claude-sonnet-4-5",
		Payload: fixtures.AnthropicChatPayload(t, []map[string]any{{"role": "user", "content": "Test streaming"}}, nil),
	}

	stream, err := exec.ExecuteStream(ctx, auth, req, cliproxyexecutor.Options{})
	require.NoError(t, err)

	// Collect all chunks
	var chunks []cliproxyexecutor.StreamChunk
	for chunk := range stream {
		if chunk.Err != nil {
			t.Fatalf("received chunk error: %v", chunk.Err)
		}
		chunks = append(chunks, chunk)
	}

	require.Greater(t, len(chunks), 0, "Should receive streaming chunks")

	// Verify content completeness
	var fullResponse strings.Builder
	for _, chunk := range chunks {
		fullResponse.Write(chunk.Payload)
	}
	responseStr := fullResponse.String()

	// CRITICAL BUG FIX: Verify content is not truncated and appears in text_delta
	assert.Contains(t, responseStr, `"text":"`+longContent+`"`, "Content should be complete in text_delta")
	assert.Contains(t, responseStr, "text_delta", "Should use proper text_delta event type")

	// Verify proper SSE structure
	assert.Contains(t, responseStr, "event: content_block_delta", "Should have content_block_delta events")
	assert.Contains(t, responseStr, `"type":"text_delta"`, "Should have text_delta type")

	t.Log("Incremental streaming test passed - content properly streamed:", len(longContent), "characters")
}