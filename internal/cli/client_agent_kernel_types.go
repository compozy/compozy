package cli

import "github.com/compozy/agh/internal/api/contract"

// AgentSoulRecord is the dedicated managed Soul read model.
type AgentSoulRecord = contract.AgentSoulPayload

// AgentSoulValidateRequest captures a managed Soul validation payload.
type AgentSoulValidateRequest = contract.AgentSoulValidateRequest

// AgentSoulPutRequest captures a managed Soul write payload.
type AgentSoulPutRequest = contract.AgentSoulPutRequest

// AgentSoulDeleteRequest captures a managed Soul delete payload.
type AgentSoulDeleteRequest = contract.AgentSoulDeleteRequest

// AgentSoulRollbackRequest captures a managed Soul rollback payload.
type AgentSoulRollbackRequest = contract.AgentSoulRollbackRequest

// AgentSoulHistoryRequest captures managed Soul history filters.
type AgentSoulHistoryRequest = contract.AgentSoulHistoryRequest

// AgentSoulHistoryRecord is the managed Soul history response.
type AgentSoulHistoryRecord = contract.AgentSoulHistoryResponse

// AgentSoulRevisionRecord is one managed Soul authoring revision.
type AgentSoulRevisionRecord = contract.AgentSoulRevisionPayload

// AgentSoulMutationRecord is the managed Soul mutation response.
type AgentSoulMutationRecord = contract.AgentSoulMutationResponse

// AgentHeartbeatRecord is the dedicated managed Heartbeat policy read model.
type AgentHeartbeatRecord = contract.HeartbeatPolicyPayload

// AgentHeartbeatValidateRequest captures a managed Heartbeat validation payload.
type AgentHeartbeatValidateRequest = contract.HeartbeatValidateRequest

// AgentHeartbeatPutRequest captures a managed Heartbeat write payload.
type AgentHeartbeatPutRequest = contract.HeartbeatPutRequest

// AgentHeartbeatDeleteRequest captures a managed Heartbeat delete payload.
type AgentHeartbeatDeleteRequest = contract.HeartbeatDeleteRequest

// AgentHeartbeatRollbackRequest captures a managed Heartbeat rollback payload.
type AgentHeartbeatRollbackRequest = contract.HeartbeatRollbackRequest

// AgentHeartbeatHistoryRequest captures managed Heartbeat history filters.
type AgentHeartbeatHistoryRequest = contract.HeartbeatHistoryRequest

// AgentHeartbeatHistoryRecord is the managed Heartbeat history response.
type AgentHeartbeatHistoryRecord = contract.HeartbeatHistoryResponse

// AgentHeartbeatRevisionRecord is one managed Heartbeat authoring revision.
type AgentHeartbeatRevisionRecord = contract.HeartbeatRevisionPayload

// AgentHeartbeatMutationRecord is the managed Heartbeat mutation response.
type AgentHeartbeatMutationRecord = contract.HeartbeatMutationResponse

// AgentHeartbeatStatusRequest captures Heartbeat status filters.
type AgentHeartbeatStatusRequest = contract.HeartbeatStatusRequest

// AgentHeartbeatStatusRecord is the shared Heartbeat status payload.
type AgentHeartbeatStatusRecord = contract.HeartbeatStatusResponse

// AgentHeartbeatWakeRequest captures one manual Heartbeat wake request.
type AgentHeartbeatWakeRequest = contract.HeartbeatWakeRequest

// AgentHeartbeatWakeDecisionRecord is one manual Heartbeat wake decision payload.
type AgentHeartbeatWakeDecisionRecord = contract.HeartbeatWakeDecisionPayload

// SkillRecord is the shared daemon skill payload.
type SkillRecord = contract.SkillPayload

// SkillShadowsRecord is the shared daemon skill shadow payload.
type SkillShadowsRecord = contract.SkillShadowsResponse

// SkillProvenanceRecord is the shared daemon skill provenance payload.
type SkillProvenanceRecord = contract.ProvenancePayload

// SkillQuery captures daemon skill filters.
type SkillQuery struct {
	Workspace string
	ForAgent  string
}

// SkillActionRecord is the shared skill enable/disable response payload.
type SkillActionRecord = contract.SkillActionResponse

// MarketplaceListingRecord is one shared marketplace discovery row.
type MarketplaceListingRecord = contract.MarketplaceListingPayload

// MarketplaceSearchRecord is one grouped marketplace discovery response.
type MarketplaceSearchRecord = contract.MarketplaceSearchResponse

// MarketplaceKindRecord is one marketplace kind response.
type MarketplaceKindRecord = contract.MarketplaceKindResponse

// MarketplaceEntryRecord is one exact marketplace detail response.
type MarketplaceEntryRecord = contract.MarketplaceEntryResponse

// MarketplaceRefreshRecord is one marketplace refresh response.
type MarketplaceRefreshRecord = contract.MarketplaceRefreshResponse

// SkillMarketplaceInstallRequest captures one daemon marketplace install request.
type SkillMarketplaceInstallRequest = contract.SkillMarketplaceInstallRequest

// SkillMarketplaceUpdateRequest captures one daemon marketplace update request.
type SkillMarketplaceUpdateRequest = contract.SkillMarketplaceUpdateRequest

// SkillMarketplaceInstallRecord is one daemon marketplace install result.
type SkillMarketplaceInstallRecord = contract.SkillMarketplaceInstallPayload

// SkillMarketplaceUpdateRecord is one daemon marketplace update result.
type SkillMarketplaceUpdateRecord = contract.SkillMarketplaceUpdatePayload

// SkillMarketplaceRemoveRecord is one daemon marketplace remove result.
type SkillMarketplaceRemoveRecord = contract.SkillMarketplaceRemovePayload

// WorkspaceCreateRequest captures the shared workspace registration payload.
type WorkspaceCreateRequest = contract.CreateWorkspaceRequest

// WorkspaceUpdateRequest captures mutable workspace fields.
type WorkspaceUpdateRequest = contract.UpdateWorkspaceRequest

// WorkspaceRecord is the shared daemon workspace registration payload.
type WorkspaceRecord = contract.WorkspacePayload

// WorkspaceSkillRecord is one resolved workspace skill returned by the daemon API.
type WorkspaceSkillRecord = contract.WorkspaceSkillPayload

// WorkspaceDetailRecord captures the workspace info payload returned by the daemon API.
type WorkspaceDetailRecord = contract.WorkspaceDetailPayload

// AgentEventRecord is one prompt-stream event returned by the daemon API.
type AgentEventRecord = contract.AgentEventPayload

// TokenUsageRecord is the prompt usage payload returned by the daemon API.
type TokenUsageRecord = contract.TokenUsagePayload
