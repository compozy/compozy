package ui

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
)

func newParallelTestModel(t *testing.T, size tea.WindowSizeMsg, taskNumbers ...int) *uiModel {
	t.Helper()
	m := newUIModel(len(taskNumbers))
	m.cfg = &config{IDE: model.IDEClaude, Model: "sonnet-4.5"}
	for i, num := range taskNumbers {
		m.handleJobQueued(&jobQueuedMsg{
			Index:      i,
			TaskNumber: num,
			TaskTitle:  fmt.Sprintf("Task %d", num),
			TaskType:   "backend",
			SafeName:   fmt.Sprintf("task_%02d-safe", num),
		})
		m.jobs[i].state = jobRunning
	}
	m.handleWindowSize(size)
	return m
}

func taskParallelEvent(t *testing.T, kind events.EventKind, payload kinds.TaskParallelPayload) events.Event {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return events.Event{Kind: kind, Payload: raw}
}

func taskParallelPlanEvent(t *testing.T, payload kinds.TaskParallelPlanPayload) events.Event {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal plan payload: %v", err)
	}
	return events.Event{Kind: events.EventKindTaskParallelPlanStarted, Payload: raw}
}

func TestTranslateParallelEvent(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		kind   events.EventKind
		input  kinds.TaskParallelPayload
		assert func(t *testing.T, msg uiMsg)
	}{
		{
			name:  "Should map wave_started to a wave assignment message",
			kind:  events.EventKindTaskParallelWaveStarted,
			input: kinds.TaskParallelPayload{WaveIndex: 1, WaveTotal: 3, TaskID: "task_02", IntegrationBranch: "b"},
			assert: func(t *testing.T, msg uiMsg) {
				got, ok := msg.(parallelWaveStartedMsg)
				if !ok {
					t.Fatalf("msg type = %T, want parallelWaveStartedMsg", msg)
				}
				if got.WaveIndex != 1 || got.WaveTotal != 3 || got.TaskID != "task_02" {
					t.Fatalf("unexpected payload: %#v", got)
				}
			},
		},
		{
			name: "Should map conflict_detected to a non-resolving conflict message",
			kind: events.EventKindTaskParallelConflictDetected,
			input: kinds.TaskParallelPayload{
				WaveIndex:     0,
				TaskID:        "task_01",
				ConflictFiles: []string{"a.go"},
				Attempt:       1,
				MaxAttempts:   3,
			},
			assert: func(t *testing.T, msg uiMsg) {
				got, ok := msg.(parallelConflictMsg)
				if !ok {
					t.Fatalf("msg type = %T, want parallelConflictMsg", msg)
				}
				if got.Resolving {
					t.Fatal("conflict_detected must not be flagged resolving")
				}
				if len(got.Files) != 1 || got.Files[0] != "a.go" || got.MaxAttempts != 3 {
					t.Fatalf("unexpected payload: %#v", got)
				}
			},
		},
		{
			name:  "Should map conflict_resolving to a resolving conflict message",
			kind:  events.EventKindTaskParallelConflictResolving,
			input: kinds.TaskParallelPayload{WaveIndex: 0, TaskID: "task_01", Attempt: 2, MaxAttempts: 3},
			assert: func(t *testing.T, msg uiMsg) {
				got, ok := msg.(parallelConflictMsg)
				if !ok || !got.Resolving {
					t.Fatalf("expected resolving conflict message, got %#v", msg)
				}
			},
		},
		{
			name:  "Should map merged to a merged message",
			kind:  events.EventKindTaskParallelMerged,
			input: kinds.TaskParallelPayload{WaveIndex: 0, TaskID: "task_01", Status: "merged"},
			assert: func(t *testing.T, msg uiMsg) {
				got, ok := msg.(parallelMergedMsg)
				if !ok || got.TaskID != "task_01" || got.Status != "merged" {
					t.Fatalf("expected merged message, got %#v", msg)
				}
			},
		},
		{
			name:  "Should map rolled_back to a rollback message",
			kind:  events.EventKindTaskParallelRolledBack,
			input: kinds.TaskParallelPayload{WaveIndex: 2, IntegrationBranch: "compozy/parallel-x"},
			assert: func(t *testing.T, msg uiMsg) {
				got, ok := msg.(parallelRolledBackMsg)
				if !ok || got.IntegrationBranch != "compozy/parallel-x" {
					t.Fatalf("expected rollback message, got %#v", msg)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			msg, ok := translateParallelEvent(taskParallelEvent(t, tc.kind, tc.input))
			if !ok {
				t.Fatalf("translateParallelEvent(%q) returned ok=false", tc.kind)
			}
			tc.assert(t, msg)
		})
	}

	t.Run("Should map plan_started to a full graph plan message", func(t *testing.T) {
		t.Parallel()

		msg, ok := translateParallelEvent(taskParallelPlanEvent(t, kinds.TaskParallelPlanPayload{
			Workflow:          "demo",
			IntegrationBranch: "compozy/parallel-x",
			ParallelLimit:     2,
			Tasks: []kinds.TaskParallelPlanTask{
				{ID: "task_01", Number: 1, Title: "First", File: "task_01.md", WaveIndex: 0},
				{
					ID:           "task_02",
					Number:       2,
					Title:        "Second",
					File:         "task_02.md",
					Dependencies: []string{"task_01"},
					WaveIndex:    1,
				},
			},
			Waves: []kinds.TaskParallelPlanWave{
				{Index: 0, TaskIDs: []string{"task_01"}},
				{Index: 1, TaskIDs: []string{"task_02"}},
			},
		}))
		if !ok {
			t.Fatal("translateParallelEvent(plan_started) returned ok=false")
		}
		got, ok := msg.(parallelPlanStartedMsg)
		if !ok {
			t.Fatalf("msg type = %T, want parallelPlanStartedMsg", msg)
		}
		if got.Workflow != "demo" || got.ParallelLimit != 2 || len(got.Tasks) != 2 || len(got.Waves) != 2 {
			t.Fatalf("unexpected plan message: %#v", got)
		}
	})
}

func TestParallelSidebarGroupsCardsByWave(t *testing.T) {
	t.Parallel()

	m := newParallelTestModel(t, tea.WindowSizeMsg{Width: 120, Height: 30}, 1, 2, 3)
	for _, msg := range []parallelWaveStartedMsg{
		{WaveIndex: 0, WaveTotal: 2, TaskID: "task_01"},
		{WaveIndex: 0, WaveTotal: 2, TaskID: "task_02"},
		{WaveIndex: 1, WaveTotal: 2, TaskID: "task_03"},
	} {
		m.applyUIMsg(msg)
	}

	content := m.sidebarContent
	wave1 := strings.Index(content, "WAVE 1")
	wave2 := strings.Index(content, "WAVE 2")
	if wave1 < 0 || wave2 < 0 {
		t.Fatalf("expected both wave headers, got:\n%s", content)
	}
	if wave1 >= wave2 {
		t.Fatalf("WAVE 1 header must precede WAVE 2:\n%s", content)
	}
	task1 := strings.Index(content, "Task 1")
	task2 := strings.Index(content, "Task 2")
	task3 := strings.Index(content, "Task 3")
	if task1 <= wave1 || task1 >= wave2 || task2 <= wave1 || task2 >= wave2 {
		t.Fatalf("tasks 1 and 2 must render under WAVE 1:\n%s", content)
	}
	if task3 <= wave2 {
		t.Fatalf("task 3 must render under WAVE 2:\n%s", content)
	}
}

func TestParallelSidebarRendersPlannedFutureWaves(t *testing.T) {
	t.Parallel()

	t.Run("Should render pending waves from the plan before they emit task events", func(t *testing.T) {
		t.Parallel()

		m := newParallelTestModel(t, tea.WindowSizeMsg{Width: 120, Height: 30}, 1)
		m.applyUIMsg(parallelPlanStartedMsg{
			IntegrationBranch: "compozy/parallel-x",
			Tasks: []parallelPlanTask{
				{ID: "task_01", Number: 1, Title: "Task 1", File: "task_01.md", WaveIndex: 0},
				{
					ID:           "task_02",
					Number:       2,
					Title:        "Task 2",
					File:         "task_02.md",
					Dependencies: []string{"task_01"},
					WaveIndex:    1,
				},
			},
			Waves: []parallelPlanWave{
				{Index: 0, TaskIDs: []string{"task_01"}},
				{Index: 1, TaskIDs: []string{"task_02"}},
			},
		})
		m.applyUIMsg(parallelWaveStartedMsg{WaveIndex: 0, WaveTotal: 2, TaskID: "task_01"})

		content := xansi.Strip(m.sidebarContent)
		if !strings.Contains(content, "WAVE 1") || !strings.Contains(content, "WAVE 2") {
			t.Fatalf("expected both planned waves, got:\n%s", content)
		}
		wave2 := strings.Index(content, "WAVE 2")
		task2 := strings.Index(content, "Task 2")
		if task2 <= wave2 {
			t.Fatalf("planned task 2 must render under pending WAVE 2, got:\n%s", content)
		}
		if strings.Contains(strings.ToLower(content), "blocked") {
			t.Fatalf("pending planned waves must not render as blocked, got:\n%s", content)
		}
	})
}

func TestParallelIntegrationPaneRendersOnlyWhenActionable(t *testing.T) {
	t.Parallel()

	t.Run("Should hide the pane while running and expand on conflict", func(t *testing.T) {
		t.Parallel()

		m := newParallelTestModel(t, tea.WindowSizeMsg{Width: 120, Height: 30}, 1, 2)
		m.applyUIMsg(
			parallelWaveStartedMsg{
				WaveIndex:         0,
				WaveTotal:         2,
				TaskID:            "task_01",
				IntegrationBranch: "compozy/parallel-x",
			},
		)

		collapsed := m.renderIntegrationContent(60)
		if collapsed != "" {
			t.Fatalf("INTEGRATION pane must be hidden during normal running, got:\n%s", collapsed)
		}

		m.applyUIMsg(parallelConflictMsg{
			WaveIndex:   0,
			TaskID:      "task_01",
			Files:       []string{"story.txt"},
			Attempt:     1,
			MaxAttempts: 3,
		})
		expanded := m.renderIntegrationContent(60)
		if !strings.Contains(expanded, "\n") {
			t.Fatalf("INTEGRATION pane must expand on conflict, got:\n%s", expanded)
		}
		if !strings.Contains(expanded, "CONFLICT") {
			t.Fatalf("expanded pane missing conflict banner:\n%s", expanded)
		}
		if !strings.Contains(expanded, "story.txt") {
			t.Fatalf("expanded pane missing conflicted files:\n%s", expanded)
		}
		if !strings.Contains(expanded, "1/3") {
			t.Fatalf("expanded pane missing attempt counter:\n%s", expanded)
		}
	})
}

func TestParallelConflictResolvingThenMergedUpdatesPaneAndWaveHeader(t *testing.T) {
	t.Parallel()

	m := newParallelTestModel(t, tea.WindowSizeMsg{Width: 120, Height: 30}, 1)
	m.applyUIMsg(parallelWaveStartedMsg{WaveIndex: 0, WaveTotal: 1, TaskID: "task_01"})
	m.applyUIMsg(parallelMergeStartedMsg{WaveIndex: 0, WaveTotal: 1})
	m.applyUIMsg(parallelConflictMsg{
		WaveIndex:   0,
		TaskID:      "task_01",
		Files:       []string{"story.txt"},
		Attempt:     1,
		MaxAttempts: 3,
		Resolving:   true,
	})
	if m.parallel.phase != integrationPhaseResolving {
		t.Fatalf("phase = %d, want resolving", m.parallel.phase)
	}
	if !strings.Contains(m.renderIntegrationContent(60), "RESOLVE") {
		t.Fatalf("expected RESOLVE banner during resolving, got:\n%s", m.renderIntegrationContent(60))
	}

	m.applyUIMsg(parallelMergedMsg{WaveIndex: 0, TaskID: "task_01", Status: "merged"})
	if m.parallel.conflict != nil {
		t.Fatal("merged event must clear the active conflict")
	}

	m.applyUIMsg(parallelWaveCompletedMsg{WaveIndex: 0, WaveTotal: 1})
	if got := m.parallel.waveStatusAt(0); got != waveStatusMerged {
		t.Fatalf("wave 0 status = %d, want merged", got)
	}
	if !strings.Contains(m.sidebarContent, "merged") {
		t.Fatalf("sidebar wave header must read merged:\n%s", m.sidebarContent)
	}
}

func TestParallelTaskCompletedRequiresCanonicalIntegratedStatus(t *testing.T) {
	t.Parallel()

	t.Run("Should reject non-canonical integrated status", func(t *testing.T) {
		t.Parallel()

		m := newParallelTestModel(t, tea.WindowSizeMsg{Width: 120, Height: 30}, 1)
		m.applyUIMsg(parallelTaskCompletedMsg{WaveIndex: 0, TaskID: "task_01", Status: " merged "})

		if got := m.jobs[0].state; got != jobFailed {
			t.Fatalf("task state = %v, want failed for non-canonical status", got)
		}
	})
}

func TestParallelRolledBackRendersWithoutPanic(t *testing.T) {
	t.Parallel()

	m := newParallelTestModel(t, tea.WindowSizeMsg{Width: 120, Height: 30}, 1, 2)
	m.applyUIMsg(parallelWaveStartedMsg{WaveIndex: 0, WaveTotal: 1, TaskID: "task_01"})
	m.applyUIMsg(parallelRolledBackMsg{WaveIndex: 0, IntegrationBranch: "compozy/parallel-x"})

	if m.parallel.phase != integrationPhaseRolledBack {
		t.Fatalf("phase = %d, want rolled back", m.parallel.phase)
	}
	pane := m.renderIntegrationContent(60)
	if !strings.Contains(pane, "ROLLED BACK") {
		t.Fatalf("expected rollback banner, got:\n%s", pane)
	}
	if view := m.View().Content; view == "" {
		t.Fatal("expected a non-empty view after rollback")
	}
}

func TestWaveStatusVisualsCoverEveryStatus(t *testing.T) {
	t.Parallel()
	for _, status := range []waveStatus{
		waveStatusPending,
		waveStatusRunning,
		waveStatusMerging,
		waveStatusConflict,
		waveStatusMerged,
	} {
		if waveStatusGlyph(status) == "" {
			t.Fatalf("missing glyph for status %d", status)
		}
		if waveStatusLabel(status) == "" {
			t.Fatalf("missing label for status %d", status)
		}
		if waveStatusColor(status) == nil {
			t.Fatalf("missing color for status %d", status)
		}
	}
}

func TestIntegrationBorderColorTracksPhase(t *testing.T) {
	t.Parallel()
	m := newUIModel(1)
	if got := m.integrationBorderColor(); got != colorBorder {
		t.Fatalf("nil parallel border = %v, want default border", got)
	}
	cases := []struct {
		phase integrationPhase
		want  bool
	}{
		{integrationPhaseRunning, false},
		{integrationPhaseConflict, true},
		{integrationPhaseResolving, true},
		{integrationPhaseRolledBack, true},
		{integrationPhaseDone, true},
	}
	for _, tc := range cases {
		m.parallel = &parallelView{phase: tc.phase}
		if got := m.integrationBorderColor(); (got != colorBorder) != tc.want {
			t.Fatalf("phase %d border distinct=%v, want %v", tc.phase, got != colorBorder, tc.want)
		}
	}
}

func TestParallelResolverLinesRenderInPane(t *testing.T) {
	t.Parallel()
	m := newParallelTestModel(t, tea.WindowSizeMsg{Width: 120, Height: 30}, 1)
	m.applyUIMsg(parallelWaveStartedMsg{WaveIndex: 0, WaveTotal: 1, TaskID: "task_01"})
	m.applyUIMsg(
		parallelConflictMsg{
			WaveIndex:   0,
			TaskID:      "task_01",
			Files:       []string{"x.go"},
			Attempt:     1,
			MaxAttempts: 3,
			Resolving:   true,
		},
	)
	m.parallel.resolverLines = []string{"applying patch to x.go", "running make verify"}
	pane := m.renderIntegrationContent(60)
	if !strings.Contains(pane, "applying patch to x.go") {
		t.Fatalf("expected streamed resolver line, got:\n%s", pane)
	}
}

func TestParallelMultiWaveCompletionMarksDone(t *testing.T) {
	t.Parallel()
	m := newParallelTestModel(t, tea.WindowSizeMsg{Width: 120, Height: 30}, 1, 2)
	m.applyUIMsg(parallelWaveStartedMsg{WaveIndex: 0, WaveTotal: 2, TaskID: "task_01"})
	m.applyUIMsg(parallelWaveStartedMsg{WaveIndex: 1, WaveTotal: 2, TaskID: "task_02"})
	m.applyUIMsg(parallelWaveCompletedMsg{WaveIndex: 0, WaveTotal: 2})
	if m.parallel.phase == integrationPhaseDone {
		t.Fatal("phase must not be done until every wave merges")
	}
	m.applyUIMsg(parallelWaveCompletedMsg{WaveIndex: 1, WaveTotal: 2})
	if m.parallel.phase != integrationPhaseDone {
		t.Fatalf("phase = %d, want done after all waves merged", m.parallel.phase)
	}
	if !allWavesMerged(m.parallel) {
		t.Fatal("allWavesMerged should report true")
	}
}

func TestParallelWaveHeaderGlyphsForStatuses(t *testing.T) {
	t.Parallel()
	m := newParallelTestModel(t, tea.WindowSizeMsg{Width: 120, Height: 30}, 1, 2, 3, 4, 5)
	for _, num := range []int{1, 2, 3, 4, 5} {
		m.applyUIMsg(parallelWaveStartedMsg{WaveIndex: 0, WaveTotal: 1, TaskID: fmt.Sprintf("task_%02d", num)})
	}
	// Five concurrent running jobs: the running glyph repeats but is capped at 4.
	if got := m.waveHeaderGlyphs(0, waveStatusRunning); len([]rune(got)) > 4 {
		t.Fatalf("running glyphs = %q, want capped at 4", got)
	}
	for _, status := range []waveStatus{waveStatusMerging, waveStatusConflict, waveStatusMerged, waveStatusPending} {
		if m.waveHeaderGlyphs(0, status) == "" {
			t.Fatalf("missing header glyph for status %d", status)
		}
	}
}

func TestParallelIntegrationPaneStaysWithinBoundsAtMinSize(t *testing.T) {
	t.Parallel()

	size := tea.WindowSizeMsg{Width: 80, Height: 24}
	m := newParallelTestModel(t, size, 1, 2)
	// Drive the busiest pane state (expanded conflict with files + resolver line).
	m.applyUIMsg(parallelWaveStartedMsg{WaveIndex: 0, WaveTotal: 2, TaskID: "task_01"})
	m.applyUIMsg(parallelWaveStartedMsg{WaveIndex: 1, WaveTotal: 2, TaskID: "task_02"})
	m.applyUIMsg(parallelMergeStartedMsg{WaveIndex: 0, WaveTotal: 2})
	m.applyUIMsg(parallelConflictMsg{
		WaveIndex:   0,
		TaskID:      "task_01",
		Files:       []string{"a/very/long/path/story.txt", "another/file.go"},
		Attempt:     2,
		MaxAttempts: 3,
		Resolving:   true,
	})

	if got, want := lipgloss.Height(m.View().Content), m.height; got != want {
		t.Fatalf("view height = %d, want %d (no overflow at minimum size)", got, want)
	}
	if got := lipgloss.Width(m.View().Content); got > m.width {
		t.Fatalf("view width = %d, want <= %d", got, m.width)
	}
}
