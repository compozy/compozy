package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	registrypkg "github.com/compozy/agh/internal/registry"
	registryclawhub "github.com/compozy/agh/internal/registry/clawhub"
	"github.com/compozy/agh/internal/skills"
	skillmarketplace "github.com/compozy/agh/internal/skills/marketplace"
	skillbundled "github.com/compozy/agh/skills"
)

const (
	skillUpdateStatusCurrent   = "already up to date"
	skillUpdateStatusAvailable = "update available"
	skillUpdateStatusUpdated   = "updated"
)

type skillRegistrySourceLoader func(*runtimeContext) ([]registrypkg.Source, error)

type skillRegistry interface {
	skillmarketplace.Registry
}

type installedMarketplaceSkill = skillmarketplace.InstalledSkill

func defaultSkillRegistrySourceLoader(
	runtime *runtimeContext,
) ([]registrypkg.Source, error) {
	registryCfg := runtime.Config.Skills.Marketplace
	registryName := strings.ToLower(strings.TrimSpace(registryCfg.Registry))
	if registryName == "" {
		registryName = defaultMarketplaceRegistry
	}

	switch registryName {
	case defaultMarketplaceRegistry:
		return []registrypkg.Source{
			registryclawhub.NewClient(registryCfg.BaseURL),
		}, nil
	default:
		return nil, fmt.Errorf("cli: unsupported marketplace registry %q", registryCfg.Registry)
	}
}

func loadSkillRegistry(deps commandDeps) (*runtimeContext, *registrypkg.MultiRegistry, error) {
	runtime, err := loadRuntimeContext(deps)
	if err != nil {
		return nil, nil, err
	}

	loader := deps.loadSkillRegistrySources
	if loader == nil {
		loader = defaultSkillRegistrySourceLoader
	}

	sources, err := loader(runtime)
	if err != nil {
		return nil, nil, err
	}
	if len(sources) == 0 {
		return nil, nil, errors.New("cli: no skill registry sources are configured")
	}

	return runtime, registrypkg.NewMultiRegistry(slog.Default(), sources...), nil
}

func normalizeSkillSlug(slug string) (string, error) {
	return skillmarketplace.NormalizeSkillSlug(slug)
}

func installMarketplaceSkillForCommand(
	ctx context.Context,
	deps commandDeps,
	slug string,
	version string,
) (_ skillInstallItem, err error) {
	client, running, err := daemonClientIfRunning(ctx, deps)
	if err != nil {
		return skillInstallItem{}, err
	}
	if running {
		item, err := client.InstallSkillMarketplace(ctx, SkillMarketplaceInstallRequest{
			Slug:    slug,
			Version: strings.TrimSpace(version),
		})
		if err != nil {
			return skillInstallItem{}, err
		}
		return skillInstallItemFromRecord(item), nil
	}

	runtime, registry, err := loadSkillRegistry(deps)
	if err != nil {
		return skillInstallItem{}, err
	}
	defer func() {
		err = errors.Join(err, registry.Close())
	}()

	return installMarketplaceSkill(ctx, runtime, registry, slug, version, "", deps.now)
}

func installMarketplaceSkill(
	ctx context.Context,
	runtime *runtimeContext,
	registry skillRegistry,
	slug string,
	version string,
	targetDirOverride string,
	now func() time.Time,
) (item skillInstallItem, err error) {
	result, err := skillmarketplace.InstallWithRegistry(
		ctx,
		runtime.HomePaths.SkillsDir,
		registry,
		slug,
		version,
		targetDirOverride,
		now,
	)
	if err != nil {
		return skillInstallItem{}, err
	}
	if err := verifyInstalledMarketplaceSkill(ctx, runtime, result); err != nil {
		return skillInstallItem{}, err
	}
	return skillInstallItem{
		Name:     result.Name,
		Slug:     result.Slug,
		Version:  result.Version,
		Registry: result.Registry,
		Path:     result.Path,
		Hash:     result.Hash,
		Status:   result.Status,
	}, nil
}

func verifyInstalledMarketplaceSkill(
	ctx context.Context,
	runtime *runtimeContext,
	result skillmarketplace.InstallResult,
) error {
	if runtime == nil {
		return errors.New("cli: skill runtime context is required")
	}

	registry := skills.NewRegistry(skills.RegistryConfig{
		BundledFS:      skillbundled.FS(),
		UserAgentsDir:  runtime.HomePaths.AgentsDir,
		UserSkillsDir:  runtime.HomePaths.SkillsDir,
		DisabledSkills: append([]string(nil), runtime.Config.Skills.DisabledSkills...),
	}, skills.WithLogger(slog.Default()))
	if err := registry.LoadAll(ctx); err != nil {
		return fmt.Errorf("cli: reload skill registry after marketplace install %q: %w", result.Name, err)
	}

	if err := skillmarketplace.VerifyInstallVisible(registry, result); err != nil {
		logSkillMarketplaceInstallVerificationFailure(result, err)
		return fmt.Errorf("cli: %w", err)
	}
	return nil
}

func logSkillMarketplaceInstallVerificationFailure(result skillmarketplace.InstallResult, err error) {
	slog.Warn(
		"skills marketplace: installed skill is not discoverable",
		"name", result.Name,
		"source", result.Registry,
		"slug", result.Slug,
		"path", result.Path,
		"reason", err.Error(),
	)
}

func updateMarketplaceSkills(
	ctx context.Context,
	runtime *runtimeContext,
	registry skillRegistry,
	args []string,
	updateAll bool,
	checkOnly bool,
	now func() time.Time,
) ([]skillUpdateItem, error) {
	name := ""
	if len(args) > 0 {
		name = args[0]
	}
	results, err := skillmarketplace.UpdateWithRegistry(
		ctx,
		runtime.HomePaths.SkillsDir,
		registry,
		skillmarketplace.UpdateRequest{
			Name:      name,
			All:       updateAll,
			CheckOnly: checkOnly,
		},
		now,
	)
	if err != nil {
		return nil, err
	}
	items := make([]skillUpdateItem, 0, len(results))
	for _, result := range results {
		items = append(items, skillUpdateItem{
			Name:           result.Name,
			Slug:           result.Slug,
			CurrentVersion: result.CurrentVersion,
			LatestVersion:  result.LatestVersion,
			Path:           result.Path,
			Status:         result.Status,
		})
	}
	return items, nil
}

func updateMarketplaceSkillsForCommand(
	ctx context.Context,
	deps commandDeps,
	args []string,
	updateAll bool,
	checkOnly bool,
) (_ []skillUpdateItem, err error) {
	client, running, err := daemonClientIfRunning(ctx, deps)
	if err != nil {
		return nil, err
	}
	if running {
		name := ""
		if len(args) > 0 {
			name = strings.TrimSpace(args[0])
		}
		items, err := client.UpdateSkillMarketplace(ctx, SkillMarketplaceUpdateRequest{
			Name:      name,
			All:       updateAll,
			CheckOnly: checkOnly,
		})
		if err != nil {
			return nil, err
		}
		return skillUpdateItemsFromRecords(items), nil
	}

	runtime, registry, err := loadSkillRegistry(deps)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, registry.Close())
	}()

	return updateMarketplaceSkills(
		ctx,
		runtime,
		registry,
		args,
		updateAll,
		checkOnly,
		deps.now,
	)
}

func updateMarketplaceSkill(
	ctx context.Context,
	runtime *runtimeContext,
	registry skillRegistry,
	installed installedMarketplaceSkill,
	checkOnly bool,
	now func() time.Time,
) (skillUpdateItem, error) {
	result, err := skillmarketplace.UpdateSkill(
		ctx,
		runtime.HomePaths.SkillsDir,
		registry,
		installed,
		checkOnly,
		now,
	)
	if err != nil {
		return skillUpdateItem{}, err
	}
	return skillUpdateItem{
		Name:           result.Name,
		Slug:           result.Slug,
		CurrentVersion: result.CurrentVersion,
		LatestVersion:  result.LatestVersion,
		Path:           result.Path,
		Status:         result.Status,
	}, nil
}

func searchMarketplaceSkills(
	ctx context.Context,
	deps commandDeps,
	query string,
	limit int,
) (_ []registrypkg.Listing, err error) {
	if limit <= 0 {
		return nil, fmt.Errorf("cli: search limit must be positive: %d", limit)
	}

	client, running, err := daemonClientIfRunning(ctx, deps)
	if err != nil {
		return nil, err
	}
	if running {
		response, err := client.BrowseMarketplace(
			ctx,
			"skill",
			query,
			limit,
			"",
			MarketplaceReadScope{Scope: contract.SettingsWorkspaceScopeGlobal},
		)
		if err != nil {
			return nil, err
		}
		return skillMarketplaceListingsFromRecords(response.Items)
	}

	_, registry, err := loadSkillRegistry(deps)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, registry.Close())
	}()

	listings, err := registry.Search(ctx, query, registrypkg.SearchOpts{
		Limit: limit,
		Type:  registrypkg.PackageTypeSkill,
	})
	if err != nil {
		return nil, err
	}

	return listings, nil
}

func versionIsNewer(current string, latest string) bool {
	return registrypkg.VersionIsNewer(current, latest)
}

func removeMarketplaceSkillForCommand(ctx context.Context, deps commandDeps, name string) (skillRemoveItem, error) {
	client, running, err := daemonClientIfRunning(ctx, deps)
	if err != nil {
		return skillRemoveItem{}, err
	}
	if running {
		item, err := client.RemoveSkillMarketplace(ctx, name)
		if err != nil {
			return skillRemoveItem{}, err
		}
		return skillRemoveItemFromRecord(item), nil
	}

	runtime, err := loadRuntimeContext(deps)
	if err != nil {
		return skillRemoveItem{}, err
	}
	return removeMarketplaceSkill(runtime.HomePaths.SkillsDir, name)
}

func removeMarketplaceSkill(skillsDir string, name string) (skillRemoveItem, error) {
	result, err := skillmarketplace.RemoveSkill(skillsDir, name)
	if err != nil {
		return skillRemoveItem{}, err
	}
	return skillRemoveItem{
		Name:   result.Name,
		Slug:   result.Slug,
		Path:   result.Path,
		Status: result.Status,
	}, nil
}

func findInstalledMarketplaceSkill(
	skillsDir string,
	name string,
) (installedMarketplaceSkill, error) {
	return skillmarketplace.FindInstalledSkill(skillsDir, name)
}

func listInstalledMarketplaceSkills(skillsDir string) ([]installedMarketplaceSkill, error) {
	return skillmarketplace.ListInstalledSkills(skillsDir)
}

func skillInstallItemFromRecord(item SkillMarketplaceInstallRecord) skillInstallItem {
	return skillInstallItem{
		Name:     item.Name,
		Slug:     item.Slug,
		Version:  item.Version,
		Registry: item.Registry,
		Path:     item.Path,
		Hash:     item.Hash,
		Status:   item.Status,
	}
}

func skillUpdateItemsFromRecords(items []SkillMarketplaceUpdateRecord) []skillUpdateItem {
	updates := make([]skillUpdateItem, 0, len(items))
	for _, item := range items {
		updates = append(updates, skillUpdateItem{
			Name:           item.Name,
			Slug:           item.Slug,
			CurrentVersion: item.CurrentVersion,
			LatestVersion:  item.LatestVersion,
			Path:           item.Path,
			Status:         item.Status,
		})
	}
	return updates
}

func skillRemoveItemFromRecord(item SkillMarketplaceRemoveRecord) skillRemoveItem {
	return skillRemoveItem{
		Name:   item.Name,
		Slug:   item.Slug,
		Path:   item.Path,
		Status: item.Status,
	}
}

func extractMarketplaceArchive(reader io.Reader, destRoot string) error {
	return registrypkg.ExtractArchive(reader, destRoot)
}

func locateExtractedSkillFile(root string) (string, error) {
	var matches []string

	err := filepath.WalkDir(root, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Name() != skillMarkdownFileName {
			return nil
		}
		matches = append(matches, current)
		if len(matches) > 1 {
			return errors.New("multiple SKILL.md files found in archive")
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", errors.New("archive did not contain SKILL.md")
	}
	return matches[0], nil
}

func moveInstalledSkillDir(extractedDir string, targetDir string, replaceExisting bool) error {
	return registrypkg.MoveInstalledDir(extractedDir, targetDir, replaceExisting)
}

func cleanArchiveEntryPath(entry string) (string, error) {
	return registrypkg.CleanArchiveEntryPath(entry)
}

func pathWithinRoot(root string, child string) (string, error) {
	return registrypkg.PathWithinRoot(root, child)
}

func criticalWarnings(warnings []skills.Warning) []string {
	items := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		if warning.Severity != skills.SeverityCritical {
			continue
		}
		items = append(items, firstNonEmpty(warning.Message, warning.Pattern))
	}
	return items
}
