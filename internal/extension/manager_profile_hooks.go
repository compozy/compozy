package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	hookspkg "github.com/compozy/compozy/internal/hooks"
)

// HookDeclarationsForProfiles resolves placement, enablement and workspace overrides before publication.
func (m *Manager) HookDeclarationsForProfiles(
	ctx context.Context,
	profiles []ProfileLens,
) ([]hookspkg.HookDecl, error) {
	if ctx == nil {
		return nil, ErrContextRequired
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if m == nil || m.registry == nil {
		return nil, ErrManagerRequired
	}

	profiles = normalizeHookProfileLenses(profiles)
	workspaces, err := m.hookProjectionWorkspaces(ctx)
	if err != nil {
		return nil, err
	}
	decls := make([]hookspkg.HookDecl, 0)
	for _, name := range slices.Sorted(maps.Keys(workspaces)) {
		for _, profile := range profiles {
			shadowed := make([]string, 0, len(workspaces[name]))
			for _, workspaceID := range workspaces[name] {
				projected, enabled, err := m.ProjectForProfile(ctx, InstanceKey{Name: name, WorkspaceID: workspaceID}, profile)
				if errors.Is(err, ErrExtensionNotFound) {
					continue
				}
				if err != nil {
					return nil, fmt.Errorf("extension: project hooks for %q in workspace %q and profile %q: %w", name, workspaceID, profile.Name, err)
				}
				if projected.Status.WorkspaceID != workspaceID {
					continue
				}
				shadowed = append(shadowed, workspaceID)
				if enabled {
					decls, err = appendProfileHookDeclarations(decls, projected, profile.ID, workspaceID, nil)
					if err != nil {
						return nil, err
					}
				}
			}
			projected, enabled, err := m.ProjectForProfile(ctx, GlobalInstanceKey(name), profile)
			if errors.Is(err, ErrExtensionNotFound) {
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("extension: project hooks for %q and profile %q: %w", name, profile.Name, err)
			}
			if enabled {
				decls, err = appendProfileHookDeclarations(decls, projected, profile.ID, "", shadowed)
				if err != nil {
					return nil, err
				}
			}
		}
	}
	return decls, nil
}

func (m *Manager) hookProjectionWorkspaces(ctx context.Context) (map[string][]string, error) {
	workspaces := make(map[string][]string)
	for _, info := range m.List() {
		workspaces[info.Name] = nil
		installations, err := m.registry.activeInstallations(ctx, info.Name)
		if err != nil {
			return nil, err
		}
		for _, installation := range installations {
			if id := installation.Scope.WorkspaceID; id != "" {
				workspaces[info.Name] = append(workspaces[info.Name], id)
			}
		}
	}
	links, err := m.registry.ListDevLinks()
	if err != nil {
		return nil, err
	}
	for _, link := range links {
		workspaces[link.ExtensionName] = append(workspaces[link.ExtensionName], link.WorkspaceID)
	}
	for name, ids := range workspaces {
		slices.Sort(ids)
		workspaces[name] = slices.Compact(ids)
	}
	return workspaces, nil
}

func normalizeHookProfileLenses(profiles []ProfileLens) []ProfileLens {
	normalized := make([]ProfileLens, 0, len(profiles))
	for _, profile := range profiles {
		profile = profile.normalize()
		if profile.valid() {
			normalized = append(normalized, profile)
		}
	}
	slices.SortFunc(normalized, func(left, right ProfileLens) int {
		if compared := strings.Compare(left.Name, right.Name); compared != 0 {
			return compared
		}
		return strings.Compare(left.ID, right.ID)
	})
	return normalized
}

func appendProfileHookDeclarations(
	destination []hookspkg.HookDecl,
	projected *Extension,
	profileID string,
	workspaceID string,
	shadowedWorkspaces []string,
) ([]hookspkg.HookDecl, error) {
	if projected == nil {
		return destination, nil
	}
	workspaceID = strings.TrimSpace(workspaceID)
	for _, declaration := range projected.Hooks {
		declaration = cloneHookDecl(declaration)
		declaration.ProfileID = strings.TrimSpace(profileID)
		declaration.ShadowedWorkspaces = slices.Clone(shadowedWorkspaces)
		if workspaceID != "" {
			declaredWorkspaceID := strings.TrimSpace(declaration.Matcher.WorkspaceID)
			if declaredWorkspaceID != "" && declaredWorkspaceID != workspaceID {
				return nil, fmt.Errorf(
					"hook %q declares workspace %q but its installation belongs to %q",
					declaration.Name,
					declaredWorkspaceID,
					workspaceID,
				)
			}
			declaration.Matcher.WorkspaceID = workspaceID
		}
		destination = append(destination, declaration)
	}
	return destination, nil
}
