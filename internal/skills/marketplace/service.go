package marketplace

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	registrypkg "github.com/compozy/agh/internal/registry"
	"github.com/compozy/agh/internal/skills"
)

const (
	// DefaultRegistry is the built-in marketplace source used when config omits
	// an explicit registry.
	DefaultRegistry = "clawhub"
	// DefaultSearchLimit matches the CLI marketplace search default.
	DefaultSearchLimit = 20
	// SkillMarkdownFileName is the canonical skill manifest file name.
	SkillMarkdownFileName = "SKILL.md"
)

const (
	// UpdateStatusCurrent reports that an installed skill is already current.
	UpdateStatusCurrent = "already up to date"
	// UpdateStatusAvailable reports that an update exists but was not applied.
	UpdateStatusAvailable = "update available"
	// UpdateStatusUpdated reports that an update was applied.
	UpdateStatusUpdated = "updated"
)

var (
	// ErrValidation classifies malformed marketplace requests.
	ErrValidation = errors.New("skills marketplace: validation error")
	// ErrNotFound classifies missing installed or remote marketplace skills.
	ErrNotFound = errors.New("skills marketplace: not found")
	// ErrNotMarketplace classifies installed skills that lack marketplace provenance.
	ErrNotMarketplace = errors.New("skills marketplace: not marketplace")
	// ErrUnavailable classifies installs that are not visible through skill discovery.
	ErrUnavailable = errors.New("skills marketplace: unavailable")
	// ErrNotConfigured classifies missing or unsupported marketplace registry setup.
	ErrNotConfigured = errors.New("skills marketplace: not configured")

	validSkillNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	validSkillSlugPattern = regexp.MustCompile(`^@[^/\s]+/[^/\s]+$`)
)

// Error preserves a stable operator-facing message while allowing callers to
// map lifecycle failures with errors.Is.
type Error struct {
	Kind    error
	Message string
	Cause   error
}

func (e *Error) Error() string {
	message := strings.TrimSpace(e.Message)
	if e.Cause == nil {
		return message
	}
	if message == "" {
		return e.Cause.Error()
	}
	return fmt.Sprintf("%s: %v", message, e.Cause)
}

func (e *Error) Unwrap() error {
	return e.Cause
}

func (e *Error) Is(target error) bool {
	return target == e.Kind
}

// Registry is the marketplace registry contract required by skill lifecycle operations.
type Registry interface {
	registrypkg.Downloader
	Info(ctx context.Context, slug string) (*registrypkg.Detail, error)
	CheckUpdate(
		ctx context.Context,
		slug string,
		currentVersion string,
	) (*registrypkg.UpdateInfo, error)
}

// NamedRegistry can resolve an installed skill's recorded registry source.
type NamedRegistry interface {
	Registry
	SourceNamed(name string) registrypkg.Source
}

// SkillResolver resolves installed skills from the loaded AGH skill catalog.
type SkillResolver interface {
	Get(name string) (*skills.Skill, bool)
}

// SourceLoader resolves configured marketplace sources.
type SourceLoader func(aghconfig.MarketplaceConfig) ([]registrypkg.Source, error)

// Service exposes daemon-safe marketplace operations for skills.
type Service struct {
	homePaths    aghconfig.HomePaths
	skillsConfig aghconfig.SkillsConfig
	logger       *slog.Logger
	now          func() time.Time
	sourceLoader SourceLoader
}

// Option customizes a Service.
type Option func(*Service)

// WithLogger sets the logger used by the underlying multi-registry.
func WithLogger(logger *slog.Logger) Option {
	return func(service *Service) {
		service.logger = logger
	}
}

// WithNow sets the clock used for install provenance timestamps.
func WithNow(now func() time.Time) Option {
	return func(service *Service) {
		service.now = now
	}
}

// WithSourceLoader overrides marketplace source resolution, primarily for tests.
func WithSourceLoader(loader SourceLoader) Option {
	return func(service *Service) {
		service.sourceLoader = loader
	}
}

// NewService constructs a skill marketplace lifecycle service.
func NewService(
	homePaths aghconfig.HomePaths,
	skillsConfig aghconfig.SkillsConfig,
	opts ...Option,
) *Service {
	service := &Service{
		homePaths:    homePaths,
		skillsConfig: skillsConfig,
		logger:       slog.Default(),
		now:          time.Now,
		sourceLoader: DefaultSourceLoader,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	if service.logger == nil {
		service.logger = slog.Default()
	}
	if service.now == nil {
		service.now = time.Now
	}
	if service.sourceLoader == nil {
		service.sourceLoader = DefaultSourceLoader
	}
	return service
}

// InstalledSkill describes one locally installed marketplace-backed skill.
type InstalledSkill struct {
	Name       string
	Dir        string
	FilePath   string
	Provenance skills.Provenance
}

// InstallResult describes one completed marketplace install.
type InstallResult struct {
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Version  string `json:"version,omitempty"`
	Registry string `json:"registry"`
	Path     string `json:"path"`
	Hash     string `json:"hash"`
	Status   string `json:"status"`
}

// RemoveResult describes one removed marketplace skill.
type RemoveResult struct {
	Name   string `json:"name"`
	Slug   string `json:"slug"`
	Path   string `json:"path"`
	Status string `json:"status"`
}

// UpdateRequest describes a single-skill or batch marketplace update.
type UpdateRequest struct {
	Name      string
	All       bool
	CheckOnly bool
}

// UpdateResult describes one marketplace update outcome.
type UpdateResult struct {
	Name           string `json:"name"`
	Slug           string `json:"slug"`
	CurrentVersion string `json:"current_version,omitempty"`
	LatestVersion  string `json:"latest_version,omitempty"`
	Path           string `json:"path"`
	Status         string `json:"status"`
}

// SourceBackedRegistry adapts one configured source into the registry contract.
type SourceBackedRegistry struct {
	Source registrypkg.Source
}
