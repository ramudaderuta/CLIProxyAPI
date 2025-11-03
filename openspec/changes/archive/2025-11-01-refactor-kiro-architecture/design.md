# Kiro Provider Refactoring Design

## Architectural Analysis

### Current State Issues
1. **Monolithic Executor (1,115+ lines)**
   - Mixes authentication, translation, and execution concerns
   - Difficult to test individual components
   - High cognitive load for maintenance

2. **Missing Translation Layer**
   - Other providers use `internal/translator/[provider]/` pattern
   - Kiro handles translation directly in executor
   - Inconsistent with established architecture

3. **Configuration Isolation**
   - Uses separate `kiro-auth-token.json` files
   - Not integrated into main `config.yaml` system
   - Breaks unified configuration management

### Design Principles

#### 1. Maintain Token-First Authentication
- Token auth is a valid first-class authentication method
- Create standardized token auth patterns
- Support both config-based and file-based token loading

#### 2. Follow Established Patterns
- Use same package structure as other providers
- Implement dedicated translator layer
- Separate concerns properly

#### 3. Backward Compatibility
- Existing `kiro-auth-token.json` files must work
- No breaking API changes
- Gradual migration path

## Proposed Architecture

### Component Separation

```
internal/
├── auth/kiro/           # Token authentication (existing)
├── translator/kiro/     # NEW: Dedicated translation layer
│   ├── request.go       # OpenAI → Kiro translation
│   ├── response.go      # Kiro → OpenAI translation
│   └── models.go        # Model mapping logic
├── runtime/executor/
│   ├── kiro_executor.go # REFACTORED: Core execution only
│   └── kiro_client.go   # NEW: HTTP client management
└── config/
    └── config.go        # EXTENDED: Kiro config support
```

### Configuration Integration

#### Option 1: Token File Path Configuration (Primary)
```yaml
kiro-token-file:
  - token-file-path: "/path/to/kiro-auth-token.json"
    region: "us-east-1"
```

#### Option 2: Auto-Detection Fallback (Legacy)
- Auto-detect `kiro-auth-token.json` in auth-dir
- Load tokens when no config present
- Maintain backward compatibility

#### Option 3: Native Token File Support
- Support native Kiro token exports (without "type":"kiro")
- Automatically enhance in memory with type information
- Preserve original file format on disk

### Translation Layer Design

Follow established patterns from other providers:
- **Request Translator**: OpenAI format → Kiro internal format
- **Response Translator**: Kiro format → OpenAI format
- **Model Mapper**: OpenAI model names → Kiro model IDs

### Executor Decomposition

Split monolithic executor into focused components:

1. **Core Executor** (~200 lines)
   - Request routing and basic validation
   - Response coordination
   - Error handling

2. **HTTP Client** (~150 lines)
   - Kiro API communication
   - Token management
   - Retry logic

3. **Translation Integration** (~100 lines)
   - Use dedicated translators
   - Format conversion coordination

## Migration Strategy

### Phase 1: Configuration Integration
- Add Kiro config structures
- Implement dual loading (config + file)
- Maintain existing file loading

### Phase 2: Extract Translation Layer
- Create `internal/translator/kiro/` package
- Move translation logic from executor
- Update executor to use translators

### Phase 3: Decompose Executor
- Split into focused components
- Extract HTTP client logic
- Simplify core executor

## Benefits

1. **Maintainability**: Smaller, focused components
2. **Testability**: Individual component testing
3. **Consistency**: Follows established patterns
4. **Extensibility**: Easier to add new features
5. **Developer Experience**: Familiar architecture

## Trade-offs

1. **Initial Complexity**: More files/packages initially
2. **Migration Effort**: Requires careful refactoring
3. **Testing Overhead**: Need comprehensive test coverage

## Success Criteria

- [ ] Executor reduced from 1,115+ to ~200 lines
- [ ] Dedicated translation layer implemented
- [ ] Token file path configuration integration completed
- [ ] Native token file format enhancement implemented
- [ ] All existing tests pass
- [ ] Backward compatibility maintained (with and without "type":"kiro")
- [ ] Code follows established patterns
- [ ] Automatic type enhancement works transparently