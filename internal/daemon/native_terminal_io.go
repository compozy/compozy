package daemon

import (
	"context"
	"fmt"
	"strconv"

	terminalpkg "github.com/compozy/compozy/internal/terminal"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) terminalWrite(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	manager, actor, workspaceID, err := n.nativeTerminalContext(ctx, scope, req)
	if err != nil {
		return toolspkg.ToolResult{}, terminalToolError(req.ToolID, err)
	}
	var input terminalWriteInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	handle, err := manager.Handle(ctx, workspaceID, actor.ProfileID, terminalpkg.ID(input.TerminalID))
	if err != nil {
		return toolspkg.ToolResult{}, terminalToolError(req.ToolID, err)
	}
	if err := handle.Write(ctx, actor, []byte(input.Data)); err != nil {
		return toolspkg.ToolResult{}, terminalToolError(req.ToolID, err)
	}
	return untrustedTerminalResult(map[string]any{"accepted": true}, "terminal input accepted")
}

func (n *daemonNativeTools) terminalRead(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	manager, actor, workspaceID, err := n.nativeTerminalContext(ctx, scope, req)
	if err != nil {
		return toolspkg.ToolResult{}, terminalToolError(req.ToolID, err)
	}
	var input terminalReadInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	sinceSeq, err := parseNativeTerminalSequence(req.ToolID, input.SinceSeq)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	handle, err := manager.Handle(ctx, workspaceID, actor.ProfileID, terminalpkg.ID(input.TerminalID))
	if err != nil {
		return toolspkg.ToolResult{}, terminalToolError(req.ToolID, err)
	}
	result, err := handle.Screen(ctx, terminalpkg.ReadOptions{
		View: input.View, MaxBytes: input.MaxBytes, SinceSeq: sinceSeq,
		FromLine: input.From, ToLine: input.To, Grep: input.Grep,
	})
	if err != nil {
		return toolspkg.ToolResult{}, terminalToolError(req.ToolID, err)
	}
	return untrustedTerminalResult(terminalReadToolResult{
		Content: result.Content, Seq: strconv.FormatUint(result.Seq, 10), Truncated: result.Truncated,
		Busy: result.Busy, Untrusted: result.Untrusted,
	}, "untrusted terminal output read")
}

type terminalReadToolResult struct {
	Content   string `json:"content"`
	Seq       string `json:"seq"`
	Truncated bool   `json:"truncated"`
	Busy      bool   `json:"busy"`
	Untrusted bool   `json:"untrusted"`
}

func parseNativeTerminalSequence(id toolspkg.ToolID, value string) (uint64, error) {
	if value == "" {
		return 0, nil
	}
	sequence, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput, id, "terminal since_seq must be a decimal uint64", err,
			toolspkg.ReasonSchemaInvalid,
		)
	}
	return sequence, nil
}

func (n *daemonNativeTools) terminalWait(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	manager, actor, workspaceID, err := n.nativeTerminalContext(ctx, scope, req)
	if err != nil {
		return toolspkg.ToolResult{}, terminalToolError(req.ToolID, err)
	}
	var input terminalWaitInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	handle, err := manager.Handle(ctx, workspaceID, actor.ProfileID, terminalpkg.ID(input.TerminalID))
	if err != nil {
		return toolspkg.ToolResult{}, terminalToolError(req.ToolID, err)
	}
	result, err := handle.Wait(ctx, terminalpkg.WaitCondition{
		Until: input.Until, Pattern: input.Pattern, TimeoutMs: input.TimeoutMS,
	})
	if err != nil {
		return toolspkg.ToolResult{}, terminalToolError(req.ToolID, err)
	}
	return untrustedTerminalResult(result, terminalWaitPreview(result))
}

func (n *daemonNativeTools) terminalSignal(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	manager, actor, workspaceID, err := n.nativeTerminalContext(ctx, scope, req)
	if err != nil {
		return toolspkg.ToolResult{}, terminalToolError(req.ToolID, err)
	}
	var input terminalSignalInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	handle, err := manager.Handle(ctx, workspaceID, actor.ProfileID, terminalpkg.ID(input.TerminalID))
	if err == nil {
		err = handle.Signal(ctx, actor, terminalpkg.Signal(input.Signal))
	}
	if err != nil {
		return toolspkg.ToolResult{}, terminalToolError(req.ToolID, err)
	}
	return untrustedTerminalResult(map[string]bool{"delivered": true}, "terminal signal delivered")
}

func terminalWaitPreview(result *terminalpkg.WaitResult) string {
	if result == nil {
		return "terminal wait returned no result"
	}
	switch result.Reason {
	case "exit":
		return "terminal exited"
	case "match":
		return "terminal output matched"
	case "idle":
		return "terminal became idle"
	case "timeout":
		return "terminal wait timed out"
	case "still_running":
		return "terminal is still running"
	case "stalled":
		return "terminal output stalled"
	default:
		return fmt.Sprintf("terminal wait ended with %s", result.Reason)
	}
}
