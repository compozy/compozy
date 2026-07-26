package presets

import (
	"context"
	"encoding/json"

	"fmt"

	bridgepkg "github.com/compozy/agh/internal/bridges"

	"github.com/compozy/agh/internal/notifications"
)

func (s *Service) deliveryForTarget(
	ctx context.Context,
	preset Preset,
	event Event,
	normalizedTarget Target,
	instance bridgepkg.BridgeInstance,
	deliveryID string,
) (bridgepkg.DeliveryEvent, error) {
	resolved, err := s.resolveTarget(ctx, normalizedTarget)
	if err != nil {
		return bridgepkg.DeliveryEvent{}, err
	}
	deliveryTarget, routingKey, err := deliveryTargetFromBridgeTarget(
		instance,
		*resolved,
		normalizedTarget,
	)
	if err != nil {
		return bridgepkg.DeliveryEvent{}, err
	}
	metadata, err := json.Marshal(struct {
		Preset string `json:"preset"`
		Event  Event  `json:"event"`
		Target Target `json:"target"`
	}{
		Preset: preset.Name,
		Event:  event,
		Target: normalizedTarget,
	})
	if err != nil {
		return bridgepkg.DeliveryEvent{}, fmt.Errorf(
			"notifications: encode preset delivery metadata: %w",
			err,
		)
	}
	delivery := bridgepkg.DeliveryEvent{
		DeliveryID:       deliveryID,
		BridgeInstanceID: instance.ID,
		RoutingKey:       routingKey,
		DeliveryTarget:   deliveryTarget,
		Seq:              event.Sequence,
		EventType:        bridgepkg.DeliveryEventTypeFinal,
		Content:          bridgepkg.MessageContent{Text: notificationText(preset, event)},
		Final:            true,
		Operation:        bridgepkg.DeliveryOperationPost,
		ProviderMetadata: metadata,
	}
	if err := delivery.Validate(); err != nil {
		return bridgepkg.DeliveryEvent{}, err
	}
	return delivery, nil
}

func (s *Service) deliverBridgeEvent(
	ctx context.Context,
	preset Preset,
	event Event,
	instance bridgepkg.BridgeInstance,
	delivery bridgepkg.DeliveryEvent,
) error {
	deliveryCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	ack, err := s.bridges.DeliverBridge(
		deliveryCtx,
		instance.ExtensionName,
		bridgepkg.DeliveryRequest{Event: delivery},
	)
	if err != nil {
		return fmt.Errorf(
			"notifications: deliver preset %q event %q to bridge %q: %w",
			preset.Name,
			event.ID,
			instance.ID,
			err,
		)
	}
	if err := ack.ValidateFor(delivery); err != nil {
		return fmt.Errorf("notifications: validate preset %q delivery ack: %w", preset.Name, err)
	}
	return nil
}

func (s *Service) resolveTarget(
	ctx context.Context,
	target Target,
) (*bridgepkg.BridgeTarget, error) {
	query := target.CanonicalRoute
	if query == "" {
		query = target.DisplayName
	}
	resolved, err := s.bridges.ResolveBridgeTarget(ctx, target.BridgeID, query)
	if err != nil {
		return nil, fmt.Errorf(
			"notifications: resolve bridge target %q on %q: %w",
			query,
			target.BridgeID,
			err,
		)
	}
	if resolved.Match == nil {
		return nil, fmt.Errorf(
			"notifications: resolve bridge target %q on %q: %w",
			query,
			target.BridgeID,
			bridgepkg.ErrBridgeTargetUnknown,
		)
	}
	return resolved.Match, nil
}

func (s *Service) advance(
	ctx context.Context,
	key notifications.CursorKey,
	event Event,
	deliveryID string,
) (notifications.Cursor, error) {
	return s.cursors.Advance(ctx, notifications.AdvanceCursor{
		Key:          key,
		LastSequence: event.Sequence,
		DeliveryID:   deliveryID,
		Now:          s.now(),
	})
}
