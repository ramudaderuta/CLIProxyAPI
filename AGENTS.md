# CLIProxyAPI - AI Model Proxy Server

## Architecture Overview

CLIProxyAPI is a Go-based HTTP proxy server that provides unified OpenAI/Gemini/Claude-compatible API interfaces for multiple AI providers. It abstracts provider differences and handles authentication, translation between API formats, and request routing.

## Current Work Context (Nov 2025)

- **Session summary prompt (drop into Claude Code when resuming work)**:  
  *“Continue the Nov’25 Kiro <> Claude Code parity effort. Kiro now clamps tool descriptions to ≤256 chars, injects a ‘Tool reference (full descriptions preserved…)’ block, and mirrors Anthropic `tool_choice` into `claudeToolChoice`. Verify future edits keep these guarantees by re-running `go test ./tests/unit/kiro -run 'BuildRequest|ParseResponse' -count=1`.”*
- **Kiro ↔ Claude Code parity push**: Sanitized Anthropic payload translation to strip ANSI escapes, stray `<system-reminder>` blocks, and other control bytes before they touch Kiro (`internal/translator/kiro/request.go`). Anthropic responses are likewise scrubbed to remove protocol metadata (e.g. `content-type` fragments) before returning to Claude Code.
- **Tool hygiene + parity**: All Claude Code built-ins (Task, Bash, Grep, Skill, SlashCommand, etc.) pass through to Kiro with descriptions clamped to ≤256 chars (hard limit enforced by Kiro). When a description is truncated we inject a `Tool reference (full descriptions preserved…)` block into the system prompt so Claude still sees the complete instructions. User-defined tools share the same sanitizer.
- **Tool-choice handling**: Anthropic `tool_choice` is translated into `claudeToolChoice` metadata (and a short “Tool directive” sentence in the system prompt) so mandatory tool calls survive the Kiro hop even though the native API doesn’t understand Anthropic’s schema.
- **Anthropic response compliance**: `BuildAnthropicMessagePayload` now generates natural-language lead-ins before a `tool_use`, strips `total_tokens`, and keeps `stop_reason`/`usage` aligned with the current Messages API.
- **iFlow TodoWrite loop fix**: The repeating `TodoWrite` invocations reported in Nov ’25 were traced to iFlow requests losing the operator-mandated `X-IFlow-Task-Directive` header when passing through the proxy. Commit `0c115df` adds `headers:` to config entries, mirrors them into auth `header:*` attributes, and calls `util.ApplyCustomHeadersFromAttrs` inside every OpenAI-compatible executor (iFlow rides this path). If you change watcher auth synthesis or executor header plumbing, re-run `go test ./tests/unit/iflow -run TestIFlowExecutorForwardsCustomHeaders -count=1` to ensure the directive still survives the hop.
- **Reference parity / captured fixtures**:
  - Recorded SSE runs from CLIProxyAPI live under `tmp/CLIProxyAPI` (checked into this repo). When adjusting streaming logic, grab the matching inputs from `src/claude/claude-kiro.js` fixtures and mirror the event order (tool blocks first, then text).
  - Legacy failure logs still live in `logs/v1-messages-2025-11-08T170438-*.log` (broken) and `logs/v1-messages-2025-11-08T175404-*.log` (fixed). Repro payloads: `/tmp/claude_request.json`, `/tmp/claude_request_desc256.json`, `/tmp/claude_request_no_tools.json`.
- **Tests covering the fixes**:
  - `TestParseResponseStripsProtocolNoiseFromContent` and `TestBuildAnthropicMessagePayloadAddsLeadInWhenContentMissing` (anthropic sanitizer).
  - `TestBuildRequestStripsControlCharactersFromUserContent`, `TestBuildRequestPreservesLongToolDescriptions`, `TestBuildRequestStripsMarkupFromToolDescriptions`, `TestBuildRequestPreservesClaudeCodeBuiltinTools`, and `TestBuildRequestMovesTrailingAssistantMessagesIntoHistory` (request translator hardening).
  - `TestBuildAnthropicStreamingChunksMatchReference`, `TestConvertKiroStreamToAnthropic_*`, and `TestConvertKiroStreamToAnthropic_StopReasonOverrides` validate the Go SSE output vs. CLIProxyAPI recordings (plain text, tool chains, follow-ups, cancel/timeout).
  - All run via `go test ./tests/unit/kiro -run 'BuildRequest|ParseResponse' -count=1`.
  - Full regression sweep: `go test ./tests/regression/kiro` plus `go test ./...` before shipping.

### Outstanding TODOs for Claude Code feature parity

> **Reference status:** CLIProxyAPI does not implement plan-mode orchestration, real streaming, or hash/fetch manifests today. We intentionally scope our TODO list to work that is feasible (plan-mode semantics) and document how the other concerns are handled so operators know the trade-offs.

Currently there are no additional in-flight TODOs beyond the regression/coverage work listed above.

The following items are **not planned** because the engineering cost / stability risk outweighs the upside at the moment. Instead, we document the current behavior so users know what to expect:

#### Plan-mode orchestration (not planned)

* **Upstream reality:** Claude Code is the only surface that understands `planMode`; every upstream provider (Kiro, iFlow/OpenAI-compatible, Gemini, etc.) just sees plain text/tool calls with no enforcement ability.
* **Our approach:** Translators continue to pass through `planMode` metadata and inject reminder text (“Tool directive… ExitPlanMode…”) so Claude gets context, but executors intentionally stay stateless and do not try to run a plan engine.
* **Why we stay here:** Implementing a runtime state machine (enter/exit enforcement, telemetry, abort paths) would add complex coordination inside every executor plus UI plumbing, yet provides limited benefit until upstream vendors expose native plan semantics. We’d rather focus on Kiro parity and regression hardening.

#### Streaming parity (live SSE)

* **Upstream reality:** Kiro’s public API only returns full responses. CLIProxyAPI buffers everything too.
* **Our approach:** We synthesize Anthropic-style SSE (`message_start → content_block start/delta/stop → message_delta → message_stop`) once the full response arrives. `internal/translator/kiro/response.go` handles synthetic builders; `internal/translator/kiro/stream_mapper.go` handles legacy SSE output. Despite being “pseudo streaming,” this preserves `followupPrompt`, `stop_reason`, tool deltas, and usage so Claude Code renders the same event sequence CLIProxyAPI emits.
* **Why we stay here:** Implementing a custom AWS event-stream reader against `SendMessageStreaming` would add long-lived HTTP handling, retry semantics, and new failure modes without upstream support. Until Amazon exposes a stable streaming API, deterministic SSE synthesis remains the safest option.

#### Tool-context hash/fetch transport

* **Kiro limit:** We still clamp each tool description to 256 chars inside `userInputMessageContext.tools[*].toolSpecification`.
* **Full text preserved:** Whenever a description is truncated we mirror the entire text in two places:
  1. `toolContextManifest` (inside `userInputMessageContext`) with entries `{ name, hash (first 64 bits of SHA-256), length, description }`.
  2. A system-prompt appendix: “Tool reference manifest (hash → tool)” listing each tool as `- Name [hash, length chars]: full description`.
* **No external registry:** The hash is only a label within the same request so Claude can match entries; nothing is fetched by hash. Keeping the manifest inline avoids adding a stateful service while still letting Claude read the full instructions.

Currently there are no additional in-flight TODOs beyond the regression/coverage work listed above.

The following items are **not planned** because the engineering cost / stability risk outweighs the upside at the moment. Instead, we document the current behavior so users know what to expect:

#### Plan-mode orchestration (not planned)

* **Upstream reality:** Claude Code is the only surface that understands `planMode`; every upstream provider (Kiro, iFlow/OpenAI-compatible, Gemini, etc.) just sees plain text/tool calls with no enforcement ability.
* **Our approach:** Translators continue to pass through `planMode` metadata and inject reminder text (“Tool directive… ExitPlanMode…”) so Claude gets context, but executors intentionally stay stateless and do not try to run a plan engine.
* **Why we stay here:** Implementing a runtime state machine (enter/exit enforcement, telemetry, abort paths) would add complex coordination inside every executor plus UI plumbing, yet provides limited benefit until upstream vendors expose native plan semantics. We’d rather focus on Kiro parity and regression hardening.

#### Streaming parity (live SSE)

* **Upstream reality:** Kiro’s public API only returns full responses today. CLIProxyAPI has no special access either—it buffers the entire body and then pretends it streamed.
* **Our approach:** CLIProxy uses the same “deterministic SSE synthesis.” When a Kiro call completes we immediately repackage the final payload into Anthropic-style events (`message_start → content_block_start/delta/stop → message_delta → message_stop`).  
  * `internal/translator/kiro/response.go` handles the synthetic OpenAI/Anthropic builders.
  * `internal/translator/kiro/stream_mapper.go` handles legacy SSE captures (e.g., the `content-type` noise that Kiro sometimes embeds) and still outputs Anthropic-compatible deltas.
* **What users get:** Even though the bytes arrive at once, the mapper keeps `followupPrompt`, `stop_reason`, partial tool JSON, and usage counters intact so Claude Code renders the exact same sequence CLIProxyAPI emits.
* **Why we stay here:** Implementing a custom AWS event-stream reader against `SendMessageStreaming` would mean maintaining long-lived HTTP connections, chunk resynchronization, retries, etc., without upstream guarantees. Until Amazon publishes a supported streaming API, buffering + deterministic SSE mapping remains the safest option.

#### Tool-context hash/fetch transport

* **Kiro limit:** CLIProxy hard-clamps each tool description to ≤256 chars.
* **Full text preserved:** The complete description is included **inside the same request** in two mirrored places so **Claude Code** can read it even though Kiro only sees the truncated version.
* **No external registry:** We intentionally **do not** use a remote hash registry or fetch-on-demand service to avoid adding a stateful dependency and failure mode.
**What we send to Kiro**
* Every tool appears in `userInputMessageContext.tools[*].toolSpecification`.
* The `description` field is **truncated to 256 chars** and **no hashing** is involved in what Kiro validates/enforces.
**How Claude Code gets the full text**
When a description is truncated, we add **two mirrored metadata blocks** that carry the **entire** description:
1. **`toolContextManifest` (inside `userInputMessageContext`)**
   * Each entry: `{ name, hash, length, description }`.
   * `hash` = **first 64 bits of SHA-256** of the original (untruncated) description.
   * Purpose: a **stable identifier within the same request** so Claude Code can match manifest items to the truncated tools.
   * **No fetching** happens via this hash—the full `description` string is already present.
2. **System-prompt appendix: “Tool reference manifest (hash → tool)”**
   * A human-readable summary line per tool, e.g. `Tool XYZ [abcdef1234567890, 512 chars] …`.
   * The `hash` here is **just a label**, **not** a lookup key.              
                                                                                                                              
### Action items for the AI Agent (close the remaining Kiro-provider gap)

1. **Plan-mode parity (not planned)**: We continue to document the limitation, pass through `planMode` metadata, and inject textual directives, but no executor/runtime state machine work is scheduled.
2. **Regression hardening**: After each change above, add targeted fixtures/tests under `tests/unit/kiro/` and update `tests/TEST_DOCUMENTATION.md` so the CLIProxyAPI parity guarantees remain enforced long-term.
### Solution approach for the Nov 2025 “Improperly formed request” issue

1. **Clamp + mirror tool descriptions**: Kiro refuses payloads with >256-char tool descriptions, so we hard-cap descriptions inside `userInputMessageContext` but append a `Tool reference (full descriptions preserved…)` block to the system prompt containing the original text. This keeps Kiro happy without hiding instructions from Claude Code.
2. **Propagate tool_choice metadata**: When Claude Code sets `tool_choice`, we emit a `claudeToolChoice` map and a short “Tool directive” sentence in the system prompt so Kiro honors mandatory tool invocations.
3. **Regression coverage**: `TestBuildRequestPreservesLongToolDescriptions`, `TestBuildRequestPreservesClaudeCodeBuiltinTools`, and `TestBuildRequestAddsToolReferenceForTruncatedDescriptions` ensure the clamping + mirroring behavior doesn’t regress. Always run `go test ./tests/unit/kiro -run 'BuildRequest|ParseResponse' -count=1` before shipping changes that touch Kiro request translation.

## 2025‑11‑13 Kiro “Improperly formed request” deep‑dive

### Bisection Results

- orig (full `claude_request_todowrite_continue.json`): 200 with SSE `error` (body: “Improperly formed request.”)
- no_tools (drop `.tools`): 200 with SSE `error` (same body)
- no_tools_no_sys (drop `.tools` and `.system`): 200 with SSE `error`
- only_first_user (keep only `messages[0]`): 200 OK, normal response
- two_user (keep `messages[0]`, `messages[1]`): 200 OK, normal response
- user_and_assistant (keep `messages[0]`, assistant `tool_use`): 400 Improperly formed request
- user_and_result (keep `messages[0]`, user `tool_result`): 400 Improperly formed request
- user_asst_result (keep `messages[0]`, assistant `tool_use`, user `tool_result`): 400 Improperly formed request

### Conclusion (request contract with Kiro)

- Kiro’s `generateAssistantResponse` rejects client‑sent transcript events: any assistant `tool_use` or user `tool_result` in the request body leads to “Improperly formed request.”
- Kiro accepts plain user text (and optional tool specifications) but treats tool events as output‑only. Therefore, client resumes must not send tool events; instead, fold tool outputs into plain text.

### Action Plan

Option 1: Strip Tool Events; Fold Results Into Text (Kiro‑safe path)                                        
                                                                                                            
- Summary: Remove assistant tool_use and user tool_result from the Kiro request. Append tool result         
  text to the user message content and keep tool specs in userInputMessageContext.tools (≤256 char          
  descriptions).                                                                                            
- UX impact:                                                                                                
    - Pros: Works with Kiro; user gets a normal continuation; no provider errors.                           
    - Cons: “Mandatory tool” semantics aren’t enforced upstream; Claude Code will render tool updates from  
      content, not structured events.                                                                       
- Difficulty: Low (translator-only change).                                                                 
- Risks:                                                                                                    
    - Large tool outputs inlined into content can increase tokens; mitigate with summarization/clamping.    
    - Some tests expecting structured toolResults must be adapted to Kiro‑safe behavior.                    
- Where to implement:                                                                                       
    - Build Kiro request: internal/translator/kiro/request.go:80–220                                        
    - Tool result extraction: internal/translator/kiro/request.go:217–260                                   
- Tests to (re)run:                                                                                         
    - go test ./tests/unit/kiro -run 'BuildRequest|ParseResponse' -count=1                                  
    - Replay fixture to verify 200 OK and no toolUse/toolResults in logs.                                   
                                                                                                            
Option 2: Intelligent Routing (Preserve Tool Semantics via OpenAI‑compat)                                   
                                                                                                            
- Summary: If transcript includes tool_use or tool_result, route the request to the OpenAI-compatible       
  executor (e.g., iFlow) instead of Kiro. Keep Kiro as default for plain-text chats.                        
- UX impact:                                                                                                
    - Pros: Tool fidelity preserved (structured events, tool_choice enforcement).                           
    - Cons: Mixed provider behavior; latency/usage differs by route; requires provider credentials          
      configured.                                                                                           
- Difficulty: Medium (routing rule + config + tests).                                                       
- Risks:                                                                                                    
    - Misrouting edge cases; add logs/metrics to monitor routing.                                           
    - Operators must supply iFlow/OpenAI-compatible keys.                                                   
- Where to implement:                                                                                       
    - Executor routing layer: internal/runtime/executor (detect in request, branch to Kiro vs.              
      openai_compat).                                                                                       
- Tests:                                                                                                    
    - Unit test to assert: with tool events → OpenAI path; without → Kiro path.                             
                                                                                                            
Option 3: Two‑Step Emulation (Textual Tool Context Injection)                                               
                                                                                                            
- Summary: For transcripts with tool events, convert to plain text and inject a small “Tool result          
  context:” block in the system/user content. Optionally add the “Tool directive” sentence to prompt        
  behavior.                                                                                                 
- UX impact:                                                                                                
    - Pros: Kiro gets the context; user sees coherent continuation; stays on one provider.                  
    - Cons: No structured tool enforcement; longer prompts; potential token increase.                       
- Difficulty: Low–Medium (translator prompt stitching + clamping policy).                                   
- Risks:                                                                                                    
    - Must sanitize to avoid control characters; clamp long tool outputs.                                   
- Where to implement:                                                                                       
    - System prompt combining: internal/translator/kiro/request.go:58–112                                   
    - Text sanitizer: internal/translator/kiro/request.go:520–566                                           
                                                                                                            
Option 4: Fallback Retry on Kiro Error (Self‑Healing)                                                       
                                                                                                            
- Summary: On “Improperly formed request.” from Kiro, automatically retry once after removing tool events   
  and folding their text into the user message.                                                             
- UX impact:                                                                                                
    - Pros: Robust; hides upstream constraints from users.                                                  
    - Cons: One extra round-trip when first attempt fails.                                                  
- Difficulty: Medium (error interception + rewrite + retry).                                                
- Risks:                                                                                                    
    - Must prevent loops; cap retries; ensure idempotence.                                                  
- Where to implement:                                                                                       
    - Kiro client: internal/runtime/executor/kiro_client.go:59–117 (wrap doRequest with retry transform).   
                                                                                                            
Option 5: Feature Flag (Operator Control)                                                                   
                                                                                                            
- Summary: Add config flag kiro.dropToolEvents: true|false to toggle Kiro‑safe behavior. Default true.      
- UX impact:                                                                                                
    - Pros: Operators can experiment without redeploys; easy rollback.                                      
    - Cons: More configuration surface.                                                                     
- Difficulty: Low (config + conditional branches).                                                          
- Risks:                                                                                                    
    - Divergent environments; document defaults clearly.                                                    
- Where to implement:                                                                                       
    - Config read: internal/config/… and branch in internal/translator/kiro/request.go.                     
                                                                                                            
Option 6 (Not Recommended Now): Build a Local Plan/Tool Orchestrator                                        
                                                                                                            
- Summary: Implement a runtime engine to ingest tool_use/tool_result client events and orchestrate          
  subcalls to Kiro.                                                                                         
- UX impact:                                                                                                
    - Pros: Full fidelity; consistent across providers.                                                     
    - Cons: Complex state machine, error handling, and long‑lived orchestration; high maintenance.          
- Difficulty: High.                                                                                         
- Risks:                                                                                                    
    - Out of scope per AGENTS.md; large stability risk without upstream support.  

### How to verify locally

- Unit tests: `go test ./tests/unit/kiro -run 'BuildRequest|ParseResponse' -count=1`
- Real replay: start `./cli-proxy-api --config config.test.yaml` and POST
  `tests/shared/testdata/nonstream/claude_request_todowrite_continue.json` to `/v1/messages`.
  - First attempt may receive an upstream error body; fallback retry should rebuild the payload and return a normal assistant response.
  - Latest request bodies are logged under `logs/v1-messages-*.log`; ensure no client‑sent `tool_use`/`tool_result` reach Kiro in the fallback request.

## Architecture Analysis

### **1. Standard Translator Architecture (e.g., claude↔openai)**

Most translators use a generic SDK-based translator framework:
```javascript
sdk/translator/
├── translator.go     # generic translator registry
├── registry.go       # format-to-format registry
└── ...

internal/translator/
├── openai/claude/     # OpenAI ↔ Claude
├── claude/openai/     # Claude ↔ OpenAI
├── gemini/claude/     # Gemini ↔ Claude
└── ...
```

These translators all:
- use the unified `interfaces.TranslateRequestFunc` interface
- register in the SDK’s registry
- support conversion between any formats (OpenAI↔Claude↔Gemini, etc.)

### **2. Kiro’s Dedicated Architecture**

Kiro uses a completely separate implementation:
```javascript
internal/translator/kiro/
├── request.go        # dedicated request translation
├── response.go       # dedicated response translation
├── models.go         # model mapping
└── ...
```

### **3. Why Not Integrate?**

**1. Fundamental API Format Differences**

- **Standard translators**: convert between similar API formats (JSON↔JSON)
- **Kiro**: converts to a completely different internal format

**Grep** "conversationState|userInputMessageContext"
⎿235 lines of output  
The search results show Kiro uses an entirely different API format:

**2. Kiro’s Unique API Format**

```javascript
{
  "conversationState": {
    "currentMessage": {
      "userInputMessage": {
        "content": "...",
        "modelId": "...",
        "origin": "AI_EDITOR",
        "userInputMessageContext": {
          "tools": [/* tools array */]
        }
      }
    },
    "history": [...]
  }
}
```

**Formats handled by standard translators**:
```javascript
{
  "model": "...",
  "messages": [...],
  "tools": [...],
  "temperature": 0.7,
  "max_tokens": 1000
}
```

**3. Functional Differences**

- **Standard translators**: simple field mapping and format conversion
- **Kiro**: complex conversation-state management, history handling, dedicated tool format

**4. Execution Differences**

- **Standard translators**: via the SDK’s generic translator registry
- **Kiro**: dedicated translator invoked directly inside the executor

### Core Components

- **API Server** (`internal/api/`): HTTP server with management endpoints and middleware
- **Authentication** (`internal/auth/`): OAuth flows for Claude, Codex, Gemini, Qwen, iFlow, and token-based auth for Kiro
- **Translators** (`internal/translator/`): Bidirectional API format conversion between providers (Claude↔Gemini, Claude↔OpenAI, Codex↔Claude, etc.)
- **Executors** (`internal/runtime/executor/`): Provider-specific request handlers with streaming support
- **Token Stores** (`internal/store/`): Multiple backends (Postgres, Git, S3-compatible, filesystem) for auth persistence
- **Registry** (`internal/registry/`): Model registration and routing logic

### Supported Providers

- **Claude** (Anthropic OAuth)
- **Codex** (Custom OAuth)
- **Gemini** (Google OAuth + API keys)
- **Qwen** (Alibaba OAuth)
- **iFlow** (Custom OAuth)
- **Kiro** (Token-based auth)
- **OpenAI-compatible** (Custom endpoints like OpenRouter)

### Project Structure

```
cmd/server/            # Application entrypoint
internal/
├── api/              # HTTP server, handlers, middleware
├── auth/             # Provider-specific authentication
├── runtime/executor/ # Request execution engines
├── translator/       # API format translation
├── store/           # Token persistence backends
├── config/          # Configuration management
└── interfaces/      # Shared interfaces and types
tests/
├── unit/            # Unit tests
├── integration/     # End-to-end tests
├── regression/      # Bug regression tests
└── benchmarks/      # Performance benchmarks
sdk/                 # Public Go SDK
```

## Quick Command Reference

```bash
# Build
go build -o cli-proxy-api ./cmd/server

# Run tests
go test ./tests/unit/... ./tests/regression/... -race -cover -v
go test ./tests/unit/kiro -run 'Executor' -v                    # Specific domain
go test -tags=integration ./tests/integration/... -v              # Integration tests
go test ./tests/benchmarks/... -bench . -benchmem -run ^$       # Benchmarks

# Update golden files
go test ./tests/unit/... -run 'SSE|Translation' -v -update

# Start server
./cli-proxy-api --config config.test.yaml
```

## Configuration

Main config file (`config.yaml`):
- **Port**: Server listening port (default: 8317)
- **Auth Directory**: Location for provider auth files (`~/.cli-proxy-api`)
- **API Keys**: Authentication for proxy access
- **Provider Configs**: API keys, OAuth settings, custom endpoints
- **Management API**: Remote management interface (disabled by default)
- **Proxy Settings**: HTTP/SOCKS5 proxy support
- **Quota Management**: Automatic project/model switching on limits
- **Request Logging**: Set `request-log: true` to mirror upstream HTTP payloads into timestamped files under `logs/` for post-mortem debugging.

## API Usage

### Endpoint
```
POST http://localhost:8317/v1/messages
```

### Authentication
Header: `Authorization: Bearer your-api-key`

### Request Format
OpenAI-compatible JSON with provider-specific extensions:
- `thinking`: Claude reasoning configuration
- `tools`: Function calling support
- `stream`: Server-sent events

### Example Request
```json
{
    "model": "claude-sonnet-4-5-20250929",
    "temperature": 0.5,
    "max_tokens": 1024,
    "stream": false,
    "thinking": { "type": "enabled", "budget_tokens": 4096 },
    "system": [
      { "type": "text", "text": "You are Claude Code.", "cache_control": { "type": "ephemeral" } }
    ],
    "tools": [
      {
        "name": "get_weather",
        "description": "Get current weather by city name",
        "input_schema": {
          "type": "object",
          "properties": {
            "city": { "type": "string" },
            "unit": { "type": "string", "enum": ["°C","°F"] }
          },
          "required": ["city"]
        }
      }
    ],
    "messages": [
      {
        "role": "user",
        "content": [{ "type": "text", "text": "Tell me how many degrees now in Tokyo?" }]
      }
    ]
}
```

## Provider Authentication

### OAuth Providers (Claude, Codex, Gemini, Qwen, iFlow)
```bash
./cli-proxy-api --claude-login    # Claude OAuth
./cli-proxy-api --codex-login     # Codex OAuth
./cli-proxy-api --login           # Gemini OAuth
./cli-proxy-api --qwen-login      # Qwen OAuth
./cli-proxy-api --iflow-login     # iFlow OAuth
```

### Kiro Token Auth
Place `kiro-auth-token.json` in auth directory or configure via `kiro-token-file`.

## Development

### Test Data
- **Golden Files**: `tests/shared/golden/` - Expected API responses
- **Test Payloads**: `tests/shared/testdata/` - Sample requests
- **Shared Utils**: `tests/shared/` - Common testing utilities

### Key Files for Changes
- **New Provider**: Add to `internal/auth/`, `internal/runtime/executor/`, `internal/translator/`
- **API Endpoints**: Modify `internal/api/handlers/`
- **Configuration**: Update `internal/config/` and `config.example.yaml`
- **Models Registration**: Update `internal/registry/model_registry.go`

## Deployment

### Environment Variables
- `DEPLOY=cloud`: Cloud deployment mode
- `PGSTORE_*`: PostgreSQL backend configuration
- `GITSTORE_*`: Git backend configuration
- `OBJECTSTORE_*`: S3-compatible backend configuration

### Management API
- **Endpoint**: `/v0/management/*`
- **Authentication**: Requires `secret-key` configuration
- **Features**: Config updates, usage stats, log viewing
- **Control Panel**: Built-in web UI (disable with `disable-control-panel: true`)

### Debug Commands
```bash
# Check configuration
./cli-proxy-api --config config.yaml --debug

# Test authentication
curl -H "Authorization: Bearer test-api-key-1234567890" \
     -H "Content-Type: application/json" \
     -d '{"model":"claude-sonnet-4-5-20250929","messages":[{"role":"user","content":[{"type":"text","text":"test"}]}]}' \
     http://localhost:8317/v1/messages

# View logs
tail -f ~/.cli-proxy-api/logs/*.log
```

### Kiro Debugging Quickstart
When `debug: true`, the Kiro executor emits truncated request/response bodies (`kiro request payload:` / `kiro response payload:`) directly into the main log—perfect for validating translated system prompts and legacy tool chunks without enabling verbose request logging. For full-fidelity captures, flip `request-log: true`, reproduce with `./cli-proxy-api --config config.test.yaml`, and inspect the generated `logs/v1-messages-*.log` file, which now includes the Anthropic-style request as well as the reconstructed `toolUseEvent` stream sent back to the client.
