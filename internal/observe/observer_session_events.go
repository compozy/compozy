package observe

import (
	"context"

	"strings"

	"github.com/compozy/agh/internal/acp"

	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
)

// OnSessionCreated tracks the live session snapshot used by observability reads.
func (o *Observer) OnSessionCreated(ctx context.Context, sess *session.Session) {
	info := sess.Info()
	snapshot := o.observedSessionSnapshot(
		ctx,
		info.ID,
		info.AgentName,
		info.Provider,
		info.Model,
		info.WorkspaceID,
		info.Lineage,
	)

	o.trackSession(info.ID, snapshot)
}

// OnSessionStopped removes the live snapshot after the manager persists lifecycle state.
func (o *Observer) OnSessionStopped(_ context.Context, sess *session.Session) {
	info := sess.Info()
	o.untrackSession(info.ID)
}

// OnAgentEvent records one lightweight cross-session event summary and any derived aggregates.
func (o *Observer) OnAgentEvent(ctx context.Context, sessionID string, payload any) {
	o.observeAgentEvent(ctx, strings.TrimSpace(sessionID), payload)
}

// OnAgentEventForSession records event summaries for the active session.
func (o *Observer) OnAgentEventForSession(ctx context.Context, sess *session.Session, payload any) {
	if sess == nil {
		return
	}
	info := sess.Info()
	if info == nil {
		return
	}
	o.observeAgentEvent(ctx, info.ID, payload)
}

func (o *Observer) observeAgentEvent(ctx context.Context, sessionID string, payload any) {
	event, ok := normalizeObservedAgentEvent(payload)
	if !ok {
		o.logger.Warn("observe: skipped unsupported agent event payload", "session_id", strings.TrimSpace(sessionID))
		return
	}

	id, snapshot, ok := o.validateObservedEvent(ctx, sessionID, event)
	if !ok {
		return
	}

	timestamp := observedEventTimestamp(event, o.now)

	if err := o.writeObservedEventSummary(ctx, id, snapshot, event, timestamp); err != nil {
		o.logObservedEventFailure("observe: write event summary failed", id, snapshot, event, err)
	}
	if err := o.aggregateObservedUsage(ctx, id, snapshot, event, timestamp); err != nil {
		o.logger.Warn(
			"observe: update token stats failed",
			"session_id",
			id,
			"agent_name",
			snapshot.agentName,
			"workspace_id",
			snapshot.workspaceID,
			"turn_id",
			event.TurnID,
			"error",
			err,
		)
	}
	if err := o.writeObservedPermissionLog(ctx, id, snapshot, event, timestamp); err != nil {
		o.logger.Warn(
			"observe: write permission log failed",
			"session_id",
			id,
			"agent_name",
			snapshot.agentName,
			"workspace_id",
			snapshot.workspaceID,
			"error",
			err,
		)
	}
}

func (o *Observer) validateObservedEvent(
	ctx context.Context,
	sessionID string,
	event acp.AgentEvent,
) (string, observedSession, bool) {
	id := strings.TrimSpace(sessionID)
	if id == "" {
		o.logger.Warn("observe: skipped agent event with empty session id", "event_type", event.Type)
		return "", observedSession{}, false
	}

	snapshot, ok := o.sessionSnapshot(id)
	if !ok {
		snapshot, ok = o.recoverSessionSnapshot(ctx, id)
		if !ok {
			o.logger.Warn(
				"observe: skipped agent event for unknown session",
				"session_id",
				id,
				"event_type",
				event.Type,
			)
			return "", observedSession{}, false
		}
	}
	if strings.TrimSpace(event.Type) == "" {
		o.logger.Warn(
			"observe: skipped agent event with empty type",
			"session_id",
			id,
			"agent_name",
			snapshot.agentName,
			"workspace_id",
			snapshot.workspaceID,
		)
		return "", observedSession{}, false
	}

	return id, snapshot, true
}

func (o *Observer) recoverSessionSnapshot(ctx context.Context, sessionID string) (observedSession, bool) {
	requireObserverContext(ctx, "recoverSessionSnapshot")

	id := strings.TrimSpace(sessionID)
	if id == "" {
		return observedSession{}, false
	}

	if o.sessionSource != nil {
		for _, info := range o.sessionSource.List() {
			if info == nil || strings.TrimSpace(info.ID) != id {
				continue
			}
			snapshot := o.observedSessionSnapshot(
				ctx,
				id,
				info.AgentName,
				info.Provider,
				info.Model,
				info.WorkspaceID,
				info.Lineage,
			)
			o.trackSession(id, snapshot)
			return snapshot, true
		}
	}

	if o.registry == nil {
		return observedSession{}, false
	}
	sessions, err := o.registry.ListSessions(ctx, store.SessionListQuery{})
	if err != nil {
		o.logger.Warn("observe: recover session snapshot failed", "session_id", id, "error", err)
		return observedSession{}, false
	}
	for _, info := range sessions {
		if strings.TrimSpace(info.ID) != id {
			continue
		}
		snapshot := o.observedSessionSnapshot(
			ctx,
			id,
			info.AgentName,
			info.Provider,
			"",
			info.WorkspaceID,
			info.Lineage,
		)
		if strings.TrimSpace(info.State) != string(session.StateStopped) {
			o.trackSession(id, snapshot)
		}
		return snapshot, true
	}
	return observedSession{}, false
}

func (o *Observer) observedSessionSnapshot(
	ctx context.Context,
	sessionID string,
	agentName string,
	provider string,
	model string,
	workspaceID string,
	lineage *store.SessionLineage,
) observedSession {
	requireObserverContext(ctx, "observedSessionSnapshot")

	normalizedLineage := store.NormalizeSessionLineage(sessionID, lineage)
	snapshot := observedSession{
		agentName:       strings.TrimSpace(agentName),
		provider:        strings.TrimSpace(provider),
		model:           strings.TrimSpace(model),
		workspaceID:     strings.TrimSpace(workspaceID),
		parentSessionID: normalizedLineage.ParentSessionID,
		rootSessionID:   normalizedLineage.RootSessionID,
		spawnDepth:      normalizedLineage.SpawnDepth,
	}
	if o.resolvePermissionMode != nil {
		permissionMode, err := o.resolvePermissionMode(ctx, snapshot.agentName, snapshot.workspaceID)
		if err != nil {
			o.logger.Warn(
				"observe: resolve permission mode failed",
				"session_id",
				strings.TrimSpace(sessionID),
				"agent_name",
				snapshot.agentName,
				"workspace_id",
				snapshot.workspaceID,
				"error",
				err,
			)
		} else {
			snapshot.permissionMode = strings.TrimSpace(permissionMode)
		}
	}
	if o.resolveProviderAuth != nil {
		authMode, err := o.resolveProviderAuth(
			ctx,
			snapshot.agentName,
			snapshot.provider,
			snapshot.model,
			snapshot.workspaceID,
		)
		if err != nil {
			o.logger.Warn(
				"observe: resolve provider auth mode failed",
				"session_id",
				strings.TrimSpace(sessionID),
				"agent_name",
				snapshot.agentName,
				"provider",
				snapshot.provider,
				"workspace_id",
				snapshot.workspaceID,
				"error",
				err,
			)
		} else {
			snapshot.authMode = authMode
		}
	}
	return snapshot
}

func requireObserverContext(ctx context.Context, caller string) {
	if ctx == nil {
		panic("observe: nil context passed to " + caller)
	}
}
