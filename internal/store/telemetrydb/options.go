package telemetrydb

// OpenOption configures telemetry storage maintenance.
type OpenOption func(*openConfig)

type openConfig struct {
	cleanupBatchSize int64
}

// WithCleanupBatchSize overrides the 256-row default; nonpositive values are ignored.
func WithCleanupBatchSize(size int64) OpenOption {
	return func(config *openConfig) {
		if size > 0 {
			config.cleanupBatchSize = size
		}
	}
}
