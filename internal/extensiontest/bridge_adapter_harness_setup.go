package extensiontest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"strings"

	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges"

	aghconfig "github.com/compozy/agh/internal/config"
	extensionpkg "github.com/compozy/agh/internal/extension"

	observepkg "github.com/compozy/agh/internal/observe"

	"github.com/compozy/agh/internal/session"

	"github.com/compozy/agh/internal/store/globaldb"

	"github.com/compozy/agh/internal/subprocess"
	aghtestutil "github.com/compozy/agh/internal/testutil"

	workspacepkg "github.com/compozy/agh/internal/workspace"
)

// ManagedInstanceConfig configures one provider-owned bridge instance created by the harness.
type ManagedInstanceConfig struct {
	ID               string
	DisplayName      string
	DMPolicy         bridgepkg.BridgeDMPolicy
	RoutingPolicy    bridgepkg.RoutingPolicy
	ProviderConfig   map[string]any
	DeliveryDefaults map[string]any
	BoundSecrets     []subprocess.InitializeBridgeBoundSecret
}

// HarnessConfig configures one subprocess-backed bridge adapter test harness.
type HarnessConfig struct {
	ExtensionDir             string
	ExtensionName            string
	DisplayName              string
	Platform                 string
	RoutingPolicy            bridgepkg.RoutingPolicy
	BoundSecrets             []subprocess.InitializeBridgeBoundSecret
	ManagedInstances         []ManagedInstanceConfig
	Driver                   session.AgentDriver
	StartTime                time.Time
	CrashOnceOnFirstDelivery bool
	BrokerOptions            []bridgepkg.DeliveryBrokerOption
	ExtraEnv                 map[string]string
}

// Harness wires the manager, host API, session manager, observer, and marker
// contract used to validate bridge adapters end to end.
type Harness struct {
	HomePaths aghconfig.HomePaths
	Markers   MarkerPaths
	Observer  *observepkg.Observer
	Bridges   *bridgepkg.Service
	Broker    *bridgepkg.Broker
	Handler   *extensionpkg.HostAPIHandler
	Manager   *extensionpkg.Manager
	Sessions  *session.Manager
	Instances []bridgepkg.BridgeInstance
}

// NewHarness starts the reusable adapter conformance harness.
func NewHarness(t testing.TB, cfg HarnessConfig) *Harness {
	t.Helper()

	if strings.TrimSpace(cfg.ExtensionDir) == "" {
		t.Fatal("extensiontest: HarnessConfig.ExtensionDir is required")
	}

	now := resolveHarnessStartTime(cfg.StartTime)
	homePaths := newHarnessHomePaths(t)
	markers := NewTempMarkerPaths(t)
	configureHarnessMarkers(t, markers, cfg)
	workspace := defaultResolvedWorkspace(filepath.Join(t.TempDir(), "workspace"), now)
	if err := os.MkdirAll(workspace.RootDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", workspace.RootDir, err)
	}
	workspaces := &staticWorkspaceResolver{resolved: workspace}

	globalDB := openHarnessGlobalDB(t, homePaths, &workspace)
	manifest, extensionRegistry, extensionName := installHarnessExtension(t, globalDB, cfg)

	bridgeRegistry := bridgepkg.NewRegistry(globalDB, bridgepkg.WithNow(func() time.Time { return now }))
	instances, managedRuntime := createHarnessManagedInstances(
		t,
		bridgeRegistry,
		&workspace,
		extensionName,
		cfg,
	)
	handler, manager, broker, observer, sessions := buildHarnessRuntime(
		t,
		cfg,
		homePaths,
		globalDB,
		workspaces,
		bridgeRegistry,
		extensionRegistry,
		manifest,
		extensionName,
		instances,
		managedRuntime,
		now,
		markers,
	)

	harness := &Harness{
		HomePaths: homePaths,
		Markers:   markers,
		Observer:  observer,
		Bridges:   bridgeRegistry,
		Broker:    broker,
		Handler:   handler,
		Manager:   manager,
		Sessions:  sessions,
		Instances: append([]bridgepkg.BridgeInstance(nil), instances...),
	}

	t.Cleanup(func() {
		harness.stopSessions(t)
		harness.Broker.Close()
		if err := harness.Manager.Stop(aghtestutil.Context(t)); err != nil {
			t.Fatalf("manager.Stop() error = %v", err)
		}
	})

	return harness
}

func resolveHarnessStartTime(startTime time.Time) time.Time {
	now := startTime.UTC()
	if now.IsZero() {
		now = time.Date(2026, 4, 11, 4, 0, 0, 0, time.UTC)
	}
	return now
}

func newHarnessHomePaths(t testing.TB) aghconfig.HomePaths {
	t.Helper()

	homePaths, err := aghconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := aghconfig.EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}
	return homePaths
}

func configureHarnessMarkers(t testing.TB, markers MarkerPaths, cfg HarnessConfig) {
	t.Helper()

	for key, value := range markers.Env() {
		if key == EnvCrashOncePath {
			continue
		}
		t.Setenv(key, value)
	}
	if cfg.CrashOnceOnFirstDelivery {
		t.Setenv(EnvCrashOncePath, markers.CrashOnce)
	}
	for key, value := range cfg.ExtraEnv {
		t.Setenv(key, value)
	}
}

func openHarnessGlobalDB(
	t testing.TB,
	homePaths aghconfig.HomePaths,
	workspace *workspacepkg.ResolvedWorkspace,
) *globaldb.GlobalDB {
	t.Helper()

	resolvedWorkspace := mustHarnessWorkspace(t, workspace)
	globalDB, err := globaldb.OpenGlobalDB(aghtestutil.Context(t), homePaths.DatabaseFile)
	if err != nil {
		t.Fatalf("globaldb.OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := globalDB.Close(aghtestutil.Context(t)); err != nil {
			t.Fatalf("globaldb.Close() error = %v", err)
		}
	})
	if err := globalDB.InsertWorkspace(aghtestutil.Context(t), resolvedWorkspace.Workspace); err != nil {
		t.Fatalf("globalDB.InsertWorkspace() error = %v", err)
	}
	return globalDB
}

func mustHarnessWorkspace(
	t testing.TB,
	workspace *workspacepkg.ResolvedWorkspace,
) *workspacepkg.ResolvedWorkspace {
	t.Helper()

	if workspace == nil {
		t.Fatal("workspace = nil")
	}
	return workspace
}

func harnessWorkspaceID(workspace *workspacepkg.ResolvedWorkspace) string {
	if workspace == nil {
		return ""
	}
	if workspaceID := strings.TrimSpace(workspace.WorkspaceID); workspaceID != "" {
		return workspaceID
	}
	return strings.TrimSpace(workspace.ID)
}

func installHarnessExtension(
	t testing.TB,
	globalDB *globaldb.GlobalDB,
	cfg HarnessConfig,
) (*extensionpkg.Manifest, *extensionpkg.Registry, string) {
	t.Helper()

	manifest, err := extensionpkg.LoadManifest(cfg.ExtensionDir)
	if err != nil {
		t.Fatalf("extension.LoadManifest() error = %v", err)
	}
	checksum, err := extensionpkg.ComputeDirectoryChecksum(cfg.ExtensionDir)
	if err != nil {
		t.Fatalf("extension.ComputeDirectoryChecksum() error = %v", err)
	}
	extensionRegistry := extensionpkg.NewRegistry(globalDB.DB())
	if err := extensionRegistry.Install(manifest, cfg.ExtensionDir, checksum); err != nil {
		t.Fatalf("extensionRegistry.Install() error = %v", err)
	}

	extensionName := strings.TrimSpace(cfg.ExtensionName)
	if extensionName == "" {
		extensionName = manifest.Name
	}
	return manifest, extensionRegistry, extensionName
}

func createHarnessManagedInstances(
	t testing.TB,
	bridgeRegistry *bridgepkg.Service,
	workspace *workspacepkg.ResolvedWorkspace,
	extensionName string,
	cfg HarnessConfig,
) ([]bridgepkg.BridgeInstance, []subprocess.InitializeBridgeManagedInstance) {
	t.Helper()

	managedConfigs := cfg.ManagedInstances
	if len(managedConfigs) == 0 {
		routingPolicy := cfg.RoutingPolicy
		if routingPolicy == (bridgepkg.RoutingPolicy{}) {
			routingPolicy = bridgepkg.RoutingPolicy{IncludePeer: true}
		}
		managedConfigs = []ManagedInstanceConfig{{
			ID:            bridgeAdapterHarnessBrgTelegramReferenceValue,
			DisplayName:   firstNonEmpty(cfg.DisplayName, "Telegram Reference"),
			RoutingPolicy: routingPolicy,
			BoundSecrets:  cloneBoundSecrets(cfg.BoundSecrets),
		}}
	}

	instances := make([]bridgepkg.BridgeInstance, 0, len(managedConfigs))
	managedRuntime := make([]subprocess.InitializeBridgeManagedInstance, 0, len(managedConfigs))
	for _, managedCfg := range managedConfigs {
		createReq, err := harnessCreateInstanceRequest(cfg, workspace, extensionName, len(instances)+1, managedCfg)
		if err != nil {
			t.Fatalf("harnessCreateInstanceRequest(%q) error = %v", managedCfg.ID, err)
		}
		instance, err := bridgeRegistry.CreateInstance(aghtestutil.Context(t), createReq)
		if err != nil {
			t.Fatalf("bridgeRegistry.CreateInstance(%q) error = %v", createReq.ID, err)
		}
		instances = append(instances, *instance)
		managedRuntime = append(managedRuntime, subprocess.InitializeBridgeManagedInstance{
			Instance:     bridgepkg.BridgeInstanceToContract(*instance),
			BoundSecrets: cloneBoundSecrets(managedCfg.BoundSecrets),
		})
	}
	return instances, managedRuntime
}

func harnessCreateInstanceRequest(
	cfg HarnessConfig,
	workspace *workspacepkg.ResolvedWorkspace,
	extensionName string,
	seq int,
	managedCfg ManagedInstanceConfig,
) (bridgepkg.CreateInstanceRequest, error) {
	if workspace == nil {
		return bridgepkg.CreateInstanceRequest{}, errors.New("workspace is required")
	}
	var providerConfig json.RawMessage
	if managedCfg.ProviderConfig != nil {
		encodedProviderConfig, err := json.Marshal(managedCfg.ProviderConfig)
		if err != nil {
			return bridgepkg.CreateInstanceRequest{}, fmt.Errorf(
				"json.Marshal(provider_config for %q): %w",
				managedCfg.ID,
				err,
			)
		}
		providerConfig = encodedProviderConfig
	}
	var deliveryDefaults json.RawMessage
	if managedCfg.DeliveryDefaults != nil {
		encodedDeliveryDefaults, err := json.Marshal(managedCfg.DeliveryDefaults)
		if err != nil {
			return bridgepkg.CreateInstanceRequest{}, fmt.Errorf(
				"json.Marshal(delivery_defaults for %q): %w",
				managedCfg.ID,
				err,
			)
		}
		deliveryDefaults = encodedDeliveryDefaults
	}

	createReq := bridgepkg.CreateInstanceRequest{
		ID:               firstNonEmpty(managedCfg.ID, fmt.Sprintf("brg-%d", seq)),
		Scope:            bridgepkg.ScopeWorkspace,
		WorkspaceID:      harnessWorkspaceID(workspace),
		Platform:         firstNonEmpty(cfg.Platform, "telegram"),
		ExtensionName:    extensionName,
		DisplayName:      firstNonEmpty(managedCfg.DisplayName, cfg.DisplayName, "Telegram Reference"),
		Enabled:          true,
		Status:           bridgepkg.BridgeStatusStarting,
		DMPolicy:         managedCfg.DMPolicy,
		RoutingPolicy:    managedCfg.RoutingPolicy,
		ProviderConfig:   providerConfig,
		DeliveryDefaults: deliveryDefaults,
	}
	if createReq.RoutingPolicy == (bridgepkg.RoutingPolicy{}) {
		createReq.RoutingPolicy = bridgepkg.RoutingPolicy{IncludePeer: true}
	}
	return createReq, nil
}
