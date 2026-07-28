package cli

import (
	"fmt"
	"strings"
	"time"
)

func bridgeBundle(item BridgeRecord) outputBundle {
	return outputBundle{
		jsonValue: item,
		human: func() (string, error) {
			return renderHumanSection(bridgeBridgeValue, []keyValue{
				{Label: "ID", Value: stringOrDash(item.ID)},
				{Label: automationNameValue, Value: stringOrDash(item.DisplayName)},
				{Label: bundlePlatformValue, Value: stringOrDash(item.Platform)},
				{Label: bridgeExtensionValue, Value: stringOrDash(item.ExtensionName)},
				{Label: automationScopeValue, Value: stringOrDash(string(item.Scope))},
				{Label: automationWorkspaceValue, Value: stringOrDash(item.WorkspaceID)},
				{Label: bridgeEnabledValue, Value: fmt.Sprintf("%t", item.Enabled)},
				{Label: automationStatusValue, Value: stringOrDash(string(item.Status))},
				{
					Label: "Routing",
					Value: stringOrDash(bridgeRoutingPolicyLabel(item.RoutingPolicy)),
				},
				{
					Label: "Notification Suppress",
					Value: fmt.Sprintf("%t", item.NotificationSuppress),
				},
				{
					Label: "Delivery Defaults",
					Value: stringOrDash(compactJSON(item.DeliveryDefaults)),
				},
				{Label: bridgeCreatedValue, Value: stringOrDash(formatTime(item.CreatedAt))},
				{
					Label: authoredContextUpdatedValue,
					Value: stringOrDash(formatTime(item.UpdatedAt)),
				},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(bridgeBridgeKey, []string{
				"id",
				bridgeDisplayNameKey,
				bridgePlatformKey,
				bridgeSetupExtensionNameKey,
				bridgeScopeKey,
				bridgeWorkspaceIDKey,
				bridgeEnabledKey,
				bridgeStatusKey,
				"routing",
				"include_peer",
				"include_thread",
				"include_group",
				"notification_suppress",
				"delivery_defaults",
				bridgeCreatedAtKey,
				bridgeUpdatedAtKey,
			}, []string{
				item.ID,
				item.DisplayName,
				item.Platform,
				item.ExtensionName,
				string(item.Scope),
				item.WorkspaceID,
				fmt.Sprintf("%t", item.Enabled),
				string(item.Status),
				bridgeRoutingPolicyLabel(item.RoutingPolicy),
				fmt.Sprintf("%t", item.RoutingPolicy.IncludePeer),
				fmt.Sprintf("%t", item.RoutingPolicy.IncludeThread),
				fmt.Sprintf("%t", item.RoutingPolicy.IncludeGroup),
				fmt.Sprintf("%t", item.NotificationSuppress),
				compactJSON(item.DeliveryDefaults),
				formatTime(item.CreatedAt),
				formatTime(item.UpdatedAt),
			}), nil
		},
	}
}

func bridgeRoutesBundle(routes []BridgeRouteRecord, now func() time.Time) outputBundle {
	return listBundle(
		routes,
		routes,
		"Bridge Routes",
		[]string{
			cliHashValue,
			automationScopeValue,
			configWorkspaceValue,
			taskPeerValue,
			taskThreadValue,
			taskGroupValue,
			"Session",
			bridgeAgentValue,
			"Last Active",
		},
		"bridge_routes",
		[]string{
			"routing_key_hash",
			bridgeScopeKey,
			bridgeWorkspaceIDKey,
			bridgePeerIDKey,
			bridgeThreadIDKey,
			bridgeGroupIDKey,
			bridgeSessionIDKey,
			bridgeAgentNameKey,
			bridgeLastActivityAtKey,
		},
		func(route BridgeRouteRecord) []string {
			return []string{
				stringOrDash(route.RoutingKeyHash),
				stringOrDash(string(route.Scope)),
				stringOrDash(route.WorkspaceID),
				stringOrDash(route.PeerID),
				stringOrDash(route.ThreadID),
				stringOrDash(route.GroupID),
				stringOrDash(route.SessionID),
				stringOrDash(route.AgentName),
				stringOrDash(formatAge(now, route.LastActivityAt)),
			}
		},
		func(route BridgeRouteRecord) []string {
			return []string{
				route.RoutingKeyHash,
				string(route.Scope),
				route.WorkspaceID,
				route.PeerID,
				route.ThreadID,
				route.GroupID,
				route.SessionID,
				route.AgentName,
				formatTime(route.LastActivityAt),
			}
		},
	)
}

func bridgeTargetsBundle(result BridgeTargetsRecord, now func() time.Time) outputBundle {
	return listBundle(
		result,
		result.Targets,
		"Bridge Targets",
		[]string{"ROUTE", automationNameValue, "TYPE", "QUALIFIER", "CAPABILITIES", "LAST SEEN"},
		"bridge_targets",
		[]string{
			"canonical_route",
			bridgeDisplayNameKey,
			"target_type",
			"qualifier",
			"capabilities",
			"last_seen_at",
		},
		func(target BridgeTargetRecord) []string {
			return []string{
				stringOrDash(target.CanonicalRoute),
				stringOrDash(target.DisplayName),
				stringOrDash(string(target.TargetType)),
				stringOrDash(target.Qualifier),
				stringOrDash(strings.Join(target.Capabilities, ",")),
				stringOrDash(formatAge(now, target.LastSeenAt)),
			}
		},
		func(target BridgeTargetRecord) []string {
			return []string{
				target.CanonicalRoute,
				target.DisplayName,
				string(target.TargetType),
				target.Qualifier,
				strings.Join(target.Capabilities, ","),
				formatTime(target.LastSeenAt),
			}
		},
	)
}

func bridgeResolveTargetBundle(result BridgeResolveTargetRecord) outputBundle {
	return outputBundle{
		jsonValue: result,
		human: func() (string, error) {
			if result.Result.Match == nil {
				return renderHumanSection("Bridge Target", []keyValue{
					{Label: automationStatusValue, Value: bridgeUnresolvedValue},
					{Label: bridgeStepValue, Value: fmt.Sprintf("%d", result.Result.Step)},
					{Label: "Ambiguous", Value: fmt.Sprintf("%t", result.Result.Ambiguous)},
					{Label: "Candidates", Value: fmt.Sprintf("%d", len(result.Result.Candidates))},
				}), nil
			}
			target := result.Result.Match
			return renderHumanSection("Bridge Target", []keyValue{
				{Label: automationStatusValue, Value: bridgeResolvedValue},
				{Label: bridgeStepValue, Value: fmt.Sprintf("%d", result.Result.Step)},
				{Label: "Route", Value: stringOrDash(target.CanonicalRoute)},
				{Label: automationNameValue, Value: stringOrDash(target.DisplayName)},
				{Label: "Type", Value: stringOrDash(string(target.TargetType))},
				{Label: "Qualifier", Value: stringOrDash(target.Qualifier)},
			}), nil
		},
		toon: func() (string, error) {
			status := bridgeUnresolvedValue
			route := ""
			name := ""
			if result.Result.Match != nil {
				status = bridgeResolvedValue
				route = result.Result.Match.CanonicalRoute
				name = result.Result.Match.DisplayName
			}
			return renderToonObject("bridge_target", []string{
				bridgeStatusKey,
				"step",
				"ambiguous",
				"canonical_route",
				bridgeDisplayNameKey,
			}, []string{
				status,
				fmt.Sprintf("%d", result.Result.Step),
				fmt.Sprintf("%t", result.Result.Ambiguous),
				route,
				name,
			}), nil
		},
	}
}

func bridgeSecretBindingListBundle(items []BridgeSecretBindingRecord) outputBundle {
	return listBundle(
		struct {
			Bindings []BridgeSecretBindingRecord `json:"bindings"`
		}{Bindings: items},
		items,
		"Bridge Secret Bindings",
		[]string{"BRIDGE", "NAME", "SECRET REF", "KIND", "UPDATED"},
		"bridge_secret_bindings",
		[]string{
			taskBridgeInstanceIDKey,
			bridgeBindingNameKey,
			"secret_ref",
			networkKindKey,
			bridgeUpdatedAtKey,
		},
		func(item BridgeSecretBindingRecord) []string {
			return []string{
				stringOrDash(item.BridgeInstanceID),
				stringOrDash(item.BindingName),
				stringOrDash(item.SecretRef),
				stringOrDash(item.Kind),
				stringOrDash(formatTime(item.UpdatedAt)),
			}
		},
		func(item BridgeSecretBindingRecord) []string {
			return []string{
				item.BridgeInstanceID,
				item.BindingName,
				item.SecretRef,
				item.Kind,
				formatTime(item.UpdatedAt),
			}
		},
	)
}

func bridgeSecretBindingBundle(item BridgeSecretBindingRecord) outputBundle {
	return outputBundle{
		jsonValue: struct {
			Binding BridgeSecretBindingRecord `json:"binding"`
		}{Binding: item},
		human: func() (string, error) {
			return renderHumanSection("Bridge Secret Binding", []keyValue{
				{Label: bridgeBridgeValue, Value: stringOrDash(item.BridgeInstanceID)},
				{Label: automationNameValue, Value: stringOrDash(item.BindingName)},
				{Label: "Secret Ref", Value: stringOrDash(item.SecretRef)},
				{Label: bridgeKindValue, Value: stringOrDash(item.Kind)},
				{Label: bridgeCreatedValue, Value: stringOrDash(formatTime(item.CreatedAt))},
				{
					Label: authoredContextUpdatedValue,
					Value: stringOrDash(formatTime(item.UpdatedAt)),
				},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"bridge_secret_binding",
				[]string{
					taskBridgeInstanceIDKey,
					bridgeBindingNameKey,
					"secret_ref",
					bundleKindKey,
					bridgeCreatedAtKey,
					bridgeUpdatedAtKey,
				},
				[]string{
					item.BridgeInstanceID,
					item.BindingName,
					item.SecretRef,
					item.Kind,
					formatTime(item.CreatedAt),
					formatTime(item.UpdatedAt),
				},
			), nil
		},
	}
}

func bridgeSecretBindingDeleteBundle(id string, bindingName string) outputBundle {
	item := struct {
		BridgeInstanceID string `json:"bridge_instance_id"`
		BindingName      string `json:"binding_name"`
		Status           string `json:"status"`
	}{
		BridgeInstanceID: strings.TrimSpace(id),
		BindingName:      strings.TrimSpace(bindingName),
		Status:           bridgeDeletedKey,
	}
	return outputBundle{
		jsonValue: item,
		human: func() (string, error) {
			return renderHumanSection("Bridge Secret Binding", []keyValue{
				{Label: bridgeBridgeValue, Value: stringOrDash(item.BridgeInstanceID)},
				{Label: automationNameValue, Value: stringOrDash(item.BindingName)},
				{Label: automationStatusValue, Value: item.Status},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"bridge_secret_binding",
				[]string{taskBridgeInstanceIDKey, bridgeBindingNameKey, bridgeStatusKey},
				[]string{item.BridgeInstanceID, item.BindingName, item.Status},
			), nil
		},
	}
}

func bridgeTestDeliveryBundle(item BridgeTestDeliveryRecord) outputBundle {
	return outputBundle{
		jsonValue: item,
		human: func() (string, error) {
			return renderHumanBlocks(
				renderHumanSection("Test Delivery", []keyValue{
					{Label: automationStatusValue, Value: stringOrDash(item.Status)},
					{Label: bridgeMessageValue, Value: stringOrDash(item.Message)},
				}),
				renderHumanSection("Delivery Target", []keyValue{
					{
						Label: bridgeBridgeValue,
						Value: stringOrDash(item.DeliveryTarget.BridgeInstanceID),
					},
					{Label: taskPeerValue, Value: stringOrDash(item.DeliveryTarget.PeerID)},
					{Label: taskThreadValue, Value: stringOrDash(item.DeliveryTarget.ThreadID)},
					{Label: taskGroupValue, Value: stringOrDash(item.DeliveryTarget.GroupID)},
					{Label: bridgeModeValue, Value: stringOrDash(string(item.DeliveryTarget.Mode))},
				}),
			), nil
		},
		toon: func() (string, error) {
			return renderToonObject("test_delivery", []string{
				bridgeStatusKey,
				bridgeMessageKey,
				taskBridgeInstanceIDKey,
				bridgePeerIDKey,
				bridgeThreadIDKey,
				bridgeGroupIDKey,
				bridgeModeKey,
			}, []string{
				item.Status,
				item.Message,
				item.DeliveryTarget.BridgeInstanceID,
				item.DeliveryTarget.PeerID,
				item.DeliveryTarget.ThreadID,
				item.DeliveryTarget.GroupID,
				string(item.DeliveryTarget.Mode),
			}), nil
		},
	}
}
