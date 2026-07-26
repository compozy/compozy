package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
	"github.com/compozy/agh/internal/subprocess"
)

// DeliverBridge calls the negotiated `bridges/deliver` service on the named
// bridge-capable extension runtime.
func (m *Manager) DeliverBridge(
	ctx context.Context,
	extensionName string,
	req bridgepkg.DeliveryRequest,
) (bridgepkg.DeliveryAck, error) {
	if ctx == nil {
		return bridgepkg.DeliveryAck{}, errors.New("extension: delivery context is required")
	}
	if err := ctx.Err(); err != nil {
		return bridgepkg.DeliveryAck{}, err
	}
	if m == nil {
		return bridgepkg.DeliveryAck{}, ErrManagerRequired
	}
	if err := req.Validate(); err != nil {
		return bridgepkg.DeliveryAck{}, err
	}

	name := strings.TrimSpace(extensionName)
	if name == "" {
		return bridgepkg.DeliveryAck{}, errors.New("extension: delivery extension name is required")
	}

	m.mu.RLock()
	ext := m.extensions[name]
	if ext == nil || ext.process == nil || !ext.active {
		m.mu.RUnlock()
		return bridgepkg.DeliveryAck{}, bridgepkg.ErrDeliveryTransportUnavailable
	}
	process := ext.process
	initialize := cloneInitializeResponse(ext.initialize)
	m.mu.RUnlock()

	if initialize == nil ||
		!slices.Contains(
			initialize.ImplementedMethods,
			string(extensionprotocol.ExtensionServiceMethodBridgesDeliver),
		) {
		return bridgepkg.DeliveryAck{}, fmt.Errorf(
			"extension: extension %q does not implement %q: %w",
			name,
			extensionprotocol.ExtensionServiceMethodBridgesDeliver,
			bridgepkg.ErrDeliveryTransportUnavailable,
		)
	}

	var ack bridgepkg.DeliveryAck
	if err := process.Call(
		ctx,
		string(extensionprotocol.ExtensionServiceMethodBridgesDeliver),
		req,
		&ack,
	); err != nil {
		if decodeErr, ok := errors.AsType[*subprocess.ResponseDecodeError](err); ok && decodeErr != nil {
			return indeterminateBridgeDeliveryAck(req), nil
		}
		return bridgepkg.DeliveryAck{}, fmt.Errorf("extension: deliver bridge via %q: %w", name, err)
	}
	if err := ack.ValidateFor(req.Event); err != nil {
		return indeterminateBridgeDeliveryAck(req), nil
	}
	return ack, nil
}

func indeterminateBridgeDeliveryAck(req bridgepkg.DeliveryRequest) bridgepkg.DeliveryAck {
	return bridgepkg.DeliveryAck{
		DeliveryID: req.Event.DeliveryID,
		Seq:        req.Event.Seq,
		Outcome:    bridgepkg.DeliveryAckOutcomeCommittedResultUnavailable,
		Error: &bridgepkg.DeliveryErrorDetail{
			Message: bridgepkg.DeliveryResultUnavailableMessage,
		},
	}
}
