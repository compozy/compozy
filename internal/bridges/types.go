package bridges

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	bridgecontract "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/vault"
)

var (
	// ErrBridgeInstanceNotFound reports that no persisted bridge instance matched the lookup.
	ErrBridgeInstanceNotFound = errors.New("bridges: bridge instance not found")
	// ErrBridgeTargetDirectoryUnavailable reports that the daemon has no target-directory persistence surface.
	ErrBridgeTargetDirectoryUnavailable = errors.New("bridges: bridge target directory unavailable")
	// ErrBridgeTargetUnknown reports that no persisted bridge target matched the lookup.
	ErrBridgeTargetUnknown = errors.New("bridges: bridge target unknown")
	// ErrBridgeTargetAmbiguous reports that a friendly bridge target lookup matched multiple candidates.
	ErrBridgeTargetAmbiguous = errors.New("bridges: bridge target ambiguous")
	// ErrInvalidBridgeTarget reports malformed target-directory input.
	ErrInvalidBridgeTarget = errors.New("bridges: invalid bridge target")
	// ErrBridgeInstanceUnavailable reports that the instance exists but cannot currently accept routing work.
	ErrBridgeInstanceUnavailable = errors.New("bridges: bridge instance unavailable")
	// ErrInvalidBridgeSecretBinding reports that a bridge secret binding payload
	// is malformed or unsupported by the active daemon secret backend.
	ErrInvalidBridgeSecretBinding = errors.New("bridges: invalid bridge secret binding")
	// ErrBridgeSecretBindingNotFound reports that no persisted secret binding matched the lookup.
	ErrBridgeSecretBindingNotFound = errors.New("bridges: bridge secret binding not found")
	// ErrBridgeRouteNotFound reports that no persisted route matched the lookup.
	ErrBridgeRouteNotFound = errors.New("bridges: bridge route not found")
	// ErrIngestDedupRecordNotFound reports that no active ingest dedup record matched the lookup.
	ErrIngestDedupRecordNotFound = errors.New("bridges: ingest dedup record not found")
	// ErrInvalidBridgeStateTransition reports that the requested instance lifecycle transition is not allowed.
	ErrInvalidBridgeStateTransition = errors.New("bridges: invalid bridge state transition")
	// ErrBridgeInstanceReadOnly reports that a managed bridge instance does not
	// allow direct spec mutation through the generic CRUD surface.
	ErrBridgeInstanceReadOnly = errors.New("bridges: bridge instance is managed and read-only")
	// ErrBridgeNotificationSuppressed reports an intentional per-instance
	// notification drop that should still advance notification cursors.
	ErrBridgeNotificationSuppressed = errors.New("bridges: bridge notification suppressed")
)

// Scope identifies whether a bridge resource is daemon-global or workspace-owned.
type Scope string

const (
	// ScopeGlobal identifies a daemon-global bridge resource.
	ScopeGlobal Scope = Scope(bridgecontract.ScopeGlobal)
	// ScopeWorkspace identifies a workspace-owned bridge resource.
	ScopeWorkspace Scope = Scope(bridgecontract.ScopeWorkspace)
)

// Normalize returns the normalized representation of the scope.
func (s Scope) Normalize() Scope {
	return Scope(bridgecontract.Scope(s).Normalize())
}

// BridgeInstanceSource identifies where a persisted bridge instance originated.
type BridgeInstanceSource string

const (
	// BridgeInstanceSourceDynamic identifies a regular operator-created bridge instance.
	BridgeInstanceSourceDynamic BridgeInstanceSource = BridgeInstanceSource(bridgecontract.BridgeInstanceSourceDynamic)
	// BridgeInstanceSourcePackage identifies an extension bundle-managed bridge instance.
	BridgeInstanceSourcePackage BridgeInstanceSource = BridgeInstanceSource(bridgecontract.BridgeInstanceSourcePackage)
)

// Normalize returns the normalized representation of the source.
func (s BridgeInstanceSource) Normalize() BridgeInstanceSource {
	return BridgeInstanceSource(bridgecontract.BridgeInstanceSource(s).Normalize())
}

// Validate reports whether the bridge-instance source is supported.
func (s BridgeInstanceSource) Validate() error {
	return bridgecontract.BridgeInstanceSource(s).Validate()
}

// Validate reports whether the scope is supported.
func (s Scope) Validate() error {
	return bridgecontract.Scope(s).Validate()
}

// ValidateScopeWorkspaceID enforces the canonical scope and workspace invariant.
func ValidateScopeWorkspaceID(scope Scope, workspaceID string) error {
	return bridgecontract.ValidateScopeWorkspaceID(bridgecontract.Scope(scope), workspaceID)
}

// BridgeStatus reports the operator-visible lifecycle state of a bridge instance.
type BridgeStatus string

const (
	// BridgeStatusDisabled reports an instance that is intentionally disabled.
	BridgeStatusDisabled BridgeStatus = BridgeStatus(bridgecontract.BridgeStatusDisabled)
	// BridgeStatusStarting reports an instance that is launching or reconnecting.
	BridgeStatusStarting BridgeStatus = BridgeStatus(bridgecontract.BridgeStatusStarting)
	// BridgeStatusReady reports an instance that is healthy and ready to ingest/deliver.
	BridgeStatusReady BridgeStatus = BridgeStatus(bridgecontract.BridgeStatusReady)
	// BridgeStatusDegraded reports an instance that is partially working with known issues.
	BridgeStatusDegraded BridgeStatus = BridgeStatus(bridgecontract.BridgeStatusDegraded)
	// BridgeStatusAuthRequired reports an instance that cannot operate until authentication is refreshed.
	BridgeStatusAuthRequired BridgeStatus = BridgeStatus(bridgecontract.BridgeStatusAuthRequired)
	// BridgeStatusError reports an instance that is unhealthy due to a terminal or repeated fault.
	BridgeStatusError BridgeStatus = BridgeStatus(bridgecontract.BridgeStatusError)
)

// Normalize returns the normalized representation of the status.
func (s BridgeStatus) Normalize() BridgeStatus {
	return BridgeStatus(bridgecontract.BridgeStatus(s).Normalize())
}

// Validate reports whether the status belongs to the closed bridge status set.
func (s BridgeStatus) Validate() error {
	return bridgecontract.BridgeStatus(s).Validate()
}

// BridgeDMPolicy controls how direct messages from unpaired senders are handled.
type BridgeDMPolicy string

const (
	// BridgeDMPolicyOpen accepts direct messages from any sender.
	BridgeDMPolicyOpen BridgeDMPolicy = BridgeDMPolicy(bridgecontract.BridgeDMPolicyOpen)
	// BridgeDMPolicyAllowlist accepts direct messages only from approved senders.
	BridgeDMPolicyAllowlist BridgeDMPolicy = BridgeDMPolicy(bridgecontract.BridgeDMPolicyAllowlist)
	// BridgeDMPolicyPairing accepts pre-populated paired identities, then falls back to the allowlist.
	BridgeDMPolicyPairing BridgeDMPolicy = BridgeDMPolicy(bridgecontract.BridgeDMPolicyPairing)
)

// Normalize returns the normalized representation of the DM policy.
func (p BridgeDMPolicy) Normalize() BridgeDMPolicy {
	return BridgeDMPolicy(bridgecontract.BridgeDMPolicy(p).Normalize())
}

// Validate reports whether the DM policy belongs to the supported bridge v1 set.
func (p BridgeDMPolicy) Validate() error {
	return bridgecontract.BridgeDMPolicy(p).Validate()
}

// BridgeDegradationReason reports the structured operational cause for a degraded bridge instance.
type BridgeDegradationReason string

const (
	BridgeDegradationReasonAuthFailed BridgeDegradationReason = BridgeDegradationReason(
		bridgecontract.BridgeDegradationReasonAuthFailed,
	)
	BridgeDegradationReasonRateLimited BridgeDegradationReason = BridgeDegradationReason(
		bridgecontract.BridgeDegradationReasonRateLimited,
	)
	BridgeDegradationReasonWebhookInvalid BridgeDegradationReason = BridgeDegradationReason(
		bridgecontract.BridgeDegradationReasonWebhookInvalid,
	)
	BridgeDegradationReasonProviderTimeout BridgeDegradationReason = BridgeDegradationReason(
		bridgecontract.BridgeDegradationReasonProviderTimeout,
	)
	BridgeDegradationReasonTenantConfigInvalid BridgeDegradationReason = BridgeDegradationReason(
		bridgecontract.BridgeDegradationReasonTenantConfigInvalid,
	)
)

// Normalize returns the normalized representation of the degradation reason.
func (r BridgeDegradationReason) Normalize() BridgeDegradationReason {
	return BridgeDegradationReason(bridgecontract.BridgeDegradationReason(r).Normalize())
}

// Validate reports whether the degradation reason belongs to the supported bridge v1 set.
func (r BridgeDegradationReason) Validate() error {
	return bridgecontract.BridgeDegradationReason(r).Validate()
}

// BridgeDegradation captures the structured degradation metadata persisted for a bridge instance.
type BridgeDegradation struct {
	Reason  BridgeDegradationReason `toml:"reason"            json:"reason"`
	Message string                  `toml:"message,omitempty" json:"message,omitempty"`
}

// IsZero reports whether the degradation payload carries any values.
func (d BridgeDegradation) IsZero() bool {
	return bridgeDegradationToContract(d).IsZero()
}

// Validate reports whether the degradation payload is internally consistent.
func (d BridgeDegradation) Validate() error {
	return bridgeDegradationToContract(d).Validate()
}

// BridgeSecretSlot describes one provider-declared secret requirement.
type BridgeSecretSlot struct {
	Name        string `toml:"name"                  json:"name"`
	Description string `toml:"description,omitempty" json:"description,omitempty"`
	Required    bool   `toml:"required,omitempty"    json:"required,omitempty"`
}

// Normalize returns the normalized representation of the secret slot.
func (s BridgeSecretSlot) Normalize() BridgeSecretSlot {
	return s.normalize()
}

// Validate reports whether the secret slot metadata is complete.
func (s BridgeSecretSlot) Validate() error {
	normalized := s.normalize()
	if err := requireField(normalized.Name, "bridge secret slot name"); err != nil {
		return err
	}
	return nil
}

// BridgeProviderConfigSchema captures static provider config schema hints from provider manifests.
type BridgeProviderConfigSchema struct {
	Schema  string `toml:"schema,omitempty"  json:"schema,omitempty"`
	Version string `toml:"version,omitempty" json:"version,omitempty"`
}

// Normalize returns the normalized representation of the config schema hint.
func (h BridgeProviderConfigSchema) Normalize() BridgeProviderConfigSchema {
	return h.normalize()
}

// IsZero reports whether the schema hint carries any values.
func (h BridgeProviderConfigSchema) IsZero() bool {
	normalized := h.normalize()
	return normalized.Schema == "" && normalized.Version == ""
}

// Validate reports whether the config schema hint is internally consistent.
func (h BridgeProviderConfigSchema) Validate() error {
	normalized := h.normalize()
	if normalized.IsZero() {
		return nil
	}
	if normalized.Schema == "" && normalized.Version == "" {
		return errors.New("bridges: bridge provider config schema hint requires schema or version")
	}
	return nil
}

// RoutingPolicy controls which platform identity dimensions participate in routing.
type RoutingPolicy struct {
	IncludePeer   bool `json:"include_peer"`
	IncludeThread bool `json:"include_thread"`
	IncludeGroup  bool `json:"include_group"`
}

// Validate reports whether the routing policy is internally consistent.
func (p RoutingPolicy) Validate() error {
	return (bridgecontract.RoutingPolicy{
		IncludePeer: p.IncludePeer, IncludeThread: p.IncludeThread, IncludeGroup: p.IncludeGroup,
	}).Validate()
}

// BridgeProvider describes one installed bridge-capable extension that can be
// selected when creating a bridge instance.
type BridgeProvider struct {
	Platform      string                      `json:"platform"`
	ExtensionName string                      `json:"extension_name"`
	DisplayName   string                      `json:"display_name"`
	Description   string                      `json:"description,omitempty"`
	SecretSlots   []BridgeSecretSlot          `json:"secret_slots,omitempty"`
	ConfigSchema  *BridgeProviderConfigSchema `json:"config_schema,omitempty"`
	Enabled       bool                        `json:"enabled"`
	State         string                      `json:"state"`
	Health        string                      `json:"health"`
	HealthMessage string                      `json:"health_message,omitempty"`
}

// BridgeInstance is the authoritative persisted configuration for one bridge adapter instance.
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

// Normalized returns the canonical representation of the bridge instance.
func (i BridgeInstance) Normalized() BridgeInstance {
	return i.normalize()
}

// Validate reports whether the persisted bridge instance shape is complete and valid.
func (i BridgeInstance) Validate() error {
	return BridgeInstanceToContract(i).Validate()
}

// BridgeSecretBinding binds one named bridge secret slot to a daemon-managed secret reference.
type BridgeSecretBinding struct {
	BridgeInstanceID string    `json:"bridge_instance_id"`
	BindingName      string    `json:"binding_name"`
	SecretRef        string    `json:"secret_ref"`
	Kind             string    `json:"kind"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// Validate reports whether the persisted secret binding is complete and valid.
func (b BridgeSecretBinding) Validate() error {
	normalized := b.normalize()
	if err := requireField(normalized.BridgeInstanceID, "bridge secret binding bridge instance id"); err != nil {
		return err
	}
	if err := requireField(normalized.BindingName, "bridge secret binding name"); err != nil {
		return err
	}
	if err := requireField(normalized.SecretRef, "bridge secret binding secret ref"); err != nil {
		return err
	}
	if err := vault.ValidateSecretRefNamespace(normalized.SecretRef, "bridges"); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidBridgeSecretBinding, err)
	}
	if err := requireField(normalized.Kind, "bridge secret binding kind"); err != nil {
		return err
	}
	return nil
}

// DeliveryTarget identifies an outbound delivery destination within one bridge instance.
type DeliveryTarget struct {
	BridgeInstanceID string       `json:"bridge_instance_id"`
	PeerID           string       `json:"peer_id,omitempty"`
	ThreadID         string       `json:"thread_id,omitempty"`
	GroupID          string       `json:"group_id,omitempty"`
	Mode             DeliveryMode `json:"mode,omitempty"`
}

// Validate reports whether the delivery target contains a supported mode and
// the identity fields required by that mode.
func (t DeliveryTarget) Validate() error {
	return deliveryTargetToContract(t).Validate()
}

// IsZero reports whether the target carries any values.
func (t DeliveryTarget) IsZero() bool {
	return deliveryTargetToContract(t).IsZero()
}

// MessageSender identifies the upstream actor that produced an inbound message.
type MessageSender struct {
	ID          string `json:"id,omitempty"`
	Username    string `json:"username,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
}

// MessageContent carries normalized text content shared by inbound and outbound bridge models.
type MessageContent struct {
	Text string `json:"text,omitempty"`
}

// MessageAttachment captures normalized attachment metadata shared by ingest and delivery flows.
type MessageAttachment struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	MIMEType string `json:"mime_type,omitempty"`
	URL      string `json:"url,omitempty"`
}

// DeliveryOperation identifies whether the outbound delivery is posting new text,
// editing an existing remote message, or deleting one.
type DeliveryOperation string

const (
	// DeliveryOperationPost creates or continues a new daemon-owned delivery.
	DeliveryOperationPost DeliveryOperation = DeliveryOperation(bridgecontract.DeliveryOperationPost)
	// DeliveryOperationEdit updates a previously delivered message in-place.
	DeliveryOperationEdit DeliveryOperation = DeliveryOperation(bridgecontract.DeliveryOperationEdit)
	// DeliveryOperationDelete removes a previously delivered message.
	DeliveryOperationDelete DeliveryOperation = DeliveryOperation(bridgecontract.DeliveryOperationDelete)
)

// Normalize returns the canonical delivery-operation representation.
func (o DeliveryOperation) Normalize() DeliveryOperation {
	return DeliveryOperation(bridgecontract.DeliveryOperation(o).Normalize())
}

// Validate reports whether the delivery operation belongs to the supported set.
func (o DeliveryOperation) Validate() error {
	return bridgecontract.DeliveryOperation(o).Validate()
}

// IngestDedupRecord tracks inbound idempotency keys with an explicit TTL.
type IngestDedupRecord struct {
	IdempotencyKey   string    `json:"idempotency_key"`
	BridgeInstanceID string    `json:"bridge_instance_id"`
	ReceivedAt       time.Time `json:"received_at"`
	ExpiresAt        time.Time `json:"expires_at"`
}

// Validate reports whether the dedup record is complete and time-consistent.
func (r IngestDedupRecord) Validate() error {
	normalized := r.normalize()
	if err := requireField(normalized.IdempotencyKey, "ingest dedup idempotency key"); err != nil {
		return err
	}
	if err := requireField(normalized.BridgeInstanceID, "ingest dedup bridge instance id"); err != nil {
		return err
	}
	if normalized.ReceivedAt.IsZero() {
		return errors.New("bridges: ingest dedup received at is required")
	}
	if normalized.ExpiresAt.IsZero() {
		return errors.New("bridges: ingest dedup expires at is required")
	}
	if !normalized.ExpiresAt.After(normalized.ReceivedAt) {
		return errors.New("bridges: ingest dedup expires at must be after received at")
	}
	return nil
}

func (i BridgeInstance) normalize() BridgeInstance {
	return bridgeInstanceFromContract(bridgecontract.NormalizeBridgeInstance(BridgeInstanceToContract(i)))
}

func (d BridgeDegradation) normalize() BridgeDegradation {
	return bridgeDegradationFromContract(
		bridgecontract.NormalizeBridgeDegradation(bridgeDegradationToContract(d)),
	)
}

func (s BridgeSecretSlot) normalize() BridgeSecretSlot {
	normalized := s
	normalized.Name = strings.TrimSpace(normalized.Name)
	normalized.Description = strings.TrimSpace(normalized.Description)
	return normalized
}

func (h BridgeProviderConfigSchema) normalize() BridgeProviderConfigSchema {
	normalized := h
	normalized.Schema = strings.TrimSpace(normalized.Schema)
	normalized.Version = strings.TrimSpace(normalized.Version)
	return normalized
}

func (b BridgeSecretBinding) normalize() BridgeSecretBinding {
	normalized := b
	normalized.BridgeInstanceID = strings.TrimSpace(normalized.BridgeInstanceID)
	normalized.BindingName = strings.TrimSpace(normalized.BindingName)
	normalized.SecretRef = strings.TrimSpace(normalized.SecretRef)
	normalized.Kind = strings.TrimSpace(normalized.Kind)
	return normalized
}

func (t DeliveryTarget) normalize() DeliveryTarget {
	return deliveryTargetFromContract(bridgecontract.NormalizeDeliveryTarget(deliveryTargetToContract(t)))
}

func (r IngestDedupRecord) normalize() IngestDedupRecord {
	normalized := r
	normalized.IdempotencyKey = strings.TrimSpace(normalized.IdempotencyKey)
	normalized.BridgeInstanceID = strings.TrimSpace(normalized.BridgeInstanceID)
	return normalized
}

func requireField(value string, label string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("bridges: %s is required", label)
	}
	return nil
}
