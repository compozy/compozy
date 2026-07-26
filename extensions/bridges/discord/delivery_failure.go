package main

import (
	"context"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
)

func (p *discordProvider) shouldContinueAfterProgressFailure(
	ctx context.Context,
	session *bridgesdk.Session,
	instanceID string,
	marker *bridgesdk.DeliveryMarker,
	err error,
) bool {
	p.reportDiscordDeliveryError(ctx, session, instanceID, err)
	if bridgesdk.ShouldContinueTextDeliveryAfterProgress(err) {
		return true
	}
	marker.Error = err.Error()
	p.markers.RecordDelivery(*marker)
	return false
}

func (p *discordProvider) recordDeliveryFailure(
	ctx context.Context,
	session *bridgesdk.Session,
	instanceID string,
	request bridgepkg.DeliveryRequest,
	state deliveryState,
	marker bridgesdk.DeliveryMarker,
	err error,
) {
	if bridgesdk.IsCommittedMutation(err) {
		p.deliveries.Delete(deliveryStateKey(instanceID, request.Event.DeliveryID))
		if state.Progress != nil {
			state.Progress.Close()
		}
	} else if state.Chunks.Active() {
		p.storeDeliveryState(instanceID, request.Event.DeliveryID, state)
	}
	marker.Error = err.Error()
	p.markers.RecordDelivery(marker)
	p.reportDiscordDeliveryError(ctx, session, instanceID, err)
}
