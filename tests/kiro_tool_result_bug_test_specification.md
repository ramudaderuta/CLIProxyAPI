# Kiro Translator Tool Result Processing Bug - Test Specification

## Issue Summary

The Kiro translator has a critical bug in tool result processing where it incorrectly parses `tool_result` content from array-based message structures, causing "Improperly formed request" errors from the Kiro API.

### Root Cause Analysis

**Location**: `/internal/translator/kiro/request.go:174-186`

**Problematic Code**:
```go
case "tool_result":
    resultContent := extractNestedContent(part.Get("content"))
    if resultContent == "" {
        resultContent = part.Get("text").String()  // ❌ WRONG FIELD
    }
    toolResults = append(toolResults, map[string]any{
        "content": []map[string]string{{"text": resultContent}},
        "status":  firstString(part.Get("status").String(), "success"),
        "toolUseId": firstString(
            part.Get("tool_use_id").String(),
            part.Get("tool_useId").String(),
        ),
    })
```

**Root Cause**: The code tries `part.Get("text").String()` as fallback, but tool results use the `content` field, not `text`. This causes empty content extraction when `extractNestedContent` fails to parse array-based content structures.

**Why This Causes "Improperly formed request"**:
1. Empty tool results → Kiro API receives malformed tool result context
2. Missing content → API rejects request as improperly formed
3. Silent failure → No logging shows the content extraction issue

## Acceptance Criteria

### AC1: Tool Result Content Extraction
- **GIVEN**: A tool result message with array-based content structure
- **WHEN**: The Kiro translator processes the message
- **THEN**: Tool result content must be correctly extracted regardless of content format (string, array, nested objects)
- **AND**: No empty content should be passed to the Kiro API

### AC2: Backward Compatibility
- **GIVEN**: Existing tool result message formats
- **WHEN**: The Kiro translator processes messages
- **THEN**: All existing message formats must continue to work without regression
- **AND**: Both string and array-based content structures must be supported

### AC3: Error Handling and Logging
- **GIVEN**: Malformed or missing tool result content
- **WHEN**: The Kiro translator attempts extraction
- **THEN**: Appropriate error handling must prevent empty content
- **AND**: Debug logging should indicate content extraction issues
- **AND**: The translator should not crash or produce malformed requests

### AC4: Tool Result Structure Validation
- **GIVEN**: Tool result messages with various content structures
- **WHEN**: Building the Kiro request
- **THEN**: The resulting toolResults array must contain properly structured entries
- **AND**: Each tool result must have non-empty content, valid status, and toolUseId

## Test Scenarios

### TS1: Basic Tool Result Processing

#### TS1.1: String Content Tool Result
```json
{
  "role": "tool",
  "content": "The weather is sunny and 75°F",
  "tool_use_id": "toolu_01abc123"
}
```
**Expected**: Content correctly extracted as "The weather is sunny and 75°F"

#### TS1.2: Array Content Tool Result (Primary Bug Scenario)
```json
{
  "role": "tool",
  "content": [
    {
      "type": "text",
      "text": "The weather is sunny and 75°F"
    }
  ],
  "tool_use_id": "toolu_01abc123"
}
```
**Expected**: Content correctly extracted as "The weather is sunny and 75°F"

#### TS1.3: Nested Array Content Tool Result
```json
{
  "role": "tool",
  "content": [
    {
      "type": "text",
      "text": [
        {"type": "text", "text": "The weather is sunny"}
      ]
    },
    {
      "type": "text",
      "text": " and 75°F"
    }
  ],
  "tool_use_id": "toolu_01abc123"
}
```
**Expected**: Content correctly concatenated as "The weather is sunny and 75°F"

### TS2: Tool Result with Different Status Values

#### TS2.1: Success Status
```json
{
  "role": "tool",
  "content": [{"type": "text", "text": "Operation completed"}],
  "tool_use_id": "toolu_01abc123",
  "status": "success"
}
```
**Expected**: Status preserved as "success"

#### TS2.2: Error Status
```json
{
  "role": "tool",
  "content": [{"type": "text", "text": "API timeout occurred"}],
  "tool_use_id": "toolu_01abc123",
  "status": "error"
}
```
**Expected**: Status preserved as "error"

#### TS2.3: Missing Status (Default to Success)
```json
{
  "role": "tool",
  "content": [{"type": "text", "text": "Operation completed"}],
  "tool_use_id": "toolu_01abc123"
}
```
**Expected**: Status defaults to "success"

### TS3: Tool Use ID Variations

#### TS3.1: Standard tool_use_id
```json
{
  "role": "tool",
  "content": [{"type": "text", "text": "Result"}],
  "tool_use_id": "toolu_01abc123"
}
```
**Expected**: toolUseId correctly extracted as "toolu_01abc123"

#### TS3.2: Alternative tool_useId Field
```json
{
  "role": "tool",
  "content": [{"type": "text", "text": "Result"}],
  "tool_useId": "toolu_01abc123"
}
```
**Expected**: toolUseId correctly extracted as "toolu_01abc123"

#### TS3.3: Both Fields Present (Priority)
```json
{
  "role": "tool",
  "content": [{"type": "text", "text": "Result"}],
  "tool_use_id": "toolu_primary",
  "tool_useId": "toolu_secondary"
}
```
**Expected**: toolUseId uses "toolu_primary" (first non-empty value)

### TS4: Edge Cases and Error Handling

#### TS4.1: Empty Content Array
```json
{
  "role": "tool",
  "content": [],
  "tool_use_id": "toolu_01abc123"
}
```
**Expected**: Graceful handling with empty string content, no crash

#### TS4.2: Missing Content Field
```json
{
  "role": "tool",
  "tool_use_id": "toolu_01abc123"
}
```
**Expected**: Graceful handling with empty string content, no crash

#### TS4.3: Malformed Content Structure
```json
{
  "role": "tool",
  "content": "not an array but has content",
  "tool_use_id": "toolu_01abc123"
}
```
**Expected**: Content extracted as string "not an array but has content"

#### TS4.4: Content with Non-Text Types
```json
{
  "role": "tool",
  "content": [
    {"type": "image", "source": {"data": "base64data"}},
    {"type": "text", "text": "Image processed"}
  ],
  "tool_use_id": "toolu_01abc123"
}
```
**Expected**: Only text content extracted: "Image processed"

### TS5: Integration with Complete Conversation Flow

#### TS5.1: Multi-Turn Conversation with Tool Results
```json
{
  "model": "claude-sonnet-4-5",
  "messages": [
    {"role": "user", "content": "What's the weather in New York?"},
    {"role": "assistant", "content": [
      {"type": "text", "text": "I'll check the weather for you."},
      {"type": "tool_use", "id": "toolu_01abc123", "name": "get_weather", "input": {"location": "New York"}}
    ]},
    {"role": "tool", "content": [
      {"type": "text", "text": "The weather in New York is sunny and 75°F"}
    ], "tool_use_id": "toolu_01abc123"}
  ]
}
```
**Expected**: Complete Kiro request built successfully with proper tool result content

#### TS5.2: Multiple Tool Results in Single Message
```json
{
  "role": "tool",
  "content": [
    {"type": "text", "text": "Weather: 75°F, sunny"},
    {"type": "text", "text": "Humidity: 45%"}
  ],
  "tool_use_id": "toolu_01abc123"
}
```
**Expected**: Content concatenated: "Weather: 75°F, sunny\nHumidity: 45%"

## Test Implementation Structure

### Unit Tests

#### TestExtractUserMessage_ToolResultContent
- Test all content extraction scenarios (TS1.1-TS1.3)
- Verify `extractUserMessage` function correctly handles tool results
- Mock gjson.Result objects with different content structures

#### TestBuildRequest_ToolResultIntegration
- Test complete request building with tool results (TS5.1-TS5.2)
- Verify Kiro request structure contains properly formatted toolResults
- Test with various message histories and tool result formats

#### TestExtractNestedContent_EdgeCases
- Test `extractNestedContent` function with malformed inputs
- Verify graceful handling of null, empty, and invalid structures
- Test recursive content extraction

### Integration Tests

#### TestKiroExecutor_WithToolResults
- End-to-end test using KiroExecutor
- Test with real Kiro token storage
- Verify no "Improperly formed request" errors

#### TestKiroTranslation_CompleteToolFlow
- Complete translation flow: OpenAI → Kiro → OpenAI
- Test with tool calls and tool results
- Verify content preservation throughout the flow

### Regression Tests

#### TestBackwardCompatibility_ExistingFormats
- Test all existing message formats continue to work
- Verify no breaking changes to current functionality
- Test with production-like message structures

## Test Data and Mocks

### Mock Token Storage
```go
token := &authkiro.KiroTokenStorage{
    ProfileArn:  "arn:aws:codewhisperer:us-east-1:699475941385:profile/test",
    AccessToken: "test_access_token",
    ExpiresAt:   time.Now().Add(24 * time.Hour),
    Type:        "kiro",
}
```

### Expected Kiro Tool Result Structure
```go
expectedToolResult := map[string]any{
    "content": []map[string]string{
        {"text": "extracted content here"},
    },
    "status": "success", // or "error"
    "toolUseId": "toolu_01abc123",
}
```

## Success Metrics

1. **All test scenarios pass**: 100% test coverage for tool result processing
2. **No regression**: Existing functionality continues to work
3. **Error reduction**: Zero "Improperly formed request" errors in integration tests
4. **Performance**: No significant performance degradation in request processing
5. **Code quality**: Clean, maintainable code with proper error handling

## Implementation Checklist

- [ ] Fix `extractUserMessage` function to properly handle array-based tool result content
- [ ] Update `extractNestedContent` to handle edge cases gracefully
- [ ] Add comprehensive unit tests for all scenarios
- [ ] Add integration tests for complete flow
- [ ] Add regression tests for backward compatibility
- [ ] Update error handling and logging
- [ ] Verify fix resolves "Improperly formed request" errors
- [ ] Document changes and update test coverage reports

## Risk Mitigation

1. **Backward Compatibility**: Maintain support for all existing message formats
2. **Performance**: Ensure content extraction doesn't impact request processing speed
3. **Error Handling**: Prevent crashes from malformed content structures
4. **Testing**: Comprehensive test coverage to prevent regressions
5. **Monitoring**: Add logging to track content extraction issues in production

This test specification provides a comprehensive framework for validating the fix to the Kiro translator tool result processing bug, ensuring robust handling of all tool result content formats while maintaining backward compatibility and preventing the "Improperly formed request" errors.