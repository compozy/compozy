package daemon

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

// RoleStatuses projects the closed background-role roster in lexical order.
func (r *roleResolver) RoleStatuses(ctx context.Context, workspaceID string) ([]contract.RoleStatus, error) {
	effective, resolvedWorkspace, err := r.effectiveRoleConfig(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	roles := aghconfig.RoleNames()
	slices.Sort(roles)
	statuses := make([]contract.RoleStatus, 0, len(roles))
	for _, role := range roles {
		status, statusErr := r.projectRoleStatus(role, effective, resolvedWorkspace)
		if statusErr != nil {
			return nil, statusErr
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

// RoleStatus projects one configured background role without invoking it.
func (r *roleResolver) RoleStatus(
	ctx context.Context,
	workspaceID string,
	role string,
) (contract.RoleStatus, error) {
	roleName := aghconfig.RoleName(strings.TrimSpace(role))
	if !knownRole(roleName) {
		return contract.RoleStatus{}, &RoleResolutionError{Code: roleErrorUnknown, Role: roleName}
	}
	effective, resolvedWorkspace, err := r.effectiveRoleConfig(ctx, workspaceID)
	if err != nil {
		return contract.RoleStatus{}, err
	}
	return r.projectRoleStatus(roleName, effective, resolvedWorkspace)
}

func (r *roleResolver) projectRoleStatus(
	role aghconfig.RoleName,
	effective *aghconfig.Config,
	resolvedWorkspace *workspacepkg.ResolvedWorkspace,
) (contract.RoleStatus, error) {
	common, err := roleConfig(&effective.Roles, role)
	if err != nil {
		return contract.RoleStatus{}, err
	}
	mode, agent, diagnostics, err := r.projectRoleIdentity(role, common.Agent, resolvedWorkspace)
	if err != nil {
		return contract.RoleStatus{}, err
	}
	status := contract.RoleStatus{
		Role:            string(role),
		Enabled:         common.Enabled,
		ResolutionMode:  mode,
		Agent:           agent,
		Provider:        roleStatusString(common.Provider),
		Model:           roleStatusString(common.Model),
		ReasoningEffort: roleStatusString(common.ReasoningEffort),
		FallbackChain:   roleStatusFallbacks(common.FallbackChain),
		Diagnostics:     diagnostics,
	}
	if role == aghconfig.RoleMemoryController {
		timeout := effective.Roles.MemoryController.Timeout.String()
		status.Timeout = &timeout
	}
	status.Provenance = roleStatusProvenance(
		roleProvenance(role, effective),
		status,
	)
	return status, nil
}

func (r *roleResolver) projectRoleIdentity(
	role aghconfig.RoleName,
	configuredAgent string,
	resolvedWorkspace *workspacepkg.ResolvedWorkspace,
) (contract.RoleResolutionMode, *string, []contract.RoleDiagnostic, error) {
	agentName := strings.TrimSpace(configuredAgent)
	if agentName == "" {
		var inherit bool
		agentName, inherit = defaultRoleIdentity(role)
		if inherit || agentName == "" {
			return contract.RoleResolutionModeInherit, nil, []contract.RoleDiagnostic{}, nil
		}
	}
	mode := contract.RoleResolutionModeCatalog
	if _, builtin := aghconfig.BuiltinAgentDef(agentName); builtin {
		mode = contract.RoleResolutionModeBuiltin
	}
	_, _, err := r.resolveRoleAgent(role, agentName, resolvedWorkspace)
	if err == nil {
		return mode, &agentName, []contract.RoleDiagnostic{}, nil
	}
	var resolutionErr *RoleResolutionError
	if errors.As(err, &resolutionErr) && resolutionErr.Code == roleErrorAgentNotFound {
		return mode, &agentName, []contract.RoleDiagnostic{{
			Code:    contract.CodeRoleAgentNotFound,
			Message: fmt.Sprintf("Agent %q is not available", agentName),
			Agent:   agentName,
		}}, nil
	}
	return "", nil, nil, err
}

func knownRole(role aghconfig.RoleName) bool {
	return slices.Contains(aghconfig.RoleNames(), role)
}

func roleStatusString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func roleStatusFallbacks(fallbacks []aghconfig.RoleFallback) []contract.RoleFallbackStatus {
	result := make([]contract.RoleFallbackStatus, 0, len(fallbacks))
	for _, fallback := range fallbacks {
		result = append(result, contract.RoleFallbackStatus{
			Provider:        strings.TrimSpace(fallback.Provider),
			Model:           strings.TrimSpace(fallback.Model),
			ReasoningEffort: strings.TrimSpace(fallback.ReasoningEffort),
		})
	}
	return result
}

func roleStatusProvenance(all map[string]string, status contract.RoleStatus) map[string]string {
	fields := []string{aghconfig.RoleFieldEnabled, aghconfig.RoleFieldFallbacks}
	if status.Agent != nil {
		fields = append(fields, daemonAgentField)
	}
	if status.Provider != nil {
		fields = append(fields, aghconfig.RoleFieldProvider)
	}
	if status.Model != nil {
		fields = append(fields, aghconfig.RoleFieldModel)
	}
	if status.ReasoningEffort != nil {
		fields = append(fields, aghconfig.RoleFieldReasoning)
	}
	if status.Timeout != nil {
		fields = append(fields, roleFieldTimeout)
	}
	provenance := make(map[string]string, len(fields))
	for _, field := range fields {
		provenance[field] = all[field]
	}
	return provenance
}
