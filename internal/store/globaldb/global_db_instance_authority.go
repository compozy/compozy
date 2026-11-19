package globaldb

import (
	"context"
	"database/sql"
	"encoding/json/v2"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"

	"github.com/compozy/compozy/internal/agentinstance"
)

var _ agentinstance.CallerReader = (*InstanceRepo)(nil)

func (r *InstanceRepo) InstanceCaller(ctx context.Context, sessionID string) (agentinstance.Caller, error) {
	caller, _, err := r.MCPProjectionCaller(ctx, sessionID)
	return caller, err
}

// MCPProjectionCaller returns the policy and the row it was decoded from.
func (r *InstanceRepo) MCPProjectionCaller(
	ctx context.Context, sessionID string,
) (agentinstance.Caller, agentinstance.ProjectionState, error) {
	if err := r.checkReady(ctx, "resolve instance caller"); err != nil {
		return agentinstance.Caller{}, agentinstance.ProjectionState{}, err
	}
	row, err := r.readQueries.GetInstanceCaller(ctx, sessionID)
	if err != nil {
		return agentinstance.Caller{}, agentinstance.ProjectionState{}, err
	}
	state, err := instanceCallerProjectionState(row)
	if err != nil {
		return agentinstance.Caller{}, agentinstance.ProjectionState{}, err
	}
	var policy store.SessionPermissionPolicy
	if err := json.Unmarshal([]byte(state.Policy), &policy); err != nil {
		return agentinstance.Caller{}, agentinstance.ProjectionState{}, err
	}
	if state.Depth > 0 {
		policy = store.DecodeInstanceCeilingIntent(policy, int(state.Depth))
	}
	return agentinstance.Caller{
		SessionID: row.ID, AgentName: state.AgentName, ProfileID: state.ProfileID,
		InstanceID: state.CallerInstanceID, InstanceDepth: int(state.Depth), Grant: state.Grant,
		PermissionPolicy: store.DecodeIntent(policy, store.LineageKind(state.LineageKind), state.SessionType),
	}, state, nil
}

func instanceCallerProjectionState(row sqlcgen.GetInstanceCallerRow) (agentinstance.ProjectionState, error) {
	id, idOK := row.OriginatingInstanceID.(string)
	depth, depthOK := row.InstanceDepth.(int64)
	if !idOK || !depthOK {
		return agentinstance.ProjectionState{}, errors.New("store: invalid instance caller lineage")
	}
	return agentinstance.ProjectionState{
		CallerInstanceID: id, InstanceID: id, Depth: depth, Policy: row.PermissionPolicyJson,
		Grant: row.EffectiveWorkspaceAccess, LineageKind: row.LineageKind, SessionType: row.SessionType,
		ProfileID: row.ProfileID, AgentName: row.AgentName,
	}, nil
}

// MCPProjectionState reads the caller ceiling and current instance binding using the same
// repository queries as policy resolution, without decoding tool IDs or occurrence history.
func (r *InstanceRepo) MCPProjectionState(
	ctx context.Context, sessionID, sessionType string,
) (agentinstance.ProjectionState, error) {
	if err := r.checkReady(ctx, "observe instance policy"); err != nil {
		return agentinstance.ProjectionState{}, err
	}
	if sessionType != "" && sessionType != "triage" {
		ancestry, err := r.mcpProjectionQueries(ctx).SessionMayHaveInstanceAncestry(ctx, sessionID)
		if err != nil {
			return agentinstance.ProjectionState{}, fmt.Errorf("store: read MCP caller ancestry: %w", err)
		}
		if ancestry.Valid && !ancestry.Bool {
			return agentinstance.ProjectionState{}, nil
		}
	}
	row, err := r.mcpProjectionQueries(ctx).GetInstanceCaller(ctx, sessionID)
	if err != nil {
		return agentinstance.ProjectionState{}, fmt.Errorf("store: read MCP caller: %w", err)
	}
	state, err := instanceCallerProjectionState(row)
	if err != nil {
		return agentinstance.ProjectionState{}, fmt.Errorf("store: decode MCP caller: %w", err)
	}
	if sessionType == "" {
		sessionType = row.SessionType
		state.WorkspaceID, err = r.mcpProjectionQueries(ctx).GetSessionWorkspaceID(ctx, sessionID)
		if err != nil {
			return agentinstance.ProjectionState{}, fmt.Errorf("store: read MCP workspace: %w", err)
		}
	}
	if sessionType == "triage" {
		binding, occurrence, err := r.CheckInSessionBinding(ctx, sessionID)
		if err != nil {
			return agentinstance.ProjectionState{}, fmt.Errorf("store: read MCP check-in binding: %w", err)
		}
		if binding != "" {
			state.InstanceID, state.OccurrenceID = binding, occurrence
		}
	}
	if state.InstanceID == "" {
		return state, nil
	}
	instance, err := r.mcpProjectionQueries(ctx).GetAgentInstance(ctx, state.InstanceID)
	if err != nil {
		return agentinstance.ProjectionState{}, fmt.Errorf("store: read MCP agent instance: %w", err)
	}
	state.MainSessionID = instance.MainSessionID.String
	state.State, state.PauseReason = instance.State, instance.PauseReason
	state.PermissionCeiling, state.ChecksEnabled, state.Archived = instance.PermissionCeiling,
		instance.ChecksEnabled, instance.ArchivedAt.Valid
	return state, nil
}

func (r *SessionRepo) CheckSessionSuperseded(ctx context.Context, sessionID string) error {
	if err := r.checkReady(ctx, "check session supersession"); err != nil {
		return err
	}
	return checkSessionSuperseded(ctx, r.readQueries, sessionID)
}

func checkSessionSuperseded(ctx context.Context, queries *sqlcgen.Queries, sessionID string) error {
	superseded, err := queries.IsSupersededInstanceSession(ctx, sessionID)
	if err != nil || !superseded {
		return err
	}
	caller, err := queries.GetInstanceCaller(ctx, sessionID)
	if err != nil {
		return err
	}
	id, ok := caller.OriginatingInstanceID.(string)
	if !ok {
		return errors.New("store: invalid originating instance")
	}
	instance, err := queries.GetAgentInstance(ctx, id)
	if err != nil {
		return err
	}
	return fmt.Errorf("%w: current main session %s", store.ErrSessionSuperseded, instance.MainSessionID.String)
}

func (r *SessionRepo) SessionLineageInstanceID(ctx context.Context, sessionID string) (string, error) {
	if err := r.checkReady(ctx, "resolve session instance lineage"); err != nil {
		return "", err
	}
	row, err := r.readQueries.GetInstanceCaller(ctx, sessionID)
	if err != nil {
		return "", err
	}
	id, ok := row.OriginatingInstanceID.(string)
	if !ok {
		return "", errors.New("store: invalid originating instance")
	}
	return id, nil
}

// RequireInstanceSessionActive is the dispatch boundary for direct Loop-owned prompts.
func (r *SessionRepo) RequireInstanceSessionActive(ctx context.Context, sessionID string) error {
	if err := r.checkReady(ctx, "admit instance managed prompt"); err != nil {
		return err
	}
	return r.withImmediateTransaction(ctx, "admit instance managed prompt", func(exec globalSQLExecutor) error {
		return requireInstanceSessionActive(ctx, sqlcgen.New(exec), sessionID)
	})
}

func requireInstanceSessionActive(ctx context.Context, q *sqlcgen.Queries, sessionID string) error {
	caller, err := q.GetInstanceCaller(ctx, sessionID)
	if err != nil {
		return err
	}
	id, ok := caller.OriginatingInstanceID.(string)
	if !ok {
		return errors.New("store: invalid originating instance")
	}
	if id == "" {
		return nil
	}
	instance, err := q.GetAgentInstance(ctx, id)
	if err != nil {
		return err
	}
	if instance.State == goalStatusPaused {
		return fmt.Errorf("%w: resume agent instance %s first", agentinstance.ErrPaused, id)
	}
	return nil
}

func loopOwnedTask(ctx context.Context, q *sqlcgen.Queries, taskRunID string) (bool, error) {
	if taskRunID == "" {
		return false, nil
	}
	run, err := q.GetTaskRun(ctx, taskRunID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return run.LoopRunID.Valid && run.LoopRunID.String != "", nil
}
