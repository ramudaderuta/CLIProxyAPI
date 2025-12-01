# Project Overview
- Purpose: Go-based HTTP proxy exposing OpenAI/Gemini/Claude/Kiro-compatible endpoints for CLI/IDE tools; handles auth, translation, routing, and streaming synthesis.
- Tech stack: Go modules; Gin HTTP server; OAuth/token auth flows; multiple provider executors; configurable via YAML; optional Postgres/Git/S3 token stores; Docker/Docker Compose scripts available.
- Entry point: `cmd/server/main.go`; build binary `cli-proxy-api`.
- Key dirs: `internal/api` (handlers/middleware), `internal/auth` (provider auth flows), `internal/runtime/executor` (per-provider executors), `internal/translator` (request/response translators), `internal/store` (token persistence), `internal/registry` (model routing), `tests/` (unit/regression/integration/bench), `sdk/` (public Go SDK), `docs/` (guides), `examples/`.
- Config: start from `config.example.yaml`; tokens/auth under `auths/`; uses `.env` for overrides.
- Current focus (per AGENTS): Claude Code ↔ Kiro parity, tool description clamping/manifest, sanitized Anthropic payloads, SSE synthesis, Kiro fallback attempts (primary/flattened/minimal).