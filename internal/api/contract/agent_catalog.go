package contract

import "time"

// AgentSessionMetricsPayload contains exact visible-session aggregates for one agent.
type AgentSessionMetricsPayload struct {
	Total          int        `json:"total"`
	Active         int        `json:"active"`
	Failed         int        `json:"failed"`
	RuntimeSeconds int64      `json:"runtime_seconds"`
	LastActivityAt *time.Time `json:"last_activity_at,omitempty"`
}

// AgentCatalogItemPayload combines one effective definition with its optional
// runtime session aggregate. Sessions is nil when the session authority is
// temporarily unavailable.
type AgentCatalogItemPayload struct {
	Agent    AgentPayload                `json:"agent"`
	Sessions *AgentSessionMetricsPayload `json:"sessions,omitempty"`
}

// AgentCatalogFacetsPayload contains fleet-wide values before search, filters,
// and cursor pagination. Active and Idle are exact only when the enclosing
// response reports SessionsAvailable.
type AgentCatalogFacetsPayload struct {
	Categories []string `json:"categories"`
	Total      int      `json:"total"`
	Active     int      `json:"active"`
	Idle       int      `json:"idle"`
}

// AgentCatalogResponse is one bounded workspace-scoped agent fleet page.
type AgentCatalogResponse struct {
	Agents            []AgentCatalogItemPayload `json:"agents"`
	Page              CountedCursorPagePayload  `json:"page"`
	Facets            AgentCatalogFacetsPayload `json:"facets"`
	SessionsAvailable bool                      `json:"sessions_available"`
}
