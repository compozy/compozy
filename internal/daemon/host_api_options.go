package daemon

import (
	extensionpkg "github.com/compozy/agh/internal/extension"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/store"
)

func buildHostAPIOptions(
	deps *extensionManagerDeps,
	capChecker *extensionpkg.CapabilityChecker,
	resourceStore resources.RawStore,
) []extensionpkg.HostAPIOption {
	opts := []extensionpkg.HostAPIOption{
		extensionpkg.WithHostAPIAutomationGetter(deps.Automation),
		extensionpkg.WithHostAPITaskManager(deps.Tasks),
		extensionpkg.WithHostAPINetworkService(deps.Network),
		extensionpkg.WithHostAPINetworkStore(deps.NetworkStore),
		extensionpkg.WithHostAPIModelCatalogService(deps.ModelCatalog),
		extensionpkg.WithHostAPICapabilityChecker(capChecker),
		extensionpkg.WithHostAPIWorkspaceResolver(deps.WorkspaceResolver),
		extensionpkg.WithHostAPIResourceStore(resourceStore),
		extensionpkg.WithHostAPIResourceCodecRegistry(deps.ResourceCodecs),
		extensionpkg.WithHostAPIResourceTrigger(deps.ResourceTrigger),
		extensionpkg.WithHostAPISoulAuthoring(deps.SoulAuthoring),
		extensionpkg.WithHostAPISoulRefresher(deps.SoulRefresher),
		extensionpkg.WithHostAPIHeartbeatAuthoring(deps.HeartbeatAuthor),
		extensionpkg.WithHostAPIHeartbeatStatus(deps.HeartbeatStatus),
		extensionpkg.WithHostAPIHeartbeatWake(deps.HeartbeatWake),
		extensionpkg.WithHostAPISessionHealth(deps.SessionHealth),
		extensionpkg.WithHostAPIHeartbeatWakeEvents(deps.WakeEvents),
		extensionpkg.WithHostAPIMemoryProviderRegistry(deps.MemoryProviderRegistry),
		extensionpkg.WithHostAPIClarify(deps.Clarify),
	}
	if usageStore, ok := deps.NetworkStore.(store.NetworkUsageStore); ok {
		opts = append(opts, extensionpkg.WithHostAPINetworkUsageStore(usageStore))
	}
	if deps.BridgeRegistry != nil {
		opts = append(opts, extensionpkg.WithHostAPIBridgeRegistry(deps.BridgeRegistry))
	}
	if deps.BridgeDedupStore != nil {
		opts = append(opts, extensionpkg.WithHostAPIBridgeDedupStore(deps.BridgeDedupStore))
	}
	if deps.BridgeBroker != nil {
		opts = append(opts, extensionpkg.WithHostAPIDeliveryBroker(deps.BridgeBroker))
	}
	return opts
}
