package executor

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/journal"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

// stallResult is the attempt outcome a frozen agent produces: the watchdog cancels
// the attempt and HandleSessionTimeout tags it retryable + stalled.
func stallResult(jb *job, lastToolCall string) jobAttemptResult {
	return jobAttemptResult{
		Status:    attemptStatusTimeout,
		ExitCode:  -2,
		Retryable: true,
		Stalled:   true,
		Failure: &failInfo{
			CodeFile: jb.CodeFileLabel(),
			ExitCode: -2,
			OutLog:   jb.OutLog,
			ErrLog:   jb.ErrLog,
			Err:      errors.New("activity timeout: no output received for 3m0s"),
		},
		LastToolCall: lastToolCall,
	}
}

func successResult() jobAttemptResult {
	return jobAttemptResult{Status: attemptStatusSuccess}
}

func ordinaryFailureResult(jb *job) jobAttemptResult {
	return jobAttemptResult{
		Status:    attemptStatusFailure,
		ExitCode:  1,
		Retryable: true,
		Failure: &failInfo{
			CodeFile: jb.CodeFileLabel(),
			ExitCode: 1,
			OutLog:   jb.OutLog,
			ErrLog:   jb.ErrLog,
			Err:      errors.New("boom"),
		},
	}
}

func requireStallGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
}

func mustStallGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, string(out))
	}
}

// initStallGitRepo seeds a committed, clean workspace: the state a freshly
// allocated per-child worktree is in when its agent starts.
func initStallGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mustStallGit(t, root, "init", "-q", "-b", "main")
	mustStallGit(t, root, "config", "user.email", "stall@example.com")
	mustStallGit(t, root, "config", "user.name", "Stall Tester")
	mustStallGit(t, root, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# initial\n"), 0o600); err != nil {
		t.Fatalf("seed README: %v", err)
	}
	mustStallGit(t, root, "add", "README.md")
	mustStallGit(t, root, "commit", "-q", "-m", "initial")
	return root
}

type stallHarness struct {
	execCtx *jobExecutionContext
	job     *job
	runner  *jobRunner
	events  <-chan eventspkg.Event
	root    string
}

type stallHarnessOptions struct {
	workspaceRoot string
	maxRetries    int
	stallRetries  int
	stallEnabled  bool
	totalJobs     int
}

func newStallHarness(t *testing.T, opts stallHarnessOptions, attempts ...jobAttemptResult) *stallHarness {
	t.Helper()

	root := opts.workspaceRoot
	if root == "" {
		root = t.TempDir()
	}
	runArtifacts := model.NewRunArtifacts(t.TempDir(), "stall-run")
	if err := os.MkdirAll(filepath.Dir(runArtifacts.EventsPath), 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	bus := eventspkg.New[eventspkg.Event](64)
	_, ch, unsubscribe := bus.Subscribe()
	runJournal, err := journal.Open(runArtifacts.EventsPath, bus, 64)
	if err != nil {
		t.Fatalf("open journal: %v", err)
	}
	t.Cleanup(func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := runJournal.Close(closeCtx); err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("close journal: %v", err)
		}
		unsubscribe()
		if err := bus.Close(context.Background()); err != nil {
			t.Fatalf("close bus: %v", err)
		}
	})

	total := opts.totalJobs
	if total == 0 {
		total = 1
	}
	jb := &job{
		SafeName:  "task_01",
		CodeFiles: []string{"task_01.md"},
		OutLog:    filepath.Join(runArtifacts.RunDir, "task_01.out.log"),
		ErrLog:    filepath.Join(runArtifacts.RunDir, "task_01.err.log"),
	}
	execCtx := &jobExecutionContext{
		ctx:     context.Background(),
		total:   total,
		journal: runJournal,
		cfg: &config{
			MaxRetries:             opts.maxRetries,
			RunArtifacts:           runArtifacts,
			WorkspaceRoot:          root,
			RetryBackoffMultiplier: 1.5,
			Stall: model.StallPolicy{
				Enabled:     opts.stallEnabled,
				IdleTimeout: 3 * time.Minute,
				Retries:     opts.stallRetries,
			},
		},
	}
	runner := newJobRunner(0, jb, execCtx)
	scripted := attempts
	var calls atomic.Int32
	runner.runAttempt = func(context.Context, time.Duration) jobAttemptResult {
		idx := int(calls.Add(1)) - 1
		if idx >= len(scripted) {
			t.Errorf("attempt %d exceeds scripted attempts (%d)", idx+1, len(scripted))
			return successResult()
		}
		return scripted[idx]
	}

	return &stallHarness{execCtx: execCtx, job: jb, runner: runner, events: ch, root: root}
}

// drain collects exactly want journal events published during the run.
func (h *stallHarness) drain(t *testing.T, want int) []eventspkg.Event {
	t.Helper()
	return collectRuntimeEvents(t, h.events, want)
}

func eventKinds(evs []eventspkg.Event) []eventspkg.EventKind {
	kindsSeen := make([]eventspkg.EventKind, 0, len(evs))
	for _, ev := range evs {
		kindsSeen = append(kindsSeen, ev.Kind)
	}
	return kindsSeen
}

func findEvent(t *testing.T, evs []eventspkg.Event, kind eventspkg.EventKind) eventspkg.Event {
	t.Helper()
	for _, ev := range evs {
		if ev.Kind == kind {
			return ev
		}
	}
	t.Fatalf("event %s not found in %v", kind, eventKinds(evs))
	return eventspkg.Event{}
}

func hasEvent(evs []eventspkg.Event, kind eventspkg.EventKind) bool {
	for _, ev := range evs {
		if ev.Kind == kind {
			return true
		}
	}
	return false
}

func TestJobRunnerStallRetryIsIndependentOfMaxRetries(t *testing.T) {
	t.Parallel()
	requireStallGit(t)

	t.Run("Should retry a stalled job once even when MaxRetries is zero", func(t *testing.T) {
		root := initStallGitRepo(t)
		harness := newStallHarness(
			t,
			stallHarnessOptions{workspaceRoot: root, maxRetries: 0, stallRetries: 1, stallEnabled: true},
			stallResult(&job{SafeName: "task_01"}, "tool-call-7"),
			successResult(),
		)

		harness.runner.executeAttempts(context.Background())

		if got := harness.job.Status; got != runStatusSucceeded {
			t.Fatalf("job status = %q, want %q", got, runStatusSucceeded)
		}
		if got := harness.runner.lifecycle.attempt; got != 2 {
			t.Fatalf("attempts = %d, want exactly one stall retry (2) with MaxRetries=0", got)
		}
		evs := harness.drain(t, 4)
		if !hasEvent(evs, eventspkg.EventKindJobStalled) {
			t.Fatalf("missing job.stalled in %v", eventKinds(evs))
		}
		if !hasEvent(evs, eventspkg.EventKindJobRetryScheduled) {
			t.Fatalf("missing job.retry_scheduled in %v", eventKinds(evs))
		}
		if hasEvent(evs, eventspkg.EventKindJobParked) {
			t.Fatalf("unexpected job.parked in %v", eventKinds(evs))
		}
		if got := atomic.LoadInt32(&harness.execCtx.failed); got != 0 {
			t.Fatalf("failed counter = %d, want 0 for a recovered job", got)
		}
	})
}

func TestJobRunnerStalledStallOrderEmitsStalledBeforeRetry(t *testing.T) {
	t.Parallel()
	requireStallGit(t)

	t.Run("Should emit job.stalled before job.retry_scheduled", func(t *testing.T) {
		root := initStallGitRepo(t)
		harness := newStallHarness(
			t,
			stallHarnessOptions{workspaceRoot: root, maxRetries: 0, stallRetries: 1, stallEnabled: true},
			stallResult(&job{SafeName: "task_01"}, "tool-call-7"),
			successResult(),
		)
		harness.runner.executeAttempts(context.Background())

		evs := harness.drain(t, 4)
		stalledAt, retryAt := -1, -1
		for idx, ev := range evs {
			switch ev.Kind {
			case eventspkg.EventKindJobStalled:
				stalledAt = idx
			case eventspkg.EventKindJobRetryScheduled:
				retryAt = idx
			}
		}
		if stalledAt < 0 || retryAt < 0 || stalledAt > retryAt {
			t.Fatalf("expected job.stalled before job.retry_scheduled, got %v", eventKinds(evs))
		}
		var payload kinds.JobStalledPayload
		decodeRuntimeEventPayload(t, evs[stalledAt], &payload)
		if payload.LastToolCall != "tool-call-7" {
			t.Fatalf("job.stalled last tool call = %q, want tool-call-7", payload.LastToolCall)
		}
		if !strings.Contains(payload.Reason, "activity timeout") {
			t.Fatalf("job.stalled reason = %q, want the activity timeout reason", payload.Reason)
		}
	})
}

func TestJobRunnerSecondStallParksWithPopulatedPayload(t *testing.T) {
	t.Parallel()
	requireStallGit(t)

	t.Run("Should park with a populated payload after a second stall", func(t *testing.T) {
		root := initStallGitRepo(t)
		harness := newStallHarness(
			t,
			stallHarnessOptions{workspaceRoot: root, maxRetries: 0, stallRetries: 1, stallEnabled: true},
			stallResult(&job{SafeName: "task_01"}, "tool-call-7"),
			stallResult(&job{SafeName: "task_01"}, "tool-call-9"),
		)

		harness.runner.executeAttempts(context.Background())

		if got := harness.job.Status; got != runStatusParked {
			t.Fatalf("job status = %q, want %q", got, runStatusParked)
		}
		if got := harness.runner.lifecycle.state; got != jobPhaseParked {
			t.Fatalf("lifecycle state = %q, want %q", got, jobPhaseParked)
		}
		if got := harness.runner.lifecycle.attempt; got != 2 {
			t.Fatalf("attempts = %d, want exactly 2 (one stall retry, then park)", got)
		}
		evs := harness.drain(t, 5)
		if hasEvent(evs, eventspkg.EventKindJobFailed) {
			t.Fatalf("a stalled job must park, not fail: %v", eventKinds(evs))
		}
		var payload kinds.JobParkedPayload
		decodeRuntimeEventPayload(t, findEvent(t, evs, eventspkg.EventKindJobParked), &payload)
		if !strings.Contains(payload.Reason, "stalled again") {
			t.Fatalf("parked reason = %q, want the second-stall reason", payload.Reason)
		}
		if payload.LastToolCall != "tool-call-9" {
			t.Fatalf("parked last tool call = %q, want tool-call-9", payload.LastToolCall)
		}
		if payload.WorktreePath != root {
			t.Fatalf("parked worktree path = %q, want %q", payload.WorktreePath, root)
		}
		if payload.LogPath != harness.job.OutLog {
			t.Fatalf("parked log path = %q, want %q", payload.LogPath, harness.job.OutLog)
		}
		if payload.LastProgressSeq == 0 {
			t.Fatal("parked payload must carry the last journal progress sequence")
		}
		if got := atomic.LoadInt32(&harness.execCtx.failed); got != 1 {
			t.Fatalf("failed counter = %d, want 1 so the run still exits non-zero", got)
		}
	})
}

func TestJobRunnerStallRetryRunsFromACleanWorktree(t *testing.T) {
	t.Parallel()
	requireStallGit(t)

	t.Run("Should run the stall retry from a clean worktree", func(t *testing.T) {
		root := initStallGitRepo(t)
		harness := newStallHarness(
			t,
			stallHarnessOptions{workspaceRoot: root, maxRetries: 0, stallRetries: 1, stallEnabled: true},
		)

		var secondAttemptDirty []string
		var calls int
		harness.runner.runAttempt = func(context.Context, time.Duration) jobAttemptResult {
			calls++
			switch calls {
			case 1:
				// The stalled attempt commits work and leaves scratch files behind.
				if err := os.WriteFile(
					filepath.Join(root, "migration.sql"),
					[]byte("DROP TABLE x;"),
					0o600,
				); err != nil {
					t.Errorf("write migration: %v", err)
				}
				mustStallGit(t, root, "add", ".")
				mustStallGit(t, root, "commit", "-q", "-m", "half-applied migration")
				if err := os.WriteFile(filepath.Join(root, "scratch.txt"), []byte("junk"), 0o600); err != nil {
					t.Errorf("write scratch: %v", err)
				}
				return stallResult(harness.job, "tool-call-1")
			default:
				secondAttemptDirty = dirtyPaths(t, root)
				return successResult()
			}
		}

		harness.runner.executeAttempts(context.Background())

		if calls != 2 {
			t.Fatalf("attempts = %d, want 2", calls)
		}
		if len(secondAttemptDirty) != 0 {
			t.Fatalf("stall retry started with a dirty worktree: %v", secondAttemptDirty)
		}
		if _, err := os.Stat(filepath.Join(root, "migration.sql")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("stalled attempt's committed side effect survived the reset: %v", err)
		}
		if harness.job.Status != runStatusSucceeded {
			t.Fatalf("job status = %q, want %q", harness.job.Status, runStatusSucceeded)
		}
	})

	t.Run("Should retry a PRD task when TasksDir is excluded from the baseline", func(t *testing.T) {
		root := initStallGitRepo(t)
		harness := newStallHarness(
			t,
			stallHarnessOptions{workspaceRoot: root, maxRetries: 0, stallRetries: 1, stallEnabled: true},
		)
		harness.execCtx.cfg.Mode = model.ExecutionModePRDTasks
		harness.execCtx.cfg.TasksDir = filepath.Join(root, ".compozy", "tasks", "demo")

		var secondAttemptDirty []string
		var calls int
		harness.runner.runAttempt = func(context.Context, time.Duration) jobAttemptResult {
			calls++
			if calls == 1 {
				if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# stalled\n"), 0o600); err != nil {
					t.Errorf("rewrite README: %v", err)
				}
				if err := os.WriteFile(filepath.Join(root, "scratch.txt"), []byte("junk"), 0o600); err != nil {
					t.Errorf("write scratch: %v", err)
				}
				return stallResult(harness.job, "tool-call-1")
			}

			secondAttemptDirty = dirtyPaths(t, root)
			return ordinaryFailureResult(harness.job)
		}

		harness.runner.executeAttempts(context.Background())

		if calls != 2 {
			t.Fatalf("attempts = %d, want 2", calls)
		}
		if len(secondAttemptDirty) != 0 {
			t.Fatalf("PRD stall retry started with a dirty worktree: %v", secondAttemptDirty)
		}
		evs := harness.drain(t, 4)
		if !hasEvent(evs, eventspkg.EventKindJobRetryScheduled) {
			t.Fatalf("missing job.retry_scheduled in %v", eventKinds(evs))
		}
		if hasEvent(evs, eventspkg.EventKindJobParked) {
			t.Fatalf("unexpected job.parked in %v", eventKinds(evs))
		}
	})
}

func dirtyPaths(t *testing.T, root string) []string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", "status", "--porcelain", "--untracked-files=all")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git status: %v", err)
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

func TestJobRunnerParksImmediatelyWhenCleanResetIsImpossible(t *testing.T) {
	t.Parallel()

	t.Run("Should park on the first stall when the workspace is not a git repo", func(t *testing.T) {
		t.Parallel()
		harness := newStallHarness(
			t,
			stallHarnessOptions{maxRetries: 0, stallRetries: 1, stallEnabled: true},
			stallResult(&job{SafeName: "task_01"}, "tool-call-1"),
		)

		harness.runner.executeAttempts(context.Background())

		if got := harness.job.Status; got != runStatusParked {
			t.Fatalf("job status = %q, want %q", got, runStatusParked)
		}
		if got := harness.runner.lifecycle.attempt; got != 1 {
			t.Fatalf("attempts = %d, want 1 (no retry when reset is impossible)", got)
		}
		evs := harness.drain(t, 3)
		var payload kinds.JobParkedPayload
		decodeRuntimeEventPayload(t, findEvent(t, evs, eventspkg.EventKindJobParked), &payload)
		if !strings.Contains(payload.Reason, "clean worktree reset is not possible") {
			t.Fatalf("parked reason = %q, want the reset-impossible reason", payload.Reason)
		}
		if hasEvent(evs, eventspkg.EventKindJobRetryScheduled) {
			t.Fatalf("unexpected retry when reset is impossible: %v", eventKinds(evs))
		}
	})

	t.Run("Should park on the first stall when siblings share the workspace", func(t *testing.T) {
		t.Parallel()
		requireStallGit(t)

		root := initStallGitRepo(t)
		harness := newStallHarness(
			t,
			stallHarnessOptions{workspaceRoot: root, maxRetries: 0, stallRetries: 1, stallEnabled: true, totalJobs: 2},
			stallResult(&job{SafeName: "task_01"}, "tool-call-1"),
		)

		harness.runner.executeAttempts(context.Background())

		if got := harness.job.Status; got != runStatusParked {
			t.Fatalf("job status = %q, want %q", got, runStatusParked)
		}
		if got := harness.runner.lifecycle.attempt; got != 1 {
			t.Fatalf("attempts = %d, want 1 (resetting a shared workspace would clobber siblings)", got)
		}
	})
}

// A stall must never fall through to the ordinary give-up path, so a pre_retry
// hook that errors or vetoes the retry parks the job instead of failing it.
func TestJobRunnerStallParksWhenPreRetryHookBlocksTheRetry(t *testing.T) {
	t.Parallel()
	requireStallGit(t)

	tests := []struct {
		name       string
		mutator    func(any) (any, error)
		wantReason string
	}{
		{
			name:       "Should park when the pre_retry hook errors",
			mutator:    func(any) (any, error) { return nil, errors.New("hook exploded") },
			wantReason: "dispatch job.pre_retry",
		},
		{
			name: "Should park when an extension vetoes the stall retry",
			mutator: func(input any) (any, error) {
				payload, ok := input.(jobPreRetryPayload)
				if !ok {
					return input, nil
				}
				proceed := false
				payload.Proceed = &proceed
				return payload, nil
			},
			wantReason: "stall retry canceled by extension",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			harness := newStallHarness(
				t,
				stallHarnessOptions{
					workspaceRoot: initStallGitRepo(t),
					maxRetries:    0,
					stallRetries:  1,
					stallEnabled:  true,
				},
				stallResult(&job{SafeName: "task_01"}, "tool-call-1"),
			)
			harness.execCtx.cfg.RuntimeManager = &executionHookManager{
				mutators: map[string]func(any) (any, error){"job.pre_retry": tt.mutator},
			}

			harness.runner.executeAttempts(context.Background())

			if got := harness.job.Status; got != runStatusParked {
				t.Fatalf("job status = %q, want %q", got, runStatusParked)
			}
			evs := harness.drain(t, 3)
			if hasEvent(evs, eventspkg.EventKindJobFailed) {
				t.Fatalf("a blocked stall retry must park, not fail: %v", eventKinds(evs))
			}
			var payload kinds.JobParkedPayload
			decodeRuntimeEventPayload(t, findEvent(t, evs, eventspkg.EventKindJobParked), &payload)
			if !strings.Contains(payload.Reason, tt.wantReason) {
				t.Fatalf("parked reason = %q, want it to mention %q", payload.Reason, tt.wantReason)
			}
		})
	}
}

// A zero stall budget (stall.retries = 0) parks on the first stall. The park
// reason must say so rather than claim a clean-state retry that never happened.
func TestJobRunnerZeroStallBudgetParksOnFirstStall(t *testing.T) {
	t.Parallel()
	requireStallGit(t)

	t.Run("Should park on the first stall when the stall budget is zero", func(t *testing.T) {
		harness := newStallHarness(
			t,
			stallHarnessOptions{
				workspaceRoot: initStallGitRepo(t),
				maxRetries:    0,
				stallRetries:  0,
				stallEnabled:  true,
			},
			stallResult(&job{SafeName: "task_01"}, "tool-call-1"),
		)

		harness.runner.executeAttempts(context.Background())

		if got := harness.job.Status; got != runStatusParked {
			t.Fatalf("job status = %q, want %q", got, runStatusParked)
		}
		if got := harness.runner.lifecycle.attempt; got != 1 {
			t.Fatalf("attempts = %d, want 1 when the stall budget is zero", got)
		}
		evs := harness.drain(t, 3)
		var payload kinds.JobParkedPayload
		decodeRuntimeEventPayload(t, findEvent(t, evs, eventspkg.EventKindJobParked), &payload)
		if !strings.Contains(payload.Reason, "no stall retry budget remains") {
			t.Fatalf("parked reason = %q, want it to report the empty stall budget", payload.Reason)
		}
		if strings.Contains(payload.Reason, "stalled again") {
			t.Fatalf("parked reason = %q, must not claim a retry that never happened", payload.Reason)
		}
	})
}

// An ordinary retryable failure must never draw on the stall budget, so a job
// whose MaxRetries is exhausted fails even while stall retries remain available.
func TestJobRunnerOrdinaryFailureDoesNotConsumeStallBudget(t *testing.T) {
	t.Parallel()
	requireStallGit(t)

	t.Run("Should not consume the stall budget on an ordinary failure", func(t *testing.T) {
		jb := &job{SafeName: "task_01"}
		harness := newStallHarness(
			t,
			stallHarnessOptions{
				workspaceRoot: initStallGitRepo(t),
				maxRetries:    1,
				stallRetries:  1,
				stallEnabled:  true,
			},
			ordinaryFailureResult(jb),
			stallResult(jb, "tool-call-1"),
			successResult(),
		)

		harness.runner.executeAttempts(context.Background())

		if got := harness.job.Status; got != runStatusSucceeded {
			t.Fatalf("job status = %q, want %q", got, runStatusSucceeded)
		}
		if got := harness.runner.lifecycle.attempt; got != 3 {
			t.Fatalf("attempts = %d, want 3 (one ordinary retry, then one stall retry)", got)
		}
	})
}

func TestJobRunnerOrdinaryFailureNeverParks(t *testing.T) {
	t.Parallel()

	t.Run("Should exhaust MaxRetries then fail", func(t *testing.T) {
		t.Parallel()
		jb := &job{SafeName: "task_01"}
		harness := newStallHarness(
			t,
			stallHarnessOptions{maxRetries: 1, stallRetries: 1, stallEnabled: true},
			ordinaryFailureResult(jb),
			ordinaryFailureResult(jb),
		)

		harness.runner.executeAttempts(context.Background())

		if got := harness.job.Status; got != runStatusFailed {
			t.Fatalf("job status = %q, want %q", got, runStatusFailed)
		}
		if got := harness.runner.lifecycle.attempt; got != 2 {
			t.Fatalf("attempts = %d, want 2", got)
		}
		evs := harness.drain(t, 3)
		if hasEvent(evs, eventspkg.EventKindJobParked) || hasEvent(evs, eventspkg.EventKindJobStalled) {
			t.Fatalf("ordinary failure took the stall path: %v", eventKinds(evs))
		}
		if !hasEvent(evs, eventspkg.EventKindJobFailed) {
			t.Fatalf("missing job.failed in %v", eventKinds(evs))
		}
	})

	t.Run("Should fail without retrying when MaxRetries is zero", func(t *testing.T) {
		t.Parallel()
		jb := &job{SafeName: "task_01"}
		harness := newStallHarness(
			t,
			stallHarnessOptions{maxRetries: 0, stallRetries: 1, stallEnabled: true},
			ordinaryFailureResult(jb),
		)

		harness.runner.executeAttempts(context.Background())

		if got := harness.job.Status; got != runStatusFailed {
			t.Fatalf("job status = %q, want %q", got, runStatusFailed)
		}
		if got := harness.runner.lifecycle.attempt; got != 1 {
			t.Fatalf("attempts = %d, want 1", got)
		}
	})
}

// TestJobRunnerStalledJobDoesNotAffectSiblings models the daemon's parallel task
// mode: each child run owns an isolated worktree and a single job. One child is
// scripted to stall twice and must park while its sibling completes untouched.
func TestJobRunnerStalledJobDoesNotAffectSiblings(t *testing.T) {
	t.Parallel()
	requireStallGit(t)

	t.Run("Should park without affecting a sibling job", func(t *testing.T) {
		stalling := newStallHarness(
			t,
			stallHarnessOptions{workspaceRoot: initStallGitRepo(t), maxRetries: 0, stallRetries: 1, stallEnabled: true},
			stallResult(&job{SafeName: "task_01"}, "tool-call-1"),
			stallResult(&job{SafeName: "task_01"}, "tool-call-2"),
		)
		siblingRoot := initStallGitRepo(t)
		sibling := newStallHarness(
			t,
			stallHarnessOptions{workspaceRoot: siblingRoot, maxRetries: 0, stallRetries: 1, stallEnabled: true},
		)
		siblingProduced := filepath.Join(siblingRoot, "sibling-output.txt")
		sibling.runner.runAttempt = func(context.Context, time.Duration) jobAttemptResult {
			if err := os.WriteFile(siblingProduced, []byte("sibling work"), 0o600); err != nil {
				t.Errorf("write sibling output: %v", err)
			}
			return successResult()
		}

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			stalling.runner.executeAttempts(context.Background())
		}()
		go func() {
			defer wg.Done()
			sibling.runner.executeAttempts(context.Background())
		}()
		wg.Wait()

		if got := stalling.job.Status; got != runStatusParked {
			t.Fatalf("stalling job status = %q, want %q", got, runStatusParked)
		}
		if got := sibling.job.Status; got != runStatusSucceeded {
			t.Fatalf("sibling job status = %q, want %q", got, runStatusSucceeded)
		}
		if _, err := os.Stat(siblingProduced); err != nil {
			t.Fatalf("sibling work was destroyed by the parked job's reset: %v", err)
		}
		if got := atomic.LoadInt32(&sibling.execCtx.failed); got != 0 {
			t.Fatalf("sibling failed counter = %d, want 0", got)
		}
		// The parked worktree is preserved for triage, not reset a second time.
		if _, err := os.Stat(filepath.Join(stalling.root, "README.md")); err != nil {
			t.Fatalf("parked worktree was not preserved: %v", err)
		}
	})
}

func TestDeriveRunStatusKeepsParkedDistinctFromFailed(t *testing.T) {
	t.Parallel()

	parkedFailure := failInfo{Err: errors.New("parked")}
	tests := []struct {
		name     string
		jobs     []job
		failures []failInfo
		want     string
	}{
		{
			name:     "Should report parked when a job parked",
			jobs:     []job{{Status: runStatusSucceeded}, {Status: runStatusParked}},
			failures: []failInfo{parkedFailure},
			want:     runStatusParked,
		},
		{
			name:     "Should prefer failed over parked",
			jobs:     []job{{Status: runStatusParked}, {Status: runStatusFailed}},
			failures: []failInfo{parkedFailure},
			want:     runStatusFailed,
		},
		{
			name:     "Should prefer canceled over parked",
			jobs:     []job{{Status: runStatusParked}, {Status: runStatusCanceled}},
			failures: []failInfo{parkedFailure},
			want:     runStatusCanceled,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := deriveRunStatus(tt.jobs, tt.failures); got != tt.want {
				t.Fatalf("deriveRunStatus = %q, want %q", got, tt.want)
			}
		})
	}
}
