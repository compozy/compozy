package globaldb

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	compozyworkspace "github.com/compozy/compozy/internal/workspace"
)

// InsertWorkspace creates a new persisted workspace registration row.
func (g *WorkspaceRepo) InsertWorkspace(ctx context.Context, ws compozyworkspace.Workspace) error {
	if err := g.checkReady(ctx, "insert workspace"); err != nil {
		return err
	}

	normalized, addDirsJSON, err := g.normalizeWorkspaceForInsert(ws)
	if err != nil {
		return err
	}
	var deleted int64
	if g.telemetry != nil {
		deleted, err = g.readQueries.HasWorkspaceTelemetryTombstone(ctx, normalized.ID)
		if err != nil {
			return fmt.Errorf("store: read workspace telemetry tombstone: %w", err)
		}
		if deleted != 0 {
			// Purge the previous registration without holding the core writer across telemetry batches.
			if _, err := g.telemetry.DeleteScope(ctx, "workspace", normalized.ID); err != nil {
				return fmt.Errorf("store: purge previous workspace telemetry: %w", err)
			}
		}
	}

	if err := g.executeWriteQuery(
		ctx,
		"globaldb.InsertWorkspace",
		func(ctx context.Context, queries *sqlcgen.Queries) error {
			if err := queries.InsertWorkspace(ctx, sqlcgen.InsertWorkspaceParams{
				ID:           normalized.ID,
				RootDir:      normalized.RootDir,
				AddDirs:      addDirsJSON,
				Name:         normalized.Name,
				DefaultAgent: nullableWorkspaceString(normalized.DefaultAgent),
				CreatedAt: store.FormatTimestamp(
					normalized.CreatedAt,
				),
				UpdatedAt: store.FormatTimestamp(normalized.UpdatedAt),
			}); err != nil {
				return err
			}
			if g.telemetry == nil {
				return nil
			}
			removed, err := queries.DeleteWorkspaceTelemetryTombstone(ctx, normalized.ID)
			if err != nil || removed == 0 {
				return err
			}
			if deleted == 0 {
				return errors.New("store: workspace deletion changed during registration; retry registration")
			}
			return nil
		},
	); err != nil {
		return fmt.Errorf(
			"store: insert workspace %q: %w",
			normalized.ID,
			mapWorkspaceWriteConstraintError(ctx, g.readDB, normalized, err),
		)
	}

	return nil
}
