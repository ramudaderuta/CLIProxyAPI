package kiro

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/tests/shared"
)

// TestSSEFormatting tests Server-Sent Events formatting
func TestSSEFormatting(t *testing.T) {
	tests := []struct {
		name        string
		event       string
		data        string
		expected    string
		description string
	}{
		{
			name:        "simple event",
			event:       "message",
			data:        `{"content":"Hello"}`,
			expected:    "event: message\ndata: {\"content\":\"Hello\"}\n\n",
			description: "Basic SSE event with data",
		},
		{
			name:        "no event type",
			event:       "",
			data:        `{"content":"Test"}`,
			expected:    "data: {\"content\":\"Test\"}\n\n",
			description: "SSE data without event type",
		},
		{
			name:        "done event",
			event:       "done",
			data:        "[DONE]",
			expected:    "event: done\ndata: [DONE]\n\n",
			description: "Stream completion marker",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			writer := shared.NewSSEWriter(&buf)

			if err := writer.WriteEvent(tt.event, tt.data); err != nil {
				t.Fatalf("Failed to write SSE event: %v", err)
			}

			got := buf.String()
			if got != tt.expected {
				t.Errorf("%s\nExpected:\n%q\nGot:\n%q", tt.description, tt.expected, got)
			}

			t.Logf("✓ %s", tt.description)
		})
	}
}

// TestSSEParsing tests parsing SSE streams
func TestSSEParsing(t *testing.T) {
	sseStream := `event: message_start
data: {"type":"message_start"}

event: content_block_delta
data: {"type":"content_block_delta","delta":{"text":"Hello"}}

event: message_stop
data: {"type":"message_stop"}

`

	scanner := bufio.NewScanner(strings.NewReader(sseStream))
	events := 0

	var currentEvent, currentData string
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "event: ") {
			currentEvent = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			currentData = strings.TrimPrefix(line, "data: ")
		} else if line == "" && currentData != "" {
			// Event complete
			events++
			t.Logf("Parsed event: %s with data: %s", currentEvent, currentData)
			currentEvent, currentData = "", ""
		}
	}

	expectedEvents := 3
	if events != expectedEvents {
		t.Errorf("Expected %d events, got %d", expectedEvents, events)
	}

	t.Logf("✓ Successfully parsed %d SSE events", events)
}

// TestSSEEventTypes tests different Kiro SSE event types
func TestSSEEventTypes(t *testing.T) {
	eventTypes := []struct {
		name        string
		eventType   string
		description string
	}{
		{"message_start", "messageStart", "Start of message"},
		{"content_block_start", "contentBlockStart", "Start of content block"},
		{"content_block_delta", "contentBlockDelta", "Content chunk"},
		{"content_block_stop", "contentBlockStop", "End of content block"},
		{"message_delta", "messageDelta", "Message metadata update"},
		{"message_stop", "messageStop", "End of message"},
	}

	for _, et := range eventTypes {
		t.Run(et.name, func(t *testing.T) {
			var buf bytes.Buffer
			writer := shared.NewSSEWriter(&buf)

			data := `{"type":"` + et.eventType + `"}`
			if err := writer.WriteEvent(et.name, data); err != nil {
				t.Fatalf("Failed to write event: %v", err)
			}

			output := buf.String()
			if !strings.Contains(output, et.name) {
				t.Errorf("Event output should contain event type %q", et.name)
			}

			t.Logf("✓ %s - %s", et.name, et.description)
		})
	}
}

// TestSSEBuffering tests SSE buffer handling
func TestSSEBuffering(t *testing.T) {
	var buf bytes.Buffer
	writer := shared.NewSSEWriter(&buf)

	// Write multiple events
	eventCount := 10
	for i := 0; i < eventCount; i++ {
		data := `{"index":` + string(rune('0'+i)) + `}`
		if err := writer.WriteEvent("chunk", data); err != nil {
			t.Fatalf("Failed to write event %d: %v", i, err)
		}
	}

	// Count events in buffer
	lines := strings.Split(buf.String(), "\n\n")
	// Last empty line doesn't count
	actualEvents := len(lines) - 1

	if actualEvents != eventCount {
		t.Errorf("Expected %d events in buffer, got %d", eventCount, actualEvents)
	}

	t.Logf("✓ Successfully buffered %d SSE events", eventCount)
}

// TestSSEEmptyLines tests handling of empty lines in SSE
func TestSSEEmptyLines(t *testing.T) {
	sseData := "data: test1\n\ndata: test2\n\n"

	parts := strings.Split(sseData, "\n\n")
	nonEmpty := 0
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			nonEmpty++
		}
	}

	expected := 2
	if nonEmpty != expected {
		t.Errorf("Expected %d non-empty parts, got %d", expected, nonEmpty)
	}

	t.Log("✓ Empty lines handled correctly")
}

// TestSSEDoneMarker tests the OpenAI [DONE] marker
func TestSSEDoneMarker(t *testing.T) {
	var buf bytes.Buffer
	writer := shared.NewSSEWriter(&buf)

	// Write done marker
	if err := writer.WriteEvent("", "data: [DONE]"); err != nil {
		t.Fatalf("Failed to write done marker: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "[DONE]") {
		t.Error("Output should contain [DONE] marker")
	}

	t.Log("✓ [DONE] marker written correctly")
}

// TestSSEContentEncoding tests SSE content with special characters
func TestSSEContentEncoding(t *testing.T) {
	specialChars := []struct {
		name    string
		content string
	}{
		{"newlines", "Line1\nLine2"},
		{"quotes", `He said "hello"`},
		{"unicode", "Hello 世界 🌍"},
		{"json", `{"key":"value","nested":{"a":1}}`},
	}

	for _, tc := range specialChars {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			writer := shared.NewSSEWriter(&buf)

			if err := writer.WriteEvent("test", tc.content); err != nil {
				t.Fatalf("Failed to write content: %v", err)
			}

			output := buf.String()
			if !strings.Contains(output, "data: ") {
				t.Error("Output should contain data prefix")
			}

			t.Logf("✓ Special characters handled: %s", tc.name)
		})
	}
}

// Benchmark SSE operations
func BenchmarkSSEWrite(b *testing.B) {
	var buf bytes.Buffer
	writer := shared.NewSSEWriter(&buf)
	data := `{"type":"content","text":"Hello, World!"}`

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		buf.Reset()
		writer.WriteEvent("message", data)
	}
}

func BenchmarkSSEParse(b *testing.B) {
	sseData := "event: test\ndata: {\"content\":\"test\"}\n\n"

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		scanner := bufio.NewScanner(strings.NewReader(sseData))
		for scanner.Scan() {
			_ = scanner.Text()
		}
	}
}

func BenchmarkSSEMultipleEvents(b *testing.B) {
	var buf bytes.Buffer
	writer := shared.NewSSEWriter(&buf)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		buf.Reset()
		for j := 0; j < 10; j++ {
			writer.WriteEvent("chunk", `{"index":`+string(rune('0'+j))+`}`)
		}
	}
}
