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

var sdkHostControlTypes = []NamedType{
	{Name: "HostAPIMethod", Value: HostAPIMethod("")},
	{Name: "HookEvent", Value: hooks.HookEvent("")},
	{Name: "DescribePayload", Value: DescribePayload{}},
	{Name: "DescribeResources", Value: DescribeResources{}},
	{Name: "DescribeSubprocess", Value: DescribeSubprocess{}},
	{Name: "DescribeNetworkParticipation", Value: DescribeNetworkParticipation{}},
	{Name: "DescribeSDKInfo", Value: DescribeSDKInfo{}},
	{Name: "ExtensionCommandSpec", Value: ExtensionCommandSpec{}},
	{Name: "ExtensionCommandGroupSpec", Value: ExtensionCommandGroupSpec{}},
	{Name: "CommandFlagType", Value: CommandFlagType("")},
	{Name: "CommandFlag", Value: CommandFlag{}},
	{Name: "IssueSeverity", Value: IssueSeverity("")},
	{Name: "ValidationIssue", Value: ValidationIssue{}},
	{Name: "ConsentArea", Value: ConsentArea{}},
	{Name: "ExtensionManifestSummary", Value: ExtensionManifestSummary{}},
	{Name: "ExtensionValidatePayload", Value: ExtensionValidatePayload{}},
}

// SDKRootTypes returns a defensive copy of every canonical generated SDK contract root.
func SDKRootTypes() []NamedType {
	types := make(
		[]NamedType,
		0,
		len(sdkRootTypes)+len(sdkBridgeControlTypes)+len(sdkHostControlTypes),
	)
	types = append(types, sdkRootTypes...)
	types = append(types, sdkBridgeControlTypes...)
	return append(types, sdkHostControlTypes...)
}
