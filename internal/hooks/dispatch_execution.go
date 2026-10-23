package hooks

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

func (h *Hooks) executeDispatch[P any, R any](
	ctx context.Context,
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

	syncHooks, asyncHooks, err := h.matchingDispatchHooks(event, payload, cfg.match)
	if err != nil {
		return payload, err
	}
	if len(syncHooks) == 0 && len(asyncHooks) == 0 {
		return payload, nil
	}

	dispatchDepth := currentDispatchDepth(ctx) + 1
	dispatchStarted := time.Now()
	logger := h.dispatchLogger(payload)
	logger.DebugContext(ctx,
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
		decode:       decodeJSONStrict[R],
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
		h.submitAsyncHooks(ctx, result, asyncHooks, pipe)
	}

	h.reportDispatchResult(
		logger,
		event,
		dispatchDepth,
		dispatchStarted,
		report,
		dispatchErr,
		len(syncHooks),
		len(asyncHooks),
	)

	return result, dispatchErr
}

func (h *Hooks) matchingDispatchHooks[P any](
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

func (h *Hooks) reportDispatchResult(
	logger *slog.Logger,
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
		logger.Warn(
			"hook.dispatch.blocked",
			"event", event.String(),
			"dispatch_depth", dispatchDepth,
			"deny_source", report.DenySource,
			"deny_reason", report.DenyReason,
			"pipeline_trace", traceStrings(report.Trace),
		)
	case dispatchErr != nil || report.FailedHook != "":
		logger.Warn(
			"hook.dispatch.failed",
			"event", event.String(),
			"dispatch_depth", dispatchDepth,
			"error", dispatchErr,
			"failed_hook", report.FailedHook,
			"required", report.FailedRequired,
			"pipeline_trace", traceStrings(report.Trace),
		)
	default:
		logger.Debug(
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

func (h *Hooks) dispatchLogger(payload any) *slog.Logger {
	session := SessionContextFromPayload(payload)
	correlation := CorrelationFromPayload(payload)
	attrs := make([]any, 0, 16)
	for _, field := range []struct{ key, value string }{
		{"session_id", session.SessionID},
		{matcherWorkspaceIDKey, session.WorkspaceID},
		{matcherTurnIDKey, TurnIDFromPayload(payload)},
		{matcherTaskIDKey, correlation.TaskID},
		{matcherRunIDKey, correlation.RunID},
		{matcherLoopRunIDKey, correlation.LoopRunID},
		{matcherCoordinatorSessionIDKey, correlation.CoordinatorSessionID},
		{"actor_id", correlation.ActorID},
	} {
		if field.value != "" {
			attrs = append(attrs, field.key, field.value)
		}
	}
	return h.logger.With(attrs...)
}

func hookDeniedError(event HookEvent, reason string) error {
	return hookDeniedByError(event, "", reason)
}

// DeniedError reports an explicit sync-hook denial separately from execution failures.
type DeniedError struct {
	Event    HookEvent
	HookName string
	Reason   string
}

func (e *DeniedError) Error() string {
	if e == nil {
		return "hooks: event denied"
	}
	reason := strings.TrimSpace(e.Reason)
	if reason == "" {
		return fmt.Sprintf("hooks: event %q denied", e.Event)
	}
	return fmt.Sprintf("hooks: event %q denied: %s", e.Event, reason)
}

func hookDeniedByError(event HookEvent, hookName string, reason string) error {
	reason = strings.TrimSpace(reason)
	return &DeniedError{Event: event, HookName: strings.TrimSpace(hookName), Reason: reason}
}
