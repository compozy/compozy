package aghsdk

import (
	"context"

	"fmt"

	"slices"
)

func (e *Extension) makeContext(request JSONRPCRequestEnvelope) ExtensionContext {
	e.mu.RLock()
	session := e.session
	e.mu.RUnlock()
	if session == nil {
		return ExtensionContext{}
	}
	return ExtensionContext{
		Request:   request,
		RequestID: cloneRawMessage(request.ID),
		Host:      e.host,
		Session:   *session,
		Logf: func(format string, args ...any) {
			if e.stderr != nil {
				fmt.Fprintf(e.stderr, format+"\n", args...)
			}
		},
	}
}

func (e *Extension) ensureReady() error {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if !e.initialized || e.session == nil {
		return NewNotInitializedError()
	}
	if e.shutdownStarted {
		return NewShutdownInProgressError(e.shutdownDeadlineMS)
	}
	return nil
}

func (e *Extension) ready() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.initialized && e.session != nil && !e.shutdownStarted
}

func (e *Extension) implementedMethodsLocked() []string {
	methods := map[string]struct{}{
		healthCheckMethod: {},
		shutdownMethod:    {},
	}
	for method := range e.handlers {
		methods[method] = struct{}{}
	}
	if len(e.toolHandlers) > 0 {
		methods[ExtensionServiceMethodProvideTools] = struct{}{}
		methods[ExtensionServiceMethodToolsCall] = struct{}{}
	}
	if len(e.watchHandlers) > 0 {
		methods[ExtensionServiceMethodWatchPoll] = struct{}{}
	}
	out := make([]string, 0, len(methods))
	for method := range methods {
		out = append(out, method)
	}
	slices.Sort(out)
	return out
}

func (e *Extension) runReadyCallbacks(ctx context.Context, session *ExtensionSession) {
	e.mu.RLock()
	callbacks := slices.Clone(e.readyCallbacks)
	host := e.host
	e.mu.RUnlock()
	for _, callback := range callbacks {
		e.runReadyCallback(ctx, callback, host, session)
	}
}

func (e *Extension) runReadyCallback(
	ctx context.Context,
	callback func(context.Context, *HostAPI, ExtensionSession) error,
	host *HostAPI,
	session *ExtensionSession,
) {
	if callback == nil || session == nil {
		return
	}
	if err := callback(ctx, host, *session); err != nil && e.stderr != nil {
		fmt.Fprintf(e.stderr, "onReady callback failed: %v\n", err)
	}
}
