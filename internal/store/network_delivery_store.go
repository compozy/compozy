package store

import "context"

// NetworkAcceptanceStore owns atomic message acceptance and wake accounting.
type NetworkAcceptanceStore interface {
	AcceptNetworkMessage(
		ctx context.Context,
		req AcceptNetworkMessageRequest,
	) (AcceptNetworkMessageResult, error)
}

// NetworkInboxStore reads immutable delivered dispositions by durable session identity.
type NetworkInboxStore interface {
	ListNetworkInbox(
		ctx context.Context,
		workspaceID string,
		sessionID string,
		channel string,
		limit int,
	) ([]NetworkMessageEntry, error)
}

// NetworkCausationStore resolves durable wake ancestry for bounded reply chains.
type NetworkCausationStore interface {
	ResolveNetworkCausation(
		ctx context.Context,
		workspaceID string,
		causationID string,
	) (rootID string, depth int, found bool, err error)
}

// NetworkWakeQueueStore reads committed wake work for daemon execution.
type NetworkWakeQueueStore interface {
	ListQueuedNetworkWakes(ctx context.Context, limit int) ([]CommittedNetworkNotification, error)
	LoadNetworkWake(
		ctx context.Context,
		workspaceID string,
		wakeID string,
	) (WakeReservation, []NetworkMessageEntry, error)
}

// NetworkUsageStore reads truthful wake-ledger detail and aggregates.
type NetworkUsageStore interface {
	GetNetworkUsage(ctx context.Context, query NetworkUsageQuery) (NetworkUsageReport, error)
}
