package contract

import (
	"encoding/json"
	"strings"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	taskpkg "github.com/compozy/agh/internal/task"
)

// CreateBridgeRequest is the shared bridge-instance creation payload.
type CreateBridgeRequest struct {
	Scope                bridgepkg.Scope               `json:"scope"`
	WorkspaceID          string                        `json:"workspace_id,omitempty"`
	Platform             string                        `json:"platform"`
	ExtensionName        string                        `json:"extension_name"`
	DisplayName          string                        `json:"display_name"`
	Enabled              bool                          `json:"enabled"`
	DMPolicy             bridgepkg.BridgeDMPolicy      `json:"dm_policy,omitempty"`
	RoutingPolicy        bridgepkg.RoutingPolicy       `json:"routing_policy"`
	ProviderConfig       BridgeProviderConfigPayload   `json:"provider_config,omitempty"`
	DeliveryDefaults     BridgeDeliveryDefaultsPayload `json:"delivery_defaults,omitempty"`
	NotificationSuppress bool                          `json:"notification_suppress,omitempty"`
}

// ToCreateInstanceRequest validates and converts the transport payload into the
// daemon-owned bridge create request.
func (r CreateBridgeRequest) ToCreateInstanceRequest() (bridgepkg.CreateInstanceRequest, error) {
	providerConfig, err := normalizeBridgeJSONPayload(
		json.RawMessage(r.ProviderConfig),
		"bridge provider config",
		validateBridgeProviderConfigPayload,
	)
	if err != nil {
		return bridgepkg.CreateInstanceRequest{}, err
	}
	deliveryDefaults, err := normalizeBridgeDeliveryDefaultsPayload(json.RawMessage(r.DeliveryDefaults))
	if err != nil {
		return bridgepkg.CreateInstanceRequest{}, err
	}

	req := bridgepkg.CreateInstanceRequest{
		Scope:                r.Scope,
		WorkspaceID:          strings.TrimSpace(r.WorkspaceID),
		Platform:             strings.TrimSpace(r.Platform),
		ExtensionName:        strings.TrimSpace(r.ExtensionName),
		DisplayName:          strings.TrimSpace(r.DisplayName),
		Enabled:              r.Enabled,
		Status:               bridgeCreateInitialStatus(r.Enabled),
		DMPolicy:             r.DMPolicy,
		RoutingPolicy:        r.RoutingPolicy,
		ProviderConfig:       providerConfig,
		DeliveryDefaults:     deliveryDefaults,
		NotificationSuppress: r.NotificationSuppress,
	}
	if err := req.Validate(); err != nil {
		return bridgepkg.CreateInstanceRequest{}, err
	}
	return req, nil
}

func bridgeCreateInitialStatus(enabled bool) bridgepkg.BridgeStatus {
	if !enabled {
		return bridgepkg.BridgeStatusDisabled
	}
	return bridgepkg.BridgeStatusStarting
}

// UpdateBridgeRequest is the shared mutable bridge-instance patch payload.
type UpdateBridgeRequest struct {
	DisplayName          *string                        `json:"display_name,omitempty"`
	DMPolicy             *bridgepkg.BridgeDMPolicy      `json:"dm_policy,omitempty"`
	RoutingPolicy        *bridgepkg.RoutingPolicy       `json:"routing_policy,omitempty"`
	ProviderConfig       *BridgeProviderConfigPayload   `json:"provider_config,omitempty"`
	DeliveryDefaults     *BridgeDeliveryDefaultsPayload `json:"delivery_defaults,omitempty"`
	NotificationSuppress *bool                          `json:"notification_suppress,omitempty"`
	Degradation          *bridgepkg.BridgeDegradation   `json:"degradation,omitempty"`
	ClearDegradation     bool                           `json:"clear_degradation,omitempty"`
}

// PutBridgeSecretBindingRequest is the shared bridge secret binding upsert payload.
type PutBridgeSecretBindingRequest struct {
	// SecretRef identifies the daemon-owned encrypted secret reference.
	SecretRef string `json:"secret_ref"`
	// Kind identifies the materialized secret kind passed to the provider runtime.
	Kind string `json:"kind"`
	// SecretValue is write-only plaintext stored into the vault when provided.
	SecretValue *string `json:"secret_value,omitempty"`
}

// ToUpdateInstanceRequest validates and converts the transport patch payload
// into the daemon-owned bridge update request for the supplied instance id.
func (r UpdateBridgeRequest) ToUpdateInstanceRequest(id string) (bridgepkg.UpdateInstanceRequest, error) {
	req := bridgepkg.UpdateInstanceRequest{
		ID:               strings.TrimSpace(id),
		ClearDegradation: r.ClearDegradation,
	}
	if r.DisplayName != nil {
		value := strings.TrimSpace(*r.DisplayName)
		req.DisplayName = &value
	}
	if r.DMPolicy != nil {
		value := *r.DMPolicy
		req.DMPolicy = &value
	}
	if r.RoutingPolicy != nil {
		value := *r.RoutingPolicy
		req.RoutingPolicy = &value
	}
	if r.ProviderConfig != nil {
		value, err := normalizeBridgeJSONPayload(
			json.RawMessage(*r.ProviderConfig),
			"bridge provider config",
			validateBridgeProviderConfigPayload,
		)
		if err != nil {
			return bridgepkg.UpdateInstanceRequest{}, err
		}
		req.ProviderConfig = &value
	}
	if r.DeliveryDefaults != nil {
		value, err := normalizeBridgeDeliveryDefaultsPayload(json.RawMessage(*r.DeliveryDefaults))
		if err != nil {
			return bridgepkg.UpdateInstanceRequest{}, err
		}
		req.DeliveryDefaults = &value
	}
	if r.NotificationSuppress != nil {
		value := *r.NotificationSuppress
		req.NotificationSuppress = &value
	}
	if r.Degradation != nil {
		req.Degradation = cloneBridgeDegradation(r.Degradation)
	}
	if err := req.Validate(); err != nil {
		return bridgepkg.UpdateInstanceRequest{}, err
	}
	return req, nil
}

// BridgeDeliveryTargetInput is the shared typed delivery-target override payload.
type BridgeDeliveryTargetInput struct {
	BridgeInstanceID string                 `json:"bridge_instance_id,omitempty"`
	PeerID           string                 `json:"peer_id,omitempty"`
	ThreadID         string                 `json:"thread_id,omitempty"`
	GroupID          string                 `json:"group_id,omitempty"`
	Mode             bridgepkg.DeliveryMode `json:"mode,omitempty"`
}

// BridgeTestDeliveryRequest is the shared typed dry-run delivery payload.
type BridgeTestDeliveryRequest struct {
	Message string                    `json:"message,omitempty"`
	Target  BridgeDeliveryTargetInput `json:"target"`
}

// CreateTaskBridgeNotificationSubscriptionRequest captures one task-scoped
// terminal bridge notification target.
type CreateTaskBridgeNotificationSubscriptionRequest struct {
	SubscriptionID   string                 `json:"subscription_id,omitempty"`
	BridgeInstanceID string                 `json:"bridge_instance_id"`
	Scope            bridgepkg.Scope        `json:"scope"`
	WorkspaceID      string                 `json:"workspace_id,omitempty"`
	PeerID           string                 `json:"peer_id,omitempty"`
	ThreadID         string                 `json:"thread_id,omitempty"`
	GroupID          string                 `json:"group_id,omitempty"`
	DeliveryMode     bridgepkg.DeliveryMode `json:"delivery_mode"`
}

// ToResolveDeliveryTargetRequest validates and converts the transport payload
// into the daemon-owned delivery-target resolution request for the supplied
// bridge instance id.
func (r BridgeTestDeliveryRequest) ToResolveDeliveryTargetRequest(
	bridgeInstanceID string,
) (bridgepkg.ResolveDeliveryTargetRequest, error) {
	req := bridgepkg.ResolveDeliveryTargetRequest{
		BridgeInstanceID: strings.TrimSpace(r.Target.BridgeInstanceID),
		PeerID:           strings.TrimSpace(r.Target.PeerID),
		ThreadID:         strings.TrimSpace(r.Target.ThreadID),
		GroupID:          strings.TrimSpace(r.Target.GroupID),
		Mode:             r.Target.Mode.Normalize(),
	}
	req.BridgeInstanceID = strings.TrimSpace(req.BridgeInstanceID)
	trimmedID := strings.TrimSpace(bridgeInstanceID)
	if req.BridgeInstanceID == "" {
		req.BridgeInstanceID = trimmedID
	}
	if req.BridgeInstanceID != trimmedID {
		return bridgepkg.ResolveDeliveryTargetRequest{}, ErrBridgeInstanceMismatch
	}
	if err := req.Validate(); err != nil {
		return bridgepkg.ResolveDeliveryTargetRequest{}, err
	}
	return req, nil
}

// ErrBridgeInstanceMismatch reports a body/path bridge-instance mismatch for
// typed delivery-target requests.
var ErrBridgeInstanceMismatch = bridgeContractError("bridge instance id must match request path")

type bridgeContractError string

func (e bridgeContractError) Error() string {
	return string(e)
}

// BridgeHealthStreamPayload wraps one bridge-health SSE snapshot payload.
type BridgeHealthStreamPayload struct {
	GeneratedAt  time.Time                      `json:"generated_at"`
	BridgeHealth map[string]BridgeHealthPayload `json:"bridge_health"`
}

// BridgeProvidersResponse wraps the shared installed provider catalog.
type BridgeProvidersResponse struct {
	Providers []BridgeProviderPayload `json:"providers"`
}

// BridgeResponse wraps one shared bridge payload.
type BridgeResponse struct {
	Bridge BridgePayload       `json:"bridge"`
	Health BridgeHealthPayload `json:"health"`
}

// BridgeRoutesResponse wraps one bridge's route set.
type BridgeRoutesResponse struct {
	Routes []bridgepkg.BridgeRoute `json:"routes"`
}

// BridgeTargetsResponse wraps one bridge's daemon-owned target directory.
type BridgeTargetsResponse struct {
	BridgeID                string                   `json:"bridge_id"`
	Targets                 []bridgepkg.BridgeTarget `json:"targets"`
	Total                   int                      `json:"total"`
	CacheStale              bool                     `json:"cache_stale"`
	GeneratedAt             time.Time                `json:"generated_at"`
	LastSuccessfulRefreshAt *time.Time               `json:"last_successful_refresh_at,omitempty"`
}

// BridgeResolveTargetRequest resolves a human-facing or canonical bridge target name.
type BridgeResolveTargetRequest struct {
	Name string `json:"name"`
}

// BridgeResolveTargetResponse wraps the deterministic bridge target resolver result.
type BridgeResolveTargetResponse struct {
	Result     bridgepkg.ResolveBridgeTargetResult `json:"result"`
	Diagnostic *DiagnosticItem                     `json:"diagnostic,omitempty"`
}

// TaskBridgeNotificationCursorPayload exposes the durable cursor identity and
// latest diagnostic state for one bridge terminal notification subscription.
type TaskBridgeNotificationCursorPayload struct {
	ConsumerID      string     `json:"consumer_id"`
	StreamName      string     `json:"stream_name"`
	SubjectID       string     `json:"subject_id"`
	LastSequence    int64      `json:"last_sequence"`
	LastDeliveryID  string     `json:"last_delivery_id,omitempty"`
	LastDeliveredAt *time.Time `json:"last_delivered_at,omitempty"`
	LastError       string     `json:"last_error,omitempty"`
	UpdatedAt       *time.Time `json:"updated_at,omitempty"`
}

// TaskBridgeNotificationSubscriptionPayload exposes one bridge terminal
// notification subscription plus its durable cursor diagnostics.
type TaskBridgeNotificationSubscriptionPayload struct {
	SubscriptionID   string                              `json:"subscription_id"`
	TaskID           string                              `json:"task_id"`
	BridgeInstanceID string                              `json:"bridge_instance_id"`
	Scope            bridgepkg.Scope                     `json:"scope"`
	WorkspaceID      string                              `json:"workspace_id,omitempty"`
	PeerID           string                              `json:"peer_id,omitempty"`
	ThreadID         string                              `json:"thread_id,omitempty"`
	GroupID          string                              `json:"group_id,omitempty"`
	DeliveryMode     bridgepkg.DeliveryMode              `json:"delivery_mode"`
	Cursor           TaskBridgeNotificationCursorPayload `json:"cursor"`
	CreatedBy        taskpkg.ActorIdentity               `json:"created_by"`
	CreatedAt        time.Time                           `json:"created_at"`
	UpdatedAt        time.Time                           `json:"updated_at"`
}

// TaskBridgeNotificationSubscriptionResponse wraps one task bridge
// notification subscription payload.
type TaskBridgeNotificationSubscriptionResponse struct {
	Subscription TaskBridgeNotificationSubscriptionPayload `json:"subscription"`
}

// TaskBridgeNotificationSubscriptionsResponse wraps a task bridge notification
// subscription list.
type TaskBridgeNotificationSubscriptionsResponse struct {
	Subscriptions []TaskBridgeNotificationSubscriptionPayload `json:"subscriptions"`
}

// BridgeTestDeliveryResponse wraps the dry-run delivery-target resolution payload.
type BridgeTestDeliveryResponse struct {
	Status         string                   `json:"status"`
	Message        string                   `json:"message,omitempty"`
	DeliveryTarget bridgepkg.DeliveryTarget `json:"delivery_target"`
}

// BridgeHealthPayload captures the additive per-instance observability fields
// exposed through bridge APIs.
type BridgeHealthPayload struct {
	BridgeInstanceID        string                       `json:"bridge_instance_id"`
	Status                  bridgepkg.BridgeStatus       `json:"status"`
	RouteCount              int                          `json:"route_count"`
	DeliveryBacklog         int                          `json:"delivery_backlog"`
	DeliveryDroppedTotal    int                          `json:"delivery_dropped_total"`
	DeliveryDroppedByReason map[string]int               `json:"delivery_dropped_by_reason,omitempty"`
	DeliveryFailuresTotal   int                          `json:"delivery_failures_total"`
	AuthFailuresTotal       int                          `json:"auth_failures_total"`
	LastSuccessAt           *time.Time                   `json:"last_success_at,omitempty"`
	LastError               string                       `json:"last_error,omitempty"`
	LastErrorAt             *time.Time                   `json:"last_error_at,omitempty"`
	Degradation             *bridgepkg.BridgeDegradation `json:"degradation,omitempty"`
	Diagnostics             []bridgepkg.BridgeDiagnostic `json:"diagnostics,omitempty"`
}

// BridgeAggregateHealthPayload captures the additive bridge summary nested
// under the daemon health response.
type BridgeAggregateHealthPayload struct {
	TotalInstances        int                       `json:"total_instances"`
	RouteCount            int                       `json:"route_count"`
	DeliveryBacklog       int                       `json:"delivery_backlog"`
	DeliveryDroppedTotal  int                       `json:"delivery_dropped_total"`
	DeliveryFailuresTotal int                       `json:"delivery_failures_total"`
	AuthFailuresTotal     int                       `json:"auth_failures_total"`
	StatusCounts          BridgeStatusCountsPayload `json:"status_counts"`
}

// BridgeProviderPayload captures provider metadata exposed by bridge-management APIs.
type BridgeProviderPayload struct {
	Platform      string                                `json:"platform"`
	ExtensionName string                                `json:"extension_name"`
	DisplayName   string                                `json:"display_name"`
	Description   string                                `json:"description,omitempty"`
	SecretSlots   []bridgepkg.BridgeSecretSlot          `json:"secret_slots,omitempty"`
	ConfigSchema  *bridgepkg.BridgeProviderConfigSchema `json:"config_schema,omitempty"`
	Enabled       bool                                  `json:"enabled"`
	State         string                                `json:"state"`
	Health        string                                `json:"health"`
	HealthMessage string                                `json:"health_message,omitempty"`
}

// BridgePayload captures the shared bridge-management contract returned by HTTP/UDS.
type BridgePayload struct {
	ID                   string                         `json:"id"`
	Scope                bridgepkg.Scope                `json:"scope"`
	WorkspaceID          string                         `json:"workspace_id,omitempty"`
	Platform             string                         `json:"platform"`
	ExtensionName        string                         `json:"extension_name"`
	DisplayName          string                         `json:"display_name"`
	Source               bridgepkg.BridgeInstanceSource `json:"source,omitempty"`
	Enabled              bool                           `json:"enabled"`
	Status               bridgepkg.BridgeStatus         `json:"status"`
	DMPolicy             bridgepkg.BridgeDMPolicy       `json:"dm_policy,omitempty"`
	RoutingPolicy        bridgepkg.RoutingPolicy        `json:"routing_policy"`
	ProviderConfig       BridgeProviderConfigPayload    `json:"provider_config,omitempty"`
	WebhookPublicURL     string                         `json:"webhook_public_url,omitempty"`
	DeliveryDefaults     BridgeDeliveryDefaultsPayload  `json:"delivery_defaults,omitempty"`
	NotificationSuppress bool                           `json:"notification_suppress"`
	Degradation          *bridgepkg.BridgeDegradation   `json:"degradation,omitempty"`
	CreatedAt            time.Time                      `json:"created_at"`
	UpdatedAt            time.Time                      `json:"updated_at"`
}

// ToBridgeSecretBinding validates and converts the transport payload into the daemon-owned binding request.
func (r PutBridgeSecretBindingRequest) ToBridgeSecretBinding(
	bridgeInstanceID string,
	bindingName string,
) (bridgepkg.BridgeSecretBinding, error) {
	binding := bridgepkg.BridgeSecretBinding{
		BridgeInstanceID: strings.TrimSpace(bridgeInstanceID),
		BindingName:      strings.TrimSpace(bindingName),
		SecretRef:        strings.TrimSpace(r.SecretRef),
		Kind:             strings.TrimSpace(r.Kind),
	}
	if err := binding.Validate(); err != nil {
		return bridgepkg.BridgeSecretBinding{}, err
	}
	return binding, nil
}

// BridgeSecretBindingsResponse wraps one bridge-instance secret binding list.
type BridgeSecretBindingsResponse struct {
	Bindings []bridgepkg.BridgeSecretBinding `json:"bindings"`
}

// BridgeSecretBindingResponse wraps one bridge secret binding payload.
type BridgeSecretBindingResponse struct {
	Binding bridgepkg.BridgeSecretBinding `json:"binding"`
}

func cloneBridgeDegradation(value *bridgepkg.BridgeDegradation) *bridgepkg.BridgeDegradation {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
