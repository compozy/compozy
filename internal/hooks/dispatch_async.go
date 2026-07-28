package hooks

import (
	"context"
	"errors"
	"time"
)

var errAsyncHookDropped = errors.New("hooks: async hook submission dropped")

func submitAsyncHooks[P any, R any](
	parent context.Context,
	h *Hooks,
	payload P,
	hooks []*ResolvedHook,
	pipe pipeline[P, R],
) {
	if h == nil || h.pool == nil {
		return
	}

	parentDepth := currentDispatchDepth(parent)
	parentChain := currentDispatchChain(parent)
	for _, hook := range hooks {
		submitAsyncHook(parent, h, payload, parentDepth, parentChain, hook, pipe)
	}
}

func submitAsyncHook[P any, R any](
	parent context.Context,
	h *Hooks,
	payload P,
	parentDepth int,
	parentChain []HookEvent,
	hook *ResolvedHook,
	pipe pipeline[P, R],
) {
	if hook == nil {
		return
	}

	asyncHook := *hook
	asyncPayload := cloneAsyncPayload(payload)
	task := asyncTask{
		hook: asyncHook.RegisteredHook,
		run:  buildAsyncHookRunner(parent, h, asyncPayload, &asyncHook, pipe, parentDepth, parentChain),
	}
	if h.pool.Submit(task) {
		return
	}

	h.emitHookRun(
		parent,
		asyncPayload,
		asyncHook.RegisteredHook,
		HookRunOutcomeDropped,
		0,
		nil,
		errAsyncHookDropped,
		parentDepth,
	)
}

func buildAsyncHookRunner[P any, R any](
	parent context.Context,
	h *Hooks,
	payload P,
	hook *ResolvedHook,
	pipe pipeline[P, R],
	parentDepth int,
	parentChain []HookEvent,
) func(context.Context) {
	return func(poolCtx context.Context) {
		if hook == nil {
			return
		}

		baseCtx, cancelBase := context.WithCancel(parent)
		stopPoolCancel := context.AfterFunc(poolCtx, cancelBase)
		defer func() {
			stopPoolCancel()
			cancelBase()
		}()

		baseCtx = context.WithValue(baseCtx, dispatchDepthContextKey{}, parentDepth)
		baseCtx = context.WithValue(baseCtx, dispatchChainContextKey{}, parentChain)
		hookCtx, depth, err := h.enterDispatch(baseCtx, hook.Event)
		if err != nil {
			h.emitHookRun(baseCtx, payload, hook.RegisteredHook, HookRunOutcomeSkipped, 0, nil, err, parentDepth)
			return
		}

		cancel := func() {}
		if hook.Timeout > 0 {
			hookCtx, cancel = context.WithTimeout(hookCtx, hook.Timeout)
		}
		defer cancel()

		started := time.Now()
		_, rawPatch, err := pipe.runHook(hookCtx, hook.RegisteredHook, payload)
		duration := time.Since(started)
		if err != nil {
			h.emitHookRun(hookCtx, payload, hook.RegisteredHook, HookRunOutcomeFailed, duration, rawPatch, err, depth)
			h.logger.WarnContext(
				hookCtx,
				"hook.dispatch.async_failed",
				"hook", hook.Name,
				"event", hook.Event.String(),
				"source", hook.Source.String(),
				"error", err,
			)
			return
		}

		h.emitHookRun(hookCtx, payload, hook.RegisteredHook, HookRunOutcomeApplied, duration, rawPatch, nil, depth)
	}
}
