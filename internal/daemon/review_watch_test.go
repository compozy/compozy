package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
	reusableagents "github.com/compozy/compozy/internal/core/agents"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/provider"
	"github.com/compozy/compozy/internal/core/reviews"
	execpkg "github.com/compozy/compozy/internal/core/run/exec"
	"github.com/compozy/compozy/internal/core/run/recovery"
	workspacecfg "github.com/compozy/compozy/internal/core/workspace"
	"github.com/compozy/compozy/internal/store/globaldb"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func TestRunManagerReviewWatchCompletesCleanWithoutEmptyRound(t *testing.T) {
	t.Run("Should complete clean review watch without creating an empty round", func(t *testing.T) {
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

		req := reviewWatchRequest("{\"run_id\":\"review-watch-clean\"}")
		req.PresentationMode = "ui"
		run := startReviewWatch(t, env, req)
		row := waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
			return row.Status == runStatusCompleted
		})
		if row.Mode != runModeReviewWatch {
			t.Fatalf("row.Mode = %q, want %q", row.Mode, runModeReviewWatch)
		}
		if row.PresentationMode != "ui" {
			t.Fatalf("row.PresentationMode = %q, want ui", row.PresentationMode)
		}
		if _, err := os.Stat(env.workflowDir(env.workflowSlug) + "/reviews-001"); !os.IsNotExist(err) {
			t.Fatalf("reviews-001 stat error = %v, want not exist", err)
		}

		started := decodeReviewWatchPayload(t, requireRunEvent(t, run.RunID, eventspkg.EventKindReviewWatchStarted))
		if !started.Dirty || started.UnpushedCommits != 2 || started.HeadSHA != "head-1" {
			t.Fatalf("watch_started payload = %#v, want dirty/unpushed/head metadata", started)
		}
		requireRunEvent(t, run.RunID, eventspkg.EventKindReviewWatchClean)
	})
}

func TestRunManagerReviewWatchCreatesReviewOnlyWorkflowDirectory(t *testing.T) {
	t.Run("Should create missing review-only workflow directory", func(t *testing.T) {
		reviewProvider := &fakeReviewWatchProvider{
			statuses: []provider.WatchStatus{currentWatchStatus("head-1")},
			fetches:  [][]provider.ReviewItem{{}},
		}
		git := &fakeReviewWatchGit{
			states: []ReviewWatchGitState{{HeadSHA: "head-1"}},
		}
		env := newReviewWatchTestEnv(t, reviewProvider, git, runManagerTestDeps{})
		slug := "pr-51"
		workflowDir := env.workflowDir(slug)
		if _, err := os.Stat(workflowDir); !os.IsNotExist(err) {
			t.Fatalf("workflow dir stat before start = %v, want not exist", err)
		}

		req := reviewWatchRequest("{\"run_id\":\"review-watch-pr-51\"}")
		req.PRRef = "51"
		run, err := env.manager.StartReviewWatch(
			context.Background(),
			env.workspaceRoot,
			slug,
			req,
		)
		if err != nil {
			t.Fatalf("StartReviewWatch() error = %v", err)
		}
		waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
			return row.Status == runStatusCompleted
		})
		info, err := os.Stat(workflowDir)
		if err != nil {
			t.Fatalf("workflow dir stat after start: %v", err)
		}
		if !info.IsDir() {
			t.Fatalf("workflow path is not a directory: %s", workflowDir)
		}
	})
}

func TestRunManagerReviewWatchRequiresExistingDirectoryForExplicitWorkflowName(t *testing.T) {
	t.Run("Should not create arbitrary missing workflow directory", func(t *testing.T) {
		reviewProvider := &fakeReviewWatchProvider{
			statuses: []provider.WatchStatus{currentWatchStatus("head-1")},
			fetches:  [][]provider.ReviewItem{{}},
		}
		git := &fakeReviewWatchGit{
			states: []ReviewWatchGitState{{HeadSHA: "head-1"}},
		}
		env := newReviewWatchTestEnv(t, reviewProvider, git, runManagerTestDeps{})
		slug := "my-featuer"
		workflowDir := env.workflowDir(slug)

		_, err := env.manager.StartReviewWatch(
			context.Background(),
			env.workspaceRoot,
			slug,
			reviewWatchRequest("{\"run_id\":\"review-watch-explicit-missing\"}"),
		)
		if err == nil || !strings.Contains(err.Error(), "review watch workflow directory not found") {
			t.Fatalf("StartReviewWatch() error = %v, want missing workflow directory", err)
		}
		if _, statErr := os.Stat(workflowDir); !os.IsNotExist(statErr) {
			t.Fatalf("workflow dir stat after failed start = %v, want not exist", statErr)
		}
	})
}

func TestRunManagerReviewWatchPersistsRoundAndStartsOneChildRun(t *testing.T) {
	t.Run("Should persist a review round and start one child run", func(t *testing.T) {
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
		if runs[0].ParentRunID != run.RunID {
			t.Fatalf("review child parent_run_id = %q, want %q", runs[0].ParentRunID, run.RunID)
		}
		fixStarted := decodeReviewWatchPayload(
			t,
			requireRunEvent(t, run.RunID, eventspkg.EventKindReviewWatchFixStarted),
		)
		if fixStarted.ChildRunID != runs[0].RunID || fixStarted.Round != 1 {
			t.Fatalf("fix_started payload = %#v, want child %q round 1", fixStarted, runs[0].RunID)
		}
		requireRunEvent(t, run.RunID, eventspkg.EventKindReviewWatchMaxRounds)
	})
}

func TestRunManagerReviewWatchChildRunMaxRetriesPrecedence(t *testing.T) {
	t.Run("Should default child max retries when no explicit value exists", func(t *testing.T) {
		got := runReviewWatchAndCaptureChildMaxRetries(t, nil)
		if got != 1 {
			t.Fatalf("child MaxRetries = %d, want default 1", got)
		}
	})

	t.Run("Should preserve project default max retries over watch child default", func(t *testing.T) {
		projectCfg := workspacecfg.ProjectConfig{
			Defaults: workspacecfg.DefaultsConfig{
				MaxRetries: intPtr(0),
			},
			WatchReviews: workspacecfg.WatchReviewsConfig{
				UntilClean: boolPtr(true),
				MaxRounds:  intPtr(1),
			},
		}
		got := runReviewWatchAndCaptureChildMaxRetries(t, &projectCfg)
		if got != 0 {
			t.Fatalf("child MaxRetries = %d, want project default 0", got)
		}
	})
}

func TestRunManagerReviewWatchCurrentSettledFetchesPendingItems(t *testing.T) {
	t.Run(
		"Should fetch and fix unresolved reviews when provider settled without a current review object",
		func(t *testing.T) {
			t.Parallel()

			reviewProvider := &fakeReviewWatchProvider{
				statuses: []provider.WatchStatus{settledWatchStatus("head-1", "old-head")},
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

			run := startReviewWatch(t, env, reviewWatchRequest("{\"run_id\":\"review-watch-settled-round\"}"))
			row := waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
				return row.Status == runStatusCompleted
			})
			if row.ErrorText != "" {
				t.Fatalf("watch row error = %q, want empty", row.ErrorText)
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
			roundFetched := decodeReviewWatchPayload(
				t,
				requireRunEvent(t, run.RunID, eventspkg.EventKindReviewWatchRoundFetched),
			)
			if roundFetched.HeadSHA != "head-1" || roundFetched.Round != 1 {
				t.Fatalf("round_fetched payload = %#v, want head-1 round 1", roundFetched)
			}
		},
	)
}

func TestRunManagerReviewWatchCurrentSettledCanCompleteClean(t *testing.T) {
	t.Run(
		"Should declare clean after provider settled current head and no unresolved reviews remain",
		func(t *testing.T) {
			t.Parallel()

			reviewProvider := &fakeReviewWatchProvider{
				statuses: []provider.WatchStatus{settledWatchStatus("head-1", "old-head")},
				fetches:  [][]provider.ReviewItem{{}},
			}
			git := &fakeReviewWatchGit{
				states: []ReviewWatchGitState{{HeadSHA: "head-1", UpstreamRemote: "origin", UpstreamBranch: "feature"}},
			}
			env := newReviewWatchTestEnv(t, reviewProvider, git, runManagerTestDeps{})

			run := startReviewWatch(t, env, reviewWatchRequest("{\"run_id\":\"review-watch-settled-clean\"}"))
			row := waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
				return row.Status == runStatusCompleted
			})
			if row.ErrorText != "" {
				t.Fatalf("watch row error = %q, want empty", row.ErrorText)
			}
			clean := decodeReviewWatchPayload(t, requireRunEvent(t, run.RunID, eventspkg.EventKindReviewWatchClean))
			if clean.Status != string(provider.WatchStatusCurrentSettled) || clean.HeadSHA != "head-1" {
				t.Fatalf("clean payload = %#v, want current_settled head-1", clean)
			}
		},
	)
}

func TestRunManagerReviewWatchRejectsDuplicateActiveWatch(t *testing.T) {
	t.Run("Should reject duplicate active review watch requests", func(t *testing.T) {
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
	})
}

func TestRunManagerReviewWatchCancellationStopsProviderWaiting(t *testing.T) {
	t.Run("Should stop provider waiting when the review watch is canceled", func(t *testing.T) {
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
	})
}

func TestRunManagerReviewWatchFailsWhenHeadDoesNotAdvanceBeforePush(t *testing.T) {
	t.Run("Should fail when HEAD does not advance before auto push", func(t *testing.T) {
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
	})
}

func TestRunManagerReviewWatchFailsWhenResolvedRoundStillHasUnresolvedIssues(t *testing.T) {
	t.Run("Should fail when a resolved round still has unresolved issues", func(t *testing.T) {
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
	})
}

func TestRunManagerReviewWatchRejectsAutoPushWithoutTarget(t *testing.T) {
	t.Run("Should reject auto push without a configured push target", func(t *testing.T) {
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
	})
}

func TestRunManagerReviewWatchRejectsAutoCommitFalseWithAutoPush(t *testing.T) {
	t.Run("Should reject auto push when runtime overrides disable auto commit", func(t *testing.T) {
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
	})
}

func TestRunManagerReviewWatchPushesAndRepeatsUntilClean(t *testing.T) {
	t.Run("Should push and repeat until the provider reports the PR clean", func(t *testing.T) {
		reviewProvider := &fakeReviewWatchProvider{
			statuses: []provider.WatchStatus{
				currentWatchStatus("head-1"),
				currentWatchStatus("head-1"),
				currentWatchStatus("head-2"),
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
				req.ReviewTimeout = "2s"
				req.MaxRounds = 2
			},
		)
		row := waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
			return isTerminalRunStatus(row.Status)
		})
		if row.Status != runStatusCompleted {
			t.Fatalf("row.Status = %q error = %q, want completed", row.Status, row.ErrorText)
		}
		if row.ErrorText != "" {
			t.Fatalf("row.ErrorText = %q, want empty", row.ErrorText)
		}
		if len(git.pushes) != 1 || git.pushes[0] != (reviewWatchPush{remote: "origin", branch: "feature"}) {
			t.Fatalf("pushes = %#v, want one origin/feature push", git.pushes)
		}
		requireRunEvent(t, run.RunID, eventspkg.EventKindReviewWatchPushCompleted)
		requireRunEvent(t, run.RunID, eventspkg.EventKindReviewWatchClean)
	})
}

func TestRunManagerReviewWatchPushesUnpushedHeadAtStartup(t *testing.T) {
	t.Run("Should push an already committed local head before waiting for provider review", func(t *testing.T) {
		git := &fakeReviewWatchGit{
			states: []ReviewWatchGitState{
				{
					HeadSHA:         "local-fix-head",
					UpstreamRemote:  "origin",
					UpstreamBranch:  "feature",
					UnpushedCommits: 1,
				},
				{
					HeadSHA:         "local-fix-head",
					UpstreamRemote:  "origin",
					UpstreamBranch:  "feature",
					UnpushedCommits: 0,
				},
			},
		}
		reviewProvider := &fakeReviewWatchProvider{
			statusFunc: func(context.Context) (provider.WatchStatus, error) {
				git.mu.Lock()
				pushed := len(git.pushes) > 0
				git.mu.Unlock()
				if !pushed {
					return currentWatchStatus("remote-reviewed-head"), nil
				}
				return currentWatchStatus("local-fix-head"), nil
			},
			fetches: [][]provider.ReviewItem{{}},
		}
		env := newReviewWatchTestEnv(t, reviewProvider, git, runManagerTestDeps{})

		run := startReviewWatch(
			t,
			env,
			reviewWatchRequest(`{"run_id":"review-watch-startup-push"}`),
			func(req *apicore.ReviewWatchRequest) {
				req.AutoPush = true
				req.ReviewTimeout = "1s"
			},
		)
		row := waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
			return isTerminalRunStatus(row.Status)
		})
		if row.Status != runStatusCompleted {
			t.Fatalf("row.Status = %q error = %q, want completed", row.Status, row.ErrorText)
		}
		if len(git.pushes) != 1 || git.pushes[0] != (reviewWatchPush{remote: "origin", branch: "feature"}) {
			t.Fatalf("pushes = %#v, want one startup push to origin/feature", git.pushes)
		}
		started := decodeReviewWatchPayload(
			t,
			requireRunEvent(t, run.RunID, eventspkg.EventKindReviewWatchPushStarted),
		)
		if started.Round != 0 ||
			started.Status != reviewWatchPushStatusStartup ||
			started.UnpushedCommits != 1 ||
			started.HeadSHA != "local-fix-head" {
			t.Fatalf("push_started payload = %#v, want startup metadata", started)
		}
		completed := decodeReviewWatchPayload(
			t,
			requireRunEvent(t, run.RunID, eventspkg.EventKindReviewWatchPushCompleted),
		)
		if completed.Round != 0 ||
			completed.Status != reviewWatchPushStatusStartup ||
			completed.UnpushedCommits != 1 ||
			completed.HeadSHA != "local-fix-head" {
			t.Fatalf("push_completed payload = %#v, want startup metadata", completed)
		}
		clean := decodeReviewWatchPayload(t, requireRunEvent(t, run.RunID, eventspkg.EventKindReviewWatchClean))
		if clean.HeadSHA != "local-fix-head" {
			t.Fatalf("clean payload = %#v, want provider-current local fix head", clean)
		}
	})
}

func TestRunManagerReviewWatchStartupPrePushHookVetoStopsWatch(t *testing.T) {
	t.Run("Should stop explicitly when the pre-push hook vetoes startup reconciliation", func(t *testing.T) {
		reviewProvider := &fakeReviewWatchProvider{}
		git := &fakeReviewWatchGit{
			states: []ReviewWatchGitState{{
				HeadSHA:         "local-fix-head",
				UpstreamRemote:  "origin",
				UpstreamBranch:  "feature",
				UnpushedCommits: 1,
			}},
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
		})

		run := startReviewWatch(
			t,
			env,
			reviewWatchRequest(`{"run_id":"review-watch-startup-veto"}`),
			func(req *apicore.ReviewWatchRequest) {
				req.AutoPush = true
			},
		)
		row := waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
			return isTerminalRunStatus(row.Status)
		})
		if row.Status != runStatusCompleted {
			t.Fatalf("row.Status = %q error = %q, want completed stopped run", row.Status, row.ErrorText)
		}
		if len(git.pushes) != 0 {
			t.Fatalf("pushes = %#v, want none after startup pre-push veto", git.pushes)
		}
		if event, ok := findRunEvent(t, run.RunID, eventspkg.EventKindReviewWatchPushStarted); ok {
			t.Fatalf("unexpected push_started event after startup veto: %#v", event)
		}
		finished, ok := hooks.lastObserver(reviewWatchHookFinished).(reviewWatchFinishedHookPayload)
		if !ok {
			t.Fatalf("finished hook payload missing or wrong type: %#v", hooks.observed(reviewWatchHookFinished))
		}
		if !finished.Stopped ||
			finished.TerminalReason != "review watch stopped: release freeze" ||
			finished.HeadSHA != "local-fix-head" {
			t.Fatalf("finished payload = %#v, want explicit startup stop reason", finished)
		}
	})
}

func TestRunManagerReviewWatchWaitsForProviderToSettleBeforeClean(t *testing.T) {
	t.Run("Should not declare clean while CodeRabbit is still processing the pushed head", func(t *testing.T) {
		reviewProvider := &fakeReviewWatchProvider{
			statuses: []provider.WatchStatus{
				currentWatchStatus("head-1"),
				currentWatchStatus("head-1"),
				currentWatchStatus("head-2"),
				{
					PRHeadSHA:       "head-2",
					ReviewCommitSHA: "head-2",
					ReviewID:        "review-head-2-in-progress",
					ReviewState:     "COMMENTED",
					State:           provider.WatchStatusPending,
					SubmittedAt:     time.Now().UTC(),
				},
				currentWatchStatus("head-2"),
				currentWatchStatus("head-2"),
			},
			fetches: [][]provider.ReviewItem{
				{watchReviewItem()},
				{watchReviewItem()},
			},
		}
		git := &fakeReviewWatchGit{
			states: []ReviewWatchGitState{
				{HeadSHA: "head-1", UpstreamRemote: "origin", UpstreamBranch: "feature"},
				{HeadSHA: "head-1", UpstreamRemote: "origin", UpstreamBranch: "feature"},
				{HeadSHA: "head-2", UpstreamRemote: "origin", UpstreamBranch: "feature"},
				{HeadSHA: "head-2", UpstreamRemote: "origin", UpstreamBranch: "feature"},
				{HeadSHA: "head-3", UpstreamRemote: "origin", UpstreamBranch: "feature"},
			},
		}
		env := newReviewWatchTestEnv(t, reviewProvider, git, runManagerTestDeps{
			execute: resolveReviewIssuesDuringRun(t),
		})

		run := startReviewWatch(
			t,
			env,
			reviewWatchRequest(`{"run_id":"review-watch-provider-settle"}`),
			func(req *apicore.ReviewWatchRequest) {
				req.AutoPush = true
				req.PushRemote = "origin"
				req.PushBranch = "feature"
				req.QuietPeriod = "1ms"
				req.ReviewTimeout = "2s"
				req.MaxRounds = 2
			},
		)
		row := waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
			return isTerminalRunStatus(row.Status)
		})
		if row.Status != runStatusCompleted {
			t.Fatalf("row.Status = %q error = %q, want completed", row.Status, row.ErrorText)
		}
		if row.ErrorText != "" {
			t.Fatalf("row.ErrorText = %q, want empty", row.ErrorText)
		}
		if _, ok := findRunEvent(t, run.RunID, eventspkg.EventKindReviewWatchClean); ok {
			t.Fatal("watch declared clean before the provider-settled actionable round was processed")
		}
		roundsFetched := 0
		for _, event := range allRunEvents(t, run.RunID) {
			if event.Kind == eventspkg.EventKindReviewWatchRoundFetched {
				roundsFetched++
			}
		}
		if roundsFetched != 2 {
			t.Fatalf("rounds fetched = %d, want 2", roundsFetched)
		}
		requireRunEvent(t, run.RunID, eventspkg.EventKindReviewWatchMaxRounds)
	})
}

func TestRunManagerReviewWatchWaitsForManualPushHeadBeforeClean(t *testing.T) {
	t.Run("Should not declare clean against the old PR head after a local non-auto-push fix", func(t *testing.T) {
		reviewProvider := &fakeReviewWatchProvider{
			statuses: []provider.WatchStatus{
				currentWatchStatus("head-1"),
				currentWatchStatus("head-1"),
			},
			fetches: [][]provider.ReviewItem{{watchReviewItem()}},
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
			reviewWatchRequest(`{"run_id":"review-watch-manual-push-head"}`),
			func(req *apicore.ReviewWatchRequest) {
				req.MaxRounds = 2
				req.ReviewTimeout = "20ms"
			},
		)
		row := waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
			return row.Status == runStatusFailed
		})
		if !strings.Contains(row.ErrorText, "timed out") {
			t.Fatalf("row.ErrorText = %q, want provider wait timeout", row.ErrorText)
		}
		if _, ok := findRunEvent(t, run.RunID, eventspkg.EventKindReviewWatchClean); ok {
			t.Fatal("watch declared clean against a PR head that did not include the local fix")
		}
	})
}

func TestRunManagerReviewWatchPrePushHookVetoStopsPush(t *testing.T) {
	t.Run("Should allow the pre-push hook to veto pushing", func(t *testing.T) {
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
	})
}

func TestRunManagerReviewWatchTwoRoundFlowWithTempGitRepository(t *testing.T) {
	t.Run("Should complete a two-round flow against a temporary git repository", func(t *testing.T) {
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
				req.ReviewTimeout = "2s"
				req.MaxRounds = 2
			},
		)
		row := waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
			return isTerminalRunStatus(row.Status)
		})
		if row.Status != runStatusCompleted {
			t.Fatalf("row.Status = %q error = %q, want completed", row.Status, row.ErrorText)
		}
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
	})
}

func TestRunManagerReviewWatchDaemonRecoveryRestartsFailedChildRun(t *testing.T) {
	t.Run("Should recover a failed review child run through the daemon recovery wrapper", func(t *testing.T) {
		requireGitForTaskMulti(t)

		const failedSafeName = "task"
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
		var executeMu sync.Mutex
		executeCalls := 0
		strategy := recovery.NewAgenticRemediation(recovery.WithPreparedPromptExecutor(
			func(
				_ context.Context,
				cfg *model.RuntimeConfig,
				_ string,
				_ *reusableagents.ExecutionContext,
				_ execpkg.SessionMCPBuilder,
			) (execpkg.PreparedPromptResult, error) {
				if cfg == nil {
					return execpkg.PreparedPromptResult{}, errors.New("recovery runtime config is required")
				}
				if cfg.RecoveryAttempt != 1 {
					return execpkg.PreparedPromptResult{}, fmt.Errorf(
						"recovery attempt = %d, want 1",
						cfg.RecoveryAttempt,
					)
				}
				if strings.TrimSpace(cfg.ParentRunID) == "" {
					return execpkg.PreparedPromptResult{}, errors.New("recovery parent run id is required")
				}
				if err := os.WriteFile(
					filepath.Join(cfg.WorkspaceRoot, "recovery-agent.txt"),
					[]byte("agentic recovery ran\n"),
					0o600,
				); err != nil {
					return execpkg.PreparedPromptResult{}, err
				}
				return execpkg.PreparedPromptResult{
					RunID:  "review-watch-recovery-agent",
					Output: "{\"decision\":\"fixed\",\"reason\":\"resolved child run failure\",\"changed_files\":[\"recovery-agent.txt\"]}",
				}, nil
			},
		))
		enabled := true
		maxAttempts := 1
		env = newReviewWatchTestEnv(t, reviewProvider, newExecReviewWatchGit(), runManagerTestDeps{
			loadProjectConfig: func(context.Context, string) (workspacecfg.ProjectConfig, error) {
				untilClean := true
				maxRounds := 2
				return workspacecfg.ProjectConfig{
					WatchReviews: workspacecfg.WatchReviewsConfig{
						UntilClean: &untilClean,
						MaxRounds:  &maxRounds,
					},
					Recovery: workspacecfg.AgentRecoveryConfig{
						Enabled:     &enabled,
						MaxAttempts: &maxAttempts,
					},
				}, nil
			},
			recoveryStrategy: strategy,
			prepare: func(
				_ context.Context,
				_ *model.RuntimeConfig,
				scope model.RunScope,
			) (*model.SolvePreparation, error) {
				if scope == nil {
					return nil, errors.New("run scope is required")
				}
				return &model.SolvePreparation{
					Jobs: []model.Job{{
						SafeName: failedSafeName,
					}},
					RunArtifacts: scope.RunArtifacts(),
				}, nil
			},
			execute: func(ctx context.Context, prep *model.SolvePreparation, cfg *model.RuntimeConfig) error {
				if prep == nil || len(prep.Jobs) != 1 || prep.Jobs[0].SafeName != failedSafeName {
					return fmt.Errorf("prepared jobs = %#v, want only %q", prep, failedSafeName)
				}
				executeMu.Lock()
				executeCalls++
				call := executeCalls
				executeMu.Unlock()
				if call == 1 {
					if err := writeDaemonTaskResultFixture(cfg, "failed", 1, "review child failed"); err != nil {
						return err
					}
					return errors.New("review child failed")
				}
				if err := resolveReviewIssuesDuringRun(t)(ctx, prep, cfg); err != nil {
					return err
				}
				return writeDaemonTaskResultFixture(cfg, "succeeded", 0, "")
			},
		})
		initializeReviewWatchGitRepository(t, env)

		run := startReviewWatch(
			t,
			env,
			reviewWatchRequest("{\"run_id\":\"review-watch-recovery\"}"),
			func(req *apicore.ReviewWatchRequest) {
				req.QuietPeriod = "1ms"
				req.ReviewTimeout = "2s"
				req.MaxRounds = 2
			},
		)
		row := waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
			return isTerminalRunStatus(row.Status)
		})
		if row.Status != runStatusCompleted {
			t.Fatalf("row.Status = %q error = %q, want completed", row.Status, row.ErrorText)
		}
		executeMu.Lock()
		calls := executeCalls
		executeMu.Unlock()
		if calls != 2 {
			t.Fatalf("execute calls = %d, want initial failure plus recovery restart", calls)
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
		childRunID := runs[0].RunID
		if runs[0].Status != runStatusCompleted || runs[0].ParentRunID != run.RunID {
			t.Fatalf("review child = %#v, want completed child of %s", runs[0], run.RunID)
		}
		started := requireRunEvent(t, childRunID, eventspkg.EventKindRunRecoveryStarted)
		var startedPayload kinds.RunRecoveryStartedPayload
		decodeRunEventPayload(t, started, &startedPayload)
		if startedPayload.Attempt != 1 || startedPayload.Strategy != "agentic" {
			t.Fatalf("recovery started payload = %#v", startedPayload)
		}
		restarting := requireRunEvent(t, childRunID, eventspkg.EventKindRunRecoveryRestarting)
		var restartingPayload kinds.RunRecoveryRestartingPayload
		decodeRunEventPayload(t, restarting, &restartingPayload)
		if !reflect.DeepEqual(restartingPayload.FailedJobIDs, []string{failedSafeName}) {
			t.Fatalf("restarting failed jobs = %#v, want %q", restartingPayload.FailedJobIDs, failedSafeName)
		}
		recovered := requireRunEvent(t, childRunID, eventspkg.EventKindRunRecovered)
		var recoveredPayload kinds.RunRecoveredPayload
		decodeRunEventPayload(t, recovered, &recoveredPayload)
		if recoveredPayload.Attempts != 1 {
			t.Fatalf("recovered attempts = %d, want 1", recoveredPayload.Attempts)
		}
		requireRunEvent(t, run.RunID, eventspkg.EventKindReviewWatchFixCompleted)
		requireRunEvent(t, run.RunID, eventspkg.EventKindReviewWatchClean)
	})
}

func TestReviewWatchHookPayloadMutabilityRules(t *testing.T) {
	t.Run("Should enforce review watch hook payload mutability rules", func(t *testing.T) {
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
	})
}

func TestRunManagerReviewWatchFailureStates(t *testing.T) {
	t.Run("Should surface review watch failure states", func(t *testing.T) {
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
				name: "startup push failure",
				provider: &fakeReviewWatchProvider{
					statuses: []provider.WatchStatus{currentWatchStatus("local-fix-head")},
					fetches:  [][]provider.ReviewItem{{}},
				},
				git: &fakeReviewWatchGit{
					states: []ReviewWatchGitState{{
						HeadSHA:         "local-fix-head",
						UpstreamRemote:  "origin",
						UpstreamBranch:  "feature",
						UnpushedCommits: 1,
					}},
					pushErr: errors.New("startup push rejected"),
				},
				mutateReq: func(req *apicore.ReviewWatchRequest) {
					req.AutoPush = true
				},
				wantError: "startup push rejected",
				wantEvent: eventspkg.EventKindReviewWatchPushFailed,
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
				t.Parallel()

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
	})
}

func TestRunManagerReviewWatchStreamExposesParentEventsAndChildReference(t *testing.T) {
	t.Run("Should expose parent events and child run references on the stream", func(t *testing.T) {
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
	})
}

func TestReviewWatchOptionResolutionValidationAndConfigFallbacks(t *testing.T) {
	t.Run("Should resolve watch options and validate invalid inputs", func(t *testing.T) {
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
		); err == nil || !strings.Contains(err.Error(), "requires provider") {
			t.Fatalf("missing provider error = %v, want provider validation error", err)
		}
		if _, err := resolveReviewWatchOptions(
			workspacecfg.ProjectConfig{},
			apicore.ReviewWatchRequest{Provider: "coderabbit"},
		); err == nil || !strings.Contains(err.Error(), "requires pr_ref") {
			t.Fatalf("missing pr_ref error = %v, want pr_ref validation error", err)
		}
		if _, err := resolveReviewWatchDuration("0s", nil, time.Second, "poll_interval"); err == nil ||
			!strings.Contains(err.Error(), "poll_interval must be a positive duration") {
			t.Fatalf("zero duration error = %v, want positive duration validation error", err)
		}
		if _, err := resolveReviewWatchDuration("bad", nil, time.Second, "poll_interval"); err == nil ||
			!strings.Contains(err.Error(), "poll_interval must be a positive duration") {
			t.Fatalf("invalid duration error = %v, want duration parse validation error", err)
		}
	})
}

func TestReviewWatchRuntimeOverrideHelpers(t *testing.T) {
	t.Run("Should normalize review watch runtime override helpers", func(t *testing.T) {
		raw, err := reviewWatchChildRuntimeOverrides(nil, false, false)
		if err != nil {
			t.Fatalf("reviewWatchChildRuntimeOverrides(empty) error = %v", err)
		}
		requireRuntimeOverrideMaxRetries(t, raw, 1)
		raw, err = reviewWatchChildRuntimeOverrides(nil, true, false)
		if err != nil {
			t.Fatalf("reviewWatchChildRuntimeOverrides(empty auto push) error = %v", err)
		}
		requireRuntimeOverrideBool(t, raw, "auto_commit", true)
		requireRuntimeOverrideMaxRetries(t, raw, 1)

		raw, err = reviewWatchChildRuntimeOverrides(
			json.RawMessage(`{"run_id":"parent","auto_commit":false,"model":"gpt"}`),
			true,
			false,
		)
		if err != nil {
			t.Fatalf("reviewWatchChildRuntimeOverrides(auto push) error = %v", err)
		}
		if strings.Contains(string(raw), "run_id") || !strings.Contains(string(raw), `"auto_commit":true`) {
			t.Fatalf("child runtime overrides = %s, want run_id stripped and auto_commit forced", raw)
		}
		requireRuntimeOverrideMaxRetries(t, raw, 1)
		raw, err = reviewWatchChildRuntimeOverrides(
			json.RawMessage(
				`{"run_id":"parent","recovery":{"enabled":true,"ide":"codex","model":"gpt-5.5","reasoning_effort":"high","max_attempts":2}}`,
			),
			false,
			false,
		)
		if err != nil {
			t.Fatalf("reviewWatchChildRuntimeOverrides(recovery) error = %v", err)
		}
		var childOverrides runtimeOverrideInput
		if err := json.Unmarshal(raw, &childOverrides); err != nil {
			t.Fatalf("decode child recovery overrides: %v", err)
		}
		if childOverrides.Recovery == nil ||
			childOverrides.Recovery.Enabled == nil || !*childOverrides.Recovery.Enabled ||
			childOverrides.Recovery.IDE == nil || *childOverrides.Recovery.IDE != "codex" ||
			childOverrides.Recovery.Model == nil || *childOverrides.Recovery.Model != "gpt-5.5" ||
			childOverrides.Recovery.ReasoningEffort == nil || *childOverrides.Recovery.ReasoningEffort != "high" ||
			childOverrides.Recovery.MaxAttempts == nil || *childOverrides.Recovery.MaxAttempts != 2 {
			t.Fatalf("unexpected child recovery override: %#v", childOverrides.Recovery)
		}
		raw, err = reviewWatchChildRuntimeOverrides(json.RawMessage(`{"run_id":"parent"}`), false, false)
		if err != nil {
			t.Fatalf("reviewWatchChildRuntimeOverrides(strip only) error = %v", err)
		}
		requireRuntimeOverrideMaxRetries(t, raw, 1)
		raw, err = reviewWatchChildRuntimeOverrides(
			json.RawMessage(`{"run_id":"parent","max_retries":0}`),
			false,
			false,
		)
		if err != nil {
			t.Fatalf("reviewWatchChildRuntimeOverrides(explicit retries) error = %v", err)
		}
		requireRuntimeOverrideMaxRetries(t, raw, 0)
		raw, err = reviewWatchChildRuntimeOverrides(nil, false, true)
		if err != nil {
			t.Fatalf("reviewWatchChildRuntimeOverrides(project retries) error = %v", err)
		}
		if len(raw) != 0 {
			t.Fatalf("project max_retries child overrides = %s, want nil", raw)
		}
		raw, err = reviewWatchChildRuntimeOverrides(nil, true, true)
		if err != nil {
			t.Fatalf("reviewWatchChildRuntimeOverrides(project retries auto push) error = %v", err)
		}
		requireRuntimeOverrideBool(t, raw, "auto_commit", true)
		requireRuntimeOverrideNoMaxRetries(t, raw)
		if _, err := reviewWatchChildRuntimeOverrides(json.RawMessage(`{`), false, false); err == nil {
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
	})
}

func requireRuntimeOverrideMaxRetries(t *testing.T, raw json.RawMessage, want int) {
	t.Helper()
	var got runtimeOverrideInput
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode runtime overrides %s: %v", raw, err)
	}
	if got.MaxRetries == nil || *got.MaxRetries != want {
		t.Fatalf("runtime overrides %s max_retries = %#v, want %d", raw, got.MaxRetries, want)
	}
}

func requireRuntimeOverrideNoMaxRetries(t *testing.T, raw json.RawMessage) {
	t.Helper()
	var got runtimeOverrideInput
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode runtime overrides %s: %v", raw, err)
	}
	if got.MaxRetries != nil {
		t.Fatalf("runtime overrides %s max_retries = %#v, want nil", raw, got.MaxRetries)
	}
}

func requireRuntimeOverrideBool(t *testing.T, raw json.RawMessage, field string, want bool) {
	t.Helper()
	values := make(map[string]json.RawMessage)
	if err := json.Unmarshal(raw, &values); err != nil {
		t.Fatalf("decode runtime overrides %s: %v", raw, err)
	}
	var got bool
	if err := json.Unmarshal(values[field], &got); err != nil {
		t.Fatalf("decode runtime override %s from %s: %v", field, raw, err)
	}
	if got != want {
		t.Fatalf("runtime overrides %s %s = %t, want %t", raw, field, got, want)
	}
}

func runReviewWatchAndCaptureChildMaxRetries(
	t *testing.T,
	projectCfg *workspacecfg.ProjectConfig,
) int {
	t.Helper()
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
	captured := make(chan int, 1)
	resolveIssues := resolveReviewIssuesDuringRun(t)
	deps := runManagerTestDeps{
		execute: func(ctx context.Context, preparation *model.SolvePreparation, cfg *model.RuntimeConfig) error {
			captured <- cfg.MaxRetries
			return resolveIssues(ctx, preparation, cfg)
		},
	}
	if projectCfg != nil {
		deps.loadProjectConfig = func(context.Context, string) (workspacecfg.ProjectConfig, error) {
			return *projectCfg, nil
		}
	}
	env := newReviewWatchTestEnv(t, reviewProvider, git, deps)

	run := startReviewWatch(t, env, reviewWatchRequest(`{"run_id":"review-watch-child-retries"}`))
	waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusCompleted
	})
	select {
	case got := <-captured:
		return got
	default:
		t.Fatal("child review run did not execute")
		return 0
	}
}

func TestReviewWatchResolverAndReservationHelpers(t *testing.T) {
	t.Run("Should resolve helper defaults and manage review watch reservations", func(t *testing.T) {
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
	})
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

func settledWatchStatus(head string, reviewHead string) provider.WatchStatus {
	return provider.WatchStatus{
		PRHeadSHA:                 head,
		ReviewCommitSHA:           reviewHead,
		ReviewID:                  "review-" + reviewHead,
		ReviewState:               "COMMENTED",
		ProviderStatusState:       "success",
		ProviderStatusDescription: "Review completed",
		ProviderStatusUpdatedAt:   time.Now().UTC(),
		State:                     provider.WatchStatusCurrentSettled,
		SubmittedAt:               time.Now().UTC().Add(-time.Minute),
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
		if cfg == nil {
			return errors.New("runtime config is required")
		}
		if err := resolveIssues(ctx, preparation, cfg); err != nil {
			return err
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

func decodeRunEventPayload(t *testing.T, event eventspkg.Event, dst any) {
	t.Helper()
	if err := json.Unmarshal(event.Payload, dst); err != nil {
		t.Fatalf("decode %s payload: %v", event.Kind, err)
	}
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
