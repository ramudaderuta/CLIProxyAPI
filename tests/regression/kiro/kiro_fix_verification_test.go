package kiro_test

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/runtime/executor"

	"github.com/router-for-me/CLIProxyAPI/v6/tests/shared"
)

// TestKiroFixVerification verifies that our fix for the "I don't know" truncation issue works correctly

func TestKiroFixVerification(t *testing.T) {
	// Test cases that demonstrate the fix working
	testCases := []struct {
		name            string
		inputContent    string
		expectedOutput  string
		description     string
	}{
		{
			name:           "Original_issue_I_dont_know_with_Thinking",
			inputContent:   "I don't know the answer to that question.\n\nThinking: I should search for this information",
			expectedOutput: "I don't know the answer to that question.",
			description:    "Original issue: 'I don't know' should not be truncated to 'I don'",
		},
		{
			name:           "Multiple_apostrophes_with_Thinking",
			inputContent:   "I can't help with that because it's not appropriate.\n\nThinking: This violates guidelines\n\nWe don't provide this type of assistance.",
			expectedOutput: "I can't help with that because it's not appropriate.\n\nWe don't provide this type of assistance.",
			description:    "Multiple apostrophe-containing phrases should be preserved",
		},
		{
			name:           "Complex_case_with_mixed_content",
			inputContent:   "It's complicated because we're dealing with user's data.\n\nThinking: Privacy concerns\n\nI don't think we can process this request.\n\nThinking: Security implications\n\nPlease contact support.",
			expectedOutput: "It's complicated because we're dealing with user's data.\n\nI don't think we can process this request.\n\nPlease contact support.",
			description:    "Complex case with multiple sections and apostrophes",
		},
		{
			name:           "Edge_case_no_Thinking",
			inputContent:   "I don't know, but I'll try to help you understand.",
			expectedOutput: "I don't know, but I'll try to help you understand.",
			description:    "Content without Thinking should remain unchanged",
		},
		{
			name:           "Edge_case_only_Thinking",
			inputContent:   "Thinking: I need to process this request",
			expectedOutput: "",
			description:    "Content with only Thinking should result in empty string",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Apply our fixed Thinking filter
			result := executor.FilterThinkingContent(tc.inputContent)

			// Verify the result matches expected output
			if result != tc.expectedOutput {
				t.Errorf("Fix verification failed for %s.\nDescription: %s\nInput:    %q\nExpected: %q\nGot:      %q",
					tc.name, tc.description, tc.inputContent, tc.expectedOutput, result)
			}

			// Verify apostrophes are preserved when expected
			if testutil.ContainsString(tc.expectedOutput, "'") && !testutil.ContainsString(result, "'") {
				t.Errorf("Apostrophes were lost in %s! Expected: %q, Got: %q",
					tc.name, tc.expectedOutput, result)
			}

			// Verify no truncation occurred
			if testutil.ContainsString(result, "I don") && !testutil.ContainsString(tc.expectedOutput, "I don") {
				t.Errorf("Found truncation 'I don' in result for %s: %q", tc.name, result)
			}
			if testutil.ContainsString(result, "I can") && !testutil.ContainsString(tc.expectedOutput, "I can") {
				t.Errorf("Found truncation 'I can' in result for %s: %q", tc.name, result)
			}

			t.Logf("✓ %s: Fix verified successfully", tc.name)
		})
	}
}

// TestKiroFixRegression ensures our fix doesn't break existing functionality
func TestKiroFixRegression(t *testing.T) {
	// Test cases that should continue to work as before
	regressionCases := []struct {
		name           string
		inputContent   string
		expectedOutput string
		description    string
	}{
		{
			name:           "Empty_content",
			inputContent:   "",
			expectedOutput: "",
			description:    "Empty content should remain empty",
		},
		{
			name:           "Content_without_Thinking",
			inputContent:   "This is a simple response without any thinking sections.",
			expectedOutput: "This is a simple response without any thinking sections.",
			description:    "Content without Thinking should remain unchanged",
		},
		{
			name:           "Content_with_only_newlines",
			inputContent:   "\n\n\n",
			expectedOutput: "\n\n\n",
			description:    "Content with only newlines should remain unchanged (no Thinking to filter)",
		},
		{
			name:           "Thinking_at_very_start",
			inputContent:   "Thinking: I need to analyze this\n\nThis is the actual response.",
			expectedOutput: "This is the actual response.",
			description:    "Thinking at start should be filtered out",
		},
		{
			name:           "Thinking_at_very_end",
			inputContent:   "This is the response.\n\nThinking: I should double check this",
			expectedOutput: "This is the response.",
			description:    "Thinking at end should be filtered out",
		},
	}

	for _, tc := range regressionCases {
		t.Run(tc.name, func(t *testing.T) {
			result := executor.FilterThinkingContent(tc.inputContent)

			if result != tc.expectedOutput {
				t.Errorf("Regression test failed for %s.\nDescription: %s\nInput:    %q\nExpected: %q\nGot:      %q",
					tc.name, tc.description, tc.inputContent, tc.expectedOutput, result)
			}

			t.Logf("✓ %s: Regression test passed", tc.name)
		})
	}
}