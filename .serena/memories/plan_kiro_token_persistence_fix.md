# Task Plan: Fix Kiro Token Refresh Persistence

## P - Plan (Technical Design)
**Objective**: Enable automatic Kiro token updates by persisting refreshed tokens to disk.
*   **Technical Approach**: 
    *   Integrate `TokenManager` into `KiroExecutor` to handle token loading, rotation, and persistence.
    *   Modify `TokenManager.GetNextToken` to return the `TokenEntry` (containing the file path) instead of just the storage.
    *   Update `KiroExecutor.Execute` to use `TokenManager` for retrieving tokens and to persist tokens to the file path upon successful refresh (e.g. after 401).
*   **Target Symbols**: 
    *   `internal/auth/kiro/token_manager.go`: `GetNextToken`
    *   `internal/runtime/executor/kiro_executor.go`: `KiroExecutor` struct, `NewKiroExecutor`, `Execute`

## D - Do (Task Checklist)
*   [ ] T001 [P] [Auth] Update `TokenManager.GetNextToken` signature
    *   DoD: `GetNextToken` returns `(*TokenEntry, error)`; updated tests in `tests/unit/kiro/kiro_token_manager_rotation_test.go` and `tests/unit/kiro/kiro_concurrency_test.go`.
*   [ ] T002 [P] [Executor] Integrate `TokenManager` into `KiroExecutor`
    *   DoD: `KiroExecutor` struct has `tokenManager` field; `NewKiroExecutor` initializes and loads tokens.
*   [ ] T003 [P] [Executor] Update `Execute` to use `TokenManager`
    *   DoD: `Execute` calls `GetNextToken`; uses `entry.Path` to persist token after 401 refresh; falls back to `auth.Metadata` if `TokenManager` has no tokens.
*   [ ] T004 [P] [Executor] Update `ExecuteStream` to use `TokenManager`
    *   DoD: `ExecuteStream` implements the same logic as `Execute`.

## C - Check (Test & Regression)
*   **New Test Cases**:
    *   Verify that `KiroExecutor` correctly loads tokens from disk.
    *   Verify that `KiroExecutor` persists refreshed tokens to disk after a 401.
*   **Regression Strategy**:
    *   Run existing Kiro unit tests: `go test ./tests/unit/kiro/...`
    *   Run integration tests: `go test ./tests/integration/kiro/...`

## A - Act (Impact & Standardization)
*   **Impact Analysis**: 
    *   `KiroExecutor` will now depend on `TokenManager`.
    *   `GetNextToken` signature change requires updating all callers (mostly tests).
*   **Standardization**:
    *   Update `kiro-refresh-implementation.md` to reflect the use of `TokenManager` and file persistence.
