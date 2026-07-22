package rundb

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func TestRunDBRoundTripsEventsAndProjectionRowsAfterReopen(t *testing.T) {
	t.Parallel()

	runID := "run-store-roundtrip"
	db := openTestRunDB(t, runID)
	path := db.Path()

	eventsToStore := []events.Event{
		mustEvent(
			t,
			runID,
			1,
			time.Date(2026, 4, 17, 20, 0, 1, 0, time.UTC),
			events.EventKindJobQueued,
			kinds.JobQueuedPayload{
				Index:     1,
				TaskTitle: "Task 01",
				CodeFile:  "task_01.md",
				IDE:       "codex",
			},
		),
		mustEvent(
			t,
			runID,
			2,
			time.Date(2026, 4, 17, 20, 0, 2, 0, time.UTC),
			events.EventKindSessionUpdate,
			kinds.SessionUpdatePayload{
				Index: 1,
				Update: kinds.SessionUpdate{
					Kind:   kinds.UpdateKindAgentMessageChunk,
					Status: kinds.StatusRunning,
					Blocks: []kinds.ContentBlock{mustTextBlock(t, "assistant reply")},
				},
			},
		),
		mustEvent(
			t,
			runID,
			3,
			time.Date(2026, 4, 17, 20, 0, 3, 0, time.UTC),
			events.EventKindSessionUpdate,
			kinds.SessionUpdatePayload{
				Index: 1,
				Update: kinds.SessionUpdate{
					Kind:       kinds.UpdateKindToolCallStarted,
					Status:     kinds.StatusRunning,
					ToolCallID: "tool-1",
				},
			},
		),
		mustEvent(
			t,
			runID,
			4,
			time.Date(2026, 4, 17, 20, 0, 4, 0, time.UTC),
			events.EventKindUsageUpdated,
			kinds.UsageUpdatedPayload{
				Index: 1,
				Usage: kinds.Usage{InputTokens: 10, OutputTokens: 5},
			},
		),
		mustEvent(
			t,
			runID,
			5,
			time.Date(2026, 4, 17, 20, 0, 5, 0, time.UTC),
			events.EventKindTaskMemoryUpdated,
			kinds.TaskMemoryUpdatedPayload{
				Path: "/tmp/task_03.md",
				Mode: "append",
			},
		),
		mustEvent(
			t,
			runID,
			6,
			time.Date(2026, 4, 17, 20, 0, 6, 0, time.UTC),
			events.EventKindSessionCompleted,
			kinds.SessionCompletedPayload{
				Index: 1,
				Usage: kinds.Usage{InputTokens: 11, OutputTokens: 6},
			},
		),
	}

	if err := db.StoreEventBatch(context.Background(), eventsToStore); err != nil {
		t.Fatalf("StoreEventBatch(): %v", err)
	}

	hookRecord := HookRunRecord{
		ID:          "hook-1",
		HookName:    "prompt.post_build",
		Source:      "planner",
		Outcome:     "success",
		DurationNS:  int64(15 * time.Millisecond),
		PayloadJSON: `{"ok":true}`,
		RecordedAt:  time.Date(2026, 4, 17, 20, 0, 7, 0, time.UTC),
	}
	if err := db.RecordHookRun(context.Background(), hookRecord); err != nil {
		t.Fatalf("RecordHookRun(): %v", err)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("Close(): %v", err)
	}

	reopened, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open(reopen): %v", err)
	}
	defer func() {
		_ = reopened.Close()
	}()

	maxSeq, err := reopened.CurrentMaxSequence(context.Background())
	if err != nil {
		t.Fatalf("CurrentMaxSequence(): %v", err)
	}
	if maxSeq != 6 {
		t.Fatalf("CurrentMaxSequence() = %d, want 6", maxSeq)
	}

	storedEvents, err := reopened.ListEvents(context.Background(), 1, 0)
	if err != nil {
		t.Fatalf("ListEvents(): %v", err)
	}
	if got := collectedSeqs(storedEvents.Events); !reflect.DeepEqual(got, []uint64{1, 2, 3, 4, 5, 6}) {
		t.Fatalf("stored event seqs = %v, want [1 2 3 4 5 6]", got)
	}
	for _, item := range storedEvents.Events {
		if item.RunID != runID {
			t.Fatalf("event run id = %q, want %q", item.RunID, runID)
		}
	}

	jobRows, err := reopened.ListJobState(context.Background())
	if err != nil {
		t.Fatalf("ListJobState(): %v", err)
	}
	if len(jobRows) != 1 {
		t.Fatalf("job row count = %d, want 1", len(jobRows))
	}
	if got, want := jobRows[0].JobID, "job-001"; got != want {
		t.Fatalf("job id = %q, want %q", got, want)
	}
	if got, want := jobRows[0].Status, "queued"; got != want {
		t.Fatalf("job status = %q, want %q", got, want)
	}
	if got, want := jobRows[0].TaskID, "Task 01"; got != want {
		t.Fatalf("job task id = %q, want %q", got, want)
	}

	transcriptRows, err := reopened.ListTranscriptMessages(context.Background())
	if err != nil {
		t.Fatalf("ListTranscriptMessages(): %v", err)
	}
	if len(transcriptRows) != 2 {
		t.Fatalf("transcript row count = %d, want 2", len(transcriptRows))
	}
	if got, want := transcriptRows[0].Role, "assistant"; got != want {
		t.Fatalf("transcript role = %q, want %q", got, want)
	}
	if got, want := transcriptRows[0].Content, "assistant reply"; got != want {
		t.Fatalf("transcript content = %q, want %q", got, want)
	}
	if got, want := transcriptRows[1].Role, "tool_call"; got != want {
		t.Fatalf("transcript role = %q, want %q", got, want)
	}
	if got, want := transcriptRows[1].Content, "tool_call:tool-1"; got != want {
		t.Fatalf("transcript content = %q, want %q", got, want)
	}

	tokenRows, err := reopened.ListTokenUsage(context.Background())
	if err != nil {
		t.Fatalf("ListTokenUsage(): %v", err)
	}
	if len(tokenRows) != 1 {
		t.Fatalf("token usage row count = %d, want 1", len(tokenRows))
	}
	if got, want := tokenRows[0].TurnID, "session-001"; got != want {
		t.Fatalf("token usage turn id = %q, want %q", got, want)
	}
	if got, want := tokenRows[0].InputTokens, 11; got != want {
		t.Fatalf("token usage input = %d, want %d", got, want)
	}
	if got, want := tokenRows[0].OutputTokens, 6; got != want {
		t.Fatalf("token usage output = %d, want %d", got, want)
	}
	if got, want := tokenRows[0].TotalTokens, 17; got != want {
		t.Fatalf("token usage total = %d, want %d", got, want)
	}

	artifactRows, err := reopened.ListArtifactSyncLog(context.Background())
	if err != nil {
		t.Fatalf("ListArtifactSyncLog(): %v", err)
	}
	if len(artifactRows) != 1 {
		t.Fatalf("artifact sync row count = %d, want 1", len(artifactRows))
	}
	if got, want := artifactRows[0].RelativePath, "/tmp/task_03.md"; got != want {
		t.Fatalf("artifact sync path = %q, want %q", got, want)
	}
	if got, want := artifactRows[0].ChangeKind, "append"; got != want {
		t.Fatalf("artifact sync change kind = %q, want %q", got, want)
	}

	hookRows, err := reopened.ListHookRuns(context.Background())
	if err != nil {
		t.Fatalf("ListHookRuns(): %v", err)
	}
	if len(hookRows) != 1 {
		t.Fatalf("hook row count = %d, want 1", len(hookRows))
	}
	if !reflect.DeepEqual(hookRows[0], HookRunRecord{
		ID:          "hook-1",
		HookName:    "prompt.post_build",
		Source:      "planner",
		Outcome:     "success",
		DurationNS:  int64(15 * time.Millisecond),
		PayloadJSON: `{"ok":true}`,
		RecordedAt:  time.Date(2026, 4, 17, 20, 0, 7, 0, time.UTC),
	}) {
		t.Fatalf("unexpected hook row: %#v", hookRows[0])
	}
}

func TestRunDBProjectsUserMessageTranscriptRows(t *testing.T) {
	t.Parallel()

	runID := "run-store-user-message"
	db := openTestRunDB(t, runID)
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatalf("Close(): %v", err)
		}
	}()

	event := mustEvent(
		t,
		runID,
		1,
		time.Date(2026, 6, 15, 12, 30, 0, 0, time.UTC),
		events.EventKindSessionUpdate,
		kinds.SessionUpdatePayload{
			Index: 0,
			Update: kinds.SessionUpdate{
				Kind:      kinds.UpdateKindUserMessageChunk,
				MessageID: "msg-1",
				Status:    kinds.StatusRunning,
				Blocks:    []kinds.ContentBlock{mustTextBlock(t, "please continue")},
			},
		},
	)
	echo := mustEvent(
		t,
		runID,
		2,
		time.Date(2026, 6, 15, 12, 30, 1, 0, time.UTC),
		events.EventKindSessionUpdate,
		kinds.SessionUpdatePayload{
			Index: 0,
			Update: kinds.SessionUpdate{
				Kind:      kinds.UpdateKindUserMessageChunk,
				MessageID: "msg-1",
				Status:    kinds.StatusRunning,
				Blocks:    []kinds.ContentBlock{mustTextBlock(t, "please continue")},
			},
		},
	)
	if err := db.StoreEventBatch(context.Background(), []events.Event{event, echo}); err != nil {
		t.Fatalf("StoreEventBatch(): %v", err)
	}

	rows, err := db.ListTranscriptMessages(context.Background())
	if err != nil {
		t.Fatalf("ListTranscriptMessages(): %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("transcript rows = %#v, want one user row", rows)
	}
	if rows[0].Role != "user" || rows[0].Content != "please continue" {
		t.Fatalf("transcript row = %#v, want user message", rows[0])
	}
}

func TestRunDBProjectsIndexZeroLifecycleToCanonicalJobID(t *testing.T) {
	t.Parallel()

	runID := "run-index-zero-lifecycle"
	db := openTestRunDB(t, runID)
	defer func() {
		_ = db.Close()
	}()

	startedAt := time.Date(2026, 6, 15, 18, 30, 0, 0, time.UTC)
	items := []events.Event{
		mustEvent(
			t,
			runID,
			1,
			startedAt,
			events.EventKindJobQueued,
			kinds.JobQueuedPayload{Index: 0, SafeName: "job-000", TaskTitle: "Pause me"},
		),
		mustEvent(
			t,
			runID,
			2,
			startedAt.Add(time.Second),
			events.EventKindJobPausing,
			kinds.JobPausingPayload{JobAttemptInfo: kinds.JobAttemptInfo{Index: 0}},
		),
		mustEvent(
			t,
			runID,
			3,
			startedAt.Add(2*time.Second),
			events.EventKindJobPaused,
			kinds.JobPausedPayload{JobAttemptInfo: kinds.JobAttemptInfo{Index: 0}},
		),
		mustEvent(
			t,
			runID,
			4,
			startedAt.Add(3*time.Second),
			events.EventKindJobResumed,
			kinds.JobResumedPayload{JobAttemptInfo: kinds.JobAttemptInfo{Index: 0}},
		),
	}
	if err := db.StoreEventBatch(context.Background(), items); err != nil {
		t.Fatalf("StoreEventBatch() error = %v", err)
	}

	rows, err := db.ListJobState(context.Background())
	if err != nil {
		t.Fatalf("ListJobState() error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("job rows = %#v, want one canonical row", rows)
	}
	if got, want := rows[0].JobID, "job-000"; got != want {
		t.Fatalf("job id = %q, want %q", got, want)
	}
	if got, want := rows[0].Status, "running"; got != want {
		t.Fatalf("job status = %q, want %q", got, want)
	}
}

func TestRunDBProjectsStalledAndParkedJobStates(t *testing.T) {
	t.Parallel()

	t.Run("Should project stalled then parked states", func(t *testing.T) {
		t.Parallel()

		runID := "run-parked-projection"
		db := openTestRunDB(t, runID)
		t.Cleanup(func() {
			if err := db.Close(); err != nil {
				t.Errorf("close run database: %v", err)
			}
		})
		startedAt := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
		initial := []events.Event{
			mustEvent(t, runID, 1, startedAt, events.EventKindJobQueued, kinds.JobQueuedPayload{
				Index: 0, SafeName: "review-batch-001", TaskTitle: "Fix review batch",
			}),
			mustEvent(t, runID, 2, startedAt.Add(time.Second), events.EventKindJobStalled, kinds.JobStalledPayload{
				JobAttemptInfo: kinds.JobAttemptInfo{Index: 0, Attempt: 1, MaxAttempts: 2},
				Reason:         "no activity",
				LastToolCall:   "tool-bash",
			}),
		}
		if err := db.StoreEventBatch(context.Background(), initial); err != nil {
			t.Fatalf("StoreEventBatch(stalled) error = %v", err)
		}

		rows, err := db.ListJobState(context.Background())
		if err != nil {
			t.Fatalf("ListJobState(stalled) error = %v", err)
		}
		if len(rows) != 1 || rows[0].Status != "stalled" {
			t.Fatalf("stalled job rows = %#v, want one stalled row", rows)
		}
		if !strings.Contains(rows[0].SummaryJSON, `"last_tool_call":"tool-bash"`) {
			t.Fatalf("job summary = %s, want stalled diagnostics", rows[0].SummaryJSON)
		}

		parked := mustEvent(
			t,
			runID,
			3,
			startedAt.Add(2*time.Second),
			events.EventKindJobParked,
			kinds.JobParkedPayload{
				JobAttemptInfo: kinds.JobAttemptInfo{Index: 0, Attempt: 2, MaxAttempts: 2},
				Reason:         "stalled again",
				WorktreePath:   "/tmp/review-job",
			},
		)
		if err := db.StoreEventBatch(context.Background(), []events.Event{parked}); err != nil {
			t.Fatalf("StoreEventBatch(parked) error = %v", err)
		}

		rows, err = db.ListJobState(context.Background())
		if err != nil {
			t.Fatalf("ListJobState(parked) error = %v", err)
		}
		if len(rows) != 1 || rows[0].Status != "parked" {
			t.Fatalf("parked job rows = %#v, want one parked row", rows)
		}
		if !strings.Contains(rows[0].SummaryJSON, `"worktree_path":"/tmp/review-job"`) {
			t.Fatalf("job summary = %s, want parked triage payload", rows[0].SummaryJSON)
		}
	})
}

func TestRunDBRecordHookRunValidatesRequiredFields(t *testing.T) {
	t.Parallel()

	db := openTestRunDB(t, "run-hook-validate")
	defer func() {
		_ = db.Close()
	}()

	err := db.RecordHookRun(context.Background(), HookRunRecord{})
	if err == nil {
		t.Fatal("RecordHookRun() error = nil, want non-nil")
	}
}

func TestRunDBAppendSyntheticEventUsesNextSequence(t *testing.T) {
	t.Parallel()

	runID := "run-synthetic-crash"
	db := openTestRunDB(t, runID)
	defer func() {
		_ = db.Close()
	}()

	first := mustEvent(
		t,
		runID,
		1,
		time.Date(2026, 4, 17, 20, 0, 1, 0, time.UTC),
		events.EventKindRunStarted,
		kinds.RunStartedPayload{Mode: "task"},
	)
	if err := db.StoreEventBatch(context.Background(), []events.Event{first}); err != nil {
		t.Fatalf("StoreEventBatch(first) error = %v", err)
	}

	appended, err := db.AppendSyntheticEvent(context.Background(), events.EventKindRunCrashed, kinds.RunCrashedPayload{
		Error: "daemon restarted before terminal state flush",
	})
	if err != nil {
		t.Fatalf("AppendSyntheticEvent() error = %v", err)
	}
	if appended.Seq != 2 {
		t.Fatalf("synthetic event sequence = %d, want 2", appended.Seq)
	}
	if appended.Kind != events.EventKindRunCrashed {
		t.Fatalf("synthetic event kind = %q, want %q", appended.Kind, events.EventKindRunCrashed)
	}

	lastEvent, err := db.LastEvent(context.Background())
	if err != nil {
		t.Fatalf("LastEvent() error = %v", err)
	}
	if lastEvent == nil || lastEvent.Seq != 2 || lastEvent.Kind != events.EventKindRunCrashed {
		t.Fatalf("last event = %#v, want seq=2 kind=run.crashed", lastEvent)
	}
}

func TestRunDBListEventsRespectsLimitAndHasMore(t *testing.T) {
	t.Parallel()

	runID := "run-limit-window"
	db := openTestRunDB(t, runID)
	defer func() {
		_ = db.Close()
	}()

	items := make([]events.Event, 0, 5)
	startedAt := time.Date(2026, 4, 17, 20, 30, 0, 0, time.UTC)
	for seq := 1; seq <= 5; seq++ {
		items = append(items, mustEvent(
			t,
			runID,
			uint64(seq),
			startedAt.Add(time.Duration(seq)*time.Second),
			events.EventKindJobQueued,
			kinds.JobQueuedPayload{Index: seq},
		))
	}
	if err := db.StoreEventBatch(context.Background(), items); err != nil {
		t.Fatalf("StoreEventBatch() error = %v", err)
	}

	window, err := db.ListEvents(context.Background(), 2, 2)
	if err != nil {
		t.Fatalf("ListEvents(limit) error = %v", err)
	}
	if !window.HasMore {
		t.Fatal("HasMore = false, want true")
	}
	if got := collectedSeqs(window.Events); !reflect.DeepEqual(got, []uint64{2, 3, 4}) {
		t.Fatalf("window seqs = %v, want [2 3 4]", got)
	}

	finalWindow, err := db.ListEvents(context.Background(), 4, 2)
	if err != nil {
		t.Fatalf("ListEvents(final limit) error = %v", err)
	}
	if finalWindow.HasMore {
		t.Fatal("final HasMore = true, want false")
	}
	if got := collectedSeqs(finalWindow.Events); !reflect.DeepEqual(got, []uint64{4, 5}) {
		t.Fatalf("final window seqs = %v, want [4 5]", got)
	}
}

func TestRunDBCompactHistoricalReadsAvoidUnboundedSessionPayloads(t *testing.T) {
	t.Parallel()

	runID := "run-compact-history"
	db := openTestRunDB(t, runID)
	defer func() {
		_ = db.Close()
	}()

	startedAt := time.Date(2026, 4, 28, 18, 0, 0, 0, time.UTC)
	items := []events.Event{
		mustEvent(
			t,
			runID,
			1,
			startedAt,
			events.EventKindJobQueued,
			kinds.JobQueuedPayload{Index: 0, SafeName: "batch_001", IDE: "codex"},
		),
		mustEvent(
			t,
			runID,
			2,
			startedAt.Add(time.Second),
			events.EventKindSessionUpdate,
			kinds.SessionUpdatePayload{
				Index: 0,
				Update: kinds.SessionUpdate{
					Kind:       kinds.UpdateKindToolCallStarted,
					Status:     kinds.StatusRunning,
					ToolCallID: "tool-1",
					Blocks: []kinds.ContentBlock{
						mustToolUseBlock(t, "tool-1", "Bash", `{"command":"make verify"}`),
					},
				},
			},
		),
		mustEvent(
			t,
			runID,
			3,
			startedAt.Add(2*time.Second),
			events.EventKindSessionUpdate,
			kinds.SessionUpdatePayload{
				Index: 0,
				Update: kinds.SessionUpdate{
					Kind:       kinds.UpdateKindToolCallUpdated,
					Status:     kinds.StatusRunning,
					ToolCallID: "tool-1",
					Blocks: []kinds.ContentBlock{
						mustToolResultBlock(t, "tool-1", strings.Repeat("x", 4096)),
					},
				},
			},
		),
		mustEvent(
			t,
			runID,
			4,
			startedAt.Add(3*time.Second),
			events.EventKindSessionUpdate,
			kinds.SessionUpdatePayload{
				Index: 0,
				Update: kinds.SessionUpdate{
					Kind:       kinds.UpdateKindToolCallUpdated,
					Status:     kinds.StatusCompleted,
					ToolCallID: "tool-1",
					Blocks: []kinds.ContentBlock{
						mustToolResultBlock(t, "tool-1", "final output"),
					},
				},
			},
		),
		mustEvent(
			t,
			runID,
			5,
			startedAt.Add(4*time.Second),
			events.EventKindSessionUpdate,
			kinds.SessionUpdatePayload{
				Index: 0,
				Update: kinds.SessionUpdate{
					Kind:   kinds.UpdateKindAgentMessageChunk,
					Status: kinds.StatusRunning,
					Blocks: []kinds.ContentBlock{
						mustTextBlock(t, "done"),
					},
				},
			},
		),
		mustEvent(
			t,
			runID,
			6,
			startedAt.Add(5*time.Second),
			events.EventKindJobCompleted,
			kinds.JobCompletedPayload{
				JobAttemptInfo: kinds.JobAttemptInfo{Index: 0, Attempt: 1, MaxAttempts: 1},
			},
		),
	}
	if err := db.StoreEventBatch(context.Background(), items); err != nil {
		t.Fatalf("StoreEventBatch() error = %v", err)
	}

	lifecycleEvents, err := db.ListEventsByKind(context.Background(), []events.EventKind{
		events.EventKindJobQueued,
		events.EventKindJobCompleted,
	})
	if err != nil {
		t.Fatalf("ListEventsByKind() error = %v", err)
	}
	if got := collectedSeqs(lifecycleEvents); !reflect.DeepEqual(got, []uint64{1, 6}) {
		t.Fatalf("lifecycle seqs = %v, want [1 6]", got)
	}

	compacted, err := db.ListCompactedSessionUpdateEvents(context.Background())
	if err != nil {
		t.Fatalf("ListCompactedSessionUpdateEvents() error = %v", err)
	}
	if got := collectedSeqs(compacted); !reflect.DeepEqual(got, []uint64{2, 4, 5}) {
		t.Fatalf("compacted seqs = %v, want [2 4 5]", got)
	}
	if strings.Contains(string(compacted[1].Payload), strings.Repeat("x", 64)) {
		t.Fatal("compacted session updates kept the superseded large tool update")
	}

	tail, err := db.ListTranscriptMessagesTail(context.Background(), 2, 0)
	if err != nil {
		t.Fatalf("ListTranscriptMessagesTail() error = %v", err)
	}
	if got := transcriptSeqs(tail); !reflect.DeepEqual(got, []uint64{4, 5}) {
		t.Fatalf("transcript tail seqs = %v, want [4 5]", got)
	}

	stats, err := db.EventAuditStats(context.Background())
	if err != nil {
		t.Fatalf("EventAuditStats() error = %v", err)
	}
	if stats.EventCount != 6 || stats.MaxSequence != 6 || stats.SessionUpdateCount != 4 ||
		stats.TranscriptMessageCount != 4 {
		t.Fatalf("unexpected audit stats: %#v", stats)
	}
}

func TestRunDBCompactedSessionUpdatesScopeToolCallsByJob(t *testing.T) {
	t.Parallel()

	const runID = "run-compact-tool-collision"
	db := openTestRunDB(t, runID)
	defer func() {
		_ = db.Close()
	}()

	startedAt := time.Date(2026, 7, 15, 2, 0, 0, 0, time.UTC)
	items := []events.Event{
		mustEvent(t, runID, 1, startedAt, events.EventKindSessionUpdate, kinds.SessionUpdatePayload{
			Index: 0,
			Update: kinds.SessionUpdate{
				Kind:       kinds.UpdateKindToolCallStarted,
				Status:     kinds.StatusRunning,
				ToolCallID: "shared-call",
			},
		}),
		mustEvent(t, runID, 2, startedAt.Add(time.Second), events.EventKindSessionUpdate, kinds.SessionUpdatePayload{
			Index: 0,
			Update: kinds.SessionUpdate{
				Kind:       kinds.UpdateKindToolCallUpdated,
				Status:     kinds.StatusCompleted,
				ToolCallID: "shared-call",
			},
		}),
		mustEvent(t, runID, 3, startedAt.Add(2*time.Second), events.EventKindSessionUpdate, kinds.SessionUpdatePayload{
			Index: 1,
			Update: kinds.SessionUpdate{
				Kind:       kinds.UpdateKindToolCallStarted,
				Status:     kinds.StatusRunning,
				ToolCallID: "shared-call",
			},
		}),
		mustEvent(t, runID, 4, startedAt.Add(3*time.Second), events.EventKindSessionUpdate, kinds.SessionUpdatePayload{
			Index: 1,
			Update: kinds.SessionUpdate{
				Kind:       kinds.UpdateKindToolCallUpdated,
				Status:     kinds.StatusCompleted,
				ToolCallID: "shared-call",
			},
		}),
	}
	if err := db.StoreEventBatch(context.Background(), items); err != nil {
		t.Fatalf("StoreEventBatch() error = %v", err)
	}

	compacted, err := db.ListCompactedSessionUpdateEvents(context.Background())
	if err != nil {
		t.Fatalf("ListCompactedSessionUpdateEvents() error = %v", err)
	}
	if got := collectedSeqs(compacted); !reflect.DeepEqual(got, []uint64{1, 2, 3, 4}) {
		t.Fatalf("compacted seqs = %v, want each job's first and latest tool update", got)
	}
}

func TestRunDBRequiresContext(t *testing.T) {
	t.Parallel()

	db := openTestRunDB(t, "run-context")
	defer func() {
		_ = db.Close()
	}()

	var nilCtx context.Context
	if err := db.StoreEventBatch(nilCtx, []events.Event{{Seq: 1}}); err == nil {
		t.Fatal("StoreEventBatch(nil) error = nil, want non-nil")
	}
	if _, err := db.ListEvents(nilCtx, 0, 0); err == nil {
		t.Fatal("ListEvents(nil) error = nil, want non-nil")
	}
}

func TestRunDBUpsertIntegrityIsStickyAndSurvivesReopen(t *testing.T) {
	t.Parallel()

	runID := "run-integrity-sticky"
	db := openTestRunDB(t, runID)
	path := db.Path()

	first, err := db.UpsertIntegrity(context.Background(), RunIntegrityUpdate{
		Incomplete:           true,
		Reasons:              []string{"journal_submit_drops"},
		JournalTerminalDrops: 2,
	})
	if err != nil {
		t.Fatalf("UpsertIntegrity(first) error = %v", err)
	}
	if !first.Incomplete {
		t.Fatal("first.Incomplete = false, want true")
	}
	if got, want := first.Reasons, []string{"journal_submit_drops"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("first.Reasons = %#v, want %#v", got, want)
	}
	if first.JournalTerminalDrops != 2 || first.JournalNonTerminalDrops != 0 {
		t.Fatalf("first drop counts = %#v, want terminal=2 non_terminal=0", first)
	}
	if first.FirstDetectedAt.IsZero() || first.UpdatedAt.IsZero() {
		t.Fatalf("first timestamps = %#v, want non-zero", first)
	}

	second, err := db.UpsertIntegrity(context.Background(), RunIntegrityUpdate{
		Reasons:                 []string{"event_gap", "transcript_gap"},
		JournalTerminalDrops:    1,
		JournalNonTerminalDrops: 3,
	})
	if err != nil {
		t.Fatalf("UpsertIntegrity(second) error = %v", err)
	}
	if got, want := second.Reasons, []string{
		"event_gap",
		"journal_submit_drops",
		"transcript_gap",
	}; !reflect.DeepEqual(
		got,
		want,
	) {
		t.Fatalf("second.Reasons = %#v, want %#v", got, want)
	}
	if second.JournalTerminalDrops != 2 || second.JournalNonTerminalDrops != 3 {
		t.Fatalf("second drop counts = %#v, want terminal=2 non_terminal=3", second)
	}
	if !second.FirstDetectedAt.Equal(first.FirstDetectedAt) {
		t.Fatalf("FirstDetectedAt = %v, want %v", second.FirstDetectedAt, first.FirstDetectedAt)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("Close(): %v", err)
	}

	reopened, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open(reopen): %v", err)
	}
	defer func() {
		_ = reopened.Close()
	}()

	persisted, err := reopened.GetIntegrity(context.Background())
	if err != nil {
		t.Fatalf("GetIntegrity(reopen): %v", err)
	}
	if got, want := persisted.Reasons, []string{
		"event_gap",
		"journal_submit_drops",
		"transcript_gap",
	}; !reflect.DeepEqual(
		got,
		want,
	) {
		t.Fatalf("persisted.Reasons = %#v, want %#v", got, want)
	}
	if persisted.JournalTerminalDrops != 2 || persisted.JournalNonTerminalDrops != 3 {
		t.Fatalf("persisted drop counts = %#v, want terminal=2 non_terminal=3", persisted)
	}
}

func TestRunDBUpsertIntegritySerializesConcurrentMerges(t *testing.T) {
	t.Parallel()

	db := openTestRunDB(t, "run-integrity-concurrent")
	defer func() {
		_ = db.Close()
	}()

	releaseNow := make(chan struct{})
	nowEntered := make(chan struct{}, 8)
	db.now = func() time.Time {
		nowEntered <- struct{}{}
		<-releaseNow
		return time.Date(2026, 4, 17, 18, 5, 0, 0, time.UTC)
	}

	errs := make(chan error, 2)
	firstUpdate := RunIntegrityUpdate{Incomplete: true, Reasons: []string{"event_gap"}}
	secondUpdate := RunIntegrityUpdate{Incomplete: true, Reasons: []string{"transcript_gap"}}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := db.UpsertIntegrity(context.Background(), firstUpdate)
		errs <- err
	}()

	<-nowEntered

	secondStarted := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		close(secondStarted)
		_, err := db.UpsertIntegrity(context.Background(), secondUpdate)
		errs <- err
	}()

	<-secondStarted
	close(releaseNow)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("UpsertIntegrity(concurrent) error = %v", err)
		}
	}

	state, err := db.GetIntegrity(context.Background())
	if err != nil {
		t.Fatalf("GetIntegrity() error = %v", err)
	}
	if !state.Incomplete {
		t.Fatal("state.Incomplete = false, want true")
	}
	if got, want := state.Reasons, []string{"event_gap", "transcript_gap"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("state.Reasons = %#v, want %#v", got, want)
	}
}

func openTestRunDB(t *testing.T, runID string) *RunDB {
	t.Helper()

	return openTestRunDBAtPath(t, filepath.Join(t.TempDir(), runID, "run.db"))
}

func openTestRunDBAtPath(t *testing.T, path string) *RunDB {
	t.Helper()

	db, err := openWithOptions(
		context.Background(),
		path,
		openOptions{
			now: func() time.Time {
				return time.Date(2026, 4, 17, 18, 0, 0, 0, time.UTC)
			},
		},
	)
	if err != nil {
		t.Fatalf("openWithOptions(): %v", err)
	}
	return db
}

func mustEvent(
	t *testing.T,
	runID string,
	seq uint64,
	timestamp time.Time,
	kind events.EventKind,
	payload any,
) events.Event {
	t.Helper()

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return events.Event{
		SchemaVersion: events.SchemaVersion,
		RunID:         runID,
		Seq:           seq,
		Timestamp:     timestamp.UTC(),
		Kind:          kind,
		Payload:       raw,
	}
}

func mustTextBlock(t *testing.T, text string) kinds.ContentBlock {
	t.Helper()

	block, err := kinds.NewContentBlock(kinds.TextBlock{
		Type: kinds.BlockText,
		Text: text,
	})
	if err != nil {
		t.Fatalf("NewContentBlock(): %v", err)
	}
	return block
}

func mustToolUseBlock(t *testing.T, id string, name string, input string) kinds.ContentBlock {
	t.Helper()

	block, err := kinds.NewContentBlock(kinds.ToolUseBlock{
		ID:    id,
		Name:  name,
		Input: json.RawMessage(input),
	})
	if err != nil {
		t.Fatalf("NewContentBlock(tool use): %v", err)
	}
	return block
}

func mustToolResultBlock(t *testing.T, id string, content string) kinds.ContentBlock {
	t.Helper()

	block, err := kinds.NewContentBlock(kinds.ToolResultBlock{
		ToolUseID: id,
		Content:   content,
	})
	if err != nil {
		t.Fatalf("NewContentBlock(tool result): %v", err)
	}
	return block
}

func collectedSeqs(items []events.Event) []uint64 {
	seqs := make([]uint64, 0, len(items))
	for _, item := range items {
		seqs = append(seqs, item.Seq)
	}
	return seqs
}

func transcriptSeqs(items []TranscriptMessageRow) []uint64 {
	seqs := make([]uint64, 0, len(items))
	for _, item := range items {
		seqs = append(seqs, item.Sequence)
	}
	return seqs
}
