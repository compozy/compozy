package daemon

import (
	"github.com/compozy/compozy/internal/windowmanager"
)

type compactClient struct {
	WorkspaceID          windowmanager.WorkspaceID `json:"workspace_id"`
	ClientID             windowmanager.ClientID    `json:"client_id"`
	Kind                 windowmanager.ClientKind  `json:"kind"`
	PresentationRevision uint64                    `json:"presentation_revision"`
	ContextRevision      uint64                    `json:"context_revision"`
	ActiveDesktopID      windowmanager.DesktopID   `json:"active_desktop_id"`
	FocusedWindowID      *windowmanager.WindowID   `json:"focused_window_id,omitzero"`
}

func projectCompactClient(p windowmanager.ClientView) compactClient {
	return compactClient{
		WorkspaceID:          p.WorkspaceID,
		ClientID:             p.ClientID,
		Kind:                 p.Kind,
		PresentationRevision: p.PresentationRevision,
		ContextRevision:      p.ContextRevision,
		ActiveDesktopID:      p.ActiveDesktopID,
		FocusedWindowID:      p.FocusedWindowID,
	}
}
