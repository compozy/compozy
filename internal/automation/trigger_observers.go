package automation

import (
	"context"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/session"
)

func (o *triggerSessionObserver) OnSessionCreated(ctx context.Context, sess *session.Session) {
	if o == nil || o.engine == nil {
		return
	}
	if _, err := o.engine.FireSessionCreated(ctx, sess); err != nil {
		o.engine.logger.Warn("automation.trigger.session_created_failed", "error", err)
	}
}

func (o *triggerSessionObserver) OnSessionStopped(ctx context.Context, sess *session.Session) {
	if o == nil || o.engine == nil {
		return
	}
	if _, err := o.engine.FireSessionStopped(ctx, sess); err != nil {
		o.engine.logger.Warn("automation.trigger.session_stopped_failed", "error", err)
	}
}

func (*triggerSessionObserver) OnAgentEvent(context.Context, string, any) {
}

type triggerHookTelemetrySink struct {
	engine *TriggerEngine
}

func (s *triggerHookTelemetrySink) WriteHookRecord(
	ctx context.Context,
	sessionID string,
	record hookspkg.HookRunRecord,
) error {
	if s == nil || s.engine == nil {
		return nil
	}
	_, err := s.engine.FireHookCompletion(ctx, sessionID, record)
	return err
}

type triggerMemoryObserver struct {
	engine *TriggerEngine
}

func (o *triggerMemoryObserver) OnMemoryConsolidated(ctx context.Context, event MemoryConsolidatedEvent) error {
	if o == nil || o.engine == nil {
		return nil
	}
	_, err := o.engine.FireMemoryConsolidated(ctx, event)
	return err
}
