package tkrouter

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/infra/server/router"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/runtime"
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
	cloned, err := core.DeepCopy[*core.Input](input)
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
	for k, v := range *original {
		(*cfg.With)[k] = v
	}
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
	workflowID := meta.WorkflowID
	if workflowID == "" {
		workflowID = preparedCfg.ID
	}
	meta.WorkflowID = workflowID
	wfConfig := &workflow.Config{ID: workflowID}
	wfConfig.Tasks = []task.Config{*preparedCfg}
	wfState := workflow.NewState(workflowID, execID, preparedCfg.With)
	if err := d.normalizeTaskConfig(wfState, wfConfig, preparedCfg); err != nil {
		return nil, err
	}
	restoreDirectInput(preparedCfg, directInputCopy)
	taskConfigs := task2.BuildTaskConfigsMap(wfConfig.Tasks)
	taskConfigs[preparedCfg.ID] = preparedCfg
	if err := d.normalizeAgentComponent(wfState, wfConfig, preparedCfg, taskConfigs); err != nil {
		return nil, err
	}
	if err := d.normalizeToolComponent(wfState, wfConfig, preparedCfg, taskConfigs); err != nil {
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
	tplEng := tplengine.NewEngine(tplengine.FormatJSON)
	envMerger := task2core.NewEnvMerger()
	factory, err := task2.NewFactory(&task2.FactoryConfig{
		TemplateEngine: tplEng,
		EnvMerger:      envMerger,
		WorkflowRepo:   workflowRepo,
		TaskRepo:       taskRepo,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create task normalizer factory: %w", err)
	}
	orchestrator, err := task2.NewConfigOrchestrator(factory)
	if err != nil {
		return nil, fmt.Errorf("failed to create task config orchestrator: %w", err)
	}
	var memManager memcore.ManagerInterface
	if state.Worker != nil {
		memManager = state.Worker.GetMemoryManager()
	}
	root, ok := router.ProjectRootPath(state)
	if !ok {
		root = ""
	}
	return &directExecutor{
		taskRepo:           taskRepo,
		workflowRepo:       workflowRepo,
		projectConfig:      projCfg,
		memoryManager:      memManager,
		templateEngine:     tplEng,
		configOrchestrator: orchestrator,
		runtimeFactory:     runtime.NewDefaultFactory(root),
	}, nil
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
	state := d.buildInitialState(execID, plan.config, plan.meta)
	now := time.Now().UTC()
	state.CreatedAt = now
	state.UpdatedAt = now
	if err := d.ensureWorkflowState(ctx, plan.config, plan.meta, state); err != nil {
		return nil, zeroID, err
	}
	if err := d.taskRepo.UpsertState(ctx, state); err != nil {
		return nil, zeroID, fmt.Errorf("failed to persist execution state: %w", err)
	}
	if err := d.updateState(ctx, state, func(s *task.State) {
		s.Status = core.StatusRunning
	}); err != nil {
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
	state := d.buildInitialState(execID, plan.config, plan.meta)
	now := time.Now().UTC()
	state.CreatedAt = now
	state.UpdatedAt = now
	if err := d.ensureWorkflowState(ctx, plan.config, plan.meta, state); err != nil {
		return zeroID, err
	}
	if err := d.taskRepo.UpsertState(ctx, state); err != nil {
		return zeroID, fmt.Errorf("failed to persist execution state: %w", err)
	}
	if err := d.updateState(ctx, state, func(s *task.State) {
		s.Status = core.StatusRunning
	}); err != nil {
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
	go d.runExecution(bgCtx, state, plan, nil)
	return execID, nil
}

func (d *directExecutor) runExecution(
	ctx context.Context,
	state *task.State,
	plan *executionPlan,
	resultCh chan<- execResult,
) {
	if plan == nil {
		res := execResult{err: fmt.Errorf("execution plan not initialized")}
		if resultCh != nil {
			select {
			case resultCh <- res:
			default:
			}
		}
		return
	}
	if plan.meta == nil {
		plan.meta = &ExecMetadata{}
	}
	log := logger.FromContext(ctx)
	res := execResult{}
	defer func() {
		if resultCh != nil {
			select {
			case resultCh <- res:
			default:
			}
		}
	}()
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("direct execution panic: %v", r)
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
			res.err = err
		}
	}()
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
	if plan == nil {
		return nil, fmt.Errorf("execution plan not initialized")
	}
	if plan.meta == nil {
		plan.meta = &ExecMetadata{}
	}
	rt, err := d.ensureRuntime(ctx)
	if err != nil {
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
	ucExec := uc.NewExecuteTask(rt, d.workflowRepo, d.memoryManager, d.templateEngine, nil)
	wfState := d.buildWorkflowState(plan.meta, state.WorkflowExecID, plan.config)
	input := &uc.ExecuteTaskInput{
		TaskConfig:     plan.config,
		WorkflowState:  wfState,
		WorkflowConfig: plan.workflowConfig,
		ProjectConfig:  d.projectConfig,
	}
	output, execErr := ucExec.Execute(ctx, input)
	log := logger.FromContext(ctx)
	if execErr != nil {
		if upErr := d.updateState(ctx, state, func(s *task.State) {
			s.Status = core.StatusFailed
			s.Error = core.NewError(execErr, "DIRECT_EXECUTION_FAILED", map[string]any{
				"task_id": s.TaskID,
			})
			s.Output = nil
		}); upErr != nil {
			log.Error("Failed to update execution state", "error", upErr)
		}
		return nil, execErr
	}
	if upErr := d.updateState(ctx, state, func(s *task.State) {
		s.Status = core.StatusSuccess
		s.Error = nil
		s.Output = output
	}); upErr != nil {
		log.Error("Failed to update execution state", "error", upErr)
	}
	return output, nil
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
