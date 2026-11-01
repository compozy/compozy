package globaldb

import (
	"context"
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
	result := agentinstance.Caller{SessionID: row.ID, AgentName: row.AgentName, ProfileID: row.ProfileID, InstanceID: instanceID, InstanceDepth: int(depth), Grant: row.EffectiveWorkspaceAccess}
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
