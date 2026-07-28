package spec

import "sort"

// Operations returns the complete transport-neutral operation registry.
func Operations() []OperationSpec {
	ops := cloneOperationSpecs(operationRegistry)
	ops = append(ops, runtimeStatusOperations()...)
	ops = append(ops, sessionAdmissionOperations()...)
	ops = append(ops, agentDefinitionMutationOperations()...)
	ops = append(ops, agentCatalogOperations()...)
	ops = append(ops, bridgeOperations()...)
	ops = append(ops, sessionTranscriptOperations()...)
	ops = append(ops, notificationPresetOperations()...)
	ops = append(ops, authoredContextOperations()...)
	ops = append(ops, append(loopsOperations(), goalOperations()...)...)
	ops = applyLoopAutomationContract(ops)
	ops = append(ops, modelCatalogOperations()...)
	ops = append(ops, marketplaceOperations()...)
	ops = append(ops, settingsMCPInstallOperation())
	ops = append(ops, settingsMCPAuthOperations()...)
	ops = append(ops, providerOperations()...)
	ops = append(ops, networkCoordinationOperations()...)
	ops = append(ops, windowManagerOperations()...)
	ops = applyToolArtifactContract(ops)
	sort.SliceStable(ops, func(i, j int) bool {
		if ops[i].Path == ops[j].Path {
			return ops[i].Method < ops[j].Method
		}
		return ops[i].Path < ops[j].Path
	})

	return ops
}
