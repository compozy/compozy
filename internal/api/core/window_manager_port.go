package core

import (
	"context"

	"github.com/compozy/agh/internal/windowmanager"
)

// WindowManagerService is the semantic window-manager port consumed by public transports.
type WindowManagerService interface {
	Snapshot(context.Context, windowmanager.WorkspaceID) (windowmanager.Snapshot, error)
	Preview(context.Context, windowmanager.CommandRequest) (windowmanager.Preview, error)
	Execute(context.Context, windowmanager.CommandRequest) (windowmanager.Result, error)
	Clients(context.Context, windowmanager.WorkspaceID) ([]windowmanager.ClientView, error)
	RegisterClient(context.Context, windowmanager.ClientRegistration) (windowmanager.ClientView, error)
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
