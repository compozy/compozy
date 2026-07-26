package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

const (
	roleErrorUnknown       = contract.CodeRoleUnknown
	roleErrorAgentNotFound = contract.CodeRoleAgentNotFound
	roleFieldTimeout       = "timeout"
)

// RoleResolutionError is a deterministic role-resolution failure.
type RoleResolutionError struct {
	Code  string
	Role  aghconfig.RoleName
	Agent string
	Err   error
}

func (e *RoleResolutionError) Error() string {
	if e == nil {
		return "role resolution failed"
	}
	parts := []string{e.Code, string(e.Role)}
	if strings.TrimSpace(e.Agent) != "" {
		parts = append(parts, strings.TrimSpace(e.Agent))
	}
	if e.Err != nil {
		parts = append(parts, e.Err.Error())
	}
	return strings.Join(parts, ": ")
}

func (e *RoleResolutionError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// DiagnosticCode returns the stable public code for transport projections.
func (e *RoleResolutionError) DiagnosticCode() string {
	if e == nil {
		return ""
	}
	return strings.TrimSpace(e.Code)
}

// ResolvedRole is one invocation-time role routing decision.
type ResolvedRole struct {
	Role            aghconfig.RoleName
	Enabled         bool
	AgentName       string
	Builtin         bool
	Inherit         bool
	AgentDef        aghconfig.AgentDef
	Provider        string
	Model           string
	ReasoningEffort string
	Fallbacks       []aghconfig.RoleFallback
	Provenance      map[string]string
	eventWriter     roleEventSummaryWriter
}

type RoleResolver interface {
	Resolve(ctx context.Context, workspaceID string, role aghconfig.RoleName) (ResolvedRole, error)
}

type roleResolver struct {
	config            *aghconfig.Config
	workspaceResolver workspacepkg.RuntimeResolver
	agents            coordinatorAgentResolver
	events            roleEventSummaryWriter
}

var _ RoleResolver = (*roleResolver)(nil)

func newRoleResolver(
	cfg *aghconfig.Config,
	workspaceResolver workspacepkg.RuntimeResolver,
	agents coordinatorAgentResolver,
	eventWriters ...roleEventSummaryWriter,
) *roleResolver {
	resolver := &roleResolver{config: cfg, workspaceResolver: workspaceResolver, agents: agents}
	if len(eventWriters) > 0 {
		resolver.events = eventWriters[0]
	}
	return resolver
}

func (r *roleResolver) Resolve(
	ctx context.Context,
	workspaceID string,
	role aghconfig.RoleName,
) (ResolvedRole, error) {
	resolved, _, err := r.resolveEffective(ctx, workspaceID, role)
	if err != nil {
		return ResolvedRole{}, errors.Join(err, r.recordRoleResolveError(ctx, workspaceID, role, err))
	}
	resolved.eventWriter = r.events
	return resolved, nil
}

func (r *roleResolver) resolveEffective(
	ctx context.Context,
	workspaceID string,
	role aghconfig.RoleName,
) (ResolvedRole, *aghconfig.Config, error) {
	effectiveConfig, resolvedWorkspace, err := r.effectiveRoleConfig(ctx, workspaceID)
	if err != nil {
		return ResolvedRole{}, nil, err
	}

	common, err := roleConfig(&effectiveConfig.Roles, role)
	if err != nil {
		return ResolvedRole{}, nil, err
	}
	resolved := ResolvedRole{
		Role:            role,
		Enabled:         common.Enabled,
		Fallbacks:       append([]aghconfig.RoleFallback(nil), common.FallbackChain...),
		Provenance:      roleProvenance(role, effectiveConfig),
		Provider:        strings.TrimSpace(common.Provider),
		Model:           strings.TrimSpace(common.Model),
		ReasoningEffort: strings.TrimSpace(common.ReasoningEffort),
	}
	if !resolved.Enabled {
		populateDisabledRoleIdentity(role, common, &resolved)
		return resolved, effectiveConfig, nil
	}

	agentName := strings.TrimSpace(common.Agent)
	if agentName == "" {
		agentName, resolved.Inherit = defaultRoleIdentity(role)
	}
	if resolved.Inherit {
		correlation := roleInvocationCorrelationFromContext(ctx, workspaceID)
		agentName = strings.TrimSpace(correlation.AgentName)
		if agentName == "" {
			return resolved, effectiveConfig, nil
		}
	}
	if agentName != "" {
		agentDef, builtin, resolveErr := r.resolveRoleAgent(role, agentName, resolvedWorkspace)
		if resolveErr != nil {
			return ResolvedRole{}, nil, resolveErr
		}
		resolved.AgentName = agentName
		resolved.AgentDef = agentDef
		resolved.Builtin = builtin
	}

	if role == aghconfig.RoleMemoryController {
		return resolved, effectiveConfig, nil
	}
	if err := resolveRoleRuntime(effectiveConfig, common, &resolved); err != nil {
		return ResolvedRole{}, nil, fmt.Errorf("daemon: resolve %s role runtime: %w", role, err)
	}
	return resolved, effectiveConfig, nil
}

func populateDisabledRoleIdentity(
	role aghconfig.RoleName,
	common aghconfig.RoleConfig,
	resolved *ResolvedRole,
) {
	if resolved == nil {
		return
	}
	agentName := strings.TrimSpace(common.Agent)
	if agentName == "" {
		agentName, resolved.Inherit = defaultRoleIdentity(role)
	}
	if builtin, ok := aghconfig.BuiltinAgentDef(agentName); ok {
		resolved.AgentName = agentName
		resolved.AgentDef = builtin
		resolved.Builtin = true
	}
}

func (r *roleResolver) effectiveRoleConfig(
	ctx context.Context,
	workspaceID string,
) (*aghconfig.Config, *workspacepkg.ResolvedWorkspace, error) {
	if ctx == nil {
		return nil, nil, errors.New("daemon: role resolver context is required")
	}
	if r == nil || r.config == nil {
		return nil, nil, errors.New("daemon: role resolver config is required")
	}
	target := strings.TrimSpace(workspaceID)
	if target == "" {
		return r.config, nil, nil
	}
	if r.workspaceResolver == nil {
		return nil, nil, errors.New("daemon: workspace resolver is required for workspace role resolution")
	}
	resolved, err := r.workspaceResolver.Resolve(ctx, target)
	if err != nil {
		return nil, nil, fmt.Errorf("daemon: resolve role workspace %q: %w", target, err)
	}
	return &resolved.Config, &resolved, nil
}

func (r *roleResolver) resolveRoleAgent(
	role aghconfig.RoleName,
	name string,
	resolvedWorkspace *workspacepkg.ResolvedWorkspace,
) (aghconfig.AgentDef, bool, error) {
	if builtin, ok := aghconfig.BuiltinAgentDef(name); ok {
		return builtin, true, nil
	}
	if r.agents == nil {
		return aghconfig.AgentDef{}, false, &RoleResolutionError{
			Code: roleErrorAgentNotFound, Role: role, Agent: name, Err: workspacepkg.ErrAgentNotAvailable,
		}
	}
	agent, err := r.agents.ResolveAgent(name, resolvedWorkspace)
	if err != nil {
		if errors.Is(err, workspacepkg.ErrAgentNotAvailable) {
			return aghconfig.AgentDef{}, false, &RoleResolutionError{
				Code: roleErrorAgentNotFound, Role: role, Agent: name, Err: err,
			}
		}
		return aghconfig.AgentDef{}, false, fmt.Errorf("daemon: resolve role agent %q: %w", name, err)
	}
	return agent, false, nil
}

func resolveRoleRuntime(cfg *aghconfig.Config, common aghconfig.RoleConfig, resolved *ResolvedRole) error {
	if resolved == nil {
		return errors.New("resolved role is required")
	}
	route := aghconfig.CloneAgentDef(resolved.AgentDef)
	route.Provider = firstRoleValue(common.Provider, route.Provider)
	route.Model = firstRoleValue(common.Model, route.Model)
	route.ReasoningEffort = firstRoleValue(common.ReasoningEffort, route.ReasoningEffort)
	if strings.TrimSpace(route.Provider) == "" && cfg != nil {
		route.Provider = strings.TrimSpace(cfg.Defaults.Provider)
	}
	if strings.TrimSpace(route.Provider) == "" {
		if strings.TrimSpace(route.Model) != "" {
			return errors.New("provider is required when model is set")
		}
		resolved.ReasoningEffort = strings.TrimSpace(route.ReasoningEffort)
		return nil
	}
	if cfg == nil {
		return errors.New("config is required")
	}
	runtime, err := cfg.ResolveAgent(route)
	if err != nil {
		return err
	}
	resolved.Provider = runtime.Provider
	resolved.Model = runtime.Model
	resolved.ReasoningEffort = runtime.ReasoningEffort
	return nil
}

func roleConfig(roles *aghconfig.RolesConfig, role aghconfig.RoleName) (aghconfig.RoleConfig, error) {
	if roles == nil {
		return aghconfig.RoleConfig{}, errors.New("roles config is required")
	}
	switch role {
	case aghconfig.RoleCoordinator:
		return roles.Coordinator.RoleConfig, nil
	case aghconfig.RoleDream:
		return roles.Dream, nil
	case aghconfig.RoleCheckpointSummary:
		return roles.CheckpointSummary, nil
	case aghconfig.RoleMemoryExtractor:
		return roles.MemoryExtractor, nil
	case aghconfig.RoleAutoTitle:
		return roles.AutoTitle, nil
	case aghconfig.RoleMemoryController:
		return aghconfig.RoleConfig{
			Enabled:         roles.MemoryController.Enabled,
			Provider:        roles.MemoryController.Provider,
			Model:           roles.MemoryController.Model,
			ReasoningEffort: roles.MemoryController.ReasoningEffort,
			FallbackChain:   roles.MemoryController.FallbackChain,
		}, nil
	default:
		return aghconfig.RoleConfig{}, &RoleResolutionError{Code: roleErrorUnknown, Role: role}
	}
}

func defaultRoleIdentity(role aghconfig.RoleName) (string, bool) {
	switch role {
	case aghconfig.RoleCoordinator:
		return aghconfig.BuiltinCoordinatorAgentName, false
	case aghconfig.RoleDream, aghconfig.RoleCheckpointSummary:
		return aghconfig.BuiltinDreamingCuratorAgentName, false
	case aghconfig.RoleMemoryExtractor, aghconfig.RoleAutoTitle:
		return "", true
	case aghconfig.RoleMemoryController:
		return "", false
	default:
		return "", false
	}
}

func roleProvenance(
	role aghconfig.RoleName,
	effective *aghconfig.Config,
) map[string]string {
	effectiveFields := roleFields(role, &effective.Roles)
	provenance := make(map[string]string, len(effectiveFields))
	for field := range effectiveFields {
		source := effective.RoleFieldSource(role, field)
		if source == "" {
			source = aghconfig.RoleFieldSourceDefault
		}
		provenance[field] = source
	}
	return provenance
}

func roleFields(role aghconfig.RoleName, roles *aghconfig.RolesConfig) map[string]any {
	common, err := roleConfig(roles, role)
	if err != nil {
		return nil
	}
	fields := map[string]any{
		aghconfig.RoleFieldEnabled:   common.Enabled,
		daemonAgentField:             common.Agent,
		aghconfig.RoleFieldProvider:  common.Provider,
		aghconfig.RoleFieldModel:     common.Model,
		aghconfig.RoleFieldReasoning: common.ReasoningEffort,
		aghconfig.RoleFieldFallbacks: common.FallbackChain,
	}
	if role == aghconfig.RoleCoordinator {
		fields["ttl"] = roles.Coordinator.TTL
		fields["max_children"] = roles.Coordinator.MaxChildren
		fields["max_active_sessions_per_workspace"] = roles.Coordinator.MaxActiveSessionsPerWorkspace
	}
	if role == aghconfig.RoleMemoryController {
		fields[roleFieldTimeout] = roles.MemoryController.Timeout
		fields["top_k"] = roles.MemoryController.TopK
		fields["prompt_version"] = roles.MemoryController.PromptVersion
		fields["max_tokens_out"] = roles.MemoryController.MaxTokensOut
	}
	return fields
}

func firstRoleValue(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
