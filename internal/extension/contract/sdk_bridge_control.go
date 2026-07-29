package contract

import (
	bridgepkg "github.com/compozy/compozy/internal/bridges/contract"
	"github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/subprocess"
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
	types := make([]NamedType, 0, len(sdkRootTypes)+len(sdkBridgeControlTypes)+6)
	types = append(types, sdkRootTypes...)
	types = append(types, sdkBridgeControlTypes...)
	return append(types,
		NamedType{Name: "HostAPIMethod", Value: HostAPIMethod("")},
		NamedType{Name: "HookEvent", Value: hooks.HookEvent("")},
		NamedType{Name: "DescribePayload", Value: DescribePayload{}},
		NamedType{Name: "DescribeSubprocess", Value: DescribeSubprocess{}},
		NamedType{Name: "DescribeSDKInfo", Value: DescribeSDKInfo{}},
		NamedType{Name: "ExtensionCommandGroupSpec", Value: ExtensionCommandGroupSpec{}},
	)
}
