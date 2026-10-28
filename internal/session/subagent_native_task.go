package session

import (
	"cmp"
	"context"
	"errors"
	"fmt"
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

// Page sizes and age bound for boot replay of committed task observations.
// Pages bound memory; replay always exhausts the relevant observations.
const (
	nativeTaskReplayEventPage   = 256
	nativeTaskReplayRowPage     = 64
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
		text, cut, err := s.boundNativeResult(ctx, row.WorkspaceID, task)
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
	text, truncated, err := s.boundNativeResult(ctx, row.WorkspaceID, task)
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

// boundNativeResult applies the configured result limit to a task summary.
// The result is truncated when either this limit or an earlier storage bound
// cut it.
func (s *subagentService) boundNativeResult(
	ctx context.Context, workspaceID string, task *NativeSubagentTask,
) (string, bool, error) {
	limit, err := s.resultLimit(ctx, workspaceID)
	if err != nil {
		return "", false, err
	}
	text := *task.Summary
	runes := []rune(text)
	if limit > 0 && len(runes) > limit {
		return string(runes[:limit]), true, nil
	}
	return text, task.SummaryTruncated, nil
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

// nativeReplayCall is what one parent turn's committed events show about a
// native call: whether it ran in the background, its launching call's terminal
// tool result, and its terminal task observations in sequence order.
type nativeReplayCall struct {
	background bool
	result     *NativeSubagentEvent
	tasks      []NativeSubagentEvent
}

func (c *nativeReplayCall) observe(ev NativeSubagentEvent) {
	if ev.Task != nil {
		if ev.Task.Background != nil && *ev.Task.Background {
			c.background = true
		}
		if _, terminal := nativeTaskOutcome(ev.Task); terminal {
			c.tasks = append(c.tasks, ev)
		}
		return
	}
	c.background = c.background || ev.Background
	if ev.Status == store.SubagentStatusCompleted || ev.Status == store.SubagentStatusFailed {
		c.result = &ev
	}
}

// replayNativeTurn applies a parent turn's committed native evidence to that
// turn's rows before recovery classifies them: the ledger commits before
// projection, so a crash between them leaves the evidence only in the
// transcript. It pages through the turn once for all rows and exhausts it
// before applying anything. With toolResults, a foreground call's own tool
// result settles its row first, as on the live path, so its full answer wins
// over the shorter task summary; a task summary settles a foreground row only
// when no tool result was committed, since none can arrive after the turn.
func (s *subagentService) replayNativeTurn(
	ctx context.Context, rows []store.SessionSubagent, toolResults bool,
) error {
	if len(rows) == 0 {
		return nil
	}
	parent, turn, workspace := rows[0].ParentSessionID, rows[0].ParentTurnID, rows[0].WorkspaceID
	calls := make(map[string]*nativeReplayCall, len(rows))
	for _, row := range rows {
		calls[row.ProviderToolCallID] = &nativeReplayCall{}
	}
	types := []string{acp.EventTypeNativeTask}
	if toolResults {
		types = append(types, acp.EventTypeToolCall, acp.EventTypeToolResult)
	}
	var after int64
	for {
		events, err := s.runtime.NativeTurnEvents(ctx, parent, turn, types, after, nativeTaskReplayEventPage)
		if err != nil {
			return err
		}
		for _, stored := range events {
			ev, ok, err := NativeSubagentEventFromStored(workspace, stored)
			if call := calls[ev.ProviderToolCallID]; err == nil && ok && call != nil && ev.ParentTurnID == turn {
				call.observe(ev)
			}
		}
		if len(events) < nativeTaskReplayEventPage {
			break
		}
		next := events[len(events)-1].Sequence
		if next <= after {
			return fmt.Errorf("session: native task replay cursor did not advance past %d", after)
		}
		after = next
	}
	for _, row := range rows {
		if err := s.replayNativeCall(ctx, row, calls[row.ProviderToolCallID]); err != nil {
			return err
		}
	}
	return nil
}

func (s *subagentService) replayNativeCall(
	ctx context.Context, row store.SessionSubagent, call *nativeReplayCall,
) error {
	if call.result != nil {
		current, err := s.store.GetSubagent(ctx, row.WorkspaceID, row.ID)
		if err != nil {
			return err
		}
		if !store.IsSubagentStatusTerminal(current.Status) {
			if _, err := s.finalizeNativeToolResult(ctx, current, *call.result, call.background); err != nil {
				return err
			}
		}
	}
	for _, ev := range call.tasks {
		status, _ := nativeTaskOutcome(ev.Task)
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

// groupNativeTurns splits native rows by parent turn, in first-seen order.
func groupNativeTurns(rows []store.SessionSubagent) [][]store.SessionSubagent {
	index := make(map[[2]string]int)
	var groups [][]store.SessionSubagent
	for _, row := range rows {
		key := [2]string{row.ParentSessionID, row.ParentTurnID}
		at, ok := index[key]
		if !ok {
			at = len(groups)
			index[key] = at
			groups = append(groups, nil)
		}
		groups[at] = append(groups[at], row)
	}
	return groups
}

// recoverNativeResults replays committed summaries into recently settled
// native rows whose result is still missing. Bounded by age; candidates are
// paged with a stable cursor so older recoverable rows are never starved, and
// each page reads a parent turn's transcript once for all of its rows.
func (s *subagentService) recoverNativeResults(ctx context.Context) error {
	since := s.now().UTC().Add(-nativeTaskReplayResultSince)
	var after store.NativeMissingResultCursor
	for {
		rows, err := s.store.ListNativeMissingResult(ctx, since, after, nativeTaskReplayRowPage)
		if err != nil {
			return err
		}
		for _, turn := range groupNativeTurns(rows) {
			unlock := s.lock(turn[0].ParentSessionID)
			err := s.replayNativeTurn(ctx, turn, false)
			unlock()
			s.recoveryResult(ctx, turn[0].ParentTurnID, "native_result", err)
		}
		if len(rows) < nativeTaskReplayRowPage {
			return nil
		}
		last := rows[len(rows)-1]
		if last.SettledAt == nil {
			return fmt.Errorf("session: native missing-result row %s has no settlement time", last.ID)
		}
		after = store.NativeMissingResultCursor{SettledAt: *last.SettledAt, ID: last.ID}
	}
}
