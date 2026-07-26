package daemon

import (
	"context"

	"fmt"
	"log/slog"
	"os"

	"time"

	aghconfig "github.com/compozy/agh/internal/config"

	"github.com/compozy/agh/internal/memory"
	"github.com/compozy/agh/internal/memory/consolidation"

	"github.com/compozy/agh/internal/store/globaldb"
	"github.com/compozy/agh/internal/store/sessiondb"
)

// WithHomePaths overrides the resolved AGH home layout.
func WithHomePaths(homePaths aghconfig.HomePaths) Option {
	return func(d *Daemon) {
		d.homePaths = homePaths
	}
}

// WithConfig overrides daemon-level configuration loading.
func WithConfig(cfg *aghconfig.Config) Option {
	return func(d *Daemon) {
		if cfg == nil {
			return
		}
		cfgCopy := *cfg
		d.loadConfig = func() (aghconfig.Config, error) {
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
	homePaths, err := aghconfig.ResolveHomePaths()
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
	d.applyTimingDefaults()
}

func (d *Daemon) applyCoreDefaults() {
	if d.now == nil {
		d.now = func() time.Time {
			return time.Now().UTC()
		}
	}
	if d.pid == nil {
		d.pid = os.Getpid
	}
	if d.acquireLock == nil {
		d.acquireLock = AcquireLock
	}
	if d.openRegistry == nil {
		d.openRegistry = func(ctx context.Context, path string) (Registry, error) {
			return globaldb.OpenGlobalDB(
				ctx,
				path,
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
