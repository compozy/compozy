package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/clientstate"
	"github.com/compozy/compozy/internal/windowmanager"
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
		for _, entry := range entries {
			profile, ok := strings.CutPrefix(entry.Key, "migration:")
			if !ok || profile == "" || entry.Deleted {
				continue
			}
			var status windowmanager.MigrationStatus
			if err := json.Unmarshal(entry.Value, &status); err != nil {
				r.logger.WarnContext(ctx, "window_manager.migration_retention_skipped",
					"workspace_id", workspace, "profile_id", profile, "error", err)
				continue
			}
			if status.State != windowManagerMigrationMigrated {
				continue
			}
			backup, err := r.engine.Get(ctx, workspace, windowManagerStateDomain, windowManagerLegacyKey(profile))
			if errors.Is(err, clientstate.ErrNotFound) {
				continue
			}
			if err != nil {
				return err
			}
			if _, err := r.engine.Apply(ctx, workspace, windowManagerStateDomain,
				[]clientstate.Op{{Kind: clientstate.OpDelete, Key: backup.Key, IfRev: backup.Rev}},
				clientstate.ApplyOptions{Origin: "window-manager.migration.expire"},
			); err != nil {
				return err
			}
		}
	}
	return nil
}
