package settings

import (
	"context"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	skillspkg "github.com/compozy/compozy/internal/skills"
	"github.com/compozy/compozy/internal/store"
)

func (s *service) withSkillSourceEventCorrelation(
	ctx context.Context,
	result MutationResult,
) (context.Context, error) {
	if result.Section != SectionSkills {
		return ctx, nil
	}
	profileID := store.DefaultProfileID
	profileName := strings.TrimSpace(result.ProfileName)
	if profileName != "" && s.profileResolver != nil {
		resolvedID, err := s.profileResolver.AvailableProfileID(ctx, profileName)
		if err != nil {
			return ctx, err
		}
		profileID = strings.TrimSpace(resolvedID)
	}
	actorID := mutationSourceFromContext(ctx)
	rootCounts, err := s.skillSourceEventRootCounts(ctx, result)
	if err != nil {
		return ctx, err
	}
	return skillspkg.WithSourceEventCorrelation(ctx, skillspkg.SourceEventCorrelation{
		Scope:       strings.TrimSpace(string(result.Scope)),
		ProfileID:   profileID,
		WorkspaceID: strings.TrimSpace(result.WorkspaceID),
		ActorKind:   "settings",
		ActorID:     actorID,
		RootCounts:  rootCounts,
	}), nil
}

func (s *service) skillSourceEventRootCounts(
	ctx context.Context,
	result MutationResult,
) (map[string]int, error) {
	cfg, resolved, err := s.loadConfig(ctx, result.Scope, result.WorkspaceID, result.ProfileName)
	if err != nil {
		return nil, fmt.Errorf("settings: load source event projection: %w", err)
	}
	if result.Scope == ScopeUser || result.Scope == "" {
		return countSkillSourceRoots(compozyconfig.ResolveGlobalSkillRoots(&cfg.Skills, s.homePaths)), nil
	}
	sourcesRuntime, ok := s.skillsRuntime.(SkillsSourcesRuntime)
	if !ok {
		return nil, nil
	}
	projection, err := s.skillProjectionWorkspace(ctx, &cfg, resolved, result.Scope, result.ProfileName)
	if err != nil {
		return nil, err
	}
	statuses, err := sourcesRuntime.SkillSourceRoots(ctx, projection)
	if err != nil {
		return nil, fmt.Errorf("settings: measure source event roots: %w", err)
	}
	counts := make(map[string]int)
	for _, status := range statuses {
		slug := strings.TrimSpace(status.Spec.SourceSlug)
		if slug == "" {
			slug = compozyconfig.SkillSourceCompozy
		}
		counts[slug]++
	}
	return counts, nil
}

func countSkillSourceRoots(roots []compozyconfig.SkillRootSpec) map[string]int {
	counts := make(map[string]int)
	for _, root := range roots {
		slug := strings.TrimSpace(root.SourceSlug)
		if slug == "" {
			slug = compozyconfig.SkillSourceCompozy
		}
		counts[slug]++
	}
	return counts
}
