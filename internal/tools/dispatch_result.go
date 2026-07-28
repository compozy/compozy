package tools

import (
	"context"
	"errors"
	"time"
)

func (r *RuntimeRegistry) completeDispatch(
	ctx context.Context,
	scope Scope,
	target *dispatchTarget,
	req CallRequest,
	started time.Time,
	providerResult ToolResult,
) (ToolResult, error) {
	sanitized, err := r.resultProcessor().Sanitize(ctx, target.descriptor, providerResult)
	if err != nil {
		return r.failResultProcessing(ctx, target, req, started, ToolResult{}, err)
	}
	patched, err := r.runPostCallHook(ctx, target, req, sanitized)
	if err != nil {
		return ToolResult{}, r.failDispatch(ctx, target, req, started, err, ToolCallDenied)
	}
	finalized, err := r.resultProcessor().Finalize(ctx, scope, target.descriptor, patched)
	if err != nil {
		return r.failResultProcessing(ctx, target, req, started, finalized, err)
	}
	if err := r.emit(ctx, target, req, ToolCallCompleted, ToolEventData{
		StartedAt: started,
		Result:    finalized,
	}); err != nil {
		return ToolResult{}, err
	}
	if finalized.Truncated {
		if err := r.emit(ctx, target, req, ToolResultTruncated, ToolEventData{
			StartedAt: started,
			Result:    finalized,
		}); err != nil {
			return ToolResult{}, err
		}
	}
	return finalized, nil
}

func (r *RuntimeRegistry) failResultProcessing(
	ctx context.Context,
	target *dispatchTarget,
	req CallRequest,
	started time.Time,
	partial ToolResult,
	err error,
) (ToolResult, error) {
	normalized := normalizeToolError(target.descriptor.ID, err)
	if embedded, ok := partialResultFromError(normalized); ok {
		partial = embedded
	}
	if hookErr := r.runPostErrorHook(ctx, target, req, normalized); hookErr != nil {
		normalized = inheritPartialResult(hookErr, normalized)
	}
	return partial, r.failDispatchWithResult(
		ctx,
		target,
		req,
		started,
		partial,
		normalized,
		ToolCallFailed,
	)
}

func (r *RuntimeRegistry) runPostCallHook(
	ctx context.Context,
	target *dispatchTarget,
	req CallRequest,
	result ToolResult,
) (ToolResult, error) {
	if r.hooks == nil {
		return result, nil
	}
	patched, err := r.hooks.PostCall(ctx, req, result)
	if err != nil {
		return result, normalizeHookError(target.descriptor.ID, err)
	}
	return patched, nil
}

func (r *RuntimeRegistry) runPostErrorHook(
	ctx context.Context,
	target *dispatchTarget,
	req CallRequest,
	callErr error,
) error {
	if r.hooks == nil {
		return nil
	}
	if err := r.hooks.PostError(ctx, req, callErr); err != nil {
		return normalizeHookError(target.descriptor.ID, err)
	}
	return nil
}

func (r *RuntimeRegistry) failDispatch(
	ctx context.Context,
	target *dispatchTarget,
	req CallRequest,
	started time.Time,
	err error,
	kind ToolCallEventKind,
) error {
	return r.failDispatchWithResult(ctx, target, req, started, ToolResult{}, err, kind)
}

func (r *RuntimeRegistry) failDispatchWithResult(
	ctx context.Context,
	target *dispatchTarget,
	req CallRequest,
	started time.Time,
	result ToolResult,
	err error,
	kind ToolCallEventKind,
) error {
	normalized := normalizeToolError(target.descriptor.ID, err)
	if emitErr := r.emit(ctx, target, req, kind, ToolEventData{
		StartedAt: started,
		Result:    result,
		Err:       normalized,
	}); emitErr != nil {
		return emitErr
	}
	return normalized
}

func (r *RuntimeRegistry) resultProcessor() ResultProcessor {
	if r.processor != nil {
		return r.processor
	}
	return NewResultProcessor(r.defaultMaxResultBytes, nil, r.sensitiveFields...)
}

func partialResultFromError(err error) (ToolResult, bool) {
	toolErr, ok := errors.AsType[*ToolError](err)
	if !ok || toolErr.PartialResult == nil {
		return ToolResult{}, false
	}
	return cloneToolResult(*toolErr.PartialResult), true
}

func inheritPartialResult(target error, source error) error {
	partial, ok := partialResultFromError(source)
	if !ok {
		return target
	}
	toolErr, ok := errors.AsType[*ToolError](target)
	if !ok || toolErr.PartialResult != nil {
		return target
	}
	return toolErr.WithPartialResult(partial)
}
