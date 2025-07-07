---
status: pending
---

<task_context>
<domain>cli</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>cobra,charmbracelet</dependencies>
</task_context>

# Task 1.0: CLI Infrastructure Setup

## Overview

Establish the foundational CLI infrastructure for Compozy with Cobra command structure, Charmbracelet styling system, API client, and TUI-by-default output management that supports all future CLI features.

## Subtasks

- [ ] 1.1 Extend root command with workflow and execution command groups
- [ ] 1.2 Implement Lipgloss styling system in cli/shared/styles.go
- [ ] 1.3 Create API client in cli/shared/client.go for server communication
- [ ] 1.4 Build output manager with TUI/non-TUI modes (--no-tui flag)
- [ ] 1.5 Add global flags (--output, --no-tui, --project, --debug)

## Implementation Details

### Command Structure

```
compozy
├── init        # Project initialization
├── workflow    # Workflow management (singular)
├── run         # Execution management
└── dev         # Existing development server
```

### TUI-by-Default Architecture

```go
// cli/shared/output.go
type OutputManager struct {
    tui    bool  // Default: true, disabled with --no-tui
    format string // json, yaml, table (for non-TUI mode)
}

func (o *OutputManager) Render(data interface{}) error {
    if !o.tui || !isTerminal() {
        return o.renderPlain(data)
    }
    return o.renderTUI(data)
}
```

### Lipgloss Style System

```go
// cli/shared/styles.go
var (
    primaryColor = lipgloss.Color("#7D56F4")
    successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#02BA84"))
    errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F87"))
    titleStyle   = lipgloss.NewStyle().Bold(true).Background(primaryColor)
)
```

### API Client

```go
// cli/shared/client.go
type Client struct {
    BaseURL    string
    HTTPClient *http.Client
}

func (c *Client) ListWorkflows(ctx context.Context) ([]Workflow, error)
func (c *Client) ExecuteWorkflow(ctx context.Context, id string, input map[string]any) (*Execution, error)
```

### Global Flags

- `--no-tui`: Disable interactive TUI (for CI/scripts)
- `--output`: Format for non-TUI mode (json, yaml, table)
- `--project`: Project directory override
- `--debug`: Enable debug logging
- `--server-url`: API server URL (default: http://localhost:3001)

## Success Criteria

- [ ] Command structure follows Cobra best practices
- [ ] TUI renders beautifully by default with Lipgloss styling
- [ ] Non-TUI mode works perfectly for CI/automation (--no-tui)
- [ ] API client handles all HTTP communication cleanly
- [ ] Output manager intelligently switches between TUI and plain modes
- [ ] Global flags are available to all subcommands
- [ ] Existing `compozy dev` command continues to work unchanged

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
