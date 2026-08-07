package config

// CloneConfig returns an ownership-safe copy of the complete runtime configuration.
func CloneConfig(source *Config) Config {
	if source == nil {
		return Config{}
	}

	cloned := *source
	cloned.WindowManager = cloneWindowManagerConfig(source.WindowManager)
	cloned.MCPServers = cloneMCPServers(source.MCPServers)
	cloned.Providers = cloneProviders(source.Providers)
	cloned.ModelCatalog.Sources.ModelsDev.Enabled = cloneBoolPtr(
		source.ModelCatalog.Sources.ModelsDev.Enabled,
	)
	cloned.Sandboxes = cloneConfigSandboxProfiles(source.Sandboxes)
	cloned.Memory = CloneMemoryConfig(&source.Memory)
	cloned.Roles = CloneRolesConfig(&source.Roles)
	cloned.RoleSources = CloneRoleFieldSources(source.RoleSources)
	cloned.Skills.DisabledSkills = cloneStrings(source.Skills.DisabledSkills)
	cloned.Skills.AllowedMarketplaceMCP = cloneStrings(source.Skills.AllowedMarketplaceMCP)
	cloned.Skills.AllowedMarketplaceHooks = cloneStrings(source.Skills.AllowedMarketplaceHooks)
	cloned.Extensions.Resources.AllowedKinds = append(
		cloned.Extensions.Resources.AllowedKinds[:0:0],
		source.Extensions.Resources.AllowedKinds...,
	)
	cloned.Tools.Policy.TrustedSources = cloneStrings(source.Tools.Policy.TrustedSources)
	cloned.Automation = cloneAutomationConfig(source.Automation)
	cloned.Gateway.Connections = append(
		[]GatewayConnectionConfig(nil),
		source.Gateway.Connections...,
	)
	cloned.Loops = cloneLoopsConfig(&source.Loops)
	cloned.Hooks.Declarations = cloneHookDecls(source.Hooks.Declarations)
	return cloned
}

func cloneConfigSandboxProfiles(source map[string]SandboxProfile) map[string]SandboxProfile {
	if len(source) == 0 {
		return map[string]SandboxProfile{}
	}
	cloned := make(map[string]SandboxProfile, len(source))
	for name, profile := range source {
		clonedProfile := profile
		clonedProfile.Env = mergeStringMaps(nil, profile.Env)
		clonedProfile.SecretEnv = mergeStringMaps(nil, profile.SecretEnv)
		clonedProfile.Network.AllowList = cloneStrings(profile.Network.AllowList)
		clonedProfile.Network.DenyList = cloneStrings(profile.Network.DenyList)
		cloned[name] = clonedProfile
	}
	return cloned
}
