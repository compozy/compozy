package task

import "context"

func (m *Service) loadTaskLiveContext(
	ctx context.Context,
	taskID string,
	cache map[string]*taskLiveContext,
) (*taskLiveContext, error) {
	if cached, ok := cache[taskID]; ok {
		return cached, nil
	}

	record, err := m.store.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	dependencies, err := m.store.ListDependencies(ctx, record.ID)
	if err != nil {
		return nil, err
	}
	runs, err := m.store.ListTaskRuns(ctx, RunQuery{TaskID: record.ID})
	if err != nil {
		return nil, err
	}
	status, err := m.canonicalTaskStatus(ctx, record, dependencies, runs)
	if err != nil {
		return nil, err
	}

	runsByID := make(map[string]Run, len(runs))
	for _, run := range runs {
		runsByID[run.ID] = run
	}

	cached := &taskLiveContext{
		record:    record,
		reference: taskReferenceFromTask(record, status),
		runsByID:  runsByID,
	}
	cache[taskID] = cached
	return cached, nil
}
