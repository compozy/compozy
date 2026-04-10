package agents

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/core/model"
)

const (
	// DefaultMaxNestedDepth bounds recursive child-agent execution.
	DefaultMaxNestedDepth = 3
	// RunAgentContextEnvVar carries the host-owned nested execution context into
	// the reserved `compozy` MCP server process.
	RunAgentContextEnvVar = "COMPOZY_RUN_AGENT_CONTEXT"
)

// NestedBaseRuntime captures the pre-agent runtime defaults that child-agent
// resolution must inherit from the parent host, without leaking prompt input.
type NestedBaseRuntime struct {
	WorkspaceRoot          string                     `json:"workspace_root,omitempty"`
	IDE                    string                     `json:"ide,omitempty"`
	Model                  string                     `json:"model,omitempty"`
	AddDirs                []string                   `json:"add_dirs,omitempty"`
	ReasoningEffort        string                     `json:"reasoning_effort,omitempty"`
	AccessMode             string                     `json:"access_mode,omitempty"`
	ExplicitRuntime        model.ExplicitRuntimeFlags `json:"explicit_runtime,omitempty"`
	Timeout                time.Duration              `json:"timeout,omitempty"`
	MaxRetries             int                        `json:"max_retries,omitempty"`
	RetryBackoffMultiplier float64                    `json:"retry_backoff_multiplier,omitempty"`
}

// RuntimeConfig materializes the child-runtime base config used before agent
// defaults are applied during nested execution.
func (r NestedBaseRuntime) RuntimeConfig() model.RuntimeConfig {
	return model.RuntimeConfig{
		WorkspaceRoot:          r.WorkspaceRoot,
		IDE:                    r.IDE,
		Model:                  r.Model,
		AddDirs:                append([]string(nil), r.AddDirs...),
		ReasoningEffort:        r.ReasoningEffort,
		AccessMode:             r.AccessMode,
		ExplicitRuntime:        r.ExplicitRuntime,
		Mode:                   model.ExecutionModeExec,
		OutputFormat:           model.OutputFormatText,
		Timeout:                r.Timeout,
		MaxRetries:             r.MaxRetries,
		RetryBackoffMultiplier: r.RetryBackoffMultiplier,
	}
}

// NestedExecutionContext is the host-owned recursion state propagated to the
// reserved `compozy` MCP server. Tool callers do not control these values.
type NestedExecutionContext struct {
	Depth            int      `json:"depth"`
	MaxDepth         int      `json:"max_depth"`
	ParentRunID      string   `json:"parent_run_id,omitempty"`
	ParentAgentName  string   `json:"parent_agent_name,omitempty"`
	ParentAccessMode string   `json:"parent_access_mode,omitempty"`
	AgentPath        []string `json:"agent_path,omitempty"`
}

// ReservedServerRuntimeContext is serialized into the reserved server
// environment so future nested tool calls can recreate the child host context.
type ReservedServerRuntimeContext struct {
	BaseRuntime NestedBaseRuntime      `json:"base_runtime"`
	Nested      NestedExecutionContext `json:"nested"`
}

// SessionMCPContext describes the host-owned session metadata needed to build
// the merged MCP server list for one reusable-agent-backed ACP session.
type SessionMCPContext struct {
	RunID                string
	ParentAgentName      string
	EffectiveAccessMode  string
	NestedDepth          int
	MaxNestedDepth       int
	AgentPath            []string
	ReservedServerBinary string
	ReservedServerArgs   []string
	BaseRuntime          *model.RuntimeConfig
}

// BuildSessionMCPServers returns the reserved `compozy` MCP server plus the
// selected agent's own MCP servers, in deterministic order.
func BuildSessionMCPServers(
	execution *ExecutionContext,
	ctx SessionMCPContext,
) ([]model.MCPServer, error) {
	if execution == nil && ctx.BaseRuntime == nil {
		return nil, nil
	}

	reserved, err := buildReservedSessionMCPServer(execution, ctx)
	if err != nil {
		return nil, err
	}

	servers := []model.MCPServer{reserved}
	if execution == nil || execution.Agent.MCP == nil || len(execution.Agent.MCP.Servers) == 0 {
		return servers, nil
	}

	for idx := range execution.Agent.MCP.Servers {
		resolved := execution.Agent.MCP.Servers[idx]
		if resolved.Name == ReservedMCPServerName {
			return nil, fmt.Errorf(
				"%w: agent %q declared reserved server %q",
				ErrReservedMCPServerName,
				execution.Agent.Name,
				resolved.Name,
			)
		}
		servers = append(servers, model.MCPServer{
			Stdio: &model.MCPServerStdio{
				Name:    resolved.Name,
				Command: resolved.Command,
				Args:    append([]string(nil), resolved.Args...),
				Env:     cloneStringMap(resolved.Env),
			},
		})
	}
	return servers, nil
}

func buildReservedSessionMCPServer(
	execution *ExecutionContext,
	ctx SessionMCPContext,
) (model.MCPServer, error) {
	command, args, err := reservedServerCommand(ctx)
	if err != nil {
		return model.MCPServer{}, err
	}

	effectiveAccessMode := strings.TrimSpace(ctx.EffectiveAccessMode)
	if effectiveAccessMode == "" {
		effectiveAccessMode = model.AccessModeDefault
	}

	baseRuntime := NestedBaseRuntime{}
	if execution != nil {
		baseRuntime = execution.BaseRuntime
	} else if ctx.BaseRuntime != nil {
		baseRuntime = captureNestedBaseRuntime(ctx.BaseRuntime)
	}

	nested := NestedExecutionContext{
		Depth:            max(0, ctx.NestedDepth),
		MaxDepth:         ctx.MaxNestedDepth,
		ParentRunID:      strings.TrimSpace(ctx.RunID),
		ParentAgentName:  strings.TrimSpace(ctx.ParentAgentName),
		ParentAccessMode: effectiveAccessMode,
		AgentPath:        cloneStringSlice(ctx.AgentPath),
	}
	if nested.MaxDepth <= 0 {
		nested.MaxDepth = DefaultMaxNestedDepth
	}
	if execution != nil && nested.ParentAgentName == "" {
		nested.ParentAgentName = execution.Agent.Name
	}
	if execution != nil && len(nested.AgentPath) == 0 {
		nested.AgentPath = []string{execution.Agent.Name}
	}

	payload, err := json.Marshal(ReservedServerRuntimeContext{
		BaseRuntime: baseRuntime,
		Nested:      nested,
	})
	if err != nil {
		return model.MCPServer{}, fmt.Errorf("marshal reserved MCP server context: %w", err)
	}

	return model.MCPServer{
		Stdio: &model.MCPServerStdio{
			Name:    ReservedMCPServerName,
			Command: command,
			Args:    args,
			Env: map[string]string{
				RunAgentContextEnvVar: string(payload),
			},
		},
	}, nil
}

func reservedServerCommand(ctx SessionMCPContext) (string, []string, error) {
	command := strings.TrimSpace(ctx.ReservedServerBinary)
	if command == "" {
		resolved, err := os.Executable()
		if err != nil {
			return "", nil, fmt.Errorf("resolve reserved MCP server executable: %w", err)
		}
		command = resolved
	}

	args := append([]string(nil), ctx.ReservedServerArgs...)
	if len(args) == 0 {
		args = []string{"mcp-serve", "--server", ReservedMCPServerName}
	}

	return command, args, nil
}

func captureNestedBaseRuntime(cfg *model.RuntimeConfig) NestedBaseRuntime {
	if cfg == nil {
		return NestedBaseRuntime{}
	}

	return NestedBaseRuntime{
		WorkspaceRoot:          cfg.WorkspaceRoot,
		IDE:                    cfg.IDE,
		Model:                  cfg.Model,
		AddDirs:                append([]string(nil), cfg.AddDirs...),
		ReasoningEffort:        cfg.ReasoningEffort,
		AccessMode:             cfg.AccessMode,
		ExplicitRuntime:        cfg.ExplicitRuntime,
		Timeout:                cfg.Timeout,
		MaxRetries:             cfg.MaxRetries,
		RetryBackoffMultiplier: cfg.RetryBackoffMultiplier,
	}
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(src))
	for key, value := range src {
		cloned[key] = value
	}
	return cloned
}

func cloneStringSlice(src []string) []string {
	if len(src) == 0 {
		return nil
	}
	return append([]string(nil), src...)
}
