# Kiro Provider Test Architecture Design

## Overview

This document outlines the comprehensive test architecture for the Kiro AI provider, designed to address the critical response format bug and implement the full TDD plan requirements.

## 1. Test Structure & Organization

### Directory Structure
```
./tests/
├── fixtures/kiro/                    # Golden files and test data
│   ├── nonstream/
│   │   ├── text_only.json
│   │   ├── text_then_tool.json
│   │   └── bug_reproduction.json
│   ├── streaming/
│   │   ├── text_chunks.ndjson
│   │   ├── tool_interleave.ndjson
│   │   └── backpressure_test.ndjson
│   └── errors/
│       ├── auth_401.json
│       ├── rate_limit_429.json
│       └── server_error_500.json
├── kiro_unit_test.go                 # Unit tests for translator logic
├── kiro_response_test.go             # Response parsing tests (existing)
├── kiro_streaming_test.go            # Streaming response tests (existing)
├── kiro_tools_test.go                # Tool call handling tests
├── kiro_bug_regression_test.go       # Specific bug reproduction tests
├── kiro_integration_test.go          # End-to-end integration tests
├── kiro_performance_test.go          # Performance and backpressure tests
└── kiro_test_utils.go                # Kiro-specific test utilities
```

### Test File Naming Conventions
- `kiro_[feature]_test.go` - Feature-specific tests
- `kiro_[feature]_integration_test.go` - Integration tests
- `kiro_bug_[description]_test.go` - Bug regression tests
- Test function names: `Test[Feature]_[Scenario]_[ExpectedBehavior]`

## 2. Test Framework & Tools

### Core Testing Stack
- **Go Testing Framework**: Standard `testing` package
- **HTTP Mocking**: Custom `RoundTripperFunc` (existing pattern)
- **JSON Validation**: `gjson` library (already used)
- **Stream Testing**: Custom SSE chunk validation utilities

### External Dependencies
- `github.com/stretchr/testify/assert` - Enhanced assertions
- `github.com/stretchr/testify/require` - Test requirements
- `github.com/gorilla/websocket` - WebSocket testing for streaming
- `github.com/tidwall/gjson` - JSON parsing (already in use)

## 3. Fixtures & Test Data Strategy

### Golden Files Structure

#### Non-Streaming Fixtures
```json
// fixtures/kiro/nonstream/text_only.json
{
  "conversationState": {
    "currentMessage": {
      "assistantResponseMessage": {
        "content": "Hello world"
      }
    }
  }
}
```

```json
// fixtures/kiro/nonstream/text_then_tool.json
{
  "conversationState": {
    "currentMessage": {
      "assistantResponseMessage": {
        "content": "Save file"
      },
      "toolUse": {
        "toolUseId": "t1",
        "name": "write_file",
        "input": {"path": "a.txt"}
      }
    }
  }
}
```

#### Bug Reproduction Fixture
```json
// fixtures/kiro/nonstream/bug_reproduction.json
{
  "conversationState": {
    "currentMessage": {
      "assistantResponseMessage": {
        "content": "Please save the following content to a file named example.txt with this text: Hello world"
      },
      "toolUse": {
        "toolUseId": "t1",
        "name": "write_file",
        "input": {"path": "example.txt", "content": "Hello world"}
      }
    }
  }
}
```

#### Streaming Fixtures
```json
// fixtures/kiro/streaming/text_chunks.ndjson
{"content": "Hello"}
{"content": " world"}
{"stop": true}
```

```json
// fixtures/kiro/streaming/tool_interleave.ndjson
{"content": "Save file"}
{"name": "write_file", "toolUseId": "t1"}
{"input": {"path": "a.txt"}}
{"stop": true}
```

### Test Data Management

#### Authentication Fixtures
```go
// Test token with full permissions
FullAuthToken = &KiroTokenStorage{
    AccessToken:  "test-access-token-full",
    RefreshToken: "test-refresh-token-full",
    ProfileArn:   "arn:aws:codewhisperer:us-west-2:123456789012:profile/full",
    ExpiresAt:    time.Now().Add(30 * time.Minute),
    AuthMethod:   "social",
    Provider:     "Github",
    Type:         "kiro",
}

// Expired token for auth error testing
ExpiredAuthToken = &KiroTokenStorage{
    AccessToken:  "expired-access-token",
    RefreshToken: "expired-refresh-token",
    ExpiresAt:    time.Now().Add(-1 * time.Hour),
    Type:         "kiro",
}
```

## 4. Mock Strategy

### HTTP Mock Server
```go
type KiroMockServer struct {
    server   *httptest.Server
    responses map[string]MockResponse
    delay    time.Duration
    errors   map[string]error
}

type MockResponse struct {
    StatusCode int
    Body       []byte
    Headers    map[string]string
    IsStream   bool
}
```

### Response Simulation
```go
// Simulate different Kiro response scenarios
func (m *KiroMockServer) SetResponse(endpoint string, response MockResponse) {
    m.responses[endpoint] = response
}

// Simulate streaming responses
func (m *KiroMockServer) SetStreamingResponse(endpoint string, chunks [][]byte) {
    // Implementation for SSE streaming simulation
}

// Simulate error conditions
func (m *KiroMockServer) SetError(endpoint string, err error) {
    m.errors[endpoint] = err
}
```

## 5. Test Utilities

### Response Validation Helpers
```go
// ValidateOpenAIResponse ensures response matches OpenAI format
func ValidateOpenAIResponse(t *testing.T, response []byte, expected OpenAIExpected) {
    // Validation logic for OpenAI-compatible responses
}

// ValidateContentPreservation checks for content clipping bugs
func ValidateContentPreservation(t *testing.T, actual, expected string) {
    // Specific validation for the clipping bug
    assert.NotContains(t, actual, ".txt\"}\n\nTool usage",
        "Content should not be clipped with artifact")
    assert.Equal(t, expected, actual, "Content should be preserved exactly")
}

// ValidateToolCalls ensures proper tool call formatting
func ValidateToolCalls(t *testing.T, actual []OpenAIToolCall, expected []OpenAIToolCall) {
    // Tool call validation logic
}
```

### Stream Testing Utilities
```go
// ParseSSEChunks parses server-sent events into structured chunks
func ParseSSEChunks(t *testing.T, data []byte) []SSEChunk {
    // SSE parsing implementation
}

// ValidateStreamOrder ensures proper sequence of streaming chunks
func ValidateStreamOrder(t *testing.T, chunks []SSEChunk, expectedOrder []string) {
    // Stream order validation
}

// SimulateBackpressure simulates slow client conditions
func SimulateBackpressure(t *testing.T, client *http.Client, highWaterMark int) {
    // Backpressure simulation implementation
}
```

### Tool Call Testing Helpers
```go
// CreateTestToolCall generates a test tool call
func CreateTestToolCall(id, name string, args map[string]any) OpenAIToolCall {
    argsBytes, _ := json.Marshal(args)
    return OpenAIToolCall{
        ID:        id,
        Name:      name,
        Arguments: string(argsBytes),
    }
}

// AssertToolCallEquals validates tool call equality
func AssertToolCallEquals(t *testing.T, actual, expected OpenAIToolCall) {
    assert.Equal(t, expected.ID, actual.ID)
    assert.Equal(t, expected.Name, actual.Name)
    assert.JSONEq(t, expected.Arguments, actual.Arguments)
}
```

## 6. Test Categories

### Unit Tests
- **Response Parsing**: `ParseResponse` function with various inputs
- **Tool Call Extraction**: `parseBracketToolCalls`, `parseEventStream` functions
- **Content Sanitization**: `sanitizeJSON`, `normalizeArguments` functions
- **Stream Building**: `BuildStreamingChunks`, `BuildOpenAIChatCompletionPayload`

### Integration Tests
- **End-to-End Workflows**: Complete request/response cycles
- **Authentication Flow**: Token validation and refresh
- **Model Dispatch**: Proper model mapping and routing
- **Error Handling**: Upstream error propagation and normalization

### Performance Tests
- **Latency**: Response time measurements under various loads
- **Throughput**: Concurrent request handling capacity
- **Backpressure**: Stream behavior under slow client conditions
- **Memory Usage**: Resource consumption during large responses

### Regression Tests
- **Bug Reproduction**: Exact scenarios that caused the clipping bug
- **Content Preservation**: Ensure no content truncation occurs
- **Tool Call Formatting**: Validate proper tool call structure
- **Delimiter Safety**: Test special character handling

## 7. Configuration

### Test Environment Variables
```bash
# Test configuration
KIRO_TEST_MODE=mock          # mock, integration, live
KIRO_TEST_TIMEOUT=30s        # Test timeout duration
KIRO_TEST_DEBUG=false        # Enable debug logging
KIRO_TEST_FIXTURES_PATH=./tests/fixtures
```

### Test Configuration Structure
```go
type TestConfig struct {
    Mode        string        // mock, integration, live
    Timeout     time.Duration // Test timeout
    Debug       bool          // Debug logging
    FixturesDir string        // Fixture files directory
    MockServer  *KiroMockServer // Mock server instance
}
```

## 8. Coverage Requirements

### Minimum Coverage Thresholds
- **Line Coverage**: 80%
- **Branch Coverage**: 75%
- **Critical Path Coverage**: 100%

### Critical Paths to Cover
- Content parsing logic (`parseEventStream`, `ParseResponse`)
- Tool call extraction and formatting
- Stream chunk generation
- Error handling and normalization
- Authentication and authorization

## 9. Continuous Integration

### Test Execution Pipeline
1. **Unit Tests**: Fast feedback on code changes
2. **Integration Tests**: Component interaction validation
3. **Performance Tests**: Regression detection for performance
4. **Bug Regression Tests**: Ensure fixed bugs stay fixed

### Coverage Reporting
- Generate coverage reports on each test run
- Fail build if coverage thresholds not met
- Track coverage trends over time

## 10. Test Data Management

### Fixture Versioning
- Version control all test fixtures
- Tag fixture versions with corresponding code releases
- Maintain fixture backward compatibility

### Sensitive Data Handling
- Use mock tokens for testing
- Never commit real authentication tokens
- Sanitize logs to remove sensitive information

This test architecture provides a comprehensive foundation for implementing the TDD plan and ensuring the Kiro provider works correctly with proper content preservation and tool call handling.