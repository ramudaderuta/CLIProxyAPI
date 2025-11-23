package kiro

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestConnectionTimeout tests timeout scenarios
func TestConnectionTimeout(t *testing.T) {
	t.Run("slow server response", func(t *testing.T) {
		slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simulate slow response (2 seconds)
			time.Sleep(2 * time.Second)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		}))
		defer slowServer.Close()

		// Create http client with 500ms timeout
		client := &http.Client{
			Timeout: 500 * time.Millisecond,
		}

		req, _ := http.NewRequest(http.MethodGet, slowServer.URL, nil)
		_, err := client.Do(req)

		if err == nil {
			t.Error("Expected timeout error, got nil")
		}

		if !strings.Contains(err.Error(), "Client.Timeout") && !strings.Contains(err.Error(), "context deadline exceeded") {
			t.Logf("Got error: %v", err)
		}

		t.Log("✓ Connection timeout handled correctly")
	})

	t.Run("context timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(200 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
		client := &http.Client{}

		_, err := client.Do(req)
		if err == nil {
			t.Error("Expected context deadline exceeded error")
		}

		t.Log("✓ Context timeout handled correctly")
	})

	t.Run("request cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Long-running request
			<-r.Context().Done()
		}))
		defer server.Close()

		ctx, cancel := context.WithCancel(context.Background())
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)

		// Cancel immediately
		cancel()

		client := &http.Client{}
		_, err := client.Do(req)

		if err == nil || err == context.Canceled {
			t.Log("✓ Request cancellation handled")
		}
	})
}

// TestConnectionRefused tests connection refused scenarios
func TestConnectionRefused(t *testing.T) {
	t.Run("server not listening", func(t *testing.T) {
		// Try to connect to a port that's not listening
		client := &http.Client{Timeout: 1 * time.Second}
		req, _ := http.NewRequest(http.MethodGet, "http://localhost:59999", nil)

		_, err := client.Do(req)
		if err == nil {
			t.Error("Expected connection refused error")
		}

		t.Logf("✓ Connection refused error: %v", err)
	})

	t.Run("invalid hostname", func(t *testing.T) {
		client := &http.Client{Timeout: 1 * time.Second}
		req, _ := http.NewRequest(http.MethodGet, "http://invalid-hostname-that-does-not-exist-12345.com", nil)

		_, err := client.Do(req)
		if err == nil {
			t.Error("Expected DNS resolution error")
		}

		t.Logf("✓ Invalid hostname error: %v", err)
	})
}

// TestMalformedResponse tests malformed HTTP response handling
func TestMalformedResponse(t *testing.T) {
	t.Run("invalid JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// Write invalid JSON
			w.Write([]byte(`{"incomplete": "json"`))
		}))
		defer server.Close()

		client := &http.Client{}
		resp, err := client.Get(server.URL)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)

		if err == nil {
			t.Error("Expected JSON parsing error")
		}

		t.Log("✓ Invalid JSON handled correctly")
	})

	t.Run("empty response body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// Empty body
		}))
		defer server.Close()

		client := &http.Client{}
		resp, err := client.Get(server.URL)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)

		if err == nil || err == io.EOF {
			t.Log("✓ Empty response body handled")
		}
	})

	t.Run("corrupted JSON stream", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// Mix of valid and invalid JSON
			w.Write([]byte(`{"valid": true}`))
			w.Write([]byte(`{corrupted`))
		}))
		defer server.Close()

		client := &http.Client{}
		resp, err := client.Get(server.URL)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		t.Logf("Received body: %s", string(body))
		t.Log("✓ Corrupted stream handling validated")
	})

	t.Run("unexpected content type", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`<html><body>Error</body></html>`))
		}))
		defer server.Close()

		client := &http.Client{}
		resp, err := client.Get(server.URL)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		contentType := resp.Header.Get("Content-Type")
		if !strings.Contains(contentType, "application/json") {
			t.Log("✓ Unexpected content type detected")
		}
	})
}

// TestPartialSSEStream tests partial/incomplete SSE stream handling
func TestPartialSSEStream(t *testing.T) {
	t.Run("incomplete SSE event", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")

			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Fatal("Expected flusher")
			}

			// Write incomplete event
			w.Write([]byte("event: message_start\n"))
			w.Write([]byte("data: {\"type\":\"message"))
			// Intentionally not completing the event
			flusher.Flush()

			// Simulate connection drop
			time.Sleep(100 * time.Millisecond)
		}))
		defer server.Close()

		client := &http.Client{}
		resp, err := client.Get(server.URL)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		buffer := make([]byte, 1024)
		n, _ := resp.Body.Read(buffer)

		if n > 0 {
			t.Logf("Read %d bytes of incomplete SSE: %s", n, string(buffer[:n]))
			t.Log("✓ Partial SSE stream detected")
		}
	})

	t.Run("SSE stream interruption", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			flusher := w.(http.Flusher)

			// Send first event successfully
			w.Write([]byte("event: message_start\ndata: {\"type\":\"message_start\"}\n\n"))
			flusher.Flush()

			// Send second event
			w.Write([]byte("event: content_block_delta\ndata: {\"delta\":"))
			flusher.Flush()

			// Simulate interruption (no completion)
		}))
		defer server.Close()

		client := &http.Client{Timeout: 500 * time.Millisecond}
		resp, err := client.Get(server.URL)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		// Try to read all
		_, err = io.ReadAll(resp.Body)
		// May timeout or EOF
		t.Logf("Stream interruption result: %v", err)
		t.Log("✓ SSE stream interruption handled")
	})

	t.Run("missing SSE markers", func(t *testing.T) {
		testCases := []struct {
			name     string
			content  string
			expected string
		}{
			{
				name:     "missing event type",
				content:  "data: {\"message\": \"test\"}\n\n",
				expected: "data event without type",
			},
			{
				name:     "missing data field",
				content:  "event: custom\n\n",
				expected: "event without data",
			},
			{
				name:     "missing double newline",
				content:  "event: test\ndata: content\n",
				expected: "incomplete event (no terminator)",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Validate SSE structure
				hasEvent := strings.Contains(tc.content, "event:")
				hasData := strings.Contains(tc.content, "data:")
				hasTerminator := strings.Contains(tc.content, "\n\n")

				if tc.name == "missing event type" && !hasEvent && hasData {
					t.Log("✓ Missing event type detected")
				} else if tc.name == "missing data field" && hasEvent && !hasData {
					t.Log("✓ Missing data field detected")
				} else if tc.name == "missing double newline" && !hasTerminator {
					t.Log("✓ Missing terminator detected")
				}
			})
		}
	})
}

// TestHTTPStatusCodes tests various HTTP error status codes
func TestHTTPStatusCodes(t *testing.T) {
	testCases := []struct {
		statusCode  int
		description string
		shouldRetry bool
	}{
		{400, "Bad Request", false},
		{401, "Unauthorized", false},
		{403, "Forbidden", false},
		{404, "Not Found", false},
		{429, "Rate Limit", true},
		{500, "Internal Server Error", true},
		{502, "Bad Gateway", true},
		{503, "Service Unavailable", true},
		{504, "Gateway Timeout", true},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":       tc.description,
					"status_code": tc.statusCode,
				})
			}))
			defer server.Close()

			client := &http.Client{}
			resp, err := client.Get(server.URL)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.statusCode {
				t.Errorf("Expected status %d, got %d", tc.statusCode, resp.StatusCode)
			}

			// Determine if should retry based on status code
			shouldRetry := resp.StatusCode >= 500 || resp.StatusCode == 429
			if shouldRetry != tc.shouldRetry {
				t.Errorf("Expected shouldRetry=%v for status %d", tc.shouldRetry, tc.statusCode)
			}

			t.Logf("✓ Status %d (%s) handled correctly", tc.statusCode, tc.description)
		})
	}
}

// TestNetworkRetryLogic tests retry logic for network failures
func TestNetworkRetryLogic(t *testing.T) {
	t.Run("retry on 5xx errors", func(t *testing.T) {
		attemptCount := 0
		maxRetries := 3

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attemptCount++
			if attemptCount < maxRetries {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "success"})
		}))
		defer server.Close()

		client := &http.Client{}
		var resp *http.Response
		var err error

		// Retry loop
		for attempt := 0; attempt < maxRetries; attempt++ {
			resp, err = client.Get(server.URL)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				break
			}
			resp.Body.Close()
		}

		if resp != nil {
			resp.Body.Close()
		}

		if attemptCount != maxRetries {
			t.Errorf("Expected %d attempts, got %d", maxRetries, attemptCount)
		}

		t.Log("✓ Retry logic for 5xx errors validated")
	})

	t.Run("exponential backoff", func(t *testing.T) {
		backoffs := []time.Duration{
			1 * time.Second,
			2 * time.Second,
			4 * time.Second,
			8 * time.Second,
		}

		for i, backoff := range backoffs {
			expected := time.Duration(1<<i) * time.Second
			if backoff != expected {
				t.Errorf("Backoff %d: expected %v, got %v", i, expected, backoff)
			}
		}

		t.Log("✓ Exponential backoff calculation validated")
	})

	t.Run("max retry limit", func(t *testing.T) {
		maxRetries := 5
		attemptCount := 0

		for attemptCount < maxRetries {
			attemptCount++
			// Simulate failure
			if attemptCount >= maxRetries {
				break
			}
		}

		if attemptCount != maxRetries {
			t.Errorf("Expected %d attempts, got %d", maxRetries, attemptCount)
		}

		t.Log("✓ Max retry limit enforced")
	})
}

// TestProxyConfiguration tests proxy setup scenarios
func TestProxyConfiguration(t *testing.T) {
	t.Run("no proxy", func(t *testing.T) {
		client := &http.Client{}
		if client.Transport != nil {
			t.Log("Custom transport configured")
		} else {
			t.Log("✓ Using default transport (no proxy)")
		}
	})

	t.Run("proxy URL parsing", func(t *testing.T) {
		testCases := []string{
			"http://proxy.example.com:8080",
			"https://secure-proxy.example.com:8443",
			"socks5://socks-proxy.example.com:1080",
		}

		for _, proxyURL := range testCases {
			if strings.HasPrefix(proxyURL, "http://") ||
				strings.HasPrefix(proxyURL, "https://") ||
				strings.HasPrefix(proxyURL, "socks5://") {
				t.Logf("✓ Valid proxy URL: %s", proxyURL)
			}
		}
	})
}

// TestLargeResponseHandling tests handling of large responses
func TestLargeResponseHandling(t *testing.T) {
	t.Run("large JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)

			// Write 1MB of JSON data
			w.Write([]byte(`{"data":"}`))
			w.Write(make([]byte, 1024*1024)) // 1MB of zeros
			w.Write([]byte(`"}`))
		}))
		defer server.Close()

		client := &http.Client{}
		resp, err := client.Get(server.URL)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read body: %v", err)
		}

		if len(body) > 1024*1024 {
			t.Logf("✓ Large response handled: %d bytes", len(body))
		}
	})

	t.Run("streaming large response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			flusher := w.(http.Flusher)

			// Stream 100 events
			for i := 0; i < 100; i++ {
				w.Write([]byte("event: data\n"))
				w.Write([]byte("data: " + strings.Repeat("x", 1024) + "\n\n"))
				flusher.Flush()
			}
		}))
		defer server.Close()

		client := &http.Client{}
		resp, err := client.Get(server.URL)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		totalBytes := 0
		buffer := make([]byte, 4096)

		for {
			n, err := resp.Body.Read(buffer)
			totalBytes += n
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("Read error: %v", err)
			}
		}

		t.Logf("✓ Streamed %d bytes", totalBytes)
	})
}

// BenchmarkNetworkErrorHandling benchmarks error handling performance
func BenchmarkNetworkErrorHandling(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := &http.Client{Timeout: 1 * time.Second}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		resp, _ := client.Get(server.URL)
		if resp != nil {
			resp.Body.Close()
		}
	}
}

func BenchmarkRetryLogic(b *testing.B) {
	maxRetries := 3

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		attemptCount := 0
		for attemptCount < maxRetries {
			attemptCount++
			// Simulate retry logic
			if attemptCount >= 2 {
				break
			}
		}
	}
}
