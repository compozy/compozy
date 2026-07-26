package globaldb

import "github.com/compozy/agh/internal/store"

// OpenOption configures optional GlobalDB composition dependencies.
type OpenOption func(*openConfig)

type openConfig struct {
	openSessionEventMetadata store.SessionEventMetadataOpener
}

// WithSessionEventMetadataOpener supplies the per-session replay reader used by watch-events.
func WithSessionEventMetadataOpener(opener store.SessionEventMetadataOpener) OpenOption {
	return func(config *openConfig) {
		config.openSessionEventMetadata = opener
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
