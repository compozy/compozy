package marketplace

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// UpdateWithRegistry checks or applies marketplace updates using the supplied registry.
func UpdateWithRegistry(
	ctx context.Context,
	skillsDir string,
	registry Registry,
	req UpdateRequest,
	now func() time.Time,
) ([]UpdateResult, error) {
	if strings.TrimSpace(req.Name) != "" && req.All {
		return nil, classifiedf(ErrValidation, "cannot combine skill name with --all")
	}
	if !req.All && strings.TrimSpace(req.Name) == "" {
		return nil, classifiedf(ErrValidation, "skill name is required unless --all is set")
	}

	if req.All {
		installedSkills, err := ListInstalledSkills(skillsDir)
		if err != nil {
			return nil, err
		}

		items := make([]UpdateResult, 0, len(installedSkills))
		failures := make([]error, 0)
		for _, installed := range installedSkills {
			item, err := UpdateSkill(ctx, skillsDir, registry, installed, req.CheckOnly, now)
			if err != nil {
				failures = append(failures, fmt.Errorf("update marketplace skill %q: %w", installed.Name, err))
				continue
			}
			items = append(items, item)
		}
		return items, errors.Join(failures...)
	}

	name, err := NormalizeSkillName(req.Name)
	if err != nil {
		return nil, err
	}

	installed, err := FindInstalledSkill(skillsDir, name)
	if err != nil {
		return nil, err
	}

	item, err := UpdateSkill(ctx, skillsDir, registry, installed, req.CheckOnly, now)
	if err != nil {
		return nil, err
	}
	return []UpdateResult{item}, nil
}

// UpdateSkill checks or applies an update for one installed marketplace skill.
func UpdateSkill(
	ctx context.Context,
	skillsDir string,
	registry Registry,
	installed InstalledSkill,
	checkOnly bool,
	now func() time.Time,
) (UpdateResult, error) {
	slug := strings.TrimSpace(installed.Provenance.Slug)
	if slug == "" {
		return UpdateResult{}, classifiedf(
			ErrValidation,
			"marketplace skill %q is missing registry slug metadata",
			installed.Name,
		)
	}
	resolvedRegistry, err := ResolveInstalledSkillRegistry(registry, installed)
	if err != nil {
		return UpdateResult{}, err
	}

	currentVersion := strings.TrimSpace(installed.Provenance.Version)
	updateInfo, err := resolvedRegistry.CheckUpdate(ctx, slug, currentVersion)
	if err != nil {
		return UpdateResult{}, classifyRegistryLookup(err)
	}
	if updateInfo == nil {
		return UpdateResult{}, classifiedf(
			ErrNotFound,
			"marketplace update check returned no result for %q",
			slug,
		)
	}

	latestVersion := strings.TrimSpace(updateInfo.LatestVersion)
	item := UpdateResult{
		Name:           installed.Name,
		Slug:           slug,
		CurrentVersion: currentVersion,
		LatestVersion:  firstNonEmpty(latestVersion, currentVersion),
		Path:           installed.Dir,
	}

	if !updateInfo.HasUpdate {
		item.Status = UpdateStatusCurrent
		return item, nil
	}
	if checkOnly {
		item.Status = UpdateStatusAvailable
		return item, nil
	}

	installedItem, err := InstallWithRegistry(
		ctx,
		skillsDir,
		resolvedRegistry,
		slug,
		latestVersion,
		installed.Dir,
		now,
	)
	if err != nil {
		return UpdateResult{}, err
	}

	item.Name = installedItem.Name
	item.LatestVersion = firstNonEmpty(installedItem.Version, latestVersion)
	item.Path = installedItem.Path
	item.Status = UpdateStatusUpdated
	return item, nil
}
