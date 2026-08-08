package daemon

import (
	"strings"

	"github.com/compozy/compozy/internal/api/core"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type nativeToolAvailabilitySet struct {
	registry            toolspkg.NativeAvailabilityFunc
	toolArtifacts       toolspkg.NativeAvailabilityFunc
	toolApprovals       toolspkg.NativeAvailabilityFunc
	clarify             toolspkg.NativeAvailabilityFunc
	skills              toolspkg.NativeAvailabilityFunc
	network             toolspkg.NativeAvailabilityFunc
	networkRead         toolspkg.NativeAvailabilityFunc
	networkUsage        toolspkg.NativeAvailabilityFunc
	sessions            toolspkg.NativeAvailabilityFunc
	sessionCatalog      toolspkg.NativeAvailabilityFunc
	sessionRuntime      toolspkg.NativeAvailabilityFunc
	sessionHealth       toolspkg.NativeAvailabilityFunc
	heartbeatStatus     toolspkg.NativeAvailabilityFunc
	heartbeatWake       toolspkg.NativeAvailabilityFunc
	workspaces          toolspkg.NativeAvailabilityFunc
	workspaceDetails    toolspkg.NativeAvailabilityFunc
	agentCreate         toolspkg.NativeAvailabilityFunc
	tasks               toolspkg.NativeAvailabilityFunc
	taskNotifications   toolspkg.NativeAvailabilityFunc
	memory              toolspkg.NativeAvailabilityFunc
	memoryAdminStore    toolspkg.NativeAvailabilityFunc
	memoryExtractor     toolspkg.NativeAvailabilityFunc
	memoryProviders     toolspkg.NativeAvailabilityFunc
	memorySessionLedger toolspkg.NativeAvailabilityFunc
	observe             toolspkg.NativeAvailabilityFunc
	bridges             toolspkg.NativeAvailabilityFunc
	gateway             toolspkg.NativeAvailabilityFunc
	config              toolspkg.NativeAvailabilityFunc
	hookRead            toolspkg.NativeAvailabilityFunc
	hookMutation        toolspkg.NativeAvailabilityFunc
	automation          toolspkg.NativeAvailabilityFunc
	loops               toolspkg.NativeAvailabilityFunc
	extensions          toolspkg.NativeAvailabilityFunc
	marketplace         toolspkg.NativeAvailabilityFunc
	resources           toolspkg.NativeAvailabilityFunc
	windowManager       toolspkg.NativeAvailabilityFunc
	mcpStatus           toolspkg.NativeAvailabilityFunc
	mcpAuth             toolspkg.NativeAvailabilityFunc
}

func (n *daemonNativeTools) nativeToolAvailability() nativeToolAvailabilitySet {
	availability := n.baseNativeToolAvailability()
	availability.windowManager = n.windowManagerAvailability()
	return availability
}

func (n *daemonNativeTools) baseNativeToolAvailability() nativeToolAvailabilitySet {
	availability := n.coreNativeToolAvailability()
	n.applySessionNativeToolAvailability(&availability)
	n.applyMemoryNativeToolAvailability(&availability)
	n.applyServiceNativeToolAvailability(&availability)
	return availability
}

func (n *daemonNativeTools) coreNativeToolAvailability() nativeToolAvailabilitySet {
	return nativeToolAvailabilitySet{
		registry: n.registryAvailability(),
		toolArtifacts: n.dependencyAvailability(func() bool {
			return n.deps.ToolArtifacts != nil
		}),
		toolApprovals: n.dependencyAvailability(func() bool {
			return n.deps.ApprovalGrants != nil
		}),
		clarify: n.dependencyAvailability(func() bool {
			return n.clarifyBroker() != nil
		}),
		skills:  n.dependencyAvailability(func() bool { return n.deps.Skills != nil }),
		network: n.networkParticipationAvailability(func() bool { return n.deps.Network != nil }),
		networkRead: n.networkParticipationAvailability(func() bool {
			return n.deps.Network != nil && n.deps.NetworkStore != nil
		}),
		networkUsage: n.networkParticipationAvailability(func() bool {
			return n.deps.Network != nil && n.deps.NetworkUsage != nil
		}),
	}
}

func (n *daemonNativeTools) applySessionNativeToolAvailability(availability *nativeToolAvailabilitySet) {
	availability.sessions = n.dependencyAvailability(func() bool { return n.deps.Sessions != nil })
	availability.sessionCatalog = n.dependencyAvailability(func() bool {
		_, ok := n.deps.Sessions.(core.SessionPageManager)
		return ok
	})
	availability.sessionRuntime = n.sessionRuntimeAvailability()
	availability.sessionHealth = n.dependencyAvailability(func() bool {
		return n.deps.SessionHealth != nil
	})
	availability.heartbeatStatus = n.dependencyAvailability(func() bool {
		return n.deps.HeartbeatStatus != nil && n.deps.WorkspaceResolver != nil
	})
	availability.heartbeatWake = n.dependencyAvailability(func() bool {
		return n.deps.HeartbeatWake != nil && n.deps.WorkspaceResolver != nil
	})
	availability.workspaces = n.dependencyAvailability(func() bool {
		return n.deps.Workspaces != nil
	})
	availability.workspaceDetails = n.dependencyAvailability(func() bool {
		return n.deps.Workspaces != nil && n.deps.Sessions != nil
	})
	availability.agentCreate = n.dependencyAvailability(func() bool {
		return n.deps.Workspaces != nil && strings.TrimSpace(n.deps.HomePaths.AgentsDir) != ""
	})
	n.applyTaskNativeToolAvailability(availability)
}

func (n *daemonNativeTools) applyTaskNativeToolAvailability(availability *nativeToolAvailabilitySet) {
	availability.tasks = n.dependencyAvailability(func() bool { return n.deps.Tasks != nil })
	availability.taskNotifications = n.dependencyAvailability(func() bool {
		return n.deps.Tasks != nil && n.deps.Bridges != nil
	})
}

func (n *daemonNativeTools) applyMemoryNativeToolAvailability(availability *nativeToolAvailabilitySet) {
	availability.memory = n.dependencyAvailability(func() bool { return n.deps.MemoryStore != nil })
	availability.memoryAdminStore = n.dependencyAvailability(func() bool { return n.deps.MemoryStore != nil })
	availability.memoryExtractor = n.dependencyAvailability(func() bool { return n.deps.MemoryExtractor != nil })
	availability.memoryProviders = n.dependencyAvailability(func() bool { return n.deps.MemoryProviders != nil })
	availability.memorySessionLedger = n.dependencyAvailability(func() bool {
		return n.deps.MemorySessionLedger != nil
	})
}

func (n *daemonNativeTools) applyServiceNativeToolAvailability(availability *nativeToolAvailabilitySet) {
	configReady := func() bool {
		return strings.TrimSpace(n.deps.HomePaths.ConfigFile) != ""
	}
	availability.observe = n.dependencyAvailability(func() bool { return n.deps.Observer != nil })
	availability.bridges = n.dependencyAvailability(n.bridgeCatalogReady)
	availability.gateway = n.dependencyAvailability(func() bool { return n.deps.gatewayService() != nil })
	availability.config = n.dependencyAvailability(configReady)
	availability.hookRead = n.dependencyAvailability(func() bool { return n.deps.Observer != nil })
	availability.hookMutation = n.dependencyAvailability(func() bool {
		return configReady() && n.deps.Observer != nil
	})
	availability.automation = n.dependencyAvailability(func() bool { return n.automationManager() != nil })
	availability.loops = n.dependencyAvailability(func() bool { return n.loopService() != nil })
	availability.extensions = n.dependencyAvailability(func() bool {
		return n.deps.ExtensionRegistry != nil && strings.TrimSpace(n.deps.HomePaths.HomeDir) != ""
	})
	availability.marketplace = n.dependencyAvailability(func() bool {
		return n.deps.MarketplaceCatalog != nil || n.deps.MarketplaceSkills != nil
	})
	availability.resources = n.dependencyAvailability(func() bool { return n.deps.Resources != nil })
	availability.mcpStatus = n.dependencyAvailability(func() bool {
		return n.mcpAuthProvider() != nil && n.settingsService() != nil
	})
	availability.mcpAuth = n.dependencyAvailability(func() bool { return n.mcpAuthProvider() != nil })
}

func (n *daemonNativeTools) sessionRuntimeAvailability() toolspkg.NativeAvailabilityFunc {
	return n.dependencyAvailability(func() bool {
		_, ok := n.deps.Sessions.(core.SessionRuntimeSelectionManager)
		return ok
	})
}

func (n *daemonNativeTools) sessionCreateAvailability() toolspkg.NativeAvailabilityFunc {
	return n.dependencyAvailability(func() bool {
		if n == nil || n.deps == nil {
			return false
		}
		_, ok := n.deps.Sessions.(core.SessionAcceptanceManager)
		return ok
	})
}
