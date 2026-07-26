package aghsdk

import (
	"context"
	"encoding/json"

	"strings"
)

func (e *Extension) handleToolCall(
	ctx context.Context,
	request JSONRPCRequestEnvelope,
	params json.RawMessage,
) (ExtensionToolCallResponse, error) {
	var call ExtensionToolCallRequest
	if err := json.Unmarshal(params, &call); err != nil {
		return ExtensionToolCallResponse{}, NewInvalidParamsError(
			"tools/call params must be an object",
			map[string]any{extensionErrorKey: err.Error()},
		)
	}
	call.Handler = strings.TrimSpace(call.Handler)
	if call.Handler == "" {
		return ExtensionToolCallResponse{}, NewInvalidParamsError("handler is required", nil)
	}
	if err := call.ToolID.Validate(); err != nil {
		return ExtensionToolCallResponse{}, err
	}
	e.mu.RLock()
	registered, ok := e.toolHandlers[call.Handler]
	e.mu.RUnlock()
	if !ok {
		return ExtensionToolCallResponse{}, NewMethodNotFoundError(call.Handler)
	}
	if registered.descriptor.ID != call.ToolID {
		return ExtensionToolCallResponse{}, NewInvalidParamsError("tool_id does not match handler", map[string]any{
			"expected_tool_id":  registered.descriptor.ID,
			"actual_tool_id":    call.ToolID,
			extensionHandlerKey: call.Handler,
		})
	}
	contextValue := e.makeContext(request)
	result, err := registered.handler(ctx, rawToolRequest{
		input:        cloneRawMessage(call.Input),
		context:      contextValue,
		toolID:       call.ToolID,
		handler:      call.Handler,
		invocationID: strings.TrimSpace(call.InvocationID),
	})
	if err != nil {
		if rpcErr := ensureRPCErrorIfTyped(err); rpcErr != nil {
			return ExtensionToolCallResponse{}, rpcErr
		}
		return ExtensionToolCallResponse{}, toolExecutionError(err, call, registered.sensitiveInputFields)
	}
	return ExtensionToolCallResponse{Result: result}, nil
}

func (e *Extension) handleUserMethod(
	ctx context.Context,
	request JSONRPCRequestEnvelope,
	params json.RawMessage,
) (any, error) {
	e.mu.RLock()
	handler := e.handlers[request.Method]
	e.mu.RUnlock()
	if handler == nil {
		return nil, NewMethodNotFoundError(request.Method)
	}
	return handler(ctx, e.makeContext(request), params)
}
