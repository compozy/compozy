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
		registryGatewayOperations(),
		registryNetworkOperations(),
		registryExtensionOperations(),
		registryHookOperations(),
		registryAgentRuntimeOperations(),
		registryMemoryOperations(),
		registryMemoryLifecycleOperations(),
		registryLogOperations(),
		registrySupportOperations(),
		registrySessionOperations(),
		registryCallOperations(),
		registryTaskManagementOperations(),
		registryTaskLifecycleOperations(),
		registryTaskRunOperations(),
		registryTaskStateOperations(),
		registrySkillOperations(),
		registrySettingsOperations(),
		registrySettingsFeatureOperations(),
		registryProfileOperations(),
		registryWorkspaceOperations(),
		registryWorktreeOperations(),
	}
	operations := make([]OperationSpec, 0)
	for _, group := range groups {
		operations = append(operations, group...)
	}
	return append(operations, supplementalOperationSpecs()...)
}
