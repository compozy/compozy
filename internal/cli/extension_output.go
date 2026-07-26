package cli

import (
	"fmt"

	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"
)

func extensionListBundle(items []ExtensionRecord) outputBundle {
	return listBundle(
		items,
		items,
		"Extensions",
		[]string{
			automationNameValue,
			versionValue,
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
			extensionTypeKey,
			stateKey,
			automationSourceKey,
			"missing_env",
			extensionCapabilitiesKey,
		},
		func(item ExtensionRecord) []string {
			return []string{
				stringOrDash(item.Name),
				stringOrDash(item.Version),
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
		human: func() (string, error) {
			return renderHumanSection("Extension", []keyValue{
				{Label: automationNameValue, Value: stringOrDash(item.Name)},
				{Label: versionValue, Value: stringOrDash(item.Version)},
				{Label: sessionTypeValue, Value: stringOrDash(item.Type)},
				{Label: authoredContextSourceValue, Value: stringOrDash(item.Source)},
				{Label: extensionEnabledValue, Value: fmt.Sprintf("%t", item.Enabled)},
				{Label: authoredContextStateValue, Value: stringOrDash(item.State)},
				{Label: "Daemon", Value: boolLabel(item.DaemonRunning, "running", "offline")},
				{Label: cliPIDValue, Value: intOrDash(item.PID)},
				{Label: cliUptimeValue, Value: stringOrDash(formatExtensionUptime(item.UptimeSeconds))},
				{
					Label: extensionHealthValue,
					Value: stringOrDash(joinExtensionHealth(item.Health, item.HealthMessage)),
				},
				{Label: extensionCapabilitiesValue, Value: stringOrDash(strings.Join(item.Capabilities, ", "))},
				{Label: "Actions", Value: stringOrDash(strings.Join(item.Actions, ", "))},
				{Label: "Requires Env", Value: stringOrDash(strings.Join(item.RequiresEnv, ", "))},
				{Label: "Missing Env", Value: stringOrDash(strings.Join(item.MissingEnv, ", "))},
				{Label: "Last Error", Value: stringOrDash(item.LastError)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(extensionExtensionKey, []string{
				automationNameKey,
				versionKey,
				extensionTypeKey,
				automationSourceKey,
				extensionEnabledKey,
				stateKey,
				"daemon_running",
				cliPIDKey,
				"uptime_seconds",
				extensionHealthKey,
				"last_error",
				extensionCapabilitiesKey,
				"actions",
				"requires_env",
				"missing_env",
			}, []string{
				item.Name,
				item.Version,
				item.Type,
				item.Source,
				fmt.Sprintf("%t", item.Enabled),
				item.State,
				fmt.Sprintf("%t", item.DaemonRunning),
				fmt.Sprintf("%d", item.PID),
				fmt.Sprintf("%d", item.UptimeSeconds),
				joinExtensionHealth(item.Health, item.HealthMessage),
				item.LastError,
				strings.Join(item.Capabilities, "|"),
				strings.Join(item.Actions, "|"),
				strings.Join(item.RequiresEnv, "|"),
				strings.Join(item.MissingEnv, "|"),
			}), nil
		},
	}
}

func formatExtensionUptime(seconds int64) string {
	if seconds <= 0 {
		return ""
	}

	duration := time.Duration(seconds) * time.Second
	if duration < time.Minute {
		return fmt.Sprintf("%ds", seconds)
	}
	if duration < time.Hour {
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	}

	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
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
