package task

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/store"
)

const SupervisedSilenceReason = "supervised_silence"
const SupervisedSilenceExhaustedReason = "supervised_silence_exhausted"

// SupervisedStop identifies a durable, verified session stop owned by the session manager.
type SupervisedStop struct {
	SessionID   string
	WorkspaceID string
	ProfileID   string
	EventID     string
}

type SupervisedRunRecoveryMutation struct {
	Source   Run
	Stop     SupervisedStop
	NewRunID string
	At       time.Time
}

type SupervisedRunRecoveryResult struct {
	Previous Run
	Run      Run
	Applied  bool
	Requeued bool
}

type supervisedRunRecoveryStore interface {
	RecoverSupervisedRun(context.Context, *SupervisedRunRecoveryMutation) (SupervisedRunRecoveryResult, error)
}

// RecoverSupervisedWork is called only while the verified stop receipt still fences session reuse.
func (m *Service) RecoverSupervisedWork(ctx context.Context, stop SupervisedStop) error {
	if stop.SessionID == "" || stop.WorkspaceID == "" || stop.ProfileID == "" || stop.EventID == "" {
		return fmt.Errorf("%w: supervised stop identity is required", ErrValidation)
	}
	runs, err := m.store.ListTaskRuns(ctx, RunQuery{
		SessionID: stop.SessionID, ReadScope: store.ReadScope{ProfileID: stop.ProfileID},
	})
	if err != nil {
		return err
	}
	candidates := supervisedWorkCandidates(runs, stop)
	var errs []error
	for _, candidate := range candidates {
		if err := m.recoverSupervisedRun(ctx, candidate, stop); err != nil {
			errs = append(errs, fmt.Errorf("recover supervised run %s: %w", candidate.ID, err))
		}
	}
	return errors.Join(errs...)
}

// recoverSupervisedRun commits one owned attempt so another candidate's failure cannot roll it back.
func (m *Service) recoverSupervisedRun(ctx context.Context, source Run, stop SupervisedStop) error {
	actor, err := DeriveDaemonActorContext("session-supervision", SupervisedSilenceReason)
	if err != nil {
		return err
	}
	newRunID, err := m.newID("run")
	if err != nil {
		return err
	}
	var result SupervisedRunRecoveryResult
	var event Event
	var taskRecord Task
	var transitions []StatusTransition
	at := m.now().UTC()
	err = m.store.WithTaskMutationTransaction(ctx, "recover supervised task work", func(tx runMutationStore) error {
		recoverer, ok := tx.(supervisedRunRecoveryStore)
		if !ok {
			return errors.New("task: supervised recovery store is unavailable")
		}
		var mutationErr error
		result, mutationErr = recoverer.RecoverSupervisedRun(ctx, &SupervisedRunRecoveryMutation{
			Source: source, Stop: stop, NewRunID: newRunID, At: at,
		})
		if mutationErr != nil || !result.Applied {
			return mutationErr
		}
		taskRecord, transitions, mutationErr = m.reconcileTaskSettlementCascadeWithStore(
			ctx,
			tx,
			result.Run.TaskID,
			actor,
			at,
		)
		if mutationErr != nil {
			return mutationErr
		}
		event, mutationErr = m.newTaskEventAt(
			result.Run.TaskID,
			result.Run.ID,
			taskEventRunRecovered,
			actor,
			at,
			supervisedRecoveryPayload(&result, stop),
		)
		if mutationErr != nil {
			return mutationErr
		}
		return tx.CreateTaskEvent(ctx, event)
	})
	if err != nil || !result.Applied {
		return err
	}
	m.publishLeaseSettlement(ctx, []Event{event}, transitions, actor)
	if result.Requeued {
		m.dispatchTaskRunEnqueued(ctx, result.Run, taskRecord, actor, "")
	}
	return nil
}

// supervisedRecoveryPayload correlates task history with the stable session stop without exposing lease credentials.
func supervisedRecoveryPayload(result *SupervisedRunRecoveryResult, stop SupervisedStop) map[string]any {
	action := "needs_attention"
	if result.Requeued {
		action = "requeue"
	}
	return map[string]any{
		"reason": SupervisedSilenceReason, "action": action, "stop_event_id": stop.EventID,
		"previous_run_id": result.Previous.ID, "previous_session_id": stop.SessionID,
		"previous_status": result.Previous.Status, "status": result.Run.Status,
		"attempt": result.Run.Attempt, "recovery_count": result.Run.RecoveryCount,
	}
}

// supervisedWorkCandidates keeps task workers from the profile-scoped query that still belong to the stopped session.
func supervisedWorkCandidates(runs []Run, stop SupervisedStop) []Run {
	var candidates []Run
	for _, run := range runs {
		if run.SessionID != stop.SessionID || (run.WorkspaceID != "" && run.WorkspaceID != stop.WorkspaceID) ||
			!run.IsTaskAnchored() || run.RunKind.Normalize() != RunKindWorker {
			continue
		}
		switch run.Status.Normalize() {
		case TaskRunStatusClaimed, TaskRunStatusStarting, TaskRunStatusRunning:
			candidates = append(candidates, run)
		}
	}
	return candidates
}
