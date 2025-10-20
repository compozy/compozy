## markdown

## status: pending

<task_context>
<domain>scripts/markdown/infrastructure</domain>
<type>implementation</type>
<scope>infrastructure_abstractions</scope>
<complexity>medium</complexity>
<dependencies>task_1.0</dependencies>
</task_context>

# Task 2.0: Infrastructure Layer - File System and Command Execution Abstractions

## Overview

Implement the infrastructure layer that provides concrete implementations for external dependencies. This includes file system operations, command execution (for IDE tools like codex/claude/droid), logging infrastructure, and prompt building.

This task can be executed in parallel with Tasks 3.0 (Core Logic) and 4.0 (UI Layer) after Task 1.0 is complete.

<critical>**MUST READ BEFORE STARTING** @.cursor/rules/go-coding-standards.mdc and @.cursor/rules/architecture.mdc</critical>

<requirements>
- Implement all port interfaces defined in Task 1.0
- Use dependency inversion (depend on abstractions)
- All functions < 50 lines
- Context-first APIs for all I/O operations
- Use `logger.FromContext(ctx)` for logging
- Use `config.FromContext(ctx)` for configuration
- No `context.Background()` in runtime code
- Proper error wrapping with fmt.Errorf
- Resource cleanup with defer
</requirements>

## Subtasks

- [ ] 2.1 Implement file system abstractions (filesystem/)
- [ ] 2.2 Implement command execution layer (execution/)
- [ ] 2.3 Implement logging infrastructure (logging/)
- [ ] 2.4 Implement prompt building system (prompts/)
- [ ] 2.5 Write comprehensive unit tests with mocks
- [ ] 2.6 Write integration tests for command execution

## Implementation Details

### 2.1 File System Abstractions (infrastructure/filesystem/)

**filesystem/reader.go**:

```go
type OSReader struct{}

func NewOSReader() *OSReader {
    return &OSReader{}
}

func (r *OSReader) ReadFile(ctx context.Context, path string) ([]byte, error) {
    logger := logger.FromContext(ctx)
    logger.Debug("reading file", "path", path)

    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("failed to read file %s: %w", path, err)
    }
    return data, nil
}

func (r *OSReader) ListDir(ctx context.Context, path string) ([]string, error) {
    // Implementation
}

func (r *OSReader) FileExists(ctx context.Context, path string) (bool, error) {
    // Implementation
}
```

**filesystem/writer.go**:

```go
type OSWriter struct{}

func NewOSWriter() *OSWriter {
    return &OSWriter{}
}

func (w *OSWriter) WriteFile(ctx context.Context, path string, data []byte, perm os.FileMode) error {
    logger := logger.FromContext(ctx)
    logger.Debug("writing file", "path", path, "size", len(data))

    if err := os.WriteFile(path, data, perm); err != nil {
        return fmt.Errorf("failed to write file %s: %w", path, err)
    }
    return nil
}

func (w *OSWriter) MkdirAll(ctx context.Context, path string, perm os.FileMode) error {
    // Implementation
}
```

**filesystem/filesystem.go**:

```go
type OSFileSystem struct {
    reader *OSReader
    writer *OSWriter
}

func NewOSFileSystem() *OSFileSystem {
    return &OSFileSystem{
        reader: NewOSReader(),
        writer: NewOSWriter(),
    }
}

// Implement FileReader interface
func (fs *OSFileSystem) ReadFile(ctx context.Context, path string) ([]byte, error) {
    return fs.reader.ReadFile(ctx, path)
}

// Implement FileWriter interface
func (fs *OSFileSystem) WriteFile(ctx context.Context, path string, data []byte, perm os.FileMode) error {
    return fs.writer.WriteFile(ctx, path, data, perm)
}

// ... additional methods
```

**filesystem/scanner.go**:

```go
type DirectoryScanner struct {
    fs ports.FileSystem
}

func NewDirectoryScanner(fs ports.FileSystem) *DirectoryScanner {
    return &DirectoryScanner{fs: fs}
}

func (s *DirectoryScanner) ScanIssues(ctx context.Context, dir string) ([]models.Issue, error) {
    // Extract issue scanning logic from check.go
    // Use s.fs for file operations
}
```

### 2.2 Command Execution Layer (infrastructure/execution/)

**execution/runner.go**:

```go
type OSCommandExecutor struct{}

func NewOSCommandExecutor() *OSCommandExecutor {
    return &OSCommandExecutor{}
}

func (e *OSCommandExecutor) Execute(ctx context.Context, cmd *ports.Command) (*ports.CommandResult, error) {
    logger := logger.FromContext(ctx)
    logger.Debug("executing command", "tool", cmd.Tool, "args", cmd.Args)

    startTime := time.Now()
    execCmd := exec.CommandContext(ctx, cmd.Tool, cmd.Args...)

    if cmd.Stdin != nil {
        execCmd.Stdin = cmd.Stdin
    }
    if cmd.Stdout != nil {
        execCmd.Stdout = cmd.Stdout
    }
    if cmd.Stderr != nil {
        execCmd.Stderr = cmd.Stderr
    }
    if cmd.WorkDir != "" {
        execCmd.Dir = cmd.WorkDir
    }
    if len(cmd.Env) > 0 {
        execCmd.Env = append(os.Environ(), cmd.Env...)
    }

    err := execCmd.Run()
    duration := time.Since(startTime)

    result := &ports.CommandResult{
        Duration: duration,
        Error:    err,
    }

    if err != nil {
        var exitErr *exec.ExitError
        if errors.As(err, &exitErr) {
            result.ExitCode = exitErr.ExitCode()
        } else {
            result.ExitCode = -1
        }
        return result, fmt.Errorf("command execution failed: %w", err)
    }

    result.ExitCode = 0
    return result, nil
}

func (e *OSCommandExecutor) IsAvailable(ctx context.Context, toolName string) (bool, error) {
    _, err := exec.LookPath(toolName)
    return err == nil, nil
}
```

**execution/ide_builder.go**:

```go
// Factory for IDE command builders
type IDECommandBuilder interface {
    BuildCommand(ctx context.Context, config models.Config, promptPath string) (*ports.Command, error)
}

type CodexCommandBuilder struct{}

func NewCodexCommandBuilder() *CodexCommandBuilder {
    return &CodexCommandBuilder{}
}

func (b *CodexCommandBuilder) BuildCommand(ctx context.Context, config models.Config, promptPath string) (*ports.Command, error) {
    // Extract codex command building logic from check.go
    args := []string{
        "--pr", config.PR,
        "--model", config.Model,
    }

    // Add reasoning effort
    switch config.ReasoningEffort {
    case "low":
        args = append(args, "--thinking", types.ThinkPromptLow)
    case "high":
        args = append(args, "--thinking", types.ThinkPromptHighDescription)
    default:
        args = append(args, "--thinking", types.ThinkPromptMedium)
    }

    // Open prompt file for stdin
    promptFile, err := os.Open(promptPath)
    if err != nil {
        return nil, fmt.Errorf("failed to open prompt file: %w", err)
    }

    return &ports.Command{
        Tool:  types.IDECodex,
        Args:  args,
        Stdin: promptFile,
    }, nil
}

type ClaudeCommandBuilder struct{}
type DroidCommandBuilder struct{}

// Similar implementations for Claude and Droid
```

**execution/factory.go**:

```go
type IDEToolFactory struct {
    executor ports.CommandExecutor
}

func NewIDEToolFactory(executor ports.CommandExecutor) *IDEToolFactory {
    return &IDEToolFactory{executor: executor}
}

func (f *IDEToolFactory) CreateBuilder(ide string) (IDECommandBuilder, error) {
    switch ide {
    case types.IDECodex:
        return NewCodexCommandBuilder(), nil
    case types.IDEClaude:
        return NewClaudeCommandBuilder(), nil
    case types.IDEDroid:
        return NewDroidCommandBuilder(), nil
    default:
        return nil, fmt.Errorf("unsupported IDE tool: %s", ide)
    }
}
```

### 2.3 Logging Infrastructure (infrastructure/logging/)

**logging/tap.go**:

```go
type LogTap struct {
    mu     sync.Mutex
    lines  []string
    writer io.Writer
}

func NewLogTap(writer io.Writer) *LogTap {
    return &LogTap{
        lines:  make([]string, 0),
        writer: writer,
    }
}

func (t *LogTap) Write(p []byte) (n int, err error) {
    t.mu.Lock()
    defer t.mu.Unlock()

    // Write to underlying writer
    if t.writer != nil {
        n, err = t.writer.Write(p)
        if err != nil {
            return n, err
        }
    }

    // Store lines
    lines := bytes.Split(p, []byte("\n"))
    for _, line := range lines {
        if len(line) > 0 {
            t.lines = append(t.lines, string(line))
        }
    }

    return len(p), nil
}

func (t *LogTap) Lines() []string {
    t.mu.Lock()
    defer t.mu.Unlock()
    return append([]string(nil), t.lines...)
}

func (t *LogTap) TailLines(n int) []string {
    t.mu.Lock()
    defer t.mu.Unlock()

    if n >= len(t.lines) {
        return append([]string(nil), t.lines...)
    }
    return append([]string(nil), t.lines[len(t.lines)-n:]...)
}
```

**logging/formatter.go**:

```go
type JSONLogFormatter struct{}

func NewJSONLogFormatter() *JSONLogFormatter {
    return &JSONLogFormatter{}
}

func (f *JSONLogFormatter) Format(ctx context.Context, data []byte) ([]byte, error) {
    // Extract JSON formatting logic from check.go
    // Pretty-print JSON if it's valid JSON, otherwise return as-is
    if json.Valid(data) {
        return pretty.Pretty(data), nil
    }
    return data, nil
}
```

**logging/usage_tracker.go**:

```go
type TokenUsageTracker struct {
    mu    sync.RWMutex
    usage map[int]models.TokenUsage // Job index -> token usage
}

func NewTokenUsageTracker() *TokenUsageTracker {
    return &TokenUsageTracker{
        usage: make(map[int]models.TokenUsage),
    }
}

func (t *TokenUsageTracker) RecordUsage(jobIndex int, usage models.TokenUsage) {
    t.mu.Lock()
    defer t.mu.Unlock()
    t.usage[jobIndex] = usage
}

func (t *TokenUsageTracker) GetUsage(jobIndex int) (models.TokenUsage, bool) {
    t.mu.RLock()
    defer t.mu.RUnlock()
    usage, ok := t.usage[jobIndex]
    return usage, ok
}

func (t *TokenUsageTracker) TotalUsage() models.TokenUsage {
    t.mu.RLock()
    defer t.mu.RUnlock()

    total := models.TokenUsage{}
    for _, usage := range t.usage {
        total.InputTokens += usage.InputTokens
        total.OutputTokens += usage.OutputTokens
        total.ThinkingTokens += usage.ThinkingTokens
        total.CacheReadTokens += usage.CacheReadTokens
        total.TotalTokens += usage.TotalTokens
    }
    return total
}
```

### 2.4 Prompt Building (infrastructure/prompts/)

**prompts/builder.go**:

```go
type PromptBuilder struct {
    fs ports.FileSystem
}

func NewPromptBuilder(fs ports.FileSystem) *PromptBuilder {
    return &PromptBuilder{fs: fs}
}

func (b *PromptBuilder) BuildPrompt(ctx context.Context, job models.Job) (string, error) {
    // Extract prompt building logic from check.go
    var sb strings.Builder

    // Write header
    sb.WriteString(fmt.Sprintf("# Batch: %s\n\n", job.Name))
    sb.WriteString("## Issues\n\n")

    // Write each issue
    for _, issue := range job.Issues {
        sb.WriteString(fmt.Sprintf("### %s\n\n", issue.Name))
        sb.WriteString(issue.Content)
        sb.WriteString("\n\n")
    }

    return sb.String(), nil
}

func (b *PromptBuilder) WritePromptToFile(ctx context.Context, prompt string, path string) error {
    return b.fs.WriteFile(ctx, path, []byte(prompt), 0644)
}
```

**prompts/formatter.go**:

```go
type PromptFormatter struct{}

func NewPromptFormatter() *PromptFormatter {
    return &PromptFormatter{}
}

func (f *PromptFormatter) FormatIssue(issue models.Issue) string {
    // Extract issue formatting logic from check.go
}

func (f *PromptFormatter) FormatBatchHeader(jobName string, files []string) string {
    // Extract batch header formatting logic
}
```

### Relevant Files

**Files to Create**:

- `scripts/markdown/infrastructure/filesystem/reader.go`
- `scripts/markdown/infrastructure/filesystem/writer.go`
- `scripts/markdown/infrastructure/filesystem/filesystem.go`
- `scripts/markdown/infrastructure/filesystem/scanner.go`
- `scripts/markdown/infrastructure/execution/runner.go`
- `scripts/markdown/infrastructure/execution/ide_builder.go`
- `scripts/markdown/infrastructure/execution/factory.go`
- `scripts/markdown/infrastructure/logging/tap.go`
- `scripts/markdown/infrastructure/logging/formatter.go`
- `scripts/markdown/infrastructure/logging/usage_tracker.go`
- `scripts/markdown/infrastructure/prompts/builder.go`
- `scripts/markdown/infrastructure/prompts/formatter.go`

**Test Files**:

- `scripts/markdown/infrastructure/filesystem/reader_test.go`
- `scripts/markdown/infrastructure/filesystem/writer_test.go`
- `scripts/markdown/infrastructure/filesystem/scanner_test.go`
- `scripts/markdown/infrastructure/execution/runner_test.go`
- `scripts/markdown/infrastructure/execution/ide_builder_test.go`
- `scripts/markdown/infrastructure/logging/tap_test.go`
- `scripts/markdown/infrastructure/logging/usage_tracker_test.go`
- `scripts/markdown/infrastructure/prompts/builder_test.go`

### Dependent Files

**Dependencies from Task 1.0**:

- `scripts/markdown/core/models/*.go` - Domain models
- `scripts/markdown/core/ports/*.go` - Interface definitions
- `scripts/markdown/shared/types/constants.go` - Constants

**Reference for extraction**:

- `scripts/markdown/check.go` - Source of infrastructure logic

## Deliverables

- [ ] File system abstractions fully implemented and tested
- [ ] Command execution layer complete with IDE tool support
- [ ] Logging infrastructure with tap and formatter
- [ ] Prompt building system functional
- [ ] All infrastructure components implement their respective port interfaces
- [ ] Comprehensive unit tests for all components
- [ ] Integration tests for command execution
- [ ] All code passes `make lint`
- [ ] All tests pass with race detector

## Tests

### Unit Tests

- [ ] **FileSystem Tests**:
  - ReadFile with valid path
  - ReadFile with non-existent file
  - ReadFile with permission errors
  - WriteFile with valid data
  - WriteFile with directory creation
  - ListDir with valid directory
  - ListDir with empty directory
  - FileExists checks

- [ ] **Command Execution Tests**:
  - Execute successful command
  - Execute command with non-zero exit code
  - Execute command with timeout (context cancellation)
  - Execute command with stdin/stdout/stderr
  - IsAvailable for existing tool
  - IsAvailable for missing tool

- [ ] **IDE Builder Tests**:
  - BuildCommand for codex with default model
  - BuildCommand for claude with custom model
  - BuildCommand for droid with reasoning effort
  - BuildCommand with invalid prompt path
  - Factory creates correct builder for each IDE type
  - Factory error for unsupported IDE

- [ ] **Logging Tests**:
  - LogTap captures writes correctly
  - LogTap returns all lines
  - LogTap returns tail lines
  - LogTap is thread-safe (concurrent writes)
  - JSONLogFormatter formats valid JSON
  - JSONLogFormatter passes through non-JSON
  - TokenUsageTracker records usage
  - TokenUsageTracker calculates total
  - TokenUsageTracker is thread-safe

- [ ] **Prompt Building Tests**:
  - BuildPrompt with single issue
  - BuildPrompt with multiple issues
  - BuildPrompt with empty issues
  - WritePromptToFile creates file correctly
  - FormatIssue preserves content
  - FormatBatchHeader includes all files

### Integration Tests

- [ ] **File System Integration**:
  - Create, write, read, and delete file cycle
  - Directory scanning with real directory structure
  - Permission handling

- [ ] **Command Execution Integration** (only if tools are available):
  - Execute echo command and capture output
  - Execute command with timeout
  - Execute command with working directory

## Success Criteria

### Functional Requirements

- [ ] All port interfaces fully implemented
- [ ] File system operations work correctly
- [ ] Command execution handles all edge cases
- [ ] Logging captures and formats output correctly
- [ ] Prompt building generates valid prompts

### Architectural Requirements

- [ ] Infrastructure layer depends only on core/ports (DIP)
- [ ] No business logic in infrastructure layer
- [ ] All implementations are swappable
- [ ] Proper error handling and wrapping
- [ ] Context propagation throughout

### Quality Requirements

- [ ] All functions < 50 lines
- [ ] All code passes `make lint`
- [ ] All tests pass with: `gotestsum --format pkgname -- -race -parallel=4 ./scripts/markdown/infrastructure/...`
- [ ] Test coverage > 80%
- [ ] No race conditions (verified with -race flag)
- [ ] Proper resource cleanup (no leaked file handles)

### Integration Requirements

- [ ] Can be used by Task 3.0 (Core Logic)
- [ ] Can be wired in Task 5.0 (Application Wiring)
- [ ] Mock implementations available for testing

## Implementation Notes

### Order of Implementation

1. File system layer (reader, writer, filesystem, scanner)
2. Command execution layer (runner, builders, factory)
3. Logging infrastructure (tap, formatter, tracker)
4. Prompt building (builder, formatter)
5. Unit tests for each component
6. Integration tests
7. Run `make fmt && make lint && make test`

### Key Design Decisions

- **Interface-based**: All implementations respect port interfaces
- **Context-first**: All I/O operations accept context
- **Logger from context**: Use `logger.FromContext(ctx)` for all logging
- **Error wrapping**: Use `fmt.Errorf` with `%w` for error chains
- **Resource cleanup**: Defer file handle closures

### Common Pitfalls to Avoid

- ❌ Don't use `context.Background()` in implementation code
- ❌ Don't leak file handles or goroutines
- ❌ Don't hardcode configuration values
- ❌ Don't add business logic to infrastructure layer
- ❌ Don't create global state or singletons

### Testing Strategy

- **Unit tests**: Mock external dependencies (file system, commands)
- **Integration tests**: Use real file system in temp directories
- **Thread safety**: Test concurrent access with race detector
- **Error paths**: Test all error conditions
- **Context cancellation**: Verify proper cleanup on cancellation

### Parallelization Notes

This task can be executed in parallel with:

- **Task 3.0 (Core Logic)**: Different layers, minimal dependencies
- **Task 4.0 (UI Layer)**: Different layers, minimal dependencies

Both can use the interfaces defined in Task 1.0 without waiting for implementations.

## Dependencies

**Blocks**: Task 5.0 (Application Wiring)

**Blocked By**: Task 1.0 (Foundation)

**Parallel With**: Task 3.0 (Core Logic), Task 4.0 (UI Layer)

## Estimated Effort

**Size**: Medium (M)
**Duration**: 1-2 days
**Complexity**: Medium - Straightforward implementations, but comprehensive testing required
