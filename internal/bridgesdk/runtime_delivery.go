package bridgesdk

import (
	"context"
	"encoding/json"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/subprocess"
)

func (r *Runtime) handleDeliver(ctx context.Context, raw json.RawMessage) (any, error) {
	session, err := r.requireServiceSession("bridge delivery")
	if err != nil {
		return nil, err
	}

	var request bridgepkg.DeliveryRequest
	if err := decodeParams(raw, &request); err != nil {
		return nil, err
	}
	if err := request.Validate(); err != nil {
		return nil, subprocess.NewRPCError(bridgeSDKRPCCodeInvalidParams, "Invalid params", map[string]string{
			runtimeErrorKey: err.Error(),
		})
	}

	handler := r.config.Deliver
	switch {
	case request.Event.EventType == bridgepkg.DeliveryEventTypeProgress:
		if r.config.Progress == nil {
			return session.AckDelivery(request, "", "")
		}
		handler = DeliveryHandler(r.config.Progress)
	case request.Event.IsLifecycleOnlyFinal():
		if r.config.Progress == nil {
			return session.AckDelivery(request, "", "")
		}
		handler = DeliveryHandler(r.config.Progress)
	}
	ack, err := handler(ctx, session, request)
	if err != nil {
		if IsCommittedMutation(err) {
			return session.AckCommittedResultUnavailable(request, err.Error())
		}
		return nil, err
	}
	if err := ack.ValidateFor(request.Event); err != nil {
		return session.AckCommittedResultUnavailable(request, err.Error())
	}
	return bridgepkg.NormalizeDeliveryAck(ack), nil
}
