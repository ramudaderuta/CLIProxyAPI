# Kiro ⇄ Anthropic Translation & Execution Protocol Contract  

## 0. Scope & Non-Goals

- **Scope**
  - Define the contract and operational logic for:
    - Translating **Anthropic Messages API** requests → **Kiro** `conversationState` requests.
    - Translating **Kiro** responses → **Anthropic Messages API** responses.
    - Executing these translations in a **non-streaming Kiro call + synthetic Anthropic SSE** model.
  - Include **defensive behavior** (sanitization, JSON parsing, tool clamping, tool manifest, error handling).

---

## 1. Core Data Models

### 1.1 Anthropic (Claude) Request/Response Types

```go
// AnthropicMessagesRequest represents /v1/messages input.
type AnthropicMessagesRequest struct {
	Model       string               `json:"model"`
	Messages    []AnthropicMessage   `json:"messages"`
	System      []AnthropicSystemMsg `json:"system,omitempty"` // optional, can be []TextBlock
	Tools       []AnthropicTool      `json:"tools,omitempty"`
	ToolChoice  *AnthropicToolChoice `json:"tool_choice,omitempty"`
	Thinking    *AnthropicThinking   `json:"thinking,omitempty"`
	Metadata    map[string]any       `json:"metadata,omitempty"`
	MaxTokens   *int                 `json:"max_tokens,omitempty"`
	Temperature *float64             `json:"temperature,omitempty"`
	TopP        *float64             `json:"top_p,omitempty"`
	Stream      bool                 `json:"stream,omitempty"`
	// PlanMode, etc., may come via metadata or system text.
}

type AnthropicMessage struct {
	Role    string               `json:"role"` // "user" | "assistant"
	Content []AnthropicBlock     `json:"content"`
}

// Content blocks used in messages.
type AnthropicBlock struct {
	Type       string                 `json:"type"` // "text" | "image" | "tool_use" | "tool_result" | ...
	Text       string                 `json:"text,omitempty"`
	Source     *AnthropicImageSource  `json:"source,omitempty"`      // for type == "image"
	ID         string                 `json:"id,omitempty"`          // for type == "tool_use"
	Name       string                 `json:"name,omitempty"`        // for type == "tool_use"
	Input      map[string]any         `json:"input,omitempty"`       // for type == "tool_use"
	ToolUseID  string                 `json:"tool_use_id,omitempty"` // for type == "tool_result"`
	ContentAny any                    `json:"content,omitempty"`     // tool_result payload; raw
}

type AnthropicImageSource struct {
	Type      string `json:"type"`       // "base64"
	MediaType string `json:"media_type"` // e.g. "image/jpeg"
	Data      string `json:"data"`       // base64 payload
}

type AnthropicTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]any         `json:"input_schema,omitempty"`
	// Other Anthropic tool fields omitted here.
}

type AnthropicToolChoice struct {
	Type string `json:"type"` // "auto" | "any" | "tool" | ...
	// When type == "tool", tool name etc. may be present.
}

type AnthropicThinking struct {
	Type          string `json:"type"`            // "enabled" | "disabled"
	BudgetTokens  *int   `json:"budget_tokens"`   // optional
	// Additional thinking fields omitted.
}

type AnthropicMessagesResponse struct {
	ID         string             `json:"id"`
	Model      string             `json:"model"`
	Role       string             `json:"role"` // "assistant"
	Content    []AnthropicBlock   `json:"content"`
	StopReason string             `json:"stop_reason,omitempty"`
	Usage      *AnthropicUsage    `json:"usage,omitempty"`
	// Other Anthropic fields omitted for brevity.
}

type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
	TotalTokens  int `json:"total_tokens,omitempty"`
}
````

---

### 1.2 Kiro Request/Response Types

```go
// KiroRequest is the internal payload for Kiro's generateAssistantResponse.
type KiroRequest struct {
	ConversationState KiroConversationState `json:"conversationState"`
}

type KiroConversationState struct {
	CurrentMessage KiroCurrentMessage   `json:"currentMessage"`
	History        []KiroHistoryMessage `json:"history,omitempty"`
}

type KiroCurrentMessage struct {
	UserInputMessage         KiroUserInputMessage `json:"userInputMessage"`
	UserInputMessageContext  KiroMessageContext   `json:"userInputMessageContext"`
}

type KiroUserInputMessage struct {
	Content string `json:"content"`          // Plain text; no tool_use/tool_result
	ModelID string `json:"modelId"`          // Mapped from Anthropic model.
	Origin  string `json:"origin"`           // e.g. "AI_EDITOR"
	// Additional fields as required by Kiro.
}

type KiroHistoryMessage struct {
	Role    string `json:"role"`    // "user" | "assistant"
	Content string `json:"content"` // Plain text only.
}

type KiroMessageContext struct {
	Tools               []KiroToolSpec        `json:"tools,omitempty"`
	ToolContextManifest []KiroToolManifest    `json:"toolContextManifest,omitempty"`
	ClaudeToolChoice    *KiroToolChoiceMeta   `json:"claudeToolChoice,omitempty"`
	// Additional context: planMode, extra metadata, etc.
}

type KiroToolSpec struct {
	Name             string `json:"name"`
	Description      string `json:"description"` // truncated to <= 256 chars for Kiro
	JSONSchema       any    `json:"jsonSchema"`  // sanitized tool schema
	// Other Kiro tool fields as needed.
}

type KiroToolManifest struct {
	Name        string `json:"name"`
	Hash        string `json:"hash"`   // first 64 bits of SHA-256 of *full* description
	LengthChars int    `json:"length"` // length of full description
	Description string `json:"description"`
}

type KiroToolChoiceMeta struct {
	// Mirror of Anthropic tool_choice semantics, e.g.
	Mode       string `json:"mode"`        // "auto", "required", etc.
	ToolName   string `json:"toolName"`    // for "tool" mode, required tool name
	RawPayload any    `json:"rawPayload"`  // original Anthropic tool_choice
}

// KiroResponse is what Kiro actually returns.
type KiroResponse struct {
	// Exact shape depends on Kiro; conceptually:
	OutputMessages []KiroAssistantMessage `json:"outputMessages"`
	Usage          *KiroUsage             `json:"usage,omitempty"`
	StopReason     string                 `json:"stopReason,omitempty"`
}

type KiroAssistantMessage struct {
	Content string `json:"content"` // Kiro returns plain text; tool-like data must be encoded in text.
	// Potentially other metadata (followupPrompt, etc.).
}

type KiroUsage struct {
	InputTokens  int `json:"inputTokens,omitempty"`
	OutputTokens int `json:"outputTokens,omitempty"`
	TotalTokens  int `json:"totalTokens,omitempty"`
}
```

---

### 1.3 Configuration & Environment

```go
type KiroTranslatorConfig struct {
	DropToolEvents bool // default: true; do NOT send tool_use/tool_result to Kiro
	ClampToolDesc  int  // default: 256 chars
	// Other flags: PlanMode behavior, debug logging, etc.
}

type KiroExecutorConfig struct {
	HTTPClient  *http.Client
	EndpointURL string
	APIKey      string
	Timeout     time.Duration
}
```

---

## 2. Defensive Utilities

### 2.1 `safeParseJSON(raw string) any`

> Defensive parsing for tool arguments / tool results that may contain truncated escape sequences or malformed data.

```go
// safeParseJSON attempts to parse a JSON string used for tool inputs/outputs.
// - Returns a parsed value (`map[string]any` / `[]any` / primitive) on success.
// - On failure, returns the original string for best-effort preservation.
func safeParseJSON(raw string) any {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return raw
	}

	cleaned := trimmed

	// Handle dangling backslash at the end of the string (e.g. `"foo\"`).
	if strings.HasSuffix(cleaned, `\`) && !strings.HasSuffix(cleaned, `\\`) {
		cleaned = cleaned[:len(cleaned)-1]
	} else if strings.HasSuffix(cleaned, `\u`) ||
		strings.HasSuffix(cleaned, `\u0`) ||
		strings.HasSuffix(cleaned, `\u00`) {
		// Handle incomplete unicode escape sequences at end of string.
		idx := strings.LastIndex(cleaned, `\u`)
		if idx >= 0 {
			cleaned = cleaned[:idx]
		}
	}

	var v any
	if err := json.Unmarshal([]byte(cleaned), &v); err != nil {
		// Degrade gracefully: return the original string if parsing fails.
		return raw
	}
	return v
}
```

---

### 2.2 `cleanJSONSchemaProperties` (tool schema sanitization)

```go
// cleanJSONSchemaProperties removes unsupported keys and recursively cleans nested schemas.
// Kiro and Anthropic both expect a subset of JSON Schema.
func cleanJSONSchemaProperties(schema any) any {
	obj, ok := schema.(map[string]any)
	if !ok {
		return schema
	}

	out := make(map[string]any)
	for key, value := range obj {
		switch key {
		case "type", "description", "properties", "required", "enum", "items":
			out[key] = value
		// Drop any other keys silently.
		}
	}

	if props, ok := out["properties"].(map[string]any); ok {
		for propName, propSchema := range props {
			props[propName] = cleanJSONSchemaProperties(propSchema)
		}
	}

	if items, ok := out["items"]; ok {
		out["items"] = cleanJSONSchemaProperties(items)
	}

	return out
}
```

---

### 2.3 Text Sanitizer (control bytes / ANSI / protocol noise)

```go
// sanitizeText strips ANSI escape sequences, non-printable control bytes,
// and known protocol noise (e.g., "<system-reminder>" blocks).
func sanitizeText(s string) string {
	// 1) Strip ANSI codes (pseudo-regex).
	s = stripANSISequences(s)

	// 2) Remove non-printable/control chars except \n, \r, \t.
	s = removeControlCharacters(s)

	// 3) Drop legacy protocol noise like "<system-reminder>" blocks.
	s = stripSystemReminderBlocks(s)

	return s
}
```

(Implementations of `stripANSISequences`, `removeControlCharacters`, and `stripSystemReminderBlocks` are left as straightforward regex/string passes.)

---

### 2.4 Reasoning Effort from Budget Tokens

```go
func determineReasoningEffortFromBudget(budgetTokens *int) string {
	if budgetTokens == nil {
		return "high"
	}
	const lowThreshold = 50
	const highThreshold = 200

	switch {
	case *budgetTokens <= lowThreshold:
		return "low"
	case *budgetTokens <= highThreshold:
		return "medium"
	default:
		return "high"
	}
}
```

---

## 3. Kiro Translation Layer – Requests

### 3.1 High-Level Entry Point

```go
// TranslateKiroRequest converts an Anthropic Messages request into a KiroRequest,
// enforcing Kiro's contract (no tool_use/tool_result in conversation state).
func TranslateKiroRequest(
	ctx context.Context,
	cfg KiroTranslatorConfig,
	in AnthropicMessagesRequest,
) (KiroRequest, error) {
	// 1) Normalize & validate model, temperature, etc.
	modelID := mapAnthropicModelToKiro(in.Model)
	if modelID == "" {
		return KiroRequest{}, fmt.Errorf("unsupported model: %s", in.Model)
	}

	// 2) Extract system instructions and augment them with tool reference manifests.
	systemPrompt := buildSystemPromptWithToolReferences(cfg, in.System, in.Tools)

	// 3) Build KiroMessageContext: tools, tool manifest, claudeToolChoice metadata.
	msgCtx := buildKiroMessageContext(cfg, in.Tools, in.ToolChoice)

	// 4) Convert Anthropic messages → Kiro history + current message content.
	history, currentContent := buildKiroConversationFromAnthropicMessages(cfg, in.Messages, systemPrompt)

	// 5) Construct Kiro request.
	req := KiroRequest{
		ConversationState: KiroConversationState{
			CurrentMessage: KiroCurrentMessage{
				UserInputMessage: KiroUserInputMessage{
					Content: currentContent,
					ModelID: modelID,
					Origin:  "AI_EDITOR",
				},
				UserInputMessageContext: msgCtx,
			},
			History: history,
		},
	}

	return req, nil
}
```

---

### 3.2 System Prompt & Tool Reference Manifest

```go
// buildSystemPromptWithToolReferences:
// - Concatenates system text blocks into a single prompt string.
// - Appends "Tool reference manifest" section whenever any tool description is truncated.
func buildSystemPromptWithToolReferences(
	cfg KiroTranslatorConfig,
	systemMsgs []AnthropicSystemMsg,
	tools []AnthropicTool,
) string {
	base := joinSystemText(systemMsgs) // join system text blocks with "\n\n".

	var manifestLines []string

	for _, t := range tools {
		fullDesc := strings.TrimSpace(t.Description)
		if fullDesc == "" {
			continue
		}

		if len([]rune(fullDesc)) > cfg.ClampToolDesc {
			// Build manifest entry (full description preserved).
			hash := first64BitsOfSHA256(fullDesc)
			entry := fmt.Sprintf(
				"- %s [%s, %d chars]: %s",
				t.Name,
				hash,
				len([]rune(fullDesc)),
				fullDesc,
			)
			manifestLines = append(manifestLines, entry)
		}
	}

	if len(manifestLines) == 0 {
		return base
	}

	preamble := "Tool reference manifest (hash → tool)\n"
	return strings.TrimSpace(base + "\n\n" + preamble + strings.Join(manifestLines, "\n"))
}
```

---

### 3.3 Building `KiroMessageContext` (tools & tool_choice)

```go
// buildKiroMessageContext:
// - Clamps tool descriptions for Kiro.
// - Builds toolContextManifest with full descriptions & hashes.
// - Mirrors Anthropic tool_choice into claudeToolChoice metadata.
func buildKiroMessageContext(
	cfg KiroTranslatorConfig,
	tools []AnthropicTool,
	toolChoice *AnthropicToolChoice,
) KiroMessageContext {
	var specs []KiroToolSpec
	var manifest []KiroToolManifest

	for _, t := range tools {
		fullDesc := strings.TrimSpace(t.Description)
		clampedDesc := clampRunes(fullDesc, cfg.ClampToolDesc)

		specs = append(specs, KiroToolSpec{
			Name:        t.Name,
			Description: clampedDesc,
			JSONSchema:  cleanJSONSchemaProperties(t.InputSchema),
		})

		if fullDesc != "" {
			manifest = append(manifest, KiroToolManifest{
				Name:        t.Name,
				Hash:        first64BitsOfSHA256(fullDesc),
				LengthChars: len([]rune(fullDesc)),
				Description: fullDesc,
			})
		}
	}

	var toolChoiceMeta *KiroToolChoiceMeta
	if toolChoice != nil {
		toolChoiceMeta = &KiroToolChoiceMeta{
			Mode:       toolChoice.Type,
			ToolName:   inferToolNameFromChoice(toolChoice),
			RawPayload: toolChoice,
		}
	}

	return KiroMessageContext{
		Tools:               specs,
		ToolContextManifest: manifest,
		ClaudeToolChoice:    toolChoiceMeta,
	}
}
```

---

### 3.4 Conversation Translation – Drop Tool Events for Kiro

> **Key Kiro Contract:**
> Kiro **rejects** any client-supplied assistant `tool_use` or user `tool_result` events.
> The translator must:
>
> * **Not include** tool events in `conversationState.history` or current message content.
> * Optionally **fold tool results into plain text** (“Tool result context”) to preserve information.

```go
// buildKiroConversationFromAnthropicMessages:
// - Collects plain-text history for Kiro.
// - Builds current user message content, including system prompt and optional textualized tool context.
// - When cfg.DropToolEvents == true, tool_use/tool_result are only represented as text summaries.
func buildKiroConversationFromAnthropicMessages(
	cfg KiroTranslatorConfig,
	messages []AnthropicMessage,
	systemPrompt string,
) (history []KiroHistoryMessage, currentContent string) {
	// We'll build a plain-text transcript while respecting Kiro's "no tool events" constraint.
	var toolContextLines []string
	var textHistory []KiroHistoryMessage

	for _, m := range messages {
		switch m.Role {
		case "user":
			// User messages may contain text and/or tool_result blocks.
			userText := extractTextBlocks(m.Content)

			if cfg.DropToolEvents {
				toolResultSummary := summarizeToolResultsAsText(m.Content)
				if toolResultSummary != "" {
					toolContextLines = append(toolContextLines, toolResultSummary)
				}
			}

			if userText != "" {
				textHistory = append(textHistory, KiroHistoryMessage{
					Role:    "user",
					Content: sanitizeText(userText),
				})
			}

		case "assistant":
			// Assistant messages may contain text and/or tool_use blocks.
			if cfg.DropToolEvents {
				toolUseSummary := summarizeToolUsesAsText(m.Content)
				if toolUseSummary != "" {
					toolContextLines = append(toolContextLines, toolUseSummary)
				}
			}

			assistantText := extractTextBlocks(m.Content)
			if assistantText != "" {
				textHistory = append(textHistory, KiroHistoryMessage{
					Role:    "assistant",
					Content: sanitizeText(assistantText),
				})
			}

		default:
			// Ignore unknown roles.
		}
	}

	// The last user message becomes the "current" message; previous ones are history.
	// For simplicity, treat all but final user text as history. A more precise version
	// can split textHistory by role.
	history, lastUser := splitHistoryAndCurrent(textHistory)

	var builder strings.Builder
	if systemPrompt != "" {
		builder.WriteString(sanitizeText(systemPrompt))
		builder.WriteString("\n\n")
	}

	if len(toolContextLines) > 0 {
		builder.WriteString("Tool result context:\n")
		builder.WriteString(strings.Join(toolContextLines, "\n"))
		builder.WriteString("\n\n")
	}

	builder.WriteString(lastUser.Content)

	return history, builder.String()
}
```

Helper functions (pseudocode):

```go
func extractTextBlocks(blocks []AnthropicBlock) string {
	var parts []string
	for _, b := range blocks {
		if b.Type == "text" && b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func summarizeToolResultsAsText(blocks []AnthropicBlock) string {
	var lines []string
	for _, b := range blocks {
		if b.Type != "tool_result" {
			continue
		}
		// Summarize; could be full or truncated.
		raw := b.ContentAny
		line := fmt.Sprintf("Tool result (%s): %v", b.ToolUseID, raw)
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func summarizeToolUsesAsText(blocks []AnthropicBlock) string {
	var lines []string
	for _, b := range blocks {
		if b.Type != "tool_use" {
			continue
		}
		line := fmt.Sprintf("Tool invoked (%s): %s", b.ID, b.Name)
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// splitHistoryAndCurrent chooses the last user message as current; everything
// before it is history. Implementation detail can be more sophisticated.
func splitHistoryAndCurrent(
	all []KiroHistoryMessage,
) (history []KiroHistoryMessage, current KiroHistoryMessage) {
	if len(all) == 0 {
		return nil, KiroHistoryMessage{Role: "user", Content: ""}
	}
	// naive: last message is current
	history = all[:len(all)-1]
	current = all[len(all)-1]
	return
}
```

---

## 4. Kiro Translation Layer – Responses

### 4.1 High-Level Entry Point

```go
// TranslateKiroResponse converts KiroResponse into an AnthropicMessagesResponse.
func TranslateKiroResponse(
	ctx context.Context,
	req AnthropicMessagesRequest, // original request (for model, tokens, etc.)
	kiroResp KiroResponse,
) (AnthropicMessagesResponse, error) {
	var contentBlocks []AnthropicBlock

	for _, msg := range kiroResp.OutputMessages {
		if strings.TrimSpace(msg.Content) == "" {
			continue
		}
		contentBlocks = append(contentBlocks, AnthropicBlock{
			Type: "text",
			Text: sanitizeText(msg.Content),
		})
	}

	resp := AnthropicMessagesResponse{
		ID:      generateAnthropicID(),
		Model:   req.Model,
		Role:    "assistant",
		Content: contentBlocks,
	}

	resp.StopReason = mapKiroStopReasonToAnthropic(kiroResp.StopReason)
	resp.Usage = mapKiroUsageToAnthropic(kiroResp.Usage)

	// Add natural-language lead-in for tool_use if needed (per Anthropic compliance).
	resp.Content = ensureLeadInBeforeToolUse(resp.Content)

	return resp, nil
}
```

---

### 4.2 Usage & Stop Reason Mapping

```go
func mapKiroUsageToAnthropic(u *KiroUsage) *AnthropicUsage {
	if u == nil {
		return nil
	}
	return &AnthropicUsage{
		InputTokens:  u.InputTokens,
		OutputTokens: u.OutputTokens,
		TotalTokens:  u.TotalTokens,
	}
}

func mapKiroStopReasonToAnthropic(kiroReason string) string {
	switch kiroReason {
	case "MAX_TOKENS", "MAX_TOKENS_REACHED":
		return "max_tokens"
	case "STOP_SEQUENCE", "STOPPED":
		return "end_turn"
	case "TIMEOUT":
		return "max_tokens" // or a more precise mapping if Anthropic supports it
	default:
		return "end_turn"
	}
}
```

Lead-in enforcement (natural language before tool_use; even if Kiro currently doesn’t emit tool_use, we keep this for correctness):

```go
func ensureLeadInBeforeToolUse(blocks []AnthropicBlock) []AnthropicBlock {
	if len(blocks) == 0 {
		return blocks
	}
	// If first block is a tool_use, prepend a short text lead-in.
	if blocks[0].Type == "tool_use" {
		lead := AnthropicBlock{
			Type: "text",
			Text: "I will call a tool to help with this:",
		}
		return append([]AnthropicBlock{lead}, blocks...)
	}
	return blocks
}
```

---

## 5. Kiro Execution Layer (Non-Streaming + Synthetic SSE)

### 5.1 Synchronous Execution

```go
// KiroExecutor wraps the HTTP client and translation logic for Kiro.
type KiroExecutor struct {
	Config   KiroExecutorConfig
	TxConfig KiroTranslatorConfig
}

// ExecuteMessages handles an Anthropic /v1/messages request via Kiro.
func (e *KiroExecutor) ExecuteMessages(
	ctx context.Context,
	in AnthropicMessagesRequest,
) (AnthropicMessagesResponse, error) {
	// 1) Translate Anthropic → Kiro.
	kiroReq, err := TranslateKiroRequest(ctx, e.TxConfig, in)
	if err != nil {
		return AnthropicMessagesResponse{}, err
	}

	// 2) Call Kiro.
	kiroResp, err := e.callKiro(ctx, kiroReq)
	if err != nil {
		// Optional: fallback if Kiro returns "Improperly formed request".
		if isImproperlyFormedRequestError(err) && e.TxConfig.DropToolEvents == false {
			// retry with tool events forcibly dropped.
			forcedCfg := e.TxConfig
			forcedCfg.DropToolEvents = true
			kiroReq, err2 := TranslateKiroRequest(ctx, forcedCfg, in)
			if err2 != nil {
				return AnthropicMessagesResponse{}, err2
			}
			kiroResp, err = e.callKiro(ctx, kiroReq)
			if err != nil {
				return AnthropicMessagesResponse{}, err
			}
		} else {
			return AnthropicMessagesResponse{}, err
		}
	}

	// 3) Translate Kiro → Anthropic.
	return TranslateKiroResponse(ctx, in, kiroResp)
}

func (e *KiroExecutor) callKiro(ctx context.Context, req KiroRequest) (KiroResponse, error) {
	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, e.Config.EndpointURL, bytes.NewReader(body))
	httpReq.Header.Set("Authorization", "Bearer "+e.Config.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := e.Config.HTTPClient.Do(httpReq)
	if err != nil {
		return KiroResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		return KiroResponse{}, fmt.Errorf("kiro error %d: %s", resp.StatusCode, string(data))
	}

	var out KiroResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return KiroResponse{}, err
	}
	return out, nil
}
```

---

### 5.2 Synthetic Anthropic SSE from Kiro (Pseudo-Streaming)

> Kiro only returns full responses; the proxy synthesizes Anthropic-style SSE events (`message_start`, `content_block_start/delta/stop`, `message_delta`, `message_stop`) once the full response arrives.

```go
// StreamMessages synthesizes SSE events for /v1/messages?stream=true
// from a full Kiro response.
func (e *KiroExecutor) StreamMessages(
	ctx context.Context,
	in AnthropicMessagesRequest,
	stream chan<- AnthropicStreamEvent,
) error {
	// 1) Run non-streaming execution.
	resp, err := e.ExecuteMessages(ctx, in)
	if err != nil {
		return err
	}

	// 2) Build synthetic event sequence.
	id := resp.ID
	model := resp.Model

	// message_start
	stream <- NewMessageStartEvent(id, model)

	// For each block, we output content_block_* events.
	for idx, block := range resp.Content {
		// content_block_start
		stream <- NewContentBlockStartEvent(id, idx, block.Type)

		switch block.Type {
		case "text":
			// In a real implementation, we may chunk by sentences; here we send as one delta.
			stream <- NewContentBlockDeltaEvent(id, idx, block.Text)
		default:
			// For non-text, we may either serialize or skip; depends on Anthropic contract.
		}

		// content_block_stop
		stream <- NewContentBlockStopEvent(id, idx)
	}

	// message_delta (stop_reason, usage, etc.)
	stream <- NewMessageDeltaEvent(id, resp.StopReason, resp.Usage)

	// message_stop
	stream <- NewMessageStopEvent(id)

	return nil
}
```

Event structs are conceptual:

```go
type AnthropicStreamEvent struct {
	Type string // "message_start", "content_block_start", ...
	Data any
}

func NewMessageStartEvent(id, model string) AnthropicStreamEvent    { /* ... */ }
func NewContentBlockStartEvent(id string, index int, blockType string) AnthropicStreamEvent { /* ... */ }
func NewContentBlockDeltaEvent(id string, index int, text string) AnthropicStreamEvent      { /* ... */ }
func NewContentBlockStopEvent(id string, index int) AnthropicStreamEvent                    { /* ... */ }
func NewMessageDeltaEvent(id, stopReason string, usage *AnthropicUsage) AnthropicStreamEvent { /* ... */ }
func NewMessageStopEvent(id string) AnthropicStreamEvent                                    { /* ... */ }
```

---

## 6. Invariants & Guarantees

1. **No Tool Events Sent to Kiro**

   * `KiroConversationState.History` and `KiroUserInputMessage.Content` **never** contain serialized `tool_use` or `tool_result` structures.
   * Tool events from Anthropic are **flattened to plain text context** when `DropToolEvents == true`.

2. **Tool Description Clamping & Manifest**

   * Every tool sent to Kiro has `Description` length ≤ `ClampToolDesc` (default: 256 chars).
   * Full descriptions are preserved in `ToolContextManifest` and in the appended system “Tool reference manifest (hash → tool)” section.

3. **Tool Choice Preservation**

   * Anthropic `tool_choice` is mirrored into `KiroMessageContext.ClaudeToolChoice`, plus a brief “Tool directive” sentence injected into the system prompt (not shown in code, but implied by `buildSystemPromptWithToolReferences` extensibility).

4. **Sanitization**

   * All user/system/assistant texts sent to Kiro are passed through `sanitizeText`, removing:

     * ANSI escape sequences
     * Non-printable control characters
     * Known protocol noise (e.g., `<system-reminder>`).

5. **JSON Safety**

   * Tool arguments and results are parsed via `safeParseJSON`.
   * Failure to parse does **not** crash the translator; it falls back to the raw string.

6. **Stable Streaming Sequence**

   * Synthetic SSE respects Anthropic’s event order:

     * `message_start → [content_block_start → content_block_delta → content_block_stop]+ → message_delta → message_stop`.

7. **Fallback on Kiro Protocol Rejection**

   * If Kiro returns an “Improperly formed request” error and `DropToolEvents == false`, executor retries once with `DropToolEvents = true`, ensuring legacy misconfigurations don’t leave the UI stuck.
