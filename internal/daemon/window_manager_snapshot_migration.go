package daemon

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/compozy/compozy/internal/windowmanager"
)

func retiredSnapshotWindows(encoded []byte) int {
	var snapshot struct {
		Windows       map[windowmanager.WindowID]windowmanager.Window `json:"windows"`
		ClosedEntries []windowmanager.ClosedEntry                     `json:"closed_entries"`
	}
	if err := json.Unmarshal(encoded, &snapshot); err != nil {
		return 0
	}
	count := 0
	for _, window := range snapshot.Windows {
		if _, retired := windowmanager.RetiredApp(window.App); retired {
			count++
		}
	}
	for _, entry := range snapshot.ClosedEntries {
		for _, window := range entry.Windows {
			if _, retired := windowmanager.RetiredApp(window.App); retired {
				count++
			}
		}
	}
	return count
}

func decodeWindowManagerSnapshot(
	encoded []byte,
	workspaceID windowmanager.WorkspaceID,
) (windowmanager.Snapshot, error) {
	if storedWindowManagerSnapshotVersion(encoded) == windowmanager.PreviousSnapshotVersion {
		snapshot, err := windowmanager.MigrateSnapshotV4(encoded)
		if err != nil {
			return windowmanager.Snapshot{}, fmt.Errorf("daemon: migrate v4 window-manager snapshot: %w", err)
		}
		if snapshot.WorkspaceID != workspaceID {
			return windowmanager.Snapshot{}, fmt.Errorf(
				"daemon: window-manager snapshot workspace mismatch: %w", windowmanager.ErrInvalidTopology,
			)
		}
		return snapshot, nil
	}
	if storedWindowManagerSnapshotVersion(encoded) == windowmanager.LegacySnapshotVersion {
		return migrateLegacyWindowManagerSnapshot(encoded, workspaceID)
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var snapshot windowmanager.Snapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return windowmanager.Snapshot{}, fmt.Errorf(
			"daemon: decode window-manager snapshot: %w",
			errors.Join(windowmanager.ErrInvalidTopology, errWindowManagerSnapshotDiscardable, err),
		)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return windowmanager.Snapshot{}, fmt.Errorf(
			"daemon: decode trailing window-manager snapshot data: %w",
			errors.Join(windowmanager.ErrInvalidTopology, errWindowManagerSnapshotDiscardable, err),
		)
	}
	if snapshot.Version != windowmanager.SnapshotVersion {
		return windowmanager.Snapshot{}, fmt.Errorf(
			"daemon: window-manager snapshot version %d is unsupported: %w",
			snapshot.Version,
			errors.Join(windowmanager.ErrInvalidTopology, errWindowManagerSnapshotDiscardable),
		)
	}
	if snapshot.WorkspaceID != workspaceID {
		return windowmanager.Snapshot{}, fmt.Errorf(
			"daemon: window-manager snapshot workspace mismatch: %w",
			windowmanager.ErrInvalidTopology,
		)
	}
	if err := windowmanager.ValidateSnapshot(snapshot); err != nil {
		return windowmanager.Snapshot{}, fmt.Errorf("daemon: validate stored window-manager snapshot: %w", err)
	}
	return snapshot, nil
}

// storedWindowManagerSnapshotVersion peeks at the version so an older aggregate
// shape can be migrated before the strict current-shape decode rejects it.
func storedWindowManagerSnapshotVersion(encoded []byte) uint32 {
	var header struct {
		Version uint32 `json:"version"`
	}
	if err := json.Unmarshal(encoded, &header); err != nil {
		return 0
	}
	return header.Version
}

// migrateLegacyWindowManagerSnapshot chains v3 through the v4 shape to v5; a
// legacy document that cannot be migrated is discarded like any other
// incompatible snapshot.
func migrateLegacyWindowManagerSnapshot(
	encoded []byte,
	workspaceID windowmanager.WorkspaceID,
) (windowmanager.Snapshot, error) {
	snapshot, err := windowmanager.MigrateLegacySnapshotV3(encoded)
	if err != nil {
		return windowmanager.Snapshot{}, fmt.Errorf(
			"daemon: migrate window-manager snapshot: %w",
			errors.Join(windowmanager.ErrInvalidTopology, errWindowManagerSnapshotDiscardable, err),
		)
	}
	if snapshot.WorkspaceID != workspaceID {
		return windowmanager.Snapshot{}, fmt.Errorf(
			"daemon: window-manager snapshot workspace mismatch: %w",
			windowmanager.ErrInvalidTopology,
		)
	}
	return snapshot, nil
}
