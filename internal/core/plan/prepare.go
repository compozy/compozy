package plan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/core/agent"
	reusableagents "github.com/compozy/compozy/internal/core/agents"
	"github.com/compozy/compozy/internal/core/memory"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/prompt"
	"github.com/compozy/compozy/internal/core/reviews"
	"github.com/compozy/compozy/internal/core/tasks"
)

// ErrNoWork indicates that no unresolved issues or pending PRD tasks were found.
var ErrNoWork = errors.New("no issues to process")

func Prepare(
	ctx context.Context,
	cfg *model.RuntimeConfig,
	scope model.RunScope,
) (*model.SolvePreparation, error) {
	if scope == nil {
		return nil, errors.New("prepare run: missing run scope")
	}

	prep := &model.SolvePreparation{}
	prep.RunArtifacts = scope.RunArtifacts()
	prep.SetRunScope(scope)
	var prepared bool
	defer func() {
		if prepared {
			return
		}
		ClosePreparationJournal(ctx, prep)
	}()

	agentExecution, err := reusableagents.ResolveExecutionContext(ctx, cfg)
	if err != nil {
		return nil, err
	}

	if cfg.Mode == model.ExecutionModeExec {
		execPrep, err := prepareExec(ctx, prep, cfg, agentExecution)
		if err != nil {
			return nil, err
		}
		prepared = true
		return execPrep, nil
	}

	if err := prepareWorkflowRun(ctx, prep, cfg, agentExecution); err != nil {
		return nil, err
	}

	prepared = true
	return prep, nil
}

func prepareWorkflowRun(
	ctx context.Context,
	prep *model.SolvePreparation,
	cfg *model.RuntimeConfig,
	agentExecution *reusableagents.ExecutionContext,
) error {
	entries, err := resolvePreparedEntries(ctx, prep, cfg)
	if err != nil {
		return err
	}

	groups, err := prepareWorkflowGroups(ctx, prep, cfg, entries)
	if err != nil {
		return err
	}
	prep.Jobs, err = prepareWorkflowJobs(ctx, prep, cfg, groups, agentExecution)
	if err != nil {
		return err
	}
	if err := ensureWorkflowRuntimesAvailable(ctx, cfg, prep.Jobs); err != nil {
		return err
	}

	return writeRunMetadata(prep, cfg)
}

func prepareWorkflowGroups(
	ctx context.Context,
	prep *model.SolvePreparation,
	cfg *model.RuntimeConfig,
	entries []model.IssueEntry,
) (map[string][]model.IssueEntry, error) {
	preGroup, err := model.DispatchMutableHook(
		ctx,
		prep.RuntimeManager(),
		"plan.pre_group",
		planEntriesPayload{
			RunID:   prep.RunArtifacts.RunID,
			Entries: entries,
		},
	)
	if err != nil {
		return nil, err
	}

	groupedEntries, _ := groupIssuesByCodeFile(preGroup.Entries)
	postGroup, err := model.DispatchMutableHook(
		ctx,
		prep.RuntimeManager(),
		"plan.post_group",
		planGroupsPayload{
			RunID:  prep.RunArtifacts.RunID,
			Groups: groupedEntries,
		},
	)
	if err != nil {
		return nil, err
	}
	return dispatchReviewPreBatch(ctx, prep, cfg, postGroup.Groups)
}

func dispatchReviewPreBatch(
	ctx context.Context,
	prep *model.SolvePreparation,
	cfg *model.RuntimeConfig,
	groups map[string][]model.IssueEntry,
) (map[string][]model.IssueEntry, error) {
	if cfg.Mode != model.ExecutionModePRReview {
		return groups, nil
	}

	reviewPreBatch, err := model.DispatchMutableHook(
		ctx,
		prep.RuntimeManager(),
		"review.pre_batch",
		reviewPreBatchPayload{
			RunID:  prep.RunArtifacts.RunID,
			PR:     cfg.PR,
			Groups: groups,
		},
	)
	if err != nil {
		return nil, err
	}
	return reviewPreBatch.Groups, nil
}

func prepareWorkflowJobs(
	ctx context.Context,
	prep *model.SolvePreparation,
	cfg *model.RuntimeConfig,
	groups map[string][]model.IssueEntry,
	agentExecution *reusableagents.ExecutionContext,
) ([]model.Job, error) {
	prePrepareJobs, err := model.DispatchMutableHook(
		ctx,
		prep.RuntimeManager(),
		"plan.pre_prepare_jobs",
		planGroupsPayload{
			RunID:  prep.RunArtifacts.RunID,
			Groups: groups,
		},
	)
	if err != nil {
		return nil, err
	}

	jobs, err := prepareJobs(
		ctx,
		cfg,
		prePrepareJobs.Groups,
		prep.RunArtifacts,
		prep.RuntimeManager(),
		agentExecution,
	)
	if err != nil {
		return nil, err
	}
	preparedJobs := clonePreparedJobsForRuntimeGuard(jobs)
	postPrepareJobs, err := model.DispatchMutableHook(
		ctx,
		prep.RuntimeManager(),
		"plan.post_prepare_jobs",
		planJobsPayload{
			RunID: prep.RunArtifacts.RunID,
			Jobs:  jobs,
		},
	)
	if err != nil {
		return nil, err
	}
	if err := validatePreparedJobRuntimeMutation(preparedJobs, postPrepareJobs.Jobs); err != nil {
		return nil, err
	}
	return postPrepareJobs.Jobs, nil
}

func resolvePreparedEntries(
	ctx context.Context,
	prep *model.SolvePreparation,
	cfg *model.RuntimeConfig,
) ([]model.IssueEntry, error) {
	var err error
	prep.ResolvedName, prep.InputDir, prep.InputDirPath, err = resolveInputs(cfg)
	if err != nil {
		return nil, err
	}
	if err := configureWorkflowInput(prep, cfg); err != nil {
		return nil, err
	}
	preDiscover, err := dispatchPlanPreDiscover(ctx, prep, cfg)
	if err != nil {
		return nil, err
	}
	if err := applyReviewPreFetch(ctx, prep, cfg); err != nil {
		return nil, err
	}

	entries, err := readIssueEntries(prep.InputDirPath, cfg.Mode, cfg.IncludeCompleted)
	if err != nil {
		return nil, err
	}
	entries, err = appendPreparedExtraEntries(prep, cfg, preDiscover.ExtraSources, entries)
	if err != nil {
		return nil, err
	}

	entries, err = validateAndFilterEntries(entries, cfg)
	if err != nil {
		return nil, err
	}
	entries, err = dispatchReviewPostFetch(ctx, prep, cfg, entries)
	if err != nil {
		return nil, err
	}
	postDiscover, err := model.DispatchMutableHook(
		ctx,
		prep.RuntimeManager(),
		"plan.post_discover",
		planPostDiscoverPayload{
			RunID:    prep.RunArtifacts.RunID,
			Workflow: prep.ResolvedName,
			Entries:  entries,
		},
	)
	if err != nil {
		return nil, err
	}
	return postDiscover.Entries, nil
}

func dispatchPlanPreDiscover(
	ctx context.Context,
	prep *model.SolvePreparation,
	cfg *model.RuntimeConfig,
) (planPreDiscoverPayload, error) {
	return model.DispatchMutableHook(
		ctx,
		prep.RuntimeManager(),
		"plan.pre_discover",
		planPreDiscoverPayload{
			RunID:    prep.RunArtifacts.RunID,
			Workflow: prep.ResolvedName,
			Mode:     cfg.Mode,
		},
	)
}

func applyReviewPreFetch(
	ctx context.Context,
	prep *model.SolvePreparation,
	cfg *model.RuntimeConfig,
) error {
	if cfg.Mode != model.ExecutionModePRReview {
		return nil
	}

	reviewPreFetch, err := model.DispatchMutableHook(
		ctx,
		prep.RuntimeManager(),
		"review.pre_fetch",
		reviewPreFetchPayload{
			RunID:    prep.RunArtifacts.RunID,
			PR:       cfg.PR,
			Provider: cfg.Provider,
			FetchConfig: model.FetchConfig{
				ReviewsDir:      prep.InputDirPath,
				IncludeResolved: cfg.IncludeResolved,
			},
		},
	)
	if err != nil {
		return err
	}
	if trimmed := strings.TrimSpace(reviewPreFetch.FetchConfig.ReviewsDir); trimmed != "" {
		resolvedReviewsDir, err := filepath.Abs(trimmed)
		if err != nil {
			return fmt.Errorf("resolve review fetch directory %q: %w", trimmed, err)
		}
		cfg.ReviewsDir = resolvedReviewsDir
		prep.InputDir = trimmed
		prep.InputDirPath = resolvedReviewsDir
	}
	cfg.IncludeResolved = reviewPreFetch.FetchConfig.IncludeResolved
	return nil
}

func appendPreparedExtraEntries(
	prep *model.SolvePreparation,
	cfg *model.RuntimeConfig,
	extraSources []string,
	entries []model.IssueEntry,
) ([]model.IssueEntry, error) {
	if len(extraSources) == 0 {
		return entries, nil
	}

	extraEntries, err := readExtraIssueEntries(
		resolveExtraSourceBaseDir(prep.InputDirPath, cfg.WorkspaceRoot),
		cfg,
		extraSources,
	)
	if err != nil {
		return nil, err
	}
	entries = append(entries, extraEntries...)
	return dedupeIssueEntries(entries), nil
}

func dispatchReviewPostFetch(
	ctx context.Context,
	prep *model.SolvePreparation,
	cfg *model.RuntimeConfig,
	entries []model.IssueEntry,
) ([]model.IssueEntry, error) {
	if cfg.Mode != model.ExecutionModePRReview {
		return entries, nil
	}

	reviewPostFetch, err := model.DispatchMutableHook(
		ctx,
		prep.RuntimeManager(),
		"review.post_fetch",
		reviewPostFetchPayload{
			RunID:  prep.RunArtifacts.RunID,
			PR:     cfg.PR,
			Issues: entries,
		},
	)
	if err != nil {
		return nil, err
	}
	return reviewPostFetch.Issues, nil
}

func configureWorkflowInput(prep *model.SolvePreparation, cfg *model.RuntimeConfig) error {
	if cfg.Mode == model.ExecutionModePRReview {
		return configureReviewInput(prep, cfg)
	}

	if _, err := tasks.RefreshTaskMeta(prep.InputDirPath); err != nil {
		return err
	}
	cfg.TasksDir = prep.InputDirPath
	return nil
}

func configureReviewInput(prep *model.SolvePreparation, cfg *model.RuntimeConfig) error {
	meta, err := reviews.ReadRoundMeta(prep.InputDirPath)
	if err != nil {
		return err
	}
	cfg.Provider = meta.Provider
	cfg.PR = meta.PR
	cfg.Round = meta.Round
	cfg.ReviewsDir = prep.InputDirPath
	prep.ResolvedProvider = meta.Provider
	prep.ResolvedPR = meta.PR
	prep.ResolvedRound = meta.Round
	return nil
}

func prepareExec(
	ctx context.Context,
	prep *model.SolvePreparation,
	cfg *model.RuntimeConfig,
	agentExecution *reusableagents.ExecutionContext,
) (*model.SolvePreparation, error) {
	promptText, err := resolveExecPrompt(cfg)
	if err != nil {
		return nil, err
	}
	if err := agent.EnsureAvailable(ctx, cfg); err != nil {
		return nil, err
	}

	prep.ResolvedName, prep.InputDir, prep.InputDirPath, err = resolveInputs(cfg)
	if err != nil {
		return nil, err
	}

	job, err := buildExecJob(prep.RunArtifacts, promptText, agentExecution, cfg)
	if err != nil {
		return nil, err
	}
	prep.Jobs = []model.Job{job}

	if err := writeRunMetadata(prep, cfg); err != nil {
		return nil, err
	}
	return prep, nil
}

func prepareJobs(
	ctx context.Context,
	cfg *model.RuntimeConfig,
	groups map[string][]model.IssueEntry,
	runArtifacts model.RunArtifacts,
	manager model.RuntimeManager,
	agentExecution *reusableagents.ExecutionContext,
) ([]model.Job, error) {
	effectiveBatchSize := cfg.BatchSize
	if cfg.Mode == model.ExecutionModePRDTasks {
		effectiveBatchSize = 1
	}
	if effectiveBatchSize <= 0 {
		effectiveBatchSize = 1
	}

	collected := flattenBatchIssues(groups, cfg.Mode)
	batches := createIssueBatches(collected, effectiveBatchSize)
	if len(batches) == 0 {
		return nil, errors.New("no batches created for prompt preparation")
	}

	jobs := make([]model.Job, 0, len(batches))
	for idx, batchIssues := range batches {
		job, err := buildBatchJob(ctx, cfg, runArtifacts, manager, idx, batchIssues, agentExecution)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	if len(jobs) == 0 {
		return nil, errors.New("no jobs finalized")
	}
	return jobs, nil
}

func ensureWorkflowRuntimesAvailable(ctx context.Context, cfg *model.RuntimeConfig, jobs []model.Job) error {
	if cfg == nil || cfg.DryRun {
		return nil
	}

	checked := make(map[string]struct{}, len(jobs))
	for idx := range jobs {
		job := &jobs[idx]
		ide := strings.TrimSpace(job.IDE)
		if ide == "" {
			continue
		}
		if _, ok := checked[ide]; ok {
			continue
		}
		runtimeCfg := cfg.Clone()
		runtimeCfg.IDE = ide
		runtimeCfg.Model = job.Model
		runtimeCfg.ReasoningEffort = job.ReasoningEffort
		runtimeCfg.TaskRuntimeRules = nil
		if err := agent.EnsureAvailable(ctx, runtimeCfg); err != nil {
			return fmt.Errorf("ensure runtime %q for job %q: %w", ide, job.SafeName, err)
		}
		checked[ide] = struct{}{}
	}
	return nil
}

func buildBatchJob(
	ctx context.Context,
	cfg *model.RuntimeConfig,
	runArtifacts model.RunArtifacts,
	manager model.RuntimeManager,
	batchIdx int,
	batchIssues []model.IssueEntry,
	agentExecution *reusableagents.ExecutionContext,
) (model.Job, error) {
	batchGroups, batchFiles := groupIssuesByCodeFile(batchIssues)
	safeName := determineBatchName(batchIdx, batchFiles, cfg.Mode)
	var (
		taskData model.TaskEntry
		err      error
	)
	params := prompt.BatchParams{
		Name:        cfg.Name,
		Round:       cfg.Round,
		Provider:    cfg.Provider,
		PR:          cfg.PR,
		ReviewsDir:  cfg.ReviewsDir,
		BatchGroups: batchGroups,
		AutoCommit:  cfg.AutoCommit,
		Mode:        cfg.Mode,
		Context:     ctx,
		RunID:       runArtifacts.RunID,
		JobID:       safeName,
		RuntimeMgr:  manager,
	}
	taskData, err = prepareBatchTaskContext(cfg, batchIssues, &params)
	if err != nil {
		return model.Job{}, err
	}
	jobRuntime, err := resolveBatchJobRuntime(
		ctx,
		cfg,
		batchIssues,
		taskData,
		safeName,
		runArtifacts.RunID,
		manager,
	)
	if err != nil {
		return model.Job{}, err
	}
	generated, err := buildBatchGeneratedJobData(
		params,
		runArtifacts,
		safeName,
		jobRuntime,
		agentExecution,
	)
	if err != nil {
		return model.Job{}, err
	}
	return model.Job{
		CodeFiles:       batchFiles,
		Groups:          batchGroups,
		TaskTitle:       taskData.Title,
		TaskType:        taskData.TaskType,
		SafeName:        safeName,
		IDE:             jobRuntime.IDE,
		Model:           jobRuntime.Model,
		ReasoningEffort: jobRuntime.ReasoningEffort,
		Prompt:          []byte(generated.promptText),
		SystemPrompt:    generated.systemPrompt,
		MCPServers:      generated.mcpServers,
		OutPromptPath:   generated.outPromptPath,
		OutLog:          generated.outLog,
		ErrLog:          generated.errLog,
	}, nil
}

type batchGeneratedJobData struct {
	promptText    string
	systemPrompt  string
	mcpServers    []model.MCPServer
	outPromptPath string
	outLog        string
	errLog        string
}

func buildBatchGeneratedJobData(
	params prompt.BatchParams,
	runArtifacts model.RunArtifacts,
	safeName string,
	jobRuntime *model.RuntimeConfig,
	agentExecution *reusableagents.ExecutionContext,
) (batchGeneratedJobData, error) {
	promptText, err := prompt.Build(params)
	if err != nil {
		return batchGeneratedJobData{}, err
	}
	systemPrompt, err := prompt.BuildSystemPromptAddendum(params)
	if err != nil {
		return batchGeneratedJobData{}, err
	}
	if agentExecution != nil {
		systemPrompt = agentExecution.SystemPrompt(systemPrompt)
	}
	mcpServers, err := reusableagents.BuildSessionMCPServers(
		agentExecution,
		reusableagents.SessionMCPContext{
			RunID:               runArtifacts.RunID,
			EffectiveAccessMode: jobRuntime.AccessMode,
			BaseRuntime:         jobRuntime,
		},
	)
	if err != nil {
		return batchGeneratedJobData{}, fmt.Errorf("build reusable-agent MCP servers: %w", err)
	}
	outPromptPath, outLog, errLog, err := writeBatchArtifacts(runArtifacts, safeName, promptText)
	if err != nil {
		return batchGeneratedJobData{}, err
	}
	return batchGeneratedJobData{
		promptText:    promptText,
		systemPrompt:  systemPrompt,
		mcpServers:    mcpServers,
		outPromptPath: outPromptPath,
		outLog:        outLog,
		errLog:        errLog,
	}, nil
}

func resolveBatchJobRuntime(
	ctx context.Context,
	cfg *model.RuntimeConfig,
	batchIssues []model.IssueEntry,
	taskData model.TaskEntry,
	safeName string,
	runID string,
	manager model.RuntimeManager,
) (*model.RuntimeConfig, error) {
	target := resolveTaskRuntimeTarget(cfg, batchIssues, taskData, safeName)
	jobRuntime := cfg.RuntimeForTask(target)
	jobRuntime.ApplyDefaults()
	if cfg != nil && cfg.Mode == model.ExecutionModePRDTasks {
		var err error
		jobRuntime, err = dispatchPlanPreResolveTaskRuntime(
			ctx,
			manager,
			runID,
			model.TaskRuntimeTask{
				ID:       target.ID,
				SafeName: safeName,
				Title:    taskData.Title,
				Type:     target.Type,
			},
			jobRuntime,
		)
		if err != nil {
			return nil, fmt.Errorf("resolve runtime for task %q: %w", target.ID, err)
		}
		jobRuntime.ApplyDefaults()
	}
	if err := validateResolvedJobRuntime(jobRuntime); err != nil {
		return nil, fmt.Errorf("resolve runtime for task %q: %w", target.ID, err)
	}
	return jobRuntime, nil
}

func dispatchPlanPreResolveTaskRuntime(
	ctx context.Context,
	manager model.RuntimeManager,
	runID string,
	task model.TaskRuntimeTask,
	runtimeCfg *model.RuntimeConfig,
) (*model.RuntimeConfig, error) {
	payload, err := model.DispatchMutableHook(
		ctx,
		manager,
		"plan.pre_resolve_task_runtime",
		planPreResolveTaskRuntimePayload{
			RunID:   runID,
			Task:    task,
			Runtime: model.TaskRuntimeFromConfig(runtimeCfg),
		},
	)
	if err != nil {
		return nil, err
	}
	updated := runtimeCfg.Clone()
	model.ApplyTaskRuntime(updated, payload.Runtime)
	return updated, nil
}

func resolveTaskRuntimeTarget(
	cfg *model.RuntimeConfig,
	batchIssues []model.IssueEntry,
	taskData model.TaskEntry,
	safeName string,
) model.TaskRuntimeTarget {
	target := model.TaskRuntimeTarget{
		ID:   safeName,
		Type: taskData.TaskType,
	}
	if cfg != nil && cfg.Mode != model.ExecutionModePRDTasks {
		target.Type = ""
		return target
	}
	if len(batchIssues) > 0 {
		target.ID = batchIssues[0].CodeFile
	}
	return target
}

func validateResolvedJobRuntime(cfg *model.RuntimeConfig) error {
	if cfg == nil {
		return agent.ErrRuntimeConfigNil
	}
	check := cfg.Clone()
	check.RunID = ""
	if check.Mode == model.ExecutionModePRDTasks {
		check.BatchSize = 1
	}
	return agent.ValidateRuntimeConfig(check)
}

func validatePreparedJobRuntimeMutation(before []model.Job, after []model.Job) error {
	beforeBySafeName := make(map[string]model.Job, len(before))
	beforeByCodeFiles := make(map[string]model.Job, len(before))
	beforeByPromptPath := make(map[string]model.Job, len(before))
	for idx := range before {
		job := before[idx]
		if key := strings.TrimSpace(job.SafeName); key != "" {
			beforeBySafeName[key] = job
		}
		if key := preparedJobCodeFilesKey(job); key != "" {
			beforeByCodeFiles[key] = job
		}
		if key := strings.TrimSpace(job.OutPromptPath); key != "" {
			beforeByPromptPath[key] = job
		}
	}

	for idx := range after {
		updated := after[idx]
		original, ok := findPreparedJobForRuntimeGuard(
			beforeBySafeName,
			beforeByCodeFiles,
			beforeByPromptPath,
			updated,
		)
		if ok && jobRuntimeChanged(original, updated) {
			return fmt.Errorf(
				"plan.post_prepare_jobs cannot mutate job runtime after task runtime resolution",
			)
		}
	}
	return nil
}

func jobRuntimeChanged(before model.Job, after model.Job) bool {
	return strings.TrimSpace(before.IDE) != strings.TrimSpace(after.IDE) ||
		strings.TrimSpace(before.Model) != strings.TrimSpace(after.Model) ||
		strings.TrimSpace(before.ReasoningEffort) != strings.TrimSpace(after.ReasoningEffort)
}

func findPreparedJobForRuntimeGuard(
	bySafeName map[string]model.Job,
	byCodeFiles map[string]model.Job,
	byPromptPath map[string]model.Job,
	job model.Job,
) (model.Job, bool) {
	if original, ok := bySafeName[strings.TrimSpace(job.SafeName)]; ok {
		return original, true
	}
	if original, ok := byCodeFiles[preparedJobCodeFilesKey(job)]; ok {
		return original, true
	}
	if original, ok := byPromptPath[strings.TrimSpace(job.OutPromptPath)]; ok {
		return original, true
	}
	return model.Job{}, false
}

func preparedJobCodeFilesKey(job model.Job) string {
	if len(job.CodeFiles) == 0 {
		return ""
	}
	return strings.Join(job.CodeFiles, "\x00")
}

func clonePreparedJobsForRuntimeGuard(src []model.Job) []model.Job {
	if len(src) == 0 {
		return nil
	}
	cloned := make([]model.Job, 0, len(src))
	for idx := range src {
		job := src[idx]
		cloned = append(cloned, model.Job{
			CodeFiles:       append([]string(nil), job.CodeFiles...),
			SafeName:        job.SafeName,
			IDE:             job.IDE,
			Model:           job.Model,
			ReasoningEffort: job.ReasoningEffort,
			OutPromptPath:   job.OutPromptPath,
		})
	}
	return cloned
}

func prepareBatchTaskContext(
	cfg *model.RuntimeConfig,
	batchIssues []model.IssueEntry,
	params *prompt.BatchParams,
) (model.TaskEntry, error) {
	if cfg.Mode != model.ExecutionModePRDTasks {
		return model.TaskEntry{}, nil
	}
	if len(batchIssues) == 0 {
		return model.TaskEntry{}, errors.New("prepare prd job: missing task issue")
	}

	taskData, err := tasks.ParseTaskFile(batchIssues[0].Content)
	if err != nil {
		return model.TaskEntry{}, tasks.WrapParseError(batchIssues[0].AbsPath, err)
	}
	memoryCtx, err := memory.Prepare(cfg.TasksDir, batchIssues[0].Name)
	if err != nil {
		return model.TaskEntry{}, fmt.Errorf("prepare memory for %s: %w", batchIssues[0].AbsPath, err)
	}
	params.Memory = &prompt.WorkflowMemoryContext{
		Directory:               memoryCtx.Directory,
		WorkflowPath:            memoryCtx.Workflow.Path,
		TaskPath:                memoryCtx.Task.Path,
		WorkflowNeedsCompaction: memoryCtx.Workflow.NeedsCompaction,
		TaskNeedsCompaction:     memoryCtx.Task.NeedsCompaction,
	}
	return taskData, nil
}

func buildExecJob(
	runArtifacts model.RunArtifacts,
	promptText string,
	agentExecution *reusableagents.ExecutionContext,
	cfg *model.RuntimeConfig,
) (model.Job, error) {
	const safeName = "exec"

	systemPrompt := ""
	if agentExecution != nil {
		systemPrompt = agentExecution.SystemPrompt("")
	}
	mcpServers, err := reusableagents.BuildSessionMCPServers(
		agentExecution,
		reusableagents.SessionMCPContext{
			RunID:               runArtifacts.RunID,
			EffectiveAccessMode: cfg.AccessMode,
			BaseRuntime:         cfg,
		},
	)
	if err != nil {
		return model.Job{}, fmt.Errorf("build reusable-agent MCP servers: %w", err)
	}

	outPromptPath, outLog, errLog, err := writeBatchArtifacts(runArtifacts, safeName, promptText)
	if err != nil {
		return model.Job{}, err
	}

	return model.Job{
		CodeFiles: []string{safeName},
		Groups: map[string][]model.IssueEntry{
			safeName: {{
				Name:     safeName,
				Content:  promptText,
				CodeFile: safeName,
			}},
		},
		SafeName:        safeName,
		IDE:             cfg.IDE,
		Model:           cfg.Model,
		ReasoningEffort: cfg.ReasoningEffort,
		Prompt:          []byte(promptText),
		SystemPrompt:    systemPrompt,
		MCPServers:      mcpServers,
		OutPromptPath:   outPromptPath,
		OutLog:          outLog,
		ErrLog:          errLog,
	}, nil
}

func determineBatchName(batchIdx int, batchFiles []string, mode model.ExecutionMode) string {
	if mode == model.ExecutionModePRDTasks {
		if len(batchFiles) == 1 {
			return prompt.SafeFileName(batchFiles[0])
		}
		return fmt.Sprintf("task_%03d", batchIdx+1)
	}
	if len(batchFiles) == 1 {
		filename := batchFiles[0]
		if strings.HasPrefix(filename, "__unknown__") {
			filename = model.UnknownFileName
		}
		return prompt.SafeFileName(filename)
	}
	return fmt.Sprintf("batch_%03d", batchIdx+1)
}

func writeBatchArtifacts(runArtifacts model.RunArtifacts, safeName, promptText string) (string, string, string, error) {
	jobArtifacts := runArtifacts.JobArtifacts(safeName)
	outPromptPath := jobArtifacts.PromptPath
	if err := os.WriteFile(outPromptPath, []byte(promptText), 0o600); err != nil {
		return "", "", "", fmt.Errorf("write prompt: %w", err)
	}
	outLog := jobArtifacts.OutLogPath
	errLog := jobArtifacts.ErrLogPath
	for _, logPath := range []string{outLog, errLog} {
		file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return "", "", "", fmt.Errorf("create log artifact %s: %w", logPath, err)
		}
		if closeErr := file.Close(); closeErr != nil {
			return "", "", "", fmt.Errorf("close log artifact %s: %w", logPath, closeErr)
		}
	}
	return outPromptPath, outLog, errLog, nil
}

type runMetadata struct {
	RunID        string    `json:"run_id"`
	Mode         string    `json:"mode"`
	IDE          string    `json:"ide"`
	Model        string    `json:"model"`
	OutputFormat string    `json:"output_format"`
	PromptSource string    `json:"prompt_source,omitempty"`
	PromptFile   string    `json:"prompt_file,omitempty"`
	ArtifactsDir string    `json:"artifacts_dir"`
	JobsDir      string    `json:"jobs_dir"`
	ResultPath   string    `json:"result_path"`
	JobCount     int       `json:"job_count"`
	CreatedAt    time.Time `json:"created_at"`
}

func writeRunMetadata(prep *model.SolvePreparation, cfg *model.RuntimeConfig) error {
	if prep == nil {
		return errors.New("missing preparation for run metadata")
	}
	payload, err := json.MarshalIndent(
		runMetadata{
			RunID:        prep.RunArtifacts.RunID,
			Mode:         string(cfg.Mode),
			IDE:          cfg.IDE,
			Model:        cfg.Model,
			OutputFormat: string(cfg.OutputFormat),
			PromptSource: promptSourceForConfig(cfg),
			PromptFile:   strings.TrimSpace(cfg.PromptFile),
			ArtifactsDir: prep.RunArtifacts.RunDir,
			JobsDir:      prep.RunArtifacts.JobsDir,
			ResultPath:   prep.RunArtifacts.ResultPath,
			JobCount:     len(prep.Jobs),
			CreatedAt:    time.Now().UTC(),
		},
		"",
		"  ",
	)
	if err != nil {
		return fmt.Errorf("marshal run metadata: %w", err)
	}
	if err := os.WriteFile(prep.RunArtifacts.RunMetaPath, payload, 0o600); err != nil {
		return fmt.Errorf("write run metadata: %w", err)
	}
	return nil
}

func promptSourceForConfig(cfg *model.RuntimeConfig) string {
	switch {
	case strings.TrimSpace(cfg.PromptFile) != "":
		return "file"
	case cfg.ReadPromptStdin:
		return "stdin"
	case strings.TrimSpace(cfg.PromptText) != "":
		return "positional"
	default:
		return ""
	}
}

func createIssueBatches(allIssues []model.IssueEntry, batchSize int) [][]model.IssueEntry {
	batches := make([][]model.IssueEntry, 0)
	for i := 0; i < len(allIssues); i += batchSize {
		end := i + batchSize
		if end > len(allIssues) {
			end = len(allIssues)
		}
		batches = append(batches, allIssues[i:end])
	}
	return batches
}

func flattenBatchIssues(
	groups map[string][]model.IssueEntry,
	mode model.ExecutionMode,
) []model.IssueEntry {
	if len(groups) == 0 {
		return nil
	}

	normalized := make(map[string][]model.IssueEntry, len(groups))
	for codeFile, entries := range groups {
		updated := make([]model.IssueEntry, len(entries))
		for idx, entry := range entries {
			entry.CodeFile = codeFile
			updated[idx] = entry
		}
		normalized[codeFile] = updated
	}

	return prompt.FlattenAndSortIssues(normalized, mode)
}

func groupIssuesByCodeFile(issues []model.IssueEntry) (map[string][]model.IssueEntry, []string) {
	batchGroups := make(map[string][]model.IssueEntry)
	for _, issue := range issues {
		batchGroups[issue.CodeFile] = append(batchGroups[issue.CodeFile], issue)
	}
	batchFiles := make([]string, 0, len(batchGroups))
	for codeFile := range batchGroups {
		batchFiles = append(batchFiles, codeFile)
	}
	sort.Strings(batchFiles)
	return batchGroups, batchFiles
}

type planPreDiscoverPayload struct {
	RunID        string              `json:"run_id"`
	Workflow     string              `json:"workflow"`
	Mode         model.ExecutionMode `json:"mode"`
	ExtraSources []string            `json:"extra_sources,omitempty"`
}

type planPostDiscoverPayload struct {
	RunID    string             `json:"run_id"`
	Workflow string             `json:"workflow"`
	Entries  []model.IssueEntry `json:"entries,omitempty"`
}

type planEntriesPayload struct {
	RunID   string             `json:"run_id"`
	Entries []model.IssueEntry `json:"entries,omitempty"`
}

type planGroupsPayload struct {
	RunID  string                        `json:"run_id"`
	Groups map[string][]model.IssueEntry `json:"groups,omitempty"`
}

type planPreResolveTaskRuntimePayload struct {
	RunID   string                `json:"run_id"`
	Task    model.TaskRuntimeTask `json:"task"`
	Runtime model.TaskRuntime     `json:"runtime"`
}

type planJobsPayload struct {
	RunID string      `json:"run_id"`
	Jobs  []model.Job `json:"jobs,omitempty"`
}

type reviewPreFetchPayload struct {
	RunID       string            `json:"run_id"`
	PR          string            `json:"pr"`
	Provider    string            `json:"provider"`
	FetchConfig model.FetchConfig `json:"fetch_config"`
}

type reviewPostFetchPayload struct {
	RunID  string             `json:"run_id"`
	PR     string             `json:"pr"`
	Issues []model.IssueEntry `json:"issues,omitempty"`
}

type reviewPreBatchPayload struct {
	RunID  string                        `json:"run_id"`
	PR     string                        `json:"pr"`
	Groups map[string][]model.IssueEntry `json:"groups,omitempty"`
}

func resolveExtraSourceBaseDir(inputDir string, workspaceRoot string) string {
	if strings.TrimSpace(workspaceRoot) != "" {
		return workspaceRoot
	}
	return inputDir
}

func readExtraIssueEntries(
	baseDir string,
	cfg *model.RuntimeConfig,
	sources []string,
) ([]model.IssueEntry, error) {
	entries := make([]model.IssueEntry, 0)
	for _, source := range sources {
		resolvedSource, err := resolveExtraSourcePath(baseDir, source)
		if err != nil {
			return nil, fmt.Errorf("resolve extra issue source %q: %w", source, err)
		}
		items, err := readIssueEntriesFromSource(resolvedSource, cfg.Mode, cfg.IncludeCompleted)
		if err != nil {
			return nil, fmt.Errorf("read extra issue source %q: %w", source, err)
		}
		entries = append(entries, items...)
	}
	return entries, nil
}

func resolveExtraSourcePath(baseDir string, source string) (string, error) {
	trimmed := strings.TrimSpace(source)
	if trimmed == "" {
		return "", errors.New("empty extra source")
	}
	if filepath.IsAbs(trimmed) {
		return filepath.Clean(trimmed), nil
	}
	if strings.TrimSpace(baseDir) == "" {
		return filepath.Abs(trimmed)
	}
	return filepath.Abs(filepath.Join(baseDir, trimmed))
}

func readIssueEntriesFromSource(
	resolvedSource string,
	mode model.ExecutionMode,
	includeCompleted bool,
) ([]model.IssueEntry, error) {
	info, err := os.Stat(resolvedSource)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return readIssueEntries(resolvedSource, mode, includeCompleted)
	}

	parent := filepath.Dir(resolvedSource)
	entries, err := readIssueEntries(parent, mode, includeCompleted)
	if err != nil {
		return nil, err
	}
	filtered := make([]model.IssueEntry, 0, 1)
	for _, entry := range entries {
		if filepath.Clean(entry.AbsPath) != filepath.Clean(resolvedSource) {
			continue
		}
		filtered = append(filtered, entry)
	}
	if len(filtered) == 0 {
		return nil, fmt.Errorf("no issue entries found for %q", resolvedSource)
	}
	return filtered, nil
}

func dedupeIssueEntries(entries []model.IssueEntry) []model.IssueEntry {
	if len(entries) <= 1 {
		return entries
	}

	seen := make(map[string]struct{}, len(entries))
	deduped := make([]model.IssueEntry, 0, len(entries))
	for _, entry := range entries {
		key := strings.TrimSpace(entry.AbsPath)
		if key == "" {
			key = entry.Name + "::" + entry.CodeFile
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, entry)
	}
	return deduped
}
