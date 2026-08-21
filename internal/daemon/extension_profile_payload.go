package daemon

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	profilepkg "github.com/compozy/compozy/internal/profile"
)

func (s *daemonExtensionService) enrichExtensionProfilePayload(
	ctx context.Context,
	payload *contract.ExtensionPayload,
	manifest *extensionpkg.Manifest,
) error {
	if payload == nil || manifest == nil {
		return nil
	}
	profiles, err := s.extensionProfiles(ctx)
	if err != nil {
		return err
	}
	byName := make(map[string]profilepkg.WithCounts, len(profiles))
	profileNames := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		name := strings.TrimSpace(profile.Name)
		byName[name] = profile
		profileNames = append(profileNames, name)
	}
	payload.DeclaredProfiles = make([]contract.ExtensionDeclaredProfilePayload, 0, len(manifest.Profiles))
	for _, declaration := range manifest.Profiles {
		name := strings.TrimSpace(declaration.Name)
		profile, exists := byName[name]
		createdByExtension := false
		if s.profiles != nil {
			createdByExtension, err = s.profiles.HasDeclaredMarker(ctx, manifest.Name, name)
			if err != nil {
				return fmt.Errorf("daemon: inspect declared profile marker %q: %w", name, err)
			}
		}
		requirements := extensionCredentialRequirements(profile, manifest.Name)
		payload.DeclaredProfiles = append(payload.DeclaredProfiles, contract.ExtensionDeclaredProfilePayload{
			Name: name, Exists: exists, CreatedByExtension: createdByExtension,
			NeedsSetup: len(requirements) > 0, CredentialRequirements: requirements,
		})
	}
	dormant := manifest.DormantPlacements(profileNames)
	dormantKeys := make(map[string]struct{}, len(dormant))
	for _, placement := range dormant {
		dormantKeys[extensionPlacementKey(placement)] = struct{}{}
	}
	placements := manifest.PlacementMatrix()
	payload.Placements = make([]contract.ExtensionPlacementPayload, 0, len(placements))
	for _, placement := range placements {
		_, isDormant := dormantKeys[extensionPlacementKey(placement)]
		item := contract.ExtensionPlacementPayload{
			Kind: placement.Kind, Resource: placement.Resource,
			Profile: placement.Profile, Dormant: isDormant,
		}
		if isDormant {
			item.CreateAction = "compozy profile create " + placement.Profile
			payload.DormantPlacements = append(payload.DormantPlacements, item)
		}
		payload.Placements = append(payload.Placements, item)
	}
	return nil
}

func (s *daemonExtensionService) extensionProfiles(
	ctx context.Context,
) ([]profilepkg.WithCounts, error) {
	if s.profiles == nil {
		return nil, nil
	}
	profiles, err := s.profiles.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("daemon: list profiles for extension detail: %w", err)
	}
	return profiles, nil
}

func extensionCredentialRequirements(
	profile profilepkg.WithCounts,
	extensionName string,
) []contract.ProfileCredentialRequirement {
	requirements := make([]contract.ProfileCredentialRequirement, 0)
	for _, requirement := range profile.CredentialRequirements {
		if requirement.SourceExtension != extensionName {
			continue
		}
		requirements = append(requirements, contract.ProfileCredentialRequirement{
			Provider: requirement.Provider, Slot: requirement.Slot,
			SourceExtension: requirement.SourceExtension, Missing: requirement.Missing,
		})
	}
	return requirements
}

func extensionPlacementKey(placement extensionpkg.ManifestPlacement) string {
	return placement.Kind + "\x00" + placement.Resource + "\x00" + placement.Profile
}
