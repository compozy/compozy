package run

import (
	"context"
	"image/color"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func TestRenderSidebarFitsContentHeight(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t)

	if got, want := lipgloss.Height(m.renderSidebar()), m.contentHeight; got != want {
		t.Fatalf("expected sidebar height %d, got %d", want, got)
	}
}

func TestBuildMetaCardFitsMainBodyWidth(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t)

	mainWidth, _ := m.mainDimensions()
	bodyWidth := mainWidth - mainHorizontalPadding
	metaCard := m.buildMetaCard(&m.jobs[m.selectedJob], bodyWidth)

	if got := lipgloss.Width(metaCard); got != bodyWidth {
		t.Fatalf("expected meta card width %d to fit main body width %d", got, bodyWidth)
	}
}

func TestJobsViewFitsWindowHeight(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t)

	if got, want := lipgloss.Height(m.View().Content), m.height; got != want {
		t.Fatalf("expected jobs view height %d, got %d", want, got)
	}
}

func TestJobsViewFitsSmallWindowHeight(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t)
	m.handleWindowSize(tea.WindowSizeMsg{Width: 80, Height: 24})

	if got, want := lipgloss.Height(m.View().Content), m.height; got != want {
		t.Fatalf("expected small jobs view height %d, got %d", want, got)
	}
}

func TestJobsViewUsesCompozyChrome(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t)
	view := m.View().Content

	for _, want := range []string{"AGENT LOOP", "SYS.PIPELINE", "SYS.STDOUT // LIVE LOGS"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected jobs view to contain %q", want)
		}
	}
	if strings.ContainsAny(view, "╭╮╰╯") {
		t.Fatalf("expected jobs view to avoid rounded border glyphs: %q", view)
	}
}

func TestSelectedSidebarItemBackgroundFillsRowWidth(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t)
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

func TestSummaryPanelsUseTechnicalChrome(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t)
	m.aggregateUsage.Add(TokenUsage{InputTokens: 42, OutputTokens: 11})

	boxes := []string{
		m.renderSummaryMainBox(60),
		m.renderSummaryTokenBox(60),
	}

	for _, box := range boxes {
		if !strings.Contains(box, "┌") {
			t.Fatalf("expected square border in summary box: %q", box)
		}
		if strings.ContainsAny(box, "╭╮╰╯") {
			t.Fatalf("expected summary box to avoid rounded border glyphs: %q", box)
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

	m := newUIModel(context.Background(), 1)

	if _, ok := m.viewport.Style.GetBackground().(lipgloss.NoColor); !ok {
		bg := m.viewport.Style.GetBackground()
		t.Fatalf("expected log viewport background to be inherited from container, got %v", bg)
	}
	if _, ok := m.sidebarViewport.Style.GetBackground().(lipgloss.NoColor); !ok {
		bg := m.sidebarViewport.Style.GetBackground()
		t.Fatalf("expected sidebar viewport background to be inherited from container, got %v", bg)
	}
}

func newPopulatedUIModelForTest(t *testing.T) *uiModel {
	t.Helper()

	m := newUIModel(context.Background(), 1)
	m.handleJobQueued(&jobQueuedMsg{
		Index:     0,
		CodeFile:  "task_01",
		CodeFiles: []string{"task_01"},
		Issues:    1,
		SafeName:  "task_01-c05bd2",
		OutLog:    "task_01.log",
		ErrLog:    "task_01.err.log",
	})
	m.handleJobStarted(jobStartedMsg{Index: 0})
	m.handleJobLogUpdate(jobLogUpdateMsg{
		Index: 0,
		Out: []string{
			`{"type":"thread.started","thread_id":"019d4a64-7477-77f2-8cc5-0009576de0d8"}`,
			`{"type":"turn.started"}`,
			`{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"Ledger Snapshot: Goal is task 1"}}`,
			`{"type":"item.started","item":{"id":"item_1","type":"command_execution","command":"pwd"}}`,
		},
		Err: []string{
			"stderr line",
		},
	})
	m.handleWindowSize(tea.WindowSizeMsg{Width: 120, Height: 30})

	return m
}

func sameColor(left, right color.Color) bool {
	lr, lg, lb, la := left.RGBA()
	rr, rg, rb, ra := right.RGBA()
	return lr == rr && lg == rg && lb == rb && la == ra
}
