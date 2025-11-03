## Why
Integrate Kiro AI provider support into CLIProxyAPI by refactoring the tested JavaScript prototype code, enabling users to access Kiro Claude Sonnet 4.5 models through the existing unified API interface.

## What Changes
- Refactor the tested JavaScript prototype from `./kiro/` directory to Go implementation in `internal/auth/kiro/`
- Implement Kiro authentication using imported `kiro-auth-token.json` file (no online OAuth flow)
- Add support for Kiro models: claude-sonnet-4-5, claude-sonnet-4-5-20250929, claude-sonnet-4-20250514, claude-3-7-sonnet-20250219, amazonq-claude-sonnet-4-20250514, amazonq-claude-3-7-sonnet-20250219
- Integrate Kiro token refresh mechanism using existing refreshToken
- Add Kiro executor and translator components following existing patterns
- Update model registry to include Kiro model mappings

## Impact
- Affected specs: New kiro-auth capability
- Affected code: `internal/auth/kiro/` (new), `internal/runtime/executor/kiro_executor.go` (new), `internal/translator/kiro/` (new), `internal/registry/model_definitions.go` (modified), `cmd/server/main.go` (modified)