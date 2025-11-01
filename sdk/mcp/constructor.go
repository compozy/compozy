package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	enginemcp "github.com/compozy/compozy/engine/mcp"
	sdkerrors "github.com/compozy/compozy/sdk/v2/internal/errors"
	"github.com/compozy/compozy/sdk/v2/internal/validate"
)

func New(ctx context.Context, id string, opts ...Option) (*enginemcp.Config, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	cfg := &enginemcp.Config{
		ID: strings.TrimSpace(id),
	}
	for _, opt := range opts {
		opt(cfg)
	}
	if err := validateConfig(ctx, cfg); err != nil {
		return nil, err
	}
	cfg.SetDefaults()
	clone, err := core.DeepCopy(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to clone mcp config: %w", err)
	}
	return clone, nil
}

func validateConfig(ctx context.Context, cfg *enginemcp.Config) error {
	collected := make([]error, 0)
	if err := validate.ID(ctx, cfg.ID); err != nil {
		collected = append(collected, fmt.Errorf("mcp id is invalid: %w", err))
	}
	if err := validateTransportSelection(cfg); err != nil {
		collected = append(collected, err)
	}
	if len(collected) > 0 {
		return &sdkerrors.BuildError{Errors: collected}
	}
	return nil
}

func validateTransportSelection(cfg *enginemcp.Config) error {
	hasCommand := strings.TrimSpace(cfg.Command) != ""
	hasURL := strings.TrimSpace(cfg.URL) != ""
	switch {
	case hasCommand && hasURL:
		return fmt.Errorf("configure either command or url, not both")
	case !hasCommand && !hasURL:
		return fmt.Errorf("either command or url must be configured")
	default:
		return nil
	}
}
