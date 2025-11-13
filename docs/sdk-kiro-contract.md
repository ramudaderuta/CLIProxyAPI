# Kiro ⇄ Anthropic Translation & Execution Contract

*(Go-style pseudocode, derived from `convert-old.js` and Nov’25 Kiro protocol notes)* 

---

## 1. Scope & Goals

This document defines the **protocol contract** between:

* The **OpenAI-style front-end** (Claude Code / CLIProxyAPI clients),
* The **Anthropic Messages API–style internal representation**, and
* The **Kiro provider** (request/response + pseudo-streaming behavior).

It captures:

1. **Translation layer** behavior (OpenAI ⇄ Claude, Claude ⇄ Gemini) as it informs the Kiro implementation.
2. **Execution layer** behavior for Kiro: how requests are built, what is *allowed* in the payload, and how responses are mapped back to Anthropic SSE.
3. **Defensive helpers** (`safeParseJSON`, `_buildFunctionResponse`, `processClaudeContentToGeminiParts`, etc.) that must be preserved and/or ported into Go.

The goal is that **Kiro never sees an “improperly formed request”** and that TodoWrite / other tools behave identically to CLIProxyAPI’s reference behavior.

---

## 2. Core Types

### 2.1 Protocol & Conversion Types

```go
// Protocol family of a model/provider.
type ProtocolPrefix string

const (
    ProtocolOpenAI          ProtocolPrefix = "openai"
    ProtocolClaude          ProtocolPrefix = "claude"
    ProtocolGemini          ProtocolPrefix = "gemini"
    ProtocolOpenAIResponses ProtocolPrefix = "openai_responses"
)

// Direction of conversion.
type ConversionType string

const (
    ConversionRequest    ConversionType = "request"
    ConversionResponse   ConversionType = "response"
    ConversionStream     ConversionType = "streamChunk"
    ConversionModelList  ConversionType = "modelList"
)
```

### 2.2 Generic Conversion Entry

```go
// Function signature for a conversion.
type ConvertFunc func(data any, model string) (any, error)

// Conversion registry keyed by type → targetProtocol → sourceProtocol.
var conversionMap = map[ConversionType]map[ProtocolPrefix]map[ProtocolPrefix]ConvertFunc{
    ConversionRequest: {
        ProtocolOpenAI: {
            ProtocolGemini: toOpenAIRequestFromGemini,
            ProtocolClaude: toOpenAIRequestFromClaude,
        },
        ProtocolClaude: {
            ProtocolOpenAI:          toClaudeRequestFromOpenAI,
            ProtocolOpenAIResponses: toClaudeRequestFromOpenAIResponses,
        },
        ProtocolGemini: {
            ProtocolOpenAI:          toGeminiRequestFromOpenAI,
            ProtocolClaude:          toGeminiRequestFromClaude,
            ProtocolOpenAIResponses: toGeminiRequestFromOpenAIResponses,
        },
    },
    ConversionResponse: {
        ProtocolOpenAI: {
            ProtocolGemini: toOpenAIChatCompletionFromGemini,
            ProtocolClaude: toOpenAIChatCompletionFromClaude,
        },
        ProtocolClaude: {
            ProtocolGemini: toClaudeChatCompletionFromGemini,
            ProtocolOpenAI: toClaudeChatCompletionFromOpenAI,
        },
        ProtocolOpenAIResponses: {
            ProtocolGemini: toOpenAIResponsesFromGemini,
            ProtocolClaude: toOpenAIResponsesFromClaude,
        },
    },
    ConversionStream: {
        ProtocolOpenAI: {
            ProtocolGemini: toOpenAIStreamChunkFromGemini,
            ProtocolClaude: toOpenAIStreamChunkFromClaude,
        },
        ProtocolClaude: {
            ProtocolGemini: toClaudeStreamChunkFromGemini,
            ProtocolOpenAI: toClaudeStreamChunkFromOpenAI,
        },
        ProtocolOpenAIResponses: {
            ProtocolGemini: toOpenAIResponsesStreamChunkFromGemini,
            ProtocolClaude: toOpenAIResponsesStreamChunkFromClaude,
        },
    },
    ConversionModelList: {
        ProtocolOpenAI: {
            ProtocolGemini: toOpenAIModelListFromGemini,
            ProtocolClaude: toOpenAIModelListFromClaude,
        },
        ProtocolClaude: {
            ProtocolGemini: toClaudeModelListFromGemini,
            ProtocolOpenAI: toClaudeModelListFromOpenAI,
        },
    },
}
:contentReference[oaicite:1]{index=1}
```

### 2.3 Generic Entry Point

```go
func ConvertData(
    data any,
    convType ConversionType,
    fromProvider string,
    toProvider string,
    model string, // only used for response/stream/modelList
) (any, error) {
    fromProto := GetProtocolPrefix(fromProvider)
    toProto := GetProtocolPrefix(toProvider)

    targets, ok := conversionMap[convType]
    if !ok {
        return nil, fmt.Errorf("unsupported conversion type: %s", convType)
    }

    byTarget, ok := targets[toProto]
    if !ok {
        return nil, fmt.Errorf("no conversions defined for target protocol %s / type %s", toProto, convType)
    }

    fn, ok := byTarget[fromProto]
    if !ok {
        return nil, fmt.Errorf("no conversion from %s to %s / type %s", fromProto, toProvider, convType)
    }

    if convType == ConversionResponse || convType == ConversionStream || convType == ConversionModelList {
        return fn(data, model)
    }
    return fn(data, "")
}
:contentReference[oaicite:2]{index=2}
```

This registry is the **reference contract** that the Go-side Kiro translator must mirror.

---

## 3. Defensive Helpers (Shared Contract)

### 3.1 `safeParseJSON`

**Purpose:** robustly parse JSON-ish strings that may contain truncated escape sequences (`\`, `\u`, etc.).

Reference behavior: 

```go
func safeParseJSON(raw string) any {
    if raw == "" {
        return ""
    }
    cleaned := raw

    // Handle dangling backslash (e.g. truncated JSON)
    if strings.HasSuffix(cleaned, `\`) && !strings.HasSuffix(cleaned, `\\`) {
        cleaned = cleaned[:len(cleaned)-1]
    } else if strings.HasSuffix(cleaned, `\u`) ||
              strings.HasSuffix(cleaned, `\u0`) ||
              strings.HasSuffix(cleaned, `\u00`) {
        // Remove incomplete Unicode escape at the end:
        idx := strings.LastIndex(cleaned, `\u`)
        if idx >= 0 {
            cleaned = cleaned[:idx]
        }
    }

    var out any
    if err := json.Unmarshal([]byte(cleaned), &out); err != nil {
        // On failure, *return original string* rather than failing hard.
        return raw
    }
    if out == nil {
        return map[string]any{}
    }
    return out
}
```

Contract:

* Never panic or return an error.
* Sanitize obviously malformed tails.
* Prefer **parsed JSON**; fall back to **original string**.

This function must be used when:

* Parsing OpenAI `tool_calls[*].function.arguments`,
* Parsing tool outputs returning as stringified JSON,
* Translating tool result content into downstream formats (Claude/Gemini/Kiro).

---

### 3.2 `ToolStateManager` + `_buildFunctionResponse`

Reference behavior: 

```go
type ToolStateManager struct {
    mu          sync.RWMutex
    toolMapping map[string]string // funcName -> toolUseID
}

var toolStateManager = NewToolStateManager()

func NewToolStateManager() *ToolStateManager {
    return &ToolStateManager{
        toolMapping: make(map[string]string),
    }
}

func (m *ToolStateManager) StoreToolMapping(funcName, toolID string) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.toolMapping[funcName] = toolID
}

func (m *ToolStateManager) GetToolID(funcName string) (string, bool) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    id, ok := m.toolMapping[funcName]
    return id, ok
}
```

```go
// Build Gemini functionResponse from a tool_result / similar content block.
func buildFunctionResponse(item map[string]any) *FunctionResponsePart {
    if item == nil {
        return nil
    }

    // Detect that this is some form of tool result.
    isResult := item["type"] == "tool_result" ||
        item["tool_use_id"] != nil ||
        item["tool_output"] != nil ||
        item["result"] != nil ||
        item["content"] != nil
    if !isResult {
        return nil
    }

    // 1. Infer function name.
    var funcName string

    // 1a. From tool_use_id and global mapping.
    toolUseID, _ := item["tool_use_id"].(string)
    if toolUseID == "" {
        toolUseID, _ = item["id"].(string)
    }

    if toolUseID != "" {
        // Try to extract potential function name from call_<name>_<hash>.
        if strings.HasPrefix(toolUseID, "call_") {
            nameAndHash := strings.TrimPrefix(toolUseID, "call_")
            if pos := strings.LastIndex(nameAndHash, "_"); pos > 0 {
                potential := nameAndHash[:pos]
                if storedID, ok := toolStateManager.GetToolID(potential); ok && storedID == toolUseID {
                    funcName = potential
                }
            }
        }
    }

    // 1b. From fields like tool_name / name / function_name.
    if funcName == "" {
        for _, key := range []string{"tool_name", "name", "function_name"} {
            if v, ok := item[key].(string); ok && v != "" {
                funcName = v
                break
            }
        }
    }
    if funcName == "" {
        return nil
    }

    // 2. Extract response payload.
    var value any

    // Probe several possible result fields.
    for _, key := range []string{"content", "tool_output", "output", "response", "result"} {
        if v, ok := item[key]; ok {
            value = v
            break
        }
    }

    // If this is an array of parts, flatten text parts.
    if arr, ok := value.([]any); ok && len(arr) > 0 {
        var buf strings.Builder
        for _, p := range arr {
            block, _ := p.(map[string]any)
            if block == nil {
                continue
            }
            if t, _ := block["type"].(string); t == "text" {
                if text, _ := block["text"].(string); text != "" {
                    buf.WriteString(text)
                }
            }
        }
        if buf.Len() > 0 {
            value = buf.String()
        }
    }

    if value == nil {
        value = ""
    }

    // Gemini requires JSON object; wrap scalars.
    var resp map[string]any
    switch v := value.(type) {
    case map[string]any:
        resp = v
    default:
        resp = map[string]any{"content": fmt.Sprint(v)}
    }

    return &FunctionResponsePart{
        FunctionResponse: FunctionResponse{
            Name:     funcName,
            Response: resp,
        },
    }
}
```

Contract:

* Robustly recovers function name using mapping and ID conventions.
* Accepts multiple possible result field names.
* Normalizes arbitrary content into a JSON object.

---

### 3.3 `processClaudeContentToGeminiParts`

Reference behavior: 

```go
// Convert Anthropic content blocks → Gemini parts.
func processClaudeContentToGeminiParts(content any) []GeminiPart {
    if content == nil {
        return nil
    }

    // String → single text part.
    if s, ok := content.(string); ok {
        return []GeminiPart{{Text: s}}
    }

    // Array of blocks.
    blocks, ok := content.([]any)
    if !ok {
        return nil
    }

    parts := make([]GeminiPart, 0, len(blocks))

    for _, raw := range blocks {
        block, _ := raw.(map[string]any)
        if block == nil {
            logWarn("Skipping invalid content block")
            continue
        }

        typ, _ := block["type"].(string)
        switch typ {

        case "text":
            if txt, _ := block["text"].(string); txt != "" {
                parts = append(parts, GeminiPart{Text: txt})
            } else {
                logWarn("Invalid text in Claude text block")
            }

        case "image":
            src, _ := block["source"].(map[string]any)
            if src == nil {
                logWarn("Invalid image source in Claude image block")
                continue
            }
            if src["type"] == "base64" {
                mime, _ := src["media_type"].(string)
                data, _ := src["data"].(string)
                if mime != "" && data != "" {
                    parts = append(parts, GeminiPart{
                        InlineData: &InlineData{
                            MimeType: mime,
                            Data:     data,
                        },
                    })
                } else {
                    logWarn("Incomplete base64 image block")
                }
            }

        case "tool_use":
            name, _ := block["name"].(string)
            input, _ := block["input"].(map[string]any)
            if name == "" || input == nil {
                logWarn("Invalid tool_use block")
                continue
            }
            parts = append(parts, GeminiPart{
                FunctionCall: &FunctionCall{
                    Name: name,
                    Args: input,
                },
            })

        case "tool_result":
            // Anthropic tool_result only has tool_use_id; Gemini functionResponse needs a name.
            toolUseID, _ := block["tool_use_id"].(string)
            if toolUseID == "" {
                logWarn("tool_result missing tool_use_id")
                continue
            }
            // For now, treat tool_use_id as function name, and wrap content.
            resp := buildFunctionResponse(block)
            if resp != nil {
                parts = append(parts, GeminiPart{
                    FunctionResponse: &resp.FunctionResponse,
                })
            }

        default:
            if txt, _ := block["text"].(string); txt != "" {
                parts = append(parts, GeminiPart{Text: txt})
            } else {
                logWarn("Unsupported Claude block type %q", typ)
            }
        }
    }

    return parts
}
```

Contract:

* Ignore invalid blocks instead of crashing.
* Handle `text`, `image`, `tool_use`, `tool_result` explicitly.
* Provide a viable Gemini `functionCall` / `functionResponse` representation even though Anthropic tool IDs do not carry names.

---

## 4. OpenAI → Claude Request Translation (`toClaudeRequestFromOpenAI`)

This path is **directly implicated in the TodoWrite “Improperly formed request” 400s**. It must:

* Produce a **valid Anthropic Messages request** and
* Respect the **Kiro contract** when this Anthropic request is subsequently mapped into Kiro.

Reference behavior:  

### 4.1 High-Level Behavior

```go
func toClaudeRequestFromOpenAI(openaiReq OpenAIChatRequest) ClaudeMessagesRequest {
    // 1. Peel off system messages.
    messages := openaiReq.Messages
    sysInstr, nonSystem := extractAndProcessSystemMessages(messages)

    var claudeMsgs []ClaudeMessage

    for _, m := range nonSystem {
        role := "user"
        if m.Role == "assistant" {
            role = "assistant"
        }
        var content []ClaudeContentBlock

        switch m.Role {

        case "tool":
            // OpenAI tool role → Anthropic tool_result wrapped in user message.
            content = append(content, ClaudeContentBlock{
                Type:      "tool_result",
                ToolUseID: m.ToolCallID, // from OpenAI tool message
                Content:   safeParseJSON(m.Content),
            })
            claudeMsgs = append(claudeMsgs, ClaudeMessage{Role: "user", Content: content})

        case "assistant":
            if len(m.ToolCalls) > 0 {
                // Assistant tool calls → tool_use blocks in assistant message.
                var blocks []ClaudeContentBlock
                for _, tc := range m.ToolCalls {
                    blocks = append(blocks, ClaudeContentBlock{
                        Type:  "tool_use",
                        ID:    tc.ID,
                        Name:  tc.Function.Name,
                        Input: safeParseJSON(tc.Function.Arguments),
                    })
                }
                claudeMsgs = append(claudeMsgs, ClaudeMessage{Role: "assistant", Content: blocks})
                continue
            }
            // fallthrough to generic content handling

        default:
            // user / everything else fall through
        }

        if len(content) == 0 {
            // Generic multimodal mapping: text, images, audio.
            content = convertOpenAIContentToClaudeBlocks(m.Content)
        }

        if len(content) > 0 {
            claudeMsgs = append(claudeMsgs, ClaudeMessage{
                Role:    role,
                Content: content,
            })
        }
    }

    // Base Claude request.
    claudeReq := ClaudeMessagesRequest{
        Model:     openaiReq.Model,
        Messages:  claudeMsgs,
        MaxTokens: defaultIfZero(openaiReq.MaxTokens, DefaultMaxTokens),
        Temperature: defaultIfZero(
            openaiReq.Temperature, DefaultTemperature,
        ),
        TopP: defaultIfZero(openaiReq.TopP, DefaultTopP),
    }

    if sysInstr != nil {
        claudeReq.System = extractTextFromMessageContent(sysInstr.Parts[0].Text)
    }

    if len(openaiReq.Tools) > 0 {
        claudeReq.Tools = mapOpenAIToolsToClaude(openaiReq.Tools)
        claudeReq.ToolChoice = buildClaudeToolChoice(openaiReq.ToolChoice)
    }

    return claudeReq
}
```

**Key points:**

* Tool **responses** from OpenAI (role `tool`) become `tool_result` blocks in a **user** message.
* Assistant tool **calls** (`tool_calls`) become `tool_use` blocks in **assistant** messages.
* Text and media payloads are converted to Anthropic blocks (`text`, `image`, placeholder for audio).
* System instructions are merged into a single `system` string.

### 4.2 Where Kiro Comes In

Kiro’s Go-side translator (`BuildKiroRequest`) consumes `ClaudeMessagesRequest` and must **strip out client-side tool events** (assistant `tool_use` and user `tool_result`) before hitting Kiro.

Kiro translator contract is:

```go
func BuildKiroRequest(claudeReq ClaudeMessagesRequest) (KiroRequest, error) {
    // 1. Partition current vs history.
    hist, current := splitMessages(claudeReq.Messages)

    // 2. Sanitize history for Kiro:
    //    - DROP all tool_use / tool_result blocks.
    //    - Fold them into plain text if necessary.
    sanitizedHistory := sanitizeHistoryForKiro(hist)

    // 3. Build Kiro conversationState.currentMessage with:
    //    - userInputMessage.content (latest user text only)
    //    - userInputMessageContext.tools (clamped tool specs)
    //    - toolContextManifest + system prompt appendix for full tool descriptions
    //    - claudeToolChoice metadata and textual “Tool directive …” block
    currentMsg := buildKiroUserInputMessage(claudeReq, current)

    // 4. Return Kiro request in provider’s native schema.
    return KiroRequest{
        ConversationState: ConversationState{
            CurrentMessage: currentMsg,
            History:        sanitizedHistory,
        },
    }, nil
}
```

Where:

```go
func sanitizeHistoryForKiro(claudeMsgs []ClaudeMessage) []KiroHistoryItem {
    var out []KiroHistoryItem

    for _, msg := range claudeMsgs {
        // Drop tool_use/tool_result events completely to satisfy Kiro contract.
        filteredBlocks := filterOutToolEvents(msg.Content)

        // Optionally fold tool outputs into text summaries if you want Kiro to “see” them
        // as natural language:
        if len(filteredBlocks) == 0 && containsToolEvents(msg.Content) {
            summary := summarizeToolEvents(msg.Content)
            if summary != "" {
                filteredBlocks = []ClaudeContentBlock{{
                    Type: "text",
                    Text: summary,
                }}
            }
        }

        if len(filteredBlocks) == 0 {
            continue
        }

        // Convert Anthropic blocks -> Kiro’s internal message schema (plain text only).
        kiroMsg := mapClaudeBlocksToKiroHistoryItem(msg.Role, filteredBlocks)
        out = append(out, kiroMsg)
    }
    return out
}
```

This is where TodoWrite was intermittently failing: **resume flows that carried `tool_use` / `tool_result` events into the rebuilt Kiro request** violate this contract and yield a 400.

---

## 5. Claude → Gemini Translation (for completeness)

Kiro may not directly speak Gemini today, but the **defensive patterns** here are reused in Kiro-side tooling, so they form part of the contract.

### 5.1 `toGeminiRequestFromClaude`

Reference behavior: 

```go
func toGeminiRequestFromClaude(claudeReq ClaudeMessagesRequest) GeminiRequest {
    if claudeReq == (ClaudeMessagesRequest{}) {
        logWarn("invalid claudeRequest")
        return GeminiRequest{Contents: nil}
    }

    var geminiReq GeminiRequest
    geminiReq.Contents = []GeminiContent{}

    // System instruction.
    if claudeReq.System != "" {
        geminiReq.SystemInstruction = GeminiSystemInstruction{
            Parts: []GeminiPart{{Text: stringifySystem(claudeReq.System)}},
        }
    }

    // Messages → contents.
    for _, msg := range claudeReq.Messages {
        if msg.Role == "" || msg.Content == nil {
            logWarn("Skipping invalid Claude message")
            continue
        }

        role := "user"
        if msg.Role == "assistant" {
            role = "model"
        }
        parts := processClaudeContentToGeminiParts(msg.Content)

        // If we see a functionResponse, that content becomes role 'function'.
        if containsFunctionResponse(parts) {
            geminiReq.Contents = append(geminiReq.Contents, GeminiContent{
                Role:  "function",
                Parts: filterFunctionResponses(parts),
            })
        } else if len(parts) > 0 {
            geminiReq.Contents = append(geminiReq.Contents, GeminiContent{
                Role:  role,
                Parts: parts,
            })
        }
    }

    // Generation config.
    geminiReq.GenerationConfig = GenerationConfig{
        MaxOutputTokens: defaultIfZero(claudeReq.MaxTokens, DefaultGeminiMaxTokens),
        Temperature:     defaultIfZero(claudeReq.Temperature, DefaultTemperature),
        TopP:            defaultIfZero(claudeReq.TopP, DefaultTopP),
    }

    // Tools.
    if len(claudeReq.Tools) > 0 {
        decls := make([]FunctionDeclaration, 0, len(claudeReq.Tools))
        for _, tool := range claudeReq.Tools {
            if tool.Name == "" {
                logWarn("Skipping invalid tool declaration")
                continue
            }
            // Optional: commented-out TodoWrite filter is here in JS reference.
            // delete tool.input_schema.$schema
            decls = append(decls, FunctionDeclaration{
                Name:        tool.Name,
                Description: tool.Description,
                Parameters:  sanitizeJSONSchema(tool.InputSchema),
            })
        }
        if len(decls) > 0 {
            geminiReq.Tools = []GeminiTools{{FunctionDeclarations: decls}}
        }
    }

    if claudeReq.ToolChoice != nil {
        geminiReq.ToolConfig = buildGeminiToolConfigFromClaude(claudeReq.ToolChoice)
    }

    return geminiReq
}
```

---

## 6. Execution Layer: KiroExecutor

### 6.1 Responsibilities

The **Kiro executor** is responsible for:

1. Accepting a canonical *provider-agnostic* request (Anthropic Messages format).
2. Applying Kiro’s **request contract**:

   * Only user text + model/context fields,
   * Tool specs in `userInputMessageContext.tools`, **truncated** descriptions,
   * No `tool_use` / `tool_result` events in history.
3. Issuing HTTP calls to Kiro (non-streaming).
4. Mapping **Kiro responses → Anthropic Messages** and then, optionally, **Anthropic SSE** for CLI/Claude Code.

### 6.2 Request Path

```go
type KiroExecutor struct {
    client *http.Client
    // config: base URL, auth token, headers, etc.
}

func (e *KiroExecutor) Execute(
    ctx context.Context,
    claudeReq ClaudeMessagesRequest,
    kiroCfg KiroConfig,
) (*ClaudeMessagesResponse, error) {

    // 1. Build Kiro request under protocol contract.
    kiroReq, err := BuildKiroRequest(claudeReq)
    if err != nil {
        return nil, fmt.Errorf("build kiro request: %w", err)
    }

    // 2. Apply provider-specific headers (X-IFlow-Task-Directive, etc.),
    //    though for Kiro this is usually auth + tracing.
    httpReq, err := http.NewRequestWithContext(ctx, "POST", kiroCfg.Endpoint, encodeJSON(kiroReq))
    if err != nil {
        return nil, err
    }

    httpReq.Header.Set("Authorization", "Bearer "+kiroCfg.Token)
    httpReq.Header.Set("Content-Type", "application/json")
    applyCustomHeadersFromAttrs(httpReq.Header, kiroCfg.CustomHeaders)

    // 3. Perform call.
    resp, err := e.client.Do(httpReq)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)

    if resp.StatusCode >= 400 {
        // For 400 "Improperly formed request", we may optionally:
        // - log,
        // - attempt a fallback rebuild that *strips* tool events even more aggressively.
        return nil, fmt.Errorf("kiro error %d: %s", resp.StatusCode, truncate(body, 2<<10))
    }

    // 4. Parse Kiro response and map → Anthropic messages.
    kiroResp, err := decodeKiroResponse(body)
    if err != nil {
        return nil, fmt.Errorf("decode kiro response: %w", err)
    }

    anthResp := ConvertKiroResponseToAnthropic(kiroResp)
    return &anthResp, nil
}
```

### 6.3 Kiro Response → Anthropic Messages

The specifics depend on Kiro’s schema, but the contract implied by your tests and docs is:

* Kiro outputs **text** and **tool use events** in a structured internal schema.
* The translator:

  * Removes **protocol noise** (e.g., internal `content-type` fragments or headers accidentally embedded in content).
  * Produces a **single Anthropic `message`** with:

    * `role: "assistant"`,
    * `content`: ordered blocks:

      * natural language lead-in text before any `tool_use`, if absent in Kiro response (your test `TestBuildAnthropicMessagePayloadAddsLeadInWhenContentMissing`),
      * `tool_use` blocks for any tools Kiro has invoked,
      * follow-up text, if any.
* `usage` is filled from Kiro’s accounting, and `stop_reason` / `stop_sequence` are aligned with Anthropic’s semantics.

Pseudocode:

```go
func ConvertKiroResponseToAnthropic(kiroResp KiroResponse) ClaudeMessagesResponse {
    blocks := []ClaudeContentBlock{}

    // 1. Optional natural-language lead-in.
    if lacksUserFacingIntro(kiroResp) {
        blocks = append(blocks, ClaudeContentBlock{
            Type: "text",
            Text: "I'll use the requested tools and then continue.",
        })
    }

    // 2. Tool invocations from Kiro internal schema.
    for _, t := range kiroResp.ToolInvocations {
        blocks = append(blocks, ClaudeContentBlock{
            Type:  "tool_use",
            ID:    t.ID,
            Name:  t.Name,
            Input: t.Args,
        })
    }

    // 3. Plain text response (sanitized).
    if txt := sanitizeKiroText(kiroResp.Text); txt != "" {
        blocks = append(blocks, ClaudeContentBlock{
            Type: "text",
            Text: txt,
        })
    }

    return ClaudeMessagesResponse{
        ID:         newMessageID(),
        Type:       "message",
        Role:       "assistant",
        Content:    blocks,
        Model:      kiroResp.Model,
        StopReason: mapKiroStopReasonToAnthropic(kiroResp.StopReason),
        Usage: Usage{
            InputTokens:  kiroResp.Usage.InputTokens,
            OutputTokens: kiroResp.Usage.OutputTokens,
        },
    }
}
```

---

## 7. Pseudo-Streaming SSE Synthesis

Because **Kiro does not stream**, KiroExecutor must:

1. Get the **full Kiro response**.
2. Convert it to an Anthropic `message`.
3. **Synthesize** Anthropic SSE events that mimic live streaming (CLIProxyAPI legacy behavior):

```go
func (e *KiroExecutor) Stream(ctx context.Context, claudeReq ClaudeMessagesRequest, writer SSEWriter) error {
    resp, err := e.Execute(ctx, claudeReq, e.cfg)
    if err != nil {
        return err
    }

    // Build deterministic SSE sequence:
    // message_start → content_block_start/delta/stop → message_delta → message_stop
    s := NewAnthropicStreamBuilder(*resp)

    writer.Send(s.BuildMessageStart())
    for i, block := range resp.Content {
        writer.Send(s.BuildContentBlockStart(i, block))
        for _, delta := range s.BuildContentDeltas(i, block) {
            writer.Send(delta)
        }
        writer.Send(s.BuildContentBlockStop(i, block))
    }
    writer.Send(s.BuildMessageDelta())
    writer.Send(s.BuildMessageStop())
    return nil
}
```

This preserves:

* Tool blocks **first**, then text (as in your recorded SSE fixtures).
* `stop_reason`, `usage`, and any `followupPrompt` or metadata fields.
