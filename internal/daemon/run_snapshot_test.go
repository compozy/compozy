package daemon

import (
	"encoding/json"
	"testing"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/store/rundb"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func TestRunSnapshotBuilderCoversLifecycleBranches(t *testing.T) {
	t.Parallel()

	builder := newRunSnapshotBuilder()
	baseTime := time.Date(2026, 4, 20, 22, 0, 0, 0, time.UTC)

	textBlock, err := kinds.NewContentBlock(kinds.TextBlock{Text: "snapshot text"})
	if err != nil {
		t.Fatalf("NewContentBlock() error = %v", err)
	}

	for _, item := range []struct {
		kind      eventspkg.EventKind
		timestamp time.Time
		payload   any
	}{
		{
			kind:      eventspkg.EventKindJobQueued,
			timestamp: baseTime,
			payload: kinds.JobQueuedPayload{
				Index:           1,
				SafeName:        "job-001",
				TaskTitle:       "Task One",
				TaskType:        "backend",
				IDE:             "codex",
				Model:           "gpt-5.4",
				ReasoningEffort: "high",
				AccessMode:      "workspace-write",
				CodeFiles:       []string{"a.go"},
			},
		},
		{
			kind:      eventspkg.EventKindJobStarted,
			timestamp: baseTime.Add(time.Second),
			payload: kinds.JobStartedPayload{
				JobAttemptInfo: kinds.JobAttemptInfo{Index: 1, Attempt: 1, MaxAttempts: 3},
				IDE:            "codex",
				Model:          "gpt-5.4",
			},
		},
		{
			kind:      eventspkg.EventKindSessionUpdate,
			timestamp: baseTime.Add(2 * time.Second),
			payload: kinds.SessionUpdatePayload{
				Index: 1,
				Update: kinds.SessionUpdate{
					Kind:   kinds.UpdateKindAgentMessageChunk,
					Status: kinds.StatusRunning,
					Blocks: []kinds.ContentBlock{textBlock},
				},
			},
		},
		{
			kind:      eventspkg.EventKindJobRetryScheduled,
			timestamp: baseTime.Add(3 * time.Second),
			payload: kinds.JobRetryScheduledPayload{
				JobAttemptInfo: kinds.JobAttemptInfo{Index: 1, Attempt: 2, MaxAttempts: 3},
				Reason:         "rate limited",
			},
		},
		{
			kind:      eventspkg.EventKindJobFailed,
			timestamp: baseTime.Add(4 * time.Second),
			payload: kinds.JobFailedPayload{
				JobAttemptInfo: kinds.JobAttemptInfo{Index: 1, Attempt: 2, MaxAttempts: 3},
				CodeFile:       "a.go",
				ExitCode:       17,
				OutLog:         "stdout",
				ErrLog:         "stderr",
				Error:          "boom",
			},
		},
		{
			kind:      eventspkg.EventKindJobQueued,
			timestamp: baseTime.Add(5 * time.Second),
			payload:   kinds.JobQueuedPayload{Index: 2, SafeName: "job-002", TaskTitle: "Task Two"},
		},
		{
			kind:      eventspkg.EventKindJobCancelled,
			timestamp: baseTime.Add(6 * time.Second),
			payload: kinds.JobCancelledPayload{
				JobAttemptInfo: kinds.JobAttemptInfo{Index: 2},
				Reason:         "operator stop",
			},
		},
		{
			kind:      eventspkg.EventKindJobQueued,
			timestamp: baseTime.Add(7 * time.Second),
			payload:   kinds.JobQueuedPayload{Index: 3, SafeName: "job-003", TaskTitle: "Task Three"},
		},
		{
			kind:      eventspkg.EventKindJobCompleted,
			timestamp: baseTime.Add(8 * time.Second),
			payload: kinds.JobCompletedPayload{
				JobAttemptInfo: kinds.JobAttemptInfo{Index: 3, Attempt: 1, MaxAttempts: 1},
				ExitCode:       0,
			},
		},
		{
			kind:      eventspkg.EventKindShutdownRequested,
			timestamp: baseTime.Add(9 * time.Second),
			payload: kinds.ShutdownRequestedPayload{
				ShutdownBase: kinds.ShutdownBase{
					Source:      "operator",
					RequestedAt: baseTime.Add(9 * time.Second),
					DeadlineAt:  baseTime.Add(10 * time.Second),
				},
			},
		},
		{
			kind:      eventspkg.EventKindShutdownDraining,
			timestamp: baseTime.Add(10 * time.Second),
			payload: kinds.ShutdownDrainingPayload{
				ShutdownBase: kinds.ShutdownBase{
					Source:      "operator",
					RequestedAt: baseTime.Add(9 * time.Second),
					DeadlineAt:  baseTime.Add(10 * time.Second),
				},
			},
		},
		{
			kind:      eventspkg.EventKindShutdownTerminated,
			timestamp: baseTime.Add(11 * time.Second),
			payload: kinds.ShutdownTerminatedPayload{
				ShutdownBase: kinds.ShutdownBase{
					Source:      "operator",
					RequestedAt: baseTime.Add(9 * time.Second),
					DeadlineAt:  baseTime.Add(10 * time.Second),
				},
				Forced: true,
			},
		},
		{
			kind:      eventspkg.EventKindRunCompleted,
			timestamp: baseTime.Add(12 * time.Second),
			payload:   kinds.RunCompletedPayload{},
		},
	} {
		rawPayload, err := json.Marshal(item.payload)
		if err != nil {
			t.Fatalf("json.Marshal(%T) error = %v", item.payload, err)
		}
		if err := builder.applyEvent(eventspkg.Event{
			RunID:     "run-snapshot-test",
			Kind:      item.kind,
			Timestamp: item.timestamp,
			Payload:   rawPayload,
		}); err != nil {
			t.Fatalf("applyEvent(%s) error = %v", item.kind, err)
		}
	}

	builder.applyTokenUsageRows([]rundb.TokenUsageRow{
		{TurnID: "run-total", InputTokens: 11, OutputTokens: 7, TotalTokens: 18},
		{TurnID: "session-1", InputTokens: 5, OutputTokens: 2, TotalTokens: 7},
		{TurnID: "session-invalid", InputTokens: 99, OutputTokens: 99, TotalTokens: 198},
	})

	states := builder.jobStates()
	if len(states) != 3 {
		t.Fatalf("len(jobStates) = %d, want 3", len(states))
	}

	if states[0].Status != runStatusFailed || states[0].Summary == nil {
		t.Fatalf("job 1 state = %#v, want failed summary", states[0])
	}
	if states[0].Summary.RetryReason != "rate limited" || states[0].Summary.ErrorText != "boom" {
		t.Fatalf("job 1 summary = %#v, want retry reason and error text", states[0].Summary)
	}
	if states[0].Summary.Session.Revision == 0 {
		t.Fatalf("job 1 session snapshot = %#v, want populated revision", states[0].Summary.Session)
	}
	if states[0].Summary.Usage.TotalTokens != 7 {
		t.Fatalf("job 1 usage = %#v, want session usage total 7", states[0].Summary.Usage)
	}

	if states[1].Status != runStatusCancelled || states[1].Summary == nil ||
		states[1].Summary.ErrorText != "operator stop" {
		t.Fatalf("job 2 state = %#v, want canceled with reason", states[1])
	}
	if states[2].Status != runStatusCompleted || states[2].Summary == nil || states[2].Summary.ExitCode != 0 {
		t.Fatalf("job 3 state = %#v, want completed with exit code 0", states[2])
	}

	if builder.usage.TotalTokens != 18 {
		t.Fatalf("run usage total = %d, want 18", builder.usage.TotalTokens)
	}
	if builder.shutdown == nil || builder.shutdown.Phase != "forcing" || builder.shutdown.Source != "operator" {
		t.Fatalf("shutdown state = %#v, want forcing/operator", builder.shutdown)
	}

	if summary := cloneRunJobSummary(
		apicore.RunJobSummary{CodeFiles: []string{"a.go", "b.go"}},
	); len(
		summary.CodeFiles,
	) != 2 {
		t.Fatalf("cloneRunJobSummary() = %#v, want copied code files", summary)
	}
	if usage := tokenUsageRowToKinds(
		rundb.TokenUsageRow{InputTokens: 2, OutputTokens: 3, TotalTokens: 5},
	); usage.TotalTokens != 5 {
		t.Fatalf("tokenUsageRowToKinds() = %#v, want total 5", usage)
	}
	if index, ok := tokenUsageIndex("session-12"); !ok || index != 12 {
		t.Fatalf("tokenUsageIndex(session-12) = %d, %v; want 12, true", index, ok)
	}
	if index, ok := tokenUsageIndex("bad"); ok || index != 0 {
		t.Fatalf("tokenUsageIndex(bad) = %d, %v; want 0, false", index, ok)
	}
	if state := shutdownStateFromPayload(
		" draining ",
		" operator ",
		baseTime,
		baseTime.Add(time.Second),
	); state.Phase != "draining" ||
		state.Source != "operator" {
		t.Fatalf("shutdownStateFromPayload() = %#v, want trimmed phase/source", state)
	}
}
