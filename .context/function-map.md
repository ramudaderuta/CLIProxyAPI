# CLIProxyAPI Function & Class Map

This document provides a comprehensive mapping of functions, classes, and their locations within the CLIProxyAPI codebase.

## Main Application Functions

| Function | Type | Location | Summary |
|----------|------|----------|---------|
| `main()` | function | `cmd/server/main.go:53` | Application entry point handling command-line flags and service startup |
| `init()` | function | `cmd/server/main.go:43` | Initializes shared logger setup and build info |
| `logging.SetupBaseLogger()` | function | `internal/logging/global_logger.go` | Sets up the base logging configuration |
| `config.LoadConfigOptional()` | function | `internal/config/config.go` | Loads configuration from YAML file with optional validation |

---

## API Server Components

### Server Main Functions

| Function | Type | Location | Summary |
|----------|------|----------|---------|
| `NewServer()` | function | `internal/api/server.go` | Creates a new HTTP server instance with Gin framework |
| `Start()` | function | `internal/api/server.go` | Starts the HTTP server and begins handling requests |
| `Stop()` | function | `internal/api/server.go` | Gracefully stops the HTTP server |
| `WithMiddleware()` | function | `internal/api/server.go:66` | Server option to append additional Gin middleware |
| `WithEngineConfigurator()` | function | `internal/api/server.go:73` | Server option to configure Gin engine |
| `WithRouterConfigurator()` | function | `internal/api/server.go:80` | Server option to configure routes after defaults |

### API Handlers

| Function | Type | Location | Summary |
|----------|------|----------|---------|
| `BaseAPIHandler` | struct | `sdk/api/handlers/` | Base handler providing common API functionality |
| `OpenAIHandler` | struct | `sdk/api/handlers/openai/` | Handles OpenAI-compatible API requests |
| `ClaudeHandler` | struct | `sdk/api/handlers/claude/` | Handles Claude API requests |
| `GeminiHandler` | struct | `sdk/api/handlers/gemini/` | Handles Gemini API requests |
| `ManagementHandler` | struct | `internal/api/handlers/management/` | Handles management API requests |

### Middleware

| Function | Type | Location | Summary |
|----------|------|----------|---------|
| `CORS()` | function | `internal/api/middleware/` | CORS middleware implementation |
| `Authentication()` | function | `internal/api/middleware/` | Authentication middleware for API requests |
| `RequestLogger()` | function | `internal/api/middleware/` | Request logging middleware |
| `RateLimit()` | function | `internal/api/middleware/` | Rate limiting middleware |

---

## Authentication Components

### OAuth Authentication

| Function | Type | Location | Summary |
|----------|------|----------|---------|
| `DoLogin()` | function | `internal/cmd/login.go` | Handles Google/Gemini OAuth login flow |
| `DoCodexLogin()` | function | `internal/cmd/openai_login.go:14` | Handles OpenAI Codex OAuth login |
| `DoClaudeLogin()` | function | `internal/cmd/anthropic_login.go` | Handles Claude OAuth login |
| `DoQwenLogin()` | function | `internal/cmd/qwen_login.go` | Handles Qwen OAuth login |
| `DoIFlowLogin()` | function | `internal/cmd/iflow_login.go` | Handles iFlow OAuth login |
| `DoAntigravityLogin()` | function | `internal/cmd/antigravity_login.go` | Handles Antigravity OAuth login |
| `DoKiroLogin()` | function | `internal/cmd/kiro_login.go` | Handles Kiro OAuth login flow |

### Auth Managers

| Class | Type | Location | Summary |
|-------|------|----------|---------|
| `Manager` | struct | `sdk/auth/manager.go` | Core authentication manager handling token lifecycle |
| `FileTokenStore` | struct | `sdk/auth/filestore.go` | File-based token storage implementation |
| `PostgresTokenStore` | struct | `internal/store/postgresstore.go` | PostgreSQL-based token storage |
| `GitTokenStore` | struct | `internal/store/gitstore.go:25` | Git-based token storage for distributed setups |
| `ObjectTokenStore` | struct | `internal/store/objectstore.go:25` | S3-compatible object storage for tokens |

### Provider-Specific Auth

| Function | Type | Location | Summary |
|----------|------|----------|---------|
| `NewCodexAuth()` | function | `internal/auth/codex/openai_auth.go` | Creates OpenAI Codex authentication handler |
| `GeneratePKCECodes()` | function | `internal/auth/codex/pkce.go` | Generates PKCE codes for OAuth flow |
| `NewClaudeAuth()` | function | `internal/auth/claude/` | Creates Claude authentication handler |
| `NewGeminiAuth()` | function | `internal/auth/gemini/` | Creates Gemini authentication handler |
| `NewKiroAuth()` | function | `internal/auth/kiro/` | Creates Kiro authentication handler |

### Kiro Authentication Layer

#### Authenticator (`internal/auth/kiro/auth.go`)

| Function | Type | Location | Summary |
|----------|------|----------|------------|
| `NewKiroAuthenticator()` | function | `internal/auth/kiro/auth.go:34` | Creates new KiroAuthenticator instance |
| `Authenticate()` | method | `internal/auth/kiro/auth.go:55` | Initiates OAuth device code flow |
| `RefreshToken()` | method | `internal/auth/kiro/auth.go:87` | Refreshes expired access token using refresh token |
| `ValidateToken()` | method | `internal/auth/kiro/auth.go:119` | Validates token and auto-refreshes if needed |
| `GetAuthenticatedClient()` | method | `internal/auth/kiro/auth.go:156` | Creates HTTP client with token authentication |

#### OAuth Device Code Flow (`internal/auth/kiro/oauth.go`)

| Function | Type | Location | Summary |
|----------|------|----------|------------|
| `NewDeviceCodeFlow()` | function | `internal/auth/kiro/oauth.go:61` | Creates device code flow handler |
| `StartDeviceFlow()` | method | `internal/auth/kiro/oauth.go:89` | Requests device code and user verification URL |
| `PollForToken()` | method | `internal/auth/kiro/oauth.go:151` | Polls token endpoint until user authorizes |
| `RefreshToken()` | method | `internal/auth/kiro/oauth.go:276` | Exchanges refresh token for new access token |
| `RegisterClient()` | function | `internal/auth/kiro/oauth.go` | Registers public client with AWS SSO OIDC |

#### Client Cache (`internal/auth/kiro/client_cache.go`)

| Function | Type | Location | Summary |
|----------|------|----------|------------|
| `LoadCachedClient()` | function | `internal/auth/kiro/client_cache.go` | Loads registered client from disk cache |
| `SaveCachedClient()` | function | `internal/auth/kiro/client_cache.go` | Saves registered client to disk cache |

#### Token Manager (`internal/auth/kiro/token_manager.go`)

| Function | Type | Location | Summary |
|----------|------|----------|------------|
| `NewTokenManager()` | function | `internal/auth/kiro/token_manager.go:35` | Creates token manager for rotation |
| `LoadTokens()` | method | `internal/auth/kiro/token_manager.go:45` | Loads and auto-discovers token files |
| `GetNextToken()` | method | `internal/auth/kiro/token_manager.go:133` | Returns next token in round-robin rotation |
| `GetTokenCount()` | method | `internal/auth/kiro/token_manager.go:194` | Returns total token count |
| `GetActiveTokenCount()` | method | `internal/auth/kiro/token_manager.go:201` | Returns non-disabled token count |
| `ResetFailures()` | method | `internal/auth/kiro/token_manager.go:215` | Resets failure counters |
| `GetTokenStats()` | method | `internal/auth/kiro/token_manager.go:230` | Returns token usage statistics |

#### Token Storage (`internal/auth/kiro/token_store.go`)

| Function | Type | Location | Summary |
|----------|------|----------|------------|
| `IsExpired()` | method | `internal/auth/kiro/token_store.go:47` | Checks if token expired with buffer |
| `SaveTokenToFile()` | method | `internal/auth/kiro/token_store.go:65` | Persists token to JSON file (0600 perms) |
| `LoadTokenFromFile()` | function | `internal/auth/kiro/token_store.go:105` | Loads token from JSON file |

---

## Provider Executors

| Class | Type | Location | Summary |
|-------|------|----------|---------|
| `CodexExecutor` | struct | `internal/runtime/executor/codex_executor.go` | Executes OpenAI Codex API requests |
| `ClaudeExecutor` | struct | `internal/runtime/executor/claude_executor.go` | Executes Claude API requests |
| `GeminiExecutor` | struct | `internal/runtime/executor/gemini_executor.go` | Executes Gemini API requests |
| `QwenExecutor` | struct | `internal/runtime/executor/qwen_executor.go` | Executes Qwen API requests |
| `IFlowExecutor` | struct | `internal/runtime/executor/iflow_executor.go` | Executes iFlow API requests |
| `AntigravityExecutor` | struct | `internal/runtime/executor/antigravity_executor.go` | Executes Antigravity API requests |
| `KiroExecutor` | struct | `internal/runtime/executor/kiro_executor.go` | Executes Kiro API requests; translates responses back to the caller’s source format (OpenAI Chat, Claude Messages, OpenAI Responses) and handles streaming conversions |

### Executor Interface Methods

| Function | Type | Location | Summary |
|----------|------|----------|---------|
| `Identifier()` | method | Multiple executor files | Returns unique executor identifier |
| `PrepareRequest()` | method | Multiple executor files | Prepares HTTP request for execution |
| `Execute()` | method | Multiple executor files | Executes the prepared request |
| `HandleResponse()` | method | Multiple executor files | Processes and transforms response |

---

## Storage Components

### Token Storage

| Function | Type | Location | Summary |
|----------|------|----------|---------|
| `NewPostgresStore()` | function | `internal/store/postgresstore.go:25` | Creates PostgreSQL-backed token store |
| `NewGitTokenStore()` | function | `internal/store/gitstore.go:13` | Creates Git-backed token store |
| `NewObjectTokenStore()` | function | `internal/store/objectstore.go:25` | Creates S3-backed token store |
| `Bootstrap()` | method | `internal/store/postgresstore.go` | Initializes storage with default configuration |

### Store Operations

| Function | Type | Location | Summary |
|----------|------|----------|---------|
| `SaveToken()` | function | Multiple store implementations | Saves authentication token |
| `LoadToken()` | function | Multiple store implementations | Loads authentication token |
| `DeleteToken()` | function | Multiple store implementations | Deletes authentication token |
| `ListTokens()` | function | Multiple store implementations | Lists all stored tokens |

---

## Translation Components

### Request/Response Translation

| Function | Type | Location | Summary |
|----------|------|----------|---------|
| `TranslateRequest()` | function | `internal/translator/` | Transforms request between API formats |
| `TranslateResponse()` | function | `internal/translator/` | Transforms response to standard format |
| `TranslateStreamingResponse()` | function | `internal/translator/` | Handles streaming response translation |

### Provider-Specific Translators

| Function | Type | Location | Summary |
|----------|------|----------|---------|
| `OpenAITranslator` | struct | `internal/translator/openai/` | OpenAI API format translator |
| `ClaudeTranslator` | struct | `internal/translator/claude/` | Claude API format translator |
| `GeminiTranslator` | struct | `internal/translator/gemini/` | Gemini API format translator |
| `CodexTranslator` | struct | `internal/translator/codex/` | Codex API format translator |
| `KiroTranslator` | struct | `internal/translator/kiro/` | Kiro API format translator |

### Kiro Translation Layer

#### OpenAI Format (`internal/translator/kiro/openai/`)

| Function | Type | Location | Summary |
|----------|------|----------|---------|
| `ConvertOpenAIRequestToKiro()` | function | `internal/translator/kiro/openai/chat-completions/kiro_openai_request.go:25` | Converts OpenAI request to Kiro conversationState |
| `ConvertKiroResponseToOpenAI()` | function | `internal/translator/kiro/openai/responses/kiro_openai_response.go:22` | Converts Kiro response to OpenAI format |
| `ConvertKiroStreamChunkToOpenAI()` | function | `internal/translator/kiro/openai/responses/kiro_openai_response.go:172` | Converts Kiro SSE chunk to OpenAI format |

#### Claude Format (`internal/translator/kiro/claude/`)

| Function | Type | Location | Summary |
|----------|------|----------|---------|
| `ConvertClaudeRequestToKiro()` | function | `internal/translator/kiro/claude/chat-completions/` | Converts Claude Messages API to Kiro format |
| `ConvertKiroResponseToClaude()` | function | `internal/translator/kiro/claude/responses/` | Converts Kiro response to Claude format |

#### Gemini Format (`internal/translator/kiro/gemini/`)

| Function | Type | Location | Summary |
|----------|------|----------|---------|
| `ConvertGeminiRequestToKiro()` | function | `internal/translator/kiro/gemini/chat-completions/` | Converts Gemini generateContent to Kiro format |
| `ConvertKiroResponseToGemini()` | function | `internal/translator/kiro/gemini/responses/` | Converts Kiro response to Gemini format |

#### Defensive Helpers (`internal/translator/kiro/helpers/`)

| Function | Type | Location | Summary |
|----------|------|----------|---------|
| `SafeParseJSON()` | function | `internal/translator/kiro/helpers/defensive.go:21` | Safely parses malformed JSON with sanitization |
| `SanitizeToolCallID()` | function | `internal/translator/kiro/helpers/defensive.go:68` | Ensures tool call ID is non-empty, generates UUID if needed |
| `FilterThinkingContent()` | function | `internal/translator/kiro/openai/responses/kiro_openai_response.go:154` | Removes `<thinking>` tags from content |
| `TruncateString()` | function | `internal/translator/kiro/helpers/defensive.go:90` | Truncates string to max length with suffix |
| `ExtractTextFromMultimodal()` | function | `internal/translator/kiro/helpers/defensive.go:108` | Extracts text parts from multimodal content |
| `SafeStringValue()` | function | `internal/translator/kiro/helpers/defensive.go:157` | Safely extracts string from map |
| `SafeInt64Value()` | function | `internal/translator/kiro/helpers/defensive.go:183` | Safely extracts int64 from map |

---

## Configuration Components

### Configuration Management

| Function | Type | Location | Summary |
|----------|------|----------|---------|
| `LoadConfig()` | function | `internal/config/config.go` | Loads configuration from YAML file |
| `SaveConfig()` | function | `internal/config/config.go` | Saves configuration to YAML file |
| `ValidateConfig()` | function | `internal/config/config.go` | Validates configuration values |
| `UpdateConfig()` | function | `internal/config/config.go` | Updates configuration with hot reload |

### Configuration Structures

| Class | Type | Location | Summary |
|-------|------|----------|---------|
| `Config` | struct | `internal/config/config.go:21` | Main application configuration structure |
| `SDKConfig` | struct | `sdk/config/config.go` | SDK-specific configuration |
| `RemoteManagement` | struct | `internal/config/config.go` | Remote management API configuration |

---

## Utility Functions

### String and Data Processing

| Function | Type | Location | Summary |
|----------|------|----------|---------|
| `SetLogLevel()` | function | `internal/util/util.go` | Sets application log level based on config |
| `ResolveAuthDir()` | function | `internal/util/util.go` | Resolves authentication directory path |
| `WritablePath()` | function | `internal/util/util.go` | Returns writable path for current user |
| `SetProxy()` | function | `internal/util/proxy.go` | Configures HTTP client with proxy settings |

### JSON Processing

| Function | Type | Location | Summary |
|----------|------|----------|---------|
| `StripUsageMetadataFromJSON()` | function | `internal/runtime/executor/usage_helpers.go:25` | Removes usage metadata from JSON responses |
| `FilterSSEUsageMetadata()` | function | `internal/runtime/executor/usage_helpers.go:10` | Filters usage metadata from SSE streams |

### File System Operations

| Function | Type | Location | Summary |
|----------|------|----------|---------|
| `CopyConfigTemplate()` | function | `internal/misc/copy-example-config.go` | Copies example configuration template |
| `EnsureRepository()` | function | `internal/store/gitstore.go` | Ensures Git repository exists and is accessible |

---

## SDK Components

### Core SDK Service

| Class | Type | Location | Summary |
|-------|------|----------|---------|
| `Service` | struct | `sdk/cliproxy/service.go:32` | Main SDK service for embedding proxy functionality |
| `Builder` | struct | `sdk/cliproxy/builder.go` | Builder pattern for service construction |

### SDK Configuration

| Function | Type | Location | Summary |
|----------|------|----------|---------|
| `NewBuilder()` | function | `sdk/cliproxy/builder.go` | Creates new service builder |
| `WithConfig()` | function | `sdk/cliproxy/builder.go` | Sets configuration on builder |
| `WithServerOptions()` | function | `sdk/cliproxy/builder.go` | Sets server options on builder |
| `Build()` | function | `sdk/cliproxy/builder.go` | Builds and returns configured service |

### SDK Authentication

| Class | Type | Location | Summary |
|-------|------|----------|---------|
| `Manager` | struct | `sdk/cliproxy/auth/` | SDK authentication manager |
| `QuotaManager` | struct | `sdk/cliproxy/auth/` | Manages quota limits and cooldowns |

---

## File Watching Components

| Class | Type | Location | Summary |
|-------|------|----------|---------|
| `Watcher` | struct | `internal/watcher/watcher.go` | File system watcher for configuration changes |
| `AuthUpdate` | struct | `internal/watcher/watcher.go` | Represents authentication file updates |

---

## WebSocket Components

| Class | Type | Location | Summary |
|-------|------|----------|---------|
| `Manager` | struct | `internal/wsrelay/manager.go` | WebSocket connection manager |
| `Session` | struct | `internal/wsrelay/session.go` | Individual WebSocket session |
| `Message` | struct | `internal/wsrelay/message.go` | WebSocket message structure |

---

## Specialized Modules

### AMP CLI Integration

| Function | Type | Location | Summary |
|----------|------|----------|---------|
| `RegisterProviderAliases()` | function | `internal/api/modules/amp/routes.go` | Registers AMP-style route aliases |
| `createReverseProxy()` | function | `internal/api/modules/amp/proxy.go` | Creates reverse proxy for AMP upstream |
| `NewStaticSecretSource()` | function | `internal/api/modules/amp/secret.go` | Creates static secret source for authentication |
| `NewMultiSourceSecret()` | function | `internal/api/modules/amp/secret.go` | Creates multi-source secret with caching |

### Model Registry

| Class | Type | Location | Summary |
|-------|------|----------|---------|
| `ModelRegistry` | struct | `internal/registry/model_registry.go` | Registry for available AI models |
| `ModelDefinition` | struct | `internal/registry/model_definitions.go` | Model definition structure |

---

## Statistics Summary

- **Total Functions Mapped**: 90+ key functions and methods
- **Total Classes/Structs**: 48+ major structures
- **Core Modules**: 10 main functional areas
- **File Coverage**: All major Go files in `/internal` and `/sdk` directories
- **Provider Support**: 7 major AI providers (OpenAI, Claude, Gemini, Qwen, iFlow, Antigravity, Kiro)
