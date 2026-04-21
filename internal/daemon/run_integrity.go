package daemon

import (
	"context"
	"errors"
	"strings"

	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/store/rundb"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
)

const (
	runIntegrityReasonEventGap             = "event_gap"
	runIntegrityReasonJournalSubmitDrops   = "journal_submit_drops"
	runIntegrityReasonTerminalEventMissing = "terminal_event_missing"
	runIntegrityReasonTranscriptGap        = "transcript_gap"

	maxSnapshotTranscriptMessages = 200
	maxSnapshotTranscriptBytes    = 64 << 10
)

func (m *RunManager) persistRuntimeIntegrity(ctx context.Context, runID string, scope model.RunScope) error {
	update := runtimeIntegrityUpdate(scope)
	if !hasRunIntegritySignals(update) {
		return nil
	}
	m.recordJournalDropTotals(runID, update.JournalTerminalDrops, update.JournalNonTerminalDrops)

	lease, err := m.acquireRunDB(ctx, runID)
	if err != nil {
		return err
	}
	defer func() {
		_ = lease.Close()
	}()

	state, err := lease.DB().UpsertIntegrity(ctx, update)
	if err != nil {
		return err
	}
	if state.Incomplete {
		m.recordIncompleteRun(runID)
	}
	return nil
}

func (m *RunManager) scopeForRun(runID string) model.RunScope {
	active := m.getActive(runID)
	if active == nil {
		return nil
	}
	return active.scope
}

func (m *RunManager) loadRunIntegrity(
	ctx context.Context,
	runID string,
	run apicore.Run,
	runDB *rundb.RunDB,
	events []eventspkg.Event,
	transcriptRows []rundb.TranscriptMessageRow,
	lastEvent *eventspkg.Event,
) (rundb.RunIntegrityState, error) {
	if runDB == nil {
		return rundb.RunIntegrityState{}, errors.New("daemon: run db is required")
	}

	state, err := runDB.GetIntegrity(ctx)
	if err != nil {
		return rundb.RunIntegrityState{}, err
	}
	if state.Incomplete {
		m.recordIncompleteRun(runID)
	}

	update := auditSnapshotIntegrity(run, events, transcriptRows, lastEvent)
	if !hasRunIntegritySignals(update) {
		return state, nil
	}

	state, err = runDB.UpsertIntegrity(ctx, update)
	if err != nil {
		return rundb.RunIntegrityState{}, err
	}
	if state.Incomplete {
		m.recordIncompleteRun(runID)
	}
	return state, nil
}

func runtimeIntegrityUpdate(scope model.RunScope) rundb.RunIntegrityUpdate {
	if scope == nil || scope.RunJournal() == nil {
		return rundb.RunIntegrityUpdate{}
	}

	terminalDrops, nonTerminalDrops := scope.RunJournal().DroppedEventCounts()
	if terminalDrops == 0 && nonTerminalDrops == 0 {
		return rundb.RunIntegrityUpdate{}
	}
	return rundb.RunIntegrityUpdate{
		Incomplete:              true,
		Reasons:                 []string{runIntegrityReasonJournalSubmitDrops},
		JournalTerminalDrops:    terminalDrops,
		JournalNonTerminalDrops: nonTerminalDrops,
	}
}

func hasRunIntegritySignals(update rundb.RunIntegrityUpdate) bool {
	return update.Incomplete || len(update.Reasons) > 0 ||
		update.JournalTerminalDrops > 0 || update.JournalNonTerminalDrops > 0
}

func auditSnapshotIntegrity(
	run apicore.Run,
	events []eventspkg.Event,
	transcriptRows []rundb.TranscriptMessageRow,
	lastEvent *eventspkg.Event,
) rundb.RunIntegrityUpdate {
	reasons := make([]string, 0, 3)
	if hasEventGap(events) {
		reasons = append(reasons, runIntegrityReasonEventGap)
	}
	if isTerminalRunStatus(run.Status) && !hasTerminalEvent(lastEvent) {
		reasons = append(reasons, runIntegrityReasonTerminalEventMissing)
	}
	if hasTranscriptGap(events, transcriptRows) {
		reasons = append(reasons, runIntegrityReasonTranscriptGap)
	}
	if len(reasons) == 0 {
		return rundb.RunIntegrityUpdate{}
	}
	return rundb.RunIntegrityUpdate{
		Incomplete: true,
		Reasons:    reasons,
	}
}

func hasEventGap(events []eventspkg.Event) bool {
	expected := uint64(1)
	for _, item := range events {
		if item.Seq != expected {
			return true
		}
		expected++
	}
	return false
}

func hasTranscriptGap(events []eventspkg.Event, transcriptRows []rundb.TranscriptMessageRow) bool {
	if len(events) == 0 {
		return false
	}

	projected := make(map[uint64]struct{}, len(transcriptRows))
	for _, row := range transcriptRows {
		projected[row.Sequence] = struct{}{}
	}

	for _, item := range events {
		row, ok, err := rundb.ProjectTranscriptMessage(item)
		if err != nil {
			return true
		}
		if !ok {
			continue
		}
		if _, exists := projected[row.Sequence]; !exists {
			return true
		}
	}
	return false
}

func hasTerminalEvent(item *eventspkg.Event) bool {
	if item == nil {
		return false
	}
	switch item.Kind {
	case eventspkg.EventKindRunCompleted,
		eventspkg.EventKindRunFailed,
		eventspkg.EventKindRunCancelled,
		eventspkg.EventKindRunCrashed:
		return true
	default:
		return false
	}
}

func assembleSnapshotTranscript(rows []rundb.TranscriptMessageRow) []apicore.RunTranscriptMessage {
	if len(rows) == 0 {
		return nil
	}

	selected := make([]rundb.TranscriptMessageRow, 0, min(len(rows), maxSnapshotTranscriptMessages))
	totalBytes := 0
	for idx := len(rows) - 1; idx >= 0; idx-- {
		row := rows[idx]
		payloadBytes := len(row.Content) + len(row.MetadataJSON)
		if len(selected) > 0 && (len(selected) >= maxSnapshotTranscriptMessages ||
			totalBytes+payloadBytes > maxSnapshotTranscriptBytes) {
			break
		}
		totalBytes += payloadBytes
		selected = append(selected, row)
	}

	result := make([]apicore.RunTranscriptMessage, 0, len(selected))
	for idx := len(selected) - 1; idx >= 0; idx-- {
		row := selected[idx]
		result = append(result, apicore.RunTranscriptMessage{
			Sequence:    row.Sequence,
			Stream:      strings.TrimSpace(row.Stream),
			Role:        strings.TrimSpace(row.Role),
			Content:     row.Content,
			MetadataRaw: rawMessageOrNil(row.MetadataJSON),
			Timestamp:   row.Timestamp,
		})
	}
	return result
}
