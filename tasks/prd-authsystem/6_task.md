---
status: completed
---

<task_context>
<domain>cli/auth</domain>
<type>implementation</type>
<scope>tooling</scope>
<complexity>low</complexity>
<dependencies>service</dependencies>
</task_context>

# Task 6.0: CLI Commands

## Overview

Add `auth` command group to CLI with sub-commands for key generation, revocation, listing, and admin user CRUD.

<requirements>
- Use `spf13/cobra` patterns used elsewhere.
- Output JSON by default.
- Accept `--role admin|user` on user creation.
</requirements>

<critical>
**MANDATORY REQUIREMENTS:**
- **ALWAYS** check dependent files APIs before write tests to avoid write wrong code
- **ALWAYS** verify against PRD and tech specs - NEVER make assumptions
- **NEVER** use workarounds, especially in tests - implement proper solutions
- **MUST** follow all established project standards:
    - Architecture patterns: @.cursor/rules/architecture.mdc
    - Go coding standards: @.cursor/rules/go-coding-standards.mdc
    - Testing requirements: @.cursor/rules/test-standard.mdc
    - API standards: @.cursor/rules/api-standards.mdc
    - Security & quality: @.cursor/rules/quality-security.md
    - GoGraph MCP tools: @.cursor/rules/gograph.mdc
    - No Backwards Compatibility: @.cursor/rules/backwards-compatibility.mdc
- **MUST** use `logger.FromContext(ctx)` - NEVER use logger as function parameter or dependency injection
- **MUST** run `make lint` and `make test` before completing ANY subtask
- **MUST** follow @.cursor/rules/task-review.mdc workflow for parent tasks
**Enforcement:** Violating these standards results in immediate task rejection.
</critical>

## Subtasks

### Foundation (Setup & Architecture)

- [x] 6.1 Set up Bubble Tea dependencies and project structure
  - Add charmbracelet packages to go.mod (bubbletea, bubbles, lipgloss)
  - Create directory structure for TUI components (cli/auth/tui/)
  - Set up base types and interfaces for TUI models
  - Implement intelligent mode detection (TTY vs JSON)

- [x] 6.2 Create auth HTTP client wrapper
  - Implement client with context support and timeout handling
  - Add retry logic with exponential backoff
  - Handle auth token refresh automatically
  - Create comprehensive test coverage for client functionality
  - Integrate with existing API endpoints from task 5

- [x] 6.3 Build shared TUI components
  - Create consistent theme with lipgloss styles
  - Build error display component with retry options
  - Implement status bar for async operations
  - Create help text component with keyboard shortcuts
  - Design layout system for consistent UI structure

### Key Management Implementation

- [x] 6.4 Implement key generation command
  - TUI: Interactive form with name/description fields using bubbles/textinput
  - JSON: Accept input via stdin or flags (--name, --description)
  - Show generated key once with copy-to-clipboard instruction
  - Implement secure key display with masked/reveal toggle
  - Add expiration date option for keys

- [x] 6.5 Implement key listing command
  - TUI: Paginated table with bubbles/table (50 items per page)
  - Support sorting by created date, name, last used
  - Add search/filter functionality with real-time results
  - Show key usage statistics in table
  - JSON: Output array with pagination metadata

- [x] 6.6 Implement key revocation command ✅ COMPLETED
  - [x] TUI: Select from list with visual confirmation dialog
  - [x] Show consequences before confirmation (affected resources) - Basic implementation
  - [x] JSON: Accept key ID via argument
  - [x] Add --force flag to skip confirmation
  - [ ] Log revocation reason for audit - Future enhancement

### User Management Implementation (Admin Only)

- [x] 6.7 Implement user creation command ✅ COMPLETED
  - [x] TUI: Multi-step wizard (email → name → role → confirmation)
  - [x] Validate email format in real-time with visual feedback - Basic validation
  - [ ] Show password requirements and strength indicator - Future enhancement
  - [x] Role selection with description of permissions
  - [x] JSON: Accept user object via stdin with validation

- [x] 6.8 Implement user listing command ✅ COMPLETED
  - [x] TUI: Searchable table with role filtering and pagination
  - [x] Support sorting by multiple columns (name, email, created, role)
  - [ ] Quick actions (edit/delete) with keyboard shortcuts - TODO for 6.9
  - [ ] Show user statistics (key count, last activity) - Pending API support
  - [x] JSON: Output with optional filters (--role, --active)

- [x] 6.9 Implement user update/delete commands ✅ COMPLETED
  - [x] TUI: Edit form with field validation and diff preview
  - [x] Confirmation for destructive actions with impact summary
  - [x] Show affected resources before delete (keys, permissions)
  - [ ] Audit log integration for all changes - Future enhancement
  - [x] JSON: Accept updates via patch syntax

### Testing & Polish

- [x] 6.11 Implement help system and navigation ✅ COMPLETED
  - [x] Context-sensitive help (? key) on every screen
  - [x] Command palette (Ctrl+K) for quick actions
  - [x] Keyboard shortcut reference card
  - [x] Interactive tutorial mode for first-time users
  - [x] Breadcrumb navigation for deep workflows

## Success Criteria

### Core Functionality

- `compozy auth generate` returns new key via API in both TUI and JSON modes
- Commands follow lowercase flag convention (--json, --tui, --force)
- Role-based access control enforced (admin-only commands restricted)
- All commands work in non-interactive environments (CI/CD)

### TUI-Specific Criteria

- TUI starts in <100ms on modern hardware
- All actions accessible via keyboard navigation only
- Graceful fallback when terminal doesn't support colors/Unicode
- Memory usage remains <50MB for 1000-item lists
- Consistent 60fps rendering during scrolling operations
- Error states clearly communicated with recovery options
- Context-sensitive help available on every screen (? key)
- Progress indicators shown for operations >500ms
- Supports standard terminal keybindings (Ctrl+C, Ctrl+D)

### Integration Criteria

- Seamless mode switching based on environment detection
- Auth token handling shared between Cobra and TUI contexts
- Consistent error messages across TUI and JSON modes
- Proper cleanup on interrupt signals (no terminal corruption)
- Respects user's terminal preferences (color scheme, locale)
