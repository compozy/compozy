package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/gate"
	storepkg "github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil"
)

// Invariant: every low-level terminal loop_runs status mutation invokes the settlement authority
// in the same function. Source routing is the product safety contract, so this AST check owns IT-030.
func TestGlobalDBLoopTerminalMutationPathsShouldInvokeSettlementAuthority(t *testing.T) {
	t.Parallel()
	t.Run("Should require every terminal mutation path to invoke settlement authority", func(t *testing.T) {
		t.Parallel()

		_, currentFile, _, ok := runtime.Caller(0)
		if !ok {
			t.Fatal("runtime.Caller() did not resolve the GlobalDB suite")
		}
		directory := filepath.Dir(currentFile)
		entries, err := os.ReadDir(directory)
		if err != nil {
			t.Fatalf("os.ReadDir() error = %v", err)
		}
		terminalMutationSelectors := map[string]struct{}{
			"CompareAndSwapLoopRunStatus":       {},
			"TransitionLoopCoordinatorBoundary": {},
		}
		terminalSettlementAuthorities := map[string]struct{}{
			"settleLoopRunTerminal":            {},
			"settleLoopRunTerminalWithRecords": {},
		}
		covered := 0
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasPrefix(entry.Name(), "global_db_") ||
				!strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			path := filepath.Join(directory, entry.Name())
			parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				t.Fatalf("parser.ParseFile(%s) error = %v", path, err)
			}
			for _, declaration := range parsed.Decls {
				function, ok := declaration.(*ast.FuncDecl)
				if !ok || function.Body == nil || !functionCallsAnySelector(function, terminalMutationSelectors) {
					continue
				}
				covered++
				if !functionCallsAnyIdentifier(function, terminalSettlementAuthorities) {
					t.Fatalf("terminal mutation function %s in %s bypasses settleLoopRunTerminal",
						function.Name.Name, entry.Name())
				}
			}
		}
		if covered != len(terminalMutationSelectors) {
			t.Fatalf("terminal mutation chokepoints covered = %d, want %d", covered, len(terminalMutationSelectors))
		}
	})
}

func functionCallsAnySelector(function *ast.FuncDecl, selectors map[string]struct{}) bool {
	found := false
	ast.Inspect(function.Body, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if ok {
			_, found = selectors[selector.Sel.Name]
		}
		return !found
	})
	return found
}

func functionCallsAnyIdentifier(function *ast.FuncDecl, names map[string]struct{}) bool {
	found := false
	ast.Inspect(function.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return !found
		}
		identifier, ok := call.Fun.(*ast.Ident)
		if !ok {
			return !found
		}
		_, found = names[identifier.Name]
		return !found
	})
	return found
}

// Invariant: terminal Loop runs own no live execution records, reconciliation is idempotent,
// and provenance repair changes metadata only. The canonical GlobalDB Loop suite owns this boundary.
func TestGlobalDBLoopTerminalReconciliationShouldConvergeExecutionRecords(t *testing.T) {
	t.Parallel()

	const terminalOrphanCase = "Should repair a terminal orphan once and project backfilled coordinator provenance UT-030 UT-031 UT-033 IT-005 IT-006 IT-025"
	t.Run(terminalOrphanCase, func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 19, 17, 3, 58, 0, time.UTC)
		run, err := globalDB.CreateLoopRunForStart(
			ctx, testLoopRun("looprun-terminal-reconcile", now, looppkg.StatusRunning), dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		coordinatorTaskID := loopCoordinatorTaskID(run.ID)
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE tasks SET metadata_json = '{}' WHERE id = ?`,
			coordinatorTaskID,
		); err != nil {
			t.Fatalf("clear coordinator metadata error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx, `UPDATE loop_runs SET status = 'failed' WHERE id = ?`, run.ID,
		); err != nil {
			t.Fatalf("seed terminal orphan error = %v", err)
		}

		report, err := globalDB.SweepLoopRunOrphans(ctx)
		if err != nil {
			t.Fatalf("SweepLoopRunOrphans() error = %v", err)
		}
		if report.RunsExamined != 1 || report.RecordsSettled == 0 || report.OrphansRepaired != 1 {
			t.Fatalf("first sweep report = %#v, want one repaired orphan", report)
		}
		taskRecord, err := globalDB.GetTask(ctx, coordinatorTaskID)
		if err != nil {
			t.Fatalf("GetTask(coordinator) error = %v", err)
		}
		if taskRecord.Status != taskpkg.TaskStatusFailed {
			t.Fatalf("coordinator status = %q, want failed", taskRecord.Status)
		}
		var liveRuns int
		if err := globalDB.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_runs WHERE loop_run_id = ?
		AND status IN ('queued','claimed','starting','running','needs_attention')`, run.ID).Scan(&liveRuns); err != nil {
			t.Fatalf("count live execution records error = %v", err)
		}
		if liveRuns != 0 {
			t.Fatalf("live execution records = %d, want zero", liveRuns)
		}
		_, err = globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope: taskpkg.ScopeWorkspace, WorkspaceID: string(run.WorkspaceID),
			RunKind: taskpkg.RunKindCoordinator, ClaimerSessionID: "boot-barrier",
			ClaimedBy:     &taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "loop"},
			LeaseDuration: time.Minute, Now: now.Add(time.Minute),
		})
		if !errors.Is(err, taskpkg.ErrNoClaimableRun) {
			t.Fatalf("ClaimNextRun(after barrier) error = %v, want ErrNoClaimableRun", err)
		}
		var reason string
		if err := globalDB.db.QueryRowContext(ctx, `SELECT json_extract(payload_json, '$.reason')
		FROM task_events WHERE task_id = ? AND event_type = 'task.status_changed'
		ORDER BY event_seq DESC LIMIT 1`, coordinatorTaskID).Scan(&reason); err != nil {
			t.Fatalf("read settlement reason error = %v", err)
		}
		if reason != reconciledRunTerminalReason {
			t.Fatalf("settlement reason = %q, want %q", reason, reconciledRunTerminalReason)
		}
		second, err := globalDB.SweepLoopRunOrphans(ctx)
		if err != nil {
			t.Fatalf("SweepLoopRunOrphans(second) error = %v", err)
		}
		if second != (looppkg.SweepReport{}) {
			t.Fatalf("second sweep = %#v, want silent idempotent report", second)
		}
		repaired, err := globalDB.BackfillLoopProvenance(ctx)
		if err != nil {
			t.Fatalf("BackfillLoopProvenance() error = %v", err)
		}
		if repaired != 1 {
			t.Fatalf("BackfillLoopProvenance() = %d, want 1", repaired)
		}
		if repairedAgain, err := globalDB.BackfillLoopProvenance(ctx); err != nil || repairedAgain != 0 {
			t.Fatalf("BackfillLoopProvenance(second) = %d, %v, want 0, nil", repairedAgain, err)
		}
		catalog, err := globalDB.ListTaskCatalog(ctx, taskpkg.CatalogQuery{
			Scope: taskpkg.CatalogScopeWorkspace, WorkspaceID: string(run.WorkspaceID),
			LoopRunID: string(run.ID), IncludeDrafts: true, Limit: 10,
		})
		if err != nil {
			t.Fatalf("ListTaskCatalog(backfilled coordinator) error = %v", err)
		}
		if len(catalog.Tasks) != 1 || catalog.Tasks[0].ID != coordinatorTaskID {
			t.Fatalf("backfilled coordinator catalog = %#v, want only %q", catalog.Tasks, coordinatorTaskID)
		}
		provenance := catalog.Tasks[0].RunProvenance
		if provenance == nil || provenance.LoopRunID != string(run.ID) ||
			provenance.RunKind != taskpkg.RunKindCoordinator {
			t.Fatalf("backfilled coordinator provenance = %#v", provenance)
		}
		if catalog.Tasks[0].Status != taskpkg.TaskStatusFailed {
			t.Fatalf("backfill changed coordinator status to %q", catalog.Tasks[0].Status)
		}
	})

	t.Run("Should audit a repaired live run under an already-terminal task once IT-006", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 20, 8, 30, 0, 0, time.UTC)
		run, err := globalDB.CreateLoopRunForStart(
			ctx, testLoopRun("looprun-terminal-task-live-run", now, looppkg.StatusRunning), dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		coordinatorTaskID := loopCoordinatorTaskID(run.ID)
		var coordinatorRunID string
		if err := globalDB.db.QueryRowContext(ctx, `SELECT id FROM task_runs
			WHERE loop_run_id = ? AND run_kind = 'coordinator'`, run.ID).Scan(&coordinatorRunID); err != nil {
			t.Fatalf("read coordinator run id error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(ctx, `UPDATE tasks SET status = 'failed', closed_at = ? WHERE id = ?`,
			storepkg.FormatTimestamp(now), coordinatorTaskID); err != nil {
			t.Fatalf("seed terminal coordinator task error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_runs SET status = 'failed' WHERE id = ?`,
			run.ID,
		); err != nil {
			t.Fatalf("seed terminal Loop run error = %v", err)
		}
		before := countTaskStatusEventsForTest(t, globalDB, coordinatorTaskID)

		report, err := globalDB.SweepLoopRunOrphans(ctx)
		if err != nil {
			t.Fatalf("SweepLoopRunOrphans() error = %v", err)
		}
		if report.RunsExamined != 1 || report.RecordsSettled != 1 || report.OrphansRepaired != 1 {
			t.Fatalf("repair report = %#v, want one audited run repair", report)
		}
		coordinator, err := globalDB.GetTask(ctx, coordinatorTaskID)
		if err != nil {
			t.Fatalf("GetTask(coordinator) error = %v", err)
		}
		if coordinator.Status != taskpkg.TaskStatusFailed {
			t.Fatalf("coordinator status = %q, want unchanged failed", coordinator.Status)
		}
		var runStatus string
		if err := globalDB.db.QueryRowContext(ctx, `SELECT status FROM task_runs WHERE id = ?`, coordinatorRunID).
			Scan(&runStatus); err != nil {
			t.Fatalf("read repaired task run status error = %v", err)
		}
		if runStatus != taskpkg.TaskRunStatusCanceled.String() {
			t.Fatalf("repaired task run status = %q, want canceled", runStatus)
		}
		var payloadJSON []byte
		if err := globalDB.db.QueryRowContext(ctx, `SELECT payload_json FROM task_events
			WHERE task_id = ? AND run_id = ? AND event_type = 'task.status_changed'
			ORDER BY event_seq DESC LIMIT 1`, coordinatorTaskID, coordinatorRunID).Scan(&payloadJSON); err != nil {
			t.Fatalf("read repair task event error = %v", err)
		}
		var payload taskStatusChangedEventPayload
		if err := json.Unmarshal(payloadJSON, &payload); err != nil {
			t.Fatalf("decode repair task event error = %v", err)
		}
		if payload.TaskID != coordinatorTaskID || payload.RunID != coordinatorRunID ||
			payload.LoopRunID != string(run.ID) || payload.WorkspaceID != string(run.WorkspaceID) ||
			payload.ActorKind != string(taskpkg.ActorKindDaemon) ||
			payload.FromStatus != string(taskpkg.TaskStatusFailed) ||
			payload.ToStatus != string(taskpkg.TaskStatusFailed) ||
			payload.ReleaseReason != reconciledRunTerminalReason {
			t.Fatalf("repair payload = %#v, want complete same-status audit anchors", payload)
		}
		second, err := globalDB.SweepLoopRunOrphans(ctx)
		if err != nil {
			t.Fatalf("SweepLoopRunOrphans(second) error = %v", err)
		}
		if second != (looppkg.SweepReport{}) {
			t.Fatalf("second sweep = %#v, want silent", second)
		}
		if after := countTaskStatusEventsForTest(t, globalDB, coordinatorTaskID); after != before+1 {
			t.Fatalf("task status event count = %d, want %d after silent second sweep", after, before+1)
		}
	})

	t.Run("Should leave active Loop execution records untouched IT-007", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 20, 9, 0, 0, 0, time.UTC)
		run, err := globalDB.CreateLoopRunForStart(
			ctx, testLoopRun("looprun-active-reconcile", now, looppkg.StatusRunning), dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		report, err := globalDB.SweepLoopRunOrphans(ctx)
		if err != nil {
			t.Fatalf("SweepLoopRunOrphans() error = %v", err)
		}
		if report != (looppkg.SweepReport{}) {
			t.Fatalf("active-run sweep = %#v, want no changes", report)
		}
		var liveRuns int
		if err := globalDB.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_runs WHERE loop_run_id = ?
		AND status IN ('queued','claimed','starting','running','needs_attention')`, run.ID).Scan(&liveRuns); err != nil {
			t.Fatalf("count active execution records error = %v", err)
		}
		if liveRuns == 0 {
			t.Fatal("active Loop lost its execution record")
		}
	})

	t.Run("Should backfill active coordinator metadata without changing lifecycle IT-029", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 20, 9, 30, 0, 0, time.UTC)
		run, err := globalDB.CreateLoopRunForStart(
			ctx, testLoopRun("looprun-active-backfill", now, looppkg.StatusRunning), dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		coordinatorTaskID := loopCoordinatorTaskID(run.ID)
		beforeTask, err := globalDB.GetTask(ctx, coordinatorTaskID)
		if err != nil {
			t.Fatalf("GetTask(before backfill) error = %v", err)
		}
		var taskRunID string
		var beforeRunStatus string
		if err := globalDB.db.QueryRowContext(ctx, `SELECT id, status FROM task_runs
			WHERE task_id = ? AND loop_run_id = ? AND run_kind = 'coordinator'`,
			coordinatorTaskID, run.ID).Scan(&taskRunID, &beforeRunStatus); err != nil {
			t.Fatalf("read coordinator run before backfill error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE tasks SET metadata_json = '{}' WHERE id = ?`,
			coordinatorTaskID,
		); err != nil {
			t.Fatalf("clear active coordinator metadata error = %v", err)
		}

		repaired, err := globalDB.BackfillLoopProvenance(ctx)
		if err != nil {
			t.Fatalf("BackfillLoopProvenance() error = %v", err)
		}
		if repaired != 1 {
			t.Fatalf("BackfillLoopProvenance() = %d, want 1", repaired)
		}
		afterTask, err := globalDB.GetTask(ctx, coordinatorTaskID)
		if err != nil {
			t.Fatalf("GetTask(after backfill) error = %v", err)
		}
		var afterRunStatus string
		if err := globalDB.db.QueryRowContext(ctx, `SELECT status FROM task_runs WHERE id = ?`, taskRunID).
			Scan(&afterRunStatus); err != nil {
			t.Fatalf("read coordinator run after backfill error = %v", err)
		}
		if afterTask.Status != beforeTask.Status || afterTask.CurrentRunID != beforeTask.CurrentRunID ||
			afterRunStatus != beforeRunStatus {
			t.Fatalf(
				"backfill changed lifecycle: task %q/%q run %q, want %q/%q run %q",
				afterTask.Status, afterTask.CurrentRunID, afterRunStatus,
				beforeTask.Status, beforeTask.CurrentRunID, beforeRunStatus,
			)
		}
		catalog, err := globalDB.ListTaskCatalog(ctx, taskpkg.CatalogQuery{
			Scope: taskpkg.CatalogScopeWorkspace, WorkspaceID: string(run.WorkspaceID),
			LoopRunID: string(run.ID), IncludeDrafts: true, Limit: 10,
		})
		if err != nil {
			t.Fatalf("ListTaskCatalog(active backfill) error = %v", err)
		}
		if len(catalog.Tasks) != 1 || catalog.Tasks[0].RunProvenance == nil ||
			catalog.Tasks[0].RunProvenance.RunKind != taskpkg.RunKindCoordinator {
			t.Fatalf("active backfill catalog = %#v", catalog.Tasks)
		}
	})

	t.Run("Should never copy foreign workspace display metadata during provenance repair", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-a", "ws-b")
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 20, 9, 45, 0, 0, time.UTC)
		local := testLoopRun("looprun-backfill-local", now, looppkg.StatusRunning)
		local.WorkspaceID = "ws-a"
		if _, err := globalDB.CreateLoopRunForStart(ctx, local, dsl.ConcurrencyAllow); err != nil {
			t.Fatalf("CreateLoopRunForStart(local) error = %v", err)
		}
		foreign := testLoopRun("looprun-backfill-foreign", now, looppkg.StatusRunning)
		foreign.WorkspaceID = "ws-b"
		if _, err := globalDB.CreateLoopRunForStart(ctx, foreign, dsl.ConcurrencyAllow); err != nil {
			t.Fatalf("CreateLoopRunForStart(foreign) error = %v", err)
		}
		localCoordinatorID := loopCoordinatorTaskID(local.ID)
		if _, err := globalDB.db.ExecContext(ctx, `UPDATE task_runs SET loop_run_id = NULL
			WHERE loop_run_id = ? AND run_kind = 'coordinator'`, foreign.ID); err != nil {
			t.Fatalf("detach foreign coordinator relation error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(ctx, `UPDATE task_runs SET loop_run_id = ?
			WHERE task_id = ? AND run_kind = 'coordinator'`, foreign.ID, localCoordinatorID); err != nil {
			t.Fatalf("seed cross-workspace Loop relation error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(ctx, `UPDATE tasks SET metadata_json = '{}'
			WHERE id = ?`, localCoordinatorID); err != nil {
			t.Fatalf("clear local coordinator metadata error = %v", err)
		}

		repaired, err := globalDB.BackfillLoopProvenance(ctx)
		if err != nil {
			t.Fatalf("BackfillLoopProvenance() error = %v", err)
		}
		if repaired != 1 {
			t.Fatalf("BackfillLoopProvenance() = %d, want 1", repaired)
		}
		record, err := globalDB.GetTask(ctx, localCoordinatorID)
		if err != nil {
			t.Fatalf("GetTask(local coordinator) error = %v", err)
		}
		var metadata map[string]any
		if err := json.Unmarshal(record.Metadata, &metadata); err != nil {
			t.Fatalf("decode repaired local metadata error = %v", err)
		}
		if metadata["loop_run_id"] != string(foreign.ID) || metadata["workspace_id"] != "ws-a" {
			t.Fatalf("repaired relational metadata = %#v", metadata)
		}
		if _, exists := metadata["loop_name"]; exists {
			t.Fatalf("foreign workspace loop_name leaked into metadata: %#v", metadata)
		}
	})

	const retentionOrphanCase = "Should settle and project a retention orphan with relational provenance only UT-032 IT-009 IT-029"
	t.Run(retentionOrphanCase, func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 20, 10, 0, 0, 0, time.UTC)
		run, err := globalDB.CreateLoopRunForStart(
			ctx, testLoopRun("looprun-retention-orphan", now, looppkg.StatusRunning), dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		coordinatorTaskID := loopCoordinatorTaskID(run.ID)
		const missingRunID = "looprun-retention-missing"
		if _, err := globalDB.db.ExecContext(ctx,
			`UPDATE task_runs SET loop_run_id = ? WHERE loop_run_id = ?`, missingRunID, run.ID); err != nil {
			t.Fatalf("seed missing Loop run reference error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE tasks SET metadata_json = '{"loop_name":"stale-name"}' WHERE id = ?`,
			coordinatorTaskID,
		); err != nil {
			t.Fatalf("seed stale coordinator metadata error = %v", err)
		}

		report, err := globalDB.SweepLoopRunOrphans(ctx)
		if err != nil {
			t.Fatalf("SweepLoopRunOrphans() error = %v", err)
		}
		if report.RunsExamined != 1 || report.OrphansRepaired != 1 {
			t.Fatalf("retention-orphan report = %#v, want one repaired orphan", report)
		}
		var reason string
		if err := globalDB.db.QueryRowContext(ctx, `SELECT json_extract(payload_json, '$.reason')
			FROM task_events WHERE task_id = ? AND event_type = 'task.status_changed'
			ORDER BY event_seq DESC LIMIT 1`, coordinatorTaskID).Scan(&reason); err != nil {
			t.Fatalf("read retention-orphan reason error = %v", err)
		}
		if reason != runMissingReason {
			t.Fatalf("retention-orphan reason = %q, want %q", reason, runMissingReason)
		}
		if repaired, err := globalDB.BackfillLoopProvenance(ctx); err != nil || repaired != 1 {
			t.Fatalf("BackfillLoopProvenance() = %d, %v, want 1, nil", repaired, err)
		}
		taskRecord, err := globalDB.GetTask(ctx, coordinatorTaskID)
		if err != nil {
			t.Fatalf("GetTask(coordinator) error = %v", err)
		}
		var metadata map[string]any
		if err := json.Unmarshal(taskRecord.Metadata, &metadata); err != nil {
			t.Fatalf("decode coordinator metadata error = %v", err)
		}
		if metadata["loop_run_id"] != missingRunID || metadata["workspace_id"] != string(run.WorkspaceID) {
			t.Fatalf("relational provenance = %#v, want run and workspace", metadata)
		}
		if _, exists := metadata["loop_name"]; exists {
			t.Fatalf("missing-run metadata retained loop_name: %#v", metadata)
		}
		catalog, err := globalDB.ListTaskCatalog(ctx, taskpkg.CatalogQuery{
			Scope: taskpkg.CatalogScopeWorkspace, WorkspaceID: string(run.WorkspaceID),
			LoopRunID: missingRunID, IncludeDrafts: true, Limit: 10,
		})
		if err != nil {
			t.Fatalf("ListTaskCatalog(retention orphan) error = %v", err)
		}
		if len(catalog.Tasks) != 1 || catalog.Tasks[0].ID != coordinatorTaskID {
			t.Fatalf("retention-orphan catalog = %#v, want only %q", catalog.Tasks, coordinatorTaskID)
		}
		provenance := catalog.Tasks[0].RunProvenance
		if provenance == nil || provenance.LoopRunID != missingRunID ||
			provenance.RunKind != taskpkg.RunKindCoordinator {
			t.Fatalf("retention-orphan provenance = %#v", provenance)
		}
	})

	t.Run("Should serialize inline settlement against a concurrent sweep IT-008", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 20, 11, 0, 0, 0, time.UTC)
		run, err := globalDB.CreateLoopRunForStart(
			ctx, testLoopRun("looprun-settlement-race", now, looppkg.StatusRunning), dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		start := make(chan struct{})
		errs := make(chan error, 2)
		go func() {
			<-start
			errs <- globalDB.CompareAndSwapLoopRunStatus(ctx, run.ID, looppkg.StatusRunning,
				looppkg.StatusFailed, looppkg.TransitionCauseCoordinatorFailure, now.Add(time.Second))
		}()
		go func() {
			<-start
			_, sweepErr := globalDB.SweepLoopRunOrphans(ctx)
			errs <- sweepErr
		}()
		close(start)
		for range 2 {
			if err := <-errs; err != nil {
				t.Fatalf("concurrent settlement error = %v", err)
			}
		}
		var settlementEvents int
		if err := globalDB.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_events
			WHERE task_id = ? AND event_type = 'task.status_changed'
			AND json_extract(payload_json, '$.reason') IN (?, ?)`, loopCoordinatorTaskID(run.ID),
			loopRunTerminalReason, reconciledRunTerminalReason).Scan(&settlementEvents); err != nil {
			t.Fatalf("count settlement events error = %v", err)
		}
		if settlementEvents != 1 {
			t.Fatalf("settlement events = %d, want one serialized winner", settlementEvents)
		}
	})
}

// Invariant: the settlement authority maps every terminal cause to one final hierarchy state.
// This real-SQLite GlobalDB table owns the complete cause matrix; public cancel, kill, and
// coordinator completion paths remain in their operation-specific canonical suites.
func TestGlobalDBLoopTerminalSettlementShouldApplyCauseMatrix(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name              string
		status            looppkg.Status
		cause             looppkg.TransitionCause
		coordinatorStatus taskpkg.Status
		detail            string
	}{
		{name: "done", status: looppkg.StatusDone, cause: looppkg.TransitionCauseContract,
			coordinatorStatus: taskpkg.TaskStatusCompleted, detail: loopSettlementDoneDetail},
		{name: "no-op", status: looppkg.StatusNoOp, cause: looppkg.TransitionCauseContract,
			coordinatorStatus: taskpkg.TaskStatusCompleted, detail: loopSettlementDoneDetail},
		{name: "failed", status: looppkg.StatusFailed, cause: looppkg.TransitionCauseContract,
			coordinatorStatus: taskpkg.TaskStatusFailed, detail: "run failed; node no longer needed"},
		{name: "exhausted", status: looppkg.StatusExhausted, cause: looppkg.TransitionCauseBudget,
			coordinatorStatus: taskpkg.TaskStatusFailed, detail: "run exhausted; node no longer needed"},
		{name: "stalled", status: looppkg.StatusStalled, cause: looppkg.TransitionCauseNoProgress,
			coordinatorStatus: taskpkg.TaskStatusFailed, detail: "run stalled; node no longer needed"},
		{name: "cancel", status: looppkg.StatusCanceled, cause: looppkg.TransitionCauseOperatorCancel,
			coordinatorStatus: taskpkg.TaskStatusCanceled, detail: "run canceled; node no longer needed"},
		{name: "kill", status: looppkg.StatusCanceled, cause: looppkg.TransitionCauseOperatorKill,
			coordinatorStatus: taskpkg.TaskStatusCanceled, detail: "run killed; node no longer needed"},
	}
	for _, testCase := range testCases {
		t.Run("Should settle final state for "+testCase.name, func(t *testing.T) {
			t.Parallel()

			globalDB := openLoopTestGlobalDB(t)
			ctx := testutil.Context(t)
			now := time.Date(2026, time.August, 20, 13, 0, 0, 0, time.UTC)
			run, err := globalDB.CreateLoopRunForStart(
				ctx, testLoopRun("looprun-cause-matrix-"+testCase.name, now, looppkg.StatusRunning),
				dsl.ConcurrencyAllow,
			)
			if err != nil {
				t.Fatalf("CreateLoopRunForStart() error = %v", err)
			}
			coordinatorTaskID := loopCoordinatorTaskID(run.ID)
			liveTask := workspaceTaskRecordForTest("task-cause-matrix-live-"+testCase.name, string(run.WorkspaceID))
			liveTask.ParentTaskID = coordinatorTaskID
			liveTask.Status = taskpkg.TaskStatusReady
			if err := globalDB.CreateTask(ctx, liveTask); err != nil {
				t.Fatalf("CreateTask(live cell) error = %v", err)
			}
			liveRun := taskRunForTest("run-cause-matrix-live-"+testCase.name, liveTask.ID)
			liveRun.LoopRunID = string(run.ID)
			liveRun.RunKind = taskpkg.RunKindWorker
			if err := globalDB.CreateTaskRun(ctx, liveRun); err != nil {
				t.Fatalf("CreateTaskRun(live cell) error = %v", err)
			}
			terminalTask := workspaceTaskRecordForTest(
				"task-cause-matrix-terminal-"+testCase.name, string(run.WorkspaceID),
			)
			terminalTask.ParentTaskID = coordinatorTaskID
			terminalTask.Status = taskpkg.TaskStatusCompleted
			terminalTask.ClosedAt = now
			if err := globalDB.CreateTask(ctx, terminalTask); err != nil {
				t.Fatalf("CreateTask(terminal cell) error = %v", err)
			}

			observer := &recordingTaskEventCommitObserver{db: globalDB}
			globalDB.SetTaskEventCommitObserver(observer)
			if err := globalDB.CompareAndSwapLoopRunStatus(
				ctx, run.ID, looppkg.StatusRunning, testCase.status, testCase.cause, now.Add(time.Second),
			); err != nil {
				t.Fatalf("CompareAndSwapLoopRunStatus() error = %v", err)
			}
			assertLoopSettlementFinalStateForTest(
				t,
				globalDB,
				run,
				coordinatorTaskID,
				liveTask.ID,
				terminalTask.ID,
				testCase.coordinatorStatus,
				taskpkg.TaskStatusCompleted,
			)
			if observer.err != nil {
				t.Fatalf("task event observer error = %v", observer.err)
			}
			if len(observer.records) != 2 {
				t.Fatalf("settlement events = %d, want live cell then coordinator", len(observer.records))
			}
			if observer.records[0].Event.TaskID != liveTask.ID ||
				observer.records[1].Event.TaskID != coordinatorTaskID {
				t.Fatalf("settlement order = %#v, want live cell before coordinator", observer.records)
			}
			for _, record := range observer.records {
				var payload taskStatusChangedEventPayload
				if err := json.Unmarshal(record.Event.Payload, &payload); err != nil {
					t.Fatalf("decode settlement payload error = %v", err)
				}
				if payload.Reason != loopRunTerminalReason || payload.Detail != testCase.detail ||
					payload.LoopRunID != string(run.ID) || payload.WorkspaceID != string(run.WorkspaceID) ||
					payload.ActorKind != string(taskpkg.ActorKindDaemon) ||
					payload.ReleaseReason != loopRunTerminalReason {
					t.Fatalf("settlement payload = %#v, want structured terminal reason", payload)
				}
			}
		})
	}
}

func assertLoopSettlementFinalStateForTest(
	t *testing.T,
	globalDB *GlobalDB,
	run looppkg.Run,
	coordinatorTaskID string,
	liveTaskID string,
	terminalTaskID string,
	wantCoordinator taskpkg.Status,
	wantTerminal taskpkg.Status,
) {
	t.Helper()
	ctx := testutil.Context(t)
	for taskID, wantStatus := range map[string]taskpkg.Status{
		coordinatorTaskID: wantCoordinator,
		liveTaskID:        taskpkg.TaskStatusCanceled,
		terminalTaskID:    wantTerminal,
	} {
		record, err := globalDB.GetTask(ctx, taskID)
		if err != nil {
			t.Fatalf("GetTask(%s) error = %v", taskID, err)
		}
		if record.Status != wantStatus {
			t.Fatalf("task %s final status = %q, want %q", taskID, record.Status, wantStatus)
		}
	}
	var liveRecords int
	if err := globalDB.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_runs WHERE loop_run_id = ?
		AND status IN ('queued','claimed','starting','running','needs_attention')`, run.ID).Scan(&liveRecords); err != nil {
		t.Fatalf("count live Loop records error = %v", err)
	}
	if liveRecords != 0 {
		t.Fatalf("live Loop records = %d, want zero", liveRecords)
	}
	var staleLeases int
	if err := globalDB.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_runs WHERE loop_run_id = ?
		AND (claim_token IS NOT NULL OR lease_until IS NOT NULL OR heartbeat_at IS NOT NULL)`, run.ID).
		Scan(&staleLeases); err != nil {
		t.Fatalf("count stale Loop leases error = %v", err)
	}
	if staleLeases != 0 {
		t.Fatalf("stale Loop leases = %d, want zero", staleLeases)
	}
}

func seedLoopSettlementHierarchyForTest(
	t *testing.T,
	globalDB *GlobalDB,
	run looppkg.Run,
	prefix string,
	at time.Time,
) (string, string, string, string) {
	t.Helper()
	ctx := testutil.Context(t)
	coordinatorTaskID := loopCoordinatorTaskID(run.ID)
	liveTask := workspaceTaskRecordForTest(prefix+"-live-task", string(run.WorkspaceID))
	liveTask.ParentTaskID = coordinatorTaskID
	liveTask.Status = taskpkg.TaskStatusReady
	if err := globalDB.CreateTask(ctx, liveTask); err != nil {
		t.Fatalf("CreateTask(live settlement cell) error = %v", err)
	}
	liveRun := taskRunForTest(prefix+"-live-run", liveTask.ID)
	liveRun.LoopRunID = string(run.ID)
	liveRun.RunKind = taskpkg.RunKindWorker
	if err := globalDB.CreateTaskRun(ctx, liveRun); err != nil {
		t.Fatalf("CreateTaskRun(live settlement cell) error = %v", err)
	}
	const settlementClaimToken = "compozy_claim_settlement_secret"
	settlementClaimHash, err := taskpkg.ClaimTokenHash(settlementClaimToken)
	if err != nil {
		t.Fatalf("ClaimTokenHash(live settlement cell) error = %v", err)
	}
	if _, err := globalDB.db.ExecContext(ctx, `UPDATE task_runs SET claim_token = ?,
		claim_token_hash = ?, lease_until = ?, heartbeat_at = ? WHERE id = ?`,
		settlementClaimToken, settlementClaimHash, at.Add(time.Minute), at, liveRun.ID); err != nil {
		t.Fatalf("seed live settlement lease error = %v", err)
	}
	terminalTask := workspaceTaskRecordForTest(prefix+"-terminal-task", string(run.WorkspaceID))
	terminalTask.ParentTaskID = coordinatorTaskID
	terminalTask.Status = taskpkg.TaskStatusFailed
	terminalTask.ClosedAt = at
	if err := globalDB.CreateTask(ctx, terminalTask); err != nil {
		t.Fatalf("CreateTask(terminal settlement cell) error = %v", err)
	}
	return coordinatorTaskID, liveTask.ID, liveRun.ID, terminalTask.ID
}

func assertLoopPublicTerminalSettlementForTest(
	t *testing.T,
	globalDB *GlobalDB,
	run looppkg.Run,
	coordinatorTaskID string,
	liveTaskID string,
	liveRunID string,
	terminalTaskID string,
	wantCoordinator taskpkg.Status,
	wantDetail string,
) {
	t.Helper()
	ctx := testutil.Context(t)
	assertLoopSettlementFinalStateForTest(
		t,
		globalDB,
		run,
		coordinatorTaskID,
		liveTaskID,
		terminalTaskID,
		wantCoordinator,
		taskpkg.TaskStatusFailed,
	)
	liveRun, err := globalDB.GetTaskRun(ctx, liveRunID)
	if err != nil {
		t.Fatalf("GetTaskRun(live settlement cell) error = %v", err)
	}
	if liveRun.Status != taskpkg.TaskRunStatusCanceled {
		t.Fatalf("live cell run final status = %q, want canceled", liveRun.Status)
	}
	var payloadJSON []byte
	if err := globalDB.db.QueryRowContext(ctx, `SELECT payload_json FROM task_events
		WHERE task_id = ? AND event_type = 'task.status_changed'
		ORDER BY event_seq DESC LIMIT 1`, coordinatorTaskID).Scan(&payloadJSON); err != nil {
		t.Fatalf("read coordinator settlement event error = %v", err)
	}
	var payload taskStatusChangedEventPayload
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		t.Fatalf("decode coordinator settlement event error = %v", err)
	}
	if payload.TaskID != coordinatorTaskID || payload.ToStatus != string(wantCoordinator) ||
		payload.Reason != loopRunTerminalReason || payload.Detail != wantDetail ||
		payload.LoopRunID != string(run.ID) || payload.WorkspaceID != string(run.WorkspaceID) ||
		payload.ActorKind != string(taskpkg.ActorKindDaemon) || payload.ReleaseReason != loopRunTerminalReason {
		t.Fatalf("coordinator settlement payload = %#v, want structured terminal truth", payload)
	}
}

func countTaskStatusEventsForTest(t *testing.T, globalDB *GlobalDB, taskID string) int {
	t.Helper()
	var count int
	if err := globalDB.db.QueryRowContext(testutil.Context(t), `SELECT COUNT(*) FROM task_events
		WHERE task_id = ? AND event_type = 'task.status_changed'`, taskID).Scan(&count); err != nil {
		t.Fatalf("count task status events error = %v", err)
	}
	return count
}

func singleTaskStatusEventPayloadForTest(
	t *testing.T,
	globalDB *GlobalDB,
	taskID string,
) taskStatusChangedEventPayload {
	t.Helper()
	if got := countTaskStatusEventsForTest(t, globalDB, taskID); got != 1 {
		t.Fatalf("task %q status events = %d, want 1", taskID, got)
	}
	var raw []byte
	if err := globalDB.db.QueryRowContext(testutil.Context(t), `SELECT payload_json FROM task_events
		WHERE task_id = ? AND event_type = 'task.status_changed'`, taskID).Scan(&raw); err != nil {
		t.Fatalf("read task %q status event error = %v", taskID, err)
	}
	var payload taskStatusChangedEventPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode task %q status event error = %v", taskID, err)
	}
	return payload
}

// Invariant: a time-travel operation and its generation or child run commit atomically under one
// workspace-scoped intent key. The canonical GlobalDB Loop suite owns replay, lineage, seed, and coordinator truth.
func TestGlobalDBLoopTimeTravelShouldCommitOneAtomicOperation(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should reactivate a terminal run once and replay an explicit rerun key UT-071 UT-074 UT-076 UT-077",
		func(t *testing.T) {
			t.Parallel()

			globalDB := openLoopTestGlobalDB(t)
			ctx := testutil.Context(t)
			now := time.Date(2026, time.August, 16, 16, 0, 0, 0, time.UTC)
			source, err := globalDB.CreateLoopRunForStart(
				ctx, testLoopRun("looprun-rerun-atomic", now, looppkg.StatusRunning), dsl.ConcurrencyAllow,
			)
			if err != nil {
				t.Fatalf("CreateLoopRunForStart() error = %v", err)
			}
			if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_generation_outputs (
			loop_run_id, generation, node_id, item_index, status, attempt, epoch
		) VALUES (?, 1, 'finish', 0, 'failed', 1, 0)`, source.ID); err != nil {
				t.Fatalf("insert source output error = %v", err)
			}
			if _, err := globalDB.db.ExecContext(
				ctx,
				`UPDATE loop_runs SET status = 'done' WHERE id = ?`,
				source.ID,
			); err != nil {
				t.Fatalf("terminalize source error = %v", err)
			}
			if _, err := globalDB.db.ExecContext(ctx, `UPDATE task_runs SET status = 'completed', ended_at = ?
			WHERE loop_run_id = ? AND run_kind = 'coordinator'`, now.Add(30*time.Second), source.ID); err != nil {
				t.Fatalf("complete source coordinator error = %v", err)
			}
			source.Status = looppkg.StatusDone
			actor := operatorActorContextForTest("operator:rerun")
			generation := int64(2)
			rerunDigest := strings.Repeat("a", 64)
			request := looppkg.RerunStoreRequest{
				WorkspaceID: source.WorkspaceID,
				Source:      &source,
				NextOutputs: []looppkg.GenerationOutput{{
					Generation: 2, NodeID: "finish", Status: "pending", Attempt: 1,
				}},
				Intent: looppkg.GenerationIntent{
					Generation: 2, ParentGeneration: 1, Origin: looppkg.OriginOperatorRerun,
				},
				Operation: looppkg.TimeTravelOp{
					ID: "loopop-rerun-atomic", Kind: "rerun", IdempotencyKey: "rerun-key",
					RequestDigest: rerunDigest, SourceRunID: source.ID, SourceGeneration: new(int64),
					FromNode: "finish", Actor: actor, Reason: "retry failed finish", ResultRunID: source.ID,
					ResultGeneration: &generation, CreatedAt: now.Add(time.Minute),
				},
				RequestDigest: rerunDigest, IdempotencyKey: "rerun-key", At: now.Add(time.Minute),
			}
			*request.Operation.SourceGeneration = 1
			result, replayed, err := globalDB.CreateRerun(ctx, request)
			if err != nil || replayed || result.Generation != 2 || result.ParentGeneration != 1 {
				t.Fatalf("CreateRerun() = %#v replayed=%v error=%v", result, replayed, err)
			}
			persisted, err := globalDB.GetLoopRun(ctx, source.WorkspaceID, source.ID)
			if err != nil {
				t.Fatalf("GetLoopRun() error = %v", err)
			}
			if persisted.Status != looppkg.StatusRunning || persisted.Generation != 2 {
				t.Fatalf("reactivated run = %#v, want running generation 2", persisted)
			}
			if got := countCoordinatorTaskRunsForLoop(ctx, t, globalDB, source.ID); got != 2 {
				t.Fatalf("coordinator task runs = %d, want initial plus rerun", got)
			}
			replay, replayed, err := globalDB.CreateRerun(ctx, request)
			if err != nil || !replayed || replay.RunID != source.ID || replay.Generation != 2 {
				t.Fatalf("CreateRerun(replay) = %#v replayed=%v error=%v", replay, replayed, err)
			}
			var operations int
			if err := globalDB.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM loop_timetravel_ops
			WHERE workspace_id = ? AND idempotency_key = 'rerun-key'`, source.WorkspaceID).Scan(&operations); err != nil {
				t.Fatalf("count rerun operations error = %v", err)
			}
			if operations != 1 {
				t.Fatalf("rerun operation count = %d, want 1", operations)
			}
			mismatch := request
			mismatch.RequestDigest = strings.Repeat("b", 64)
			_, _, err = globalDB.CreateRerun(ctx, mismatch)
			if !errors.Is(err, looppkg.ErrTimeTravelKeyReuse) {
				t.Fatalf("CreateRerun(key reuse) error = %v, want ErrTimeTravelKeyReuse", err)
			}
		},
	)

	t.Run("Should seed and link a fork or abort the whole transaction UT-078 through UT-082b", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 16, 17, 0, 0, 0, time.UTC)
		source, err := globalDB.CreateLoopRunForStart(
			ctx, testLoopRun("looprun-fork-source", now, looppkg.StatusRunning), dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		payload := json.RawMessage(`{"result":"baseline"}`)
		outputRef := looppkg.OutputRefForPayload(payload)
		if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_output_blobs (
			output_ref, payload_json, byte_size, created_at, last_used_at
		) VALUES (?, ?, ?, ?, ?)`, outputRef, payload, len(payload), now, now); err != nil {
			t.Fatalf("insert source blob error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_generation_outputs (
			loop_run_id, generation, node_id, item_index, status, output_ref, attempt, epoch
		) VALUES (?, 1, 'finish', 0, 'succeeded', ?, 1, 0)`, source.ID, outputRef); err != nil {
			t.Fatalf("insert source output error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_runs SET status = 'done' WHERE id = ?`,
			source.ID,
		); err != nil {
			t.Fatalf("terminalize source error = %v", err)
		}
		source.Status = looppkg.StatusDone
		before, err := globalDB.GetLoopRun(ctx, source.WorkspaceID, source.ID)
		if err != nil {
			t.Fatalf("GetLoopRun(source before) error = %v", err)
		}
		child := testLoopRun("looprun-fork-child", now.Add(time.Minute), looppkg.StatusRunning)
		child.Generation = 1
		child.SetForkedFrom(&looppkg.ForkRef{RunID: source.ID, Generation: 1})
		child.DefinitionDigest = source.DefinitionDigest
		child.DefinitionSnapshot = append(json.RawMessage(nil), source.DefinitionSnapshot...)
		child.Inputs = map[string]any{"tasks": "override-ref"}
		sourceGeneration := int64(1)
		forkDigest := strings.Repeat("c", 64)
		request := looppkg.ForkStoreRequest{
			Source: &source, Child: &child,
			SeedOutputs: []looppkg.GenerationOutput{
				{
					Generation: 1, NodeID: "finish", OutputID: "report", ArtifactName: "report-final.md",
					Status: "succeeded", OutputRef: outputRef, Attempt: 1,
				},
				{
					Generation: 1, NodeID: "select", Status: "succeeded",
					OutputRef: `{"environment":"staging"}`, Attempt: 1,
				},
			},
			Concurrency: dsl.ConcurrencyAllow,
			Operation: looppkg.TimeTravelOp{
				ID: "loopop-fork-atomic", Kind: "fork", IdempotencyKey: "fork-key",
				RequestDigest: forkDigest, SourceRunID: source.ID, SourceGeneration: &sourceGeneration,
				Actor: operatorActorContextForTest("operator:fork"), Reason: "compare a safer input",
				ResultRunID: child.ID, CreatedAt: now.Add(time.Minute),
			},
			RequestDigest: forkDigest, IdempotencyKey: "fork-key", At: now.Add(time.Minute),
		}
		created, replayed, err := globalDB.CreateFork(ctx, request)
		if err != nil || replayed || created.ID != child.ID {
			t.Fatalf("CreateFork() = %#v replayed=%v error=%v", created, replayed, err)
		}
		persisted, err := globalDB.GetLoopRun(ctx, child.WorkspaceID, child.ID)
		if err != nil {
			t.Fatalf("GetLoopRun(child) error = %v", err)
		}
		persistedSource := persisted.ForkedFromSnapshot()
		childSource := child.ForkedFromSnapshot()
		if persisted.Generation != 1 || persistedSource == nil || childSource == nil ||
			*persistedSource != *childSource {
			t.Fatalf("persisted fork = %#v", persisted)
		}
		generations, err := globalDB.ListGenerations(ctx, string(child.WorkspaceID), string(child.ID))
		if err != nil {
			t.Fatalf("ListGenerations(child) error = %v", err)
		}
		if len(generations) != 2 || generations[0].Origin != looppkg.OriginForkSeed ||
			generations[1].Origin != looppkg.OriginInitial || generations[1].ParentGeneration != 1 {
			t.Fatalf("fork generations = %#v", generations)
		}
		outputs, err := globalDB.ListGenerationOutputs(ctx, child.WorkspaceID, child.ID, 1)
		if err != nil || len(outputs) != 2 || outputs[0].OutputID != "report" ||
			outputs[0].ArtifactName != "report-final.md" || outputs[0].OutputRef != outputRef ||
			outputs[1].OutputRef != `{"environment":"staging"}` {
			t.Fatalf("fork seed outputs = %#v error=%v", outputs, err)
		}
		if got := countCoordinatorTaskRunsForLoop(ctx, t, globalDB, child.ID); got != 1 {
			t.Fatalf("fork coordinator task runs = %d, want generation-2 reservation", got)
		}
		forks, err := globalDB.ListForks(ctx, source.WorkspaceID, source.ID)
		if err != nil || len(forks) != 1 || forks[0].RunID != child.ID || forks[0].Generation != 1 {
			t.Fatalf("ListForks() = %#v error=%v", forks, err)
		}
		after, err := globalDB.GetLoopRun(ctx, source.WorkspaceID, source.ID)
		if err != nil || !reflect.DeepEqual(before, after) {
			t.Fatalf("source after fork = %#v error=%v, want byte-identical projection %#v", after, err, before)
		}
		replay, replayed, err := globalDB.CreateFork(ctx, request)
		if err != nil || !replayed || replay.ID != child.ID {
			t.Fatalf("CreateFork(replay) = %#v replayed=%v error=%v", replay, replayed, err)
		}

		missing := request
		missing.Child = new(child)
		missing.Child.ID = "looprun-fork-missing-blob"
		missing.Child.SetForkedFrom(&looppkg.ForkRef{RunID: source.ID, Generation: 1})
		missing.SeedOutputs = []looppkg.GenerationOutput{{
			Generation: 1, NodeID: "finish", Status: "succeeded",
			OutputRef: "sha256:" + strings.Repeat("f", 64), Attempt: 1,
		}}
		missing.Operation.ID = "loopop-fork-missing-blob"
		missing.Operation.IdempotencyKey = ""
		missing.Operation.ResultRunID = missing.Child.ID
		missing.IdempotencyKey = ""
		_, _, err = globalDB.CreateFork(ctx, missing)
		if !errors.Is(err, looppkg.ErrOutputRefNotFound) {
			t.Fatalf("CreateFork(missing blob) error = %v, want ErrOutputRefNotFound", err)
		}
		if _, err := globalDB.GetLoopRun(
			ctx,
			missing.Child.WorkspaceID,
			missing.Child.ID,
		); !errors.Is(
			err,
			looppkg.ErrRunNotFound,
		) {
			t.Fatalf("GetLoopRun(partial missing-blob child) error = %v, want ErrRunNotFound", err)
		}
	})
}

// Invariant: one request row and its request wait form a single-winner, workspace-scoped lifecycle.
// The canonical GlobalDB Loop suite owns ordering, admission, idempotency, expiry, and cancellation truth.
func TestGlobalDBLoopRequestsShouldOwnOneAtomicLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should paginate pending lanes and admit one schema-valid answer", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 16, 13, 0, 0, 0, time.UTC)
		globalDB.LoopRepo.now = func() time.Time { return now.Add(10 * time.Minute) }
		run, err := globalDB.CreateLoopRunForStart(
			ctx, testLoopRun("looprun-request-admission", now, looppkg.StatusRunning), dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		schema := json.RawMessage(
			`{"type":"object","required":["environment"],"properties":{"environment":{"type":"string"}}}`,
		)
		for lane, expiry := range []time.Time{now.Add(2 * time.Hour), now.Add(time.Hour)} {
			seedLoopWaitCellForTest(
				t, globalDB, run, "select", lane, looppkg.NodeWaitKindRequest, int64(lane+1),
				schema, nil, &expiry, now.Add(time.Duration(lane)*time.Minute),
			)
			seedLoopRequestForTest(
				t,
				globalDB,
				run,
				"select",
				lane,
				schema,
				&expiry,
				now.Add(time.Duration(lane)*time.Minute),
			)
		}

		first, err := globalDB.ListRequests(ctx, run.WorkspaceID, looppkg.RequestQuery{Limit: 1})
		if err != nil {
			t.Fatalf("ListRequests(first) error = %v", err)
		}
		if len(first.Items) != 1 || first.Items[0].ItemIndex != 1 || first.Pending != 2 || first.NextCursor == "" {
			t.Fatalf("ListRequests(first) = %#v, want earliest expiry and cursor", first)
		}
		second, err := globalDB.ListRequests(ctx, run.WorkspaceID, looppkg.RequestQuery{
			Limit: 1, Cursor: first.NextCursor,
		})
		if err != nil {
			t.Fatalf("ListRequests(second) error = %v", err)
		}
		if len(second.Items) != 1 || second.Items[0].ItemIndex != 0 || second.NextCursor != "" {
			t.Fatalf("ListRequests(second) = %#v, want final lane without cursor", second)
		}
		detail, err := globalDB.GetRequest(ctx, run.WorkspaceID, looppkg.RequestRef{
			RunID: run.ID, NodeID: "select", ItemIndex: 1,
		}, true)
		if err != nil || !strings.Contains(string(detail.Context), `"full":"lane-1"`) {
			t.Fatalf("GetRequest(full) = %#v, error = %v", detail, err)
		}

		actor := operatorActorContextForTest("operator:one")
		_, err = globalDB.RespondRequest(ctx, looppkg.RespondInput{
			WorkspaceID: run.WorkspaceID, RunID: run.ID, NodeID: "select", ItemIndex: 1,
			Payload: json.RawMessage(`{}`), Actor: actor,
		})
		var validationReason *looppkg.ReasonError
		if !errors.Is(err, looppkg.ErrRequestValidationFailed) ||
			!errors.As(err, &validationReason) || validationReason.Code != looppkg.ReasonCodeRequestValidationFailed {
			t.Fatalf("RespondRequest(invalid) error = %#v", err)
		}
		pending, err := globalDB.GetRequest(ctx, run.WorkspaceID, looppkg.RequestRef{
			RunID: run.ID, NodeID: "select", ItemIndex: 1,
		}, false)
		if err != nil || pending.State != looppkg.RequestStatePending {
			t.Fatalf("GetRequest(after invalid) = %#v, error = %v", pending, err)
		}

		answer := json.RawMessage(`{"environment":"production"}`)
		result, err := globalDB.RespondRequest(ctx, looppkg.RespondInput{
			WorkspaceID: run.WorkspaceID, RunID: run.ID, NodeID: "select", ItemIndex: 1,
			Payload: answer, Actor: actor,
		})
		if err != nil || !result.Won || result.Request.State != looppkg.RequestStateAnswered ||
			result.Coordinator == nil {
			t.Fatalf("RespondRequest(valid) = %#v, error = %v", result, err)
		}
		var outputRef string
		if err := globalDB.db.QueryRowContext(ctx, `SELECT output_ref FROM loop_generation_outputs
			WHERE loop_run_id = ? AND generation = ? AND node_id = ? AND item_index = ?`,
			run.ID, result.Request.Generation, "select", 1).Scan(&outputRef); err != nil {
			t.Fatalf("load admitted request output ref error = %v", err)
		}
		payload, err := globalDB.GetGenerationOutputPayload(ctx, looppkg.GenerationOutputPayloadKey{
			WorkspaceID: run.WorkspaceID,
			RunID:       run.ID,
			Generation:  result.Request.Generation,
			NodeID:      "select",
			ItemIndex:   1,
			OutputRef:   outputRef,
		})
		if err != nil || string(payload) != string(answer) {
			t.Fatalf("admitted request output = %s, error = %v, want %s", payload, err, answer)
		}
		replay, err := globalDB.RespondRequest(ctx, looppkg.RespondInput{
			WorkspaceID: run.WorkspaceID, RunID: run.ID, NodeID: "select", ItemIndex: 1,
			Payload: answer, Actor: actor,
		})
		if err != nil || replay.Won {
			t.Fatalf("RespondRequest(replay) = %#v, error = %v, want idempotent echo", replay, err)
		}
		_, err = globalDB.RespondRequest(ctx, looppkg.RespondInput{
			WorkspaceID: run.WorkspaceID, RunID: run.ID, NodeID: "select", ItemIndex: 1,
			Payload: answer, Actor: operatorActorContextForTest("operator:two"),
		})
		var conflictReason *looppkg.ReasonError
		if !errors.Is(err, looppkg.ErrRequestAlreadyAnswered) ||
			!errors.As(err, &conflictReason) ||
			conflictReason.Meta[looppkg.ReasonMetaRecordedDecision] != looppkg.RequestDecisionRespond {
			t.Fatalf("RespondRequest(loser) error = %#v", err)
		}
	})

	t.Run("Should admit each review decision without starting the action clock early", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 16, 14, 30, 0, 0, time.UTC)
		globalDB.LoopRepo.now = func() time.Time { return now.Add(time.Minute) }
		editSchema := json.RawMessage(`{"type":"object","required":["tag"],"properties":{"tag":{"type":"string"}}}`)
		respondSchema := json.RawMessage(
			`{"type":"object","required":["release_url"],"properties":{"release_url":{"type":"string"}}}`,
		)
		proposed := json.RawMessage(`{"tag":"v1"}`)

		seedReview := func(ctx context.Context, id string) looppkg.Run {
			t.Helper()
			run, err := globalDB.CreateLoopRunForStart(
				ctx, testLoopRun(id, now, looppkg.StatusRunning), dsl.ConcurrencyAllow,
			)
			if err != nil {
				t.Fatalf("CreateLoopRunForStart(%s) error = %v", id, err)
			}
			seedLoopWaitCellForTest(
				t, globalDB, run, "publish", 0, looppkg.NodeWaitKindRequest, 1,
				nil, nil, nil, now,
			)
			if _, err := globalDB.db.ExecContext(ctx, `UPDATE loop_generation_outputs
				SET first_scheduled_at = NULL WHERE loop_run_id = ? AND generation = 1
				AND node_id = 'publish' AND item_index = 0`, run.ID); err != nil {
				t.Fatalf("clear review scheduling clock error = %v", err)
			}
			contextPayload := json.RawMessage(`{}`)
			contextRef := looppkg.OutputRefForPayload(contextPayload)
			proposedRef := looppkg.OutputRefForPayload(proposed)
			for ref, payload := range map[string]json.RawMessage{
				contextRef: contextPayload, proposedRef: proposed,
			} {
				if err := storepkg.UpsertLoopOutputBlob(ctx, globalDB.db, ref, payload, now); err != nil {
					t.Fatalf("UpsertLoopOutputBlob(%s) error = %v", ref, err)
				}
			}
			if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_requests (
				workspace_id, loop_run_id, generation, node_id, item_index, kind, state,
				prompt, context_preview_json, context_ref, edit_schema_json, respond_schema_json,
				decisions_json, proposed_ref, proposed_preview_json, opened_at
			) VALUES (?, ?, 1, 'publish', 0, 'review', 'pending', 'Review publish', '{}', ?, ?, ?,
				'["approve","edit","reject","respond"]', ?, ?, ?)`, run.WorkspaceID, run.ID,
				contextRef, string(editSchema), string(respondSchema), proposedRef, string(proposed), now); err != nil {
				t.Fatalf("insert review request error = %v", err)
			}
			return run
		}

		invalidRun := seedReview(ctx, "looprun-review-invalid-edit")
		_, err := globalDB.RespondRequest(ctx, looppkg.RespondInput{
			WorkspaceID: invalidRun.WorkspaceID, RunID: invalidRun.ID, NodeID: "publish",
			Decision: looppkg.RequestDecisionEdit, Payload: json.RawMessage(`{}`),
			Actor: operatorActorContextForTest("operator:review"), RequestKind: looppkg.RequestKindReview,
		})
		if !errors.Is(err, looppkg.ErrRequestValidationFailed) {
			t.Fatalf("RespondRequest(invalid edit) error = %v, want validation failure", err)
		}
		pending, err := globalDB.GetRequest(ctx, invalidRun.WorkspaceID, looppkg.RequestRef{
			RunID: invalidRun.ID, NodeID: "publish",
		}, false)
		if err != nil || pending.State != looppkg.RequestStatePending {
			t.Fatalf("review after invalid edit = %#v, error = %v", pending, err)
		}

		for _, tt := range []struct {
			name          string
			decision      string
			payload       json.RawMessage
			rejectRoute   looppkg.NodeID
			wantStatus    string
			wantPayload   json.RawMessage
			wantRefPrefix string
		}{
			{name: "approve", decision: looppkg.RequestDecisionApprove, wantStatus: "pending", wantPayload: proposed},
			{name: "edit", decision: looppkg.RequestDecisionEdit, payload: json.RawMessage(`{"tag":"v2"}`),
				wantStatus: "pending", wantPayload: json.RawMessage(`{"tag":"v2"}`)},
			{name: "respond", decision: looppkg.RequestDecisionRespond,
				payload:    json.RawMessage(`{"release_url":"https://example.test/v2"}`),
				wantStatus: "succeeded", wantPayload: json.RawMessage(`{"release_url":"https://example.test/v2"}`)},
			{name: "reject route", decision: looppkg.RequestDecisionReject, rejectRoute: "halt",
				wantStatus: "succeeded", wantRefPrefix: reviewRejectedRouteOutputRefPrefix + "halt"},
			{name: "reject failure", decision: looppkg.RequestDecisionReject,
				wantStatus: "failed", wantRefPrefix: `{"kind":"action_failure","code":"quality_rejection"`},
		} {
			t.Run("Should admit "+tt.name, func(t *testing.T) {
				t.Parallel()

				ctx := testutil.Context(t)
				run := seedReview(ctx, "looprun-review-"+strings.ReplaceAll(tt.name, " ", "-"))
				result, err := globalDB.RespondRequest(ctx, looppkg.RespondInput{
					WorkspaceID: run.WorkspaceID, RunID: run.ID, NodeID: "publish",
					Decision: tt.decision, Payload: tt.payload, RejectRoute: tt.rejectRoute,
					Actor: operatorActorContextForTest("operator:review"), RequestKind: looppkg.RequestKindReview,
				})
				if err != nil || !result.Won || result.Coordinator == nil {
					t.Fatalf("RespondRequest(%s) = %#v, error = %v", tt.name, result, err)
				}
				var status, outputRef string
				var firstScheduled sql.NullTime
				if err := globalDB.db.QueryRowContext(ctx, `SELECT status, output_ref, first_scheduled_at
					FROM loop_generation_outputs WHERE loop_run_id = ? AND generation = 1
					AND node_id = 'publish' AND item_index = 0`, run.ID).
					Scan(&status, &outputRef, &firstScheduled); err != nil {
					t.Fatalf("load review output error = %v", err)
				}
				if status != tt.wantStatus || firstScheduled.Valid {
					t.Fatalf("review output status=%q first_scheduled=%v, want %q and unset clock",
						status, firstScheduled, tt.wantStatus)
				}
				if tt.wantRefPrefix != "" && !strings.HasPrefix(outputRef, tt.wantRefPrefix) {
					t.Fatalf("review output_ref = %q, want prefix %q", outputRef, tt.wantRefPrefix)
				}
				if len(tt.wantPayload) > 0 {
					payload, err := globalDB.GetGenerationOutputPayload(ctx, looppkg.GenerationOutputPayloadKey{
						WorkspaceID: run.WorkspaceID, RunID: run.ID, Generation: 1,
						NodeID: "publish", OutputRef: outputRef,
					})
					if err != nil || string(payload) != string(tt.wantPayload) {
						t.Fatalf("review admitted payload = %s, error = %v, want %s", payload, err, tt.wantPayload)
					}
				}
			})
		}
	})

	t.Run("Should serialize concurrent responders and cancellation", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 16, 13, 30, 0, 0, time.UTC)
		globalDB.LoopRepo.now = func() time.Time { return now.Add(time.Minute) }
		schema := json.RawMessage(`{"type":"object","required":["value"],"properties":{"value":{"type":"string"}}}`)
		seedRun := func(id string) looppkg.Run {
			t.Helper()
			run, err := globalDB.CreateLoopRunForStart(
				ctx, testLoopRun(id, now, looppkg.StatusRunning), dsl.ConcurrencyAllow,
			)
			if err != nil {
				t.Fatalf("CreateLoopRunForStart(%s) error = %v", id, err)
			}
			expires := now.Add(time.Hour)
			seedLoopWaitCellForTest(
				t,
				globalDB,
				run,
				"select",
				0,
				looppkg.NodeWaitKindRequest,
				1,
				schema,
				nil,
				&expires,
				now,
			)
			seedLoopRequestForTest(t, globalDB, run, "select", 0, schema, &expires, now)
			return run
		}

		respondRun := seedRun("looprun-request-race-respond")
		start := make(chan struct{})
		respondErrors := make(chan error, 2)
		var responders sync.WaitGroup
		for index := range 2 {
			responders.Add(1)
			go func(index int) {
				defer responders.Done()
				<-start
				_, err := globalDB.RespondRequest(ctx, looppkg.RespondInput{
					WorkspaceID: respondRun.WorkspaceID, RunID: respondRun.ID, NodeID: "select",
					Payload: json.RawMessage(fmt.Sprintf(`{"value":"answer-%d"}`, index)),
					Actor:   operatorActorContextForTest(fmt.Sprintf("operator:%d", index)),
				})
				respondErrors <- err
			}(index)
		}
		close(start)
		responders.Wait()
		close(respondErrors)
		wins := 0
		conflicts := 0
		for err := range respondErrors {
			switch {
			case err == nil:
				wins++
			case errors.Is(err, looppkg.ErrRequestAlreadyAnswered):
				conflicts++
			default:
				t.Fatalf("concurrent RespondRequest() error = %v", err)
			}
		}
		if wins != 1 || conflicts != 1 {
			t.Fatalf("respond race wins/conflicts = %d/%d, want 1/1", wins, conflicts)
		}

		cancelRun := seedRun("looprun-request-race-cancel")
		start = make(chan struct{})
		raceErrors := make(chan error, 2)
		var race sync.WaitGroup
		race.Add(2)
		go func() {
			defer race.Done()
			<-start
			_, err := globalDB.RespondRequest(ctx, looppkg.RespondInput{
				WorkspaceID: cancelRun.WorkspaceID, RunID: cancelRun.ID, NodeID: "select",
				Payload: json.RawMessage(`{"value":"answer"}`),
				Actor:   operatorActorContextForTest("operator:respond"),
			})
			if err != nil && !errors.Is(err, looppkg.ErrRequestCanceled) {
				raceErrors <- err
				return
			}
			raceErrors <- nil
		}()
		go func() {
			defer race.Done()
			<-start
			_, err := globalDB.RequestRunCancellation(ctx, looppkg.CancellationMutation{
				WorkspaceID: cancelRun.WorkspaceID,
				RunID:       cancelRun.ID,
				Kind:        looppkg.RunCancelCancel,
				Reason:      "race",
				Actor:       operatorActorContextForTest("operator:cancel"),
				RequestedAt: now.Add(2 * time.Minute),
			})
			raceErrors <- err
		}()
		close(start)
		race.Wait()
		close(raceErrors)
		for err := range raceErrors {
			if err != nil {
				t.Fatalf("respond/cancel race error = %v", err)
			}
		}
		request, err := globalDB.GetRequest(ctx, cancelRun.WorkspaceID, looppkg.RequestRef{
			RunID: cancelRun.ID, NodeID: "select",
		}, false)
		if err != nil ||
			(request.State != looppkg.RequestStateAnswered && request.State != looppkg.RequestStateCanceled) {
			t.Fatalf("respond/cancel race request = %#v, error = %v", request, err)
		}
		var answeredEvents, canceledEvents int
		if err := globalDB.db.QueryRowContext(ctx, `SELECT
			SUM(CASE WHEN kind = 'request_answered' THEN 1 ELSE 0 END),
			SUM(CASE WHEN kind = 'request_canceled' THEN 1 ELSE 0 END)
			FROM loop_run_events WHERE loop_run_id = ?`, cancelRun.ID).Scan(&answeredEvents, &canceledEvents); err != nil {
			t.Fatalf("count request race events error = %v", err)
		}
		if answeredEvents+canceledEvents != 1 {
			t.Fatalf("respond/cancel race events answered/canceled = %d/%d, want one terminal request event",
				answeredEvents, canceledEvents)
		}
	})

	t.Run("Should cancel a pending request in the run cancellation transaction", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 16, 14, 0, 0, 0, time.UTC)
		run, err := globalDB.CreateLoopRunForStart(
			ctx, testLoopRun("looprun-request-cancel", now, looppkg.StatusRunning), dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		schema := json.RawMessage(`{"type":"object"}`)
		expires := now.Add(time.Hour)
		seedLoopWaitCellForTest(
			t,
			globalDB,
			run,
			"select",
			0,
			looppkg.NodeWaitKindRequest,
			1,
			schema,
			nil,
			&expires,
			now,
		)
		seedLoopRequestForTest(t, globalDB, run, "select", 0, schema, &expires, now)

		_, err = globalDB.RequestRunCancellation(ctx, looppkg.CancellationMutation{
			WorkspaceID: run.WorkspaceID,
			RunID:       run.ID,
			Kind:        looppkg.RunCancelCancel,
			Reason:      "operator canceled",
			Actor:       operatorActorContextForTest("operator:cancel"),
			RequestedAt: now.Add(time.Minute),
		})
		if err != nil {
			t.Fatalf("RequestRunCancellation() error = %v", err)
		}
		request, err := globalDB.GetRequest(ctx, run.WorkspaceID, looppkg.RequestRef{
			RunID: run.ID, NodeID: "select",
		}, false)
		if err != nil || request.State != looppkg.RequestStateCanceled {
			t.Fatalf("GetRequest(canceled) = %#v, error = %v", request, err)
		}
		var eventCount int
		if err := globalDB.db.QueryRowContext(ctx, `SELECT count(*) FROM loop_run_events
			WHERE loop_run_id = ? AND kind = ?`, run.ID, loopRunEventRequestCanceled).Scan(&eventCount); err != nil {
			t.Fatalf("count request_canceled events error = %v", err)
		}
		if eventCount != 1 {
			t.Fatalf("request_canceled events = %d, want 1", eventCount)
		}
		page, err := globalDB.ListRequests(ctx, run.WorkspaceID, looppkg.RequestQuery{Limit: 10})
		if err != nil || page.Pending != 0 || len(page.Items) != 0 {
			t.Fatalf("ListRequests(after cancel) = %#v, error = %v", page, err)
		}
		_, err = globalDB.RespondRequest(ctx, looppkg.RespondInput{
			WorkspaceID: run.WorkspaceID, RunID: run.ID, NodeID: "select",
			Payload: json.RawMessage(`{}`), Actor: operatorActorContextForTest("operator:late"),
		})
		var canceledReason *looppkg.ReasonError
		if !errors.Is(err, looppkg.ErrRequestCanceled) || !errors.As(err, &canceledReason) ||
			canceledReason.Code != looppkg.ReasonCodeRequestCanceled {
			t.Fatalf("RespondRequest(canceled) error = %#v", err)
		}
	})

	t.Run("Should expire one request exactly once", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 16, 15, 0, 0, 0, time.UTC)
		run, err := globalDB.CreateLoopRunForStart(
			ctx, testLoopRun("looprun-request-expiry", now, looppkg.StatusRunning), dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		schema := json.RawMessage(`{"type":"object"}`)
		expires := now.Add(time.Minute)
		seedLoopWaitCellForTest(
			t,
			globalDB,
			run,
			"select",
			0,
			looppkg.NodeWaitKindRequest,
			1,
			schema,
			nil,
			&expires,
			now,
		)
		seedLoopRequestForTest(t, globalDB, run, "select", 0, schema, &expires, now)
		due := &dueWaitEscalation{run: run, wait: looppkg.NodeWait{
			LoopRunID: run.ID, Generation: 1, NodeID: "select", ItemIndex: 0, IssuedEpoch: 1,
		}}
		if err := expireLoopRequestWithExecutor(ctx, globalDB.db, due, expires); err != nil {
			t.Fatalf("expireLoopRequestWithExecutor() error = %v", err)
		}
		if err := expireLoopRequestWithExecutor(
			ctx,
			globalDB.db,
			due,
			expires,
		); !errors.Is(
			err,
			looppkg.ErrTransitionConflict,
		) {
			t.Fatalf("expireLoopRequestWithExecutor(replay) error = %v, want conflict", err)
		}
		var eventCount int
		if err := globalDB.db.QueryRowContext(ctx, `SELECT count(*) FROM loop_run_events
			WHERE loop_run_id = ? AND kind = ?`, run.ID, loopRunEventRequestExpired).Scan(&eventCount); err != nil {
			t.Fatalf("count request_expired events error = %v", err)
		}
		if eventCount != 1 {
			t.Fatalf("request_expired events = %d, want 1", eventCount)
		}
		_, err = globalDB.RespondRequest(ctx, looppkg.RespondInput{
			WorkspaceID: run.WorkspaceID, RunID: run.ID, NodeID: "select",
			Payload: json.RawMessage(`{}`), Actor: operatorActorContextForTest("operator:late"),
		})
		if !errors.Is(err, looppkg.ErrRequestExpired) {
			t.Fatalf("RespondRequest(expired) error = %v", err)
		}
	})
}

// Invariant: amendments are append-only overlays; recorded generation rows never change and every overlay blob remains rooted.
// The canonical GlobalDB Loop suite owns amendment sequencing, guards, overlay reads, provenance, and blob durability.
func TestGlobalDBLoopAmendmentsShouldPreserveRecordedOutputs(t *testing.T) {
	t.Parallel()

	globalDB := openLoopTestGlobalDB(t)
	ctx := testutil.Context(t)
	now := time.Date(2026, time.August, 16, 15, 0, 0, 0, time.UTC)
	run, err := globalDB.CreateLoopRunForStart(
		ctx, testLoopRun("looprun-amendments", now, looppkg.StatusRunning), dsl.ConcurrencyAllow,
	)
	if err != nil {
		t.Fatalf("CreateLoopRunForStart() error = %v", err)
	}
	original := json.RawMessage(`{"value":"recorded"}`)
	originalRef := looppkg.OutputRefForPayload(original)
	if err := storepkg.UpsertLoopOutputBlob(ctx, globalDB.db, originalRef, original, now); err != nil {
		t.Fatalf("UpsertLoopOutputBlob(original) error = %v", err)
	}
	if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_generation_outputs (
		loop_run_id, generation, node_id, item_index, status, output_ref, attempt, epoch
	) VALUES (?, 1, 'repair', 2, 'succeeded', ?, 1, 7)`, run.ID, originalRef); err != nil {
		t.Fatalf("insert amendment output error = %v", err)
	}
	if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_node_lane_pauses (
		workspace_id, loop_run_id, generation, node_id, item_index, actor_kind, actor_id, reason, mode, requested_at
	) VALUES (?, ?, 1, 'repair', 2, 'human', 'operator:one', 'inspect', 'drain', ?)`,
		run.WorkspaceID, run.ID, now); err != nil {
		t.Fatalf("insert amendment lane pause error = %v", err)
	}
	rowSnapshot := func() string {
		t.Helper()
		var snapshot string
		if err := globalDB.db.QueryRowContext(ctx, `SELECT json_object(
			'loop_run_id', loop_run_id, 'generation', generation, 'node_id', node_id,
			'item_index', item_index, 'status', status, 'output_ref', output_ref,
			'task_run_id', task_run_id, 'attempt', attempt, 'next_attempt_at', next_attempt_at,
			'first_scheduled_at', first_scheduled_at, 'epoch', epoch
		) FROM loop_generation_outputs WHERE loop_run_id = ? AND generation = 1
		AND node_id = 'repair' AND item_index = 2`, run.ID).Scan(&snapshot); err != nil {
			t.Fatalf("snapshot recorded output error = %v", err)
		}
		return snapshot
	}
	before := rowSnapshot()
	schema := json.RawMessage(`{"type":"object","required":["value"],"properties":{"value":{"type":"string"}}}`)
	actor := operatorActorContextForTest("operator:one")
	firstPayload := json.RawMessage(`{"value":"first repair"}`)
	first, err := globalDB.AmendNodeOutput(ctx, looppkg.AmendInput{
		WorkspaceID: run.WorkspaceID,
		RunID:       run.ID,
		Generation:  1,
		NodeID:      "repair",
		ItemIndex:   2,
		Payload:     firstPayload,
		Schema:      schema,
		Reason:      "correct evidence",
		Actor:       actor,
		RequestedAt: now.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("AmendNodeOutput(first) error = %v", err)
	}
	secondPayload := json.RawMessage(`{"value":"second repair"}`)
	second, err := globalDB.AmendNodeOutput(ctx, looppkg.AmendInput{
		WorkspaceID: run.WorkspaceID, RunID: run.ID, Generation: 1, NodeID: "repair", ItemIndex: 2,
		Payload: secondPayload, Schema: schema, Reason: "final evidence", Actor: actor,
		RequestedAt: now.Add(2 * time.Minute),
	})
	if err != nil {
		t.Fatalf("AmendNodeOutput(second) error = %v", err)
	}
	if after := rowSnapshot(); after != before {
		t.Fatalf("recorded generation output changed:\nbefore=%s\nafter=%s", before, after)
	}
	firstMatches := first.Sequence == 1 && first.OriginalRef == originalRef &&
		first.AmendedRef == looppkg.OutputRefForPayload(firstPayload)
	secondMatches := second.Sequence == 2 && second.OriginalRef == first.AmendedRef &&
		second.AmendedRef == looppkg.OutputRefForPayload(secondPayload)
	if !firstMatches || !secondMatches {
		t.Fatalf("amendment chain first=%#v second=%#v", first, second)
	}
	view, err := globalDB.ApplyGenerationOutputOverlays(ctx, run.WorkspaceID, run.ID, 1, []looppkg.GenerationOutput{{
		Generation: 1, NodeID: "repair", ItemIndex: 2, Status: "succeeded", OutputRef: originalRef,
	}})
	if err != nil || len(view) != 1 || view[0].OutputRef != second.AmendedRef {
		t.Fatalf("ApplyGenerationOutputOverlays() = %#v, error = %v", view, err)
	}
	effective, err := globalDB.GetGenerationOutputPayload(ctx, looppkg.GenerationOutputPayloadKey{
		WorkspaceID: run.WorkspaceID, RunID: run.ID, Generation: 1, NodeID: "repair",
		ItemIndex: 2, OutputRef: view[0].OutputRef,
	})
	if err != nil || string(effective) != string(secondPayload) {
		t.Fatalf("GetGenerationOutputPayload(amended) = %s, error = %v, want %s", effective, err, secondPayload)
	}
	amendments, err := globalDB.ListNodeAmendments(ctx, run.WorkspaceID, run.ID)
	if err != nil || len(amendments) != 2 || string(amendments[0].Original) != string(original) ||
		string(amendments[1].Amended) != string(secondPayload) {
		t.Fatalf("ListNodeAmendments() = %#v, error = %v", amendments, err)
	}

	t.Run("Should serialize concurrent amendments without sequence collisions", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 16, 15, 20, 0, 0, time.UTC)
		run, err := globalDB.CreateLoopRunForStart(
			ctx, testLoopRun("looprun-amendment-race", now, looppkg.StatusRunning), dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		original := json.RawMessage(`{"value":"recorded"}`)
		originalRef := looppkg.OutputRefForPayload(original)
		if err := storepkg.UpsertLoopOutputBlob(ctx, globalDB.db, originalRef, original, now); err != nil {
			t.Fatalf("UpsertLoopOutputBlob(original) error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_generation_outputs (
			loop_run_id, generation, node_id, item_index, status, output_ref, attempt, epoch
		) VALUES (?, 1, 'repair', 0, 'succeeded', ?, 1, 1)`, run.ID, originalRef); err != nil {
			t.Fatalf("insert amendment race output error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_node_lane_pauses (
			workspace_id, loop_run_id, generation, node_id, item_index, actor_kind, actor_id, reason, mode, requested_at
		) VALUES (?, ?, 1, 'repair', 0, 'human', 'operator:race', 'inspect', 'drain', ?)`,
			run.WorkspaceID, run.ID, now); err != nil {
			t.Fatalf("insert amendment race pause error = %v", err)
		}
		schema := json.RawMessage(`{"type":"object","required":["value"],"properties":{"value":{"type":"string"}}}`)
		results := make([]looppkg.NodeAmendment, 2)
		errorsByIndex := make([]error, 2)
		start := make(chan struct{})
		var group sync.WaitGroup
		group.Add(2)
		for index := range results {
			go func(index int) {
				defer group.Done()
				<-start
				results[index], errorsByIndex[index] = globalDB.AmendNodeOutput(
					context.Background(),
					looppkg.AmendInput{
						WorkspaceID: run.WorkspaceID,
						RunID:       run.ID,
						Generation:  1,
						NodeID:      "repair",
						Payload:     json.RawMessage(fmt.Sprintf(`{"value":"repair-%d"}`, index)),
						Schema:      schema,
						Actor: operatorActorContextForTest(
							"operator:race",
						),
						RequestedAt: now.Add(time.Duration(index+1) * time.Second),
					},
				)
			}(index)
		}
		close(start)
		group.Wait()
		for index, err := range errorsByIndex {
			if err != nil {
				t.Fatalf("AmendNodeOutput(race %d) error = %v", index, err)
			}
		}
		sequences := []int{results[0].Sequence, results[1].Sequence}
		slices.Sort(sequences)
		if !slices.Equal(sequences, []int{1, 2}) {
			t.Fatalf("concurrent amendment sequences = %#v, want 1 and 2", sequences)
		}
	})

	t.Run("Should reject a parked cell without a settled output", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 16, 15, 25, 0, 0, time.UTC)
		run, err := globalDB.CreateLoopRunForStart(
			ctx, testLoopRun("looprun-amendment-no-output", now, looppkg.StatusRunning), dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_generation_outputs (
			loop_run_id, generation, node_id, item_index, status, attempt, epoch
		) VALUES (?, 1, 'repair', 0, 'paused', 1, 1)`, run.ID); err != nil {
			t.Fatalf("insert no-output amendment cell error = %v", err)
		}
		_, err = globalDB.AmendNodeOutput(ctx, looppkg.AmendInput{
			WorkspaceID: run.WorkspaceID, RunID: run.ID, Generation: 1, NodeID: "repair",
			Payload: json.RawMessage(`{"value":"repair"}`), Schema: json.RawMessage(`{"type":"object"}`),
			Actor: operatorActorContextForTest("operator:no-output"), RequestedAt: now.Add(time.Minute),
		})
		if !errors.Is(err, looppkg.ErrAmendNoOutput) {
			t.Fatalf("AmendNodeOutput(no output) error = %v, want ErrAmendNoOutput", err)
		}
	})

	t.Run("Should not reuse a lane pause from another generation", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 16, 15, 24, 0, 0, time.UTC)
		run, err := globalDB.CreateLoopRunForStart(
			ctx, testLoopRun("looprun-amendment-generation", now, looppkg.StatusRunning), dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		payload := json.RawMessage(`{"value":"recorded"}`)
		outputRef := looppkg.OutputRefForPayload(payload)
		if err := storepkg.UpsertLoopOutputBlob(ctx, globalDB.db, outputRef, payload, now); err != nil {
			t.Fatalf("UpsertLoopOutputBlob() error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_generation_outputs (
			loop_run_id, generation, node_id, item_index, status, output_ref, attempt, epoch
		) VALUES (?, 2, 'repair', 0, 'succeeded', ?, 1, 1)`, run.ID, outputRef); err != nil {
			t.Fatalf("insert generation output error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_node_lane_pauses (
			workspace_id, loop_run_id, generation, node_id, item_index, actor_kind, actor_id, reason, mode, requested_at
		) VALUES (?, ?, 1, 'repair', 0, 'human', 'operator:one', 'inspect', 'drain', ?)`,
			run.WorkspaceID, run.ID, now); err != nil {
			t.Fatalf("insert prior-generation lane pause error = %v", err)
		}

		_, err = globalDB.AmendNodeOutput(ctx, looppkg.AmendInput{
			WorkspaceID: run.WorkspaceID, RunID: run.ID, Generation: 2, NodeID: "repair",
			Payload: json.RawMessage(`{"value":"amended"}`), Schema: json.RawMessage(`{"type":"object"}`),
			Actor: operatorActorContextForTest("operator:one"), RequestedAt: now.Add(time.Minute),
		})
		if !errors.Is(err, looppkg.ErrAmendNotParked) {
			t.Fatalf("AmendNodeOutput(cross-generation pause) error = %v, want ErrAmendNotParked", err)
		}
	})

	t.Run("Should reject invalid payloads and targets that are not parked", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)

		_, err := globalDB.AmendNodeOutput(ctx, looppkg.AmendInput{
			WorkspaceID: run.WorkspaceID,
			RunID:       run.ID,
			Generation:  1,
			NodeID:      "repair",
			ItemIndex:   2,
			Payload: json.RawMessage(
				`{"value":1}`,
			),
			Schema:      schema,
			Actor:       actor,
			RequestedAt: now.Add(3 * time.Minute),
		})
		if !errors.Is(err, looppkg.ErrRequestValidationFailed) {
			t.Fatalf("AmendNodeOutput(invalid payload) error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(ctx, `DELETE FROM loop_node_lane_pauses
			WHERE loop_run_id = ? AND generation = 1 AND node_id = 'repair' AND item_index = 2`, run.ID); err != nil {
			t.Fatalf("delete lane pause error = %v", err)
		}
		_, err = globalDB.AmendNodeOutput(ctx, looppkg.AmendInput{
			WorkspaceID: run.WorkspaceID, RunID: run.ID, Generation: 1, NodeID: "repair", ItemIndex: 2,
			Payload: firstPayload, Schema: schema, Actor: actor, RequestedAt: now.Add(4 * time.Minute),
		})
		if !errors.Is(err, looppkg.ErrAmendNotParked) {
			t.Fatalf("AmendNodeOutput(unparked) error = %v", err)
		}
	})

	t.Run("Should keep amendment refs while reclaiming a true orphan", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)

		orphan := json.RawMessage(`{"value":"orphan"}`)
		orphanRef := looppkg.OutputRefForPayload(orphan)
		if err := storepkg.UpsertLoopOutputBlob(ctx, globalDB.db, orphanRef, orphan, now); err != nil {
			t.Fatalf("UpsertLoopOutputBlob(orphan) error = %v", err)
		}
		if err := sweepOrphanedLoopOutputBlobsWithExecutor(ctx, globalDB.db); err != nil {
			t.Fatalf("sweepOrphanedLoopOutputBlobsWithExecutor() error = %v", err)
		}
		for _, ref := range []string{originalRef, first.AmendedRef, second.AmendedRef} {
			if _, err := getLoopOutputByRefWithExecutor(ctx, globalDB.db, ref); err != nil {
				t.Fatalf("rooted amendment ref %s error = %v", ref, err)
			}
		}
		if _, err := getLoopOutputByRefWithExecutor(
			ctx,
			globalDB.db,
			orphanRef,
		); !errors.Is(
			err,
			looppkg.ErrOutputRefNotFound,
		) {
			t.Fatalf("orphan lookup error = %v, want ErrOutputRefNotFound", err)
		}
	})
}

// Invariant: node inventories expose one workspace-scoped, stable keyset order with exactly
// one detail matching the requested state. This GlobalDB Loop suite owns inventory truth.
func TestGlobalDBLoopNodeInventoryShouldPaginateAndIsolateStateTruth(t *testing.T) {
	t.Parallel()

	globalDB := openLoopTestGlobalDB(t, "ws-inventory", "ws-foreign")
	ctx := testutil.Context(t)
	now := time.Date(2026, time.August, 3, 8, 0, 0, 0, time.UTC)
	createRun := func(id string, workspaceID looppkg.WorkspaceID, loopName string) looppkg.Run {
		t.Helper()
		seed := testLoopRun(id, now, looppkg.StatusRunning)
		seed.WorkspaceID = workspaceID
		seed.LoopName = loopName
		created, err := globalDB.CreateLoopRunForStart(ctx, seed, dsl.ConcurrencyAllow)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart(%s) error = %v", id, err)
		}
		return created
	}
	runA := createRun("looprun-inventory-a", "ws-inventory", "alpha")
	runB := createRun("looprun-inventory-b", "ws-inventory", "beta")
	foreign := createRun("looprun-inventory-c", "ws-foreign", "alpha")
	waitAt := now.Add(-3 * time.Hour)
	seedLoopWaitCellForTest(
		t, globalDB, runA, "wait-a", 0, "event", 1,
		json.RawMessage(`{"type":"object"}`), nil, nil, waitAt,
	)
	seedLoopWaitCellForTest(t, globalDB, runA, "wait-b", 0, "event", 2, nil, nil, nil, waitAt)
	seedLoopWaitCellForTest(t, globalDB, runB, "wait-c", 0, "event", 3, nil, nil, nil, waitAt)
	seedLoopWaitCellForTest(t, globalDB, foreign, "wait-d", 0, "event", 4, nil, nil, nil, waitAt)

	if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_node_controls (
		loop_run_id, node_id, quarantined, quarantine_entry_json, quarantined_at, revision, updated_at
	) VALUES (?, 'quarantine', 1, '{"node_id":"quarantine"}', ?, 2, ?)`, runA.ID, now.Add(-2*time.Hour), now.Add(-2*time.Hour)); err != nil {
		t.Fatalf("insert quarantined inventory row error = %v", err)
	}
	if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_node_controls (
		loop_run_id, node_id, attention_flag, attention_reason, revision, updated_at
	) VALUES (?, 'attention', 'silence', 'no evidence', 3, ?)`, runA.ID, now.Add(-time.Hour)); err != nil {
		t.Fatalf("insert attention inventory row error = %v", err)
	}
	nextAttemptAt := now.Add(time.Minute)
	if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_generation_outputs (
		loop_run_id, generation, node_id, item_index, status, attempt, next_attempt_at, first_scheduled_at, epoch
	) VALUES (?, 1, 'retry', 0, 'retrying', 2, ?, ?, 5)`, runA.ID, nextAttemptAt, now); err != nil {
		t.Fatalf("insert retrying inventory output error = %v", err)
	}
	if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_node_attempts (
		loop_run_id, generation, node_id, item_index, attempt, failure_class, failure_code,
		cause, disposition, started_at, ended_at, next_attempt_at
	) VALUES (?, 1, 'retry', 0, 2, 'transport', 'unavailable', 'retry later', 'retried', ?, ?, ?)`,
		runA.ID, now.Add(-time.Minute), now, nextAttemptAt); err != nil {
		t.Fatalf("insert retrying inventory attempt error = %v", err)
	}

	first, err := globalDB.ListNodeInventory(ctx, looppkg.NodeInventoryQuery{
		WorkspaceID: "ws-inventory", State: looppkg.NodeInventoryWaiting, Limit: 2,
	})
	if err != nil {
		t.Fatalf("ListNodeInventory(first page) error = %v", err)
	}
	if got := inventoryNodeIDsForTest(first.Items); !slices.Equal(got, []looppkg.NodeID{"wait-a", "wait-b"}) {
		t.Fatalf("first inventory page node ids = %v", got)
	}
	if first.NextCursor == "" {
		t.Fatal("first inventory page next_cursor is empty")
	}
	for _, item := range first.Items {
		if item.Wait == nil || item.Control != nil || item.Output != nil || item.Attempt != nil {
			t.Fatalf("waiting inventory detail = %#v, want wait only", item)
		}
	}
	second, err := globalDB.ListNodeInventory(ctx, looppkg.NodeInventoryQuery{
		WorkspaceID: "ws-inventory", State: looppkg.NodeInventoryWaiting,
		Cursor: first.NextCursor, Limit: 2,
	})
	if err != nil {
		t.Fatalf("ListNodeInventory(second page) error = %v", err)
	}
	secondIDs := inventoryNodeIDsForTest(second.Items)
	if !slices.Equal(secondIDs, []looppkg.NodeID{"wait-c"}) || second.NextCursor != "" {
		t.Fatalf("second inventory page = %#v", second)
	}

	for name, query := range map[string]looppkg.NodeInventoryQuery{
		"loop": {WorkspaceID: "ws-inventory", State: looppkg.NodeInventoryWaiting, LoopName: "alpha"},
		"run":  {WorkspaceID: "ws-inventory", State: looppkg.NodeInventoryWaiting, RunID: runB.ID},
	} {
		page, queryErr := globalDB.ListNodeInventory(ctx, query)
		if queryErr != nil {
			t.Fatalf("ListNodeInventory(%s filter) error = %v", name, queryErr)
		}
		want := []looppkg.NodeID{"wait-a", "wait-b"}
		if name == "run" {
			want = []looppkg.NodeID{"wait-c"}
		}
		if got := inventoryNodeIDsForTest(page.Items); !slices.Equal(got, want) {
			t.Fatalf("%s-filtered inventory node ids = %v, want %v", name, got, want)
		}
	}
	foreignPage, err := globalDB.ListNodeInventory(ctx, looppkg.NodeInventoryQuery{
		WorkspaceID: foreign.WorkspaceID, State: looppkg.NodeInventoryWaiting,
	})
	if err != nil {
		t.Fatalf("ListNodeInventory(foreign workspace) error = %v", err)
	}
	if got := inventoryNodeIDsForTest(foreignPage.Items); !slices.Equal(got, []looppkg.NodeID{"wait-d"}) {
		t.Fatalf("foreign inventory node ids = %v", got)
	}
	_, err = globalDB.ListNodeInventory(ctx, looppkg.NodeInventoryQuery{
		WorkspaceID: foreign.WorkspaceID, State: looppkg.NodeInventoryWaiting,
		Cursor: first.NextCursor,
	})
	if !errors.Is(err, looppkg.ErrValidation) {
		t.Fatalf("cross-workspace cursor error = %v, want ErrValidation", err)
	}

	for state, assertion := range map[looppkg.NodeInventoryState]func(looppkg.NodeInventoryItem) bool{
		looppkg.NodeInventoryQuarantined: func(item looppkg.NodeInventoryItem) bool {
			return item.Control != nil && item.Control.Quarantined && item.Wait == nil && item.Output == nil
		},
		looppkg.NodeInventoryAttention: func(item looppkg.NodeInventoryItem) bool {
			return item.Control != nil && item.Control.AttentionFlag == "silence" && item.Wait == nil && item.Output == nil
		},
		looppkg.NodeInventoryRetrying: func(item looppkg.NodeInventoryItem) bool {
			return item.Output != nil && item.Attempt != nil && item.Output.Attempt == 2 &&
				item.Control == nil && item.Wait == nil
		},
	} {
		page, queryErr := globalDB.ListNodeInventory(ctx, looppkg.NodeInventoryQuery{
			WorkspaceID: runA.WorkspaceID, State: state,
		})
		if queryErr != nil {
			t.Fatalf("ListNodeInventory(%s) error = %v", state, queryErr)
		}
		if len(page.Items) != 1 || !assertion(page.Items[0]) {
			t.Fatalf("%s inventory = %#v", state, page.Items)
		}
	}
	empty, err := globalDB.ListNodeInventory(ctx, looppkg.NodeInventoryQuery{
		WorkspaceID: "ws-empty", State: looppkg.NodeInventoryWaiting,
	})
	if err != nil || len(empty.Items) != 0 || empty.NextCursor != "" {
		t.Fatalf("empty inventory = %#v, error = %v", empty, err)
	}
}

func inventoryNodeIDsForTest(items []looppkg.NodeInventoryItem) []looppkg.NodeID {
	ids := make([]looppkg.NodeID, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.NodeID)
	}
	return ids
}

// Invariant: public cooperative cancel and kill requests fence nodes, settle live cells and runs,
// preserve terminal cells, and never complete canceled work. This real-SQLite GlobalDB suite owns
// the atomic cancellation boundary and final hierarchy truth for IT-002 and IT-003.
func TestGlobalDBLoopRunCancellationShouldFenceBeforeTerminalizing(t *testing.T) {
	t.Parallel()

	globalDB := openLoopTestGlobalDB(t, "ws-1", "ws-other")
	ctx := testutil.Context(t)
	now := time.Date(2026, time.August, 3, 1, 0, 0, 0, time.UTC)
	run, taskRunID := seedLiveLoopLivenessCellForTest(t, globalDB, now)
	coordinatorTaskID, liveTaskID, liveRunID, terminalTaskID := seedLoopSettlementHierarchyForTest(
		t, globalDB, run, "cancel-public", now,
	)
	foreignSeed := testLoopRun("looprun-cancel-foreign", now, looppkg.StatusRunning)
	foreignSeed.WorkspaceID = "ws-other"
	foreignRun, err := globalDB.CreateLoopRunForStart(ctx, foreignSeed, dsl.ConcurrencyAllow)
	if err != nil {
		t.Fatalf("CreateLoopRunForStart(foreign cancellation fixture) error = %v", err)
	}
	if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_generation_outputs (
		loop_run_id, generation, node_id, item_index, status, attempt, first_scheduled_at, epoch
	) VALUES (?, 1, 'other-workspace', 0, 'waiting', 1, ?, 1)`, foreignRun.ID, now); err != nil {
		t.Fatalf("seed foreign run cancellation output error = %v", err)
	}
	if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_generation_outputs (
		loop_run_id, generation, node_id, item_index, status, attempt, first_scheduled_at, epoch
	) VALUES (?, 1, 'hold', 0, 'waiting', 1, ?, 1)`, run.ID, now); err != nil {
		t.Fatalf("seed run cancellation wait output error = %v", err)
	}
	if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_node_waits (
		loop_run_id, generation, node_id, item_index, kind, claim_state, issued_epoch, created_at
	) VALUES (?, 1, 'hold', 0, 'timer', 'waiting', 1, ?)`, run.ID, now); err != nil {
		t.Fatalf("seed run cancellation wait error = %v", err)
	}
	actor := operatorActorContextForTest("operator:cancel")
	mutation := looppkg.CancellationMutation{
		WorkspaceID: run.WorkspaceID, RunID: run.ID, Kind: looppkg.RunCancelCancel,
		Reason: "operator request", Actor: actor, RequestedAt: now.Add(time.Minute),
	}
	result, err := globalDB.RequestRunCancellation(ctx, mutation)
	if err != nil {
		t.Fatalf("RequestRunCancellation() error = %v", err)
	}
	if !result.Applied || result.Terminal {
		t.Fatalf("RequestRunCancellation() = %#v, want durable non-terminal request", result)
	}
	seedLoopCancellationBindingForTest(
		t,
		globalDB,
		string(run.ID),
		string(run.WorkspaceID),
		"main",
		1,
		"session-cancel-owned",
		now,
	)
	foreignMutation := looppkg.CancellationMutation{
		WorkspaceID: foreignRun.WorkspaceID, RunID: foreignRun.ID, Kind: looppkg.RunCancelCancel,
		Reason: "foreign operator request", Actor: operatorActorContextForTest("operator:cancel-foreign"),
		RequestedAt: now.Add(90 * time.Second),
	}
	foreignResult, err := globalDB.RequestRunCancellation(ctx, foreignMutation)
	if err != nil {
		t.Fatalf("RequestRunCancellation(foreign) error = %v", err)
	}
	if !foreignResult.Applied || foreignResult.Terminal {
		t.Fatalf("RequestRunCancellation(foreign) = %#v, want durable non-terminal request", foreignResult)
	}
	seedLoopCancellationBindingForTest(
		t,
		globalDB,
		string(foreignRun.ID),
		string(foreignRun.WorkspaceID),
		"other-workspace",
		1,
		"session-cancel-other",
		now,
	)
	earlyProvenanceAt := now.Add(30 * time.Second)
	laterProvenanceAt := now.Add(2 * time.Minute)
	if _, err := globalDB.db.ExecContext(ctx, `UPDATE loop_node_controls
		SET cancel_actor_kind = ?, cancel_actor_id = ?, cancel_reason = ?, cancel_requested_at = ?
		WHERE loop_run_id = ? AND node_id = 'hold'`,
		taskpkg.ActorKindHuman, "z-early-actor", "z-early-reason", earlyProvenanceAt, run.ID); err != nil {
		t.Fatalf("seed earliest run cancellation provenance error = %v", err)
	}
	if _, err := globalDB.db.ExecContext(ctx, `UPDATE loop_node_controls
		SET cancel_actor_kind = ?, cancel_actor_id = ?, cancel_reason = ?, cancel_requested_at = ?
		WHERE loop_run_id = ? AND node_id = 'work'`,
		taskpkg.ActorKindAgentSession, "a-later-actor", "a-later-reason", laterProvenanceAt, run.ID); err != nil {
		t.Fatalf("seed later run cancellation provenance error = %v", err)
	}
	pending, err := globalDB.ListPendingCancellations(ctx, 10)
	if err != nil {
		t.Fatalf("ListPendingCancellations(requested) error = %v", err)
	}
	if len(pending) != 2 {
		t.Fatalf("pending cancellations = %#v, want one command per workspace", pending)
	}
	pendingByRun := make(map[looppkg.RunID]looppkg.PendingCancellation, len(pending))
	for _, command := range pending {
		pendingByRun[command.RunID] = command
	}
	ownedPending, ok := pendingByRun[run.ID]
	if !ok || ownedPending.WorkspaceID != run.WorkspaceID || ownedPending.NodeID != "" ||
		ownedPending.State != looppkg.CancelStateRequested ||
		ownedPending.RequestedBy.Kind != taskpkg.ActorKindHuman ||
		ownedPending.RequestedBy.Ref != "z-early-actor" || ownedPending.Reason != "z-early-reason" ||
		!ownedPending.RequestedAt.Equal(earlyProvenanceAt) ||
		!slices.Equal(ownedPending.SessionIDs, []string{"session-cancel-owned"}) {
		t.Fatalf("owned pending cancellation = %#v, want correlated earliest provenance", ownedPending)
	}
	foreignPending, ok := pendingByRun[foreignRun.ID]
	if !ok || foreignPending.WorkspaceID != foreignRun.WorkspaceID || foreignPending.NodeID != "" ||
		foreignPending.State != looppkg.CancelStateRequested ||
		foreignPending.RequestedBy.Ref != foreignMutation.Actor.Actor.Ref ||
		!slices.Equal(foreignPending.SessionIDs, []string{"session-cancel-other"}) {
		t.Fatalf("foreign pending cancellation = %#v, want exact foreign workspace delivery", foreignPending)
	}
	if _, err := globalDB.db.ExecContext(ctx, `UPDATE loop_node_controls
		SET cancel_actor_kind = ?, cancel_actor_id = ?, cancel_reason = ?, cancel_requested_at = ?
		WHERE loop_run_id = ?`, mutation.Actor.Actor.Kind, mutation.Actor.Actor.Ref,
		mutation.Reason, mutation.RequestedAt, run.ID); err != nil {
		t.Fatalf("restore run cancellation provenance error = %v", err)
	}
	for _, state := range []looppkg.CancelState{looppkg.CancelStateDelivering, looppkg.CancelStateDraining} {
		if _, err := globalDB.AdvanceRunCancellation(ctx, foreignMutation, state); err != nil {
			t.Fatalf("AdvanceRunCancellation(foreign, %s) error = %v", state, err)
		}
	}
	stored, err := globalDB.GetLoopRun(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("GetLoopRun() error = %v", err)
	}
	if !stored.CancelRequested || stored.CancelKind != looppkg.RunCancelCancel ||
		stored.Status != looppkg.StatusRunning {
		t.Fatalf("requested Run = %#v, want live cancel projection", stored)
	}
	var epoch int64
	if err := globalDB.db.QueryRowContext(ctx, `SELECT epoch FROM loop_generation_outputs
		WHERE loop_run_id = ? AND task_run_id = ?`, run.ID, taskRunID).Scan(&epoch); err != nil {
		t.Fatalf("read canceled output epoch error = %v", err)
	}
	if epoch != 5 {
		t.Fatalf("canceled output epoch = %d, want 5", epoch)
	}
	controls, err := globalDB.ListNodeControls(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("ListNodeControls() error = %v", err)
	}
	if len(controls) != 2 {
		t.Fatalf("cancel controls = %#v, want first-writer request provenance", controls)
	}
	for _, control := range controls {
		if control.CancelState != looppkg.CancelStateRequested || control.CancelProvenance == nil ||
			control.CancelProvenance.ActorID != "operator:cancel" {
			t.Fatalf("cancel control = %#v, want first-writer request provenance", control)
		}
	}
	for _, state := range []looppkg.CancelState{
		looppkg.CancelStateDelivering, looppkg.CancelStateDraining,
	} {
		result, err = globalDB.AdvanceRunCancellation(ctx, mutation, state)
		if err != nil {
			t.Fatalf("AdvanceRunCancellation(%s) error = %v", state, err)
		}
		if state == looppkg.CancelStateDraining &&
			(result.Coordinator == nil || result.Coordinator.LoopRunID != string(run.ID) ||
				taskpkg.IsTerminalRunStatus(result.Coordinator.Status)) {
			t.Fatalf("draining coordinator = %#v, want an open wake for the canceled Run", result.Coordinator)
		}
		if state == looppkg.CancelStateDraining &&
			!strings.Contains(cancellationWakeIdempotencyKey(mutation), ".run:"+string(run.ID)+".") {
			t.Fatalf(
				"run cancellation wake key = %q, want explicit run scope",
				cancellationWakeIdempotencyKey(mutation),
			)
		}
		pending, err = globalDB.ListPendingCancellations(ctx, 10)
		if err != nil {
			t.Fatalf("ListPendingCancellations(%s) error = %v", state, err)
		}
		if state == looppkg.CancelStateDelivering {
			if len(pending) != 1 || pending[0].State != looppkg.CancelStateDelivering {
				t.Fatalf("delivering pending cancellation = %#v, want one delivering command", pending)
			}
		} else if len(pending) != 0 {
			t.Fatalf("draining pending cancellations = %#v, want none", pending)
		}
	}
	if result.Terminal || result.Run.Status != looppkg.StatusRunning {
		t.Fatalf("draining cancellation = %#v, want non-terminal running truth", result)
	}
	claim := claimCoordinatorRunForTest(
		ctx,
		t,
		globalDB,
		run.ID,
		"cancel-drain",
		now.Add(2*time.Minute),
	)
	endedAt := now.Add(2*time.Minute + 30*time.Second)
	failureClass := looppkg.FailureCancellation
	expectedEpoch := int64(5)
	expectedWaitEpoch := int64(2)
	completion, err := globalDB.CompleteCoordinatorAndEnqueueNext(
		ctx,
		taskpkg.CoordinatorCompletion{
			RunID: claim.Run.ID, ClaimToken: claim.ClaimToken,
			Actor: coordinatorActorContextForTest(), Now: endedAt,
			Plan: taskpkg.CoordinatorCompletionPlan{
				CancellationDrain: true,
				Snapshot: taskpkg.GenerationSnapshot{
					LoopRunID: string(run.ID), Generation: 1,
					Payload: looppkg.GenerationSnapshotPayload{
						Outputs: []looppkg.GenerationOutput{{
							Generation: 1, NodeID: "work", Status: "canceled", TaskRunID: taskRunID,
							Attempt: 1, Epoch: expectedEpoch, ExpectedEpoch: &expectedEpoch,
						}, {
							Generation: 1, NodeID: "hold", Status: "canceled", Attempt: 1,
							Epoch: expectedWaitEpoch, ExpectedEpoch: &expectedWaitEpoch,
						}},
						Attempts: []looppkg.NodeAttempt{{
							LoopRunID: run.ID, Generation: 1, NodeID: "work", Attempt: 1,
							FailureClass: &failureClass, FailureCode: string(looppkg.TransitionCauseOperatorCancel),
							Cause: mutation.Reason, Disposition: looppkg.AttemptCanceled,
							StartedAt: mutation.RequestedAt, EndedAt: &endedAt,
						}, {
							LoopRunID: run.ID, Generation: 1, NodeID: "hold", Attempt: 1,
							FailureClass: &failureClass, FailureCode: string(looppkg.TransitionCauseOperatorCancel),
							Cause: mutation.Reason, Disposition: looppkg.AttemptCanceled,
							StartedAt: mutation.RequestedAt, EndedAt: &endedAt,
						}},
						Controls: []looppkg.NodeControlMutation{{
							Kind: looppkg.NodeControlMutationCancel, NodeID: "work", ExpectedRevision: 3,
							ExpectExisting: true, CancelState: looppkg.CancelStateCanceled, At: endedAt,
						}, {
							Kind: looppkg.NodeControlMutationCancel, NodeID: "hold", ExpectedRevision: 3,
							ExpectExisting: true, CancelState: looppkg.CancelStateCanceled, At: endedAt,
						}},
						Events: []looppkg.GenerationLifecycleEventIntent{{
							Kind: looppkg.GenerationLifecycleEventNodeCanceled, NodeID: "work", Attempt: 1,
							Failure: &looppkg.ClassifiedFailure{
								Class: looppkg.FailureCancellation,
								Code:  string(looppkg.TransitionCauseOperatorCancel),
								Cause: mutation.Reason,
							},
							Disposition: looppkg.AttemptCanceled,
						}, {
							Kind: looppkg.GenerationLifecycleEventNodeCanceled, NodeID: "hold", Attempt: 1,
							Failure: &looppkg.ClassifiedFailure{
								Class: looppkg.FailureCancellation,
								Code:  string(looppkg.TransitionCauseOperatorCancel),
								Cause: mutation.Reason,
							},
							Disposition: looppkg.AttemptCanceled,
						}},
					},
				},
				Terminal: &taskpkg.CoordinatorTerminal{
					Status: string(looppkg.StatusCanceled), Cause: string(looppkg.TransitionCauseOperatorCancel),
				},
			},
		},
		looppkg.NewStoreFinalizer(),
	)
	if err != nil {
		t.Fatalf("CompleteCoordinatorAndEnqueueNext(cancel drain) error = %v", err)
	}
	if !completion.Terminal {
		t.Fatalf("coordinator cancellation result = %#v, want terminal", completion)
	}
	var outputStatus string
	if err := globalDB.db.QueryRowContext(ctx, `SELECT status FROM loop_generation_outputs
		WHERE loop_run_id = ? AND task_run_id = ?`, run.ID, taskRunID).Scan(&outputStatus); err != nil {
		t.Fatalf("read terminal output status error = %v", err)
	}
	if outputStatus != "canceled" {
		t.Fatalf("terminal output status = %q, want canceled", outputStatus)
	}
	if err := globalDB.db.QueryRowContext(ctx, `SELECT status FROM loop_generation_outputs
		WHERE loop_run_id = ? AND node_id = 'hold'`, run.ID).Scan(&outputStatus); err != nil {
		t.Fatalf("read terminal wait output status error = %v", err)
	}
	if outputStatus != "canceled" {
		t.Fatalf("terminal wait output status = %q, want canceled", outputStatus)
	}
	waits, err := globalDB.ListNodeWaits(ctx, run.WorkspaceID, run.ID)
	if err != nil || len(waits) != 0 {
		t.Fatalf("ListNodeWaits(after Run cancellation) = %#v, %v, want no active waits", waits, err)
	}
	var claimState, claimedByKind, claimedByID string
	if err := globalDB.db.QueryRowContext(ctx, `SELECT claim_state, claimed_by_kind, claimed_by_id
		FROM loop_node_waits WHERE loop_run_id = ? AND node_id = 'hold'`, run.ID).Scan(
		&claimState, &claimedByKind, &claimedByID,
	); err != nil {
		t.Fatalf("read Run cancellation wait claim error = %v", err)
	}
	if claimState != string(looppkg.WaitClaimClaimed) ||
		claimedByKind != string(actor.Actor.Kind.Normalize()) || claimedByID != actor.Actor.Ref {
		t.Fatalf("Run cancellation wait claim = %q/%q/%q", claimState, claimedByKind, claimedByID)
	}
	stored, err = globalDB.GetLoopRun(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("GetLoopRun(terminal) error = %v", err)
	}
	if stored.Status != looppkg.StatusCanceled {
		t.Fatalf("terminal Run status = %q, want canceled", stored.Status)
	}
	controls, err = globalDB.ListNodeControls(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("ListNodeControls(terminal) error = %v", err)
	}
	if len(controls) != 2 || controls[0].CancelState != looppkg.CancelStateCanceled ||
		controls[1].CancelState != looppkg.CancelStateCanceled {
		t.Fatalf("terminal cancel controls = %#v", controls)
	}
	assertLoopPublicTerminalSettlementForTest(
		t,
		globalDB,
		run,
		coordinatorTaskID,
		liveTaskID,
		liveRunID,
		terminalTaskID,
		taskpkg.TaskStatusCanceled,
		"run canceled; node no longer needed",
	)

	t.Run(
		"Should kill through the public cancellation boundary and preserve terminal truth IT-003",
		func(t *testing.T) {
			t.Parallel()

			db := openLoopTestGlobalDB(t)
			ctx := testutil.Context(t)
			killAt := now.Add(20 * time.Minute)
			killedRun, err := db.CreateLoopRunForStart(
				ctx,
				testLoopRun("looprun-kill-public", killAt, looppkg.StatusRunning),
				dsl.ConcurrencyAllow,
			)
			if err != nil {
				t.Fatalf("CreateLoopRunForStart(kill) error = %v", err)
			}
			coordinatorID, liveTaskID, liveRunID, terminalTaskID := seedLoopSettlementHierarchyForTest(
				t, db, killedRun, "kill-public", killAt,
			)
			result, err := db.RequestRunCancellation(ctx, looppkg.CancellationMutation{
				WorkspaceID: killedRun.WorkspaceID,
				RunID:       killedRun.ID,
				Kind:        looppkg.RunCancelKill,
				Reason:      "operator kill",
				Actor:       operatorActorContextForTest("operator:kill"),
				RequestedAt: killAt.Add(time.Minute),
			})
			if err != nil {
				t.Fatalf("RequestRunCancellation(kill) error = %v", err)
			}
			if !result.Applied || !result.Terminal || result.Run.Status != looppkg.StatusCanceled {
				t.Fatalf("RequestRunCancellation(kill) = %#v, want terminal canceled result", result)
			}
			assertLoopPublicTerminalSettlementForTest(
				t,
				db,
				killedRun,
				coordinatorID,
				liveTaskID,
				liveRunID,
				terminalTaskID,
				taskpkg.TaskStatusCanceled,
				"run killed; node no longer needed",
			)
		},
	)

	for _, status := range []looppkg.Status{
		looppkg.StatusQueued,
		looppkg.StatusWatching,
		looppkg.StatusNeedsApproval,
		looppkg.StatusPaused,
	} {
		t.Run("Should terminalize a node-free "+string(status)+" Run directly", func(t *testing.T) {
			t.Parallel()

			db := openLoopTestGlobalDB(t)
			ctx := testutil.Context(t)
			created, err := db.CreateLoopRunForStart(
				ctx,
				testLoopRun("looprun-cancel-direct-"+string(status), now, status),
				dsl.ConcurrencyAllow,
			)
			if err != nil {
				t.Fatalf("CreateLoopRunForStart(%s) error = %v", status, err)
			}
			result, err := db.RequestRunCancellation(ctx, looppkg.CancellationMutation{
				WorkspaceID: created.WorkspaceID,
				RunID:       created.ID,
				Kind:        looppkg.RunCancelCancel,
				Reason:      "operator request",
				Actor:       operatorActorContextForTest("operator:direct-cancel"),
				RequestedAt: now.Add(time.Minute),
			})
			if err != nil {
				t.Fatalf("RequestRunCancellation(%s) error = %v", status, err)
			}
			if !result.Applied || !result.Terminal || result.Run.Status != looppkg.StatusCanceled {
				t.Fatalf("direct cancellation from %s = %#v", status, result)
			}
			if result.Run.ControlActor.Kind != taskpkg.ActorKindHuman ||
				result.Run.ControlActor.Ref != "operator:direct-cancel" ||
				!result.Run.ControlRequestedAt.Equal(now.Add(time.Minute)) {
				t.Fatalf("direct cancellation actor from %s = %#v at %s", status,
					result.Run.ControlActor, result.Run.ControlRequestedAt)
			}
			persisted, err := db.GetLoopRun(ctx, created.WorkspaceID, created.ID)
			if err != nil {
				t.Fatalf("GetLoopRun(%s) error = %v", status, err)
			}
			if persisted.ControlActor != result.Run.ControlActor ||
				!persisted.ControlRequestedAt.Equal(result.Run.ControlRequestedAt) {
				t.Fatalf("persisted cancellation actor from %s = %#v at %s, result %#v at %s",
					status, persisted.ControlActor, persisted.ControlRequestedAt,
					result.Run.ControlActor, result.Run.ControlRequestedAt)
			}
		})
	}

	t.Run("Should repair a canceled coordinator task and replay boot reconciliation once", func(t *testing.T) {
		t.Parallel()

		db := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		repairAt := now.Add(10 * time.Minute)
		cancelingRun, liveTaskRunID := seedLiveLoopLivenessCellForTest(t, db, repairAt)
		if strings.TrimSpace(liveTaskRunID) == "" {
			t.Fatal("seedLiveLoopLivenessCellForTest() task run id is empty")
		}
		cancelMutation := looppkg.CancellationMutation{
			WorkspaceID: cancelingRun.WorkspaceID,
			RunID:       cancelingRun.ID,
			Kind:        looppkg.RunCancelCancel,
			Reason:      "operator request",
			Actor:       operatorActorContextForTest("operator:repair-cancel"),
			RequestedAt: repairAt.Add(time.Minute),
		}
		if result, err := db.RequestRunCancellation(ctx, cancelMutation); err != nil {
			t.Fatalf("RequestRunCancellation() error = %v", err)
		} else if !result.Applied || result.Terminal {
			t.Fatalf("RequestRunCancellation() = %#v, want durable non-terminal request", result)
		}

		coordinatorTaskID := loopCoordinatorTaskID(cancelingRun.ID)
		manager, err := taskpkg.NewManager(
			taskpkg.WithStore(db),
			taskpkg.WithManagerNow(func() time.Time { return repairAt.Add(2 * time.Minute) }),
		)
		if err != nil {
			t.Fatalf("task.NewManager() error = %v", err)
		}
		canceledTask, err := manager.CancelTask(
			ctx,
			coordinatorTaskID,
			taskpkg.CancelTask{Reason: "simulate canceled coordinator pulse"},
			operatorActorContextForTest("operator:cancel-coordinator"),
		)
		if err != nil {
			t.Fatalf("CancelTask(coordinator) error = %v", err)
		}
		if canceledTask.Status != taskpkg.TaskStatusCanceled {
			t.Fatalf("canceled coordinator task status = %q, want canceled", canceledTask.Status)
		}

		origin := taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "daemon.boot"}
		recovered, err := db.ReconcileLoopCoordinatorsOnBoot(ctx, origin, repairAt.Add(3*time.Minute))
		if err != nil {
			t.Fatalf("ReconcileLoopCoordinatorsOnBoot() error = %v", err)
		}
		if len(recovered) != 1 {
			t.Fatalf("recovered coordinator runs = %#v, want one", recovered)
		}
		if recovered[0].TaskID != coordinatorTaskID ||
			recovered[0].LoopRunID != string(cancelingRun.ID) ||
			recovered[0].WorkspaceID != string(cancelingRun.WorkspaceID) ||
			recovered[0].RunKind.Normalize() != taskpkg.RunKindCoordinator {
			t.Fatalf("recovered coordinator = %#v, want exact workspace-scoped Loop coordinator", recovered[0])
		}
		repairedTask, err := db.GetTask(ctx, coordinatorTaskID)
		if err != nil {
			t.Fatalf("GetTask(repaired coordinator) error = %v", err)
		}
		if repairedTask.Status != taskpkg.TaskStatusInProgress {
			t.Fatalf("repaired coordinator task status = %q, want in_progress", repairedTask.Status)
		}
		if !repairedTask.ClosedAt.IsZero() || !repairedTask.UpdatedAt.Equal(repairAt.Add(3*time.Minute)) {
			t.Fatalf(
				"repaired coordinator task timestamps = updated %s closed %s, want repair time and open task",
				repairedTask.UpdatedAt,
				repairedTask.ClosedAt,
			)
		}

		replayed, err := db.ReconcileLoopCoordinatorsOnBoot(ctx, origin, repairAt.Add(4*time.Minute))
		if err != nil {
			t.Fatalf("ReconcileLoopCoordinatorsOnBoot(replay) error = %v", err)
		}
		if len(replayed) != 0 {
			t.Fatalf("replayed coordinator runs = %#v, want no duplicate", replayed)
		}
		if _, err := manager.CancelRun(
			ctx,
			recovered[0].ID,
			taskpkg.CancelRun{Reason: "simulate canceled recovery pulse"},
			coordinatorActorContextForTest(),
		); err != nil {
			t.Fatalf("CancelRun(recovered coordinator) error = %v", err)
		}
		recoveredAgain, err := db.ReconcileLoopCoordinatorsOnBoot(ctx, origin, repairAt.Add(5*time.Minute))
		if err != nil {
			t.Fatalf("ReconcileLoopCoordinatorsOnBoot(after canceled recovery) error = %v", err)
		}
		if len(recoveredAgain) != 1 || recoveredAgain[0].ID == recovered[0].ID {
			t.Fatalf("recovered coordinator after canceled recovery = %#v, want one new run", recoveredAgain)
		}
		replayedAgain, err := db.ReconcileLoopCoordinatorsOnBoot(ctx, origin, repairAt.Add(6*time.Minute))
		if err != nil {
			t.Fatalf("ReconcileLoopCoordinatorsOnBoot(second replay) error = %v", err)
		}
		if len(replayedAgain) != 0 {
			t.Fatalf("second replay coordinator runs = %#v, want no duplicate", replayedAgain)
		}

		if _, err := db.AdvanceRunCancellation(ctx, cancelMutation, looppkg.CancelStateDelivering); err != nil {
			t.Fatalf("AdvanceRunCancellation(delivering) error = %v", err)
		}
		draining, err := db.AdvanceRunCancellation(ctx, cancelMutation, looppkg.CancelStateDraining)
		if err != nil {
			t.Fatalf("AdvanceRunCancellation(draining) error = %v", err)
		}
		if draining.Coordinator == nil || draining.Coordinator.ID != recoveredAgain[0].ID {
			t.Fatalf(
				"draining coordinator = %#v, want recovered run %q",
				draining.Coordinator,
				recoveredAgain[0].ID,
			)
		}

		statusEvents, err := db.ListTaskEvents(ctx, taskpkg.EventQuery{
			TaskID: coordinatorTaskID, EventType: string(hookspkg.HookTaskStatusChanged),
		})
		if err != nil {
			t.Fatalf("ListTaskEvents(status changed) error = %v", err)
		}
		repairEvents := 0
		for _, event := range statusEvents {
			var statusPayload struct {
				FromStatus string `json:"from_status"`
				ToStatus   string `json:"to_status"`
			}
			if err := json.Unmarshal(event.Payload, &statusPayload); err != nil {
				t.Fatalf("json.Unmarshal(status changed payload) error = %v", err)
			}
			if statusPayload.FromStatus == string(taskpkg.TaskStatusCanceled) &&
				statusPayload.ToStatus == string(taskpkg.TaskStatusInProgress) {
				repairEvents++
				if event.Actor.Kind.Normalize() != taskpkg.ActorKindDaemon ||
					event.Origin.Kind.Normalize() != taskpkg.OriginKindDaemon {
					t.Fatalf("repair event actor/origin = %#v/%#v, want daemon ownership", event.Actor, event.Origin)
				}
			}
		}
		if repairEvents != 1 {
			t.Fatalf("canceled to in_progress repair events = %d, want one", repairEvents)
		}
	})
}

// Invariant: node cancellation fences only that node, delivers only its managed session,
// closes each affected attempt, and commits on_cancel with the terminal event. Kill closes the
// same durable truth but never writes a node-trigger delivery. This store suite owns the node
// cancellation transaction.
func TestGlobalDBLoopNodeCancellationShouldCloseAttemptsAndEffectsAtomically(t *testing.T) {
	t.Parallel()

	t.Run("Should drain one node and commit one on_cancel delivery", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 3, 1, 30, 0, 0, time.UTC)
		run, taskRunID := seedLiveLoopLivenessCellForTest(t, globalDB, now)
		seedLoopCancellationBindingForTest(t, globalDB, string(run.ID), string(run.WorkspaceID), "main", 1,
			"session-work", now)
		seedLoopCancellationBindingForTest(t, globalDB, string(run.ID), string(run.WorkspaceID), "other", 1,
			"session-other", now)
		mutation := nodeCancellationMutationForTest(run, looppkg.RunCancelCancel, now.Add(time.Minute))
		mutation.Effects = []looppkg.RenderedEffectIntent{{
			Trigger: looppkg.EffectTriggerOnCancel, Generation: 1, NodeID: "work",
			Entry: json.RawMessage(`{"kind":"emit","emit":{"kind":"work_canceled"}}`),
		}}

		result, err := globalDB.RequestNodeCancellation(ctx, mutation)
		if err != nil {
			t.Fatalf("RequestNodeCancellation() error = %v", err)
		}
		if !result.Applied || !slices.Equal(result.SessionIDs, []string{"session-work"}) {
			t.Fatalf("RequestNodeCancellation() = %#v, want only work session", result)
		}
		var epoch int64
		if err := globalDB.db.QueryRowContext(ctx, `SELECT epoch FROM loop_generation_outputs
			WHERE loop_run_id = ? AND task_run_id = ?`, run.ID, taskRunID).Scan(&epoch); err != nil {
			t.Fatalf("read fenced node epoch error = %v", err)
		}
		if epoch != 5 {
			t.Fatalf("fenced node epoch = %d, want 5", epoch)
		}
		for _, state := range []looppkg.CancelState{
			looppkg.CancelStateDelivering, looppkg.CancelStateDraining, looppkg.CancelStateCanceled,
		} {
			advanced, err := globalDB.AdvanceNodeCancellation(ctx, mutation, state)
			if err != nil {
				t.Fatalf("AdvanceNodeCancellation(%s) error = %v", state, err)
			}
			if state == looppkg.CancelStateDraining &&
				(advanced.Coordinator == nil || advanced.Coordinator.LoopRunID != string(run.ID) ||
					taskpkg.IsTerminalRunStatus(advanced.Coordinator.Status)) {
				t.Fatalf(
					"draining node coordinator = %#v, want an open wake for the canceled node",
					advanced.Coordinator,
				)
			}
			if state == looppkg.CancelStateDraining &&
				!strings.Contains(cancellationWakeIdempotencyKey(mutation), ".node:work.") {
				t.Fatalf(
					"node cancellation wake key = %q, want explicit node scope",
					cancellationWakeIdempotencyKey(mutation),
				)
			}
		}

		var outputStatus, failureClass, failureCode, cause, disposition string
		if err := globalDB.db.QueryRowContext(ctx, `SELECT output.status, attempt.failure_class,
			attempt.failure_code, attempt.cause, attempt.disposition
			FROM loop_generation_outputs AS output
			JOIN loop_node_attempts AS attempt ON attempt.loop_run_id = output.loop_run_id
			 AND attempt.generation = output.generation AND attempt.node_id = output.node_id
			 AND attempt.item_index = output.item_index AND attempt.attempt = output.attempt
			WHERE output.loop_run_id = ? AND output.node_id = 'work'`, run.ID).Scan(
			&outputStatus, &failureClass, &failureCode, &cause, &disposition,
		); err != nil {
			t.Fatalf("read canceled node attempt error = %v", err)
		}
		if outputStatus != "canceled" || failureClass != "cancellation" ||
			failureCode != string(looppkg.TransitionCauseOperatorCancel) ||
			cause != mutation.Reason || disposition != "canceled" {
			t.Fatalf("canceled node = %q/%q/%q/%q/%q", outputStatus, failureClass, failureCode, cause, disposition)
		}
		entries, err := globalDB.ListEffectOutbox(ctx, run.WorkspaceID, run.ID)
		if err != nil {
			t.Fatalf("ListEffectOutbox() error = %v", err)
		}
		if len(entries) != 1 || entries[0].Trigger != string(looppkg.EffectTriggerOnCancel) {
			t.Fatalf("node cancel outbox = %#v, want one on_cancel", entries)
		}
		events, err := globalDB.ListLoopRunEvents(ctx, looppkg.RunEventQuery{
			WorkspaceID: run.WorkspaceID, RunID: run.ID,
		})
		if err != nil {
			t.Fatalf("ListLoopRunEvents() error = %v", err)
		}
		if countLoopEventKindForTest(events, loopRunEventNodeCanceled) != 1 {
			t.Fatalf("node canceled events = %#v, want one", events)
		}
	})

	t.Run("Should cancel only the addressed fan-out cell", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 3, 1, 40, 0, 0, time.UTC)
		run, err := globalDB.CreateLoopRunForStart(
			ctx, testLoopRun("looprun-node-cancel-lane", now, looppkg.StatusRunning), dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		completeInitialCoordinatorForParkedFixture(t, globalDB, run, now)
		if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_generation_outputs (
			loop_run_id, generation, node_id, item_index, status, attempt, first_scheduled_at, epoch
		) VALUES (?, 1, 'work', 0, 'pending', 1, ?, 3),
			(?, 1, 'work', 1, 'pending', 1, ?, 7)`, run.ID, now, run.ID, now); err != nil {
			t.Fatalf("insert lane cancellation fixture error = %v", err)
		}
		itemIndex := 1
		mutation := nodeCancellationMutationForTest(run, looppkg.RunCancelCancel, now.Add(time.Minute))
		mutation.ItemIndex = &itemIndex
		mutation.Effects = []looppkg.RenderedEffectIntent{{
			Trigger: looppkg.EffectTriggerOnCancel, Generation: 1, NodeID: "work", ItemIndex: itemIndex,
			Entry: json.RawMessage(`{"kind":"emit","emit":{"kind":"lane_canceled"}}`),
		}}
		result, err := globalDB.RequestNodeCancellation(ctx, mutation)
		if err != nil {
			t.Fatalf("RequestNodeCancellation(lane) error = %v", err)
		}
		if !result.Applied || result.Coordinator == nil {
			t.Fatalf("RequestNodeCancellation(lane) = %#v, want applied coordinator wake", result)
		}
		rows, err := globalDB.db.QueryContext(ctx, `SELECT item_index, status, epoch
			FROM loop_generation_outputs WHERE loop_run_id = ? AND node_id = 'work' ORDER BY item_index`, run.ID)
		if err != nil {
			t.Fatalf("read lane cancellation outputs error = %v", err)
		}
		type laneState struct {
			itemIndex int
			status    string
			epoch     int
		}
		states := make([]laneState, 0, 2)
		for rows.Next() {
			var state laneState
			if err := rows.Scan(&state.itemIndex, &state.status, &state.epoch); err != nil {
				t.Fatalf("scan lane cancellation output error = %v", err)
			}
			states = append(states, state)
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("iterate lane cancellation outputs error = %v", err)
		}
		if err := rows.Close(); err != nil {
			t.Fatalf("close lane cancellation outputs error = %v", err)
		}
		wantStates := []laneState{{itemIndex: 0, status: "pending", epoch: 3},
			{itemIndex: 1, status: "canceled", epoch: 8}}
		if !reflect.DeepEqual(states, wantStates) {
			t.Fatalf("lane cancellation outputs = %#v, want %#v", states, wantStates)
		}
		entries, err := globalDB.ListEffectOutbox(ctx, run.WorkspaceID, run.ID)
		if err != nil {
			t.Fatalf("ListEffectOutbox(lane) error = %v", err)
		}
		if len(entries) != 1 || entries[0].ItemIndex != itemIndex {
			t.Fatalf("lane cancellation outbox = %#v, want item %d only", entries, itemIndex)
		}
	})

	t.Run("Should invalidate backoff and suppress node effects on kill", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 3, 1, 45, 0, 0, time.UTC)
		run, _ := seedLiveLoopLivenessCellForTest(t, globalDB, now)
		seedLoopCancellationBindingForTest(t, globalDB, string(run.ID), string(run.WorkspaceID), "main", 1,
			"session-work", now)
		nextAttemptAt := now.Add(time.Hour)
		if _, err := globalDB.db.ExecContext(ctx, `UPDATE loop_generation_outputs
			SET status = 'retrying', next_attempt_at = ? WHERE loop_run_id = ? AND node_id = 'work'`,
			nextAttemptAt, run.ID); err != nil {
			t.Fatalf("seed retrying output error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_node_attempts (
			loop_run_id, generation, node_id, item_index, attempt, failure_class, failure_code,
			cause, disposition, started_at, ended_at, next_attempt_at
		) VALUES (?, 1, 'work', 0, 1, 'transport', 'transport_closed', 'lost', 'retried', ?, ?, ?)`,
			run.ID, now, now.Add(time.Minute), nextAttemptAt); err != nil {
			t.Fatalf("seed retry attempt error = %v", err)
		}
		mutation := nodeCancellationMutationForTest(run, looppkg.RunCancelKill, now.Add(2*time.Minute))
		mutation.Effects = []looppkg.RenderedEffectIntent{{
			Trigger: looppkg.EffectTriggerOnCancel, Generation: 1, NodeID: "work",
			Entry: json.RawMessage(`{"kind":"emit","emit":{"kind":"must_not_fire"}}`),
		}}
		result, err := globalDB.RequestNodeCancellation(ctx, mutation)
		if err != nil {
			t.Fatalf("RequestNodeCancellation(kill) error = %v", err)
		}
		if !result.Applied || !slices.Equal(result.SessionIDs, []string{"session-work"}) {
			t.Fatalf("RequestNodeCancellation(kill) = %#v", result)
		}
		if result.Coordinator == nil || result.Coordinator.LoopRunID != string(run.ID) ||
			taskpkg.IsTerminalRunStatus(result.Coordinator.Status) {
			t.Fatalf("node kill coordinator = %#v, want an open reconciliation wake", result.Coordinator)
		}
		var status string
		var nextAt *time.Time
		if err := globalDB.db.QueryRowContext(ctx, `SELECT status, next_attempt_at
			FROM loop_generation_outputs WHERE loop_run_id = ? AND node_id = 'work'`, run.ID).Scan(
			&status, &nextAt,
		); err != nil {
			t.Fatalf("read killed retry output error = %v", err)
		}
		if status != "canceled" || nextAt != nil {
			t.Fatalf("killed retry output = %q/%v, want canceled/no due time", status, nextAt)
		}
		var failureCode, disposition string
		if err := globalDB.db.QueryRowContext(ctx, `SELECT failure_code, disposition FROM loop_node_attempts
			WHERE loop_run_id = ? AND node_id = 'work' AND attempt = 2`, run.ID).Scan(
			&failureCode, &disposition,
		); err != nil {
			t.Fatalf("read killed pending attempt error = %v", err)
		}
		if failureCode != string(looppkg.TransitionCauseOperatorKill) || disposition != "canceled" {
			t.Fatalf("killed pending attempt = %q/%q", failureCode, disposition)
		}
		entries, err := globalDB.ListEffectOutbox(ctx, run.WorkspaceID, run.ID)
		if err != nil {
			t.Fatalf("ListEffectOutbox(kill) error = %v", err)
		}
		if len(entries) != 0 {
			t.Fatalf("kill node outbox = %#v, want none", entries)
		}
	})

	t.Run("Should fence waiting and paused nodes as live cancellation targets", func(t *testing.T) {
		t.Parallel()

		for _, status := range []string{"waiting", "paused"} {
			t.Run("Should fence "+status+" node", func(t *testing.T) {
				t.Parallel()

				globalDB := openLoopTestGlobalDB(t)
				ctx := testutil.Context(t)
				now := time.Date(2026, time.August, 3, 2, 0, 0, 0, time.UTC)
				run, _ := seedLiveLoopLivenessCellForTest(t, globalDB, now)
				if _, err := globalDB.db.ExecContext(ctx, `UPDATE loop_generation_outputs
					SET status = ?, task_run_id = NULL WHERE loop_run_id = ? AND node_id = 'work'`,
					status, run.ID); err != nil {
					t.Fatalf("seed %s output error = %v", status, err)
				}
				if status == "waiting" {
					if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_node_waits (
						loop_run_id, generation, node_id, item_index, kind, claim_state,
						issued_epoch, created_at
					) VALUES (?, 1, 'work', 0, 'timer', 'waiting', 4, ?)`, run.ID, now); err != nil {
						t.Fatalf("seed waiting row error = %v", err)
					}
				}

				mutation := nodeCancellationMutationForTest(
					run,
					looppkg.RunCancelCancel,
					now.Add(time.Minute),
				)
				result, err := globalDB.RequestNodeCancellation(ctx, mutation)
				if err != nil {
					t.Fatalf("RequestNodeCancellation(%s) error = %v", status, err)
				}
				if !result.Applied {
					t.Fatalf("RequestNodeCancellation(%s) = %#v, want applied", status, result)
				}
				controls, err := globalDB.ListNodeControls(ctx, run.WorkspaceID, run.ID)
				if err != nil {
					t.Fatalf("ListNodeControls(%s) error = %v", status, err)
				}
				if len(controls) != 1 || controls[0].CancelState != looppkg.CancelStateRequested {
					t.Fatalf("%s cancel controls = %#v, want requested", status, controls)
				}
				for _, state := range []looppkg.CancelState{
					looppkg.CancelStateDelivering, looppkg.CancelStateDraining, looppkg.CancelStateCanceled,
				} {
					if _, err := globalDB.AdvanceNodeCancellation(ctx, mutation, state); err != nil {
						t.Fatalf("AdvanceNodeCancellation(%s, %s) error = %v", status, state, err)
					}
				}
				if status == "waiting" {
					waits, err := globalDB.ListNodeWaits(ctx, run.WorkspaceID, run.ID)
					if err != nil || len(waits) != 0 {
						t.Fatalf("ListNodeWaits(after cancellation) = %#v, %v, want no active waits", waits, err)
					}
					var claimState, claimedByKind, claimedByID string
					if err := globalDB.db.QueryRowContext(ctx, `SELECT claim_state, claimed_by_kind, claimed_by_id
						FROM loop_node_waits WHERE loop_run_id = ? AND node_id = 'work'`, run.ID).Scan(
						&claimState, &claimedByKind, &claimedByID,
					); err != nil {
						t.Fatalf("read canceled wait claim error = %v", err)
					}
					if claimState != string(looppkg.WaitClaimClaimed) ||
						claimedByKind != string(mutation.Actor.Actor.Kind.Normalize()) ||
						claimedByID != mutation.Actor.Actor.Ref {
						t.Fatalf("canceled wait claim = %q/%q/%q", claimState, claimedByKind, claimedByID)
					}
				}
			})
		}
	})
}

// Invariant: ambiguous silence can only raise attention; fresh evidence clears only that flag and
// resets the death-resume streak. This store suite owns the evidence CAS.
func TestGlobalDBLoopNodeLivenessShouldRaiseAndSelfClearSilenceAttention(t *testing.T) {
	t.Parallel()

	globalDB := openLoopTestGlobalDB(t)
	ctx := testutil.Context(t)
	now := time.Date(2026, time.August, 3, 2, 0, 0, 0, time.UTC)
	run, taskRunID := seedLiveLoopLivenessCellForTest(t, globalDB, now)
	observation := looppkg.NodeLivenessObservation{
		WorkspaceID: run.WorkspaceID, LoopRunID: run.ID, TaskRunID: taskRunID,
		ObservedAt: now, Evidence: true, SilenceAfter: 30 * time.Minute,
	}
	if err := globalDB.RecordNodeLiveness(ctx, observation); err != nil {
		t.Fatalf("RecordNodeLiveness(evidence) error = %v", err)
	}
	observation.Evidence = false
	observation.ObservedAt = now.Add(31 * time.Minute)
	if err := globalDB.RecordNodeLiveness(ctx, observation); err != nil {
		t.Fatalf("RecordNodeLiveness(silence) error = %v", err)
	}
	controls, err := globalDB.ListNodeControls(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("ListNodeControls(silence) error = %v", err)
	}
	if len(controls) != 1 || controls[0].AttentionFlag != looppkg.AttentionSilence {
		t.Fatalf("silence controls = %#v, want silence attention", controls)
	}
	if _, err := globalDB.db.ExecContext(ctx, `UPDATE loop_node_controls SET death_resume_streak = 2
		WHERE loop_run_id = ? AND node_id = 'work'`, run.ID); err != nil {
		t.Fatalf("seed death resume streak error = %v", err)
	}
	observation.Evidence = true
	observation.ObservedAt = now.Add(32 * time.Minute)
	if err := globalDB.RecordNodeLiveness(ctx, observation); err != nil {
		t.Fatalf("RecordNodeLiveness(recovery) error = %v", err)
	}
	controls, err = globalDB.ListNodeControls(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("ListNodeControls(recovery) error = %v", err)
	}
	if controls[0].AttentionFlag != "" || controls[0].DeathResumeStreak != 0 ||
		controls[0].LastEvidenceAt == nil || !controls[0].LastEvidenceAt.Equal(observation.ObservedAt) {
		t.Fatalf("recovered controls = %#v, want cleared silence and reset streak", controls[0])
	}
	events, err := globalDB.ListLoopRunEvents(ctx, looppkg.RunEventQuery{
		WorkspaceID: run.WorkspaceID, RunID: run.ID,
	})
	if err != nil {
		t.Fatalf("ListLoopRunEvents() error = %v", err)
	}
	if countLoopEventKindForTest(events, loopRunEventNodeAttentionFlagged) != 1 ||
		countLoopEventKindForTest(events, loopRunEventNodeAttentionCleared) != 1 {
		t.Fatalf("attention events = %#v, want one flagged and one cleared", events)
	}
	if _, err := globalDB.db.ExecContext(ctx, `UPDATE loop_node_controls SET
		attention_flag = 'expired_wait', attention_reason = 'operator intervention required',
		death_resume_streak = 2, revision = 9 WHERE loop_run_id = ? AND node_id = 'work'`, run.ID); err != nil {
		t.Fatalf("seed unrelated attention error = %v", err)
	}
	observation.ObservedAt = now.Add(33 * time.Minute)
	if err := globalDB.RecordNodeLiveness(ctx, observation); err != nil {
		t.Fatalf("RecordNodeLiveness(unrelated attention) error = %v", err)
	}
	var attention, reason string
	var revision, deathStreak int
	var lastEvidenceAt time.Time
	if err := globalDB.db.QueryRowContext(ctx, `SELECT attention_flag, attention_reason,
		revision, death_resume_streak, last_evidence_at FROM loop_node_controls
		WHERE loop_run_id = ? AND node_id = 'work'`, run.ID).
		Scan(&attention, &reason, &revision, &deathStreak, &lastEvidenceAt); err != nil {
		t.Fatalf("read unrelated attention truth error = %v", err)
	}
	if attention != "expired_wait" || reason != "operator intervention required" ||
		revision != 9 || deathStreak != 0 || !lastEvidenceAt.Equal(observation.ObservedAt) {
		t.Fatalf("unrelated attention truth = %q/%q/r%d/streak%d/%v, want preserved alert without revision bump",
			attention, reason, revision, deathStreak, lastEvidenceAt)
	}
}

// Invariant: one confirmed-death identity can retire one live worker and reserve at most one
// checkpoint-carrying continuation. Cancellation, parked state, and the streak cap win without
// mutating the cell or attempt ledger. This store suite owns the atomic death-resume boundary.
func TestGlobalDBResumeDeadNodeShouldCommitOneBoundedContinuation(t *testing.T) {
	t.Parallel()

	t.Run("Should retire the dead worker and replay one continuation", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 3, 3, 0, 0, 0, time.UTC)
		run, taskRunID := seedLiveLoopLivenessCellForTest(t, globalDB, now)
		request := deadNodeResumeRequestForTest(run, taskRunID, 4, now)

		result, err := globalDB.ResumeDeadNode(ctx, request)
		if err != nil {
			t.Fatalf("ResumeDeadNode() error = %v", err)
		}
		if !result.Continued || result.Replay || result.IssuedEpoch != 5 || result.Run.ID == "" {
			t.Fatalf("ResumeDeadNode() = %#v, want new epoch-5 continuation", result)
		}
		oldRun, err := globalDB.GetTaskRun(ctx, taskRunID)
		if err != nil {
			t.Fatalf("GetTaskRun(dead) error = %v", err)
		}
		if oldRun.Status != taskpkg.TaskRunStatusFailed || oldRun.Error != request.Cause {
			t.Fatalf("dead task run = %#v, want failed confirmed-death provenance", oldRun)
		}
		var status, currentTaskRunID string
		var attempt int
		var epoch int64
		if err := globalDB.db.QueryRowContext(ctx, `SELECT status, task_run_id, attempt, epoch
			FROM loop_generation_outputs
			WHERE loop_run_id = ? AND generation = 1 AND node_id = 'work' AND item_index = 0`,
			run.ID).Scan(&status, &currentTaskRunID, &attempt, &epoch); err != nil {
			t.Fatalf("read resumed output error = %v", err)
		}
		if status != "enqueued" || currentTaskRunID != result.Run.ID || attempt != 2 || epoch != 5 {
			t.Fatalf("resumed output = %q/%q/%d/%d, want enqueued/%q/2/5",
				status, currentTaskRunID, attempt, epoch, result.Run.ID)
		}
		var continuationMetadata struct {
			ContinuationKind    string                         `json:"continuation_kind"`
			ResumeFromTaskRunID string                         `json:"resume_from_task_run_id"`
			ResumeFromSessionID string                         `json:"resume_from_session_id"`
			Checkpoint          *looppkg.DeathResumeCheckpoint `json:"death_resume_checkpoint"`
		}
		if err := json.Unmarshal(result.Run.Metadata, &continuationMetadata); err != nil {
			t.Fatalf("decode continuation metadata error = %v", err)
		}
		if continuationMetadata.ContinuationKind != "death_resume" ||
			continuationMetadata.ResumeFromTaskRunID != taskRunID ||
			continuationMetadata.ResumeFromSessionID != request.SourceSessionID ||
			continuationMetadata.Checkpoint == nil || continuationMetadata.Checkpoint.EventEndSeq != 14 {
			t.Fatalf("continuation metadata = %#v, want pinned checkpoint provenance", continuationMetadata)
		}
		var disposition, cause string
		if err := globalDB.db.QueryRowContext(ctx, `SELECT disposition, cause FROM loop_node_attempts
			WHERE loop_run_id = ? AND generation = 1 AND node_id = 'work' AND item_index = 0 AND attempt = 1`,
			run.ID).Scan(&disposition, &cause); err != nil {
			t.Fatalf("read death-resume ledger error = %v", err)
		}
		if disposition != "resumed" || cause != request.Cause {
			t.Fatalf("resume ledger = %q/%q, want resumed/%q", disposition, cause, request.Cause)
		}

		replay, err := globalDB.ResumeDeadNode(ctx, request)
		if err != nil {
			t.Fatalf("ResumeDeadNode(replay) error = %v", err)
		}
		if !replay.Continued || !replay.Replay || replay.Run.ID != result.Run.ID || replay.IssuedEpoch != 5 {
			t.Fatalf("ResumeDeadNode(replay) = %#v, want same continuation", replay)
		}
		assertDeathResumeCountsForTest(t, globalDB, run.ID, result.Run.TaskID, 2, 1)
	})

	t.Run("Should let cancellation and parked state win without continuation", func(t *testing.T) {
		t.Parallel()

		for _, testCase := range []struct {
			name    string
			prepare func(*testing.T, *GlobalDB, looppkg.Run, string, time.Time)
		}{
			{
				name: "run cancellation",
				prepare: func(t *testing.T, globalDB *GlobalDB, run looppkg.Run, _ string, at time.Time) {
					t.Helper()
					_, err := globalDB.RequestRunCancellation(testutil.Context(t), looppkg.CancellationMutation{
						WorkspaceID: run.WorkspaceID, RunID: run.ID, Kind: looppkg.RunCancelCancel,
						Reason: "cancel wins", Actor: operatorActorContextForTest("operator:death-race"), RequestedAt: at,
					})
					if err != nil {
						t.Fatalf("RequestRunCancellation() error = %v", err)
					}
				},
			},
			{
				name: "parked output",
				prepare: func(t *testing.T, globalDB *GlobalDB, run looppkg.Run, _ string, _ time.Time) {
					t.Helper()
					if _, err := globalDB.db.ExecContext(testutil.Context(t), `UPDATE loop_generation_outputs
						SET status = 'paused' WHERE loop_run_id = ? AND node_id = 'work'`, run.ID); err != nil {
						t.Fatalf("park output error = %v", err)
					}
				},
			},
		} {
			t.Run("Should no-op for "+testCase.name, func(t *testing.T) {
				t.Parallel()

				globalDB := openLoopTestGlobalDB(t)
				ctx := testutil.Context(t)
				now := time.Date(2026, time.August, 3, 4, 0, 0, 0, time.UTC)
				run, taskRunID := seedLiveLoopLivenessCellForTest(t, globalDB, now)
				testCase.prepare(t, globalDB, run, taskRunID, now.Add(time.Minute))
				result, err := globalDB.ResumeDeadNode(
					ctx,
					deadNodeResumeRequestForTest(run, taskRunID, 4, now.Add(2*time.Minute)),
				)
				if err != nil {
					t.Fatalf("ResumeDeadNode() error = %v", err)
				}
				if result.Continued || result.Replay {
					t.Fatalf("ResumeDeadNode() = %#v, want deterministic no-op", result)
				}
				assertDeathResumeCountsForTest(t, globalDB, run.ID, "", 1, 0)
			})
		}
	})

	t.Run("Should flag the third no-progress continuation and suppress the fourth", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 3, 5, 0, 0, 0, time.UTC)
		run, taskRunID := seedLiveLoopLivenessCellForTest(t, globalDB, now)
		epoch := int64(4)
		for resume := 1; resume <= 3; resume++ {
			result, err := globalDB.ResumeDeadNode(ctx, deadNodeResumeRequestForTest(
				run, taskRunID, epoch, now.Add(time.Duration(resume)*time.Minute),
			))
			if err != nil {
				t.Fatalf("ResumeDeadNode(%d) error = %v", resume, err)
			}
			if !result.Continued || result.Replay {
				t.Fatalf("ResumeDeadNode(%d) = %#v, want continuation", resume, result)
			}
			taskRunID = result.Run.ID
			epoch = result.IssuedEpoch
		}
		controls, err := globalDB.ListNodeControls(ctx, run.WorkspaceID, run.ID)
		if err != nil {
			t.Fatalf("ListNodeControls() error = %v", err)
		}
		if len(controls) != 1 || controls[0].DeathResumeStreak != 3 ||
			controls[0].AttentionFlag != "resume_exhausted" {
			t.Fatalf("death-resume controls = %#v, want streak 3 + resume_exhausted", controls)
		}
		fourth := deadNodeResumeRequestForTest(run, taskRunID, epoch, now.Add(4*time.Minute))
		result, err := globalDB.ResumeDeadNode(ctx, fourth)
		if err != nil {
			t.Fatalf("ResumeDeadNode(4) error = %v", err)
		}
		if result.Continued || !result.AttentionRequired {
			t.Fatalf("ResumeDeadNode(4) = %#v, want attention-only no-op", result)
		}
		assertDeathResumeCountsForTest(t, globalDB, run.ID, "", 4, 3)
	})
}

// Invariant: requeue clears one active quarantine, appends actor provenance, fences the old
// output, emits one event, and reserves one coordinator wake in the same transaction. The Loop
// store suite owns this atomic repair boundary.
func TestGlobalDBLoopNodeRequeueShouldBeAtomic(t *testing.T) {
	t.Parallel()

	t.Run("Should clear quarantine and reserve one repair generation", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 2, 23, 50, 0, 0, time.UTC)
		run := seedQuarantinedLoopNodeForTest(t, globalDB, now, looppkg.StatusRunning)
		expectedRevision := int64(4)
		requeueReason := strings.Repeat("target repaired ", 2_000) + "api_key=planted-secret"
		mutation := looppkg.NodeRequeueMutation{
			WorkspaceID:      looppkg.WorkspaceID(" " + string(run.WorkspaceID) + " "),
			RunID:            looppkg.RunID(" " + string(run.ID) + " "),
			NodeID:           " finish ",
			Reason:           requeueReason,
			ExpectedRevision: &expectedRevision,
			Actor:            operatorActorContextForTest("operator:alice"),
			RequestedAt:      now.Add(time.Minute),
		}
		if _, err := globalDB.db.ExecContext(ctx, `CREATE TEMP TRIGGER fail_requeue_task_status_event
			BEFORE INSERT ON task_events
			WHEN NEW.event_type = 'task.status_changed'
			BEGIN SELECT RAISE(ABORT, 'forced requeue task status event failure'); END`); err != nil {
			t.Fatalf("create requeue task status event failure trigger error = %v", err)
		}
		if _, err := globalDB.RequeueNode(ctx, mutation); err == nil ||
			!strings.Contains(err.Error(), "forced requeue task status event failure") {
			t.Fatalf("RequeueNode(forced event failure) error = %v", err)
		}
		cellTaskID := looppkg.NodeCellTaskID(run.ID, 2, "finish", 0)
		stillParked, err := globalDB.GetTask(ctx, cellTaskID)
		if err != nil {
			t.Fatalf("GetTask(after rolled back requeue) error = %v", err)
		}
		if stillParked.Status != taskpkg.TaskStatusNeedsAttention || stillParked.NeedsAttention == nil {
			t.Fatalf("task after rolled back requeue = %#v, want unchanged park", stillParked)
		}
		rolledBackControls, err := globalDB.ListNodeControls(ctx, run.WorkspaceID, run.ID)
		if err != nil {
			t.Fatalf("ListNodeControls(after rolled back requeue) error = %v", err)
		}
		finishRolledBack := slices.ContainsFunc(rolledBackControls, func(control looppkg.NodeControl) bool {
			return control.NodeID == "finish" && control.Quarantined && control.Revision == 4
		})
		if len(rolledBackControls) != 3 || !finishRolledBack {
			t.Fatalf("controls after rolled back requeue = %#v, want finish quarantine revision 4", rolledBackControls)
		}
		if got := countTaskStatusEventsForTest(t, globalDB, cellTaskID); got != 0 {
			t.Fatalf("requeue task status events after rollback = %d, want 0", got)
		}
		if _, err := globalDB.db.ExecContext(ctx, `DROP TRIGGER fail_requeue_task_status_event`); err != nil {
			t.Fatalf("drop requeue task status event failure trigger error = %v", err)
		}
		result, err := globalDB.RequeueNode(ctx, mutation)
		if err != nil {
			t.Fatalf("RequeueNode() error = %v", err)
		}
		if result.Control.NodeID != "finish" || result.Control.Quarantined || result.Control.Revision != 5 ||
			result.Coordinator.Status != taskpkg.TaskRunStatusQueued {
			t.Fatalf("RequeueNode() result = %#v, want cleared control and queued coordinator", result)
		}
		var entry looppkg.QuarantineEntry
		if err := json.Unmarshal(result.Control.QuarantineEntry, &entry); err != nil {
			t.Fatalf("json.Unmarshal(quarantine entry) error = %v", err)
		}
		if len(entry.Requeues) != 1 || entry.Requeues[0].ActorID != "operator:alice" ||
			entry.Requeues[0].Generation != 3 ||
			strings.Contains(entry.Requeues[0].Reason, "planted-secret") {
			t.Fatalf("requeue provenance = %#v, want sanitized generation-3 actor", entry.Requeues)
		}
		controls, err := globalDB.ListNodeControls(ctx, run.WorkspaceID, run.ID)
		if err != nil {
			t.Fatalf("ListNodeControls() error = %v", err)
		}
		controlByNode := make(map[string]looppkg.NodeControl, len(controls))
		for _, control := range controls {
			controlByNode[string(control.NodeID)] = control
		}
		if len(controls) != 3 || controlByNode["finish"].Quarantined || controlByNode["finish"].Revision != 5 {
			t.Fatalf("controls = %#v, want cleared quarantine on finish", controls)
		}
		if flagged := controlByNode["finish_consumer"]; flagged.AttentionFlag != "" ||
			flagged.AttentionProducerNodeID != "" {
			t.Fatalf("finish consumer control = %#v, want released dependency attention", flagged)
		}
		if unrelated := controlByNode["other_consumer"]; unrelated.AttentionFlag != "dependency_quarantined" ||
			unrelated.AttentionProducerNodeID != "other_producer" {
			t.Fatalf("unrelated control = %#v, want preserved attention", unrelated)
		}
		var attentionAt sql.NullTime
		var taskStatus string
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT needs_attention_at, status FROM tasks WHERE id = ?`,
			cellTaskID,
		).Scan(&attentionAt, &taskStatus); err != nil {
			t.Fatalf("read requeued cell task attention error = %v", err)
		}
		if attentionAt.Valid || taskStatus != string(taskpkg.TaskStatusReady) {
			t.Fatalf(
				"cell task attention/status = %v/%q, want cleared and ready on requeue",
				attentionAt,
				taskStatus,
			)
		}
		statusPayload := singleTaskStatusEventPayloadForTest(
			t,
			globalDB,
			cellTaskID,
		)
		if statusPayload.FromStatus != string(taskpkg.TaskStatusNeedsAttention) ||
			statusPayload.ToStatus != string(taskpkg.TaskStatusReady) ||
			statusPayload.Reason != diagnostics.RedactAndBound(requeueReason, 1024) ||
			statusPayload.LoopRunID != string(run.ID) ||
			statusPayload.ActorKind != string(taskpkg.ActorKindHuman) ||
			statusPayload.ActorID != "operator:alice" {
			t.Fatalf("requeue task status event = %#v, want audited unpark", statusPayload)
		}
		if _, replayErr := globalDB.RequeueNode(ctx, mutation); replayErr == nil {
			t.Fatal("RequeueNode(replay) error = nil, want quarantine conflict")
		}
		if got := countTaskStatusEventsForTest(
			t,
			globalDB,
			cellTaskID,
		); got != 1 {
			t.Fatalf("requeue task status events after conflict = %d, want 1", got)
		}
		var epoch int64
		var firstScheduledAt time.Time
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT epoch, first_scheduled_at FROM loop_generation_outputs
			 WHERE loop_run_id = ? AND generation = 2 AND node_id = 'finish'`,
			run.ID,
		).Scan(&epoch, &firstScheduledAt); err != nil {
			t.Fatalf("read fenced output epoch error = %v", err)
		}
		storedRun, err := globalDB.GetLoopRun(ctx, run.WorkspaceID, run.ID)
		if err != nil {
			t.Fatalf("GetLoopRun() error = %v", err)
		}
		if epoch != 10 || !firstScheduledAt.Equal(now) || !storedRun.StartedAt.Equal(now.Add(time.Minute)) {
			t.Fatalf("requeue clocks = epoch:%d first:%s started:%s, want 10/%s/%s",
				epoch, firstScheduledAt, storedRun.StartedAt, now, now.Add(time.Minute))
		}
		events, err := globalDB.ListLoopRunEvents(ctx, looppkg.RunEventQuery{
			WorkspaceID: run.WorkspaceID,
			RunID:       run.ID,
		})
		if err != nil {
			t.Fatalf("ListLoopRunEvents() error = %v", err)
		}
		if got := countLoopEventKindForTest(events, loopRunEventNodeRequeued); got != 1 {
			t.Fatalf("node_requeued events = %d, want 1", got)
		}
		var requeuePayload map[string]any
		for _, event := range events {
			if event.Kind != loopRunEventNodeRequeued {
				continue
			}
			if err := json.Unmarshal(event.Payload, &requeuePayload); err != nil {
				t.Fatalf("json.Unmarshal(node_requeued payload) error = %v", err)
			}
		}
		reason, _ := requeuePayload[loopRunEventPayloadKeyReason].(string)
		if requeuePayload[loopRunEventPayloadKeyNodeID] != "finish" || reason == "" ||
			strings.Contains(reason, "planted-secret") || len(reason) > 1024 {
			t.Fatalf("node_requeued payload = %#v, want bounded structured provenance", requeuePayload)
		}

		claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			RunID:            result.Coordinator.ID,
			Scope:            taskpkg.ScopeWorkspace,
			WorkspaceID:      string(run.WorkspaceID),
			RunKind:          taskpkg.RunKindCoordinator,
			ClaimerSessionID: "daemon-loop-requeue",
			ClaimedBy:        &taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "loop"},
			LeaseDuration:    5 * time.Minute,
			Now:              now.Add(2 * time.Minute),
		})
		if err != nil {
			t.Fatalf("ClaimNextRun(requeue) error = %v", err)
		}
		runner, err := looppkg.NewCoordinatorRunner(
			globalDB,
			globalDB,
			globalDB,
			slog.Default(),
			looppkg.WithCoordinatorNodeControlReader(globalDB),
		)
		if err != nil {
			t.Fatalf("NewCoordinatorRunner() error = %v", err)
		}
		plan, err := runner.Run(ctx, taskpkg.RunID(claim.Run.ID))
		if err != nil {
			t.Fatalf("CoordinatorRunner.Run(requeue) error = %v", err)
		}
		if _, err := globalDB.CompleteCoordinatorAndEnqueueNext(
			ctx,
			taskpkg.CoordinatorCompletion{
				RunID: claim.Run.ID, ClaimToken: claim.ClaimToken,
				Actor: coordinatorActorContextForTest(), Now: now.Add(3 * time.Minute), Plan: plan,
			},
			looppkg.NewStoreFinalizer(),
		); err != nil {
			t.Fatalf("CompleteCoordinatorAndEnqueueNext(requeue) error = %v", err)
		}
		advanced, err := globalDB.GetLoopRunByID(ctx, run.ID)
		if err != nil {
			t.Fatalf("GetLoopRunByID(after requeue) error = %v", err)
		}
		var generationCount int
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM loop_generations WHERE loop_run_id = ?`,
			run.ID,
		).Scan(&generationCount); err != nil {
			t.Fatalf("count loop generations error = %v", err)
		}
		var origin string
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT origin FROM loop_generations WHERE loop_run_id = ? AND generation = 3`,
			run.ID,
		).Scan(&origin); err != nil {
			t.Fatalf("read requeue generation origin error = %v", err)
		}
		if advanced.Generation != 3 || generationCount != advanced.Generation ||
			origin != string(looppkg.OriginRequeue) {
			t.Fatalf(
				"generation cursor/count/origin = %d/%d/%q, want 3/3/requeue",
				advanced.Generation,
				generationCount,
				origin,
			)
		}
	})

	t.Run("Should preserve the fenced epoch on a requeued continuation", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 2, 23, 52, 0, 0, time.UTC)
		run := seedQuarantinedLoopNodeForTest(t, globalDB, now, looppkg.StatusRunning)
		continuationTask := taskRecordForTest(looppkg.NodeCellTaskID(run.ID, 3, "finish", 0))
		continuationTask.WorkspaceID = string(run.WorkspaceID)
		continuationTask.Scope = taskpkg.ScopeWorkspace
		continuationTask.Metadata = json.RawMessage(
			`{"generation":3,"node_id":"finish","item_index":0,"attempt":1,"epoch":1}`,
		)
		if err := globalDB.CreateTask(ctx, continuationTask); err != nil {
			t.Fatalf("CreateTask(quarantined continuation) error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE tasks SET status = 'needs_attention',
			 needs_attention_reason = 'loop node finish is quarantined', needs_attention_at = ?,
			 needs_attention_by_kind = 'daemon', needs_attention_by_ref = 'loop-coordinator'
			 WHERE id = ?`,
			storepkg.FormatTimestamp(now),
			continuationTask.ID,
		); err != nil {
			t.Fatalf("mark continuation needs attention error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`INSERT INTO loop_generation_outputs
			 (loop_run_id, generation, node_id, item_index, status, attempt, epoch)
			 VALUES (?, 3, 'finish', 0, 'quarantined', 1, 1)`,
			run.ID,
		); err != nil {
			t.Fatalf("insert quarantined continuation output error = %v", err)
		}
		expectedRevision := int64(4)
		result, err := globalDB.RequeueNode(ctx, looppkg.NodeRequeueMutation{
			WorkspaceID:      run.WorkspaceID,
			RunID:            run.ID,
			NodeID:           "finish",
			Reason:           "operator repaired the target",
			ExpectedRevision: &expectedRevision,
			Actor:            operatorActorContextForTest("operator:alice"),
			RequestedAt:      now.Add(time.Minute),
		})
		if err != nil {
			t.Fatalf("RequeueNode() error = %v", err)
		}
		var workerRunID string
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT task_run_id FROM loop_generation_outputs
			 WHERE loop_run_id = ? AND generation = 3 AND node_id = 'finish' AND item_index = 0`,
			run.ID,
		).Scan(&workerRunID); err != nil {
			t.Fatalf("read requeued worker run id error = %v", err)
		}
		wantWorkerRunID := looppkg.NodeCellAttemptRunID(run.ID, 3, "finish", 0, 1)
		if workerRunID != wantWorkerRunID {
			t.Fatalf("requeued worker run id = %q, want preserved attempt %q", workerRunID, wantWorkerRunID)
		}
		if len(result.Workers) != 1 || result.Workers[0].ID != wantWorkerRunID {
			t.Fatalf("requeued worker activations = %#v, want %q", result.Workers, wantWorkerRunID)
		}
		var workerMetadata struct {
			Attempt int   `json:"attempt"`
			Epoch   int64 `json:"epoch"`
		}
		if err := json.Unmarshal(result.Workers[0].Metadata, &workerMetadata); err != nil {
			t.Fatalf("json.Unmarshal(requeued worker metadata) error = %v", err)
		}
		if workerMetadata.Attempt != 1 || workerMetadata.Epoch != 2 {
			t.Fatalf("requeued worker attempt/epoch = %d/%d, want 1/2", workerMetadata.Attempt, workerMetadata.Epoch)
		}
		claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			RunID:            workerRunID,
			Scope:            taskpkg.ScopeWorkspace,
			WorkspaceID:      string(run.WorkspaceID),
			RunKind:          taskpkg.RunKindWorker,
			ClaimerSessionID: "daemon-loop-requeue-worker",
			ClaimedBy:        &taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "loop"},
			LeaseDuration:    5 * time.Minute,
			Now:              now.Add(2 * time.Minute),
		})
		if err != nil {
			t.Fatalf("ClaimNextRun(requeued worker) error = %v", err)
		}
		if _, err := globalDB.CompleteRunLease(ctx, taskpkg.LeaseCompletion{
			RunID:      claim.Run.ID,
			ClaimToken: claim.ClaimToken,
			Result:     taskpkg.RunResult{Value: json.RawMessage(`{"summary":"repaired"}`)},
			Actor:      coordinatorActorContextForTest(),
			Now:        now.Add(3 * time.Minute),
		}); err != nil {
			t.Fatalf("CompleteRunLease(requeued worker) error = %v", err)
		}
		var outputStatus string
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT status FROM loop_generation_outputs
			 WHERE loop_run_id = ? AND generation = 3 AND node_id = 'finish' AND item_index = 0`,
			run.ID,
		).Scan(&outputStatus); err != nil {
			t.Fatalf("read requeued output status error = %v", err)
		}
		if outputStatus != loopNodeOutputSucceeded || strings.TrimSpace(result.Coordinator.ID) == "" {
			t.Fatalf(
				"requeued output/coordinator = %q/%q, want succeeded with a wake",
				outputStatus,
				result.Coordinator.ID,
			)
		}
		var generationOrigin string
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT origin FROM loop_generations WHERE loop_run_id = ? AND generation = 3`,
			run.ID,
		).Scan(&generationOrigin); err != nil {
			t.Fatalf("read retained continuation generation origin error = %v", err)
		}
		if generationOrigin != string(looppkg.OriginRequeue) {
			t.Fatalf("retained continuation generation origin = %q, want requeue", generationOrigin)
		}
	})

	t.Run("Should return concurrent requeue winner provenance to the loser", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		now := time.Date(2026, time.August, 2, 23, 53, 0, 0, time.UTC)
		run := seedQuarantinedLoopNodeForTest(t, globalDB, now, looppkg.StatusRunning)
		expectedRevision := int64(4)
		results := make([]looppkg.NodeRequeueResult, 2)
		errs := make([]error, 2)
		start := make(chan struct{})
		var waitGroup sync.WaitGroup
		waitGroup.Add(2)
		for index := range results {
			go func(index int) {
				defer waitGroup.Done()
				<-start
				results[index], errs[index] = globalDB.RequeueNode(context.Background(), looppkg.NodeRequeueMutation{
					WorkspaceID:      run.WorkspaceID,
					RunID:            run.ID,
					NodeID:           "finish",
					Reason:           fmt.Sprintf("repair-%d", index),
					ExpectedRevision: &expectedRevision,
					Actor:            operatorActorContextForTest(fmt.Sprintf("operator:race-%d", index)),
					RequestedAt:      now.Add(time.Duration(index+1) * time.Minute),
				})
			}(index)
		}
		close(start)
		waitGroup.Wait()

		winner := -1
		loser := -1
		for index, mutationErr := range errs {
			if mutationErr == nil {
				winner = index
			} else {
				loser = index
			}
		}
		if winner < 0 || loser < 0 {
			t.Fatalf("concurrent requeue errors = %v, want one winner and one loser", errs)
		}
		var winnerEntry looppkg.QuarantineEntry
		if err := json.Unmarshal(results[winner].Control.QuarantineEntry, &winnerEntry); err != nil {
			t.Fatalf("json.Unmarshal(winner quarantine entry) error = %v", err)
		}
		if len(winnerEntry.Requeues) != 1 {
			t.Fatalf("winner requeue provenance = %#v", winnerEntry.Requeues)
		}
		var reason *looppkg.ReasonError
		if !errors.As(errs[loser], &reason) || reason.Code != looppkg.ReasonCodeAlreadyDecided {
			t.Fatalf("loser error = %v, want already_decided ReasonError", errs[loser])
		}
		if reason.Meta[looppkg.ReasonMetaActualState] != nodeLifecycleStateActive ||
			reason.Meta[looppkg.ReasonMetaAllowedTransitions] != "pause,cancel,kill" ||
			reason.Meta[looppkg.ReasonMetaWinnerActorID] != winnerEntry.Requeues[0].ActorID ||
			reason.Meta[looppkg.ReasonMetaWinnerReason] != winnerEntry.Requeues[0].Reason {
			t.Fatalf("loser reason metadata = %#v, winner = %#v", reason.Meta, winnerEntry.Requeues[0])
		}
	})

	t.Run("Should reject a terminal run without clearing its entry", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 2, 23, 55, 0, 0, time.UTC)
		run := seedQuarantinedLoopNodeForTest(t, globalDB, now, looppkg.StatusFailed)
		_, err := globalDB.RequeueNode(ctx, looppkg.NodeRequeueMutation{
			WorkspaceID: run.WorkspaceID,
			RunID:       run.ID,
			NodeID:      "finish",
			Actor:       operatorActorContextForTest("operator:bob"),
			RequestedAt: now.Add(time.Minute),
		})
		if !errors.Is(err, looppkg.ErrInvalidTransition) {
			t.Fatalf("RequeueNode(terminal) error = %v, want ErrInvalidTransition", err)
		}
		var reason *looppkg.ReasonError
		if !errors.As(err, &reason) || reason.Code != looppkg.ReasonCodeRunTerminal ||
			reason.Meta[looppkg.ReasonMetaActualState] != string(looppkg.StatusFailed) ||
			reason.Meta[looppkg.ReasonMetaAllowedTransitions] != "" {
			t.Fatalf("RequeueNode(terminal) ReasonError = %#v", reason)
		}
		controls, listErr := globalDB.ListNodeControls(ctx, run.WorkspaceID, run.ID)
		if listErr != nil {
			t.Fatalf("ListNodeControls() error = %v", listErr)
		}
		if len(controls) != 3 || !controls[0].Quarantined || len(controls[0].QuarantineEntry) == 0 {
			t.Fatalf("terminal controls = %#v, want inspectable quarantine unchanged", controls)
		}
	})

	t.Run("Should reject a cross-workspace requeue without exposing its entry", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 2, 23, 56, 0, 0, time.UTC)
		run := seedQuarantinedLoopNodeForTest(t, globalDB, now, looppkg.StatusRunning)
		_, err := globalDB.RequeueNode(ctx, looppkg.NodeRequeueMutation{
			WorkspaceID: "ws-other",
			RunID:       run.ID,
			NodeID:      "finish",
			Actor:       operatorActorContextForTest("operator:mallory"),
			RequestedAt: now.Add(time.Minute),
		})
		if !errors.Is(err, looppkg.ErrRunNotFound) {
			t.Fatalf("RequeueNode(cross-workspace) error = %v, want ErrRunNotFound", err)
		}
		controls, listErr := globalDB.ListNodeControls(ctx, run.WorkspaceID, run.ID)
		if listErr != nil {
			t.Fatalf("ListNodeControls() error = %v", listErr)
		}
		if len(controls) != 3 || !controls[0].Quarantined || controls[0].Revision != 4 {
			t.Fatalf("cross-workspace controls = %#v, want unchanged quarantine", controls)
		}
	})

	t.Run("Should reject requeue beyond the iteration cap", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 2, 23, 58, 0, 0, time.UTC)
		run := seedQuarantinedLoopNodeForTest(t, globalDB, now, looppkg.StatusRunning)
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_runs SET iteration_cap = generation WHERE id = ?`,
			run.ID,
		); err != nil {
			t.Fatalf("set exhausted iteration cap error = %v", err)
		}
		_, err := globalDB.RequeueNode(ctx, looppkg.NodeRequeueMutation{
			WorkspaceID: run.WorkspaceID,
			RunID:       run.ID,
			NodeID:      "finish",
			Actor:       operatorActorContextForTest("operator:cap"),
			RequestedAt: now.Add(time.Minute),
		})
		if !errors.Is(err, looppkg.ErrInvalidTransition) {
			t.Fatalf("RequeueNode(cap) error = %v, want ErrInvalidTransition", err)
		}
		controls, listErr := globalDB.ListNodeControls(ctx, run.WorkspaceID, run.ID)
		if listErr != nil {
			t.Fatalf("ListNodeControls() error = %v", listErr)
		}
		if len(controls) != 3 || !controls[0].Quarantined || controls[0].Revision != 4 {
			t.Fatalf("cap controls = %#v, want unchanged quarantine", controls)
		}
	})
}

// Invariant: coordinator boundaries persist gate revision counters by gate node and item lane.
// The GlobalDB Loop suite owns the durable node-control projection.
func TestGlobalDBGateRevisionCountersShouldPersistPerItem(t *testing.T) {
	t.Parallel()

	t.Run("Should round-trip isolated item counters through a coordinator boundary", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 16, 12, 0, 0, 0, time.UTC)
		run, err := globalDB.CreateLoopRunForStart(
			ctx,
			testLoopRun("looprun-gate-revisions", now, looppkg.StatusRunning),
			dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		claim := claimCoordinatorRunForTest(ctx, t, globalDB, run.ID, "gate-revisions", now)
		_, err = globalDB.CompleteCoordinatorAndEnqueueNext(
			ctx,
			taskpkg.CoordinatorCompletion{
				RunID: claim.Run.ID, ClaimToken: claim.ClaimToken,
				Actor: coordinatorActorContextForTest(), Now: now.Add(time.Second),
				Plan: taskpkg.CoordinatorCompletionPlan{
					Yield: true,
					Snapshot: taskpkg.GenerationSnapshot{
						LoopRunID: string(run.ID), Generation: 1,
						Payload: looppkg.GenerationSnapshotPayload{
							Controls: []looppkg.NodeControlMutation{{
								Kind: looppkg.NodeControlMutationGateRevision, NodeID: "quality_gate",
								GateRevisions: map[int]int{0: 1, 2: 3}, At: now.Add(time.Second),
							}},
						},
					},
				},
			},
			looppkg.NewStoreFinalizer(),
		)
		if err != nil {
			t.Fatalf("CompleteCoordinatorAndEnqueueNext() error = %v", err)
		}
		controls, err := globalDB.ListNodeControls(ctx, run.WorkspaceID, run.ID)
		if err != nil {
			t.Fatalf("ListNodeControls() error = %v", err)
		}
		if len(controls) != 1 || controls[0].NodeID != "quality_gate" ||
			len(controls[0].GateRevisions) != 2 || controls[0].GateRevisions[0] != 1 ||
			controls[0].GateRevisions[2] != 3 {
			t.Fatalf("gate revision controls = %#v, want isolated lanes {0:1, 2:3}", controls)
		}
	})
}

// Invariant: a quarantine boundary atomically parks its reserved continuation while
// preserving any completed cell task from the quarantined generation.
func TestGlobalDBQuarantineSnapshotShouldParkCellTasks(t *testing.T) {
	t.Parallel()
	t.Run("Should park the reserved continuation without rewriting a completed cell", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 21, 10, 0, 0, 0, time.UTC)
		created, err := globalDB.CreateLoopRunForStart(
			ctx,
			testLoopRun("looprun-quarantine-continuation", now, looppkg.StatusRunning),
			dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		if err := insertLoopGenerationWithExecutor(ctx, globalDB.db, created.ID, looppkg.GenerationIntent{
			Generation: 2, ParentGeneration: 1, Origin: looppkg.OriginReattempt,
		}, now.Add(time.Second)); err != nil {
			t.Fatalf("insert generation-2 provenance error = %v", err)
		}
		completedTaskID := looppkg.NodeCellTaskID(created.ID, 2, "primary", 0)
		completedTask := workspaceTaskRecordForTest(completedTaskID, string(created.WorkspaceID))
		completedTask.ParentTaskID = loopCoordinatorTaskID(created.ID)
		completedTask.Status = taskpkg.TaskStatusCompleted
		completedTask.ClosedAt = now.Add(time.Second)
		if err := globalDB.CreateTask(ctx, completedTask); err != nil {
			t.Fatalf("CreateTask(completed generation-2 cell) error = %v", err)
		}
		claim := claimCoordinatorRunForTest(
			ctx,
			t,
			globalDB,
			created.ID,
			"quarantine-continuation",
			now.Add(2*time.Second),
		)
		entry, err := json.Marshal(looppkg.QuarantineEntry{
			NodeID:   "primary",
			InputRef: "loop-run:" + string(created.ID) + ":node:primary:input",
			Episodes: []looppkg.QuarantineEpisode{{
				Generation: 2, QuarantinedAt: now.Add(3 * time.Second),
				Attempts: []looppkg.NodeAttempt{{
					LoopRunID: created.ID, Generation: 2, NodeID: "primary", Attempt: 1,
					Disposition: looppkg.AttemptQuarantined, StartedAt: now,
				}},
			}},
		})
		if err != nil {
			t.Fatalf("json.Marshal(quarantine entry) error = %v", err)
		}
		continuationTaskID := looppkg.NodeCellTaskID(created.ID, 3, "primary", 0)
		completion := taskpkg.CoordinatorCompletion{
			RunID: claim.Run.ID, ClaimToken: claim.ClaimToken,
			Actor: coordinatorActorContextForTest(), Now: now.Add(3 * time.Second),
			Plan: taskpkg.CoordinatorCompletionPlan{
				Snapshot: taskpkg.GenerationSnapshot{
					LoopRunID: string(created.ID), Generation: 2,
					Payload: looppkg.GenerationSnapshotPayload{
						Outputs: []looppkg.GenerationOutput{{
							Generation: 2, NodeID: "primary", Status: "quarantined", Attempt: 1, Epoch: 1,
						}},
						Controls: []looppkg.NodeControlMutation{{
							Kind: looppkg.NodeControlMutationQuarantine, NodeID: "primary",
							QuarantineEntry: entry, At: now.Add(3 * time.Second),
						}},
					},
				},
				PostReserveSnapshot: &taskpkg.GenerationSnapshot{
					LoopRunID: string(created.ID), Generation: 3,
					Payload: looppkg.GenerationSnapshotPayload{
						Outputs: []looppkg.GenerationOutput{{
							Generation: 3, NodeID: "primary", Status: "quarantined", Attempt: 1, Epoch: 1,
						}},
						GenerationProvenance: &looppkg.GenerationIntent{
							Generation: 3, ParentGeneration: 2, Origin: looppkg.OriginReattempt,
						},
					},
				},
				NextCoordinator: &taskpkg.EnqueueSpec{
					TaskID:         loopCoordinatorTaskID(created.ID),
					RunID:          loopCoordinatorRunID(created.ID, 3),
					RunKind:        taskpkg.RunKindCoordinator,
					LoopRunID:      string(created.ID),
					IdempotencyKey: "quarantine-completed-source-g3",
				},
			},
		}
		if _, err := globalDB.CompleteCoordinatorAndEnqueueNext(
			ctx,
			completion,
			looppkg.NewStoreFinalizer(),
		); err != nil {
			t.Fatalf("CompleteCoordinatorAndEnqueueNext(quarantine continuation) error = %v", err)
		}
		completed, err := globalDB.GetTask(ctx, completedTaskID)
		if err != nil {
			t.Fatalf("GetTask(completed generation-2 cell) error = %v", err)
		}
		if completed.Status != taskpkg.TaskStatusCompleted || completed.NeedsAttention != nil {
			t.Fatalf("completed generation-2 cell = %#v, want completed without attention", completed)
		}
		continuation, err := globalDB.GetTask(ctx, continuationTaskID)
		if err != nil {
			t.Fatalf("GetTask(generation-3 continuation) error = %v", err)
		}
		if continuation.Status != taskpkg.TaskStatusNeedsAttention || continuation.NeedsAttention == nil {
			t.Fatalf("generation-3 continuation = %#v, want quarantined needs-attention", continuation)
		}
		if got := countTaskStatusEventsForTest(t, globalDB, completedTaskID); got != 0 {
			t.Fatalf("completed generation-2 cell status events = %d, want none", got)
		}
		statusPayload := singleTaskStatusEventPayloadForTest(t, globalDB, continuationTaskID)
		if statusPayload.FromStatus != string(taskpkg.TaskStatusReady) ||
			statusPayload.ToStatus != string(taskpkg.TaskStatusNeedsAttention) ||
			statusPayload.LoopRunID != string(created.ID) {
			t.Fatalf("generation-3 continuation status event = %#v, want audited quarantine park", statusPayload)
		}
		controls, err := globalDB.ListNodeControls(ctx, created.WorkspaceID, created.ID)
		if err != nil {
			t.Fatalf("ListNodeControls() error = %v", err)
		}
		if len(controls) != 1 || !controls[0].Quarantined || controls[0].NodeID != "primary" {
			t.Fatalf("node controls = %#v, want committed primary quarantine", controls)
		}
		outputs, err := globalDB.ListGenerationOutputs(ctx, created.WorkspaceID, created.ID, 2)
		if err != nil {
			t.Fatalf("ListGenerationOutputs() error = %v", err)
		}
		if len(outputs) != 1 || outputs[0].Status != "quarantined" {
			t.Fatalf("generation-2 outputs = %#v, want committed quarantine snapshot", outputs)
		}
		persistedRun, err := globalDB.GetLoopRun(ctx, created.WorkspaceID, created.ID)
		if err != nil {
			t.Fatalf("GetLoopRun() error = %v", err)
		}
		if persistedRun.Generation != 2 {
			t.Fatalf("quarantined run generation = %d, want parked at generation 2", persistedRun.Generation)
		}
		replayClaim := claimCoordinatorRunForTest(
			ctx, t, globalDB, created.ID, "quarantine-completed-source-replay", now.Add(4*time.Second),
		)
		if _, err := globalDB.CompleteCoordinatorAndEnqueueNext(
			ctx,
			taskpkg.CoordinatorCompletion{
				RunID: replayClaim.Run.ID, ClaimToken: replayClaim.ClaimToken,
				Actor: coordinatorActorContextForTest(), Now: now.Add(5 * time.Second),
				Plan: taskpkg.CoordinatorCompletionPlan{
					Yield: true,
					Snapshot: taskpkg.GenerationSnapshot{
						LoopRunID: string(created.ID), Generation: 2,
						Payload: looppkg.GenerationSnapshotPayload{Outputs: []looppkg.GenerationOutput{{
							Generation: 2, NodeID: "primary", Status: "quarantined", Attempt: 1, Epoch: 1,
						}}},
					},
				},
			},
			looppkg.NewStoreFinalizer(),
		); err != nil {
			t.Fatalf("CompleteCoordinatorAndEnqueueNext(completed-source replay) error = %v", err)
		}
		replayedCompleted, err := globalDB.GetTask(ctx, completedTaskID)
		if err != nil {
			t.Fatalf("GetTask(completed source after replay) error = %v", err)
		}
		if replayedCompleted.Status != taskpkg.TaskStatusCompleted || replayedCompleted.NeedsAttention != nil {
			t.Fatalf("completed source after replay = %#v, want unchanged completed", replayedCompleted)
		}
	})

	t.Run("Should park the continuation after the source run fails and its task becomes retryable", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 22, 10, 0, 0, 0, time.UTC)
		created, err := globalDB.CreateLoopRunForStart(
			ctx,
			testLoopRun("looprun-quarantine-failed-source", now, looppkg.StatusRunning),
			dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		if err := insertLoopGenerationWithExecutor(ctx, globalDB.db, created.ID, looppkg.GenerationIntent{
			Generation: 2, ParentGeneration: 1, Origin: looppkg.OriginReattempt,
		}, now.Add(time.Second)); err != nil {
			t.Fatalf("insert generation-2 provenance error = %v", err)
		}
		sourceTaskID := looppkg.NodeCellTaskID(created.ID, 2, "review", 0)
		sourceTask := workspaceTaskRecordForTest(sourceTaskID, string(created.WorkspaceID))
		sourceTask.ParentTaskID = loopCoordinatorTaskID(created.ID)
		sourceTask.Status = taskpkg.TaskStatusReady
		if err := globalDB.CreateTask(ctx, sourceTask); err != nil {
			t.Fatalf("CreateTask(generation-2 source) error = %v", err)
		}
		sourceRun := taskRunForTest("taskrun-quarantine-failed-source", sourceTaskID)
		sourceRun.LoopRunID = string(created.ID)
		sourceRun.RunKind = taskpkg.RunKindWorker
		if err := globalDB.CreateTaskRun(ctx, sourceRun); err != nil {
			t.Fatalf("CreateTaskRun(source) error = %v", err)
		}
		sourceRun, err = globalDB.GetTaskRun(ctx, sourceRun.ID)
		if err != nil {
			t.Fatalf("GetTaskRun(source) error = %v", err)
		}
		failed, err := forceFailTaskRunForTest(
			ctx,
			globalDB,
			taskpkg.NewForceFailRunMutation(sourceRun, "schema-invalid reviewer output", now.Add(2*time.Second)),
		)
		if err != nil {
			t.Fatalf("ForceFailTaskRun(source) error = %v", err)
		}
		if failed.Run.Status != taskpkg.TaskRunStatusFailed {
			t.Fatalf("source run status = %q, want failed", failed.Run.Status)
		}
		claim := claimCoordinatorRunForTest(
			ctx,
			t,
			globalDB,
			created.ID,
			"quarantine-failed-source",
			now.Add(3*time.Second),
		)
		entry, err := json.Marshal(looppkg.QuarantineEntry{
			NodeID:   "review",
			InputRef: "loop-run:" + string(created.ID) + ":node:review:input",
			Episodes: []looppkg.QuarantineEpisode{{
				Generation: 2, QuarantinedAt: now.Add(4 * time.Second),
				Attempts: []looppkg.NodeAttempt{{
					LoopRunID: created.ID, Generation: 2, NodeID: "review", Attempt: 1,
					Disposition: looppkg.AttemptQuarantined, StartedAt: now,
				}},
			}},
		})
		if err != nil {
			t.Fatalf("json.Marshal(quarantine entry) error = %v", err)
		}
		continuationTaskID := looppkg.NodeCellTaskID(created.ID, 3, "review", 0)
		completion := taskpkg.CoordinatorCompletion{
			RunID: claim.Run.ID, ClaimToken: claim.ClaimToken,
			Actor: coordinatorActorContextForTest(), Now: now.Add(4 * time.Second),
			Plan: taskpkg.CoordinatorCompletionPlan{
				Snapshot: taskpkg.GenerationSnapshot{
					LoopRunID: string(created.ID), Generation: 2,
					Payload: looppkg.GenerationSnapshotPayload{
						Outputs: []looppkg.GenerationOutput{{
							Generation: 2, NodeID: "review", Status: "quarantined", TaskRunID: sourceRun.ID,
							Attempt: 1, Epoch: 1,
						}},
						Controls: []looppkg.NodeControlMutation{{
							Kind: looppkg.NodeControlMutationQuarantine, NodeID: "review",
							QuarantineEntry: entry, At: now.Add(4 * time.Second),
						}},
					},
				},
				PostReserveSnapshot: &taskpkg.GenerationSnapshot{
					LoopRunID: string(created.ID), Generation: 3,
					Payload: looppkg.GenerationSnapshotPayload{
						Outputs: []looppkg.GenerationOutput{{
							Generation: 3, NodeID: "review", Status: "quarantined", TaskRunID: sourceRun.ID,
							Attempt: 1, Epoch: 1,
						}},
						GenerationProvenance: &looppkg.GenerationIntent{
							Generation: 3, ParentGeneration: 2, Origin: looppkg.OriginReattempt,
						},
					},
				},
				NextCoordinator: &taskpkg.EnqueueSpec{
					TaskID:         loopCoordinatorTaskID(created.ID),
					RunID:          loopCoordinatorRunID(created.ID, 3),
					RunKind:        taskpkg.RunKindCoordinator,
					LoopRunID:      string(created.ID),
					IdempotencyKey: "quarantine-failed-source-g3",
				},
			},
		}
		if _, err := globalDB.CompleteCoordinatorAndEnqueueNext(
			ctx,
			completion,
			looppkg.NewStoreFinalizer(),
		); err != nil {
			t.Fatalf("CompleteCoordinatorAndEnqueueNext(quarantine failed source) error = %v", err)
		}
		source, err := globalDB.GetTask(ctx, sourceTaskID)
		if err != nil {
			t.Fatalf("GetTask(generation-2 source) error = %v", err)
		}
		if source.Status != taskpkg.TaskStatusReady || source.NeedsAttention != nil {
			t.Fatalf("generation-2 source = %#v, want retryable historical source", source)
		}
		continuation, err := globalDB.GetTask(ctx, continuationTaskID)
		if err != nil {
			t.Fatalf("GetTask(generation-3 continuation) error = %v", err)
		}
		if continuation.Status != taskpkg.TaskStatusNeedsAttention || continuation.NeedsAttention == nil {
			t.Fatalf("generation-3 continuation = %#v, want quarantined needs-attention", continuation)
		}
		replayClaim := claimCoordinatorRunForTest(
			ctx,
			t,
			globalDB,
			created.ID,
			"quarantine-failed-source-replay",
			now.Add(5*time.Second),
		)
		if _, err := globalDB.CompleteCoordinatorAndEnqueueNext(
			ctx,
			taskpkg.CoordinatorCompletion{
				RunID: replayClaim.Run.ID, ClaimToken: replayClaim.ClaimToken,
				Actor: coordinatorActorContextForTest(), Now: now.Add(6 * time.Second),
				Plan: taskpkg.CoordinatorCompletionPlan{
					Yield: true,
					Snapshot: taskpkg.GenerationSnapshot{
						LoopRunID: string(created.ID), Generation: 3,
						Payload: looppkg.GenerationSnapshotPayload{Outputs: []looppkg.GenerationOutput{{
							Generation: 3, NodeID: "review", Status: "quarantined", TaskRunID: sourceRun.ID,
							Attempt: 1, Epoch: 1,
						}}},
					},
				},
			},
			looppkg.NewStoreFinalizer(),
		); err != nil {
			t.Fatalf("CompleteCoordinatorAndEnqueueNext(quarantine replay) error = %v", err)
		}
		replayed, err := globalDB.GetTask(ctx, continuationTaskID)
		if err != nil {
			t.Fatalf("GetTask(generation-3 continuation after replay) error = %v", err)
		}
		if replayed.Status != taskpkg.TaskStatusNeedsAttention || replayed.NeedsAttention == nil {
			t.Fatalf("generation-3 continuation after replay = %#v, want idempotent needs-attention", replayed)
		}
	})

	t.Run("Should requeue every fan-out continuation once without rewriting completed cells", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 21, 11, 0, 0, 0, time.UTC)
		created, err := globalDB.CreateLoopRunForStart(
			ctx,
			testLoopRun("looprun-quarantine-fanout-continuations", now, looppkg.StatusRunning),
			dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		if err := insertLoopGenerationWithExecutor(ctx, globalDB.db, created.ID, looppkg.GenerationIntent{
			Generation: 2, ParentGeneration: 1, Origin: looppkg.OriginReattempt,
		}, now.Add(time.Second)); err != nil {
			t.Fatalf("insert generation-2 provenance error = %v", err)
		}
		itemIndexes := []int{1, 3}
		currentOutputs := make([]looppkg.GenerationOutput, 0, len(itemIndexes))
		continuationOutputs := make([]looppkg.GenerationOutput, 0, len(itemIndexes))
		attempts := make([]looppkg.NodeAttempt, 0, len(itemIndexes))
		for _, itemIndex := range itemIndexes {
			completedTaskID := looppkg.NodeCellTaskID(created.ID, 2, "primary", itemIndex)
			completedTask := workspaceTaskRecordForTest(completedTaskID, string(created.WorkspaceID))
			completedTask.ParentTaskID = loopCoordinatorTaskID(created.ID)
			completedTask.Status = taskpkg.TaskStatusCompleted
			completedTask.ClosedAt = now.Add(time.Second)
			if err := globalDB.CreateTask(ctx, completedTask); err != nil {
				t.Fatalf("CreateTask(completed generation-2 cell %d) error = %v", itemIndex, err)
			}
			currentOutputs = append(currentOutputs, looppkg.GenerationOutput{
				Generation: 2, NodeID: "primary", ItemIndex: itemIndex,
				Status: "quarantined", Attempt: 1, Epoch: 1,
			})
			continuationOutputs = append(continuationOutputs, looppkg.GenerationOutput{
				Generation: 3, NodeID: "primary", ItemIndex: itemIndex,
				Status: "quarantined", Attempt: 1, Epoch: 1,
			})
			attempts = append(attempts, looppkg.NodeAttempt{
				LoopRunID: created.ID, Generation: 2, NodeID: "primary", ItemIndex: itemIndex,
				Attempt: 1, Disposition: looppkg.AttemptQuarantined, StartedAt: now,
			})
		}
		claim := claimCoordinatorRunForTest(
			ctx,
			t,
			globalDB,
			created.ID,
			"quarantine-fanout-continuations",
			now.Add(2*time.Second),
		)
		entry, err := json.Marshal(looppkg.QuarantineEntry{
			NodeID:   "primary",
			InputRef: "loop-run:" + string(created.ID) + ":node:primary:input",
			Episodes: []looppkg.QuarantineEpisode{{
				Generation: 2, QuarantinedAt: now.Add(3 * time.Second), Attempts: attempts,
			}},
		})
		if err != nil {
			t.Fatalf("json.Marshal(quarantine entry) error = %v", err)
		}
		nextCoordinatorID := loopCoordinatorRunID(created.ID, 3)
		completion := taskpkg.CoordinatorCompletion{
			RunID: claim.Run.ID, ClaimToken: claim.ClaimToken,
			Actor: coordinatorActorContextForTest(), Now: now.Add(3 * time.Second),
			Plan: taskpkg.CoordinatorCompletionPlan{
				Snapshot: taskpkg.GenerationSnapshot{
					LoopRunID: string(created.ID), Generation: 2,
					Payload: looppkg.GenerationSnapshotPayload{
						Outputs: currentOutputs,
						Controls: []looppkg.NodeControlMutation{{
							Kind: looppkg.NodeControlMutationQuarantine, NodeID: "primary",
							QuarantineEntry: entry, At: now.Add(3 * time.Second),
						}},
					},
				},
				PostReserveSnapshot: &taskpkg.GenerationSnapshot{
					LoopRunID: string(created.ID), Generation: 3,
					Payload: looppkg.GenerationSnapshotPayload{
						Outputs: continuationOutputs,
						GenerationProvenance: &looppkg.GenerationIntent{
							Generation: 3, ParentGeneration: 2, Origin: looppkg.OriginReattempt,
						},
					},
				},
				NextCoordinator: &taskpkg.EnqueueSpec{
					TaskID: claim.Run.TaskID, RunID: nextCoordinatorID,
					RunKind: taskpkg.RunKindCoordinator, LoopRunID: string(created.ID),
					IdempotencyKey: "quarantine-fanout-continuations-g3",
				},
			},
		}
		if _, err := globalDB.CompleteCoordinatorAndEnqueueNext(
			ctx,
			completion,
			looppkg.NewStoreFinalizer(),
		); err != nil {
			t.Fatalf("CompleteCoordinatorAndEnqueueNext(fan-out quarantine) error = %v", err)
		}
		for _, itemIndex := range itemIndexes {
			completedTaskID := looppkg.NodeCellTaskID(created.ID, 2, "primary", itemIndex)
			completed, err := globalDB.GetTask(ctx, completedTaskID)
			if err != nil {
				t.Fatalf("GetTask(completed generation-2 cell %d) error = %v", itemIndex, err)
			}
			if completed.Status != taskpkg.TaskStatusCompleted || completed.NeedsAttention != nil {
				t.Fatalf("completed generation-2 cell %d = %#v, want completed without attention", itemIndex, completed)
			}
			continuationTaskID := looppkg.NodeCellTaskID(created.ID, 3, "primary", itemIndex)
			continuation, err := globalDB.GetTask(ctx, continuationTaskID)
			if err != nil {
				t.Fatalf("GetTask(generation-3 continuation %d) error = %v", itemIndex, err)
			}
			if continuation.Status != taskpkg.TaskStatusNeedsAttention || continuation.NeedsAttention == nil {
				t.Fatalf("generation-3 continuation %d = %#v, want needs-attention", itemIndex, continuation)
			}
		}
		controls, err := globalDB.ListNodeControls(ctx, created.WorkspaceID, created.ID)
		if err != nil {
			t.Fatalf("ListNodeControls() error = %v", err)
		}
		if len(controls) != 1 || controls[0].NodeID != "primary" || !controls[0].Quarantined {
			t.Fatalf("node controls = %#v, want active primary quarantine", controls)
		}
		expectedRevision := controls[0].Revision
		result, err := globalDB.RequeueNode(ctx, looppkg.NodeRequeueMutation{
			WorkspaceID:      created.WorkspaceID,
			RunID:            created.ID,
			NodeID:           "primary",
			Reason:           "fan-out inputs repaired",
			ExpectedRevision: &expectedRevision,
			Actor:            operatorActorContextForTest("operator:fanout-repair"),
			RequestedAt:      now.Add(4 * time.Second),
		})
		if err != nil {
			t.Fatalf("RequeueNode(fan-out continuation) error = %v", err)
		}
		if result.Coordinator.ID != nextCoordinatorID || result.Coordinator.Status != taskpkg.TaskRunStatusQueued {
			t.Fatalf("requeue coordinator = %#v, want queued generation-3 coordinator", result.Coordinator)
		}
		generationTwoOutputs, err := globalDB.ListGenerationOutputs(ctx, created.WorkspaceID, created.ID, 2)
		if err != nil {
			t.Fatalf("ListGenerationOutputs(generation 2) error = %v", err)
		}
		generationThreeOutputs, err := globalDB.ListGenerationOutputs(ctx, created.WorkspaceID, created.ID, 3)
		if err != nil {
			t.Fatalf("ListGenerationOutputs(generation 3) error = %v", err)
		}
		if len(generationTwoOutputs) != len(itemIndexes) || len(generationThreeOutputs) != len(itemIndexes) {
			t.Fatalf(
				"generation output counts = %d/%d, want %d/%d",
				len(generationTwoOutputs), len(generationThreeOutputs), len(itemIndexes), len(itemIndexes),
			)
		}
		for index, itemIndex := range itemIndexes {
			completedTaskID := looppkg.NodeCellTaskID(created.ID, 2, "primary", itemIndex)
			completed, err := globalDB.GetTask(ctx, completedTaskID)
			if err != nil {
				t.Fatalf("GetTask(completed cell %d after requeue) error = %v", itemIndex, err)
			}
			if completed.Status != taskpkg.TaskStatusCompleted || completed.NeedsAttention != nil {
				t.Fatalf("completed cell %d after requeue = %#v, want unchanged completed", itemIndex, completed)
			}
			continuationTaskID := looppkg.NodeCellTaskID(created.ID, 3, "primary", itemIndex)
			continuation, err := globalDB.GetTask(ctx, continuationTaskID)
			if err != nil {
				t.Fatalf("GetTask(continuation %d after requeue) error = %v", itemIndex, err)
			}
			if continuation.Status != taskpkg.TaskStatusReady || continuation.NeedsAttention != nil {
				t.Fatalf("continuation %d after requeue = %#v, want released ready", itemIndex, continuation)
			}
			runID := looppkg.NodeCellAttemptRunID(created.ID, 3, "primary", itemIndex, 1)
			idempotencyKey := looppkg.NodeCellAttemptIdempotencyKey(created.ID, 3, "primary", itemIndex, 1)
			reserved, err := globalDB.GetTaskRun(ctx, runID)
			if err != nil {
				t.Fatalf("GetTaskRun(continuation %d) error = %v", itemIndex, err)
			}
			if reserved.TaskID != continuationTaskID || reserved.RunKind != taskpkg.RunKindWorker ||
				reserved.LoopRunID != string(created.ID) || reserved.Status != taskpkg.TaskRunStatusQueued ||
				reserved.IdempotencyKey != idempotencyKey {
				t.Fatalf("continuation run %d = %#v, want queued worker for exact cell", itemIndex, reserved)
			}
			var runCount int
			if err := globalDB.db.QueryRowContext(
				ctx,
				`SELECT COUNT(*) FROM task_runs WHERE task_id = ?`,
				continuationTaskID,
			).Scan(&runCount); err != nil {
				t.Fatalf("count continuation runs %d error = %v", itemIndex, err)
			}
			if runCount != 1 {
				t.Fatalf("continuation runs %d = %d, want exactly 1", itemIndex, runCount)
			}
			if generationTwoOutputs[index].ItemIndex != itemIndex ||
				generationTwoOutputs[index].Status != "quarantined" {
				t.Fatalf(
					"generation-2 output %d = %#v, want preserved quarantine",
					itemIndex,
					generationTwoOutputs[index],
				)
			}
			if generationThreeOutputs[index].ItemIndex != itemIndex ||
				generationThreeOutputs[index].Status != "enqueued" ||
				generationThreeOutputs[index].TaskRunID != runID {
				t.Fatalf(
					"generation-3 output %d = %#v, want exact enqueued run %q",
					itemIndex,
					generationThreeOutputs[index],
					runID,
				)
			}
			if got := countTaskStatusEventsForTest(t, globalDB, continuationTaskID); got != 2 {
				t.Fatalf("continuation %d status events = %d, want one park and one release", itemIndex, got)
			}
		}
		if _, err := globalDB.GetTask(
			ctx,
			looppkg.NodeCellTaskID(created.ID, 3, "primary", 0),
		); !errors.Is(err, taskpkg.ErrTaskNotFound) {
			t.Fatalf("GetTask(unrequested item 0) error = %v, want ErrTaskNotFound", err)
		}
	})

	t.Run("Should mark quarantined cell tasks needs-attention in the boundary transaction", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 5, 20, 0, 0, 0, time.UTC)
		seed := testLoopRun("looprun-quarantine-park", now, looppkg.StatusRunning)
		created, err := globalDB.CreateLoopRunForStart(ctx, seed, dsl.ConcurrencyAllow)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		claim := claimCoordinatorRunForTest(
			ctx,
			t,
			globalDB,
			created.ID,
			"seed-quarantine-park",
			now.Add(time.Millisecond),
		)
		entry, err := json.Marshal(looppkg.QuarantineEntry{
			NodeID:   "execute",
			InputRef: "loop-run:" + string(created.ID) + ":node:execute:input",
			Episodes: []looppkg.QuarantineEpisode{{
				Generation: 1, QuarantinedAt: now,
				Attempts: []looppkg.NodeAttempt{{
					LoopRunID: created.ID, Generation: 1, NodeID: "execute", Attempt: 1,
					Disposition: looppkg.AttemptQuarantined, StartedAt: now,
				}},
			}},
		})
		if err != nil {
			t.Fatalf("json.Marshal(quarantine entry) error = %v", err)
		}
		cellTaskID := looppkg.NodeCellTaskID(created.ID, 1, "execute", 0)
		completion := taskpkg.CoordinatorCompletion{
			RunID: claim.Run.ID, ClaimToken: claim.ClaimToken,
			Actor: coordinatorActorContextForTest(), Now: now.Add(time.Second),
			Plan: taskpkg.CoordinatorCompletionPlan{
				Yield: true,
				NodeTasks: []taskpkg.CoordinatorTaskSpec{{
					TaskID: cellTaskID, Title: "Loop quarantine-park node execute",
				}},
				Snapshot: taskpkg.GenerationSnapshot{
					LoopRunID: string(created.ID), Generation: 1,
					Payload: looppkg.GenerationSnapshotPayload{
						Outputs: []looppkg.GenerationOutput{{
							Generation: 1, NodeID: "execute", Status: "quarantined", Attempt: 1, Epoch: 1,
						}},
						Controls: []looppkg.NodeControlMutation{{
							Kind: looppkg.NodeControlMutationQuarantine, NodeID: "execute",
							QuarantineEntry: entry, At: now.Add(time.Second),
						}},
					},
				},
			},
		}
		if _, err := globalDB.db.ExecContext(ctx, `CREATE TEMP TRIGGER fail_quarantine_task_status_event
			BEFORE INSERT ON task_events
			WHEN NEW.event_type = 'task.status_changed'
			BEGIN SELECT RAISE(ABORT, 'forced quarantine task status event failure'); END`); err != nil {
			t.Fatalf("create task status event failure trigger error = %v", err)
		}
		if _, err := globalDB.CompleteCoordinatorAndEnqueueNext(
			ctx,
			completion,
			looppkg.NewStoreFinalizer(),
		); err == nil || !strings.Contains(err.Error(), "forced quarantine task status event failure") {
			t.Fatalf("CompleteCoordinatorAndEnqueueNext(forced event failure) error = %v", err)
		}
		if _, err := globalDB.GetTask(ctx, cellTaskID); !errors.Is(err, taskpkg.ErrTaskNotFound) {
			t.Fatalf("GetTask(after rolled back quarantine) error = %v, want ErrTaskNotFound", err)
		}
		if got := countTaskStatusEventsForTest(t, globalDB, cellTaskID); got != 0 {
			t.Fatalf("quarantine task status events after rollback = %d, want 0", got)
		}
		var controlCount int
		if err := globalDB.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM loop_node_controls
			WHERE loop_run_id = ? AND node_id = 'execute'`, created.ID).Scan(&controlCount); err != nil {
			t.Fatalf("count node controls after rolled back quarantine error = %v", err)
		}
		if controlCount != 0 {
			t.Fatalf("node controls after rolled back quarantine = %d, want 0", controlCount)
		}
		if _, err := globalDB.db.ExecContext(ctx, `DROP TRIGGER fail_quarantine_task_status_event`); err != nil {
			t.Fatalf("drop task status event failure trigger error = %v", err)
		}
		if _, err := globalDB.CompleteCoordinatorAndEnqueueNext(
			ctx,
			completion,
			looppkg.NewStoreFinalizer(),
		); err != nil {
			t.Fatalf("CompleteCoordinatorAndEnqueueNext(quarantine) error = %v", err)
		}
		parked, err := globalDB.GetTask(ctx, cellTaskID)
		if err != nil {
			t.Fatalf("GetTask(parked cell) error = %v", err)
		}
		if parked.Status != taskpkg.TaskStatusNeedsAttention || parked.NeedsAttention == nil ||
			parked.NeedsAttention.At.IsZero() ||
			!strings.Contains(parked.NeedsAttention.Reason, "quarantined") {
			t.Fatalf(
				"cell task status/needs_attention = %q/%#v, want quarantine park with reason",
				parked.Status,
				parked.NeedsAttention,
			)
		}
		statusPayload := singleTaskStatusEventPayloadForTest(t, globalDB, cellTaskID)
		wantReason := "loop node execute is quarantined; requeue it from the run to resume"
		if statusPayload.FromStatus != string(taskpkg.TaskStatusReady) ||
			statusPayload.ToStatus != string(taskpkg.TaskStatusNeedsAttention) ||
			statusPayload.Reason != wantReason || statusPayload.LoopRunID != string(created.ID) ||
			statusPayload.ActorKind != string(taskpkg.ActorKindDaemon) || statusPayload.ActorID != "loop" {
			t.Fatalf("quarantine task status event = %#v, want audited park", statusPayload)
		}
		if _, err := globalDB.CompleteCoordinatorAndEnqueueNext(
			ctx,
			completion,
			looppkg.NewStoreFinalizer(),
		); !errors.Is(err, taskpkg.ErrInvalidStatusTransition) {
			t.Fatalf(
				"CompleteCoordinatorAndEnqueueNext(quarantine replay) error = %v, want ErrInvalidStatusTransition",
				err,
			)
		}
		if got := countTaskStatusEventsForTest(t, globalDB, cellTaskID); got != 1 {
			t.Fatalf("quarantine task status events after replay = %d, want 1", got)
		}
	})
}

// Invariant: an effect acknowledgement changes one pending row and appends at most one event per
// delivery key in the same transaction. The loop store suite owns this durable idempotency rule.
func TestGlobalDBLoopEffectAcknowledgementShouldBeAtomicAndIdempotent(t *testing.T) {
	t.Parallel()

	globalDB := openLoopTestGlobalDB(t)
	ctx := testutil.Context(t)
	now := time.Date(2026, time.August, 2, 23, 45, 0, 0, time.UTC)
	loopRun, err := globalDB.CreateLoopRunForStart(
		ctx,
		testLoopRun("looprun-effect-ack", now, looppkg.StatusRunning),
		dsl.ConcurrencyAllow,
	)
	if err != nil {
		t.Fatalf("CreateLoopRunForStart() error = %v", err)
	}
	err = globalDB.withTaskImmediateTransaction(
		ctx,
		"seed loop effect acknowledgement",
		func(exec taskSQLExecutor) error {
			sourceEventID, _, appendErr := appendLoopRunEventWithIdentity(
				ctx,
				exec,
				loopRun.ID,
				loopRun.WorkspaceID,
				loopRunEventNodeFailed,
				map[string]any{"node_id": "fetch", "generation": 1, "item_index": 0},
				now,
			)
			if appendErr != nil {
				return appendErr
			}
			return insertLoopEffectIntentsWithExecutor(
				ctx,
				exec,
				loopRun,
				sourceEventID,
				[]looppkg.RenderedEffectIntent{{
					Trigger: looppkg.EffectTriggerOnError, Generation: 1, NodeID: "fetch", EntryIndex: 0,
					Entry: json.RawMessage(`{"kind":"emit","emit":{"kind":"fetch_failed","payload":{"safe":true}}}`),
				}},
				now,
			)
		},
	)
	if err != nil {
		t.Fatalf("seed loop effect acknowledgement error = %v", err)
	}
	pending, err := globalDB.ListPendingLoopEffects(ctx, 10)
	if err != nil {
		t.Fatalf("ListPendingLoopEffects() error = %v", err)
	}
	if len(pending) != 1 || pending[0].WorkspaceID != loopRun.WorkspaceID {
		t.Fatalf("pending effects = %#v, want one workspace-owned row", pending)
	}
	ack := looppkg.EffectAcknowledgement{
		Entry: pending[0], Outcome: looppkg.EffectResultOK, Duration: 25 * time.Millisecond,
		CustomEvent: json.RawMessage(`{"authored_kind":"fetch_failed","payload":{"safe":true}}`),
		At:          now.Add(time.Second),
	}
	acknowledged, err := globalDB.AcknowledgeLoopEffect(ctx, ack)
	if err != nil {
		t.Fatalf("AcknowledgeLoopEffect(first) error = %v", err)
	}
	if !acknowledged {
		t.Fatal("AcknowledgeLoopEffect(first) = false, want winner")
	}
	acknowledged, err = globalDB.AcknowledgeLoopEffect(ctx, ack)
	if err != nil {
		t.Fatalf("AcknowledgeLoopEffect(replay) error = %v", err)
	}
	if acknowledged {
		t.Fatal("AcknowledgeLoopEffect(replay) = true, want idempotent loser")
	}
	outbox, err := globalDB.ListEffectOutbox(ctx, loopRun.WorkspaceID, loopRun.ID)
	if err != nil {
		t.Fatalf("ListEffectOutbox() error = %v", err)
	}
	if len(outbox) != 1 || outbox[0].State != looppkg.EffectDelivered || outbox[0].Attempts != 1 {
		t.Fatalf("effect outbox = %#v, want delivered once", outbox)
	}
	events, err := globalDB.ListLoopRunEvents(ctx, looppkg.RunEventQuery{
		WorkspaceID: loopRun.WorkspaceID,
		RunID:       loopRun.ID,
	})
	if err != nil {
		t.Fatalf("ListLoopRunEvents() error = %v", err)
	}
	wantKeys := map[string]int{
		pending[0].DeliveryID + ":" + loopRunEventCustomEvent:   0,
		pending[0].DeliveryID + ":" + loopRunEventEffectResults: 0,
	}
	for _, event := range events {
		if _, ok := wantKeys[event.DeliveryKey]; ok {
			wantKeys[event.DeliveryKey]++
		}
	}
	for key, count := range wantKeys {
		if count != 1 {
			t.Fatalf("delivery key %q count = %d, want exactly 1", key, count)
		}
	}
}

func TestValidateLoopCoordinatorReactivation(t *testing.T) {
	t.Parallel()

	t.Run("Should reject approval with more than one gate decision", func(t *testing.T) {
		t.Parallel()

		actor, err := taskpkg.DeriveHumanActorContext(
			"operator",
			taskpkg.OriginKindCLI,
			"loop approve",
		)
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		run := looppkg.Run{
			ID:           "looprun-approval",
			WorkspaceID:  "ws-1",
			Status:       looppkg.StatusNeedsApproval,
			ActiveGateID: "human",
		}
		decision := looppkg.GateDecisionRecord{
			RunID:       run.ID,
			WorkspaceID: run.WorkspaceID,
		}

		err = validateLoopCoordinatorReactivation(&looppkg.CoordinatorReactivationRequest{
			Run:           run,
			Cause:         looppkg.TransitionCauseApproval,
			Actor:         actor,
			Decisions:     []looppkg.GateDecisionRecord{decision, decision},
			ReactivatedAt: time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC),
		})
		if !errors.Is(err, looppkg.ErrValidation) {
			t.Fatalf("validateLoopCoordinatorReactivation() error = %v, want ErrValidation", err)
		}
		if !strings.Contains(err.Error(), "one decision") {
			t.Fatalf("validateLoopCoordinatorReactivation() error = %v, want decision count detail", err)
		}
	})
}

func TestGlobalDBLoopConfigShouldPersistOverrides(t *testing.T) {
	t.Parallel()

	t.Run("Should round trip loop config by workspace and loop name", func(t *testing.T) {
		t.Parallel()

		// Invariant: every loop config field round-trips within its workspace and loop key.
		// Owning layer: GlobalDB loop config repository. Canonical suite: global_db_loop_test.go.
		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		humanGate := true
		reattempt := looppkg.ReattemptFullBody
		onExceeded := dsl.BudgetExceededEscalate
		workerModel := "stored-worker"
		judgeModel := "stored-judge"
		environment := dsl.EnvironmentSpec{Mode: dsl.EnvironmentPerRun}

		err := globalDB.UpsertLoopConfig(ctx, "ws-1", "delivery", looppkg.LoopConfig{
			HumanGateEnabled:  &humanGate,
			ReattemptStrategy: &reattempt,
			EnabledChecks:     []byte(`{"command":true}`),
			IterationCap:      new(11),
			BudgetTokens:      new(2000),
			BudgetWallSec:     new(300),
			BudgetOnExceeded:  &onExceeded,
			NoProgressWindow:  new(4),
			FanOutWidth:       new(5),
			GateMaxRevisions:  new(6),
			RuntimeDefaults: &looppkg.RuntimeDefaults{
				Worker: looppkg.RuntimeSpec{Model: workerModel},
				Judge:  looppkg.RuntimeSpec{Model: judgeModel},
			},
			Environment: &environment,
		})
		if err != nil {
			t.Fatalf("UpsertLoopConfig() error = %v", err)
		}

		got, err := globalDB.GetLoopConfig(ctx, "ws-1", "delivery")
		if err != nil {
			t.Fatalf("GetLoopConfig() error = %v", err)
		}
		if got.HumanGateEnabled == nil || !*got.HumanGateEnabled {
			t.Fatalf("HumanGateEnabled = %#v, want true", got.HumanGateEnabled)
		}
		if got.ReattemptStrategy == nil || *got.ReattemptStrategy != looppkg.ReattemptFullBody {
			t.Fatalf("ReattemptStrategy = %#v, want full_body", got.ReattemptStrategy)
		}
		if string(got.EnabledChecks) != `{"command":true}` {
			t.Fatalf("EnabledChecks = %s, want command check", got.EnabledChecks)
		}
		if got.FanOutWidth == nil || *got.FanOutWidth != 5 {
			t.Fatalf("FanOutWidth = %#v, want 5", got.FanOutWidth)
		}
		if got.RuntimeDefaults == nil {
			t.Fatal("RuntimeDefaults = nil, want stored defaults")
		}
		if got.RuntimeDefaults.Worker.Model != "stored-worker" {
			t.Fatalf("RuntimeDefaults.Worker.Model = %#v, want stored-worker", got.RuntimeDefaults.Worker)
		}
		if got.RuntimeDefaults.Judge.Model != "stored-judge" {
			t.Fatalf("RuntimeDefaults.Judge.Model = %#v, want stored-judge", got.RuntimeDefaults.Judge)
		}
		if got.Environment == nil || got.Environment.Mode != dsl.EnvironmentPerRun {
			t.Fatalf("Environment = %#v, want per_run", got.Environment)
		}
		_, err = globalDB.GetLoopConfig(ctx, "ws-2", "delivery")
		if !errors.Is(err, looppkg.ErrConfigNotFound) {
			t.Fatalf("GetLoopConfig(other workspace) error = %v, want ErrConfigNotFound", err)
		}
	})

	t.Run("Should reject empty loop config keys", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		cases := []struct {
			name     string
			ws       looppkg.WorkspaceID
			loopName string
			want     string
		}{
			{name: "Should reject empty workspace", ws: " ", loopName: "delivery", want: "workspace_id is required"},
			{name: "Should reject empty loop name", ws: "ws-1", loopName: " ", want: "loop_name is required"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				err := globalDB.UpsertLoopConfig(ctx, tc.ws, tc.loopName, looppkg.LoopConfig{})
				if !errors.Is(err, looppkg.ErrValidation) {
					t.Fatalf("UpsertLoopConfig() error = %v, want ErrValidation", err)
				}
				if !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("UpsertLoopConfig() error = %v, want %q", err, tc.want)
				}
			})
		}
	})

	t.Run("Should preserve omitted overrides on partial update", func(t *testing.T) {
		t.Parallel()

		// Invariant: a partial loop config update changes only fields present in the patch.
		// Owning layer: GlobalDB loop config repository. Canonical suite: global_db_loop_test.go.
		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		humanGate := true
		reattempt := looppkg.ReattemptFullBody
		workerModel := "stored-worker"
		environment := dsl.EnvironmentSpec{Mode: dsl.EnvironmentPerRun}
		if err := globalDB.UpsertLoopConfig(ctx, "ws-1", "delivery", looppkg.LoopConfig{
			HumanGateEnabled:  &humanGate,
			ReattemptStrategy: &reattempt,
			EnabledChecks:     []byte(`{"command":true}`),
			BudgetTokens:      new(2000),
			FanOutWidth:       new(5),
			RuntimeDefaults: &looppkg.RuntimeDefaults{
				Worker: looppkg.RuntimeSpec{Model: workerModel},
			},
			Environment: &environment,
		}); err != nil {
			t.Fatalf("UpsertLoopConfig(initial) error = %v", err)
		}
		if err := globalDB.UpsertLoopConfig(ctx, "ws-1", "delivery", looppkg.LoopConfig{
			BudgetTokens: new(5000),
		}); err != nil {
			t.Fatalf("UpsertLoopConfig(partial) error = %v", err)
		}

		got, err := globalDB.GetLoopConfig(ctx, "ws-1", "delivery")
		if err != nil {
			t.Fatalf("GetLoopConfig() error = %v", err)
		}
		if got.HumanGateEnabled == nil || !*got.HumanGateEnabled {
			t.Fatalf("HumanGateEnabled = %#v, want preserved true", got.HumanGateEnabled)
		}
		if got.ReattemptStrategy == nil || *got.ReattemptStrategy != looppkg.ReattemptFullBody {
			t.Fatalf("ReattemptStrategy = %#v, want preserved full_body", got.ReattemptStrategy)
		}
		if string(got.EnabledChecks) != `{"command":true}` {
			t.Fatalf("EnabledChecks = %s, want preserved command check", got.EnabledChecks)
		}
		if got.FanOutWidth == nil || *got.FanOutWidth != 5 {
			t.Fatalf("FanOutWidth = %#v, want preserved 5", got.FanOutWidth)
		}
		if got.BudgetTokens == nil || *got.BudgetTokens != 5000 {
			t.Fatalf("BudgetTokens = %#v, want updated 5000", got.BudgetTokens)
		}
		if got.RuntimeDefaults == nil || got.RuntimeDefaults.Worker.Model != "stored-worker" {
			t.Fatalf("RuntimeDefaults.Worker = %#v, want preserved stored-worker", got.RuntimeDefaults)
		}
		if got.Environment == nil || *got.Environment != environment {
			t.Fatalf("Environment = %#v, want preserved %#v", got.Environment, environment)
		}
	})
}

func TestGlobalDBLoopRunCreateShouldSeedInitialCoordinator(t *testing.T) {
	t.Parallel()

	t.Run("Should create a workspace-scoped coordinator for a running loop", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 7, 9, 0, 0, 0, time.UTC)
		run := testLoopRun("looprun-seed", now, looppkg.StatusRunning)
		run.GoalContextNudgeRatio = 0.37
		created, err := globalDB.CreateLoopRunForStart(ctx, run, dsl.ConcurrencyAllow)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		if created.Status != looppkg.StatusRunning {
			t.Fatalf("created status = %q, want running", created.Status)
		}
		persisted, err := globalDB.GetLoopRun(ctx, run.WorkspaceID, run.ID)
		if err != nil {
			t.Fatalf("GetLoopRun() error = %v", err)
		}
		if persisted.GoalContextNudgeRatio != 0.37 {
			t.Fatalf("GoalContextNudgeRatio = %v, want pinned 0.37", persisted.GoalContextNudgeRatio)
		}
		generations, err := globalDB.ListGenerations(ctx, string(run.WorkspaceID), string(run.ID))
		if err != nil {
			t.Fatalf("ListGenerations() error = %v", err)
		}
		if got, want := len(generations), 1; got != want {
			t.Fatalf("initial generations = %d, want %d", got, want)
		}
		initial := generations[0]
		if initial.Generation != 1 || initial.ParentGeneration != 0 || initial.Origin != looppkg.OriginInitial {
			t.Fatalf("initial generation = %#v, want generation 1 from initial origin", initial)
		}

		queued, err := globalDB.ListTaskRunsByStatus(ctx, []taskpkg.RunStatus{taskpkg.TaskRunStatusQueued})
		if err != nil {
			t.Fatalf("ListTaskRunsByStatus() error = %v", err)
		}
		if got, want := len(queued), 1; got != want {
			t.Fatalf("queued task runs = %d, want %d", got, want)
		}
		coordinator := queued[0]
		if coordinator.RunKind.Normalize() != taskpkg.RunKindCoordinator {
			t.Fatalf("RunKind = %q, want coordinator", coordinator.RunKind)
		}
		if got, want := coordinator.LoopRunID, string(created.ID); got != want {
			t.Fatalf("LoopRunID = %q, want %q", got, want)
		}
		if got, want := coordinator.ID, loopCoordinatorRunID(created.ID, created.Generation+1); got != want {
			t.Fatalf("coordinator run id = %q, want %q", got, want)
		}
		wantIdempotencyKey := loopCoordinatorIdempotencyKey(created.ID, created.Generation+1)
		if got, want := coordinator.IdempotencyKey, wantIdempotencyKey; got != want {
			t.Fatalf("IdempotencyKey = %q, want %q", got, want)
		}

		taskRecord, err := globalDB.GetTask(ctx, coordinator.TaskID)
		if err != nil {
			t.Fatalf("GetTask(coordinator) error = %v", err)
		}
		if got, want := taskRecord.ID, loopCoordinatorTaskID(created.ID); got != want {
			t.Fatalf("coordinator task id = %q, want %q", got, want)
		}
		if taskRecord.Scope.Normalize() != taskpkg.ScopeWorkspace {
			t.Fatalf("coordinator task scope = %q, want workspace", taskRecord.Scope)
		}
		if got, want := taskRecord.WorkspaceID, string(created.WorkspaceID); got != want {
			t.Fatalf("coordinator task workspace_id = %q, want %q", got, want)
		}
		if taskRecord.AutoEnqueueOnReady {
			t.Fatal("coordinator task AutoEnqueueOnReady = true, want false")
		}
	})

	t.Run("Should not create a coordinator for queued loop starts", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 7, 9, 5, 0, 0, time.UTC)
		first := testLoopRun("looprun-seed-running", now, looppkg.StatusRunning)
		if _, err := globalDB.CreateLoopRunForStart(ctx, first, dsl.ConcurrencyQueue); err != nil {
			t.Fatalf("CreateLoopRunForStart(first) error = %v", err)
		}
		second := testLoopRun("looprun-seed-queued", now.Add(time.Second), looppkg.StatusRunning)
		queuedRun, err := globalDB.CreateLoopRunForStart(ctx, second, dsl.ConcurrencyQueue)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart(second) error = %v", err)
		}
		if queuedRun.Status != looppkg.StatusQueued {
			t.Fatalf("second status = %q, want queued", queuedRun.Status)
		}
		if got := countCoordinatorTaskRunsForLoop(ctx, t, globalDB, queuedRun.ID); got != 0 {
			t.Fatalf("queued loop coordinator task runs = %d, want 0", got)
		}
	})

	t.Run("Should reject a nonzero creation cursor", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		run := testLoopRun(
			"looprun-nonzero-create-cursor",
			time.Date(2026, 7, 7, 9, 7, 0, 0, time.UTC),
			looppkg.StatusRunning,
		)
		run.Generation = 1
		_, err := globalDB.CreateLoopRunForStart(testutil.Context(t), run, dsl.ConcurrencyAllow)
		if !errors.Is(err, looppkg.ErrValidation) || !strings.Contains(err.Error(), "cursor must be zero") {
			t.Fatalf("CreateLoopRunForStart() error = %v, want zero-cursor validation", err)
		}
	})

	t.Run("Should reserve coordinator wakes beyond the task retry budget", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 7, 9, 10, 0, 0, time.UTC)
		created, err := globalDB.CreateLoopRunForStart(
			ctx,
			testLoopRun("looprun-coordinator-budget", now, looppkg.StatusRunning),
			dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		actor := coordinatorActorContextForTest()
		claimCoordinator := func(runID string, at time.Time) taskpkg.ClaimResult {
			t.Helper()
			claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
				RunID:            runID,
				Scope:            taskpkg.ScopeWorkspace,
				WorkspaceID:      string(created.WorkspaceID),
				RunKind:          taskpkg.RunKindCoordinator,
				ClaimerSessionID: "daemon-loop-coordinator-budget",
				ClaimedBy:        &taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "loop"},
				LeaseDuration:    time.Minute,
				Now:              at,
			})
			if err != nil {
				t.Fatalf("ClaimNextRun(%s) error = %v", runID, err)
			}
			return claim
		}
		completeCoordinator := func(claim taskpkg.ClaimResult, at time.Time) {
			t.Helper()
			_, err := globalDB.CompleteCoordinatorAndEnqueueNext(ctx, taskpkg.CoordinatorCompletion{
				RunID:      claim.Run.ID,
				ClaimToken: claim.ClaimToken,
				Actor:      actor,
				Plan: taskpkg.CoordinatorCompletionPlan{
					Snapshot: taskpkg.GenerationSnapshot{
						LoopRunID:  string(created.ID),
						Generation: created.Generation + 1,
					},
					Yield: true,
				},
				Now: at,
			}, looppkg.NewStoreFinalizer())
			if err != nil {
				t.Fatalf("CompleteCoordinatorAndEnqueueNext(%s) error = %v", claim.Run.ID, err)
			}
		}

		initialRunID := loopCoordinatorRunID(created.ID, created.Generation+1)
		completeCoordinator(claimCoordinator(initialRunID, now.Add(time.Second)), now.Add(2*time.Second))
		for attempt := 2; attempt <= taskpkg.DefaultTaskMaxAttempts; attempt++ {
			wakeRun, added, err := globalDB.EnqueueLoopCoordinatorWake(
				ctx,
				string(created.ID),
				fmt.Sprintf("coordinator-budget-wake-%d", attempt),
				actor.Origin,
				now.Add(time.Duration(attempt*2+1)*time.Second),
			)
			if err != nil {
				t.Fatalf("EnqueueLoopCoordinatorWake(%d) error = %v", attempt, err)
			}
			if !added {
				t.Fatalf("EnqueueLoopCoordinatorWake(%d) added = false, want true", attempt)
			}
			completeCoordinator(
				claimCoordinator(wakeRun.ID, now.Add(time.Duration(attempt*2+2)*time.Second)),
				now.Add(time.Duration(attempt*2+3)*time.Second),
			)
		}

		afterBudget, added, err := globalDB.EnqueueLoopCoordinatorWake(
			ctx,
			string(created.ID),
			"coordinator-budget-after-default",
			actor.Origin,
			now.Add(20*time.Second),
		)
		if err != nil {
			t.Fatalf("EnqueueLoopCoordinatorWake(after budget) error = %v", err)
		}
		if !added {
			t.Fatal("EnqueueLoopCoordinatorWake(after budget) added = false, want true")
		}
		if got, want := afterBudget.Attempt, int32(taskpkg.DefaultTaskMaxAttempts+1); got != want {
			t.Fatalf("after-budget attempt = %d, want %d", got, want)
		}
	})
}

func TestGlobalDBLoopHistoryShouldPersistMachineFacts(t *testing.T) {
	t.Parallel()

	t.Run("Should import immutable history and refuse to delete runtime state", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-1")
		ctx := testutil.Context(t)
		now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
		live := testLoopRun("looprun-live-delete-guard", now, looppkg.StatusRunning)
		if _, err := globalDB.CreateLoopRunForStart(ctx, live, dsl.ConcurrencyAllow); err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		if err := globalDB.DeleteRunHistory(ctx, live.WorkspaceID, live.ID); !errors.Is(
			err,
			looppkg.ErrInvalidTransition,
		) {
			t.Fatalf("DeleteRunHistory(live) error = %v, want ErrInvalidTransition", err)
		}
		if _, err := globalDB.GetLoopRun(ctx, live.WorkspaceID, live.ID); err != nil {
			t.Fatalf("GetLoopRun(live after guarded delete) error = %v", err)
		}

		historical := testLoopRun(
			"looprun-imported-history",
			now.Add(time.Minute),
			looppkg.StatusFailed,
		)
		historical.Generation = 1
		actor, err := taskpkg.DeriveHumanActorContext(
			"operator",
			taskpkg.OriginKindCLI,
			"seed history",
		)
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		command, err := looppkg.NewRunHistoryImport(&looppkg.RunHistorySnapshot{
			Run: historical,
			Generations: []looppkg.RunHistoryGeneration{{
				Intent:    looppkg.GenerationIntent{Generation: 1, Origin: looppkg.OriginInitial},
				CreatedAt: historical.CreatedAt,
			}},
			Actor: actor,
		})
		if err != nil {
			t.Fatalf("NewRunHistoryImport() error = %v", err)
		}
		if err := globalDB.ImportRunHistory(ctx, &command); err != nil {
			t.Fatalf("ImportRunHistory() error = %v", err)
		}
		persisted, err := globalDB.GetLoopRun(ctx, historical.WorkspaceID, historical.ID)
		if err != nil {
			t.Fatalf("GetLoopRun(historical) error = %v", err)
		}
		if !persisted.Historical || persisted.Status != looppkg.StatusFailed {
			t.Fatalf("GetLoopRun(historical) = %#v, want imported failed history", persisted)
		}
		liveOnly := true
		runs, err := globalDB.ListLoopRuns(ctx, looppkg.RunListQuery{
			WorkspaceID: historical.WorkspaceID,
			Live:        &liveOnly,
			Limit:       20,
		})
		if err != nil {
			t.Fatalf("ListLoopRuns(live) error = %v", err)
		}
		if len(runs) != 1 || runs[0].ID != live.ID {
			t.Fatalf("ListLoopRuns(live) = %#v, want only runtime run %q", runs, live.ID)
		}
		if err := globalDB.DeleteRunHistory(ctx, historical.WorkspaceID, historical.ID); err != nil {
			t.Fatalf("DeleteRunHistory(historical) error = %v", err)
		}
		var count int
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM loop_runs WHERE id = ?`,
			historical.ID,
		).Scan(&count); err != nil {
			t.Fatalf("count deleted history error = %v", err)
		}
		if count != 0 {
			t.Fatalf("deleted historical runs = %d, want 0", count)
		}
	})

	t.Run("Should preserve parent lineage inside one workspace", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-1", "ws-2")
		ctx := testutil.Context(t)
		now := time.Date(2026, 8, 22, 13, 0, 0, 0, time.UTC)
		actor, err := taskpkg.DeriveHumanActorContext("operator", taskpkg.OriginKindCLI, "seed history")
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		parent := testLoopRun("looprun-history-parent", now, looppkg.StatusDone)
		if _, err := globalDB.CreateLoopRunForStart(ctx, parent, dsl.ConcurrencyAllow); err != nil {
			t.Fatalf("CreateLoopRunForStart(parent) error = %v", err)
		}
		foreignParent := testLoopRun("looprun-history-foreign-parent", now, looppkg.StatusDone)
		foreignParent.WorkspaceID = "ws-2"
		if _, err := globalDB.CreateLoopRunForStart(ctx, foreignParent, dsl.ConcurrencyAllow); err != nil {
			t.Fatalf("CreateLoopRunForStart(foreign parent) error = %v", err)
		}
		newCommand := func(id string, parentID looppkg.RunID) looppkg.RunHistoryImport {
			t.Helper()
			run := testLoopRun(id, now.Add(time.Minute), looppkg.StatusFailed)
			run.Generation = 1
			run.ParentLoopRunID = parentID
			command, commandErr := looppkg.NewRunHistoryImport(&looppkg.RunHistorySnapshot{
				Run: run,
				Generations: []looppkg.RunHistoryGeneration{{
					Intent: looppkg.GenerationIntent{
						Generation: 1,
						Origin:     looppkg.OriginInitial,
					},
					CreatedAt: run.CreatedAt,
				}},
				Actor: actor,
			})
			if commandErr != nil {
				t.Fatalf("NewRunHistoryImport(%q) error = %v", id, commandErr)
			}
			return command
		}

		missing := newCommand("looprun-history-missing-parent", "looprun-missing")
		if err := globalDB.ImportRunHistory(ctx, &missing); !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("ImportRunHistory(missing parent) error = %v, want sql.ErrNoRows", err)
		}
		foreign := newCommand("looprun-history-foreign-child", foreignParent.ID)
		if err := globalDB.ImportRunHistory(ctx, &foreign); !errors.Is(err, looppkg.ErrValidation) {
			t.Fatalf("ImportRunHistory(foreign parent) error = %v, want ErrValidation", err)
		}
		valid := newCommand("looprun-history-valid-child", parent.ID)
		if err := globalDB.ImportRunHistory(ctx, &valid); err != nil {
			t.Fatalf("ImportRunHistory(valid parent) error = %v", err)
		}
	})

	t.Run("Should scope deterministic generation and verdict history to its workspace", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-1", "ws-2")
		ctx := testutil.Context(t)
		now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
		run := testLoopRun("looprun-history", now, looppkg.StatusRunning)
		created, err := globalDB.CreateLoopRunForStart(ctx, run, dsl.ConcurrencyAllow)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		if err := insertLoopGenerationWithExecutor(ctx, globalDB.db, created.ID, looppkg.GenerationIntent{
			Generation: 2, ParentGeneration: 1, Origin: looppkg.OriginReattempt,
		}, now.Add(time.Minute)); err != nil {
			t.Fatalf("insertLoopGenerationWithExecutor() error = %v", err)
		}
		for generation, outputRef := range map[int]string{
			1: `{"draft":"initial"}`,
			2: `{"draft":"revised"}`,
		} {
			if err := looppkg.NewStoreFinalizer().WriteGenerationSnapshot(
				ctx,
				globalDB.db,
				taskpkg.GenerationSnapshot{
					LoopRunID: string(created.ID), Generation: generation,
					Payload: looppkg.GenerationSnapshotPayload{Outputs: []looppkg.GenerationOutput{{
						Generation: generation, NodeID: "draft", Status: "succeeded", OutputRef: outputRef,
					}}},
				},
			); err != nil {
				t.Fatalf("WriteGenerationSnapshot(generation %d) error = %v", generation, err)
			}
		}
		routeRankZero := 0
		routeRankOne := 1
		for _, verdict := range []struct {
			intent    gate.VerdictIntent
			decidedAt time.Time
		}{
			{
				intent: gate.VerdictIntent{
					GateID: "gate-a", Outcome: gate.VerdictOutcomeApproved, RouteCauseRank: &routeRankOne,
					BlockingIssues: json.RawMessage(`[]`), Criteria: json.RawMessage(`[]`),
				},
				decidedAt: now.Add(3 * time.Minute),
			},
			{
				intent: gate.VerdictIntent{
					GateID: "gate-b", Outcome: gate.VerdictOutcomeRejected, RouteCauseRank: &routeRankZero,
					BlockingIssues: json.RawMessage(`[]`), Criteria: json.RawMessage(`[]`),
				},
				decidedAt: now.Add(time.Minute),
			},
			{
				intent: gate.VerdictIntent{
					GateID:         "gate-c",
					Outcome:        gate.VerdictOutcomeBlocked,
					BlockingIssues: json.RawMessage(`[]`),
					Criteria:       json.RawMessage(`[]`),
				},
				decidedAt: now.Add(2 * time.Minute),
			},
		} {
			if err := insertLoopGateVerdictWithExecutor(
				ctx,
				globalDB.db,
				created.ID,
				2,
				verdict.intent,
				verdict.decidedAt,
			); err != nil {
				t.Fatalf("insertLoopGateVerdictWithExecutor(%q) error = %v", verdict.intent.GateID, err)
			}
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_runs SET generation = 2 WHERE id = ?`,
			string(created.ID),
		); err != nil {
			t.Fatalf("advance loop generation fixture error = %v", err)
		}
		bestGeneration := int64(2)
		bestScore := 0.9
		if err := updateLoopRunBestWithExecutor(
			ctx,
			globalDB.db,
			created.WorkspaceID,
			created.ID,
			&bestGeneration,
			&bestScore,
		); err != nil {
			t.Fatalf("updateLoopRunBestWithExecutor() error = %v", err)
		}

		generations, err := globalDB.ListGenerations(ctx, "ws-1", string(created.ID))
		if err != nil {
			t.Fatalf("ListGenerations() error = %v", err)
		}
		if got, want := len(generations), 2; got != want {
			t.Fatalf("generation count = %d, want %d", got, want)
		}
		if generations[0].Generation != 1 || generations[1].Generation != 2 || generations[1].ParentGeneration != 1 ||
			generations[1].Origin != looppkg.OriginReattempt {
			t.Fatalf("generations = %#v, want ordered initial and reattempt lineage", generations)
		}
		verdicts, err := globalDB.ListGateVerdicts(ctx, "ws-1", string(created.ID), 2)
		if err != nil {
			t.Fatalf("ListGateVerdicts() error = %v", err)
		}
		if got, want := len(
			verdicts,
		), 3; got != want || verdicts[0].GateID != "gate-a" || verdicts[1].GateID != "gate-b" ||
			verdicts[2].GateID != "gate-c" {
			t.Fatalf("verdicts = %#v, want gate-id/item-index order independent of decided_at", verdicts)
		}
		routeCauses, err := globalDB.ListRouteCausingVerdicts(ctx, "ws-1", string(created.ID), 2)
		if err != nil {
			t.Fatalf("ListRouteCausingVerdicts() error = %v", err)
		}
		if got, want := len(routeCauses), 2; got != want ||
			routeCauses[0].GateID != "gate-b" || routeCauses[0].RouteCauseRank == nil || *routeCauses[0].RouteCauseRank != 0 ||
			routeCauses[1].GateID != "gate-a" || routeCauses[1].RouteCauseRank == nil || *routeCauses[1].RouteCauseRank != 1 {
			t.Fatalf("route-causing verdicts = %#v, want route rank order without unranked verdicts", routeCauses)
		}
		persisted, err := globalDB.GetLoopRun(ctx, created.WorkspaceID, created.ID)
		if err != nil {
			t.Fatalf("GetLoopRun() error = %v", err)
		}
		if persisted.BestGeneration == nil || persisted.BestScore == nil ||
			*persisted.BestGeneration != bestGeneration ||
			*persisted.BestScore != bestScore {
			t.Fatalf(
				"best state = (%#v, %#v), want (%d, %v)",
				persisted.BestGeneration,
				persisted.BestScore,
				bestGeneration,
				bestScore,
			)
		}
		history, err := looppkg.ReadGenerationHistory(ctx, globalDB, persisted, 3)
		if err != nil {
			t.Fatalf("ReadGenerationHistory() error = %v", err)
		}
		if history.Previous == nil || history.Previous.Generation != 2 ||
			len(history.Previous.Verdicts) != 3 || len(history.Previous.RouteCauses) != 2 {
			t.Fatalf("previous history = %#v, want generation 2 with verdicts and route causes", history.Previous)
		}
		previousDraft, ok := history.Previous.Nodes["draft"][0].Output.(map[string]any)
		if !ok || previousDraft["draft"] != "revised" {
			t.Fatalf("previous draft output = %#v, want revised", history.Previous.Nodes["draft"][0].Output)
		}
		if history.Best == nil {
			t.Fatal("best history = nil, want generation 2")
		}
		bestNodes := history.Best.Nodes["draft"]
		if len(bestNodes) == 0 {
			t.Fatalf("best history draft outputs = %#v, want one output", bestNodes)
		}
		bestDraft, ok := bestNodes[0].Output.(map[string]any)
		if !ok {
			t.Fatalf("best draft output = %#v, want object", bestNodes[0].Output)
		}
		if history.Best.Generation != 2 || history.Best.Score != bestScore || bestDraft["draft"] != "revised" {
			t.Fatalf("best history = %#v, want generation 2 score %.1f", history.Best, bestScore)
		}
		if foreign, err := globalDB.ListGenerations(ctx, "ws-2", string(created.ID)); err != nil || len(foreign) != 0 {
			t.Fatalf("ListGenerations(other workspace) = %#v, %v; want empty, nil", foreign, err)
		}
		if foreign, err := globalDB.ListGateVerdicts(
			ctx,
			"ws-2",
			string(created.ID),
			2,
		); err != nil ||
			len(foreign) != 0 {
			t.Fatalf("ListGateVerdicts(other workspace) = %#v, %v; want empty, nil", foreign, err)
		}
		var humanDecisionCount int
		if err := globalDB.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM loop_gate_decisions WHERE loop_run_id = ?`, string(created.ID)).
			Scan(&humanDecisionCount); err != nil {
			t.Fatalf("count human decisions error = %v", err)
		}
		if humanDecisionCount != 0 {
			t.Fatalf("human decisions = %d, want machine verdicts to leave them untouched", humanDecisionCount)
		}
	})

	t.Run("Should persist route causes per generation and isolate workspaces", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-1", "ws-2")
		ctx := testutil.Context(t)
		now := time.Date(2026, 8, 16, 14, 0, 0, 0, time.UTC)
		created, err := globalDB.CreateLoopRunForStart(
			ctx,
			testLoopRun("looprun-route-causes", now, looppkg.StatusRunning),
			dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		for generation, route := range map[int]string{1: "review", 2: "fallback"} {
			event := looppkg.GenerationLifecycleEventIntent{
				Kind: looppkg.GenerationLifecycleEventRouteTaken, NodeID: "router",
				SelectedRoute: route, Reason: "default", DefaultRoute: true,
			}
			if err := appendGenerationLifecycleEventWithExecutor(
				ctx,
				globalDB.db,
				created,
				generation,
				looppkg.GenerationIntent{},
				nil,
				event,
				now.Add(time.Duration(generation)*time.Minute),
			); err != nil {
				t.Fatalf("appendGenerationLifecycleEventWithExecutor(%d) error = %v", generation, err)
			}
		}
		causes, err := globalDB.ListRouteCauses(ctx, created.WorkspaceID, created.ID, 1)
		if err != nil {
			t.Fatalf("ListRouteCauses() error = %v", err)
		}
		if len(causes) != 1 || causes[0].Generation != 1 || causes[0].NodeID != "router" ||
			causes[0].Route != "review" || !causes[0].Default {
			t.Fatalf("generation 1 route causes = %#v", causes)
		}
		second, err := globalDB.ListRouteCauses(ctx, created.WorkspaceID, created.ID, 2)
		if err != nil || len(second) != 1 || second[0].Route != "fallback" {
			t.Fatalf("generation 2 route causes = %#v, %v", second, err)
		}
		foreign, err := globalDB.ListRouteCauses(ctx, "ws-2", created.ID, 1)
		if err != nil || len(foreign) != 0 {
			t.Fatalf("foreign route causes = %#v, %v; want empty", foreign, err)
		}
	})

	t.Run("Should persist each fan-out gate item independently", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-1")
		ctx := testutil.Context(t)
		now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
		created, err := globalDB.CreateLoopRunForStart(
			ctx,
			testLoopRun("looprun-fanout-verdicts", now, looppkg.StatusRunning),
			dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		for itemIndex, outcome := range []gate.VerdictOutcome{
			gate.VerdictOutcomeRejected,
			gate.VerdictOutcomeApproved,
		} {
			intent := gate.VerdictIntent{
				GateID: "quality", ItemIndex: itemIndex, Outcome: outcome,
				BlockingIssues: json.RawMessage(`[]`), Criteria: json.RawMessage(`[]`),
			}
			if err := insertLoopGateVerdictWithExecutor(
				ctx,
				globalDB.db,
				created.ID,
				1,
				intent,
				now,
			); err != nil {
				t.Fatalf("insertLoopGateVerdictWithExecutor(item %d) error = %v", itemIndex, err)
			}
		}
		verdicts, err := globalDB.ListGateVerdicts(ctx, "ws-1", string(created.ID), 1)
		if err != nil {
			t.Fatalf("ListGateVerdicts() error = %v", err)
		}
		if len(verdicts) != 2 || verdicts[0].ItemIndex != 0 || verdicts[1].ItemIndex != 1 ||
			verdicts[0].Outcome != gate.VerdictOutcomeRejected ||
			verdicts[1].Outcome != gate.VerdictOutcomeApproved {
			t.Fatalf("fan-out verdicts = %#v, want two ordered gate instances", verdicts)
		}
	})

	t.Run("Should persist human approvals outside the machine verdict history", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 8, 1, 13, 0, 0, 0, time.UTC)
		created, err := globalDB.CreateLoopRunForStart(
			ctx,
			testLoopRun("looprun-human-decision", now, looppkg.StatusRunning),
			dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		actor, err := taskpkg.DeriveHumanActorContext("operator", taskpkg.OriginKindCLI, "loop approve")
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		if err := globalDB.RecordLoopGateDecisions(ctx, []looppkg.GateDecisionRecord{{
			WorkspaceID: created.WorkspaceID,
			RunID:       created.ID,
			Generation:  0,
			GateID:      "human-review",
			CriterionID: "operator-approval",
			Decision:    looppkg.GateDecisionApprove,
			Actor:       actor,
			DecidedAt:   now,
		}}); err != nil {
			t.Fatalf("RecordLoopGateDecisions() error = %v", err)
		}
		decisions, err := globalDB.ListLoopGateDecisions(ctx, created.WorkspaceID, created.ID, 0, "human-review")
		if err != nil {
			t.Fatalf("ListLoopGateDecisions() error = %v", err)
		}
		decision, ok := decisions["operator-approval"]
		if !ok || decision.Decision != gate.HumanDecisionApprove {
			t.Fatalf("human decisions = %#v, want stored operator approval", decisions)
		}
		var machineVerdictCount int
		if err := globalDB.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM loop_gate_verdicts WHERE loop_run_id = ?`, string(created.ID)).
			Scan(&machineVerdictCount); err != nil {
			t.Fatalf("count machine verdicts error = %v", err)
		}
		if machineVerdictCount != 0 {
			t.Fatalf("machine verdicts = %d, want human decision writer to leave them untouched", machineVerdictCount)
		}
	})
}

func TestGlobalDBLoopRetryDueShouldFenceAndPageSchedules(t *testing.T) {
	t.Parallel()

	t.Run("Should select only due live unparked cells with a stable cursor", func(t *testing.T) {
		t.Parallel()
		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 8, 2, 14, 0, 0, 0, time.UTC)
		seedLoopRetryDueCell(t, globalDB, "looprun-retry-due-a", now, now.Add(-2*time.Second), false, false)
		seedLoopRetryDueCell(t, globalDB, "looprun-retry-due-b", now, now.Add(-time.Second), false, false)
		seedLoopRetryDueCell(t, globalDB, "looprun-retry-future", now, now.Add(time.Minute), false, false)
		seedLoopRetryDueCell(t, globalDB, "looprun-retry-paused", now, now.Add(-time.Second), true, false)
		seedLoopRetryDueCell(t, globalDB, "looprun-retry-terminal", now, now.Add(-time.Second), false, true)

		first, cursor, err := globalDB.ListDueLoopRetries(ctx, now, looppkg.RetryDueCursor{}, 1)
		if err != nil {
			t.Fatalf("ListDueLoopRetries(first) error = %v", err)
		}
		if len(first) != 1 || first[0].LoopRunID != "looprun-retry-due-a" || cursor.Empty() {
			t.Fatalf("first due page = %#v cursor=%#v, want due-a and continuation", first, cursor)
		}
		second, cursor, err := globalDB.ListDueLoopRetries(ctx, now, cursor, 1)
		if err != nil {
			t.Fatalf("ListDueLoopRetries(second) error = %v", err)
		}
		if len(second) != 1 || second[0].LoopRunID != "looprun-retry-due-b" || cursor.Empty() {
			t.Fatalf("second due page = %#v cursor=%#v, want due-b and continuation", second, cursor)
		}
		last, cursor, err := globalDB.ListDueLoopRetries(ctx, now, cursor, 1)
		if err != nil {
			t.Fatalf("ListDueLoopRetries(last) error = %v", err)
		}
		if len(last) != 0 || !cursor.Empty() {
			t.Fatalf("last due page = %#v cursor=%#v, want empty reset", last, cursor)
		}
	})

	t.Run("Should converge timer and due scan on one epoch-fenced wake", func(t *testing.T) {
		t.Parallel()
		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 8, 2, 14, 30, 0, 0, time.UTC)
		cell := seedLoopRetryDueCell(
			t, globalDB, "looprun-retry-converged", now, now.Add(-time.Second), false, false,
		)
		actor := coordinatorActorContextForTest()
		timerRun, added, current, err := globalDB.EnqueueLoopRetryWakeIfCurrent(ctx, actor.Origin, cell, now)
		if err != nil {
			t.Fatalf("EnqueueLoopRetryWakeIfCurrent(timer) error = %v", err)
		}
		if !added || !current || timerRun.IdempotencyKey != looppkg.RetryWakeIdempotencyKey(cell) {
			t.Fatalf("timer wake = %#v added=%v current=%v", timerRun, added, current)
		}
		secondDueAt := cell.NextAttemptAt
		firstScheduledAt := now.Add(-time.Minute)
		if err := looppkg.NewStoreFinalizer().WriteGenerationSnapshot(
			ctx,
			globalDB.db,
			taskpkg.GenerationSnapshot{
				LoopRunID: string(cell.LoopRunID), Generation: 1,
				Payload: looppkg.GenerationSnapshotPayload{Outputs: []looppkg.GenerationOutput{{
					Generation: 1, NodeID: "second_retry", Status: "retrying", Attempt: 1, Epoch: 1,
					NextAttemptAt: &secondDueAt, FirstScheduledAt: &firstScheduledAt,
				}}},
			},
		); err != nil {
			t.Fatalf("WriteGenerationSnapshot(second due cell) error = %v", err)
		}
		runs, _, err := globalDB.EnqueueDueLoopRetryWakesPage(
			ctx, actor.Origin, now, looppkg.RetryDueCursor{}, 10,
		)
		if err != nil {
			t.Fatalf("EnqueueDueLoopRetryWakesPage() error = %v", err)
		}
		if len(runs) != 0 {
			t.Fatalf("due-scan duplicate wakes = %#v, want none", runs)
		}

		stale := cell
		stale.Epoch--
		_, added, current, err = globalDB.EnqueueLoopRetryWakeIfCurrent(ctx, actor.Origin, stale, now)
		if err != nil {
			t.Fatalf("EnqueueLoopRetryWakeIfCurrent(stale) error = %v", err)
		}
		if added || current {
			t.Fatalf("stale wake added=%v current=%v, want false/false", added, current)
		}
		var diagnostics int
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM loop_run_events WHERE loop_run_id = ? AND kind = ?`,
			cell.LoopRunID,
			loopRunEventStaleScheduleDropped,
		).Scan(&diagnostics); err != nil {
			t.Fatalf("count stale schedule diagnostics error = %v", err)
		}
		if diagnostics != 1 {
			t.Fatalf("stale schedule diagnostics = %d, want 1", diagnostics)
		}
	})

	t.Run("Should preserve a null current epoch when the output row is missing", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 8, 2, 14, 45, 0, 0, time.UTC)
		cell := seedLoopRetryDueCell(
			t, globalDB, "looprun-retry-missing-output", now, now.Add(-time.Second), false, false,
		)
		if _, err := globalDB.db.ExecContext(
			ctx,
			`DELETE FROM loop_generation_outputs
			 WHERE loop_run_id = ? AND generation = ? AND node_id = ? AND item_index = ?`,
			cell.LoopRunID,
			cell.Generation,
			cell.NodeID,
			cell.ItemIndex,
		); err != nil {
			t.Fatalf("delete retry output error = %v", err)
		}
		_, added, current, err := globalDB.EnqueueLoopRetryWakeIfCurrent(
			ctx,
			coordinatorActorContextForTest().Origin,
			cell,
			now,
		)
		if err != nil {
			t.Fatalf("EnqueueLoopRetryWakeIfCurrent(missing output) error = %v", err)
		}
		if added || current {
			t.Fatalf("missing-output wake added=%v current=%v, want false/false", added, current)
		}
		var raw string
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT payload_json FROM loop_run_events
			 WHERE loop_run_id = ? AND kind = ? ORDER BY seq DESC LIMIT 1`,
			cell.LoopRunID,
			loopRunEventStaleScheduleDropped,
		).Scan(&raw); err != nil {
			t.Fatalf("read stale retry payload error = %v", err)
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			t.Fatalf("json.Unmarshal(stale retry payload) error = %v", err)
		}
		if value, exists := payload[loopRunEventPayloadKeyCurrentEpoch]; !exists || value != nil {
			t.Fatalf("current_epoch = %#v exists=%v, want explicit null", value, exists)
		}
	})
}

func seedLoopRetryDueCell(
	t *testing.T,
	globalDB *GlobalDB,
	runID string,
	createdAt time.Time,
	nextAttemptAt time.Time,
	paused bool,
	terminal bool,
) looppkg.RetryDueCell {
	t.Helper()
	ctx := testutil.Context(t)
	created, err := globalDB.CreateLoopRunForStart(
		ctx,
		testLoopRun(runID, createdAt, looppkg.StatusRunning),
		dsl.ConcurrencyAllow,
	)
	if err != nil {
		t.Fatalf("CreateLoopRunForStart(%s) error = %v", runID, err)
	}
	firstScheduledAt := createdAt.UTC()
	claim := claimCoordinatorRunForTest(
		ctx,
		t,
		globalDB,
		created.ID,
		"run-retry-due-seed-"+runID,
		createdAt.Add(time.Millisecond),
	)
	if _, err := globalDB.CompleteCoordinatorAndEnqueueNext(
		ctx,
		taskpkg.CoordinatorCompletion{
			RunID: claim.Run.ID, ClaimToken: claim.ClaimToken,
			Actor: coordinatorActorContextForTest(), Now: createdAt.Add(2 * time.Millisecond),
			Plan: taskpkg.CoordinatorCompletionPlan{Yield: true, Snapshot: taskpkg.GenerationSnapshot{
				LoopRunID:  string(created.ID),
				Generation: 1,
				Payload: looppkg.GenerationSnapshotPayload{Outputs: []looppkg.GenerationOutput{{
					Generation: 1, NodeID: "retry_node", Status: "retrying", Attempt: 2,
					NextAttemptAt: &nextAttemptAt, FirstScheduledAt: &firstScheduledAt, Epoch: 3,
				}}},
			}},
		},
		looppkg.NewStoreFinalizer(),
	); err != nil {
		t.Fatalf("CompleteCoordinatorAndEnqueueNext(%s) error = %v", runID, err)
	}
	if paused {
		if _, err := globalDB.db.ExecContext(
			ctx,
			`INSERT INTO loop_node_controls (loop_run_id, node_id, paused, updated_at) VALUES (?, ?, 1, ?)`,
			created.ID,
			"retry_node",
			createdAt,
		); err != nil {
			t.Fatalf("insert paused retry control error = %v", err)
		}
	}
	if terminal {
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_runs SET status = 'done' WHERE id = ?`,
			created.ID,
		); err != nil {
			t.Fatalf("terminalize retry loop error = %v", err)
		}
	}
	return looppkg.RetryDueCell{
		WorkspaceID: created.WorkspaceID, LoopRunID: created.ID, Generation: 1,
		NodeID: "retry_node", ItemIndex: 0, Attempt: 2, Epoch: 3,
		NextAttemptAt: nextAttemptAt.UTC(),
	}
}

func TestGlobalDBLoopRunShouldPreserveGoalPolicyAcrossReopen(t *testing.T) {
	t.Parallel()

	t.Run("Should load the original context nudge ratio after a database restart", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), storepkg.GlobalDatabaseName)
		globalDB, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		activeDB := globalDB
		t.Cleanup(func() {
			if activeDB == nil {
				return
			}
			if closeErr := activeDB.Close(testutil.Context(t)); closeErr != nil {
				t.Errorf("Close() error = %v", closeErr)
			}
		})
		seedLoopTestWorkspaces(t, globalDB, "ws-1")

		run := testLoopRun(
			"looprun-goal-policy-reopen",
			time.Date(2026, 7, 10, 15, 0, 0, 0, time.UTC),
			looppkg.StatusRunning,
		)
		run.GoalContextNudgeRatio = 0.23
		if _, err := globalDB.CreateLoopRunForStart(ctx, run, dsl.ConcurrencyAllow); err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		if err := globalDB.Close(ctx); err != nil {
			t.Fatalf("Close(before reopen) error = %v", err)
		}
		activeDB = nil

		reopened, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(reopen) error = %v", err)
		}
		activeDB = reopened
		persisted, err := reopened.GetLoopRun(ctx, run.WorkspaceID, run.ID)
		if err != nil {
			t.Fatalf("GetLoopRun(after reopen) error = %v", err)
		}
		if persisted.GoalContextNudgeRatio != 0.23 {
			t.Fatalf("GoalContextNudgeRatio = %v, want pinned 0.23", persisted.GoalContextNudgeRatio)
		}
	})
}

func TestGlobalDBLoopRunStatusShouldUseCompareAndSwap(t *testing.T) {
	t.Parallel()

	t.Run("Should allow only one concurrent transition from the same status", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 4, 14, 0, 0, 0, time.UTC)
		run := testLoopRun("looprun-cas", now, looppkg.StatusRunning)
		created, err := globalDB.CreateLoopRunForStart(ctx, run, dsl.ConcurrencyAllow)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		if created.Status != looppkg.StatusRunning {
			t.Fatalf("CreateLoopRunForStart() status = %s, want running", created.Status)
		}
		snapshot, err := globalDB.GetLoopDefinitionSnapshot(ctx, created.WorkspaceID, created.DefinitionDigest)
		if err != nil {
			t.Fatalf("GetLoopDefinitionSnapshot() error = %v", err)
		}
		if snapshot.Digest != created.DefinitionDigest || snapshot.ByteSize != len(created.DefinitionSnapshot) {
			t.Fatalf("snapshot = %#v, want digest %q and byte size %d",
				snapshot,
				created.DefinitionDigest,
				len(created.DefinitionSnapshot),
			)
		}

		attempts := make([]error, 8)
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(len(attempts))
		for idx := range attempts {
			go func(idx int) {
				defer wg.Done()
				<-start
				attempts[idx] = globalDB.CompareAndSwapLoopRunStatus(
					context.Background(),
					run.ID,
					looppkg.StatusRunning,
					looppkg.StatusPaused,
					looppkg.TransitionCausePauseBoundary,
					now.Add(time.Duration(idx)*time.Millisecond),
				)
			}(idx)
		}
		close(start)
		wg.Wait()

		wins := 0
		conflicts := 0
		for idx, err := range attempts {
			if err == nil {
				wins++
				continue
			}
			if errors.Is(err, looppkg.ErrTransitionConflict) {
				conflicts++
				continue
			}
			t.Fatalf("attempt %d error = %v, want nil or ErrTransitionConflict", idx, err)
		}
		if wins != 1 {
			t.Fatalf("wins = %d, want 1", wins)
		}
		if conflicts != len(attempts)-1 {
			t.Fatalf("conflicts = %d, want %d", conflicts, len(attempts)-1)
		}
		stored, err := globalDB.GetLoopRun(ctx, "ws-1", run.ID)
		if err != nil {
			t.Fatalf("GetLoopRun() error = %v", err)
		}
		if stored.Status != looppkg.StatusPaused {
			t.Fatalf("stored status = %q, want paused", stored.Status)
		}
		if got, want := stored.IterationCap, run.IterationCap; got != want {
			t.Fatalf("stored iteration cap = %d, want %d", got, want)
		}
		eventCount := countLoopRunEvents(ctx, t, globalDB, run.ID)
		if eventCount != 2 {
			t.Fatalf("status event count = %d, want create + transition events", eventCount)
		}
	})

	t.Run("Should ignore same-status compare-and-swap without appending an event", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 4, 14, 5, 0, 0, time.UTC)
		run := testLoopRun("looprun-cas-noop", now, looppkg.StatusRunning)
		if _, err := globalDB.CreateLoopRunForStart(ctx, run, dsl.ConcurrencyAllow); err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		if err := globalDB.CompareAndSwapLoopRunStatus(
			ctx,
			run.ID,
			looppkg.StatusRunning,
			looppkg.StatusRunning,
			looppkg.TransitionCauseApproval,
			now.Add(time.Second),
		); err != nil {
			t.Fatalf("CompareAndSwapLoopRunStatus(no-op) error = %v", err)
		}
		if eventCount := countLoopRunEvents(ctx, t, globalDB, run.ID); eventCount != 1 {
			t.Fatalf("status event count = %d, want only create event", eventCount)
		}
	})
}

func TestGlobalDBLoopDefinitionSnapshotShouldRejectDigestCollisions(t *testing.T) {
	t.Parallel()

	t.Run("Should roll back a new Run when the workspace digest already owns different content", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
		first := testLoopRun("looprun-snapshot-first", now, looppkg.StatusRunning)
		if _, err := globalDB.CreateLoopRunForStart(ctx, first, dsl.ConcurrencyAllow); err != nil {
			t.Fatalf("CreateLoopRunForStart(first) error = %v", err)
		}
		collidingJSON := `{"format_version":1,"different":true}`
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_definition_snapshots
			 SET definition_json = ?, byte_size = ?
			 WHERE workspace_id = ? AND definition_digest = ?`,
			collidingJSON,
			len(collidingJSON),
			string(first.WorkspaceID),
			first.DefinitionDigest,
		); err != nil {
			t.Fatalf("corrupt existing snapshot fixture error = %v", err)
		}

		second := first
		second.ID = "looprun-snapshot-second"
		second.CreatedAt = now.Add(time.Minute)
		second.StartedAt = second.CreatedAt
		second.LastProgressAt = second.CreatedAt
		_, err := globalDB.CreateLoopRunForStart(ctx, second, dsl.ConcurrencyAllow)
		if !errors.Is(err, looppkg.ErrValidation) {
			t.Fatalf("CreateLoopRunForStart(collision) error = %v, want ErrValidation", err)
		}
		if _, err := globalDB.GetLoopRunByID(ctx, second.ID); !errors.Is(err, looppkg.ErrRunNotFound) {
			t.Fatalf("GetLoopRunByID(rolled back) error = %v, want ErrRunNotFound", err)
		}
	})
}

func TestGlobalDBLoopRunCreateShouldApplyConcurrencyPolicyAtomically(t *testing.T) {
	t.Parallel()

	t.Run("Should allow only one concurrent forbid start", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 4, 14, 15, 0, 0, time.UTC)
		attempts := make([]error, 8)
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(len(attempts))
		for idx := range attempts {
			go func(idx int) {
				defer wg.Done()
				<-start
				run := testLoopRun(
					"looprun-forbid-"+time.Duration(idx).String(),
					now.Add(time.Duration(idx)*time.Millisecond),
					looppkg.StatusRunning,
				)
				_, attempts[idx] = globalDB.CreateLoopRunForStart(
					context.Background(),
					run,
					dsl.ConcurrencyForbid,
				)
			}(idx)
		}
		close(start)
		wg.Wait()

		wins := 0
		conflicts := 0
		for idx, err := range attempts {
			if err == nil {
				wins++
				continue
			}
			if errors.Is(err, looppkg.ErrConcurrencyConflict) {
				conflicts++
				continue
			}
			t.Fatalf("attempt %d error = %v, want nil or ErrConcurrencyConflict", idx, err)
		}
		if wins != 1 {
			t.Fatalf("wins = %d, want 1", wins)
		}
		if conflicts != len(attempts)-1 {
			t.Fatalf("conflicts = %d, want %d", conflicts, len(attempts)-1)
		}
		running := countLoopRunsByStatus(ctx, t, globalDB, "ws-1", "delivery", looppkg.StatusRunning)
		if running != 1 {
			t.Fatalf("running loop_runs = %d, want 1", running)
		}
	})

	t.Run("Should queue concurrent queue starts after the first running run", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 4, 14, 20, 0, 0, time.UTC)
		attempts := make([]error, 8)
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(len(attempts))
		for idx := range attempts {
			go func(idx int) {
				defer wg.Done()
				<-start
				run := testLoopRun(
					"looprun-queue-"+time.Duration(idx).String(),
					now.Add(time.Duration(idx)*time.Millisecond),
					looppkg.StatusRunning,
				)
				_, attempts[idx] = globalDB.CreateLoopRunForStart(
					context.Background(),
					run,
					dsl.ConcurrencyQueue,
				)
			}(idx)
		}
		close(start)
		wg.Wait()

		for idx, err := range attempts {
			if err != nil {
				t.Fatalf("attempt %d error = %v, want nil", idx, err)
			}
		}
		running := countLoopRunsByStatus(ctx, t, globalDB, "ws-1", "delivery", looppkg.StatusRunning)
		if running != 1 {
			t.Fatalf("running loop_runs = %d, want 1", running)
		}
		queued := countLoopRunsByStatus(ctx, t, globalDB, "ws-1", "delivery", looppkg.StatusQueued)
		if queued != len(attempts)-1 {
			t.Fatalf("queued loop_runs = %d, want %d", queued, len(attempts)-1)
		}
	})
}

// Invariant: pause provenance, cell fencing, retry restoration, and the coordinator wake commit
// under one SQLite authority. This store suite owns the pause/resume state machine.
func TestGlobalDBLoopNodePauseShouldFenceAndRestoreRetryState(t *testing.T) {
	t.Parallel()

	globalDB := openLoopTestGlobalDB(t)
	ctx := testutil.Context(t)
	now := time.Date(2026, time.August, 4, 9, 0, 0, 0, time.UTC)
	globalDB.now = func() time.Time { return now }
	run, err := globalDB.CreateLoopRunForStart(
		ctx,
		testLoopRun("looprun-node-pause", now, looppkg.StatusRunning),
		dsl.ConcurrencyAllow,
	)
	if err != nil {
		t.Fatalf("CreateLoopRunForStart() error = %v", err)
	}
	firstScheduledAt := now.Add(-time.Minute)
	nextAttemptAt := now.Add(10 * time.Minute)
	completeInitialCoordinatorForParkedFixture(t, globalDB, run, now)
	if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_generation_outputs (
		loop_run_id, generation, node_id, item_index, status, output_ref, attempt,
		next_attempt_at, first_scheduled_at, epoch
	) VALUES (?, 1, 'finish', 0, 'retrying', 'retry evidence', 2, ?, ?, 4)`,
		run.ID, nextAttemptAt, firstScheduledAt); err != nil {
		t.Fatalf("insert pause fixture output error = %v", err)
	}
	actor := operatorActorContextForTest("operator:pause")
	pauseAt := now.Add(time.Minute)
	paused, err := globalDB.PauseNode(ctx, looppkg.NodePauseMutation{
		WorkspaceID: run.WorkspaceID, RunID: run.ID, NodeID: "finish",
		Mode: looppkg.NodePauseDrain, Reason: "inspect retry storm", Actor: actor, RequestedAt: pauseAt,
	})
	if err != nil {
		t.Fatalf("PauseNode() error = %v", err)
	}
	if !paused.Applied || !paused.Control.Paused || paused.Control.PauseProvenance == nil ||
		paused.Control.PauseProvenance.ActorID != "operator:pause" {
		t.Fatalf("PauseNode() = %#v, want applied pause provenance", paused)
	}
	replayed, err := globalDB.PauseNode(ctx, looppkg.NodePauseMutation{
		WorkspaceID: run.WorkspaceID, RunID: run.ID, NodeID: "finish",
		Mode: looppkg.NodePauseDrain, Reason: "duplicate", Actor: actor, RequestedAt: pauseAt.Add(time.Second),
	})
	if err != nil || replayed.Applied || !replayed.Control.Paused {
		t.Fatalf("PauseNode(replay) = %#v, %v, want idempotent committed truth", replayed, err)
	}
	resumeAt := now.Add(3 * time.Minute)
	resumed, err := globalDB.ResumeNode(ctx, looppkg.NodeResumeMutation{
		WorkspaceID: run.WorkspaceID, RunID: run.ID, NodeID: "finish",
		Mode: looppkg.NodeResumePlain, Actor: actor, RequestedAt: resumeAt,
	})
	if err != nil {
		t.Fatalf("ResumeNode() error = %v", err)
	}
	if !resumed.Applied || resumed.Control.Paused || resumed.Coordinator == nil {
		t.Fatalf("ResumeNode() = %#v, want released pause plus coordinator", resumed)
	}
	var status string
	var attempt, epoch int
	var storedNext, storedFirst time.Time
	if err := globalDB.db.QueryRowContext(ctx, `SELECT status, attempt, epoch,
		next_attempt_at, first_scheduled_at FROM loop_generation_outputs
		WHERE loop_run_id = ? AND generation = 1 AND node_id = 'finish'`, run.ID).
		Scan(&status, &attempt, &epoch, &storedNext, &storedFirst); err != nil {
		t.Fatalf("read resumed pause output error = %v", err)
	}
	if status != "retrying" || attempt != 2 || epoch != 6 || !storedNext.Equal(nextAttemptAt) ||
		!storedFirst.Equal(firstScheduledAt.Add(2*time.Minute)) {
		t.Fatalf("resumed output = %s/a%d/e%d next=%v first=%v, want preserved retry and shifted clock",
			status, attempt, epoch, storedNext, storedFirst)
	}
	_, err = globalDB.ResumeNode(ctx, looppkg.NodeResumeMutation{
		WorkspaceID: run.WorkspaceID, RunID: run.ID, NodeID: "finish",
		Mode: looppkg.NodeResumePlain, Actor: actor, RequestedAt: resumeAt.Add(time.Second),
	})
	if !errors.Is(err, looppkg.ErrInvalidTransition) {
		t.Fatalf("ResumeNode(unpaused) error = %v, want ErrInvalidTransition", err)
	}
	var reason *looppkg.ReasonError
	if !errors.As(err, &reason) || reason.Code != looppkg.ReasonCodeNodeNotPaused ||
		reason.Meta[looppkg.ReasonMetaActualState] != nodeLifecycleStateActive ||
		reason.Meta[looppkg.ReasonMetaAllowedTransitions] != "pause,cancel,kill" {
		t.Fatalf("ResumeNode(unpaused) ReasonError = %#v", reason)
	}
	resumeClaim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
		RunID: resumed.Coordinator.ID, Scope: taskpkg.ScopeWorkspace,
		WorkspaceID: string(run.WorkspaceID), RunKind: taskpkg.RunKindCoordinator,
		ClaimerSessionID: "daemon-loop-pause-resume", ClaimedBy: &taskpkg.ActorIdentity{
			Kind: taskpkg.ActorKindDaemon, Ref: "loop",
		},
		LeaseDuration: time.Minute, Now: resumeAt.Add(2 * time.Second),
	})
	if err != nil {
		t.Fatalf("ClaimNextRun(pause resume) error = %v", err)
	}
	expectedEpoch := int64(6)
	if _, err := globalDB.CompleteCoordinatorAndEnqueueNext(
		ctx,
		taskpkg.CoordinatorCompletion{
			RunID: resumeClaim.Run.ID, ClaimToken: resumeClaim.ClaimToken,
			Actor: coordinatorActorContextForTest(), Now: resumeAt.Add(3 * time.Second),
			Plan: taskpkg.CoordinatorCompletionPlan{Yield: true, Snapshot: taskpkg.GenerationSnapshot{
				LoopRunID: string(run.ID), Generation: 1,
				Payload: looppkg.GenerationSnapshotPayload{Outputs: []looppkg.GenerationOutput{{
					Generation: 1, NodeID: "finish", Status: "retrying", OutputRef: "retry evidence",
					Attempt: 2, NextAttemptAt: &nextAttemptAt, FirstScheduledAt: &storedFirst,
					Epoch: expectedEpoch, ExpectedEpoch: &expectedEpoch,
				}}},
			}},
		},
		looppkg.NewStoreFinalizer(),
	); err != nil {
		t.Fatalf("CompleteCoordinatorAndEnqueueNext(resume fixture) error = %v", err)
	}
	secondPauseAt := now.Add(4 * time.Minute)
	if _, err := globalDB.PauseNode(ctx, looppkg.NodePauseMutation{
		WorkspaceID: run.WorkspaceID, RunID: run.ID, NodeID: "finish",
		Mode: looppkg.NodePauseDrain, Reason: "reset retry", Actor: actor, RequestedAt: secondPauseAt,
	}); err != nil {
		t.Fatalf("PauseNode(second episode) error = %v", err)
	}
	if _, err := globalDB.ResumeNode(ctx, looppkg.NodeResumeMutation{
		WorkspaceID: run.WorkspaceID, RunID: run.ID, NodeID: "finish",
		Mode: looppkg.NodeResumeResetAttempts, Actor: actor, RequestedAt: secondPauseAt.Add(time.Minute),
	}); err != nil {
		t.Fatalf("ResumeNode(reset attempts) error = %v", err)
	}
	var resetNext sql.NullTime
	if err := globalDB.db.QueryRowContext(ctx, `SELECT status, attempt, next_attempt_at
		FROM loop_generation_outputs WHERE loop_run_id = ? AND node_id = 'finish'`, run.ID).
		Scan(&status, &attempt, &resetNext); err != nil {
		t.Fatalf("read reset pause output error = %v", err)
	}
	if status != "pending" || attempt != 1 || resetNext.Valid {
		t.Fatalf("reset output = %s/a%d next=%v, want pending attempt 1 without delay", status, attempt, resetNext)
	}
	events, err := globalDB.ListLoopRunEvents(ctx, looppkg.RunEventQuery{
		WorkspaceID: run.WorkspaceID, RunID: run.ID, Limit: 100,
	})
	if err != nil {
		t.Fatalf("ListLoopRunEvents() error = %v", err)
	}
	if countLoopEventKindForTest(events, loopRunEventNodePaused) != 2 ||
		countLoopEventKindForTest(events, loopRunEventNodeResumed) != 2 {
		t.Fatalf("pause event counts = paused:%d resumed:%d, want 2 each",
			countLoopEventKindForTest(events, loopRunEventNodePaused),
			countLoopEventKindForTest(events, loopRunEventNodeResumed))
	}
	effects, err := globalDB.ListEffectOutbox(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("ListEffectOutbox() error = %v", err)
	}
	if len(effects) != 2 || effects[0].Trigger != string(looppkg.EffectTriggerOnPause) ||
		effects[1].Trigger != string(looppkg.EffectTriggerOnPause) {
		t.Fatalf("pause effects = %#v, want one on_pause delivery per applied episode", effects)
	}

	t.Run("Should preserve attempts and clear backoff on immediate resume", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 4, 9, 30, 0, 0, time.UTC)
		run, err := globalDB.CreateLoopRunForStart(
			ctx,
			testLoopRun("looprun-node-pause-immediate", now, looppkg.StatusRunning),
			dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		completeInitialCoordinatorForParkedFixture(t, globalDB, run, now)
		nextAttemptAt := now.Add(10 * time.Minute)
		if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_generation_outputs (
			loop_run_id, generation, node_id, item_index, status, output_ref, attempt,
			next_attempt_at, first_scheduled_at, epoch
		) VALUES (?, 1, 'finish', 0, 'retrying', 'retry evidence', 5, ?, ?, 4)`,
			run.ID, nextAttemptAt, now.Add(-time.Minute)); err != nil {
			t.Fatalf("insert immediate-resume fixture error = %v", err)
		}
		actor := operatorActorContextForTest("operator:immediate")
		if _, err := globalDB.PauseNode(ctx, looppkg.NodePauseMutation{
			WorkspaceID: run.WorkspaceID, RunID: run.ID, NodeID: "finish",
			Mode: looppkg.NodePauseDrain, Reason: "repair now", Actor: actor, RequestedAt: now.Add(time.Minute),
		}); err != nil {
			t.Fatalf("PauseNode() error = %v", err)
		}
		resumed, err := globalDB.ResumeNode(ctx, looppkg.NodeResumeMutation{
			WorkspaceID: run.WorkspaceID, RunID: run.ID, NodeID: "finish",
			Mode: looppkg.NodeResumeImmediate, Actor: actor, RequestedAt: now.Add(2 * time.Minute),
		})
		if err != nil {
			t.Fatalf("ResumeNode(immediate) error = %v", err)
		}
		var status string
		var attempt int
		var next sql.NullTime
		var outputRef sql.NullString
		if err := globalDB.db.QueryRowContext(ctx, `SELECT status, attempt, next_attempt_at, output_ref
			FROM loop_generation_outputs WHERE loop_run_id = ? AND node_id = 'finish'`, run.ID).
			Scan(&status, &attempt, &next, &outputRef); err != nil {
			t.Fatalf("read immediate resume output error = %v", err)
		}
		if !resumed.Applied || resumed.Coordinator == nil || status != "pending" || attempt != 5 ||
			next.Valid || outputRef.Valid {
			t.Fatalf("immediate resume = %#v output=%s/a%d next=%v ref=%v", resumed, status, attempt, next, outputRef)
		}
	})

	t.Run("Should distinguish drain from cancel for an active cell", func(t *testing.T) {
		t.Parallel()

		for _, testCase := range []struct {
			name       string
			mode       looppkg.NodePauseMode
			wantStatus string
		}{
			{name: "drain", mode: looppkg.NodePauseDrain, wantStatus: "running"},
			{name: "cancel", mode: looppkg.NodePauseCancel, wantStatus: "paused"},
		} {
			t.Run("Should apply "+testCase.name+" semantics", func(t *testing.T) {
				t.Parallel()

				globalDB := openLoopTestGlobalDB(t)
				ctx := testutil.Context(t)
				now := time.Date(2026, time.August, 4, 9, 45, 0, 0, time.UTC)
				run, err := globalDB.CreateLoopRunForStart(
					ctx,
					testLoopRun("looprun-node-pause-active-"+testCase.name, now, looppkg.StatusRunning),
					dsl.ConcurrencyAllow,
				)
				if err != nil {
					t.Fatalf("CreateLoopRunForStart() error = %v", err)
				}
				completeInitialCoordinatorForParkedFixture(t, globalDB, run, now)
				if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_generation_outputs (
					loop_run_id, generation, node_id, item_index, status, attempt, first_scheduled_at, epoch
				) VALUES (?, 1, 'finish', 0, 'running', 1, ?, 4)`, run.ID, now); err != nil {
					t.Fatalf("insert active pause fixture error = %v", err)
				}
				if _, err := globalDB.PauseNode(ctx, looppkg.NodePauseMutation{
					WorkspaceID: run.WorkspaceID, RunID: run.ID, NodeID: "finish",
					Mode: testCase.mode, Reason: "hold active work",
					Actor: operatorActorContextForTest("operator:pause-active"), RequestedAt: now.Add(time.Minute),
				}); err != nil {
					t.Fatalf("PauseNode(%s) error = %v", testCase.name, err)
				}
				var status string
				if err := globalDB.db.QueryRowContext(ctx, `SELECT status FROM loop_generation_outputs
					WHERE loop_run_id = ? AND node_id = 'finish'`, run.ID).Scan(&status); err != nil {
					t.Fatalf("read %s status error = %v", testCase.name, err)
				}
				if status != testCase.wantStatus {
					t.Fatalf("active pause status = %q, want %q for %s", status, testCase.wantStatus, testCase.name)
				}
			})
		}
	})

	t.Run("Should reload cancellation sessions when a cancel pause is replayed", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 4, 9, 48, 0, 0, time.UTC)
		run, taskRunID := seedLiveLoopLivenessCellForTest(t, globalDB, now)
		if _, err := globalDB.db.ExecContext(ctx, `UPDATE task_runs
			SET metadata_json = json_set(metadata_json, '$.node_id', 'finish') WHERE id = ?`, taskRunID); err != nil {
			t.Fatalf("retarget pause task metadata error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(ctx, `UPDATE loop_generation_outputs
			SET node_id = 'finish' WHERE loop_run_id = ? AND task_run_id = ?`, run.ID, taskRunID); err != nil {
			t.Fatalf("retarget pause output error = %v", err)
		}
		seedLoopCancellationBindingForTest(
			t, globalDB, string(run.ID), string(run.WorkspaceID), "main", 1, "session-work", now,
		)
		mutation := looppkg.NodePauseMutation{
			WorkspaceID: run.WorkspaceID, RunID: run.ID, NodeID: "finish",
			Mode: looppkg.NodePauseCancel, Reason: "repair active work",
			Actor: operatorActorContextForTest("operator:pause-retry"), RequestedAt: now.Add(time.Minute),
		}
		first, err := globalDB.PauseNode(ctx, mutation)
		if err != nil {
			t.Fatalf("PauseNode(first cancel pause) error = %v", err)
		}
		replay, err := globalDB.PauseNode(ctx, mutation)
		if err != nil {
			t.Fatalf("PauseNode(replayed cancel pause) error = %v", err)
		}
		if !first.Applied || replay.Applied ||
			!slices.Equal(first.SessionIDs, []string{"session-work"}) ||
			!slices.Equal(replay.SessionIDs, first.SessionIDs) {
			t.Fatalf("cancel pause first/replay = %#v / %#v, want durable session redelivery", first, replay)
		}
	})

	t.Run("Should pause and resume only the addressed fan-out cell", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 4, 9, 49, 0, 0, time.UTC)
		run, err := globalDB.CreateLoopRunForStart(
			ctx, testLoopRun("looprun-node-pause-lane", now, looppkg.StatusRunning), dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		completeInitialCoordinatorForParkedFixture(t, globalDB, run, now)
		nextAttemptAt := now.Add(10 * time.Minute)
		if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_generation_outputs (
			loop_run_id, generation, node_id, item_index, status, attempt, next_attempt_at,
			first_scheduled_at, epoch
		) VALUES (?, 1, 'finish', 0, 'retrying', 2, ?, ?, 4),
			(?, 1, 'finish', 1, 'retrying', 3, ?, ?, 8)`, run.ID, nextAttemptAt, now,
			run.ID, nextAttemptAt, now); err != nil {
			t.Fatalf("insert lane pause fixture error = %v", err)
		}
		itemIndex := 1
		actor := operatorActorContextForTest("operator:pause-lane")
		paused, err := globalDB.PauseNode(ctx, looppkg.NodePauseMutation{
			WorkspaceID: run.WorkspaceID, RunID: run.ID, NodeID: "finish", ItemIndex: &itemIndex,
			Mode: looppkg.NodePauseDrain, Reason: "inspect one lane", Actor: actor, RequestedAt: now.Add(time.Minute),
		})
		if err != nil {
			t.Fatalf("PauseNode(lane) error = %v", err)
		}
		if !paused.Applied {
			t.Fatalf("PauseNode(lane) = %#v, want applied", paused)
		}
		resumed, err := globalDB.ResumeNode(ctx, looppkg.NodeResumeMutation{
			WorkspaceID: run.WorkspaceID, RunID: run.ID, NodeID: "finish", ItemIndex: &itemIndex,
			Mode: looppkg.NodeResumeImmediate, Actor: actor, RequestedAt: now.Add(2 * time.Minute),
		})
		if err != nil {
			t.Fatalf("ResumeNode(lane) error = %v", err)
		}
		if !resumed.Applied || resumed.Coordinator == nil {
			t.Fatalf("ResumeNode(lane) = %#v, want applied coordinator wake", resumed)
		}
		readLane := func(itemIndex int) (string, int, time.Time) {
			t.Helper()
			var status string
			var epoch int
			var first time.Time
			if err := globalDB.db.QueryRowContext(ctx, `SELECT status, epoch, first_scheduled_at
				FROM loop_generation_outputs WHERE loop_run_id = ? AND node_id = 'finish'
				AND item_index = ?`, run.ID, itemIndex).Scan(&status, &epoch, &first); err != nil {
				t.Fatalf("read lane %d pause output error = %v", itemIndex, err)
			}
			return status, epoch, first
		}
		siblingStatus, siblingEpoch, siblingFirst := readLane(0)
		laneStatus, laneEpoch, laneFirst := readLane(1)
		if siblingStatus != "retrying" || siblingEpoch != 4 || !siblingFirst.Equal(now) ||
			laneStatus != "pending" || laneEpoch != 10 || !laneFirst.Equal(now.Add(time.Minute)) {
			t.Fatalf("lane pause outputs = sibling %s/e%d/%v lane %s/e%d/%v", siblingStatus, siblingEpoch,
				siblingFirst, laneStatus, laneEpoch, laneFirst)
		}
	})

	t.Run("Should reject a pause on a terminal run", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 4, 9, 50, 0, 0, time.UTC)
		run, err := globalDB.CreateLoopRunForStart(
			ctx,
			testLoopRun("looprun-node-pause-terminal", now, looppkg.StatusRunning),
			dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		_, err = globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_runs SET status = 'done' WHERE id = ?`,
			run.ID,
		)
		if err != nil {
			t.Fatalf("terminalize pause fixture error = %v", err)
		}
		_, err = globalDB.PauseNode(ctx, looppkg.NodePauseMutation{
			WorkspaceID: run.WorkspaceID, RunID: run.ID, NodeID: "finish",
			Mode: looppkg.NodePauseDrain, Reason: "too late", Actor: operatorActorContextForTest("operator:late"),
			RequestedAt: now.Add(time.Minute),
		})
		if !errors.Is(err, looppkg.ErrInvalidTransition) {
			t.Fatalf("PauseNode(terminal) error = %v, want ErrInvalidTransition", err)
		}
	})
}

// Invariant: the wait claim, cell result, provenance event, and coordinator reservation are
// indivisible, and schema admission failure remains parked until the durable intervention bound.
// This store suite owns the governed resume transaction.
func TestGlobalDBLoopWaitResumeShouldClaimExactlyOnce(t *testing.T) {
	t.Parallel()

	globalDB := openLoopTestGlobalDB(t)
	ctx := testutil.Context(t)
	now := time.Date(2026, time.August, 4, 10, 0, 0, 0, time.UTC)
	globalDB.now = func() time.Time { return now }
	run, err := globalDB.CreateLoopRunForStart(
		ctx,
		testLoopRun("looprun-wait-resume", now, looppkg.StatusRunning),
		dsl.ConcurrencyAllow,
	)
	if err != nil {
		t.Fatalf("CreateLoopRunForStart() error = %v", err)
	}
	completeInitialCoordinatorForParkedFixture(t, globalDB, run, now)
	expect := json.RawMessage(
		`{"type":"object","required":["approved"],` +
			`"properties":{"approved":{"type":"boolean"}},"additionalProperties":false}`,
	)
	seedLoopWaitCellForTest(t, globalDB, run, "approval", 0, "event", 3, expect, nil, nil, now)
	results := make([]looppkg.WaitResumeResult, 2)
	errs := make([]error, 2)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for index := range results {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			results[index], errs[index] = globalDB.ResumeWait(context.Background(), looppkg.WaitResumeMutation{
				WorkspaceID: run.WorkspaceID, RunID: run.ID, Generation: 1,
				NodeID: "approval", ItemIndex: 0, Payload: []byte(`{"approved":true}`),
				ClaimedByKind: "operator", ClaimedByID: fmt.Sprintf("operator:%d", index),
				AdmissionAttempts: 3, RequestedAt: now.Add(time.Minute),
			})
		}(index)
	}
	close(start)
	wg.Wait()
	wins := 0
	winnerID := ""
	for index, result := range results {
		if errs[index] != nil {
			t.Fatalf("ResumeWait(%d) error = %v", index, errs[index])
		}
		if result.Won {
			wins++
			winnerID = result.Wait.ClaimedByID
			if result.Coordinator == nil {
				t.Fatalf("ResumeWait(%d) winner has no coordinator", index)
			}
		}
	}
	if wins != 1 {
		t.Fatalf("ResumeWait() wins = %d, want exactly 1", wins)
	}
	for _, result := range results {
		if result.Wait.ClaimedByID != winnerID || result.Wait.ClaimState != looppkg.WaitClaimResumed {
			t.Fatalf("ResumeWait() result = %#v, want winner provenance %q", result, winnerID)
		}
	}
	var waitState, outputStatus string
	var outputEpoch int
	if err := globalDB.db.QueryRowContext(ctx, `SELECT wait.claim_state, output.status, output.epoch
		FROM loop_node_waits AS wait JOIN loop_generation_outputs AS output
		ON output.loop_run_id = wait.loop_run_id AND output.generation = wait.generation
		AND output.node_id = wait.node_id AND output.item_index = wait.item_index
		WHERE wait.loop_run_id = ? AND wait.node_id = 'approval'`, run.ID).
		Scan(&waitState, &outputStatus, &outputEpoch); err != nil {
		t.Fatalf("read resumed wait truth error = %v", err)
	}
	if waitState != string(looppkg.WaitClaimResumed) || outputStatus != "succeeded" || outputEpoch != 4 {
		t.Fatalf("resumed wait truth = %s/%s/e%d, want resumed/succeeded/e4", waitState, outputStatus, outputEpoch)
	}
	events, err := globalDB.ListLoopRunEvents(ctx, looppkg.RunEventQuery{
		WorkspaceID: run.WorkspaceID, RunID: run.ID, Limit: 100,
	})
	if err != nil {
		t.Fatalf("ListLoopRunEvents() error = %v", err)
	}
	if countLoopEventKindForTest(events, loopRunEventNodeWaitResumed) != 1 {
		t.Fatalf("node_wait_resumed count = %d, want 1", countLoopEventKindForTest(events, loopRunEventNodeWaitResumed))
	}

	seedLoopWaitCellForTest(t, globalDB, run, "invalid", 0, "event", 7, expect, nil, nil, now)
	for attempt := 1; attempt <= 3; attempt++ {
		_, err := globalDB.ResumeWait(ctx, looppkg.WaitResumeMutation{
			WorkspaceID: run.WorkspaceID, RunID: run.ID, Generation: 1,
			NodeID: "invalid", ItemIndex: 0, Payload: []byte(`{"approved":"yes"}`),
			ClaimedByKind: "operator", ClaimedByID: "operator:invalid",
			AdmissionAttempts: 3, RequestedAt: now.Add(time.Duration(attempt) * time.Minute),
		})
		if !errors.Is(err, looppkg.ErrValidation) {
			t.Fatalf("ResumeWait(invalid attempt %d) error = %v, want ErrValidation", attempt, err)
		}
	}
	var failures int
	if err := globalDB.db.QueryRowContext(ctx, `SELECT claim_state, admission_failures
		FROM loop_node_waits WHERE loop_run_id = ? AND node_id = 'invalid'`, run.ID).
		Scan(&waitState, &failures); err != nil {
		t.Fatalf("read invalid wait truth error = %v", err)
	}
	if waitState != string(looppkg.WaitClaimInterventionRequired) || failures != 3 {
		t.Fatalf("invalid wait truth = %s/%d, want intervention_required/3", waitState, failures)
	}
	events, err = globalDB.ListLoopRunEvents(ctx, looppkg.RunEventQuery{
		WorkspaceID: run.WorkspaceID, RunID: run.ID, Limit: 100,
	})
	if err != nil {
		t.Fatalf("ListLoopRunEvents(after intervention) error = %v", err)
	}
	if countLoopEventKindForTest(events, loopRunEventNodeAttentionFlagged) != 1 {
		t.Fatalf("node_attention_flagged count = %d, want 1",
			countLoopEventKindForTest(events, loopRunEventNodeAttentionFlagged))
	}

	t.Run("Should list only active waits with exact fanout identity", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 4, 10, 15, 0, 0, time.UTC)
		run, err := globalDB.CreateLoopRunForStart(
			ctx,
			testLoopRun("looprun-wait-inventory", now, looppkg.StatusRunning),
			dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		completeInitialCoordinatorForParkedFixture(t, globalDB, run, now)
		waits, err := globalDB.ListNodeWaits(ctx, run.WorkspaceID, run.ID)
		if err != nil || len(waits) != 0 {
			t.Fatalf("ListNodeWaits(empty) = %#v, %v, want truthful empty inventory", waits, err)
		}
		expect := json.RawMessage(`{"type":"object"}`)
		seedLoopWaitCellForTest(t, globalDB, run, "wait_for_review", 0, "event", 3, expect, nil, nil, now)
		seedLoopWaitCellForTest(
			t, globalDB, run, "wait_for_review", 1, looppkg.NodeWaitKindEvent, 5,
			expect, nil, nil, now.Add(time.Second),
		)
		waits, err = globalDB.ListNodeWaits(ctx, run.WorkspaceID, run.ID)
		if err != nil {
			t.Fatalf("ListNodeWaits(active) error = %v", err)
		}
		if len(waits) != 2 || waits[0].ItemIndex != 0 || waits[1].ItemIndex != 1 ||
			waits[0].ClaimState != looppkg.WaitClaimWaiting || !waits[1].CreatedAt.Equal(now.Add(time.Second)) {
			t.Fatalf("ListNodeWaits(active) = %#v, want two ordered fanout identities", waits)
		}
		if _, err := globalDB.ResumeWait(ctx, looppkg.WaitResumeMutation{
			WorkspaceID: run.WorkspaceID, RunID: run.ID, Generation: 1,
			NodeID: "wait_for_review", ItemIndex: 0, Payload: []byte(`{}`),
			ClaimedByKind: "operator", ClaimedByID: "operator:inventory",
			AdmissionAttempts: 3, RequestedAt: now.Add(time.Minute),
		}); err != nil {
			t.Fatalf("ResumeWait(inventory item) error = %v", err)
		}
		waits, err = globalDB.ListNodeWaits(ctx, run.WorkspaceID, run.ID)
		if err != nil || len(waits) != 1 || waits[0].ItemIndex != 1 {
			t.Fatalf("ListNodeWaits(after resume) = %#v, %v, want only unresolved item 1", waits, err)
		}
		foreign, err := globalDB.ListNodeWaits(ctx, "ws-foreign", run.ID)
		if err != nil || len(foreign) != 0 {
			t.Fatalf("ListNodeWaits(foreign workspace) = %#v, %v, want empty", foreign, err)
		}
	})

	t.Run("Should resume exactly once after reopening the database", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), storepkg.GlobalDatabaseName)
		globalDB, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		activeDB := globalDB
		t.Cleanup(func() {
			if activeDB == nil {
				return
			}
			if closeErr := activeDB.Close(testutil.Context(t)); closeErr != nil {
				t.Errorf("Close() error = %v", closeErr)
			}
		})
		seedLoopTestWorkspaces(t, globalDB, "ws-1")
		now := time.Date(2026, time.August, 4, 10, 20, 0, 0, time.UTC)
		globalDB.now = func() time.Time { return now }
		run, err := globalDB.CreateLoopRunForStart(
			ctx,
			testLoopRun("looprun-wait-reopen", now, looppkg.StatusRunning),
			dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		completeInitialCoordinatorForParkedFixture(t, globalDB, run, now)
		seedLoopWaitCellForTest(
			t, globalDB, run, "wait_after_restart", 0, "event", 3,
			json.RawMessage(`{"type":"object"}`), nil, nil, now,
		)
		if err := globalDB.Close(ctx); err != nil {
			t.Fatalf("Close(before reopen) error = %v", err)
		}
		activeDB = nil

		reopened, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(reopen) error = %v", err)
		}
		activeDB = reopened
		reopened.now = func() time.Time { return now.Add(time.Minute) }
		mutation := looppkg.WaitResumeMutation{
			WorkspaceID: run.WorkspaceID, RunID: run.ID, Generation: 1,
			NodeID: "wait_after_restart", ItemIndex: 0, Payload: []byte(`{}`),
			ClaimedByKind: "event", ClaimedByID: "task_events:42",
			AdmissionAttempts: 3, RequestedAt: now.Add(time.Minute),
		}
		winner, err := reopened.ResumeWait(ctx, mutation)
		if err != nil {
			t.Fatalf("ResumeWait(after reopen) error = %v", err)
		}
		replay, err := reopened.ResumeWait(ctx, mutation)
		if err != nil {
			t.Fatalf("ResumeWait(replay after reopen) error = %v", err)
		}
		if !winner.Won || winner.Coordinator == nil || replay.Won || replay.Coordinator != nil ||
			replay.Wait.ClaimedByID != winner.Wait.ClaimedByID {
			t.Fatalf("restart resume winner/replay = %#v / %#v", winner, replay)
		}
		var coordinatorRuns int
		if err := reopened.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_runs
			WHERE loop_run_id = ? AND run_kind = 'coordinator'`, run.ID).Scan(&coordinatorRuns); err != nil {
			t.Fatalf("count coordinator runs after replay error = %v", err)
		}
		if coordinatorRuns != 2 {
			t.Fatalf("coordinator runs after restart replay = %d, want initial plus one resume", coordinatorRuns)
		}
	})

	t.Run("Should roll back a claim when a later resume write fails", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 4, 10, 25, 0, 0, time.UTC)
		run, err := globalDB.CreateLoopRunForStart(
			ctx,
			testLoopRun("looprun-wait-resume-rollback", now, looppkg.StatusRunning),
			dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		completeInitialCoordinatorForParkedFixture(t, globalDB, run, now)
		seedLoopWaitCellForTest(
			t, globalDB, run, "wait_for_commit", 0, "event", 3,
			json.RawMessage(`{"type":"object"}`), nil, nil, now,
		)
		if _, err := globalDB.db.ExecContext(ctx, `CREATE TRIGGER reject_wait_resume_event
			BEFORE INSERT ON loop_run_events WHEN NEW.kind = 'node_wait_resumed'
			BEGIN SELECT RAISE(ABORT, 'forced wait resume failure'); END`); err != nil {
			t.Fatalf("create wait resume failure trigger error = %v", err)
		}
		mutation := looppkg.WaitResumeMutation{
			WorkspaceID: run.WorkspaceID, RunID: run.ID, Generation: 1,
			NodeID: "wait_for_commit", ItemIndex: 0, Payload: []byte(`{}`),
			ClaimedByKind: "operator", ClaimedByID: "operator:rollback",
			AdmissionAttempts: 3, RequestedAt: now.Add(time.Minute),
		}
		if _, err := globalDB.ResumeWait(ctx, mutation); err == nil ||
			!strings.Contains(err.Error(), "forced wait resume failure") {
			t.Fatalf("ResumeWait(forced failure) error = %v, want injected failure", err)
		}
		var waitState, outputStatus string
		if err := globalDB.db.QueryRowContext(ctx, `SELECT wait.claim_state, output.status
			FROM loop_node_waits AS wait JOIN loop_generation_outputs AS output
			ON output.loop_run_id = wait.loop_run_id AND output.generation = wait.generation
			AND output.node_id = wait.node_id AND output.item_index = wait.item_index
			WHERE wait.loop_run_id = ? AND wait.node_id = 'wait_for_commit'`, run.ID).
			Scan(&waitState, &outputStatus); err != nil {
			t.Fatalf("read rolled-back wait truth error = %v", err)
		}
		if waitState != string(looppkg.WaitClaimWaiting) || outputStatus != "waiting" {
			t.Fatalf("rolled-back wait truth = %s/%s, want waiting/waiting", waitState, outputStatus)
		}
		if _, err := globalDB.db.ExecContext(ctx, `DROP TRIGGER reject_wait_resume_event`); err != nil {
			t.Fatalf("drop wait resume failure trigger error = %v", err)
		}
		result, err := globalDB.ResumeWait(ctx, mutation)
		if err != nil || !result.Won || result.Coordinator == nil {
			t.Fatalf("ResumeWait(after rollback) = %#v, %v, want one committed winner", result, err)
		}
	})

	t.Run("Should classify a missing wait output as a transition conflict", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 4, 10, 28, 0, 0, time.UTC)
		run, err := globalDB.CreateLoopRunForStart(
			ctx,
			testLoopRun("looprun-wait-resume-missing-output", now, looppkg.StatusRunning),
			dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		completeInitialCoordinatorForParkedFixture(t, globalDB, run, now)
		seedLoopWaitCellForTest(
			t, globalDB, run, "wait_without_output", 0, "event", 3,
			json.RawMessage(`{"type":"object"}`), nil, nil, now,
		)
		if _, err := globalDB.db.ExecContext(ctx, `DELETE FROM loop_generation_outputs
			WHERE loop_run_id = ? AND node_id = 'wait_without_output'`, run.ID); err != nil {
			t.Fatalf("delete wait output error = %v", err)
		}
		_, err = globalDB.ResumeWait(ctx, looppkg.WaitResumeMutation{
			WorkspaceID: run.WorkspaceID, RunID: run.ID, Generation: 1,
			NodeID: "wait_without_output", ItemIndex: 0, Payload: []byte(`{}`),
			ClaimedByKind: "operator", ClaimedByID: "operator:missing-output",
			AdmissionAttempts: 3, RequestedAt: now.Add(time.Minute),
		})
		if !errors.Is(err, looppkg.ErrTransitionConflict) {
			t.Fatalf("ResumeWait(missing output) error = %v, want ErrTransitionConflict", err)
		}
	})
}

// Invariant: the due-timer scanner uses the executed node lifecycle policy when deciding
// when invalid resume payloads require intervention. This store suite owns due wait recovery.
func TestGlobalDBLoopWaitDueShouldUsePinnedAdmissionAttempts(t *testing.T) {
	t.Parallel()

	globalDB := openLoopTestGlobalDB(t)
	ctx := testutil.Context(t)
	now := time.Date(2026, time.August, 4, 10, 30, 0, 0, time.UTC)
	globalDB.now = func() time.Time { return now }
	run := waitEscalationTestLoopRun("looprun-wait-due-admission", now, false, 1)
	created, err := globalDB.CreateLoopRunForStart(ctx, run, dsl.ConcurrencyAllow)
	if err != nil {
		t.Fatalf("CreateLoopRunForStart() error = %v", err)
	}
	completeInitialCoordinatorForParkedFixture(t, globalDB, created, now)
	expect := json.RawMessage(`{"type":"object","required":["approved"]}`)
	seedLoopWaitCellForTest(t, globalDB, created, "wait_for_ack", 0, "timer", 3, expect, &now, nil, now)

	runs, _, err := globalDB.ResumeDueLoopWaitsPage(ctx, now, looppkg.WaitDueCursor{}, 100)
	if !errors.Is(err, looppkg.ErrValidation) || len(runs) != 0 {
		t.Fatalf("ResumeDueLoopWaitsPage() = %d runs, %v, want validation failure without coordinator", len(runs), err)
	}
	var state string
	var failures int
	if err := globalDB.db.QueryRowContext(ctx, `SELECT claim_state, admission_failures
		FROM loop_node_waits WHERE loop_run_id = ? AND node_id = 'wait_for_ack'`, created.ID).
		Scan(&state, &failures); err != nil {
		t.Fatalf("read due timer admission state error = %v", err)
	}
	if state != string(looppkg.WaitClaimInterventionRequired) || failures != 1 {
		t.Fatalf("due timer admission = %s/%d, want intervention_required/1", state, failures)
	}

	for _, testCase := range []struct {
		name   string
		id     string
		park   func(*testing.T, *GlobalDB, looppkg.Run, time.Time)
		unpark func(*testing.T, *GlobalDB, looppkg.Run, time.Time)
	}{
		{
			name: "run pause",
			id:   "run-pause",
			park: func(t *testing.T, globalDB *GlobalDB, run looppkg.Run, _ time.Time) {
				t.Helper()
				if _, err := globalDB.db.ExecContext(testutil.Context(t),
					`UPDATE loop_runs SET status = 'paused' WHERE id = ?`, run.ID); err != nil {
					t.Fatalf("pause Loop run fixture error = %v", err)
				}
			},
			unpark: func(t *testing.T, globalDB *GlobalDB, run looppkg.Run, _ time.Time) {
				t.Helper()
				if _, err := globalDB.db.ExecContext(testutil.Context(t),
					`UPDATE loop_runs SET status = 'running' WHERE id = ?`, run.ID); err != nil {
					t.Fatalf("resume Loop run fixture error = %v", err)
				}
			},
		},
		{
			name: "node pause",
			id:   "node-pause",
			park: func(t *testing.T, globalDB *GlobalDB, run looppkg.Run, now time.Time) {
				t.Helper()
				if _, err := globalDB.PauseNode(testutil.Context(t), looppkg.NodePauseMutation{
					WorkspaceID: run.WorkspaceID, RunID: run.ID, NodeID: "wait_for_ack",
					Mode: looppkg.NodePauseDrain, Reason: "hold timer",
					Actor: operatorActorContextForTest("operator:timer"), RequestedAt: now,
				}); err != nil {
					t.Fatalf("PauseNode(timer fixture) error = %v", err)
				}
			},
			unpark: func(t *testing.T, globalDB *GlobalDB, run looppkg.Run, now time.Time) {
				t.Helper()
				if _, err := globalDB.ResumeNode(testutil.Context(t), looppkg.NodeResumeMutation{
					WorkspaceID: run.WorkspaceID, RunID: run.ID, NodeID: "wait_for_ack",
					Mode: looppkg.NodeResumePlain, Actor: operatorActorContextForTest("operator:timer"),
					RequestedAt: now,
				}); err != nil {
					t.Fatalf("ResumeNode(timer fixture) error = %v", err)
				}
			},
		},
	} {
		t.Run("Should leave due timers parked during "+testCase.name, func(t *testing.T) {
			t.Parallel()

			globalDB := openLoopTestGlobalDB(t)
			ctx := testutil.Context(t)
			now := time.Date(2026, time.August, 4, 10, 45, 0, 0, time.UTC)
			run := waitEscalationTestLoopRun("looprun-wait-due-"+testCase.id, now, false, 3)
			created, err := globalDB.CreateLoopRunForStart(ctx, run, dsl.ConcurrencyAllow)
			if err != nil {
				t.Fatalf("CreateLoopRunForStart() error = %v", err)
			}
			completeInitialCoordinatorForParkedFixture(t, globalDB, created, now)
			expect := json.RawMessage(`{"type":"object"}`)
			seedLoopWaitCellForTest(t, globalDB, created, "wait_for_ack", 0, "timer", 3, expect, &now, nil, now)
			testCase.park(t, globalDB, created, now.Add(time.Minute))
			runs, _, err := globalDB.ResumeDueLoopWaitsPage(
				ctx, now.Add(2*time.Minute), looppkg.WaitDueCursor{}, 100,
			)
			if err != nil || len(runs) != 0 {
				t.Fatalf("ResumeDueLoopWaitsPage(%s) = %#v, %v, want no wake", testCase.name, runs, err)
			}
			waits, err := globalDB.ListNodeWaits(ctx, created.WorkspaceID, created.ID)
			if err != nil || len(waits) != 1 || waits[0].ClaimState != looppkg.WaitClaimWaiting {
				t.Fatalf("ListNodeWaits(%s) = %#v, %v, want one waiting row", testCase.name, waits, err)
			}
			testCase.unpark(t, globalDB, created, now.Add(3*time.Minute))
			runs, _, err = globalDB.ResumeDueLoopWaitsPage(
				ctx, now.Add(4*time.Minute), looppkg.WaitDueCursor{}, 100,
			)
			if err != nil || len(runs) != 1 {
				t.Fatalf("ResumeDueLoopWaitsPage(after %s) = %#v, %v, want one wake", testCase.name, runs, err)
			}
			waits, err = globalDB.ListNodeWaits(ctx, created.WorkspaceID, created.ID)
			if err != nil || len(waits) != 0 {
				t.Fatalf("ListNodeWaits(after %s) = %#v, %v, want resolved wait absent", testCase.name, waits, err)
			}
		})
	}
}

// Invariant: one normalized watch delivery identity creates one run even under contention;
// every loser receives the durable winner and increments the same loud tombstone counter.
// This store suite owns admission claims inside run-start transactions.
func TestGlobalDBLoopAdmissionClaimShouldSuppressConcurrentRedelivery(t *testing.T) {
	t.Parallel()

	globalDB := openLoopTestGlobalDB(t)
	ctx := testutil.Context(t)
	now := time.Date(2026, time.August, 4, 11, 0, 0, 0, time.UTC)
	globalDB.now = func() time.Time { return now }
	const contenders = 8
	results := make([]looppkg.Run, contenders)
	errs := make([]error, contenders)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for index := range contenders {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			run := testLoopRun(fmt.Sprintf("looprun-admission-%d", index), now, looppkg.StatusRunning)
			run.SetAdmission(looppkg.AdmissionIdentity{
				SourceKey: "  review-source  ", EventKey: "  review:42  ", Horizon: 7 * 24 * time.Hour,
			})
			results[index], errs[index] = globalDB.CreateLoopRunForStart(
				context.Background(), run, dsl.ConcurrencyAllow,
			)
		}(index)
	}
	close(start)
	wg.Wait()
	winnerID := looppkg.RunID("")
	wins := 0
	for index, result := range results {
		if errs[index] != nil {
			t.Fatalf("CreateLoopRunForStart(%d) error = %v", index, errs[index])
		}
		if result.Admission == nil || !result.Admission.Suppressed {
			wins++
			winnerID = result.ID
		}
	}
	if wins != 1 || winnerID == "" {
		t.Fatalf("admission wins = %d id=%q, want one winner", wins, winnerID)
	}
	for index, result := range results {
		if result.ID != winnerID {
			t.Fatalf("result %d run id = %q, want winner %q", index, result.ID, winnerID)
		}
	}
	claim, err := globalDB.GetAdmissionClaim(ctx, "ws-1", "delivery", "review-source", "review:42")
	if err != nil {
		t.Fatalf("GetAdmissionClaim() error = %v", err)
	}
	if claim.LoopRunID != winnerID || claim.SuppressedCount != contenders-1 {
		t.Fatalf("admission claim = %#v, want winner and %d suppressions", claim, contenders-1)
	}
	if !claim.ExpiresAt.Equal(now.Add(7 * 24 * time.Hour)) {
		t.Fatalf("admission expiry = %s, want %s", claim.ExpiresAt, now.Add(7*24*time.Hour))
	}
	_, err = globalDB.db.ExecContext(
		ctx,
		`UPDATE loop_runs SET status = 'done' WHERE id = ?`,
		winnerID,
	)
	if err != nil {
		t.Fatalf("terminalize admission winner error = %v", err)
	}
	terminalDuplicate := testLoopRun(
		"looprun-admission-terminal-redelivery",
		now.Add(time.Minute),
		looppkg.StatusRunning,
	)
	terminalDuplicate.SetAdmission(looppkg.AdmissionIdentity{
		SourceKey: "review-source", EventKey: "review:42", Horizon: 7 * 24 * time.Hour,
	})
	suppressed, err := globalDB.CreateLoopRunForStart(ctx, terminalDuplicate, dsl.ConcurrencyAllow)
	if err != nil {
		t.Fatalf("CreateLoopRunForStart(terminal duplicate) error = %v", err)
	}
	if suppressed.Admission == nil || !suppressed.Admission.Suppressed || suppressed.ID != winnerID {
		t.Fatalf("terminal duplicate = %#v, want original tombstone winner", suppressed)
	}
	distinct := testLoopRun(
		"looprun-admission-distinct",
		now.Add(2*time.Minute),
		looppkg.StatusRunning,
	)
	distinct.SetAdmission(looppkg.AdmissionIdentity{
		SourceKey: "review-source", EventKey: "review:43", Horizon: 7 * 24 * time.Hour,
	})
	admitted, err := globalDB.CreateLoopRunForStart(ctx, distinct, dsl.ConcurrencyAllow)
	if err != nil || admitted.Admission == nil || admitted.Admission.Suppressed {
		t.Fatalf("CreateLoopRunForStart(distinct) = %#v, %v, want admitted", admitted, err)
	}
	expiresAt := now.Add(7 * 24 * time.Hour)
	swept, err := globalDB.SweepAdmissionClaims(ctx, expiresAt.Add(-time.Nanosecond), 100)
	if err != nil || swept != 0 {
		t.Fatalf("SweepAdmissionClaims(before expiry) = %d, %v, want 0", swept, err)
	}
	globalDB.now = func() time.Time { return expiresAt }
	expiredDuplicate := testLoopRun(
		"looprun-admission-after-expiry",
		expiresAt,
		looppkg.StatusRunning,
	)
	expiredDuplicate.SetAdmission(looppkg.AdmissionIdentity{
		SourceKey: "review-source", EventKey: "review:42", Horizon: 7 * 24 * time.Hour,
	})
	reclaimed, err := globalDB.CreateLoopRunForStart(ctx, expiredDuplicate, dsl.ConcurrencyAllow)
	if err != nil || reclaimed.Admission == nil || reclaimed.Admission.Suppressed ||
		reclaimed.ID != expiredDuplicate.ID {
		t.Fatalf("CreateLoopRunForStart(expired identity) = %#v, %v, want newly admitted run", reclaimed, err)
	}
	swept, err = globalDB.SweepAdmissionClaims(ctx, expiresAt, 100)
	if err != nil || swept != 1 {
		t.Fatalf("SweepAdmissionClaims(expired) = %d, %v, want 1 unrelated expired claim", swept, err)
	}
}

// Invariant: authored wait escalation advances in durable order through the effect outbox;
// a decision wins the same claim race and cancels every future ladder step, while a terminal
// expiry without a route becomes visible attention. This store suite owns ladder due work.
func TestGlobalDBLoopWaitEscalationShouldUseRelayAndHonorDecision(t *testing.T) {
	t.Parallel()

	t.Run("Should stop a ladder after a decision claims the wait", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 4, 12, 0, 0, 0, time.UTC)
		globalDB.now = func() time.Time { return now }
		run := waitEscalationTestLoopRun("looprun-wait-ladder-decision", now, true, 3)
		created, err := globalDB.CreateLoopRunForStart(ctx, run, dsl.ConcurrencyAllow)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		seedWaitEscalationFixture(t, globalDB, created, now, true)
		if _, err := globalDB.EscalateDueLoopWaitsPage(
			ctx, now.Add(time.Minute), looppkg.WaitEscalationCursor{}, 100,
		); err != nil {
			t.Fatalf("EscalateDueLoopWaitsPage(first step) error = %v", err)
		}
		entries, err := globalDB.ListEffectOutbox(ctx, created.WorkspaceID, created.ID)
		if err != nil {
			t.Fatalf("ListEffectOutbox() error = %v", err)
		}
		if len(entries) != 1 || entries[0].Trigger != string(looppkg.EffectTriggerWaitEscalation) ||
			entries[0].EntryIndex != 0 || entries[0].State != looppkg.EffectPending {
			t.Fatalf("wait ladder outbox = %#v, want one pending first step", entries)
		}
		decision, err := globalDB.ResumeWait(ctx, looppkg.WaitResumeMutation{
			WorkspaceID: created.WorkspaceID, RunID: created.ID, Generation: 1,
			NodeID: "wait_for_ack", ItemIndex: 0, Payload: []byte(`{"approved":true}`),
			ClaimedByKind: "operator", ClaimedByID: "operator:approver",
			AdmissionAttempts: 3, RequestedAt: now.Add(90 * time.Second),
		})
		if err != nil || !decision.Won {
			t.Fatalf("ResumeWait(decision) = %#v, %v, want winner", decision, err)
		}
		if _, err := globalDB.EscalateDueLoopWaitsPage(
			ctx, now.Add(3*time.Minute), looppkg.WaitEscalationCursor{}, 100,
		); err != nil {
			t.Fatalf("EscalateDueLoopWaitsPage(after decision) error = %v", err)
		}
		entries, err = globalDB.ListEffectOutbox(ctx, created.WorkspaceID, created.ID)
		if err != nil {
			t.Fatalf("ListEffectOutbox(after decision) error = %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("wait ladder outbox after decision = %#v, want no remaining steps", entries)
		}
	})

	t.Run("Should route terminal expiry after ordered effect steps", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 4, 13, 0, 0, 0, time.UTC)
		globalDB.now = func() time.Time { return now }
		created, err := globalDB.CreateLoopRunForStart(
			ctx,
			waitEscalationTestLoopRun("looprun-wait-ladder-route", now, true, 3),
			dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		seedWaitEscalationFixture(t, globalDB, created, now, true)
		if _, err := globalDB.EscalateDueLoopWaitsPage(
			ctx, now.Add(time.Minute), looppkg.WaitEscalationCursor{}, 100,
		); err != nil {
			t.Fatalf("EscalateDueLoopWaitsPage(effect) error = %v", err)
		}
		if _, err := globalDB.EscalateDueLoopWaitsPage(
			ctx, now.Add(2*time.Minute), looppkg.WaitEscalationCursor{}, 100,
		); err != nil {
			t.Fatalf("EscalateDueLoopWaitsPage(route) error = %v", err)
		}
		var claimState, claimedByKind, claimedByID, outputStatus, outputRef string
		if err := globalDB.db.QueryRowContext(ctx, `SELECT wait.claim_state,
			COALESCE(wait.claimed_by_kind, ''), COALESCE(wait.claimed_by_id, ''),
			output.status, COALESCE(output.output_ref, '') FROM loop_node_waits AS wait
			JOIN loop_generation_outputs AS output ON output.loop_run_id = wait.loop_run_id
			AND output.generation = wait.generation AND output.node_id = wait.node_id
			AND output.item_index = wait.item_index WHERE wait.loop_run_id = ?
			AND wait.node_id = 'wait_for_ack'`, created.ID).
			Scan(&claimState, &claimedByKind, &claimedByID, &outputStatus, &outputRef); err != nil {
			t.Fatalf("read routed wait expiry error = %v", err)
		}
		if claimState != string(looppkg.WaitClaimResumed) || claimedByKind != "expiry" ||
			claimedByID != "timeout" || outputStatus != "succeeded" ||
			outputRef != looppkg.WaitExpiryRouteOutputRef("timeout") {
			t.Fatalf("routed expiry = %s/%s/%s/%s/%s, want resumed expiry timeout route",
				claimState, claimedByKind, claimedByID, outputStatus, outputRef)
		}
		assertExpiredWaitResumeEventPayloadForTest(t, globalDB, created, "timeout", looppkg.NodeWaitKindTimer)
	})

	t.Run("Should flag terminal expiry when no timeout route exists", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 4, 14, 0, 0, 0, time.UTC)
		globalDB.now = func() time.Time { return now }
		created, err := globalDB.CreateLoopRunForStart(
			ctx,
			waitEscalationTestLoopRun("looprun-wait-expired-attention", now, false, 3),
			dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		seedWaitEscalationFixture(t, globalDB, created, now, false)
		if _, err := globalDB.EscalateDueLoopWaitsPage(
			ctx, now.Add(time.Minute), looppkg.WaitEscalationCursor{}, 100,
		); err != nil {
			t.Fatalf("EscalateDueLoopWaitsPage(expiry attention) error = %v", err)
		}
		var claimState, attentionFlag string
		if err := globalDB.db.QueryRowContext(ctx, `SELECT wait.claim_state,
			COALESCE(control.attention_flag, '') FROM loop_node_waits AS wait
			LEFT JOIN loop_node_controls AS control ON control.loop_run_id = wait.loop_run_id
			AND control.node_id = wait.node_id WHERE wait.loop_run_id = ?
			AND wait.node_id = 'wait_for_ack'`, created.ID).Scan(&claimState, &attentionFlag); err != nil {
			t.Fatalf("read expired wait attention error = %v", err)
		}
		if claimState != string(looppkg.WaitClaimInterventionRequired) || attentionFlag != "expired_wait" {
			t.Fatalf("expired wait = %s/%s, want intervention_required/expired_wait", claimState, attentionFlag)
		}
	})

	t.Run("Should let an approval decision atomically stop its escalation ladder", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 4, 14, 30, 0, 0, time.UTC)
		globalDB.now = func() time.Time { return now }
		created, err := globalDB.CreateLoopRunForStart(
			ctx,
			approvalEscalationTestLoopRun("looprun-approval-ladder-decision", now),
			dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		seedApprovalEscalationFixture(t, globalDB, created, now)
		created.Status = looppkg.StatusNeedsApproval
		created.ActiveGateID = "approval"
		if _, err := globalDB.EscalateDueLoopWaitsPage(
			ctx, now.Add(time.Minute), looppkg.WaitEscalationCursor{}, 100,
		); err != nil {
			t.Fatalf("EscalateDueLoopWaitsPage(first approval step) error = %v", err)
		}
		decidedAt := now.Add(90 * time.Second)
		actor := operatorActorContextForTest("approver-1")
		result, err := globalDB.ReactivateLoopCoordinator(ctx, &looppkg.CoordinatorReactivationRequest{
			Run: created, Cause: looppkg.TransitionCauseApproval, Actor: actor,
			Decisions: []looppkg.GateDecisionRecord{{
				WorkspaceID: created.WorkspaceID, RunID: created.ID, Generation: 1,
				GateID: "approval", CriterionID: "operator", Decision: looppkg.GateDecisionApprove,
				Actor: actor, DecidedAt: decidedAt,
			}},
			ReactivatedAt: decidedAt,
		})
		if err != nil || result.Run.ID == "" {
			t.Fatalf("ReactivateLoopCoordinator() = %#v, %v, want committed approval wake", result, err)
		}
		if _, err := globalDB.EscalateDueLoopWaitsPage(
			ctx, now.Add(3*time.Minute), looppkg.WaitEscalationCursor{}, 100,
		); err != nil {
			t.Fatalf("EscalateDueLoopWaitsPage(after approval) error = %v", err)
		}
		entries, err := globalDB.ListEffectOutbox(ctx, created.WorkspaceID, created.ID)
		if err != nil {
			t.Fatalf("ListEffectOutbox() error = %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("approval ladder outbox = %#v, want no step after decision", entries)
		}
		stored, err := globalDB.GetLoopRun(ctx, created.WorkspaceID, created.ID)
		if err != nil {
			t.Fatalf("GetLoopRun() error = %v", err)
		}
		var state, claimedBy string
		if err := globalDB.db.QueryRowContext(ctx, `SELECT claim_state, claimed_by_id
			FROM loop_node_waits WHERE loop_run_id = ? AND node_id = 'approval'`, created.ID).
			Scan(&state, &claimedBy); err != nil {
			t.Fatalf("read approval wait decision error = %v", err)
		}
		if stored.Status != looppkg.StatusRunning || stored.ActiveGateID != "" ||
			state != string(looppkg.WaitClaimResumed) || claimedBy != "approver-1" ||
			!stored.StartedAt.Equal(now.Add(90*time.Second)) {
			t.Fatalf("approved wait = status:%s gate:%s state:%s actor:%s started:%s",
				stored.Status, stored.ActiveGateID, state, claimedBy, stored.StartedAt)
		}
	})

	t.Run("Should reject an approval decision without exact fanout item identity", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 4, 14, 45, 0, 0, time.UTC)
		globalDB.now = func() time.Time { return now }
		created, err := globalDB.CreateLoopRunForStart(
			ctx,
			approvalEscalationTestLoopRun("looprun-approval-fanout-ambiguous", now),
			dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		seedApprovalEscalationFixture(t, globalDB, created, now)
		if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_generation_outputs (
			loop_run_id, generation, node_id, item_index, status, output_ref, attempt, epoch
		) VALUES (?, 1, 'approval', 1, 'succeeded', '{"outcome":"awaiting_approval"}', 1, 3)`,
			created.ID,
		); err != nil {
			t.Fatalf("insert second approval output error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_node_waits (
			loop_run_id, generation, node_id, item_index, kind, claim_state, issued_epoch, created_at
		) VALUES (?, 1, 'approval', 1, 'approval_escalation', 'waiting', 3, ?)`,
			created.ID,
			now,
		); err != nil {
			t.Fatalf("insert second approval wait error = %v", err)
		}
		created.Status = looppkg.StatusNeedsApproval
		created.ActiveGateID = "approval"
		actor := operatorActorContextForTest("approver-ambiguous")
		_, err = globalDB.ReactivateLoopCoordinator(ctx, &looppkg.CoordinatorReactivationRequest{
			Run: created, Cause: looppkg.TransitionCauseApproval, Actor: actor,
			Decisions: []looppkg.GateDecisionRecord{{
				WorkspaceID: created.WorkspaceID, RunID: created.ID, Generation: 1,
				GateID: "approval", CriterionID: "operator", Decision: looppkg.GateDecisionApprove,
				Actor: actor, DecidedAt: now.Add(time.Minute),
			}},
			ReactivatedAt: now.Add(time.Minute),
		})
		if !errors.Is(err, looppkg.ErrTransitionConflict) {
			t.Fatalf("ReactivateLoopCoordinator(ambiguous fanout) error = %v, want ErrTransitionConflict", err)
		}
		var activeWaits, decisions int
		if err := globalDB.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM loop_node_waits
			WHERE loop_run_id = ? AND node_id = 'approval' AND claim_state = 'waiting'`, created.ID).
			Scan(&activeWaits); err != nil {
			t.Fatalf("count active approval waits error = %v", err)
		}
		if err := globalDB.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM loop_gate_decisions
			WHERE loop_run_id = ? AND gate_id = 'approval'`, created.ID).Scan(&decisions); err != nil {
			t.Fatalf("count rolled-back approval decisions error = %v", err)
		}
		if activeWaits != 2 || decisions != 0 {
			t.Fatalf("ambiguous approval truth = %d waits/%d decisions, want 2/0", activeWaits, decisions)
		}
	})

	t.Run("Should route an expired approval and reactivate the run", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 4, 15, 0, 0, 0, time.UTC)
		globalDB.now = func() time.Time { return now }
		created, err := globalDB.CreateLoopRunForStart(
			ctx,
			approvalEscalationTestLoopRun("looprun-approval-ladder-route", now),
			dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		seedApprovalEscalationFixture(t, globalDB, created, now)
		if _, err := globalDB.EscalateDueLoopWaitsPage(
			ctx, now.Add(time.Minute), looppkg.WaitEscalationCursor{}, 100,
		); err != nil {
			t.Fatalf("EscalateDueLoopWaitsPage(effect) error = %v", err)
		}
		if _, err := globalDB.EscalateDueLoopWaitsPage(
			ctx, now.Add(2*time.Minute), looppkg.WaitEscalationCursor{}, 100,
		); err != nil {
			t.Fatalf("EscalateDueLoopWaitsPage(route) error = %v", err)
		}
		stored, err := globalDB.GetLoopRun(ctx, created.WorkspaceID, created.ID)
		if err != nil {
			t.Fatalf("GetLoopRun() error = %v", err)
		}
		var state, outputRef string
		if err := globalDB.db.QueryRowContext(ctx, `SELECT wait.claim_state, output.output_ref
			FROM loop_node_waits AS wait JOIN loop_generation_outputs AS output
			ON output.loop_run_id = wait.loop_run_id AND output.generation = wait.generation
			AND output.node_id = wait.node_id AND output.item_index = wait.item_index
			WHERE wait.loop_run_id = ? AND wait.node_id = 'approval'`, created.ID).
			Scan(&state, &outputRef); err != nil {
			t.Fatalf("read routed approval wait error = %v", err)
		}
		if stored.Status != looppkg.StatusRunning || stored.ActiveGateID != "" ||
			state != string(looppkg.WaitClaimResumed) ||
			outputRef != looppkg.WaitExpiryRouteOutputRef("timed_out") ||
			!stored.StartedAt.Equal(now.Add(2*time.Minute)) {
			t.Fatalf("routed approval = status:%s gate:%s state:%s output:%s started:%s",
				stored.Status, stored.ActiveGateID, state, outputRef, stored.StartedAt)
		}
		assertExpiredWaitResumeEventPayloadForTest(
			t, globalDB, created, "timed_out", looppkg.NodeWaitKindApprovalEscalation,
		)
	})

	t.Run("Should classify a missing escalation output as a transition conflict", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, time.August, 4, 15, 15, 0, 0, time.UTC)
		err := validateDueWaitEscalationCell(ctx, globalDB.db, waitEscalationCell{
			nextEscalationAt: now,
			runID:            "looprun-missing-escalation-output",
			generation:       1,
			nodeID:           "wait_for_ack",
			itemIndex:        0,
		}, looppkg.NodeWait{Kind: looppkg.NodeWaitKindTimer, IssuedEpoch: 3})
		if !errors.Is(err, looppkg.ErrTransitionConflict) {
			t.Fatalf("validateDueWaitEscalationCell(missing output) error = %v, want ErrTransitionConflict", err)
		}
	})
}

func testLoopRun(id string, at time.Time, status looppkg.Status) looppkg.Run {
	definition, err := dsl.Parse([]byte(
		`{"apiVersion":"compozy.loop/v1","kind":"Loop","meta":{"name":"delivery","version":1},"contract":{"goal":"test","definition_of_done":"done","iteration_cap":7,"no_progress":{"window":1},"budget":{"tokens":1,"wall_clock_sec":1}},"graph":{"nodes":[{"id":"finish","class":"action","kind":"transform","params":{"map":{"ok":{"value":true}}},"on_pause":[{"emit":{"kind":"finish_paused"}}]}],"edges":[]}}`,
	))
	if err != nil {
		panic(fmt.Sprintf("parse test Loop definition: %v", err))
	}
	resolved, err := looppkg.NewCompiler().Compile(definition)
	if err != nil {
		panic(fmt.Sprintf("compile test Loop definition: %v", err))
	}
	effective, err := looppkg.ResolveEffectiveConfig(
		resolved,
		looppkg.DefaultLoopDefaults(),
		nil,
		looppkg.LoopConfig{},
	)
	if err != nil {
		panic(fmt.Sprintf("resolve test Loop config: %v", err))
	}
	snapshot, digest, err := looppkg.BuildExecutedDefinitionSnapshot(resolved, effective)
	if err != nil {
		panic(fmt.Sprintf("build test executed definition snapshot: %v", err))
	}
	return looppkg.Run{
		ID:                  looppkg.RunID(id),
		WorkspaceID:         "ws-1",
		LoopName:            "delivery",
		Status:              status,
		ReattemptStrategy:   looppkg.ReattemptFailedOnly,
		CreatedAt:           at,
		StartedAt:           at,
		LastProgressAt:      at,
		DefinitionVersion:   resolved.DefinitionVersion,
		DefinitionDigest:    digest,
		DefinitionSnapshot:  snapshot,
		ActiveHumanCriteria: []byte(`[]`),
		StartMetadata:       map[string]any{},
		IterationCap:        7,
		BudgetOnExceeded:    dsl.BudgetExceededHalt,
		Origin:              &looppkg.RunOrigin{Kind: looppkg.RunOriginCatalog},
		Inputs:              map[string]any{"tasks": "task-ref"},
	}
}

func waitEscalationTestLoopRun(id string, at time.Time, withRoute bool, waitAdmissionAttempts int) looppkg.Run {
	source := `{"apiVersion":"compozy.loop/v1","kind":"Loop","meta":{"name":"delivery","version":1},"contract":{"goal":"test","definition_of_done":"done","iteration_cap":7,"no_progress":{"window":1},"budget":{"tokens":100,"wall_clock_sec":3600}},"graph":{"nodes":[{"id":"wait_for_ack","class":"control","kind":"wait","params":{"for":"24h","expect":{"type":"object","required":["approved"],"properties":{"approved":{"type":"boolean"}}},"expires":{"after":"1m"}}},{"id":"normal","class":"action","kind":"transform","params":{"map":{"path":{"value":"normal"}}}}],"edges":[{"from":"wait_for_ack","to":"normal"}]}}`
	if withRoute {
		source = `{"apiVersion":"compozy.loop/v1","kind":"Loop","meta":{"name":"delivery","version":1},"contract":{"goal":"test","definition_of_done":"done","iteration_cap":7,"no_progress":{"window":1},"budget":{"tokens":100,"wall_clock_sec":3600}},"graph":{"nodes":[{"id":"wait_for_ack","class":"control","kind":"wait","params":{"for":"24h","expect":{"type":"object","required":["approved"],"properties":{"approved":{"type":"boolean"}}},"expires":{"after":"1m","escalate":[{"emit":{"kind":"wait_reminder","payload":{"reminder":true}}}],"route":"timeout"}}},{"id":"normal","class":"action","kind":"transform","params":{"map":{"path":{"value":"normal"}}}},{"id":"timeout","class":"action","kind":"transform","params":{"map":{"path":{"value":"timeout"}}}}],"edges":[{"from":"wait_for_ack","to":"normal"},{"from":"wait_for_ack","to":"timeout"}]}}`
	}
	definition, err := dsl.Parse([]byte(source))
	if err != nil {
		panic(fmt.Sprintf("parse wait escalation test Loop definition: %v", err))
	}
	resolved, err := looppkg.NewCompiler().Compile(definition)
	if err != nil {
		panic(fmt.Sprintf("compile wait escalation test Loop definition: %v", err))
	}
	effective, err := looppkg.ResolveEffectiveConfig(
		resolved,
		looppkg.DefaultLoopDefaults(),
		nil,
		looppkg.LoopConfig{},
	)
	if err != nil {
		panic(fmt.Sprintf("resolve wait escalation test Loop config: %v", err))
	}
	effective.Lifecycle.WaitAdmissionAttempts = new(waitAdmissionAttempts)
	snapshot, digest, err := looppkg.BuildExecutedDefinitionSnapshot(resolved, effective)
	if err != nil {
		panic(fmt.Sprintf("build wait escalation test executed definition snapshot: %v", err))
	}
	return looppkg.Run{
		ID: looppkg.RunID(id), WorkspaceID: "ws-1", LoopName: "delivery", Status: looppkg.StatusRunning,
		ReattemptStrategy: looppkg.ReattemptFailedOnly, CreatedAt: at, StartedAt: at,
		LastProgressAt: at, DefinitionVersion: resolved.DefinitionVersion,
		DefinitionDigest: digest, DefinitionSnapshot: snapshot, ActiveHumanCriteria: []byte(`[]`),
		StartMetadata: map[string]any{}, IterationCap: 7, BudgetOnExceeded: dsl.BudgetExceededHalt,
		Origin: &looppkg.RunOrigin{Kind: looppkg.RunOriginCatalog}, Inputs: map[string]any{"request": "42"},
	}
}

func approvalEscalationTestLoopRun(id string, at time.Time) looppkg.Run {
	source := `{"apiVersion":"compozy.loop/v1","kind":"Loop","meta":{"name":"delivery","version":1},"contract":{"goal":"test","definition_of_done":"done","iteration_cap":7,"no_progress":{"window":1},"budget":{"tokens":100,"wall_clock_sec":3600}},"graph":{"nodes":[{"id":"approval","class":"control","kind":"gate","criteria":[{"id":"check","type":"command","check":"true","expect":"exit_zero"}],"verdict_policy":"fixed_passes","expires":{"after":"1m","escalate":[{"emit":{"kind":"approval_reminder","payload":{"reminder":true}}}],"route":"timed_out"}},{"id":"normal","class":"action","kind":"transform","params":{"map":{"path":{"value":"normal"}}}},{"id":"timed_out","class":"action","kind":"transform","params":{"map":{"path":{"value":"timed_out"}}}}],"edges":[{"from":"approval","to":"normal"},{"from":"approval","to":"timed_out"}]}}`
	definition, err := dsl.Parse([]byte(source))
	if err != nil {
		panic(fmt.Sprintf("parse approval escalation test Loop definition: %v", err))
	}
	resolved, err := looppkg.NewCompiler().Compile(definition)
	if err != nil {
		panic(fmt.Sprintf("compile approval escalation test Loop definition: %v", err))
	}
	effective, err := looppkg.ResolveEffectiveConfig(
		resolved,
		looppkg.DefaultLoopDefaults(),
		nil,
		looppkg.LoopConfig{},
	)
	if err != nil {
		panic(fmt.Sprintf("resolve approval escalation test Loop config: %v", err))
	}
	snapshot, digest, err := looppkg.BuildExecutedDefinitionSnapshot(resolved, effective)
	if err != nil {
		panic(fmt.Sprintf("build approval escalation executed definition snapshot: %v", err))
	}
	return looppkg.Run{
		ID: looppkg.RunID(id), WorkspaceID: "ws-1", LoopName: "delivery", Status: looppkg.StatusRunning,
		ReattemptStrategy: looppkg.ReattemptFailedOnly, CreatedAt: at, StartedAt: at,
		LastProgressAt: at, DefinitionVersion: resolved.DefinitionVersion,
		DefinitionDigest: digest, DefinitionSnapshot: snapshot, ActiveHumanCriteria: []byte(`[]`),
		StartMetadata: map[string]any{}, IterationCap: 7, BudgetOnExceeded: dsl.BudgetExceededHalt,
		Origin: &looppkg.RunOrigin{Kind: looppkg.RunOriginCatalog}, Inputs: map[string]any{"request": "42"},
	}
}

func seedApprovalEscalationFixture(
	t *testing.T,
	globalDB *GlobalDB,
	run looppkg.Run,
	at time.Time,
) {
	t.Helper()
	ctx := testutil.Context(t)
	completeInitialCoordinatorForParkedFixture(t, globalDB, run, at)
	if _, err := globalDB.db.ExecContext(ctx, `UPDATE loop_runs SET status = 'needs-approval',
		active_gate_id = 'approval', active_human_criteria_json = '[{"id":"operator","type":"human","outcome":"awaiting_approval"}]'
		WHERE id = ?`, run.ID); err != nil {
		t.Fatalf("activate approval wait run error = %v", err)
	}
	nextEscalationAt := at.Add(time.Minute)
	if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_generation_outputs (
		loop_run_id, generation, node_id, item_index, status, output_ref, attempt, epoch
	) VALUES (?, 1, 'approval', 0, 'succeeded', '{"outcome":"awaiting_approval"}', 1, 3),
		(?, 1, 'normal', 0, 'pending', '', 1, 0),
		(?, 1, 'timed_out', 0, 'pending', '', 1, 0)`, run.ID, run.ID, run.ID); err != nil {
		t.Fatalf("insert approval wait outputs error = %v", err)
	}
	if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_node_waits (
		loop_run_id, generation, node_id, item_index, kind, next_escalation_at,
		claim_state, issued_epoch, created_at
	) VALUES (?, 1, 'approval', 0, 'approval_escalation', ?, 'waiting', 3, ?)`,
		run.ID, nextEscalationAt, at.UTC()); err != nil {
		t.Fatalf("insert approval wait row error = %v", err)
	}
	if err := appendLoopRunEventWithExecutor(ctx, globalDB.db, run.ID, run.WorkspaceID,
		loopRunEventNodeWaitStarted, map[string]any{
			loopRunEventPayloadKeyGeneration: 1, loopRunEventPayloadKeyNodeID: "approval",
			loopRunEventPayloadKeyItemIndex: 0, loopRunEventPayloadKeyIssuedEpoch: 3,
			"wait_kind": "approval_escalation",
		}, at.UTC()); err != nil {
		t.Fatalf("append approval wait started event error = %v", err)
	}
}

func seedWaitEscalationFixture(
	t *testing.T,
	globalDB *GlobalDB,
	run looppkg.Run,
	at time.Time,
	withRoute bool,
) {
	t.Helper()
	ctx := testutil.Context(t)
	completeInitialCoordinatorForParkedFixture(t, globalDB, run, at)
	resumeAt := at.Add(24 * time.Hour)
	nextEscalationAt := at.Add(time.Minute)
	expect := json.RawMessage(`{"type":"object","required":["approved"],"properties":{"approved":{"type":"boolean"}}}`)
	seedLoopWaitCellForTest(
		t, globalDB, run, "wait_for_ack", 0, "timer", 3, expect,
		&resumeAt, &nextEscalationAt, at,
	)
	if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_generation_outputs (
		loop_run_id, generation, node_id, item_index, status, attempt, epoch
	) VALUES (?, 1, 'normal', 0, 'pending', 1, 0)`, run.ID); err != nil {
		t.Fatalf("insert normal wait branch output error = %v", err)
	}
	if withRoute {
		if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_generation_outputs (
			loop_run_id, generation, node_id, item_index, status, attempt, epoch
		) VALUES (?, 1, 'timeout', 0, 'pending', 1, 0)`, run.ID); err != nil {
			t.Fatalf("insert timeout wait branch output error = %v", err)
		}
	}
}

func seedLoopWaitCellForTest(
	t *testing.T,
	globalDB *GlobalDB,
	run looppkg.Run,
	nodeID string,
	itemIndex int,
	kind string,
	epoch int64,
	expect json.RawMessage,
	resumeAt *time.Time,
	nextEscalationAt *time.Time,
	createdAt time.Time,
) {
	t.Helper()
	ctx := testutil.Context(t)
	if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_generation_outputs (
		loop_run_id, generation, node_id, item_index, status, attempt, first_scheduled_at, epoch
	) VALUES (?, 1, ?, ?, 'waiting', 1, ?, ?)`, run.ID, nodeID, itemIndex, createdAt.UTC(), epoch); err != nil {
		t.Fatalf("insert wait output %s/%d error = %v", nodeID, itemIndex, err)
	}
	var expectValue any
	if len(expect) > 0 {
		expectValue = string(expect)
	}
	if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_node_waits (
		loop_run_id, generation, node_id, item_index, kind, resume_at, next_escalation_at,
		claim_state, expect_json, issued_epoch, created_at
	) VALUES (?, 1, ?, ?, ?, ?, ?, 'waiting', ?, ?, ?)`, run.ID, nodeID, itemIndex,
		kind, resumeAt, nextEscalationAt, expectValue, epoch, createdAt.UTC()); err != nil {
		t.Fatalf("insert wait row %s/%d error = %v", nodeID, itemIndex, err)
	}
	if err := appendLoopRunEventWithExecutor(ctx, globalDB.db, run.ID, run.WorkspaceID,
		loopRunEventNodeWaitStarted, map[string]any{
			loopRunEventPayloadKeyGeneration:  1,
			loopRunEventPayloadKeyNodeID:      nodeID,
			loopRunEventPayloadKeyItemIndex:   itemIndex,
			loopRunEventPayloadKeyIssuedEpoch: epoch,
			"wait_kind":                       kind,
		}, createdAt.UTC()); err != nil {
		t.Fatalf("append wait started event %s/%d error = %v", nodeID, itemIndex, err)
	}
}

func seedLoopRequestForTest(
	t *testing.T,
	globalDB *GlobalDB,
	run looppkg.Run,
	nodeID string,
	itemIndex int,
	schema json.RawMessage,
	expiresAt *time.Time,
	openedAt time.Time,
) {
	t.Helper()
	ctx := testutil.Context(t)
	contextPayload := json.RawMessage(fmt.Sprintf(`{"full":"lane-%d"}`, itemIndex))
	contextRef := looppkg.OutputRefForPayload(contextPayload)
	if err := storepkg.UpsertLoopOutputBlob(ctx, globalDB.db, contextRef, contextPayload, openedAt); err != nil {
		t.Fatalf("UpsertLoopOutputBlob(request context) error = %v", err)
	}
	_, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_requests (
		workspace_id, loop_run_id, generation, node_id, item_index, kind, state,
		prompt, context_preview_json, context_ref, answer_schema_json, respond_schema_json,
		decisions_json, opened_at, expires_at
	) VALUES (?, ?, 1, ?, ?, 'ask', 'pending', ?, ?, ?, ?, ?, '["respond"]', ?, ?)`,
		run.WorkspaceID, run.ID, nodeID, itemIndex, "Choose an environment",
		fmt.Sprintf(`{"lane":%d}`, itemIndex), contextRef, string(schema), string(schema),
		openedAt.UTC(), expiresAt)
	if err != nil {
		t.Fatalf("insert Loop request %s/%d error = %v", nodeID, itemIndex, err)
	}
	if err := appendLoopRunEventWithExecutor(ctx, globalDB.db, run.ID, run.WorkspaceID,
		loopRunEventRequestOpened, map[string]any{
			loopRunEventPayloadKeyGeneration: 1,
			loopRunEventPayloadKeyNodeID:     nodeID,
			loopRunEventPayloadKeyItemIndex:  itemIndex,
		}, openedAt.UTC()); err != nil {
		t.Fatalf("append request_opened event %s/%d error = %v", nodeID, itemIndex, err)
	}
}

func completeInitialCoordinatorForParkedFixture(
	t *testing.T,
	globalDB *GlobalDB,
	run looppkg.Run,
	at time.Time,
) {
	t.Helper()
	ctx := testutil.Context(t)
	claim := claimCoordinatorRunForTest(ctx, t, globalDB, run.ID, "parked-fixture", at.Add(time.Millisecond))
	if _, err := globalDB.CompleteCoordinatorAndEnqueueNext(
		ctx,
		taskpkg.CoordinatorCompletion{
			RunID: claim.Run.ID, ClaimToken: claim.ClaimToken,
			Actor: coordinatorActorContextForTest(), Now: at.Add(2 * time.Millisecond),
			Plan: taskpkg.CoordinatorCompletionPlan{Yield: true, Snapshot: taskpkg.GenerationSnapshot{
				LoopRunID: string(run.ID), Generation: 1,
				Payload: looppkg.GenerationSnapshotPayload{},
			}},
		},
		looppkg.NewStoreFinalizer(),
	); err != nil {
		t.Fatalf("CompleteCoordinatorAndEnqueueNext(parked fixture) error = %v", err)
	}
}

func seedQuarantinedLoopNodeForTest(
	t *testing.T,
	globalDB *GlobalDB,
	at time.Time,
	status looppkg.Status,
) looppkg.Run {
	t.Helper()
	ctx := testutil.Context(t)
	seed := testLoopRun("looprun-node-requeue-"+string(status), at, looppkg.StatusRunning)
	created, err := globalDB.CreateLoopRunForStart(ctx, seed, dsl.ConcurrencyAllow)
	if err != nil {
		t.Fatalf("CreateLoopRunForStart() error = %v", err)
	}
	claim := claimCoordinatorRunForTest(
		ctx,
		t,
		globalDB,
		created.ID,
		"seed-node-requeue-"+string(status),
		at.Add(time.Millisecond),
	)
	if _, err := globalDB.CompleteCoordinatorAndEnqueueNext(
		ctx,
		taskpkg.CoordinatorCompletion{
			RunID: claim.Run.ID, ClaimToken: claim.ClaimToken,
			Actor: coordinatorActorContextForTest(), Now: at.Add(2 * time.Millisecond),
			Plan: taskpkg.CoordinatorCompletionPlan{Yield: true, Snapshot: taskpkg.GenerationSnapshot{
				LoopRunID: string(created.ID), Generation: 1,
				Payload: looppkg.GenerationSnapshotPayload{},
			}},
		},
		looppkg.NewStoreFinalizer(),
	); err != nil {
		t.Fatalf("CompleteCoordinatorAndEnqueueNext(seed) error = %v", err)
	}
	if err := insertLoopGenerationWithExecutor(ctx, globalDB.db, created.ID, looppkg.GenerationIntent{
		Generation: 2, ParentGeneration: 1, Origin: looppkg.OriginReattempt,
	}, at.Add(time.Second)); err != nil {
		t.Fatalf("insert generation 2 error = %v", err)
	}
	entryJSON, err := json.Marshal(looppkg.QuarantineEntry{
		NodeID:   "finish",
		InputRef: "loop-run:" + string(created.ID) + ":node:finish:input",
		Target:   "transform:finish",
		Episodes: []looppkg.QuarantineEpisode{{
			Generation: 2, QuarantinedAt: at,
			Attempts: []looppkg.NodeAttempt{{
				LoopRunID: created.ID, Generation: 2, NodeID: "finish", Attempt: 1,
				Disposition: looppkg.AttemptQuarantined, StartedAt: at,
			}},
		}},
	})
	if err != nil {
		t.Fatalf("json.Marshal(quarantine entry) error = %v", err)
	}
	if _, err := globalDB.db.ExecContext(
		ctx,
		`UPDATE loop_runs SET status = ?, generation = 2 WHERE id = ?`,
		status,
		created.ID,
	); err != nil {
		t.Fatalf("update quarantined loop state error = %v", err)
	}
	if _, err := globalDB.db.ExecContext(
		ctx,
		`INSERT INTO loop_generation_outputs
			(loop_run_id, generation, node_id, status, attempt, first_scheduled_at, epoch)
		 VALUES (?, 2, 'finish', 'quarantined', 1, ?, 9)`,
		created.ID,
		at.Add(-time.Minute),
	); err != nil {
		t.Fatalf("insert quarantined generation output error = %v", err)
	}
	if _, err := globalDB.db.ExecContext(
		ctx,
		`INSERT INTO loop_node_controls
			(loop_run_id, node_id, quarantined, quarantine_entry_json, quarantined_at, revision, updated_at)
		 VALUES (?, 'finish', 1, ?, ?, 4, ?)`,
		created.ID,
		string(entryJSON),
		at,
		at,
	); err != nil {
		t.Fatalf("insert quarantined node control error = %v", err)
	}
	if _, err := globalDB.db.ExecContext(
		ctx,
		`INSERT INTO loop_node_controls
			(loop_run_id, node_id, attention_flag, attention_reason, attention_producer_node_id, revision, updated_at)
		 VALUES (?, 'other_consumer', 'dependency_quarantined',
			'node other_consumer requires parked producer other_producer', 'other_producer', 2, ?)`,
		created.ID,
		at,
	); err != nil {
		t.Fatalf("insert unrelated dependency attention error = %v", err)
	}
	if _, err := globalDB.db.ExecContext(
		ctx,
		`INSERT INTO loop_node_controls
			(loop_run_id, node_id, attention_flag, attention_reason, attention_producer_node_id, revision, updated_at)
		 VALUES (?, 'finish_consumer', 'dependency_quarantined',
			'node finish_consumer requires parked producer finish', 'finish', 3, ?)`,
		created.ID,
		at,
	); err != nil {
		t.Fatalf("insert finish dependency attention error = %v", err)
	}
	cellTask := taskRecordForTest(looppkg.NodeCellTaskID(created.ID, 2, "finish", 0))
	cellTask.WorkspaceID = string(created.WorkspaceID)
	cellTask.Scope = taskpkg.ScopeWorkspace
	if err := globalDB.CreateTask(ctx, cellTask); err != nil {
		t.Fatalf("CreateTask(quarantined cell) error = %v", err)
	}
	if _, err := globalDB.db.ExecContext(
		ctx,
		`UPDATE tasks SET status = 'needs_attention',
			needs_attention_reason = 'loop node finish is quarantined',
			needs_attention_at = ?, needs_attention_by_kind = 'daemon',
			needs_attention_by_ref = 'loop-coordinator' WHERE id = ?`,
		storepkg.FormatTimestamp(at),
		cellTask.ID,
	); err != nil {
		t.Fatalf("mark quarantined cell task error = %v", err)
	}
	created.Status = status
	created.Generation = 2
	return created
}

func seedLiveLoopLivenessCellForTest(
	t *testing.T,
	globalDB *GlobalDB,
	at time.Time,
) (looppkg.Run, string) {
	t.Helper()
	ctx := testutil.Context(t)
	caseID, err := storepkg.NewID("case")
	if err != nil {
		t.Fatalf("store.NewID(liveness case) error = %v", err)
	}
	run, err := globalDB.CreateLoopRunForStart(
		ctx,
		testLoopRun("looprun-liveness-"+caseID, at, looppkg.StatusRunning),
		dsl.ConcurrencyAllow,
	)
	if err != nil {
		t.Fatalf("CreateLoopRunForStart(liveness) error = %v", err)
	}
	taskRecord := workspaceTaskRecordForTest("task-loop-liveness-"+caseID, string(run.WorkspaceID))
	taskRecord.MaxAttempts = taskpkg.MaxTaskMaxAttempts
	if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
		t.Fatalf("CreateTask(liveness) error = %v", err)
	}
	metadata, err := json.Marshal(map[string]any{
		"generation": 1, "node_id": "work", "item_index": 0, "attempt": 1, "epoch": 4,
		"session_handle": "main",
	})
	if err != nil {
		t.Fatalf("json.Marshal(liveness metadata) error = %v", err)
	}
	taskRun := taskRunForTest("run-loop-liveness-"+caseID, taskRecord.ID)
	taskRun.WorkspaceID = string(run.WorkspaceID)
	taskRun.RunKind = taskpkg.RunKindWorker
	taskRun.LoopRunID = string(run.ID)
	taskRun.DesignationGroupID = "designation-" + caseID
	taskRun.RunWorktreeState = &taskpkg.RunWorktreeState{
		ResolvedWorktreeMode: taskpkg.WorktreeModeNone,
	}
	taskRun.RequiredCapabilities = []string{"go"}
	taskRun.PreferredCapabilities = []string{"review"}
	taskRun.Metadata = metadata
	if err := globalDB.CreateTaskRun(ctx, taskRun); err != nil {
		t.Fatalf("CreateTaskRun(liveness) error = %v", err)
	}
	if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_generation_outputs (
		loop_run_id, generation, node_id, item_index, status, task_run_id, attempt, epoch
	) VALUES (?, 1, 'work', 0, 'running', ?, 1, 4)`, run.ID, taskRun.ID); err != nil {
		t.Fatalf("insert liveness output error = %v", err)
	}
	return run, taskRun.ID
}

func nodeCancellationMutationForTest(
	run looppkg.Run,
	kind looppkg.RunCancelKind,
	at time.Time,
) looppkg.CancellationMutation {
	return looppkg.CancellationMutation{
		WorkspaceID: run.WorkspaceID, RunID: run.ID, NodeID: "work", Kind: kind,
		Reason: "operator request", Actor: operatorActorContextForTest("operator:node-cancel"), RequestedAt: at,
	}
}

func seedLoopCancellationBindingForTest(
	t *testing.T,
	globalDB *GlobalDB,
	runID string,
	workspaceID string,
	handle string,
	bindingEpoch int64,
	sessionID string,
	at time.Time,
) {
	t.Helper()
	ctx := testutil.Context(t)
	if err := globalDB.RegisterSession(ctx, storepkg.SessionInfo{
		ID: sessionID, AgentName: "codex", RuntimeStatus: storepkg.SessionRuntimeUnbound,
		WorkspaceID: workspaceID, State: "active", CreatedAt: at, UpdatedAt: at,
	}); err != nil {
		t.Fatalf("RegisterSession(cancellation) error = %v", err)
	}
	if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_session_bindings (
		loop_run_id, handle, binding_epoch, binding_attempt_id, session_id, workspace_id,
		creation_profile_ref, policy_spec_digest, creation_digest, ownership, state,
		created_at, activated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'run-owned', 'active', ?, ?)`,
		runID, handle, bindingEpoch, "binding-attempt:"+runID+":"+handle, sessionID, workspaceID,
		"profile:"+runID, "policy:"+runID, "creation:"+runID, at, at,
	); err != nil {
		t.Fatalf("insert cancellation binding error = %v", err)
	}
}

func deadNodeResumeRequestForTest(
	run looppkg.Run,
	taskRunID string,
	epoch int64,
	at time.Time,
) looppkg.DeadNodeResumeRequest {
	return looppkg.DeadNodeResumeRequest{
		WorkspaceID: run.WorkspaceID, RunID: run.ID, Generation: 1, NodeID: "work", ItemIndex: 0,
		TaskRunID: taskRunID, SourceSessionID: "session-dead-work", ExpectedEpoch: epoch, DeathStreakLimit: 3,
		Checkpoint: &looppkg.DeathResumeCheckpoint{
			SessionID: "session-dead-work", EventStartSeq: 12, EventEndSeq: 14,
			Partials: []looppkg.DeathResumePartial{{Sequence: 14, Type: "agent_message", Text: "partial progress"}},
		},
		Cause: "confirmed ACP process exit", Actor: operatorActorContextForTest("daemon:liveness"), ConfirmedAt: at,
	}
}

func assertDeathResumeCountsForTest(
	t *testing.T,
	globalDB *GlobalDB,
	runID looppkg.RunID,
	taskID string,
	wantRuns int,
	wantLedger int,
) {
	t.Helper()
	ctx := testutil.Context(t)
	if taskID == "" {
		if err := globalDB.db.QueryRowContext(ctx, `SELECT task_id FROM task_runs
			WHERE loop_run_id = ? ORDER BY queued_at LIMIT 1`, runID).Scan(&taskID); err != nil {
			t.Fatalf("read death-resume task id error = %v", err)
		}
	}
	var runCount int
	if err := globalDB.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_runs WHERE task_id = ?`, taskID).
		Scan(&runCount); err != nil {
		t.Fatalf("count death-resume task runs error = %v", err)
	}
	var ledgerCount int
	if err := globalDB.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM loop_node_attempts
		WHERE loop_run_id = ? AND node_id = 'work'`, runID).Scan(&ledgerCount); err != nil {
		t.Fatalf("count death-resume ledger error = %v", err)
	}
	if runCount != wantRuns || ledgerCount != wantLedger {
		t.Fatalf("death-resume counts = runs:%d ledger:%d, want runs:%d ledger:%d",
			runCount, ledgerCount, wantRuns, wantLedger)
	}
}

func countLoopEventKindForTest(events []looppkg.RunEvent, kind string) int {
	count := 0
	for _, event := range events {
		if event.Kind == kind {
			count++
		}
	}
	return count
}

func assertExpiredWaitResumeEventPayloadForTest(
	t *testing.T,
	globalDB *GlobalDB,
	run looppkg.Run,
	route string,
	waitKind string,
) {
	t.Helper()
	events, err := globalDB.ListLoopRunEvents(testutil.Context(t), looppkg.RunEventQuery{
		WorkspaceID: run.WorkspaceID,
		RunID:       run.ID,
		Limit:       100,
	})
	if err != nil {
		t.Fatalf("ListLoopRunEvents(expired wait) error = %v", err)
	}
	for _, event := range events {
		if event.Kind != loopRunEventNodeWaitResumed {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Fatalf("json.Unmarshal(expired wait event) error = %v", err)
		}
		content, ok := payload[eventSummaryContentPayloadKey].(map[string]any)
		if !ok || content[loopRunEventPayloadKeyExpired] != true ||
			content[loopRunEventPayloadKeyRoute] != route ||
			payload[loopRunEventPayloadKeyActorKind] != loopWaitClaimedByExpiry ||
			payload[loopRunEventPayloadKeyWaitKind] != waitKind {
			t.Fatalf("expired wait event payload = %#v, want shared expiry payload for %q", payload, waitKind)
		}
		return
	}
	t.Fatalf("expired wait events = %#v, want node_wait_resumed", events)
}

func countLoopRunsByStatus(
	ctx context.Context,
	t *testing.T,
	globalDB *GlobalDB,
	workspaceID looppkg.WorkspaceID,
	loopName string,
	status looppkg.Status,
) int {
	t.Helper()
	var count int
	if err := globalDB.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM loop_runs WHERE workspace_id = ? AND loop_name = ? AND status = ?`,
		string(workspaceID),
		loopName,
		string(status),
	).Scan(&count); err != nil {
		t.Fatalf("count loop_runs by status error = %v", err)
	}
	return count
}

func countLoopRunEvents(
	ctx context.Context,
	t *testing.T,
	globalDB *GlobalDB,
	runID looppkg.RunID,
) int {
	t.Helper()
	var count int
	if err := globalDB.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM loop_run_events WHERE loop_run_id = ?`,
		string(runID),
	).Scan(&count); err != nil {
		t.Fatalf("count loop_run_events error = %v", err)
	}
	return count
}

func countCoordinatorTaskRunsForLoop(
	ctx context.Context,
	t *testing.T,
	globalDB *GlobalDB,
	runID looppkg.RunID,
) int {
	t.Helper()
	var count int
	if err := globalDB.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM task_runs WHERE loop_run_id = ? AND run_kind = 'coordinator'`,
		string(runID),
	).Scan(&count); err != nil {
		t.Fatalf("count coordinator task runs error = %v", err)
	}
	return count
}
