package tools

import (
	"context"
	"errors"
)

func (r *RuntimeRegistry) bindCallInput(
	ctx context.Context,
	scope Scope,
	descriptor Descriptor,
	req CallRequest,
) (CallRequest, error) {
	if r.callInputBinder == nil {
		return req, nil
	}
	bound, err := r.callInputBinder.BindCallInput(ctx, scope, cloneDescriptor(descriptor), req.Input)
	if err != nil {
		return req, normalizeToolError(descriptor.ID, err)
	}
	req.Input = normalizeCallInput(bound)
	return req, nil
}

func callInputFailureEvent(err error) ToolCallEventKind {
	if errors.Is(err, ErrToolDenied) {
		return ToolCallDenied
	}
	return ToolCallFailed
}
