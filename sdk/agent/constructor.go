package agent

import (
	"context"
	"fmt"
	"strings"

	engineagent "github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/v2/internal/errors"
	"github.com/compozy/compozy/sdk/v2/internal/validate"
)

// New creates an agent configuration using functional options
func New(ctx context.Context, id string, opts ...Option) (*engineagent.Config, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	log := logger.FromContext(ctx)
	log.Debug("creating agent configuration", "agent", id)
	cfg := &engineagent.Config{
		Resource: string(core.ConfigAgent),
		ID:       strings.TrimSpace(id),
	}
	for _, opt := range opts {
		opt(cfg)
	}
	collected := make([]error, 0, 8)
	cfg.ID = strings.TrimSpace(cfg.ID)
	if err := validate.ID(ctx, cfg.ID); err != nil {
		collected = append(collected, fmt.Errorf("agent id is invalid: %w", err))
	}
	cfg.Instructions = strings.TrimSpace(cfg.Instructions)
	if err := validate.NonEmpty(ctx, "instructions", cfg.Instructions); err != nil {
		collected = append(collected, err)
	}
	if err := validateModel(ctx, cfg); err != nil {
		collected = append(collected, err)
	}
	if err := validateKnowledge(cfg); err != nil {
		collected = append(collected, err)
	}
	if err := validateMemory(cfg); err != nil {
		collected = append(collected, err)
	}
	filtered := make([]error, 0, len(collected))
	for _, err := range collected {
		if err != nil {
			filtered = append(filtered, err)
		}
	}
	if len(filtered) > 0 {
		return nil, &sdkerrors.BuildError{Errors: filtered}
	}
	cloned, err := core.DeepCopy(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to clone agent config: %w", err)
	}
	return cloned, nil
}

func validateModel(ctx context.Context, cfg *engineagent.Config) error {
	if cfg.Model.HasRef() {
		cfg.Model.Ref = strings.TrimSpace(cfg.Model.Ref)
		if err := validate.NonEmpty(ctx, "model reference", cfg.Model.Ref); err != nil {
			return err
		}
		return nil
	}
	if cfg.Model.HasConfig() {
		provider := strings.ToLower(strings.TrimSpace(string(cfg.Model.Config.Provider)))
		modelName := strings.TrimSpace(cfg.Model.Config.Model)
		if err := validate.NonEmpty(ctx, "model provider", provider); err != nil {
			return err
		}
		if err := validate.NonEmpty(ctx, "model name", modelName); err != nil {
			return err
		}
		cfg.Model.Config.Provider = core.ProviderName(provider)
		cfg.Model.Config.Model = modelName
		return nil
	}
	return nil
}

func validateKnowledge(cfg *engineagent.Config) error {
	if len(cfg.Knowledge) > 1 {
		return fmt.Errorf("only one knowledge binding is supported")
	}
	if len(cfg.Knowledge) == 1 {
		binding := cfg.Knowledge[0]
		if strings.TrimSpace(binding.ID) == "" {
			return fmt.Errorf("knowledge binding id cannot be empty")
		}
		cfg.Knowledge[0].ID = strings.TrimSpace(binding.ID)
	}
	return nil
}

func validateMemory(cfg *engineagent.Config) error {
	for idx := range cfg.Memory {
		cfg.Memory[idx].ID = strings.TrimSpace(cfg.Memory[idx].ID)
		if cfg.Memory[idx].ID == "" {
			return fmt.Errorf("memory reference at index %d is missing an id", idx)
		}
	}
	return nil
}
