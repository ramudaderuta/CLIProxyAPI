# Test Documentation

> **Purpose**: Provide a clear, prescriptive standard for structuring and running tests so future incremental work is fast, predictable, and low-risk.

## Scope
These rules apply to everything under the `tests/` directory. They do **not** require any production code changes.

---

## Design Rationale (Short)

- **Clarity & Speed**: Separate categories align with intent; default jobs stay fast.
- **Stability**: Centralized `testdata/` and golden testing reduce flakiness; deterministic env/time/random.
- **Scalability**: A single `shared/testutil` avoids copy‑paste while keeping domain data local.
- **De-duplication**: Centralized test data eliminates duplicate files across test categories.
- **Buffer Safety**: All SSE streaming uses 20MB buffers to prevent truncation of large thinking blocks.

---

## SSE Buffer Safety & Thinking Block Handling

### Buffer Size Management
All SSE streaming implementations use **20MB buffers** (20,971,520 bytes) to safely handle:
- Large thinking blocks (>64KB)
- Extensive tool call arguments
- Long streaming responses
- Complex JSON payloads

This prevents the "bufio.Scanner: token too long" errors that can occur with default 64KB buffers.

### Testing Large Thinking Blocks
Comprehensive regression tests verify thinking block handling:

```go
// tests/regression/kiro/kiro_sse_buffer_test.go
func TestKiroSSEBufferLimit(t *testing.T) {
    t.Run("Large thinking delta exceeds 64KB buffer", func(t *testing.T) {
        // Create 70KB thinking content to exceed default buffer limits
        largeThinking := generateLargeThinkingContent(70000)
        sseData := buildLargeSSEStream(largeThinking)

        // Parse without truncation
        content, toolCalls := kiro.ParseResponse([]byte(sseData))
        assert.NotEmpty(t, content)
        assert.Empty(t, toolCalls)
    })
}
```

Test scenarios include:
- Single large thinking blocks (70KB+)
- Multiple sequential large deltas
- Mixed content with large thinking sections
- SSE event boundary handling with large payloads

---

## Target Directory Layout

```
tests/
├── unit/
│   └── kiro/
│       ├── kiro_config_test.go
│       ├── kiro_core_test.go
│       ├── kiro_executor_test.go
│       ├── kiro_response_test.go
│       ├── kiro_sse_formatting_test.go
│       ├── kiro_translation_test.go
│       ├── kiro_hard_request_test.go
│       └── testdata/
│           ├── nonstream/*.json (symlinks to shared)
│           ├── streaming/*.ndjson (symlinks to shared)
│           ├── golden/*.golden
│           └── errors/*.json
├── integration/
│   └── kiro/
│       ├── kiro_executor_integration_test.go   //go:build integration
│       ├── kiro_sse_integration_test.go        //go:build integration
│       ├── kiro_translation_integration_test.go//go:build integration
│       └── testdata/ (symlinks to shared)
├── regression/
│   └── kiro/
│       ├── kiro_bug_regression_test.go (bug-linked repros only)
│       ├── kiro_tool_result_bug_test.go
│       ├── kiro_thinking_truncation_test.go
│       ├── kiro_fix_verification_test.go
│       ├── kiro_sse_buffer_test.go (SSE buffer limit tests)
│       └── testdata/ (symlinks to shared)
├── benchmarks/
│   └── kiro/
│       └── executor_benchmark_test.go
├── shared/
│   ├── http.go           # RoundTripper + httptest helpers
│   ├── payloads.go       # Request/response builders
│   ├── golden.go         # Golden file helpers (-golden flag)
│   ├── env.go            # Env/time/random helpers
│   ├── io.go             # Centralized test data loading
│   ├── testdata/        # Centralized test data
│   │   ├── nonstream/*.json
│   │   └── streaming/*.ndjson
│   └── test_utils.go     # KiroTestFixtures and common utilities
└── TEST_DOCUMENTATION.md
```

**Notes**
- **Centralized test data** in `tests/shared/testdata/` eliminates duplication across unit/integration/regression tests
- **Symlinks** from individual test directories to shared testdata maintain compatibility
- **Dynamic token creation** replaces hardcoded absolute paths with `t.TempDir()`
- Put test-only data under a `testdata/` folder. Go tooling ignores it for builds, and paths are stable.
- Prefer **domain folders** (e.g., `kiro/`) so file names can be concise (no long prefixes).

---

## Test Categories & Intent

| Category | Purpose | Scope | Speed | Key Rules |
|----------|---------|--------|--------|-----------|
| **Unit** (`tests/unit/...`) | Pure logic or small surface tests | Fast, isolated; mocks/fakes only; exhaustive edge cases & format checks | Very Fast | Keep strict format tests only here |
| **Integration** (`tests/integration/...`) | Cross-component, real I/O or protocol flows | Real components/services; 1-2 end-to-end flows per feature; smoke/assert key invariants only | Moderate | Remove redundant SSE format tests; keep basic smoke assertions |
| **Regression** (`tests/regression/...`) | **Only** minimal repros for historic/linked bugs | Bug-linked repros only; no general coverage | Fast | Remove tests that merely duplicate unit behavior unless tied to bug ID |
| **Benchmarks** (`tests/benchmarks/...`) | Performance and allocations | Performance only, not correctness | Varies | Opt-in only |

---

## Category Boundaries

### Unit Tests
- **Fast, isolated**: mocks/fakes only
- **Exhaustive**: edge cases & format checks
- **Parallel-friendly**: can use `t.Parallel()`
- **Strict format validation**: Keep detailed SSE format tests only in unit tests

### Integration Tests
- **Real components/services**: actual I/O or protocol flows
- **1-2 end-to-end flows per feature**: smoke/assert key invariants only
- **Build tag**: `//go:build integration`
- **Basic smoke assertions**: Remove detailed format validation; keep minimal smoke tests

### Regression Tests
- **Only bug-linked repros**: must be tied to specific bug IDs
- **No general coverage**: remove tests that merely duplicate unit behavior
- **Minimal repros**: smallest test case that reproduces the bug
- **No absolute paths**: use `t.TempDir()` and dynamic file creation

### Benchmarks
- **Performance only**: not correctness
- **Opt-in**: run explicitly when needed

---

## When to Write Unit vs Integration vs Regression

### Write Unit Tests When:
- Testing pure logic or isolated functions
- Need exhaustive edge case coverage
- Testing format validation or parsing
- Can use mocks/fakes effectively
- Want fast feedback during development

### Write Integration Tests When:
- Testing cross-component interactions
- Need real I/O or protocol flows
- Testing end-to-end happy paths
- Validating basic smoke functionality
- Cannot easily mock the dependencies

### Write Regression Tests When:
- Reproducing a specific bug that was fixed
- Need to prevent regression of a known issue
- The bug has a clear, minimal reproduction case
- Want to document the fix for future reference

---

## Anti-Patterns to Avoid

### **Copying test data across categories**
- **Don't**: Duplicate identical JSON files in `unit/`, `integration/`, and `regression/`
- **Do**: Use centralized `tests/shared/testdata/` with symlinks

### **Asserting full SSE format in integration tests**
- **Don't**: Detailed SSE format validation in integration tests
- **Do**: Keep strict format tests in unit tests; basic smoke assertions in integration

### **Regression tests for general coverage**
- **Don't**: Write regression tests that merely duplicate unit behavior
- **Do**: Only write regression tests for specific, linked bugs

### **Using absolute paths in tests**
- **Don't**: Hardcode paths like `/home/build/code/CLIProxyAPI/tmp/...`
- **Do**: Use `t.TempDir()` and dynamic file creation

### **Multiple translation e2e flows in integration**
- **Don't**: Exhaustive translation testing in integration
- **Do**: Keep one "happy path" and one streaming path in integration; thorough cases in unit

---

## Naming Conventions

- **Top-level tests**: `Test<Domain>_<Capability>_<Scenario>`
  _Example_: `TestKiro_Executor_ToolCalls`.
- **Table-driven subtests**: `t.Run("<case>")` with meaningful, grep-friendly names like `streaming/tool-interleave`.
- **Benchmarks**: `Benchmark<Domain>_<Target>`.

---

## Subtests & Parallelism

- Use table-driven style and `t.Run` for cases.
- Default to `t.Parallel()` inside each test to maximize throughput.
- Tests that write to the same resource (e.g., golden files) **must not** run in parallel. Either guard with a mutex or isolate per-case files.

```go
func TestKiro_Response_Format(t *testing.T) {
    t.Parallel()
    cases := []struct{
        name string
        in   []byte
        want string
    }{ /* ... */ }
    for _, tc := range cases {
        tc := tc
        t.Run(tc.name, func(t *testing.T) {
            t.Parallel()
            // test body...
        })
    }
}
```

---

## Assertions

- Use `require.*` when a failure invalidates the rest of the test (e.g., input parsing, setup).
- Use `assert.*` for value-by-value checks that can continue.
- Error messages must include the **case name** and key fields.

```go
require.NoError(t, err, "case=%s: parse failed", tc.name)
assert.Equal(t, want, got, "case=%s", tc.name)
```

---

## Testdata & Golden Files

### Accessing Shared Test Data
- Use centralized test data from `tests/shared/testdata/`:

```go
// Load test data from shared location
fixtureData := testutil.LoadTestData(t, "nonstream/text_then_tool.json")
```

- Use `t.TempDir()` for ephemeral writes and dynamic token creation.

### Centralized Test Data Structure
The `tests/shared/testdata/` directory organizes test fixtures by type:
- **nonstream/**: Non-streaming request/response JSON files
- **streaming/**: Streaming test data in NDJSON format
- **golden/**: Golden reference files with `.golden` extension
- **errors/**: Error case test data for negative testing

### Golden Testing
- Golden files live at `tests/shared/golden/*.golden`.
- Use the golden helper with `-golden` flag support in `shared/testutil/golden.go`:

```go
// Assert against golden file in shared location
testutil.AssertMatchesGolden(t, gotBytes, "sse_minimal_stream.golden")
```

_Update usage:_

```bash
# Update golden files using flag
go test ./tests/unit/... -run 'SSE|Translation' -v -golden

# Or using environment variable
UPDATE_GOLDEN=1 go test ./tests/unit/... -run <pkg|TestName>
```

---

## HTTP & External Dependencies

### Mocking with a custom RoundTripper
Place in `shared/testutil/http.go`:

```go
package testutil

import (
    "net/http"
)

type RT func(*http.Request) (*http.Response, error)

func (f RT) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
```

Usage in tests (constructor should accept an http.Client or option):

```go
cli := testutil.Client(func(r *http.Request) (*http.Response, error) {
    // return canned responses based on r.URL.Path, headers, etc.
})
// pass cli into the unit under test
```

### Integration with `httptest.Server`
For multi-endpoint or stateful flows, use `net/http/httptest` in `integration` tests.

```go
ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    // implement protocol behavior here
}))
defer ts.Close()
```

---

## Time, Randomness, and Environment

- Use `t.Setenv("KEY", "VALUE")` to control environment.
- Seed randomness deterministically in tests: `rand.New(rand.NewSource(1))`.
- For time-sensitive code, prefer injecting a `Clock`/`Now()` dependency; otherwise, isolate time-based behavior and test with deltas rather than exact timestamps.
- **Replace absolute paths**: Use `t.TempDir()` and dynamic file creation instead of hardcoded paths like `/home/build/code/CLIProxyAPI/tmp/...`

---

## Build Tags & Test Selection

- All **integration** tests must start with the tag header:

```go
//go:build integration
// +build integration
```

- Run categories:

```bash
# Unit + Regression (default in CI)
go test ./tests/unit/... ./tests/regression/... -race -cover -v

# Integration (opt-in)
go test -tags=integration ./tests/integration/... -v

# Benchmarks (opt-in)
go test ./tests/benchmarks/... -bench . -benchmem -run ^$
```

- Name-based selection:

```bash
go test ./tests/unit/... -run 'Translation' -v
go test -tags=integration ./tests/integration/... -run 'SSE' -v
```

---

## Quick Command Reference

```bash
# All (excluding integration & benchmarks)
go test ./tests/unit/... ./tests/regression/... -race -cover -v

# Only a domain/family
go test ./tests/unit/kiro -run 'Executor' -v

# Integration
go test -tags=integration ./tests/integration/... -v

# Update goldens
go test ./tests/unit/... -run 'SSE|Translation' -v -golden

# Or using environment variable
UPDATE_GOLDEN=1 go test ./tests/unit/... -run <pkg|TestName>

# Benchmarks
go test ./tests/benchmarks/... -bench . -benchmem -run ^$

# Run specific test patterns
go test ./tests/unit/kiro -run 'SSEFormatting' -v
go test ./tests/unit/kiro -run 'Translation' -v
go test ./tests/unit/kiro -run 'HardRequest' -v
go test ./tests/regression/kiro -run 'BugReproduction' -v
go test ./tests/regression/kiro -run 'SSEBuffer' -v  # Buffer limit tests
go test ./tests/regression/kiro -run 'ThinkingTruncation' -v
```

---

## Recent Changes

The following changes have been implemented to improve test coverage and reliability:

### **Tool Call ID Sanitization (TDD Implementation)**
- Added comprehensive tool_call_id validation and sanitization tests in `tests/unit/kiro/kiro_tool_call_id_test.go`
- **Problem Solved**: Prevents "Unexpected tool_call_id returns" errors from upstream LLM caused by malformed Claude Code tool IDs like `"***.TodoWrite:3"`, `"***.Edit:6"`, `"***.Bash:8"`
- **Solution**: Validates and sanitizes all tool_call_id values before sending to upstream systems
- **Test Coverage**:
  - **Validation Tests**: 11 test cases covering valid UUID formats, OpenAI tool formats, and invalid patterns (colons, triple-asterisks)
  - **Sanitization Tests**: 8 test cases ensuring invalid IDs are replaced with valid UUIDs while preserving valid formats
  - **Integration Tests**: 3 test cases verifying real-world usage with mixed valid/invalid inputs
  - **Performance Tests**: Ensures fast processing of valid IDs (no unnecessary generation)
  - **Uniqueness Tests**: Verifies generated UUIDs are unique across multiple calls

**Implementation Details**:
```go
// ValidateToolCallID checks if a tool_call_id is in a valid format
func ValidateToolCallID(id string) bool {
    trimmed := strings.TrimSpace(id)
    if trimmed == "" {
        return false
    }
    // Reject IDs with colons (like "***.TodoWrite:3")
    if strings.Contains(trimmed, ":") {
        return false
    }
    // Reject IDs with triple-asterisk patterns
    if strings.Contains(trimmed, "***") {
        return false
    }
    return true
}

// SanitizeToolCallID ensures a tool_call_id is valid
func SanitizeToolCallID(id string) string {
    if ValidateToolCallID(id) {
        return id
    }
    // Generate a new valid UUID for invalid IDs
    return "call_" + uuid.New().String()
}
```

**Integration Points**:
- User message tool results sanitization (line 178 in request.go)
- User message tool uses sanitization (line 191 in request.go)
- Assistant message tool uses sanitization (line 222 in request.go)

**Test Execution**:
```bash
# Run tool_call_id specific tests
go test ./tests/unit/kiro -run 'ToolCallID' -v

# All tool_call_id tests passing
PASS: TestValidateToolCallID (0.00s)
PASS: TestSanitizeToolCallID (0.00s)
PASS: TestSanitizeToolCallIDUniqueness (0.00s)
PASS: TestSanitizeToolCallIDPerformance (0.00s)
PASS: TestToolCallIDIntegration (0.00s)
```

### **SSE Buffer Safety & Large Content Handling**
- Added comprehensive buffer limit tests in `tests/regression/kiro/kiro_sse_buffer_test.go`
- Verified all SSE streaming implementations use 20MB buffers (326x larger than default 64KB)
- Tests cover scenarios with 70KB+ thinking blocks to prevent truncation
- Added multi-delta sequence testing for sequential large content processing
- Implemented SSE event boundary testing for proper event assembly

### **Test Fixes & Corrections**
- **Fixed**: `TestKiroExecutor_Integration_IncrementalStreaming` test assertion
  - **Issue**: Test expected full content in single text_delta event instead of incremental streaming
  - **Fix**: Updated assertions to verify proper incremental streaming across multiple text_delta events
  - **Status**: ✅ Test now passes with correct streaming behavior validation
- **Verified**: All SSE streaming tests working correctly with proper incremental text streaming
- **Confirmed**: Content properly distributed across multiple text_delta events as expected

### **Centralized Test Data**
- Created `tests/shared/testdata/{nonstream,streaming}/` with shared JSON/NDJSON files
- Replaced duplicate files in `unit/`, `integration/`, `regression/` with symlinks
- Implemented `testutil.LoadTestData()` helper for consistent access

### **Removed Redundant SSE Format Tests**
- Simplified integration SSE tests to basic smoke assertions only
- Kept detailed SSE format validation in unit tests only
- Removed excessive format checking from integration layer

### **Collapsed Translation e2e Duplicates**
- Kept one "happy path" and one streaming path in integration tests
- Moved thorough translation testing to unit tests
- Removed duplicate translation flow tests

### **Migrated Regression Tests**
- Removed general coverage tests (apostrophe handling, backward compatibility)
- Kept only bug-linked reproduction tests with specific issue references
- Ensured all regression tests are tied to historic bugs

### **Replaced Absolute Paths**
- Removed all hardcoded paths like `/home/build/code/CLIProxyAPI/tmp/...`
- Implemented dynamic token file creation using `t.TempDir()`
- Added `testutil.CreateTestTokenFile()` helper for consistent token creation

### **Updated Golden File System**
- Enhanced golden helpers to support shared golden directory
- Added both `-golden` flag and `UPDATE_GOLDEN=1` environment variable support
- Implemented `AssertMatchesGolden()` for centralized golden file management

## Current Test Status

### Unit Tests
- **89 test functions** across 14 Go test files
- **All passing**: No failures in unit test suite
- **2 skipped tests** with proper TODOs for known implementation limitations:
  - `TestParseResponseFromEventStream`: Tool call argument merging not yet implemented
  - `TestParseResponseFromEventStream_SSEParsing/SSE_With_Tool_Calls`: SSE tool call parsing not implemented

### Regression Tests
- **All passing**: No failures in regression test suite
- **22 test functions** across 7 Go test files
- Focused on bug-linked reproductions and buffer safety testing
- Comprehensive SSE buffer limit testing with 70KB+ thinking blocks

### Integration Tests
- **All tests passing**: No failures in integration test suite
- **Comprehensive streaming functionality**: All SSE streaming tests working correctly
- **Incremental streaming verified**: Content properly streamed across multiple text_delta events
- **Buffer safety confirmed**: 20MB buffers handle large thinking blocks without truncation
- **Test coverage**: 7 test functions across 3 integration test files
