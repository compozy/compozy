package daemon

import (
	"context"

	"fmt"
	"log/slog"
	"os"
	"strings"

	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/gateway"

	"github.com/compozy/compozy/internal/memory"
	"github.com/compozy/compozy/internal/memory/consolidation"
	"github.com/compozy/compozy/internal/procutil"
	"github.com/compozy/compozy/internal/providers"

	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/store/sessiondb"
)

// WithHomePaths overrides the resolved Compozy home layout.
func WithHomePaths(homePaths compozyconfig.HomePaths) Option {
	return func(d *Daemon) {
		d.homePaths = homePaths
	}
}

// WithConfig overrides daemon-level configuration loading.
func WithConfig(cfg *compozyconfig.Config) Option {
	return func(d *Daemon) {
		if cfg == nil {
			return
		}
		cfgCopy := *cfg
		d.loadConfig = func() (compozyconfig.Config, error) {
			return cfgCopy, nil
		}
	}
}

// WithConfigLoader overrides daemon-level configuration loading.
func WithConfigLoader(loader ConfigLoader) Option {
	return func(d *Daemon) {
		d.loadConfig = loader
	}
}

// WithLogger injects the daemon logger.
func WithLogger(logger *slog.Logger) Option {
	return func(d *Daemon) {
		d.logger = logger
		d.closeLogger = func() error { return nil }
	}
}

// WithBridgeSecretResolver injects the daemon-owned resolver used to convert
// bridge secret bindings into launch-time bound secret material. When this
// option is not supplied, daemon boot wires the canonical vault-backed resolver.
func WithBridgeSecretResolver(resolver BridgeSecretResolver) Option {
	return func(d *Daemon) {
		d.bridgeSecretResolver = resolver
		d.bridgeSecretResolverExplicit = true
	}
}

// WithNow overrides the daemon clock, mainly for tests.
func WithNow(now func() time.Time) Option {
	return func(d *Daemon) {
		d.now = now
	}
}

// WithHTTPServerFactory overrides HTTP server construction.
func WithHTTPServerFactory(factory ServerFactory) Option {
	return func(d *Daemon) {
		d.httpFactory = factory
	}
}

// WithUDSServerFactory overrides UDS server construction.
func WithUDSServerFactory(factory ServerFactory) Option {
	return func(d *Daemon) {
		d.udsFactory = factory
	}
}

// WithGatewayTierServerFactory overrides daemon-owned tier listener construction.
func WithGatewayTierServerFactory(factory GatewayTierServerFactory) Option {
	return func(d *Daemon) {
		d.gatewayTierFactory = factory
	}
}

// WithGatewayProviderEffects injects the provider preflight, establish, verify, advertise,
// withdraw, teardown, and health-supervision lifecycle used by gateway reconciliation.
func WithGatewayProviderEffects(effects gateway.EffectDriver) Option {
	return func(d *Daemon) {
		d.gatewayProviderEffects = effects
	}
}

// WithSignalBridge overrides OS signal delivery, mainly for tests.
func WithSignalBridge(ch <-chan os.Signal) Option {
	return func(d *Daemon) {
		d.signalCh = ch
	}
}

// WithBoundaryVerification enables best-effort import boundary verification on boot.
func WithBoundaryVerification(enabled bool) Option {
	return func(d *Daemon) {
		d.verifyBoundaries = enabled
	}
}

// New constructs the daemon composition root.
func New(opts ...Option) (*Daemon, error) {
	homePaths, err := compozyconfig.ResolveHomePaths()
	if err != nil {
		return nil, fmt.Errorf("daemon: resolve home paths: %w", err)
	}

	d := &Daemon{
		homePaths: homePaths,
		readyCh:   make(chan struct{}),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(d)
		}
	}

	d.applyDefaults()

	return d, nil
}

func (d *Daemon) applyDefaults() {
	d.applyCoreDefaults()
	d.applyRuntimeFactoryDefaults()
	d.applyServerFactoryDefaults()
	d.applySystemDefaults()
}

func (d *Daemon) applyCoreDefaults() {
	if d.providerPreStarter == nil {
		d.providerPreStarter = providers.NewPreStarter()
	}
	if d.now == nil {
		d.now = func() time.Time {
			return time.Now().UTC()
		}
	}
	if d.pid == nil {
		d.pid = os.Getpid
	}
	if d.processStartedAt == nil {
		d.processStartedAt = procutil.StartedAt
	}
	if d.acquireLock == nil {
		d.acquireLock = AcquireLock
	}
	if d.openRegistry == nil {
		d.openRegistry = func(ctx context.Context, path string) (Registry, error) {
			operatorHome, err := compozyconfig.ResolveOperatorHomeDirWithLookup(
				d.homePaths,
				func(key string) (string, bool) {
					value := d.getenv(key)
					return value, strings.TrimSpace(value) != ""
				},
			)
			if err != nil {
				return nil, fmt.Errorf("daemon: resolve operator home for global database: %w", err)
			}
			return globaldb.OpenGlobalDB(
				ctx,
				path,
				globaldb.WithOperatorHomeDir(operatorHome),
				globaldb.WithSessionEventMetadataOpener(
					sessiondb.NewEventMetadataOpener(d.homePaths.SessionsDir),
				),
			)
		}
	}
}

func (d *Daemon) applyRuntimeFactoryDefaults() {
	d.applySessionManagerFactoryDefault()
	if d.newDreamService == nil {
		d.newDreamService = func(opts ...memory.Option) consolidation.Service {
			return memory.NewService(opts...)
		}
	}
	d.applyObserverFactoryDefault()
	d.applyExtensionManagerFactoryDefault()
	d.applyAutomationManagerFactoryDefault()
	d.applyResourceReconcileDriverFactoryDefault()
}
