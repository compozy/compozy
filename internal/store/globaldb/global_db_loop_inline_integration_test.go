//go:build integration

package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
)

func TestGlobalDBInlineGoalStartAndReplaceIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should serialize same-session starts while allowing different sessions", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-inline-race")
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC)
		origins := []looppkg.RunOrigin{
			inlineGoalOriginForTest("session-inline-race-a"),
			inlineGoalOriginForTest("session-inline-race-b"),
			inlineGoalOriginForTest("session-inline-race-shared"),
		}
		for _, origin := range origins {
			registerGoalSessionIdentityForTest(
				t,
				globalDB,
				goalSessionInfoForTest(origin.SessionID, "ws-inline-race", now),
				store.SessionCreationIdentity{
					CreationProfileRef: origin.CreationProfileRef,
					PolicySpecDigest:   origin.PolicySpecDigest,
					CreationDigest:     origin.CreationDigest,
				},
			)
		}

		distinctRuns := []looppkg.Run{
			inlineGoalRunForTest(t, "run-inline-race-a", "ws-inline-race", origins[0], now),
			inlineGoalRunForTest(t, "run-inline-race-b", "ws-inline-race", origins[1], now),
		}
		distinctResults := runInlineStartsConcurrently(ctx, globalDB, distinctRuns)
		for _, result := range distinctResults {
			if result.err != nil {
				t.Fatalf("distinct-session CreateInlineLoopRunForStart() error = %v", result.err)
			}
		}

		sharedRuns := []looppkg.Run{
			inlineGoalRunForTest(t, "run-inline-race-shared-a", "ws-inline-race", origins[2], now.Add(time.Second)),
			inlineGoalRunForTest(t, "run-inline-race-shared-b", "ws-inline-race", origins[2], now.Add(time.Second)),
		}
		sharedResults := runInlineStartsConcurrently(ctx, globalDB, sharedRuns)
		successes := 0
		conflicts := 0
		for _, result := range sharedResults {
			if result.err == nil {
				successes++
				continue
			}
			var reason *looppkg.ReasonError
			if errors.As(result.err, &reason) && reason.Code == looppkg.ReasonCodeGoalReplaceRequired {
				conflicts++
				continue
			}
			t.Fatalf("same-session CreateInlineLoopRunForStart() error = %v", result.err)
		}
		if successes != 1 || conflicts != 1 {
			t.Fatalf("same-session race successes/conflicts = %d/%d, want 1/1", successes, conflicts)
		}
	})

	t.Run("Should give one concurrent replacement winner and recover it after reopen", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-inline-replace-race")
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 11, 0, 30, 0, 0, time.UTC)
		origin := inlineGoalOriginForTest("session-inline-replace-race")
		registerGoalSessionIdentityForTest(
			t,
			globalDB,
			goalSessionInfoForTest(origin.SessionID, "ws-inline-replace-race", now),
			store.SessionCreationIdentity{
				CreationProfileRef: origin.CreationProfileRef,
				PolicySpecDigest:   origin.PolicySpecDigest,
				CreationDigest:     origin.CreationDigest,
			},
		)
		first := inlineGoalRunForTest(t, "run-inline-replace-race-old", "ws-inline-replace-race", origin, now)
		if _, err := globalDB.CreateInlineLoopRunForStart(ctx, first); err != nil {
			t.Fatalf("CreateInlineLoopRunForStart(old) error = %v", err)
		}
		actor, err := taskpkg.DeriveHumanActorContext("operator", taskpkg.OriginKindHTTP, "goal.replace")
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		replacementA := inlineGoalRunForTest(
			t, "run-inline-replace-race-a", "ws-inline-replace-race", origin, now.Add(time.Second),
		)
		replacementB := inlineGoalRunForTest(
			t, "run-inline-replace-race-b", "ws-inline-replace-race", origin, now.Add(time.Second),
		)
		requests := []looppkg.InlineReplaceStoreRequest{
			{
				ExpectedRunID: first.ID,
				Run:           &replacementA,
				Actor:         actor, ReplacedAt: now.Add(time.Second),
			},
			{
				ExpectedRunID: first.ID,
				Run:           &replacementB,
				Actor:         actor, ReplacedAt: now.Add(time.Second),
			},
		}
		results := runInlineReplacementsConcurrently(ctx, globalDB, requests)
		winnerID := looppkg.RunID("")
		stale := 0
		for _, result := range results {
			if result.err == nil {
				winnerID = result.result.Run.ID
				continue
			}
			var reason *looppkg.ReasonError
			if errors.As(result.err, &reason) && reason.Code == looppkg.ReasonCodeGoalReplaceStale {
				stale++
				continue
			}
			t.Fatalf("ReplaceInlineLoopRun(concurrent) error = %v", result.err)
		}
		if winnerID == "" || stale != 1 {
			t.Fatalf("replacement winner/stale = %q/%d, want one winner and one stale result", winnerID, stale)
		}

		path := globalDB.path
		if err := globalDB.Close(ctx); err != nil {
			t.Fatalf("Close(before reopen) error = %v", err)
		}
		reopened, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(reopen) error = %v", err)
		}
		t.Cleanup(func() {
			if err := reopened.Close(testutil.Context(t)); err != nil {
				t.Errorf("Close(reopened) error = %v", err)
			}
		})
		projection, err := reopened.GetSessionGoalProjection(ctx, "ws-inline-replace-race", origin.SessionID)
		if err != nil {
			t.Fatalf("GetSessionGoalProjection(reopen) error = %v", err)
		}
		if !projection.Found || projection.Cleared || projection.RunID != winnerID {
			t.Fatalf("reopened replacement projection = %#v, want winner %q", projection, winnerID)
		}
		old, err := reopened.GetLoopRun(ctx, "ws-inline-replace-race", first.ID)
		if err != nil {
			t.Fatalf("GetLoopRun(old after reopen) error = %v", err)
		}
		if old.Status != looppkg.StatusFailed {
			t.Fatalf("reopened old Run status = %q, want failed", old.Status)
		}
	})

	t.Run("Should map the active-session unique index and roll its injected conflict back", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-inline-index")
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 11, 1, 0, 0, 0, time.UTC)
		origin := inlineGoalOriginForTest("session-inline-index")
		registerGoalSessionIdentityForTest(
			t,
			globalDB,
			goalSessionInfoForTest(origin.SessionID, "ws-inline-index", now),
			store.SessionCreationIdentity{
				CreationProfileRef: origin.CreationProfileRef,
				PolicySpecDigest:   origin.PolicySpecDigest,
				CreationDigest:     origin.CreationDigest,
			},
		)
		seed := inlineGoalRunForTest(t, "run-inline-index-seed", "ws-inline-index", origin, now)
		if _, err := globalDB.CreateInlineLoopRunForStart(ctx, seed); err != nil {
			t.Fatalf("CreateInlineLoopRunForStart(seed) error = %v", err)
		}
		if err := globalDB.CompareAndSwapLoopRunStatus(
			ctx,
			seed.ID,
			looppkg.StatusRunning,
			looppkg.StatusDone,
			looppkg.TransitionCauseContract,
			now.Add(time.Second),
		); err != nil {
			t.Fatalf("CompareAndSwapLoopRunStatus(seed) error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(ctx, `
			CREATE TRIGGER inject_inline_goal_unique_conflict
			BEFORE INSERT ON loop_runs
			WHEN NEW.id = 'run-inline-index-conflict'
			BEGIN
				UPDATE loop_runs SET status = 'running' WHERE id = 'run-inline-index-seed';
			END
		`); err != nil {
			t.Fatalf("create injected uniqueness trigger error = %v", err)
		}
		candidate := inlineGoalRunForTest(
			t,
			"run-inline-index-conflict",
			"ws-inline-index",
			origin,
			now.Add(2*time.Second),
		)
		_, err := globalDB.CreateInlineLoopRunForStart(ctx, candidate)
		requireInlineGoalReason(t, err, looppkg.ReasonCodeGoalReplaceRequired)
		persistedSeed, err := globalDB.GetLoopRun(ctx, "ws-inline-index", seed.ID)
		if err != nil {
			t.Fatalf("GetLoopRun(seed after conflict) error = %v", err)
		}
		if persistedSeed.Status != looppkg.StatusDone {
			t.Fatalf("seed status after rolled-back trigger = %q, want done", persistedSeed.Status)
		}
		if _, err := globalDB.GetLoopRun(
			ctx,
			"ws-inline-index",
			candidate.ID,
		); !errors.Is(
			err,
			looppkg.ErrRunNotFound,
		) {
			t.Fatalf("GetLoopRun(candidate) error = %v, want ErrRunNotFound", err)
		}
	})

	t.Run("Should roll back the old Run when replacement persistence fails before commit", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-inline-replace-rollback")
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 11, 1, 30, 0, 0, time.UTC)
		origin := inlineGoalOriginForTest("session-inline-replace-rollback")
		registerGoalSessionIdentityForTest(
			t,
			globalDB,
			goalSessionInfoForTest(origin.SessionID, "ws-inline-replace-rollback", now),
			store.SessionCreationIdentity{
				CreationProfileRef: origin.CreationProfileRef,
				PolicySpecDigest:   origin.PolicySpecDigest,
				CreationDigest:     origin.CreationDigest,
			},
		)
		old := inlineGoalRunForTest(t, "run-inline-replace-rollback-old", "ws-inline-replace-rollback", origin, now)
		if _, err := globalDB.CreateInlineLoopRunForStart(ctx, old); err != nil {
			t.Fatalf("CreateInlineLoopRunForStart(old) error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(ctx, `
			CREATE TRIGGER inject_inline_goal_replace_outbox_failure
			BEFORE INSERT ON loop_goal_session_outbox
			WHEN NEW.cause = 'replace'
			BEGIN
				SELECT RAISE(ABORT, 'injected replacement outbox failure');
			END
		`); err != nil {
			t.Fatalf("create injected replacement trigger error = %v", err)
		}
		actor, err := taskpkg.DeriveHumanActorContext("operator", taskpkg.OriginKindHTTP, "goal.replace")
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		candidate := inlineGoalRunForTest(
			t,
			"run-inline-replace-rollback-new",
			"ws-inline-replace-rollback",
			origin,
			now.Add(time.Second),
		)
		if _, err := globalDB.ReplaceInlineLoopRun(ctx, looppkg.InlineReplaceStoreRequest{
			ExpectedRunID: old.ID,
			Run:           &candidate,
			Actor:         actor,
			ReplacedAt:    now.Add(time.Second),
		}); err == nil {
			t.Fatal("ReplaceInlineLoopRun(injected failure) error = nil")
		}
		persistedOld, err := globalDB.GetLoopRun(ctx, "ws-inline-replace-rollback", old.ID)
		if err != nil {
			t.Fatalf("GetLoopRun(old after rollback) error = %v", err)
		}
		if persistedOld.Status != looppkg.StatusRunning {
			t.Fatalf("old Run status after rollback = %q, want running", persistedOld.Status)
		}
		if _, err := globalDB.GetLoopRun(
			ctx,
			"ws-inline-replace-rollback",
			candidate.ID,
		); !errors.Is(
			err,
			looppkg.ErrRunNotFound,
		) {
			t.Fatalf("GetLoopRun(candidate after rollback) error = %v, want ErrRunNotFound", err)
		}
	})

	t.Run("Should enforce one active Goal per session and atomically replace the expected Run", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-inline")
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 10, 23, 0, 0, 0, time.UTC)
		origin := inlineGoalOriginForTest("session-inline")
		registerGoalSessionIdentityForTest(
			t,
			globalDB,
			goalSessionInfoForTest(origin.SessionID, "ws-inline", now),
			store.SessionCreationIdentity{
				CreationProfileRef: origin.CreationProfileRef,
				PolicySpecDigest:   origin.PolicySpecDigest,
				CreationDigest:     origin.CreationDigest,
			},
		)

		first := inlineGoalRunForTest(t, "run-inline-1", "ws-inline", origin, now)
		created, err := globalDB.CreateInlineLoopRunForStart(ctx, first)
		if err != nil {
			t.Fatalf("CreateInlineLoopRunForStart() error = %v", err)
		}
		if created.Origin == nil || *created.Origin != origin || created.Status != looppkg.StatusRunning {
			t.Fatalf("created inline Run = %#v", created)
		}
		persisted, err := globalDB.GetLoopRun(ctx, "ws-inline", created.ID)
		if err != nil {
			t.Fatalf("GetLoopRun(created) error = %v", err)
		}
		if persisted.Origin == nil || *persisted.Origin != origin {
			t.Fatalf("persisted origin = %#v, want %#v", persisted.Origin, origin)
		}

		conflict := inlineGoalRunForTest(t, "run-inline-conflict", "ws-inline", origin, now.Add(time.Second))
		_, err = globalDB.CreateInlineLoopRunForStart(ctx, conflict)
		requireInlineGoalReason(t, err, looppkg.ReasonCodeGoalReplaceRequired)

		actor, err := taskpkg.DeriveHumanActorContext("operator", taskpkg.OriginKindHTTP, "goal.replace")
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		replacement := inlineGoalRunForTest(t, "run-inline-2", "ws-inline", origin, now.Add(2*time.Second))
		_, err = globalDB.ReplaceInlineLoopRun(ctx, looppkg.InlineReplaceStoreRequest{
			ExpectedRunID: "run-stale",
			Run:           &replacement,
			Actor:         actor,
			ReplacedAt:    now.Add(2 * time.Second),
		})
		requireInlineGoalReason(t, err, looppkg.ReasonCodeGoalReplaceStale)
		if stillLive, getErr := globalDB.GetLoopRun(ctx, "ws-inline", first.ID); getErr != nil ||
			stillLive.Status != looppkg.StatusRunning {
			t.Fatalf("stale replace changed old Run: run=%#v error=%v", stillLive, getErr)
		}

		result, err := globalDB.ReplaceInlineLoopRun(ctx, looppkg.InlineReplaceStoreRequest{
			ExpectedRunID: first.ID,
			Run:           &replacement,
			Actor:         actor,
			ReplacedAt:    now.Add(2 * time.Second),
		})
		if err != nil {
			t.Fatalf("ReplaceInlineLoopRun() error = %v", err)
		}
		if result.ReplacedRunID != first.ID || result.Run.ID != replacement.ID ||
			result.ReplacedRun.Status != looppkg.StatusFailed {
			t.Fatalf("replacement result = %#v", result)
		}
		var replaceEvents int
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM loop_goal_session_outbox
			 WHERE loop_run_id = ? AND cause = 'replace'`,
			string(replacement.ID),
		).Scan(&replaceEvents); err != nil {
			t.Fatalf("count replacement outbox error = %v", err)
		}
		if replaceEvents != 1 {
			t.Fatalf("replacement outbox events = %d, want 1", replaceEvents)
		}

		cleared, err := globalDB.ClearInlineGoal(ctx, looppkg.InlineGoalClearStoreRequest{
			WorkspaceID:     "ws-inline",
			OriginSessionID: origin.SessionID,
			Actor:           actor,
			ClearedAt:       now.Add(3 * time.Second),
		})
		if err != nil {
			t.Fatalf("ClearInlineGoal(live) error = %v", err)
		}
		if !cleared.Terminalized || cleared.Run.ID != replacement.ID || cleared.Run.Status != looppkg.StatusFailed {
			t.Fatalf("ClearInlineGoal(live) result = %#v", cleared)
		}
		projection, err := globalDB.GetSessionGoalProjection(ctx, "ws-inline", origin.SessionID)
		if err != nil {
			t.Fatalf("GetSessionGoalProjection(cleared) error = %v", err)
		}
		if !projection.Found || !projection.Cleared || projection.RunID != replacement.ID {
			t.Fatalf("cleared projection = %#v", projection)
		}
		var clearEvents int
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM loop_goal_session_outbox
			 WHERE loop_run_id = ? AND cause = 'clear' AND bound_session_id IS NULL`,
			string(replacement.ID),
		).Scan(&clearEvents); err != nil {
			t.Fatalf("count clear outbox error = %v", err)
		}
		if clearEvents != 1 {
			t.Fatalf("clear outbox events = %d, want 1", clearEvents)
		}
		_, err = globalDB.ClearInlineGoal(ctx, looppkg.InlineGoalClearStoreRequest{
			WorkspaceID:     "ws-inline",
			OriginSessionID: origin.SessionID,
			Actor:           actor,
			ClearedAt:       now.Add(4 * time.Second),
		})
		requireInlineGoalReason(t, err, looppkg.ReasonCodeGoalNotActive)
	})

	t.Run("Should clear only the newest terminal Goal without changing its audit status", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-inline-terminal")
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 10, 23, 30, 0, 0, time.UTC)
		origin := inlineGoalOriginForTest("session-inline-terminal")
		registerGoalSessionIdentityForTest(
			t,
			globalDB,
			goalSessionInfoForTest(origin.SessionID, "ws-inline-terminal", now),
			store.SessionCreationIdentity{
				CreationProfileRef: origin.CreationProfileRef,
				PolicySpecDigest:   origin.PolicySpecDigest,
				CreationDigest:     origin.CreationDigest,
			},
		)
		actor, err := taskpkg.DeriveHumanActorContext("operator", taskpkg.OriginKindUDS, "goal.clear")
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		first := inlineGoalRunForTest(t, "run-inline-terminal-a", "ws-inline-terminal", origin, now)
		if _, err := globalDB.CreateInlineLoopRunForStart(ctx, first); err != nil {
			t.Fatalf("CreateInlineLoopRunForStart(first) error = %v", err)
		}
		if err := globalDB.CompareAndSwapLoopRunStatus(
			ctx,
			first.ID,
			looppkg.StatusRunning,
			looppkg.StatusDone,
			looppkg.TransitionCauseContract,
			now.Add(time.Second),
		); err != nil {
			t.Fatalf("CompareAndSwapLoopRunStatus(first) error = %v", err)
		}
		second := inlineGoalRunForTest(t, "run-inline-terminal-b", "ws-inline-terminal", origin, now.Add(2*time.Second))
		if _, err := globalDB.CreateInlineLoopRunForStart(ctx, second); err != nil {
			t.Fatalf("CreateInlineLoopRunForStart(second) error = %v", err)
		}
		if err := globalDB.CompareAndSwapLoopRunStatus(
			ctx,
			second.ID,
			looppkg.StatusRunning,
			looppkg.StatusDone,
			looppkg.TransitionCauseContract,
			now.Add(3*time.Second),
		); err != nil {
			t.Fatalf("CompareAndSwapLoopRunStatus(second) error = %v", err)
		}

		cleared, err := globalDB.ClearInlineGoal(ctx, looppkg.InlineGoalClearStoreRequest{
			WorkspaceID:     "ws-inline-terminal",
			OriginSessionID: origin.SessionID,
			Actor:           actor,
			ClearedAt:       now.Add(4 * time.Second),
		})
		if err != nil {
			t.Fatalf("ClearInlineGoal(terminal) error = %v", err)
		}
		if cleared.Terminalized || cleared.Run.ID != second.ID || cleared.Run.Status != looppkg.StatusDone {
			t.Fatalf("ClearInlineGoal(terminal) result = %#v", cleared)
		}
		persistedFirst, err := globalDB.GetLoopRun(ctx, "ws-inline-terminal", first.ID)
		if err != nil {
			t.Fatalf("GetLoopRun(first) error = %v", err)
		}
		persistedSecond, err := globalDB.GetLoopRun(ctx, "ws-inline-terminal", second.ID)
		if err != nil {
			t.Fatalf("GetLoopRun(second) error = %v", err)
		}
		if persistedFirst.Status != looppkg.StatusDone || persistedSecond.Status != looppkg.StatusDone {
			t.Fatalf("terminal statuses = [%q %q], want both done", persistedFirst.Status, persistedSecond.Status)
		}
		projection, err := globalDB.GetSessionGoalProjection(ctx, "ws-inline-terminal", origin.SessionID)
		if err != nil {
			t.Fatalf("GetSessionGoalProjection() error = %v", err)
		}
		if !projection.Cleared || projection.RunID != second.ID {
			t.Fatalf("newest cleared projection = %#v, want second Run tombstone", projection)
		}
		var firstClearedAt, secondClearedAt sql.NullString
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT goal_cleared_at FROM loop_runs WHERE id = ?`,
			string(first.ID),
		).Scan(&firstClearedAt); err != nil {
			t.Fatalf("load first clear tombstone error = %v", err)
		}
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT goal_cleared_at FROM loop_runs WHERE id = ?`,
			string(second.ID),
		).Scan(&secondClearedAt); err != nil {
			t.Fatalf("load second clear tombstone error = %v", err)
		}
		if firstClearedAt.Valid || !secondClearedAt.Valid {
			t.Fatalf("clear tombstones = first:%v second:%v", firstClearedAt.Valid, secondClearedAt.Valid)
		}
	})
}

type inlineStartResult struct {
	run looppkg.Run
	err error
}

func runInlineStartsConcurrently(
	ctx context.Context,
	globalDB *GlobalDB,
	runs []looppkg.Run,
) []inlineStartResult {
	results := make(chan inlineStartResult, len(runs))
	start := make(chan struct{})
	var workers sync.WaitGroup
	workers.Add(len(runs))
	for _, candidate := range runs {
		candidate := candidate
		go func() {
			defer workers.Done()
			<-start
			created, err := globalDB.CreateInlineLoopRunForStart(ctx, candidate)
			results <- inlineStartResult{run: created, err: err}
		}()
	}
	close(start)
	workers.Wait()
	close(results)

	collected := make([]inlineStartResult, 0, len(runs))
	for result := range results {
		collected = append(collected, result)
	}
	return collected
}

type inlineReplaceResult struct {
	result looppkg.InlineReplaceStoreResult
	err    error
}

func runInlineReplacementsConcurrently(
	ctx context.Context,
	globalDB *GlobalDB,
	requests []looppkg.InlineReplaceStoreRequest,
) []inlineReplaceResult {
	results := make(chan inlineReplaceResult, len(requests))
	start := make(chan struct{})
	var workers sync.WaitGroup
	workers.Add(len(requests))
	for _, request := range requests {
		request := request
		go func() {
			defer workers.Done()
			<-start
			result, err := globalDB.ReplaceInlineLoopRun(ctx, request)
			results <- inlineReplaceResult{result: result, err: err}
		}()
	}
	close(start)
	workers.Wait()
	close(results)

	collected := make([]inlineReplaceResult, 0, len(requests))
	for result := range results {
		collected = append(collected, result)
	}
	return collected
}

func inlineGoalOriginForTest(sessionID string) looppkg.RunOrigin {
	return looppkg.RunOrigin{
		Kind: looppkg.RunOriginSession, SessionID: sessionID,
		CreationProfileRef: "profile:" + sessionID,
		PolicySpecDigest:   "policy:" + sessionID,
		CreationDigest:     "creation:" + sessionID,
	}
}

func inlineGoalRunForTest(
	t *testing.T,
	runID string,
	workspaceID looppkg.WorkspaceID,
	origin looppkg.RunOrigin,
	at time.Time,
) looppkg.Run {
	t.Helper()
	definition := dsl.Definition{
		APIVersion:  dsl.APIVersion,
		Kind:        dsl.KindLoop,
		Meta:        dsl.Meta{Name: looppkg.InlineGoalLoopName, Version: 1},
		Concurrency: dsl.ConcurrencyAllow,
		Contract: dsl.Contract{
			Goal: "ship the release", DefinitionOfDone: "The objective is satisfied.",
			IterationCap: 3, NoProgress: dsl.NoProgress{Window: 1},
			Budget:        dsl.Budget{OnExceeded: dsl.BudgetExceededHalt},
			ModelDefaults: &dsl.ModelDefaults{Judge: "judge-v1"},
		},
		Graph: dsl.Graph{Nodes: []dsl.Node{{
			ID: "goal", Class: dsl.NodeClassAction, Kind: string(dsl.ActionGoal),
			Session: &dsl.SessionSpec{Mode: dsl.SessionModeContinuous},
			Params: dsl.NodeParams{
				"agent": "codex", "objective": "ship the release", "max_turns": 3,
				"judge": []dsl.GateCriterion{{
					ID: "objective_satisfied", Type: dsl.CriterionAgentJudge, Rubric: "ship the release",
				}},
				"output_schema": map[string]any{
					"status": map[string]any{"type": "string", "enum": []string{"complete", "blocked"}},
				},
			},
		}}, Edges: []dsl.Edge{}},
	}
	resolved, err := looppkg.NewCompiler().Compile(definition)
	if err != nil {
		t.Fatalf("Compile(inline Goal) error = %v", err)
	}
	effective, err := looppkg.ResolveEffectiveConfig(
		resolved,
		looppkg.DefaultLoopDefaults(),
		nil,
		looppkg.LoopConfig{},
	)
	if err != nil {
		t.Fatalf("ResolveEffectiveConfig(inline Goal) error = %v", err)
	}
	snapshot, digest, err := looppkg.BuildExecutedDefinitionSnapshot(resolved, effective)
	if err != nil {
		t.Fatalf("BuildExecutedDefinitionSnapshot(inline Goal) error = %v", err)
	}
	actor, err := taskpkg.DeriveHumanActorContext("operator", taskpkg.OriginKindHTTP, "goal.start")
	if err != nil {
		t.Fatalf("DeriveHumanActorContext(start) error = %v", err)
	}
	return looppkg.Run{
		ID: looppkg.RunID(runID), WorkspaceID: workspaceID,
		LoopName: looppkg.InlineGoalLoopName, Status: looppkg.StatusRunning,
		ReattemptStrategy: looppkg.ReattemptFailedOnly,
		CreatedAt:         at, StartedAt: at, LastProgressAt: at,
		StartedBy: actor.Actor, StartedOrigin: actor.Origin,
		DefinitionVersion: resolved.DefinitionVersion,
		DefinitionDigest:  digest, DefinitionSnapshot: snapshot,
		ActiveHumanCriteria: []byte(`[]`), StartMetadata: map[string]any{},
		IterationCap: effective.IterationCap, BudgetOnExceeded: effective.BudgetOnExceeded,
		GoalContextNudgeRatio: 0.8, Origin: &origin, Inputs: map[string]any{},
	}
}

func requireInlineGoalReason(t *testing.T, err error, want looppkg.ReasonCode) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want reason code %q", want)
	}
	var reason *looppkg.ReasonError
	if !errors.As(err, &reason) || reason.Code != want {
		t.Fatalf("error = %v, want reason code %q", err, want)
	}
}
