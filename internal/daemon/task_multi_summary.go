package daemon

import (
	"context"
	"log/slog"
	"strings"
	"time"

	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
)

// childSummaryReadTimeout bounds the durable read of one child's job lifecycle
// events. The summary is best-effort reporting on an already-settled batch, so a
// slow or wedged child store must never hold the parent's finalization open.
const childSummaryReadTimeout = 5 * time.Second

// childSummaryKinds are the only event kinds the recovery summary reads. Each one
// is terminal-ish for a job, so the set stays tiny no matter how long the child ran.
var childSummaryKinds = []eventspkg.EventKind{
	eventspkg.EventKindJobStalled,
	eventspkg.EventKindJobParked,
	eventspkg.EventKindJobCompleted,
	eventspkg.EventKindJobFailed,
}

type childSummaryEvidence struct {
	status string
	events []eventspkg.Event
}

// childOutcome is one child's contribution to the end-of-run recovery summary.
type childOutcome int

const (
	// childOutcomeOther covers failed, canceled, and never-started children. They
	// are reported by the existing failure paths, not by the recovery summary.
	childOutcomeOther childOutcome = iota
	// childOutcomeCompleted is a plain completion: the child never stalled.
	childOutcomeCompleted
	// childOutcomeRecovered is a completion that followed a stall. The clean-state
	// retry worked and the user never had to intervene.
	childOutcomeRecovered
	// childOutcomeParked is a child that stalled again after its retry and is
	// preserved for triage.
	childOutcomeParked
)

// taskMultiRecoverySummary is the closing count for a parallel parent run.
// Completed counts every child that finished successfully; Recovered is the
// subset of those that needed a stall recovery to get there, so a recovered child
// is reported in both. Parked is disjoint from both: a parked child never
// completed.
type taskMultiRecoverySummary struct {
	Total     int
	Completed int
	Recovered int
	Parked    int
}

func (s *taskMultiRecoverySummary) add(outcome childOutcome) {
	switch outcome {
	case childOutcomeCompleted:
		s.Completed++
	case childOutcomeRecovered:
		s.Completed++
		s.Recovered++
	case childOutcomeParked:
		s.Parked++
	}
}

// classifyChildOutcome maps one settled child and its complete terminal job
// lifecycle to a summary bucket. A park wins because it requires operator triage.
// Completion and recovery require both a successful child run and no failed job.
func classifyChildOutcome(child childSummaryEvidence) childOutcome {
	var stalled, completed, failed, parked bool
	for idx := range child.events {
		switch child.events[idx].Kind {
		case eventspkg.EventKindJobStalled:
			stalled = true
		case eventspkg.EventKindJobParked:
			parked = true
		case eventspkg.EventKindJobCompleted:
			completed = true
		case eventspkg.EventKindJobFailed:
			failed = true
		}
	}
	switch {
	case parked:
		return childOutcomeParked
	case strings.TrimSpace(child.status) != runStatusCompleted || failed:
		return childOutcomeOther
	case completed && stalled:
		return childOutcomeRecovered
	case completed:
		return childOutcomeCompleted
	default:
		return childOutcomeOther
	}
}

// summarizeChildOutcomes folds every child's settled status and recorded event
// stream into the batch summary. Total is the queue size, not the number of
// streams, so children that never started still count toward the denominator.
func summarizeChildOutcomes(total int, children []childSummaryEvidence) taskMultiRecoverySummary {
	summary := taskMultiRecoverySummary{Total: total}
	for idx := range children {
		summary.add(classifyChildOutcome(children[idx]))
	}
	return summary
}

// collectTaskMultiRecoverySummary reads each launched child's durable result and
// job events, then folds them into the batch summary. Unreadable evidence produces
// no recovery outcome rather than changing the parent's result.
func (m *RunManager) collectTaskMultiRecoverySummary(
	ctx context.Context,
	total int,
	childRunIDs []string,
) taskMultiRecoverySummary {
	children := make([]childSummaryEvidence, 0, len(childRunIDs))
	for _, runID := range childRunIDs {
		runID = strings.TrimSpace(runID)
		if runID == "" {
			continue
		}
		children = append(children, m.readChildSummaryEvidence(ctx, runID))
	}
	return summarizeChildOutcomes(total, children)
}

func (m *RunManager) readChildSummaryEvidence(ctx context.Context, runID string) childSummaryEvidence {
	readCtx, cancel := context.WithTimeout(detachContext(ctx), childSummaryReadTimeout)
	defer cancel()
	row, err := m.globalDB.GetRun(readCtx, runID)
	if err != nil {
		slog.Default().Warn("daemon: child summary result unavailable", "run_id", runID, "error", err)
		return childSummaryEvidence{}
	}
	lease, err := m.acquireRunDB(readCtx, runID)
	if err != nil {
		slog.Default().Warn("daemon: child summary store unavailable", "run_id", runID, "error", err)
		return childSummaryEvidence{}
	}
	defer func() {
		if closeErr := lease.Close(); closeErr != nil {
			slog.Default().Warn("daemon: release child summary store", "run_id", runID, "error", closeErr)
		}
	}()
	evs, err := lease.DB().ListEventsByKind(readCtx, childSummaryKinds)
	if err != nil {
		slog.Default().Warn("daemon: child summary events unreadable", "run_id", runID, "error", err)
		return childSummaryEvidence{}
	}
	return childSummaryEvidence{status: row.Status, events: evs}
}
