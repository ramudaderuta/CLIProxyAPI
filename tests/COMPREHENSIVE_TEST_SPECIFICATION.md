# Comprehensive Test Specification: Kiro Response Parsing and SSE Stream Handling

## Executive Summary

This document provides a comprehensive test specification for the failing Kiro response parsing tests, focusing on SSE stream parsing edge cases, JSON error handling, invalid input formats, and special character preservation. The specification defines acceptance criteria, identifies edge cases, and creates test scenarios to ensure robust response parsing across all input types.

## Test Categories

### 1. SSE Stream Parsing Tests
**Focus Areas**: Server-Sent Events parsing, chunked responses, control character handling

#### 1.1 Basic SSE Stream Parsing
**Test Name**: `TestParseResponseFromEventStream`

**Acceptance Criteria**:
- Must correctly aggregate text content from multiple `data:` lines
- Must properly merge tool call arguments from fragmented chunks
- Must handle tool call completion with `stop: true` markers
- Must maintain order of text chunks and tool calls

**Edge Cases to Test**:
- Empty data lines mixed with valid content
- Malformed JSON within data lines
- Tool calls with fragmented arguments across multiple chunks
- Missing stop markers for tool calls
- Duplicate tool call IDs requiring deduplication

**Test Scenarios**:
```go
// Valid multi-line SSE stream
stream := strings.Join([]string{
    `data: {"content":"Line 1"}`,
    `data: {"content":"Line 2"}`,
    `data: {"name":"lookup","toolUseId":"call-1","input":{"foo":"bar"}}`,
    `data: {"name":"lookup","toolUseId":"call-1","input":{"baz":1},"stop":true}`,
}, "\n")

expectedText := "Line 1Line 2" // Note: spaces may need to be added
expectedToolCalls := []OpenAIToolCall{
    {ID: "call-1", Name: "lookup", Arguments: `{"foo":"bar","baz":1}`},
}
```

#### 1.2 SSE Stream with Control Delimiters
**Test Name**: `TestParseResponseFromEventStreamWithControlDelimiters`

**Acceptance Criteria**:
- Must parse content with control characters (vertical tabs, etc.)
- Must handle mixed control delimiter formats
- Must extract content from malformed-looking SSE streams
- Must ignore metering events and other non-content events

**Edge Cases to Test**:
- Vertical tab characters (`\v`) as delimiters
- Mixed control character sequences
- Embedded metering events that should be ignored
- Control characters within JSON strings vs. as delimiters

**Test Scenarios**:
```go
raw := strings.Join([]string{
    `:message-typeevent{"content":"I don"}`,
    "\v:message-typeevent{\"content\":\"'t have access\"}",
    "\v:message-typeevent{\"content\":\" to data.\"}",
    "\v:metering-event{\"unit\":\"credit\",\"usage\":0.01}",
}, "")

expectedText := "I don't have access to data."
expectedToolCalls := []OpenAIToolCall{} // No tool calls
```

#### 1.3 Anthropic-Style SSE Streams
**Test Name**: `TestParseResponseFromAnthropicStyleStream`

**Acceptance Criteria**:
- Must parse standard Anthropic SSE format with event: and data: lines
- Must handle content_block_start/delta/stop sequences
- Must aggregate text deltas correctly
- Must maintain proper event ordering

**Edge Cases to Test**:
- Missing event lines (data-only streams)
- Empty content blocks
- Malformed event types
- Nested content within delta events

**Test Scenarios**:
```go
stream := strings.Join([]string{
    "event: message_start",
    `data: {"type":"message_start","message":{"content":[{"type":"text","text":""}]}}`,
    "",
    "event: content_block_start",
    `data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
    "",
    "event: content_block_delta",
    `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"I don"}}`,
    "",
    "event: content_block_delta",
    `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"'t have access"}}`,
    "",
    "event: content_block_delta",
    `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" to data."}}`,
    "",
    "event: content_block_stop",
    `data: {"type":"content_block_stop","index":0}`,
    "",
    "event: message_stop",
    `data: {"type":"message_stop"}`,
}, "\n")

expectedText := "I don't have access to data."
```

#### 1.4 Anthropic Tool Use SSE Streams
**Test Name**: `TestParseResponseFromAnthropicToolStream`

**Acceptance Criteria**:
- Must handle mixed text and tool_use content blocks
- Must accumulate partial_json deltas for tool arguments
- Must maintain block indexing for multiple tools
- Must properly reconstruct tool call arguments from fragments

**Edge Cases to Test**:
- Multiple tool use blocks with different indices
- Incomplete partial_json that needs sanitization
- Tool calls with no arguments
- Text and tool blocks interleaved

**Test Scenarios**:
```go
stream := strings.Join([]string{
    "event: content_block_start",
    `data: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_1","name":"get_weather","input":{}}}`,
    "",
    "event: content_block_delta",
    `data: {"type":"content_block_delta","index":1,"delta":{"type":"tool_use_delta","partial_json":"{\"location\": \"Tokyo\""}}`,
    "",
    "event: content_block_delta",
    `data: {"type":"content_block_delta","index":1,"delta":{"type":"tool_use_delta","partial_json":", \"unit\": \"°C\"}"}}`,
    "",
    "event: content_block_stop",
    `data: {"type":"content_block_stop","index":1}`,
}, "\n")

expectedToolCalls := []OpenAIToolCall{
    {ID: "toolu_1", Name: "get_weather", Arguments: `{"location":"Tokyo","unit":"°C"}`},
}
```

### 2. JSON Error Handling Tests
**Focus Areas**: Malformed JSON recovery, invalid input handling, graceful degradation

#### 2.1 Invalid JSON Input Handling
**Test Name**: `TestKiroParseResponse_InvalidJSON`

**Acceptance Criteria**:
- Must not panic on invalid JSON input
- Should return raw content as fallback for non-empty input
- Must handle completely malformed JSON gracefully
- Should preserve some content when possible

**Edge Cases to Test**:
- Completely invalid JSON syntax
- Partially valid JSON with trailing garbage
- Plain text with no JSON structure
- Empty strings and null inputs

**Test Scenarios**:
```go
invalidInputs := []string{
    "invalid json string",           // Plain text
    "{ malformed json }",            // Syntax error
    "just plain text",               // Non-JSON content
    "",                             // Empty string
    "{\"unclosed\": \"string",       // Unclosed quotes/braces
    "random characters !@#$%^&*()",   // Gibberish input
}

for _, input := range invalidInputs {
    content, toolCalls := kiro.ParseResponse([]byte(input))
    if input != "" {
        assert.NotEmpty(t, content, "Should return some content for non-empty input")
    }
    assert.Empty(t, toolCalls, "Should return no tool calls for invalid JSON")
}
```

#### 2.2 Hard Request Error Handling
**Test Name**: `TestKiroHardRequestErrorHandling`

**Acceptance Criteria**:
- Must handle BuildRequest errors gracefully
- Should provide meaningful error messages
- Must not crash on malformed fixture data
- Should preserve partial functionality when possible

**Edge Cases to Test**:
- Invalid JSON in fixture files
- Missing required fields in requests
- Malformed token data
- Extremely large or complex fixtures

**Test Scenarios**:
```go
invalidJSON := []byte(`{"invalid": json}`)

// BuildRequest should handle invalid JSON gracefully
_, err := kirotranslator.BuildRequest("claude-sonnet-4-5", invalidJSON, token, nil)
assert.Error(t, err, "Should return error for invalid JSON")

// ParseResponse should not panic on invalid JSON
content, toolCalls := kiro.ParseResponse(invalidJSON)
assert.NotEmpty(t, content, "Should return raw content as fallback")
assert.Empty(t, toolCalls, "Should return no tool calls for invalid input")
```

### 3. Special Character Preservation Tests
**Focus Areas**: Apostrophe handling, Unicode support, edge case character combinations

#### 3.1 Apostrophe at End of Text
**Test Name**: `TestSpecialCharacterPreservation/EdgeCase_Apostrophe_at_end_of_text`

**Acceptance Criteria**:
- Must preserve trailing apostrophes in content
- Should handle apostrophes at string boundaries
- Must not truncate content at final apostrophe
- Should maintain JSON validity while preserving characters

**Edge Cases to Test**:
- Single apostrophe at end of string
- Multiple consecutive trailing apostrophes
- Apostrophes combined with other punctuation
- Apostrophes in quoted text at end

**Test Scenarios**:
```go
testCases := []struct {
    input    string
    expected string
}{
    {
        input:    `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "That's all folks'"}}}`,
        expected: "That's all folks'",
    },
    {
        input:    `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "Rock''n''roll'"}}}`,
        expected: "Rock''n''roll'",
    },
    {
        input:    `{"conversationState": {"currentMessage": {"assistantResponseMessage": {"content": "Quote: \"It's fine\"'"}}}`,
        expected: `Quote: "It's fine"'`,
    },
}
```

#### 3.2 Comprehensive Character Preservation
**Test Name**: `TestSpecialCharacterPreservation` (overall)

**Acceptance Criteria**:
- Must preserve all English contractions (don't, won't, can't, etc.)
- Should handle Unicode characters with apostrophes
- Must preserve special symbols and punctuation
- Should not truncate at any character combination

**Edge Cases to Test**:
- All common English contractions
- Unicode characters combined with apostrophes
- Special characters that might trigger parsing issues
- Mixed quotes and apostrophes
- Technical content with apostrophes

**Test Scenarios**:
```go
contractions := []string{
    `don't`, `won't`, `can't`, `it's`, `that's`, `we're`, `they're`,
    `John's`, `isn't`, `aren't`, `wasn't`, `weren't`, `haven't`,
    `let's`, `there's`, `you've`, `we've`, `I'll`, `it'll`,
}

specialSymbols := []string{
    "'", `"`, `\`, `/`, `?`, `!`, `,`, `.`, `:`, `;`,
    `(`, `)`, `[`, `]`, `{`, `}`, `<`, `>`, `*`, `&`,
    `%`, `+`, `=`, `#`, `@`, `$`, `^`, `~`, "`", `|`,
    `_`, `-`,
}

edgeCases := []struct {
    input    string
    expected string
}{
    {
        input:    `{"content": "Don't, won't, can't, shouldn't"}`,
        expected: "Don't, won't, can't, shouldn't",
    },
    {
        input:    `{"content": "Café's résumé isn't finished"}`,
        expected: "Café's résumé isn't finished",
    },
    {
        input:    `{"content": "Class of '99 and students' grades"}`,
        expected: "Class of '99 and students' grades",
    },
}
```

## Edge Case Matrix

| Category | Test Case | Input Type | Expected Behavior | Failure Mode |
|----------|-----------|------------|------------------|--------------|
| SSE Parsing | Empty data lines | `data: \n\ndata: {"content":"hi"}` | Skip empty lines, parse content | Missing content aggregation |
| SSE Parsing | Malformed JSON in data | `data: {invalid json}` | Skip invalid JSON, continue parsing | Parse failure, lost content |
| SSE Parsing | Control characters | `\v:message-typeevent{"content":"hi"}` | Parse content after control chars | Truncation at control chars |
| JSON Error | Invalid syntax | `{malformed json}` | Return raw content as fallback | Empty result, panic |
| JSON Error | Partial JSON | `{"partial": "content" garbage` | Extract valid JSON portion | Parse failure |
| Special Chars | Trailing apostrophe | `"text'"` | Preserve trailing apostrophe | Truncation at apostrophe |
| Special Chars | Contractions | `"don't stop"` | Preserve all contractions | Apostrophe removal |
| Special Chars | Unicode + apostrophe | `"Café's menu"` | Preserve Unicode + apostrophe | Encoding issues |
| Tool Calls | Fragmented args | Multiple partial_json deltas | Combine into valid JSON | Incomplete arguments |
| Tool Calls | Missing stop flag | Tool call without stop marker | Auto-finalize at end | Lost tool calls |

## Implementation Requirements

### 1. SSE Stream Parser Enhancements
- **Robust line parsing**: Handle mixed control characters and whitespace
- **JSON validation**: Skip invalid JSON without failing entire parse
- **Content aggregation**: Properly combine text from multiple chunks
- **Tool call reconstruction**: Accumulate partial_json deltas correctly

### 2. JSON Error Recovery
- **Graceful degradation**: Return raw content when JSON parsing fails
- **Partial extraction**: Extract valid JSON from malformed input
- **Error handling**: Never panic on invalid input
- **Fallback mechanisms**: Multiple strategies for content extraction

### 3. Character Preservation
- **Boundary handling**: Special handling for string start/end characters
- **Unicode support**: Proper UTF-8 handling throughout pipeline
- **Escape sequence handling**: Correct processing of JSON escapes
- **No truncation**: Ensure no character combination causes content loss

### 4. Tool Call Processing
- **Argument accumulation**: Proper merging of fragmented tool arguments
- **JSON sanitization**: Fix common JSON formatting issues
- **Deduplication**: Remove duplicate tool calls while preserving order
- **Validation**: Ensure final tool call arguments are valid JSON

## Test Data Requirements

### 1. SSE Stream Samples
- Basic multi-line streams with text only
- Complex streams with mixed text and tool calls
- Streams with control characters and malformed data
- Anthropic-format streams with proper event sequences

### 2. JSON Error Samples
- Various malformed JSON structures
- Partially valid JSON with trailing garbage
- Plain text and non-JSON content
- Edge cases with special characters

### 3. Special Character Samples
- All English contractions in context
- Unicode text with apostrophes and accents
- Technical content with various symbols
- Edge cases with boundary characters

## Success Metrics

### 1. Functional Requirements
- [ ] All failing tests pass with 100% success rate
- [ ] No regressions in existing functionality
- [ ] Proper handling of all edge cases in matrix
- [ ] Performance remains acceptable for large inputs

### 2. Quality Requirements
- [ ] Code coverage ≥ 95% for modified functions
- [ ] No memory leaks or panics in error conditions
- [ ] Proper error messages for debugging
- [ ] Documentation updates for new behaviors

### 3. Compatibility Requirements
- [ ] Backward compatibility with existing valid inputs
- [ ] No breaking changes to public APIs
- [ ] Consistent behavior across all input types
- [ ] Proper integration with existing test suite

## Risk Assessment

### High Risk Areas
1. **SSE Parser Complexity**: Multiple event types and formats increase complexity
2. **Character Encoding**: Unicode and special character handling across different encodings
3. **JSON Recovery**: Balancing error recovery with security considerations

### Mitigation Strategies
1. **Incremental Development**: Implement fixes one category at a time
2. **Comprehensive Testing**: Extensive edge case coverage before deployment
3. **Fallback Mechanisms**: Multiple strategies for content extraction
4. **Performance Monitoring**: Ensure fixes don't impact performance

## Implementation Priority

### Phase 1: Critical Fixes
1. Fix trailing apostrophe truncation issue
2. Implement basic SSE stream parsing for simple cases
3. Add JSON error recovery mechanisms

### Phase 2: Enhanced Functionality
1. Complex SSE stream handling with control characters
2. Advanced JSON sanitization and partial extraction
3. Comprehensive special character preservation

### Phase 3: Optimization & Polish
1. Performance optimization for large inputs
2. Enhanced error messages and debugging
3. Additional edge case coverage

This specification provides a comprehensive framework for addressing the failing tests while ensuring robust, maintainable code that handles all edge cases and input variations effectively.