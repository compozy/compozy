package daemon

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/api/core"
	callspkg "github.com/compozy/compozy/internal/calls"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) callToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	bind := func(call toolspkg.NativeToolFunc) toolspkg.NativeToolFunc {
		return func(
			ctx context.Context,
			scope toolspkg.Scope,
			req toolspkg.CallRequest,
		) (toolspkg.ToolResult, error) {
			result, err := call(ctx, scope, req)
			return result, nativeCallToolError(req.ToolID, err)
		}
	}
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDAgentCall:    {call: bind(n.agentCall), availability: availability},
		toolspkg.ToolIDCallReturn:   {call: bind(n.callReturn), availability: availability},
		toolspkg.ToolIDCallAwait:    {call: bind(n.callAwait), availability: availability},
		toolspkg.ToolIDCallCancel:   {call: bind(n.callCancel), availability: availability},
		toolspkg.ToolIDCallResult:   {call: bind(n.callResult), availability: availability},
		toolspkg.ToolIDCallPublish:  {call: bind(n.callPublish), availability: availability},
		toolspkg.ToolIDAgentMessage: {call: bind(n.agentMessage), availability: availability},
	}
}

func nativeCallToolError(id toolspkg.ToolID, err error) error {
	if err == nil {
		return nil
	}
	if toolErr, ok := errors.AsType[*toolspkg.ToolError](err); ok {
		return toolErr
	}
	callErr, ok := errors.AsType[*callspkg.Error](err)
	if !ok {
		return err
	}
	code := toolspkg.ErrorCodeConflict
	reasons := make([]toolspkg.ReasonCode, 0, 1)
	switch callErr.Code {
	case callspkg.CodeTargetDenied, callspkg.CodeSettlementDenied, callspkg.CodeMessageTargetDenied,
		callspkg.CodeMessageTargetBlocked, callspkg.CodeWideningRejected:
		code = toolspkg.ErrorCodeDenied
		reasons = append(reasons, toolspkg.ReasonSessionDenied)
	case callspkg.CodeWorkspaceDenied:
		code = toolspkg.ErrorCodeDenied
		reasons = append(reasons, toolspkg.ReasonWorkspaceAccessDenied)
	case callspkg.CodeNotFound, callspkg.CodeAgentUnknown, callspkg.CodeMessageNotFound:
		code = toolspkg.ErrorCodeNotFound
	case callspkg.CodeValidation, callspkg.CodeExpectInvalid, callspkg.CodePromptRequired,
		callspkg.CodeDeadlineInvalid, callspkg.CodeResultInvalid, callspkg.CodeResultOverBudget,
		callspkg.CodeMessageTooLarge:
		code = toolspkg.ErrorCodeInvalidInput
		reasons = append(reasons, toolspkg.ReasonSchemaInvalid)
	}
	return toolspkg.NewToolError(code, id, callErr.Error(), callErr, reasons...)
}

func (n *daemonNativeTools) callsService() core.CallsService {
	if n == nil || n.deps == nil || n.deps.Calls == nil {
		return nil
	}
	return n.deps.Calls()
}
