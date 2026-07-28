package bridges

import "encoding/json"

func cloneDeliveryEvent(event DeliveryEvent) DeliveryEvent {
	return event.normalize()
}

func cloneDeliverySnapshot(snapshot DeliverySnapshot) DeliverySnapshot {
	return snapshot.normalize()
}

func cloneDeliveryRequest(req DeliveryRequest) DeliveryRequest {
	cloned := DeliveryRequest{Event: cloneDeliveryEvent(req.Event)}
	if req.Snapshot != nil {
		snapshot := cloneDeliverySnapshot(*req.Snapshot)
		cloned.Snapshot = &snapshot
	}
	return cloned
}

func cloneRawJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}
