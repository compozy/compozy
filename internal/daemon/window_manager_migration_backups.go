package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/internal/clientstate"
	"github.com/compozy/compozy/internal/windowmanager"
	"github.com/compozy/compozy/internal/windowmanager/statekeys"
)

// The archive bytes and their conservative disposition commit with the converted snapshot.
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
		previous, exists := archived[key]
		if exists {
			if !bytes.Equal(previous.Value, legacy.Value) {
				return nil, nil, fmt.Errorf("superseded backup %q has different bytes", key)
			}
		} else {
			ops = append(ops, clientstate.Op{
				Kind: clientstate.OpCopy, Key: key, SourceKey: legacy.Key, SourceRev: legacy.Rev,
			})
			keys = append(keys, key)
		}
		ops = append(ops, supersededDispositionOps(archived, profile, key, exists)...)
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

func supersededDispositionOps(
	entries map[string]clientstate.Entry,
	profile, backup string,
	exists bool,
) []clientstate.Op {
	key := statekeys.SupersededLegacyDisposition(backup)
	previous := entries[key]
	state := migrationBackupState(entries[windowManagerMigrationKey(profile)].Value)
	if exists && migrationBackupState(previous.Value) != windowManagerMigrationMigrated {
		state = windowManagerMigrationFailed
	}
	value := []byte(`{"state":"` + state + `"}`)
	if bytes.Equal(value, previous.Value) {
		return nil
	}
	return []clientstate.Op{{Kind: clientstate.OpPut, Key: key, Value: value, IfRev: previous.Rev}}
}

func migrationBackupState(raw []byte) string {
	var status windowmanager.MigrationStatus
	if err := json.Unmarshal(raw, &status); err != nil || status.State != windowManagerMigrationMigrated ||
		status.Error != nil {
		return windowManagerMigrationFailed
	}
	return windowManagerMigrationMigrated
}
