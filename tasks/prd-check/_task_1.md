## markdown

## status: pending

<task_context>
<domain>scripts/markdown</domain>
<type>implementation</type>
<scope>foundation_architecture</scope>
<complexity>medium</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 1.0: Foundation Setup - Create Directory Structure and Domain Models

## Overview

Establish the foundational clean architecture structure for the refactored check.go implementation. This task creates the directory hierarchy, extracts pure domain models from the monolithic file, and defines core interfaces that will guide the rest of the refactoring effort.

This is the critical first task that unblocks all parallel development tracks (infrastructure, core logic, and UI).

<critical>**MUST READ BEFORE STARTING** @.cursor/rules/go-coding-standards.mdc and @.cursor/rules/architecture.mdc</critical>

<requirements>
- All domain models must be pure Go structs with no external dependencies
- Interfaces must be small and focused (ISP compliance)
- No business logic in models (SRP compliance)
- All functions < 50 lines
- Follow Go 1.25.2 patterns
- Use context-first APIs
- No global state or singletons
</requirements>

## Subtasks

- [ ] 1.1 Create complete directory structure for clean architecture layers
- [ ] 1.2 Extract domain models from check.go (Issue, Job, Config, TokenUsage, Preparation)
- [ ] 1.3 Define core interfaces (ports) for external dependencies
- [ ] 1.4 Create shared utilities package structure
- [ ] 1.5 Set up basic package documentation
- [ ] 1.6 Write unit tests for domain model validation logic

## Implementation Details

### 1.1 Directory Structure

Create the following structure under `scripts/markdown/`:

```
scripts/markdown/
├── cmd/
│   └── check/
│       ├── main.go           # Application entry point
│       ├── root.go           # Cobra command setup
│       └── di.go             # Dependency injection container
├── ui/
│   ├── tea/                  # Bubble Tea UI components
│   ├── forms/                # Huh form builders
│   └── styles/               # Lipgloss styling
├── core/
│   ├── models/               # Domain entities
│   ├── services/             # Business services
│   ├── usecases/             # Use case orchestrators
│   └── ports/                # Interface definitions
├── infrastructure/
│   ├── filesystem/           # File I/O abstractions
│   ├── execution/            # Command execution
│   ├── logging/              # Log formatting and tapping
│   └── prompts/              # Prompt building
└── shared/
    ├── errors/               # Error utilities
    ├── types/                # Common types
    └── utils/                # Helper functions
```

### 1.2 Domain Models to Extract

From the current `check.go`, extract and refactor these types:

**core/models/issue.go**:

```go
type Issue struct {
    Name     string // Filename of the issue markdown
    AbsPath  string // Absolute path to issue file
    Content  string // Full markdown content
    CodeFile string // Repository-relative code file path
}
```

**core/models/job.go**:

```go
type Job struct {
    Index      int      // Job index in batch
    Name       string   // Human-readable job name
    Files      []string // Code files included in this job
    PromptPath string   // Path to generated prompt file
    LogPath    string   // Path to log output
    OutputPath string   // Path to JSON output
    Issues     []Issue  // Issues included in this job
}

type JobResult struct {
    Job          Job
    Success      bool
    Error        error
    TokensUsed   TokenUsage
    Duration     time.Duration
    ExitCode     int
}

type JobStatus int

const (
    JobStatusPending JobStatus = iota
    JobStatusRunning
    JobStatusCompleted
    JobStatusFailed
)
```

**core/models/config.go**:

```go
type Config struct {
    PR              string
    IssuesDir       string
    DryRun          bool
    Concurrent      int
    BatchSize       int
    IDE             string
    Model           string
    Grouped         bool
    TailLines       int
    ReasoningEffort string
}

func (c *Config) Validate() error {
    // Validation logic extracted from cliArgs.validate()
}
```

**core/models/token_usage.go**:

```go
type TokenUsage struct {
    InputTokens     int
    OutputTokens    int
    ThinkingTokens  int
    CacheReadTokens int
    TotalTokens     int
}
```

**core/models/preparation.go**:

```go
type Preparation struct {
    ResolvedIssuesDir string
    PromptRoot        string
    AllIssues         []Issue
    Jobs              []Job
}
```

### 1.3 Core Interfaces (Ports)

**core/ports/filesystem.go**:

```go
type FileReader interface {
    ReadFile(ctx context.Context, path string) ([]byte, error)
    ListDir(ctx context.Context, path string) ([]string, error)
    FileExists(ctx context.Context, path string) (bool, error)
}

type FileWriter interface {
    WriteFile(ctx context.Context, path string, data []byte, perm os.FileMode) error
    MkdirAll(ctx context.Context, path string, perm os.FileMode) error
}

type FileSystem interface {
    FileReader
    FileWriter
}
```

**core/ports/executor.go**:

```go
type CommandExecutor interface {
    Execute(ctx context.Context, cmd *Command) (*CommandResult, error)
    IsAvailable(ctx context.Context, toolName string) (bool, error)
}

type Command struct {
    Tool    string
    Args    []string
    Stdin   io.Reader
    Stdout  io.Writer
    Stderr  io.Writer
    WorkDir string
    Env     []string
}

type CommandResult struct {
    ExitCode int
    Duration time.Duration
    Error    error
}
```

**core/ports/logger.go**:

```go
type LogFormatter interface {
    Format(ctx context.Context, data []byte) ([]byte, error)
}

type LogTap interface {
    Write(p []byte) (n int, err error)
    Lines() []string
    TailLines(n int) []string
}
```

**core/ports/ui.go**:

```go
type UINotifier interface {
    NotifyJobStarted(ctx context.Context, job Job) error
    NotifyJobProgress(ctx context.Context, job Job, progress float64) error
    NotifyJobCompleted(ctx context.Context, result JobResult) error
    NotifyJobFailed(ctx context.Context, job Job, err error) error
}
```

### 1.4 Shared Utilities

**shared/errors/errors.go**:

```go
// Domain-specific error types
var (
    ErrInvalidConfig = errors.New("invalid configuration")
    ErrInvalidIssue  = errors.New("invalid issue format")
    ErrJobFailed     = errors.New("job execution failed")
)

// Error wrapping utilities following project standards
func WrapError(err error, msg string) error {
    return fmt.Errorf("%s: %w", msg, err)
}
```

**shared/types/constants.go**:

```go
// Extract constants from check.go
const (
    UnknownFileName            = "unknown"
    IDECodex                   = "codex"
    IDEClaude                  = "claude"
    IDEDroid                   = "droid"
    DefaultCodexModel          = "gpt-5-codex"
    DefaultClaudeModel         = "sonnet[1m]"
    ThinkPromptMedium          = "Think hard through problems..."
    ThinkPromptLow             = "Think concisely and act quickly..."
    ThinkPromptHighDescription = "Ultrathink deeply..."
)
```

**shared/utils/path.go**:

```go
// Path manipulation utilities
func SafeFileName(name string) string {
    // Extract from check.go
}

func ResolveAbsPath(ctx context.Context, path string) (string, error) {
    // Extract from check.go
}
```

### Relevant Files

**Files to Create**:

- `scripts/markdown/cmd/check/main.go`
- `scripts/markdown/cmd/check/root.go`
- `scripts/markdown/cmd/check/di.go`
- `scripts/markdown/core/models/issue.go`
- `scripts/markdown/core/models/job.go`
- `scripts/markdown/core/models/config.go`
- `scripts/markdown/core/models/token_usage.go`
- `scripts/markdown/core/models/preparation.go`
- `scripts/markdown/core/ports/filesystem.go`
- `scripts/markdown/core/ports/executor.go`
- `scripts/markdown/core/ports/logger.go`
- `scripts/markdown/core/ports/ui.go`
- `scripts/markdown/shared/errors/errors.go`
- `scripts/markdown/shared/types/constants.go`
- `scripts/markdown/shared/utils/path.go`

**Empty directory structure** (create placeholder files or package docs):

- `scripts/markdown/ui/tea/doc.go`
- `scripts/markdown/ui/forms/doc.go`
- `scripts/markdown/ui/styles/doc.go`
- `scripts/markdown/core/services/doc.go`
- `scripts/markdown/core/usecases/doc.go`
- `scripts/markdown/infrastructure/filesystem/doc.go`
- `scripts/markdown/infrastructure/execution/doc.go`
- `scripts/markdown/infrastructure/logging/doc.go`
- `scripts/markdown/infrastructure/prompts/doc.go`

### Dependent Files

**Reference for extraction**:

- `scripts/markdown/check.go` - Source of domain models and constants

**Project standards**:

- `.cursor/rules/go-coding-standards.mdc`
- `.cursor/rules/architecture.mdc`

## Deliverables

- [ ] Complete directory structure created with all package folders
- [ ] All domain model files created and populated:
  - `core/models/issue.go`
  - `core/models/job.go`
  - `core/models/config.go`
  - `core/models/token_usage.go`
  - `core/models/preparation.go`
- [ ] All port interface files created:
  - `core/ports/filesystem.go`
  - `core/ports/executor.go`
  - `core/ports/logger.go`
  - `core/ports/ui.go`
- [ ] Shared utilities populated:
  - `shared/errors/errors.go`
  - `shared/types/constants.go`
  - `shared/utils/path.go`
- [ ] Package documentation files (`doc.go`) for all empty packages
- [ ] Stub `cmd/check/main.go` with basic structure (will be completed in Task 5.0)
- [ ] All code passes `make lint`
- [ ] All tests pass `make test`

## Tests

Unit tests for domain models and utilities:

- [ ] **Test: Config.Validate()** - Test configuration validation rules
  - Valid config with all required fields
  - Invalid PR number (empty)
  - Invalid concurrent value (< 1)
  - Invalid batch size (< 1)
  - Invalid IDE value (not codex/claude/droid)
  - Invalid reasoning effort (not low/medium/high)

- [ ] **Test: Issue Model** - Test issue construction and validation
  - Create issue with valid data
  - Issue with invalid CodeFile format
  - Issue with empty required fields

- [ ] **Test: Job Model** - Test job creation and status transitions
  - Create job with valid data
  - Job with empty files list
  - Job status transitions (pending → running → completed/failed)

- [ ] **Test: TokenUsage** - Test token usage calculations
  - Calculate total tokens
  - Add token usage from multiple results
  - Handle zero values

- [ ] **Test: Path utilities (shared/utils/path.go)**
  - SafeFileName with special characters
  - SafeFileName with Unicode characters
  - ResolveAbsPath with relative paths
  - ResolveAbsPath with already absolute paths
  - ResolveAbsPath with tilde expansion

- [ ] **Test: Error wrapping (shared/errors/errors.go)**
  - WrapError maintains error chain
  - WrapError with nil error
  - Domain error types are distinct

## Success Criteria

### Functional Requirements

- [ ] All domain models compile without errors
- [ ] All interfaces are properly defined and documented
- [ ] Constants are extracted and accessible
- [ ] Path utilities work correctly
- [ ] Config validation logic is comprehensive

### Architectural Requirements

- [ ] Domain models have no external dependencies (pure Go)
- [ ] Interfaces follow Interface Segregation Principle (small, focused)
- [ ] Directory structure matches clean architecture pattern
- [ ] All packages have proper documentation
- [ ] No circular dependencies between packages

### Quality Requirements

- [ ] All functions < 50 lines
- [ ] All code passes `make lint`
- [ ] All tests pass with `gotestsum --format pkgname -- -race -parallel=4 ./scripts/markdown/core/... ./scripts/markdown/shared/...`
- [ ] Test coverage > 80% for domain models
- [ ] All public types and functions have doc comments

### Unblocking Criteria

- [ ] Other developers can start Task 2.0 (Infrastructure)
- [ ] Other developers can start Task 3.0 (Core Logic)
- [ ] Other developers can start Task 4.0 (UI Layer)

## Implementation Notes

### Order of Implementation

1. Create directory structure first (all folders)
2. Create shared utilities (no dependencies)
3. Create domain models (depend only on shared)
4. Create port interfaces (depend on domain models)
5. Create package docs for empty packages
6. Write unit tests
7. Run `make fmt && make lint && make test`

### Key Design Decisions

- **No business logic in models**: Models are pure data structures
- **Context-first APIs**: All methods that perform I/O accept `context.Context` as first parameter
- **Interface-based design**: Define interfaces in core/ports, implementations come later
- **Shared utilities are minimal**: Only truly shared code goes in shared/

### Common Pitfalls to Avoid

- ❌ Don't add business logic to domain models
- ❌ Don't create circular dependencies between packages
- ❌ Don't use global state or singletons
- ❌ Don't create interfaces with too many methods (ISP violation)
- ❌ Don't hardcode paths or configuration values

### Testing Strategy for This Task

- Focus on unit tests for validation logic
- Test edge cases for path utilities
- Ensure error wrapping maintains error chains
- Mock-free testing (pure functions, no external dependencies)
- Use `t.Context()` instead of `context.Background()` in tests

## Dependencies

**Blocks**: Tasks 2.0, 3.0, 4.0, 5.0, 6.0

**Blocked By**: None (this is the starting task)

**Parallel Potential**: None (must complete before other tasks can start)

## Estimated Effort

**Size**: Medium (M)
**Duration**: 1-2 days
**Complexity**: Medium - Requires careful extraction and organization, but no complex logic
