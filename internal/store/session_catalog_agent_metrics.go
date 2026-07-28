package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// SessionAgentMetricsQuery describes one workspace-scoped grouped catalog read.
// Active runtime session IDs are excluded before the session manager overlays
// their current snapshots.
type SessionAgentMetricsQuery struct {
	WorkspaceID         string
	ExcludeIDs          []string
	ExcludeSessionTypes []string
	ExcludeSpawnRoles   []string
}

// Validate ensures grouped metrics cannot cross workspace boundaries.
func (q SessionAgentMetricsQuery) Validate() error {
	if strings.TrimSpace(q.WorkspaceID) == "" {
		return fmt.Errorf("store: session agent metrics workspace id is required")
	}
	return nil
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
