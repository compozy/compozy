package ui

import (
	"strings"
	"testing"

	xansi "github.com/charmbracelet/x/ansi"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

// applyEventToModel drives one journal event through the same translate-then-apply
// path the live UI uses, so these tests assert on observable state rather than on
// hand-built messages.
func applyEventToModel(t *testing.T, m *uiModel, translator *uiEventTranslator, ev eventspkg.Event) {
	t.Helper()
	msg, ok := translator.translateEvent(ev)
	if !ok {
		t.Fatalf("translate %s: not handled", ev.Kind)
	}
	m.applyUIMsg(msg)
}

func newStallTestModel(total int) (*uiModel, *uiEventTranslator) {
	m := newUIModel(total)
	for idx := range total {
		m.ensureJobSlot(idx)
		m.jobs[idx].state = jobRunning
	}
	return m, newUIEventTranslator()
}

func jobStalledEvent(t *testing.T, index int, attempt int) eventspkg.Event {
	t.Helper()
	return mustRuntimeEventUITest(t, eventspkg.EventKindJobStalled, kinds.JobStalledPayload{
		JobAttemptInfo: kinds.JobAttemptInfo{Index: index, Attempt: attempt, MaxAttempts: 2},
		Reason:         "no output for 3m0s",
		LastToolCall:   "Bash go test ./...",
	})
}

func jobRetryEvent(t *testing.T, index int, attempt int) eventspkg.Event {
	t.Helper()
	return mustRuntimeEventUITest(t, eventspkg.EventKindJobRetryScheduled, kinds.JobRetryScheduledPayload{
		JobAttemptInfo: kinds.JobAttemptInfo{Index: index, Attempt: attempt, MaxAttempts: 2},
		Reason:         "stall recovery",
	})
}

func jobParkedEvent(t *testing.T, index int) eventspkg.Event {
	t.Helper()
	return mustRuntimeEventUITest(t, eventspkg.EventKindJobParked, kinds.JobParkedPayload{
		JobAttemptInfo:  kinds.JobAttemptInfo{Index: index, Attempt: 2, MaxAttempts: 2},
		Reason:          "no output for 3m0s",
		LastToolCall:    "Bash go test ./...",
		LastProgressSeq: 42,
		WorktreePath:    "/tmp/wt/task_02",
		LogPath:         "/tmp/logs/task_02.out.log",
	})
}

func jobCompletedEvent(t *testing.T, index int) eventspkg.Event {
	t.Helper()
	return mustRuntimeEventUITest(t, eventspkg.EventKindJobCompleted, kinds.JobCompletedPayload{
		JobAttemptInfo: kinds.JobAttemptInfo{Index: index, Attempt: 1, MaxAttempts: 2},
		DurationMs:     1500,
	})
}

func TestStalledJobRendersStalledThenRetryingState(t *testing.T) {
	t.Parallel()

	t.Run("Should render the stalled state on job.stalled", func(t *testing.T) {
		t.Parallel()

		m, translator := newStallTestModel(1)
		applyEventToModel(t, m, translator, jobStalledEvent(t, 0, 1))

		job := &m.jobs[0]
		if job.state != jobStalled {
			t.Fatalf("expected jobStalled, got %v", job.state)
		}
		if !job.stalled {
			t.Fatal("expected the sticky stalled flag to be set")
		}
		if got, want := m.jobStateIcon(job.state), jobIconStalled; got != want {
			t.Fatalf("expected icon %q, got %q", want, got)
		}
		if got := m.sidebarTimeString(job); got != "stalled" {
			t.Fatalf("expected sidebar time string %q, got %q", "stalled", got)
		}
		if !strings.Contains(job.stallReason, "no output for 3m0s") ||
			!strings.Contains(job.stallReason, "Bash go test ./...") {
			t.Fatalf("expected reason and last tool call in %q", job.stallReason)
		}
	})

	t.Run("Should render the retrying state after job.retry_scheduled", func(t *testing.T) {
		t.Parallel()

		m, translator := newStallTestModel(1)
		applyEventToModel(t, m, translator, jobStalledEvent(t, 0, 1))
		applyEventToModel(t, m, translator, jobRetryEvent(t, 0, 2))

		job := &m.jobs[0]
		if job.state != jobRetrying {
			t.Fatalf("expected jobRetrying, got %v", job.state)
		}
		if got, want := m.jobStateIcon(job.state), jobIconRetry; got != want {
			t.Fatalf("expected icon %q, got %q", want, got)
		}
		if !job.stalled {
			t.Fatal("expected the stalled flag to survive the retry so recovery stays derivable")
		}
		if !m.hasActiveJobs() {
			t.Fatal("expected a retrying job to keep the run active")
		}
	})
}

func TestParkedJobRendersTerminalParkedState(t *testing.T) {
	t.Parallel()

	m, translator := newStallTestModel(1)
	applyEventToModel(t, m, translator, jobStalledEvent(t, 0, 1))
	applyEventToModel(t, m, translator, jobRetryEvent(t, 0, 2))
	applyEventToModel(t, m, translator, jobParkedEvent(t, 0))

	job := &m.jobs[0]
	if job.state != jobParked {
		t.Fatalf("expected jobParked, got %v", job.state)
	}
	if got, want := m.jobStateIcon(job.state), jobIconParked; got != want {
		t.Fatalf("expected icon %q, got %q", want, got)
	}
	if got := m.sidebarTimeString(job); got != "parked" {
		t.Fatalf("expected sidebar time string %q, got %q", "parked", got)
	}
	if got, want := job.worktreePath, "/tmp/wt/task_02"; got != want {
		t.Fatalf("expected preserved worktree %q, got %q", want, got)
	}
	if m.parked != 1 {
		t.Fatalf("expected parked=1, got %d", m.parked)
	}
	if m.completed != 0 || m.failed != 0 {
		t.Fatalf("parked must not count as completed or failed, got completed=%d failed=%d", m.completed, m.failed)
	}
	if !m.isRunComplete() {
		t.Fatal("expected a parked job to settle the run")
	}
	if m.hasActiveJobs() {
		t.Fatal("expected no active jobs after a park")
	}
	if got := m.composerDisabledLabel(job); got != "Task parked - needs attention" {
		t.Fatalf("unexpected composer label %q", got)
	}
}

func TestSummaryCountsRecoveredSeparatelyFromPlainCompletion(t *testing.T) {
	t.Parallel()

	t.Run("Should count a stalled-then-completed job as recovered", func(t *testing.T) {
		t.Parallel()

		m, translator := newStallTestModel(1)
		applyEventToModel(t, m, translator, jobStalledEvent(t, 0, 1))
		applyEventToModel(t, m, translator, jobRetryEvent(t, 0, 2))
		applyEventToModel(t, m, translator, jobCompletedEvent(t, 0))

		if m.completed != 1 {
			t.Fatalf("expected completed=1, got %d", m.completed)
		}
		if m.recovered != 1 {
			t.Fatalf("expected recovered=1, got %d", m.recovered)
		}
		if m.parked != 0 {
			t.Fatalf("expected parked=0, got %d", m.parked)
		}
	})

	t.Run("Should not count a plain completion as recovered", func(t *testing.T) {
		t.Parallel()

		m, translator := newStallTestModel(1)
		applyEventToModel(t, m, translator, jobCompletedEvent(t, 0))

		if m.completed != 1 {
			t.Fatalf("expected completed=1, got %d", m.completed)
		}
		if m.recovered != 0 {
			t.Fatalf("expected recovered=0 for a plain completion, got %d", m.recovered)
		}
	})

	t.Run("Should count one recovered and one parked across a batch", func(t *testing.T) {
		t.Parallel()

		m, translator := newStallTestModel(3)
		applyEventToModel(t, m, translator, jobCompletedEvent(t, 0))
		applyEventToModel(t, m, translator, jobStalledEvent(t, 1, 1))
		applyEventToModel(t, m, translator, jobRetryEvent(t, 1, 2))
		applyEventToModel(t, m, translator, jobCompletedEvent(t, 1))
		applyEventToModel(t, m, translator, jobStalledEvent(t, 2, 1))
		applyEventToModel(t, m, translator, jobRetryEvent(t, 2, 2))
		applyEventToModel(t, m, translator, jobParkedEvent(t, 2))

		if m.completed != 2 || m.recovered != 1 || m.parked != 1 || m.failed != 0 {
			t.Fatalf(
				"expected completed=2 recovered=1 parked=1 failed=0, got completed=%d recovered=%d parked=%d failed=%d",
				m.completed, m.recovered, m.parked, m.failed,
			)
		}
		if !m.isRunComplete() {
			t.Fatal("expected the batch to settle with one recovered and one parked job")
		}
	})
}

func TestSummaryBoxReportsRecoveredAndParked(t *testing.T) {
	t.Parallel()

	t.Run("Should report zero recovered and parked for a run with no stalls", func(t *testing.T) {
		t.Parallel()

		m, translator := newStallTestModel(2)
		applyEventToModel(t, m, translator, jobCompletedEvent(t, 0))
		applyEventToModel(t, m, translator, jobCompletedEvent(t, 1))

		box := xansi.Strip(m.renderSummaryMainBox(80))
		for _, want := range []string{"SUCCEEDED 2", "RECOVERED 0", "PARKED    0", "FAILED    0", "TOTAL     2"} {
			if !strings.Contains(box, want) {
				t.Fatalf("expected summary box to contain %q, got:\n%s", want, box)
			}
		}
		if !strings.Contains(box, "All Jobs Complete: 2/2 succeeded") {
			t.Fatalf("expected the clean-run header, got:\n%s", box)
		}
	})

	t.Run("Should report recovered and parked counts when recovery happened", func(t *testing.T) {
		t.Parallel()

		m, translator := newStallTestModel(3)
		applyEventToModel(t, m, translator, jobCompletedEvent(t, 0))
		applyEventToModel(t, m, translator, jobStalledEvent(t, 1, 1))
		applyEventToModel(t, m, translator, jobCompletedEvent(t, 1))
		applyEventToModel(t, m, translator, jobStalledEvent(t, 2, 1))
		applyEventToModel(t, m, translator, jobParkedEvent(t, 2))

		box := xansi.Strip(m.renderSummaryMainBox(80))
		for _, want := range []string{"SUCCEEDED 2", "RECOVERED 1", "PARKED    1", "FAILED    0", "TOTAL     3"} {
			if !strings.Contains(box, want) {
				t.Fatalf("expected summary box to contain %q, got:\n%s", want, box)
			}
		}
		if !strings.Contains(box, "1 recovered") || !strings.Contains(box, "1 parked") {
			t.Fatalf("expected the header to name recovered and parked jobs, got:\n%s", box)
		}
	})
}

// TestZeroStallRunLeavesPerJobLayoutUnchanged pins requirement 4: introducing the
// stalled and parked states must not shift a single rendered row for a run that
// never stalls.
func TestZeroStallRunLeavesPerJobLayoutUnchanged(t *testing.T) {
	t.Parallel()

	m, translator := newStallTestModel(2)
	before := m.renderSidebarItem(0, &m.jobs[0], true)
	applyEventToModel(t, m, translator, jobCompletedEvent(t, 1))
	m.jobs[0].sidebarCacheValid = false
	after := m.renderSidebarItem(0, &m.jobs[0], true)

	if before != after {
		t.Fatalf("expected an untouched job card to render identically:\nbefore=%q\nafter=%q", before, after)
	}
	if m.recovered != 0 || m.parked != 0 {
		t.Fatalf("expected recovered=0 parked=0, got recovered=%d parked=%d", m.recovered, m.parked)
	}
}

func TestStallDetailText(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		reason       string
		lastToolCall string
		want         string
	}{
		{name: "Should render nothing when neither field is set"},
		{
			name:         "Should render only the tool call when the reason is blank",
			lastToolCall: "Bash go test ./...",
			want:         "last tool call: Bash go test ./...",
		},
		{
			name:   "Should render only the reason when no tool call was observed",
			reason: "no output for 3m0s",
			want:   "no output for 3m0s",
		},
		{
			name:         "Should join the reason and the tool call",
			reason:       "no output for 3m0s",
			lastToolCall: "Bash go test ./...",
			want:         "no output for 3m0s; last tool call: Bash go test ./...",
		},
		{
			name:         "Should treat whitespace-only fields as absent",
			reason:       "   ",
			lastToolCall: "\t",
			want:         "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := stallDetailText(tc.reason, tc.lastToolCall); got != tc.want {
				t.Fatalf("stallDetailText(%q, %q) = %q, want %q", tc.reason, tc.lastToolCall, got, tc.want)
			}
		})
	}
}

// TestStallMetaLabel pins the timeline meta line to the two states where the user
// needs the stall explanation: the live stall and the terminal park. Every other
// state, including the retry that follows a stall, stays silent.
func TestStallMetaLabel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		job  *uiJob
		want string
	}{
		{name: "Should render nothing for a nil job"},
		{
			name: "Should render nothing when no stall reason was recorded",
			job:  &uiJob{state: jobStalled},
		},
		{
			name: "Should render nothing when the reason is whitespace only",
			job:  &uiJob{state: jobParked, stallReason: "  "},
		},
		{
			name: "Should label a stalled job",
			job:  &uiJob{state: jobStalled, stallReason: "no output for 3m0s"},
			want: "stalled: no output for 3m0s",
		},
		{
			name: "Should label a parked job",
			job:  &uiJob{state: jobParked, stallReason: "no output for 3m0s"},
			want: "parked: no output for 3m0s",
		},
		{
			name: "Should stay silent on the retry that follows a stall",
			job:  &uiJob{state: jobRetrying, stallReason: "no output for 3m0s"},
			want: "",
		},
		{
			name: "Should stay silent once a stalled job recovers",
			job:  &uiJob{state: jobSuccess, stallReason: "no output for 3m0s"},
			want: "",
		},
		{
			name: "Should truncate a long reason to the timeline budget",
			job:  &uiJob{state: jobStalled, stallReason: strings.Repeat("x", 100)},
			want: "stalled: " + truncateString(strings.Repeat("x", 100), 72),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := stallMetaLabel(tc.job); got != tc.want {
				t.Fatalf("stallMetaLabel() = %q, want %q", got, tc.want)
			}
		})
	}
}
