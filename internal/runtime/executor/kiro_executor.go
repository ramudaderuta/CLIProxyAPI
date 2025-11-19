package executor

import (
	"bytes"
	"context"
	"encoding/json"
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
	cfg          *config.Config
	client       *kiroClient
	tokenRotator *kiroTokenRotator
}

type requestAttempt struct {
	label string
	body  []byte
}

// NewKiroExecutor creates a new Kiro executor instance.
func NewKiroExecutor(cfg *config.Config) *KiroExecutor {
	return &KiroExecutor{
		cfg:          cfg,
		client:       newKiroClient(cfg),
		tokenRotator: newKiroTokenRotator(cfg),
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

	// Detect request format and build appropriate response
	format := e.detectRequestFormat(req)
	var payload []byte

	if format == "anthropic" {
		payload, err = kirotranslator.BuildAnthropicMessagePayload(req.Model, result.Text, result.ToolCalls, result.PromptTokens, result.CompletionTokens)
	} else {
		// Default to OpenAI format
		payload, err = kirotranslator.BuildOpenAIChatCompletionPayload(req.Model, result.Text, result.ToolCalls, result.PromptTokens, result.CompletionTokens)
	}

	if err != nil {
		return resp, err
	}

	reporter.publish(ctx, usageDetail(result.PromptTokens, result.CompletionTokens))

	resp.Payload = payload
	resp.Metadata = map[string]any{
		"provider":   e.Identifier(),
		"model":      req.Model,
		"kiro_model": result.KiroModel,
		"format":     format,
	}
	return resp, nil
}

// ExecuteStream performs a streaming request to the Kiro API.
func (e *KiroExecutor) ExecuteStream(ctx context.Context, auth *cliproxyauth.Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (<-chan cliproxyexecutor.StreamChunk, error) {
	result, err := e.performCompletion(ctx, auth, req, opts)
	if err != nil {
		return nil, err
	}

	// Detect request format and use appropriate streaming format
	format := e.detectRequestFormat(req)

	stream := make(chan cliproxyexecutor.StreamChunk, 4)
	go func() {
		defer close(stream)
		created := time.Now().Unix()
		id := fmt.Sprintf("chatcmpl_%s", uuid.NewString())

		chunks := result.StreamChunks
		if len(chunks) == 0 {
			if format == "anthropic" {
				chunks = kirotranslator.BuildAnthropicStreamingChunks(id, req.Model, created, result.Text, result.ToolCalls, result.PromptTokens, result.CompletionTokens)
			} else {
				// Default to OpenAI format
				chunks = kirotranslator.BuildStreamingChunks(id, req.Model, created, result.Text, result.ToolCalls)
			}
		}

		for _, payload := range chunks {
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
	ts, err := e.tokenStorageFromAuth(ctx, auth)
	if err != nil {
		return nil, err
	}
	if ts == nil {
		return auth, nil
	}

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

// DetectRequestFormat detects whether the request is OpenAI or Anthropic format
func (e *KiroExecutor) DetectRequestFormat(req cliproxyexecutor.Request) string {
	if len(req.Payload) == 0 {
		return "unknown"
	}

	var payload map[string]any
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		return "unknown"
	}

	// Check for Anthropic format by looking for max_tokens field
	if _, hasMaxTokens := payload["max_tokens"]; hasMaxTokens {
		// Additional check for messages array to confirm it's Anthropic
		if _, hasMessages := payload["messages"]; hasMessages {
			return "anthropic"
		}
	}

	// Check for OpenAI format by looking for messages array without max_tokens
	if _, hasMessages := payload["messages"]; hasMessages {
		if _, hasMaxTokens := payload["max_tokens"]; !hasMaxTokens {
			return "openai"
		}
	}

	return "unknown"
}

// detectRequestFormat is an alias for DetectRequestFormat (kept for backward compatibility)
func (e *KiroExecutor) detectRequestFormat(req cliproxyexecutor.Request) string {
	return e.DetectRequestFormat(req)
}

func (e *KiroExecutor) performCompletion(ctx context.Context, auth *cliproxyauth.Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (*kiroResult, error) {
	log.WithField("executor", "kiro").Debug("performCompletion invoked")
	if auth == nil {
		return nil, fmt.Errorf("kiro executor: auth is nil")
	}
	ts, err := e.tokenStorageFromAuth(ctx, auth)
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

	primaryBody, err := kirotranslator.BuildRequest(req.Model, req.Payload, ts, opts.Metadata)
	if err != nil {
		return nil, err
	}

	attempts := make([]requestAttempt, 0, 3)
	attempts = append(attempts, requestAttempt{label: "primary", body: primaryBody})
	if flattened, ferr := BuildFlattenedKiroRequest(primaryBody); ferr == nil {
		attempts = append(attempts, requestAttempt{label: "flattened", body: flattened})
	} else {
		log.WithField("executor", "kiro").Debugf("kiro fallback builder (flattened) failed: %v", ferr)
	}
	if minimal, ferr := BuildMinimalKiroRequest(primaryBody); ferr == nil {
		attempts = append(attempts, requestAttempt{label: "minimal", body: minimal})
	} else {
		log.WithField("executor", "kiro").Debugf("kiro fallback builder (minimal) failed: %v", ferr)
	}

	var data []byte
	var attemptBody []byte
	var lastErr error

	for idx, attempt := range attempts {
		attemptBody = attempt.body
		log.WithField("executor", "kiro").Debugf("sending %s kiro request attempt", attempt.label)
		data, _, _, err = e.client.doRequest(ctx, auth, ts, regionOverride, req.Model, attemptBody)
		if err != nil {
			lastErr = err
			if shouldAttemptFlattenedFallback(err) && idx+1 < len(attempts) {
				log.WithField("executor", "kiro").Warnf("kiro request (%s) failed due to %v; trying %s variant", attempt.label, err, attempts[idx+1].label)
				continue
			}
			return nil, err
		}
		if isImproperlyFormedResponsePayload(data) {
			lastErr = fmt.Errorf("improperly formed request")
			if idx+1 < len(attempts) {
				log.WithField("executor", "kiro").Warnf("kiro request (%s) returned Improperly formed request; trying %s variant", attempt.label, attempts[idx+1].label)
				continue
			}
			return nil, fmt.Errorf("kiro executor: Improperly formed request after %s attempt", attempt.label)
		}
		break
	}

	if lastErr != nil && data == nil {
		return nil, lastErr
	}

	text, toolCalls := kirotranslator.ParseResponse(data)

	// Filter out "Thinking" content from streaming responses while preserving actual response content
	originalText := text
	text = FilterThinkingContent(text)

	// Debug logging to understand truncation
	if len(text) != len(originalText) {
		log.Debugf("FilterThinkingContent: %d -> %d chars", len(originalText), len(text))
		log.Debugf("Original: %q", originalText)
		log.Debugf("Filtered: %q", text)
	}
	promptTokens, _ := estimatePromptTokens(req.Model, req.Payload)
	completionTokens := estimateCompletionTokens(text, toolCalls)
	streamChunks := kirotranslator.ConvertKiroStreamToAnthropic(data, req.Model, promptTokens, completionTokens)

	return &kiroResult{
		Text:             text,
		ToolCalls:        toolCalls,
		KiroModel:        kirotranslator.MapModel(req.Model),
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		StreamChunks:     streamChunks,
	}, nil
}

// FilterThinkingContent removes Thinking sections from text while preserving actual response content.
// This function handles various Thinking section formats and preserves content that appears before,
// between, and after Thinking sections.
func FilterThinkingContent(text string) string {
	if text == "" {
		return text
	}

	// Handle standalone "Thinking" text (common in JSON content)
	if text == "Thinking" {
		return ""
	}

	// If no Thinking content, return as-is
	if !strings.Contains(text, "Thinking") {
		return text
	}

	// Split by lines to handle Thinking sections properly
	lines := strings.Split(text, "\n")
	var filteredLines []string
	var inThinkingBlock bool
	var pendingSeparator bool

	appendBlank := func() {
		if len(filteredLines) == 0 || filteredLines[len(filteredLines)-1] != "" {
			filteredLines = append(filteredLines, "")
		}
	}

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		if strings.HasPrefix(trimmedLine, "Thinking") || strings.HasPrefix(trimmedLine, "Thinking:") {
			inThinkingBlock = true
			pendingSeparator = false
			continue
		}

		if inThinkingBlock {
			if trimmedLine == "" {
				pendingSeparator = true
				continue
			}
			inThinkingBlock = false
			pendingSeparator = true
		}

		if trimmedLine == "" {
			if !inThinkingBlock {
				appendBlank()
			}
			continue
		}

		if pendingSeparator {
			appendBlank()
			pendingSeparator = false
		}

		filteredLines = append(filteredLines, line)
	}

	// Join the filtered lines and clean up extra whitespace
	result := strings.Join(filteredLines, "\n")

	for strings.Contains(result, "\n\n\n") {
		result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	}

	return strings.TrimSpace(result)
}

func BuildFlattenedKiroRequest(original []byte) ([]byte, error) {
	var root map[string]any
	if err := json.Unmarshal(original, &root); err != nil {
		return nil, err
	}
	cs, _ := root["conversationState"].(map[string]any)
	if cs == nil {
		return nil, fmt.Errorf("kiro fallback: missing conversationState")
	}
	historyAny, _ := cs["history"].([]any)

	lines := make([]string, 0, len(historyAny)+4)
	for _, raw := range historyAny {
		entry, _ := raw.(map[string]any)
		if entry == nil {
			continue
		}
		if user, ok := entry["userInputMessage"].(map[string]any); ok {
			lines = append(lines, summarizeUserEntry(user)...)
			continue
		}
		if assistant, ok := entry["assistantResponseMessage"].(map[string]any); ok {
			lines = append(lines, summarizeAssistantEntry(assistant)...)
		}
	}

	current, _ := cs["currentMessage"].(map[string]any)
	if current == nil {
		return nil, fmt.Errorf("kiro fallback: missing currentMessage")
	}
	uim, _ := current["userInputMessage"].(map[string]any)
	if uim == nil {
		return nil, fmt.Errorf("kiro fallback: missing userInputMessage")
	}
	lines = append(lines, summarizeUserEntry(uim)...)

	flattened := strings.TrimSpace(strings.Join(lines, "\n\n"))
	if flattened == "" {
		flattened = "."
	} else {
		flattened = flattened + "\n\n(Structured tool transcripts were flattened to satisfy Kiro request requirements.)"
	}

	cs["history"] = []any{}

	newMsg := map[string]any{
		"content": flattened,
		"modelId": uim["modelId"],
		"origin":  uim["origin"],
	}
	if images, ok := uim["images"]; ok {
		newMsg["images"] = images
	}
	if ctx, ok := uim["userInputMessageContext"].(map[string]any); ok && len(ctx) > 0 {
		newCtx := map[string]any{}
		if plan, ok := ctx["planMode"]; ok {
			newCtx["planMode"] = plan
		}
		if tools, ok := ctx["tools"]; ok {
			newCtx["tools"] = tools
		}
		if manifest, ok := ctx["toolContextManifest"]; ok {
			newCtx["toolContextManifest"] = manifest
		}
		if choice, ok := ctx["claudeToolChoice"]; ok {
			newCtx["claudeToolChoice"] = choice
		}
		if len(newCtx) > 0 {
			newMsg["userInputMessageContext"] = newCtx
		}
	}
	current["userInputMessage"] = newMsg
	delete(current, "toolUses")

	return json.Marshal(root)
}

func BuildMinimalKiroRequest(original []byte) ([]byte, error) {
	var root map[string]any
	if err := json.Unmarshal(original, &root); err != nil {
		return nil, err
	}
	cs, _ := root["conversationState"].(map[string]any)
	if cs == nil {
		return nil, fmt.Errorf("kiro minimal fallback: missing conversationState")
	}
	current, _ := cs["currentMessage"].(map[string]any)
	if current == nil {
		return nil, fmt.Errorf("kiro minimal fallback: missing currentMessage")
	}
	uim, _ := current["userInputMessage"].(map[string]any)
	if uim == nil {
		return nil, fmt.Errorf("kiro minimal fallback: missing userInputMessage")
	}

	summary := "Continue the previous task using the latest TodoWrite state."
	if historyAny, ok := cs["history"].([]any); ok {
		lines := make([]string, 0, len(historyAny))
		for _, raw := range historyAny {
			entry, _ := raw.(map[string]any)
			if entry == nil {
				continue
			}
			if user, ok := entry["userInputMessage"].(map[string]any); ok {
				lines = append(lines, summarizeUserEntry(user)...)
			}
		}
		if n := len(lines); n > 0 {
			summary = lines[n-1]
			if strings.HasPrefix(summary, "User: ") {
				summary = strings.TrimPrefix(summary, "User: ")
			}
		}
	}

	cs["history"] = []any{}
	minimal := map[string]any{
		"content": summary,
		"modelId": uim["modelId"],
		"origin":  uim["origin"],
	}
	current["userInputMessage"] = minimal
	return json.Marshal(root)
}

func summarizeUserEntry(user map[string]any) []string {
	lines := make([]string, 0, 2)
	if user == nil {
		return lines
	}
	if text := strings.TrimSpace(fmt.Sprint(user["content"])); text != "" && text != "." {
		lines = append(lines, "User: "+text)
	}
	if ctx, ok := user["userInputMessageContext"].(map[string]any); ok {
		lines = append(lines, summarizeToolResults(ctx)...)
	}
	return lines
}

func summarizeAssistantEntry(assistant map[string]any) []string {
	lines := make([]string, 0, 2)
	if assistant == nil {
		return lines
	}
	if text := strings.TrimSpace(fmt.Sprint(assistant["content"])); text != "" && text != "." {
		lines = append(lines, "Assistant: "+text)
	}
	if tus, ok := assistant["toolUses"].([]any); ok {
		for _, raw := range tus {
			use, _ := raw.(map[string]any)
			if use == nil {
				continue
			}
			name := strings.TrimSpace(fmt.Sprint(use["name"]))
			input := stringifyJSON(use["input"])
			lines = append(lines, fmt.Sprintf("Assistant tool call %s with input %s", name, input))
		}
	}
	return lines
}

func summarizeToolResults(ctx map[string]any) []string {
	results, ok := ctx["toolResults"].([]any)
	if !ok || len(results) == 0 {
		return nil
	}
	lines := make([]string, 0, len(results))
	for _, raw := range results {
		result, _ := raw.(map[string]any)
		if result == nil {
			continue
		}
		texts := extractToolResultTexts(result["content"])
		if len(texts) == 0 {
			continue
		}
		useID := strings.TrimSpace(fmt.Sprint(result["toolUseId"]))
		prefix := "Tool result"
		if useID != "" {
			prefix = fmt.Sprintf("Tool result %s", useID)
		}
		lines = append(lines, fmt.Sprintf("%s: %s", prefix, strings.Join(texts, "\n")))
	}
	return lines
}

func extractToolResultTexts(content any) []string {
	switch typed := content.(type) {
	case []any:
		lines := make([]string, 0, len(typed))
		for _, raw := range typed {
			if entry, ok := raw.(map[string]any); ok {
				if text, _ := entry["text"].(string); strings.TrimSpace(text) != "" {
					lines = append(lines, strings.TrimSpace(text))
				}
			}
		}
		return lines
	case []map[string]any:
		lines := make([]string, 0, len(typed))
		for _, entry := range typed {
			if text, _ := entry["text"].(string); strings.TrimSpace(text) != "" {
				lines = append(lines, strings.TrimSpace(text))
			}
		}
		return lines
	case string:
		if strings.TrimSpace(typed) != "" {
			return []string{strings.TrimSpace(typed)}
		}
	}
	return nil
}

func stringifyJSON(v any) string {
	if v == nil {
		return "null"
	}
	buf, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	const limit = 256
	if len(buf) > limit {
		return string(buf[:limit]) + "..."
	}
	return string(buf)
}

func shouldAttemptFlattenedFallback(err error) bool {
	statusErr, ok := err.(kiroStatusError)
	if !ok {
		return false
	}
	if statusErr.code != http.StatusBadRequest {
		return false
	}
	return strings.Contains(strings.ToLower(statusErr.msg), "improperly formed request")
}

func isImproperlyFormedResponsePayload(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	return bytes.Contains(bytes.ToLower(data), []byte("improperly formed request"))
}
