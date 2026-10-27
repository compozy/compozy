package session

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
)

// Bounds for task observations held until their Agent/Task call is ingested.
const (
	nativeTaskQueueCallLimit   = 8
	nativeTaskQueueParentLimit = 32
)

// Bounds for boot replay of committed task observations.
const (
	nativeTaskReplayEventLimit  = 256
	nativeTaskReplayRowLimit    = 64
	nativeTaskReplayResultSince = 24 * time.Hour
)

// onNativeTask applies one provider task observation to the native row of its
// launching Agent/Task call. It never creates a row: an observation for a call
// the parent's stream has not shown yet waits, bounded, for that call.
func (s *subagentService) onNativeTask(ctx context.Context, parent string, ev NativeSubagentEvent) error {
	unlock := s.lock(parent)
	settled, err := s.applyNativeTask(ctx, parent, ev)
	unlock()
	if err != nil || !settled {
		return err
	}
	return s.OnChildSettled(ctx, parent)
}

func (s *subagentService) applyNativeTask(ctx context.Context, parent string, ev NativeSubagentEvent) (bool, error) {
	call, known := s.observeNativeTaskCall(parent, ev)
	if !known {
		s.queueNativeTask(parent, ev)
		return false, nil
	}
	row, err := s.store.GetSubagent(ctx, ev.WorkspaceID, subagentID(parent, ev.ProviderToolCallID))
	if errors.Is(err, store.ErrSubagentNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	// A frame of another turn is stale: it never touches a later turn's row.
	if row.Origin != store.SubagentOriginProviderNative || row.ParentTurnID != ev.ParentTurnID {
		s.logger.DebugContext(ctx, "subagent.native_task_stale", "subagent_id", row.ID, "turn_id", ev.ParentTurnID)
		return false, nil
	}
	status, terminal := nativeTaskOutcome(ev.Task)
	switch {
	case !terminal:
		if !store.IsSubagentStatusTerminal(row.Status) {
			s.queueProgress(row.WorkspaceID, parent, row.ID, ev.Task.Progress)
		}
		return false, nil
	case store.IsSubagentStatusTerminal(row.Status):
		return false, s.enrichNativeResult(ctx, row, status, ev.Task, ev.At)
	case !call.background:
		// A foreground call's own tool result carries its full outcome.
		return false, nil
	default:
		return s.finalizeNativeTask(ctx, row, status, ev.Task, ev.At)
	}
}

// observeNativeTaskCall reports the known call, recording a backgrounding the
// task observation reports (task_started or a moved-to-background patch).
func (s *subagentService) observeNativeTaskCall(parent string, ev NativeSubagentEvent) (nativeCall, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	call, known := s.nativeKnown[parent][ev.ProviderToolCallID]
	if known && ev.Task.Background != nil && *ev.Task.Background && !call.background {
		call.background = true
		s.nativeKnown[parent][ev.ProviderToolCallID] = call
	}
	return call, known
}

// nativeTaskOutcome maps a provider task status to a terminal row status.
func nativeTaskOutcome(task *NativeSubagentTask) (string, bool) {
	if task == nil || task.Kind != acp.NativeTaskKindState {
		return "", false
	}
	switch task.Status {
	case acp.NativeTaskStatusCompleted:
		return store.SubagentStatusCompleted, true
	case acp.NativeTaskStatusFailed:
		return store.SubagentStatusFailed, true
	case acp.NativeTaskStatusKilled, acp.NativeTaskStatusStopped:
		return store.SubagentStatusCanceled, true
	default:
		return "", false
	}
}

// finalizeNativeTask settles a live native row on authoritative task evidence.
// An absent summary leaves the result NULL for a later notification to fill.
func (s *subagentService) finalizeNativeTask(
	ctx context.Context,
	row store.SessionSubagent,
	status string,
	task *NativeSubagentTask,
	at time.Time,
) (bool, error) {
	var result *string
	truncated := false
	if task.Summary != nil {
		text, cut, err := s.boundNativeResult(ctx, row.WorkspaceID, *task.Summary)
		if err != nil {
			return false, err
		}
		result, truncated = &text, cut
	}
	if at.IsZero() {
		at = s.now().UTC()
	}
	updated, changed, err := s.store.FinalizeSubagent(ctx, store.SubagentFinalize{
		ID: row.ID, Status: status, WorkState: store.SubagentWorkStateResultAvailable,
		Result: result, Error: nativeTaskFailure(status, task), ResultTruncated: truncated, SettledAt: at,
	})
	if err != nil || !changed {
		return false, err
	}
	s.publishTerminal(ctx, updated)
	return true, s.settleParent(ctx, row.ParentSessionID)
}

// enrichNativeResult fills a settled native row's missing result from a later
// summary of the same outcome. It is an ordinary row update: no second settle
// hook, wake, or delivery change.
func (s *subagentService) enrichNativeResult(
	ctx context.Context,
	row store.SessionSubagent,
	status string,
	task *NativeSubagentTask,
	at time.Time,
) error {
	if task.Summary == nil || row.Result != nil {
		return nil
	}
	if row.Status != status {
		s.logger.DebugContext(ctx, "subagent.native_task_conflict", "subagent_id", row.ID,
			"status", row.Status, "observed", status)
		return nil
	}
	text, truncated, err := s.boundNativeResult(ctx, row.WorkspaceID, *task.Summary)
	if err != nil {
		return err
	}
	var failure *string
	if row.Error == nil {
		failure = nativeTaskFailure(status, task)
	}
	if at.IsZero() {
		at = s.now().UTC()
	}
	updated, changed, err := s.store.EnrichNativeSubagentResult(ctx, store.NativeSubagentResultEnrichment{
		ID: row.ID, WorkspaceID: row.WorkspaceID, ParentSessionID: row.ParentSessionID,
		ParentTurnID: row.ParentTurnID, ProviderToolCallID: row.ProviderToolCallID, Status: status,
		Result: text, Error: failure, ResultTruncated: truncated, At: at,
	})
	if err == nil && changed {
		s.publish(ctx, updated)
	}
	return err
}

// nativeTaskFailure is a failed task's structured error, else its summary.
func nativeTaskFailure(status string, task *NativeSubagentTask) *string {
	if status != store.SubagentStatusFailed {
		return nil
	}
	summary := ""
	if task.Summary != nil {
		summary = *task.Summary
	}
	if message := cmp.Or(task.Error, summary); message != "" {
		return &message
	}
	return nil
}

func (s *subagentService) boundNativeResult(ctx context.Context, workspaceID, text string) (string, bool, error) {
	limit, err := s.resultLimit(ctx, workspaceID)
	if err != nil {
		return "", false, err
	}
	runes := []rune(text)
	if limit > 0 && len(runes) > limit {
		return string(runes[:limit]), true, nil
	}
	return text, false, nil
}

// queueNativeTask holds an observation for a call not yet ingested. When full,
// progress yields to terminal evidence.
func (s *subagentService) queueNativeTask(parent string, ev NativeSubagentEvent) {
	_, terminal := nativeTaskOutcome(ev.Task)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.nativeQueued == nil {
		s.nativeQueued = make(map[string]map[string][]NativeSubagentEvent)
	}
	calls := s.nativeQueued[parent]
	if calls == nil {
		calls = make(map[string][]NativeSubagentEvent)
		s.nativeQueued[parent] = calls
	}
	queued, exists := calls[ev.ProviderToolCallID]
	if !exists && len(calls) >= nativeTaskQueueParentLimit {
		return
	}
	if len(queued) >= nativeTaskQueueCallLimit {
		if !terminal {
			return
		}
		drop := slices.IndexFunc(queued, func(held NativeSubagentEvent) bool {
			_, heldTerminal := nativeTaskOutcome(held.Task)
			return !heldTerminal
		})
		queued = slices.Delete(queued, max(drop, 0), max(drop, 0)+1)
	}
	calls[ev.ProviderToolCallID] = append(queued, ev)
}

// drainNativeTasks applies observations that arrived before their call.
func (s *subagentService) drainNativeTasks(ctx context.Context, parent, call string) error {
	s.mu.Lock()
	queued := s.nativeQueued[parent][call]
	delete(s.nativeQueued[parent], call)
	if len(s.nativeQueued[parent]) == 0 {
		delete(s.nativeQueued, parent)
	}
	s.mu.Unlock()
	for _, ev := range queued {
		if err := s.onNativeTask(ctx, parent, ev); err != nil {
			return err
		}
	}
	return nil
}

// dropNativeTasks discards a closed turn's observations that never met their
// call: missing correlation is reported, never guessed.
func (s *subagentService) dropNativeTasks(ctx context.Context, parent, turn string) {
	s.mu.Lock()
	var dropped []string
	for call, queued := range s.nativeQueued[parent] {
		if turn == "" || len(queued) > 0 && queued[0].ParentTurnID == turn {
			dropped = append(dropped, call)
			delete(s.nativeQueued[parent], call)
		}
	}
	if len(s.nativeQueued[parent]) == 0 {
		delete(s.nativeQueued, parent)
	}
	s.mu.Unlock()
	for _, call := range dropped {
		s.logger.WarnContext(ctx, "subagent.native_task_uncorrelated",
			"parent_session_id", parent, "provider_tool_call_id", call)
	}
}

// replayNativeTasks applies a turn's committed task observations for a native
// row before recovery classifies it: the ledger commits before projection, so
// a crash between them leaves the evidence only in the transcript.
func (s *subagentService) replayNativeTasks(ctx context.Context, row store.SessionSubagent) error {
	events, err := s.runtime.NativeTaskEvents(ctx, row.ParentSessionID, row.ParentTurnID, nativeTaskReplayEventLimit)
	if err != nil {
		return err
	}
	for _, stored := range events {
		ev, ok, err := NativeSubagentEventFromStored(row.WorkspaceID, stored)
		if err != nil || !ok || ev.Task == nil || ev.ProviderToolCallID != row.ProviderToolCallID ||
			ev.ParentTurnID != row.ParentTurnID {
			continue
		}
		status, terminal := nativeTaskOutcome(ev.Task)
		if !terminal {
			continue
		}
		current, err := s.store.GetSubagent(ctx, row.WorkspaceID, row.ID)
		if err != nil {
			return err
		}
		if store.IsSubagentStatusTerminal(current.Status) {
			err = s.enrichNativeResult(ctx, current, status, ev.Task, ev.At)
		} else {
			_, err = s.finalizeNativeTask(ctx, current, status, ev.Task, ev.At)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// recoverNativeResults replays committed summaries into recently settled
// native rows whose result is still missing. Bounded by age and count.
func (s *subagentService) recoverNativeResults(ctx context.Context) error {
	rows, err := s.store.ListNativeMissingResult(
		ctx, s.now().UTC().Add(-nativeTaskReplayResultSince), nativeTaskReplayRowLimit,
	)
	if err != nil {
		return err
	}
	for _, row := range rows {
		unlock := s.lock(row.ParentSessionID)
		err := s.replayNativeTasks(ctx, row)
		unlock()
		s.recoveryResult(ctx, row.ID, "native_result", err)
	}
	return nil
}
