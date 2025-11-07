<!-- OPENSPEC:START -->
# OpenSpec Instructions

These instructions are for AI assistants working in this project.

Always open `@/openspec/AGENTS.md` when the request:
- Mentions planning or proposals (words like proposal, spec, change, plan)
- Introduces new capabilities, breaking changes, architecture shifts, or big performance/security work
- Sounds ambiguous and you need the authoritative spec before coding

Use `@/openspec/AGENTS.md` to learn:
- How to create and apply change proposals
- Spec format and conventions
- Project structure and guidelines

Keep this managed block so 'openspec update' can refresh the instructions.

<!-- OPENSPEC:END -->

## PROJECT GUIDELINE   
  
Here’s an improved and professional version of your **“How to build and start”** section in English 👇

---

## Quick Command Reference

```
# All (excluding integration & benchmarks)
go test ./tests/unit/... ./tests/regression/... -race -cover -v

# Only a domain/family
go test ./tests/unit/kiro -run 'Executor' -v

# Integration
go test -tags=integration ./tests/integration/... -v

# Update goldens
go test ./tests/unit/... -run 'SSE|Translation' -v -update

# Benchmarks
go test ./tests/benchmarks/... -bench . -benchmem -run ^$
```

### How to Build and Start

```bash
# 1. Build the binary
go build -o cli-proxy-api ./cmd/server

# 2. Start the server with a specific configuration file
./cli-proxy-api --config config.test.yaml
```
> [!NOTE]  
> Kiro has been configured in config.test.yaml

#### Request example:

```
POST http://localhost:8317/v1/messages
```

Request body example:

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

> [!NOTE]  
> Request api keys: "test-api-key-1234567890"

#### Stopping the Server

To gracefully stop the server process:

```bash
pkill cli-proxy-api
```


## Recent Changes  
- Kiro AI support via token-based authentication (import kiro-auth-token.json)
- Refactor kiro architecture
> [!NOTE]  
> Original kiro token for testing is located at /home/build/.cli-proxy-api/kiro-auth-token.json