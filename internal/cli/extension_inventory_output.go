package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func extensionInventoryBundle(payload ExtensionInventoryRecord) outputBundle {
	return outputBundle{
		jsonValue: payload,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, payload)
		},
		human: func() (string, error) {
			blocks := []string{
				renderHumanSection("Extension inventory", []keyValue{
					{Label: "Extension", Value: payload.Extension},
					{Label: cliFormatValue, Value: extensionFormatLabel(payload.Format)},
					{Label: automationEnabledValue, Value: fmt.Sprintf("%t", payload.Enabled)},
				}),
				extensionKitResourceHumanTable("Resources", cliLiveValue, extensionKitItemRows(payload.Items)),
			}
			if skipped := extensionSkippedDiagnosticsHuman(payload.Diagnostics); skipped != "" {
				blocks = append(blocks, skipped)
			}
			if live := extensionLiveDiagnosticsHuman(payload.Diagnostics); live != "" {
				blocks = append(blocks, live)
			}
			return renderHumanBlocks(blocks...), nil
		},
		toon: func() (string, error) {
			diagnosticsJSON, err := json.Marshal(payload.Diagnostics)
			if err != nil {
				return "", fmt.Errorf("cli: marshal extension inventory diagnostics: %w", err)
			}
			return renderHumanBlocks(
				renderToonObject(
					"extension_inventory",
					[]string{extensionExtensionKey, cliFormatKey, extensionEnabledKey, "diagnostics"},
					[]string{
						payload.Extension, payload.Format, fmt.Sprintf("%t", payload.Enabled), string(diagnosticsJSON),
					},
				),
				extensionKitResourceToonArray(cliItemsKey, cliLiveKey, extensionKitItemRows(payload.Items)),
			), nil
		},
	}
}

func extensionKitResourceHumanTable(title string, stateHeader string, rows [][]string) string {
	return renderHumanTable(title, []string{cliKindValue, cliNameValue, "ID", stateHeader}, rows)
}

func extensionKitResourceToonArray(name string, stateKey string, rows [][]string) string {
	return renderToonArray(name, []string{cliKindKey, automationNameKey, "id", stateKey}, rows)
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
