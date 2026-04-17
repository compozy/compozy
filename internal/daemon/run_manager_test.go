package daemon

import (
	"context"
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
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/plan"
	workspacecfg "github.com/compozy/compozy/internal/core/workspace"
	"github.com/compozy/compozy/internal/store/globaldb"
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

func (e *runManagerTestEnv) createReviewRound(t *testing.T, round int) string {
	t.Helper()

	reviewDir := filepath.Join(
		model.TaskDirectoryForWorkspace(e.workspaceRoot, e.workflowSlug),
		fmt.Sprintf("reviews-%03d", round),
	)
	if err := os.MkdirAll(reviewDir, 0o755); err != nil {
		t.Fatalf("mkdir review round dir: %v", err)
	}
	return reviewDir
}

func (e *runManagerTestEnv) startTaskRun(t *testing.T, runID string, runtimeOverrides json.RawMessage) apicore.Run {
	t.Helper()

	if len(runtimeOverrides) == 0 {
		runtimeOverrides = rawJSON(t, fmt.Sprintf(`{"run_id":%q}`, runID))
	}

	run, err := e.manager.StartTaskRun(context.Background(), e.workspaceRoot, e.workflowSlug, apicore.TaskRunRequest{
		Workspace:        e.workspaceRoot,
		PresentationMode: defaultPresentationMode,
		RuntimeOverrides: runtimeOverrides,
	})
	if err != nil {
		t.Fatalf("StartTaskRun(%q) error = %v", runID, err)
	}
	return run
}

func (e *runManagerTestEnv) startReviewRun(
	t *testing.T,
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
		e.workflowSlug,
		round,
		apicore.ReviewRunRequest{
			Workspace:        e.workspaceRoot,
			PresentationMode: defaultPresentationMode,
			RuntimeOverrides: runtimeOverrides,
			Batching:         batching,
		},
	)
	if err != nil {
		t.Fatalf("StartReviewRun(%q) error = %v", runID, err)
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
