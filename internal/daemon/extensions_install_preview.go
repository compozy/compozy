package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func (s *daemonExtensionService) PreviewInstall(
	ctx context.Context,
	req contract.InstallExtensionRequest,
	actor taskpkg.ActorContext,
) (_ contract.ExtensionInstallPreviewPayload, resultErr error) {
	if err := s.checkReady(); err != nil {
		return contract.ExtensionInstallPreviewPayload{}, err
	}
	if err := validateExtensionWriteActor(actor); err != nil {
		return contract.ExtensionInstallPreviewPayload{}, err
	}
	prepared, err := s.prepareExtensionInstall(ctx, req, actor, extensionInstalledBy(actor))
	if err != nil {
		return contract.ExtensionInstallPreviewPayload{}, err
	}
	defer func() {
		if closeErr := prepared.Close(); closeErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("daemon: close extension install preview: %w", closeErr))
		}
	}()
	if prepared.manifest == nil {
		return contract.ExtensionInstallPreviewPayload{}, errors.New("daemon: prepared extension manifest is required")
	}
	if s.profiles == nil {
		return contract.ExtensionInstallPreviewPayload{}, errors.New(
			"daemon: profile manager is required for extension install preview",
		)
	}
	plan, err := extensionpkg.BuildDeclaredProfilePlan(ctx, s.profiles, prepared.manifest)
	if err != nil {
		return contract.ExtensionInstallPreviewPayload{}, err
	}
	digest, err := extensionpkg.NetworkParticipationRequirementDigest(prepared.manifest.NetworkParticipation)
	if err != nil {
		return contract.ExtensionInstallPreviewPayload{}, err
	}
	result := contract.ExtensionInstallPreviewPayload{
		Name: prepared.name, NetworkRequirementDigest: digest,
		DeclaredProfiles: make([]contract.ExtensionInstallDeclaredProfilePayload, 0, len(plan.Profiles)),
		Placements:       make([]contract.ExtensionPlacementPayload, 0),
	}
	for _, entry := range plan.Profiles {
		item := contract.ExtensionInstallDeclaredProfilePayload{
			Name: entry.Name, Create: entry.Create,
			Credentials: make([]contract.ProfileCredentialRequirement, 0, len(entry.NeedsSetup)),
		}
		for _, credential := range entry.NeedsSetup {
			item.Credentials = append(item.Credentials, contract.ProfileCredentialRequirement{
				Provider: credential.Provider, Slot: credential.Slot,
				SourceExtension: prepared.name, Missing: true,
			})
		}
		result.DeclaredProfiles = append(result.DeclaredProfiles, item)
	}
	for _, placement := range prepared.manifest.PlacementMatrix() {
		result.Placements = append(result.Placements, contract.ExtensionPlacementPayload{
			Kind: placement.Kind, Resource: placement.Resource, Profile: placement.Profile,
		})
	}
	return result, nil
}

func (s *daemonExtensionService) prepareInstallNetworkConfirmation(
	manifest *extensionpkg.Manifest,
	expectedDigest string,
	actor taskpkg.ActorContext,
) (*extensionpkg.NetworkConfirmation, error) {
	digest, err := extensionpkg.NetworkParticipationRequirementDigest(manifest.NetworkParticipation)
	if err != nil {
		return nil, err
	}
	if digest == "" {
		return nil, nil
	}
	if expectedDigest != digest {
		return nil, &extensionpkg.NetworkConfirmationRequiredError{CurrentDigest: digest}
	}
	confirmedBy, err := extensionNetworkConfirmationActor(actor)
	if err != nil {
		return nil, err
	}
	return &extensionpkg.NetworkConfirmation{
		Digest: digest, ConfirmedBy: confirmedBy, ConfirmedAt: s.now().UTC(),
	}, nil
}
