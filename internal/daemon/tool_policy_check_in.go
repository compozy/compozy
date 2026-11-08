package daemon

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/compozy/compozy/internal/agentinstance"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/session"
	toolspkg "github.com/compozy/compozy/internal/tools"
	builtintools "github.com/compozy/compozy/internal/tools/builtin"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

type checkInPolicyStore interface {
	CheckInSessionBinding(context.Context, string) (string, string, error)
	Get(context.Context, string) (agentinstance.Instance, error)
	WakeState(context.Context, string) (heartbeat.InstanceWakeState, error)
	ApprovedAuthority(context.Context, string, string) (string, error)
}

func (r *nativeToolPolicyResolver) resolveCheckInPolicy(
	ctx context.Context,
	inputs *toolspkg.PolicyInputs,
	info *session.Info,
	scope toolspkg.Scope,
	workspace *workspacepkg.ResolvedAgentConfig,
	cfg *compozyconfig.Config,
) error {
	policy := &toolspkg.CheckInPolicy{
		Disabled:      cfg.Agents.AlwaysOn.CheckInTools == hookActionDisabled,
		InstanceID:    info.InstanceID,
		MaxToolCalls:  cfg.Agents.AlwaysOn.CheckInMaxToolCalls,
		MaxWriteCalls: cfg.Agents.AlwaysOn.CheckInMaxWriteCalls,
	}
	inputs.CheckIn = policy
	inputs.ApprovalAvailable = false
	repo, ok := r.instanceCallers.(checkInPolicyStore)
	if !ok {
		return nil
	}
	instanceID, occurrence, err := repo.CheckInSessionBinding(ctx, info.ID)
	if err != nil {
		return err
	}
	if instanceID == "" {
		caller, err := r.instanceCallers.InstanceCaller(ctx, info.ID)
		if err != nil {
			return err
		}
		instanceID = caller.InstanceID
	}
	if instanceID == "" {
		return nil
	}
	policy.InstanceID = instanceID
	policy.OccurrenceID = occurrence
	instance, err := repo.Get(ctx, instanceID)
	if err != nil {
		return err
	}
	if err := r.applyCheckInAuthority(ctx, policy, info, workspace, cfg, repo); err != nil {
		return err
	}
	return r.bindMainCheckInCeiling(ctx, policy, instance.MainSessionID, scope)
}

func (r *nativeToolPolicyResolver) applyCheckInAuthority(
	ctx context.Context,
	policy *toolspkg.CheckInPolicy,
	info *session.Info,
	workspace *workspacepkg.ResolvedAgentConfig,
	cfg *compozyconfig.Config,
	repo checkInPolicyStore,
) error {
	agent, err := r.resolveAgent(info.AgentName, workspace)
	if err != nil {
		return err
	}
	authored, digest, dir, err := r.cachedAuthorityPolicy(ctx, agent, info, workspace, cfg)
	if err != nil {
		return nil
	}
	approved, err := repo.ApprovedAuthority(ctx, info.ProfileID, dir)
	if err != nil {
		return nil
	}
	if digest != "" && approved == digest {
		policy.AuthorityApproved = true
		policy.AllowWrites = authored.Frontmatter.CheckIns.AllowWrites
		if v := authored.Frontmatter.CheckIns.MaxToolCalls; v != nil && *v < policy.MaxToolCalls {
			policy.MaxToolCalls = *v
		}
		if v := authored.Frontmatter.CheckIns.MaxWriteCalls; v != nil && *v < policy.MaxWriteCalls {
			policy.MaxWriteCalls = *v
		}
	}
	return nil
}

func (r *nativeToolPolicyResolver) bindMainCheckInCeiling(
	ctx context.Context,
	policy *toolspkg.CheckInPolicy,
	mainSessionID string,
	scope toolspkg.Scope,
) error {
	if mainSessionID == "" {
		policy.NoMainSession = true
		return nil
	}
	mainScope := scope
	mainScope.SessionID = mainSessionID
	mainInputs, err := r.Resolve(ctx, mainScope)
	if err != nil {
		return err
	}
	mainInputs.ApprovalAvailable = true
	catalog, err := builtintools.ToolsetCatalog()
	if err != nil {
		return err
	}
	descriptors := builtintools.NativeDescriptors()
	ids := make([]toolspkg.ToolID, 0, len(descriptors)+1)
	for _, d := range descriptors {
		ids = append(ids, d.ID)
	}
	mainEvaluator, err := toolspkg.NewEffectivePolicyEvaluator(mainInputs, catalog, ids)
	if err != nil {
		return err
	}
	ceilingInputs := mainInputs
	if ceilingInputs.SystemPermissionMode != toolspkg.PermissionModeDenyAll {
		ceilingInputs.SystemPermissionMode = toolspkg.PermissionModeApproveAll
	}
	ceilingEvaluator, err := toolspkg.NewEffectivePolicyEvaluator(ceilingInputs, catalog, ids)
	if err != nil {
		return err
	}
	evaluate := func(callCtx context.Context, d toolspkg.Descriptor, input json.RawMessage, readsOnly bool) bool {
		if callCtx.Err() != nil {
			return false
		}
		evaluator := mainEvaluator
		if readsOnly && toolspkg.AccessOf(d) == toolspkg.AccessWrite {
			evaluator = ceilingEvaluator
		}
		decision, err := evaluator.EvaluateInput(d, input)
		if err != nil || !decision.Callable {
			return false
		}
		if !decision.ApprovalRequired {
			return true
		}
		if r.approvalGrants == nil {
			return false
		}
		key, err := toolApprovalGrantKey(mainScope, toolspkg.CallRequest{Input: input}, d.ID)
		if err != nil {
			return false
		}
		grant, found, err := r.approvalGrants.LookupApprovalGrant(callCtx, key)
		return err == nil && found && grant.Decision == toolspkg.ApprovalGrantAllow
	}
	policy.Ceiling = func(d toolspkg.Descriptor, input json.RawMessage) bool { return evaluate(ctx, d, input, true) }
	policy.RunsUnattended = func(callCtx context.Context, d toolspkg.Descriptor, input json.RawMessage) bool {
		return evaluate(callCtx, d, input, false)
	}
	return nil
}

func (r *nativeToolPolicyResolver) applyAgentMCPReadAuthority(
	ctx context.Context,
	inputs *toolspkg.PolicyInputs,
	info *session.Info,
	workspace *workspacepkg.ResolvedAgentConfig,
	cfg *compozyconfig.Config,
) error {
	agent, err := r.resolveAgent(info.AgentName, workspace)
	if err != nil {
		return err
	}
	overrides := heartbeat.AuthorityOverrides(agent.MCPServers)
	if len(overrides) == 0 {
		return nil
	}
	policy := &toolspkg.CheckInPolicy{}
	repo, ok := r.instanceCallers.(checkInPolicyStore)
	approved := false
	if ok {
		if err := r.applyCheckInAuthority(ctx, policy, info, workspace, cfg, repo); err != nil {
			return err
		}
		approved = policy.AuthorityApproved
	}
	inputs.MCPReadOnlyTools = map[string]bool{}
	for server, names := range overrides {
		for _, name := range names {
			id, err := toolspkg.Canonicalize(server, name)
			if err != nil {
				slog.WarnContext(ctx, "mcp_read_only_override_invalid", "agent_name", info.AgentName,
					"session_id", info.ID, "server", server, "tool", name, "error", err)
				continue
			}
			inputs.MCPReadOnlyTools[string(id)] = approved
		}
	}
	return nil
}

func (r *nativeToolPolicyResolver) applyResolvedMCPReadAuthority(
	ctx context.Context,
	inputs *toolspkg.PolicyInputs,
	info *session.Info,
	resolvedScope toolspkg.Scope,
	resolvedWorkspace *workspacepkg.ResolvedAgentConfig,
	cfg *compozyconfig.Config,
) error {
	if resolvedScope.AgentName != "" &&
		workspaceaccess.ActorKind(resolvedScope.ActorKind) != workspaceaccess.ActorDaemon {
		authorityInfo := info
		if authorityInfo == nil {
			authorityInfo = &session.Info{
				AgentName:   resolvedScope.AgentName,
				ProfileID:   resolvedScope.ProfileID,
				WorkspaceID: resolvedScope.WorkspaceID,
			}
		}
		if err := r.applyAgentMCPReadAuthority(ctx, inputs, authorityInfo, resolvedWorkspace, cfg); err != nil {
			return err
		}
	}
	return nil
}
