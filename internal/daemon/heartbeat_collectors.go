package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	workspacepkg "github.com/compozy/compozy/internal/workspace"

	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/monitor"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
)

type workLogWorkspaceResolver interface {
	Resolve(context.Context, string) (workspacepkg.ResolvedWorkspace, error)
}

type instanceDigestCollector struct {
	workspaces workLogWorkspaceResolver
	logger     *slog.Logger
	instances  *globaldb.InstanceRepo
	sessions   *session.Manager
	monitors   *monitor.Service
}

func (c instanceDigestCollector) Collect(
	ctx context.Context,
	id string,
	scope heartbeat.CollectorScope,
	include []string,
	policy, instructions string,
) (heartbeat.CheckDigest, error) {
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
			facts.Monitors = append(facts.Monitors, monitorDigestFact(item))
		}
	}
	if (include == nil || slices.Contains(include, "work_log")) && scope.WorkspaceID != "" {
		workspace, err := c.workspaces.Resolve(ctx, scope.WorkspaceID)
		if err != nil {
			logger := c.logger
			if logger == nil {
				logger = slog.Default()
			}
			logger.Warn("work_log: resolve workspace", "workspace_id", scope.WorkspaceID, "error", err)
		} else {
			facts.WorkLog, facts.WorkLogTruncated = readWorkspaceWorkLog(workspace.RootDir, c.logger)
		}
	}
	return heartbeat.CollectDigest(scope, facts, include, policy, instructions)
}

// monitorDigestFact keeps pull_request monitors out of the fired bucket: their wakes already reached
// the session, so none of them may buy a check on top.
func monitorDigestFact(item monitor.Monitor) heartbeat.MonitorFact {
	return heartbeat.MonitorFact{
		ID: item.ID, SessionID: item.SessionID,
		Fired: item.Status == monitor.StatusFired && item.Kind != monitor.KindPullRequest,
	}
}
