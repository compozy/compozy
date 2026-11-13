package daemon

import (
	"context"
	"fmt"
	"strconv"

	"github.com/compozy/compozy/internal/cmdpalette"
	"github.com/compozy/compozy/internal/windowmanager"
)

type cmdPaletteClientDirectory struct {
	windowManagers cmdPaletteClientDirectorySource
}

type cmdPaletteClientDirectorySource interface {
	ClientsInWorkspace(
		context.Context, clientProfileScope, windowmanager.WorkspaceID,
	) ([]windowmanager.ClientView, error)
	CommandClientsInWorkspace(
		context.Context, clientProfileScope, windowmanager.WorkspaceID,
	) ([]windowmanager.ClientView, error)
	ManagerForClient(
		context.Context, clientProfileScope, windowmanager.WorkspaceID, windowmanager.ClientID,
	) (*windowmanager.Manager, error)
}

var _ cmdpalette.ClientDirectory = (*cmdPaletteClientDirectory)(nil)
var _ cmdpalette.GlobalShortcutStatusDirectory = (*cmdPaletteClientDirectory)(nil)

// paletteClientScope maps a palette profile lens onto the window-manager
// profile whose clients it may reach; only the aggregate lens widens the read.
func paletteClientScope(profileLens cmdpalette.ProfileLens) clientProfileScope {
	if profileLens.IsAggregate() {
		return clientsOfAllProfiles()
	}
	return clientsOfProfile(string(profileLens.ID))
}

func (d *cmdPaletteClientDirectory) Clients(
	ctx context.Context,
	profileLens cmdpalette.ProfileLens,
	workspaceID cmdpalette.WorkspaceID,
) ([]cmdpalette.Client, error) {
	if d == nil || d.windowManagers == nil {
		return []cmdpalette.Client{}, nil
	}
	views, err := d.windowManagers.CommandClientsInWorkspace(
		ctx, paletteClientScope(profileLens), windowmanager.WorkspaceID(workspaceID),
	)
	if err != nil {
		return nil, fmt.Errorf("cmd palette: list window-manager clients: %w", err)
	}
	return paletteClients(views, workspaceID), nil
}

func (d *cmdPaletteClientDirectory) AttachedClients(
	ctx context.Context,
	profileLens cmdpalette.ProfileLens,
	workspaceID cmdpalette.WorkspaceID,
) ([]cmdpalette.Client, error) {
	if d == nil || d.windowManagers == nil {
		return []cmdpalette.Client{}, nil
	}
	views, err := d.windowManagers.ClientsInWorkspace(
		ctx, paletteClientScope(profileLens), windowmanager.WorkspaceID(workspaceID),
	)
	if err != nil {
		return nil, fmt.Errorf("cmd palette: list attached clients: %w", err)
	}
	return paletteClients(views, workspaceID), nil
}

func paletteClients(views []windowmanager.ClientView, workspaceID cmdpalette.WorkspaceID) []cmdpalette.Client {
	clients := make([]cmdpalette.Client, 0, len(views))
	for _, view := range views {
		clients = append(clients, cmdpalette.Client{
			ID: cmdpalette.ClientID(view.ClientID), Kind: string(view.Kind), WorkspaceID: workspaceID,
			AttachedAt: view.ConnectedAt, ContextRevision: strconv.FormatUint(view.ContextRevision, 10),
		})
	}
	return clients
}

func (d *cmdPaletteClientDirectory) Context(
	ctx context.Context,
	profileLens cmdpalette.ProfileLens,
	workspaceID cmdpalette.WorkspaceID,
	clientID cmdpalette.ClientID,
) (cmdpalette.ContextSnapshot, error) {
	client, err := d.findClient(ctx, profileLens, workspaceID, clientID, "resolve client context")
	if err != nil {
		return cmdpalette.ContextSnapshot{}, err
	}
	return cmdPaletteContextSnapshot(client), nil
}

func (d *cmdPaletteClientDirectory) Authorize(
	ctx context.Context,
	profileLens cmdpalette.ProfileLens,
	workspaceID cmdpalette.WorkspaceID,
	clientID cmdpalette.ClientID,
	token string,
) error {
	if d == nil || d.windowManagers == nil {
		return cmdpalette.ErrClientUnauthorized
	}
	manager, err := d.windowManagers.ManagerForClient(
		ctx, paletteClientScope(profileLens), windowmanager.WorkspaceID(workspaceID), windowmanager.ClientID(clientID),
	)
	if err != nil {
		return cmdpalette.ErrClientUnauthorized
	}
	return manager.AuthorizeClient(
		ctx, windowmanager.WorkspaceID(workspaceID), windowmanager.ClientID(clientID), token,
	)
}

func (d *cmdPaletteClientDirectory) GlobalShortcutStatuses(
	ctx context.Context,
	profileLens cmdpalette.ProfileLens,
	workspaceID cmdpalette.WorkspaceID,
	clientID cmdpalette.ClientID,
) (map[cmdpalette.CommandID]cmdpalette.GlobalShortcut, error) {
	if d == nil || d.windowManagers == nil {
		return map[cmdpalette.CommandID]cmdpalette.GlobalShortcut{}, nil
	}
	client, err := d.findClient(ctx, profileLens, workspaceID, clientID, "resolve global shortcut status")
	if err != nil {
		return nil, err
	}
	result := make(map[cmdpalette.CommandID]cmdpalette.GlobalShortcut, len(client.GlobalShortcuts))
	for _, status := range client.GlobalShortcuts {
		result[cmdpalette.CommandID(status.CommandID)] = cmdpalette.GlobalShortcut{
			IntendedChord: status.IntendedChord,
			ActiveChord:   status.ActiveChord,
			Status:        string(status.Status),
			Reason:        status.Reason,
			SettingsURL:   status.SettingsURL,
		}
	}
	return result, nil
}

func (d *cmdPaletteClientDirectory) findClient(
	ctx context.Context,
	profileLens cmdpalette.ProfileLens,
	workspaceID cmdpalette.WorkspaceID,
	clientID cmdpalette.ClientID,
	op string,
) (windowmanager.ClientView, error) {
	if d == nil || d.windowManagers == nil {
		return windowmanager.ClientView{}, cmdpalette.ErrNoAttachedShell
	}
	clients, err := d.windowManagers.ClientsInWorkspace(
		ctx, paletteClientScope(profileLens), windowmanager.WorkspaceID(workspaceID),
	)
	if err != nil {
		return windowmanager.ClientView{}, fmt.Errorf("cmd palette: %s: %w", op, err)
	}
	for _, client := range clients {
		if client.ClientID == windowmanager.ClientID(clientID) {
			return client, nil
		}
	}
	return windowmanager.ClientView{}, cmdpalette.ErrNoAttachedShell
}

func cmdPaletteContextSnapshot(client windowmanager.ClientView) cmdpalette.ContextSnapshot {
	paletteContext := client.PaletteContext
	return cmdpalette.ContextSnapshot{
		Revision: strconv.FormatUint(client.ContextRevision, 10),
		Values: map[cmdpalette.ContextKey]any{
			cmdpalette.ContextWindowFocused:      paletteContext.WindowFocused,
			cmdpalette.ContextWindowFloating:     paletteContext.WindowFloating,
			cmdpalette.ContextWindowStacked:      paletteContext.WindowInSplit,
			cmdpalette.ContextDesktopWindowCount: paletteContext.DesktopWindowCount,
			cmdpalette.ContextScopeGlobal:        paletteContext.ScopeGlobal,
			cmdpalette.ContextShellDesktop:       paletteContext.ShellDesktop,
			cmdpalette.ContextSessionState:       paletteContext.FocusedSessionState,
			cmdpalette.ContextWorkspaceTrusted:   paletteContext.WorkspaceTrusted,
		},
	}
}
