package cli

import "github.com/compozy/agh/internal/api/contract"

func taskRunOperationalSummaryRows(summary contract.TaskRunOperationalSummaryPayload) []keyValue {
	return []keyValue{
		{Label: "Last Activity", Value: stringOrDash(formatTime(summary.LastActivityAt))},
		{Label: "Last Event", Value: stringOrDash(summary.LastEventType)},
		{Label: "Tool Calls", Value: stringOrDash(formatInt64Ptr(summary.ToolCallCount))},
		{Label: cliTurnsValue, Value: stringOrDash(formatInt64Ptr(summary.TurnCount))},
		{Label: cliInputTokensValue, Value: stringOrDash(formatInt64Ptr(summary.InputTokens))},
		{Label: cliOutputTokensValue, Value: stringOrDash(formatInt64Ptr(summary.OutputTokens))},
		{Label: "Total Tokens", Value: stringOrDash(formatInt64Ptr(summary.TotalTokens))},
		{Label: "Total Cost", Value: stringOrDash(formatCostProvenance(
			summary.TotalCost,
			formatStringPtr(summary.CostCurrency),
			summary.CostStatus,
		))},
		{Label: "Cost Status", Value: stringOrDash(string(summary.CostStatus))},
		{Label: "Cost Source", Value: stringOrDash(string(summary.CostSource))},
	}
}
