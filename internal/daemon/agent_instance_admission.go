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
	authority   session.MCPAuthorityResolver
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
	if definition.Kind != compozyconfig.AgentKindAlwaysOn {
		return agentinstance.Instance{}, agentinstance.ErrNotAlwaysOn
	}
	result := agentinstance.Instance{
		ProfileID:     req.Profile,
		WorkspaceID:   req.Workspace,
		AgentName:     req.Agent,
		CreatedByKind: string(req.Actor.Kind),
		CreatedByID:   req.Actor.SessionID,
	}
	policy := store.DecodeInstanceCeilingIntent(definition.Policy, 0)
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
		policy, err = a.intersectCreateAuthority(ctx, req, caller.PermissionPolicy, policy)
		if err != nil {
			return agentinstance.Instance{}, err
		}
	}
	policy.MaterializedTools, policy.MaterializedMCPServers = false, false
	encoded, err := json.Marshal(policy)
	if err != nil {
		return agentinstance.Instance{}, err
	}
	result.PermissionCeiling = string(encoded)
	return result, nil
}

// Same-scope instances retain the caller's stored intent. Only a scope change
// freezes live authority, so later source admission cannot widen cross-workspace reach.
func (a *instanceAdmission) intersectCreateAuthority(
	ctx context.Context, req agentinstance.CreateRequest, caller, definition store.SessionPermissionPolicy,
) (store.SessionPermissionPolicy, error) {
	ceiling := caller
	if a.authority != nil {
		var err error
		ceiling, err = a.authority.EffectiveMCPAuthority(ctx, req.Actor.SessionID)
		if err != nil {
			return store.SessionPermissionPolicy{}, err
		}
	}
	crossScope := req.Actor.WorkspaceID != req.Workspace
	if crossScope {
		caller = ceiling
		caller.MCPServersMode, caller.ToolsMode = store.PermissionList, store.PermissionList
	}
	policy := agentinstance.IntersectPermissions(caller, definition)
	if crossScope {
		policy.MCPServersMode, policy.ToolsMode = store.PermissionList, store.PermissionList
		policy.AncestorMCPCeiling = store.IntersectMCPCeiling(policy.AncestorMCPCeiling, ceiling.MCPServers)
	}
	candidate := policy
	if a.authority != nil {
		pending := &session.Info{
			ID: req.Actor.SessionID + "/pending-instance", ProfileID: req.Profile, WorkspaceID: req.Workspace,
			AgentName: req.Agent, Type: session.SessionTypeSystem,
			Lineage: &store.SessionLineage{PermissionPolicy: policy},
		}
		var err error
		candidate, err = a.authority.EffectiveMCPAuthority(session.WithMCPAuthoritySession(ctx, pending), pending.ID)
		if err != nil {
			return store.SessionPermissionPolicy{}, err
		}
	}
	return policy, session.ValidatePermissionSubset(ceiling, candidate)
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
	if definition.Kind != compozyconfig.AgentKindAlwaysOn {
		return agentinstance.ErrNotAlwaysOn
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
	if err := a.validateAdoptionAuthority(ctx, &info, ceiling, policy); err != nil {
		return agentinstance.Instance{}, err
	}
	policy = agentinstance.IntersectPermissions(ceiling, policy)
	if info.EffectiveWorkspaceAccess == workspaceaccess.Unrestricted &&
		instance.GrantCeiling != workspaceaccess.Unrestricted {
		return agentinstance.Instance{}, agentinstance.ErrWorkspaceAccessDenied
	}
	policy.MaterializedTools, policy.MaterializedMCPServers = false, false
	encoded, err := json.Marshal(policy)
	if err != nil {
		return agentinstance.Instance{}, err
	}
	instance.PermissionCeiling = string(encoded)
	return instance, nil
}

func (a *instanceAdmission) validateAdoptionAuthority(
	ctx context.Context, info *store.SessionInfo, ceiling, candidate store.SessionPermissionPolicy,
) error {
	if a.authority != nil {
		var err error
		candidate, err = a.authority.EffectiveMCPAuthority(ctx, info.ID)
		if err != nil {
			return err
		}
		pending := &session.Info{ID: info.ID, ProfileID: info.ProfileID, WorkspaceID: info.WorkspaceID,
			AgentName: info.AgentName, Type: session.Type(info.SessionType),
			Lineage: &store.SessionLineage{PermissionPolicy: ceiling}}
		ceiling, err = a.authority.EffectiveMCPAuthority(session.WithMCPAuthoritySession(ctx, pending), info.ID)
		if err != nil {
			return err
		}
	}
	if err := session.ValidatePermissionSubset(ceiling, candidate); err != nil {
		return agentinstance.ErrWorkspaceAccessDenied
	}
	return nil
}
