package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/gate"
	storepkg "github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil"
)

func TestLoopRunEventKindValidShouldMatchPublicContract(t *testing.T) {
	t.Parallel()

	localKinds := map[string]struct{}{
		loopRunEventNodeRunning:             {},
		loopRunEventNodeSucceeded:           {},
		loopRunEventNodeFailed:              {},
		loopRunEventGateVerdict:             {},
		loopRunEventGenerationStarted:       {},
		loopRunEventChannelMsg:              {},
		loopRunEventTokenTick:               {},
		loopRunEventNeedsApproval:           {},
		loopRunEventStatusChanged:           {},
		loopRunEventGoalTurnStarted:         {},
		loopRunEventGoalTurnCompleted:       {},
		loopRunEventGoalStatusChanged:       {},
		loopRunEventRuntimeApplied:          {},
		loopRunEventNodeRetryScheduled:      {},
		loopRunEventNodePaused:              {},
		loopRunEventNodeResumed:             {},
		loopRunEventNodeCanceled:            {},
		loopRunEventNodeKilled:              {},
		loopRunEventNodeQuarantined:         {},
		loopRunEventNodeRequeued:            {},
		loopRunEventNodeWaitStarted:         {},
		loopRunEventNodeWaitResumed:         {},
		loopRunEventNodeAttentionFlagged:    {},
		loopRunEventNodeAttentionCleared:    {},
		loopRunEventEffectResults:           {},
		loopRunEventCustomEvent:             {},
		loopRunEventDuplicateSuppressed:     {},
		loopRunEventTargetBreakerTransition: {},
		loopRunEventStaleScheduleDropped:    {},
		loopRunEventLateArrival:             {},
	}
	for _, kind := range contract.LoopRunEventKindValues() {
		t.Run("Should accept public kind "+kind, func(t *testing.T) {
			t.Parallel()
			if !loopRunEventKindValid(kind) {
				t.Fatalf("loopRunEventKindValid(%q) = false, want true", kind)
			}
			if _, ok := localKinds[kind]; !ok {
				t.Fatalf("contract kind %q is missing from local loop event constants", kind)
			}
		})
	}
	for kind := range localKinds {
		t.Run("Should publish local kind "+kind, func(t *testing.T) {
			t.Parallel()
			if !slices.Contains(contract.LoopRunEventKindValues(), kind) {
				t.Fatalf("local loop event kind %q is missing from public contract", kind)
			}
		})
	}
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

// Invariant: a Run cancellation request, every node fence, Goal cleanup, and canceled terminal
// truth share one SQLite authority. This store suite owns the atomic cancellation boundary.
func TestGlobalDBLoopRunCancellationShouldFenceBeforeTerminalizing(t *testing.T) {
	t.Parallel()

	globalDB := openLoopTestGlobalDB(t, "ws-1", "ws-other")
	ctx := testutil.Context(t)
	now := time.Date(2026, time.August, 3, 1, 0, 0, 0, time.UTC)
	run, taskRunID := seedLiveLoopLivenessCellForTest(t, globalDB, now)
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
		result, err := globalDB.RequeueNode(ctx, looppkg.NodeRequeueMutation{
			WorkspaceID:      looppkg.WorkspaceID(" " + string(run.WorkspaceID) + " "),
			RunID:            looppkg.RunID(" " + string(run.ID) + " "),
			NodeID:           " finish ",
			Reason:           strings.Repeat("target repaired ", 2_000) + "api_key=planted-secret",
			ExpectedRevision: &expectedRevision,
			Actor:            operatorActorContextForTest("operator:alice"),
			RequestedAt:      now.Add(time.Minute),
		})
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
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT needs_attention_at FROM tasks WHERE id = ?`,
			looppkg.NodeCellTaskID(run.ID, 2, "finish", 0),
		).Scan(&attentionAt); err != nil {
			t.Fatalf("read requeued cell task attention error = %v", err)
		}
		if attentionAt.Valid {
			t.Fatalf("cell task needs_attention_at = %v, want cleared on requeue", attentionAt.Time)
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

// Invariant: applying a quarantine control mutation parks the quarantined cell's
// workspace task in needs-attention within the same boundary transaction.
func TestGlobalDBQuarantineSnapshotShouldParkCellTasks(t *testing.T) {
	t.Parallel()

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
		if _, err := globalDB.CompleteCoordinatorAndEnqueueNext(
			ctx,
			taskpkg.CoordinatorCompletion{
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
			},
			looppkg.NewStoreFinalizer(),
		); err != nil {
			t.Fatalf("CompleteCoordinatorAndEnqueueNext(quarantine) error = %v", err)
		}
		parked, err := globalDB.GetTask(ctx, cellTaskID)
		if err != nil {
			t.Fatalf("GetTask(parked cell) error = %v", err)
		}
		if parked.NeedsAttention == nil || parked.NeedsAttention.At.IsZero() ||
			!strings.Contains(parked.NeedsAttention.Reason, "quarantined") {
			t.Fatalf(
				"cell task needs_attention = %#v, want quarantine park with reason",
				parked.NeedsAttention,
			)
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
		`UPDATE tasks SET needs_attention_reason = 'loop node finish is quarantined',
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
	taskRecord := taskRecordForTest("task-loop-liveness-" + caseID)
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
	taskRun.RunKind = taskpkg.RunKindWorker
	taskRun.LoopRunID = string(run.ID)
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
