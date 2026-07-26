package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/network/participation"
	taskpkg "github.com/compozy/agh/internal/task"
)

type loopHookCoordinatorStore interface {
	AdvanceLoopRunProgress(ctx context.Context, loopRunID string, at time.Time) error
	EnqueueLoopCoordinatorWake(
		ctx context.Context,
		loopRunID string,
		idempotencyKey string,
		origin taskpkg.Origin,
		now time.Time,
	) (taskpkg.Run, bool, error)
	PromoteOldestQueuedLoopRun(
		ctx context.Context,
		workspaceID string,
		loopName string,
		origin taskpkg.Origin,
		now time.Time,
	) (taskpkg.Run, bool, error)
	LookupLoopGenerationOutputStatus(
		ctx context.Context,
		loopRunID string,
		taskRunID string,
	) (string, bool, error)
	ListGenerationOutputs(
		ctx context.Context,
		runID looppkg.RunID,
		generation int,
	) ([]looppkg.GenerationOutput, error)
	GetTaskRun(ctx context.Context, id string) (taskpkg.Run, error)
	GetTask(ctx context.Context, id string) (taskpkg.Task, error)
}

type loopNodeTerminalDispatcher interface {
	DispatchLoopNodeTerminal(
		context.Context,
		hookspkg.LoopNodeTerminalPayload,
	) (hookspkg.LoopNodeTerminalPayload, error)
}

type loopCoordinatorBackstopRunner interface {
	RunLoopCoordinatorBackstop(context.Context, time.Time, taskpkg.ActorContext) (int, error)
}

type loopNativeHookObserver struct {
	store      loopHookCoordinatorStore
	dispatcher loopNodeTerminalDispatcher
	backstop   loopCoordinatorBackstopRunner
	actor      taskpkg.ActorContext
	now        func() time.Time
}

var _ loopStartedObserver = (*loopNativeHookObserver)(nil)
var _ taskRunTerminalObserver = (*loopNativeHookObserver)(nil)
var _ loopTerminalObserver = (*loopNativeHookObserver)(nil)

func newLoopNativeHookObserver(
	store loopHookCoordinatorStore,
	dispatcher loopNodeTerminalDispatcher,
	backstop loopCoordinatorBackstopRunner,
	now func() time.Time,
) (*loopNativeHookObserver, error) {
	if store == nil {
		return nil, fmt.Errorf("daemon: loop native hook observer requires coordinator store")
	}
	if dispatcher == nil {
		return nil, fmt.Errorf("daemon: loop native hook observer requires loop dispatcher")
	}
	if backstop == nil {
		return nil, fmt.Errorf("daemon: loop native hook observer requires coordinator backstop")
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	actor, err := taskpkg.DeriveDaemonActorContext("loop-hook-observer", "loop.hooks")
	if err != nil {
		return nil, fmt.Errorf("daemon: derive loop hook actor: %w", err)
	}
	return &loopNativeHookObserver{
		store:      store,
		dispatcher: dispatcher,
		backstop:   backstop,
		actor:      actor,
		now:        now,
	}, nil
}

func (o *loopNativeHookObserver) OnLoopStarted(
	ctx context.Context,
	payload hookspkg.LoopStartedPayload,
) error {
	if looppkg.Status(strings.TrimSpace(payload.Status)) != looppkg.StatusRunning {
		return nil
	}
	return o.startLoopCoordinatorBackstop(ctx)
}

func (o *loopNativeHookObserver) OnTaskRunTerminal(
	ctx context.Context,
	payload hookspkg.TaskRunLeasePayload,
) error {
	loopRunID := strings.TrimSpace(payload.LoopRunID)
	if loopRunID == "" || optionalRunKind(payload.RunKind) == taskpkg.RunKindCoordinator.String() {
		return nil
	}

	var errs []error
	loopPayload := hookspkg.LoopNodeTerminalPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookLoopNodeTerminal,
			Timestamp: payload.Timestamp,
		},
		LoopContext: hookspkg.LoopContext{
			LoopRunID:                    loopRunID,
			WorkspaceID:                  strings.TrimSpace(payload.WorkspaceID),
			TaskID:                       strings.TrimSpace(payload.TaskID),
			RunID:                        strings.TrimSpace(payload.RunID),
			RunKind:                      optionalRunKind(payload.RunKind),
			WorkflowID:                   strings.TrimSpace(payload.WorkflowID),
			ResolvedNetworkParticipation: participation.CloneSpec(payload.NetworkSpecSnapshot()),
			AgentName:                    strings.TrimSpace(payload.AgentName),
			SessionID:                    strings.TrimSpace(payload.SessionID),
			ActorKind:                    strings.TrimSpace(payload.ActorKind),
			ActorID:                      strings.TrimSpace(payload.ActorID),
			OriginKind:                   strings.TrimSpace(payload.OriginKind),
			OriginRef:                    strings.TrimSpace(payload.OriginRef),
		},
		TaskStatus: strings.TrimSpace(payload.TaskStatus),
		RunStatus:  strings.TrimSpace(payload.RunStatus),
		Error:      strings.TrimSpace(payload.Error),
	}
	if err := o.store.AdvanceLoopRunProgress(ctx, loopRunID, payload.Timestamp); err != nil {
		errs = append(errs, err)
	}
	suppress, suppressErr := o.suppressIntermediateGoalTerminal(ctx, loopRunID, payload.RunID)
	if suppressErr != nil {
		errs = append(errs, suppressErr)
	}
	if err := o.enqueueNodeTerminalWake(ctx, payload); err != nil {
		errs = append(errs, err)
	}
	if !suppress && suppressErr == nil {
		_, err := o.dispatcher.DispatchLoopNodeTerminal(ctx, loopPayload)
		if err != nil {
			errs = append(errs, fmt.Errorf("dispatch loop.node.terminal: %w", err))
		}
	}
	return errors.Join(errs...)
}

func (o *loopNativeHookObserver) suppressIntermediateGoalTerminal(
	ctx context.Context,
	loopRunID string,
	taskRunID string,
) (bool, error) {
	status, found, err := o.store.LookupLoopGenerationOutputStatus(ctx, loopRunID, strings.TrimSpace(taskRunID))
	if err != nil {
		return true, fmt.Errorf("lookup loop generation output status: %w", err)
	}
	if !found {
		return false, nil
	}
	return status == looppkg.GenerationOutputStatusControlPending ||
		status == looppkg.GenerationOutputStatusAwaitingGoal, nil
}

func (o *loopNativeHookObserver) enqueueNodeTerminalWake(
	ctx context.Context,
	payload hookspkg.TaskRunLeasePayload,
) error {
	_, added, err := o.store.EnqueueLoopCoordinatorWake(
		ctx,
		strings.TrimSpace(payload.LoopRunID),
		loopNodeTerminalWakeKey(payload),
		o.actor.Origin,
		o.now().UTC(),
	)
	if err := normalizeLoopWakeError(err); err != nil {
		return err
	}
	if added {
		return o.startLoopCoordinatorBackstop(ctx)
	}
	return nil
}

func (o *loopNativeHookObserver) OnLoopTerminal(
	ctx context.Context,
	payload hookspkg.LoopTerminalPayload,
) error {
	var errs []error
	if err := o.dispatchSettledGoalNodeTerminals(ctx, payload); err != nil {
		errs = append(errs, err)
	}
	parentWakeAdded, err := o.enqueueParentAwaitWake(ctx, payload)
	if err != nil {
		errs = append(errs, err)
	}
	queuedPromotionAdded, err := o.promoteQueuedLoop(ctx, payload)
	if err != nil {
		errs = append(errs, err)
	}
	if parentWakeAdded || queuedPromotionAdded {
		if err := o.startLoopCoordinatorBackstop(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (o *loopNativeHookObserver) dispatchSettledGoalNodeTerminals(
	ctx context.Context,
	payload hookspkg.LoopTerminalPayload,
) error {
	loopStatus := looppkg.Status(strings.TrimSpace(payload.Status))
	if loopStatus != looppkg.StatusBlocked && loopStatus != looppkg.StatusExhausted {
		return nil
	}
	if payload.Generation <= 0 {
		return fmt.Errorf("daemon: settled Goal node hook requires a positive generation")
	}
	outputs, err := o.store.ListGenerationOutputs(ctx, looppkg.RunID(payload.LoopRunID), payload.Generation)
	if err != nil {
		return fmt.Errorf("daemon: list settled Goal outputs: %w", err)
	}
	var errs []error
	for _, output := range outputs {
		if output.Status != hookAgentEventsFailedKey || strings.TrimSpace(output.TaskRunID) == "" {
			continue
		}
		if err := o.dispatchSettledGoalNodeTerminal(ctx, payload, output); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (o *loopNativeHookObserver) dispatchSettledGoalNodeTerminal(
	ctx context.Context,
	payload hookspkg.LoopTerminalPayload,
	output looppkg.GenerationOutput,
) error {
	run, err := o.store.GetTaskRun(ctx, strings.TrimSpace(output.TaskRunID))
	if err != nil {
		return fmt.Errorf("daemon: load settled Goal task run %q: %w", output.TaskRunID, err)
	}
	if run.Status.Normalize() != taskpkg.TaskRunStatusCompleted || !isLoopActionControlEnvelope(run.Result) {
		return nil
	}
	control, err := looppkg.DecodeActionControlResult(run.Result)
	if err != nil {
		return fmt.Errorf("daemon: decode settled Goal task run %q: %w", run.ID, err)
	}
	if control.Disposition != looppkg.ActionDispositionBlocked &&
		control.Disposition != looppkg.ActionDispositionExhausted {
		return nil
	}
	taskRecord, err := o.store.GetTask(ctx, run.TaskID)
	if err != nil {
		return fmt.Errorf("daemon: load settled Goal task %q: %w", run.TaskID, err)
	}
	details, err := json.Marshal(control)
	if err != nil {
		return fmt.Errorf("daemon: marshal settled Goal control: %w", err)
	}
	nodePayload := hookspkg.LoopNodeTerminalPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookLoopNodeTerminal,
			Timestamp: payload.Timestamp,
		},
		LoopContext: hookspkg.LoopContext{
			LoopRunID:                    strings.TrimSpace(payload.LoopRunID),
			WorkspaceID:                  strings.TrimSpace(payload.WorkspaceID),
			LoopName:                     strings.TrimSpace(payload.LoopName),
			Generation:                   output.Generation,
			TaskID:                       strings.TrimSpace(run.TaskID),
			RunID:                        strings.TrimSpace(run.ID),
			RunKind:                      run.RunKind.Normalize().String(),
			NodeID:                       strings.TrimSpace(output.NodeID),
			ResolvedNetworkParticipation: participation.CloneSpec(run.NetworkSpecSnapshot()),
			SessionID:                    strings.TrimSpace(run.SessionID),
			ActorKind:                    strings.TrimSpace(payload.ActorKind),
			ActorID:                      strings.TrimSpace(payload.ActorID),
			OriginKind:                   string(run.Origin.Kind.Normalize()),
			OriginRef:                    strings.TrimSpace(run.Origin.Ref),
		},
		TaskStatus: string(taskRecord.Status.Normalize()),
		RunStatus:  run.Status.Normalize().String(),
		Error:      string(control.Cause),
		Details:    details,
	}
	if _, err := o.dispatcher.DispatchLoopNodeTerminal(ctx, nodePayload); err != nil {
		return fmt.Errorf("dispatch settled Goal loop.node.terminal: %w", err)
	}
	return nil
}

func isLoopActionControlEnvelope(raw json.RawMessage) bool {
	var envelope struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return false
	}
	return strings.TrimSpace(envelope.Kind) == looppkg.CoordinatorControlKindLoopAction
}

func (o *loopNativeHookObserver) enqueueParentAwaitWake(
	ctx context.Context,
	payload hookspkg.LoopTerminalPayload,
) (bool, error) {
	parentLoopRunID := strings.TrimSpace(payload.ParentLoopRunID)
	if parentLoopRunID == "" {
		return false, nil
	}
	_, added, err := o.store.EnqueueLoopCoordinatorWake(
		ctx,
		parentLoopRunID,
		loopParentAwaitWakeKey(payload),
		o.actor.Origin,
		o.now().UTC(),
	)
	if err := normalizeLoopWakeError(err); err != nil {
		return false, err
	}
	return added, nil
}

func (o *loopNativeHookObserver) promoteQueuedLoop(
	ctx context.Context,
	payload hookspkg.LoopTerminalPayload,
) (bool, error) {
	workspaceID := strings.TrimSpace(payload.WorkspaceID)
	loopName := strings.TrimSpace(payload.LoopName)
	if workspaceID == "" || loopName == "" {
		return false, nil
	}
	_, added, err := o.store.PromoteOldestQueuedLoopRun(ctx, workspaceID, loopName, o.actor.Origin, o.now().UTC())
	if err := normalizeLoopWakeError(err); err != nil {
		return false, err
	}
	return added, nil
}

func (o *loopNativeHookObserver) startLoopCoordinatorBackstop(ctx context.Context) error {
	_, err := o.backstop.RunLoopCoordinatorBackstop(ctx, o.now().UTC(), o.actor)
	return err
}

func optionalRunKind(runKind *string) string {
	if runKind == nil {
		return ""
	}
	return strings.TrimSpace(*runKind)
}

func loopNodeTerminalWakeKey(payload hookspkg.TaskRunLeasePayload) string {
	return fmt.Sprintf(
		"loop.coordinator.node_terminal.%s.%s",
		strings.TrimSpace(payload.LoopRunID),
		strings.TrimSpace(payload.RunID),
	)
}

func loopParentAwaitWakeKey(payload hookspkg.LoopTerminalPayload) string {
	return fmt.Sprintf(
		"loop.coordinator.child_terminal.%s.%s",
		strings.TrimSpace(payload.ParentLoopRunID),
		strings.TrimSpace(payload.LoopRunID),
	)
}

func normalizeLoopWakeError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, taskpkg.ErrInvalidStatusTransition), errors.Is(err, taskpkg.ErrConflict):
		return nil
	default:
		return err
	}
}
