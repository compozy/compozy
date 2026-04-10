package mcpserver

import (
	"context"
	"fmt"
	"strings"

	reusableagents "github.com/compozy/compozy/internal/core/agents"
	"github.com/compozy/compozy/internal/core/model"
	execpkg "github.com/compozy/compozy/internal/core/run/exec"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

// HostContext carries the host-owned state that a reserved `run_agent` tool
// call is allowed to inherit.
type HostContext struct {
	BaseRuntime reusableagents.NestedBaseRuntime
	Nested      reusableagents.NestedExecutionContext
}

// RunAgentRequest is the generic nested-agent tool contract.
type RunAgentRequest struct {
	Name  string `json:"name"`
	Input string `json:"input"`
}

// RunAgentResult is the deterministic nested-agent success/failure payload.
type RunAgentResult struct {
	Name            string                           `json:"name"`
	Source          string                           `json:"source"`
	Output          string                           `json:"output"`
	RunID           string                           `json:"run_id,omitempty"`
	Success         bool                             `json:"success"`
	Error           string                           `json:"error,omitempty"`
	Blocked         bool                             `json:"blocked,omitempty"`
	BlockedReason   kinds.ReusableAgentBlockedReason `json:"blocked_reason,omitempty"`
	ParentAgentName string                           `json:"parent_agent_name,omitempty"`
	Depth           int                              `json:"depth,omitempty"`
	MaxDepth        int                              `json:"max_depth,omitempty"`
}

// Engine resolves child agents and executes them as real nested ACP sessions.
type Engine struct {
	executePreparedPrompt func(
		context.Context,
		*model.RuntimeConfig,
		string,
		*reusableagents.ExecutionContext,
		execpkg.SessionMCPBuilder,
	) (execpkg.PreparedPromptResult, error)
}

// Option configures an Engine.
type Option func(*Engine)

// WithPromptExecutor overrides the real nested ACP prompt runner for tests.
func WithPromptExecutor(
	fn func(
		context.Context,
		*model.RuntimeConfig,
		string,
		*reusableagents.ExecutionContext,
		execpkg.SessionMCPBuilder,
	) (execpkg.PreparedPromptResult, error),
) Option {
	return func(engine *Engine) {
		if fn != nil {
			engine.executePreparedPrompt = fn
		}
	}
}

// NewEngine constructs a nested-agent execution engine with default dependencies.
func NewEngine(opts ...Option) *Engine {
	engine := &Engine{
		executePreparedPrompt: execpkg.ExecutePreparedPrompt,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(engine)
		}
	}
	return engine
}

// RunAgent executes one child reusable agent and always returns a structured
// result payload suitable for the reserved `run_agent` tool.
func (e *Engine) RunAgent(
	ctx context.Context,
	host HostContext,
	req RunAgentRequest,
) RunAgentResult {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return failureResult(
			name,
			"",
			execpkg.PreparedPromptResult{},
			normalizeNestedContext(host),
			"missing agent name",
		)
	}

	nested := normalizeNestedContext(host)
	if nested.Depth >= nested.MaxDepth {
		return blockedResult(
			name,
			"",
			execpkg.PreparedPromptResult{},
			nested,
			kinds.ReusableAgentBlockedReasonDepthLimit,
			fmt.Sprintf(
				"nested execution blocked: max depth %d reached at depth %d",
				nested.MaxDepth,
				nested.Depth,
			),
		)
	}
	if cycle := cycleDetected(name, nested.AgentPath); cycle != "" {
		return blockedResult(
			name,
			"",
			execpkg.PreparedPromptResult{},
			nested,
			kinds.ReusableAgentBlockedReasonCycleDetected,
			fmt.Sprintf("nested execution blocked: cycle detected for agent %q via %s", name, cycle),
		)
	}

	baseRuntime := buildChildRuntime(host.BaseRuntime, name, req.Input)
	agentExecution, err := reusableagents.ResolveExecutionContext(ctx, &baseRuntime)
	if err != nil {
		if reason, ok := reusableagents.BlockedReasonForError(err); ok {
			return blockedResult(name, "", execpkg.PreparedPromptResult{}, nested, reason, err.Error())
		}
		return failureResult(name, "", execpkg.PreparedPromptResult{}, nested, err.Error())
	}

	source := string(agentExecution.Agent.Source.Scope)
	baseRuntime.AccessMode = capAccessMode(baseRuntime.AccessMode, nested.ParentAccessMode)
	prepared, err := e.executeChild(ctx, &baseRuntime, req.Input, agentExecution, nested)
	if err != nil {
		if reason, ok := reusableagents.BlockedReasonForError(err); ok {
			return blockedResult(agentExecution.Agent.Name, source, prepared, nested, reason, err.Error())
		}
		return failureResult(agentExecution.Agent.Name, source, prepared, nested, err.Error())
	}
	return successResult(agentExecution.Agent.Name, source, prepared, nested)
}

func (e *Engine) executeChild(
	ctx context.Context,
	cfg *model.RuntimeConfig,
	input string,
	agentExecution *reusableagents.ExecutionContext,
	nested reusableagents.NestedExecutionContext,
) (execpkg.PreparedPromptResult, error) {
	return e.executePreparedPrompt(
		ctx,
		cfg,
		input,
		agentExecution,
		func(runID string) ([]model.MCPServer, error) {
			return reusableagents.BuildSessionMCPServers(
				agentExecution,
				reusableagents.SessionMCPContext{
					RunID:               runID,
					ParentAgentName:     agentExecution.Agent.Name,
					EffectiveAccessMode: cfg.AccessMode,
					NestedDepth:         nested.Depth + 1,
					MaxNestedDepth:      nested.MaxDepth,
					AgentPath:           append(cloneAgentPath(nested.AgentPath), agentExecution.Agent.Name),
				},
			)
		},
	)
}

func normalizeNestedContext(host HostContext) reusableagents.NestedExecutionContext {
	nested := host.Nested
	if nested.MaxDepth <= 0 {
		nested.MaxDepth = reusableagents.DefaultMaxNestedDepth
	}
	if nested.Depth < 0 {
		nested.Depth = 0
	}
	if strings.TrimSpace(nested.ParentAccessMode) == "" {
		base := host.BaseRuntime.RuntimeConfig()
		base.ApplyDefaults()
		nested.ParentAccessMode = base.AccessMode
	}
	if len(nested.AgentPath) == 0 && strings.TrimSpace(nested.ParentAgentName) != "" {
		nested.AgentPath = []string{nested.ParentAgentName}
	}
	return nested
}

func capAccessMode(child, parent string) string {
	if strings.TrimSpace(parent) != model.AccessModeFull {
		return model.AccessModeDefault
	}
	if strings.TrimSpace(child) == model.AccessModeFull {
		return model.AccessModeFull
	}
	return model.AccessModeDefault
}

func buildChildRuntime(
	base reusableagents.NestedBaseRuntime,
	name string,
	input string,
) model.RuntimeConfig {
	runtime := base.RuntimeConfig()
	runtime.AgentName = name
	runtime.ResolvedPromptText = input
	runtime.PromptText = ""
	runtime.PromptFile = ""
	runtime.ReadPromptStdin = false
	runtime.TUI = false
	runtime.Persist = false
	runtime.OutputFormat = model.OutputFormatText
	runtime.Mode = model.ExecutionModeExec
	return runtime
}

func successResult(
	name string,
	source string,
	prepared execpkg.PreparedPromptResult,
	nested reusableagents.NestedExecutionContext,
) RunAgentResult {
	return RunAgentResult{
		Name:            name,
		Source:          source,
		Output:          prepared.Output,
		RunID:           prepared.RunID,
		Success:         true,
		ParentAgentName: nested.ParentAgentName,
		Depth:           nested.Depth + 1,
		MaxDepth:        nested.MaxDepth,
	}
}

func failureResult(
	name string,
	source string,
	prepared execpkg.PreparedPromptResult,
	nested reusableagents.NestedExecutionContext,
	errText string,
) RunAgentResult {
	return RunAgentResult{
		Name:            name,
		Source:          source,
		Output:          prepared.Output,
		RunID:           prepared.RunID,
		Success:         false,
		Error:           errText,
		ParentAgentName: nested.ParentAgentName,
		Depth:           nested.Depth + 1,
		MaxDepth:        nested.MaxDepth,
	}
}

func blockedResult(
	name string,
	source string,
	prepared execpkg.PreparedPromptResult,
	nested reusableagents.NestedExecutionContext,
	reason kinds.ReusableAgentBlockedReason,
	errText string,
) RunAgentResult {
	result := failureResult(name, source, prepared, nested, errText)
	result.Blocked = true
	result.BlockedReason = reason
	return result
}

func cycleDetected(name string, path []string) string {
	normalized := strings.TrimSpace(name)
	if normalized == "" {
		return ""
	}
	for _, entry := range path {
		if strings.TrimSpace(entry) == normalized {
			return strings.Join(append(cloneAgentPath(path), normalized), " -> ")
		}
	}
	return ""
}

func cloneAgentPath(path []string) []string {
	if len(path) == 0 {
		return nil
	}
	return append([]string(nil), path...)
}
