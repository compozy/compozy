package cli

import (
	"time"

	"github.com/compozy/compozy/internal/api/contract"
)

// HookCatalogQuery captures the CLI filters for resolved hook catalog queries.
type HookCatalogQuery = contract.HookCatalogQuery

// HookCatalogRecord is one resolved hook returned by the daemon API.
type HookCatalogRecord = contract.HookCatalogPayload

// HookRunsQuery captures the CLI filters for hook execution history queries.
type HookRunsQuery = contract.HookRunsQuery

// HookRunRecord is one persisted hook execution audit record.
type HookRunRecord = contract.HookRunPayload

// HookEventsQuery captures the CLI filters for hook taxonomy queries.
type HookEventsQuery = contract.HookEventsQuery

// HookEventRecord is one supported hook taxonomy row returned by the daemon API.
type HookEventRecord = contract.HookEventPayload

// LogEventRecord is one cross-session runtime log row.
type LogEventRecord = contract.LogEventPayload

// LogsListQuery captures the CLI filters for cross-session runtime log queries.
type LogsListQuery struct {
	WorkspaceRef  string
	SessionID     string
	AgentName     string
	Type          string
	RunID         string
	ActorKind     string
	ActorID       string
	Provider      string
	Outcome       string
	Component     string
	ErrorOnly     bool
	AfterSequence int64
	Since         time.Time
	Last          int
}
