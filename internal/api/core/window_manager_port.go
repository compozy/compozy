package core

import (
	"context"

	"github.com/compozy/compozy/internal/windowmanager"
)

// WindowManagerProvider resolves the window manager that owns one profile's desks.
// Desktops are per-profile working state, so every transport names the profile it
// is acting as before it can reach any window state (US-026).
type WindowManagerProvider interface {
	WindowManagerFor(profileID string) (WindowManagerService, error)
	// ClaimClient attaches one client id to one profile as a single operation.
	//
	// A browser tab presents one profile at a time, so registering has to move the
	// client rather than add an attachment — and moving it is only truthful if the
	// whole claim is serialized, which is why the transport asks for it as one call
	// instead of retiring and registering in two.
	ClaimClient(
		ctx context.Context,
		profileID string,
		registration windowmanager.ClientRegistration,
	) (windowmanager.ClientView, error)
}

// WindowManagerService is the semantic window-manager port consumed by public transports.
type WindowManagerService interface {
	Snapshot(context.Context, windowmanager.WorkspaceID) (windowmanager.Snapshot, error)
	Preview(context.Context, windowmanager.CommandRequest) (windowmanager.Preview, error)
	Execute(context.Context, windowmanager.CommandRequest) (windowmanager.Result, error)
	Clients(context.Context, windowmanager.WorkspaceID) ([]windowmanager.ClientView, error)
	RegisterClient(context.Context, windowmanager.ClientRegistration) (windowmanager.ClientView, error)
	UpdateClientContext(context.Context, windowmanager.ClientContextUpdate) (windowmanager.ClientView, error)
	AuthorizeClient(context.Context, windowmanager.WorkspaceID, windowmanager.ClientID, string) error
	AttachClientCommands(
		context.Context,
		windowmanager.WorkspaceID,
		windowmanager.ClientID,
	) (windowmanager.ClientCommandConnection, error)
	DispatchClientCommand(
		context.Context,
		windowmanager.WorkspaceID,
		windowmanager.ClientID,
		windowmanager.ClientCommand,
	) (windowmanager.ClientCommandResponse, error)
	UnregisterClient(context.Context, windowmanager.WorkspaceID, windowmanager.ClientID) error
	ExportLayout(context.Context, windowmanager.WorkspaceID) (windowmanager.LayoutDocument, error)
	ValidateLayout(
		context.Context,
		windowmanager.WorkspaceID,
		windowmanager.LayoutDocument,
	) (windowmanager.Validation, error)
	ReplaceLayout(context.Context, windowmanager.ReplaceLayoutRequest) (windowmanager.Result, error)
	Subscribe(context.Context, windowmanager.SubscriptionRequest) (windowmanager.Subscription, error)
}
