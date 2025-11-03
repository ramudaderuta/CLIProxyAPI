# Kiro Provider Tests

This directory contains comprehensive tests for the Kiro provider refactoring in CLIProxyAPI.

## Test Structure

```
tests/
├── kiro_integration_test.go          # Original integration tests
├── kiro_performance_test.go          # Performance benchmarking tests
├── kiro_config_test.go              # Configuration and token loading tests
├── kiro_component_integration_test.go # Component integration tests
├── kiro_backward_compatibility_test.go # Backward compatibility tests
├── fixtures/                        # Test data and fixtures
│   ├── kiro_token_native.json       # Native Kiro token (without type field)
│   ├── kiro_token_enhanced.json     # Enhanced Kiro token (with type field)
│   ├── kiro_token_expired.json      # Expired token for refresh testing
│   ├── kiro_token_invalid.json      # Invalid token for error testing
│   ├── kiro_test_config_basic.yaml  # Basic configuration
│   ├── kiro_test_config_with_paths.yaml # Configuration with explicit token paths
│   └── kiro_test_config_multiple.yaml # Configuration with multiple token files
└── mocks/                           # Mock implementations for testing
    ├── kiro_auth_mocks.go           # Authentication mocks
    ├── kiro_client_mocks.go         # HTTP client mocks
    └── kiro_translator_mocks.go     # Translator mocks
```

## Test Categories

### 1. Configuration Tests (`kiro_config_test.go`)

Tests for the new token file configuration approach:
- Token file precedence (configured paths > auto-detection)
- Token file format enhancement for native Kiro tokens
- Dual token loading logic
- Configuration validation

### 2. Component Integration Tests (`kiro_component_integration_test.go`)

Tests for the interaction between decomposed components:
- Translator + Client + Executor coordination
- Error propagation across components
- Token refresh integration
- Concurrent execution

### 3. Backward Compatibility Tests (`kiro_backward_compatibility_test.go`)

Tests to ensure existing functionality continues to work:
- Native Kiro token files (without "type": "kiro")
- Enhanced Kiro token files (with "type": "kiro")
- Migration path from auto-detection to explicit configuration
- Mixed configuration scenarios

### 4. Performance Tests (`kiro_performance_test.go`)

Benchmark tests for performance validation:
- Single execution performance
- Concurrent execution performance
- Memory usage characteristics

## Running Tests

### Run All Kiro Tests
```bash
go test ./tests -v
```

### Run Specific Test Categories
```bash
# Configuration tests
go test ./tests/kiro_config_test.go -v

# Component integration tests
go test ./tests/kiro_component_integration_test.go -v

# Backward compatibility tests
go test ./tests/kiro_backward_compatibility_test.go -v

# Performance tests
go test ./tests/kiro_performance_test.go -bench=.
```

### Run with Coverage
```bash
go test ./tests -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

## Test Data

### Token Files

1. **Native Token** (`/home/build/code/CLIProxyAPI/tmp/kiro-test/kiro-auth-token.json`)
   - Missing `"type": "kiro"` field
   - Should be automatically enhanced in memory

2. **Enhanced Token** (`/home/build/.cli-proxy-api/kiro-auth-token.json`)
   - Contains `"type": "kiro"` field
   - Should work without modification

## Test Requirements

- Valid Kiro token files in the specified locations
- Go 1.19 or higher
- All dependencies installed via `go mod tidy`

## Maintaining Tests

1. **Update Fixtures**: When API changes affect token formats, update the fixture files accordingly
2. **Add New Test Cases**: For new functionality, add corresponding test cases in the appropriate test file
3. **Validate Coverage**: Ensure new code is covered by tests
4. **Run Performance Tests**: Validate that performance characteristics are maintained

## Troubleshooting

### Missing Token Files
Ensure the test token files exist at the expected locations:
- `/home/build/code/CLIProxyAPI/tmp/kiro-test/kiro-auth-token.json`
- `/home/build/.cli-proxy-api/kiro-auth-token.json`

### Test Failures
1. Check that token files contain valid data
2. Verify configuration file paths are correct
3. Ensure network connectivity for integration tests
4. Check for race conditions in concurrent tests