// Package shared provides common test utilities for HTTP testing
package shared

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// RoundTripFunc is a function type that implements http.RoundTripper
type RoundTripFunc func(*http.Request) (*http.Response, error)

// RoundTrip executes the RoundTripFunc
func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// NewTestHTTPClient creates a test HTTP client with a custom round tripper
func NewTestHTTPClient(fn RoundTripFunc) *http.Client {
	return &http.Client{
		Transport: fn,
	}
}

// MockHTTPResponse creates a mock HTTP response
func MockHTTPResponse(statusCode int, body string, headers map[string]string) *http.Response {
	resp := &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}

	for key, value := range headers {
		resp.Header.Set(key, value)
	}

	return resp
}

// MockServer represents a test HTTP server
type MockServer struct {
	*httptest.Server
	RequestLog []*http.Request
	t          *testing.T
}

// NewMockServer creates a new mock HTTP server
func NewMockServer(t *testing.T, handler http.HandlerFunc) *MockServer {
	ms := &MockServer{
		t:          t,
		RequestLog: make([]*http.Request, 0),
	}

	// Wrap handler to log requests
	wrappedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Clone request for logging (body can only be read once)
		bodyBytes, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		clonedReq := r.Clone(r.Context())
		clonedReq.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		ms.RequestLog = append(ms.RequestLog, clonedReq)

		handler(w, r)
	})

	ms.Server = httptest.NewServer(wrappedHandler)
	return ms
}

// GetLastRequest returns the most recent request received by the server
func (ms *MockServer) GetLastRequest() *http.Request {
	if len(ms.RequestLog) == 0 {
		ms.t.Fatal("No requests logged")
	}
	return ms.RequestLog[len(ms.RequestLog)-1]
}

// GetRequestCount returns the number of requests received
func (ms *MockServer) GetRequestCount() int {
	return len(ms.RequestLog)
}

// Reset clears the request log
func (ms *MockServer) Reset() {
	ms.RequestLog = make([]*http.Request, 0)
}

// SSEWriter writes Server-Sent Events
type SSEWriter struct {
	w io.Writer
}

// NewSSEWriter creates a new SSE writer
func NewSSEWriter(w io.Writer) *SSEWriter {
	return &SSEWriter{w: w}
}

// WriteEvent writes an SSE event
func (sw *SSEWriter) WriteEvent(event, data string) error {
	if event != "" {
		if _, err := sw.w.Write([]byte("event: " + event + "\n")); err != nil {
			return err
		}
	}

	if _, err := sw.w.Write([]byte("data: " + data + "\n\n")); err != nil {
		return err
	}

	return nil
}

// MockSSEResponse creates a mock SSE streaming response
func MockSSEResponse(events []struct{ Event, Data string }) *http.Response {
	var buf bytes.Buffer
	writer := NewSSEWriter(&buf)

	for _, e := range events {
		writer.WriteEvent(e.Event, e.Data)
	}

	return &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		Body: io.NopCloser(&buf),
	}
}
