package task

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/network/participation"
)

func (m *Service) settleTerminalRunCommand(
	ctx context.Context,
	command TerminalRunCommand,
) (terminalRunCommandSettlement, error) {
	settlement := terminalRunCommandSettlement{command: command}
	err := m.store.WithTaskMutationTransaction(
		ctx,
		terminalRunCommandAction(command.Kind()),
		func(store runMutationStore) error {
			consumed, err := store.ConsumeTerminalRunCommand(
				ctx,
				command,
				command.requiredSettlementPhase(),
			)
			if err != nil {
				return err
			}
			if !consumed.sameIntent(command) {
				return fmt.Errorf(
					"%w: task run %q terminal receipt changed before settlement",
					ErrTerminalRunCommandInProgress,
					command.RunID(),
				)
			}
			settlement.command = consumed
			current, err := store.GetTaskRun(ctx, consumed.RunID())
			if err != nil {
				return err
			}
			switch consumed.Kind() {
			case TerminalRunCommandCompleted:
				return m.settleCompletedTerminalRunCommand(ctx, store, current, &settlement)
			case TerminalRunCommandFailed:
				return m.settleFailedTerminalRunCommand(ctx, store, current, &settlement)
			case TerminalRunCommandCanceled:
				return m.settleCanceledTerminalRunCommand(ctx, store, current, &settlement)
			case TerminalRunCommandNeedsAttention:
				return m.settleNeedsAttentionTerminalRunCommand(ctx, store, current, &settlement)
			default:
				return fmt.Errorf(
					"%w: unsupported terminal run command kind %q",
					ErrValidation,
					consumed.Kind(),
				)
			}
		},
	)
	if err != nil {
		return terminalRunCommandSettlement{}, err
	}
	return settlement, nil
}

func (m *Service) settleCompletedTerminalRunCommand(
	ctx context.Context,
	store runMutationStore,
	current Run,
	settlement *terminalRunCommandSettlement,
) error {
	command := settlement.command
	mutation, err := command.terminalMutation(current)
	if err != nil {
		return err
	}
	completed, err := m.completeRunMutationSettlementWithStore(
		ctx,
		store,
		mutation,
		command.Actor(),
	)
	if err != nil {
		return err
	}
	eventID, err := command.eventID(0)
	if err != nil {
		return err
	}
	event, err := m.newTaskEventWithIDAt(
		eventID,
		completed.Run.TaskID,
		completed.Run.ID,
		taskEventRunCompleted,
		command.Actor(),
		completed.Run.EndedAt,
		completedRunPayload{
			Status:     completed.Run.Status,
			TaskStatus: completed.Task.Status,
			Result:     completed.Run.ResultValue(),
		},
	)
	if err != nil {
		return err
	}
	if err := store.CreateTaskEvent(ctx, event); err != nil {
		return err
	}
	settlement.run = completed.Run
	settlement.task = completed.Task
	settlement.statusTransitions = completed.StatusTransitions
	settlement.rolledUpRuns = completed.RolledUpRuns
	settlement.events = []Event{event}
	return nil
}

func (m *Service) settleFailedTerminalRunCommand(
	ctx context.Context,
	store runMutationStore,
	current Run,
	settlement *terminalRunCommandSettlement,
) error {
	command := settlement.command
	mutation, err := command.terminalMutation(current)
	if err != nil {
		return err
	}
	settledRun, err := store.TransitionTerminalRun(ctx, mutation)
	if err != nil {
		return err
	}
	reconciledTask, transitions, err := m.reconcileTaskSettlementCascadeWithStore(
		ctx,
		store,
		settledRun.TaskID,
		command.Actor(),
		settledRun.EndedAt,
	)
	if err != nil {
		return err
	}
	failure := command.failure()
	eventID, err := command.eventID(0)
	if err != nil {
		return err
	}
	event, err := m.newTaskEventWithIDAt(
		eventID,
		settledRun.TaskID,
		settledRun.ID,
		taskEventRunFailed,
		command.Actor(),
		settledRun.EndedAt,
		failedRunPayload{
			Status:     settledRun.Status,
			TaskStatus: reconciledTask.Status,
			Error:      settledRun.Error,
			Metadata:   cloneRawJSON(failure.Metadata),
		},
	)
	if err != nil {
		return err
	}
	if err := store.CreateTaskEvent(ctx, event); err != nil {
		return err
	}
	events := []Event{event}
	recoveryEvent, err := m.recordFailedTerminalRecoveryEvent(
		ctx,
		store,
		&command,
		&settledRun,
		&reconciledTask,
	)
	if err != nil {
		return err
	}
	if recoveryEvent != nil {
		events = append(events, *recoveryEvent)
	}
	settlement.run = settledRun
	settlement.task = reconciledTask
	settlement.statusTransitions = transitions
	settlement.events = events
	return nil
}

func (m *Service) recordFailedTerminalRecoveryEvent(
	ctx context.Context,
	store runMutationStore,
	command *TerminalRunCommand,
	settledRun *Run,
	reconciledTask *Task,
) (*Event, error) {
	audit := command.recoveryAudit()
	if audit == nil {
		return nil, nil
	}
	recoveryEventID, err := command.eventID(1)
	if err != nil {
		return nil, err
	}
	recoveryEvent, err := m.newTaskEventWithIDAt(
		recoveryEventID,
		settledRun.TaskID,
		settledRun.ID,
		taskEventRunRecovered,
		command.Actor(),
		settledRun.EndedAt,
		recoveredRunEventPayload(
			audit.recovery,
			audit.previousStatus,
			audit.previousSessionID,
		)(&NominalRunMutationResult{Run: *settledRun}, *reconciledTask),
	)
	if err != nil {
		return nil, err
	}
	if err := store.CreateTaskEvent(ctx, recoveryEvent); err != nil {
		return nil, err
	}
	return &recoveryEvent, nil
}

func (m *Service) settleCanceledTerminalRunCommand(
	ctx context.Context,
	store runMutationStore,
	current Run,
	settlement *terminalRunCommandSettlement,
) error {
	command := settlement.command
	mutation, err := command.terminalMutation(current)
	if err != nil {
		return err
	}
	req, opts, err := command.cancellation()
	if err != nil {
		return err
	}
	result, err := m.persistCanceledRun(
		ctx,
		store,
		mutation,
		req,
		command.Actor(),
		opts,
		command.StopRequired(),
		command.StopRequired(),
		command.intent.EventIDs,
	)
	if err != nil {
		return err
	}
	settlement.run = result.Run
	settlement.task = result.Task
	settlement.statusTransitions = result.StatusTransitions
	settlement.events = result.Events
	return nil
}

func (m *Service) settleNeedsAttentionTerminalRunCommand(
	ctx context.Context,
	store runMutationStore,
	current Run,
	settlement *terminalRunCommandSettlement,
) error {
	command := settlement.command
	if !command.Fence().Matches(current) {
		return fmt.Errorf(
			"%w: task run %q changed after needs-attention admission",
			ErrInvalidStatusTransition,
			command.RunID(),
		)
	}
	mutation, err := store.MarkRunNeedsAttentionMutation(
		ctx,
		NewRunNeedsAttentionCommand(current, command.diagnostic(), command.CommandAt()),
	)
	if err != nil {
		return err
	}
	if !mutation.Applied {
		return fmt.Errorf(
			"%w: task run %q needs-attention command was not applied",
			ErrInvalidStatusTransition,
			command.RunID(),
		)
	}
	eventID, err := command.eventID(0)
	if err != nil {
		return err
	}
	event, err := m.newTaskEventWithIDAt(
		eventID,
		mutation.Run.TaskID,
		mutation.Run.ID,
		taskEventRunNeedsAttention,
		command.Actor(),
		command.CommandAt(),
		RunNeedsAttentionEventPayload{
			PreviousStatus:               mutation.Previous.Status.Normalize(),
			Status:                       mutation.Run.Status.Normalize(),
			SessionID:                    mutation.Run.SessionID,
			Diagnostic:                   command.diagnostic(),
			QueuedAt:                     mutation.Run.QueuedAt,
			ResolvedNetworkParticipation: participation.CloneSpec(mutation.Run.NetworkSpecSnapshot()),
		},
	)
	if err != nil {
		return err
	}
	if err := store.CreateTaskEvent(ctx, event); err != nil {
		return err
	}
	settlement.run = mutation.Run
	settlement.events = []Event{event}
	return nil
}
