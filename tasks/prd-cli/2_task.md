## markdown

## status: completed # Options: pending, in-progress, completed, excluded

<task_context>
<domain>cli/config</domain>
<type>implementation</type>
<scope>configuration</scope>
<complexity>low</complexity>
<dependencies>external_apis</dependencies>
</task_context>

# Task 2.0: Enhanced Configuration Management

## Overview

Extend existing config system with CLI-specific settings, add configuration validation with detailed error messages and suggestions, and implement configuration display with source tracking and sensitive data redaction.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- **SIMPLIFIED**: pkg/config system is already comprehensive with CLIConfig, validation, source tracking
- **REUSE**: pkg/config CLIConfig has APIKey, BaseURL, Timeout, Mode - may only need minor additions
- **REUSE**: Leverage existing koanf-based multi-source loading (env, yaml, cli, defaults)
- **REUSE**: Use existing SensitiveString type for secure data handling and redaction
- **REUSE**: Apply pkg/config validation and error handling patterns
- **LIBRARY**: Consider fsnotify/fsnotify for config file watching if hot-reload needed
- **OPTIMIZE**: Most configuration infrastructure already exists - focus on workflow-specific additions only
- Requirements: 8.1, 8.2, 8.3, 8.4
</requirements>

## Subtasks

- [x] 2.1 Extend CLIConfig struct with CLI-specific settings ✅ COMPLETED
- [x] 2.2 Implement config validation with detailed error messages ✅ COMPLETED
- [x] 2.3 Create config display command with source tracking ✅ COMPLETED
- [x] 2.4 Add sensitive data redaction for config output ✅ COMPLETED
- [x] 2.5 Update config diagnostics functionality ✅ COMPLETED

## Implementation Details

### Configuration Extensions

Add CLI-specific settings to the existing configuration system, including ServerURL, DefaultFormat, ColorMode, PageSize, and Timeout as specified in the techspec CLIConfig struct.

### Validation System

Implement comprehensive validation with helpful error messages and suggestions for common configuration issues.

### Display and Diagnostics

Create commands to show current configuration with sources (file, env, defaults) and validate configuration files with specific line numbers and suggestions.

### Relevant Files

- `cli/config.go` - Extend existing configuration
- `cli/commands/config.go` - New config command implementations
- `pkg/config/` - Integration with existing config system

### Dependent Files

- `pkg/config/config.go` - Existing configuration foundation
- `cli/auth/` - Auth configuration patterns to follow

## Success Criteria

- CLI-specific configuration settings properly integrated with existing system
- Configuration validation provides clear, actionable error messages
- Config display shows source tracking and redacts sensitive information
- All config commands work in both TUI and JSON modes
- Configuration diagnostics help users troubleshoot setup issues
