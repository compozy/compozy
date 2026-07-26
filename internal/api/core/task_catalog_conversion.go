package core

import (
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	observepkg "github.com/compozy/agh/internal/observe"
	taskpkg "github.com/compozy/agh/internal/task"
)

// TaskCatalogResponseFromPage converts one bounded catalog page into its wire envelope.
func TaskCatalogResponseFromPage(page taskpkg.CatalogPage) contract.TasksResponse {
	items := make([]contract.TaskCatalogItemPayload, 0, len(page.Tasks))
	for index := range page.Tasks {
		items = append(items, contract.TaskCatalogItemPayloadFromSummary(&page.Tasks[index]))
	}
	response := contract.TasksResponse{
		Tasks: items,
		Page: contract.CountedCursorPagePayload{
			NextCursor: page.NextCursor,
			HasMore:    page.HasMore,
			Total:      page.Total,
			Limit:      page.Limit,
		},
		Facets: contract.TaskCatalogFacetsPayload{
			Statuses: make([]contract.TaskCatalogStatusFacetPayload, 0, len(page.StatusFacets)),
			Owners:   make([]contract.TaskCatalogOwnerFacetPayload, 0, len(page.OwnerFacets)),
		},
	}
	for _, facet := range page.StatusFacets {
		response.Facets.Statuses = append(response.Facets.Statuses, contract.TaskCatalogStatusFacetPayload{
			Status: facet.Status,
			Count:  facet.Count,
		})
	}
	for _, facet := range page.OwnerFacets {
		response.Facets.Owners = append(response.Facets.Owners, contract.TaskCatalogOwnerFacetPayload{
			Owner: facet.Owner,
			Count: facet.Count,
		})
	}
	return response
}

// TaskInboxPayloadFromView converts one actor-scoped inbox page into its wire envelope.
func TaskInboxPayloadFromView(view observepkg.TaskInboxView) contract.TaskInboxPayload {
	payload := contract.TaskInboxPayload{
		UnreadTotal:   view.UnreadTotal,
		ArchivedTotal: view.ArchivedTotal,
		Groups:        make([]contract.TaskInboxLaneGroupPayload, 0, len(view.Groups)),
		Page: contract.CountedCursorPagePayload{
			NextCursor: view.NextCursor,
			HasMore:    view.HasMore,
			Total:      view.Total,
			Limit:      view.Limit,
		},
		Facets: contract.TaskInboxFacetsPayload{
			Statuses:   make([]contract.TaskInboxStatusFacetPayload, 0, len(view.StatusFacets)),
			Priorities: make([]contract.TaskInboxPriorityFacetPayload, 0, len(view.PriorityFacets)),
		},
	}
	for _, facet := range view.StatusFacets {
		payload.Facets.Statuses = append(payload.Facets.Statuses, contract.TaskInboxStatusFacetPayload{
			Status: facet.Status,
			Count:  facet.Count,
		})
	}
	for _, facet := range view.PriorityFacets {
		payload.Facets.Priorities = append(payload.Facets.Priorities, contract.TaskInboxPriorityFacetPayload{
			Priority: facet.Priority,
			Count:    facet.Count,
		})
	}
	for _, group := range view.Groups {
		groupPayload := contract.TaskInboxLaneGroupPayload{
			Lane:        contract.TaskInboxLane(group.Lane),
			Count:       group.Count,
			UnreadCount: group.UnreadCount,
			Items:       make([]contract.TaskInboxItemPayload, 0, len(group.Items)),
		}
		for _, item := range group.Items {
			groupPayload.Items = append(groupPayload.Items, contract.TaskInboxItemPayload{
				Task:             contract.TaskInboxTaskPayloadFromReference(item.Task),
				Lane:             contract.TaskInboxLane(item.Lane),
				ApprovalPolicy:   item.ApprovalPolicy,
				ApprovalState:    item.ApprovalState,
				BlockingReason:   taskpkg.RedactClaimTokens(strings.TrimSpace(item.BlockingReason)),
				LatestActivityAt: item.LatestActivityAt,
				Run:              contract.TaskCatalogRunPayloadFromSummary(item.Run),
				Triage:           TaskTriageStatePayloadFromState(item.Triage),
			})
		}
		payload.Groups = append(payload.Groups, groupPayload)
	}
	return payload
}
