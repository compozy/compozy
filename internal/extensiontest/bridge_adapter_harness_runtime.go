package extensiontest

import (
	"context"
	"encoding/json"
	"errors"

	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	bridgepkg "github.com/compozy/agh/internal/bridges"

	aghconfig "github.com/compozy/agh/internal/config"
	extensionpkg "github.com/compozy/agh/internal/extension"

	observepkg "github.com/compozy/agh/internal/observe"
	sandboxlocal "github.com/compozy/agh/internal/sandbox/local"
	"github.com/compozy/agh/internal/session"
	skillspkg "github.com/compozy/agh/internal/skills"
	"github.com/compozy/agh/internal/store/globaldb"
	"github.com/compozy/agh/internal/store/sessiondb"
	"github.com/compozy/agh/internal/subprocess"
	aghtestutil "github.com/compozy/agh/internal/testutil"

	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func buildHarnessRuntime(
	t testing.TB,
	cfg HarnessConfig,
	homePaths aghconfig.HomePaths,
	globalDB *globaldb.GlobalDB,
	workspaces workspacepkg.RuntimeResolver,
	bridgeRegistry *bridgepkg.Service,
	extensionRegistry *extensionpkg.Registry,
	manifest *extensionpkg.Manifest,
	extensionName string,
	instances []bridgepkg.BridgeInstance,
	managedRuntime []subprocess.InitializeBridgeManagedInstance,
	now time.Time,
	markers MarkerPaths,
) (*extensionpkg.HostAPIHandler, *extensionpkg.Manager, *bridgepkg.Broker, *observepkg.Observer, *session.Manager) {
	t.Helper()

	checker := &extensionpkg.CapabilityChecker{}
	checker.Register(extensionName, extensionpkg.SourceUser, manifest)

	var hostHandler *extensionpkg.HostAPIHandler
	telemetrySink := &deferredBridgeTelemetrySink{}
	hostForwarder := newHarnessHostForwarder(t, markers, func() *extensionpkg.HostAPIHandler {
		return hostHandler
	})

	manager := extensionpkg.NewManager(
		extensionRegistry,
		extensionpkg.WithCapabilityChecker(checker),
		extensionpkg.WithBridgeRuntimeResolver(&stubBridgeRuntimeResolver{
			runtimes: map[string]*subprocess.InitializeBridgeRuntime{
				extensionName: {
					RuntimeVersion:   subprocess.InitializeBridgeRuntimeVersion2,
					Purpose:          subprocess.BridgeRuntimePurposeService,
					Provider:         extensionName,
					Platform:         instances[0].Platform,
					ManagedInstances: cloneManagedRuntime(managedRuntime),
				},
			},
		}),
		extensionpkg.WithBridgeTelemetrySink(telemetrySink),
		extensionpkg.WithHostMethodHandler("bridges/instances/list", hostForwarder("bridges/instances/list")),
		extensionpkg.WithHostMethodHandler("bridges/messages/ingest", hostForwarder("bridges/messages/ingest")),
		extensionpkg.WithHostMethodHandler("bridges/instances/get", hostForwarder("bridges/instances/get")),
		extensionpkg.WithHostMethodHandler(
			"bridges/instances/report_state",
			hostForwarder("bridges/instances/report_state"),
		),
		extensionpkg.WithHealthCheckTimeout(20*time.Millisecond),
		extensionpkg.WithSubprocessSignalGrace(15*time.Millisecond),
	)

	broker := bridgepkg.NewBroker(manager, cfg.BrokerOptions...)
	observer := newHarnessObserver(t, globalDB, homePaths, workspaces, bridgeRegistry, broker, now)
	telemetrySink.observer = observer

	driver := harnessDriver(cfg, now)
	notifier := extensionpkg.NewBridgeDeliveryNotifier(broker, observer)
	sessions := newHarnessSessions(t, homePaths, driver, notifier, workspaces, now)

	hostHandler = extensionpkg.NewHostAPIHandler(
		sessions,
		nil,
		observer,
		skillspkg.NewRegistry(skillspkg.RegistryConfig{}),
		extensionpkg.WithHostAPICapabilityChecker(checker),
		extensionpkg.WithHostAPIWorkspaceResolver(workspaces),
		extensionpkg.WithHostAPIBridgeRegistry(bridgeRegistry),
		extensionpkg.WithHostAPIBridgeDedupStore(globalDB),
		extensionpkg.WithHostAPIDeliveryBroker(broker),
		extensionpkg.WithHostAPINow(func() time.Time { return now }),
		extensionpkg.WithHostAPIBridgeIngressConfig(15*time.Minute, time.Minute),
		extensionpkg.WithHostAPIRateLimit(1000, 1000),
	)

	if err := manager.Start(aghtestutil.Context(t)); err != nil {
		t.Fatalf("manager.Start() error = %v", err)
	}
	return hostHandler, manager, broker, observer, sessions
}

func newHarnessHostForwarder(
	t testing.TB,
	markers MarkerPaths,
	handler func() *extensionpkg.HostAPIHandler,
) func(string) subprocess.HandlerFunc {
	t.Helper()

	return func(method string) subprocess.HandlerFunc {
		return func(ctx context.Context, params json.RawMessage) (any, error) {
			hostHandler := handler()
			if hostHandler == nil {
				return nil, errors.New("extensiontest: host api handler is not initialized")
			}
			result, err := hostHandler.HandleMethod(method)(ctx, params)
			if method == "bridges/instances/report_state" {
				recordHostStateTransition(t, markers.State, params, result, err)
			}
			return result, err
		}
	}
}

func newHarnessObserver(
	t testing.TB,
	globalDB *globaldb.GlobalDB,
	homePaths aghconfig.HomePaths,
	workspaces workspacepkg.RuntimeResolver,
	bridgeRegistry *bridgepkg.Service,
	broker *bridgepkg.Broker,
	now time.Time,
) *observepkg.Observer {
	t.Helper()

	observer, err := observepkg.New(
		aghtestutil.Context(t),
		observepkg.WithRegistry(globalDB),
		observepkg.WithHomePaths(homePaths),
		observepkg.WithWorkspaceResolver(workspaces),
		observepkg.WithBridgeSource(harnessBridgeSource{service: bridgeRegistry, broker: broker}),
		observepkg.WithNow(func() time.Time { return now }),
		observepkg.WithStartTime(now),
	)
	if err != nil {
		t.Fatalf("observe.New() error = %v", err)
	}
	return observer
}

func harnessDriver(cfg HarnessConfig, now time.Time) session.AgentDriver {
	if cfg.Driver != nil {
		return cfg.Driver
	}
	return NewScriptedPromptDriver(now, []ScriptedPromptEvent{
		{Type: acp.EventTypeAgentMessage, Text: bridgeAdapterHarnessHelloKey},
		{Type: acp.EventTypeDone},
	})
}

func newHarnessSessions(
	t testing.TB,
	homePaths aghconfig.HomePaths,
	driver session.AgentDriver,
	notifier session.Notifier,
	workspaces workspacepkg.RuntimeResolver,
	now time.Time,
) *session.Manager {
	t.Helper()

	sandboxRegistry, err := sandboxlocal.NewRegistry()
	if err != nil {
		t.Fatalf("local.NewRegistry() error = %v", err)
	}
	sessions, err := session.NewManager(
		session.WithHomePaths(homePaths),
		session.WithDriver(driver),
		session.WithNotifier(notifier),
		session.WithWorkspaceResolver(workspaces),
		session.WithStore(func(ctx context.Context, sessionID string, path string) (session.EventRecorder, error) {
			return sessiondb.OpenSessionDB(ctx, sessionID, path)
		}),
		session.WithNow(func() time.Time { return now }),
		session.WithSessionIDGenerator(sequentialIDGenerator("sess")),
		session.WithTurnIDGenerator(sequentialIDGenerator("turn")),
		session.WithSandboxRegistry(sandboxRegistry),
	)
	if err != nil {
		t.Fatalf("session.NewManager() error = %v", err)
	}
	return sessions
}
