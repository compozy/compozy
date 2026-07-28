package contract

import "time"

// LoopCatalogEntryPayload is one fully hydrated row from a bounded catalog page.
type LoopCatalogEntryPayload struct {
	Name          string                      `json:"name"`
	Version       int                         `json:"version"`
	Description   string                      `json:"description,omitempty"`
	Source        LoopSource                  `json:"source"`
	Catalog       LoopCatalogResourceSpec     `json:"catalog"`
	Inputs        map[string]LoopInput        `json:"inputs,omitempty"`
	Start         []LoopStartBinding          `json:"start,omitempty"`
	Contract      LoopContract                `json:"contract"`
	LastRun       *LoopCatalogLastRunPayload  `json:"last_run,omitempty"`
	Aggregate30d  LoopCatalogAggregatePayload `json:"aggregate_30d"`
	SuccessRate30 float64                     `json:"success_rate_30d"`
}

// LoopCatalogLastRunPayload identifies the latest run shown in a catalog entry.
type LoopCatalogLastRunPayload struct {
	ID        string        `json:"id"`
	Status    LoopRunStatus `json:"status"`
	CreatedAt time.Time     `json:"created_at"`
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
	Loops  []LoopCatalogEntryPayload `json:"loops"`
	Facets LoopCatalogFacetsPayload  `json:"facets"`
	Page   CountedCursorPagePayload  `json:"page"`
}
