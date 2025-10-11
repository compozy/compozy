package builder

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/agent"
	agentuc "github.com/compozy/compozy/engine/agent/uc"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/runtime/toolenv"
	"github.com/compozy/compozy/engine/runtime/toolenvstate"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/directexec"
	"github.com/compozy/compozy/engine/workflow"
)

const (
	defaultAgentTimeout = 60 * time.Second
	maxAgentTimeout     = 300 * time.Second
	promptActionID      = "__prompt__"
)

var (
	errTaskRepoRequired    = errors.New("tool environment: task repository is required")
	errStoreRequired       = errors.New("tool environment: resource store is required")
	errAgentIDRequired     = errors.New("tool environment: agent id is required")
	errActionOrPrompt      = errors.New("tool environment: either action or prompt is required")
	errProjectNameRequired = errors.New("tool environment: project name is required")
	errNegativeTimeout     = errors.New("tool environment: timeout must be non-negative")
	errTimeoutTooLarge     = fmt.Errorf("tool environment: timeout cannot exceed %s", maxAgentTimeout)
	errUnknownAgentAction  = errors.New("tool environment: unknown agent action")
)

// Build constructs a tool environment backed by a direct executor.
func Build(
	projectConfig *project.Config,
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	repo task.Repository,
	store resources.ResourceStore,
) (toolenv.Environment, error) {
	if projectConfig == nil {
		return nil, fmt.Errorf("tool environment: project config is required")
	}
	if repo == nil {
		return nil, errTaskRepoRequired
	}
	if store == nil {
		return nil, errStoreRequired
	}
	state, err := appstate.NewState(appstate.NewBaseDeps(projectConfig, workflows, nil, nil), nil)
	if err != nil {
		return nil, fmt.Errorf("tool environment: create app state: %w", err)
	}
	state.SetResourceStore(store)
	executor := &directAgentExecutor{
		state:        state,
		taskRepo:     repo,
		workflowRepo: workflowRepo,
		store:        store,
	}
	env := toolenv.New(executor, repo, store)
	if env == nil {
		return nil, fmt.Errorf("tool environment: initialization failed")
	}
	toolenvstate.Store(state, env)
	return env, nil
}

type directAgentExecutor struct {
	state        *appstate.State
	taskRepo     task.Repository
	workflowRepo workflow.Repository
	store        resources.ResourceStore
}

func (e *directAgentExecutor) ExecuteAgent(
	ctx context.Context,
	req toolenv.AgentRequest,
) (*toolenv.AgentResult, error) {
	if ctx == nil {
		return nil, fmt.Errorf("tool environment: context is required")
	}
	if req.AgentID == "" {
		return nil, errAgentIDRequired
	}
	if req.Action == "" && req.Prompt == "" {
		return nil, errActionOrPrompt
	}
	timeout, err := normalizeTimeout(req.Timeout)
	if err != nil {
		return nil, err
	}
	projectName := ""
	if e.state.ProjectConfig != nil {
		projectName = e.state.ProjectConfig.Name
	}
	if projectName == "" {
		return nil, errProjectNameRequired
	}
	agentConfig, err := e.loadAgentConfig(ctx, projectName, req.AgentID)
	if err != nil {
		return nil, err
	}
	if err := validateAgentAction(agentConfig, req.Action); err != nil {
		return nil, err
	}
	execCfg := buildTaskConfig(req.AgentID, agentConfig, req, timeout)
	executor, err := directexec.NewDirectExecutor(e.state, e.taskRepo, e.workflowRepo)
	if err != nil {
		return nil, fmt.Errorf("tool environment: resolve direct executor: %w", err)
	}
	meta := directexec.ExecMetadata{
		Component: core.ComponentAgent,
		AgentID:   req.AgentID,
		ActionID:  resolveActionID(req, execCfg),
		TaskID:    execCfg.ID,
	}
	output, execID, execErr := executor.ExecuteSync(ctx, execCfg, &meta, timeout)
	if execErr != nil {
		return &toolenv.AgentResult{ExecID: execID}, execErr
	}
	return &toolenv.AgentResult{ExecID: execID, Output: output}, nil
}

func (e *directAgentExecutor) loadAgentConfig(ctx context.Context, projectName, agentID string) (*agent.Config, error) {
	getUC := agentuc.NewGet(e.store)
	out, err := getUC.Execute(ctx, &agentuc.GetInput{Project: projectName, ID: agentID})
	if err != nil {
		return nil, err
	}
	cfg := &agent.Config{}
	if err := cfg.FromMap(out.Agent); err != nil {
		return nil, fmt.Errorf("tool environment: decode agent config: %w", err)
	}
	return cfg, nil
}

func validateAgentAction(agentConfig *agent.Config, action string) error {
	if action == "" {
		return nil
	}
	if _, err := agent.FindActionConfig(agentConfig.Actions, action); err != nil {
		return fmt.Errorf("%w: %v", errUnknownAgentAction, err)
	}
	return nil
}

func buildTaskConfig(
	agentID string,
	agentConfig *agent.Config,
	req toolenv.AgentRequest,
	timeout time.Duration,
) *task.Config {
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
		withCopy := req.With
		cfg.With = &withCopy
	}
	cfg.Timeout = timeout.String()
	return cfg
}

func resolveActionID(req toolenv.AgentRequest, cfg *task.Config) string {
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
		return 0, errNegativeTimeout
	}
	if raw == 0 {
		return defaultAgentTimeout, nil
	}
	if raw > maxAgentTimeout {
		return 0, errTimeoutTooLarge
	}
	return raw, nil
}
