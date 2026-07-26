package globaldb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
)

type transitionResult struct {
	name string
	run  taskpkg.Run
	err  error
}

func TestGlobalDBTaskRunLeaseAdversarialFencing(t *testing.T) {
	t.Parallel()

	t.Run("Should claim each queued run exactly once under concurrent workers", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC)
		runs := seedAdversarialClaimRunsAcrossTasks(ctx, t, globalDB, "concurrent", 3, now)

		type claimAttempt struct {
			result taskpkg.ClaimResult
			err    error
		}
		attempts := make([]claimAttempt, 12)
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(len(attempts))
		for idx := range attempts {
			go func(idx int) {
				defer wg.Done()
				<-start
				attempts[idx].result, attempts[idx].err = globalDB.ClaimNextRun(
					ctx,
					taskpkg.ClaimCriteria{
						Scope:            taskpkg.ScopeGlobal,
						ClaimerSessionID: fmt.Sprintf("sess-concurrent-%02d", idx),
						LeaseDuration:    time.Minute,
						Now:              now.Add(time.Duration(idx) * time.Millisecond),
					},
				)
			}(idx)
		}
		close(start)
		wg.Wait()

		claimedRunIDs := map[string]int{}
		for idx, attempt := range attempts {
			if attempt.err != nil {
				if !errors.Is(attempt.err, taskpkg.ErrNoClaimableRun) {
					t.Fatalf("attempt %d error = %v, want ErrNoClaimableRun", idx, attempt.err)
				}
				continue
			}
			if attempt.result.ClaimToken == "" {
				t.Fatalf("attempt %d claim token = empty, want raw token returned once", idx)
			}
			if !taskpkg.VerifyClaimToken(attempt.result.ClaimToken, attempt.result.Run.ClaimTokenHash) {
				t.Fatalf("attempt %d claim token does not verify against persisted hash", idx)
			}
			claimedRunIDs[attempt.result.Run.ID]++
		}
		if got, want := len(claimedRunIDs), len(runs); got != want {
			t.Fatalf("claimed unique runs = %d, want %d (attempts=%#v)", got, want, attempts)
		}
		for _, run := range runs {
			if got := claimedRunIDs[run.ID]; got != 1 {
				t.Fatalf("claimedRunIDs[%s] = %d, want 1", run.ID, got)
			}
			stored, err := globalDB.GetTaskRun(ctx, run.ID)
			if err != nil {
				t.Fatalf("GetTaskRun(%q) error = %v", run.ID, err)
			}
			if stored.Status != taskpkg.TaskRunStatusClaimed ||
				stored.SessionID == "" ||
				stored.ClaimTokenHash == "" {
				t.Fatalf("stored run %q = %#v, want claimed with owner and token hash", run.ID, stored)
			}
		}
	})

	t.Run("Should give one owner to concurrent exact-run claims", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 18, 21, 45, 0, 0, time.UTC)
		run := seedAdversarialClaimRunsAcrossTasks(ctx, t, globalDB, "exact-run", 1, now)[0]

		type claimAttempt struct {
			result taskpkg.ClaimResult
			err    error
		}
		attempts := make([]claimAttempt, 2)
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(len(attempts))
		for idx := range attempts {
			go func(idx int) {
				defer wg.Done()
				<-start
				attempts[idx].result, attempts[idx].err = globalDB.ClaimNextRun(
					ctx,
					taskpkg.ClaimCriteria{
						RunID:            run.ID,
						Scope:            taskpkg.ScopeGlobal,
						ClaimerSessionID: fmt.Sprintf("sess-exact-run-%d", idx),
						LeaseDuration:    time.Minute,
						Now:              now,
					},
				)
			}(idx)
		}
		close(start)
		wg.Wait()

		var winner *taskpkg.ClaimResult
		losers := 0
		for idx := range attempts {
			switch {
			case attempts[idx].err == nil:
				winner = &attempts[idx].result
			case errors.Is(attempts[idx].err, taskpkg.ErrNoClaimableRun):
				losers++
			default:
				t.Fatalf("attempt %d error = %v, want success or ErrNoClaimableRun", idx, attempts[idx].err)
			}
		}
		if winner == nil || losers != 1 {
			t.Fatalf("exact-run outcomes = %#v, want one winner and one ErrNoClaimableRun", attempts)
		}

		stored, err := globalDB.GetTaskRun(ctx, run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun(%q) error = %v", run.ID, err)
		}
		if stored.SessionID != winner.Run.SessionID || stored.ClaimTokenHash != winner.Run.ClaimTokenHash {
			t.Fatalf("stored ownership = %#v, want winner %#v", stored, winner.Run)
		}
		if _, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			RunID:            run.ID,
			Scope:            taskpkg.ScopeGlobal,
			ClaimerSessionID: "sess-exact-run-late",
			LeaseDuration:    time.Minute,
			Now:              now.Add(time.Second),
		}); !errors.Is(err, taskpkg.ErrNoClaimableRun) {
			t.Fatalf("ClaimNextRun(already claimed exact run) error = %v, want ErrNoClaimableRun", err)
		}
		afterLateClaim, err := globalDB.GetTaskRun(ctx, run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun(%q, after late claim) error = %v", run.ID, err)
		}
		if afterLateClaim.SessionID != stored.SessionID || afterLateClaim.ClaimTokenHash != stored.ClaimTokenHash {
			t.Fatalf("ownership after late claim = %#v, want unchanged %#v", afterLateClaim, stored)
		}
	})

	t.Run("Should allow only one release complete or fail transition for one active lease", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 5, 19, 10, 30, 0, 0, time.UTC)
		runs := seedAdversarialClaimRuns(ctx, t, globalDB, "transition-race", 1, now)
		claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:            taskpkg.ScopeGlobal,
			ClaimerSessionID: "sess-transition-race",
			LeaseDuration:    time.Minute,
			Now:              now,
		})
		if err != nil {
			t.Fatalf("ClaimNextRun() error = %v", err)
		}
		if got, want := claim.Run.ID, runs[0].ID; got != want {
			t.Fatalf("ClaimNextRun().Run.ID = %q, want %q", got, want)
		}

		assertLeaseRejectsWrongTokens(ctx, t, globalDB, &claim, now.Add(10*time.Second))

		type transitionAttempt struct {
			name string
			run  func() (taskpkg.Run, error)
		}
		attempts := []transitionAttempt{
			{
				name: "release",
				run: func() (taskpkg.Run, error) {
					return globalDB.ReleaseRunLease(ctx, taskpkg.LeaseRelease{
						RunID:      claim.Run.ID,
						ClaimToken: claim.ClaimToken,
						Reason:     "handoff-race",
						Now:        now.Add(20 * time.Second),
					})
				},
			},
			{
				name: "complete",
				run: func() (taskpkg.Run, error) {
					return globalDB.CompleteRunLease(ctx, taskpkg.LeaseCompletion{
						Actor:      coordinatorActorContextForTest(),
						RunID:      claim.Run.ID,
						ClaimToken: claim.ClaimToken,
						Result:     taskpkg.RunResult{Value: json.RawMessage(`{"ok":true}`)},
						Now:        now.Add(20 * time.Second),
					})
				},
			},
			{
				name: "fail",
				run: func() (taskpkg.Run, error) {
					return globalDB.FailRunLease(ctx, taskpkg.LeaseFailure{
						Actor:      coordinatorActorContextForTest(),
						RunID:      claim.Run.ID,
						ClaimToken: claim.ClaimToken,
						Failure:    taskpkg.RunFailure{Error: "worker failed during race"},
						Now:        now.Add(20 * time.Second),
					})
				},
			},
		}

		results := make([]transitionResult, len(attempts))
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(len(attempts))
		for idx := range attempts {
			go func(idx int) {
				defer wg.Done()
				<-start
				updated, err := attempts[idx].run()
				results[idx] = transitionResult{name: attempts[idx].name, run: updated, err: err}
			}(idx)
		}
		close(start)
		wg.Wait()

		successes := make([]transitionResult, 0, 1)
		for _, result := range results {
			if result.err == nil {
				successes = append(successes, result)
				continue
			}
			if !isExpectedLeaseRaceError(result.err) {
				t.Fatalf("%s error = %v, want invalid token or status transition", result.name, result.err)
			}
		}
		if got, want := len(successes), 1; got != want {
			t.Fatalf("successful transitions = %d, want %d (results=%#v)", got, want, results)
		}
		assertLeaseRaceWinner(ctx, t, globalDB, claim.Run.TaskID, &successes[0])
		if _, err := globalDB.HeartbeatRunLease(ctx, taskpkg.LeaseHeartbeat{
			RunID:         claim.Run.ID,
			ClaimToken:    claim.ClaimToken,
			LeaseDuration: time.Minute,
			Now:           now.Add(30 * time.Second),
		}); !isExpectedLeaseRaceError(err) {
			t.Fatalf("HeartbeatRunLease(after race winner) error = %v, want invalid token or status transition", err)
		}
	})
}

func TestGlobalDBClaimNextRunWorkspaceActiveRunCap(t *testing.T) {
	t.Parallel()

	t.Run("Should admit only one workspace run when concurrent claims reach the cap", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 18, 22, 0, 0, 0, time.UTC)
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "claim-cap", t.TempDir())
		runs := seedWorkspaceClaimRunsAcrossTasks(ctx, t, globalDB, "cap", workspaceID, 2, now)

		type claimAttempt struct {
			result taskpkg.ClaimResult
			err    error
		}
		attempts := make([]claimAttempt, len(runs))
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(len(attempts))
		for idx := range attempts {
			go func(idx int) {
				defer wg.Done()
				<-start
				attempts[idx].result, attempts[idx].err = globalDB.ClaimNextRun(
					ctx,
					taskpkg.ClaimCriteria{
						Scope:                 taskpkg.ScopeWorkspace,
						WorkspaceID:           workspaceID,
						ClaimerSessionID:      fmt.Sprintf("sess-cap-%d", idx),
						LeaseDuration:         time.Minute,
						Now:                   now,
						WorkspaceActiveRunCap: 1,
					},
				)
			}(idx)
		}
		close(start)
		wg.Wait()

		successes := 0
		deferred := 0
		for idx, attempt := range attempts {
			switch {
			case attempt.err == nil:
				successes++
			case errors.Is(attempt.err, taskpkg.ErrWorkspaceActiveRunCapReached):
				deferred++
			default:
				t.Fatalf("attempt %d error = %v, want success or workspace cap deferral", idx, attempt.err)
			}
		}
		if successes != 1 || deferred != 1 {
			t.Fatalf("claim outcomes = successes:%d deferred:%d, want 1/1", successes, deferred)
		}

		queued := 0
		for _, run := range runs {
			stored, err := globalDB.GetTaskRun(ctx, run.ID)
			if err != nil {
				t.Fatalf("GetTaskRun(%q) error = %v", run.ID, err)
			}
			if stored.Status.Normalize() == taskpkg.TaskRunStatusQueued {
				queued++
			}
		}
		if queued != 1 {
			t.Fatalf("queued runs after cap deferral = %d, want 1", queued)
		}
	})

	t.Run("Should isolate workspace capacity and reopen after completion or lease expiry", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 18, 22, 30, 0, 0, time.UTC)
		workspaceA := registerWorkspaceForGlobalTests(t, globalDB, "claim-cap-a", t.TempDir())
		workspaceB := registerWorkspaceForGlobalTests(t, globalDB, "claim-cap-b", t.TempDir())
		runsA := seedWorkspaceClaimRunsAcrossTasks(ctx, t, globalDB, "cap-a", workspaceA, 2, now)
		runsB := seedWorkspaceClaimRunsAcrossTasks(ctx, t, globalDB, "cap-b", workspaceB, 1, now)

		first, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			RunID: runsA[0].ID, Scope: taskpkg.ScopeWorkspace, WorkspaceID: workspaceA,
			ClaimerSessionID: "sess-cap-a-first", LeaseDuration: time.Minute, Now: now,
			WorkspaceActiveRunCap: 1,
		})
		if err != nil {
			t.Fatalf("ClaimNextRun(workspace A first) error = %v", err)
		}
		if _, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			RunID: runsA[1].ID, Scope: taskpkg.ScopeWorkspace, WorkspaceID: workspaceA,
			ClaimerSessionID: "sess-cap-a-deferred", LeaseDuration: time.Minute, Now: now.Add(30 * time.Second),
			WorkspaceActiveRunCap: 1,
		}); !errors.Is(err, taskpkg.ErrWorkspaceActiveRunCapReached) {
			t.Fatalf("ClaimNextRun(workspace A capped) error = %v, want ErrWorkspaceActiveRunCapReached", err)
		}
		if _, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			RunID: runsB[0].ID, Scope: taskpkg.ScopeWorkspace, WorkspaceID: workspaceB,
			ClaimerSessionID: "sess-cap-b", LeaseDuration: time.Minute, Now: now.Add(30 * time.Second),
			WorkspaceActiveRunCap: 1,
		}); err != nil {
			t.Fatalf("ClaimNextRun(workspace B isolated) error = %v", err)
		}
		if _, err := globalDB.CompleteRunLease(ctx, taskpkg.LeaseCompletion{
			Actor:      coordinatorActorContextForTest(),
			RunID:      first.Run.ID,
			ClaimToken: first.ClaimToken,
			Result:     taskpkg.RunResult{Value: json.RawMessage(`{"completed":true}`)},
			Now:        now.Add(40 * time.Second),
		}); err != nil {
			t.Fatalf("CompleteRunLease(workspace A first) error = %v", err)
		}
		second, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			RunID: runsA[1].ID, Scope: taskpkg.ScopeWorkspace, WorkspaceID: workspaceA,
			ClaimerSessionID: "sess-cap-a-after-completion", LeaseDuration: time.Minute,
			Now: now.Add(40 * time.Second), WorkspaceActiveRunCap: 1,
		})
		if err != nil {
			t.Fatalf("ClaimNextRun(workspace A after completion) error = %v", err)
		}

		afterExpiry := seedWorkspaceClaimRunsAcrossTasks(
			ctx,
			t,
			globalDB,
			"cap-a-after-expiry",
			workspaceA,
			1,
			now,
		)[0]
		if _, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			RunID: afterExpiry.ID, Scope: taskpkg.ScopeWorkspace, WorkspaceID: workspaceA,
			ClaimerSessionID: "sess-cap-a-still-full", LeaseDuration: time.Minute,
			Now: now.Add(50 * time.Second), WorkspaceActiveRunCap: 1,
		}); !errors.Is(err, taskpkg.ErrWorkspaceActiveRunCapReached) {
			t.Fatalf("ClaimNextRun(workspace A before expiry) error = %v, want ErrWorkspaceActiveRunCapReached", err)
		}
		if _, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			RunID: afterExpiry.ID, Scope: taskpkg.ScopeWorkspace, WorkspaceID: workspaceA,
			ClaimerSessionID: "sess-cap-a-after-expiry", LeaseDuration: time.Minute,
			Now: second.LeaseUntil.Add(time.Nanosecond), WorkspaceActiveRunCap: 1,
		}); err != nil {
			t.Fatalf("ClaimNextRun(workspace A after expiry) error = %v", err)
		}
	})

	t.Run("Should exempt global work selected by a workspace caller", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 18, 23, 0, 0, 0, time.UTC)
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "claim-cap-global", t.TempDir())
		workspaceRun := seedWorkspaceClaimRunsAcrossTasks(
			ctx,
			t,
			globalDB,
			"cap-global-active",
			workspaceID,
			1,
			now,
		)[0]
		if _, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			RunID: workspaceRun.ID, Scope: taskpkg.ScopeWorkspace, WorkspaceID: workspaceID,
			ClaimerSessionID: "sess-cap-global-active", LeaseDuration: time.Minute, Now: now,
			WorkspaceActiveRunCap: 1,
		}); err != nil {
			t.Fatalf("ClaimNextRun(workspace active) error = %v", err)
		}
		globalRun := seedAdversarialClaimRunsAcrossTasks(ctx, t, globalDB, "cap-global", 1, now)[0]

		claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			RunID: globalRun.ID, Scope: taskpkg.ScopeWorkspace, WorkspaceID: workspaceID,
			ClaimerSessionID: "sess-cap-global", LeaseDuration: time.Minute, Now: now,
			WorkspaceActiveRunCap: 1,
		})
		if err != nil {
			t.Fatalf("ClaimNextRun(global at workspace cap) error = %v", err)
		}
		if claim.Run.ID != globalRun.ID {
			t.Fatalf("ClaimNextRun(global at workspace cap).Run.ID = %q, want %q", claim.Run.ID, globalRun.ID)
		}
	})
}

func seedAdversarialClaimRuns(
	ctx context.Context,
	t *testing.T,
	globalDB *GlobalDB,
	suffix string,
	runCount int,
	now time.Time,
) []taskpkg.Run {
	t.Helper()

	taskRecord := taskRecordForTest("task-adversarial-" + suffix)
	taskRecord.Status = taskpkg.TaskStatusReady
	if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	runs := make([]taskpkg.Run, 0, runCount)
	for idx := range runCount {
		run := taskRunForTest(fmt.Sprintf("run-adversarial-%s-%02d", suffix, idx), taskRecord.ID)
		run.QueuedAt = now.Add(time.Duration(idx) * time.Second)
		if err := globalDB.CreateTaskRun(ctx, run); err != nil {
			t.Fatalf("CreateTaskRun(%q) error = %v", run.ID, err)
		}
		runs = append(runs, run)
	}
	return runs
}

func seedAdversarialClaimRunsAcrossTasks(
	ctx context.Context,
	t *testing.T,
	globalDB *GlobalDB,
	suffix string,
	runCount int,
	now time.Time,
) []taskpkg.Run {
	t.Helper()

	runs := make([]taskpkg.Run, 0, runCount)
	for idx := range runCount {
		taskRecord := taskRecordForTest(fmt.Sprintf("task-adversarial-%s-%02d", suffix, idx))
		taskRecord.Status = taskpkg.TaskStatusReady
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask(%q) error = %v", taskRecord.ID, err)
		}
		run := taskRunForTest(fmt.Sprintf("run-adversarial-%s-%02d", suffix, idx), taskRecord.ID)
		run.QueuedAt = now.Add(time.Duration(idx) * time.Second)
		if err := globalDB.CreateTaskRun(ctx, run); err != nil {
			t.Fatalf("CreateTaskRun(%q) error = %v", run.ID, err)
		}
		runs = append(runs, run)
	}
	return runs
}

func seedWorkspaceClaimRunsAcrossTasks(
	ctx context.Context,
	t *testing.T,
	globalDB *GlobalDB,
	suffix string,
	workspaceID string,
	runCount int,
	now time.Time,
) []taskpkg.Run {
	t.Helper()

	runs := make([]taskpkg.Run, 0, runCount)
	for idx := range runCount {
		taskRecord := taskRecordForTest(fmt.Sprintf("task-workspace-%s-%02d", suffix, idx))
		taskRecord.Scope = taskpkg.ScopeWorkspace
		taskRecord.WorkspaceID = workspaceID
		taskRecord.Status = taskpkg.TaskStatusReady
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask(%q) error = %v", taskRecord.ID, err)
		}
		run := taskRunForTest(fmt.Sprintf("run-workspace-%s-%02d", suffix, idx), taskRecord.ID)
		run.QueuedAt = now.Add(time.Duration(idx) * time.Second)
		if err := globalDB.CreateTaskRun(ctx, run); err != nil {
			t.Fatalf("CreateTaskRun(%q) error = %v", run.ID, err)
		}
		runs = append(runs, run)
	}
	return runs
}

func assertLeaseRejectsWrongTokens(
	ctx context.Context,
	t *testing.T,
	globalDB *GlobalDB,
	claim *taskpkg.ClaimResult,
	now time.Time,
) {
	t.Helper()

	checks := []struct {
		name string
		run  func() error
	}{
		{
			name: "heartbeat",
			run: func() error {
				_, err := globalDB.HeartbeatRunLease(ctx, taskpkg.LeaseHeartbeat{
					RunID:         claim.Run.ID,
					ClaimToken:    "agh_claim_wrong",
					LeaseDuration: time.Minute,
					Now:           now,
				})
				return err
			},
		},
		{
			name: "release",
			run: func() error {
				_, err := globalDB.ReleaseRunLease(ctx, taskpkg.LeaseRelease{
					RunID:      claim.Run.ID,
					ClaimToken: "agh_claim_wrong",
					Reason:     "wrong-token",
					Now:        now,
				})
				return err
			},
		},
		{
			name: "complete",
			run: func() error {
				_, err := globalDB.CompleteRunLease(ctx, taskpkg.LeaseCompletion{
					Actor:      coordinatorActorContextForTest(),
					RunID:      claim.Run.ID,
					ClaimToken: "agh_claim_wrong",
					Result:     taskpkg.RunResult{Value: json.RawMessage(`{"ok":false}`)},
					Now:        now,
				})
				return err
			},
		},
		{
			name: "fail",
			run: func() error {
				_, err := globalDB.FailRunLease(ctx, taskpkg.LeaseFailure{
					Actor:      coordinatorActorContextForTest(),
					RunID:      claim.Run.ID,
					ClaimToken: "agh_claim_wrong",
					Failure:    taskpkg.RunFailure{Error: "wrong token should not fail run"},
					Now:        now,
				})
				return err
			},
		},
	}
	for _, check := range checks {
		if err := check.run(); !errors.Is(err, taskpkg.ErrInvalidClaimToken) {
			t.Fatalf("%s wrong-token error = %v, want %v", check.name, err, taskpkg.ErrInvalidClaimToken)
		}
	}

	stored, err := globalDB.GetTaskRun(ctx, claim.Run.ID)
	if err != nil {
		t.Fatalf("GetTaskRun(after wrong-token attempts) error = %v", err)
	}
	if stored.Status != taskpkg.TaskRunStatusClaimed ||
		stored.SessionID != claim.Run.SessionID ||
		stored.ClaimTokenHash != claim.Run.ClaimTokenHash {
		t.Fatalf("stored after wrong-token attempts = %#v, want original active lease", stored)
	}
}

func isExpectedLeaseRaceError(err error) bool {
	return errors.Is(err, taskpkg.ErrInvalidClaimToken) ||
		errors.Is(err, taskpkg.ErrInvalidStatusTransition)
}

func assertLeaseRaceWinner(
	ctx context.Context,
	t *testing.T,
	globalDB *GlobalDB,
	taskID string,
	winner *transitionResult,
) {
	t.Helper()

	stored, err := globalDB.GetTaskRun(ctx, winner.run.ID)
	if err != nil {
		t.Fatalf("GetTaskRun(%q) error = %v", winner.run.ID, err)
	}
	switch winner.name {
	case "release":
		if stored.Status != taskpkg.TaskRunStatusQueued ||
			stored.SessionID != "" ||
			stored.ClaimTokenHash != "" {
			t.Fatalf("release winner stored run = %#v, want queued and unowned", stored)
		}
	case "complete":
		if stored.Status != taskpkg.TaskRunStatusCompleted || stored.Result == nil {
			t.Fatalf("complete winner stored run = %#v, want completed with result", stored)
		}
	case "fail":
		if stored.Status != taskpkg.TaskRunStatusFailed || stored.Error != "worker failed during race" {
			t.Fatalf("fail winner stored run = %#v, want failed with race error", stored)
		}
	default:
		t.Fatalf("unexpected race winner %q", winner.name)
	}
	taskRecord, err := globalDB.GetTask(ctx, taskID)
	if err != nil {
		t.Fatalf("GetTask(%q) error = %v", taskID, err)
	}
	if taskRecord.CurrentRunID != "" {
		t.Fatalf("task.CurrentRunID = %q, want cleared after %s", taskRecord.CurrentRunID, winner.name)
	}
}
