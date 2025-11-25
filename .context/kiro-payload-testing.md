# Kiro Payload Testing and Anthropic Format Support

**Date**: 2025-11-26  
**Status**: Live formats working ✅ | Server integration fixed ✅ | Event-stream decoding fixed ✅

## Overview

This document tracks Kiro payload validation, translator behavior, and the current test/runtime context after recent fixes.

## Completed Work

### 1. Test Data Organization

Created comprehensive test data for both OpenAI and Anthropic/Claude formats:

**OpenAI Format Payloads** (`tests/shared/testdata/nonstream/`):
- `openai_format_simple.json` - Basic request without tools (295 bytes)
- `openai_format.json` - Complex request with 11 tools, multi-turn conversation (22,224 bytes)
- `openai_format_with_tools.json` - Full debugging workflow with tool calls (9,448 bytes)

**Anthropic Format Payloads**:
- `orignal.json` - Full request with native tools and history (22,739 bytes)
- `orignal_tool_call.json` - With tool usage (5,235 bytes)
- `orignal_tool_call_no_result.json` - Tool call without result (4,661 bytes)
- `orignal_tool_call_no_tools.json` - No tools defined (3,432 bytes)

**Streaming Data** (NDJSON files):
- `text_chunks.ndjson` - Basic text aggregation
- `cross_chunk_spaces.ndjson` - Space handling across chunks
- `tool_interleave.ndjson` - Mixed content and tool calls

### 2. Anthropic Format Support Implementation

**Problem Identified**: The translator only supported OpenAI format (`system` as a message in `messages` array), but test data used Anthropic format (`system` as top-level array with `cache_control`).

**Solution Implemented** (`internal/translator/kiro/openai/chat-completions/kiro_openai_request.go`):

Added `extractAnthropicSystemPrompt()` function:
```go
// Extract system prompt - check both formats
systemPrompt := extractAnthropicSystemPrompt(inputJSON.Get("system"))
if systemPrompt == "" {
    // Fallback to OpenAI format
    systemPrompt = extractSystemPrompt(inputJSON.Get("messages"))
}
```

**Features**:
- Handles Anthropic `system` array with `{type: "text", text: "...", cache_control: {...}}`
- Falls back to OpenAI `system` message in `messages` array
- Anthropic format takes precedence when both are present
- Aggregates multiple text parts from system array

### 3. Test Suite

**Created Test Files**:

1. `tests/unit/kiro/kiro_streaming_data_test.go`
   - SSE content parsing and aggregation
   - NDJSON format validation
   - **Result**: 9/9 PASS ✅

2. `tests/unit/kiro/kiro_payload_validation_test.go`
   - Request translation verification
   - Response conversion testing
   - Parameter preservation checks
   - **Result**: 14/14 PASS ✅

### 4. Current Integration State (Updated 2025-11-26)

- Live endpoints verified against real Kiro:
  - `/v1/chat/completions` → OpenAI Chat format (choices[].message, tool_calls supported).
  - `/v1/messages` → Claude Messages format (`type: "message"`, content array/tool_use blocks).
  - `/v1/responses` → OpenAI Responses format (`object: "response"`, output content parts).
- Request converter now handles OpenAI/Anthropic/OpenAI Responses inputs, including `input` array fallback and tool specs/manifest.
- Response converters cover OpenAI Chat, Claude Messages, OpenAI Responses; streaming path emits provider-specific chunks.
- **Event-stream decoding**: ✅ Fixed (2025-11-26)
  - AWS event-stream binary format → text via `NormalizeKiroStreamPayload()`
  - Content extraction via updated `parseSSEEventsForContent()` handles `:message-typeevent` prefix
  - All unit tests pass (9/9 streaming, 14/14 payload validation)
  - Parser validation: 3/3 test cases (event-stream, plain JSON, mixed format)
- Environment note: ipv6 listeners are blocked here; all httptest servers use IPv4 or `t.Skip` with the note "Current runtime environment disables tcp6, needs to be enabled in CI that supports IPv4 later."

## Issues Discovered and Resolved

### Event-Stream Content Extraction (FIXED 2025-11-26)

**Symptom**: All API requests returned HTTP 200 OK but with empty content in responses.

**Root Cause**: 
- Kiro API returns AWS event-stream binary format
- `NormalizeKiroStreamPayload()` correctly decoded binary → text format: `:message-typeevent{"content":"..."}`
- `parseSSEEventsForContent()` was searching for `{"content":""}` pattern, missing the `:message-typeevent` prefix

**Solution**:
Updated `parseSSEEventsForContent()` in [kiro_executor.go](file:///home/build/code/CLIProxyAPI/internal/runtime/executor/kiro_executor.go#L217-L298) to:
1. Search for `:message-typeevent{` prefix first (AWS event-stream format)
2. Fall back to plain `{"` format (backward compatibility)
3. Extract complete JSON objects with proper brace counting
4. Use gjson to safely extract `content` field
5. Aggregate all content chunks into final response

**Verification**:
- ✅ Unit tests: 9/9 SSE streaming tests pass
- ✅ Payload tests: 14/14 validation tests pass
- ✅ Parser tests: 3/3 format handling tests pass
- See [test_parser_fix.go](file:///home/build/code/CLIProxyAPI/tests/manual/test_parser_fix.go) for validation details

### Server Integration Problem (Historical)

**Symptom (old)**: All API requests returned HTTP 500 Internal Server Error

**Root Cause (old)**: Kiro API returned `400 ValidationException` for all requests. Current request formatting and response translation have been fixed; live calls succeed.

**Evidence** (from `debug_kiro.log`):
```
Kiro Response Status: 400 Bad Request
X-Amzn-Errortype: ValidationException
```

**All 3 Fallback Levels Attempted**:
1. **Primary request** (full history) - 400 ValidationException
2. **Flattened history** (text-only) - 400 ValidationException  
3. **Minimal request** (no history) - 400 ValidationException

**Sample Failed Request**:
```json
{
  "conversationState": {
    "currentMessage": {
      "content": "(Continuing from previous context) ",
      "role": "user"
    },
    "history": [],
    "maxTokens": 16384,
    "model": "kiro-sonnet"
  }
}
```

**Observations**:
- ✅ Token is valid and accepted by API
- ✅ Requests reach Kiro API successfully
- ❌ API rejects requests with ValidationException
- ❌ Issue appears to be with request format/structure

## Next Steps for Investigation

### 1. Analyze Working Request Format
- Capture a successful request from actual Claude Code CLI
- Compare with our generated requests
- Identify differences in structure/fields

### 2. Review Kiro API Requirements
- Check if `maxTokens` is required/optional
- Verify `currentMessage.content` constraints (non-empty?)
- Confirm `model` name format
- Review required vs optional fields

### 3. Debug Request Generation
- Add more detailed logging to see exact JSON sent
- Test with minimal valid request first
- Gradually add complexity

## Files Modified

### Core Implementation
- `internal/translator/kiro/openai/chat-completions/kiro_openai_request.go`
  - Added `extractAnthropicSystemPrompt()` function
  - Modified `ConvertOpenAIRequestToKiro()` to check both formats

### Test Files Created
- `tests/unit/kiro/kiro_streaming_data_test.go`
- `tests/unit/kiro/kiro_payload_validation_test.go`

### Test Data Created
- `tests/shared/testdata/nonstream/openai_format_simple.json`
- `tests/shared/testdata/nonstream/openai_format.json`
- `tests/shared/testdata/nonstream/openai_format_with_tools.json`

### Scripts
- `test-all-payloads.sh` - Automated testing script (optimized 2025-11-26)
  - **Separates test formats by endpoint**:
    - OpenAI Chat Completions format (3 files) → `/v1/chat/completions`
    - Anthropic Messages format (4 files) → `/v1/messages`
  - **Format-specific validation**:
    - OpenAI: validates `.id`, `.choices`, `.message`, `.finish_reason`
    - Anthropic: validates `.id`, `.content`, `.role`, `.stop_reason`
  - **Enhanced output**: Color-coded results, clear section headers, sample responses

## References

- Implementation Plan: See `walkthrough.md` artifact
- Task Tracking: See `task.md` artifact  
- Debug Logs: `/home/build/code/CLIProxyAPI/debug_kiro.log`
- Server Logs: `/tmp/kiro-server-new.log`
- Request Logs: `/home/build/code/CLIProxyAPI/logs/v1-chat-completions-*.log`

## Conclusion

The translation layer is **fully functional** as verified by comprehensive mock testing. The Anthropic format support has been successfully implemented. However, server integration is blocked by Kiro API validation errors. The next session should focus on debugging the request format to match Kiro API expectations.

## TODOs / Gaps to Close (3W1H)

1) Golden fixtures for responses (What/Why/Who/How)
- What: Create golden files (non-stream + stream) for OpenAI Chat, Claude Messages, and OpenAI Responses outputs from Kiro. Cover tool_calls/tool_use, mixed text/tool interleave, and thinking-tag filtering.
- Why: Locks response shape to prevent regressions and mirrors antigravity’s coverage discipline.
- Who: Translator/QA owners for Kiro.
- How: Add golden JSON/NDJSON under `tests/unit/kiro/testdata/golden` and assert byte-for-byte matches in Kiro response/stream tests.

2) Token accounting parity (What/Why/Who/How)
- What: Replace the char/4 estimate in Kiro executor with a richer per-call accounting (prompt/completion/total, and stream usage when available).
- Why: Aligns with other providers’ token reporting and prevents surprising usage gaps for clients.
- Who: Runtime executor owner.
- How: Plumb usage from Kiro payloads when present; otherwise add a lightweight tokenizer fallback. Add unit tests that assert usage fields in non-stream and stream final chunks.

3) Pipeline integration review (What/Why/Who/How)
- What: Evaluate routing Kiro through the shared translator pipeline (registry) instead of the custom inline path for source-format translation.
- Why: Reduces divergence from other providers and centralizes format conversions.
- Who: Runtime/translator owners.
- How: Spike a branch that maps Kiro into the standard pipeline hooks; compare behavior with current inline path; keep the current approach if latency/compatibility tradeoffs are unacceptable but document the decision.

4) Reverse translations (optional) (What/Why/Who/How)
- What: Add registry entries for Kiro→other providers if bidirectional flows become necessary (e.g., replaying Kiro outputs as inputs to another provider).
- Why: Future-proofs cross-provider chaining; currently not required for caller-format conversions.
- Who: Translator owners.
- How: Mirror antigravity-style reverse registrations; add targeted tests to ensure role/content/tool fidelity across conversions.
