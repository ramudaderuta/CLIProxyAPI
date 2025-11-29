package executor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestParseSSEEventsToKiroResponse(t *testing.T) {
	tests := []struct {
		name                    string
		input                   string
		wantContent             string
		wantConversationID      string
		wantFollowupPromptCount int
		wantToolCallCount       int
		wantError               bool
	}{
		{
			name: "simple content only",
			input: `:message-typeevent{"content":"Hello! "}` +
				`:message-typeevent{"content":"How can I help?"}`,
			wantContent:             "Hello! How can I help?",
			wantConversationID:      "",
			wantFollowupPromptCount: 0,
			wantToolCallCount:       0,
			wantError:               false,
		},
		{
			name: "content with conversationId",
			input: `:message-typeevent{"conversationId":"test-123"}` +
				`:message-typeevent{"content":"Response"}`,
			wantContent:             "Response",
			wantConversationID:      "test-123",
			wantFollowupPromptCount: 0,
			wantToolCallCount:       0,
			wantError:               false,
		},
		{
			name: "content with followup prompts",
			input: `:message-typeevent{"content":"Answer"}` +
				`:message-typeevent{"followupPrompt":{"content":"What about X?"}}` +
				`:message-typeevent{"followupPrompt":{"content":"What about Y?"}}`,
			wantContent:             "Answer",
			wantConversationID:      "",
			wantFollowupPromptCount: 2,
			wantToolCallCount:       0,
			wantError:               false,
		},
		{
			name: "complete response with all fields",
			input: `:message-typeevent{"conversationId":"abc-456"}` +
				`:message-typeevent{"content":"Part 1 "}` +
				`:message-typeevent{"content":"Part 2"}` +
				`:message-typeevent{"followupPrompt":{"content":"Continue?"}}`,
			wantContent:             "Part 1 Part 2",
			wantConversationID:      "abc-456",
			wantFollowupPromptCount: 1,
			wantToolCallCount:       0,
			wantError:               false,
		},
		{
			name: "with tool calls",
			input: `:message-typeevent{"content":"Using tool..."}` +
				`:message-typeevent{"toolCall":{"id":"call_123","name":"search","input":{"query":"test"}}}`,
			wantContent:             "Using tool...",
			wantConversationID:      "",
			wantFollowupPromptCount: 0,
			wantToolCallCount:       1,
			wantError:               false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseSSEEventsToKiroResponse([]byte(tt.input))

			if tt.wantError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)

			// Parse the result JSON
			parsed := gjson.ParseBytes(result)

			// Check content
			content := parsed.Get("conversationState.currentMessage.content").String()
			assert.Equal(t, tt.wantContent, content, "content mismatch")

			// Check role
			role := parsed.Get("conversationState.currentMessage.role").String()
			assert.Equal(t, "assistant", role, "role should be assistant")

			// Check conversationId
			convID := parsed.Get("conversationState.conversationId").String()
			if tt.wantConversationID != "" {
				assert.Equal(t, tt.wantConversationID, convID, "conversationId mismatch")
			}

			// Check followup prompts count
			followups := parsed.Get("conversationState.followupPrompts").Array()
			assert.Equal(t, tt.wantFollowupPromptCount, len(followups), "followup prompts count mismatch")

			// Check tool calls count
			toolCalls := parsed.Get("conversationState.currentMessage.toolCalls").Array()
			assert.Equal(t, tt.wantToolCallCount, len(toolCalls), "tool calls count mismatch")
		})
	}
}

func TestParseSSEEventsToKiroResponse_EmptyInput(t *testing.T) {
	result, err := parseSSEEventsToKiroResponse([]byte(""))

	require.NoError(t, err)
	require.NotNil(t, result)

	parsed := gjson.ParseBytes(result)
	content := parsed.Get("conversationState.currentMessage.content").String()
	assert.Equal(t, "", content, "empty input should produce empty content")
}

func TestParseSSEEventsToKiroResponse_PlainJSON(t *testing.T) {
	// Test fallback to plain JSON format (without :message-typeevent prefix)
	input := `{"content":"Plain JSON"}{"conversationId":"plain-123"}`

	result, err := parseSSEEventsToKiroResponse([]byte(input))

	require.NoError(t, err)
	require.NotNil(t, result)

	parsed := gjson.ParseBytes(result)
	content := parsed.Get("conversationState.currentMessage.content").String()
	assert.Equal(t, "Plain JSON", content)

	convID := parsed.Get("conversationState.conversationId").String()
	assert.Equal(t, "plain-123", convID)
}
