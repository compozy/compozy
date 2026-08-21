package deadentity

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

// List returns durable dead marks for one workspace for diagnostic projections.
func (s *Service) List(ctx context.Context, workspaceID string) ([]store.DeadEntity, error) {
	if ctx == nil {
		return nil, fmt.Errorf("deadentity: list context is required")
	}
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	trimmed := strings.TrimSpace(workspaceID)
	if trimmed == "" {
		return nil, fmt.Errorf("%w: workspace_id is required", store.ErrInvalidDeadEntity)
	}
	if s == nil || s.store == nil {
		return []store.DeadEntity{}, nil
	}
	entities, err := s.store.ListDeadEntities(ctx, store.ReadScope{AllProfiles: true}, trimmed)
	if err != nil && contextError(ctx) != nil {
		return nil, contextError(ctx)
	}
	return entities, err
}
