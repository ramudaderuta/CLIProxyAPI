# CLIProxyAPI - AI Model Proxy Server

## Architecture Overview

CLIProxyAPI is a production-ready Go-based HTTP proxy server that provides unified OpenAI/Gemini/Claude-compatible API interfaces for multiple AI providers. It abstracts provider differences and handles authentication, translation between API formats, request routing, and quota management.

### Design Philosophy

- **Unified Interface**: Single OpenAI-compatible API for all providers
- **Provider Abstraction**: Transparent translation between different API formats
- **Flexible Storage**: Multiple backend options for token persistence
- **Production Ready**: Comprehensive error handling, logging, and monitoring
- **Extensible**: Easy to add new providers and storage backends

### Core Components

- **API Server** (`internal/api/`): Gin-based HTTP server with management endpoints and middleware pipeline
- **Authentication** (`internal/auth/`): OAuth flows for Claude, Codex, Gemini, Qwen, iFlow, Amp, and token-based auth for Kiro
- **Translators** (`internal/translator/`): Bidirectional API format conversion between providers (Claude↔Gemini, Claude↔OpenAI, Codex↔Claude, etc.)
- **Executors** (`internal/runtime/executor/`): 20+ provider-specific request handlers with streaming support
- **Token Stores** (`internal/store/`): Multiple backends (Postgres, Git, S3-compatible, filesystem) for auth persistence
- **Registry** (`internal/registry/`): Model registration, routing logic, and quota management
- **Middleware** (`internal/middleware/`): CORS, authentication, logging, rate limiting, and error handling

### Supported Providers

- **Claude** (Anthropic OAuth) - Full API support including thinking mode
- **Codex** (Custom OAuth) - Code generation models
- **Gemini** (Google OAuth + API keys) - Google AI models with thinking budget conversion
- **Qwen** (Alibaba OAuth) - Qwen language models
- **iFlow** (Custom OAuth) - iFlow AI models with tool call ID sanitization
- **Kiro** (Token-based auth) - Kiro AI models
- **Amp** (OAuth) - Amp CLI integration
- **OpenAI-compatible** (Custom endpoints) - OpenRouter and other compatible services

### Project Structure

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
│   │   └── kiro/           # Token-based authentication
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

## Quick Command Reference

### Build & Run

```bash
# Build the server
go build -o cli-proxy-api ./cmd/server

# Build with version info
go build -ldflags "-X main.Version=1.0.0" -o cli-proxy-api ./cmd/server

# Start server with default config
./cli-proxy-api

# Start with custom config
./cli-proxy-api --config config.yaml

# Start with debug logging
./cli-proxy-api --config config.yaml --debug

# Check configuration
./cli-proxy-api --config config.yaml --check
```

### Testing

```bash
# Run all unit and regression tests
go test ./tests/unit/... ./tests/regression/... -race -cover -v

# Run specific provider tests
go test ./tests/unit/kiro -run 'Executor' -v
go test ./tests/unit/claude -v
go test ./tests/unit/gemini -v

# Run integration tests (requires running server)
go test -tags=integration ./tests/integration/... -v

# Run performance benchmarks
go test ./tests/benchmarks/... -bench . -benchmem -run ^$

# Update golden files (expected test outputs)
go test ./tests/unit/... -run 'SSE|Translation' -v -update

# Run tests with coverage report
go test ./tests/unit/... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Code Quality

```bash
# Format code
gofmt -w .

# Run linter
go vet ./...

# Run static analysis
golangci-lint run

# Check for security issues
gosec ./...
```

### Development

```bash
# Install dependencies
go mod download

# Update dependencies
go mod tidy

# Vendor dependencies
go mod vendor

# Generate mocks (if using mockgen)
go generate ./...
```

### Adding a New Provider

To add support for a new AI provider, follow these steps:

#### 1. Authentication (`internal/auth/newprovider/`)

Create authentication handler:

```go
package newprovider

import (
    "context"
    "github.com/router-for-me/CLIProxyAPI/internal/interfaces"
)

type Authenticator struct {
    clientID     string
    clientSecret string
}

func NewAuthenticator(clientID, clientSecret string) *Authenticator {
    return &Authenticator{
        clientID:     clientID,
        clientSecret: clientSecret,
    }
}

func (a *Authenticator) Authenticate(ctx context.Context) (interfaces.Token, error) {
    // Implement OAuth flow or API key validation
    return token, nil
}

func (a *Authenticator) RefreshToken(ctx context.Context, refreshToken string) (interfaces.Token, error) {
    // Implement token refresh logic
    return newToken, nil
}
```

#### 2. Translator (`internal/translator/newprovider/`)

Create bidirectional translator:

```go
package newprovider

import (
    "github.com/router-for-me/CLIProxyAPI/internal/interfaces"
)

type Translator struct{}

func NewTranslator() *Translator {
    return &Translator{}
}

// OpenAI format → Provider format
func (t *Translator) TranslateRequest(req *interfaces.OpenAIRequest) (*ProviderRequest, error) {
    // Convert request format
    return providerReq, nil
}

// Provider format → OpenAI format
func (t *Translator) TranslateResponse(resp *ProviderResponse) (*interfaces.OpenAIResponse, error) {
    // Convert response format
    return openaiResp, nil
}

// Handle streaming responses
func (t *Translator) TranslateStreamChunk(chunk *ProviderStreamChunk) (*interfaces.OpenAIStreamChunk, error) {
    // Convert streaming chunk
    return openaiChunk, nil
}
```

#### 3. Executor (`internal/runtime/executor/newprovider/`)

Create request executor:

```go
package newprovider

import (
    "context"
    "github.com/router-for-me/CLIProxyAPI/internal/interfaces"
)

type Executor struct {
    baseURL    string
    httpClient *http.Client
    translator *Translator
}

func NewExecutor(baseURL string) *Executor {
    return &Executor{
        baseURL:    baseURL,
        httpClient: &http.Client{Timeout: 60 * time.Second},
        translator: NewTranslator(),
    }
}

func (e *Executor) Execute(ctx context.Context, req *interfaces.OpenAIRequest, token interfaces.Token) (*interfaces.OpenAIResponse, error) {
    // Translate request
    providerReq, err := e.translator.TranslateRequest(req)
    if err != nil {
        return nil, err
    }

    // Execute HTTP request
    resp, err := e.executeHTTP(ctx, providerReq, token)
    if err != nil {
        return nil, err
    }

    // Translate response
    return e.translator.TranslateResponse(resp)
}

func (e *Executor) ExecuteStream(ctx context.Context, req *interfaces.OpenAIRequest, token interfaces.Token, writer io.Writer) error {
    // Implement streaming logic
    return nil
}
```

#### 4. Configuration (`internal/config/config.go`)

Add provider configuration:

```go
type Config struct {
    // ... existing fields

    NewProvider struct {
        Enabled      bool   `yaml:"enabled"`
        ClientID     string `yaml:"oauth-client-id"`
        ClientSecret string `yaml:"oauth-client-secret"`
        BaseURL      string `yaml:"base-url"`
    } `yaml:"newprovider"`
}
```

#### 5. Registry (`internal/registry/model_registry.go`)

Register provider models:

```go
func init() {
    RegisterModel("newprovider-model-1", ModelInfo{
        Provider:     "newprovider",
        DisplayName:  "New Provider Model 1",
        ContextSize:  128000,
        MaxOutput:    4096,
    })
}
```

#### 6. Tests

Create comprehensive tests:

```go
// tests/unit/newprovider/translator_test.go
func TestTranslateRequest(t *testing.T) {
    translator := newprovider.NewTranslator()
    // Test request translation
}

// tests/unit/newprovider/executor_test.go
func TestExecute(t *testing.T) {
    executor := newprovider.NewExecutor("https://api.newprovider.com")
    // Test execution
}
```

### Code Style Guidelines

- Follow Go standard formatting (`gofmt`)
- Use meaningful variable names
- Add comments for exported functions
- Handle errors explicitly
- Use context for cancellation
- Implement proper logging with structured fields
- Write unit tests for new functionality
- Update golden files when changing response formats

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
> 1. Run `python3 ./context_restore.py` to restore base project context.  
> 2. Use the recommended skills to outline at least two viable architectures.  
> 3. For each option, list pros/cons and when it is preferable.  
> 4. Conclude with a recommendation and reasoning.  
> 5. Return the result as a structured markdown document (headings, bullet points) and save to `./.context/*.md`.
