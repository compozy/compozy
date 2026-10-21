package daemon

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"log/slog"

	"github.com/compozy/compozy/internal/clientstate"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/store/workspacedb"
	"github.com/compozy/compozy/internal/windowmanager"
)

func reconcileUserScopeMigrations(
	ctx context.Context,
	db *globaldb.GlobalDB,
	engine *clientstate.Engine,
	homeDir string,
	logger *slog.Logger,
) error {
	migrations, err := db.PendingUserScopeMigrations(ctx)
	if err != nil {
		return err
	}
	for _, migration := range migrations {
		id := migration.SourceWorkspaceID
		if migration.JournalState != "verified" && migration.JournalState != "skipped" {
			if err := workspacedb.ImportLegacyJournal(ctx, homeDir, id, migration.ProfileIDs); err != nil {
				return fmt.Errorf("migrate User journal %s: %w", id, err)
			}
			if err := db.MarkUserScopeStoreVerified(ctx, id, "journal"); err != nil {
				return err
			}
			logger.InfoContext(ctx, "user_scope.migration.journal_verified", "workspace_id", id)
		}
		if migration.LayoutState != "verified" && migration.LayoutState != "skipped" {
			if err := migrateUserScopeLayouts(ctx, engine, id, logger); err != nil {
				return err
			}
			if err := db.MarkUserScopeStoreVerified(ctx, id, "layout"); err != nil {
				return err
			}
			logger.InfoContext(ctx, "user_scope.migration.layout_verified", "workspace_id", id)
		}
		if err := db.CompleteUserScopeMigration(ctx, id); err != nil {
			logger.WarnContext(ctx, "user_scope.migration.registration_retained", "workspace_id", id, "error", err)
			continue
		}
		logger.InfoContext(ctx, "user_scope.migration.registration_removed", "workspace_id", id)
	}
	return nil
}

func migrateUserScopeLayouts(
	ctx context.Context,
	engine *clientstate.Engine,
	sourceID string,
	logger *slog.Logger,
) error {
	entries, err := engine.List(ctx, clientstate.WorkspaceID(sourceID), windowManagerStateDomain)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Deleted {
			continue
		}
		if _, ok := windowManagerSnapshotProfile(entry.Key); !ok {
			continue
		}
		target := clientstate.WorkspaceID(windowmanager.UserDesktopWorkspaceID)
		_, err := engine.Get(ctx, target, windowManagerStateDomain, entry.Key)
		if err == nil {
			logger.InfoContext(
				ctx,
				"user_scope.migration.layout_source_retained",
				"workspace_id",
				sourceID,
				"key",
				entry.Key,
			)
			continue
		}
		if !errors.Is(err, clientstate.ErrNotFound) {
			return err
		}
		var snapshot windowmanager.Snapshot
		if err := json.Unmarshal(entry.Value, &snapshot); err != nil {
			return err
		}
		if snapshot.WorkspaceID != windowmanager.WorkspaceID(sourceID) {
			return errors.New("legacy layout workspace identity mismatch")
		}
		snapshot.WorkspaceID = windowmanager.UserDesktopWorkspaceID
		encoded, err := json.Marshal(snapshot)
		if err != nil {
			return err
		}
		if _, err := engine.Apply(
			ctx,
			target,
			windowManagerStateDomain,
			[]clientstate.Op{{Kind: clientstate.OpPut, Key: entry.Key, Value: encoded}},
			clientstate.ApplyOptions{Origin: "user_scope_migration"},
		); err != nil {
			return err
		}
		saved, err := engine.Get(ctx, target, windowManagerStateDomain, entry.Key)
		if err != nil {
			return err
		}
		if string(saved.Value) != string(encoded) {
			return errors.New("User layout verification failed")
		}
	}
	return nil
}
