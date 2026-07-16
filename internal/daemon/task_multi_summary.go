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

// classifyChildOutcome maps one child's job lifecycle events to a summary bucket.
// A park wins over everything: a parked job records a failure so the run exits
// non-zero, but it is neither completed nor plainly failed. Otherwise a completion
// that was preceded by a stall is a recovery.
func classifyChildOutcome(evs []eventspkg.Event) childOutcome {
	var stalled, completed, parked bool
	for idx := range evs {
		switch evs[idx].Kind {
		case eventspkg.EventKindJobStalled:
			stalled = true
		case eventspkg.EventKindJobParked:
			parked = true
		case eventspkg.EventKindJobCompleted:
			completed = true
		}
	}
	switch {
	case parked:
		return childOutcomeParked
	case completed && stalled:
		return childOutcomeRecovered
	case completed:
		return childOutcomeCompleted
	default:
		return childOutcomeOther
	}
}

// summarizeChildOutcomes folds every child's recorded event stream into the
// batch summary. Total is the queue size, not the number of streams, so children
// that never started still count toward the denominator the user sees.
func summarizeChildOutcomes(total int, childEvents [][]eventspkg.Event) taskMultiRecoverySummary {
	summary := taskMultiRecoverySummary{Total: total}
	for idx := range childEvents {
		summary.add(classifyChildOutcome(childEvents[idx]))
	}
	return summary
}

// collectTaskMultiRecoverySummary reads each launched child's durable job events
// and folds them into the batch summary. A child whose store cannot be read is
// reported as no recovery outcome rather than failing finalization: the summary is
// visibility, and losing it must never change the parent's result.
func (m *RunManager) collectTaskMultiRecoverySummary(
	ctx context.Context,
	total int,
	childRunIDs []string,
) taskMultiRecoverySummary {
	childEvents := make([][]eventspkg.Event, 0, len(childRunIDs))
	for _, runID := range childRunIDs {
		runID = strings.TrimSpace(runID)
		if runID == "" {
			continue
		}
		childEvents = append(childEvents, m.readChildSummaryEvents(ctx, runID))
	}
	return summarizeChildOutcomes(total, childEvents)
}

func (m *RunManager) readChildSummaryEvents(ctx context.Context, runID string) []eventspkg.Event {
	readCtx, cancel := context.WithTimeout(detachContext(ctx), childSummaryReadTimeout)
	defer cancel()
	lease, err := m.acquireRunDB(readCtx, runID)
	if err != nil {
		slog.Default().Warn("daemon: child summary store unavailable", "run_id", runID, "error", err)
		return nil
	}
	defer func() {
		if closeErr := lease.Close(); closeErr != nil {
			slog.Default().Warn("daemon: release child summary store", "run_id", runID, "error", closeErr)
		}
	}()
	evs, err := lease.DB().ListEventsByKind(readCtx, childSummaryKinds)
	if err != nil {
		slog.Default().Warn("daemon: child summary events unreadable", "run_id", runID, "error", err)
		return nil
	}
	return evs
}
