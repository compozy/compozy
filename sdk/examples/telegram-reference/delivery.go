package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
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

	if shouldCrashOnce(r.env.crashOncePath) {
		r.reportSideEffectError("write pre-crash delivery marker", appendJSONLine(r.env.deliveryPath, marker))
		r.reportSideEffectError("write crash marker", writeJSONFile(r.env.crashOncePath, map[string]any{
			"crashed":            true,
			"pid":                os.Getpid(),
			"delivery_id":        strings.TrimSpace(request.Event.DeliveryID),
			"bridge_instance_id": instanceID,
		}))
		os.Exit(23)
	}

	ack, err := r.ackDelivery(request)
	if err != nil {
		r.setLastError(err)
		marker.Error = err.Error()
		r.reportSideEffectError("write failed delivery marker", appendJSONLine(r.env.deliveryPath, marker))
		return bridgepkg.DeliveryAck{}, err
	}
	marker.Ack = &ack
	r.reportSideEffectError("write delivery marker", appendJSONLine(r.env.deliveryPath, marker))
	r.clearLastError()
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
		r.reportSideEffectError("write failed progress marker", appendJSONLine(r.env.deliveryPath, marker))
		return bridgepkg.DeliveryAck{}, err
	}
	marker.Ack = &ack
	r.reportSideEffectError("write progress marker", appendJSONLine(r.env.deliveryPath, marker))
	return ack, nil
}

func (r *telegramReferenceRuntime) deliveryMarkerForManagedInstance(
	session *bridgesdk.Session,
	request bridgepkg.DeliveryRequest,
) (deliveryMarker, string, error) {
	marker := deliveryMarker{
		PID:     os.Getpid(),
		Request: request,
	}

	instanceID := strings.TrimSpace(request.Event.BridgeInstanceID)
	if _, ok := session.Cache().Get(instanceID); ok {
		return marker, instanceID, nil
	}

	err := fmt.Errorf("telegram-reference: delivery targeted unmanaged instance %q", instanceID)
	marker.Error = err.Error()
	r.reportSideEffectError("write failed delivery marker", appendJSONLine(r.env.deliveryPath, marker))
	r.setLastError(err)
	return marker, "", err
}
