package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	authkiro "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/runtime/executor"
	kirotranslator "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func withRoundTripper(ctx context.Context, fn roundTripperFunc) context.Context {
	return context.WithValue(ctx, "cliproxy.roundtripper", http.RoundTripper(fn))
}

func newTestToken() *authkiro.KiroTokenStorage {
	return &authkiro.KiroTokenStorage{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		ProfileArn:   "arn:aws:codewhisperer:us-west-2:123456789012:profile/test",
		ExpiresAt:    time.Now().Add(30 * time.Minute),
		AuthMethod:   "social",
		Provider:     "Github",
		Type:         "kiro",
	}
}

func newTestAuth(token *authkiro.KiroTokenStorage, attrs map[string]string) *cliproxyauth.Auth {
	if token == nil {
		token = newTestToken()
	}
	if attrs == nil {
		attrs = map[string]string{}
	}
	return &cliproxyauth.Auth{
		ID:         "auth-test",
		Provider:   "kiro",
		Attributes: attrs,
		Metadata:   map[string]any{},
		Runtime:    token,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

func openAIChatPayload(t *testing.T, messages []map[string]any, tools []map[string]any) []byte {
	t.Helper()
	payload := map[string]any{
		"model":    "claude-sonnet-4-5",
		"messages": messages,
	}
	if len(tools) > 0 {
		payload["tools"] = tools
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return data
}

func TestKiroExecutorExecuteIntegration(t *testing.T) {
	cfg := &config.Config{}
	exec := executor.NewKiroExecutor(cfg)
	auth := newTestAuth(nil, map[string]string{"region": "ap-southeast-1"})

	var captured []byte
	rt := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		captured = append([]byte(nil), body...)

		response := map[string]any{
			"conversationState": map[string]any{
				"currentMessage": map[string]any{
					"assistantResponseMessage": map[string]any{
						"content": "Hello from Kiro",
					},
				},
			},
		}
		raw, _ := json.Marshal(response)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(raw)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	ctx := withRoundTripper(context.Background(), rt)
	req := cliproxyexecutor.Request{
		Model: "claude-sonnet-4-5",
		Payload: openAIChatPayload(t, []map[string]any{
			{"role": "system", "content": "You are a helpful assistant."},
			{"role": "user", "content": "Hello!"},
		}, []map[string]any{
			{
				"type": "function",
				"function": map[string]any{
					"name":        "get_weather",
					"description": "Get weather details",
					"parameters": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"location": map[string]any{"type": "string"},
						},
						"required": []any{"location"},
					},
				},
			},
		}),
	}
	opts := cliproxyexecutor.Options{
		Metadata: map[string]any{"project": "demo-project"},
	}

	resp, err := exec.Execute(ctx, auth, req, opts)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if len(resp.Payload) == 0 {
		t.Fatal("expected payload from Execute")
	}
	if resp.Metadata["kiro_model"] != kirotranslator.MapModel(req.Model) {
		t.Fatalf("expected metadata kiro_model to be %s, got %v", kirotranslator.MapModel(req.Model), resp.Metadata["kiro_model"])
	}

	var completion map[string]any
	if err := json.Unmarshal(resp.Payload, &completion); err != nil {
		t.Fatalf("failed to parse completion payload: %v", err)
	}
	choices, _ := completion["choices"].([]any)
	if len(choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(choices))
	}
	first := choices[0].(map[string]any)
	message := first["message"].(map[string]any)
	if got := message["content"]; got != "Hello from Kiro" {
		t.Fatalf("expected assistant content from response, got %v", got)
	}

	var translated map[string]any
	if err := json.Unmarshal(captured, &translated); err != nil {
		t.Fatalf("failed to parse translated request: %v", err)
	}
	state := translated["conversationState"].(map[string]any)
	current := state["currentMessage"].(map[string]any)
	user := current["userInputMessage"].(map[string]any)
	if user["modelId"] != kirotranslator.MapModel(req.Model) {
		t.Fatalf("expected modelId %s, got %v", kirotranslator.MapModel(req.Model), user["modelId"])
	}
	contextBlock, ok := user["userInputMessageContext"].(map[string]any)
	if !ok {
		t.Fatal("expected userInputMessageContext to be present")
	}
	if _, ok := contextBlock["tools"]; !ok {
		t.Fatal("expected tool definitions to be forwarded to Kiro request")
	}
	if convID, _ := state["conversationId"].(string); convID == "" {
		t.Fatal("expected conversationId to be populated")
	}
}

func TestKiroExecutorExecuteStreamIntegration(t *testing.T) {
	cfg := &config.Config{}
	exec := executor.NewKiroExecutor(cfg)
	auth := newTestAuth(nil, nil)

	sse := `
data: {"content":"Thinking","followupPrompt":false}
data: {"name":"get_weather","toolUseId":"call_1","input":{"location":"Seattle"}}
data: {"name":"get_weather","toolUseId":"call_1","stop":true}
data: {"content":"It is sunny","followupPrompt":false}
`

	rt := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte(sse))),
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		}, nil
	})

	ctx := withRoundTripper(context.Background(), rt)
	req := cliproxyexecutor.Request{
		Model:   "claude-sonnet-4-5",
		Payload: openAIChatPayload(t, []map[string]any{{"role": "user", "content": "Weather?"}}, nil),
	}
	stream, err := exec.ExecuteStream(ctx, auth, req, cliproxyexecutor.Options{})
	if err != nil {
		t.Fatalf("ExecuteStream returned error: %v", err)
	}

	var chunks [][]byte
	for chunk := range stream {
		if chunk.Err != nil {
			t.Fatalf("received chunk error: %v", chunk.Err)
		}
		chunks = append(chunks, append([]byte(nil), chunk.Payload...))
	}

	if len(chunks) < 4 {
		t.Fatalf("expected at least 4 streaming chunks, got %d", len(chunks))
	}

	var first map[string]any
	if err := json.Unmarshal(chunks[0], &first); err != nil {
		t.Fatalf("failed to parse first chunk: %v", err)
	}
	choices, _ := first["choices"].([]any)
	delta := choices[0].(map[string]any)["delta"].(map[string]any)
	if delta["role"] != "assistant" {
		t.Fatalf("expected streaming delta role assistant, got %v", delta["role"])
	}

	var contentChunk map[string]any
	if err := json.Unmarshal(chunks[1], &contentChunk); err != nil {
		t.Fatalf("failed to parse content chunk: %v", err)
	}
	choices = contentChunk["choices"].([]any)
	delta = choices[0].(map[string]any)["delta"].(map[string]any)
	if delta["content"] != "It is sunny" {
		t.Fatalf("expected assistant content from stream, got %v", delta["content"])
	}

	var toolChunk map[string]any
	if err := json.Unmarshal(chunks[2], &toolChunk); err != nil {
		t.Fatalf("failed to parse tool chunk: %v", err)
	}
	choices = toolChunk["choices"].([]any)
	delta = choices[0].(map[string]any)["delta"].(map[string]any)
	toolCalls, _ := delta["tool_calls"].([]any)
	if len(toolCalls) != 1 {
		t.Fatalf("expected 1 tool call delta, got %d", len(toolCalls))
	}
	tc := toolCalls[0].(map[string]any)
	function := tc["function"].(map[string]any)
	if function["name"] != "get_weather" {
		t.Fatalf("expected tool call name get_weather, got %v", function["name"])
	}
	if function["arguments"] == "" {
		t.Fatal("expected tool call arguments to be populated")
	}
}

func TestKiroExecutorExecute_ErrorPropagation(t *testing.T) {
	cfg := &config.Config{}
	exec := executor.NewKiroExecutor(cfg)
	auth := newTestAuth(nil, nil)

	rt := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Body:       io.NopCloser(bytes.NewReader([]byte(`{"message":"quota exceeded"}`))),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	ctx := withRoundTripper(context.Background(), rt)
	req := cliproxyexecutor.Request{
		Model:   "claude-sonnet-4-5",
		Payload: openAIChatPayload(t, []map[string]any{{"role": "user", "content": "Hello"}}, nil),
	}
	_, err := exec.Execute(ctx, auth, req, cliproxyexecutor.Options{})
	if err == nil {
		t.Fatal("expected Execute to surface error")
	}
	statusErr, ok := err.(cliproxyexecutor.StatusError)
	if !ok {
		t.Fatalf("expected StatusError, got %T: %v", err, err)
	}
	if statusErr.StatusCode() != http.StatusTooManyRequests {
		t.Fatalf("expected status code 429, got %d", statusErr.StatusCode())
	}
}

func TestKiroExecutorConcurrentExecute(t *testing.T) {
	cfg := &config.Config{}
	exec := executor.NewKiroExecutor(cfg)
	auth := newTestAuth(nil, nil)

	var calls atomic.Int32
	rt := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		calls.Add(1)
		response := map[string]any{
			"conversationState": map[string]any{
				"currentMessage": map[string]any{
					"assistantResponseMessage": map[string]any{
						"content": "ok",
					},
				},
			},
		}
		raw, _ := json.Marshal(response)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(raw)),
		}, nil
	})

	ctx := withRoundTripper(context.Background(), rt)
	req := cliproxyexecutor.Request{
		Model:   "claude-sonnet-4-5",
		Payload: openAIChatPayload(t, []map[string]any{{"role": "user", "content": "ping"}}, nil),
	}

	const workers = 8
	const perWorker = 5
	errCh := make(chan error, workers*perWorker)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				if _, err := exec.Execute(ctx, auth, req, cliproxyexecutor.Options{}); err != nil {
					errCh <- err
				}
			}
		}()
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Fatalf("concurrent execute failed: %v", err)
	}

	want := int32(workers * perWorker)
	if got := calls.Load(); got != want {
		t.Fatalf("expected %d upstream calls, got %d", want, got)
	}
}
