package ui

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/contentconv"
	"github.com/compozy/compozy/internal/core/model"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func translateEventForTest(t *testing.T, ev eventspkg.Event) (uiMsg, bool) {
	t.Helper()
	return newUIEventTranslator().translateEvent(ev)
}

func TestTranslateEventJobStarted(t *testing.T) {
	t.Parallel()

	msg, ok := translateEventForTest(t, mustRuntimeEventUITest(
		t,
		eventspkg.EventKindJobStarted,
		kinds.JobStartedPayload{
			JobAttemptInfo: kinds.JobAttemptInfo{Index: 2, Attempt: 1, MaxAttempts: 3},
		},
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

func TestTranslateEventJobQueued(t *testing.T) {
	t.Parallel()

	msg, ok := translateEventForTest(t, mustRuntimeEventUITest(
		t,
		eventspkg.EventKindJobQueued,
		kinds.JobQueuedPayload{
			Index:     1,
			CodeFile:  "task_02.md",
			CodeFiles: []string{"task_02.md"},
			TaskTitle: "Remote Queue",
		},
	))
	if !ok {
		t.Fatal("expected job.queued to translate")
	}

	queued, ok := msg.(jobQueuedMsg)
	if !ok {
		t.Fatalf("expected jobQueuedMsg, got %T", msg)
	}
	if queued.Index != 1 || queued.CodeFile != "task_02.md" || queued.TaskTitle != "Remote Queue" {
		t.Fatalf("unexpected queued payload: %#v", queued)
	}
}

func TestTranslateEventSessionUpdateProducesSnapshot(t *testing.T) {
	t.Parallel()

	textBlock := mustContentBlockUITest(t, model.TextBlock{Text: "hello from ACP"})
	update, err := contentconv.PublicSessionUpdate(model.SessionUpdate{
		Kind:   model.UpdateKindAgentMessageChunk,
		Blocks: []model.ContentBlock{textBlock},
		Status: model.StatusRunning,
	})
	if err != nil {
		t.Fatalf("contentconv.PublicSessionUpdate: %v", err)
	}

	msg, ok := translateEventForTest(t, mustRuntimeEventUITest(
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
				kinds.JobRetryScheduledPayload{
					JobAttemptInfo: kinds.JobAttemptInfo{Index: 1, Attempt: 2, MaxAttempts: 4},
					Reason:         "retry me",
				},
			),
			want: jobRetryMsg{Index: 1, Attempt: 2, MaxAttempts: 4, Reason: "retry me"},
		},
		{
			name: "job failed",
			ev: mustRuntimeEventUITest(
				t,
				eventspkg.EventKindJobFailed,
				kinds.JobFailedPayload{
					JobAttemptInfo: kinds.JobAttemptInfo{Index: 1, Attempt: 2, MaxAttempts: 4},
					CodeFile:       "task_01.md",
					ExitCode:       23,
					OutLog:         "task_01.out.log",
					ErrLog:         "task_01.err.log",
					Error:          "boom",
				},
			),
			want: jobFailureMsg{
				Failure: failInfo{
					CodeFile: "task_01.md",
					ExitCode: 23,
					OutLog:   "task_01.out.log",
					ErrLog:   "task_01.err.log",
					Err:      eventError("boom"),
				},
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			msg, ok := translateEventForTest(t, tc.ev)
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
		if msg, ok := translateEventForTest(t, ev); ok {
			t.Fatalf("expected %s to be ignored, got %T", ev.Kind, msg)
		}
	}
}

func TestTranslateEventShutdownStatus(t *testing.T) {
	t.Parallel()

	msg, ok := translateEventForTest(t, mustRuntimeEventUITest(
		t,
		eventspkg.EventKindShutdownTerminated,
		kinds.ShutdownTerminatedPayload{
			ShutdownBase: kinds.ShutdownBase{
				Source:      string(shutdownSourceTimer),
				RequestedAt: time.Unix(10, 0).UTC(),
				DeadlineAt:  time.Unix(20, 0).UTC(),
			},
			Forced: true,
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
			JobAttemptInfo: kinds.JobAttemptInfo{Index: 0},
			CodeFile:       "task_01.md",
			ExitCode:       23,
			Error:          "boom",
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

func TestUIEventAdapterStopUnsubscribes(t *testing.T) {
	bus := eventspkg.New[eventspkg.Event](8)
	defer func() {
		if err := bus.Close(context.Background()); err != nil {
			t.Fatalf("close bus: %v", err)
		}
	}()

	delivered := make(chan eventspkg.Event, 1)
	stop, done := startUIEventAdapter(context.Background(), bus, func(ev eventspkg.Event) {
		delivered <- ev
	})

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

	waitForCondition(t, time.Second, func() bool {
		return bus.SubscriberCount() == 0
	})

	select {
	case ev := <-delivered:
		t.Fatalf("expected no delivery after immediate stop, got %s", ev.Kind)
	default:
	}
}

func TestUIEventAdapterForwardsRawEvents(t *testing.T) {
	t.Parallel()

	bus := eventspkg.New[eventspkg.Event](8)
	defer func() {
		if err := bus.Close(context.Background()); err != nil {
			t.Fatalf("close bus: %v", err)
		}
	}()

	delivered := make(chan eventspkg.Event, 1)
	stop, done := startUIEventAdapter(context.Background(), bus, func(ev eventspkg.Event) {
		delivered <- ev
	})
	defer func() {
		stop()
		<-done
	}()

	want := mustRuntimeEventUITest(
		t,
		eventspkg.EventKindJobStarted,
		kinds.JobStartedPayload{
			JobAttemptInfo: kinds.JobAttemptInfo{Index: 0, Attempt: 1, MaxAttempts: 1},
		},
	)
	bus.Publish(context.Background(), want)

	select {
	case got := <-delivered:
		if got.Kind != want.Kind {
			t.Fatalf("expected delivered event kind %s, got %s", want.Kind, got.Kind)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for forwarded raw event")
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

			converted, err := contentconv.InternalContentBlock(block)
			if err != nil {
				t.Fatalf("contentconv.InternalContentBlock(%s): %v", block.Type, err)
			}
			if converted.Type == "" {
				t.Fatalf("expected converted block type for %s", block.Type)
			}
		})
	}
}

func TestPrepareDispatchBatchCoalescesBurstAndPreservesLifecycleOrder(t *testing.T) {
	t.Parallel()

	updateOne, err := contentconv.PublicSessionUpdate(model.SessionUpdate{
		Kind:   model.UpdateKindAgentMessageChunk,
		Blocks: []model.ContentBlock{mustContentBlockUITest(t, model.TextBlock{Text: "hello"})},
		Status: model.StatusRunning,
	})
	if err != nil {
		t.Fatalf("contentconv.PublicSessionUpdate: %v", err)
	}
	updateTwo, err := contentconv.PublicSessionUpdate(model.SessionUpdate{
		Kind:   model.UpdateKindAgentMessageChunk,
		Blocks: []model.ContentBlock{mustContentBlockUITest(t, model.TextBlock{Text: " world"})},
		Status: model.StatusRunning,
	})
	if err != nil {
		t.Fatalf("contentconv.PublicSessionUpdate: %v", err)
	}

	ctrl := &uiController{translator: newUIEventTranslator()}
	batch := ctrl.prepareDispatchBatch([]any{
		mustRuntimeEventUITest(
			t,
			eventspkg.EventKindJobStarted,
			kinds.JobStartedPayload{JobAttemptInfo: kinds.JobAttemptInfo{Index: 0, Attempt: 1, MaxAttempts: 1}},
		),
		mustRuntimeEventUITest(
			t,
			eventspkg.EventKindSessionUpdate,
			kinds.SessionUpdatePayload{Index: 0, Update: updateOne},
		),
		mustRuntimeEventUITest(
			t,
			eventspkg.EventKindSessionUpdate,
			kinds.SessionUpdatePayload{Index: 0, Update: updateTwo},
		),
		mustRuntimeEventUITest(
			t,
			eventspkg.EventKindUsageUpdated,
			kinds.UsageUpdatedPayload{Index: 0, Usage: kinds.Usage{InputTokens: 4, OutputTokens: 3, TotalTokens: 7}},
		),
		mustRuntimeEventUITest(
			t,
			eventspkg.EventKindUsageUpdated,
			kinds.UsageUpdatedPayload{Index: 0, Usage: kinds.Usage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2}},
		),
		mustRuntimeEventUITest(
			t,
			eventspkg.EventKindJobCompleted,
			kinds.JobCompletedPayload{
				JobAttemptInfo: kinds.JobAttemptInfo{Index: 0, Attempt: 1, MaxAttempts: 1},
				ExitCode:       0,
			},
		),
	})

	if got := len(batch.msgs); got != 4 {
		t.Fatalf("expected started + coalesced update + coalesced usage + finished, got %#v", batch.msgs)
	}
	if _, ok := batch.msgs[0].(jobStartedMsg); !ok {
		t.Fatalf("expected first message to preserve jobStarted lifecycle, got %T", batch.msgs[0])
	}
	updateMsg, ok := batch.msgs[1].(jobUpdateMsg)
	if !ok {
		t.Fatalf("expected second message to be coalesced jobUpdateMsg, got %T", batch.msgs[1])
	}
	if got := len(updateMsg.Snapshot.Entries); got != 1 {
		t.Fatalf("expected one merged transcript entry, got %#v", updateMsg.Snapshot.Entries)
	}
	text, err := updateMsg.Snapshot.Entries[0].Blocks[0].AsText()
	if err != nil {
		t.Fatalf("AsText: %v", err)
	}
	if got := text.Text; got != "hello world" {
		t.Fatalf("expected coalesced transcript text, got %q", got)
	}
	usageMsg, ok := batch.msgs[2].(usageUpdateMsg)
	if !ok {
		t.Fatalf("expected third message to be coalesced usageUpdateMsg, got %T", batch.msgs[2])
	}
	if usageMsg.Usage.TotalTokens != 9 || usageMsg.Usage.InputTokens != 5 || usageMsg.Usage.OutputTokens != 4 {
		t.Fatalf("unexpected aggregated usage payload: %#v", usageMsg)
	}
	if _, ok := batch.msgs[3].(jobFinishedMsg); !ok {
		t.Fatalf("expected terminal lifecycle to stay last, got %T", batch.msgs[3])
	}
}

func TestUIEventAdapterPipelineUpdatesModelAndView(t *testing.T) {
	t.Parallel()

	bus := eventspkg.New[eventspkg.Event](16)
	defer func() {
		if err := bus.Close(context.Background()); err != nil {
			t.Fatalf("close bus: %v", err)
		}
	}()

	eventsCh := make(chan eventspkg.Event, 8)
	stop, done := startUIEventAdapter(context.Background(), bus, func(ev eventspkg.Event) {
		eventsCh <- ev
	})
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

	textBlock := mustContentBlockUITest(t, model.TextBlock{Text: "hello from ACP"})
	update, err := contentconv.PublicSessionUpdate(model.SessionUpdate{
		Kind:   model.UpdateKindAgentMessageChunk,
		Blocks: []model.ContentBlock{textBlock},
		Usage:  model.Usage{InputTokens: 4, OutputTokens: 3, TotalTokens: 7},
		Status: model.StatusRunning,
	})
	if err != nil {
		t.Fatalf("contentconv.PublicSessionUpdate: %v", err)
	}

	bus.Publish(context.Background(), mustRuntimeEventUITest(
		t,
		eventspkg.EventKindJobStarted,
		kinds.JobStartedPayload{JobAttemptInfo: kinds.JobAttemptInfo{Index: 0, Attempt: 1, MaxAttempts: 1}},
	))
	bus.Publish(context.Background(), mustRuntimeEventUITest(
		t,
		eventspkg.EventKindSessionUpdate,
		kinds.SessionUpdatePayload{Index: 0, Update: update},
	))
	bus.Publish(context.Background(), mustRuntimeEventUITest(
		t,
		eventspkg.EventKindUsageUpdated,
		kinds.UsageUpdatedPayload{
			Index: 0,
			Usage: kinds.Usage{InputTokens: 4, OutputTokens: 3, TotalTokens: 7},
		},
	))
	bus.Publish(context.Background(), mustRuntimeEventUITest(
		t,
		eventspkg.EventKindJobCompleted,
		kinds.JobCompletedPayload{
			JobAttemptInfo: kinds.JobAttemptInfo{Index: 0, Attempt: 1, MaxAttempts: 1},
			ExitCode:       0,
		},
	))

	ctrl := &uiController{translator: newUIEventTranslator()}
	rawEvents := collectEvents(t, eventsCh, 4)
	inputs := make([]any, 0, len(rawEvents))
	for _, ev := range rawEvents {
		inputs = append(inputs, ev)
	}
	applyUIMsgs(mdl, ctrl.prepareDispatchBatch(inputs).msgs...)

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

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal(%T): %v", payload, err)
	}
	return eventspkg.Event{
		SchemaVersion: eventspkg.SchemaVersion,
		RunID:         "ui-adapter-test",
		Timestamp:     time.Now().UTC(),
		Kind:          kind,
		Payload:       raw,
	}
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
		if gotValue.Failure.CodeFile != wantValue.Failure.CodeFile ||
			gotValue.Failure.ExitCode != wantValue.Failure.ExitCode ||
			gotValue.Failure.OutLog != wantValue.Failure.OutLog ||
			gotValue.Failure.ErrLog != wantValue.Failure.ErrLog ||
			errorText(gotValue.Failure.Err) != errorText(wantValue.Failure.Err) {
			return "unexpected jobFailureMsg payload"
		}
	default:
		return "unsupported comparison type"
	}
	return ""
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func applyUIMsgs(mdl *uiModel, msgs ...uiMsg) {
	for _, msg := range msgs {
		mdl.Update(msg)
	}
}

func collectEvents(t *testing.T, ch <-chan eventspkg.Event, want int) []eventspkg.Event {
	t.Helper()

	got := make([]eventspkg.Event, 0, want)
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()

	for len(got) < want {
		select {
		case msg, ok := <-ch:
			if !ok {
				t.Fatalf("UI message channel closed after %d/%d messages", len(got), want)
			}
			got = append(got, msg)
		case <-deadline.C:
			t.Fatalf("timed out waiting for %d UI messages, got %d", want, len(got))
		}
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
