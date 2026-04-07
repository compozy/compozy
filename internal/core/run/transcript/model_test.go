package transcript

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

func TestSessionViewModelMergesOverlappingAgentTextWithoutDuplicatingBoundary(t *testing.T) {
	t.Parallel()

	viewModel := newSessionViewModel()
	first := mustContentBlockTranscriptTest(t, model.TextBlock{Text: "Ledger Snapshot: Goal"})
	second := mustContentBlockTranscriptTest(t, model.TextBlock{Text: "Goal is fix the TUI"})

	if _, changed := viewModel.Apply(model.SessionUpdate{
		Kind:   model.UpdateKindAgentMessageChunk,
		Blocks: []model.ContentBlock{first},
	}); !changed {
		t.Fatal("expected first chunk to change the snapshot")
	}

	snapshot, changed := viewModel.Apply(model.SessionUpdate{
		Kind:   model.UpdateKindAgentMessageChunk,
		Blocks: []model.ContentBlock{second},
	})
	if !changed {
		t.Fatal("expected overlapping chunk to update visible transcript")
	}

	textBlock, err := snapshot.Entries[0].Blocks[0].AsText()
	if err != nil {
		t.Fatalf("decode overlapping text block: %v", err)
	}
	if want := "Ledger Snapshot: Goal is fix the TUI"; textBlock.Text != want {
		t.Fatalf("unexpected overlap-aware merge: got %q want %q", textBlock.Text, want)
	}
}

func TestSessionViewModelReplacesDivergingNonDeltaAgentSnapshot(t *testing.T) {
	t.Parallel()

	viewModel := newSessionViewModel()
	first := mustContentBlockTranscriptTest(t, model.TextBlock{
		Text: "Ledger Snapshot: Goal close review issues with scoped production fixes.",
	})
	second := mustContentBlockTranscriptTest(t, model.TextBlock{
		Text: "Ledger Snapshot: Goal remediate entity review safely against the implementation.",
	})

	if _, changed := viewModel.Apply(model.SessionUpdate{
		Kind:   model.UpdateKindAgentMessageChunk,
		Blocks: []model.ContentBlock{first},
	}); !changed {
		t.Fatal("expected first snapshot update to change the transcript")
	}

	snapshot, changed := viewModel.Apply(model.SessionUpdate{
		Kind:   model.UpdateKindAgentMessageChunk,
		Blocks: []model.ContentBlock{second},
	})
	if !changed {
		t.Fatal("expected newer non-delta snapshot to change the transcript")
	}

	if len(snapshot.Entries) != 1 {
		t.Fatalf("expected snapshot replacement to keep one entry, got %#v", snapshot.Entries)
	}
	textBlock, err := snapshot.Entries[0].Blocks[0].AsText()
	if err != nil {
		t.Fatalf("decode replacement text block: %v", err)
	}
	if want := "Ledger Snapshot: Goal remediate entity review safely against the implementation."; textBlock.Text != want {
		t.Fatalf("unexpected replacement merge: got %q want %q", textBlock.Text, want)
	}
}

func TestSessionViewModelUpsertsToolCallByIDWithoutSyntheticSummary(t *testing.T) {
	t.Parallel()

	viewModel := newSessionViewModel()
	start := []model.ContentBlock{
		mustContentBlockTranscriptTest(t, model.ToolUseBlock{
			ID:    "tool-1",
			Name:  "Read",
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
	if got := snapshot.Entries[0].Title; got != "Read README.md" {
		t.Fatalf("expected tool title to include normalized path, got %q", got)
	}
	if got := snapshot.Entries[0].Preview; got != "README.md" {
		t.Fatalf("expected tool preview to come from tool input summary, got %q", got)
	}
}

func TestSessionViewModelToolCallScenarios(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		setup  func(t *testing.T) []model.SessionUpdate
		update func(t *testing.T) model.SessionUpdate
		verify func(t *testing.T, snapshot SessionViewSnapshot)
	}{
		{
			name: "replace header metadata when tool call updates",
			setup: func(t *testing.T) []model.SessionUpdate {
				t.Helper()

				start := []model.ContentBlock{
					mustContentBlockTranscriptTest(t, model.ToolUseBlock{
						ID:   "tool-1",
						Name: "Read",
					}),
				}
				return []model.SessionUpdate{{
					Kind:          model.UpdateKindToolCallStarted,
					ToolCallID:    "tool-1",
					ToolCallState: model.ToolCallStatePending,
					Blocks:        start,
				}}
			},
			update: func(t *testing.T) model.SessionUpdate {
				t.Helper()

				update := []model.ContentBlock{
					mustContentBlockTranscriptTest(t, model.ToolUseBlock{
						ID:    "tool-1",
						Name:  "Read",
						Input: []byte(`{"file_path":"README.md","start_line":5,"end_line":12}`),
					}),
					mustContentBlockTranscriptTest(t, model.ToolResultBlock{
						ToolUseID: "tool-1",
						Content:   "updated",
					}),
				}
				return model.SessionUpdate{
					Kind:          model.UpdateKindToolCallUpdated,
					ToolCallID:    "tool-1",
					ToolCallState: model.ToolCallStateCompleted,
					Blocks:        update,
				}
			},
			verify: func(t *testing.T, snapshot SessionViewSnapshot) {
				t.Helper()

				if got := snapshot.Entries[0].Title; got != "Read README.md:5-12" {
					t.Fatalf("expected updated tool title to include line range, got %q", got)
				}
				if got := snapshot.Entries[0].Preview; got != "README.md:5-12" {
					t.Fatalf("expected updated tool preview to include line range, got %q", got)
				}
			},
		},
		{
			name: "merge null header input when tool call updates",
			setup: func(t *testing.T) []model.SessionUpdate {
				t.Helper()

				start := []model.ContentBlock{
					mustContentBlockTranscriptTest(t, model.ToolUseBlock{
						ID:    "tool-1",
						Name:  "Read",
						Input: []byte(`null`),
					}),
				}
				return []model.SessionUpdate{{
					Kind:          model.UpdateKindToolCallStarted,
					ToolCallID:    "tool-1",
					ToolCallState: model.ToolCallStatePending,
					Blocks:        start,
				}}
			},
			update: func(t *testing.T) model.SessionUpdate {
				t.Helper()

				update := []model.ContentBlock{
					mustContentBlockTranscriptTest(t, model.ToolUseBlock{
						ID:    "tool-1",
						Name:  "Read",
						Input: []byte(`{"file_path":"README.md"}`),
					}),
				}
				return model.SessionUpdate{
					Kind:          model.UpdateKindToolCallUpdated,
					ToolCallID:    "tool-1",
					ToolCallState: model.ToolCallStateInProgress,
					Blocks:        update,
				}
			},
			verify: func(t *testing.T, snapshot SessionViewSnapshot) {
				t.Helper()

				if got := snapshot.Entries[0].Title; got != "Read README.md" {
					t.Fatalf("expected merged tool title to use updated object input, got %q", got)
				}
				if got := snapshot.Entries[0].Preview; got != "README.md" {
					t.Fatalf("expected merged tool preview to use updated object input, got %q", got)
				}
			},
		},
		{
			name: "create a failure placeholder when tool call updates arrive before start",
			setup: func(t *testing.T) []model.SessionUpdate {
				t.Helper()
				return nil
			},
			update: func(t *testing.T) model.SessionUpdate {
				t.Helper()

				update := []model.ContentBlock{
					mustContentBlockTranscriptTest(t, model.ToolUseBlock{
						ID:       "tool-missing",
						Name:     "Read",
						Title:    "Read",
						Input:    []byte(`{"file_path":"README.md"}`),
						RawInput: []byte(`{"path":"README.md"}`),
					}),
					mustContentBlockTranscriptTest(t, model.ToolResultBlock{
						ToolUseID: "tool-missing",
						Content:   "loaded README.md",
					}),
				}
				return model.SessionUpdate{
					Kind:          model.UpdateKindToolCallUpdated,
					ToolCallID:    "tool-missing",
					ToolCallState: model.ToolCallStateCompleted,
					Blocks:        update,
				}
			},
			verify: func(t *testing.T, snapshot SessionViewSnapshot) {
				t.Helper()

				if len(snapshot.Entries) != 1 {
					t.Fatalf("expected one tool placeholder entry, got %#v", snapshot.Entries)
				}
				entry := snapshot.Entries[0]
				if entry.Kind != transcriptEntryToolCall {
					t.Fatalf("expected tool entry, got %#v", entry)
				}
				if entry.ToolCallState != model.ToolCallStateFailed {
					t.Fatalf("expected failed placeholder state, got %q", entry.ToolCallState)
				}
				if entry.Title != "Tool call not found" {
					t.Fatalf("expected missing-tool title, got %q", entry.Title)
				}
				if entry.Preview != "Tool call not found" {
					t.Fatalf("expected missing-tool preview, got %q", entry.Preview)
				}
			},
		},
		{
			name: "use display title instead of the programmatic tool name",
			setup: func(t *testing.T) []model.SessionUpdate {
				t.Helper()
				return nil
			},
			update: func(t *testing.T) model.SessionUpdate {
				t.Helper()

				start := []model.ContentBlock{
					mustContentBlockTranscriptTest(t, model.ToolUseBlock{
						ID:       "tool-1",
						Name:     "Read",
						Title:    "Read",
						ToolName: "read_file",
						Input:    []byte(`{"file_path":"README.md"}`),
						RawInput: []byte(`{"path":"README.md"}`),
					}),
				}
				return model.SessionUpdate{
					Kind:          model.UpdateKindToolCallStarted,
					ToolCallID:    "tool-1",
					ToolCallState: model.ToolCallStatePending,
					Blocks:        start,
				}
			},
			verify: func(t *testing.T, snapshot SessionViewSnapshot) {
				t.Helper()

				if got := snapshot.Entries[0].Title; got != "Read README.md" {
					t.Fatalf("expected visible title to use display label, got %q", got)
				}
				if got := snapshot.Entries[0].Preview; got != "README.md" {
					t.Fatalf("expected visible preview to use normalized input, got %q", got)
				}

				toolUse, err := snapshot.Entries[0].Blocks[0].AsToolUse()
				if err != nil {
					t.Fatalf("decode preserved tool header: %v", err)
				}
				if toolUse.ToolName != "read_file" {
					t.Fatalf("expected preserved programmatic tool name, got %q", toolUse.ToolName)
				}
			},
		},
		{
			name: "preserve prior tool output when a header-only update arrives",
			setup: func(t *testing.T) []model.SessionUpdate {
				t.Helper()

				start := []model.ContentBlock{
					mustContentBlockTranscriptTest(t, model.ToolUseBlock{
						ID:    "tool-keep",
						Name:  "Read",
						Input: []byte(`{"file_path":"README.md"}`),
					}),
				}
				withOutput := []model.ContentBlock{
					mustContentBlockTranscriptTest(t, model.ToolUseBlock{
						ID:    "tool-keep",
						Name:  "Read",
						Input: []byte(`{"file_path":"README.md"}`),
					}),
					mustContentBlockTranscriptTest(t, model.ToolResultBlock{
						ToolUseID: "tool-keep",
						Content:   "loaded README.md",
					}),
				}
				return []model.SessionUpdate{
					{
						Kind:          model.UpdateKindToolCallStarted,
						ToolCallID:    "tool-keep",
						ToolCallState: model.ToolCallStatePending,
						Blocks:        start,
					},
					{
						Kind:          model.UpdateKindToolCallUpdated,
						ToolCallID:    "tool-keep",
						ToolCallState: model.ToolCallStateInProgress,
						Blocks:        withOutput,
					},
				}
			},
			update: func(t *testing.T) model.SessionUpdate {
				t.Helper()

				headerOnly := []model.ContentBlock{
					mustContentBlockTranscriptTest(t, model.ToolUseBlock{
						ID:    "tool-keep",
						Name:  "Read",
						Input: []byte(`{"file_path":"README.md","start_line":5,"end_line":12}`),
					}),
				}
				return model.SessionUpdate{
					Kind:          model.UpdateKindToolCallUpdated,
					ToolCallID:    "tool-keep",
					ToolCallState: model.ToolCallStateCompleted,
					Blocks:        headerOnly,
				}
			},
			verify: func(t *testing.T, snapshot SessionViewSnapshot) {
				t.Helper()

				entry := snapshot.Entries[0]
				if got := entry.Title; got != "Read README.md:5-12" {
					t.Fatalf("expected header-only update to refresh tool title, got %q", got)
				}
				if got := entry.Preview; got != "README.md:5-12" {
					t.Fatalf("expected header-only update to refresh tool preview, got %q", got)
				}
				if len(entry.Blocks) != 2 {
					t.Fatalf("expected preserved tool output blocks, got %#v", entry.Blocks)
				}
				result, err := entry.Blocks[1].AsToolResult()
				if err != nil {
					t.Fatalf("decode preserved tool result block: %v", err)
				}
				if result.Content != "loaded README.md" {
					t.Fatalf("expected existing tool output to be preserved, got %q", result.Content)
				}
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("Should "+tc.name, func(t *testing.T) {
			t.Parallel()

			viewModel := newSessionViewModel()
			for index, setupUpdate := range tc.setup(t) {
				if _, changed := viewModel.Apply(setupUpdate); !changed {
					t.Fatalf("expected setup update %d to change session view", index)
				}
			}

			snapshot, changed := viewModel.Apply(tc.update(t))
			if !changed {
				t.Fatal("expected tool call scenario to change session view")
			}
			tc.verify(t, snapshot)
		})
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
