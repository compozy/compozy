package update

import (
	"context"

	"errors"
	"fmt"
	"log/slog"

	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// NewManager binds the update flow to the current CompozyOS runtime.
func NewManager(cfg Config) (*Manager, error) {
	executablePath := cfg.ExecutablePath
	if executablePath == nil {
		executablePath = os.Executable
	}
	resolveSymlinks := cfg.ResolveSymlinks
	if resolveSymlinks == nil {
		resolveSymlinks = filepath.EvalSymlinks
	}
	getenv := cfg.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	now := cfg.Now
	if now == nil {
		now = func() time.Time {
			return time.Now().UTC()
		}
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultHTTPTimeout}
	}
	lookPath := cfg.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	runCommand := cfg.RunCommand
	if runCommand == nil {
		runCommand = defaultRunCommand
	}
	resolvedExecutable, artifactPolicy, err := resolveManagerInputs(cfg, executablePath, resolveSymlinks)
	if err != nil {
		return nil, err
	}

	releaseTrack := releaseTrackStable
	if !isDevVersion(cfg.CurrentVersion) {
		var trackErr error
		releaseTrack, _, trackErr = releaseTrackForVersion(cfg.CurrentVersion)
		if trackErr != nil {
			return nil, fmt.Errorf("update: resolve release track: %w", trackErr)
		}
	}

	manager := &Manager{
		homePaths:      cfg.HomePaths,
		currentVersion: strings.TrimSpace(cfg.CurrentVersion),
		executablePath: resolvedExecutable,
		getenv:         getenv,
		now:            now,
		httpClient:     httpClient,
		runtimeOS:      strings.TrimSpace(cfg.RuntimeOS),
		runtimeArch:    strings.TrimSpace(cfg.RuntimeArch),
		lookPath:       lookPath,
		runCommand:     runCommand,
		binaryApplier:  cfg.BinaryApplier,
		artifactPolicy: artifactPolicy,
		releaseTrack:   releaseTrack,
	}
	if manager.runtimeOS == "" {
		manager.runtimeOS = runtime.GOOS
	}
	if manager.runtimeArch == "" {
		manager.runtimeArch = runtime.GOARCH
	}
	if manager.binaryApplier == nil {
		manager.binaryApplier = selfBinaryApplier{}
	}
	if cfg.BundleVerifier != nil {
		manager.bundleVerifier = cfg.BundleVerifier
	} else {
		manager.bundleVerifier = sigstoreBundleVerifier{cachePath: manager.sigstoreCachePath()}
	}
	if strings.TrimSpace(manager.homePaths.HomeDir) == "" {
		return nil, errors.New("update: CompozyOS home directory is required")
	}
	operationEvents := cfg.OperationEvents
	if operationEvents == nil {
		operationEvents = NewSlogOperationEventEmitter(slog.Default())
	}
	operationStore, err := NewOperationStore(manager.homePaths, operationEvents)
	if err != nil {
		return nil, err
	}
	manager.operationStore = operationStore
	return manager, nil
}

func resolveManagerInputs(
	cfg Config,
	executablePath func() (string, error),
	resolveSymlinks func(string) (string, error),
) (string, ArtifactPolicy, error) {
	artifactPolicy, err := resolveArtifactPolicy(cfg.ArtifactPolicy)
	if err != nil {
		return "", ArtifactPolicy{}, err
	}
	resolvedExecutable, err := resolveExecutablePath(executablePath, resolveSymlinks)
	if err != nil {
		return "", ArtifactPolicy{}, err
	}
	return resolvedExecutable, artifactPolicy, nil
}

func resolveExecutablePath(
	executablePath func() (string, error),
	resolveSymlinks func(string) (string, error),
) (string, error) {
	resolved, err := executablePath()
	if err != nil {
		return "", fmt.Errorf("update: resolve executable path: %w", err)
	}
	resolved = strings.TrimSpace(resolved)
	if resolved == "" {
		return "", errors.New("update: executable path is required")
	}
	if symlinkResolved, resolveErr := resolveSymlinks(resolved); resolveErr == nil {
		if trimmed := strings.TrimSpace(symlinkResolved); trimmed != "" {
			resolved = trimmed
		}
	}
	return resolved, nil
}

// Check returns the current update state and the latest release metadata when available.
func (m *Manager) Check(ctx context.Context, opts CheckOptions) (State, *Release, error) {
	install, err := m.detectInstall(ctx)
	if err != nil {
		return State{}, nil, err
	}
	if isDevVersion(m.currentVersion) {
		return m.composeState(install, nil, nil), nil, nil
	}

	var (
		latest    *Release
		checkedAt *time.Time
	)
	if cached, err := readCache(m.cachePath()); err == nil {
		latest = cached.release()
		checked := cached.CheckedAt.UTC()
		checkedAt = &checked
	} else if err != nil && !errors.Is(err, ErrNoCachedRelease) {
		return State{}, nil, err
	}

	needRefresh := opts.ForceRefresh || checkedAt == nil
	if !needRefresh && checkedAt != nil {
		needRefresh = m.now().UTC().Sub(checkedAt.UTC()) >= cacheTTL
	}

	if needRefresh {
		freshRelease, err := m.fetchLatestRelease(ctx)
		if err != nil {
			state := m.composeState(install, latest, checkedAt)
			state.LastError = err.Error()
			state.Message = "Failed to check for a newer CompozyOS release on the active channel."
			if latest != nil && opts.AllowCachedOnFailure {
				return state, latest, nil
			}
			state.Status = StatusFailed
			return state, nil, err
		}

		latest = freshRelease
		checked := m.now().UTC()
		checkedAt = &checked
		entry, err := cacheEntryFromRelease(latest, checked)
		if err != nil {
			return State{}, nil, err
		}
		if err := writeCache(m.cachePath(), entry); err != nil {
			return State{}, nil, err
		}
	}

	return m.composeState(install, latest, checkedAt), latest, nil
}

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

	tmpDir, err := os.MkdirTemp("", "compozy-update-*")
	if err != nil {
		return AppliedBinary{}, fmt.Errorf("update: create temp directory: %w", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(tmpDir); removeErr != nil {
			err = errors.Join(err, fmt.Errorf("update: remove temp directory %q: %w", tmpDir, removeErr))
		}
	}()

	if err := notifyApplyObserver(ctx, observer, ApplyStep{Phase: PhaseDownloading, Percent: 0}); err != nil {
		return AppliedBinary{}, err
	}
	downloaded, err := m.downloadReleaseArtifacts(ctx, tmpDir, assets)
	if err != nil {
		return AppliedBinary{}, err
	}
	if err := notifyApplyObserver(ctx, observer, ApplyStep{Phase: PhaseVerifying, Percent: -1}); err != nil {
		return AppliedBinary{}, err
	}
	if err := m.verifyReleaseArtifacts(ctx, downloaded, assets.archive.Name); err != nil {
		return AppliedBinary{}, err
	}
	compatibility, err := readCompatibility(downloaded.compatibilityPath)
	if err != nil {
		return AppliedBinary{}, err
	}

	binaryPath, binaryMode, err := extractBinaryFromTarGz(
		downloaded.archivePath,
		tmpDir,
		m.archiveBinaryName(),
		m.artifactPolicy,
	)
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
	if install.Method == string(InstallMethodDesktopApp) {
		if err := rewriteDesktopProvenance(m.homePaths, m.executablePath); err != nil {
			restoreErr := m.binaryApplier.RestoreBinary(backupPath, m.executablePath, currentInfo.Mode().Perm())
			if restoreErr == nil {
				restoreErr = rewriteDesktopProvenance(m.homePaths, m.executablePath)
			}
			return AppliedBinary{}, errors.Join(err, restoreErr)
		}
	}

	return AppliedBinary{
		TargetPath: m.executablePath,
		BackupPath: backupPath,
		Version:    strings.TrimSpace(release.Version),
	}, nil
}

func notifyApplyObserver(ctx context.Context, observer ApplyObserver, step ApplyStep) error {
	if observer == nil {
		return nil
	}
	return observer(ctx, step)
}

// RuntimeTargetPath returns the concrete executable path owned by this manager.
func (m *Manager) RuntimeTargetPath() string { return m.executablePath }

type releaseAssets struct {
	archive   ReleaseAsset
	checksums ReleaseAsset
	bundle    ReleaseAsset
	compat    ReleaseAsset
}

type downloadedReleaseArtifacts struct {
	archivePath       string
	checksumsPath     string
	bundlePath        string
	compatibilityPath string
}

func (m *Manager) resolveReleaseAssets(release *Release) (releaseAssets, error) {
	archiveName, err := archiveAssetName(m.runtimeOS, m.runtimeArch)
	if err != nil {
		return releaseAssets{}, err
	}

	archiveAsset, ok := release.findAsset(archiveName)
	if !ok {
		return releaseAssets{}, fmt.Errorf(
			"update: release %q does not publish %s",
			release.Version,
			archiveName,
		)
	}
	checksumsAsset, ok := release.findAsset(checksumsAssetName)
	if !ok {
		return releaseAssets{}, fmt.Errorf(
			"update: release %q is missing %s",
			release.Version,
			checksumsAssetName,
		)
	}
	bundleAsset, ok := release.findAsset(checksumsBundleAssetName)
	if !ok {
		return releaseAssets{}, fmt.Errorf(
			"update: release %q is missing %s",
			release.Version,
			checksumsBundleAssetName,
		)
	}
	compatibilityAsset, ok := release.findAsset(compatibilityAssetName)
	if !ok {
		return releaseAssets{}, fmt.Errorf(
			"update: release %q is missing %s",
			release.Version,
			compatibilityAssetName,
		)
	}

	return releaseAssets{
		archive:   archiveAsset,
		checksums: checksumsAsset,
		bundle:    bundleAsset,
		compat:    compatibilityAsset,
	}, nil
}

func (m *Manager) downloadReleaseArtifacts(
	ctx context.Context,
	tmpDir string,
	assets releaseAssets,
) (downloadedReleaseArtifacts, error) {
	downloaded := downloadedReleaseArtifacts{
		archivePath:       filepath.Join(tmpDir, assets.archive.Name),
		checksumsPath:     filepath.Join(tmpDir, assets.checksums.Name),
		bundlePath:        filepath.Join(tmpDir, assets.bundle.Name),
		compatibilityPath: filepath.Join(tmpDir, assets.compat.Name),
	}

	if err := m.downloadFile(
		ctx,
		assets.archive.DownloadURL,
		downloaded.archivePath,
		m.artifactPolicy.MaxArchiveBytes,
	); err != nil {
		var downloadErr *downloadSizeError
		if errors.As(err, &downloadErr) {
			return downloadedReleaseArtifacts{}, &ArtifactSizeError{
				Kind:  ArtifactKindArchive,
				Size:  downloadErr.Size,
				Limit: downloadErr.Limit,
			}
		}
		return downloadedReleaseArtifacts{}, err
	}
	if err := m.downloadFile(
		ctx,
		assets.compat.DownloadURL,
		downloaded.compatibilityPath,
		maxCompatibilityBytes,
	); err != nil {
		return downloadedReleaseArtifacts{}, err
	}
	if err := m.downloadFile(
		ctx,
		assets.checksums.DownloadURL,
		downloaded.checksumsPath,
		maxChecksumsBytes,
	); err != nil {
		return downloadedReleaseArtifacts{}, err
	}
	if err := m.downloadFile(
		ctx,
		assets.bundle.DownloadURL,
		downloaded.bundlePath,
		maxSigstoreBundleBytes,
	); err != nil {
		return downloadedReleaseArtifacts{}, err
	}

	return downloaded, nil
}

func (m *Manager) verifyReleaseArtifacts(
	ctx context.Context,
	downloaded downloadedReleaseArtifacts,
	archiveName string,
) error {
	if err := m.bundleVerifier.VerifyChecksums(ctx, downloaded.checksumsPath, downloaded.bundlePath); err != nil {
		return err
	}

	expectedChecksum, err := checksumForAsset(downloaded.checksumsPath, archiveName)
	if err != nil {
		return err
	}
	if err := verifySHA256(downloaded.archivePath, expectedChecksum); err != nil {
		return err
	}
	expectedCompatibilityChecksum, err := checksumForAsset(downloaded.checksumsPath, compatibilityAssetName)
	if err != nil {
		return err
	}
	return verifySHA256(downloaded.compatibilityPath, expectedCompatibilityChecksum)
}
