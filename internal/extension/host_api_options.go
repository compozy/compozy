package extensionpkg

import (
	"context"

	"time"

	"github.com/compozy/agh/internal/memory"

	"github.com/compozy/agh/internal/resources"

	"github.com/compozy/agh/internal/store"

	workspacepkg "github.com/compozy/agh/internal/workspace"
)

// WithHostAPICapabilityChecker injects the capability checker used for Host API authorization.
func WithHostAPICapabilityChecker(checker *CapabilityChecker) HostAPIOption {
	return func(handler *HostAPIHandler) {
		handler.capChecker = checker
	}
}

// WithHostAPIAutomationManager injects the automation manager used for automation Host API methods.
func WithHostAPIAutomationManager(manager HostAPIAutomationManager) HostAPIOption {
	return func(handler *HostAPIHandler) {
		handler.automation = manager
	}
}

// WithHostAPITaskManager injects the task manager used for task Host API methods.
func WithHostAPITaskManager(manager hostAPITaskManager) HostAPIOption {
	return func(handler *HostAPIHandler) {
		handler.tasks = manager
	}
}

// WithHostAPINetworkService injects the network runtime used by network Host API methods.
func WithHostAPINetworkService(service hostAPINetworkService) HostAPIOption {
	return func(handler *HostAPIHandler) {
		handler.network = service
	}
}

// WithHostAPINetworkStore injects the durable conversation store used by network Host API methods.
func WithHostAPINetworkStore(networkStore store.NetworkConversationStore) HostAPIOption {
	return func(handler *HostAPIHandler) {
		handler.networkStore = networkStore
	}
}

// WithHostAPINetworkUsageStore injects the bounded Network usage reader.
func WithHostAPINetworkUsageStore(networkUsage store.NetworkUsageStore) HostAPIOption {
	return func(handler *HostAPIHandler) {
		handler.networkUsage = networkUsage
	}
}

// WithHostAPIAutomationGetter injects a lazy automation lookup used when the runtime boots after extensions.
func WithHostAPIAutomationGetter(getter func() HostAPIAutomationManager) HostAPIOption {
	return func(handler *HostAPIHandler) {
		handler.automationGetter = getter
	}
}

// WithHostAPIWorkspaceResolver injects workspace resolution for workspace-scoped Host API methods.
func WithHostAPIWorkspaceResolver(resolver workspacepkg.RuntimeResolver) HostAPIOption {
	return func(handler *HostAPIHandler) {
		handler.workspaces = resolver
	}
}

// WithHostAPIBridgeRegistry injects the bridge registry used by bridge Host API methods.
func WithHostAPIBridgeRegistry(registry hostAPIBridgeRegistry) HostAPIOption {
	return func(handler *HostAPIHandler) {
		handler.bridges = registry
	}
}

// WithHostAPIBridgeDedupStore injects the dedup persistence used by inbound bridge ingest.
func WithHostAPIBridgeDedupStore(store hostAPIBridgeDedupStore) HostAPIOption {
	return func(handler *HostAPIHandler) {
		handler.dedupStore = store
	}
}

// WithHostAPIDeliveryBroker injects the session-to-bridge delivery projection broker.
func WithHostAPIDeliveryBroker(broker hostAPIDeliveryBroker) HostAPIOption {
	return func(handler *HostAPIHandler) {
		handler.deliveryBroker = broker
	}
}

// WithHostAPIResourceStore injects the canonical raw resource store used by
// the extension resource Host API methods.
func WithHostAPIResourceStore(store resources.RawStore) HostAPIOption {
	return func(handler *HostAPIHandler) {
		handler.resourceStore = store
	}
}

// WithHostAPIResourceCodecRegistry injects resource codecs used to validate
// and canonicalize snapshot specs before persistence.
func WithHostAPIResourceCodecRegistry(registry *resources.CodecRegistry) HostAPIOption {
	return func(handler *HostAPIHandler) {
		handler.resourceCodecs = registry
	}
}

// WithHostAPIResourceTrigger injects the reconcile trigger used after
// successful snapshot writes.
func WithHostAPIResourceTrigger(
	trigger func(context.Context, resources.ResourceKind, resources.ReconcileReason) error,
) HostAPIOption {
	return func(handler *HostAPIHandler) {
		handler.resourceTrigger = trigger
	}
}

// WithHostAPISoulAuthoring injects managed SOUL.md read and mutation support.
func WithHostAPISoulAuthoring(service hostAPISoulAuthoringService) HostAPIOption {
	return func(handler *HostAPIHandler) {
		handler.soulAuthoring = service
	}
}

// WithHostAPISoulRefresher injects managed session Soul refresh support.
func WithHostAPISoulRefresher(refresher hostAPISoulRefresher) HostAPIOption {
	return func(handler *HostAPIHandler) {
		handler.soulRefresher = refresher
	}
}

// WithHostAPIHeartbeatAuthoring injects managed HEARTBEAT.md mutation support.
func WithHostAPIHeartbeatAuthoring(service hostAPIHeartbeatAuthoringService) HostAPIOption {
	return func(handler *HostAPIHandler) {
		handler.heartbeatAuthor = service
	}
}

// WithHostAPIHeartbeatStatus injects managed Heartbeat status support.
func WithHostAPIHeartbeatStatus(service hostAPIHeartbeatStatusService) HostAPIOption {
	return func(handler *HostAPIHandler) {
		handler.heartbeatStatus = service
	}
}

// WithHostAPIHeartbeatWake injects managed Heartbeat wake support.
func WithHostAPIHeartbeatWake(service hostAPIHeartbeatWakeService) HostAPIOption {
	return func(handler *HostAPIHandler) {
		handler.heartbeatWake = service
	}
}

// WithHostAPISessionHealth injects metadata-only session health reads.
func WithHostAPISessionHealth(reader hostAPISessionHealthReader) HostAPIOption {
	return func(handler *HostAPIHandler) {
		handler.sessionHealth = reader
	}
}

// WithHostAPIHeartbeatWakeEvents injects retained wake audit reads.
func WithHostAPIHeartbeatWakeEvents(reader hostAPIHeartbeatWakeEventReader) HostAPIOption {
	return func(handler *HostAPIHandler) {
		handler.wakeEvents = reader
	}
}

// WithHostAPIBridgeIngressConfig overrides dedup TTL and cleanup cadence for bridge ingest.
func WithHostAPIBridgeIngressConfig(dedupTTL time.Duration, cleanupInterval time.Duration) HostAPIOption {
	return func(handler *HostAPIHandler) {
		handler.bridgeIngestDedupTTL = dedupTTL
		handler.bridgeCleanupInterval = cleanupInterval
	}
}

// WithHostAPIMemoryProviderRegistry injects MemoryProvider registration state.
func WithHostAPIMemoryProviderRegistry(registry *MemoryProviderRegistry) HostAPIOption {
	return func(handler *HostAPIHandler) {
		handler.memoryProviders = registry
	}
}

// WithHostAPIRateLimit overrides the per-extension Host API token bucket settings.
func WithHostAPIRateLimit(limit int, burst int) HostAPIOption {
	return func(handler *HostAPIHandler) {
		handler.rateLimit = limit
		handler.rateBurst = burst
	}
}

// WithHostAPINow overrides the handler clock, mainly for tests.
func WithHostAPINow(now func() time.Time) HostAPIOption {
	return func(handler *HostAPIHandler) {
		handler.now = now
	}
}

// NewHostAPIHandler constructs a Host API handler with sensible defaults.
func NewHostAPIHandler(
	sessions hostAPISessionManager,
	memoryStore *memory.Store,
	observer hostAPIObserver,
	skillsRegistry hostAPISkillsRegistry,
	opts ...HostAPIOption,
) *HostAPIHandler {
	handler := newHostAPIHandlerDefaults(sessions, memoryStore, observer, skillsRegistry)

	for _, opt := range opts {
		if opt != nil {
			opt(handler)
		}
	}

	normalizeHostAPIHandlerDefaults(handler)
	handler.limiter = newHostAPIRateLimiter(handler.rateLimit, handler.rateBurst, handler.now)
	handler.methods = hostAPIMethodHandlers(handler)

	return handler
}

func newHostAPIHandlerDefaults(
	sessions hostAPISessionManager,
	memoryStore *memory.Store,
	observer hostAPIObserver,
	skillsRegistry hostAPISkillsRegistry,
) *HostAPIHandler {
	return &HostAPIHandler{
		sessions:              sessions,
		memory:                memoryStore,
		observer:              observer,
		skills:                skillsRegistry,
		capChecker:            &CapabilityChecker{},
		rateLimit:             defaultHostAPIRateLimit,
		rateBurst:             defaultHostAPIBurst,
		bridgeIngestDedupTTL:  defaultHostAPIBridgeIngestDedupTTL,
		bridgeCleanupInterval: defaultHostAPIBridgeCleanupInterval,
		bridgeLocks:           newHostAPIKeyLocker(),
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func normalizeHostAPIHandlerDefaults(handler *HostAPIHandler) {
	if handler == nil {
		return
	}
	if handler.now == nil {
		handler.now = func() time.Time {
			return time.Now().UTC()
		}
	}
	if handler.capChecker == nil {
		handler.capChecker = &CapabilityChecker{}
	}
	if handler.bridgeIngestDedupTTL <= 0 {
		handler.bridgeIngestDedupTTL = defaultHostAPIBridgeIngestDedupTTL
	}
	if handler.bridgeCleanupInterval <= 0 {
		handler.bridgeCleanupInterval = defaultHostAPIBridgeCleanupInterval
	}
	if handler.bridgeLocks == nil {
		handler.bridgeLocks = newHostAPIKeyLocker()
	}
}
