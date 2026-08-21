package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	hookspkg "github.com/compozy/compozy/internal/hooks"
)

// HookDeclarationsForProfiles projects extension hooks through per-profile
// placement and enablement before publishing them to the shared dispatcher.
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
	decls := make([]hookspkg.HookDecl, 0)
	for _, info := range m.List() {
		for _, profile := range profiles {
			projected, enabled, err := m.ProjectForProfile(ctx, GlobalInstanceKey(info.Name), profile)
			if err != nil {
				return nil, fmt.Errorf(
					"extension: project hooks for %q and profile %q: %w",
					info.Name,
					profile.Name,
					err,
				)
			}
			if enabled {
				decls, err = appendProfileHookDeclarations(decls, projected, profile.ID, "")
				if err != nil {
					return nil, fmt.Errorf(
						"extension: bind hooks for %q and profile %q: %w",
						info.Name,
						profile.Name,
						err,
					)
				}
			}
		}
	}

	links, err := m.registry.ListDevLinks()
	if err != nil {
		return nil, fmt.Errorf("extension: list development links for hook projection: %w", err)
	}
	for _, link := range links {
		key := (InstanceKey{Name: link.ExtensionName, WorkspaceID: link.WorkspaceID}).Normalize()
		for _, profile := range profiles {
			projected, enabled, projectErr := m.ProjectForProfile(ctx, key, profile)
			if errors.Is(projectErr, ErrExtensionNotFound) {
				continue
			}
			if projectErr != nil {
				return nil, fmt.Errorf(
					"extension: project development hooks for %q, workspace %q, and profile %q: %w",
					key.Name,
					key.WorkspaceID,
					profile.Name,
					projectErr,
				)
			}
			if enabled {
				decls, projectErr = appendProfileHookDeclarations(
					decls,
					projected,
					profile.ID,
					key.WorkspaceID,
				)
				if projectErr != nil {
					return nil, fmt.Errorf(
						"extension: bind development hooks for %q, workspace %q, and profile %q: %w",
						key.Name,
						key.WorkspaceID,
						profile.Name,
						projectErr,
					)
				}
			}
		}
	}
	return decls, nil
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
) ([]hookspkg.HookDecl, error) {
	if projected == nil {
		return destination, nil
	}
	workspaceID = strings.TrimSpace(workspaceID)
	for _, declaration := range projected.Hooks {
		declaration = cloneHookDecl(declaration)
		declaration.ProfileID = strings.TrimSpace(profileID)
		if workspaceID != "" {
			declaredWorkspaceID := strings.TrimSpace(declaration.Matcher.WorkspaceID)
			if declaredWorkspaceID != "" && declaredWorkspaceID != workspaceID {
				return nil, fmt.Errorf(
					"hook %q declares workspace %q but its development link belongs to %q",
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
