package tests

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/runtime/executor"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro"
)

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) >= len(substr) && containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestKiroThinkingContentTruncation tests the specific issue where "Thinking" content filtering
// in the executor causes truncation of responses containing apostrophes
func TestKiroThinkingContentTruncation(t *testing.T) {
	// Test the current problematic Thinking filtering logic directly
	tests := []struct {
		name               string
		inputContent       string
		expectedAfterThinkingFilter string
		description        string
	}{
		{
			name: "I_dont_know_without_Thinking_should_remain_intact",
			inputContent:       "I don't know the answer to that question.",
			expectedAfterThinkingFilter: "I don't know the answer to that question.",
			description:        "Content without Thinking should not be affected",
		},
		{
			name: "Content_with_Thinking_at_start_should_extract_after",
			inputContent:       "Thinking: I need to analyze this\n\nI don't have enough information.",
			expectedAfterThinkingFilter: "I don't have enough information.",
			description:        "Should extract content after Thinking section",
		},
		{
			name: "Content_with_Thinking_at_end_should_preserve_before",
			inputContent:       "I don't think this will work.\n\nThinking: Let me consider alternatives",
			expectedAfterThinkingFilter: "I don't think this will work.",
			description:        "Should preserve content before Thinking section",
		},
		{
			name: "Content_with_Thinking_in_middle_should_extract_correctly",
			inputContent:       "I can't help with that.\n\nThinking: This might be inappropriate\n\nActually, I don't think I should respond.",
			expectedAfterThinkingFilter: "I can't help with that.\n\nActually, I don't think I should respond.",
			description:        "Should handle multiple Thinking sections and preserve non-Thinking content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Apply the new fixed Thinking filtering logic from executor
			filteredContent := executor.FilterThinkingContent(tt.inputContent)

			// This test should now pass with the fixed implementation
			if filteredContent != tt.expectedAfterThinkingFilter {
				t.Errorf("Thinking filtering failed.\nInput:    %q\nExpected: %q\nGot:      %q\nDescription: %s",
					tt.inputContent, tt.expectedAfterThinkingFilter, filteredContent, tt.description)
			}

			// Verify apostrophes are preserved
			if containsString(tt.expectedAfterThinkingFilter, "'") && !containsString(filteredContent, "'") {
				t.Errorf("Apostrophes were lost after Thinking filtering! Expected: %q, Got: %q",
					tt.expectedAfterThinkingFilter, filteredContent)
			}

			// Verify no truncation occurred
			if containsString(filteredContent, "I don") && !containsString(tt.expectedAfterThinkingFilter, "I don") {
				t.Errorf("Found truncation 'I don' in filtered result: %q", filteredContent)
			}
			if containsString(filteredContent, "I can") && !containsString(tt.expectedAfterThinkingFilter, "I can") {
				t.Errorf("Found truncation 'I can' in filtered result: %q", filteredContent)
			}
		})
	}
}

// TestKiroResponseParsingWithThinking tests full Kiro response parsing with Thinking content
func TestKiroResponseParsingWithThinking(t *testing.T) {
	tests := []struct {
		name            string
		rawKiroResponse string
		expectedContent string
		description     string
	}{
		{
			name: "Kiro_response_with_I_dont_know_and_Thinking",
			rawKiroResponse: `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "I don't know the answer.\n\nThinking: I should search for this information"}}}}`,
			expectedContent: "I don't know the answer.",
			description:     "Should preserve 'I don't know' and filter Thinking section",
		},
		{
			name: "Kiro_response_with_multiple_apostrophes_and_Thinking",
			rawKiroResponse: `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "I can't help with that because it's not appropriate.\n\nThinking: This violates guidelines\n\nWe don't provide this type of assistance."}}}}`,
			expectedContent: "I can't help with that because it's not appropriate.\n\nWe don't provide this type of assistance.",
			description:     "Should preserve all apostrophe-containing phrases while filtering Thinking",
		},
		{
			name: "Kiro_response_without_Thinking_should_preserve_all",
			rawKiroResponse: `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "I don't have information about that topic, but I'll try to help you understand."}}}}`,
			expectedContent: "I don't have information about that topic, but I'll try to help you understand.",
			description:     "Should preserve all content when no Thinking section present",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the response using kirotranslator.ParseResponse
			content, toolCalls := kiro.ParseResponse([]byte(tt.rawKiroResponse))

			// Apply the new fixed Thinking filtering logic that happens in the executor
			filteredContent := executor.FilterThinkingContent(content)

			if filteredContent != tt.expectedContent {
				t.Errorf("Response parsing failed.\nInput:    %s\nExpected: %q\nGot:      %q\nDescription: %s",
					tt.rawKiroResponse, tt.expectedContent, filteredContent, tt.description)
			}

			// Verify apostrophes are preserved
			if containsString(tt.expectedContent, "'") && !containsString(filteredContent, "'") {
				t.Errorf("Apostrophes were lost in response parsing! Expected: %q, Got: %q",
					tt.expectedContent, filteredContent)
			}

			// Verify no tool calls in these content-only tests
			if len(toolCalls) != 0 {
				t.Errorf("Expected no tool calls, got %d", len(toolCalls))
			}
		})
	}
}


