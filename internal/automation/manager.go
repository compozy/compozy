package automation

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"

	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/session"

	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/vault"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

var (
	// ErrManagerNotRunning reports that a runtime-only manager action was called
	// before Start or after Shutdown.
	ErrManagerNotRunning = errors.New("automation: manager not running")
	// ErrDefinitionReadOnly reports that a managed definition cannot be
	// mutated through the runtime CRUD surface.
	ErrDefinitionReadOnly = errors.New("automation: definition is managed and read-only")
	// ErrSessionTaskActorNotFound reports that no automation-linked task actor
	// context is recorded for the supplied session.
	ErrSessionTaskActorNotFound = errors.New("automation: session task actor context not found")
)

const managerRuntimeCleanupTimeout = 2 * time.Second

type managerRuntimeComponent interface {
	Shutdown(ctx context.Context) error
}

// SessionManager is the runtime session surface required by the automation
// manager. It extends the dispatcher path with lookup support for hook-derived
// trigger ingress.
type SessionManager interface {
	SessionCreator
	Status(ctx context.Context, id string) (*session.Info, error)
}

// WebhookSecretResolver resolves a persisted webhook secret reference.
type WebhookSecretResolver interface {
	ResolveRef(ctx context.Context, ref string) (string, error)
}

// WebhookSecretStore persists daemon-managed webhook secret values.
type WebhookSecretStore interface {
	WebhookSecretResolver
	PutSecret(ctx context.Context, ref string, kind string, value string) (vault.Metadata, error)
	DeleteSecret(ctx context.Context, ref string) error
}

// WebhookSecretWrite carries the optional write-only webhook secret mutation.
type WebhookSecretWrite struct {
	Ref   string
	Value *string
}

// ResourceStatus reports total and enabled counts for one automation resource
// family.
type ResourceStatus struct {
	Total   int `json:"total"`
	Enabled int `json:"enabled"`
}

// SyncStats summarizes one TOML synchronization pass.
type SyncStats struct {
	JobsSynced      int       `json:"jobs_synced"`
	TriggersSynced  int       `json:"triggers_synced"`
	JobsRemoved     int       `json:"jobs_removed"`
	TriggersRemoved int       `json:"triggers_removed"`
	SyncedAt        time.Time `json:"synced_at"`
}

// ManagerStatus exposes automation lifecycle, count, and next-fire metadata
// without transport-specific wrappers.
type ManagerStatus struct {
	Running          bool                `json:"running"`
	SchedulerRunning bool                `json:"scheduler_running"`
	Jobs             ResourceStatus      `json:"jobs"`
	Triggers         ResourceStatus      `json:"triggers"`
	ScheduledJobs    []ScheduledJobState `json:"scheduled_jobs,omitempty"`
	NextFire         *time.Time          `json:"next_fire,omitempty"`
	LastSync         SyncStats           `json:"last_sync"`
}

// Option customizes automation manager construction.
type Option func(*managerOptions)

type managerOptions struct {
	store               Store
	sessions            SessionManager
	tasks               TaskService
	loopStarter         LoopStarter
	workspaceResolver   workspacepkg.RuntimeResolver
	config              aghconfig.AutomationConfig
	logger              *slog.Logger
	globalWorkspacePath string
	webhookSecrets      WebhookSecretStore
	dispatcherOptions   []DispatcherOption
	schedulerOptions    []SchedulerOption
	triggerOptions      []TriggerEngineOption
	jobResources        resources.Store[Job]
	triggerResources    resources.Store[Trigger]
	resourceActor       resources.MutationActor
	resourceTrigger     func(context.Context, resources.ResourceKind, resources.ReconcileReason) error
	now                 func() time.Time
}

func defaultManagerOptions() managerOptions {
	return managerOptions{
		logger: slog.Default(),
		now: func() time.Time {
			return time.Now().UTC()
		},
		config: aghconfig.AutomationConfig{
			Timezone:          DefaultTimezone,
			MaxConcurrentJobs: DefaultMaxConcurrentJobs,
			DefaultFireLimit:  DefaultFireLimitConfig(),
			Suggestions: aghconfig.AutomationSuggestionsConfig{
				PendingCap: DefaultSuggestionPendingCap,
			},
		},
	}
}

func applyManagerOptions(options *managerOptions, opts []Option) {
	for _, opt := range opts {
		if opt != nil {
			opt(options)
		}
	}
}

func finalizeManagerOptions(options *managerOptions) error {
	if options.store == nil {
		return errors.New("automation: store is required")
	}
	if options.sessions == nil {
		return errors.New("automation: session manager is required")
	}
	if options.workspaceResolver == nil {
		return errors.New("automation: workspace resolver is required")
	}
	if options.logger == nil {
		options.logger = slog.Default()
	}
	if options.now == nil {
		options.now = func() time.Time {
			return time.Now().UTC()
		}
	}
	if strings.TrimSpace(options.config.Timezone) == "" {
		options.config.Timezone = DefaultTimezone
	}
	if options.config.MaxConcurrentJobs <= 0 {
		options.config.MaxConcurrentJobs = DefaultMaxConcurrentJobs
	}
	if options.config.DefaultFireLimit.Max == 0 || strings.TrimSpace(options.config.DefaultFireLimit.Window) == "" {
		options.config.DefaultFireLimit = DefaultFireLimitConfig()
	}
	if options.config.Suggestions.PendingCap <= 0 {
		options.config.Suggestions.PendingCap = DefaultSuggestionPendingCap
	}
	if options.jobResources != nil || options.triggerResources != nil {
		if options.jobResources == nil {
			return errors.New("automation: job resource store is required when resource definitions are enabled")
		}
		if options.triggerResources == nil {
			return errors.New("automation: trigger resource store is required when resource definitions are enabled")
		}
		if options.resourceActor.Kind == "" {
			options.resourceActor = defaultAutomationResourceActor()
		}
		if err := options.resourceActor.Kind.Normalize().Validate("automation.resource_actor.kind"); err != nil {
			return err
		}
	}
	if strings.TrimSpace(options.globalWorkspacePath) == "" {
		return errors.New("automation: global workspace path is required")
	}
	return nil
}

func managerDispatcherOptions(options *managerOptions) []DispatcherOption {
	dispatcherOpts := []DispatcherOption{
		WithDispatcherLogger(options.logger),
		WithDispatcherGlobalWorkspacePath(options.globalWorkspacePath),
		WithDispatcherMaxConcurrent(options.config.MaxConcurrentJobs),
	}
	if options.tasks != nil {
		dispatcherOpts = append(dispatcherOpts, WithDispatcherTasks(options.tasks))
	}
	if options.loopStarter != nil {
		dispatcherOpts = append(dispatcherOpts, WithDispatcherLoopStarter(options.loopStarter))
	}
	return append(dispatcherOpts, options.dispatcherOptions...)
}

// Manager composes persistence, dispatch, schedules, triggers, and runtime
// status into one daemon-owned automation subsystem.
type Manager struct {
	store               Store
	sessions            SessionManager
	tasks               TaskService
	loopStarter         LoopStarter
	workspaceResolver   workspacepkg.RuntimeResolver
	config              aghconfig.AutomationConfig
	logger              *slog.Logger
	globalWorkspacePath string
	webhookSecrets      WebhookSecretStore
	dispatcher          *Dispatcher
	schedulerOptions    []SchedulerOption
	triggerOptions      []TriggerEngineOption
	jobResources        resources.Store[Job]
	triggerResources    resources.Store[Trigger]
	resourceActor       resources.MutationActor
	resourceTrigger     func(context.Context, resources.ResourceKind, resources.ReconcileReason) error
	now                 func() time.Time

	mu                sync.RWMutex
	running           bool
	runtimeCtx        context.Context
	runtimeCancel     context.CancelFunc
	scheduler         *Scheduler
	triggers          *TriggerEngine
	lastSync          SyncStats
	projectedJobs     map[string]Job
	projectedTriggers map[string]Trigger
	jobRevision       int64
	triggerRevision   int64

	taskActorMu       sync.RWMutex
	sessionTaskActors map[string]taskpkg.ActorContext
}

// WithStore injects the automation persistence store.
func WithStore(store Store) Option {
	return func(opts *managerOptions) {
		opts.store = store
	}
}

// WithSessions injects the runtime session manager used by the dispatcher and
// hook-derived trigger ingress.
func WithSessions(sessions SessionManager) Option {
	return func(opts *managerOptions) {
		opts.sessions = sessions
	}
}

// WithTasks injects the task-domain service used for task-backed automation
// jobs.
func WithTasks(tasks TaskService) Option {
	return func(opts *managerOptions) {
		opts.tasks = tasks
	}
}

// WithWorkspaceResolver injects the canonical workspace resolver used to turn
// TOML workspace references into registered workspace IDs.
func WithWorkspaceResolver(resolver workspacepkg.RuntimeResolver) Option {
	return func(opts *managerOptions) {
		opts.workspaceResolver = resolver
	}
}

// WithConfig injects the loaded automation config.
func WithConfig(cfg aghconfig.AutomationConfig) Option {
	return func(opts *managerOptions) {
		opts.config = cfg
	}
}

// WithLogger injects the subsystem logger.
func WithLogger(logger *slog.Logger) Option {
	return func(opts *managerOptions) {
		opts.logger = logger
	}
}

// WithGlobalWorkspacePath injects the fallback workspace path used for global
// automation sessions.
func WithGlobalWorkspacePath(path string) Option {
	return func(opts *managerOptions) {
		opts.globalWorkspacePath = strings.TrimSpace(path)
	}
}

// WithWebhookSecretStore injects the vault-backed store used for webhook trigger secrets.
func WithWebhookSecretStore(store WebhookSecretStore) Option {
	return func(opts *managerOptions) {
		opts.webhookSecrets = store
	}
}

// WithHooks injects the automation lifecycle hook dispatcher used by the shared dispatcher path.
func WithHooks(hooks HookDispatcher) Option {
	return func(opts *managerOptions) {
		if hooks == nil {
			return
		}
		opts.dispatcherOptions = append(opts.dispatcherOptions, WithDispatcherHooks(hooks))
	}
}

// WithDispatcherOptions appends dispatcher options used when constructing the
// shared dispatcher.
func WithDispatcherOptions(options ...DispatcherOption) Option {
	return func(opts *managerOptions) {
		opts.dispatcherOptions = append(opts.dispatcherOptions, options...)
	}
}

// WithSchedulerOptions appends scheduler options used when constructing the
// runtime scheduler.
func WithSchedulerOptions(options ...SchedulerOption) Option {
	return func(opts *managerOptions) {
		opts.schedulerOptions = append(opts.schedulerOptions, options...)
	}
}

// WithTriggerEngineOptions appends trigger-engine options used when
// constructing the runtime engine.
func WithTriggerEngineOptions(options ...TriggerEngineOption) Option {
	return func(opts *managerOptions) {
		opts.triggerOptions = append(opts.triggerOptions, options...)
	}
}

// WithManagerNow overrides the manager clock used for sync bookkeeping.
func WithManagerNow(now func() time.Time) Option {
	return func(opts *managerOptions) {
		opts.now = now
	}
}

// WithResourceDefinitions switches desired-state automation definitions to the
// shared resource runtime while keeping operational run state on Store.
func WithResourceDefinitions(
	jobStore resources.Store[Job],
	triggerStore resources.Store[Trigger],
	actor resources.MutationActor,
	trigger func(context.Context, resources.ResourceKind, resources.ReconcileReason) error,
) Option {
	return func(opts *managerOptions) {
		opts.jobResources = jobStore
		opts.triggerResources = triggerStore
		opts.resourceActor = actor
		opts.resourceTrigger = trigger
	}
}

// New constructs the composed automation manager.
func New(opts ...Option) (*Manager, error) {
	options := defaultManagerOptions()
	applyManagerOptions(&options, opts)
	if err := finalizeManagerOptions(&options); err != nil {
		return nil, err
	}

	dispatcherOpts := managerDispatcherOptions(&options)
	dispatcher, err := NewDispatcher(options.sessions, options.store, dispatcherOpts...)
	if err != nil {
		return nil, fmt.Errorf("automation: construct dispatcher: %w", err)
	}

	manager := &Manager{
		store:               options.store,
		sessions:            options.sessions,
		tasks:               options.tasks,
		loopStarter:         options.loopStarter,
		workspaceResolver:   options.workspaceResolver,
		config:              options.config,
		logger:              options.logger,
		globalWorkspacePath: options.globalWorkspacePath,
		webhookSecrets:      options.webhookSecrets,
		dispatcher:          dispatcher,
		schedulerOptions:    append([]SchedulerOption(nil), options.schedulerOptions...),
		triggerOptions:      append([]TriggerEngineOption(nil), options.triggerOptions...),
		jobResources:        options.jobResources,
		triggerResources:    options.triggerResources,
		resourceActor:       options.resourceActor,
		resourceTrigger:     options.resourceTrigger,
		now:                 options.now,
		projectedJobs:       make(map[string]Job),
		projectedTriggers:   make(map[string]Trigger),
		sessionTaskActors:   make(map[string]taskpkg.ActorContext),
	}
	if manager.tasks != nil {
		manager.dispatcher.taskActors = manager
	}

	return manager, nil
}
