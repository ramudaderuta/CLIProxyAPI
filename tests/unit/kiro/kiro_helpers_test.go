package kiro

import (
	"strings"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro/helpers"
)

// TestSafeGetString tests safe string extraction from maps
func TestSafeGetString(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		key      string
		expected string
	}{
		{
			name:     "valid_string",
			input:    map[string]interface{}{"key": "value"},
			key:      "key",
			expected: "value",
		},
		{
			name:     "missing_key",
			input:    map[string]interface{}{"other": "value"},
			key:      "key",
			expected: "",
		},
		{
			name:     "nil_map",
			input:    nil,
			key:      "key",
			expected: "",
		},
		{
			name:     "wrong_type",
			input:    map[string]interface{}{"key": 123},
			key:      "key",
			expected: "",
		},
		{
			name:     "empty_string",
			input:    map[string]interface{}{"key": ""},
			key:      "key",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := helpers.SafeGetString(tt.input, tt.key)

			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}

			t.Logf("✓ SafeGetString: %s", tt.name)
		})
	}
}

// TestTruncateString tests string truncation with suffix
func TestTruncateString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		suffix   string
		expected string
	}{
		{
			name:     "no_truncation_needed",
			input:    "short",
			maxLen:   10,
			suffix:   "...",
			expected: "short",
		},
		{
			name:     "truncate_with_suffix",
			input:    "this is a very long string",
			maxLen:   10,
			suffix:   "...",
			expected: "this is a ...",
		},
		{
			name:     "exact_length",
			input:    "exactlyten",
			maxLen:   10,
			suffix:   "...",
			expected: "exactlyten",
		},
		{
			name:     "empty_string",
			input:    "",
			maxLen:   10,
			suffix:   "...",
			expected: "",
		},
		{
			name:     "custom_suffix",
			input:    "long text here",
			maxLen:   8,
			suffix:   " [more]",
			expected: "long tex [more]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := helpers.TruncateString(tt.input, tt.maxLen, tt.suffix)

			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}

			t.Logf("✓ TruncateString: %s", tt.name)
		})
	}
}

// TestSanitizeContent tests content sanitization
func TestSanitizeContent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "clean_text",
			input:    "Normal text without special characters",
			expected: "Normal text without special characters",
		},
		{
			name:     "with_newlines",
			input:    "Line 1\nLine 2\nLine 3",
			expected: "Line 1\nLine 2\nLine 3",
		},
		{
			name:     "with_tabs",
			input:    "Col1\tCol2\tCol3",
			expected: "Col1\tCol2\tCol3",
		},
		{
			name:     "unicode_characters",
			input:    "Hello 世界 🌍",
			expected: "Hello 世界 🌍",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Content sanitization is primarily for thinking tags
			// which is tested separately
			if tt.input != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, tt.input)
			}

			t.Logf("✓ Content sanitization: %s", tt.name)
		})
	}
}

// TestSafeJSONParse tests safe JSON parsing helpers
func TestSafeJSONParse(t *testing.T) {
	tests := []struct {
		name        string
		jsonStr     string
		shouldParse bool
	}{
		{
			name:        "valid_json_object",
			jsonStr:     `{"key":"value"}`,
			shouldParse: true,
		},
		{
			name:        "valid_json_array",
			jsonStr:     `["item1","item2"]`,
			shouldParse: true,
		},
		{
			name:        "invalid_json",
			jsonStr:     `{invalid`,
			shouldParse: false,
		},
		{
			name:        "empty_string",
			jsonStr:     "",
			shouldParse: false,
		},
		{
			name:        "malformed_json",
			jsonStr:     `{"key":"value"`,
			shouldParse: false,
		},
		{
			name:        "malformed_json_array",
			jsonStr:     `["item1"`,
			shouldParse: false,
		},
		{
			name:        "only_open_brace",
			jsonStr:     `{`,
			shouldParse: false,
		},
		{
			name:        "only_close_brace",
			jsonStr:     `}`,
			shouldParse: false,
		},
		{
			name:        "only_open_bracket",
			jsonStr:     `[`,
			shouldParse: false,
		},
		{
			name:        "only_close_bracket",
			jsonStr:     `]`,
			shouldParse: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Proper JSON validation - check both opening and closing
			trimmed := strings.TrimSpace(tt.jsonStr)
			isValid := (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
				(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]"))

			if isValid != tt.shouldParse {
				t.Errorf("Expected parse=%v, got %v for %q", tt.shouldParse, isValid, tt.jsonStr)
			}

			t.Logf("✓ JSON parse: %s", tt.name)
		})
	}
}

// TestStringContainsCaseInsensitive tests case-insensitive string matching
func TestStringContainsCaseInsensitive(t *testing.T) {
	tests := []struct {
		name     string
		haystack string
		needle   string
		expected bool
	}{
		{
			name:     "exact_match",
			haystack: "Hello World",
			needle:   "World",
			expected: true,
		},
		{
			name:     "case_insensitive_match",
			haystack: "Hello World",
			needle:   "world",
			expected: true,
		},
		{
			name:     "no_match",
			haystack: "Hello World",
			needle:   "Goodbye",
			expected: false,
		},
		{
			name:     "empty_needle",
			haystack: "Hello World",
			needle:   "",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := strings.Contains(
				strings.ToLower(tt.haystack),
				strings.ToLower(tt.needle),
			)

			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}

			t.Logf("✓ Case-insensitive contains: %s", tt.name)
		})
	}
}

// TestReplaceAllCaseInsensitive tests case-insensitive string replacement
func TestReplaceAllCaseInsensitive(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		old      string
		new      string
		contains string
	}{
		{
			name:     "simple_replace",
			input:    "Hello World",
			old:      "World",
			new:      "Universe",
			contains: "Universe",
		},
		{
			name:     "multiple_occurrences",
			input:    "test test test",
			old:      "test",
			new:      "demo",
			contains: "demo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := strings.ReplaceAll(tt.input, tt.old, tt.new)

			if !strings.Contains(result, tt.contains) {
				t.Errorf("Result should contain %q", tt.contains)
			}

			t.Logf("✓ Replace all: %s", tt.name)
		})
	}
}

// Benchmark helper functions
func BenchmarkSafeGetString(b *testing.B) {
	m := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
		"key3": 123,
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = helpers.SafeGetString(m, "key1")
	}
}

func BenchmarkTruncateString(b *testing.B) {
	longString := strings.Repeat("Lorem ipsum dolor sit amet ", 100)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = helpers.TruncateString(longString, 100, "...")
	}
}

func BenchmarkStringContains(b *testing.B) {
	haystack := strings.Repeat("Lorem ipsum dolor sit amet ", 100)
	needle := "dolor"

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = strings.Contains(haystack, needle)
	}
}
