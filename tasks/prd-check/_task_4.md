## markdown

## status: pending

<task_context>
<domain>scripts/markdown/ui</domain>
<type>implementation</type>
<scope>presentation_layer</scope>
<complexity>medium</complexity>
<dependencies>task_1.0</dependencies>
</task_context>

# Task 4.0: UI Layer - Form Builders and Components

## Overview

Extract and refactor all UI-related code from check.go into modular, reusable components. This includes Huh form builders for CLI input collection and Bubble Tea UI components for interactive job monitoring and visualization.

This task can be executed in parallel with Tasks 2.0 (Infrastructure) and 3.0 (Core Logic) after Task 1.0 is complete.

<critical>**MUST READ BEFORE STARTING** @.cursor/rules/go-coding-standards.mdc and @.cursor/rules/architecture.mdc</critical>

<requirements>
- UI components have no business logic
- Depend only on port interfaces for notifications
- All functions < 50 lines
- Context-first APIs
- Use `logger.FromContext(ctx)` for logging
- Proper state management in Bubble Tea models
- Thread-safe event handling
- No global state
</requirements>

## Subtasks

- [ ] 4.1 Implement Huh form builders for CLI input collection
- [ ] 4.2 Implement Bubble Tea UI components (model, sidebar, main content, logs)
- [ ] 4.3 Implement UI styling and themes (Lipgloss)
- [ ] 4.4 Implement UI event handling and state management
- [ ] 4.5 Write unit tests for form builders
- [ ] 4.6 Write integration tests for UI workflows

## Implementation Details

### 4.1 Huh Form Builders (ui/forms/)

**forms/builder.go**:
```go
type FormBuilder struct {
    form   *huh.Form
    groups []*huh.Group
}

func NewFormBuilder() *FormBuilder {
    return &FormBuilder{
        groups: make([]*huh.Group, 0),
    }
}

func (b *FormBuilder) AddField(field huh.Field) *FormBuilder {
    // Add field to current group
    if len(b.groups) == 0 {
        b.groups = append(b.groups, huh.NewGroup(field))
    } else {
        b.groups[len(b.groups)-1].Fields = append(b.groups[len(b.groups)-1].Fields, field)
    }
    return b
}

func (b *FormBuilder) Build() *huh.Form {
    b.form = huh.NewForm(b.groups...)
    return b.form
}

func (b *FormBuilder) Run(ctx context.Context) error {
    if b.form == nil {
        return fmt.Errorf("form not built")
    }
    return b.form.RunWithContext(ctx)
}
```

**forms/inputs.go**:
```go
type InputFactory struct{}

func NewInputFactory() *InputFactory {
    return &InputFactory{}
}

// CreatePRInput creates a PR number input field
func (f *InputFactory) CreatePRInput(target *string) huh.Field {
    return huh.NewInput().
        Title("PR Number").
        Description("Enter the pull request number to process").
        Placeholder("e.g., 259").
        Value(target).
        Validate(func(s string) error {
            if s == "" {
                return fmt.Errorf("PR number is required")
            }
            if _, err := strconv.Atoi(s); err != nil {
                return fmt.Errorf("PR number must be numeric")
            }
            return nil
        })
}

// CreateIssuesDirInput creates an issues directory input field
func (f *InputFactory) CreateIssuesDirInput(target *string, defaultVal string) huh.Field {
    return huh.NewInput().
        Title("Issues Directory").
        Description("Directory containing issue markdown files").
        Placeholder(defaultVal).
        Value(target)
}

// CreateConcurrentInput creates a concurrency input field
func (f *InputFactory) CreateConcurrentInput(target *string) huh.Field {
    return huh.NewInput().
        Title("Concurrent Jobs").
        Description("Number of jobs to run in parallel").
        Placeholder("1").
        Value(target).
        Validate(func(s string) error {
            if s == "" {
                return nil // Will use default
            }
            n, err := strconv.Atoi(s)
            if err != nil {
                return fmt.Errorf("must be a number")
            }
            if n < 1 {
                return fmt.Errorf("must be at least 1")
            }
            return nil
        })
}

// CreateBatchSizeInput creates a batch size input field
func (f *InputFactory) CreateBatchSizeInput(target *string) huh.Field {
    return huh.NewInput().
        Title("Batch Size").
        Description("Number of issues per batch").
        Placeholder("3").
        Value(target).
        Validate(func(s string) error {
            if s == "" {
                return nil // Will use default
            }
            n, err := strconv.Atoi(s)
            if err != nil {
                return fmt.Errorf("must be a number")
            }
            if n < 1 {
                return fmt.Errorf("must be at least 1")
            }
            return nil
        })
}

// CreateIDESelectInput creates an IDE selection field
func (f *InputFactory) CreateIDESelectInput(target *string) huh.Field {
    return huh.NewSelect[string]().
        Title("IDE Tool").
        Description("Select the IDE tool to use").
        Options(
            huh.NewOption("Codex", types.IDECodex),
            huh.NewOption("Claude", types.IDEClaude),
            huh.NewOption("Droid", types.IDEDroid),
        ).
        Value(target)
}

// CreateModelInput creates a model input field
func (f *InputFactory) CreateModelInput(target *string, ide string) huh.Field {
    defaultModel := types.DefaultCodexModel
    if ide == types.IDEClaude {
        defaultModel = types.DefaultClaudeModel
    }

    return huh.NewInput().
        Title("Model").
        Description("AI model to use").
        Placeholder(defaultModel).
        Value(target)
}

// CreateReasoningEffortInput creates a reasoning effort selection field
func (f *InputFactory) CreateReasoningEffortInput(target *string) huh.Field {
    return huh.NewSelect[string]().
        Title("Reasoning Effort").
        Description("How deeply the AI should think").
        Options(
            huh.NewOption("Low (fast)", "low"),
            huh.NewOption("Medium (balanced)", "medium"),
            huh.NewOption("High (thorough)", "high"),
        ).
        Value(target)
}

// CreateConfirmInput creates a confirmation field
func (f *InputFactory) CreateConfirmInput(title, description string, target *bool) huh.Field {
    return huh.NewConfirm().
        Title(title).
        Description(description).
        Value(target)
}

// CreateTailLinesInput creates a tail lines input field
func (f *InputFactory) CreateTailLinesInput(target *string) huh.Field {
    return huh.NewInput().
        Title("Tail Lines").
        Description("Number of log lines to show in UI").
        Placeholder("5").
        Value(target).
        Validate(func(s string) error {
            if s == "" {
                return nil
            }
            n, err := strconv.Atoi(s)
            if err != nil {
                return fmt.Errorf("must be a number")
            }
            if n < 0 {
                return fmt.Errorf("must be non-negative")
            }
            return nil
        })
}
```

**forms/validators.go**:
```go
type Validator struct{}

func NewValidator() *Validator {
    return &Validator{}
}

func (v *Validator) ValidatePR(pr string) error {
    if pr == "" {
        return fmt.Errorf("PR number is required")
    }
    if _, err := strconv.Atoi(pr); err != nil {
        return fmt.Errorf("PR number must be numeric")
    }
    return nil
}

func (v *Validator) ValidateConcurrency(n int) error {
    if n < 1 {
        return fmt.Errorf("concurrency must be at least 1")
    }
    return nil
}

func (v *Validator) ValidateBatchSize(n int) error {
    if n < 1 {
        return fmt.Errorf("batch size must be at least 1")
    }
    return nil
}

func (v *Validator) ValidateIDE(ide string) error {
    valid := []string{types.IDECodex, types.IDEClaude, types.IDEDroid}
    for _, v := range valid {
        if ide == v {
            return nil
        }
    }
    return fmt.Errorf("invalid IDE: %s (must be one of: %v)", ide, valid)
}

func (v *Validator) ValidateReasoningEffort(effort string) error {
    valid := []string{"low", "medium", "high"}
    for _, v := range valid {
        if effort == v {
            return nil
        }
    }
    return fmt.Errorf("invalid reasoning effort: %s (must be one of: %v)", effort, valid)
}
```

### 4.2 Bubble Tea UI Components (ui/tea/)

**tea/model.go**:
```go
type UIModel struct {
    ctx       context.Context
    jobs      []models.Job
    results   map[int]models.JobResult
    sidebar   *SidebarComponent
    main      *MainComponent
    logs      *LogComponent
    width     int
    height    int
    ready     bool
}

func NewUIModel(ctx context.Context, jobs []models.Job) *UIModel {
    return &UIModel{
        ctx:     ctx,
        jobs:    jobs,
        results: make(map[int]models.JobResult),
        sidebar: NewSidebarComponent(),
        main:    NewMainComponent(),
        logs:    NewLogComponent(),
    }
}

func (m *UIModel) Init() tea.Cmd {
    return nil
}

func (m *UIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        m.ready = true
        return m, nil

    case tea.KeyMsg:
        switch msg.String() {
        case "ctrl+c", "q":
            return m, tea.Quit
        }

    case JobStartedMsg:
        return m.handleJobStarted(msg)

    case JobCompletedMsg:
        return m.handleJobCompleted(msg)

    case JobFailedMsg:
        return m.handleJobFailed(msg)

    case LogLineMsg:
        return m.handleLogLine(msg)
    }

    return m, nil
}

func (m *UIModel) View() string {
    if !m.ready {
        return "Initializing..."
    }

    // Layout: sidebar | main | logs
    sidebarView := m.sidebar.Render(m.jobs, m.results)
    mainView := m.main.Render(m.currentJob(), m.results)
    logsView := m.logs.Render(m.tailLines)

    return lipgloss.JoinHorizontal(
        lipgloss.Top,
        sidebarView,
        mainView,
        logsView,
    )
}

func (m *UIModel) handleJobStarted(msg JobStartedMsg) (tea.Model, tea.Cmd) {
    // Update job status
    return m, nil
}

func (m *UIModel) handleJobCompleted(msg JobCompletedMsg) (tea.Model, tea.Cmd) {
    m.results[msg.Result.Job.Index] = msg.Result
    return m, nil
}

func (m *UIModel) handleJobFailed(msg JobFailedMsg) (tea.Model, tea.Cmd) {
    m.results[msg.Job.Index] = models.JobResult{
        Job:     msg.Job,
        Success: false,
        Error:   msg.Error,
    }
    return m, nil
}

func (m *UIModel) handleLogLine(msg LogLineMsg) (tea.Model, tea.Cmd) {
    m.logs.AppendLine(msg.Line)
    return m, nil
}

func (m *UIModel) currentJob() *models.Job {
    // Return currently executing or next job
    for i, job := range m.jobs {
        if _, ok := m.results[i]; !ok {
            return &job
        }
    }
    return nil
}

// Message types
type JobStartedMsg struct {
    Job models.Job
}

type JobCompletedMsg struct {
    Result models.JobResult
}

type JobFailedMsg struct {
    Job   models.Job
    Error error
}

type LogLineMsg struct {
    Line string
}
```

**tea/sidebar.go**:
```go
type SidebarComponent struct {
    width int
}

func NewSidebarComponent() *SidebarComponent {
    return &SidebarComponent{
        width: 30,
    }
}

func (c *SidebarComponent) Render(jobs []models.Job, results map[int]models.JobResult) string {
    var sb strings.Builder

    sb.WriteString(styles.SidebarTitle.Render("Jobs"))
    sb.WriteString("\n\n")

    for _, job := range jobs {
        status := c.getJobStatus(job, results)
        icon := c.getStatusIcon(status)

        line := fmt.Sprintf("%s %s", icon, job.Name)
        if status == JobStatusRunning {
            line = styles.RunningJob.Render(line)
        } else if status == JobStatusCompleted {
            line = styles.CompletedJob.Render(line)
        } else if status == JobStatusFailed {
            line = styles.FailedJob.Render(line)
        }

        sb.WriteString(line)
        sb.WriteString("\n")
    }

    return styles.Sidebar.
        Width(c.width).
        Render(sb.String())
}

func (c *SidebarComponent) getJobStatus(job models.Job, results map[int]models.JobResult) models.JobStatus {
    result, ok := results[job.Index]
    if !ok {
        return models.JobStatusPending
    }
    if result.Success {
        return models.JobStatusCompleted
    }
    return models.JobStatusFailed
}

func (c *SidebarComponent) getStatusIcon(status models.JobStatus) string {
    switch status {
    case models.JobStatusPending:
        return "○"
    case models.JobStatusRunning:
        return "●"
    case models.JobStatusCompleted:
        return "✓"
    case models.JobStatusFailed:
        return "✗"
    default:
        return "?"
    }
}
```

**tea/main.go**:
```go
type MainComponent struct {
    width int
}

func NewMainComponent() *MainComponent {
    return &MainComponent{
        width: 60,
    }
}

func (c *MainComponent) Render(job *models.Job, results map[int]models.JobResult) string {
    if job == nil {
        return styles.Main.
            Width(c.width).
            Render("No active job")
    }

    var sb strings.Builder

    sb.WriteString(styles.JobTitle.Render(job.Name))
    sb.WriteString("\n\n")

    sb.WriteString(styles.Label.Render("Files:"))
    sb.WriteString("\n")
    for _, file := range job.Files {
        sb.WriteString(fmt.Sprintf("  • %s\n", file))
    }
    sb.WriteString("\n")

    sb.WriteString(styles.Label.Render("Issues:"))
    sb.WriteString(fmt.Sprintf(" %d\n\n", len(job.Issues)))

    if result, ok := results[job.Index]; ok {
        sb.WriteString(c.renderResult(result))
    } else {
        sb.WriteString(styles.Muted.Render("Running..."))
    }

    return styles.Main.
        Width(c.width).
        Render(sb.String())
}

func (c *MainComponent) renderResult(result models.JobResult) string {
    var sb strings.Builder

    if result.Success {
        sb.WriteString(styles.Success.Render("✓ Completed"))
    } else {
        sb.WriteString(styles.Error.Render("✗ Failed"))
        if result.Error != nil {
            sb.WriteString(fmt.Sprintf("\n%s", result.Error.Error()))
        }
    }

    sb.WriteString("\n\n")
    sb.WriteString(fmt.Sprintf("Duration: %s\n", result.Duration))
    sb.WriteString(fmt.Sprintf("Exit Code: %d\n", result.ExitCode))

    return sb.String()
}
```

**tea/logs.go**:
```go
type LogComponent struct {
    viewport viewport.Model
    lines    []string
}

func NewLogComponent() *LogComponent {
    vp := viewport.New(40, 20)
    return &LogComponent{
        viewport: vp,
        lines:    make([]string, 0),
    }
}

func (c *LogComponent) AppendLine(line string) {
    c.lines = append(c.lines, line)
    c.updateViewport()
}

func (c *LogComponent) Render(tailLines int) string {
    // Get tail lines
    displayLines := c.lines
    if tailLines > 0 && len(c.lines) > tailLines {
        displayLines = c.lines[len(c.lines)-tailLines:]
    }

    var sb strings.Builder
    for _, line := range displayLines {
        sb.WriteString(line)
        sb.WriteString("\n")
    }

    c.viewport.SetContent(sb.String())
    return styles.Logs.Render(c.viewport.View())
}

func (c *LogComponent) updateViewport() {
    c.viewport.GotoBottom()
}
```

### 4.3 UI Styling (ui/styles/)

**styles/theme.go**:
```go
var (
    // Colors
    primaryColor   = lipgloss.Color("86")
    secondaryColor = lipgloss.Color("39")
    successColor   = lipgloss.Color("42")
    errorColor     = lipgloss.Color("196")
    mutedColor     = lipgloss.Color("241")

    // Styles
    Sidebar = lipgloss.NewStyle().
        BorderStyle(lipgloss.NormalBorder()).
        BorderRight(true).
        BorderForeground(mutedColor).
        Padding(1, 2)

    Main = lipgloss.NewStyle().
        Padding(1, 2)

    Logs = lipgloss.NewStyle().
        BorderStyle(lipgloss.NormalBorder()).
        BorderLeft(true).
        BorderForeground(mutedColor).
        Padding(1, 2)

    SidebarTitle = lipgloss.NewStyle().
        Bold(true).
        Foreground(primaryColor)

    JobTitle = lipgloss.NewStyle().
        Bold(true).
        Foreground(primaryColor).
        Underline(true)

    Label = lipgloss.NewStyle().
        Bold(true)

    RunningJob = lipgloss.NewStyle().
        Foreground(secondaryColor)

    CompletedJob = lipgloss.NewStyle().
        Foreground(successColor)

    FailedJob = lipgloss.NewStyle().
        Foreground(errorColor)

    Success = lipgloss.NewStyle().
        Bold(true).
        Foreground(successColor)

    Error = lipgloss.NewStyle().
        Bold(true).
        Foreground(errorColor)

    Muted = lipgloss.NewStyle().
        Foreground(mutedColor).
        Italic(true)
)
```

### Relevant Files

**Files to Create**:
- `scripts/markdown/ui/forms/builder.go`
- `scripts/markdown/ui/forms/inputs.go`
- `scripts/markdown/ui/forms/validators.go`
- `scripts/markdown/ui/tea/model.go`
- `scripts/markdown/ui/tea/sidebar.go`
- `scripts/markdown/ui/tea/main.go`
- `scripts/markdown/ui/tea/logs.go`
- `scripts/markdown/ui/styles/theme.go`

**Test Files**:
- `scripts/markdown/ui/forms/builder_test.go`
- `scripts/markdown/ui/forms/inputs_test.go`
- `scripts/markdown/ui/forms/validators_test.go`
- `scripts/markdown/ui/tea/model_test.go`
- `scripts/markdown/ui/tea/sidebar_test.go`

### Dependent Files

**Dependencies from Task 1.0**:
- `scripts/markdown/core/models/*.go` - Domain models
- `scripts/markdown/shared/types/constants.go` - Constants

**Reference for extraction**:
- `scripts/markdown/check.go` - Source of UI code

## Deliverables

- [ ] Form builders fully implemented and tested
- [ ] Bubble Tea UI components complete
- [ ] UI styling and themes defined
- [ ] Event handling and state management functional
- [ ] No business logic in UI layer
- [ ] Unit tests for form builders and validators
- [ ] Integration tests for UI workflows
- [ ] All code passes `make lint`
- [ ] All tests pass

## Tests

### Unit Tests

- [ ] **Form Builder Tests**:
  - AddField adds fields correctly
  - Build creates form with all fields
  - Run executes form
  - Multiple groups support

- [ ] **Input Factory Tests**:
  - CreatePRInput validates correctly
  - CreateConcurrentInput validates range
  - CreateBatchSizeInput validates range
  - CreateIDESelectInput has correct options
  - CreateModelInput uses correct defaults
  - CreateReasoningEffortInput has correct options
  - CreateConfirmInput works correctly

- [ ] **Validator Tests**:
  - ValidatePR with valid/invalid inputs
  - ValidateConcurrency with valid/invalid inputs
  - ValidateBatchSize with valid/invalid inputs
  - ValidateIDE with valid/invalid inputs
  - ValidateReasoningEffort with valid/invalid inputs

- [ ] **UI Model Tests**:
  - Init returns correct command
  - Update handles WindowSizeMsg
  - Update handles KeyMsg (quit)
  - Update handles JobStartedMsg
  - Update handles JobCompletedMsg
  - Update handles JobFailedMsg
  - Update handles LogLineMsg
  - View renders without panic
  - View layout is correct

- [ ] **Sidebar Tests**:
  - Render shows all jobs
  - getJobStatus returns correct status
  - getStatusIcon returns correct icon

- [ ] **Main Component Tests**:
  - Render with no active job
  - Render with active job
  - Render with completed job
  - Render with failed job
  - renderResult formats correctly

- [ ] **Log Component Tests**:
  - AppendLine adds lines
  - Render shows tail lines
  - Render respects tailLines limit
  - updateViewport scrolls to bottom

### Integration Tests

- [ ] **Form Workflow Tests**:
  - Complete form submission flow
  - Form validation prevents invalid submission
  - Form applies values correctly

- [ ] **UI Event Flow Tests**:
  - Job lifecycle (started → completed)
  - Job lifecycle (started → failed)
  - Log streaming updates UI
  - Multiple jobs processed sequentially

## Success Criteria

### Functional Requirements
- [ ] Forms collect all required input
- [ ] Forms validate input correctly
- [ ] UI displays job progress accurately
- [ ] UI updates in real-time
- [ ] UI handles errors gracefully

### Architectural Requirements
- [ ] UI layer has no business logic
- [ ] UI components are modular and reusable
- [ ] UI state is properly managed
- [ ] Event handling is clean and testable
- [ ] Styles are centralized and consistent

### Quality Requirements
- [ ] All functions < 50 lines
- [ ] All code passes `make lint`
- [ ] All tests pass: `gotestsum --format pkgname -- -race -parallel=4 ./scripts/markdown/ui/...`
- [ ] Test coverage > 70% (UI code is harder to test)
- [ ] No rendering panics or crashes

### Integration Requirements
- [ ] Can be integrated with Task 3.0 use cases
- [ ] Can be wired in Task 5.0 (Application Wiring)
- [ ] UI notifications work with any notifier implementation

## Implementation Notes

### Order of Implementation
1. Form builders (inputs, validators, builder)
2. UI styles (theme)
3. UI components (model, sidebar, main, logs)
4. Unit tests for forms
5. Unit tests for UI components
6. Integration tests
7. Run `make fmt && make lint && make test`

### Key Design Decisions
- **No business logic**: UI only displays and collects data
- **Component-based**: Each UI concern is a separate component
- **Message-driven**: Bubble Tea message pattern for events
- **Testable**: UI logic separated from rendering
- **Styled centrally**: All styles in one place

### Common Pitfalls to Avoid
- ❌ Don't add business logic to UI components
- ❌ Don't hardcode dimensions (use dynamic sizing)
- ❌ Don't skip error handling in forms
- ❌ Don't create tight coupling between components
- ❌ Don't forget to handle edge cases (no jobs, empty results)

### Testing Strategy
- **Unit tests**: Test logic, not rendering
- **Mock messages**: Test message handlers independently
- **Validation tests**: Comprehensive validator coverage
- **Integration tests**: Full form and UI workflows

### Parallelization Notes
This task can be executed in parallel with:
- **Task 2.0 (Infrastructure)**: Independent concerns
- **Task 3.0 (Core Logic)**: Independent concerns

All three can proceed after Task 1.0 without blocking each other.

## Dependencies

**Blocks**: Task 5.0 (Application Wiring)

**Blocked By**: Task 1.0 (Foundation)

**Parallel With**: Task 2.0 (Infrastructure), Task 3.0 (Core Logic)

## Estimated Effort

**Size**: Medium (M)
**Duration**: 1-2 days
**Complexity**: Medium - UI code with state management and event handling
