package cli

import (
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"

	bridgepkg "github.com/compozy/compozy/internal/bridges"

	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/sse"
)

// NetworkStatusRecord is the shared network status payload.
type NetworkStatusRecord = contract.NetworkStatusPayload

// NetworkKindMetricRecord is one per-kind network metric row.
type NetworkKindMetricRecord = contract.NetworkKindMetricPayload

// NetworkSendRequest captures one outbound network send payload.
type NetworkSendRequest = contract.NetworkSendRequest

// NetworkSendRecord is the shared network send response payload.
type NetworkSendRecord = contract.NetworkSendPayload

// NetworkPeerRecord is the shared visible-peer payload.
type NetworkPeerRecord = contract.NetworkPeerPayload

// NetworkPeerCardRecord is the shared peer-card payload nested under peers.
type NetworkPeerCardRecord = contract.NetworkPeerCardPayload

// NetworkChannelRecord is the shared active-channel payload.
type NetworkChannelRecord = contract.NetworkChannelPayload

// NetworkChannelDetailRecord is the shared detailed channel payload.
type NetworkChannelDetailRecord = contract.NetworkChannelDetailPayload

// CreateNetworkChannelRequest captures one network channel creation payload.
type CreateNetworkChannelRequest = contract.CreateNetworkChannelRequest

// UpdateNetworkChannelRequest captures one network channel policy update payload.
type UpdateNetworkChannelRequest = contract.UpdateNetworkChannelRequest

// NetworkSubscriptionRequest captures one network delivery preference mutation.
type NetworkSubscriptionRequest = contract.NetworkSubscriptionRequest

// NetworkSubscriptionRecord is the shared delivery preference payload.
type NetworkSubscriptionRecord = contract.NetworkSubscriptionPayload

// NetworkThreadRecord is the shared public-thread summary payload.
type NetworkThreadRecord = contract.NetworkThreadSummaryPayload

// NetworkDirectRoomRecord is the shared direct-room summary payload.
type NetworkDirectRoomRecord = contract.NetworkDirectRoomPayload

// NetworkConversationMessageRecord is the shared conversation message payload.
type NetworkConversationMessageRecord = contract.NetworkConversationMessagePayload

// NetworkWorkRecord is the shared network work payload.
type NetworkWorkRecord = contract.NetworkWorkPayload

// NetworkDirectResolveRequest captures direct-room resolution inputs.
type NetworkDirectResolveRequest = contract.NetworkDirectResolveRequest

// NetworkEnvelopeRecord is the shared surfaced envelope payload.
type NetworkEnvelopeRecord = contract.NetworkEnvelopePayload

// PromoteNetworkThreadTaskRequest captures thread-to-task promotion inputs.
type PromoteNetworkThreadTaskRequest = contract.PromoteNetworkThreadTaskRequest

// PromoteNetworkThreadTaskRecord is the shared promotion response payload.
type PromoteNetworkThreadTaskRecord = contract.PromoteNetworkThreadTaskResponse

// InstallExtensionRequest captures the shared extension install payload.
type InstallExtensionRequest = contract.InstallExtensionRequest

// ExtensionSearchRequest captures source-union discovery filters.
type ExtensionSearchRequest = contract.ExtensionSearchRequest

// ExtensionSearchRecord is one stable source-union discovery page.
type ExtensionSearchRecord = contract.ExtensionSearchResponse

// ExtensionCommandsRecord is the contributed-command discovery projection.
type ExtensionCommandsRecord = contract.ExtensionCommandsResponse

// UpdateExtensionRequest captures the shared extension update payload.
type UpdateExtensionRequest = contract.UpdateExtensionRequest

// UpdateExtensionsRequest captures one daemon-selected update batch.
type UpdateExtensionsRequest = contract.UpdateExtensionsRequest

// ExtensionRecord is the shared extension response payload.
type ExtensionRecord = contract.ExtensionPayload

// EnableExtensionRequest carries digest-exact network consent.
type EnableExtensionRequest = contract.EnableExtensionRequest

// ExtensionEnableRecord is the committed enable action result.
type ExtensionEnableRecord = contract.ExtensionEnableResult

// ExtensionKitItemRecord is one shipped or live extension resource.
type ExtensionKitItemRecord = contract.ExtensionKitItemPayload

// ExtensionInventoryRecord is the shipped/live resource union for one extension.
type ExtensionInventoryRecord = contract.ExtensionInventoryPayload

// ExtensionEnablePreviewRecord is the mutation-free enable projection.
type ExtensionEnablePreviewRecord = contract.ExtensionEnablePreviewPayload

// DevLinkExtensionRequest links an immutable generation to the resolved workspace.
type DevLinkExtensionRequest = contract.DevLinkExtensionRequest

// ReloadExtensionRequest swaps a dev-linked extension generation.
type ReloadExtensionRequest = contract.ReloadExtensionRequest

// ExtensionLogRecord is one redacted extension stderr record.
type ExtensionLogRecord = contract.ExtensionLogPayload

// ExtensionLogsRecord is one cursor-addressable snapshot of an extension log ring.
type ExtensionLogsRecord = contract.ExtensionLogsResponse

// ExtensionProvenanceRecord is one installed extension provenance payload.
type ExtensionProvenanceRecord = contract.ExtensionProvenancePayload

// ExtensionSecretsRecord is the presence-only extension secret projection.
type ExtensionSecretsRecord = contract.ExtensionSecretsPayload

// SetExtensionSecretsRequest is the write-only extension secret mutation payload.
type SetExtensionSecretsRequest = contract.SetExtensionSecretsRequest

// ExtensionUpdateRecord is one daemon-owned extension update result.
type ExtensionUpdateRecord = contract.ManagedExtensionUpdatePayload

// ManagedExtensionRemoveRecord is one daemon-owned extension removal result.
type ManagedExtensionRemoveRecord = contract.ManagedExtensionRemovePayload

// CreateBridgeRequest captures the shared bridge-instance creation payload.
type CreateBridgeRequest = contract.CreateBridgeRequest

// UpdateBridgeRequest captures mutable bridge-instance fields.
type UpdateBridgeRequest = contract.UpdateBridgeRequest

// BridgeTestDeliveryRequest captures the typed bridge delivery-target dry-run request.
type BridgeTestDeliveryRequest = contract.BridgeTestDeliveryRequest

// BridgeDeliveryTargetInput captures the typed bridge delivery-target override input.
type BridgeDeliveryTargetInput = contract.BridgeDeliveryTargetInput

// BridgeRecord is the shared bridge-instance response payload.
type BridgeRecord = bridgepkg.BridgeInstance

// BridgeRouteRecord is one persisted bridge route returned by the daemon API.
type BridgeRouteRecord = bridgepkg.BridgeRoute

// BridgeTargetRecord is one persisted bridge target returned by the daemon API.
type BridgeTargetRecord = bridgepkg.BridgeTarget

// BridgeTargetsRecord wraps the bridge target directory response.
type BridgeTargetsRecord = contract.BridgeTargetsResponse

// BridgeResolveTargetRequest captures one bridge target resolve lookup.
type BridgeResolveTargetRequest = contract.BridgeResolveTargetRequest

// BridgeResolveTargetRecord wraps one bridge target resolver response.
type BridgeResolveTargetRecord = contract.BridgeResolveTargetResponse

// BridgeSecretBindingRequest captures one bridge secret binding write payload.
type BridgeSecretBindingRequest = contract.PutBridgeSecretBindingRequest

// BridgeSecretBindingRecord is one bridge secret binding payload.
type BridgeSecretBindingRecord = bridgepkg.BridgeSecretBinding

// DeliveryTargetRecord is the resolved typed outbound target returned by the daemon API.
type DeliveryTargetRecord = bridgepkg.DeliveryTarget

// BridgeTestDeliveryRecord is the shared dry-run bridge delivery response payload.
type BridgeTestDeliveryRecord = contract.BridgeTestDeliveryResponse

// NotificationPresetRecord is one persisted notification preset.
type NotificationPresetRecord = contract.NotificationPresetPayload

// NotificationPresetTarget is one bridge target attached to a preset.
type NotificationPresetTarget = contract.NotificationTargetPayload

// NotificationPresetListRecord wraps notification preset list results.
type NotificationPresetListRecord = contract.NotificationPresetListResponse

// CreateNotificationPresetRequest captures notification preset creation input.
type CreateNotificationPresetRequest = contract.CreateNotificationPresetRequest

// UpdateNotificationPresetRequest captures mutable notification preset fields.
type UpdateNotificationPresetRequest = contract.UpdateNotificationPresetRequest

// NotificationPresetQuery filters preset list operations.
type NotificationPresetQuery struct {
	Enabled *bool
	BuiltIn *bool
	Name    string
	Limit   int
}

// IdentityRecord is the local agent identity exposed by `compozy whoami`.
type IdentityRecord struct {
	SessionID string `json:"session_id,omitempty"`
	Agent     string `json:"agent,omitempty"`
	AgentName string `json:"agent_name,omitempty"`
}

// ResourceRecord is one desired-state resource payload.
type ResourceRecord = contract.ResourceRecordPayload

// ResourcePutRequest captures one desired-state resource upsert.
type ResourcePutRequest = contract.PutResourceRequest

// ResourceDeleteRequest captures one desired-state resource delete request.
type ResourceDeleteRequest = contract.DeleteResourceRequest

// ResourceListQuery captures CLI filters for resource list calls.
type ResourceListQuery struct {
	Kind       resources.ResourceKind
	ScopeKind  resources.ResourceScopeKind
	ScopeID    string
	OwnerKind  resources.ResourceOwnerKind
	OwnerID    string
	SourceKind resources.ResourceSourceKind
	SourceID   string
	Limit      int
}

// SSEEvent is one parsed server-sent event frame.
type SSEEvent = sse.Event
type SSEHandler = sse.Handler

type daemonClient struct {
	target       ClientTarget
	httpClient   *http.Client
	streamClient *http.Client
}
