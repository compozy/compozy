package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	extensionpkg "github.com/compozy/agh/internal/extension"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	toolspkg "github.com/compozy/agh/internal/tools"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

type nativeToolPolicyResolverDeps struct {
	Config            *aghconfig.Config
	Sessions          nativeToolPolicySessionReader
	WorkspaceResolver workspacepkg.RuntimeResolver
	AgentResolver     nativeToolPolicyAgentResolver
	ExtensionRegistry *extensionpkg.Registry
	ApprovalAvailable bool
}

type nativeToolPolicySessionReader interface {
	Status(ctx context.Context, id string) (*session.Info, error)
}

type nativeToolPolicyAgentResolver interface {
	ResolveAgent(name string, resolved *workspacepkg.ResolvedWorkspace) (aghconfig.AgentDef, error)
}

type nativeToolPolicyResolver struct {
	cfg               *aghconfig.Config
	sessions          nativeToolPolicySessionReader
	workspaceResolver workspacepkg.RuntimeResolver
	agentResolver     nativeToolPolicyAgentResolver
	extensionRegistry *extensionpkg.Registry
	approvalAvailable bool
}

var _ toolspkg.PolicyInputResolver = (*nativeToolPolicyResolver)(nil)

func newNativeToolPolicyResolver(deps nativeToolPolicyResolverDeps) (*nativeToolPolicyResolver, error) {
	if deps.Config == nil {
		return nil, errors.New("daemon: native tool policy config is required")
	}
	return &nativeToolPolicyResolver{
		cfg:               deps.Config,
		sessions:          deps.Sessions,
		workspaceResolver: deps.WorkspaceResolver,
		agentResolver:     deps.AgentResolver,
		extensionRegistry: deps.ExtensionRegistry,
		approvalAvailable: deps.ApprovalAvailable,
	}, nil
}

func newNativeToolPolicyResolverForBoot(state *bootState) (*nativeToolPolicyResolver, error) {
	return newNativeToolPolicyResolver(nativeToolPolicyResolverDeps{
		Config:            &state.cfg,
		Sessions:          state.sessions,
		WorkspaceResolver: state.workspaceResolver,
		AgentResolver: agentCatalogDependency(state.agentCatalog, agentSidecarCatalogs{
			soul:      state.soulCatalog,
			heartbeat: state.heartbeatCatalog,
		}),
		ExtensionRegistry: extensionRegistryDependency(state.registry),
		ApprovalAvailable: true,
	})
}

func (r *nativeToolPolicyResolver) Resolve(ctx context.Context, scope toolspkg.Scope) (toolspkg.PolicyInputs, error) {
	if r == nil || r.cfg == nil {
		return toolspkg.PolicyInputs{}, errors.New("daemon: native tool policy resolver is not configured")
	}
	resolvedScope := normalizeToolPolicyScope(scope)
	info, err := r.sessionInfo(ctx, resolvedScope.SessionID)
	if err != nil {
		return toolspkg.PolicyInputs{}, err
	}
	if info != nil {
		resolvedScope = fillToolPolicyScopeFromSession(resolvedScope, info)
	}
	resolvedWorkspace, cfg, err := r.resolveWorkspaceConfig(ctx, resolvedScope.WorkspaceID)
	if err != nil {
		return toolspkg.PolicyInputs{}, err
	}
	inputs, err := nativeToolPolicyInputs(cfg)
	if err != nil {
		return toolspkg.PolicyInputs{}, err
	}
	inputs.ApprovalAvailable = r.approvalAvailable
	if err := r.applyBundledExtensionTrust(&inputs); err != nil {
		return toolspkg.PolicyInputs{}, err
	}
	if err := applySessionToolPolicy(&inputs, info); err != nil {
		return toolspkg.PolicyInputs{}, err
	}
	if resolvedScope.AgentName != "" {
		if err := r.applyAgentToolPolicy(&inputs, resolvedScope.AgentName, resolvedWorkspace, cfg); err != nil {
			if inputs.Session.Enforced && errors.Is(err, workspacepkg.ErrAgentNotAvailable) {
				return inputs, nil
			}
			return toolspkg.PolicyInputs{}, err
		}
	}
	return inputs, nil
}

func (r *nativeToolPolicyResolver) applyBundledExtensionTrust(inputs *toolspkg.PolicyInputs) error {
	if r == nil || r.extensionRegistry == nil {
		return nil
	}
	names, err := r.extensionRegistry.EnabledBundledNames()
	if err != nil {
		return fmt.Errorf("daemon: list bundled extension tool policy sources: %w", err)
	}
	for _, name := range names {
		grant := toolspkg.SourceGrant{
			Kind:  toolspkg.SourceExtension,
			Owner: strings.TrimSpace(name),
		}
		if err := grant.Validate("bundled_extension_source"); err != nil {
			return fmt.Errorf("daemon: bundled extension source %q: %w", name, err)
		}
		if !sourceGrantExists(inputs.TrustedSources, grant) {
			inputs.TrustedSources = append(inputs.TrustedSources, grant)
		}
	}
	return nil
}

func sourceGrantExists(grants []toolspkg.SourceGrant, target toolspkg.SourceGrant) bool {
	for _, grant := range grants {
		if grant.Kind == target.Kind && grant.Owner == target.Owner {
			return true
		}
	}
	return false
}

func (r *nativeToolPolicyResolver) sessionInfo(
	ctx context.Context,
	sessionID string,
) (*session.Info, error) {
	if strings.TrimSpace(sessionID) == "" || r.sessions == nil {
		return nil, nil
	}
	info, err := r.sessions.Status(ctx, sessionID)
	if err == nil {
		return info, nil
	}
	if errors.Is(err, session.ErrSessionNotFound) {
		return nil, nil
	}
	return nil, fmt.Errorf("daemon: resolve session tool policy %q: %w", sessionID, err)
}

func (r *nativeToolPolicyResolver) resolveWorkspaceConfig(
	ctx context.Context,
	workspaceID string,
) (*workspacepkg.ResolvedWorkspace, *aghconfig.Config, error) {
	cfg := r.cfg
	if strings.TrimSpace(workspaceID) == "" {
		return nil, cfg, nil
	}
	if r.workspaceResolver == nil {
		return nil, cfg, nil
	}
	resolved, err := r.workspaceResolver.Resolve(ctx, workspaceID)
	if err != nil {
		return nil, nil, fmt.Errorf("daemon: resolve workspace tool policy %q: %w", workspaceID, err)
	}
	return &resolved, &resolved.Config, nil
}

func (r *nativeToolPolicyResolver) applyAgentToolPolicy(
	inputs *toolspkg.PolicyInputs,
	agentName string,
	resolvedWorkspace *workspacepkg.ResolvedWorkspace,
	cfg *aghconfig.Config,
) error {
	agent, err := r.resolveAgent(agentName, resolvedWorkspace)
	if err != nil {
		return err
	}
	resolvedAgent, err := cfg.ResolveAgent(agent)
	if err != nil {
		return fmt.Errorf("daemon: resolve agent tool policy %q: %w", agentName, err)
	}
	policy, err := resolvedAgentToolPolicy(resolvedAgent)
	if err != nil {
		return err
	}
	inputs.Agent = policy
	if strings.TrimSpace(resolvedAgent.Permissions) != "" {
		inputs.SystemPermissionMode = nativeToolPermissionMode(aghconfig.PermissionMode(resolvedAgent.Permissions))
	}
	return nil
}

func (r *nativeToolPolicyResolver) resolveAgent(
	agentName string,
	resolvedWorkspace *workspacepkg.ResolvedWorkspace,
) (aghconfig.AgentDef, error) {
	if r.agentResolver != nil {
		agent, err := r.agentResolver.ResolveAgent(agentName, resolvedWorkspace)
		if err != nil {
			return aghconfig.AgentDef{}, fmt.Errorf("daemon: resolve agent tool policy %q: %w", agentName, err)
		}
		return agent, nil
	}
	agent, err := resolveAgentFromWorkspaceSnapshot(agentName, resolvedWorkspace)
	if err != nil {
		return aghconfig.AgentDef{}, fmt.Errorf("daemon: resolve agent tool policy %q: %w", agentName, err)
	}
	return agent, nil
}

func resolvedAgentToolPolicy(resolved aghconfig.ResolvedAgent) (toolspkg.AgentToolPolicy, error) {
	tools, err := toolspkg.ParseToolPatterns(resolved.Tools)
	if err != nil {
		return toolspkg.AgentToolPolicy{}, fmt.Errorf("daemon: agent %q tools: %w", resolved.Name, err)
	}
	denyTools, err := toolspkg.ParseToolPatterns(resolved.DenyTools)
	if err != nil {
		return toolspkg.AgentToolPolicy{}, fmt.Errorf("daemon: agent %q deny_tools: %w", resolved.Name, err)
	}
	toolsets, err := parseAgentToolsets(resolved)
	if err != nil {
		return toolspkg.AgentToolPolicy{}, err
	}
	return toolspkg.AgentToolPolicy{
		Tools:     tools,
		Toolsets:  toolsets,
		DenyTools: denyTools,
	}, nil
}

func parseAgentToolsets(resolved aghconfig.ResolvedAgent) ([]toolspkg.ToolsetID, error) {
	toolsets := make([]toolspkg.ToolsetID, 0, len(resolved.Toolsets))
	for i, raw := range resolved.Toolsets {
		id := toolspkg.ToolsetID(strings.TrimSpace(raw))
		if err := id.Validate(); err != nil {
			return nil, fmt.Errorf("daemon: agent %q toolsets[%d]: %w", resolved.Name, i, err)
		}
		toolsets = append(toolsets, id)
	}
	return toolsets, nil
}

func applySessionToolPolicy(inputs *toolspkg.PolicyInputs, info *session.Info) error {
	if info == nil {
		return nil
	}
	lineage := store.NormalizeSessionLineage(info.ID, info.Lineage)
	if lineage == nil {
		return nil
	}
	policy := store.NormalizeSessionPermissionPolicy(lineage.PermissionPolicy)
	if lineage.ParentSessionID == "" && len(policy.Tools) == 0 {
		return nil
	}
	ids := make([]toolspkg.ToolID, 0, len(policy.Tools))
	for i, raw := range policy.Tools {
		id := toolspkg.ToolID(strings.TrimSpace(raw))
		if err := id.Validate(); err != nil {
			return fmt.Errorf("daemon: session %q lineage tools[%d]: %w", info.ID, i, err)
		}
		ids = append(ids, id)
	}
	inputs.Session = toolspkg.SessionToolPolicy{
		Enforced: true,
		Tools:    ids,
	}
	return nil
}

func normalizeToolPolicyScope(scope toolspkg.Scope) toolspkg.Scope {
	return toolspkg.Scope{
		WorkspaceID: strings.TrimSpace(scope.WorkspaceID),
		SessionID:   strings.TrimSpace(scope.SessionID),
		AgentName:   strings.TrimSpace(scope.AgentName),
		Operator:    scope.Operator,
	}
}

func fillToolPolicyScopeFromSession(scope toolspkg.Scope, info *session.Info) toolspkg.Scope {
	if info == nil {
		return scope
	}
	if strings.TrimSpace(scope.SessionID) == "" {
		scope.SessionID = strings.TrimSpace(info.ID)
	}
	if strings.TrimSpace(scope.WorkspaceID) == "" {
		scope.WorkspaceID = strings.TrimSpace(info.WorkspaceID)
	}
	if strings.TrimSpace(scope.AgentName) == "" {
		scope.AgentName = strings.TrimSpace(info.AgentName)
	}
	return scope
}
