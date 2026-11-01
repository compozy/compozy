package daemon

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/session"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) agentDraftInstructions(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input contract.DraftAgentInstructionsRequest
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	if strings.TrimSpace(input.Goal) == "" || strings.TrimSpace(input.Runtime.Provider) == "" {
		return toolspkg.ToolResult{}, errors.New("goal and runtime.provider are required")
	}
	drafter, ok := n.deps.Sessions.(core.AgentInstructionsDrafter)
	if !ok {
		return toolspkg.ToolResult{}, session.ErrAgentDraftFailed
	}
	instructions, err := drafter.DraftInstructions(ctx, session.DraftInstructionsRequest{
		Goal: input.Goal, ProfileID: scope.ProfileID, Provider: input.Runtime.Provider,
		Model: input.Runtime.Model, ReasoningEffort: input.Runtime.ReasoningEffort,
	})
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(contract.DraftAgentInstructionsResponse{Instructions: instructions}, "Draft instructions")
}
