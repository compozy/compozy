package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

func extensionListBundle(items []ExtensionRecord) outputBundle {
	return listBundle(
		items,
		items,
		"Extensions",
		[]string{
			automationNameValue,
			versionValue,
			extensionUpdateValue,
			sessionTypeValue,
			authoredContextStateValue,
			authoredContextSourceValue,
			"Missing Env",
			extensionCapabilitiesValue,
		},
		"extensions",
		[]string{
			automationNameKey,
			versionKey,
			updateUpdateKey,
			extensionTypeKey,
			stateKey,
			automationSourceKey,
			extensionMissingEnvKey,
			extensionCapabilitiesKey,
		},
		func(item ExtensionRecord) []string {
			return []string{
				stringOrDash(item.Name),
				stringOrDash(item.Version),
				stringOrDash(extensionUpdateLabel(item.UpdateAvailable, item.RemoteVersion)),
				stringOrDash(item.Type),
				stringOrDash(item.State),
				stringOrDash(item.Source),
				stringOrDash(strings.Join(item.MissingEnv, ", ")),
				stringOrDash(strings.Join(item.Capabilities, ", ")),
			}
		},
		func(item ExtensionRecord) []string {
			return []string{
				item.Name,
				item.Version,
				extensionUpdateLabel(item.UpdateAvailable, item.RemoteVersion),
				item.Type,
				item.State,
				item.Source,
				strings.Join(item.MissingEnv, "|"),
				strings.Join(item.Capabilities, "|"),
			}
		},
	)
}

func extensionBundle(item ExtensionRecord) outputBundle {
	return outputBundle{
		jsonValue: item,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, item)
		},
		human: func() (string, error) {
			return extensionHumanDetail(item, true), nil
		},
		toon: func() (string, error) {
			diagnosticsJSON, err := json.Marshal(item.Diagnostics)
			if err != nil {
				return "", fmt.Errorf("cli: marshal extension diagnostics output: %w", err)
			}
			return renderToonObject(extensionExtensionKey, []string{
				automationNameKey,
				versionKey,
				extensionTypeKey,
				cliFormatKey,
				automationSourceKey,
				extensionEnabledKey,
				stateKey,
				"daemon_running",
				cliPIDKey,
				"uptime_seconds",
				extensionHealthKey,
				"last_error",
				extensionCapabilitiesKey,
				configPermissionsKey,
				"requires_env",
				extensionMissingEnvKey,
				"consecutive_failures",
				"restart_backoff_ms",
				"summary",
				"diagnostics",
			}, []string{
				item.Name,
				item.Version,
				item.Type,
				item.Format,
				item.Source,
				fmt.Sprintf("%t", item.Enabled),
				item.State,
				fmt.Sprintf("%t", item.DaemonRunning),
				fmt.Sprintf("%d", item.PID),
				fmt.Sprintf("%d", item.UptimeSeconds),
				joinExtensionHealth(item.Health, item.HealthMessage),
				item.LastError,
				strings.Join(item.Capabilities, "|"),
				strings.Join(item.Permissions, "|"),
				strings.Join(item.RequiresEnv, "|"),
				strings.Join(item.MissingEnv, "|"),
				fmt.Sprintf("%d", item.ConsecutiveFailures),
				fmt.Sprintf("%d", item.RestartBackoffMS),
				extensionRuntimeSummary(item),
				string(diagnosticsJSON),
			}), nil
		},
	}
}

func extensionHumanDetail(item ExtensionRecord, includeFormat bool) string {
	rows := []keyValue{
		{Label: automationNameValue, Value: stringOrDash(item.Name)},
		{Label: versionValue, Value: stringOrDash(item.Version)},
		{Label: sessionTypeValue, Value: stringOrDash(item.Type)},
	}
	if includeFormat {
		rows = append(rows, keyValue{Label: cliFormatValue, Value: extensionFormatLabel(item.Format)})
	}
	rows = append(
		rows,
		keyValue{Label: authoredContextSourceValue, Value: stringOrDash(item.Source)},
		keyValue{Label: extensionEnabledValue, Value: fmt.Sprintf("%t", item.Enabled)},
		keyValue{Label: authoredContextStateValue, Value: stringOrDash(item.State)},
		keyValue{Label: "Daemon", Value: boolLabel(item.DaemonRunning, "running", "offline")},
		keyValue{Label: cliPIDValue, Value: intOrDash(item.PID)},
		keyValue{Label: cliUptimeValue, Value: stringOrDash(formatExtensionUptime(item.UptimeSeconds))},
		keyValue{
			Label: extensionHealthValue,
			Value: stringOrDash(joinExtensionHealth(item.Health, item.HealthMessage)),
		},
		keyValue{Label: extensionCapabilitiesValue, Value: stringOrDash(strings.Join(item.Capabilities, ", "))},
		keyValue{Label: installPermissionsValue, Value: stringOrDash(strings.Join(item.Permissions, ", "))},
		keyValue{Label: "Requires Env", Value: stringOrDash(strings.Join(item.RequiresEnv, ", "))},
		keyValue{Label: "Missing Env", Value: stringOrDash(strings.Join(item.MissingEnv, ", "))},
		keyValue{Label: "Consecutive Failures", Value: fmt.Sprintf("%d", item.ConsecutiveFailures)},
		keyValue{Label: "Restart Backoff", Value: formatExtensionBackoff(item.RestartBackoffMS)},
		keyValue{Label: "Summary", Value: extensionRuntimeSummary(item)},
		keyValue{Label: "Last Error", Value: stringOrDash(item.LastError)},
	)
	blocks := []string{renderHumanSection("Extension", rows)}
	if diagnostics := extensionDiagnosticsHuman("Diagnostics", item.Diagnostics); diagnostics != "" {
		blocks = append(blocks, diagnostics)
	}
	return renderHumanBlocks(blocks...)
}

func extensionEnableBundle(result ExtensionEnableRecord) outputBundle {
	return outputBundle{
		jsonValue: result,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, result)
		},
		human: func() (string, error) {
			detail, err := extensionBundle(result.Extension).human()
			if err != nil {
				return "", err
			}
			blocks := []string{fmt.Sprintf("✓ Enabled %s", strings.TrimSpace(result.Extension.Name))}
			if len(result.AutomationStarted) > 0 {
				automation := renderHumanSection("Automation started", []keyValue{
					{Label: "Definitions", Value: strings.Join(result.AutomationStarted, ", ")},
				})
				blocks = append(blocks, automation)
			}
			blocks = append(blocks, extensionEnableNextStep(result.Extension), detail)
			return renderHumanBlocks(blocks...), nil
		},
		toon: func() (string, error) {
			extensionJSON, err := json.Marshal(result.Extension)
			if err != nil {
				return "", fmt.Errorf("cli: marshal enabled extension output: %w", err)
			}
			return renderToonObject(
				"extension_enable",
				[]string{extensionExtensionKey, "automation_started"},
				[]string{string(extensionJSON), strings.Join(result.AutomationStarted, "|")},
			), nil
		},
	}
}

func extensionEnableNextStep(item ExtensionRecord) string {
	if len(item.MissingEnv) > 0 {
		return "next: compozy extension secrets set " + strings.TrimSpace(item.Name) +
			" --env " + strings.TrimSpace(item.MissingEnv[0])
	}
	return "next: compozy extension status " + strings.TrimSpace(item.Name)
}

func extensionSuccessBundle(verb string, item ExtensionRecord) outputBundle {
	bundle := extensionBundle(item)
	baseHuman := bundle.human
	bundle.human = func() (string, error) {
		detail, err := baseHuman()
		if err != nil {
			return "", err
		}
		return renderHumanBlocks(
			fmt.Sprintf("✓ %s %s", strings.TrimSpace(verb), strings.TrimSpace(item.Name)),
			"next: "+extensionSuccessNextCommand(verb, item.Name),
			detail,
		), nil
	}
	return bundle
}

func extensionSuccessNextCommand(verb string, name string) string {
	trimmedName := strings.TrimSpace(name)
	switch strings.TrimSpace(verb) {
	case extensionDevVerb, extensionReloadVerb:
		return "compozy extension logs " + trimmedName + " --follow"
	default:
		return "compozy extension status " + trimmedName
	}
}

func extensionUpdateLabel(available bool, remoteVersion string) string {
	if !available {
		return ""
	}
	if version := strings.TrimSpace(remoteVersion); version != "" {
		return "→ " + version
	}
	return extensionUpdateAvailable
}

func formatExtensionBackoff(milliseconds int64) string {
	if milliseconds <= 0 {
		return "0s"
	}
	if milliseconds > math.MaxInt64/int64(time.Millisecond) {
		return fmt.Sprintf("%dms", milliseconds)
	}
	return (time.Duration(milliseconds) * time.Millisecond).String()
}

func extensionRuntimeSummary(item ExtensionRecord) string {
	if item.ConsecutiveFailures > 0 {
		return fmt.Sprintf(
			"crash-looping (%d failures, backoff %s)",
			item.ConsecutiveFailures,
			formatExtensionBackoff(item.RestartBackoffMS),
		)
	}
	if !item.Enabled {
		return daemonDisabledKey
	}
	if item.DaemonRunning && strings.EqualFold(strings.TrimSpace(item.Health), "healthy") {
		if skipped := extensionSkippedComponentCount(item.Diagnostics); skipped > 0 {
			component := "components"
			if skipped == 1 {
				component = cliComponentKey
			}
			return fmt.Sprintf("running (%d %s skipped)", skipped, component)
		}
		return "running and healthy"
	}
	if state := strings.TrimSpace(item.State); state != "" {
		return state
	}
	return extensionRuntimeUnknown
}

func formatExtensionUptime(seconds int64) string {
	if seconds <= 0 {
		return ""
	}

	if seconds < int64(time.Minute/time.Second) {
		return fmt.Sprintf("%ds", seconds)
	}
	if seconds < int64(time.Hour/time.Second) {
		return fmt.Sprintf("%dm", seconds/int64(time.Minute/time.Second))
	}

	hours := seconds / int64(time.Hour/time.Second)
	minutes := seconds % int64(time.Hour/time.Second) / int64(time.Minute/time.Second)
	if minutes == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh %dm", hours, minutes)
}

func joinExtensionHealth(health string, message string) string {
	if strings.TrimSpace(health) == "" {
		return ""
	}
	if strings.TrimSpace(message) == "" {
		return health
	}
	return health + " (" + message + ")"
}

func extensionProvenanceBundle(item ExtensionProvenanceRecord) outputBundle {
	return outputBundle{
		jsonValue: item,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, item)
		},
		human: func() (string, error) {
			return renderHumanSection("Extension Provenance", []keyValue{
				{Label: extensionMarketplaceSlugValue, Value: stringOrDash(item.Slug)},
				{Label: "Installed From", Value: stringOrDash(item.InstalledFrom)},
				{Label: "Source URL", Value: stringOrDash(item.SourceURL)},
				{Label: "Checksum", Value: stringOrDash(item.ChecksumSHA256)},
				{Label: "Checksum Verified", Value: fmt.Sprintf("%t", item.ChecksumVerified)},
				{Label: "Registry Tier", Value: stringOrDash(item.RegistryTier)},
				{Label: "Allow Unverified", Value: fmt.Sprintf("%t", item.AllowUnverified)},
				{Label: "Installed By", Value: stringOrDash(item.InstalledBy)},
				{Label: "Trust", Value: stringOrDash(extensionTrustDecisionLabel(item.Trust))},
				{Label: "Permissions", Value: stringOrDash(strings.Join(item.Permissions, ", "))},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject("extension_provenance", []string{
				extensionMarketplaceSlugKey,
				"installed_from",
				"source_url",
				"checksum_sha256",
				"checksum_verified",
				"registry_tier",
				"allow_unverified",
				"installed_by",
				"trust",
				"permissions",
			}, []string{
				item.Slug,
				item.InstalledFrom,
				item.SourceURL,
				item.ChecksumSHA256,
				fmt.Sprintf("%t", item.ChecksumVerified),
				item.RegistryTier,
				fmt.Sprintf("%t", item.AllowUnverified),
				item.InstalledBy,
				extensionTrustDecisionLabel(item.Trust),
				strings.Join(item.Permissions, "|"),
			}), nil
		},
	}
}

func extensionTrustDecisionLabel(trust *contract.ExtensionTrustReportPayload) string {
	if trust == nil {
		return ""
	}
	return strings.TrimSpace(trust.Decision)
}

func boolLabel(value bool, whenTrue string, whenFalse string) string {
	if value {
		return whenTrue
	}
	return whenFalse
}
