package task

import "context"

type cancelTaskRecordPlan struct {
	record               Task
	runCommands          []TerminalRunCommand
	propagatedFromTaskID string
	taskEventID          string
	skipTaskMutation     bool
}

type cancelTaskTreePlan struct {
	records []cancelTaskRecordPlan
}

func (m *Service) prepareCancelTaskTree(
	ctx context.Context,
	rootTaskID string,
	tree []Task,
	req CancelTask,
	actor ActorContext,
) (cancelTaskTreePlan, error) {
	plan := cancelTaskTreePlan{records: make([]cancelTaskRecordPlan, 0, len(tree))}
	for index, record := range tree {
		prepared, err := m.prepareCancelTaskRecord(ctx, rootTaskID, index, record, req, actor)
		if err != nil {
			return cancelTaskTreePlan{}, err
		}
		plan.records = append(plan.records, prepared)
	}
	return plan, nil
}

func (m *Service) prepareCancelTaskRecord(
	ctx context.Context,
	rootTaskID string,
	index int,
	record Task,
	req CancelTask,
	actor ActorContext,
) (cancelTaskRecordPlan, error) {
	runs, status, err := m.loadTaskRuntimeState(ctx, record)
	if err != nil {
		return cancelTaskRecordPlan{}, err
	}
	record.Status = status
	prepared := cancelTaskRecordPlan{
		record:               record,
		propagatedFromTaskID: cancellationPropagationRoot(rootTaskID, index),
	}
	if index > 0 && isTerminalTaskStatus(status) {
		prepared.skipTaskMutation = true
		return prepared, nil
	}
	for _, run := range runs {
		if isTerminalRunStatus(run.Status) {
			continue
		}
		command, err := m.prepareCancellationTerminalRunCommand(
			record,
			run,
			CancelRun(req),
			actor,
			cancelRunOptions{
				propagatedFromTaskID: prepared.propagatedFromTaskID,
				reconcileTask:        false,
			},
		)
		if err != nil {
			return cancelTaskRecordPlan{}, err
		}
		prepared.runCommands = append(prepared.runCommands, command)
	}
	if status.Normalize() == TaskStatusCanceled && len(prepared.runCommands) == 0 {
		prepared.skipTaskMutation = true
		return prepared, nil
	}
	prepared.taskEventID, err = m.reserveTaskEventID()
	if err != nil {
		return cancelTaskRecordPlan{}, err
	}
	return prepared, nil
}

func (m *Service) executeCancelTaskTree(
	ctx context.Context,
	plan cancelTaskTreePlan,
	root Task,
	req CancelTask,
	actor ActorContext,
) (*Task, error) {
	canceledRoot := root
	for index := range plan.records {
		record, err := m.executeCancelTaskRecord(ctx, &plan.records[index], req, actor)
		if err != nil {
			return nil, err
		}
		if record.ID == root.ID {
			canceledRoot = record
		}
	}
	return &canceledRoot, nil
}

func (m *Service) executeCancelTaskRecord(
	ctx context.Context,
	plan *cancelTaskRecordPlan,
	req CancelTask,
	actor ActorContext,
) (Task, error) {
	if plan.skipTaskMutation {
		return plan.record, nil
	}
	canceledRunIDs := make([]string, 0, len(plan.runCommands))
	for _, command := range plan.runCommands {
		canceledRun, err := m.executeCancellationTerminalRunCommand(ctx, command)
		if err != nil {
			return Task{}, err
		}
		canceledRunIDs = append(canceledRunIDs, canceledRun.ID)
	}
	return m.persistCancelledTask(
		ctx,
		plan.record,
		req,
		actor,
		plan.propagatedFromTaskID,
		canceledRunIDs,
		plan.taskEventID,
	)
}
