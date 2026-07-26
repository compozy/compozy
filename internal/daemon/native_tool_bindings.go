package daemon

import toolspkg "github.com/compozy/agh/internal/tools"

func (n *daemonNativeTools) bindings() map[toolspkg.ToolID]nativeToolBinding {
	availability := n.nativeToolAvailability()
	bindings := make(map[toolspkg.ToolID]nativeToolBinding, 32)
	addNativeToolBindings(bindings, n.registryToolBindings(availability.registry))
	addNativeToolBindings(bindings, n.toolArtifactBindings(availability.toolArtifacts))
	addNativeToolBindings(bindings, n.toolApprovalBindings(availability.toolApprovals))
	addNativeToolBindings(bindings, n.clarifyToolBindings(availability.clarify))
	addNativeToolBindings(bindings, n.skillToolBindings(availability.skills))
	addNativeToolBindings(
		bindings,
		n.networkToolBindings(availability.network, availability.networkRead, availability.networkUsage),
	)
	addNativeToolBindings(bindings, n.sessionToolBindings(availability.sessions, availability.sessionCatalog))
	addNativeToolBindings(
		bindings,
		n.authoredContextToolBindings(
			availability.sessionHealth,
			availability.heartbeatStatus,
			availability.heartbeatWake,
		),
	)
	addNativeToolBindings(
		bindings,
		n.workspaceToolBindings(availability.workspaces, availability.workspaceDetails, availability.agentCreate),
	)
	addNativeToolBindings(bindings, n.providerModelToolBindings(
		n.providerModelReadAvailability(),
		n.providerModelMutationAvailability(),
	))
	addNativeToolBindings(bindings, n.memoryToolBindings(availability.memory))
	addNativeToolBindings(bindings, n.memoryAdminToolBindings(memoryAdminAvailabilitySet{
		store:         availability.memoryAdminStore,
		extractor:     availability.memoryExtractor,
		providers:     availability.memoryProviders,
		sessionLedger: availability.memorySessionLedger,
	}))
	addNativeToolBindings(bindings, n.observeToolBindings(availability.observe))
	addNativeToolBindings(bindings, n.bridgeToolBindings(availability.bridges))
	addNativeToolBindings(bindings, n.taskToolBindings(availability.tasks, availability.taskNotifications))
	addNativeToolBindings(bindings, n.autonomyToolBindings(availability.tasks))
	addNativeToolBindings(bindings, n.configToolBindings(availability.config))
	addNativeToolBindings(bindings, n.hookToolBindings(availability.hookRead, availability.hookMutation))
	addNativeToolBindings(bindings, n.loopToolBindings(availability.loops))
	addNativeToolBindings(bindings, n.automationToolBindings(availability.automation))
	addNativeToolBindings(bindings, n.marketplaceToolBindings(availability.marketplace))
	addNativeToolBindings(bindings, n.extensionToolBindings(availability.extensions))
	addNativeToolBindings(bindings, n.bundleToolBindings(availability.bundles))
	addNativeToolBindings(bindings, n.resourceToolBindings(availability.resources))
	addNativeToolBindings(bindings, n.windowManagerToolBindings(availability.windowManager))
	addNativeToolBindings(bindings, n.mcpAuthToolBindings(availability.mcpStatus, availability.mcpAuth))
	return bindings
}
