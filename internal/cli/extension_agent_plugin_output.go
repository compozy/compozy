package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	diagnosticcontract "github.com/compozy/compozy/internal/diagnosticcontract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
)

const extensionAgentPluginDiagnosticPrefix = "extension_agent_plugin_"

func extensionFormatLabel(format string) string {
	if strings.TrimSpace(format) == string(extensionpkg.FormatAgentPlugin) {
		return extensionAgentPluginValue
	}
	return string(extensionpkg.FormatCompozy)
}

func extensionSkippedComponentCount(items []contract.DiagnosticItem) int {
	count := 0
	for _, item := range items {
		if isExtensionSkippedDiagnostic(item) {
			count++
		}
	}
	return count
}

func extensionDiagnosticsHuman(title string, items []contract.DiagnosticItem) string {
	rows := make([]keyValue, 0, len(items))
	for _, item := range items {
		scope := extensionDiagnosticScope(item)
		rows = append(rows, keyValue{
			Label: strings.ToUpper(strings.TrimSpace(item.Severity)) + " " + scope,
			Value: extensionDiagnosticReason(item, scope),
		})
	}
	if len(rows) == 0 {
		return ""
	}
	return renderHumanSection(title, rows)
}

func extensionSkippedDiagnosticsHuman(items []contract.DiagnosticItem) string {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		if !isExtensionSkippedDiagnostic(item) {
			continue
		}
		scope := extensionDiagnosticScope(item)
		component := extensionDiagnosticComponentFromScope(scope)
		if value, ok := item.Evidence[cliComponentKey].(string); ok && strings.TrimSpace(value) != "" {
			component = strings.TrimSpace(value)
		}
		rows = append(rows, []string{
			component,
			extensionDiagnosticNameFromScope(scope),
			extensionDiagnosticReason(item, scope),
		})
	}
	if len(rows) == 0 {
		return ""
	}
	return renderHumanTable("Skipped", []string{cliKindValue, cliNameValue, cliReasonValue}, rows)
}

func extensionLiveDiagnosticsHuman(items []contract.DiagnosticItem) string {
	live := make([]contract.DiagnosticItem, 0, len(items))
	for _, item := range items {
		if !isExtensionSkippedDiagnostic(item) {
			live = append(live, item)
		}
	}
	return extensionDiagnosticsHuman("Diagnostics", live)
}

func isExtensionSkippedDiagnostic(item contract.DiagnosticItem) bool {
	return item.Code == diagnosticcontract.CodeExtensionAgentPluginComponentSkipped
}

func extensionDiagnosticScope(item contract.DiagnosticItem) string {
	if value, ok := item.Evidence["scope"].(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(item.Code)
}

func extensionDiagnosticReason(item contract.DiagnosticItem, scope string) string {
	message := strings.TrimSpace(item.Message)
	if name, ok := strings.CutPrefix(scope, extensionMCPKind+":"); ok {
		prefix := fmt.Sprintf("mcp server %q: ", name)
		message = strings.TrimPrefix(message, prefix)
	}
	if name, ok := strings.CutPrefix(scope, extensionSkillKind+":"); ok {
		prefix := fmt.Sprintf("skill %q: ", name)
		message = strings.TrimPrefix(message, prefix)
	}
	return message
}

func extensionInstallSuccessBundle(item *ExtensionRecord, report *extensionpkg.ValidationReport) outputBundle {
	bundle := extensionBundle(item)
	bundle.human = func() (string, error) {
		portable := strings.TrimSpace(item.Format) == string(extensionpkg.FormatAgentPlugin)
		blocks := []string{fmt.Sprintf("✓ install %s", strings.TrimSpace(item.Name))}
		if portable {
			if report == nil {
				return "", fmt.Errorf("cli: portable install report is unavailable for %q", item.Name)
			}
			blocks = append(blocks, extensionPortableInstallReportHuman(report))
		} else if report != nil && report.DualManifest {
			blocks = append(blocks,
				"note: directory carries both extension.toml and plugin.json; installed as a CompozyOS extension "+
					"(native manifest wins)")
		}
		next := "next: compozy extension status " + strings.TrimSpace(item.Name)
		if portable {
			next = "next: compozy extension enable " + strings.TrimSpace(item.Name)
		}
		blocks = append(blocks, next, extensionHumanDetail(item, portable))
		return renderHumanBlocks(blocks...), nil
	}
	return bundle
}

func extensionUpdatesSuccessBundle(
	items []extensionUpdateItem,
	portable []extensionPortableUpdateOutput,
) outputBundle {
	bundle := extensionUpdateBundle(items)
	baseHuman := bundle.human
	bundle.human = func() (string, error) {
		table, err := baseHuman()
		if err != nil {
			return "", err
		}
		blocks := []string{table}
		for _, output := range portable {
			if output.Report == nil {
				return "", fmt.Errorf("cli: portable update report is unavailable for %q", output.Update.Name)
			}
			blocks = append(blocks,
				fmt.Sprintf("✓ update %s", strings.TrimSpace(output.Update.Name)),
				extensionPortableInstallReportHuman(output.Report),
				"note: data directory preserved ("+strings.TrimSpace(output.DataPath)+")",
				"next: compozy extension status "+strings.TrimSpace(output.Update.Name),
			)
		}
		return renderHumanBlocks(blocks...), nil
	}
	return bundle
}

func extensionPortableInstallReportHuman(report *extensionpkg.ValidationReport) string {
	ingested := make([][]string, 0, len(report.WouldIngest))
	for _, item := range report.WouldIngest {
		ingested = append(ingested, []string{item.Kind, item.Name, item.Detail})
	}
	skipped := make([][]string, 0, len(report.Issues))
	for _, issue := range report.Issues {
		if issue.Severity == extensionpkg.IssueSeverityError {
			continue
		}
		skipped = append(skipped, []string{
			extensionDiagnosticComponentFromScope(issue.Scope),
			extensionDiagnosticNameFromScope(issue.Scope),
			issue.Message,
		})
	}
	blocks := []string{
		cliFormatValue + ": " + extensionAgentPluginValue,
		renderHumanTable("Ingested", []string{cliKindValue, cliNameValue, cliDetailValue}, ingested),
	}
	if len(skipped) > 0 {
		blocks = append(
			blocks,
			renderHumanTable("Skipped", []string{cliKindValue, cliNameValue, cliReasonValue}, skipped),
		)
	}
	return renderHumanBlocks(blocks...)
}

func extensionDiagnosticComponentFromScope(scope string) string {
	switch {
	case strings.HasPrefix(scope, extensionMCPKind+":") || scope == extensionMCPKind:
		return extensionMCPServerKind
	case strings.HasPrefix(scope, extensionSkillKind+":") || scope == "skills":
		return extensionSkillKind
	default:
		return "manifest"
	}
}

func extensionDiagnosticNameFromScope(scope string) string {
	if _, name, ok := strings.Cut(scope, ":"); ok {
		return name
	}
	return scope
}

func extensionPortableValidationToon(report *extensionpkg.ValidationReport) (string, error) {
	wouldIngest, err := json.Marshal(report.WouldIngest)
	if err != nil {
		return "", fmt.Errorf("cli: marshal portable validation resources: %w", err)
	}
	issues, err := json.Marshal(report.Issues)
	if err != nil {
		return "", fmt.Errorf("cli: marshal portable validation issues: %w", err)
	}
	return renderToonObject("extension_validate", []string{
		automationStatusKey, cliFormatKey, automationNameKey, versionKey, "would_ingest", cliIssuesKey,
	}, []string{
		report.Status, report.Format, report.Name, report.Version, string(wouldIngest), string(issues),
	}), nil
}
