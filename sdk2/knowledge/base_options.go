package knowledge

import engineknowledge "github.com/compozy/compozy/engine/knowledge"

// BaseOption is a functional option for configuring BaseConfig
type BaseOption func(*engineknowledge.BaseConfig)

// WithDescription sets the Description field
func WithDescription(description string) BaseOption {
	return func(cfg *engineknowledge.BaseConfig) {
		cfg.Description = description
	}
}

// WithEmbedder sets the Embedder field
func WithEmbedder(embedder string) BaseOption {
	return func(cfg *engineknowledge.BaseConfig) {
		cfg.Embedder = embedder
	}
}

// WithVectorDB sets the VectorDB field
func WithVectorDB(vectorDB string) BaseOption {
	return func(cfg *engineknowledge.BaseConfig) {
		cfg.VectorDB = vectorDB
	}
}

// WithIngest sets the Ingest field
func WithIngest(ingest engineknowledge.IngestMode) BaseOption {
	return func(cfg *engineknowledge.BaseConfig) {
		cfg.Ingest = ingest
	}
}

// WithSources sets the Sources field
func WithSources(sources []engineknowledge.SourceConfig) BaseOption {
	return func(cfg *engineknowledge.BaseConfig) {
		cfg.Sources = sources
	}
}

// WithChunking sets the Chunking field
func WithChunking(chunking engineknowledge.ChunkingConfig) BaseOption {
	return func(cfg *engineknowledge.BaseConfig) {
		cfg.Chunking = chunking
	}
}

// WithPreprocess sets the Preprocess field
func WithPreprocess(preprocess engineknowledge.PreprocessConfig) BaseOption {
	return func(cfg *engineknowledge.BaseConfig) {
		cfg.Preprocess = preprocess
	}
}

// WithRetrieval sets the Retrieval field
func WithRetrieval(retrieval *engineknowledge.RetrievalConfig) BaseOption {
	return func(cfg *engineknowledge.BaseConfig) {
		if retrieval != nil {
			cfg.Retrieval = *retrieval
		}
	}
}

// WithMetadata sets the Metadata field
func WithMetadata(metadata engineknowledge.MetadataConfig) BaseOption {
	return func(cfg *engineknowledge.BaseConfig) {
		cfg.Metadata = metadata
	}
}
