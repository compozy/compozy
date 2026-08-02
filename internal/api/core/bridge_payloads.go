package core

import (
	"fmt"
	"maps"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	bridgepkg "github.com/compozy/compozy/internal/bridges"
	"github.com/compozy/compozy/internal/notifications"
	observepkg "github.com/compozy/compozy/internal/observe"
)

// BridgeAggregateHealthPayloadFromObserve converts the observer bridge
// summary into the shared payload.
func BridgeAggregateHealthPayloadFromObserve(
	summary observepkg.BridgeAggregateHealth,
) contract.BridgeAggregateHealthPayload {
	return contract.BridgeAggregateHealthPayload{
		TotalInstances:        summary.TotalInstances,
		RouteCount:            summary.RouteCount,
		DeliveryBacklog:       summary.DeliveryBacklog,
		DeliveryDroppedTotal:  summary.DeliveryDroppedTotal,
		DeliveryFailuresTotal: summary.DeliveryFailuresTotal,
		AuthFailuresTotal:     summary.AuthFailuresTotal,
		StatusCounts: contract.BridgeStatusCountsPayload{
			Disabled:     summary.StatusCounts.Disabled,
			Starting:     summary.StatusCounts.Starting,
			Ready:        summary.StatusCounts.Ready,
			Degraded:     summary.StatusCounts.Degraded,
			AuthRequired: summary.StatusCounts.AuthRequired,
			Error:        summary.StatusCounts.Error,
		},
	}
}

// BridgeHealthPayloadFromObserve converts the observer per-instance bridge
// health snapshot into the shared payload.
func BridgeHealthPayloadFromObserve(health observepkg.BridgeInstanceHealth) contract.BridgeHealthPayload {
	var lastSuccessAt *time.Time
	if !health.LastSuccessAt.IsZero() {
		timestamp := health.LastSuccessAt
		lastSuccessAt = &timestamp
	}

	var lastErrorAt *time.Time
	if !health.LastErrorAt.IsZero() {
		timestamp := health.LastErrorAt
		lastErrorAt = &timestamp
	}

	return contract.BridgeHealthPayload{
		BridgeInstanceID:        health.BridgeInstanceID,
		Status:                  health.Status,
		RouteCount:              health.RouteCount,
		DeliveryBacklog:         health.DeliveryBacklog,
		DeliveryDroppedTotal:    health.DeliveryDroppedTotal,
		DeliveryDroppedByReason: maps.Clone(health.DeliveryDroppedByReason),
		DeliveryFailuresTotal:   health.DeliveryFailuresTotal,
		AuthFailuresTotal:       health.AuthFailuresTotal,
		LastSuccessAt:           lastSuccessAt,
		LastError:               health.LastError,
		LastErrorAt:             lastErrorAt,
	}
}

// BridgePayloadFromBridgeInstance converts the daemon-owned bridge record into
// the shared bridge-management payload exposed by transports and OpenAPI.
func BridgePayloadFromBridgeInstance(instance bridgepkg.BridgeInstance) contract.BridgePayload {
	webhookPublicURL, err := bridgepkg.WebhookPublicURL(instance)
	if err != nil {
		// Missing or invalid setup is represented by omission; the bridge contract owns validation.
		webhookPublicURL = ""
	}
	return contract.BridgePayload{
		ID:                   instance.ID,
		Scope:                instance.Scope,
		WorkspaceID:          instance.WorkspaceID,
		Platform:             instance.Platform,
		ExtensionName:        instance.ExtensionName,
		DisplayName:          instance.DisplayName,
		Source:               instance.Source,
		Enabled:              instance.Enabled,
		Status:               instance.Status,
		DMPolicy:             instance.DMPolicy,
		RoutingPolicy:        instance.RoutingPolicy,
		ProviderConfig:       contract.BridgeProviderConfigPayload(cloneRawMessage(instance.ProviderConfig)),
		WebhookPublicURL:     webhookPublicURL,
		DeliveryDefaults:     contract.BridgeDeliveryDefaultsPayload(cloneRawMessage(instance.DeliveryDefaults)),
		NotificationSuppress: instance.NotificationSuppress,
		Degradation:          cloneBridgeDegradation(instance.Degradation),
		CreatedAt:            instance.CreatedAt,
		UpdatedAt:            instance.UpdatedAt,
	}
}

// TaskBridgeNotificationSubscriptionPayloadFromSubscription converts one
// bridge task subscription into the shared task-scoped transport payload.
func TaskBridgeNotificationSubscriptionPayloadFromSubscription(
	subscription bridgepkg.BridgeTaskSubscription,
) (contract.TaskBridgeNotificationSubscriptionPayload, error) {
	normalized := subscription.Normalize()
	if err := normalized.Validate(); err != nil {
		return contract.TaskBridgeNotificationSubscriptionPayload{}, fmt.Errorf(
			"api: project invalid task bridge notification subscription: %w",
			err,
		)
	}
	cursor, err := TaskBridgeNotificationCursorPayloadFromKey(normalized.CursorKey())
	if err != nil {
		return contract.TaskBridgeNotificationSubscriptionPayload{}, err
	}
	return contract.TaskBridgeNotificationSubscriptionPayload{
		SubscriptionID:   normalized.SubscriptionID,
		TaskID:           normalized.TaskID,
		BridgeInstanceID: normalized.BridgeInstanceID,
		Scope:            normalized.Scope,
		WorkspaceID:      normalized.WorkspaceID,
		PeerID:           normalized.PeerID,
		ThreadID:         normalized.ThreadID,
		GroupID:          normalized.GroupID,
		DeliveryMode:     normalized.DeliveryMode,
		Cursor:           cursor,
		CreatedBy:        normalized.CreatedBy,
		CreatedAt:        normalized.CreatedAt,
		UpdatedAt:        normalized.UpdatedAt,
	}, nil
}

// TaskBridgeNotificationSubscriptionPayloadFromSubscriptionAndCursor converts
// one bridge task subscription with its persisted cursor diagnostics.
func TaskBridgeNotificationSubscriptionPayloadFromSubscriptionAndCursor(
	subscription bridgepkg.BridgeTaskSubscription,
	cursor notifications.Cursor,
) (contract.TaskBridgeNotificationSubscriptionPayload, error) {
	payload, err := TaskBridgeNotificationSubscriptionPayloadFromSubscription(subscription)
	if err != nil {
		return contract.TaskBridgeNotificationSubscriptionPayload{}, err
	}
	expectedKey, err := subscription.CursorKey().Normalize()
	if err != nil {
		return contract.TaskBridgeNotificationSubscriptionPayload{}, err
	}
	actualKey, err := cursor.Key.Normalize()
	if err != nil {
		return contract.TaskBridgeNotificationSubscriptionPayload{}, err
	}
	if actualKey != expectedKey {
		return contract.TaskBridgeNotificationSubscriptionPayload{}, fmt.Errorf(
			"%w: persisted task bridge notification cursor does not match subscription %q",
			notifications.ErrInvalidCursor,
			subscription.SubscriptionID,
		)
	}
	cursorPayload, err := TaskBridgeNotificationCursorPayloadFromCursor(cursor)
	if err != nil {
		return contract.TaskBridgeNotificationSubscriptionPayload{}, err
	}
	payload.Cursor = cursorPayload
	return payload, nil
}

// TaskBridgeNotificationSubscriptionPayloadsFromSubscriptions converts
// bridge task subscriptions into shared task-scoped transport payloads.
func TaskBridgeNotificationSubscriptionPayloadsFromSubscriptions(
	subscriptions []bridgepkg.BridgeTaskSubscription,
) ([]contract.TaskBridgeNotificationSubscriptionPayload, error) {
	payloads := make([]contract.TaskBridgeNotificationSubscriptionPayload, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		payload, err := TaskBridgeNotificationSubscriptionPayloadFromSubscription(subscription)
		if err != nil {
			return nil, err
		}
		payloads = append(payloads, payload)
	}
	return payloads, nil
}

// TaskBridgeNotificationCursorPayloadFromKey converts a durable cursor identity
// into the transport diagnostics shape before any delivery has been persisted.
func TaskBridgeNotificationCursorPayloadFromKey(
	key notifications.CursorKey,
) (contract.TaskBridgeNotificationCursorPayload, error) {
	normalized, err := key.Normalize()
	if err != nil {
		return contract.TaskBridgeNotificationCursorPayload{}, fmt.Errorf(
			"api: project invalid task bridge notification cursor: %w",
			err,
		)
	}
	return contract.TaskBridgeNotificationCursorPayload{
		Scope:        normalized.Scope,
		ConsumerID:   normalized.ConsumerID,
		StreamName:   normalized.StreamName,
		SubjectID:    normalized.SubjectID,
		LastSequence: 0,
	}, nil
}

// TaskBridgeNotificationCursorPayloadFromCursor converts persisted cursor
// diagnostics into the transport payload used by HTTP, UDS, CLI, and web.
func TaskBridgeNotificationCursorPayloadFromCursor(
	cursor notifications.Cursor,
) (contract.TaskBridgeNotificationCursorPayload, error) {
	payload, err := TaskBridgeNotificationCursorPayloadFromKey(cursor.Key)
	if err != nil {
		return contract.TaskBridgeNotificationCursorPayload{}, err
	}
	payload.LastSequence = cursor.LastSequence
	payload.LastDeliveryID = cursor.LastDeliveryID
	payload.LastError = cursor.LastError
	if !cursor.LastDeliveredAt.IsZero() {
		lastDeliveredAt := cursor.LastDeliveredAt.UTC()
		payload.LastDeliveredAt = &lastDeliveredAt
	}
	if !cursor.UpdatedAt.IsZero() {
		updatedAt := cursor.UpdatedAt.UTC()
		payload.UpdatedAt = &updatedAt
	}
	return payload, nil
}

// BridgeProviderPayloadFromBridgeProvider converts installed provider metadata
// into the shared bridge-management provider catalog payload.
func BridgeProviderPayloadFromBridgeProvider(provider bridgepkg.BridgeProvider) contract.BridgeProviderPayload {
	var configSchema *bridgepkg.BridgeProviderConfigSchema
	if provider.ConfigSchema != nil {
		cloned := *provider.ConfigSchema
		configSchema = &cloned
	}

	secretSlots := make([]bridgepkg.BridgeSecretSlot, 0, len(provider.SecretSlots))
	secretSlots = append(secretSlots, provider.SecretSlots...)

	return contract.BridgeProviderPayload{
		Platform:      provider.Platform,
		ExtensionName: provider.ExtensionName,
		DisplayName:   provider.DisplayName,
		Description:   provider.Description,
		SecretSlots:   secretSlots,
		ConfigSchema:  configSchema,
		Enabled:       provider.Enabled,
		State:         provider.State,
		Health:        provider.Health,
		HealthMessage: provider.HealthMessage,
	}
}
