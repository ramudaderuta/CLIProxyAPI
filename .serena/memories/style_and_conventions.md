# Style and Conventions
- Go standards: `gofmt` required; explicit error handling; meaningful names; use contexts for cancellation; structured logging via logrus (`internal/logging`).
- Tests live under `tests/` (unit/regression/integration/bench). Keep golden files and fixtures in `tests/testdata/`; prefer `testutil.LoadTestData`; avoid `_test.go` in prod dirs unless necessary.
- SSE/stream safety: translators and SSE mappers expect 20MB buffers; preserve tool/event ordering; sanitize control characters/ANSI and strip protocol noise per Kiro/Anthropic parity rules.
- Tool handling: clamp tool descriptions to ≤256 chars; when truncated, emit `toolContextManifest` and system "Tool reference manifest"; preserve tool_choice via `claudeToolChoice` and add directive text.
- Transcript hygiene: ensure final user turn has non-empty content (".") when needed; assistant tool_use-only turns get placeholder text; maintain tool_use/tool_result blocks in history.
- Config/auth: support Postgres/Git/S3/FS token stores; `.env` optional; default writable path via `util.WritablePath()`.
- Documentation references: CLAUDE.md for architecture/commands; tests/TEST_DOCUMENTATION.md for testing layout and expectations.