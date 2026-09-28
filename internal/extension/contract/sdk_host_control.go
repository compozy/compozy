package contract

import "github.com/compozy/compozy/internal/hooks"

var sdkHostControlTypes = []NamedType{
	{Name: "HostAPIMethod", Value: HostAPIMethod("")},
	{Name: "HookEvent", Value: hooks.HookEvent("")},
	{Name: "DescribePayload", Value: DescribePayload{}},
	{Name: "DescribeResources", Value: DescribeResources{}},
	{Name: "DescribeSubprocess", Value: DescribeSubprocess{}},
	{Name: "DescribeGatewayRequirement", Value: DescribeGatewayRequirement{}},
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
	{Name: "CmdPaletteConfig", Value: CmdPaletteConfig{}},
}

// SDKRootTypes returns a defensive copy of every canonical generated SDK contract root.
func SDKRootTypes() []NamedType {
	types := make(
		[]NamedType,
		0,
		len(sdkRootTypes)+len(sdkHostControlTypes),
	)
	types = append(types, sdkRootTypes...)
	return append(types, sdkHostControlTypes...)
}
