package contract

// BridgeStatusCountsPayload captures aggregate per-status bridge counts.
type BridgeStatusCountsPayload struct {
	Disabled     int `json:"disabled"`
	Starting     int `json:"starting"`
	Ready        int `json:"ready"`
	Degraded     int `json:"degraded"`
	AuthRequired int `json:"auth_required"`
	Error        int `json:"error"`
}

// BridgeCatalogFacetsPayload reports exact counts for the complete filtered authority.
type BridgeCatalogFacetsPayload struct {
	Platforms map[string]int            `json:"platforms"`
	Statuses  BridgeStatusCountsPayload `json:"statuses"`
}

// BridgesResponse wraps one bounded public bridge catalog page.
type BridgesResponse struct {
	Bridges      []BridgePayload                `json:"bridges"`
	BridgeHealth map[string]BridgeHealthPayload `json:"bridge_health"`
	Facets       BridgeCatalogFacetsPayload     `json:"facets"`
	Page         CountedCursorPagePayload       `json:"page"`
}
