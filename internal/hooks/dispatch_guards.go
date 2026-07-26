package hooks

import (
	"context"

	"fmt"
	"strings"
)

func guardTaskRunPreClaimPatch(
	_ context.Context,
	hook RegisteredHook,
	payload TaskRunPreClaimPayload,
	patch TaskRunPreClaimPatch,
) error {
	if patch.PriorityMin != nil && *patch.PriorityMin < payload.Criteria.PriorityMin {
		return fmt.Errorf(
			"%w: hook %q cannot lower task-run claim priority_min from %d to %d",
			ErrHookPatchRejected,
			hook.Name,
			payload.Criteria.PriorityMin,
			*patch.PriorityMin,
		)
	}
	for _, capability := range patch.AddRequiredCapabilities {
		if strings.TrimSpace(capability) == "" {
			return fmt.Errorf(
				"%w: hook %q cannot add blank task-run claim capability",
				ErrHookPatchRejected,
				hook.Name,
			)
		}
	}
	return nil
}

func guardSpawnCreatePatch(
	_ context.Context,
	hook RegisteredHook,
	payload SpawnPreCreatePayload,
	patch SpawnCreatePatch,
) error {
	if patch.TTLSeconds != nil && *patch.TTLSeconds <= 0 {
		return fmt.Errorf(
			"%w: hook %q cannot set non-positive spawn ttl_seconds",
			ErrHookPatchRejected,
			hook.Name,
		)
	}

	child := payload.ChildPermissions
	if patch.ChildPermissions != nil {
		child = patch.ChildPermissions
	}
	if err := validatePermissionSubset(payload.ParentPermissions, child); err != nil {
		return fmt.Errorf("%w: hook %q spawn permission patch rejected: %w", ErrHookPatchRejected, hook.Name, err)
	}
	return nil
}

func validatePermissionSubset(parent *PermissionSet, child *PermissionSet) error {
	if err := validatePermissionAtoms("tools", permissionTools(parent), permissionTools(child)); err != nil {
		return err
	}
	if err := validatePermissionAtoms("skills", permissionSkills(parent), permissionSkills(child)); err != nil {
		return err
	}
	if err := validatePermissionAtoms(
		"mcp_servers",
		permissionMCPServers(parent),
		permissionMCPServers(child),
	); err != nil {
		return err
	}
	if err := validatePermissionAtoms(
		"workspace_paths",
		permissionWorkspacePaths(parent),
		permissionWorkspacePaths(child),
	); err != nil {
		return err
	}
	if err := validatePermissionAtoms(
		"network_channels",
		permissionNetworkChannels(parent),
		permissionNetworkChannels(child),
	); err != nil {
		return err
	}
	return validatePermissionAtoms(
		"sandbox_profiles",
		permissionSandboxProfiles(parent),
		permissionSandboxProfiles(child),
	)
}

func permissionTools(src *PermissionSet) []string {
	if src == nil {
		return nil
	}
	return src.Tools
}

func permissionSkills(src *PermissionSet) []string {
	if src == nil {
		return nil
	}
	return src.Skills
}

func permissionMCPServers(src *PermissionSet) []string {
	if src == nil {
		return nil
	}
	return src.MCPServers
}

func permissionWorkspacePaths(src *PermissionSet) []string {
	if src == nil {
		return nil
	}
	return src.WorkspacePaths
}

func permissionNetworkChannels(src *PermissionSet) []string {
	if src == nil {
		return nil
	}
	return src.NetworkChannels
}

func permissionSandboxProfiles(src *PermissionSet) []string {
	if src == nil {
		return nil
	}
	return src.SandboxProfiles
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

func normalizePermissionSet(src *PermissionSet) *PermissionSet {
	if src == nil {
		return nil
	}
	return &PermissionSet{
		Tools:           uniqueTrimmedStrings(src.Tools),
		Skills:          uniqueTrimmedStrings(src.Skills),
		MCPServers:      uniqueTrimmedStrings(src.MCPServers),
		WorkspacePaths:  uniqueTrimmedStrings(src.WorkspacePaths),
		NetworkChannels: uniqueTrimmedStrings(src.NetworkChannels),
		SandboxProfiles: uniqueTrimmedStrings(src.SandboxProfiles),
	}
}

func unionStringSet(base []string, additions []string) []string {
	return uniqueTrimmedStrings(append(append([]string(nil), base...), additions...))
}

func uniqueTrimmedStrings(values []string) []string {
	if values == nil {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}
