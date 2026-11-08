package daemon

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"

	"github.com/compozy/compozy/internal/agentinstance"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/session"
	toolspkg "github.com/compozy/compozy/internal/tools"
	builtintools "github.com/compozy/compozy/internal/tools/builtin"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
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
		Disabled:      cfg.Agents.AlwaysOn.CheckInTools == "disabled",
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
	root := ""
	if workspace != nil {
		root = workspace.RootDir
	}
	var policyResolver heartbeat.PolicyResolver
	if resolver, ok := r.agentResolver.(heartbeat.PolicyResolver); ok {
		policyResolver = resolver
	}
	authored, err := resolveTimerPolicy(
		ctx,
		policyResolver,
		heartbeat.AuthoringTarget{
			AgentPath:     agent.SourcePath,
			WorkspaceRoot: root,
			AgentName:     info.AgentName,
			ProfileID:     info.ProfileID,
			WorkspaceID:   info.WorkspaceID,
			Config:        cfg.Agents.Heartbeat,
			CheckInLimits: &heartbeat.CheckInLimits{
				MaxToolCalls:  cfg.Agents.AlwaysOn.CheckInMaxToolCalls,
				MaxWriteCalls: cfg.Agents.AlwaysOn.CheckInMaxWriteCalls,
			},
		},
	)
	if err != nil {
		return err
	}
	digest, err := heartbeat.AuthorityDigest(
		authored.Frontmatter.CheckIns,
		heartbeat.AuthorityActions(authored.Frontmatter.Actions),
	)
	if err != nil {
		return err
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(agent.SourcePath))
	if err != nil {
		return err
	}
	approved, err := repo.ApprovedAuthority(ctx, info.ProfileID, dir)
	if err != nil {
		return err
	}
	if digest != "" && approved == digest {
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
	evaluate := func(d toolspkg.Descriptor, input json.RawMessage, readsOnly bool) bool {
		selected := mainInputs
		if selected.SystemPermissionMode == toolspkg.PermissionModeDenyAll {
			return false
		}
		if readsOnly && toolspkg.AccessOf(d) == toolspkg.AccessWrite {
			selected.SystemPermissionMode = toolspkg.PermissionModeApproveAll
		}
		evaluator, err := toolspkg.NewEffectivePolicyEvaluator(selected, catalog, append(slices.Clone(ids), d.ID))
		if err != nil {
			return false
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
		grant, found, err := r.approvalGrants.LookupApprovalGrant(ctx, key)
		return err == nil && found && grant.Decision == toolspkg.ApprovalGrantAllow
	}
	policy.Ceiling = func(d toolspkg.Descriptor, input json.RawMessage) bool { return evaluate(d, input, true) }
	policy.RunsUnattended = func(_ context.Context, d toolspkg.Descriptor, input json.RawMessage) bool {
		return evaluate(d, input, false)
	}
	return nil
}
