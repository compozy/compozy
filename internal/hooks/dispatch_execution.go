package hooks

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

func executeDispatch[P any, R any](
	ctx context.Context,
	h *Hooks,
	event HookEvent,
	payload P,
	cfg dispatchConfig[P, R],
) (P, error) {
	if h == nil {
		return payload, errors.New("hooks: dispatcher is nil")
	}
	if ctx == nil {
		return payload, errors.New("hooks: dispatch context is nil")
	}

	syncHooks, asyncHooks, err := matchingDispatchHooks(h, event, payload, cfg.match)
	if err != nil {
		return payload, err
	}
	if len(syncHooks) == 0 && len(asyncHooks) == 0 {
		return payload, nil
	}

	dispatchDepth := currentDispatchDepth(ctx) + 1
	dispatchStarted := time.Now()
	h.logger.Info(
		"hook.dispatch.started",
		"event", event.String(),
		"dispatch_depth", dispatchDepth,
		"sync_hooks", len(syncHooks),
		"async_hooks", len(asyncHooks),
	)

	result := payload
	var dispatchErr error
	pipe := pipeline[P, R]{
		event:        event,
		hooksRuntime: h,
		hooks:        func(P) []*ResolvedHook { return syncHooks },
		apply:        cfg.apply,
		encode:       encodeJSON[P],
		decode:       decodeJSON[R],
		denied:       cfg.denied,
		guard:        cfg.guard,
		enter:        h.enterDispatch,
	}
	var report dispatchReport
	if len(syncHooks) > 0 {
		result, report, dispatchErr = pipe.executeWithDisposition(ctx, payload)
		if dispatchErr == nil && report.Denied && cfg.denyErr != nil {
			dispatchErr = cfg.denyErr(result, report)
		}
	}

	if dispatchErr == nil && !report.Denied && len(asyncHooks) > 0 {
		submitAsyncHooks(ctx, h, result, asyncHooks, pipe)
	}

	reportDispatchResult(h, event, dispatchDepth, dispatchStarted, report, dispatchErr, len(syncHooks), len(asyncHooks))

	return result, dispatchErr
}

func matchingDispatchHooks[P any](
	h *Hooks,
	event HookEvent,
	payload P,
	match matcherFunc[P],
) ([]*ResolvedHook, []*ResolvedHook, error) {
	snapshot, err := h.hookSnapshot(event)
	if err != nil {
		return nil, nil, err
	}
	syncHooks, asyncHooks := selectMatchingHooks(snapshot, payload, match)
	return syncHooks, asyncHooks, nil
}

func reportDispatchResult(
	h *Hooks,
	event HookEvent,
	dispatchDepth int,
	dispatchStarted time.Time,
	report dispatchReport,
	dispatchErr error,
	syncHookCount int,
	asyncHookCount int,
) {
	pipelineDuration := time.Since(dispatchStarted)
	h.metrics.observePipeline(event, pipelineDuration)

	switch {
	case report.Denied:
		h.logger.Warn(
			"hook.dispatch.blocked",
			"event", event.String(),
			"dispatch_depth", dispatchDepth,
			"deny_source", report.DenySource,
			"deny_reason", report.DenyReason,
			"pipeline_trace", traceStrings(report.Trace),
		)
	case dispatchErr != nil:
		h.logger.Warn(
			"hook.dispatch.failed",
			"event", event.String(),
			"dispatch_depth", dispatchDepth,
			"error", dispatchErr,
			"failed_hook", report.FailedHook,
			"required", report.FailedRequired,
			"pipeline_trace", traceStrings(report.Trace),
		)
	default:
		h.logger.Info(
			"hook.dispatch.completed",
			"event", event.String(),
			"dispatch_depth", dispatchDepth,
			"duration_ms", pipelineDuration.Milliseconds(),
			"pipeline_trace", traceStrings(report.Trace),
			"sync_hooks", syncHookCount,
			"async_hooks", asyncHookCount,
		)
	}
}

func hookDeniedError(event HookEvent, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return fmt.Errorf("hooks: event %q denied", event)
	}
	return fmt.Errorf("hooks: event %q denied: %s", event, reason)
}
