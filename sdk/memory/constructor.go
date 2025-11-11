package memory

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	enginememory "github.com/compozy/compozy/engine/memory"
	memorycore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/v2/internal/errors"
	"github.com/compozy/compozy/sdk/v2/internal/validate"
)

var supportedTypes = map[string]memorycore.Type{
	"token_based":         memorycore.TokenBasedMemory,
	"message_count_based": memorycore.MessageCountBasedMemory,
	"buffer":              memorycore.BufferMemory,
}

var typeList = []string{
	"token_based",
	"message_count_based",
	"buffer",
}

var supportedPersistence = map[string]memorycore.PersistenceType{
	"redis":     memorycore.RedisPersistence,
	"in_memory": memorycore.InMemoryPersistence,
}

var persistenceList = []string{
	"redis",
	"in_memory",
}

// New creates a memory configuration using functional options
func New(ctx context.Context, id string, memType string, opts ...Option) (*enginememory.Config, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	log := logger.FromContext(ctx)
	log.Debug("creating memory configuration", "id", id, "type", memType)
	normalizedType := strings.ToLower(strings.TrimSpace(memType))
	typeVal, ok := supportedTypes[normalizedType]
	if !ok {
		return nil, fmt.Errorf(
			"type %q is not supported; must be one of %s",
			normalizedType,
			strings.Join(typeList, ", "),
		)
	}
	cfg := &enginememory.Config{
		Resource: string(core.ConfigMemory),
		ID:       strings.TrimSpace(id),
		Type:     typeVal,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	collected := make([]error, 0, 8)
	if err := validate.ID(ctx, cfg.ID); err != nil {
		collected = append(collected, fmt.Errorf("id is invalid: %w", err))
	}
	if err := validateResource(ctx, cfg); err != nil {
		collected = append(collected, err)
	}
	if err := validateType(ctx, cfg); err != nil {
		collected = append(collected, err)
	}
	if err := validatePersistence(ctx, cfg); err != nil {
		collected = append(collected, err)
	}
	if errs := validateLimits(cfg); len(errs) > 0 {
		collected = append(collected, errs...)
	}
	if len(collected) > 0 {
		return nil, &sdkerrors.BuildError{Errors: collected}
	}
	cloned, err := core.DeepCopy(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to clone memory config: %w", err)
	}
	return cloned, nil
}

func validateResource(_ context.Context, cfg *enginememory.Config) error {
	if cfg.Resource != string(core.ConfigMemory) {
		return fmt.Errorf("resource field must be %q, got %q", core.ConfigMemory, cfg.Resource)
	}
	return nil
}

func validateType(_ context.Context, cfg *enginememory.Config) error {
	typeStr := strings.ToLower(string(cfg.Type))
	if _, ok := supportedTypes[typeStr]; !ok {
		return fmt.Errorf("type %q is not supported; must be one of %s", cfg.Type, strings.Join(typeList, ", "))
	}
	return nil
}

func validatePersistence(_ context.Context, cfg *enginememory.Config) error {
	persistenceType := strings.ToLower(string(cfg.Persistence.Type))
	if persistenceType == "" {
		return fmt.Errorf("persistence.type is required")
	}
	mapped, ok := supportedPersistence[persistenceType]
	if !ok {
		return fmt.Errorf(
			"persistence.type %q is not supported; must be one of %s",
			persistenceType,
			strings.Join(persistenceList, ", "),
		)
	}
	cfg.Persistence.Type = mapped
	if cfg.Persistence.Type != memorycore.InMemoryPersistence && cfg.Persistence.TTL == "" {
		return fmt.Errorf("persistence.ttl is required for persistence type %q", cfg.Persistence.Type)
	}
	if cfg.Persistence.TTL != "" {
		parsedTTL, err := core.ParseHumanDuration(cfg.Persistence.TTL)
		if err != nil {
			return fmt.Errorf("invalid persistence.ttl duration format %q: %w", cfg.Persistence.TTL, err)
		}
		if parsedTTL < 0 {
			return fmt.Errorf("persistence.ttl must be non-negative, got %q", cfg.Persistence.TTL)
		}
	}
	return nil
}

func validateLimits(cfg *enginememory.Config) []error {
	errs := make([]error, 0, 5)
	if cfg.MaxTokens < 0 {
		errs = append(errs, fmt.Errorf("max_tokens must be non-negative: got %d", cfg.MaxTokens))
	}
	if cfg.MaxMessages < 0 {
		errs = append(errs, fmt.Errorf("max_messages must be non-negative: got %d", cfg.MaxMessages))
	}
	if cfg.MaxContextRatio < 0 || cfg.MaxContextRatio > 1 {
		errs = append(
			errs,
			fmt.Errorf("max_context_ratio must be between 0 and 1 inclusive: got %v", cfg.MaxContextRatio),
		)
	}
	if cfg.Type == memorycore.TokenBasedMemory {
		if cfg.MaxTokens <= 0 && cfg.MaxContextRatio <= 0 && cfg.MaxMessages <= 0 {
			errs = append(
				errs,
				fmt.Errorf(
					"token_based memory must have at least one limit configured (max_tokens, max_context_ratio, or max_messages)",
				),
			)
		}
		if cfg.MaxContextRatio > 0 && cfg.TokenProvider == nil {
			errs = append(errs, fmt.Errorf("max_context_ratio requires token_provider configuration"))
		}
	}
	if cfg.Expiration != "" {
		duration, err := core.ParseHumanDuration(cfg.Expiration)
		if err != nil {
			errs = append(errs, fmt.Errorf("invalid expiration duration %q: %w", cfg.Expiration, err))
		} else if duration < 0 {
			errs = append(errs, fmt.Errorf("expiration duration must be non-negative, got %q", cfg.Expiration))
		}
	}
	return errs
}
