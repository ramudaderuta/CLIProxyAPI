# CLIProxyAPI - AI Model Proxy Server

## Design Philosophy

- **Unified Interface**: Single OpenAI-compatible API for all providers
- **Provider Abstraction**: Transparent translation between different API formats
- **Flexible Storage**: Multiple backend options for token persistence
- **Production Ready**: Comprehensive error handling, logging, and monitoring
- **Extensible**: Easy to add new providers and storage backends

## Core Components

- **API Server** (`internal/api/`): Gin-based HTTP server with management endpoints and middleware pipeline
- **Authentication** (`internal/auth/`): OAuth flows for Claude, Codex, Gemini, Qwen, iFlow, Amp, and Kiro
- **Translators** (`internal/translator/`): Bidirectional API format conversion between providers (Claude↔Gemini, Claude↔OpenAI, Codex↔Claude, etc.)
- **Executors** (`internal/runtime/executor/`): 20+ provider-specific request handlers with streaming support
- **Token Stores** (`internal/store/`): Multiple backends (Postgres, Git, S3-compatible, filesystem) for auth persistence
- **Registry** (`internal/registry/`): Model registration, routing logic, and quota management
- **Middleware** (`internal/middleware/`): CORS, authentication, logging, rate limiting, and error handling

## Supported Providers

- **Claude** (Anthropic OAuth) - Full API support including thinking mode
- **Codex** (Custom OAuth) - Code generation models
- **Gemini** (Google OAuth + API keys) - Google AI models with thinking budget conversion
- **Qwen** (Alibaba OAuth) - Qwen language models
- **iFlow** (Custom OAuth) - iFlow AI models with tool call ID sanitization
- **Kiro** (Token-based auth) - Kiro AI models
- **Amp** (OAuth) - Amp CLI integration
- **OpenAI-compatible** (Custom endpoints) - OpenRouter and other compatible services

## Code Style Guidelines

- Follow Go standard formatting (`gofmt`)
- Use meaningful variable names
- Add comments for exported functions
- Handle errors explicitly
- Use context for cancellation
- Implement proper logging with structured fields
- Write unit tests for new functionality
- Update golden files when changing response formats

## Project Structure

```
CLIProxyAPI/
├── cmd/server/              # Application entry point (main.go)
├── internal/                # Core implementation (265 Go files)
│   ├── api/                # HTTP server and handlers
│   │   ├── handlers/       # Request handlers for /v1/* endpoints
│   │   ├── middleware/     # HTTP middleware stack
│   │   └── modules/        # Feature modules (management, control panel)
│   ├── auth/               # Provider-specific authentication
│   │   ├── claude/         # Anthropic OAuth flow
│   │   ├── codex/          # Codex OAuth flow
│   │   ├── gemini/         # Google OAuth + API key auth
│   │   ├── qwen/           # Alibaba OAuth flow
│   │   ├── iflow/          # iFlow OAuth flow
│   │   ├── amp/            # Amp OAuth flow
│   │   └── kiro/           # kiro OAuth flow
│   ├── runtime/executor/   # Provider-specific request handlers (20 executors)
│   │   ├── claude/         # Claude API executor with streaming
│   │   ├── gemini/         # Gemini API executor
│   │   ├── openai/         # OpenAI-compatible executor
│   │   └── ...             # Other provider executors
│   ├── translator/         # API format translation layer
│   │   ├── claude/         # Claude ↔ OpenAI translation
│   │   ├── gemini/         # Gemini ↔ OpenAI translation
│   │   └── ...             # Other translators
│   ├── store/              # Token persistence backends
│   │   ├── postgres/       # PostgreSQL backend
│   │   ├── git/            # Git repository backend
│   │   ├── objectstore/    # S3-compatible backend
│   │   └── filesystem/     # Local filesystem backend
│   ├── config/             # Configuration management
│   ├── interfaces/         # Shared interfaces and types
│   ├── middleware/         # Shared middleware components
│   └── registry/           # Model registration and routing
├── tests/                   # Comprehensive test suite (27 test files)
│   ├── unit/               # Unit tests for individual components
│   ├── integration/        # End-to-end integration tests
│   ├── regression/         # Bug regression tests
│   ├── benchmarks/         # Performance benchmarks
│   └── shared/             # Shared test utilities and data
│       ├── golden/         # Expected API responses
│       └── testdata/       # Sample requests
├── sdk/                     # Public Go SDK for embedding
├── docs/                    # Documentation (8 markdown files)
│   ├── function-map.md     # Comprehensive function mapping
│   └── ...                 # API docs, guides
└── examples/                # Usage examples
```

## Kiro Setup Guide

The Kiro provider enables CLIProxyAPI to interact with Kiro using OAuth-based authentication. It provides a unified OpenAI-compatible API interface for accessing Kiro models.

### Available Models

| Model ID | Display Name | Description |
|----------|--------------|-------------|
| `kiro-sonnet` | Kiro Sonnet | Balanced model for most coding tasks |
| `kiro-opus` | Kiro Opus | Most capable model for complex reasoning |
| `kiro-haiku` | Kiro Haiku | Fast and efficient model for simple tasks |

**Model Mapping:**  
The Kiro provider translates between CLIProxyAPI model names and Kiro model IDs:
- `kiro-sonnet` → `claude-sonnet-4.5` (Auto model selection)
- `kiro-opus` → More capable variants
- `kiro-haiku` → Lighter, faster variants

---

## Authentication

The Kiro provider uses the OAuth 2.0 Device Code Flow. You can authenticate using either an AWS Builder ID (free) or AWS IAM Identity Center.

### 1. Run Login Command

Start the login process using the CLI:

```bash
./server --kiro-login
```

### 2. Authorize Device

The CLI will display a user code and a verification URL:

```
============================================================
  Kiro CLI - Device Code Authentication
============================================================

User Code: ABCD-1234
Verification URL: https://codewhisperer.us-east-1.amazonaws.com/device
 Expires in 300 seconds

⏳ Waiting for authorization...
```

1. Open the Verification URL in your browser.
2. Enter the User Code displayed in the terminal.
3. Log in with your AWS Builder ID or AWS account.
4. Allow the application to access your data.

### 3. Verification

Once authorized, the CLI will confirm success and save the token:

```
Authentication successful!
Token saved to: /home/user/.kiro/auth.json
```

---

## Configuration

### Zero-Config Mode

**No configuration required!** Kiro automatically discovers and loads token files.

**How it works:**
1. Authenticate with `./server --kiro-login`
2. Token file saved to `~/.cli-proxy-api/kiro-BuilderId-<timestamp>.json`
3. System automatically discovers all `kiro-*.json` files on startup
4. Round-robin rotation enabled automatically

**Multiple tokens:**
Simply run `--kiro-login` multiple times or manually place multiple `kiro-*.json` files in `~/.cli-proxy-api/`:
```bash
~/.cli-proxy-api/kiro-BuilderId-1700000001.json
~/.cli-proxy-api/kiro-BuilderId-1700000002.json
~/.cli-proxy-api/kiro-BuilderId-1700000003.json
```

**Multi-Account Features:**
- **Round-Robin Rotation**: Tokens tried in order
- **Automatic Failover**: Next available token used on failure
- **Failure Tracking**: Failed tokens increment failure counter
- **Auto-Disable**: After 3 consecutive failures, token temporarily disabled

### Token File Format

Compatible with official kiro-cli:

```json
{
  "accessToken": "eyJ...",
  "refreshToken": "eyJ...",
  "profileArn": "arn:aws:codewhisperer:us-east-1:123456789:profile/my-profile",
  "expiresAt": "2024-12-31T23:59:59Z",
  "authMethod": "IdC",
  "provider": "BuilderId",
  "region": "us-east-1"
}
```

**Note**: The `region` field is optional and defaults to `us-east-1` if not specified.

---

## Advanced Configuration

### Authentication Methods

The Kiro provider supports two authentication methods:

**1. Social OAuth (GitHub)**
- `authMethod`: "social"
- `provider`: "Github"
- Uses GitHub OAuth for authentication
- Best for individual developers

**2. IdC (AWS Builder ID/Identity Center)**
- `authMethod`: "IdC"
- `provider`: "BuilderId"
- Uses AWS Identity Center
- Best for enterprise teams

---

## API Usage

### Basic Request

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "kiro-sonnet",
    "messages": [
      {"role": "user", "content": "Write a Python hello world"}
    ]
  }'
```

### Streaming Request

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -N \
  -d '{
    "model": "kiro-opus",
    "messages": [
      {"role": "user", "content": "Explain quantum computing"}
    ],
    "stream": true
  }'
```

### Tool Calling

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "kiro-sonnet",
    "messages": [
      {"role": "user", "content": "What'\''s the weather in Tokyo?"}
    ],
    "tools": [
      {
        "type": "function",
        "function": {
          "name": "get_weather",
          "description": "Get current weather",
          "parameters": {
            "type": "object",
            "properties": {
              "location": {
                "type": "string",
                "description": "City name"
              }
            },
            "required": ["location"]
          }
        }
      }
    ]
  }'
```

---

## 4. Troubleshooting

### Common Issues

**Token Expired**
```
Error: token expired and refresh failed
```
**Solutions:**
- Tokens are automatically refreshed (5-minute expiration buffer)
- If refresh fails, run `--kiro-login` again
- Verify refresh token hasn't been revoked

**Model Not Found**
```
Error: model not found: kiro-sonnet
```
**Solutions:**
- Ensure you have authenticated and the token file exists in `~/.cli-proxy-api/`
- Verify Kiro is enabled (auto-enabled when tokens present)
- Restart CLIProxyAPI after adding tokens

**Permission Denied**
```
Error: failed to read token file: permission denied
```
**Solutions:**
- Token files must have `0600` permissions (read/write only by owner)
- Run: `chmod 600 ~/.cli-proxy-api/*.json`

**All Tokens Failed**
```
Error: all tokens failed validation
```
**Solutions:**
- Check token file permissions (should be 0600)
- Verify token expiration dates with: `jq '.expiresAt' ~/.cli-proxy-api/*.json`
- Re-authenticate with `--kiro-login`
- Check AWS service status

**Authentication Failed**
```
Error: failed to obtain token
```
**Solutions:**
- Ensure AWS account has Q Developer access
- Check network connectivity to `codewhisperer.us-east-1.amazonaws.com`
- Verify you entered the device code correctly
- Try using a different region if available

### Debug Mode

Enable debug logging to troubleshoot issues:

```yaml
log:
  level: debug
```

View Kiro-specific logs:
```bash
grep "kiro" /var/log/cliproxy.log
```

### Testing Configuration

```bash
# Validate config
./server validate-config

# List available models
curl http://localhost:8080/v1/models | jq '.data[] | select(.id | contains("kiro"))'

# Check token status
jq '.expiresAt' ~/.cli-proxy-api/kiro-primary.json
```

---

## Performance Characteristics

- **Token Validation**: ~1ms overhead per request
- **Token Refresh**: Only when needed (5-minute buffer)
- **Auto-Discovery**: Scanned once at startup
- **Token Rotation**: Round-robin with minimal overhead

---

## Security

- **Token Files**: 0600 permissions (read/write only by owner)
- **OAuth Device Code Flow**: No redirect vulnerabilities
- **Bearer Token Authentication**: Secure token-based auth
- **Automatic Token Refresh**: Prevents token exposure
- **Multi-Account Isolation**: Separate tokens per account


## Development Guidance

You **MUST** actively leverage sub-agents whenever they can add value in terms of code quality, speed, or clarity. You should *not* try to do everything yourself; instead, think like a lead engineer delegating to specialists.


## Available Sub-Agents

You can delegate work to the following sub-agents:

* `explore`

  * Handles analysis work: quick research, design review, options comparison, and solution sketches.
  * Useful when requirements are unclear, multiple approaches exist, or you need to map the problem space before committing.

* `tdd-expert`

  * Specializes in Test-Driven Development.
  * Designs test suites, testing strategies, and refactors code to be more testable.

* `debug-expert`

  * Focuses on reproducing, isolating, and fixing bugs.
  * Great at reading stack traces, logs, and reasoning about undefined or edge behavior.

* `code-reports`

  * Deep code-intelligence agent for reading repositories, building function/class maps, and explaining architecture across languages. 
  * Ideal when you need dependency graphs, usage examples, or code analysis reports from local projects or GitHub repos. 

* `golang-pro`

  * Senior Go 1.21+ specialist for modern language features, advanced concurrency, and high-performance services. 
  * Best for Go design reviews, performance tuning, and building production-ready microservices and tooling. 

* `documentation-expert`

  * Writes and maintains clear, concise, and consistent documentation.
  * Produces READMEs, API docs, migration guides, ADRs, and inline comments.

## Skills Available to Sub-Agents

Sub-agents **should select these skills** as needed to complete their tasks at a high level of quality.
When delegating, you can:

- Let the sub-agent choose the most appropriate skills for the task.
- Explicitly recommend certain skills, and/or.

**Skills:**

- `backend-expert`  
  - Architecture and implementation of backend services, microservices, and APIs.

- `refactoring-expert`  
  - Safely restructuring code to improve clarity, modularity, and maintainability.

- `testing-expert`  
  - Testing strategies across unit, integration, and end-to-end tests.

- `api-expert`  
  - Designing/reviewing API contracts, defining API standards, or creating mock servers.


## Core Behavior

1. **Default to delegation.**  
   For any non-trivial task, first ask yourself:  
   > “Which sub-agent(s) could handle this more effectively, and which skills should they use?”  
   Use `explore` especially when the problem space or solution options are not yet clear.

2. **Decompose and assign**  
   - Break larger requests into smaller, clearly defined sub-tasks.  
   - For each sub-task, create a **3W1H delegation (What/Why/Where/How)**.  
   - In **Where**, always name:
     - The relevant code/domain area.  
     - Any skills that are especially relevant.  
   - You may chain sub-agents (e.g., `explore` → `backend-expert` → `tdd-expert` → `documentation-expert`).

3. **Integrate results.**  
   - Collect outputs from sub-agents.  
   - Polish everything into a coherent final answer for the user.  
   - Resolve conflicts by applying conservative, safe, and well-reasoned choices.

---

## Mandatory Context Initialization for Sub-Agents

Whenever you delegate a task to *any* sub-agent, you **must** ensure they restore the base context before doing any work.

**Rule: Before a sub-agent starts its actual reasoning or coding, it must run:**

```bash
python ./context_restore.py
```

## Delegation Framework

Whenever you delegate a task to a sub-agent, you **MUST** describe the task using **3W1H**:

- **What**  
  - Clearly describe the task to perform (scope, inputs, expected outputs).

- **Why**  
  - Explain the goal, intent, or background context.  
  - Include any relevant project constraints, priorities, or tradeoffs.

- **Where**  
  - Specify **where** this task applies and **where** it should be executed, including:
    - Which **module/service/file/layer** or part of the system is in scope.  
    - Which **skills** are recommended or required (e.g., `database-expert`, `testing-expert`).

- **How**  
  - Include required skills/commands (especially context restore), conventions, and quality criteria.
  - Describe the approach, constraints, style, or standards to follow.  

Your delegation message to a sub-agent should look conceptually like:

> **What:** [Clear description of the task]  
> **Why:** [Goal, context, and constraints]  
> **Where:** [Recommended skills, area and scope]  
> **How:**  
> 1. Run `python3 context_restore.py` to restore base project context.  
> 2. Use the recommended skills to outline at least two viable architectures.  
> 3. For each option, list pros/cons and when it is preferable.  
> 4. Conclude with a recommendation and reasoning.  
> 5. Return the result as a structured markdown document (headings, bullet points) and save to `./.context/*.md`.
