package core

import (
	"context"

	"github.com/compozy/compozy/internal/windowmanager"
)

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
