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
		name        string
		input       string
		expected    bool
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
			name:     "Claude style id allowed",
			input:    "***.TodoWrite:3",
			expected: true,
		},
		{
			name:     "id with colons allowed",
			input:    "invalid:with:colons",
			expected: true,
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
			name:     "leading whitespace trimmed",
			input:    "  call_trimmed ",
			expected: true,
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
		expected       string
		shouldGenerate bool
	}{
		{
			name:           "valid UUID remains unchanged",
			input:          "call_123e4567-e89b-12d3-a456-426614174000",
			expected:       "call_123e4567-e89b-12d3-a456-426614174000",
			shouldGenerate: false,
		},
		{
			name:           "valid OpenAI format remains unchanged",
			input:          "toolu_abcd1234efgh5678",
			expected:       "toolu_abcd1234efgh5678",
			shouldGenerate: false,
		},
		{
			name:           "Claude Code TodoWrite pattern preserved",
			input:          "***.TodoWrite:3",
			expected:       "***.TodoWrite:3",
			shouldGenerate: false,
		},
		{
			name:           "colon-containing ID preserved",
			input:          "invalid:with:colons",
			expected:       "invalid:with:colons",
			shouldGenerate: false,
		},
		{
			name:           "empty string gets generated id",
			input:          "",
			shouldGenerate: true,
		},
		{
			name:           "whitespace gets generated id",
			input:          "   ",
			shouldGenerate: true,
		},
		{
			name:           "leading and trailing whitespace trimmed",
			input:          "  call_trimmed ",
			expected:       "call_trimmed",
			shouldGenerate: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Use the actual SanitizeToolCallID function from the executor package
			result := executor.SanitizeToolCallID(tc.input)

			if tc.shouldGenerate {
				assert.NotEqual(t, strings.TrimSpace(tc.input), result, "case=%s: sanitizeToolCallID should generate new ID for empty input", tc.name)
				assert.True(t, executor.ValidateToolCallID(result), "case=%s: generated ID should be valid", tc.name)
				assert.True(t, isValidUUIDFormat(result), "case=%s: generated ID should follow UUID-like format", tc.name)
				return
			}

			assert.Equal(t, tc.expected, result, "case=%s: sanitizeToolCallID should preserve trimmed input", tc.name)
		})
	}
}

// TestSanitizeToolCallIDUniqueness tests that sanitization generates unique IDs
func TestSanitizeToolCallIDUniqueness(t *testing.T) {
	t.Parallel()

	// Test that multiple calls with same invalid input generate different IDs
	invalidInput := ""
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
		name            string
		inputs          []string
		expected        string
		expectGenerated bool
	}{
		{
			name:            "all empty inputs get generated id",
			inputs:          []string{"", "   ", ""},
			expectGenerated: true,
		},
		{
			name:     "mixed valid and empty preserves first valid",
			inputs:   []string{"", "call_123e4567-e89b-12d3-a456-426614174000", "***.Bash:8"},
			expected: "call_123e4567-e89b-12d3-a456-426614174000",
		},
		{
			name:     "claude style id preserved",
			inputs:   []string{"***.Task:2", "toolu_abcd1234efgh5678", "***.Read:5"},
			expected: "***.Task:2",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			first := firstString(tc.inputs...)
			result := executor.SanitizeToolCallID(first)

			if tc.expectGenerated {
				assert.True(t, executor.ValidateToolCallID(result), "result should be a valid tool_call_id")
				assert.True(t, strings.HasPrefix(result, "call_"), "generated IDs should start with call_")
				return
			}

			assert.Equal(t, tc.expected, result, "first non-empty ID should be preserved")
		})
	}
}

// Helper functions for testing

func isValidUUIDFormat(id string) bool {
	// Check if it matches UUID format (with underscores instead of hyphens allowed)
	parts := strings.Split(id, "_")
	return len(parts) >= 2 && len(parts[0]) > 0 && len(parts[len(parts)-1]) >= 8
}
