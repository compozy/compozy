package daemon

import (
	"github.com/compozy/compozy/internal/api/core"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/resources"
)

func buildHostAPIOptions(
	deps *extensionManagerDeps,
	capChecker *extensionpkg.CapabilityChecker,
	resourceStore resources.RawStore,
) []extensionpkg.HostAPIOption {
	opts := []extensionpkg.HostAPIOption{
		extensionpkg.WithHostAPIAutomationGetter(deps.Automation),
		extensionpkg.WithHostAPITaskManager(deps.Tasks),
		extensionpkg.WithHostAPITaskCatalogFilterMapper(core.ApplyTaskLoopCatalogFilters),
		extensionpkg.WithHostAPIModelCatalogService(deps.ModelCatalog),
		extensionpkg.WithHostAPICapabilityChecker(capChecker),
		extensionpkg.WithHostAPIWorkspaceResolver(deps.WorkspaceResolver),
		extensionpkg.WithHostAPIProfileReader(deps.Profiles),
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
		extensionpkg.WithHostAPIMemoryStoreResolver(deps.MemoryStoreResolver),
		extensionpkg.WithHostAPIClarify(deps.Clarify),
		extensionpkg.WithHostAPIViewService(deps.CmdPalette),
	}
	if deps.ViewPatches != nil {
		opts = append(opts, extensionpkg.WithHostAPIViewPatchPublisher(deps.ViewPatches))
	}
	return opts
}
