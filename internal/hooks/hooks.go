package hooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

const (
	hooksConfigKey = "config"
	hooksSkillKey  = "skill"
)

// Option customizes a Hooks dispatcher during construction.
type Option func(*Hooks)

// DeclarationProvider returns the hook declarations for one source class.
type DeclarationProvider func(context.Context) ([]HookDecl, error)

// Hooks owns the hot-reloadable registry snapshot and typed dispatch surface.
type Hooks struct {
	mu       sync.RWMutex
	snapshot map[HookEvent][]*ResolvedHook

	pool *asyncPool

	version     atomic.Int64
	fingerprint string

	logger          *slog.Logger
	now             func() time.Time
	resolveExecutor ExecutorResolver
	telemetrySink   TelemetrySink
	metrics         *hookMetrics
	debugPatchAudit bool

	nativeProvider DeclarationProvider
	configProvider DeclarationProvider
	agentProvider  DeclarationProvider
	skillProvider  DeclarationProvider

	asyncWorkerCount   int
	asyncQueueCapacity int
	asyncDrainTimeout  time.Duration
}

type snapshotFingerprint struct {
	Event HookEvent                 `json:"event"`
	Hooks []resolvedHookFingerprint `json:"hooks"`
}

type resolvedHookFingerprint struct {
	Name         string            `json:"name"`
	Event        HookEvent         `json:"event"`
	Source       HookSource        `json:"source"`
	Mode         HookMode          `json:"mode"`
	Required     bool              `json:"required"`
	Priority     int32             `json:"priority"`
	Timeout      time.Duration     `json:"timeout"`
	Matcher      HookMatcher       `json:"matcher"`
	Metadata     map[string]string `json:"metadata"`
	ExecutorKind HookExecutorKind  `json:"executor_kind"`
	Command      string            `json:"command"`
	Args         []string          `json:"args"`
	Env          map[string]string `json:"env"`
	SecretEnv    map[string]string `json:"secret_env"`
	SkillSource  HookSkillSource   `json:"skill_source"`
}

// WithLogger injects the logger used for hook diagnostics.
func WithLogger(logger *slog.Logger) Option {
	return func(hooks *Hooks) {
		hooks.logger = logger
	}
}

// WithNow injects the clock used by notifier payload construction.
func WithNow(now func() time.Time) Option {
	return func(hooks *Hooks) {
		hooks.now = now
	}
}

// WithExecutorResolver injects the resolver used to bind declarations to
// executors during rebuild.
func WithExecutorResolver(resolve ExecutorResolver) Option {
	return func(hooks *Hooks) {
		hooks.resolveExecutor = resolve
	}
}

// WithTelemetrySink injects the persistence sink used when no active
// session-scoped writer is attached to the dispatch context.
func WithTelemetrySink(sink TelemetrySink) Option {
	return func(hooks *Hooks) {
		hooks.telemetrySink = sink
	}
}

// WithDebugPatchAudit enables patch capture for non-security hook families.
func WithDebugPatchAudit(enabled bool) Option {
	return func(hooks *Hooks) {
		hooks.debugPatchAudit = enabled
	}
}

// WithAsyncWorkerCount configures the async worker pool size.
func WithAsyncWorkerCount(count int) Option {
	return func(hooks *Hooks) {
		hooks.asyncWorkerCount = count
	}
}

// WithAsyncQueueCapacity configures the async worker pool queue depth.
func WithAsyncQueueCapacity(capacity int) Option {
	return func(hooks *Hooks) {
		hooks.asyncQueueCapacity = capacity
	}
}

// WithAsyncDrainTimeout configures the async pool shutdown deadline.
func WithAsyncDrainTimeout(timeout time.Duration) Option {
	return func(hooks *Hooks) {
		hooks.asyncDrainTimeout = timeout
	}
}

// WithNativeDeclarationProvider injects the native-hook declaration source.
func WithNativeDeclarationProvider(provider DeclarationProvider) Option {
	return func(hooks *Hooks) {
		hooks.nativeProvider = provider
	}
}

// WithConfigDeclarationProvider injects the config-hook declaration source.
func WithConfigDeclarationProvider(provider DeclarationProvider) Option {
	return func(hooks *Hooks) {
		hooks.configProvider = provider
	}
}

// WithAgentDeclarationProvider injects the agent-definition declaration source.
func WithAgentDeclarationProvider(provider DeclarationProvider) Option {
	return func(hooks *Hooks) {
		hooks.agentProvider = provider
	}
}

// WithSkillDeclarationProvider injects the skill-hook declaration source.
func WithSkillDeclarationProvider(provider DeclarationProvider) Option {
	return func(hooks *Hooks) {
		hooks.skillProvider = provider
	}
}

// WithNativeDeclarations injects a static native declaration set.
func WithNativeDeclarations(decls []HookDecl) Option {
	return WithNativeDeclarationProvider(staticDeclarationProvider(decls))
}

// WithConfigDeclarations injects a static config declaration set.
func WithConfigDeclarations(decls []HookDecl) Option {
	return WithConfigDeclarationProvider(staticDeclarationProvider(decls))
}

// WithAgentDeclarations injects a static agent-definition declaration set.
func WithAgentDeclarations(decls []HookDecl) Option {
	return WithAgentDeclarationProvider(staticDeclarationProvider(decls))
}

// WithSkillDeclarations injects a static skill declaration set.
func WithSkillDeclarations(decls []HookDecl) Option {
	return WithSkillDeclarationProvider(staticDeclarationProvider(decls))
}

// NewHooks constructs a hook dispatcher with an empty registry snapshot and a
// started async pool.
func NewHooks(opts ...Option) *Hooks {
	hooks := &Hooks{
		snapshot:           make(map[HookEvent][]*ResolvedHook),
		logger:             slog.Default(),
		now:                time.Now,
		resolveExecutor:    defaultExecutorResolver,
		metrics:            newHookMetrics(),
		nativeProvider:     emptyDeclarationProvider,
		configProvider:     emptyDeclarationProvider,
		agentProvider:      emptyDeclarationProvider,
		skillProvider:      emptyDeclarationProvider,
		asyncWorkerCount:   defaultAsyncWorkerCount,
		asyncQueueCapacity: defaultAsyncQueueCapacity,
		asyncDrainTimeout:  defaultAsyncDrainTimeout,
	}

	for _, opt := range opts {
		if opt != nil {
			opt(hooks)
		}
	}

	if hooks.logger == nil {
		hooks.logger = slog.Default()
	}
	if hooks.now == nil {
		hooks.now = time.Now
	}
	if hooks.resolveExecutor == nil {
		hooks.resolveExecutor = defaultExecutorResolver
	}
	if hooks.metrics == nil {
		hooks.metrics = newHookMetrics()
	}
	if hooks.nativeProvider == nil {
		hooks.nativeProvider = emptyDeclarationProvider
	}
	if hooks.configProvider == nil {
		hooks.configProvider = emptyDeclarationProvider
	}
	if hooks.agentProvider == nil {
		hooks.agentProvider = emptyDeclarationProvider
	}
	if hooks.skillProvider == nil {
		hooks.skillProvider = emptyDeclarationProvider
	}

	hooks.pool = newAsyncPool(asyncPoolConfig{
		WorkerCount:   hooks.asyncWorkerCount,
		QueueCapacity: hooks.asyncQueueCapacity,
		DrainTimeout:  hooks.asyncDrainTimeout,
		Logger:        hooks.logger,
		Metrics:       hooks.metrics,
	})
	hooks.pool.Start(context.Background())

	return hooks
}

// Version returns the current registry snapshot version.
func (h *Hooks) Version() int64 {
	if h == nil {
		return 0
	}

	return h.version.Load()
}

// Close drains the async worker pool.
func (h *Hooks) Close() {
	if h == nil || h.pool == nil {
		return
	}

	h.pool.Close()
}

// Rebuild reloads all declaration sources, validates the full snapshot, and
// swaps it atomically when the semantic contents changed.
func (h *Hooks) Rebuild(ctx context.Context) error {
	if h == nil {
		return errors.New("hooks: dispatcher is required")
	}
	if ctx == nil {
		return errors.New("hooks: rebuild context is required")
	}

	decls, err := h.collectDeclarations(ctx)
	if err != nil {
		return err
	}

	state, err := h.BuildBindingState(decls)
	if err != nil {
		return err
	}
	return h.ApplyBindingState(state, 0)
}

func (h *Hooks) collectDeclarations(ctx context.Context) ([]HookDecl, error) {
	collected := make([]HookDecl, 0, 16)

	sources := []struct {
		name     string
		source   HookSource
		provider DeclarationProvider
	}{
		{name: string(HookExecutorNative), source: HookSourceNative, provider: h.nativeProvider},
		{name: hooksConfigKey, source: HookSourceConfig, provider: h.configProvider},
		{name: "agent_definition", source: HookSourceAgentDefinition, provider: h.agentProvider},
		{name: hooksSkillKey, source: HookSourceSkill, provider: h.skillProvider},
	}

	for _, source := range sources {
		provider := source.provider
		if provider == nil {
			continue
		}

		decls, err := provider(ctx)
		if err != nil {
			return nil, fmt.Errorf("hooks: load %s declarations: %w", source.name, err)
		}

		for _, decl := range decls {
			if !decl.EnabledValue() {
				continue
			}
			normalized := cloneHookDecl(decl)
			normalized.Source = source.source
			collected = append(collected, normalized)
		}
	}

	return collected, nil
}

func (h *Hooks) hookSnapshot(event HookEvent) ([]*ResolvedHook, error) {
	if h == nil {
		return nil, errors.New("hooks: dispatcher is required")
	}
	if err := event.Validate(); err != nil {
		return nil, err
	}

	h.mu.RLock()
	snapshot := h.snapshot[event]
	h.mu.RUnlock()

	return snapshot, nil
}

func buildHookSnapshot(resolved []ResolvedHook) map[HookEvent][]*ResolvedHook {
	snapshot := make(map[HookEvent][]*ResolvedHook)
	for idx := range resolved {
		hook := resolved[idx]
		snapshot[hook.Event] = append(snapshot[hook.Event], &hook)
	}

	for _, event := range AllHookEvents() {
		SortResolvedHooks(snapshot[event])
	}

	return snapshot
}

func countResolvedHooks(snapshot map[HookEvent][]*ResolvedHook) int {
	count := 0
	for _, hooks := range snapshot {
		count += len(hooks)
	}
	return count
}

func fingerprintHookSnapshot(snapshot map[HookEvent][]*ResolvedHook) (string, error) {
	fingerprints := make([]snapshotFingerprint, 0, len(AllHookEvents()))
	for _, event := range AllHookEvents() {
		entry := snapshotFingerprint{
			Event: event,
			Hooks: make([]resolvedHookFingerprint, 0, len(snapshot[event])),
		}

		for _, hook := range snapshot[event] {
			if hook == nil {
				continue
			}

			entry.Hooks = append(entry.Hooks, resolvedHookFingerprint{
				Name:         hook.Name,
				Event:        hook.Event,
				Source:       hook.Source,
				Mode:         hook.Mode,
				Required:     hook.Required,
				Priority:     hook.Priority,
				Timeout:      hook.Timeout,
				Matcher:      hook.Matcher,
				Metadata:     cloneStringMap(hook.Metadata),
				ExecutorKind: hook.Decl.ExecutorKind,
				Command:      hook.Decl.Command,
				Args:         append([]string(nil), hook.Decl.Args...),
				Env:          cloneStringMap(hook.Decl.Env),
				SecretEnv:    cloneStringMap(hook.Decl.SecretEnv),
				SkillSource:  hook.Decl.SkillSource,
			})
		}

		fingerprints = append(fingerprints, entry)
	}

	encoded, err := json.Marshal(fingerprints)
	if err != nil {
		return "", fmt.Errorf("hooks: fingerprint snapshot: %w", err)
	}

	return string(encoded), nil
}

func defaultExecutorResolver(decl HookDecl) (Executor, error) {
	switch decl.ExecutorKind {
	case HookExecutorNative:
		return nil, fmt.Errorf("hooks: native executor for hook %q requires an explicit resolver", decl.Name)
	case HookExecutorSubprocess:
		return NewSubprocessExecutor(
			decl.Command,
			decl.Args,
			WithSubprocessDir(decl.WorkingDir),
			WithSubprocessEnv(decl.Env),
			WithSubprocessSecretEnv(decl.SecretEnv, nil),
		), nil
	case HookExecutorWASM:
		return &WasmExecutor{}, nil
	default:
		return nil, fmt.Errorf("hooks: unsupported executor kind %q for hook %q", decl.ExecutorKind, decl.Name)
	}
}

func emptyDeclarationProvider(context.Context) ([]HookDecl, error) {
	return nil, nil
}

func staticDeclarationProvider(decls []HookDecl) DeclarationProvider {
	cloned := cloneHookDecls(decls)
	return func(context.Context) ([]HookDecl, error) {
		return cloneHookDecls(cloned), nil
	}
}

func cloneHookDecls(decls []HookDecl) []HookDecl {
	if len(decls) == 0 {
		return nil
	}

	cloned := make([]HookDecl, 0, len(decls))
	for _, decl := range decls {
		cloned = append(cloned, cloneHookDecl(decl))
	}
	return cloned
}

func cloneHookDecl(decl HookDecl) HookDecl {
	cloned := decl
	cloned.Args = append([]string(nil), decl.Args...)
	cloned.Env = cloneStringMap(decl.Env)
	cloned.SecretEnv = cloneStringMap(decl.SecretEnv)
	cloned.Metadata = cloneStringMap(decl.Metadata)
	cloned.Enabled = cloneBoolPtr(decl.Enabled)
	cloned.Matcher = cloneHookMatcher(decl.Matcher)
	return cloned
}
