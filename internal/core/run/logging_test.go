package run

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/journal"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func TestLineBufferUnlimitedRetentionKeepsFullHistory(t *testing.T) {
	t.Parallel()

	buffer := newLineBuffer(0)
	for _, line := range []string{"one", "two", "three", "four"} {
		buffer.appendLine(line)
	}

	got := buffer.snapshot()
	want := []string{"one", "two", "three", "four"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("expected full history %v, got %v", want, got)
	}
}

func TestLineBufferLimitedRetentionKeepsNewestEntries(t *testing.T) {
	t.Parallel()

	buffer := newLineBuffer(2)
	for _, line := range []string{"one", "two", "three", "four"} {
		buffer.appendLine(line)
	}

	got := buffer.snapshot()
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
	handler := newSessionUpdateHandler(
		3,
		model.IDECodex,
		"sess-123",
		nil,
		runID,
		&out,
		&err,
		runJournal,
		&jobUsage,
		&aggregate,
		&sync.Mutex{},
		nil,
	)

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

func TestSessionUpdateHandlerMergesTranscriptAndCarriesSessionState(t *testing.T) {
	t.Parallel()

	runID, runJournal, eventsCh, cleanup := openRuntimeEventCapture(t)
	defer cleanup()

	handler := newSessionUpdateHandler(
		1,
		model.IDECodex,
		"sess-merge",
		nil,
		runID,
		io.Discard,
		io.Discard,
		runJournal,
		nil,
		nil,
		nil,
		nil,
	)

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
	handler := newSessionUpdateHandler(
		0,
		model.IDECodex,
		"sess-mixed",
		nil,
		runID,
		&out,
		&err,
		runJournal,
		&jobUsage,
		&aggregate,
		&aggregateMu,
		nil,
	)

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
	handler := newSessionUpdateHandler(
		0,
		model.IDECodex,
		"sess-full",
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

func TestSessionUpdateHandlerCompletionSignalsDone(t *testing.T) {
	t.Parallel()

	runID, runJournal, _, cleanup := openRuntimeEventCapture(t)
	defer cleanup()

	handler := newSessionUpdateHandler(
		0,
		model.IDECodex,
		"sess-done",
		nil,
		runID,
		io.Discard,
		io.Discard,
		runJournal,
		nil,
		nil,
		nil,
		nil,
	)

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

	handler := newSessionUpdateHandler(
		0,
		model.IDECodex,
		"sess-failed",
		nil,
		runID,
		io.Discard,
		&errBuf,
		runJournal,
		nil,
		nil,
		nil,
		nil,
	)

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

	handler := newSessionUpdateHandler(
		0,
		model.IDECodex,
		"sess-write-fail",
		nil,
		runID,
		io.Discard,
		failingWriter{},
		runJournal,
		nil,
		nil,
		nil,
		nil,
	)
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
