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
- **Translator runtime guarantees (Kiro request)**:
  - The active current turn is always a user message with non-empty text. If the last user turn is tool_result-only or sanitizes to empty/whitespace, we synthesize `"."` and move the tool_result into history so Kiro never sees a tool_result in the active turn.
  - Assistant turns that contain only `tool_use` get a minimal `"."` text placeholder.
  - Trailing assistant `tool_use` blocks are attached to the upcoming current user turn (`toolUses`) only when they precede the final user text; when the transcript ends with a tool_result, we preserve the assistant tool_use in history and insert a text-only assistant placeholder so a non-tool message always follows that tool_result.
  - History user messages that carry only `toolResults` also receive a `"."` placeholder to satisfy Kiro’s non-empty-content requirement in complex transcripts.
  - When the request omits `tools` but messages reference tool names via `tool_use`, we synthesize minimal tool specifications for those names (generic object schema, short description) so Kiro accepts the referenced tools.
- **Kiro executor fallback handling**:
  - `performCompletion` now retries “Improperly formed request” failures with progressively more conservative payloads. Attempt order:
    1. **Primary**: translator output (full history, structured tool events).
    2. **Flattened**: `BuildFlattenedKiroRequest` collapses the full transcript (system, user, assistant, tool events) into a single plain-text block plus a note indicating the flattening, while still passing through tools/plan metadata.
    3. **Minimal**: `BuildMinimalKiroRequest` sends a short “continue previous task” summary with no history when both prior attempts fail.
  - Each attempt is logged (`sending <variant> kiro request attempt`, `retrying … due to Improperly formed request`) so operators can confirm which payload succeeded.
  - These fallbacks mirror the Nov’25 CLIProxy recordings and now keep both non-stream and streaming repros alive even when the upstream service rejects richly structured transcripts.
- **iFlow TodoWrite loop fix**: The repeating `TodoWrite` invocations reported in Nov ’25 were traced to iFlow requests losing the operator-mandated `X-IFlow-Task-Directive` header when passing through the proxy. Commit `0c115df` adds `headers:` to config entries, mirrors them into auth `header:*` attributes, and calls `util.ApplyCustomHeadersFromAttrs` inside every OpenAI-compatible executor (iFlow rides this path). If you change watcher auth synthesis or executor header plumbing, re-run `go test ./tests/unit/iflow -run TestIFlowExecutorForwardsCustomHeaders -count=1` to ensure the directive still survives the hop.
- **Reference parity / captured fixtures**:
  - Recorded SSE runs from CLIProxyAPI live under `tmp/CLIProxyAPI` (checked into this repo). When adjusting streaming logic, grab the matching inputs from `src/claude/claude-kiro.js` fixtures and mirror the event order (tool blocks first, then text).
  - Legacy failure logs still live in `logs/v1-messages-2025-11-08T170438-*.log` (broken) and `logs/v1-messages-2025-11-08T175404-*.log` (fixed). Repro payloads: `/tmp/claude_request.json`, `/tmp/claude_request_desc256.json`, `/tmp/claude_request_no_tools.json`.
- **Tests covering the fixes**:
  - `TestParseResponseStripsProtocolNoiseFromContent` and `TestBuildAnthropicMessagePayloadAddsLeadInWhenContentMissing` (anthropic sanitizer).
  - `TestBuildRequestStripsControlCharactersFromUserContent`, `TestBuildRequestPreservesLongToolDescriptions`, `TestBuildRequestStripsMarkupFromToolDescriptions`, `TestBuildRequestPreservesClaudeCodeBuiltinTools`, and `TestBuildRequestMovesTrailingAssistantMessagesIntoHistory` (request translator hardening).
  - `TestBuildRequest_PreservesToolEventsInHistory` and `TestBuildRequest_EnsuresNonEmptyFinalUserContent` ensure tool_use/tool_result blocks stay structured in history, trailing tool_result-only user messages are moved out of the active turn, and a placeholder `"."` user turn is emitted whenever Kiro would otherwise receive a tool_result message as the final user turn.
  - `TestBuildAnthropicStreamingChunksMatchReference`, `TestConvertKiroStreamToAnthropic_*`, and `TestConvertKiroStreamToAnthropic_StopReasonOverrides` validate the Go SSE output vs. CLIProxyAPI recordings (plain text, tool chains, follow-ups, cancel/timeout).
  - All run via `go test ./tests/unit/kiro -run 'BuildRequest|ParseResponse' -count=1`.
  - Full regression sweep: `go test ./tests/regression/kiro` plus `go test ./...` (latest run passes) before shipping.

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

### Revised understanding of the Nov 2025 “Improperly formed request” issue

After retesting, `tool_use` and `tool_result` in history are broadly acceptable to Kiro. The 400 “Improperly formed request” errors reproduce reliably in only these cases:
- The last user turn ends with a `tool_result` and no trailing user text (`tests/shared/testdata/nonstream/claude_request_todowrite_bad.json`).
- The effective final user turn has empty/whitespace content (e.g., a trailing newline), even if a prior `tool_use` exists (`tests/shared/testdata/nonstream/claude_request_todowrite_bad2.json`).

What we are doing instead:
- Keep `tool_use`/`tool_result` blocks intact in history and current turns. Do not strip or summarize them by default.
- Ensure the final forwarded user turn has non-empty `content`. If the last user message would sanitize to empty or ends with only `tool_result`, synthesize a minimal placeholder (e.g., `"."`) as the current user turn so the request never ends on a tool result.
- Ensure assistant turns that contain only `tool_use` have non-empty `content` by injecting a minimal placeholder (e.g., `"."`).
- For trailing assistants, attach their `tool_use` entries to the current user’s `toolUses` and insert a text-only assistant placeholder after any preceding tool_result; this guarantees a non-tool message follows each tool_result.
- History user tool_result-only entries also get a `"."` placeholder to avoid empty text in complex transcripts.
- When no `tools` are provided but tool_use names are present, synthesize minimal tool specs for those names so Kiro recognizes referenced tools.
- Continue using `safeParseJSON` to defensively parse tool inputs with truncated escapes or control bytes.

Tests to rely on:
- `TestSafeParseJSON_TruncatedJSON` remains valid and protects input parsing.
- The edge cases now live in `tests/unit/kiro/kiro_translation_test.go` and validate that `BuildRequest` produces a valid Kiro request (non-empty current user content) while preserving structured tool events and ordering.

### How to verify locally

- Unit tests: `go test ./tests/unit/kiro -run 'BuildRequest|ParseResponse' -count=1`
- Sanitization tests: `go test ./tests/unit/kiro -run 'TestBuildRequest_StripsToolEventsFromHistory|TestSafeParseJSON_TruncatedJSON' -v`
- Full regression: `go test ./tests/unit/... ./tests/regression/... -race -cover`
- Real replay: start `./cli-proxy-api --config config.test.yaml` and POST
  `tests/shared/testdata/nonstream/claude_request_todowrite_continue.json` to `/v1/messages`.
  - Should now succeed without "Improperly formed request" errors (you should see fallback log messages only on the first attempt; the final SSE delivered to the client matches the Anthropic reference stream).
  - Latest request bodies are logged under `logs/v1-messages-*.log`; verify structured `tool_use`/`tool_result` are preserved in history, assistant tool_use-only turns have a `content` placeholder, and the active current user turn contains non-empty text (".") when needed. If a fallback fires you’ll also see the flattened/minimal payload snapshots inline in the logs.
  - Hard case: POST `tests/shared/testdata/nonstream/test_hard_request.json` (stream=true) and `tests/shared/testdata/streaming/orignal.json`. Expect Anthropic-style SSE with the same event order the reference CLIProxy recordings show; even if the upstream emits the legacy “Improperly formed request” body, the executor should automatically resend and the stream shown to the client must contain the full assistant response.

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
