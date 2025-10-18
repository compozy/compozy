package directexec

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	providermetrics "github.com/compozy/compozy/engine/llm/provider/metrics"
	"github.com/compozy/compozy/engine/llm/usage"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/runtime/toolenv"
	"github.com/compozy/compozy/engine/runtime/toolenvstate"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/engine/task2"
	task2core "github.com/compozy/compozy/engine/task2/core"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/tplengine"
)

type ExecMetadata struct {
	Component  core.ComponentType
	AgentID    string
	ActionID   string
	TaskID     string
	WorkflowID string
}

type DirectExecutor interface {
	ExecuteSync(
		ctx context.Context,
		cfg *task.Config,
		meta *ExecMetadata,
		timeout time.Duration,
	) (*core.Output, core.ID, error)
	ExecuteAsync(ctx context.Context, cfg *task.Config, meta *ExecMetadata) (core.ID, error)
}

type directExecutor struct {
	taskRepo           task.Repository
	workflowRepo       workflow.Repository
	toolEnvironment    toolenv.Environment
	appState           *appstate.State
	usageMetrics       usage.Metrics
	providerMetrics    providermetrics.Recorder
	resourceStore      resources.ResourceStore
	projectConfig      *project.Config
	memoryManager      memcore.ManagerInterface
	templateEngine     *tplengine.TemplateEngine
	configOrchestrator *task2.ConfigOrchestrator
	runtimeFactory     runtime.Factory
	runtimeOnce        sync.Once
	runtime            runtime.Runtime
	runtimeErr         error
}

type execResult struct {
	output *core.Output
	err    error
}

type executionPlan struct {
	config         *task.Config
	workflowConfig *workflow.Config
	meta           *ExecMetadata
}

func cloneTaskInput(input *core.Input) (*core.Input, error) {
	if input == nil {
		return nil, nil
	}
	cloned, err := core.DeepCopy(input)
	if err != nil {
		return nil, err
	}
	return cloned, nil
}

func restoreDirectInput(cfg *task.Config, original *core.Input) {
	if cfg == nil || original == nil {
		return
	}
	if cfg.With == nil {
		cfg.With = original
		return
	}
	mergedMap := core.CopyMaps(*original, *cfg.With)
	merged := core.Input(mergedMap)
	cfg.With = &merged
}

func (d *directExecutor) normalizeTaskConfig(
	wfState *workflow.State,
	wfConfig *workflow.Config,
	cfg *task.Config,
) error {
	if err := d.configOrchestrator.NormalizeTask(wfState, wfConfig, cfg); err != nil {
		return fmt.Errorf("failed to normalize task %s: %w", cfg.ID, err)
	}
	return nil
}

func (d *directExecutor) normalizeAgentComponent(
	wfState *workflow.State,
	wfConfig *workflow.Config,
	cfg *task.Config,
	allTaskConfigs map[string]*task.Config,
) error {
	if cfg.Agent == nil {
		return nil
	}
	if err := d.configOrchestrator.NormalizeAgentComponent(
		wfState,
		wfConfig,
		cfg,
		cfg.Agent,
		allTaskConfigs,
	); err != nil {
		return fmt.Errorf("failed to normalize agent component for %s: %w", cfg.ID, err)
	}
	return nil
}

func (d *directExecutor) normalizeToolComponent(
	wfState *workflow.State,
	wfConfig *workflow.Config,
	cfg *task.Config,
	allTaskConfigs map[string]*task.Config,
) error {
	if cfg.Tool == nil {
		return nil
	}
	if err := d.configOrchestrator.NormalizeToolComponent(
		wfState,
		wfConfig,
		cfg,
		cfg.Tool,
		allTaskConfigs,
	); err != nil {
		return fmt.Errorf("failed to normalize tool component for %s: %w", cfg.ID, err)
	}
	return nil
}

func (d *directExecutor) initializeWorkflowState(
	execID core.ID,
	preparedCfg *task.Config,
	meta *ExecMetadata,
) (*workflow.Config, *workflow.State) {
	workflowID := meta.WorkflowID
	if workflowID == "" {
		workflowID = preparedCfg.ID
	}
	meta.WorkflowID = workflowID
	wfConfig := &workflow.Config{ID: workflowID}
	wfConfig.Tasks = []task.Config{*preparedCfg}
	wfState := workflow.NewState(workflowID, execID, preparedCfg.With)
	return wfConfig, wfState
}

func (d *directExecutor) normalizeComponents(
	wfState *workflow.State,
	wfConfig *workflow.Config,
	preparedCfg *task.Config,
	directInputCopy *core.Input,
) error {
	if err := d.normalizeTaskConfig(wfState, wfConfig, preparedCfg); err != nil {
		return err
	}
	restoreDirectInput(preparedCfg, directInputCopy)
	taskConfigs := task2.BuildTaskConfigsMap(wfConfig.Tasks)
	taskConfigs[preparedCfg.ID] = preparedCfg
	if err := d.normalizeAgentComponent(wfState, wfConfig, preparedCfg, taskConfigs); err != nil {
		return err
	}
	if err := d.normalizeToolComponent(wfState, wfConfig, preparedCfg, taskConfigs); err != nil {
		return err
	}
	return nil
}

func (d *directExecutor) prepareExecutionPlan(
	_ context.Context,
	cfg *task.Config,
	meta *ExecMetadata,
	execID core.ID,
) (*executionPlan, error) {
	if cfg == nil {
		return nil, fmt.Errorf("task config is required")
	}
	if meta == nil {
		meta = &ExecMetadata{}
	}
	if d.configOrchestrator == nil {
		return nil, fmt.Errorf("task configuration orchestrator not initialized")
	}
	cfgCopy, err := core.DeepCopy[*task.Config](cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to clone task config: %w", err)
	}
	preparedCfg := d.prepareTaskConfig(cfgCopy, execID, meta)
	directInputCopy, cloneErr := cloneTaskInput(preparedCfg.With)
	if cloneErr != nil {
		return nil, fmt.Errorf("failed to clone task input: %w", cloneErr)
	}
	wfConfig, wfState := d.initializeWorkflowState(execID, preparedCfg, meta)
	if err := d.normalizeComponents(wfState, wfConfig, preparedCfg, directInputCopy); err != nil {
		return nil, err
	}
	wfConfig.Tasks = []task.Config{*preparedCfg}
	wfState.Input = preparedCfg.With
	return &executionPlan{
		config:         preparedCfg,
		workflowConfig: wfConfig,
		meta:           meta,
	}, nil
}

func setupConfigOrchestrator(
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
) (*task2.ConfigOrchestrator, *tplengine.TemplateEngine, error) {
	tplEng := tplengine.NewEngine(tplengine.FormatJSON)
	envMerger := task2core.NewEnvMerger()
	factory, err := task2.NewFactory(&task2.FactoryConfig{
		TemplateEngine: tplEng,
		EnvMerger:      envMerger,
		WorkflowRepo:   workflowRepo,
		TaskRepo:       taskRepo,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create task normalizer factory: %w", err)
	}
	orchestrator, err := task2.NewConfigOrchestrator(factory)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create task config orchestrator: %w", err)
	}
	return orchestrator, tplEng, nil
}

func NewDirectExecutor(
	state *appstate.State,
	taskRepo task.Repository,
	workflowRepo workflow.Repository,
) (DirectExecutor, error) {
	if state == nil {
		return nil, fmt.Errorf("app state is required")
	}
	if taskRepo == nil {
		return nil, fmt.Errorf("task repository is required")
	}
	projCfg := state.ProjectConfig
	if projCfg == nil {
		return nil, fmt.Errorf("project configuration not available")
	}
	if workflowRepo == nil && state.Store != nil {
		workflowRepo = state.Store.NewWorkflowRepo()
	}
	var usageMetrics usage.Metrics
	providerMetrics := providermetrics.Nop()
	if svc, ok := state.MonitoringService(); ok && svc != nil && svc.IsInitialized() {
		usageMetrics = svc.LLMUsageMetrics()
		providerMetrics = svc.LLMProviderMetrics()
	}
	orchestrator, tplEng, err := setupConfigOrchestrator(workflowRepo, taskRepo)
	if err != nil {
		return nil, err
	}
	var memManager memcore.ManagerInterface
	if state.Worker != nil {
		memManager = state.Worker.GetMemoryManager()
	}
	root := resolveProjectRoot(state)
	var resourceStore resources.ResourceStore
	if ext, ok := state.ResourceStore(); ok {
		if store, ok := ext.(resources.ResourceStore); ok {
			resourceStore = store
		}
	}
	toolEnvironment, _ := toolenvstate.Load(state)
	if toolEnvironment == nil {
		return nil, fmt.Errorf("tool environment unavailable: ensure agent runner initialized")
	}
	return &directExecutor{
		taskRepo:           taskRepo,
		workflowRepo:       workflowRepo,
		toolEnvironment:    toolEnvironment,
		appState:           state,
		usageMetrics:       usageMetrics,
		providerMetrics:    providerMetrics,
		resourceStore:      resourceStore,
		projectConfig:      projCfg,
		memoryManager:      memManager,
		templateEngine:     tplEng,
		configOrchestrator: orchestrator,
		runtimeFactory:     runtime.NewDefaultFactory(root),
	}, nil
}

func resolveProjectRoot(state *appstate.State) string {
	if state == nil || state.CWD == nil {
		return ""
	}
	return state.CWD.PathStr()
}

func (d *directExecutor) attachUsageCollector(ctx context.Context, state *task.State) context.Context {
	if state == nil {
		return ctx
	}
	collector := usage.NewCollector(d.usageMetrics, usage.Metadata{
		Component:      state.Component,
		WorkflowExecID: state.WorkflowExecID,
		TaskExecID:     state.TaskExecID,
		AgentID:        state.AgentID,
	})
	return usage.ContextWithCollector(ctx, collector)
}

func (d *directExecutor) persistUsageSummary(ctx context.Context, state *task.State, finalized *usage.Finalized) {
	if d == nil || state == nil || finalized == nil || finalized.Summary == nil {
		return
	}
	taskSummary := finalized.Summary.CloneWithSource(usage.SourceTask)
	if taskSummary == nil || len(taskSummary.Entries) == 0 {
		return
	}
	log := logger.FromContext(ctx)
	if err := d.taskRepo.MergeUsage(ctx, state.TaskExecID, taskSummary); err != nil {
		log.Warn(
			"Failed to merge task usage", "task_exec_id", state.TaskExecID.String(), "error", err,
		)
	}
	if d.workflowRepo != nil && !state.WorkflowExecID.IsZero() {
		workflowSummary := finalized.Summary.CloneWithSource(usage.SourceWorkflow)
		if err := d.workflowRepo.MergeUsage(ctx, state.WorkflowExecID, workflowSummary); err != nil {
			log.Warn(
				"Failed to merge workflow usage", "workflow_exec_id", state.WorkflowExecID.String(), "error", err,
			)
		}
	}
}

func (d *directExecutor) ExecuteSync(
	ctx context.Context,
	cfg *task.Config,
	meta *ExecMetadata,
	timeout time.Duration,
) (*core.Output, core.ID, error) {
	var zeroID core.ID
	if ctx == nil {
		return nil, zeroID, fmt.Errorf("context is required")
	}
	if cfg == nil {
		return nil, zeroID, fmt.Errorf("task config is required")
	}
	if meta == nil {
		meta = &ExecMetadata{}
	}
	execID := core.MustNewID()
	plan, err := d.prepareExecutionPlan(ctx, cfg, meta, execID)
	if err != nil {
		return nil, zeroID, err
	}
	state, err := d.initExecutionState(ctx, plan, execID)
	if err != nil {
		return nil, zeroID, err
	}
	if _, err := d.ensureRuntime(ctx); err != nil {
		log := logger.FromContext(ctx)
		if errUp := d.updateState(ctx, state, func(s *task.State) {
			s.Status = core.StatusFailed
			s.Error = core.NewError(err, "DIRECT_EXECUTION_FAILED", nil)
			s.Output = nil
		}); errUp != nil {
			log.Error(
				"Failed to update execution state",
				"error", errUp,
				"task_id", state.TaskID,
				"exec_id", state.TaskExecID.String(),
			)
		}
		return nil, zeroID, err
	}
	resultCh := make(chan execResult, 1)
	execCtx, cancel := context.WithCancel(ctx)
	execCtx = d.attachUsageCollector(execCtx, state)
	go d.runExecution(execCtx, state, plan, resultCh)
	if timeout <= 0 {
		res := <-resultCh
		cancel()
		return res.output, execID, res.err
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case res := <-resultCh:
		cancel()
		return res.output, execID, res.err
	case <-timer.C:
		cancel()
		return nil, execID, context.DeadlineExceeded
	case <-ctx.Done():
		cancel()
		return nil, execID, ctx.Err()
	}
}

func (d *directExecutor) initExecutionState(
	ctx context.Context,
	plan *executionPlan,
	execID core.ID,
) (*task.State, error) {
	state := d.buildInitialState(execID, plan.config, plan.meta)
	now := time.Now().UTC()
	state.CreatedAt = now
	state.UpdatedAt = now
	if err := d.ensureWorkflowState(ctx, plan.config, plan.meta, state); err != nil {
		return nil, err
	}
	if err := d.taskRepo.UpsertState(ctx, state); err != nil {
		return nil, fmt.Errorf("failed to persist execution state: %w", err)
	}
	if err := d.updateState(ctx, state, func(s *task.State) {
		s.Status = core.StatusRunning
	}); err != nil {
		return nil, err
	}
	return state, nil
}

func (d *directExecutor) ExecuteAsync(ctx context.Context, cfg *task.Config, meta *ExecMetadata) (core.ID, error) {
	var zeroID core.ID
	if ctx == nil {
		return zeroID, fmt.Errorf("context is required")
	}
	if cfg == nil {
		return zeroID, fmt.Errorf("task config is required")
	}
	if meta == nil {
		meta = &ExecMetadata{}
	}
	execID := core.MustNewID()
	plan, err := d.prepareExecutionPlan(ctx, cfg, meta, execID)
	if err != nil {
		return zeroID, err
	}
	state, err := d.initExecutionState(ctx, plan, execID)
	if err != nil {
		return zeroID, err
	}
	if _, err := d.ensureRuntime(ctx); err != nil {
		log := logger.FromContext(ctx)
		if errUp := d.updateState(ctx, state, func(s *task.State) {
			s.Status = core.StatusFailed
			s.Error = core.NewError(err, "DIRECT_EXECUTION_FAILED", nil)
			s.Output = nil
		}); errUp != nil {
			log.Error(
				"Failed to update execution state",
				"error", errUp,
				"task_id", state.TaskID,
				"exec_id", state.TaskExecID.String(),
			)
		}
		return zeroID, err
	}
	bgCtx := context.WithoutCancel(ctx)
	bgCtx = d.attachUsageCollector(bgCtx, state)
	go d.runExecution(bgCtx, state, plan, nil)
	return execID, nil
}

func (d *directExecutor) runExecution(
	ctx context.Context,
	state *task.State,
	plan *executionPlan,
	resultCh chan<- execResult,
) {
	res := execResult{}
	if err := d.ensurePlanReady(plan); err != nil {
		res.err = err
		d.sendExecResult(resultCh, &res)
		return
	}
	defer d.sendExecResult(resultCh, &res)
	defer d.recoverExecution(ctx, state, &res)
	output, err := d.executeOnce(ctx, state, plan)
	if err != nil {
		res.err = err
		return
	}
	res.output = output
}

func (d *directExecutor) executeOnce(
	ctx context.Context,
	state *task.State,
	plan *executionPlan,
) (*core.Output, error) {
	if err := d.ensurePlanReady(plan); err != nil {
		return nil, err
	}
	rt, err := d.acquireRuntime(ctx, state)
	if err != nil {
		return nil, err
	}
	ucExec := uc.NewExecuteTask(
		rt,
		d.workflowRepo,
		d.memoryManager,
		d.templateEngine,
		nil,
		d.providerMetrics,
		d.toolEnvironment,
	)
	wfState := d.buildWorkflowState(plan.meta, state.WorkflowExecID, plan.config)
	input := &uc.ExecuteTaskInput{
		TaskConfig:     plan.config,
		WorkflowState:  wfState,
		WorkflowConfig: plan.workflowConfig,
		ProjectConfig:  d.projectConfig,
	}
	output, execErr := ucExec.Execute(ctx, input)
	if execErr != nil {
		d.markExecutionFailure(ctx, state, execErr)
		return nil, execErr
	}
	d.markExecutionSuccess(ctx, state, output)
	return output, nil
}

func (d *directExecutor) ensurePlanReady(plan *executionPlan) error {
	if plan == nil {
		return fmt.Errorf("execution plan not initialized")
	}
	if plan.meta == nil {
		plan.meta = &ExecMetadata{}
	}
	return nil
}

func (d *directExecutor) sendExecResult(ch chan<- execResult, res *execResult) {
	if ch == nil {
		return
	}
	select {
	case ch <- *res:
	default:
	}
}

func (d *directExecutor) recoverExecution(
	ctx context.Context,
	state *task.State,
	res *execResult,
) {
	if r := recover(); r != nil {
		err := fmt.Errorf("direct execution panic: %v", r)
		log := logger.FromContext(ctx)
		log.Error("Direct execution panicked", "error", err)
		if errUp := d.updateState(ctx, state, func(s *task.State) {
			s.Status = core.StatusFailed
			s.Error = core.NewError(err, "DIRECT_EXECUTION_FAILED", nil)
			s.Output = nil
		}); errUp != nil {
			log.Error(
				"Failed to update execution state",
				"error", errUp,
				"task_id", state.TaskID,
				"exec_id", state.TaskExecID.String(),
			)
		}
		if collector := usage.FromContext(ctx); collector != nil {
			finalized, finalizeErr := collector.Finalize(ctx, core.StatusFailed)
			if finalizeErr != nil {
				log.Warn("Failed to aggregate usage after execution panic", "error", finalizeErr)
			} else {
				d.persistUsageSummary(ctx, state, finalized)
			}
		}
		res.err = err
	}
}

func (d *directExecutor) acquireRuntime(
	ctx context.Context,
	state *task.State,
) (runtime.Runtime, error) {
	rt, err := d.ensureRuntime(ctx)
	if err == nil {
		return rt, nil
	}
	log := logger.FromContext(ctx)
	if upErr := d.updateState(ctx, state, func(s *task.State) {
		s.Status = core.StatusFailed
		s.Error = core.NewError(err, "DIRECT_EXECUTION_FAILED", nil)
		s.Output = nil
	}); upErr != nil {
		log.Error("Failed to update execution state", "error", upErr)
	}
	return nil, err
}

func (d *directExecutor) markExecutionFailure(
	ctx context.Context,
	state *task.State,
	err error,
) {
	log := logger.FromContext(ctx)
	if upErr := d.updateState(ctx, state, func(s *task.State) {
		s.Status = core.StatusFailed
		s.Error = core.NewError(err, "DIRECT_EXECUTION_FAILED", map[string]any{
			"task_id": s.TaskID,
		})
		s.Output = nil
	}); upErr != nil {
		log.Error("Failed to update execution state", "error", upErr)
	}
	if collector := usage.FromContext(ctx); collector != nil {
		finalized, finalizeErr := collector.Finalize(ctx, core.StatusFailed)
		if finalizeErr != nil {
			log.Warn("Failed to aggregate usage after execution failure", "error", finalizeErr)
		} else {
			d.persistUsageSummary(ctx, state, finalized)
		}
	}
}

func (d *directExecutor) markExecutionSuccess(
	ctx context.Context,
	state *task.State,
	output *core.Output,
) {
	log := logger.FromContext(ctx)
	if upErr := d.updateState(ctx, state, func(s *task.State) {
		s.Status = core.StatusSuccess
		s.Error = nil
		s.Output = output
	}); upErr != nil {
		log.Error("Failed to update execution state", "error", upErr)
	}
	if collector := usage.FromContext(ctx); collector != nil {
		finalized, err := collector.Finalize(ctx, core.StatusSuccess)
		if err != nil {
			log.Warn("Failed to aggregate usage after execution success", "error", err)
		} else {
			d.persistUsageSummary(ctx, state, finalized)
		}
	}
}

func (d *directExecutor) ensureRuntime(ctx context.Context) (runtime.Runtime, error) {
	d.runtimeOnce.Do(func() {
		cfg := config.FromContext(ctx)
		if cfg == nil {
			d.runtimeErr = fmt.Errorf("runtime configuration missing from context")
			return
		}
		rt, err := d.runtimeFactory.CreateRuntimeFromAppConfig(ctx, &cfg.Runtime)
		if err != nil {
			d.runtimeErr = fmt.Errorf("failed to initialize runtime: %w", err)
			return
		}
		d.runtime = rt
	})
	return d.runtime, d.runtimeErr
}

func (d *directExecutor) buildInitialState(execID core.ID, cfg *task.Config, meta *ExecMetadata) *task.State {
	if meta == nil {
		meta = &ExecMetadata{}
	}
	component := meta.Component
	if component == "" {
		component = d.deriveComponent(cfg)
	}
	workflowID := meta.WorkflowID
	if workflowID == "" {
		workflowID = cfg.ID
	}
	taskID := meta.TaskID
	if taskID == "" {
		taskID = cfg.ID
	}
	state := &task.State{
		Component:      component,
		Status:         core.StatusPending,
		TaskID:         taskID,
		TaskExecID:     execID,
		WorkflowID:     workflowID,
		WorkflowExecID: execID,
		ExecutionType:  task.ExecutionBasic,
	}
	if cfg != nil && cfg.With != nil {
		state.Input = cfg.With
	}
	if meta.AgentID != "" {
		state.AgentID = &meta.AgentID
	}
	if meta.ActionID != "" {
		state.ActionID = &meta.ActionID
	}
	return state
}

func (d *directExecutor) deriveComponent(cfg *task.Config) core.ComponentType {
	if cfg == nil {
		return core.ComponentTask
	}
	if cfg.Agent != nil {
		return core.ComponentAgent
	}
	if cfg.Tool != nil {
		return core.ComponentTool
	}
	return core.ComponentTask
}

func (d *directExecutor) prepareTaskConfig(cfg *task.Config, execID core.ID, meta *ExecMetadata) *task.Config {
	if cfg.ID == "" {
		cfg.ID = fmt.Sprintf("direct-%s", execID.String())
	}
	if cfg.With == nil {
		m := core.Input{}
		cfg.With = &m
	}
	if meta != nil && meta.AgentID != "" && cfg.Agent != nil {
		cfg.Agent.ID = meta.AgentID
	}
	return cfg
}

func (d *directExecutor) buildWorkflowState(meta *ExecMetadata, execID core.ID, cfg *task.Config) *workflow.State {
	if meta == nil {
		meta = &ExecMetadata{}
	}
	workflowID := meta.WorkflowID
	if workflowID == "" {
		workflowID = cfg.ID
	}
	input := cfg.With
	return workflow.NewState(workflowID, execID, input)
}

func (d *directExecutor) ensureWorkflowState(
	ctx context.Context,
	cfg *task.Config,
	meta *ExecMetadata,
	state *task.State,
) error {
	if d.workflowRepo == nil {
		return fmt.Errorf("workflow repository is required for direct execution")
	}
	if state == nil {
		return fmt.Errorf("task state is required for workflow persistence")
	}
	wfCfg := cfg
	if wfCfg == nil {
		wfCfg = &task.Config{}
	}
	wfState := d.buildWorkflowState(meta, state.WorkflowExecID, wfCfg)
	if err := d.workflowRepo.UpsertState(ctx, wfState); err != nil {
		return fmt.Errorf("failed to persist workflow state: %w", err)
	}
	return nil
}

func (d *directExecutor) updateState(ctx context.Context, state *task.State, mutate func(*task.State)) error {
	clone := *state
	mutate(&clone)
	clone.UpdatedAt = time.Now().UTC()
	if err := d.taskRepo.UpsertState(ctx, &clone); err != nil {
		return err
	}
	*state = clone
	return nil
}
