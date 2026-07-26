package observe

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"sync"
	"time"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	"github.com/compozy/agh/internal/store/sessiondb"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/version"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

// Registry is the narrowed global persistence surface consumed by observe/.
type Registry interface {
	RegisterSession(ctx context.Context, session store.SessionInfo) error
	ListSessions(ctx context.Context, query store.SessionListQuery) ([]store.SessionInfo, error)
	ReconcileSessions(ctx context.Context, sessions []store.SessionInfo) (store.ReconcileResult, error)
	WriteEventSummary(ctx context.Context, summary store.EventSummary) error
	ListEventSummaries(ctx context.Context, query store.EventSummaryQuery) ([]store.EventSummary, error)
	RecordTokenUsage(ctx context.Context, stats store.TokenStatsUpdate, daily store.TokenUsageDailyUpdate) error
	ListTokenStats(ctx context.Context, query store.TokenStatsQuery) ([]store.TokenStats, error)
	WritePermissionLog(ctx context.Context, entry store.PermissionLogEntry) error
	ListPermissionLog(ctx context.Context, query store.PermissionLogQuery) ([]store.PermissionLogEntry, error)
	ListNetworkAudit(ctx context.Context, query store.NetworkAuditQuery) ([]store.NetworkAuditEntry, error)
	ListTasks(ctx context.Context, query taskpkg.Query) ([]taskpkg.Summary, error)
	CountDependencies(ctx context.Context, taskID string) (int, error)
	ListTaskRuns(ctx context.Context, query taskpkg.RunQuery) ([]taskpkg.Run, error)
	ListTaskEvents(ctx context.Context, query taskpkg.EventQuery) ([]taskpkg.Event, error)
	ListTaskTriageStates(ctx context.Context, actor taskpkg.ActorIdentity) ([]taskpkg.TriageState, error)
	Path() string
	Close(ctx context.Context) error
}

// SessionSource reports the currently active in-memory sessions.
type SessionSource interface {
	List() []*session.Info
}

// PermissionModeResolver resolves the effective permission mode for a live
// session using its durable workspace reference.
type PermissionModeResolver func(ctx context.Context, agentName, workspaceID string) (string, error)

// VersionSource returns the current daemon build metadata.
type VersionSource func() version.Info

// MemoryEventSource exposes canonical memory events across all memory DB authorities.
type MemoryEventSource interface {
	ListMemoryEventSummaries(
		ctx context.Context,
		workspaces []string,
		query store.EventSummaryQuery,
	) ([]store.EventSummary, error)
}

// HookCatalogSource provides resolved hook catalog views from the live runtime.
type HookCatalogSource interface {
	Catalog(filter hookspkg.CatalogFilter) ([]hookspkg.CatalogEntry, error)
}

// HookRunStore is the session-scoped storage surface used for hook run audits.
type HookRunStore interface {
	RecordHookRun(context.Context, hookspkg.HookRunRecord) error
	QueryHookRuns(context.Context, store.HookRunQuery) ([]hookspkg.HookRunRecord, error)
	Close(context.Context) error
}

// HookStoreOpener opens the per-session store used for hook run audit queries.
type HookStoreOpener func(ctx context.Context, sessionID string, path string) (HookRunStore, error)

// Option customizes Observer construction.
type Option func(*Observer)

type observedSession struct {
	agentName       string
	provider        string
	model           string
	authMode        aghconfig.ProviderAuthMode
	workspaceID     string
	permissionMode  string
	parentSessionID string
	rootSessionID   string
	spawnDepth      int
}

// TaskHealthConfig controls task-run stuck detection in the read-side health view.
type TaskHealthConfig struct {
	ClaimedStuckAfter  time.Duration
	StartingStuckAfter time.Duration
	RunningStuckAfter  time.Duration
}

// TaskDashboardConfig controls task dashboard freshness/backlog thresholds and active-run list size.
type TaskDashboardConfig struct {
	ActiveRunLimit   int
	BacklogWarnAfter time.Duration
	StaleAfter       time.Duration
}

// AgentProbeTargetSource resolves the currently configured ACP-compatible
// agent/provider commands that should be checked in observe health.
type AgentProbeTargetSource func(ctx context.Context) ([]acp.ProbeTarget, error)

type taskDashboardConfig struct {
	activeRunLimit   int
	backlogWarnAfter time.Duration
	staleAfter       time.Duration
}

// Observer implements session.Notifier and exposes query/health helpers for global observability.
type Observer struct {
	mu sync.RWMutex

	registry              Registry
	homePaths             aghconfig.HomePaths
	sessionSource         SessionSource
	resolvePermissionMode PermissionModeResolver
	resolveProviderAuth   ProviderAuthModeResolver
	costCatalog           CostCatalog
	memoryEventSource     MemoryEventSource
	workspaceResolver     workspacepkg.RuntimeResolver
	now                   func() time.Time
	startedAt             time.Time
	logger                *slog.Logger
	versionSource         VersionSource
	sessions              map[string]observedSession
	bridgeSource          BridgeSource
	bridgeState           map[string]observedBridgeState
	hookCatalogSource     HookCatalogSource
	openHookStore         HookStoreOpener
	taskHealthConfig      TaskHealthConfig
	taskDashboardConfig   taskDashboardConfig
	retention             RetentionConfig
	retentionHealth       RetentionHealth
	retentionCancel       context.CancelFunc
	retentionWG           sync.WaitGroup
	retentionMu           sync.RWMutex
	agentProbeSource      AgentProbeTargetSource
	agentProbeTimeout     time.Duration
}

var _ session.Notifier = (*Observer)(nil)
var _ session.AgentEventNotifier = (*Observer)(nil)

// WithRegistry injects the global registry implementation used by observe/.
func WithRegistry(registry Registry) Option {
	return func(observer *Observer) {
		observer.registry = registry
	}
}

// WithHomePaths overrides the AGH home layout used by observe/.
func WithHomePaths(homePaths aghconfig.HomePaths) Option {
	return func(observer *Observer) {
		observer.homePaths = homePaths
	}
}

// WithSessionSource injects the active session source used for health metrics.
func WithSessionSource(source SessionSource) Option {
	return func(observer *Observer) {
		observer.sessionSource = source
	}
}

// WithPermissionModeResolver injects custom permission resolution logic.
func WithPermissionModeResolver(resolver PermissionModeResolver) Option {
	return func(observer *Observer) {
		observer.resolvePermissionMode = resolver
	}
}

// WithMemoryEventSource injects the Memory v2 canonical event aggregation source.
func WithMemoryEventSource(source MemoryEventSource) Option {
	return func(observer *Observer) {
		observer.memoryEventSource = source
	}
}

// WithWorkspaceResolver injects workspace resolution for config lookups that
// need a filesystem root.
func WithWorkspaceResolver(resolver workspacepkg.RuntimeResolver) Option {
	return func(observer *Observer) {
		observer.workspaceResolver = resolver
	}
}

// WithLogger injects the logger used for best-effort observer failures.
func WithLogger(logger *slog.Logger) Option {
	return func(observer *Observer) {
		observer.logger = logger
	}
}

// WithNow overrides the observer clock, mainly for tests.
func WithNow(now func() time.Time) Option {
	return func(observer *Observer) {
		observer.now = now
	}
}

// WithStartTime overrides the recorded daemon start time.
func WithStartTime(startedAt time.Time) Option {
	return func(observer *Observer) {
		observer.startedAt = startedAt
	}
}

// WithVersionSource overrides the build metadata source.
func WithVersionSource(source VersionSource) Option {
	return func(observer *Observer) {
		observer.versionSource = source
	}
}

// WithHookCatalogSource injects the runtime hook catalog source used by hook introspection.
func WithHookCatalogSource(source HookCatalogSource) Option {
	return func(observer *Observer) {
		observer.hookCatalogSource = source
	}
}

// WithHookStoreOpener overrides the per-session hook run store opener, mainly for tests.
func WithHookStoreOpener(opener HookStoreOpener) Option {
	return func(observer *Observer) {
		observer.openHookStore = opener
	}
}

// WithTaskHealthConfig overrides the task-run stuck thresholds used by the
// observer health view.
func WithTaskHealthConfig(cfg TaskHealthConfig) Option {
	return func(observer *Observer) {
		observer.taskHealthConfig = cfg
	}
}

// WithTaskDashboardConfig overrides the dashboard thresholds and active-run
// list sizing used by the observer task dashboard view.
func WithTaskDashboardConfig(cfg TaskDashboardConfig) Option {
	return func(observer *Observer) {
		observer.taskDashboardConfig = normalizeTaskDashboardConfig(observer.taskDashboardConfig, cfg)
	}
}

// WithObservabilityConfig applies observability settings that affect observer-owned background work.
func WithObservabilityConfig(cfg aghconfig.ObservabilityConfig) Option {
	return func(observer *Observer) {
		observer.retention = RetentionConfigFromObservability(cfg)
	}
}

// WithRetentionConfig overrides retention behavior, mainly for tests.
func WithRetentionConfig(cfg RetentionConfig) Option {
	return func(observer *Observer) {
		observer.retention = cfg
	}
}

// WithAgentProbeSource injects the downstream ACP command source used by health.
func WithAgentProbeSource(source AgentProbeTargetSource, timeout time.Duration) Option {
	return func(observer *Observer) {
		observer.agentProbeSource = source
		observer.agentProbeTimeout = timeout
	}
}

func defaultTaskDashboardConfig() taskDashboardConfig {
	return taskDashboardConfig{
		activeRunLimit:   4,
		backlogWarnAfter: 10 * time.Minute,
		staleAfter:       2 * time.Minute,
	}
}

func normalizeTaskDashboardConfig(base taskDashboardConfig, cfg TaskDashboardConfig) taskDashboardConfig {
	normalized := base
	if normalized.activeRunLimit <= 0 {
		normalized = defaultTaskDashboardConfig()
	}
	if cfg.ActiveRunLimit > 0 {
		normalized.activeRunLimit = cfg.ActiveRunLimit
	}
	if cfg.BacklogWarnAfter > 0 {
		normalized.backlogWarnAfter = cfg.BacklogWarnAfter
	}
	if cfg.StaleAfter > 0 {
		normalized.staleAfter = cfg.StaleAfter
	}
	return normalized
}

// New constructs an Observer and opens the global AGH database when needed.
func New(ctx context.Context, opts ...Option) (*Observer, error) {
	if ctx == nil {
		return nil, errors.New("observe: context is required")
	}

	homePaths, err := aghconfig.ResolveHomePaths()
	if err != nil {
		return nil, fmt.Errorf("observe: resolve home paths: %w", err)
	}

	observer := &Observer{
		homePaths: homePaths,
		now: func() time.Time {
			return time.Now().UTC()
		},
		logger:        slog.Default(),
		versionSource: version.Current,
		sessions:      make(map[string]observedSession),
		bridgeState:   make(map[string]observedBridgeState),
		taskHealthConfig: TaskHealthConfig{
			ClaimedStuckAfter:  5 * time.Minute,
			StartingStuckAfter: 5 * time.Minute,
			RunningStuckAfter:  30 * time.Minute,
		},
		taskDashboardConfig: defaultTaskDashboardConfig(),
	}

	for _, opt := range opts {
		if opt != nil {
			opt(observer)
		}
	}

	if observer.now == nil {
		observer.now = func() time.Time {
			return time.Now().UTC()
		}
	}
	if observer.startedAt.IsZero() {
		observer.startedAt = observer.now()
	}
	if observer.logger == nil {
		observer.logger = slog.Default()
	}
	if observer.versionSource == nil {
		observer.versionSource = version.Current
	}
	if observer.sessions == nil {
		observer.sessions = make(map[string]observedSession)
	}
	if observer.bridgeState == nil {
		observer.bridgeState = make(map[string]observedBridgeState)
	}
	if observer.resolvePermissionMode == nil {
		observer.resolvePermissionMode = defaultPermissionModeResolver(observer.homePaths, observer.workspaceResolver)
	}
	if observer.resolveProviderAuth == nil {
		observer.resolveProviderAuth = defaultProviderAuthModeResolver(observer.homePaths, observer.workspaceResolver)
	}
	if observer.openHookStore == nil {
		observer.openHookStore = func(ctx context.Context, sessionID string, path string) (HookRunStore, error) {
			return sessiondb.OpenSessionDB(ctx, sessionID, path)
		}
	}
	observer.retention = normalizeRetentionConfig(observer.retention)
	observer.setRetentionHealth(observer.initialRetentionHealth())

	if observer.registry == nil {
		if err := aghconfig.EnsureHomeLayout(observer.homePaths); err != nil {
			return nil, fmt.Errorf("observe: ensure home layout: %w", err)
		}

		registry, err := globaldb.OpenGlobalDB(ctx, observer.homePaths.DatabaseFile)
		if err != nil {
			return nil, fmt.Errorf("observe: open global database: %w", err)
		}
		observer.registry = registry
	}

	return observer, nil
}
