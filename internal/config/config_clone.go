package config

// CloneConfig returns an ownership-safe copy of the complete runtime configuration.
func CloneConfig(source *Config) Config {
	if source == nil {
		return Config{}
	}

	cloned := *source
	cloned.Marketplace.PluginSources = cloneMarketplacePluginSources(source.Marketplace.PluginSources)
	cloned.WindowManager = cloneWindowManagerConfig(source.WindowManager)
	cloned.Terminal = source.Terminal
	cloned.CmdPalette = CloneCmdPaletteConfig(source.CmdPalette)
	cloned.MCPServers = cloneMCPServers(source.MCPServers)
	cloned.Providers = cloneProviders(source.Providers)
	cloned.ModelCatalog.Sources.ModelsDev.Enabled = cloneBoolPtr(
		source.ModelCatalog.Sources.ModelsDev.Enabled,
	)
	cloned.Roles = CloneRolesConfig(&source.Roles)
	cloned.RoleSources = CloneRoleFieldSources(source.RoleSources)
	cloned.Skills.Sources = cloneStrings(source.Skills.Sources)
	cloned.Skills.CustomSources = cloneStrings(source.Skills.CustomSources)
	cloned.Skills.DisabledSkills = cloneStrings(source.Skills.DisabledSkills)
	cloned.Skills.AllowedMarketplaceHooks = cloneStrings(source.Skills.AllowedMarketplaceHooks)
	cloned.Extensions.Resources.AllowedKinds = append(
		cloned.Extensions.Resources.AllowedKinds[:0:0],
		source.Extensions.Resources.AllowedKinds...,
	)
	cloned.Tools.Policy.TrustedSources = cloneStrings(source.Tools.Policy.TrustedSources)
	cloned.Session.Attachments.AllowedMIME = cloneStrings(source.Session.Attachments.AllowedMIME)
	cloned.Automation = cloneAutomationConfig(source.Automation)
	cloned.Gateway.Connections = append(
		[]GatewayConnectionConfig(nil),
		source.Gateway.Connections...,
	)
	cloned.Loops = cloneLoopsConfig(&source.Loops)
	cloned.Hooks.Declarations = cloneHookDecls(source.Hooks.Declarations)
	return cloned
}
