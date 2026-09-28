package cli

import (
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"

	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/sse"
)

// InstallExtensionRequest captures the shared extension install payload.
type InstallExtensionRequest = contract.InstallExtensionRequest

// ExtensionInstallPreviewRecord is the mutation-free install summary.
type ExtensionInstallPreviewRecord = contract.ExtensionInstallPreviewPayload

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

// EnableExtensionRequest captures extension enable inputs.
type EnableExtensionRequest = contract.EnableExtensionRequest

// ExtensionEnableRecord is the committed enable action result.
type ExtensionEnableRecord = contract.ExtensionEnableResult

// ExtensionEnablementRecord is one effective profile-specific state.
type ExtensionEnablementRecord = contract.ExtensionEnablementPayload

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
