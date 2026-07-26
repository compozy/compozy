package globaldb

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
)

func nullableBridgeString(value string) sql.NullString {
	return store.SQLNullString(value)
}

func nullableBridgeTimestamp(value time.Time) sql.NullString {
	if value.IsZero() {
		return sql.NullString{}
	}
	return sql.NullString{String: store.FormatTimestamp(value), Valid: true}
}

func bridgeGeneratedString(value any, label string) (string, error) {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed), nil
	case []byte:
		return strings.TrimSpace(string(typed)), nil
	case nil:
		return "", nil
	default:
		return "", fmt.Errorf("store: %s encoded as %T, want string", label, value)
	}
}

func bridgeStringValue(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return strings.TrimSpace(value.String)
}

func nullableBridgeValue(value any, label string) (sql.NullString, error) {
	if value == nil {
		return sql.NullString{}, nil
	}
	text, ok := value.(string)
	if !ok {
		return sql.NullString{}, fmt.Errorf("store: %s encoded as %T, want string", label, value)
	}
	return nullableBridgeString(text), nil
}

func bridgeInstanceInsertParams(record bridgeInstanceRecord) (sqlcgen.InsertBridgeInstanceParams, error) {
	instance := record.instance
	providerConfig, err := nullableBridgeValue(record.providerConfig, "bridge provider config")
	if err != nil {
		return sqlcgen.InsertBridgeInstanceParams{}, err
	}
	deliveryDefaults, err := nullableBridgeValue(record.deliveryDefaults, "bridge delivery defaults")
	if err != nil {
		return sqlcgen.InsertBridgeInstanceParams{}, err
	}
	degradationReason, err := nullableBridgeValue(record.degradationReason, "bridge degradation reason")
	if err != nil {
		return sqlcgen.InsertBridgeInstanceParams{}, err
	}
	degradationMessage, err := nullableBridgeValue(record.degradationMessage, "bridge degradation message")
	if err != nil {
		return sqlcgen.InsertBridgeInstanceParams{}, err
	}
	return sqlcgen.InsertBridgeInstanceParams{
		ID: instance.ID, Scope: string(instance.Scope), WorkspaceID: nullableBridgeString(instance.WorkspaceID),
		Platform: instance.Platform, ExtensionName: instance.ExtensionName, DisplayName: instance.DisplayName,
		Source: string(instance.Source), Enabled: instance.Enabled, Status: string(instance.Status),
		DmPolicy: string(instance.DMPolicy), RoutingPolicy: record.routingPolicyJSON,
		ProviderConfig: providerConfig, DeliveryDefaults: deliveryDefaults,
		NotificationSuppress: instance.NotificationSuppress, DegradationReason: degradationReason,
		DegradationMessage: degradationMessage, CreatedAt: store.FormatTimestamp(instance.CreatedAt),
		UpdatedAt: store.FormatTimestamp(instance.UpdatedAt),
	}, nil
}

func bridgeInstanceUpdateParams(record bridgeInstanceRecord) (sqlcgen.UpdateBridgeInstanceParams, error) {
	insert, err := bridgeInstanceInsertParams(record)
	if err != nil {
		return sqlcgen.UpdateBridgeInstanceParams{}, err
	}
	return sqlcgen.UpdateBridgeInstanceParams{
		Scope: insert.Scope, WorkspaceID: insert.WorkspaceID, Platform: insert.Platform,
		ExtensionName: insert.ExtensionName, DisplayName: insert.DisplayName, Source: insert.Source,
		Enabled: insert.Enabled, Status: insert.Status, DmPolicy: insert.DmPolicy,
		RoutingPolicy: insert.RoutingPolicy, ProviderConfig: insert.ProviderConfig,
		DeliveryDefaults: insert.DeliveryDefaults, NotificationSuppress: insert.NotificationSuppress,
		DegradationReason: insert.DegradationReason, DegradationMessage: insert.DegradationMessage,
		UpdatedAt: insert.UpdatedAt, ID: insert.ID,
	}, nil
}

func bridgeInstanceUpsertParams(record bridgeInstanceRecord) (sqlcgen.UpsertBridgeInstanceParams, error) {
	insert, err := bridgeInstanceInsertParams(record)
	if err != nil {
		return sqlcgen.UpsertBridgeInstanceParams{}, err
	}
	return sqlcgen.UpsertBridgeInstanceParams(insert), nil
}

func bridgeInstanceFromGenerated(row sqlcgen.BridgeInstance) (bridges.BridgeInstance, error) {
	instance := bridges.BridgeInstance{
		ID: row.ID, Scope: bridges.Scope(row.Scope), WorkspaceID: bridgeStringValue(row.WorkspaceID),
		Platform: row.Platform, ExtensionName: row.ExtensionName, DisplayName: row.DisplayName,
		Source: bridges.BridgeInstanceSource(row.Source), Enabled: row.Enabled,
		Status: bridges.BridgeStatus(row.Status), DMPolicy: bridges.BridgeDMPolicy(row.DmPolicy),
		NotificationSuppress: row.NotificationSuppress,
	}
	if err := json.Unmarshal([]byte(row.RoutingPolicy), &instance.RoutingPolicy); err != nil {
		return bridges.BridgeInstance{}, fmt.Errorf("store: decode bridge routing policy: %w", err)
	}
	if row.ProviderConfig.Valid {
		instance.ProviderConfig = json.RawMessage(strings.TrimSpace(row.ProviderConfig.String))
	}
	if row.DeliveryDefaults.Valid {
		instance.DeliveryDefaults = json.RawMessage(strings.TrimSpace(row.DeliveryDefaults.String))
	}
	if row.DegradationReason.Valid || row.DegradationMessage.Valid {
		instance.Degradation = &bridges.BridgeDegradation{
			Reason:  bridges.BridgeDegradationReason(strings.TrimSpace(row.DegradationReason.String)),
			Message: strings.TrimSpace(row.DegradationMessage.String),
		}
	}
	createdAt, err := store.ParseTimestamp(row.CreatedAt)
	if err != nil {
		return bridges.BridgeInstance{}, err
	}
	updatedAt, err := store.ParseTimestamp(row.UpdatedAt)
	if err != nil {
		return bridges.BridgeInstance{}, err
	}
	instance.CreatedAt, instance.UpdatedAt = createdAt, updatedAt
	if err := instance.Validate(); err != nil {
		return bridges.BridgeInstance{}, err
	}
	return instance.Normalized(), nil
}

func bridgeBindingFromGenerated(row sqlcgen.BridgeSecretBinding) (bridges.BridgeSecretBinding, error) {
	createdAt, err := store.ParseTimestamp(row.CreatedAt)
	if err != nil {
		return bridges.BridgeSecretBinding{}, err
	}
	updatedAt, err := store.ParseTimestamp(row.UpdatedAt)
	if err != nil {
		return bridges.BridgeSecretBinding{}, err
	}
	binding := bridges.BridgeSecretBinding{
		BridgeInstanceID: row.BridgeInstanceID, BindingName: row.BindingName,
		SecretRef: row.SecretRef, Kind: row.Kind, CreatedAt: createdAt, UpdatedAt: updatedAt,
	}
	if err := binding.Validate(); err != nil {
		return bridges.BridgeSecretBinding{}, err
	}
	return binding, nil
}

func bridgeRouteFromGenerated(row sqlcgen.BridgeRoute) (bridges.BridgeRoute, error) {
	lastActivityAt, err := store.ParseTimestamp(row.LastActivityAt)
	if err != nil {
		return bridges.BridgeRoute{}, err
	}
	createdAt, err := store.ParseTimestamp(row.CreatedAt)
	if err != nil {
		return bridges.BridgeRoute{}, err
	}
	updatedAt, err := store.ParseTimestamp(row.UpdatedAt)
	if err != nil {
		return bridges.BridgeRoute{}, err
	}
	route := bridges.BridgeRoute{
		RoutingKeyHash: row.RoutingKeyHash, Scope: bridges.Scope(row.Scope),
		WorkspaceID: bridgeStringValue(row.WorkspaceID), BridgeInstanceID: row.BridgeInstanceID,
		PeerID: bridgeStringValue(row.PeerID), ThreadID: bridgeStringValue(row.ThreadID),
		GroupID: bridgeStringValue(row.GroupID), SessionID: row.SessionID, AgentName: row.AgentName,
		LastActivityAt: lastActivityAt, CreatedAt: createdAt, UpdatedAt: updatedAt,
	}
	return route.Canonicalize()
}

func bridgeDedupFromGenerated(row sqlcgen.BridgeIngestDedup) (bridges.IngestDedupRecord, error) {
	receivedAt, err := store.ParseTimestamp(row.ReceivedAt)
	if err != nil {
		return bridges.IngestDedupRecord{}, err
	}
	expiresAt, err := store.ParseTimestamp(row.ExpiresAt)
	if err != nil {
		return bridges.IngestDedupRecord{}, err
	}
	record := bridges.IngestDedupRecord{
		IdempotencyKey: row.IdempotencyKey, BridgeInstanceID: row.BridgeInstanceID,
		ReceivedAt: receivedAt, ExpiresAt: expiresAt,
	}
	if err := record.Validate(); err != nil {
		return bridges.IngestDedupRecord{}, err
	}
	return record, nil
}

func bridgeSubscriptionFromGenerated(row sqlcgen.BridgeTaskSubscription) (bridges.BridgeTaskSubscription, error) {
	createdAt, err := store.ParseTimestamp(row.CreatedAt)
	if err != nil {
		return bridges.BridgeTaskSubscription{}, err
	}
	updatedAt, err := store.ParseTimestamp(row.UpdatedAt)
	if err != nil {
		return bridges.BridgeTaskSubscription{}, err
	}
	subscription := bridges.BridgeTaskSubscription{
		SubscriptionID: row.SubscriptionID, TaskID: row.TaskID, BridgeInstanceID: row.BridgeInstanceID,
		Scope: bridges.Scope(row.Scope), WorkspaceID: bridgeStringValue(row.WorkspaceID),
		PeerID: bridgeStringValue(row.PeerID), ThreadID: bridgeStringValue(row.ThreadID),
		GroupID: bridgeStringValue(row.GroupID), DeliveryMode: bridges.DeliveryMode(row.DeliveryMode),
		CreatedBy: taskpkg.ActorIdentity{Kind: taskpkg.ActorKind(row.CreatedByKind), Ref: row.CreatedByRef},
		CreatedAt: createdAt, UpdatedAt: updatedAt,
	}
	if err := subscription.Validate(); err != nil {
		return bridges.BridgeTaskSubscription{}, err
	}
	return subscription.Normalize(), nil
}

func bridgeTargetFromGenerated(row sqlcgen.BridgeTargetDirectory) (bridges.BridgeTarget, error) {
	capabilities, err := decodeBridgeTargetCapabilities(row.Capabilities)
	if err != nil {
		return bridges.BridgeTarget{}, err
	}
	updatedAt, err := store.ParseTimestamp(row.UpdatedAt)
	if err != nil {
		return bridges.BridgeTarget{}, fmt.Errorf("store: parse bridge target updated_at: %w", err)
	}
	var lastSeenAt time.Time
	if row.LastSeenAt.Valid && strings.TrimSpace(row.LastSeenAt.String) != "" {
		lastSeenAt, err = store.ParseTimestamp(row.LastSeenAt.String)
		if err != nil {
			return bridges.BridgeTarget{}, fmt.Errorf("store: parse bridge target last_seen_at: %w", err)
		}
	}
	target := bridges.BridgeTarget{
		BridgeID: row.BridgeID, CanonicalRoute: row.CanonicalRoute,
		DisplayName: row.DisplayName, Normalized: row.Normalized,
		TargetType: bridges.BridgeTargetType(row.TargetType).Normalize(), Qualifier: row.Qualifier,
		Capabilities: capabilities, UpdatedAt: updatedAt.UTC(), LastSeenAt: lastSeenAt.UTC(),
	}
	return normalizeGlobalDBBridgeTarget(target.BridgeID, target, target.UpdatedAt)
}
