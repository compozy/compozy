package acpshared

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/internal/runshared"
	"github.com/compozy/compozy/internal/core/run/journal"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func TestLineBufferUnlimitedRetentionKeepsFullHistory(t *testing.T) {
	t.Parallel()

	lines := runshared.NewLineBuffer(0)
	for _, line := range []string{"one", "two", "three", "four"} {
		lines.AppendLine(line)
	}

	got := lines.Snapshot()
	want := []string{"one", "two", "three", "four"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("expected full history %v, got %v", want, got)
	}
}

func TestLineBufferLimitedRetentionKeepsNewestEntries(t *testing.T) {
	t.Parallel()

	buffer := runshared.NewLineBuffer(2)
	for _, line := range []string{"one", "two", "three", "four"} {
		buffer.AppendLine(line)
	}

	got := buffer.Snapshot()
	want := []string{"three", "four"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("expected capped history %v, got %v", want, got)
	}
}

func TestSessionUpdateHandlerRoutesTextBlocksToLogAndSnapshot(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	var err bytes.Buffer
	runID, runJournal, eventsCh, cleanup := openRuntimeEventCapture(t)
	defer cleanup()

	var jobUsage model.Usage
	var aggregate model.Usage
	handler := newSessionUpdateHandler(SessionUpdateHandlerConfig{
		Context:        context.Background(),
		Index:          3,
		AgentID:        model.IDECodex,
		SessionID:      "sess-123",
		RunID:          runID,
		OutWriter:      &out,
		ErrWriter:      &err,
		RunJournal:     runJournal,
		JobUsage:       &jobUsage,
		AggregateUsage: &aggregate,
		AggregateMu:    &sync.Mutex{},
	})

	textBlock := mustContentBlockLoggingTest(t, model.TextBlock{Text: "hello from ACP"})
	if handleErr := handler.HandleUpdate(model.SessionUpdate{
		Kind:   model.UpdateKindAgentMessageChunk,
		Blocks: []model.ContentBlock{textBlock},
		Status: model.StatusRunning,
	}); handleErr != nil {
		t.Fatalf("handle update: %v", handleErr)
	}

	if got := out.String(); !strings.Contains(got, "hello from ACP") {
		t.Fatalf("expected stdout log to contain text block, got %q", got)
	}
	if got := err.String(); got != "" {
		t.Fatalf("expected stderr log to remain empty, got %q", got)
	}

	events := collectRuntimeEvents(t, eventsCh, 1)
	if got := events[0].Kind; got != eventspkg.EventKindSessionUpdate {
		t.Fatalf("expected session.update event, got %s", got)
	}

	var payload kinds.SessionUpdatePayload
	decodeRuntimeEventPayload(t, events[0], &payload)
	if payload.Index != 3 {
		t.Fatalf("expected session update index 3, got %d", payload.Index)
	}
	if len(payload.Update.Blocks) != 1 {
		t.Fatalf("expected one serialized block, got %#v", payload.Update.Blocks)
	}

	snapshot := handler.Snapshot()
	if len(snapshot.Entries) != 1 || snapshot.Entries[0].Kind != transcriptEntryAssistantMessage {
		t.Fatalf("unexpected snapshot entries: %#v", snapshot.Entries)
	}
}

func TestSessionUpdateHandlerKeepsActivityActiveWhileSubmittingUpdate(t *testing.T) {
	t.Run("Should keep activity active while submitting update", func(t *testing.T) {
		t.Parallel()

		submitter := &blockingRuntimeEventSubmitter{
			entered: make(chan struct{}),
			release: make(chan struct{}),
		}
		activity := newActivityMonitor()
		handler := newSessionUpdateHandler(SessionUpdateHandlerConfig{
			Context:    context.Background(),
			Index:      1,
			AgentID:    model.IDECodex,
			SessionID:  "sess-active-update",
			RunID:      "run-active-update",
			RunJournal: submitter,
			Activity:   activity,
		})

		textBlock := mustContentBlockLoggingTest(t, model.TextBlock{Text: "active"})
		errCh := make(chan error, 1)
		var releaseOnce sync.Once
		releaseSubmitter := func() {
			releaseOnce.Do(func() {
				close(submitter.release)
			})
		}
		drained := false
		defer func() {
			releaseSubmitter()
			if drained {
				return
			}
			select {
			case <-errCh:
			case <-time.After(time.Second):
			}
		}()
		go func() {
			errCh <- handler.HandleUpdate(model.SessionUpdate{
				Kind:   model.UpdateKindAgentMessageChunk,
				Blocks: []model.ContentBlock{textBlock},
				Status: model.StatusRunning,
			})
		}()

		select {
		case <-submitter.entered:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for session update submission")
		}
		if got := activity.TimeSinceLastActivity(); got != 0 {
			t.Fatalf("expected in-flight update handling to report active work, got %v", got)
		}

		releaseSubmitter()
		select {
		case err := <-errCh:
			drained = true
			if err != nil {
				t.Fatalf("handle update: %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for session update handling")
		}
	})
}

type blockingRuntimeEventSubmitter struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (s *blockingRuntimeEventSubmitter) Submit(ctx context.Context, _ eventspkg.Event) error {
	s.once.Do(func() {
		close(s.entered)
	})
	select {
	case <-s.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestSessionUpdateHandlerMergesTranscriptAndCarriesSessionState(t *testing.T) {
	t.Parallel()

	runID, runJournal, eventsCh, cleanup := openRuntimeEventCapture(t)
	defer cleanup()

	handler := newSessionUpdateHandler(SessionUpdateHandlerConfig{
		Context:    context.Background(),
		Index:      1,
		AgentID:    model.IDECodex,
		SessionID:  "sess-merge",
		RunID:      runID,
		OutWriter:  io.Discard,
		ErrWriter:  io.Discard,
		RunJournal: runJournal,
	})

	updates := []model.SessionUpdate{
		{
			Kind:   model.UpdateKindAgentMessageChunk,
			Blocks: []model.ContentBlock{mustContentBlockLoggingTest(t, model.TextBlock{Text: "Ledger Snapshot: "})},
			Status: model.StatusRunning,
		},
		{
			Kind:   model.UpdateKindAgentMessageChunk,
			Blocks: []model.ContentBlock{mustContentBlockLoggingTest(t, model.TextBlock{Text: "Goal is fix the TUI"})},
			Status: model.StatusRunning,
		},
		{
			Kind: model.UpdateKindPlanUpdated,
			PlanEntries: []model.SessionPlanEntry{{
				Content:  "Ship redesign",
				Priority: "high",
				Status:   "in_progress",
			}},
			Status: model.StatusRunning,
		},
		{
			Kind:          model.UpdateKindCurrentModeUpdated,
			CurrentModeID: "review",
			Status:        model.StatusRunning,
		},
	}

	for _, update := range updates {
		if err := handler.HandleUpdate(update); err != nil {
			t.Fatalf("handle update: %v", err)
		}
	}

	events := collectRuntimeEvents(t, eventsCh, len(updates))
	for _, event := range events {
		if got := event.Kind; got != eventspkg.EventKindSessionUpdate {
			t.Fatalf("expected only session.update events, got %s", got)
		}
	}

	snapshot := handler.Snapshot()
	if len(snapshot.Entries) != 1 {
		t.Fatalf("expected merged assistant entry, got %#v", snapshot.Entries)
	}
	textBlock, err := snapshot.Entries[0].Blocks[0].AsText()
	if err != nil {
		t.Fatalf("decode merged text block: %v", err)
	}
	if want := "Ledger Snapshot: Goal is fix the TUI"; textBlock.Text != want {
		t.Fatalf("unexpected merged transcript text: got %q want %q", textBlock.Text, want)
	}
	if snapshot.Plan.RunningCount != 1 {
		t.Fatalf("expected plan state in snapshot, got %#v", snapshot.Plan)
	}
	if snapshot.Session.CurrentModeID != "review" {
		t.Fatalf("expected current mode in snapshot, got %q", snapshot.Session.CurrentModeID)
	}
}

func TestSessionUpdateHandlerRoutesMixedBlocksAndUsage(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	var err bytes.Buffer
	runID, runJournal, eventsCh, cleanup := openRuntimeEventCapture(t)
	defer cleanup()

	var jobUsage model.Usage
	var aggregate model.Usage
	var aggregateMu sync.Mutex
	handler := newSessionUpdateHandler(SessionUpdateHandlerConfig{
		Context:        context.Background(),
		Index:          0,
		AgentID:        model.IDECodex,
		SessionID:      "sess-mixed",
		RunID:          runID,
		OutWriter:      &out,
		ErrWriter:      &err,
		RunJournal:     runJournal,
		JobUsage:       &jobUsage,
		AggregateUsage: &aggregate,
		AggregateMu:    &aggregateMu,
	})

	toolUseBlock := mustContentBlockLoggingTest(t, model.ToolUseBlock{
		ID:       "tool-1",
		Name:     "Read",
		Title:    "Read",
		ToolName: "read_file",
		Input:    []byte(`{"file_path":"README.md"}`),
		RawInput: []byte(`{"path":"README.md"}`),
	})
	diffBlock := mustContentBlockLoggingTest(t, model.DiffBlock{
		FilePath: "README.md",
		Diff:     "@@ -1 +1 @@\n-old\n+new",
		NewText:  "new",
	})
	toolErrBlock := mustContentBlockLoggingTest(t, model.ToolResultBlock{
		ToolUseID: "tool-1",
		Content:   "permission denied",
		IsError:   true,
	})

	update := model.SessionUpdate{
		Kind:   model.UpdateKindToolCallUpdated,
		Blocks: []model.ContentBlock{toolUseBlock, diffBlock, toolErrBlock},
		Usage: model.Usage{
			InputTokens:  10,
			OutputTokens: 5,
			TotalTokens:  15,
			CacheReads:   2,
		},
		Status: model.StatusRunning,
	}
	if handleErr := handler.HandleUpdate(update); handleErr != nil {
		t.Fatalf("handle update: %v", handleErr)
	}

	if got := out.String(); !strings.Contains(got, "[TOOL] Read README.md") ||
		strings.Contains(got, "[TOOL] read_file") ||
		!strings.Contains(got, `"file_path":"README.md"`) ||
		strings.Contains(got, `"path":"README.md"`) {
		t.Fatalf("expected mixed stdout rendering, got %q", got)
	}
	if got := err.String(); !strings.Contains(got, "permission denied") {
		t.Fatalf("expected tool error content in stderr log, got %q", got)
	}
	if got := aggregate; got.TotalTokens != 15 || got.CacheReads != 2 {
		t.Fatalf("unexpected aggregate usage: %#v", got)
	}
	if got := jobUsage; got.TotalTokens != 15 || got.CacheReads != 2 {
		t.Fatalf("unexpected job usage: %#v", got)
	}

	events := collectRuntimeEvents(t, eventsCh, 2)
	if got := events[0].Kind; got != eventspkg.EventKindSessionUpdate {
		t.Fatalf("expected first event to be session.update, got %s", got)
	}
	if got := events[1].Kind; got != eventspkg.EventKindUsageUpdated {
		t.Fatalf("expected second event to be usage.updated, got %s", got)
	}

	var usagePayload kinds.UsageUpdatedPayload
	decodeRuntimeEventPayload(t, events[1], &usagePayload)
	if usagePayload.Index != 0 || usagePayload.Usage.TotalTokens != 15 {
		t.Fatalf("unexpected usage payload: %#v", usagePayload)
	}

	snapshot := handler.Snapshot()
	if len(snapshot.Entries) < 1 {
		t.Fatalf("expected at least one snapshot entry, got %#v", snapshot.Entries)
	}
}

func TestSessionUpdateHandlerDoesNotBlockWhenSessionStateIsTracked(t *testing.T) {
	t.Parallel()

	runID, runJournal, eventsCh, cleanup := openRuntimeEventCapture(t)
	defer cleanup()

	var jobUsage model.Usage
	var aggregate model.Usage
	var aggregateMu sync.Mutex
	handler := newSessionUpdateHandler(SessionUpdateHandlerConfig{
		Context:        context.Background(),
		Index:          0,
		AgentID:        model.IDECodex,
		SessionID:      "sess-full",
		RunID:          runID,
		OutWriter:      io.Discard,
		ErrWriter:      io.Discard,
		RunJournal:     runJournal,
		JobUsage:       &jobUsage,
		AggregateUsage: &aggregate,
		AggregateMu:    &aggregateMu,
	})
	textBlock := mustContentBlockLoggingTest(t, model.TextBlock{Text: "non-blocking"})

	done := make(chan error, 1)
	go func() {
		done <- handler.HandleUpdate(model.SessionUpdate{
			Kind:   model.UpdateKindAgentMessageChunk,
			Blocks: []model.ContentBlock{textBlock},
			Usage:  model.Usage{InputTokens: 4, OutputTokens: 3, TotalTokens: 7},
			Status: model.StatusRunning,
		})
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("handle update: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for handler with full UI channel")
	}

	if got := aggregate.TotalTokens; got != 7 {
		t.Fatalf("expected aggregate usage update despite full UI channel, got %#v", aggregate)
	}

	events := collectRuntimeEvents(t, eventsCh, 2)
	if got := events[0].Kind; got != eventspkg.EventKindSessionUpdate {
		t.Fatalf("expected first event to be session.update, got %s", got)
	}
	if got := events[1].Kind; got != eventspkg.EventKindUsageUpdated {
		t.Fatalf("expected second event to be usage.updated, got %s", got)
	}
}

func TestSessionUpdateHandlerDispatchesUpdateObserverWithoutBlocking(t *testing.T) {
	t.Parallel()

	runID, runJournal, _, cleanup := openRuntimeEventCapture(t)
	defer cleanup()

	manager := newAsyncObserverManager()
	handler := newSessionUpdateHandler(SessionUpdateHandlerConfig{
		Context:    context.Background(),
		Index:      0,
		AgentID:    model.IDECodex,
		JobID:      "job-123",
		SessionID:  "sess-observer",
		RunID:      runID,
		OutWriter:  io.Discard,
		ErrWriter:  io.Discard,
		RunJournal: runJournal,
		RunManager: manager,
	})

	update := model.SessionUpdate{
		Kind:   model.UpdateKindAgentMessageChunk,
		Blocks: []model.ContentBlock{mustContentBlockLoggingTest(t, model.TextBlock{Text: "observer update"})},
		Status: model.StatusRunning,
	}

	for i := 0; i < 2; i++ {
		done := make(chan error, 1)
		go func() {
			done <- handler.HandleUpdate(update)
		}()

		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("handle update %d: %v", i, err)
			}
		case <-time.After(200 * time.Millisecond):
			t.Fatalf("handle update %d blocked on observer dispatch", i)
		}
	}

	manager.release()
	calls := manager.waitForCalls(t, 2)
	for _, call := range calls {
		if call.hook != "agent.on_session_update" {
			t.Fatalf("unexpected observer hook %q", call.hook)
		}
		payload := call.payload.(sessionUpdateHookPayload)
		if payload.JobID != "job-123" || payload.SessionID != "sess-observer" {
			t.Fatalf("unexpected observer payload: %#v", payload)
		}
	}
}

func TestSessionUpdateHandlerMarshalsPublicUpdateOnceAndReusesRawPayload(t *testing.T) {
	runID, runJournal, updates, cleanup := openRuntimeEventCapture(t)
	defer cleanup()

	manager := newAsyncObserverManager()
	handler := newSessionUpdateHandler(SessionUpdateHandlerConfig{
		Context:    context.Background(),
		Index:      4,
		AgentID:    model.IDECodex,
		JobID:      "job-raw",
		SessionID:  "sess-raw",
		RunID:      runID,
		OutWriter:  io.Discard,
		ErrWriter:  io.Discard,
		RunJournal: runJournal,
		RunManager: manager,
	})

	originalMarshal := marshalPublicSessionUpdateJSON
	marshalCalls := 0
	marshalPublicSessionUpdateJSON = func(v any) ([]byte, error) {
		marshalCalls++
		return json.Marshal(v)
	}
	defer func() {
		marshalPublicSessionUpdateJSON = originalMarshal
	}()

	update := model.SessionUpdate{
		Kind:   model.UpdateKindAgentMessageChunk,
		Status: model.StatusRunning,
		Blocks: []model.ContentBlock{mustContentBlockLoggingTest(t, model.TextBlock{Text: "raw update"})},
	}

	if err := handler.HandleUpdate(update); err != nil {
		t.Fatalf("handle update: %v", err)
	}

	manager.release()
	calls := manager.waitForCalls(t, 1)
	if marshalCalls != 1 {
		t.Fatalf("public update marshal calls = %d, want 1", marshalCalls)
	}

	payload := calls[0].payload.(sessionUpdateHookPayload)
	if len(payload.Update) == 0 {
		t.Fatal("expected observer hook payload to include raw update bytes")
	}

	runtimeEvents := collectRuntimeEvents(t, updates, 1)
	if runtimeEvents[0].Kind != eventspkg.EventKindSessionUpdate {
		t.Fatalf("runtime event kind = %s, want %s", runtimeEvents[0].Kind, eventspkg.EventKindSessionUpdate)
	}

	var eventPayload sessionUpdateEventPayload
	decodeRuntimeEventPayload(t, runtimeEvents[0], &eventPayload)
	if eventPayload.Index != 4 {
		t.Fatalf("runtime payload index = %d, want 4", eventPayload.Index)
	}
	if string(eventPayload.Update) != string(payload.Update) {
		t.Fatal("raw update bytes differ between runtime event and observer hook")
	}
}

func TestSessionUpdateHandlerCompletionSignalsDone(t *testing.T) {
	t.Parallel()

	runID, runJournal, _, cleanup := openRuntimeEventCapture(t)
	defer cleanup()

	handler := newSessionUpdateHandler(SessionUpdateHandlerConfig{
		Context:    context.Background(),
		Index:      0,
		AgentID:    model.IDECodex,
		SessionID:  "sess-done",
		RunID:      runID,
		OutWriter:  io.Discard,
		ErrWriter:  io.Discard,
		RunJournal: runJournal,
	})

	if err := handler.HandleUpdate(model.SessionUpdate{Status: model.StatusCompleted}); err != nil {
		t.Fatalf("handle update: %v", err)
	}

	select {
	case <-handler.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for handler done")
	}

	if err := handler.Err(); err != nil {
		t.Fatalf("expected nil handler error, got %v", err)
	}
}

func TestSessionUpdateHandlerFailedStatusPropagatesError(t *testing.T) {
	t.Parallel()

	var errBuf bytes.Buffer
	runID, runJournal, eventsCh, cleanup := openRuntimeEventCapture(t)
	defer cleanup()

	handler := newSessionUpdateHandler(SessionUpdateHandlerConfig{
		Context:    context.Background(),
		Index:      0,
		AgentID:    model.IDECodex,
		SessionID:  "sess-failed",
		RunID:      runID,
		OutWriter:  io.Discard,
		ErrWriter:  &errBuf,
		RunJournal: runJournal,
	})

	if err := handler.HandleUpdate(model.SessionUpdate{Status: model.StatusFailed}); err != nil {
		t.Fatalf("handle update: %v", err)
	}
	if got := handler.Err(); got == nil || !strings.Contains(got.Error(), "reported failed status") {
		t.Fatalf("expected failed status error, got %v", got)
	}

	if err := handler.HandleCompletion(errors.New("boom")); err != nil {
		t.Fatalf("handle completion: %v", err)
	}
	if got := handler.Err(); got == nil || !strings.Contains(got.Error(), "boom") {
		t.Fatalf("expected completion error to override handler error, got %v", got)
	}
	if got := errBuf.String(); !strings.Contains(got, "ACP session error: boom") {
		t.Fatalf("expected completion error to be written to stderr log, got %q", got)
	}

	events := collectRuntimeEvents(t, eventsCh, 2)
	if got := events[0].Kind; got != eventspkg.EventKindSessionUpdate {
		t.Fatalf("expected first event to be session.update, got %s", got)
	}
	if got := events[1].Kind; got != eventspkg.EventKindSessionFailed {
		t.Fatalf("expected second event to be session.failed, got %s", got)
	}

	var payload kinds.SessionFailedPayload
	decodeRuntimeEventPayload(t, events[1], &payload)
	if payload.Index != 0 || payload.Error != "boom" {
		t.Fatalf("unexpected session failed payload: %#v", payload)
	}
}

func TestSessionUpdateHandlerCompletionWriteFailureStillSignalsDone(t *testing.T) {
	t.Parallel()

	runID, runJournal, eventsCh, cleanup := openRuntimeEventCapture(t)
	defer cleanup()

	handler := newSessionUpdateHandler(SessionUpdateHandlerConfig{
		Context:    context.Background(),
		Index:      0,
		AgentID:    model.IDECodex,
		SessionID:  "sess-write-fail",
		RunID:      runID,
		OutWriter:  io.Discard,
		ErrWriter:  failingWriter{},
		RunJournal: runJournal,
	})
	err := handler.HandleCompletion(errors.New("boom"))
	if err == nil || !strings.Contains(err.Error(), "write ACP session completion error") {
		t.Fatalf("expected completion write failure, got %v", err)
	}

	select {
	case <-handler.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for handler done after write failure")
	}

	if got := handler.Err(); got == nil || !strings.Contains(got.Error(), "boom") {
		t.Fatalf("expected original completion error to be preserved, got %v", got)
	}

	events := collectRuntimeEvents(t, eventsCh, 1)
	if got := events[0].Kind; got != eventspkg.EventKindSessionFailed {
		t.Fatalf("expected session.failed event, got %s", got)
	}
}

func TestSessionUpdateHandlerEmitsNestedReusableAgentLifecycleEvents(t *testing.T) {
	t.Parallel()

	runID, runJournal, eventsCh, cleanup := openRuntimeEventCapture(t)
	defer cleanup()

	handler := newSessionUpdateHandler(SessionUpdateHandlerConfig{
		Context:    context.Background(),
		Index:      0,
		AgentID:    model.IDECodex,
		SessionID:  "sess-nested-success",
		RunID:      runID,
		OutWriter:  io.Discard,
		ErrWriter:  io.Discard,
		RunJournal: runJournal,
		ReusableAgent: &reusableAgentExecution{
			Name:   "parent",
			Source: "workspace",
		},
	})

	startBlock := mustContentBlockLoggingTest(t, model.ToolUseBlock{
		ID:       "tool-1",
		Name:     "run_agent",
		ToolName: "run_agent",
		Input:    json.RawMessage(`{"name":"child","input":"delegate this"}`),
	})
	if err := handler.HandleUpdate(model.SessionUpdate{
		Kind:          model.UpdateKindToolCallStarted,
		ToolCallID:    "tool-1",
		ToolCallState: model.ToolCallStatePending,
		Blocks:        []model.ContentBlock{startBlock},
		Status:        model.StatusRunning,
	}); err != nil {
		t.Fatalf("handle nested start update: %v", err)
	}

	resultBlock := mustContentBlockLoggingTest(t, model.ToolResultBlock{
		ToolUseID: "tool-1",
		Content:   `{"name":"child","source":"workspace","run_id":"run-child","success":true,"parent_agent_name":"parent","depth":1,"max_depth":3}`,
	})
	if err := handler.HandleUpdate(model.SessionUpdate{
		Kind:          model.UpdateKindToolCallUpdated,
		ToolCallID:    "tool-1",
		ToolCallState: model.ToolCallStateCompleted,
		Blocks:        []model.ContentBlock{resultBlock},
		Status:        model.StatusRunning,
	}); err != nil {
		t.Fatalf("handle nested completion update: %v", err)
	}

	events := collectRuntimeEvents(t, eventsCh, 4)
	if got := []eventspkg.EventKind{
		events[0].Kind,
		events[1].Kind,
		events[2].Kind,
		events[3].Kind,
	}; !reflect.DeepEqual(
		got,
		[]eventspkg.EventKind{
			eventspkg.EventKindReusableAgentLifecycle,
			eventspkg.EventKindSessionUpdate,
			eventspkg.EventKindReusableAgentLifecycle,
			eventspkg.EventKindSessionUpdate,
		},
	) {
		t.Fatalf("unexpected event order: %v", got)
	}

	var started kinds.ReusableAgentLifecyclePayload
	decodeRuntimeEventPayload(t, events[0], &started)
	if started.Stage != kinds.ReusableAgentLifecycleStageNestedStarted ||
		started.AgentName != "child" ||
		started.ParentAgentName != "parent" {
		t.Fatalf("unexpected nested start payload: %#v", started)
	}

	var completed kinds.ReusableAgentLifecyclePayload
	decodeRuntimeEventPayload(t, events[2], &completed)
	if completed.Stage != kinds.ReusableAgentLifecycleStageNestedCompleted ||
		completed.AgentName != "child" ||
		completed.OutputRunID != "run-child" ||
		!completed.Success {
		t.Fatalf("unexpected nested completion payload: %#v", completed)
	}
}

func TestSessionUpdateHandlerEmitsNestedBlockedLifecycleAndKeepsParentSessionUsable(t *testing.T) {
	t.Parallel()

	runID, runJournal, eventsCh, cleanup := openRuntimeEventCapture(t)
	defer cleanup()

	handler := newSessionUpdateHandler(SessionUpdateHandlerConfig{
		Context:    context.Background(),
		Index:      0,
		AgentID:    model.IDECodex,
		SessionID:  "sess-nested-blocked",
		RunID:      runID,
		OutWriter:  io.Discard,
		ErrWriter:  io.Discard,
		RunJournal: runJournal,
		ReusableAgent: &reusableAgentExecution{
			Name:   "parent",
			Source: "workspace",
		},
	})

	startBlock := mustContentBlockLoggingTest(t, model.ToolUseBlock{
		ID:       "tool-1",
		Name:     "run_agent",
		ToolName: "run_agent",
		Input:    json.RawMessage(`{"name":"child","input":"delegate this"}`),
	})
	if err := handler.HandleUpdate(model.SessionUpdate{
		Kind:          model.UpdateKindToolCallStarted,
		ToolCallID:    "tool-1",
		ToolCallState: model.ToolCallStatePending,
		Blocks:        []model.ContentBlock{startBlock},
		Status:        model.StatusRunning,
	}); err != nil {
		t.Fatalf("handle nested start update: %v", err)
	}

	blockedBlock := mustContentBlockLoggingTest(t, model.ToolResultBlock{
		ToolUseID: "tool-1",
		Content:   `{"name":"child","source":"workspace","success":false,"blocked":true,"blocked_reason":"cycle-detected","error":"nested execution blocked: cycle detected","parent_agent_name":"parent","depth":2,"max_depth":3}`,
		IsError:   true,
	})
	if err := handler.HandleUpdate(model.SessionUpdate{
		Kind:          model.UpdateKindToolCallUpdated,
		ToolCallID:    "tool-1",
		ToolCallState: model.ToolCallStateFailed,
		Blocks:        []model.ContentBlock{blockedBlock},
		Status:        model.StatusRunning,
	}); err != nil {
		t.Fatalf("handle nested blocked update: %v", err)
	}

	recoveryBlock := mustContentBlockLoggingTest(t, model.TextBlock{Text: "parent recovered"})
	if err := handler.HandleUpdate(model.SessionUpdate{
		Kind:   model.UpdateKindAgentMessageChunk,
		Blocks: []model.ContentBlock{recoveryBlock},
		Status: model.StatusRunning,
	}); err != nil {
		t.Fatalf("handle recovery update: %v", err)
	}

	events := collectRuntimeEvents(t, eventsCh, 5)
	if got := []eventspkg.EventKind{
		events[0].Kind,
		events[1].Kind,
		events[2].Kind,
		events[3].Kind,
		events[4].Kind,
	}; !reflect.DeepEqual(
		got,
		[]eventspkg.EventKind{
			eventspkg.EventKindReusableAgentLifecycle,
			eventspkg.EventKindSessionUpdate,
			eventspkg.EventKindReusableAgentLifecycle,
			eventspkg.EventKindSessionUpdate,
			eventspkg.EventKindSessionUpdate,
		},
	) {
		t.Fatalf("unexpected event order: %v", got)
	}

	var blocked kinds.ReusableAgentLifecyclePayload
	decodeRuntimeEventPayload(t, events[2], &blocked)
	if blocked.Stage != kinds.ReusableAgentLifecycleStageNestedBlocked ||
		blocked.BlockedReason != kinds.ReusableAgentBlockedReasonCycleDetected ||
		!blocked.Blocked {
		t.Fatalf("unexpected nested blocked payload: %#v", blocked)
	}

	snapshot := handler.Snapshot()
	if len(snapshot.Entries) < 2 {
		t.Fatalf("expected blocked tool call plus recovery transcript, got %#v", snapshot.Entries)
	}
	text, err := snapshot.Entries[len(snapshot.Entries)-1].Blocks[0].AsText()
	if err != nil {
		t.Fatalf("decode recovery text: %v", err)
	}
	if text.Text != "parent recovered" {
		t.Fatalf("unexpected recovery transcript: %#v", snapshot.Entries)
	}
}

func TestSessionUpdateHandlerBuffersNestedBlockedLifecycleUntilStartArrives(t *testing.T) {
	t.Parallel()

	runID, runJournal, eventsCh, cleanup := openRuntimeEventCapture(t)
	defer cleanup()

	handler := newSessionUpdateHandler(SessionUpdateHandlerConfig{
		Context:    context.Background(),
		Index:      0,
		AgentID:    model.IDECodex,
		SessionID:  "sess-nested-buffered-blocked",
		RunID:      runID,
		OutWriter:  io.Discard,
		ErrWriter:  io.Discard,
		RunJournal: runJournal,
		ReusableAgent: &reusableAgentExecution{
			Name:   "parent",
			Source: "workspace",
		},
	})

	blockedBlock := mustContentBlockLoggingTest(t, model.ToolResultBlock{
		ToolUseID: "tool-1",
		Content:   `{"name":"child","source":"workspace","success":false,"blocked":true,"blocked_reason":"cycle-detected","error":"nested execution blocked: cycle detected","parent_agent_name":"parent","depth":2,"max_depth":3}`,
		IsError:   true,
	})
	if err := handler.HandleUpdate(model.SessionUpdate{
		Kind:          model.UpdateKindToolCallUpdated,
		ToolCallID:    "tool-1",
		ToolCallState: model.ToolCallStateFailed,
		Blocks:        []model.ContentBlock{blockedBlock},
		Status:        model.StatusRunning,
	}); err != nil {
		t.Fatalf("handle nested blocked update before start: %v", err)
	}

	startBlock := mustContentBlockLoggingTest(t, model.ToolUseBlock{
		ID:       "tool-1",
		Name:     "run_agent",
		ToolName: "run_agent",
		Input:    json.RawMessage(`{"name":"child","input":"delegate this"}`),
	})
	if err := handler.HandleUpdate(model.SessionUpdate{
		Kind:          model.UpdateKindToolCallStarted,
		ToolCallID:    "tool-1",
		ToolCallState: model.ToolCallStatePending,
		Blocks:        []model.ContentBlock{startBlock},
		Status:        model.StatusRunning,
	}); err != nil {
		t.Fatalf("handle nested start update after blocked result: %v", err)
	}

	recoveryBlock := mustContentBlockLoggingTest(t, model.TextBlock{Text: "parent recovered"})
	if err := handler.HandleUpdate(model.SessionUpdate{
		Kind:   model.UpdateKindAgentMessageChunk,
		Blocks: []model.ContentBlock{recoveryBlock},
		Status: model.StatusRunning,
	}); err != nil {
		t.Fatalf("handle recovery update: %v", err)
	}

	events := collectRuntimeEvents(t, eventsCh, 5)
	var lifecycle []kinds.ReusableAgentLifecyclePayload
	for _, event := range events {
		if event.Kind != eventspkg.EventKindReusableAgentLifecycle {
			continue
		}
		var payload kinds.ReusableAgentLifecyclePayload
		decodeRuntimeEventPayload(t, event, &payload)
		lifecycle = append(lifecycle, payload)
	}
	if len(lifecycle) != 2 {
		t.Fatalf("expected buffered nested lifecycle events, got %#v", lifecycle)
	}
	if lifecycle[0].Stage != kinds.ReusableAgentLifecycleStageNestedStarted {
		t.Fatalf("unexpected nested started payload: %#v", lifecycle[0])
	}
	if lifecycle[1].Stage != kinds.ReusableAgentLifecycleStageNestedBlocked ||
		lifecycle[1].BlockedReason != kinds.ReusableAgentBlockedReasonCycleDetected ||
		!lifecycle[1].Blocked {
		t.Fatalf("unexpected nested blocked payload: %#v", lifecycle[1])
	}

	if _, exists := handler.pendingNestedResults["tool-1"]; exists {
		t.Fatalf("expected buffered nested result to be cleared, got %#v", handler.pendingNestedResults)
	}
	if _, exists := handler.nestedToolCalls["tool-1"]; exists {
		t.Fatalf("expected nested tool call tracking to be cleared, got %#v", handler.nestedToolCalls)
	}

	snapshot := handler.Snapshot()
	if len(snapshot.Entries) < 2 {
		t.Fatalf("expected tool use plus recovery transcript, got %#v", snapshot.Entries)
	}
	text, err := snapshot.Entries[len(snapshot.Entries)-1].Blocks[0].AsText()
	if err != nil {
		t.Fatalf("decode recovery text: %v", err)
	}
	if text.Text != "parent recovered" {
		t.Fatalf("unexpected recovery transcript: %#v", snapshot.Entries)
	}
}

func TestSessionUpdateHandlerSkipsNestedTrackingWhenLifecycleStartSubmitFails(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	submitter := &stubRuntimeEventSubmitter{
		submitFn: func(ev eventspkg.Event) error {
			if ev.Kind == eventspkg.EventKindReusableAgentLifecycle {
				return errors.New("journal unavailable")
			}
			return nil
		},
	}
	handler := newSessionUpdateHandler(SessionUpdateHandlerConfig{
		Context:    context.Background(),
		Index:      0,
		AgentID:    model.IDECodex,
		SessionID:  "sess-nested-start-failure",
		RunID:      "run-lifecycle",
		OutWriter:  io.Discard,
		ErrWriter:  io.Discard,
		RunJournal: submitter,
		Logger: slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{
			Level: slog.LevelWarn,
		})),
		ReusableAgent: &reusableAgentExecution{
			Name:   "parent",
			Source: "workspace",
		},
	})

	startBlock := mustContentBlockLoggingTest(t, model.ToolUseBlock{
		ID:       "tool-1",
		Name:     "run_agent",
		ToolName: "run_agent",
		Input:    json.RawMessage(`{"name":"child","input":"delegate this"}`),
	})
	if err := handler.HandleUpdate(model.SessionUpdate{
		Kind:          model.UpdateKindToolCallStarted,
		ToolCallID:    "tool-1",
		ToolCallState: model.ToolCallStatePending,
		Blocks:        []model.ContentBlock{startBlock},
		Status:        model.StatusRunning,
	}); err != nil {
		t.Fatalf("handle nested start update: %v", err)
	}

	if _, exists := handler.nestedToolCalls["tool-1"]; exists {
		t.Fatalf("expected failed lifecycle submit not to track nested tool call, got %#v", handler.nestedToolCalls)
	}
	if !strings.Contains(logs.String(), "failed to emit reusable agent lifecycle from session update; continuing") {
		t.Fatalf("expected lifecycle warning log, got %q", logs.String())
	}
	if got := submitter.countKind(eventspkg.EventKindSessionUpdate); got != 1 {
		t.Fatalf("expected session update event to continue, got %d", got)
	}
}

func TestSessionUpdateHandlerRetainsNestedTrackingWhenLifecycleCompletionSubmitFails(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	lifecycleAttempts := 0
	submitter := &stubRuntimeEventSubmitter{
		submitFn: func(ev eventspkg.Event) error {
			if ev.Kind != eventspkg.EventKindReusableAgentLifecycle {
				return nil
			}
			lifecycleAttempts++
			if lifecycleAttempts == 2 {
				return errors.New("journal unavailable")
			}
			return nil
		},
	}
	handler := newSessionUpdateHandler(SessionUpdateHandlerConfig{
		Context:    context.Background(),
		Index:      0,
		AgentID:    model.IDECodex,
		SessionID:  "sess-nested-complete-failure",
		RunID:      "run-lifecycle",
		OutWriter:  io.Discard,
		ErrWriter:  io.Discard,
		RunJournal: submitter,
		Logger: slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{
			Level: slog.LevelWarn,
		})),
		ReusableAgent: &reusableAgentExecution{
			Name:   "parent",
			Source: "workspace",
		},
	})

	startBlock := mustContentBlockLoggingTest(t, model.ToolUseBlock{
		ID:       "tool-1",
		Name:     "run_agent",
		ToolName: "run_agent",
		Input:    json.RawMessage(`{"name":"child","input":"delegate this"}`),
	})
	if err := handler.HandleUpdate(model.SessionUpdate{
		Kind:          model.UpdateKindToolCallStarted,
		ToolCallID:    "tool-1",
		ToolCallState: model.ToolCallStatePending,
		Blocks:        []model.ContentBlock{startBlock},
		Status:        model.StatusRunning,
	}); err != nil {
		t.Fatalf("handle nested start update: %v", err)
	}

	resultBlock := mustContentBlockLoggingTest(t, model.ToolResultBlock{
		ToolUseID: "tool-1",
		Content:   `{"name":"child","source":"workspace","run_id":"run-child","success":true,"parent_agent_name":"parent","depth":1,"max_depth":3}`,
	})
	if err := handler.HandleUpdate(model.SessionUpdate{
		Kind:          model.UpdateKindToolCallUpdated,
		ToolCallID:    "tool-1",
		ToolCallState: model.ToolCallStateCompleted,
		Blocks:        []model.ContentBlock{resultBlock},
		Status:        model.StatusRunning,
	}); err != nil {
		t.Fatalf("handle nested completion update: %v", err)
	}

	if _, exists := handler.nestedToolCalls["tool-1"]; !exists {
		t.Fatalf(
			"expected failed completion lifecycle submit to preserve nested tool call, got %#v",
			handler.nestedToolCalls,
		)
	}
	if !strings.Contains(logs.String(), "failed to emit reusable agent lifecycle from session update; continuing") {
		t.Fatalf("expected lifecycle warning log, got %q", logs.String())
	}
	if got := submitter.countKind(eventspkg.EventKindSessionUpdate); got != 2 {
		t.Fatalf("expected session update events to continue, got %d", got)
	}
}

func TestSessionUpdateHandlerDispatchesPostSessionEndOncePerCompletion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		err        error
		wantStatus model.SessionStatus
		wantError  string
	}{
		{
			name:       "success",
			wantStatus: model.StatusCompleted,
		},
		{
			name:       "error",
			err:        errors.New("boom"),
			wantStatus: model.StatusFailed,
			wantError:  "boom",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			runID, runJournal, _, cleanup := openRuntimeEventCapture(t)
			defer cleanup()

			manager := newAsyncObserverManager()
			manager.release()
			handler := newSessionUpdateHandler(SessionUpdateHandlerConfig{
				Context:    context.Background(),
				Index:      0,
				AgentID:    model.IDECodex,
				JobID:      "job-456",
				SessionID:  "sess-complete",
				RunID:      runID,
				OutWriter:  io.Discard,
				ErrWriter:  io.Discard,
				RunJournal: runJournal,
				RunManager: manager,
			})

			if err := handler.HandleCompletion(tc.err); err != nil {
				t.Fatalf("handle completion: %v", err)
			}

			calls := manager.waitForCalls(t, 1)
			if len(calls) != 1 {
				t.Fatalf("expected one observer call, got %d", len(calls))
			}
			if calls[0].hook != "agent.post_session_end" {
				t.Fatalf("unexpected observer hook %q", calls[0].hook)
			}

			payload := calls[0].payload.(sessionEndedHookPayload)
			if payload.Outcome.Status != tc.wantStatus || payload.Outcome.Error != tc.wantError {
				t.Fatalf("unexpected completion payload: %#v", payload)
			}
		})
	}
}
func TestRenderContentBlocksHandlesTerminalImageAndDecodeFallback(t *testing.T) {
	t.Parallel()

	terminalBlock := mustContentBlockLoggingTest(t, model.TerminalOutputBlock{
		Command:  "pwd",
		Output:   "/tmp/project",
		ExitCode: 0,
	})
	imageBlock := mustContentBlockLoggingTest(t, model.ImageBlock{
		Data:     "ZGF0YQ==",
		MimeType: "image/png",
	})
	invalidBlock := model.ContentBlock{
		Type: model.BlockDiff,
		Data: []byte(`{"type":"diff","filePath":1}`),
	}

	outLines, errLines := renderContentBlocks([]model.ContentBlock{terminalBlock, imageBlock, invalidBlock})
	if len(errLines) != 0 {
		t.Fatalf("expected no stderr lines from terminal/image/decode fallback, got %v", errLines)
	}
	joined := strings.Join(outLines, "\n")
	for _, want := range []string{"$ pwd", "/tmp/project", "[IMAGE] image/png inline", "[decode diff block failed]"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected rendered output to contain %q, got %q", want, joined)
		}
	}
}

func mustContentBlockLoggingTest(t *testing.T, payload any) model.ContentBlock {
	t.Helper()

	block, err := model.NewContentBlock(payload)
	if err != nil {
		t.Fatalf("new content block: %v", err)
	}
	return block
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

type asyncObserverCall struct {
	hook    string
	payload any
}

type asyncObserverManager struct {
	releaseCh chan struct{}
	callsCh   chan asyncObserverCall
}

func newAsyncObserverManager() *asyncObserverManager {
	return &asyncObserverManager{
		releaseCh: make(chan struct{}),
		callsCh:   make(chan asyncObserverCall, 16),
	}
}

func (*asyncObserverManager) Start(context.Context) error { return nil }

func (*asyncObserverManager) Shutdown(context.Context) error { return nil }

func (m *asyncObserverManager) DispatchMutableHook(
	context.Context,
	string,
	any,
) (any, error) {
	return nil, nil
}

func (m *asyncObserverManager) DispatchObserverHook(_ context.Context, hook string, payload any) {
	go func() {
		<-m.releaseCh
		m.callsCh <- asyncObserverCall{hook: hook, payload: payload}
	}()
}

func (m *asyncObserverManager) release() {
	select {
	case <-m.releaseCh:
	default:
		close(m.releaseCh)
	}
}

func (m *asyncObserverManager) waitForCalls(t *testing.T, count int) []asyncObserverCall {
	t.Helper()

	calls := make([]asyncObserverCall, 0, count)
	deadline := time.After(2 * time.Second)
	for len(calls) < count {
		select {
		case call := <-m.callsCh:
			calls = append(calls, call)
		case <-deadline:
			t.Fatalf("timed out waiting for %d observer calls, got %d", count, len(calls))
		}
	}
	return calls
}

func openRuntimeEventCapture(
	t *testing.T,
) (string, *journal.Journal, <-chan eventspkg.Event, func()) {
	t.Helper()

	workspaceRoot := t.TempDir()
	runArtifacts := model.NewRunArtifacts(workspaceRoot, "logging-test-run")
	if err := os.MkdirAll(filepath.Dir(runArtifacts.EventsPath), 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	bus := eventspkg.New[eventspkg.Event](16)
	_, ch, unsubscribe := bus.Subscribe()
	runJournal, err := journal.Open(runArtifacts.EventsPath, bus, 16)
	if err != nil {
		t.Fatalf("open journal: %v", err)
	}

	cleanup := func() {
		t.Helper()
		closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := runJournal.Close(closeCtx); err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("close journal: %v", err)
		}
		unsubscribe()
		if err := bus.Close(context.Background()); err != nil {
			t.Fatalf("close bus: %v", err)
		}
	}

	return runArtifacts.RunID, runJournal, ch, cleanup
}

func collectRuntimeEvents(t *testing.T, ch <-chan eventspkg.Event, want int) []eventspkg.Event {
	t.Helper()

	got := make([]eventspkg.Event, 0, want)
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()

	for len(got) < want {
		select {
		case ev := <-ch:
			got = append(got, ev)
		case <-deadline.C:
			t.Fatalf("timed out waiting for %d runtime events, got %d", want, len(got))
		}
	}

	return got
}

func decodeRuntimeEventPayload(t *testing.T, ev eventspkg.Event, dst any) {
	t.Helper()

	if err := json.Unmarshal(ev.Payload, dst); err != nil {
		t.Fatalf("decode %s payload: %v", ev.Kind, err)
	}
}
