package executor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/usage"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
)

const (
	kiroBaseURLTemplate    = "https://codewhisperer.%s.amazonaws.com/generateAssistantResponse"
	kiroAmazonQURLTemplate = "https://codewhisperer.%s.amazonaws.com/SendMessageStreaming"
	kiroOrigin             = "AI_EDITOR"
	kiroChatTrigger        = "MANUAL"
	kiroDefaultRegion      = "us-east-1"
	kiroAgentPrefix        = "aws-sdk-js/1.0.7"
	kiroIDEVersion         = "KiroIDE-0.1.25"
)

// KiroModelMapping maps OpenAI-compatible model names to Kiro internal model identifiers.
var KiroModelMapping = map[string]string{
	"claude-sonnet-4-5":                  "CLAUDE_SONNET_4_5_20250929_V1_0",
	"claude-sonnet-4-5-20250929":         "CLAUDE_SONNET_4_5_20250929_V1_0",
	"claude-sonnet-4-20250514":           "CLAUDE_SONNET_4_20250514_V1_0",
	"claude-3-7-sonnet-20250219":         "CLAUDE_3_7_SONNET_20250219_V1_0",
	"amazonq-claude-sonnet-4-20250514":   "CLAUDE_SONNET_4_20250514_V1_0",
	"amazonq-claude-3-7-sonnet-20250219": "CLAUDE_3_7_SONNET_20250219_V1_0",
}

// KiroExecutor is a stateless executor for Kiro AI services.
type KiroExecutor struct {
	cfg     *config.Config
	auth    *kiro.KiroAuth
	macOnce sync.Once
	macHash string
}

// NewKiroExecutor creates a new Kiro executor instance.
func NewKiroExecutor(cfg *config.Config) *KiroExecutor {
	return &KiroExecutor{
		cfg:  cfg,
		auth: kiro.NewKiroAuth(),
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

	payload, err := buildOpenAIChatCompletionPayload(req.Model, result.Text, result.ToolCalls, result.PromptTokens, result.CompletionTokens)
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

		for _, payload := range buildStreamingChunks(id, req.Model, created, result.Text, result.ToolCalls) {
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

	cfg := e.cfg
	if cfg == nil {
		cfg = &config.Config{}
	}
	if _, err := e.auth.GetAuthenticatedClient(ctx, ts, cfg); err != nil {
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

// performCompletion sends the request to Kiro and normalizes the response.
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

	// Ensure token freshness.
	cfg := e.cfg
	if cfg == nil {
		cfg = &config.Config{}
	}
	if _, err := e.auth.GetAuthenticatedClient(ctx, ts, cfg); err != nil {
		return nil, fmt.Errorf("kiro executor: auth refresh failed: %w", err)
	}

	body, err := buildKiroRequestPayload(req.Model, req.Payload, ts, opts.Metadata)
	if err != nil {
		return nil, err
	}

	endpoint := e.buildEndpoint(req.Model, ts.ProfileArn)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	e.applyHeaders(httpReq, ts.AccessToken)

	if auth != nil {
		recordAPIRequest(ctx, e.cfg, upstreamRequestLog{
			URL:       endpoint,
			Method:    http.MethodPost,
			Headers:   httpReq.Header.Clone(),
			Body:      body,
			Provider:  e.Identifier(),
			AuthID:    auth.ID,
			AuthLabel: auth.Label,
		})
	}

	httpClient := newProxyAwareHTTPClient(ctx, e.cfg, auth, 120*time.Second)
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 120 * time.Second}
	}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		recordAPIResponseError(ctx, e.cfg, err)
		return nil, err
	}
	defer func() {
		if errClose := resp.Body.Close(); errClose != nil {
			log.Errorf("kiro executor: close body error: %v", errClose)
		}
	}()

	recordAPIResponseMetadata(ctx, e.cfg, resp.StatusCode, resp.Header.Clone())
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		recordAPIResponseError(ctx, e.cfg, err)
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		appendAPIResponseChunk(ctx, e.cfg, data)
		return nil, kiroStatusError{code: resp.StatusCode, msg: string(data)}
	}

	text, toolCalls := parseKiroResponse(data)
	if text == "" {
		text = strings.TrimSpace(string(data))
	}

	promptTokens, _ := estimatePromptTokens(req.Model, req.Payload)
	completionTokens := estimateCompletionTokens(text, toolCalls)

	return &kiroResult{
		Text:             text,
		ToolCalls:        toolCalls,
		KiroModel:        mapKiroModel(req.Model),
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
	}, nil
}

// buildEndpoint chooses the correct Kiro endpoint for the model.
func (e *KiroExecutor) buildEndpoint(model, profileArn string) string {
	region := e.extractRegionFromARN(profileArn)
	if region == "" {
		region = kiroDefaultRegion
	}
	if strings.HasPrefix(strings.ToLower(model), "amazonq-") {
		return fmt.Sprintf(kiroAmazonQURLTemplate, region)
	}
	return fmt.Sprintf(kiroBaseURLTemplate, region)
}

// tokenStorageFromAuth resolves the Kiro token storage from auth metadata or file.
func (e *KiroExecutor) tokenStorageFromAuth(auth *cliproxyauth.Auth) (*kiro.KiroTokenStorage, error) {
	if auth == nil {
		return nil, fmt.Errorf("kiro executor: auth is nil")
	}
	if ts, ok := auth.Runtime.(*kiro.KiroTokenStorage); ok && ts != nil {
		return ts, nil
	}
	if token := extractTokenFromMetadata(auth.Metadata); token != nil {
		auth.Runtime = token
		return token, nil
	}
	path := e.tokenFilePath(auth)
	if path == "" {
		return nil, fmt.Errorf("kiro executor: token path unavailable for %s", auth.ID)
	}
	ts, err := kiro.LoadTokenFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("kiro executor: load token: %w", err)
	}
	auth.Runtime = ts
	return ts, nil
}

func (e *KiroExecutor) tokenFilePath(auth *cliproxyauth.Auth) string {
	if auth == nil {
		return ""
	}
	if auth.Attributes != nil {
		if p := strings.TrimSpace(auth.Attributes["path"]); p != "" {
			return expandPath(p)
		}
	}
	candidates := []string{auth.FileName, auth.ID}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if filepath.IsAbs(candidate) {
			return candidate
		}
		if e.cfg != nil && e.cfg.AuthDir != "" {
			return filepath.Join(expandPath(e.cfg.AuthDir), candidate)
		}
		return candidate
	}
	return ""
}

// buildKiroRequestPayload converts an OpenAI-style request into Kiro's structure.
func buildKiroRequestPayload(model string, payload []byte, ts *kiro.KiroTokenStorage, metadata map[string]any) ([]byte, error) {
	if ts == nil {
		return nil, fmt.Errorf("kiro executor: token storage missing")
	}
	root := gjson.ParseBytes(payload)
	messages := root.Get("messages")
	if !messages.Exists() || !messages.IsArray() || len(messages.Array()) == 0 {
		return nil, fmt.Errorf("kiro executor: messages array is required")
	}

	systemPrompt := strings.TrimSpace(root.Get("system").String())
	tools := root.Get("tools")
	kiroModel := mapKiroModel(model)

	history := make([]map[string]any, 0, len(messages.Array()))
	startIndex := 0

	if systemPrompt != "" {
		first := messages.Array()[0]
		if strings.EqualFold(first.Get("role").String(), "user") {
			text, _, _, _ := extractUserMessage(first)
			content := combineContent(systemPrompt, text)
			history = append(history, wrapUserMessage(content, kiroModel, nil, nil, nil, nil))
			startIndex = 1
		} else {
			history = append(history, wrapUserMessage(systemPrompt, kiroModel, nil, nil, nil, nil))
		}
	}

	for i := startIndex; i < len(messages.Array())-1; i++ {
		msg := messages.Array()[i]
		role := strings.ToLower(strings.TrimSpace(msg.Get("role").String()))
		switch role {
		case "assistant":
			text, toolUses := extractAssistantMessage(msg)
			history = append(history, wrapAssistantMessage(text, toolUses))
		case "user", "system", "tool":
			text, toolResults, toolUses, images := extractUserMessage(msg)
			history = append(history, wrapUserMessage(text, kiroModel, toolResults, toolUses, images, nil))
		}
	}

	current := messages.Array()[len(messages.Array())-1]
	currentRole := strings.ToLower(strings.TrimSpace(current.Get("role").String()))
	var currentPayload map[string]any
	if currentRole == "assistant" {
		text, toolUses := extractAssistantMessage(current)
		currentPayload = map[string]any{
			"assistantResponseMessage": map[string]any{
				"content":  text,
				"toolUses": toolUses,
			},
		}
	} else {
		text, toolResults, toolUses, images := extractUserMessage(current)
		context := map[string]any{}
		if len(toolResults) > 0 {
			context["toolResults"] = toolResults
		}
		if toolDefinitions := buildToolSpecifications(tools); len(toolDefinitions) > 0 {
			context["tools"] = toolDefinitions
		}
		if len(context) == 0 {
			context = nil
		}

		currentPayload = map[string]any{
			"userInputMessage": map[string]any{
				"content": text,
				"modelId": kiroModel,
				"origin":  kiroOrigin,
			},
		}
		if len(images) > 0 {
			currentPayload["userInputMessage"].(map[string]any)["images"] = images
		}
		if context != nil {
			currentPayload["userInputMessage"].(map[string]any)["userInputMessageContext"] = context
		}
		if len(toolUses) > 0 {
			currentPayload["userInputMessage"].(map[string]any)["toolUses"] = toolUses
		}
	}

	request := map[string]any{
		"conversationState": map[string]any{
			"chatTriggerType": kiroChatTrigger,
			"conversationId":  uuid.NewString(),
			"currentMessage":  currentPayload,
			"history":         history,
		},
	}
	if strings.EqualFold(ts.AuthMethod, "social") && ts.ProfileArn != "" {
		request["profileArn"] = ts.ProfileArn
	}
	if project, ok := metadata["project"].(string); ok && project != "" {
		request["projectName"] = project
	}

	return json.Marshal(request)
}

func wrapUserMessage(content, model string, toolResults, toolUses, images []map[string]any, tools []map[string]any) map[string]any {
	payload := map[string]any{
		"userInputMessage": map[string]any{
			"content": content,
			"modelId": model,
			"origin":  kiroOrigin,
		},
	}
	context := map[string]any{}
	if len(toolResults) > 0 {
		context["toolResults"] = toolResults
	}
	if len(tools) > 0 {
		context["tools"] = tools
	}
	if len(context) > 0 {
		payload["userInputMessage"].(map[string]any)["userInputMessageContext"] = context
	}
	if len(images) > 0 {
		payload["userInputMessage"].(map[string]any)["images"] = images
	}
	if len(toolUses) > 0 {
		payload["userInputMessage"].(map[string]any)["toolUses"] = toolUses
	}
	return payload
}

func wrapAssistantMessage(content string, toolUses []map[string]any) map[string]any {
	payload := map[string]any{
		"assistantResponseMessage": map[string]any{
			"content": content,
		},
	}
	if len(toolUses) > 0 {
		payload["assistantResponseMessage"].(map[string]any)["toolUses"] = toolUses
	}
	return payload
}

func extractUserMessage(msg gjson.Result) (string, []map[string]any, []map[string]any, []map[string]any) {
	content := msg.Get("content")
	textParts := make([]string, 0, 4)
	toolResults := make([]map[string]any, 0)
	toolUses := make([]map[string]any, 0)
	images := make([]map[string]any, 0)

	if content.Type == gjson.String {
		textParts = append(textParts, content.String())
	} else if content.IsArray() {
		content.ForEach(func(_, part gjson.Result) bool {
			switch strings.ToLower(part.Get("type").String()) {
			case "text", "input_text", "output_text":
				textParts = append(textParts, part.Get("text").String())
			case "tool_result":
				resultContent := extractNestedContent(part.Get("content"))
				if resultContent == "" {
					resultContent = part.Get("text").String()
				}
				toolResults = append(toolResults, map[string]any{
					"content": []map[string]string{{"text": resultContent}},
					"status":  firstString(part.Get("status").String(), "success"),
					"toolUseId": firstString(
						part.Get("tool_use_id").String(),
						part.Get("tool_useId").String(),
					),
				})
			case "tool_use":
				toolUses = append(toolUses, map[string]any{
					"name":      part.Get("name").String(),
					"toolUseId": firstString(part.Get("id").String(), part.Get("tool_use_id").String()),
					"input":     parseJSONSafely(part.Get("input"), part.Get("arguments")),
				})
			case "image", "input_image":
				if img := buildImagePart(part); img != nil {
					images = append(images, img)
				}
			}
			return true
		})
	} else if content.Exists() {
		textParts = append(textParts, content.String())
	}
	return strings.TrimSpace(strings.Join(textParts, "\n")), toolResults, toolUses, images
}

func extractAssistantMessage(msg gjson.Result) (string, []map[string]any) {
	content := msg.Get("content")
	textParts := make([]string, 0, 4)
	toolUses := make([]map[string]any, 0)

	if content.Type == gjson.String {
		textParts = append(textParts, content.String())
	} else if content.IsArray() {
		content.ForEach(func(_, part gjson.Result) bool {
			switch strings.ToLower(part.Get("type").String()) {
			case "text", "output_text":
				textParts = append(textParts, part.Get("text").String())
			case "tool_use":
				toolUses = append(toolUses, map[string]any{
					"name":      part.Get("name").String(),
					"toolUseId": firstString(part.Get("id").String(), part.Get("tool_use_id").String()),
					"input":     parseJSONSafely(part.Get("input"), part.Get("arguments")),
				})
			}
			return true
		})
	} else if content.Exists() {
		textParts = append(textParts, content.String())
	}
	return strings.TrimSpace(strings.Join(textParts, "\n")), toolUses
}

func buildToolSpecifications(tools gjson.Result) []map[string]any {
	if !tools.Exists() || !tools.IsArray() {
		return nil
	}
	specs := make([]map[string]any, 0, len(tools.Array()))
	tools.ForEach(func(_, tool gjson.Result) bool {
		if strings.ToLower(tool.Get("type").String()) != "function" {
			return true
		}
		function := tool.Get("function")
		if !function.Exists() {
			return true
		}
		entry := map[string]any{
			"toolSpecification": map[string]any{
				"name":        function.Get("name").String(),
				"description": function.Get("description").String(),
				"inputSchema": map[string]any{"json": parseJSONSafely(function.Get("parameters"), gjson.Result{})},
			},
		}
		specs = append(specs, entry)
		return true
	})
	return specs
}

func buildImagePart(part gjson.Result) map[string]any {
	if source := part.Get("source"); source.Exists() {
		mediaType := source.Get("media_type").String()
		format := ""
		if idx := strings.Index(mediaType, "/"); idx != -1 && idx+1 < len(mediaType) {
			format = mediaType[idx+1:]
		}
		data := source.Get("data").String()
		if format == "" || data == "" {
			return nil
		}
		return map[string]any{
			"format": format,
			"source": map[string]any{"bytes": data},
		}
	}
	return nil
}

func extractNestedContent(value gjson.Result) string {
	if !value.Exists() {
		return ""
	}
	if value.Type == gjson.String {
		return value.String()
	}
	if value.IsArray() {
		parts := make([]string, 0, len(value.Array()))
		value.ForEach(func(_, part gjson.Result) bool {
			if part.Type == gjson.String {
				parts = append(parts, part.String())
			} else if part.Get("text").Exists() {
				parts = append(parts, part.Get("text").String())
			}
			return true
		})
		return strings.Join(parts, "")
	}
	return value.String()
}

func combineContent(parts ...string) string {
	acc := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			acc = append(acc, trimmed)
		}
	}
	return strings.Join(acc, "\n\n")
}

func parseJSONSafely(primary, fallback gjson.Result) any {
	if primary.Exists() && primary.Raw != "" {
		var obj any
		if err := json.Unmarshal([]byte(primary.Raw), &obj); err == nil {
			return obj
		}
	}
	if fallback.Exists() && fallback.Raw != "" {
		var obj any
		if err := json.Unmarshal([]byte(fallback.Raw), &obj); err == nil {
			return obj
		}
	}
	return nil
}

func mapKiroModel(model string) string {
	if mapped, ok := KiroModelMapping[strings.TrimSpace(model)]; ok {
		return mapped
	}
	return KiroModelMapping["claude-sonnet-4-5"]
}

func (e *KiroExecutor) applyHeaders(req *http.Request, token string) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	agentSuffix := e.macHashValue()
	req.Header.Set("x-amz-user-agent", fmt.Sprintf("%s %s-%s", kiroAgentPrefix, kiroIDEVersion, agentSuffix))
	req.Header.Set("user-agent", fmt.Sprintf("%s ua/2.1 os/cli lang/go api/codewhispererstreaming#1.0.7 m/E %s-%s", kiroAgentPrefix, kiroIDEVersion, agentSuffix))
	req.Header.Set("amz-sdk-request", "attempt=1; max=1")
	req.Header.Set("x-amzn-kiro-agent-mode", "vibe")
}

func (e *KiroExecutor) macHashValue() string {
	e.macOnce.Do(func() {
		interfaces, err := net.Interfaces()
		if err != nil {
			e.macHash = "0000000000000000"
			return
		}
		for _, iface := range interfaces {
			if iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			addr := iface.HardwareAddr.String()
			if addr == "" {
				continue
			}
			sum := sha256.Sum256([]byte(addr))
			e.macHash = hex.EncodeToString(sum[:])
			return
		}
		e.macHash = "0000000000000000"
	})
	return e.macHash
}

func (e *KiroExecutor) extractRegionFromARN(arn string) string {
	parts := strings.Split(arn, ":")
	if len(parts) > 3 {
		region := parts[3]
		if strings.HasPrefix(region, "us") || strings.HasPrefix(region, "eu") || strings.HasPrefix(region, "ap") {
			return region
		}
	}
	return ""
}

// parseKiroResponse attempts to extract assistant text and tool calls from the upstream payload.
func parseKiroResponse(data []byte) (string, []openAIToolCall) {
	if len(data) == 0 {
		return "", nil
	}
	if gjson.ValidBytes(data) {
		root := gjson.ParseBytes(data)
		if content := root.Get("conversationState.currentMessage.assistantResponseMessage.content"); content.Exists() {
			return content.String(), nil
		}
		if history := root.Get("conversationState.history"); history.Exists() && history.IsArray() {
			for i := len(history.Array()) - 1; i >= 0; i-- {
				item := history.Array()[i]
				if content := item.Get("assistantResponseMessage.content"); content.Exists() {
					return content.String(), nil
				}
			}
		}
	}
	return parseEventStream(string(data))
}

func parseEventStream(raw string) (string, []openAIToolCall) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	result := strings.Builder{}
	toolCalls := make([]openAIToolCall, 0)
	currentCall := (*openAIToolCall)(nil)

	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if idx := strings.Index(line, "{"); idx >= 0 {
			line = line[idx:]
		}
		event := firstValidJSON(line)
		if len(event) == 0 {
			continue
		}
		node := gjson.ParseBytes(event)
		if name := node.Get("name").String(); name != "" && node.Get("toolUseId").Exists() {
			if currentCall == nil {
				currentCall = &openAIToolCall{
					ID:   node.Get("toolUseId").String(),
					Name: name,
				}
			}
			if input := node.Get("input"); input.Exists() {
				currentCall.Arguments += input.Raw
			}
			if node.Get("stop").Bool() && currentCall != nil {
				if args := normalizeArguments(currentCall.Arguments); args != "" {
					currentCall.Arguments = args
				}
				toolCalls = append(toolCalls, *currentCall)
				currentCall = nil
			}
			continue
		}
		if content := node.Get("content").String(); content != "" && !node.Get("followupPrompt").Bool() {
			decoded := strings.ReplaceAll(content, `\n`, "\n")
			result.WriteString(decoded)
		}
	}
	if currentCall != nil {
		if args := normalizeArguments(currentCall.Arguments); args != "" {
			currentCall.Arguments = args
		}
		toolCalls = append(toolCalls, *currentCall)
	}

	bracketCalls := parseBracketToolCalls(raw)
	if len(bracketCalls) > 0 {
		toolCalls = append(toolCalls, bracketCalls...)
	}

	content := strings.TrimSpace(result.String())
	if content == "" {
		content = strings.TrimSpace(raw)
	}
	return content, deduplicateToolCalls(toolCalls)
}

func parseBracketToolCalls(raw string) []openAIToolCall {
	pattern := regexp.MustCompile(`(?s)\[Called\s+([A-Za-z0-9_]+)\s+with\s+args:\s*(\{.*?\})\]`)
	matches := pattern.FindAllStringSubmatch(raw, -1)
	calls := make([]openAIToolCall, 0, len(matches))
	for _, match := range matches {
		name := match[1]
		argBlock := sanitizeJSON(match[2])
		if name == "" || argBlock == "" {
			continue
		}
		calls = append(calls, openAIToolCall{
			ID:        fmt.Sprintf("call_%s", uuid.New().String()),
			Name:      name,
			Arguments: argBlock,
		})
	}
	return calls
}

func deduplicateToolCalls(calls []openAIToolCall) []openAIToolCall {
	seen := make(map[string]struct{}, len(calls))
	deduped := make([]openAIToolCall, 0, len(calls))
	for _, call := range calls {
		key := call.Name + ":" + call.Arguments
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, call)
	}
	return deduped
}

func sanitizeJSON(input string) string {
	if input == "" {
		return ""
	}
	value := regexp.MustCompile(`,\s*([}\]])`).ReplaceAllString(input, "$1")
	value = regexp.MustCompile(`([{,]\s*)([A-Za-z0-9_]+)\s*:`).ReplaceAllString(value, `$1"$2":`)
	if json.Valid([]byte(value)) {
		return value
	}
	return ""
}

func firstValidJSON(block string) []byte {
	block = strings.TrimSpace(block)
	for i := len(block); i > 0; i-- {
		snippet := strings.TrimSpace(block[:i])
		if len(snippet) == 0 {
			continue
		}
		if json.Valid([]byte(snippet)) {
			return []byte(snippet)
		}
	}
	return nil
}

func normalizeArguments(args string) string {
	args = strings.TrimSpace(args)
	if args == "" {
		return ""
	}
	if json.Valid([]byte(args)) {
		return args
	}
	if fixed := sanitizeJSON(args); fixed != "" {
		return fixed
	}
	return ""
}

func estimatePromptTokens(model string, payload []byte) (int64, error) {
	enc, err := tokenizerForModel(model)
	if err != nil {
		return 0, err
	}
	return countOpenAIChatTokens(enc, payload)
}

func estimateCompletionTokens(text string, toolCalls []openAIToolCall) int64 {
	length := utf8.RuneCountInString(text)
	for _, call := range toolCalls {
		length += utf8.RuneCountInString(call.Arguments)
	}
	tokens := math.Ceil(float64(length) / 4)
	if tokens < 1 {
		return 1
	}
	return int64(tokens)
}

func buildOpenAIChatCompletionPayload(model, content string, toolCalls []openAIToolCall, promptTokens, completionTokens int64) ([]byte, error) {
	message := map[string]any{
		"role":    "assistant",
		"content": content,
	}
	if len(toolCalls) > 0 {
		tc := make([]map[string]any, 0, len(toolCalls))
		for _, call := range toolCalls {
			tc = append(tc, map[string]any{
				"id":   call.ID,
				"type": "function",
				"function": map[string]any{
					"name":      call.Name,
					"arguments": call.Arguments,
				},
			})
		}
		message["tool_calls"] = tc
	}

	payload := map[string]any{
		"id":      fmt.Sprintf("chatcmpl_%s", uuid.NewString()),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []map[string]any{{
			"index":         0,
			"message":       message,
			"finish_reason": "stop",
		}},
		"usage": map[string]any{
			"prompt_tokens":     promptTokens,
			"completion_tokens": completionTokens,
			"total_tokens":      promptTokens + completionTokens,
		},
	}
	return json.Marshal(payload)
}

func buildStreamingChunks(id, model string, created int64, content string, toolCalls []openAIToolCall) [][]byte {
	chunks := make([][]byte, 0, 3)
	initial := map[string]any{
		"id":      id,
		"object":  "chat.completion.chunk",
		"created": created,
		"model":   model,
		"choices": []map[string]any{{
			"index": 0,
			"delta": map[string]any{"role": "assistant"},
		}},
	}
	chunks = append(chunks, marshalStreamChunk(initial))

	if strings.TrimSpace(content) != "" {
		data := map[string]any{
			"id":      id,
			"object":  "chat.completion.chunk",
			"created": created,
			"model":   model,
			"choices": []map[string]any{{
				"index": 0,
				"delta": map[string]any{"content": content},
			}},
		}
		chunks = append(chunks, marshalStreamChunk(data))
	}

	if len(toolCalls) > 0 {
		tc := make([]map[string]any, 0, len(toolCalls))
		for _, call := range toolCalls {
			tc = append(tc, map[string]any{
				"id":   call.ID,
				"type": "function",
				"function": map[string]any{
					"name":      call.Name,
					"arguments": call.Arguments,
				},
			})
		}
		data := map[string]any{
			"id":      id,
			"object":  "chat.completion.chunk",
			"created": created,
			"model":   model,
			"choices": []map[string]any{{
				"index": 0,
				"delta": map[string]any{"tool_calls": tc},
			}},
		}
		chunks = append(chunks, marshalStreamChunk(data))
	}

	final := map[string]any{
		"id":      id,
		"object":  "chat.completion.chunk",
		"created": created,
		"model":   model,
		"choices": []map[string]any{{
			"index":         0,
			"delta":         map[string]any{},
			"finish_reason": "stop",
		}},
	}
	chunks = append(chunks, marshalStreamChunk(final))
	return chunks
}

func marshalStreamChunk(payload map[string]any) []byte {
	data, _ := json.Marshal(payload)
	return data
}

func usageDetail(prompt, completion int64) usage.Detail {
	return usage.Detail{
		InputTokens:  prompt,
		OutputTokens: completion,
		TotalTokens:  prompt + completion,
	}
}

func expandPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || path[0] != '~' {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == "~" {
		return home
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~/"))
}

func firstString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func extractTokenFromMetadata(meta map[string]any) *kiro.KiroTokenStorage {
	if len(meta) == 0 {
		return nil
	}
	ts := &kiro.KiroTokenStorage{
		AccessToken:  kiroStringValue(meta["accessToken"], meta["access_token"]),
		RefreshToken: kiroStringValue(meta["refreshToken"], meta["refresh_token"]),
		ProfileArn:   kiroStringValue(meta["profileArn"], meta["profile_arn"]),
		AuthMethod:   kiroStringValue(meta["authMethod"], meta["auth_method"]),
		Provider:     kiroStringValue(meta["provider"]),
		Type:         "kiro",
	}
	if expires, ok := parseExpiry(meta["expiresAt"], meta["expires_at"]); ok {
		ts.ExpiresAt = expires
	}
	if ts.AccessToken == "" && ts.RefreshToken == "" {
		return nil
	}
	return ts
}

func attachTokenMetadata(meta map[string]any, ts *kiro.KiroTokenStorage) map[string]any {
	if ts == nil {
		return meta
	}
	if meta == nil {
		meta = make(map[string]any)
	}
	meta["accessToken"] = ts.AccessToken
	meta["refreshToken"] = ts.RefreshToken
	meta["profileArn"] = ts.ProfileArn
	meta["authMethod"] = ts.AuthMethod
	meta["provider"] = ts.Provider
	if !ts.ExpiresAt.IsZero() {
		meta["expiresAt"] = ts.ExpiresAt.Format(time.RFC3339)
	}
	meta["type"] = "kiro"
	return meta
}

func kiroStringValue(values ...any) string {
	for _, value := range values {
		if str, ok := value.(string); ok && strings.TrimSpace(str) != "" {
			return str
		}
	}
	return ""
}

func parseExpiry(values ...any) (time.Time, bool) {
	for _, value := range values {
		switch v := value.(type) {
		case string:
			if strings.TrimSpace(v) == "" {
				continue
			}
			if ts, err := time.Parse(time.RFC3339, v); err == nil {
				return ts, true
			}
			if unix, err := strconv.ParseInt(v, 10, 64); err == nil {
				return time.Unix(unix, 0), true
			}
		case float64:
			return time.Unix(int64(v), 0), true
		case int64:
			return time.Unix(v, 0), true
		case json.Number:
			if val, err := v.Int64(); err == nil {
				return time.Unix(val, 0), true
			}
		}
	}
	return time.Time{}, false
}

type kiroResult struct {
	Text             string
	ToolCalls        []openAIToolCall
	KiroModel        string
	PromptTokens     int64
	CompletionTokens int64
}

type openAIToolCall struct {
	ID        string
	Name      string
	Arguments string
}

type kiroStatusError struct {
	code int
	msg  string
}

func (e kiroStatusError) Error() string {
	if strings.TrimSpace(e.msg) != "" {
		return e.msg
	}
	return fmt.Sprintf("status %d", e.code)
}

func (e kiroStatusError) StatusCode() int { return e.code }
