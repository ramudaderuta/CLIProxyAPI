package kiro_test

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro"
	"github.com/stretchr/testify/assert"
)

// TestSpecialCharacterPreservation tests that special characters, particularly apostrophes,
// are preserved correctly during parsing
func TestSpecialCharacterPreservation(t *testing.T) {
	// This test will fail because the current implementation truncates trailing apostrophes
	t.Run("EdgeCase_Apostrophe_at_end_of_text", func(t *testing.T) {
		// Test case where apostrophe is at the end of text - this currently gets truncated
		input := `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "That's all folks'"}}}}`
		expected := "That's all folks'"

		content, _ := kiro.ParseResponse([]byte(input))

		// This assertion will fail because the trailing apostrophe is being truncated
		assert.Equal(t, expected, content, "Trailing apostrophe should be preserved")
	})

	t.Run("Apostrophe_in_middle_of_text", func(t *testing.T) {
		// Test case with apostrophe in middle of text - should pass with current implementation
		input := `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "It's a beautiful day"}}}}`
		expected := "It's a beautiful day"

		content, _ := kiro.ParseResponse([]byte(input))

		assert.Equal(t, expected, content, "Apostrophe in middle should be preserved")
	})

	t.Run("Multiple_apostrophes", func(t *testing.T) {
		// Test case with multiple apostrophes - this may fail due to truncation issues
		input := `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "Don't stop believin'"}}}}`
		expected := "Don't stop believin'"

		content, _ := kiro.ParseResponse([]byte(input))

		// This assertion may fail if trailing apostrophes are truncated
		assert.Equal(t, expected, content, "Multiple apostrophes should be preserved")
	})

	t.Run("Apostrophe_in_contractions", func(t *testing.T) {
		// Test case with various contractions - should reveal parsing issues
		testCases := []struct {
			name     string
			input    string
			expected string
		}{
			{
				name:     "don't",
				input:    `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "I don't know"}}}}`,
				expected: "I don't know",
			},
			{
				name:     "won't",
				input:    `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "It won't work"}}}}`,
				expected: "It won't work",
			},
			{
				name:     "can't",
				input:    `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "I can't do it"}}}}`,
				expected: "I can't do it",
			},
			{
				name:     "it's",
				input:    `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "It's correct"}}}}`,
				expected: "It's correct",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				content, _ := kiro.ParseResponse([]byte(tc.input))
				// These assertions will fail if apostrophes are not properly preserved
				assert.Equal(t, tc.expected, content, "Contraction should be preserved correctly")
			})
		}
	})

	t.Run("Apostrophe_in_possessives", func(t *testing.T) {
		// Test case with possessives - may reveal parsing issues
		input := `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "John's book is Mary's favorite"}}}}`
		expected := "John's book is Mary's favorite"

		content, _ := kiro.ParseResponse([]byte(input))

		// This assertion will fail if apostrophes in possessives are not preserved
		assert.Equal(t, expected, content, "Possessives should be preserved correctly")
	})
}