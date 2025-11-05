# Test Documentation

> **Purpose**: Provide a clear, prescriptive standard for structuring and running tests so future incremental work is fast, predictable, and low-risk.

## Scope
These rules apply to everything under the `tests/` directory. They do **not** require any production code changes.

---

## Target Directory Layout

```
tests/
├── unit/
│   └── kiro/
│       ├── config_test.go
│       ├── core_test.go
│       ├── executor_test.go
│       ├── response_test.go
│       ├── sse_formatting_test.go
│       ├── translation_test.go
│       ├── hard_request_test.go
│       └── testdata/
│           ├── nonstream/*.json
│           ├── streaming/*.ndjson
│           └── golden/*.golden
├── integration/
│   └── kiro/
│       ├── executor_integration_test.go   //go:build integration
│       ├── sse_integration_test.go        //go:build integration
│       ├── translation_integration_test.go//go:build integration
│       └── testdata/ (mirrors the needed fixtures)
├── regression/
│   └── kiro/
│       ├── apostrophe_test.go
│       ├── backward_compatibility_test.go
│       ├── bug_regression_test.go
│       ├── tool_result_bug_test.go
│       ├── thinking_truncation_test.go
│       └── fix_verification_test.go
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

## CI Recommendations

- Default jobs:
  - **unit+regression**: `-race -cover -v`
  - Cache modules and test build cache.
- Optional jobs:
  - **integration**: behind tag/label or nightly workflow.
  - **benchmarks**: manual trigger.
- Artifacts:
  - `-coverprofile=coverage.out`; publish HTML via `go tool cover -html=coverage.out -o coverage.html`.

---

## Migration Plan (Zero-Risk First)

**T01: Layout Only**
1. Create the directory tree shown above.
2. Move existing tests into the appropriate category folders (no code changes).
3. Move all data under the closest `testdata/` folder and fix read paths.
4. Ensure `go test ./tests/...` still passes.

**T02: Utilities & Golden**
1. Introduce `shared/testutil` with `golden.go`, `http.go`, `payloads.go`, `env.go`, `fs.go`.
2. Convert stable-output tests (e.g., formatting/streaming/translation) to golden assertions.
3. Add `t.Parallel()` where safe and split any shared-write tests.

**T03: Templates & Examples**
1. Add a canonical table-driven example in `tests/unit/kiro/...` that new tests can copy.
2. For integration, prefer `httptest.Server` and minimize external dependencies.
3. Document new cases in file headers (what is covered, what is out of scope).

---

## Acceptance Checklist (for each PR)

- [ ] Directory matches the target layout.
- [ ] No test reads from paths **outside** `testdata/`.
- [ ] Unit tests are parallelized (`t.Parallel()`) unless serialized for a reason.
- [ ] Assertions use `require` for blockers and `assert` for value checks.
- [ ] Integration tests are guarded by `//go:build integration`.
- [ ] Golden helpers in place; `-update` works.
- [ ] CI jobs and `README` snippets updated.
- [ ] Benchmarks do not run in default pipelines.
- [ ] No nondeterministic sleeps or timeouts without comments/justification.
- [ ] Test names are grep-friendly and precise.

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
```

---

## Design Rationale (Short)

- **Clarity & Speed**: Separate categories align with intent; default jobs stay fast.
- **Stability**: `testdata/` and golden testing reduce flakiness; deterministic env/time/random.
- **Scalability**: A single `shared/testutil` avoids copy‑paste while keeping domain data local.
- **Incrementality**: Three small PRs keep review load low and rollbacks simple.