# Kiro Translation Layer

## Purpose
Extract translation logic from the monolithic Kiro executor into a dedicated translation layer following established patterns used by other providers.

## ADDED Requirements

### Requirement: Dedicated Kiro Translation Package
The system SHALL provide a dedicated translation package for Kiro following the same patterns as other providers.

#### Scenario: OpenAI to Kiro request translation
- **WHEN** clients send OpenAI-format requests to Kiro models
- **THEN** the request translator SHALL convert OpenAI chat completion format to Kiro internal format
- **AND** the translator SHALL handle message formatting, tool calls, and parameter mapping
- **AND** the translator SHALL maintain semantic equivalence between formats

#### Scenario: Kiro to OpenAI response translation
- **WHEN** Kiro service returns responses in internal format
- **THEN** the response translator SHALL convert Kiro format to OpenAI chat completion format
- **AND** the translator SHALL handle streaming responses, tool results, and error formatting
- **AND** the translator SHALL preserve response metadata and usage information

### Requirement: Model Mapping Standardization
The system SHALL provide standardized model mapping between OpenAI-compatible names and Kiro internal identifiers.

#### Scenario: Model name resolution
- **WHEN** clients request models using OpenAI-compatible names
- **THEN** the model mapper SHALL resolve to appropriate Kiro internal model identifiers
- **AND** the mapper SHALL support alias mapping for different model versions
- **AND** the mapper SHALL validate model availability and capabilities

#### Scenario: Model capability mapping
- **WHEN** requests include tool calls or specific features
- **THEN** the model mapper SHALL validate Kiro model support for requested capabilities
- **AND** the mapper SHALL provide appropriate error responses for unsupported features

### Requirement: Translation Layer Extensibility
The translation layer SHALL be designed for extensibility to support future Kiro API changes.

#### Scenario: API format evolution
- **WHEN** Kiro introduces new API features or format changes
- **THEN** the translation layer SHALL be extensible without requiring executor changes
- **AND** the translators SHALL maintain backward compatibility for existing request formats

## REMOVED Requirements

### Requirement: Inline Translation in Executor
The system SHALL NOT handle API format translation directly within the executor component.

#### Scenario: Separation of concerns
- **WHEN** processing Kiro requests
- **THEN** the executor SHALL delegate all format translation to the dedicated translation package
- **AND** the executor SHALL focus only on request coordination and response handling