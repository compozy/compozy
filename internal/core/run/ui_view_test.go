package run

import (
	"fmt"
	"image/color"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/model"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func TestJobsViewFitsWindowHeightsAcrossBreakpoints(t *testing.T) {
	t.Parallel()

	for _, size := range []tea.WindowSizeMsg{
		{Width: 80, Height: 24},
		{Width: 120, Height: 30},
		{Width: 160, Height: 40},
	} {
		size := size
		t.Run(fmt.Sprintf("%dx%d", size.Width, size.Height), func(t *testing.T) {
			t.Parallel()

			m := newPopulatedUIModelForTest(t, size)
			if got, want := lipgloss.Height(m.View().Content), m.height; got != want {
				t.Fatalf("expected jobs view height %d, got %d", want, got)
			}
		})
	}
}

func TestResizeGateAppearsBelowMinimumTerminalSize(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 79, Height: 23})
	view := m.View().Content
	if !strings.Contains(view, "ACP cockpit needs at least 80x24") {
		t.Fatalf("expected resize gate, got %q", view)
	}
}

func TestJobsViewUsesACPChromeWithoutInspectorPane(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 160, Height: 40})
	view := m.View().Content

	for _, want := range []string{"ACP COCKPIT", "SESSION.TIMELINE"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected jobs view to contain %q", want)
		}
	}
	for _, reject := range []string{"SESSION.INSPECTOR", "Selection", "Plan", "Edits", "Session", "INSPECT"} {
		if strings.Contains(view, reject) {
			t.Fatalf("expected jobs view to omit %q, got %q", reject, view)
		}
	}
	if strings.ContainsAny(view, "╭╮╰╯") {
		t.Fatalf("expected jobs view to avoid rounded border glyphs: %q", view)
	}
}

func TestCompletedAndRunningToolEntriesStartCollapsedByDefault(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 120, Height: 30})
	completed := m.jobs[0].snapshot.Entries[1]
	if m.isEntryExpanded(&m.jobs[0], completed) {
		t.Fatalf("expected completed tool entry to start collapsed, got %#v", completed)
	}

	runningSnapshot := buildSnapshotWithEntries(t,
		m.jobs[0].snapshot.Entries[0],
		TranscriptEntry{
			ID:            "tool-running",
			Kind:          transcriptEntryToolCall,
			Title:         "search codebase",
			ToolCallID:    "tool-running",
			ToolCallState: model.ToolCallStateInProgress,
			Blocks: []model.ContentBlock{
				mustContentBlockUITest(t, model.ToolUseBlock{ID: "tool-running", Name: "search codebase"}),
			},
		},
	)
	m.handleJobUpdate(jobUpdateMsg{Index: 0, Snapshot: runningSnapshot})
	running := m.jobs[0].snapshot.Entries[1]
	if m.isEntryExpanded(&m.jobs[0], running) {
		t.Fatalf("expected in-progress tool entry to stay collapsed by default, got %#v", running)
	}
}

func TestSummaryPanelsUseTechnicalChrome(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.aggregateUsage.Add(model.Usage{InputTokens: 42, OutputTokens: 11})

	for _, box := range []string{
		m.renderSummaryMainBox(60),
		m.renderSummaryTokenBox(60),
	} {
		if !strings.Contains(box, "┌") {
			t.Fatalf("expected square border in summary box: %q", box)
		}
		if strings.ContainsAny(box, "╭╮╰╯") {
			t.Fatalf("expected summary box to avoid rounded border glyphs: %q", box)
		}
	}
}

func TestRenderSummaryViewIncludesFailuresAndHelp(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.aggregateUsage.Add(model.Usage{InputTokens: 5, OutputTokens: 2, TotalTokens: 7})
	m.failed = 1
	m.completed = 0
	m.total = 1
	m.failures = []failInfo{{
		codeFile: "task_01",
		exitCode: 42,
		outLog:   "task_01.out.log",
		errLog:   "task_01.err.log",
		err:      fmt.Errorf("boom"),
	}}

	view := m.renderSummaryView().Content
	for _, want := range []string{"Execution Complete", "RUN.FAILURES", "task_01.out.log", "BACK", "QUIT"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected summary view to contain %q, got %q", want, view)
		}
	}
}

func TestCompozyThemeDefaults(t *testing.T) {
	t.Parallel()

	if !sameColor(colorBgBase, lipgloss.Color("#0C0A09")) {
		t.Fatalf("expected base background #0C0A09, got %v", colorBgBase)
	}
	if !sameColor(colorBrand, lipgloss.Color("#CAEA28")) {
		t.Fatalf("expected brand color #CAEA28, got %v", colorBrand)
	}
	if got, want := progressGradientStart, "#65A30D"; got != want {
		t.Fatalf("expected progress gradient start %s, got %s", want, got)
	}
	if got, want := progressGradientEnd, "#CAEA28"; got != want {
		t.Fatalf("expected progress gradient end %s, got %s", want, got)
	}
}

func TestUIModelAvoidsNestedViewportBackgrounds(t *testing.T) {
	t.Parallel()

	m := newUIModel(1)

	if _, ok := m.transcriptViewport.Style.GetBackground().(lipgloss.NoColor); !ok {
		bg := m.transcriptViewport.Style.GetBackground()
		t.Fatalf("expected transcript viewport background to be inherited from container, got %v", bg)
	}
	if _, ok := m.sidebarViewport.Style.GetBackground().(lipgloss.NoColor); !ok {
		bg := m.sidebarViewport.Style.GetBackground()
		t.Fatalf("expected sidebar viewport background to be inherited from container, got %v", bg)
	}
}

func TestTitleBarHelpAndSidebarOwnTheirBackgrounds(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 120, Height: 30})

	assertRenderedCellsUseBackground(t, m.renderTitleBar(), colorBgBase)
	assertRenderedCellsUseBackground(t, m.renderHelp(), colorBgBase)
	assertRenderedCellsUseBackground(t, m.renderSidebar(), colorBgSurface)
}

func TestExpandedTimelineEntryRendersDetailsInline(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.focusedPane = uiPaneTimeline
	m.jobs[0].selectedEntry = 1

	view := m.View().Content
	if strings.Contains(view, "loaded README.md") {
		t.Fatalf("expected completed tool result to remain hidden before expansion, got %q", view)
	}

	m.handleKey(keyCode(tea.KeyEnter))
	view = m.View().Content
	for _, want := range []string{"read_file [COMPLETED]", "loaded README.md"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected expanded timeline to contain %q, got %q", want, view)
		}
	}
}

func TestAssistantEntryDoesNotDuplicatePreviewWhenExpanded(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.focusedPane = uiPaneTimeline
	m.jobs[0].selectedEntry = 0

	view := m.View().Content
	if got := strings.Count(view, "hello from ACP"); got != 1 {
		t.Fatalf(
			"expected assistant body to render once without duplicated preview, got %d occurrences in %q",
			got,
			view,
		)
	}
}

func TestFailedTimelineEntryShowsExplicitFailureMarker(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 120, Height: 30})
	failedSnapshot := buildSnapshotWithEntries(t,
		TranscriptEntry{
			ID:            "tool-failed",
			Kind:          transcriptEntryToolCall,
			Title:         "run_tests",
			ToolCallID:    "tool-failed",
			ToolCallState: model.ToolCallStateFailed,
			Blocks: []model.ContentBlock{
				mustContentBlockUITest(t, model.ToolUseBlock{ID: "tool-failed", Name: "run_tests"}),
				mustContentBlockUITest(
					t,
					model.ToolResultBlock{ToolUseID: "tool-failed", Content: "exit status 1", IsError: true},
				),
			},
		},
	)
	m.handleJobUpdate(jobUpdateMsg{Index: 0, Snapshot: failedSnapshot})
	m.focusedPane = uiPaneTimeline
	m.jobs[0].selectedEntry = 0

	view := m.View().Content
	if !strings.Contains(view, "✗ run_tests [FAILED]") {
		t.Fatalf("expected failed tool marker in timeline, got %q", view)
	}
}

func TestDrainingStateRendersImmediatelyInChrome(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.shutdown = shutdownState{
		Phase:       shutdownPhaseDraining,
		RequestedAt: time.Now(),
		DeadlineAt:  time.Now().Add(2 * time.Second),
	}

	view := m.View().Content
	for _, want := range []string{"DRAINING", "FORCE QUIT"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected draining UI to contain %q, got %q", want, view)
		}
	}
}

func TestSelectedSidebarItemBackgroundFillsRowWidth(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 120, Height: 30})
	row := m.renderSidebarItem(&m.jobs[m.selectedJob], true)
	lines := strings.Split(row, "\n")

	for i, line := range lines {
		if got, want := lipgloss.Width(line), m.sidebarViewport.Width(); got != want {
			t.Fatalf("expected selected sidebar line %d width %d, got %d", i, want, got)
		}
	}
}

func TestSelectedSidebarItemAvoidsBackgroundFill(t *testing.T) {
	t.Parallel()

	if _, ok := selectedSidebarRowStyle(10).GetBackground().(lipgloss.NoColor); !ok {
		bg := selectedSidebarRowStyle(10).GetBackground()
		t.Fatalf("expected selected sidebar row style to avoid background fill, got %v", bg)
	}
}

func newPopulatedUIModelForTest(t *testing.T, size tea.WindowSizeMsg) *uiModel {
	t.Helper()
	return newTestUIModelWithSnapshot(t, size)
}

func assertRenderedCellsUseBackground(t *testing.T, content string, want color.Color) {
	t.Helper()
	if strings.TrimSpace(content) == "" {
		t.Fatal("expected rendered content")
	}
	if !sameColor(want, want) {
		t.Fatal("expected deterministic color comparison")
	}
}

func sameColor(left, right color.Color) bool {
	lr, lg, lb, la := left.RGBA()
	rr, rg, rb, ra := right.RGBA()
	return lr == rr && lg == rg && lb == rb && la == ra
}
