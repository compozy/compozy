package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
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
		return toolspkg.ToolResult{}, terminalNativeError(req.ToolID, err)
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
	result, err := manager.Exec(ctx, terminalpkg.ExecRequest{
		WS: workspaceID, Command: input.Command, Args: input.Args, Cwd: input.Cwd, Env: input.Env,
		YieldMs: input.YieldMS, Visible: input.Visible, Output: input.Output,
		Approval: approval, Actor: actor,
		Capabilities: terminalpkg.Capabilities{Interactive: true},
	})
	if err != nil {
		return toolspkg.ToolResult{}, terminalNativeError(req.ToolID, err)
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
		return toolspkg.ToolResult{}, terminalNativeError(req.ToolID, err)
	}
	var input terminalOpenInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	handle, err := manager.Open(ctx, terminalpkg.OpenRequest{
		WS: workspaceID, Cwd: input.Cwd, Shell: input.Shell, Title: input.Title,
		Cols: input.Cols, Rows: input.Rows, Actor: actor,
		Capabilities: terminalpkg.Capabilities{Interactive: true},
	})
	if err != nil {
		return toolspkg.ToolResult{}, terminalNativeError(req.ToolID, err)
	}
	return structuredResult(map[string]any{"terminal_id": handle.Info().ID}, "interactive terminal opened")
}

func (n *daemonNativeTools) terminalWrite(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	manager, actor, workspaceID, err := n.nativeTerminalContext(ctx, scope, req)
	if err != nil {
		return toolspkg.ToolResult{}, terminalNativeError(req.ToolID, err)
	}
	var input terminalWriteInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	handle, err := manager.Handle(ctx, workspaceID, actor.ProfileID, terminalpkg.ID(input.TerminalID))
	if err != nil {
		return toolspkg.ToolResult{}, terminalNativeError(req.ToolID, err)
	}
	if input.Data == "" {
		result, pollErr := n.terminalWritePoll(ctx, scope, input.TerminalID, handle)
		if pollErr != nil {
			return toolspkg.ToolResult{}, terminalNativeError(req.ToolID, pollErr)
		}
		return result, nil
	}
	if err := handle.Write(ctx, actor, []byte(input.Data)); err != nil {
		return toolspkg.ToolResult{}, terminalNativeError(req.ToolID, err)
	}
	info := handle.Info()
	return structuredResult(map[string]any{"accepted": true, "lease_state": info.Lease}, "terminal input accepted")
}

func (n *daemonNativeTools) terminalRead(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	manager, actor, workspaceID, err := n.nativeTerminalContext(ctx, scope, req)
	if err != nil {
		return toolspkg.ToolResult{}, terminalNativeError(req.ToolID, err)
	}
	var input terminalReadInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	handle, err := manager.Handle(ctx, workspaceID, actor.ProfileID, terminalpkg.ID(input.TerminalID))
	if err != nil {
		return toolspkg.ToolResult{}, terminalNativeError(req.ToolID, err)
	}
	result, err := handle.Screen(ctx, terminalpkg.ReadOptions{
		View: input.View, MaxBytes: input.MaxBytes, SinceSeq: input.SinceSeq,
		FromLine: input.From, ToLine: input.To, Grep: input.Grep,
	})
	if err != nil {
		return toolspkg.ToolResult{}, terminalNativeError(req.ToolID, err)
	}
	return structuredResult(result, "untrusted terminal output read")
}

func (n *daemonNativeTools) terminalWait(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	manager, actor, workspaceID, err := n.nativeTerminalContext(ctx, scope, req)
	if err != nil {
		return toolspkg.ToolResult{}, terminalNativeError(req.ToolID, err)
	}
	var input terminalWaitInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	handle, err := manager.Handle(ctx, workspaceID, actor.ProfileID, terminalpkg.ID(input.TerminalID))
	if err != nil {
		return toolspkg.ToolResult{}, terminalNativeError(req.ToolID, err)
	}
	result, err := handle.Wait(ctx, terminalpkg.WaitCondition{
		Until: input.Until, Pattern: input.Pattern, TimeoutMs: input.TimeoutMS,
	})
	if err != nil {
		return toolspkg.ToolResult{}, terminalNativeError(req.ToolID, err)
	}
	return structuredResult(result, "terminal wait completed")
}

func (n *daemonNativeTools) terminalSignal(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	manager, actor, workspaceID, err := n.nativeTerminalContext(ctx, scope, req)
	if err != nil {
		return toolspkg.ToolResult{}, terminalNativeError(req.ToolID, err)
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
		return toolspkg.ToolResult{}, terminalNativeError(req.ToolID, err)
	}
	return structuredResult(map[string]bool{"delivered": true}, "terminal signal delivered")
}

func (n *daemonNativeTools) terminalClose(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	manager, actor, workspaceID, err := n.nativeTerminalContext(ctx, scope, req)
	if err != nil {
		return toolspkg.ToolResult{}, terminalNativeError(req.ToolID, err)
	}
	var input terminalIDInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	exit, err := manager.Close(ctx, workspaceID, terminalpkg.ID(input.TerminalID), actor, terminalpkg.SignalHUP)
	if err != nil {
		return toolspkg.ToolResult{}, terminalNativeError(req.ToolID, err)
	}
	return structuredResult(map[string]any{"exit": exit}, "terminal closed")
}

func (n *daemonNativeTools) terminalList(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	manager, actor, workspaceID, err := n.nativeTerminalContext(ctx, scope, req)
	if err != nil {
		return toolspkg.ToolResult{}, terminalNativeError(req.ToolID, err)
	}
	items, err := manager.List(ctx, workspaceID, store.ReadScope{ProfileID: actor.ProfileID})
	if err != nil {
		return toolspkg.ToolResult{}, terminalNativeError(req.ToolID, err)
	}
	projected, err := n.terminalToolInfos(ctx, items)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(map[string]any{"terminals": projected}, fmt.Sprintf("%d terminals", len(projected)))
}

func (n *daemonNativeTools) terminalRequestInput(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	manager, actor, workspaceID, err := n.nativeTerminalContext(ctx, scope, req)
	if err != nil {
		return toolspkg.ToolResult{}, terminalNativeError(req.ToolID, err)
	}
	var input terminalRequestInputInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	handle, err := manager.Handle(ctx, workspaceID, actor.ProfileID, terminalpkg.ID(input.TerminalID))
	if err != nil {
		return toolspkg.ToolResult{}, terminalNativeError(req.ToolID, err)
	}
	outcome, err := handle.RequestInput(ctx, terminalpkg.InputRequest{
		Reason: input.Reason, PromptExcerpt: input.PromptExcerpt, Redact: input.Redact,
	})
	if err != nil {
		return toolspkg.ToolResult{}, terminalNativeError(req.ToolID, err)
	}
	return structuredResult(outcome, "terminal input request resolved")
}

func (n *daemonNativeTools) terminalYield(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	manager, actor, workspaceID, err := n.nativeTerminalContext(ctx, scope, req)
	if err != nil {
		return toolspkg.ToolResult{}, terminalNativeError(req.ToolID, err)
	}
	var input terminalYieldInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	handle, err := manager.Handle(ctx, workspaceID, actor.ProfileID, terminalpkg.ID(input.TerminalID))
	if err == nil {
		err = handle.Yield(ctx, actor)
	}
	if err != nil {
		return toolspkg.ToolResult{}, terminalNativeError(req.ToolID, err)
	}
	return structuredResult(map[string]any{"lease_state": handle.Info().Lease}, "terminal control yielded")
}

func (n *daemonNativeTools) terminalClaim(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	manager, actor, workspaceID, err := n.nativeTerminalContext(ctx, scope, req)
	if err != nil {
		return toolspkg.ToolResult{}, terminalNativeError(req.ToolID, err)
	}
	var input terminalIDInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	claimer, ok := manager.(interface {
		Claim(context.Context, string, terminalpkg.ID, terminalpkg.Actor) error
	})
	if !ok {
		return toolspkg.ToolResult{}, nativeCommandDependencyError(req.ToolID, "terminal claim is unavailable")
	}
	if err := claimer.Claim(ctx, workspaceID, terminalpkg.ID(input.TerminalID), actor); err != nil {
		return toolspkg.ToolResult{}, terminalNativeError(req.ToolID, err)
	}
	info, err := manager.Get(ctx, workspaceID, actor.ProfileID, terminalpkg.ID(input.TerminalID))
	if err != nil {
		return toolspkg.ToolResult{}, terminalNativeError(req.ToolID, err)
	}
	return structuredResult(map[string]any{"granted": true, "lease_state": info.Lease}, "terminal control claimed")
}

func (n *daemonNativeTools) nativeTerminalContext(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (terminalpkg.Manager, terminalpkg.Actor, string, error) {
	if n == nil || n.deps == nil || n.deps.Terminals == nil {
		return nil, terminalpkg.Actor{}, "", errors.New("terminal service is unavailable")
	}
	manager := n.deps.Terminals()
	if manager == nil {
		return nil, terminalpkg.Actor{}, "", errors.New("terminal service is unavailable")
	}
	workspaceID := strings.TrimSpace(scope.WorkspaceID)
	if workspaceID == "" {
		return nil, terminalpkg.Actor{}, "", &terminalpkg.Error{
			Code: "terminal_requires_workspace", Message: "terminal actions require a workspace", Err: terminalpkg.ErrRequiresWorkspace,
		}
	}
	profileID := strings.TrimSpace(scope.ProfileID)
	if profileID == "" {
		return nil, terminalpkg.Actor{}, "", errors.New("terminal actor profile is unavailable")
	}
	sessionID := firstNonEmpty(scope.SessionID, req.SessionID)
	agentName := firstNonEmpty(scope.AgentName, req.AgentName)
	generation := int64(0)
	if sessionID != "" {
		if n.deps.Sessions == nil {
			return nil, terminalpkg.Actor{}, "", errors.New("session service is unavailable")
		}
		info, err := n.deps.Sessions.Status(ctx, sessionID)
		if err != nil {
			return nil, terminalpkg.Actor{}, "", err
		}
		if info == nil {
			return nil, terminalpkg.Actor{}, "", errors.New("terminal agent session is stale")
		}
		generation = info.RuntimeGeneration
		if agentName == "" {
			agentName = info.AgentName
		}
	}
	if agentName == "" {
		return nil, terminalpkg.Actor{}, "", errors.New("terminal agent identity is unavailable")
	}
	actor := terminalpkg.Actor{
		Kind: terminalpkg.ActorKindAgent, ID: agentName, ProfileID: profileID,
		SessionID: sessionID, Generation: generation,
	}
	return manager, actor, workspaceID, nil
}

func (n *daemonNativeTools) terminalWritePoll(
	ctx context.Context,
	scope toolspkg.Scope,
	terminalID string,
	handle terminalpkg.Handle,
) (toolspkg.ToolResult, error) {
	key := strings.Join([]string{scope.ProfileID, scope.SessionID, terminalID}, "\x00")
	var since uint64
	if value, ok := n.terminalReads.Load(key); ok {
		stored, valid := value.(uint64)
		if valid {
			since = stored
		}
	}
	result, err := handle.Screen(ctx, terminalpkg.ReadOptions{View: "tail", SinceSeq: since})
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	n.terminalReads.Store(key, result.Seq)
	return structuredResult(map[string]any{
		"accepted": false, "lease_state": handle.Info().Lease,
		"content": result.Content, "seq": result.Seq, "busy": result.Busy, "untrusted": true,
	}, "untrusted terminal output polled")
}

func (n *daemonNativeTools) terminalToolInfos(ctx context.Context, items []terminalpkg.Info) ([]terminalToolInfo, error) {
	projected := make([]terminalToolInfo, 0, len(items))
	for _, item := range items {
		profileName := ""
		if n.deps.Profiles != nil {
			name, err := n.deps.Profiles.ProfileName(ctx, item.ProfileID)
			if err != nil {
				return nil, err
			}
			profileName = name
		}
		projected = append(projected, terminalToolInfo{Info: item, ProfileName: profileName})
	}
	return projected, nil
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
		return "", nativeCommandToolError(
			toolspkg.ErrorCodeDenied,
			req.ToolID,
			"approval_rejected — irreversible terminal command refused",
			toolspkg.ErrToolDenied,
			toolspkg.ReasonSessionDenied,
		)
	}
	if classification.Verdict == terminalpkg.CommandVerdictAllowlisted {
		return "allowlisted", nil
	}
	if req.ApprovalGranted {
		if label := strings.TrimSpace(req.ApprovalLabel); label != "" {
			return label, nil
		}
		return "approved_once", nil
	}
	if strings.TrimSpace(req.ApprovalToken) != "" {
		return "approved_once", nil
	}
	if n != nil && n.deps != nil && n.deps.TerminalExecApprover != nil {
		label, err := n.deps.TerminalExecApprover.ApproveTerminalExec(ctx, scope, req)
		if err != nil {
			return "", err
		}
		return label, nil
	}
	return "", nativeCommandToolError(
		toolspkg.ErrorCodeApprovalRequired,
		req.ToolID,
		"approval_required — terminal command requires operator approval",
		toolspkg.ErrToolApprovalRequired,
		toolspkg.ReasonApprovalRequired,
	)
}

func terminalNativeError(id toolspkg.ToolID, err error) error {
	var terminalErr *terminalpkg.Error
	if !errors.As(err, &terminalErr) {
		return err
	}
	message := fmt.Sprintf("%s — %s", terminalErr.Code, terminalErr.Error())
	code := toolspkg.ErrorCodeDenied
	reason := toolspkg.ReasonSessionDenied
	cause := toolspkg.ErrToolDenied
	if errors.Is(err, terminalpkg.ErrNotFound) {
		code, reason, cause = toolspkg.ErrorCodeNotFound, toolspkg.ReasonToolUnknown, toolspkg.ErrToolNotFound
	}
	return nativeCommandToolError(code, id, message, cause, reason)
}
