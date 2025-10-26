package agentexec

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/agent"
	agentuc "github.com/compozy/compozy/engine/agent/uc"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/runtime/toolenv"
	"github.com/compozy/compozy/engine/runtime/toolenvstate"
	"github.com/compozy/compozy/engine/task"
	tkrouter "github.com/compozy/compozy/engine/task/router"
)

const (
	// DefaultTimeout defines the fallback timeout applied when callers omit a value.
	DefaultTimeout = 60 * time.Second
	// MaxTimeout caps how long agent executions are allowed to run.
	MaxTimeout = 300 * time.Second
	// promptActionID labels prompt-only executions so task state constraints remain satisfied.
	promptActionID = "__prompt__"
)

var (
	ErrAgentIDRequired        = errors.New("agent id is required")
	ErrActionOrPromptRequired = errors.New("either action or prompt is required")
	ErrStateRequired          = errors.New("app state is required")
	ErrRepositoryRequired     = errors.New("task repository is required")
	ErrResourceStoreRequired  = errors.New("resource store is required")
	ErrProjectNameRequired    = errors.New("project name is required")
	ErrNegativeTimeout        = errors.New("timeout must be non-negative")
	ErrTimeoutTooLarge        = fmt.Errorf("timeout cannot exceed %s", MaxTimeout)
	ErrUnknownAction          = errors.New("unknown action")
)

// Runner executes agents synchronously using the DirectExecutor pipeline.
type Runner struct {
	state *appstate.State
	repo  task.Repository
	store resources.ResourceStore
}

// ExecuteRequest describes a single synchronous agent execution request.
type ExecuteRequest struct {
	AgentID string
	Action  string
	Prompt  string
	With    core.Input
	Timeout time.Duration
}

// ExecuteResult contains the outcome of a synchronous agent execution.
type ExecuteResult struct {
	ExecID core.ID
	Output *core.Output
}

// PreparedExecution captures the resources required to run an agent execution.
type PreparedExecution struct {
	Config   *task.Config
	Metadata tkrouter.ExecMetadata
	Executor tkrouter.DirectExecutor
	Timeout  time.Duration
}

// NewRunner constructs a Runner using the provided application state, task repository, and resource store.
func NewRunner(state *appstate.State, repo task.Repository, store resources.ResourceStore) *Runner {
	runner := &Runner{
		state: state,
		repo:  repo,
		store: store,
	}
	if state != nil && repo != nil && store != nil {
		env := mustCreateEnvironment(runner, repo, store)
		toolenvstate.Store(state, env)
	}
	return runner
}

// ExecuteAgent satisfies toolenv.AgentExecutor by adapting AgentRequest into the
// runner's native ExecuteRequest.
func (r *Runner) ExecuteAgent(ctx context.Context, req toolenv.AgentRequest) (*toolenv.AgentResult, error) {
	execReq := ExecuteRequest{
		AgentID: req.AgentID,
		Action:  req.Action,
		Prompt:  req.Prompt,
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
	return &toolenv.AgentResult{
		ExecID: result.ExecID,
		Output: result.Output,
	}, nil
}

// Execute runs an agent synchronously using DirectExecutor semantics.
func (r *Runner) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResult, error) {
	prepared, err := r.Prepare(ctx, req)
	if err != nil {
		return nil, err
	}
	return r.ExecutePrepared(ctx, prepared)
}

// ExecutePrepared executes a previously prepared agent request.
func (r *Runner) ExecutePrepared(ctx context.Context, prepared *PreparedExecution) (*ExecuteResult, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	if prepared == nil {
		return nil, fmt.Errorf("prepared execution is required")
	}
	output, execID, execErr := prepared.Executor.ExecuteSync(ctx, prepared.Config, &prepared.Metadata, prepared.Timeout)
	if execErr != nil {
		return &ExecuteResult{ExecID: execID}, fmt.Errorf("agent execution failed: %w", execErr)
	}
	return &ExecuteResult{
		ExecID: execID,
		Output: output,
	}, nil
}

// Prepare validates the request, loads configuration, and resolves execution dependencies.
func (r *Runner) Prepare(ctx context.Context, req ExecuteRequest) (*PreparedExecution, error) {
	if err := r.validatePreparePreconditions(ctx, req); err != nil {
		return nil, err
	}
	projectName, err := r.projectName()
	if err != nil {
		return nil, err
	}
	timeout, err := normalizeTimeout(req.Timeout)
	if err != nil {
		return nil, err
	}
	agentConfig, err := r.loadAgentConfig(ctx, projectName, req.AgentID)
	if err != nil {
		return nil, err
	}
	if err := validateAgentAction(agentConfig, req.Action); err != nil {
		return nil, err
	}
	taskCfg, err := buildTaskConfig(req.AgentID, agentConfig, req, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to build task config: %w", err)
	}
	executor, err := tkrouter.ResolveDirectExecutor(ctx, r.state, r.repo)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize executor: %w", err)
	}
	return &PreparedExecution{
		Config:   taskCfg,
		Metadata: buildExecMetadata(req, taskCfg),
		Executor: executor,
		Timeout:  timeout,
	}, nil
}

func (r *Runner) validatePreparePreconditions(ctx context.Context, req ExecuteRequest) error {
	if ctx == nil {
		return fmt.Errorf("context is required")
	}
	if r.state == nil {
		return ErrStateRequired
	}
	if r.repo == nil {
		return ErrRepositoryRequired
	}
	if r.store == nil {
		return ErrResourceStoreRequired
	}
	if req.AgentID == "" {
		return ErrAgentIDRequired
	}
	if req.Action == "" && req.Prompt == "" {
		return ErrActionOrPromptRequired
	}
	return nil
}

func (r *Runner) projectName() (string, error) {
	if r.state != nil && r.state.ProjectConfig != nil {
		if name := r.state.ProjectConfig.Name; name != "" {
			return name, nil
		}
	}
	return "", ErrProjectNameRequired
}

func buildExecMetadata(req ExecuteRequest, cfg *task.Config) tkrouter.ExecMetadata {
	metadata := tkrouter.ExecMetadata{
		Component: core.ComponentAgent,
		AgentID:   req.AgentID,
	}
	if cfg != nil {
		metadata.TaskID = cfg.ID
	}
	metadata.ActionID = resolveAgentActionID(req, cfg)
	return metadata
}

// NewEnvironment constructs a tool execution environment backed by the runner.
func NewEnvironment(state *appstate.State, repo task.Repository, store resources.ResourceStore) toolenv.Environment {
	runner := NewRunner(state, repo, store)
	return mustCreateEnvironment(runner, repo, store)
}

// mustCreateEnvironment panics when the tool environment cannot be initialized.
func mustCreateEnvironment(
	agent toolenv.AgentExecutor,
	repo task.Repository,
	store resources.ResourceStore,
) toolenv.Environment {
	env, err := toolenv.New(agent, toolenv.NoopTaskExecutor(), toolenv.NoopWorkflowExecutor(), repo, store)
	if err != nil {
		panic(fmt.Sprintf("tool environment initialization failed: %v", err))
	}
	return env
}

func (r *Runner) loadAgentConfig(ctx context.Context, projectName, agentID string) (*agent.Config, error) {
	getUC := agentuc.NewGet(r.store)
	out, err := getUC.Execute(ctx, &agentuc.GetInput{Project: projectName, ID: agentID})
	if err != nil {
		return nil, err
	}
	cfg := &agent.Config{}
	if err := cfg.FromMap(out.Agent); err != nil {
		return nil, fmt.Errorf("failed to decode agent config: %w", err)
	}
	return cfg, nil
}

func validateAgentAction(agentConfig *agent.Config, action string) error {
	if action == "" {
		return nil
	}
	if _, err := agent.FindActionConfig(agentConfig.Actions, action); err != nil {
		return fmt.Errorf("%w: %v", ErrUnknownAction, err)
	}
	return nil
}

func buildTaskConfig(
	agentID string,
	agentConfig *agent.Config,
	req ExecuteRequest,
	timeout time.Duration,
) (*task.Config, error) {
	cfg := &task.Config{
		BaseConfig: task.BaseConfig{
			ID:    fmt.Sprintf("agent:%s", agentID),
			Type:  task.TaskTypeBasic,
			Agent: agentConfig,
		},
	}
	if req.Action != "" {
		cfg.Action = req.Action
	}
	if req.Prompt != "" {
		cfg.Prompt = req.Prompt
	}
	if len(req.With) > 0 {
		copied, err := core.DeepCopy(req.With)
		if err != nil {
			return nil, fmt.Errorf("failed to copy input: %w", err)
		}
		cfg.With = &copied
	}
	cfg.Timeout = timeout.String()
	return cfg, nil
}

func resolveAgentActionID(req ExecuteRequest, cfg *task.Config) string {
	if req.Action != "" {
		return req.Action
	}
	if cfg != nil && cfg.Action != "" {
		return cfg.Action
	}
	if req.Prompt != "" {
		return promptActionID
	}
	return ""
}

func normalizeTimeout(raw time.Duration) (time.Duration, error) {
	if raw < 0 {
		return 0, ErrNegativeTimeout
	}
	if raw == 0 {
		return DefaultTimeout, nil
	}
	if raw > MaxTimeout {
		return 0, ErrTimeoutTooLarge
	}
	return raw, nil
}
