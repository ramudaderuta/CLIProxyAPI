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
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v6/sdk/translator"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
)

const (
	// KiroAPIEndpoint is the default AWS CodeWhisperer endpoint
	KiroAPIEndpoint = "https://codewhisperer.us-east-1.amazonaws.com/generateAssistantResponse"
)

// KiroExecutor implements the executor interface for Kiro API.
type KiroExecutor struct {
	cfg          *config.Config
	tokenManager *kiro.TokenManager
}

// NewKiroExecutor creates a new Kiro executor instance.
func NewKiroExecutor(cfg *config.Config) *KiroExecutor {
	tm := kiro.NewTokenManager(cfg)
	// Best effort load tokens - errors are logged inside LoadTokens
	_ = tm.LoadTokens(context.Background())

	return &KiroExecutor{
		cfg:          cfg,
		tokenManager: tm,
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

	var validToken *kiro.KiroTokenStorage
	var tokenEntry *kiro.TokenEntry

	// Try to get token from TokenManager first
	if e.tokenManager != nil && e.tokenManager.GetTokenCount() > 0 {
		entry, err := e.tokenManager.GetNextToken(ctx)
		if err == nil {
			log.Debugf("Using token from TokenManager: %s", entry.Label)
			validToken = entry.Storage
			tokenEntry = entry
		} else {
			log.Warnf("Failed to get token from TokenManager: %v", err)
		}
	}

	// Fallback to auth metadata if no token from manager
	if validToken == nil {
		log.Debug("Falling back to auth metadata token")
		tokenStorage, err := e.loadToken(auth)
		if err != nil {
			return resp, fmt.Errorf("failed to load token: %w", err)
		}

		// Try to validate and refresh token (best-effort, non-blocking)
		authenticator := kiro.NewKiroAuthenticator(e.cfg)
		vt, refreshed := authenticator.TryValidateToken(ctx, tokenStorage)
		if vt == nil {
			validToken = tokenStorage
		} else {
			validToken = vt
		}

		if refreshed {
			log.Info("Token was proactively refreshed, updating auth metadata")
			if err := e.saveToken(auth, validToken); err != nil {
				log.Warnf("Failed to save proactively refreshed token: %v", err)
			}
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

		authenticator := kiro.NewKiroAuthenticator(e.cfg)

		// Attempt to refresh the token
		newToken, refreshErr := authenticator.RefreshToken(ctx, validToken)
		if refreshErr != nil {
			return resp, fmt.Errorf("401 unauthorized and token refresh failed: %w", refreshErr)
		}

		log.Info("Token refreshed after 401, saving and retrying request")

		// Save the refreshed token to file if we have an entry
		if tokenEntry != nil && tokenEntry.Path != "" {
			if err := newToken.SaveTokenToFile(tokenEntry.Path); err != nil {
				log.Warnf("Failed to save refreshed token to file %s: %v", tokenEntry.Path, err)
			} else {
				log.Infof("Refreshed token persisted to %s", tokenEntry.Path)
				// Update entry storage
				tokenEntry.Storage = newToken
			}
		}

		// Always update auth metadata
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

	// Debug: Log raw response body
	bodyPreview := string(body)
	if len(bodyPreview) > 500 {
		bodyPreview = bodyPreview[:500]
	}
	log.Debugf("Raw Kiro response body (first 500 bytes): %s", bodyPreview)

	// Decode Amazon event-stream binary format if present
	normalizedBody, err := helpers.NormalizeKiroStreamPayload(body)
	if err != nil {
		log.Warnf("Failed to normalize event-stream payload: %v", err)
		normalizedBody = body // Use raw body if normalization fails
	}

	// Debug: Log normalized payload
	normalizedPreview := string(normalizedBody)
	if len(normalizedPreview) > 500 {
		normalizedPreview = normalizedPreview[:500]
	}
	log.Debugf("Normalized payload (first 500 bytes): %s", normalizedPreview)

	// Parse SSE events and build complete Kiro response structure
	kiroResponse, err := parseSSEEventsToKiroResponse(normalizedBody)
	if err != nil {
		log.Warnf("Failed to parse SSE events: %v", err)
		// Fallback to minimal response with extracted content
		kiroResponse = []byte(fmt.Sprintf(`{"conversationState":{"currentMessage":{"content":"","role":"assistant"}}}`))
	}

	// Debug: Log parsed Kiro response
	responsePreview := string(kiroResponse)
	if len(responsePreview) > 500 {
		responsePreview = responsePreview[:500]
	}
	log.Debugf("Parsed Kiro response (first 500 bytes): %s", responsePreview)

	// Check for HTTP errors (after potential retry)
	if httpResp.StatusCode != http.StatusOK {
		return resp, fmt.Errorf("kiro API error: status %d: %s", httpResp.StatusCode, string(body))
	}

	// Log if fallback was used
	if !bytes.Equal(finalRequestBody, kiroRequest) {
		log.Info("Successfully recovered using fallback mechanism")
	}

	// Convert response to OpenAI format
	openAIResp := chat_completions.ConvertKiroResponseToOpenAI(kiroResponse, req.Model, false)

	// Translate response to requested source format when needed
	if opts.SourceFormat != "" && opts.SourceFormat != sdktranslator.FormatOpenAI {
		translated := sdktranslator.TranslateNonStream(
			context.Background(),
			sdktranslator.FormatOpenAI,
			opts.SourceFormat,
			req.Model,
			opts.OriginalRequest,
			kiroRequest,
			openAIResp,
			nil,
		)
		resp.Payload = []byte(translated)
	} else {
		resp.Payload = openAIResp
	}

	log.Debugf("Kiro Execute completed successfully")
	return resp, nil
}

// parseSSEEventsToKiroResponse aggregates ALL SSE events into a complete Kiro response structure.
// Handles assistantResponseEvent (content), messageMetadataEvent (conversationId),
// followupPromptEvent, and toolCall events.
// Returns a JSON bytes representation of the complete conversationState structure.
func parseSSEEventsToKiroResponse(data []byte) ([]byte, error) {
	dataStr := string(data)

	// Aggregation state
	var contentParts []string
	conversationID := ""
	var followupPrompts []map[string]interface{}
	var toolCalls []map[string]interface{}

	// Process the data to find all JSON objects
	offset := 0
	for offset < len(dataStr) {
		// Look for JSON object start with event-stream format marker
		eventMarker := strings.Index(dataStr[offset:], ":message-typeevent{")
		if eventMarker < 0 {
			// Try plain JSON format as fallback
			plainStart := strings.Index(dataStr[offset:], `{"`)
			if plainStart < 0 {
				break
			}
			eventMarker = plainStart
		} else {
			// Adjust for the prefix length
			eventMarker = eventMarker + len(":message-typeevent")
		}

		jsonStart := offset + eventMarker
		if eventMarker == len(`:message-typeevent`) {
			// We found the event-stream marker, adjust offset
			jsonStart = offset + eventMarker
		}

		// Find the end of this JSON object by counting braces
		braceCount := 0
		jsonEnd := -1
		inString := false
		escaped := false

		for i := jsonStart; i < len(dataStr); i++ {
			ch := dataStr[i]

			if escaped {
				escaped = false
				continue
			}

			if ch == '\\' {
				escaped = true
				continue
			}

			if ch == '"' {
				inString = !inString
				continue
			}

			if !inString {
				if ch == '{' {
					braceCount++
				} else if ch == '}' {
					braceCount--
					if braceCount == 0 {
						jsonEnd = i + 1
						break
					}
				}
			}
		}

		if jsonEnd < 0 {
			// Incomplete JSON, stop parsing
			break
		}

		// Extract and parse the JSON object
		jsonStr := dataStr[jsonStart:jsonEnd]
		result := gjson.Parse(jsonStr)

		// Extract different event types
		if content := result.Get("content").String(); content != "" {
			contentParts = append(contentParts, content)
		}

		if convID := result.Get("conversationId").String(); convID != "" {
			conversationID = convID
		}

		if followup := result.Get("followupPrompt"); followup.Exists() {
			followupPrompts = append(followupPrompts, map[string]interface{}{
				"content": followup.Get("content").String(),
			})
		}

		if toolCall := result.Get("toolCall"); toolCall.Exists() {
			toolCalls = append(toolCalls, map[string]interface{}{
				"id":    toolCall.Get("id").String(),
				"name":  toolCall.Get("name").String(),
				"input": toolCall.Get("input").Value(),
			})
		}

		// Move past this JSON object
		offset = jsonEnd
	}

	// Build the complete Kiro response structure
	response := map[string]interface{}{
		"conversationState": map[string]interface{}{
			"currentMessage": map[string]interface{}{
				"role":    "assistant",
				"content": strings.Join(contentParts, ""),
			},
		},
	}

	// Add optional fields if present
	if conversationID != "" {
		response["conversationState"].(map[string]interface{})["conversationId"] = conversationID
	}

	if len(followupPrompts) > 0 {
		response["conversationState"].(map[string]interface{})["followupPrompts"] = followupPrompts
	}

	if len(toolCalls) > 0 {
		response["conversationState"].(map[string]interface{})["currentMessage"].(map[string]interface{})["toolCalls"] = toolCalls
	}

	// Convert to JSON
	jsonBytes, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Kiro response: %w", err)
	}

	return jsonBytes, nil
}

// ExecuteStream performs a streaming request to Kiro API.
func (e *KiroExecutor) ExecuteStream(
	ctx context.Context,
	auth *cliproxyauth.Auth,
	req cliproxyexecutor.Request,
	opts cliproxyexecutor.Options,
) (stream <-chan cliproxyexecutor.StreamChunk, err error) {
	log.Debugf("Kiro ExecuteStream: model=%s", req.Model)

	var validToken *kiro.KiroTokenStorage
	var tokenEntry *kiro.TokenEntry

	// Try to get token from TokenManager first
	if e.tokenManager != nil && e.tokenManager.GetTokenCount() > 0 {
		entry, err := e.tokenManager.GetNextToken(ctx)
		if err == nil {
			log.Debugf("Using token from TokenManager: %s", entry.Label)
			validToken = entry.Storage
			tokenEntry = entry
		} else {
			log.Warnf("Failed to get token from TokenManager: %v", err)
		}
	}

	// Fallback to auth metadata if no token from manager
	if validToken == nil {
		log.Debug("Falling back to auth metadata token")
		tokenStorage, err := e.loadToken(auth)
		if err != nil {
			return nil, fmt.Errorf("failed to load token: %w", err)
		}

		// Try to validate and refresh token (best-effort, non-blocking)
		authenticator := kiro.NewKiroAuthenticator(e.cfg)
		vt, refreshed := authenticator.TryValidateToken(ctx, tokenStorage)
		if vt == nil {
			validToken = tokenStorage
		} else {
			validToken = vt
		}

		if refreshed {
			log.Info("Token was proactively refreshed in stream, updating auth metadata")
			if err := e.saveToken(auth, validToken); err != nil {
				log.Warnf("Failed to save proactively refreshed token in stream: %v", err)
			}
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

		authenticator := kiro.NewKiroAuthenticator(e.cfg)

		// Attempt to refresh the token
		newToken, refreshErr := authenticator.RefreshToken(ctx, validToken)
		if refreshErr != nil {
			return nil, fmt.Errorf("stream 401 unauthorized and token refresh failed: %w", refreshErr)
		}

		log.Info("Stream token refreshed after 401, saving and retrying request")

		// Save the refreshed token to file if we have an entry
		if tokenEntry != nil && tokenEntry.Path != "" {
			if err := newToken.SaveTokenToFile(tokenEntry.Path); err != nil {
				log.Warnf("Failed to save refreshed token to file %s: %v", tokenEntry.Path, err)
			} else {
				log.Infof("Refreshed token persisted to %s", tokenEntry.Path)
				// Update entry storage
				tokenEntry.Storage = newToken
			}
		}

		// Always update auth metadata
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
	go e.processStream(ctx, httpResp.Body, req.Model, opts, kiroRequest, streamChan)

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

	// Log request details to file with pretty formatting
	// Create logs directory if it doesn't exist
	logDir := "/home/build/code/CLIProxyAPI/logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Warnf("Failed to create logs directory: %v", err)
	}

	// Generate timestamped filename: debug_kiro_YYYYMMDD_HHMMSS.log
	timestamp := time.Now().Format("20060102_150405")
	logFile := fmt.Sprintf("%s/debug_kiro_%s.log", logDir, timestamp)

	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err == nil {
		defer f.Close()
		f.WriteString(strings.Repeat("=", 80) + "\n")
		f.WriteString(fmt.Sprintf("KIRO API REQUEST - %s\n", time.Now().Format("2006-01-02 15:04:05")))
		f.WriteString(strings.Repeat("=", 80) + "\n")
		f.WriteString(fmt.Sprintf("URL: %s\n", KiroAPIEndpoint))
		f.WriteString(fmt.Sprintf("Method: POST\n\n"))

		f.WriteString("Headers:\n")
		for k, v := range req.Header {
			if k == "Authorization" {
				f.WriteString(fmt.Sprintf("  %s: Bearer %s...%s\n", k, token.AccessToken[:10], token.AccessToken[len(token.AccessToken)-10:]))
			} else {
				f.WriteString(fmt.Sprintf("  %s: %v\n", k, v))
			}
		}

		f.WriteString("\nRequest Body (Pretty-Printed):\n")
		var prettyJSON bytes.Buffer
		if err := json.Indent(&prettyJSON, requestBody, "", "  "); err == nil {
			f.WriteString(prettyJSON.String() + "\n")
		} else {
			f.WriteString(string(requestBody) + "\n")
		}
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
		f.WriteString("\n" + strings.Repeat("-", 80) + "\n")
		f.WriteString(fmt.Sprintf("KIRO API RESPONSE\n"))
		f.WriteString(strings.Repeat("-", 80) + "\n")
		f.WriteString(fmt.Sprintf("Status: %s\n", resp.Status))
		f.WriteString(fmt.Sprintf("Status Code: %d\n\n", resp.StatusCode))

		f.WriteString("Response Headers:\n")
		for k, v := range resp.Header {
			f.WriteString(fmt.Sprintf("  %s: %v\n", k, v))
		}
		f.WriteString("\n")

		// Read and log response body
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body = io.NopCloser(bytes.NewReader(bodyBytes)) // Restore body for later use

		f.WriteString("Response Body (Pretty-Printed):\n")
		var prettyResp bytes.Buffer
		if err := json.Indent(&prettyResp, bodyBytes, "", "  "); err == nil {
			f.WriteString(prettyResp.String() + "\n")
		} else {
			f.WriteString(string(bodyBytes) + "\n")
		}

		// Validate response structure
		f.WriteString("\n" + strings.Repeat("-", 80) + "\n")
		f.WriteString("VALIDATION:\n")
		f.WriteString(strings.Repeat("-", 80) + "\n")

		if resp.StatusCode == http.StatusOK {
			f.WriteString("✓ HTTP Status: OK (200)\n")

			// Check for required Kiro response fields
			var respData map[string]interface{}
			if err := json.Unmarshal(bodyBytes, &respData); err == nil {
				if _, ok := respData["conversationState"]; ok {
					f.WriteString("✓ Has conversationState field\n")
					if convState, ok := respData["conversationState"].(map[string]interface{}); ok {
						if currentMsg, ok := convState["currentMessage"]; ok {
							f.WriteString("✓ Has conversationState.currentMessage field\n")
							if msg, ok := currentMsg.(map[string]interface{}); ok {
								if content, ok := msg["content"].(string); ok {
									f.WriteString(fmt.Sprintf("✓ Content length: %d characters\n", len(content)))
									if len(content) > 0 {
										f.WriteString("✓ Content is non-empty\n")
									} else {
										f.WriteString("⚠ WARNING: Content is empty\n")
									}
								} else {
									f.WriteString("✗ Missing or invalid content field\n")
								}
							}
						} else {
							f.WriteString("✗ Missing currentMessage field\n")
						}
					}
				} else {
					f.WriteString("⚠ No conversationState field (might be different format)\n")
				}
			} else {
				f.WriteString(fmt.Sprintf("✗ Failed to parse JSON: %v\n", err))
			}
		} else {
			f.WriteString(fmt.Sprintf("✗ HTTP Status: %s (%d)\n", resp.Status, resp.StatusCode))
		}

		f.WriteString(strings.Repeat("=", 80) + "\n\n")
	}

	return resp, nil
}

// processStream processes SSE stream from Kiro API.
func (e *KiroExecutor) processStream(ctx context.Context, body io.ReadCloser, model string, opts cliproxyexecutor.Options, requestRawJSON []byte, streamChan chan cliproxyexecutor.StreamChunk) {
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
			chunk := chat_completions.ConvertKiroStreamChunkToOpenAI(buf[:n], model)
			if chunk != nil {
				if opts.SourceFormat != "" && opts.SourceFormat != sdktranslator.FormatOpenAI {
					converted := sdktranslator.TranslateStream(
						ctx,
						sdktranslator.FormatOpenAI,
						opts.SourceFormat,
						model,
						opts.OriginalRequest,
						requestRawJSON,
						chunk,
						nil,
					)
					for _, line := range converted {
						streamChan <- cliproxyexecutor.StreamChunk{
							Payload: []byte(line),
						}
					}
					continue
				}
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
		if content == "" {
			if u, ok := msg["userInputMessage"].(map[string]interface{}); ok {
				if c, ok := u["content"].(string); ok {
					content = c
				}
			}
		}
		if content == "" {
			if a, ok := msg["assistantResponseMessage"].(map[string]interface{}); ok {
				if c, ok := a["content"].(string); ok {
					content = c
				}
			}
		}

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
		if userMsg, ok := currentMsg["userInputMessage"].(map[string]interface{}); ok {
			text, _ := userMsg["content"].(string)
			userMsg["content"] = "(Continuing from previous context) " + text
		} else {
			currentMsg["content"] = "(Continuing from previous context) " + content
		}
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
