# AIClient-2-API Kiro Implementation Notes

Summary of Kiro (Claude via Kiro OAuth) support in `justlovemaki/AIClient-2-API`, gathered for quick reference.

## Configuration & Provider Identity
- Provider key: `claude-kiro-oauth` (`MODEL_PROVIDER.KIRO_API`), protocol prefix `claude`.
- CLI flags / config: `--kiro-oauth-creds-base64`, `--kiro-oauth-creds-file`; falls back to `~/.aws/sso/cache/kiro-auth-token.json`.
- Default provider list normalized in `config-manager.js`; prompt logging and provider pools supported. Region defaults to `us-east-1` if missing.
- Provider pool aware: `service-manager.js` + `ProviderPoolManager`; unhealthy providers are marked on failures.

## Adapter & Service Wiring
- `adapter.js` registers `KiroApiServiceAdapter` under `MODEL_PROVIDER.KIRO_API`.
- Adapter defers to `KiroApiService` (in `src/claude/claude-kiro.js`) for generate/list/stream and token refresh; lazy initializes and retries on 403 with forced refresh.

## Auth & Tokens (`claude-kiro.js`)
- Loads credentials in order: Base64 JSON (constructor) → explicit file path → default dir merge of JSON files; writes refreshed tokens back to the chosen path.
- Refresh endpoints: `https://prod.{region}.auth.desktop.kiro.dev/refreshToken` (social) or `https://oidc.{region}.amazonaws.com/token` (non-social). Base URLs templated by region; Amazon Q uses a different streaming URL.
- Access token refresh on demand (forceRefresh flag) and when near expiry via `isExpiryDateNear` using `CRON_NEAR_MINUTES`.
- MAC SHA256 is embedded in UA headers; system proxy can be disabled via `USE_SYSTEM_PROXY_KIRO`.

## Request Build
- Accepts OpenAI-style messages; `buildCodewhispererRequest` maps to CodeWhisperer `conversationState`.
- System prompt applied/merged into first user message or standalone; supports tool results, images, and tools (as toolSpecification).
- Model mapping table maps user model names to CodeWhisperer IDs; default model `claude-opus-4-5`.

## Response Handling
- Kiro backend is non-streaming; `_processApiResponse` parses SSE/legacy event blobs, extracts text and tool calls, deduplicates `[Called ... with args:{...}]` blocks, cleans them from text.
- Builds Claude-compatible responses:
  - Non-stream: `message` with `content` blocks (text or `tool_use`), `stop_reason` `tool_use` when tool calls exist.
  - Pseudo-stream: emits Claude event sequence (`message_start`, `content_block_start/delta/stop`, `message_delta`, `message_stop`), sets stop_reason `tool_use` if tool calls present.
- Tool call arguments are JSON-repaired when possible; duplicates skipped.

## Model List
- `listModels` returns `{ models: [{ name: <MODEL_MAPPING key> }, ...] }` for discovery of mapped model names.

## Retry & Robustness
- Axios timeout 120s; request retry with exponential backoff for 429/5xx (configurable via `REQUEST_MAX_RETRIES`/`REQUEST_BASE_DELAY`).
- On stream/unary errors, provider pool entry is marked unhealthy.
