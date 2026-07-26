package daemon

import (
	"context"

	"fmt"

	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"

	memorypkg "github.com/compozy/agh/internal/memory"
	memcontract "github.com/compozy/agh/internal/memory/contract"

	toolspkg "github.com/compozy/agh/internal/tools"
)

func (n *daemonNativeTools) memoryAdminDreamStatus(
	_ context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	if err := decodeNativeInput(req, &struct{}{}); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(
		contract.MemoryDreamListResponse{Dreams: []contract.MemoryDreamPayload{}},
		"0 memory dreams",
	)
}

func (n *daemonNativeTools) memoryAdminDreamList(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input memoryAdminDreamListInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	if err := nativeMemoryAdminLimit(req.ToolID, input.Limit); err != nil {
		return toolspkg.ToolResult{}, err
	}
	selector, err := n.memoryAdminResolvedSelector(ctx, scope, input.memoryAdminSelectorInput)
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	records, err := n.deps.MemoryStore.ListDreamRunRecords(ctx, memorypkg.DreamRunListQuery{
		Scope:       selector.Scope,
		WorkspaceID: selector.WorkspaceID,
		AgentName:   selector.AgentName,
		AgentTier:   selector.AgentTier,
		Limit:       input.Limit,
	})
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	payloads := make([]contract.MemoryDreamPayload, 0, len(records))
	for _, record := range records {
		payloads = append(payloads, memoryAdminDreamPayload(record))
	}
	return structuredResult(
		contract.MemoryDreamListResponse{Dreams: payloads},
		fmt.Sprintf("%d memory dreams", len(payloads)),
	)
}

func (n *daemonNativeTools) memoryAdminDreamShow(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input memoryAdminDreamIDInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	dreamID, err := requiredNativeString(req.ToolID, "dream_id", input.DreamID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	record, err := n.deps.MemoryStore.LoadDreamRunRecord(ctx, dreamID)
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	return structuredResult(contract.MemoryDreamResponse{Dream: memoryAdminDreamPayload(record)}, dreamID)
}

func (n *daemonNativeTools) memoryAdminDreamTrigger(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input memoryAdminDreamTriggerInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	if n.deps.DreamTrigger == nil || !n.deps.DreamTrigger.Enabled() {
		payload := contract.MemoryDreamTriggerResponse{
			Dream: contract.MemoryDreamPayload{
				Status:      contract.MemoryDreamStateSkipped,
				Scope:       memcontract.Scope(input.Scope).Normalize(),
				WorkspaceID: strings.TrimSpace(input.WorkspaceID),
				AgentName:   strings.TrimSpace(input.AgentName),
				AgentTier:   memcontract.AgentTier(input.AgentTier).Normalize(),
				StartedAt:   time.Now().UTC(),
			},
			Triggered: false,
			Reason:    "dream consolidation is disabled",
		}
		return structuredResult(payload, payload.Reason)
	}
	triggered, reason, err := n.deps.DreamTrigger.Trigger(ctx, strings.TrimSpace(input.WorkspaceID))
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	status := contract.MemoryDreamStateSkipped
	if triggered {
		status = contract.MemoryDreamStateRunning
	}
	payload := contract.MemoryDreamTriggerResponse{
		Dream: contract.MemoryDreamPayload{
			Status:      status,
			Scope:       memcontract.Scope(input.Scope).Normalize(),
			WorkspaceID: strings.TrimSpace(input.WorkspaceID),
			AgentName:   strings.TrimSpace(input.AgentName),
			AgentTier:   memcontract.AgentTier(input.AgentTier).Normalize(),
			StartedAt:   time.Now().UTC(),
		},
		Triggered: triggered,
		Reason:    strings.TrimSpace(reason),
	}
	return structuredResult(payload, fmt.Sprintf("dream triggered=%t", triggered))
}

func (n *daemonNativeTools) memoryAdminDreamRetry(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input memoryAdminDreamRetryInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	runID := firstNonEmpty(input.FailureID, input.DreamID)
	if err := nativeMemoryAdminDreamRetryTarget(req.ToolID, input); err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	if n.deps.DreamTrigger == nil || !n.deps.DreamTrigger.Enabled() {
		payload := contract.MemoryDreamRetryResponse{
			Dream: contract.MemoryDreamPayload{
				ID:        strings.TrimSpace(runID),
				Status:    contract.MemoryDreamStateSkipped,
				StartedAt: time.Now().UTC(),
			},
			Retried: false,
		}
		return structuredResult(payload, "dream retry skipped")
	}
	triggered, reason, err := n.deps.DreamTrigger.Trigger(ctx, runID)
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	status := contract.MemoryDreamStateSkipped
	if triggered {
		status = contract.MemoryDreamStateRunning
	}
	payload := contract.MemoryDreamRetryResponse{
		Dream: contract.MemoryDreamPayload{
			ID:            strings.TrimSpace(runID),
			Status:        status,
			FailureReason: strings.TrimSpace(reason),
			StartedAt:     time.Now().UTC(),
		},
		Retried: triggered,
	}
	return structuredResult(payload, fmt.Sprintf("dream retried=%t", triggered))
}
