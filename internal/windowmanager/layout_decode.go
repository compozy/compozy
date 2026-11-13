package windowmanager

import (
	"encoding/json"
	"fmt"
)

// UnmarshalJSON permanently accepts saved v4/v5 layouts at the document boundary.
func (document *LayoutDocument) UnmarshalJSON(raw []byte) error {
	var header struct {
		Version uint32 `json:"version"`
	}
	if err := json.Unmarshal(raw, &header); err != nil {
		return err
	}
	if header.Version == 4 || header.Version == 5 {
		var legacy LayoutDocumentV5
		if err := decodeSnapshotJSON(raw, &legacy); err != nil {
			return err
		}
		workspace := legacy.WorkspaceID
		if workspace == "" {
			workspace = "window-layout-resource"
		}
		snapshot, _, err := ConvertSnapshotV5(
			SnapshotV5{
				Version:     5,
				WorkspaceID: workspace,
				Desktops:    legacy.Desktops,
				Windows:     legacy.Windows,
				Overrides:   legacy.Overrides,
			},
		)
		if err != nil {
			return fmt.Errorf("convert layout document: %w", err)
		}
		*document = LayoutDocument{
			Version:     SnapshotVersion,
			WorkspaceID: legacy.WorkspaceID,
			Desktops:    snapshot.Desktops,
			Windows:     snapshot.Windows,
			Overrides:   snapshot.Overrides,
		}
		return nil
	}
	type current LayoutDocument
	var decoded current
	if err := decodeSnapshotJSON(raw, &decoded); err != nil {
		return err
	}
	*document = LayoutDocument(decoded)
	return nil
}
