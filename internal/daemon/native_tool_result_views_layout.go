package daemon

import (
	"time"

	"github.com/compozy/compozy/internal/windowmanager"
)

type compactSnapshot struct {
	Migration   *windowmanager.MigrationStatus                  `json:"migration,omitzero"`
	Version     uint32                                          `json:"version"`
	WorkspaceID windowmanager.WorkspaceID                       `json:"workspace_id"`
	Revision    windowmanager.Revision                          `json:"revision"`
	Desktops    []windowmanager.Desktop                         `json:"desktops"`
	Windows     map[windowmanager.WindowID]windowmanager.Window `json:"windows"`
	Overrides   windowmanager.WorkspaceConfig                   `json:"overrides"`
	UpdatedAt   time.Time                                       `json:"updated_at"`
}

func projectCompactSnapshot(p windowmanager.Snapshot) compactSnapshot {
	return compactSnapshot{
		Migration:   p.Migration,
		Version:     p.Version,
		WorkspaceID: p.WorkspaceID,
		Revision:    p.Revision,
		Desktops:    p.Desktops,
		Windows:     p.Windows,
		Overrides:   p.Overrides,
		UpdatedAt:   p.UpdatedAt,
	}
}
