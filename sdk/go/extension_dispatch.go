package aghsdk

import (
	"context"
	"encoding/json"
)

func (e *Extension) dispatch(
	ctx context.Context,
	params json.RawMessage,
	request JSONRPCRequestEnvelope,
) (any, error) {
	switch request.Method {
	case initializeMethod:
		return e.handleInitialize(params)
	case healthCheckMethod:
		if err := e.ensureReady(); err != nil {
			return nil, err
		}
		return e.handleHealthCheck(ctx, request, params)
	case shutdownMethod:
		if err := e.ensureReady(); err != nil {
			return nil, err
		}
		return e.handleShutdown(ctx, request, params)
	case ExtensionServiceMethodProvideTools:
		if err := e.ensureReady(); err != nil {
			return nil, err
		}
		return ExtensionProvideToolsResponse{Tools: e.GetToolDescriptors()}, nil
	case ExtensionServiceMethodToolsCall:
		if err := e.ensureReady(); err != nil {
			return nil, err
		}
		return e.handleToolCall(ctx, request, params)
	case ExtensionServiceMethodWatchPoll:
		if err := e.ensureReady(); err != nil {
			return nil, err
		}
		return e.handleWatchPoll(ctx, request, params)
	default:
		if err := e.ensureReady(); err != nil {
			return nil, err
		}
		return e.handleUserMethod(ctx, request, params)
	}
}
