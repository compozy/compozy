package globaldb

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	aghworkspace "github.com/compozy/agh/internal/workspace"
)

func normalizeBridgeInstanceRecord(
	instance bridges.BridgeInstance,
) (bridges.BridgeInstance, string, any, any, any, any, error) {
	normalized := instance.Normalized()
	if err := normalized.Validate(); err != nil {
		return bridges.BridgeInstance{}, "", nil, nil, nil, nil, err
	}

	routingPolicyJSON, err := json.Marshal(normalized.RoutingPolicy)
	if err != nil {
		return bridges.BridgeInstance{}, "", nil, nil, nil, nil, fmt.Errorf(
			"store: encode bridge routing policy: %w",
			err,
		)
	}

	providerConfig, err := normalizeOptionalRawJSON(normalized.ProviderConfig)
	if err != nil {
		return bridges.BridgeInstance{}, "", nil, nil, nil, nil, fmt.Errorf(
			"store: encode bridge provider config: %w",
			err,
		)
	}

	deliveryDefaults, err := normalizeOptionalRawJSON(normalized.DeliveryDefaults)
	if err != nil {
		return bridges.BridgeInstance{}, "", nil, nil, nil, nil, fmt.Errorf(
			"store: encode bridge delivery defaults: %w",
			err,
		)
	}

	var degradationReason any
	var degradationMessage any
	if normalized.Degradation != nil && !normalized.Degradation.IsZero() {
		degradationReason = string(normalized.Degradation.Reason.Normalize())
		degradationMessage = store.NullableString(normalized.Degradation.Message)
	}

	return normalized, string(
		routingPolicyJSON,
	), providerConfig, deliveryDefaults, degradationReason, degradationMessage, nil
}

func normalizeOptionalRawJSON(value json.RawMessage) (any, error) {
	trimmed := strings.TrimSpace(string(value))
	if trimmed == "" {
		return nil, nil
	}
	if !json.Valid([]byte(trimmed)) {
		return nil, errors.New("invalid JSON payload")
	}
	return trimmed, nil
}

func scanBridgeInstance(scanner rowScanner) (bridges.BridgeInstance, error) {
	var (
		instance             bridges.BridgeInstance
		scopeRaw             string
		workspaceID          sql.NullString
		sourceRaw            string
		enabled              bool
		statusRaw            string
		dmPolicyRaw          string
		routingPolicyRaw     string
		providerConfigRaw    sql.NullString
		deliveryDefaultsRaw  sql.NullString
		notificationSuppress bool
		degradationReason    sql.NullString
		degradationMessage   sql.NullString
		createdAtRaw         string
		updatedAtRaw         string
	)
	if err := scanner.Scan(
		&instance.ID,
		&scopeRaw,
		&workspaceID,
		&instance.Platform,
		&instance.ExtensionName,
		&instance.DisplayName,
		&sourceRaw,
		&enabled,
		&statusRaw,
		&dmPolicyRaw,
		&routingPolicyRaw,
		&providerConfigRaw,
		&deliveryDefaultsRaw,
		&notificationSuppress,
		&degradationReason,
		&degradationMessage,
		&createdAtRaw,
		&updatedAtRaw,
	); err != nil {
		return bridges.BridgeInstance{}, fmt.Errorf("store: scan bridge instance: %w", err)
	}

	instance.Scope = bridges.Scope(scopeRaw)
	if value := store.NullString(workspaceID); value != nil {
		instance.WorkspaceID = *value
	}
	instance.Source = bridges.BridgeInstanceSource(sourceRaw)
	instance.Enabled = enabled
	instance.Status = bridges.BridgeStatus(statusRaw)
	instance.DMPolicy = bridges.BridgeDMPolicy(dmPolicyRaw)
	if err := json.Unmarshal([]byte(routingPolicyRaw), &instance.RoutingPolicy); err != nil {
		return bridges.BridgeInstance{}, fmt.Errorf("store: decode bridge routing policy: %w", err)
	}
	if providerConfigRaw.Valid {
		instance.ProviderConfig = json.RawMessage(strings.TrimSpace(providerConfigRaw.String))
	}
	if deliveryDefaultsRaw.Valid {
		instance.DeliveryDefaults = json.RawMessage(strings.TrimSpace(deliveryDefaultsRaw.String))
	}
	instance.NotificationSuppress = notificationSuppress
	if degradationReason.Valid || degradationMessage.Valid {
		instance.Degradation = &bridges.BridgeDegradation{
			Reason:  bridges.BridgeDegradationReason(strings.TrimSpace(degradationReason.String)),
			Message: strings.TrimSpace(degradationMessage.String),
		}
	}

	createdAt, err := store.ParseTimestamp(createdAtRaw)
	if err != nil {
		return bridges.BridgeInstance{}, err
	}
	updatedAt, err := store.ParseTimestamp(updatedAtRaw)
	if err != nil {
		return bridges.BridgeInstance{}, err
	}
	instance.CreatedAt = createdAt
	instance.UpdatedAt = updatedAt

	if err := instance.Validate(); err != nil {
		return bridges.BridgeInstance{}, err
	}
	return instance.Normalized(), nil
}

func scanBridgeSecretBinding(scanner rowScanner) (bridges.BridgeSecretBinding, error) {
	var (
		binding      bridges.BridgeSecretBinding
		createdAtRaw string
		updatedAtRaw string
	)
	if err := scanner.Scan(
		&binding.BridgeInstanceID,
		&binding.BindingName,
		&binding.SecretRef,
		&binding.Kind,
		&createdAtRaw,
		&updatedAtRaw,
	); err != nil {
		return bridges.BridgeSecretBinding{}, fmt.Errorf("store: scan bridge secret binding: %w", err)
	}

	createdAt, err := store.ParseTimestamp(createdAtRaw)
	if err != nil {
		return bridges.BridgeSecretBinding{}, err
	}
	updatedAt, err := store.ParseTimestamp(updatedAtRaw)
	if err != nil {
		return bridges.BridgeSecretBinding{}, err
	}
	binding.CreatedAt = createdAt
	binding.UpdatedAt = updatedAt

	if err := binding.Validate(); err != nil {
		return bridges.BridgeSecretBinding{}, err
	}
	return binding, nil
}

func scanBridgeTaskSubscription(scanner rowScanner) (bridges.BridgeTaskSubscription, error) {
	var (
		subscription  bridges.BridgeTaskSubscription
		scopeRaw      string
		workspaceID   sql.NullString
		peerID        sql.NullString
		threadID      sql.NullString
		groupID       sql.NullString
		deliveryMode  string
		createdByKind string
		createdAtRaw  string
		updatedAtRaw  string
	)
	if err := scanner.Scan(
		&subscription.SubscriptionID,
		&subscription.TaskID,
		&subscription.BridgeInstanceID,
		&scopeRaw,
		&workspaceID,
		&peerID,
		&threadID,
		&groupID,
		&deliveryMode,
		&createdByKind,
		&subscription.CreatedBy.Ref,
		&createdAtRaw,
		&updatedAtRaw,
	); err != nil {
		return bridges.BridgeTaskSubscription{}, fmt.Errorf("store: scan bridge task subscription: %w", err)
	}
	subscription.Scope = bridges.Scope(scopeRaw)
	if value := store.NullString(workspaceID); value != nil {
		subscription.WorkspaceID = *value
	}
	if value := store.NullString(peerID); value != nil {
		subscription.PeerID = *value
	}
	if value := store.NullString(threadID); value != nil {
		subscription.ThreadID = *value
	}
	if value := store.NullString(groupID); value != nil {
		subscription.GroupID = *value
	}
	subscription.DeliveryMode = bridges.DeliveryMode(deliveryMode)
	subscription.CreatedBy.Kind = taskpkg.ActorKind(createdByKind)

	createdAt, err := store.ParseTimestamp(createdAtRaw)
	if err != nil {
		return bridges.BridgeTaskSubscription{}, err
	}
	updatedAt, err := store.ParseTimestamp(updatedAtRaw)
	if err != nil {
		return bridges.BridgeTaskSubscription{}, err
	}
	subscription.CreatedAt = createdAt
	subscription.UpdatedAt = updatedAt
	if err := subscription.Validate(); err != nil {
		return bridges.BridgeTaskSubscription{}, err
	}
	return subscription.Normalize(), nil
}

func mapBridgeInstanceConstraintError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case isSQLiteForeignKeyConstraint(err):
		return aghworkspace.ErrWorkspaceNotFound
	default:
		return err
	}
}

func mapBridgeChildConstraintError(err error) error {
	if err == nil {
		return nil
	}

	if isSQLiteForeignKeyConstraint(err) {
		return bridges.ErrBridgeInstanceNotFound
	}
	return err
}
