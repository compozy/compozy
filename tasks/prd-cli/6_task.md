---
status: pending
---

<task_context>
<domain>cli/ci</domain>
<type>implementation</type>
<scope>enhancement</scope>
<complexity>low</complexity>
<dependencies>json_output,exit_codes</dependencies>
</task_context>

# Task 6.0: CI/CD Integration Features

## Overview

Ensure the CLI works seamlessly in automated environments by providing robust non-TUI modes, machine-readable output, proper exit codes, and GitHub Actions integration.

## Subtasks

- [ ] 6.1 Ensure all commands respect --no-tui flag properly
- [ ] 6.2 Add comprehensive JSON/YAML output for all commands
- [ ] 6.3 Implement consistent exit code conventions
- [ ] 6.4 Create GitHub Actions for common workflows
- [ ] 6.5 Add environment variable support for CI configuration

## Implementation Details

### Non-TUI Mode

```bash
# CI/automation usage with --no-tui
compozy workflow list --no-tui --output json
compozy run create my-workflow --no-tui --input-file input.json
compozy run status exec-123 --no-tui --output yaml
```

### Exit Code Standards

```go
const (
    ExitSuccess       = 0
    ExitGeneralError  = 1
    ExitValidation    = 2
    ExitNotFound      = 3
    ExitTimeout       = 4
    ExitCancelled     = 5
)
```

### Environment Detection

```go
func shouldDisableTUI() bool {
    // Auto-disable TUI in CI environments
    if os.Getenv("CI") == "true" ||
       os.Getenv("GITHUB_ACTIONS") == "true" ||
       os.Getenv("JENKINS_URL") != "" {
        return true
    }
    return false
}
```

### GitHub Actions

```yaml
name: Deploy Workflow
on: push
jobs:
    deploy:
        runs-on: ubuntu-latest
        steps:
            - uses: actions/checkout@v3
            - name: Deploy workflow
              run: |
                  compozy workflow deploy production \
                    --no-tui \
                    --server-url ${{ secrets.COMPOZY_URL }}
```

## Success Criteria

- [ ] TUI is automatically disabled in CI environments
- [ ] All commands produce clean JSON/YAML with --no-tui
- [ ] Exit codes follow consistent conventions
- [ ] No command ever blocks waiting for input in CI mode
- [ ] GitHub Actions examples work out of the box
- [ ] Error messages are parseable in CI logs

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
