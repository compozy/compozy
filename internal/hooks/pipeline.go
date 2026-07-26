package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type hookSelector[P any] func(P) []*ResolvedHook

type patchGuard[P any, R any] func(context.Context, RegisteredHook, P, R) error

type denyDetector[R any] func(R) bool

type typedNativeExecutor[P any, R any] interface {
	Executor
	ExecuteTyped(context.Context, RegisteredHook, P) (R, error)
}

// pipeline executes one sync hook chain for a concrete payload/patch pair.
type pipeline[P any, R any] struct {
	event        HookEvent
	hooksRuntime *Hooks
	hooks        hookSelector[P]
	apply        func(P, R) P
	encode       func(P) ([]byte, error)
	decode       func([]byte) (R, error)
	denied       denyDetector[R]
	guard        patchGuard[P, R]
	enter        func(context.Context, HookEvent) (context.Context, int, error)
}

func (p pipeline[P, R]) execute(ctx context.Context, payload P) (P, error) {
	result, _, err := p.executeWithDisposition(ctx, payload)
	return result, err
}

func (p pipeline[P, R]) executeWithDisposition(ctx context.Context, payload P) (P, dispatchReport, error) {
	if err := p.validate(); err != nil {
		return payload, dispatchReport{}, err
	}

	enterDispatchFn := p.enter
	if enterDispatchFn == nil {
		enterDispatchFn = enterDispatch
	}

	dispatchCtx, depth, err := enterDispatchFn(ctx, p.event)
	if err != nil {
		return payload, dispatchReport{}, err
	}

	current := payload
	report := dispatchReport{Trace: make([]hookTraceEntry, 0)}
	for _, hook := range orderedResolvedHooksIfNeeded(p.hooks(payload)) {
		if hook == nil {
			continue
		}

		next, denied, trace, err := p.executeHook(dispatchCtx, hook, current, depth)
		if trace.Hook != "" {
			report.Trace = append(report.Trace, trace)
		}
		if err != nil {
			report.FailedHook = hook.Name
			report.FailedRequired = hook.Required
			if hook.Required {
				return current, report, fmt.Errorf(
					"hooks: required hook %q failed for event %q: %w",
					hook.Name,
					p.event,
					err,
				)
			}
			continue
		}

		current = next
		if denied {
			report.Denied = true
			report.DenySource = hook.Name
			report.DenyReason = denyReasonFromRawPatch(trace.Patch)
			return current, report, nil
		}
	}

	return current, report, nil
}

func (p pipeline[P, R]) validate() error {
	if err := p.event.Validate(); err != nil {
		return err
	}
	if p.hooks == nil {
		return fmt.Errorf("hooks: pipeline for event %q requires a hook selector", p.event)
	}
	if p.apply == nil {
		return fmt.Errorf("hooks: pipeline for event %q requires an apply function", p.event)
	}
	if p.encode == nil {
		return fmt.Errorf("hooks: pipeline for event %q requires an encode function", p.event)
	}
	if p.decode == nil {
		return fmt.Errorf("hooks: pipeline for event %q requires a decode function", p.event)
	}
	return nil
}

func (p pipeline[P, R]) executeHook(
	ctx context.Context,
	hook *ResolvedHook,
	payload P,
	depth int,
) (P, bool, hookTraceEntry, error) {
	if err := validateResolvedHook(hook); err != nil {
		return payload, false, hookTraceEntry{}, err
	}

	hookCtx, cancel := hookExecutionContext(ctx, hook.Timeout)
	defer cancel()

	started := time.Now()
	p.emitDispatchEvent(
		hookCtx,
		payload,
		hook.RegisteredHook,
		DispatchPhaseStart,
		"",
		nil,
		depth,
		started.UTC(),
	)
	patch, rawPatch, err := p.runHook(hookCtx, hook.RegisteredHook, payload)
	duration := time.Since(started)
	trace := newHookTrace(hook, duration, rawPatch)
	if err != nil {
		return p.finishHookFailure(
			hookCtx,
			payload,
			hook.RegisteredHook,
			depth,
			started,
			duration,
			rawPatch,
			trace,
			HookRunOutcomeFailed,
			err,
			err,
		)
	}
	if p.guard != nil {
		if guardErr := p.guard(hookCtx, hook.RegisteredHook, payload, patch); guardErr != nil {
			outcome := HookRunOutcomeFailed
			returnErr := guardErr
			if errors.Is(guardErr, ErrHookPatchRejected) {
				outcome = HookRunOutcomeRejected
				returnErr = nil
			}
			return p.finishHookFailure(
				hookCtx,
				payload,
				hook.RegisteredHook,
				depth,
				started,
				duration,
				rawPatch,
				trace,
				outcome,
				guardErr,
				returnErr,
			)
		}
	}

	next := p.apply(payload, patch)
	denied := p.denied != nil && p.denied(patch)
	trace.Outcome = hookOutcomeFromDenied(denied)
	p.finishHookRun(
		hookCtx,
		payload,
		hook.RegisteredHook,
		trace.Outcome,
		duration,
		rawPatch,
		nil,
		depth,
		started.Add(duration).UTC(),
	)
	return next, denied, trace, nil
}

func validateResolvedHook(hook *ResolvedHook) error {
	if hook == nil {
		return errors.New("hooks: resolved hook is required")
	}
	if hook.Executor == nil {
		return fmt.Errorf("hooks: hook %q executor is required", hook.Name)
	}
	return nil
}

func hookExecutionContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

func newHookTrace(hook *ResolvedHook, duration time.Duration, rawPatch json.RawMessage) hookTraceEntry {
	return hookTraceEntry{
		Hook:     hook.Name,
		Duration: duration,
		Required: hook.Required,
		Patch:    cloneRawJSON(rawPatch),
	}
}

func hookOutcomeFromDenied(denied bool) HookRunOutcome {
	if denied {
		return HookRunOutcomeDenied
	}
	return HookRunOutcomeApplied
}

func (p pipeline[P, R]) finishHookFailure(
	ctx context.Context,
	payload P,
	hook RegisteredHook,
	depth int,
	started time.Time,
	duration time.Duration,
	rawPatch json.RawMessage,
	trace hookTraceEntry,
	outcome HookRunOutcome,
	runErr error,
	returnErr error,
) (P, bool, hookTraceEntry, error) {
	trace.Outcome = outcome
	trace.Error = runErr.Error()
	p.finishHookRun(
		ctx,
		payload,
		hook,
		trace.Outcome,
		duration,
		rawPatch,
		runErr,
		depth,
		started.Add(duration).UTC(),
	)
	return payload, false, trace, returnErr
}

func (p pipeline[P, R]) finishHookRun(
	ctx context.Context,
	payload P,
	hook RegisteredHook,
	outcome HookRunOutcome,
	duration time.Duration,
	rawPatch json.RawMessage,
	err error,
	depth int,
	timestamp time.Time,
) {
	p.recordHookRun(ctx, payload, hook, outcome, duration, rawPatch, err, depth)
	p.emitDispatchEvent(
		ctx,
		payload,
		hook,
		DispatchPhaseComplete,
		outcome,
		err,
		depth,
		timestamp,
	)
}

func (p pipeline[P, R]) emitDispatchEvent(
	ctx context.Context,
	payload P,
	hook RegisteredHook,
	phase DispatchPhase,
	outcome HookRunOutcome,
	err error,
	depth int,
	timestamp time.Time,
) {
	emitter := DispatchEventEmitterFromContext(ctx)
	if emitter == nil {
		return
	}
	emitter.EmitHookDispatchEvent(ctx, payload, hook, phase, outcome, err, depth, timestamp)
}

func (p pipeline[P, R]) runHook(ctx context.Context, hook RegisteredHook, payload P) (R, json.RawMessage, error) {
	if hook.Executor.Kind() == HookExecutorNative {
		if executor, ok := hook.Executor.(typedNativeExecutor[P, R]); ok {
			patch, err := executor.ExecuteTyped(ctx, hook, payload)
			if err != nil {
				var zero R
				return zero, nil, err
			}
			rawPatch, marshalErr := json.Marshal(patch)
			if marshalErr != nil {
				var zero R
				return zero, nil, fmt.Errorf("hooks: encode native patch for hook %q: %w", hook.Name, marshalErr)
			}
			return patch, rawPatch, nil
		}
	}

	encoded, err := p.encode(payload)
	if err != nil {
		var zero R
		return zero, nil, fmt.Errorf("hooks: encode payload for hook %q: %w", hook.Name, err)
	}

	rawPatch, err := hook.Executor.Execute(ctx, hook, encoded)
	if err != nil {
		var zero R
		return zero, nil, err
	}

	patch, err := p.decode(rawPatch)
	if err != nil {
		var zero R
		return zero, rawPatch, fmt.Errorf("hooks: decode patch for hook %q: %w", hook.Name, err)
	}

	return patch, rawPatch, nil
}

func encodeJSON[T any](payload T) ([]byte, error) {
	return json.Marshal(payload)
}

func decodeJSON[T any](payload []byte) (T, error) {
	var decoded T
	if len(bytes.TrimSpace(payload)) == 0 {
		return decoded, nil
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return decoded, err
	}
	return decoded, nil
}

func denyReasonFromRawPatch(rawPatch json.RawMessage) string {
	if len(bytes.TrimSpace(rawPatch)) == 0 {
		return ""
	}
	var patch ControlPatch
	if err := json.Unmarshal(rawPatch, &patch); err != nil {
		return ""
	}
	return strings.TrimSpace(patch.DenyReason)
}

func (p pipeline[P, R]) recordHookRun(
	ctx context.Context,
	payload P,
	hook RegisteredHook,
	outcome HookRunOutcome,
	duration time.Duration,
	rawPatch json.RawMessage,
	err error,
	depth int,
) {
	if p.hooksRuntime == nil {
		return
	}
	p.hooksRuntime.emitHookRun(ctx, payload, hook, outcome, duration, rawPatch, err, depth)
}
