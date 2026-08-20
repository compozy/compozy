package extensionpkg

import (
	"context"

	"errors"

	"log/slog"

	"os"

	"sync"
	"time"

	automationpkg "github.com/compozy/compozy/internal/automation"
	bridgepkg "github.com/compozy/compozy/internal/bridges"
	compozyconfig "github.com/compozy/compozy/internal/config"

	hookspkg "github.com/compozy/compozy/internal/hooks"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/resources"
	skillspkg "github.com/compozy/compozy/internal/skills"
	"github.com/compozy/compozy/internal/subprocess"
	"github.com/compozy/compozy/internal/toolruntime"
	"github.com/compozy/compozy/internal/windowmanager"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

const (
	managerShutdownKey = "shutdown"
)

const (
	managerPathValue      = "PATH"
	managerExtensionKey   = "extension"
	managerHealthCheckKey = "health_check"
)

const (
	defaultProtocolVersion     = "1"
	defaultHealthCheckInterval = 30 * time.Second
	defaultHealthCheckTimeout  = 5 * time.Second
	defaultInitializeTimeout   = 15 * time.Second
	defaultHookTimeout         = 5 * time.Second
	defaultViewTimeout         = 3 * time.Second
	defaultShutdownTimeout     = 10 * time.Second
	defaultRestartBackoffMax   = 60 * time.Second
	// DefaultRestartFailureThreshold is the production crash-loop cutoff shared by status and doctor projections.
	DefaultRestartFailureThreshold = 5
	defaultHealthPollFloor         = 50 * time.Millisecond
	defaultHealthPollCeiling       = time.Second
	defaultSubprocessSignalGrace   = 10 * time.Second
	manifestFileExtTOML            = ".toml"
	manifestFileExtJSON            = ".json"
)

var (
	// ErrContextRequired reports that a manager operation requires a non-nil context.
	ErrContextRequired = errors.New("extension: context is required")
	// ErrManagerRequired reports that a manager-backed operation was invoked on a nil manager.
	ErrManagerRequired = errors.New("extension: manager is required")
	// ErrRegistryRequired reports that a manager operation requires a configured registry.
	ErrRegistryRequired = errors.New("extension: registry is required")
	// ErrBridgeRuntimeResolverRequired reports that a bridge-capable extension cannot start
	// without a bridge runtime resolver.
	ErrBridgeRuntimeResolverRequired = errors.New("extension: bridge runtime resolver is required")
	// ErrPathEscapesExtensionRoot reports that a requested resource path resolves outside the
	// extension root.
	ErrPathEscapesExtensionRoot = errors.New("extension: path escapes extension root")
	// ErrBridgeRuntimeDeferred reports that a bridge-capable extension is
	// installed and registered, but no enabled bridge instance exists yet for
	// the runtime launch handshake.
	ErrBridgeRuntimeDeferred = errors.New("extension: bridge runtime deferred")
)

var safeSubprocessEnvKeys = []string{
	managerPathValue,
	"HOME",
	"USER",
	"LOGNAME",
	"TMPDIR",
	"TMP",
	"TEMP",
	"LANG",
	"LC_ALL",
	"SHELL",
	"SystemRoot",
	"ComSpec",
	"PATHEXT",
	"USERPROFILE",
}

// Option customizes an extension manager.
type Option func(*Manager)

type processHandle interface {
	HandleMethod(string, subprocess.HandlerFunc) error
	Call(context.Context, string, any, any) error
	Initialize(context.Context, subprocess.InitializeRequest) (subprocess.InitializeResponse, error)
	Shutdown(context.Context) error
	Done() <-chan struct{}
	Wait() error
	HealthState() subprocess.HealthState
	PID() int
}

type processLauncher func(context.Context, subprocess.LaunchConfig) (processHandle, error)

// BridgeRuntimeResolver resolves one provider-scoped bridge launch payload
// for a bridge-capable extension session.
type BridgeRuntimeResolver interface {
	ResolveBridgeRuntime(ctx context.Context, extensionName string) (*subprocess.InitializeBridgeRuntime, error)
}

// BridgeTelemetrySink records live bridge runtime/auth telemetry for
// per-instance observability surfaces.
type BridgeTelemetrySink interface {
	RecordBridgeAuthFailure(bridgeInstanceID string)
	RecordBridgeRuntimeIssue(bridgeInstanceID string, status bridgepkg.BridgeStatus, message string)
	ClearBridgeRuntimeIssue(bridgeInstanceID string)
}

// ExtensionPhase names one lifecycle phase or supervisor state for an extension.
type ExtensionPhase string

const (
	ExtensionPhaseDiscover   ExtensionPhase = "discover"
	ExtensionPhaseParse      ExtensionPhase = "parse"
	ExtensionPhaseValidate   ExtensionPhase = "validate"
	ExtensionPhaseRegister   ExtensionPhase = "register"
	ExtensionPhaseInitialize ExtensionPhase = "initialize"
	ExtensionPhaseActivate   ExtensionPhase = "activate"
	ExtensionPhaseRecover    ExtensionPhase = "recover"
	ExtensionPhaseStop       ExtensionPhase = "stop"
)

// ExtensionStatus captures the runtime state exposed to health/observer code.
type ExtensionStatus struct {
	Name                string
	WorkspaceID         string
	Version             string
	Source              ExtensionSource
	Enabled             bool
	MissingEnv          []string
	MissingEnvChecked   bool
	Registered          bool
	Active              bool
	Phase               ExtensionPhase
	PID                 int
	Healthy             bool
	HealthMessage       string
	HealthLastCheckedAt time.Time
	ConsecutiveFailures int
	RestartBackoff      time.Duration
	LastError           string
	FailureCode         string
	GenerationHash      string
	LastGoodGeneration  string
	LastStartedAt       time.Time
	LastExitedAt        time.Time
}

// Extension is the manager-visible snapshot for one installed extension.
type Extension struct {
	Info                  ExtensionInfo
	Manifest              *Manifest
	RootDir               string
	Hooks                 []hookspkg.HookDecl
	Agents                []compozyconfig.AgentDef
	StaticAgents          []StaticAgent
	Skills                []*skillspkg.Skill
	Loops                 []looppkg.ResourceSpec
	AutomationJobs        []automationpkg.Job
	AutomationTriggers    []automationpkg.Trigger
	Layouts               []windowmanager.LayoutResource
	GrantedPermissions    []string
	GrantedSecurity       []string
	GrantedResourceKinds  []resources.ResourceKind
	GrantedResourceScopes []resources.ResourceScopeKind
	InitializeResult      *subprocess.InitializeResponse
	Status                ExtensionStatus
	DevLink               *DevLink
	OverridesPublished    bool
}

type managedExtension struct {
	key                   InstanceKey
	info                  ExtensionInfo
	rootDir               string
	manifest              *Manifest
	hooks                 []hookspkg.HookDecl
	agents                []compozyconfig.AgentDef
	staticAgents          []StaticAgent
	skills                []*skillspkg.Skill
	loops                 []looppkg.ResourceSpec
	automationJobs        []automationpkg.Job
	automationTriggers    []automationpkg.Trigger
	layouts               []windowmanager.LayoutResource
	grantedPermissions    []string
	grantedSecurity       []string
	grantedResourceKinds  []resources.ResourceKind
	grantedResourceScopes []resources.ResourceScopeKind
	pendingGrant          EffectiveGrant
	capabilityGrantID     string
	initialize            *subprocess.InitializeResponse
	process               processHandle
	runtime               subprocess.InitializeRuntime
	healthInterval        time.Duration
	generation            int64
	generationHash        string
	lastGoodGeneration    string
	sessionNonce          string
	redactionCleanups     []func()

	phase               ExtensionPhase
	registered          bool
	active              bool
	awaitingStability   bool
	consecutiveFailures int
	restartBackoff      time.Duration
	lastError           string
	failureCode         string
	lastStartedAt       time.Time
	lastExitedAt        time.Time
	logRing             *ExtensionLogRing
	deferSupervision    bool
	supervisionStopped  bool
	startup             *preparedExtensionStartup
}

type launchedRuntime struct {
	process           processHandle
	response          subprocess.InitializeResponse
	runtime           subprocess.InitializeRuntime
	healthInterval    time.Duration
	sessionNonce      string
	capabilityGrantID string
	redactionCleanups []func()
}

var _ bridgepkg.DeliveryTransport = (*Manager)(nil)
var _ bridgepkg.TargetSnapshotTransport = (*Manager)(nil)

// Manager orchestrates extension loading, subprocess lifecycle, and resource registration.
type Manager struct {
	lifecycleMu sync.Mutex
	mu          sync.RWMutex

	registry              *Registry
	capChecker            *CapabilityChecker
	bridgeRuntimeResolver BridgeRuntimeResolver
	bridgeTelemetrySink   BridgeTelemetrySink
	lifecycleEventSink    LifecycleEventSink
	sourceSessions        resources.SourceSessionManager
	workspaceResolver     workspacepkg.RuntimeResolver
	homePaths             compozyconfig.HomePaths
	processRegistry       *toolruntime.Registry
	logger                *slog.Logger
	now                   func() time.Time
	getenv                func(string) string
	compozyExecutable     func() (string, error)
	secretResolver        SecretRefResolver
	envBindings           EnvBindingStore
	launch                processLauncher
	toolCallTracker       ExtensionToolCallTracker
	hostMethods           map[string]subprocess.HandlerFunc

	protocolVersion           string
	supportedProtocolVersions []string
	initializeTimeout         time.Duration
	healthCheckTimeout        time.Duration
	defaultHookTimeout        time.Duration
	defaultViewTimeout        time.Duration
	defaultShutdownTimeout    time.Duration
	restartBackoffMax         time.Duration
	restartFailureThreshold   int
	healthPollFloor           time.Duration
	healthPollCeiling         time.Duration
	subprocessSignalGrace     time.Duration

	lifecycleCtx context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	started      bool
	stopping     bool

	devOperations     int
	devOperationsDone chan struct{}
	viewCallGates     viewCallGateRegistry

	extensions      map[string]*managedExtension
	devExtensions   map[InstanceKey]*managedExtension
	devCoordinators map[InstanceKey]*sync.Mutex
	devLogs         map[InstanceKey]*ExtensionLogRing
	pendingCleanups []pendingExtensionCleanup
}

// NewManager constructs an extension manager with sensible defaults.
func NewManager(registry *Registry, opts ...Option) *Manager {
	manager := newManagerDefaults(registry)

	for _, opt := range opts {
		if opt != nil {
			opt(manager)
		}
	}

	normalizeManagerDefaults(manager)
	return manager
}

func newManagerDefaults(registry *Registry) *Manager {
	manager := &Manager{
		registry:                  registry,
		capChecker:                &CapabilityChecker{},
		logger:                    slog.Default(),
		now:                       func() time.Time { return time.Now().UTC() },
		getenv:                    os.Getenv,
		compozyExecutable:         os.Executable,
		hostMethods:               make(map[string]subprocess.HandlerFunc),
		protocolVersion:           defaultProtocolVersion,
		supportedProtocolVersions: []string{defaultProtocolVersion},
		initializeTimeout:         defaultInitializeTimeout,
		healthCheckTimeout:        defaultHealthCheckTimeout,
		defaultHookTimeout:        defaultHookTimeout,
		defaultViewTimeout:        defaultViewTimeout,
		defaultShutdownTimeout:    defaultShutdownTimeout,
		restartBackoffMax:         defaultRestartBackoffMax,
		restartFailureThreshold:   DefaultRestartFailureThreshold,
		healthPollFloor:           defaultHealthPollFloor,
		healthPollCeiling:         defaultHealthPollCeiling,
		subprocessSignalGrace:     defaultSubprocessSignalGrace,
		extensions:                make(map[string]*managedExtension),
		devExtensions:             make(map[InstanceKey]*managedExtension),
		devCoordinators:           make(map[InstanceKey]*sync.Mutex),
		devLogs:                   make(map[InstanceKey]*ExtensionLogRing),
	}
	manager.launch = func(ctx context.Context, cfg subprocess.LaunchConfig) (processHandle, error) {
		return subprocess.Launch(ctx, cfg)
	}
	return manager
}

func normalizeManagerDefaults(manager *Manager) {
	if manager == nil {
		return
	}
	if manager.capChecker == nil {
		manager.capChecker = &CapabilityChecker{}
	}
	if manager.logger == nil {
		manager.logger = slog.Default()
	}
	if manager.now == nil {
		manager.now = func() time.Time { return time.Now().UTC() }
	}
	if manager.getenv == nil {
		manager.getenv = os.Getenv
	}
	if manager.compozyExecutable == nil {
		manager.compozyExecutable = os.Executable
	}
	if manager.launch == nil {
		manager.launch = func(ctx context.Context, cfg subprocess.LaunchConfig) (processHandle, error) {
			return subprocess.Launch(ctx, cfg)
		}
	}
	if len(manager.supportedProtocolVersions) == 0 {
		manager.supportedProtocolVersions = []string{defaultProtocolVersion}
	}
	if manager.protocolVersion == "" {
		manager.protocolVersion = manager.supportedProtocolVersions[0]
	}
	if manager.initializeTimeout <= 0 {
		manager.initializeTimeout = defaultInitializeTimeout
	}
	if manager.healthCheckTimeout <= 0 {
		manager.healthCheckTimeout = defaultHealthCheckTimeout
	}
	if manager.defaultHookTimeout <= 0 {
		manager.defaultHookTimeout = defaultHookTimeout
	}
	if manager.defaultViewTimeout <= 0 {
		manager.defaultViewTimeout = defaultViewTimeout
	}
	if manager.defaultShutdownTimeout <= 0 {
		manager.defaultShutdownTimeout = defaultShutdownTimeout
	}
	if manager.restartBackoffMax <= 0 {
		manager.restartBackoffMax = defaultRestartBackoffMax
	}
	if manager.restartFailureThreshold <= 0 {
		manager.restartFailureThreshold = DefaultRestartFailureThreshold
	}
	if manager.healthPollFloor <= 0 {
		manager.healthPollFloor = defaultHealthPollFloor
	}
	if manager.healthPollCeiling <= 0 {
		manager.healthPollCeiling = defaultHealthPollCeiling
	}
	if manager.subprocessSignalGrace <= 0 {
		manager.subprocessSignalGrace = defaultSubprocessSignalGrace
	}
}
