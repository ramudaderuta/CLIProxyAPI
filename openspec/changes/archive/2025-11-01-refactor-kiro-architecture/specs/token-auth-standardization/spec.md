# Token Authentication Standardization

## Purpose
Establish token-based authentication as a first-class pattern in CLIProxyAPI, standardizing the Kiro token management approach while maintaining backward compatibility.

## ADDED Requirements

### Requirement: Token File Authentication as First-Class Pattern
The system SHALL treat token file-based authentication as a standardized, first-class authentication method alongside OAuth.

#### Scenario: Token file path configuration in main config
- **WHEN** users configure kiro-token-file in config.yaml
- **THEN** the system SHALL load tokens from specified file paths using standardized patterns
- **AND** the system SHALL support token refresh and validation through unified token management
- **AND** the system SHALL automatically enhance native token files with type information

#### Scenario: Legacy token file auto-detection
- **WHEN** no kiro-token-file is configured but kiro-auth-token.json exists in auth-dir
- **THEN** the system SHALL automatically detect and load tokens from the legacy file format
- **AND** the system SHALL ensure proper type identification for backward compatibility
- **AND** the system SHALL maintain full backward compatibility with existing token files

### Requirement: Unified Token File Management Interface
The system SHALL provide a unified interface for token file-based authentication that can be reused by future token-only providers.

#### Scenario: Token file validation and refresh
- **WHEN** a token-based request is made
- **THEN** the system SHALL validate token file expiration and automatically refresh when needed
- **AND** the system SHALL handle token refresh failures with appropriate error responses
- **AND** the system SHALL update the token file with refreshed credentials

#### Scenario: Token file format enhancement
- **WHEN** loading native Kiro token files without "type":"kiro"
- **THEN** the system SHALL automatically enhance the in-memory token with type information
- **AND** the system SHALL preserve the original file format on disk
- **AND** the system SHALL maintain backward compatibility when saving refreshed tokens

#### Scenario: Token file storage consistency
- **WHEN** tokens are refreshed or updated
- **THEN** the system SHALL store updated tokens in the original token file
- **AND** the system SHALL maintain the original file format while ensuring proper type identification
- **AND** the system SHALL handle both configured and auto-detected token file paths consistently

## MODIFIED Requirements

### Requirement: Kiro Authentication Integration (from kiro-auth spec)
The system SHALL provide authentication for Kiro AI services using both configured token file paths and auto-detected token files.

#### Scenario: Dual token loading support
- **WHEN** the CLIProxyAPI server starts
- **THEN** the system SHALL prioritize kiro-token-file configuration if present
- **ELSE** the system SHALL fallback to kiro-auth-token.json auto-detection in auth-dir
- **AND** the system SHALL initialize Kiro client with available credentials
- **AND** the system SHALL ensure proper type identification for all loaded token files

#### Scenario: Native token file compatibility
- **WHEN** loading token files that lack "type":"kiro" field
- **THEN** the system SHALL automatically enhance the token in memory only
- **AND** the system SHALL preserve original file format on disk
- **AND** the system SHALL maintain full functionality with native Kiro token exports