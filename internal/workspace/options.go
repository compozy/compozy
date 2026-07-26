package workspace

import (
	"context"
	"errors"
	"log/slog"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
)

const defaultCacheTTL = 10 * time.Minute

// ConfigLoader loads the effective configuration for a workspace root.
type ConfigLoader func(rootDir string) (aghconfig.Config, error)

// ChangeHook runs after persisted workspace mutations that affect resolved runtime state.
type ChangeHook func(context.Context) error

// Option customizes a Resolver instance.
type Option func(*resolverOptions)

type resolverOptions struct {
	homePaths   aghconfig.HomePaths
	loadConfig  ConfigLoader
	logger      *slog.Logger
	now         func() time.Time
	cacheTTL    time.Duration
	idGenerator func(prefix string) string
	changeHook  ChangeHook
}

// WithHomePaths overrides the global AGH home layout used for agent and skill discovery.
func WithHomePaths(homePaths aghconfig.HomePaths) Option {
	return func(opts *resolverOptions) {
		opts.homePaths = homePaths
	}
}

// WithConfigLoader overrides the configuration loader used during workspace resolution.
func WithConfigLoader(loader ConfigLoader) Option {
	return func(opts *resolverOptions) {
		opts.loadConfig = loader
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

// WithIDGenerator overrides workspace ID generation.
func WithIDGenerator(generator func(prefix string) string) Option {
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
	homePaths, err := aghconfig.ResolveHomePaths()
	if err != nil {
		return resolverOptions{}, err
	}

	resolved := resolverOptions{
		homePaths: homePaths,
		loadConfig: func(rootDir string) (aghconfig.Config, error) {
			return aghconfig.Load(aghconfig.WithWorkspaceRoot(rootDir))
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
