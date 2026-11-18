package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/clientstate"
	"github.com/compozy/compozy/internal/windowmanager"
	"github.com/compozy/compozy/internal/windowmanager/statekeys"
	"golang.org/x/mod/semver"
)

// Successful upgrade backups expire in v0.8.0. Failed input remains recoverable
// regardless of release, acknowledgement, or the number of failed retries.
func (r *windowManagerRegistry) expireMigrationBackups(ctx context.Context, release string) error {
	release = "v" + strings.TrimPrefix(release, "v")
	if !semver.IsValid(release) || semver.Compare(release, "v0.8.0") < 0 {
		return nil
	}
	workspaces, err := r.partitionWorkspaceIDs(ctx)
	if err != nil {
		return err
	}
	for _, workspace := range workspaces {
		entries, err := r.engine.List(ctx, workspace, windowManagerStateDomain)
		if errors.Is(err, clientstate.ErrWorkspaceNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		ops := r.migrationExpiryOps(ctx, workspace, entries)
		if len(ops) == 0 {
			continue
		}
		if _, err := r.engine.Apply(ctx, workspace, windowManagerStateDomain, ops,
			clientstate.ApplyOptions{Origin: "window-manager.migration.expire"},
		); err != nil {
			return err
		}
	}
	return nil
}

func (r *windowManagerRegistry) migrationExpiryOps(
	ctx context.Context,
	workspace clientstate.WorkspaceID,
	entries []clientstate.Entry,
) []clientstate.Op {
	active := make(map[string]clientstate.Entry)
	for _, entry := range entries {
		if !entry.Deleted {
			active[entry.Key] = entry
		}
	}
	var ops []clientstate.Op
	remove := func(key string) {
		if entry, ok := active[key]; ok {
			ops = append(ops, clientstate.Op{Kind: clientstate.OpDelete, Key: key, IfRev: entry.Rev})
		}
	}
	for _, entry := range entries {
		if entry.Deleted {
			continue
		}
		if statekeys.IsSupersededLegacy(entry.Key) {
			disposition := statekeys.SupersededLegacyDisposition(entry.Key)
			if migrationBackupState(active[disposition].Value) == windowManagerMigrationMigrated {
				remove(entry.Key)
				remove(disposition)
			}
			continue
		}
		profile, ok := strings.CutPrefix(entry.Key, "migration:")
		if !ok || profile == "" {
			continue
		}
		var status windowmanager.MigrationStatus
		if err := json.Unmarshal(entry.Value, &status); err != nil {
			r.logger.WarnContext(ctx, "window_manager.migration_retention_skipped",
				"workspace_id", workspace, "profile_id", profile, "error", err)
			continue
		}
		if migrationBackupState(entry.Value) == windowManagerMigrationMigrated {
			remove(windowManagerLegacyKey(profile))
		}
	}
	return ops
}
