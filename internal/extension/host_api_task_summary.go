package extensionpkg

import (
	apicontract "github.com/compozy/compozy/internal/api/contract"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func taskRunOperationalSummaryPayloadFromSummary(
	summary taskpkg.RunOperationalSummary,
) apicontract.TaskRunOperationalSummaryPayload {
	return apicontract.TaskRunOperationalSummaryPayload{
		LastActivityAt: summary.LastActivityAt,
		LastEventType:  summary.LastEventType,
		ToolCallCount:  summary.ToolCallCount,
		TurnCount:      summary.TurnCount,
		InputTokens:    summary.InputTokens,
		OutputTokens:   summary.OutputTokens,
		TotalTokens:    summary.TotalTokens,
		TotalCost:      summary.TotalCost,
		CostCurrency:   summary.CostCurrency,
		CostStatus:     apicontract.CostStatus(summary.CostStatus),
		CostSource:     apicontract.CostSource(summary.CostSource),
	}
}
