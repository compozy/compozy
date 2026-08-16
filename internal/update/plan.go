package update

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// PlanOperation verifies release identities and builds a durable acquisition request.
func (m *Manager) PlanOperation(
	ctx context.Context,
	actor Actor,
	targets []Target,
	holder Holder,
) (request OperationRequest, returnErr error) {
	if err := validateTargets(targets); err != nil {
		return OperationRequest{}, err
	}
	state, release, err := m.CheckAll(ctx, CheckOptions{ForceRefresh: true})
	if err != nil {
		return OperationRequest{}, err
	}
	if release == nil {
		return OperationRequest{}, errors.New("update: release metadata is unavailable")
	}
	assets, err := m.resolveReleaseAssets(release)
	if err != nil {
		return OperationRequest{}, err
	}
	tempDir, err := os.MkdirTemp("", "compozy-update-plan-*")
	if err != nil {
		return OperationRequest{}, fmt.Errorf("update: create plan temp directory: %w", err)
	}
	defer func() {
		returnErr = errors.Join(returnErr, os.RemoveAll(tempDir))
	}()
	downloaded, err := m.downloadReleaseArtifacts(ctx, tempDir, assets)
	if err != nil {
		return OperationRequest{}, err
	}
	if err := m.verifyReleaseArtifacts(ctx, downloaded, assets.archive.Name); err != nil {
		return OperationRequest{}, err
	}
	if _, err := readCompatibility(downloaded.compatibilityPath); err != nil {
		return OperationRequest{}, err
	}

	request = OperationRequest{
		RequestedBy: actor,
		Targets:     append([]Target(nil), targets...),
		Holder:      holder,
		Deadline:    m.now().Add(30 * time.Minute),
	}
	for _, target := range targets {
		switch target {
		case TargetRuntime:
			digest, err := checksumForAsset(downloaded.checksumsPath, assets.archive.Name)
			if err != nil {
				return OperationRequest{}, err
			}
			request.Runtime = &RuntimeOperationState{
				ArtifactIdentity: ArtifactIdentity{
					FromVersion: state.Runtime.CurrentVersion, ToVersion: release.Version,
					ReleaseTag: release.Version, Asset: assets.archive.Name, Digest: "sha256:" + digest,
				},
				InstallMethod: InstallMethod(state.Runtime.InstallMethod),
				Phase:         PhasePending,
			}
		case TargetApp:
			if state.App == nil {
				return OperationRequest{}, errors.New("update: desktop app is not installed")
			}
			appAsset, err := m.resolveAppReleaseAsset(release)
			if err != nil {
				return OperationRequest{}, err
			}
			digest, err := checksumForAsset(downloaded.checksumsPath, appAsset.Name)
			if err != nil {
				return OperationRequest{}, err
			}
			attemptID, err := randomOperationID()
			if err != nil {
				return OperationRequest{}, err
			}
			request.App = &AppOperationState{
				ArtifactIdentity: ArtifactIdentity{
					FromVersion: state.App.CurrentVersion, ToVersion: release.Version,
					ReleaseTag: release.Version, Asset: appAsset.Name, Digest: "sha256:" + digest,
				},
				AttemptID: attemptID,
				Phase:     PhasePending,
			}
		}
	}
	return request, nil
}

// AcquireOperation publishes one verified update plan.
func (m *Manager) AcquireOperation(ctx context.Context, request OperationRequest) (*Operation, error) {
	return m.operationStore.Acquire(ctx, request)
}

func (m *Manager) resolveAppReleaseAsset(release *Release) (ReleaseAsset, error) {
	wantedArch := []string{strings.ToLower(m.runtimeArch)}
	if m.runtimeArch == runtimeArchAMD64 {
		wantedArch = append(wantedArch, "x64", "x86_64")
	}
	for _, asset := range release.Assets {
		name := strings.ToLower(strings.TrimSpace(asset.Name))
		if !strings.Contains(name, "compozy") ||
			(!strings.HasSuffix(name, ".dmg") && !strings.HasSuffix(name, ".zip") && !strings.HasSuffix(name, ".appimage")) {
			continue
		}
		for _, arch := range wantedArch {
			if strings.Contains(name, arch) {
				return asset, nil
			}
		}
	}
	return ReleaseAsset{}, fmt.Errorf("update: release %q has no desktop app asset for %s", release.Version, m.runtimeArch)
}
