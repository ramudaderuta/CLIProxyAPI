# CLIProxyAPI - AI Model Proxy Server

## Architecture Overview

CLIProxyAPI is a Go-based HTTP proxy server that provides unified OpenAI/Gemini/Claude-compatible API interfaces for multiple AI providers. It abstracts provider differences and handles authentication, translation between API formats, and request routing.

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