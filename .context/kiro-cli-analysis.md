# Kiro CLI Analysis Report

## Executive Summary

Kiro CLI is an AWS-based AI assistant that provides access to Claude models through social authentication (GitHub/Google) and AWS Builder ID/Identity Center. It's a comprehensive CLI tool with rich tooling capabilities, agent management, and conversation persistence.

---

## 1. Tool Overview

### 1.1 Basic Information
- **Version**: 1.20.0
- **Binary**: `/home/build/.local/bin/kiro-cli` (105MB executable)
- **Architecture**: Rust-based CLI tool with multiple subcommands
- **Authentication**: Token-based via AWS Builder ID/Identity Center with social login (GitHub/Google)
- **Data Storage**: SQLite database at `~/.local/share/kiro-cli/data.sqlite3`

### 1.2 Command Structure
```bash
kiro-cli [OPTIONS] [SUBCOMMAND]
```

**Core Subcommands:**
- `chat` - AI assistant in terminal (primary interface)
- `agent` - Manage AI agents (profiles/configs)
- `login/logout` - Authentication management
- `whoami` - User authentication status
- `settings` - Configuration management
- `mcp` - Model Context Protocol support
- `translate` - Natural Language to Shell translation

---

## 2. Authentication Mechanism

### 2.1 Authentication Methods
- **Social Login**: GitHub, Google OAuth
- **AWS Builder ID**: Free tier authentication
- **Identity Center**: Pro tier with enterprise integration

### 2.2 OAuth Flow
```bash
# Social authentication
kiro-cli login --social google
kiro-cli login --social github

# Identity Center (Pro)
kiro-cli login --license pro --identity-provider <URL> --region <REGION>

# Builder ID (Free)
kiro-cli login --license free
```

### 2.3 Token Structure (from SQLite database)
```json
{
  "access_token": "eyJraWQiOiJrZXkt...",
  "expires_at": "2025-11-22T14:54:26.849048581Z",
  "refresh_token": "aorAAAAAGmTRt...",
  "region": "us-east-1",
  "oauth_flow": "DeviceCode",
  "scopes": [
    "codewhisperer:completions",
    "codewhisperer:analysis", 
    "codewhisperer:conversations"
  ]
}
```

### 2.4 Key Authentication Findings
- **Device Code Flow**: Primary OAuth mechanism (`DeviceCode` flow)
- **Scopes**: Limited to CodeWhisperer services
- **Token Storage**: Encrypted SQLite database with key-value storage
- **Auto-refresh**: Tokens appear to be refreshed automatically
- **Regional**: Authentication tied to AWS regions (us-east-1 observed)

---

## 3. Supported Models

### 3.1 Available Models
- **Auto**: Automatic model selection (default)
- **claude-sonnet-4.5**: Latest Claude Sonnet model
- **claude-sonnet-4**: Claude Sonnet 4 model  
- **claude-haiku-4.5**: Claude Haiku 4.5 model

### 3.2 Model Selection
```bash
kiro-cli chat --model Auto "Your prompt"           # Automatic selection
kiro-cli chat --model claude-sonnet-4.5 "Your prompt"
kiro-cli chat --model claude-haiku-4.5 "Your prompt"
```

### 3.3 Model Information (from conversation data)
```json
{
  "model_name": "Auto",
  "description": "Models chosen by task for optimal usage and consistent quality",
  "model_id": "auto",
  "context_window_tokens": 200000,
  "rate_multiplier": 1.0,
  "rate_unit": "credit"
}
```

---

## 4. API Structure & Capabilities

### 4.1 Chat Interface
```bash
kiro-cli chat [OPTIONS] [INPUT]
```

**Key Options:**
- `--agent <AGENT>` - Context profile to use
- `--model <MODEL>` - Model selection
- `--no-interactive` - Non-interactive mode (API-friendly)
- `--trust-all-tools` - Allow tool usage without confirmation
- `--trust-tools <NAMES>` - Trust specific tools only
- `--wrap <always|never|auto>` - Line wrapping behavior

### 4.2 Response Format
- **Interactive Mode**: Colored output with ASCII art branding
- **Non-Interactive Mode**: Clean text output suitable for API consumption
- **Streaming**: Real-time response streaming with timing metrics
- **Metadata**: Shows model, plan information, and usage statistics

### 4.3 Tool System (Complete List from conversation data)
Kiro provides a comprehensive tool system with the following capabilities:

1. **AWS Integration**
   - `use_aws` - Make AWS CLI API calls
   - Support for all AWS services and operations

2. **File System Operations**
   - `fs_read` - Read files, directories, images with multiple modes
   - `fs_write` - Create, edit files (create, append, str_replace, insert)
   - Batch operations supported

3. **System Integration**
   - `execute_bash` - Execute bash commands
   - `introspect` - Query Kiro CLI capabilities and documentation

4. **Support Tools**
   - `report_issue` - GitHub issue reporting with context
   - `dummy` - Tool fallback for missing/invalid tools

### 4.4 Context Management
- **Profile-based**: Multiple agent configurations
- **File Context**: Automatic file discovery and context inclusion
- **Conversation History**: Persistent conversation state
- **MCP Support**: Model Context Protocol integration

### 4.5 Request/Response Structure

**User Request Format:**
```json
{
  "user": {
    "additional_context": "",
    "env_context": {
      "env_state": {
        "operating_system": "linux",
        "current_working_directory": "/home/build/code/CLIProxyAPI"
      }
    },
    "content": {
      "Prompt": {
        "prompt": "What is 2+2?"
      }
    },
    "timestamp": "2025-11-22T21:57:14.268837474+08:00"
  }
}
```

**Assistant Response Format:**
```json
{
  "assistant": {
    "Response": {
      "message_id": "6b13c602-53df-4a09-aef5-f9a5c548946a",
      "content": "2+2 = 4"
    }
  },
  "request_metadata": {
    "request_id": "b0072d08-ddfd-4401-a6d1-7050ce285238",
    "model_id": "auto",
    "time_to_first_chunk": {
      "secs": 3,
      "nanos": 332819678
    },
    "response_size": 7,
    "chat_conversation_type": "NotToolUse"
  }
}
```

---

## 5. Configuration Management

### 5.1 Settings System
```bash
# Configuration commands
kiro-cli settings list           # List all settings
kiro-cli settings <KEY> [VALUE] # Get/set specific settings
kiro-cli settings open          # Open settings file
kiro-cli settings --format json # JSON output format
```

### 5.2 Configuration Files
- **Global Settings**: `~/.kiro/settings/cli.json`
- **Agent Configs**: `~/.kiro/agents/` directory
- **Database**: `~/.local/share/kiro-cli/data.sqlite3`

### 5.3 Agent System
```bash
kiro-cli agent list         # List available agents
kiro-cli agent create       # Create new agent config
kiro-cli agent edit         # Edit existing agent
kiro-cli agent set-default  # Set default agent
```

**Available Agents:**
- `kiro_default` - Default configuration

---

## 6. Streaming Capabilities

### 6.1 Real-time Streaming
- **Chunked Responses**: Data streamed in real-time
- **Timing Metrics**: First chunk time and inter-chunk intervals
- **Progress Indicators**: Visual feedback during generation

### 6.2 Streaming Metadata
```json
{
  "request_start_timestamp_ms": 1763819834270,
  "stream_end_timestamp_ms": 1763819837684,
  "time_to_first_chunk": {
    "secs": 3,
    "nanos": 332819678
  },
  "time_between_chunks": [
    {"secs": 0, "nanos": 8576},
    {"secs": 0, "nanos": 80865432},
    {"secs": 0, "nanos": 35598}
  ]
}
```

---

## 7. Rate Limits & Quotas

### 7.1 Usage Tracking
- **Credits**: Usage measured in credits
- **Rate Multiplier**: 1.0x for standard usage
- **Model-specific**: Different rates per model tier

### 7.2 Usage Information
```json
{
  "usage_info": [
    {
      "value": 0.018731205140961858,
      "unit": "credit",
      "unit_plural": "credits"
    }
  ]
}
```

---

## 8. Security & Privacy

### 8.1 Data Protection
- **Encrypted Storage**: Tokens stored in encrypted SQLite database
- **Local Processing**: Configuration stored locally
- **Scope Limitation**: Limited to CodeWhisperer services

### 8.2 Authentication Security
- **Device Code Flow**: Secure OAuth mechanism
- **Token Refresh**: Automatic token refresh
- **Regional Isolation**: Authentication tied to specific AWS regions

---

## 9. Integration Recommendations for CLIProxyAPI

### 9.1 Authentication Layer
```go
type KiroAuthenticator struct {
    clientID     string
    clientSecret string
    region       string
    scopes       []string
}

// OAuth Device Code Flow
func (a *KiroAuthenticator) Authenticate(ctx context.Context) (*KiroToken, error)
func (a *KiroAuthenticator) RefreshToken(ctx context.Context, refreshToken string) (*KiroToken, error)
```

### 9.2 Request Translation
- **OpenAI → Kiro**: Convert OpenAI chat completion format to Kiro conversation state
- **History Management**: Map OpenAI message history to Kiro conversation format
- **Tool Integration**: Translate OpenAI function calls to Kiro tool specifications

### 9.3 Response Translation
- **Kiro → OpenAI**: Convert Kiro responses to OpenAI chat completion format
- **Streaming**: Handle Kiro's streaming chunks and convert to SSE format
- **Tool Calls**: Map Kiro tool results back to OpenAI function call responses

### 9.4 Key Integration Points

1. **Executor Implementation**: Create `KiroExecutor` that:
   - Uses stored OAuth tokens for authentication
   - Translates OpenAI requests to Kiro conversation format
   - Handles streaming responses and converts to SSE
   - Manages tool call translation

2. **Token Storage**: Implement `KiroTokenStore` that:
   - Reads/writes Kiro's SQLite database format
   - Handles token rotation and refresh
   - Supports multiple accounts/regions

3. **Translator**: Create `KiroTranslator` that:
   - Maps message formats between OpenAI and Kiro
   - Handles tool specification conversion
   - Manages conversation state persistence

### 9.5 Recommended Implementation Strategy

1. **Phase 1**: Basic chat completion support
2. **Phase 2**: Streaming capabilities
3. **Phase 3**: Tool/function calling support
4. **Phase 4**: Multi-account and token rotation
5. **Phase 5**: Agent and context management integration

---

## 10. Technical Challenges & Considerations

### 10.1 Authentication Complexity
- **OAuth Device Flow**: More complex than simple API keys
- **Token Management**: Requires refresh logic and expiration handling
- **Multi-region Support**: Need to handle different AWS regions

### 10.2 Data Format Differences
- **Conversation State**: Kiro uses complex conversation state vs simple message arrays
- **Tool Systems**: Different tool specification formats
- **Streaming**: Kiro's streaming format differs from OpenAI SSE

### 10.3 Tool Integration
- **Rich Tool System**: Kiro has extensive tool capabilities that need mapping
- **AWS Integration**: Many tools are AWS-specific
- **Permission Management**: Tool trust and confirmation systems

### 10.4 Performance Considerations
- **Startup Time**: Kiro CLI has initialization overhead
- **Context Loading**: Agent and file context loading may impact performance
- **Database Access**: SQLite database access for token retrieval

---

## Conclusion

Kiro CLI represents a sophisticated AI assistant with enterprise-grade features, comprehensive tooling, and robust authentication mechanisms. For CLIProxyAPI integration, the primary challenges will be:

1. **Authentication**: Implementing OAuth device code flow with proper token management
2. **Translation**: Converting between OpenAI's simple message format and Kiro's complex conversation state
3. **Tool Integration**: Mapping between OpenAI function calls and Kiro's extensive tool system
4. **Streaming**: Handling Kiro's streaming format and converting to SSE

The integration is technically feasible but requires careful attention to authentication, state management, and format conversion. Kiro's rich feature set and enterprise capabilities make it a valuable addition to CLIProxyAPI's supported providers.
