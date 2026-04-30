package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/provider"
	"github.com/compozy/compozy/internal/core/reviews"
	workspacecfg "github.com/compozy/compozy/internal/core/workspace"
	"github.com/compozy/compozy/internal/store/globaldb"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func TestRunManagerReviewWatchCompletesCleanWithoutEmptyRound(t *testing.T) {
	reviewProvider := &fakeReviewWatchProvider{
		statuses: []provider.WatchStatus{currentWatchStatus("head-1")},
		fetches:  [][]provider.ReviewItem{{}},
	}
	git := &fakeReviewWatchGit{
		states: []ReviewWatchGitState{{
			HeadSHA:         "head-1",
			UpstreamRemote:  "origin",
			UpstreamBranch:  "feature",
			Dirty:           true,
			UnpushedCommits: 2,
		}},
	}
	env := newReviewWatchTestEnv(t, reviewProvider, git, runManagerTestDeps{})

	run := startReviewWatch(t, env, reviewWatchRequest(`{"run_id":"review-watch-clean"}`))
	row := waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusCompleted
	})
	if row.Mode != runModeReviewWatch {
		t.Fatalf("row.Mode = %q, want %q", row.Mode, runModeReviewWatch)
	}
	if _, err := os.Stat(env.workflowDir(env.workflowSlug) + "/reviews-001"); !os.IsNotExist(err) {
		t.Fatalf("reviews-001 stat error = %v, want not exist", err)
	}

	started := decodeReviewWatchPayload(t, requireRunEvent(t, run.RunID, eventspkg.EventKindReviewWatchStarted))
	if !started.Dirty || started.UnpushedCommits != 2 || started.HeadSHA != "head-1" {
		t.Fatalf("watch_started payload = %#v, want dirty/unpushed/head metadata", started)
	}
	requireRunEvent(t, run.RunID, eventspkg.EventKindReviewWatchClean)
}

func TestRunManagerReviewWatchPersistsRoundAndStartsOneChildRun(t *testing.T) {
	reviewProvider := &fakeReviewWatchProvider{
		statuses: []provider.WatchStatus{currentWatchStatus("head-1")},
		fetches:  [][]provider.ReviewItem{{watchReviewItem()}},
	}
	git := &fakeReviewWatchGit{
		states: []ReviewWatchGitState{
			{HeadSHA: "head-1", UpstreamRemote: "origin", UpstreamBranch: "feature"},
			{HeadSHA: "head-1", UpstreamRemote: "origin", UpstreamBranch: "feature"},
			{HeadSHA: "head-1", UpstreamRemote: "origin", UpstreamBranch: "feature"},
		},
	}
	env := newReviewWatchTestEnv(t, reviewProvider, git, runManagerTestDeps{
		execute: resolveReviewIssuesDuringRun(t),
	})

	run := startReviewWatch(t, env, reviewWatchRequest(`{"run_id":"review-watch-round"}`))
	row := waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusCompleted
	})
	if row.ErrorText != "" {
		t.Fatalf("watch row error = %q, want empty", row.ErrorText)
	}

	reviewDir := env.workflowDir(env.workflowSlug) + "/reviews-001"
	if _, err := os.Stat(reviewDir + "/issue_001.md"); err != nil {
		t.Fatalf("expected review issue in %s: %v", reviewDir, err)
	}
	runs, err := env.manager.List(context.Background(), apicore.RunListQuery{
		Workspace: env.workspaceRoot,
		Mode:      runModeReview,
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("List(review runs) error = %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("review child runs = %d, want 1: %#v", len(runs), runs)
	}
	fixStarted := decodeReviewWatchPayload(t, requireRunEvent(t, run.RunID, eventspkg.EventKindReviewWatchFixStarted))
	if fixStarted.ChildRunID != runs[0].RunID || fixStarted.Round != 1 {
		t.Fatalf("fix_started payload = %#v, want child %q round 1", fixStarted, runs[0].RunID)
	}
	requireRunEvent(t, run.RunID, eventspkg.EventKindReviewWatchMaxRounds)
}

func TestRunManagerReviewWatchRejectsDuplicateActiveWatch(t *testing.T) {
	reviewProvider := &fakeReviewWatchProvider{
		statuses: []provider.WatchStatus{{PRHeadSHA: "head-1", State: provider.WatchStatusPending}},
	}
	git := &fakeReviewWatchGit{states: []ReviewWatchGitState{{HeadSHA: "head-1"}}}
	env := newReviewWatchTestEnv(t, reviewProvider, git, runManagerTestDeps{})

	run := startReviewWatch(
		t,
		env,
		reviewWatchRequest(`{"run_id":"review-watch-duplicate-a"}`),
		func(req *apicore.ReviewWatchRequest) {
			req.ReviewTimeout = "10s"
		},
	)
	duplicateReq := reviewWatchRequest(`{"run_id":"review-watch-duplicate-b"}`)
	duplicateReq.ReviewTimeout = "10s"
	_, err := env.manager.StartReviewWatch(context.Background(), env.workspaceRoot, env.workflowSlug, duplicateReq)
	var problem *apicore.Problem
	if !errors.As(err, &problem) {
		t.Fatalf("duplicate StartReviewWatch error = %T %v, want problem", err, err)
	}
	if problem.Status != 409 || problem.Code != "review_watch_already_active" {
		t.Fatalf(
			"duplicate problem = status:%d code:%q, want 409 review_watch_already_active",
			problem.Status,
			problem.Code,
		)
	}

	if err := env.manager.Cancel(context.Background(), run.RunID); err != nil {
		t.Fatalf("Cancel(parent) error = %v", err)
	}
	waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusCancelled
	})
}

func TestRunManagerReviewWatchCancellationStopsProviderWaiting(t *testing.T) {
	reviewProvider := &fakeReviewWatchProvider{
		statuses: []provider.WatchStatus{{PRHeadSHA: "head-1", State: provider.WatchStatusPending}},
	}
	git := &fakeReviewWatchGit{states: []ReviewWatchGitState{{HeadSHA: "head-1"}}}
	env := newReviewWatchTestEnv(t, reviewProvider, git, runManagerTestDeps{})

	run := startReviewWatch(
		t,
		env,
		reviewWatchRequest(`{"run_id":"review-watch-cancel"}`),
		func(req *apicore.ReviewWatchRequest) {
			req.ReviewTimeout = "10s"
		},
	)
	waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusRunning
	})
	if err := env.manager.Cancel(context.Background(), run.RunID); err != nil {
		t.Fatalf("Cancel(parent) error = %v", err)
	}
	waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusCancelled
	})
	runs, err := env.manager.List(context.Background(), apicore.RunListQuery{
		Workspace: env.workspaceRoot,
		Mode:      runModeReview,
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("List(review runs) error = %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("child review runs = %d, want 0", len(runs))
	}
}

func TestRunManagerReviewWatchFailsWhenHeadDoesNotAdvanceBeforePush(t *testing.T) {
	reviewProvider := &fakeReviewWatchProvider{
		statuses: []provider.WatchStatus{currentWatchStatus("head-1")},
		fetches:  [][]provider.ReviewItem{{watchReviewItem()}},
	}
	git := &fakeReviewWatchGit{
		states: []ReviewWatchGitState{
			{HeadSHA: "head-1"},
			{HeadSHA: "head-1"},
			{HeadSHA: "head-1"},
		},
	}
	env := newReviewWatchTestEnv(t, reviewProvider, git, runManagerTestDeps{
		execute: resolveReviewIssuesDuringRun(t),
	})

	run := startReviewWatch(
		t,
		env,
		reviewWatchRequest(`{"run_id":"review-watch-no-commit"}`),
		func(req *apicore.ReviewWatchRequest) {
			req.AutoPush = true
			req.PushRemote = "origin"
			req.PushBranch = "feature"
		},
	)
	row := waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusFailed
	})
	if !strings.Contains(row.ErrorText, "without advancing HEAD") {
		t.Fatalf("row.ErrorText = %q, want unchanged HEAD failure", row.ErrorText)
	}
	if len(git.pushes) != 0 {
		t.Fatalf("pushes = %#v, want none", git.pushes)
	}
}

func TestRunManagerReviewWatchFailsWhenResolvedRoundStillHasUnresolvedIssues(t *testing.T) {
	reviewProvider := &fakeReviewWatchProvider{
		statuses: []provider.WatchStatus{currentWatchStatus("head-1")},
		fetches:  [][]provider.ReviewItem{{watchReviewItem()}},
	}
	git := &fakeReviewWatchGit{
		states: []ReviewWatchGitState{{HeadSHA: "head-1"}, {HeadSHA: "head-1"}, {HeadSHA: "head-1"}},
	}
	env := newReviewWatchTestEnv(t, reviewProvider, git, runManagerTestDeps{
		execute: func(context.Context, *model.SolvePreparation, *model.RuntimeConfig) error {
			return nil
		},
	})

	run := startReviewWatch(t, env, reviewWatchRequest(`{"run_id":"review-watch-unresolved"}`))
	row := waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusFailed
	})
	if !strings.Contains(row.ErrorText, "still has 1 unresolved") {
		t.Fatalf("row.ErrorText = %q, want unresolved verification failure", row.ErrorText)
	}
}

func TestRunManagerReviewWatchRejectsAutoPushWithoutTarget(t *testing.T) {
	reviewProvider := &fakeReviewWatchProvider{
		statuses: []provider.WatchStatus{currentWatchStatus("head-1")},
		fetches:  [][]provider.ReviewItem{{}},
	}
	git := &fakeReviewWatchGit{states: []ReviewWatchGitState{{HeadSHA: "head-1"}}}
	env := newReviewWatchTestEnv(t, reviewProvider, git, runManagerTestDeps{})

	run := startReviewWatch(
		t,
		env,
		reviewWatchRequest(`{"run_id":"review-watch-no-target"}`),
		func(req *apicore.ReviewWatchRequest) {
			req.AutoPush = true
		},
	)
	row := waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusFailed
	})
	if !strings.Contains(row.ErrorText, "auto_push requires push remote and branch") {
		t.Fatalf("row.ErrorText = %q, want auto_push target validation", row.ErrorText)
	}
}

func TestRunManagerReviewWatchRejectsAutoCommitFalseWithAutoPush(t *testing.T) {
	reviewProvider := &fakeReviewWatchProvider{
		statuses: []provider.WatchStatus{currentWatchStatus("head-1")},
		fetches:  [][]provider.ReviewItem{{}},
	}
	git := &fakeReviewWatchGit{states: []ReviewWatchGitState{{HeadSHA: "head-1"}}}
	env := newReviewWatchTestEnv(t, reviewProvider, git, runManagerTestDeps{})

	req := reviewWatchRequest(`{"run_id":"review-watch-autocommit-false","auto_commit":false}`)
	req.AutoPush = true
	req.PushRemote = "origin"
	req.PushBranch = "feature"
	_, err := env.manager.StartReviewWatch(context.Background(), env.workspaceRoot, env.workflowSlug, req)
	var problem *apicore.Problem
	if !errors.As(err, &problem) {
		t.Fatalf("StartReviewWatch() error = %T %v, want problem", err, err)
	}
	if problem.Status != 422 || problem.Code != "invalid_watch_request" {
		t.Fatalf("problem = status:%d code:%q, want 422 invalid_watch_request", problem.Status, problem.Code)
	}
}

func TestRunManagerReviewWatchPushesAndRepeatsUntilClean(t *testing.T) {
	reviewProvider := &fakeReviewWatchProvider{
		statuses: []provider.WatchStatus{
			currentWatchStatus("head-1"),
			currentWatchStatus("head-2"),
		},
		fetches: [][]provider.ReviewItem{
			{watchReviewItem()},
			{},
		},
	}
	git := &fakeReviewWatchGit{
		states: []ReviewWatchGitState{
			{HeadSHA: "head-1", UpstreamRemote: "origin", UpstreamBranch: "feature"},
			{HeadSHA: "head-1", UpstreamRemote: "origin", UpstreamBranch: "feature"},
			{HeadSHA: "head-2", UpstreamRemote: "origin", UpstreamBranch: "feature"},
		},
	}
	env := newReviewWatchTestEnv(t, reviewProvider, git, runManagerTestDeps{
		execute: resolveReviewIssuesDuringRun(t),
	})

	run := startReviewWatch(
		t,
		env,
		reviewWatchRequest(`{"run_id":"review-watch-push"}`),
		func(req *apicore.ReviewWatchRequest) {
			req.AutoPush = true
			req.PushRemote = "origin"
			req.PushBranch = "feature"
			req.QuietPeriod = "1ms"
			req.MaxRounds = 2
		},
	)
	row := waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusCompleted
	})
	if row.ErrorText != "" {
		t.Fatalf("row.ErrorText = %q, want empty", row.ErrorText)
	}
	if len(git.pushes) != 1 || git.pushes[0] != (reviewWatchPush{remote: "origin", branch: "feature"}) {
		t.Fatalf("pushes = %#v, want one origin/feature push", git.pushes)
	}
	requireRunEvent(t, run.RunID, eventspkg.EventKindReviewWatchPushCompleted)
	requireRunEvent(t, run.RunID, eventspkg.EventKindReviewWatchClean)
}

func TestRunManagerReviewWatchPrePushHookVetoStopsPush(t *testing.T) {
	reviewProvider := &fakeReviewWatchProvider{
		statuses: []provider.WatchStatus{currentWatchStatus("head-1")},
		fetches:  [][]provider.ReviewItem{{watchReviewItem()}},
	}
	git := &fakeReviewWatchGit{
		states: []ReviewWatchGitState{
			{HeadSHA: "head-1", UpstreamRemote: "origin", UpstreamBranch: "feature"},
			{HeadSHA: "head-1", UpstreamRemote: "origin", UpstreamBranch: "feature"},
			{HeadSHA: "head-2", UpstreamRemote: "origin", UpstreamBranch: "feature"},
		},
	}
	hooks := &reviewWatchTestHookManager{
		mutable: func(_ context.Context, hook string, payload any) (any, error) {
			if hook != reviewWatchHookPrePush {
				return payload, nil
			}
			updated, ok := payload.(reviewWatchPrePushHookPayload)
			if !ok {
				return payload, nil
			}
			updated.Push = false
			updated.StopReason = "release freeze"
			return updated, nil
		},
	}
	env := newReviewWatchTestEnv(t, reviewProvider, git, runManagerTestDeps{
		openRunScope: newTestOpenRunScope(hooks),
		execute:      resolveReviewIssuesDuringRun(t),
	})

	run := startReviewWatch(
		t,
		env,
		reviewWatchRequest(`{"run_id":"review-watch-pre-push-veto"}`),
		func(req *apicore.ReviewWatchRequest) {
			req.AutoPush = true
			req.PushRemote = "origin"
			req.PushBranch = "feature"
			req.MaxRounds = 2
		},
	)
	row := waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusCompleted
	})
	if row.ErrorText != "" {
		t.Fatalf("row.ErrorText = %q, want empty", row.ErrorText)
	}
	if len(git.pushes) != 0 {
		t.Fatalf("pushes = %#v, want none after pre-push veto", git.pushes)
	}
	if event, ok := findRunEvent(t, run.RunID, eventspkg.EventKindReviewWatchPushStarted); ok {
		t.Fatalf("unexpected push_started event after veto: %#v", event)
	}

	postRound, ok := hooks.lastObserver(reviewWatchHookPostRound).(reviewWatchPostRoundHookPayload)
	if !ok {
		t.Fatalf("post-round hook payload missing or wrong type: %#v", hooks.observed(reviewWatchHookPostRound))
	}
	if postRound.Status != "stopped" || postRound.StopReason != "release freeze" || postRound.Pushed {
		t.Fatalf("post-round payload = %#v, want stopped release freeze without push", postRound)
	}
	finished, ok := hooks.lastObserver(reviewWatchHookFinished).(reviewWatchFinishedHookPayload)
	if !ok {
		t.Fatalf("finished hook payload missing or wrong type: %#v", hooks.observed(reviewWatchHookFinished))
	}
	if !finished.Stopped || finished.TerminalReason != "review watch stopped: release freeze" {
		t.Fatalf("finished payload = %#v, want explicit stopped reason", finished)
	}
}

func TestRunManagerReviewWatchTwoRoundFlowWithTempGitRepository(t *testing.T) {
	var env *runManagerTestEnv
	reviewProvider := &fakeReviewWatchProvider{
		statusFunc: func(ctx context.Context) (provider.WatchStatus, error) {
			head, err := runGitOutputContext(ctx, env.workspaceRoot, "rev-parse", "HEAD")
			if err != nil {
				return provider.WatchStatus{}, err
			}
			return currentWatchStatus(head), nil
		},
		fetches: [][]provider.ReviewItem{
			{watchReviewItem()},
			{},
		},
	}
	env = newReviewWatchTestEnv(t, reviewProvider, newExecReviewWatchGit(), runManagerTestDeps{
		execute: resolveReviewIssuesAndCommitDuringRun(t),
	})
	remoteDir := initializeReviewWatchGitRepository(t, env)

	run := startReviewWatch(
		t,
		env,
		reviewWatchRequest(`{"run_id":"review-watch-temp-git"}`),
		func(req *apicore.ReviewWatchRequest) {
			req.AutoPush = true
			req.PushRemote = "origin"
			req.PushBranch = "feature"
			req.QuietPeriod = "1ms"
			req.MaxRounds = 2
		},
	)
	row := waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusCompleted
	})
	if row.ErrorText != "" {
		t.Fatalf("row.ErrorText = %q, want empty", row.ErrorText)
	}
	localHead := runGitOutput(t, env.workspaceRoot, "rev-parse", "HEAD")
	remoteHead := runGitOutput(t, remoteDir, "rev-parse", "refs/heads/feature")
	if localHead != remoteHead {
		t.Fatalf("remote feature head = %q, want local head %q", remoteHead, localHead)
	}
	requireRunEvent(t, run.RunID, eventspkg.EventKindReviewWatchFixStarted)
	requireRunEvent(t, run.RunID, eventspkg.EventKindReviewWatchPushCompleted)
	requireRunEvent(t, run.RunID, eventspkg.EventKindReviewWatchClean)
}

func TestReviewWatchHookPayloadMutabilityRules(t *testing.T) {
	beforeRound := reviewWatchPreRoundHookPayload{
		RunID:       "run-1",
		Provider:    "coderabbit",
		PR:          "123",
		Workflow:    "workflow",
		Round:       1,
		HeadSHA:     "head-1",
		ReviewID:    "review-1",
		ReviewState: "COMMENTED",
		Status:      string(provider.WatchStatusPending),
		Continue:    true,
	}
	afterRound := beforeRound
	afterRound.Status = string(provider.WatchStatusCurrentReviewed)
	err := validateReviewWatchPreRoundHookPayload(beforeRound, afterRound)
	if err == nil || !strings.Contains(err.Error(), "may only change") {
		t.Fatalf("pre-round immutable mutation error = %v, want clear allowlist rejection", err)
	}
	afterRound = beforeRound
	afterRound.Nitpicks = true
	afterRound.RuntimeOverrides = json.RawMessage(`{"model":"gpt-5.5"}`)
	afterRound.Batching = json.RawMessage(`{"concurrent":1}`)
	if err := validateReviewWatchPreRoundHookPayload(beforeRound, afterRound); err != nil {
		t.Fatalf("valid pre-round patch rejected: %v", err)
	}

	beforePush := reviewWatchPrePushHookPayload{
		RunID:    "run-1",
		Provider: "coderabbit",
		PR:       "123",
		Workflow: "workflow",
		Round:    1,
		HeadSHA:  "head-1",
		Remote:   "origin",
		Branch:   "feature",
		Push:     true,
	}
	afterPush := beforePush
	afterPush.HeadSHA = "head-2"
	err = validateReviewWatchPrePushHookPayload(beforePush, afterPush)
	if err == nil || !strings.Contains(err.Error(), "may only change") {
		t.Fatalf("pre-push immutable mutation error = %v, want clear allowlist rejection", err)
	}
	afterPush = beforePush
	afterPush.Remote = "fork"
	afterPush.Branch = "review-watch"
	if err := validateReviewWatchPrePushHookPayload(beforePush, afterPush); err != nil {
		t.Fatalf("valid pre-push patch rejected: %v", err)
	}
	afterPush.Push = false
	if err := validateReviewWatchPrePushHookPayload(beforePush, afterPush); err == nil ||
		!strings.Contains(err.Error(), "stop_reason") {
		t.Fatalf("pre-push veto without reason error = %v, want stop_reason requirement", err)
	}
}

func TestRunManagerReviewWatchFailureStates(t *testing.T) {
	testCases := []struct {
		name      string
		provider  *fakeReviewWatchProvider
		git       *fakeReviewWatchGit
		deps      runManagerTestDeps
		mutateReq func(*apicore.ReviewWatchRequest)
		wantError string
		wantEvent eventspkg.EventKind
	}{
		{
			name: "provider timeout",
			provider: &fakeReviewWatchProvider{
				statuses: []provider.WatchStatus{{PRHeadSHA: "head-1", State: provider.WatchStatusPending}},
			},
			git:       &fakeReviewWatchGit{states: []ReviewWatchGitState{{HeadSHA: "head-1"}}},
			wantError: "timed out",
		},
		{
			name: "provider error",
			provider: &fakeReviewWatchProvider{
				statusErr: errors.New("provider unavailable"),
			},
			git:       &fakeReviewWatchGit{states: []ReviewWatchGitState{{HeadSHA: "head-1"}}},
			wantError: "provider unavailable",
		},
		{
			name: "fetch error",
			provider: &fakeReviewWatchProvider{
				statuses: []provider.WatchStatus{currentWatchStatus("head-1")},
				fetchErr: errors.New("fetch unavailable"),
			},
			git:       &fakeReviewWatchGit{states: []ReviewWatchGitState{{HeadSHA: "head-1"}}},
			wantError: "fetch unavailable",
		},
		{
			name: "git state error",
			provider: &fakeReviewWatchProvider{
				statuses: []provider.WatchStatus{currentWatchStatus("head-1")},
			},
			git:       &fakeReviewWatchGit{stateErr: errors.New("git unavailable")},
			wantError: "git unavailable",
		},
		{
			name: "unknown provider",
			provider: &fakeReviewWatchProvider{
				statuses: []provider.WatchStatus{currentWatchStatus("head-1")},
			},
			git: &fakeReviewWatchGit{states: []ReviewWatchGitState{{HeadSHA: "head-1"}}},
			mutateReq: func(req *apicore.ReviewWatchRequest) {
				req.Provider = "missing"
			},
			wantError: "unknown review provider",
		},
		{
			name: "child failure",
			provider: &fakeReviewWatchProvider{
				statuses: []provider.WatchStatus{currentWatchStatus("head-1")},
				fetches:  [][]provider.ReviewItem{{watchReviewItem()}},
			},
			git: &fakeReviewWatchGit{
				states: []ReviewWatchGitState{{HeadSHA: "head-1"}, {HeadSHA: "head-1"}},
			},
			deps: runManagerTestDeps{
				prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
					return &model.SolvePreparation{}, nil
				},
				execute: func(context.Context, *model.SolvePreparation, *model.RuntimeConfig) error {
					return errors.New("child failed")
				},
			},
			wantError: "child failed",
		},
		{
			name: "child cancellation",
			provider: &fakeReviewWatchProvider{
				statuses: []provider.WatchStatus{currentWatchStatus("head-1")},
				fetches:  [][]provider.ReviewItem{{watchReviewItem()}},
			},
			git: &fakeReviewWatchGit{
				states: []ReviewWatchGitState{{HeadSHA: "head-1"}, {HeadSHA: "head-1"}},
			},
			deps: runManagerTestDeps{
				prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
					return &model.SolvePreparation{}, nil
				},
				execute: func(context.Context, *model.SolvePreparation, *model.RuntimeConfig) error {
					return context.Canceled
				},
			},
			wantError: "canceled",
		},
		{
			name: "push failure",
			provider: &fakeReviewWatchProvider{
				statuses: []provider.WatchStatus{currentWatchStatus("head-1")},
				fetches:  [][]provider.ReviewItem{{watchReviewItem()}},
			},
			git: &fakeReviewWatchGit{
				states: []ReviewWatchGitState{
					{HeadSHA: "head-1"},
					{HeadSHA: "head-1"},
					{HeadSHA: "head-2"},
				},
				pushErr: errors.New("push rejected"),
			},
			deps: runManagerTestDeps{
				execute: resolveReviewIssuesDuringRun(t),
			},
			mutateReq: func(req *apicore.ReviewWatchRequest) {
				req.AutoPush = true
				req.PushRemote = "origin"
				req.PushBranch = "feature"
			},
			wantError: "push rejected",
			wantEvent: eventspkg.EventKindReviewWatchPushFailed,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			env := newReviewWatchTestEnv(t, tc.provider, tc.git, tc.deps)
			run := startReviewWatch(
				t,
				env,
				reviewWatchRequest(`{"run_id":"`+strings.ReplaceAll(tc.name, " ", "-")+`"}`),
				tc.mutateReq,
			)
			row := waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
				return row.Status == runStatusFailed
			})
			if !strings.Contains(row.ErrorText, tc.wantError) {
				t.Fatalf("row.ErrorText = %q, want substring %q", row.ErrorText, tc.wantError)
			}
			if tc.wantEvent != "" {
				requireRunEvent(t, run.RunID, tc.wantEvent)
			}
		})
	}
}

func TestRunManagerReviewWatchStreamExposesParentEventsAndChildReference(t *testing.T) {
	reviewProvider := &fakeReviewWatchProvider{
		statuses: []provider.WatchStatus{currentWatchStatus("head-1")},
		fetches:  [][]provider.ReviewItem{{watchReviewItem()}},
	}
	git := &fakeReviewWatchGit{
		states: []ReviewWatchGitState{{HeadSHA: "head-1"}, {HeadSHA: "head-1"}, {HeadSHA: "head-1"}},
	}
	env := newReviewWatchTestEnv(t, reviewProvider, git, runManagerTestDeps{
		execute: resolveReviewIssuesDuringRun(t),
	})

	run := startReviewWatch(t, env, reviewWatchRequest(`{"run_id":"review-watch-stream"}`))
	stream, err := env.manager.OpenStream(context.Background(), run.RunID, apicore.StreamCursor{})
	if err != nil {
		t.Fatalf("OpenStream() error = %v", err)
	}
	defer func() {
		_ = stream.Close()
	}()

	var sawChild string
	for sawChild == "" {
		item := waitForStreamItem(t, stream.Events())
		if item.Event == nil || item.Event.Kind != eventspkg.EventKindReviewWatchFixStarted {
			continue
		}
		payload := decodeReviewWatchPayloadFromRaw(t, item.Event.Payload)
		sawChild = payload.ChildRunID
	}
	if sawChild == "" {
		t.Fatal("stream did not expose child run id")
	}

	waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusCompleted
	})
}

func TestReviewWatchOptionResolutionValidationAndConfigFallbacks(t *testing.T) {
	providerName := "coderabbit"
	pushRemote := "origin"
	pushBranch := "main"
	pollInterval := "2s"
	reviewTimeout := "3s"
	quietPeriod := "4s"
	autoPush := true
	untilClean := true
	maxRounds := 3

	options, err := resolveReviewWatchOptions(workspacecfg.ProjectConfig{
		FetchReviews: workspacecfg.FetchReviewsConfig{
			Provider: &providerName,
		},
		WatchReviews: workspacecfg.WatchReviewsConfig{
			MaxRounds:     &maxRounds,
			PollInterval:  &pollInterval,
			ReviewTimeout: &reviewTimeout,
			QuietPeriod:   &quietPeriod,
			AutoPush:      &autoPush,
			UntilClean:    &untilClean,
			PushRemote:    &pushRemote,
			PushBranch:    &pushBranch,
		},
	}, apicore.ReviewWatchRequest{PRRef: "123"})
	if err != nil {
		t.Fatalf("resolveReviewWatchOptions() error = %v", err)
	}
	if options.Provider != "coderabbit" || options.PR != "123" || !options.AutoPush || !options.UntilClean ||
		options.MaxRounds != 3 || options.PushRemote != "origin" || options.PushBranch != "main" ||
		options.PollInterval != 2*time.Second || options.ReviewTimeout != 3*time.Second ||
		options.QuietPeriod != 4*time.Second {
		t.Fatalf("unexpected options from config fallback: %#v", options)
	}

	if _, err := resolveReviewWatchOptions(
		workspacecfg.ProjectConfig{},
		apicore.ReviewWatchRequest{PRRef: "123"},
	); err == nil {
		t.Fatal("missing provider error = nil, want validation error")
	}
	if _, err := resolveReviewWatchOptions(
		workspacecfg.ProjectConfig{},
		apicore.ReviewWatchRequest{Provider: "coderabbit"},
	); err == nil {
		t.Fatal("missing pr_ref error = nil, want validation error")
	}
	if _, err := resolveReviewWatchDuration("0s", nil, time.Second, "poll_interval"); err == nil {
		t.Fatal("zero duration error = nil, want validation error")
	}
	if _, err := resolveReviewWatchDuration("bad", nil, time.Second, "poll_interval"); err == nil {
		t.Fatal("invalid duration error = nil, want validation error")
	}
}

func TestReviewWatchRuntimeOverrideHelpers(t *testing.T) {
	raw, err := reviewWatchChildRuntimeOverrides(nil, false)
	if err != nil {
		t.Fatalf("reviewWatchChildRuntimeOverrides(empty) error = %v", err)
	}
	if len(raw) != 0 {
		t.Fatalf("empty child runtime overrides = %s, want nil", raw)
	}
	raw, err = reviewWatchChildRuntimeOverrides(nil, true)
	if err != nil {
		t.Fatalf("reviewWatchChildRuntimeOverrides(empty auto push) error = %v", err)
	}
	if string(raw) != `{"auto_commit":true}` {
		t.Fatalf("empty auto-push child overrides = %s, want auto_commit true", raw)
	}

	raw, err = reviewWatchChildRuntimeOverrides(
		json.RawMessage(`{"run_id":"parent","auto_commit":false,"model":"gpt"}`),
		true,
	)
	if err != nil {
		t.Fatalf("reviewWatchChildRuntimeOverrides(auto push) error = %v", err)
	}
	if strings.Contains(string(raw), "run_id") || !strings.Contains(string(raw), `"auto_commit":true`) {
		t.Fatalf("child runtime overrides = %s, want run_id stripped and auto_commit forced", raw)
	}
	raw, err = reviewWatchChildRuntimeOverrides(json.RawMessage(`{"run_id":"parent"}`), false)
	if err != nil {
		t.Fatalf("reviewWatchChildRuntimeOverrides(strip only) error = %v", err)
	}
	if len(raw) != 0 {
		t.Fatalf("child runtime overrides = %s, want nil after stripping only run_id", raw)
	}
	if _, err := reviewWatchChildRuntimeOverrides(json.RawMessage(`{`), false); err == nil {
		t.Fatal("invalid child runtime overrides error = nil, want validation error")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitReviewWatchDuration(ctx, time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("waitReviewWatchDuration(canceled) = %v, want context.Canceled", err)
	}
	if err := waitReviewWatchDuration(context.Background(), 0); err != nil {
		t.Fatalf("waitReviewWatchDuration(zero) error = %v", err)
	}
	if err := reviewWatchContextError(
		context.DeadlineExceeded,
		"timed out",
	); !strings.Contains(
		err.Error(),
		"timed out",
	) {
		t.Fatalf("reviewWatchContextError() = %v, want timeout message", err)
	}
	if cloneReviewWatchKey(nil) != nil {
		t.Fatal("cloneReviewWatchKey(nil) != nil")
	}
	if got := cloneJSON(nil); got != nil {
		t.Fatalf("cloneJSON(nil) = %s, want nil", got)
	}
	cloned := cloneJSON(json.RawMessage(`{"a":1}`))
	if string(cloned) != `{"a":1}` {
		t.Fatalf("cloneJSON(nonempty) = %s, want original JSON", cloned)
	}
}

func TestReviewWatchResolverAndReservationHelpers(t *testing.T) {
	if resolveReviewProviderRegistryFactory(nil) == nil {
		t.Fatal("resolveReviewProviderRegistryFactory(nil) = nil, want default factory")
	}
	customFactory := func(context.Context, string, string) (provider.RegistryReader, func(), error) {
		return nil, nil, errors.New("unused")
	}
	if resolveReviewProviderRegistryFactory(customFactory) == nil {
		t.Fatal("resolveReviewProviderRegistryFactory(custom) = nil, want custom factory")
	}
	if resolveReviewWatchGit(nil) == nil {
		t.Fatal("resolveReviewWatchGit(nil) = nil, want default git")
	}
	customGit := &fakeReviewWatchGit{}
	if resolveReviewWatchGit(customGit) != customGit {
		t.Fatal("resolveReviewWatchGit(custom) did not return custom git")
	}

	env := newReviewWatchTestEnv(t, &fakeReviewWatchProvider{}, &fakeReviewWatchGit{}, runManagerTestDeps{})
	key := reviewWatchKey{WorkspaceID: " ws ", Provider: "CodeRabbit", PR: "123"}
	if err := env.manager.reserveReviewWatch(key); err != nil {
		t.Fatalf("reserveReviewWatch() error = %v", err)
	}
	if err := env.manager.reserveReviewWatch(key); err == nil {
		t.Fatal("duplicate reserveReviewWatch() error = nil, want conflict")
	}
	env.manager.releaseReviewWatch(key)
	if err := env.manager.reserveReviewWatch(key); err != nil {
		t.Fatalf("reserveReviewWatch(after release) error = %v", err)
	}
	env.manager.releaseReviewWatch(key)
	if err := env.manager.reserveReviewWatch(reviewWatchKey{}); err == nil {
		t.Fatal("reserveReviewWatch(incomplete) error = nil, want error")
	}
}

func newReviewWatchTestEnv(
	t *testing.T,
	reviewProvider provider.Provider,
	git ReviewWatchGit,
	deps runManagerTestDeps,
) *runManagerTestEnv {
	t.Helper()
	if deps.execute != nil && deps.prepare == nil {
		deps.prepare = func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
			return &model.SolvePreparation{}, nil
		}
	}
	deps.reviewProviderRegistry = func(context.Context, string, string) (provider.RegistryReader, func(), error) {
		registry := provider.NewRegistry()
		registry.Register(reviewProvider)
		return registry, func() {}, nil
	}
	deps.reviewWatchGit = git
	if deps.loadProjectConfig == nil {
		deps.loadProjectConfig = func(context.Context, string) (workspacecfg.ProjectConfig, error) {
			untilClean := true
			maxRounds := 1
			return workspacecfg.ProjectConfig{
				WatchReviews: workspacecfg.WatchReviewsConfig{
					UntilClean: &untilClean,
					MaxRounds:  &maxRounds,
				},
			}, nil
		}
	}
	return newRunManagerTestEnv(t, deps)
}

func startReviewWatch(
	t *testing.T,
	env *runManagerTestEnv,
	req apicore.ReviewWatchRequest,
	mutators ...func(*apicore.ReviewWatchRequest),
) apicore.Run {
	t.Helper()
	for _, mutate := range mutators {
		if mutate != nil {
			mutate(&req)
		}
	}
	run, err := env.manager.StartReviewWatch(context.Background(), env.workspaceRoot, env.workflowSlug, req)
	if err != nil {
		t.Fatalf("StartReviewWatch() error = %v", err)
	}
	return run
}

func reviewWatchRequest(runtimeOverrides string) apicore.ReviewWatchRequest {
	return apicore.ReviewWatchRequest{
		Workspace:        "",
		Provider:         "coderabbit",
		PRRef:            "123",
		UntilClean:       true,
		MaxRounds:        1,
		PollInterval:     "1ms",
		ReviewTimeout:    "20ms",
		QuietPeriod:      "1ms",
		RuntimeOverrides: json.RawMessage(runtimeOverrides),
	}
}

func currentWatchStatus(head string) provider.WatchStatus {
	return provider.WatchStatus{
		PRHeadSHA:       head,
		ReviewCommitSHA: head,
		ReviewID:        "review-" + head,
		ReviewState:     "COMMENTED",
		State:           provider.WatchStatusCurrentReviewed,
		SubmittedAt:     time.Now().UTC(),
	}
}

func watchReviewItem() provider.ReviewItem {
	return provider.ReviewItem{
		Title:       "Fix review issue",
		File:        "internal/app.go",
		Line:        12,
		Severity:    "medium",
		Author:      "coderabbitai",
		Body:        "Please fix this issue.",
		ProviderRef: "thread-1",
	}
}

func resolveReviewIssuesDuringRun(
	t *testing.T,
) func(context.Context, *model.SolvePreparation, *model.RuntimeConfig) error {
	t.Helper()
	return func(_ context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
		entries, err := reviews.ReadReviewEntries(cfg.ReviewsDir)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			updated := strings.Replace(entry.Content, "status: pending", "status: resolved", 1)
			if err := os.WriteFile(entry.AbsPath, []byte(updated), 0o600); err != nil {
				return err
			}
		}
		return nil
	}
}

func resolveReviewIssuesAndCommitDuringRun(
	t *testing.T,
) func(context.Context, *model.SolvePreparation, *model.RuntimeConfig) error {
	t.Helper()
	resolveIssues := resolveReviewIssuesDuringRun(t)
	return func(ctx context.Context, preparation *model.SolvePreparation, cfg *model.RuntimeConfig) error {
		if err := resolveIssues(ctx, preparation, cfg); err != nil {
			return err
		}
		if cfg == nil {
			return errors.New("runtime config is required")
		}
		reviewsDir := cfg.ReviewsDir
		if rel, err := filepath.Rel(cfg.WorkspaceRoot, cfg.ReviewsDir); err == nil {
			reviewsDir = rel
		}
		if _, err := runGitOutputContext(ctx, cfg.WorkspaceRoot, "add", reviewsDir); err != nil {
			return err
		}
		if _, err := runGitOutputContext(ctx, cfg.WorkspaceRoot, "commit", "-m", "resolve review round"); err != nil {
			return err
		}
		return nil
	}
}

func initializeReviewWatchGitRepository(t *testing.T, env *runManagerTestEnv) string {
	t.Helper()
	env.writeWorkflowFile(t, env.workflowSlug, "task_01.md", daemonTaskBody("pending", "Review watch temp git flow"))
	remoteDir := filepath.Join(t.TempDir(), "remote.git")
	if err := os.MkdirAll(remoteDir, 0o755); err != nil {
		t.Fatalf("mkdir remote dir: %v", err)
	}
	runGitOutput(t, env.workspaceRoot, "init", "--initial-branch=feature")
	runGitOutput(t, env.workspaceRoot, "config", "user.email", "review-watch@example.com")
	runGitOutput(t, env.workspaceRoot, "config", "user.name", "Review Watch Test")
	runGitOutput(t, env.workspaceRoot, "add", ".compozy/tasks/"+env.workflowSlug+"/task_01.md")
	runGitOutput(t, env.workspaceRoot, "commit", "-m", "initial workflow")
	runGitOutput(t, remoteDir, "init", "--bare")
	runGitOutput(t, env.workspaceRoot, "remote", "add", "origin", remoteDir)
	runGitOutput(t, env.workspaceRoot, "push", "-u", "origin", "HEAD:feature")
	return remoteDir
}

func runGitOutput(t *testing.T, workDir string, args ...string) string {
	t.Helper()
	output, err := runGitOutputContext(context.Background(), workDir, args...)
	if err != nil {
		t.Fatalf("git %v in %s failed: %v", args, workDir, err)
	}
	return output
}

func runGitOutputContext(ctx context.Context, workDir string, args ...string) (string, error) {
	cmdArgs := append([]string{"-C", workDir}, args...)
	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func requireRunEvent(t *testing.T, runID string, kind eventspkg.EventKind) eventspkg.Event {
	t.Helper()
	if event, ok := findRunEvent(t, runID, kind); ok {
		return event
	}
	events := allRunEvents(t, runID)
	t.Fatalf("run %s missing event %s; events=%v", runID, kind, eventKinds(events))
	return eventspkg.Event{}
}

func findRunEvent(t *testing.T, runID string, kind eventspkg.EventKind) (eventspkg.Event, bool) {
	t.Helper()
	for _, event := range allRunEvents(t, runID) {
		if event.Kind == kind {
			return event, true
		}
	}
	return eventspkg.Event{}, false
}

func allRunEvents(t *testing.T, runID string) []eventspkg.Event {
	t.Helper()
	runDB, err := openRunDBForRunID(context.Background(), runID)
	if err != nil {
		t.Fatalf("openRunDBForRunID(%q) error = %v", runID, err)
	}
	defer func() {
		_ = runDB.Close()
	}()
	result, err := runDB.ListEvents(context.Background(), 0, 0)
	if err != nil {
		t.Fatalf("ListEvents(%q) error = %v", runID, err)
	}
	return result.Events
}

func eventKinds(events []eventspkg.Event) []eventspkg.EventKind {
	kinds := make([]eventspkg.EventKind, 0, len(events))
	for _, event := range events {
		kinds = append(kinds, event.Kind)
	}
	return kinds
}

func decodeReviewWatchPayload(t *testing.T, event eventspkg.Event) kinds.ReviewWatchPayload {
	t.Helper()
	return decodeReviewWatchPayloadFromRaw(t, event.Payload)
}

func decodeReviewWatchPayloadFromRaw(t *testing.T, raw json.RawMessage) kinds.ReviewWatchPayload {
	t.Helper()
	var payload kinds.ReviewWatchPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode review watch payload: %v", err)
	}
	return payload
}

type fakeReviewWatchProvider struct {
	mu         sync.Mutex
	statuses   []provider.WatchStatus
	statusErr  error
	statusFunc func(context.Context) (provider.WatchStatus, error)
	fetches    [][]provider.ReviewItem
	fetchErr   error
}

var _ provider.WatchStatusProvider = (*fakeReviewWatchProvider)(nil)

func (*fakeReviewWatchProvider) Name() string {
	return "coderabbit"
}

func (p *fakeReviewWatchProvider) WatchStatus(
	ctx context.Context,
	_ provider.WatchStatusRequest,
) (provider.WatchStatus, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.statusErr != nil {
		return provider.WatchStatus{}, p.statusErr
	}
	if p.statusFunc != nil {
		return p.statusFunc(ctx)
	}
	if len(p.statuses) == 0 {
		return provider.WatchStatus{PRHeadSHA: "head", State: provider.WatchStatusPending}, nil
	}
	status := p.statuses[0]
	if len(p.statuses) > 1 {
		p.statuses = p.statuses[1:]
	}
	return status, nil
}

func (p *fakeReviewWatchProvider) FetchReviews(context.Context, provider.FetchRequest) ([]provider.ReviewItem, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.fetchErr != nil {
		return nil, p.fetchErr
	}
	if len(p.fetches) == 0 {
		return nil, nil
	}
	items := append([]provider.ReviewItem(nil), p.fetches[0]...)
	if len(p.fetches) > 1 {
		p.fetches = p.fetches[1:]
	}
	return items, nil
}

func (*fakeReviewWatchProvider) ResolveIssues(context.Context, string, []provider.ResolvedIssue) error {
	return nil
}

type fakeReviewWatchGit struct {
	mu       sync.Mutex
	states   []ReviewWatchGitState
	stateErr error
	pushErr  error
	pushes   []reviewWatchPush
}

var _ ReviewWatchGit = (*fakeReviewWatchGit)(nil)

type reviewWatchPush struct {
	remote string
	branch string
}

func (g *fakeReviewWatchGit) State(context.Context, string) (ReviewWatchGitState, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.stateErr != nil {
		return ReviewWatchGitState{}, g.stateErr
	}
	if len(g.states) == 0 {
		return ReviewWatchGitState{HeadSHA: "head"}, nil
	}
	state := g.states[0]
	if len(g.states) > 1 {
		g.states = g.states[1:]
	}
	return state, nil
}

func (g *fakeReviewWatchGit) Push(_ context.Context, _ string, remote string, branch string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.pushes = append(g.pushes, reviewWatchPush{remote: remote, branch: branch})
	return g.pushErr
}

type reviewWatchObservedHook struct {
	hook    string
	payload any
}

type reviewWatchTestHookManager struct {
	mu        sync.Mutex
	mutable   func(context.Context, string, any) (any, error)
	observers []reviewWatchObservedHook
}

func (*reviewWatchTestHookManager) Start(context.Context) error {
	return nil
}

func (m *reviewWatchTestHookManager) DispatchMutableHook(ctx context.Context, hook string, payload any) (any, error) {
	if m != nil && m.mutable != nil {
		return m.mutable(ctx, hook, payload)
	}
	return payload, nil
}

func (m *reviewWatchTestHookManager) DispatchObserverHook(_ context.Context, hook string, payload any) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.observers = append(m.observers, reviewWatchObservedHook{hook: hook, payload: payload})
}

func (*reviewWatchTestHookManager) Shutdown(context.Context) error {
	return nil
}

func (*reviewWatchTestHookManager) WaitForObserverHooks(context.Context) error {
	return nil
}

func (m *reviewWatchTestHookManager) observed(hook string) []any {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	payloads := make([]any, 0)
	for _, observed := range m.observers {
		if observed.hook == hook {
			payloads = append(payloads, observed.payload)
		}
	}
	return payloads
}

func (m *reviewWatchTestHookManager) lastObserver(hook string) any {
	payloads := m.observed(hook)
	if len(payloads) == 0 {
		return nil
	}
	return payloads[len(payloads)-1]
}
