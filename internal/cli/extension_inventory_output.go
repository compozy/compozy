package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func extensionInventoryBundle(payload ExtensionInventoryRecord) outputBundle {
	return outputBundle{
		jsonValue: payload,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, payload)
		},
		human: func() (string, error) {
			return renderHumanBlocks(
				renderHumanSection("Extension inventory", []keyValue{
					{Label: "Extension", Value: payload.Extension},
					{Label: automationEnabledValue, Value: fmt.Sprintf("%t", payload.Enabled)},
				}),
				renderHumanTable(
					"Resources",
					[]string{bundleKindValue, bundleNameValue, "ID", cliLiveValue},
					extensionKitItemRows(payload.Items),
				),
			), nil
		},
		toon: func() (string, error) {
			return renderHumanBlocks(
				renderToonObject(
					"extension_inventory",
					[]string{extensionExtensionKey, extensionEnabledKey},
					[]string{payload.Extension, fmt.Sprintf("%t", payload.Enabled)},
				),
				renderToonArray(
					cliItemsKey,
					[]string{bundleKindKey, automationNameKey, "id", cliLiveKey},
					extensionKitItemRows(payload.Items),
				),
			), nil
		},
	}
}

func extensionEnablePreviewBundle(payload ExtensionEnablePreviewRecord) outputBundle {
	return outputBundle{
		jsonValue: payload,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, payload)
		},
		human: func() (string, error) {
			blocks := []string{renderHumanSection("Enable preview", []keyValue{
				{Label: "Extension", Value: payload.Extension},
				{Label: "Agent conflicts", Value: strings.Join(payload.AgentConflicts, ", ")},
				{Label: "Missing env", Value: strings.Join(payload.MissingEnv, ", ")},
				{Label: "Automation starting", Value: strings.Join(payload.AutomationStarting, ", ")},
				{Label: "Network digest", Value: payload.NetworkRequirementDigest},
				{Label: "Confirmation required", Value: fmt.Sprintf("%t", payload.NetworkConfirmationRequired)},
			})}
			blocks = append(blocks, renderHumanTable(
				"Would publish",
				[]string{bundleKindValue, bundleNameValue, "ID", cliLiveValue},
				extensionKitItemRows(payload.WouldPublish),
			))
			return renderHumanBlocks(blocks...), nil
		},
		toon: func() (string, error) {
			return renderHumanBlocks(
				renderToonObject(
					"extension_preview",
					[]string{
						extensionExtensionKey, "agent_conflicts", extensionMissingEnvKey, "automation_starting",
						"network_requirement_digest", "network_confirmation_required",
					},
					[]string{
						payload.Extension,
						strings.Join(payload.AgentConflicts, "|"),
						strings.Join(payload.MissingEnv, "|"),
						strings.Join(payload.AutomationStarting, "|"),
						payload.NetworkRequirementDigest,
						fmt.Sprintf("%t", payload.NetworkConfirmationRequired),
					},
				),
				renderToonArray(
					"would_publish",
					[]string{bundleKindKey, automationNameKey, "id", cliLiveKey},
					extensionKitItemRows(payload.WouldPublish),
				),
			), nil
		},
	}
}

func extensionKitItemRows(items []ExtensionKitItemRecord) [][]string {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			string(item.Kind), item.Name, item.ID, fmt.Sprintf("%t", item.Live),
		})
	}
	return rows
}
