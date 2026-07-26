package daemon

import (
	"log/slog"
	"sync"
	"time"

	"github.com/compozy/agh/internal/api/udsapi"
	aghconfig "github.com/compozy/agh/internal/config"
	extensionpkg "github.com/compozy/agh/internal/extension"
	marketplacepkg "github.com/compozy/agh/internal/marketplace"
	"github.com/compozy/agh/internal/store"
)

type daemonExtensionService struct {
	marketplaceMu      sync.RWMutex
	registry           *extensionpkg.Registry
	runtime            extensionRuntime
	hookBinds          hookBindingPublisher
	agentSkill         agentSkillPublisher
	toolMCP            toolMCPPublisher
	bundles            bundleResourcePublisher
	loops              loopResourcePublisher
	homePaths          aghconfig.HomePaths
	logger             *slog.Logger
	now                func() time.Time
	marketplace        aghconfig.ExtensionsMarketplaceConfig
	marketplaceLoader  extensionMarketplaceSourceLoader
	marketplaceCatalog marketplacepkg.Service
	eventWriter        store.EventSummaryStore
}

var _ udsapi.ExtensionService = (*daemonExtensionService)(nil)

type daemonExtensionServiceOption func(*daemonExtensionService)

func withDaemonExtensionMarketplace(
	cfg aghconfig.ExtensionsMarketplaceConfig,
	loader extensionMarketplaceSourceLoader,
) daemonExtensionServiceOption {
	return func(service *daemonExtensionService) {
		service.marketplace = cfg
		service.marketplaceLoader = loader
	}
}

func withDaemonExtensionCatalog(catalog marketplacepkg.Service) daemonExtensionServiceOption {
	return func(service *daemonExtensionService) {
		service.marketplaceCatalog = catalog
	}
}

func withDaemonExtensionEventWriter(writer store.EventSummaryStore) daemonExtensionServiceOption {
	return func(service *daemonExtensionService) {
		service.eventWriter = writer
	}
}

func newDaemonExtensionService(
	registry *extensionpkg.Registry,
	runtime extensionRuntime,
	hookBinds hookBindingPublisher,
	agentSkill agentSkillPublisher,
	toolMCP toolMCPPublisher,
	bundles bundleResourcePublisher,
	loops loopResourcePublisher,
	homePaths aghconfig.HomePaths,
	logger *slog.Logger,
	now func() time.Time,
	opts ...daemonExtensionServiceOption,
) udsapi.ExtensionService {
	if registry == nil {
		return nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	if now == nil {
		now = func() time.Time {
			return time.Now().UTC()
		}
	}
	service := &daemonExtensionService{
		registry:   registry,
		runtime:    runtime,
		hookBinds:  hookBinds,
		agentSkill: agentSkill,
		toolMCP:    toolMCP,
		bundles:    bundles,
		loops:      loops,
		homePaths:  homePaths,
		logger:     logger,
		now:        now,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	return service
}

func (s *daemonExtensionService) marketplaceConfig() aghconfig.ExtensionsMarketplaceConfig {
	s.marketplaceMu.RLock()
	defer s.marketplaceMu.RUnlock()
	return s.marketplace
}

func (s *daemonExtensionService) reconcileMarketplaceConfig(cfg aghconfig.ExtensionsMarketplaceConfig) {
	s.marketplaceMu.Lock()
	s.marketplace = cfg
	s.marketplaceMu.Unlock()
}
