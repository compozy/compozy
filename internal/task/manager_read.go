package task

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// GetTask returns one expanded task view after enforcing read authority.
func (m *Service) GetTask(ctx context.Context, id string, actor ActorContext) (*View, error) {
	if err := requireReadAuthority(actor); err != nil {
		return nil, err
	}

	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return nil, fmt.Errorf("%w: task id is required", ErrValidation)
	}

	record, err := m.loadAuthorizedTask(ctx, m.store, trimmedID, actor)
	if err != nil {
		return nil, err
	}

	children, err := m.listTaskSummaries(ctx, Query{ParentTaskID: trimmedID})
	if err != nil {
		return nil, err
	}
	children = m.filterAuthorizedTaskSummaries(ctx, actor, children)
	dependencies, err := m.store.ListDependencies(ctx, trimmedID)
	if err != nil {
		return nil, err
	}
	runs, err := m.store.ListTaskRuns(ctx, RunQuery{TaskID: trimmedID})
	if err != nil {
		return nil, err
	}
	events, err := m.store.ListTaskEvents(ctx, EventQuery{TaskID: trimmedID})
	if err != nil {
		return nil, err
	}

	summary, err := m.enrichTaskSummaryFromState(
		ctx,
		record,
		len(children),
		dependencies,
		runs,
		events,
	)
	if err != nil {
		return nil, err
	}
	visibleDependencies, err := m.filterAuthorizedDependencies(ctx, actor, dependencies)
	if err != nil {
		return nil, err
	}
	visibleRuns, err := m.filterAuthorizedRuns(ctx, actor, record, runs)
	if err != nil {
		return nil, err
	}
	visibleEvents, err := m.filterAuthorizedEvents(ctx, actor, record, events)
	if err != nil {
		return nil, err
	}
	dependencyReferences, err := m.buildDependencyReferences(ctx, visibleDependencies)
	if err != nil {
		return nil, err
	}
	summary.ChildCount = ClampSummaryCount(len(children))
	summary.DependencyCount = ClampSummaryCount(len(visibleDependencies))
	summary.Dependencies = dependencyReferences
	summary.ActiveRun = activeRunSummary(visibleRuns, record.MaxAttempts)
	summary.BlockedReasons = filterAuthorizedBlockedReasons(summary.BlockedReasons, visibleDependencies)
	if strings.TrimSpace(record.CurrentRunID) != "" && !containsRunID(visibleRuns, record.CurrentRunID) {
		record.CurrentRunID = ""
		summary.CurrentRunID = ""
	}

	view := &View{
		Summary:              summary,
		Task:                 record,
		Children:             children,
		Dependencies:         visibleDependencies,
		DependencyReferences: dependencyReferences,
		Runs:                 visibleRuns,
		Events:               visibleEvents,
	}
	view.Task.Status = summary.Status
	return view, nil
}

// ListTaskRuns returns task runs for one task after enforcing read authority and
// task existence.
func (m *Service) ListTaskRuns(
	ctx context.Context,
	taskID string,
	query RunQuery,
	actor ActorContext,
) ([]Run, error) {
	if err := requireReadAuthority(actor); err != nil {
		return nil, err
	}

	trimmedID := strings.TrimSpace(taskID)
	if trimmedID == "" {
		return nil, fmt.Errorf("%w: task id is required", ErrValidation)
	}

	taskRecord, err := m.loadAuthorizedTask(ctx, m.store, trimmedID, actor)
	if err != nil {
		return nil, err
	}

	normalizedQuery := query
	normalizedQuery.TaskID = trimmedID
	runs, err := m.store.ListTaskRuns(ctx, normalizedQuery)
	if err != nil {
		return nil, err
	}
	return m.filterAuthorizedRuns(ctx, actor, taskRecord, runs)
}

func (m *Service) filterAuthorizedTaskSummaries(
	ctx context.Context,
	actor ActorContext,
	summaries []Summary,
) []Summary {
	if isTaskOperator(actor) {
		return summaries
	}
	visible := make([]Summary, 0, len(summaries))
	for idx := range summaries {
		if err := m.authorizeTaskResource(ctx, actor, taskRecordFromSummary(&summaries[idx])); err == nil {
			visible = append(visible, summaries[idx])
		}
	}
	return visible
}

func (m *Service) filterAuthorizedRuns(
	ctx context.Context,
	actor ActorContext,
	taskRecord Task,
	runs []Run,
) ([]Run, error) {
	if isTaskOperator(actor) {
		return runs, nil
	}
	visible := make([]Run, 0, len(runs))
	for _, run := range runs {
		err := m.runReadAuthorizer.AuthorizeRunRead(ctx, actor, run, &taskRecord)
		if err == nil {
			visible = append(visible, run)
			continue
		}
		if !errors.Is(err, ErrTaskRunNotFound) && !errors.Is(err, ErrPermissionDenied) {
			return nil, err
		}
	}
	return visible, nil
}

func (m *Service) filterAuthorizedEvents(
	ctx context.Context,
	actor ActorContext,
	taskRecord Task,
	events []Event,
) ([]Event, error) {
	if isTaskOperator(actor) {
		return events, nil
	}
	visible := make([]Event, 0, len(events))
	for _, event := range events {
		if strings.TrimSpace(event.RunID) == "" {
			visible = append(visible, event)
			continue
		}
		run, err := m.store.GetTaskRun(ctx, event.RunID)
		if err != nil {
			return nil, err
		}
		if err := m.runReadAuthorizer.AuthorizeRunRead(ctx, actor, run, &taskRecord); err == nil {
			visible = append(visible, event)
		} else if !errors.Is(err, ErrTaskRunNotFound) && !errors.Is(err, ErrPermissionDenied) {
			return nil, err
		}
	}
	return visible, nil
}

func (m *Service) filterAuthorizedDependencies(
	ctx context.Context,
	actor ActorContext,
	dependencies []Dependency,
) ([]Dependency, error) {
	if isTaskOperator(actor) {
		return dependencies, nil
	}
	visible := make([]Dependency, 0, len(dependencies))
	for _, dependency := range dependencies {
		dependsOn, err := m.store.GetTask(ctx, dependency.DependsOnTaskID)
		if err != nil {
			return nil, err
		}
		if err := m.authorizeTaskResource(ctx, actor, dependsOn); err == nil {
			visible = append(visible, dependency)
		} else if !errors.Is(err, ErrPermissionDenied) {
			return nil, err
		}
	}
	return visible, nil
}

func filterAuthorizedBlockedReasons(
	reasons *[]BlockedReason,
	dependencies []Dependency,
) *[]BlockedReason {
	if reasons == nil {
		return nil
	}
	visibleDependencyIDs := make(map[string]struct{}, len(dependencies))
	for _, dependency := range dependencies {
		visibleDependencyIDs[dependency.DependsOnTaskID] = struct{}{}
	}
	filtered := make([]BlockedReason, 0, len(*reasons))
	for _, reason := range *reasons {
		if len(reason.DependsOnTaskIDs) == 0 {
			filtered = append(filtered, reason)
			continue
		}
		visibleIDs := make([]string, 0, len(reason.DependsOnTaskIDs))
		for _, taskID := range reason.DependsOnTaskIDs {
			if _, ok := visibleDependencyIDs[taskID]; ok {
				visibleIDs = append(visibleIDs, taskID)
			}
		}
		if len(visibleIDs) > 0 {
			reason.DependsOnTaskIDs = visibleIDs
			filtered = append(filtered, reason)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	return &filtered
}

func containsRunID(runs []Run, runID string) bool {
	for _, run := range runs {
		if strings.TrimSpace(run.ID) == strings.TrimSpace(runID) {
			return true
		}
	}
	return false
}

// ListTasks returns task summaries that satisfy the supplied query filters
// after enforcing read authority.
func (m *Service) ListTasks(
	ctx context.Context,
	query Query,
	actor ActorContext,
) ([]Summary, error) {
	if err := requireReadAuthority(actor); err != nil {
		return nil, err
	}
	summaries, err := m.listTaskSummaries(ctx, query)
	if err != nil {
		return nil, err
	}
	if isTaskOperator(actor) {
		return summaries, nil
	}
	visible := make([]Summary, 0, len(summaries))
	for idx := range summaries {
		if err := m.authorizeTaskResource(ctx, actor, taskRecordFromSummary(&summaries[idx])); err == nil {
			visible = append(visible, summaries[idx])
		}
	}
	return visible, nil
}

func (m *Service) listTaskSummaries(ctx context.Context, query Query) ([]Summary, error) {
	summaries, err := m.store.ListTasks(ctx, query)
	if err != nil {
		return nil, err
	}

	enriched := make([]Summary, 0, len(summaries))
	for idx := range summaries {
		item, err := m.enrichTaskSummary(ctx, &summaries[idx])
		if err != nil {
			return nil, err
		}
		enriched = append(enriched, item)
	}
	return enriched, nil
}

func (m *Service) enrichTaskSummary(ctx context.Context, summary *Summary) (Summary, error) {
	childCount, err := m.store.CountDirectChildren(ctx, summary.ID)
	if err != nil {
		return Summary{}, err
	}
	dependencies, err := m.store.ListDependencies(ctx, summary.ID)
	if err != nil {
		return Summary{}, err
	}
	runs, err := m.store.ListTaskRuns(ctx, RunQuery{TaskID: summary.ID})
	if err != nil {
		return Summary{}, err
	}
	events, err := m.store.ListTaskEvents(ctx, EventQuery{TaskID: summary.ID, Limit: 1})
	if err != nil {
		return Summary{}, err
	}
	return m.enrichTaskSummaryFromState(
		ctx,
		taskRecordFromSummary(summary),
		childCount,
		dependencies,
		runs,
		events,
	)
}

func (m *Service) enrichTaskSummaryFromState(
	ctx context.Context,
	record Task,
	childCount int,
	dependencies []Dependency,
	runs []Run,
	events []Event,
) (Summary, error) {
	status, err := m.canonicalTaskStatus(ctx, record, dependencies, runs)
	if err != nil {
		return Summary{}, err
	}

	summary := summaryFromTaskRecord(record)
	summary.Status = status
	summary.Draft = status == TaskStatusDraft
	summary.ChildCount = ClampSummaryCount(childCount)
	summary.DependencyCount = ClampSummaryCount(len(dependencies))
	summary.ActiveRun = activeRunSummary(runs, record.MaxAttempts)
	summary.LastActivityAt = latestTaskActivityAt(record, runs, events)
	blockedReasons, err := m.blockedReasonsForSnapshot(ctx, record, dependencies)
	if err != nil {
		return Summary{}, err
	}
	if len(blockedReasons) > 0 {
		summary.BlockedReasons = &blockedReasons
	}
	if err := m.applyEffectivePauseToSummary(ctx, &summary); err != nil {
		return Summary{}, err
	}
	summary.Dependencies, err = m.buildDependencyReferences(ctx, dependencies)
	if err != nil {
		return Summary{}, err
	}
	return summary, nil
}

func (m *Service) buildDependencyReferences(
	ctx context.Context,
	dependencies []Dependency,
) ([]DependencyReference, error) {
	refs := make([]DependencyReference, 0, len(dependencies))
	for _, dependency := range dependencies {
		dependsOn, err := m.taskReference(ctx, dependency.DependsOnTaskID)
		if err != nil {
			return nil, err
		}
		refs = append(refs, DependencyReference{
			TaskID:          dependency.TaskID,
			DependsOnTaskID: dependency.DependsOnTaskID,
			Kind:            dependency.Kind,
			CreatedAt:       dependency.CreatedAt,
			DependsOn:       dependsOn,
		})
	}
	return refs, nil
}
