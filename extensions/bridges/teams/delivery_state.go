package main

import (
	"time"

	"github.com/compozy/agh/internal/bridgesdk"
)

const (
	teamsDeliveryStateMaxEntries    = 4_096
	teamsDeliveryStateRetentionTime = 24 * time.Hour
)

type deliveryState struct {
	LastSeq                int64
	RemoteMessageID        string
	ReplaceRemoteMessageID string
	LastContent            string
	Chunks                 bridgesdk.DeliveryChunkCursor
	ResolvedTarget         teamsResolvedTarget
	Progress               *bridgesdk.ProgressDispatcher
}

type teamsDeliveryStateLookup func(deliveryID string) (deliveryState, bool)

func newTeamsDeliveryStateStore() *bridgesdk.DeliveryStateStore[deliveryState] {
	return bridgesdk.NewDeliveryStateStore(
		bridgesdk.WithDeliveryStateRetention[deliveryState](
			teamsDeliveryStateMaxEntries,
			teamsDeliveryStateRetentionTime,
			nil,
		),
	)
}

func (p *teamsProvider) deliveryState(instanceID string, deliveryID string) deliveryState {
	state, _ := p.deliveries.Load(deliveryStateKey(instanceID, deliveryID))
	return state
}

func (p *teamsProvider) storeDeliveryState(
	instanceID string,
	deliveryID string,
	state deliveryState,
) {
	p.deliveries.Store(deliveryStateKey(instanceID, deliveryID), state)
}

func (p *teamsProvider) storeDeliveryRetryState(
	instanceID string,
	deliveryID string,
	state deliveryState,
) {
	if !state.Chunks.Active() {
		return
	}
	p.deliveries.Store(deliveryStateKey(instanceID, deliveryID), state)
}
