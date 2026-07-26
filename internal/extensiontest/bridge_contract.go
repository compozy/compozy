package extensiontest

import (
	"encoding/json"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	bridgecontract "github.com/compozy/agh/internal/bridges/contract"
)

func bridgeInstanceDomain(instance bridgecontract.BridgeInstance) bridgepkg.BridgeInstance {
	converted := bridgepkg.BridgeInstance{
		ID:            instance.ID,
		Scope:         bridgepkg.Scope(instance.Scope),
		WorkspaceID:   instance.WorkspaceID,
		Platform:      instance.Platform,
		ExtensionName: instance.ExtensionName,
		DisplayName:   instance.DisplayName,
		Source:        bridgepkg.BridgeInstanceSource(instance.Source),
		Enabled:       instance.Enabled,
		Status:        bridgepkg.BridgeStatus(instance.Status),
		DMPolicy:      bridgepkg.BridgeDMPolicy(instance.DMPolicy),
		RoutingPolicy: bridgepkg.RoutingPolicy{
			IncludePeer:   instance.RoutingPolicy.IncludePeer,
			IncludeThread: instance.RoutingPolicy.IncludeThread,
			IncludeGroup:  instance.RoutingPolicy.IncludeGroup,
		},
		NotificationSuppress: instance.NotificationSuppress,
		CreatedAt:            instance.CreatedAt,
		UpdatedAt:            instance.UpdatedAt,
	}
	converted.ProviderConfig = append(json.RawMessage(nil), instance.ProviderConfig...)
	converted.DeliveryDefaults = append(json.RawMessage(nil), instance.DeliveryDefaults...)
	converted.Degradation = bridgeDegradationDomain(instance.Degradation)
	return converted
}

func bridgeStatusDomain(status bridgecontract.BridgeStatus) bridgepkg.BridgeStatus {
	return bridgepkg.BridgeStatus(status)
}

func bridgeDegradationDomain(degradation *bridgecontract.BridgeDegradation) *bridgepkg.BridgeDegradation {
	if degradation == nil {
		return nil
	}
	return &bridgepkg.BridgeDegradation{
		Reason:  bridgepkg.BridgeDegradationReason(degradation.Reason),
		Message: degradation.Message,
	}
}
