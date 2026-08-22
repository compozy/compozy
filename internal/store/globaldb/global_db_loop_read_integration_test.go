//go:build integration

package globaldb

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/testutil"
)

func TestGlobalDBLoopReadServiceIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should satisfy IT-017 IT-019 and IT-020 with faithful roster and briefing rereads", func(t *testing.T) {
		t.Parallel()
		globalDB := openLoopTestGlobalDB(t, "ws-read")
		ctx := testutil.Context(t)
		now := time.Date(2026, 8, 20, 19, 0, 0, 0, time.UTC)
		run := loopRunWithRouteDefinitionForReadTest(t, "run-read-faithful", "ws-read", now)
		created, err := globalDB.CreateLoopRunForStart(ctx, run, dsl.ConcurrencyAllow)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(ctx, `UPDATE loop_runs
			SET generation = 1, tokens_used = 321, last_progress_at = ? WHERE id = ?`,
			now.Add(time.Minute), created.ID); err != nil {
			t.Fatalf("advance read fixture error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_generation_outputs
			(loop_run_id, generation, node_id, item_index, status, attempt)
			VALUES (?, 1, 'selected', 0, 'running', 1)`, created.ID); err != nil {
			t.Fatalf("insert running output error = %v", err)
		}
		appendRouteTakenForReadTest(t, globalDB, created, now)
		reads := looppkg.NewRunReadService(globalDB, func() time.Time { return now.Add(2 * time.Minute) })

		mid, err := reads.NodeRoster(ctx, "ws-read", created.ID, looppkg.RosterQuery{State: looppkg.NodeStateFilterAll})
		if err != nil {
			t.Fatalf("NodeRoster(mid-generation) error = %v", err)
		}
		assertReadRosterStates(t, mid, map[looppkg.NodeID]looppkg.NodeState{
			"selected": looppkg.NodeStateRunning,
			"skipped":  looppkg.NodeStateNotTaken,
		})
		runningOnly, err := reads.NodeRoster(
			ctx,
			"ws-read",
			created.ID,
			looppkg.RosterQuery{State: looppkg.NodeStateFilter(looppkg.NodeStateRunning)},
		)
		if err != nil {
			t.Fatalf("NodeRoster(running) error = %v", err)
		}
		if len(runningOnly.Nodes) != 1 || runningOnly.Nodes[0].NodeID != "selected" {
			t.Fatalf("running roster = %#v, want only selected", runningOnly.Nodes)
		}
		briefing, err := reads.Briefing(ctx, "ws-read", created.ID)
		if err != nil {
			t.Fatalf("Briefing() error = %v", err)
		}
		storedRun, err := globalDB.GetLoopRun(ctx, "ws-read", created.ID)
		if err != nil {
			t.Fatalf("GetLoopRun() error = %v", err)
		}
		wantBriefing := looppkg.ProjectBriefing(&looppkg.BriefingSource{
			Run:    storedRun,
			Roster: mid,
			Now:    now.Add(2 * time.Minute),
		})
		if briefing.Headline != wantBriefing.Headline || briefing.Progress != wantBriefing.Progress ||
			briefing.Usage.Tokens != 321 {
			t.Fatalf("served/internal briefing = %#v/%#v", briefing, wantBriefing)
		}

		if _, err := globalDB.db.ExecContext(ctx, `UPDATE loop_generation_outputs
			SET status = 'succeeded' WHERE loop_run_id = ? AND generation = 1 AND node_id = 'selected'`,
			created.ID); err != nil {
			t.Fatalf("settle selected output error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_runs SET status = 'done' WHERE id = ?`,
			created.ID,
		); err != nil {
			t.Fatalf("terminalize read fixture error = %v", err)
		}
		terminal, err := reads.NodeRoster(
			ctx,
			"ws-read",
			created.ID,
			looppkg.RosterQuery{State: looppkg.NodeStateFilterAll},
		)
		if err != nil {
			t.Fatalf("NodeRoster(terminal) error = %v", err)
		}
		assertReadRosterStates(t, terminal, map[looppkg.NodeID]looppkg.NodeState{
			"selected": looppkg.NodeStateSucceeded,
			"skipped":  looppkg.NodeStateNotTaken,
		})
	})

	t.Run("Should satisfy IT-018 with stable hundred-item pagination while rows settle", func(t *testing.T) {
		t.Parallel()
		globalDB := openLoopTestGlobalDB(t, "ws-fanout")
		ctx := testutil.Context(t)
		now := time.Date(2026, 8, 20, 19, 15, 0, 0, time.UTC)
		run := testLoopRun("run-fanout-100", now, looppkg.StatusRunning)
		run.WorkspaceID = "ws-fanout"
		created, err := globalDB.CreateLoopRunForStart(ctx, run, dsl.ConcurrencyAllow)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_runs SET generation = 1 WHERE id = ?`,
			created.ID,
		); err != nil {
			t.Fatalf("advance fanout run error = %v", err)
		}
		for itemIndex := 0; itemIndex < 100; itemIndex++ {
			if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_generation_outputs
				(loop_run_id, generation, node_id, item_index, status, attempt)
				VALUES (?, 1, 'finish', ?, 'running', 1)`, created.ID, itemIndex); err != nil {
				t.Fatalf("insert fanout item %d error = %v", itemIndex, err)
			}
		}
		reads := looppkg.NewRunReadService(globalDB, nil)
		query := looppkg.RosterQuery{State: looppkg.NodeStateFilterAll, Limit: 17}
		seen := make(map[int]bool, 100)
		pageIndex := 0
		for {
			page, err := reads.NodeRoster(ctx, "ws-fanout", created.ID, query)
			if err != nil {
				t.Fatalf("NodeRoster(page %d) error = %v", pageIndex, err)
			}
			if len(page.FanoutRollups) != 1 || page.FanoutRollups[0].Total != 100 {
				t.Fatalf("fanout rollup page %d = %#v", pageIndex, page.FanoutRollups)
			}
			for _, node := range page.Nodes {
				if seen[node.ItemIndex] {
					t.Fatalf("fanout item %d duplicated", node.ItemIndex)
				}
				seen[node.ItemIndex] = true
			}
			if pageIndex == 0 {
				if _, err := globalDB.db.ExecContext(ctx, `UPDATE loop_generation_outputs SET status = 'succeeded'
					WHERE loop_run_id = ? AND generation = 1 AND item_index >= 50`, created.ID); err != nil {
					t.Fatalf("settle fanout tail error = %v", err)
				}
			}
			if page.NextCursor == "" {
				break
			}
			query.Cursor = page.NextCursor
			pageIndex++
		}
		if len(seen) != 100 {
			t.Fatalf("fanout items seen = %d, want 100", len(seen))
		}
	})

	t.Run("Should satisfy IT-021 IT-022 and IT-023 with fenced readers branches and tiers", func(t *testing.T) {
		t.Parallel()
		globalDB := openLoopTestGlobalDB(t, "ws-events")
		ctx := testutil.Context(t)
		now := time.Date(2026, 8, 20, 19, 30, 0, 0, time.UTC)
		runA := testLoopRun("run-events-a", now, looppkg.StatusRunning)
		runA.WorkspaceID = "ws-events"
		createdA, err := globalDB.CreateLoopRunForStart(ctx, runA, dsl.ConcurrencyAllow)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart(A) error = %v", err)
		}
		insertReadEvent(t, globalDB, createdA, 2, looppkg.RunEventNodeRunning, now.Add(2*time.Second))
		insertReadEvent(t, globalDB, createdA, 3, looppkg.RunEventTokenTick, now.Add(3*time.Second))
		insertReadEvent(t, globalDB, createdA, 4, looppkg.RunEventNodeSucceeded, now.Add(4*time.Second))
		reads := looppkg.NewRunReadService(globalDB, nil)
		first, err := reads.Timeline(ctx, "ws-events", createdA.ID, looppkg.TimelineQuery{
			View:  looppkg.TimelineViewAll,
			Limit: 2,
		})
		if err != nil {
			t.Fatalf("Timeline(first) error = %v", err)
		}
		insertReadEvent(t, globalDB, createdA, 5, looppkg.RunEventNodeFailed, now.Add(5*time.Second))
		var readerSequences [2][]int64
		var readerErrors [2]error
		var waitGroup sync.WaitGroup
		for readerIndex := range readerSequences {
			waitGroup.Add(1)
			go func(index int) {
				defer waitGroup.Done()
				page, readErr := reads.Timeline(ctx, "ws-events", createdA.ID, looppkg.TimelineQuery{
					View:     looppkg.TimelineViewAll,
					AfterSeq: 1,
					Limit:    2,
				})
				if readErr != nil {
					readerErrors[index] = readErr
					return
				}
				readerSequences[index], readerErrors[index] = drainTimelineSequencesForReadTest(
					ctx,
					reads,
					"ws-events",
					createdA.ID,
					page,
					looppkg.TimelineQuery{View: looppkg.TimelineViewAll, AfterSeq: 1, Limit: 2},
				)
			}(readerIndex)
		}
		waitGroup.Wait()
		if readerErrors[0] != nil || readerErrors[1] != nil ||
			!slices.Equal(readerSequences[0], readerSequences[1]) ||
			!slices.Equal(readerSequences[0], []int64{5, 4, 3, 2}) {
			t.Fatalf(
				"timeline readers = %v/%v errors=%v/%v",
				readerSequences[0],
				readerSequences[1],
				readerErrors[0],
				readerErrors[1],
			)
		}
		if first.HeadSeq != 4 {
			t.Fatalf("first fenced head = %d, want 4", first.HeadSeq)
		}

		runB := testLoopRun("run-events-b", now.Add(time.Minute), looppkg.StatusRunning)
		runB.WorkspaceID = "ws-events"
		createdB, err := globalDB.CreateLoopRunForStart(ctx, runB, dsl.ConcurrencyAllow)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart(B) error = %v", err)
		}
		_, err = reads.Timeline(ctx, "ws-events", createdB.ID, looppkg.TimelineQuery{
			View:   looppkg.TimelineViewAll,
			Cursor: first.NextCursor,
		})
		if !errors.Is(err, looppkg.ErrTimelineBranchChanged) {
			t.Fatalf("Timeline(foreign cursor) error = %v", err)
		}
		fork := testLoopRun("run-events-fork", now.Add(2*time.Minute), looppkg.StatusRunning)
		fork.WorkspaceID = "ws-events"
		fork.SetForkedFrom(&looppkg.ForkRef{RunID: createdA.ID, Generation: 1})
		createdFork, err := globalDB.CreateLoopRunForStart(ctx, fork, dsl.ConcurrencyAllow)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart(fork) error = %v", err)
		}
		_, err = reads.Timeline(ctx, "ws-events", createdFork.ID, looppkg.TimelineQuery{
			View:   looppkg.TimelineViewAll,
			Cursor: first.NextCursor,
		})
		if !errors.Is(err, looppkg.ErrTimelineBranchChanged) {
			t.Fatalf("Timeline(fork cursor) error = %v", err)
		}

		notable, err := reads.Timeline(ctx, "ws-events", createdA.ID, looppkg.TimelineQuery{
			View:  looppkg.TimelineViewNotable,
			Limit: 500,
		})
		if err != nil {
			t.Fatalf("Timeline(notable) error = %v", err)
		}
		all, err := reads.Timeline(ctx, "ws-events", createdA.ID, looppkg.TimelineQuery{
			View:  looppkg.TimelineViewAll,
			Limit: 500,
		})
		if err != nil {
			t.Fatalf("Timeline(all) error = %v", err)
		}
		if len(notable.Entries) != 3 || len(all.Entries) != 5 {
			t.Fatalf("timeline tier counts notable/all = %d/%d, want 3/5", len(notable.Entries), len(all.Entries))
		}
	})

	t.Run("Should satisfy IT-027 with verb-to-roster visibility and stale conflict", func(t *testing.T) {
		t.Parallel()
		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 8, 20, 19, 45, 0, 0, time.UTC)
		run := seedQuarantinedLoopNodeForTest(t, globalDB, now, looppkg.StatusRunning)
		service, err := looppkg.NewService(
			globalDB,
			looppkg.DefinitionResolverFunc(
				func(context.Context, looppkg.WorkspaceID, string) (*looppkg.ResolvedDefinition, error) {
					return nil, errors.New("unexpected definition resolution")
				},
			),
			looppkg.GoalRunPolicyResolverFunc(
				func(context.Context, looppkg.WorkspaceID) (*looppkg.GoalRunPolicy, error) {
					return &looppkg.GoalRunPolicy{ContextNudgeRatio: 0.8}, nil
				},
			),
			looppkg.WithClock(func() time.Time { return now.Add(time.Minute) }),
		)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		lifecycle, ok := service.(looppkg.NodeLifecycleService)
		if !ok {
			t.Fatalf("NewService() = %T, want NodeLifecycleService", service)
		}
		reads := looppkg.NewRunReadService(globalDB, nil)
		before, err := reads.NodeRoster(
			ctx,
			string(run.WorkspaceID),
			run.ID,
			looppkg.RosterQuery{State: looppkg.NodeStateFilterAll},
		)
		if err != nil {
			t.Fatalf("NodeRoster(before requeue) error = %v", err)
		}
		assertReadRosterStates(t, before, map[looppkg.NodeID]looppkg.NodeState{"finish": looppkg.NodeStateQuarantined})
		if _, err := lifecycle.RequeueNode(
			ctx,
			run.WorkspaceID,
			run.ID,
			"finish",
			"operator repaired the target",
			operatorActorContextForTest("operator:read"),
		); err != nil {
			t.Fatalf("RequeueNode() error = %v", err)
		}
		if err := insertLoopGenerationWithExecutor(ctx, globalDB.db, run.ID, looppkg.GenerationIntent{
			Generation:       int64(run.Generation + 1),
			ParentGeneration: int64(run.Generation),
			Origin:           looppkg.OriginRequeue,
		}, now.Add(2*time.Minute)); err != nil {
			t.Fatalf("materialize requeue generation error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_runs SET generation = ?, last_progress_at = ? WHERE id = ?`,
			run.Generation+1,
			now.Add(2*time.Minute),
			run.ID,
		); err != nil {
			t.Fatalf("advance requeue generation error = %v", err)
		}
		after, err := reads.NodeRoster(ctx, string(run.WorkspaceID), run.ID, looppkg.RosterQuery{
			State:      looppkg.NodeStateFilterAll,
			Generation: run.Generation + 1,
		})
		if err != nil {
			t.Fatalf("NodeRoster(after requeue) error = %v", err)
		}
		assertReadRosterStates(t, after, map[looppkg.NodeID]looppkg.NodeState{"finish": looppkg.NodeStatePending})
		_, err = lifecycle.RequeueNode(
			ctx,
			run.WorkspaceID,
			run.ID,
			"finish",
			"stale replay",
			operatorActorContextForTest("operator:read"),
		)
		if !errors.Is(err, looppkg.ErrTransitionConflict) {
			t.Fatalf("RequeueNode(stale) error = %v, want transition conflict", err)
		}
	})
}

func appendRouteTakenForReadTest(t testing.TB, globalDB *GlobalDB, run looppkg.Run, at time.Time) {
	t.Helper()
	if err := appendLoopRunEventWithExecutor(
		testutil.Context(t),
		globalDB.db,
		run.ID,
		run.WorkspaceID,
		loopRunEventRouteTaken,
		map[string]any{
			"generation": 1,
			"node_id":    "router",
			"item_index": 0,
			"route":      "selected",
			"cause":      "matched_when",
		},
		at,
	); err != nil {
		t.Fatalf("append route_taken error = %v", err)
	}
}

func assertReadRosterStates(
	t testing.TB,
	page looppkg.RosterPage,
	want map[looppkg.NodeID]looppkg.NodeState,
) {
	t.Helper()
	for nodeID, wantState := range want {
		found := false
		for _, node := range page.Nodes {
			if node.NodeID != nodeID {
				continue
			}
			found = true
			if node.State != wantState {
				t.Fatalf("node %q state = %q, want %q; roster=%#v", nodeID, node.State, wantState, page.Nodes)
			}
		}
		if !found {
			t.Fatalf("node %q missing from roster %#v", nodeID, page.Nodes)
		}
	}
}

func insertReadEvent(
	t testing.TB,
	globalDB *GlobalDB,
	run looppkg.Run,
	sequence int64,
	kind looppkg.RunEventKind,
	at time.Time,
) {
	t.Helper()
	payload := `{"generation":1,"node_id":"finish","attempt":1}`
	if _, err := globalDB.db.ExecContext(testutil.Context(t), `INSERT INTO loop_run_events
		(id, loop_run_id, workspace_id, seq, kind, payload_json, at, delivery_key)
		VALUES (?, ?, ?, ?, ?, ?, ?, NULL)`,
		fmt.Sprintf("event-read-%s-%d", run.ID, sequence), run.ID, run.WorkspaceID,
		sequence, kind, payload, at); err != nil {
		t.Fatalf("insert read event %d error = %v", sequence, err)
	}
}

func drainTimelineSequencesForReadTest(
	ctx context.Context,
	reads looppkg.RunReadService,
	workspaceID string,
	runID looppkg.RunID,
	page looppkg.TimelinePage,
	query looppkg.TimelineQuery,
) ([]int64, error) {
	sequences := make([]int64, 0)
	for {
		for _, entry := range page.Entries {
			sequences = append(sequences, entry.Seq)
		}
		if page.NextCursor == "" {
			return sequences, nil
		}
		query.Cursor = page.NextCursor
		var err error
		page, err = reads.Timeline(ctx, workspaceID, runID, query)
		if err != nil {
			return nil, fmt.Errorf("read timeline continuation: %w", err)
		}
	}
}
