package daemon

import (
	"strings"

	"github.com/compozy/agh/internal/api/core"
	toolspkg "github.com/compozy/agh/internal/tools"
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
	config              toolspkg.NativeAvailabilityFunc
	hookRead            toolspkg.NativeAvailabilityFunc
	hookMutation        toolspkg.NativeAvailabilityFunc
	automation          toolspkg.NativeAvailabilityFunc
	loops               toolspkg.NativeAvailabilityFunc
	extensions          toolspkg.NativeAvailabilityFunc
	marketplace         toolspkg.NativeAvailabilityFunc
	bundles             toolspkg.NativeAvailabilityFunc
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
	configReady := func() bool {
		return strings.TrimSpace(n.deps.HomePaths.ConfigFile) != ""
	}
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
		sessions: n.dependencyAvailability(func() bool { return n.deps.Sessions != nil }),
		sessionCatalog: n.dependencyAvailability(func() bool {
			_, ok := n.deps.Sessions.(core.SessionPageManager)
			return ok
		}),
		sessionHealth: n.dependencyAvailability(func() bool {
			return n.deps.SessionHealth != nil
		}),
		heartbeatStatus: n.dependencyAvailability(func() bool {
			return n.deps.HeartbeatStatus != nil && n.deps.WorkspaceResolver != nil
		}),
		heartbeatWake: n.dependencyAvailability(func() bool {
			return n.deps.HeartbeatWake != nil && n.deps.WorkspaceResolver != nil
		}),
		workspaces: n.dependencyAvailability(func() bool {
			return n.deps.Workspaces != nil
		}),
		workspaceDetails: n.dependencyAvailability(func() bool {
			return n.deps.Workspaces != nil && n.deps.Sessions != nil
		}),
		agentCreate: n.dependencyAvailability(func() bool {
			return n.deps.Workspaces != nil && strings.TrimSpace(n.deps.HomePaths.AgentsDir) != ""
		}),
		taskNotifications: n.dependencyAvailability(func() bool {
			return n.deps.Tasks != nil && n.deps.Bridges != nil
		}),
		memory:           n.dependencyAvailability(func() bool { return n.deps.MemoryStore != nil }),
		memoryAdminStore: n.dependencyAvailability(func() bool { return n.deps.MemoryStore != nil }),
		memoryExtractor:  n.dependencyAvailability(func() bool { return n.deps.MemoryExtractor != nil }),
		memoryProviders:  n.dependencyAvailability(func() bool { return n.deps.MemoryProviders != nil }),
		memorySessionLedger: n.dependencyAvailability(func() bool {
			return n.deps.MemorySessionLedger != nil
		}),
		observe: n.dependencyAvailability(func() bool {
			return n.deps.Observer != nil
		}),
		bridges:  n.dependencyAvailability(n.bridgeCatalogReady),
		tasks:    n.dependencyAvailability(func() bool { return n.deps.Tasks != nil }),
		config:   n.dependencyAvailability(configReady),
		hookRead: n.dependencyAvailability(func() bool { return n.deps.Observer != nil }),
		hookMutation: n.dependencyAvailability(func() bool {
			return configReady() && n.deps.Observer != nil
		}),
		automation: n.dependencyAvailability(func() bool { return n.automationManager() != nil }),
		loops:      n.dependencyAvailability(func() bool { return n.loopService() != nil }),
		extensions: n.dependencyAvailability(func() bool {
			return n.deps.ExtensionRegistry != nil && strings.TrimSpace(n.deps.HomePaths.HomeDir) != ""
		}),
		marketplace: n.dependencyAvailability(func() bool {
			return n.deps.MarketplaceCatalog != nil || n.bundleService() != nil || n.deps.MarketplaceSkills != nil
		}),
		bundles:   n.dependencyAvailability(func() bool { return n.bundleService() != nil }),
		resources: n.dependencyAvailability(func() bool { return n.deps.Resources != nil }),
		mcpStatus: n.dependencyAvailability(func() bool {
			return n.mcpAuthProvider() != nil && n.settingsService() != nil
		}),
		mcpAuth: n.dependencyAvailability(func() bool { return n.mcpAuthProvider() != nil }),
	}
}
