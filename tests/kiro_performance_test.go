package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/runtime/executor"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
)

func BenchmarkKiroExecutorExecute(b *testing.B) {
	fixtures := NewKiroTestFixtures()
	cfg := &config.Config{}
	exec := executor.NewKiroExecutor(cfg)
	auth := fixtures.NewTestAuth(nil, nil)

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
	rt := RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(raw)),
		}, nil
	})

	ctx := fixtures.WithRoundTripper(context.Background(), rt)
	req := cliproxyexecutor.Request{
		Model:   "claude-sonnet-4-5",
		Payload: fixtures.OpenAIChatPayloadNoHelper([]map[string]any{{"role": "user", "content": "ping"}}, nil),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := exec.Execute(ctx, auth, req, cliproxyexecutor.Options{}); err != nil {
			b.Fatalf("execute failed: %v", err)
		}
	}
}

func BenchmarkKiroExecutorExecuteParallel(b *testing.B) {
	fixtures := NewKiroTestFixtures()
	cfg := &config.Config{}
	exec := executor.NewKiroExecutor(cfg)
	auth := fixtures.NewTestAuth(nil, nil)

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
	rt := RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(raw)),
		}, nil
	})

	payload := fixtures.OpenAIChatPayloadNoHelper([]map[string]any{{"role": "user", "content": "ping"}}, nil)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ctx := fixtures.WithRoundTripper(context.Background(), rt)
		req := cliproxyexecutor.Request{
			Model:   "claude-sonnet-4-5",
			Payload: payload,
		}
		for pb.Next() {
			if _, err := exec.Execute(ctx, auth, req, cliproxyexecutor.Options{}); err != nil {
				b.Fatalf("execute failed: %v", err)
			}
		}
	})
}
