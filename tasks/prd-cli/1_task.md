## markdown

## status: completed # Options: pending, in-progress, completed, excluded

<task_context>
<domain>cli/core</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>http_server</dependencies>
</task_context>

# Task 1.0: Core Infrastructure Setup

## Overview

Set up core infrastructure and shared utilities for the enhanced CLI system. This includes creating the API client foundation with HTTP communication, authentication, and error handling, implementing enhanced output mode detection system with CI environment detection for TUI vs JSON selection, and creating command execution framework following existing auth module patterns.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- **REUSE**: Extend cli/auth/client.go to cli/api_client.go for workflow endpoints (retry logic, auth, JSON handling ready)
- **REUSE**: Apply cli/auth/executor.go CommandExecutor pattern for dual-mode commands (eliminates boilerplate)
- **ENHANCED**: Extend cli/auth/mode.go DetectMode() with CI environment detection (GITHUB_ACTIONS, CI, etc.)
- **REUSE**: Use pkg/config CLIConfig structure (APIKey, BaseURL, Timeout, Mode fully implemented)
- **LIBRARY**: Integrate go-resty/resty/v2 for HTTP middleware, interceptors, and rate limiting
- **LIBRARY**: Add charmbracelet/bubbletea + bubbles for interactive TUI components
- **ENHANCED**: Implement interface segregation for API services (read-only vs mutate operations)
- **REUSE**: Apply logger.FromContext(ctx) pattern consistently (no dependency injection)
- **ENHANCED**: Add --output flag as alias for --format for CLI consistency
- Requirements: 7.1, 7.2, 9.1, 9.2
</requirements>

## Subtasks

- [x] 1.1 Create unified API client with interface segregation and rate limiting
- [x] 1.2 Implement enhanced output mode detection with CI environment support
- [x] 1.3 Create command execution framework with dual handlers and context cancellation
- [x] 1.4 Set up shared utilities and enhanced error handling foundation
- [x] 1.5 Establish testing patterns for CLI components with improved mocking

## Implementation Details

### Enhanced API Client Architecture

Create a modular API client that provides interface segregation for read-only vs mutate operations, enabling easier mocking and future caching layers. Include rate-limiting middleware and aggressive context.Context usage for cancellation.

### Enhanced Mode Detection

Implement auto-detection that checks CI environment variables (CI, GITHUB_ACTIONS, JENKINS_URL, etc.) to automatically default to JSON mode in CI environments, plus explicit format flags and terminal capabilities.

### Command Execution Pattern

Follow the existing auth module pattern with dual handlers for TUI and JSON modes, enhanced with proper context cancellation for long-running operations.

### Relevant Files

- `cli/api_client.go` - New unified API client with interface segregation
- `cli/output.go` - Enhanced mode detection with CI environment support
- `cli/command_executor.go` - New command execution framework with context handling
- `cli/shared/` - New shared utilities directory
- `cli/interfaces/` - New service interface definitions with segregation

### Dependent Files

- `cli/auth/client.go` - Existing auth patterns to follow
- `cli/config.go` - Existing configuration system
- `cli/tui/` - Existing TUI components to extend
- `engine/*/router/` - API endpoints to interact with

## Success Criteria

- API client successfully communicates with all Compozy server endpoints using segregated interfaces
- Output mode detection automatically switches to JSON in CI environments and supports --output alias
- Command execution framework handles both interactive and automation modes with proper cancellation
- Rate limiting prevents API abuse and interface segregation enables easier testing
- All components follow established project patterns and pass linting/testing
