package iflow_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/runtime/executor"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v6/sdk/translator"
)

// TestIFlowExecutorForwardsCustomHeaders ensures auth-defined headers make it to the upstream request.
func TestIFlowExecutorForwardsCustomHeaders(t *testing.T) {
	t.Parallel()

	const headerName = "X-IFlow-Task-Directive"
	const headerValue = "allow-tool-advance"

	headerChannel := make(chan http.Header, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		_, _ = io.ReadAll(r.Body)
		headerChannel <- r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "chatcmpl-test",
			"object": "chat.completion",
			"created": 1,
			"model": "iflow-model",
			"choices": [{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "ack"
				},
				"finish_reason": "stop"
			}],
			"usage": {
				"prompt_tokens": 1,
				"completion_tokens": 1,
				"total_tokens": 2
			}
		}`))
	}))
	t.Cleanup(server.Close)

	exec := executor.NewOpenAICompatExecutor("iflow", &config.Config{})
	auth := &cliproxyauth.Auth{
		Provider: "iflow",
		Attributes: map[string]string{
			"base_url":             server.URL,
			"api_key":              "sk-test",
			"header:" + headerName: headerValue,
		},
	}

	payload := []byte(`{
		"model": "iflow-model",
		"messages": [{
			"role": "user",
			"content": [{
				"type": "text",
				"text": "hi"
			}]
		}]
	}`)
	req := cliproxyexecutor.Request{
		Model:   "iflow-model",
		Payload: payload,
		Format:  sdktranslator.FromString("openai"),
	}
	opts := cliproxyexecutor.Options{
		OriginalRequest: payload,
		SourceFormat:    sdktranslator.FromString("openai"),
	}

	resp, err := exec.Execute(context.Background(), auth, req, opts)
	require.NoError(t, err, "execute should succeed with mocked upstream")
	require.NotNil(t, resp.Payload, "response payload should not be nil")
	require.NotEmpty(t, resp.Payload, "response payload should not be empty")

	select {
	case headers := <-headerChannel:
		require.Equal(t, headerValue, headers.Get(headerName), "custom header must propagate upstream")
		assert.Equal(t, "cli-proxy-openai-compat", headers.Get("User-Agent"))
	default:
		t.Fatal("expected upstream request headers to be captured")
	}
}
