# Suggested Commands
- Build server: `go build -o cli-proxy-api ./cmd/server`
- Run server with config: `./cli-proxy-api --config config.yaml` (use `config.example.yaml` as a base); debug mode `--debug`; config check `--check`.
- OAuth logins: `./cli-proxy-api --login` (Gemini), `--claude-login`, `--codex-login`, `--qwen-login`, `--iflow-login`, `--iflow-cookie`, `--antigravity-login`; set auth-dir in config or env.
- Development run without build: `go run ./cmd/server --config config.yaml`
- Unit/regression focus (Kiro parity): `go test ./tests/unit/kiro -run 'BuildRequest|ParseResponse' -count=1` and `go test ./tests/unit/kiro -run 'ConvertKiroStreamToAnthropic|NormalizeKiroStreamPayload' -count=1`.
- Full suites: `go test ./tests/unit/... ./tests/regression/... -race -cover`, integration (needs services): `go test -tags=integration ./tests/integration/...`.
- Benchmarks: `go test ./tests/benchmarks/... -bench . -benchmem -run ^$`.
- Code quality: `gofmt -w .`, `go vet ./...`, `golangci-lint run`, `gosec ./...` (if installed).
- Dependencies: `go mod download`, tidy deps `go mod tidy`.