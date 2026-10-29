package globaldb

import (
	"context"
	"database/sql"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	"github.com/compozy/compozy/internal/workspace"
)

func (g *WorkspaceRepo) SetWorkspaceArchived(
	ctx context.Context,
	id string,
	archived bool,
) (workspace.Workspace, error) {
	if err := g.checkReady(ctx, "archive workspace"); err != nil {
		return workspace.Workspace{}, err
	}
	var updated workspace.Workspace
	err := store.ExecuteWrite(ctx, g.db, func(ctx context.Context, tx *store.WriteTx) error {
		queries := sqlcgen.New(tx)
		row, err := queries.GetWorkspace(ctx, id)
		if err != nil {
			_, err = workspaceFromGenerated(row, err)
			return err
		}
		if row.ArchivedAt.Valid == archived {
			updated, err = workspaceFromGenerated(row, nil)
			return err
		}
		var archivedAt sql.NullString
		now := store.FormatTimestamp(g.now())
		if archived {
			blockers, err := queries.ListWorkspaceArchiveBlockers(ctx, id)
			if err != nil {
				return err
			}
			if len(blockers) > 0 {
				busy := &workspace.BusyError{Blockers: make([]workspace.ArchiveBlocker, 0, len(blockers))}
				for _, blocker := range blockers {
					busy.Blockers = append(busy.Blockers, workspace.ArchiveBlocker{
						Kind: blocker.Kind, ID: blocker.ID, Name: blocker.Name.String,
					})
				}
				return busy
			}
			archivedAt = sql.NullString{String: now, Valid: true}
		}
		if _, err := queries.SetWorkspaceArchived(ctx, sqlcgen.SetWorkspaceArchivedParams{
			ID: id, ArchivedAt: archivedAt, UpdatedAt: now,
		}); err != nil {
			return err
		}
		row.ArchivedAt, row.UpdatedAt = archivedAt, now
		updated, err = workspaceFromGenerated(row, nil)
		return err
	})
	return updated, err
}
