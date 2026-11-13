package daemon

import (
	"bytes"
	"context"
	"fmt"
	"slices"

	"github.com/compozy/compozy/internal/clientstate"
	"github.com/compozy/compozy/internal/windowmanager"
	"github.com/compozy/compozy/internal/windowmanager/statekeys"
)

// Callers commit backups with the converted snapshot and status; superseded bytes never expire.
func migrationBackupOps(
	ctx context.Context,
	service clientstate.Service,
	workspace clientstate.WorkspaceID,
	profile string,
	raw []byte,
	replacement clientstate.Op,
) ([]clientstate.Op, []string, error) {
	entries, err := service.List(ctx, workspace, windowManagerStateDomain)
	if err != nil {
		return nil, nil, err
	}
	keys := supersededBackupKeys(entries, profile)
	var legacy clientstate.Entry
	archived := make(map[string]clientstate.Entry)
	for _, entry := range entries {
		if entry.Deleted {
			continue
		}
		if entry.Key == windowManagerLegacyKey(profile) {
			legacy = entry
		}
		archived[entry.Key] = entry
	}
	if legacy.Key != "" && bytes.Equal(legacy.Value, raw) {
		return nil, keys, nil
	}
	var ops []clientstate.Op
	if legacy.Key != "" {
		key := statekeys.SupersededLegacy(profile, legacy.Value)
		if previous, exists := archived[key]; exists {
			if !bytes.Equal(previous.Value, legacy.Value) {
				return nil, nil, fmt.Errorf("superseded backup %q has different bytes", key)
			}
		} else {
			ops = append(ops, clientstate.Op{
				Kind: clientstate.OpCopy, Key: key, SourceKey: legacy.Key, SourceRev: legacy.Rev,
			})
			keys = append(keys, key)
		}
	}
	replacement.Key, replacement.IfRev = windowManagerLegacyKey(profile), legacy.Rev
	ops = append(ops, replacement)
	return ops, keys, nil
}

func supersededBackupKeys(entries []clientstate.Entry, profile string) []string {
	var keys []string
	for _, entry := range entries {
		owner, ok := statekeys.Profile(entry.Key)
		if !entry.Deleted && ok && owner == profile && statekeys.IsSupersededLegacy(entry.Key) {
			keys = append(keys, entry.Key)
		}
	}
	return keys
}

// State remains authoritative; the diagnostic notes archives, while retry uses the latest legacy bytes.
func annotateSupersededBackups(status *windowmanager.MigrationStatus, keys []string) {
	if len(keys) == 0 {
		return
	}
	message := fmt.Sprintf("%d earlier layout backup(s) were set aside and preserved; "+
		"retry uses the current legacy backup.", len(keys))
	if status.Error == nil {
		status.Error = &windowmanager.Diagnostic{
			Code: "migration.backup_superseded", Path: slices.Min(keys), Message: message,
		}
		return
	}
	diagnostic := *status.Error
	diagnostic.Message += " " + message
	status.Error = &diagnostic
}

func (r *windowManagerRepository) annotateSupersededBackups(
	ctx context.Context,
	workspace windowmanager.WorkspaceID,
	status *windowmanager.MigrationStatus,
) error {
	entries, err := r.service.List(ctx, clientstate.WorkspaceID(workspace), windowManagerStateDomain)
	if err != nil {
		return err
	}
	annotateSupersededBackups(status, supersededBackupKeys(entries, r.profileID))
	return nil
}
