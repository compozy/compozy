package globaldb

import (
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
)

// OpenOption configures optional GlobalDB composition dependencies.
type OpenOption func(*openConfig)

type openConfig struct {
	healthFlushInterval      time.Duration
	readStatementCacheLimit  int
	openSessionEventMetadata store.SessionEventMetadataOpener
	operatorHomeDir          string
	migrationProgress        MigrationProgressFunc
	now                      func() time.Time
}

// WithSessionEventMetadataOpener supplies the per-session replay reader used by watch-events.
func WithSessionEventMetadataOpener(opener store.SessionEventMetadataOpener) OpenOption {
	return func(config *openConfig) {
		config.openSessionEventMetadata = opener
	}
}

// WithOperatorHomeDir supplies the canonical operator home to workspace-removal migrations.
func WithOperatorHomeDir(operatorHomeDir string) OpenOption {
	return func(config *openConfig) {
		config.operatorHomeDir = strings.TrimSpace(operatorHomeDir)
	}
}

// WithMigrationProgress observes data-volume-dependent migrations while the database opens.
func WithMigrationProgress(report MigrationProgressFunc) OpenOption {
	return func(config *openConfig) {
		config.migrationProgress = report
	}
}

// WithHealthFlushInterval controls health write-behind latency; nonpositive values use the default.
func WithHealthFlushInterval(interval time.Duration) OpenOption {
	return func(config *openConfig) {
		if interval > 0 {
			config.healthFlushInterval = interval
		}
	}
}

// WithReadStatementCacheLimit bounds the dynamic read shapes kept prepared; zero disables reuse of
// both dynamic and static reads.
func WithReadStatementCacheLimit(limit int) OpenOption {
	return func(config *openConfig) {
		if limit >= 0 {
			config.readStatementCacheLimit = limit
		}
	}
}

// WithClock replaces the wall clock repositories stamp records with. Fixture writers such as
// the demo seed use it to place records on a reproducible timeline; the daemon keeps the default.
func WithClock(now func() time.Time) OpenOption {
	return func(config *openConfig) {
		config.now = now
	}
}

// staticReadStatementCacheLimit bounds sqlc's static read statements, which come from a fixed
// set of generated queries; disabling dynamic reuse disables static reuse too.
func (c openConfig) staticReadStatementCacheLimit() int {
	if c.readStatementCacheLimit == 0 {
		return 0
	}
	return defaultStaticReadStatementCacheLimit
}

func newOpenConfig(options []OpenOption) openConfig {
	config := openConfig{
		healthFlushInterval:     500 * time.Millisecond,
		readStatementCacheLimit: defaultReadStatementCacheLimit,
	}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	return config
}
