package knowledge

import engineknowledge "github.com/compozy/compozy/engine/knowledge"

// VectorDBOption is a functional option for configuring VectorDBConfig
type VectorDBOption func(*engineknowledge.VectorDBConfig)

// WithVectorDBConfig sets the Config field
func WithVectorDBConfig(config *engineknowledge.VectorDBConnConfig) VectorDBOption {
	return func(cfg *engineknowledge.VectorDBConfig) {
		if config != nil {
			cfg.Config = *config
		}
	}
}

// WithDSN sets the DSN in the vector DB config
func WithDSN(dsn string) VectorDBOption {
	return func(cfg *engineknowledge.VectorDBConfig) {
		cfg.Config.DSN = dsn
	}
}

// WithPath sets the Path in the vector DB config
func WithPath(path string) VectorDBOption {
	return func(cfg *engineknowledge.VectorDBConfig) {
		cfg.Config.Path = path
	}
}

// WithCollection sets the Collection in the vector DB config
func WithCollection(collection string) VectorDBOption {
	return func(cfg *engineknowledge.VectorDBConfig) {
		cfg.Config.Collection = collection
	}
}

// WithDimension sets the Dimension in the vector DB config
func WithVectorDBDimension(dimension int) VectorDBOption {
	return func(cfg *engineknowledge.VectorDBConfig) {
		cfg.Config.Dimension = dimension
	}
}

// WithPGVector sets the PGVector config
func WithPGVector(pgVector *engineknowledge.PGVectorConfig) VectorDBOption {
	return func(cfg *engineknowledge.VectorDBConfig) {
		if pgVector != nil {
			cfg.Config.PGVector = pgVector
		}
	}
}
