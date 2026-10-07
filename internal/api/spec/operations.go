package spec

import (
	"cmp"
	"slices"
)

// Operations returns the complete transport-neutral operation registry.
func Operations() []OperationSpec {
	ops := cloneOperationSpecs(operationRegistry)
	ops = append(ops, runtimeStatusOperations()...)
	ops = append(ops, sessionAdmissionOperations()...)
	ops = append(ops, agentDefinitionMutationOperations()...)
	ops = append(ops, agentCatalogOperations()...)
	ops = append(ops, sessionTranscriptOperations()...)
	ops = append(ops, sessionNavigationOperations()...)
	ops = append(ops, attentionNotificationOperations()...)
	ops = append(ops, authoredContextOperations()...)
	ops = append(ops, append(loopsOperations(), goalOperations()...)...)
	ops = applyLoopAutomationContract(ops)
	ops = append(ops, modelCatalogOperations()...)
	ops = append(ops, marketplaceOperations()...)
	ops = append(ops, settingsMCPAuthOperations()...)
	ops = append(ops, providerOperations()...)
	ops = append(ops, windowManagerOperations()...)
	ops = append(ops, terminalOperations()...)
	ops = append(ops, cmdPaletteOperations()...)
	ops = applyToolArtifactContract(ops)
	ops = applyAgentIdentityContract(ops)
	slices.SortStableFunc(ops, func(a, b OperationSpec) int {
		return cmp.Or(cmp.Compare(a.Path, b.Path), cmp.Compare(a.Method, b.Method))
	})

	return ops
}
