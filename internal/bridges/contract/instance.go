package contract

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Scope identifies whether a bridge resource is daemon-global or workspace-owned.
type Scope string

const (
	// ScopeGlobal identifies a daemon-global bridge resource.
	ScopeGlobal Scope = "global"
	// ScopeWorkspace identifies a workspace-owned bridge resource.
	ScopeWorkspace Scope = "workspace"
)

// Normalize returns the canonical scope.
func (s Scope) Normalize() Scope {
	return Scope(strings.ToLower(strings.TrimSpace(string(s))))
}

// Validate reports whether the scope is supported.
func (s Scope) Validate() error {
	switch s.Normalize() {
	case ScopeGlobal, ScopeWorkspace:
		return nil
	case "":
		return errors.New("bridges: scope is required")
	default:
		return fmt.Errorf("bridges: unsupported scope %q", s)
	}
}

// ValidateScopeWorkspaceID enforces the canonical scope and workspace invariant.
func ValidateScopeWorkspaceID(scope Scope, workspaceID string) error {
	normalizedScope := scope.Normalize()
	trimmedWorkspaceID := strings.TrimSpace(workspaceID)
	if err := normalizedScope.Validate(); err != nil {
		return err
	}
	switch normalizedScope {
	case ScopeGlobal:
		if trimmedWorkspaceID != "" {
			return errors.New("bridges: global scope cannot include workspace id")
		}
	case ScopeWorkspace:
		if trimmedWorkspaceID == "" {
			return errors.New("bridges: workspace scope requires workspace id")
		}
	}
	return nil
}

// BridgeInstanceSource identifies where a bridge instance originated.
type BridgeInstanceSource string

const (
	// BridgeInstanceSourceDynamic identifies an operator-created instance.
	BridgeInstanceSourceDynamic BridgeInstanceSource = "dynamic"
	// BridgeInstanceSourcePackage identifies an extension-managed instance.
	BridgeInstanceSourcePackage BridgeInstanceSource = "package"
)

// Normalize returns the canonical instance source.
func (s BridgeInstanceSource) Normalize() BridgeInstanceSource {
	return BridgeInstanceSource(strings.ToLower(strings.TrimSpace(string(s))))
}

// Validate reports whether the instance source is supported.
func (s BridgeInstanceSource) Validate() error {
	switch s.Normalize() {
	case BridgeInstanceSourceDynamic, BridgeInstanceSourcePackage:
		return nil
	case "":
		return errors.New("bridges: bridge instance source is required")
	default:
		return fmt.Errorf("bridges: unsupported bridge instance source %q", s)
	}
}

// BridgeStatus reports the lifecycle state of a bridge instance.
type BridgeStatus string

const (
	BridgeStatusDisabled     BridgeStatus = "disabled"
	BridgeStatusStarting     BridgeStatus = "starting"
	BridgeStatusReady        BridgeStatus = "ready"
	BridgeStatusDegraded     BridgeStatus = "degraded"
	BridgeStatusAuthRequired BridgeStatus = "auth_required"
	BridgeStatusError        BridgeStatus = "error"
)

// Normalize returns the canonical bridge status.
func (s BridgeStatus) Normalize() BridgeStatus {
	return BridgeStatus(strings.ToLower(strings.TrimSpace(string(s))))
}

// Validate reports whether the status belongs to the closed status set.
func (s BridgeStatus) Validate() error {
	switch s.Normalize() {
	case BridgeStatusDisabled, BridgeStatusStarting, BridgeStatusReady, BridgeStatusDegraded,
		BridgeStatusAuthRequired, BridgeStatusError:
		return nil
	case "":
		return errors.New("bridges: bridge status is required")
	default:
		return fmt.Errorf("bridges: unsupported bridge status %q", s)
	}
}

// BridgeDMPolicy controls direct messages from unpaired senders.
type BridgeDMPolicy string

const (
	BridgeDMPolicyOpen      BridgeDMPolicy = "open"
	BridgeDMPolicyAllowlist BridgeDMPolicy = "allowlist"
	BridgeDMPolicyPairing   BridgeDMPolicy = "pairing"
)

// Normalize returns the canonical direct-message policy.
func (p BridgeDMPolicy) Normalize() BridgeDMPolicy {
	return BridgeDMPolicy(strings.ToLower(strings.TrimSpace(string(p))))
}

// Validate reports whether the direct-message policy is supported.
func (p BridgeDMPolicy) Validate() error {
	switch p.Normalize() {
	case "", BridgeDMPolicyOpen, BridgeDMPolicyAllowlist, BridgeDMPolicyPairing:
		return nil
	default:
		return fmt.Errorf("bridges: unsupported dm policy %q", p)
	}
}

// BridgeDegradationReason identifies an operational degradation cause.
type BridgeDegradationReason string

const (
	BridgeDegradationReasonAuthFailed          BridgeDegradationReason = "auth_failed"
	BridgeDegradationReasonRateLimited         BridgeDegradationReason = "rate_limited"
	BridgeDegradationReasonWebhookInvalid      BridgeDegradationReason = "webhook_invalid"
	BridgeDegradationReasonProviderTimeout     BridgeDegradationReason = "provider_timeout"
	BridgeDegradationReasonTenantConfigInvalid BridgeDegradationReason = "tenant_config_invalid"
)

// Normalize returns the canonical degradation reason.
func (r BridgeDegradationReason) Normalize() BridgeDegradationReason {
	return BridgeDegradationReason(strings.ToLower(strings.TrimSpace(string(r))))
}

// Validate reports whether the degradation reason is supported.
func (r BridgeDegradationReason) Validate() error {
	switch r.Normalize() {
	case BridgeDegradationReasonAuthFailed, BridgeDegradationReasonRateLimited,
		BridgeDegradationReasonWebhookInvalid, BridgeDegradationReasonProviderTimeout,
		BridgeDegradationReasonTenantConfigInvalid:
		return nil
	case "":
		return errors.New("bridges: bridge degradation reason is required")
	default:
		return fmt.Errorf("bridges: unsupported bridge degradation reason %q", r)
	}
}

// BridgeDegradation captures structured instance degradation metadata.
type BridgeDegradation struct {
	Reason  BridgeDegradationReason `toml:"reason"            json:"reason"`
	Message string                  `toml:"message,omitempty" json:"message,omitempty"`
}

// IsZero reports whether the degradation carries any values.
func (d BridgeDegradation) IsZero() bool {
	normalized := d.normalize()
	return normalized.Reason == "" && normalized.Message == ""
}

// Validate reports whether the degradation is internally consistent.
func (d BridgeDegradation) Validate() error {
	normalized := d.normalize()
	if normalized.IsZero() {
		return nil
	}
	return normalized.Reason.Validate()
}

func (d BridgeDegradation) normalize() BridgeDegradation {
	d.Reason = d.Reason.Normalize()
	d.Message = strings.TrimSpace(d.Message)
	return d
}

// RoutingPolicy selects the platform dimensions used for routing.
type RoutingPolicy struct {
	IncludePeer   bool `json:"include_peer"`
	IncludeThread bool `json:"include_thread"`
	IncludeGroup  bool `json:"include_group"`
}

// Validate reports whether the routing policy is internally consistent.
func (p RoutingPolicy) Validate() error {
	if p.IncludeThread && !p.IncludePeer && !p.IncludeGroup {
		return errors.New("bridges: routing policy cannot include thread without peer or group")
	}
	return nil
}

// BridgeInstance is one daemon-owned bridge instance snapshot.
type BridgeInstance struct {
	ID                   string               `json:"id"`
	Scope                Scope                `json:"scope"`
	WorkspaceID          string               `json:"workspace_id,omitempty"`
	Platform             string               `json:"platform"`
	ExtensionName        string               `json:"extension_name"`
	DisplayName          string               `json:"display_name"`
	Source               BridgeInstanceSource `json:"source,omitempty"`
	Enabled              bool                 `json:"enabled"`
	Status               BridgeStatus         `json:"status"`
	DMPolicy             BridgeDMPolicy       `json:"dm_policy,omitempty"`
	RoutingPolicy        RoutingPolicy        `json:"routing_policy"`
	ProviderConfig       json.RawMessage      `json:"provider_config,omitempty"`
	DeliveryDefaults     json.RawMessage      `json:"delivery_defaults,omitempty"`
	NotificationSuppress bool                 `json:"notification_suppress"`
	Degradation          *BridgeDegradation   `json:"degradation,omitempty"`
	CreatedAt            time.Time            `json:"created_at"`
	UpdatedAt            time.Time            `json:"updated_at"`
}

// NormalizeBridgeInstance returns the canonical wire representation used by
// composition-layer adapters.
func NormalizeBridgeInstance(instance BridgeInstance) BridgeInstance {
	normalized := instance.normalize()
	normalized.ProviderConfig = slices.Clone(normalized.ProviderConfig)
	normalized.DeliveryDefaults = slices.Clone(normalized.DeliveryDefaults)
	return normalized
}

// NormalizeBridgeDegradation returns canonical degradation metadata.
func NormalizeBridgeDegradation(degradation BridgeDegradation) BridgeDegradation {
	return degradation.normalize()
}

// ValidateBridgeInstanceLifecycle enforces the enabled/status wire invariant.
func ValidateBridgeInstanceLifecycle(enabled bool, status BridgeStatus) error {
	return validateInstanceLifecycle(enabled, status)
}

// Validate reports whether the bridge instance is complete and valid.
func (i BridgeInstance) Validate() error {
	normalized := i.normalize()
	if err := requireField(normalized.ID, "bridge instance id"); err != nil {
		return err
	}
	if err := ValidateScopeWorkspaceID(normalized.Scope, normalized.WorkspaceID); err != nil {
		return err
	}
	if err := requireField(normalized.Platform, "bridge instance platform"); err != nil {
		return err
	}
	if err := requireField(normalized.ExtensionName, "bridge instance extension name"); err != nil {
		return err
	}
	if err := requireField(normalized.DisplayName, "bridge instance display name"); err != nil {
		return err
	}
	if err := normalized.Source.Validate(); err != nil {
		return err
	}
	if err := normalized.Status.Validate(); err != nil {
		return err
	}
	if err := validateInstanceLifecycle(normalized.Enabled, normalized.Status); err != nil {
		return err
	}
	if err := normalized.DMPolicy.Validate(); err != nil {
		return err
	}
	if err := normalized.RoutingPolicy.Validate(); err != nil {
		return err
	}
	if _, err := normalizeProviderConfigJSON(normalized.ProviderConfig); err != nil {
		return err
	}
	if _, err := NormalizeDeliveryDefaultsJSON(normalized.DeliveryDefaults); err != nil {
		return err
	}
	if normalized.Degradation == nil {
		return nil
	}
	if err := normalized.Degradation.Validate(); err != nil {
		return err
	}
	switch normalized.Status {
	case BridgeStatusDegraded, BridgeStatusAuthRequired, BridgeStatusError:
		return nil
	default:
		return errors.New("bridges: bridge degradation requires degraded, auth_required, or error status")
	}
}

func (i BridgeInstance) normalize() BridgeInstance {
	i.ID = strings.TrimSpace(i.ID)
	i.Scope = i.Scope.Normalize()
	i.WorkspaceID = strings.TrimSpace(i.WorkspaceID)
	i.Platform = strings.TrimSpace(i.Platform)
	i.ExtensionName = strings.TrimSpace(i.ExtensionName)
	i.DisplayName = strings.TrimSpace(i.DisplayName)
	i.Source = i.Source.Normalize()
	if i.Source == "" {
		i.Source = BridgeInstanceSourceDynamic
	}
	i.Status = i.Status.Normalize()
	i.DMPolicy = i.DMPolicy.Normalize()
	if i.DMPolicy == "" {
		i.DMPolicy = BridgeDMPolicyOpen
	}
	i.ProviderConfig = bytes.TrimSpace(i.ProviderConfig)
	i.DeliveryDefaults = bytes.TrimSpace(i.DeliveryDefaults)
	if i.Degradation != nil {
		degradation := i.Degradation.normalize()
		if degradation.IsZero() {
			i.Degradation = nil
		} else {
			i.Degradation = &degradation
		}
	}
	return i
}

func validateInstanceLifecycle(enabled bool, status BridgeStatus) error {
	normalized := status.Normalize()
	if err := normalized.Validate(); err != nil {
		return err
	}
	if !enabled && normalized != BridgeStatusDisabled {
		return fmt.Errorf("bridges: disabled bridge instance must report status %q", BridgeStatusDisabled)
	}
	if enabled && normalized == BridgeStatusDisabled {
		return fmt.Errorf("bridges: enabled bridge instance cannot report status %q", BridgeStatusDisabled)
	}
	return nil
}
