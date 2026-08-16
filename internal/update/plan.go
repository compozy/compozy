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
	archivedApp, err := m.operationStore.LatestArchivedApp(ctx)
	if err != nil {
		return OperationRequest{}, err
	}
	plan := operationPlan{
		state:       state,
		release:     release,
		downloaded:  downloaded,
		assets:      assets,
		archivedApp: archivedApp,
	}
	for _, target := range targets {
		if err := m.addPlannedTarget(&request, target, plan); err != nil {
			return OperationRequest{}, err
		}
	}
	return request, nil
}

type operationPlan struct {
	state       MultiState
	release     *Release
	downloaded  downloadedReleaseArtifacts
	assets      releaseAssets
	archivedApp *Operation
}

func (m *Manager) addPlannedTarget(request *OperationRequest, target Target, plan operationPlan) error {
	switch target {
	case TargetRuntime:
		runtimeState, err := plannedRuntimeState(plan)
		request.Runtime = runtimeState
		return err
	case TargetApp:
		appState, err := m.plannedAppState(plan)
		request.App = appState
		return err
	default:
		return fmt.Errorf("update: invalid target %q", target)
	}
}

func plannedRuntimeState(plan operationPlan) (*RuntimeOperationState, error) {
	digest, err := checksumForAsset(plan.downloaded.checksumsPath, plan.assets.archive.Name)
	if err != nil {
		return nil, err
	}
	return &RuntimeOperationState{
		ArtifactIdentity: ArtifactIdentity{
			FromVersion: plan.state.Runtime.CurrentVersion,
			ToVersion:   plan.release.Version,
			ReleaseTag:  plan.release.Version,
			Asset:       plan.assets.archive.Name,
			Digest:      "sha256:" + digest,
		},
		InstallMethod: InstallMethod(plan.state.Runtime.InstallMethod),
		Phase:         PhasePending,
	}, nil
}

func (m *Manager) plannedAppState(plan operationPlan) (*AppOperationState, error) {
	if plan.state.App == nil {
		return nil, errors.New("update: desktop app is not installed")
	}
	appAsset, err := m.resolveAppReleaseAsset(plan.release)
	if err != nil {
		return nil, err
	}
	digest, err := checksumForAsset(plan.downloaded.checksumsPath, appAsset.Name)
	if err != nil {
		return nil, err
	}
	attemptID, err := randomOperationID()
	if err != nil {
		return nil, err
	}
	return &AppOperationState{
		ArtifactIdentity: ArtifactIdentity{
			FromVersion: plan.state.App.CurrentVersion,
			ToVersion:   plan.release.Version,
			ReleaseTag:  plan.release.Version,
			Asset:       appAsset.Name,
			Digest:      "sha256:" + digest,
		},
		AttemptID:           attemptID,
		Phase:               PhasePending,
		ConsecutiveFailures: consecutiveAppFailures(plan.state.App.CurrentVersion, plan.archivedApp),
	}, nil
}

func consecutiveAppFailures(currentVersion string, archived *Operation) int {
	if archived == nil || archived.App == nil || archived.App.Phase != PhaseFailed ||
		archived.App.ConsecutiveFailures < 1 {
		return 0
	}
	comparison, err := compareVersions(currentVersion, archived.App.ToVersion)
	if err == nil && comparison >= 0 {
		return 0
	}
	return archived.App.ConsecutiveFailures
}

// AcquireOperation publishes one verified update plan.
func (m *Manager) AcquireOperation(ctx context.Context, request OperationRequest) (*Operation, error) {
	return m.operationStore.Acquire(ctx, request)
}

func (m *Manager) resolveAppReleaseAsset(release *Release) (ReleaseAsset, error) {
	var suffix string
	switch m.runtimeOS {
	case runtimeOSDarwin:
		suffix = ".zip"
	case runtimeOSLinux:
		suffix = ".appimage"
	default:
		return ReleaseAsset{}, fmt.Errorf("update: desktop app auto-update is unsupported on %s", m.runtimeOS)
	}
	wantedArch := []string{strings.ToLower(m.runtimeArch)}
	if m.runtimeArch == runtimeArchAMD64 {
		wantedArch = append(wantedArch, "x64", "x86_64")
	}
	for _, asset := range release.Assets {
		name := strings.ToLower(strings.TrimSpace(asset.Name))
		if !strings.Contains(name, "compozy") || !strings.HasSuffix(name, suffix) {
			continue
		}
		for _, arch := range wantedArch {
			if strings.Contains(name, arch) {
				return asset, nil
			}
		}
	}
	return ReleaseAsset{}, fmt.Errorf(
		"update: release %q has no desktop app asset for %s",
		release.Version,
		m.runtimeArch,
	)
}
