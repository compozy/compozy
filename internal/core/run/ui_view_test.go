package run

import (
	"fmt"
	"image/color"
	"strings"
	"testing"
	"unicode/utf8"

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

	m := newUIModel(1)

	if _, ok := m.viewport.Style.GetBackground().(lipgloss.NoColor); !ok {
		bg := m.viewport.Style.GetBackground()
		t.Fatalf("expected log viewport background to be inherited from container, got %v", bg)
	}
	if _, ok := m.sidebarViewport.Style.GetBackground().(lipgloss.NoColor); !ok {
		bg := m.sidebarViewport.Style.GetBackground()
		t.Fatalf("expected sidebar viewport background to be inherited from container, got %v", bg)
	}
}

func TestTitleBarOwnsBaseBackground(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t)

	assertRenderedCellsUseBackground(t, m.renderTitleBar(), colorBgBase)
}

func TestHelpOwnsBaseBackground(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t)

	assertRenderedCellsUseBackground(t, m.renderHelp(), colorBgBase)
}

func TestSidebarOwnsSurfaceBackground(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t)

	assertRenderedCellsUseBackground(t, m.renderSidebar(), colorBgSurface)
}

func TestMetaCardOwnsSurfaceBackground(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t)
	mainWidth, _ := m.mainDimensions()
	bodyWidth := mainWidth - mainHorizontalPadding

	assertRenderedCellsUseBackground(t, m.buildMetaCard(&m.jobs[m.selectedJob], bodyWidth), colorBgSurface)
}

func TestLogsHeaderOwnsBaseBackground(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t)
	mainWidth, _ := m.mainDimensions()
	bodyWidth := mainWidth - mainHorizontalPadding

	assertRenderedCellsUseBackground(t, m.renderLogsHeader(bodyWidth), colorBgBase)
}

func newPopulatedUIModelForTest(t *testing.T) *uiModel {
	t.Helper()

	outBuffer := newLineBuffer(0)
	errBuffer := newLineBuffer(0)
	m := newUIModel(1)
	m.handleJobQueued(&jobQueuedMsg{
		Index:     0,
		CodeFile:  "task_01",
		CodeFiles: []string{"task_01"},
		Issues:    1,
		SafeName:  "task_01-c05bd2",
		OutLog:    "task_01.log",
		ErrLog:    "task_01.err.log",
		OutBuffer: outBuffer,
		ErrBuffer: errBuffer,
	})
	m.handleJobStarted(jobStartedMsg{Index: 0})
	for _, line := range []string{
		`{"type":"thread.started","thread_id":"019d4a64-7477-77f2-8cc5-0009576de0d8"}`,
		`{"type":"turn.started"}`,
		`{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"Ledger Snapshot: Goal is task 1"}}`,
		`{"type":"item.started","item":{"id":"item_1","type":"command_execution","command":"pwd"}}`,
	} {
		outBuffer.appendLine(line)
	}
	errBuffer.appendLine("stderr line")
	m.handleJobLogUpdate(jobLogUpdateMsg{Index: 0})
	m.handleWindowSize(tea.WindowSizeMsg{Width: 120, Height: 30})

	return m
}

func sameColor(left, right color.Color) bool {
	lr, lg, lb, la := left.RGBA()
	rr, rg, rb, ra := right.RGBA()
	return lr == rr && lg == rg && lb == rb && la == ra
}

type ansiBackgroundState struct {
	set  bool
	code string
}

func assertRenderedCellsUseBackground(t *testing.T, rendered string, want color.Color) {
	t.Helper()

	wantCode := backgroundCode(want)
	var state ansiBackgroundState

	for i := 0; i < len(rendered); {
		if rendered[i] == '\x1b' {
			if i+1 >= len(rendered) || rendered[i+1] != '[' {
				t.Fatalf("unsupported ANSI escape at byte %d in %q", i, rendered)
			}
			seqEnd := strings.IndexByte(rendered[i:], 'm')
			if seqEnd < 0 {
				t.Fatalf("unterminated ANSI escape at byte %d in %q", i, rendered)
			}
			state.apply(rendered[i+2 : i+seqEnd])
			i += seqEnd + 1
			continue
		}

		r, size := utf8.DecodeRuneInString(rendered[i:])
		if r == utf8.RuneError && size == 1 {
			t.Fatalf("invalid UTF-8 rune at byte %d in %q", i, rendered)
		}
		if r == '\n' || r == '\r' {
			i += size
			continue
		}
		if !state.set || state.code != wantCode {
			t.Fatalf(
				"expected background %s for rune %q at byte %d, got %q in %q",
				wantCode,
				r,
				i,
				state.code,
				rendered,
			)
		}
		i += size
	}
}

func backgroundCode(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("48;2;%d;%d;%d", r>>8, g>>8, b>>8)
}

func (s *ansiBackgroundState) apply(seq string) {
	if seq == "" {
		s.set = false
		s.code = ""
		return
	}

	params := strings.Split(seq, ";")
	for i := 0; i < len(params); i++ {
		if params[i] == "" {
			params[i] = "0"
		}

		switch params[i] {
		case "0":
			s.set = false
			s.code = ""
		case "49":
			s.set = false
			s.code = ""
		case "48":
			if i+1 >= len(params) {
				continue
			}
			switch params[i+1] {
			case "2":
				if i+4 >= len(params) {
					continue
				}
				s.set = true
				s.code = strings.Join(params[i:i+5], ";")
				i += 4
			case "5":
				if i+2 >= len(params) {
					continue
				}
				s.set = true
				s.code = strings.Join(params[i:i+3], ";")
				i += 2
			}
		}
	}
}
