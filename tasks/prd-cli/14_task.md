## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>cli/core</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>external_apis</dependencies>
</task_context>

# Task 14.0: Global Flags and Help System

## Overview

Implement consistent global flags (--server-url, --config, --debug, --quiet, --format, --output), create comprehensive help system with examples for all commands, build debug logging system with verbose output options, and add quiet mode for automation-friendly output.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- **REUSE**: Leverage pkg/config system for global flag configuration management
- **REUSE**: Extend spf13/cobra patterns from existing CLI structure
- **REUSE**: Apply cli/auth/mode.go for debug and quiet mode detection
- **LIBRARY**: Use charmbracelet/lipgloss for help system formatting
- **REUSE**: Use logger.FromContext(ctx) with debug level configuration
- **REUSE**: Apply pkg/config override mechanisms for flag precedence
- **LIBRARY**: Use golang.org/x/term for terminal detection and capabilities
- **REUSE**: Follow existing global flag patterns from auth commands
- **ENHANCED**: Add --output flag as alias for --format for CLI consistency
- **ENHANCED**: Ensure flag aliases work seamlessly across all commands
- Requirements: 9.1, 9.2, 9.3, 9.4, 9.5, 10.5
</requirements>

## Subtasks

- [x] 14.0 Update cli/dev.go that has redundant arguments that should be using this global system
- [x] 14.1 Implement global flags across all commands with aliases
- [x] 14.2 Create comprehensive help system with examples
- [x] 14.3 Build debug logging system with verbose output
- [x] 14.4 Add quiet mode for automation scenarios
- [x] 14.5 Integrate help content with contextual assistance

## Implementation Details

### Global Flags with Aliases

Implement consistent global flags that are available across all commands: --server-url, --config, --debug, --quiet, --format/--output, --no-color, --interactive. Ensure --output works as a perfect alias for --format for better CLI consistency.

### Help System

Create comprehensive help text for all commands with usage examples, common scenarios, and troubleshooting tips.

### Debug Logging

Implement debug logging that can be enabled with --debug flag, providing detailed information about API calls, configuration loading, and internal operations.

### Quiet Mode

Add --quiet flag that suppresses non-essential output, making commands suitable for automation and scripting.

### Relevant Files

- `cli/flags/global.go` - Global flag definitions and handling with aliases
- `cli/help/` - Help content and examples
- `cli/logging/` - Debug logging utilities
- Root command files - Integration with global flags

### Dependent Files

- `cli/config.go` - Configuration system integration
- `pkg/logger/` - Existing logging infrastructure
- All command files - Global flag integration

## Success Criteria

- Global flags work consistently across all commands with proper alias support
- --output flag works identically to --format for better CLI consistency
- Help system provides useful examples and guidance for all operations
- Debug mode provides detailed logging for troubleshooting
- Quiet mode produces minimal output suitable for automation
- Help content is comprehensive and includes practical examples
