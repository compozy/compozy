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

func TestSessionUpdateHandlerRoutesTextBlocksToLogAndUI(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	var err bytes.Buffer
	uiCh := make(chan uiMsg, 2)
	var aggregate model.Usage
	handler := newSessionUpdateHandler(3, model.IDECodex, "sess-123", &out, &err, uiCh, &aggregate, &sync.Mutex{})

	textBlock, blockErr := model.NewContentBlock(model.TextBlock{Text: "hello from ACP"})
	if blockErr != nil {
		t.Fatalf("new content block: %v", blockErr)
	}

	if handleErr := handler.HandleUpdate(model.SessionUpdate{
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
		if len(updateMsg.Blocks) != 1 || updateMsg.Blocks[0].Type != model.BlockText {
			t.Fatalf("unexpected job update blocks: %#v", updateMsg.Blocks)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for UI update")
	}
}

func TestSessionUpdateHandlerRoutesMixedBlocksAndUsage(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	var err bytes.Buffer
	uiCh := make(chan uiMsg, 4)
	var aggregate model.Usage
	var aggregateMu sync.Mutex
	handler := newSessionUpdateHandler(0, model.IDECodex, "sess-mixed", &out, &err, uiCh, &aggregate, &aggregateMu)

	toolUseBlock, blockErr := model.NewContentBlock(model.ToolUseBlock{
		ID:    "tool-1",
		Name:  "read_file",
		Input: []byte(`{"path":"README.md"}`),
	})
	if blockErr != nil {
		t.Fatalf("new tool_use block: %v", blockErr)
	}
	diffBlock, blockErr := model.NewContentBlock(model.DiffBlock{
		FilePath: "README.md",
		Diff:     "@@ -1 +1 @@\n-old\n+new",
		NewText:  "new",
	})
	if blockErr != nil {
		t.Fatalf("new diff block: %v", blockErr)
	}
	toolErrBlock, blockErr := model.NewContentBlock(model.ToolResultBlock{
		ToolUseID: "tool-1",
		Content:   "permission denied",
		IsError:   true,
	})
	if blockErr != nil {
		t.Fatalf("new tool_result block: %v", blockErr)
	}

	update := model.SessionUpdate{
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

	var sawBlocks bool
	var sawUsage bool
	for i := 0; i < 2; i++ {
		select {
		case msg := <-uiCh:
			switch v := msg.(type) {
			case jobUpdateMsg:
				sawBlocks = true
				if len(v.Blocks) != 3 {
					t.Fatalf("expected 3 blocks in job update, got %d", len(v.Blocks))
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
	if !sawBlocks || !sawUsage {
		t.Fatalf("expected both job and usage updates, got blocks=%v usage=%v", sawBlocks, sawUsage)
	}
}

func TestSessionUpdateHandlerCompletionSignalsDone(t *testing.T) {
	t.Parallel()

	handler := newSessionUpdateHandler(0, model.IDECodex, "sess-done", io.Discard, io.Discard, nil, nil, nil)

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
	handler := newSessionUpdateHandler(0, model.IDECodex, "sess-failed", io.Discard, &errBuf, nil, nil, nil)

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

func TestRenderContentBlocksHandlesTerminalImageAndDecodeFallback(t *testing.T) {
	t.Parallel()

	terminalBlock, err := model.NewContentBlock(model.TerminalOutputBlock{
		Command:  "pwd",
		Output:   "/tmp/project",
		ExitCode: 0,
	})
	if err != nil {
		t.Fatalf("new terminal block: %v", err)
	}
	imageBlock, err := model.NewContentBlock(model.ImageBlock{
		Data:     "ZGF0YQ==",
		MimeType: "image/png",
	})
	if err != nil {
		t.Fatalf("new image block: %v", err)
	}
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
