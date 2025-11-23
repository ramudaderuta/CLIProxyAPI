# Kiro Provider Test Suite Documentation

## Overview

Comprehensive test suite for the Kiro CLI provider implementation, covering unit tests, integration tests, regression tests, and performance benchmarks.

## Test Organization

### Directory Structure

```
tests/
├── shared/              # Shared test utilities (6 files)
│   ├── io.go           # Test data loading from testdata
│   ├── http.go         # HTTP mocking & SSE helpers
│   ├── golden.go       # Golden file testing with -golden flag
│   ├── env.go          # Environment & time utilities
│   ├── payloads.go     # Request/response builders
│   └── test_utils.go   # Kiro-specific fixtures
├── unit/kiro/          # Unit tests (16 files, 100+ tests)
│   ├── kiro_config_test.go                   # Configuration & auto-discovery
│   ├── kiro_core_test.go                     # Auth & token management
│   ├── kiro_translation_test.go              # OpenAI ↔ Kiro translation
│   ├── kiro_claude_translator_test.go        # Claude format translation
│   ├── kiro_gemini_translator_test.go        # Gemini format translation
│   ├── kiro_sse_formatting_test.go           # SSE event formatting
│   ├── kiro_executor_test.go                 # Executor logic
│   ├── kiro_response_test.go                 # Response handling
│   ├── kiro_hard_request_test.go             # Edge cases
│   ├── kiro_helpers_test.go                  # Helper functions
│   ├── kiro_oauth_flow_test.go               # OAuth device code flow
│   ├── kiro_token_manager_rotation_test.go   # Token rotation & failover
│   ├── kiro_network_errors_test.go           # Network error handling
│   ├── kiro_concurrency_test.go              # Concurrency & edge cases
│   ├── kiro_fallback_test.go                 # 3-level fallback mechanism
│   └── kiro_stream_decoder_test.go           # Amazon event-stream decoder
├── integration/kiro/   # Integration tests (5 files, 9+ tests)
│   ├── kiro_comprehensive_test.go            # End-to-end flows
│   ├── kiro_smoke_test.go                    # Basic smoke tests
│   ├── kiro_executor_integration_test.go     # Full executor flow
│   ├── kiro_sse_integration_test.go          # SSE streaming
│   └── kiro_translation_integration_test.go  # Translation flows
├── regression/kiro/    # Regression tests (2 files, 9+ tests)
│   ├── kiro_thinking_truncation_test.go      # Thinking tag removal
│   └── kiro_sse_buffer_test.go               # SSE buffer limits
└── benchmarks/kiro/    # Performance benchmarks (1 file, 5+ benches)
    └── kiro_executor_benchmark_test.go       # Performance tests

```


## Running Tests

### Quick Start

```bash
# All unit tests
go test ./tests/unit/kiro/

# All tests (short mode, skips slow tests)
go test ./tests/... -short

# With coverage
go test -cover ./tests/unit/kiro/
go test -coverprofile=coverage.out ./tests/unit/kiro/...
go tool cover -html=coverage.out

# With race detection
go test -race ./tests/unit/kiro/...
```

### By Category

```bash
# Unit tests only
go test ./tests/unit/kiro/... -v

# Integration tests
go test -tags=integration ./tests/integration/kiro/... -v

# Regression tests
go test ./tests/regression/kiro/... -v

# Benchmarks
go test -bench=. ./tests/benchmarks/kiro/... -v

# Run specific test file
go test ./tests/unit/kiro/kiro_oauth_flow_test.go -v
```

### Specific Tests

```bash
# Run specific test
go test ./tests/unit/kiro/ -run=TestOAuthFlow

# Run multiple related tests
go test ./tests/unit/kiro/ -run="TestToken"

# Verbose output
go test ./tests/unit/kiro/ -v

# Update golden files
go test ./tests/unit/kiro/ -golden
```

## Test Categories

### Unit Tests (16 files, 100+ tests)

#### kiro_config_test.go
- Configuration structure validation
- Auto-discovery flag behavior
- Multi-account configuration
- Default region handling (us-east-1)
- Token file validation

**Key Tests:**
- `TestKiroConfig` - Config structure validation
- `TestAutoDiscoveryFlag` - Auto-discovery logic
- `TestDefaultRegion` - Default region fallback

#### kiro_core_test.go
- Token storage and loading
- Token validation and expiration
- File permissions (0600 security)
- Token manager initialization
- Refresh flow detection (5-minute buffer)

**Key Tests:**
- `TestTokenStorage` - Save/load operations
- `TestTokenValidation` - Expiration checks
- `TestTokenFilePermissions` - Security validation
- `TestTokenManager` - Manager initialization

#### kiro_translation_test.go
- OpenAI → Kiro request translation
- Kiro → OpenAI response translation
- Tool/function call handling
- Message history processing
- Content sanitization

**Key Tests:**
- `TestOpenAIToKiroTranslation` - Request conversion
- `TestKiroToOpenAITranslation` - Response conversion
- `TestToolTranslation` - Function calling
- `TestMessageHistory` - History handling
- `TestContentSanitization` - Content filtering

#### kiro_claude_translator_test.go
- Claude → Kiro format conversion
- Kiro → Claude response translation
- Claude-specific tool use patterns
- Claude streaming format handling
- Claude safety settings

**Key Tests:**
- `TestClaudeToKiroConversion` - Request translation
- `TestKiroToClaudeConversion` - Response translation
- `TestClaudeToolUseConversion` - Tool handling

#### kiro_gemini_translator_test.go
- Gemini → Kiro format conversion
- Kiro → Gemini response translation
- Gemini function calling
- Gemini streaming format
- Gemini safety settings

**Key Tests:**
- `TestGeminiToKiroConversion` - Request translation
- `TestKiroToGeminiConversion` - Response translation
- `TestGeminiFunctionCallConversion` - Function calls

#### kiro_helpers_test.go
- Safe string extraction
- String truncation utilities
- Content sanitization helpers
- Safe JSON parsing
- String utilities

**Key Tests:**
- `TestSafeGetString` - Safe extraction
- `TestTruncateString` - Truncation logic
- `TestSanitizeContent` - Content cleaning

#### kiro_sse_formatting_test.go
- SSE event formatting (event:, data:)
- Event type handling (message_start, content_block_delta, etc.)
- Buffer management
- Special character encoding
- [DONE] marker handling

**Key Tests:**
- `TestSSEFormatting` - Event structure
- `TestSSEParsing` - Stream parsing
- `TestSSEEventTypes` - All event types
- `TestSSEBuffering` - Buffer handling
- `TestSSEContentEncoding` - Character encoding

#### kiro_executor_test.go
- Request preparation
- Token validation before requests
- Error handling (401, 429, 500)
- Retry logic
- Streaming flag propagation

**Key Tests:**
- `TestExecutorRequestPreparation` - Request setup
- `TestExecutorTokenValidation` - Pre-request validation
- `TestExecutorErrorHandling` - Error scenarios
- `TestExecutorRetryLogic` - Retry behavior

#### kiro_response_test.go
- Response parsing
- Usage metadata extraction
- Error response handling
- Streaming chunk conversion
- Finish reasons (stop, length, tool_calls)

**Key Tests:**
- `TestResponseParsing` - JSON parsing
- `TestUsageMetadata` - Token counting
- `TestStreamingChunkConversion` - SSE chunks
- `TestFinishReasons` - Completion reasons

#### kiro_hard_request_test.go
- Very long messages (10k+ chars)
- Unicode and emoji
- Special characters (XML/HTML)
- Large payloads (1MB)
- Malformed requests
- Tool call edge cases

**Key Tests:**
- `TestHardRequestEdgeCases` - Edge cases
- `TestMalformedRequests` - Invalid input
- `TestLargePayloads` - Performance at scale
- `TestNestedContent` - Complex structures

#### kiro_oauth_flow_test.go
- OAuth device code flow
- Token polling behavior
- Token refresh logic
- OAuth error scenarios
- Timeout handling
- Client configuration

**Key Tests:**
- `TestDeviceCodeFlow` - Device code request/response
- `TestTokenPolling` - Polling with exponential backoff
- `TestTokenRefresh` - Token refresh flow
- `TestOAuthErrors` - Error handling (5 scenarios)
- `TestOAuthTimeout` - Timeout scenarios
- `TestOAuthClientConfiguration` - Client setup

#### kiro_token_manager_rotation_test.go
- Round-robin token selection
- Automatic failover
- Failure count tracking
- Token disable/re-enable logic
- Token statistics
- Auto-discovery
- Concurrent token access

**Key Tests:**
- `TestRoundRobinSelection` - Token rotation
- `TestAutomaticFailover` - Failover logic
- `TestFailureCountTracking` - Failure handling
- `TestTokenDisableAfterFailures` - Disable logic
- `TestTokenManagerStats` - Statistics
- `TestAutoDiscovery` - File discovery
- `TestTokenManagerConcurrency` - Concurrent access

**Coverage Improvement:** Token manager 75% → 95%

#### kiro_network_errors_test.go
- Connection timeouts
- Connection refused scenarios
- Malformed HTTP responses
- Partial SSE streams
- HTTP status codes (400, 401, 403, 404, 429, 500, 502, 503, 504)
- Retry logic with exponential backoff
- Proxy configuration
- Large response handling

**Key Tests:**
- `TestConnectionTimeout` - Timeout scenarios (3 subtests)
- `TestConnectionRefused` - Connection failures (2 subtests)
- `TestMalformedResponse` - Invalid responses (4 subtests)
- `TestPartialSSEStream` - Incomplete streams (6 subtests)
- `TestHTTPStatusCodes` - Status code handling (9 subtests)
- `TestNetworkRetryLogic` - Retry behavior (3 subtests)
- `TestLargeResponseHandling` - Large payloads (2 subtests)

#### kiro_concurrency_test.go
- Concurrent token access (100 parallel requests)
- Token refresh race conditions
- Streaming concurrency
- Large response edge cases (10MB)
- Unicode content handling
- Token file corruption scenarios
- Disk I/O errors
- Race condition detection

**Key Tests:**
- `TestConcurrentTokenAccess` - Parallel access
- `TestTokenRefreshRaceCondition` - Race conditions
- `TestStreamingConcurrency` - Concurrent streams
- `TestLargeResponseEdgeCases` - Large data
- `TestUnicodeContent` - Unicode/emoji (6 scenarios)
- `TestTokenFileCorruption` - File corruption (5 scenarios)
- `TestDiskErrors` - Disk failures
- `TestRaceConditions` - Race detection

#### kiro_fallback_test.go
- 3-level fallback mechanism (Primary → Flattened → Minimal)
- "Improperly formed request" error detection
- History flattening logic
- Minimal request construction
- Request counting and attempt tracking
- Edge case handling (invalid JSON, missing fields)
- Fallback integration scenarios

**Key Tests:**
- `TestPrimaryRequestSuccess` - Primary succeeds without fallback
- `TestFlattenedFallbackSuccess` - Level 2 fallback recovery
- `TestMinimalFallbackSuccess` - Level 3 fallback recovery
- `TestAllFallbacksFail` - All levels exhausted
- `TestIsImproperlyFormedError` - Error detection (8 subtests)
- `TestFlattenHistoryTransformation` - JSON transformation
- `TestFlattenHistoryEdgeCases` - Edge cases (4 subtests)
- `TestBuildMinimalRequest` - Minimal request builder
- `TestBuildMinimalRequestEdgeCases` - Builder edge cases (3 subtests)
- `TestFallbackRequestCounting` - Attempt counting (4 scenarios)
- `TestFallbackIntegrationScenario` - End-to-end integration

**Coverage:** 90-100% for all fallback functions (`attemptRequestWithFallback`, `flattenHistory`, `buildMinimalRequest`, `isImproperlFormedError`)

#### kiro_stream_decoder_test.go
- Amazon event-stream binary format decoding
- Event frame parsing (prelude, headers, payload, CRC)
- Multiple event handling
- CRC validation
- Non-event-stream passthrough
- Malformed stream handling

**Key Tests:**
- `TestStreamDecoder` - Binary format parsing
- `TestEventFrameStructure` - Frame validation
- `TestMultipleEvents` - Multiple frame handling
- `TestCRCValidation` - Checksum verification
- `TestNonEventStream` - Passthrough for regular SSE
- `TestMalformedStream` - Error handling

### Integration Tests (5 files, 9+ tests)

#### kiro_comprehensive_test.go
- Full request/response cycle with mock server
- Non-streaming and streaming flows
- Thinking content filtering
- Token manager integration

**Key Tests:**
- `TestRequestTranslation` - OpenAI to Kiro conversion
- `TestEndToEndNonStreaming` - Full non-streaming flow
- `TestEndToEndStreaming` - Full streaming flow
- `TestTokenManager` - Token management
- `TestKiroPackagesCompile` - Package compilation

#### kiro_executor_integration_test.go
- Complete executor flow
- Multi-account failover
- Token refresh during requests
- Mock Kiro API server

**Key Tests:**
- `TestExecutorIntegration` - End-to-end executor
- `TestMultiAccountFailover` - Account rotation

#### kiro_sse_integration_test.go
- Full SSE streaming flow
- Event ordering validation
- Connection handling
- Client-side parsing
- Large stream handling

**Key Tests:**
- `TestSSEStreamingIntegration` - Complete SSE flow
- `TestSSEEventOrdering` - Event sequence
- `TestSSEBufferSizes` - Various buffer sizes

#### kiro_translation_integration_test.go
- End-to-end translation flows
- Complex conversations with tools
- Multimodal content (text + images)
- Full roundtrip validation

**Key Tests:**
- `TestTranslationIntegration` - Complete translation
- `TestMultimodalTranslation` - Image handling

#### kiro_smoke_test.go
- Basic smoke tests
- Quick sanity checks
- Minimal configuration validation

### Regression Tests (2 files, 9+ tests)

#### kiro_thinking_truncation_test.go
- Thinking tag removal (`<thinking>...</thinking>`)
- Nested thinking tags
- Multiple thinking blocks
- Edge cases (malformed tags, case sensitivity)
- Performance with large content (1000+ tags)

**Key Tests:**
- `TestThinkingTagRemoval` - Tag filtering (7 subtests)
- `TestThinkingTagEdgeCases` - Edge cases (4 subtests)
- `TestThinkingTruncationPerformance` - Large content

#### kiro_sse_buffer_test.go
- SSE buffer limits (1KB to 64KB)
- Buffer overflow handling
- Multiple flush scenarios
- Memory management
- Connection timeout simulation

**Key Tests:**
- `TestSSEBufferLimits` - Various sizes (4 subtests)
- `TestSSEBufferOverflow` - Large events (1MB)
- `TestSSEMemoryLimits` - Long-running streams (1000 events)
- `TestSSEConnectionTimeout` - Partial events
- `TestSSEBufferReset` - Buffer reset

### Benchmarks (1 file, 5+ benchmarks)

#### kiro_executor_benchmark_test.go
- Full request cycle performance
- Token validation overhead
- SSE event processing
- Translation performance (both directions)

**Benchmarks:**
- `BenchmarkExecutorFullCycle` - Complete flow (~755 ns/op)
- `BenchmarkTokenValidation` - Validation speed (~0.37 ns/op)
- `BenchmarkSSEStreaming` - Event processing (~419 ns/op)
- `BenchmarkTranslationOpenAIToKiro` - Request translation (~1474 ns/op)
- `BenchmarkTranslationKiroToOpenAI` - Response translation (~845 ns/op)

## Test Utilities

### Shared Utilities (`tests/shared/`)

#### HTTP Mocking (http.go)
```go
// Create mock server
server := shared.NewMockServer(t, func(w http.ResponseWriter, r *http.Request) {
    // Handle request
})
defer server.Close()

// SSE helper
writer := shared.NewSSEWriter(w)
writer.WriteEvent("message_start", data)
```

#### Test Fixtures (test_utils.go)
```go
// Create test fixtures with token, config, authenticator
fixtures := shared.NewKiroTestFixtures(t)

// Save and load token
fixtures.SaveToken()
loaded := fixtures.LoadToken()

// Use expired token
expiredFixtures := fixtures.WithExpiredToken()
```

#### Golden Files (golden.go)
```go
// Compare with golden file
shared.CompareWithGolden(t, "test_name", actualOutput)

// Update golden files
// go test -golden
```

#### Payload Builders (payloads.go)
```go
// Build OpenAI request
req := shared.BuildOpenAIRequest("kiro-sonnet", messages, streaming)

// Build Kiro response
resp := shared.BuildKiroResponse("response content")

// Build tool definition
tool := shared.BuildToolDefinition("function_name", "description", params)

// Build SSE chunk
chunk := shared.BuildSSEChunk("event_type", data)
```

## Best Practices

### Writing New Tests

1. **Isolation**: Each test should be independent
2. **Cleanup**: Use `t.Cleanup()` for resource cleanup
3. **Deterministic**: Use fixed seeds/times from `shared.FixedTestTime()`
4. **Descriptive**: Clear test and subtest names
5. **Fast**: Unit tests should complete in < 100ms
6. **Documentation**: Add description fields to table-driven tests
7. **Naming**: Follow `kiro_[component]_test.go` convention

### Test Template

```go
func TestFeatureName(t *testing.T) {
    shared.SkipIfShort(t, "feature test requires resources")
    
    fixtures := shared.NewKiroTestFixtures(t)
    
    tests := []struct {
        name        string
        input       interface{}
        expected    interface{}
        description string
    }{
        {
            name:        "test_case_1",
            input:       testInput,
            expected:    expectedOutput,
            description: "What this test validates",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test logic
            
            t.Logf("✓ %s", tt.description)
        })
    }
}
```

### Integration Test Template

```go
//go:build integration

package kiro

import (
    "testing"
    "github.com/router-for-me/CLIProxyAPI/v6/tests/shared"
)

func TestIntegrationFeature(t *testing.T) {
    shared.SkipIfShort(t, "integration test")
    
    // Setup mock server
    server := shared.NewMockServer(t, handler)
    defer server.Close()
    
    // Run integration test
}
```

## Troubleshooting

### Tests Failing

1. Run with `-v` for verbose output
2. Check golden files are up-to-date
3. Verify test data files exist
4. Run with `-race` to detect race conditions

### Race Condition Detection

```bash
go test -race ./tests/unit/kiro/...
```

### Coverage Analysis

```bash
go test -coverprofile=coverage.out ./tests/unit/kiro/...
go tool cover -html=coverage.out
```

## Continuous Integration

Tests run automatically on:
- Pull requests
- Main branch commits
- Release tags

All tests must pass before merge.
