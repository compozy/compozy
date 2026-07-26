package extensionpkg

import (
	apicontract "github.com/compozy/agh/internal/api/contract"

	taskpkg "github.com/compozy/agh/internal/task"
)

func taskSummaryPayloadsFromSummaries(tasks []taskpkg.Summary) []apicontract.TaskSummaryPayload {
	payloads := make([]apicontract.TaskSummaryPayload, len(tasks))
	for i := range tasks {
		payloads[i] = taskSummaryPayloadFromSummary(&tasks[i])
	}
	return payloads
}

func taskCatalogResponseFromPage(page taskpkg.CatalogPage) apicontract.TasksResponse {
	items := make([]apicontract.TaskCatalogItemPayload, 0, len(page.Tasks))
	for index := range page.Tasks {
		items = append(items, apicontract.TaskCatalogItemPayloadFromSummary(&page.Tasks[index]))
	}
	response := apicontract.TasksResponse{
		Tasks: items,
		Page: apicontract.CountedCursorPagePayload{
			NextCursor: page.NextCursor,
			HasMore:    page.HasMore,
			Total:      page.Total,
			Limit:      page.Limit,
		},
		Facets: apicontract.TaskCatalogFacetsPayload{
			Statuses: make([]apicontract.TaskCatalogStatusFacetPayload, 0, len(page.StatusFacets)),
			Owners:   make([]apicontract.TaskCatalogOwnerFacetPayload, 0, len(page.OwnerFacets)),
		},
	}
	for _, facet := range page.StatusFacets {
		response.Facets.Statuses = append(response.Facets.Statuses, apicontract.TaskCatalogStatusFacetPayload{
			Status: facet.Status,
			Count:  facet.Count,
		})
	}
	for _, facet := range page.OwnerFacets {
		response.Facets.Owners = append(response.Facets.Owners, apicontract.TaskCatalogOwnerFacetPayload{
			Owner: facet.Owner,
			Count: facet.Count,
		})
	}
	return response
}

func taskDependencyPayloadsFromDependencies(dependencies []taskpkg.Dependency) []apicontract.TaskDependencyPayload {
	payloads := make([]apicontract.TaskDependencyPayload, len(dependencies))
	for i := range dependencies {
		dependency := &dependencies[i]
		payloads[i] = apicontract.TaskDependencyPayload{
			TaskID:          dependency.TaskID,
			DependsOnTaskID: dependency.DependsOnTaskID,
			Kind:            dependency.Kind,
			CreatedAt:       dependency.CreatedAt,
		}
	}
	return payloads
}

func taskRunPayloadsFromRuns(runs []taskpkg.Run) []apicontract.TaskRunPayload {
	payloads := make([]apicontract.TaskRunPayload, len(runs))
	for i := range runs {
		payloads[i] = taskRunPayloadFromRun(&runs[i])
	}
	return payloads
}

func taskReferencePayloadFromReference(record taskpkg.Reference) apicontract.TaskReferencePayload {
	return apicontract.TaskReferencePayload{
		ID:             record.ID,
		Identifier:     record.Identifier,
		Title:          taskpkg.RedactClaimTokens(record.Title),
		Status:         record.Status,
		Priority:       record.Priority,
		Owner:          cloneOwnership(record.Owner),
		Scope:          record.Scope,
		WorkspaceID:    record.WorkspaceID,
		LatestEventSeq: record.LatestEventSeq,
	}
}

func taskRunSummaryPayloadFromSummary(summary *taskpkg.RunSummary) *apicontract.TaskRunSummaryPayload {
	if summary == nil {
		return nil
	}

	return &apicontract.TaskRunSummaryPayload{
		ID:                           summary.ID,
		TaskID:                       summary.TaskID,
		Status:                       summary.Status,
		Attempt:                      summary.Attempt,
		MaxAttempts:                  summary.MaxAttempts,
		SessionID:                    summary.SessionID,
		ClaimedBy:                    cloneActorIdentity(summary.ClaimedBy),
		ResolvedNetworkParticipation: cloneResolvedParticipation(summary.ResolvedNetworkParticipation),
		QueuedAt:                     summary.QueuedAt,
		ClaimedAt:                    optionalTime(summary.ClaimedAt),
		StartedAt:                    optionalTime(summary.StartedAt),
		EndedAt:                      optionalTime(summary.EndedAt),
		Error:                        taskpkg.RedactClaimTokens(summary.Error),
	}
}

func taskDependencyReferencePayloadsFromReferences(
	dependencies []taskpkg.DependencyReference,
) []apicontract.TaskDependencyReferencePayload {
	payloads := make([]apicontract.TaskDependencyReferencePayload, len(dependencies))
	for i := range dependencies {
		dependency := &dependencies[i]
		payloads[i] = apicontract.TaskDependencyReferencePayload{
			TaskID:          dependency.TaskID,
			DependsOnTaskID: dependency.DependsOnTaskID,
			Kind:            dependency.Kind,
			CreatedAt:       dependency.CreatedAt,
			DependsOn:       taskReferencePayloadFromReference(dependency.DependsOn),
		}
	}
	return payloads
}

func taskEventPayloadsFromEvents(events []taskpkg.Event) []apicontract.TaskEventPayload {
	payloads := make([]apicontract.TaskEventPayload, len(events))
	for i := range events {
		event := &events[i]
		payloads[i] = apicontract.TaskEventPayload{
			ID:        event.ID,
			TaskID:    event.TaskID,
			RunID:     event.RunID,
			EventType: event.EventType,
			Actor:     event.Actor,
			Origin:    event.Origin,
			Payload:   taskpkg.RedactClaimTokenJSON(event.Payload),
			Timestamp: event.Timestamp,
		}
	}
	return payloads
}

func taskDetailPayloadFromView(view *taskpkg.View) apicontract.TaskDetailPayload {
	if view == nil {
		return apicontract.TaskDetailPayload{}
	}

	return apicontract.TaskDetailPayload{
		Summary:              taskSummaryPayloadFromSummary(&view.Summary),
		Task:                 taskPayloadFromTask(&view.Task),
		Children:             taskSummaryPayloadsFromSummaries(view.Children),
		Dependencies:         taskDependencyPayloadsFromDependencies(view.Dependencies),
		DependencyReferences: taskDependencyReferencePayloadsFromReferences(view.DependencyReferences),
		Runs:                 taskRunPayloadsFromRuns(view.Runs),
		Events:               taskEventPayloadsFromEvents(view.Events),
	}
}

func taskTimelineItemPayloadsFromItems(items []taskpkg.TimelineItem) []apicontract.TaskTimelineItemPayload {
	payloads := make([]apicontract.TaskTimelineItemPayload, len(items))
	for i := range items {
		payloads[i] = taskTimelineItemPayloadFromItem(items[i])
	}
	return payloads
}

func taskTimelineItemPayloadFromItem(item taskpkg.TimelineItem) apicontract.TaskTimelineItemPayload {
	return apicontract.TaskTimelineItemPayload{
		Sequence:  item.Sequence,
		EventID:   item.EventID,
		Task:      taskReferencePayloadFromReference(item.Task),
		Run:       taskRunSummaryPayloadFromSummary(item.Run),
		EventType: item.EventType,
		Actor:     item.Actor,
		Origin:    item.Origin,
		Payload:   taskpkg.RedactClaimTokenJSON(item.Payload),
		Timestamp: item.Timestamp,
	}
}

func taskTreePayloadFromView(view *taskpkg.TreeView) apicontract.TaskTreePayload {
	if view == nil {
		return apicontract.TaskTreePayload{}
	}

	payload := apicontract.TaskTreePayload{
		Root: taskTreeNodePayloadFromNode(view.Root),
	}
	if len(view.Descendants) == 0 {
		return payload
	}

	payload.Descendants = make([]apicontract.TaskTreeNodePayload, 0, len(view.Descendants))
	for _, node := range view.Descendants {
		payload.Descendants = append(payload.Descendants, taskTreeNodePayloadFromNode(node))
	}
	return payload
}

func taskTreeNodePayloadFromNode(node taskpkg.TreeNode) apicontract.TaskTreeNodePayload {
	return apicontract.TaskTreeNodePayload{
		Task:           taskReferencePayloadFromReference(node.Task),
		ParentTaskID:   node.ParentTaskID,
		Depth:          node.Depth,
		ChildCount:     node.ChildCount,
		ActiveRun:      taskRunSummaryPayloadFromSummary(node.ActiveRun),
		LastActivityAt: node.LastActivityAt,
	}
}

func taskRunSessionPayloadFromSession(session *taskpkg.RunSessionRef) *apicontract.TaskRunSessionPayload {
	if session == nil {
		return nil
	}

	return &apicontract.TaskRunSessionPayload{
		SessionID:   session.SessionID,
		WorkspaceID: session.WorkspaceID,
		AgentName:   session.AgentName,
		Name:        session.Name,
		State:       session.State,
		CreatedAt:   session.CreatedAt,
		UpdatedAt:   session.UpdatedAt,
	}
}
