package workspace

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

const defaultCacheTTL = 10 * time.Minute

// ConfigLoader loads the effective configuration for a workspace root.
type ConfigLoader func(rootDir string) (compozyconfig.Config, error)

// ProfileConfigLoader loads the five-layer effective config for one active profile.
type ProfileConfigLoader func(rootDir string, profileName string) (compozyconfig.Config, error)

// ProfileAvailabilityChecker rejects profile-owned resource discovery while a
// lifecycle operation has exclusive ownership of that profile.
type ProfileAvailabilityChecker interface {
	EnsureAvailableName(context.Context, string) error
	AvailableProfileID(context.Context, string) (string, error)
}

// ChangeHook runs after persisted workspace mutations that affect resolved runtime state.
type ChangeHook func(context.Context) error

// Option customizes a Resolver instance.
type Option func(*resolverOptions)

type idGenerator func(prefix string) (string, error)

type resolverOptions struct {
	homePaths           compozyconfig.HomePaths
	loadConfig          ConfigLoader
	loadProfileConfig   ProfileConfigLoader
	profileAvailability ProfileAvailabilityChecker
	logger              *slog.Logger
	now                 func() time.Time
	cacheTTL            time.Duration
	idGenerator         idGenerator
	changeHook          ChangeHook
	operatorHomeDir     string
}

// WithHomePaths overrides the global Compozy home layout used for agent and skill discovery.
func WithHomePaths(homePaths compozyconfig.HomePaths) Option {
	return func(opts *resolverOptions) {
		opts.homePaths = homePaths
	}
}

// WithOperatorHomeDir overrides the canonical operator home rejected by workspace registration.
func WithOperatorHomeDir(operatorHomeDir string) Option {
	return func(opts *resolverOptions) {
		opts.operatorHomeDir = strings.TrimSpace(operatorHomeDir)
	}
}

// WithConfigLoader overrides the configuration loader used during workspace resolution.
func WithConfigLoader(loader ConfigLoader) Option {
	return func(opts *resolverOptions) {
		opts.loadConfig = loader
	}
}

// WithProfileConfigLoader overrides profile-aware config loading during workspace resolution.
func WithProfileConfigLoader(loader ProfileConfigLoader) Option {
	return func(opts *resolverOptions) {
		opts.loadProfileConfig = loader
	}
}

// WithProfileAvailabilityChecker installs the lifecycle availability gate used
// before profile-aware discovery reads any profile-owned resource roots.
func WithProfileAvailabilityChecker(checker ProfileAvailabilityChecker) Option {
	return func(opts *resolverOptions) {
		opts.profileAvailability = checker
	}
}

// WithLogger overrides the structured logger used for resolver diagnostics.
func WithLogger(logger *slog.Logger) Option {
	return func(opts *resolverOptions) {
		opts.logger = logger
	}
}

func withNow(now func() time.Time) Option {
	return func(opts *resolverOptions) {
		opts.now = now
	}
}

// WithCacheTTL overrides the idle cache eviction window.
func WithCacheTTL(ttl time.Duration) Option {
	return func(opts *resolverOptions) {
		opts.cacheTTL = ttl
	}
}

// WithIDGenerator overrides workspace ID generation; errors abort registration.
func WithIDGenerator(generator idGenerator) Option {
	return func(opts *resolverOptions) {
		opts.idGenerator = generator
	}
}

// WithChangeHook installs a post-mutation hook for derived runtime projections.
func WithChangeHook(hook ChangeHook) Option {
	return func(opts *resolverOptions) {
		opts.changeHook = hook
	}
}

func resolveOptions(opts []Option) (resolverOptions, error) {
	homePaths, err := compozyconfig.ResolveHomePaths()
	if err != nil {
		return resolverOptions{}, err
	}

	resolved := resolverOptions{
		homePaths: homePaths,
		loadConfig: func(rootDir string) (compozyconfig.Config, error) {
			return compozyconfig.Load(compozyconfig.WithWorkspaceRoot(rootDir))
		},
		loadProfileConfig: func(rootDir string, profileName string) (compozyconfig.Config, error) {
			return compozyconfig.Load(
				compozyconfig.WithWorkspaceRoot(rootDir),
				compozyconfig.WithProfile(profileName),
			)
		},
		logger:      slog.Default(),
		now:         time.Now,
		cacheTTL:    defaultCacheTTL,
		idGenerator: generateID,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&resolved)
		}
	}
	if strings.TrimSpace(resolved.operatorHomeDir) == "" {
		operatorHomeDir, err := compozyconfig.ResolveOperatorHomeDir(resolved.homePaths)
		if err != nil {
			return resolverOptions{}, fmt.Errorf("workspace: resolve operator home: %w", err)
		}
		resolved.operatorHomeDir = operatorHomeDir
	}

	if resolved.logger == nil {
		resolved.logger = slog.Default()
	}
	if resolved.now == nil {
		resolved.now = time.Now
	}
	if resolved.idGenerator == nil {
		resolved.idGenerator = generateID
	}

	if resolved.loadConfig == nil {
		return resolverOptions{}, errors.New("workspace: config loader is required")
	}
	if resolved.cacheTTL <= 0 {
		resolved.cacheTTL = defaultCacheTTL
	}

	return resolved, nil
}
