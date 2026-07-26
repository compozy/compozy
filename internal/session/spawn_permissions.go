package session

import (
	"fmt"
	"strings"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"

	"github.com/compozy/agh/internal/store"
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
			return fmt.Errorf("%w: %w", ErrSpawnPermissionDenied, err)
		}
	}
	return nil
}

func validatePermissionAtoms(category string, parent []string, child []string) error {
	allowed := make(map[string]struct{}, len(parent))
	for _, atom := range parent {
		trimmed := strings.TrimSpace(atom)
		if trimmed == "" {
			return fmt.Errorf("parent %s includes a blank permission atom", category)
		}
		allowed[trimmed] = struct{}{}
	}
	for _, atom := range child {
		trimmed := strings.TrimSpace(atom)
		if trimmed == "" {
			return fmt.Errorf("child %s includes a blank permission atom", category)
		}
		if _, ok := allowed[trimmed]; !ok {
			return fmt.Errorf("child %s permission atom %q widens parent permissions", category, trimmed)
		}
	}
	return nil
}

func effectiveSpawnBudget(budget store.SessionSpawnBudget) store.SessionSpawnBudget {
	normalized := budget
	if normalized.MaxChildren <= 0 {
		normalized.MaxChildren = DefaultSpawnMaxChildren
	}
	if normalized.MaxDepth <= 0 {
		normalized.MaxDepth = DefaultSpawnMaxDepth
	}
	return normalized
}

func hookPermissionSetFromPolicy(policy store.SessionPermissionPolicy) *hookspkg.PermissionSet {
	normalized := store.NormalizeSessionPermissionPolicy(policy)
	return &hookspkg.PermissionSet{
		Tools:           append([]string(nil), normalized.Tools...),
		Skills:          append([]string(nil), normalized.Skills...),
		MCPServers:      append([]string(nil), normalized.MCPServers...),
		WorkspacePaths:  append([]string(nil), normalized.WorkspacePaths...),
		NetworkChannels: append([]string(nil), normalized.NetworkChannels...),
		SandboxProfiles: append([]string(nil), normalized.SandboxProfiles...),
	}
}

func policyFromHookPermissionSet(src *hookspkg.PermissionSet) store.SessionPermissionPolicy {
	if src == nil {
		return store.NormalizeSessionPermissionPolicy(store.SessionPermissionPolicy{})
	}
	return store.NormalizeSessionPermissionPolicy(store.SessionPermissionPolicy{
		Tools:           append([]string(nil), src.Tools...),
		Skills:          append([]string(nil), src.Skills...),
		MCPServers:      append([]string(nil), src.MCPServers...),
		WorkspacePaths:  append([]string(nil), src.WorkspacePaths...),
		NetworkChannels: append([]string(nil), src.NetworkChannels...),
		SandboxProfiles: append([]string(nil), src.SandboxProfiles...),
	})
}

func spawnWorkspaceCreateRefs(parent *Info) (string, string) {
	if parent == nil {
		return "", ""
	}
	if workspaceID := strings.TrimSpace(parent.WorkspaceID); workspaceID != "" {
		return workspaceID, ""
	}
	return "", strings.TrimSpace(parent.Workspace)
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
