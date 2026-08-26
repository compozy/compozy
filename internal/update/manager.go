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

var _ CoordinatorManager = (*Manager)(nil)

// NewManager binds the update flow to the current CompozyOS runtime.
func NewManager(cfg *Config) (*Manager, error) {
	if cfg == nil {
		return nil, errors.New("update: manager config is required")
	}
	dependencies := managerDependenciesFor(cfg)
	resolvedExecutable, artifactPolicy, err := resolveManagerInputs(
		cfg,
		dependencies.executablePath,
		dependencies.resolveSymlinks,
	)
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
		getenv:         dependencies.getenv,
		now:            dependencies.now,
		httpClient:     dependencies.httpClient,
		runtimeOS:      strings.TrimSpace(cfg.RuntimeOS),
		runtimeArch:    strings.TrimSpace(cfg.RuntimeArch),
		lookPath:       dependencies.lookPath,
		runCommand:     dependencies.runCommand,
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

type managerDependencies struct {
	executablePath  func() (string, error)
	resolveSymlinks func(string) (string, error)
	getenv          func(string) string
	now             func() time.Time
	httpClient      *http.Client
	lookPath        func(string) (string, error)
	runCommand      func(context.Context, string, ...string) (string, error)
}

func managerDependenciesFor(cfg *Config) managerDependencies {
	dependencies := managerDependencies{
		executablePath:  cfg.ExecutablePath,
		resolveSymlinks: cfg.ResolveSymlinks,
		getenv:          cfg.Getenv,
		now:             cfg.Now,
		httpClient:      cfg.HTTPClient,
		lookPath:        cfg.LookPath,
		runCommand:      cfg.RunCommand,
	}
	if dependencies.executablePath == nil {
		dependencies.executablePath = os.Executable
	}
	if dependencies.resolveSymlinks == nil {
		dependencies.resolveSymlinks = filepath.EvalSymlinks
	}
	if dependencies.getenv == nil {
		dependencies.getenv = os.Getenv
	}
	if dependencies.now == nil {
		dependencies.now = func() time.Time { return time.Now().UTC() }
	}
	if dependencies.httpClient == nil {
		dependencies.httpClient = &http.Client{Timeout: defaultHTTPTimeout}
	}
	if dependencies.lookPath == nil {
		dependencies.lookPath = exec.LookPath
	}
	if dependencies.runCommand == nil {
		dependencies.runCommand = defaultRunCommand
	}
	return dependencies
}

func resolveManagerInputs(
	cfg *Config,
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
	return m.resolvePlanReleaseAssets(release, true)
}

func (m *Manager) resolvePlanReleaseAssets(release *Release, includeRuntime bool) (releaseAssets, error) {
	var archiveAsset ReleaseAsset
	if includeRuntime {
		archiveName, err := archiveAssetName(m.runtimeOS, m.runtimeArch)
		if err != nil {
			return releaseAssets{}, err
		}
		var ok bool
		archiveAsset, ok = release.findAsset(archiveName)
		if !ok {
			return releaseAssets{}, fmt.Errorf(
				"update: release %q does not publish %s",
				release.Version,
				archiveName,
			)
		}
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
		checksumsPath:     filepath.Join(tmpDir, assets.checksums.Name),
		bundlePath:        filepath.Join(tmpDir, assets.bundle.Name),
		compatibilityPath: filepath.Join(tmpDir, assets.compat.Name),
	}

	if assets.archive.Name != "" {
		downloaded.archivePath = filepath.Join(tmpDir, assets.archive.Name)
		if err := m.downloadFile(
			ctx,
			assets.archive.DownloadURL,
			downloaded.archivePath,
			m.artifactPolicy.MaxArchiveBytes,
		); err != nil {
			if downloadErr, ok := errors.AsType[*downloadSizeError](err); ok {
				return downloadedReleaseArtifacts{}, &ArtifactSizeError{
					Kind:  ArtifactKindArchive,
					Size:  downloadErr.Size,
					Limit: downloadErr.Limit,
				}
			}
			return downloadedReleaseArtifacts{}, err
		}
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

	if archiveName != "" {
		expectedChecksum, err := checksumForAsset(downloaded.checksumsPath, archiveName)
		if err != nil {
			return err
		}
		if err := verifySHA256(downloaded.archivePath, expectedChecksum); err != nil {
			return err
		}
	}
	expectedCompatibilityChecksum, err := checksumForAsset(downloaded.checksumsPath, compatibilityAssetName)
	if err != nil {
		return err
	}
	return verifySHA256(downloaded.compatibilityPath, expectedCompatibilityChecksum)
}
