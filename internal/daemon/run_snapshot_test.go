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
				CodeFile:        "task_15",
				SafeName:        "task_15-daad11",
				TaskTitle:       "Task One",
				TaskType:        "backend",
				IDE:             "codex",
				Model:           "gpt-5.5",
				ReasoningEffort: "high",
				AccessMode:      "workspace-write",
				CodeFiles:       []string{"task_15"},
			},
		},
		{
			kind:      eventspkg.EventKindJobStarted,
			timestamp: baseTime.Add(time.Second),
			payload: kinds.JobStartedPayload{
				JobAttemptInfo: kinds.JobAttemptInfo{Index: 1, Attempt: 1, MaxAttempts: 3},
				IDE:            "codex",
				Model:          "gpt-5.5",
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
	if states[0].Summary.TaskNumber != 15 {
		t.Fatalf("job 1 task number = %d, want 15", states[0].Summary.TaskNumber)
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

	t.Run("Should clone run job summary code files", func(t *testing.T) {
		t.Parallel()

		summary := cloneRunJobSummary(apicore.RunJobSummary{CodeFiles: []string{"a.go", "b.go"}})
		if len(summary.CodeFiles) != 2 {
			t.Fatalf("cloneRunJobSummary() = %#v, want copied code files", summary)
		}
	})

	t.Run("Should convert token usage rows to usage totals", func(t *testing.T) {
		t.Parallel()

		usage := tokenUsageRowToKinds(rundb.TokenUsageRow{
			InputTokens:  2,
			OutputTokens: 3,
			TotalTokens:  5,
		})
		if usage.TotalTokens != 5 {
			t.Fatalf("tokenUsageRowToKinds() = %#v, want total 5", usage)
		}
	})

	t.Run("Should parse token usage indexes from valid and invalid turn ids", func(t *testing.T) {
		t.Parallel()

		if index, ok := tokenUsageIndex("session-12"); !ok || index != 12 {
			t.Fatalf("tokenUsageIndex(session-12) = %d, %v; want 12, true", index, ok)
		}
		if index, ok := tokenUsageIndex("bad"); ok || index != 0 {
			t.Fatalf("tokenUsageIndex(bad) = %d, %v; want 0, false", index, ok)
		}
	})

	t.Run("Should trim shutdown payload phase and source", func(t *testing.T) {
		t.Parallel()

		state := shutdownStateFromPayload(
			" draining ",
			" operator ",
			baseTime,
			baseTime.Add(time.Second),
		)
		if state.Phase != "draining" || state.Source != "operator" {
			t.Fatalf("shutdownStateFromPayload() = %#v, want trimmed phase/source", state)
		}
	})
}

func TestRunSnapshotBuilderTracksJobPauseAndResumeStates(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name       string
		eventKind  eventspkg.EventKind
		payload    any
		wantStatus string
	}{
		{
			name:      "Should mark pausing job state",
			eventKind: eventspkg.EventKindJobPausing,
			payload: kinds.JobPausingPayload{
				JobAttemptInfo: kinds.JobAttemptInfo{Index: 0},
				SessionID:      "sess-1",
			},
			wantStatus: "pausing",
		},
		{
			name:      "Should mark paused job state",
			eventKind: eventspkg.EventKindJobPaused,
			payload: kinds.JobPausedPayload{
				JobAttemptInfo: kinds.JobAttemptInfo{Index: 0},
				SessionID:      "sess-1",
			},
			wantStatus: "paused",
		},
		{
			name:      "Should mark resumed job state as running",
			eventKind: eventspkg.EventKindJobResumed,
			payload: kinds.JobResumedPayload{
				JobAttemptInfo: kinds.JobAttemptInfo{Index: 0},
				SessionID:      "sess-1",
				MessageID:      "msg-1",
			},
			wantStatus: runStatusRunning,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			builder := newRunSnapshotBuilder()
			for _, item := range []struct {
				kind    eventspkg.EventKind
				payload any
			}{
				{
					kind: eventspkg.EventKindJobQueued,
					payload: kinds.JobQueuedPayload{
						Index:     0,
						SafeName:  "task_01",
						TaskTitle: "Pause me",
					},
				},
				{
					kind: eventspkg.EventKindJobStarted,
					payload: kinds.JobStartedPayload{
						JobAttemptInfo: kinds.JobAttemptInfo{Index: 0, Attempt: 1, MaxAttempts: 1},
					},
				},
				{kind: tc.eventKind, payload: tc.payload},
			} {
				rawPayload, err := json.Marshal(item.payload)
				if err != nil {
					t.Fatalf("json.Marshal(%T) error = %v", item.payload, err)
				}
				if err := builder.applyEvent(eventspkg.Event{
					RunID:     "run-pause-snapshot",
					Kind:      item.kind,
					Timestamp: baseTime,
					Payload:   rawPayload,
				}); err != nil {
					t.Fatalf("applyEvent(%s) error = %v", item.kind, err)
				}
			}

			states := builder.jobStates()
			if len(states) != 1 {
				t.Fatalf("job states = %#v, want one state", states)
			}
			if states[0].Status != tc.wantStatus {
				t.Fatalf("job status = %q, want %q", states[0].Status, tc.wantStatus)
			}
		})
	}
}

func TestRunSnapshotBuilderInfersSparseQueuedTaskNumber(t *testing.T) {
	t.Parallel()

	t.Run("Should infer sparse queued task number from code files", func(t *testing.T) {
		t.Parallel()

		builder := newRunSnapshotBuilder()
		rawPayload, err := json.Marshal(kinds.JobQueuedPayload{
			Index:     0,
			CodeFiles: []string{"notes.md", ".compozy/tasks/demo/task_15.md"},
		})
		if err != nil {
			t.Fatalf("json.Marshal(JobQueuedPayload) error = %v", err)
		}
		if err := builder.applyEvent(eventspkg.Event{
			RunID:     "run-sparse-task-number",
			Kind:      eventspkg.EventKindJobQueued,
			Timestamp: time.Date(2026, 6, 15, 18, 0, 0, 0, time.UTC),
			Payload:   rawPayload,
		}); err != nil {
			t.Fatalf("applyEvent(job.queued) error = %v", err)
		}

		states := builder.jobStates()
		if len(states) != 1 {
			t.Fatalf("job states = %#v, want one state", states)
		}
		if got, want := states[0].Summary.TaskNumber, 15; got != want {
			t.Fatalf("task number = %d, want %d", got, want)
		}
	})
}
