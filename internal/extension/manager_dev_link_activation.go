package extensionpkg

import (
	"context"
	"errors"
	"strings"
)

// StageDevelopmentLink verifies and persists one immutable generation without starting it.
func (m *Manager) StageDevelopmentLink(
	ctx context.Context,
	key InstanceKey,
	originPath string,
	generationHash string,
) (*DevLink, error) {
	operation, err := m.beginDevOperation(ctx, key)
	if err != nil {
		return nil, err
	}
	defer operation.close()
	ctx = operation.ctx
	key = key.Normalize()
	coordinator := m.coordinatorFor(key)
	coordinator.Lock()
	defer coordinator.Unlock()

	verified, err := m.resolveDevGeneration(ctx, key, originPath, generationHash)
	if err != nil {
		return nil, err
	}
	if err := operation.ensureActive(); err != nil {
		return nil, err
	}
	return m.stageDevelopmentLinkLocked(key, verified)
}

// ActivateDevelopmentLink starts only a staged generation whose network requirement is already confirmed.
func (m *Manager) ActivateDevelopmentLink(ctx context.Context, key InstanceKey) (*Extension, error) {
	operation, err := m.beginDevOperation(ctx, key)
	if err != nil {
		return nil, err
	}
	defer operation.close()
	ctx = operation.ctx
	key = key.Normalize()
	coordinator := m.coordinatorFor(key)
	coordinator.Lock()
	defer coordinator.Unlock()

	link, err := m.registry.GetDevLink(key.Name, key.WorkspaceID)
	if err != nil {
		return nil, err
	}
	verified, err := m.resolveDevGeneration(ctx, key, link.OriginPath, link.BundleGeneration)
	if err != nil {
		return nil, err
	}
	if err := operation.ensureActive(); err != nil {
		return nil, err
	}
	extension, err := m.activateDevelopmentLinkLocked(ctx, key, link, verified)
	if err != nil {
		if operationErr := operation.ensureActive(); operationErr != nil {
			return nil, errors.Join(err, operationErr)
		}
	}
	return extension, err
}

func (m *Manager) stageDevelopmentLinkLocked(
	key InstanceKey,
	verified *verifiedDevGeneration,
) (*DevLink, error) {
	return m.registry.LinkDev(DevLinkRequest{
		Name:                     key.Name,
		WorkspaceID:              key.WorkspaceID,
		OriginPath:               verified.OriginPath,
		GenerationHash:           verified.GenerationHash,
		NetworkRequirementDigest: verified.NetworkRequirementDigest,
		Format:                   verified.Manifest.Format,
		IngestDiagnostics:        verified.Manifest.IngestDiagnostics,
	})
}

func (m *Manager) activateDevelopmentLinkLocked(
	ctx context.Context,
	key InstanceKey,
	link *DevLink,
	verified *verifiedDevGeneration,
) (*Extension, error) {
	if developmentLinkRequiresConfirmation(link, verified.NetworkRequirementDigest) {
		return nil, &NetworkConfirmationRequiredError{CurrentDigest: verified.NetworkRequirementDigest}
	}
	candidate, err := m.startVerifiedDevCandidate(ctx, key, verified)
	if err != nil {
		return nil, err
	}
	candidate.lastGoodGeneration = verified.GenerationHash
	previous, err := m.activateAndPublishDevCandidate(ctx, key, candidate)
	if err != nil {
		return nil, err
	}
	m.stopReplacedDevProcess(ctx, previous)
	snapshot := m.cloneExtension(candidate)
	snapshot.DevLink = link
	_, publishedErr := m.registry.Get(key.Name)
	snapshot.OverridesPublished = publishedErr == nil
	return snapshot, nil
}

func (m *Manager) developmentLinkSnapshot(key InstanceKey) (*DevLink, error) {
	link, err := m.registry.GetDevLink(key.Name, key.WorkspaceID)
	if errors.Is(err, ErrExtensionNotDevLinked) {
		return nil, nil
	}
	return link, err
}

func (m *Manager) restoreDevelopmentLinkSnapshot(key InstanceKey, snapshot *DevLink) error {
	if snapshot == nil {
		err := m.registry.UnlinkDev(key.Name, key.WorkspaceID)
		if errors.Is(err, ErrExtensionNotDevLinked) {
			return nil
		}
		return err
	}
	if _, err := m.registry.LinkDev(DevLinkRequest{
		Name:                     snapshot.ExtensionName,
		WorkspaceID:              snapshot.WorkspaceID,
		OriginPath:               snapshot.OriginPath,
		GenerationHash:           snapshot.BundleGeneration,
		NetworkRequirementDigest: snapshot.NetworkRequirementDigest,
		Format:                   snapshot.Format,
		IngestDiagnostics:        snapshot.IngestDiagnostics,
	}); err != nil {
		return err
	}
	return m.registry.RestoreNetworkConfirmation(key, NetworkConfirmation{
		Digest: snapshot.NetworkRequirementDigest, ConfirmedBy: snapshot.NetworkConfirmedBy,
		ConfirmedAt: snapshot.NetworkConfirmedAt,
	})
}

func developmentLinkRequiresConfirmation(link *DevLink, digest string) bool {
	digest = strings.TrimSpace(digest)
	return digest != "" && (link == nil || strings.TrimSpace(link.NetworkRequirementDigest) != digest ||
		strings.TrimSpace(link.NetworkConfirmedBy) == "" || link.NetworkConfirmedAt.IsZero())
}
