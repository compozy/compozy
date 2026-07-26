package bridges

import (
	"slices"

	bridgecontract "github.com/compozy/agh/internal/bridges/contract"
)

// BridgeInstanceToContract maps one domain snapshot into its provider-facing wire contract.
func BridgeInstanceToContract(instance BridgeInstance) bridgecontract.BridgeInstance {
	converted := bridgecontract.BridgeInstance{
		ID:            instance.ID,
		Scope:         bridgecontract.Scope(instance.Scope),
		WorkspaceID:   instance.WorkspaceID,
		Platform:      instance.Platform,
		ExtensionName: instance.ExtensionName,
		DisplayName:   instance.DisplayName,
		Source:        bridgecontract.BridgeInstanceSource(instance.Source),
		Enabled:       instance.Enabled,
		Status:        bridgecontract.BridgeStatus(instance.Status),
		DMPolicy:      bridgecontract.BridgeDMPolicy(instance.DMPolicy),
		RoutingPolicy: bridgecontract.RoutingPolicy{
			IncludePeer:   instance.RoutingPolicy.IncludePeer,
			IncludeThread: instance.RoutingPolicy.IncludeThread,
			IncludeGroup:  instance.RoutingPolicy.IncludeGroup,
		},
		NotificationSuppress: instance.NotificationSuppress,
		CreatedAt:            instance.CreatedAt,
		UpdatedAt:            instance.UpdatedAt,
	}
	converted.ProviderConfig = slices.Clone(instance.ProviderConfig)
	converted.DeliveryDefaults = slices.Clone(instance.DeliveryDefaults)
	if instance.Degradation != nil {
		converted.Degradation = &bridgecontract.BridgeDegradation{
			Reason:  bridgecontract.BridgeDegradationReason(instance.Degradation.Reason),
			Message: instance.Degradation.Message,
		}
	}
	return converted
}

func bridgeInstanceFromContract(instance bridgecontract.BridgeInstance) BridgeInstance {
	converted := BridgeInstance{
		ID:            instance.ID,
		Scope:         Scope(instance.Scope),
		WorkspaceID:   instance.WorkspaceID,
		Platform:      instance.Platform,
		ExtensionName: instance.ExtensionName,
		DisplayName:   instance.DisplayName,
		Source:        BridgeInstanceSource(instance.Source),
		Enabled:       instance.Enabled,
		Status:        BridgeStatus(instance.Status),
		DMPolicy:      BridgeDMPolicy(instance.DMPolicy),
		RoutingPolicy: RoutingPolicy{
			IncludePeer:   instance.RoutingPolicy.IncludePeer,
			IncludeThread: instance.RoutingPolicy.IncludeThread,
			IncludeGroup:  instance.RoutingPolicy.IncludeGroup,
		},
		NotificationSuppress: instance.NotificationSuppress,
		CreatedAt:            instance.CreatedAt,
		UpdatedAt:            instance.UpdatedAt,
	}
	converted.ProviderConfig = slices.Clone(instance.ProviderConfig)
	converted.DeliveryDefaults = slices.Clone(instance.DeliveryDefaults)
	if instance.Degradation != nil {
		converted.Degradation = &BridgeDegradation{
			Reason: BridgeDegradationReason(instance.Degradation.Reason), Message: instance.Degradation.Message,
		}
	}
	return converted
}

func bridgeDegradationToContract(degradation BridgeDegradation) bridgecontract.BridgeDegradation {
	return bridgecontract.BridgeDegradation{
		Reason:  bridgecontract.BridgeDegradationReason(degradation.Reason),
		Message: degradation.Message,
	}
}

func bridgeDegradationFromContract(degradation bridgecontract.BridgeDegradation) BridgeDegradation {
	return BridgeDegradation{
		Reason:  BridgeDegradationReason(degradation.Reason),
		Message: degradation.Message,
	}
}
