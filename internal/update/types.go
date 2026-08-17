package update

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

const (
	DefaultOperationDeadline = 30 * time.Minute
	DefaultLeaseDuration     = 2 * time.Minute
)

const (
	githubLatestReleaseAPIURL  = "https://api.github.com/repos/compozy/compozy/releases/latest"
	githubReleasesAPIURL       = "https://api.github.com/repos/compozy/compozy/releases?per_page=100"
	githubRepositorySlug       = "compozy/compozy"
	goInstallModulePath        = "github.com/compozy/compozy"
	checksumsAssetName         = "checksums.txt"
	checksumsBundleAssetName   = "checksums.txt.sigstore.json"
	compatibilityAssetName     = "compat.json"
	sigstoreOIDCIssuer         = "https://token.actions.githubusercontent.com"
	releaseWorkflowIdentityExp = `^https://github\.com/compozy/compozy/\.github/workflows/release\.yml@refs/heads/main$`
)

const (
	compozyBinaryName        = "compozy"
	compozyWindowsBinaryName = "compozy.exe"
	managedPathUsrBin        = "/usr/bin/compozy"
	managedPathBin           = "/bin/compozy"
	managedPathUsrLocalBin   = "/usr/local/bin/compozy"
)

const (
	runtimeOSLinux   = "linux"
	runtimeOSDarwin  = "darwin"
	runtimeOSWindows = "windows"
	runtimeArchAMD64 = "amd64"
	runtimeArchARM64 = "arm64"
)

const (
	cacheTTL               = 24 * time.Hour
	defaultHTTPTimeout     = 30 * time.Second
	maxChecksumsBytes      = int64(1 << 20)
	maxSigstoreBundleBytes = int64(8 << 20)
	maxCompatibilityBytes  = int64(64 << 10)
)

var ErrNoCachedRelease = errors.New("update: cached release info not found")

// ManagedEnvName overrides the install-method detector for managed package installs.
const ManagedEnvName = "COMPOZY_MANAGED"

// Status reports the operator-facing update state.
type Status string

const (
	StatusUpToDate    Status = "up-to-date"
	StatusAvailable   Status = "available"
	StatusAccepted    Status = "accepted"
	StatusApplying    Status = "applying"
	StatusStaged      Status = "staged"
	StatusUpdated     Status = "updated"
	StatusFailed      Status = "failed"
	StatusBlocked     Status = "blocked"
	StatusUnsupported Status = "unsupported"
	StatusCanceled    Status = "canceled"
)

// InstallMethod reports how the running CompozyOS binary was installed.
type InstallMethod string

const (
	InstallMethodDirectBinary InstallMethod = "direct-binary"
	InstallMethodHomebrew     InstallMethod = "homebrew"
	InstallMethodNPM          InstallMethod = "npm"
	InstallMethodAPT          InstallMethod = "apt"
	InstallMethodDNF          InstallMethod = "dnf"
	InstallMethodRPM          InstallMethod = "rpm"
	InstallMethodScoop        InstallMethod = "scoop"
	InstallMethodGoInstall    InstallMethod = "go-install"
	InstallMethodDesktopApp   InstallMethod = "desktop-app"
	InstallMethodUnknown      InstallMethod = "unknown"
)

// ReleaseAsset identifies one downloadable release artifact.
type ReleaseAsset struct {
	Name        string
	DownloadURL string
}

// Release holds the metadata required to inspect or apply one release.
type Release struct {
	Version     string
	ReleaseURL  string
	PublishedAt time.Time
	Assets      []ReleaseAsset
}

// State is the transport-safe update status snapshot shared by CLI and API surfaces.
type State struct {
	Supported       bool       `json:"supported"`
	Managed         bool       `json:"managed"`
	InstallMethod   string     `json:"install_method"`
	CurrentVersion  string     `json:"current_version"`
	LatestVersion   string     `json:"latest_version,omitempty"`
	Available       bool       `json:"available"`
	Status          Status     `json:"status"`
	Recommendation  string     `json:"recommendation,omitempty"`
	ReleaseURL      string     `json:"release_url,omitempty"`
	CheckedAt       *time.Time `json:"checked_at,omitempty"`
	LastError       string     `json:"last_error,omitempty"`
	Message         string     `json:"message,omitempty"`
	RestoredVersion string     `json:"restored_version,omitempty"`
	DaemonRestarted bool       `json:"daemon_restarted"`
}

// AppliedBinary describes one on-disk binary swap that still retains a rollback backup.
type AppliedBinary struct {
	TargetPath      string
	BackupPath      string
	Version         string
	PreviousVersion string
	InstallMethod   InstallMethod
}

// CheckOptions customize one update status query.
type CheckOptions struct {
	ForceRefresh         bool
	AllowCachedOnFailure bool
}

// BundleVerifier verifies the signed checksum catalog before the archive checksum is trusted.
type BundleVerifier interface {
	VerifyChecksums(ctx context.Context, checksumsPath string, bundlePath string) error
}

// BinaryApplier atomically swaps the current executable with a verified replacement.
type BinaryApplier interface {
	ApplyBinary(sourcePath string, targetPath string, backupPath string, mode os.FileMode) error
	RestoreBinary(backupPath string, targetPath string, mode os.FileMode) error
}

// Config builds one update manager bound to the current runtime.
type Config struct {
	HomePaths       compozyconfig.HomePaths
	CurrentVersion  string
	ExecutablePath  func() (string, error)
	ResolveSymlinks func(string) (string, error)
	Getenv          func(string) string
	Now             func() time.Time
	HTTPClient      *http.Client
	RuntimeOS       string
	RuntimeArch     string
	LookPath        func(string) (string, error)
	RunCommand      func(context.Context, string, ...string) (string, error)
	BundleVerifier  BundleVerifier
	BinaryApplier   BinaryApplier
	ArtifactPolicy  ArtifactPolicy
	OperationEvents OperationEventEmitter
}

type cacheEntry struct {
	LatestVersion string         `json:"latest_version"`
	ReleaseURL    string         `json:"release_url"`
	PublishedAt   time.Time      `json:"published_at"`
	Assets        []ReleaseAsset `json:"assets"`
	CheckedAt     time.Time      `json:"checked_at"`
}

type installInfo struct {
	Method  string
	Managed bool
}

// Manager owns the CompozyOS self-update flow for the current runtime.
type Manager struct {
	homePaths      compozyconfig.HomePaths
	currentVersion string
	executablePath string
	getenv         func(string) string
	now            func() time.Time
	httpClient     *http.Client
	runtimeOS      string
	runtimeArch    string
	lookPath       func(string) (string, error)
	runCommand     func(context.Context, string, ...string) (string, error)
	bundleVerifier BundleVerifier
	binaryApplier  BinaryApplier
	artifactPolicy ArtifactPolicy
	releaseTrack   releaseTrack
	installMu      sync.Mutex
	installFlight  chan struct{}
	install        *installInfo
	operationStore *OperationStore
}

func (m *Manager) cachePath() string {
	return filepath.Join(m.homePaths.HomeDir, "cache", "update-state-"+string(m.releaseTrack)+".json")
}

func (m *Manager) sigstoreCachePath() string {
	return filepath.Join(m.homePaths.HomeDir, "cache", "sigstore-tuf")
}
