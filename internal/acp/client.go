package acp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"time"

	acpsdk "github.com/coder/acp-go-sdk"

	"github.com/compozy/agh/internal/sandbox"

	"github.com/compozy/agh/internal/toolruntime"
)

const (
	clientAgentWaitingKey = "agent_waiting"
	clientDefaultKey      = "default"
)

const (
	defaultStopTimeout          = 5 * time.Second
	defaultPromptBufSize        = 128
	defaultPromptDrain          = 50 * time.Millisecond
	defaultPermissionWait       = 5 * time.Minute
	defaultProcessRecordTimeout = time.Second
	defaultClientName           = "agh"
	defaultClientVersion        = "dev"
)

var (
	// ErrAgentDoesNotSupportSession reports that resume was requested for an ACP agent without session/load support.
	ErrAgentDoesNotSupportSession = errors.New("acp: agent does not support session/load")
	// ErrLoadSessionFailed reports that ACP session/load failed during resume.
	ErrLoadSessionFailed = errors.New("acp: load session failed")
	// errModelConfigOptionRequired reports that a requested model needs an advertised ACP model option.
	errModelConfigOptionRequired = errors.New("acp: model config option required")
	// errProcessConnectionUninitialized reports that the driver received a process without an ACP connection.
	errProcessConnectionUninitialized = errors.New("acp: process connection is not initialized")
	// errProcessLifecycleUninitialized reports that the driver received a process without a managed lifecycle.
	errProcessLifecycleUninitialized = errors.New("acp: process lifecycle is not initialized")
)

const requestErrorResourceNotFoundCode = -32002

// Option customizes the ACP driver.
type Option func(*Driver)

// Driver launches ACP agent subprocesses and brokers JSON-RPC traffic.
type Driver struct {
	logger               *slog.Logger
	stopTimeout          time.Duration
	promptBufferCap      int
	promptDrainWait      time.Duration
	permissionWait       time.Duration
	processRecordTimeout time.Duration
	launcher             sandbox.Launcher
	toolHost             sandbox.ToolHost
	processRegistry      *toolruntime.Registry
	steerSource          SteerSource
}

// WithLogger directs driver diagnostics to the provided logger.
func WithLogger(logger *slog.Logger) Option {
	return func(driver *Driver) {
		driver.logger = logger
	}
}

// WithStopTimeout overrides how long Stop waits before escalating to SIGKILL.
func WithStopTimeout(timeout time.Duration) Option {
	return func(driver *Driver) {
		driver.stopTimeout = timeout
	}
}

// WithPromptBufferSize overrides the per-prompt event buffer size.
func WithPromptBufferSize(size int) Option {
	return func(driver *Driver) {
		driver.promptBufferCap = size
	}
}

// WithPromptDrainWait overrides how long Prompt waits for trailing asynchronous updates.
func WithPromptDrainWait(wait time.Duration) Option {
	return func(driver *Driver) {
		driver.promptDrainWait = wait
	}
}

// WithPermissionTimeout overrides how long an interactive permission request waits for approval.
func WithPermissionTimeout(timeout time.Duration) Option {
	return func(driver *Driver) {
		driver.permissionWait = timeout
	}
}

// WithLauncher overrides the sandbox launcher used by default for new ACP sessions.
func WithLauncher(launcher sandbox.Launcher) Option {
	return func(driver *Driver) {
		driver.launcher = launcher
	}
}

// WithToolHost overrides the sandbox tool host used by default for new ACP sessions.
func WithToolHost(toolHost sandbox.ToolHost) Option {
	return func(driver *Driver) {
		driver.toolHost = toolHost
	}
}

// WithProcessRegistry injects shared tool process tracking and scoped interrupts.
func WithProcessRegistry(registry *toolruntime.Registry) Option {
	return func(driver *Driver) {
		driver.processRegistry = registry
	}
}

// WithProcessRecordTimeout bounds process registry writes for ACP subprocesses.
func WithProcessRecordTimeout(timeout time.Duration) Option {
	return func(driver *Driver) {
		driver.processRecordTimeout = timeout
	}
}

// WithSteerSource injects the staged busy-input source consumed at tool-result boundaries.
func WithSteerSource(source SteerSource) Option {
	return func(driver *Driver) {
		driver.steerSource = source
	}
}

// New constructs an ACP driver with sensible defaults.
func New(opts ...Option) *Driver {
	driver := &Driver{
		logger:               slog.Default(),
		stopTimeout:          defaultStopTimeout,
		promptBufferCap:      defaultPromptBufSize,
		promptDrainWait:      defaultPromptDrain,
		permissionWait:       defaultPermissionWait,
		processRecordTimeout: defaultProcessRecordTimeout,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(driver)
		}
	}
	if driver.logger == nil {
		driver.logger = slog.Default()
	}
	if driver.stopTimeout <= 0 {
		driver.stopTimeout = defaultStopTimeout
	}
	if driver.promptBufferCap <= 0 {
		driver.promptBufferCap = defaultPromptBufSize
	}
	if driver.promptDrainWait <= 0 {
		driver.promptDrainWait = defaultPromptDrain
	}
	if driver.permissionWait <= 0 {
		driver.permissionWait = defaultPermissionWait
	}
	if driver.processRecordTimeout <= 0 {
		driver.processRecordTimeout = defaultProcessRecordTimeout
	}
	if driver.launcher == nil {
		driver.launcher = newLocalLauncher(driver.logger, driver.stopTimeout)
	}
	return driver
}

func (d *Driver) cleanupFailedStart(process *AgentProcess, startErr error) error {
	if startErr == nil || process == nil {
		return startErr
	}
	if stopErr := d.Stop(context.Background(), process); stopErr != nil {
		return errors.Join(startErr, fmt.Errorf("acp: stop failed while cleaning up failed start: %w", stopErr))
	}
	return startErr
}

// IsLoadSessionResourceMissing reports whether a resume failed because the
// upstream ACP implementation no longer knows the referenced session id.
func IsLoadSessionResourceMissing(err error) bool {
	if !errors.Is(err, ErrLoadSessionFailed) {
		return false
	}

	var reqErr *acpsdk.RequestError
	if !errors.As(err, &reqErr) {
		return false
	}

	return reqErr.Code == requestErrorResourceNotFoundCode && requestErrorIndicatesSessionLoss(reqErr)
}
