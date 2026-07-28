package cli

import (
	"strconv"
	"strings"
)

func bundleLayoutRows(items []BundleLayoutRecord) [][]string {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			stringOrDash(item.DisplayName),
			stringOrDash(item.AspectVariant),
			strconv.Itoa(len(item.ParticipantSlots)),
			stringOrDash(item.OverflowPolicy),
			strings.Join(item.ParticipantSlots, "|"),
		})
	}
	return rows
}

func bundleAgentRows(items []BundleAgentRecord) [][]string {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			stringOrDash(item.Name),
			stringOrDash(item.Provider),
			stringOrDash(item.Model),
			formatBool(item.HasSoul),
			formatBool(item.HasHeartbeat),
		})
	}
	return rows
}

func bundleInventoryRows(items []BundleInventoryRecord) [][]string {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			stringOrDash(item.ResourceKind),
			stringOrDash(item.ResourceID),
			stringOrDash(item.ResourceName),
		})
	}
	return rows
}

func bundleDeclaredChannelRows(items []DeclaredNetworkChannelRecord) [][]string {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			stringOrDash(item.ActivationID),
			stringOrDash(item.ExtensionName),
			stringOrDash(item.BundleName),
			stringOrDash(item.ProfileName),
			stringOrDash(item.WorkspaceID),
			stringOrDash(item.Name),
			formatBool(item.Primary),
		})
	}
	return rows
}
