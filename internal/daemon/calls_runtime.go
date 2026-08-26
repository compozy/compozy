package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	callspkg "github.com/compozy/compozy/internal/calls"
	"github.com/compozy/compozy/internal/store"
)

const callDispatchInterval = time.Second

type callRuntime struct {
	service *callspkg.Service
	ctx     context.Context
	logger  *slog.Logger
	cancel  context.CancelFunc
	done    chan struct{}
}

func (d *Daemon) bootCalls(ctx context.Context, state *bootState, cleanup *bootCleanup) error {
	if state == nil || state.tasks == nil || state.tasks.manager == nil || state.sessions == nil {
		return nil
	}
	callStore, ok := state.registry.(callspkg.Store)
	if !ok {
		return nil
	}
	callSessions, ok := state.sessions.(callSessionManager)
	if !ok {
		return nil
	}
	directory := &daemonCallDirectory{store: callStore, state: state}
	invoker := &daemonCallSessionInvoker{
		sessions: callSessions, maxChildren: state.cfg.Calls.MaxChildren, maxDepth: state.cfg.Calls.MaxDepth,
	}
	service, err := callspkg.NewService(
		callspkg.WithStore(callStore),
		callspkg.WithDirectory(directory),
		callspkg.WithActivationClaimer(state.tasks.manager),
		callspkg.WithActivationRunCanceler(state.tasks.manager),
		callspkg.WithSessionInvoker(invoker),
		callspkg.WithPublishBridge(&daemonCallPublishBridge{network: func() coreNetworkSender {
			return state.network
		}}),
		callspkg.WithHookDispatcher(daemonCallHookDispatcher{
			state: state, logger: state.logger, now: d.now,
		}),
		callspkg.WithConfig(state.cfg.Calls),
		callspkg.WithClock(d.now),
		callspkg.WithIDGenerator(store.NewID),
	)
	if err != nil {
		return fmt.Errorf("daemon: create calls service: %w", err)
	}
	if err := service.RecoverCallRuntime(ctx); err != nil {
		return fmt.Errorf("daemon: recover calls runtime: %w", err)
	}
	if err := service.DrainDeliveries(ctx, "", 100); err != nil {
		logger := state.logger
		if logger == nil {
			logger = slog.Default()
		}
		logger.Warn("daemon: call deliveries remain pending after recovery", "error", err)
	}
	runCtx, cancel := context.WithCancel(ctx)
	runtime := &callRuntime{
		service: service, ctx: runCtx, logger: state.logger,
		cancel: cancel, done: make(chan struct{}),
	}
	state.calls = runtime
	state.deps.Calls = newCallSurfaceService(service, state.sessions)
	if registrar, ok := state.sessions.(turnEndNotifierRegistrar); ok {
		registrar.AddTurnEndNotifier(runtime.onTurnEnd)
	}
	go runtime.run(runCtx, state.logger)
	cleanup.add(runtime.shutdown)
	return nil
}

func (r *callRuntime) run(ctx context.Context, logger *slog.Logger) {
	defer close(r.done)
	if logger == nil {
		logger = slog.Default()
	}
	ticker := time.NewTicker(callDispatchInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if _, err := r.service.DispatchQueued(ctx, 100); err != nil && !errors.Is(err, context.Canceled) {
				logger.Warn("daemon: dispatch queued calls failed", "error", err)
			}
			if _, err := r.service.SweepDeadlines(ctx, now); err != nil && !errors.Is(err, context.Canceled) {
				logger.Warn("daemon: sweep call deadlines failed", "error", err)
			}
			if err := r.service.DrainDeliveries(ctx, "", 100); err != nil && !errors.Is(err, context.Canceled) {
				logger.Warn("daemon: drain call deliveries failed", "error", err)
			}
		}
	}
}

func (r *callRuntime) onTurnEnd(sessionID string) {
	if r == nil || r.service == nil || r.ctx == nil {
		return
	}
	if err := r.service.DrainDeliveries(r.ctx, sessionID, 100); err != nil &&
		!errors.Is(err, context.Canceled) {
		logger := r.logger
		if logger == nil {
			logger = slog.Default()
		}
		logger.Warn("daemon: drain call deliveries at turn boundary failed", "session_id", sessionID, "error", err)
	}
}

func (r *callRuntime) shutdown(ctx context.Context) error {
	if r == nil {
		return nil
	}
	r.cancel()
	select {
	case <-r.done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("daemon: stop calls runtime: %w", ctx.Err())
	}
}
