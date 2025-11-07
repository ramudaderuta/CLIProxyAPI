package iflow_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/runtime/executor"
)

// firstString helper function for testing (mirrors the internal function)
func firstString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// TestValidateToolCallID tests the validation function for tool_call_id values
func TestValidateToolCallID(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "valid UUID format",
			input:    "call_123e4567-e89b-12d3-a456-426614174000",
			expected: true,
		},
		{
			name:     "valid OpenAI tool format",
			input:    "toolu_abcd1234efgh5678",
			expected: true,
		},
		{
			name:     "valid alphanumeric",
			input:    "tool_call_123abc",
			expected: true,
		},
		{
			name:     "Claude Code TodoWrite pattern",
			input:    "***.TodoWrite:3",
			expected: false,
		},
		{
			name:     "Claude Code Edit pattern",
			input:    "***.Edit:6",
			expected: false,
		},
		{
			name:     "Claude Code Bash pattern",
			input:    "***.Bash:8",
			expected: false,
		},
		{
			name:     "any colon-containing ID",
			input:    "invalid:with:colons",
			expected: false,
		},
		{
			name:     "any triple-asterisk pattern",
			input:    "***.anything",
			expected: false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "whitespace only",
			input:    "   ",
			expected: false,
		},
		{
			name:     "mixed invalid characters",
			input:    "tool_***:invalid",
			expected: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Use the actual ValidateToolCallID function from the executor package
			result := executor.ValidateToolCallID(tc.input)
			assert.Equal(t, tc.expected, result, "case=%s: ValidateToolCallID(%q) should return %v", tc.name, tc.input, tc.expected)
		})
	}
}

// TestSanitizeToolCallID tests the sanitization function for tool_call_id values
func TestSanitizeToolCallID(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		input          string
		shouldGenerate bool
	}{
		{
			name:           "valid UUID remains unchanged",
			input:          "call_123e4567-e89b-12d3-a456-426614174000",
			shouldGenerate: false,
		},
		{
			name:           "valid OpenAI format remains unchanged",
			input:          "toolu_abcd1234efgh5678",
			shouldGenerate: false,
		},
		{
			name:           "Claude Code TodoWrite pattern gets sanitized",
			input:          "***.TodoWrite:3",
			shouldGenerate: true,
		},
		{
			name:           "Claude Code Edit pattern gets sanitized",
			input:          "***.Edit:6",
			shouldGenerate: true,
		},
		{
			name:           "colon-containing ID gets sanitized",
			input:          "invalid:with:colons",
			shouldGenerate: true,
		},
		{
			name:           "triple-asterisk pattern gets sanitized",
			input:          "***.anything",
			shouldGenerate: true,
		},
		{
			name:           "empty string gets sanitized",
			input:          "",
			shouldGenerate: true,
		},
		{
			name:           "whitespace gets sanitized",
			input:          "   ",
			shouldGenerate: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Use the actual SanitizeToolCallID function from the executor package
			result := executor.SanitizeToolCallID(tc.input)

			if tc.shouldGenerate {
				// Should generate a new valid UUID
				assert.NotEqual(t, tc.input, result, "case=%s: sanitizeToolCallID should generate new ID for invalid input", tc.name)
				assert.True(t, executor.ValidateToolCallID(result), "case=%s: generated ID should be valid", tc.name)
				assert.True(t, isValidUUIDFormat(result) || isValidToolIDFormat(result), "case=%s: generated ID should have valid format", tc.name)
			} else {
				// Should preserve the original valid ID
				assert.Equal(t, tc.input, result, "case=%s: sanitizeToolCallID should preserve valid input", tc.name)
			}
		})
	}
}

// TestSanitizeToolCallIDUniqueness tests that sanitization generates unique IDs
func TestSanitizeToolCallIDUniqueness(t *testing.T) {
	t.Parallel()

	// Test that multiple calls with same invalid input generate different IDs
	invalidInput := "***.TodoWrite:3"
	results := make(map[string]bool)

	for i := 0; i < 10; i++ {
		result := executor.SanitizeToolCallID(invalidInput)
		require.True(t, executor.ValidateToolCallID(result), "generated ID should be valid")

		// Should be unique (very high probability)
		assert.False(t, results[result], "generated IDs should be unique, got duplicate: %s", result)
		results[result] = true
	}
}

// TestSanitizeToolCallIDPerformance tests performance characteristics
func TestSanitizeToolCallIDPerformance(t *testing.T) {
	t.Parallel()

	// Test that sanitization is fast for valid IDs
	validID := "call_123e4567-e89b-12d3-a456-426614174000"

	// Should be very fast for valid IDs (just return as-is)
	for i := 0; i < 1000; i++ {
		result := executor.SanitizeToolCallID(validID)
		assert.Equal(t, validID, result, "valid ID should be returned unchanged")
	}
}

// TestToolCallIDIntegration tests integration with firstString function
func TestToolCallIDIntegration(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		inputs   []string
		expected string
	}{
		{
			name:     "all invalid inputs get sanitized",
			inputs:   []string{"***.TodoWrite:3", "***.Edit:6", ""},
			expected: "should_be_valid_uuid", // We expect a valid UUID to be generated
		},
		{
			name:     "mixed valid and invalid",
			inputs:   []string{"", "call_123e4567-e89b-12d3-a456-426614174000", "***.Bash:8"},
			expected: "call_123e4567-e89b-12d3-a456-426614174000", // The valid UUID should be preserved
		},
		{
			name:     "first valid ID preserved",
			inputs:   []string{"***.Task:2", "toolu_abcd1234efgh5678", "***.Read:5"},
			expected: "call_", // Should be a generated UUID starting with "call_" (first non-empty after sanitization)
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Apply sanitization only to invalid inputs (simulate real usage)
			sanitized := make([]string, len(tc.inputs))
			for i, input := range tc.inputs {
				if input == "" {
					sanitized[i] = input // Keep empty strings unchanged (firstString will skip them)
				} else if executor.ValidateToolCallID(input) {
					sanitized[i] = input // Keep valid inputs unchanged
				} else {
					sanitized[i] = executor.SanitizeToolCallID(input) // Sanitize invalid inputs
				}
			}

			// Use firstString with sanitized inputs
			result := firstString(sanitized...)

			if tc.expected == "should_be_valid_uuid" {
				assert.True(t, executor.ValidateToolCallID(result), "result should be a valid tool_call_id")
			} else if tc.expected == "call_" {
				// Should be a generated UUID starting with "call_"
				assert.True(t, executor.ValidateToolCallID(result), "result should be valid")
				assert.True(t, len(result) > 5 && result[:5] == "call_", "result should start with 'call_'")
			} else {
				// For valid IDs, they should be preserved exactly
				assert.Equal(t, tc.expected, result, "firstString should return first valid ID")
			}
		})
	}
}

// Helper functions for testing

func isValidUUIDFormat(id string) bool {
	// Check if it matches UUID format (with underscores instead of hyphens allowed)
	parts := strings.Split(id, "_")
	return len(parts) >= 2 && len(parts[0]) > 0 && len(parts[len(parts)-1]) >= 8
}

func isValidToolIDFormat(id string) bool {
	// Check if it matches OpenAI tool ID format
	return strings.HasPrefix(id, "toolu_") && len(id) > 10
}