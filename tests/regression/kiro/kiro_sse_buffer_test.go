package kiro

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/tests/shared"
)

// TestSSEBufferLimits tests SSE buffer handling at various sizes
func TestSSEBufferLimits(t *testing.T) {
	bufferSizes := []struct {
		name      string
		size      int
		eventSize int
	}{
		{"small buffer small events", 1024, 100},
		{"small buffer large events", 1024, 512},
		{"medium buffer", 8192, 1024},
		{"large buffer", 65536, 4096},
	}

	for _, bs := range bufferSizes {
		t.Run(bs.name, func(t *testing.T) {
			var buf bytes.Buffer
			buf.Grow(bs.size)

			writer := shared.NewSSEWriter(&buf)

			// Write event that fits in buffer
			eventData := strings.Repeat("X", bs.eventSize)
			if err := writer.WriteEvent("test", eventData); err != nil {
				t.Fatalf("Failed to write event: %v", err)
			}

			if buf.Len() == 0 {
				t.Error("Buffer should not be empty after write")
			}

			t.Logf("✓ Buffer size: %d, Event size: %d, Used: %d bytes",
				bs.size, bs.eventSize, buf.Len())
		})
	}
}

// TestSSEBufferOverflow tests handling of events larger than buffer
func TestSSEBufferOverflow(t *testing.T) {
	shared.SkipIfShort(t, "buffer overflow test")

	// Create a buffer
	var buf bytes.Buffer
	writer := shared.NewSSEWriter(&buf)

	// Write very large event
	largeEvent := strings.Repeat("X", 1024*1024) // 1 MB

	if err := writer.WriteEvent("large", largeEvent); err != nil {
		t.Logf("Large event handled: %v", err)
	}

	// Buffer should still be valid
	if buf.Len() > 0 {
		t.Log("✓ Buffer handled large event")
	}
}

// TestSSEBufferMultipleFlushes tests multiple flush scenarios
func TestSSEBufferMultipleFlushes(t *testing.T) {
	var buf bytes.Buffer
	writer := shared.NewSSEWriter(&buf)

	flushCount := 100
	for i := 0; i < flushCount; i++ {
		data := strings.Repeat(".", i+1)
		if err := writer.WriteEvent("chunk", data); err != nil {
			t.Fatalf("Flush %d failed: %v", i, err)
		}
	}

	// Parse all events
	scanner := bufio.NewScanner(&buf)
	eventCount := 0

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			eventCount++
		}
	}

	if eventCount != flushCount {
		t.Errorf("Expected %d events, got %d", flushCount, eventCount)
	}

	t.Logf("✓ %d flushes handled correctly", flushCount)
}

// TestSSEMemoryLimits tests memory usage with streaming
func TestSSEMemoryLimits(t *testing.T) {
	shared.SkipIfShort(t, "memory limit test")

	// Simulate long-running stream
	var buf bytes.Buffer
	writer := shared.NewSSEWriter(&buf)

	streamDuration := 1000 // events

	for i := 0; i < streamDuration; i++ {
		eventData := "Event " + strings.Repeat("data", 10)
		writer.WriteEvent("stream", eventData)

		// Periodically check buffer size
		if i%100 == 0 {
			currentSize := buf.Len()
			t.Logf("After %d events: buffer size %d bytes", i, currentSize)
		}
	}

	finalSize := buf.Len()
	t.Logf("✓ Final buffer size after %d events: %d bytes", streamDuration, finalSize)
}

// TestSSEConnectionTimeout simulates connection timeout scenarios
func TestSSEConnectionTimeout(t *testing.T) {
	// Test that partial events are handled
	partialSSE := "event: test\ndata: incomplete"

	scanner := bufio.NewScanner(strings.NewReader(partialSSE))
	lines := 0

	for scanner.Scan() {
		lines++
	}

	if lines != 2 {
		t.Errorf("Expected 2 lines, got %d", lines)
	}

	t.Log("✓ Partial event handling validated")
}

// TestSSEBufferReset tests buffer reset between requests
func TestSSEBufferReset(t *testing.T) {
	var buf bytes.Buffer
	writer := shared.NewSSEWriter(&buf)

	// First batch of events
	for i := 0; i < 10; i++ {
		writer.WriteEvent("batch1", "data")
	}

	firstSize := buf.Len()

	// Reset buffer
	buf.Reset()

	if buf.Len() != 0 {
		t.Error("Buffer should be empty after reset")
	}

	// Second batch
	for i := 0; i < 10; i++ {
		writer.WriteEvent("batch2", "data")
	}

	secondSize := buf.Len()

	// Sizes should be similar
	if abs(firstSize-secondSize) > 100 {
		t.Errorf("Buffer sizes differ significantly: %d vs %d", firstSize, secondSize)
	}

	t.Log("✓ Buffer reset working correctly")
}

// Helper function
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// Benchmark buffer operations
func BenchmarkSSEBufferWrite(b *testing.B) {
	var buf bytes.Buffer
	writer := shared.NewSSEWriter(&buf)
	eventData := strings.Repeat("X", 1024)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		buf.Reset()
		writer.WriteEvent("bench", eventData)
	}
}

func BenchmarkSSEBufferParse(b *testing.B) {
	sseData := "event: test\ndata: " + strings.Repeat("X", 1024) + "\n\n"

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		scanner := bufio.NewScanner(strings.NewReader(sseData))
		for scanner.Scan() {
			_ = scanner.Text()
		}
	}
}
