package run

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/model"
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
	uiCh := make(chan uiMsg, 2)
	var aggregate model.Usage
	handler := newSessionUpdateHandler(
		3,
		model.IDECodex,
		"sess-123",
		nil,
		&out,
		&err,
		uiCh,
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

	select {
	case msg := <-uiCh:
		updateMsg, ok := msg.(jobUpdateMsg)
		if !ok {
			t.Fatalf("expected jobUpdateMsg, got %T", msg)
		}
		if updateMsg.Index != 3 {
			t.Fatalf("expected job update index 3, got %d", updateMsg.Index)
		}
		if len(updateMsg.Snapshot.Entries) != 1 ||
			updateMsg.Snapshot.Entries[0].Kind != transcriptEntryAssistantMessage {
			t.Fatalf("unexpected snapshot entries: %#v", updateMsg.Snapshot.Entries)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for UI update")
	}
}

func TestSessionUpdateHandlerMergesTranscriptAndCarriesSessionState(t *testing.T) {
	t.Parallel()

	uiCh := make(chan uiMsg, 8)
	handler := newSessionUpdateHandler(
		1,
		model.IDECodex,
		"sess-merge",
		nil,
		io.Discard,
		io.Discard,
		uiCh,
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

	var last jobUpdateMsg
	for i := 0; i < len(updates); i++ {
		msg := <-uiCh
		update, ok := msg.(jobUpdateMsg)
		if !ok {
			t.Fatalf("expected jobUpdateMsg, got %T", msg)
		}
		last = update
	}

	if len(last.Snapshot.Entries) != 1 {
		t.Fatalf("expected merged assistant entry, got %#v", last.Snapshot.Entries)
	}
	textBlock, err := last.Snapshot.Entries[0].Blocks[0].AsText()
	if err != nil {
		t.Fatalf("decode merged text block: %v", err)
	}
	if want := "Ledger Snapshot: Goal is fix the TUI"; textBlock.Text != want {
		t.Fatalf("unexpected merged transcript text: got %q want %q", textBlock.Text, want)
	}
	if last.Snapshot.Plan.RunningCount != 1 {
		t.Fatalf("expected plan state in snapshot, got %#v", last.Snapshot.Plan)
	}
	if last.Snapshot.Session.CurrentModeID != "review" {
		t.Fatalf("expected current mode in snapshot, got %q", last.Snapshot.Session.CurrentModeID)
	}
}

func TestSessionUpdateHandlerRoutesMixedBlocksAndUsage(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	var err bytes.Buffer
	uiCh := make(chan uiMsg, 4)
	var aggregate model.Usage
	var aggregateMu sync.Mutex
	handler := newSessionUpdateHandler(
		0,
		model.IDECodex,
		"sess-mixed",
		nil,
		&out,
		&err,
		uiCh,
		&aggregate,
		&aggregateMu,
		nil,
	)

	toolUseBlock := mustContentBlockLoggingTest(t, model.ToolUseBlock{
		ID:    "tool-1",
		Name:  "read_file",
		Input: []byte(`{"path":"README.md"}`),
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

	if got := out.String(); !strings.Contains(got, "[TOOL] read_file") || !strings.Contains(got, "README.md") {
		t.Fatalf("expected mixed stdout rendering, got %q", got)
	}
	if got := err.String(); !strings.Contains(got, "permission denied") {
		t.Fatalf("expected tool error content in stderr log, got %q", got)
	}
	if got := aggregate; got.TotalTokens != 15 || got.CacheReads != 2 {
		t.Fatalf("unexpected aggregate usage: %#v", got)
	}

	var sawSnapshot bool
	var sawUsage bool
	for i := 0; i < 2; i++ {
		select {
		case msg := <-uiCh:
			switch v := msg.(type) {
			case jobUpdateMsg:
				sawSnapshot = true
				if len(v.Snapshot.Entries) < 1 {
					t.Fatalf("expected at least one snapshot entry, got %#v", v.Snapshot.Entries)
				}
			case usageUpdateMsg:
				sawUsage = true
				if v.Usage.TotalTokens != 15 {
					t.Fatalf("unexpected usage update: %#v", v.Usage)
				}
			default:
				t.Fatalf("unexpected UI message type %T", msg)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for mixed update messages")
		}
	}
	if !sawSnapshot || !sawUsage {
		t.Fatalf("expected both snapshot and usage updates, got snapshot=%v usage=%v", sawSnapshot, sawUsage)
	}
}

func TestSessionUpdateHandlerDoesNotBlockWhenUIChannelIsFull(t *testing.T) {
	t.Parallel()

	uiCh := make(chan uiMsg, 1)
	uiCh <- jobUpdateMsg{Index: 99}

	var aggregate model.Usage
	var aggregateMu sync.Mutex
	handler := newSessionUpdateHandler(
		0,
		model.IDECodex,
		"sess-full",
		nil,
		io.Discard,
		io.Discard,
		uiCh,
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
}

func TestSessionUpdateHandlerCompletionSignalsDone(t *testing.T) {
	t.Parallel()

	handler := newSessionUpdateHandler(0, model.IDECodex, "sess-done", nil, io.Discard, io.Discard, nil, nil, nil, nil)

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
	handler := newSessionUpdateHandler(0, model.IDECodex, "sess-failed", nil, io.Discard, &errBuf, nil, nil, nil, nil)

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
}

func TestSessionUpdateHandlerCompletionWriteFailureStillSignalsDone(t *testing.T) {
	t.Parallel()

	handler := newSessionUpdateHandler(
		0,
		model.IDECodex,
		"sess-write-fail",
		nil,
		io.Discard,
		failingWriter{},
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
