package session

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/workspaceaccess"

	hookspkg "github.com/compozy/compozy/internal/hooks"

	"github.com/compozy/compozy/internal/store"
)

// ValidatePermissionSubset fails closed unless every child permission atom is
// present in the corresponding known parent category.
func ValidatePermissionSubset(parent store.SessionPermissionPolicy, child store.SessionPermissionPolicy) error {
	normalizedParent := store.NormalizeSessionPermissionPolicy(parent)
	normalizedChild := store.NormalizeSessionPermissionPolicy(child)
	for _, category := range knownPermissionCategories {
		if err := validatePermissionAtoms(
			category.name,
			category.values(normalizedParent),
			category.values(normalizedChild),
		); err != nil {
			return err
		}
	}
	return nil
}

func validatePermissionAtoms(category string, parent []string, child []string) error {
	allowed := make(map[string]struct{}, len(parent))
	for _, atom := range parent {
		trimmed := strings.TrimSpace(atom)
		if trimmed == "" {
			return fmt.Errorf("%w: parent %s includes a blank permission atom", ErrSpawnPermissionDenied, category)
		}
		allowed[trimmed] = struct{}{}
	}
	for _, atom := range child {
		trimmed := strings.TrimSpace(atom)
		if trimmed == "" {
			return &SpawnPermissionError{Field: category, Rule: SpawnPermissionRuleBlankAtom}
		}
		if _, ok := allowed[trimmed]; !ok {
			return &SpawnPermissionError{
				Field:   category,
				Value:   trimmed,
				Rule:    SpawnPermissionRuleSubset,
				Allowed: append([]string(nil), parent...),
			}
		}
	}
	return nil
}

func hookPermissionSetFromPolicy(policy store.SessionPermissionPolicy) *hookspkg.PermissionSet {
	normalized := store.NormalizeSessionPermissionPolicy(policy)
	return &hookspkg.PermissionSet{
		Tools:          append([]string(nil), normalized.Tools...),
		Skills:         append([]string(nil), normalized.Skills...),
		MCPServers:     append([]string(nil), normalized.MCPServers...),
		WorkspacePaths: append([]string(nil), normalized.WorkspacePaths...),
	}
}

func policyFromHookPermissionSet(src *hookspkg.PermissionSet) store.SessionPermissionPolicy {
	if src == nil {
		return store.NormalizeSessionPermissionPolicy(store.SessionPermissionPolicy{})
	}
	return store.NormalizeSessionPermissionPolicy(store.SessionPermissionPolicy{
		Tools:          append([]string(nil), src.Tools...),
		Skills:         append([]string(nil), src.Skills...),
		MCPServers:     append([]string(nil), src.MCPServers...),
		WorkspacePaths: append([]string(nil), src.WorkspacePaths...),
	})
}

func spawnWorkspaceCreateRefs(parent *Info, opts SpawnOpts) (string, string) {
	if workspaceID := strings.TrimSpace(opts.Workspace); workspaceID != "" {
		return workspaceID, ""
	}
	if workspacePath := strings.TrimSpace(opts.WorkspacePath); workspacePath != "" {
		return "", workspacePath
	}
	if parent == nil {
		return "", ""
	}
	if workspaceID := strings.TrimSpace(parent.WorkspaceID); workspaceID != "" {
		return workspaceID, ""
	}
	return workspaceaccess.UserSelector, ""
}

func normalizeSpawnRole(role string) string {
	trimmed := strings.TrimSpace(role)
	if trimmed == "" {
		return DefaultSpawnRole
	}
	return trimmed
}

func isCoordinatorSpawnRole(role string) bool {
	return strings.EqualFold(strings.TrimSpace(role), string(SessionTypeCoordinator))
}

func isLiveSpawnState(state State) bool {
	switch state {
	case StateStarting, StateActive, StateStopping:
		return true
	default:
		return false
	}
}

func durationSecondsCeil(duration time.Duration) int64 {
	if duration <= 0 {
		return 0
	}
	seconds := int64(duration / time.Second)
	if duration%time.Second != 0 {
		seconds++
	}
	if seconds <= 0 {
		return 1
	}
	return seconds
}

func spawnValidation(message string) error {
	return fmt.Errorf("%w: %s", ErrSpawnValidation, strings.TrimSpace(message))
}

// effectiveSpawnAuthority resolves the pending child's own scope before its row exists.
func (m *Manager) effectiveSpawnAuthority(
	ctx context.Context, parent *Info, opts *SpawnOpts,
) (store.SessionPermissionPolicy, error) {
	workspaceID := parent.WorkspaceID
	if opts.Workspace != "" {
		workspaceID = opts.Workspace
	}
	if workspaceID == workspaceaccess.UserSelector {
		workspaceID = ""
	}
	child := &Info{
		ID: parent.ID + "/pending-spawn", ProfileID: parent.ProfileID, WorkspaceID: workspaceID,
		AgentName: opts.AgentName, Type: SessionTypeSpawned,
		Lineage: &store.SessionLineage{Kind: store.LineageKindSpawn, PermissionPolicy: opts.PermissionPolicy},
	}
	return m.EffectiveMCPAuthority(WithMCPAuthoritySession(ctx, child), child.ID)
}

func (m *Manager) validateEffectiveSpawnSubset(ctx context.Context, parent *Info, opts *SpawnOpts) error {
	ceiling, err := m.EffectiveMCPAuthority(WithMCPAuthoritySession(ctx, parent), parent.ID)
	if err != nil {
		return err
	}
	child, err := m.effectiveSpawnAuthority(ctx, parent, opts)
	if err != nil {
		return err
	}
	return ValidatePermissionSubset(ceiling, child)
}
