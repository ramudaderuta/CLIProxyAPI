# CLIProxyAPI - AI Model Proxy Server

## Architecture Overview

CLIProxyAPI is a Go-based HTTP proxy server that provides unified OpenAI/Gemini/Claude-compatible API interfaces for multiple AI providers. It abstracts provider differences and handles authentication, translation between API formats, and request routing.

## Current Work Context (Nov 2025)

- **Kiro ↔ Claude Code parity push**: Sanitized Anthropic payload translation to strip ANSI escapes, stray `<system-reminder>` blocks, and other control bytes before they touch Kiro (`internal/translator/kiro/request.go`). Anthropic responses are likewise scrubbed to remove protocol metadata (e.g. `content-type` fragments) before returning to Claude Code.
- **Tool hygiene + parity**: All Claude Code built-ins (Task, Bash, Grep, Skill, SlashCommand, etc.) pass through to Kiro with descriptions clamped to ≤256 chars (hard limit enforced by Kiro). When a description is truncated we inject a `Tool reference (full descriptions preserved…)` block into the system prompt so Claude still sees the complete instructions. User-defined tools share the same sanitizer.
- **Tool-choice handling**: Anthropic `tool_choice` is translated into `claudeToolChoice` metadata (and a short “Tool directive” sentence in the system prompt) so mandatory tool calls survive the Kiro hop even though the native API doesn’t understand Anthropic’s schema.
- **Anthropic response compliance**: `BuildAnthropicMessagePayload` now generates natural-language lead-ins before a `tool_use`, strips `total_tokens`, and keeps `stop_reason`/`usage` aligned with the current Messages API.
- **Regression artifacts**:
  - Logs demonstrating previous failure and success are archived under `logs/v1-messages-2025-11-08T170438-*.log` and `logs/v1-messages-2025-11-08T175404-*.log`.
  - Repro payload saved at `/tmp/claude_request.json`; filtered variants used for testing live inside `/tmp/claude_request_desc256.json` and `/tmp/claude_request_no_tools.json`.
- **Tests covering the fixes**:
  - `TestParseResponseStripsProtocolNoiseFromContent` and `TestBuildAnthropicMessagePayloadAddsLeadInWhenContentMissing` (anthropic sanitizer).
  - `TestBuildRequestStripsControlCharactersFromUserContent`, `TestBuildRequestPreservesLongToolDescriptions`, `TestBuildRequestStripsMarkupFromToolDescriptions`, and `TestBuildRequestPreservesClaudeCodeBuiltinTools` (request translator hardening).
  - All run via `go test ./tests/unit/kiro -run 'BuildRequest|ParseResponse' -count=1`.

### Outstanding TODOs for Claude Code feature parity

1. **Plan-mode / multi-agent orchestration** – We still treat plan-mode helpers (Task ⇄ ExitPlanMode) as plain tools; no orchestration metadata reaches Kiro. Need to surface plan transitions so Claude Code can spin background agents safely.
2. **Streaming parity** – Kiro still emits raw protocol fragments (`content-type`, chunk headers) during streaming. Map those to Anthropic `content_block_*` events instead of dropping them so Claude Code can render intermediate progress accurately.
3. **Tool-context transport upgrades** – The “Tool reference” block keeps full descriptions accessible, but it bloats the system prompt. Explore richer transports (hash + fetch-on-demand) so we stay under Kiro limits without repeating large texts each turn.

### Solution approach for the Nov 2025 “Improperly formed request” issue

1. **Clamp + mirror tool descriptions**: Kiro refuses payloads with >256-char tool descriptions, so we hard-cap descriptions inside `userInputMessageContext` but append a `Tool reference (full descriptions preserved…)` block to the system prompt containing the original text. This keeps Kiro happy without hiding instructions from Claude Code.
2. **Propagate tool_choice metadata**: When Claude Code sets `tool_choice`, we emit a `claudeToolChoice` map and a short “Tool directive” sentence in the system prompt so Kiro honors mandatory tool invocations.
3. **Regression coverage**: `TestBuildRequestPreservesLongToolDescriptions`, `TestBuildRequestPreservesClaudeCodeBuiltinTools`, and `TestBuildRequestAddsToolReferenceForTruncatedDescriptions` ensure the clamping + mirroring behavior doesn’t regress. Always run `go test ./tests/unit/kiro -run 'BuildRequest|ParseResponse' -count=1` before shipping changes that touch Kiro request translation.

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
