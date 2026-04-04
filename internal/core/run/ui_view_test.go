package run

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	xansi "github.com/charmbracelet/x/ansi"
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

func TestRetryingJobRendersAttemptMetadata(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.handleJobRetry(jobRetryMsg{
		Index:       0,
		Attempt:     2,
		MaxAttempts: 3,
		Reason:      "temporary setup failure",
	})

	row := m.renderSidebarItem(&m.jobs[0], true)
	for _, want := range []string{"RETRY", "ATTEMPT 2/3"} {
		if !strings.Contains(row, want) {
			t.Fatalf("expected retry sidebar row to contain %q, got %q", want, row)
		}
	}

	meta := m.timelineMeta(&m.jobs[0])
	for _, want := range []string{"attempt 2/3", "retrying: temporary setup failure"} {
		if !strings.Contains(meta, want) {
			t.Fatalf("expected retry timeline meta to contain %q, got %q", want, meta)
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

func TestHelpOwnsBaseBackground(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 120, Height: 30})

	assertRenderedCellsUseBackground(t, m.renderHelp(), colorBgBase)
}

func TestTimelineContentOwnsSurfaceBackgroundAcrossWrappedRows(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 120, Height: 30})
	longAssistant := TranscriptEntry{
		ID:    "assistant-wrapped",
		Kind:  transcriptEntryAssistantMessage,
		Title: "Assistant",
		Blocks: []model.ContentBlock{
			mustContentBlockUITest(
				t,
				model.TextBlock{Text: narrativeWrapText("assistant")},
			),
		},
	}
	runtime := TranscriptEntry{
		ID:    "runtime-1",
		Kind:  transcriptEntryRuntimeNotice,
		Title: "Runtime",
		Blocks: []model.ContentBlock{
			mustContentBlockUITest(t, model.TextBlock{Text: "syncing transcript state"}),
		},
	}
	m.handleJobUpdate(jobUpdateMsg{Index: 0, Snapshot: buildSnapshotWithEntries(t, longAssistant, runtime)})
	m.jobs[0].expandedEntryIDs = map[string]bool{
		longAssistant.ID: true,
		runtime.ID:       true,
	}
	m.jobs[0].expansionRevision++

	const width = 72
	rendered := m.buildTimelineContent(&m.jobs[0], width)
	assertRenderedCellsUseBackground(t, rendered.content, colorBgSurface)
	assertRenderedLinesFitWidth(t, rendered.content, width)
	if !strings.Contains(xansi.Strip(rendered.content), "tail-marker") {
		t.Fatalf("expected wrapped narrative tail to remain visible, got %q", xansi.Strip(rendered.content))
	}
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

func TestNarrativeEntriesWrapWhenExpanded(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		entry TranscriptEntry
	}{
		{
			name: "assistant",
			entry: TranscriptEntry{
				ID:    "assistant-wrap",
				Kind:  transcriptEntryAssistantMessage,
				Title: "Assistant",
				Blocks: []model.ContentBlock{
					mustContentBlockUITest(t, model.TextBlock{Text: narrativeWrapText("assistant")}),
				},
			},
		},
		{
			name: "thinking",
			entry: TranscriptEntry{
				ID:    "thinking-wrap",
				Kind:  transcriptEntryAssistantThinking,
				Title: "Thinking",
				Blocks: []model.ContentBlock{
					mustContentBlockUITest(t, model.TextBlock{Text: narrativeWrapText("thinking")}),
				},
			},
		},
		{
			name: "runtime",
			entry: TranscriptEntry{
				ID:    "runtime-wrap",
				Kind:  transcriptEntryRuntimeNotice,
				Title: "Runtime",
				Blocks: []model.ContentBlock{
					mustContentBlockUITest(t, model.TextBlock{Text: narrativeWrapText("runtime")}),
				},
			},
		},
		{
			name: "stderr",
			entry: TranscriptEntry{
				ID:    "stderr-wrap",
				Kind:  transcriptEntryStderrEvent,
				Title: "stderr",
				Blocks: []model.ContentBlock{
					mustContentBlockUITest(t, model.TextBlock{Text: narrativeWrapText("stderr")}),
				},
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 120, Height: 30})
			m.handleJobUpdate(jobUpdateMsg{Index: 0, Snapshot: buildSnapshotWithEntries(t, tc.entry)})
			m.jobs[0].selectedEntry = 0
			m.jobs[0].expandedEntryIDs = map[string]bool{tc.entry.ID: true}
			m.jobs[0].expansionRevision++

			const width = 28
			rendered := m.buildTimelineContent(&m.jobs[0], width)
			stripped := xansi.Strip(rendered.content)
			if !strings.Contains(stripped, "tail-marker") {
				t.Fatalf("expected wrapped narrative to keep tail marker visible, got %q", stripped)
			}
			if strings.Contains(stripped, "…") {
				t.Fatalf("expected wrapped narrative to avoid truncation ellipsis, got %q", stripped)
			}
			if got := strings.Count(stripped, "\n"); got < 4 {
				t.Fatalf("expected wrapped narrative to span multiple rows, got %d newlines in %q", got, stripped)
			}
			assertRenderedLinesFitWidth(t, rendered.content, width)
		})
	}
}

func TestCompactTimelineDetailsRemainTruncated(t *testing.T) {
	t.Parallel()

	entry := TranscriptEntry{
		ID:            "tool-compact",
		Kind:          transcriptEntryToolCall,
		Title:         "run_tests",
		ToolCallID:    "tool-compact",
		ToolCallState: model.ToolCallStateCompleted,
		Blocks: []model.ContentBlock{
			mustContentBlockUITest(t, model.ToolUseBlock{ID: "tool-compact", Name: "run_tests"}),
			mustContentBlockUITest(
				t,
				model.ToolResultBlock{ToolUseID: "tool-compact", Content: compactTruncationText()},
			),
		},
	}

	m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.handleJobUpdate(jobUpdateMsg{Index: 0, Snapshot: buildSnapshotWithEntries(t, entry)})
	m.jobs[0].selectedEntry = 0
	m.jobs[0].expandedEntryIDs = map[string]bool{entry.ID: true}
	m.jobs[0].expansionRevision++

	const width = 28
	rendered := m.buildTimelineContent(&m.jobs[0], width)
	stripped := xansi.Strip(rendered.content)
	if !strings.Contains(stripped, "…") {
		t.Fatalf("expected compact tool details to stay truncated, got %q", stripped)
	}
	if strings.Contains(stripped, "tail-marker") {
		t.Fatalf("expected truncated compact details to hide tail marker, got %q", stripped)
	}
	assertRenderedLinesFitWidth(t, rendered.content, width)
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
	if strings.TrimSpace(xansi.Strip(content)) == "" {
		t.Fatal("expected rendered content")
	}

	wantColor := normalizedColor(want)
	var current *color.RGBA

	for i := 0; i < len(content); {
		switch content[i] {
		case '\x1b':
			if next, bg, ok := parseANSIBackground(content, i); ok {
				i = next
				current = bg
				continue
			}
		case '\n', '\r':
			i++
			continue
		}

		r, size := utf8.DecodeRuneInString(content[i:])
		if r == utf8.RuneError && size == 0 {
			t.Fatalf("failed to decode content at byte %d", i)
		}
		if xansi.StringWidth(string(r)) > 0 && runeNeedsBackgroundCheck(r) {
			if current == nil {
				t.Fatalf("expected background %s on visible rune %q in %q", colorLabel(wantColor), r, content)
			}
			if !sameColor(*current, wantColor) {
				t.Fatalf(
					"expected background %s on visible rune %q, got %s in %q",
					colorLabel(wantColor),
					r,
					colorLabel(*current),
					content,
				)
			}
		}
		i += size
	}
}

func assertRenderedLinesFitWidth(t *testing.T, content string, want int) {
	t.Helper()

	for idx, line := range strings.Split(content, "\n") {
		if got := xansi.StringWidth(line); got != want {
			t.Fatalf("expected rendered line %d width %d, got %d: %q", idx, want, got, xansi.Strip(line))
		}
	}
}

func sameColor(left, right color.Color) bool {
	lr, lg, lb, la := left.RGBA()
	rr, rg, rb, ra := right.RGBA()
	return lr == rr && lg == rg && lb == rb && la == ra
}

func normalizedColor(c color.Color) color.RGBA {
	r, g, b, a := c.RGBA()
	return color.RGBA{
		R: uint8(r >> 8),
		G: uint8(g >> 8),
		B: uint8(b >> 8),
		A: uint8(a >> 8),
	}
}

func parseANSIBackground(content string, start int) (next int, bg *color.RGBA, ok bool) {
	if start+1 >= len(content) || content[start] != '\x1b' || content[start+1] != '[' {
		return start, nil, false
	}

	end := start + 2
	for end < len(content) && content[end] != 'm' {
		end++
	}
	if end >= len(content) || content[end] != 'm' {
		return start, nil, false
	}

	params := []int{0}
	if raw := content[start+2 : end]; raw != "" {
		parts := strings.Split(raw, ";")
		params = make([]int, 0, len(parts))
		for _, part := range parts {
			if part == "" {
				params = append(params, 0)
				continue
			}
			value, err := strconv.Atoi(part)
			if err != nil {
				return end + 1, bg, true
			}
			params = append(params, value)
		}
	}

	var current *color.RGBA
	for idx := 0; idx < len(params); idx++ {
		switch params[idx] {
		case 0, 49:
			current = nil
		case 48:
			if idx+1 >= len(params) {
				continue
			}
			switch params[idx+1] {
			case 2:
				if idx+4 >= len(params) {
					idx = len(params)
					continue
				}
				current = &color.RGBA{
					R: uint8(params[idx+2]),
					G: uint8(params[idx+3]),
					B: uint8(params[idx+4]),
					A: 0xff,
				}
				idx += 4
			case 5:
				// Indexed colors are not used in these regressions; treat them as "set but unknown".
				current = nil
				idx += 2
			}
		case 38:
			if idx+1 >= len(params) {
				continue
			}
			switch params[idx+1] {
			case 2:
				idx += 4
			case 5:
				idx += 2
			}
		}
	}

	return end + 1, current, true
}

func colorLabel(c color.RGBA) string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

func runeNeedsBackgroundCheck(r rune) bool {
	return r == ' ' || r == '░'
}

func narrativeWrapText(kind string) string {
	return fmt.Sprintf(
		"%s alpha bravo charlie delta echo foxtrot gulf hotel india juliet kilo tail-marker",
		kind,
	)
}

func compactTruncationText() string {
	return "tool output alpha bravo charlie delta echo foxtrot gulf hotel india juliet kilo tail-marker"
}
