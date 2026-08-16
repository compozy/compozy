package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	diagnosticcontract "github.com/compozy/compozy/internal/diagnosticcontract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
)

func extensionFormatLabel(format string) string {
	if strings.TrimSpace(format) == string(extensionpkg.FormatAgentPlugin) {
		return "agent plugin"
	}
	return string(extensionpkg.FormatCompozy)
}

func extensionSkippedComponentCount(items []contract.DiagnosticItem) int {
	count := 0
	for _, item := range items {
		if item.Code == diagnosticcontract.CodeExtensionAgentPluginComponentSkipped {
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
			Label: strings.ToUpper(strings.TrimSpace(string(item.Severity))) + " " + scope,
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
		if item.Code != diagnosticcontract.CodeExtensionAgentPluginComponentSkipped {
			continue
		}
		scope := extensionDiagnosticScope(item)
		component := extensionDiagnosticComponentFromScope(scope)
		if value, ok := item.Evidence["component"].(string); ok && strings.TrimSpace(value) != "" {
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
	return renderHumanTable("Skipped", []string{"Kind", "Name", "Reason"}, rows)
}

func extensionLiveDiagnosticsHuman(items []contract.DiagnosticItem) string {
	live := make([]contract.DiagnosticItem, 0, len(items))
	for _, item := range items {
		if item.Code != diagnosticcontract.CodeExtensionAgentPluginComponentSkipped {
			live = append(live, item)
		}
	}
	return extensionDiagnosticsHuman("Diagnostics", live)
}

func extensionDiagnosticScope(item contract.DiagnosticItem) string {
	if value, ok := item.Evidence["scope"].(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(item.Code)
}

func extensionDiagnosticReason(item contract.DiagnosticItem, scope string) string {
	message := strings.TrimSpace(item.Message)
	if strings.HasPrefix(scope, "mcp:") {
		name := strings.TrimPrefix(scope, "mcp:")
		prefix := fmt.Sprintf("mcp server %q: ", name)
		message = strings.TrimPrefix(message, prefix)
	}
	if strings.HasPrefix(scope, "skill:") {
		name := strings.TrimPrefix(scope, "skill:")
		prefix := fmt.Sprintf("skill %q: ", name)
		message = strings.TrimPrefix(message, prefix)
	}
	return message
}

func extensionInstallSuccessBundle(item ExtensionRecord, report *extensionpkg.ValidationReport) outputBundle {
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
				"note: directory carries both extension.toml and plugin.json; installed as a Compozy extension "+
					"(native manifest wins)",
			)
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

func extensionUpdateSuccessBundle(output extensionPortableUpdateOutput) outputBundle {
	bundle := extensionUpdateBundle([]extensionUpdateItem{output.Update})
	bundle.human = func() (string, error) {
		if output.Report == nil {
			return "", fmt.Errorf("cli: portable update report is unavailable for %q", output.Update.Name)
		}
		return renderHumanBlocks(
			fmt.Sprintf("✓ update %s", strings.TrimSpace(output.Update.Name)),
			extensionPortableInstallReportHuman(output.Report),
			"note: data directory preserved ("+strings.TrimSpace(output.DataPath)+")",
			extensionHumanDetail(output.Status, true),
		), nil
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
		"Format: agent plugin",
		renderHumanTable("Ingested", []string{"Kind", "Name", "Detail"}, ingested),
	}
	if len(skipped) > 0 {
		blocks = append(blocks, renderHumanTable("Skipped", []string{"Kind", "Name", "Reason"}, skipped))
	}
	return renderHumanBlocks(blocks...)
}

func extensionDiagnosticComponentFromScope(scope string) string {
	switch {
	case strings.HasPrefix(scope, "mcp:") || scope == "mcp":
		return "mcp_server"
	case strings.HasPrefix(scope, "skill:") || scope == "skills":
		return "skill"
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
		"status", "format", "name", "version", "would_ingest", "issues",
	}, []string{
		report.Status, report.Format, report.Name, report.Version, string(wouldIngest), string(issues),
	}), nil
}
