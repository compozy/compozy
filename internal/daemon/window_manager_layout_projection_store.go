package daemon

import (
	"context"
	"log/slog"

	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/windowmanager"
)

// Invalid saved layouts stay stored for repair without blocking other boot projections.
type windowLayoutProjectionStore struct {
	resources.RawStore
	codec  resources.KindCodec[windowmanager.LayoutResource]
	logger *slog.Logger
}

var _ resources.RawStore = windowLayoutProjectionStore{}

func (s windowLayoutProjectionStore) ListRaw(
	ctx context.Context,
	actor resources.MutationActor,
	filter resources.ResourceFilter,
) ([]resources.RawRecord, error) {
	records, err := s.RawStore.ListRaw(ctx, actor, filter)
	if err != nil {
		return nil, err
	}
	logger := s.logger
	if logger == nil {
		logger = slog.Default()
	}
	visible := make([]resources.RawRecord, 0, len(records))
	for _, record := range records {
		if record.Kind == windowmanager.WindowLayoutResourceKind {
			if _, err := s.codec.DecodeAndValidate(ctx, record.Scope, record.SpecJSON); err != nil {
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				logger.Warn("window_manager.layout_projection_skipped", "resource_id", record.ID, "error", err)
				continue
			}
		}
		visible = append(visible, record)
	}
	return visible, nil
}
