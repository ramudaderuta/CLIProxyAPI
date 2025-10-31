// Package chat_completions provides request translation functionality for OpenAI to Kiro API compatibility.
// This file contains simplified unit tests for the core request translation functionality.
package chat_completions

import (
	"testing"
)

func TestGetKiroModel(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected string
	}{
		{
			name:     "Claude Sonnet 4.5",
			model:    "claude-sonnet-4-5",
			expected: "CLAUDE_SONNET_4_5_20250929_V1_0",
		},
		{
			name:     "Claude Sonnet 4.5 with date",
			model:    "claude-sonnet-4-5-20250929",
			expected: "CLAUDE_SONNET_4_5_20250929_V1_0",
		},
		{
			name:     "Unknown model",
			model:    "unknown-model",
			expected: "CLAUDE_SONNET_4_5_20250929_V1_0", // Should default to claude-sonnet-4-5
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getKiroModel(tt.model)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}