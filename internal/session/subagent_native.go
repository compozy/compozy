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
	unlock := s.lock(parent)
	err := s.ingestNative(ctx, parent, ev)
	unlock()
	if err != nil {
		return err
	}
	return s.OnChildSettled(ctx, parent)
}

func (s *subagentService) ingestNative(ctx context.Context, parent string, ev NativeSubagentEvent) error {
	id := subagentID(parent, ev.ProviderToolCallID)
	row, err := s.store.GetSubagent(ctx, ev.WorkspaceID, id)
	if ev.ToolName != "Agent" && ev.ToolName != "Task" {
		if errors.Is(err, store.ErrSubagentNotFound) {
			s.nativeStitchMiss(ctx, parent, ev.ProviderToolCallID)
			return nil
		}
		return err
	}
	if err != nil && !errors.Is(err, store.ErrSubagentNotFound) {
		return err
	}
	terminal := ev.Status == store.SubagentStatusCompleted || ev.Status == store.SubagentStatusFailed
	if errors.Is(err, store.ErrSubagentNotFound) {
		if terminal {
			s.nativeStitchMiss(ctx, parent, ev.ProviderToolCallID)
			return nil
		}
		row, err = s.reserveNative(ctx, parent, ev)
		if err != nil {
			return err
		}
		s.publish(ctx, row)
	}
	if row.Origin != store.SubagentOriginProviderNative {
		return store.ErrSubagentIdempotencyConflict
	}
	title := strings.TrimSpace(ev.Title)
	if title != "" && title != "Agent" && title != "Task" && title != row.Title {
		runes := []rune(title)
		at := ev.At
		if at.IsZero() {
			at = s.now().UTC()
		}
		var changed bool
		row, changed, err = s.store.UpdateNativeSubagentTitle(ctx, id, string(runes[:min(512, len(runes))]), at)
		if err != nil {
			return err
		}
		if changed {
			s.publish(ctx, row)
		}
	}
	if !terminal || store.IsSubagentStatusTerminal(row.Status) {
		return nil
	}
	limit, err := s.resultLimit(ctx, row.WorkspaceID)
	if err != nil {
		return err
	}
	result := []rune(ev.Result)
	truncated := limit > 0 && len(result) > limit
	if truncated {
		result = result[:limit]
	}
	status := store.SubagentStatusCompleted
	var failure *string
	if ev.Status == store.SubagentStatusFailed {
		status = store.SubagentStatusFailed
		failure = new(ev.Error)
	}
	at := ev.At
	if at.IsZero() {
		at = s.now().UTC()
	}
	row, changed, err := s.store.FinalizeSubagent(
		ctx,
		store.SubagentFinalize{
			ID:              id,
			Status:          status,
			WorkState:       store.SubagentWorkStateResultAvailable,
			Result:          new(string(result)),
			Error:           failure,
			ResultTruncated: truncated,
			SettledAt:       at,
		},
	)
	if err != nil || !changed {
		return err
	}
	s.publishTerminal(ctx, row)
	return s.settleParent(ctx, parent)
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
