package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// PurgeDeletedTelemetry resumes physical cleanup after core deletion has already
// hidden the affected scopes. Retention repeats it to collect in-flight appends.
func (g *GlobalDB) PurgeDeletedTelemetry(ctx context.Context) error {
	if err := g.ObserveRepo.checkReady(ctx, "purge deleted telemetry"); err != nil {
		return err
	}
	return g.ObserveRepo.sweepDeletedTelemetry(ctx)
}

func (g *ObserveRepo) validateEventSummaryOwner(ctx context.Context, summary store.EventSummary) error {
	owner, err := g.readQueries.GetTelemetryWriteOwner(ctx, sqlcgen.GetTelemetryWriteOwnerParams{
		ProfileID: summary.ProfileID, WorkspaceID: summary.WorkspaceID,
	})
	if errors.Is(err, sql.ErrNoRows) || (err == nil && owner.State != "active") {
		return errors.New("store: profile_archived")
	}
	if err != nil {
		return err
	}
	if owner.Unavailable != 0 {
		return errors.New("store: profile_unavailable")
	}
	if owner.WorkspaceDeleted != 0 {
		return errors.New("store: telemetry workspace was deleted")
	}
	return nil
}

func (g *repoBase) sweepDeletedTelemetry(ctx context.Context) error {
	scopes, err := g.readQueries.ListTelemetryDeletedScopes(ctx)
	if err != nil {
		return err
	}
	for _, scope := range scopes {
		if _, err := g.telemetry.DeleteScope(ctx, scope.Kind, scope.ScopeID); err != nil {
			return fmt.Errorf("store: purge deleted telemetry scope: %w", err)
		}
	}
	return nil
}
