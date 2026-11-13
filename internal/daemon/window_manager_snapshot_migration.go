package daemon

import (
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/internal/windowmanager"
)

func decodeWindowManagerSnapshot(
	encoded []byte,
	workspaceID windowmanager.WorkspaceID,
) (windowmanager.Snapshot, error) {
	if version := storedWindowManagerSnapshotVersion(encoded); version > windowmanager.SnapshotVersion {
		return windowmanager.Snapshot{}, &windowmanager.SnapshotVersionError{Version: version}
	}
	snapshot, _, err := windowmanager.DecodeSnapshot(encoded)
	if err != nil {
		return windowmanager.Snapshot{}, fmt.Errorf("decode stored snapshot: %w", err)
	}
	if snapshot.WorkspaceID != workspaceID {
		return windowmanager.Snapshot{}, fmt.Errorf("snapshot workspace mismatch: %w", windowmanager.ErrInvalidTopology)
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
