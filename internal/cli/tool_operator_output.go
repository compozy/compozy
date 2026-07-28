package cli

import (
	"fmt"

	"strings"
)

func toolListBundle(response ToolsResponseRecord) outputBundle {
	return listBundle(
		response,
		response.Tools,
		toolOperatorToolsValue,
		[]string{"TOOL ID", "BACKEND", "SOURCE", cliStatusHeader, "CALLABLE", "REASONS"},
		"tools",
		[]string{
			toolOperatorToolIDKey,
			toolOperatorBackendKey,
			automationSourceKey,
			automationStatusKey,
			"callable",
			toolOperatorReasonsKey,
		},
		func(item ToolRecord) []string {
			return []string{
				item.Descriptor.ToolID.String(),
				string(item.Descriptor.Backend.Kind),
				toolSourceSummary(item.Descriptor.Source),
				toolAvailabilitySummary(item.Availability),
				formatBool(item.Decision.Callable),
				joinReasons(item.Availability.ReasonCodes, item.Decision.ReasonCodes),
			}
		},
		func(item ToolRecord) []string {
			return []string{
				item.Descriptor.ToolID.String(),
				string(item.Descriptor.Backend.Kind),
				toolSourceSummary(item.Descriptor.Source),
				toolAvailabilitySummary(item.Availability),
				formatBool(item.Decision.Callable),
				joinReasons(item.Availability.ReasonCodes, item.Decision.ReasonCodes),
			}
		},
	)
}

func toolInfoBundle(response *ToolResponseRecord) outputBundle {
	tool := response.Tool
	return outputBundle{
		jsonValue: response,
		human: func() (string, error) {
			rows := []keyValue{
				{Label: toolOperatorToolIDValue, Value: tool.Descriptor.ToolID.String()},
				{Label: toolOperatorTitleValue, Value: stringOrDash(tool.Descriptor.DisplayTitle)},
				{Label: toolOperatorBackendValue, Value: string(tool.Descriptor.Backend.Kind)},
				{Label: toolOperatorSourceValue, Value: toolSourceSummary(tool.Descriptor.Source)},
				{Label: "Risk", Value: string(tool.Descriptor.Risk)},
				{Label: "Visibility", Value: string(tool.Descriptor.Visibility)},
				{Label: toolOperatorStatusValue, Value: toolAvailabilitySummary(tool.Availability)},
				{Label: "Callable", Value: formatBool(tool.Decision.Callable)},
				{Label: "Approval Required", Value: formatBool(tool.Decision.ApprovalRequired)},
				{
					Label: "Reasons",
					Value: stringOrDash(joinReasons(tool.Availability.ReasonCodes, tool.Decision.ReasonCodes)),
				},
				{Label: "Input Schema", Value: stringOrDash(compactJSON(tool.Descriptor.InputSchema))},
			}
			if output := compactJSON(tool.Descriptor.OutputSchema); output != "" {
				rows = append(rows, keyValue{Label: "Output Schema", Value: output})
			}
			return renderHumanSection("Tool", rows), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"tool",
				[]string{
					toolOperatorToolIDKey,
					toolOperatorBackendKey,
					automationSourceKey,
					automationStatusKey,
					"callable",
					toolOperatorReasonsKey,
				},
				[]string{
					tool.Descriptor.ToolID.String(),
					string(tool.Descriptor.Backend.Kind),
					toolSourceSummary(tool.Descriptor.Source),
					toolAvailabilitySummary(tool.Availability),
					formatBool(tool.Decision.Callable),
					joinReasons(tool.Availability.ReasonCodes, tool.Decision.ReasonCodes),
				},
			), nil
		},
	}
}

func toolApprovalBundle(response ToolApprovalRecord) outputBundle {
	return outputBundle{
		jsonValue: struct {
			Approval ToolApprovalRecord `json:"approval"`
		}{Approval: response},
		human: func() (string, error) {
			return renderHumanSection("Tool Approval", []keyValue{
				{Label: toolOperatorToolIDValue, Value: response.ToolID.String()},
				{Label: "Approval Token", Value: stringOrDash(response.ApprovalToken)},
				{Label: "Input Digest", Value: stringOrDash(response.InputDigest)},
				{Label: toolOperatorExpiresValue, Value: stringOrDash(formatTime(response.ExpiresAt))},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"tool_approval",
				[]string{toolOperatorToolIDKey, "approval_token", "input_digest", toolOperatorExpiresAtKey},
				[]string{
					response.ToolID.String(),
					response.ApprovalToken,
					response.InputDigest,
					formatTime(response.ExpiresAt),
				},
			), nil
		},
	}
}

func toolInvokeBundle(response ToolInvokeResponseRecord) outputBundle {
	return outputBundle{
		jsonValue: response,
		human: func() (string, error) {
			rows := []keyValue{
				{Label: toolOperatorToolIDValue, Value: response.ToolID.String()},
				{Label: toolOperatorStatusValue, Value: stringOrDash(response.Status)},
				{Label: "Truncated", Value: formatBool(response.Truncated)},
				{Label: cliDurationValue, Value: fmt.Sprintf("%dms", response.DurationMS)},
				{Label: "Bytes", Value: fmt.Sprintf("%d", response.Result.Bytes)},
			}
			if preview := strings.TrimSpace(response.Result.Preview); preview != "" {
				rows = append(rows, keyValue{Label: toolOperatorPreviewValue, Value: preview})
			}
			if len(response.Result.Redactions) > 0 {
				rows = append(
					rows,
					keyValue{Label: "Redactions", Value: fmt.Sprintf("%d", len(response.Result.Redactions))},
				)
			}
			return renderHumanSection("Tool Invocation", rows), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"tool_invocation",
				[]string{toolOperatorToolIDKey, automationStatusKey, "truncated", cliDurationMSKey, "bytes"},
				[]string{
					response.ToolID.String(),
					response.Status,
					formatBool(response.Truncated),
					fmt.Sprintf("%d", response.DurationMS),
					fmt.Sprintf("%d", response.Result.Bytes),
				},
			), nil
		},
	}
}

func toolsetListBundle(response ToolsetsResponseRecord) outputBundle {
	return listBundle(
		response,
		response.Toolsets,
		"Toolsets",
		[]string{"TOOLSET ID", cliStatusHeader, "EXPANDED TOOLS", "REASONS"},
		"toolsets",
		[]string{"id", automationStatusKey, "expanded_tools", toolOperatorReasonsKey},
		func(item ToolsetRecord) []string {
			return []string{
				item.ID.String(),
				stringOrDash(item.Status),
				strings.Join(toolIDsToStrings(item.ExpandedTools), ","),
				joinReasons(item.ReasonCodes),
			}
		},
		func(item ToolsetRecord) []string {
			return []string{
				item.ID.String(),
				stringOrDash(item.Status),
				strings.Join(toolIDsToStrings(item.ExpandedTools), ","),
				joinReasons(item.ReasonCodes),
			}
		},
	)
}

func toolsetInfoBundle(response ToolsetResponseRecord) outputBundle {
	toolset := response.Toolset
	return outputBundle{
		jsonValue: response,
		human: func() (string, error) {
			rows := []keyValue{
				{Label: "Toolset ID", Value: toolset.ID.String()},
				{Label: toolOperatorStatusValue, Value: stringOrDash(toolset.Status)},
				{Label: toolOperatorToolsValue, Value: stringOrDash(strings.Join(toolset.Tools, ","))},
				{
					Label: "Nested Toolsets",
					Value: stringOrDash(strings.Join(toolsetIDsToStrings(toolset.Toolsets), ",")),
				},
				{
					Label: "Expanded Tools",
					Value: stringOrDash(strings.Join(toolIDsToStrings(toolset.ExpandedTools), ",")),
				},
				{Label: "Reasons", Value: stringOrDash(joinReasons(toolset.ReasonCodes))},
			}
			return renderHumanSection("Toolset", rows), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"toolset",
				[]string{"id", automationStatusKey, "expanded_tools", toolOperatorReasonsKey},
				[]string{
					toolset.ID.String(),
					toolset.Status,
					strings.Join(toolIDsToStrings(toolset.ExpandedTools), ","),
					joinReasons(toolset.ReasonCodes),
				},
			), nil
		},
	}
}
