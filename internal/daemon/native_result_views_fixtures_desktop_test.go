package daemon

import (
	"testing"
	"time"

	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/windowmanager"
)

func nativeResultViewFixturesDesktop(t *testing.T) []nativeResultViewFixture {
	t.Helper()
	p := windowManagerClientsResult{
		WorkspaceID: "ws",
		Revision:    4,
		Clients: []windowmanager.ClientView{
			{
				WorkspaceID:          "ws",
				ClientID:             "client",
				Kind:                 windowmanager.ClientKindBrowser,
				ActiveDesktopID:      "desk",
				FocusedWindowID:      new(windowmanager.WindowID("win")),
				PresentationRevision: 8,
				ContextRevision:      7,
				FocusOrder:           []windowmanager.WindowID{"win"},
				ConnectedAt:          time.Unix(1, 0).UTC(),
			},
		},
	}
	return []nativeResultViewFixture{
		{
			ToolID: toolspkg.ToolIDDesktopClients,
			Name:   "attached client",
			Result: func(v toolspkg.ResultView) (toolspkg.ToolResult, error) { return desktopClientsViews.result(v, p) },
			MustKeep: []string{
				"workspace_id",
				"revision",
				"clients.0.client_id",
				"clients.0.presentation_revision",
				"clients.0.context_revision",
				"clients.0.active_desktop_id",
				"clients.0.focused_window_id",
			},
		},
		{
			ToolID: toolspkg.ToolIDDesktopClients,
			Name:   "empty list",
			Result: func(v toolspkg.ResultView) (toolspkg.ToolResult, error) {
				p.Clients = []windowmanager.ClientView{}
				return desktopClientsViews.result(v, p)
			},
			MustKeep: []string{"clients", "revision"},
		},
	}
}
