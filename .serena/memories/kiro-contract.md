# Kiro ⇄ CLIProxyAPI Translation & Execution Contract

## 1. Scope & Goals

This document defines the **protocol contract** between:

*   The **OpenAI-style front-end** (Claude Code / CLIProxyAPI clients),
*   The **Kiro provider** (request/response + streaming behavior).

It captures:

1.  **Translation layer** behavior (`BuildRequest`, `ParseResponse`) which maps OpenAI/Anthropic/OpenAI Responses messages to Kiro's `conversationState` and back (OpenAI Chat, Claude Messages, OpenAI Responses; non-stream + stream).
2.  **Execution layer** behavior (`KiroExecutor`), including token rotation and fallback mechanisms for "Improperly formed request" errors.
3.  **Authentication layer** behavior (`KiroAuth`, `KiroTokenStorage`) for OAuth token management.
4.  **Streaming layer** behavior (`ConvertKiroStreamToAnthropic`, event stream decoding) for SSE stream conversion.
5.  **Defensive helpers** (`safeParseJSON`, `SanitizeToolCallID`, `FilterThinkingContent`) that ensure robustness.

The goal is that **Kiro never sees an "improperly formed request"** and that the executor gracefully recovers if it does.

---

## 2. Core Types

### 2.1 Authentication Layer Design

#### KiroAuthenticator (internal/auth/kiro/auth.go)
```go
type KiroAuthenticator struct {
    clientID     string
    clientSecret string
    region       string
    scopes       []string
    tokenStore   TokenStore
    httpClient   *http.Client
}

// Authenticate initiates OAuth device code flow
func (a *KiroAuthenticator) Authenticate(ctx context.Context, license LicenseType, options *LoginOptions) (*KiroToken, error)

// RefreshToken handles token refresh
func (a *KiroAuthenticator) RefreshToken(ctx context.Context, refreshToken string) (*KiroToken, error)

// ValidateToken checks if token is valid and not expired
func (a *KiroAuthenticator) ValidateToken(ctx context.Context, token *KiroToken) (bool, error)
```

#### OAuth Device Code Flow (internal/auth/kiro/oauth.go)
```go
type DeviceCodeFlow struct {
    authURL      string
    tokenURL     string
    clientID     string
    scopes       []string
}

type DeviceCodeResponse struct {
    DeviceCode      string `json:"device_code"`
    UserCode        string `json:"user_code"`
    VerificationURI string `json:"verification_uri"`
    ExpiresIn       int    `json:"expires_in"`
    Interval        int    `json:"interval"`
}

// StartDeviceFlow initiates device code authentication
func (f *DeviceCodeFlow) StartDeviceFlow(ctx context.Context) (*DeviceCodeResponse, error)

// PollForToken polls for token completion
func (f *DeviceCodeFlow) PollForToken(ctx context.Context, deviceCode string) (*KiroToken, error)
```

#### Token Storage (internal/auth/kiro/token_store.go)
```go
type TokenStore interface {
    // SaveToken stores Kiro token in SQLite format compatible with kiro-cli
    SaveToken(ctx context.Context, token *KiroToken) error
    
    // LoadToken retrieves stored Kiro token
    LoadToken(ctx context.Context) (*KiroToken, error)
    
    // DeleteToken removes stored token
    DeleteToken(ctx context.Context) error
    
    // ListTokens returns all stored tokens for multi-account support
    ListTokens(ctx context.Context) ([]*KiroToken, error)
}
```

### 2.2 Translation Functions

Kiro translators are registered in the shared registry (FormatKiro) and support OpenAI Chat, Claude Messages, and OpenAI Responses:

```go
// BuildRequest converts an OpenAI/Anthropic/Responses payload into Kiro conversationState.
func BuildRequest(
    model string,
    payload []byte,
    token *authkiro.KiroTokenStorage,
    metadata map[string]any,
) ([]byte, error)

// ParseResponse extracts assistant text and tool calls from a Kiro upstream payload (JSON or SSE).
func ParseResponse(data []byte) (string, []OpenAIToolCall)
```

Streaming translators emit the target schema:
* OpenAI Chat (`ConvertKiroStreamChunkToOpenAI`)
* Claude Messages (`ConvertKiroStreamChunkToClaude`)
* OpenAI Responses (`ConvertKiroResponseToOpenAIResponsesStream`)

### 2.3 Kiro Conversation Schema

Kiro expects a `conversationState` object:

```json
{
  "conversationState": {
    "chatTriggerType": "MANUAL",
    "conversationId": "uuid...",
    "currentMessage": {
      "userInputMessage": {
        "content": "...",
        "modelId": "...",
        "origin": "AI_EDITOR",
        "userInputMessageContext": {
           "tools": [...],
           "toolResults": [...],
           "toolContextManifest": [...],
           "claudeToolChoice": {...},
           "planMode": {...}
        },
        "toolUses": [...],
        "images": [...]
      }
    },
    "history": [
      { "userInputMessage": { ... } },
      { "assistantResponseMessage": { ... } }
    ]
  },
  "profileArn": "...",
  "projectName": "..."
}
```

---

## 3. Authentication & Token Management

### 3.1 KiroAuth

The `KiroAuth` component handles OAuth authentication flows:

```go
type KiroAuth struct {}

// GetAuthenticatedClient configures and returns an HTTP client ready for authenticated API calls
func (auth *KiroAuth) GetAuthenticatedClient(
    ctx context.Context, 
    ts *KiroTokenStorage, 
    proxyURL string
) (*http.Client, error)

// refreshToken refreshes the access token using the refresh token
// Handles both social and IDC authentication methods
func (auth *KiroAuth) refreshToken(
    ctx context.Context, 
    ts *KiroTokenStorage, 
    proxyURL string
) error

// ValidateToken checks if the current token is valid and not expired
func (auth *KiroAuth) ValidateToken(ts *KiroTokenStorage) bool
```

**Contract:**
*   Supports both "social" (GitHub OAuth) and "IdC" (AWS Builder ID) authentication methods.
*   Automatically refreshes tokens if near expiration (within 5-minute buffer).
*   Validates token structure before use.

### 3.2 KiroTokenStorage

Token storage structure for Kiro authentication:

```go
type KiroTokenStorage struct {
    AccessToken  string    `json:"accessToken"`   // OAuth access token (required)
    RefreshToken string    `json:"refreshToken"`  // Token for refresh (required)
    ProfileArn   string    `json:"profileArn"`    // AWS profile ARN (required)
    ExpiresAt    time.Time `json:"expiresAt"`     // Expiration timestamp (required)
    AuthMethod   string    `json:"authMethod"`    // "social" or "IdC" (optional)
    Provider     string    `json:"provider"`      // OAuth provider, e.g., "Github", "BuilderId" (optional)
}

// IsExpired checks if token is expired or will expire within 5 minutes
func (ts *KiroTokenStorage) IsExpired() bool

// SaveTokenToFile serializes token storage to JSON file
func (ts *KiroTokenStorage) SaveTokenToFile(authFilePath string) error
```

**Contract:**
*   Tokens are considered expired if they expire within 5 minutes (safety buffer).
*   Required fields: `AccessToken`, `RefreshToken`, `ProfileArn`, `ExpiresAt`
*   Additional fields like `clientIdHash`, `region` may be present and are preserved
*   Files are saved with 0700 permissions for security.

### 3.3 Token Rotation

The `kiroTokenRotator` manages multiple token files for automatic failover:

```go
type kiroTokenRotator struct {
    entries []kiroRotatorEntry
    cursor  uint32
}

// candidates returns token candidates in round-robin order
func (r *kiroTokenRotator) candidates() []kiroTokenCandidate

// advance moves to the next token after a failed attempt
func (r *kiroTokenRotator) advance(idx int)
```

**Contract:**
*   Tokens are tried in round-robin order starting from current cursor position.
*   After failed attempt, cursor advances to next token.
*   Supports token discovery in auth directory.
*   Configured via `KiroTokenFiles` in config with optional region and label per token.

---

## 4. Defensive Helpers

### 4.1 `safeParseJSON`

**Purpose:** Robustly parse JSON-ish strings that may contain truncated escape sequences (`\`, `\u`, etc.).

**Contract:**
*   Never panic or return an error.
*   Sanitize obviously malformed tails (dangling backslashes, incomplete `\u` escapes).
*   Prefer **parsed JSON**; fall back to **original string**.
*   Returns `map[string]any{}` if parsed value is `null`.

### 4.2 `SanitizeToolCallID`

**Purpose:** Ensure tool call IDs are never empty, as Kiro (and downstream tools) requires them.

**Contract:**
*   Trims whitespace.
*   If empty, generates a `call_<uuid>` ID.

### 4.3 `FilterThinkingContent`

**Purpose:** Remove "Thinking" sections from Kiro responses to prevent leaking internal monologues.

**Contract:**
*   Removes content between `<thinking>` tags (case-insensitive).
*   Preserves actual response content before, between, and after thinking sections.
*   Handles various thinking block formats.

---

## 5. Request Translation (OpenAI → Kiro)

The `BuildRequest` function transforms OpenAI/Anthropic messages into Kiro's `conversationState`.

### 5.1 Message Extraction

Messages are processed using `extractUserMessage` and `extractAssistantMessage` which handle Anthropic-style content blocks:

*   **Text**: Extracted and joined with newlines.
*   **Tool Use**: Mapped to `toolUses` array with `toolUseId`, `name`, and `input`.
*   **Tool Result**: Mapped to `toolResults` array (status "success", content wrapped in text).
*   **Images**: Mapped to `images` array (base64 format with source bytes).

### 5.2 Tool Specifications

**Explicit Tools:**
Tools are converted from OpenAI/Anthropic format to Kiro's `toolSpecification` format:

```go
{
  "toolSpecification": {
    "name": "tool_name",
    "description": "...",  // Truncated to 256 chars if needed
    "inputSchema": {
      "json": { /* JSON schema */ }
    }
  }
}
```

**Synthetic Tools:**
If no explicit tools are provided but the transcript contains `tool_use` blocks, minimal tool specifications are synthesized:
*   `collectToolUseNames` scans messages for referenced tool names.
*   `buildSyntheticToolSpecifications` creates minimal specs with generic object schema.
*   Prevents "Improperly formed request" errors from missing tool definitions.

**Tool Description Handling:**
*   Descriptions longer than 256 characters are truncated.
*   Full descriptions are hashed (SHA-256, first 8 bytes) and tracked in `toolContextManifest`.
*   Angle-bracket blocks (`<...>`) are stripped from descriptions.
*   Empty descriptions default to `"Tool <name>"`.

### 5.3 History Construction

1.  **System Prompt**: If present, it is prepended to the **first user message** in history.
2.  **Tool Event Preservation**: `tool_use` and `tool_result` blocks are preserved in the history items (`userInputMessage` or `assistantResponseMessage`).
3.  **Role Ordering**:
    *   `user` → `userInputMessage`
    *   `assistant` → `assistantResponseMessage`
    *   `system`/`tool` → `userInputMessage`

### 5.4 Current Message Construction

The `currentMessage` is always a `userInputMessage`.

*   **Content**: Must be non-empty. If the user message only contains tool results, content is collapsed from tool results or `.` is injected.
*   **Context**:
    *   `tools`: Derived from `tools` definition or synthesized from `tool_use` history if missing.
    *   `toolResults`: From the current turn.
    *   `toolContextManifest`: Hashed descriptions of truncated tools.
    *   `claudeToolChoice`: Metadata from `tool_choice` parameter.
    *   `planMode`: Metadata for plan-based tool usage.

### 5.5 Edge Case Handling

To avoid "Improperly formed request" errors:

1.  **Trailing User Tool Results**: If the last messages are user `tool_result`s (without new text), they are **moved to history**. The `currentMessage` becomes a synthetic "continue" turn with `.` content.
2.  **Trailing Assistant Tool Uses**: If the history ends with assistant `tool_use`s that haven't been responded to, they are attached to the `currentMessage`'s `toolUses` to maintain context.
3.  **Empty Content**: Kiro rejects empty content.
    *   User messages with only tool results get collapsed tool result text or `.` content.
    *   Assistant messages with only tool uses get `.` content.

### 5.6 Model Mapping

The `MapModel` function translates friendly model names to Kiro's internal identifiers:

```go
var modelMapping = map[string]string{
    "claude-sonnet-4-5": "CLAUDE_SONNET_4_5",
}
```

---

## 6. Response Translation (Kiro → OpenAI)

The `ParseResponse` function handles both JSON and SSE streams using a dependency injection architecture.

### 6.1 Architecture

**Interfaces:**

```go
// JSONProcessor provides unified interface for JSON parsing
type JSONProcessor interface {
    IsValidJSON(data []byte) bool
    ParseJSON(data []byte) gjson.Result
    ExtractJSONObjects(line string) []string
    SanitizeJSON(input string) string
    NormalizeArguments(args string) string
}

// ContentExtractor extracts content from JSON structures
type ContentExtractor interface {
    ExtractTextFromContent(result gjson.Result) string
    ExtractToolCallsFromContent(result gjson.Result) []OpenAIToolCall
}

// ResponseParser defines main parsing interface
type ResponseParser interface {
    ParseResponse(data []byte) (string, []OpenAIToolCall)
}
```

**Implementations:**
*   `KiroJSONProcessor` implements `JSONProcessor` using `gjson`
*   `KiroContentExtractor` implements `ContentExtractor` for Kiro responses
*   `KiroResponseParser` implements `ResponseParser` with dependency injection

### 6.2 Parsing Strategy

1.  **JSON**: Tries to parse as full JSON object.
    *   Extracts content from `conversationState.currentMessage.assistantResponseMessage.content`
    *   Falls back to `conversationState.history` (last assistant message)
    *   Tries Anthropic-style paths: `content`, `message.content`, `message`
2.  **SSE**: If JSON fails, parses as Event Stream.
    *   Handles `content_block_start`, `content_block_delta`, `content_block_stop`
    *   Accumulates `tool_use` parts across events
    *   Supports "Thinking" blocks (which are filtered out)
    *   Properly handles partial JSON streaming for tool arguments

### 6.3 SSE Event Processing

**SSE Architecture:**

```go
// SSEProcessingContext holds state for SSE processing
type SSEProcessingContext struct {
    TextBuilder   strings.Builder
    ToolOrder     []string
    ToolByID      map[string]*toolAccumulator
    ToolIndex     map[int]string
    JSONProcessor JSONProcessor
}

// SSEEventProcessor processes SSE events
type SSEEventProcessor interface {
    ProcessEvent(eventType string, node gjson.Result, context *SSEProcessingContext)
}
```

**Supported Event Types:**
*   `content_block_start`: Initialize text or tool blocks
*   `content_block_delta`: Accumulate text or tool argument fragments
*   `content_block_stop`: Finalize tool calls
*   `message_start`, `message_delta`, `message`: Extract message content
*   `text_delta`: Append text content
*   `input_json_delta`: Accumulate tool argument JSON fragments

**Tool Call Accumulation:**
*   Tool calls are tracked by ID across multiple events
*   `partial_json` fragments are concatenated for tool arguments
*   Tool calls are finalized when `content_block_stop` is received

### 6.4 Thinking Block Filtering

`FilterThinkingContent` removes "Thinking" sections from responses:

**Contract:**
*   Matches `<thinking>...</thinking>` tags (case-insensitive, multi-line)
*   Preserves content before, between, and after thinking blocks
*   Handles multiple thinking sections in one response
*   Cleans up excessive whitespace after removal

### 6.5 Stream Normalization

`NormalizeKiroStreamPayload` decodes Amazon Event Stream binary format:

**Contract:**
*   Detects Amazon event-stream encoded responses (binary envelope)
*   Extracts text-based SSE payload from binary frames
*   Each frame has: prelude (8 bytes) + headers + payload + CRC (4 bytes)
*   Returns original payload unchanged if not event-stream format
*   Filters out metering and context usage events

### 6.6 Output Format

*   **Chat Completion**: `BuildOpenAIChatCompletionPayload` creates a standard OpenAI JSON response with usage statistics.
*   **Streaming**: `BuildStreamingChunks` creates OpenAI-compatible SSE chunks:
    *   Initial chunk with `role: "assistant"`
    *   Content chunks with `delta.content`
    *   Tool call chunks with `delta.tool_calls`
    *   Final chunk with `finish_reason: "stop"`

### 6.7 Anthropic SSE Conversion

`ConvertKiroStreamToAnthropic` maps legacy Kiro SSE to Anthropic-compatible format:

**Contract:**
*   Parses Kiro-specific SSE frames
*   Generates Anthropic `message_start`, `content_block_start`, `content_block_delta`, `content_block_stop`, `message_delta`, `message_stop` events
*   Handles mixed text and tool blocks
*   Preserves streaming order and block indices
*   Includes token usage in final `message_delta` event

---

## 7. Execution Layer: KiroExecutor

The `KiroExecutor` manages the API calls and implements resilience.

### 7.1 Client Layer

The `kiroClient` handles low-level HTTP communication:

**Endpoint Selection:**
```go
// Base endpoint for Kiro models
kiroBaseURLTemplate = "https://codewhisperer.%s.amazonaws.com/generateAssistantResponse"

// Region extraction from profile ARN or override
// Defaults to us-east-1
```

**Headers:**
```go
// Standard headers
Content-Type: application/json
Accept: application/json
Authorization: Bearer <access_token>

// User agent headers with MAC hash for attribution
x-amz-user-agent: aws-sdk-js/1.0.7 KiroIDE-0.1.25-<mac_hash>
user-agent: aws-sdk-js/1.0.7 ua/2.1 os/cli lang/go api/codewhispererstreaming#1.0.7 m/E KiroIDE-0.1.25-<mac_hash>

// SDK metadata
amz-sdk-request: attempt=1; max=1
x-amzn-kiro-agent-mode: vibe
```

**MAC Hash:**
*   Computed once from first non-loopback network interface
*   SHA-256 hash of MAC address, first 16 hex chars used
*   Cached for performance

**Debug Logging:**
*   Dumps request/response payloads when debug mode is enabled
*   Truncates to 4096 bytes to prevent log flooding
*   Sanitizes binary/control characters for readability

### 7.2 Fallback Mechanism

If Kiro returns a **400 Bad Request** with "Improperly formed request", the executor attempts recovery:

1.  **Primary Attempt**: Sends the full, structured `conversationState` with tools and history.
2.  **Flattened Fallback**:
    *   Triggered if Primary fails with "Improperly formed request".
    *   Flattens `history` into a text-only transcript (removing structured `toolUses`/`toolResults`).
    *   Retains the structured `currentMessage` (with tools).
    *   Appends a note: "(Structured tool transcripts were flattened...)".
3.  **Minimal Fallback**:
    *   Triggered if Flattened fails.
    *   Clears `history` entirely.
    *   Summarizes the context into a single "Continue the previous task..." prompt in `currentMessage`.

**Contract:**
*   Only attempts flattened/minimal fallbacks for 400 status with "improperly formed" in response.
*   Each fallback preserves progressively less structure.
*   Logs warnings when fallbacks are used.

### 7.3 Token Selection & Rotation

The executor uses a multi-source token selection strategy:

**Token Sources (in order):**
1.  **Auth Directory Discovery**: Auto-discovered `kiro_*.json` files in auth directory
2.  **Rotator Configured Tokens**: Tokens from `config.KiroTokenFiles` in round-robin order
3.  **Metadata Token**: Token embedded in `auth.Metadata`

**Rotation Logic:**
*   If multiple tokens configured, tries each in round-robin order
*   Advances cursor after each attempt failure
*   Refreshes tokens if near expiration (5-minute buffer) before request
*   Attributes successful token path back to auth metadata for persistence

**Contract:**
*   Token refresh attempted before each request for expired/near-expired tokens
*   Failed authentication triggers advance to next token in rotation
*   Token path stored in metadata key `_kiro_token_path` for tracking

### 7.4 Streaming Support

The executor supports both non-streaming and streaming requests:

**Non-Streaming:**
*   Single HTTP request/response
*   Returns complete response after full completion
*   Processes JSON or SSE payload returned by Kiro

**Streaming:**
*   Opens persistent HTTP connection
*   Streams SSE events as they arrive
*   Builds Anthropic-compatible streaming chunks
*   Sends chunks to channel as received
*   Handles connection errors with graceful fallback

**Contract:**
*   Streaming uses same endpoint as non-streaming
*   Response may be JSON (one-shot) or SSE stream
*   SSE streams normalized via `NormalizeKiroStreamPayload`
*   Converted to Anthropic format via `ConvertKiroStreamToAnthropic`

### 7.5 Token Counting

The executor provides approximate token counting:

```go
func (e *KiroExecutor) CountTokens(
    ctx context.Context,
    auth *cliproxyauth.Auth,
    req cliproxyexecutor.Request,
    opts cliproxyexecutor.Options
) (cliproxyexecutor.Response, error)
```

**Estimation Method:**
*   **Prompt Tokens**: Character count / 4 (approximation)
*   **Completion Tokens**: Based on expected response size
*   No actual API call made, purely client-side estimation

---

## 8. Complete Flow Example

### 8.1 Request Flow

1.  **Client** sends OpenAI-compatible chat completion request
2.  **KiroExecutor** selects token from rotation pool
3.  **KiroExecutor** refreshes token if near expiration
4.  **BuildRequest** translates OpenAI/Anthropic messages to Kiro format:
    *   Extracts system prompt
    *   Builds tool specifications (or synthesizes if missing)
    *   Constructs history from all but last user message
    *   Moves trailing tool results to history
    *   Builds current user message with context
5.  **kiroClient** builds endpoint URL
6.  **kiroClient** extracts region from profile ARN or uses override
7.  **kiroClient** applies headers with MAC hash and bearer token
8.  **HTTP POST** to Kiro endpoint with conversation state
9.  If **400 "improperly formed"**: try flattened then minimal fallback
10. On success: Response returned to ParseResponse

### 8.2 Response Flow

1.  **kiroClient** receives response from Kiro
2.  **NormalizeKiroStreamPayload** detects and decodes event-stream format
3.  **ParseResponse** (or `KiroResponseParser`) determines payload type:
    *   **JSON**: Extract from `conversationState` or fallback paths
    *   **SSE**: Parse event stream with `SSEEventProcessor`
4.  **FilterThinkingContent** removes thinking blocks from text
5.  **Tool calls** extracted and deduplicated
6.  **BuildOpenAIChatCompletionPayload** or **BuildStreamingChunks** formats response
7.  Response returned to client

### 8.3 Streaming Flow

1.  Steps 1-8 same as Request Flow
2.  **HTTP response** is SSE stream (not JSON)
3.  **NormalizeKiroStreamPayload** decodes binary event-stream frames
4.  **ConvertKiroStreamToAnthropic** parses SSE and generates Anthropic events:
    *   `message_start`
    *   `content_block_start` (for each text/tool block)
    *   `content_block_delta` (incremental content/tool args)
    *   `content_block_stop` (finalize blocks)
    *   `message_delta` (usage stats)
    *   `message_stop`
5.  Events streamed to client as generated

---

## 9. Error Handling

### 9.1 Authentication Errors

*   **Token Expired**: Automatic refresh attempted before request
*   **Refresh Failed**: Advance to next token in rotation
*   **All Tokens Failed**: Return authentication error to client

### 9.2 Request Errors

*   **400 "improperly formed"**: Trigger fallback mechanism
*   **400 Other**: Return error to client
*   **401 Unauthorized**: Refresh token or advance rotation
*   **429 Rate Limit**: Return error to client (no retry)
*   **500+ Server Error**: Return error to client

### 9.3 Response Errors

*   **Invalid JSON**: Attempt SSE parsing
*   **Invalid SSE**: Extract plain text fallback
*   **Empty Response**: Return empty content (non-error)
*   **Truncated Stream**: Process accumulated content, log warning

---

## 10. Configuration

### 10.1 Token File Format

**Example 1: Social/GitHub OAuth**
```json
{
  "accessToken": "...",
  "refreshToken": "...",
  "profileArn": "arn:aws:codewhisperer:us-east-1:...:profile/...",
  "expiresAt": "2025-01-01T00:00:00Z",
  "authMethod": "social",
  "provider": "Github"
}
```

**Example 2: IdC/BuilderId OAuth**
```json
{
  "accessToken": "...",
  "refreshToken": "...",
  "profileArn": "arn:aws:codewhisperer:us-east-1:...:profile/...",
  "expiresAt": "2025-11-22T19:33:48.907Z",
  "clientIdHash": "e909a0580879b06ece1202964fbe9dda95ea4ce3",
  "authMethod": "IdC",
  "provider": "BuilderId",
  "region": "us-east-1"
}
```

**Notes:**
- Additional fields like `clientIdHash` and `region` are preserved but not required
- Only `accessToken`, `refreshToken`, `profileArn`, and `expiresAt` are strictly required

---

## 11. Implementation Notes

### 11.1 Thread Safety

*   `kiroTokenRotator.cursor` uses atomic operations for thread-safe rotation
*   `kiroClient.macHash` computed once with `sync.Once`
*   Each request gets independent context and state

### 11.2 Performance

*   MAC hash cached per client instance
*   Token validation cached until near-expiration
*   JSON parsing uses zero-copy `gjson` where possible
*   SSE streaming processes events incrementally (no full buffer)

### 11.3 Testing

The implementation includes comprehensive tests:
*   Unit tests for each translator function
*   Integration tests for full request/response cycles
*   Regression tests for specific bug scenarios
*   SSE parsing and streaming tests
*   Token rotation and failover tests
*   Thinking block filtering tests
