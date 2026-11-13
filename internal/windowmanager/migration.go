package windowmanager

import "time"

type MigrationSummary struct {
	Desktops           int `json:"desktops"`
	Tabs               int `json:"tabs"`
	Windows            int `json:"windows"`
	FloatingToTabs     int `json:"floating_to_tabs"`
	MinimizedToTabs    int `json:"minimized_to_tabs"`
	IslandsMerged      int `json:"islands_merged"`
	PinsDropped        int `json:"pins_dropped"`
	ZoomDesktopsMerged int `json:"zoom_desktops_merged"`
}

type MigrationStatus struct {
	State             string            `json:"state"`
	FromVersion       uint32            `json:"from_version,omitempty"`
	At                time.Time         `json:"at,omitzero"`
	Acknowledged      bool              `json:"acknowledged"`
	Summary           *MigrationSummary `json:"summary,omitzero"`
	Error             *Diagnostic       `json:"error,omitzero"`
	FailureRevision   Revision          `json:"failure_revision,omitempty"`
	SupersededBackups int               `json:"superseded_backups,omitempty"`
}

type MigrationNotice struct {
	WorkspaceID WorkspaceID
	ProfileID   string
	Status      MigrationStatus
}

type Deprecation struct {
	Code           string `json:"code"`
	Surface        string `json:"surface"`
	Replacement    string `json:"replacement,omitempty"`
	RemovalRelease string `json:"removal_release"`
	Message        string `json:"message"`
}
