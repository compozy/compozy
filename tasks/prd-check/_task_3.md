## markdown

## status: pending

<task_context>
<domain>scripts/markdown/core</domain>
<type>implementation</type>
<scope>business_logic</scope>
<complexity>high</complexity>
<dependencies>task_1.0</dependencies>
</task_context>

# Task 3.0: Core Business Logic - Extract Services and Use Cases

## Overview

Extract and refactor all business logic from check.go into well-structured services and use cases. This is the heart of the refactoring, implementing the domain layer with pure business logic independent of UI and infrastructure concerns.

This task implements the critical path through the system and can be executed in parallel with Tasks 2.0 (Infrastructure) and 4.0 (UI Layer) after Task 1.0 is complete.

<critical>**MUST READ BEFORE STARTING** @.cursor/rules/go-coding-standards.mdc and @.cursor/rules/architecture.mdc</critical>

<requirements>
- Pure business logic with no infrastructure dependencies
- Depend only on port interfaces (DIP)
- All functions < 50 lines
- Context-first APIs
- Use `logger.FromContext(ctx)` for logging
- Comprehensive error handling with fmt.Errorf
- Thread-safe implementations where concurrent access is expected
- No global state
</requirements>

## Subtasks

- [ ] 3.1 Implement issue processing service
- [ ] 3.2 Implement job execution orchestration service
- [ ] 3.3 Implement batch grouping and preparation service
- [ ] 3.4 Implement summary generation service
- [ ] 3.5 Implement main use case orchestrator (SolveIssues)
- [ ] 3.6 Write comprehensive unit tests for all services
- [ ] 3.7 Write integration tests for use case orchestration

## Implementation Details

### 3.1 Issue Processing Service (core/services/issue_processor.go)

```go
type IssueProcessor struct {
    fs ports.FileSystem
}

func NewIssueProcessor(fs ports.FileSystem) *IssueProcessor {
    return &IssueProcessor{fs: fs}
}

// ScanIssueDirectory scans a directory for issue markdown files
func (p *IssueProcessor) ScanIssueDirectory(ctx context.Context, dir string) ([]models.Issue, error) {
    logger := logger.FromContext(ctx)
    logger.Debug("scanning issue directory", "dir", dir)

    files, err := p.fs.ListDir(ctx, dir)
    if err != nil {
        return nil, fmt.Errorf("failed to list directory: %w", err)
    }

    issues := make([]models.Issue, 0, len(files))
    for _, file := range files {
        if !strings.HasSuffix(file, ".md") {
            continue
        }

        issue, err := p.loadIssue(ctx, filepath.Join(dir, file))
        if err != nil {
            logger.Warn("failed to load issue", "file", file, "error", err)
            continue
        }

        issues = append(issues, issue)
    }

    logger.Info("scanned issues", "count", len(issues))
    return issues, nil
}

func (p *IssueProcessor) loadIssue(ctx context.Context, path string) (models.Issue, error) {
    // Extract issue loading logic from check.go
    // Read file, parse content, extract code file path
}

// GroupIssuesByCodeFile groups issues by the code file they reference
func (p *IssueProcessor) GroupIssuesByCodeFile(ctx context.Context, issues []models.Issue) (map[string][]models.Issue, error) {
    logger := logger.FromContext(ctx)
    logger.Debug("grouping issues by code file", "total", len(issues))

    groups := make(map[string][]models.Issue)
    for _, issue := range issues {
        codeFile := issue.CodeFile
        if codeFile == "" {
            codeFile = types.UnknownFileName
        }
        groups[codeFile] = append(groups[codeFile], issue)
    }

    logger.Info("grouped issues", "groups", len(groups))
    return groups, nil
}

func (p *IssueProcessor) parseCodeFileFromContent(content string) string {
    // Extract regex logic from check.go to find "**File:**`path:line`"
}
```

### 3.2 Job Execution Service (core/services/job_executor.go)

```go
type JobExecutor struct {
    executor   ports.CommandExecutor
    ideFactory *infrastructure.IDEToolFactory
    logTap     ports.LogTap
}

func NewJobExecutor(executor ports.CommandExecutor, ideFactory *infrastructure.IDEToolFactory) *JobExecutor {
    return &JobExecutor{
        executor:   executor,
        ideFactory: ideFactory,
    }
}

// ExecuteJob executes a single job using the configured IDE tool
func (e *JobExecutor) ExecuteJob(ctx context.Context, job models.Job, config models.Config) (models.JobResult, error) {
    logger := logger.FromContext(ctx)
    logger.Info("executing job", "index", job.Index, "name", job.Name)

    startTime := time.Now()

    // Build IDE command
    builder, err := e.ideFactory.CreateBuilder(config.IDE)
    if err != nil {
        return models.JobResult{Job: job, Success: false, Error: err}, fmt.Errorf("failed to create IDE builder: %w", err)
    }

    cmd, err := builder.BuildCommand(ctx, config, job.PromptPath)
    if err != nil {
        return models.JobResult{Job: job, Success: false, Error: err}, fmt.Errorf("failed to build command: %w", err)
    }

    // Set up logging
    logFile, err := e.setupLogging(ctx, job.LogPath)
    if err != nil {
        return models.JobResult{Job: job, Success: false, Error: err}, fmt.Errorf("failed to setup logging: %w", err)
    }
    defer logFile.Close()

    cmd.Stdout = logFile
    cmd.Stderr = logFile

    // Execute
    result, err := e.executor.Execute(ctx, cmd)
    duration := time.Since(startTime)

    jobResult := models.JobResult{
        Job:      job,
        Duration: duration,
        ExitCode: result.ExitCode,
    }

    if err != nil {
        jobResult.Success = false
        jobResult.Error = err
        return jobResult, fmt.Errorf("job execution failed: %w", err)
    }

    jobResult.Success = true
    return jobResult, nil
}

func (e *JobExecutor) setupLogging(ctx context.Context, logPath string) (*os.File, error) {
    // Create log file and directory
}

// ExecuteJobsConcurrently executes multiple jobs with controlled concurrency
func (e *JobExecutor) ExecuteJobsConcurrently(ctx context.Context, jobs []models.Job, config models.Config, concurrency int) ([]models.JobResult, error) {
    logger := logger.FromContext(ctx)
    logger.Info("executing jobs concurrently", "total", len(jobs), "concurrency", concurrency)

    resultsCh := make(chan models.JobResult, len(jobs))
    semaphore := make(chan struct{}, concurrency)

    var wg sync.WaitGroup
    for i := range jobs {
        wg.Add(1)
        go func(job models.Job) {
            defer wg.Done()

            // Acquire semaphore
            semaphore <- struct{}{}
            defer func() { <-semaphore }()

            result, err := e.ExecuteJob(ctx, job, config)
            if err != nil {
                logger.Error("job failed", "index", job.Index, "error", err)
            }
            resultsCh <- result
        }(jobs[i])
    }

    // Wait for completion
    wg.Wait()
    close(resultsCh)

    // Collect results
    results := make([]models.JobResult, 0, len(jobs))
    for result := range resultsCh {
        results = append(results, result)
    }

    // Sort by job index
    sort.Slice(results, func(i, j int) bool {
        return results[i].Job.Index < results[j].Job.Index
    })

    return results, nil
}
```

### 3.3 Batch Preparation Service (core/services/batch_preparer.go)

```go
type BatchPreparer struct {
    fs            ports.FileSystem
    promptBuilder *infrastructure.PromptBuilder
}

func NewBatchPreparer(fs ports.FileSystem, promptBuilder *infrastructure.PromptBuilder) *BatchPreparer {
    return &BatchPreparer{
        fs:            fs,
        promptBuilder: promptBuilder,
    }
}

// CreateBatches groups issues into batches based on batch size
func (p *BatchPreparer) CreateBatches(ctx context.Context, issues []models.Issue, batchSize int) ([][]models.Issue, error) {
    logger := logger.FromContext(ctx)
    logger.Debug("creating batches", "total_issues", len(issues), "batch_size", batchSize)

    if batchSize < 1 {
        return nil, fmt.Errorf("batch size must be at least 1")
    }

    batches := make([][]models.Issue, 0)
    for i := 0; i < len(issues); i += batchSize {
        end := i + batchSize
        if end > len(issues) {
            end = len(issues)
        }
        batches = append(batches, issues[i:end])
    }

    logger.Info("created batches", "count", len(batches))
    return batches, nil
}

// PrepareJob creates a job from a batch of issues
func (p *BatchPreparer) PrepareJob(ctx context.Context, batchIndex int, batch []models.Issue, config models.Config) (models.Job, error) {
    logger := logger.FromContext(ctx)

    // Determine job name
    files := p.extractUniqueFiles(batch)
    jobName := p.generateJobName(batchIndex, files)

    // Create job structure
    job := models.Job{
        Index:  batchIndex,
        Name:   jobName,
        Files:  files,
        Issues: batch,
    }

    // Generate paths
    promptRoot := p.getPromptRoot(config.PR)
    job.PromptPath = filepath.Join(promptRoot, fmt.Sprintf("batch_%d_prompt.md", batchIndex))
    job.LogPath = filepath.Join(promptRoot, fmt.Sprintf("batch_%d_log.txt", batchIndex))
    job.OutputPath = filepath.Join(promptRoot, fmt.Sprintf("batch_%d_output.json", batchIndex))

    // Build and write prompt
    prompt, err := p.promptBuilder.BuildPrompt(ctx, job)
    if err != nil {
        return models.Job{}, fmt.Errorf("failed to build prompt: %w", err)
    }

    if err := p.promptBuilder.WritePromptToFile(ctx, prompt, job.PromptPath); err != nil {
        return models.Job{}, fmt.Errorf("failed to write prompt: %w", err)
    }

    logger.Debug("prepared job", "index", batchIndex, "name", jobName)
    return job, nil
}

func (p *BatchPreparer) extractUniqueFiles(issues []models.Issue) []string {
    // Extract unique code files from issues
}

func (p *BatchPreparer) generateJobName(batchIndex int, files []string) string {
    // Extract job naming logic from check.go
}

func (p *BatchPreparer) getPromptRoot(pr string) string {
    return filepath.Join(".tmp", "codex-prompts", fmt.Sprintf("pr-%s", pr))
}
```

### 3.4 Summary Generation Service (core/services/summarizer.go)

```go
type Summarizer struct {
    fs ports.FileSystem
}

func NewSummarizer(fs ports.FileSystem) *Summarizer {
    return &Summarizer{fs: fs}
}

// GenerateGroupedSummaries writes grouped issue summaries to files
func (s *Summarizer) GenerateGroupedSummaries(ctx context.Context, issuesDir string, groups map[string][]models.Issue) error {
    logger := logger.FromContext(ctx)
    logger.Info("generating grouped summaries", "groups", len(groups))

    groupedDir := filepath.Join(issuesDir, "grouped")
    if err := s.fs.MkdirAll(ctx, groupedDir, 0755); err != nil {
        return fmt.Errorf("failed to create grouped directory: %w", err)
    }

    for codeFile, issues := range groups {
        summary, err := s.buildSummary(ctx, codeFile, issues)
        if err != nil {
            return fmt.Errorf("failed to build summary for %s: %w", codeFile, err)
        }

        safeFileName := s.makeSafeFileName(codeFile)
        summaryPath := filepath.Join(groupedDir, fmt.Sprintf("%s.md", safeFileName))

        if err := s.fs.WriteFile(ctx, summaryPath, []byte(summary), 0644); err != nil {
            return fmt.Errorf("failed to write summary: %w", err)
        }
    }

    logger.Info("generated grouped summaries", "count", len(groups))
    return nil
}

func (s *Summarizer) buildSummary(ctx context.Context, codeFile string, issues []models.Issue) (string, error) {
    // Extract summary building logic from check.go
}

func (s *Summarizer) makeSafeFileName(name string) string {
    // Extract safe filename logic from check.go
}

// GenerateExecutionReport creates a final execution report
func (s *Summarizer) GenerateExecutionReport(ctx context.Context, results []models.JobResult) string {
    var sb strings.Builder

    successful := 0
    failed := 0
    totalDuration := time.Duration(0)
    totalTokens := models.TokenUsage{}

    for _, result := range results {
        if result.Success {
            successful++
        } else {
            failed++
        }
        totalDuration += result.Duration
        totalTokens.InputTokens += result.TokensUsed.InputTokens
        totalTokens.OutputTokens += result.TokensUsed.OutputTokens
        totalTokens.ThinkingTokens += result.TokensUsed.ThinkingTokens
        totalTokens.TotalTokens += result.TokensUsed.TotalTokens
    }

    sb.WriteString(fmt.Sprintf("Execution Report:\n"))
    sb.WriteString(fmt.Sprintf("  Total Jobs: %d\n", len(results)))
    sb.WriteString(fmt.Sprintf("  Successful: %d\n", successful))
    sb.WriteString(fmt.Sprintf("  Failed: %d\n", failed))
    sb.WriteString(fmt.Sprintf("  Total Duration: %s\n", totalDuration))
    sb.WriteString(fmt.Sprintf("  Total Tokens: %d\n", totalTokens.TotalTokens))

    return sb.String()
}
```

### 3.5 Main Use Case Orchestrator (core/usecases/solve_issues.go)

```go
type SolveIssuesUseCase struct {
    issueProcessor *services.IssueProcessor
    batchPreparer  *services.BatchPreparer
    jobExecutor    *services.JobExecutor
    summarizer     *services.Summarizer
}

func NewSolveIssuesUseCase(
    issueProcessor *services.IssueProcessor,
    batchPreparer *services.BatchPreparer,
    jobExecutor *services.JobExecutor,
    summarizer *services.Summarizer,
) *SolveIssuesUseCase {
    return &SolveIssuesUseCase{
        issueProcessor: issueProcessor,
        batchPreparer:  batchPreparer,
        jobExecutor:    jobExecutor,
        summarizer:     summarizer,
    }
}

// Execute runs the complete solve issues workflow
func (uc *SolveIssuesUseCase) Execute(ctx context.Context, config models.Config) error {
    logger := logger.FromContext(ctx)
    logger.Info("starting solve issues workflow", "pr", config.PR)

    // Step 1: Prepare
    prep, err := uc.prepare(ctx, config)
    if err != nil {
        return fmt.Errorf("preparation failed: %w", err)
    }

    // Step 2: Execute jobs
    if config.DryRun {
        logger.Info("dry run mode - skipping execution")
        return nil
    }

    results, err := uc.execute(ctx, prep, config)
    if err != nil {
        return fmt.Errorf("execution failed: %w", err)
    }

    // Step 3: Generate report
    report := uc.summarizer.GenerateExecutionReport(ctx, results)
    logger.Info("workflow completed", "report", report)

    return nil
}

func (uc *SolveIssuesUseCase) prepare(ctx context.Context, config models.Config) (*models.Preparation, error) {
    // Scan issues
    issues, err := uc.issueProcessor.ScanIssueDirectory(ctx, config.IssuesDir)
    if err != nil {
        return nil, err
    }

    // Group issues
    groups, err := uc.issueProcessor.GroupIssuesByCodeFile(ctx, issues)
    if err != nil {
        return nil, err
    }

    // Generate grouped summaries if requested
    if config.Grouped {
        if err := uc.summarizer.GenerateGroupedSummaries(ctx, config.IssuesDir, groups); err != nil {
            return nil, err
        }
    }

    // Flatten and create batches
    flatIssues := uc.flattenIssues(groups)
    batches, err := uc.batchPreparer.CreateBatches(ctx, flatIssues, config.BatchSize)
    if err != nil {
        return nil, err
    }

    // Prepare jobs
    jobs, err := uc.prepareJobs(ctx, batches, config)
    if err != nil {
        return nil, err
    }

    return &models.Preparation{
        ResolvedIssuesDir: config.IssuesDir,
        PromptRoot:        uc.batchPreparer.getPromptRoot(config.PR),
        AllIssues:         flatIssues,
        Jobs:              jobs,
    }, nil
}

func (uc *SolveIssuesUseCase) execute(ctx context.Context, prep *models.Preparation, config models.Config) ([]models.JobResult, error) {
    return uc.jobExecutor.ExecuteJobsConcurrently(ctx, prep.Jobs, config, config.Concurrent)
}

func (uc *SolveIssuesUseCase) flattenIssues(groups map[string][]models.Issue) []models.Issue {
    // Extract flattening logic
}

func (uc *SolveIssuesUseCase) prepareJobs(ctx context.Context, batches [][]models.Issue, config models.Config) ([]models.Job, error) {
    jobs := make([]models.Job, 0, len(batches))
    for i, batch := range batches {
        job, err := uc.batchPreparer.PrepareJob(ctx, i, batch, config)
        if err != nil {
            return nil, fmt.Errorf("failed to prepare job %d: %w", i, err)
        }
        jobs = append(jobs, job)
    }
    return jobs, nil
}
```

### Relevant Files

**Files to Create**:

- `scripts/markdown/core/services/issue_processor.go`
- `scripts/markdown/core/services/job_executor.go`
- `scripts/markdown/core/services/batch_preparer.go`
- `scripts/markdown/core/services/summarizer.go`
- `scripts/markdown/core/usecases/solve_issues.go`

**Test Files**:

- `scripts/markdown/core/services/issue_processor_test.go`
- `scripts/markdown/core/services/job_executor_test.go`
- `scripts/markdown/core/services/batch_preparer_test.go`
- `scripts/markdown/core/services/summarizer_test.go`
- `scripts/markdown/core/usecases/solve_issues_test.go`

### Dependent Files

**Dependencies from Task 1.0**:

- `scripts/markdown/core/models/*.go` - Domain models
- `scripts/markdown/core/ports/*.go` - Interface definitions
- `scripts/markdown/shared/*.go` - Shared utilities

**Dependencies from Task 2.0** (interfaces only, not implementations):

- `scripts/markdown/core/ports/filesystem.go`
- `scripts/markdown/core/ports/executor.go`

**Reference for extraction**:

- `scripts/markdown/check.go` - Source of business logic

## Deliverables

- [ ] All services implemented and tested
- [ ] Use case orchestrator complete
- [ ] All business logic extracted from check.go
- [ ] Services depend only on port interfaces
- [ ] Comprehensive unit tests with > 80% coverage
- [ ] Integration tests for use case orchestration
- [ ] All code passes `make lint`
- [ ] All tests pass with race detector

## Tests

### Unit Tests for Services

- [ ] **IssueProcessor Tests**:
  - ScanIssueDirectory with valid directory
  - ScanIssueDirectory with no markdown files
  - ScanIssueDirectory with malformed files
  - GroupIssuesByCodeFile with mixed issues
  - GroupIssuesByCodeFile with unknown files
  - ParseCodeFileFromContent with valid header
  - ParseCodeFileFromContent with missing header

- [ ] **JobExecutor Tests**:
  - ExecuteJob with successful execution
  - ExecuteJob with failed execution
  - ExecuteJob with context cancellation
  - ExecuteJobsConcurrently with multiple jobs
  - ExecuteJobsConcurrently respects concurrency limit
  - ExecuteJobsConcurrently handles partial failures
  - SetupLogging creates directories
  - SetupLogging handles write errors

- [ ] **BatchPreparer Tests**:
  - CreateBatches with exact multiple
  - CreateBatches with remainder
  - CreateBatches with batch size > total
  - CreateBatches with batch size = 1
  - PrepareJob creates valid job structure
  - PrepareJob writes prompt file
  - PrepareJob generates correct paths
  - ExtractUniqueFiles removes duplicates
  - GenerateJobName with single file
  - GenerateJobName with multiple files

- [ ] **Summarizer Tests**:
  - GenerateGroupedSummaries creates files
  - GenerateGroupedSummaries handles write errors
  - BuildSummary formats correctly
  - MakeSafeFileName handles special characters
  - GenerateExecutionReport with all successful
  - GenerateExecutionReport with failures
  - GenerateExecutionReport calculates totals

### Integration Tests for Use Case

- [ ] **SolveIssuesUseCase Tests**:
  - Execute complete workflow (happy path)
  - Execute with dry-run mode
  - Execute with empty issues directory
  - Execute with concurrent jobs
  - Execute with context cancellation
  - Prepare phase creates correct jobs
  - Prepare phase groups issues correctly
  - Execute phase handles job failures
  - Report generation includes all metrics

### Edge Cases

- [ ] **Concurrency Edge Cases**:
  - Race conditions in concurrent execution
  - Context cancellation during execution
  - Partial failures don't block other jobs
  - Results are collected in order

- [ ] **Error Handling**:
  - Graceful degradation on partial failures
  - Error wrapping maintains context
  - Errors are properly logged
  - Cleanup happens on errors

## Success Criteria

### Functional Requirements

- [ ] All business logic extracted from check.go
- [ ] Workflow executes correctly end-to-end
- [ ] Concurrent execution works without race conditions
- [ ] Dry-run mode works correctly
- [ ] Grouped summaries generated correctly

### Architectural Requirements

- [ ] Services depend only on port interfaces (DIP)
- [ ] No infrastructure dependencies in core layer
- [ ] Single responsibility per service (SRP)
- [ ] Services are composable and testable
- [ ] Use case orchestrates without business logic

### Quality Requirements

- [ ] All functions < 50 lines
- [ ] All code passes `make lint`
- [ ] All tests pass: `gotestsum --format pkgname -- -race -parallel=4 ./scripts/markdown/core/...`
- [ ] Test coverage > 80%
- [ ] No race conditions (verified with -race)
- [ ] All error paths tested

### Integration Requirements

- [ ] Can be wired with Task 2.0 infrastructure implementations
- [ ] Can be integrated with Task 4.0 UI components
- [ ] Ready for Task 5.0 dependency injection

## Implementation Notes

### Order of Implementation

1. IssueProcessor (no dependencies on other services)
2. Summarizer (minimal dependencies)
3. BatchPreparer (depends on IssueProcessor concepts)
4. JobExecutor (depends on infrastructure ports)
5. SolveIssuesUseCase (orchestrates all services)
6. Unit tests for each service
7. Integration tests for use case
8. Run `make fmt && make lint && make test`

### Key Design Decisions

- **Pure business logic**: No UI or infrastructure code
- **Port-based dependencies**: Depend on abstractions, not implementations
- **Orchestration in use case**: Services are atomic, use case coordinates
- **Thread-safe services**: Services can be called concurrently
- **Context propagation**: All methods accept context for cancellation

### Common Pitfalls to Avoid

- ❌ Don't add infrastructure code in services
- ❌ Don't couple services to each other (use interfaces)
- ❌ Don't use global state or singletons
- ❌ Don't create functions > 50 lines (split into helpers)
- ❌ Don't skip error handling or wrapping

### Testing Strategy

- **Unit tests**: Mock port interfaces for isolated testing
- **Integration tests**: Use real implementations with test fixtures
- **Table-driven tests**: For multiple scenarios
- **Concurrent tests**: Verify thread safety with race detector
- **Error injection**: Test error paths thoroughly

### Parallelization Notes

This task can be executed in parallel with:

- **Task 2.0 (Infrastructure)**: Use port interfaces, not implementations
- **Task 4.0 (UI Layer)**: Independent concerns

Both can proceed after Task 1.0 without blocking each other.

## Dependencies

**Blocks**: Task 5.0 (Application Wiring)

**Blocked By**: Task 1.0 (Foundation)

**Parallel With**: Task 2.0 (Infrastructure), Task 4.0 (UI Layer)

## Estimated Effort

**Size**: Large (L)
**Duration**: 3+ days
**Complexity**: High - Core business logic with complex orchestration and concurrency
