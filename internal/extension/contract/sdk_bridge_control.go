package contract

import (
	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/subprocess"
)

var sdkBridgeControlTypes = []NamedType{
	{Name: "BridgeRuntimePurpose", Value: subprocess.BridgeRuntimePurpose("")},
	{Name: "ControlMethod", Value: bridgepkg.ControlMethod("")},
	{Name: "BridgeCheckStatus", Value: bridgepkg.BridgeCheckStatus("")},
	{Name: "BridgeCheckRecord", Value: bridgepkg.BridgeCheckRecord{}},
	{Name: "BridgeCheckRequest", Value: bridgepkg.BridgeCheckRequest{}},
	{Name: "BridgeCheckResponse", Value: bridgepkg.BridgeCheckResponse{}},
	{Name: "BridgeWebhookRegistrationRequest", Value: bridgepkg.BridgeWebhookRegistrationRequest{}},
	{Name: "BridgeWebhookRegistrationResponse", Value: bridgepkg.BridgeWebhookRegistrationResponse{}},
}

// SDKRootTypes returns a defensive copy of every canonical generated SDK contract root.
func SDKRootTypes() []NamedType {
	types := make([]NamedType, 0, len(sdkRootTypes)+len(sdkBridgeControlTypes))
	types = append(types, sdkRootTypes...)
	return append(types, sdkBridgeControlTypes...)
}
