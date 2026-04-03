package run

import (
	"testing"

	"github.com/compozy/compozy/internal/core/model"
)

func TestSessionViewModelMergesChunkedAgentText(t *testing.T) {
	t.Parallel()

	viewModel := newSessionViewModel()
	first := mustContentBlockTranscriptTest(t, model.TextBlock{Text: "Ledger Snapshot: "})
	second := mustContentBlockTranscriptTest(t, model.TextBlock{Text: "Goal is fix the TUI"})

	if snapshot, changed := viewModel.Apply(model.SessionUpdate{
		Kind:   model.UpdateKindAgentMessageChunk,
		Blocks: []model.ContentBlock{first},
	}); !changed || len(snapshot.Entries) != 1 {
		t.Fatalf("expected first chunk to create one entry, changed=%v entries=%#v", changed, snapshot.Entries)
	}

	snapshot, changed := viewModel.Apply(model.SessionUpdate{
		Kind:   model.UpdateKindAgentMessageChunk,
		Blocks: []model.ContentBlock{second},
	})
	if !changed {
		t.Fatal("expected second chunk to update visible transcript")
	}
	if len(snapshot.Entries) != 1 {
		t.Fatalf("expected merged snapshot to contain one entry, got %d", len(snapshot.Entries))
	}

	textBlock, err := snapshot.Entries[0].Blocks[0].AsText()
	if err != nil {
		t.Fatalf("decode merged text block: %v", err)
	}
	if want := "Ledger Snapshot: Goal is fix the TUI"; textBlock.Text != want {
		t.Fatalf("unexpected merged text: got %q want %q", textBlock.Text, want)
	}
}

func TestSessionViewModelUpsertsToolCallByIDWithoutSyntheticSummary(t *testing.T) {
	t.Parallel()

	viewModel := newSessionViewModel()
	start := []model.ContentBlock{
		mustContentBlockTranscriptTest(t, model.ToolUseBlock{
			ID:    "tool-1",
			Name:  "read_file",
			Input: []byte(`{"path":"README.md"}`),
		}),
	}
	if _, changed := viewModel.Apply(model.SessionUpdate{
		Kind:          model.UpdateKindToolCallStarted,
		ToolCallID:    "tool-1",
		ToolCallState: model.ToolCallStatePending,
		Blocks:        start,
	}); !changed {
		t.Fatal("expected tool call start to change session view")
	}

	update := []model.ContentBlock{
		mustContentBlockTranscriptTest(t, model.ToolResultBlock{
			ToolUseID: "tool-1",
			Content:   "loaded README.md",
		}),
		mustContentBlockTranscriptTest(t, model.DiffBlock{
			FilePath: "README.md",
			Diff:     "@@ -1 +1 @@\n-old\n+new",
			NewText:  "new",
		}),
	}
	snapshot, changed := viewModel.Apply(model.SessionUpdate{
		Kind:          model.UpdateKindToolCallUpdated,
		ToolCallID:    "tool-1",
		ToolCallState: model.ToolCallStateCompleted,
		Blocks:        update,
	})
	if !changed {
		t.Fatal("expected tool call update to replace tool entry")
	}
	if len(snapshot.Entries) != 1 {
		t.Fatalf("expected one explicit tool entry, got %d entries", len(snapshot.Entries))
	}
	if snapshot.Entries[0].Kind != transcriptEntryToolCall {
		t.Fatalf("expected first entry to be tool call, got %s", snapshot.Entries[0].Kind)
	}
	if got := snapshot.Entries[0].Preview; got != "loaded README.md" {
		t.Fatalf("expected tool preview to come from tool output, got %q", got)
	}
}

func TestSessionViewModelKeepsConsecutiveContextToolsExplicit(t *testing.T) {
	t.Parallel()

	viewModel := newSessionViewModel()
	for _, tool := range []struct {
		id   string
		name string
	}{
		{"tool-1", "read README"},
		{"tool-2", "search codebase"},
		{"tool-3", "fetch docs"},
	} {
		_, _ = viewModel.Apply(model.SessionUpdate{
			Kind:          model.UpdateKindToolCallStarted,
			ToolCallID:    tool.id,
			ToolCallState: model.ToolCallStateCompleted,
			Blocks: []model.ContentBlock{
				mustContentBlockTranscriptTest(t, model.ToolUseBlock{ID: tool.id, Name: tool.name}),
			},
		})
	}

	snapshot, changed := viewModel.Apply(model.SessionUpdate{
		Kind:          model.UpdateKindCurrentModeUpdated,
		CurrentModeID: "review",
		Status:        model.StatusRunning,
	})
	if !changed {
		t.Fatal("expected mode update to produce a new snapshot")
	}
	if len(snapshot.Entries) != 3 {
		t.Fatalf("expected three explicit tool entries, got %#v", snapshot.Entries)
	}
	for i, entry := range snapshot.Entries {
		if entry.Kind != transcriptEntryToolCall {
			t.Fatalf("expected entry %d to be a tool call, got %#v", i, entry)
		}
	}
	if snapshot.Session.CurrentModeID != "review" {
		t.Fatalf("expected current mode to be preserved, got %q", snapshot.Session.CurrentModeID)
	}
}

func TestSessionViewModelPreservesPlanAndCommands(t *testing.T) {
	t.Parallel()

	viewModel := newSessionViewModel()
	snapshot, changed := viewModel.Apply(model.SessionUpdate{
		Kind: model.UpdateKindPlanUpdated,
		PlanEntries: []model.SessionPlanEntry{{
			Content:  "Ship redesign",
			Priority: "high",
			Status:   "in_progress",
		}},
	})
	if !changed {
		t.Fatal("expected plan update to change snapshot")
	}
	if snapshot.Plan.RunningCount != 1 || len(snapshot.Plan.Entries) != 1 {
		t.Fatalf("unexpected plan state: %#v", snapshot.Plan)
	}

	snapshot, changed = viewModel.Apply(model.SessionUpdate{
		Kind: model.UpdateKindAvailableCommandsUpdated,
		AvailableCommands: []model.SessionAvailableCommand{{
			Name:         "run",
			Description:  "Run the task",
			ArgumentHint: "--fast",
		}},
	})
	if !changed {
		t.Fatal("expected commands update to change snapshot")
	}
	if len(snapshot.Session.AvailableCommands) != 1 || snapshot.Session.AvailableCommands[0].Name != "run" {
		t.Fatalf("unexpected available commands: %#v", snapshot.Session.AvailableCommands)
	}
}

func TestSessionViewModelSkipsDuplicateVisibleState(t *testing.T) {
	t.Parallel()

	viewModel := newSessionViewModel()
	update := model.SessionUpdate{
		Kind: model.UpdateKindPlanUpdated,
		PlanEntries: []model.SessionPlanEntry{{
			Content:  "Ship redesign",
			Priority: "high",
			Status:   "in_progress",
		}},
		Status: model.StatusRunning,
	}

	firstSnapshot, changed := viewModel.Apply(update)
	if !changed {
		t.Fatal("expected first visible update to change the snapshot")
	}

	secondSnapshot, changed := viewModel.Apply(update)
	if changed {
		t.Fatalf("expected duplicate visible state to be ignored, got snapshot %#v", secondSnapshot)
	}
	if firstSnapshot.Revision == 0 {
		t.Fatalf("expected first snapshot revision to be set, got %#v", firstSnapshot)
	}
}

func mustContentBlockTranscriptTest(t *testing.T, payload any) model.ContentBlock {
	t.Helper()

	block, err := model.NewContentBlock(payload)
	if err != nil {
		t.Fatalf("new content block: %v", err)
	}
	return block
}
