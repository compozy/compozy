package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	callspkg "github.com/compozy/compozy/internal/calls"
	"github.com/compozy/compozy/internal/network/participation"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
)

type callSessionContext struct {
	id, profileID, workspaceID, state, parentID, rootID string
	agentName, provider, model, reasoningEffort, speed  string
	depth                                               int
	parkedAt, idleExpiresAt                             sql.NullString
	permissionJSON                                      string
}

func (g *CallRepo) ResolveCallTargetContext(
	ctx context.Context,
	input callspkg.CreateInput,
) (callspkg.TargetContext, error) {
	if err := g.checkReady(ctx, "resolve call target"); err != nil {
		return callspkg.TargetContext{}, err
	}
	callerID, err := g.resolveCallCallerSessionID(ctx, input.Caller)
	if err != nil {
		return callspkg.TargetContext{}, err
	}
	caller, err := g.loadCallSessionContext(ctx, callerID)
	if err != nil {
		return callspkg.TargetContext{}, err
	}
	policy, err := store.DecodeSessionPermissionPolicy(caller.permissionJSON)
	if err != nil {
		return callspkg.TargetContext{}, fmt.Errorf("store: decode caller permissions: %w", err)
	}
	rootID := caller.rootID
	if rootID == "" {
		rootID = caller.id
	}
	base := callspkg.TargetContext{
		ProfileID: caller.profileID, WorkspaceID: caller.workspaceID,
		ParentSessionID: caller.id, GovernedRootID: rootID, Depth: caller.depth + 1,
		CallerPolicy: policy, Allowed: caller.profileID == strings.TrimSpace(input.ProfileID),
	}
	if strings.TrimSpace(input.Target.Agent) != "" {
		if err := g.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions child
			WHERE child.parent_session_id = ? AND child.state <> 'stopped'
			AND NOT EXISTS (SELECT 1 FROM operator_caller_sessions o WHERE o.session_id = child.id)`, caller.id).
			Scan(&base.LiveChildren); err != nil {
			return callspkg.TargetContext{}, fmt.Errorf("store: count live call children: %w", err)
		}
		return base, nil
	}
	target, err := g.loadCallSessionContext(ctx, input.Target.SessionID)
	if errors.Is(err, sql.ErrNoRows) {
		base.State = callspkg.TargetStateMissing
		return base, nil
	}
	if err != nil {
		return callspkg.TargetContext{}, err
	}
	operator, err := g.IsOperatorCallerSession(ctx, target.id)
	if err != nil {
		return callspkg.TargetContext{}, err
	}
	targetRoot := target.rootID
	if targetRoot == "" {
		targetRoot = target.id
	}
	base.ProfileID = target.profileID
	base.WorkspaceID = target.workspaceID
	base.ChildSessionID = target.id
	base.AgentName = ""
	base.GovernedRootID = targetRoot
	base.Depth = target.depth
	base.Runtime.Provider = target.provider
	base.Runtime.Model = target.model
	base.Runtime.ReasoningEffort = target.reasoningEffort
	if target.speed != "" {
		parsedSpeed, parseErr := speedpkg.Parse(target.speed)
		if parseErr != nil {
			return callspkg.TargetContext{}, fmt.Errorf("store: parse call target speed: %w", parseErr)
		}
		base.Runtime.Speed = parsedSpeed
	}
	base.Allowed = !operator && target.profileID == caller.profileID &&
		target.workspaceID == caller.workspaceID && target.parentID == caller.id && targetRoot == rootID
	base.State = callspkg.TargetState(target.state)
	if target.parkedAt.Valid {
		base.State = callspkg.TargetStateParked
	}
	if target.idleExpiresAt.Valid {
		expiresAt, parseErr := store.ParseTimestamp(target.idleExpiresAt.String)
		if parseErr != nil {
			return callspkg.TargetContext{}, fmt.Errorf("store: parse call target idle expiry: %w", parseErr)
		}
		base.ExpiredAt = expiresAt
		if !expiresAt.After(g.now()) {
			base.State = callspkg.TargetStateExpired
		}
	}
	return base, nil
}

func (g *CallRepo) resolveCallCallerSessionID(ctx context.Context, owner participation.OwnerRef) (string, error) {
	id := strings.TrimSpace(owner.ID)
	var sessionID sql.NullString
	var err error
	switch owner.Kind {
	case participation.OwnerKindSession:
		sessionID = sql.NullString{String: id, Valid: id != ""}
	case participation.OwnerKindTaskRun:
		err = g.db.QueryRowContext(ctx, `SELECT session_id FROM task_runs WHERE id = ?`, id).Scan(&sessionID)
	case participation.OwnerKindLoopRun:
		err = g.db.QueryRowContext(ctx, `SELECT COALESCE(origin_session_id, (
			SELECT session_id FROM loop_session_bindings WHERE loop_run_id = loop_runs.id AND state = 'active'
			ORDER BY binding_epoch DESC LIMIT 1)) FROM loop_runs WHERE id = ?`, id).Scan(&sessionID)
	case participation.OwnerKindAutomationRun:
		err = g.db.QueryRowContext(ctx, `SELECT COALESCE(session_id, (
			SELECT session_id FROM task_runs WHERE id = automation_runs.task_run_id))
			FROM automation_runs WHERE id = ?`, id).Scan(&sessionID)
	default:
		return "", &callspkg.Error{Code: callspkg.CodeTargetDenied, Message: "unsupported caller owner"}
	}
	if errors.Is(err, sql.ErrNoRows) || !sessionID.Valid || strings.TrimSpace(sessionID.String) == "" {
		return "", &callspkg.Error{Code: callspkg.CodeTargetDenied, Message: "caller has no live session"}
	}
	if err != nil {
		return "", fmt.Errorf("store: resolve call caller session: %w", err)
	}
	return strings.TrimSpace(sessionID.String), nil
}

func (g *CallRepo) loadCallSessionContext(ctx context.Context, id string) (callSessionContext, error) {
	var item callSessionContext
	err := g.db.QueryRowContext(ctx, `SELECT id, profile_id, workspace_id, state,
		COALESCE(parent_session_id, ''), COALESCE(root_session_id, ''), spawn_depth,
		agent_name, provider, model, reasoning_effort, speed, parked_at, idle_expires_at,
		permission_policy_json FROM sessions WHERE id = ?`, strings.TrimSpace(id)).Scan(
		&item.id, &item.profileID, &item.workspaceID, &item.state, &item.parentID, &item.rootID,
		&item.depth, &item.agentName, &item.provider, &item.model, &item.reasoningEffort, &item.speed,
		&item.parkedAt, &item.idleExpiresAt, &item.permissionJSON,
	)
	if err != nil {
		return callSessionContext{}, err
	}
	return item, nil
}
