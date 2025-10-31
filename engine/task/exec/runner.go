package taskexec

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/agent"
	agentuc "github.com/compozy/compozy/engine/agent/uc"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/runtime/toolenv"
	"github.com/compozy/compozy/engine/task"
	taskrouter "github.com/compozy/compozy/engine/task/router"
	taskuc "github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/pkg/config"
)

const (
	defaultTaskTimeout = 60 * time.Second
)

var (
	ErrTaskIDRequired        = errors.New("task id is required")
	ErrStateRequired         = errors.New("application state is required")
	ErrRepositoryRequired    = errors.New("task repository is required")
	ErrResourceStoreRequired = errors.New("resource store is required")
	ErrProjectNameRequired   = errors.New("project name is required")
	ErrNegativeTimeout       = errors.New("timeout must be non-negative")
)

// Runner executes tasks in embedded mode synchronously using the DirectExecutor pipeline.
type Runner struct {
	state *appstate.State
	repo  task.Repository
	store resources.ResourceStore
}

// ExecuteRequest describes a task execution request resolved from tool input.
type ExecuteRequest struct {
	TaskID string
	With   core.Input
	// Timeout overrides the configured default when greater than zero.
	Timeout time.Duration
}

// ExecuteResult captures the outcome of a synchronous task execution.
type ExecuteResult struct {
	ExecID core.ID
	Output *core.Output
}

// PreparedExecution bundles the resolved configuration required for execution.
type PreparedExecution struct {
	Config   *task.Config
	Executor taskrouter.DirectExecutor
	Metadata taskrouter.ExecMetadata
	Timeout  time.Duration
}

// NewRunner constructs a Runner with the provided application state, task repository, and resource store.
func NewRunner(state *appstate.State, repo task.Repository, store resources.ResourceStore) *Runner {
	return &Runner{state: state, repo: repo, store: store}
}

// ExecuteTask satisfies toolenv.TaskExecutor.
func (r *Runner) ExecuteTask(ctx context.Context, req toolenv.TaskRequest) (*toolenv.TaskResult, error) {
	execReq := ExecuteRequest{
		TaskID:  req.TaskID,
		With:    req.With,
		Timeout: req.Timeout,
	}
	result, err := r.Execute(ctx, execReq)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	return &toolenv.TaskResult{ExecID: result.ExecID, Output: result.Output}, nil
}

// Execute runs a task synchronously using DirectExecutor semantics.
func (r *Runner) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResult, error) {
	prepared, err := r.Prepare(ctx, req)
	if err != nil {
		return nil, err
	}
	return r.ExecutePrepared(ctx, prepared)
}

// Prepare validates the request, loads configuration, and resolves execution dependencies.
func (r *Runner) Prepare(ctx context.Context, req ExecuteRequest) (*PreparedExecution, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	if r.state == nil {
		return nil, ErrStateRequired
	}
	if r.repo == nil {
		return nil, ErrRepositoryRequired
	}
	if r.store == nil {
		return nil, ErrResourceStoreRequired
	}
	if strings.TrimSpace(req.TaskID) == "" {
		return nil, ErrTaskIDRequired
	}
	projectName, err := r.projectName()
	if err != nil {
		return nil, err
	}
	taskCfg, err := r.loadTaskConfig(ctx, projectName, req.TaskID)
	if err != nil {
		return nil, err
	}
	if err := r.hydrateAgentConfig(ctx, projectName, req.TaskID, taskCfg); err != nil {
		return nil, err
	}
	if err := applyExecutionInput(taskCfg, req.With); err != nil {
		return nil, err
	}
	timeout, err := resolveTaskTimeout(ctx, req.Timeout)
	if err != nil {
		return nil, err
	}
	if timeout > 0 {
		taskCfg.Timeout = timeout.String()
	}
	executor, err := taskrouter.ResolveDirectExecutor(ctx, r.state, r.repo)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve direct executor: %w", err)
	}
	metadata := taskrouter.ExecMetadata{Component: deriveComponent(taskCfg), TaskID: taskCfg.ID}
	return &PreparedExecution{
		Config:   taskCfg,
		Executor: executor,
		Metadata: metadata,
		Timeout:  timeout,
	}, nil
}

// ExecutePrepared executes a previously prepared task request.
func (r *Runner) ExecutePrepared(ctx context.Context, prepared *PreparedExecution) (*ExecuteResult, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	if prepared == nil {
		return nil, fmt.Errorf("prepared execution is required")
	}
	if prepared.Config == nil {
		return nil, fmt.Errorf("task configuration is required")
	}
	output, execID, execErr := prepared.Executor.ExecuteSync(ctx, prepared.Config, &prepared.Metadata, prepared.Timeout)
	if execErr != nil {
		return &ExecuteResult{ExecID: execID}, fmt.Errorf("task execution failed: %w", execErr)
	}
	return &ExecuteResult{ExecID: execID, Output: output}, nil
}

func (r *Runner) projectName() (string, error) {
	if r.state != nil && r.state.ProjectConfig != nil {
		if name := r.state.ProjectConfig.Name; strings.TrimSpace(name) != "" {
			return name, nil
		}
	}
	return "", ErrProjectNameRequired
}

func (r *Runner) loadTaskConfig(ctx context.Context, projectName, taskID string) (*task.Config, error) {
	get := taskuc.NewGet(r.store)
	out, err := get.Execute(ctx, &taskuc.GetInput{Project: projectName, ID: taskID})
	if err != nil {
		return nil, fmt.Errorf("failed to load task %s: %w", taskID, err)
	}
	cfg := &task.Config{}
	if err := cfg.FromMap(out.Task); err != nil {
		return nil, fmt.Errorf("failed to decode task %s: %w", taskID, err)
	}
	if strings.TrimSpace(cfg.ID) == "" {
		cfg.ID = taskID
	}
	return cfg, nil
}

func (r *Runner) hydrateAgentConfig(ctx context.Context, projectName, taskID string, cfg *task.Config) error {
	if cfg == nil || cfg.Agent == nil {
		return nil
	}
	agentID := strings.TrimSpace(cfg.Agent.ID)
	if agentID == "" {
		return fmt.Errorf("task %s missing agent identifier", taskID)
	}
	if len(cfg.Agent.Actions) > 0 {
		return nil
	}
	getAgent := agentuc.NewGet(r.store)
	out, err := getAgent.Execute(ctx, &agentuc.GetInput{Project: projectName, ID: agentID})
	if err != nil {
		return fmt.Errorf("failed to load agent %s: %w", agentID, err)
	}
	agentCfg := &agent.Config{}
	if err := agentCfg.FromMap(out.Agent); err != nil {
		return fmt.Errorf("failed to decode agent %s: %w", agentID, err)
	}
	cfg.Agent = agentCfg
	return nil
}

func applyExecutionInput(cfg *task.Config, input core.Input) error {
	if cfg == nil {
		return fmt.Errorf("task config is required")
	}
	if len(input) == 0 {
		return nil
	}
	cloned, err := core.DeepCopyInput(input, core.Input{})
	if err != nil {
		return fmt.Errorf("failed to copy input payload: %w", err)
	}
	if cfg.With == nil {
		withCopy := cloned
		cfg.With = &withCopy
		return nil
	}
	merged := core.CopyMaps(*cfg.With, cloned)
	rebuilt := core.Input(merged)
	cfg.With = &rebuilt
	return nil
}

func resolveTaskTimeout(ctx context.Context, requested time.Duration) (time.Duration, error) {
	if requested < 0 {
		return 0, ErrNegativeTimeout
	}
	defaults := config.DefaultNativeToolsConfig()
	actions := defaults.CallTask
	if appCfg := config.FromContext(ctx); appCfg != nil {
		actions = appCfg.Runtime.NativeTools.CallTask
	}
	timeout := actions.DefaultTimeout
	if timeout <= 0 {
		timeout = defaultTaskTimeout
	}
	if requested > 0 {
		timeout = requested
	}
	if timeout < 0 {
		return 0, ErrNegativeTimeout
	}
	return timeout, nil
}

func deriveComponent(cfg *task.Config) core.ComponentType {
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
