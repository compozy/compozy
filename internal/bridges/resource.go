package bridges

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/store"
)

const (
	// BridgeInstanceResourceKind is the canonical desired-state kind for bridge instances.
	BridgeInstanceResourceKind resources.ResourceKind = "bridge.instance"

	bridgeInstanceResourceMaxBytes = 256 << 10
)

// BridgeProviderLookup resolves provider-authored bridge manifest metadata for resource validation.
type BridgeProviderLookup func(context.Context, string) (BridgeProvider, bool, error)

// BridgeInstanceSpec is the canonical desired-state payload for bridge.instance records.
//
// Runtime status, degradation, routes, delivery state, and assigned-instance reporting stay in
// the bridge runtime store. This spec carries only desired configuration plus provider manifest
// metadata that must be validated with the provider before persistence.
type BridgeInstanceSpec struct {
	Scope                Scope                       `json:"scope,omitempty"`
	WorkspaceID          string                      `json:"workspace_id,omitempty"`
	Platform             string                      `json:"platform"`
	ExtensionName        string                      `json:"extension_name"`
	DisplayName          string                      `json:"display_name"`
	Source               BridgeInstanceSource        `json:"source,omitempty"`
	Enabled              bool                        `json:"enabled"`
	DMPolicy             BridgeDMPolicy              `json:"dm_policy,omitempty"`
	RoutingPolicy        RoutingPolicy               `json:"routing_policy"`
	ProviderConfig       json.RawMessage             `json:"provider_config,omitempty"`
	DeliveryDefaults     json.RawMessage             `json:"delivery_defaults,omitempty"`
	NotificationSuppress bool                        `json:"notification_suppress"`
	SecretSlots          []BridgeSecretSlot          `json:"secret_slots,omitempty"`
	ConfigSchema         *BridgeProviderConfigSchema `json:"config_schema,omitempty"`
}

// NewBridgeInstanceResourceCodec builds the typed codec for bridge.instance records.
func NewBridgeInstanceResourceCodec(
	providerLookup BridgeProviderLookup,
) (resources.KindCodec[BridgeInstanceSpec], error) {
	validator := func(
		ctx context.Context,
		scope resources.ResourceScope,
		spec BridgeInstanceSpec,
	) (BridgeInstanceSpec, error) {
		return validateBridgeInstanceResourceSpec(ctx, scope, spec, providerLookup)
	}
	return resources.NewJSONCodec(BridgeInstanceResourceKind, bridgeInstanceResourceMaxBytes, validator)
}

// ResourceScopeForBridge converts bridge scope fields into the shared resource scope.
func ResourceScopeForBridge(scope Scope, workspaceID string) resources.ResourceScope {
	switch scope.Normalize() {
	case ScopeWorkspace:
		return resources.ResourceScope{
			Kind: resources.ResourceScopeKindWorkspace,
			ID:   strings.TrimSpace(workspaceID),
		}
	default:
		return resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal}
	}
}

// BridgeInstanceSpecFromCreateRequest converts a transport/domain create request into desired resource state.
func BridgeInstanceSpecFromCreateRequest(
	req CreateInstanceRequest,
	now func() time.Time,
) (string, BridgeInstanceSpec, error) {
	instance, err := req.toInstance(now)
	if err != nil {
		return "", BridgeInstanceSpec{}, err
	}
	return instance.ID, BridgeInstanceSpecFromInstance(instance), nil
}

// BridgeInstanceSpecFromInstance strips bridge-owned operational fields from a bridge instance.
func BridgeInstanceSpecFromInstance(instance BridgeInstance) BridgeInstanceSpec {
	normalized := instance.normalize()
	return BridgeInstanceSpec{
		Scope:                normalized.Scope,
		WorkspaceID:          normalized.WorkspaceID,
		Platform:             normalized.Platform,
		ExtensionName:        normalized.ExtensionName,
		DisplayName:          normalized.DisplayName,
		Source:               normalized.Source,
		Enabled:              normalized.Enabled,
		DMPolicy:             normalized.DMPolicy,
		RoutingPolicy:        normalized.RoutingPolicy,
		ProviderConfig:       cloneRawJSON(normalized.ProviderConfig),
		DeliveryDefaults:     cloneRawJSON(normalized.DeliveryDefaults),
		NotificationSuppress: normalized.NotificationSuppress,
	}
}

func validateBridgeInstanceResourceSpec(
	ctx context.Context,
	scope resources.ResourceScope,
	spec BridgeInstanceSpec,
	providerLookup BridgeProviderLookup,
) (BridgeInstanceSpec, error) {
	if ctx == nil {
		return BridgeInstanceSpec{}, errors.New("bridges: bridge resource validation context is required")
	}
	normalizedScope := scope.Normalize()
	if err := normalizedScope.Validate("scope"); err != nil {
		return BridgeInstanceSpec{}, err
	}

	next := normalizeBridgeInstanceResourceSpec(spec)
	if err := bindBridgeResourceScope(&next.Scope, &next.WorkspaceID, normalizedScope); err != nil {
		return BridgeInstanceSpec{}, err
	}
	var err error
	next, err = validateBridgeInstanceDesiredFields(next)
	if err != nil {
		return BridgeInstanceSpec{}, err
	}
	if err := validateBridgeProviderMetadata(ctx, &next, providerLookup); err != nil {
		return BridgeInstanceSpec{}, err
	}
	return next, nil
}

func normalizeBridgeInstanceResourceSpec(spec BridgeInstanceSpec) BridgeInstanceSpec {
	next := spec
	next.Scope = next.Scope.Normalize()
	next.WorkspaceID = strings.TrimSpace(next.WorkspaceID)
	next.Platform = strings.TrimSpace(next.Platform)
	next.ExtensionName = strings.TrimSpace(next.ExtensionName)
	next.DisplayName = strings.TrimSpace(next.DisplayName)
	next.Source = next.Source.Normalize()
	if next.Source == "" {
		next.Source = BridgeInstanceSourceDynamic
	}
	next.DMPolicy = next.DMPolicy.Normalize()
	if next.DMPolicy == "" {
		next.DMPolicy = BridgeDMPolicyOpen
	}
	next.ProviderConfig = bytes.TrimSpace(next.ProviderConfig)
	next.DeliveryDefaults = bytes.TrimSpace(next.DeliveryDefaults)
	next.SecretSlots = normalizeBridgeSecretSlotsForResource(next.SecretSlots)
	if next.ConfigSchema != nil {
		normalized := next.ConfigSchema.Normalize()
		if normalized.IsZero() {
			next.ConfigSchema = nil
		} else {
			next.ConfigSchema = &normalized
		}
	}
	return next
}

func bindBridgeResourceScope(
	domainScope *Scope,
	workspaceID *string,
	resourceScope resources.ResourceScope,
) error {
	switch resourceScope.Kind {
	case resources.ResourceScopeKindGlobal:
		if *domainScope == "" {
			*domainScope = ScopeGlobal
		}
		if *domainScope != ScopeGlobal {
			return fmt.Errorf(
				"%w: bridge.scope %q does not match resource scope %q",
				resources.ErrInvalidScopeBinding,
				*domainScope,
				resourceScope.Kind,
			)
		}
		if strings.TrimSpace(*workspaceID) != "" {
			return fmt.Errorf(
				"%w: bridge.workspace_id must be empty for global resource scope",
				resources.ErrInvalidScopeBinding,
			)
		}
		*workspaceID = ""
	case resources.ResourceScopeKindWorkspace:
		if *domainScope == "" {
			*domainScope = ScopeWorkspace
		}
		if *domainScope != ScopeWorkspace {
			return fmt.Errorf(
				"%w: bridge.scope %q does not match resource scope %q",
				resources.ErrInvalidScopeBinding,
				*domainScope,
				resourceScope.Kind,
			)
		}
		trimmedWorkspaceID := strings.TrimSpace(*workspaceID)
		switch {
		case trimmedWorkspaceID == "":
			*workspaceID = resourceScope.ID
		case trimmedWorkspaceID != resourceScope.ID:
			return fmt.Errorf(
				"%w: bridge.workspace_id %q does not match resource scope %q",
				resources.ErrInvalidScopeBinding,
				trimmedWorkspaceID,
				resourceScope.ID,
			)
		default:
			*workspaceID = trimmedWorkspaceID
		}
	}
	return nil
}

func validateBridgeInstanceDesiredFields(spec BridgeInstanceSpec) (BridgeInstanceSpec, error) {
	if err := ValidateScopeWorkspaceID(spec.Scope, spec.WorkspaceID); err != nil {
		return BridgeInstanceSpec{}, err
	}
	if err := requireField(spec.Platform, "bridge instance platform"); err != nil {
		return BridgeInstanceSpec{}, err
	}
	if err := requireField(spec.ExtensionName, "bridge instance extension name"); err != nil {
		return BridgeInstanceSpec{}, err
	}
	if err := requireField(spec.DisplayName, "bridge instance display name"); err != nil {
		return BridgeInstanceSpec{}, err
	}
	if err := spec.Source.Validate(); err != nil {
		return BridgeInstanceSpec{}, err
	}
	if err := spec.DMPolicy.Validate(); err != nil {
		return BridgeInstanceSpec{}, err
	}
	if err := spec.RoutingPolicy.Validate(); err != nil {
		return BridgeInstanceSpec{}, err
	}
	providerConfig, err := normalizeOptionalJSONObject(spec.ProviderConfig, "bridge instance provider config")
	if err != nil {
		return BridgeInstanceSpec{}, err
	}
	spec.ProviderConfig = providerConfig
	deliveryDefaults, err := NormalizeDeliveryDefaultsJSON(spec.DeliveryDefaults)
	if err != nil {
		return BridgeInstanceSpec{}, err
	}
	spec.DeliveryDefaults = deliveryDefaults
	for _, slot := range spec.SecretSlots {
		if err := slot.Validate(); err != nil {
			return BridgeInstanceSpec{}, err
		}
	}
	if spec.ConfigSchema != nil {
		if err := spec.ConfigSchema.Validate(); err != nil {
			return BridgeInstanceSpec{}, err
		}
	}
	return spec, nil
}

func validateBridgeProviderMetadata(
	ctx context.Context,
	spec *BridgeInstanceSpec,
	providerLookup BridgeProviderLookup,
) error {
	if providerLookup == nil {
		return nil
	}
	provider, ok, err := providerLookup(ctx, spec.ExtensionName)
	if err != nil {
		return fmt.Errorf("bridges: lookup bridge provider %q: %w", spec.ExtensionName, err)
	}
	if !ok {
		return fmt.Errorf("bridges: bridge provider %q is not installed", spec.ExtensionName)
	}

	expectedPlatform := strings.TrimSpace(provider.Platform)
	if expectedPlatform == "" {
		return fmt.Errorf("bridges: bridge provider %q has no platform", spec.ExtensionName)
	}
	if spec.Platform != expectedPlatform {
		return fmt.Errorf(
			"bridges: bridge provider %q platform %q does not match resource platform %q",
			spec.ExtensionName,
			expectedPlatform,
			spec.Platform,
		)
	}

	expectedSlots := normalizeBridgeSecretSlotsForResource(provider.SecretSlots)
	if len(spec.SecretSlots) == 0 {
		spec.SecretSlots = expectedSlots
	} else if !slices.Equal(spec.SecretSlots, expectedSlots) {
		return fmt.Errorf(
			"bridges: bridge provider %q secret_slots metadata does not match manifest",
			spec.ExtensionName,
		)
	}

	expectedSchema := normalizeBridgeProviderConfigSchemaPointer(provider.ConfigSchema)
	if spec.ConfigSchema == nil {
		spec.ConfigSchema = expectedSchema
	} else if !sameBridgeProviderConfigSchema(spec.ConfigSchema, expectedSchema) {
		return fmt.Errorf(
			"bridges: bridge provider %q config_schema metadata does not match manifest",
			spec.ExtensionName,
		)
	}
	return nil
}

func normalizeBridgeSecretSlotsForResource(slots []BridgeSecretSlot) []BridgeSecretSlot {
	if len(slots) == 0 {
		return nil
	}
	normalized := make([]BridgeSecretSlot, 0, len(slots))
	for _, slot := range slots {
		next := slot.Normalize()
		if next.Name == "" && next.Description == "" && !next.Required {
			continue
		}
		normalized = append(normalized, next)
	}
	slices.SortFunc(normalized, func(left BridgeSecretSlot, right BridgeSecretSlot) int {
		if byName := strings.Compare(left.Name, right.Name); byName != 0 {
			return byName
		}
		return strings.Compare(left.Description, right.Description)
	})
	return normalized
}

func normalizeBridgeProviderConfigSchemaPointer(
	schema *BridgeProviderConfigSchema,
) *BridgeProviderConfigSchema {
	if schema == nil {
		return nil
	}
	normalized := schema.Normalize()
	if normalized.IsZero() {
		return nil
	}
	return &normalized
}

func sameBridgeProviderConfigSchema(left *BridgeProviderConfigSchema, right *BridgeProviderConfigSchema) bool {
	left = normalizeBridgeProviderConfigSchemaPointer(left)
	right = normalizeBridgeProviderConfigSchemaPointer(right)
	switch {
	case left == nil && right == nil:
		return true
	case left == nil || right == nil:
		return false
	default:
		return *left == *right
	}
}

func normalizeOptionalJSONObject(raw json.RawMessage, label string) (json.RawMessage, error) {
	return normalizeJSONObject(raw, label)
}

func bridgeInstanceFromResourceRecord(
	record resources.Record[BridgeInstanceSpec],
	existing *BridgeInstance,
	now func() time.Time,
) (BridgeInstance, error) {
	clock := now
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}

	timestamp := record.UpdatedAt.UTC()
	if timestamp.IsZero() {
		timestamp = clock()
	}
	createdAt := record.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = timestamp
	}

	instance := BridgeInstance{
		ID:                   strings.TrimSpace(record.ID),
		Scope:                record.Spec.Scope,
		WorkspaceID:          record.Spec.WorkspaceID,
		Platform:             record.Spec.Platform,
		ExtensionName:        record.Spec.ExtensionName,
		DisplayName:          record.Spec.DisplayName,
		Source:               record.Spec.Source,
		Enabled:              record.Spec.Enabled,
		Status:               bridgeStatusForProjectedRecord(record.Spec.Enabled, existing),
		DMPolicy:             record.Spec.DMPolicy,
		RoutingPolicy:        record.Spec.RoutingPolicy,
		ProviderConfig:       cloneRawJSON(record.Spec.ProviderConfig),
		DeliveryDefaults:     cloneRawJSON(record.Spec.DeliveryDefaults),
		NotificationSuppress: record.Spec.NotificationSuppress,
		CreatedAt:            createdAt,
		UpdatedAt:            timestamp,
	}
	if existing != nil && record.Spec.Enabled {
		instance.Degradation = cloneBridgeDegradationPointer(existing.Degradation)
	}
	if !record.Spec.Enabled {
		instance.Degradation = nil
	}
	if instance.ID == "" {
		instance.ID = store.NewID("brg")
	}
	instance = instance.normalize()
	if err := instance.Validate(); err != nil {
		return BridgeInstance{}, err
	}
	return instance, nil
}

func bridgeStatusForProjectedRecord(enabled bool, existing *BridgeInstance) BridgeStatus {
	if !enabled {
		return BridgeStatusDisabled
	}
	if existing == nil {
		return BridgeStatusStarting
	}
	status := existing.Status.Normalize()
	if status == "" || status == BridgeStatusDisabled {
		return BridgeStatusStarting
	}
	return status
}

func cloneBridgeDegradationPointer(value *BridgeDegradation) *BridgeDegradation {
	if value == nil {
		return nil
	}
	cloned := value.normalize()
	if cloned.IsZero() {
		return nil
	}
	return &cloned
}
