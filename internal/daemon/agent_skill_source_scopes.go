package daemon

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/resources"
	skillspkg "github.com/compozy/compozy/internal/skills"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func appendScopedSkillResources(
	desired *agentSkillDeclarations,
	fallback resources.ResourceScope,
	source skillPublicationSource,
	skills []*skillspkg.Skill,
	allowed ...resources.ResourceScopeKind,
) {
	allowedKinds := make(map[resources.ResourceScopeKind]struct{}, len(allowed))
	for _, kind := range allowed {
		allowedKinds[kind] = struct{}{}
	}
	type scopedSkills struct {
		scope  resources.ResourceScope
		skills []*skillspkg.Skill
	}
	groups := make([]scopedSkills, 0, len(allowedKinds)+1)
	indexByScope := make(map[string]int, len(allowedKinds)+1)
	for _, skill := range skills {
		if skill == nil {
			continue
		}
		scope := skill.ResourceScope.Normalize()
		if scope.Kind == "" {
			scope = fallback.Normalize()
		}
		if len(allowedKinds) > 0 {
			if _, ok := allowedKinds[scope.Kind]; !ok {
				continue
			}
		}
		key := string(scope.Kind) + "\x00" + scope.ID
		index, exists := indexByScope[key]
		if !exists {
			groups = append(groups, scopedSkills{scope: scope})
			index = len(groups) - 1
			indexByScope[key] = index
		}
		groups[index].skills = append(groups[index].skills, skill)
	}
	for _, group := range groups {
		appendSkillResources(desired, group.scope, source, group.skills)
	}
}

func appendProfiledSkillResources(
	ctx context.Context,
	desired *agentSkillDeclarations,
	homePaths compozyconfig.HomePaths,
	profiles extensionProfileCatalog,
	workspaceResolver workspacepkg.RuntimeResolver,
	workspaces []workspacepkg.ResolvedWorkspace,
	registry *skillspkg.Registry,
) error {
	if profiles == nil || registry == nil {
		return nil
	}
	listed, err := profiles.List(ctx)
	if err != nil {
		return fmt.Errorf("daemon: list profiles for skill sync: %w", err)
	}
	for _, profile := range listed {
		if profile.State != profilepkg.StateActive {
			continue
		}
		if err := appendPersonalProfileSkills(ctx, desired, homePaths, profile, registry); err != nil {
			return err
		}
		if err := appendWorkspaceProfileSkills(
			ctx, desired, profile, workspaceResolver, workspaces, registry,
		); err != nil {
			return err
		}
	}
	return nil
}

func appendPersonalProfileSkills(
	ctx context.Context,
	desired *agentSkillDeclarations,
	homePaths compozyconfig.HomePaths,
	profile profilepkg.WithCounts,
	registry *skillspkg.Registry,
) error {
	cfg, err := compozyconfig.LoadForHome(homePaths, compozyconfig.WithProfile(profile.Name))
	if err != nil {
		return fmt.Errorf("daemon: load profile %q config for skill sync: %w", profile.Name, err)
	}
	resolved := &workspacepkg.ResolvedWorkspace{
		ProfileID: profile.ID, ProfileName: profile.Name,
		ProfileRoot: filepath.Join(homePaths.ProfilesDir, profile.Name), Config: cfg,
	}
	skills, _, err := registry.DiscoverWorkspace(ctx, resolved)
	if err != nil {
		return fmt.Errorf("daemon: discover profile %q skills: %w", profile.Name, err)
	}
	appendScopedSkillResources(
		desired,
		resources.ResourceScope{Kind: resources.ResourceScopeKindProfile, ID: strings.TrimSpace(profile.ID)},
		skillPublicationSource{prefix: "skills/profile/" + strings.TrimSpace(profile.ID)},
		skills,
		resources.ResourceScopeKindProfile,
	)
	return nil
}

func appendWorkspaceProfileSkills(
	ctx context.Context,
	desired *agentSkillDeclarations,
	profile profilepkg.WithCounts,
	resolver workspacepkg.RuntimeResolver,
	workspaces []workspacepkg.ResolvedWorkspace,
	registry *skillspkg.Registry,
) error {
	profileResolver, ok := resolver.(workspacepkg.ProfileRuntimeResolver)
	if !ok {
		return nil
	}
	for index := range workspaces {
		workspaceID := strings.TrimSpace(workspaces[index].WorkspaceID)
		if workspaceID == "" {
			workspaceID = strings.TrimSpace(workspaces[index].ID)
		}
		resolved, err := profileResolver.ResolveForProfile(ctx, workspaceID, profile.Name)
		if err != nil {
			return fmt.Errorf(
				"daemon: resolve workspace %q for profile %q skill sync: %w",
				workspaceID,
				profile.Name,
				err,
			)
		}
		skills, _, err := registry.DiscoverWorkspace(ctx, &resolved)
		if err != nil {
			return fmt.Errorf(
				"daemon: discover workspace %q profile %q skills: %w",
				workspaceID,
				profile.Name,
				err,
			)
		}
		appendScopedSkillResources(
			desired,
			resources.ResourceScope{
				Kind: resources.ResourceScopeKindWorkspaceProfile,
				ID:   workspaceID + "@pf:" + strings.TrimSpace(profile.Name),
			},
			skillPublicationSource{
				prefix: "skills/workspace/" + workspaceID + "/profile/" + strings.TrimSpace(profile.ID),
			},
			skills,
			resources.ResourceScopeKindWorkspaceProfile,
		)
	}
	return nil
}
