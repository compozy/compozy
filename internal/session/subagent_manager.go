package session

import (
	"context"
	"encoding/json/v2"
	"sync"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
)

func (m *Manager) logSubagentError(err error) {
	if err != nil {
		m.logger.Error("session: subagent transition failed", "error", err)
	}
}
func (m *Manager) publishSubagentLifecycleEdge(ctx context.Context, before, after *Info) {
	service := m.subagentService()
	if service == nil {
		return
	}
	// Terminal transcript queries wait for recorder finalization; that edge is
	// delivered by finalizeStoppedOwned after releasing its finalization receipt.
	if after.State != StateStopped && after.Lineage != nil && after.Lineage.SpawnRole == store.SubagentSpawnRole {
		m.logSubagentError(service.OnChildSettled(ctx, after.ID))
	}
	if before != nil && before.Liveness != nil && before.Liveness.Activity != nil &&
		before.Liveness.Activity.TurnID != "" &&
		(after.Liveness == nil || after.Liveness.Activity == nil || after.Liveness.Activity.TurnID == "") {
		m.logSubagentError(service.OnParentTurnSettled(ctx, after.ID, before.Liveness.Activity.TurnID))
	}
}
func (m *Manager) publishSubagentActivity(ctx context.Context, child *Session, event acp.AgentEvent) {
	if event.Type != acp.EventTypeToolCall && event.Type != acp.EventTypeThought {
		return
	}
	info := child.Info()
	if info.Lineage == nil || info.Lineage.SpawnRole != store.SubagentSpawnRole {
		return
	}
	if service := m.subagentService(); service != nil {
		progress := event.Title
		if progress == "" {
			progress = event.Text
		}
		service.OnChildActivity(ctx, child.ID, progress)
	}
}
func (m *Manager) publishSubagentWakeCanceled(ctx context.Context, entry *store.SessionInputQueueEntry) {
	if entry.SyntheticPrompt == nil {
		return
	}
	var meta acp.PromptSyntheticMeta
	if err := json.Unmarshal(entry.SyntheticPrompt.Metadata, &meta); err != nil {
		m.logSubagentError(err)
		return
	}
	if meta.Kind == subagentWakeKind {
		if service := m.subagentService(); service != nil {
			m.logSubagentError(service.OnWakeCanceled(ctx, entry.SessionID, entry.MessageID))
		}
	}
}
func (m *Manager) drainSubagentInputEvents(events <-chan acp.AgentEvent) (canceled, failed bool) {
	for event := range events {
		if event.Type == acp.EventTypeError {
			failed = true
		}
		if event.PromptStopReason == acp.PromptStopReasonCancelled {
			canceled = true
		}
	}
	return canceled, failed && !canceled
}

type subagentDispatchLockKey struct{}
type subagentDispatchLock struct {
	service *subagentService
	parent  string
}

func (m *Manager) lockSubagentInputDispatch(
	ctx context.Context,
	entry *store.SessionInputQueueEntry,
) (context.Context, func()) {
	service, ok := m.subagentService().(*subagentService)
	if !ok || entry.SyntheticPrompt == nil {
		return ctx, func() {}
	}
	var meta acp.PromptSyntheticMeta
	if json.Unmarshal(entry.SyntheticPrompt.Metadata, &meta) != nil || meta.Kind != subagentWakeKind {
		return ctx, func() {}
	}
	unlock := sync.OnceFunc(service.lock(entry.SessionID))
	return context.WithValue(
		ctx,
		subagentDispatchLockKey{},
		subagentDispatchLock{service: service, parent: entry.SessionID},
	), unlock
}
func (s *subagentService) lockWakeDispatch(ctx context.Context, parent string) func() {
	held, ok := ctx.Value(subagentDispatchLockKey{}).(subagentDispatchLock)
	if ok && held.service == s && held.parent == parent {
		return func() {}
	}
	return s.lock(parent)
}
