package update

import (
	"context"

	"errors"
	"fmt"

	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// NewManager binds the update flow to the current AGH runtime.
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

	resolvedExecutable, err := executablePath()
	if err != nil {
		return nil, fmt.Errorf("update: resolve executable path: %w", err)
	}
	trimmedExecutable := strings.TrimSpace(resolvedExecutable)
	if trimmedExecutable == "" {
		return nil, errors.New("update: executable path is required")
	}
	resolvedExecutable = trimmedExecutable
	symlinkResolved, err := resolveSymlinks(resolvedExecutable)
	if err == nil && strings.TrimSpace(symlinkResolved) != "" {
		resolvedExecutable = strings.TrimSpace(symlinkResolved)
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
		return nil, errors.New("update: AGH home directory is required")
	}
	return manager, nil
}

// Check returns the current update state and the latest release metadata when available.
func (m *Manager) Check(ctx context.Context, opts CheckOptions) (State, *Release, error) {
	install := m.detectInstall(ctx)
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
			state.Message = "Failed to check for a newer stable AGH release."
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

// ApplyRelease downloads, verifies, extracts, and swaps in the supplied release.
func (m *Manager) ApplyRelease(ctx context.Context, release *Release) (AppliedBinary, error) {
	if release == nil {
		return AppliedBinary{}, errors.New("update: release metadata is required")
	}

	assets, err := m.resolveReleaseAssets(release)
	if err != nil {
		return AppliedBinary{}, err
	}
	currentInfo, err := os.Stat(m.executablePath)
	if err != nil {
		return AppliedBinary{}, fmt.Errorf("update: stat current executable %q: %w", m.executablePath, err)
	}

	tmpDir, err := os.MkdirTemp("", "agh-update-*")
	if err != nil {
		return AppliedBinary{}, fmt.Errorf("update: create temp directory: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	downloaded, err := m.downloadReleaseArtifacts(ctx, tmpDir, assets)
	if err != nil {
		return AppliedBinary{}, err
	}
	if err := m.verifyReleaseArtifacts(ctx, downloaded, assets.archive.Name); err != nil {
		return AppliedBinary{}, err
	}

	binaryPath, binaryMode, err := extractBinaryFromTarGz(
		downloaded.archivePath,
		tmpDir,
		m.archiveBinaryName(),
	)
	if err != nil {
		return AppliedBinary{}, err
	}
	binaryMode = executableBinaryMode(binaryMode, currentInfo.Mode().Perm())

	backupPath := siblingBackupPath(m.executablePath, m.now().UTC())
	if err := m.binaryApplier.ApplyBinary(binaryPath, m.executablePath, backupPath, binaryMode); err != nil {
		return AppliedBinary{}, err
	}

	return AppliedBinary{
		TargetPath: m.executablePath,
		BackupPath: backupPath,
		Version:    strings.TrimSpace(release.Version),
	}, nil
}

type releaseAssets struct {
	archive   ReleaseAsset
	checksums ReleaseAsset
	bundle    ReleaseAsset
}

type downloadedReleaseArtifacts struct {
	archivePath   string
	checksumsPath string
	bundlePath    string
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

	return releaseAssets{
		archive:   archiveAsset,
		checksums: checksumsAsset,
		bundle:    bundleAsset,
	}, nil
}

func (m *Manager) downloadReleaseArtifacts(
	ctx context.Context,
	tmpDir string,
	assets releaseAssets,
) (downloadedReleaseArtifacts, error) {
	downloaded := downloadedReleaseArtifacts{
		archivePath:   filepath.Join(tmpDir, assets.archive.Name),
		checksumsPath: filepath.Join(tmpDir, assets.checksums.Name),
		bundlePath:    filepath.Join(tmpDir, assets.bundle.Name),
	}

	if err := m.downloadFile(
		ctx,
		assets.archive.DownloadURL,
		downloaded.archivePath,
		maxArchiveDownloadBytes,
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
	return verifySHA256(downloaded.archivePath, expectedChecksum)
}
