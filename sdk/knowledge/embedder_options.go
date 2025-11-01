package knowledge

import engineknowledge "github.com/compozy/compozy/engine/knowledge"

// EmbedderOption is a functional option for configuring EmbedderConfig
type EmbedderOption func(*engineknowledge.EmbedderConfig)

// WithAPIKey sets the APIKey field
func WithAPIKey(apiKey string) EmbedderOption {
	return func(cfg *engineknowledge.EmbedderConfig) {
		cfg.APIKey = apiKey
	}
}

// WithEmbedderConfig sets the Config field
func WithEmbedderConfig(config engineknowledge.EmbedderRuntimeConfig) EmbedderOption {
	return func(cfg *engineknowledge.EmbedderConfig) {
		cfg.Config = config
	}
}

// WithDimension sets the dimension in the embedder config
func WithDimension(dimension int) EmbedderOption {
	return func(cfg *engineknowledge.EmbedderConfig) {
		cfg.Config.Dimension = dimension
	}
}

// WithBatchSize sets the batch size in the embedder config
func WithBatchSize(batchSize int) EmbedderOption {
	return func(cfg *engineknowledge.EmbedderConfig) {
		cfg.Config.BatchSize = batchSize
	}
}

// WithMaxConcurrentWorkers sets the max concurrent workers in the embedder config
func WithMaxConcurrentWorkers(maxWorkers int) EmbedderOption {
	return func(cfg *engineknowledge.EmbedderConfig) {
		cfg.Config.MaxConcurrentWorkers = maxWorkers
	}
}
