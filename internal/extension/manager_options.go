package extensionpkg

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/subprocess"
	"github.com/compozy/agh/internal/toolruntime"
)

// SecretRefResolver resolves env: and vault: refs for extension launch bindings.
type SecretRefResolver interface {
	ResolveRef(context.Context, string) (string, error)
}

// WithCapabilityChecker injects the grant evaluator used for Host API authorization.
func WithCapabilityChecker(checker *CapabilityChecker) Option {
	return func(manager *Manager) {
		manager.capChecker = checker
	}
}

// WithBridgeRuntimeResolver injects the bridge launch material resolver used
// for bridge-capable extension sessions.
func WithBridgeRuntimeResolver(resolver BridgeRuntimeResolver) Option {
	return func(manager *Manager) {
		manager.bridgeRuntimeResolver = resolver
	}
}

// WithBridgeTelemetrySink injects the sink used to publish per-instance
// runtime degradation/error signals into observability surfaces.
func WithBridgeTelemetrySink(sink BridgeTelemetrySink) Option {
	return func(manager *Manager) {
		manager.bridgeTelemetrySink = sink
	}
}

// WithSourceSessionManager injects the resource source-session manager used to
// activate extension nonces for snapshot publication.
func WithSourceSessionManager(manager resources.SourceSessionManager) Option {
	return func(mgr *Manager) {
		mgr.sourceSessions = manager
	}
}

// WithProcessRegistry injects shared tool process ownership tracking.
func WithProcessRegistry(registry *toolruntime.Registry) Option {
	return func(manager *Manager) {
		manager.processRegistry = registry
	}
}

// WithLogger injects the logger used for extension diagnostics.
func WithLogger(logger *slog.Logger) Option {
	return func(manager *Manager) {
		manager.logger = logger
	}
}

// WithNow overrides the manager clock, mainly for tests.
func WithNow(now func() time.Time) Option {
	return func(manager *Manager) {
		manager.now = now
	}
}

// WithGetenv overrides environment lookup used for manifest template expansion.
func WithGetenv(getenv func(string) string) Option {
	return func(manager *Manager) {
		manager.getenv = getenv
	}
}

// WithAGHExecutableResolver overrides the binary used by {{agh_executable}} manifest templates.
func WithAGHExecutableResolver(resolver func() (string, error)) Option {
	return func(manager *Manager) {
		manager.aghExecutable = resolver
	}
}

// WithSecretResolver injects the daemon vault resolver used for extension secret env bindings.
func WithSecretResolver(resolver SecretRefResolver) Option {
	return func(manager *Manager) {
		manager.secretResolver = resolver
	}
}

// WithHostMethodHandler registers one Host API method handler for launched extensions.
func WithHostMethodHandler(method string, handler subprocess.HandlerFunc) Option {
	return func(manager *Manager) {
		if manager.hostMethods == nil {
			manager.hostMethods = make(map[string]subprocess.HandlerFunc)
		}
		manager.hostMethods[strings.TrimSpace(method)] = handler
	}
}

// WithInitializeTimeout overrides the initialize handshake timeout.
func WithInitializeTimeout(timeout time.Duration) Option {
	return func(manager *Manager) {
		manager.initializeTimeout = timeout
	}
}

// WithHealthCheckTimeout overrides the negotiated health probe timeout.
func WithHealthCheckTimeout(timeout time.Duration) Option {
	return func(manager *Manager) {
		manager.healthCheckTimeout = timeout
	}
}

// WithDefaultHookTimeout overrides the negotiated default hook timeout.
func WithDefaultHookTimeout(timeout time.Duration) Option {
	return func(manager *Manager) {
		manager.defaultHookTimeout = timeout
	}
}

// WithSubprocessSignalGrace overrides the SIGTERM -> SIGKILL grace interval.
func WithSubprocessSignalGrace(timeout time.Duration) Option {
	return func(manager *Manager) {
		manager.subprocessSignalGrace = timeout
	}
}

func withProcessLauncher(launcher processLauncher) Option {
	return func(manager *Manager) {
		manager.launch = launcher
	}
}

func withRestartBackoffMax(maximum time.Duration) Option {
	return func(manager *Manager) {
		manager.restartBackoffMax = maximum
	}
}

func withRestartFailureThreshold(threshold int) Option {
	return func(manager *Manager) {
		manager.restartFailureThreshold = threshold
	}
}

func withHealthPollBounds(floor, ceiling time.Duration) Option {
	return func(manager *Manager) {
		manager.healthPollFloor = floor
		manager.healthPollCeiling = ceiling
	}
}
