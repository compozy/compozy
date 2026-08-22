package extensionpkg

import (
	"database/sql"
	"sync"
	"time"

	diagnosticcontract "github.com/compozy/compozy/internal/diagnosticcontract"
)

// Registry persists installed extension metadata in the global SQLite database.
type Registry struct {
	db  *sql.DB
	now func() time.Time

	bundledCacheMu     sync.RWMutex
	bundledCacheLoaded bool
	bundledCacheNames  []string
}

// ExtensionInfo is one persisted extension registry row.
type ExtensionInfo struct {
	Name                     string
	Version                  string
	Source                   ExtensionSource
	Enabled                  bool
	ManifestPath             string
	Format                   ExtensionFormat
	IngestDiagnostics        []diagnosticcontract.DiagnosticItem
	InstalledAt              time.Time
	Capabilities             CapabilitiesConfig
	Permissions              PermissionsConfig
	Checksum                 string
	lifecycleToken           string
	RegistrySlug             *string
	RegistryName             *string
	RemoteVersion            *string
	Provenance               ExtensionProvenance
	NetworkRequirementDigest string
	NetworkConfirmedBy       string
	NetworkConfirmedAt       time.Time
}

type installConfig struct {
	source           ExtensionSource
	enabled          bool
	replaceExisting  bool
	installedAt      *time.Time
	registrySlug     *string
	registryName     *string
	remoteVersion    *string
	provenance       *ExtensionProvenance
	boundaryObserver func(managedInstallBoundary) error
}

// InstallOption customizes one extension registry install operation.
type InstallOption func(*installConfig)

// WithInstallSource overrides the persisted source tier for one install.
func WithInstallSource(source ExtensionSource) InstallOption {
	return func(cfg *installConfig) {
		cfg.source = source
	}
}

// WithInstallEnabled controls the initial enabled state for a new registry row.
// Replace installs preserve the existing row's state.
func WithInstallEnabled(enabled bool) InstallOption {
	return func(cfg *installConfig) {
		cfg.enabled = enabled
	}
}

// WithInstallReplaceExisting allows an install to overwrite an existing row.
func WithInstallReplaceExisting() InstallOption {
	return func(cfg *installConfig) {
		cfg.replaceExisting = true
	}
}

func withInstallInstalledAt(installedAt time.Time) InstallOption {
	return func(cfg *installConfig) {
		value := installedAt.UTC()
		cfg.installedAt = &value
	}
}

// WithInstallRegistryMetadata records remote registry provenance for one install.
func WithInstallRegistryMetadata(slug string, registryName string, remoteVersion string) InstallOption {
	return func(cfg *installConfig) {
		cfg.registrySlug = optionalInstallString(slug)
		cfg.registryName = optionalInstallString(registryName)
		cfg.remoteVersion = optionalInstallString(remoteVersion)
	}
}

// WithInstallProvenance records the explicit source and trust evidence for one install.
func WithInstallProvenance(provenance ExtensionProvenance) InstallOption {
	return func(cfg *installConfig) {
		cfg.provenance = &provenance
	}
}

// ExtensionNotFoundError describes a missing extension registry row.
type ExtensionNotFoundError struct {
	Name string
}

// ExtensionExistsError describes a duplicate extension install attempt.
type ExtensionExistsError struct {
	Name string
}

// ExtensionChecksumMismatchError describes a checksum verification failure.
type ExtensionChecksumMismatchError struct {
	ExpectedChecksum string
	ActualChecksum   string
}
