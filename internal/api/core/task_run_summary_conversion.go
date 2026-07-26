package core

import (
	"github.com/compozy/agh/internal/api/contract"
	taskpkg "github.com/compozy/agh/internal/task"
)

// TaskRunOperationalSummaryPayloadFromSummary converts run-detail operational metrics into the shared payload.
func TaskRunOperationalSummaryPayloadFromSummary(
	summary taskpkg.RunOperationalSummary,
) contract.TaskRunOperationalSummaryPayload {
	return contract.TaskRunOperationalSummaryPayload{
		LastActivityAt: summary.LastActivityAt,
		LastEventType:  summary.LastEventType,
		ToolCallCount:  summary.ToolCallCount,
		TurnCount:      summary.TurnCount,
		InputTokens:    summary.InputTokens,
		OutputTokens:   summary.OutputTokens,
		TotalTokens:    summary.TotalTokens,
		TotalCost:      summary.TotalCost,
		CostCurrency:   summary.CostCurrency,
		CostStatus:     contract.CostStatus(summary.CostStatus),
		CostSource:     contract.CostSource(summary.CostSource),
	}
}
