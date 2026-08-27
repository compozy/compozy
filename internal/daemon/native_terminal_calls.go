package daemon

import (
	"context"
	"fmt"
	"strings"

	terminalpkg "github.com/compozy/compozy/internal/terminal"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) terminalExec(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	manager, actor, workspaceID, err := n.nativeTerminalContext(ctx, scope, req)
	if err != nil {
		return toolspkg.ToolResult{}, terminalToolError(req.ToolID, err)
	}
	var input terminalExecInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	argv := append([]string{input.Command}, input.Args...)
	classification := terminalpkg.ClassifyArgv(argv, nil)
	approval, err := n.authorizeNativeTerminalExec(ctx, scope, req, classification)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	capabilities, err := n.nativeTerminalCapabilities(ctx, workspaceID)
	if err != nil {
		return toolspkg.ToolResult{}, terminalToolError(req.ToolID, err)
	}
	result, err := manager.Exec(ctx, terminalpkg.ExecRequest{
		WS: workspaceID, Command: input.Command, Args: input.Args, Cwd: input.Cwd, Env: input.Env,
		YieldMs: input.YieldMS, Visible: input.Visible, Output: input.Output,
		Approval: approval, Actor: actor,
		Capabilities: capabilities,
	})
	if err != nil {
		return toolspkg.ToolResult{}, terminalToolError(req.ToolID, err)
	}
	return structuredResult(result, terminalExecPreview(result))
}

func (n *daemonNativeTools) terminalOpen(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	manager, actor, workspaceID, err := n.nativeTerminalContext(ctx, scope, req)
	if err != nil {
		return toolspkg.ToolResult{}, terminalToolError(req.ToolID, err)
	}
	var input terminalOpenInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	capabilities, err := n.nativeTerminalCapabilities(ctx, workspaceID)
	if err != nil {
		return toolspkg.ToolResult{}, terminalToolError(req.ToolID, err)
	}
	handle, err := manager.Open(ctx, terminalpkg.OpenRequest{
		WS: workspaceID, Cwd: input.Cwd, Shell: input.Shell, Title: input.Title,
		Cols: input.Cols, Rows: input.Rows, Actor: actor,
		Capabilities: capabilities,
	})
	if err != nil {
		return toolspkg.ToolResult{}, terminalToolError(req.ToolID, err)
	}
	return structuredResult(map[string]any{"terminal_id": handle.Info().ID}, "interactive terminal opened")
}

func terminalExecPreview(result *terminalpkg.ExecResult) string {
	if result != nil && result.StillRunning && result.TerminalID != nil {
		return fmt.Sprintf("terminal command still running in %s", *result.TerminalID)
	}
	return "terminal command completed"
}

func (n *daemonNativeTools) authorizeNativeTerminalExec(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
	classification terminalpkg.CommandClassification,
) (string, error) {
	if classification.Verdict == terminalpkg.CommandVerdictDenied {
		return "", terminalCodeToolError(
			req.ToolID,
			"approval_rejected",
			"irreversible terminal command refused",
			toolspkg.ErrToolDenied,
		)
	}
	if classification.Verdict == terminalpkg.CommandVerdictAllowlisted {
		return "allowlisted", nil
	}
	if req.ApprovalGranted {
		if label := strings.TrimSpace(req.ApprovalLabel); label != "" {
			return label, nil
		}
		return toolApprovalApprovedOnceLabel, nil
	}
	if strings.TrimSpace(req.ApprovalToken) != "" {
		return toolApprovalApprovedOnceLabel, nil
	}
	if n != nil && n.deps != nil && n.deps.TerminalExecApprover != nil {
		label, err := n.deps.TerminalExecApprover.ApproveTerminalExec(ctx, scope, req)
		if err != nil {
			return "", err
		}
		return label, nil
	}
	return "", terminalCodeToolError(
		req.ToolID,
		"approval_required",
		"terminal command requires operator approval",
		toolspkg.ErrToolApprovalRequired,
	)
}
