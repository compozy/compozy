## markdown

## status: pending

<task_context>
<domain>scripts/markdown/cmd</domain>
<type>integration</type>
<scope>application_wiring</scope>
<complexity>medium</complexity>
<dependencies>task_1.0,task_2.0,task_3.0,task_4.0</dependencies>
</task_context>

# Task 5.0: Application Wiring - Dependency Injection and Integration

## Overview

Wire all the refactored components together into a cohesive application. This task implements the application layer (cmd/check/) which handles CLI setup, dependency injection, and orchestration of the entire workflow. This is where all the layers come together.

This task MUST wait for Tasks 2.0, 3.0, and 4.0 to complete, as it depends on all their implementations.

<critical>**MUST READ BEFORE STARTING** @.cursor/rules/go-coding-standards.mdc and @.cursor/rules/architecture.mdc</critical>

<requirements>
- Proper dependency injection (no global state)
- Context propagation throughout the application
- Graceful shutdown on signals (SIGINT, SIGTERM)
- Use `logger.FromContext(ctx)` and `config.FromContext(ctx)`
- All functions < 50 lines
- Clean separation between CLI parsing and business logic
- No business logic in cmd layer (only wiring)
</requirements>

## Subtasks

- [ ] 5.1 Implement main application entry point (main.go)
- [ ] 5.2 Implement Cobra command setup (root.go)
- [ ] 5.3 Implement dependency injection container (di.go)
- [ ] 5.4 Implement CLI flow controller (flow.go)
- [ ] 5.5 Implement graceful shutdown handling
- [ ] 5.6 Write integration tests for complete workflows
- [ ] 5.7 Write end-to-end tests

## Implementation Details

### 5.1 Main Entry Point (cmd/check/main.go)

```go
package main

import (
    "context"
    "fmt"
    "os"
    "os/signal"
    "syscall"

    "github.com/compozy/compozy/engine/core/config"
    "github.com/compozy/compozy/engine/core/logger"
)

func main() {
    // Create root context with cancellation
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Setup signal handling for graceful shutdown
    ctx = setupSignalHandler(ctx, cancel)

    // Initialize logger and config (if needed for startup)
    ctx = initializeContext(ctx)

    // Execute root command
    if err := Execute(ctx); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}

func setupSignalHandler(ctx context.Context, cancel context.CancelFunc) context.Context {
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

    go func() {
        select {
        case sig := <-sigCh:
            logger := logger.FromContext(ctx)
            logger.Info("received signal, shutting down gracefully", "signal", sig)
            cancel()
        case <-ctx.Done():
        }
    }()

    return ctx
}

func initializeContext(ctx context.Context) context.Context {
    // Initialize logger if needed
    log := logger.New("check", "info")
    ctx = logger.WithContext(ctx, log)

    // Initialize config if needed
    // ctx = config.WithContext(ctx, cfg)

    return ctx
}
```

### 5.2 Cobra Command Setup (cmd/check/root.go)

```go
package main

import (
    "context"
    "fmt"

    "github.com/spf13/cobra"

    "scripts/markdown/core/models"
    "scripts/markdown/shared/types"
)

var (
    // CLI flags
    flagPR              string
    flagIssuesDir       string
    flagDryRun          bool
    flagConcurrent      int
    flagBatchSize       int
    flagIDE             string
    flagModel           string
    flagGrouped         bool
    flagTailLines       int
    flagReasoningEffort string
)

var rootCmd = &cobra.Command{
    Use:   "check",
    Short: "Solve PR issues by processing issue files and running IDE tools",
    Long: `Refactored solve-pr-issues tool with clean architecture.
Scans issue markdown files, groups them, and processes them using IDE tools (codex, claude, or droid).`,
    RunE: runCheckCommand,
}

func init() {
    setupFlags()
}

func setupFlags() {
    // Required flags
    rootCmd.Flags().StringVar(&flagPR, "pr", "", "PR number (required)")

    // Optional flags
    rootCmd.Flags().StringVar(&flagIssuesDir, "issues-dir", "", "Issues directory (auto-detected if not provided)")
    rootCmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Prepare but do not execute")
    rootCmd.Flags().IntVar(&flagConcurrent, "concurrent", 1, "Number of concurrent jobs")
    rootCmd.Flags().IntVar(&flagBatchSize, "batch-size", 3, "Number of issues per batch")
    rootCmd.Flags().StringVar(&flagIDE, "ide", types.IDECodex, "IDE tool to use (codex, claude, droid)")
    rootCmd.Flags().StringVar(&flagModel, "model", "", "AI model (default depends on IDE)")
    rootCmd.Flags().BoolVar(&flagGrouped, "grouped", false, "Generate grouped issue summaries")
    rootCmd.Flags().IntVar(&flagTailLines, "tail-lines", 5, "Number of log lines to show in UI")
    rootCmd.Flags().StringVar(&flagReasoningEffort, "reasoning-effort", "medium", "Reasoning effort (low, medium, high)")
}

func runCheckCommand(cmd *cobra.Command, args []string) error {
    ctx := cmd.Context()
    logger := logger.FromContext(ctx)

    logger.Info("starting check command", "pr", flagPR)

    // Check if interactive mode is needed
    if flagPR == "" {
        return runInteractiveMode(ctx, cmd)
    }

    // Build configuration
    config, err := buildConfig()
    if err != nil {
        return fmt.Errorf("invalid configuration: %w", err)
    }

    // Validate configuration
    if err := config.Validate(); err != nil {
        return fmt.Errorf("configuration validation failed: %w", err)
    }

    // Execute workflow
    return executeWorkflow(ctx, config)
}

func runInteractiveMode(ctx context.Context, cmd *cobra.Command) error {
    logger := logger.FromContext(ctx)
    logger.Info("entering interactive mode")

    // Use form builder from Task 4.0
    flow := NewCLIFlow()
    config, err := flow.CollectInput(ctx)
    if err != nil {
        return fmt.Errorf("failed to collect input: %w", err)
    }

    return executeWorkflow(ctx, config)
}

func buildConfig() (models.Config, error) {
    config := models.Config{
        PR:              flagPR,
        IssuesDir:       flagIssuesDir,
        DryRun:          flagDryRun,
        Concurrent:      flagConcurrent,
        BatchSize:       flagBatchSize,
        IDE:             flagIDE,
        Model:           flagModel,
        Grouped:         flagGrouped,
        TailLines:       flagTailLines,
        ReasoningEffort: flagReasoningEffort,
    }

    // Resolve issues directory if not provided
    if config.IssuesDir == "" {
        config.IssuesDir = fmt.Sprintf("ai-docs/reviews-pr-%s/issues", config.PR)
    }

    // Set default model if not provided
    if config.Model == "" {
        if config.IDE == types.IDEClaude {
            config.Model = types.DefaultClaudeModel
        } else {
            config.Model = types.DefaultCodexModel
        }
    }

    return config, nil
}

func Execute(ctx context.Context) error {
    rootCmd.SetContext(ctx)
    return rootCmd.Execute()
}
```

### 5.3 Dependency Injection (cmd/check/di.go)

```go
package main

import (
    "context"

    "scripts/markdown/core/services"
    "scripts/markdown/core/usecases"
    "scripts/markdown/infrastructure/execution"
    "scripts/markdown/infrastructure/filesystem"
    "scripts/markdown/infrastructure/logging"
    "scripts/markdown/infrastructure/prompts"
)

type Container struct {
    // Infrastructure
    fs            *filesystem.OSFileSystem
    executor      *execution.OSCommandExecutor
    ideFactory    *execution.IDEToolFactory
    logFormatter  *logging.JSONLogFormatter
    usageTracker  *logging.TokenUsageTracker

    // Services
    issueProcessor *services.IssueProcessor
    jobExecutor    *services.JobExecutor
    batchPreparer  *services.BatchPreparer
    summarizer     *services.Summarizer

    // Use Cases
    solveIssuesUC *usecases.SolveIssuesUseCase
}

func NewContainer(ctx context.Context) (*Container, error) {
    c := &Container{}

    // Initialize infrastructure layer
    if err := c.initInfrastructure(ctx); err != nil {
        return nil, err
    }

    // Initialize services layer
    if err := c.initServices(ctx); err != nil {
        return nil, err
    }

    // Initialize use cases layer
    if err := c.initUseCases(ctx); err != nil {
        return nil, err
    }

    return c, nil
}

func (c *Container) initInfrastructure(ctx context.Context) error {
    // File system
    c.fs = filesystem.NewOSFileSystem()

    // Command execution
    c.executor = execution.NewOSCommandExecutor()
    c.ideFactory = execution.NewIDEToolFactory(c.executor)

    // Logging
    c.logFormatter = logging.NewJSONLogFormatter()
    c.usageTracker = logging.NewTokenUsageTracker()

    return nil
}

func (c *Container) initServices(ctx context.Context) error {
    // Prompt builder
    promptBuilder := prompts.NewPromptBuilder(c.fs)

    // Services
    c.issueProcessor = services.NewIssueProcessor(c.fs)
    c.jobExecutor = services.NewJobExecutor(c.executor, c.ideFactory)
    c.batchPreparer = services.NewBatchPreparer(c.fs, promptBuilder)
    c.summarizer = services.NewSummarizer(c.fs)

    return nil
}

func (c *Container) initUseCases(ctx context.Context) error {
    // Main use case
    c.solveIssuesUC = usecases.NewSolveIssuesUseCase(
        c.issueProcessor,
        c.batchPreparer,
        c.jobExecutor,
        c.summarizer,
    )

    return nil
}

func (c *Container) SolveIssuesUseCase() *usecases.SolveIssuesUseCase {
    return c.solveIssuesUC
}

func (c *Container) Cleanup(ctx context.Context) error {
    // Cleanup resources if needed
    return nil
}
```

### 5.4 CLI Flow Controller (cmd/check/flow.go)

```go
package main

import (
    "context"
    "fmt"

    "scripts/markdown/core/models"
    "scripts/markdown/ui/forms"
)

type CLIFlow struct {
    formBuilder *forms.FormBuilder
    inputFactory *forms.InputFactory
    validator   *forms.Validator
}

func NewCLIFlow() *CLIFlow {
    return &CLIFlow{
        formBuilder:  forms.NewFormBuilder(),
        inputFactory: forms.NewInputFactory(),
        validator:    forms.NewValidator(),
    }
}

func (f *CLIFlow) CollectInput(ctx context.Context) (models.Config, error) {
    // String targets for form fields
    var (
        prStr              string
        issuesDirStr       string
        concurrentStr      string
        batchSizeStr       string
        ideStr             string
        modelStr           string
        tailLinesStr       string
        reasoningEffortStr string
        dryRun             bool
        grouped            bool
    )

    // Build form
    form := f.formBuilder.
        AddField(f.inputFactory.CreatePRInput(&prStr)).
        AddField(f.inputFactory.CreateIssuesDirInput(&issuesDirStr, "")).
        AddField(f.inputFactory.CreateConcurrentInput(&concurrentStr)).
        AddField(f.inputFactory.CreateBatchSizeInput(&batchSizeStr)).
        AddField(f.inputFactory.CreateIDESelectInput(&ideStr)).
        AddField(f.inputFactory.CreateModelInput(&modelStr, ideStr)).
        AddField(f.inputFactory.CreateReasoningEffortInput(&reasoningEffortStr)).
        AddField(f.inputFactory.CreateTailLinesInput(&tailLinesStr)).
        AddField(f.inputFactory.CreateConfirmInput("Dry Run", "Prepare but do not execute?", &dryRun)).
        AddField(f.inputFactory.CreateConfirmInput("Grouped", "Generate grouped summaries?", &grouped)).
        Build()

    // Run form
    if err := form.RunWithContext(ctx); err != nil {
        return models.Config{}, fmt.Errorf("form collection failed: %w", err)
    }

    // Parse and validate
    config, err := f.parseConfig(prStr, issuesDirStr, concurrentStr, batchSizeStr, ideStr, modelStr, tailLinesStr, reasoningEffortStr, dryRun, grouped)
    if err != nil {
        return models.Config{}, err
    }

    return config, nil
}

func (f *CLIFlow) parseConfig(prStr, issuesDirStr, concurrentStr, batchSizeStr, ideStr, modelStr, tailLinesStr, reasoningEffortStr string, dryRun, grouped bool) (models.Config, error) {
    // Parse PR
    if err := f.validator.ValidatePR(prStr); err != nil {
        return models.Config{}, err
    }

    // Parse concurrent
    concurrent := 1
    if concurrentStr != "" {
        if _, err := fmt.Sscanf(concurrentStr, "%d", &concurrent); err != nil {
            return models.Config{}, fmt.Errorf("invalid concurrent value: %w", err)
        }
    }
    if err := f.validator.ValidateConcurrency(concurrent); err != nil {
        return models.Config{}, err
    }

    // Parse batch size
    batchSize := 3
    if batchSizeStr != "" {
        if _, err := fmt.Sscanf(batchSizeStr, "%d", &batchSize); err != nil {
            return models.Config{}, fmt.Errorf("invalid batch size: %w", err)
        }
    }
    if err := f.validator.ValidateBatchSize(batchSize); err != nil {
        return models.Config{}, err
    }

    // Parse tail lines
    tailLines := 5
    if tailLinesStr != "" {
        if _, err := fmt.Sscanf(tailLinesStr, "%d", &tailLines); err != nil {
            return models.Config{}, fmt.Errorf("invalid tail lines: %w", err)
        }
    }

    // Validate IDE
    if err := f.validator.ValidateIDE(ideStr); err != nil {
        return models.Config{}, err
    }

    // Validate reasoning effort
    if err := f.validator.ValidateReasoningEffort(reasoningEffortStr); err != nil {
        return models.Config{}, err
    }

    // Build config
    config := models.Config{
        PR:              prStr,
        IssuesDir:       issuesDirStr,
        DryRun:          dryRun,
        Concurrent:      concurrent,
        BatchSize:       batchSize,
        IDE:             ideStr,
        Model:           modelStr,
        Grouped:         grouped,
        TailLines:       tailLines,
        ReasoningEffort: reasoningEffortStr,
    }

    // Resolve issues directory if empty
    if config.IssuesDir == "" {
        config.IssuesDir = fmt.Sprintf("ai-docs/reviews-pr-%s/issues", config.PR)
    }

    return config, nil
}
```

### 5.5 Workflow Execution (cmd/check/workflow.go)

```go
package main

import (
    "context"
    "fmt"

    "scripts/markdown/core/models"
    "github.com/compozy/compozy/engine/core/logger"
)

func executeWorkflow(ctx context.Context, config models.Config) error {
    logger := logger.FromContext(ctx)
    logger.Info("executing workflow", "pr", config.PR, "dry_run", config.DryRun)

    // Create DI container
    container, err := NewContainer(ctx)
    if err != nil {
        return fmt.Errorf("failed to create container: %w", err)
    }
    defer func() {
        if err := container.Cleanup(ctx); err != nil {
            logger.Warn("cleanup failed", "error", err)
        }
    }()

    // Execute use case
    useCase := container.SolveIssuesUseCase()
    if err := useCase.Execute(ctx, config); err != nil {
        return fmt.Errorf("workflow execution failed: %w", err)
    }

    logger.Info("workflow completed successfully")
    return nil
}
```

### Relevant Files

**Files to Create**:
- `scripts/markdown/cmd/check/main.go`
- `scripts/markdown/cmd/check/root.go`
- `scripts/markdown/cmd/check/di.go`
- `scripts/markdown/cmd/check/flow.go`
- `scripts/markdown/cmd/check/workflow.go`

**Test Files**:
- `scripts/markdown/cmd/check/root_test.go`
- `scripts/markdown/cmd/check/di_test.go`
- `scripts/markdown/cmd/check/flow_test.go`
- `scripts/markdown/cmd/check/workflow_test.go`
- `scripts/markdown/cmd/check/integration_test.go`

### Dependent Files

**Dependencies from All Previous Tasks**:
- Task 1.0: Domain models, ports, shared utilities
- Task 2.0: Infrastructure implementations
- Task 3.0: Services and use cases
- Task 4.0: UI components and forms

## Deliverables

- [ ] Main application entry point implemented
- [ ] Cobra command setup complete
- [ ] Dependency injection container functional
- [ ] CLI flow controller implemented
- [ ] Graceful shutdown handling working
- [ ] Integration tests for complete workflows
- [ ] End-to-end tests passing
- [ ] All code passes `make lint`
- [ ] All tests pass

## Tests

### Unit Tests

- [ ] **Root Command Tests**:
  - buildConfig with all flags
  - buildConfig with defaults
  - buildConfig resolves issues directory
  - buildConfig sets default model

- [ ] **DI Container Tests**:
  - NewContainer initializes all dependencies
  - Container cleanup works
  - Container provides correct instances

- [ ] **Flow Controller Tests**:
  - CollectInput returns valid config
  - parseConfig handles all inputs
  - parseConfig validates correctly
  - parseConfig uses defaults

### Integration Tests

- [ ] **End-to-End Workflow Tests**:
  - Complete workflow with small test data
  - Dry-run mode executes correctly
  - Interactive mode works
  - CLI flags override defaults
  - Graceful shutdown on signal
  - Context cancellation propagates
  - Errors are handled gracefully

- [ ] **Signal Handling Tests**:
  - SIGINT triggers graceful shutdown
  - SIGTERM triggers graceful shutdown
  - In-flight jobs complete gracefully
  - Resources are cleaned up

### E2E Tests

- [ ] **Complete Scenarios**:
  - Process real issue files (small set)
  - Execute with codex (if available)
  - Execute with dry-run
  - Execute with concurrency
  - Verify all artifacts created
  - Verify logs are written

## Success Criteria

### Functional Requirements
- [ ] CLI works with flags and interactive mode
- [ ] Complete workflow executes successfully
- [ ] Dry-run mode works correctly
- [ ] Graceful shutdown works
- [ ] All features from original check.go preserved

### Architectural Requirements
- [ ] Proper dependency injection throughout
- [ ] No global state or singletons
- [ ] Context propagation everywhere
- [ ] Clean separation of concerns
- [ ] All layers properly wired

### Quality Requirements
- [ ] All functions < 50 lines
- [ ] All code passes `make lint`
- [ ] All tests pass: `gotestsum --format pkgname -- -race -parallel=4 ./scripts/markdown/...`
- [ ] Integration tests verify complete workflows
- [ ] E2E tests verify real-world scenarios

### Migration Requirements
- [ ] Original CLI interface preserved
- [ ] All original functionality works
- [ ] Performance is comparable or better
- [ ] No breaking changes to user experience

## Implementation Notes

### Order of Implementation
1. Main entry point (main.go)
2. Cobra command setup (root.go)
3. Dependency injection (di.go)
4. CLI flow controller (flow.go)
5. Workflow execution (workflow.go)
6. Unit tests
7. Integration tests
8. E2E tests
9. Run `make fmt && make lint && make test`

### Key Design Decisions
- **No business logic**: Cmd layer only wires components
- **Context everywhere**: Proper context propagation
- **Graceful shutdown**: Handle signals properly
- **DI container**: Centralized dependency management
- **Separation of concerns**: CLI, DI, and workflow are separate

### Common Pitfalls to Avoid
- ❌ Don't add business logic to cmd layer
- ❌ Don't use global state
- ❌ Don't skip context propagation
- ❌ Don't forget graceful shutdown
- ❌ Don't hardcode dependencies

### Testing Strategy
- **Unit tests**: Test each file independently
- **Integration tests**: Test complete workflows
- **E2E tests**: Test with real data (small sets)
- **Signal tests**: Verify graceful shutdown

### Dependencies
This task CANNOT proceed in parallel. It requires:
- Task 1.0 (Foundation) - COMPLETE
- Task 2.0 (Infrastructure) - COMPLETE
- Task 3.0 (Core Logic) - COMPLETE
- Task 4.0 (UI Layer) - COMPLETE

## Dependencies

**Blocks**: Task 6.0 (Testing & Migration)

**Blocked By**: Tasks 1.0, 2.0, 3.0, 4.0 (ALL must be complete)

**Parallel With**: None (must wait for all parallel tracks to complete)

## Estimated Effort

**Size**: Medium (M)
**Duration**: 1-2 days
**Complexity**: Medium - Wiring complexity, but no complex logic
