---
status: pending
---

<task_context>
<domain>cli</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>http_server</dependencies>
</task_context>

# Task 11.0: CLI Command Implementation

## Overview

Implement comprehensive CLI commands for organization, user, and API key management with intuitive workflows. This provides command-line access to all multi-tenant management operations.

<critical>
**MANDATORY REQUIREMENTS:**
- **ALWAYS** verify against PRD and tech specs - NEVER make assumptions
- **NEVER** use workarounds, especially in tests - implement proper solutions
- **MUST** follow all established project standards:
    - Architecture patterns: `.cursor/rules/architecture.mdc`
    - Go coding standards: `.cursor/rules/go-coding-standards.mdc`
    - Testing requirements: `.cursor/rules/testing-standards.mdc`
    - API standards: `.cursor/rules/api-standards.mdc`
    - Security & quality: `.cursor/rules/quality-security.mdc`
- **MUST** run `make lint` and `make test` before completing ANY subtask
- **MUST** follow `.cursor/rules/task-completion.mdc` workflow for parent tasks
**Enforcement:** Violating these standards results in immediate task rejection.
</critical>

## Subtasks

- [ ] 11.1 Create organization commands: create, list, show, update, delete
- [ ] 11.2 Create user commands: add, list, show, update, remove with org and role flags
- [ ] 11.3 Create API key commands: create, list, show, update, revoke with user association
- [ ] 11.4 Implement proper flag validation and comprehensive help text
- [ ] 11.5 Add configuration file support for API endpoint and authentication
- [ ] 11.6 Implement output formatting (table, JSON) with --output flag
- [ ] 11.7 Add interactive prompts for destructive operations
- [ ] 11.8 Implement progress indicators for long-running operations

## Implementation Details

Create CLI commands using cobra:

1. **Organization commands**: create, list, show, update, delete
2. **User commands**: add, list, show, update, remove with org and role flags
3. **API key commands**: create, list, show, update, revoke with user association
4. **Proper flag validation** and help text
5. **Configuration file support** for API endpoint and authentication
6. **Output formatting** (table, JSON) with --output flag
7. **Interactive prompts** for destructive operations
8. **Progress indicators** for long-running operations

Commands: 'compozy org create', 'compozy user add', 'compozy apikey create' with intuitive flag structure.

### Relevant Files

- `engine/auth/commands/org.go` - Organization CLI commands
- `engine/auth/commands/user.go` - User CLI commands
- `engine/auth/commands/apikey.go` - API key CLI commands
- `engine/auth/commands/config.go` - CLI configuration management
- `cli/auth.go` - Main CLI entry point that imports and registers auth commands

## Success Criteria

- All organization management commands functional with proper validation
- User management commands support role assignment and organization context
- API key commands provide complete lifecycle management
- Flag validation prevents invalid command usage
- Configuration file enables easy endpoint and auth setup
- Output formatting supports both human-readable and machine-readable formats
- Interactive prompts prevent accidental destructive operations
- Progress indicators provide feedback for long-running operations
