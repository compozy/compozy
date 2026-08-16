package update

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

// ApplyStep reports the next journal phase immediately before its work begins.
type ApplyStep struct {
	Phase         OperationPhase
	Percent       int
	BackupPath    string
	Compatibility *Compatibility
}

// ApplyObserver journals and fences one apply step before the manager performs it.
type ApplyObserver func(context.Context, ApplyStep) error

// ApplyRelease downloads, verifies, extracts, and swaps in the supplied release.
func (m *Manager) ApplyRelease(ctx context.Context, release *Release) (AppliedBinary, error) {
	return m.applyRelease(ctx, release, nil)
}

// ApplyReleaseObserved exposes real apply boundaries to the operation coordinator.
func (m *Manager) ApplyReleaseObserved(
	ctx context.Context,
	release *Release,
	observer ApplyObserver,
) (AppliedBinary, error) {
	if observer == nil {
		return AppliedBinary{}, errors.New("update: apply observer is required")
	}
	return m.applyRelease(ctx, release, observer)
}

func (m *Manager) applyRelease(
	ctx context.Context,
	release *Release,
	observer ApplyObserver,
) (applied AppliedBinary, err error) {
	if release == nil {
		return AppliedBinary{}, errors.New("update: release metadata is required")
	}
	install, err := m.detectInstall(ctx)
	if err != nil {
		return AppliedBinary{}, err
	}
	assets, err := m.resolveReleaseAssets(release)
	if err != nil {
		return AppliedBinary{}, err
	}
	currentInfo, err := os.Stat(m.executablePath)
	if err != nil {
		return AppliedBinary{}, fmt.Errorf("update: stat current executable %q: %w", m.executablePath, err)
	}

	tempDir, err := os.MkdirTemp("", "compozy-update-*")
	if err != nil {
		return AppliedBinary{}, fmt.Errorf("update: create temp directory: %w", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(tempDir); removeErr != nil {
			err = errors.Join(err, fmt.Errorf("update: remove temp directory %q: %w", tempDir, removeErr))
		}
	}()

	binaryPath, binaryMode, compatibility, err := m.prepareReleaseBinary(ctx, tempDir, assets, observer)
	if err != nil {
		return AppliedBinary{}, err
	}
	binaryMode = executableBinaryMode(binaryMode, currentInfo.Mode().Perm())
	backupPath := siblingBackupPath(m.executablePath, m.now().UTC())
	if err := notifyApplyObserver(ctx, observer, ApplyStep{
		Phase: PhaseSwapping, Percent: -1, BackupPath: backupPath, Compatibility: &compatibility,
	}); err != nil {
		return AppliedBinary{}, err
	}
	if err := m.binaryApplier.ApplyBinary(binaryPath, m.executablePath, backupPath, binaryMode); err != nil {
		return AppliedBinary{}, err
	}
	if err := m.rewriteAppliedDesktopProvenance(install, backupPath, currentInfo.Mode().Perm()); err != nil {
		return AppliedBinary{}, err
	}
	return AppliedBinary{
		TargetPath: m.executablePath,
		BackupPath: backupPath,
		Version:    strings.TrimSpace(release.Version),
	}, nil
}

func (m *Manager) prepareReleaseBinary(
	ctx context.Context,
	tempDir string,
	assets releaseAssets,
	observer ApplyObserver,
) (string, os.FileMode, Compatibility, error) {
	if err := notifyApplyObserver(ctx, observer, ApplyStep{Phase: PhaseDownloading, Percent: 0}); err != nil {
		return "", 0, Compatibility{}, err
	}
	downloaded, err := m.downloadReleaseArtifacts(ctx, tempDir, assets)
	if err != nil {
		return "", 0, Compatibility{}, err
	}
	if err := notifyApplyObserver(ctx, observer, ApplyStep{Phase: PhaseVerifying, Percent: -1}); err != nil {
		return "", 0, Compatibility{}, err
	}
	if err := m.verifyReleaseArtifacts(ctx, downloaded, assets.archive.Name); err != nil {
		return "", 0, Compatibility{}, err
	}
	compatibility, err := readCompatibility(downloaded.compatibilityPath)
	if err != nil {
		return "", 0, Compatibility{}, err
	}
	binaryPath, binaryMode, err := extractBinaryFromTarGz(
		downloaded.archivePath,
		tempDir,
		m.archiveBinaryName(),
		m.artifactPolicy,
	)
	return binaryPath, binaryMode, compatibility, err
}

func (m *Manager) rewriteAppliedDesktopProvenance(
	install installInfo,
	backupPath string,
	currentMode os.FileMode,
) error {
	if install.Method != string(InstallMethodDesktopApp) {
		return nil
	}
	if err := rewriteDesktopProvenance(m.homePaths, m.executablePath); err != nil {
		restoreErr := m.binaryApplier.RestoreBinary(backupPath, m.executablePath, currentMode)
		if restoreErr == nil {
			restoreErr = rewriteDesktopProvenance(m.homePaths, m.executablePath)
		}
		return errors.Join(err, restoreErr)
	}
	return nil
}

func notifyApplyObserver(ctx context.Context, observer ApplyObserver, step ApplyStep) error {
	if observer == nil {
		return nil
	}
	return observer(ctx, step)
}
