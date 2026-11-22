package daemon

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
)

// guardDependencyInUse refuses removing or disabling name while an installed dependent still resolves its
// dependency to it (rule 14). profileID selects a disable of one profile; target selects the removed installation.
// force proceeds and only logs the dependents left unmet; nothing cascades.
func (s *daemonExtensionService) guardDependencyInUse(
	ctx context.Context, name, profileID string, target *extensionpkg.InstallationScope, force bool,
) error {
	dependents, err := s.dependentsOf(ctx, name, profileID, target)
	if err != nil || len(dependents) == 0 {
		return err
	}
	if !force {
		return &extensionpkg.DependencyInUseError{Name: name, Dependents: dependents}
	}
	for _, dependent := range dependents {
		s.logger.Warn("extension dependency unmet", "dependent", dependent.Name, "dependency", name,
			"workspace_id", dependent.WorkspaceID, "profile", dependent.Profile, "cause", "forced")
	}
	return nil
}

// guardProfileDisable applies the guard to a profile disable that changes state; a no-op disable passes.
func (s *daemonExtensionService) guardProfileDisable(
	ctx context.Context, name, profileID string, req contract.SetExtensionEnablementRequest,
) error {
	if req.Enabled {
		return nil
	}
	enabled, err := s.registry.IsEnabledForProfile(name, profileID)
	if err != nil || !enabled {
		return err
	}
	return s.guardDependencyInUse(ctx, name, profileID, nil, req.Force)
}

// dependentsOf lists installed extensions declaring name whose instance cell resolves name to the target (D11).
func (s *daemonExtensionService) dependentsOf(
	ctx context.Context, name, profileID string, target *extensionpkg.InstallationScope,
) ([]contract.ExtensionDependentPayload, error) {
	infos, err := s.registry.List()
	if err != nil {
		return nil, err
	}
	var dependents []contract.ExtensionDependentPayload
	for index := range infos {
		info := &infos[index]
		if info.Name == name || !s.installedManifestRequires(info, name) {
			continue
		}
		installations, err := s.registry.Installations(ctx, info.Name)
		if err != nil {
			return nil, err
		}
		for _, installation := range installations {
			cell, ok := dependentCell(installation.Scope, profileID, target)
			if !ok {
				continue
			}
			resolved, err := s.registry.ResolveInstallation(ctx, name, cell)
			if errors.Is(err, extensionpkg.ErrExtensionNotFound) {
				continue
			}
			if err != nil {
				return nil, err
			}
			if target != nil && resolved.Scope != *target {
				continue
			}
			profile, err := s.dependencyProfileName(ctx, cell.ProfileID)
			if err != nil {
				return nil, err
			}
			dependents = append(dependents, contract.ExtensionDependentPayload{
				Name: info.Name, WorkspaceID: cell.WorkspaceID, Profile: profile,
			})
		}
	}
	return dependents, nil
}

// dependentCell is the instance cell a dependent installation resolves its dependencies from. A disable of one
// profile only concerns installations visible to it; an all-profiles installation resolves in the target's profile.
func dependentCell(
	installation extensionpkg.InstallationScope, profileID string, target *extensionpkg.InstallationScope,
) (extensionpkg.InstallationScope, bool) {
	if profileID != "" {
		if installation.ProfileID != "" && installation.ProfileID != profileID {
			return extensionpkg.InstallationScope{}, false
		}
		return extensionpkg.InstallationScope{ProfileID: profileID, WorkspaceID: installation.WorkspaceID}, true
	}
	if installation.ProfileID == "" && target != nil {
		installation.ProfileID = target.ProfileID
	}
	return installation, true
}

// installedManifestRequires reads an installed manifest; one that no longer loads cannot be a dependent.
func (s *daemonExtensionService) installedManifestRequires(info *extensionpkg.ExtensionInfo, name string) bool {
	manifest, err := extensionpkg.LoadInstalledManifest(info)
	if err != nil {
		s.logger.Debug("extension dependency guard skipped unreadable manifest", "extension", info.Name, "error", err)
		return false
	}
	return slices.ContainsFunc(manifest.Requires.Extensions, func(dependency extensionpkg.ExtensionDependency) bool {
		return dependency.Name == name
	})
}

func (s *daemonExtensionService) dependencyProfileName(ctx context.Context, profileID string) (string, error) {
	if profileID == "" || s.profiles == nil {
		return profileID, nil
	}
	return s.profiles.ProfileName(ctx, profileID)
}

// cellUnmetDependencies classifies the direct requires.extensions of one instance cell without acquiring
// packages (D11): missing, version_unmet or disabled for the cell's profile.
func (s *daemonExtensionService) cellUnmetDependencies(
	ctx context.Context,
	dependent string,
	requires []extensionpkg.ExtensionDependency,
	cell extensionpkg.InstallationScope,
) ([]contract.ExtensionDependencyPayload, error) {
	var unmet []contract.ExtensionDependencyPayload
	for _, dependency := range requires {
		item := contract.ExtensionDependencyPayload{
			Name: dependency.Name, Source: dependency.Source, Ref: dependency.Ref, MinVersion: dependency.MinVersion,
			RequiredBy: []string{dependent}, Depth: 1,
		}
		state, installedVersion, err := s.cellDependencyState(ctx, dependency, cell)
		if err != nil {
			return nil, err
		}
		if state == dependencyStateMet {
			continue
		}
		item.State, item.InstalledVersion = state, installedVersion
		if state == dependencyStateVersionUnmet {
			item.RetryCommand = "compozy extension update " + dependency.Name
		}
		unmet = append(unmet, item)
	}
	return unmet, nil
}

func (s *daemonExtensionService) cellDependencyState(
	ctx context.Context, dependency extensionpkg.ExtensionDependency, cell extensionpkg.InstallationScope,
) (state, installedVersion string, err error) {
	if _, err := s.registry.ResolveInstallation(ctx, dependency.Name, cell); err != nil {
		if errors.Is(err, extensionpkg.ErrExtensionNotFound) {
			return dependencyStateMissing, "", nil
		}
		return "", "", err
	}
	info, err := s.registry.Get(dependency.Name)
	if err != nil {
		return "", "", err
	}
	if !extensionpkg.VersionSatisfies(info.Version, dependency.MinVersion) {
		return dependencyStateVersionUnmet, info.Version, nil
	}
	enabled, err := s.registry.IsEnabledForProfile(dependency.Name, cell.ProfileID)
	if err != nil {
		return "", "", err
	}
	if !enabled {
		return dependencyStateDisabled, info.Version, nil
	}
	return dependencyStateMet, "", nil
}

// checkProfileEnablementDependencies refuses enabling a dependent whose dependencies are unmet in any of its
// installation cells for the profile (rule 15), with the install payload. Boot never runs this gate.
func (s *daemonExtensionService) checkProfileEnablementDependencies(ctx context.Context, name, profileID string) error {
	ext, err := s.runtime.InspectPackageResources(ctx, name)
	if err != nil || ext == nil || ext.Manifest == nil || len(ext.Manifest.Requires.Extensions) == 0 {
		return err
	}
	manifest := ext.Manifest
	installations, err := s.registry.Installations(ctx, name)
	if err != nil {
		return err
	}
	var cells []extensionpkg.InstallationScope
	for _, installation := range installations {
		if cell, ok := dependentCell(installation.Scope, profileID, nil); ok && !slices.Contains(cells, cell) {
			cells = append(cells, cell)
		}
	}
	if len(cells) == 0 {
		cells = append(cells, extensionpkg.InstallationScope{ProfileID: profileID})
	}
	var unmet []contract.ExtensionDependencyPayload
	for _, cell := range cells {
		items, err := s.cellUnmetDependencies(ctx, name, manifest.Requires.Extensions, cell)
		if err != nil {
			return err
		}
		for _, item := range items {
			if !slices.ContainsFunc(unmet, func(seen contract.ExtensionDependencyPayload) bool {
				return seen.Name == item.Name
			}) {
				unmet = append(unmet, item)
			}
		}
	}
	if len(unmet) == 0 {
		return nil
	}
	return &extensionpkg.DependenciesRequiredError{Dependencies: unmet}
}

// populateExtensionDependencyStatus reports unmet_dependencies on read; a dependent left unmet keeps running.
func (s *daemonExtensionService) populateExtensionDependencyStatus(
	ctx context.Context, ext *extensionpkg.Extension, profileID string, payload *contract.ExtensionPayload,
) error {
	if ext.Manifest == nil || len(ext.Manifest.Requires.Extensions) == 0 {
		return nil
	}
	unmet, err := s.cellUnmetDependencies(ctx, ext.Info.Name, ext.Manifest.Requires.Extensions,
		extensionpkg.InstallationScope{ProfileID: profileID, WorkspaceID: ext.Status.WorkspaceID})
	if err != nil {
		return err
	}
	payload.UnmetDependencies = unmet
	if len(unmet) > 0 {
		names := make([]string, 0, len(unmet))
		for _, item := range unmet {
			names = append(names, item.Name)
		}
		s.logger.Warn("extension dependency unmet", "dependent", ext.Info.Name,
			"dependency", strings.Join(names, ","), "profile_id", profileID)
	}
	return nil
}
