# Design Document

## Overview

This design transforms Compozy's CLI from a basic development server tool into a comprehensive workflow orchestration interface. The enhancement provides complete API coverage through both beautiful Terminal User Interface (TUI) components and machine-readable JSON output, ensuring excellent developer experience while maintaining perfect automation compatibility.

The design leverages existing patterns from the auth module and TUI components, extending them to cover all workflow management operations. The architecture maintains strict separation between presentation logic and business logic, enabling consistent behavior across output formats.

## Architecture

### Command Structure

The CLI follows a hierarchical command structure using Cobra, organized by functional domains:

```
compozy
├── init                    # Project initialization
├── workflow               # Workflow management
│   ├── list
│   ├── get <id>
│   └── execute <id>
├── execution              # Execution management
│   ├── list
│   ├── get <exec-id>
│   └── signal <exec-id> <signal>
├── schedule               # Schedule management
│   ├── list
│   ├── update <workflow-id>
│   └── delete <workflow-id>
├── event                  # Event management
│   └── send <event-name>
├── config                 # Configuration management (existing)
│   ├── show
│   ├── validate
│   └── diagnostics
└── auth                   # Authentication (existing)
```

### Output Mode Architecture

The design implements a dual-mode architecture supporting both interactive TUI and automation-friendly JSON output:

#### Mode Detection Strategy

```go
type OutputMode int

const (
    ModeAuto OutputMode = iota  // Auto-detect based on terminal
    ModeTUI                     // Force interactive TUI
    ModeJSON                    // Force JSON output
)

func DetectOutputMode(cmd *cobra.Command) OutputMode {
    // Check explicit format flags
    if format, _ := cmd.Flags().GetString("format"); format == "json" {
        return ModeJSON
    }

    if interactive, _ := cmd.Flags().GetBool("interactive"); interactive {
        return ModeTUI
    }

    // Auto-detect based on terminal capabilities
    if isatty.IsTerminal(os.Stdout.Fd()) && !isOutputRedirected() {
        return ModeTUI
    }

    return ModeJSON
}
```

#### Command Execution Pattern

Following the existing auth module pattern, each command implements dual handlers:

```go
type CommandHandlers struct {
    TUI  func(ctx context.Context, cmd *cobra.Command, client *APIClient, args []string) error
    JSON func(ctx context.Context, cmd *cobra.Command, client *APIClient, args []string) error
}

func ExecuteCommand(cmd *cobra.Command, handlers CommandHandlers, args []string) error {
    mode := DetectOutputMode(cmd)
    client := NewAPIClient(cmd)
    ctx := context.Background()

    switch mode {
    case ModeTUI:
        return handlers.TUI(ctx, cmd, client, args)
    case ModeJSON:
        return handlers.JSON(ctx, cmd, client, args)
    }
}
```

### API Client Architecture

A unified API client provides consistent server communication across all commands:

```go
type APIClient struct {
    baseURL    string
    httpClient *http.Client
    authToken  string
}

type WorkflowService interface {
    List(ctx context.Context, filters WorkflowFilters) ([]Workflow, error)
    Get(ctx context.Context, id string) (*WorkflowDetail, error)
    Execute(ctx context.Context, id string, input ExecutionInput) (*ExecutionResult, error)
}

type ExecutionService interface {
    List(ctx context.Context, filters ExecutionFilters) ([]Execution, error)
    Get(ctx context.Context, id string) (*ExecutionDetail, error)
    Signal(ctx context.Context, execID, signal string, payload interface{}) error
    Follow(ctx context.Context, execID string) (<-chan ExecutionEvent, error)
}
```

## Components and Interfaces

### TUI Component System

Building on the existing TUI infrastructure, the design extends components for workflow-specific interfaces:

#### Enhanced Table Component

```go
type TableConfig struct {
    Headers     []string
    Rows        [][]string
    Sortable    []bool
    Filterable  bool
    Paginated   bool
    PageSize    int
}

type WorkflowTable struct {
    *table.Model
    workflows []Workflow
    filters   WorkflowFilters
}

func (wt *WorkflowTable) Update(msg tea.Msg) (tea.Model, tea.Cmd)
func (wt *WorkflowTable) View() string
```

#### Real-time Progress Component

```go
type ProgressMonitor struct {
    executionID string
    events      <-chan ExecutionEvent
    logs        []LogEntry
    status      ExecutionStatus
}

func (pm *ProgressMonitor) Update(msg tea.Msg) (tea.Model, tea.Cmd)
func (pm *ProgressMonitor) View() string
```

#### Interactive Forms

```go
type FormField struct {
    Label       string
    Type        FieldType
    Required    bool
    Validation  func(string) error
    Options     []string  // For select fields
}

type InteractiveForm struct {
    fields   []FormField
    values   map[string]string
    focused  int
    errors   map[string]string
}
```

### JSON Output System

Consistent JSON output structure across all commands:

```go
type JSONResponse struct {
    Success   bool        `json:"success"`
    Data      interface{} `json:"data,omitempty"`
    Error     *APIError   `json:"error,omitempty"`
    Metadata  *Metadata   `json:"metadata,omitempty"`
}

type Metadata struct {
    Timestamp   time.Time `json:"timestamp"`
    RequestID   string    `json:"request_id,omitempty"`
    Pagination  *PaginationInfo `json:"pagination,omitempty"`
}
```

### Configuration Management

Enhanced configuration system supporting CLI-specific settings:

```go
type CLIConfig struct {
    ServerURL     string        `koanf:"server_url"`
    DefaultFormat string        `koanf:"default_format"`
    ColorMode     string        `koanf:"color_mode"`
    PageSize      int           `koanf:"page_size"`
    Timeout       time.Duration `koanf:"timeout"`
}
```

## Data Models

### Core Workflow Types

```go
type Workflow struct {
    ID          string            `json:"id"`
    Name        string            `json:"name"`
    Description string            `json:"description"`
    Version     string            `json:"version"`
    Status      WorkflowStatus    `json:"status"`
    CreatedAt   time.Time         `json:"created_at"`
    UpdatedAt   time.Time         `json:"updated_at"`
    Tags        []string          `json:"tags"`
    Metadata    map[string]string `json:"metadata"`
}

type WorkflowDetail struct {
    Workflow
    Tasks       []Task            `json:"tasks"`
    Inputs      []InputSchema     `json:"inputs"`
    Outputs     []OutputSchema    `json:"outputs"`
    Schedule    *Schedule         `json:"schedule,omitempty"`
    Statistics  *WorkflowStats    `json:"statistics"`
}

type Execution struct {
    ID          string            `json:"id"`
    WorkflowID  string            `json:"workflow_id"`
    Status      ExecutionStatus   `json:"status"`
    StartedAt   time.Time         `json:"started_at"`
    CompletedAt *time.Time        `json:"completed_at,omitempty"`
    Duration    *time.Duration    `json:"duration,omitempty"`
    Input       interface{}       `json:"input,omitempty"`
    Output      interface{}       `json:"output,omitempty"`
    Error       *ExecutionError   `json:"error,omitempty"`
}

type ExecutionDetail struct {
    Execution
    Logs        []LogEntry        `json:"logs"`
    TaskResults []TaskResult      `json:"task_results"`
    Metrics     *ExecutionMetrics `json:"metrics"`
}
```

### Event and Signal Types

```go
type Event struct {
    Name      string      `json:"name"`
    Payload   interface{} `json:"payload,omitempty"`
    Timestamp time.Time   `json:"timestamp"`
    Source    string      `json:"source"`
}

type Signal struct {
    Name      string      `json:"name"`
    Payload   interface{} `json:"payload,omitempty"`
    TargetID  string      `json:"target_id"`
}
```

### Schedule Management

```go
type Schedule struct {
    WorkflowID  string    `json:"workflow_id"`
    CronExpr    string    `json:"cron_expression"`
    Enabled     bool      `json:"enabled"`
    NextRun     time.Time `json:"next_run"`
    LastRun     *time.Time `json:"last_run,omitempty"`
    Timezone    string    `json:"timezone"`
}
```

## Error Handling

### Structured Error System

```go
type CLIError struct {
    Code       string `json:"code"`
    Message    string `json:"message"`
    Details    string `json:"details,omitempty"`
    Suggestion string `json:"suggestion,omitempty"`
}

func (e *CLIError) Error() string {
    return e.Message
}

// Error categories
const (
    ErrCodeValidation   = "VALIDATION_ERROR"
    ErrCodeNotFound     = "NOT_FOUND"
    ErrCodeUnauthorized = "UNAUTHORIZED"
    ErrCodeServerError  = "SERVER_ERROR"
    ErrCodeTimeout      = "TIMEOUT"
)
```

### Error Display Strategy

TUI Mode:

- Styled error messages with colors and icons
- Contextual help and suggestions
- Interactive error recovery options

JSON Mode:

- Structured error objects
- Consistent error codes for programmatic handling
- Detailed error context for debugging

### Graceful Degradation

```go
func HandleError(err error, mode OutputMode) {
    switch mode {
    case ModeTUI:
        displayStyledError(err)
        showSuggestions(err)
    case ModeJSON:
        outputJSONError(err)
    }
}
```

## Testing Strategy

### Unit Testing Approach

1. **Command Logic Testing**: Test command parsing, validation, and routing
2. **API Client Testing**: Mock HTTP responses for all API interactions
3. **TUI Component Testing**: Test component state management and user interactions
4. **JSON Output Testing**: Validate JSON structure and content
5. **Error Handling Testing**: Test error scenarios and recovery

### Integration Testing

1. **End-to-End Command Testing**: Test complete command execution flows
2. **API Integration Testing**: Test against real Compozy server instances
3. **Terminal Compatibility Testing**: Test across different terminal environments
4. **Output Format Testing**: Validate both TUI and JSON outputs

### Test Structure

```go
func TestWorkflowListCommand(t *testing.T) {
    tests := []struct {
        name     string
        args     []string
        flags    map[string]string
        mockResp *WorkflowListResponse
        wantErr  bool
        mode     OutputMode
    }{
        {
            name: "list workflows in JSON mode",
            flags: map[string]string{"format": "json"},
            mockResp: &WorkflowListResponse{...},
            mode: ModeJSON,
        },
        {
            name: "list workflows in TUI mode",
            mockResp: &WorkflowListResponse{...},
            mode: ModeTUI,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

### Mock Strategy

```go
type MockAPIClient struct {
    workflows   []Workflow
    executions  []Execution
    schedules   []Schedule
    responses   map[string]interface{}
}

func (m *MockAPIClient) ListWorkflows(ctx context.Context, filters WorkflowFilters) ([]Workflow, error) {
    return m.workflows, nil
}
```

## Implementation Phases

### Phase 1: Foundation (Core Infrastructure)

- Enhanced API client with unified error handling
- Extended TUI component system
- Dual-mode command execution framework
- Configuration management enhancements

### Phase 2: Workflow Management

- `compozy workflow` command group
- Workflow listing with filtering and sorting
- Workflow detail views with task information
- Workflow execution with input handling

### Phase 3: Execution Monitoring

- `compozy execution` command group
- Execution listing and filtering
- Real-time execution following with TUI progress
- Execution detail views with logs and metrics

### Phase 4: Advanced Features

- `compozy schedule` command group for schedule management
- `compozy event` command group for event handling
- Interactive project initialization
- Enhanced configuration diagnostics

### Phase 5: Polish and Optimization

- Performance optimizations for large datasets
- Advanced TUI features (search, bulk operations)
- Comprehensive error handling and recovery
- Documentation and help system enhancements

## Security Considerations

### Authentication Integration

- Leverage existing auth module for API key management
- Support for multiple authentication methods
- Secure credential storage and transmission

### Input Validation

- Strict validation of all user inputs
- Protection against injection attacks
- Safe handling of file paths and URLs

### Output Sanitization

- Prevent sensitive data leakage in logs
- Redact credentials in configuration displays
- Safe handling of user-provided content

## Performance Considerations

### Efficient Data Handling

- Pagination for large result sets
- Streaming for real-time data (execution logs)
- Caching for frequently accessed data

### TUI Optimization

- Lazy loading for large tables
- Efficient rendering for smooth interactions
- Memory management for long-running sessions

### Network Optimization

- Connection pooling for API requests
- Request batching where appropriate
- Timeout handling and retry logic
