package daemon

import (
	"context"
	"fmt"
	"slices"

	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/monitor"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
)

type instanceDigestCollector struct {
	instances *globaldb.InstanceRepo
	sessions  *session.Manager
	monitors  *monitor.Service
}

func (c instanceDigestCollector) Collect(ctx context.Context, id string, scope heartbeat.CollectorScope, include []string, policy, instructions string) (heartbeat.CheckDigest, error) {
	scope, facts, err := c.instances.CollectorFacts(ctx, id, scope)
	if err != nil {
		return heartbeat.CheckDigest{}, err
	}
	if include == nil || slices.Contains(include, "sessions_idle") {
		for i := range facts.Sessions {
			events, err := c.sessions.EventMetadata(ctx, facts.Sessions[i].ID, store.EventQuery{Limit: 1})
			if err != nil {
				return heartbeat.CheckDigest{}, fmt.Errorf("collect idle session %s: %w", facts.Sessions[i].ID, err)
			}
			if len(events) > 0 {
				facts.Sessions[i].LastEventAt = events[0].Timestamp
				facts.Sessions[i].LastSeq = events[0].Sequence
			}
		}
	}
	if (include == nil || slices.Contains(include, "monitors_fired")) && scope.MainSessionID != "" {
		for _, item := range c.monitors.List(scope.MainSessionID) {
			facts.Monitors = append(facts.Monitors, heartbeat.MonitorFact{ID: item.ID, SessionID: item.SessionID, Fired: item.Status == monitor.StatusFired})
		}
	}
	return heartbeat.CollectDigest(scope, facts, include, policy, instructions)
}
