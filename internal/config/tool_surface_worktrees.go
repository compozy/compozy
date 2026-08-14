package config

func worktreeToolSurfaceMutableConfigKinds() map[string]ValueKind {
	return map[string]ValueKind{
		worktreeRootConfigPath:      ConfigValueString,
		worktreeNamespaceConfigPath: ConfigValueString,
		worktreeCopyListConfigPath:  ConfigValueStringSlice,
		worktreeSetupCommandPath:    ConfigValueString,
		worktreeSetupTimeoutPath:    ConfigValueDuration,
		worktreeDiscoveryTTLPath:    ConfigValueDuration,
	}
}
