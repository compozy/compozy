package ui

import (
	"strings"
	"testing"

	xansi "github.com/charmbracelet/x/ansi"
)

// parkedDetailModel drives one job through stall -> retry -> park using the same
// translate-then-apply path the live UI uses, so the assertions below read the
// state a returning user actually sees.
func parkedDetailModel(t *testing.T) *uiModel {
	t.Helper()
	m, translator := newStallTestModel(1)
	m.jobs[0].taskTitle = "task_02"
	applyEventToModel(t, m, translator, jobStalledEvent(t, 0, 1))
	applyEventToModel(t, m, translator, jobRetryEvent(t, 0, 2))
	applyEventToModel(t, m, translator, jobParkedEvent(t, 0))
	return m
}

func TestParkedJobCarriesFullTriageDetail(t *testing.T) {
	t.Parallel()

	job := &parkedDetailModel(t).jobs[0]
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"reason", job.stallReason, "no output for 3m0s"},
		{"last tool call", job.stallLastToolCall, "Bash go test ./..."},
		{"worktree", job.worktreePath, "/tmp/wt/task_02"},
		{"log", job.parkLogPath, "/tmp/logs/task_02.out.log"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.got != tc.want {
				t.Fatalf("%s = %q, want %q", tc.name, tc.got, tc.want)
			}
		})
	}
	if job.parkProgressSeq != 42 {
		t.Fatalf("last progress seq = %d, want 42", job.parkProgressSeq)
	}
}

func TestSummaryParkedBoxRendersActionableDetail(t *testing.T) {
	t.Parallel()

	m := parkedDetailModel(t)
	parked := m.parkedJobs()
	if len(parked) != 1 {
		t.Fatalf("expected exactly one parked job, got %d", len(parked))
	}

	box := xansi.Strip(m.renderSummaryParkedBox(80, parked))
	for _, want := range []string{
		"RUN.PARKED",
		statusLabelParked,
		"task_02",
		"no output for 3m0s",
		"Bash go test ./...",
		"seq 42",
		"/tmp/wt/task_02",
		"/tmp/logs/task_02.out.log",
	} {
		if !strings.Contains(box, want) {
			t.Fatalf("parked box missing %q:\n%s", want, box)
		}
	}
}

// The parked panel is only rendered when a job actually parked; a clean run must
// not grow an empty triage box.
func TestSummaryViewOmitsParkedBoxWithoutParkedJobs(t *testing.T) {
	t.Parallel()

	m, _ := newStallTestModel(1)
	if got := m.parkedJobs(); len(got) != 0 {
		t.Fatalf("expected no parked jobs, got %d", len(got))
	}
	m.width = 100
	m.height = 40
	if view := xansi.Strip(m.renderSummaryView().Content); strings.Contains(view, "RUN.PARKED") {
		t.Fatalf("summary view rendered a parked box for a run with no parks:\n%s", view)
	}
}

// A park that never recorded durable progress, or whose log was never created,
// must not render dangling labels with empty values.
func TestParkedDetailLinesSkipEmptyFields(t *testing.T) {
	t.Parallel()

	lines := parkedDetailLines(&uiJob{stallReason: "no output for 3m0s"})
	if len(lines) != 1 {
		t.Fatalf("expected only the reason line, got %#v", lines)
	}
	joined := xansi.Strip(strings.Join(lines, "\n"))
	if !strings.Contains(joined, "reason") || !strings.Contains(joined, "no output for 3m0s") {
		t.Fatalf("unexpected reason line %q", joined)
	}
	for _, unwanted := range []string{"progress", "worktree", "log", "last call"} {
		if strings.Contains(joined, unwanted) {
			t.Fatalf("expected %q to be skipped, got %q", unwanted, joined)
		}
	}
}

func TestParkedProgressValue(t *testing.T) {
	t.Parallel()

	if got := parkedProgressValue(0); got != "" {
		t.Fatalf("sequence zero must render nothing, got %q", got)
	}
	if got, want := parkedProgressValue(1284), "seq 1284"; got != want {
		t.Fatalf("parkedProgressValue(1284) = %q, want %q", got, want)
	}
}
