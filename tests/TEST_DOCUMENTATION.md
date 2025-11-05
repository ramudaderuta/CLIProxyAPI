# Test Documentation

> **Purpose**: Provide a clear, prescriptive standard for structuring and running tests so future incremental work is fast, predictable, and low-risk.

## Scope
These rules apply to everything under the `tests/` directory. They do **not** require any production code changes.

---

## Design Rationale (Short)

- **Clarity & Speed**: Separate categories align with intent; default jobs stay fast.
- **Stability**: `testdata/` and golden testing reduce flakiness; deterministic env/time/random.
- **Scalability**: A single `shared/testutil` avoids copy‑paste while keeping domain data local.

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
│           ├── nonstream/*.json
│           ├── streaming/*.ndjson
│           ├── golden/*.golden
│           └── errors/*.json
├── integration/
│   └── kiro/
│       ├── kiro_executor_integration_test.go   //go:build integration
│       ├── kiro_sse_integration_test.go        //go:build integration
│       ├── kiro_translation_integration_test.go//go:build integration
│       └── testdata/ (mirrors the needed fixtures)
├── regression/
│   └── kiro/
│       ├── kiro_apostrophe_test.go
│       ├── kiro_backward_compatibility_test.go
│       ├── kiro_bug_regression_test.go
│       ├── kiro_tool_result_bug_test.go
│       ├── kiro_thinking_truncation_test.go
│       └── kiro_fix_verification_test.go
├── benchmarks/
│   └── kiro/
│       └── executor_benchmark_test.go
└── shared/
    ├── http.go           # RoundTripper + httptest helpers
    ├── payloads.go       # Request/response builders
    ├── golden.go         # Golden file helpers (-update)
    ├── env.go            # Env/time/random helpers
    ├── fs.go             # Testdata helpers
    └── test_utils.go     # KiroTestFixtures and common utilities
```

**Notes**
- Put test-only data under a `testdata/` folder. Go tooling ignores it for builds, and paths are stable.
- Prefer **domain folders** (e.g., `kiro/`) so file names can be concise (no long prefixes).

---

## Test Categories & Intent

- **Unit/Functional** (`tests/unit/...`): Pure logic or small surface tests; fast; parallel-friendly.
- **Integration** (`tests/integration/...`): Cross-component, real I/O or protocol flows; guarded by `integration` build tag.
- **Regression/Compatibility** (`tests/regression/...`): Lock in fixes for bugs and compatibility guarantees.
- **Benchmarks** (`tests/benchmarks/...`): Performance and allocations; opt‑in only.

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

### Accessing `testdata/`
- Read files using stable relative paths anchored at the package working dir:

```go
path := filepath.Join("testdata", "nonstream", "sample.json")
b, err := os.ReadFile(path)
require.NoError(t, err)
```

- Use `t.TempDir()` for ephemeral writes.

### Testdata Directory Structure
The `testdata/` directory organizes test fixtures by type:
- **nonstream/**: Non-streaming request/response JSON files
- **streaming/**: Streaming test data in NDJSON format
- **golden/**: Golden reference files with `.golden` extension
- **errors/**: Error case test data for negative testing

### Golden testing
- Golden files live at `testdata/golden/*.golden`.
- Provide a single helper with `-update` support in `shared/testutil/golden.go`:

```go
package testutil

import (
    "flag"
    "os"
    "path/filepath"
    "testing"

    "github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "update golden files")

func AssertGoldenBytes(t *testing.T, name string, got []byte) {
    t.Helper()
    p := filepath.Join("testdata", "golden", name+".golden")
    if *update {
        require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
        require.NoError(t, os.WriteFile(p, got, 0o644))
    }
    want, err := os.ReadFile(p)
    require.NoError(t, err, "missing golden: %s", p)
    require.Equal(t, string(want), string(got))
}
```

_Update usage:_

```
go test ./tests/unit/... -run 'SSE|Translation' -v -update
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

func Client(rt RT) *http.Client {
    return &http.Client{Transport: rt}
}
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

---

## Build Tags & Test Selection

- All **integration** tests must start with the tag header:

```go
//go:build integration
// +build integration
```

- Run categories:

```
# Unit + Regression (default in CI)
go test ./tests/unit/... ./tests/regression/... -race -cover -v

# Integration (opt-in)
go test -tags=integration ./tests/integration/... -v

# Benchmarks (opt-in, no regular tests)
go test ./tests/benchmarks/... -bench . -benchmem -run ^$
```

- Name-based selection:

```
go test ./tests/unit/... -run 'Translation' -v
go test -tags=integration ./tests/integration/... -run 'SSE' -v
```

---

## Quick Command Reference

```
# All (excluding integration & benchmarks)
go test ./tests/unit/... ./tests/regression/... -race -cover -v

# Only a domain/family
go test ./tests/unit/kiro -run 'Executor' -v

# Integration
go test -tags=integration ./tests/integration/... -v

# Update goldens
go test ./tests/unit/... -run 'SSE|Translation' -v -update

# Benchmarks
go test ./tests/benchmarks/... -bench . -benchmem -run ^$

# Run specific test patterns
go test ./tests/unit/kiro -run 'SSEFormatting' -v
go test ./tests/unit/kiro -run 'Translation' -v
go test ./tests/unit/kiro -run 'HardRequest' -v
go test ./tests/regression/kiro -run 'BackwardCompatibility' -v
```
