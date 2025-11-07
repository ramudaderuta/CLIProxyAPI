<!-- START -->
# Claude Code: Best Practices for Effective Collaboration  
  
This document outlines best practices for working with Claude Code to ensure efficient and successful software development tasks.  
  
## Task Management  
  
For complex or multi-step tasks, Claude Code will use:  
*   **TodoWrite**: To create a structured task list, breaking down the work into manageable steps. This provides clarity on the plan and allows for tracking progress.  
*   **TodoRead**: To review the current list of tasks and their status, ensuring alignment and that all objectives are being addressed.  
  
## File Handling and Reading  
  
Understanding file content is crucial before making modifications.  
  
1.  **Targeted Information Retrieval**:  
    *   When searching for specific content, patterns, or definitions within a codebase, prefer using search tools like `Grep` or `Task` (with a focused search prompt). This is more efficient than reading entire files.  
  
2.  **Reading File Content**:  
    *   **Small to Medium Files**: For files where full context is needed or that are not excessively large, the `Read` tool can be used to retrieve the entire content.  
    *   **Large File Strategy**:  
        1.  **Assess Size**: Before reading a potentially large file, its size should be determined (e.g., using `ls -l` via the `Bash` tool or by an initial `Read` with a small `limit` to observe if content is truncated).  
        2.  **Chunked Reading**: If a file is large (e.g., over a few thousand lines), it should be read in manageable chunks (e.g., 1000-2000 lines at a time) using the `offset` and `limit` parameters of the `Read` tool. This ensures all content can be processed without issues.  
    *   Always ensure that the file path provided to `Read` is absolute.  
  
## File Editing  
  
Precision is key for successful file edits. The following strategies lead to reliable modifications:  
  
1.  **Pre-Edit Read**: **Always** use the `Read` tool to fetch the content of the file *immediately before* attempting any `Edit` or `MultiEdit` operation. This ensures modifications are based on the absolute latest version of the file.  
  
2.  **Constructing `old_string` (The text to be replaced)**:  
    *   **Exact Match**: The `old_string` must be an *exact* character-for-character match of the segment in the file you intend to replace. This includes all whitespace (spaces, tabs, newlines) and special characters.  
    *   **No Read Artifacts**: Crucially, do *not* include any formatting artifacts from the `Read` tool's output (e.g., `cat -n` style line numbers or display-only leading tabs) in the `old_string`. It must only contain the literal characters as they exist in the raw file.  
    *   **Sufficient Context & Uniqueness**: Provide enough context (surrounding lines) in `old_string` to make it uniquely identifiable at the intended edit location. The "Anchor on a Known Good Line" strategy is preferred: `old_string` is a larger, unique block of text surrounding the change or insertion point. This is highly reliable.  
  
3.  **Constructing `new_string` (The replacement text)**:  
    *   **Exact Representation**: The `new_string` must accurately represent the desired state of the code, including correct indentation, whitespace, and newlines.  
    *   **No Read Artifacts**: As with `old_string`, ensure `new_string` does *not* contain any `Read` tool output artifacts.  
  
4.  **Choosing the Right Editing Tool**:  
    *   **`Edit` Tool**: Suitable for a single, well-defined replacement in a file.  
    *   **`MultiEdit` Tool**: Preferred when multiple changes are needed within the same file. Edits are applied sequentially, with each subsequent edit operating on the result of the previous one. This tool is highly effective for complex modifications.  
  
5.  **Verification**:  
    *   The success confirmation from the `Edit` or `MultiEdit` tool (especially if `expected_replacements` is used and matches) is the primary indicator that the change was made.  
    *   If further visual confirmation is needed, use the `Read` tool with `offset` and `limit` parameters to view only the specific section of the file that was changed, rather than re-reading the entire file.  
  
### Reliable Code Insertion with MultiEdit  
  
When inserting larger blocks of new code (e.g., multiple functions or methods) where a simple `old_string` might be fragile due to surrounding code, the following `MultiEdit` strategy can be more robust:  
  
1.  **First Edit - Targeted Insertion Point**: For the primary code block you want to insert (e.g., new methods within a class), identify a short, unique, and stable line of code immediately *after* your desired insertion point. Use this stable line as the `old_string`.  
    *   The `new_string` will consist of your new block of code, followed by a newline, and then the original `old_string` (the stable line you matched on).  
    *   Example: If inserting methods into a class, the `old_string` might be the closing brace `}` of the class, or a comment that directly follows the class.  
  
2.  **Second Edit (Optional) - Ancillary Code**: If there's another, smaller piece of related code to insert (e.g., a function call within an existing method, or an import statement), perform this as a separate, more straightforward edit within the `MultiEdit` call. This edit usually has a more clearly defined and less ambiguous `old_string`.  
  
**Rationale**:  
*   By anchoring the main insertion on a very stable, unique line *after* the insertion point and prepending the new code to it, you reduce the risk of `old_string` mismatches caused by subtle variations in the code *before* the insertion point.  
*   Keeping ancillary edits separate allows them to succeed even if the main insertion point is complex, as they often target simpler, more reliable `old_string` patterns.  
*   This approach leverages `MultiEdit`'s sequential application of changes effectively.  
  
**Example Scenario**: Adding new methods to a class and a call to one of these new methods elsewhere.  
*   **Edit 1**: Insert the new methods. `old_string` is the class's closing brace `}`. `new_string` is `  
    [new methods code]  
    }`.  
*   **Edit 2**: Insert the call to a new method. `old_string` is `// existing line before call`. `new_string` is `// existing line before call  
    this.newMethodCall();`.  
  
This method provides a balance between precise editing and handling larger code insertions reliably when direct `old_string` matches for the entire new block are problematic.  
  
## Handling Large Files for Incremental Refactoring  

When refactoring large files incrementally rather than rewriting them completely:  

1. **Initial Exploration and Planning**:  
   * Begin with targeted searches using `rg` to locate specific patterns or sections within the file.  
   * Use `Bash` commands like `rg "pattern" file` to find line numbers for specific areas of interest.  
   * Create a clear mental model of the file structure before proceeding with edits.  

2. **Chunked Reading for Large Files**:  
   * For files too large to read at once, use multiple `Read` operations with different `offset` and `limit` parameters.  
   * Read sequential chunks to build a complete understanding of the file.  
   * Use `rg` to pinpoint key sections, then read just those sections with targeted `offset` parameters.  

3. **Finding Key Implementation Sections**:  
   * Use `Bash` commands with `rg -A N` (to show N lines after a match) or `rg -B N` (to show N lines before) to locate function or method implementations.  
   * Example: `rg "function findTagBoundaries" -A 20 filename.js` to see the first 20 lines of a function.  

4. **Pattern-Based Replacement Strategy**:  
   * Identify common patterns that need to be replaced across the file.  
   * Use the `Bash` tool with `sed` for quick previews of potential replacements.  
   * Example: `sed -n "s/oldPattern/newPattern/gp" filename.js` to preview changes without making them.  

5. **Sequential Selective Edits**:  
   * Target specific sections or patterns one at a time rather than attempting a complete rewrite.  
   * Focus on clearest/simplest cases first to establish a pattern of successful edits.  
   * Use `Edit` for well-defined single changes within the file.  

6. **Batch Similar Changes Together**:  
   * Group similar types of changes (e.g., all references to a particular function or variable).  
   * Use `rg` to preview the scope of batch changes: `rg "pattern" filename.js | wc -l`  
   * For systematic changes across a file, consider using `sed` through the `Bash` tool: `sed -i "s/oldPattern/newPattern/g" filename.js`  

7. **Incremental Verification**:  
   * After each set of changes, verify the specific sections that were modified.  
   * For critical components, read the surrounding context to ensure the changes integrate correctly.  
   * Validate that each change maintains the file's structure and logic before proceeding to the next.  

8. **Progress Tracking for Large Refactors**:  
   * Use the `TodoWrite` tool to track which sections or patterns have been updated.  
   * Create a checklist of all required changes and mark them off as they're completed.  
   * Record any sections that require special attention or that couldn't be automatically refactored.  

## Shell & CLI Tools

Claude Code must leverage the comprehensive suite of installed command-line tools to augment built-in capabilities for maximum efficiency, precision, and flexibility.

### Tool Selection Strategy

For operations where command-line tools provide superior performance or precision compared to file system abstractions, Claude Code will prefer the `Bash` tool. This includes bulk text processing, data transformation, system introspection, build operations, and validation tasks.

### Core Text Processing and Data Parsing

1.  **`ripgrep (rg)`**: For all codebase searches, Claude Code will use `rg` instead of `grep`. It respects `.gitignore` by default and provides faster, more readable output.
    *   Locate definitions: `rg "function\s+\w+" --line-number`
    *   Contextual search: `rg "class\s+User" -A 20 -B 5` (20 lines after, 5 lines before match)
    *   Find TODOs: `rg "TODO|FIXME" --line-number | awk -F: '{print $1}' | sort | uniq -c`

2.  **`jq`/`yq`**: For processing structured data files (JSON/YAML) and API responses.
    *   Extract values: `yq eval '.dependencies.react.version' package.json`
    *   Chain with curl: `curl -s https://api.github.com/repos/owner/repo | jq '.stargazers_count'`
    *   Update YAML: `yq eval '.services.app.image = "new:tag"' -i docker-compose.yml`

3.  **`sed`**: For batch replacements across files where `MultiEdit` would be inefficient.
    *   **Always preview first**: `sed -n 's/oldPattern/newPattern/gp' filename.js`
    *   **Apply changes**: `sed -i 's/oldPattern/newPattern/g' filename.js`
    *   Target specific line ranges: `sed -i '100,200s/foo/bar/g' large_file.cpp`

4.  **`awk`**: For columnar data processing and pattern-based extraction.
    *   Parse logs: `awk '$4 > 100 {print $1, $5}' metrics.log`
    *   Field extraction: `awk '{print $2}' file.txt | sort | uniq -c`

### Development and Build Toolchain

1.  **`cmake`/`ninja`**: For C/C++ projects, Claude Code will use these directly rather than abstracted build commands.
    *   Configure: `cmake -B build -G Ninja -DCMAKE_BUILD_TYPE=Release -DCMAKE_EXPORT_COMPILE_COMMANDS=ON`
    *   Build: `ninja -C build -j$(nproc)`
    *   Clean rebuild: `rm -rf build && cmake -B build -G Ninja && ninja -C build`

2.  **`llvm-config`**: To obtain correct LLVM compilation flags dynamically.
    *   **Always use**: `llvm-config --cxxflags --ldflags --libs core`
    *   Never hardcode LLVM paths or versions.

3.  **`pkg-config`**: For obtaining library compiler/linker flags.
    *   Standard pattern: `pkg-config --cflags --libs opencv4`
    *   Check availability: `pkg-config --exists libname && echo "Found" || echo "Missing"`

4.  **`cargo`**: For Rust operations, prefer direct cargo commands.
    *   Check: `cargo check --workspace --all-targets`
    *   Build: `cargo build --release`
    *   Test: `cargo test --workspace`

### Language Compiler Direct Usage

1.  **`go`**: Use `go build`, `go test`, `go mod tidy` for Go projects.
2.  **`gcc`/`g++`**: For quick C/C++ compilation tests or single-file builds: `g++ -std=c++20 -Wall -Wextra file.cpp -o file`
3.  **`clang`/`clang++`**: Prefer for modern C++ features and better diagnostics.
4.  **`rustc`/`rustfmt`**: Use for individual file compilation or formatting verification.

### Network and System Operations

1.  **`docker`**: For container and image management.
    *   Build: `docker build -t app:latest .`
    *   Compose operations: `docker compose up -d --build`
    *   Inspect metadata: `docker inspect container_name | jq '.[0].State.Status'`
    *   Clean up: `docker system prune -f`

2.  **`curl`**: For HTTP requests and file downloads.
    *   API testing: `curl -X POST -H "Content-Type: application/json" -d '{"key":"value"}' https://api.example.com`
    *   Download with redirect: `curl -L -o archive.tar.gz https://example.com/file.tar.gz`
    *   Header inspection: `curl -I https://example.com`

3.  **`sqlite3`**: For database inspection and quick queries.
    *   Execute: `sqlite3 database.db "SELECT COUNT(*) FROM migrations;"`
    *   Export: `sqlite3 database.db ".mode csv" ".output data.csv" "SELECT * FROM table;"`

4.  **`netcat (nc)`**: For network connectivity testing.
    *   Port test: `nc -zv localhost 3000`
    *   Service check: `echo "test" | nc -w 2 localhost 8080`

### File and System Introspection

1.  **`lsof`**: To identify processes using files or ports.
    *   Port conflict resolution: `lsof -i :3000`
    *   Open files by process: `lsof -p $(pgrep -f process_name)`

2.  **`tree`**: To visualize directory structure.
    *   JSON output: `tree -J -L 2 | jq '.[0].contents'`

3.  **`fd-find (fdfind)`**: To locate files by pattern.
    *   Find source files: `fd -e cpp -e hpp src/`
    *   Find directories: `fd -t d -L 2`

4.  **`openssl`**: For certificate and cryptographic operations.
    *   Generate self-signed: `openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes`

### Effective Usage Patterns

1.  **Preview Before Apply**: Claude Code will always preview destructive operations before execution.
    *   Use `--dry-run`, `-n` flags, or redirect to stdout first to verify behavior.

2.  **Tool Composition**: Chain tools for complex operations.
    *   Process grep results: `rg "pattern" --line-number | awk -F: '{print $1}' | sort | uniq -c`

3.  **Output Filtering**: Narrow results before processing with Claude's tools.
    *   Filter containers: `docker ps --format json | jq '. | select(.State == "running")'`

4.  **Batch Processing**: Use loops for repetitive operations.
    *   Clean temporary files: `for file in $(fd -e .tmp); do rm "$file"; done`

5.  **Dynamic Values**: Use command substitution for dynamic data.
    *   Parallel builds: `ninja -C build -j$(nproc)`

### Package Runtime Tools (npx/uvx)

For isolated execution of ecosystem tools without global installation:

1.  **`npx` (Node.js)**:
    *   Run linters: `npx eslint . --fix`
    *   Execute scaffolding: `npx create-vite@latest my-project -- --template react`
    *   One-off tools: `npx http-server -p 8080`
    *   Version pinning: `npx playwright@1.40.0 test`

2.  **`uvx` (Python)**:
    *   Format code: `uvx ruff format .`
    *   Type checking: `uvx --with mypy-extensions mypy src/`
    *   Run scripts with deps: `uvx --with requests --with pydantic python script.py`
    *   Tool installation: `uvx --reinstall tool-name`

## Leveraging Subagents and Skills

Claude Code **MUST** proactively leverage installed subagents and skills as primary mechanisms for specialized tasks, ensuring maximum efficiency and consistent application of expertise. Direct implementation should be considered secondary to delegation when a relevant subagent or skill is available.

### Delegation Priority Rule

When a task aligns with any subagent or skill description, Claude Code will delegate first and only implement directly as a fallback.

**Verification After Delegation:**
- Confirm which subagent(s)/skill(s) were invoked
- Summarize the specialized expertise applied
- Explain how outputs integrate with the task

**Failure Handling:**
If delegation fails:
1. Analyze the failure reason
2. Attempt alternative subagents with overlapping capabilities
3. Document the limitation before direct implementation

### Subagent Utilization Protocol

**Automatic Delegation Triggers:**
Claude Code will automatically invoke appropriate subagents when detecting:
- **Architecture & Design Requests**: Any mention of API design, system architecture, or pattern selection → `backend-architect`, `graphql-architect`, `architecture-patterns`
- **Security & Performance**: Security audits, vulnerability scanning, or performance optimization → `security-auditor`, `performance-engineer`
- **Debugging & Diagnostics**: Error investigation or root cause analysis → `debugger`, `error-detective`
- **Testing Requirements**: Test generation, test coverage analysis, or testing strategy → `test-automator`, `tdd-orchestrator`
- **Technology-Specific Tasks**: Language/framework-specific implementations (FastAPI, Django, Rust, Go) → `fastapi-pro`, `django-pro`, `rust-pro`, `golang-pro`
- **Deployment & Operations**: Deployment, CI/CD setup, or infrastructure → `deployment-engineer`, `devops-troubleshooter`

**Explicit Invocation Patterns:**
Claude Code will recognize and act on explicit natural language cues:
- "Have the code-reviewer subagent analyze this PR"
- "Use the debugger to investigate this stack trace"
- "Get the backend-architect to design this authentication API"
- "Run the security-auditor on this module"

### Prompt for Task Delegation

Effective delegation requires intentionally structured prompts that clearly communicate task boundaries, success criteria, and integration points. Use these patterns to maximize subagent effectiveness:

**1. Explicit Context Transfer**
When delegating, always provide:
- **Current state**: What has been done so far
- **Specific goal**: What needs to be accomplished
- **Constraints**: Non-negotiable requirements (performance, security, style)
- **Integration points**: How results connect to the broader system

**2. Actionable Task Framing**

Ineffective prompts lack specificity, context, and clear success criteria, leading to suboptimal delegation. Effective framing transforms vague requests into structured, executable directives:

1. **Code Review Requests**
   - **Ineffective pattern**: Generic requests like "Review this code" without scope or focus areas
   - **Effective transformation**: Specify analysis scope, target areas, and required output format
   - **Key elements**: Explicit scope (security + API design), quantified output (3+ recommendations), structured format (severity levels)

2. **Bug Investigation Delegation**
   - **Ineffective pattern**: "Fix this bug" without reproduction context or process definition
   - **Effective transformation**: Provide reproduction data, define root cause analysis steps, and require verification procedures
   - **Key elements**: Multi-step process with tool specifications, quantitative alternatives (2+ solutions), safety-first approach

3. **Test Generation Tasks**
   - **Ineffective pattern**: "Write tests" without coverage requirements or framework context
   - **Effective transformation**: Define coverage targets, testing frameworks, and specific edge cases to validate
   - **Key elements**: Specific coverage targets (happy path + edge cases), framework constraints (pytest fixtures), technical requirements (async patterns)

4. **Performance Optimization**
   - **Ineffective pattern**: Ambiguous goals like "Make it faster" without measurement criteria
   - **Effective transformation**: Specify profiling tools, validation methodology, and measurable success targets
   - **Key elements**: Named profiling tool (cProfile), quantified validation (3+ iterations), specific performance target (>20% improvement)

### Skill Activation Strategy

When requirements match installed skill descriptions, Claude Code automatically activates the relevant skills without explicit prompting.

**Activation Examples:**
- Python async code → `async-python-patterns` + `python-testing-patterns`
- Database queries → `sql-optimization-patterns`
- API implementation → `api-design-principles` + `auth-implementation-patterns`
- Git operations → `git-advanced-workflows`
- Error handling → `error-handling-patterns`
- Code review → `code-review-excellence`

When activating skills, Claude Code will acknowledge them: "Activating `[skill-name]` for [specific aspect]."

## General Interaction  
  
Claude Code will directly apply proposed changes and modifications using the available tools, rather than describing them and asking you to implement them manually. This ensures a more efficient and direct workflow.  
<!-- END -->

# CLIProxyAPI - AI Model Proxy Server

## Architecture Overview

CLIProxyAPI is a Go-based HTTP proxy server that provides unified OpenAI/Gemini/Claude-compatible API interfaces for multiple AI providers. It abstracts provider differences and handles authentication, translation between API formats, and request routing.

### Core Components

- **API Server** (`internal/api/`): HTTP server with management endpoints and middleware
- **Authentication** (`internal/auth/`): OAuth flows for Claude, Codex, Gemini, Qwen, iFlow, and token-based auth for Kiro
- **Translators** (`internal/translator/`): Bidirectional API format conversion between providers (Claude↔Gemini, Claude↔OpenAI, Codex↔Claude, etc.)
- **Executors** (`internal/runtime/executor/`): Provider-specific request handlers with streaming support
- **Token Stores** (`internal/store/`): Multiple backends (Postgres, Git, S3-compatible, filesystem) for auth persistence
- **Registry** (`internal/registry/`): Model registration and routing logic

### Supported Providers

- **Claude** (Anthropic OAuth)
- **Codex** (Custom OAuth)
- **Gemini** (Google OAuth + API keys)
- **Qwen** (Alibaba OAuth)
- **iFlow** (Custom OAuth)
- **Kiro** (Token-based auth)
- **OpenAI-compatible** (Custom endpoints like OpenRouter)

### Project Structure

```
cmd/server/            # Application entrypoint
internal/
├── api/              # HTTP server, handlers, middleware
├── auth/             # Provider-specific authentication
├── runtime/executor/ # Request execution engines
├── translator/       # API format translation
├── store/           # Token persistence backends
├── config/          # Configuration management
└── interfaces/      # Shared interfaces and types
tests/
├── unit/            # Unit tests
├── integration/     # End-to-end tests
├── regression/      # Bug regression tests
└── benchmarks/      # Performance benchmarks
sdk/                 # Public Go SDK
```

## Quick Command Reference

```bash
# Build
go build -o cli-proxy-api ./cmd/server

# Run tests
go test ./tests/unit/... ./tests/regression/... -race -cover -v
go test ./tests/unit/kiro -run 'Executor' -v                    # Specific domain
go test -tags=integration ./tests/integration/... -v              # Integration tests
go test ./tests/benchmarks/... -bench . -benchmem -run ^$       # Benchmarks

# Update golden files
go test ./tests/unit/... -run 'SSE|Translation' -v -update

# Start server
./cli-proxy-api --config config.test.yaml
```

## Configuration

Main config file (`config.yaml`):
- **Port**: Server listening port (default: 8317)
- **Auth Directory**: Location for provider auth files (`~/.cli-proxy-api`)
- **API Keys**: Authentication for proxy access
- **Provider Configs**: API keys, OAuth settings, custom endpoints
- **Management API**: Remote management interface (disabled by default)
- **Proxy Settings**: HTTP/SOCKS5 proxy support
- **Quota Management**: Automatic project/model switching on limits

## API Usage

### Endpoint
```
POST http://localhost:8317/v1/messages
```

### Authentication
Header: `Authorization: Bearer your-api-key`

### Request Format
OpenAI-compatible JSON with provider-specific extensions:
- `thinking`: Claude reasoning configuration
- `tools`: Function calling support
- `stream`: Server-sent events

### Example Request
```json
{
    "model": "claude-sonnet-4-5-20250929",
    "temperature": 0.5,
    "max_tokens": 1024,
    "stream": false,
    "thinking": { "type": "enabled", "budget_tokens": 4096 },
    "system": [
      { "type": "text", "text": "You are Claude Code.", "cache_control": { "type": "ephemeral" } }
    ],
    "tools": [
      {
        "name": "get_weather",
        "description": "Get current weather by city name",
        "input_schema": {
          "type": "object",
          "properties": {
            "city": { "type": "string" },
            "unit": { "type": "string", "enum": ["°C","°F"] }
          },
          "required": ["city"]
        }
      }
    ],
    "messages": [
      {
        "role": "user",
        "content": [{ "type": "text", "text": "Tell me how many degrees now in Tokyo?" }]
      }
    ]
}
```

## Provider Authentication

### OAuth Providers (Claude, Codex, Gemini, Qwen, iFlow)
```bash
./cli-proxy-api --claude-login    # Claude OAuth
./cli-proxy-api --codex-login     # Codex OAuth
./cli-proxy-api --login           # Gemini OAuth
./cli-proxy-api --qwen-login      # Qwen OAuth
./cli-proxy-api --iflow-login     # iFlow OAuth
```

### Kiro Token Auth
Place `kiro-auth-token.json` in auth directory or configure via `kiro-token-file`.

## Development

### Test Data
- **Golden Files**: `tests/shared/golden/` - Expected API responses
- **Test Payloads**: `tests/shared/testdata/` - Sample requests
- **Shared Utils**: `tests/shared/` - Common testing utilities

### Key Files for Changes
- **New Provider**: Add to `internal/auth/`, `internal/runtime/executor/`, `internal/translator/`
- **API Endpoints**: Modify `internal/api/handlers/`
- **Configuration**: Update `internal/config/` and `config.example.yaml`
- **Models Registration**: Update `internal/registry/model_registry.go`

## Deployment

### Environment Variables
- `DEPLOY=cloud`: Cloud deployment mode
- `PGSTORE_*`: PostgreSQL backend configuration
- `GITSTORE_*`: Git backend configuration
- `OBJECTSTORE_*`: S3-compatible backend configuration

### Management API
- **Endpoint**: `/v0/management/*`
- **Authentication**: Requires `secret-key` configuration
- **Features**: Config updates, usage stats, log viewing
- **Control Panel**: Built-in web UI (disable with `disable-control-panel: true`)

### Debug Commands
```bash
# Check configuration
./cli-proxy-api --config config.yaml --debug

# Test authentication
curl -H "Authorization: Bearer test-api-key-1234567890" \
     -H "Content-Type: application/json" \
     -d '{"model":"claude-sonnet-4-5-20250929","messages":[{"role":"user","content":[{"type":"text","text":"test"}]}]}' \
     http://localhost:8317/v1/messages

# View logs
tail -f ~/.cli-proxy-api/logs/*.log
```