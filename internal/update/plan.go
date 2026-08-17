package update

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
)

// PlanOperation verifies release identities and builds a durable acquisition request.
func (m *Manager) PlanOperation(
	ctx context.Context,
	actor Actor,
	targets []Target,
	holder Holder,
) (request OperationRequest, returnErr error) {
	if err := validateOperationRequestIdentity(actor, targets, holder); err != nil {
		return OperationRequest{}, err
	}
	state, release, err := m.CheckAll(ctx, CheckOptions{ForceRefresh: true})
	if err != nil {
		return OperationRequest{}, err
	}
	if release == nil {
		return OperationRequest{}, errors.New("update: release metadata is unavailable")
	}
	if err := validatePlanTargets(state, targets); err != nil {
		return OperationRequest{}, err
	}
	assets, err := m.resolvePlanReleaseAssets(release, slices.Contains(targets, TargetRuntime))
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
	compatibility, err := readCompatibility(downloaded.compatibilityPath)
	if err != nil {
		return OperationRequest{}, err
	}
	if err := validateCompatibilityRelease(compatibility, release.Version); err != nil {
		return OperationRequest{}, err
	}

	request = OperationRequest{
		RequestedBy: actor,
		Targets:     append([]Target(nil), targets...),
		Holder:      holder,
		Deadline:    m.now().Add(DefaultOperationDeadline),
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
			return OperationRequest{}, &TargetError{Target: target, Err: err}
		}
	}
	return request, nil
}

func validatePlanTargets(state MultiState, targets []Target) error {
	for _, target := range targets {
		switch target {
		case TargetRuntime:
			if state.Runtime.Status != StatusAvailable || state.Runtime.Managed {
				return errors.New("update: runtime is not eligible for in-place update")
			}
		case TargetApp:
			if state.App == nil || state.App.Status != StatusAvailable {
				return errors.New("update: desktop app update is not available")
			}
		default:
			return fmt.Errorf("update: invalid target %q", target)
		}
	}
	return nil
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
		ArtifactIdentity: plannedArtifactIdentity(
			plan.state.Runtime.CurrentVersion,
			plan.release.Version,
			plan.assets.archive.Name,
			digest,
		),
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
		ArtifactIdentity: plannedArtifactIdentity(
			plan.state.App.CurrentVersion,
			plan.release.Version,
			appAsset.Name,
			digest,
		),
		AttemptID:           attemptID,
		Phase:               PhasePending,
		ConsecutiveFailures: consecutiveAppFailures(plan.state.App.CurrentVersion, plan.archivedApp),
	}, nil
}

func plannedArtifactIdentity(fromVersion, toVersion, asset, digest string) ArtifactIdentity {
	return ArtifactIdentity{
		FromVersion: fromVersion,
		ToVersion:   toVersion,
		ReleaseTag:  toVersion,
		Asset:       asset,
		Digest:      "sha256:" + digest,
	}
}

func consecutiveAppFailures(currentVersion string, archived *Operation) int {
	return archivedAppFailureCount(currentVersion, archived)
}

// AcquireOperation publishes one verified update plan.
func (m *Manager) AcquireOperation(ctx context.Context, request OperationRequest) (*Operation, error) {
	return m.operationStore.Acquire(ctx, request)
}

func (m *Manager) resolveAppReleaseAsset(release *Release) (ReleaseAsset, error) {
	version := strings.TrimPrefix(strings.TrimSpace(release.Version), "v")
	var name string
	switch m.runtimeOS {
	case runtimeOSDarwin:
		arch := "arm64"
		if m.runtimeArch == runtimeArchAMD64 {
			arch = "x64"
		} else if m.runtimeArch != runtimeArchARM64 {
			return ReleaseAsset{}, fmt.Errorf("update: unsupported architecture %q", m.runtimeArch)
		}
		name = "CompozyOS-" + version + "-mac-" + arch + ".zip"
	case runtimeOSLinux:
		if m.runtimeArch != runtimeArchAMD64 {
			return ReleaseAsset{}, fmt.Errorf("update: desktop app auto-update is unsupported on linux/%s", m.runtimeArch)
		}
		name = "CompozyOS-" + version + "-linux-x64.AppImage"
	default:
		return ReleaseAsset{}, fmt.Errorf("update: desktop app auto-update is unsupported on %s", m.runtimeOS)
	}
	if asset, ok := release.findAsset(name); ok {
		return asset, nil
	}
	return ReleaseAsset{}, fmt.Errorf(
		"update: release %q does not publish %s for %s",
		release.Version,
		name,
		m.runtimeArch,
	)
}
