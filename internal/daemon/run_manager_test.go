package daemon

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
	corepkg "github.com/compozy/compozy/internal/core"
	extensions "github.com/compozy/compozy/internal/core/extension"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/plan"
	runpkg "github.com/compozy/compozy/internal/core/run"
	"github.com/compozy/compozy/internal/core/run/journal"
	"github.com/compozy/compozy/internal/core/run/recovery"
	"github.com/compozy/compozy/internal/core/taskgroups"
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

	runArtifacts := env.manager.runArtifacts(run.RunID)
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

func TestRunManagerCanceledContendedReviewStartLeavesNoRun(t *testing.T) {
	t.Run("Should leave no run when a contended start is canceled", func(t *testing.T) {
		const runID = "review-run-canceled-before-commit"

		type writerLockResult struct {
			tx  *sql.Tx
			err error
		}
		writerLockCh := make(chan writerLockResult, 1)
		var writerDB *sql.DB
		var executed atomic.Bool
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			syncWorkflow: func(
				ctx context.Context,
				db *globaldb.GlobalDB,
				workspace globaldb.Workspace,
				cfg model.SyncConfig,
			) (*corepkg.SyncResult, error) {
				writerTx, err := writerDB.BeginTx(context.Background(), nil)
				if err == nil {
					_, err = writerTx.ExecContext(
						context.Background(),
						`UPDATE workspaces SET updated_at = updated_at WHERE id = ?`,
						workspace.ID,
					)
				}
				if err != nil && writerTx != nil {
					if rollbackErr := writerTx.Rollback(); rollbackErr != nil {
						err = errors.Join(err, fmt.Errorf("rollback failed writer transaction: %w", rollbackErr))
					}
					writerTx = nil
				}
				writerLockCh <- writerLockResult{tx: writerTx, err: err}
				if err != nil {
					return nil, err
				}
				return corepkg.SyncWithDB(ctx, db, workspace, cfg)
			},
			execute: func(context.Context, *model.SolvePreparation, *model.RuntimeConfig) error {
				executed.Store(true)
				return nil
			},
		})
		env.createReviewRound(t, 1)

		workspace, err := env.globalDB.ResolveOrRegister(context.Background(), env.workspaceRoot)
		if err != nil {
			t.Fatalf("ResolveOrRegister() error = %v", err)
		}
		if _, err := corepkg.SyncWithDB(
			context.Background(),
			env.globalDB,
			workspace,
			model.SyncConfig{TasksDir: env.workflowDir(env.workflowSlug)},
		); err != nil {
			t.Fatalf("initial SyncWithDB() error = %v", err)
		}

		writerDB, err = store.OpenSQLiteDatabase(context.Background(), env.paths.GlobalDBPath, nil)
		if err != nil {
			t.Fatalf("OpenSQLiteDatabase(writer) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := writerDB.Close(); closeErr != nil {
				t.Errorf("close writer database: %v", closeErr)
			}
		})
		var writerTx *sql.Tx
		writerReleased := false
		t.Cleanup(func() {
			if writerTx != nil && !writerReleased {
				if rollbackErr := writerTx.Rollback(); rollbackErr != nil {
					t.Errorf("rollback writer transaction during cleanup: %v", rollbackErr)
				}
			}
		})

		type startResult struct {
			run apicore.Run
			err error
		}
		ctx, cancel := context.WithCancel(context.Background())
		runtimeOverrides := rawJSON(t, `{"run_id":"`+runID+`","dry_run":true}`)
		resultCh := make(chan startResult, 1)
		go func() {
			run, startErr := env.manager.StartReviewRun(
				ctx,
				env.workspaceRoot,
				env.workflowSlug,
				1,
				apicore.ReviewRunRequest{
					Workspace:        env.workspaceRoot,
					PresentationMode: defaultPresentationMode,
					RuntimeOverrides: runtimeOverrides,
				},
			)
			resultCh <- startResult{run: run, err: startErr}
		}()

		select {
		case lockResult := <-writerLockCh:
			if lockResult.err != nil {
				t.Fatalf("hold global.db writer lock: %v", lockResult.err)
			}
			writerTx = lockResult.tx
		case <-time.After(5 * time.Second):
			t.Fatal("StartReviewRun() did not enter pre-run sync")
		}
		cancel()

		var result startResult
		returnedWhileLocked := false
		select {
		case result = <-resultCh:
			returnedWhileLocked = true
		case <-time.After(time.Second):
		}

		if writerTx == nil {
			t.Fatal("global.db writer transaction = nil")
		}
		if err := writerTx.Rollback(); err != nil {
			t.Fatalf("release global.db writer lock: %v", err)
		}
		writerReleased = true
		if !returnedWhileLocked {
			select {
			case result = <-resultCh:
			case <-time.After(5 * time.Second):
				t.Fatal("StartReviewRun() did not return after releasing writer lock")
			}
			t.Fatal("StartReviewRun() ignored cancellation while blocked on global.db")
		}

		if !errors.Is(result.err, context.Canceled) {
			t.Fatalf("StartReviewRun() error = %v, want context.Canceled", result.err)
		}
		if result.run.RunID != "" {
			t.Fatalf("StartReviewRun() run id = %q, want empty", result.run.RunID)
		}
		if _, err := env.globalDB.GetRun(context.Background(), runID); !errors.Is(err, globaldb.ErrRunNotFound) {
			t.Fatalf("GetRun(%q) error = %v, want ErrRunNotFound", runID, err)
		}
		if _, err := os.Stat(env.manager.runArtifacts(runID).RunDir); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("stat canceled run directory error = %v, want os.ErrNotExist", err)
		}
		if executed.Load() {
			t.Fatal("canceled review start executed after returning an error")
		}
	})
}

func TestRunManagerCallerCancellationAfterCommitDoesNotOwnStartup(t *testing.T) {
	t.Run("Should complete daemon-owned startup after caller cancellation", func(t *testing.T) {
		const runID = "review-run-daemon-owned-after-commit"

		scopeEntered := make(chan struct{})
		releaseScope := make(chan struct{})
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			openRunScope: func(
				ctx context.Context,
				cfg *model.RuntimeConfig,
				_ model.OpenRunScopeOptions,
			) (model.RunScope, error) {
				close(scopeEntered)
				<-releaseScope
				if ctx.Done() == nil {
					return nil, errors.New("post-commit startup context has no daemon cancellation")
				}
				return model.OpenBaseRunScope(ctx, cfg)
			},
		})
		env.createReviewRound(t, 1)

		type startResult struct {
			run apicore.Run
			err error
		}
		callerCtx, cancelCaller := context.WithCancel(context.Background())
		runtimeOverrides := rawJSON(t, `{"run_id":"`+runID+`","dry_run":true}`)
		resultCh := make(chan startResult, 1)
		go func() {
			run, err := env.manager.StartReviewRun(
				callerCtx,
				env.workspaceRoot,
				env.workflowSlug,
				1,
				apicore.ReviewRunRequest{
					Workspace:        env.workspaceRoot,
					PresentationMode: defaultPresentationMode,
					RuntimeOverrides: runtimeOverrides,
				},
			)
			resultCh <- startResult{run: run, err: err}
		}()

		select {
		case <-scopeEntered:
		case <-time.After(5 * time.Second):
			t.Fatal("StartReviewRun() did not reach post-commit scope initialization")
		}
		row, err := env.globalDB.GetRun(context.Background(), runID)
		if err != nil {
			t.Fatalf("GetRun(%q) after scope entry error = %v", runID, err)
		}
		if row.Status != runStatusStarting {
			t.Fatalf("committed row status = %q, want %q", row.Status, runStatusStarting)
		}

		cancelCaller()
		close(releaseScope)

		var result startResult
		select {
		case result = <-resultCh:
		case <-time.After(5 * time.Second):
			t.Fatal("StartReviewRun() did not finish daemon-owned startup")
		}
		if result.err != nil {
			t.Fatalf("StartReviewRun() after caller cancellation error = %v", result.err)
		}
		if result.run.RunID != runID {
			t.Fatalf("StartReviewRun() run id = %q, want %q", result.run.RunID, runID)
		}
		terminal := waitForRun(t, env.globalDB, runID, func(row globaldb.Run) bool {
			return isTerminalRunStatus(row.Status)
		})
		if terminal.Status != runStatusCompleted {
			t.Fatalf("terminal status = %q, want %q", terminal.Status, runStatusCompleted)
		}
	})
}

func TestRunManagerRejectsCompletedTaskWorkflowBeforeCreatingRun(t *testing.T) {
	env := newRunManagerTestEnv(t, runManagerTestDeps{})
	env.writeWorkflowFile(t, env.workflowSlug, "task_01.md", daemonTaskBody("completed", "Done task"))

	const runID = "task-run-no-pending"
	_, err := env.manager.StartTaskRun(
		context.Background(),
		env.workspaceRoot,
		env.workflowSlug,
		apicore.TaskRunRequest{
			Workspace:        env.workspaceRoot,
			PresentationMode: defaultPresentationMode,
			RuntimeOverrides: rawJSON(t, `{"run_id":"`+runID+`","dry_run":true}`),
		},
	)
	var problem *apicore.Problem
	if !errors.As(err, &problem) {
		t.Fatalf("StartTaskRun(completed workflow) error = %v, want problem", err)
	}
	if problem.Status != http.StatusConflict || problem.Code != "workflow_no_pending_tasks" {
		t.Fatalf("problem = status:%d code:%q, want 409 workflow_no_pending_tasks", problem.Status, problem.Code)
	}
	if got := problem.Details["task_pending"]; got != 0 {
		t.Fatalf("problem task_pending = %#v, want 0", got)
	}
	if _, err := env.globalDB.GetRun(context.Background(), runID); !errors.Is(err, globaldb.ErrRunNotFound) {
		t.Fatalf("GetRun(%q) error = %v, want ErrRunNotFound", runID, err)
	}
}

func TestRunManagerIncludeCompletedStartsCompletedTaskWorkflow(t *testing.T) {
	t.Run("Should include completed tasks when include_completed is true", func(t *testing.T) {
		seenIncludeCompleted := make(chan bool, 1)
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
				return &model.SolvePreparation{}, nil
			},
			execute: func(_ context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
				seenIncludeCompleted <- cfg.IncludeCompleted
				return nil
			},
		})
		env.writeWorkflowFile(t, env.workflowSlug, "task_01.md", daemonTaskBody("completed", "Done task"))

		const runID = "task-run-include-completed"
		run := env.startTaskRun(
			t,
			runID,
			rawJSON(t, `{"run_id":"`+runID+`","include_completed":true}`),
		)
		if !waitForBool(t, seenIncludeCompleted) {
			t.Fatal("execute saw IncludeCompleted=false, want true")
		}
		terminal := waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
			return row.Status == runStatusCompleted
		})
		if terminal.EndedAt == nil {
			t.Fatal("EndedAt = nil, want terminal timestamp")
		}
	})
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
	t.Run("Should include jobs transcript and next cursor", func(t *testing.T) {
		executed := make(chan string, 1)
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
				return &model.SolvePreparation{}, nil
			},
			execute: func(ctx context.Context, prep *model.SolvePreparation, cfg *model.RuntimeConfig) error {
				runArtifacts, err := model.ResolveRuntimeRunArtifacts(cfg)
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
				submitEvent(
					ctx,
					t,
					prep.Journal(),
					cfg.RunID,
					eventspkg.EventKindSessionUpdate,
					kinds.SessionUpdatePayload{
						Index: 1,
						Update: kinds.SessionUpdate{
							Kind:   kinds.UpdateKindAgentMessageChunk,
							Status: kinds.StatusRunning,
							Blocks: []kinds.ContentBlock{textBlock},
						},
					},
				)
				submitEvent(
					ctx,
					t,
					prep.Journal(),
					cfg.RunID,
					eventspkg.EventKindJobCompleted,
					kinds.JobCompletedPayload{
						JobAttemptInfo: kinds.JobAttemptInfo{Index: 1, Attempt: 1, MaxAttempts: 1},
					},
				)
				submitEvent(
					ctx,
					t,
					prep.Journal(),
					cfg.RunID,
					eventspkg.EventKindRunCompleted,
					kinds.RunCompletedPayload{
						ArtifactsDir:   runArtifacts.RunDir,
						ResultPath:     runArtifacts.ResultPath,
						SummaryMessage: "completed for snapshot",
					},
				)
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
	})
}

func TestRunManagerHistoricalSnapshotAndTranscriptUseCompactProjection(t *testing.T) {
	t.Run("Should use compact projection for historical snapshot and transcript", func(t *testing.T) {
		executed := make(chan string, 1)
		const obsoleteOutput = "obsolete streamed output"
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
				return &model.SolvePreparation{}, nil
			},
			execute: func(ctx context.Context, prep *model.SolvePreparation, cfg *model.RuntimeConfig) error {
				runArtifacts, err := model.ResolveRuntimeRunArtifacts(cfg)
				if err != nil {
					return err
				}
				submitEvent(ctx, t, prep.Journal(), cfg.RunID, eventspkg.EventKindJobQueued, kinds.JobQueuedPayload{
					Index:    0,
					SafeName: "batch_001",
					IDE:      "codex",
				})
				submitEvent(ctx, t, prep.Journal(), cfg.RunID, eventspkg.EventKindJobStarted, kinds.JobStartedPayload{
					JobAttemptInfo: kinds.JobAttemptInfo{Index: 0, Attempt: 1, MaxAttempts: 1},
					IDE:            "codex",
				})
				submitEvent(ctx, t, prep.Journal(), cfg.RunID, eventspkg.EventKindJobQueued, kinds.JobQueuedPayload{
					Index:    1,
					SafeName: "batch_002",
					IDE:      "cursor",
				})
				submitEvent(ctx, t, prep.Journal(), cfg.RunID, eventspkg.EventKindJobStarted, kinds.JobStartedPayload{
					JobAttemptInfo: kinds.JobAttemptInfo{Index: 1, Attempt: 1, MaxAttempts: 1},
					IDE:            "cursor",
				})
				submitEvent(
					ctx,
					t,
					prep.Journal(),
					cfg.RunID,
					eventspkg.EventKindSessionUpdate,
					kinds.SessionUpdatePayload{
						Index: 0,
						Update: kinds.SessionUpdate{
							Kind:       kinds.UpdateKindToolCallStarted,
							Status:     kinds.StatusRunning,
							ToolCallID: "tool-1",
							Blocks: []kinds.ContentBlock{
								mustRunManagerToolUseBlock(t, "tool-1", "Bash", `{"command":"make verify"}`),
							},
						},
					},
				)
				submitEvent(
					ctx,
					t,
					prep.Journal(),
					cfg.RunID,
					eventspkg.EventKindSessionUpdate,
					kinds.SessionUpdatePayload{
						Index: 1,
						Update: kinds.SessionUpdate{
							Kind:       kinds.UpdateKindToolCallStarted,
							Status:     kinds.StatusRunning,
							ToolCallID: "tool-1",
							Blocks: []kinds.ContentBlock{
								mustRunManagerToolUseBlock(t, "tool-1", "Bash", `{"command":"go test ./..."}`),
							},
						},
					},
				)
				for range 3 {
					submitEvent(
						ctx,
						t,
						prep.Journal(),
						cfg.RunID,
						eventspkg.EventKindSessionUpdate,
						kinds.SessionUpdatePayload{
							Index: 0,
							Update: kinds.SessionUpdate{
								Kind:       kinds.UpdateKindToolCallUpdated,
								Status:     kinds.StatusRunning,
								ToolCallID: "tool-1",
								Blocks: []kinds.ContentBlock{
									mustRunManagerToolResultBlock(
										t,
										"tool-1",
										obsoleteOutput+strings.Repeat("x", maxSnapshotTranscriptBytes),
									),
								},
							},
						},
					)
				}
				submitEvent(
					ctx,
					t,
					prep.Journal(),
					cfg.RunID,
					eventspkg.EventKindSessionUpdate,
					kinds.SessionUpdatePayload{
						Index: 0,
						Update: kinds.SessionUpdate{
							Kind:       kinds.UpdateKindToolCallUpdated,
							Status:     kinds.StatusCompleted,
							ToolCallID: "tool-1",
							Blocks: []kinds.ContentBlock{
								mustRunManagerToolResultBlock(t, "tool-1", "final output"),
							},
						},
					},
				)
				submitEvent(
					ctx,
					t,
					prep.Journal(),
					cfg.RunID,
					eventspkg.EventKindSessionUpdate,
					kinds.SessionUpdatePayload{
						Index: 1,
						Update: kinds.SessionUpdate{
							Kind:       kinds.UpdateKindToolCallUpdated,
							Status:     kinds.StatusCompleted,
							ToolCallID: "tool-1",
							Blocks: []kinds.ContentBlock{
								mustRunManagerToolResultBlock(t, "tool-1", "job 1 final output"),
							},
						},
					},
				)
				submitEvent(
					ctx,
					t,
					prep.Journal(),
					cfg.RunID,
					eventspkg.EventKindSessionUpdate,
					kinds.SessionUpdatePayload{
						Index: 0,
						Update: kinds.SessionUpdate{
							Kind:   kinds.UpdateKindAgentMessageChunk,
							Status: kinds.StatusCompleted,
							Blocks: []kinds.ContentBlock{
								mustRunManagerTextBlock(t, "done"),
							},
						},
					},
				)
				submitEvent(
					ctx,
					t,
					prep.Journal(),
					cfg.RunID,
					eventspkg.EventKindSessionUpdate,
					kinds.SessionUpdatePayload{
						Index: 1,
						Update: kinds.SessionUpdate{
							Kind:   kinds.UpdateKindAgentMessageChunk,
							Status: kinds.StatusCompleted,
							Blocks: []kinds.ContentBlock{
								mustRunManagerTextBlock(t, "job 1 done"),
							},
						},
					},
				)
				submitEvent(
					ctx,
					t,
					prep.Journal(),
					cfg.RunID,
					eventspkg.EventKindJobCompleted,
					kinds.JobCompletedPayload{
						JobAttemptInfo: kinds.JobAttemptInfo{Index: 0, Attempt: 1, MaxAttempts: 1},
					},
				)
				submitEvent(
					ctx,
					t,
					prep.Journal(),
					cfg.RunID,
					eventspkg.EventKindJobCompleted,
					kinds.JobCompletedPayload{
						JobAttemptInfo: kinds.JobAttemptInfo{Index: 1, Attempt: 1, MaxAttempts: 1},
					},
				)
				submitEvent(
					ctx,
					t,
					prep.Journal(),
					cfg.RunID,
					eventspkg.EventKindRunCompleted,
					kinds.RunCompletedPayload{
						ArtifactsDir:   runArtifacts.RunDir,
						ResultPath:     runArtifacts.ResultPath,
						SummaryMessage: "completed for compact projection",
					},
				)
				executed <- cfg.RunID
				return nil
			},
		})

		run := env.startTaskRun(t, "task-run-compact-history", nil)
		waitForString(t, executed, run.RunID)
		waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
			return row.Status == runStatusCompleted
		})

		snapshot, err := env.manager.Snapshot(context.Background(), run.RunID)
		if err != nil {
			t.Fatalf("Snapshot(%q) error = %v", run.RunID, err)
		}
		if len(snapshot.Jobs) != 2 || snapshot.Jobs[0].Status != runStatusCompleted ||
			snapshot.Jobs[1].Status != runStatusCompleted {
			t.Fatalf("snapshot jobs = %#v, want two completed jobs", snapshot.Jobs)
		}
		if snapshot.Jobs[0].Summary == nil || snapshot.Jobs[1].Summary == nil {
			t.Fatal("snapshot job summary = nil, want compact lifecycle metadata for both jobs")
		}
		for index, job := range snapshot.Jobs {
			if entries := job.Summary.Session.Entries; len(entries) == 0 {
				t.Fatalf("historical snapshot job %d session entries are empty, want compact ACP transcript", index)
			}
		}
		firstSessionJSON, err := json.Marshal(snapshot.Jobs[0].Summary.Session)
		if err != nil {
			t.Fatalf("json.Marshal(snapshot job 0 session) error = %v", err)
		}
		firstSessionPayload := string(firstSessionJSON)
		for _, want := range []string{"make verify", "final output", "done"} {
			if !strings.Contains(firstSessionPayload, want) {
				t.Fatalf("snapshot job 0 session missing %q: %s", want, firstSessionPayload)
			}
		}
		for _, unwanted := range []string{obsoleteOutput, "go test ./...", "job 1 final output", "job 1 done"} {
			if strings.Contains(firstSessionPayload, unwanted) {
				t.Fatalf("snapshot job 0 session retained unrelated %q", unwanted)
			}
		}
		secondSessionJSON, err := json.Marshal(snapshot.Jobs[1].Summary.Session)
		if err != nil {
			t.Fatalf("json.Marshal(snapshot job 1 session) error = %v", err)
		}
		secondSessionPayload := string(secondSessionJSON)
		for _, want := range []string{"go test ./...", "job 1 final output", "job 1 done"} {
			if !strings.Contains(secondSessionPayload, want) {
				t.Fatalf("snapshot job 1 session missing %q: %s", want, secondSessionPayload)
			}
		}
		for _, unwanted := range []string{obsoleteOutput, "make verify"} {
			if strings.Contains(secondSessionPayload, unwanted) {
				t.Fatalf("snapshot job 1 session retained unrelated %q", unwanted)
			}
		}
		snapshotJSON, err := json.Marshal(snapshot)
		if err != nil {
			t.Fatalf("json.Marshal(snapshot) error = %v", err)
		}
		if strings.Contains(string(snapshotJSON), obsoleteOutput) {
			t.Fatal("snapshot payload retained superseded large tool output")
		}

		transcript, err := env.manager.Transcript(context.Background(), run.RunID)
		if err != nil {
			t.Fatalf("Transcript(%q) error = %v", run.RunID, err)
		}
		transcriptJSON, err := json.Marshal(transcript)
		if err != nil {
			t.Fatalf("json.Marshal(transcript) error = %v", err)
		}
		transcriptPayload := string(transcriptJSON)
		for _, want := range []string{
			"make verify",
			"final output",
			"done",
			"go test ./...",
			"job 1 final output",
			"job 1 done",
		} {
			if !strings.Contains(transcriptPayload, want) {
				t.Fatalf("transcript payload missing %q: %s", want, transcriptPayload)
			}
		}
		if strings.Contains(transcriptPayload, obsoleteOutput) {
			t.Fatal("transcript payload retained superseded large tool output")
		}
	})
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

func TestRunManagerSnapshotBootstrapsPreparedQueuedReviewJobs(t *testing.T) {
	ready := make(chan string, 1)
	release := make(chan struct{})
	env := newRunManagerTestEnv(t, runManagerTestDeps{
		prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
			return &model.SolvePreparation{
				Jobs: []model.Job{
					{
						CodeFiles:       []string{"issue_001.md", "issue_002.md", "issue_003.md", "issue_004.md"},
						Groups:          map[string][]model.IssueEntry{"batch-a": make([]model.IssueEntry, 20)},
						TaskTitle:       "review batch 001",
						TaskType:        "review",
						SafeName:        "job-000",
						IDE:             "codex",
						Model:           "gpt-5.5",
						ReasoningEffort: "high",
						OutLog:          "/tmp/job-000.out.log",
						ErrLog:          "/tmp/job-000.err.log",
					},
					{
						CodeFiles:       []string{"issue_021.md", "issue_022.md"},
						Groups:          map[string][]model.IssueEntry{"batch-b": make([]model.IssueEntry, 15)},
						TaskTitle:       "review batch 002",
						TaskType:        "review",
						SafeName:        "job-001",
						IDE:             "codex",
						Model:           "gpt-5.5",
						ReasoningEffort: "high",
						OutLog:          "/tmp/job-001.out.log",
						ErrLog:          "/tmp/job-001.err.log",
					},
				},
			}, nil
		},
		execute: func(_ context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
			ready <- cfg.RunID
			<-release
			return context.Canceled
		},
	})
	env.createReviewRound(t, 1)

	run := env.startReviewRun(
		t,
		"review-run-queued-snapshot",
		1,
		nil,
		rawJSON(t, `{"batch_size":20,"concurrent":1}`),
	)
	waitForString(t, ready, run.RunID)
	waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusRunning
	})

	var snapshot apicore.RunSnapshot
	waitForCondition(t, 5*time.Second, "prepared queued review jobs in snapshot", func() bool {
		var err error
		snapshot, err = env.manager.Snapshot(context.Background(), run.RunID)
		if err != nil {
			t.Fatalf("Snapshot(%q) error = %v", run.RunID, err)
		}
		return len(snapshot.Jobs) == 2
	})

	first := snapshot.Jobs[0]
	if first.Status != "queued" {
		t.Fatalf("first queued status = %q, want queued", first.Status)
	}
	if first.Summary == nil {
		t.Fatal("first queued summary = nil, want populated batch metadata")
	}
	if first.Summary.Issues != 20 {
		t.Fatalf("first queued issues = %d, want 20", first.Summary.Issues)
	}
	if got := len(first.Summary.CodeFiles); got != 4 {
		t.Fatalf("first queued file count = %d, want 4", got)
	}
	if first.Summary.TaskTitle != "review batch 001" || first.Summary.SafeName != "job-000" {
		t.Fatalf("unexpected first queued summary: %#v", first.Summary)
	}

	second := snapshot.Jobs[1]
	if second.Status != "queued" {
		t.Fatalf("second queued status = %q, want queued", second.Status)
	}
	if second.Summary == nil {
		t.Fatal("second queued summary = nil, want populated batch metadata")
	}
	if second.Summary.Issues != 15 {
		t.Fatalf("second queued issues = %d, want 15", second.Summary.Issues)
	}
	if got := len(second.Summary.CodeFiles); got != 2 {
		t.Fatalf("second queued file count = %d, want 2", got)
	}
	if second.Summary.TaskTitle != "review batch 002" || second.Summary.SafeName != "job-001" {
		t.Fatalf("unexpected second queued summary: %#v", second.Summary)
	}

	close(release)
	waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusCancelled
	})
}

func TestRunManagerListBatchesWorkflowSlugLookup(t *testing.T) {
	var (
		env              *runManagerTestEnv
		lookupCalls      atomic.Int64
		getWorkflowCalls atomic.Int64
	)
	env = newRunManagerTestEnv(t, runManagerTestDeps{
		lookupWorkflowSlugs: func(ctx context.Context, ids []string) (map[string]string, error) {
			lookupCalls.Add(1)
			return env.globalDB.WorkflowSlugsByIDs(ctx, ids)
		},
		getWorkflow: func(ctx context.Context, workflowID string) (globaldb.Workflow, error) {
			getWorkflowCalls.Add(1)
			return env.globalDB.GetWorkflow(ctx, workflowID)
		},
	})

	runA := env.startTaskRun(t, "task-run-list-a", nil)
	runB := env.startTaskRun(t, "task-run-list-b", nil)
	runC := env.startTaskRun(t, "task-run-list-c", nil)

	for _, runID := range []string{runA.RunID, runB.RunID, runC.RunID} {
		waitForRun(t, env.globalDB, runID, func(row globaldb.Run) bool {
			return row.Status == runStatusCompleted
		})
	}

	runs, err := env.manager.List(context.Background(), apicore.RunListQuery{
		Workspace: env.workspaceRoot,
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(runs) != 3 {
		t.Fatalf("len(runs) = %d, want 3", len(runs))
	}
	if got := lookupCalls.Load(); got != 1 {
		t.Fatalf("workflow slug lookups = %d, want 1", got)
	}
	if got := getWorkflowCalls.Load(); got != 0 {
		t.Fatalf("fallback workflow loads = %d, want 0", got)
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

func TestRunManagerPrepareFailurePersistsOriginalTerminalWithoutIntegrityIssue(t *testing.T) {
	prepareErr := errors.New("plan hook rejected preparation")
	runtimeManager := &stubRuntimeManager{
		mutableHook: "plan.post_prepare_jobs",
		mutableErr:  prepareErr,
	}
	var executeCalled atomic.Bool
	env := newRunManagerTestEnv(t, runManagerTestDeps{
		openRunScope: newTestOpenRunScope(runtimeManager),
		prepare:      plan.Prepare,
		execute: func(context.Context, *model.SolvePreparation, *model.RuntimeConfig) error {
			executeCalled.Store(true)
			return nil
		},
	})
	env.writeWorkflowFile(t, env.workflowSlug, "task_01.md", daemonTaskBody("pending", "Prepare failure"))

	const runID = "task-run-prepare-failure"
	run := env.startTaskRun(
		t,
		runID,
		rawJSON(t, `{"run_id":"`+runID+`","dry_run":true}`),
	)
	terminal := waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusFailed
	})
	if executeCalled.Load() {
		t.Fatal("execute called after prepare failure, want false")
	}
	if !strings.Contains(terminal.ErrorText, prepareErr.Error()) {
		t.Fatalf("terminal.ErrorText = %q, want original prepare error %q", terminal.ErrorText, prepareErr)
	}
	if strings.Contains(terminal.ErrorText, journal.ErrClosed.Error()) {
		t.Fatalf("terminal.ErrorText = %q, must not be replaced by journal closure", terminal.ErrorText)
	}

	lastEvent := env.lastRunEvent(t, run.RunID)
	if lastEvent == nil || lastEvent.Kind != eventspkg.EventKindRunFailed {
		t.Fatalf("last event = %#v, want run.failed", lastEvent)
	}
	var payload kinds.RunFailedPayload
	if err := json.Unmarshal(lastEvent.Payload, &payload); err != nil {
		t.Fatalf("decode run.failed payload: %v", err)
	}
	if !strings.Contains(payload.Error, prepareErr.Error()) {
		t.Fatalf("run.failed error = %q, want original prepare error %q", payload.Error, prepareErr)
	}

	snapshot, err := env.manager.Snapshot(context.Background(), run.RunID)
	if err != nil {
		t.Fatalf("Snapshot(%q) error = %v", run.RunID, err)
	}
	if snapshot.Incomplete || len(snapshot.IncompleteReasons) != 0 {
		t.Fatalf(
			"snapshot integrity = incomplete:%v reasons:%v, want complete",
			snapshot.Incomplete,
			snapshot.IncompleteReasons,
		)
	}
}

func TestRunManagerTerminalResolutionFailurePreservesOriginalRunError(t *testing.T) {
	prepareErr := errors.New("prepare failed before terminal persistence")
	terminalStoreErr := errors.New("terminal store unavailable")
	env := newRunManagerTestEnv(t, runManagerTestDeps{
		prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
			return nil, prepareErr
		},
		openRunDB: func(context.Context, string) (*rundb.RunDB, error) {
			return nil, terminalStoreErr
		},
	})
	env.writeWorkflowFile(t, env.workflowSlug, "task_01.md", daemonTaskBody("pending", "Terminal store failure"))

	const runID = "task-run-terminal-resolution-failure"
	run := env.startTaskRun(
		t,
		runID,
		rawJSON(t, `{"run_id":"`+runID+`","dry_run":true}`),
	)
	terminal := waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusFailed
	})
	for _, want := range []string{prepareErr.Error(), terminalStoreErr.Error()} {
		if !strings.Contains(terminal.ErrorText, want) {
			t.Fatalf("terminal.ErrorText = %q, want diagnostic %q", terminal.ErrorText, want)
		}
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

func TestRunManagerWorkspaceStreamFiltersAndDeliversEvents(t *testing.T) {
	t.Run("Should filter and deliver workspace events", func(t *testing.T) {
		env := newRunManagerTestEnv(t, runManagerTestDeps{})
		workspace, err := env.globalDB.ResolveOrRegister(context.Background(), env.workspaceRoot)
		if err != nil {
			t.Fatalf("ResolveOrRegister() error = %v", err)
		}

		stream, err := env.manager.OpenWorkspaceStream(context.Background(), workspace.ID)
		if err != nil {
			t.Fatalf("OpenWorkspaceStream() error = %v", err)
		}
		defer func() {
			_ = stream.Close()
		}()

		env.manager.publishWorkspaceEvent(context.Background(), apicore.WorkspaceEvent{
			WorkspaceID:  "other-workspace",
			Kind:         apicore.WorkspaceEventKindRunStatusChanged,
			RunID:        "ignored",
			WorkflowSlug: env.workflowSlug,
		})
		env.manager.publishWorkspaceEvent(context.Background(), apicore.WorkspaceEvent{
			WorkspaceID:  workspace.ID,
			Kind:         apicore.WorkspaceEventKindRunStatusChanged,
			RunID:        "run-1",
			WorkflowSlug: env.workflowSlug,
			Status:       runStatusRunning,
		})

		item := waitForWorkspaceStreamItem(t, stream.Events())
		if item.Event == nil {
			t.Fatalf("workspace stream item = %#v, want event", item)
		}
		if item.Event.WorkspaceID != workspace.ID ||
			item.Event.RunID != "run-1" ||
			item.Event.Kind != apicore.WorkspaceEventKindRunStatusChanged ||
			item.Event.Seq == 0 ||
			item.Event.TS.IsZero() {
			t.Fatalf("unexpected workspace event: %#v", item.Event)
		}
	})
}

func TestRunManagerRunDBCacheReusesSingleHandleAndEvictsIdleEntries(t *testing.T) {
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	var openCalls atomic.Int64

	env := newRunManagerTestEnv(t, runManagerTestDeps{
		now:           func() time.Time { return now },
		runDBCacheTTL: 50 * time.Millisecond,
	})
	openCapturedRunDB := env.manager.openRunDB
	env.manager.openRunDB = func(ctx context.Context, runID string) (*rundb.RunDB, error) {
		openCalls.Add(1)
		return openCapturedRunDB(ctx, runID)
	}

	run := env.startTaskRun(t, "task-run-cache", nil)
	waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusCompleted
	})
	baselineOpens := openCalls.Load()

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			if _, err := env.manager.Snapshot(context.Background(), run.RunID); err != nil {
				t.Errorf("Snapshot() error = %v", err)
			}
		}()
		go func() {
			defer wg.Done()
			if _, err := env.manager.Events(
				context.Background(),
				run.RunID,
				apicore.RunEventPageQuery{Limit: 10},
			); err != nil {
				t.Errorf("Events() error = %v", err)
			}
		}()
	}
	wg.Wait()

	if got := openCalls.Load() - baselineOpens; got != 1 {
		t.Fatalf("run db opens after concurrent reads = %d, want 1", got)
	}
	if got := len(env.manager.runDBs); got != 1 {
		t.Fatalf("cached run db count = %d, want 1", got)
	}

	now = now.Add(100 * time.Millisecond)
	if err := env.manager.evictIdleRunDBs(now); err != nil {
		t.Fatalf("evictIdleRunDBs() error = %v", err)
	}
	if got := len(env.manager.runDBs); got != 0 {
		t.Fatalf("cached run db count after idle eviction = %d, want 0", got)
	}

	if _, err := env.manager.Snapshot(context.Background(), run.RunID); err != nil {
		t.Fatalf("Snapshot(after eviction) error = %v", err)
	}
	if got := openCalls.Load() - baselineOpens; got != 2 {
		t.Fatalf("run db opens after cache eviction = %d, want 2", got)
	}
}

func TestRunManagerOpenStreamReplaysAllPersistedPages(t *testing.T) {
	const totalUpdates = defaultStreamPageLimit*2 + 17

	env := newRunManagerTestEnv(t, runManagerTestDeps{
		prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
			return &model.SolvePreparation{}, nil
		},
		execute: func(ctx context.Context, prep *model.SolvePreparation, cfg *model.RuntimeConfig) error {
			for idx := 1; idx <= totalUpdates; idx++ {
				block, err := kinds.NewContentBlock(kinds.TextBlock{Text: fmt.Sprintf("chunk-%03d", idx)})
				if err != nil {
					return err
				}
				submitEvent(
					ctx,
					t,
					prep.Journal(),
					cfg.RunID,
					eventspkg.EventKindSessionUpdate,
					kinds.SessionUpdatePayload{
						Index: 1,
						Update: kinds.SessionUpdate{
							Kind:   kinds.UpdateKindAgentMessageChunk,
							Status: kinds.StatusRunning,
							Blocks: []kinds.ContentBlock{block},
						},
					},
				)
			}
			return nil
		},
	})

	run := env.startTaskRun(t, "task-run-stream-pages", nil)
	waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusCompleted
	})

	stream, err := env.manager.OpenStream(context.Background(), run.RunID, apicore.StreamCursor{})
	if err != nil {
		t.Fatalf("OpenStream() error = %v", err)
	}
	defer func() {
		_ = stream.Close()
	}()

	deadline := time.After(5 * time.Second)
	count := 0
	var lastEvent *eventspkg.Event

	for {
		select {
		case item, ok := <-stream.Events():
			if !ok {
				if count != totalUpdates+1 {
					t.Fatalf("replayed event count = %d, want %d", count, totalUpdates+1)
				}
				if lastEvent == nil || lastEvent.Kind != eventspkg.EventKindRunCompleted {
					t.Fatalf("last replayed event = %#v, want run.completed", lastEvent)
				}
				return
			}
			if item.Event == nil {
				t.Fatalf("stream item = %#v, want event", item)
			}
			count++
			lastEvent = item.Event
		case err, ok := <-stream.Errors():
			if ok && err != nil {
				t.Fatalf("stream error = %v", err)
			}
		case <-deadline:
			t.Fatal("timed out draining replay stream")
		}
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

func TestRunManagerStartExecRunRetriesImplicitRunIDCollision(t *testing.T) {
	t.Run("Should retry an implicit exec run ID collision", func(t *testing.T) {
		const collidingRunID = "exec-implicit-collision"
		release := make(chan struct{})
		started := make(chan string, 1)
		var buildCalls atomic.Int64
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			buildRunID: func(*model.RuntimeConfig) (string, error) {
				call := buildCalls.Add(1)
				if call == 1 {
					return collidingRunID, nil
				}
				return fmt.Sprintf("%s-%d", collidingRunID, call), nil
			},
			executeExec: func(ctx context.Context, cfg *model.RuntimeConfig, _ model.RunScope) error {
				started <- cfg.RunID
				select {
				case <-release:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			},
		})
		collidingArtifacts := env.manager.runArtifacts(collidingRunID)
		if err := os.MkdirAll(collidingArtifacts.RunDir, 0o755); err != nil {
			t.Fatalf("mkdir colliding run dir: %v", err)
		}

		run, err := env.manager.StartExecRun(context.Background(), apicore.ExecRequest{
			WorkspacePath:    env.workspaceRoot,
			Prompt:           "daemon exec prompt",
			PresentationMode: defaultPresentationMode,
		})
		if err != nil {
			t.Fatalf("StartExecRun(implicit collision) error = %v", err)
		}
		if run.RunID != "exec-implicit-collision-2" {
			t.Fatalf("run.RunID = %q, want retry candidate", run.RunID)
		}
		waitForString(t, started, run.RunID)
		close(release)
		waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
			return row.Status == runStatusCompleted
		})
	})
}

func TestRunManagerStartExecRunExplicitDuplicateRunIDStillFails(t *testing.T) {
	t.Run("Should reject an explicit duplicate exec run ID", func(t *testing.T) {
		const runID = "exec-explicit-duplicate"
		release := make(chan struct{})
		started := make(chan string, 1)
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			executeExec: func(ctx context.Context, cfg *model.RuntimeConfig, _ model.RunScope) error {
				started <- cfg.RunID
				select {
				case <-release:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			},
		})

		run := env.startExecRun(t, runID, nil)
		waitForString(t, started, run.RunID)
		_, err := env.manager.StartExecRun(context.Background(), apicore.ExecRequest{
			WorkspacePath:    env.workspaceRoot,
			Prompt:           "daemon exec prompt",
			PresentationMode: defaultPresentationMode,
			RuntimeOverrides: rawJSON(t, "{\"run_id\":\""+runID+"\"}"),
		})
		if !errors.Is(err, globaldb.ErrRunAlreadyExists) {
			t.Fatalf("StartExecRun(explicit duplicate) error = %v, want ErrRunAlreadyExists", err)
		}
		close(release)
		waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
			return row.Status == runStatusCompleted
		})
	})
}

func TestRunManagerStartExecRunAllocatesDistinctImplicitRunIDsInParallel(t *testing.T) {
	t.Run("Should allocate distinct implicit exec run IDs in parallel", func(t *testing.T) {
		const totalRuns = 3
		release := make(chan struct{})
		started := make(chan string, totalRuns)
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			executeExec: func(ctx context.Context, cfg *model.RuntimeConfig, _ model.RunScope) error {
				started <- cfg.RunID
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
		results := make(chan result, totalRuns)
		for range totalRuns {
			go func() {
				<-start
				run, err := env.manager.StartExecRun(context.Background(), apicore.ExecRequest{
					WorkspacePath:    env.workspaceRoot,
					Prompt:           "daemon exec prompt",
					PresentationMode: defaultPresentationMode,
				})
				results <- result{run: run, err: err}
			}()
		}
		close(start)

		runs := make([]apicore.Run, 0, totalRuns)
		seen := make(map[string]struct{}, totalRuns)
		for range totalRuns {
			result := <-results
			if result.err != nil {
				t.Fatalf("StartExecRun(parallel implicit) error = %v", result.err)
			}
			if strings.TrimSpace(result.run.RunID) == "" {
				t.Fatal("parallel implicit run id is empty")
			}
			if _, ok := seen[result.run.RunID]; ok {
				t.Fatalf("duplicate implicit run id allocated: %q", result.run.RunID)
			}
			seen[result.run.RunID] = struct{}{}
			runs = append(runs, result.run)
		}

		startedIDs := make(map[string]struct{}, totalRuns)
		deadline := time.After(5 * time.Second)
		for len(startedIDs) < totalRuns {
			select {
			case runID := <-started:
				startedIDs[runID] = struct{}{}
			case <-deadline:
				t.Fatalf("timed out waiting for %d exec starts; got %d", totalRuns, len(startedIDs))
			}
		}
		for _, run := range runs {
			if _, ok := startedIDs[run.RunID]; !ok {
				t.Fatalf("run %q returned but executeExec did not start", run.RunID)
			}
		}

		close(release)
		for _, run := range runs {
			waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
				return row.Status == runStatusCompleted
			})
		}
	})
}

func TestRunManagerOpenRunScopeFailurePreservesFailedRun(t *testing.T) {
	t.Run("Should preserve a failed row and clean its reserved directory", func(t *testing.T) {
		scopeErr := errors.New("scope unavailable")
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			openRunScope: func(context.Context, *model.RuntimeConfig, model.OpenRunScopeOptions) (model.RunScope, error) {
				return nil, scopeErr
			},
		})
		const runID = "scope-open-failure"

		_, err := env.manager.StartTaskRun(
			context.Background(),
			env.workspaceRoot,
			env.workflowSlug,
			apicore.TaskRunRequest{
				Workspace:        env.workspaceRoot,
				PresentationMode: defaultPresentationMode,
				RuntimeOverrides: rawJSON(t, `{"run_id":"`+runID+`"}`),
			},
		)
		if !errors.Is(err, scopeErr) {
			t.Fatalf("StartTaskRun(open scope failure) error = %v, want %v", err, scopeErr)
		}

		runArtifacts := env.manager.runArtifacts(runID)
		if _, err := os.Stat(runArtifacts.RunDir); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("run dir stat error = %v, want not exist", err)
		}
		row := waitForRun(t, env.globalDB, runID, func(row globaldb.Run) bool {
			return row.Status == runStatusFailed
		})
		if row.EndedAt == nil {
			t.Fatal("EndedAt = nil, want terminal timestamp")
		}
		if !strings.Contains(row.ErrorText, scopeErr.Error()) {
			t.Fatalf("row.ErrorText = %q, want %q", row.ErrorText, scopeErr.Error())
		}
	})
}

func TestRunManagerStartExecRunOpenRunScopeFailureMarksResumedRowFailed(t *testing.T) {
	t.Run("Should fail a resumed exec row inserted from persisted metadata", func(t *testing.T) {
		scopeErr := errors.New("scope unavailable")
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			openRunScope: func(context.Context, *model.RuntimeConfig, model.OpenRunScopeOptions) (model.RunScope, error) {
				return nil, scopeErr
			},
		})
		const runID = "exec-resume-inserted"
		writePersistedExecRun(t, env.workspaceRoot, runID, time.Date(2026, 5, 13, 15, 0, 0, 0, time.UTC))

		_, err := env.manager.StartExecRun(context.Background(), apicore.ExecRequest{
			WorkspacePath:    env.workspaceRoot,
			Prompt:           "daemon exec prompt",
			PresentationMode: defaultPresentationMode,
			RuntimeOverrides: rawJSON(t, `{"run_id":"exec-resume-inserted"}`),
		})
		if !errors.Is(err, scopeErr) {
			t.Fatalf("StartExecRun(resumed inserted open scope failure) error = %v, want %v", err, scopeErr)
		}

		row := waitForRun(t, env.globalDB, runID, func(row globaldb.Run) bool {
			return row.Status == runStatusFailed
		})
		if row.EndedAt == nil {
			t.Fatal("EndedAt = nil, want terminal timestamp")
		}
		if !strings.Contains(row.ErrorText, scopeErr.Error()) {
			t.Fatalf("row.ErrorText = %q, want %q", row.ErrorText, scopeErr.Error())
		}
	})

	t.Run("Should fail a resumed exec row reset from an existing global row", func(t *testing.T) {
		scopeErr := errors.New("scope unavailable")
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			openRunScope: func(context.Context, *model.RuntimeConfig, model.OpenRunScopeOptions) (model.RunScope, error) {
				return nil, scopeErr
			},
		})
		const runID = "exec-resume-existing"
		createdAt := time.Date(2026, 5, 13, 15, 5, 0, 0, time.UTC)
		writePersistedExecRun(t, env.workspaceRoot, runID, createdAt)
		workspace, err := env.globalDB.ResolveOrRegister(context.Background(), env.workspaceRoot)
		if err != nil {
			t.Fatalf("ResolveOrRegister() error = %v", err)
		}
		if _, err := env.globalDB.PutRun(context.Background(), globaldb.Run{
			RunID:            runID,
			WorkspaceID:      workspace.ID,
			Mode:             runModeExec,
			Status:           runStatusCompleted,
			PresentationMode: defaultPresentationMode,
			StartedAt:        createdAt,
		}); err != nil {
			t.Fatalf("PutRun(existing exec) error = %v", err)
		}

		_, err = env.manager.StartExecRun(context.Background(), apicore.ExecRequest{
			WorkspacePath:    env.workspaceRoot,
			Prompt:           "daemon exec prompt",
			PresentationMode: defaultPresentationMode,
			RuntimeOverrides: rawJSON(t, `{"run_id":"exec-resume-existing"}`),
		})
		if !errors.Is(err, scopeErr) {
			t.Fatalf("StartExecRun(resumed existing open scope failure) error = %v, want %v", err, scopeErr)
		}

		row := waitForRun(t, env.globalDB, runID, func(row globaldb.Run) bool {
			return row.Status == runStatusFailed
		})
		if row.EndedAt == nil {
			t.Fatal("EndedAt = nil, want terminal timestamp")
		}
		if row.StartedAt != createdAt {
			t.Fatalf("row.StartedAt = %v, want %v", row.StartedAt, createdAt)
		}
		if !strings.Contains(row.ErrorText, scopeErr.Error()) {
			t.Fatalf("row.ErrorText = %q, want %q", row.ErrorText, scopeErr.Error())
		}
	})
}

func TestRunManagerStartRunSyncFailureLeavesNoRun(t *testing.T) {
	t.Run("Should leave no run when workflow synchronization fails", func(t *testing.T) {
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

		if _, err := env.globalDB.GetRun(context.Background(), runID); !errors.Is(err, globaldb.ErrRunNotFound) {
			t.Fatalf("GetRun(%q) error = %v, want ErrRunNotFound", runID, err)
		}
		if _, err := os.Stat(env.manager.runArtifacts(runID).RunDir); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("stat failed-sync run directory error = %v, want os.ErrNotExist", err)
		}
		if active := env.manager.getActive(runID); active != nil {
			t.Fatalf("active run after sync failure = %#v, want nil", active)
		}
	})
}

func TestRunManagerStartTaskRunBindsDaemonHostBridgeToRunScopeContext(t *testing.T) {
	var capturedToken string

	env := newRunManagerTestEnv(t, runManagerTestDeps{
		openRunScope: func(
			ctx context.Context,
			cfg *model.RuntimeConfig,
			_ model.OpenRunScopeOptions,
		) (model.RunScope, error) {
			bridge := extensions.DaemonHostBridgeFromContext(ctx)
			if bridge == nil {
				t.Fatal("DaemonHostBridgeFromContext() = nil, want daemon bridge")
			}
			capturedToken = bridge.HostCapabilityToken()
			return model.OpenBaseRunScope(ctx, cfg)
		},
	})

	run := env.startTaskRun(
		t,
		"daemon-bridge-task",
		rawJSON(t, `{"run_id":"daemon-bridge-task","enable_executable_extensions":true}`),
	)
	waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return isTerminalRunStatus(row.Status)
	})

	if strings.TrimSpace(capturedToken) == "" {
		t.Fatal("capturedToken is empty, want per-run host capability token")
	}
}

func TestExtensionBridgeStartRunCreatesDetachedExecRun(t *testing.T) {
	env := newRunManagerTestEnv(t, runManagerTestDeps{})

	bridge, err := newExtensionBridge(env.manager, env.workspaceRoot)
	if err != nil {
		t.Fatalf("newExtensionBridge() error = %v", err)
	}

	handle, err := bridge.StartRun(context.Background(), &model.RuntimeConfig{
		WorkspaceRoot: env.workspaceRoot,
		Mode:          model.ExecutionModeExec,
		PromptText:    "nested exec prompt",
		ParentRunID:   "parent-run-001",
	})
	if err != nil {
		t.Fatalf("StartRun() error = %v", err)
	}
	if handle.RunID == "" {
		t.Fatal("handle.RunID is empty")
	}
	if handle.ParentRunID != "parent-run-001" {
		t.Fatalf("handle.ParentRunID = %q, want %q", handle.ParentRunID, "parent-run-001")
	}

	row := waitForRun(t, env.globalDB, handle.RunID, func(row globaldb.Run) bool {
		return isTerminalRunStatus(row.Status)
	})
	if row.Mode != runModeExec {
		t.Fatalf("row.Mode = %q, want %q", row.Mode, runModeExec)
	}
	if row.PresentationMode != daemonExtensionPresentationMode {
		t.Fatalf("row.PresentationMode = %q, want %q", row.PresentationMode, daemonExtensionPresentationMode)
	}
	if row.ParentRunID != "parent-run-001" {
		t.Fatalf("row.ParentRunID = %q, want %q", row.ParentRunID, "parent-run-001")
	}
}

func TestExtensionBridgeStartRunRejectsNilContext(t *testing.T) {
	env := newRunManagerTestEnv(t, runManagerTestDeps{})

	bridge, err := newExtensionBridge(env.manager, env.workspaceRoot)
	if err != nil {
		t.Fatalf("newExtensionBridge() error = %v", err)
	}

	var nilCtx context.Context
	_, err = bridge.StartRun(nilCtx, &model.RuntimeConfig{
		WorkspaceRoot: env.workspaceRoot,
		Mode:          model.ExecutionModeExec,
		PromptText:    "nested exec prompt",
		ParentRunID:   "parent-run-001",
	})
	if err == nil || !strings.Contains(err.Error(), "context is required") {
		t.Fatalf("StartRun(nil context) error = %v, want context requirement", err)
	}
}

func TestExtensionBridgeStartRunRejectsDifferentWorkspaceRoot(t *testing.T) {
	env := newRunManagerTestEnv(t, runManagerTestDeps{})

	otherWorkspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(otherWorkspace, model.WorkflowRootDirName), 0o755); err != nil {
		t.Fatalf("MkdirAll(other workspace marker) error = %v", err)
	}

	bridge, err := newExtensionBridge(env.manager, env.workspaceRoot)
	if err != nil {
		t.Fatalf("newExtensionBridge() error = %v", err)
	}

	_, err = bridge.StartRun(context.Background(), &model.RuntimeConfig{
		WorkspaceRoot: otherWorkspace,
		Mode:          model.ExecutionModeExec,
		PromptText:    "nested exec prompt",
		ParentRunID:   "parent-run-001",
	})
	if err == nil {
		t.Fatal("StartRun(different workspace) error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "workspace_root") {
		t.Fatalf("StartRun(different workspace) error = %v, want workspace_root context", err)
	}
}

func TestExtensionBridgeStartRunCreatesDetachedTaskRun(t *testing.T) {
	env := newRunManagerTestEnv(t, runManagerTestDeps{})

	bridge, err := newExtensionBridge(env.manager, env.workspaceRoot)
	if err != nil {
		t.Fatalf("newExtensionBridge() error = %v", err)
	}

	handle, err := bridge.StartRun(context.Background(), &model.RuntimeConfig{
		WorkspaceRoot: env.workspaceRoot,
		Name:          env.workflowSlug,
		Mode:          model.ExecutionModePRDTasks,
		ParentRunID:   "parent-task-run-001",
	})
	if err != nil {
		t.Fatalf("StartRun(task) error = %v", err)
	}

	row := waitForRun(t, env.globalDB, handle.RunID, func(row globaldb.Run) bool {
		return isTerminalRunStatus(row.Status)
	})
	if row.Mode != runModeTask {
		t.Fatalf("row.Mode = %q, want %q", row.Mode, runModeTask)
	}
	if row.PresentationMode != daemonExtensionPresentationMode {
		t.Fatalf("row.PresentationMode = %q, want %q", row.PresentationMode, daemonExtensionPresentationMode)
	}
	if row.ParentRunID != "parent-task-run-001" {
		t.Fatalf("row.ParentRunID = %q, want %q", row.ParentRunID, "parent-task-run-001")
	}
}

func TestExtensionBridgeTaskGroupRunsResolveChildScope(t *testing.T) {
	// INVARIANT: extension-created task group runs retain the selected child
	// identity and ignore caller-provided sibling operational directories.
	// OWNING_LAYER: service-integration. CONTRACT: IT-019, IT-024.
	env := newRunManagerTestEnv(t, runManagerTestDeps{})
	initiative := "watcher"
	taskGroupRef := initiative + "/TG-001"
	taskGroupDir := filepath.Join(env.workflowDir(initiative), "_task_groups", "TG-001")
	siblingDir := filepath.Join(env.workflowDir(initiative), "_task_groups", "TG-002")
	env.writeWorkflowFile(t, initiative, "_prd.md", "# Canonical PRD\n")
	env.writeWorkflowFile(t, initiative, "_techspec.md", "# Canonical TechSpec\n")
	env.writeWorkflowFile(t, initiative, "_task_groups.md", daemonTaskGroupPlan(" "))
	env.writeWorkflowFile(
		t,
		initiative,
		filepath.Join("_task_groups", "TG-001", "task_01.md"),
		daemonTaskBody("pending", "Task Group task"),
	)
	env.writeWorkflowFile(
		t,
		initiative,
		filepath.Join("_task_groups", "TG-001", "reviews-001", "issue_001.md"),
		daemonReviewIssueBody("pending", "high"),
	)
	env.writeWorkflowFile(
		t,
		initiative,
		filepath.Join("_task_groups", "TG-002", "task_01.md"),
		"sibling artifact must not be read\n",
	)
	env.writeWorkflowFile(
		t,
		initiative,
		filepath.Join("_task_groups", "TG-002", "reviews-001", "issue_001.md"),
		"sibling review must not be read\n",
	)

	bridge, err := newExtensionBridge(env.manager, env.workspaceRoot)
	if err != nil {
		t.Fatalf("newExtensionBridge() error = %v", err)
	}
	taskHandle, err := bridge.StartRun(context.Background(), &model.RuntimeConfig{
		WorkspaceRoot: env.workspaceRoot,
		Name:          taskGroupRef,
		TasksDir:      siblingDir,
		Mode:          model.ExecutionModePRDTasks,
		ParentRunID:   "parent-task-group-task",
	})
	if err != nil {
		t.Fatalf("StartRun(task group task) error = %v", err)
	}
	taskRun := waitForRun(t, env.globalDB, taskHandle.RunID, func(row globaldb.Run) bool {
		return isTerminalRunStatus(row.Status)
	})

	workspace, err := env.globalDB.ResolveOrRegister(context.Background(), env.workspaceRoot)
	if err != nil {
		t.Fatalf("ResolveOrRegister() error = %v", err)
	}
	child, err := env.globalDB.GetActiveWorkflowBySlug(context.Background(), workspace.ID, taskGroupRef)
	if err != nil {
		t.Fatalf("GetActiveWorkflowBySlug(task group): %v", err)
	}
	if taskRun.WorkflowID == nil || *taskRun.WorkflowID != child.ID {
		t.Fatalf("IT-019 task WorkflowID = %v, want child %q", taskRun.WorkflowID, child.ID)
	}

	reviewHandle, err := bridge.StartRun(context.Background(), &model.RuntimeConfig{
		WorkspaceRoot: env.workspaceRoot,
		Name:          taskGroupRef,
		Round:         1,
		ReviewsDir:    filepath.Join(siblingDir, "reviews-001"),
		Mode:          model.ExecutionModePRReview,
		ParentRunID:   "parent-task-group-review",
	})
	if err != nil {
		t.Fatalf("StartRun(task group review) error = %v", err)
	}
	reviewRun := waitForRun(t, env.globalDB, reviewHandle.RunID, func(row globaldb.Run) bool {
		return isTerminalRunStatus(row.Status)
	})
	if reviewRun.WorkflowID == nil || *reviewRun.WorkflowID != child.ID {
		t.Fatalf("IT-022 review WorkflowID = %v, want child %q", reviewRun.WorkflowID, child.ID)
	}
	if _, err := os.Stat(filepath.Join(taskGroupDir, "reviews-001", "issue_001.md")); err != nil {
		t.Fatalf("selected task group review artifact was not retained: %v", err)
	}
}

func TestExtensionBridgeStartRunCreatesDetachedReviewRun(t *testing.T) {
	env := newRunManagerTestEnv(t, runManagerTestDeps{})
	env.createReviewRound(t, 1)

	bridge, err := newExtensionBridge(env.manager, env.workspaceRoot)
	if err != nil {
		t.Fatalf("newExtensionBridge() error = %v", err)
	}

	handle, err := bridge.StartRun(context.Background(), &model.RuntimeConfig{
		WorkspaceRoot: env.workspaceRoot,
		Name:          env.workflowSlug,
		Round:         1,
		Mode:          model.ExecutionModePRReview,
		ParentRunID:   "parent-review-run-001",
	})
	if err != nil {
		t.Fatalf("StartRun(review) error = %v", err)
	}

	row := waitForRun(t, env.globalDB, handle.RunID, func(row globaldb.Run) bool {
		return isTerminalRunStatus(row.Status)
	})
	if row.Mode != runModeReview {
		t.Fatalf("row.Mode = %q, want %q", row.Mode, runModeReview)
	}
	if row.PresentationMode != daemonExtensionPresentationMode {
		t.Fatalf("row.PresentationMode = %q, want %q", row.PresentationMode, daemonExtensionPresentationMode)
	}
	if row.ParentRunID != "parent-review-run-001" {
		t.Fatalf("row.ParentRunID = %q, want %q", row.ParentRunID, "parent-review-run-001")
	}
}

func TestRunManagerExecRunCompletesAndReplaysPersistedStream(t *testing.T) {
	executed := make(chan string, 1)
	env := newRunManagerTestEnv(t, runManagerTestDeps{
		executeExec: func(ctx context.Context, cfg *model.RuntimeConfig, scope model.RunScope) error {
			runArtifacts, err := model.ResolveRuntimeRunArtifacts(cfg)
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

func TestDaemonWorkflowPreparedRunRestartFailedTreatsEmptyFailedJobIDsAsRestartAll(t *testing.T) {
	t.Parallel()

	t.Run("Should restart all prepared jobs when failed job IDs are empty", func(t *testing.T) {
		var preparedJobNames []string
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			prepare: func(_ context.Context, _ *model.RuntimeConfig, scope model.RunScope) (*model.SolvePreparation, error) {
				return &model.SolvePreparation{
					Jobs: []model.Job{
						{SafeName: "task_01"},
						{SafeName: "task_02"},
					},
					RunArtifacts: scope.RunArtifacts(),
				}, nil
			},
			execute: func(_ context.Context, prep *model.SolvePreparation, cfg *model.RuntimeConfig) error {
				preparedJobNames = make([]string, 0, len(prep.Jobs))
				for _, job := range prep.Jobs {
					preparedJobNames = append(preparedJobNames, job.SafeName)
				}
				payload, err := json.Marshal(struct {
					SchemaVersion int                   `json:"schema_version"`
					RunID         string                `json:"run_id"`
					Status        recovery.RunStatus    `json:"status"`
					ArtifactsDir  string                `json:"artifacts_dir"`
					ResultPath    string                `json:"result_path,omitempty"`
					Jobs          []recovery.JobOutcome `json:"jobs"`
				}{
					SchemaVersion: recovery.ResultSchemaVersion,
					RunID:         cfg.RunID,
					Status:        recovery.StatusSucceeded,
					ArtifactsDir:  prep.RunArtifacts.RunDir,
					ResultPath:    prep.RunArtifacts.ResultPath,
					Jobs: []recovery.JobOutcome{
						{SafeName: "task_01", Status: recovery.StatusSucceeded, ExitCode: 0},
						{SafeName: "task_02", Status: recovery.StatusSucceeded, ExitCode: 0},
					},
				})
				if err != nil {
					return err
				}
				if err := os.MkdirAll(filepath.Dir(prep.RunArtifacts.ResultPath), 0o755); err != nil {
					return err
				}
				return os.WriteFile(prep.RunArtifacts.ResultPath, payload, 0o600)
			},
		})

		cfg := (&model.RuntimeConfig{
			RunID:         "restart-all-prepared-jobs",
			WorkspaceRoot: env.workspaceRoot,
			TasksDir:      model.TaskDirectoryForWorkspace(env.workspaceRoot, env.workflowSlug),
			Mode:          model.ExecutionModePRDTasks,
			AccessMode:    model.AccessModeDefault,
		}).Clone()
		cfg.ApplyDefaults()

		scope, err := newTestOpenRunScope(nil)(context.Background(), cfg, model.OpenRunScopeOptions{})
		if err != nil {
			t.Fatalf("open test scope: %v", err)
		}
		defer func() {
			_ = scope.Close(context.Background())
		}()

		outcome, err := newDaemonWorkflowPreparedRun(env.manager, cfg, scope).
			RestartFailed(context.Background(), []string{})
		if err != nil {
			t.Fatalf("RestartFailed() error = %v", err)
		}
		if !slices.Equal(preparedJobNames, []string{"task_01", "task_02"}) {
			t.Fatalf("prepared jobs = %#v, want all jobs", preparedJobNames)
		}
		if len(outcome.Jobs) != 2 {
			t.Fatalf("outcome jobs = %#v, want both prepared jobs", outcome.Jobs)
		}
	})
}

func TestRunManagerHelperOverridesAndUtilities(t *testing.T) {
	t.Run("Should merge daemon recovery overrides with project defaults", func(t *testing.T) {
		cfg, err := resolveDaemonRecoveryConfig(workspacecfg.ProjectConfig{
			Recovery: workspacecfg.AgentRecoveryConfig{
				Enabled:         boolPtr(false),
				IDE:             stringPtr("claude"),
				Model:           stringPtr("sonnet"),
				ReasoningEffort: stringPtr("medium"),
				MaxAttempts:     intPtr(1),
			},
		}, runtimeOverrideInput{
			Recovery: &workspacecfg.AgentRecoveryConfig{
				Enabled:         boolPtr(true),
				Model:           stringPtr("gpt-5.5"),
				ReasoningEffort: stringPtr("high"),
				MaxAttempts:     intPtr(2),
			},
		})
		if err != nil {
			t.Fatalf("resolveDaemonRecoveryConfig() error = %v", err)
		}
		if cfg.Enabled == nil || !*cfg.Enabled ||
			cfg.IDE == nil || *cfg.IDE != "claude" ||
			cfg.Model == nil || *cfg.Model != "gpt-5.5" ||
			cfg.ReasoningEffort == nil || *cfg.ReasoningEffort != "high" ||
			cfg.MaxAttempts == nil || *cfg.MaxAttempts != 2 {
			t.Fatalf("unexpected recovery config: %#v", cfg)
		}
	})

	t.Run("Should parse snake_case parallel task runtime overrides from JSON", func(t *testing.T) {
		overrides, err := parseRuntimeOverrides(rawJSON(
			t,
			`{"parallel_tasks":{"enabled":true,"max_concurrency":3,"conflict_resolver":{"model":"gpt-5.5","reasoning_effort":"high","max_attempts":2,"validation_command":["go","test","./..."]}}}`,
		))
		if err != nil {
			t.Fatalf("parseRuntimeOverrides() error = %v", err)
		}
		cfg, err := resolveDaemonParallelTasksConfig(workspacecfg.ProjectConfig{}, overrides)
		if err != nil {
			t.Fatalf("resolveDaemonParallelTasksConfig() error = %v", err)
		}
		if cfg.Enabled == nil || !*cfg.Enabled {
			t.Fatalf("parallel enabled = %#v, want true", cfg.Enabled)
		}
		if cfg.MaxConcurrency == nil || *cfg.MaxConcurrency != 3 {
			t.Fatalf("max concurrency = %#v, want 3", cfg.MaxConcurrency)
		}
		if cfg.ConflictResolver == nil ||
			cfg.ConflictResolver.Model == nil || *cfg.ConflictResolver.Model != "gpt-5.5" ||
			cfg.ConflictResolver.ReasoningEffort == nil || *cfg.ConflictResolver.ReasoningEffort != "high" ||
			cfg.ConflictResolver.MaxAttempts == nil || *cfg.ConflictResolver.MaxAttempts != 2 ||
			cfg.ConflictResolver.ValidationCommand == nil ||
			strings.Join(*cfg.ConflictResolver.ValidationCommand, " ") != "go test ./..." {
			t.Fatalf("parallel conflict resolver = %#v", cfg.ConflictResolver)
		}
	})

	t.Run("Should parse and apply snake_case stall runtime overrides from JSON", func(t *testing.T) {
		overrides, err := parseRuntimeOverrides(rawJSON(
			t,
			`{"stall":{"enabled":false,"timeout":"200ms","child_timeout":"500ms","terminal_command_timeout":"1s","retries":2}}`,
		))
		if err != nil {
			t.Fatalf("parseRuntimeOverrides() error = %v", err)
		}

		cfg := &model.RuntimeConfig{}
		if err := applyRuntimeOverrideInput(cfg, overrides); err != nil {
			t.Fatalf("applyRuntimeOverrideInput() error = %v", err)
		}
		if cfg.StallEnabled == nil || *cfg.StallEnabled {
			t.Fatalf("stall enabled = %#v, want false", cfg.StallEnabled)
		}
		if cfg.StallTimeout != 200*time.Millisecond {
			t.Fatalf("stall timeout = %v, want 200ms", cfg.StallTimeout)
		}
		if cfg.ChildStallTimeout != 500*time.Millisecond {
			t.Fatalf("child stall timeout = %v, want 500ms", cfg.ChildStallTimeout)
		}
		if cfg.TerminalCommandTimeout != time.Second {
			t.Fatalf("terminal command timeout = %v, want 1s", cfg.TerminalCommandTimeout)
		}
		if cfg.StallRetries == nil || *cfg.StallRetries != 2 {
			t.Fatalf("stall retries = %#v, want 2", cfg.StallRetries)
		}
	})

	t.Run("Should reject unknown snake_case stall runtime overrides", func(t *testing.T) {
		_, err := parseRuntimeOverrides(rawJSON(t, `{"stall":{"unknown":true}}`))
		if err == nil {
			t.Fatal("parseRuntimeOverrides() error = nil, want non-nil")
		}
		assertProblemStatus(t, err, http.StatusUnprocessableEntity)
	})

	t.Run("Should apply runtime and project overrides in order", func(t *testing.T) {
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
		applyTaskProjectConfig(cfg, workspacecfg.DefaultsConfig{
			ByComplexity: workspacecfg.TaskRuntimeByComplexityConfig{
				Low: workspacecfg.TaskRuntimeOverrides{ReasoningEffort: stringPtr("low")},
			},
		}, workspacecfg.TaskRunConfig{
			IncludeCompleted: boolPtr(true),
			OutputFormat:     stringPtr(string(model.OutputFormatRawJSON)),
			TaskRuntimeRules: &rules,
		})
		if err := applyReviewProjectConfig(cfg, workspacecfg.FixReviewsConfig{
			Concurrent:      intPtr(4),
			BatchSize:       intPtr(2),
			IncludeResolved: boolPtr(true),
			OutputFormat:    stringPtr(string(model.OutputFormatJSON)),
			Stall: workspacecfg.StallOverrides{
				Timeout:                stringPtr("45s"),
				TerminalCommandTimeout: stringPtr("8m"),
				Retries:                intPtr(4),
			},
		}); err != nil {
			t.Fatalf("applyReviewProjectConfig() error = %v", err)
		}
		if cfg.StallTimeout != 45*time.Second || cfg.TerminalCommandTimeout != 8*time.Minute {
			t.Fatalf(
				"review stall durations = %v / %v, want 45s / 8m",
				cfg.StallTimeout,
				cfg.TerminalCommandTimeout,
			)
		}
		if cfg.StallRetries == nil || *cfg.StallRetries != 4 {
			t.Fatalf("review stall retries = %#v, want 4", cfg.StallRetries)
		}
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
			RunID:     stringPtr("override-run"),
			IDE:       stringPtr("codex"),
			Model:     stringPtr("gpt-5-mini"),
			AgentName: stringPtr("council"),
			ExplicitRuntime: &model.ExplicitRuntimeFlags{
				Model:           true,
				ReasoningEffort: true,
				AccessMode:      true,
			},
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

		if cfg.RunID != "override-run" || cfg.IDE != "codex" || cfg.Model != "gpt-5-mini" ||
			cfg.AgentName != "council" {
			t.Fatalf("runtime override application failed: %#v", cfg)
		}
		if !cfg.ExplicitRuntime.Model || !cfg.ExplicitRuntime.ReasoningEffort || !cfg.ExplicitRuntime.AccessMode {
			t.Fatalf("explicit runtime flags were not preserved: %#v", cfg.ExplicitRuntime)
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
		if len(cfg.TaskRuntimeRules) != 2 || cfg.TaskRuntimeRules[0].Complexity == nil ||
			*cfg.TaskRuntimeRules[0].Complexity != "low" || cfg.TaskRuntimeRules[1].Type == nil ||
			*cfg.TaskRuntimeRules[1].Type != "backend" {
			t.Fatalf("task runtime rules = %#v, want complexity then cloned backend rule", cfg.TaskRuntimeRules)
		}
	})

	t.Run("Should apply duration and override error helpers", func(t *testing.T) {
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

	t.Run("Should apply stall overrides with per-run precedence over project", func(t *testing.T) {
		cfg := &model.RuntimeConfig{}
		if err := applyRuntimeOverridesFromProject(cfg, workspacecfg.RuntimeOverrides{
			Stall: workspacecfg.StallOverrides{
				Enabled:      boolPtr(true),
				Timeout:      stringPtr("2m"),
				ChildTimeout: stringPtr("5m"),
				Retries:      intPtr(3),
			},
		}, "defaults"); err != nil {
			t.Fatalf("applyRuntimeOverridesFromProject() error = %v", err)
		}
		if cfg.StallTimeout != 2*time.Minute {
			t.Fatalf("project stall timeout = %v, want 2m", cfg.StallTimeout)
		}
		if err := applyRuntimeOverrideInput(cfg, runtimeOverrideInput{
			Stall: &workspacecfg.StallOverrides{
				Timeout:                stringPtr("5m"),
				TerminalCommandTimeout: stringPtr("20m"),
				Enabled:                boolPtr(false),
			},
		}); err != nil {
			t.Fatalf("applyRuntimeOverrideInput() error = %v", err)
		}
		cfg.ApplyDefaults()

		policy := cfg.StallPolicy()
		if policy.IdleTimeout != 5*time.Minute {
			t.Fatalf("per-run stall timeout precedence failed: %v, want 5m", policy.IdleTimeout)
		}
		if policy.Enabled {
			t.Fatal("per-run stall disable did not take precedence")
		}
		if policy.TerminalCap != 20*time.Minute {
			t.Fatalf("terminal cap = %v, want 20m", policy.TerminalCap)
		}
		if policy.Retries != 3 {
			t.Fatalf("project retries not retained: %d, want 3", policy.Retries)
		}
		if policy.ChildTimeout <= policy.IdleTimeout {
			t.Fatalf("child %v must exceed idle %v after correction", policy.ChildTimeout, policy.IdleTimeout)
		}
	})

	t.Run("Should surface an invalid stall duration parse error", func(t *testing.T) {
		cfg := &model.RuntimeConfig{}
		err := applyRuntimeOverridesFromProject(cfg, workspacecfg.RuntimeOverrides{
			Stall: workspacecfg.StallOverrides{Timeout: stringPtr("not-a-duration")},
		}, "defaults")
		if err == nil {
			t.Fatal("applyRuntimeOverridesFromProject(invalid stall.timeout) error = nil, want non-nil")
		}
		assertProblemStatus(t, err, 422)

		perRunErr := applyRuntimeOverrideInput(cfg, runtimeOverrideInput{
			Stall: &workspacecfg.StallOverrides{ChildTimeout: stringPtr("bogus")},
		})
		if perRunErr == nil {
			t.Fatal("applyRuntimeOverrideInput(invalid stall.child_timeout) error = nil, want non-nil")
		}
		assertProblemStatus(t, perRunErr, 422)

		terminalErr := applyRuntimeOverridesFromProject(cfg, workspacecfg.RuntimeOverrides{
			Stall: workspacecfg.StallOverrides{TerminalCommandTimeout: stringPtr("nope")},
		}, "defaults")
		if terminalErr == nil {
			t.Fatal(
				"applyRuntimeOverridesFromProject(invalid stall.terminal_command_timeout) error = nil, want non-nil",
			)
		}
		assertProblemStatus(t, terminalErr, 422)

		reviewErr := applyReviewProjectConfig(cfg, workspacecfg.FixReviewsConfig{
			Stall: workspacecfg.StallOverrides{Timeout: stringPtr("invalid-review-timeout")},
		})
		if reviewErr == nil {
			t.Fatal("applyReviewProjectConfig(invalid stall.timeout) error = nil, want non-nil")
		}
		assertProblemStatus(t, reviewErr, 422)
	})

	t.Run("Should resolve the parked sound override", func(t *testing.T) {
		cfg := &model.RuntimeConfig{}
		applySoundConfig(cfg, workspacecfg.SoundConfig{OnParked: stringPtr("  ping  ")})
		if cfg.SoundOnParked != "ping" {
			t.Fatalf("parked sound override = %q, want trimmed ping", cfg.SoundOnParked)
		}
	})

	t.Run("Should resolve a loaded stall config through the daemon apply path", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv(compozyconfig.HomeEnvVar, "")
		root := t.TempDir()
		configDir := filepath.Join(root, ".compozy")
		if err := os.MkdirAll(configDir, 0o755); err != nil {
			t.Fatalf("mkdir config dir: %v", err)
		}
		content := "[defaults.stall]\n" +
			"timeout = \"2m\"\n" +
			"child_timeout = \"9m\"\n" +
			"terminal_command_timeout = \"30m\"\n" +
			"retries = 2\n\n" +
			"[fix_reviews.stall]\n" +
			"timeout = \"45s\"\n" +
			"terminal_command_timeout = \"8m\"\n" +
			"retries = 4\n\n" +
			"[sound]\n" +
			"on_parked = \"ping\"\n"
		if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(content), 0o600); err != nil {
			t.Fatalf("write config: %v", err)
		}

		projectCfg, _, err := workspacecfg.LoadConfig(context.Background(), root)
		if err != nil {
			t.Fatalf("LoadConfig() error = %v", err)
		}

		cfg := &model.RuntimeConfig{}
		applySoundConfig(cfg, projectCfg.Sound)
		if err := applyRuntimeOverridesFromProject(
			cfg,
			projectCfg.Defaults.RuntimeOverrides,
			"defaults",
		); err != nil {
			t.Fatalf("applyRuntimeOverridesFromProject() error = %v", err)
		}
		if err := applyReviewProjectConfig(cfg, projectCfg.FixReviews); err != nil {
			t.Fatalf("applyReviewProjectConfig() error = %v", err)
		}
		cfg.ApplyDefaults()

		policy := cfg.StallPolicy()
		if !policy.Enabled {
			t.Fatalf("expected enabled-by-default when unset, got %#v", policy)
		}
		if policy.IdleTimeout != 45*time.Second || policy.ChildTimeout != 9*time.Minute {
			t.Fatalf("resolved timeouts = %v / %v, want 45s / 9m", policy.IdleTimeout, policy.ChildTimeout)
		}
		if policy.TerminalCap != 8*time.Minute || policy.Retries != 4 {
			t.Fatalf("resolved cap/retries = %v / %d, want 8m / 4", policy.TerminalCap, policy.Retries)
		}
		if cfg.SoundOnParked != "ping" {
			t.Fatalf("resolved parked sound = %q, want ping", cfg.SoundOnParked)
		}
	})

	t.Run("Should detach context and normalize raw helper values", func(t *testing.T) {
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

	t.Run("Should enforce filesystem helper behavior", func(t *testing.T) {
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
	workspaceByID, workflowIDByID, _, err := env.manager.resolveWorkflowContext(
		context.Background(),
		workspace.ID,
		env.workflowSlug,
	)
	if err != nil {
		t.Fatalf("resolveWorkflowContext(workspace id) error = %v", err)
	}
	if workspaceByID.ID != workspace.ID {
		t.Fatalf("workspaceByID.ID = %q, want %q", workspaceByID.ID, workspace.ID)
	}
	if workflowIDByID == nil || *workflowIDByID != *firstID {
		t.Fatalf("workflowIDByID = %v, want %v", workflowIDByID, firstID)
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

func TestResolveTerminalStatePersistsFallbackAfterJournalWriterCloses(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	cfg := (&model.RuntimeConfig{
		RunID: "terminal-run-closed-journal",
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
	if err := scope.RunJournal().Close(context.Background()); err != nil {
		t.Fatalf("close journal writer: %v", err)
	}

	prepareErr := errors.New("prepare failed before run.started")
	fallback := failedTerminalState(scope.RunArtifacts(), prepareErr)
	terminal, err := resolveTerminalState(context.Background(), cfg.RunID, fallback, scope)
	if err != nil {
		t.Fatalf("resolveTerminalState() error = %v", err)
	}
	if terminal.status != runStatusFailed {
		t.Fatalf("terminal.status = %q, want %q", terminal.status, runStatusFailed)
	}
	if !strings.Contains(terminal.errorText, prepareErr.Error()) {
		t.Fatalf("terminal.errorText = %q, want %q", terminal.errorText, prepareErr)
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
			runArtifactSyncCount(t, env.manager, run.RunID) >= 1
	})

	if err := env.manager.Cancel(context.Background(), run.RunID); err != nil {
		t.Fatalf("Cancel(%q) error = %v", run.RunID, err)
	}
	waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusCancelled
	})

	rowsBeforeStop := runArtifactSyncCount(t, env.manager, run.RunID)
	checkpointAfterStop := queryWorkflowCheckpointChecksum(t, env.paths.GlobalDBPath, env.workflowSlug)
	env.writeWorkflowFile(t, env.workflowSlug, "task_01.md", daemonTaskBody("pending", "post-cancel change"))
	time.Sleep(200 * time.Millisecond)

	if got := runArtifactSyncCount(t, env.manager, run.RunID); got != rowsBeforeStop {
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
	workspace, err := env.globalDB.ResolveOrRegister(context.Background(), env.workspaceRoot)
	if err != nil {
		t.Fatalf("ResolveOrRegister(%q) error = %v", env.workspaceRoot, err)
	}
	if _, err := corepkg.SyncWithDB(context.Background(), env.globalDB, workspace, corepkg.SyncConfig{
		TasksDir: env.workflowDir(otherSlug),
	}); err != nil {
		t.Fatalf("SyncWithDB(other workflow) error = %v", err)
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
			runArtifactSyncCount(t, env.manager, run.RunID) >= 6
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

func TestRunManagerTaskGroupLifecycleUsesChildScopeForTaskAndReviewPreparation(t *testing.T) {
	// INVARIANT: task group task and review preparation retain the public child
	// workflow identity while all mutable inputs resolve under that task group.
	// OWNING_LAYER: service-integration. EXISTING_SUITE: internal/daemon/run_manager_test.go.
	env := newRunManagerTestEnv(t, runManagerTestDeps{})
	initiative := "watcher"
	taskGroupRef := initiative + "/TG-001"
	taskGroupDir := filepath.Join(env.workflowDir(initiative), "_task_groups", "TG-001")
	env.writeWorkflowFile(t, initiative, "_prd.md", "# Canonical PRD\n")
	env.writeWorkflowFile(t, initiative, "_techspec.md", "# Canonical TechSpec\n")
	env.writeWorkflowFile(t, initiative, "_task_groups.md", daemonTaskGroupPlan(" "))
	env.writeWorkflowFile(
		t,
		initiative,
		filepath.Join("_task_groups", "TG-001", "task_01.md"),
		daemonTaskBody("pending", "Task Group task"),
	)
	env.writeWorkflowFile(
		t,
		initiative,
		filepath.Join("_task_groups", "TG-002", "task_01.md"),
		"sibling artifact must not be read\n",
	)
	env.writeWorkflowFile(
		t,
		initiative,
		filepath.Join("_task_groups", "TG-001", "reviews-001", "issue_001.md"),
		daemonReviewIssueBody("pending", "high"),
	)

	_, workflowID, taskCfg, _, _, _, err := env.manager.prepareTaskStart(
		context.Background(),
		env.workspaceRoot,
		taskGroupRef,
		apicore.TaskRunRequest{Workspace: env.workspaceRoot},
	)
	if err != nil {
		t.Fatalf("prepareTaskStart(task group): %v", err)
	}
	canonicalTaskGroupDir, err := filepath.EvalSymlinks(taskGroupDir)
	if err != nil {
		t.Fatalf("EvalSymlinks(task group): %v", err)
	}
	if workflowID == nil || taskCfg.ExecutionScope == nil || taskCfg.Name != taskGroupRef ||
		taskCfg.WorkflowName != taskGroupRef ||
		taskCfg.TasksDir != taskCfg.ExecutionScope.TasksDir ||
		taskCfg.ExecutionScope.OperationalDir != canonicalTaskGroupDir {
		t.Fatalf("IT-017/IT-019 task task group config = %#v workflowID=%v", taskCfg, workflowID)
	}
	workspace, err := env.globalDB.ResolveOrRegister(context.Background(), env.workspaceRoot)
	if err != nil {
		t.Fatalf("ResolveOrRegister(workspace): %v", err)
	}
	workflow, err := env.globalDB.GetActiveWorkflowBySlug(context.Background(), workspace.ID, taskGroupRef)
	if err != nil {
		t.Fatalf("GetActiveWorkflowBySlug(task group): %v", err)
	}
	if workflow.ID != *workflowID || workflow.Kind != globaldb.WorkflowKindTaskGroup {
		t.Fatalf("child workflow = %#v, want task group child %q", workflow, *workflowID)
	}

	_, reviewWorkflowID, reviewCfg, _, _, err := env.manager.prepareReviewStart(
		context.Background(),
		env.workspaceRoot,
		taskGroupRef,
		1,
		apicore.ReviewRunRequest{Workspace: env.workspaceRoot},
	)
	if err != nil {
		t.Fatalf("prepareReviewStart(task group): %v", err)
	}
	if reviewWorkflowID == nil || *reviewWorkflowID != *workflowID || reviewCfg.ExecutionScope == nil ||
		reviewCfg.ReviewsDir != reviewCfg.ExecutionScope.ReviewDir(1) {
		t.Fatalf("IT-022/IT-041 review task group config = %#v workflowID=%v", reviewCfg, reviewWorkflowID)
	}
	if err := os.Remove(filepath.Join(env.workflowDir(initiative), "_techspec.md")); err != nil {
		t.Fatalf("remove canonical techspec: %v", err)
	}
	if _, _, _, _, _, _, err := env.manager.prepareTaskStart(
		context.Background(),
		env.workspaceRoot,
		taskGroupRef,
		apicore.TaskRunRequest{Workspace: env.workspaceRoot},
	); err == nil || !strings.Contains(err.Error(), "_techspec.md") {
		t.Fatalf("IT-038 prepareTaskStart() error = %v, want inaccessible canonical techspec", err)
	}
}

// IT-015, IT-016, IT-033, IT-034, IT-035 and IT-036: the daemon owns the
// final task group readiness decision and persists why a one-run override was
// requested and whether it was actually required.
func TestRunManagerTaskRunPreflightUsesCurrentTaskGroupReadinessAndRecordsOverride(t *testing.T) {
	env := newRunManagerTestEnv(t, runManagerTestDeps{
		prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
			return &model.SolvePreparation{}, nil
		},
	})
	initiative := "watcher"
	taskGroupRef := initiative + "/TG-002"
	writeDaemonDependentTaskGroupFixture(t, env, initiative, false)

	_, err := env.manager.StartTaskRun(
		context.Background(),
		env.workspaceRoot,
		initiative,
		apicore.TaskRunRequest{
			Workspace:        env.workspaceRoot,
			TaskGroupID:      "TG-002",
			PresentationMode: defaultPresentationMode,
			RuntimeOverrides: rawJSON(t, `{"run_id":"task-group-blocked"}`),
		},
	)
	var problem *apicore.Problem
	if !errors.As(err, &problem) || problem.Status != http.StatusConflict ||
		problem.Code != "task_group_dependencies_unmet" {
		t.Fatalf("blocked task group error = %#v (%v), want 409 dependency problem", problem, err)
	}
	if _, err := env.globalDB.GetRun(
		context.Background(),
		"task-group-blocked",
	); !errors.Is(
		err,
		globaldb.ErrRunNotFound,
	) {
		t.Fatalf("GetRun(task-group-blocked) error = %v, want no durable run", err)
	}

	run, err := env.manager.StartTaskRun(
		context.Background(),
		env.workspaceRoot,
		initiative,
		apicore.TaskRunRequest{
			Workspace:        env.workspaceRoot,
			TaskGroupID:      "TG-002",
			AllowOutOfOrder:  true,
			PresentationMode: defaultPresentationMode,
			RuntimeOverrides: rawJSON(t, `{"run_id":"task-group-override-needed"}`),
		},
	)
	if err != nil {
		t.Fatalf("StartTaskRun(override) error = %v", err)
	}
	_ = waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return isTerminalRunStatus(row.Status)
	})
	row, err := env.globalDB.GetRun(context.Background(), run.RunID)
	if err != nil {
		t.Fatalf("GetRun(%q) error = %v", run.RunID, err)
	}
	if !row.OutOfOrderRequested || !row.OutOfOrderNeeded {
		t.Fatalf(
			"override metadata = requested:%t needed:%t, want both true",
			row.OutOfOrderRequested,
			row.OutOfOrderNeeded,
		)
	}

	// Change the source plan after the prior run. A new start must re-resolve
	// this state rather than reuse the earlier blocked readiness result.
	writeDaemonDependentTaskGroupFixture(t, env, initiative, true)
	run, err = env.manager.StartTaskRun(
		context.Background(),
		env.workspaceRoot,
		initiative,
		apicore.TaskRunRequest{
			Workspace:        env.workspaceRoot,
			TaskGroupID:      "TG-002",
			AllowOutOfOrder:  true,
			PresentationMode: defaultPresentationMode,
			RuntimeOverrides: rawJSON(t, `{"run_id":"task-group-override-unneeded"}`),
		},
	)
	if err != nil {
		t.Fatalf("StartTaskRun(current plan) error = %v", err)
	}
	_ = waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return isTerminalRunStatus(row.Status)
	})
	row, err = env.globalDB.GetRun(context.Background(), run.RunID)
	if err != nil {
		t.Fatalf("GetRun(%q) error = %v", run.RunID, err)
	}
	if !row.OutOfOrderRequested || row.OutOfOrderNeeded {
		t.Fatalf(
			"current-plan metadata = requested:%t needed:%t, want true/false",
			row.OutOfOrderRequested,
			row.OutOfOrderNeeded,
		)
	}
	if run.WorkflowSlug != taskGroupRef {
		t.Fatalf("run workflow = %q, want %q", run.WorkflowSlug, taskGroupRef)
	}
}

func writeDaemonDependentTaskGroupFixture(
	t *testing.T,
	env *runManagerTestEnv,
	initiative string,
	firstTaskGroupCompleted bool,
) {
	t.Helper()
	plan, err := taskgroups.RenderPlan(taskgroups.Plan{
		SchemaVersion: taskgroups.SchemaVersion,
		Initiative:    initiative,
		TaskGroups: []taskgroups.TaskGroup{
			{
				ID:         "TG-001",
				Title:      "Foundation",
				Outcome:    "Provide the prerequisite",
				Directory:  "_task_groups/TG-001",
				Completed:  firstTaskGroupCompleted,
				OwnedScope: []string{"foundation"},
			},
			{
				ID:         "TG-002",
				Title:      "Delivery",
				Outcome:    "Use the prerequisite",
				Directory:  "_task_groups/TG-002",
				OwnedScope: []string{"delivery"},
			},
		},
		Edges: []taskgroups.Dependency{{
			From:      "TG-001",
			To:        "TG-002",
			Rationale: "Foundation must be complete first",
		}},
	})
	if err != nil {
		t.Fatalf("RenderPlan() error = %v", err)
	}
	env.writeWorkflowFile(t, initiative, "_prd.md", "# Canonical PRD\n")
	env.writeWorkflowFile(t, initiative, "_techspec.md", "# Canonical TechSpec\n")
	env.writeWorkflowFile(t, initiative, "_task_groups.md", string(plan))
	env.writeWorkflowFile(
		t,
		initiative,
		filepath.Join("_task_groups", "TG-002", "task_01.md"),
		daemonTaskBody("pending", "Task Group delivery task"),
	)
}

func TestRunManagerRejectsEscapedTaskGroupManifestBeforeTaskStarts(t *testing.T) {
	// INVARIANT: task group task starts never accept graph nodes that resolve outside the selected task group.
	// OWNING_LAYER: service-integration. EXISTING_SUITE: internal/daemon/run_manager_test.go.
	newEscapedTaskGroup := func(t *testing.T) (*runManagerTestEnv, string) {
		t.Helper()

		env := newRunManagerTestEnv(t, runManagerTestDeps{})
		initiative := "watcher"
		taskGroupRef := initiative + "/TG-002"
		writeDaemonDependentTaskGroupFixture(t, env, initiative, true)
		env.writeWorkflowFile(
			t,
			initiative,
			filepath.Join("_task_groups", "TG-001", "task_01.md"),
			daemonTaskBody("pending", "Sibling task group task"),
		)
		env.writeWorkflowFile(
			t,
			initiative,
			filepath.Join("_task_groups", "TG-002", "_tasks.md"),
			strings.Replace(
				taskGroupTaskGraphManifest(taskGroupRef),
				"file: task_01.md",
				"file: ../TG-001/task_01.md",
				1,
			),
		)
		return env, taskGroupRef
	}

	assertContainmentFailure := func(t *testing.T, err error) {
		t.Helper()

		var taskGroupErr *taskgroups.Error
		if !errors.As(err, &taskGroupErr) || !errors.Is(err, taskgroups.ErrInvalidPlan) {
			t.Fatalf("task group start error = %v, want invalid task group manifest", err)
		}
		if len(taskGroupErr.Issues) != 1 || !strings.Contains(taskGroupErr.Issues[0].Message, "sibling-ownership") {
			t.Fatalf("task group containment issues = %#v", taskGroupErr.Issues)
		}
	}

	t.Run("Should reject an escaped manifest on a single task run", func(t *testing.T) {
		t.Parallel()

		env, taskGroupRef := newEscapedTaskGroup(t)
		const runID = "task-group-escaped-single"
		_, err := env.manager.StartTaskRun(
			context.Background(),
			env.workspaceRoot,
			taskGroupRef,
			apicore.TaskRunRequest{
				Workspace:        env.workspaceRoot,
				PresentationMode: defaultPresentationMode,
				RuntimeOverrides: rawJSON(t, `{"run_id":"`+runID+`","dry_run":true}`),
			},
		)
		assertContainmentFailure(t, err)
		if _, err := env.globalDB.GetRun(context.Background(), runID); !errors.Is(err, globaldb.ErrRunNotFound) {
			t.Fatalf("GetRun(%q) error = %v, want no run created", runID, err)
		}
	})

	t.Run("Should reject an escaped manifest on a multiple task run", func(t *testing.T) {
		t.Parallel()

		env, _ := newEscapedTaskGroup(t)
		_, err := env.manager.StartTaskRunMultiple(
			context.Background(),
			env.workspaceRoot,
			apicore.TaskRunMultipleRequest{
				Workspace:        env.workspaceRoot,
				Targets:          []apicore.TaskRunTarget{{InitiativeSlug: "watcher", TaskGroupID: "TG-002"}},
				PresentationMode: defaultPresentationMode,
			},
		)
		assertContainmentFailure(t, err)
	})
}

func TestRunManagerTaskGroupWorktreeExecutionIsRejected(t *testing.T) {
	// INVARIANT: task group lifecycle operations never delegate Git worktree
	// creation or switching to the parallel task runner.
	// OWNING_LAYER: service-integration. CONTRACT: IT-030.
	env := newRunManagerTestEnv(t, runManagerTestDeps{})
	initiative := "watcher"
	taskGroupRef := initiative + "/TG-001"
	env.writeWorkflowFile(t, initiative, "_prd.md", "# Canonical PRD\n")
	env.writeWorkflowFile(t, initiative, "_techspec.md", "# Canonical TechSpec\n")
	env.writeWorkflowFile(t, initiative, "_task_groups.md", daemonTaskGroupPlan(" "))
	env.writeWorkflowFile(
		t,
		initiative,
		filepath.Join("_task_groups", "TG-001", "task_01.md"),
		daemonTaskBody("pending", "Task Group task"),
	)

	assertTaskGroupGitMutationForbidden := func(t *testing.T, err error) {
		t.Helper()
		var problem *apicore.Problem
		if !errors.As(err, &problem) || problem.Status != http.StatusUnprocessableEntity ||
			problem.Code != "task_group_git_mutation_forbidden" {
			t.Fatalf("problem = %#v error = %v, want 422 task_group_git_mutation_forbidden", problem, err)
		}
		if got := problem.Details["workflow"]; got != taskGroupRef {
			t.Fatalf("problem workflow = %#v, want %q", got, taskGroupRef)
		}
	}

	_, err := env.manager.StartTaskRun(
		context.Background(),
		env.workspaceRoot,
		taskGroupRef,
		apicore.TaskRunRequest{
			Workspace:        env.workspaceRoot,
			PresentationMode: defaultPresentationMode,
			RuntimeOverrides: rawJSON(t, `{"run_id":"task-group-parallel-task","parallel_tasks":{"enabled":true}}`),
		},
	)
	assertTaskGroupGitMutationForbidden(t, err)
	if _, err := env.globalDB.GetRun(
		context.Background(),
		"task-group-parallel-task",
	); !errors.Is(
		err,
		globaldb.ErrRunNotFound,
	) {
		t.Fatalf("GetRun(task-group-parallel-task) error = %v, want no run created", err)
	}

	_, err = env.manager.StartTaskRunMultiple(
		context.Background(),
		env.workspaceRoot,
		apicore.TaskRunMultipleRequest{
			Workspace:        env.workspaceRoot,
			Slugs:            []string{taskGroupRef},
			Mode:             workspacecfg.TaskRunMultipleModeParallel,
			PresentationMode: defaultPresentationMode,
			RuntimeOverrides: rawJSON(t, `{"run_id":"task-group-parallel-multi"}`),
		},
	)
	assertTaskGroupGitMutationForbidden(t, err)
	if _, err := env.globalDB.GetRun(
		context.Background(),
		"task-group-parallel-multi",
	); !errors.Is(
		err,
		globaldb.ErrRunNotFound,
	) {
		t.Fatalf("GetRun(task-group-parallel-multi) error = %v, want no run created", err)
	}
}

func TestRunManagerTaskGroupEmptyTaskSuiteFailsBeforeRunCreation(t *testing.T) {
	// INVARIANT: an opted-in task group with no executable task file never starts
	// an agent session or creates a durable run.
	// OWNING_LAYER: service-integration. CONTRACT: IT-018.
	env := newRunManagerTestEnv(t, runManagerTestDeps{})
	initiative := "watcher"
	taskGroupRef := initiative + "/TG-001"
	env.writeWorkflowFile(t, initiative, "_prd.md", "# Canonical PRD\n")
	env.writeWorkflowFile(t, initiative, "_techspec.md", "# Canonical TechSpec\n")
	env.writeWorkflowFile(t, initiative, "_task_groups.md", daemonTaskGroupPlan(" "))
	env.writeWorkflowFile(t, initiative, filepath.Join("_task_groups", "TG-001", "_tasks.md"), "# Empty task group\n")

	_, err := env.manager.StartTaskRun(
		context.Background(),
		env.workspaceRoot,
		taskGroupRef,
		apicore.TaskRunRequest{
			Workspace:        env.workspaceRoot,
			PresentationMode: defaultPresentationMode,
			RuntimeOverrides: rawJSON(t, `{"run_id":"task-group-empty-suite"}`),
		},
	)
	var problem *apicore.Problem
	if !errors.As(err, &problem) || problem.Status != http.StatusUnprocessableEntity ||
		problem.Code != "task_group_no_executable_tasks" {
		t.Fatalf("IT-018 problem = %#v error = %v", problem, err)
	}
	if _, err := env.globalDB.GetRun(
		context.Background(),
		"task-group-empty-suite",
	); !errors.Is(
		err,
		globaldb.ErrRunNotFound,
	) {
		t.Fatalf("GetRun(task-group-empty-suite) error = %v, want no run created", err)
	}
}

func TestRunManagerTaskGroupRerunUsesExistingCompletionPolicy(t *testing.T) {
	// INVARIANT: task group runs use the established include-completed policy and
	// never rewrite the completion checkbox while applying it.
	// OWNING_LAYER: service-integration. CONTRACT: IT-014, IT-017.
	env := newRunManagerTestEnv(t, runManagerTestDeps{})
	initiative := "watcher"
	taskGroupRef := initiative + "/TG-001"
	env.writeWorkflowFile(t, initiative, "_prd.md", "# Canonical PRD\n")
	env.writeWorkflowFile(t, initiative, "_techspec.md", "# Canonical TechSpec\n")
	env.writeWorkflowFile(t, initiative, "_task_groups.md", daemonTaskGroupPlan(" "))
	env.writeWorkflowFile(
		t,
		initiative,
		filepath.Join("_task_groups", "TG-001", "task_01.md"),
		daemonTaskBody("completed", "Completed task group task"),
	)

	_, _, _, _, _, _, err := env.manager.prepareTaskStart(
		context.Background(),
		env.workspaceRoot,
		taskGroupRef,
		apicore.TaskRunRequest{Workspace: env.workspaceRoot},
	)
	var problem *apicore.Problem
	if !errors.As(err, &problem) || problem.Status != http.StatusConflict ||
		problem.Code != "workflow_no_pending_tasks" {
		t.Fatalf("IT-017 completed task group problem = %#v error = %v", problem, err)
	}

	_, _, runtimeCfg, _, _, _, err := env.manager.prepareTaskStart(
		context.Background(),
		env.workspaceRoot,
		taskGroupRef,
		apicore.TaskRunRequest{
			Workspace:        env.workspaceRoot,
			RuntimeOverrides: rawJSON(t, `{"include_completed":true}`),
		},
	)
	if err != nil || runtimeCfg == nil || !runtimeCfg.IncludeCompleted || runtimeCfg.ExecutionScope == nil {
		t.Fatalf("IT-014 include-completed task group config = %#v error = %v", runtimeCfg, err)
	}
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
			runArtifactSyncCount(t, env.manager, runA.RunID) >= 1
	})

	time.Sleep(200 * time.Millisecond)
	if got := runArtifactSyncCount(t, env.manager, runB.RunID); got != 0 {
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
	now                    func() time.Time
	buildRunID             func(*model.RuntimeConfig) (string, error)
	syncWorkflow           syncWorkflowFunc
	openRunScope           func(context.Context, *model.RuntimeConfig, model.OpenRunScopeOptions) (model.RunScope, error)
	prepare                func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error)
	execute                func(context.Context, *model.SolvePreparation, *model.RuntimeConfig) error
	executeExec            func(context.Context, *model.RuntimeConfig, model.RunScope) error
	recoveryStrategy       recovery.RemediationStrategy
	openRunDB              func(context.Context, string) (*rundb.RunDB, error)
	loadProjectConfig      func(context.Context, string) (workspacecfg.ProjectConfig, error)
	reviewProviderRegistry reviewProviderRegistryFactory
	reviewWatchGit         ReviewWatchGit
	lookupWorkflowSlugs    func(context.Context, []string) (map[string]string, error)
	getWorkflow            func(context.Context, string) (globaldb.Workflow, error)
	shutdownDrainTimeout   time.Duration
	watcherDebounce        time.Duration
	runDBCacheTTL          time.Duration
}

func newRunManagerTestEnv(tb testing.TB, deps runManagerTestDeps) *runManagerTestEnv {
	tb.Helper()

	homeDir := tb.TempDir()

	paths, err := compozyconfig.ResolveHomePathsFrom(filepath.Join(homeDir, ".compozy"))
	if err != nil {
		tb.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := compozyconfig.EnsureHomeLayout(paths); err != nil {
		tb.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	globalDB, err := globaldb.Open(context.Background(), paths.GlobalDBPath)
	if err != nil {
		tb.Fatalf("globaldb.Open() error = %v", err)
	}
	tb.Cleanup(func() {
		_ = globalDB.Close()
	})

	workspaceRoot := filepath.Join(tb.TempDir(), "workspace")
	if err := os.MkdirAll(filepath.Join(workspaceRoot, ".compozy"), 0o755); err != nil {
		tb.Fatalf("mkdir workspace marker: %v", err)
	}

	workflowSlug := "daemon-workflow"
	if err := os.MkdirAll(model.TaskDirectoryForWorkspace(workspaceRoot, workflowSlug), 0o755); err != nil {
		tb.Fatalf("mkdir task workflow dir: %v", err)
	}

	manager, err := NewRunManager(RunManagerConfig{
		GlobalDB:               globalDB,
		LifecycleContext:       context.Background(),
		HomePaths:              paths,
		WorktreesRoot:          paths.WorktreesDir,
		ShutdownDrainTimeout:   deps.shutdownDrainTimeout,
		Now:                    deps.now,
		BuildRunID:             deps.buildRunID,
		SyncWorkflow:           deps.syncWorkflow,
		OpenRunScope:           firstOpenRunScope(deps.openRunScope),
		Prepare:                firstPrepare(deps.prepare),
		Execute:                firstExecute(deps.execute),
		ExecuteExec:            firstExecuteExec(deps.executeExec),
		RecoveryStrategy:       deps.recoveryStrategy,
		OpenRunDB:              deps.openRunDB,
		ReviewProviderRegistry: deps.reviewProviderRegistry,
		ReviewWatchGit:         deps.reviewWatchGit,
		WatcherDebounce:        deps.watcherDebounce,
		RunDBCacheTTL:          deps.runDBCacheTTL,
		LookupWorkflowSlugs:    deps.lookupWorkflowSlugs,
		GetWorkflow:            deps.getWorkflow,
		LoadProjectConfig:      firstProjectConfig(deps.loadProjectConfig),
	})
	if err != nil {
		tb.Fatalf("NewRunManager() error = %v", err)
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

	runDB, err := e.manager.openRunDB(context.Background(), runID)
	if err != nil {
		t.Fatalf("openRunDBForRunID(%q) error = %v", runID, err)
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
	startErr    error
	mutableHook string
	mutableErr  error
}

func (m *stubRuntimeManager) Start(context.Context) error {
	return m.startErr
}

func (m *stubRuntimeManager) DispatchMutableHook(_ context.Context, hook string, payload any) (any, error) {
	if m != nil && hook == m.mutableHook && m.mutableErr != nil {
		return nil, m.mutableErr
	}
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

func firstProjectConfig(
	fn func(context.Context, string) (workspacecfg.ProjectConfig, error),
) func(context.Context, string) (workspacecfg.ProjectConfig, error) {
	if fn != nil {
		return fn
	}
	return func(context.Context, string) (workspacecfg.ProjectConfig, error) {
		return workspacecfg.ProjectConfig{}, nil
	}
}

func rawJSON(t *testing.T, value string) json.RawMessage {
	t.Helper()
	return json.RawMessage(value)
}

func writePersistedExecRun(t *testing.T, workspaceRoot string, runID string, createdAt time.Time) {
	t.Helper()

	runArtifacts, err := model.ResolvePersistedRunArtifacts(workspaceRoot, runID)
	if err != nil {
		t.Fatalf("ResolvePersistedRunArtifacts(%q) error = %v", runID, err)
	}
	if err := os.MkdirAll(runArtifacts.RunDir, 0o755); err != nil {
		t.Fatalf("mkdir persisted exec run dir: %v", err)
	}
	record := runpkg.PersistedExecRun{
		Version:       1,
		Mode:          model.ModeExec,
		RunID:         runID,
		Status:        "completed",
		WorkspaceRoot: workspaceRoot,
		CreatedAt:     createdAt,
		UpdatedAt:     createdAt.Add(time.Minute),
		EventsPath:    runArtifacts.EventsPath,
		TurnsDir:      runArtifacts.TurnsDir,
	}
	payload, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("json.Marshal(PersistedExecRun) error = %v", err)
	}
	if err := os.WriteFile(runArtifacts.RunMetaPath, payload, 0o644); err != nil {
		t.Fatalf("write persisted exec run metadata: %v", err)
	}
}

func mustRunManagerTextBlock(t *testing.T, text string) kinds.ContentBlock {
	t.Helper()

	block, err := kinds.NewContentBlock(kinds.TextBlock{Text: text})
	if err != nil {
		t.Fatalf("NewContentBlock(text): %v", err)
	}
	return block
}

func mustRunManagerToolUseBlock(t *testing.T, id string, name string, input string) kinds.ContentBlock {
	t.Helper()

	block, err := kinds.NewContentBlock(kinds.ToolUseBlock{
		ID:    id,
		Name:  name,
		Input: json.RawMessage(input),
	})
	if err != nil {
		t.Fatalf("NewContentBlock(tool use): %v", err)
	}
	return block
}

func mustRunManagerToolResultBlock(t *testing.T, id string, content string) kinds.ContentBlock {
	t.Helper()

	block, err := kinds.NewContentBlock(kinds.ToolResultBlock{
		ToolUseID: id,
		Content:   content,
	})
	if err != nil {
		t.Fatalf("NewContentBlock(tool result): %v", err)
	}
	return block
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

const (
	// waitForRunBudget bounds how long the run-lifecycle poll helpers wait for
	// their predicate to hold. It is generous enough to absorb Git worktree and
	// race-detector contention when the full daemon suite runs with
	// -race -parallel (a fixed few-second budget proved flaky there), yet short
	// enough to fail fast on a genuine regression.
	waitForRunBudget = 60 * time.Second
	// waitForRunDeadlineMargin keeps the poll deadline strictly inside the
	// test's own deadline so an expiry surfaces a diagnostic Fatalf before the
	// `go test` harness force-kills the binary with an opaque timeout panic.
	waitForRunDeadlineMargin = 2 * time.Second
)

// waitDeadlineContext derives a poll context bounded by the smaller of a
// generous fixed budget and the test's own deadline (minus a safety margin).
// Tying the ceiling to t.Deadline() keeps the wait proportional to how long the
// test is actually allowed to run instead of an arbitrary wall-clock guess that
// is unrelated to suite load.
func waitDeadlineContext(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	deadline := time.Now().Add(waitForRunBudget)
	if testDeadline, ok := t.Deadline(); ok {
		if capped := testDeadline.Add(-waitForRunDeadlineMargin); capped.Before(deadline) {
			deadline = capped
		}
	}
	return context.WithDeadline(context.Background(), deadline)
}

func waitForRun(
	t *testing.T,
	db *globaldb.GlobalDB,
	runID string,
	predicate func(globaldb.Run) bool,
) globaldb.Run {
	t.Helper()

	ctx, cancel := waitDeadlineContext(t)
	defer cancel()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	var (
		lastRow globaldb.Run
		lastErr error
		seen    bool
	)
	for {
		row, err := db.GetRun(context.Background(), runID)
		if err == nil {
			lastRow, seen = row, true
			if predicate(row) {
				return row
			}
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			if seen {
				t.Fatalf(
					"timed out waiting for run %q: last status=%q error_text=%q last_read_err=%v",
					runID, lastRow.Status, lastRow.ErrorText, lastErr,
				)
			}
			t.Fatalf("timed out waiting for run %q: no row observed, last_read_err=%v", runID, lastErr)
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

	ctx, cancel := waitDeadlineContext(t)
	defer cancel()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	var (
		lastCount int
		lastErr   error
	)
	for {
		runs, err := manager.List(context.Background(), apicore.RunListQuery{
			Workspace: workspace,
			Status:    status,
			Limit:     10,
		})
		if err == nil {
			lastCount = len(runs)
			if lastCount == want {
				return
			}
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			t.Fatalf(
				"timed out waiting for %d run(s) with status %q: last observed count=%d last_list_err=%v",
				want, status, lastCount, lastErr,
			)
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

func waitForBool(t *testing.T, ch <-chan bool) bool {
	t.Helper()

	select {
	case got := <-ch:
		return got
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for bool")
	}
	return false
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

func waitForWorkspaceStreamItem(
	t *testing.T,
	ch <-chan apicore.WorkspaceStreamItem,
) apicore.WorkspaceStreamItem {
	t.Helper()

	select {
	case item := <-ch:
		return item
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for workspace stream item")
		return apicore.WorkspaceStreamItem{}
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

func runArtifactSyncCount(t *testing.T, manager *RunManager, runID string) int {
	t.Helper()

	return len(runArtifactSyncLog(t, manager, runID))
}

func runArtifactSyncLog(t *testing.T, manager *RunManager, runID string) []rundb.ArtifactSyncRow {
	t.Helper()

	runDB, err := manager.openRunDB(context.Background(), runID)
	if err != nil {
		t.Fatalf("openRunDBForRunID(%q) error = %v", runID, err)
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
		"provider: coderabbit",
		"pr: \"123\"",
		"round: 1",
		"round_created_at: 2026-04-17T20:00:00Z",
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
