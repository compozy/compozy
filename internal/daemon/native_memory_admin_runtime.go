package daemon

import (
	"context"

	"fmt"

	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"

	memorypkg "github.com/compozy/agh/internal/memory"

	toolspkg "github.com/compozy/agh/internal/tools"
)

func (n *daemonNativeTools) memoryAdminDailyList(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input memoryAdminDailyListInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	if err := nativeMemoryAdminLimit(req.ToolID, input.Limit); err != nil {
		return toolspkg.ToolResult{}, err
	}
	date := strings.TrimSpace(input.Date)
	if date != "" {
		if _, err := time.Parse("2006-01-02", date); err != nil {
			return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, core.NewMemoryValidationError(err))
		}
	}
	selector, err := n.memoryAdminResolvedSelector(ctx, scope, input.memoryAdminSelectorInput)
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	records, err := n.deps.MemoryStore.ListDailyLogRecords(ctx, memorypkg.DailyLogListQuery{
		Date:        date,
		Scope:       selector.Scope,
		WorkspaceID: selector.WorkspaceID,
		AgentName:   selector.AgentName,
		AgentTier:   selector.AgentTier,
		Limit:       input.Limit,
	})
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	payloads := make([]contract.MemoryDailyLogPayload, 0, len(records))
	for _, record := range records {
		payloads = append(payloads, memoryAdminDailyLogPayload(record))
	}
	return structuredResult(
		contract.MemoryDailyLogListResponse{Logs: payloads},
		fmt.Sprintf("%d memory daily logs", len(payloads)),
	)
}

func (n *daemonNativeTools) memoryAdminExtractorStatus(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	if err := decodeNativeInput(req, &struct{}{}); err != nil {
		return toolspkg.ToolResult{}, err
	}
	status, err := n.deps.MemoryExtractor.Status(ctx)
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	return structuredResult(contract.MemoryExtractorStatusResponse{Extractor: status}, string(status.Status))
}

func (n *daemonNativeTools) memoryAdminExtractorFailures(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	if err := decodeNativeInput(req, &struct{}{}); err != nil {
		return toolspkg.ToolResult{}, err
	}
	failures, err := n.deps.MemoryExtractor.ListFailures(ctx)
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	return structuredResult(
		contract.MemoryExtractorFailuresResponse{Failures: failures},
		fmt.Sprintf("%d extractor failures", len(failures)),
	)
}

func (n *daemonNativeTools) memoryAdminExtractorRetry(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input memoryAdminExtractorRetryInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	response, err := n.deps.MemoryExtractor.Retry(ctx, contract.MemoryExtractorRetryRequest{
		FailureID: strings.TrimSpace(input.FailureID),
		SessionID: strings.TrimSpace(input.SessionID),
	})
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	return structuredResult(response, fmt.Sprintf("retried %d extractor failures", response.Retried))
}

func (n *daemonNativeTools) memoryAdminExtractorDrain(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	if err := decodeNativeInput(req, &struct{}{}); err != nil {
		return toolspkg.ToolResult{}, err
	}
	response, err := n.deps.MemoryExtractor.Drain(ctx)
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	return structuredResult(response, fmt.Sprintf("%d extractor items remaining", response.Remaining))
}

func (n *daemonNativeTools) memoryAdminProviderList(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input memoryAdminWorkspaceInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	providers, err := n.deps.MemoryProviders.List(ctx, strings.TrimSpace(input.WorkspaceID))
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	return structuredResult(
		contract.MemoryProviderListResponse{Providers: providers},
		fmt.Sprintf("%d memory providers", len(providers)),
	)
}

func (n *daemonNativeTools) memoryAdminProviderGet(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input memoryAdminProviderInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	name, err := requiredNativeString(req.ToolID, "name", input.Name)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	provider, err := n.deps.MemoryProviders.Get(ctx, strings.TrimSpace(input.WorkspaceID), name)
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	return structuredResult(contract.MemoryProviderResponse{Provider: provider}, provider.Name)
}

func (n *daemonNativeTools) memoryAdminProviderSelect(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input memoryAdminProviderInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	name, err := requiredNativeString(req.ToolID, "name", input.Name)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	provider, err := n.deps.MemoryProviders.Select(ctx, strings.TrimSpace(input.WorkspaceID), name)
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	return structuredResult(contract.MemoryProviderResponse{Provider: provider}, provider.Name)
}

func (n *daemonNativeTools) memoryAdminProviderEnable(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input memoryAdminProviderLifecycleInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	name, err := requiredNativeString(req.ToolID, "name", input.Name)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	response, err := n.deps.MemoryProviders.Enable(ctx, strings.TrimSpace(input.WorkspaceID), name, input.Reason)
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	return structuredResult(
		response,
		fmt.Sprintf("memory provider %s changed=%t", response.Provider.Name, response.Changed),
	)
}

func (n *daemonNativeTools) memoryAdminProviderDisable(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input memoryAdminProviderLifecycleInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	name, err := requiredNativeString(req.ToolID, "name", input.Name)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	response, err := n.deps.MemoryProviders.Disable(ctx, strings.TrimSpace(input.WorkspaceID), name, input.Reason)
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	return structuredResult(
		response,
		fmt.Sprintf("memory provider %s changed=%t", response.Provider.Name, response.Changed),
	)
}
