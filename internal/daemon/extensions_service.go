package daemon

import (
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/api/udsapi"
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	marketplacepkg "github.com/compozy/compozy/internal/marketplace"
	mcppkg "github.com/compozy/compozy/internal/mcp"
	"github.com/compozy/compozy/internal/resources"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type daemonExtensionService struct {
	marketplaceMu      sync.RWMutex
	searchMu           sync.Mutex
	consumerSyncOnce   sync.Once
	consumerSyncGate   chan struct{}
	searchCache        map[string]extensionSearchSnapshot
	registry           *extensionpkg.Registry
	runtime            extensionRuntime
	hookBinds          hookBindingPublisher
	agentSkill         agentSkillPublisher
	toolMCP            toolMCPPublisher
	loops              loopResourcePublisher
	extensionKit       extensionKitResourcePublisher
	homePaths          compozyconfig.HomePaths
	logger             *slog.Logger
	now                func() time.Time
	getenv             func(string) string
	extensionConfig    compozyconfig.ExtensionsConfig
	marketplaceLoader  extensionMarketplaceSourceLoader
	marketplaceCatalog marketplacepkg.Service
	eventWriter        extensionLifecycleEventWriter
	workspaceResolver  workspacepkg.RuntimeResolver
	envBindings        extensionpkg.EnvBindingLifecycleStore
	secretVault        extensionSecretVault
	automation         extensionAutomationPreviewer
	lifecycle          *extensionLifecycleCoordinator
	resourceStore      resources.RawStore
	resourceActor      resources.MutationActor
	resourceCodecs     *resources.CodecRegistry
	mcpRuntimeHealth   *mcppkg.RuntimeHealthRegistry
}

var _ udsapi.ExtensionService = (*daemonExtensionService)(nil)

type daemonExtensionServiceDeps struct {
	Registry     *extensionpkg.Registry
	Runtime      extensionRuntime
	HookBindings hookBindingPublisher
	AgentSkill   agentSkillPublisher
	ToolMCP      toolMCPPublisher
	Loops        loopResourcePublisher
	HomePaths    compozyconfig.HomePaths
	Logger       *slog.Logger
	Now          func() time.Time
	Getenv       func(string) string
}

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

func withDaemonExtensionEventWriter(writer extensionLifecycleEventWriter) daemonExtensionServiceOption {
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

func withDaemonExtensionAutomation(reader extensionAutomationPreviewer) daemonExtensionServiceOption {
	return func(service *daemonExtensionService) {
		service.automation = reader
	}
}

func withDaemonExtensionResources(
	store resources.RawStore,
	actor resources.MutationActor,
	codecs *resources.CodecRegistry,
) daemonExtensionServiceOption {
	return func(service *daemonExtensionService) {
		service.resourceStore = store
		service.resourceActor = actor
		service.resourceCodecs = codecs
	}
}

func withDaemonExtensionMCPRuntimeHealth(registry *mcppkg.RuntimeHealthRegistry) daemonExtensionServiceOption {
	return func(service *daemonExtensionService) {
		service.mcpRuntimeHealth = registry
	}
}

func newDaemonExtensionService(
	deps daemonExtensionServiceDeps,
	opts ...daemonExtensionServiceOption,
) udsapi.ExtensionService {
	if deps.Registry == nil {
		return nil
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	if deps.Now == nil {
		deps.Now = func() time.Time {
			return time.Now().UTC()
		}
	}
	if deps.Getenv == nil {
		deps.Getenv = os.Getenv
	}
	service := &daemonExtensionService{
		registry:   deps.Registry,
		runtime:    deps.Runtime,
		hookBinds:  deps.HookBindings,
		agentSkill: deps.AgentSkill,
		toolMCP:    deps.ToolMCP,
		loops:      deps.Loops,
		homePaths:  deps.HomePaths,
		logger:     deps.Logger,
		now:        deps.Now,
		getenv:     deps.Getenv,
		lifecycle:  newExtensionLifecycleCoordinator(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	return service
}

func (s *daemonExtensionService) extensionConsumerSyncGate() chan struct{} {
	s.consumerSyncOnce.Do(func() {
		s.consumerSyncGate = make(chan struct{}, 1)
		s.consumerSyncGate <- struct{}{}
	})
	return s.consumerSyncGate
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
