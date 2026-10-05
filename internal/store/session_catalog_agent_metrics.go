package store

import (
	"context"
	"time"
)

// SessionAgentMetricsQuery describes a profile-scoped grouped read; an empty workspace selects Global.
type SessionAgentMetricsQuery struct {
	ReadScope           ReadScope
	WorkspaceID         string
	ExcludeIDs          []string
	ExcludeSessionTypes []string
	ExcludeSpawnRoles   []string
}

// Validate requires an explicit profile read scope.
func (q SessionAgentMetricsQuery) Validate() error {
	return q.ReadScope.Validate()
}

// SessionAgentMetrics contains exact visible-session aggregates for one agent.
type SessionAgentMetrics struct {
	AgentName      string
	Total          int
	Active         int
	Failed         int
	RuntimeSeconds int64
	LastActivityAt time.Time
}

// SessionAgentMetricsReader exposes one grouped durable-catalog read for agent projections.
type SessionAgentMetricsReader interface {
	AggregateSessionsByAgent(
		ctx context.Context,
		query SessionAgentMetricsQuery,
	) ([]SessionAgentMetrics, error)
}
