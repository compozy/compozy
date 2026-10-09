package session

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
)

// OnNativeToolEvent records provider-owned work without creating sessions or wakes.
func (s *subagentService) OnNativeToolEvent(ctx context.Context, parent string, ev NativeSubagentEvent) error {
	if ev.ProviderToolCallID == "" {
		return nil
	}
	if ev.ToolName != "Agent" && ev.ToolName != "Task" {
		s.mu.Lock()
		known := s.nativeKnown[parent][ev.ProviderToolCallID]
		s.mu.Unlock()
		if !known {
			s.nativeStitchMiss(ctx, parent, ev.ProviderToolCallID)
		}
		return nil
	}
	unlock := s.lock(parent)
	changed, err := s.ingestNative(ctx, parent, ev)
	unlock()
	if err != nil || !changed {
		return err
	}
	return s.OnChildSettled(ctx, parent)
}

func (s *subagentService) ingestNative(ctx context.Context, parent string, ev NativeSubagentEvent) (bool, error) {
	id := subagentID(parent, ev.ProviderToolCallID)
	row, err := s.store.GetSubagent(ctx, ev.WorkspaceID, id)
	if err != nil && !errors.Is(err, store.ErrSubagentNotFound) {
		return false, err
	}
	created := errors.Is(err, store.ErrSubagentNotFound)
	if created {
		row, err = s.reserveNative(ctx, parent, ev)
		if err != nil {
			return false, err
		}
		s.publish(ctx, row)
		s.runtime.PublishParent(ctx, row.ParentSessionID)
	}
	if row.Origin != store.SubagentOriginProviderNative {
		return false, store.ErrSubagentIdempotencyConflict
	}
	s.mu.Lock()
	if s.nativeKnown == nil {
		s.nativeKnown = make(map[string]map[string]bool)
	}
	if s.nativeKnown[parent] == nil {
		s.nativeKnown[parent] = make(map[string]bool)
	}
	s.nativeKnown[parent][ev.ProviderToolCallID] = true
	s.mu.Unlock()
	terminal := ev.Status == store.SubagentStatusCompleted || ev.Status == store.SubagentStatusFailed
	if !terminal || store.IsSubagentStatusTerminal(row.Status) {
		return created, nil
	}
	limit, err := s.resultLimit(ctx, row.WorkspaceID)
	if err != nil {
		return false, err
	}
	result := []rune(ev.Result)
	truncated := limit > 0 && len(result) > limit
	if truncated {
		result = result[:limit]
	}
	var failure *string
	if ev.Status == store.SubagentStatusFailed {
		failure = new(ev.Error)
	}
	at := ev.At
	if at.IsZero() {
		at = s.now().UTC()
	}
	row, changed, err := s.store.FinalizeSubagent(ctx, store.SubagentFinalize{
		ID: id, Status: ev.Status, WorkState: store.SubagentWorkStateResultAvailable,
		Result: new(string(result)), Error: failure, ResultTruncated: truncated, SettledAt: at,
	})
	if err != nil || !changed {
		return created, err
	}
	s.publishTerminal(ctx, row)
	return true, s.settleParent(ctx, parent)
}

func (s *subagentService) interruptNative(ctx context.Context, parent, turn string) error {
	rows, err := s.parentRows(ctx, parent)
	if err != nil {
		return err
	}
	changed := false
	for _, row := range rows {
		if row.Origin != store.SubagentOriginProviderNative || store.IsSubagentStatusTerminal(row.Status) ||
			(turn != "" && row.ParentTurnID != turn) {
			continue
		}
		if err := s.finalizeInterruptedNative(ctx, row); err != nil {
			return err
		}
		changed = true
	}
	s.mu.Lock()
	delete(s.nativeKnown, parent)
	s.mu.Unlock()
	if changed {
		return s.settleParent(ctx, parent)
	}
	return nil
}

func (s *subagentService) finalizeInterruptedNative(ctx context.Context, row store.SessionSubagent) error {
	updated, changed, err := s.store.FinalizeSubagent(ctx, store.SubagentFinalize{
		ID: row.ID, Status: store.SubagentStatusInterrupted, WorkState: store.SubagentWorkStateResultAvailable,
		SettledAt: s.now().UTC(),
	})
	if err == nil && changed {
		s.publishTerminal(ctx, updated)
	}
	return err
}

func (s *subagentService) reserveNative(
	ctx context.Context,
	parent string,
	ev NativeSubagentEvent,
) (store.SessionSubagent, error) {
	snap, err := s.runtime.Snapshot(ctx, parent)
	if err != nil {
		return store.SessionSubagent{}, err
	}
	if snap.Info.WorkspaceID != ev.WorkspaceID {
		return store.SessionSubagent{}, ErrSubagentNotFound
	}
	depth, err := s.depth(ctx, parent)
	if err != nil {
		return store.SessionSubagent{}, err
	}
	title := strings.TrimSpace(ev.Title)
	if title == "" {
		title = ev.ToolName
	}
	runes := []rune(title)
	title = string(runes[:min(512, len(runes))])
	at := ev.At
	if at.IsZero() {
		at = s.now().UTC()
	}
	row, _, err := s.store.ReserveSubagent(ctx, store.SessionSubagent{
		ID: subagentID(
			parent,
			ev.ProviderToolCallID,
		),
		WorkspaceID:        ev.WorkspaceID,
		ParentSessionID:    parent,
		ParentTurnID:       ev.ParentTurnID,
		Origin:             store.SubagentOriginProviderNative,
		ProviderToolCallID: ev.ProviderToolCallID,
		IdempotencyKey:     ev.ProviderToolCallID,
		RequestFingerprint: "provider_native:" + ev.ProviderToolCallID,
		Title:              title,
		Role:               "general",
		Depth:              depth + 1,
		Status:             store.SubagentStatusRunning,
		WorkState:          store.SubagentWorkStateWorking,
		RuntimeAgent:       snap.Info.AgentName,
		RuntimeProvider:    snap.Info.Provider,
		RuntimeModel:       ev.Model,
		WakePolicy:         store.SubagentWakePolicySettledOnly,
		Delivery:           store.SubagentDeliveryNone,
		StartedAt:          new(at),
		CreatedAt:          at,
		UpdatedAt:          at,
	})
	return row, err
}

func (s *subagentService) nativeStitchMiss(ctx context.Context, parent, id string) {
	s.mu.Lock()
	if s.nativeMisses == nil {
		s.nativeMisses = make(map[string]bool)
	}
	key := parent + ":" + id
	seen := s.nativeMisses[key]
	if len(s.nativeMisses) >= 1024 {
		clear(s.nativeMisses)
	}
	s.nativeMisses[key] = true
	s.mu.Unlock()
	if !seen {
		s.logger.WarnContext(
			ctx,
			"subagent.native_stitch_miss",
			"parent_session_id",
			parent,
			"provider_tool_call_id",
			id,
		)
	}
}

func (m *Manager) publishNativeSubagentEvent(
	ctx context.Context,
	child *Session,
	persisted store.SessionEvent,
	event acp.AgentEvent,
) {
	service := m.subagentService()
	if service == nil {
		return
	}
	info := child.Info()
	native, ok, err := NativeSubagentEventFromStored(info.WorkspaceID, persisted)
	if err == nil && ok {
		err = service.OnNativeToolEvent(ctx, child.ID, native)
	}
	if err == nil && !ok && event.ParentToolCallID() != "" {
		err = service.OnNativeToolEvent(
			ctx,
			child.ID,
			NativeSubagentEvent{WorkspaceID: info.WorkspaceID, ProviderToolCallID: event.ParentToolCallID()},
		)
	}
	if err != nil {
		m.sessionLogger(child).
			ErrorContext(ctx, "session: native subagent ingest failed", "sequence", persisted.Sequence, "error", err)
	}
}
