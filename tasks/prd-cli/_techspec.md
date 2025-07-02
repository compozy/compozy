# CLI Commands Plan

## Overview

This document outlines the missing CLI commands for Compozy's MVP. Based on deep analysis of the REST API, the CLI should mirror the sophisticated workflow management capabilities available through the HTTP interface.

**Current State**: Only `compozy dev` command exists  
**Goal**: Complete CLI interface for workflow orchestration lifecycle

## API Analysis Summary

The REST API reveals comprehensive workflow management through these endpoints:

- `/api/workflows` - Workflow discovery and component listing
- `/api/workflows/:id/executions` - Workflow execution management
- `/api/executions/workflows` - Execution control (pause/resume/cancel/signal)
- `/api/schedules` - Schedule management with CRON support
- `/api/events` - Event-driven workflow triggering

## Revised MVP Command Set

Based on API capabilities, the CLI should provide:

```
compozy
├── dev           (existing) - Local development server
├── init          - Initialize new project
├── workflow      - Workflow management and execution
├── execution     - Execution lifecycle control
├── schedule      - Schedule management
├── event         - Event triggering
├── config        - Configuration management
└── version       - Version info
```

## Command Specifications

### `compozy init [project-name]`

**Purpose**: Initialize a new Compozy project with basic structure

**Behavior**:

- Creates `compozy.yaml` with minimal project configuration
- Sets up `.env.example` with common environment variables
- Creates directory structure: `workflows/`, `tools/`, `agents/`
- Optionally accepts project name as argument

**Options**:

- `--template <name>` - Use specific project template
- `--dir <path>` - Initialize in specific directory

**Example**:

```bash
compozy init my-ai-project
compozy init --template basic-agent
```

### `compozy workflow` - Workflow Management

#### `compozy workflow list`

**Purpose**: List all available workflows in the project

**API Mapping**: `GET /api/workflows`

**Options**:

- `--format <format>` - Output format: table, json, yaml
- `--show-components` - Include tasks, agents, tools count

**Example**:

```bash
compozy workflow list
compozy workflow list --format json
```

#### `compozy workflow get <workflow_id>`

**Purpose**: Get detailed information about a specific workflow

**API Mapping**: `GET /api/workflows/:workflow_id`

**Options**:

- `--show-tasks` - Include task details
- `--show-agents` - Include agent details
- `--show-tools` - Include tool details

**Example**:

```bash
compozy workflow get customer-support
compozy workflow get data-pipeline --show-tasks --show-agents
```

#### `compozy workflow execute <workflow_id>`

**Purpose**: Execute a workflow through the API

**API Mapping**: `POST /api/workflows/:workflow_id/executions`

**Options**:

- `--input <json>` - Pass input data as JSON string
- `--input-file <file>` - Pass input data from file
- `--task <task_id>` - Execute specific task only
- `--wait` - Wait for execution completion
- `--follow` - Follow execution logs in real-time

**Example**:

```bash
compozy workflow execute customer-support --input '{"customer_id": "123"}'
compozy workflow execute data-pipeline --input-file input.json --follow
```

#### `compozy workflow tasks <workflow_id>`

**Purpose**: List tasks within a workflow

**API Mapping**: `GET /api/workflows/:workflow_id/tasks`

**Example**:

```bash
compozy workflow tasks customer-support --format table
```

### `compozy execution` - Execution Management

#### `compozy execution list`

**Purpose**: List workflow executions

**API Mapping**: `GET /api/executions/workflows`

**Options**:

- `--workflow <workflow_id>` - Filter by workflow
- `--status <status>` - Filter by status (PENDING, RUNNING, COMPLETED, FAILED)
- `--limit <n>` - Limit number of results
- `--format <format>` - Output format

**Example**:

```bash
compozy execution list --workflow customer-support --status RUNNING
```

#### `compozy execution get <exec_id>`

**Purpose**: Get detailed execution information

**API Mapping**: `GET /api/executions/workflows/:exec_id`

**Options**:

- `--show-logs` - Include execution logs

**Example**:

```bash
compozy execution get exec-12345 --show-logs
```

#### `compozy execution pause/resume/cancel` (Future Phase)

**Note**: Execution control operations will be added in a future phase to keep MVP focused.

#### `compozy execution signal <exec_id> <signal_name>`

**Purpose**: Send signal to running execution

**API Mapping**: `POST /api/executions/workflows/:exec_id/signals`

**Options**:

- `--payload <json>` - Signal payload as JSON
- `--payload-file <file>` - Signal payload from file

**Example**:

```bash
compozy execution signal exec-12345 user_input --payload '{"action": "approve"}'
```

### `compozy schedule` - Schedule Management

#### `compozy schedule list`

**Purpose**: List all scheduled workflows

**API Mapping**: `GET /api/schedules`

**Options**:

- `--enabled-only` - Show only enabled schedules
- `--format <format>` - Output format

#### `compozy schedule get <workflow_id>`

**Purpose**: Get schedule details for a workflow

**API Mapping**: `GET /api/schedules/:workflow_id`

#### `compozy schedule update <workflow_id>`

**Purpose**: Update workflow schedule

**API Mapping**: `PATCH /api/schedules/:workflow_id`

**Options**:

- `--cron <expression>` - CRON expression
- `--enabled <true|false>` - Enable/disable schedule

**Example**:

```bash
compozy schedule update daily-report --cron "0 9 * * *" --enabled true
```

#### `compozy schedule delete <workflow_id>`

**Purpose**: Delete workflow schedule

**API Mapping**: `DELETE /api/schedules/:workflow_id`

### `compozy event` - Event Management

#### `compozy event send <event_name>`

**Purpose**: Send event to trigger workflows

**API Mapping**: `POST /api/events`

**Options**:

- `--payload <json>` - Event payload as JSON
- `--payload-file <file>` - Event payload from file

**Example**:

```bash
compozy event send customer_created --payload '{"customer_id": "456"}'
```

### `compozy config` - Configuration Management

#### `compozy config validate [file]`

**Purpose**: Validate project configuration

**Options**:

- `--strict` - Enable strict validation mode

#### `compozy config show`

**Purpose**: Show current configuration

**Options**:

- `--include-defaults` - Show default values

### `compozy version`

**Purpose**: Display version information

**Example**:

```bash
compozy version
```

## Implementation Strategy

### API-First Approach with Modern TUI

All CLI commands should be implemented as HTTP API clients that communicate with the Compozy server, enhanced with modern Terminal User Interface (TUI) capabilities using the Charmbracelet ecosystem for world-class developer experience.

```go
// CLI Client Architecture
type CompozyClient struct {
    BaseURL    string
    HTTPClient *http.Client
    Auth       AuthConfig
}

// Core methods mapping to API endpoints
func (c *CompozyClient) ListWorkflows() ([]Workflow, error)
func (c *CompozyClient) ExecuteWorkflow(id string, req ExecuteWorkflowRequest) (*ExecutionResponse, error)
func (c *CompozyClient) GetExecution(execID string) (*WorkflowExecutionResponse, error)
func (c *CompozyClient) SendSignal(execID, signalName string, payload map[string]interface{}) error
```

### Phase 1: Foundation with TUI (Weeks 1-2)

**Lipgloss Universal Styling** (All Commands):

- Beautiful, consistent output styling
- Error messages with color coding
- Progress indicators and status display

**Priority 1**: Core workflow operations (MVP)

1. `compozy init` - Interactive project initialization with **Huh forms**
2. `compozy workflow list` - Workflow discovery with **beautiful tables**
3. `compozy workflow execute` - Workflow execution with **real-time progress**
4. `compozy execution list` - Execution monitoring with **styled tables**
5. `compozy execution get` - Execution details with **formatted output**

**Priority 2**: Essential operations (MVP) 6. `compozy config validate` - Configuration validation with **styled errors** 7. `compozy version` - Version information with **styled output**

**Hybrid Mode Strategy**:

- `--interactive` flag to toggle TUI mode
- `--json` output always available for scripting
- Auto-detection: TUI disabled for pipes/automation

### Phase 2: Advanced TUI Features (Weeks 3-4)

**Rich Interactive Experiences**:

- `compozy workflow list` - Interactive selection with filtering
- `compozy execution get --follow` - Live dashboard with log streaming
- `compozy workflow execute` - **Bubble Tea** real-time monitoring

**Execution Control** (Deferred from MVP):

- `compozy execution pause/resume/cancel` - Execution lifecycle control

**Schedule Management**:

- `compozy schedule list/get/update/delete` - Schedule operations

**Event System**:

- `compozy event send` - Event triggering
- `compozy execution signal` - Signal sending

**Enhanced Operations**:

- `compozy workflow tasks/agents/tools` - Component inspection

## Global vs Command-Specific Arguments

### Global Arguments (All Commands)

Based on analysis of the current `dev` command, these flags should be available globally:

**Infrastructure & Configuration:**

- `--config` / `-c`: Project configuration file (default: "compozy.yaml")
- `--cwd`: Working directory override
- `--env-file`: Environment file path (default: ".env")

**Logging & Output:**

- `--log-level`: Log level (debug, info, warn, error)
- `--log-json`: JSON log format
- `--log-source`: Include source in logs
- `--debug`: Debug mode (sets log-level to debug)
- `--output` / `-o`: Output format (table, json, yaml) - **JSON output is first-class citizen**
- `--quiet` / `-q`: Suppress non-essential output
- `--verbose` / `-v`: Verbose output

**Server Connection (Client Commands):**

- `--server-url`: Compozy server URL (default: "http://localhost:3001")
- `--timeout`: Request timeout (default: 30s)
- `--api-key`: API authentication key (future)

**TUI & Interaction:**

- `--interactive`: Force interactive TUI mode (auto-detected by default)
- `--no-interactive`: Force non-interactive mode for scripting
- `--no-color`: Disable color output (respects NO_COLOR env var)

### Command-Specific Arguments

#### `compozy dev` (Server Command)

**Server Configuration:**

- `--port`: Server port (default: 3001)
- `--host`: Server host (default: "0.0.0.0")
- `--cors`: Enable CORS

**Database Configuration:**

- `--db-host`, `--db-port`, `--db-user`, `--db-password`
- `--db-name`, `--db-ssl-mode`, `--db-conn-string`

**Temporal Configuration:**

- `--temporal-host`: Temporal server host:port
- `--temporal-namespace`: Temporal namespace
- `--temporal-task-queue`: Task queue name

**Development Features:**

- `--watch`: File watcher for auto-restart
- `--tool-execution-timeout`: Tool timeout (default: 60s)
- `--max-nesting-depth`: Task nesting limit (default: 20)

**Dispatcher Configuration:**

- `--dispatcher-heartbeat-interval`: Heartbeat interval (default: 30s)
- `--dispatcher-heartbeat-ttl`: Heartbeat TTL (default: 300s)
- `--dispatcher-stale-threshold`: Stale threshold (default: 120s)

#### `compozy workflow execute` (Client Command)

- `--input`: Input data as JSON string
- `--input-file`: Input data from file
- `--task`: Execute specific task only
- `--wait`: Wait for execution completion
- `--follow`: Follow execution logs

#### `compozy execution list` (Client Command)

- `--workflow`: Filter by workflow ID
- `--status`: Filter by execution status
- `--limit`: Limit number of results
- `--since`: Show executions since timestamp

#### `compozy schedule update` (Client Command)

- `--cron`: CRON expression
- `--enabled`: Enable/disable schedule

## TUI Architecture & Charmbracelet Integration

### Charmbracelet Ecosystem Overview

**Core TUI Packages:**

- **Bubble Tea**: TUI framework using Elm Architecture (model/update/view)
- **Lipgloss**: CSS-like styling for terminal layouts
- **Huh**: Interactive forms and prompts
- **Bubbles**: Pre-built components (tables, lists, spinners, progress bars)
- **Glamour**: Markdown rendering with ANSI styling

### Hybrid Architecture Pattern

```go
// cli/shared/output.go - Smart output manager
type OutputManager struct {
    interactive bool
    format      string
    quiet       bool
    noColor     bool
}

func (o *OutputManager) RenderWorkflowList(workflows []Workflow) error {
    // JSON always takes precedence for scripting
    if o.format == "json" {
        return o.renderJSON(workflows)
    }

    // Interactive TUI mode for complex operations
    if o.interactive && !o.quiet && isTerminal() {
        return o.renderInteractiveTable(workflows)
    }

    // Beautiful styled output with Lipgloss
    return o.renderStyledTable(workflows)
}

// Auto-detection logic
func (global *GlobalConfig) ShouldUseInteractive() bool {
    if global.OutputFormat == "json" || global.Quiet || global.NoInteractive {
        return false
    }
    if !isatty.IsTerminal(os.Stdout.Fd()) {
        return false // Piped output - disable TUI
    }
    return global.Interactive || isTerminal()
}
```

### TUI Implementation Strategy by Command

#### `compozy init` - Interactive Project Setup

```go
// Use Huh for guided project initialization
import "github.com/charmbracelet/huh"

func runInitInteractive() error {
    var projectName, template string

    form := huh.NewForm(
        huh.NewInput().
            Title("Project name").
            Placeholder("my-ai-project").
            Value(&projectName),
        huh.NewSelect[string]().
            Title("Choose a template").
            Options(
                huh.NewOption("Basic Agent", "basic-agent"),
                huh.NewOption("Data Pipeline", "data-pipeline"),
                huh.NewOption("Customer Support", "customer-support"),
            ).
            Value(&template),
        huh.NewConfirm().
            Title("Initialize Git repository?").
            Value(&initGit),
    )

    return form.Run()
}
```

#### `compozy workflow execute` - Real-time Progress

```go
// Use Bubble Tea for real-time execution monitoring
import tea "github.com/charmbracelet/bubbletea"
import "github.com/charmbracelet/bubbles/spinner"
import "github.com/charmbracelet/bubbles/progress"

type executeModel struct {
    spinner    spinner.Model
    progress   progress.Model
    logs       []string
    status     string
    execID     string
    client     *CompozyClient
}

func (m executeModel) Init() tea.Cmd {
    return tea.Batch(
        m.spinner.Tick,
        m.pollExecutionStatus(),
    )
}

func (m executeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case executionStatusMsg:
        m.status = msg.status
        m.progress.SetPercent(msg.progress)
        return m, m.pollExecutionStatus()
    case spinner.TickMsg:
        var cmd tea.Cmd
        m.spinner, cmd = m.spinner.Update(msg)
        return m, cmd
    }
    return m, nil
}

func (m executeModel) View() string {
    return lipgloss.JoinVertical(lipgloss.Left,
        titleStyle.Render("Executing Workflow: "+m.execID),
        "",
        m.spinner.View()+" "+m.status,
        m.progress.View(),
        "",
        logStyle.Render(strings.Join(m.logs, "\n")),
    )
}
```

#### `compozy workflow list` - Beautiful Tables

```go
// Use Bubbles table with Lipgloss styling
import "github.com/charmbracelet/bubbles/table"

func renderWorkflowTable(workflows []Workflow) string {
    columns := []table.Column{
        {Title: "ID", Width: 20},
        {Title: "Status", Width: 10},
        {Title: "Tasks", Width: 8},
        {Title: "Last Run", Width: 15},
    }

    rows := make([]table.Row, len(workflows))
    for i, wf := range workflows {
        status := "○ inactive"
        if wf.Active {
            status = "✓ active"
        }

        rows[i] = table.Row{
            wf.ID,
            status,
            fmt.Sprintf("%d", len(wf.Tasks)),
            wf.LastRun.Format("2 mins ago"),
        }
    }

    t := table.New(
        table.WithColumns(columns),
        table.WithRows(rows),
        table.WithFocused(true),
        table.WithHeight(7),
    )

    return tableStyle.Render(t.View())
}
```

### Lipgloss Style System

```go
// cli/shared/styles.go - Centralized styling
package shared

import "github.com/charmbracelet/lipgloss"

var (
    // Brand colors
    primaryColor = lipgloss.Color("#7D56F4")
    successColor = lipgloss.Color("#02BA84")
    warningColor = lipgloss.Color("#FF8700")
    errorColor   = lipgloss.Color("#FF5F87")

    // Text styles
    titleStyle = lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("#FAFAFA")).
        Background(primaryColor).
        Padding(0, 1)

    errorStyle = lipgloss.NewStyle().
        Bold(true).
        Foreground(errorColor)

    successStyle = lipgloss.NewStyle().
        Bold(true).
        Foreground(successColor)

    // Table styles
    tableStyle = lipgloss.NewStyle().
        BorderStyle(lipgloss.NormalBorder()).
        BorderForeground(lipgloss.Color("#383838"))

    // Log styles
    logStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("#767676")).
        MarginTop(1)
)

// Style helper functions
func RenderError(msg string) string {
    return errorStyle.Render("✗ " + msg)
}

func RenderSuccess(msg string) string {
    return successStyle.Render("✓ " + msg)
}

func RenderTitle(title string) string {
    return titleStyle.Render(title)
}
```

### Mode Detection and Fallbacks

```go
// cli/shared/tui.go - TUI utilities
package shared

import (
    "os"
    "github.com/mattn/go-isatty"
)

func isTerminal() bool {
    return isatty.IsTerminal(os.Stdout.Fd())
}

func shouldUseTUI(global *GlobalConfig) bool {
    // Explicit flags take precedence
    if global.NoInteractive {
        return false
    }
    if global.Interactive {
        return true
    }

    // Auto-detection rules
    if global.OutputFormat == "json" || global.Quiet {
        return false
    }

    // Check if output is piped
    if !isTerminal() {
        return false
    }

    // Check NO_COLOR environment variable
    if os.Getenv("NO_COLOR") != "" {
        return false
    }

    return true
}

func NewOutputManager(global *GlobalConfig) *OutputManager {
    return &OutputManager{
        interactive: shouldUseTUI(global),
        format:      global.OutputFormat,
        quiet:       global.Quiet,
        noColor:     global.NoColor || os.Getenv("NO_COLOR") != "",
    }
}
```

### Progressive Enhancement Strategy

**Week 1: Lipgloss Foundation**

```bash
# Before: Plain text output
$ compozy version
compozy version 1.0.0

# After: Styled output
$ compozy version
╭─────────────────────────────────╮
│ 🎭 Compozy CLI v1.0.0 │
│ AI Workflow Orchestration │
│ Built with ❤️ at Compozy │
╰─────────────────────────────────╯
```

**Week 2: Interactive Forms**

```bash
# Before: Multiple flags required
$ compozy init my-project --template basic-agent --git

# After: Interactive guided setup
$ compozy init
✨ Create new Compozy project

? Project name: › my-project
? Choose template:
  ▸ Basic Agent
    Data Pipeline
    Customer Support
? Initialize Git repository? (y/N) › Yes

✓ Project created successfully!
```

**Week 3: Real-time Monitoring**

```bash
# Before: Static output, manual polling
$ compozy workflow execute customer-support --input '{"id":"123"}'
Execution started: exec-456
$ compozy execution get exec-456 # Manual check

# After: Real-time progress with TUI
$ compozy workflow execute customer-support --input '{"id":"123"}'
┌─ Executing Workflow: customer-support ─┐
│ ⠋ Processing customer inquiry... │
│ ████████████████░░░░ 75% │
│ │
│ Recent logs: │
│ 14:32:01 Started agent initialization │
│ 14:32:03 Connected to knowledge base │
│ 14:32:05 Processing customer request... │
└─────────────────────────────────────────┘
```

## Technical Architecture

### File Structure

```
cli/
├── root.go                 # Root command with global flags
├── version.go              # Version command
├── config/                 # Configuration management
│   ├── global.go          # Global configuration struct
│   ├── loader.go          # Config file loading (compozy.yaml)
│   ├── resolver.go        # Flag/env/config resolution
│   └── env.go             # Environment variable handling
├── commands/               # Command implementations
│   ├── dev.go             # Development server (existing)
│   ├── init.go            # Project initialization with Huh forms
│   ├── workflow/          # Workflow commands
│   │   ├── list.go        # workflow list with Bubble Tea tables
│   │   ├── get.go         # workflow get with styled output
│   │   ├── execute.go     # workflow execute with real-time progress
│   │   └── tasks.go       # workflow tasks/agents/tools
│   ├── execution/         # Execution commands
│   │   ├── list.go        # execution list with beautiful tables
│   │   ├── get.go         # execution get with live monitoring
│   │   ├── control.go     # pause/resume/cancel (future)
│   │   └── signal.go      # execution signal
│   ├── schedule/          # Schedule commands
│   │   ├── list.go        # schedule list
│   │   ├── get.go         # schedule get
│   │   ├── update.go      # schedule update
│   │   └── delete.go      # schedule delete
│   ├── event/             # Event commands
│   │   └── send.go        # event send
│   └── config/            # Config commands
│       ├── validate.go    # config validate with styled errors
│       └── show.go        # config show
├── shared/                 # Shared utilities
│   ├── flags.go           # Common flag definitions
│   ├── output.go          # Smart output manager (TUI/JSON/styled)
│   ├── client.go          # HTTP API client
│   ├── errors.go          # Error handling with styling
│   ├── styles.go          # Lipgloss style definitions
│   └── tui.go             # TUI utilities and mode detection
├── tui/                    # TUI Components
│   ├── components/        # Reusable Bubble Tea components
│   │   ├── table.go       # Enhanced table component
│   │   ├── progress.go    # Progress indicators
│   │   ├── logs.go        # Log viewer component
│   │   └── forms.go       # Huh form helpers
│   ├── models/            # Bubble Tea models
│   │   ├── execute.go     # Workflow execution model
│   │   ├── monitor.go     # Real-time monitoring model
│   │   └── select.go      # Interactive selection model
│   └── views/             # View rendering logic
│       ├── workflows.go   # Workflow list/detail views
│       ├── executions.go  # Execution views
│       └── common.go      # Common view components
└── internal/               # Internal utilities
    ├── completion.go      # Shell completion
    └── templates.go       # Command templates
```

### Configuration Management Architecture

```go
// Global configuration available to all commands
type GlobalConfig struct {
    // Project & Files
    ConfigFile string `yaml:"config_file" env:"COMPOZY_CONFIG_FILE"`
    CWD        string `yaml:"cwd" env:"COMPOZY_CWD"`
    EnvFile    string `yaml:"env_file" env:"COMPOZY_ENV_FILE"`

    // Logging
    LogLevel  string `yaml:"log_level" env:"COMPOZY_LOG_LEVEL"`
    LogJSON   bool   `yaml:"log_json" env:"COMPOZY_LOG_JSON"`
    LogSource bool   `yaml:"log_source" env:"COMPOZY_LOG_SOURCE"`
    Debug     bool   `yaml:"debug" env:"COMPOZY_DEBUG"`

    // Output & TUI
    OutputFormat  string `yaml:"output_format" env:"COMPOZY_OUTPUT_FORMAT"`
    Quiet         bool   `yaml:"quiet" env:"COMPOZY_QUIET"`
    Verbose       bool   `yaml:"verbose" env:"COMPOZY_VERBOSE"`
    Interactive   bool   `yaml:"interactive" env:"COMPOZY_INTERACTIVE"`
    NoInteractive bool   `yaml:"no_interactive" env:"COMPOZY_NO_INTERACTIVE"`
    NoColor       bool   `yaml:"no_color" env:"NO_COLOR"`

    // Server Connection (for client commands)
    ServerURL string        `yaml:"server_url" env:"COMPOZY_SERVER_URL"`
    Timeout   time.Duration `yaml:"timeout" env:"COMPOZY_TIMEOUT"`
    APIKey    string        `yaml:"api_key" env:"COMPOZY_API_KEY"`
}

// Configuration resolver with hierarchy
type ConfigResolver struct {
    global     *GlobalConfig
    envVars    map[string]string
    configFile map[string]interface{}
    flags      map[string]interface{}
}

// Configuration hierarchy (highest to lowest priority):
// 1. CLI flags
// 2. Environment variables (COMPOZY_*)
// 3. Project configuration file (compozy.yaml)
// 4. Global defaults
func (r *ConfigResolver) Resolve(key string, target interface{}) error
```

### HTTP Client Architecture

```go
// API client for server communication
type CompozyClient struct {
    BaseURL    string
    HTTPClient *http.Client
    APIKey     string
    Logger     logger.Logger
}

// Client methods mapping to API endpoints
func (c *CompozyClient) ListWorkflows(ctx context.Context) ([]Workflow, error)
func (c *CompozyClient) GetWorkflow(ctx context.Context, id string) (*Workflow, error)
func (c *CompozyClient) ExecuteWorkflow(ctx context.Context, id string, req ExecuteWorkflowRequest) (*ExecutionResponse, error)
func (c *CompozyClient) ListExecutions(ctx context.Context, filters ExecutionFilters) ([]Execution, error)
func (c *CompozyClient) GetExecution(ctx context.Context, execID string) (*Execution, error)
func (c *CompozyClient) PauseExecution(ctx context.Context, execID string) error
func (c *CompozyClient) ResumeExecution(ctx context.Context, execID string) error
func (c *CompozyClient) CancelExecution(ctx context.Context, execID string, reason string) error
func (c *CompozyClient) SendSignal(ctx context.Context, execID, signalName string, payload map[string]interface{}) error

// Configuration from global config
func NewCompozyClient(global *GlobalConfig) *CompozyClient {
    return &CompozyClient{
        BaseURL: global.ServerURL,
        HTTPClient: &http.Client{Timeout: global.Timeout},
        APIKey: global.APIKey,
        Logger: logger.FromContext(ctx),
    }
}
```

### Environment Variable Patterns

Following current patterns but with consistent COMPOZY\_ prefix:

```bash
# Global Configuration
COMPOZY_CONFIG_FILE=compozy.yaml
COMPOZY_CWD=/path/to/project
COMPOZY_ENV_FILE=.env
COMPOZY_LOG_LEVEL=info
COMPOZY_DEBUG=false
COMPOZY_SERVER_URL=http://localhost:3001
COMPOZY_TIMEOUT=30s

# Dev Command (Server) - Keep existing patterns for compatibility
DB_HOST=localhost
DB_PORT=5432
DB_USER=compozy
DB_PASSWORD=password
DB_NAME=compozy
TEMPORAL_HOST=localhost:7233
TEMPORAL_NAMESPACE=default
OPENAI_API_KEY=sk-...

# Future: Prefixed versions for consistency
COMPOZY_DB_HOST=localhost
COMPOZY_TEMPORAL_HOST=localhost:7233
COMPOZY_OPENAI_API_KEY=sk-...
```

### Error Handling Strategy

```go
// CLI-specific error types
type CLIError struct {
    Command   string
    Operation string
    Err       error
}

func (e *CLIError) Error() string {
    return fmt.Sprintf("command '%s' %s: %s", e.Command, e.Operation, e.Err.Error())
}

// Map API errors to user-friendly messages
func HandleAPIError(err error, operation string) error {
    switch {
    case isConnectionError(err):
        return &CLIError{
            Operation: operation,
            Err: fmt.Errorf("cannot connect to Compozy server. Is it running? Use 'compozy dev' to start"),
        }
    case isNotFoundError(err):
        return &CLIError{
            Operation: operation,
            Err: fmt.Errorf("%s not found. Use 'compozy workflow list' to see available workflows", operation),
        }
    case isValidationError(err):
        return &CLIError{
            Operation: operation,
            Err: fmt.Errorf("validation failed: %s. Use 'compozy config validate' to check configuration", err),
        }
    default:
        return &CLIError{
            Operation: operation,
            Err: fmt.Errorf("operation failed: %s", err),
        }
    }
}

// Context-aware logging setup
func SetupCommandContext(cmd *cobra.Command) (context.Context, *GlobalConfig, error) {
    global, err := LoadGlobalConfig(cmd)
    if err != nil {
        return nil, nil, fmt.Errorf("failed to load configuration: %w", err)
    }

    logger := logger.SetupLogger(global.LogLevel, global.LogJSON, global.LogSource)
    ctx := logger.ContextWithLogger(context.Background(), logger)

    return ctx, global, nil
}
```

### Command Implementation Patterns

Following the current `dev` command structure, here's the recommended pattern for new commands:

```go
// Example: cli/commands/workflow/execute.go
package workflow

import (
    "context"
    "encoding/json"
    "fmt"
    "io/ioutil"

    "github.com/spf13/cobra"
    "github.com/compozy/cli/config"
    "github.com/compozy/cli/shared"
)

type ExecuteOptions struct {
    WorkflowID string
    Input      string
    InputFile  string
    TaskID     string
    Wait       bool
    Follow     bool
}

func NewExecuteCommand() *cobra.Command {
    opts := &ExecuteOptions{}

    cmd := &cobra.Command{
        Use:   "execute <workflow-id>",
        Short: "Execute a workflow",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            opts.WorkflowID = args[0]
            return runExecute(cmd, opts)
        },
    }

    // Command-specific flags
    cmd.Flags().StringVar(&opts.Input, "input", "", "Input data as JSON string")
    cmd.Flags().StringVar(&opts.InputFile, "input-file", "", "Input data from file")
    cmd.Flags().StringVar(&opts.TaskID, "task", "", "Execute specific task only")
    cmd.Flags().BoolVar(&opts.Wait, "wait", false, "Wait for execution completion")
    cmd.Flags().BoolVar(&opts.Follow, "follow", false, "Follow execution logs")

    return cmd
}

func runExecute(cmd *cobra.Command, opts *ExecuteOptions) error {
    // Setup context and global config
    ctx, global, err := shared.SetupCommandContext(cmd)
    if err != nil {
        return err
    }

    // Create API client
    client := shared.NewCompozyClient(global)

    // Prepare input data
    var inputData map[string]interface{}
    if opts.InputFile != "" {
        data, err := ioutil.ReadFile(opts.InputFile)
        if err != nil {
            return shared.HandleAPIError(err, "reading input file")
        }
        if err := json.Unmarshal(data, &inputData); err != nil {
            return shared.HandleAPIError(err, "parsing input file")
        }
    } else if opts.Input != "" {
        if err := json.Unmarshal([]byte(opts.Input), &inputData); err != nil {
            return shared.HandleAPIError(err, "parsing input data")
        }
    }

    // Execute workflow
    req := ExecuteWorkflowRequest{
        Input:  inputData,
        TaskID: opts.TaskID,
    }

    resp, err := client.ExecuteWorkflow(ctx, opts.WorkflowID, req)
    if err != nil {
        return shared.HandleAPIError(err, "executing workflow")
    }

    // Output result
    if global.OutputFormat == "json" {
        return shared.OutputJSON(resp)
    }

    fmt.Printf("Workflow executed successfully:\n")
    fmt.Printf("  Execution ID: %s\n", resp.Data.ExecID)
    fmt.Printf("  Status URL: %s\n", resp.Data.ExecURL)

    // Handle wait/follow options
    if opts.Wait || opts.Follow {
        return followExecution(ctx, client, resp.Data.ExecID, opts.Follow)
    }

    return nil
}

func followExecution(ctx context.Context, client *shared.CompozyClient, execID string, follow bool) error {
    // Implementation for following execution status
    // Poll execution status and optionally stream logs
    return nil
}
```

### Shared Utilities Implementation

```go
// cli/shared/flags.go
package shared

import "github.com/spf13/cobra"

// Add global flags to root command
func AddGlobalFlags(cmd *cobra.Command) {
    // Infrastructure & Configuration
    cmd.PersistentFlags().StringP("config", "c", "compozy.yaml", "Configuration file")
    cmd.PersistentFlags().String("cwd", "", "Working directory")
    cmd.PersistentFlags().String("env-file", ".env", "Environment file")

    // Logging & Output
    cmd.PersistentFlags().String("log-level", "info", "Log level (debug, info, warn, error)")
    cmd.PersistentFlags().Bool("log-json", false, "JSON log format")
    cmd.PersistentFlags().Bool("log-source", false, "Include source in logs")
    cmd.PersistentFlags().Bool("debug", false, "Debug mode")
    cmd.PersistentFlags().StringP("output", "o", "table", "Output format (table, json, yaml)")
    cmd.PersistentFlags().BoolP("quiet", "q", false, "Suppress non-essential output")
    cmd.PersistentFlags().BoolP("verbose", "v", false, "Verbose output")

    // Server Connection
    cmd.PersistentFlags().String("server-url", "http://localhost:3001", "Compozy server URL")
    cmd.PersistentFlags().Duration("timeout", 30*time.Second, "Request timeout")
    cmd.PersistentFlags().String("api-key", "", "API authentication key")
}

// Extract common flags for database configuration (used by dev command)
func AddDatabaseFlags(cmd *cobra.Command) {
    cmd.Flags().String("db-host", "", "Database host")
    cmd.Flags().Int("db-port", 5432, "Database port")
    cmd.Flags().String("db-user", "", "Database user")
    cmd.Flags().String("db-password", "", "Database password")
    cmd.Flags().String("db-name", "", "Database name")
    cmd.Flags().String("db-ssl-mode", "disable", "Database SSL mode")
    cmd.Flags().String("db-conn-string", "", "Database connection string")
}

// Extract common flags for Temporal configuration (used by dev command)
func AddTemporalFlags(cmd *cobra.Command) {
    cmd.Flags().String("temporal-host", "", "Temporal server host:port")
    cmd.Flags().String("temporal-namespace", "default", "Temporal namespace")
    cmd.Flags().String("temporal-task-queue", "", "Temporal task queue")
}
```

### Root Command Integration

```go
// cli/root.go
package cli

import (
    "github.com/spf13/cobra"
    "github.com/compozy/cli/commands"
    "github.com/compozy/cli/commands/workflow"
    "github.com/compozy/cli/commands/execution"
    "github.com/compozy/cli/shared"
)

func NewRootCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "compozy",
        Short: "Compozy - AI Workflow Orchestration Engine",
        Long:  `Compozy is a workflow orchestration engine for AI agents.`,
    }

    // Add global flags
    shared.AddGlobalFlags(cmd)

    // Add commands
    cmd.AddCommand(commands.NewDevCommand())     // Existing dev command
    cmd.AddCommand(commands.NewInitCommand())    // New init command
    cmd.AddCommand(commands.NewVersionCommand()) // Version command

    // Add command groups
    cmd.AddCommand(workflow.NewWorkflowCommand())   // workflow subcommands
    cmd.AddCommand(execution.NewExecutionCommand()) // execution subcommands
    cmd.AddCommand(schedule.NewScheduleCommand())   // schedule subcommands
    cmd.AddCommand(event.NewEventCommand())         // event subcommands
    cmd.AddCommand(config.NewConfigCommand())       // config subcommands

    return cmd
}

// Workflow command group
func NewWorkflowCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "workflow",
        Short: "Workflow management commands",
    }

    cmd.AddCommand(workflow.NewListCommand())
    cmd.AddCommand(workflow.NewGetCommand())
    cmd.AddCommand(workflow.NewExecuteCommand())
    cmd.AddCommand(workflow.NewTasksCommand())

    return cmd
}
```

### Testing Strategy

```go
// cli/commands/workflow/execute_test.go
package workflow

import (
    "testing"
    "net/http/httptest"
    "github.com/stretchr/testify/assert"
    "github.com/spf13/cobra"
)

func TestExecuteCommand(t *testing.T) {
    tests := []struct {
        name       string
        args       []string
        serverResp string
        wantErr    bool
        wantOutput string
    }{
        {
            name: "successful execution",
            args: []string{"execute", "my-workflow", "--input", `{"key": "value"}`},
            serverResp: `{"message": "success", "data": {"exec_id": "exec-123", "exec_url": "/executions/exec-123"}}`,
            wantOutput: "Workflow executed successfully",
        },
        {
            name:    "missing workflow id",
            args:    []string{"execute"},
            wantErr: true,
        },
        {
            name:    "invalid json input",
            args:    []string{"execute", "my-workflow", "--input", "invalid-json"},
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Create mock server
            server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                w.Header().Set("Content-Type", "application/json")
                w.WriteHeader(http.StatusOK)
                w.Write([]byte(tt.serverResp))
            }))
            defer server.Close()

            // Create command with test server URL
            cmd := NewExecuteCommand()
            cmd.SetArgs(tt.args)
            cmd.Flags().Set("server-url", server.URL)

            // Execute command
            err := cmd.Execute()

            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

## Dependencies & Setup

### Required Charmbracelet Dependencies

```go
// go.mod additions
require (
    github.com/charmbracelet/bubbletea v0.24.2
    github.com/charmbracelet/lipgloss v0.9.1
    github.com/charmbracelet/huh v0.2.3
    github.com/charmbracelet/bubbles v0.16.1
    github.com/charmbracelet/glamour v0.6.0
    github.com/mattn/go-isatty v0.0.20  // Terminal detection
)
```

### Environment Variables

```bash
# TUI Configuration
COMPOZY_INTERACTIVE=true    # Force interactive mode
COMPOZY_NO_INTERACTIVE=true # Force non-interactive mode
NO_COLOR=1                  # Disable all colors (standard)
COMPOZY_OUTPUT_FORMAT=json  # Default output format

# Existing variables remain unchanged
COMPOZY_SERVER_URL=http://localhost:3001
COMPOZY_LOG_LEVEL=info
```

## Implementation Roadmap

### Phase 1: TUI Foundation (Week 1-2)

**Core Infrastructure with Lipgloss Styling:**

1. Add Charmbracelet dependencies to go.mod
2. Implement `GlobalConfig` with TUI flags in `cli/config/`
3. Create `OutputManager` with hybrid rendering in `cli/shared/`
4. Implement universal Lipgloss styling system
5. Setup terminal detection and mode switching

**Essential Commands with Beautiful Output:**

1. `compozy version` - Styled version information
2. `compozy config show` - Formatted configuration display
3. `compozy config validate` - Styled error reporting
4. `compozy workflow list` - Beautiful table output

**TUI Infrastructure:**

- Terminal detection (`isatty`)
- Mode switching logic (interactive vs scriptable)
- Lipgloss style system with brand colors
- JSON output always available

### Phase 2: Interactive Commands (Week 3-4)

**High-Value TUI Integration:**

1. `compozy init` - **Huh interactive forms** for project setup
2. `compozy workflow execute` - **Bubble Tea real-time progress**
3. `compozy execution list` - **Bubbles tables** with navigation
4. `compozy execution get` - Formatted execution details

**Enhanced User Experience:**

- Interactive project initialization
- Real-time execution monitoring
- Filterable, navigable tables
- Progress indicators and spinners

### Phase 3: Advanced TUI Features (Week 5-6)

**Rich Interactive Experiences:**

1. `compozy workflow list` - Interactive selection and filtering
2. `compozy execution get --follow` - Live dashboard with log streaming
3. Enhanced error handling with styled messages
4. Context-aware help and documentation

**Advanced Components:**

- Multi-selection interfaces
- Real-time log streaming
- Interactive debugging
- Advanced table features (sorting, filtering)

### Phase 4: Production Polish (Week 7-8)

**Testing & Quality:**

1. TUI component testing strategy
2. Snapshot testing for visual components
3. Integration tests with mock terminals
4. Cross-platform compatibility testing

**Performance & Compatibility:**

1. Graceful fallbacks for limited terminals
2. Performance optimization for large datasets
3. Memory usage optimization
4. CI/CD integration with automated testing

## Success Metrics

### MVP Success Criteria

- **Functionality**: All major API endpoints accessible via beautiful CLI interface
- **TUI Experience**: Interactive commands provide world-class developer experience
- **Hybrid Compatibility**: Perfect JSON output for automation, beautiful TUI for humans
- **Performance**: Commands complete within acceptable time limits with real-time feedback

### Quality Standards

- **Test Coverage**: >80% test coverage for all commands (including TUI components)
- **Visual Consistency**: Consistent styling across all commands with Lipgloss
- **Accessibility**: Graceful fallbacks for limited terminals and screen readers
- **Documentation**: Complete help text with visual examples for TUI features
- **Error Handling**: Beautifully styled error messages with actionable suggestions
- **Automation Compatibility**: JSON output works perfectly in scripts and CI/CD

### Developer Experience Goals

- **Time to Value**: New users can initialize and execute workflows within 2 minutes
- **Discoverability**: Interactive modes guide users to discover features naturally
- **Efficiency**: Power users can bypass TUI with flags for maximum speed
- **Visual Appeal**: CLI output rivals modern developer tools like GitHub CLI, K9s

This comprehensive plan delivers a CLI that combines the power of workflow orchestration with the beauty and usability of modern terminal interfaces, setting a new standard for developer tools in the AI/automation space.
