package core

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	settingspkg "github.com/compozy/compozy/internal/settings"
	"github.com/compozy/compozy/internal/skills"
)

func (h *BaseHandlers) marketplaceScopedSkills(
	ctx context.Context,
	scope marketplaceReadScope,
) ([]*skills.Skill, error) {
	if h == nil || h.SkillsRegistry == nil {
		return nil, errors.Join(
			ErrMarketplaceUnavailable,
			errors.New("skills registry is not configured"),
		)
	}
	switch scope.scope {
	case settingspkg.ScopeProfile:
		profileRegistry, ok := h.SkillsRegistry.(ProfileSkillsRegistry)
		if !ok {
			return nil, errors.Join(
				ErrMarketplaceUnavailable,
				errors.New("profile-scoped skill reads are not configured"),
			)
		}
		return profileRegistry.ForProfile(
			ctx,
			scope.profileName,
			filepath.Join(h.HomePaths.ProfilesDir, scope.profileName),
		)
	case settingspkg.ScopeWorkspace:
		if h.Workspaces == nil {
			return nil, errors.Join(
				ErrMarketplaceUnavailable,
				errors.New("workspace resolver is not configured"),
			)
		}
		resolved, err := h.Workspaces.Resolve(ctx, scope.workspaceID)
		if err != nil {
			return nil, err
		}
		return h.SkillsRegistry.ForWorkspace(ctx, &resolved)
	default:
		return h.SkillsRegistry.List(), nil
	}
}

func marketplaceSkillInstallIndex(items []*skills.Skill) marketplaceInstallIndex {
	index := newMarketplaceInstallIndex()
	for _, item := range items {
		if item == nil || item.Provenance == nil {
			continue
		}
		slug := strings.TrimSpace(item.Provenance.Slug)
		if slug == "" {
			continue
		}
		index.bySlug[slug] = marketplaceInstall{
			name:       strings.TrimSpace(item.Meta.Name),
			version:    strings.TrimSpace(item.Provenance.Version),
			managePath: marketplaceSkillsInstalledPath,
		}
	}
	return index
}
