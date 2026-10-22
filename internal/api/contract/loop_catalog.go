package contract

import (
	"time"
)

// LoopCatalogEntryPayload is one fully hydrated row from a bounded catalog page.
type LoopCatalogEntryPayload struct {
	WorkspaceID   string                      `json:"workspace_id,omitempty"`
	Name          string                      `json:"name"`
	Version       int                         `json:"version"`
	Description   string                      `json:"description,omitempty"`
	Source        LoopSource                  `json:"source"`
	Catalog       LoopCatalogResourceSpec     `json:"catalog"`
	Inputs        map[string]LoopInput        `json:"inputs,omitempty"`
	Start         []LoopStartBinding          `json:"start,omitempty"`
	Contract      LoopContract                `json:"contract"`
	LastRun       *LoopCatalogLastRunPayload  `json:"last_run,omitzero"`
	Aggregate30d  LoopCatalogAggregatePayload `json:"aggregate_30d"`
	SuccessRate30 float64                     `json:"success_rate_30d"`
}

// LoopCatalogLastRunPayload identifies the latest run shown in a catalog entry.
type LoopCatalogLastRunPayload struct {
	ID             string        `json:"id"`
	Status         LoopRunStatus `json:"status"`
	BestGeneration *int64        `json:"best_generation,omitzero"`
	BestScore      *float64      `json:"best_score,omitzero"`
	CreatedAt      time.Time     `json:"created_at"`
}

// LoopCatalogResourceSpec is the public catalog metadata projection.
type LoopCatalogResourceSpec struct {
	UseWhen  string   `json:"use_when,omitempty"`
	Keywords []string `json:"keywords,omitempty"`
	Category string   `json:"category,omitempty"`
}

// LoopCatalogAggregatePayload summarizes recent loop activity.
type LoopCatalogAggregatePayload struct {
	Runs      int `json:"runs"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
}

// LoopCatalogFacetsPayload contains exact self-filtered catalog counts.
type LoopCatalogFacetsPayload struct {
	Kinds      map[string]int `json:"kinds"`
	Categories map[string]int `json:"categories"`
	Statuses   map[string]int `json:"statuses"`
}

// LoopsResponse is returned by GET /workspaces/{workspace_id}/loops.
type LoopsResponse struct {
	Aggregate *LoopCatalogBatchResponse `json:"-"`
	Loops     []LoopCatalogEntryPayload `json:"loops"`
	Facets    LoopCatalogFacetsPayload  `json:"facets"`
	Page      CountedCursorPagePayload  `json:"page"`
}

// LoopCatalogBatchResponse reports bounded, independently resolved definition populations.
type LoopCatalogBatchResponse struct {
	Entries   []LoopCatalogAggregateEntryPayload `json:"entries"`
	Errors    []LoopCatalogWorkspaceError        `json:"errors"`
	Truncated bool                               `json:"truncated"`
}

type LoopCatalogWorkspaceError struct {
	WorkspaceID string `json:"workspace_id,omitempty"`
	Code        string `json:"code"`
	Message     string `json:"message"`
}

type LoopCatalogSourcePayload struct {
	Kind  LoopSource `json:"kind"`
	Layer string     `json:"layer"`
}

// LoopCatalogAggregateEntryPayload is one catalog entry of the cross-workspace batch. It repeats the
// LoopCatalogEntryPayload fields instead of embedding them because its source is an object
// ({kind, layer}) rather than the plain source kind; embedding would shadow the field and break the
// generated SDK clients.
type LoopCatalogAggregateEntryPayload struct {
	WorkspaceID   string                      `json:"workspace_id,omitempty"`
	Name          string                      `json:"name"`
	Version       int                         `json:"version"`
	Description   string                      `json:"description,omitempty"`
	Source        LoopCatalogSourcePayload    `json:"source"`
	Catalog       LoopCatalogResourceSpec     `json:"catalog"`
	Inputs        map[string]LoopInput        `json:"inputs,omitempty"`
	Start         []LoopStartBinding          `json:"start,omitempty"`
	Contract      LoopContract                `json:"contract"`
	LastRun       *LoopCatalogLastRunPayload  `json:"last_run,omitzero"`
	Aggregate30d  LoopCatalogAggregatePayload `json:"aggregate_30d"`
	SuccessRate30 float64                     `json:"success_rate_30d"`
}

// NewLoopCatalogAggregateEntry builds a batch entry from a workspace catalog entry and its layer.
func NewLoopCatalogAggregateEntry(entry LoopCatalogEntryPayload, layer string) LoopCatalogAggregateEntryPayload {
	return LoopCatalogAggregateEntryPayload{
		WorkspaceID:   entry.WorkspaceID,
		Name:          entry.Name,
		Version:       entry.Version,
		Description:   entry.Description,
		Source:        LoopCatalogSourcePayload{Kind: entry.Source, Layer: layer},
		Catalog:       entry.Catalog,
		Inputs:        entry.Inputs,
		Start:         entry.Start,
		Contract:      entry.Contract,
		LastRun:       entry.LastRun,
		Aggregate30d:  entry.Aggregate30d,
		SuccessRate30: entry.SuccessRate30,
	}
}
