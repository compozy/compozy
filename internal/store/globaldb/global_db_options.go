package globaldb

import (
	"strings"

	"github.com/compozy/compozy/internal/store"
)

// OpenOption configures optional GlobalDB composition dependencies.
type OpenOption func(*openConfig)

type openConfig struct {
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

func newOpenConfig(options []OpenOption) openConfig {
	var config openConfig
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	return config
}
