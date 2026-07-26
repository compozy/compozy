package registry

import (
	"errors"

	"io"

	"os"

	"regexp"

	"time"
)

const (
	installerApplicationGzipPath = "application/gzip"
)

const (
	// DefaultMaxArchiveSize caps the compressed archive stream before extraction.
	DefaultMaxArchiveSize int64 = 50 * 1024 * 1024

	defaultInstallerTempDirPattern = ".agh-install-*"
	defaultInstallerTempDirMaxAge  = time.Hour
)

const (
	installerSkillManifestName     = "SKILL.md"
	installerExtensionManifestName = "extension.toml"
)

var (
	// ErrArchiveTooLargeCompressed reports a compressed archive stream above its configured limit.
	ErrArchiveTooLargeCompressed = errors.New("registry: archive exceeds max compressed size")

	errArchiveTooLargeCompressed = ErrArchiveTooLargeCompressed
	errInstallMissingManifest    = errors.New("registry: archive missing extension.toml or SKILL.md at root")
	errUnexpectedContentType     = errors.New("registry: unexpected download content type")
	errVerificationBlocked       = errors.New("registry: install blocked by content verification")
)

type installerVerificationRule struct {
	key     string
	regex   *regexp.Regexp
	message string
}

var installerVerificationRules = []installerVerificationRule{
	{
		key: "ignore-previous-instructions",
		regex: regexp.MustCompile(
			`(?i)\bignore\s+(?:\w+\s+)*(?:all|previous|prior|above)\s+(?:\w+\s+)*(?:instructions|rules|guidelines)\b`,
		),
		message: "content attempts to override existing instructions",
	},
	{
		key: "disregard-existing-rules",
		regex: regexp.MustCompile(
			`(?i)\bdisregard\s+(?:\w+\s+)*(?:all|previous|prior|your)\s+(?:\w+\s+)*(?:instructions|rules|guidelines)\b`,
		),
		message: "content attempts to bypass current rules",
	},
	{
		key: "forget-your-instructions",
		regex: regexp.MustCompile(
			`(?i)\bforget\s+(?:\w+\s+)*(?:your|all)\s+(?:\w+\s+)*(?:instructions|rules|guidelines)\b`,
		),
		message: "content attempts to erase active instructions",
	},
	{
		key:     "role-hijack-you-are-now",
		regex:   regexp.MustCompile(`(?i)\byou\s+are\s+now\s+(?:a|an|the|assistant|agent|bot|system)\b`),
		message: "content attempts to redefine the agent role",
	},
	{
		key:     "new-instructions",
		regex:   regexp.MustCompile(`(?i)\bnew\s+instructions\s*:`),
		message: "content introduces overriding instructions",
	},
	{
		key:     "system-prompt-override",
		regex:   regexp.MustCompile(`(?i)\bsystem\s+prompt\s+override\b`),
		message: "content attempts to override the system prompt",
	},
	{
		key:     "delete-all-files",
		regex:   regexp.MustCompile(`(?i)\bdelete\s+all\s+files\b`),
		message: "content instructs destructive file deletion",
	},
	{
		key:     "rm-rf",
		regex:   regexp.MustCompile(`(?i)\brm\s+-rf\b`),
		message: "content includes a destructive shell command",
	},
	{
		key: "credential-extraction",
		regex: regexp.MustCompile(
			`(?i)\b(?:print|show|reveal|display|output)\s+(?:the\s+|your\s+)?(?:api\s+key|access\s+token|credentials?|secret(?:s)?|password(?:s)?)\b`,
		),
		message: "content attempts to extract credentials",
	},
}

// InstallerOption customizes the installer pipeline.
type InstallerOption func(*Installer)

// Installer handles download, extraction, validation, verification, and the
// final atomic move into place.
type Installer struct {
	downloader          Downloader
	maxArchiveSize      int64
	maxDecompressedSize int64
	maxFileCount        int
	now                 func() time.Time
	removeAll           func(string) error
	tempDirMaxAge       time.Duration
}

type countingReader struct {
	reader io.Reader
	total  int64
}

type installedPackageMetadata struct {
	name          string
	version       string
	manifestPath  string
	verifyContent string
}

type skillManifestHeader struct {
	Name        string         `yaml:"name"`
	Description string         `yaml:"description,omitempty"`
	Version     string         `yaml:"version,omitempty"`
	Metadata    map[string]any `yaml:"metadata,omitempty"`
}

type extensionManifestHeader struct {
	Extension struct {
		Name    string `toml:"name"`
		Version string `toml:"version"`
	} `toml:"extension"`
	Name    string `toml:"name"`
	Version string `toml:"version"`
}

// NewInstaller constructs a new domain-agnostic install pipeline.
func NewInstaller(dl Downloader, opts ...InstallerOption) *Installer {
	installer := &Installer{
		downloader:          dl,
		maxArchiveSize:      DefaultMaxArchiveSize,
		maxDecompressedSize: DefaultMaxDecompressedSize,
		maxFileCount:        DefaultMaxFileCount,
		now:                 time.Now,
		removeAll:           os.RemoveAll,
		tempDirMaxAge:       defaultInstallerTempDirMaxAge,
	}

	for _, opt := range opts {
		if opt != nil {
			opt(installer)
		}
	}

	if installer.maxArchiveSize <= 0 {
		installer.maxArchiveSize = DefaultMaxArchiveSize
	}
	if installer.maxDecompressedSize <= 0 {
		installer.maxDecompressedSize = DefaultMaxDecompressedSize
	}
	if installer.maxFileCount <= 0 {
		installer.maxFileCount = DefaultMaxFileCount
	}
	if installer.now == nil {
		installer.now = time.Now
	}
	if installer.removeAll == nil {
		installer.removeAll = os.RemoveAll
	}
	if installer.tempDirMaxAge <= 0 {
		installer.tempDirMaxAge = defaultInstallerTempDirMaxAge
	}

	return installer
}

// WithInstallerMaxArchiveSize overrides the compressed archive limit.
func WithInstallerMaxArchiveSize(size int64) InstallerOption {
	return func(installer *Installer) {
		installer.maxArchiveSize = size
	}
}

// WithInstallerMaxDecompressedSize overrides the extracted payload limit.
func WithInstallerMaxDecompressedSize(size int64) InstallerOption {
	return func(installer *Installer) {
		installer.maxDecompressedSize = size
	}
}

// WithInstallerMaxFileCount overrides the extracted file-count limit.
func WithInstallerMaxFileCount(count int) InstallerOption {
	return func(installer *Installer) {
		installer.maxFileCount = count
	}
}

// WithInstallerNow overrides the clock used for stale-temp cleanup.
func WithInstallerNow(now func() time.Time) InstallerOption {
	return func(installer *Installer) {
		installer.now = now
	}
}

// WithInstallerTempDirMaxAge overrides the stale-temp cleanup threshold.
func WithInstallerTempDirMaxAge(age time.Duration) InstallerOption {
	return func(installer *Installer) {
		installer.tempDirMaxAge = age
	}
}
