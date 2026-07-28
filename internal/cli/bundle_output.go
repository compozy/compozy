package cli

import (
	"strconv"
	"strings"
)

func bundleCatalogBundle(items []BundleCatalogRecord) outputBundle {
	return listBundle(
		struct {
			Bundles []BundleCatalogRecord `json:"bundles"`
		}{Bundles: items},
		items,
		"Bundle Catalog",
		[]string{
			bridgeExtensionValue,
			bundleBundleValue,
			"Profiles",
			"Layouts",
			"Agents",
			"Jobs",
			"Triggers",
			"Bridges",
		},
		"bundles",
		[]string{
			bundleExtensionKey,
			bundleBundleKey,
			"profiles",
			"layouts",
			"agents",
			"jobs",
			"triggers",
			"bridges",
		},
		func(item BundleCatalogRecord) []string {
			counts := bundleCatalogCounts(item)
			return []string{
				stringOrDash(item.ExtensionName),
				stringOrDash(item.BundleName),
				strings.Join(bundleProfileNames(item), ","),
				strconv.Itoa(counts.layouts),
				strconv.Itoa(counts.agents),
				strconv.Itoa(counts.jobs),
				strconv.Itoa(counts.triggers),
				strconv.Itoa(counts.bridges),
			}
		},
		func(item BundleCatalogRecord) []string {
			counts := bundleCatalogCounts(item)
			return []string{
				item.ExtensionName,
				item.BundleName,
				strings.Join(bundleProfileNames(item), "|"),
				strconv.Itoa(counts.layouts),
				strconv.Itoa(counts.agents),
				strconv.Itoa(counts.jobs),
				strconv.Itoa(counts.triggers),
				strconv.Itoa(counts.bridges),
			}
		},
	)
}

func bundleActivationListBundle(items []BundleActivationRecord) outputBundle {
	return listBundle(
		struct {
			Activations []BundleActivationRecord `json:"activations"`
		}{Activations: items},
		items,
		"Bundle Activations",
		[]string{
			"ID",
			versionValue,
			bridgeExtensionValue,
			bundleBundleValue,
			bundleProfileValue,
			automationScopeValue,
			authoredContextWorkspaceValue,
			"Layouts",
			"Agents",
			"Inventory",
		},
		"bundle_activations",
		[]string{
			"id",
			versionKey,
			bundleExtensionKey,
			bundleBundleKey,
			bundleProfileKey,
			automationScopeKey,
			workspaceSkillSource,
			"layouts",
			"agents",
			"inventory",
		},
		func(item BundleActivationRecord) []string {
			return []string{
				stringOrDash(item.ID),
				strconv.FormatInt(item.Version, 10),
				stringOrDash(item.ExtensionName),
				stringOrDash(item.BundleName),
				stringOrDash(item.ProfileName),
				stringOrDash(item.Scope),
				stringOrDash(item.WorkspaceID),
				strconv.Itoa(len(item.Layouts)),
				strconv.Itoa(len(item.Agents)),
				strconv.Itoa(len(item.Inventory)),
			}
		},
		func(item BundleActivationRecord) []string {
			return []string{
				item.ID,
				strconv.FormatInt(item.Version, 10),
				item.ExtensionName,
				item.BundleName,
				item.ProfileName,
				item.Scope,
				item.WorkspaceID,
				strconv.Itoa(len(item.Layouts)),
				strconv.Itoa(len(item.Agents)),
				strconv.Itoa(len(item.Inventory)),
			}
		},
	)
}

func bundleActivationBundle(item BundleActivationRecord) outputBundle {
	return outputBundle{
		jsonValue: item,
		human: func() (string, error) {
			return renderHumanBlocks(
				renderHumanSection("Bundle Activation", []keyValue{
					{Label: "ID", Value: item.ID},
					{Label: versionValue, Value: strconv.FormatInt(item.Version, 10)},
					{Label: bridgeExtensionValue, Value: item.ExtensionName},
					{Label: bundleBundleValue, Value: item.BundleName},
					{Label: bundleProfileValue, Value: item.ProfileName},
					{Label: automationScopeValue, Value: item.Scope},
					{Label: authoredContextWorkspaceValue, Value: stringOrDash(item.WorkspaceID)},
					{
						Label: "Network Requirement Digest",
						Value: stringOrDash(item.NetworkRequirementDigest),
					},
					{
						Label: "Network Requirement Confirmed By",
						Value: stringOrDash(item.NetworkRequirementConfirmedBy),
					},
					{Label: bundleCreatedValue, Value: formatTime(item.CreatedAt)},
					{Label: bundleUpdatedValue, Value: formatTime(item.UpdatedAt)},
				}),
				bundleChannelsTable(item.Channels),
				bundleLayoutsTable(item.Layouts),
				bundleAgentsTable(item.Agents),
				bundleJobsTable(item.Jobs),
				bundleTriggersTable(item.Triggers),
				bundleBridgesTable(item.Bridges),
				bundleInventoryTable(item.Inventory),
			), nil
		},
		toon: func() (string, error) {
			return renderHumanBlocks(
				renderToonObject(
					"bundle_activation",
					[]string{
						"id",
						versionKey,
						bundleExtensionKey,
						bundleBundleKey,
						bundleProfileKey,
						automationScopeKey,
						workspaceSkillSource,
					},
					[]string{
						item.ID,
						strconv.FormatInt(item.Version, 10),
						item.ExtensionName,
						item.BundleName,
						item.ProfileName,
						item.Scope,
						item.WorkspaceID,
					},
				),
				bundleLayoutsToon(item.Layouts),
				renderToonArray(
					"agents",
					[]string{
						automationNameKey,
						cliProviderKey,
						bundleModelKey,
						authoredContextSoulKey,
						bundleHeartbeatKey,
					},
					bundleAgentRows(item.Agents),
				),
				renderToonArray(
					"inventory",
					[]string{bundleKindKey, "id", automationNameKey},
					bundleInventoryRows(item.Inventory),
				),
			), nil
		},
	}
}

func bundleLayoutsToon(items []BundleLayoutRecord) string {
	return renderToonArray(
		"layouts",
		[]string{
			automationNameKey,
			"aspect",
			"slot_count",
			"overflow",
			"participant_slots",
		},
		bundleLayoutRows(items),
	)
}

func bundleNetworkSettingsBundle(item BundleNetworkSettingsRecord) outputBundle {
	return outputBundle{
		jsonValue: item,
		human: func() (string, error) {
			return renderHumanBlocks(
				renderHumanSection("Bundle Network Settings", []keyValue{
					{Label: "Declared Channels", Value: strconv.Itoa(len(item.DeclaredChannels))},
				}),
				renderHumanTable(
					"Declared Channels",
					[]string{
						"Activation",
						bridgeExtensionValue,
						bundleBundleValue,
						bundleProfileValue,
						authoredContextWorkspaceValue,
						bundleNameValue,
						"Primary",
					},
					bundleDeclaredChannelRows(item.DeclaredChannels),
				),
			), nil
		},
		toon: func() (string, error) {
			return renderToonArray(
				"declared_channels",
				[]string{
					"activation",
					bundleExtensionKey,
					bundleBundleKey,
					bundleProfileKey,
					workspaceSkillSource,
					automationNameKey,
					"primary",
				},
				bundleDeclaredChannelRows(item.DeclaredChannels),
			), nil
		},
	}
}
