package daemon

// Suite: task-multi recovery summary
// Invariant: completed and recovered counts include only successful child runs without failed jobs.
// Boundary IN: child summary classification plus durable run-status and event reads
// Boundary OUT: parent queue orchestration in task_multi_test.go

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store/globaldb"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func summaryEvent(kind eventspkg.EventKind) eventspkg.Event {
	return eventspkg.Event{SchemaVersion: eventspkg.SchemaVersion, Kind: kind}
}

// recordedChild builds one child's job lifecycle stream in emission order.
func recordedChild(kinds ...eventspkg.EventKind) []eventspkg.Event {
	evs := make([]eventspkg.Event, 0, len(kinds))
	for _, kind := range kinds {
		evs = append(evs, summaryEvent(kind))
	}
	return evs
}

func TestClassifyChildOutcome(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		status string
		evs    []eventspkg.Event
		want   childOutcome
	}{
		{
			name:   "Should classify a plain completion as completed, not recovered",
			status: runStatusCompleted,
			evs:    recordedChild(eventspkg.EventKindJobCompleted),
			want:   childOutcomeCompleted,
		},
		{
			name:   "Should classify stalled then completed as recovered",
			status: runStatusCompleted,
			evs:    recordedChild(eventspkg.EventKindJobStalled, eventspkg.EventKindJobCompleted),
			want:   childOutcomeRecovered,
		},
		{
			name:   "Should classify stalled twice then parked as parked",
			status: runStatusParked,
			evs: recordedChild(
				eventspkg.EventKindJobStalled,
				eventspkg.EventKindJobStalled,
				eventspkg.EventKindJobParked,
			),
			want: childOutcomeParked,
		},
		{
			name:   "Should let a park win over a completion recorded for another attempt",
			status: runStatusParked,
			evs: recordedChild(
				eventspkg.EventKindJobCompleted,
				eventspkg.EventKindJobStalled,
				eventspkg.EventKindJobParked,
			),
			want: childOutcomeParked,
		},
		{
			name:   "Should classify a stall with no terminal job event as neither completed nor parked",
			status: runStatusRunning,
			evs:    recordedChild(eventspkg.EventKindJobStalled),
			want:   childOutcomeOther,
		},
		{
			name:   "Should not let an earlier completed job hide a later failed job",
			status: runStatusFailed,
			evs: recordedChild(
				eventspkg.EventKindJobCompleted,
				eventspkg.EventKindJobFailed,
			),
			want: childOutcomeOther,
		},
		{
			name:   "Should not classify a stalled child as recovered when a later job fails",
			status: runStatusFailed,
			evs: recordedChild(
				eventspkg.EventKindJobStalled,
				eventspkg.EventKindJobCompleted,
				eventspkg.EventKindJobFailed,
			),
			want: childOutcomeOther,
		},
		{
			name:   "Should let failed job evidence override an inconsistent completed run status",
			status: runStatusCompleted,
			evs: recordedChild(
				eventspkg.EventKindJobCompleted,
				eventspkg.EventKindJobFailed,
			),
			want: childOutcomeOther,
		},
		{
			name: "Should classify an empty stream as neither completed nor parked",
			evs:  nil,
			want: childOutcomeOther,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			child := childSummaryEvidence{status: tc.status, events: tc.evs}
			if got := classifyChildOutcome(child); got != tc.want {
				t.Fatalf("classifyChildOutcome() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestSummarizeChildOutcomesOverRecordedBatch feeds a recorded event stream for a
// batch with one recovered and one parked child and asserts the closing counts.
func TestSummarizeChildOutcomesOverRecordedBatch(t *testing.T) {
	t.Parallel()

	t.Run("Should count one recovered and one parked child", func(t *testing.T) {
		t.Parallel()

		summary := summarizeChildOutcomes(3, []childSummaryEvidence{
			{status: runStatusCompleted, events: recordedChild(eventspkg.EventKindJobCompleted)},
			{
				status: runStatusCompleted,
				events: recordedChild(eventspkg.EventKindJobStalled, eventspkg.EventKindJobCompleted),
			},
			{
				status: runStatusParked,
				events: recordedChild(eventspkg.EventKindJobStalled, eventspkg.EventKindJobParked),
			},
		})

		want := taskMultiRecoverySummary{Total: 3, Completed: 2, Recovered: 1, Parked: 1}
		if summary != want {
			t.Fatalf("summarizeChildOutcomes() = %+v, want %+v", summary, want)
		}
	})

	t.Run("Should report zero recovered and parked for a batch with no stalls", func(t *testing.T) {
		t.Parallel()

		summary := summarizeChildOutcomes(2, []childSummaryEvidence{
			{status: runStatusCompleted, events: recordedChild(eventspkg.EventKindJobCompleted)},
			{status: runStatusCompleted, events: recordedChild(eventspkg.EventKindJobCompleted)},
		})

		want := taskMultiRecoverySummary{Total: 2, Completed: 2}
		if summary != want {
			t.Fatalf("summarizeChildOutcomes() = %+v, want %+v", summary, want)
		}
	})

	t.Run("Should keep total as the queue size when a child never started", func(t *testing.T) {
		t.Parallel()

		summary := summarizeChildOutcomes(2, []childSummaryEvidence{
			{status: runStatusCompleted, events: recordedChild(eventspkg.EventKindJobCompleted)},
		})

		want := taskMultiRecoverySummary{Total: 2, Completed: 1}
		if summary != want {
			t.Fatalf("summarizeChildOutcomes() = %+v, want %+v", summary, want)
		}
	})

	t.Run("Should not count a failed child as completed or parked", func(t *testing.T) {
		t.Parallel()

		summary := summarizeChildOutcomes(1, []childSummaryEvidence{{status: runStatusFailed}})

		want := taskMultiRecoverySummary{Total: 1}
		if summary != want {
			t.Fatalf("summarizeChildOutcomes() = %+v, want %+v", summary, want)
		}
	})
}

func TestCollectTaskMultiRecoverySummaryUsesSettledChildStatus(t *testing.T) {
	t.Parallel()

	env := newRunManagerTestEnv(t, runManagerTestDeps{})
	workspace, err := env.globalDB.ResolveOrRegister(context.Background(), env.workspaceRoot)
	if err != nil {
		t.Fatalf("ResolveOrRegister(%q) error = %v", env.workspaceRoot, err)
	}
	const runID = "child-completed-then-failed"
	endedAt := time.Date(2026, time.July, 22, 20, 0, 0, 0, time.UTC)
	if _, err := env.globalDB.PutRun(context.Background(), globaldb.Run{
		RunID:            runID,
		WorkspaceID:      workspace.ID,
		Mode:             runModeTask,
		Status:           runStatusFailed,
		PresentationMode: defaultPresentationMode,
		StartedAt:        endedAt.Add(-time.Second),
		EndedAt:          &endedAt,
	}); err != nil {
		t.Fatalf("PutRun(%q) error = %v", runID, err)
	}
	artifacts := env.manager.runArtifacts(runID)
	if err := os.MkdirAll(artifacts.RunDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", artifacts.RunDir, err)
	}
	lease, err := env.manager.acquireRunDB(context.Background(), runID)
	if err != nil {
		t.Fatalf("acquireRunDB(%q) error = %v", runID, err)
	}
	defer func() {
		if closeErr := lease.Close(); closeErr != nil {
			t.Errorf("release child run DB error = %v", closeErr)
		}
	}()
	if _, err := lease.DB().AppendSyntheticEvent(
		context.Background(),
		eventspkg.EventKindJobCompleted,
		kinds.JobCompletedPayload{JobAttemptInfo: kinds.JobAttemptInfo{Index: 0, Attempt: 1}},
	); err != nil {
		t.Fatalf("AppendSyntheticEvent(job.completed) error = %v", err)
	}
	if _, err := lease.DB().AppendSyntheticEvent(
		context.Background(),
		eventspkg.EventKindJobFailed,
		kinds.JobFailedPayload{JobAttemptInfo: kinds.JobAttemptInfo{Index: 1, Attempt: 1}, Error: "later job failed"},
	); err != nil {
		t.Fatalf("AppendSyntheticEvent(job.failed) error = %v", err)
	}

	summary := env.manager.collectTaskMultiRecoverySummary(context.Background(), 1, []string{runID})
	want := taskMultiRecoverySummary{Total: 1}
	if summary != want {
		t.Fatalf("collectTaskMultiRecoverySummary() = %+v, want %+v", summary, want)
	}
}
