package task

import (
	"context"
	"fmt"
)

type failRunRecordOptions struct {
	stopTerminalSession bool
	recoveryAudit       *bootRecoveryAudit
}

type bootRecoveryAudit struct {
	recovery          RunBootRecovery
	previousStatus    RunStatus
	previousSessionID string
}

func (m *Service) failRunRecord(
	ctx context.Context,
	taskRecord Task,
	run Run,
	failure RunFailure,
	actor ActorContext,
) (*Run, error) {
	return m.failRunRecordWithOptions(ctx, taskRecord, run, failure, actor, failRunRecordOptions{
		stopTerminalSession: true,
	})
}

func (m *Service) failRunRecordWithOptions(
	ctx context.Context,
	taskRecord Task,
	run Run,
	failure RunFailure,
	actor ActorContext,
	opts failRunRecordOptions,
) (*Run, error) {
	normalizedFailure, err := normalizeRunFailure(failure)
	if err != nil {
		return nil, err
	}
	switch run.Status.Normalize() {
	case TaskRunStatusStarting, TaskRunStatusRunning:
	default:
		return nil, requireRunTransition(run, TaskRunStatusFailed)
	}
	commandAt := m.now().UTC()
	if err := m.preflightTaskEvent(
		run.TaskID,
		run.ID,
		taskEventRunFailed,
		actor,
		failedRunPayload{
			Status: TaskRunStatusFailed, TaskStatus: taskRecord.Status,
			Error: normalizedFailure.Error, Metadata: cloneRawJSON(normalizedFailure.Metadata),
		},
	); err != nil {
		return nil, err
	}
	if opts.recoveryAudit != nil {
		preview := run
		preview.Status = TaskRunStatusFailed
		preview.Error = normalizedFailure.Error
		preview.EndedAt = commandAt
		if err := m.preflightTaskEvent(
			preview.TaskID,
			preview.ID,
			taskEventRunRecovered,
			actor,
			commandRecoveryEventPayload(opts.recoveryAudit, preview, taskRecord),
		); err != nil {
			return nil, err
		}
	}
	commandID, err := m.newID("terminal-command")
	if err != nil {
		return nil, fmt.Errorf("task: generate terminal command id: %w", err)
	}
	eventCount := 1
	if opts.recoveryAudit != nil {
		eventCount++
	}
	eventIDs, err := m.reserveTaskEventIDs(eventCount)
	if err != nil {
		return nil, err
	}
	command, err := newFailureTerminalRunCommand(
		commandID,
		eventIDs,
		run,
		normalizedFailure,
		actor,
		commandAt,
		opts.stopTerminalSession,
		opts.recoveryAudit,
	)
	if err != nil {
		return nil, err
	}
	settlement, err := m.executeTerminalRunCommand(ctx, command)
	if err != nil {
		return nil, err
	}
	if err := m.publishTerminalRunCommandSettlement(ctx, &settlement); err != nil {
		return &settlement.run, err
	}
	return &settlement.run, nil
}

func commandRecoveryEventPayload(
	audit *bootRecoveryAudit,
	run Run,
	taskRecord Task,
) any {
	if audit == nil {
		return nil
	}
	return recoveredRunEventPayload(
		audit.recovery,
		audit.previousStatus,
		audit.previousSessionID,
	)(&NominalRunMutationResult{Run: run}, taskRecord)
}
