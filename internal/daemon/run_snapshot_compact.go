package daemon

import (
	"context"
	"fmt"
	"log/slog"

	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/store/rundb"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
)

var compactSnapshotEventKinds = []eventspkg.EventKind{
	eventspkg.EventKindJobQueued,
	eventspkg.EventKindJobStarted,
	eventspkg.EventKindJobRetryScheduled,
	eventspkg.EventKindJobCompleted,
	eventspkg.EventKindJobFailed,
	eventspkg.EventKindJobCancelled,
	eventspkg.EventKindShutdownRequested,
	eventspkg.EventKindShutdownDraining,
	eventspkg.EventKindShutdownTerminated,
}

func (m *RunManager) compactSnapshot(
	ctx context.Context,
	runID string,
	runView apicore.Run,
	runDB *rundb.RunDB,
	lastEvent *eventspkg.Event,
) (apicore.RunSnapshot, error) {
	if err := m.persistRuntimeIntegrity(ctx, runID, m.scopeForRun(runID)); err != nil {
		slog.Default().
			Warn("daemon compact snapshot runtime integrity persistence failed", "run_id", runID, "error", err)
	}

	integrity, err := m.loadCompactRunIntegrity(ctx, runID, runView, runDB, lastEvent)
	if err != nil {
		return apicore.RunSnapshot{}, err
	}

	eventRows, err := runDB.ListEventsByKind(ctx, compactSnapshotEventKinds)
	if err != nil {
		return apicore.RunSnapshot{}, err
	}
	transcriptRows, err := runDB.ListTranscriptMessagesTail(
		ctx,
		maxSnapshotTranscriptMessages,
		maxSnapshotTranscriptBytes,
	)
	if err != nil {
		return apicore.RunSnapshot{}, err
	}
	tokenUsageRows, err := runDB.ListTokenUsage(ctx)
	if err != nil {
		return apicore.RunSnapshot{}, err
	}

	builder := newRunSnapshotBuilder()
	for _, item := range eventRows {
		if err := builder.applyEvent(item); err != nil {
			return apicore.RunSnapshot{}, err
		}
	}
	builder.applyTokenUsageRows(tokenUsageRows)

	snapshot := apicore.RunSnapshot{
		Run:               runView,
		Jobs:              builder.jobStates(),
		Transcript:        assembleSnapshotTranscript(transcriptRows),
		Usage:             builder.usage,
		Shutdown:          builder.shutdown,
		Incomplete:        integrity.Incomplete,
		IncompleteReasons: append([]string(nil), integrity.Reasons...),
	}
	if lastEvent != nil {
		cursor := apicore.CursorFromEvent(*lastEvent)
		snapshot.NextCursor = &cursor
	}
	return snapshot, nil
}

func (m *RunManager) loadCompactRunIntegrity(
	ctx context.Context,
	runID string,
	run apicore.Run,
	runDB *rundb.RunDB,
	lastEvent *eventspkg.Event,
) (rundb.RunIntegrityState, error) {
	state, err := runDB.GetIntegrity(ctx)
	if err != nil {
		return rundb.RunIntegrityState{}, fmt.Errorf("daemon: get compact run integrity for %q: %w", runID, err)
	}
	if state.Incomplete {
		m.recordIncompleteRun(runID)
	}

	stats, err := runDB.EventAuditStats(ctx)
	if err != nil {
		return rundb.RunIntegrityState{}, fmt.Errorf("daemon: load compact run stats for %q: %w", runID, err)
	}
	update := auditCompactSnapshotIntegrity(run, stats, lastEvent)
	if !hasRunIntegritySignals(update) {
		return state, nil
	}

	state, err = runDB.UpsertIntegrity(ctx, update)
	if err != nil {
		return rundb.RunIntegrityState{}, fmt.Errorf("daemon: upsert compact run integrity for %q: %w", runID, err)
	}
	if state.Incomplete {
		m.recordIncompleteRun(runID)
	}
	return state, nil
}

func auditCompactSnapshotIntegrity(
	run apicore.Run,
	stats rundb.EventAuditStats,
	lastEvent *eventspkg.Event,
) rundb.RunIntegrityUpdate {
	reasons := make([]string, 0, 3)
	if stats.EventCount > 0 && stats.MaxSequence != stats.EventCount {
		reasons = append(reasons, runIntegrityReasonEventGap)
	}
	if isTerminalRunStatus(run.Status) && !hasTerminalEvent(lastEvent) {
		reasons = append(reasons, runIntegrityReasonTerminalEventMissing)
	}
	if len(reasons) == 0 {
		return rundb.RunIntegrityUpdate{}
	}
	return rundb.RunIntegrityUpdate{
		Incomplete: true,
		Reasons:    reasons,
	}
}
