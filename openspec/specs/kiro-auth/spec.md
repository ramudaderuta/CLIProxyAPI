# kiro-auth Specification

## Purpose
TBD - created by archiving change add-kiro-auth. Update Purpose after archive.
## Requirements
### Requirement: Kiro Authentication Integration
The system SHALL provide authentication for Kiro AI services using imported OAuth token files.

#### Scenario: Load Kiro credentials from file
- **WHEN** the CLIProxyAPI server starts with kiro-auth-token.json present in auth directory
- **THEN** the system SHALL load accessToken, refreshToken, and region from the JSON file
- **AND** the system SHALL initialize Kiro client with these credentials

#### Scenario: Token refresh for expired credentials
- **WHEN** the Kiro accessToken expires during API requests
- **THEN** the system SHALL automatically use the refreshToken to obtain new credentials
- **AND** the system SHALL update the stored credentials file with new tokens

### Requirement: Kiro Model Support
The system SHALL provide access to Kiro-supported Claude models through OpenAI-compatible endpoints.

#### Scenario: Model listing includes Kiro models
- **WHEN** clients request GET /v1/models
- **THEN** the response SHALL include all Kiro-supported models with proper aliases
- **AND** each model SHALL be mapped to appropriate internal Kiro model identifiers

#### Scenario: Chat completion with Kiro models
- **WHEN** clients send POST /v1/chat/completions with a Kiro model name
- **THEN** the request SHALL be routed to Kiro service with proper model mapping
- **AND** the response SHALL follow OpenAI format with Kiro-generated content

### Requirement: Kiro Token Management
The system SHALL manage Kiro authentication tokens without requiring online OAuth flow.

#### Scenario: Import existing credentials
- **WHEN** users place kiro-auth-token.json in the configured auth directory
- **THEN** the system SHALL automatically detect and import the credentials
- **AND** the system SHALL validate token format and expiration

#### Scenario: No online authentication required
- **WHEN** Kiro authentication is configured
- **THEN** the system SHALL NOT attempt OAuth login flows
- **AND** the system SHALL rely solely on imported token files

