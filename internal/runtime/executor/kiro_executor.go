package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	kiro "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro/helpers"
	chat_completions "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro/openai/chat-completions"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro/openai/responses"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
)

const (
	// KiroAPIEndpoint is the default AWS CodeWhisperer endpoint
	KiroAPIEndpoint = "https://codewhisperer.us-east-1.amazonaws.com/generateAssistantResponse"
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

// MapModel returns the upstream Kiro model identifier for the provided alias.
func (e *KiroExecutor) MapModel(model string) string {
	modelMapping := map[string]string{
		"kiro-sonnet": "CLAUDE_SONNET_4_5",
		"kiro-haiku":  "CLAUDE_HAIKU_4_5",
	}

	trimmedModel := strings.TrimSpace(model)
	if mapped, ok := modelMapping[trimmedModel]; ok {
		return mapped
	}
	// Return original model if not in mapping
	return trimmedModel
}

// Execute performs a non-streaming request to Kiro API with 3-level fallback.
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

	// Try to validate and refresh token (best-effort, non-blocking)
	// Even if this fails, we'll proceed with the request and let the API be the source of truth
	authenticator := kiro.NewKiroAuthenticator(e.cfg)
	validToken, refreshed := authenticator.TryValidateToken(ctx, tokenStorage)
	if validToken == nil {
		// TryValidateToken returned nil, fall back to original
		validToken = tokenStorage
	}

	if refreshed {
		log.Info("Token was proactively refreshed, updating auth metadata")
		if err := e.saveToken(auth, validToken); err != nil {
			log.Warnf("Failed to save proactively refreshed token: %v", err)
		}
	}

	// Map model to Kiro ID
	kiroModel := e.MapModel(req.Model)
	log.Debugf("Mapped model %s to %s", req.Model, kiroModel)

	// Translate request to Kiro format
	kiroRequest, err := chat_completions.ConvertOpenAIRequestToKiro(kiroModel, req.Payload, validToken, opts.Metadata)
	if err != nil {
		return resp, fmt.Errorf("failed to build Kiro request: %w", err)
	}

	// Try request with 3-level fallback mechanism
	httpResp, finalRequestBody, err := e.attemptRequestWithFallback(ctx, validToken, kiroRequest)

	// Handle 401 Unauthorized - attempt token refresh and retry once
	if httpResp != nil && httpResp.StatusCode == http.StatusUnauthorized {
		log.Info("Received 401 Unauthorized, attempting token refresh and retry")
		httpResp.Body.Close()

		// Attempt to refresh the token
		newToken, refreshErr := authenticator.RefreshToken(ctx, validToken)
		if refreshErr != nil {
			return resp, fmt.Errorf("401 unauthorized and token refresh failed: %w", refreshErr)
		}

		log.Info("Token refreshed after 401, saving and retrying request")
		// Save the refreshed token
		if err := e.saveToken(auth, newToken); err != nil {
			log.Warnf("Failed to save refreshed token after 401: %v", err)
		}

		// Retry the request with the new token
		// Need to rebuild request with new token
		kiroRequest, err = chat_completions.ConvertOpenAIRequestToKiro(kiroModel, req.Payload, newToken, opts.Metadata)
		if err != nil {
			return resp, fmt.Errorf("failed to rebuild Kiro request after refresh: %w", err)
		}

		httpResp, finalRequestBody, err = e.attemptRequestWithFallback(ctx, newToken, kiroRequest)
		if err != nil {
			return resp, fmt.Errorf("retry after token refresh failed: %w", err)
		}
	} else if err != nil {
		// Non-401 error from initial request
		return resp, err
	}
	defer httpResp.Body.Close()

	// Read response body
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return resp, fmt.Errorf("failed to read response: %w", err)
	}
	// Decode Amazon event-stream binary format if present
	body, err = helpers.NormalizeKiroStreamPayload(body)
	if err != nil {
		log.Warnf("Failed to normalize event-stream payload: %v", err)
	}

	// Parse SSE events and aggregate content
	aggregatedContent := parseSSEEventsForContent(body)

	// Build a conversationState-like response for the converter
	kiroResponse := fmt.Sprintf(`{"conversationState":{"currentMessage":{"content":"%s"}}}`, strings.ReplaceAll(aggregatedContent, `"`, `\"`))

	// Check for HTTP errors (after potential retry)
	if httpResp.StatusCode != http.StatusOK {
		return resp, fmt.Errorf("kiro API error: status %d: %s", httpResp.StatusCode, string(body))
	}

	// Log if fallback was used
	if !bytes.Equal(finalRequestBody, kiroRequest) {
		log.Info("Successfully recovered using fallback mechanism")
	}

	// Convert response to OpenAI format
	openAIResp := responses.ConvertKiroResponseToOpenAI([]byte(kiroResponse), req.Model, false)

	resp.Payload = openAIResp

	log.Debugf("Kiro Execute completed successfully")
	return resp, nil
}

// parseSSEEventsForContent extracts content from SSE events using gjson
func parseSSEEventsForContent(data []byte) string {
	// Find all {"content":"..."} JSON objects and extract content
	var result strings.Builder
	dataStr := string(data)

	// Search for all occurrences of {"content":"
	for {
		start := strings.Index(dataStr, `{"content":"`)
		if start < 0 {
			break
		}

		// Find matching }
		end := strings.Index(dataStr[start:], `"}`)
		if end < 0 {
			break
		}

		// Extract JSON and parse with gjson
		jsonStr := dataStr[start : start+end+2]
		content := gjson.Get(jsonStr, "content").String()
		if content != "" {
			result.WriteString(content)
		}

		// Move past this JSON object
		dataStr = dataStr[start+end+2:]
	}

	return result.String()
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

	// Try to validate and refresh token (best-effort, non-blocking)
	authenticator := kiro.NewKiroAuthenticator(e.cfg)
	validToken, refreshed := authenticator.TryValidateToken(ctx, tokenStorage)
	if validToken == nil {
		validToken = tokenStorage
	}

	if refreshed {
		log.Info("Token was proactively refreshed in stream, updating auth metadata")
		if err := e.saveToken(auth, validToken); err != nil {
			log.Warnf("Failed to save proactively refreshed token in stream: %v", err)
		}
	}

	// Map model to Kiro ID
	kiroModel := e.MapModel(req.Model)
	log.Debugf("Mapped model %s to %s", req.Model, kiroModel)

	// Translate request to Kiro format with streaming enabled
	kiroRequest, err := chat_completions.ConvertOpenAIRequestToKiro(kiroModel, req.Payload, validToken, opts.Metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to build Kiro request: %w", err)
	}

	// Send request
	httpResp, err := e.sendRequest(ctx, validToken, kiroRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Handle 401 Unauthorized - attempt token refresh and retry once
	if httpResp.StatusCode == http.StatusUnauthorized {
		log.Info("Stream received 401 Unauthorized, attempting token refresh and retry")
		body, _ := io.ReadAll(httpResp.Body)
		httpResp.Body.Close()
		log.Debugf("401 response body: %s", string(body))

		// Attempt to refresh the token
		newToken, refreshErr := authenticator.RefreshToken(ctx, validToken)
		if refreshErr != nil {
			return nil, fmt.Errorf("stream 401 unauthorized and token refresh failed: %w", refreshErr)
		}

		log.Info("Stream token refreshed after 401, saving and retrying request")
		// Save the refreshed token
		if err := e.saveToken(auth, newToken); err != nil {
			log.Warnf("Failed to save refreshed token after stream 401: %v", err)
		}

		// Retry with new token - rebuild request
		kiroRequest, err = chat_completions.ConvertOpenAIRequestToKiro(kiroModel, req.Payload, newToken, opts.Metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to rebuild Kiro stream request after refresh: %w", err)
		}

		httpResp, err = e.sendRequest(ctx, newToken, kiroRequest)
		if err != nil {
			return nil, fmt.Errorf("stream retry after token refresh failed: %w", err)
		}

		// Check status code again after retry
		if httpResp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(httpResp.Body)
			httpResp.Body.Close()
			return nil, fmt.Errorf("Kiro stream API error after retry: status %d: %s", httpResp.StatusCode, string(body))
		}
	} else if httpResp.StatusCode != http.StatusOK {
		// Non-401 error
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

	// Add Kiro-specific headers with proper hash
	// SHA256 of "test"
	macHash := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	req.Header.Set("x-amz-user-agent", "aws-sdk-js/1.0.7 KiroIDE-0.1.25-"+macHash)
	req.Header.Set("user-agent", "aws-sdk-js/1.0.7 ua/2.1 os/cli lang/go api/codewhispererstreaming#1.0.7 m/E KiroIDE-0.1.25-"+macHash)
	req.Header.Set("amz-sdk-request", "attempt=1; max=1")
	req.Header.Set("x-amzn-kiro-agent-mode", "vibe")

	// Log request details to file
	f, err := os.OpenFile("/home/build/code/CLIProxyAPI/debug_kiro.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		defer f.Close()
		f.WriteString(fmt.Sprintf("Kiro Request URL: %s\n", KiroAPIEndpoint))
		f.WriteString(fmt.Sprintf("Kiro Request Headers: %v\n", req.Header))
		f.WriteString(fmt.Sprintf("Kiro Request Body: %s\n", string(requestBody)))
	}

	// Create HTTP client
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Log response details to file
	if f != nil {
		f.WriteString(fmt.Sprintf("Kiro Response Status: %s\n", resp.Status))
		f.WriteString(fmt.Sprintf("Kiro Response Headers: %v\n", resp.Header))
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

// attemptRequestWithFallback tries request with 3-level fallback mechanism.
// Returns the HTTP response and the final request body used.
func (e *KiroExecutor) attemptRequestWithFallback(
	ctx context.Context,
	token *kiro.KiroTokenStorage,
	requestBody []byte,
) (*http.Response, []byte, error) {
	// Level 1: Try primary request (full structured conversationState)
	log.Debug("Attempting primary request with full history")
	httpResp, err := e.sendRequest(ctx, token, requestBody)

	if err == nil && !isImproperlFormedError(httpResp) {
		return httpResp, requestBody, nil
	}

	// Close failed response
	if httpResp != nil {
		httpResp.Body.Close()
	}

	// Level 2: Try flattened history (text-only history)
	log.Warn("Primary request failed with 'Improperly formed', trying flattened history")
	flattenedBody := e.flattenHistory(requestBody)
	httpResp, err = e.sendRequest(ctx, token, flattenedBody)

	if err == nil && !isImproperlFormedError(httpResp) {
		log.Info("Flattened history request succeeded")
		return httpResp, flattenedBody, nil
	}

	// Close failed response
	if httpResp != nil {
		httpResp.Body.Close()
	}

	// Level 3: Try minimal request (no history)
	log.Warn("Flattened request failed, trying minimal request with no history")
	minimalBody := e.buildMinimalRequest(requestBody)
	httpResp, err = e.sendRequest(ctx, token, minimalBody)

	if err == nil && !isImproperlFormedError(httpResp) {
		log.Info("Minimal request succeeded")
		return httpResp, minimalBody, nil
	}

	// All fallbacks failed
	if httpResp != nil {
		httpResp.Body.Close()
	}
	return nil, nil, fmt.Errorf("all fallback attempts failed: %w", err)
}

// flattenHistory converts structured history to text-only format.
func (e *KiroExecutor) flattenHistory(requestBody []byte) []byte {
	// Parse request JSON
	var req map[string]interface{}
	if err := json.Unmarshal(requestBody, &req); err != nil {
		log.Warnf("Failed to parse request for flattening: %v", err)
		return requestBody
	}

	// Get conversationState
	convState, ok := req["conversationState"].(map[string]interface{})
	if !ok {
		return requestBody
	}

	// Get and flatten history
	history, ok := convState["history"].([]interface{})
	if !ok || len(history) == 0 {
		return requestBody // No history to flatten
	}

	// Convert history to text transcript
	var textHistory []string
	for _, item := range history {
		msg, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		role, _ := msg["role"].(string)
		content, _ := msg["content"].(string)

		if role != "" && content != "" {
			textHistory = append(textHistory, fmt.Sprintf("%s: %s", role, content))
		}
	}

	// Replace history with text-only version
	flattenedHistory := []interface{}{
		map[string]interface{}{
			"role":    "user",
			"content": "Previous conversation:\n" + string(bytes.Join([][]byte{[]byte(fmt.Sprint(textHistory))}, []byte("\n"))) + "\n\n(Structured tool transcripts were flattened to text for compatibility)",
		},
	}
	convState["history"] = flattenedHistory

	// Re-encode
	flattened, err := json.Marshal(req)
	if err != nil {
		log.Warnf("Failed to marshal flattened request: %v", err)
		return requestBody
	}

	return flattened
}

// buildMinimalRequest creates minimal request with no history.
func (e *KiroExecutor) buildMinimalRequest(requestBody []byte) []byte {
	// Parse request JSON
	var req map[string]interface{}
	if err := json.Unmarshal(requestBody, &req); err != nil {
		log.Warnf("Failed to parse request for minimal: %v", err)
		return requestBody
	}

	// Get conversationState
	convState, ok := req["conversationState"].(map[string]interface{})
	if !ok {
		return requestBody
	}

	// Clear history
	convState["history"] = []interface{}{}

	// Add note to current message
	currentMsg, ok := convState["currentMessage"].(map[string]interface{})
	if ok {
		content, _ := currentMsg["content"].(string)
		currentMsg["content"] = "(Continuing from previous context) " + content
	}

	// Re-encode
	minimal, err := json.Marshal(req)
	if err != nil {
		log.Warnf("Failed to marshal minimal request: %v", err)
		return requestBody
	}

	return minimal
}

// isImproperlFormedError checks if response indicates "Improperly formed request".
func isImproperlFormedError(resp *http.Response) bool {
	if resp == nil || resp.StatusCode != http.StatusBadRequest {
		return false
	}

	// Read body to check error message
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}

	// Re-wrap body for future reads
	resp.Body = io.NopCloser(bytes.NewReader(body))

	// Check if error contains "improperly formed" or "malformed"
	bodyStr := string(bytes.ToLower(body))
	return bytes.Contains([]byte(bodyStr), []byte("improperly formed")) ||
		bytes.Contains([]byte(bodyStr), []byte("malformed"))
}
