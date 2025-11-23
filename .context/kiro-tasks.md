# Implementation Tasks: Add Kiro CLI Provider

## 1. Authentication Layer Implementation
- [ ] 1.1 Create `internal/auth/kiro/` package structure
- [ ] 1.2 Implement `auth.go` with KiroAuthenticator struct and methods
  - [ ] 1.2.1 Implement `NewKiroAuthenticator()` constructor
  - [ ] 1.2.2 Implement `Authenticate()` for device code flow initiation
  - [ ] 1.2.3 Implement `RefreshToken()` for token refresh (social and IdC)
  - [ ] 1.2.4 Implement `ValidateToken()` for token validation
  - [ ] 1.2.5 Implement `GetAuthenticatedClient()` for HTTP client setup
- [ ] 1.3 Implement `oauth.go` with device code flow
  - [ ] 1.3.1 Implement `DeviceCodeFlow` struct
  - [ ] 1.3.2 Implement `StartDeviceFlow()` to request device code
  - [ ] 1.3.3 Implement `PollForToken()` with exponential backoff
  - [ ] 1.3.4 Handle authorization_pending and slow_down responses
- [ ] 1.4 Implement `token_store.go` with token storage interface
  - [ ] 1.4.1 Define `TokenStore` interface
  - [ ] 1.4.2 Implement `KiroTokenStorage` struct with JSON serialization
  - [ ] 1.4.3 Implement `SaveTokenToFile()` with 0700 permissions
  - [ ] 1.4.4 Implement `LoadTokenFromFile()` with validation
  - [ ] 1.4.5 Implement `IsExpired()` with 5-minute buffer
- [ ] 1.5 Implement `errors.go` with authentication error types
- [ ] 1.6 Write unit tests for authentication layer
  - [ ] 1.6.1 Test device code flow success and failure cases
  - [ ] 1.6.2 Test token refresh for social and IdC methods
  - [ ] 1.6.3 Test token validation and expiration checks
  - [ ] 1.6.4 Test token storage and retrieval
  - [ ] 1.6.5 Test error handling and edge cases

## 2. Translation Layer Implementation
- [ ] 2.1 Create `internal/translator/kiro/` package structure
- [ ] 2.2 Implement OpenAI to Kiro translation
  - [ ] 2.2.1 Create `openai/chat-completions/` subdirectory
  - [ ] 2.2.2 Implement `BuildRequest()` function
  - [ ] 2.2.3 Implement `extractUserMessage()` helper
  - [ ] 2.2.4 Implement `extractAssistantMessage()` helper
  - [ ] 2.2.5 Implement tool definition conversion
  - [ ] 2.2.6 Implement tool call conversion with ID sanitization
  - [ ] 2.2.7 Implement tool result conversion
  - [ ] 2.2.8 Implement multimodal content (images) conversion
  - [ ] 2.2.9 Implement conversation history building
- [ ] 2.3 Implement Kiro to OpenAI translation
  - [ ] 2.3.1 Implement `ParseResponse()` function
  - [ ] 2.3.2 Implement text content extraction
  - [ ] 2.3.3 Implement tool call extraction
  - [ ] 2.3.4 Implement usage metadata extraction
  - [ ] 2.3.5 Implement `FilterThinkingContent()` helper
- [ ] 2.4 Implement streaming translation
  - [ ] 2.4.1 Create `openai/responses/` subdirectory for streaming
  - [ ] 2.4.2 Implement `ConvertKiroStreamToAnthropic()` function
  - [ ] 2.4.3 Implement SSE event parsing
  - [ ] 2.4.4 Implement event type conversion (messageStart, contentBlockDelta, etc.)
  - [ ] 2.4.5 Implement thinking content filtering in streams
  - [ ] 2.4.6 Implement usage metadata filtering from streams
- [ ] 2.5 Implement defensive helpers
  - [ ] 2.5.1 Implement `safeParseJSON()` with escape sequence sanitization
  - [ ] 2.5.2 Implement `SanitizeToolCallID()` with UUID generation
  - [ ] 2.5.3 Implement regex-based thinking content removal
- [ ] 2.6 Implement Claude format support
  - [ ] 2.6.1 Create `claude/` subdirectory
  - [ ] 2.6.2 Implement Anthropic content block conversion
  - [ ] 2.6.3 Implement tool_result conversion
- [ ] 2.7 Implement Gemini format support
  - [ ] 2.7.1 Create `gemini/` subdirectory
  - [ ] 2.7.2 Implement Gemini parts format conversion
  - [ ] 2.7.3 Implement functionCall conversion
- [ ] 2.8 Write unit tests for translation layer
  - [ ] 2.8.1 Test OpenAI to Kiro request translation with golden files
  - [ ] 2.8.2 Test Kiro to OpenAI response translation with golden files
  - [ ] 2.8.3 Test streaming translation with SSE samples
  - [ ] 2.8.4 Test defensive JSON parsing with malformed inputs
  - [ ] 2.8.5 Test tool call ID sanitization
  - [ ] 2.8.6 Test thinking content filtering
  - [ ] 2.8.7 Test Claude and Gemini format conversions

## 3. Executor Implementation
- [ ] 3.1 Create `internal/runtime/executor/kiro_executor.go`
- [ ] 3.2 Implement `KiroExecutor` struct
  - [ ] 3.2.1 Implement `NewKiroExecutor()` constructor
  - [ ] 3.2.2 Implement `Identifier()` returning "kiro"
  - [ ] 3.2.3 Implement `PrepareRequest()` for request setup
- [ ] 3.3 Implement non-streaming execution
  - [ ] 3.3.1 Implement `Execute()` method
  - [ ] 3.3.2 Integrate translation layer (BuildRequest)
  - [ ] 3.3.3 Implement HTTP request construction with auth headers
  - [ ] 3.3.4 Implement response decompression (gzip, brotli, zstd)
  - [ ] 3.3.5 Integrate translation layer (ParseResponse)
  - [ ] 3.3.6 Implement usage tracking and reporting
- [ ] 3.4 Implement streaming execution
  - [ ] 3.4.1 Implement `ExecuteStream()` method
  - [ ] 3.4.2 Implement SSE stream reading
  - [ ] 3.4.3 Integrate streaming translation layer
  - [ ] 3.4.4 Implement channel-based streaming to client
  - [ ] 3.4.5 Implement usage extraction from streams
- [ ] 3.5 Implement token rotation
  - [ ] 3.5.1 Implement `kiroTokenRotator` struct
  - [ ] 3.5.2 Implement `candidates()` for round-robin selection
  - [ ] 3.5.3 Implement `advance()` for cursor movement
  - [ ] 3.5.4 Implement retry logic on "Improperly formed request" error
  - [ ] 3.5.5 Implement token refresh during rotation
- [ ] 3.6 Implement error handling
  - [ ] 3.6.1 Implement status code error handling
  - [ ] 3.6.2 Implement network error handling with retry
  - [ ] 3.6.3 Implement rate limit handling (429)
  - [ ] 3.6.4 Implement timeout and cancellation handling
- [ ] 3.7 Implement request logging
  - [ ] 3.7.1 Implement debug logging for requests
  - [ ] 3.7.2 Implement debug logging for responses
  - [ ] 3.7.3 Implement sensitive data sanitization
  - [ ] 3.7.4 Implement token rotation event logging
- [ ] 3.8 Write unit tests for executor
  - [ ] 3.8.1 Test non-streaming execution with mock HTTP
  - [ ] 3.8.2 Test streaming execution with mock SSE
  - [ ] 3.8.3 Test token rotation logic
  - [ ] 3.8.4 Test error handling and retries
  - [ ] 3.8.5 Test usage tracking
  - [ ] 3.8.6 Test compression handling

## 4. Configuration Integration
- [ ] 4.1 Update `internal/config/config.go`
  - [ ] 4.1.1 Add `KiroConfig` struct definition
  - [ ] 4.1.2 Add `KiroTokenFile` struct definition
  - [ ] 4.1.3 Add Kiro field to main Config struct
  - [ ] 4.1.4 Implement configuration validation for Kiro
- [ ] 4.2 Update configuration loading
  - [ ] 4.2.1 Add Kiro section parsing in LoadConfig
  - [ ] 4.2.2 Implement token file path validation
  - [ ] 4.2.3 Implement token file permission checks
  - [ ] 4.2.4 Implement region validation
- [ ] 4.3 Update configuration hot reload
  - [ ] 4.3.1 Add Kiro configuration reload support
  - [ ] 4.3.2 Add token file watcher integration
- [ ] 4.4 Create example configuration
  - [ ] 4.4.1 Add Kiro section to config.example.yaml
  - [ ] 4.4.2 Document all Kiro configuration options
  - [ ] 4.4.3 Provide multi-account example
- [ ] 4.5 Write configuration tests
  - [ ] 4.5.1 Test configuration parsing
  - [ ] 4.5.2 Test configuration validation
  - [ ] 4.5.3 Test hot reload

## 5. Login Command Implementation
- [ ] 5.1 Create `internal/cmd/kiro_login.go`
- [ ] 5.2 Implement `DoKiroLogin()` function
  - [ ] 5.2.1 Implement license type selection (Pro, Enterprise)
  - [ ] 5.2.2 Implement device code flow initiation
  - [ ] 5.2.3 Implement user code display and verification URI
  - [ ] 5.2.4 Implement token polling with progress indication
  - [ ] 5.2.5 Implement token storage on success
  - [ ] 5.2.6 Implement error handling and user feedback
- [ ] 5.3 Integrate login command with CLI
  - [ ] 5.3.1 Add kiro-login subcommand to main CLI
  - [ ] 5.3.2 Add command-line flags (--region, --label, --output)
  - [ ] 5.3.3 Add help text and usage examples
- [ ] 5.4 Write login command tests
  - [ ] 5.4.1 Test device code flow with mock OAuth server
  - [ ] 5.4.2 Test token storage
  - [ ] 5.4.3 Test error scenarios

## 6. Model Registry Integration
- [ ] 6.1 Update `internal/registry/model_registry.go`
  - [ ] 6.1.1 Register Kiro models (kiro-sonnet, kiro-opus, etc.)
  - [ ] 6.1.2 Define model capabilities (context size, max output)
  - [ ] 6.1.3 Map model names to Kiro provider
- [ ] 6.2 Update model routing logic
  - [ ] 6.2.1 Add Kiro to provider routing
  - [ ] 6.2.2 Implement model name resolution
- [ ] 6.3 Write registry tests
  - [ ] 6.3.1 Test model registration
  - [ ] 6.3.2 Test model lookup
  - [ ] 6.3.3 Test routing to Kiro executor

## 7. Multi-Account Support Integration
- [ ] 7.1 Update account discovery
  - [ ] 7.1.1 Add Kiro account discovery on startup
  - [ ] 7.1.2 Implement account registration with metadata
  - [ ] 7.1.3 Add Kiro to account status display
- [ ] 7.2 Update quota management
  - [ ] 7.2.1 Add Kiro to per-account quota tracking
  - [ ] 7.2.2 Implement quota limit enforcement for Kiro
  - [ ] 7.2.3 Add Kiro to cooldown management
- [ ] 7.3 Update usage statistics
  - [ ] 7.3.1 Add Kiro to usage aggregation
  - [ ] 7.3.2 Add Kiro to usage reporting endpoints
- [ ] 7.4 Write multi-account tests
  - [ ] 7.4.1 Test account discovery
  - [ ] 7.4.2 Test load balancing across Kiro accounts
  - [ ] 7.4.3 Test quota enforcement

## 8. Integration Testing
- [ ] 8.1 Create `tests/integration/kiro/` directory
- [ ] 8.2 Implement end-to-end tests
  - [ ] 8.2.1 Test complete authentication flow
  - [ ] 8.2.2 Test non-streaming chat completion
  - [ ] 8.2.3 Test streaming chat completion
  - [ ] 8.2.4 Test tool calling flow
  - [ ] 8.2.5 Test multimodal requests
  - [ ] 8.2.6 Test multi-account rotation
- [ ] 8.3 Implement mock Kiro API server
  - [ ] 8.3.1 Mock OAuth endpoints
  - [ ] 8.3.2 Mock chat completion endpoints
  - [ ] 8.3.3 Mock streaming endpoints
  - [ ] 8.3.4 Mock error scenarios
- [ ] 8.4 Write integration test suite
  - [ ] 8.4.1 Test with OpenAI-compatible client
  - [ ] 8.4.2 Test with Anthropic-compatible client
  - [ ] 8.4.3 Test error recovery
  - [ ] 8.4.4 Test performance under load

## 9. Documentation
- [ ] 9.1 Update README.md
  - [ ] 9.1.1 Add Kiro to provider list in Overview
  - [ ] 9.1.2 Add Kiro to feature list
  - [ ] 9.1.3 Add Kiro multi-account support mention
- [ ] 9.2 Create Kiro-specific documentation
  - [ ] 9.2.1 Create `docs/kiro-setup.md` with setup instructions
  - [ ] 9.2.2 Document authentication flow
  - [ ] 9.2.3 Document configuration options
  - [ ] 9.2.4 Document multi-account setup
  - [ ] 9.2.5 Document troubleshooting common issues
- [ ] 9.3 Update API documentation
  - [ ] 9.3.1 Document Kiro-specific endpoints (if any)
  - [ ] 9.3.2 Document Kiro model names
  - [ ] 9.3.3 Document Kiro-specific headers or parameters
- [ ] 9.4 Update function map
  - [ ] 9.4.1 Add Kiro functions to `docs/function-map.md`
  - [ ] 9.4.2 Document key classes and methods
- [ ] 9.5 Create usage examples
  - [ ] 9.5.1 Create example configuration file
  - [ ] 9.5.2 Create example API requests
  - [ ] 9.5.3 Create example streaming usage

## 10. Testing and Validation
- [ ] 10.1 Run comprehensive test suite
  - [ ] 10.1.1 Run all unit tests: `go test ./tests/unit/kiro/... -v`
  - [ ] 10.1.2 Run integration tests: `go test -tags=integration ./tests/integration/kiro/... -v`
  - [ ] 10.1.3 Run regression tests to ensure no breakage
  - [ ] 10.1.4 Run benchmarks: `go test ./tests/benchmarks/... -bench . -benchmem`
- [ ] 10.2 Manual testing
  - [ ] 10.2.1 Test login flow with real Kiro OAuth
  - [ ] 10.2.2 Test chat completion with real Kiro API
  - [ ] 10.2.3 Test streaming with real Kiro API
  - [ ] 10.2.4 Test multi-account rotation
  - [ ] 10.2.5 Test error scenarios and recovery
- [ ] 10.3 Performance testing
  - [ ] 10.3.1 Benchmark translation performance
  - [ ] 10.3.2 Benchmark streaming performance
  - [ ] 10.3.3 Profile memory usage
  - [ ] 10.3.4 Test under concurrent load
- [ ] 10.4 Security review
  - [ ] 10.4.1 Review token storage security
  - [ ] 10.4.2 Review sensitive data sanitization in logs
  - [ ] 10.4.3 Review authentication flow security
  - [ ] 10.4.4 Review error message information disclosure

## 11. Code Quality and Cleanup
- [ ] 11.1 Code formatting
  - [ ] 11.1.1 Run `gofmt -w .` on all new files
  - [ ] 11.1.2 Run `go vet ./...` and fix issues
  - [ ] 11.1.3 Run `golangci-lint run` and address warnings
- [ ] 11.2 Code review preparation
  - [ ] 11.2.1 Add comprehensive code comments
  - [ ] 11.2.2 Add package documentation
  - [ ] 11.2.3 Review error messages for clarity
  - [ ] 11.2.4 Review logging statements
- [ ] 11.3 Dependency management
  - [ ] 11.3.1 Run `go mod tidy`
  - [ ] 11.3.2 Verify no unnecessary dependencies added
  - [ ] 11.3.3 Update go.sum if needed

## 12. Release Preparation
- [ ] 12.1 Update version information
  - [ ] 12.1.1 Update CHANGELOG.md with Kiro provider addition
  - [ ] 12.1.2 Document breaking changes (if any)
  - [ ] 12.1.3 Document new features
- [ ] 12.2 Create migration guide
  - [ ] 12.2.1 Document how to enable Kiro provider
  - [ ] 12.2.2 Document configuration migration (if applicable)
  - [ ] 12.2.3 Document authentication setup
- [ ] 12.3 Prepare release notes
  - [ ] 12.3.1 Highlight Kiro provider support
  - [ ] 12.3.2 List key features
  - [ ] 12.3.3 Include usage examples
  - [ ] 12.3.4 Note any limitations or known issues

## Dependencies and Parallelization

### Can be done in parallel:
- Tasks 1 (Authentication), 2 (Translation), and 4 (Configuration) can start simultaneously
- Tasks 6 (Registry) and 7 (Multi-account) can be done in parallel after Task 4
- Task 9 (Documentation) can be written in parallel with implementation

### Sequential dependencies:
- Task 3 (Executor) depends on Tasks 1 and 2
- Task 5 (Login Command) depends on Task 1
- Task 8 (Integration Testing) depends on Tasks 1, 2, 3, 4, 5
- Task 10 (Testing) depends on all implementation tasks
- Task 11 (Cleanup) depends on Task 10
- Task 12 (Release) depends on Task 11

### Critical path:
1 → 2 → 3 → 8 → 10 → 11 → 12
