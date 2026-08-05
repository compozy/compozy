package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	bridgepkg "github.com/compozy/compozy/internal/bridges/contract"
	"github.com/compozy/compozy/internal/bridgesdk"
)

func (r *telegramReferenceRuntime) handleBridgesDeliver(
	_ context.Context,
	session *bridgesdk.Session,
	request bridgepkg.DeliveryRequest,
) (bridgepkg.DeliveryAck, error) {
	marker, instanceID, err := r.deliveryMarkerForManagedInstance(session, request)
	if err != nil {
		return bridgepkg.DeliveryAck{}, err
	}

	if r.markers.ShouldCrashOnce() {
		r.markers.RecordDelivery(marker)
		r.markers.RecordCrash(map[string]any{
			"crashed":            true,
			"pid":                os.Getpid(),
			"delivery_id":        strings.TrimSpace(request.Event.DeliveryID),
			"bridge_instance_id": instanceID,
		})
		os.Exit(23)
	}

	ack, err := r.ackDelivery(request)
	if err != nil {
		r.lifecycle.SetError(err)
		marker.Error = err.Error()
		r.markers.RecordDelivery(marker)
		return bridgepkg.DeliveryAck{}, err
	}
	marker.Ack = &ack
	r.markers.RecordDelivery(marker)
	r.lifecycle.ClearError()
	return ack, nil
}

func (r *telegramReferenceRuntime) handleBridgesProgress(
	_ context.Context,
	session *bridgesdk.Session,
	request bridgepkg.DeliveryRequest,
) (bridgepkg.DeliveryAck, error) {
	marker, _, err := r.deliveryMarkerForManagedInstance(session, request)
	if err != nil {
		return bridgepkg.DeliveryAck{}, err
	}

	ack, err := session.AckDelivery(request, "", "")
	if err != nil {
		marker.Error = err.Error()
		r.markers.RecordDelivery(marker)
		return bridgepkg.DeliveryAck{}, err
	}
	marker.Ack = &ack
	r.markers.RecordDelivery(marker)
	return ack, nil
}

func (r *telegramReferenceRuntime) deliveryMarkerForManagedInstance(
	session *bridgesdk.Session,
	request bridgepkg.DeliveryRequest,
) (bridgesdk.DeliveryMarker, string, error) {
	marker := bridgesdk.DeliveryMarker{
		PID:     os.Getpid(),
		Request: request,
	}

	instanceID := strings.TrimSpace(request.Event.BridgeInstanceID)
	if _, ok := session.Cache().Get(instanceID); ok {
		return marker, instanceID, nil
	}

	err := fmt.Errorf("telegram-reference: delivery targeted unmanaged instance %q", instanceID)
	marker.Error = err.Error()
	r.markers.RecordDelivery(marker)
	r.lifecycle.SetError(err)
	return marker, "", err
}
