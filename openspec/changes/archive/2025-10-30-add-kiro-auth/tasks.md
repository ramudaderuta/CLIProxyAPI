## 1. Authentication Infrastructure
- [x] 1.1 Create `internal/auth/kiro/` package structure
- [x] 1.2 Implement `kiro_token.go` for token data structures and file loading
- [x] 1.3 Implement `kiro_auth.go` for authentication logic and token refresh
- [x] 1.4 Add Kiro token storage interface implementation

## 2. Request Processing
- [x] 2.1 Create `internal/runtime/executor/kiro_executor.go` for Kiro API requests
- [x] 2.2 Implement request/response translation in `internal/translator/kiro/`
- [x] 2.3 Add streaming support for Kiro chat completions
- [x] 2.4 Implement error handling for Kiro-specific error codes

## 3. Model Integration
- [x] 3.1 Update `internal/registry/model_definitions.go` with Kiro model mappings
- [x] 3.2 Add Kiro model constants and validation
- [x] 3.3 Update model registry to include Kiro in unified models endpoint

## 4. Configuration and Integration
- [x] 4.1 Update `cmd/server/main.go` to include Kiro authentication
- [x] 4.2 Add Kiro configuration options to config structures
- [x] 4.3 Integrate Kiro auth manager with existing access management

## 5. Testing and Validation
- [x] 5.1 Create unit tests for Kiro authentication and token refresh
- [x] 5.2 Test chat completions with all supported Kiro models
- [x] 5.3 Verify streaming responses work correctly
- [x] 5.4 Test error scenarios and token expiration handling

## 6. Documentation and Cleanup
- [x] 6.1 Update CLAUDE.md with Kiro provider information
- [x] 6.2 Remove or archive the original `./kiro/` prototype code
- [x] 6.3 Update README.md with Kiro model support information