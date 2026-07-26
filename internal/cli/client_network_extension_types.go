package cli

import (
	"net/http"

	"github.com/compozy/agh/internal/api/contract"

	bridgepkg "github.com/compozy/agh/internal/bridges"

	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/sse"
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

// UpdateExtensionRequest captures the shared extension update payload.
type UpdateExtensionRequest = contract.UpdateExtensionRequest

// ExtensionRecord is the shared extension response payload.
type ExtensionRecord = contract.ExtensionPayload

// ExtensionProvenanceRecord is one installed extension provenance payload.
type ExtensionProvenanceRecord = contract.ExtensionProvenancePayload

// ExtensionUpdateRecord is one daemon-owned extension update result.
type ExtensionUpdateRecord = contract.ManagedExtensionUpdatePayload

// ManagedExtensionRemoveRecord is one daemon-owned extension removal result.
type ManagedExtensionRemoveRecord = contract.ManagedExtensionRemovePayload

// BundleCatalogRecord is one extension bundle catalog entry.
type BundleCatalogRecord = contract.BundleCatalogPayload

// BundleActivationRecord is one concrete or previewed bundle activation payload.
type BundleActivationRecord = contract.BundleActivationPayload

// BundleNetworkSettingsRecord captures bundle-derived network defaults.
type BundleNetworkSettingsRecord = contract.BundleNetworkSettingsPayload

// BundleChannelRecord is one channel declared by a bundle profile.
type BundleChannelRecord = contract.BundleChannelPayload

// BundleProfileCatalogRecord is one bundle profile catalog summary.
type BundleProfileCatalogRecord = contract.BundleProfileCatalogPayload

// BundleAgentRecord is one agent declared by a bundle profile.
type BundleAgentRecord = contract.BundleAgentPayload

// BundleLayoutRecord is one window layout declared by a bundle profile.
type BundleLayoutRecord = contract.BundleLayoutPayload

// BundleJobRecord is one automation job declared by a bundle profile.
type BundleJobRecord = contract.BundleJobPayload

// BundleTriggerRecord is one automation trigger declared by a bundle profile.
type BundleTriggerRecord = contract.BundleTriggerPayload

// BundleBridgeRecord is one bridge preset declared by a bundle profile.
type BundleBridgeRecord = contract.BundleBridgePayload

// BundleInventoryRecord is one resource owned by a bundle activation.
type BundleInventoryRecord = contract.BundleInventoryPayload

// DeclaredNetworkChannelRecord is one bundle-declared network channel.
type DeclaredNetworkChannelRecord = contract.DeclaredNetworkChannelPayload

// ActivateBundleRequest captures bundle preview and activation inputs.
type ActivateBundleRequest = contract.ActivateBundleRequest

// UpdateBundleActivationRequest captures mutable bundle activation overlays.
type UpdateBundleActivationRequest = contract.UpdateBundleActivationRequest

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

// IdentityRecord is the local agent identity exposed by `agh whoami`.
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

type unixSocketClient struct {
	socketPath   string
	httpClient   *http.Client
	streamClient *http.Client
}
