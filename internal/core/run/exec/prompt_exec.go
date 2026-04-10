package exec

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/core/agent"
	reusableagents "github.com/compozy/compozy/internal/core/agents"
	"github.com/compozy/compozy/internal/core/model"
)

// PreparedPromptResult captures the stable nested-execution details needed by
// the reserved `run_agent` engine.
type PreparedPromptResult struct {
	RunID    string
	Output   string
	Snapshot SessionViewSnapshot
	Identity agent.SessionIdentity
}

// SessionMCPBuilder builds the final session MCP list after the child run id is
// allocated but before the ACP session is opened.
type SessionMCPBuilder func(runID string) ([]model.MCPServer, error)

// ExecutePreparedPrompt runs one real ACP-backed exec prompt without emitting
// nested output to the parent stdout stream.
func ExecutePreparedPrompt(
	ctx context.Context,
	cfg *model.RuntimeConfig,
	promptText string,
	agentExecution *reusableagents.ExecutionContext,
	buildMCPServers SessionMCPBuilder,
) (PreparedPromptResult, error) {
	if cfg == nil {
		return PreparedPromptResult{}, fmt.Errorf("execute prepared prompt: missing runtime config")
	}
	if strings.TrimSpace(promptText) == "" {
		return PreparedPromptResult{}, fmt.Errorf("execute prepared prompt: prompt is empty")
	}
	if err := agent.EnsureAvailable(ctx, cfg); err != nil {
		return PreparedPromptResult{}, err
	}

	state, err := prepareExecRunState(ctx, cfg)
	if err != nil {
		return PreparedPromptResult{}, err
	}
	defer state.close()

	if err := state.writeStarted(cfg); err != nil {
		return PreparedPromptResult{}, err
	}

	var mcpServers []model.MCPServer
	if buildMCPServers != nil {
		mcpServers, err = buildMCPServers(state.runArtifacts.RunID)
		if err != nil {
			failure := execExecutionResult{
				status: runStatusFailed,
				err:    err,
			}
			if completeErr := state.completeTurn(failure); completeErr != nil {
				if !errors.Is(completeErr, err) {
					return buildPreparedPromptResult(state, failure), errors.Join(err, completeErr)
				}
				return buildPreparedPromptResult(state, failure), completeErr
			}
			return buildPreparedPromptResult(state, failure), err
		}
	}

	// Nested execution should be observable in the returned result object, not
	// written into the parent process stdout stream.
	state.emitText = false
	state.events = nil

	internalCfg := newConfig(cfg, state.runArtifacts)
	execJob, err := newExecRuntimeJobWithMCP(promptText, state, agentExecution, mcpServers)
	if err != nil {
		return PreparedPromptResult{}, err
	}
	result := executeExecJob(ctx, internalCfg, &execJob, cfg.WorkspaceRoot, false, state)
	if completeErr := state.completeTurn(result); completeErr != nil {
		if result.err != nil && !errors.Is(completeErr, result.err) {
			return buildPreparedPromptResult(state, result), errors.Join(result.err, completeErr)
		}
		return buildPreparedPromptResult(state, result), completeErr
	}
	return buildPreparedPromptResult(state, result), nil
}

func buildPreparedPromptResult(state *execRunState, result execExecutionResult) PreparedPromptResult {
	runID := ""
	if state != nil {
		runID = state.runArtifacts.RunID
	}
	return PreparedPromptResult{
		RunID:    runID,
		Output:   result.output,
		Snapshot: result.snapshot,
		Identity: result.identity,
	}
}
