package daemon

import (
	"context"
	"strings"

	"github.com/compozy/compozy/internal/api/core"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) automationRunsCancel(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input automationRunIDInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	runID, err := requiredNativeString(req.ToolID, "run_id", input.RunID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	readScope, err := n.nativeProfileReadScope(ctx, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID := strings.TrimSpace(input.WorkspaceID)
	if callerWorkspace := strings.TrimSpace(scope.WorkspaceID); callerWorkspace != "" && !scope.Operator {
		if workspaceID != "" && workspaceID != callerWorkspace {
			return toolspkg.ToolResult{}, nativeScopeMismatchError(req.ToolID, "workspace")
		}
		workspaceID = callerWorkspace
	}
	run, err := n.nativeAutomationRunForWorkspace(ctx, workspaceID, runID, readScope)
	if err != nil {
		return toolspkg.ToolResult{}, nativeAutomationToolError(req.ToolID, err)
	}
	run, err = n.automationManager().CancelRun(ctx, run.ID)
	if err != nil {
		return toolspkg.ToolResult{}, nativeAutomationToolError(req.ToolID, err)
	}
	payload := core.RunPayloadFromRun(run)
	return structuredResult(map[string]any{nativeAutomationToolsRunKey: payload}, payload.ID)
}
