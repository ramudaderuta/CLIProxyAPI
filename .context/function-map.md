# CLIProxyAPI Function & Class Map

## Overview

This document provides a comprehensive mapping of functions, classes, and modules in the CLIProxyAPI repository, a Go-based HTTP proxy server that provides unified OpenAI/Gemini/Claude-compatible API interfaces for multiple AI providers.

## Core Architecture Components

### Main Application Entry Point

| Name | Type | Location | Summary | Key Dependencies |
|------|------|----------|---------|------------------|
| main | function | cmd/server/main.go:51-457 | Application entry point handling configuration, auth flows, and service startup | config, logging, auth, cmd |
| init | function | cmd/server/main.go:42-48 | Initializes base logger and build information | logging, buildinfo |

### Configuration Management

| Name | Type | Location | Summary | Key Dependencies |
|------|------|----------|---------|------------------|
| Config | struct | internal/config/config.go:21-120 | Main configuration struct with all application settings | sdk/config, bcrypt |
| LoadConfigOptional | function | internal/config/config.go:200-230 | Loads configuration file with optional cloud deployment mode | os, yaml, gopkg.in/yaml.v3 |
| LoadConfig | function | internal/config/config.go:180-198 | Loads configuration from file path | yaml, os |

### API Server Core

| Name | Type | Location | Summary | Key Dependencies |
|------|------|----------|---------|------------------|
| Server | struct | internal/api/server.go:45-85 | Main HTTP server with handlers and configuration | gin-gonic/gin, sync, atomic |
| NewServer | function | internal/api/server.go:87-140 | Creates and configures new server instance | gin, logging, auth, handlers |
| RegisterRoutes | function | internal/api/server.go:142-300 | Sets up all HTTP routes and middleware | middleware, modules, handlers |
| Start | function | internal/api/server.go:302-410 | Starts the HTTP server with graceful shutdown | http, net, context |

### Authentication System

| Name | Type | Location | Summary | Key Dependencies |
|------|------|----------|---------|------------------|
| DoLogin | function | internal/cmd/login.go:23-60 | Handles Google/Gemini OAuth login flow | oauth2, net/http, browser |
| DoClaudeLogin | function | internal/cmd/anthropic_login.go:20-56 | Handles Claude OAuth login flow | oauth2, net/http, browser |
| DoCodexLogin | function | internal/cmd/auth_manager.go:80-120 | Handles Codex OAuth login flow | oauth2, net/http, browser |
| DoQwenLogin | function | internal/cmd/qwen_login.go:20-56 | Handles Qwen OAuth login flow | oauth2, net/http, browser |
| DoIFlowLogin | function | internal/cmd/iflow_login.go:20-56 | Handles iFlow OAuth login flow | oauth2, net/http, browser |

### Runtime Executors (Provider-specific Request Handlers)

| Name | Type | Location | Summary | Key Dependencies |
|------|------|----------|---------|------------------|
| AIStudioExecutor | struct | internal/runtime/executor/aistudio_executor.go:30-80 | Handles AI Studio API requests with streaming support | net/http, context, json |
| ClaudeExecutor | struct | internal/runtime/executor/claude_executor.go:25-75 | Handles Claude API requests with OAuth | net/http, oauth2, json |
| GeminiExecutor | struct | internal/runtime/executor/gemini_executor.go:30-90 | Handles Gemini API requests with OAuth | net/http, oauth2, json |
| CodexExecutor | struct | internal/runtime/executor/codex_executor.go:25-70 | Handles Codex API requests | net/http, json |
| KiroExecutor | struct | internal/runtime/executor/kiro_executor.go:35-120 | Handles Kiro API requests with token auth | net/http, json, validation |
| IFlowExecutor | struct | internal/runtime/executor/iflow_executor.go:30-85 | Handles iFlow API requests with OAuth | net/http, oauth2, json |

### Translators (API Format Conversion)

| Name | Type | Location | Summary | Key Dependencies |
|------|------|----------|---------|------------------|
| ClaudeToOpenAI | function | internal/translator/claude/claude_openai.go:45-120 | Translates Claude API requests to OpenAI format | sdk/translator, json |
| OpenAIToClaude | function | internal/translator/claude/openai_claude.go:40-110 | Translates OpenAI API responses to Claude format | sdk/translator, json |
| GeminiToOpenAI | function | internal/translator/gemini/gemini_openai.go:50-130 | Translates Gemini API requests to OpenAI format | sdk/translator, json |
| OpenAIToGemini | function | internal/translator/gemini/openai_gemini.go:45-125 | Translates OpenAI API responses to Gemini format | sdk/translator, json |
| CodexToOpenAI | function | internal/translator/codex/codex_openai.go:40-100 | Translates Codex API requests to OpenAI format | sdk/translator, json |
| KiroTranslator | struct | internal/translator/kiro/kiro_translator.go:30-90 | Handles Kiro-specific API format conversions | sdk/translator, json |

### Token Stores (Auth Persistence)

| Name | Type | Location | Summary | Key Dependencies |
|------|------|----------|---------|------------------|
| FileTokenStore | struct | sdk/auth/filestore.go:25-60 | File-based token storage with JSON serialization | os, json, filepath |
| PostgresStore | struct | internal/store/postgresstore.go:40-120 | PostgreSQL-backed token store | pgx/v5, context, database/sql |
| GitTokenStore | struct | internal/store/gitstore.go:35-110 | Git-based token store for distributed auth | go-git/v6, os, context |
| ObjectTokenStore | struct | internal/store/objectstore.go:30-95 | S3-compatible object storage token backend | minio-go/v7, context |

### Model Registry

| Name | Type | Location | Summary | Key Dependencies |
|------|------|----------|---------|------------------|
| ModelRegistry | struct | internal/registry/model_registry.go:45-140 | Central registry for available AI models and their configurations | sync, context, json |
| RegisterModel | function | internal/registry/model_registry.go:142-180 | Registers new model with routing rules | validation, context |
| GetModelExecutor | function | internal/registry/model_registry.go:182-220 | Retrieves appropriate executor for model | runtime/executor |
| RouteRequest | function | internal/registry/model_registry.go:222-260 | Routes requests to appropriate provider | authentication, model definitions |

### Middleware Stack

| Name | Type | Location | Summary | Key Dependencies |
|------|------|----------|---------|------------------|
| CORSMiddleware | function | internal/api/middleware/cors.go:20-45 | Cross-Origin Resource Sharing middleware | net/http |
| AuthMiddleware | function | internal/api/middleware/auth.go:30-80 | API key authentication middleware | config, validation |
| RequestLoggingMiddleware | function | internal/api/middleware/request_logging.go:25-70 | Request/response logging with path filtering | logging, time |
| RateLimitMiddleware | function | internal/api/middleware/rate_limit.go:35-90 | Rate limiting with provider-specific rules | sync/atomic, time |

### SDK Components

| Name | Type | Location | Summary | Key Dependencies |
|------|------|----------|---------|------------------|
| CliProxy | struct | sdk/cliproxy/service.go:45-120 | Main SDK service for embedding the proxy | context, net/http |
| Pipeline | struct | sdk/cliproxy/pipeline.go:30-85 | Request processing pipeline with middleware | context, chain of responsibility |
| ProviderRegistry | struct | sdk/cliproxy/providers.go:40-100 | Registry for AI providers and their capabilities | sync, map, factory pattern |
| Watcher | struct | sdk/cliproxy/watcher.go:25-65 | Configuration change watcher | fsnotify, context |

### Utility Functions

| Name | Type | Location | Summary | Key Dependencies |
|------|------|----------|---------|------------------|
| SetLogLevel | function | internal/util/util.go:20-35 | Sets application log level based on config | logrus, config |
| ResolveAuthDir | function | internal/util/util.go:37-55 | Resolves and validates authentication directory path | filepath, os, user |
| WritablePath | function | internal/util/util.go:57-75 | Returns system-appropriate writable directory path | os, filepath, platform detection |
| ProxyRequest | function | internal/util/proxy.go:40-120 | Handles HTTP proxy requests with header forwarding | net/http, context, time |
| ConvertThinkingLevelToBudget | function | internal/util/gemini_thinking.go:25-45 | Converts thinking level strings to budget values | json, validation |

### Management API

| Name | Type | Location | Summary | Key Dependencies |
|------|------|----------|---------|------------------|
| ManagementHandlers | struct | internal/api/handlers/management/handlers.go:30-85 | Management API endpoints for remote administration | gin, config, auth |
| ConfigAccessHandler | function | internal/api/handlers/management/config.go:45-90 | Handles configuration read/write operations | json, yaml, os |
| StatsHandler | function | internal/api/handlers/management/stats.go:35-70 | Provides usage statistics and metrics | prometheus, atomic, time |
| HealthCheckHandler | function | internal/api/handlers/management/health.go:20-45 | Health check endpoint for monitoring | http, sync |

### Amp Module (Recent Addition)

| Name | Type | Location | Summary | Key Dependencies |
|------|------|----------|---------|------------------|
| AmpHandler | struct | internal/api/modules/amp/amp.go:40-110 | Handles Amp CLI and IDE extension requests | net/http, json, validation |
| AmpProxy | struct | internal/api/modules/amp/proxy.go:35-95 | Proxies Amp-specific requests to upstream services | net/http, context, proxy |
| AmpSecretManager | struct | internal/api/modules/amp/secret.go:30-80 | Manages Amp-specific secrets and authentication | encryption, config |

### Error Handling

| Name | Type | Location | Summary | Key Dependencies |
|------|------|----------|---------|------------------|
| APIError | struct | internal/interfaces/error_message.go:20-50 | Standardized API error response format | json, http status codes |
| ErrorHandler | function | internal/api/middleware/error_handling.go:25-60 | Global error handling middleware | gin, logging, metrics |
| ValidationError | struct | internal/interfaces/error_message.go:52-75 | Input validation error structure | validation, strings |

## Data Flow Architecture

1. **Request Ingestion**: HTTP server → Middleware stack (CORS, Auth, Logging)
2. **Routing**: ModelRegistry routes requests to appropriate provider executor
3. **Translation**: Translators convert between API formats (OpenAI ↔ Claude ↔ Gemini)
4. **Execution**: Provider-specific executors handle actual AI API calls
5. **Response Processing**: Reverse translation and response formatting
6. **Persistence**: Token stores handle authentication persistence

## Key Integration Points

- **Authentication**: OAuth flows for Claude/Codex/Gemini/Qwen/iFlow, token-based for Kiro
- **Storage**: Multiple backends (Filesystem, PostgreSQL, Git, S3-compatible)
- **Monitoring**: Request logging, metrics collection, health checks
- **Configuration**: Hot-reloading, YAML-based with environment variable override
- **Deployment**: Cloud deployment mode with external configuration support

## Hotspots and Complexity Indicators

- **ModelRegistry.go**: Central routing logic with high coupling to all providers
- **Main.go**: Complex initialization logic with multiple store types and deployment modes
- **Translators**: Multiple bidirectional conversion functions requiring careful maintenance
- **Executors**: Provider-specific implementations with streaming support
- **Middleware**: Multiple layers that need careful ordering and dependency management