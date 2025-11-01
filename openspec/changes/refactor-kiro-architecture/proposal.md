# Refactor Kiro Provider Architecture

## Summary
This proposal refactors the Kiro provider to align with CLIProxyAPI's established architecture while maintaining token-based authentication as a first-class option. The refactoring addresses the monolithic executor (1,115+ lines) and missing dedicated translation layer by introducing proper separation of concerns and consistent patterns.

## Problems Addressed
- **Monolithic executor**: The current `kiro_executor.go` contains 1,115+ lines handling multiple responsibilities
- **Missing translation layer**: Kiro handles translations directly in the executor instead of using dedicated translator patterns
- **Configuration inconsistency**: Kiro uses separate token files instead of integrated config management
- **Architectural deviation**: Doesn't follow established patterns used by other providers

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