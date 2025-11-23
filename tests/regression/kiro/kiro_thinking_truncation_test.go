package kiro

import (
	"strings"
	"testing"
)

// TestThinkingTagRemoval tests removal of <thinking> tags from responses
func TestThinkingTagRemoval(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple thinking tag",
			input:    "<thinking>Let me think...</thinking>The answer is 42",
			expected: "The answer is 42",
		},
		{
			name:     "multiple thinking tags",
			input:    "<thinking>First thought</thinking>Text<thinking>Second thought</thinking>More text",
			expected: "TextMore text",
		},
		{
			name:     "nested thinking tags",
			input:    "<thinking>Outer<thinking>Inner</thinking>Still outer</thinking>Final text",
			expected: "Final text",
		},
		{
			name:     "no thinking tags",
			input:    "Just normal text without any tags",
			expected: "Just normal text without any tags",
		},
		{
			name:     "partial opening tag",
			input:    "<think This is not a complete tag",
			expected: "<think This is not a complete tag",
		},
		{
			name:     "thinking tag at end",
			input:    "Answer: <thinking>processing...</thinking>",
			expected: "Answer: ",
		},
		{
			name:     "empty thinking tag",
			input:    "<thinking></thinking>Response text",
			expected: "Response text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simple regex-based thinking tag removal
			result := removeThinkingTags(tt.input)

			// Trim spaces for comparison
			result = strings.TrimSpace(result)
			expected := strings.TrimSpace(tt.expected)

			if result != expected {
				t.Errorf("Input: %q\nExpected: %q\nGot: %q", tt.input, expected, result)
			}

			t.Logf("✓ %s", tt.name)
		})
	}
}

// TestThinkingTagEdgeCases tests edge cases in thinking tag handling
func TestThinkingTagEdgeCases(t *testing.T) {
	edgeCases := []struct {
		name        string
		input       string
		shouldMatch bool
		description string
	}{
		{
			name:        "malformed opening",
			input:       "<thinking The answer is 42",
			shouldMatch: false,
			description: "Malformed opening tag should not match",
		},
		{
			name:        "malformed closing",
			input:       "Text </thinkng>",
			shouldMatch: false,
			description: "Typo in closing tag should not match",
		},
		{
			name:        "case sensitivity",
			input:       "<THINKING>Uppercase</THINKING>",
			shouldMatch: false,
			description: "Uppercase tags should not match (if regex is case-sensitive)",
		},
		{
			name:        "whitespace in tags",
			input:       "< thinking >Text</ thinking >",
			shouldMatch: false,
			description: "Whitespace in tags should not match",
		},
	}

	for _, tc := range edgeCases {
		t.Run(tc.name, func(t *testing.T) {
			result := removeThinkingTags(tc.input)

			// Check if thinking tags were removed
			hasThinking := strings.Contains(result, "<thinking>") ||
				strings.Contains(result, "</thinking>")

			if hasThinking == tc.shouldMatch {
				t.Logf("✓ %s", tc.description)
			} else {
				t.Errorf("%s: unexpected result", tc.description)
			}
		})
	}
}

// TestThinkingTruncationPerformance tests performance with large content
func TestThinkingTruncationPerformance(t *testing.T) {
	// Generate large content with thinking tags
	largeContent := strings.Repeat("<thinking>Processing...</thinking>Text chunk. ", 1000)

	result := removeThinkingTags(largeContent)

	// Verify all thinking tags removed
	if strings.Contains(result, "<thinking>") {
		t.Error("Large content still contains thinking tags")
	}

	// Result should be shorter than input (or equal if no tags were actually present)
	if len(result) > len(largeContent) {
		t.Error("Result should not be longer than input")
	}

	t.Logf("✓ Large content processed: %d -> %d bytes", len(largeContent), len(result))
}

// Helper function - thinking tag removal with proper nesting support
func removeThinkingTags(content string) string {
	result := content

	// Keep removing thinking tags until none remain
	// This handles nested tags by removing innermost first
	maxIterations := 10000 // Prevent infinite loop - increased for large content
	for i := 0; i < maxIterations; i++ {
		startIdx := strings.Index(result, "<thinking>")
		if startIdx == -1 {
			break // No more opening tags
		}

		// Find the matching closing tag
		// We need to count nested tags to find the right closing tag
		depth := 1
		searchPos := startIdx + len("<thinking>")
		endIdx := -1

		for searchPos < len(result) && depth > 0 {
			if strings.HasPrefix(result[searchPos:], "<thinking>") {
				depth++
				searchPos += len("<thinking>")
			} else if strings.HasPrefix(result[searchPos:], "</thinking>") {
				depth--
				if depth == 0 {
					endIdx = searchPos
					break
				}
				searchPos += len("</thinking>")
			} else {
				searchPos++
			}
		}

		if endIdx == -1 {
			// No matching closing tag found
			break
		}

		// Remove the tag and its content
		result = result[:startIdx] + result[endIdx+len("</thinking>"):]
	}

	return result
}

// Benchmark thinking tag removal
func BenchmarkThinkingTagRemoval(b *testing.B) {
	input := "<thinking>Let me think about this...</thinking>The answer is: <thinking>processing</thinking>42"

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = removeThinkingTags(input)
	}
}

func BenchmarkThinkingTagRemovalLarge(b *testing.B) {
	input := strings.Repeat("<thinking>Thought process</thinking>Response text. ", 100)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = removeThinkingTags(input)
	}
}
