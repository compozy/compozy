package daemon

import (
	"testing"
	"time"

	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/windowmanager"
)

func nativeResultViewFixturesLayout(t *testing.T) []nativeResultViewFixture {
	t.Helper()
	s := windowmanager.Snapshot{
		Version:     windowmanager.SnapshotVersion,
		WorkspaceID: "ws",
		Revision:    4,
		Desktops:    []windowmanager.Desktop{{ID: "desk", Name: "Work"}},
		Windows: map[windowmanager.WindowID]windowmanager.Window{
			"win": {
				ID:        "win",
				App:       "terminal",
				DesktopID: "desk",
				Route:     windowmanager.RouteIntent{Pathname: "/terminal"},
			},
		},
		ClosedEntries: []windowmanager.ClosedEntry{{DesktopID: "desk"}},
		History:       windowmanager.History{Undo: []windowmanager.HistoryEntry{{CommandID: "cmd"}}},
		UpdatedAt:     time.Unix(1, 0).UTC(),
		Migration:     &windowmanager.MigrationStatus{State: "migrated", FromVersion: 5, SupersededBackups: 2},
	}
	p := windowManagerSnapshotResult{WorkspaceID: "ws", Revision: 4, Snapshot: s}
	preview := windowManagerPreviewResult{
		Deprecations: []windowmanager.Deprecation{windowmanager.SurfaceDeprecation("window.zoom")},
		WorkspaceID:  "ws",
		Revision:     4,
		CommandID:    "layout.arrange",
		Snapshot:     s,
		Changes:      windowmanager.ChangeSet{WindowIDs: []windowmanager.WindowID{"win"}},
		Diagnostics: []windowmanager.Diagnostic{
			{Code: "invalid_topology", Path: "desktops.0", Message: "Cannot arrange"},
			{Code: "missing_window", Path: "windows.win", Message: "Window unavailable"},
		},
		Client: &windowmanager.ClientView{WorkspaceID: "ws", ClientID: "client"},
	}
	return []nativeResultViewFixture{
		{
			ToolID: toolspkg.ToolIDLayoutGet,
			Name:   "layout with history",
			Result: func(v toolspkg.ResultView) (toolspkg.ToolResult, error) { return layoutGetViews.result(v, p) },
			MustKeep: []string{
				"workspace_id",
				"revision",
				"snapshot.version",
				"snapshot.migration.state",
				"snapshot.migration.from_version",
				"snapshot.migration.superseded_backups",
				"snapshot.desktops.0.id",
				"snapshot.windows.win.id",
				"snapshot.overrides",
				"snapshot.updated_at",
			},
		},
		{
			ToolID: toolspkg.ToolIDLayoutPreview,
			Name:   "failure diagnostics",
			Result: func(v toolspkg.ResultView) (toolspkg.ToolResult, error) { return layoutPreviewViews.result(v, preview) },
			MustKeep: []string{
				"command_id",
				"deprecations.0.replacement",
				"deprecations.0.removal_release",
				"changed",
				"changes.window_ids.0",
				"diagnostics.0.code",
				"diagnostics.0.message",
				"diagnostics.1.code",
				"diagnostics.1.message",
				"client.client_id",
				"snapshot.windows.win.id",
			},
		},
		{
			ToolID: toolspkg.ToolIDLayoutPreview,
			Name:   "optional client absent",
			Result: func(v toolspkg.ResultView) (toolspkg.ToolResult, error) {
				preview.Client = nil
				return layoutPreviewViews.result(v, preview)
			},
			MustKeep: []string{"diagnostics.1.message", "snapshot.revision"},
		},
	}
}
