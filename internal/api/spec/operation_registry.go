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
		registryExtensionOperations(),
		registryHookOperations(),
		registryAgentRuntimeOperations(),
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
