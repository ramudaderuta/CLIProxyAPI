# Kiro AI Provider TDD Test Specification

## Overview

This document provides a comprehensive test specification for fixing the Kiro AI provider response format issues in the Claude Code proxy API. The specification addresses the critical bug where Kiro provider returns incorrect response format with clipped text like ".txt"… Tool usage" tail, and covers all feature areas outlined in the TDD plan.

## Bug Summary

**Critical Issue**: Kiro provider returns malformed responses with content truncated at `.txt"` and artifacts like `}\n\nTool usage` appended.

**Expected Behavior**: Proper content aggregation and tool call formatting without truncation or artifacts.

---

## A. Non-Stream Aggregation Tests

### A1. Text-Only Response Aggregation
**Objective**: Verify proper aggregation of text content chunks without tool calls.

**Acceptance Criteria**:
- Multiple text segments are concatenated correctly
- No content truncation occurs
- Final content matches expected concatenation
- No tool_calls array in response

**Test Cases**:
1. **Simple concatenation**: `text("Hello") + text(" world")` → `"Hello world"`
2. **Unicode content**: Text with emojis and special characters
3. **Empty chunks**: Handle empty or null text gracefully
4. **Large content**: Stress test with large text payloads (>10KB)

### A2. Text Followed by Tool Call
**Objective**: Verify proper separation of text content and tool calls.

**Acceptance Criteria**:
- Text content preserved before tool calls
- Tool calls properly formatted with correct structure
- No mixing of text and tool call data
- Proper JSON serialization of tool arguments

**Test Cases**:
1. **Single tool call**: Text → tool_use → stop
2. **Multiple tool calls**: Text → tool1 → tool2 → stop
3. **Complex arguments**: Tool calls with nested JSON arguments
4. **Special characters**: Tool arguments with quotes, newlines, escape sequences

### A3. Bug Reproduction - Critical Fix Validation
**Objective**: Verify the specific bug with `.txt` truncation is fixed.

**Acceptance Criteria**:
- Content containing `.txt` is NOT truncated
- No `}\n\nTool usage` artifacts appear
- Full text content preserved
- Tool calls properly extracted when present

**Critical Test Scenarios**:
1. **Exact bug reproduction**: Content with `.txt` followed by tool_use mentioning `.txt`
2. **Variations**: Different file extensions (.json, .md, .csv)
3. **Edge cases**: Content ending with various delimiters
4. **Tool usage string**: Content containing the literal string "Tool usage"

**Expected Fix Validation**:
```json
// Input: "I've saved your document.txt with config"
// + tool_use for write_file with path "document.txt"
// Expected: Full content preserved + proper tool_calls
// Bug: ".txt\"}\n\nTool usage" ← MUST NOT OCCUR
```

---

## B. Streaming Interleave & Backpressure Tests

### B1. Text Chunk Streaming
**Objective**: Verify proper SSE formatting and text chunk aggregation.

**Acceptance Criteria**:
- Each text chunk emitted as separate SSE frame
- Proper SSE format with `data: ` prefix
- Terminal `[DONE]` marker present
- No frame coalescing

**Test Cases**:
1. **Multiple small chunks**: "Hello" → ", " → "world" → "!"
2. **Variable chunk sizes**: 1 byte to 1KB chunks
3. **Unicode streaming**: Multi-byte character handling
4. **Empty chunks**: Handle empty data frames gracefully

### B2. Text-Tool Interleaving
**Objective**: Verify proper handling of interleaved text and tool call deltas.

**Acceptance Criteria**:
- Text deltas accumulated correctly
- Tool call deltas with stable IDs
- Arguments merged properly across chunks
- Correct OpenAI tool_call delta format

**Test Cases**:
1. **Simple interleave**: Text → tool_start → arg_delta → text → stop
2. **Complex interleave**: Text → tool1 → tool2 → text → tool1_complete
3. **Argument accumulation**: Multiple argument chunks merged
4. **Multiple tools**: Concurrent tool call streams

### B3. Backpressure Handling
**Objective**: Verify proper flow control with slow clients.

**Acceptance Criteria**:
- Frames flushed per chunk, not coalesced
- No truncation due to buffer limits
- Proper handling of highWaterMark limits
- Stream remains responsive under backpressure

**Test Cases**:
1. **Tiny buffer**: highWaterMark of 64 bytes
2. **Slow reader**: Simulated slow client consumption
3. **Burst handling**: Large chunks with small buffer
4. **Memory efficiency**: No memory leaks under backpressure

---

## C. Tool Loop Support Tests

### C1. Tool Execution Loop
**Objective**: Verify complete tool execution workflow (if supported).

**Acceptance Criteria**:
- Tool calls streamed correctly to client
- Tool results posted back properly
- Kiro continuation handled correctly
- Final response streamed to completion

**Test Cases**:
1. **Single tool loop**: Tool call → result → continuation → completion
2. **Multiple tool loop**: Tool1 → result1 → Tool2 → result2 → completion
3. **Error handling**: Tool execution errors handled gracefully
4. **Timeout handling**: Tool execution timeouts handled

### C2. Unsupported Feature Fallback
**Objective**: Verify graceful handling when tool loops are not supported.

**Acceptance Criteria**:
- Clear normalized error returned
- Error type: `unsupported_provider_feature`
- No clipped text or artifacts
- Consistent error format

**Test Cases**:
1. **Tool call without loop support**: Verify normalized error
2. **Error format validation**: Check canonical error envelope
3. **Client communication**: Error properly transmitted to client

---

## D. Delimiter Safety Tests

### D1. Special Character Handling
**Objective**: Verify content with special delimiters is preserved.

**Acceptance Criteria**:
- Content containing `}` preserved completely
- Content containing "Tool usage" string preserved
- No static stop-list behavior
- No truncation at delimiter boundaries

**Test Cases**:
1. **Brace handling**: Content with embedded JSON structures
2. **Tool usage string**: Literal "Tool usage" in content
3. **Mixed delimiters**: Content with multiple special patterns
4. **Edge cases**: Content starting/ending with delimiters

### D2. Complex Content Preservation
**Objective**: Verify complex content structures are maintained.

**Acceptance Criteria**:
- JSON structures in content preserved
- Code blocks with special characters preserved
- Multi-line content with delimiters preserved
- No content transformation or sanitization

**Test Cases**:
1. **JSON in content**: Complex nested JSON structures
2. **Code snippets**: Code with braces and special characters
3. **Mixed content**: Text + JSON + code combinations
4. **Large structures**: Big JSON objects or arrays in content

---

## E. Provider Gating & Model Dispatch Tests

### E1. Authentication Gating
**Objective**: Verify proper Kiro provider access control.

**Acceptance Criteria**:
- No Kiro token → Kiro models absent from model list
- Kiro models filtered by `owned_by:"kiro"`
- Other providers unaffected
- Proper error for unauthorized access

**Test Cases**:
1. **No token scenario**: Verify Kiro models hidden
2. **Valid token scenario**: Verify Kiro models available
3. **Invalid token scenario**: Verify proper error handling
4. **Mixed provider**: Verify other providers work normally

### E2. Model Routing
**Objective**: Verify proper model dispatching for Kiro models.

**Acceptance Criteria**:
- Kiro model IDs routed to Kiro provider
- Non-Kiro models routed to respective providers
- Model ID matching is exact
- No model ID collisions

**Test Cases**:
1. **Kiro model routing**: claude-sonnet-4-5-20250929 → Kiro
2. **Non-Kiro routing**: gpt-4 → OpenAI (or other provider)
3. **Model not found**: Invalid model returns proper error
4. **Provider fallback**: Fallback behavior when provider unavailable

---

## F. Error Normalization Tests

### F1. HTTP Error Mapping
**Objective**: Verify proper error envelope formatting.

**Acceptance Criteria**:
- All upstream errors mapped to canonical format
- Consistent error structure across error types
- Provider information preserved
- Upstream error codes preserved

**Canonical Error Envelope**:
```json
{
  "error": {
    "type": "error_type",
    "code": "error_code",
    "message": "Human readable message",
    "status": 429,
    "provider": "kiro",
    "upstream_code": "rate_limit_exceeded",
    "request_id": "req_123"
  }
}
```

**Test Cases**:
1. **401/403 errors**: Authentication/authorization failures
2. **429 errors**: Rate limiting with retry-after headers
3. **5xx errors**: Server errors with proper mapping
4. **Network errors**: Connection failures, timeouts

### F2. Stream Error Handling
**Objective**: Verify proper error handling during streaming.

**Acceptance Criteria**:
- Mid-stream errors terminate SSE properly
- Single normalized error in stream termination
- No partial or corrupted data
- Proper error event or close stream

**Test Cases**:
1. **Mid-stream failure**: Error after some chunks delivered
2. **Immediate failure**: Error before any chunks
3. **Partial recovery**: Error after partial tool call
4. **Timeout errors**: Stream timeout handling

---

## G. Responses API Parity Tests

### G1. Non-Stream Parity
**Objective**: Verify `/v1/responses` endpoint consistency.

**Acceptance Criteria**:
- Same behavior as `/v1/chat/completions` for non-stream
- Consistent response format
- Same error handling
- Tool calls handled identically

**Test Cases**:
1. **Text-only responses**: Consistent format across endpoints
2. **Tool call responses`: Consistent tool call format
3. **Error responses**: Consistent error format
4. **Model routing`: Consistent model dispatch

### G2. Stream Parity
**Objective**: Verify streaming consistency across endpoints.

**Acceptance Criteria**:
- Same SSE format across endpoints
- Consistent chunking behavior
- Same tool call streaming format
- Same termination behavior

**Test Cases**:
1. **Text streaming`: Consistent chunking across endpoints
2. **Tool call streaming`: Consistent tool call format
3. **Error streaming`: Consistent error handling
4. **Performance`: Similar latency/throughput

### G3. Unsupported Feature Handling
**Objective**: Verify graceful handling when `/v1/responses` not supported.

**Acceptance Criteria**:
- Clear `unsupported_provider_feature` error
- Consistent error format
- No partial functionality
- Clear documentation of limitations

---

## Edge Cases and Boundary Conditions

### Content Edge Cases
1. **Empty responses**: Completely empty content
2. **Maximum content**: Content at token limits
3. **Binary data**: Binary or non-UTF8 content
4. **Malformed content**: Invalid UTF8 sequences

### Tool Call Edge Cases
1. **Empty tool arguments**: Tool calls with no arguments
2. **Large arguments**: Tool arguments at size limits
3. **Nested arguments**: Deeply nested JSON structures
4. **Special characters**: Arguments with quotes, newlines, etc.

### Streaming Edge Cases
1. **Single character chunks**: Minimal chunk sizes
2. **Large chunks**: Maximum chunk sizes
3. **Mixed chunk types**: Text, tool, error chunks mixed
4. **Connection drops**: Handling client disconnections

### Performance Edge Cases
1. **High frequency**: Rapid successive requests
2. **Concurrent requests**: Multiple simultaneous requests
3. **Memory pressure**: Requests under memory constraints
4. **Network latency**: High latency scenarios

---

## Performance and Backpressure Tests

### Performance Benchmarks
1. **Latency**: Response time measurements
2. **Throughput**: Requests per second capability
3. **Memory usage**: Memory consumption patterns
4. **Scalability**: Behavior under increasing load

### Backpressure Scenarios
1. **Slow consumers**: Clients reading data slowly
2. **Buffer overflow**: Exceeding buffer limits
3. **Memory constraints**: Limited memory environments
4. **Network congestion**: Simulated network issues

### Stress Testing
1. **Sustained load**: Long-running high load
2. **Burst traffic**: Sudden traffic spikes
3. **Resource exhaustion**: Testing failure modes
4. **Recovery testing**: Behavior after stress conditions

---

## Integration Test Scenarios

### End-to-End Workflows
1. **Complete chat flow**: Authentication → request → response
2. **Tool execution flow**: Tool call → execution → result → continuation
3. **Error recovery flow**: Error → retry → success
4. **Multi-turn conversation**: Context preservation across turns

### Provider Integration
1. **Kiro provider integration**: Full Kiro workflow testing
2. **Multi-provider scenarios**: Kiro + other providers
3. **Provider failover**: Fallback behavior
4. **Provider isolation**: Ensuring providers don't interfere

### API Integration
1. **REST API integration**: Full HTTP request/response cycle
2. **WebSocket integration**: Real-time communication testing
3. **Authentication integration**: Token-based auth testing
4. **Rate limiting integration**: API rate limiting testing

---

## Critical Bug Fix Validation

### Primary Bug Scenario
**Test Case**: Exact reproduction of reported bug
```json
Input: Content containing ".txt" followed by tool_use
Expected: Full content preserved + proper tool_calls
Bug Output: ".txt\"}\n\nTool usage" ← MUST BE FIXED
```

### Validation Criteria
1. **Content Preservation**: Full text content preserved without truncation
2. **Artifact Elimination**: No `}\n\nTool usage` or similar artifacts
3. **Tool Call Integrity**: Tool calls properly formatted and complete
4. **Consistent Behavior**: Same content handled correctly across all scenarios

### Regression Prevention
1. **Automated testing**: Comprehensive test suite covering all scenarios
2. **Continuous validation**: Automated checks on each commit
3. **Performance monitoring**: Monitor for similar issues in production
4. **Error tracking**: Proactive detection of response format issues

---

## Test Implementation Guidelines

### Test Structure
1. **Unit tests**: Individual component testing
2. **Integration tests**: Component interaction testing
3. **End-to-end tests**: Complete workflow testing
4. **Performance tests**: Load and stress testing

### Test Data Management
1. **Golden fixtures**: Expected response fixtures for all scenarios
2. **Bug reproduction fixtures**: Specific fixtures for known bugs
3. **Edge case fixtures**: Fixtures for boundary conditions
4. **Performance data**: Baseline performance metrics

### Test Automation
1. **CI/CD integration**: Automated test execution
2. **Regression testing**: Automated regression detection
3. **Performance monitoring**: Continuous performance validation
4. **Coverage reporting**: Test coverage metrics and goals

### Test Environment
1. **Isolated testing**: Independent test environments
2. **Mock services**: Mock external dependencies
3. **Data cleanup**: Proper test data cleanup
4. **Environment parity**: Test environments match production

---

## Success Metrics

### Functional Metrics
1. **Bug fix validation**: Critical bug resolved and stays resolved
2. **Feature completeness**: All TDD plan features implemented
3. **Error handling**: All error scenarios handled properly
4. **API consistency**: Consistent behavior across all endpoints

### Quality Metrics
1. **Test coverage**: >95% code coverage for new features
2. **Performance**: No performance regression from fixes
3. **Reliability**: <0.1% error rate in normal operation
4. **Compatibility**: No breaking changes to existing functionality

### Operational Metrics
1. **Deployment success**: Smooth deployment without issues
2. **Monitoring**: Proper observability and alerting
3. **Documentation**: Complete documentation for changes
4. **Maintainability**: Code remains maintainable and extensible

---

## Conclusion

This comprehensive test specification addresses all aspects of the Kiro AI provider TDD plan, with special focus on the critical response format bug. The specification provides detailed acceptance criteria, test scenarios, and validation procedures to ensure robust, reliable, and performant Kiro provider integration.

Implementation of these tests will ensure that the critical bug is fixed, regression is prevented, and all features work correctly under various conditions and edge cases.