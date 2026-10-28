package session

import (
	"cmp"
	"context"
	"errors"
	"maps"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
)

// OnNativeToolEvent records provider-owned work without creating sessions or wakes.
func (s *subagentService) OnNativeToolEvent(ctx context.Context, parent string, ev NativeSubagentEvent) error {
	if ev.ProviderToolCallID == "" {
		return nil
	}
	if ev.Task != nil {
		return s.onNativeTask(ctx, parent, ev)
	}
	if ev.ToolName != nativeSubagentAgentTool && ev.ToolName != nativeSubagentTaskTool {
		s.mu.Lock()
		_, known := s.nativeKnown[parent][ev.ProviderToolCallID]
		s.mu.Unlock()
		if !known {
			s.nativeStitchMiss(ctx, parent, ev.ProviderToolCallID)
			return nil
		}
		// A nested event is the native subagent's heartbeat: it records progress
		// and bumps updated_at without changing the row's status.
		s.queueProgress(ev.WorkspaceID, parent, subagentID(parent, ev.ProviderToolCallID), ev.Progress)
		return nil
	}
	unlock := s.lock(parent)
	changed, err := s.ingestNative(ctx, parent, ev)
	unlock()
	if err == nil {
		err = s.drainNativeTasks(ctx, parent, ev.ProviderToolCallID)
	}
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
	row, err = s.updateNativeTitle(ctx, row, ev)
	if err != nil {
		return false, err
	}
	call := s.rememberNativeCall(parent, ev)
	if store.IsSubagentStatusTerminal(row.Status) {
		return created, nil
	}
	changed, err := s.finalizeNativeToolResult(ctx, row, ev, call.background)
	if err != nil || !changed {
		return created, err
	}
	return true, s.settleParent(ctx, parent)
}

// finalizeNativeToolResult settles a live native row from its launching
// call's terminal tool result. A background launch completes with the
// provider's receipt, not the subagent's outcome; that row runs until its task
// reports a terminal state (subagent_native_task.go) or, as a fallback, the
// launching turn ends.
func (s *subagentService) finalizeNativeToolResult(
	ctx context.Context, row store.SessionSubagent, ev NativeSubagentEvent, background bool,
) (bool, error) {
	terminal := ev.Status == store.SubagentStatusCompleted || ev.Status == store.SubagentStatusFailed
	if !terminal || background && ev.Status == store.SubagentStatusCompleted {
		return false, nil
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
	updated, changed, err := s.store.FinalizeSubagent(ctx, store.SubagentFinalize{
		ID: row.ID, Status: ev.Status, WorkState: store.SubagentWorkStateResultAvailable,
		Result: new(string(result)), Error: failure, ResultTruncated: truncated, SettledAt: at,
	})
	if err != nil || !changed {
		return false, err
	}
	s.publishTerminal(ctx, updated)
	return true, nil
}

// nativeCall is what the parent's stream has shown about one native Agent/Task call.
type nativeCall struct {
	background bool
}

func (s *subagentService) rememberNativeCall(parent string, ev NativeSubagentEvent) nativeCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.nativeKnown == nil {
		s.nativeKnown = make(map[string]map[string]nativeCall)
	}
	if s.nativeKnown[parent] == nil {
		s.nativeKnown[parent] = make(map[string]nativeCall)
	}
	call := s.nativeKnown[parent][ev.ProviderToolCallID]
	call.background = call.background || ev.Background
	s.nativeKnown[parent][ev.ProviderToolCallID] = call
	return call
}

// OnNativeTurnDone is the fallback for background native subagents of a turn
// the provider ended. Task lifecycle observations settle each row as its task
// finishes; claude-agent-acp holds a prompt's response until every background
// subagent that turn spawned has settled, so a row still live here lacked that
// evidence (an older adapter, or a dropped frame) and completes without a result.
func (s *subagentService) OnNativeTurnDone(ctx context.Context, parent, turn string, at time.Time) error {
	s.dropNativeTasks(ctx, parent, turn)
	unlock := s.lock(parent)
	changed, err := s.settleBackgroundNative(ctx, parent, turn, at)
	unlock()
	if err != nil || !changed {
		return err
	}
	return s.OnChildSettled(ctx, parent)
}

func (s *subagentService) settleBackgroundNative(
	ctx context.Context, parent, turn string, at time.Time,
) (bool, error) {
	s.mu.Lock()
	calls := maps.Clone(s.nativeKnown[parent])
	s.mu.Unlock()
	if len(calls) == 0 {
		return false, nil
	}
	rows, err := s.parentRows(ctx, parent)
	if err != nil {
		return false, err
	}
	if at.IsZero() {
		at = s.now().UTC()
	}
	changed := false
	for _, row := range rows {
		if row.Origin != store.SubagentOriginProviderNative || store.IsSubagentStatusTerminal(row.Status) ||
			row.ParentTurnID != turn || !calls[row.ProviderToolCallID].background {
			continue
		}
		updated, finalized, err := s.store.FinalizeSubagent(ctx, store.SubagentFinalize{
			ID: row.ID, Status: store.SubagentStatusCompleted, WorkState: store.SubagentWorkStateResultAvailable,
			SettledAt: at,
		})
		if err != nil {
			return changed, err
		}
		if finalized {
			s.publishTerminal(ctx, updated)
			changed = true
		}
	}
	if !changed {
		return false, nil
	}
	return true, s.settleParent(ctx, parent)
}

func (s *subagentService) updateNativeTitle(
	ctx context.Context, row store.SessionSubagent, ev NativeSubagentEvent,
) (store.SessionSubagent, error) {
	title := strings.TrimSpace(ev.Title)
	if title != "" && title != nativeSubagentAgentTool && title != nativeSubagentTaskTool && title != row.Title {
		runes := []rune(title)
		at := ev.At
		if at.IsZero() {
			at = s.now().UTC()
		}
		var changed bool
		var err error
		row, changed, err = s.store.UpdateNativeSubagentTitle(ctx, row.ID, string(runes[:min(512, len(runes))]), at)
		if err != nil {
			return row, err
		}
		if changed {
			s.publish(ctx, row)
		}
	}
	return row, nil
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
	delete(s.nativeQueued, parent)
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

// publishNativeSubagentEvent ingests a committed event on the persistence path,
// the goroutine that later settles the turn: interruptNative treats any row
// still live then as interrupted and cannot accept a late terminal update.
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
		nested := NativeSubagentEvent{WorkspaceID: info.WorkspaceID, ProviderToolCallID: event.ParentToolCallID()}
		if event.Type == acp.EventTypeToolCall || event.Type == acp.EventTypeThought {
			nested.Progress = cmp.Or(event.Title, event.Text)
		}
		err = service.OnNativeToolEvent(ctx, child.ID, nested)
	}
	if err != nil {
		m.sessionLogger(child).
			ErrorContext(ctx, "session: native subagent ingest failed", "sequence", persisted.Sequence, "error", err)
	}
}

// settleNativeTurnDone runs on the persistence path when the provider answers a
// prompt, before turn settlement interrupts every live native row. A canceled
// turn may leave background work running, so settlement interrupts those rows.
func (m *Manager) settleNativeTurnDone(ctx context.Context, turnState *promptTurnDispatchState, event acp.AgentEvent) {
	if event.Type != acp.EventTypeDone || event.PromptStopReason == acp.PromptStopReasonCancelled ||
		turnState == nil || turnState.session == nil {
		return
	}
	service := m.subagentService()
	if service == nil {
		return
	}
	if err := service.OnNativeTurnDone(ctx, turnState.session.ID, turnState.turnID, event.Timestamp); err != nil {
		m.sessionLogger(turnState.session).
			ErrorContext(ctx, "session: native background subagent settlement failed", "turn_id", turnState.turnID, "error", err)
	}
}
