package run

import (
	"fmt"
	"image/color"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/core/model"

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
	m.aggregateUsage.Add(model.Usage{InputTokens: 42, OutputTokens: 11})

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

func TestRenderSummaryViewIncludesFailuresAndHelp(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t)
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

func TestBuildViewportContentRendersTypedBlocks(t *testing.T) {
	t.Parallel()

	t.Run("text block", func(t *testing.T) {
		t.Parallel()

		m := newViewportRenderModelForTest(t)
		m.handleJobUpdate(jobUpdateMsg{
			Index: 0,
			Blocks: []model.ContentBlock{
				mustContentBlockViewTest(t, model.TextBlock{Text: "hello from ACP"}),
			},
		})

		content, hasContent := m.buildViewportContent(&m.jobs[0], 120)
		if !hasContent {
			t.Fatal("expected text content")
		}
		if !strings.Contains(content, "hello from ACP") {
			t.Fatalf("expected text block rendering, got %q", content)
		}
	})

	t.Run("diff block", func(t *testing.T) {
		t.Parallel()

		m := newViewportRenderModelForTest(t)
		m.handleJobUpdate(jobUpdateMsg{
			Index: 0,
			Blocks: []model.ContentBlock{
				mustContentBlockViewTest(t, model.DiffBlock{
					FilePath: "README.md",
					Diff:     "@@ -1 +1 @@\n-old\n+new",
					NewText:  "new",
				}),
			},
		})

		content, _ := m.buildViewportContent(&m.jobs[0], 120)
		for _, want := range []string{"DIFF README.md", "@@ -1 +1 @@", "-old", "+new"} {
			if !strings.Contains(content, want) {
				t.Fatalf("expected diff rendering to contain %q, got %q", want, content)
			}
		}
	})

	t.Run("tool use block", func(t *testing.T) {
		t.Parallel()

		m := newViewportRenderModelForTest(t)
		m.handleJobUpdate(jobUpdateMsg{
			Index: 0,
			Blocks: []model.ContentBlock{
				mustContentBlockViewTest(t, model.ToolUseBlock{
					ID:    "tool-1",
					Name:  "read_file",
					Input: []byte(`{"path":"README.md"}`),
				}),
			},
		})

		content, _ := m.buildViewportContent(&m.jobs[0], 120)
		for _, want := range []string{"TOOL read_file", "tool-1", "\"path\": \"README.md\""} {
			if !strings.Contains(content, want) {
				t.Fatalf("expected tool_use rendering to contain %q, got %q", want, content)
			}
		}
	})

	t.Run("terminal output block", func(t *testing.T) {
		t.Parallel()

		m := newViewportRenderModelForTest(t)
		m.handleJobUpdate(jobUpdateMsg{
			Index: 0,
			Blocks: []model.ContentBlock{
				mustContentBlockViewTest(t, model.TerminalOutputBlock{
					Command:  "go test ./...",
					Output:   "ok\tgithub.com/compozy/compozy/internal/core/run",
					ExitCode: 1,
				}),
			},
		})

		content, _ := m.buildViewportContent(&m.jobs[0], 120)
		for _, want := range []string{"TERMINAL OUTPUT", "$ go test ./...", "github.com/compozy/compozy/internal/core/run", "exit code 1"} {
			if !strings.Contains(content, want) {
				t.Fatalf("expected terminal output rendering to contain %q, got %q", want, content)
			}
		}
	})

	t.Run("unknown block fallback", func(t *testing.T) {
		t.Parallel()

		m := newViewportRenderModelForTest(t)
		m.handleJobUpdate(jobUpdateMsg{
			Index: 0,
			Blocks: []model.ContentBlock{{
				Type: model.ContentBlockType("custom_block"),
				Data: []byte(`{"type":"custom_block","payload":{"foo":"bar"}}`),
			}},
		})

		content, _ := m.buildViewportContent(&m.jobs[0], 120)
		for _, want := range []string{"RAW BLOCK custom_block", "\"type\": \"custom_block\"", "\"foo\": \"bar\""} {
			if !strings.Contains(content, want) {
				t.Fatalf("expected unknown block fallback to contain %q, got %q", want, content)
			}
		}
	})

	t.Run("mixed block order", func(t *testing.T) {
		t.Parallel()

		m := newViewportRenderModelForTest(t)
		m.handleJobUpdate(jobUpdateMsg{Index: 0, Blocks: mixedViewportBlocksForTest(t)})

		content, _ := m.buildViewportContent(&m.jobs[0], 120)
		markers := []string{
			"hello from ACP",
			"DIFF README.md",
			"TOOL read_file",
			"TOOL RESULT tool-1",
			"TERMINAL OUTPUT",
			"IMAGE image/png inline",
			"RAW BLOCK custom_block",
		}
		lastIndex := -1
		for _, marker := range markers {
			idx := strings.Index(content, marker)
			if idx < 0 {
				t.Fatalf("expected mixed rendering to contain %q, got %q", marker, content)
			}
			if idx <= lastIndex {
				t.Fatalf("expected marker %q after prior markers in %q", marker, content)
			}
			lastIndex = idx
		}
	})
}

func TestRenderSummaryTokenBoxDisplaysAllUsageFields(t *testing.T) {
	t.Parallel()

	m := newUIModel(1)
	m.aggregateUsage = &model.Usage{
		InputTokens:  13,
		OutputTokens: 21,
		TotalTokens:  34,
		CacheReads:   5,
		CacheWrites:  2,
	}

	box := m.renderSummaryTokenBox(60)
	for _, want := range []string{"INPUT", "13", "OUTPUT", "21", "CACHER", "5", "CACHEW", "2", "TOTAL", "34"} {
		if !strings.Contains(box, want) {
			t.Fatalf("expected summary token box to contain %q, got %q", want, box)
		}
	}
}

func TestBuildMetaCardDisplaysUsageCounts(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t)
	m.jobs[0].tokenUsage = &model.Usage{
		InputTokens:  8,
		OutputTokens: 12,
		TotalTokens:  20,
		CacheReads:   3,
		CacheWrites:  1,
	}

	mainWidth, _ := m.mainDimensions()
	bodyWidth := mainWidth - mainHorizontalPadding
	card := m.buildMetaCard(&m.jobs[0], bodyWidth)
	for _, want := range []string{"TOKENS", "8 IN / 12 OUT / 20 TOTAL", "CACHE", "3 READ / 1 WRITE"} {
		if !strings.Contains(card, want) {
			t.Fatalf("expected meta card to contain %q, got %q", want, card)
		}
	}
}

func TestUIMessageFlowRendersMixedContentBlocks(t *testing.T) {
	t.Parallel()

	m := newViewportRenderModelForTest(t)
	m.handleJobStarted(jobStartedMsg{Index: 0})
	m.handleJobUpdate(jobUpdateMsg{Index: 0, Blocks: mixedViewportBlocksForTest(t)})

	view := m.View().Content
	for _, want := range []string{"hello from ACP", "DIFF README.md", "TOOL read_file", "TERMINAL OUTPUT", "IMAGE image/png inline"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected jobs view to contain %q, got %q", want, view)
		}
	}
}

func TestSummaryViewUsesUsageAfterCompletion(t *testing.T) {
	t.Parallel()

	m := newViewportRenderModelForTest(t)
	m.handleJobStarted(jobStartedMsg{Index: 0})
	m.handleUsageUpdate(usageUpdateMsg{
		Index: 0,
		Usage: model.Usage{
			InputTokens:  11,
			OutputTokens: 9,
			TotalTokens:  20,
			CacheReads:   4,
			CacheWrites:  2,
		},
	})
	m.handleJobFinished(jobFinishedMsg{Index: 0, Success: true})
	m.showSummaryView()

	view := m.renderSummaryView().Content
	for _, want := range []string{"usage.tokens", "11", "9", "4", "2", "20"} {
		if !strings.Contains(strings.ToLower(view), strings.ToLower(want)) {
			t.Fatalf("expected summary view to contain %q, got %q", want, view)
		}
	}
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
		appendJobTextUpdateViewTest(t, m, 0, line)
	}
	errBlock, err := model.NewContentBlock(model.ToolResultBlock{
		ToolUseID: "session",
		Content:   "stderr line",
		IsError:   true,
	})
	if err != nil {
		t.Fatalf("new err content block: %v", err)
	}
	m.handleJobUpdate(jobUpdateMsg{Index: 0, Blocks: []model.ContentBlock{errBlock}})
	m.handleWindowSize(tea.WindowSizeMsg{Width: 120, Height: 30})

	return m
}

func newViewportRenderModelForTest(t *testing.T) *uiModel {
	t.Helper()

	m := newUIModel(1)
	m.handleJobQueued(&jobQueuedMsg{
		Index:     0,
		CodeFile:  "task_01",
		CodeFiles: []string{"task_01"},
		Issues:    1,
		SafeName:  "task_01-render",
		OutLog:    "task_01.out.log",
		ErrLog:    "task_01.err.log",
		OutBuffer: newLineBuffer(0),
		ErrBuffer: newLineBuffer(0),
	})
	m.handleWindowSize(tea.WindowSizeMsg{Width: 120, Height: 80})
	return m
}

func mixedViewportBlocksForTest(t *testing.T) []model.ContentBlock {
	t.Helper()

	return []model.ContentBlock{
		mustContentBlockViewTest(t, model.TextBlock{Text: "hello from ACP"}),
		mustContentBlockViewTest(t, model.DiffBlock{
			FilePath: "README.md",
			Diff:     "@@ -1 +1 @@\n-old\n+new",
			NewText:  "new",
		}),
		mustContentBlockViewTest(t, model.ToolUseBlock{
			ID:    "tool-1",
			Name:  "read_file",
			Input: []byte(`{"path":"README.md"}`),
		}),
		mustContentBlockViewTest(t, model.ToolResultBlock{
			ToolUseID: "tool-1",
			Content:   "loaded README.md",
		}),
		mustContentBlockViewTest(t, model.TerminalOutputBlock{
			Command:  "go test ./...",
			Output:   "ok\tgithub.com/compozy/compozy/internal/core/run",
			ExitCode: 0,
		}),
		mustContentBlockViewTest(t, model.ImageBlock{
			Data:     "ZGF0YQ==",
			MimeType: "image/png",
		}),
		{
			Type: model.ContentBlockType("custom_block"),
			Data: []byte(`{"type":"custom_block","payload":{"foo":"bar"}}`),
		},
	}
}

func mustContentBlockViewTest(t *testing.T, payload any) model.ContentBlock {
	t.Helper()

	block, err := model.NewContentBlock(payload)
	if err != nil {
		t.Fatalf("new content block: %v", err)
	}
	return block
}

func appendJobTextUpdateViewTest(t *testing.T, m *uiModel, index int, text string) {
	t.Helper()

	block, err := model.NewContentBlock(model.TextBlock{Text: text})
	if err != nil {
		t.Fatalf("new content block: %v", err)
	}
	m.handleJobUpdate(jobUpdateMsg{Index: index, Blocks: []model.ContentBlock{block}})
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
