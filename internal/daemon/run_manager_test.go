package daemon

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
	corepkg "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/plan"
	workspacecfg "github.com/compozy/compozy/internal/core/workspace"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/store/rundb"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func TestRunManagerStartTaskRunAllocatesRunDBAndRejectsDuplicateRunID(t *testing.T) {
	env := newRunManagerTestEnv(t, runManagerTestDeps{})

	run := env.startTaskRun(t, "task-run-duplicate", nil)
	terminal := waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return isTerminalRunStatus(row.Status)
	})
	if terminal.Status != runStatusCompleted {
		t.Fatalf("terminal status = %q, want %q", terminal.Status, runStatusCompleted)
	}

	runArtifacts, err := model.ResolveHomeRunArtifacts(run.RunID)
	if err != nil {
		t.Fatalf("ResolveHomeRunArtifacts(%q) error = %v", run.RunID, err)
	}
	if _, err := os.Stat(runArtifacts.RunDBPath); err != nil {
		t.Fatalf("stat run.db %q: %v", runArtifacts.RunDBPath, err)
	}

	row, err := env.globalDB.GetRun(context.Background(), run.RunID)
	if err != nil {
		t.Fatalf("GetRun(%q) error = %v", run.RunID, err)
	}
	if row.Mode != runModeTask {
		t.Fatalf("row.Mode = %q, want %q", row.Mode, runModeTask)
	}
	if row.PresentationMode != defaultPresentationMode {
		t.Fatalf("row.PresentationMode = %q, want %q", row.PresentationMode, defaultPresentationMode)
	}

	_, err = env.manager.StartTaskRun(context.Background(), env.workspaceRoot, env.workflowSlug, apicore.TaskRunRequest{
		Workspace:        env.workspaceRoot,
		PresentationMode: defaultPresentationMode,
		RuntimeOverrides: rawJSON(t, `{"run_id":"task-run-duplicate"}`),
	})
	if !errors.Is(err, globaldb.ErrRunAlreadyExists) {
		t.Fatalf("StartTaskRun(duplicate) error = %v, want ErrRunAlreadyExists", err)
	}
}

func TestRunManagerCancelTaskRunMirrorsTerminalStateAndIsIdempotent(t *testing.T) {
	started := make(chan string, 1)
	env := newRunManagerTestEnv(t, runManagerTestDeps{
		prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
			return &model.SolvePreparation{}, nil
		},
		execute: func(ctx context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
			started <- cfg.RunID
			<-ctx.Done()
			return ctx.Err()
		},
	})

	run := env.startTaskRun(t, "task-run-cancel", nil)
	waitForString(t, started, run.RunID)

	if err := env.manager.Cancel(context.Background(), run.RunID); err != nil {
		t.Fatalf("Cancel(first) error = %v", err)
	}
	if err := env.manager.Cancel(context.Background(), run.RunID); err != nil {
		t.Fatalf("Cancel(second) error = %v", err)
	}

	terminal := waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusCancelled
	})
	if terminal.EndedAt == nil {
		t.Fatal("EndedAt = nil, want terminal timestamp")
	}
	if strings.TrimSpace(terminal.ErrorText) == "" {
		t.Fatal("ErrorText = empty, want cancellation reason")
	}

	lastEvent := env.lastRunEvent(t, run.RunID)
	if lastEvent == nil {
		t.Fatal("LastEvent() = nil, want terminal event")
	}
	if lastEvent.Kind != eventspkg.EventKindRunCancelled {
		t.Fatalf("last event kind = %q, want %q", lastEvent.Kind, eventspkg.EventKindRunCancelled)
	}

	if err := env.manager.Cancel(context.Background(), run.RunID); err != nil {
		t.Fatalf("Cancel(terminal) error = %v", err)
	}
}

func TestRunManagerSnapshotIncludesJobsTranscriptAndNextCursor(t *testing.T) {
	executed := make(chan string, 1)
	env := newRunManagerTestEnv(t, runManagerTestDeps{
		prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
			return &model.SolvePreparation{}, nil
		},
		execute: func(ctx context.Context, prep *model.SolvePreparation, cfg *model.RuntimeConfig) error {
			runArtifacts, err := model.ResolveHomeRunArtifacts(cfg.RunID)
			if err != nil {
				return err
			}

			submitEvent(ctx, t, prep.Journal(), cfg.RunID, eventspkg.EventKindJobQueued, kinds.JobQueuedPayload{
				Index:     1,
				SafeName:  "job-001",
				TaskTitle: "daemon-run-manager",
			})
			submitEvent(ctx, t, prep.Journal(), cfg.RunID, eventspkg.EventKindJobStarted, kinds.JobStartedPayload{
				JobAttemptInfo: kinds.JobAttemptInfo{Index: 1, Attempt: 1, MaxAttempts: 1},
			})
			textBlock, err := kinds.NewContentBlock(kinds.TextBlock{Text: "hello from daemon attach"})
			if err != nil {
				return err
			}
			submitEvent(ctx, t, prep.Journal(), cfg.RunID, eventspkg.EventKindSessionUpdate, kinds.SessionUpdatePayload{
				Index: 1,
				Update: kinds.SessionUpdate{
					Kind:   kinds.UpdateKindAgentMessageChunk,
					Status: kinds.StatusRunning,
					Blocks: []kinds.ContentBlock{textBlock},
				},
			})
			submitEvent(ctx, t, prep.Journal(), cfg.RunID, eventspkg.EventKindJobCompleted, kinds.JobCompletedPayload{
				JobAttemptInfo: kinds.JobAttemptInfo{Index: 1, Attempt: 1, MaxAttempts: 1},
			})
			submitEvent(ctx, t, prep.Journal(), cfg.RunID, eventspkg.EventKindRunCompleted, kinds.RunCompletedPayload{
				ArtifactsDir:   runArtifacts.RunDir,
				ResultPath:     runArtifacts.ResultPath,
				SummaryMessage: "completed for snapshot",
			})
			executed <- cfg.RunID
			return nil
		},
	})

	run := env.startTaskRun(t, "task-run-snapshot", nil)
	waitForString(t, executed, run.RunID)
	waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusCompleted
	})

	snapshot, err := env.manager.Snapshot(context.Background(), run.RunID)
	if err != nil {
		t.Fatalf("Snapshot(%q) error = %v", run.RunID, err)
	}
	if snapshot.Run.Mode != runModeTask {
		t.Fatalf("snapshot mode = %q, want %q", snapshot.Run.Mode, runModeTask)
	}
	if len(snapshot.Jobs) != 1 {
		t.Fatalf("len(snapshot.Jobs) = %d, want 1", len(snapshot.Jobs))
	}
	if snapshot.Jobs[0].Status != "completed" {
		t.Fatalf("snapshot job status = %q, want completed", snapshot.Jobs[0].Status)
	}
	if len(snapshot.Transcript) != 1 {
		t.Fatalf("len(snapshot.Transcript) = %d, want 1", len(snapshot.Transcript))
	}
	if snapshot.Transcript[0].Content != "hello from daemon attach" {
		t.Fatalf("transcript content = %q, want agent text", snapshot.Transcript[0].Content)
	}
	if snapshot.NextCursor == nil || snapshot.NextCursor.Sequence == 0 {
		t.Fatalf("NextCursor = %#v, want persisted cursor", snapshot.NextCursor)
	}

	page, err := env.manager.Events(context.Background(), run.RunID, apicore.RunEventPageQuery{})
	if err != nil {
		t.Fatalf("Events(%q) error = %v", run.RunID, err)
	}
	if len(page.Events) < 5 {
		t.Fatalf("len(page.Events) = %d, want at least 5", len(page.Events))
	}
	if page.NextCursor == nil || page.NextCursor.Sequence == 0 {
		t.Fatalf("Events.NextCursor = %#v, want non-zero cursor", page.NextCursor)
	}
}

func TestRunManagerModeSpecificStartsProduceSharedLifecycleContract(t *testing.T) {
	env := newRunManagerTestEnv(t, runManagerTestDeps{})
	env.createReviewRound(t, 1)

	taskRun := env.startTaskRun(t, "task-mode-run", nil)
	reviewRun := env.startReviewRun(t, "review-mode-run", 1, nil, nil)
	execRun := env.startExecRun(t, "exec-mode-run", nil)

	for _, run := range []apicore.Run{taskRun, reviewRun, execRun} {
		waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
			return isTerminalRunStatus(row.Status)
		})
	}

	runs, err := env.manager.List(context.Background(), apicore.RunListQuery{Workspace: env.workspaceRoot, Limit: 10})
	if err != nil {
		t.Fatalf("List(workspace) error = %v", err)
	}
	if len(runs) != 3 {
		t.Fatalf("len(runs) = %d, want 3", len(runs))
	}

	modes := []string{taskRun.Mode, reviewRun.Mode, execRun.Mode}
	slices.Sort(modes)
	wantModes := []string{runModeExec, runModeReview, runModeTask}
	slices.Sort(wantModes)
	if !slices.Equal(modes, wantModes) {
		t.Fatalf("run modes = %#v, want %#v", modes, wantModes)
	}

	execRuns, err := env.manager.List(context.Background(), apicore.RunListQuery{
		Workspace: env.workspaceRoot,
		Mode:      runModeExec,
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("List(exec) error = %v", err)
	}
	if len(execRuns) != 1 || execRuns[0].RunID != execRun.RunID {
		t.Fatalf("exec runs = %#v, want %q", execRuns, execRun.RunID)
	}

	reviewSnapshot, err := env.manager.Snapshot(context.Background(), reviewRun.RunID)
	if err != nil {
		t.Fatalf("Snapshot(review) error = %v", err)
	}
	if reviewSnapshot.Run.Mode != runModeReview {
		t.Fatalf("review snapshot mode = %q, want %q", reviewSnapshot.Run.Mode, runModeReview)
	}

	execRow, err := env.globalDB.GetRun(context.Background(), execRun.RunID)
	if err != nil {
		t.Fatalf("GetRun(exec) error = %v", err)
	}
	if execRow.Mode != runModeExec {
		t.Fatalf("exec row mode = %q, want %q", execRow.Mode, runModeExec)
	}
}

func TestRunManagerStartFailureBeforeChildExecutionMarksRunFailed(t *testing.T) {
	runtimeManager := &stubRuntimeManager{startErr: errors.New("runtime failed to start")}
	var executeCalled atomic.Bool
	env := newRunManagerTestEnv(t, runManagerTestDeps{
		openRunScope: newTestOpenRunScope(runtimeManager),
		prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
			return &model.SolvePreparation{}, nil
		},
		execute: func(context.Context, *model.SolvePreparation, *model.RuntimeConfig) error {
			executeCalled.Store(true)
			return nil
		},
	})

	run := env.startTaskRun(t, "task-run-start-failure", nil)
	terminal := waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusFailed
	})
	if executeCalled.Load() {
		t.Fatal("execute called after runtime start failure, want false")
	}
	if terminal.EndedAt == nil {
		t.Fatal("EndedAt = nil, want terminal timestamp")
	}

	lastEvent := env.lastRunEvent(t, run.RunID)
	if lastEvent == nil || lastEvent.Kind != eventspkg.EventKindRunFailed {
		t.Fatalf("last event = %#v, want run.failed", lastEvent)
	}
}

func TestRunManagerAllowsConcurrentDistinctRunIDsAndStreamsLiveEvents(t *testing.T) {
	started := make(chan string, 3)
	release := make(chan struct{})
	env := newRunManagerTestEnv(t, runManagerTestDeps{
		prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
			return &model.SolvePreparation{}, nil
		},
		execute: func(ctx context.Context, prep *model.SolvePreparation, cfg *model.RuntimeConfig) error {
			submitEvent(ctx, t, prep.Journal(), cfg.RunID, eventspkg.EventKindJobQueued, kinds.JobQueuedPayload{
				Index:    1,
				SafeName: "job-001",
			})
			started <- cfg.RunID
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	})

	runA := env.startTaskRun(t, "task-run-a", nil)
	runB := env.startTaskRun(t, "task-run-b", nil)

	waitForString(t, started, runA.RunID)
	waitForString(t, started, runB.RunID)
	waitForRunCount(t, env.manager, env.workspaceRoot, runStatusRunning, 2)

	stream, err := env.manager.OpenStream(context.Background(), runA.RunID, apicore.StreamCursor{})
	if err != nil {
		t.Fatalf("OpenStream(%q) error = %v", runA.RunID, err)
	}
	defer func() {
		_ = stream.Close()
	}()

	first := waitForStreamItem(t, stream.Events())
	if first.Event == nil || first.Event.Kind != eventspkg.EventKindJobQueued {
		t.Fatalf("first stream item = %#v, want job.queued", first)
	}

	close(release)

	waitForRun(t, env.globalDB, runA.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusCompleted
	})
	waitForRun(t, env.globalDB, runB.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusCompleted
	})

	second := waitForStreamItem(t, stream.Events())
	if second.Event == nil || second.Event.Kind != eventspkg.EventKindRunCompleted {
		t.Fatalf("second stream item = %#v, want run.completed", second)
	}
}

func TestRunManagerRejectsConcurrentDuplicateRunID(t *testing.T) {
	release := make(chan struct{})
	env := newRunManagerTestEnv(t, runManagerTestDeps{
		prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
			return &model.SolvePreparation{}, nil
		},
		execute: func(ctx context.Context, _ *model.SolvePreparation, _ *model.RuntimeConfig) error {
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	})

	start := make(chan struct{})
	type result struct {
		run apicore.Run
		err error
	}
	results := make(chan result, 2)
	for range 2 {
		go func() {
			<-start
			run, err := env.manager.StartTaskRun(
				context.Background(),
				env.workspaceRoot,
				env.workflowSlug,
				apicore.TaskRunRequest{
					Workspace:        env.workspaceRoot,
					PresentationMode: defaultPresentationMode,
					RuntimeOverrides: rawJSON(t, `{"run_id":"duplicate-run"}`),
				},
			)
			results <- result{run: run, err: err}
		}()
	}
	close(start)

	var (
		success apicore.Run
		errs    []error
	)
	for i := 0; i < 2; i++ {
		result := <-results
		if result.err != nil {
			errs = append(errs, result.err)
			continue
		}
		success = result.run
	}

	if success.RunID == "" {
		t.Fatal("success.RunID = empty, want one successful start")
	}
	if len(errs) != 1 || !errors.Is(errs[0], globaldb.ErrRunAlreadyExists) {
		t.Fatalf("errors = %#v, want one ErrRunAlreadyExists", errs)
	}

	close(release)
	waitForRun(t, env.globalDB, success.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusCompleted
	})
}

func TestRunManagerRejectsInvalidRequests(t *testing.T) {
	env := newRunManagerTestEnv(t, runManagerTestDeps{})
	env.createReviewRound(t, 1)

	testCases := []struct {
		name string
		run  func() error
	}{
		{
			name: "task invalid presentation mode",
			run: func() error {
				_, err := env.manager.StartTaskRun(
					context.Background(),
					env.workspaceRoot,
					env.workflowSlug,
					apicore.TaskRunRequest{
						Workspace:        env.workspaceRoot,
						PresentationMode: "invalid",
					},
				)
				return err
			},
		},
		{
			name: "task invalid runtime overrides",
			run: func() error {
				_, err := env.manager.StartTaskRun(
					context.Background(),
					env.workspaceRoot,
					env.workflowSlug,
					apicore.TaskRunRequest{
						Workspace:        env.workspaceRoot,
						PresentationMode: defaultPresentationMode,
						RuntimeOverrides: rawJSON(t, `{"timeout":"not-a-duration"`),
					},
				)
				return err
			},
		},
		{
			name: "review invalid round",
			run: func() error {
				_, err := env.manager.StartReviewRun(
					context.Background(),
					env.workspaceRoot,
					env.workflowSlug,
					0,
					apicore.ReviewRunRequest{
						Workspace: env.workspaceRoot,
					},
				)
				return err
			},
		},
		{
			name: "review invalid batching",
			run: func() error {
				_, err := env.manager.StartReviewRun(
					context.Background(),
					env.workspaceRoot,
					env.workflowSlug,
					1,
					apicore.ReviewRunRequest{
						Workspace: env.workspaceRoot,
						Batching:  rawJSON(t, `{"concurrent":"bad"}`),
					},
				)
				return err
			},
		},
		{
			name: "exec missing workspace path",
			run: func() error {
				_, err := env.manager.StartExecRun(context.Background(), apicore.ExecRequest{
					Prompt: "hello",
				})
				return err
			},
		},
		{
			name: "exec missing prompt",
			run: func() error {
				_, err := env.manager.StartExecRun(context.Background(), apicore.ExecRequest{
					WorkspacePath: env.workspaceRoot,
				})
				return err
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.run()
			assertProblemStatus(t, err, 422)
		})
	}
}

func TestRunManagerStartExecRunCancelsAndGetReturnsUpdatedRow(t *testing.T) {
	started := make(chan string, 1)
	env := newRunManagerTestEnv(t, runManagerTestDeps{
		executeExec: func(ctx context.Context, cfg *model.RuntimeConfig, _ model.RunScope) error {
			started <- cfg.RunID
			<-ctx.Done()
			return ctx.Err()
		},
	})

	run := env.startExecRun(t, "exec-run-cancel", nil)
	waitForString(t, started, run.RunID)

	running := waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusRunning
	})
	if running.Mode != runModeExec {
		t.Fatalf("running mode = %q, want %q", running.Mode, runModeExec)
	}

	current, err := env.manager.Get(context.Background(), run.RunID)
	if err != nil {
		t.Fatalf("Get(%q) error = %v", run.RunID, err)
	}
	if current.Mode != runModeExec {
		t.Fatalf("current.Mode = %q, want %q", current.Mode, runModeExec)
	}
	if current.Status != runStatusRunning {
		t.Fatalf("current.Status = %q, want %q", current.Status, runStatusRunning)
	}

	if err := env.manager.Cancel(context.Background(), run.RunID); err != nil {
		t.Fatalf("Cancel(%q) error = %v", run.RunID, err)
	}
	waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusCancelled
	})

	current, err = env.manager.Get(context.Background(), run.RunID)
	if err != nil {
		t.Fatalf("Get(%q) after cancel error = %v", run.RunID, err)
	}
	if current.Status != runStatusCancelled {
		t.Fatalf("current.Status after cancel = %q, want %q", current.Status, runStatusCancelled)
	}
}

func TestRunManagerOpenRunScopeFailureCleansReservedDirectory(t *testing.T) {
	scopeErr := errors.New("scope unavailable")
	env := newRunManagerTestEnv(t, runManagerTestDeps{
		openRunScope: func(context.Context, *model.RuntimeConfig, model.OpenRunScopeOptions) (model.RunScope, error) {
			return nil, scopeErr
		},
	})

	_, err := env.manager.StartTaskRun(
		context.Background(),
		env.workspaceRoot,
		env.workflowSlug,
		apicore.TaskRunRequest{
			Workspace:        env.workspaceRoot,
			PresentationMode: defaultPresentationMode,
			RuntimeOverrides: rawJSON(t, `{"run_id":"scope-open-failure"}`),
		},
	)
	if !errors.Is(err, scopeErr) {
		t.Fatalf("StartTaskRun(open scope failure) error = %v, want %v", err, scopeErr)
	}

	runArtifacts, err := model.ResolveHomeRunArtifacts("scope-open-failure")
	if err != nil {
		t.Fatalf("ResolveHomeRunArtifacts() error = %v", err)
	}
	if _, err := os.Stat(runArtifacts.RunDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("run dir stat error = %v, want not exist", err)
	}
	if _, err := env.globalDB.GetRun(
		context.Background(),
		"scope-open-failure",
	); !errors.Is(
		err,
		globaldb.ErrRunNotFound,
	) {
		t.Fatalf("GetRun(open scope failure) error = %v, want ErrRunNotFound", err)
	}
}

func TestRunManagerStartRunSyncFailureMarksRunFailed(t *testing.T) {
	env := newRunManagerTestEnv(t, runManagerTestDeps{})

	workspace, err := env.globalDB.ResolveOrRegister(context.Background(), env.workspaceRoot)
	if err != nil {
		t.Fatalf("ResolveOrRegister() error = %v", err)
	}

	runID := "sync-failure-run"
	workflowRoot := filepath.Join(env.workspaceRoot, ".compozy", "tasks", "missing-workflow")
	_, err = env.manager.startRun(context.Background(), startRunSpec{
		workspace:        workspace,
		workflowSlug:     "missing-workflow",
		workflowRoot:     workflowRoot,
		mode:             runModeTask,
		presentationMode: defaultPresentationMode,
		runtimeCfg: &model.RuntimeConfig{
			RunID:         runID,
			WorkspaceRoot: env.workspaceRoot,
			Name:          "missing-workflow",
			TasksDir:      workflowRoot,
			Mode:          model.ExecutionModePRDTasks,
			DaemonOwned:   true,
		},
	})
	if err == nil {
		t.Fatal("startRun(sync failure) error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "sync workflow") {
		t.Fatalf("startRun(sync failure) error = %v, want sync workflow context", err)
	}

	row := waitForRun(t, env.globalDB, runID, func(row globaldb.Run) bool {
		return row.Status == runStatusFailed
	})
	if row.EndedAt == nil {
		t.Fatal("EndedAt = nil, want terminal timestamp")
	}
	if !strings.Contains(row.ErrorText, "sync workflow") {
		t.Fatalf("row.ErrorText = %q, want sync workflow context", row.ErrorText)
	}
	if active := env.manager.getActive(runID); active != nil {
		t.Fatalf("active run after sync failure = %#v, want nil", active)
	}
}

func TestRunManagerExecRunCompletesAndReplaysPersistedStream(t *testing.T) {
	executed := make(chan string, 1)
	env := newRunManagerTestEnv(t, runManagerTestDeps{
		executeExec: func(ctx context.Context, cfg *model.RuntimeConfig, scope model.RunScope) error {
			runArtifacts, err := model.ResolveHomeRunArtifacts(cfg.RunID)
			if err != nil {
				return err
			}
			submitEvent(
				ctx,
				t,
				scope.RunJournal(),
				cfg.RunID,
				eventspkg.EventKindRunCompleted,
				kinds.RunCompletedPayload{
					ArtifactsDir:   runArtifacts.RunDir,
					ResultPath:     runArtifacts.ResultPath,
					SummaryMessage: "exec completed",
				},
			)
			executed <- cfg.RunID
			return nil
		},
	})

	run := env.startExecRun(t, "exec-run-complete", nil)
	waitForString(t, executed, run.RunID)
	waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusCompleted
	})

	stream, err := env.manager.OpenStream(context.Background(), run.RunID, apicore.StreamCursor{})
	if err != nil {
		t.Fatalf("OpenStream(%q) error = %v", run.RunID, err)
	}
	defer func() {
		_ = stream.Close()
	}()

	if stream.Errors() == nil {
		t.Fatal("stream.Errors() = nil, want error channel")
	}
	item := waitForStreamItem(t, stream.Events())
	if item.Event == nil || item.Event.Kind != eventspkg.EventKindRunCompleted {
		t.Fatalf("stream item = %#v, want run.completed", item)
	}
	waitForClosedRunStream(t, stream)
}

func TestRunManagerExecRunFailureMarksRunFailed(t *testing.T) {
	execErr := errors.New("exec exploded")
	env := newRunManagerTestEnv(t, runManagerTestDeps{
		executeExec: func(context.Context, *model.RuntimeConfig, model.RunScope) error {
			return execErr
		},
	})

	run := env.startExecRun(t, "exec-run-failed", nil)
	terminal := waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusFailed
	})
	if terminal.EndedAt == nil {
		t.Fatal("EndedAt = nil, want terminal timestamp")
	}
	if !strings.Contains(terminal.ErrorText, execErr.Error()) {
		t.Fatalf("terminal.ErrorText = %q, want %q", terminal.ErrorText, execErr.Error())
	}

	lastEvent := env.lastRunEvent(t, run.RunID)
	if lastEvent == nil || lastEvent.Kind != eventspkg.EventKindRunFailed {
		t.Fatalf("last event = %#v, want run.failed", lastEvent)
	}
}

func TestRunManagerHelperOverridesAndUtilities(t *testing.T) {
	t.Run("apply overrides", func(t *testing.T) {
		cfg := &model.RuntimeConfig{}
		rules := []model.TaskRuntimeRule{
			{Type: stringPtr("backend"), IDE: stringPtr("codex")},
		}
		addDirs := []string{"./one", "./two"}

		if err := applyRuntimeOverridesFromProject(cfg, workspacecfg.RuntimeOverrides{
			IDE:                    stringPtr("claude"),
			Model:                  stringPtr("gpt-5"),
			OutputFormat:           stringPtr(string(model.OutputFormatJSON)),
			ReasoningEffort:        stringPtr("high"),
			AccessMode:             stringPtr(model.AccessModeDefault),
			Timeout:                stringPtr("2m"),
			TailLines:              intPtr(20),
			AddDirs:                &addDirs,
			AutoCommit:             boolPtr(true),
			MaxRetries:             intPtr(3),
			RetryBackoffMultiplier: floatPtr(2.0),
		}, "defaults"); err != nil {
			t.Fatalf("applyRuntimeOverridesFromProject() error = %v", err)
		}
		applyTaskProjectConfig(cfg, workspacecfg.StartConfig{
			IncludeCompleted: boolPtr(true),
			OutputFormat:     stringPtr(string(model.OutputFormatRawJSON)),
			TaskRuntimeRules: &rules,
		})
		applyReviewProjectConfig(cfg, workspacecfg.FixReviewsConfig{
			Concurrent:      intPtr(4),
			BatchSize:       intPtr(2),
			IncludeResolved: boolPtr(true),
			OutputFormat:    stringPtr(string(model.OutputFormatJSON)),
		})
		if err := applyExecProjectConfig(cfg, workspacecfg.ExecConfig{
			RuntimeOverrides: workspacecfg.RuntimeOverrides{
				Timeout: stringPtr("3m"),
			},
			Verbose: boolPtr(true),
			Persist: boolPtr(true),
		}); err != nil {
			t.Fatalf("applyExecProjectConfig() error = %v", err)
		}
		if err := applyRuntimeOverrideInput(cfg, runtimeOverrideInput{
			RunID:                      stringPtr("override-run"),
			IDE:                        stringPtr("codex"),
			Model:                      stringPtr("gpt-5-mini"),
			OutputFormat:               stringPtr(string(model.OutputFormatText)),
			ReasoningEffort:            stringPtr("medium"),
			AccessMode:                 stringPtr(model.AccessModeFull),
			Timeout:                    stringPtr("4m"),
			TailLines:                  intPtr(30),
			AddDirs:                    &[]string{"./three"},
			AutoCommit:                 boolPtr(false),
			MaxRetries:                 intPtr(5),
			RetryBackoffMultiplier:     floatPtr(3.0),
			Concurrent:                 intPtr(6),
			BatchSize:                  intPtr(7),
			Verbose:                    boolPtr(true),
			Persist:                    boolPtr(true),
			IncludeCompleted:           boolPtr(false),
			IncludeResolved:            boolPtr(false),
			EnableExecutableExtensions: boolPtr(true),
		}); err != nil {
			t.Fatalf("applyRuntimeOverrideInput() error = %v", err)
		}
		applyReviewBatching(cfg, reviewBatchingInput{
			Concurrent:      intPtr(8),
			BatchSize:       intPtr(9),
			IncludeResolved: boolPtr(true),
		})
		applySoundConfig(cfg, workspacecfg.SoundConfig{
			Enabled:     boolPtr(true),
			OnCompleted: stringPtr("glass"),
			OnFailed:    stringPtr("basso"),
		})

		if cfg.RunID != "override-run" || cfg.IDE != "codex" || cfg.Model != "gpt-5-mini" {
			t.Fatalf("runtime override application failed: %#v", cfg)
		}
		if cfg.Concurrent != 8 || cfg.BatchSize != 9 || !cfg.IncludeResolved {
			t.Fatalf("review batching application failed: %#v", cfg)
		}
		if cfg.OutputFormat != model.OutputFormatText || cfg.Timeout != 4*time.Minute {
			t.Fatalf("runtime output/timeout = %q / %v, want text / 4m", cfg.OutputFormat, cfg.Timeout)
		}
		if !cfg.SoundEnabled || cfg.SoundOnCompleted != "glass" || cfg.SoundOnFailed != "basso" {
			t.Fatalf("sound config application failed: %#v", cfg)
		}
		if len(cfg.TaskRuntimeRules) != 1 || cfg.TaskRuntimeRules[0].Type == nil ||
			*cfg.TaskRuntimeRules[0].Type != "backend" {
			t.Fatalf("task runtime rules = %#v, want cloned backend rule", cfg.TaskRuntimeRules)
		}
	})

	t.Run("duration and error helpers", func(t *testing.T) {
		cfg := &model.RuntimeConfig{}
		if err := applyOptionalDuration(cfg, stringPtr("90s")); err != nil {
			t.Fatalf("applyOptionalDuration(valid) error = %v", err)
		}
		if cfg.Timeout != 90*time.Second {
			t.Fatalf("cfg.Timeout = %v, want 90s", cfg.Timeout)
		}
		if err := applyOptionalDuration(cfg, stringPtr("")); err != nil {
			t.Fatalf("applyOptionalDuration(empty) error = %v", err)
		}
		if cfg.Timeout != 0 {
			t.Fatalf("cfg.Timeout = %v, want 0 after empty override", cfg.Timeout)
		}
		if err := applyOptionalDuration(cfg, stringPtr("bad-duration")); err == nil {
			t.Fatal("applyOptionalDuration(invalid) error = nil, want non-nil")
		}

		err := overrideValueError("runtime_overrides", "timeout", errors.New("bad duration"))
		assertProblemStatus(t, err, 422)
	})

	t.Run("context and raw helpers", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		detached := detachContext(ctx)
		if detached.Err() != nil {
			t.Fatalf("detachContext(ctx).Err() = %v, want nil", detached.Err())
		}

		ctxWithID := withRequestID(context.Background(), "req-123")
		if got := apicore.RequestIDFromContext(ctxWithID); got != "req-123" {
			t.Fatalf("RequestIDFromContext() = %q, want req-123", got)
		}

		if got := string(rawMessageOrNil(` {"hello":"world"} `)); got == "" {
			t.Fatal("rawMessageOrNil(non-empty) = empty, want raw payload")
		}
		if got := rawMessageOrNil("   "); got != nil {
			t.Fatalf("rawMessageOrNil(empty) = %#v, want nil", got)
		}
		if got := errorString(errors.New(" boom ")); got != "boom" {
			t.Fatalf("errorString(trimmed) = %q, want boom", got)
		}
		if got := errorString(nil); got != "" {
			t.Fatalf("errorString(nil) = %q, want empty", got)
		}
	})

	t.Run("filesystem helpers", func(t *testing.T) {
		runDir := filepath.Join(t.TempDir(), "runs", "helper-run")
		if err := reserveRunDirectory(runDir); err != nil {
			t.Fatalf("reserveRunDirectory() error = %v", err)
		}
		if err := reserveRunDirectory(runDir); !errors.Is(err, globaldb.ErrRunAlreadyExists) {
			t.Fatalf("reserveRunDirectory(duplicate) error = %v, want ErrRunAlreadyExists", err)
		}
		if err := requireDirectory(runDir); err != nil {
			t.Fatalf("requireDirectory(dir) error = %v", err)
		}

		filePath := filepath.Join(t.TempDir(), "file.txt")
		if err := os.WriteFile(filePath, []byte("content"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		if err := requireDirectory(filePath); err == nil {
			t.Fatal("requireDirectory(file) error = nil, want non-nil")
		}

		cleanupRunDirectory(runDir)
		if _, err := os.Stat(runDir); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("cleanupRunDirectory() stat error = %v, want not exist", err)
		}
	})
}

func TestNewRunManagerRequiresGlobalDBAndAppliesDefaults(t *testing.T) {
	if _, err := NewRunManager(RunManagerConfig{}); err == nil {
		t.Fatal("NewRunManager(nil GlobalDB) error = nil, want non-nil")
	}

	env := newRunManagerTestEnv(t, runManagerTestDeps{})
	manager, err := NewRunManager(RunManagerConfig{GlobalDB: env.globalDB})
	if err != nil {
		t.Fatalf("NewRunManager(defaults) error = %v", err)
	}
	if manager.globalDB != env.globalDB {
		t.Fatal("manager.globalDB mismatch")
	}
	if manager.lifecycleCtx == nil || manager.now == nil || manager.openRunScope == nil {
		t.Fatal("manager defaults not initialized")
	}
	if manager.prepare == nil || manager.execute == nil || manager.executeExec == nil ||
		manager.loadProjectConfig == nil {
		t.Fatal("manager dependency defaults not initialized")
	}
}

func TestRunManagerEnsureWorkflowIdentityValidatesAndReusesRows(t *testing.T) {
	env := newRunManagerTestEnv(t, runManagerTestDeps{})
	workspace, err := env.globalDB.ResolveOrRegister(context.Background(), env.workspaceRoot)
	if err != nil {
		t.Fatalf("ResolveOrRegister() error = %v", err)
	}

	if _, err := env.manager.ensureWorkflowIdentity(context.Background(), workspace.ID, ""); err == nil {
		t.Fatal("ensureWorkflowIdentity(empty slug) error = nil, want non-nil")
	} else {
		assertProblemStatus(t, err, 422)
	}

	firstID, err := env.manager.ensureWorkflowIdentity(context.Background(), workspace.ID, env.workflowSlug)
	if err != nil {
		t.Fatalf("ensureWorkflowIdentity(first) error = %v", err)
	}
	secondID, err := env.manager.ensureWorkflowIdentity(context.Background(), workspace.ID, env.workflowSlug)
	if err != nil {
		t.Fatalf("ensureWorkflowIdentity(second) error = %v", err)
	}
	if firstID == nil || secondID == nil || *firstID != *secondID {
		t.Fatalf("workflow IDs differ: first=%v second=%v", firstID, secondID)
	}

	projectCfg, err := env.manager.loadProjectConfig(context.Background(), env.workspaceRoot)
	if err != nil {
		t.Fatalf("loadProjectConfig() error = %v", err)
	}
	workspaceRow, workflowID, _, err := env.manager.resolveWorkflowContext(
		context.Background(),
		env.workspaceRoot,
		env.workflowSlug,
	)
	if err != nil {
		t.Fatalf("resolveWorkflowContext() error = %v", err)
	}
	if workspaceRow.ID != workspace.ID {
		t.Fatalf("workspaceRow.ID = %q, want %q", workspaceRow.ID, workspace.ID)
	}
	if workflowID == nil || *workflowID != *firstID {
		t.Fatalf("workflowID = %v, want %v", workflowID, firstID)
	}
	if projectCfg != (workspacecfg.ProjectConfig{}) {
		t.Fatalf("projectCfg = %#v, want zero-value defaults", projectCfg)
	}
}

func TestHostSetHTTPPortPersistsInfo(t *testing.T) {
	paths := mustHomePaths(t)
	host := &Host{
		paths: paths,
		info: Info{
			PID:        4242,
			Version:    "test",
			SocketPath: paths.SocketPath,
			StartedAt:  time.Now().UTC(),
			State:      ReadyStateReady,
		},
	}

	if err := host.SetHTTPPort(context.Background(), 43123); err != nil {
		t.Fatalf("SetHTTPPort() error = %v", err)
	}
	current, err := ReadInfo(paths.InfoPath)
	if err != nil {
		t.Fatalf("ReadInfo() error = %v", err)
	}
	if current.HTTPPort != 43123 {
		t.Fatalf("current.HTTPPort = %d, want 43123", current.HTTPPort)
	}
	if host.Info().HTTPPort != 43123 {
		t.Fatalf("host.Info().HTTPPort = %d, want 43123", host.Info().HTTPPort)
	}
}

func TestHostCloseRemovesOwnedInfoAndReleasesLock(t *testing.T) {
	paths := mustHomePaths(t)
	lock, err := acquireLock(paths.LockPath, 5150, lockDeps{
		processAlive: func(pid int) bool { return pid == 5150 },
	})
	if err != nil {
		t.Fatalf("acquireLock() error = %v", err)
	}

	info := Info{
		PID:        5150,
		Version:    "test",
		SocketPath: paths.SocketPath,
		StartedAt:  time.Now().UTC(),
		State:      ReadyStateReady,
	}
	if err := WriteInfo(paths.InfoPath, info); err != nil {
		t.Fatalf("WriteInfo() error = %v", err)
	}

	host := &Host{
		paths: paths,
		lock:  lock,
		info:  info,
	}
	if err := host.Close(context.Background()); err != nil {
		t.Fatalf("Host.Close() error = %v", err)
	}

	if _, err := os.Stat(paths.InfoPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("info path stat error = %v, want not exist", err)
	}
	lockPID, err := readLockPID(paths.LockPath)
	if err != nil {
		t.Fatalf("readLockPID() error = %v", err)
	}
	if lockPID != 0 {
		t.Fatalf("lock PID = %d, want 0", lockPID)
	}
}

func TestResolveTerminalStateReturnsPersistedTerminalEvent(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	cfg := (&model.RuntimeConfig{
		RunID: "terminal-run-failed",
		Mode:  model.ExecutionModeExec,
	}).Clone()
	cfg.ApplyDefaults()

	scope, err := newTestOpenRunScope(nil)(context.Background(), cfg, model.OpenRunScopeOptions{})
	if err != nil {
		t.Fatalf("open test scope: %v", err)
	}
	defer func() {
		_ = scope.Close(context.Background())
	}()

	submitEvent(
		context.Background(),
		t,
		scope.RunJournal(),
		cfg.RunID,
		eventspkg.EventKindRunFailed,
		kinds.RunFailedPayload{
			Error: "boom",
		},
	)

	terminal, err := resolveTerminalState(context.Background(), cfg.RunID, terminalState{}, scope)
	if err != nil {
		t.Fatalf("resolveTerminalState() error = %v", err)
	}
	if terminal.status != runStatusFailed {
		t.Fatalf("terminal.status = %q, want %q", terminal.status, runStatusFailed)
	}
	if terminal.errorText != "boom" {
		t.Fatalf("terminal.errorText = %q, want boom", terminal.errorText)
	}
}

func TestResolveTerminalStateErrorsWithoutTerminalSignal(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	cfg := (&model.RuntimeConfig{
		RunID: "terminal-run-missing",
		Mode:  model.ExecutionModeExec,
	}).Clone()
	cfg.ApplyDefaults()

	scope, err := newTestOpenRunScope(nil)(context.Background(), cfg, model.OpenRunScopeOptions{})
	if err != nil {
		t.Fatalf("open test scope: %v", err)
	}
	defer func() {
		_ = scope.Close(context.Background())
	}()

	if _, err := resolveTerminalState(context.Background(), cfg.RunID, terminalState{}, scope); err == nil {
		t.Fatal("resolveTerminalState(no signal) error = nil, want non-nil")
	}

	fallback := completedTerminalState(scope.RunArtifacts(), "done")
	if _, err := resolveTerminalState(context.Background(), cfg.RunID, fallback, nil); err == nil {
		t.Fatal("resolveTerminalState(nil scope) error = nil, want non-nil")
	}
}

func TestRunManagerHelperEdgeCases(t *testing.T) {
	var nilCtx context.Context
	if got := detachContext(nilCtx); got == nil {
		t.Fatal("detachContext(nil) = nil, want background context")
	}
	if got := withRequestID(context.Background(), ""); got == nil {
		t.Fatal("withRequestID(background, empty) = nil, want background context")
	}

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if ok := sendRunStreamItem(cancelledCtx, make(chan apicore.RunStreamItem), apicore.RunStreamItem{}); ok {
		t.Fatal("sendRunStreamItem(canceled) = true, want false")
	}

	var stream *runStream
	if stream.Events() != nil {
		t.Fatal("nil runStream Events() = non-nil, want nil")
	}
	if stream.Errors() != nil {
		t.Fatal("nil runStream Errors() = non-nil, want nil")
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("nil runStream Close() error = %v, want nil", err)
	}
}

func TestRunManagerTaskRunWatcherSyncsTaskEditsAndStopsOnCancel(t *testing.T) {
	env := newRunManagerTestEnv(t, runManagerTestDeps{
		watcherDebounce: 40 * time.Millisecond,
		prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
			return &model.SolvePreparation{}, nil
		},
		execute: func(ctx context.Context, _ *model.SolvePreparation, _ *model.RuntimeConfig) error {
			<-ctx.Done()
			return ctx.Err()
		},
	})

	env.writeWorkflowFile(t, env.workflowSlug, "task_01.md", daemonTaskBody("pending", "Watcher task"))
	env.writeWorkflowFile(t, env.workflowSlug, "_meta.md", daemonLegacyWorkflowMetaBody())
	env.writeWorkflowFile(t, env.workflowSlug, "_tasks.md", "Legacy generated summary\n")

	run := env.startTaskRun(t, "task-run-watch", nil)
	waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusRunning
	})

	if _, err := os.Stat(
		filepath.Join(env.workflowDir(env.workflowSlug), "_meta.md"),
	); !errors.Is(
		err,
		os.ErrNotExist,
	) {
		t.Fatalf("expected pre-run sync to remove workflow _meta.md, got %v", err)
	}
	if _, err := os.Stat(
		filepath.Join(env.workflowDir(env.workflowSlug), "_tasks.md"),
	); !errors.Is(
		err,
		os.ErrNotExist,
	) {
		t.Fatalf("expected pre-run sync to remove generated _tasks.md, got %v", err)
	}

	checkpointBefore := queryWorkflowCheckpointChecksum(t, env.paths.GlobalDBPath, env.workflowSlug)
	env.writeWorkflowFile(t, env.workflowSlug, "task_01.md", daemonTaskBody("completed", "Watcher task updated"))

	waitForCondition(t, 5*time.Second, "task watcher sync", func() bool {
		title, status, ok := queryTaskItem(t, env.paths.GlobalDBPath, env.workflowSlug, 1)
		return ok &&
			title == "Watcher task updated" &&
			status == "completed" &&
			queryWorkflowCheckpointChecksum(t, env.paths.GlobalDBPath, env.workflowSlug) != checkpointBefore &&
			runArtifactSyncCount(t, run.RunID) >= 1
	})

	if err := env.manager.Cancel(context.Background(), run.RunID); err != nil {
		t.Fatalf("Cancel(%q) error = %v", run.RunID, err)
	}
	waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusCancelled
	})

	rowsBeforeStop := runArtifactSyncCount(t, run.RunID)
	checkpointAfterStop := queryWorkflowCheckpointChecksum(t, env.paths.GlobalDBPath, env.workflowSlug)
	env.writeWorkflowFile(t, env.workflowSlug, "task_01.md", daemonTaskBody("pending", "post-cancel change"))
	time.Sleep(200 * time.Millisecond)

	if got := runArtifactSyncCount(t, run.RunID); got != rowsBeforeStop {
		t.Fatalf("artifact sync rows after cancel = %d, want %d", got, rowsBeforeStop)
	}
	if got := queryWorkflowCheckpointChecksum(t, env.paths.GlobalDBPath, env.workflowSlug); got != checkpointAfterStop {
		t.Fatalf("checkpoint after cancel change = %q, want %q", got, checkpointAfterStop)
	}
	if title, status, ok := queryTaskItem(t, env.paths.GlobalDBPath, env.workflowSlug, 1); !ok ||
		title != "Watcher task updated" || status != "completed" {
		t.Fatalf("task row after cancel change = title:%q status:%q ok:%v", title, status, ok)
	}
	if _, err := os.Stat(
		filepath.Join(env.workflowDir(env.workflowSlug), "_meta.md"),
	); !errors.Is(
		err,
		os.ErrNotExist,
	) {
		t.Fatalf("expected workflow _meta.md to stay absent after run shutdown, got %v", err)
	}
}

func TestRunManagerReviewRunWatcherSyncsOwnedWorkflowArtifacts(t *testing.T) {
	env := newRunManagerTestEnv(t, runManagerTestDeps{
		watcherDebounce: 40 * time.Millisecond,
		prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
			return &model.SolvePreparation{}, nil
		},
		execute: func(ctx context.Context, _ *model.SolvePreparation, _ *model.RuntimeConfig) error {
			<-ctx.Done()
			return ctx.Err()
		},
	})

	writeOwnedWorkflowArtifacts(t, env, env.workflowSlug)
	otherSlug := "other-workflow"
	writeOwnedWorkflowArtifacts(t, env, otherSlug)
	if _, err := corepkg.SyncDirect(context.Background(), corepkg.SyncConfig{
		TasksDir: env.workflowDir(otherSlug),
	}); err != nil {
		t.Fatalf("SyncDirect(other workflow) error = %v", err)
	}

	run := env.startReviewRunForWorkflow(t, env.workflowSlug, "review-run-watch", 1, nil, nil)
	waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusRunning
	})

	otherCheckpoint := queryWorkflowCheckpointChecksum(t, env.paths.GlobalDBPath, otherSlug)
	otherMemoryBody, _ := queryArtifactSnapshotBody(
		t,
		env.paths.GlobalDBPath,
		otherSlug,
		"memory/MEMORY.md",
	)

	env.writeWorkflowFile(
		t,
		env.workflowSlug,
		filepath.Join("reviews-001", "issue_001.md"),
		daemonReviewIssueBody("resolved", "high"),
	)
	env.writeWorkflowFile(t, env.workflowSlug, filepath.Join("memory", "MEMORY.md"), "# Workflow Memory\nupdated\n")
	env.writeWorkflowFile(t, env.workflowSlug, filepath.Join("prompts", "task-run.md"), "# Prompt\nupdated\n")
	env.writeWorkflowFile(t, env.workflowSlug, filepath.Join("protocol", "handoff.md"), "# Protocol\nupdated\n")
	env.writeWorkflowFile(t, env.workflowSlug, filepath.Join("adrs", "adr-001.md"), "# ADR 001\nupdated\n")
	env.writeWorkflowFile(t, env.workflowSlug, filepath.Join("qa", "verification-report.md"), "# QA\nupdated\n")

	waitForCondition(t, 5*time.Second, "review watcher sync", func() bool {
		reviewStatus, ok := queryReviewIssueStatus(t, env.paths.GlobalDBPath, env.workflowSlug, 1, 1)
		memoryBody, _ := queryArtifactSnapshotBody(t, env.paths.GlobalDBPath, env.workflowSlug, "memory/MEMORY.md")
		promptBody, _ := queryArtifactSnapshotBody(t, env.paths.GlobalDBPath, env.workflowSlug, "prompts/task-run.md")
		protocolBody, _ := queryArtifactSnapshotBody(t, env.paths.GlobalDBPath, env.workflowSlug, "protocol/handoff.md")
		adrBody, _ := queryArtifactSnapshotBody(t, env.paths.GlobalDBPath, env.workflowSlug, "adrs/adr-001.md")
		qaBody, _ := queryArtifactSnapshotBody(t, env.paths.GlobalDBPath, env.workflowSlug, "qa/verification-report.md")
		return ok &&
			reviewStatus == "resolved" &&
			strings.Contains(memoryBody, "updated") &&
			strings.Contains(promptBody, "updated") &&
			strings.Contains(protocolBody, "updated") &&
			strings.Contains(adrBody, "updated") &&
			strings.Contains(qaBody, "updated") &&
			runArtifactSyncCount(t, run.RunID) >= 6
	})

	if got := queryWorkflowCheckpointChecksum(t, env.paths.GlobalDBPath, otherSlug); got != otherCheckpoint {
		t.Fatalf("other workflow checkpoint = %q, want %q", got, otherCheckpoint)
	}
	if got, _ := queryArtifactSnapshotBody(
		t,
		env.paths.GlobalDBPath,
		otherSlug,
		"memory/MEMORY.md",
	); got != otherMemoryBody {
		t.Fatalf("other workflow memory body changed unexpectedly\nwant:\n%s\ngot:\n%s", otherMemoryBody, got)
	}

	if err := env.manager.Cancel(context.Background(), run.RunID); err != nil {
		t.Fatalf("Cancel(%q) error = %v", run.RunID, err)
	}
	waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusCancelled
	})
}

func TestRunManagerTaskRunWatchersStayIsolatedAcrossWorkflows(t *testing.T) {
	env := newRunManagerTestEnv(t, runManagerTestDeps{
		watcherDebounce: 40 * time.Millisecond,
		prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
			return &model.SolvePreparation{}, nil
		},
		execute: func(ctx context.Context, _ *model.SolvePreparation, _ *model.RuntimeConfig) error {
			<-ctx.Done()
			return ctx.Err()
		},
	})

	env.writeWorkflowFile(t, env.workflowSlug, "task_01.md", daemonTaskBody("pending", "Primary workflow"))
	otherSlug := "other-workflow"
	env.writeWorkflowFile(t, otherSlug, "task_01.md", daemonTaskBody("pending", "Secondary workflow"))

	runA := env.startTaskRunForWorkflow(t, env.workflowSlug, "task-run-a-watch", nil)
	runB := env.startTaskRunForWorkflow(t, otherSlug, "task-run-b-watch", nil)
	waitForRun(t, env.globalDB, runA.RunID, func(row globaldb.Run) bool { return row.Status == runStatusRunning })
	waitForRun(t, env.globalDB, runB.RunID, func(row globaldb.Run) bool { return row.Status == runStatusRunning })

	secondaryCheckpoint := queryWorkflowCheckpointChecksum(t, env.paths.GlobalDBPath, otherSlug)
	env.writeWorkflowFile(t, env.workflowSlug, "task_01.md", daemonTaskBody("completed", "Primary workflow updated"))

	waitForCondition(t, 5*time.Second, "primary workflow watcher sync", func() bool {
		title, status, ok := queryTaskItem(t, env.paths.GlobalDBPath, env.workflowSlug, 1)
		return ok && title == "Primary workflow updated" && status == "completed" &&
			runArtifactSyncCount(t, runA.RunID) >= 1
	})

	time.Sleep(200 * time.Millisecond)
	if got := runArtifactSyncCount(t, runB.RunID); got != 0 {
		t.Fatalf("secondary run artifact sync rows = %d, want 0", got)
	}
	if got := queryWorkflowCheckpointChecksum(t, env.paths.GlobalDBPath, otherSlug); got != secondaryCheckpoint {
		t.Fatalf("secondary workflow checkpoint = %q, want %q", got, secondaryCheckpoint)
	}
	if title, status, ok := queryTaskItem(t, env.paths.GlobalDBPath, otherSlug, 1); !ok ||
		title != "Secondary workflow" || status != "pending" {
		t.Fatalf("secondary workflow row = title:%q status:%q ok:%v", title, status, ok)
	}

	if err := env.manager.Cancel(context.Background(), runA.RunID); err != nil {
		t.Fatalf("Cancel(runA) error = %v", err)
	}
	if err := env.manager.Cancel(context.Background(), runB.RunID); err != nil {
		t.Fatalf("Cancel(runB) error = %v", err)
	}
	waitForRun(t, env.globalDB, runA.RunID, func(row globaldb.Run) bool { return row.Status == runStatusCancelled })
	waitForRun(t, env.globalDB, runB.RunID, func(row globaldb.Run) bool { return row.Status == runStatusCancelled })
}

type runManagerTestEnv struct {
	homeDir       string
	paths         compozyconfig.HomePaths
	workspaceRoot string
	workflowSlug  string
	globalDB      *globaldb.GlobalDB
	manager       *RunManager
}

type runManagerTestDeps struct {
	now                  func() time.Time
	openRunScope         func(context.Context, *model.RuntimeConfig, model.OpenRunScopeOptions) (model.RunScope, error)
	prepare              func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error)
	execute              func(context.Context, *model.SolvePreparation, *model.RuntimeConfig) error
	executeExec          func(context.Context, *model.RuntimeConfig, model.RunScope) error
	shutdownDrainTimeout time.Duration
	watcherDebounce      time.Duration
}

func newRunManagerTestEnv(t *testing.T, deps runManagerTestDeps) *runManagerTestEnv {
	t.Helper()

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	paths, err := compozyconfig.ResolveHomePathsFrom(filepath.Join(homeDir, ".compozy"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := compozyconfig.EnsureHomeLayout(paths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	globalDB, err := globaldb.Open(context.Background(), paths.GlobalDBPath)
	if err != nil {
		t.Fatalf("globaldb.Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = globalDB.Close()
	})

	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(filepath.Join(workspaceRoot, ".compozy"), 0o755); err != nil {
		t.Fatalf("mkdir workspace marker: %v", err)
	}

	workflowSlug := "daemon-workflow"
	if err := os.MkdirAll(model.TaskDirectoryForWorkspace(workspaceRoot, workflowSlug), 0o755); err != nil {
		t.Fatalf("mkdir task workflow dir: %v", err)
	}

	manager, err := NewRunManager(RunManagerConfig{
		GlobalDB:             globalDB,
		LifecycleContext:     context.Background(),
		ShutdownDrainTimeout: deps.shutdownDrainTimeout,
		Now:                  deps.now,
		OpenRunScope:         firstOpenRunScope(deps.openRunScope),
		Prepare:              firstPrepare(deps.prepare),
		Execute:              firstExecute(deps.execute),
		ExecuteExec:          firstExecuteExec(deps.executeExec),
		WatcherDebounce:      deps.watcherDebounce,
		LoadProjectConfig: func(context.Context, string) (workspacecfg.ProjectConfig, error) {
			return workspacecfg.ProjectConfig{}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewRunManager() error = %v", err)
	}

	return &runManagerTestEnv{
		homeDir:       homeDir,
		paths:         paths,
		workspaceRoot: workspaceRoot,
		workflowSlug:  workflowSlug,
		globalDB:      globalDB,
		manager:       manager,
	}
}

func (e *runManagerTestEnv) workflowDir(slug string) string {
	return model.TaskDirectoryForWorkspace(e.workspaceRoot, slug)
}

func (e *runManagerTestEnv) writeWorkflowFile(t *testing.T, slug, relativePath, content string) string {
	t.Helper()

	workflowDir := e.workflowDir(slug)
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}

	path := filepath.Join(workflowDir, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir workflow file parent: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write workflow file %s: %v", path, err)
	}
	return path
}

func (e *runManagerTestEnv) createReviewRound(t *testing.T, round int) string {
	t.Helper()

	reviewDir := filepath.Join(e.workflowDir(e.workflowSlug), fmt.Sprintf("reviews-%03d", round))
	if err := os.MkdirAll(reviewDir, 0o755); err != nil {
		t.Fatalf("mkdir review round dir: %v", err)
	}
	e.writeWorkflowFile(
		t,
		e.workflowSlug,
		filepath.Join(fmt.Sprintf("reviews-%03d", round), "_meta.md"),
		daemonReviewRoundMetaBody("coderabbit", "123", round),
	)
	e.writeWorkflowFile(
		t,
		e.workflowSlug,
		filepath.Join(fmt.Sprintf("reviews-%03d", round), "issue_001.md"),
		daemonReviewIssueBody("pending", "medium"),
	)
	return reviewDir
}

func (e *runManagerTestEnv) startTaskRunForWorkflow(
	t *testing.T,
	workflowSlug string,
	runID string,
	runtimeOverrides json.RawMessage,
) apicore.Run {
	t.Helper()

	if len(runtimeOverrides) == 0 {
		runtimeOverrides = rawJSON(t, fmt.Sprintf(`{"run_id":%q}`, runID))
	}

	run, err := e.manager.StartTaskRun(
		context.Background(),
		e.workspaceRoot,
		workflowSlug,
		apicore.TaskRunRequest{
			Workspace:        e.workspaceRoot,
			PresentationMode: defaultPresentationMode,
			RuntimeOverrides: runtimeOverrides,
		},
	)
	if err != nil {
		t.Fatalf("StartTaskRun(%q, %q) error = %v", workflowSlug, runID, err)
	}
	return run
}

func (e *runManagerTestEnv) startTaskRun(t *testing.T, runID string, runtimeOverrides json.RawMessage) apicore.Run {
	t.Helper()
	return e.startTaskRunForWorkflow(t, e.workflowSlug, runID, runtimeOverrides)
}

func (e *runManagerTestEnv) startReviewRun(
	t *testing.T,
	runID string,
	round int,
	runtimeOverrides json.RawMessage,
	batching json.RawMessage,
) apicore.Run {
	t.Helper()

	return e.startReviewRunForWorkflow(t, e.workflowSlug, runID, round, runtimeOverrides, batching)
}

func (e *runManagerTestEnv) startReviewRunForWorkflow(
	t *testing.T,
	workflowSlug string,
	runID string,
	round int,
	runtimeOverrides json.RawMessage,
	batching json.RawMessage,
) apicore.Run {
	t.Helper()

	if len(runtimeOverrides) == 0 {
		runtimeOverrides = rawJSON(t, fmt.Sprintf(`{"run_id":%q}`, runID))
	}

	run, err := e.manager.StartReviewRun(
		context.Background(),
		e.workspaceRoot,
		workflowSlug,
		round,
		apicore.ReviewRunRequest{
			Workspace:        e.workspaceRoot,
			PresentationMode: defaultPresentationMode,
			RuntimeOverrides: runtimeOverrides,
			Batching:         batching,
		},
	)
	if err != nil {
		t.Fatalf("StartReviewRun(%q, %q) error = %v", workflowSlug, runID, err)
	}
	return run
}

func (e *runManagerTestEnv) startExecRun(t *testing.T, runID string, runtimeOverrides json.RawMessage) apicore.Run {
	t.Helper()

	if len(runtimeOverrides) == 0 {
		runtimeOverrides = rawJSON(t, fmt.Sprintf(`{"run_id":%q}`, runID))
	}

	run, err := e.manager.StartExecRun(context.Background(), apicore.ExecRequest{
		WorkspacePath:    e.workspaceRoot,
		Prompt:           "daemon exec prompt",
		PresentationMode: defaultPresentationMode,
		RuntimeOverrides: runtimeOverrides,
	})
	if err != nil {
		t.Fatalf("StartExecRun(%q) error = %v", runID, err)
	}
	return run
}

func (e *runManagerTestEnv) lastRunEvent(t *testing.T, runID string) *eventspkg.Event {
	t.Helper()

	runDB, err := openRunDB(context.Background(), runID)
	if err != nil {
		t.Fatalf("openRunDB(%q) error = %v", runID, err)
	}
	defer func() {
		_ = runDB.Close()
	}()

	lastEvent, err := runDB.LastEvent(context.Background())
	if err != nil {
		t.Fatalf("LastEvent(%q) error = %v", runID, err)
	}
	return lastEvent
}

type stubRuntimeManager struct {
	startErr error
}

func (m *stubRuntimeManager) Start(context.Context) error {
	return m.startErr
}

func (*stubRuntimeManager) DispatchMutableHook(_ context.Context, _ string, payload any) (any, error) {
	return payload, nil
}

func (*stubRuntimeManager) DispatchObserverHook(context.Context, string, any) {}

func (*stubRuntimeManager) Shutdown(context.Context) error {
	return nil
}

type wrappedRunScope struct {
	model.RunScope
	runtime model.RuntimeManager
}

func (s *wrappedRunScope) RunManager() model.RuntimeManager {
	if s == nil {
		return nil
	}
	return s.runtime
}

func (s *wrappedRunScope) Close(ctx context.Context) error {
	if s == nil {
		return nil
	}

	var closeErr error
	if s.RunScope != nil {
		closeErr = errors.Join(closeErr, s.RunScope.Close(ctx))
	}
	if s.runtime != nil {
		closeErr = errors.Join(closeErr, s.runtime.Shutdown(ctx))
	}
	return closeErr
}

func newTestOpenRunScope(
	runtimeManager model.RuntimeManager,
) func(context.Context, *model.RuntimeConfig, model.OpenRunScopeOptions) (model.RunScope, error) {
	return func(ctx context.Context, cfg *model.RuntimeConfig, _ model.OpenRunScopeOptions) (model.RunScope, error) {
		base, err := model.OpenBaseRunScope(ctx, cfg)
		if err != nil {
			return nil, err
		}
		if runtimeManager == nil {
			return base, nil
		}
		return &wrappedRunScope{
			RunScope: base,
			runtime:  runtimeManager,
		}, nil
	}
}

func firstOpenRunScope(
	fn func(context.Context, *model.RuntimeConfig, model.OpenRunScopeOptions) (model.RunScope, error),
) func(context.Context, *model.RuntimeConfig, model.OpenRunScopeOptions) (model.RunScope, error) {
	if fn != nil {
		return fn
	}
	return newTestOpenRunScope(nil)
}

func firstPrepare(
	fn func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error),
) func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
	if fn != nil {
		return fn
	}
	return func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
		return nil, plan.ErrNoWork
	}
}

func firstExecute(
	fn func(context.Context, *model.SolvePreparation, *model.RuntimeConfig) error,
) func(context.Context, *model.SolvePreparation, *model.RuntimeConfig) error {
	if fn != nil {
		return fn
	}
	return func(context.Context, *model.SolvePreparation, *model.RuntimeConfig) error {
		return nil
	}
}

func firstExecuteExec(
	fn func(context.Context, *model.RuntimeConfig, model.RunScope) error,
) func(context.Context, *model.RuntimeConfig, model.RunScope) error {
	if fn != nil {
		return fn
	}
	return func(context.Context, *model.RuntimeConfig, model.RunScope) error {
		return nil
	}
}

func rawJSON(t *testing.T, value string) json.RawMessage {
	t.Helper()
	return json.RawMessage(value)
}

func submitEvent(
	ctx context.Context,
	t *testing.T,
	journal interface {
		Submit(context.Context, eventspkg.Event) error
	},
	runID string,
	kind eventspkg.EventKind,
	payload any,
) {
	t.Helper()

	rawPayload, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal(%T) error = %v", payload, err)
	}
	event := eventspkg.Event{
		RunID:   runID,
		Kind:    kind,
		Payload: rawPayload,
	}
	if submitter, ok := any(journal).(interface {
		SubmitWithSeq(context.Context, eventspkg.Event) (uint64, error)
	}); ok {
		if _, err := submitter.SubmitWithSeq(ctx, event); err != nil {
			t.Fatalf("SubmitWithSeq(%s) error = %v", kind, err)
		}
		return
	}
	if err := journal.Submit(ctx, event); err != nil {
		t.Fatalf("Submit(%s) error = %v", kind, err)
	}
}

func waitForRun(
	t *testing.T,
	db *globaldb.GlobalDB,
	runID string,
	predicate func(globaldb.Run) bool,
) globaldb.Run {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		row, err := db.GetRun(context.Background(), runID)
		if err == nil && predicate(row) {
			return row
		}
		select {
		case <-ctx.Done():
			t.Fatalf("timed out waiting for run %q: last err=%v", runID, err)
		case <-ticker.C:
		}
	}
}

func waitForRunCount(
	t *testing.T,
	manager *RunManager,
	workspace string,
	status string,
	want int,
) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		runs, err := manager.List(context.Background(), apicore.RunListQuery{
			Workspace: workspace,
			Status:    status,
			Limit:     10,
		})
		if err == nil && len(runs) == want {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("timed out waiting for %d run(s) with status %q", want, status)
		case <-ticker.C:
		}
	}
}

func waitForString(t *testing.T, ch <-chan string, want string) {
	t.Helper()

	select {
	case got := <-ch:
		if got != want {
			t.Fatalf("channel value = %q, want %q", got, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %q", want)
	}
}

func waitForCondition(t *testing.T, timeout time.Duration, label string, fn func() bool) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		if fn() {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("timed out waiting for %s", label)
		case <-ticker.C:
		}
	}
}

func waitForStreamItem(t *testing.T, ch <-chan apicore.RunStreamItem) apicore.RunStreamItem {
	t.Helper()

	select {
	case item := <-ch:
		return item
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for stream item")
		return apicore.RunStreamItem{}
	}
}

func waitForClosedRunStream(t *testing.T, stream apicore.RunStream) {
	t.Helper()

	select {
	case _, ok := <-stream.Events():
		if ok {
			t.Fatal("stream.Events() remained open, want closed channel")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for stream events channel to close")
	}

	select {
	case _, ok := <-stream.Errors():
		if ok {
			t.Fatal("stream.Errors() remained open, want closed channel")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for stream errors channel to close")
	}
}

func assertProblemStatus(t *testing.T, err error, want int) {
	t.Helper()

	var problem *apicore.Problem
	if !errors.As(err, &problem) {
		t.Fatalf("error = %T %v, want *core.Problem", err, err)
	}
	if problem.Status != want {
		t.Fatalf("problem status = %d, want %d", problem.Status, want)
	}
}

func openGlobalCatalog(t *testing.T, path string) *sql.DB {
	t.Helper()

	db, err := store.OpenSQLiteDatabase(context.Background(), path, nil)
	if err != nil {
		t.Fatalf("OpenSQLiteDatabase(%q) error = %v", path, err)
	}
	return db
}

func queryWorkflowCheckpointChecksum(t *testing.T, dbPath string, workflowSlug string) string {
	t.Helper()

	db := openGlobalCatalog(t, dbPath)
	defer func() {
		_ = db.Close()
	}()

	var checksum string
	if err := db.QueryRowContext(
		context.Background(),
		`SELECT sc.checksum
		 FROM sync_checkpoints sc
		 JOIN workflows w ON w.id = sc.workflow_id
		 WHERE w.slug = ? AND sc.scope = 'workflow' AND w.archived_at IS NULL`,
		workflowSlug,
	).Scan(&checksum); err != nil {
		t.Fatalf("query checkpoint checksum for %q: %v", workflowSlug, err)
	}
	return checksum
}

func queryTaskItem(t *testing.T, dbPath string, workflowSlug string, taskNumber int) (string, string, bool) {
	t.Helper()

	db := openGlobalCatalog(t, dbPath)
	defer func() {
		_ = db.Close()
	}()

	var (
		title  string
		status string
	)
	err := db.QueryRowContext(
		context.Background(),
		`SELECT ti.title, ti.status
		 FROM task_items ti
		 JOIN workflows w ON w.id = ti.workflow_id
		 WHERE w.slug = ? AND ti.task_number = ? AND w.archived_at IS NULL`,
		workflowSlug,
		taskNumber,
	).Scan(&title, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", false
	}
	if err != nil {
		t.Fatalf("query task item %q/%d: %v", workflowSlug, taskNumber, err)
	}
	return title, status, true
}

func queryReviewIssueStatus(
	t *testing.T,
	dbPath string,
	workflowSlug string,
	roundNumber int,
	issueNumber int,
) (string, bool) {
	t.Helper()

	db := openGlobalCatalog(t, dbPath)
	defer func() {
		_ = db.Close()
	}()

	var status string
	err := db.QueryRowContext(
		context.Background(),
		`SELECT ri.status
		 FROM review_issues ri
		 JOIN review_rounds rr ON rr.id = ri.round_id
		 JOIN workflows w ON w.id = rr.workflow_id
		 WHERE w.slug = ? AND rr.round_number = ? AND ri.issue_number = ? AND w.archived_at IS NULL`,
		workflowSlug,
		roundNumber,
		issueNumber,
	).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false
	}
	if err != nil {
		t.Fatalf("query review issue %q/%d/%d: %v", workflowSlug, roundNumber, issueNumber, err)
	}
	return status, true
}

func queryArtifactSnapshotBody(t *testing.T, dbPath string, workflowSlug string, relativePath string) (string, bool) {
	t.Helper()

	db := openGlobalCatalog(t, dbPath)
	defer func() {
		_ = db.Close()
	}()

	var body sql.NullString
	err := db.QueryRowContext(
		context.Background(),
		`SELECT a.body_text
		 FROM artifact_snapshots a
		 JOIN workflows w ON w.id = a.workflow_id
		 WHERE w.slug = ? AND a.relative_path = ? AND w.archived_at IS NULL`,
		workflowSlug,
		relativePath,
	).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false
	}
	if err != nil {
		t.Fatalf("query artifact body %q/%s: %v", workflowSlug, relativePath, err)
	}
	return body.String, true
}

func runArtifactSyncCount(t *testing.T, runID string) int {
	t.Helper()

	return len(runArtifactSyncLog(t, runID))
}

func runArtifactSyncLog(t *testing.T, runID string) []rundb.ArtifactSyncRow {
	t.Helper()

	runDB, err := openRunDB(context.Background(), runID)
	if err != nil {
		t.Fatalf("openRunDB(%q) error = %v", runID, err)
	}
	defer func() {
		_ = runDB.Close()
	}()

	rows, err := runDB.ListArtifactSyncLog(context.Background())
	if err != nil {
		t.Fatalf("ListArtifactSyncLog(%q) error = %v", runID, err)
	}
	return rows
}

func writeOwnedWorkflowArtifacts(t *testing.T, env *runManagerTestEnv, workflowSlug string) {
	t.Helper()

	env.writeWorkflowFile(t, workflowSlug, "task_01.md", daemonTaskBody("pending", "Workflow task"))
	env.writeWorkflowFile(t, workflowSlug, filepath.Join("memory", "MEMORY.md"), "# Workflow Memory\n")
	env.writeWorkflowFile(t, workflowSlug, filepath.Join("prompts", "task-run.md"), "# Prompt\n")
	env.writeWorkflowFile(t, workflowSlug, filepath.Join("protocol", "handoff.md"), "# Protocol\n")
	env.writeWorkflowFile(t, workflowSlug, filepath.Join("adrs", "adr-001.md"), "# ADR 001\n")
	env.writeWorkflowFile(t, workflowSlug, filepath.Join("qa", "verification-report.md"), "# QA\n")
	env.writeWorkflowFile(
		t,
		workflowSlug,
		filepath.Join("reviews-001", "_meta.md"),
		daemonReviewRoundMetaBody("coderabbit", "123", 1),
	)
	env.writeWorkflowFile(
		t,
		workflowSlug,
		filepath.Join("reviews-001", "issue_001.md"),
		daemonReviewIssueBody("pending", "medium"),
	)
}

func daemonTaskBody(status string, title string) string {
	return strings.Join([]string{
		"---",
		"status: " + status,
		"title: " + title,
		"type: backend",
		"complexity: low",
		"---",
		"",
		"# " + title,
		"",
	}, "\n")
}

func daemonLegacyWorkflowMetaBody() string {
	return strings.Join([]string{
		"---",
		"created_at: 2026-04-17T20:00:00Z",
		"updated_at: 2026-04-17T20:05:00Z",
		"---",
		"",
		"## Summary",
		"- Total: 1",
		"- Completed: 0",
		"- Pending: 1",
		"",
	}, "\n")
}

func daemonReviewRoundMetaBody(provider string, pr string, round int) string {
	return strings.Join([]string{
		"---",
		"provider: " + provider,
		"pr: " + pr,
		fmt.Sprintf("round: %d", round),
		"created_at: 2026-04-17T20:00:00Z",
		"---",
		"",
		"## Summary",
		"- Total: 1",
		"- Resolved: 0",
		"- Unresolved: 1",
		"",
	}, "\n")
}

func daemonReviewIssueBody(status string, severity string) string {
	return strings.Join([]string{
		"---",
		"status: " + status,
		"file: internal/app/service.go",
		"line: 42",
		"severity: " + severity,
		"author: coderabbitai[bot]",
		"provider_ref: thread:PRT_1,comment:RC_1",
		"---",
		"",
		"# Issue 001: Example",
		"",
	}, "\n")
}

func stringPtr(value string) *string {
	return &value
}

func intPtr(value int) *int {
	return &value
}

func boolPtr(value bool) *bool {
	return &value
}

func floatPtr(value float64) *float64 {
	return &value
}
