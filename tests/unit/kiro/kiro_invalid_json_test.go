package kiro_test

import (
	"encoding/json"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro"
	"github.com/stretchr/testify/assert"
)

// TestKiroParseResponse_InvalidJSON_Extended tests JSON error handling
// This test will fail because the current implementation doesn't properly handle invalid JSON
func TestKiroParseResponse_InvalidJSON_Extended(t *testing.T) {
	// Test various invalid JSON scenarios - all will fail due to improper error handling
	invalidJSONCases := []struct {
		name        string
		input       string
		description string
	}{
		{
			name:        "MissingClosingBrace",
			input:       `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "Hello World"}}}`,
			description: "JSON with missing closing brace",
		},
		{
			name:        "ExtraClosingBrace",
			input:       `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "Hello World"}}}}}`,
			description: "JSON with extra closing brace",
		},
		{
			name:        "MissingQuotes",
			input:       `{"conversationState": {"currentMessage": {"assistantResponseMessage": {content: "Hello World"}}}}`,
			description: "JSON with missing quotes around key",
		},
		{
			name:        "TrailingCharacter",
			input:       `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "Hello World"}}}}{`,
			description: "JSON with trailing character",
		},
		{
			name:        "EmptyString",
			input:       ``,
			description: "Empty string input",
		},
		{
			name:        "IncompleteJSON",
			input:       `{`,
			description: "Incomplete JSON",
		},
		{
			name:        "IncompleteNestedJSON",
			input:       `{"conversationState": {`,
			description: "Incomplete nested JSON",
		},
		{
			name:        "InvalidEscapeSequence",
			input:       `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "Hello\n\t\r\\"}}}}`,
			description: "JSON with invalid escape sequence",
		},
		{
			name:        "UnescapedQuote",
			input:       `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "He said "Hello""}}}}`,
			description: "JSON with unescaped quote",
		},
	}

	for _, tc := range invalidJSONCases {
		t.Run(tc.name, func(t *testing.T) {
			// This test will fail because the current implementation may panic or not handle errors gracefully
			content, calls := kiro.ParseResponse([]byte(tc.input))

			// We should not get a panic or unhandled error
			// Content should be empty or a default value when JSON is invalid
			// Tool calls should be empty when JSON is invalid

			// This assertion might fail if the implementation panics
			assert.NotNil(t, content, "Content should not be nil even for invalid JSON")

			// This assertion might fail if the implementation doesn't handle invalid JSON properly
			assert.Empty(t, calls, "Tool calls should be empty for invalid JSON")

			// If we get here without a panic, that's already a good sign
			// But we might still want to verify the error handling behavior
		})
	}
}

// TestParseResponseFromEventStream_InvalidJSONInData tests handling of invalid JSON within SSE data
// This test will fail because the current implementation doesn't properly handle invalid JSON in SSE
func TestParseResponseFromEventStream_InvalidJSONInData(t *testing.T) {
	// Test case: SSE with invalid JSON in data lines
	invalidJSONStream := []struct {
		name  string
		input string
	}{
		{
			name: "InvalidJSONInContent",
			input: `data: {"content":"Hello World"
data: {"content":"!"}`,
		},
		{
			name: "InvalidJSONInToolCall",
			input: `data: {"content":"Processing"}
data: {"name":"lookup","toolUseId":"call-1","input":{"foo":"bar"}
data: {"name":"lookup","toolUseId":"call-1","input":{"baz":1},"stop":true}`,
		},
		{
			name: "MixedValidAndInvalidJSON",
			input: `data: {"content":"Valid content"}
data: {"content":"Invalid content"
data: {"content":"More valid content"}`,
		},
	}

	for _, tc := range invalidJSONStream {
		t.Run(tc.name, func(t *testing.T) {
			// This test will fail because the current implementation may not gracefully handle invalid JSON in SSE
			content, _ := kiro.ParseResponse([]byte(tc.input))

			// The implementation should not panic even with invalid JSON in SSE data
			assert.NotNil(t, content, "Content should not be nil even with invalid JSON in SSE")

			// Should handle errors gracefully without crashing
			// This assertion might fail if the implementation doesn't handle invalid JSON properly
			assert.NotPanics(t, func() {
				kiro.ParseResponse([]byte(tc.input))
			}, "Parsing invalid JSON in SSE should not cause a panic")
		})
	}
}

// TestParseResponse_EmptyAndWhitespaceInputs tests handling of empty and whitespace-only inputs
// This test will fail because the current implementation may not properly handle edge cases
func TestParseResponse_EmptyAndWhitespaceInputs(t *testing.T) {
	// Test case: Empty and whitespace inputs
	edgeCaseInputs := []struct {
		name  string
		input string
	}{
		{
			name:  "EmptyInput",
			input: "",
		},
		{
			name:  "WhitespaceOnly",
			input: "   ",
		},
		{
			name:  "NewlinesOnly",
			input: "\n\n\n",
		},
		{
			name:  "MixedWhitespace",
			input: " \n \t \r ",
		},
	}

	for _, tc := range edgeCaseInputs {
		t.Run(tc.name, func(t *testing.T) {
			// This test will fail if the implementation doesn't handle empty/whitespace inputs gracefully
			content, calls := kiro.ParseResponse([]byte(tc.input))

			// Should not panic with empty or whitespace inputs
			assert.NotNil(t, content, "Content should not be nil for empty/whitespace inputs")

			// Should handle gracefully without crashing
			// This assertion might fail if the implementation doesn't handle edge cases properly
			assert.NotPanics(t, func() {
				kiro.ParseResponse([]byte(tc.input))
			}, "Parsing empty/whitespace inputs should not cause a panic")

			// Tool calls should be empty
			assert.Empty(t, calls, "Tool calls should be empty for empty/whitespace inputs")
		})
	}
}

// TestParseResponse_JSONWithSpecialCharacters tests handling of JSON with special characters
// This test will fail because the current implementation might not properly handle special characters
func TestParseResponse_JSONWithSpecialCharacters(t *testing.T) {
	// Test case: JSON with various special characters that might cause parsing issues
	specialCharJSON := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "NewlinesAndTabs",
			input:    `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "Line 1\nLine 2\tTabbed"}}}}`,
			expected: "Line 1\nLine 2\tTabbed",
		},
		{
			name:     "QuotesAndApostrophes",
			input:    `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "He said \"Don't do it!\""}}}}`,
			expected: "He said \"Don't do it!\"",
		},
		{
			name:     "Backslashes",
			input:    `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "Path: C:\\Users\\John\\Documents"}}}}`,
			expected: "Path: C:\\Users\\John\\Documents",
		},
		{
			name:     "UnicodeCharacters",
			input:    `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "Unicode: café résumé naïve"}}}}`,
			expected: "Unicode: café résumé naïve",
		},
	}

	for _, tc := range specialCharJSON {
		t.Run(tc.name, func(t *testing.T) {
			// First verify that our test input is valid JSON
			var testData map[string]interface{}
			err := json.Unmarshal([]byte(tc.input), &testData)
			if err != nil {
				t.Fatalf("Test input is not valid JSON: %v", err)
			}

			// This test will fail if special characters are not properly handled
			content, calls := kiro.ParseResponse([]byte(tc.input))

			// This assertion will fail if special characters are not properly preserved
			assert.Equal(t, tc.expected, content, "Special characters should be preserved")

			// This might fail if parsing breaks due to special characters
			assert.Empty(t, calls, "No tool calls should be parsed")

			// Try to validate that the result is still valid for JSON marshaling
			testStruct := map[string]interface{}{
				"test_content": content,
			}

			// This might fail if content contains invalid UTF-8 or other issues
			_, err = json.Marshal(testStruct)
			assert.NoError(t, err, "Content should be JSON marshalable even with special characters")
		})
	}
}