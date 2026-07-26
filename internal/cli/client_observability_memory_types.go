package cli

import (
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"

	"github.com/compozy/agh/internal/api/contract"
)

// HookCatalogQuery captures the CLI filters for resolved hook catalog queries.
type HookCatalogQuery = contract.HookCatalogQuery

// HookCatalogRecord is one resolved hook returned by the daemon API.
type HookCatalogRecord = contract.HookCatalogPayload

// HookRunsQuery captures the CLI filters for hook execution history queries.
type HookRunsQuery = contract.HookRunsQuery

// HookRunRecord is one persisted hook execution audit record.
type HookRunRecord = contract.HookRunPayload

// HookEventsQuery captures the CLI filters for hook taxonomy queries.
type HookEventsQuery = contract.HookEventsQuery

// HookEventRecord is one supported hook taxonomy row returned by the daemon API.
type HookEventRecord = contract.HookEventPayload

// LogEventRecord is one cross-session runtime log row.
type LogEventRecord = contract.LogEventPayload

// LogsListQuery captures the CLI filters for cross-session runtime log queries.
type LogsListQuery struct {
	WorkspaceRef  string
	SessionID     string
	AgentName     string
	Type          string
	RunID         string
	ActorKind     string
	ActorID       string
	Provider      string
	Outcome       string
	Component     string
	ErrorOnly     bool
	AfterSequence int64
	Since         time.Time
	Last          int
}

// MemoryHealthRecord is the shared daemon memory health payload.
type MemoryHealthRecord = contract.MemoryHealthPayload

// MemorySelectorQuery captures scope selectors sent through Memory v2 query parameters.
type MemorySelectorQuery struct {
	Scope         memcontract.Scope
	WorkspaceID   string
	AgentName     string
	AgentTier     memcontract.AgentTier
	IncludeSystem bool
}

// MemoryHistoryQuery captures filters for memory operation history.
type MemoryHistoryQuery struct {
	Scope       memcontract.Scope
	WorkspaceID string
	AgentName   string
	AgentTier   memcontract.AgentTier
	Operation   string
	Since       time.Time
	Limit       int
}

// MemoryHistoryRecord is one redacted memory operation history row.
type MemoryHistoryRecord = contract.MemoryOperationHistoryPayload

// MemoryEntryRecord wraps one Memory v2 entry.
type MemoryEntryRecord = contract.MemoryEntryResponse

// MemoryCreateRequest captures the daemon API memory create/propose payload.
type MemoryCreateRequest = contract.MemoryCreateRequest

// MemoryEditRequest captures the daemon API memory edit/propose payload.
type MemoryEditRequest = contract.MemoryEditRequest

// MemoryDeleteRecord captures the daemon API memory delete response.
type MemoryDeleteRecord = contract.MemoryDeleteResponse

// MemoryMutationRecord captures the daemon API memory write/edit response.
type MemoryMutationRecord = contract.MemoryMutationDecisionResponse

// MemorySearchRequest captures the daemon API deterministic recall/search payload.
type MemorySearchRequest = contract.MemorySearchRequest

// MemorySearchRecord wraps deterministic recall/search output.
type MemorySearchRecord = contract.MemorySearchResponse

// MemoryReindexRequest captures the daemon API memory reindex payload.
type MemoryReindexRequest = contract.MemoryReindexV2Request

// MemoryReindexRecord captures the daemon API memory reindex response.
type MemoryReindexRecord = contract.MemoryReindexResponse

// MemoryPromoteRequest captures the daemon API memory promotion payload.
type MemoryPromoteRequest = contract.MemoryPromoteRequest

// MemoryPromoteRecord captures the daemon API memory promotion response.
type MemoryPromoteRecord = contract.MemoryPromoteResponse

// MemoryResetRequest captures the daemon API memory reset payload.
type MemoryResetRequest = contract.MemoryResetRequest

// MemoryResetRecord captures the daemon API memory reset response.
type MemoryResetRecord = contract.MemoryResetResponse

// MemoryReloadRecord captures the daemon API memory reload response.
type MemoryReloadRecord = contract.MemoryReloadResponse

// MemoryScopeShowRecord captures effective memory scope resolution.
type MemoryScopeShowRecord = contract.MemoryScopeShowResponse

// MemoryRecallTraceRecord captures one redaction-safe recall trace.
type MemoryRecallTraceRecord = contract.MemoryRecallTraceResponse

// MemoryDreamListRecord wraps dreaming runtime records.
type MemoryDreamListRecord = contract.MemoryDreamListResponse

// MemoryDreamRecord wraps one dreaming runtime record.
type MemoryDreamRecord = contract.MemoryDreamResponse

// MemoryDreamTriggerRequest captures a dreaming trigger request.
type MemoryDreamTriggerRequest = contract.MemoryDreamTriggerRequest

// MemoryDreamTriggerRecord captures a dreaming trigger response.
type MemoryDreamTriggerRecord = contract.MemoryDreamTriggerResponse

// MemoryDreamRetryRequest captures a dreaming retry request.
type MemoryDreamRetryRequest = contract.MemoryDreamRetryRequest

// MemoryDreamRetryRecord captures a dreaming retry response.
type MemoryDreamRetryRecord = contract.MemoryDreamRetryResponse

// MemoryDailyLogListRecord wraps daily memory log artifacts.
type MemoryDailyLogListRecord = contract.MemoryDailyLogListResponse

// MemoryExtractorStatusRecord wraps extractor runtime status.
type MemoryExtractorStatusRecord = contract.MemoryExtractorStatusResponse

// MemoryExtractorFailuresRecord wraps extractor DLQ records.
type MemoryExtractorFailuresRecord = contract.MemoryExtractorFailuresResponse

// MemoryExtractorRetryRequest captures an extractor retry request.
type MemoryExtractorRetryRequest = contract.MemoryExtractorRetryRequest

// MemoryExtractorRetryRecord captures extractor retry results.
type MemoryExtractorRetryRecord = contract.MemoryExtractorRetryResponse

// MemoryExtractorDrainRecord captures extractor drain completion.
type MemoryExtractorDrainRecord = contract.MemoryExtractorDrainResponse

// MemoryProviderListRecord wraps registered memory providers.
type MemoryProviderListRecord = contract.MemoryProviderListResponse

// MemoryProviderRecord wraps one memory provider.
type MemoryProviderRecord = contract.MemoryProviderResponse

// MemoryProviderSelectRequest captures active-provider selection.
type MemoryProviderSelectRequest = contract.MemoryProviderSelectRequest

// MemoryProviderLifecycleRequest captures provider lifecycle mutation.
type MemoryProviderLifecycleRequest = contract.MemoryProviderLifecycleRequest

// MemoryProviderLifecycleRecord captures provider lifecycle state after mutation.
type MemoryProviderLifecycleRecord = contract.MemoryProviderLifecycleResponse

// MemoryAdhocNoteRequest captures the ad-hoc memory note write surface.
type MemoryAdhocNoteRequest = contract.MemoryAdhocNoteRequest

// MemoryAdhocNoteRecord captures the created ad-hoc memory note artifact.
type MemoryAdhocNoteRecord = contract.MemoryAdhocNoteResponse
