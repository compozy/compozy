package cli

import (
	"strconv"
	"strings"
)

func bundleProfileNames(item BundleCatalogRecord) []string {
	names := make([]string, 0, len(item.Profiles))
	for _, profile := range item.Profiles {
		names = append(names, strings.TrimSpace(profile.Name))
	}
	return names
}

type bundleCatalogResourceCounts struct {
	layouts  int
	agents   int
	jobs     int
	triggers int
	bridges  int
}

func bundleCatalogCounts(item BundleCatalogRecord) bundleCatalogResourceCounts {
	var counts bundleCatalogResourceCounts
	for _, profile := range item.Profiles {
		counts.layouts += profile.LayoutCount
		counts.agents += profile.AgentCount
		counts.jobs += profile.JobCount
		counts.triggers += profile.TriggerCount
		counts.bridges += profile.BridgeCount
	}
	return counts
}

func bundleChannelsTable(items []BundleChannelRecord) string {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			stringOrDash(item.Name),
			formatBool(item.Primary),
			stringOrDash(item.Description),
		})
	}
	return renderHumanTable(
		"Channels",
		[]string{bundleNameValue, "Primary", bundleDescriptionValue},
		rows,
	)
}

func bundleAgentsTable(items []BundleAgentRecord) string {
	return renderHumanTable(
		"Agents",
		[]string{bundleNameValue, agentKernelProviderValue, bundleModelValue, "Soul", "Heartbeat"},
		bundleAgentRows(items),
	)
}

func bundleLayoutsTable(items []BundleLayoutRecord) string {
	return renderHumanTable(
		"Layouts",
		[]string{bundleNameValue, "Aspect", "Slots", "Overflow", "Participant Slots"},
		bundleLayoutRows(items),
	)
}

func bundleJobsTable(items []BundleJobRecord) string {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			stringOrDash(item.Name),
			stringOrDash(item.AgentName),
			formatBool(item.Enabled),
		})
	}
	return renderHumanTable(
		"Jobs",
		[]string{bundleNameValue, bundleAgentValue, extensionEnabledValue},
		rows,
	)
}

func bundleTriggersTable(items []BundleTriggerRecord) string {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			stringOrDash(item.Name),
			stringOrDash(item.AgentName),
			stringOrDash(item.Event),
			formatBool(item.Enabled),
		})
	}
	return renderHumanTable(
		"Triggers",
		[]string{bundleNameValue, bundleAgentValue, bundleEventValue, extensionEnabledValue},
		rows,
	)
}

func bundleBridgesTable(items []BundleBridgeRecord) string {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			stringOrDash(item.Name),
			stringOrDash(item.ExtensionName),
			stringOrDash(item.Platform),
			stringOrDash(item.DisplayName),
			strconv.Itoa(len(item.SecretSlots)),
		})
	}
	return renderHumanTable(
		"Bridges",
		[]string{
			bundleNameValue,
			bridgeExtensionValue,
			bundlePlatformValue,
			"Display",
			"Secret Slots",
		},
		rows,
	)
}

func bundleInventoryTable(items []BundleInventoryRecord) string {
	return renderHumanTable(
		"Inventory",
		[]string{bundleKindValue, "ID", bundleNameValue},
		bundleInventoryRows(items),
	)
}
