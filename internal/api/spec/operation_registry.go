package spec

var operationRegistry = buildOperationRegistry()

func buildOperationRegistry() []OperationSpec {
	groups := [][]OperationSpec{
		registryResourceOperations(),
		registryVaultOperations(),
		registryToolOperations(),
		registryToolsetOperations(),
		registryAgentOperations(),
		registryRolesOperations(),
		registryAutomationOperations(),
		registryOnboardingOperations(),
		registryFilesystemOperations(),
		registryNetworkOperations(),
		registryExtensionOperations(),
		registryBundleOperations(),
		registryHookOperations(),
		registryAgentRuntimeOperations(),
		registryMemoryOperations(),
		registryMemoryLifecycleOperations(),
		registryLogOperations(),
		registrySupportOperations(),
		registrySessionOperations(),
		registryTaskManagementOperations(),
		registryTaskLifecycleOperations(),
		registryTaskRunOperations(),
		registryTaskStateOperations(),
		registrySkillOperations(),
		registrySettingsOperations(),
		registrySettingsFeatureOperations(),
		registryWorkspaceOperations(),
	}
	operations := make([]OperationSpec, 0)
	for _, group := range groups {
		operations = append(operations, group...)
	}
	return append(operations, supplementalOperationSpecs()...)
}
