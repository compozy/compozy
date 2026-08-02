package compozysdk

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"
)

type readyCallbackLifecycle struct {
	ctx    context.Context
	cancel context.CancelFunc

	mu        sync.Mutex
	accepting bool
	running   int
	errs      []error
	done      chan struct{}
}

type readyCallbackDrainResult struct {
	callbackErr error
	waitErr     error
}

func newReadyCallbackLifecycle() *readyCallbackLifecycle {
	ctx, cancel := context.WithCancel(context.Background())
	return &readyCallbackLifecycle{
		ctx:       ctx,
		cancel:    cancel,
		accepting: true,
		done:      make(chan struct{}),
	}
}

func (l *readyCallbackLifecycle) start(run func(context.Context) error) {
	if l == nil || run == nil {
		return
	}
	l.mu.Lock()
	if !l.accepting {
		l.mu.Unlock()
		return
	}
	l.running++
	ctx := l.ctx
	l.mu.Unlock()
	go func() {
		var err error
		returned := false
		defer func() {
			if recovered := recover(); recovered != nil {
				err = fmt.Errorf("extension: onReady callback panic: %v", recovered)
			} else if !returned {
				err = errors.New("extension: onReady callback exited without returning")
			}
			l.complete(err)
		}()
		err = run(ctx)
		returned = true
	}()
}

func (l *readyCallbackLifecycle) stop() {
	if l == nil {
		return
	}
	l.mu.Lock()
	if !l.accepting {
		l.mu.Unlock()
		return
	}
	l.accepting = false
	if l.running == 0 {
		close(l.done)
	}
	cancel := l.cancel
	l.mu.Unlock()
	cancel()
}

func (l *readyCallbackLifecycle) complete(err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.running--
	if err != nil {
		l.errs = append(l.errs, err)
	}
	if !l.accepting && l.running == 0 {
		close(l.done)
	}
}

func (l *readyCallbackLifecycle) drain(ctx context.Context) readyCallbackDrainResult {
	if l == nil {
		return readyCallbackDrainResult{}
	}
	for {
		l.mu.Lock()
		switch {
		case l.completedLocked():
			result := l.snapshotLocked(nil)
			l.mu.Unlock()
			return result
		case ctx.Err() != nil:
			result := l.snapshotLocked(ctx.Err())
			l.mu.Unlock()
			return result
		default:
			done := l.done
			ctxDone := ctx.Done()
			l.mu.Unlock()
			select {
			case <-done:
			case <-ctxDone:
			}
		}
	}
}

func (l *readyCallbackLifecycle) completedLocked() bool {
	if l.accepting || l.running != 0 {
		return false
	}
	select {
	case <-l.done:
		return true
	default:
		return false
	}
}

func (l *readyCallbackLifecycle) snapshotLocked(waitErr error) readyCallbackDrainResult {
	return readyCallbackDrainResult{
		callbackErr: errors.Join(slices.Clone(l.errs)...),
		waitErr:     waitErr,
	}
}

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

func (e *Extension) startReadyCallbacksLocked(session *ExtensionSession) {
	callbacks := slices.Clone(e.readyCallbacks)
	host := e.host
	for _, callback := range callbacks {
		e.startReadyCallback(callback, host, session)
	}
}

func (e *Extension) startReadyCallback(
	callback func(context.Context, *HostAPI, ExtensionSession) error,
	host *HostAPI,
	session *ExtensionSession,
) {
	if e == nil || e.readyLifecycle == nil || callback == nil {
		return
	}
	e.readyLifecycle.start(func(ctx context.Context) error {
		return e.runReadyCallback(ctx, callback, host, session)
	})
}

func (e *Extension) runReadyCallback(
	ctx context.Context,
	callback func(context.Context, *HostAPI, ExtensionSession) error,
	host *HostAPI,
	session *ExtensionSession,
) error {
	if callback == nil || session == nil {
		return nil
	}
	callbackErr := callback(ctx, host, *session)
	if cancellationErr := ctx.Err(); cancellationErr != nil {
		return withoutCancellationError(callbackErr, cancellationErr)
	}
	return callbackErr
}

func withoutCancellationError(err error, cancellationErr error) error {
	if err == nil {
		return nil
	}
	changed, retained := filterCancellationError(err, cancellationErr)
	if !changed {
		return err
	}
	return retained
}

func filterCancellationError(err error, cancellationErr error) (bool, error) {
	if err == nil {
		return false, nil
	}
	if multi, ok := err.(interface{ Unwrap() []error }); ok {
		unwrapped := multi.Unwrap()
		filtered := make([]error, 0, len(unwrapped))
		changed := false
		for _, child := range unwrapped {
			childChanged, retained := filterCancellationError(child, cancellationErr)
			changed = changed || childChanged
			if retained != nil {
				filtered = append(filtered, retained)
			}
		}
		if !changed {
			return false, err
		}
		return true, errors.Join(filtered...)
	}
	if single, ok := err.(interface{ Unwrap() error }); ok {
		changed, retained := filterCancellationError(single.Unwrap(), cancellationErr)
		if changed {
			return true, retained
		}
	}
	if errors.Is(err, cancellationErr) {
		return true, nil
	}
	return false, err
}

func (e *Extension) stopReadyCallbacks() error {
	if e == nil {
		return nil
	}
	e.mu.Lock()
	e.readyCallbacksClosed = true
	readyLifecycle := e.readyLifecycle
	session := e.session
	shutdownDeadlineAt := e.shutdownDeadlineAt
	e.mu.Unlock()
	if readyLifecycle == nil {
		return nil
	}
	readyLifecycle.stop()
	if session == nil {
		return nil
	}
	shutdownCtx, cancel, err := readyCallbackShutdownContext(shutdownDeadlineAt, session.Runtime)
	if err != nil {
		return err
	}
	defer cancel()
	drainResult := readyLifecycle.drain(shutdownCtx)
	if drainResult.waitErr != nil {
		return errors.Join(
			fmt.Errorf("extension: wait for ready callbacks: %w", drainResult.waitErr),
			drainResult.callbackErr,
		)
	}
	if drainResult.callbackErr != nil {
		return fmt.Errorf("extension: onReady callback: %w", drainResult.callbackErr)
	}
	return nil
}

func readyCallbackShutdownContext(
	shutdownDeadlineAt time.Time,
	runtime InitializeRuntime,
) (context.Context, context.CancelFunc, error) {
	if !shutdownDeadlineAt.IsZero() {
		ctx, cancel := context.WithDeadline(context.Background(), shutdownDeadlineAt)
		return ctx, cancel, nil
	}
	timeout, ok := millisecondsDuration(runtime.ShutdownTimeoutMS)
	if !ok {
		return nil, nil, errors.New("extension: initialized shutdown timeout is invalid")
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	return ctx, cancel, nil
}
