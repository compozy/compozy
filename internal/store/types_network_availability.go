package store

import (
	"context"
	"time"
)

// NetworkAvailability is the persisted admission kill-switch state.
type NetworkAvailability struct {
	Enabled   bool
	Epoch     int64
	UpdatedAt time.Time
	UpdatedBy string
}

// NetworkAvailabilityStore serializes availability changes with network admission writes.
type NetworkAvailabilityStore interface {
	GetNetworkAvailability(ctx context.Context) (NetworkAvailability, error)
	SetNetworkAvailability(
		ctx context.Context,
		enabled bool,
		updatedBy string,
	) (NetworkAvailability, error)
}
