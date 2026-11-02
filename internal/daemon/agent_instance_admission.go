package daemon

import (
	"context"
	"encoding/json/v2"

	"github.com/compozy/compozy/internal/agentinstance"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

type instanceDefinition struct {
	Kind   string
	Grant  string
	Policy store.SessionPermissionPolicy
}

type instanceDefinitionReader interface {
	InstanceDefinition(context.Context, string, string, string) (instanceDefinition, error)
}

type instanceAdoptionReader interface {
	AdoptionSession(context.Context, string) (store.SessionInfo, store.SessionPermissionPolicy, error)
}

type instanceReach interface {
	AuthorizeInstanceWorkspace(context.Context, workspaceaccess.ActorRef, string, string) error
}

type instanceAdmission struct {
	definitions instanceDefinitionReader
	callers     agentinstance.CallerReader
	sessions    instanceAdoptionReader
	reach       instanceReach
}

var _ agentinstance.Admission = (*instanceAdmission)(nil)

func (a *instanceAdmission) ResolveCreate(
	ctx context.Context,
	req agentinstance.CreateRequest,
) (agentinstance.Instance, error) {
	if err := a.reach.AuthorizeInstanceWorkspace(ctx, req.Actor, req.Workspace, req.Profile); err != nil {
		return agentinstance.Instance{}, err
	}
	definition, err := a.definitions.InstanceDefinition(ctx, req.Profile, req.Workspace, req.Agent)
	if err != nil {
		return agentinstance.Instance{}, err
	}
	if definition.Kind != compozyconfig.AgentKindStaysOn {
		return agentinstance.Instance{}, agentinstance.ErrNotStaysOn
	}
	result := agentinstance.Instance{
		ProfileID:     req.Profile,
		WorkspaceID:   req.Workspace,
		AgentName:     req.Agent,
		CreatedByKind: string(req.Actor.Kind),
		CreatedByID:   req.Actor.SessionID,
	}
	policy := definition.Policy
	if req.Actor.Operator {
		if req.Workspace == "" {
			result.GrantCeiling = definition.Grant
		}
	} else {
		if req.Actor.Kind != workspaceaccess.ActorAgentSession {
			return agentinstance.Instance{}, agentinstance.ErrForbidden
		}
		caller, err := a.callers.InstanceCaller(ctx, req.Actor.SessionID)
		if err != nil {
			return agentinstance.Instance{}, err
		}
		if caller.AgentName != req.Agent || caller.ProfileID != req.Profile {
			return agentinstance.Instance{}, agentinstance.ErrForbidden
		}
		if caller.InstanceDepth >= 1 {
			return agentinstance.Instance{}, agentinstance.ErrDepth
		}
		result.Depth = 1
		policy = agentinstance.IntersectPermissions(caller.PermissionPolicy, definition.Policy)
		if err := session.ValidatePermissionSubset(caller.PermissionPolicy, policy); err != nil {
			return agentinstance.Instance{}, err
		}
	}
	encoded, err := json.Marshal(policy)
	if err != nil {
		return agentinstance.Instance{}, err
	}
	result.PermissionCeiling = string(encoded)
	return result, nil
}

func (a *instanceAdmission) ValidateResume(ctx context.Context, instance agentinstance.Instance) error {
	definition, err := a.definitions.InstanceDefinition(
		ctx,
		instance.ProfileID,
		instance.WorkspaceID,
		instance.AgentName,
	)
	if err != nil {
		return err
	}
	if definition.Kind != compozyconfig.AgentKindStaysOn {
		return agentinstance.ErrNotStaysOn
	}
	return nil
}

func (a *instanceAdmission) ResolveAdopt(
	ctx context.Context,
	req agentinstance.AdoptRequest,
) (agentinstance.Instance, error) {
	if !req.Actor.Operator {
		return agentinstance.Instance{}, agentinstance.ErrForbidden
	}
	info, policy, err := a.sessions.AdoptionSession(ctx, req.SessionID)
	if err != nil {
		return agentinstance.Instance{}, err
	}
	if info.Lineage != nil && info.Lineage.ParentSessionID != "" {
		return agentinstance.Instance{}, agentinstance.ErrSessionNotRoot
	}
	if info.SessionType != string(TurnOriginUser) || info.ArchivedAt != nil {
		return agentinstance.Instance{}, agentinstance.ErrScopeMismatch
	}
	if info.ProfileID != req.Profile || info.AgentName != req.Agent || info.WorkspaceID != req.Workspace {
		return agentinstance.Instance{}, agentinstance.ErrScopeMismatch
	}
	instance, err := a.ResolveCreate(ctx, req.CreateRequest)
	if err != nil {
		return agentinstance.Instance{}, err
	}
	var ceiling store.SessionPermissionPolicy
	if err := json.Unmarshal([]byte(instance.PermissionCeiling), &ceiling); err != nil {
		return agentinstance.Instance{}, err
	}
	if err := session.ValidatePermissionSubset(ceiling, policy); err != nil {
		return agentinstance.Instance{}, agentinstance.ErrWorkspaceAccessDenied
	}
	if info.EffectiveWorkspaceAccess == workspaceaccess.Unrestricted &&
		instance.GrantCeiling != workspaceaccess.Unrestricted {
		return agentinstance.Instance{}, agentinstance.ErrWorkspaceAccessDenied
	}
	encoded, err := json.Marshal(policy)
	if err != nil {
		return agentinstance.Instance{}, err
	}
	instance.PermissionCeiling = string(encoded)
	return instance, nil
}
