package daemon

import (
	"context"

	"fmt"

	"strings"

	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
	aghconfig "github.com/compozy/agh/internal/config"

	memcontract "github.com/compozy/agh/internal/memory/contract"

	taskpkg "github.com/compozy/agh/internal/task"
	toolspkg "github.com/compozy/agh/internal/tools"
)

const (
	nativeMemoryAdminToolsCompletedKey = "completed"
	nativeMemoryAdminToolsCanceledKey  = "canceled"
	nativeMemoryAdminToolsFailedKey    = "failed"
)

type memoryAdminAvailabilitySet struct {
	store         toolspkg.NativeAvailabilityFunc
	extractor     toolspkg.NativeAvailabilityFunc
	providers     toolspkg.NativeAvailabilityFunc
	sessionLedger toolspkg.NativeAvailabilityFunc
}

const (
	nativeMemoryMetadataIDKey             = "idempotency_key"
	nativeMemoryMetadataTargetFilenameKey = "target_filename"
	nativeMemoryHealthStatusDegraded      = "degraded"
	nativeMemoryHealthStatusDisabled      = "disabled"
)

type memoryAdminSelectorInput struct {
	Scope       string `json:"scope,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	Workspace   string `json:"workspace,omitempty"`
	AgentName   string `json:"agent_name,omitempty"`
	AgentTier   string `json:"agent_tier,omitempty"`
}

type memoryAdminHealthInput struct {
	WorkspaceID string `json:"workspace_id,omitempty"`
	Workspace   string `json:"workspace,omitempty"`
}

type memoryAdminHistoryInput struct {
	memoryAdminSelectorInput
	Operation string `json:"operation,omitempty"`
	Since     string `json:"since,omitempty"`
	Limit     int    `json:"limit,omitempty"`
}

type memoryAdminReindexInput struct {
	memoryAdminSelectorInput
	IncludeSystem bool `json:"include_system,omitempty"`
}

type memoryAdminPromoteInput struct {
	Filename       string                              `json:"filename"`
	From           contract.MemoryScopeSelectorPayload `json:"from"`
	To             contract.MemoryScopeSelectorPayload `json:"to"`
	IdempotencyKey string                              `json:"idempotency_key,omitempty"`
	DryRun         bool                                `json:"dry_run,omitempty"`
}

type memoryAdminResetInput struct {
	memoryAdminSelectorInput
	DerivedOnly bool `json:"derived_only"`
	Confirm     bool `json:"confirm"`
}

type memoryAdminRecallTraceInput struct {
	SessionID string `json:"session_id"`
	TurnSeq   int64  `json:"turn_seq"`
}

type memoryAdminDreamListInput struct {
	memoryAdminSelectorInput
	Limit int `json:"limit,omitempty"`
}

type memoryAdminDreamIDInput struct {
	DreamID string `json:"dream_id"`
}

type memoryAdminDreamTriggerInput struct {
	memoryAdminSelectorInput
	Force bool `json:"force,omitempty"`
}

type memoryAdminDreamRetryInput struct {
	FailureID string `json:"failure_id,omitempty"`
	DreamID   string `json:"dream_id,omitempty"`
	Force     bool   `json:"force,omitempty"`
}

type memoryAdminDailyListInput struct {
	memoryAdminSelectorInput
	Date  string `json:"date,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

type memoryAdminExtractorRetryInput struct {
	FailureID string `json:"failure_id,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

type memoryAdminWorkspaceInput struct {
	WorkspaceID string `json:"workspace_id,omitempty"`
}

type memoryAdminProviderInput struct {
	Name        string `json:"name"`
	WorkspaceID string `json:"workspace_id,omitempty"`
}

type memoryAdminProviderLifecycleInput struct {
	Name        string `json:"name"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

type memoryAdminSessionIDInput struct {
	WorkspaceID string `json:"workspace_id"`
	SessionID   string `json:"session_id"`
}

type memoryAdminSessionReplayInput struct {
	WorkspaceID       string `json:"workspace_id"`
	SessionID         string `json:"session_id"`
	IncludeToolEvents bool   `json:"include_tool_events,omitempty"`
	IncludeMemory     bool   `json:"include_memory,omitempty"`
}

type memoryAdminSessionsPruneInput struct {
	OlderThanHours int  `json:"older_than_hours"`
	DryRun         bool `json:"dry_run,omitempty"`
}

func (n *daemonNativeTools) memoryAdminToolBindings(
	availability memoryAdminAvailabilitySet,
) map[toolspkg.ToolID]nativeToolBinding {
	storeAvailability := availability.store
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDMemoryHealth:          {call: n.memoryAdminHealth, availability: storeAvailability},
		toolspkg.ToolIDMemoryScopeShow:       {call: n.memoryAdminScopeShow, availability: storeAvailability},
		toolspkg.ToolIDMemoryAdminHistory:    {call: n.memoryAdminHistory, availability: storeAvailability},
		toolspkg.ToolIDMemoryReindex:         {call: n.memoryAdminReindex, availability: storeAvailability},
		toolspkg.ToolIDMemoryPromote:         {call: n.memoryAdminPromote, availability: storeAvailability},
		toolspkg.ToolIDMemoryReset:           {call: n.memoryAdminReset, availability: storeAvailability},
		toolspkg.ToolIDMemoryReload:          {call: n.memoryAdminReload, availability: storeAvailability},
		toolspkg.ToolIDMemoryDecisionsList:   {call: n.memoryAdminDecisionsList, availability: storeAvailability},
		toolspkg.ToolIDMemoryDecisionsShow:   {call: n.memoryAdminDecisionsShow, availability: storeAvailability},
		toolspkg.ToolIDMemoryDecisionsRevert: {call: n.memoryAdminDecisionsRevert, availability: storeAvailability},
		toolspkg.ToolIDMemoryRecallTrace:     {call: n.memoryAdminRecallTrace, availability: storeAvailability},
		toolspkg.ToolIDMemoryDreamStatus:     {call: n.memoryAdminDreamStatus, availability: storeAvailability},
		toolspkg.ToolIDMemoryDreamList:       {call: n.memoryAdminDreamList, availability: storeAvailability},
		toolspkg.ToolIDMemoryDreamShow:       {call: n.memoryAdminDreamShow, availability: storeAvailability},
		toolspkg.ToolIDMemoryDreamTrigger:    {call: n.memoryAdminDreamTrigger, availability: storeAvailability},
		toolspkg.ToolIDMemoryDreamRetry:      {call: n.memoryAdminDreamRetry, availability: storeAvailability},
		toolspkg.ToolIDMemoryDailyList:       {call: n.memoryAdminDailyList, availability: storeAvailability},
		toolspkg.ToolIDMemoryExtractorStatus: {
			call:         n.memoryAdminExtractorStatus,
			availability: availability.extractor,
		},
		toolspkg.ToolIDMemoryExtractorFailures: {
			call:         n.memoryAdminExtractorFailures,
			availability: availability.extractor,
		},
		toolspkg.ToolIDMemoryExtractorRetry: {call: n.memoryAdminExtractorRetry, availability: availability.extractor},
		toolspkg.ToolIDMemoryExtractorDrain: {call: n.memoryAdminExtractorDrain, availability: availability.extractor},
		toolspkg.ToolIDMemoryProviderList:   {call: n.memoryAdminProviderList, availability: availability.providers},
		toolspkg.ToolIDMemoryProviderGet:    {call: n.memoryAdminProviderGet, availability: availability.providers},
		toolspkg.ToolIDMemoryProviderSelect: {call: n.memoryAdminProviderSelect, availability: availability.providers},
		toolspkg.ToolIDMemoryProviderEnable: {call: n.memoryAdminProviderEnable, availability: availability.providers},
		toolspkg.ToolIDMemoryProviderDisable: {
			call:         n.memoryAdminProviderDisable,
			availability: availability.providers,
		},
		toolspkg.ToolIDMemorySessionLedger: {
			call:         n.memoryAdminSessionLedger,
			availability: availability.sessionLedger,
		},
		toolspkg.ToolIDMemorySessionReplay: {
			call:         n.memoryAdminSessionReplay,
			availability: availability.sessionLedger,
		},
		toolspkg.ToolIDMemorySessionsPrune: {
			call:         n.memoryAdminSessionsPrune,
			availability: availability.sessionLedger,
		},
		toolspkg.ToolIDMemorySessionsRepair: {
			call:         n.memoryAdminSessionsRepair,
			availability: availability.sessionLedger,
		},
	}
}

func (n *daemonNativeTools) memoryAdminHealth(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input memoryAdminHealthInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	dreamAgent := strings.TrimSpace(n.deps.Config.Roles.Dream.Agent)
	if dreamAgent == "" {
		dreamAgent = aghconfig.BuiltinDreamingCuratorAgentName
	}
	payload := contract.MemoryHealthPayload{
		Status:             "ok",
		Enabled:            n.deps.Config.Memory.Enabled,
		Configured:         strings.TrimSpace(n.deps.Config.Memory.GlobalDir) != "",
		GlobalDir:          strings.TrimSpace(n.deps.Config.Memory.GlobalDir),
		DreamAgent:         dreamAgent,
		DreamMinHours:      n.deps.Config.Memory.Dream.MinHours,
		DreamMinSessions:   n.deps.Config.Memory.Dream.MinSessions,
		DreamCheckInterval: n.deps.Config.Memory.Dream.CheckInterval.String(),
	}
	if !payload.Enabled {
		payload.Status = nativeMemoryHealthStatusDisabled
		payload.Reason = "memory is disabled"
		return structuredResult(payload, payload.Status)
	}
	if n.deps.DreamTrigger != nil {
		payload.DreamEnabled = n.deps.DreamTrigger.Enabled()
		lastConsolidation, err := n.deps.DreamTrigger.LastConsolidatedAt()
		if err != nil {
			payload.Status = nativeMemoryHealthStatusDegraded
			payload.Reason = taskpkg.RedactClaimTokens(err.Error())
		} else if !lastConsolidation.IsZero() {
			lastConsolidation = lastConsolidation.UTC()
			payload.LastConsolidation = &lastConsolidation
		}
	}
	globalCount, err := n.deps.MemoryStore.SourceHeaderCount(memcontract.ScopeGlobal)
	if err != nil {
		payload.Status = "unavailable"
		payload.Reason = taskpkg.RedactClaimTokens(err.Error())
		return structuredResult(payload, payload.Status)
	}
	payload.GlobalFiles = globalCount
	workspaces, err := n.memoryAdminHealthWorkspaces(ctx, input)
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	payload.WorkspaceCount = len(workspaces)
	for _, workspace := range workspaces {
		count, err := n.deps.MemoryStore.ForWorkspace(workspace).SourceHeaderCount(memcontract.ScopeWorkspace)
		if err != nil {
			payload.Status = nativeMemoryHealthStatusDegraded
			payload.Reason = taskpkg.RedactClaimTokens(err.Error())
			return structuredResult(payload, payload.Status)
		}
		payload.WorkspaceFiles += count
	}
	stats, err := n.deps.MemoryStore.HealthStats(ctx, workspaces)
	if err != nil {
		payload.Status = nativeMemoryHealthStatusDegraded
		payload.Reason = taskpkg.RedactClaimTokens(err.Error())
		return structuredResult(payload, payload.Status)
	}
	payload.IndexedFiles = stats.IndexedFiles
	payload.OrphanedFiles = stats.OrphanedFiles
	payload.LastReindex = stats.LastReindex
	payload.OperationCount = stats.OperationCount
	payload.LastOperationAt = stats.LastOperationAt
	if payload.Status == "ok" && payload.OrphanedFiles > 0 {
		payload.Status = nativeMemoryHealthStatusDegraded
		payload.Reason = "memory catalog has orphaned files"
	}
	return structuredResult(payload, payload.Status)
}

func (n *daemonNativeTools) memoryAdminScopeShow(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input memoryAdminSelectorInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	location, err := n.memoryAdminLocation(ctx, scope, req.ToolID, input, n.memoryAdminDefaultScope(scope, input))
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	payload := contract.MemoryScopeShowResponse{
		Selector:   memoryAdminSelectorPayload(location),
		Precedence: memoryAdminPrecedencePayloads(location),
		Roots:      n.memoryAdminRoots(location),
	}
	return structuredResult(payload, string(payload.Selector.Scope))
}

func (n *daemonNativeTools) memoryAdminHistory(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input memoryAdminHistoryInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	since, err := parseNativeOptionalRFC3339(req.ToolID, "since", input.Since)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if err := nativeMemoryAdminLimit(req.ToolID, input.Limit); err != nil {
		return toolspkg.ToolResult{}, err
	}
	query, err := n.memoryAdminOperationHistoryQuery(ctx, scope, input)
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	query.Since = since
	records, err := n.deps.MemoryStore.History(ctx, query)
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	payload := contract.MemoryOperationHistoryResponse{
		Operations: core.MemoryOperationHistoryPayloads(records),
	}
	return structuredResult(payload, fmt.Sprintf("%d memory operations", len(payload.Operations)))
}

func (n *daemonNativeTools) memoryAdminReindex(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input memoryAdminReindexInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	location, err := n.memoryAdminLocation(
		ctx,
		scope,
		req.ToolID,
		input.memoryAdminSelectorInput,
		memcontract.ScopeGlobal,
	)
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	result, err := location.Store.Reindex(ctx, memcontract.ReindexOptions{
		Scope:     location.Scope,
		Workspace: location.Workspace,
	})
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	payload := contract.MemoryReindexResponse{
		IndexedFiles: result.IndexedFiles,
		Scope:        result.Scope,
		WorkspaceID:  firstNonEmpty(location.WorkspaceID, result.Workspace),
		AgentName:    location.AgentName,
		AgentTier:    location.AgentTier,
		CompletedAt:  result.CompletedAt.UTC(),
	}
	return structuredResult(payload, fmt.Sprintf("indexed %d files", payload.IndexedFiles))
}
