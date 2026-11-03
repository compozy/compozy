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
	if err := r.checkReady(ctx, "resolve instance caller"); err != nil {
		return agentinstance.Caller{}, err
	}
	row, err := r.queries.GetInstanceCaller(ctx, sessionID)
	if err != nil {
		return agentinstance.Caller{}, err
	}
	instanceID, ok := row.OriginatingInstanceID.(string)
	if !ok {
		return agentinstance.Caller{}, errors.New("store: invalid originating instance")
	}
	depth, ok := row.InstanceDepth.(int64)
	if !ok {
		return agentinstance.Caller{}, errors.New("store: invalid instance depth")
	}
	result := agentinstance.Caller{
		SessionID:     row.ID,
		AgentName:     row.AgentName,
		ProfileID:     row.ProfileID,
		InstanceID:    instanceID,
		InstanceDepth: int(depth),
		Grant:         row.EffectiveWorkspaceAccess,
	}
	if err := json.Unmarshal([]byte(row.PermissionPolicyJson), &result.PermissionPolicy); err != nil {
		return agentinstance.Caller{}, err
	}
	return result, nil
}

func (r *SessionRepo) CheckSessionSuperseded(ctx context.Context, sessionID string) error {
	if err := r.checkReady(ctx, "check session supersession"); err != nil {
		return err
	}
	return checkSessionSuperseded(ctx, r.queries, sessionID)
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
	row, err := r.queries.GetInstanceCaller(ctx, sessionID)
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
