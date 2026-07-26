package extensionpkg

import (
	"encoding/json"

	"strings"
	"time"

	apicontract "github.com/compozy/agh/internal/api/contract"

	observepkg "github.com/compozy/agh/internal/observe"
	taskpkg "github.com/compozy/agh/internal/task"
)

func taskInboxPayloadFromView(view observepkg.TaskInboxView) apicontract.TaskInboxPayload {
	payload := apicontract.TaskInboxPayload{
		UnreadTotal:   view.UnreadTotal,
		ArchivedTotal: view.ArchivedTotal,
		Groups:        make([]apicontract.TaskInboxLaneGroupPayload, 0, len(view.Groups)),
		Page: apicontract.CountedCursorPagePayload{
			NextCursor: view.NextCursor,
			HasMore:    view.HasMore,
			Total:      view.Total,
			Limit:      view.Limit,
		},
		Facets: apicontract.TaskInboxFacetsPayload{
			Statuses:   make([]apicontract.TaskInboxStatusFacetPayload, 0, len(view.StatusFacets)),
			Priorities: make([]apicontract.TaskInboxPriorityFacetPayload, 0, len(view.PriorityFacets)),
		},
	}
	for _, facet := range view.StatusFacets {
		payload.Facets.Statuses = append(payload.Facets.Statuses, apicontract.TaskInboxStatusFacetPayload{
			Status: facet.Status,
			Count:  facet.Count,
		})
	}
	for _, facet := range view.PriorityFacets {
		payload.Facets.Priorities = append(payload.Facets.Priorities, apicontract.TaskInboxPriorityFacetPayload{
			Priority: facet.Priority,
			Count:    facet.Count,
		})
	}
	for _, group := range view.Groups {
		groupPayload := apicontract.TaskInboxLaneGroupPayload{
			Lane:        apicontract.TaskInboxLane(group.Lane),
			Count:       group.Count,
			UnreadCount: group.UnreadCount,
		}
		if len(group.Items) > 0 {
			groupPayload.Items = make([]apicontract.TaskInboxItemPayload, 0, len(group.Items))
			for _, item := range group.Items {
				groupPayload.Items = append(groupPayload.Items, apicontract.TaskInboxItemPayload{
					Task:             apicontract.TaskInboxTaskPayloadFromReference(item.Task),
					Lane:             apicontract.TaskInboxLane(item.Lane),
					ApprovalPolicy:   item.ApprovalPolicy,
					ApprovalState:    item.ApprovalState,
					BlockingReason:   taskpkg.RedactClaimTokens(strings.TrimSpace(item.BlockingReason)),
					LatestActivityAt: item.LatestActivityAt,
					Run:              apicontract.TaskCatalogRunPayloadFromSummary(item.Run),
					Triage:           taskTriageStatePayloadFromState(item.Triage),
				})
			}
		}
		payload.Groups = append(payload.Groups, groupPayload)
	}
	return payload
}

func taskTriageStatePayloadFromState(state taskpkg.TriageState) apicontract.TaskTriageStatePayload {
	return apicontract.TaskTriageStatePayload{
		TaskID:             state.TaskID,
		Actor:              state.Actor,
		Read:               state.Read,
		Archived:           state.Archived,
		Dismissed:          state.Dismissed,
		LastSeenActivityAt: optionalTime(state.LastSeenActivityAt),
		UpdatedAt:          state.UpdatedAt,
	}
}

func filterTaskRuns(runs []taskpkg.Run, query taskpkg.RunQuery) []taskpkg.Run {
	filtered := make([]taskpkg.Run, 0, len(runs))
	for _, run := range runs {
		if query.Status.Normalize() != taskpkg.TaskRunStatusUnknown &&
			run.Status.Normalize() != query.Status.Normalize() {
			continue
		}
		if strings.TrimSpace(query.SessionID) != "" &&
			strings.TrimSpace(run.SessionID) != strings.TrimSpace(query.SessionID) {
			continue
		}
		filtered = append(filtered, run)
	}
	if query.Limit > 0 && len(filtered) > query.Limit {
		return filtered[:query.Limit]
	}
	return filtered
}

func cloneOwnership(source *taskpkg.Ownership) *taskpkg.Ownership {
	if source == nil {
		return nil
	}
	return &taskpkg.Ownership{
		Kind: source.Kind.Normalize(),
		Ref:  strings.TrimSpace(source.Ref),
	}
}

func cloneActorIdentity(source *taskpkg.ActorIdentity) *taskpkg.ActorIdentity {
	if source == nil {
		return nil
	}
	return &taskpkg.ActorIdentity{
		Kind: source.Kind.Normalize(),
		Ref:  strings.TrimSpace(source.Ref),
	}
}

func trimStringPtr(source *string) *string {
	if source == nil {
		return nil
	}
	value := strings.TrimSpace(*source)
	return &value
}

func normalizePriorityPtr(source *taskpkg.Priority) *taskpkg.Priority {
	if source == nil {
		return nil
	}
	normalized := source.Normalize()
	return &normalized
}

func normalizeApprovalPolicyPtr(source *taskpkg.ApprovalPolicy) *taskpkg.ApprovalPolicy {
	if source == nil {
		return nil
	}
	normalized := source.Normalize()
	return &normalized
}

func optionalTime(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	cloned := value
	return &cloned
}

func cloneRawMessagePtr(source *json.RawMessage) *json.RawMessage {
	if source == nil {
		return nil
	}
	cloned := cloneRawMessage(*source)
	return &cloned
}

func cloneRawMessage(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}
