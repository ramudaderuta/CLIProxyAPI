package tests

import (
	"encoding/json"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro"
)

func TestKiroApostropheHandling(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		expectedText     string
		shouldContain    []string
		shouldNotContain []string
	}{
		{
			name:          "Basic apostrophe preservation in Kiro format",
			input:         `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "I don't understand what you mean."}}}}`,
			expectedText:  "I don't understand what you mean.",
			shouldContain: []string{"don't", "understand", "mean"},
		},
		{
			name:          "Multiple apostrophes in Kiro response",
			input:         `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "It's a beautiful day, isn't it? We're going to the park."}}}}`,
			expectedText:  "It's a beautiful day, isn't it? We're going to the park.",
			shouldContain: []string{"It's", "isn't", "We're"},
		},
		{
			name:          "Apostrophes in contractions and possessives",
			input:         `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "John's car won't start, but he's going to fix it."}}}}`,
			expectedText:  "John's car won't start, but he's going to fix it.",
			shouldContain: []string{"John's", "won't", "he's"},
		},
		{
			name:          "Mixed quotes and apostrophes",
			input:         `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "She said \"It's fine\" and then left."}}}}`,
			expectedText:  `She said "It's fine" and then left.`,
			shouldContain: []string{"It's", "fine", "said"},
		},
		{
			name:          "Apostrophes in tool call arguments",
			input:         `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "Processing weather request"}, "toolUse": [{"name": "get_weather", "arguments": "{\"city\": \"St. John's\", \"query\": \"What's the weather?\"}"}]}}}`,
			expectedText:  "Processing weather request",
			shouldContain: []string{"Processing weather request"},
		},
		{
			name:          "Complex nested JSON with apostrophes",
			input:         `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "We can't process this because it's invalid"}}}}`,
			expectedText:  "We can't process this because it's invalid",
			shouldContain: []string{"can't", "it's"},
		},
		{
			name:          "Unicode characters with apostrophes",
			input:         `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "Café's menu isn't available today"}}}}`,
			expectedText:  "Café's menu isn't available today",
			shouldContain: []string{"Café's", "isn't"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, toolCalls := kiro.ParseResponse([]byte(tt.input))

			// Check that expected text is preserved
			if tt.expectedText != "" && content != tt.expectedText {
				t.Errorf("Expected content %q, got %q", tt.expectedText, content)
			}

			// Check that apostrophe-containing text is preserved
			for _, expected := range tt.shouldContain {
				if !containsString(content, expected) {
					t.Errorf("Expected result to contain %q, but got: %q", expected, content)
				}
			}

			// Check that unwanted text is not present
			for _, unexpected := range tt.shouldNotContain {
				if containsString(content, unexpected) {
					t.Errorf("Expected result NOT to contain %q, but got: %q", unexpected, content)
				}
			}

			// Verify the result is valid JSON if we marshal it back
			if content != "" {
				testJSON := map[string]interface{}{
					"content": content,
				}
				if _, err := json.Marshal(testJSON); err != nil {
					t.Errorf("Result content should be JSON-marshalable: %v", err)
				}
			}

			// Verify tool calls are preserved when expected
			if len(toolCalls) > 0 {
				for _, expected := range tt.shouldContain {
					found := false
					for _, call := range toolCalls {
						if containsString(call.Arguments, expected) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected tool calls to contain %q, but didn't find in any call", expected)
					}
				}
			}
		})
	}
}

func TestFirstValidJSONWithApostrophes(t *testing.T) {
	// Test firstValidJSON function directly through ParseResponse using Kiro format
	tests := []struct {
		name         string
		input        string
		expectedJSON string
	}{
		{
			name:         "Simple JSON with apostrophe in Kiro format",
			input:        `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "I don't care"}}}}`,
			expectedJSON: `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "I don't care"}}}}`,
		},
		{
			name:         "JSON with escaped apostrophe in Kiro format",
			input:        `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "He said \"It's fine\""}}}}`,
			expectedJSON: `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "He said \"It's fine\""}}}}`,
		},
		{
			name:         "JSON with multiple apostrophes in Kiro format",
			input:        `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "We're going, and they're coming too"}}}}`,
			expectedJSON: `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "We're going, and they're coming too"}}}}`,
		},
		{
			name:         "Nested JSON with apostrophes in Kiro format",
			input:        `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "It's working"}}}}`,
			expectedJSON: `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "It's working"}}}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, _ := kiro.ParseResponse([]byte(tt.input))

			// For content extraction tests, we're mainly checking that apostrophes are preserved
			// The exact JSON structure might be transformed by extraction process
			if !containsString(content, "'") && containsString(tt.input, "'") {
				t.Errorf("Apostrophes were lost! Input: %q, Result: %q", tt.input, content)
			}
		})
	}
}

func TestSpecialCharacterPreservation(t *testing.T) {
	// Common English contractions with apostrophes
	contractions := []string{
		`don't`, `won't`, `can't`, `it's`, `that's`, `we're`, `they're`, `you're`, `I'm`,
		`John's`, `Mary's`, `isn't`, `aren't`, `wasn't`, `weren't`, `haven't`, `hasn't`,
		`won't've`, `didn't`, `doesn't`, `don't`, `couldn't`, `wouldn't`, `shouldn't`, `mightn't`, `mustn't`,
		`let's`, `let's`, `there's`, `here's`, `what's`, `where's`, `who's`, `how's`, `when's`,
		`you've`, `we've`, `they've`, `I've`, `you'd`, `he'd`, `she'd`, `we'd`, `they'd`,
		`you'll`, `we'll`, `they'll`, `I'll`, `it'll`, `that'll`, `there'll`, `who'll`,
		`I'd`, `you'd`, `he'd`, `she'd`, `we'd`, `they'd`,
	}

	for _, testCase := range contractions {
		t.Run("Contraction_Preserve_"+testCase, func(t *testing.T) {
			input := `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "This contains ` + testCase + ` in the sentence"}}}}`
			content, _ := kiro.ParseResponse([]byte(input))

			if !containsString(content, testCase) {
				t.Errorf("Expected result to contain contraction %q, but got: %q", testCase, content)
			}
		})
	}

	// Test special symbols and punctuation that could cause truncation
	specialSymbols := []struct {
		name   string
		symbol string
	}{
		{"Apostrophe", "'"},
		{"Quotation Mark", `"`},
		{"Backslash", `\`},
		{"Forward Slash", "/"},
		{"Backslash", "\\"},
		{"Question Mark", "?"},
		{"Exclamation Mark", "!"},
		{"Comma", ","},
		{"Period", "."},
		{"Colon", ":"},
		{"Semicolon", ";"},
		{"Parentheses Open", "("},
		{"Parentheses Close", ")"},
		{"Brackets Open", "["},
		{"Brackets Close", "]"},
		{"Braces Open", "{"},
		{"Braces Close", "}"},
		{"Angle Brackets Open", "<"},
		{"Angle Brackets Close", ">"},
		{"Asterisk", "*"},
		{"Ampersand", "&"},
		{"Percent", "%"},
		{"Plus", "+"},
		{"Equals", "="},
		{"Hash", "#"},
		{"At Symbol", "@"},
		{"Dollar Sign", "$"},
		{"Caret", "^"},
		{"Tilde", "~"},
		{"Grave Accent", "`"},
		{"Pipe", "|"},
		{"Underscore", "_"},
		{"Dash", "-"},
	}

	for _, testCase := range specialSymbols {
		t.Run("Symbol_Preserve_"+testCase.name, func(t *testing.T) {
			input := `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "Test symbol ` + testCase.symbol + ` here"}}}}`
			content, _ := kiro.ParseResponse([]byte(input))

			if !containsString(content, testCase.symbol) {
				t.Errorf("Expected result to contain symbol %q, but got: %q", testCase.symbol, content)
			}
		})
	}

	// Test edge cases that could trigger truncation
	edgeCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Multiple apostrophes in sequence",
			input:    `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "Don't, won't, can't, shouldn't"}}}}`,
			expected: "Don't, won't, can't, shouldn't",
		},
		{
			name:     "Apostrophe at start of text",
			input:    `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "'Tis the season"}}}}`,
			expected: "'Tis the season",
		},
		{
			name:     "Apostrophe at end of text",
			input:    `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "That's all folks'"}}}`,
			expected: "That's all folks'",
		},
		{
			name:     "Mixed apostrophes and quotes",
			input:    `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "He said \"It's fine\" and \"don't worry\""}}}}`,
			expected: `He said "It's fine" and "don't worry"`,
		},
		{
			name:     "Unicode with apostrophes",
			input:    `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "Café's résumé isn't finished"}}}}`,
			expected: "Café's résumé isn't finished",
		},
		{
			name:     "Numbers and apostrophes",
			input:    `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "Class of '99 and students' grades"}}}}`,
			expected: "Class of '99 and students' grades",
		},
		{
			name:     "Multiple consecutive apostrophes",
			input:    `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "Rock 'n' roll is good"}}}}`,
			expected: "Rock 'n' roll is good",
		},
		{
			name:     "Apostrophe in technical terms",
			input:    `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "Don't use eval() - it's dangerous"}}}}`,
			expected: "Don't use eval() - it's dangerous",
		},
		{
			name:     "Apostrophe in code examples",
			input:    `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "Use printf() not cout() for C++"}}}}`,
			expected: "Use printf() not cout() for C++",
		},
		{
			name:     "Apostrophe in file paths",
			input:    `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "Check user's home directory"}}}}`,
			expected: "Check user's home directory",
		},
	}

	for _, testCase := range edgeCases {
		t.Run("EdgeCase_"+testCase.name, func(t *testing.T) {
			content, _ := kiro.ParseResponse([]byte(testCase.input))

			if !containsString(content, testCase.expected) {
				t.Errorf("Expected result to contain %q, but got: %q", testCase.expected, content)
			}
		})
	}
}
