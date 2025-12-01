# Management API & Function Map (CLIProxyAPI)

## Management API (base `http://localhost:8317/v0/management`)
- **Auth**: all routes require management key; remote access needs `allow-remote-management: true`; plaintext key accepted via `Authorization: Bearer <key>` or `X-Management-Key`; missing/disabled key returns 404; 5 failed remote attempts trigger ~30m ban.
- **Not changeable via API**: `allow-remote-management`, `remote-management-key` (plaintext auto-bcrypt on startup).
- **Conventions**: JSON bodies unless noted; simple PUT/PATCH use `{ "value": <type> }`; arrays allow raw array or `{ "items": [...] }`; array/object PATCH supports `old/new`, `index/value`, or `match` by key; Content-Type `application/json` except `/config.yaml`.
- **Core endpoints** (all require auth):
  - `GET /usage` – in-memory metrics (totals, per-hour/day, by API/model, failed_requests mirror failure_count).
  - Config: `GET /config`; `GET/PUT /config.yaml` (raw YAML, validated before save; returns changed keys on success).
  - Debug flag: `GET /debug`; `PUT/PATCH /debug` boolean.
  - Logging: `GET/PUT/PATCH /logging-to-file` bool; `GET /logs` stream recent lines (`after` timestamp); `DELETE /logs` clears rotated + truncates current.
  - Usage collection toggle: `GET/PUT/PATCH /usage-statistics-enabled` bool.
  - Proxy URL: `GET/PUT/PATCH/DELETE /proxy-url` string.
  - Quota toggles: `GET/PUT/PATCH /quota-exceeded/switch-project`, `/quota-exceeded/switch-preview-model` (bools).
  - API keys (proxy auth): `/api-keys` GET/PUT/PATCH/DELETE (list string; legacy top-level kept in sync).
  - Provider keys (object arrays): `/gemini-api-key` (headers/proxy/base-url), `/generative-language-api-key` (alias), `/codex-api-key`, `/claude-api-key`, `/openai-compatibility` (name, base-url, api-key-entries, models) with GET/PUT/PATCH/DELETE; PATCH supports `index` or `match`/`name`.
  - Auth files under `auth-dir`: `/auth-files` list, download (`/download?name=`), upload (multipart or raw JSON with `?name=`), delete single (`?name=`) or all (`?all=true`). Requires core auth manager.
  - OAuth URL starters: `/anthropic-auth-url`, `/codex-auth-url`, `/gemini-cli-auth-url?project_id=`, `/qwen-auth-url`, `/iflow-auth-url`; poll `/get-auth-status?state=` for completion (`wait|ok|error`). `?is_webui=true` supported for some flows.
  - Request settings: `/request-retry` int GET/PUT/PATCH; `/request-log` bool GET/PUT/PATCH.
- **Error shape**: consistent JSON codes (400 invalid body, 401 missing/invalid key, 403 remote disabled, 404 missing item/file, 422 invalid_config, 500 write/save failures, 503 core auth manager unavailable).

## Function/Class Map (high level)
- **Entry point**: `cmd/server/main.go` (init logging/build info; parses flags; loads config/.env; selects token stores; starts server).
- **Config**: `internal/config/config.go` (Config struct, LoadConfig/LoadConfigOptional, cloud deploy support).
- **API server**: `internal/api/server.go` (Server, NewServer, RegisterRoutes, Start) using Gin middleware/handlers/modules.
- **Auth flows**: `internal/cmd/*login.go` for Google/Gemini, Claude, Codex, Qwen, iFlow, Antigravity; token storage under `auths/`.
- **Executors**: provider handlers in `internal/runtime/executor/*` (Claude, Gemini, Codex, Kiro, iFlow, OpenAI-compatibility, AI Studio) with streaming/fallback logic.
- **Translators**: `internal/translator/*` (Claude↔OpenAI, Gemini↔OpenAI, Codex↔OpenAI, Kiro translator) sanitize/control-char stripping, tool manifest/clamping, plan/tool-choice handling.
- **Stores**: `sdk/auth/filestore.go`, `internal/store/postgresstore.go`, `internal/store/gitstore.go`, `internal/store/objectstore.go` for token persistence.
- **Registry**: `internal/registry/model_registry.go` registers models, picks executors, routes requests.
- **Middleware**: CORS, auth, request logging, rate limiting in `internal/api/middleware/*`.
- **SDK**: `sdk/cliproxy` (CliProxy service, Pipeline, ProviderRegistry, Watcher) for embedding proxy.
- **Management handlers**: `internal/api/handlers/management/*` implement endpoints described above (config access, stats, health, logging, auth-files, OAuth URLs).
- **Amp module**: `internal/api/modules/amp/*` for Amp CLI/IDE integration (proxy, secrets).
- **Utilities**: `internal/util/*` (log level, auth dir resolution, writable path, proxy helpers, Gemini thinking budget), `internal/logging` setup.
- **Hotspots**: Model registry coupling, main initialization, translator/executor streaming and tool handling; 20MB SSE buffers and sanitized payloads for Claude/Kiro parity.
