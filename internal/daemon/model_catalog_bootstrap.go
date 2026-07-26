package daemon

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/agh/internal/modelcatalog"
)

// rehydrateStaticModelCatalogSources rebuilds derived rows whose schema or
// builtin metadata may have changed since the previous daemon run.
func rehydrateStaticModelCatalogSources(
	ctx context.Context,
	service modelcatalog.Service,
	now func() time.Time,
) error {
	refreshedAt := time.Now().UTC()
	if now != nil {
		refreshedAt = now().UTC()
	}
	for _, sourceID := range []string{modelcatalog.SourceIDBuiltin, modelcatalog.SourceIDConfig} {
		if _, err := service.Refresh(ctx, modelcatalog.RefreshOptions{
			SourceID: sourceID,
			Force:    true,
			Now:      refreshedAt,
		}); err != nil {
			return fmt.Errorf("daemon: rehydrate model catalog source %q: %w", sourceID, err)
		}
	}
	return nil
}
