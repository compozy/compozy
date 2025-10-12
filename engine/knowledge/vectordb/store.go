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
	errInvalidDimension = errors.New("vector_db dimension must be greater than zero")
)

// New instantiates a vector store backed by the requested provider.
func New(ctx context.Context, cfg *Config) (Store, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	switch cfg.Provider {
	case ProviderPGVector:
		return newPGStore(ctx, cfg)
	case ProviderQdrant:
		return newQdrantStore(ctx, cfg)
	case ProviderMemory:
		return newMemoryStore(cfg), nil
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
	if cfg.Provider != ProviderMemory && strings.TrimSpace(cfg.DSN) == "" {
		return fmt.Errorf("vector_db %q: %w", cfg.ID, errMissingDSN)
	}
	if cfg.Dimension <= 0 {
		return fmt.Errorf("vector_db %q: %w", cfg.ID, errInvalidDimension)
	}
	return nil
}
