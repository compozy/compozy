package main

import (
	"context"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
)

func (p *slackProvider) recordDeliveryFailure(
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
		closeProgressDispatcher(state.Progress)
	} else {
		p.storeDeliveryRetryState(instanceID, request.Event.DeliveryID, state)
	}
	marker.Error = err.Error()
	p.markers.RecordDelivery(marker)
	classified := bridgesdk.ClassifyError(err)
	_, _, reportErr := session.ReportClassifiedError(ctx, instanceID, classified)
	if reportErr != nil {
		p.setLastError(reportErr)
		return
	}
	p.setLastError(err)
}
