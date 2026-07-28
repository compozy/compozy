package task

import (
	"context"
	"strings"
)

// RunDetail returns one persisted run and its task reference when the run is task-backed.
func (m *Service) RunDetail(
	ctx context.Context,
	runID string,
	actor ActorContext,
) (*RunDetailView, error) {
	if err := requireReadAuthority(actor); err != nil {
		return nil, err
	}

	run, err := m.loadRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	taskRecord, err := m.runDetailTask(ctx, run)
	if err != nil {
		return nil, err
	}
	if err := m.runReadAuthorizer.AuthorizeRunRead(ctx, actor, run, taskRecord); err != nil {
		return nil, err
	}
	taskReference, err := m.runDetailTaskReference(ctx, taskRecord)
	if err != nil {
		return nil, err
	}

	session := baseRunSessionRef(run.SessionID)
	summary := RunOperationalSummary{}
	if m.runtimeViews != nil && strings.TrimSpace(run.SessionID) != "" {
		if enriched, runtimeErr := m.runtimeViews.GetSession(ctx, run.SessionID); runtimeErr == nil && enriched != nil {
			session = enriched
		}
		summary = m.bestEffortRunOperationalSummary(ctx, run.SessionID)
	}

	return &RunDetailView{
		Run:     run,
		Task:    taskReference,
		Session: session,
		Summary: summary,
	}, nil
}

func (m *Service) runDetailTask(ctx context.Context, run Run) (*Task, error) {
	if strings.TrimSpace(run.TaskID) == "" {
		return nil, nil
	}

	taskRecord, err := m.store.GetTask(ctx, run.TaskID)
	if err != nil {
		return nil, err
	}
	return &taskRecord, nil
}

func (m *Service) runDetailTaskReference(ctx context.Context, taskRecord *Task) (*Reference, error) {
	if taskRecord == nil {
		return nil, nil
	}

	dependencies, err := m.store.ListDependencies(ctx, taskRecord.ID)
	if err != nil {
		return nil, err
	}
	runs, err := m.store.ListTaskRuns(ctx, RunQuery{TaskID: taskRecord.ID})
	if err != nil {
		return nil, err
	}
	status, err := m.canonicalTaskStatus(ctx, *taskRecord, dependencies, runs)
	if err != nil {
		return nil, err
	}

	reference := taskReferenceFromTask(*taskRecord, status)
	return &reference, nil
}
