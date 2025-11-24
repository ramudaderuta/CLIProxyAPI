# CLIProxyAPI - AI Model Proxy Server

## Architecture Overview

CLIProxyAPI is a Go-based HTTP proxy server that provides unified OpenAI/Gemini/Claude-compatible API interfaces for multiple AI providers. It abstracts provider differences and handles authentication, translation between API formats, and request routing.

## Quick Command Reference

### Build & Run

```bash
# Build the server
go build -o cli-proxy-api ./cmd/server

# Build with version info
go build -ldflags "-X main.Version=1.0.0" -o cli-proxy-api ./cmd/server

# Start server with default config
go run ./cmd/server/main.go

# Check configuration
go run ./cmd/server/main.go --config config.yaml --check

# Start with debug logging
go run ./cmd/server/main.go --config config.yaml --debug
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

#### 7. Usage

You can use any OpenAI-compatible client to interact with Kiro models.

**Example Request:**
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

Streaming is fully supported via Server-Sent Events (SSE).

**Example Request:**
```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -N \
  -d '{
    "model": "kiro-sonnet",
    "messages": [{"role": "user", "content": "Count to 10"}],
    "stream": true
  }'
```

Kiro models support tool calling (function calling) compatible with the OpenAI format.

**Example Request:**
```json
{
  "model": "kiro-sonnet",
  "messages": [{"role": "user", "content": "What's the weather in Tokyo?"}],
  "tools": [{
    "type": "function",
    "function": {
      "name": "get_weather",
      "parameters": {
        "type": "object",
        "properties": {
          "location": {"type": "string"}
        }
      }
    }
  }]
}
```

## Current Work Context

- **Kiro Provider**: Implementation complete.
  - OAuth Authentication: ✅
  - Translation Layer: ✅
  - Execution Layer: ✅
  - Tests: ✅
  - Documentation: ✅

## Changelog

All notable changes to this project will be documented in this file.

### Added
- **Kiro Provider**: Added support for Kiro via OAuth.
  - Device code flow authentication (`--kiro-login`).
  - Support for `kiro-sonnet`, `kiro-opus`, and `kiro-haiku` models.
  - Multi-account support with automatic load balancing.
  - Streaming and tool calling support.
