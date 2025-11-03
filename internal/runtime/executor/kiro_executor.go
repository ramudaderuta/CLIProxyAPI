package executor

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	kirotranslator "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
	log "github.com/sirupsen/logrus"
)

// KiroExecutor is a stateless executor for Kiro AI services.
type KiroExecutor struct {
	cfg    *config.Config
	client *kiroClient
}

// NewKiroExecutor creates a new Kiro executor instance.
func NewKiroExecutor(cfg *config.Config) *KiroExecutor {
	return &KiroExecutor{
		cfg:    cfg,
		client: newKiroClient(cfg),
	}
}

// Identifier returns the executor identifier for Kiro.
func (e *KiroExecutor) Identifier() string { return "kiro" }

// PrepareRequest prepares the HTTP request for execution (no-op for Kiro).
func (e *KiroExecutor) PrepareRequest(_ *http.Request, _ *cliproxyauth.Auth) error { return nil }

// Execute performs a non-streaming request to the Kiro API.
func (e *KiroExecutor) Execute(ctx context.Context, auth *cliproxyauth.Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (resp cliproxyexecutor.Response, err error) {
	reporter := newUsageReporter(ctx, e.Identifier(), req.Model, auth)
	defer reporter.trackFailure(ctx, &err)

	result, err := e.performCompletion(ctx, auth, req, opts)
	if err != nil {
		return resp, err
	}

	payload, err := kirotranslator.BuildOpenAIChatCompletionPayload(req.Model, result.Text, result.ToolCalls, result.PromptTokens, result.CompletionTokens)
	if err != nil {
		return resp, err
	}

	reporter.publish(ctx, usageDetail(result.PromptTokens, result.CompletionTokens))

	resp.Payload = payload
	resp.Metadata = map[string]any{
		"provider":   e.Identifier(),
		"model":      req.Model,
		"kiro_model": result.KiroModel,
	}
	return resp, nil
}

// ExecuteStream performs a streaming request to the Kiro API.
func (e *KiroExecutor) ExecuteStream(ctx context.Context, auth *cliproxyauth.Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (<-chan cliproxyexecutor.StreamChunk, error) {
	result, err := e.performCompletion(ctx, auth, req, opts)
	if err != nil {
		return nil, err
	}

	stream := make(chan cliproxyexecutor.StreamChunk, 4)
	go func() {
		defer close(stream)
		created := time.Now().Unix()
		id := fmt.Sprintf("chatcmpl_%s", uuid.NewString())

		for _, payload := range kirotranslator.BuildStreamingChunks(id, req.Model, created, result.Text, result.ToolCalls) {
			stream <- cliproxyexecutor.StreamChunk{Payload: payload}
		}
	}()
	return stream, nil
}

// CountTokens returns an approximate token count for the request payload.
func (e *KiroExecutor) CountTokens(_ context.Context, _ *cliproxyauth.Auth, req cliproxyexecutor.Request, _ cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	enc, err := tokenizerForModel(req.Model)
	if err != nil {
		return cliproxyexecutor.Response{}, err
	}
	count, err := countOpenAIChatTokens(enc, req.Payload)
	if err != nil {
		return cliproxyexecutor.Response{}, err
	}
	return cliproxyexecutor.Response{Payload: buildOpenAIUsageJSON(count)}, nil
}

// Refresh updates the underlying auth by refreshing tokens and persisting metadata.
func (e *KiroExecutor) Refresh(ctx context.Context, auth *cliproxyauth.Auth) (*cliproxyauth.Auth, error) {
	if auth == nil {
		return nil, fmt.Errorf("kiro executor: auth is nil")
	}
	ts, err := e.tokenStorageFromAuth(auth)
	if err != nil {
		return nil, err
	}
	if ts == nil {
		return auth, nil
	}

	if err := e.client.ensureToken(ctx, ts); err != nil {
		return nil, err
	}

	auth.Runtime = ts
	auth.Metadata = attachTokenMetadata(auth.Metadata, ts)
	now := time.Now().UTC()
	auth.LastRefreshedAt = now
	if !ts.ExpiresAt.IsZero() {
		auth.NextRefreshAfter = ts.ExpiresAt.Add(-5 * time.Minute)
	} else {
		auth.NextRefreshAfter = time.Time{}
	}
	if path := e.tokenFilePath(auth); path != "" {
		if err := ts.SaveTokenToFile(path); err != nil {
			log.Warnf("kiro executor: failed to persist refreshed token %s: %v", auth.ID, err)
		}
	}
	return auth, nil
}

func (e *KiroExecutor) performCompletion(ctx context.Context, auth *cliproxyauth.Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (*kiroResult, error) {
	if auth == nil {
		return nil, fmt.Errorf("kiro executor: auth is nil")
	}
	ts, err := e.tokenStorageFromAuth(auth)
	if err != nil {
		return nil, err
	}
	if ts == nil {
		return nil, fmt.Errorf("kiro executor: token storage unavailable")
	}

	regionOverride := ""
	if auth.Attributes != nil {
		regionOverride = strings.TrimSpace(auth.Attributes["region"])
	}

	body, err := kirotranslator.BuildRequest(req.Model, req.Payload, ts, opts.Metadata)
	if err != nil {
		return nil, err
	}

	data, _, _, err := e.client.doRequest(ctx, auth, ts, regionOverride, req.Model, body)
	if err != nil {
		return nil, err
	}

	text, toolCalls := kirotranslator.ParseResponse(data)

	// Filter out "Thinking" content from streaming responses
	if strings.Contains(text, "Thinking") {
		parts := strings.Split(text, "Thinking")
		if len(parts) > 1 {
			text = strings.TrimSpace(parts[1])
		} else {
			text = ""
		}
	}
	promptTokens, _ := estimatePromptTokens(req.Model, req.Payload)
	completionTokens := estimateCompletionTokens(text, toolCalls)

	return &kiroResult{
		Text:             text,
		ToolCalls:        toolCalls,
		KiroModel:        kirotranslator.MapModel(req.Model),
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
	}, nil
}
