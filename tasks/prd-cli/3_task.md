---
status: pending
---

<task_context>
<domain>cli/cmd/workflow</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>api_client,lipgloss,bubbles</dependencies>
</task_context>

# Task 3.0: Workflow Management Commands

## Overview

Implement core workflow management commands with beautiful TUI tables by default, interactive selection, and comprehensive validation to ensure workflow correctness before deployment.

## Subtasks

- [ ] 3.1 Implement `workflow list` with interactive table (Bubbles)
- [ ] 3.2 Create `workflow get` with styled detailed view
- [ ] 3.3 Build `workflow deploy` with validation and TUI confirmation
- [ ] 3.4 Add `workflow validate` for local validation with styled output
- [ ] 3.5 Implement non-TUI formatters for CI/automation (--no-tui)

## Implementation Details

### Interactive List Command

```go
// Uses Bubbles table component
type listModel struct {
    table  table.Model
    client *shared.Client
}

// Interactive selection with arrow keys
// Press Enter to view workflow details
// Press 'd' to deploy selected workflow
```

### Beautiful Styled Output

```go
// Workflow details with Lipgloss
╭─ Workflow: customer-support ────────────╮
│ Version:    1.2.3                       │
│ Status:     ✓ Active                    │
│ Tasks:      12                          │
│ Last Run:   2 hours ago                 │
│                                         │
│ Description:                            │
│ Automated customer support workflow     │
│ with AI-powered responses               │
╰─────────────────────────────────────────╯
```

### Deploy Process (TUI Mode)

1. Validate workflow locally
2. Show deployment preview with diff
3. Interactive confirmation with Huh
4. Deploy with progress indicator
5. Show success/error with styling

### Non-TUI Mode

```bash
# For CI/automation
compozy workflow list --no-tui --output json
compozy workflow deploy my-workflow --no-tui --yes
```

## Success Criteria

- [ ] List command shows beautiful interactive table by default
- [ ] Workflows can be selected and viewed interactively
- [ ] Deploy shows clear preview and requires confirmation
- [ ] Validation provides helpful, styled error messages
- [ ] Non-TUI mode outputs clean JSON/YAML for scripting
- [ ] All commands handle API errors gracefully
- [ ] Performance is acceptable for large workflow lists

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
- **MUST** run `make lint` and `make test` before completing parent tasks
- **MUST** follow `.cursor/rules/task-review.mdc` workflow for parent tasks

**Enforcement:** Violating these standards results in immediate task rejection.
</critical>
