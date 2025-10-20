# Technical Specification: Refactoring check.go for Clean Architecture

## Executive Summary

The current `scripts/markdown/check.go` file is a **3,028-line monolithic script** that violates multiple software engineering principles. This technical specification outlines a comprehensive refactoring strategy to break this file into a modular, maintainable architecture following SOLID principles, clean architecture patterns, and dependency inversion.

## Current Architecture Analysis

### File Statistics

- **Total Lines**: 3,028
- **Functions**: 158
- **LOC per Function**: Average ~19 lines (ranging from 1-200+ lines)
- **Cyclomatic Complexity**: High (multiple nested conditionals and state management)

### Current Responsibilities (Violating SRP)

| Responsibility     | Functions     | Lines      | Dependencies                  |
| ------------------ | ------------- | ---------- | ----------------------------- |
| **CLI Layer**      | 25+ functions | ~400 lines | Cobra, Huh Forms              |
| **UI Layer**       | 40+ functions | ~800 lines | Bubble Tea, Lipgloss          |
| **Business Logic** | 30+ functions | ~600 lines | Core domain logic             |
| **Infrastructure** | 20+ functions | ~400 lines | File I/O, Command execution   |
| **Domain Models**  | 15+ types     | ~200 lines | Pure domain entities          |
| **Utilities**      | 25+ functions | ~300 lines | Path manipulation, formatting |

### Current Issues

#### 1. Single Responsibility Principle Violation

- One file handles CLI parsing, UI rendering, business logic, file operations, and command execution
- Functions mix concerns (e.g., `runOneJob` handles UI updates, command execution, and error handling)

#### 2. Dependency Inversion Principle Violation

```go
// ❌ Direct dependency on external libraries
func createIDECommand(ctx context.Context, args *cliArgs) *exec.Cmd {
    // Direct exec.Cmd creation
    return exec.CommandContext(ctx, ideCodex, args...)
}
```

#### 3. Interface Segregation Principle Violation

- Massive UI model with 40+ methods handling all UI concerns
- No separation between different UI responsibilities (sidebar, main content, logs, etc.)

#### 4. Open/Closed Principle Violation

- Hard to extend without modifying existing code
- Adding new IDE tools requires changes across multiple functions

#### 5. Tight Coupling Issues

- UI components directly coupled to business logic
- Command execution logic mixed with logging and error handling
- No clear separation between data and presentation

## Proposed Architecture

### Clean Architecture Structure

```
scripts/markdown/
├── cmd/                    # Application Layer (CLI Entry Point)
│   └── check/
│       ├── main.go        # Main application entry point
│       ├── root.go        # Cobra command setup
│       └── config.go      # Configuration management
│
├── ui/                    # Presentation Layer (UI Components)
│   ├── tea/
│   │   ├── model.go       # Main UI model (orchestration only)
│   │   ├── sidebar.go     # Sidebar component
│   │   ├── main.go        # Main content area
│   │   └── logs.go        # Log viewport component
│   ├── forms/
│   │   ├── builder.go     # Form builder abstraction
│   │   ├── inputs.go      # Form input definitions
│   │   └── validators.go  # Input validation logic
│   └── styles/
│       └── theme.go       # UI styling and themes
│
├── core/                  # Domain Layer (Business Logic)
│   ├── models/
│   │   ├── issue.go       # Issue domain entities
│   │   ├── job.go         # Job domain entities
│   │   └── config.go      # Domain configuration
│   ├── services/
│   │   ├── processor.go   # Issue processing service
│   │   ├── executor.go    # Job execution service
│   │   └── summarizer.go  # Summary generation service
│   └── usecases/
│       ├── solve_issues.go # Main use case orchestrator
│       └── prepare_batch.go # Batch preparation use case
│
├── infrastructure/        # Infrastructure Layer (External Dependencies)
│   ├── filesystem/
│   │   ├── reader.go      # File reading abstraction
│   │   ├── writer.go      # File writing abstraction
│   │   └── scanner.go     # Directory scanning
│   ├── execution/
│   │   ├── runner.go      # Command execution abstraction
│   │   ├── codex.go       # Codex tool integration
│   │   ├── claude.go      # Claude tool integration
│   │   └── droid.go       # Droid tool integration
│   ├── logging/
│   │   ├── formatter.go   # Log formatting
│   │   ├── tap.go         # Log stream tapping
│   │   └── usage.go       # Token usage tracking
│   └── prompts/
│       ├── builder.go     # Prompt building abstraction
│       ├── templates.go   # Prompt templates
│       └── formatter.go   # Prompt formatting
│
└── shared/                # Shared Kernel (Common Utilities)
    ├── errors/
    ├── types/
    └── utils/
```

### Layer Responsibilities

#### 1. Domain Layer (`core/`)

- **Pure business logic** with no external dependencies
- Domain entities: `Issue`, `Job`, `Config`, `TokenUsage`
- Business rules and use cases
- Interface definitions for external dependencies

#### 2. Application Layer (`cmd/`)

- **CLI orchestration** and configuration
- Dependency injection setup
- Application-specific logic
- Coordinates between layers

#### 3. Infrastructure Layer (`infrastructure/`)

- **External service implementations**
- File I/O, command execution, HTTP clients
- Data persistence adapters
- Implements domain interfaces

#### 4. Presentation Layer (`ui/`)

- **User interface concerns only**
- Input collection and display
- No business logic
- Uses infrastructure services through interfaces

### Dependency Flow (Following DIP)

```
cmd/ → core/ → infrastructure/
  ↑       ↓        ↑
  └───────┴────────┘
UI Components ← Interfaces ← Implementations
```

## Detailed Refactoring Plan

### Phase 1: Domain Layer Extraction

#### 1.1 Domain Models (`core/models/`)

**Extract from current file:**

- `issueEntry` → `core/models/issue.go`
- `job` → `core/models/job.go`
- `cliArgs` → `core/models/config.go`
- `TokenUsage` → `core/models/token_usage.go`
- `solvePreparation` → `core/models/preparation.go`

**Add interfaces:**

```go
// core/ports/
type IssueProcessor interface {
    ProcessIssues(ctx context.Context, issues []Issue) ([]Job, error)
}

type JobExecutor interface {
    ExecuteJob(ctx context.Context, job Job) (JobResult, error)
}

type FileSystem interface {
    ReadFile(path string) ([]byte, error)
    WriteFile(path string, data []byte) error
    ListDir(path string) ([]string, error)
}
```

#### 1.2 Business Services (`core/services/`)

**Extract business logic:**

- Issue processing logic → `processor.go`
- Job execution orchestration → `executor.go`
- Summary generation → `summarizer.go`

### Phase 2: Infrastructure Layer

#### 2.1 File System Abstraction (`infrastructure/filesystem/`)

```go
// filesystem/reader.go
type Reader struct {
    // Implementation
}

func (r *Reader) ReadFile(path string) ([]byte, error) {
    return os.ReadFile(path)
}

// filesystem/writer.go
type Writer struct {
    // Implementation
}

func (w *Writer) WriteFile(path string, data []byte) error {
    return os.WriteFile(path, data, 0644)
}
```

#### 2.2 Command Execution (`infrastructure/execution/`)

```go
// execution/runner.go
type Runner interface {
    Execute(ctx context.Context, cmd Command) (Result, error)
}

type Command struct {
    Tool    string
    Args    []string
    Stdin   io.Reader
    WorkDir string
}

// execution/codex.go
type CodexRunner struct {
    executor Runner
}

func (c *CodexRunner) RunCodex(ctx context.Context, prompt string) error {
    cmd := Command{
        Tool:  "codex",
        Args:  c.buildCodexArgs(),
        Stdin: strings.NewReader(prompt),
    }
    return c.executor.Execute(ctx, cmd)
}
```

#### 2.3 Logging Infrastructure (`infrastructure/logging/`)

```go
// logging/formatter.go
type Formatter interface {
    Format(data []byte) ([]byte, error)
}

// logging/tap.go
type Tap interface {
    Write(p []byte) (n int, err error)
}
```

### Phase 3: Presentation Layer

#### 3.1 Form Builder (`ui/forms/`)

```go
// forms/builder.go
type FormBuilder interface {
    AddField(field Field) error
    Build() (*huh.Form, error)
}

// forms/inputs.go
type InputFactory interface {
    CreatePRInput() huh.Field
    CreateIDEInput() huh.Field
    CreateNumericInput(title, desc string, min, max int) huh.Field
}
```

#### 3.2 UI Components (`ui/tea/`)

```go
// tea/model.go - Main UI orchestration only
type UIModel struct {
    sidebar  SidebarComponent
    main     MainComponent
    logs     LogComponent
    // No direct business logic
}

// tea/sidebar.go
type SidebarComponent interface {
    Render(jobs []Job) string
    HandleInput(msg tea.Msg) (tea.Cmd, error)
}

// tea/main.go
type MainComponent interface {
    Render(job Job) string
    UpdateContent(job Job) error
}
```

### Phase 4: Application Layer

#### 4.1 Dependency Injection (`cmd/check/`)

```go
// cmd/check/main.go
func main() {
    // Setup dependency injection
    fs := filesystem.NewOSReaderWriter()
    executor := execution.NewCommandExecutor()
    processor := core.NewIssueProcessor(fs)
    ui := ui.NewTeaUI()

    app := NewApplication(processor, executor, ui)
    if err := app.Run(os.Args); err != nil {
        log.Fatal(err)
    }
}
```

## SOLID Principles Implementation

### 1. Single Responsibility Principle

**Before:**

```go
// ❌ Multiple responsibilities
func runOneJob(ctx context.Context, args *cliArgs, index int, j *job, ...) {
    // UI notification, command execution, error handling, logging
}
```

**After:**

```go
// ✅ Single responsibility
func (e *JobExecutor) Execute(ctx context.Context, job Job) error {
    return e.runner.Execute(ctx, e.buildCommand(job))
}

func (u *UINotifier) NotifyJobStart(job Job) {
    u.channel <- JobStartedEvent{Job: job}
}
```

### 2. Open/Closed Principle

**Before:**

```go
// ❌ Must modify existing code to add new IDE
func createIDECommand(ctx context.Context, args *cliArgs) *exec.Cmd {
    switch args.ide {
    case ideCodex:
        return codexCommand(ctx, args.model, args.reasoningEffort)
    case ideClaude:
        return claudeCommand(ctx, args.model, args.reasoningEffort)
    // Must add new case here
    }
}
```

**After:**

```go
// ✅ Open for extension, closed for modification
type IDETool interface {
    BuildCommand(ctx context.Context, config ToolConfig) (*exec.Cmd, error)
}

func (f *IDEToolFactory) Create(tool string) IDETool {
    switch tool {
    case "codex":
        return &CodexTool{config: f.config}
    case "claude":
        return &ClaudeTool{config: f.config}
    // Easy to add new tools
    }
}
```

### 3. Liskov Substitution Principle

**Before:**

```go
// ❌ Different command builders with different interfaces
func codexCommand(ctx context.Context, model, reasoning string) *exec.Cmd
func claudeCommand(ctx context.Context, model, reasoning string) *exec.Cmd
```

**After:**

```go
// ✅ Consistent interface
type CommandBuilder interface {
    Build(ctx context.Context, config ToolConfig) (*exec.Cmd, error)
}

func (c *CodexBuilder) Build(ctx context.Context, config ToolConfig) (*exec.Cmd, error)
func (c *ClaudeBuilder) Build(ctx context.Context, config ToolConfig) (*exec.Cmd, error)
```

### 4. Interface Segregation Principle

**Before:**

```go
// ❌ Fat interface with many responsibilities
type UIModel struct {
    // 40+ methods for all UI concerns
}
```

**After:**

```go
// ✅ Segregated interfaces
type SidebarRenderer interface {
    Render(jobs []Job) string
}

type MainContentRenderer interface {
    Render(job Job) string
}

type LogViewer interface {
    UpdateContent(lines []string) error
}
```

### 5. Dependency Inversion Principle

**Before:**

```go
// ❌ High-level module depends on low-level module
func (ui *UIModel) handleJobFinished(v jobFinishedMsg) {
    // Direct file operations
    os.WriteFile(logPath, data, 0644)
}
```

**After:**

```go
// ✅ Depend on abstraction
type UIModel struct {
    fileWriter FileWriter // Interface
}

func (ui *UIModel) handleJobFinished(v jobFinishedMsg) {
    return ui.fileWriter.WriteFile(logPath, data) // Interface
}
```

## Implementation Strategy

### Phase 1: Foundation (Week 1-2)

1. Create directory structure
2. Extract domain models and interfaces
3. Set up basic dependency injection
4. Create infrastructure abstractions

### Phase 2: Business Logic (Week 3-4)

1. Extract business services
2. Implement use cases
3. Add comprehensive tests
4. Ensure all business logic is testable

### Phase 3: Infrastructure (Week 5-6)

1. Implement file system abstractions
2. Create command execution layer
3. Add logging infrastructure
4. Implement prompt building

### Phase 4: UI Layer (Week 7-8)

1. Extract form builder abstractions
2. Create UI components
3. Implement presentation layer
4. Add UI tests

### Phase 5: Integration & Testing (Week 9-10)

1. Wire everything together
2. Comprehensive integration tests
3. Performance testing
4. Documentation updates

## Quality Assurance

### Testing Strategy

- **Unit Tests**: Each component thoroughly tested in isolation
- **Integration Tests**: Test layer interactions
- **E2E Tests**: Full workflow validation
- **Performance Tests**: Ensure no regression in execution speed

### Metrics to Track

- **Cyclomatic Complexity**: Target < 10 per function
- **Function Length**: Target < 50 lines per function
- **Test Coverage**: Target > 80%
- **Dependency Count**: Minimize cross-layer dependencies

### Code Quality Gates

- All existing functionality preserved
- Performance characteristics maintained
- Memory usage not significantly increased
- Build time not significantly increased

## Migration Strategy

### Backward Compatibility

- Maintain current CLI interface during refactoring
- Use feature flags for gradual migration
- Keep original file as reference during development

### Risk Mitigation

- Incremental refactoring with continuous testing
- Feature branches for each architectural layer
- Rollback plan for each phase

## Benefits of New Architecture

### 1. Maintainability

- **Single Responsibility**: Each module has one clear purpose
- **Easy Testing**: Isolated components are simple to test
- **Clear Dependencies**: Explicit interface contracts

### 2. Extensibility

- **New IDE Tools**: Add without modifying existing code
- **New UI Components**: Plugin architecture for UI elements
- **New Output Formats**: Easy to add new export formats

### 3. Testability

- **Dependency Injection**: Easy to mock external dependencies
- **Isolated Testing**: Test business logic without UI
- **Fast Tests**: Unit tests run quickly without external dependencies

### 4. Performance

- **Lazy Loading**: Load only needed components
- **Efficient Dependencies**: Minimal import overhead
- **Better Caching**: Clear separation allows better caching strategies

## Conclusion

This refactoring transforms a **3,028-line monolith** into a **maintainable, extensible, and testable codebase** following industry best practices. The new architecture ensures:

- **SOLID compliance** across all components
- **Clean separation of concerns** between layers
- **Dependency inversion** for better testability
- **Open/closed design** for easy extensibility
- **Comprehensive test coverage** for reliability

The proposed architecture provides a solid foundation for future enhancements while maintaining all existing functionality and improving code quality significantly.

## References

- [Clean Architecture by Robert C. Martin](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html)
- [SOLID Principles](https://en.wikipedia.org/wiki/SOLID)
- [Dependency Inversion Principle](https://en.wikipedia.org/wiki/Dependency_inversion_principle)
- [Hexagonal Architecture](https://herbertograca.com/2017/11/16/explicit-architecture-01-ddd-hexagonal-onion-clean-cqrs-how-i-put-it-all-together/)
