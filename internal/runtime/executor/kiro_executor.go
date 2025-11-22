package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	chat_completions "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro/openai/chat-completions"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro/openai/responses"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
	log "github.com/sirupsen/logrus"
)

const (
	// KiroAPIEndpoint is the default AWS CodeWhisperer endpoint
	KiroAPIEndpoint = "https://codewhisperer.us-east-1.amazonaws.com/v1/conversation"
)

// KiroExecutor implements the executor interface for Kiro API.
type KiroExecutor struct {
	cfg *config.Config
}

// NewKiroExecutor creates a new Kiro executor instance.
func NewKiroExecutor(cfg *config.Config) *KiroExecutor {
	return &KiroExecutor{
		cfg: cfg,
	}
}

// Identifier returns the executor identifier.
func (e *KiroExecutor) Identifier() string {
	return "kiro"
}

// PrepareRequest prepares the HTTP request for Kiro API.
func (e *KiroExecutor) PrepareRequest(req *http.Request, auth *cliproxyauth.Auth) error {
	// This is called before sending the request
	// Token will be added in Execute method
	return nil
}

// Execute performs a non-streaming request to Kiro API.
func (e *KiroExecutor) Execute(
	ctx context.Context,
	auth *cliproxyauth.Auth,
	req cliproxyexecutor.Request,
	opts cliproxyexecutor.Options,
) (resp cliproxyexecutor.Response, err error) {
	log.Debugf("Kiro Execute: model=%s, stream=%v", req.Model, opts.Stream)

	// Load token from auth metadata
	tokenStorage, err := e.loadToken(auth)
	if err != nil {
		return resp, fmt.Errorf("failed to load token: %w", err)
	}

	// Validate and refresh token if needed
	authenticator := kiro.NewKiroAuthenticator(e.cfg)
	validToken, refreshed, err := authenticator.ValidateToken(ctx, tokenStorage)
	if err != nil {
		return resp, fmt.Errorf("token validation failed: %w", err)
	}

	if refreshed {
		log.Info("Token was refreshed, updating auth metadata")
		// Update auth metadata with refreshed token
		if err := e.saveToken(auth, validToken); err != nil {
			log.Warnf("Failed to save refreshed token: %v", err)
		}
	}

	// Translate request to Kiro format
	kiroRequest := chat_completions.ConvertOpenAIRequestToKiro(req.Model, req.Payload, false)

	// Send request to Kiro API
	httpResp, err := e.sendRequest(ctx, validToken, kiroRequest)
	if err != nil {
		return resp, fmt.Errorf("failed to send request: %w", err)
	}
	defer httpResp.Body.Close()

	// Read response body
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return resp, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for HTTP errors
	if httpResp.StatusCode != http.StatusOK {
		return resp, fmt.Errorf("Kiro API error: status %d: %s", httpResp.StatusCode, string(body))
	}

	// Convert response to OpenAI format
	openAIResp := responses.ConvertKiroResponseToOpenAI(body, req.Model, false)

	resp.Payload = openAIResp

	log.Debugf("Kiro Execute completed successfully")
	return resp, nil
}

// ExecuteStream performs a streaming request to Kiro API.
func (e *KiroExecutor) ExecuteStream(
	ctx context.Context,
	auth *cliproxyauth.Auth,
	req cliproxyexecutor.Request,
	opts cliproxyexecutor.Options,
) (stream <-chan cliproxyexecutor.StreamChunk, err error) {
	log.Debugf("Kiro ExecuteStream: model=%s", req.Model)

	// Load and validate token
	tokenStorage, err := e.loadToken(auth)
	if err != nil {
		return nil, fmt.Errorf("failed to load token: %w", err)
	}

	authenticator := kiro.NewKiroAuthenticator(e.cfg)
	validToken, _, err := authenticator.ValidateToken(ctx, tokenStorage)
	if err != nil {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}

	// Translate request to Kiro format with streaming enabled
	kiroRequest := chat_completions.ConvertOpenAIRequestToKiro(req.Model, req.Payload, true)

	// Send request
	httpResp, err := e.sendRequest(ctx, validToken, kiroRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Check status code
	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		httpResp.Body.Close()
		return nil, fmt.Errorf("Kiro API error: status %d: %s", httpResp.StatusCode, string(body))
	}

	// Create streaming channel
	streamChan := make(chan cliproxyexecutor.StreamChunk, 10)

	// Start goroutine to process SSE stream
	go e.processStream(ctx, httpResp.Body, req.Model, streamChan)

	return streamChan, nil
}

// CountTokens estimates token count for a request.
func (e *KiroExecutor) CountTokens(
	ctx context.Context,
	auth *cliproxyauth.Auth,
	req cliproxyexecutor.Request,
	opts cliproxyexecutor.Options,
) (cliproxyexecutor.Response, error) {
	// Simple character-based estimation
	charCount := len(req.Payload)
	estimatedTokens := charCount / 4

	// Return response with usage metadata
	response := cliproxyexecutor.Response{
		Payload: []byte(fmt.Sprintf(`{"prompt_tokens":%d,"total_tokens":%d}`, estimatedTokens, estimatedTokens)),
	}

	return response, nil
}

// Refresh refreshes the Kiro authentication token.
func (e *KiroExecutor) Refresh(ctx context.Context, auth *cliproxyauth.Auth) (*cliproxyauth.Auth, error) {
	log.Debugf("kiro executor: refresh called")
	if auth == nil {
		return nil, fmt.Errorf("kiro executor: auth is nil")
	}

	// Load current token
	tokenStorage, err := e.loadToken(auth)
	if err != nil {
		return nil, fmt.Errorf("failed to load token: %w", err)
	}

	// Refresh the token
	authenticator := kiro.NewKiroAuthenticator(e.cfg)
	newToken, err := authenticator.RefreshToken(ctx, tokenStorage)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	// Save refreshed token to auth
	if err := e.saveToken(auth, newToken); err != nil {
		return nil, fmt.Errorf("failed to save token: %w", err)
	}

	return auth, nil
}

// loadToken loads Kiro token from auth metadata.
func (e *KiroExecutor) loadToken(auth *cliproxyauth.Auth) (*kiro.KiroTokenStorage, error) {
	if auth == nil || auth.Metadata == nil {
		return nil, fmt.Errorf("no auth metadata provided")
	}

	// Marshal metadata to JSON
	metadataJSON, err := json.Marshal(auth.Metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Unmarshal into token storage
	var token kiro.KiroTokenStorage
	if err := json.Unmarshal(metadataJSON, &token); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token: %w", err)
	}

	return &token, nil
}

// saveToken saves Kiro token to auth metadata.
func (e *KiroExecutor) saveToken(auth *cliproxyauth.Auth, token *kiro.KiroTokenStorage) error {
	if auth == nil {
		return fmt.Errorf("auth is nil")
	}

	// Convert token to map
	tokenJSON, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("failed to marshal token: %w", err)
	}

	var tokenMap map[string]interface{}
	if err := json.Unmarshal(tokenJSON, &tokenMap); err != nil {
		return fmt.Errorf("failed to unmarshal to map: %w", err)
	}

	auth.Metadata = tokenMap
	return nil
}

// sendRequest sends an HTTP request to Kiro API.
func (e *KiroExecutor) sendRequest(ctx context.Context, token *kiro.KiroTokenStorage, requestBody []byte) (*http.Response, error) {
	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", KiroAPIEndpoint, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	// Create HTTP client
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	return resp, nil
}

// processStream processes SSE stream from Kiro API.
func (e *KiroExecutor) processStream(ctx context.Context, body io.ReadCloser, model string, streamChan chan cliproxyexecutor.StreamChunk) {
	defer body.Close()
	defer close(streamChan)

	reader := io.Reader(body)
	buf := make([]byte, 4096)

	for {
		select {
		case <-ctx.Done():
			log.Debug("Stream context cancelled")
			return
		default:
		}

		n, err := reader.Read(buf)
		if err != nil {
			if err != io.EOF {
				log.Errorf("Stream read error: %v", err)
				streamChan <- cliproxyexecutor.StreamChunk{
					Err: err,
				}
			}
			return
		}

		if n > 0 {
			// Convert chunk to OpenAI format
			chunk := responses.ConvertKiroStreamChunkToOpenAI(buf[:n], model)
			if chunk != nil {
				streamChan <- cliproxyexecutor.StreamChunk{
					Payload: chunk,
				}
			}
		}
	}
}
