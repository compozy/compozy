package globaldb

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// ListAttentionWorkspaceMutes returns the muted workspace registrations for one profile.
func (r *AttentionRepo) ListAttentionWorkspaceMutes(ctx context.Context, profileID string) ([]string, error) {
	if err := r.checkReady(ctx, "list attention workspace mutes"); err != nil {
		return nil, err
	}
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return nil, errors.New("store: attention profile id is required")
	}
	workspaceIDs, err := r.queries.ListAttentionWorkspaceMutes(ctx, profileID)
	if err != nil {
		return nil, fmt.Errorf("store: list attention workspace mutes for profile %q: %w", profileID, err)
	}
	return workspaceIDs, nil
}

// IsAttentionWorkspaceMuted reports whether one profile muted one workspace.
func (r *AttentionRepo) IsAttentionWorkspaceMuted(
	ctx context.Context,
	profileID string,
	workspaceID string,
) (bool, error) {
	if err := r.checkReady(ctx, "read attention workspace mute"); err != nil {
		return false, err
	}
	profileID, workspaceID = strings.TrimSpace(profileID), strings.TrimSpace(workspaceID)
	if profileID == "" || workspaceID == "" {
		return false, errors.New("store: attention profile id and workspace id are required")
	}
	muted, err := r.queries.IsAttentionWorkspaceMuted(ctx, sqlcgen.IsAttentionWorkspaceMutedParams{
		ProfileID: profileID, WorkspaceID: workspaceID,
	})
	if err != nil {
		return false, fmt.Errorf(
			"store: read attention workspace mute for profile %q and workspace %q: %w",
			profileID,
			workspaceID,
			err,
		)
	}
	return muted, nil
}

// ReplaceAttentionWorkspaceMutes atomically replaces one profile's mute set.
func (r *AttentionRepo) ReplaceAttentionWorkspaceMutes(
	ctx context.Context,
	profileID string,
	workspaceIDs []string,
) (err error) {
	if err := r.checkReady(ctx, "replace attention workspace mutes"); err != nil {
		return err
	}
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return errors.New("store: attention profile id is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin attention workspace mute replacement: %w", err)
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, tx.Rollback())
		}
	}()
	queries := r.queries.WithTx(tx)
	if err := queries.DeleteAttentionWorkspaceMutesForProfile(ctx, profileID); err != nil {
		return fmt.Errorf("store: clear attention workspace mutes for profile %q: %w", profileID, err)
	}
	for _, workspaceID := range workspaceIDs {
		workspaceID = strings.TrimSpace(workspaceID)
		if workspaceID == "" {
			return errors.New("store: attention workspace id is required")
		}
		if _, err := queries.InsertAttentionWorkspaceMute(ctx, sqlcgen.InsertAttentionWorkspaceMuteParams{
			ProfileID: profileID, WorkspaceID: workspaceID,
		}); err != nil {
			return fmt.Errorf(
				"store: insert attention workspace mute for profile %q and workspace %q: %w",
				profileID,
				workspaceID,
				err,
			)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit attention workspace mute replacement: %w", err)
	}
	return nil
}
