package ui

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"
	"testing"
	"time"

	xansi "github.com/charmbracelet/x/ansi"
	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/core/model"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"

	"charm.land/bubbles/v2/viewport"
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
		t.Run(fmt.Sprintf("Should fit jobs view at %dx%d", size.Width, size.Height), func(t *testing.T) {
			t.Parallel()

			m := newPopulatedUIModelForTest(t, size)
			if got, want := lipgloss.Height(m.View().Content), m.height; got != want {
				t.Fatalf("expected jobs view height %d, got %d", want, got)
			}
		})
	}
}

func TestJobsBodyFitsWindowWidthAcrossBreakpoints(t *testing.T) {
	t.Parallel()

	for _, size := range []tea.WindowSizeMsg{
		{Width: 80, Height: 24},
		{Width: 120, Height: 30},
		{Width: 160, Height: 40},
	} {
		size := size
		t.Run(fmt.Sprintf("Should fit jobs body at %dx%d", size.Width, size.Height), func(t *testing.T) {
			t.Parallel()

			m := newPopulatedUIModelForTest(t, size)
			body := m.renderJobsBody()
			if got, want := lipgloss.Width(body), m.width; got != want {
				t.Fatalf("expected jobs body width %d, got %d", want, got)
			}
			if got, want := lipgloss.Height(body), m.contentHeight; got != want {
				t.Fatalf("expected jobs body height %d, got %d", want, got)
			}
		})
	}
}

func TestResizeGateAppearsBelowMinimumTerminalSize(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 79, Height: 23})
	view := m.View().Content
	if !strings.Contains(view, "Compozy needs at least 80x24") {
		t.Fatalf("expected resize gate, got %q", view)
	}
}

func TestJobsViewUsesACPChromeWithoutInspectorPane(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 160, Height: 40})
	view := m.View().Content

	for _, want := range []string{"COMPOZY", "SESSION.TIMELINE"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected jobs view to contain %q", want)
		}
	}
	for _, reject := range []string{
		"ACP COCKPIT",
		"SYS.PIPELINE",
		"SESSION.INSPECTOR",
		"Selection",
		"Plan",
		"Edits",
		"Session",
		"INSPECT",
	} {
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
		CodeFile: "task_01",
		ExitCode: 42,
		OutLog:   "task_01.out.log",
		ErrLog:   "task_01.err.log",
		Err:      fmt.Errorf("boom"),
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

	row := xansi.Strip(m.renderSidebarItem(0, &m.jobs[0], true))
	if strings.Contains(row, "FILES") || strings.Contains(row, "ISSUES") {
		t.Fatalf("sidebar card must drop files/issues meta, got %q", row)
	}
	if !strings.Contains(row, "2/3") {
		t.Fatalf("expected retrying card to surface attempt 2/3, got %q", row)
	}

	meta := m.timelineMetaForWidth(&m.jobs[0], 80)
	for _, want := range []string{"attempt 2/3", "retrying:"} {
		if !strings.Contains(meta, want) {
			t.Fatalf("expected retry timeline meta to contain %q, got %q", want, meta)
		}
	}
}

func TestTimelineHeaderLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		job  *uiJob
		want string
	}{
		{
			name: "fallback without title",
			job:  &uiJob{},
			want: "session.timeline",
		},
		{
			name: "title and badge",
			job: &uiJob{
				taskTitle: "acp agent layer",
				taskType:  "backend",
			},
			want: "ACP AGENT LAYER  [backend]",
		},
		{
			name: "title without badge",
			job: &uiJob{
				taskTitle: "acp agent layer",
			},
			want: "ACP AGENT LAYER",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run("Should render "+tt.name, func(t *testing.T) {
			t.Parallel()

			if got := timelineHeaderLabel(tt.job); got != tt.want {
				t.Fatalf("expected header %q, got %q", tt.want, got)
			}
		})
	}
}

func TestTimelineMetaRightAlignsRuntimeLabel(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 120, Height: 30})
	meta := m.timelineMetaForWidth(&m.jobs[0], 48)

	if !strings.Contains(meta, "selected 2/2") {
		t.Fatalf("expected left-side counter in meta row, got %q", meta)
	}
	if !strings.HasSuffix(meta, "Claude · sonnet-4.5") {
		t.Fatalf("expected right-aligned runtime label suffix, got %q", meta)
	}
	if got, want := lipgloss.Width(meta), 48; got != want {
		t.Fatalf("expected meta row width %d, got %d in %q", want, got, meta)
	}
}

func TestTimelineMetaUsesCurrentTimelineWidth(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 120, Height: 30})
	got := m.timelineMeta(&m.jobs[0])
	wantWidth := panelContentWidth(m.timelineWidth)
	if width := lipgloss.Width(got); width != wantWidth {
		t.Fatalf("expected meta width %d from current timeline width, got %d", wantWidth, width)
	}
}

func TestTimelineEntryMetaHandlesNilAndEmptySnapshots(t *testing.T) {
	t.Parallel()

	m := newUIModel(1)
	if got := m.timelineEntryMeta(nil); got != "No ACP transcript yet" {
		t.Fatalf("expected nil job fallback meta, got %q", got)
	}

	job := &uiJob{}
	if got := m.timelineEntryMeta(job); got != "No ACP transcript yet" {
		t.Fatalf("expected empty snapshot fallback meta, got %q", got)
	}
}

func TestTimelineMetaTruncatesLeftSideFirst(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.handleJobRetry(jobRetryMsg{
		Index:       0,
		Attempt:     2,
		MaxAttempts: 3,
		Reason:      "temporary setup failure while loading a very large artifact",
	})

	meta := m.timelineMetaForWidth(&m.jobs[0], 32)

	if !strings.HasSuffix(meta, "Claude · sonnet-4.5") {
		t.Fatalf("expected runtime label to remain intact, got %q", meta)
	}
	if !strings.Contains(meta, "…") {
		t.Fatalf("expected left side to truncate first, got %q", meta)
	}
}

func TestTimelineRuntimeMetaFallbacks(t *testing.T) {
	t.Parallel()

	m := newUIModel(1)
	if got := m.timelineRuntimeMeta(); got != "" {
		t.Fatalf("expected empty runtime meta without cfg, got %q", got)
	}

	m.cfg = &config{Model: "sonnet-4.5"}
	if got := m.timelineRuntimeMeta(); got != "sonnet-4.5" {
		t.Fatalf("expected model-only runtime meta, got %q", got)
	}

	m.cfg = &config{IDE: model.IDEClaude}
	if got := m.timelineRuntimeMeta(); got != "Claude" {
		t.Fatalf("expected provider-only runtime meta, got %q", got)
	}

	m.jobs = []uiJob{{ide: model.IDECodex, model: "gpt-5.5"}}
	m.selectedJob = 0
	if got := m.timelineRuntimeMeta(); got != "Codex · gpt-5.5" {
		t.Fatalf("expected current job runtime meta to override cfg, got %q", got)
	}

	m.jobs = []uiJob{{
		ide:             model.IDECodex,
		model:           "gpt-5.5",
		reasoningEffort: "xhigh",
		tokenUsage:      &model.Usage{InputTokens: 8123, OutputTokens: 4200},
	}}
	if got := m.timelineRuntimeMeta(); got != "Codex · gpt-5.5 · xhigh · 12.3k tok" {
		t.Fatalf("expected reasoning effort and provider token total in runtime meta, got %q", got)
	}
}

func TestRenderTimelinePanelKeepsFallbackHeaderWithoutTaskTitle(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 120, Height: 30})
	panel := normalizedStrippedPanelText(m.renderTimelinePanel(&m.jobs[0], 80))

	if !strings.Contains(panel, "SESSION.TIMELINE") {
		t.Fatalf("expected fallback timeline label, got %q", panel)
	}
	if strings.Contains(panel, "[backend]") {
		t.Fatalf("expected no badge without task title, got %q", panel)
	}
	if !strings.Contains(panel, "Claude · sonnet-4.5") {
		t.Fatalf("expected provider/model meta in fallback layout, got %q", panel)
	}
}

func TestRenderTimelinePanelSplitsIntoThreeBorderedBoxes(t *testing.T) {
	t.Parallel()

	t.Run("Should split timeline panel into three bordered boxes", func(t *testing.T) {
		t.Parallel()

		m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 120, Height: 30})
		m.jobs[0].taskTitle = "acp agent layer"
		m.jobs[0].taskType = "backend"
		m.handleJobUpdate(jobUpdateMsg{
			Index: 0,
			Snapshot: buildSnapshotWithEntries(t,
				TranscriptEntry{
					ID:     "assistant-1",
					Kind:   transcriptEntryAssistantMessage,
					Title:  "Assistant",
					Blocks: []model.ContentBlock{mustContentBlockUITest(t, model.TextBlock{Text: "hello from ACP"})},
				},
				TranscriptEntry{
					ID:            "tool-1",
					Kind:          transcriptEntryToolCall,
					Title:         "read_file",
					ToolCallID:    "tool-1",
					ToolCallState: model.ToolCallStateCompleted,
					Blocks: []model.ContentBlock{
						mustContentBlockUITest(t, model.ToolUseBlock{ID: "tool-1", Name: "read_file"}),
						mustContentBlockUITest(
							t,
							model.ToolResultBlock{ToolUseID: "tool-1", Content: "loaded README.md"},
						),
					},
				},
				TranscriptEntry{
					ID:     "assistant-2",
					Kind:   transcriptEntryAssistantMessage,
					Title:  "Assistant",
					Blocks: []model.ContentBlock{mustContentBlockUITest(t, model.TextBlock{Text: "task complete"})},
				},
			),
		})

		panel := m.renderTimelinePanel(&m.jobs[0], 80)
		stripped := xansi.Strip(panel)

		// The page content is three bordered boxes: header, streaming messages, and
		// composer textbox stay visually distinct with their own divider rows.
		topBorders := 0
		bottomBorders := 0
		for _, line := range strings.Split(stripped, "\n") {
			if strings.HasPrefix(line, "┌") {
				topBorders++
			}
			if strings.HasPrefix(line, "└") {
				bottomBorders++
			}
		}
		if topBorders != 3 {
			t.Fatalf("expected three box top borders, got %d in:\n%s", topBorders, stripped)
		}
		if bottomBorders != 3 {
			t.Fatalf("expected three box bottom borders, got %d in:\n%s", bottomBorders, stripped)
		}
		if dividers := strings.Count(stripped, "┘\n┌"); dividers != 2 {
			t.Fatalf("expected two divider boundaries between boxes, got %d in:\n%s", dividers, stripped)
		}
		if strings.ContainsAny(stripped, "╭╮╰╯") {
			t.Fatalf("expected square borders only, got %q", stripped)
		}

		// Header box: task title + type badge and the right-aligned runtime meta.
		if !strings.Contains(stripped, "ACP AGENT LAYER  [backend]") {
			t.Fatalf("expected header box to show title and badge, got:\n%s", stripped)
		}
		if !strings.Contains(stripped, "Claude · sonnet-4.5") {
			t.Fatalf("expected header box to show runtime meta, got:\n%s", stripped)
		}
		// Messages box: the streamed transcript entries.
		for _, want := range []string{"Assistant", "hello from ACP", "✓ read_file", "task complete"} {
			if !strings.Contains(stripped, want) {
				t.Fatalf("expected messages box to contain %q, got:\n%s", want, stripped)
			}
		}
		// Composer box: the prompt glyph stays visible even while disabled.
		if !strings.Contains(stripped, composerPromptGlyph+"Task running") {
			t.Fatalf("expected composer box to show the disabled label, got:\n%s", stripped)
		}

		// Every rendered line spans the full panel width so the boxes align exactly.
		for i, line := range strings.Split(panel, "\n") {
			if got := xansi.StringWidth(line); got != 80 {
				t.Fatalf("expected panel line %d width 80, got %d: %q", i, got, xansi.Strip(line))
			}
		}
	})
}

func TestComposerPanelRendersPromptGlyphForAllStates(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.composer.SetValue("draft")
	baseValue := m.composer.Value()

	cases := []struct {
		name       string
		job        *uiJob
		busy       bool
		errorText  string
		wantSuffix string
	}{
		{name: "no task", job: nil, wantSuffix: "No task selected"},
		{name: "pending", job: composerStateJob(m.jobs[0], jobPending), wantSuffix: "Task pending"},
		{name: "running", job: composerStateJob(m.jobs[0], jobRunning), wantSuffix: "Task running"},
		{name: "pausing", job: composerStateJob(m.jobs[0], jobPausing), wantSuffix: "Pausing task..."},
		{name: "paused input", job: composerStateJob(m.jobs[0], jobPaused), wantSuffix: "draft"},
		{name: "sending", job: composerStateJob(m.jobs[0], jobPaused), busy: true, wantSuffix: "Sending message..."},
		{
			name:       "error",
			job:        composerStateJob(m.jobs[0], jobRunning),
			errorText:  "pause failed",
			wantSuffix: "pause failed",
		},
		{name: "completed", job: composerStateJob(m.jobs[0], jobSuccess), wantSuffix: "Task completed"},
		{name: "failed", job: composerStateJob(m.jobs[0], jobFailed), wantSuffix: "Task failed"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("Should render "+tc.name, func(t *testing.T) {
			m.composerBusy = tc.busy
			m.composerError = tc.errorText
			m.composer.SetValue(baseValue)

			panel := renderComposerForTest(m, tc.job, 80)
			stripped := xansi.Strip(panel)
			if !strings.Contains(stripped, composerPromptGlyph+tc.wantSuffix) {
				t.Fatalf("expected composer prompt before %q, got:\n%s", tc.wantSuffix, stripped)
			}
			if got := m.composer.Value(); got != baseValue {
				t.Fatalf("composer value = %q, want unchanged %q", got, baseValue)
			}
		})
	}
}

func composerStateJob(job uiJob, state jobState) *uiJob {
	job.state = state
	return &job
}

func renderComposerForTest(m *uiModel, job *uiJob, panelWidth int) string {
	return m.renderComposerContent(job, panelContentWidth(panelWidth))
}

func TestComposerPanelRendersPromptGlyphOutsideInputValue(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.handleJobPaused(jobPausedMsg{Index: 0})
	m.focusedPane = uiPaneTimeline
	m.composer.Blur()
	m.composer.SetValue("")

	panel := renderComposerForTest(m, &m.jobs[0], 80)
	stripped := xansi.Strip(panel)
	if !strings.Contains(stripped, composerPromptGlyph+composerPausedTaskPrompt) {
		t.Fatalf("expected composer prompt before placeholder, got:\n%s", stripped)
	}
	if got := m.composer.Value(); got != "" {
		t.Fatalf("composer value = %q, want prompt glyph outside input value", got)
	}

	m.composer.SetValue("send details")
	panel = renderComposerForTest(m, &m.jobs[0], 80)
	stripped = xansi.Strip(panel)
	if !strings.Contains(stripped, composerPromptGlyph+"send details") {
		t.Fatalf("expected composer prompt before input value, got:\n%s", stripped)
	}
	if got := m.composer.Value(); got != "send details" {
		t.Fatalf("composer value = %q, want unchanged input value", got)
	}
}

func TestComposerPanelFocusChangesTextForegroundOnly(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.handleJobPaused(jobPausedMsg{Index: 0})
	m.focusedPane = uiPaneComposer
	m.composer.SetValue("send details")
	m.composer.CursorEnd()
	_ = m.composer.Focus()

	panel := renderComposerForTest(m, &m.jobs[0], 80)
	assertNoForcedBackground(t, panel)
	styles := m.composer.Styles()
	for name, style := range map[string]lipgloss.Style{
		"focused base":        styles.Focused.Base,
		"focused text":        styles.Focused.Text,
		"focused cursor line": styles.Focused.CursorLine,
		"blurred base":        styles.Blurred.Base,
		"blurred text":        styles.Blurred.Text,
		"blurred cursor line": styles.Blurred.CursorLine,
	} {
		if !sameColor(style.GetBackground(), lipgloss.NoColor{}) {
			t.Fatalf("expected %s to inherit terminal background, got %v", name, style.GetBackground())
		}
	}
	if !sameColor(styles.Focused.Text.GetForeground(), colorFgBright) {
		t.Fatalf("focused text foreground = %v, want %v", styles.Focused.Text.GetForeground(), colorFgBright)
	}
	if !sameColor(styles.Blurred.Text.GetForeground(), colorMuted) {
		t.Fatalf("blurred text foreground = %v, want %v", styles.Blurred.Text.GetForeground(), colorMuted)
	}
	if !sameColor(styles.Focused.Prompt.GetForeground(), colorMuted) {
		t.Fatalf("focused prompt foreground = %v, want muted prompt", styles.Focused.Prompt.GetForeground())
	}
}

func TestRenderTimelinePanelMinWidthPreservesRuntimeLabel(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.jobs[0].taskTitle = "acp agent layer"
	m.jobs[0].taskType = "backend"

	panel := normalizedStrippedPanelText(m.renderTimelinePanel(&m.jobs[0], timelineMinWidth))

	if !strings.Contains(panel, "Claude · sonnet-4.5") {
		t.Fatalf("expected runtime label to remain visible at min width, got %q", panel)
	}
}

func TestRenderTimelinePanelTranscriptHeightAccountsForThreeBoxes(t *testing.T) {
	t.Parallel()

	t.Run("Should keep transcript height independent of title", func(t *testing.T) {
		t.Parallel()

		m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 120, Height: 30})

		m.jobs[0].taskTitle = ""
		m.renderTimelinePanel(&m.jobs[0], 80)
		withoutTitle := m.transcriptViewport.Height()

		m.jobs[0].taskTitle = "acp agent layer"
		m.renderTimelinePanel(&m.jobs[0], 80)
		withTitle := m.transcriptViewport.Height()

		// The header now lives in its own box, so the task title no longer steals a
		// transcript row. The messages region keeps a stable height regardless of title.
		if withTitle != withoutTitle {
			t.Fatalf("expected transcript height to be title-independent, got %d with title vs %d without",
				withTitle, withoutTitle)
		}
		if want := max(m.contentHeight-timelineChromeRows, logViewportMinHeight); withTitle != want {
			t.Fatalf("expected transcript height %d (contentHeight - timelineChromeRows), got %d", want, withTitle)
		}
	})
}

func TestRenderMainPanelsReturnsBlankWithoutCurrentJob(t *testing.T) {
	t.Parallel()

	t.Run("Should return blank content without current job", func(t *testing.T) {
		t.Parallel()

		m := newUIModel(1)
		m.width = 100
		m.sidebarWidth = 30

		content := m.renderMainPanels()
		if got, want := lipgloss.Width(content), 70; got != want {
			t.Fatalf("expected blank main panel width %d, got %d", want, got)
		}
	})
}

func TestSidebarStatusAndShutdownLabels(t *testing.T) {
	t.Parallel()

	t.Run("Should render status and shutdown labels", func(t *testing.T) {
		t.Parallel()

		m := newUIModel(3)

		if got := xansi.Strip(m.renderSidebarTitle(40)); !strings.Contains(got, "JOB 0/3") {
			t.Fatalf("expected running sidebar status text, got %q", got)
		}

		m.failed = 1
		if got := xansi.Strip(m.renderSidebarTitle(40)); !strings.Contains(got, "JOB 1/3 · 1 FAIL") {
			t.Fatalf("expected partial failure sidebar status text, got %q", got)
		}

		m.shutdown = shutdownState{
			Phase:      shutdownPhaseDraining,
			DeadlineAt: time.Now().Add(1500 * time.Millisecond),
		}
		draining := xansi.Strip(m.renderSidebarTitle(40))
		if !strings.Contains(draining, "JOB DRAINING 1/3") || !strings.Contains(draining, "s") {
			t.Fatalf("expected draining status with countdown, got %q", draining)
		}

		m.shutdown = shutdownState{Phase: shutdownPhaseForcing}
		if got := m.shutdownHeaderLabel(); got != "FORCING 1/3" {
			t.Fatalf("expected forcing header label, got %q", got)
		}
		m.shutdown = shutdownState{Phase: shutdownPhaseDraining}
		if got := m.shutdownHeaderLabel(); got != "DRAINING 1/3" {
			t.Fatalf("expected draining header without countdown, got %q", got)
		}
		m.shutdown = shutdownState{}
		if got := m.shutdownHeaderLabel(); got != "RUN 1/3" {
			t.Fatalf("expected default run header label, got %q", got)
		}
		if got := m.shutdownCountdownLabel(); got != "" {
			t.Fatalf("expected empty countdown without deadline, got %q", got)
		}
		m.shutdown = shutdownState{DeadlineAt: time.Now().Add(-time.Second)}
		if got := m.shutdownCountdownLabel(); got != "0s" {
			t.Fatalf("expected expired countdown to clamp to 0s, got %q", got)
		}

		m.completed = 2
		m.failed = 1
		if got := xansi.Strip(m.renderSidebarTitle(40)); !strings.Contains(got, "JOB 3/3 · 1 FAIL") {
			t.Fatalf("expected completed failure sidebar summary, got %q", got)
		}

		m.shutdown = shutdownState{}
		m.completed = 3
		m.failed = 0
		if got := xansi.Strip(m.renderSidebarTitle(40)); !strings.Contains(got, "JOB 3/3") {
			t.Fatalf("expected success sidebar summary, got %q", got)
		}
	})
}

func TestSidebarStatusLinePreservesSemanticColors(t *testing.T) {
	t.Run("Should preserve semantic colors", func(t *testing.T) {
		forceTrueColorForTest(t)

		m := newUIModel(3)
		warningStyle := lipgloss.NewStyle().Bold(true).Foreground(colorWarning)
		successStyle := lipgloss.NewStyle().Bold(true).Foreground(colorSuccess)

		m.failed = 1
		if got := m.renderSidebarTitle(40); !strings.Contains(got, warningStyle.Render("1/3 · 1 FAIL")) {
			t.Fatalf("expected failed status to use warning color, got %q", got)
		}

		m.completed = 1
		m.failed = 0
		m.shutdown = shutdownState{Phase: shutdownPhaseDraining}
		if got := m.renderSidebarTitle(40); !strings.Contains(got, warningStyle.Render("DRAINING 1/3")) {
			t.Fatalf("expected draining status to use warning color, got %q", got)
		}

		m.shutdown = shutdownState{}
		m.completed = 3
		if got := m.renderSidebarTitle(40); !strings.Contains(got, successStyle.Render("3/3")) {
			t.Fatalf("expected successful status to use success color, got %q", got)
		}
	})
}

func TestSidebarStatusLinePrioritizesStatusWhenTokenLabelDoesNotFit(t *testing.T) {
	t.Parallel()

	t.Run("Should prioritize status when token label does not fit", func(t *testing.T) {
		t.Parallel()

		m := newUIModel(3)
		m.completed = 1
		m.shutdown = shutdownState{Phase: shutdownPhaseDraining}
		m.aggregateUsage.Add(model.Usage{TotalTokens: 32490537})

		rendered := m.renderSidebarTitle(24)
		firstLine := strings.Split(rendered, "\n")[0]
		stripped := xansi.Strip(firstLine)
		if !strings.Contains(stripped, "JOB DRAINING 1/3") {
			t.Fatalf("expected narrow status line to preserve status, got %q", stripped)
		}
		if strings.Contains(stripped, "tok") {
			t.Fatalf("expected narrow status line to drop token label, got %q", stripped)
		}
		if got, want := xansi.StringWidth(firstLine), 24; got != want {
			t.Fatalf("expected narrow status line width %d, got %d: %q", want, got, stripped)
		}
	})
}

func TestRenderSidebarStackNormalizesMalformedCards(t *testing.T) {
	t.Parallel()

	t.Run("Should normalize malformed cards", func(t *testing.T) {
		t.Parallel()

		content := renderSidebarStack([]string{
			"top\nmid",
			"a\nb\nc\nd\ne",
		}, 12)
		lines := strings.Split(content, "\n")
		if got, want := len(lines), sidebarRowLines+sidebarRowStride; got != want {
			t.Fatalf("expected normalized stack height %d, got %d: %q", want, got, content)
		}
		for i, line := range lines {
			if got, want := xansi.StringWidth(line), 12; got != want {
				t.Fatalf("expected normalized line %d width %d, got %d: %q", i, want, got, xansi.Strip(line))
			}
		}
	})
}

func TestShouldRenderEntryPreview(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 120, Height: 30})
	job := &m.jobs[0]

	assistant := TranscriptEntry{
		ID:    "assistant",
		Kind:  transcriptEntryAssistantMessage,
		Title: "Assistant",
	}
	job.expandedEntryIDs[assistant.ID] = false
	if !m.shouldRenderEntryPreview(job, assistant) {
		t.Fatal("expected collapsed assistant entry to render preview")
	}

	job.expandedEntryIDs[assistant.ID] = true
	if m.shouldRenderEntryPreview(job, assistant) {
		t.Fatal("expected expanded narrative entry to suppress preview")
	}

	tool := TranscriptEntry{
		ID:            "tool",
		Kind:          transcriptEntryToolCall,
		Title:         "read_file",
		ToolCallState: model.ToolCallStateCompleted,
	}
	job.expandedEntryIDs[tool.ID] = true
	if !m.shouldRenderEntryPreview(job, tool) {
		t.Fatal("expected expanded tool entry to keep preview")
	}
}

func TestToolCallStateMappings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		state     model.ToolCallState
		wantLabel string
		wantIcon  string
		wantColor color.Color
	}{
		{state: model.ToolCallStatePending, wantLabel: "PENDING", wantIcon: "○", wantColor: colorAccentAlt},
		{state: model.ToolCallStateInProgress, wantLabel: "RUNNING", wantIcon: "●", wantColor: colorBrand},
		{state: model.ToolCallStateCompleted, wantLabel: "", wantIcon: "✓", wantColor: colorSuccess},
		{state: model.ToolCallStateFailed, wantLabel: "FAILED", wantIcon: "✗", wantColor: colorError},
		{
			state:     model.ToolCallStateWaitingForConfirmation,
			wantLabel: "CONFIRM",
			wantIcon:  "!",
			wantColor: colorWarning,
		},
		{state: model.ToolCallState("mystery"), wantLabel: "READY", wantIcon: "•", wantColor: colorInfo},
	}

	for _, tt := range tests {
		tt := tt
		t.Run("Should render tool call state "+string(tt.state), func(t *testing.T) {
			t.Parallel()

			if got := toolCallStateLabel(tt.state); got != tt.wantLabel {
				t.Fatalf("expected label %q, got %q", tt.wantLabel, got)
			}
			if got := toolCallStateIcon(tt.state); got != tt.wantIcon {
				t.Fatalf("expected icon %q, got %q", tt.wantIcon, got)
			}
			if !sameColor(toolCallStateColor(tt.state), tt.wantColor) {
				t.Fatalf("expected matching color for state %q", tt.state)
			}
		})
	}
}

func TestBuildTimelineContentUsesWaitingAndCachePaths(t *testing.T) {
	t.Parallel()

	m := newUIModel(1)
	job := &uiJob{
		selectedEntry:    -1,
		expandedEntryIDs: make(map[string]bool),
	}

	waiting := m.buildTimelineContent(job, 40)
	if !strings.Contains(waiting.content, "Waiting for ACP updates") {
		t.Fatalf("expected waiting content, got %q", waiting.content)
	}
	if !job.timelineCacheValid {
		t.Fatal("expected waiting content to populate cache")
	}

	job.snapshot = buildSnapshotWithEntries(t,
		TranscriptEntry{
			ID:     "assistant-1",
			Kind:   transcriptEntryAssistantMessage,
			Title:  "Assistant",
			Blocks: []model.ContentBlock{mustContentBlockUITest(t, model.TextBlock{Text: "cached content"})},
		},
	)
	job.selectedEntry = 0
	job.timelineCacheValid = false

	first := m.buildTimelineContent(job, 40)
	second := m.buildTimelineContent(job, 40)
	if first.content != second.content {
		t.Fatalf("expected cached timeline content to be reused")
	}
}

func TestRenderWrappedBlocksLinesAndWrapViewportLines(t *testing.T) {
	t.Parallel()

	m := newUIModel(1)
	lines := m.renderWrappedBlocksLines(
		[]model.ContentBlock{
			mustContentBlockUITest(
				t,
				model.TextBlock{Text: narrativeWrapText("assistant") + "\n\n" + narrativeWrapText("followup")},
			),
		},
		18,
	)
	if len(lines) < 4 {
		t.Fatalf("expected wrapped narrative lines, got %#v", lines)
	}
	if !containsString(lines, "") {
		t.Fatalf("expected wrapped lines to preserve blank separators, got %#v", lines)
	}

	withErrLines := m.renderWrappedBlocksLines(
		[]model.ContentBlock{
			mustContentBlockUITest(t, model.TextBlock{Text: "stdout line"}),
			mustContentBlockUITest(t, model.ToolResultBlock{
				ToolUseID: "tool-1",
				Content:   "stderr line one\nstderr line two",
				IsError:   true,
			}),
		},
		18,
	)
	if !containsString(withErrLines, "") {
		t.Fatalf("expected wrapped err lines to be separated, got %#v", withErrLines)
	}
}

func TestRestoreTranscriptViewportTracksOffsets(t *testing.T) {
	t.Parallel()

	m := newUIModel(1)
	job := &uiJob{
		selectedEntry:        0,
		transcriptFollowTail: false,
	}
	m.transcriptViewport.SetContent(strings.Repeat("line\n", 20))
	m.transcriptViewport.SetHeight(3)
	m.restoreTranscriptViewport(job, nil)
	if !job.transcriptFollowTail || job.transcriptYOffset != 0 || job.transcriptXOffset != 0 {
		t.Fatalf("expected empty offsets to reset transcript state, got %#v", job)
	}

	job.selectedEntry = 2
	job.transcriptFollowTail = false
	job.transcriptYOffset = 0
	m.restoreTranscriptViewport(job, []int{0, 3, 8})
	if got := m.transcriptViewport.YOffset(); got == 0 {
		t.Fatalf("expected selected entry to be scrolled into view, got offset %d", got)
	}
}

func TestRenderTimelinePanelSkipsViewportSetContentOnCacheHit(t *testing.T) {
	m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 120, Height: 30})
	job := &m.jobs[0]

	calls := 0
	previous := m.setTranscriptViewportContent
	m.setTranscriptViewportContent = func(vp *viewport.Model, content string) {
		if vp == &m.transcriptViewport {
			calls++
		}
		previous(vp, content)
	}

	_ = m.renderTimelinePanel(job, m.timelineWidth)
	if calls != 1 {
		t.Fatalf("expected first render to set transcript content once, got %d calls", calls)
	}

	calls = 0
	_ = m.renderTimelinePanel(job, m.timelineWidth)
	if calls != 0 {
		t.Fatalf("expected cache hit to skip transcript SetContent, got %d calls", calls)
	}
}

func TestRenderTimelinePanelRemountsCachedTranscriptWhenSelectedJobChanges(t *testing.T) {
	t.Parallel()

	m := newUIModel(2)
	m.handleJobQueued(&jobQueuedMsg{
		Index:     0,
		CodeFile:  "task_01",
		CodeFiles: []string{"task_01"},
		Issues:    1,
		SafeName:  "task_01-safe",
	})
	m.handleJobQueued(&jobQueuedMsg{
		Index:     1,
		CodeFile:  "task_02",
		CodeFiles: []string{"task_02"},
		Issues:    1,
		SafeName:  "task_02-safe",
	})
	m.handleWindowSize(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.handleJobUpdate(jobUpdateMsg{
		Index: 0,
		Snapshot: buildSnapshotWithEntries(t, TranscriptEntry{
			ID:     "assistant-1",
			Kind:   transcriptEntryAssistantMessage,
			Title:  "Assistant",
			Blocks: []model.ContentBlock{mustContentBlockUITest(t, model.TextBlock{Text: "first transcript"})},
		}),
	})
	m.handleJobUpdate(jobUpdateMsg{
		Index: 1,
		Snapshot: buildSnapshotWithEntries(t, TranscriptEntry{
			ID:     "assistant-2",
			Kind:   transcriptEntryAssistantMessage,
			Title:  "Assistant",
			Blocks: []model.ContentBlock{mustContentBlockUITest(t, model.TextBlock{Text: "second transcript"})},
		}),
	})
	m.selectedJob = 0

	var mounted []string
	previous := m.setTranscriptViewportContent
	m.setTranscriptViewportContent = func(vp *viewport.Model, content string) {
		if vp == &m.transcriptViewport {
			mounted = append(mounted, xansi.Strip(content))
		}
		previous(vp, content)
	}

	_ = m.renderTimelinePanel(&m.jobs[0], m.timelineWidth)
	mounted = mounted[:0]

	_ = m.buildTimelineContent(&m.jobs[1], panelContentWidth(m.timelineWidth))
	m.selectedJob = 1

	panel := normalizedStrippedPanelText(m.renderTimelinePanel(&m.jobs[1], m.timelineWidth))
	if got := len(mounted); got != 1 {
		t.Fatalf("expected selected-job switch to remount cached transcript content once, got %d", got)
	}
	if !strings.Contains(mounted[0], "second transcript") || strings.Contains(mounted[0], "first transcript") {
		t.Fatalf("expected remounted transcript to belong to job 2, got %q", mounted[0])
	}
	if !strings.Contains(panel, "second transcript") {
		t.Fatalf("expected selected panel to show job 2 transcript, got %q", panel)
	}
	if strings.Contains(panel, "first transcript") {
		t.Fatalf("expected selected panel to stop showing job 1 transcript, got %q", panel)
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

	assertNoForcedBackground(t, m.renderHelp())
}

func TestActiveRunHelpUsesExitLabel(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 120, Height: 30})

	help := m.renderHelp()
	if !strings.Contains(help, "EXIT") {
		t.Fatalf("expected active-run help to advertise EXIT, got %q", help)
	}
	if strings.Contains(help, "FORCE QUIT") {
		t.Fatalf("expected active-run help not to advertise force quit before shutdown, got %q", help)
	}
}

func TestJobPaneHelpUsesVerticalNavigation(t *testing.T) {
	t.Parallel()

	t.Run("Should advertise vertical navigation for job pane", func(t *testing.T) {
		m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 120, Height: 30})
		m.focusedPane = uiPaneJobs

		help := m.renderHelp()
		if !strings.Contains(help, "↑↓/JK") {
			t.Fatalf("expected job help to advertise vertical navigation, got %q", help)
		}
		if strings.Contains(help, "←→/HL") {
			t.Fatalf("expected job help not to advertise horizontal navigation, got %q", help)
		}
	})
}

func TestTitleBarShowsBrandAndWorkflowChip(t *testing.T) {
	t.Parallel()

	t.Run("Should show brand and workflow chip", func(t *testing.T) {
		t.Parallel()

		m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 120, Height: 30})
		m.cfg.Name = "my-workflow"

		bar := xansi.Strip(m.renderTitleBar())
		for _, want := range []string{"COMPOZY", "my-workflow", "RUNNING"} {
			if !strings.Contains(bar, want) {
				t.Fatalf("expected single-run title row to contain %q, got %q", want, bar)
			}
		}
		if got := strings.Count(bar, "COMPOZY"); got != 1 {
			t.Fatalf("expected the brand exactly once on the title row, got %d in %q", got, bar)
		}
	})
}

func TestRunChipStatusUsesAggregateRunStatus(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		status    string
		wantLabel string
		wantColor color.Color
	}{
		{name: "Should render failed run status", status: remoteRunStatusFailed, wantLabel: "FAILED", wantColor: colorError},
		{name: "Should render crashed run status", status: remoteRunStatusCrashed, wantLabel: "CRASHED", wantColor: colorError},
		{name: "Should render canceled run status", status: remoteRunStatusCanceled, wantLabel: "CANCELED", wantColor: colorWarning},
		{name: "Should render completed run status", status: remoteRunStatusCompleted, wantLabel: "DONE", wantColor: colorSuccess},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			m := newUIModel(0)
			m.runStatus = tc.status
			m.shutdown = shutdownState{Phase: shutdownPhaseDraining}

			_, label, gotColor := m.runChipStatus()
			if label != tc.wantLabel {
				t.Fatalf("runChipStatus() label = %q, want %q", label, tc.wantLabel)
			}
			if !sameColor(gotColor, tc.wantColor) {
				t.Fatalf("runChipStatus() color = %#v, want %#v", gotColor, tc.wantColor)
			}
		})
	}

	t.Run("Should preserve canceled snapshot status for empty remote runs", func(t *testing.T) {
		t.Parallel()

		_, msgs := remoteSnapshotBootstrap(apicore.RunSnapshot{
			Run: apicore.Run{RunID: "run-canceled-empty", Status: remoteRunStatusCanceled},
		})
		m := newUIModel(0)
		for _, msg := range msgs {
			m.applyUIMsg(msg)
		}

		_, label, _ := m.runChipStatus()
		if label != "CANCELED" {
			t.Fatalf("runChipStatus() label = %q, want CANCELED", label)
		}
	})

	t.Run("Should preserve crashed stream event status", func(t *testing.T) {
		t.Parallel()

		msg, ok := translateRunEvent(eventspkg.Event{Kind: eventspkg.EventKindRunCrashed})
		if !ok {
			t.Fatal("translateRunEvent(run.crashed) did not produce a message")
		}
		m := newUIModel(0)
		m.applyUIMsg(msg)

		_, label, _ := m.runChipStatus()
		if label != "CRASHED" {
			t.Fatalf("runChipStatus() label = %q, want CRASHED", label)
		}
	})
}

func TestRunTerminalFailurePayloadOpensSummaryBeforeJobsSettle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		kind       eventspkg.EventKind
		payload    any
		wantStatus string
		wantError  string
	}{
		{
			name:       "failed run",
			kind:       eventspkg.EventKindRunFailed,
			payload:    kinds.RunFailedPayload{Error: "prepare workflow: invalid task graph"},
			wantStatus: remoteRunStatusFailed,
			wantError:  "prepare workflow: invalid task graph",
		},
		{
			name:       "crashed run",
			kind:       eventspkg.EventKindRunCrashed,
			payload:    kinds.RunCrashedPayload{Error: "reconcile daemon state: journal unavailable"},
			wantStatus: remoteRunStatusCrashed,
			wantError:  "reconcile daemon state: journal unavailable",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			msg, ok := translateRunEvent(mustRuntimeEventUITest(t, tt.kind, tt.payload))
			if !ok {
				t.Fatalf("translateRunEvent(%s) did not produce a message", tt.kind)
			}
			status, ok := msg.(runStatusMsg)
			if !ok {
				t.Fatalf("translateRunEvent(%s) message = %T, want runStatusMsg", tt.kind, msg)
			}
			if status.Err == nil || status.Err.Error() != tt.wantError {
				t.Fatalf("translateRunEvent(%s) error = %v, want %q", tt.kind, status.Err, tt.wantError)
			}

			m := newUIModel(3)
			m.applyUIMsg(status)

			if m.runStatus != tt.wantStatus {
				t.Fatalf("run status = %q, want %q", m.runStatus, tt.wantStatus)
			}
			if m.settledJobs() != 0 {
				t.Fatalf("pre-job failure settled %d jobs, want 0", m.settledJobs())
			}
			if m.currentView != uiViewSummary {
				t.Fatalf("current view = %q, want failure summary", m.currentView)
			}
			if len(m.failures) != 1 || m.failures[0].Err == nil || m.failures[0].Err.Error() != tt.wantError {
				t.Fatalf("run failures = %#v, want terminal error %q", m.failures, tt.wantError)
			}
			view := xansi.Strip(m.View().Content)
			if !strings.Contains(view, tt.wantError) {
				t.Fatalf("failure summary omitted terminal error %q: %q", tt.wantError, view)
			}
		})
	}
}

func TestRunFailureSummaryExplainsDirtyReviewIsolationRecovery(t *testing.T) {
	t.Parallel()

	const failure = "prepare concurrent review worktrees: review isolation requires source changes outside " +
		".compozy/tasks/nested-workflows to be committed first: internal/core/run/ui/model.go, README.md"
	msg, ok := translateRunEvent(mustRuntimeEventUITest(
		t,
		eventspkg.EventKindRunFailed,
		kinds.RunFailedPayload{Error: failure},
	))
	if !ok {
		t.Fatal("translateRunEvent(run.failed) did not produce a message")
	}

	m := newUIModel(2)
	m.handleWindowSize(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.applyUIMsg(msg)
	view := xansi.Strip(m.View().Content)

	for _, want := range []string{
		"prepare concurrent review worktrees",
		"Blocking paths: internal/core/run/ui/model.go, README.md",
		"committed HEAD",
		"commit or stash",
		"--concurrent 1",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("failure summary omitted %q: %q", want, view)
		}
	}
}

func TestEmbeddedChildOmitsBrandHeaderRow(t *testing.T) {
	t.Parallel()

	t.Run("Should omit brand header row when embedded", func(t *testing.T) {
		t.Parallel()

		m := newUIModel(1)
		m.headerHidden = true
		m.cfg = &config{IDE: model.IDEClaude, Model: "sonnet-4.5"}
		m.handleJobQueued(&jobQueuedMsg{Index: 0, CodeFile: "task_01", SafeName: "task_01-safe"})
		m.handleWindowSize(tea.WindowSizeMsg{Width: 120, Height: 30})

		view := m.View().Content
		if strings.Contains(view, "COMPOZY") {
			t.Fatalf("expected embedded child to omit the brand row, got:\n%s", view)
		}
		if !strings.Contains(view, "FOCUS") {
			t.Fatalf("expected embedded child to keep its footer, got:\n%s", view)
		}
		if got, want := lipgloss.Height(view), m.height; got != want {
			t.Fatalf("expected embedded child view height %d, got %d", want, got)
		}
	})
}

func TestHelpShowsWorkdirOnRight(t *testing.T) {
	t.Parallel()

	t.Run("Should show workdir on the right", func(t *testing.T) {
		t.Parallel()

		m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 160, Height: 30})
		m.cfg.WorkspaceRoot = "/tmp/compozy-workspace-xyz"

		help := xansi.Strip(m.renderHelp())
		if !strings.Contains(help, "compozy-workspace-xyz") {
			t.Fatalf("expected the working directory tail on the right of the help row, got %q", help)
		}
	})
}

func TestTimelinePaneHelpAdvertisesPauseWhilePausable(t *testing.T) {
	t.Parallel()

	t.Run("Should advertise pause while pausable", func(t *testing.T) {
		t.Parallel()

		m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 160, Height: 30})
		m.focusedPane = uiPaneTimeline
		if !m.jobCanPause(m.currentJob()) {
			t.Fatalf("precondition: expected the running job to be pausable")
		}

		help := m.renderHelp()
		if !strings.Contains(help, "PAUSE") {
			t.Fatalf("expected timeline help to advertise the pause shortcut, got %q", help)
		}
	})
}

func TestComposerPaneHelpAdvertisesSendAndCancel(t *testing.T) {
	t.Parallel()

	t.Run("Should advertise send and cancel", func(t *testing.T) {
		t.Parallel()

		m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 160, Height: 30})
		m.handleJobPaused(jobPausedMsg{Index: 0})
		if m.focusedPane != uiPaneComposer {
			t.Fatalf("precondition: expected pause to focus the composer, got %q", m.focusedPane)
		}

		help := m.renderHelp()
		for _, want := range []string{"SEND", "CANCEL"} {
			if !strings.Contains(help, want) {
				t.Fatalf("expected composer help to advertise %q, got %q", want, help)
			}
		}
	})
}

func TestQuitDialogViewContainsChoices(t *testing.T) {
	t.Parallel()

	m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.quitDialog.Open()

	view := m.View().Content
	for _, want := range []string{
		"Leave Active Run?",
		"This run is still active.",
		"Close TUI",
		"Stop Run",
		"Cancel",
		"[enter/q] confirm",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected quit dialog view to contain %q, got %q", want, view)
		}
	}
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
	assertNoForcedBackground(t, rendered.content)
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
	for _, want := range []string{"✓ read_file", "loaded README.md"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected expanded timeline to contain %q, got %q", want, view)
		}
	}
	if strings.Contains(view, "[COMPLETED]") {
		t.Fatalf("expected completed tool entries to omit the completed badge, got %q", view)
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
		t.Run("Should render "+tc.name, func(t *testing.T) {
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

func TestSelectedSidebarCardLinesFillWidth(t *testing.T) {
	t.Parallel()

	t.Run("Should fill selected sidebar card lines to viewport width", func(t *testing.T) {
		t.Parallel()

		m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 120, Height: 30})
		row := m.renderSidebarItem(m.selectedJob, &m.jobs[m.selectedJob], true)
		lines := strings.Split(row, "\n")

		if len(lines) != 4 {
			t.Fatalf("expected a four-line bordered card, got %d: %q", len(lines), row)
		}
		for i, line := range lines {
			if got, want := lipgloss.Width(line), m.sidebarViewport.Width(); got != want {
				t.Fatalf("expected card line %d width %d, got %d", i, want, got)
			}
		}
	})
}

func TestSidebarContentSharesBordersBetweenTaskCards(t *testing.T) {
	t.Run("Should share borders between task cards", func(t *testing.T) {
		forceTrueColorForTest(t)
		m := newUIModel(3)
		m.handleWindowSize(tea.WindowSizeMsg{Width: 120, Height: 30})
		for i := 0; i < 3; i++ {
			m.handleJobQueued(&jobQueuedMsg{
				Index:    i,
				CodeFile: fmt.Sprintf("task_%02d.md", i+1),
				SafeName: fmt.Sprintf("task_%02d-safe", i+1),
			})
		}

		for selected := range m.jobs {
			m.selectedJob = selected
			m.sidebarDirty = true
			m.refreshSidebarContent()
			assertSidebarStackHasNoGapOrAccentLeak(t, m.sidebarContent, m.sidebarViewport.Width(), len(m.jobs))
		}
	})
}

func TestSelectedSidebarItemAvoidsBackgroundFill(t *testing.T) {
	t.Parallel()

	t.Run("Should avoid background fill for selected sidebar item", func(t *testing.T) {
		t.Parallel()

		m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 120, Height: 30})
		selected := m.renderSidebarItem(m.selectedJob, &m.jobs[m.selectedJob], true)
		unselected := m.renderSidebarItem(m.selectedJob, &m.jobs[m.selectedJob], false)
		// The cockpit is foreground-only: a job is a bordered card, never a bg fill,
		// and selection is conveyed by text treatment so card borders can stack
		// without leaking accent color into neighboring cards.
		assertNoForcedBackground(t, selected)
		assertNoAccentBorderGlyphs(t, strings.Split(selected, "\n"))
		stripped := xansi.Strip(selected)
		if !strings.Contains(stripped, "┌") || !strings.Contains(stripped, "└") {
			t.Fatalf("expected a bordered card with all four sides, got %q", stripped)
		}
		if selected == unselected {
			t.Fatal("expected the selected card border to differ from the unselected card")
		}
	})
}

func TestHasForcedBackgroundSGR(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		in   string
		want bool
	}{
		{"Should ignore foreground only styling", "\x1b[38;2;255;255;255mtext\x1b[0m", false},
		{"Should ignore foreground 256 color index matching background selector", "\x1b[38;5;48mtext\x1b[0m", false},
		{"Should detect truecolor backgrounds", "\x1b[48;2;1;2;3mtext\x1b[0m", true},
		{"Should detect 256 color backgrounds", "\x1b[48;5;12mtext\x1b[0m", true},
		{"Should detect standard 16 color backgrounds", "\x1b[44mtext\x1b[0m", true},
		{"Should detect bright 16 color backgrounds", "\x1b[104mtext\x1b[0m", true},
		{"Should detect reverse video", "\x1b[7mtext\x1b[0m", true},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := hasForcedBackgroundSGR(tc.in); got != tc.want {
				t.Fatalf("hasForcedBackgroundSGR(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func newPopulatedUIModelForTest(t *testing.T, size tea.WindowSizeMsg) *uiModel {
	t.Helper()
	return newTestUIModelWithSnapshot(t, size)
}

// assertNoForcedBackground verifies the cockpit renders foreground-only so the
// terminal's native background shows through. Accent chips are tested separately.
func assertNoForcedBackground(t *testing.T, content string) {
	t.Helper()
	if strings.TrimSpace(xansi.Strip(content)) == "" {
		t.Fatal("expected rendered content")
	}
	if hasForcedBackgroundSGR(content) {
		t.Fatalf("expected foreground-only content with no forced background, got %q", content)
	}
}

func hasForcedBackgroundSGR(content string) bool {
	for cursor := 0; cursor < len(content); {
		start := strings.Index(content[cursor:], "\x1b[")
		if start < 0 {
			return false
		}
		start += cursor + len("\x1b[")
		end := strings.IndexByte(content[start:], 'm')
		if end < 0 {
			return false
		}
		if sgrParamsHaveForcedBackground(content[start : start+end]) {
			return true
		}
		cursor = start + end + 1
	}
	return false
}

func sgrParamsHaveForcedBackground(params string) bool {
	if params == "" {
		return false
	}
	parts := strings.Split(params, ";")
	for i := 0; i < len(parts); i++ {
		raw := parts[i]
		value, err := strconv.Atoi(raw)
		if err != nil {
			continue
		}
		if value == 7 || (value >= 40 && value <= 47) || (value >= 100 && value <= 107) {
			return true
		}
		if value != 38 && value != 48 {
			continue
		}
		if i+1 >= len(parts) {
			continue
		}
		mode, err := strconv.Atoi(parts[i+1])
		if err != nil {
			continue
		}
		switch mode {
		case 5:
			if value == 48 {
				return true
			}
			if i+2 < len(parts) {
				i += 2
			} else {
				i++
			}
		case 2:
			if value == 48 {
				return true
			}
			if i+4 < len(parts) {
				i += 4
			} else {
				i = len(parts)
			}
		}
	}
	return false
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

func narrativeWrapText(kind string) string {
	return fmt.Sprintf(
		"%s alpha bravo charlie delta echo foxtrot gulf hotel india juliet kilo tail-marker",
		kind,
	)
}

func compactTruncationText() string {
	return "tool output alpha bravo charlie delta echo foxtrot gulf hotel india juliet kilo tail-marker"
}

func normalizedStrippedPanelText(content string) string {
	lines := strings.Split(xansi.Strip(content), "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " ")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func TestFormatTokens(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		in   int
		want string
	}{
		{-5, "0"},
		{0, "0"},
		{850, "850"},
		{999, "999"},
		{1000, "1k"},
		{8123, "8.1k"},
		{12345, "12.3k"},
		{48600, "48.6k"},
		{999_000, "999k"},
		{999_999, "1M"},
		{1_000_000, "1M"},
		{1_200_000, "1.2M"},
	} {
		tc := tc
		t.Run(fmt.Sprintf("Should format %d tokens", tc.in), func(t *testing.T) {
			t.Parallel()

			if got := formatTokens(tc.in); got != tc.want {
				t.Fatalf("formatTokens(%d) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestFormatUsageTotalLabel(t *testing.T) {
	t.Parallel()

	t.Run("Should format usage total labels", func(t *testing.T) {
		t.Parallel()

		if got := formatUsageTotalLabel(nil); got != "" {
			t.Fatalf("expected empty label for nil usage, got %q", got)
		}
		if got := formatUsageTotalLabel(&model.Usage{}); got != "" {
			t.Fatalf("expected empty label for zero usage, got %q", got)
		}
		if got := formatUsageTotalLabel(&model.Usage{InputTokens: 8123, OutputTokens: 4200}); got != "12.3k tok" {
			t.Fatalf("unexpected derived total label, got %q", got)
		}
		usage := &model.Usage{
			InputTokens:  22139,
			OutputTokens: 190016,
			TotalTokens:  32490537,
			CacheReads:   31873174,
			CacheWrites:  405208,
		}
		if got := formatUsageTotalLabel(usage); got != "32.5M tok" {
			t.Fatalf("expected provider total label from real ACP usage payload, got %q", got)
		}
	})
}

func TestSidebarCardShowsNumberedTitle(t *testing.T) {
	t.Parallel()

	t.Run("Should show numbered title", func(t *testing.T) {
		t.Parallel()

		m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 160, Height: 30})
		job := &m.jobs[0]
		job.taskTitle = "Wire ACP token usage"
		job.taskType = "backend"
		job.safeName = "task_01-8824d9"

		row := m.renderSidebarItem(0, job, false)
		if got := strings.Count(row, "\n"); got != 3 {
			t.Fatalf("expected a four-line bordered card, got %d newlines: %q", got, row)
		}
		stripped := xansi.Strip(row)
		if !strings.Contains(stripped, "01") {
			t.Fatalf("expected zero-padded task number, got %q", stripped)
		}
		if !strings.Contains(stripped, "Wire ACP token usage") {
			t.Fatalf("expected human task title, got %q", stripped)
		}
		if !strings.Contains(stripped, "backend") {
			t.Fatalf("expected task type on the card meta line, got %q", stripped)
		}
		if strings.Contains(stripped, "task_01-8824d9") {
			t.Fatalf("safe name should be hidden when a title exists, got %q", stripped)
		}
		if strings.Contains(stripped, "FILES") || strings.Contains(stripped, "ISSUES") {
			t.Fatalf("files/issues meta must be gone, got %q", stripped)
		}
	})
}

func TestSidebarCardUsesOfficialTaskNumber(t *testing.T) {
	t.Parallel()

	t.Run("Should use official task number", func(t *testing.T) {
		t.Parallel()

		m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 160, Height: 30})
		job := &m.jobs[0]
		job.taskTitle = "Twentieth task"
		job.taskNumber = 20

		// The official task number must win over the 1-based slice position so a run
		// of task_01/task_04/task_20 reads as 01/04/20 rather than 01/02/03.
		stripped := xansi.Strip(m.renderSidebarItem(2, job, false))
		if !strings.Contains(stripped, "20") {
			t.Fatalf("expected official task number 20, got %q", stripped)
		}
		if strings.Contains(stripped, "03") {
			t.Fatalf("slice-position number 03 must not appear, got %q", stripped)
		}
	})
}

func TestSidebarRowFallsBackToSafeNameWithoutTitle(t *testing.T) {
	t.Parallel()

	t.Run("Should fall back to safe name without title", func(t *testing.T) {
		t.Parallel()

		m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 160, Height: 30})
		job := &m.jobs[0]
		job.taskTitle = ""
		job.safeName = "job-007"

		row := xansi.Strip(m.renderSidebarItem(6, job, false))
		if !strings.Contains(row, "07") {
			t.Fatalf("expected zero-padded number 07, got %q", row)
		}
		if !strings.Contains(row, "job-007") {
			t.Fatalf("expected safe-name fallback, got %q", row)
		}
	})
}

func TestSidebarRowHandlesThreeDigitJobNumber(t *testing.T) {
	t.Parallel()

	t.Run("Should handle three digit job number", func(t *testing.T) {
		t.Parallel()

		m := newPopulatedUIModelForTest(t, tea.WindowSizeMsg{Width: 160, Height: 30})
		job := &m.jobs[0]
		job.taskTitle = "Hundredth task"

		row := m.renderSidebarItem(149, job, false)
		lines := strings.Split(row, "\n")
		if len(lines) != 4 {
			t.Fatalf("expected a four-line bordered card for 3-digit number, got %d lines: %q", len(lines), row)
		}
		for i, line := range lines {
			if got, want := lipgloss.Width(line), m.sidebarViewport.Width(); got != want {
				t.Fatalf("expected card line %d width %d, got %d", i, want, got)
			}
		}
		if !strings.Contains(xansi.Strip(row), "150") {
			t.Fatalf("expected 1-based number 150, got %q", xansi.Strip(row))
		}
	})
}

func TestSidebarTitleShowsProgressAndAggregateTokens(t *testing.T) {
	t.Parallel()

	t.Run("Should show progress and aggregate tokens", func(t *testing.T) {
		t.Parallel()

		m := newUIModel(3)
		m.completed = 1
		m.failed = 1
		m.aggregateUsage.Add(model.Usage{TotalTokens: 48600})

		title := xansi.Strip(m.renderSidebarTitle(40))
		if !strings.Contains(title, "JOB") {
			t.Fatalf("expected sidebar title label, got %q", title)
		}
		if !strings.Contains(title, "2/3") {
			t.Fatalf("expected completed/total count, got %q", title)
		}
		if !strings.Contains(title, "48.6k tok") {
			t.Fatalf("expected aggregate provider token total, got %q", title)
		}
		if got := len(strings.Split(title, "\n")); got != sidebarHeaderRows {
			t.Fatalf("expected sidebar header to reserve %d rows, got %d: %q", sidebarHeaderRows, got, title)
		}
		if strings.Contains(title, "SYS.PIPELINE") {
			t.Fatalf("pipeline label moved out of global chrome and must not appear in sidebar title, got %q", title)
		}
	})
}
