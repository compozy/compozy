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
	openSessionEventMetadata store.SessionEventMetadataOpener
	operatorHomeDir          string
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

// WithHealthFlushInterval controls health write-behind latency; nonpositive values use the default.
func WithHealthFlushInterval(interval time.Duration) OpenOption {
	return func(config *openConfig) {
		if interval > 0 {
			config.healthFlushInterval = interval
		}
	}
}

func newOpenConfig(options []OpenOption) openConfig {
	config := openConfig{healthFlushInterval: 500 * time.Millisecond}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	return config
}
