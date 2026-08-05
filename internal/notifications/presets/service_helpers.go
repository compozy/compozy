package presets

import (
	"context"
	"fmt"
	"strings"
	"time"

	bridgepkg "github.com/compozy/compozy/internal/bridges"
	eventspkg "github.com/compozy/compozy/internal/events"
)

func normalizeUpdateRequest(req UpdateRequest, now time.Time) UpdateRequest {
	normalized := req
	if req.Events != nil {
		events := normalizePresetEvents(*req.Events)
		normalized.Events = &events
	}
	if req.Targets != nil {
		targets := normalizePresetTargets(*req.Targets)
		normalized.Targets = &targets
	}
	if req.Filter != nil {
		value := strings.TrimSpace(*req.Filter)
		normalized.Filter = &value
	}
	if normalized.Now.IsZero() {
		normalized.Now = now.UTC()
	} else {
		normalized.Now = normalized.Now.UTC()
	}
	return normalized
}

func notificationText(preset Preset, event Event) string {
	summary := strings.TrimSpace(event.Summary)
	if summary == "" {
		summary = event.Type
	}
	return fmt.Sprintf("Compozy %s: %s", preset.Name, summary)
}

func notificationPresetLifecycleSummary(eventType string, name string) string {
	normalizedName := strings.TrimSpace(name)
	if normalizedName == "" {
		normalizedName = "unknown"
	}
	switch eventType {
	case eventspkg.NotificationPresetCreated:
		return fmt.Sprintf("notification preset %s created", normalizedName)
	case eventspkg.NotificationPresetUpdated:
		return fmt.Sprintf("notification preset %s updated", normalizedName)
	case eventspkg.NotificationPresetDeleted:
		return fmt.Sprintf("notification preset %s deleted", normalizedName)
	default:
		return fmt.Sprintf("notification preset %s changed", normalizedName)
	}
}

func detachedContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return context.WithoutCancel(ctx)
}

func deliveryTargetFromBridgeTarget(
	instance bridgepkg.BridgeInstance,
	target bridgepkg.BridgeTarget,
	presetTarget Target,
) (bridgepkg.DeliveryTarget, bridgepkg.RoutingKey, error) {
	req := bridgepkg.ResolveDeliveryTargetRequest{
		BridgeInstanceID: instance.ID,
		Mode:             presetTarget.DeliveryMode,
	}
	switch target.TargetType.Normalize() {
	case bridgepkg.BridgeTargetTypeUser:
		req.PeerID = target.CanonicalRoute
	case bridgepkg.BridgeTargetTypeThread:
		req.ThreadID = target.CanonicalRoute
		if strings.TrimSpace(target.Qualifier) != "" {
			req.GroupID = target.Qualifier
		}
	default:
		req.GroupID = target.CanonicalRoute
	}
	deliveryTarget, err := bridgepkg.BuildDeliveryTarget(instance, req)
	if err != nil {
		return bridgepkg.DeliveryTarget{}, bridgepkg.RoutingKey{}, err
	}
	routingKey := bridgepkg.RoutingKey{
		Scope:            instance.Scope,
		WorkspaceID:      instance.WorkspaceID,
		BridgeInstanceID: instance.ID,
		PeerID:           deliveryTarget.PeerID,
		ThreadID:         deliveryTarget.ThreadID,
		GroupID:          deliveryTarget.GroupID,
	}
	return deliveryTarget, routingKey, nil
}
