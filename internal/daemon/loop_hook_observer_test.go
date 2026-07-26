package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
	looppkg "github.com/compozy/agh/internal/loop"
	taskpkg "github.com/compozy/agh/internal/task"
)

func TestLoopNativeHookObserverShouldProtectDurableNodeTerminalWake(t *testing.T) {
	t.Parallel()

	t.Run("Should enqueue node terminal wake before public hook dispatch can exhaust the context", func(t *testing.T) {
		t.Parallel()

		fixedNow := time.Date(2026, 7, 4, 12, 30, 0, 0, time.UTC)
		loopStore := &contextAwareLoopHookStore{}
		backstop := &recordingLoopBackstopRunner{}
		observer, err := newLoopNativeHookObserver(
			loopStore,
			blockingLoopNodeTerminalDispatcher{},
			backstop,
			func() time.Time { return fixedNow },
		)
		if err != nil {
			t.Fatalf("newLoopNativeHookObserver() error = %v", err)
		}

		ctx, cancel := context.WithTimeout(t.Context(), 25*time.Millisecond)
		defer cancel()
		workerKind := taskpkg.RunKindWorker.String()
		err = observer.OnTaskRunTerminal(ctx, hookspkg.TaskRunLeasePayload{
			PayloadBase: hookspkg.PayloadBase{Event: hookspkg.HookTaskRunCompleted, Timestamp: fixedNow},
			TaskRunContext: hookspkg.TaskRunContext{
				TaskID:      "task-node",
				RunID:       "run-node",
				RunKind:     &workerKind,
				LoopRunID:   "loop-run-1",
				WorkspaceID: "ws-1",
				RunStatus:   taskpkg.TaskRunStatusCompleted.String(),
				TaskStatus:  string(taskpkg.TaskStatusCompleted),
			},
		})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("OnTaskRunTerminal() error = %v, want context deadline exceeded", err)
		}

		if got, want := len(loopStore.progress), 1; got != want {
			t.Fatalf("progress calls = %d, want %d", got, want)
		}
		if got, want := len(loopStore.wakes), 1; got != want {
			t.Fatalf("wake calls = %d, want %d", got, want)
		}
		if got, want := loopStore.wakes[0].idempotencyKey, "loop.coordinator.node_terminal.loop-run-1.run-node"; got != want {
			t.Fatalf("wake key = %q, want %q", got, want)
		}
		if got, want := len(backstop.calls), 1; got != want {
			t.Fatalf("backstop calls = %d, want %d", got, want)
		}
	})
}

func TestLoopNativeHookObserverShouldSuppressIntermediateGoalTerminal(t *testing.T) {
	t.Parallel()

	for _, status := range []string{"control_pending", "awaiting_goal"} {
		t.Run("Should suppress the false node terminal hook for "+status, func(t *testing.T) {
			t.Parallel()

			fixedNow := time.Date(2026, 7, 10, 19, 0, 0, 0, time.UTC)
			loopStore := &recordingLoopHookStore{
				outputStatuses: map[string]string{"loop-goal/run-goal": status},
			}
			dispatcher := &recordingLoopNodeTerminalDispatcher{}
			backstop := &recordingLoopBackstopRunner{}
			observer, err := newLoopNativeHookObserver(loopStore, dispatcher, backstop, func() time.Time {
				return fixedNow
			})
			if err != nil {
				t.Fatalf("newLoopNativeHookObserver() error = %v", err)
			}

			workerKind := taskpkg.RunKindWorker.String()
			if err := observer.OnTaskRunTerminal(t.Context(), hookspkg.TaskRunLeasePayload{
				PayloadBase: hookspkg.PayloadBase{Event: hookspkg.HookTaskRunCompleted, Timestamp: fixedNow},
				TaskRunContext: hookspkg.TaskRunContext{
					TaskID:      "task-goal",
					RunID:       "run-goal",
					RunKind:     &workerKind,
					LoopRunID:   "loop-goal",
					WorkspaceID: "ws-goal",
					RunStatus:   taskpkg.TaskRunStatusCompleted.String(),
				},
			}); err != nil {
				t.Fatalf("OnTaskRunTerminal() error = %v", err)
			}
			if got := len(dispatcher.payloads); got != 0 {
				t.Fatalf("node terminal dispatches = %d, want 0", got)
			}
			if got, want := len(loopStore.wakes), 1; got != want {
				t.Fatalf("coordinator wakes = %d, want %d", got, want)
			}
			if got, want := len(loopStore.progress), 1; got != want {
				t.Fatalf("progress updates = %d, want %d", got, want)
			}
		})
	}
}

func TestLoopNativeHookObserverShouldEmitGoalTerminalAfterSettlement(t *testing.T) {
	t.Run("Should emit the blocked Goal node hook from the committed coordinator boundary", func(t *testing.T) {
		t.Parallel()

		fixedNow := time.Date(2026, 7, 10, 19, 30, 0, 0, time.UTC)
		control := looppkg.ActionControl{
			Disposition:  looppkg.ActionDispositionBlocked,
			GoalStatus:   "blocked",
			Cause:        "goal_reported_blocked",
			CheckpointID: "checkpoint-blocked",
		}
		controlPayload, err := json.Marshal(control)
		if err != nil {
			t.Fatalf("json.Marshal(control) error = %v", err)
		}
		result, err := json.Marshal(taskpkg.CoordinatorControlResult{
			Kind:    looppkg.CoordinatorControlKindLoopAction,
			Payload: controlPayload,
		})
		if err != nil {
			t.Fatalf("json.Marshal(result) error = %v", err)
		}
		loopStore := &recordingLoopHookStore{
			outputs: []looppkg.GenerationOutput{{
				Generation: 4,
				NodeID:     "converge",
				Status:     "failed",
				TaskRunID:  "run-goal-blocked",
			}},
			runs: map[string]taskpkg.Run{
				"run-goal-blocked": {
					ID:        "run-goal-blocked",
					TaskID:    "task-goal-blocked",
					RunKind:   taskpkg.RunKindWorker,
					Status:    taskpkg.TaskRunStatusCompleted,
					LoopRunID: "loop-goal-blocked",
					Result:    result,
				},
			},
			tasks: map[string]taskpkg.Task{
				"task-goal-blocked": {ID: "task-goal-blocked", Status: taskpkg.TaskStatusCompleted},
			},
		}
		dispatcher := &recordingLoopNodeTerminalDispatcher{}
		observer, err := newLoopNativeHookObserver(
			loopStore,
			dispatcher,
			&recordingLoopBackstopRunner{},
			func() time.Time { return fixedNow },
		)
		if err != nil {
			t.Fatalf("newLoopNativeHookObserver() error = %v", err)
		}

		if err := observer.OnLoopTerminal(t.Context(), hookspkg.LoopTerminalPayload{
			PayloadBase: hookspkg.PayloadBase{Event: hookspkg.HookLoopTerminal, Timestamp: fixedNow},
			LoopContext: hookspkg.LoopContext{
				LoopRunID:   "loop-goal-blocked",
				WorkspaceID: "ws-goal",
				Generation:  4,
			},
			Status: string(looppkg.StatusBlocked),
		}); err != nil {
			t.Fatalf("OnLoopTerminal() error = %v", err)
		}
		if got, want := len(dispatcher.payloads), 1; got != want {
			t.Fatalf("node terminal dispatches = %d, want %d", got, want)
		}
		payload := dispatcher.payloads[0]
		if got, want := payload.NodeID, "converge"; got != want {
			t.Fatalf("node_id = %q, want %q", got, want)
		}
		if got, want := payload.Error, "goal_reported_blocked"; got != want {
			t.Fatalf("error = %q, want %q", got, want)
		}
	})
}

func TestLoopNativeHookObserverShouldSnapshotGoalSuppressionBeforeBackstop(t *testing.T) {
	t.Run("Should emit only the settled blocked hook when the backstop advances synchronously", func(t *testing.T) {
		t.Parallel()

		fixedNow := time.Date(2026, 7, 10, 20, 0, 0, 0, time.UTC)
		control := looppkg.ActionControl{
			Disposition:  looppkg.ActionDispositionBlocked,
			GoalStatus:   "blocked",
			Cause:        looppkg.ReasonCodeGoalReportedBlocked,
			CheckpointID: "checkpoint-synchronous-backstop",
		}
		controlPayload, err := json.Marshal(control)
		if err != nil {
			t.Fatalf("json.Marshal(control) error = %v", err)
		}
		result, err := json.Marshal(taskpkg.CoordinatorControlResult{
			Kind:    looppkg.CoordinatorControlKindLoopAction,
			Payload: controlPayload,
		})
		if err != nil {
			t.Fatalf("json.Marshal(result) error = %v", err)
		}
		loopStore := &recordingLoopHookStore{
			outputStatuses: map[string]string{"loop-goal-sync/run-goal-sync": "control_pending"},
			outputs: []looppkg.GenerationOutput{{
				Generation: 3,
				NodeID:     "converge",
				Status:     "failed",
				TaskRunID:  "run-goal-sync",
			}},
			runs: map[string]taskpkg.Run{
				"run-goal-sync": {
					ID:        "run-goal-sync",
					TaskID:    "task-goal-sync",
					RunKind:   taskpkg.RunKindWorker,
					Status:    taskpkg.TaskRunStatusCompleted,
					LoopRunID: "loop-goal-sync",
					Result:    result,
				},
			},
			tasks: map[string]taskpkg.Task{
				"task-goal-sync": {ID: "task-goal-sync", Status: taskpkg.TaskStatusCompleted},
			},
		}
		dispatcher := &recordingLoopNodeTerminalDispatcher{}
		var observer *loopNativeHookObserver
		backstop := callbackLoopBackstopRunner{run: func(ctx context.Context) error {
			loopStore.outputStatuses["loop-goal-sync/run-goal-sync"] = "failed"
			return observer.OnLoopTerminal(ctx, hookspkg.LoopTerminalPayload{
				PayloadBase: hookspkg.PayloadBase{Event: hookspkg.HookLoopTerminal, Timestamp: fixedNow},
				LoopContext: hookspkg.LoopContext{
					LoopRunID:   "loop-goal-sync",
					WorkspaceID: "ws-goal-sync",
					Generation:  3,
				},
				Status: string(looppkg.StatusBlocked),
			})
		}}
		observer, err = newLoopNativeHookObserver(loopStore, dispatcher, backstop, func() time.Time {
			return fixedNow
		})
		if err != nil {
			t.Fatalf("newLoopNativeHookObserver() error = %v", err)
		}

		workerKind := taskpkg.RunKindWorker.String()
		if err := observer.OnTaskRunTerminal(t.Context(), hookspkg.TaskRunLeasePayload{
			PayloadBase: hookspkg.PayloadBase{Event: hookspkg.HookTaskRunCompleted, Timestamp: fixedNow},
			TaskRunContext: hookspkg.TaskRunContext{
				TaskID:      "task-goal-sync",
				RunID:       "run-goal-sync",
				RunKind:     &workerKind,
				LoopRunID:   "loop-goal-sync",
				WorkspaceID: "ws-goal-sync",
				RunStatus:   taskpkg.TaskRunStatusCompleted.String(),
			},
		}); err != nil {
			t.Fatalf("OnTaskRunTerminal() error = %v", err)
		}
		if got, want := len(dispatcher.payloads), 1; got != want {
			t.Fatalf("node terminal dispatches = %d, want %d", got, want)
		}
		payload := dispatcher.payloads[0]
		if payload.NodeID != "converge" || payload.Generation != 3 || len(payload.Details) == 0 {
			t.Fatalf("settled node terminal payload = %#v", payload)
		}
	})
}

type callbackLoopBackstopRunner struct {
	run func(context.Context) error
}

func (r callbackLoopBackstopRunner) RunLoopCoordinatorBackstop(
	ctx context.Context,
	_ time.Time,
	_ taskpkg.ActorContext,
) (int, error) {
	if err := r.run(ctx); err != nil {
		return 0, err
	}
	return 1, nil
}

type contextAwareLoopHookStore struct {
	recordingLoopHookStore
}

type blockingLoopNodeTerminalDispatcher struct{}

type recordingLoopNodeTerminalDispatcher struct {
	payloads []hookspkg.LoopNodeTerminalPayload
}

func (d *recordingLoopNodeTerminalDispatcher) DispatchLoopNodeTerminal(
	_ context.Context,
	payload hookspkg.LoopNodeTerminalPayload,
) (hookspkg.LoopNodeTerminalPayload, error) {
	d.payloads = append(d.payloads, payload)
	return payload, nil
}

func (blockingLoopNodeTerminalDispatcher) DispatchLoopNodeTerminal(
	ctx context.Context,
	payload hookspkg.LoopNodeTerminalPayload,
) (hookspkg.LoopNodeTerminalPayload, error) {
	<-ctx.Done()
	return payload, ctx.Err()
}

func (r *contextAwareLoopHookStore) AdvanceLoopRunProgress(
	ctx context.Context,
	loopRunID string,
	at time.Time,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.recordingLoopHookStore.AdvanceLoopRunProgress(ctx, loopRunID, at)
}

func (r *contextAwareLoopHookStore) EnqueueLoopCoordinatorWake(
	ctx context.Context,
	loopRunID string,
	idempotencyKey string,
	origin taskpkg.Origin,
	now time.Time,
) (taskpkg.Run, bool, error) {
	if err := ctx.Err(); err != nil {
		return taskpkg.Run{}, false, err
	}
	return r.recordingLoopHookStore.EnqueueLoopCoordinatorWake(ctx, loopRunID, idempotencyKey, origin, now)
}
