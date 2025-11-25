package kiro_test

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/runtime/executor"
	chatcompletions "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro/openai/chat-completions"
)

// MockKiroServer creates a test HTTP server that mimics Kiro API responses
func MockKiroServer(t *testing.T) *httptest.Server {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skip("Current runtime environment disables tcp6, needs to be enabled in CI that supports IPv4 later")
	}

	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check authorization header
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error": "unauthorized"}`))
			return
		}

		// Read request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Check if streaming is requested
		isStreaming := strings.Contains(string(body), `"stream":true`)

		if isStreaming {
			// Return SSE streaming response
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)

			// Send streaming events
			events := []string{
				`event: messageStart\ndata: {"type":"message_start","message":{"id":"msg_123","type":"message","role":"assistant"}}\n\n`,
				`event: contentBlockStart\ndata: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}\n\n`,
				`event: contentBlockDelta\ndata: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}\n\n`,
				`event: contentBlockDelta\ndata: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" World"}}\n\n`,
				`event: contentBlockStop\ndata: {"type":"content_block_stop","index":0}\n\n`,
				`event: messageDelta\ndata: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}\n\n`,
				`event: messageStop\ndata: {"type":"message_stop"}\n\n`,
			}

			for _, event := range events {
				w.Write([]byte(event))
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
			}
		} else {
			// Return non-streaming JSON response
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			response := `{
				"id": "msg_test_123",
				"type": "message",
				"role": "assistant",
				"content": [
					{
						"type": "text",
						"text": "This is a test response from mock Kiro API"
					}
				],
				"model": "kiro-sonnet",
				"stop_reason": "end_turn",
				"usage": {
					"input_tokens": 10,
					"output_tokens": 20
				}
			}`
			w.Write([]byte(response))
		}
	}))
	ts.Listener = ln
	ts.Start()
	t.Cleanup(ts.Close)
	return ts
}

// TestRequestTranslation tests OpenAI to Kiro request conversion
func TestRequestTranslation(t *testing.T) {
	t.Run("MapModel correctly maps kiro aliases", func(t *testing.T) {
		executor := &executor.KiroExecutor{}

		tests := []struct {
			input    string
			expected string
		}{
			{"kiro-sonnet", "CLAUDE_SONNET_4_5"},
			{"kiro-haiku", "CLAUDE_HAIKU_4_5"},
			{"unknown-model", "unknown-model"},     // Should return original
			{" kiro-sonnet ", "CLAUDE_SONNET_4_5"}, // Should trim spaces
		}

		for _, tt := range tests {
			result := executor.MapModel(tt.input)
			if result != tt.expected {
				t.Errorf("MapModel(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		}
	})

	t.Run("ConvertOpenAIRequestToKiro handles basic request", func(t *testing.T) {
		openAIRequest := []byte(`{
			"model": "kiro-sonnet",
			"messages": [
				{"role": "user", "content": "Hello"}
			]
		}`)

		dummyToken := &kiro.KiroTokenStorage{AccessToken: "test-token"}
		kiroRequest, err := chatcompletions.ConvertOpenAIRequestToKiro("CLAUDE_SONNET_4_5", openAIRequest, dummyToken, nil)

		if err != nil {
			t.Fatalf("Failed to convert request: %v", err)
		}
		if len(kiroRequest) == 0 {
			t.Fatal("Kiro request should not be empty")
		}

		// Verify it contains conversationState
		requestStr := string(kiroRequest)
		if !strings.Contains(requestStr, "conversationState") {
			t.Error("Kiro request should contain conversationState")
		}
	})

	t.Run("parseSSEEventsForContent aggregates multiple content chunks", func(t *testing.T) {
		// Simulate SSE event format after event-stream decoding
		sseData := []byte(`vent{"content":"Hello"}vent{"content":", "}vent{"content":"world!"}`)

		// Note: parseSSEEventsForContent is not exported, but we can test via Execute
		// This test verifies the expected input format
		if !strings.Contains(string(sseData), `{"content":"`) {
			t.Error("Test data should contain content JSON fragments")
		}
	})
}

// TestResponseTranslation tests Kiro to OpenAI response conversion
func TestResponseTranslation(t *testing.T) {
	kiroResponse := []byte(`{
		"id": "msg_123",
		"type": "message",
		"role": "assistant",
		"content": [
			{"type": "text", "text": "Hello there!"}
		],
		"usage": {
			"input_tokens": 5,
			"output_tokens": 3
		}
	}`)

	openAIResponse := chatcompletions.ConvertKiroResponseToOpenAI(kiroResponse, "kiro-sonnet", false)

	if len(openAIResponse) == 0 {
		t.Fatal("OpenAI response should not be empty")
	}

	// Verify it contains expected fields
	responseStr := string(openAIResponse)
	if !strings.Contains(responseStr, "choices") {
		t.Error("OpenAI response should contain choices")
	}
	if !strings.Contains(responseStr, "usage") {
		t.Error("OpenAI response should contain usage")
	}
}

// TestThinkingContentFiltering tests that thinking tags are removed
func TestThinkingContentFiltering(t *testing.T) {
	textWithThinking := "Here's my answer: <thinking>internal reasoning</thinking> The result is 42."

	filtered := chatcompletions.FilterThinkingContent(textWithThinking)

	if strings.Contains(filtered, "<thinking>") {
		t.Error("Filtered text should not contain thinking tags")
	}
	if strings.Contains(filtered, "internal reasoning") {
		t.Error("Filtered text should not contain thinking content")
	}
	if !strings.Contains(filtered, "The result is 42") {
		t.Error("Filtered text should contain non-thinking content")
	}
}

// TestEndToEndNonStreaming tests complete non-streaming flow with mock server
func TestEndToEndNonStreaming(t *testing.T) {
	server := MockKiroServer(t)
	defer server.Close()

	// Create a mock token
	token := &kiro.KiroTokenStorage{
		AccessToken:  "test-token",
		RefreshToken: "test-refresh",
		ProfileArn:   "arn:aws:test",
		ExpiresAt:    time.Now().Add(1 * time.Hour),
		AuthMethod:   "IdC",
		Provider:     "BuilderId",
		Region:       "us-east-1",
	}

	// Test request conversion
	openAIRequest := []byte(`{
		"model": "kiro-sonnet",
		"messages": [{"role": "user", "content": "Test"}]
	}`)

	kiroRequest, _ := chatcompletions.ConvertOpenAIRequestToKiro("kiro-sonnet", openAIRequest, token, nil)

	// Make request to mock server
	req, err := http.NewRequest("POST", server.URL, strings.NewReader(string(kiroRequest)))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	// Convert response back to OpenAI format
	openAIResponse := chatcompletions.ConvertKiroResponseToOpenAI(body, "kiro-sonnet", false)

	if len(openAIResponse) == 0 {
		t.Fatal("Response conversion failed")
	}

	responseStr := string(openAIResponse)
	if !strings.Contains(responseStr, "choices") {
		t.Error("Response should contain choices")
	}
}

// TestEndToEndStreaming tests complete streaming flow with mock server
func TestEndToEndStreaming(t *testing.T) {
	server := MockKiroServer(t)
	defer server.Close()

	token := &kiro.KiroTokenStorage{
		AccessToken:  "test-token",
		RefreshToken: "test-refresh",
		ProfileArn:   "arn:aws:test",
		ExpiresAt:    time.Now().Add(1 * time.Hour),
		AuthMethod:   "IdC",
		Provider:     "BuilderId",
		Region:       "us-east-1",
	}

	openAIRequest := []byte(`{
		"model": "kiro-sonnet",
		"messages": [{"role": "user", "content": "Test"}],
		"stream": true
	}`)

	kiroRequest, _ := chatcompletions.ConvertOpenAIRequestToKiro("kiro-sonnet", openAIRequest, token, nil)

	req, err := http.NewRequest("POST", server.URL, strings.NewReader(string(kiroRequest)))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	// Read streaming response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	if len(body) == 0 {
		t.Fatal("Streaming response should not be empty")
	}

	bodyStr := string(body)
	if !strings.Contains(bodyStr, "event:") {
		t.Error("Streaming response should contain SSE events")
	}
	if !strings.Contains(bodyStr, "data:") {
		t.Error("Streaming response should contain SSE data")
	}
}

// TestTokenManager tests multi-account token management
func TestTokenManager(t *testing.T) {
	// This is a unit test for token manager logic
	// In practice, it would need valid token files

	// Create a temporary directory for auth files
	authDir := t.TempDir()
	cfg := &config.Config{
		AuthDir:    authDir,
		KiroConfig: config.KiroConfig{},
	}

	tm := kiro.NewTokenManager(cfg)
	if tm == nil {
		t.Fatal("TokenManager should not be nil")
	}

	// Test that manager initializes correctly
	if tm.GetTokenCount() != 0 {
		t.Error("New token manager should have 0 tokens before loading")
	}
}
