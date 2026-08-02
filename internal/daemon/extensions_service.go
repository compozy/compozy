package daemon

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/api/udsapi"
	automationpkg "github.com/compozy/compozy/internal/automation"
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	marketplacepkg "github.com/compozy/compozy/internal/marketplace"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/vault"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type daemonExtensionService struct {
	marketplaceMu      sync.RWMutex
	searchMu           sync.Mutex
	searchCache        map[string]extensionSearchSnapshot
	registry           *extensionpkg.Registry
	runtime            extensionRuntime
	hookBinds          hookBindingPublisher
	agentSkill         agentSkillPublisher
	toolMCP            toolMCPPublisher
	bundles            bundleResourcePublisher
	loops              loopResourcePublisher
	extensionKit       extensionKitResourcePublisher
	homePaths          compozyconfig.HomePaths
	logger             *slog.Logger
	now                func() time.Time
	extensionConfig    compozyconfig.ExtensionsConfig
	marketplaceLoader  extensionMarketplaceSourceLoader
	marketplaceCatalog marketplacepkg.Service
	eventWriter        store.EventSummaryStore
	workspaceResolver  workspacepkg.RuntimeResolver
	envBindings        extensionpkg.EnvBindingLifecycleStore
	secretVault        extensionSecretVault
	automation         extensionAutomationReader
	lifecycle          extensionLifecycleCoordinator
	resourceStore      resources.RawStore
	resourceActor      resources.MutationActor
}

var _ udsapi.ExtensionService = (*daemonExtensionService)(nil)

type daemonExtensionServiceOption func(*daemonExtensionService)

func withDaemonExtensionMarketplace(
	cfg compozyconfig.ExtensionsConfig,
	loader extensionMarketplaceSourceLoader,
) daemonExtensionServiceOption {
	return func(service *daemonExtensionService) {
		service.extensionConfig = cfg
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

func withDaemonExtensionWorkspaceResolver(
	resolver workspacepkg.RuntimeResolver,
) daemonExtensionServiceOption {
	return func(service *daemonExtensionService) {
		service.workspaceResolver = resolver
	}
}

func withDaemonExtensionKitPublisher(publisher extensionKitResourcePublisher) daemonExtensionServiceOption {
	return func(service *daemonExtensionService) {
		service.extensionKit = publisher
	}
}

func withDaemonExtensionSecrets(
	bindings extensionpkg.EnvBindingLifecycleStore,
	secretVault extensionSecretVault,
) daemonExtensionServiceOption {
	return func(service *daemonExtensionService) {
		service.envBindings = bindings
		service.secretVault = secretVault
	}
}

type extensionSecretVault interface {
	PutSecret(context.Context, string, string, string) (vault.Metadata, error)
	ResolveRef(context.Context, string) (string, error)
	GetMetadata(context.Context, string) (vault.Metadata, error)
	DeleteSecret(context.Context, string) error
}

type extensionAutomationReader interface {
	Jobs(context.Context) ([]automationpkg.Job, error)
	Triggers(context.Context) ([]automationpkg.Trigger, error)
}

type extensionAutomationPreviewer interface {
	EffectivePackageAutomation(
		context.Context,
		[]automationpkg.Job,
		[]automationpkg.Trigger,
	) ([]automationpkg.Job, []automationpkg.Trigger, error)
}

func withDaemonExtensionAutomation(reader extensionAutomationReader) daemonExtensionServiceOption {
	return func(service *daemonExtensionService) {
		service.automation = reader
	}
}

func withDaemonExtensionResources(
	store resources.RawStore,
	actor resources.MutationActor,
) daemonExtensionServiceOption {
	return func(service *daemonExtensionService) {
		service.resourceStore = store
		service.resourceActor = actor
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
	homePaths compozyconfig.HomePaths,
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
		lifecycle:  *newExtensionLifecycleCoordinator(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	return service
}

func (s *daemonExtensionService) marketplaceConfig() compozyconfig.ExtensionsConfig {
	s.marketplaceMu.RLock()
	defer s.marketplaceMu.RUnlock()
	return s.extensionConfig
}

func (s *daemonExtensionService) reconcileMarketplaceConfig(cfg compozyconfig.ExtensionsConfig) {
	s.marketplaceMu.Lock()
	s.extensionConfig = cfg
	s.marketplaceMu.Unlock()
}
