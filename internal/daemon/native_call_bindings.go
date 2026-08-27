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
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDAgentCall:    {call: nativeCallErrorAdapter(n.agentCall), availability: availability},
		toolspkg.ToolIDCallReturn:   {call: nativeCallErrorAdapter(n.callReturn), availability: availability},
		toolspkg.ToolIDCallAwait:    {call: nativeCallErrorAdapter(n.callAwait), availability: availability},
		toolspkg.ToolIDCallCancel:   {call: nativeCallErrorAdapter(n.callCancel), availability: availability},
		toolspkg.ToolIDCallResult:   {call: nativeCallErrorAdapter(n.callResult), availability: availability},
		toolspkg.ToolIDCallPublish:  {call: nativeCallErrorAdapter(n.callPublish), availability: availability},
		toolspkg.ToolIDAgentMessage: {call: nativeCallErrorAdapter(n.agentMessage), availability: availability},
	}
}

func nativeCallErrorAdapter(call toolspkg.NativeToolFunc) toolspkg.NativeToolFunc {
	return func(
		ctx context.Context,
		scope toolspkg.Scope,
		req toolspkg.CallRequest,
	) (toolspkg.ToolResult, error) {
		result, err := call(ctx, scope, req)
		return result, nativeCallToolError(req.ToolID, err)
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
	return toolspkg.NewToolError(
		toolspkg.ErrorCode(callErr.Code),
		id,
		callErr.Error(),
		callErr,
		toolspkg.ReasonCode(callErr.Code),
	)
}

func (n *daemonNativeTools) callsService() core.CallsService {
	if n == nil || n.deps == nil || n.deps.Calls == nil {
		return nil
	}
	return n.deps.Calls()
}
