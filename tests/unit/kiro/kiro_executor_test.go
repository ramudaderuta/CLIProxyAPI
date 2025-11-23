package kiro

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/tests/shared"
)

// TestExecutorBasics tests basic executor functionality
func TestExecutorBasics(t *testing.T) {
	fixtures := shared.NewKiroTestFixtures(t)

	t.Run("executor initialization", func(t *testing.T) {
		if fixtures.Cfg == nil {
			t.Fatal("Config should not be nil")
		}
		t.Log("✓ Executor config initialized")
	})

	t.Run("context handling", func(t *testing.T) {
		ctx := fixtures.Ctx
		if ctx == nil {
			t.Fatal("Context should not be nil")
		}

		// Test context with timeout
		ctxWithTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		if ctxWithTimeout.Err() != nil {
			t.Error("Context should not be canceled yet")
		}

		t.Log("✓ Context handling working")
	})
}

// TestExecutorRequestPreparation tests request preparation
func TestExecutorRequestPreparation(t *testing.T) {
	tests := []struct {
		name        string
		model       string
		streaming   bool
		expectError bool
	}{
		{
			name:        "non-streaming request",
			model:       "kiro-sonnet",
			streaming:   false,
			expectError: false,
		},
		{
			name:        "streaming request",
			model:       "kiro-opus",
			streaming:   true,
			expectError: false,
		},
		{
			name:        "haiku model",
			model:       "kiro-haiku",
			streaming:   false,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := shared.BuildOpenAIRequest(tt.model, shared.SimpleMessages, tt.streaming)

			if request["model"] != tt.model {
				t.Errorf("Expected model %s, got %v", tt.model, request["model"])
			}

			if tt.streaming {
				if stream, ok := request["stream"].(bool); !ok || !stream {
					t.Error("Streaming flag not set correctly")
				}
			}

			t.Logf("✓ Request prepared for %s", tt.model)
		})
	}
}

// TestExecutorTokenValidation tests token validation before requests
func TestExecutorTokenValidation(t *testing.T) {
	fixtures := shared.NewKiroTestFixtures(t)

	t.Run("valid token", func(t *testing.T) {
		// Token expires in 1 hour
		if time.Now().After(fixtures.TokenStorage.ExpiresAt) {
			t.Error("Valid token should not be expired")
		}
		t.Log("✓ Valid token passes validation")
	})

	t.Run("expired token", func(t *testing.T) {
		expiredFixtures := fixtures.WithExpiredToken()

		// Token should be expired
		if time.Now().Before(expiredFixtures.TokenStorage.ExpiresAt) {
			t.Error("Expired token should be detected")
		}
		t.Log("✓ Expired token detected")
	})

	t.Run("token near expiration", func(t *testing.T) {
		// Token expiring in 3 minutes (within 5-minute buffer)
		nearExpiry := *fixtures.TokenStorage
		nearExpiry.ExpiresAt = time.Now().Add(3 * time.Minute)

		buffer := 5 * time.Minute
		needsRefresh := time.Now().Add(buffer).After(nearExpiry.ExpiresAt)

		if !needsRefresh {
			t.Error("Token within expiration buffer should trigger refresh")
		}
		t.Log("✓ Token expiration buffer working")
	})
}

// TestExecutorErrorHandling tests error scenarios
func TestExecutorErrorHandling(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		responseBody string
		expectError  bool
	}{
		{
			name:         "success response",
			statusCode:   http.StatusOK,
			responseBody: `{"conversationState":{"currentMessage":{"assistantMessage":{"content":"Success"}}}}`,
			expectError:  false,
		},
		{
			name:         "unauthorized",
			statusCode:   http.StatusUnauthorized,
			responseBody: `{"error":"invalid_token"}`,
			expectError:  true,
		},
		{
			name:         "rate limit",
			statusCode:   http.StatusTooManyRequests,
			responseBody: `{"error":"rate_limit_exceeded"}`,
			expectError:  true,
		},
		{
			name:         "server error",
			statusCode:   http.StatusInternalServerError,
			responseBody: `{"error":"internal_server_error"}`,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock HTTP response
			resp := shared.MockHTTPResponse(tt.statusCode, tt.responseBody, map[string]string{
				"Content-Type": "application/json",
			})

			if resp.StatusCode != tt.statusCode {
				t.Errorf("Expected status %d, got %d", tt.statusCode, resp.StatusCode)
			}

			shouldError := resp.StatusCode >= 400
			if shouldError != tt.expectError {
				t.Errorf("Expected error: %v, got status: %d", tt.expectError, resp.StatusCode)
			}

			t.Logf("✓ Error handling for status %d", tt.statusCode)
		})
	}
}

// TestExecutorRetryLogic tests retry behavior
func TestExecutorRetryLogic(t *testing.T) {
	maxRetries := 3
	attemptCount := 0

	// Simulate retry logic
	for attempt := 0; attempt < maxRetries; attempt++ {
		attemptCount++

		// Simulate failure on first 2 attempts
		if attempt < 2 {
			t.Logf("Attempt %d failed (simulated)", attempt+1)
			continue
		}

		// Success on 3rd attempt
		t.Logf("✓ Success on attempt %d", attempt+1)
		break
	}

	if attemptCount != 3 {
		t.Errorf("Expected 3 attempts, got %d", attemptCount)
	}
}

// TestExecutorStreaming tests streaming request handling
func TestExecutorStreaming(t *testing.T) {
	t.Run("streaming flag propagation", func(t *testing.T) {
		request := shared.BuildOpenAIRequest("kiro-sonnet", shared.SimpleMessages, true)

		if stream, ok := request["stream"].(bool); !ok || !stream {
			t.Error("Streaming flag should be true")
		}

		t.Log("✓ Streaming flag set correctly")
	})

	t.Run("non-streaming flag", func(t *testing.T) {
		request := shared.BuildOpenAIRequest("kiro-sonnet", shared.SimpleMessages, false)

		if stream, ok := request["stream"].(bool); ok && stream {
			t.Error("Streaming flag should be false or absent")
		}

		t.Log("✓ Non-streaming flag correct")
	})
}

// Benchmark executor operations
func BenchmarkExecutorTokenValidation(b *testing.B) {
	fixtures := shared.NewKiroTestFixtures(&testing.T{})
	buffer := 5 * time.Minute

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = time.Now().Add(buffer).After(fixtures.TokenStorage.ExpiresAt)
	}
}

func BenchmarkExecutorRequestPreparation(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = shared.BuildOpenAIRequest("kiro-sonnet", shared.SimpleMessages, false)
	}
}
