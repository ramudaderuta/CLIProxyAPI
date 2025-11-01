# Refactor Kiro Provider Architecture

## Summary
This proposal refactors the Kiro provider to align with CLIProxyAPI's established architecture while maintaining token-based authentication as a first-class option. The refactoring addresses the monolithic executor (1,115+ lines) and missing dedicated translation layer by introducing proper separation of concerns and consistent patterns.

## Why
The current Kiro provider implementation suffers from significant architectural issues that impact maintainability, testability, and consistency with the rest of the CLIProxyAPI codebase:

1. **Maintainability**: The monolithic executor with 1,115+ lines is difficult to maintain, debug, and extend
2. **Code quality**: Mixing translation logic directly in the executor violates single responsibility principle
3. **Consistency**: Other providers follow established patterns that Kiro currently ignores
4. **Testing**: Large monolithic components are difficult to unit test effectively
5. **Onboarding**: New developers struggle with inconsistent patterns across providers

## Problems Addressed
- **Monolithic executor**: The current `kiro_executor.go` contains 1,115+ lines handling multiple responsibilities
- **Missing translation layer**: Kiro handles translations directly in the executor instead of using dedicated translator patterns
- **Configuration inconsistency**: Kiro uses separate token files instead of integrated config management
- **Architectural deviation**: Doesn't follow established patterns used by other providers

## What Changes
This refactoring will implement the following structural changes:

1. **Create dedicated Kiro translator** (`internal/translator/kiro_translator.go`)
   - Extract all request/response translation logic from the executor
   - Implement standard translator interface used by other providers
   - Handle Kiro-specific message formatting and model mapping

2. **Decompose monolithic executor** into focused components:
   - `internal/runtime/executor/kiro_executor.go` - Core request execution logic
   - `internal/runtime/executor/kiro_client.go` - HTTP client and API communication
   - `internal/runtime/executor/kiro_token_manager.go` - Token validation and refresh

3. **Integrate configuration management**:
   - Add Kiro configuration to main `config.yaml` structure
   - Support `kiro-token-file` entries for explicit token file paths
   - Maintain backward compatibility with existing `kiro-auth-token.json` files
   - Update `internal/watcher/watcher.go` for proper token file monitoring

4. **Standardize token authentication**:
   - Document token-based authentication as a first-class pattern
   - Create reusable token management utilities
   - Ensure consistent error handling for token-related issues

## Constraints
- **Token-only authentication**: Kiro cannot support OAuth and must remain token-based
- **Backward compatibility**: Existing `kiro-auth-token.json` files must continue working
- **No breaking changes**: API compatibility must be maintained

## Capabilities
1. **Standardize Token Authentication**: Establish token-based auth as a first-class pattern
2. **Extract Translation Layer**: Create dedicated Kiro translator following established patterns
3. **Decompose Executor**: Split monolithic executor into focused, testable components
4. **Integrate Configuration**: Add Kiro to main config system while supporting legacy token files

## Relationships
This refactoring builds on the existing `kiro-auth` specification and extends it to address architectural concerns.