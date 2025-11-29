# Task Plan: Repair Kiro Translator Response Parsing

## P - Plan (Technical Design)

**Objective**: Fix Kiro translator to properly parse SSE events and build complete response structures with tool calls, metadata, and followup prompts.

### Technical Approach

**Root Cause Analysis:**
1. `parseSSEEventsForContent()` (kiro_executor.go:219-304) only extracts `content` text, discarding:
   - Tool calls (`toolCall` events)
   - Followup prompts (`followupPrompt` events)
   - Conversation metadata (`conversationId` events)
   - Usage data
2. Executor manually constructs minimal JSON: `{conversationState:{currentMessage:{content:"..."}}}`
3. Converter functions (`ConvertKiroResponseToOpenAI`, `ConvertKiroResponseToClaude`) expect full Kiro response structure
4. Architecture mismatch causes data loss before conversion

**Solution:**
Replace `parseSSEEventsForContent` with `parseSSEEventsToKiroResponse` that aggregates ALL SSE event data into a complete Kiro response structure matching the contract specification.

### Target Symbols

**Files to Modify:**
- `internal/runtime/executor/kiro_executor.go` - Replace parseSSEEventsForContent function
  - New function: `parseSSEEventsToKiroResponse(data []byte) ([]byte, error)`
  - Update Execute method (line 175)
  - Update ExecuteStream method if needed

**Response Structure (per kiro-contract.md § 2.2):**
```json
{
  "conversationState": {
    "conversationId": "...",
    "currentMessage": {
      "content": "...",
      "toolCalls": [...],
      "role": "assistant"
    },
    "followupPrompts": [...]
  }
}
```

### Event Types to Aggregate

Based on debug logs and AIClient-2-API reference:
1. `{\"content\":\"...\"}` - Text content chunks
2. `{\"conversationId\":\"...\"}` - Conversation metadata
3. `{\"followupPrompt\":{...}}` - Suggested next questions
4. `{\"toolCall\":{...}}` - Tool invocations (currently missing!)

---

## D - Do (Task Checklist)

### T001 [P] [Analysis] Examine existing SSE event patterns in test data
**DoD:** 
- Reviewed debug_kiro.log to identify all SSE event types
- Documented event format patterns with examples
- Confirmed tool call event format (if present in logs)

### T002 [P] [Backend] Implement parseSSEEventsToKiroResponse function
**DoD:**
- Created new function in `kiro_executor.go`
- Aggregates all event types (content, conversationId, followupPrompt, toolCall)
- Builds complete conversationState structure per contract
- Returns proper JSON bytes compatible with converter functions
- Handles malformed events gracefully
- Includes debug logging for aggregated structure

### T003 [P] [Backend] Update Execute method to use new parser
**DoD:**
- Line 175: Replace `parseSSEEventsForContent(normalizedBody)` call
- Line 181: Remove manual JSON construction, use parser output directly
- Line 194: Pass result to `ConvertKiroResponseToOpenAI` unchanged
- Debug logs verify full structure reaches converter

### T004 [P] [Backend] Update ProcessStream method for streaming
**DoD:**
- Reviewed processStream method (line 626)
- Ensured streaming chunks properly construct incremental Kiro responses
- Verified ConvertKiroStreamChunkToOpenAI handles tool call chunks

### T005 [P] [Test] Verify with simple text request (baseline)
**DoD:**
- Run test-all-payloads.sh with simple text payload
- Confirm content still extracted correctly
- No regression in basic functionality

### T006 [P] [Test] Verify with tool-enabled payload
**DoD:**
- Create/use test payload with tool specifications
- Confirm tool calls extracted from SSE events
- Verify OpenAI format output includes `tool_calls` array
- Verify Claude format output includes `tool_use` content blocks

### T007 [P] [Test] Verify followup prompts preservation
**DoD:**
- Confirm followupPrompt events aggregated
- Verify metadata preserved through conversion pipeline

### T008 [P] [Cleanup] Remove obsolete parseSSEEventsForContent
**DoD:**
- Function deleted after new parser working
- No references remaining in codebase
- Debug logs cleaned (keep only essential)

---

## C - Check (Test & Regression)

### New Test Cases (Must Write)

**Test File:** `tests/unit/kiro/kiro_sse_parsing_test.go` (update existing)

**Case A: Complete SSE Aggregation**
- Input: Mock SSE string with content + conversationId + followupPrompt events
- Expected: KiroResponse struct with all fields populated
- Verify: conversationState.conversationId, currentMessage.content, followupPrompts array

**Case B: Tool Call Event Parsing**
- Input: SSE string containing tool call events
- Expected: KiroResponse with toolCalls array
- Verify: Tool name, ID, arguments correctly extracted

**Case C: Mixed Event Order**
- Input: Interleaved content, tool, followup events
- Expected: All aggregated correctly regardless of order
- Verify: Content concatenated, tool calls collected, followup prompts aggregated

**Case D: Malformed Events**
- Input: SSE with invalid JSON chunks
- Expected: Graceful degradation, parse valid events
- Verify: No crash, partial data returned

### Integration Tests

**Test File:** `tests/integration/kiro/kiro_translation_integration_test.go`

**Case E: End-to-End Tool Call Flow**
- Run full Execute() with tool-enabled payload
- Mock Kiro API response with tool call SSE events
- Verify OpenAI output contains properly formatted tool_calls
- Verify Claude output contains tool_use content blocks

### Regression Strategy

**Command:** `go test ./tests/unit/kiro/... ./tests/integration/kiro/... -v -race`
- **Success Criteria:** All 100+ existing tests pass + new tests pass
- **Focus Areas:** 
  - token handling (no regression)
  - SSE streaming (verify chunk handling)
  - fallback mechanism (3-level still works)
  - thinking tag filtering (ensure not affected)

**Live Test:**
```bash
API_KEY=test-api-key-1234567890 ./tests/test-all-payloads.sh
```
- **Success Criteria:** 
  - Simple payloads: Content extracted correctly
  - Tool payloads: Tool calls appear in responses
  - Both OpenAI and Claude formats working
  - No JSON parsing errors in debug_kiro.log

---

## A - Act (Impact & Standardization)

### Impact Analysis (Blast Radius)

**Affected Files:**
- `internal/runtime/executor/kiro_executor.go` - Core changes
- All Kiro translators - Receive richer input (benefit, no breaking change)
- Test files - Need updates for new behavior

**Potential Risks:**
- Response structure changes might affect downstream consumers
- **Mitigation:** Contract-compliant structure, converters already expect it
- Streaming behavior changes
- **Mitigation:** Test streaming thoroughly, incremental aggregation

**API Breaking Changes:** NONE
- External API unchanged (still /v1/chat/completions, /v1/messages)
- Internal contract compliance improved

### Standardization

**Update Documentation:**
- `.serena/memories/kiro-translator-implementation.md` - Document new SSE parser
- `.serena/memories/kiro-payload-testing.md` - Add tool call test results
- Update CLAUDE.md/AGENTS.md if SSE parsing details documented there

**Code Quality:**
- Add comprehensive comments to parseSSEEventsToKiroResponse
- Document expected SSE event formats
- Include examples in function docstring

---

## Next Steps

1. Examine debug_kiro.log for tool call event patterns (T001)
2. Implement parseSSEEventsToKiroResponse (T002)
3. Integrate into Execute method (T003)
4. Write/update tests (T005-T007)
5. Run full regression (test-all-payloads.sh + unit tests)
6. Update memories with findings
