package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"strings"

	diagnosticcontract "github.com/compozy/compozy/internal/diagnosticcontract"
)

const (
	extensionFailureMissingOrigin    = "missing_origin"
	extensionFailureActivationFailed = "activation_failed"
)

// DevelopmentGeneration is the verified, mutation-free identity of one dev artifact.
type DevelopmentGeneration struct {
	Name                     string
	OriginPath               string
	GenerationHash           string
	NetworkRequirementDigest string
	Format                   ExtensionFormat
	IngestDiagnostics        []diagnosticcontract.DiagnosticItem
}

// InspectDevelopmentGeneration verifies a dev artifact without linking or starting it.
func (m *Manager) InspectDevelopmentGeneration(
	ctx context.Context,
	workspaceID string,
	originPath string,
	generationHash string,
) (DevelopmentGeneration, error) {
	if m == nil {
		return DevelopmentGeneration{}, ErrManagerRequired
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return DevelopmentGeneration{}, errors.New("extension: workspace id is required for a development generation")
	}
	if m.workspaceResolver == nil {
		return DevelopmentGeneration{}, errors.New(
			"extension: workspace resolver is required for development extensions",
		)
	}
	workspace, err := m.workspaceResolver.Resolve(ctx, workspaceID)
	if err != nil {
		return DevelopmentGeneration{}, fmt.Errorf("extension: resolve development workspace %q: %w", workspaceID, err)
	}
	canonicalOrigin, err := canonicalizeDevOrigin(workspace.RootDir, originPath)
	if err != nil {
		return DevelopmentGeneration{}, err
	}
	verified, err := m.verifyDevelopmentGeneration(canonicalOrigin, generationHash, workspaceID)
	if err != nil {
		return DevelopmentGeneration{}, err
	}
	return DevelopmentGeneration{
		Name:                     strings.TrimSpace(verified.Manifest.Name),
		OriginPath:               verified.OriginPath,
		GenerationHash:           verified.GenerationHash,
		NetworkRequirementDigest: verified.NetworkRequirementDigest,
		Format:                   verified.Manifest.Format,
		IngestDiagnostics:        verified.Manifest.IngestDiagnostics,
	}, nil
}

// LinkDevelopmentFromOrigin derives the authored name from a verified generation.
func (m *Manager) LinkDevelopmentFromOrigin(
	ctx context.Context,
	workspaceID string,
	originPath string,
	generationHash string,
) (*Extension, error) {
	if m == nil {
		return nil, ErrManagerRequired
	}
	verified, err := m.InspectDevelopmentGeneration(ctx, workspaceID, originPath, generationHash)
	if err != nil {
		return nil, err
	}
	return m.LinkDevelopment(
		ctx,
		InstanceKey{Name: verified.Name, WorkspaceID: workspaceID},
		verified.OriginPath,
		verified.GenerationHash,
	)
}

// LinkDevelopment verifies, activates, and persists one workspace-local generation.
func (m *Manager) LinkDevelopment(
	ctx context.Context,
	key InstanceKey,
	originPath string,
	generationHash string,
) (*Extension, error) {
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
	previous, err := m.developmentLinkSnapshot(key)
	if err != nil {
		return nil, err
	}
	link, err := m.stageDevelopmentLinkLocked(key, verified)
	if err != nil {
		return nil, err
	}
	if developmentLinkRequiresConfirmation(link, verified.NetworkRequirementDigest) {
		confirmationErr := &NetworkConfirmationRequiredError{CurrentDigest: verified.NetworkRequirementDigest}
		return nil, errors.Join(confirmationErr, m.restoreDevelopmentLinkSnapshot(key, previous))
	}
	extension, err := m.activateDevelopmentLinkLocked(ctx, key, link, verified)
	if err != nil {
		restoreErr := m.restoreDevelopmentLinkSnapshot(key, previous)
		if operationErr := operation.ensureActive(); operationErr != nil {
			return nil, errors.Join(err, restoreErr, operationErr)
		}
		return nil, errors.Join(err, restoreErr)
	}
	return extension, nil
}

// ReloadExtension atomically replaces one active dev generation.
func (m *Manager) ReloadExtension(
	ctx context.Context,
	key InstanceKey,
	generationHash string,
) (*Extension, error) {
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
	current, ok := m.lookupInstance(key)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrExtensionNotDevLinked, key.runtimeID())
	}
	verified, err := m.resolveDevGeneration(ctx, key, link.OriginPath, generationHash)
	if err != nil {
		return nil, err
	}
	candidate, activationErr := m.startVerifiedDevCandidate(ctx, key, verified)
	if activationErr != nil {
		if operationErr := operation.ensureActive(); operationErr != nil {
			return nil, errors.Join(activationErr, operationErr)
		}
		return m.restartLastGood(key, current, activationErr)
	}
	if err := operation.ensureActive(); err != nil {
		m.discardDevCandidate(ctx, candidate)
		return nil, err
	}
	updatedLink, err := m.registry.LinkDev(DevLinkRequest{
		Name:                     key.Name,
		WorkspaceID:              key.WorkspaceID,
		OriginPath:               verified.OriginPath,
		GenerationHash:           verified.GenerationHash,
		NetworkRequirementDigest: verified.NetworkRequirementDigest,
		Format:                   verified.Manifest.Format,
		IngestDiagnostics:        verified.Manifest.IngestDiagnostics,
	})
	if err != nil {
		m.discardDevCandidate(ctx, candidate)
		return nil, err
	}
	candidate.lastGoodGeneration = verified.GenerationHash
	previous, err := m.activateAndPublishDevCandidate(ctx, key, candidate)
	if err != nil {
		rollbackErr := m.restoreDevLink(key, link)
		if rollbackErr != nil {
			err = errors.Join(err, rollbackErr)
		}
		if operationErr := operation.ensureActive(); operationErr != nil {
			return nil, errors.Join(err, operationErr)
		}
		return m.restartLastGood(key, current, err)
	}
	m.stopReplacedDevProcess(ctx, previous)
	snapshot := m.cloneExtension(candidate)
	snapshot.DevLink = updatedLink
	_, publishedErr := m.registry.Get(key.Name)
	snapshot.OverridesPublished = publishedErr == nil
	return snapshot, nil
}

// UnlinkDevelopment removes and stops only one workspace-local instance.
func (m *Manager) UnlinkDevelopment(ctx context.Context, key InstanceKey) error {
	operation, err := m.beginDevOperation(ctx, key)
	if err != nil {
		return err
	}
	defer operation.close()
	ctx = operation.ctx
	key = key.Normalize()
	coordinator := m.coordinatorFor(key)
	coordinator.Lock()
	defer coordinator.Unlock()
	if err := operation.ensureActive(); err != nil {
		return err
	}
	if err := m.registry.UnlinkDev(key.Name, key.WorkspaceID); err != nil {
		return err
	}
	current, _ := m.lookupInstance(key)
	m.mu.Lock()
	m.deleteInstanceLocked(key)
	delete(m.devLogs, key)
	m.mu.Unlock()
	if current != nil {
		return m.stopManagedExtension(ctx, current)
	}
	return nil
}

// Logs returns retained redacted stderr entries after a cursor within the current ring identity.
func (m *Manager) Logs(key InstanceKey, cursor ExtensionLogCursor) (ExtensionLogSnapshot, error) {
	if m == nil {
		return ExtensionLogSnapshot{}, ErrManagerRequired
	}
	key = key.Normalize()
	if err := key.Validate(); err != nil {
		return ExtensionLogSnapshot{}, err
	}
	if !key.IsGlobal() {
		if _, err := m.registry.GetDevLink(key.Name, key.WorkspaceID); err != nil {
			return ExtensionLogSnapshot{}, err
		}
	} else if _, err := m.registry.Get(key.Name); err != nil {
		return ExtensionLogSnapshot{}, err
	}
	return m.logRingFor(key).snapshot(cursor), nil
}

func (m *Manager) startDevLinkOnBoot(ctx context.Context, link DevLink) {
	key := InstanceKey{Name: link.ExtensionName, WorkspaceID: link.WorkspaceID}.Normalize()
	verified, err := m.resolveDevGeneration(ctx, key, link.OriginPath, link.BundleGeneration)
	if err != nil {
		failureCode := devBootFailureCode(err)
		ext := &managedExtension{
			key: key,
			info: ExtensionInfo{
				Name: key.Name, Source: SourceWorkspace, Enabled: true,
				Format: link.Format, IngestDiagnostics: cloneDiagnosticItems(link.IngestDiagnostics),
			},
			generationHash:     link.BundleGeneration,
			lastGoodGeneration: link.BundleGeneration,
			logRing:            m.logRingFor(key),
		}
		m.setDevBootFailure(key, ext, failureCode, err)
		m.mu.Lock()
		m.devExtensions[key] = ext
		m.mu.Unlock()
		return
	}
	ext, err := m.startVerifiedDevCandidate(ctx, key, verified)
	if err != nil {
		ext = managedDevExtension(key, verified, m.logRingFor(key))
		m.setDevBootFailure(key, ext, extensionFailureActivationFailed, err)
	}
	ext.lastGoodGeneration = link.BundleGeneration
	if err == nil {
		ext.lastGoodGeneration = link.BundleGeneration
		if _, activateErr := m.activateAndPublishDevCandidate(ctx, key, ext); activateErr != nil {
			m.setDevBootFailure(key, ext, extensionFailureActivationFailed, activateErr)
			m.swapDevInstance(key, ext)
			return
		}
		return
	}
	m.swapDevInstance(key, ext)
}

func (m *Manager) restoreDevLink(key InstanceKey, prior *DevLink) error {
	if prior == nil {
		if err := m.registry.UnlinkDev(key.Name, key.WorkspaceID); err != nil &&
			!errors.Is(err, ErrExtensionNotDevLinked) {
			return fmt.Errorf("extension: rollback development link %q: %w", key.runtimeID(), err)
		}
		return nil
	}
	if _, err := m.registry.LinkDev(DevLinkRequest{
		Name:                     prior.ExtensionName,
		WorkspaceID:              prior.WorkspaceID,
		OriginPath:               prior.OriginPath,
		GenerationHash:           prior.BundleGeneration,
		NetworkRequirementDigest: prior.NetworkRequirementDigest,
		Format:                   prior.Format,
		IngestDiagnostics:        prior.IngestDiagnostics,
	}); err != nil {
		return fmt.Errorf("extension: restore development link %q: %w", key.runtimeID(), err)
	}
	return nil
}
