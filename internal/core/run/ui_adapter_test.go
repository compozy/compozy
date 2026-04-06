package run

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/journal"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func TestTranslateEventJobStarted(t *testing.T) {
	t.Parallel()

	msg, ok := translateEvent(mustRuntimeEventUITest(
		t,
		eventspkg.EventKindJobStarted,
		kinds.JobStartedPayload{Index: 2, Attempt: 1, MaxAttempts: 3},
	))
	if !ok {
		t.Fatal("expected job.started to translate")
	}

	started, ok := msg.(jobStartedMsg)
	if !ok {
		t.Fatalf("expected jobStartedMsg, got %T", msg)
	}
	if started.Index != 2 || started.Attempt != 1 || started.MaxAttempts != 3 {
		t.Fatalf("unexpected started payload: %#v", started)
	}
}

func TestTranslateEventSessionUpdateProducesSnapshot(t *testing.T) {
	t.Parallel()

	textBlock := mustContentBlockLoggingTest(t, model.TextBlock{Text: "hello from ACP"})
	update, err := publicSessionUpdate(model.SessionUpdate{
		Kind:   model.UpdateKindAgentMessageChunk,
		Blocks: []model.ContentBlock{textBlock},
		Status: model.StatusRunning,
	})
	if err != nil {
		t.Fatalf("publicSessionUpdate: %v", err)
	}

	msg, ok := translateEvent(mustRuntimeEventUITest(
		t,
		eventspkg.EventKindSessionUpdate,
		kinds.SessionUpdatePayload{Index: 4, Update: update},
	))
	if !ok {
		t.Fatal("expected session.update to translate")
	}

	jobUpdate, ok := msg.(jobUpdateMsg)
	if !ok {
		t.Fatalf("expected jobUpdateMsg, got %T", msg)
	}
	if jobUpdate.Index != 4 {
		t.Fatalf("expected job index 4, got %d", jobUpdate.Index)
	}
	if len(jobUpdate.Snapshot.Entries) != 1 {
		t.Fatalf("expected one transcript entry, got %#v", jobUpdate.Snapshot.Entries)
	}
	if jobUpdate.Snapshot.Entries[0].Kind != transcriptEntryAssistantMessage {
		t.Fatalf("unexpected transcript entry kind: %#v", jobUpdate.Snapshot.Entries[0])
	}
}

func TestTranslateEventUsageRetryAndFailure(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		ev   eventspkg.Event
		want any
	}{
		{
			name: "usage updated",
			ev: mustRuntimeEventUITest(
				t,
				eventspkg.EventKindUsageUpdated,
				kinds.UsageUpdatedPayload{
					Index: 1,
					Usage: kinds.Usage{InputTokens: 7, OutputTokens: 3, TotalTokens: 10},
				},
			),
			want: usageUpdateMsg{Index: 1, Usage: model.Usage{InputTokens: 7, OutputTokens: 3, TotalTokens: 10}},
		},
		{
			name: "job retry",
			ev: mustRuntimeEventUITest(
				t,
				eventspkg.EventKindJobRetryScheduled,
				kinds.JobRetryScheduledPayload{Index: 1, Attempt: 2, MaxAttempts: 4, Reason: "retry me"},
			),
			want: jobRetryMsg{Index: 1, Attempt: 2, MaxAttempts: 4, Reason: "retry me"},
		},
		{
			name: "job failed",
			ev: mustRuntimeEventUITest(
				t,
				eventspkg.EventKindJobFailed,
				kinds.JobFailedPayload{
					Index:       1,
					Attempt:     2,
					MaxAttempts: 4,
					CodeFile:    "task_01.md",
					ExitCode:    23,
					OutLog:      "task_01.out.log",
					ErrLog:      "task_01.err.log",
					Error:       "boom",
				},
			),
			want: jobFailureMsg{
				Failure: failInfo{
					codeFile: "task_01.md",
					exitCode: 23,
					outLog:   "task_01.out.log",
					errLog:   "task_01.err.log",
					err:      eventError("boom"),
				},
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			msg, ok := translateEvent(tc.ev)
			if !ok {
				t.Fatalf("expected %s to translate", tc.ev.Kind)
			}
			if diff := compareUIMsg(tc.want, msg); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestTranslateEventIgnoresNonRenderedKinds(t *testing.T) {
	t.Parallel()

	cases := []eventspkg.Event{
		mustRuntimeEventUITest(
			t,
			eventspkg.EventKindTaskFileUpdated,
			kinds.TaskFileUpdatedPayload{TaskName: "task_01.md"},
		),
		mustRuntimeEventUITest(
			t,
			eventspkg.EventKindReviewStatusFinalized,
			kinds.ReviewStatusFinalizedPayload{IssueIDs: []string{"issue-1"}},
		),
		mustRuntimeEventUITest(
			t,
			eventspkg.EventKindProviderCallStarted,
			kinds.ProviderCallStartedPayload{CallID: "call-1", Provider: "github"},
		),
	}

	for _, ev := range cases {
		if msg, ok := translateEvent(ev); ok {
			t.Fatalf("expected %s to be ignored, got %T", ev.Kind, msg)
		}
	}
}

func TestTranslateEventShutdownStatus(t *testing.T) {
	t.Parallel()

	msg, ok := translateEvent(mustRuntimeEventUITest(
		t,
		eventspkg.EventKindShutdownTerminated,
		kinds.ShutdownTerminatedPayload{
			Source:      string(shutdownSourceTimer),
			RequestedAt: time.Unix(10, 0).UTC(),
			DeadlineAt:  time.Unix(20, 0).UTC(),
			Forced:      true,
		},
	))
	if !ok {
		t.Fatal("expected shutdown.terminated to translate")
	}

	status, ok := msg.(shutdownStatusMsg)
	if !ok {
		t.Fatalf("expected shutdownStatusMsg, got %T", msg)
	}
	if status.State.Phase != shutdownPhaseForcing {
		t.Fatalf("expected forcing phase, got %#v", status.State)
	}
	if status.State.Source != shutdownSourceTimer {
		t.Fatalf("expected timer shutdown source, got %#v", status.State)
	}
}

func TestUIEventTranslatorAddsFailureTerminalMessage(t *testing.T) {
	t.Parallel()

	translator := newUIEventTranslator()
	msgs := translator.translateMessages(mustRuntimeEventUITest(
		t,
		eventspkg.EventKindJobFailed,
		kinds.JobFailedPayload{
			Index:    0,
			CodeFile: "task_01.md",
			ExitCode: 23,
			Error:    "boom",
		},
	))
	if len(msgs) != 2 {
		t.Fatalf("expected failure event to emit failure and terminal messages, got %#v", msgs)
	}
	if _, ok := msgs[0].(jobFailureMsg); !ok {
		t.Fatalf("expected first message to be jobFailureMsg, got %T", msgs[0])
	}
	finished, ok := msgs[1].(jobFinishedMsg)
	if !ok {
		t.Fatalf("expected second message to be jobFinishedMsg, got %T", msgs[1])
	}
	if finished.Success || finished.ExitCode != 23 {
		t.Fatalf("unexpected terminal failure message: %#v", finished)
	}
}

func TestUIEventAdapterStopClosesSinkAndUnsubscribes(t *testing.T) {
	bus := eventspkg.New[eventspkg.Event](8)
	defer func() {
		if err := bus.Close(context.Background()); err != nil {
			t.Fatalf("close bus: %v", err)
		}
	}()

	baseGoroutines := runtime.NumGoroutine()
	sink := make(chan uiMsg, 4)
	stop, done := startUIEventAdapter(bus, sink)

	waitForCondition(t, time.Second, func() bool {
		return bus.SubscriberCount() == 1
	})

	stop()
	stop()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for adapter shutdown")
	}

	if _, ok := <-sink; ok {
		t.Fatal("expected adapter sink to be closed")
	}

	waitForCondition(t, time.Second, func() bool {
		return bus.SubscriberCount() == 0
	})
	waitForCondition(t, time.Second, func() bool {
		return runtime.NumGoroutine() <= baseGoroutines+2
	})
}

func TestUIEventAdapterDropsMessagesWhenSinkIsFull(t *testing.T) {
	t.Parallel()

	bus := eventspkg.New[eventspkg.Event](8)
	defer func() {
		if err := bus.Close(context.Background()); err != nil {
			t.Fatalf("close bus: %v", err)
		}
	}()

	sink := make(chan uiMsg, 1)
	stop, done := startUIEventAdapter(bus, sink)
	defer func() {
		stop()
		<-done
	}()

	bus.Publish(context.Background(), mustRuntimeEventUITest(
		t,
		eventspkg.EventKindJobStarted,
		kinds.JobStartedPayload{Index: 0, Attempt: 1, MaxAttempts: 1},
	))
	waitForCondition(t, time.Second, func() bool {
		return len(sink) == 1
	})

	bus.Publish(context.Background(), mustRuntimeEventUITest(
		t,
		eventspkg.EventKindUsageUpdated,
		kinds.UsageUpdatedPayload{Index: 0, Usage: kinds.Usage{TotalTokens: 9}},
	))
	bus.Publish(context.Background(), mustRuntimeEventUITest(
		t,
		eventspkg.EventKindJobCompleted,
		kinds.JobCompletedPayload{Index: 0, ExitCode: 0},
	))

	stop()
	<-done

	got := drainClosedUIMessages(sink)
	if len(got) != 1 {
		t.Fatalf("expected exactly one retained UI message, got %#v", got)
	}
	if _, ok := got[0].(jobStartedMsg); !ok {
		t.Fatalf("expected retained message to be the first jobStartedMsg, got %T", got[0])
	}
}

func TestInternalContentBlockConvertsAllPublicBlockTypes(t *testing.T) {
	t.Parallel()

	oldText := "old"
	cases := []kinds.ContentBlock{
		mustPublicContentBlockUITest(t, kinds.TextBlock{Type: kinds.BlockText, Text: "hello"}),
		mustPublicContentBlockUITest(t, kinds.ToolUseBlock{
			Type:     kinds.BlockToolUse,
			ID:       "tool-1",
			Name:     "Read",
			Title:    "Read",
			ToolName: "read_file",
			Input:    []byte(`{"path":"README.md"}`),
			RawInput: []byte(`{"raw":true}`),
		}),
		mustPublicContentBlockUITest(t, kinds.ToolResultBlock{
			Type:      kinds.BlockToolResult,
			ToolUseID: "tool-1",
			Content:   "done",
			IsError:   true,
		}),
		mustPublicContentBlockUITest(t, kinds.DiffBlock{
			Type:     kinds.BlockDiff,
			FilePath: "README.md",
			Diff:     "@@ -1 +1 @@",
			OldText:  &oldText,
			NewText:  "new",
		}),
		mustPublicContentBlockUITest(t, kinds.TerminalOutputBlock{
			Type:       kinds.BlockTerminalOutput,
			Command:    "go test ./...",
			Output:     "ok",
			ExitCode:   0,
			TerminalID: "term-1",
		}),
		mustPublicContentBlockUITest(t, kinds.ImageBlock{
			Type:     kinds.BlockImage,
			Data:     "AAAA",
			MimeType: "image/png",
		}),
	}

	for _, block := range cases {
		block := block
		t.Run(string(block.Type), func(t *testing.T) {
			t.Parallel()

			converted, err := internalContentBlock(block)
			if err != nil {
				t.Fatalf("internalContentBlock(%s): %v", block.Type, err)
			}
			if converted.Type == "" {
				t.Fatalf("expected converted block type for %s", block.Type)
			}
		})
	}
}

func TestUIEventAdapterPipelineUpdatesModelAndView(t *testing.T) {
	t.Parallel()

	runID, runJournal, bus, cleanup := openUIAdapterRuntime(t)
	defer cleanup()

	sink := make(chan uiMsg, 16)
	stop, done := startUIEventAdapter(bus, sink)
	defer func() {
		stop()
		<-done
	}()

	mdl := newUIModel(1)
	mdl.cfg = &config{}
	applyUIMsgs(mdl, jobQueuedMsg{
		Index:     0,
		CodeFile:  "task_01.md",
		CodeFiles: []string{"task_01.md"},
		Issues:    1,
		SafeName:  "task_01",
		OutLog:    "task_01.out.log",
		ErrLog:    "task_01.err.log",
	})

	runtimeJob := job{
		codeFiles: []string{"task_01.md"},
		outLog:    "task_01.out.log",
		errLog:    "task_01.err.log",
	}
	execCtx := &jobExecutionContext{
		cfg: &config{
			outputFormat: model.OutputFormatJSON,
			runArtifacts: model.RunArtifacts{RunID: runID},
		},
		journal: runJournal,
	}
	lifecycle := newJobLifecycle(0, &runtimeJob, execCtx)

	var jobUsage model.Usage
	var aggregate model.Usage
	var aggregateMu sync.Mutex
	handler := newSessionUpdateHandler(
		0,
		model.IDECodex,
		"sess-ui",
		nil,
		runID,
		io.Discard,
		io.Discard,
		runJournal,
		&jobUsage,
		&aggregate,
		&aggregateMu,
		nil,
	)

	textBlock := mustContentBlockLoggingTest(t, model.TextBlock{Text: "hello from ACP"})
	lifecycle.startAttempt(1, 1, time.Second)
	if err := handler.HandleUpdate(model.SessionUpdate{
		Kind:   model.UpdateKindAgentMessageChunk,
		Blocks: []model.ContentBlock{textBlock},
		Usage:  model.Usage{InputTokens: 4, OutputTokens: 3, TotalTokens: 7},
		Status: model.StatusRunning,
	}); err != nil {
		t.Fatalf("handle update: %v", err)
	}
	lifecycle.markSuccess()

	applyUIMsgs(mdl, collectUIMessages(t, sink, 4)...)

	if got := mdl.jobs[0].state; got != jobSuccess {
		t.Fatalf("expected job state success, got %v", got)
	}
	if got := mdl.completed; got != 1 {
		t.Fatalf("expected one completed job, got %d", got)
	}
	if got := mdl.jobs[0].tokenUsage; got == nil || got.TotalTokens != 7 {
		t.Fatalf("unexpected per-job usage: %#v", got)
	}
	if got := len(mdl.jobs[0].snapshot.Entries); got != 1 {
		t.Fatalf("expected one transcript entry, got %#v", mdl.jobs[0].snapshot.Entries)
	}
	view := mdl.View()
	if !strings.Contains(view.Content, "hello from ACP") {
		t.Fatalf("expected rendered view to include transcript text, got %q", view.Content)
	}
	if !strings.Contains(view.Content, "SUCCESS") {
		t.Fatalf("expected rendered view to include success state, got %q", view.Content)
	}
}

func mustRuntimeEventUITest(t *testing.T, kind eventspkg.EventKind, payload any) eventspkg.Event {
	t.Helper()

	ev, err := newRuntimeEvent("ui-adapter-test", kind, payload)
	if err != nil {
		t.Fatalf("newRuntimeEvent(%s): %v", kind, err)
	}
	return ev
}

func mustPublicContentBlockUITest(t *testing.T, block any) kinds.ContentBlock {
	t.Helper()

	value, err := kinds.NewContentBlock(block)
	if err != nil {
		t.Fatalf("kinds.NewContentBlock(%T): %v", block, err)
	}
	return value
}

func compareUIMsg(want any, got any) string {
	switch wantValue := want.(type) {
	case usageUpdateMsg:
		gotValue, ok := got.(usageUpdateMsg)
		if !ok {
			return "expected usageUpdateMsg"
		}
		if gotValue.Index != wantValue.Index || gotValue.Usage != wantValue.Usage {
			return "unexpected usageUpdateMsg payload"
		}
	case jobRetryMsg:
		gotValue, ok := got.(jobRetryMsg)
		if !ok {
			return "expected jobRetryMsg"
		}
		if gotValue != wantValue {
			return "unexpected jobRetryMsg payload"
		}
	case jobFailureMsg:
		gotValue, ok := got.(jobFailureMsg)
		if !ok {
			return "expected jobFailureMsg"
		}
		if gotValue.Failure.codeFile != wantValue.Failure.codeFile ||
			gotValue.Failure.exitCode != wantValue.Failure.exitCode ||
			gotValue.Failure.outLog != wantValue.Failure.outLog ||
			gotValue.Failure.errLog != wantValue.Failure.errLog ||
			errorString(gotValue.Failure.err) != errorString(wantValue.Failure.err) {
			return "unexpected jobFailureMsg payload"
		}
	default:
		return "unsupported comparison type"
	}
	return ""
}

func applyUIMsgs(mdl *uiModel, msgs ...uiMsg) {
	for _, msg := range msgs {
		mdl.Update(msg)
	}
}

func collectUIMessages(t *testing.T, ch <-chan uiMsg, want int) []uiMsg {
	t.Helper()

	got := make([]uiMsg, 0, want)
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()

	for len(got) < want {
		select {
		case msg := <-ch:
			got = append(got, msg)
		case <-deadline.C:
			t.Fatalf("timed out waiting for %d UI messages, got %d", want, len(got))
		}
	}

	return got
}

func drainClosedUIMessages(ch <-chan uiMsg) []uiMsg {
	var got []uiMsg
	for msg := range ch {
		got = append(got, msg)
	}
	return got
}

func waitForCondition(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	for {
		if fn() {
			return
		}
		select {
		case <-ticker.C:
		case <-deadline.C:
			t.Fatal("condition not met before timeout")
		}
	}
}

func openUIAdapterRuntime(t *testing.T) (string, *journal.Journal, *eventspkg.Bus[eventspkg.Event], func()) {
	t.Helper()

	workspaceRoot := t.TempDir()
	runArtifacts := model.NewRunArtifacts(workspaceRoot, "ui-adapter-run")
	if err := os.MkdirAll(filepath.Dir(runArtifacts.EventsPath), 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	bus := eventspkg.New[eventspkg.Event](16)
	runJournal, err := journal.Open(runArtifacts.EventsPath, bus, 16)
	if err != nil {
		t.Fatalf("open journal: %v", err)
	}

	cleanup := func() {
		t.Helper()

		closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := runJournal.Close(closeCtx); err != nil {
			t.Fatalf("close journal: %v", err)
		}
		if err := bus.Close(context.Background()); err != nil {
			t.Fatalf("close bus: %v", err)
		}
	}

	return runArtifacts.RunID, runJournal, bus, cleanup
}
