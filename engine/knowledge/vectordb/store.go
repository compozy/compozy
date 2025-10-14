package vectordb

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var (
	errMissingID        = errors.New("vector_db id is required")
	errMissingProvider  = errors.New("vector_db provider is required")
	errMissingDSN       = errors.New("vector_db dsn is required")
	errMissingPath      = errors.New("vector_db path is required")
	errInvalidDimension = errors.New("vector_db dimension must be greater than zero")
)

// New instantiates a vector store backed by the requested provider.
func New(ctx context.Context, cfg *Config) (Store, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	return instantiateStore(ctx, cfg)
}

func instantiateStore(ctx context.Context, cfg *Config) (Store, error) {
	switch cfg.Provider {
	case ProviderPGVector:
		return newPGStore(ctx, cfg)
	case ProviderQdrant:
		return newQdrantStore(ctx, cfg)
	case ProviderRedis:
		return newRedisStore(ctx, cfg)
	case ProviderFilesystem:
		return newFileStore(cfg)
	default:
		return nil, fmt.Errorf("vector_db %q: provider %q is not supported", cfg.ID, cfg.Provider)
	}
}

func validateConfig(cfg *Config) error {
	if cfg == nil {
		return errors.New("vector_db config is required")
	}
	if strings.TrimSpace(cfg.ID) == "" {
		return errMissingID
	}
	if strings.TrimSpace(string(cfg.Provider)) == "" {
		return fmt.Errorf("vector_db %q: %w", cfg.ID, errMissingProvider)
	}
	dsn := strings.TrimSpace(cfg.DSN)
	path := strings.TrimSpace(cfg.Path)
	if dsn != cfg.DSN {
		cfg.DSN = dsn
	}
	if path != cfg.Path {
		cfg.Path = path
	}
	switch cfg.Provider {
	case ProviderPGVector, ProviderQdrant:
		if dsn == "" {
			return fmt.Errorf("vector_db %q: %w", cfg.ID, errMissingDSN)
		}
	case ProviderFilesystem:
		if path == "" {
			return fmt.Errorf("vector_db %q: %w", cfg.ID, errMissingPath)
		}
	}
	if cfg.Dimension <= 0 {
		return fmt.Errorf("vector_db %q: %w", cfg.ID, errInvalidDimension)
	}
	if cfg.MaxTopK < 0 {
		return fmt.Errorf("vector_db %q: max_top_k must be non-negative", cfg.ID)
	}
	return nil
}
