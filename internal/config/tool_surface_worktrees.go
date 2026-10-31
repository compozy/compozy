package config

func worktreeToolSurfaceMutableConfigKinds() map[string]ValueKind {
	return map[string]ValueKind{
		worktreeTeardownCommandPath: ConfigValueString,
		worktreeTeardownTimeoutPath: ConfigValueDuration,
		worktreeRootConfigPath:      ConfigValueString,
		worktreeNamespaceConfigPath: ConfigValueString,
		worktreeCopyListConfigPath:  ConfigValueStringSlice,
		worktreeSetupCommandPath:    ConfigValueString,
		worktreeSetupTimeoutPath:    ConfigValueDuration,
		worktreeCheckoutTimeoutPath: ConfigValueDuration,
		worktreeDiscoveryTTLPath:    ConfigValueDuration,
	}
}
