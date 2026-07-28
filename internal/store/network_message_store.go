package store

import "context"

// NetworkMessageStore manages persisted network timeline messages.
type NetworkMessageStore interface {
	WriteNetworkMessage(ctx context.Context, entry NetworkMessageEntry) error
	ListNetworkMessages(ctx context.Context, query NetworkMessageQuery) ([]NetworkMessageEntry, error)
}
