package automation

import (
	"context"

	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"

	"github.com/compozy/agh/internal/session"
)

func (m *Manager) fireHookRecord(ctx context.Context, sessionID string, record hookspkg.HookRunRecord) error {
	engine, runtimeCtx, ok := m.triggerRuntime()
	if !ok {
		return nil
	}
	mergedCtx, cancel := mergedRuntimeContext(ctx, runtimeCtx)
	defer cancel()
	_, err := engine.FireHookCompletion(mergedCtx, sessionID, record)
	return err
}

func (m *Manager) fireMemoryConsolidated(ctx context.Context, event MemoryConsolidatedEvent) error {
	engine, runtimeCtx, ok := m.triggerRuntime()
	if !ok {
		return nil
	}
	mergedCtx, cancel := mergedRuntimeContext(ctx, runtimeCtx)
	defer cancel()
	_, err := engine.FireMemoryConsolidated(mergedCtx, event)
	return err
}

func mergedRuntimeContext(parent context.Context, runtimeCtx context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		if runtimeCtx == nil {
			return nil, func() {}
		}
		return context.WithCancel(runtimeCtx)
	}
	if runtimeCtx == nil {
		return parent, func() {}
	}

	mergedCtx, cancel := context.WithCancel(context.WithoutCancel(parent))
	stopParent := context.AfterFunc(parent, cancel)
	stopRuntime := context.AfterFunc(runtimeCtx, cancel)

	if err := parent.Err(); err != nil {
		cancel()
	}
	if err := runtimeCtx.Err(); err != nil {
		cancel()
	}

	return mergedCtx, func() {
		stopParent()
		stopRuntime()
		cancel()
	}
}

func earliestNextFire(states []ScheduledJobState) (time.Time, bool) {
	var (
		next time.Time
		set  bool
	)
	for _, state := range states {
		if state.NextRun == nil || state.NextRun.IsZero() {
			continue
		}
		if !set || state.NextRun.Before(next) {
			next = *state.NextRun
			set = true
		}
	}
	return next, set
}

type managerSessionObserver struct {
	manager *Manager
}

func (o managerSessionObserver) OnSessionCreated(ctx context.Context, sess *session.Session) {
	if o.manager != nil {
		o.manager.fireSessionCreated(ctx, sess)
	}
}

func (o managerSessionObserver) OnSessionStopped(ctx context.Context, sess *session.Session) {
	if o.manager != nil {
		if sess != nil {
			o.manager.DeleteAutomationSessionTaskActor(sess.ID)
		}
		o.manager.fireSessionStopped(ctx, sess)
	}
}

func (managerSessionObserver) OnAgentEvent(context.Context, string, any) {
}

type managerHookTelemetrySink struct {
	manager *Manager
}

func (s managerHookTelemetrySink) WriteHookRecord(
	ctx context.Context,
	sessionID string,
	record hookspkg.HookRunRecord,
) error {
	if s.manager == nil {
		return nil
	}
	return s.manager.fireHookRecord(ctx, sessionID, record)
}

type managerMemoryObserver struct {
	manager *Manager
}

func (o managerMemoryObserver) OnMemoryConsolidated(ctx context.Context, event MemoryConsolidatedEvent) error {
	if o.manager == nil {
		return nil
	}
	return o.manager.fireMemoryConsolidated(ctx, event)
}
