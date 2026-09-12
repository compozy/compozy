package core

import (
	"context"
	"fmt"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func (h *BaseHandlers) activeConfig(ctx context.Context) (compozyconfig.Config, error) {
	if source, ok := h.Settings.(interface {
		ActiveConfig(context.Context) (compozyconfig.Config, error)
	}); ok {
		cfg, err := source.ActiveConfig(ctx)
		if err != nil {
			return compozyconfig.Config{}, fmt.Errorf("api: read active configuration: %w", err)
		}
		return cfg, nil
	}
	return compozyconfig.CloneConfig(&h.Config), nil
}
