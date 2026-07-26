package globaldb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/automation"
	hookspkg "github.com/compozy/agh/internal/hooks"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	watchpkg "github.com/compozy/agh/internal/loop/watch"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/sessiondb"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
)

const watchEventsGenerationOutputEnqueuedForTest = "enqueued"

func TestGlobalDBWatchEventsReadMatches(t *testing.T) {
	t.Parallel()

	t.Run("Should read task events strictly after cursor with join-scoped workspace projection", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openLoopTestGlobalDB(t, "ws-a", "ws-b")
		parent := workspaceTaskRecordForTest("watch-parent", "ws-a")
		if err := globalDB.CreateTask(ctx, parent); err != nil {
			t.Fatalf("CreateTask(parent) error = %v", err)
		}
		child := workspaceTaskRecordForTest("watch-child", "ws-a")
		child.ParentTaskID = parent.ID
		if err := globalDB.CreateTask(ctx, child); err != nil {
			t.Fatalf("CreateTask(child) error = %v", err)
		}
		foreign := workspaceTaskRecordForTest("watch-foreign", "ws-b")
		if err := globalDB.CreateTask(ctx, foreign); err != nil {
			t.Fatalf("CreateTask(foreign) error = %v", err)
		}
		base := time.Date(2026, 7, 8, 14, 0, 0, 0, time.UTC)
		appendTaskWatchEventForTest(ctx, t, globalDB, child.ID, base, "ready")
		appendTaskWatchEventForTest(ctx, t, globalDB, child.ID, base.Add(time.Minute), "blocked")
		appendTaskWatchEventForTest(ctx, t, globalDB, foreign.ID, base.Add(2*time.Minute), "blocked")

		cursors, err := globalDB.ReadCursors(ctx, looppkg.WatchEventsQuery{
			WorkspaceID: "ws-a",
			Streams:     map[string]int64{looppkg.WatchEventsTaskStream: 0},
			Kinds:       []string{string(hookspkg.HookTaskStatusChanged)},
			Limit:       10,
		})
		if err != nil {
			t.Fatalf("ReadCursors() error = %v", err)
		}
		if got, want := cursors[looppkg.WatchEventsTaskStream], int64(2); got != want {
			t.Fatalf("task cursor = %d, want %d", got, want)
		}

		events, err := globalDB.ReadMatches(ctx, looppkg.WatchEventsQuery{
			WorkspaceID: "ws-a",
			Streams:     map[string]int64{looppkg.WatchEventsTaskStream: 1},
			Kinds:       []string{string(hookspkg.HookTaskStatusChanged)},
			Limit:       10,
		})
		if err != nil {
			t.Fatalf("ReadMatches() error = %v", err)
		}
		if got, want := len(events), 1; got != want {
			t.Fatalf("events len = %d, want %d: %#v", got, want, events)
		}
		event := events[0]
		if event.Seq != 2 || event.TaskID != child.ID || event.WorkspaceID != "ws-a" {
			t.Fatalf("task event projection = %#v", event)
		}
		if got, want := event.Payload["parent_task_id"], parent.ID; got != want {
			t.Fatalf("parent_task_id = %v, want %q", got, want)
		}
		assertWatchEventRFC3339UTC(t, event.At)

		appendTaskWatchEventForTest(ctx, t, globalDB, parent.ID, base.Add(3*time.Minute), "completed")
		rootEvents, err := globalDB.ReadMatches(ctx, looppkg.WatchEventsQuery{
			WorkspaceID: "ws-a",
			Streams:     map[string]int64{looppkg.WatchEventsTaskStream: 2},
			Kinds:       []string{string(hookspkg.HookTaskStatusChanged)},
			Limit:       10,
		})
		if err != nil {
			t.Fatalf("ReadMatches(root) error = %v", err)
		}
		if got, want := len(rootEvents), 1; got != want {
			t.Fatalf("root events len = %d, want %d: %#v", got, want, rootEvents)
		}
		if got, want := rootEvents[0].Payload["parent_task_id"], ""; got != want {
			t.Fatalf("root parent_task_id = %v, want empty string", got)
		}
	})

	t.Run("Should apply limit per stream without starving loop events", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openLoopTestGlobalDB(t, "ws-a")
		taskRecord := workspaceTaskRecordForTest("watch-hot-task", "ws-a")
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		base := time.Date(2026, 7, 8, 15, 0, 0, 0, time.UTC)
		appendTaskWatchEventForTest(ctx, t, globalDB, taskRecord.ID, base, "ready")
		appendTaskWatchEventForTest(ctx, t, globalDB, taskRecord.ID, base.Add(time.Minute), "blocked")
		loopRun := testLoopRun("watch-loop-run", base, looppkg.StatusRunning)
		loopRun.WorkspaceID = "ws-a"
		if _, err := globalDB.CreateLoopRunForStart(ctx, loopRun, dsl.ConcurrencyAllow); err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		for index := range 256 {
			if err := appendLoopRunEventWithExecutor(
				ctx,
				globalDB.db,
				loopRun.ID,
				loopRun.WorkspaceID,
				loopRunEventStatusChanged,
				map[string]string{"from": "paused", "to": "running", "status": "running"},
				base.Add(time.Duration(index+1)*time.Millisecond),
			); err != nil {
				t.Fatalf("append non-terminal loop status event %d error = %v", index, err)
			}
		}
		if err := globalDB.CompareAndSwapLoopRunStatus(
			ctx,
			loopRun.ID,
			looppkg.StatusRunning,
			looppkg.StatusDone,
			looppkg.TransitionCauseContract,
			base.Add(2*time.Minute),
		); err != nil {
			t.Fatalf("CompareAndSwapLoopRunStatus() error = %v", err)
		}
		cursors, err := globalDB.ReadCursors(ctx, looppkg.WatchEventsQuery{
			WorkspaceID: "ws-a",
			Streams: map[string]int64{
				looppkg.WatchEventsTaskStream: 0,
				looppkg.WatchEventsLoopStream: 0,
			},
			Kinds: []string{
				string(hookspkg.HookTaskStatusChanged),
				"status_changed",
			},
			Limit: 1,
		})
		if err != nil {
			t.Fatalf("ReadCursors(loop stream) error = %v", err)
		}
		if cursors[looppkg.WatchEventsLoopStream] == 0 {
			t.Fatalf("loop cursor = %d, want non-zero", cursors[looppkg.WatchEventsLoopStream])
		}

		events, err := globalDB.ReadMatches(ctx, looppkg.WatchEventsQuery{
			WorkspaceID: "ws-a",
			Streams: map[string]int64{
				looppkg.WatchEventsTaskStream: 0,
				looppkg.WatchEventsLoopStream: 0,
			},
			Kinds: []string{
				string(hookspkg.HookTaskStatusChanged),
				"status_changed",
			},
			Limit: 1,
		})
		if err != nil {
			t.Fatalf("ReadMatches() error = %v", err)
		}
		counts := watchEventCountsByStream(events)
		if got, want := counts[looppkg.WatchEventsTaskStream], 1; got != want {
			t.Fatalf("task stream count = %d, want %d", got, want)
		}
		if got, want := counts[looppkg.WatchEventsLoopStream], 1; got != want {
			t.Fatalf("loop stream count = %d, want %d; events=%#v", got, want, events)
		}
		for _, event := range events {
			assertWatchEventRFC3339UTC(t, event.At)
			if event.Stream == looppkg.WatchEventsLoopStream && event.Payload["to"] != string(looppkg.StatusDone) {
				t.Fatalf("loop event payload = %#v, want terminal done status", event.Payload)
			}
		}
	})

	t.Run("Should scope loop cursors and matches to the requested workspace", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openLoopTestGlobalDB(t, "ws-a", "ws-b")
		base := time.Date(2026, 7, 8, 16, 0, 0, 0, time.UTC)
		local := testLoopRun("watch-loop-local", base, looppkg.StatusRunning)
		local.WorkspaceID = "ws-a"
		if _, err := globalDB.CreateLoopRunForStart(ctx, local, dsl.ConcurrencyAllow); err != nil {
			t.Fatalf("CreateLoopRunForStart(local) error = %v", err)
		}
		if err := globalDB.CompareAndSwapLoopRunStatus(
			ctx,
			local.ID,
			looppkg.StatusRunning,
			looppkg.StatusDone,
			looppkg.TransitionCauseContract,
			base.Add(time.Minute),
		); err != nil {
			t.Fatalf("CompareAndSwapLoopRunStatus(local) error = %v", err)
		}
		foreign := testLoopRun("watch-loop-foreign", base.Add(2*time.Minute), looppkg.StatusRunning)
		foreign.WorkspaceID = "ws-b"
		if _, err := globalDB.CreateLoopRunForStart(ctx, foreign, dsl.ConcurrencyAllow); err != nil {
			t.Fatalf("CreateLoopRunForStart(foreign) error = %v", err)
		}
		if err := globalDB.CompareAndSwapLoopRunStatus(
			ctx,
			foreign.ID,
			looppkg.StatusRunning,
			looppkg.StatusDone,
			looppkg.TransitionCauseContract,
			base.Add(3*time.Minute),
		); err != nil {
			t.Fatalf("CompareAndSwapLoopRunStatus(foreign) error = %v", err)
		}

		events, err := globalDB.ReadMatches(ctx, looppkg.WatchEventsQuery{
			WorkspaceID: "ws-a",
			Streams:     map[string]int64{looppkg.WatchEventsLoopStream: 0},
			Kinds:       []string{"status_changed"},
			Limit:       10,
		})
		if err != nil {
			t.Fatalf("ReadMatches(loop workspace) error = %v", err)
		}
		if got, want := len(events), 1; got != want {
			t.Fatalf("loop events len = %d, want %d: %#v", got, want, events)
		}
		if got := events[0]; got.WorkspaceID != "ws-a" || got.LoopRunID != string(local.ID) {
			t.Fatalf("loop event = %#v, want local workspace/run", got)
		}
		cursors, err := globalDB.ReadCursors(ctx, looppkg.WatchEventsQuery{
			WorkspaceID: "ws-a",
			Streams:     map[string]int64{looppkg.WatchEventsLoopStream: 0},
			Kinds:       []string{"status_changed"},
			Limit:       10,
		})
		if err != nil {
			t.Fatalf("ReadCursors(loop workspace) error = %v", err)
		}
		if got, want := cursors[looppkg.WatchEventsLoopStream], events[0].Seq; got != want {
			t.Fatalf("loop cursor = %d, want local terminal seq %d", got, want)
		}
	})

	t.Run("Should read automation terminal rows with join-scoped workspace projection", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openLoopTestGlobalDB(t, "ws-a", "ws-b")
		jobA, err := globalDB.CreateJob(
			ctx,
			automationJobForTest(
				automation.AutomationScopeWorkspace,
				"watch-auto-a",
				"ws-a",
				automation.JobSourceDynamic,
			),
		)
		if err != nil {
			t.Fatalf("CreateJob(ws-a) error = %v", err)
		}
		jobB, err := globalDB.CreateJob(
			ctx,
			automationJobForTest(
				automation.AutomationScopeWorkspace,
				"watch-auto-b",
				"ws-b",
				automation.JobSourceDynamic,
			),
		)
		if err != nil {
			t.Fatalf("CreateJob(ws-b) error = %v", err)
		}
		base := time.Date(2026, 7, 8, 20, 0, 0, 0, time.UTC)
		runningSeed := automationRunForJob(jobA.ID, automation.RunRunning, 1, base)
		runningSeed.EndedAt = nil
		running, err := globalDB.CreateRun(ctx, runningSeed)
		if err != nil {
			t.Fatalf("CreateRun(running) error = %v", err)
		}
		if _, err := globalDB.CreateRun(
			ctx,
			automationRunForJob(jobA.ID, automation.RunCompleted, 1, base.Add(time.Minute)),
		); err != nil {
			t.Fatalf("CreateRun(seed completed) error = %v", err)
		}
		cursorBefore, err := globalDB.ReadCursors(ctx, looppkg.WatchEventsQuery{
			WorkspaceID: "ws-a",
			Streams:     map[string]int64{looppkg.WatchEventsAutomationStream: 0},
			Kinds:       []string{string(hookspkg.HookAutomationRunCompleted)},
			Limit:       10,
		})
		if err != nil {
			t.Fatalf("ReadCursors(before terminal) error = %v", err)
		}
		running.Status = automation.RunCompleted
		running.SessionID = "sess-auto-watch"
		running.EndedAt = timePointer(base.Add(3 * time.Minute))
		updated, err := globalDB.UpdateRun(ctx, running)
		if err != nil {
			t.Fatalf("UpdateRun(terminal) error = %v", err)
		}
		if _, err := globalDB.CreateRun(
			ctx,
			automationRunForJob(jobB.ID, automation.RunCompleted, 1, base.Add(4*time.Minute)),
		); err != nil {
			t.Fatalf("CreateRun(foreign completed) error = %v", err)
		}

		events, err := globalDB.ReadMatches(ctx, looppkg.WatchEventsQuery{
			WorkspaceID: "ws-a",
			Streams: map[string]int64{
				looppkg.WatchEventsAutomationStream: cursorBefore[looppkg.WatchEventsAutomationStream],
			},
			Kinds: []string{string(hookspkg.HookAutomationRunCompleted)},
			Limit: 10,
		})
		if err != nil {
			t.Fatalf("ReadMatches(automation) error = %v", err)
		}
		if got, want := len(events), 1; got != want {
			t.Fatalf("automation events len = %d, want %d: %#v", got, want, events)
		}
		event := events[0]
		if event.RunID != updated.ID || event.WorkspaceID != "ws-a" ||
			event.Seq <= cursorBefore[looppkg.WatchEventsAutomationStream] {
			t.Fatalf(
				"automation event projection = %#v, cursorBefore=%d",
				event,
				cursorBefore[looppkg.WatchEventsAutomationStream],
			)
		}
		if got, want := event.Payload[watchEventsPayloadJobIDKey], jobA.ID; got != want {
			t.Fatalf("automation job_id = %v, want %q", got, want)
		}
		if got, want := event.Payload[watchEventsPayloadDurationMSKey], (3 * time.Minute).Milliseconds(); got != want {
			t.Fatalf("automation duration_ms = %v, want %d", got, want)
		}
		assertWatchEventRFC3339UTC(t, event.At)

		failedJob := automationJobForTest(
			automation.AutomationScopeWorkspace,
			"watch-auto-failed",
			"ws-a",
			automation.JobSourceDynamic,
		)
		failedJob.Retry = automation.DefaultBackoffRetryConfig()
		createdFailedJob, err := globalDB.CreateJob(ctx, failedJob)
		if err != nil {
			t.Fatalf("CreateJob(failed automation) error = %v", err)
		}
		failedRun := automationRunForJob(createdFailedJob.ID, automation.RunFailed, 1, base.Add(5*time.Minute))
		failedRun.SessionID = "sess-auto-failed"
		failedRun.Error = "agent crashed"
		failed, err := globalDB.CreateRun(ctx, failedRun)
		if err != nil {
			t.Fatalf("CreateRun(failed automation) error = %v", err)
		}

		failedEvents, err := globalDB.ReadMatches(ctx, looppkg.WatchEventsQuery{
			WorkspaceID: "ws-a",
			Streams:     map[string]int64{looppkg.WatchEventsAutomationStream: 0},
			Kinds:       []string{string(hookspkg.HookAutomationRunFailed)},
			Limit:       10,
		})
		if err != nil {
			t.Fatalf("ReadMatches(automation failed) error = %v", err)
		}
		if got, want := len(failedEvents), 1; got != want {
			t.Fatalf("automation failed events len = %d, want %d: %#v", got, want, failedEvents)
		}
		failedEvent := failedEvents[0]
		if failedEvent.Kind != string(hookspkg.HookAutomationRunFailed) || failedEvent.RunID != failed.ID {
			t.Fatalf("automation failed event = %#v", failedEvent)
		}
		if got, want := failedEvent.Payload[watchEventsPayloadWillRetryKey], true; got != want {
			t.Fatalf("automation will_retry = %v, want %t", got, want)
		}
		if got, want := failedEvent.Payload[watchEventsPayloadErrorKey], "agent crashed"; got != want {
			t.Fatalf("automation error = %v, want %q", got, want)
		}
	})

	t.Run("Should preserve automation terminal events across run deletion and sequence reuse", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openLoopTestGlobalDB(t, "ws-a")
		job, err := globalDB.CreateJob(
			ctx,
			automationJobForTest(
				automation.AutomationScopeWorkspace,
				"watch-auto-durable-sequence",
				"ws-a",
				automation.JobSourceDynamic,
			),
		)
		if err != nil {
			t.Fatalf("CreateJob() error = %v", err)
		}
		base := time.Date(2026, 7, 8, 20, 20, 0, 0, time.UTC)
		first, err := globalDB.CreateRun(
			ctx,
			automationRunForJob(job.ID, automation.RunCompleted, 1, base),
		)
		if err != nil {
			t.Fatalf("CreateRun(first terminal) error = %v", err)
		}
		cursors, err := globalDB.ReadCursors(ctx, looppkg.WatchEventsQuery{
			WorkspaceID: "ws-a",
			Streams:     map[string]int64{looppkg.WatchEventsAutomationStream: 0},
			Kinds:       []string{string(hookspkg.HookAutomationRunCompleted)},
			Limit:       10,
		})
		if err != nil {
			t.Fatalf("ReadCursors(first terminal) error = %v", err)
		}
		firstCursor := cursors[looppkg.WatchEventsAutomationStream]
		if firstCursor == 0 {
			t.Fatal("first automation cursor = 0, want durable sequence")
		}
		if err := globalDB.DeleteRun(ctx, first.ID); err != nil {
			t.Fatalf("DeleteRun(first terminal) error = %v", err)
		}
		preserved, err := globalDB.ReadMatches(ctx, looppkg.WatchEventsQuery{
			WorkspaceID: "ws-a",
			Streams:     map[string]int64{looppkg.WatchEventsAutomationStream: 0},
			Kinds:       []string{string(hookspkg.HookAutomationRunCompleted)},
			Limit:       10,
		})
		if err != nil {
			t.Fatalf("ReadMatches(after delete) error = %v", err)
		}
		if got, want := len(preserved), 1; got != want || preserved[0].RunID != first.ID {
			t.Fatalf("preserved automation events = %#v, want run %q", preserved, first.ID)
		}

		second, err := globalDB.CreateRun(
			ctx,
			automationRunForJob(job.ID, automation.RunCompleted, 1, base.Add(time.Minute)),
		)
		if err != nil {
			t.Fatalf("CreateRun(second terminal) error = %v", err)
		}
		resumed, err := globalDB.ReadMatches(ctx, looppkg.WatchEventsQuery{
			WorkspaceID: "ws-a",
			Streams:     map[string]int64{looppkg.WatchEventsAutomationStream: firstCursor},
			Kinds:       []string{string(hookspkg.HookAutomationRunCompleted)},
			Limit:       10,
		})
		if err != nil {
			t.Fatalf("ReadMatches(after sequence cursor) error = %v", err)
		}
		if got, want := len(resumed), 1; got != want || resumed[0].RunID != second.ID {
			t.Fatalf("resumed automation events = %#v, want run %q", resumed, second.ID)
		}
		if resumed[0].Seq <= firstCursor {
			t.Fatalf("second automation seq = %d, want > %d", resumed[0].Seq, firstCursor)
		}
	})

	t.Run("Should surface malformed automation retry metadata", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openLoopTestGlobalDB(t, "ws-a")
		job := automationJobForTest(
			automation.AutomationScopeWorkspace,
			"watch-auto-malformed-retry",
			"ws-a",
			automation.JobSourceDynamic,
		)
		job.Retry = automation.DefaultBackoffRetryConfig()
		createdJob, err := globalDB.CreateJob(ctx, job)
		if err != nil {
			t.Fatalf("CreateJob() error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE automation_jobs SET retry = '{' WHERE id = ?`,
			createdJob.ID,
		); err != nil {
			t.Fatalf("corrupt automation retry error = %v", err)
		}
		if _, err := globalDB.CreateRun(
			ctx,
			automationRunForJob(
				createdJob.ID,
				automation.RunFailed,
				1,
				time.Date(2026, 7, 8, 20, 25, 0, 0, time.UTC),
			),
		); err != nil {
			t.Fatalf("CreateRun(failed) error = %v", err)
		}

		_, err = globalDB.ReadMatches(ctx, looppkg.WatchEventsQuery{
			WorkspaceID: "ws-a",
			Streams:     map[string]int64{looppkg.WatchEventsAutomationStream: 0},
			Kinds:       []string{string(hookspkg.HookAutomationRunFailed)},
			Limit:       10,
		})
		if err == nil || !strings.Contains(err.Error(), "automation.watch_events.retry") {
			t.Fatalf("ReadMatches(malformed retry) error = %v, want retry decode context", err)
		}
	})

	t.Run("Should read network work transitions with workspace-column scoping", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openLoopTestGlobalDB(t, "ws-a", "ws-b")
		registerWatchSessionForTest(ctx, t, globalDB, "coder.sess-a", "ws-a", "coder")
		registerWatchSessionForTest(ctx, t, globalDB, "reviewer.sess-a", "ws-a", "reviewer")
		registerWatchSessionForTest(ctx, t, globalDB, "coder.sess-b", "ws-b", "coder")
		registerWatchSessionForTest(ctx, t, globalDB, "reviewer.sess-b", "ws-b", "reviewer")
		base := time.Date(2026, 7, 8, 20, 30, 0, 0, time.UTC)
		opening := networkWatchThreadMessageForTest(
			"ws-a",
			"msg-work-open",
			"thread_watch",
			"coder.sess-a",
			"please review",
			base,
		)
		opening.PeerTo = "reviewer.sess-a"
		opening.WorkID = "work-watch"
		if _, err := globalDB.WriteConversationMessage(ctx, opening); err != nil {
			t.Fatalf("WriteConversationMessage(opening) error = %v", err)
		}
		transition := networkWatchTraceMessageForTest(
			"ws-a",
			"msg-work-transition",
			"thread_watch",
			"reviewer.sess-a",
			"work-watch",
			store.NetworkWorkStateWorking,
			base.Add(time.Minute),
		)
		if _, err := globalDB.WriteConversationMessage(ctx, transition); err != nil {
			t.Fatalf("WriteConversationMessage(transition) error = %v", err)
		}
		foreign := networkWatchThreadMessageForTest(
			"ws-b",
			"msg-work-open-foreign",
			"thread_watch_foreign",
			"coder.sess-b",
			"please review",
			base,
		)
		foreign.PeerTo = "reviewer.sess-b"
		foreign.WorkID = "work-foreign"
		if _, err := globalDB.WriteConversationMessage(ctx, foreign); err != nil {
			t.Fatalf("WriteConversationMessage(foreign opening) error = %v", err)
		}

		threadEvents, err := globalDB.ReadMatches(ctx, looppkg.WatchEventsQuery{
			WorkspaceID: "ws-a",
			Streams:     map[string]int64{looppkg.WatchEventsNetworkStream: 0},
			Kinds:       []string{string(hookspkg.HookNetworkThreadOpened)},
			Limit:       10,
		})
		if err != nil {
			t.Fatalf("ReadMatches(thread opened) error = %v", err)
		}
		if got, want := len(threadEvents), 1; got != want {
			t.Fatalf("thread opened events len = %d, want %d: %#v", got, want, threadEvents)
		}
		if got, want := threadEvents[0].Payload[watchEventsPayloadThreadIDKey], "thread_watch"; got != want {
			t.Fatalf("thread_id = %v, want %q", got, want)
		}

		directID, err := networkWatchDirectIDForTest("ws-a", "builders", "coder.sess-a", "reviewer.sess-a")
		if err != nil {
			t.Fatalf("networkWatchDirectIDForTest() error = %v", err)
		}
		directMessage := networkWatchDirectMessageForTest(
			"ws-a",
			"msg-direct-open",
			directID,
			"coder.sess-a",
			"reviewer.sess-a",
			"open direct",
			base.Add(2*time.Minute),
		)
		if _, err := globalDB.WriteConversationMessage(ctx, directMessage); err != nil {
			t.Fatalf("WriteConversationMessage(direct) error = %v", err)
		}
		directEvents, err := globalDB.ReadMatches(ctx, looppkg.WatchEventsQuery{
			WorkspaceID: "ws-a",
			Streams:     map[string]int64{looppkg.WatchEventsNetworkStream: 0},
			Kinds:       []string{string(hookspkg.HookNetworkDirectRoomOpened)},
			Limit:       10,
		})
		if err != nil {
			t.Fatalf("ReadMatches(direct opened) error = %v", err)
		}
		if got, want := len(directEvents), 1; got != want {
			t.Fatalf("direct opened events len = %d, want %d: %#v", got, want, directEvents)
		}
		if got, want := directEvents[0].Payload[watchEventsPayloadDirectIDKey], directID; got != want {
			t.Fatalf("direct_id = %v, want %q", got, want)
		}

		events, err := globalDB.ReadMatches(ctx, looppkg.WatchEventsQuery{
			WorkspaceID: "ws-a",
			Streams:     map[string]int64{looppkg.WatchEventsNetworkStream: 0},
			Kinds:       []string{string(hookspkg.HookNetworkWorkTransitioned)},
			Limit:       10,
		})
		if err != nil {
			t.Fatalf("ReadMatches(network) error = %v", err)
		}
		if got, want := len(events), 1; got != want {
			t.Fatalf("network events len = %d, want %d: %#v", got, want, events)
		}
		event := events[0]
		if event.WorkspaceID != "ws-a" || event.Channel != "builders" || event.WorkID != "work-watch" {
			t.Fatalf("network event projection = %#v", event)
		}
		if got, want := event.Payload[watchEventsPayloadWorkStateKey], store.NetworkWorkStateWorking; got != want {
			t.Fatalf("network work_state = %v, want %q", got, want)
		}
		if got, want := event.Payload[taskRunResultKindKey], store.NetworkKindTrace; got != want {
			t.Fatalf("network payload kind = %v, want %q", got, want)
		}
		cursors, err := globalDB.ReadCursors(ctx, looppkg.WatchEventsQuery{
			WorkspaceID: "ws-a",
			Streams:     map[string]int64{looppkg.WatchEventsNetworkStream: 0},
			Kinds:       []string{string(hookspkg.HookNetworkWorkTransitioned)},
			Limit:       10,
		})
		if err != nil {
			t.Fatalf("ReadCursors(network) error = %v", err)
		}
		if got, want := cursors[looppkg.WatchEventsNetworkStream], event.Seq; got != want {
			t.Fatalf("network cursor = %d, want event seq %d", got, want)
		}
	})

	t.Run("Should replay coordinator observe rows monotonically across restart", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 9, 9, 0, 0, 0, time.UTC)
		globalDB := openLoopTestGlobalDB(t, "ws-a", "ws-b")
		appendCoordinatorWatchSummaryForTest(
			ctx,
			t,
			globalDB,
			hookspkg.HookCoordinatorStopped,
			"ws-a",
			"coord-a-before",
			now,
		)
		cursors, err := globalDB.ReadCursors(ctx, looppkg.WatchEventsQuery{
			WorkspaceID: "ws-a",
			Streams:     map[string]int64{looppkg.WatchEventsObserveStream: 0},
			Kinds:       []string{string(hookspkg.HookCoordinatorStopped)},
			Limit:       10,
		})
		if err != nil {
			t.Fatalf("ReadCursors(coordinator before restart) error = %v", err)
		}
		cursor := cursors[looppkg.WatchEventsObserveStream]
		if cursor == 0 {
			t.Fatal("coordinator observe cursor = 0, want rowid anchor")
		}
		path := globalDB.path
		if err := globalDB.Close(ctx); err != nil {
			t.Fatalf("Close(globalDB before restart) error = %v", err)
		}
		reopened, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(restart) error = %v", err)
		}
		t.Cleanup(func() {
			if err := reopened.Close(testutil.Context(t)); err != nil {
				t.Fatalf("Close(reopened) error = %v", err)
			}
		})
		appendCoordinatorWatchSummaryForTest(
			ctx,
			t,
			reopened,
			hookspkg.HookCoordinatorStopped,
			"ws-a",
			"coord-a-after",
			now.Add(time.Second),
		)
		appendCoordinatorWatchSummaryForTest(
			ctx,
			t,
			reopened,
			hookspkg.HookCoordinatorStopped,
			"ws-b",
			"coord-b-after",
			now.Add(2*time.Second),
		)

		events, err := reopened.ReadMatches(ctx, looppkg.WatchEventsQuery{
			WorkspaceID: "ws-a",
			Streams:     map[string]int64{looppkg.WatchEventsObserveStream: cursor},
			Kinds:       []string{string(hookspkg.HookCoordinatorStopped)},
			Limit:       10,
		})
		if err != nil {
			t.Fatalf("ReadMatches(coordinator after restart) error = %v", err)
		}
		if got, want := len(events), 1; got != want {
			t.Fatalf("coordinator events len = %d, want %d: %#v", got, want, events)
		}
		event := events[0]
		if event.Seq <= cursor ||
			event.Kind != string(hookspkg.HookCoordinatorStopped) ||
			event.Stream != looppkg.WatchEventsObserveStream ||
			event.WorkspaceID != "ws-a" ||
			event.SessionID != "coord-a-after" {
			t.Fatalf("coordinator event = %#v, want ws-a row after restart", event)
		}
		if got, want := event.Payload[watchEventsPayloadCoordinatorSessionIDKey], "coord-a-after"; got != want {
			t.Fatalf("coordinator_session_id = %v, want %q", got, want)
		}
		assertWatchEventRFC3339UTC(t, event.At)
	})

	t.Run("Should read session post-record rows without exposing content", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 9, 10, 0, 0, 0, time.UTC)
		globalDB := openLoopTestGlobalDB(t, "ws-a", "ws-b")
		registerWatchSessionForTest(ctx, t, globalDB, "sess-watch-a", "ws-a", "coder")
		registerWatchSessionForTest(ctx, t, globalDB, "sess-watch-b", "ws-b", "reviewer")
		appendSessionWatchEventForTest(
			ctx,
			t,
			globalDB,
			"sess-watch-a",
			store.SessionEvent{
				TurnID:    "turn-target-1",
				Type:      "agent_message",
				AgentName: "coder",
				Content:   `{"text":"do not leak target content"}`,
				Timestamp: now,
			},
		)
		appendSessionWatchEventForTest(
			ctx,
			t,
			globalDB,
			"sess-watch-b",
			store.SessionEvent{
				TurnID:    "turn-foreign-1",
				Type:      "agent_message",
				AgentName: "reviewer",
				Content:   `{"text":"do not leak foreign content"}`,
				Timestamp: now.Add(time.Second),
			},
		)
		stream := looppkg.WatchEventsSessionStreamForSession("sess-watch-a")
		cursors, err := globalDB.ReadCursors(ctx, looppkg.WatchEventsQuery{
			WorkspaceID: "ws-a",
			Streams:     map[string]int64{stream: 0},
			Kinds:       []string{string(hookspkg.HookEventPostRecord)},
			Limit:       10,
		})
		if err != nil {
			t.Fatalf("ReadCursors(event.post_record) error = %v", err)
		}
		if got, want := cursors[stream], int64(1); got != want {
			t.Fatalf("session cursor = %d, want %d", got, want)
		}
		events, err := globalDB.ReadMatches(ctx, looppkg.WatchEventsQuery{
			WorkspaceID: "ws-a",
			Streams:     map[string]int64{stream: 0},
			Kinds:       []string{string(hookspkg.HookEventPostRecord)},
			Limit:       10,
		})
		if err != nil {
			t.Fatalf("ReadMatches(event.post_record) error = %v", err)
		}
		if got, want := len(events), 1; got != want {
			t.Fatalf("session events len = %d, want %d: %#v", got, want, events)
		}
		event := events[0]
		if event.Kind != string(hookspkg.HookEventPostRecord) ||
			event.Stream != stream ||
			event.SessionID != "sess-watch-a" ||
			event.WorkspaceID != "ws-a" ||
			event.Seq != 1 {
			t.Fatalf("session event = %#v, want target session metadata", event)
		}
		if got, want := event.Payload[watchEventsPayloadRecordTypeKey], "agent_message"; got != want {
			t.Fatalf("record_type = %v, want %q", got, want)
		}
		if got, want := event.Payload[watchEventsPayloadTurnIDKey], "turn-target-1"; got != want {
			t.Fatalf("turn_id = %v, want %q", got, want)
		}
		if _, ok := event.Payload["content"]; ok {
			t.Fatalf("event.post_record payload leaked content field: %#v", event.Payload)
		}
		encoded, err := json.Marshal(event)
		if err != nil {
			t.Fatalf("Marshal(event) error = %v", err)
		}
		if strings.Contains(string(encoded), "do not leak") || strings.Contains(string(encoded), "content") {
			t.Fatalf("event.post_record encoded event leaked content: %s", encoded)
		}
	})
}

func TestGlobalDBWatchEventsCoordinatorIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should park then wake and enqueue downstream with confirmed batch", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 8, 16, 0, 0, 0, time.UTC)
		globalDB := openLoopTestGlobalDB(t, "ws-a")
		targetTask := workspaceTaskRecordForTest("watch-integration-target", "ws-a")
		if err := globalDB.CreateTask(ctx, targetTask); err != nil {
			t.Fatalf("CreateTask(target) error = %v", err)
		}
		resolved := compileWatchEventsIntegrationDefinitionForTest(t)
		loopRun := testLoopRun("watch-events-integration", now, looppkg.StatusRunning)
		pinLoopRunDefinitionForTest(t, &loopRun, resolved)
		loopRun.WorkspaceID = "ws-a"
		loopRun.Inputs = map[string]any{"target_task_id": targetTask.ID}
		created, err := globalDB.CreateLoopRunForStart(ctx, loopRun, dsl.ConcurrencyAllow)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		runner := newGlobalDBWatchEventsCoordinatorForTest(t, globalDB)
		actor := coordinatorActorContextForTest()
		firstClaim := claimCoordinatorRunForTest(
			ctx,
			t,
			globalDB,
			created.ID,
			"run-watch-events-integration-first",
			now,
		)

		firstPlan, err := runner.Run(ctx, taskpkg.RunID(firstClaim.Run.ID))
		if err != nil {
			t.Fatalf("Run(first) error = %v", err)
		}
		if firstPlan.Terminal == nil || firstPlan.Terminal.Status != string(looppkg.StatusWatching) {
			t.Fatalf("first terminal = %#v, want watching", firstPlan.Terminal)
		}
		firstResult, err := globalDB.CompleteCoordinatorAndEnqueueNext(ctx, taskpkg.CoordinatorCompletion{
			RunID:      firstClaim.Run.ID,
			ClaimToken: firstClaim.ClaimToken,
			Actor:      actor,
			Plan:       firstPlan,
			Now:        now.Add(time.Second),
		}, looppkg.NewStoreFinalizer())
		if err != nil {
			t.Fatalf("CompleteCoordinatorAndEnqueueNext(first) error = %v", err)
		}
		if got, want := coordinatorResultStatus(t, &firstResult), string(looppkg.StatusWatching); got != want {
			t.Fatalf("first loop status = %q, want %q", got, want)
		}

		appendTaskWatchEventForTest(ctx, t, globalDB, targetTask.ID, now.Add(2*time.Second), "blocked")
		wakeRun, added, err := globalDB.EnqueueLoopCoordinatorWake(
			ctx,
			string(created.ID),
			"watch-events-integration-wake",
			actor.Origin,
			now.Add(3*time.Second),
		)
		if err != nil {
			t.Fatalf("EnqueueLoopCoordinatorWake() error = %v", err)
		}
		if !added {
			t.Fatal("EnqueueLoopCoordinatorWake() added = false, want true")
		}
		secondClaim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			RunID:            wakeRun.ID,
			Scope:            taskpkg.ScopeWorkspace,
			WorkspaceID:      "ws-a",
			RunKind:          taskpkg.RunKindCoordinator,
			ClaimerSessionID: "daemon-loop-watch-events-integration",
			ClaimedBy:        &taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "loop"},
			LeaseDuration:    time.Minute,
			Now:              now.Add(4 * time.Second),
		})
		if err != nil {
			t.Fatalf("ClaimNextRun(wake) error = %v", err)
		}

		secondPlan, err := runner.Run(ctx, taskpkg.RunID(secondClaim.Run.ID))
		if err != nil {
			t.Fatalf("Run(second) error = %v", err)
		}
		if secondPlan.Terminal != nil || secondPlan.Yield {
			t.Fatalf(
				"second plan Terminal=%#v Yield=%v, want downstream enqueue",
				secondPlan.Terminal,
				secondPlan.Yield,
			)
		}
		if got, want := len(secondPlan.NodeRuns), 1; got != want {
			t.Fatalf("second NodeRuns = %d, want %d", got, want)
		}
		secondResult, err := globalDB.CompleteCoordinatorAndEnqueueNext(ctx, taskpkg.CoordinatorCompletion{
			RunID:      secondClaim.Run.ID,
			ClaimToken: secondClaim.ClaimToken,
			Actor:      actor,
			Plan:       secondPlan,
			Now:        now.Add(5 * time.Second),
		}, looppkg.NewStoreFinalizer())
		if err != nil {
			t.Fatalf("CompleteCoordinatorAndEnqueueNext(second) error = %v", err)
		}
		if got, want := len(secondResult.EnqueuedRuns), 1; got != want {
			t.Fatalf("second EnqueuedRuns = %d, want %d", got, want)
		}
		outputs, err := globalDB.ListGenerationOutputs(ctx, created.ID, 1)
		if err != nil {
			t.Fatalf("ListGenerationOutputs() error = %v", err)
		}
		byNode := watchEventsGenerationOutputsByNode(outputs)
		watchOutput := byNode["watch_tasks"]
		if watchOutput.Status != "succeeded" {
			t.Fatalf("watch output status = %q, want succeeded", watchOutput.Status)
		}
		confirmed := decodeWatchEventsConfirmedRefForTest(t, watchOutput.OutputRef)
		if got, want := confirmed.Cursors[looppkg.WatchEventsTaskStream], int64(1); got != want {
			t.Fatalf("confirmed cursor = %d, want %d", got, want)
		}
		var events []looppkg.WatchEvent
		if err := json.Unmarshal(confirmed.Events, &events); err != nil {
			t.Fatalf("Unmarshal confirmed events error = %v", err)
		}
		if got, want := len(events), 1; got != want {
			t.Fatalf("confirmed events len = %d, want %d", got, want)
		}
		if events[0].TaskID != targetTask.ID || events[0].Payload["to_status"] != "blocked" {
			t.Fatalf("confirmed event = %#v, want target blocked event", events[0])
		}
		downstream := byNode["summarize"]
		if downstream.Status != watchEventsGenerationOutputEnqueuedForTest || downstream.TaskRunID == "" {
			t.Fatalf("downstream output = %#v, want enqueued with task run", downstream)
		}
		storedRun, err := globalDB.GetTaskRun(ctx, downstream.TaskRunID)
		if err != nil {
			t.Fatalf("GetTaskRun(downstream) error = %v", err)
		}
		if storedRun.Status != taskpkg.TaskRunStatusQueued || storedRun.LoopRunID != string(created.ID) {
			t.Fatalf("downstream run = %#v, want queued loop worker", storedRun)
		}
	})

	t.Run("Should wake automation subscriptions from terminal run rows", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 8, 21, 0, 0, 0, time.UTC)
		globalDB := openLoopTestGlobalDB(t, "ws-a")
		job, err := globalDB.CreateJob(
			ctx,
			automationJobForTest(
				automation.AutomationScopeWorkspace,
				"watch-auto-integration",
				"ws-a",
				automation.JobSourceDynamic,
			),
		)
		if err != nil {
			t.Fatalf("CreateJob(automation) error = %v", err)
		}
		created := parkWatchEventsLoopWithDefinitionForTest(
			ctx,
			t,
			globalDB,
			now,
			"watch-events-automation-integration",
			map[string]any{watchEventsPayloadJobIDKey: job.ID},
			compileAutomationWatchEventsIntegrationDefinitionForTest(t),
		)
		runningSeed := automationRunForJob(job.ID, automation.RunRunning, 1, now.Add(2*time.Second))
		runningSeed.EndedAt = nil
		running, err := globalDB.CreateRun(ctx, runningSeed)
		if err != nil {
			t.Fatalf("CreateRun(running automation) error = %v", err)
		}
		running.Status = automation.RunCompleted
		running.SessionID = "sess-auto-integration"
		running.EndedAt = timePointer(now.Add(42 * time.Second))
		updated, err := globalDB.UpdateRun(ctx, running)
		if err != nil {
			t.Fatalf("UpdateRun(completed automation) error = %v", err)
		}
		actor := coordinatorActorContextForTest()
		wakeRun, added, err := globalDB.EnqueueLoopCoordinatorWake(
			ctx,
			string(created.ID),
			"watch-events-automation-integration-wake",
			actor.Origin,
			now.Add(43*time.Second),
		)
		if err != nil {
			t.Fatalf("EnqueueLoopCoordinatorWake(automation) error = %v", err)
		}
		if !added {
			t.Fatal("EnqueueLoopCoordinatorWake(automation) added = false, want true")
		}

		events, byNode := claimAndRunWatchEventsWakeForTest(
			ctx,
			t,
			globalDB,
			created,
			wakeRun,
			now.Add(44*time.Second),
			"watch_automation",
		)
		if got, want := len(events), 1; got != want {
			t.Fatalf("automation confirmed events len = %d, want %d: %#v", got, want, events)
		}
		event := events[0]
		if event.Kind != string(hookspkg.HookAutomationRunCompleted) ||
			event.RunID != updated.ID ||
			event.WorkspaceID != "ws-a" ||
			event.Stream != looppkg.WatchEventsAutomationStream {
			t.Fatalf("automation confirmed event = %#v", event)
		}
		if got, want := event.Payload[watchEventsPayloadJobIDKey], job.ID; got != want {
			t.Fatalf("automation job_id = %v, want %q", got, want)
		}
		if got, want := event.Payload[watchEventsPayloadSessionIDKey], "sess-auto-integration"; got != want {
			t.Fatalf("automation session_id = %v, want %q", got, want)
		}
		downstream := byNode["summarize"]
		if downstream.Status != watchEventsGenerationOutputEnqueuedForTest || downstream.TaskRunID == "" {
			t.Fatalf("automation downstream output = %#v, want enqueued", downstream)
		}
	})
}

func TestGlobalDBWatchEventsParkedIndexAndRecovery(t *testing.T) {
	t.Parallel()

	t.Run("Should list parked subscriptions from durable pending output refs", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 8, 18, 30, 0, 0, time.UTC)
		globalDB := openLoopTestGlobalDB(t, "ws-a")
		created, targetTask := parkWatchEventsLoopForRecoveryTest(ctx, t, globalDB, now)

		parked, err := globalDB.ListParkedWatchEventSubscriptions(ctx)
		if err != nil {
			t.Fatalf("ListParkedWatchEventSubscriptions() error = %v", err)
		}
		if got, want := len(parked), 1; got != want {
			t.Fatalf("parked len = %d, want %d", got, want)
		}
		subscription := parked[0]
		if got, want := subscription.LoopRunID, string(created.ID); got != want {
			t.Fatalf("LoopRunID = %q, want %q", got, want)
		}
		if got, want := subscription.NodeID, "watch_tasks"; got != want {
			t.Fatalf("NodeID = %q, want %q", got, want)
		}
		if got, want := subscription.Inputs["target_task_id"], targetTask.ID; got != want {
			t.Fatalf("target_task_id = %v, want %q", got, want)
		}
		if got, want := subscription.Cursors[looppkg.WatchEventsTaskStream], int64(0); got != want {
			t.Fatalf("task cursor = %d, want %d", got, want)
		}
	})

	t.Run("Should scope and page parked subscriptions with a rotating cursor", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 8, 18, 35, 0, 0, time.UTC)
		globalDB := openLoopTestGlobalDB(t, "ws-a")
		definition := compileWatchEventsIntegrationDefinitionForTest(t)
		loopIDs := []string{"watch-page-a", "watch-page-b", "watch-page-c"}
		for index, loopID := range loopIDs {
			target := workspaceTaskRecordForTest("target-"+loopID, "ws-a")
			if err := globalDB.CreateTask(ctx, target); err != nil {
				t.Fatalf("CreateTask(%s) error = %v", target.ID, err)
			}
			parkWatchEventsLoopWithDefinitionForTest(
				ctx,
				t,
				globalDB,
				now.Add(time.Duration(index)*time.Minute),
				loopID,
				map[string]any{"target_task_id": target.ID},
				definition,
			)
		}

		scoped, err := globalDB.ListParkedWatchEventSubscriptionsForLoopRun(ctx, loopIDs[1])
		if err != nil {
			t.Fatalf("ListParkedWatchEventSubscriptionsForLoopRun() error = %v", err)
		}
		if got, want := len(scoped), 1; got != want || scoped[0].LoopRunID != loopIDs[1] {
			t.Fatalf("scoped parked subscriptions = %#v, want only %q", scoped, loopIDs[1])
		}

		first, err := globalDB.ListParkedWatchEventSubscriptionsPage(
			ctx,
			looppkg.ParkedWatchEventScanCursor{},
			2,
		)
		if err != nil {
			t.Fatalf("ListParkedWatchEventSubscriptionsPage(first) error = %v", err)
		}
		if got := parkedWatchEventLoopRunIDs(first); !slices.Equal(got, loopIDs[:2]) {
			t.Fatalf("first parked page loop IDs = %#v, want %#v", got, loopIDs[:2])
		}
		second, err := globalDB.ListParkedWatchEventSubscriptionsPage(
			ctx,
			parkedWatchEventScanCursor(first[len(first)-1]),
			2,
		)
		if err != nil {
			t.Fatalf("ListParkedWatchEventSubscriptionsPage(second) error = %v", err)
		}
		if got, want := parkedWatchEventLoopRunIDs(second), loopIDs[2:]; !slices.Equal(got, want) {
			t.Fatalf("second parked page loop IDs = %#v, want %#v", got, want)
		}

		runs, next, err := globalDB.EnqueueWatchEventsGapWakesPage(
			ctx,
			coordinatorActorContextForTest().Origin,
			now.Add(4*time.Minute),
			parkedWatchEventScanCursor(second[0]),
			2,
		)
		if err != nil {
			t.Fatalf("EnqueueWatchEventsGapWakesPage(wrap) error = %v", err)
		}
		if len(runs) != 0 {
			t.Fatalf("gap wake runs = %#v, want none without ledger gaps", runs)
		}
		if got, want := next.LoopRunID, loopIDs[1]; got != want {
			t.Fatalf("wrapped next loop run ID = %q, want %q", got, want)
		}
	})

	t.Run("Should advance past a malformed subscription and recover later gaps", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 8, 18, 40, 0, 0, time.UTC)
		globalDB := openLoopTestGlobalDB(t, "ws-a")
		definition := compileWatchEventsIntegrationDefinitionForTest(t)
		poisonTask := workspaceTaskRecordForTest("watch-gap-poison-target", "ws-a")
		if err := globalDB.CreateTask(ctx, poisonTask); err != nil {
			t.Fatalf("CreateTask(poison target) error = %v", err)
		}
		poisonLoop := parkWatchEventsLoopWithDefinitionForTest(
			ctx,
			t,
			globalDB,
			now,
			"watch-gap-a-poison",
			map[string]any{"target_task_id": poisonTask.ID},
			definition,
		)
		recoveryTask := workspaceTaskRecordForTest("watch-gap-recovery-target", "ws-a")
		if err := globalDB.CreateTask(ctx, recoveryTask); err != nil {
			t.Fatalf("CreateTask(recovery target) error = %v", err)
		}
		recoveryLoop := parkWatchEventsLoopWithDefinitionForTest(
			ctx,
			t,
			globalDB,
			now.Add(time.Minute),
			"watch-gap-b-recovery",
			map[string]any{"target_task_id": recoveryTask.ID},
			definition,
		)
		poisonRef, err := watchpkg.EventsPendingOutputRef(watchpkg.EventsPendingState{
			Subscriptions: []watchpkg.EventSubscriptionRef{{Kind: "unsupported.watch.event"}},
			Cursors:       map[string]int64{looppkg.WatchEventsTaskStream: 0},
		})
		if err != nil {
			t.Fatalf("EventsPendingOutputRef(poison) error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_generation_outputs SET output_ref = ? WHERE loop_run_id = ? AND node_id = ?`,
			poisonRef,
			poisonLoop.ID,
			"watch_tasks",
		); err != nil {
			t.Fatalf("update poisoned watch-events output ref error = %v", err)
		}
		appendTaskWatchEventForTest(ctx, t, globalDB, recoveryTask.ID, now.Add(2*time.Minute), "blocked")

		runs, next, err := globalDB.EnqueueWatchEventsGapWakesPage(
			ctx,
			coordinatorActorContextForTest().Origin,
			now.Add(3*time.Minute),
			looppkg.ParkedWatchEventScanCursor{},
			2,
		)
		if err == nil || !strings.Contains(err.Error(), "watch-events kind is unsupported") {
			t.Fatalf("EnqueueWatchEventsGapWakesPage() error = %v, want unsupported-kind error", err)
		}
		if got, want := next.LoopRunID, string(recoveryLoop.ID); got != want {
			t.Fatalf("next.LoopRunID = %q, want %q", got, want)
		}
		if got, want := len(runs), 1; got != want {
			t.Fatalf("gap wake runs = %d, want %d", got, want)
		}
		if got, want := runs[0].LoopRunID, string(recoveryLoop.ID); got != want {
			t.Fatalf("gap wake loop_run_id = %q, want %q", got, want)
		}
	})

	t.Run("Should enqueue idempotent gap wakes without claiming coordinator runs", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 8, 18, 45, 0, 0, time.UTC)
		globalDB := openLoopTestGlobalDB(t, "ws-a")
		created, targetTask := parkWatchEventsLoopForRecoveryTest(ctx, t, globalDB, now)
		appendTaskWatchEventForTest(ctx, t, globalDB, targetTask.ID, now.Add(time.Second), "blocked")
		actor := coordinatorActorContextForTest()

		runs, err := globalDB.EnqueueWatchEventsGapWakes(ctx, actor.Origin, now.Add(2*time.Second))
		if err != nil {
			t.Fatalf("EnqueueWatchEventsGapWakes() error = %v", err)
		}
		if got, want := len(runs), 1; got != want {
			t.Fatalf("gap wake runs = %d, want %d", got, want)
		}
		if got, want := runs[0].LoopRunID, string(created.ID); got != want {
			t.Fatalf("gap wake loop_run_id = %q, want %q", got, want)
		}
		again, err := globalDB.EnqueueWatchEventsGapWakes(ctx, actor.Origin, now.Add(3*time.Second))
		if err != nil {
			t.Fatalf("EnqueueWatchEventsGapWakes(second) error = %v", err)
		}
		if got := len(again); got != 0 {
			t.Fatalf("second gap wake runs = %d, want coalesced 0", got)
		}
		queued, err := globalDB.ListTaskRunsByStatus(ctx, []taskpkg.RunStatus{taskpkg.TaskRunStatusQueued})
		if err != nil {
			t.Fatalf("ListTaskRunsByStatus(queued) error = %v", err)
		}
		if got, want := len(queued), 1; got != want {
			t.Fatalf("queued coordinator runs = %d, want %d", got, want)
		}
		summaries, err := globalDB.ListEventSummaries(ctx, EventSummaryQuery{Type: watchEventsWakeEnqueuedEvent})
		if err != nil {
			t.Fatalf("ListEventSummaries(wake_enqueued) error = %v", err)
		}
		if got := len(summaries); got == 0 {
			t.Fatal("wake_enqueued summaries = 0, want at least one")
		}
		coalesced, err := globalDB.ListEventSummaries(ctx, EventSummaryQuery{
			Type: "loop.watch_events.wake_coalesced",
		})
		if err != nil {
			t.Fatalf("ListEventSummaries(wake_coalesced) error = %v", err)
		}
		if got, want := len(coalesced), 1; got != want {
			t.Fatalf("wake_coalesced summaries = %d, want %d", got, want)
		}
	})

	t.Run("Should return committed wake runs when a later audit write fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 8, 18, 50, 0, 0, time.UTC)
		globalDB := openLoopTestGlobalDB(t, "ws-a")
		definition := compileWatchEventsIntegrationDefinitionForTest(t)
		firstTask := workspaceTaskRecordForTest("watch-partial-target-a", "ws-a")
		if err := globalDB.CreateTask(ctx, firstTask); err != nil {
			t.Fatalf("CreateTask(first target) error = %v", err)
		}
		secondTask := workspaceTaskRecordForTest("watch-partial-target-b", "ws-a")
		if err := globalDB.CreateTask(ctx, secondTask); err != nil {
			t.Fatalf("CreateTask(second target) error = %v", err)
		}
		firstLoop := parkWatchEventsLoopWithDefinitionForTest(
			ctx,
			t,
			globalDB,
			now,
			"watch-events-partial-a",
			map[string]any{"target_task_id": firstTask.ID},
			definition,
		)
		secondLoop := parkWatchEventsLoopWithDefinitionForTest(
			ctx,
			t,
			globalDB,
			now,
			"watch-events-partial-b",
			map[string]any{"target_task_id": secondTask.ID},
			definition,
		)
		appendTaskWatchEventForTest(ctx, t, globalDB, firstTask.ID, now.Add(2*time.Second), "blocked")
		appendTaskWatchEventForTest(ctx, t, globalDB, secondTask.ID, now.Add(3*time.Second), "blocked")
		if _, err := globalDB.db.ExecContext(ctx, `
			CREATE TRIGGER fail_second_watch_events_wake_audit
			BEFORE INSERT ON event_summaries
			WHEN NEW.type = 'loop.watch_events.wake_enqueued'
			 AND instr(NEW.content_json, '"loop_run_id":"watch-events-partial-b"') > 0
			BEGIN
				SELECT RAISE(ABORT, 'forced watch-events wake audit failure');
			END;
		`); err != nil {
			t.Fatalf("create wake audit failure trigger error = %v", err)
		}

		runs, err := globalDB.EnqueueWatchEventsGapWakes(
			ctx,
			coordinatorActorContextForTest().Origin,
			now.Add(4*time.Second),
		)
		if err == nil || !strings.Contains(err.Error(), "forced watch-events wake audit failure") {
			t.Fatalf("EnqueueWatchEventsGapWakes() error = %v, want forced audit failure", err)
		}
		if got, want := len(runs), 2; got != want {
			t.Fatalf("committed wake runs = %d, want %d", got, want)
		}
		gotLoopIDs := []string{runs[0].LoopRunID, runs[1].LoopRunID}
		wantLoopIDs := []string{string(firstLoop.ID), string(secondLoop.ID)}
		if !slices.Equal(gotLoopIDs, wantLoopIDs) {
			t.Fatalf("committed wake loop IDs = %#v, want %#v", gotLoopIDs, wantLoopIDs)
		}
	})

	t.Run("Should include watch-events gap wakes in boot reconcile", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 8, 19, 0, 0, 0, time.UTC)
		globalDB := openLoopTestGlobalDB(t, "ws-a")
		created, targetTask := parkWatchEventsLoopForRecoveryTest(ctx, t, globalDB, now)
		appendTaskWatchEventForTest(ctx, t, globalDB, targetTask.ID, now.Add(time.Second), "blocked")
		actor := coordinatorActorContextForTest()

		runs, err := globalDB.ReconcileLoopCoordinatorsOnBoot(ctx, actor.Origin, now.Add(2*time.Second))
		if err != nil {
			t.Fatalf("ReconcileLoopCoordinatorsOnBoot() error = %v", err)
		}
		if got, want := len(runs), 1; got != want {
			t.Fatalf("boot reconcile runs = %d, want %d", got, want)
		}
		if got, want := runs[0].LoopRunID, string(created.ID); got != want {
			t.Fatalf("boot reconcile loop_run_id = %q, want %q", got, want)
		}
	})

	t.Run("Should reconcile network message subscriptions from rows written while down", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 8, 21, 30, 0, 0, time.UTC)
		globalDB := openLoopTestGlobalDB(t, "ws-a", "ws-b")
		registerWatchSessionForTest(ctx, t, globalDB, "coder.sess-a", "ws-a", "coder")
		registerWatchSessionForTest(ctx, t, globalDB, "reviewer.sess-a", "ws-a", "reviewer")
		registerWatchSessionForTest(ctx, t, globalDB, "coder.sess-b", "ws-b", "coder")
		registerWatchSessionForTest(ctx, t, globalDB, "reviewer.sess-b", "ws-b", "reviewer")
		created := parkWatchEventsLoopWithDefinitionForTest(
			ctx,
			t,
			globalDB,
			now,
			"watch-events-network-recovery",
			map[string]any{
				watchEventsPayloadChannelKey: "builders",
				watchEventsPayloadWorkIDKey:  "work-watch",
			},
			compileNetworkMessageWatchEventsIntegrationDefinitionForTest(t),
		)
		message := networkWatchThreadMessageForTest(
			"ws-a",
			"msg-network-recovery",
			"thread_network_recovery",
			"coder.sess-a",
			"please inspect this work",
			now.Add(time.Second),
		)
		message.PeerTo = "reviewer.sess-a"
		message.WorkID = "work-watch"
		if _, err := globalDB.WriteConversationMessage(ctx, message); err != nil {
			t.Fatalf("WriteConversationMessage(network) error = %v", err)
		}
		foreign := networkWatchThreadMessageForTest(
			"ws-b",
			"msg-network-foreign",
			"thread_network_foreign",
			"coder.sess-b",
			"cross workspace work",
			now.Add(2*time.Second),
		)
		foreign.PeerTo = "reviewer.sess-b"
		foreign.WorkID = "work-watch"
		if _, err := globalDB.WriteConversationMessage(ctx, foreign); err != nil {
			t.Fatalf("WriteConversationMessage(foreign network) error = %v", err)
		}
		actor := coordinatorActorContextForTest()
		runs, err := globalDB.ReconcileLoopCoordinatorsOnBoot(ctx, actor.Origin, now.Add(3*time.Second))
		if err != nil {
			t.Fatalf("ReconcileLoopCoordinatorsOnBoot(network) error = %v", err)
		}
		if got, want := len(runs), 1; got != want {
			t.Fatalf("network boot reconcile runs = %d, want %d", got, want)
		}
		if got, want := runs[0].LoopRunID, string(created.ID); got != want {
			t.Fatalf("network boot reconcile loop_run_id = %q, want %q", got, want)
		}

		events, byNode := claimAndRunWatchEventsWakeForTest(
			ctx,
			t,
			globalDB,
			created,
			runs[0],
			now.Add(4*time.Second),
			"watch_network_messages",
		)
		if got, want := len(events), 1; got != want {
			t.Fatalf("network confirmed events len = %d, want %d: %#v", got, want, events)
		}
		event := events[0]
		if event.Kind != string(hookspkg.HookNetworkMessagePersisted) ||
			event.WorkspaceID != "ws-a" ||
			event.Channel != "builders" ||
			event.WorkID != "work-watch" ||
			event.Stream != looppkg.WatchEventsNetworkStream {
			t.Fatalf("network confirmed event = %#v", event)
		}
		if got, want := event.Payload[watchEventsPayloadMessageIDKey], "msg-network-recovery"; got != want {
			t.Fatalf("network message_id = %v, want %q", got, want)
		}
		if got, want := event.Payload[watchEventsPayloadWorkIDKey], "work-watch"; got != want {
			t.Fatalf("network work_id = %v, want %q", got, want)
		}
		downstream := byNode["summarize"]
		if downstream.Status != watchEventsGenerationOutputEnqueuedForTest || downstream.TaskRunID == "" {
			t.Fatalf("network downstream output = %#v, want enqueued", downstream)
		}
	})

	t.Run(
		"Should reconcile coordinator stopped subscriptions from observe rows written while down",
		func(t *testing.T) {
			t.Parallel()

			ctx := testutil.Context(t)
			now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
			globalDB := openLoopTestGlobalDB(t, "ws-a")
			created := parkWatchEventsLoopWithDefinitionForTest(
				ctx,
				t,
				globalDB,
				now,
				"watch-events-coordinator-recovery",
				map[string]any{"coordinator_session_id": "coord-target"},
				compileCoordinatorStoppedWatchEventsIntegrationDefinitionForTest(t),
			)
			appendCoordinatorWatchSummaryForTest(
				ctx,
				t,
				globalDB,
				hookspkg.HookCoordinatorStopped,
				"ws-a",
				"coord-target",
				now.Add(time.Second),
			)
			appendCoordinatorWatchSummaryForTest(
				ctx,
				t,
				globalDB,
				hookspkg.HookCoordinatorStopped,
				"ws-a",
				"coord-foreign",
				now.Add(2*time.Second),
			)
			actor := coordinatorActorContextForTest()
			runs, err := globalDB.ReconcileLoopCoordinatorsOnBoot(ctx, actor.Origin, now.Add(3*time.Second))
			if err != nil {
				t.Fatalf("ReconcileLoopCoordinatorsOnBoot(coordinator) error = %v", err)
			}
			if got, want := len(runs), 1; got != want {
				t.Fatalf("coordinator boot reconcile runs = %d, want %d", got, want)
			}
			if got, want := runs[0].LoopRunID, string(created.ID); got != want {
				t.Fatalf("coordinator boot reconcile loop_run_id = %q, want %q", got, want)
			}

			events, byNode := claimAndRunWatchEventsWakeForTest(
				ctx,
				t,
				globalDB,
				created,
				runs[0],
				now.Add(4*time.Second),
				"watch_coordinator",
			)
			if got, want := len(events), 1; got != want {
				t.Fatalf("coordinator confirmed events len = %d, want %d: %#v", got, want, events)
			}
			event := events[0]
			if event.Kind != string(hookspkg.HookCoordinatorStopped) ||
				event.WorkspaceID != "ws-a" ||
				event.SessionID != "coord-target" ||
				event.Stream != looppkg.WatchEventsObserveStream {
				t.Fatalf("coordinator confirmed event = %#v", event)
			}
			if got, want := event.Payload[watchEventsPayloadStopReasonKey], "completed"; got != want {
				t.Fatalf("coordinator stop_reason = %v, want %q", got, want)
			}
			downstream := byNode["summarize"]
			if downstream.Status != watchEventsGenerationOutputEnqueuedForTest || downstream.TaskRunID == "" {
				t.Fatalf("coordinator downstream output = %#v, want enqueued", downstream)
			}
		},
	)

	t.Run(
		"Should reconcile session-scoped event post-record subscriptions without content leakage",
		func(t *testing.T) {
			t.Parallel()

			ctx := testutil.Context(t)
			now := time.Date(2026, 7, 9, 12, 30, 0, 0, time.UTC)
			globalDB := openLoopTestGlobalDB(t, "ws-a")
			registerWatchSessionForTest(ctx, t, globalDB, "sess-target", "ws-a", "coder")
			registerWatchSessionForTest(ctx, t, globalDB, "sess-foreign", "ws-a", "reviewer")
			appendSessionWatchEventForTest(ctx, t, globalDB, "sess-target", store.SessionEvent{
				TurnID:    "turn-target-seed",
				Type:      "agent_message",
				AgentName: "coder",
				Content:   `{"text":"seed target content"}`,
				Timestamp: now,
			})
			appendSessionWatchEventForTest(ctx, t, globalDB, "sess-foreign", store.SessionEvent{
				TurnID:    "turn-foreign-seed",
				Type:      "agent_message",
				AgentName: "reviewer",
				Content:   `{"text":"seed foreign content"}`,
				Timestamp: now,
			})
			created := parkWatchEventsLoopWithDefinitionForTest(
				ctx,
				t,
				globalDB,
				now.Add(time.Second),
				"watch-events-session-recovery",
				map[string]any{watchEventsPayloadSessionIDKey: "sess-target"},
				compileEventPostRecordWatchEventsIntegrationDefinitionForTest(t),
			)
			target := appendSessionWatchEventForTest(ctx, t, globalDB, "sess-target", store.SessionEvent{
				TurnID:    "turn-target-after",
				Type:      "agent_message",
				AgentName: "coder",
				Content:   `{"text":"target content must not leak"}`,
				Timestamp: now.Add(2 * time.Second),
			})
			appendSessionWatchEventForTest(ctx, t, globalDB, "sess-foreign", store.SessionEvent{
				TurnID:    "turn-foreign-after",
				Type:      "agent_message",
				AgentName: "reviewer",
				Content:   `{"text":"foreign content must not leak"}`,
				Timestamp: now.Add(3 * time.Second),
			})
			actor := coordinatorActorContextForTest()
			runs, err := globalDB.ReconcileLoopCoordinatorsOnBoot(ctx, actor.Origin, now.Add(4*time.Second))
			if err != nil {
				t.Fatalf("ReconcileLoopCoordinatorsOnBoot(event.post_record) error = %v", err)
			}
			if got, want := len(runs), 1; got != want {
				t.Fatalf("event.post_record boot reconcile runs = %d, want %d", got, want)
			}
			if got, want := runs[0].LoopRunID, string(created.ID); got != want {
				t.Fatalf("event.post_record boot reconcile loop_run_id = %q, want %q", got, want)
			}

			events, byNode := claimAndRunWatchEventsWakeForTest(
				ctx,
				t,
				globalDB,
				created,
				runs[0],
				now.Add(5*time.Second),
				"watch_session_events",
			)
			if got, want := len(events), 1; got != want {
				t.Fatalf("event.post_record confirmed events len = %d, want %d: %#v", got, want, events)
			}
			event := events[0]
			if event.Kind != string(hookspkg.HookEventPostRecord) ||
				event.WorkspaceID != "ws-a" ||
				event.SessionID != "sess-target" ||
				event.Stream != looppkg.WatchEventsSessionStreamForSession("sess-target") ||
				event.Seq != target.Sequence {
				t.Fatalf("event.post_record confirmed event = %#v", event)
			}
			if got, want := event.Payload[watchEventsPayloadTurnIDKey], "turn-target-after"; got != want {
				t.Fatalf("event.post_record turn_id = %v, want %q", got, want)
			}
			if _, ok := event.Payload["content"]; ok {
				t.Fatalf("event.post_record payload leaked content field: %#v", event.Payload)
			}
			encoded, err := json.Marshal(event)
			if err != nil {
				t.Fatalf("Marshal(event.post_record confirmed event) error = %v", err)
			}
			if strings.Contains(string(encoded), "must not leak") || strings.Contains(string(encoded), "content") {
				t.Fatalf("event.post_record confirmed event leaked content: %s", encoded)
			}
			downstream := byNode["summarize"]
			if downstream.Status != watchEventsGenerationOutputEnqueuedForTest || downstream.TaskRunID == "" {
				t.Fatalf("event.post_record downstream output = %#v, want enqueued", downstream)
			}
		},
	)
}

func TestGlobalDBWatchEventsHelpers(t *testing.T) {
	t.Parallel()

	t.Run("Should decode watch event payload edge cases", func(t *testing.T) {
		t.Parallel()

		if payload, err := decodeWatchEventPayload(""); err != nil || len(payload) != 0 {
			t.Fatalf("decodeWatchEventPayload(empty) = (%#v, %v), want empty/nil", payload, err)
		}
		if payload, err := decodeWatchEventPayload("null"); err != nil || len(payload) != 0 {
			t.Fatalf("decodeWatchEventPayload(null) = (%#v, %v), want empty/nil", payload, err)
		}
		if _, err := decodeWatchEventPayload("{"); err == nil {
			t.Fatal("decodeWatchEventPayload(malformed) error = nil, want error")
		}
	})

	t.Run("Should build recovery coordinator wake keys", func(t *testing.T) {
		t.Parallel()

		got := watchEventsCoordinatorWakeKey(" loop-run-1 ", " watch_tasks ")
		want := "loop.coordinator.watch_events.loop-run-1.watch_tasks"
		if got != want {
			t.Fatalf("watchEventsCoordinatorWakeKey() = %q, want %q", got, want)
		}
	})

	t.Run("Should normalize benign recovery wake errors", func(t *testing.T) {
		t.Parallel()

		unexpected := errors.New("wake failed")
		cases := []struct {
			name    string
			err     error
			wantNil bool
		}{
			{name: "Should accept nil", err: nil, wantNil: true},
			{name: "Should swallow conflict", err: taskpkg.ErrConflict, wantNil: true},
			{name: "Should swallow invalid transition", err: taskpkg.ErrInvalidStatusTransition, wantNil: true},
			{name: "Should return unexpected errors", err: unexpected, wantNil: false},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				err := normalizeWatchEventsGapWakeError(tc.err)
				if (err == nil) != tc.wantNil {
					t.Fatalf("normalizeWatchEventsGapWakeError() = %v, wantNil %v", err, tc.wantNil)
				}
				if !tc.wantNil && !errors.Is(err, unexpected) {
					t.Fatalf("normalizeWatchEventsGapWakeError() = %v, want %v", err, unexpected)
				}
			})
		}
	})

	t.Run("Should summarize recovery event outcomes", func(t *testing.T) {
		t.Parallel()

		subscription := looppkg.ParkedWatchEventSubscription{NodeID: " watch_tasks "}
		cases := []struct {
			name      string
			eventType string
			want      string
		}{
			{
				name:      "Should summarize a matched gap",
				eventType: watchEventsMatchedEvent,
				want:      "watch-events gap matched for watch_tasks",
			},
			{
				name:      "Should summarize a wake error",
				eventType: watchEventsWakeErrorEvent,
				want:      "watch-events gap wake failed for watch_tasks",
			},
			{
				name:      "Should summarize a coalesced wake",
				eventType: watchEventsWakeCoalescedEvent,
				want:      "watch-events gap wake coalesced for watch_tasks",
			},
			{
				name:      "Should summarize an enqueued wake",
				eventType: watchEventsWakeEnqueuedEvent,
				want:      "watch-events gap wake enqueued for watch_tasks",
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				if got := watchEventsGapSummary(tc.eventType, subscription); got != tc.want {
					t.Fatalf("watchEventsGapSummary() = %q, want %q", got, tc.want)
				}
			})
		}
	})

	t.Run("Should stringify optional recovery errors", func(t *testing.T) {
		t.Parallel()

		if got := watchEventsErrorString(nil); got != "" {
			t.Fatalf("watchEventsErrorString(nil) = %q, want empty", got)
		}
		if got, want := watchEventsErrorString(errors.New("boom")), "boom"; got != want {
			t.Fatalf("watchEventsErrorString(error) = %q, want %q", got, want)
		}
	})
}

func parkWatchEventsLoopForRecoveryTest(
	ctx context.Context,
	t *testing.T,
	globalDB *GlobalDB,
	now time.Time,
) (looppkg.Run, taskpkg.Task) {
	t.Helper()
	targetTask := workspaceTaskRecordForTest("watch-recovery-target", "ws-a")
	if err := globalDB.CreateTask(ctx, targetTask); err != nil {
		t.Fatalf("CreateTask(target) error = %v", err)
	}
	resolved := compileWatchEventsIntegrationDefinitionForTest(t)
	loopRun := testLoopRun("watch-events-recovery", now, looppkg.StatusRunning)
	pinLoopRunDefinitionForTest(t, &loopRun, resolved)
	loopRun.WorkspaceID = "ws-a"
	loopRun.Inputs = map[string]any{"target_task_id": targetTask.ID}
	created, err := globalDB.CreateLoopRunForStart(ctx, loopRun, dsl.ConcurrencyAllow)
	if err != nil {
		t.Fatalf("CreateLoopRunForStart() error = %v", err)
	}
	runner := newGlobalDBWatchEventsCoordinatorForTest(t, globalDB)
	claim := claimCoordinatorRunForTest(ctx, t, globalDB, created.ID, "run-watch-events-recovery-first", now)
	plan, err := runner.Run(ctx, taskpkg.RunID(claim.Run.ID))
	if err != nil {
		t.Fatalf("Run(first) error = %v", err)
	}
	if plan.Terminal == nil || plan.Terminal.Status != string(looppkg.StatusWatching) {
		t.Fatalf("first terminal = %#v, want watching", plan.Terminal)
	}
	result, err := globalDB.CompleteCoordinatorAndEnqueueNext(ctx, taskpkg.CoordinatorCompletion{
		RunID:      claim.Run.ID,
		ClaimToken: claim.ClaimToken,
		Actor:      coordinatorActorContextForTest(),
		Plan:       plan,
		Now:        now.Add(time.Second),
	}, looppkg.NewStoreFinalizer())
	if err != nil {
		t.Fatalf("CompleteCoordinatorAndEnqueueNext(first) error = %v", err)
	}
	if got, want := coordinatorResultStatus(t, &result), string(looppkg.StatusWatching); got != want {
		t.Fatalf("loop status = %q, want %q", got, want)
	}
	return created, targetTask
}

func parkWatchEventsLoopWithDefinitionForTest(
	ctx context.Context,
	t *testing.T,
	globalDB *GlobalDB,
	now time.Time,
	loopRunID string,
	inputs map[string]any,
	resolved *looppkg.ResolvedDefinition,
) looppkg.Run {
	t.Helper()
	loopRun := testLoopRun(loopRunID, now, looppkg.StatusRunning)
	pinLoopRunDefinitionForTest(t, &loopRun, resolved)
	loopRun.WorkspaceID = "ws-a"
	loopRun.Inputs = inputs
	created, err := globalDB.CreateLoopRunForStart(ctx, loopRun, dsl.ConcurrencyAllow)
	if err != nil {
		t.Fatalf("CreateLoopRunForStart(%s) error = %v", loopRunID, err)
	}
	runner := newGlobalDBWatchEventsCoordinatorForTest(t, globalDB)
	claim := claimCoordinatorRunForTest(ctx, t, globalDB, created.ID, "run-"+loopRunID+"-first", now)
	plan, err := runner.Run(ctx, taskpkg.RunID(claim.Run.ID))
	if err != nil {
		t.Fatalf("Run(%s first) error = %v", loopRunID, err)
	}
	if plan.Terminal == nil || plan.Terminal.Status != string(looppkg.StatusWatching) {
		t.Fatalf("%s first terminal = %#v, want watching", loopRunID, plan.Terminal)
	}
	result, err := globalDB.CompleteCoordinatorAndEnqueueNext(ctx, taskpkg.CoordinatorCompletion{
		RunID:      claim.Run.ID,
		ClaimToken: claim.ClaimToken,
		Actor:      coordinatorActorContextForTest(),
		Plan:       plan,
		Now:        now.Add(time.Second),
	}, looppkg.NewStoreFinalizer())
	if err != nil {
		t.Fatalf("CompleteCoordinatorAndEnqueueNext(%s first) error = %v", loopRunID, err)
	}
	if got, want := coordinatorResultStatus(t, &result), string(looppkg.StatusWatching); got != want {
		t.Fatalf("%s loop status = %q, want %q", loopRunID, got, want)
	}
	return created
}

func claimAndRunWatchEventsWakeForTest(
	ctx context.Context,
	t *testing.T,
	globalDB *GlobalDB,
	loopRun looppkg.Run,
	wakeRun taskpkg.Run,
	now time.Time,
	watchNodeID string,
) ([]looppkg.WatchEvent, map[string]looppkg.GenerationOutput) {
	t.Helper()
	runner := newGlobalDBWatchEventsCoordinatorForTest(t, globalDB)
	claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
		RunID:            wakeRun.ID,
		Scope:            taskpkg.ScopeWorkspace,
		WorkspaceID:      string(loopRun.WorkspaceID),
		RunKind:          taskpkg.RunKindCoordinator,
		ClaimerSessionID: "daemon-loop-" + wakeRun.ID,
		ClaimedBy:        &taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "loop"},
		LeaseDuration:    time.Minute,
		Now:              now,
	})
	if err != nil {
		t.Fatalf("ClaimNextRun(%s) error = %v", wakeRun.ID, err)
	}
	plan, err := runner.Run(ctx, taskpkg.RunID(claim.Run.ID))
	if err != nil {
		t.Fatalf("Run(%s) error = %v", wakeRun.ID, err)
	}
	if plan.Terminal != nil || plan.Yield {
		t.Fatalf("wake plan Terminal=%#v Yield=%v, want downstream enqueue", plan.Terminal, plan.Yield)
	}
	if got, want := len(plan.NodeRuns), 1; got != want {
		t.Fatalf("wake NodeRuns = %d, want %d", got, want)
	}
	result, err := globalDB.CompleteCoordinatorAndEnqueueNext(ctx, taskpkg.CoordinatorCompletion{
		RunID:      claim.Run.ID,
		ClaimToken: claim.ClaimToken,
		Actor:      coordinatorActorContextForTest(),
		Plan:       plan,
		Now:        now.Add(time.Second),
	}, looppkg.NewStoreFinalizer())
	if err != nil {
		t.Fatalf("CompleteCoordinatorAndEnqueueNext(%s) error = %v", wakeRun.ID, err)
	}
	if got, want := len(result.EnqueuedRuns), 1; got != want {
		t.Fatalf("wake EnqueuedRuns = %d, want %d", got, want)
	}
	outputs, err := globalDB.ListGenerationOutputs(ctx, loopRun.ID, 1)
	if err != nil {
		t.Fatalf("ListGenerationOutputs(%s) error = %v", loopRun.ID, err)
	}
	byNode := watchEventsGenerationOutputsByNode(outputs)
	watchOutput := byNode[watchNodeID]
	if watchOutput.Status != "succeeded" {
		t.Fatalf("%s output status = %q, want succeeded", watchNodeID, watchOutput.Status)
	}
	confirmed := decodeWatchEventsConfirmedRefForTest(t, watchOutput.OutputRef)
	var events []looppkg.WatchEvent
	if err := json.Unmarshal(confirmed.Events, &events); err != nil {
		t.Fatalf("Unmarshal confirmed events error = %v", err)
	}
	return events, byNode
}

func appendTaskWatchEventForTest(
	ctx context.Context,
	t *testing.T,
	globalDB *GlobalDB,
	taskID string,
	at time.Time,
	toStatus string,
) {
	t.Helper()
	err := globalDB.withTaskImmediateTransaction(
		ctx,
		"append task watch event test",
		func(exec taskSQLExecutor) error {
			return appendTaskEventPayloadWithExecutor(
				ctx,
				exec,
				taskID,
				"",
				string(hookspkg.HookTaskStatusChanged),
				coordinatorActorContextForTest(),
				at,
				map[string]string{
					"from_status": string(taskpkg.TaskStatusPending),
					"to_status":   toStatus,
				},
			)
		},
	)
	if err != nil {
		t.Fatalf("appendTaskEventPayloadWithExecutor() error = %v", err)
	}
}

func appendCoordinatorWatchSummaryForTest(
	ctx context.Context,
	t *testing.T,
	globalDB *GlobalDB,
	event hookspkg.HookEvent,
	workspaceID string,
	coordinatorSessionID string,
	at time.Time,
) {
	t.Helper()
	content, err := json.Marshal(map[string]any{
		watchEventsPayloadAgentNameKey:            "coordinator-agent",
		watchEventsPayloadCoordinatorSessionIDKey: coordinatorSessionID,
		"resolved_network_participation": map[string]any{
			"version":    "network-participation/v1",
			"mode":       "live",
			"channel_id": "default",
			"source":     "explicit_request",
		},
		watchEventsPayloadWorkflowIDKey:   "wf-watch",
		watchEventsPayloadProviderKey:     "mock",
		watchEventsPayloadModelKey:        "mock-model",
		watchEventsPayloadDecisionKindKey: "stop",
		watchEventsPayloadDecisionKey:     "stop after verification",
		watchEventsPayloadStopReasonKey:   "completed",
	})
	if err != nil {
		t.Fatalf("Marshal(coordinator watch content) error = %v", err)
	}
	if err := globalDB.WriteEventSummary(ctx, EventSummary{
		SessionID:   coordinatorSessionID,
		WorkspaceID: workspaceID,
		Type:        string(event),
		AgentName:   "coordinator-agent",
		Provider:    "mock",
		Outcome:     "info",
		Content:     content,
		EventCorrelation: store.EventCorrelation{
			HookEvent:            string(event),
			CoordinatorSessionID: coordinatorSessionID,
			WorkflowID:           "wf-watch",
		},
		Summary:   "coordinator watch summary",
		Timestamp: at,
	}); err != nil {
		t.Fatalf("WriteEventSummary(%s) error = %v", event, err)
	}
}

func registerWatchSessionForTest(
	ctx context.Context,
	t *testing.T,
	globalDB *GlobalDB,
	sessionID string,
	workspaceID string,
	agentName string,
) {
	t.Helper()
	if err := globalDB.RegisterSession(ctx, SessionInfo{
		ID:          sessionID,
		Name:        sessionID,
		AgentName:   agentName,
		Provider:    "mock",
		WorkspaceID: workspaceID,
		SessionType: defaultSessionType,
		State:       globalDBSessionStateActive,
		Lineage:     store.NormalizeSessionLineage(sessionID, nil),
		CreatedAt:   time.Date(2026, 7, 9, 9, 30, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 7, 9, 9, 30, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("RegisterSession(%s) error = %v", sessionID, err)
	}
}

func appendSessionWatchEventForTest(
	ctx context.Context,
	t *testing.T,
	globalDB *GlobalDB,
	sessionID string,
	event store.SessionEvent,
) store.SessionEvent {
	t.Helper()
	sessionDir := filepath.Join(filepath.Dir(globalDB.path), "sessions", sessionID)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", sessionDir, err)
	}
	writer, err := sessiondb.OpenSessionDB(ctx, sessionID, store.SessionDBFile(sessionDir))
	if err != nil {
		t.Fatalf("OpenSessionDB(%s) error = %v", sessionID, err)
	}
	persisted, recordErr := writer.RecordPersisted(ctx, event)
	closeErr := writer.Close(ctx)
	if recordErr != nil {
		t.Fatalf("RecordPersisted(%s) error = %v", sessionID, recordErr)
	}
	if closeErr != nil {
		t.Fatalf("Close(session %s) error = %v", sessionID, closeErr)
	}
	return persisted
}

func networkWatchThreadMessageForTest(
	workspaceID string,
	messageID string,
	threadID string,
	peerFrom string,
	text string,
	timestamp time.Time,
) store.NetworkConversationMessage {
	return store.NetworkConversationMessage{
		MessageID:   messageID,
		SessionID:   peerFrom,
		WorkspaceID: workspaceID,
		Channel:     "builders",
		Surface:     store.NetworkSurfaceThread,
		ThreadID:    threadID,
		Direction:   "sent",
		PeerFrom:    peerFrom,
		Kind:        store.NetworkKindSay,
		Text:        text,
		PreviewText: text,
		Body:        []byte(`{"text":"` + text + `"}`),
		Timestamp:   timestamp,
	}
}

func networkWatchDirectIDForTest(workspaceID, channel, peerA, peerB string) (string, error) {
	directID, _, _, err := store.NetworkDirectRoomIdentity(workspaceID, channel, peerA, peerB)
	if err != nil {
		return "", fmt.Errorf("derive network direct room identity: %w", err)
	}
	return directID, nil
}

func networkWatchDirectMessageForTest(
	workspaceID string,
	messageID string,
	directID string,
	peerFrom string,
	peerTo string,
	text string,
	timestamp time.Time,
) store.NetworkConversationMessage {
	return store.NetworkConversationMessage{
		MessageID:   messageID,
		SessionID:   peerFrom,
		WorkspaceID: workspaceID,
		Channel:     "builders",
		Surface:     store.NetworkSurfaceDirect,
		DirectID:    directID,
		Direction:   "sent",
		PeerFrom:    peerFrom,
		PeerTo:      peerTo,
		Kind:        store.NetworkKindSay,
		Text:        text,
		PreviewText: text,
		Body:        []byte(`{"text":"` + text + `"}`),
		Timestamp:   timestamp,
	}
}

func networkWatchTraceMessageForTest(
	workspaceID string,
	messageID string,
	threadID string,
	peerFrom string,
	workID string,
	state string,
	timestamp time.Time,
) store.NetworkConversationMessage {
	return store.NetworkConversationMessage{
		MessageID:   messageID,
		SessionID:   peerFrom,
		WorkspaceID: workspaceID,
		Channel:     "builders",
		Surface:     store.NetworkSurfaceThread,
		ThreadID:    threadID,
		Direction:   "received",
		PeerFrom:    peerFrom,
		Kind:        store.NetworkKindTrace,
		WorkID:      workID,
		PreviewText: state,
		Body:        []byte(`{"state":"` + state + `"}`),
		Timestamp:   timestamp,
	}
}

func compileWatchEventsIntegrationDefinitionForTest(t *testing.T) *looppkg.ResolvedDefinition {
	t.Helper()
	resolved, err := looppkg.NewCompiler().Compile(dsl.Definition{
		APIVersion: dsl.APIVersion,
		Kind:       dsl.KindLoop,
		Inputs: map[string]dsl.Input{
			"target_task_id": {Type: dsl.InputTypeString},
		},
		Graph: dsl.Graph{
			Nodes: []dsl.Node{
				{
					ID:    "watch_tasks",
					Class: dsl.NodeClassSource,
					Kind:  string(dsl.SourceWatchEvents),
					Events: []dsl.EventSubscription{{
						Kind: string(hookspkg.HookTaskStatusChanged),
						Filter: "event.task_id == inputs.target_task_id" +
							" && " + `event.payload.to_status == "blocked"`,
					}},
				},
				{
					ID:    "summarize",
					Class: dsl.NodeClassAction,
					Kind:  string(dsl.ActionTransform),
					Params: dsl.NodeParams{
						"map": map[string]any{"ok": map[string]any{"value": true}},
					},
				},
			},
			Edges: []dsl.Edge{{From: "watch_tasks", To: "summarize"}},
		},
	})
	if err != nil {
		t.Fatalf("Compile(watch-events integration) error = %v", err)
	}
	return resolved
}

func compileCoordinatorStoppedWatchEventsIntegrationDefinitionForTest(t *testing.T) *looppkg.ResolvedDefinition {
	t.Helper()
	resolved, err := looppkg.NewCompiler().Compile(dsl.Definition{
		APIVersion: dsl.APIVersion,
		Kind:       dsl.KindLoop,
		Inputs: map[string]dsl.Input{
			"coordinator_session_id": {Type: dsl.InputTypeString},
		},
		Graph: dsl.Graph{
			Nodes: []dsl.Node{
				{
					ID:    "watch_coordinator",
					Class: dsl.NodeClassSource,
					Kind:  string(dsl.SourceWatchEvents),
					Events: []dsl.EventSubscription{{
						Kind: string(hookspkg.HookCoordinatorStopped),
						Filter: "event.session_id == inputs.coordinator_session_id" +
							" && " + `event.payload.stop_reason == "completed"`,
					}},
				},
				{
					ID:    "summarize",
					Class: dsl.NodeClassAction,
					Kind:  string(dsl.ActionTransform),
					Params: dsl.NodeParams{
						"map": map[string]any{"ok": map[string]any{"value": true}},
					},
				},
			},
			Edges: []dsl.Edge{{From: "watch_coordinator", To: "summarize"}},
		},
	})
	if err != nil {
		t.Fatalf("Compile(coordinator watch-events integration) error = %v", err)
	}
	return resolved
}

func compileEventPostRecordWatchEventsIntegrationDefinitionForTest(t *testing.T) *looppkg.ResolvedDefinition {
	t.Helper()
	resolved, err := looppkg.NewCompiler().Compile(dsl.Definition{
		APIVersion: dsl.APIVersion,
		Kind:       dsl.KindLoop,
		Inputs: map[string]dsl.Input{
			watchEventsPayloadSessionIDKey: {Type: dsl.InputTypeString},
		},
		Graph: dsl.Graph{
			Nodes: []dsl.Node{
				{
					ID:    "watch_session_events",
					Class: dsl.NodeClassSource,
					Kind:  string(dsl.SourceWatchEvents),
					Events: []dsl.EventSubscription{{
						Kind: string(hookspkg.HookEventPostRecord),
						Filter: "event.session_id == inputs.session_id" +
							" && " + `event.payload.record_type == "agent_message"`,
					}},
				},
				{
					ID:    "summarize",
					Class: dsl.NodeClassAction,
					Kind:  string(dsl.ActionTransform),
					Params: dsl.NodeParams{
						"map": map[string]any{"ok": map[string]any{"value": true}},
					},
				},
			},
			Edges: []dsl.Edge{{From: "watch_session_events", To: "summarize"}},
		},
	})
	if err != nil {
		t.Fatalf("Compile(event.post_record watch-events integration) error = %v", err)
	}
	return resolved
}

func compileAutomationWatchEventsIntegrationDefinitionForTest(t *testing.T) *looppkg.ResolvedDefinition {
	t.Helper()
	resolved, err := looppkg.NewCompiler().Compile(dsl.Definition{
		APIVersion: dsl.APIVersion,
		Kind:       dsl.KindLoop,
		Inputs: map[string]dsl.Input{
			watchEventsPayloadJobIDKey: {Type: dsl.InputTypeString},
		},
		Graph: dsl.Graph{
			Nodes: []dsl.Node{
				{
					ID:    "watch_automation",
					Class: dsl.NodeClassSource,
					Kind:  string(dsl.SourceWatchEvents),
					Events: []dsl.EventSubscription{{
						Kind:   string(hookspkg.HookAutomationRunCompleted),
						Filter: "event.payload.job_id == inputs.job_id",
					}},
				},
				{
					ID:    "summarize",
					Class: dsl.NodeClassAction,
					Kind:  string(dsl.ActionTransform),
					Params: dsl.NodeParams{
						"map": map[string]any{"ok": map[string]any{"value": true}},
					},
				},
			},
			Edges: []dsl.Edge{{From: "watch_automation", To: "summarize"}},
		},
	})
	if err != nil {
		t.Fatalf("Compile(automation watch-events integration) error = %v", err)
	}
	return resolved
}

func compileNetworkMessageWatchEventsIntegrationDefinitionForTest(t *testing.T) *looppkg.ResolvedDefinition {
	t.Helper()
	resolved, err := looppkg.NewCompiler().Compile(dsl.Definition{
		APIVersion: dsl.APIVersion,
		Kind:       dsl.KindLoop,
		Inputs: map[string]dsl.Input{
			watchEventsPayloadChannelKey: {Type: dsl.InputTypeString},
			watchEventsPayloadWorkIDKey:  {Type: dsl.InputTypeString},
		},
		Graph: dsl.Graph{
			Nodes: []dsl.Node{
				{
					ID:    "watch_network_messages",
					Class: dsl.NodeClassSource,
					Kind:  string(dsl.SourceWatchEvents),
					Events: []dsl.EventSubscription{{
						Kind: string(hookspkg.HookNetworkMessagePersisted),
						Filter: "event.channel == inputs.channel" +
							" && " + "event.payload.work_id == inputs.work_id",
					}},
				},
				{
					ID:    "summarize",
					Class: dsl.NodeClassAction,
					Kind:  string(dsl.ActionTransform),
					Params: dsl.NodeParams{
						"map": map[string]any{"ok": map[string]any{"value": true}},
					},
				},
			},
			Edges: []dsl.Edge{{From: "watch_network_messages", To: "summarize"}},
		},
	})
	if err != nil {
		t.Fatalf("Compile(network watch-events integration) error = %v", err)
	}
	return resolved
}

func newGlobalDBWatchEventsCoordinatorForTest(
	t *testing.T,
	globalDB *GlobalDB,
) *looppkg.CoordinatorRunner {
	t.Helper()
	runner, err := looppkg.NewCoordinatorRunner(
		globalDB,
		globalDB,
		globalDB,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		looppkg.WithCoordinatorWatchEventsLedger(globalDB),
	)
	if err != nil {
		t.Fatalf("NewCoordinatorRunner() error = %v", err)
	}
	return runner
}

func pinLoopRunDefinitionForTest(
	t *testing.T,
	run *looppkg.Run,
	resolved *looppkg.ResolvedDefinition,
) {
	t.Helper()
	if resolved == nil {
		t.Fatal("resolved Loop definition is required")
	}
	effective, err := looppkg.ResolveEffectiveConfig(
		resolved,
		looppkg.DefaultLoopDefaults(),
		nil,
		looppkg.LoopConfig{},
	)
	if err != nil {
		t.Fatalf("ResolveEffectiveConfig() error = %v", err)
	}
	snapshot, digest, err := looppkg.BuildExecutedDefinitionSnapshot(resolved, effective)
	if err != nil {
		t.Fatalf("BuildExecutedDefinitionSnapshot() error = %v", err)
	}
	run.DefinitionVersion = resolved.DefinitionVersion
	run.DefinitionDigest = digest
	run.DefinitionSnapshot = slices.Clone(snapshot)
}

func watchEventsGenerationOutputsByNode(
	outputs []looppkg.GenerationOutput,
) map[string]looppkg.GenerationOutput {
	byNode := make(map[string]looppkg.GenerationOutput, len(outputs))
	for _, output := range outputs {
		byNode[output.NodeID] = output
	}
	return byNode
}

type watchEventsConfirmedRefForTest struct {
	Kind    string           `json:"kind"`
	Events  json.RawMessage  `json:"events"`
	Cursors map[string]int64 `json:"cursors"`
}

func decodeWatchEventsConfirmedRefForTest(
	t *testing.T,
	ref string,
) watchEventsConfirmedRefForTest {
	t.Helper()
	var confirmed watchEventsConfirmedRefForTest
	if err := json.Unmarshal([]byte(ref), &confirmed); err != nil {
		t.Fatalf("Unmarshal watch-events confirmed ref error = %v", err)
	}
	if confirmed.Kind != "watch_events_confirmed" {
		t.Fatalf("watch-events ref kind = %q, want watch_events_confirmed", confirmed.Kind)
	}
	return confirmed
}

func watchEventCountsByStream(events []looppkg.WatchEvent) map[string]int {
	counts := make(map[string]int)
	for _, event := range events {
		counts[event.Stream]++
	}
	return counts
}

func parkedWatchEventLoopRunIDs(entries []looppkg.ParkedWatchEventSubscription) []string {
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, entry.LoopRunID)
	}
	return ids
}

func assertWatchEventRFC3339UTC(t *testing.T, value string) {
	t.Helper()
	if !strings.HasSuffix(value, "Z") {
		t.Fatalf("watch event at = %q, want UTC RFC3339 suffix", value)
	}
	if _, err := time.Parse(time.RFC3339Nano, value); err != nil {
		t.Fatalf("time.Parse(%q) error = %v", value, err)
	}
}

func TestGlobalDBWatchEventsReadMatchesShouldRejectInvalidQuery(t *testing.T) {
	t.Parallel()

	t.Run("Should reject unsupported watch-events streams", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openLoopTestGlobalDB(t, "ws-a")
		_, err := globalDB.ReadMatches(ctx, looppkg.WatchEventsQuery{
			WorkspaceID: "ws-a",
			Streams:     map[string]int64{"unsupported": 0},
			Kinds:       []string{string(hookspkg.HookTaskStatusChanged)},
			Limit:       1,
		})
		if err == nil {
			t.Fatal("ReadMatches() error = nil, want validation error")
		}
	})
}
