package extensionpkg

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	automationpkg "github.com/compozy/agh/internal/automation"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	aghconfig "github.com/compozy/agh/internal/config"
	extensioncontract "github.com/compozy/agh/internal/extension/contract"
	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/modelcatalog"
	"github.com/compozy/agh/internal/resources"
	skillspkg "github.com/compozy/agh/internal/skills"
	"github.com/compozy/agh/internal/subprocess"
	"github.com/compozy/agh/internal/testutil"
	"github.com/compozy/agh/internal/toolruntime"
	toolspkg "github.com/compozy/agh/internal/tools"
	"github.com/compozy/agh/internal/windowmanager"
)

const (
	extensionHelperEnvKey      = "AGH_TEST_EXTENSION_HELPER"
	extensionHelperScenarioKey = "AGH_TEST_EXTENSION_SCENARIO"
	extensionHelperMarkerKey   = "AGH_TEST_EXTENSION_MARKER"
)

type noopBridgeTelemetrySink struct{}

var _ BridgeTelemetrySink = (*noopBridgeTelemetrySink)(nil)

func (noopBridgeTelemetrySink) RecordBridgeAuthFailure(string) {}

func (noopBridgeTelemetrySink) RecordBridgeRuntimeIssue(string, bridgepkg.BridgeStatus, string) {}

func (noopBridgeTelemetrySink) ClearBridgeRuntimeIssue(string) {}

type recordingBridgeTelemetrySink struct {
	issues []string
	clears []string
}

func (r *recordingBridgeTelemetrySink) RecordBridgeAuthFailure(string) {}

func (r *recordingBridgeTelemetrySink) RecordBridgeRuntimeIssue(
	bridgeInstanceID string,
	status bridgepkg.BridgeStatus,
	message string,
) {
	r.issues = append(r.issues, fmt.Sprintf("%s:%s:%s", bridgeInstanceID, status, message))
}

func (r *recordingBridgeTelemetrySink) ClearBridgeRuntimeIssue(bridgeInstanceID string) {
	r.clears = append(r.clears, bridgeInstanceID)
}

func TestExtensionManagerHelperProcess(_ *testing.T) {
	if os.Getenv(extensionHelperEnvKey) != "1" {
		return
	}

	server := newExtensionHelperServer(
		os.Getenv(extensionHelperScenarioKey),
		strings.TrimSpace(os.Getenv(extensionHelperMarkerKey)),
	)
	os.Exit(server.run())
}

func TestManagerStartRegistersResourcesAndActivatesExtension(t *testing.T) {
	t.Parallel()

	withDaemonVersion(t, "0.5.0")
	env := newRegistryTestEnv(t)
	fixture := createManagerTestExtension(t, managerTestManifest("ext-runtime", managerManifestOptions{
		command:      "fake-extension",
		withSkills:   true,
		withAgents:   true,
		withHooks:    true,
		withMCP:      true,
		capabilities: []string{"memory.backend"},
		actions:      []string{"sessions/list"},
		security:     []string{"session.read"},
	}), map[string]string{
		"skills/review.md": managerSkillFile("ext-review", "External review workflow"),
		"agents/coder.md":  managerAgentFile("ext-agent"),
	})
	installManagerFixture(t, env.registry, fixture, SourceUser, true)

	fakeProc := newFakeProcess(101)
	launcher := &fakeLauncher{queue: []*fakeProcess{fakeProc}}
	processRegistry := toolruntime.NewRegistry(toolruntime.NewMemoryStore())

	manager := NewManager(
		env.registry,
		WithHostMethodHandler("sessions/list", func(_ context.Context, _ json.RawMessage) (any, error) {
			return []map[string]string{{"id": "sess-1"}}, nil
		}),
		WithHealthCheckTimeout(20*time.Millisecond),
		WithDefaultHookTimeout(25*time.Millisecond),
		withProcessLauncher(launcher.launch),
		withHealthPollBounds(time.Millisecond, 2*time.Millisecond),
		WithProcessRegistry(processRegistry),
	)

	if err := manager.Start(testutil.Context(t)); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Stop(testutil.Context(t)); err != nil {
			t.Fatalf("Stop() cleanup error = %v", err)
		}
	})

	if got := launcher.launchCount(); got != 1 {
		t.Fatalf("launch count = %d, want 1", got)
	}
	launcher.mu.Lock()
	launchConfig := launcher.config[0]
	launcher.mu.Unlock()
	if launchConfig.ProcessRegistry != processRegistry {
		t.Fatalf("LaunchConfig.ProcessRegistry = %p, want %p", launchConfig.ProcessRegistry, processRegistry)
	}
	if launchConfig.ProcessRecord.Source != toolruntime.ProcessSourceExtension ||
		launchConfig.ProcessRecord.Owner.ExtensionName != "ext-runtime" {
		t.Fatalf("LaunchConfig.ProcessRecord = %#v, want extension ownership", launchConfig.ProcessRecord)
	}
	if len(fakeProc.initRequests()) != 1 {
		t.Fatalf("len(initialize requests) = %d, want 1", len(fakeProc.initRequests()))
	}

	request := fakeProc.initRequests()[0]
	if request.ProtocolVersion != defaultProtocolVersion {
		t.Fatalf("initialize protocol version = %q, want %q", request.ProtocolVersion, defaultProtocolVersion)
	}
	if strings.TrimSpace(request.SessionNonce) == "" {
		t.Fatal("initialize session nonce = empty, want daemon-issued nonce")
	}
	if !slices.Equal(request.Capabilities.GrantedActions, []extensionprotocol.HostAPIMethod{
		extensionprotocol.HostAPIMethodSessionsList,
	}) {
		t.Fatalf("initialize granted actions = %#v, want [sessions/list]", request.Capabilities.GrantedActions)
	}
	if !slices.Equal(request.Capabilities.GrantedSecurity, []string{"session.read"}) {
		t.Fatalf("initialize granted security = %#v, want [session.read]", request.Capabilities.GrantedSecurity)
	}
	if !slices.Equal(request.Methods.ExtensionServices, []string{"memory/forget", "memory/recall", "memory/store"}) {
		t.Fatalf("initialize extension services = %#v, want memory backend methods", request.Methods.ExtensionServices)
	}

	decls, err := manager.HookDeclarations(testutil.Context(t))
	if err != nil {
		t.Fatalf("HookDeclarations() error = %v", err)
	}
	if len(decls) != 1 {
		t.Fatalf("len(HookDeclarations()) = %d, want 1", len(decls))
	}
	if got, want := decls[0].Name, "ext-runtime-hook"; got != want {
		t.Fatalf("HookDeclarations()[0].Name = %q, want %q", got, want)
	}
	if got, want := decls[0].Metadata["extension"], "ext-runtime"; got != want {
		t.Fatalf("HookDeclarations()[0].Metadata[extension] = %q, want %q", got, want)
	}

	agents := manager.AgentDefinitions()
	if len(agents) != 1 || agents[0].Name != "ext-agent" {
		t.Fatalf("AgentDefinitions() = %#v, want ext-agent", agents)
	}

	loaded, err := manager.Get("ext-runtime")
	if err != nil {
		t.Fatalf("Get(ext-runtime) error = %v", err)
	}
	if len(loaded.Skills) != 1 || loaded.Skills[0].Meta.Name != "ext-review" {
		t.Fatalf("Get(ext-runtime).Skills = %#v, want ext-review extension snapshot", loaded.Skills)
	}
	if got, want := loaded.Skills[0].InstalledFromExtension, "ext-runtime"; got != want {
		t.Fatalf("Get(ext-runtime).Skills[0].InstalledFromExtension = %q, want %q", got, want)
	}
	if !loaded.Status.Active {
		t.Fatalf("Get(ext-runtime).Status.Active = false, want true")
	}
	if loaded.Manifest == nil || loaded.Manifest.Resources.MCPServers["kubectl"].Command == "" {
		t.Fatalf(
			"Get(ext-runtime).Manifest.Resources.MCPServers = %#v, want kubectl manifest declaration",
			loaded.Manifest,
		)
	}
	if got, want := loaded.Status.Phase, ExtensionPhaseActivate; got != want {
		t.Fatalf("Get(ext-runtime).Status.Phase = %q, want %q", got, want)
	}
}

func TestExtensionSkillInstalledFrom(t *testing.T) {
	t.Parallel()

	registrySlug := "acme/marketplace-ext"
	tests := []struct {
		name string
		info ExtensionInfo
		want string
	}{
		{
			name: "Should prefer marketplace registry slug",
			info: ExtensionInfo{
				Name:         "runtime-name",
				RegistrySlug: &registrySlug,
				Provenance: ExtensionProvenance{
					Slug: "acme/provenance-ext",
				},
			},
			want: "acme/marketplace-ext",
		},
		{
			name: "Should use provenance slug when registry slug is absent",
			info: ExtensionInfo{
				Name: "runtime-name",
				Provenance: ExtensionProvenance{
					Slug: "acme/provenance-ext",
				},
			},
			want: "acme/provenance-ext",
		},
		{
			name: "Should fall back to extension name for local installs",
			info: ExtensionInfo{Name: "runtime-name"},
			want: "runtime-name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := extensionSkillInstalledFrom(tt.info); got != tt.want {
				t.Fatalf("extensionSkillInstalledFrom() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestManagerStartBridgeAdapterNegotiatesScopedLaunchRuntime(t *testing.T) {
	t.Parallel()

	withDaemonVersion(t, "0.5.0")
	env := newRegistryTestEnv(t)
	fixture := createManagerTestExtension(t, managerTestManifest("ext-bridge", managerManifestOptions{
		command:      "fake-extension",
		capabilities: []string{extensionprotocol.CapabilityProvideBridgeAdapter},
		actions: []string{
			string(extensionprotocol.HostAPIMethodBridgesMessagesIngest),
			string(extensionprotocol.HostAPIMethodBridgesInstancesGet),
			string(extensionprotocol.HostAPIMethodBridgesInstancesReportState),
		},
		security: []string{"bridge.read", "bridge.write"},
	}), nil)
	installManagerFixture(t, env.registry, fixture, SourceUser, true)

	fakeProc := newFakeProcess(303)
	launcher := &fakeLauncher{queue: []*fakeProcess{fakeProc}}
	manager := NewManager(
		env.registry,
		WithBridgeRuntimeResolver(&stubBridgeRuntimeResolver{
			runtimes: map[string]*subprocess.InitializeBridgeRuntime{
				"ext-bridge": testScopedBridgeRuntime(
					"ext-bridge",
					"brg-1",
					[]subprocess.InitializeBridgeBoundSecret{
						{BindingName: "bot_token", Kind: "bot_token", Value: "token-1"},
					},
				),
			},
		}),
		withProcessLauncher(launcher.launch),
		WithHealthCheckTimeout(20*time.Millisecond),
		withHealthPollBounds(time.Millisecond, 2*time.Millisecond),
	)

	if err := manager.Start(testutil.Context(t)); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Stop(testutil.Context(t)); err != nil {
			t.Fatalf("Stop() cleanup error = %v", err)
		}
	})

	requests := fakeProc.initRequests()
	if len(requests) != 1 {
		t.Fatalf("len(initialize requests) = %d, want 1", len(requests))
	}

	request := requests[0]
	if !slices.Equal(request.Methods.ExtensionServices, []string{"bridges/deliver", "bridges/targets/snapshot"}) {
		t.Fatalf(
			"initialize extension services = %#v, want [bridges/deliver bridges/targets/snapshot]",
			request.Methods.ExtensionServices,
		)
	}
	if !slices.Equal(request.Capabilities.GrantedActions, []extensionprotocol.HostAPIMethod{
		extensionprotocol.HostAPIMethodBridgesInstancesGet,
		extensionprotocol.HostAPIMethodBridgesInstancesReportState,
		extensionprotocol.HostAPIMethodBridgesMessagesIngest,
	}) {
		t.Fatalf("initialize granted actions = %#v, want bridge actions", request.Capabilities.GrantedActions)
	}
	if !slices.Equal(request.Capabilities.GrantedSecurity, []string{"bridge.read", "bridge.write"}) {
		t.Fatalf("initialize granted security = %#v, want bridge grants", request.Capabilities.GrantedSecurity)
	}
	if request.Runtime.Bridge == nil {
		t.Fatal("initialize runtime bridge = nil, want scoped bridge launch payload")
	}
	managed := mustSingleManagedBridge(t, request.Runtime.Bridge)
	if got, want := managed.Instance.ID, "brg-1"; got != want {
		t.Fatalf("initialize runtime bridge instance id = %q, want %q", got, want)
	}
	if got, want := managed.Instance.ExtensionName, "ext-bridge"; got != want {
		t.Fatalf("initialize runtime bridge instance extension = %q, want %q", got, want)
	}
	if got := managed.BoundSecrets; len(got) != 1 || got[0].BindingName != "bot_token" || got[0].Value != "token-1" {
		t.Fatalf("initialize runtime bridge bound secrets = %#v, want only bot_token", got)
	}

	for _, method := range request.Capabilities.GrantedActions {
		if strings.Contains(string(method), "vault/") || strings.Contains(string(method), "secret/") {
			t.Fatalf("initialize granted actions leaked secret lookup method %q", method)
		}
	}
	for _, method := range request.Methods.ExtensionServices {
		if strings.Contains(method, "vault/") || strings.Contains(method, "secret/") {
			t.Fatalf("initialize extension services leaked secret lookup method %q", method)
		}
	}
}

func TestManagerStartBridgeAdapterRequiresScopedLaunchRuntime(t *testing.T) {
	t.Parallel()

	withDaemonVersion(t, "0.5.0")
	env := newRegistryTestEnv(t)
	fixture := createManagerTestExtension(t, managerTestManifest("ext-bridge-missing", managerManifestOptions{
		command:      "fake-extension",
		capabilities: []string{extensionprotocol.CapabilityProvideBridgeAdapter},
		actions:      []string{string(extensionprotocol.HostAPIMethodBridgesInstancesGet)},
		security:     []string{"bridge.read"},
	}), nil)
	installManagerFixture(t, env.registry, fixture, SourceUser, true)

	manager := NewManager(
		env.registry,
		withProcessLauncher((&fakeLauncher{queue: []*fakeProcess{newFakeProcess(404)}}).launch),
	)

	err := manager.Start(testutil.Context(t))
	if err == nil {
		t.Fatal("Start() error = nil, want missing bridge runtime resolver failure")
	}
	if !errors.Is(err, ErrBridgeRuntimeResolverRequired) {
		t.Fatalf("Start() error = %v, want %v", err, ErrBridgeRuntimeResolverRequired)
	}
}

func TestManagerStartBridgeAdapterDefersUntilRuntimeExists(t *testing.T) {
	t.Parallel()

	withDaemonVersion(t, "0.5.0")
	env := newRegistryTestEnv(t)
	fixture := createManagerTestExtension(t, managerTestManifest("ext-bridge-deferred", managerManifestOptions{
		command:      "fake-extension",
		capabilities: []string{extensionprotocol.CapabilityProvideBridgeAdapter},
		actions: []string{
			string(extensionprotocol.HostAPIMethodBridgesMessagesIngest),
			string(extensionprotocol.HostAPIMethodBridgesInstancesGet),
		},
		security: []string{"bridge.read"},
	}), nil)
	installManagerFixture(t, env.registry, fixture, SourceUser, true)

	launcher := &fakeLauncher{}
	manager := NewManager(
		env.registry,
		WithBridgeRuntimeResolver(&stubBridgeRuntimeResolver{err: ErrBridgeRuntimeDeferred}),
		withProcessLauncher(launcher.launch),
	)

	if err := manager.Start(testutil.Context(t)); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Stop(testutil.Context(t)); err != nil {
			t.Fatalf("Stop() cleanup error = %v", err)
		}
	})

	if got := launcher.launchCount(); got != 0 {
		t.Fatalf("launch count = %d, want 0 while runtime is deferred", got)
	}

	loaded, err := manager.Get("ext-bridge-deferred")
	if err != nil {
		t.Fatalf("Get(ext-bridge-deferred) error = %v", err)
	}
	if loaded.Status.Active {
		t.Fatal("Get(ext-bridge-deferred).Status.Active = true, want false")
	}
	if !loaded.Status.Registered {
		t.Fatal("Get(ext-bridge-deferred).Status.Registered = false, want true")
	}
	if loaded.Status.LastError != "" {
		t.Fatalf("Get(ext-bridge-deferred).Status.LastError = %q, want empty", loaded.Status.LastError)
	}
}

func TestManagerStartSkipsDisabledExtensions(t *testing.T) {
	t.Parallel()

	withDaemonVersion(t, "0.5.0")
	env := newRegistryTestEnv(t)
	fixture := createManagerTestExtension(t, managerTestManifest("ext-disabled", managerManifestOptions{
		command:      "fake-extension",
		capabilities: []string{"memory.backend"},
		actions:      []string{"sessions/list"},
		security:     []string{"session.read"},
	}), nil)
	installManagerFixture(t, env.registry, fixture, SourceUser, false)

	launcher := &fakeLauncher{}
	manager := NewManager(env.registry, withProcessLauncher(launcher.launch))

	if err := manager.Start(testutil.Context(t)); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Stop(testutil.Context(t)); err != nil {
			t.Fatalf("Stop() cleanup error = %v", err)
		}
	})

	if got := launcher.launchCount(); got != 0 {
		t.Fatalf("launch count = %d, want 0", got)
	}

	statuses := manager.Statuses()
	if len(statuses) != 1 || statuses[0].Enabled {
		t.Fatalf("Statuses() = %#v, want one disabled extension", statuses)
	}
}

func TestManagerStartContinuesAfterParseFailure(t *testing.T) {
	t.Parallel()

	withDaemonVersion(t, "0.5.0")
	env := newRegistryTestEnv(t)

	badFixture := createManagerTestExtension(t, managerTestManifest("ext-bad", managerManifestOptions{
		command:      "fake-extension",
		capabilities: []string{"memory.backend"},
		actions:      []string{"sessions/list"},
		security:     []string{"session.read"},
	}), nil)
	installManagerFixture(t, env.registry, badFixture, SourceUser, true)
	writeFile(t, filepath.Join(badFixture.dir, manifestTOMLFileName), "not = [valid")

	goodFixture := createManagerTestExtension(t, managerTestManifest("ext-good", managerManifestOptions{
		command:      "fake-extension",
		capabilities: []string{"memory.backend"},
		actions:      []string{"sessions/list"},
		security:     []string{"session.read"},
		withHooks:    true,
	}), nil)
	installManagerFixture(t, env.registry, goodFixture, SourceUser, true)

	launcher := &fakeLauncher{queue: []*fakeProcess{newFakeProcess(202)}}
	manager := NewManager(
		env.registry,
		withProcessLauncher(launcher.launch),
		withHealthPollBounds(time.Millisecond, 2*time.Millisecond),
	)

	err := manager.Start(testutil.Context(t))
	if err == nil {
		t.Fatal("Start() error = nil, want joined parse failure")
	}
	if !strings.Contains(err.Error(), `extension "ext-bad" parse`) {
		t.Fatalf("Start() error = %v, want parse phase detail for ext-bad", err)
	}
	t.Cleanup(func() {
		if stopErr := manager.Stop(testutil.Context(t)); stopErr != nil {
			t.Fatalf("Stop() cleanup error = %v", stopErr)
		}
	})

	if got := launcher.launchCount(); got != 1 {
		t.Fatalf("launch count = %d, want 1 for ext-good only", got)
	}
	good, err := manager.Get("ext-good")
	if err != nil {
		t.Fatalf("Get(ext-good) error = %v", err)
	}
	if !good.Status.Active {
		t.Fatalf("Get(ext-good).Status.Active = false, want true")
	}
}

func TestManagerStartRejectsIncompatibleManifest(t *testing.T) {
	t.Parallel()

	withDaemonVersion(t, "0.5.0")
	env := newRegistryTestEnv(t)
	fixture := createManagerTestExtension(t, managerTestManifest("ext-incompatible", managerManifestOptions{
		command:      "fake-extension",
		capabilities: []string{"memory.backend"},
		actions:      []string{"sessions/list"},
		security:     []string{"session.read"},
	}), nil)
	installManagerFixture(t, env.registry, fixture, SourceUser, true)
	writeFile(
		t,
		filepath.Join(fixture.dir, manifestTOMLFileName),
		managerTestManifest("ext-incompatible", managerManifestOptions{
			command:      "fake-extension",
			minVersion:   "9.0.0",
			capabilities: []string{"memory.backend"},
			actions:      []string{"sessions/list"},
			security:     []string{"session.read"},
		}),
	)

	launcher := &fakeLauncher{}
	manager := NewManager(env.registry, withProcessLauncher(launcher.launch))

	err := manager.Start(testutil.Context(t))
	if err == nil {
		t.Fatal("Start() error = nil, want incompatibility error")
	}
	if !errors.Is(err, ErrManifestIncompatible) {
		t.Fatalf("Start() error = %v, want ErrManifestIncompatible", err)
	}
	if got := launcher.launchCount(); got != 0 {
		t.Fatalf("launch count = %d, want 0", got)
	}
}

func TestManagerCrashTriggersRestartWithBackoff(t *testing.T) {
	t.Parallel()

	withDaemonVersion(t, "0.5.0")
	env := newRegistryTestEnv(t)
	fixture := createManagerTestExtension(t, managerTestManifest("ext-restart", managerManifestOptions{
		command:      "fake-extension",
		capabilities: []string{"memory.backend"},
		actions:      []string{"sessions/list"},
		security:     []string{"session.read"},
	}), nil)
	installManagerFixture(t, env.registry, fixture, SourceUser, true)

	first := newFakeProcess(301)
	second := newFakeProcess(302)
	launcher := &fakeLauncher{queue: []*fakeProcess{first, second}}

	manager := NewManager(
		env.registry,
		withProcessLauncher(launcher.launch),
		withRestartBackoffMax(2*time.Millisecond),
		withHealthPollBounds(time.Millisecond, 2*time.Millisecond),
	)

	if err := manager.Start(testutil.Context(t)); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Stop(testutil.Context(t)); err != nil {
			t.Fatalf("Stop() cleanup error = %v", err)
		}
	})

	first.crash(errors.New("boom"))

	waitForManagerCondition(t, time.Second, func() bool {
		if launcher.launchCount() != 2 {
			return false
		}
		loaded, err := manager.Get("ext-restart")
		return err == nil && loaded.Status.Active && loaded.Status.PID == 302
	})

	loaded, err := manager.Get("ext-restart")
	if err != nil {
		t.Fatalf("Get(ext-restart) error = %v", err)
	}
	if !loaded.Status.Active || loaded.Status.PID != 302 {
		t.Fatalf("Get(ext-restart).Status = %#v, want restarted process pid 302", loaded.Status)
	}
}

func TestManagerStartDetachesSupervisorFromStartContext(t *testing.T) {
	t.Parallel()

	withDaemonVersion(t, "0.5.0")
	env := newRegistryTestEnv(t)
	fixture := createManagerTestExtension(t, managerTestManifest("ext-detached", managerManifestOptions{
		command:      "fake-extension",
		capabilities: []string{"memory.backend"},
		actions:      []string{"sessions/list"},
		security:     []string{"session.read"},
	}), nil)
	installManagerFixture(t, env.registry, fixture, SourceUser, true)

	first := newFakeProcess(601)
	second := newFakeProcess(602)
	launcher := &fakeLauncher{queue: []*fakeProcess{first, second}}

	manager := NewManager(
		env.registry,
		withProcessLauncher(launcher.launch),
		withRestartBackoffMax(2*time.Millisecond),
		withHealthPollBounds(time.Millisecond, 2*time.Millisecond),
	)

	startCtx, cancelStart := context.WithCancel(testutil.Context(t))
	if err := manager.Start(startCtx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	cancelStart()
	t.Cleanup(func() {
		if err := manager.Stop(testutil.Context(t)); err != nil {
			t.Fatalf("Stop() cleanup error = %v", err)
		}
	})

	first.crash(errors.New("boom"))

	waitForManagerCondition(t, time.Second, func() bool {
		if launcher.launchCount() != 2 {
			return false
		}
		loaded, err := manager.Get("ext-detached")
		return err == nil && loaded.Status.Active && loaded.Status.PID == 602
	})
}

func TestManagerDisablesExtensionAfterConsecutiveFailures(t *testing.T) {
	t.Parallel()

	withDaemonVersion(t, "0.5.0")
	env := newRegistryTestEnv(t)
	fixture := createManagerTestExtension(t, managerTestManifest("ext-flaky", managerManifestOptions{
		command:      "fake-extension",
		capabilities: []string{"memory.backend"},
		actions:      []string{"sessions/list"},
		security:     []string{"session.read"},
		withHooks:    true,
	}), nil)
	installManagerFixture(t, env.registry, fixture, SourceUser, true)

	queue := make([]*fakeProcess, 0, 5)
	for pid := 401; pid <= 405; pid++ {
		proc := newFakeProcess(pid)
		proc.initHook = func(p *fakeProcess) func() {
			return func() {
				go p.crash(errors.New("panic"))
			}
		}(proc)
		queue = append(queue, proc)
	}
	launcher := &fakeLauncher{queue: queue}

	manager := NewManager(
		env.registry,
		withProcessLauncher(launcher.launch),
		withRestartBackoffMax(2*time.Millisecond),
		withHealthPollBounds(time.Millisecond, 2*time.Millisecond),
	)

	if err := manager.Start(testutil.Context(t)); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Stop(testutil.Context(t)); err != nil {
			t.Fatalf("Stop() cleanup error = %v", err)
		}
	})

	waitForManagerCondition(t, time.Second, func() bool {
		info, err := env.registry.Get("ext-flaky")
		return err == nil && !info.Enabled
	})

	if got := launcher.launchCount(); got != 5 {
		t.Fatalf("launch count = %d, want 5 before disable", got)
	}

	decls, err := manager.HookDeclarations(testutil.Context(t))
	if err != nil {
		t.Fatalf("HookDeclarations() error = %v", err)
	}
	if len(decls) != 0 {
		t.Fatalf("HookDeclarations() = %#v, want resources removed after disable", decls)
	}
}

func TestManagerStopUsesRealSubprocessShutdown(t *testing.T) {
	t.Parallel()

	withDaemonVersion(t, "0.5.0")
	env := newRegistryTestEnv(t)
	markerPath := filepath.Join(t.TempDir(), "shutdown.marker")
	fixture := createManagerTestExtension(t, managerTestManifest("ext-stop", managerManifestOptions{
		command:      helperCommand(t),
		args:         helperArgs(),
		withEnv:      helperEnv("default", markerPath),
		capabilities: []string{"memory.backend"},
		actions:      []string{"sessions/list"},
		security:     []string{"session.read"},
	}), nil)
	installManagerFixture(t, env.registry, fixture, SourceUser, true)

	manager := NewManager(
		env.registry,
		WithHealthCheckTimeout(25*time.Millisecond),
		WithDefaultHookTimeout(25*time.Millisecond),
		WithSubprocessSignalGrace(15*time.Millisecond),
	)

	if err := manager.Start(testutil.Context(t)); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if err := manager.Stop(testutil.Context(t)); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if _, err := os.Stat(markerPath); err != nil {
		t.Fatalf("os.Stat(%q) error = %v, want shutdown marker", markerPath, err)
	}
}

func TestManagerStopKillsHungSubprocessAfterTimeout(t *testing.T) {
	t.Parallel()

	withDaemonVersion(t, "0.5.0")
	env := newRegistryTestEnv(t)
	fixture := createManagerTestExtension(t, managerTestManifest("ext-hang", managerManifestOptions{
		command:      helperCommand(t),
		args:         helperArgs(),
		withEnv:      helperEnv("shutdown_hang", ""),
		capabilities: []string{"memory.backend"},
		actions:      []string{"sessions/list"},
		security:     []string{"session.read"},
		shutdown:     40 * time.Millisecond,
	}), nil)
	installManagerFixture(t, env.registry, fixture, SourceUser, true)

	manager := NewManager(
		env.registry,
		WithHealthCheckTimeout(20*time.Millisecond),
		WithSubprocessSignalGrace(20*time.Millisecond),
	)

	if err := manager.Start(testutil.Context(t)); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if err := manager.Stop(testutil.Context(t)); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	statuses := manager.Statuses()
	if len(statuses) != 1 || statuses[0].Active {
		t.Fatalf("Statuses() after Stop = %#v, want inactive extension", statuses)
	}
}

func TestNewManagerAppliesOptionsAndRestoresDefaults(t *testing.T) {
	t.Parallel()

	telemetrySink := &noopBridgeTelemetrySink{}
	manager := NewManager(
		nil,
		WithCapabilityChecker(nil),
		WithBridgeTelemetrySink(telemetrySink),
		WithLogger(nil),
		WithNow(nil),
		WithGetenv(nil),
		WithHostMethodHandler(" sessions/list ", func(context.Context, json.RawMessage) (any, error) {
			return "ok", nil
		}),
		WithInitializeTimeout(0),
		WithHealthCheckTimeout(0),
		WithDefaultHookTimeout(0),
		WithSubprocessSignalGrace(0),
		withProcessLauncher(nil),
		withRestartBackoffMax(0),
		withRestartFailureThreshold(0),
		withHealthPollBounds(0, 0),
	)

	if manager.capChecker == nil {
		t.Fatal("capChecker = nil, want default checker")
	}
	if manager.logger == nil {
		t.Fatal("logger = nil, want default logger")
	}
	if manager.now == nil {
		t.Fatal("now = nil, want default clock")
	}
	if manager.getenv == nil {
		t.Fatal("getenv = nil, want default env resolver")
	}
	if manager.bridgeTelemetrySink != telemetrySink {
		t.Fatalf("bridgeTelemetrySink = %#v, want injected sink %#v", manager.bridgeTelemetrySink, telemetrySink)
	}
	if manager.launch == nil {
		t.Fatal("launch = nil, want default launcher")
	}
	if got, want := manager.initializeTimeout, defaultInitializeTimeout; got != want {
		t.Fatalf("initializeTimeout = %v, want %v", got, want)
	}
	if got, want := manager.healthCheckTimeout, defaultHealthCheckTimeout; got != want {
		t.Fatalf("healthCheckTimeout = %v, want %v", got, want)
	}
	if got, want := manager.defaultHookTimeout, defaultHookTimeout; got != want {
		t.Fatalf("defaultHookTimeout = %v, want %v", got, want)
	}
	if got, want := manager.restartBackoffMax, defaultRestartBackoffMax; got != want {
		t.Fatalf("restartBackoffMax = %v, want %v", got, want)
	}
	if got, want := manager.restartFailureThreshold, defaultRestartFailureThreshold; got != want {
		t.Fatalf("restartFailureThreshold = %d, want %d", got, want)
	}
	if got, want := manager.healthPollFloor, defaultHealthPollFloor; got != want {
		t.Fatalf("healthPollFloor = %v, want %v", got, want)
	}
	if got, want := manager.healthPollCeiling, defaultHealthPollCeiling; got != want {
		t.Fatalf("healthPollCeiling = %v, want %v", got, want)
	}
	if got, want := manager.subprocessSignalGrace, defaultSubprocessSignalGrace; got != want {
		t.Fatalf("subprocessSignalGrace = %v, want %v", got, want)
	}
	if _, ok := manager.hostMethods["sessions/list"]; !ok {
		t.Fatalf("hostMethods = %#v, want trimmed sessions/list key", manager.hostMethods)
	}
}

func TestManagerReloadValidatesAndRestarts(t *testing.T) {
	t.Parallel()

	t.Run("Should reject nil manager", func(t *testing.T) {
		t.Parallel()

		var nilManager *Manager
		if err := nilManager.Reload(testutil.Context(t)); !errors.Is(err, ErrManagerRequired) {
			t.Fatalf("nil manager Reload() error = %v, want %v", err, ErrManagerRequired)
		}
	})

	t.Run("Should reject canceled context", func(t *testing.T) {
		t.Parallel()

		manager := NewManager(nil)
		ctx, cancel := context.WithCancel(testutil.Context(t))
		cancel()
		if err := manager.Reload(ctx); !errors.Is(err, context.Canceled) {
			t.Fatalf("Reload(canceled context) error = %v, want %v", err, context.Canceled)
		}
	})

	t.Run("Should reject missing registry", func(t *testing.T) {
		t.Parallel()

		manager := NewManager(nil)
		err := manager.Reload(testutil.Context(t))
		if !errors.Is(err, ErrRegistryRequired) {
			t.Fatalf("Reload() error = %v, want %v", err, ErrRegistryRequired)
		}
	})

	t.Run("Should restart loaded extensions", func(t *testing.T) {
		t.Parallel()

		withDaemonVersion(t, "0.5.0")
		env := newRegistryTestEnv(t)
		fixture := createManagerTestExtension(t, managerTestManifest("ext-reload", managerManifestOptions{
			command:      "fake-extension",
			capabilities: []string{"memory.backend"},
			actions:      []string{"sessions/list"},
			security:     []string{"session.read"},
		}), nil)
		installManagerFixture(t, env.registry, fixture, SourceUser, true)

		firstProc := newFakeProcess(101)
		secondProc := newFakeProcess(202)
		launcher := &fakeLauncher{queue: []*fakeProcess{firstProc, secondProc}}
		startedManager := NewManager(
			env.registry,
			withProcessLauncher(launcher.launch),
			withHealthPollBounds(time.Millisecond, 2*time.Millisecond),
		)
		if err := startedManager.Start(testutil.Context(t)); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		t.Cleanup(func() {
			if err := startedManager.Stop(testutil.Context(t)); err != nil {
				t.Fatalf("Stop() cleanup error = %v", err)
			}
		})

		if got, want := launcher.launchCount(), 1; got != want {
			t.Fatalf("launch count after Start() = %d, want %d", got, want)
		}

		if err := startedManager.Reload(testutil.Context(t)); err != nil {
			t.Fatalf("Reload(started manager) error = %v", err)
		}

		if got, want := launcher.launchCount(), 2; got != want {
			t.Fatalf("launch count after Reload() = %d, want %d", got, want)
		}
		if got, want := firstProc.shutdownCnt, 1; got != want {
			t.Fatalf("first process shutdown count = %d, want %d", got, want)
		}
		if got, want := len(secondProc.initRequests()), 1; got != want {
			t.Fatalf("len(second process initialize requests) = %d, want %d", got, want)
		}
		statuses := startedManager.Statuses()
		if got, want := len(statuses), 1; got != want {
			t.Fatalf("len(Statuses()) = %d, want %d", got, want)
		}
		if got, want := statuses[0].PID, 202; got != want {
			t.Fatalf("Statuses()[0].PID = %d, want %d after reload restart", got, want)
		}
	})

	t.Run("Should serialize overlapping reloads", func(t *testing.T) {
		t.Parallel()

		withDaemonVersion(t, "0.5.0")
		env := newRegistryTestEnv(t)
		fixture := createManagerTestExtension(t, managerTestManifest("ext-reload-serialized", managerManifestOptions{
			command:      "fake-extension",
			capabilities: []string{"memory.backend"},
			actions:      []string{"sessions/list"},
			security:     []string{"session.read"},
		}), nil)
		installManagerFixture(t, env.registry, fixture, SourceUser, true)

		secondInitializeStarted := make(chan struct{})
		releaseSecondInitialize := make(chan struct{})
		var releaseOnce sync.Once
		t.Cleanup(func() {
			releaseOnce.Do(func() { close(releaseSecondInitialize) })
		})

		firstProc := newFakeProcess(301)
		secondProc := newFakeProcess(302)
		secondProc.initHook = func() {
			close(secondInitializeStarted)
			<-releaseSecondInitialize
		}
		thirdProc := newFakeProcess(303)
		launcher := &fakeLauncher{queue: []*fakeProcess{firstProc, secondProc, thirdProc}}
		thirdLaunchStarted := make(chan struct{})
		var thirdLaunchOnce sync.Once
		launch := func(ctx context.Context, cfg subprocess.LaunchConfig) (processHandle, error) {
			process, err := launcher.launch(ctx, cfg)
			if launcher.launchCount() >= 3 {
				thirdLaunchOnce.Do(func() { close(thirdLaunchStarted) })
			}
			return process, err
		}

		manager := NewManager(
			env.registry,
			withProcessLauncher(launch),
			withHealthPollBounds(time.Millisecond, 2*time.Millisecond),
		)
		if err := manager.Start(testutil.Context(t)); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		t.Cleanup(func() {
			if err := manager.Stop(testutil.Context(t)); err != nil {
				t.Fatalf("Stop() cleanup error = %v", err)
			}
		})

		firstReloadErr := make(chan error, 1)
		go func() {
			firstReloadErr <- manager.Reload(testutil.Context(t))
		}()
		select {
		case <-secondInitializeStarted:
		case <-time.After(time.Second):
			t.Fatal("first Reload() did not reach the second process initialization")
		}

		secondReloadStarted := make(chan struct{})
		secondReloadErr := make(chan error, 1)
		go func() {
			close(secondReloadStarted)
			secondReloadErr <- manager.Reload(testutil.Context(t))
		}()
		<-secondReloadStarted

		select {
		case <-thirdLaunchStarted:
			t.Fatal("overlapping Reload() launched a third process before the first reload completed")
		case <-time.After(100 * time.Millisecond):
		}

		releaseOnce.Do(func() { close(releaseSecondInitialize) })
		for index, errCh := range []<-chan error{firstReloadErr, secondReloadErr} {
			select {
			case err := <-errCh:
				if err != nil {
					t.Fatalf("Reload() call %d error = %v", index+1, err)
				}
			case <-time.After(time.Second):
				t.Fatalf("Reload() call %d did not complete", index+1)
			}
		}

		statuses := manager.Statuses()
		if got, want := len(statuses), 1; got != want {
			t.Fatalf("len(Statuses()) = %d, want %d", got, want)
		}
		if got, want := statuses[0].PID, 303; got != want {
			t.Fatalf("Statuses()[0].PID = %d, want %d after serialized reloads", got, want)
		}
	})
}

func TestManagerHelperPathsAndAccessors(t *testing.T) {
	t.Parallel()

	withDaemonVersion(t, "0.5.0")
	env := newRegistryTestEnv(t)
	fixture := createManagerTestExtension(t, managerTestManifest("ext-fallback", managerManifestOptions{
		command:      "fake-extension",
		capabilities: []string{"memory.backend"},
		actions:      []string{"sessions/list"},
		security:     []string{"session.read"},
	}), nil)
	installManagerFixture(t, env.registry, fixture, SourceUser, true)

	manager := NewManager(env.registry)

	fallback, err := manager.Get("ext-fallback")
	if err != nil {
		t.Fatalf("Get(ext-fallback) error = %v", err)
	}
	if fallback.Info.Name != "ext-fallback" || !fallback.Status.Enabled {
		t.Fatalf("Get(ext-fallback) = %#v, want registry-backed enabled snapshot", fallback)
	}
	if _, err := manager.Get(" "); err == nil {
		t.Fatal("Get(empty) error = nil, want validation error")
	}

	now := time.Now().UTC()
	proc := newFakeProcess(707)
	proc.health = subprocess.HealthState{
		Healthy:       true,
		Message:       "healthy",
		LastCheckedAt: now,
	}

	manager.extensions = map[string]*managedExtension{
		"zeta": {
			info:                ExtensionInfo{Name: "zeta", Version: "1.0.0", Source: SourceUser, Enabled: true},
			process:             proc,
			active:              true,
			registered:          true,
			phase:               ExtensionPhaseActivate,
			generation:          2,
			awaitingStability:   true,
			consecutiveFailures: 2,
			restartBackoff:      2 * time.Second,
			healthInterval:      40 * time.Millisecond,
			runtime:             subprocess.InitializeRuntime{ShutdownTimeoutMS: 321},
		},
		"alpha": {
			info:       ExtensionInfo{Name: "alpha", Version: "0.1.0", Source: SourceMarketplace, Enabled: false},
			registered: false,
			phase:      ExtensionPhaseDiscover,
		},
	}

	listed := manager.List()
	if got := []string{listed[0].Name, listed[1].Name}; !slices.Equal(got, []string{"alpha", "zeta"}) {
		t.Fatalf("List() names = %#v, want sorted alpha/zeta", got)
	}

	statuses := manager.Statuses()
	if got := []string{statuses[0].Name, statuses[1].Name}; !slices.Equal(got, []string{"alpha", "zeta"}) {
		t.Fatalf("Statuses() names = %#v, want sorted alpha/zeta", got)
	}
	if statuses[1].PID != 707 || !statuses[1].Healthy || statuses[1].HealthMessage != "healthy" {
		t.Fatalf("Statuses()[1] = %#v, want pid/health from process", statuses[1])
	}

	manager.markStable("zeta", 2)
	if manager.extensions["zeta"].awaitingStability {
		t.Fatal("awaitingStability = true, want false after markStable")
	}
	if manager.extensions["zeta"].consecutiveFailures != 0 || manager.extensions["zeta"].restartBackoff != 0 {
		t.Fatalf(
			"post-markStable failures/backoff = %d/%v, want 0/0",
			manager.extensions["zeta"].consecutiveFailures,
			manager.extensions["zeta"].restartBackoff,
		)
	}

	current, interval, ok := manager.currentProcess("zeta", 2)
	if !ok || current == nil || current.PID() != 707 || interval != 40*time.Millisecond {
		t.Fatalf("currentProcess() = (%v, %v, %v), want process pid 707 and interval 40ms", current, interval, ok)
	}
	if manager.shouldStopSupervision("zeta", 2, proc) {
		t.Fatal("shouldStopSupervision() = true, want false for current process")
	}
	manager.stopping = true
	if !manager.shouldStopSupervision("zeta", 2, proc) {
		t.Fatal("shouldStopSupervision() = false, want true while stopping")
	}
	manager.stopping = false

	if got, want := manager.shutdownDeadlineForProcess("zeta", 2), 321*time.Millisecond; got != want {
		t.Fatalf("shutdownDeadlineForProcess() = %v, want %v", got, want)
	}
	if got, want := manager.healthPollInterval(0), manager.healthPollCeiling; got != want {
		t.Fatalf("healthPollInterval(0) = %v, want %v", got, want)
	}
	if got, want := manager.healthPollInterval(time.Millisecond), manager.healthPollFloor; got != want {
		t.Fatalf("healthPollInterval(1ms) = %v, want %v", got, want)
	}

	select {
	case <-manager.lifecycleDone():
	default:
		t.Fatal("lifecycleDone() returned open bridge, want closed bridge when lifecycle is nil")
	}
	if manager.lifecycleContext().Done() != nil {
		t.Fatal("lifecycleContext() returned cancellable context, want background context")
	}

	if got := restartBackoff(0, 5*time.Second); got != 0 {
		t.Fatalf("restartBackoff(0) = %v, want 0", got)
	}
	if got := restartBackoff(10, 5*time.Second); got != 5*time.Second {
		t.Fatalf("restartBackoff(10) = %v, want capped 5s", got)
	}

	if !requiresSubprocess(&Manifest{Actions: ActionsConfig{Requires: []string{"sessions/list"}}}) {
		t.Fatal("requiresSubprocess(actions-only) = false, want true")
	}
	if !requiresSubprocess(
		&Manifest{Resources: ResourcesConfig{Publish: ResourceGrantRequest{Families: []string{"tools"}}}},
	) {
		t.Fatal("requiresSubprocess(resource-publish) = false, want true")
	}
	if requiresSubprocess(&Manifest{}) {
		t.Fatal("requiresSubprocess(empty manifest) = true, want false")
	}
	if got, want := skillSourceForExtension(SourceWorkspace), skillspkg.SourceWorkspace; got != want {
		t.Fatalf("skillSourceForExtension(workspace) = %v, want %v", got, want)
	}
	if got, want := skillSourceForExtension(ExtensionSource(99)), skillspkg.SourceUser; got != want {
		t.Fatalf("skillSourceForExtension(default) = %v, want %v", got, want)
	}
	if _, err := loadManifestAtPath(filepath.Join(t.TempDir(), "extension.txt")); err == nil {
		t.Fatal("loadManifestAtPath(.txt) error = nil, want unsupported-extension error")
	}
}

func TestManagerResolveCommandKeepsPathLikeValuesInsideExtensionRoot(t *testing.T) {
	t.Parallel()

	manager := NewManager(nil)
	root := t.TempDir()
	inside := filepath.Join(root, "bin", "tool")

	got, err := manager.resolveCommand(root, "./bin/tool")
	if err != nil {
		t.Fatalf("resolveCommand(relative) error = %v", err)
	}
	if got != inside {
		t.Fatalf("resolveCommand(relative) = %q, want %q", got, inside)
	}

	got, err = manager.resolveCommand(root, inside)
	if err != nil {
		t.Fatalf("resolveCommand(absolute inside) error = %v", err)
	}
	if got != inside {
		t.Fatalf("resolveCommand(absolute inside) = %q, want %q", got, inside)
	}

	outsideCommand := filepath.Join(t.TempDir(), "tool")
	got, err = manager.resolveCommand(root, outsideCommand)
	if err != nil {
		t.Fatalf("resolveCommand(absolute outside) error = %v", err)
	}
	if got != outsideCommand {
		t.Fatalf("resolveCommand(absolute outside) = %q, want %q", got, outsideCommand)
	}

	got, err = manager.resolveCommand(root, "node")
	if err != nil {
		t.Fatalf("resolveCommand(bare) error = %v", err)
	}
	if got != "node" {
		t.Fatalf("resolveCommand(bare) = %q, want %q", got, "node")
	}

	if _, err := manager.resolveCommand(root, "../outside/tool"); !errors.Is(err, ErrPathEscapesExtensionRoot) {
		t.Fatalf("resolveCommand(escape) error = %v, want %v", err, ErrPathEscapesExtensionRoot)
	}

	if _, err := resolveResourcePath(root, "../skills"); !errors.Is(err, ErrPathEscapesExtensionRoot) {
		t.Fatalf("resolveResourcePath(escape) error = %v, want %v", err, ErrPathEscapesExtensionRoot)
	}

	resourceRoot, err := resolveResourcePath(root, "skills")
	if err != nil {
		t.Fatalf("resolveResourcePath(within root) error = %v", err)
	}
	if resourceRoot != filepath.Join(root, "skills") {
		t.Fatalf("resolveResourcePath(within root) = %q, want %q", resourceRoot, filepath.Join(root, "skills"))
	}
}

func TestManagerResolveEnvMapUsesSafeBaselineOnly(t *testing.T) {
	t.Parallel()

	manager := NewManager(nil, WithGetenv(func(key string) string {
		switch key {
		case "PATH":
			return "/usr/bin:/bin"
		case "HOME":
			return "/tmp/home"
		case "LANG":
			return "en_US.UTF-8"
		case "SECRET_TOKEN":
			return "top-secret"
		default:
			return ""
		}
	}))

	env, cleanups, err := manager.resolveEnvMap(context.Background(), t.TempDir(), map[string]string{
		"APP_MODE": "sandbox",
		"PATH":     "/custom/bin",
	}, nil)
	if err != nil {
		t.Fatalf("resolveEnvMap() error = %v", err)
	}
	t.Cleanup(func() { runExtensionRedactionCleanups(cleanups) })

	decoded := envListToMap(t, env)
	if decoded["PATH"] != "/custom/bin" {
		t.Fatalf("PATH = %q, want %q", decoded["PATH"], "/custom/bin")
	}
	if decoded["HOME"] != "/tmp/home" {
		t.Fatalf("HOME = %q, want %q", decoded["HOME"], "/tmp/home")
	}
	if decoded["LANG"] != "en_US.UTF-8" {
		t.Fatalf("LANG = %q, want %q", decoded["LANG"], "en_US.UTF-8")
	}
	if decoded["APP_MODE"] != "sandbox" {
		t.Fatalf("APP_MODE = %q, want %q", decoded["APP_MODE"], "sandbox")
	}
	if _, ok := decoded["SECRET_TOKEN"]; ok {
		t.Fatalf("resolveEnvMap() leaked SECRET_TOKEN in %#v", decoded)
	}
}

func TestManagerCloneExtensionReturnsIsolatedSnapshot(t *testing.T) {
	t.Parallel()

	manager := NewManager(nil)
	ext := &managedExtension{
		info: ExtensionInfo{
			Name:    "snapshot",
			Version: "1.0.0",
			Source:  SourceUser,
			Enabled: true,
			Capabilities: CapabilitiesConfig{
				Provides: []string{"memory.backend"},
			},
			Actions: ActionsConfig{
				Requires: []string{"sessions/list"},
			},
		},
		manifest: &Manifest{
			Name:    "snapshot",
			Version: "1.0.0",
			Resources: ResourcesConfig{
				Skills: []string{"skills/"},
				Publish: ResourceGrantRequest{
					Families: []string{"tools"},
					MaxScope: resources.ResourceScopeKindWorkspace,
				},
			},
			Capabilities: CapabilitiesConfig{
				Provides: []string{"memory.backend"},
			},
			Actions: ActionsConfig{
				Requires: []string{"sessions/list"},
			},
			Subprocess: SubprocessConfig{
				Command: "snapshot-extension",
				Args:    []string{"--config", "snapshot.toml"},
				Env: map[string]string{
					"TOKEN": "value",
				},
			},
			Security: SecurityConfig{
				Capabilities: []string{"memory.read"},
			},
		},
		skills: []*skillspkg.Skill{{
			Meta: skillspkg.SkillMeta{
				Name:        "snapshot-skill",
				Description: "Snapshot skill",
				Metadata: map[string]any{
					"nested": map[string]any{"value": "original"},
					"list":   []any{"original", map[string]any{"child": "value"}},
				},
			},
			Hooks: []hookspkg.HookDecl{{
				Name:      "snapshot-hook",
				Args:      []string{"cleanup"},
				Env:       map[string]string{"PHASE": "stop"},
				SecretEnv: map[string]string{"TOKEN": "vault:hooks/snapshot-hook/token"},
			}},
			MCPServers: []skillspkg.MCPServerDecl{{
				Name:    "snapshot-server",
				Command: "server",
				Args:    []string{"--once"},
				Env:     map[string]string{"ROOT": "/tmp/original"},
			}},
			Provenance: &skillspkg.Provenance{
				Hash: "hash-original",
			},
		}},
		bundles: []BundleSpec{{
			Name:        "bundle-one",
			Description: "Original bundle",
			Profiles: []BundleProfile{{
				Name:        "default",
				Description: "Default profile",
				Channels: BundleChannelsConfig{
					Primary: "primary",
					Items:   []BundleChannel{{Name: "primary", Description: "Primary"}},
				},
				Layouts: []BundleLayout{{
					Path:   "layouts/two-up.json",
					Layout: testBundleLayoutResource("two-up"),
				}},
				Agents: []BundleAgent{{
					Path: "agents/coder",
					Agent: aghconfig.AgentDef{
						Name:       "coder",
						Provider:   "codex",
						Tools:      []string{"read"},
						Prompt:     "Run bundle work.",
						SourcePath: "/extension/agents/coder/AGENT.md",
					},
					Soul: &BundleAgentSidecar{
						SourcePath: "agents/coder/SOUL.md",
						Body:       "Original soul.",
					},
				}},
				Jobs: []BundleJob{{
					Name:      "job-one",
					AgentName: "coder",
					Prompt:    "do work",
					Task: &automationpkg.JobTaskConfig{
						Title: "Job task",
					},
					Enabled: true,
				}},
				Bridges: []BundleBridgePreset{{
					Name:        "bridge-one",
					DisplayName: "Bridge",
					RoutingPolicy: bridgepkg.RoutingPolicy{
						IncludePeer: true,
					},
				}},
			}},
		}},
		grantedResourceKinds:  []resources.ResourceKind{resources.ResourceKind("tool")},
		grantedResourceScopes: []resources.ResourceScopeKind{resources.ResourceScopeKindWorkspace},
		initialize: &subprocess.InitializeResponse{
			ImplementedMethods:  []string{"shutdown"},
			SupportedHookEvents: []string{"turn.start"},
			AcceptedCapabilities: subprocess.AcceptedCapabilities{
				Provides: []string{"memory.backend"},
				Actions:  []extensionprotocol.HostAPIMethod{"sessions/list"},
				Security: []string{"memory.read"},
			},
		},
	}

	clone := manager.cloneExtension(ext)
	if clone == nil {
		t.Fatal("cloneExtension() = nil, want snapshot")
		return
	}

	clone.Info.Capabilities.Provides[0] = "changed"
	clone.Info.Actions.Requires[0] = "changed"
	clone.Manifest.Resources.Skills[0] = "changed"
	clone.Manifest.Subprocess.Env["TOKEN"] = "changed"
	clone.Manifest.Resources.Publish.Families[0] = "changed"
	clone.Skills[0].Meta.Name = "changed"
	clone.Skills[0].Meta.Metadata["nested"].(map[string]any)["value"] = "changed"
	clone.Skills[0].Meta.Metadata["list"].([]any)[0] = "changed"
	clone.Skills[0].Meta.Metadata["list"].([]any)[1].(map[string]any)["child"] = "changed"
	clone.Skills[0].Hooks[0].Args[0] = "changed"
	clone.Skills[0].Hooks[0].SecretEnv["TOKEN"] = "vault:hooks/snapshot-hook/changed"
	clone.Skills[0].MCPServers[0].Env["ROOT"] = "/tmp/changed"
	clone.Skills[0].Provenance.Hash = "hash-changed"
	clone.Bundles[0].Profiles[0].Layouts[0].Layout.ParticipantSlots[0] = "changed"
	clone.Bundles[0].Profiles[0].Agents[0].Agent.Tools[0] = "changed"
	clone.Bundles[0].Profiles[0].Agents[0].Soul.Body = "Changed soul."
	clone.Bundles[0].Profiles[0].Jobs[0].Task.Title = "changed"
	clone.Bundles[0].Profiles[0].Bridges[0].DisplayName = "changed"
	clone.GrantedResourceKinds[0] = resources.ResourceKind("changed")
	clone.GrantedResourceScopes[0] = resources.ResourceScopeKindGlobal
	clone.InitializeResult.ImplementedMethods[0] = "changed"
	clone.InitializeResult.AcceptedCapabilities.Provides[0] = "changed"

	if ext.info.Capabilities.Provides[0] != "memory.backend" {
		t.Fatalf("original capabilities mutated to %#v", ext.info.Capabilities.Provides)
	}
	if ext.info.Actions.Requires[0] != "sessions/list" {
		t.Fatalf("original actions mutated to %#v", ext.info.Actions.Requires)
	}
	if ext.manifest.Resources.Skills[0] != "skills/" {
		t.Fatalf("original manifest resources mutated to %#v", ext.manifest.Resources.Skills)
	}
	if ext.manifest.Resources.Publish.Families[0] != "tools" {
		t.Fatalf("original manifest publish request mutated to %#v", ext.manifest.Resources.Publish)
	}
	if ext.manifest.Subprocess.Env["TOKEN"] != "value" {
		t.Fatalf("original manifest env mutated to %#v", ext.manifest.Subprocess.Env)
	}
	if ext.skills[0].Meta.Name != "snapshot-skill" {
		t.Fatalf("original skill name mutated to %q", ext.skills[0].Meta.Name)
	}
	if ext.skills[0].Meta.Metadata["nested"].(map[string]any)["value"] != "original" {
		t.Fatalf("original skill metadata mutated to %#v", ext.skills[0].Meta.Metadata)
	}
	if ext.skills[0].Meta.Metadata["list"].([]any)[0] != "original" {
		t.Fatalf("original skill metadata list mutated to %#v", ext.skills[0].Meta.Metadata["list"])
	}
	if ext.skills[0].Hooks[0].Args[0] != "cleanup" {
		t.Fatalf("original skill hook args mutated to %#v", ext.skills[0].Hooks[0].Args)
	}
	if got, want := ext.skills[0].Hooks[0].SecretEnv["TOKEN"], "vault:hooks/snapshot-hook/token"; got != want {
		t.Fatalf("original skill hook secret env mutated to %q, want %q", got, want)
	}
	if ext.skills[0].MCPServers[0].Env["ROOT"] != "/tmp/original" {
		t.Fatalf("original skill MCP env mutated to %#v", ext.skills[0].MCPServers[0].Env)
	}
	if ext.skills[0].Provenance.Hash != "hash-original" {
		t.Fatalf("original skill provenance mutated to %#v", ext.skills[0].Provenance)
	}
	if got, want := ext.bundles[0].Profiles[0].Layouts[0].Layout.ParticipantSlots[0],
		windowmanager.WindowID("primary"); got != want {
		t.Fatalf("original bundle layout participant slot = %q, want %q", got, want)
	}
	if got := ext.bundles[0].Profiles[0].Agents[0].Agent.Tools[0]; got != "read" {
		t.Fatalf("original bundle agent tools mutated to %q", got)
	}
	if got := ext.bundles[0].Profiles[0].Agents[0].Soul.Body; got != "Original soul." {
		t.Fatalf("original bundle agent soul mutated to %q", got)
	}
	if ext.bundles[0].Profiles[0].Jobs[0].Task.Title != "Job task" {
		t.Fatalf("original bundle job task mutated to %#v", ext.bundles[0].Profiles[0].Jobs[0].Task)
	}
	if ext.bundles[0].Profiles[0].Bridges[0].DisplayName != "Bridge" {
		t.Fatalf("original bundle bridge mutated to %#v", ext.bundles[0].Profiles[0].Bridges[0])
	}
	if ext.grantedResourceKinds[0] != resources.ResourceKind("tool") {
		t.Fatalf("original granted resource kinds mutated to %#v", ext.grantedResourceKinds)
	}
	if ext.grantedResourceScopes[0] != resources.ResourceScopeKindWorkspace {
		t.Fatalf("original granted resource scopes mutated to %#v", ext.grantedResourceScopes)
	}
	if ext.initialize.ImplementedMethods[0] != "shutdown" {
		t.Fatalf("original initialize methods mutated to %#v", ext.initialize.ImplementedMethods)
	}
	if ext.initialize.AcceptedCapabilities.Provides[0] != "memory.backend" {
		t.Fatalf("original initialize provides mutated to %#v", ext.initialize.AcceptedCapabilities.Provides)
	}
}

func TestManagerBridgeRuntimeIssueHelpers(t *testing.T) {
	t.Parallel()

	sink := &recordingBridgeTelemetrySink{}
	manager := NewManager(nil, WithBridgeTelemetrySink(sink))

	manager.reportBridgeRuntimeIssue("  bridge-1  ", bridgepkg.BridgeStatusDegraded, errors.New("boom"))
	manager.reportBridgeRuntimeIssue("", bridgepkg.BridgeStatusReady, errors.New("ignored"))
	manager.reportBridgeRuntimeIssues(
		[]string{"bridge-2", "bridge-3"},
		bridgepkg.BridgeStatusError,
		errors.New("failed"),
	)
	manager.clearBridgeRuntimeIssue("  bridge-1  ")
	manager.clearBridgeRuntimeIssues([]string{"bridge-2", "bridge-3"})

	if len(sink.issues) != 3 {
		t.Fatalf("len(issues) = %d, want 3", len(sink.issues))
	}
	if len(sink.clears) != 3 {
		t.Fatalf("len(clears) = %d, want 3", len(sink.clears))
	}
	if sink.issues[0] != "bridge-1:degraded:boom" {
		t.Fatalf("issues[0] = %q, want bridge-1 degraded entry", sink.issues[0])
	}
}

func TestManagerDirectPhaseAndMonitorBranches(t *testing.T) {
	t.Parallel()

	withDaemonVersion(t, "0.5.0")
	manager := NewManager(nil, WithGetenv(func(key string) string {
		if key == "EXT_MODE" {
			return "resolved-mode"
		}
		return ""
	}))

	discover := &managedExtension{info: ExtensionInfo{Name: "ext-discover"}}
	if err := manager.discoverExtension(discover); err == nil {
		t.Fatal("discoverExtension() error = nil, want missing manifest path")
	}
	discover.info.ManifestPath = manifestTOMLFileName
	if err := manager.discoverExtension(discover); err == nil {
		t.Fatal("discoverExtension() error = nil, want invalid relative manifest path")
	}

	resolved, err := manager.resolveString("/tmp/ext", "{{config_dir}}/{{env:EXT_MODE}}")
	if err != nil {
		t.Fatalf("resolveString() error = %v", err)
	}
	if got, want := resolved, "/tmp/ext/resolved-mode"; got != want {
		t.Fatalf("resolveString() = %q, want %q", got, want)
	}
	if _, err := manager.resolveString("/tmp/ext", "{{env:EXT_TOKEN"); err == nil {
		t.Fatal("resolveString(invalid template) error = nil, want template error")
	}

	validate := &managedExtension{info: ExtensionInfo{Name: "ext-validate", Source: SourceUser}}
	if err := manager.validateExtension(validate); err == nil {
		t.Fatal("validateExtension(nil manifest) error = nil, want manifest-required error")
	}
	validate.manifest = &Manifest{Name: "other", Version: "1.0.0", MinAGHVersion: "0.5.0"}
	if err := manager.validateExtension(validate); err == nil {
		t.Fatal("validateExtension(name mismatch) error = nil, want mismatch error")
	}
	validate.info.Version = "1.0.0"
	validate.manifest = &Manifest{Name: "ext-validate", Version: "2.0.0", MinAGHVersion: "0.5.0"}
	if err := manager.validateExtension(validate); err == nil {
		t.Fatal("validateExtension(version mismatch) error = nil, want mismatch error")
	}
	validate.info.Version = ""
	validate.manifest = &Manifest{
		Name:          "ext-validate",
		Version:       "1.0.0",
		MinAGHVersion: "0.5.0",
		Actions:       ActionsConfig{Requires: []string{"sessions/list"}},
	}
	if err := manager.validateExtension(validate); err == nil {
		t.Fatal("validateExtension(missing subprocess command) error = nil, want subprocess validation error")
	}

	lite := &managedExtension{
		info: ExtensionInfo{Name: "ext-lite", Source: SourceUser, Enabled: true},
		manifest: &Manifest{
			Name:          "ext-lite",
			Version:       "1.0.0",
			MinAGHVersion: "0.5.0",
		},
	}
	if err := manager.validateExtension(lite); err != nil {
		t.Fatalf("validateExtension(lite) error = %v", err)
	}
	if err := manager.initializeExtension(context.Background(), lite); err != nil {
		t.Fatalf("initializeExtension(lite) error = %v", err)
	}
	manager.activateExtension(lite)
	if !lite.active || lite.phase != ExtensionPhaseActivate {
		t.Fatalf("lite extension after activate = %#v, want active activate-phase extension", lite)
	}

	rootDir := t.TempDir()
	skillsDir := filepath.Join(rootDir, "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", skillsDir, err)
	}
	writeFile(t, filepath.Join(skillsDir, "skill.md"), managerSkillFile("missing-registry", "Needs registry"))
	skillExt := &managedExtension{
		info:    ExtensionInfo{Name: "ext-skills", Source: SourceUser},
		rootDir: rootDir,
		manifest: &Manifest{
			Name:          "ext-skills",
			Version:       "1.0.0",
			MinAGHVersion: "0.5.0",
			Resources: ResourcesConfig{
				Skills: []string{"skills"},
			},
		},
	}
	if err := manager.registerExtension(context.Background(), skillExt); err != nil {
		t.Fatalf("registerExtension(skills) error = %v", err)
	}
	if len(skillExt.skills) != 1 || skillExt.skills[0].Meta.Name != "missing-registry" {
		t.Fatalf("registerExtension(skills).skills = %#v, want loaded extension skill snapshot", skillExt.skills)
	}

	manager.capChecker.Register("ext-host", SourceUser, &Manifest{
		Actions:  ActionsConfig{Requires: []string{"sessions/list"}},
		Security: SecurityConfig{Capabilities: []string{"session.read"}},
	})
	allowed := manager.wrapHostHandler(
		"ext-host",
		"sessions/list",
		nil,
		nil,
		func(_ context.Context, _ json.RawMessage) (any, error) {
			return "ok", nil
		},
	)
	result, err := allowed(context.Background(), json.RawMessage(`{}`))
	if err != nil || result != "ok" {
		t.Fatalf("wrapHostHandler allowed call = (%v, %v), want (ok, nil)", result, err)
	}

	resourceSession := &hostAPIResourceSession{
		Actor: resources.MutationActor{
			Kind:         resources.MutationActorKindExtension,
			ID:           "ext-host",
			SessionNonce: "nonce-wrap",
			Source: resources.ResourceSource{
				Kind: resources.ResourceSourceKind("extension"),
				ID:   "ext-host",
			},
			MaxScope:      resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
			GrantedKinds:  []resources.ResourceKind{"tool.definition"},
			GrantedScopes: []resources.ResourceScopeKind{resources.ResourceScopeKindGlobal},
		},
	}
	bridgeRuntime := &subprocess.InitializeBridgeRuntime{
		ManagedInstances: []subprocess.InitializeBridgeManagedInstance{
			{
				Instance: bridgepkg.BridgeInstanceToContract(bridgepkg.BridgeInstance{
					ID:            "brg-wrap",
					ExtensionName: "ext-host",
				}),
			},
		},
	}
	injected := manager.wrapHostHandler(
		"ext-host",
		"sessions/list",
		bridgeRuntime,
		resourceSession,
		func(ctx context.Context, _ json.RawMessage) (any, error) {
			if got := hostAPIExtensionNameFromContext(ctx); got != "ext-host" {
				t.Fatalf("hostAPIExtensionNameFromContext(ctx) = %q, want ext-host", got)
			}
			runtime := hostAPIBridgeRuntimeFromContext(ctx)
			if runtime == nil {
				t.Fatal("hostAPIBridgeRuntimeFromContext(ctx) = nil, want runtime")
				return nil, nil
			}
			if got := runtime.ManagedInstances[0].Instance.ID; got != "brg-wrap" {
				t.Fatalf("runtime.ManagedInstances[0].Instance.ID = %q, want brg-wrap", got)
			}
			session, ok := hostAPIResourceSessionFromContext(ctx)
			if !ok {
				t.Fatal("hostAPIResourceSessionFromContext(ctx) = false, want true")
			}
			if got := session.Actor.SessionNonce; got != "nonce-wrap" {
				t.Fatalf("session.Actor.SessionNonce = %q, want nonce-wrap", got)
			}
			return "injected", nil
		},
	)
	result, err = injected(context.Background(), json.RawMessage(`{}`))
	if err != nil || result != "injected" {
		t.Fatalf("wrapHostHandler injected call = (%v, %v), want (injected, nil)", result, err)
	}

	denied := manager.wrapHostHandler(
		"ext-denied",
		"sessions/list",
		nil,
		nil,
		func(_ context.Context, _ json.RawMessage) (any, error) {
			return "never", nil
		},
	)
	if _, err := denied(context.Background(), nil); err == nil {
		t.Fatal("wrapHostHandler denied call error = nil, want capability denial")
	}

	monitorManager := NewManager(nil, withHealthPollBounds(time.Millisecond, time.Millisecond))
	monitorCtx := t.Context()
	monitorManager.lifecycleCtx = monitorCtx
	unhealthy := newFakeProcess(808)
	unhealthy.health = subprocess.HealthState{
		Healthy:       false,
		Message:       "probe failed",
		LastError:     "rpc timeout",
		LastCheckedAt: time.Now().UTC(),
	}
	monitorManager.extensions = map[string]*managedExtension{
		"ext-health": {
			info:           ExtensionInfo{Name: "ext-health"},
			process:        unhealthy,
			generation:     1,
			healthInterval: 4 * time.Millisecond,
			runtime:        subprocess.InitializeRuntime{ShutdownTimeoutMS: 7},
		},
	}

	shouldRecover, reason := monitorManager.monitorProcess("ext-health", 1, unhealthy, 4*time.Millisecond)
	if !shouldRecover {
		t.Fatal("monitorProcess() shouldRecover = false, want true for unhealthy process")
	}
	if reason == nil || !strings.Contains(reason.Error(), "health check failed") ||
		!strings.Contains(reason.Error(), "rpc timeout") {
		t.Fatalf("monitorProcess() reason = %v, want joined health failure detail", reason)
	}
	if unhealthy.shutdownCnt != 1 {
		t.Fatalf("Shutdown count = %d, want 1 after unhealthy process", unhealthy.shutdownCnt)
	}
}

type managerFixture struct {
	dir      string
	manifest *Manifest
	checksum string
}

type managerManifestOptions struct {
	command           string
	args              []string
	withEnv           map[string]string
	withSkills        bool
	withAgents        bool
	withHooks         bool
	withMCP           bool
	resourceFamilies  []string
	resourceMaxScope  string
	minVersion        string
	capabilities      []string
	actions           []string
	security          []string
	bridgePlatform    string
	bridgeDisplayName string
	shutdown          time.Duration
}

type fakeLauncher struct {
	mu     sync.Mutex
	queue  []*fakeProcess
	config []subprocess.LaunchConfig
}

func (l *fakeLauncher) launch(_ context.Context, cfg subprocess.LaunchConfig) (processHandle, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.config = append(l.config, cfg)
	if len(l.queue) == 0 {
		return nil, errors.New("no fake processes queued")
	}
	process := l.queue[0]
	l.queue = l.queue[1:]
	return process, nil
}

func (l *fakeLauncher) launchCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.config)
}

type fakeProcess struct {
	mu          sync.Mutex
	pid         int
	done        chan struct{}
	waitErr     error
	closed      bool
	initReqs    []subprocess.InitializeRequest
	initResp    subprocess.InitializeResponse
	initErr     error
	initHook    func()
	health      subprocess.HealthState
	handlers    map[string]subprocess.HandlerFunc
	callFn      func(context.Context, string, any, any) error
	shutdownFn  func(context.Context) error
	shutdownCnt int
}

func newFakeProcess(pid int) *fakeProcess {
	return &fakeProcess{
		pid:      pid,
		done:     make(chan struct{}),
		handlers: make(map[string]subprocess.HandlerFunc),
		health: subprocess.HealthState{
			Healthy: true,
		},
	}
}

func (p *fakeProcess) HandleMethod(method string, handler subprocess.HandlerFunc) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handlers[method] = handler
	return nil
}

func (p *fakeProcess) Initialize(
	_ context.Context,
	req subprocess.InitializeRequest,
) (subprocess.InitializeResponse, error) {
	p.mu.Lock()
	p.initReqs = append(p.initReqs, req)
	resp := p.initResp
	err := p.initErr
	hook := p.initHook
	p.mu.Unlock()

	if err != nil {
		return subprocess.InitializeResponse{}, err
	}
	if resp.ProtocolVersion == "" {
		resp = fakeInitializeResponse(req)
	}
	if hook != nil {
		hook()
	}
	return resp, nil
}

func (p *fakeProcess) Call(ctx context.Context, method string, params, result any) error {
	p.mu.Lock()
	callFn := p.callFn
	p.mu.Unlock()
	if callFn != nil {
		return callFn(ctx, method, params, result)
	}
	return nil
}

func (p *fakeProcess) Shutdown(ctx context.Context) error {
	p.mu.Lock()
	p.shutdownCnt++
	shutdownFn := p.shutdownFn
	p.mu.Unlock()

	if shutdownFn != nil {
		return shutdownFn(ctx)
	}
	p.close(nil)
	return nil
}

func (p *fakeProcess) Done() <-chan struct{} {
	return p.done
}

func (p *fakeProcess) Wait() error {
	<-p.done
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.waitErr
}

func (p *fakeProcess) HealthState() subprocess.HealthState {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.health
}

func (p *fakeProcess) PID() int {
	return p.pid
}

func (p *fakeProcess) initRequests() []subprocess.InitializeRequest {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]subprocess.InitializeRequest(nil), p.initReqs...)
}

func (p *fakeProcess) crash(err error) {
	p.close(err)
}

func (p *fakeProcess) close(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}
	p.closed = true
	p.waitErr = err
	close(p.done)
}

func fakeInitializeResponse(req subprocess.InitializeRequest) subprocess.InitializeResponse {
	implemented := append([]string{"health_check", "shutdown"}, req.Methods.ExtensionServices...)
	slices.Sort(implemented)
	return subprocess.InitializeResponse{
		ProtocolVersion: req.ProtocolVersion,
		ExtensionInfo: subprocess.InitializeExtensionInfo{
			Name:    req.Extension.Name,
			Version: req.Extension.Version,
		},
		AcceptedCapabilities: subprocess.AcceptedCapabilities{
			Provides: slices.Clone(req.Capabilities.Provides),
			Actions:  slices.Clone(req.Capabilities.GrantedActions),
			Security: slices.Clone(req.Capabilities.GrantedSecurity),
		},
		ImplementedMethods:  implemented,
		SupportedHookEvents: []string{string(hookspkg.HookTurnStart)},
		Supports: subprocess.InitializeSupports{
			HealthCheck: true,
		},
	}
}

type extensionHelperServer struct {
	scenario string
	marker   string

	mu         sync.Mutex
	writer     *bufio.Writer
	pendingReq string
	extension  string
}

func newExtensionHelperServer(scenario string, marker string) *extensionHelperServer {
	return &extensionHelperServer{
		scenario: strings.TrimSpace(scenario),
		marker:   marker,
		writer:   bufio.NewWriter(os.Stdout),
	}
}

func (h *extensionHelperServer) run() int {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 1024), 10<<20)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var envelope map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &envelope); err != nil {
			return 1
		}

		if _, ok := envelope["method"]; ok {
			var req helperRequest
			if err := json.Unmarshal([]byte(line), &req); err != nil {
				return 1
			}
			if err := h.handleRequest(req); err != nil {
				return 1
			}
			continue
		}

		var response helperResponse
		if err := json.Unmarshal([]byte(line), &response); err != nil {
			return 1
		}
		h.handleResponse(response)
	}

	if err := scanner.Err(); err != nil {
		return 1
	}
	return 0
}

func (h *extensionHelperServer) handleRequest(req helperRequest) error {
	switch req.Method {
	case "initialize":
		var params subprocess.InitializeRequest
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return err
		}
		response := fakeInitializeResponse(params)
		h.mu.Lock()
		h.extension = params.Extension.Name
		h.mu.Unlock()
		if err := h.sendResult(req.ID, response); err != nil {
			return err
		}
		if h.scenario == "record_initialize" || h.scenario == "auto_exit_record_initialize" {
			if err := h.recordInitialize(params, response); err != nil {
				return err
			}
		}

		switch h.scenario {
		case "host_call":
			h.mu.Lock()
			h.pendingReq = "host-1"
			h.mu.Unlock()
			go func() {
				time.Sleep(15 * time.Millisecond)
				if err := h.sendRequest("host-1", "sessions/list", map[string]string{"workspace": "ext"}); err != nil {
					os.Exit(1)
				}
			}()
			return nil
		case "auto_exit":
			if h.marker != "" {
				if err := appendMarkerLine(h.marker, fmt.Sprintf("%d", os.Getpid())); err != nil {
					return err
				}
			}
			go func() {
				time.Sleep(15 * time.Millisecond)
				os.Exit(1)
			}()
		case "auto_exit_record_initialize":
			go func() {
				time.Sleep(15 * time.Millisecond)
				os.Exit(1)
			}()
		}
		return nil
	case "health_check":
		return h.sendResult(req.ID, subprocess.HealthCheckResponse{Healthy: true})
	case "bridges/deliver":
		var params bridgepkg.DeliveryRequest
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return err
		}
		if h.scenario == "slow_record_deliveries" {
			time.Sleep(200 * time.Millisecond)
		}
		if err := h.recordDelivery(params); err != nil {
			return err
		}
		switch h.scenario {
		case "exit_once_record_deliveries":
			if markerLineCount(h.marker) == 1 {
				os.Exit(1)
			}
		case "transient_fail_once_record_deliveries":
			if markerLineCount(h.marker) == 1 {
				return h.sendError(req.ID, -32031, "Transient bridge delivery failure", map[string]string{
					"kind": "transient_delivery_failure",
				})
			}
		}

		ack := bridgepkg.DeliveryAck{
			DeliveryID: strings.TrimSpace(params.Event.DeliveryID),
			Seq:        params.Event.Seq,
		}
		switch h.scenario {
		case "delivery_ack_missing_seq":
			return h.sendResult(req.ID, map[string]any{"delivery_id": ack.DeliveryID})
		case "delivery_ack_null_seq":
			return h.sendResult(req.ID, map[string]any{"delivery_id": ack.DeliveryID, "seq": nil})
		case "delivery_ack_string_seq":
			return h.sendResult(req.ID, map[string]any{"delivery_id": ack.DeliveryID, "seq": "0"})
		case "delivery_ack_non_object":
			return h.sendResult(req.ID, []string{"not", "an", "ack"})
		}
		if ack.Seq > 0 {
			ack.RemoteMessageID = fmt.Sprintf("remote-%d", ack.Seq)
		}
		if ack.Seq > 1 {
			ack.ReplaceRemoteMessageID = fmt.Sprintf("remote-%d", ack.Seq-1)
		}
		return h.sendResult(req.ID, ack)
	case "bridges/targets/snapshot":
		var params bridgepkg.BridgeTargetSnapshotRequest
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return err
		}
		return h.sendResult(req.ID, bridgepkg.BridgeTargetSnapshotResponse{
			Targets: []bridgepkg.BridgeTargetSnapshot{
				{
					CanonicalRoute: "bridge://" + strings.TrimSpace(params.BridgeInstanceID) + "/general",
					DisplayName:    "general",
					TargetType:     bridgepkg.BridgeTargetTypeChannel,
					Qualifier:      "workspace",
					Capabilities:   []string{"send"},
				},
			},
		})
	case "models/list":
		var params extensioncontract.ModelSourceListParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return err
		}
		if h.scenario == "model_source_error" {
			return h.sendError(req.ID, -32020, "Model source unavailable", map[string]string{
				"error": "model source unavailable",
			})
		}
		sourceID, err := modelcatalog.SourceKindExtensionID(h.extensionName())
		if err != nil {
			return err
		}
		providerID := strings.TrimSpace(params.ProviderID)
		if providerID == "" {
			providerID = "codex"
		}
		row := extensioncontract.ModelSourceRow{
			SourceID:    sourceID,
			ProviderID:  providerID,
			ModelID:     "subprocess-model",
			DisplayName: "Subprocess Model",
			Priority:    modelcatalog.PriorityExtension,
		}
		if h.scenario == "model_source_malformed" {
			row.ModelID = ""
		}
		return h.sendResult(req.ID, extensioncontract.ModelSourceListResponse{
			Rows: []extensioncontract.ModelSourceRow{row},
		})
	case "provide_tools":
		return h.sendResult(req.ID, h.toolRuntimeDescriptors())
	case "tools/call":
		var params toolspkg.ExtensionToolCallRequest
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return err
		}
		if h.scenario == "tool_call_slow" {
			time.Sleep(200 * time.Millisecond)
		}
		if h.scenario == "tool_call_error" {
			return h.sendError(req.ID, -32010, "Tool execution failed", map[string]string{
				"error": "handler failed",
			})
		}
		return h.sendResult(req.ID, toolspkg.ExtensionToolCallResponse{
			Result: toolspkg.ToolResult{
				Content: []toolspkg.ToolContent{{
					Type: "text",
					Text: fmt.Sprintf("search:%s", strings.TrimSpace(string(params.Input))),
				}},
			},
		})
	case "shutdown":
		if h.scenario == "shutdown_hang" {
			select {}
		}
		if h.marker != "" {
			if err := os.WriteFile(h.marker, []byte("shutdown"), 0o600); err != nil {
				return err
			}
		}
		if err := h.sendResult(req.ID, subprocess.ShutdownResponse{Acknowledged: true}); err != nil {
			return err
		}
		return nil
	default:
		return h.sendResult(req.ID, map[string]any{})
	}
}

func (h *extensionHelperServer) toolRuntimeDescriptors() toolspkg.ExtensionProvideToolsResponse {
	id, err := toolspkg.CanonicalToolID("ext", h.extensionName(), "search")
	if err != nil {
		id = "ext__helper__search"
	}
	descriptor := toolspkg.ExtensionToolRuntimeDescriptor{
		ID:                id,
		Handler:           "search",
		InputSchemaDigest: "1dc63095e8672403bbe40fa26719d175e695c0167f6daad6b9655f6506491f01",
		ReadOnly:          true,
		Risk:              toolspkg.RiskRead,
	}
	switch h.scenario {
	case "tool_runtime_handler_mismatch":
		descriptor.Handler = "lookup"
	case "tool_runtime_schema_mismatch":
		descriptor.InputSchemaDigest = "bad-digest"
	case "tool_runtime_risk_mismatch":
		descriptor.ReadOnly = false
		descriptor.Risk = toolspkg.RiskMutating
	case "tool_runtime_missing_handler":
		descriptor.Handler = ""
	}
	return toolspkg.ExtensionProvideToolsResponse{
		Tools: []toolspkg.ExtensionToolRuntimeDescriptor{descriptor},
	}
}

func (h *extensionHelperServer) extensionName() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return strings.TrimSpace(h.extension)
}

func (h *extensionHelperServer) handleResponse(resp helperResponse) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.pendingReq == "" || fmt.Sprint(resp.ID) != h.pendingReq || h.marker == "" {
		return
	}
	if len(resp.Result) > 0 {
		if err := os.WriteFile(h.marker, resp.Result, 0o600); err != nil {
			h.pendingReq = ""
			return
		}
	}
	h.pendingReq = ""
}

func (h *extensionHelperServer) sendRequest(id string, method string, params any) error {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}
	return h.write(payload)
}

func (h *extensionHelperServer) sendResult(id any, result any) error {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	}
	return h.write(payload)
}

func (h *extensionHelperServer) sendError(id any, code int, message string, data any) error {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]any{
			"code":    code,
			"message": message,
			"data":    data,
		},
	}
	return h.write(payload)
}

func (h *extensionHelperServer) write(payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if _, err := h.writer.Write(append(data, '\n')); err != nil {
		return err
	}
	return h.writer.Flush()
}

func (h *extensionHelperServer) recordInitialize(
	request subprocess.InitializeRequest,
	response subprocess.InitializeResponse,
) error {
	if strings.TrimSpace(h.marker) == "" {
		return nil
	}

	payload, err := json.Marshal(struct {
		Request  subprocess.InitializeRequest  `json:"request"`
		Response subprocess.InitializeResponse `json:"response"`
	}{
		Request:  request,
		Response: response,
	})
	if err != nil {
		return err
	}
	return appendMarkerLine(h.marker, string(payload))
}

func (h *extensionHelperServer) recordDelivery(request bridgepkg.DeliveryRequest) error {
	if strings.TrimSpace(h.marker) == "" {
		return nil
	}

	payload, err := json.Marshal(managerDeliveryMarker{
		PID:     os.Getpid(),
		Request: request,
	})
	if err != nil {
		return err
	}
	return appendMarkerLine(h.marker, string(payload))
}

type helperRequest struct {
	ID     any             `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type helperResponse struct {
	ID     any             `json:"id"`
	Result json.RawMessage `json:"result"`
}

type managerDeliveryMarker struct {
	PID     int                       `json:"pid"`
	Request bridgepkg.DeliveryRequest `json:"request"`
}

type stubBridgeRuntimeResolver struct {
	runtimes map[string]*subprocess.InitializeBridgeRuntime
	err      error
}

func (r *stubBridgeRuntimeResolver) ResolveBridgeRuntime(
	_ context.Context,
	extensionName string,
) (*subprocess.InitializeBridgeRuntime, error) {
	if r == nil {
		return nil, nil
	}
	if r.err != nil {
		return nil, r.err
	}
	if r.runtimes == nil {
		return nil, nil
	}
	return subprocess.CloneInitializeBridgeRuntime(r.runtimes[strings.TrimSpace(extensionName)]), nil
}

func createManagerTestExtension(t *testing.T, manifestContent string, files map[string]string) managerFixture {
	t.Helper()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, manifestTOMLFileName), manifestContent)
	for relPath, content := range files {
		absPath := filepath.Join(dir, relPath)
		if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
			t.Fatalf("os.MkdirAll(%q) error = %v", filepath.Dir(absPath), err)
		}
		writeFile(t, absPath, content)
	}

	manifest, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest(%q) error = %v", dir, err)
	}
	checksum, err := ComputeDirectoryChecksum(dir)
	if err != nil {
		t.Fatalf("ComputeDirectoryChecksum(%q) error = %v", dir, err)
	}

	return managerFixture{
		dir:      dir,
		manifest: manifest,
		checksum: checksum,
	}
}

func installManagerFixture(
	t *testing.T,
	registry *Registry,
	fixture managerFixture,
	source ExtensionSource,
	enabled bool,
) {
	t.Helper()

	if err := registry.Install(fixture.manifest, fixture.dir, fixture.checksum, WithInstallSource(source)); err != nil {
		t.Fatalf("Install(%q) error = %v", fixture.manifest.Name, err)
	}
	if !enabled {
		if err := registry.Disable(fixture.manifest.Name); err != nil {
			t.Fatalf("Disable(%q) error = %v", fixture.manifest.Name, err)
		}
	}
}

func managerSkillFile(name string, description string) string {
	return fmt.Sprintf(
		`---
name: %s
description: %s
---
Use this skill carefully.
`,
		name,
		description,
	)
}

func managerAgentFile(name string) string {
	return fmt.Sprintf(
		`---
name: %s
provider: codex
---
Prompt body.
`,
		name,
	)
}

func managerTestManifest(name string, opts managerManifestOptions) string {
	minVersion := opts.minVersion
	if minVersion == "" {
		minVersion = "0.5.0"
	}
	command := opts.command
	if command == "" {
		command = "fake-extension"
	}
	capabilities := opts.capabilities
	if len(capabilities) == 0 {
		capabilities = []string{"memory.backend"}
	}
	actions := opts.actions
	if len(actions) == 0 {
		actions = []string{"sessions/list"}
	}
	security := opts.security
	if len(security) == 0 {
		security = []string{"session.read"}
	}
	bridgePlatform := opts.bridgePlatform
	bridgeDisplayName := opts.bridgeDisplayName
	if slices.Contains(capabilities, extensionprotocol.CapabilityProvideBridgeAdapter) {
		if bridgePlatform == "" {
			bridgePlatform = "telegram"
		}
		if bridgeDisplayName == "" {
			bridgeDisplayName = "Telegram"
		}
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, `[extension]
name = %q
version = "0.2.1"
description = "Extension manager test fixture"
min_agh_version = %q

[resources]
`, name, minVersion)
	if opts.withSkills {
		builder.WriteString(`skills = ["skills/"]
`)
	}
	if opts.withAgents {
		builder.WriteString(`agents = ["agents/"]
`)
	}
	if opts.withHooks {
		builder.WriteString(`
[[resources.hooks]]
name = "` + name + `-hook"
event = "turn.start"
mode = "sync"
executor.kind = "subprocess"
executor.command = "./bin/hook"
executor.args = ["--hook"]
`)
	}
	if opts.withMCP {
		builder.WriteString(`
[resources.mcp_servers]
[resources.mcp_servers.kubectl]
command = "mcp-kubectl"
args = ["--context", "prod"]
`)
	}
	if len(opts.resourceFamilies) > 0 || strings.TrimSpace(opts.resourceMaxScope) != "" {
		builder.WriteString(`
[resources.publish]
`)
		if len(opts.resourceFamilies) > 0 {
			builder.WriteString("families = " + tomlStringArray(opts.resourceFamilies) + "\n")
		}
		if scope := strings.TrimSpace(opts.resourceMaxScope); scope != "" {
			builder.WriteString(`max_scope = "` + scope + `"
`)
		}
	}

	builder.WriteString(`
[capabilities]
provides = ` + tomlStringArray(capabilities) + `

`)
	if bridgePlatform != "" || bridgeDisplayName != "" {
		fmt.Fprintf(&builder, `[bridge]
platform = %q
display_name = %q

`, bridgePlatform, bridgeDisplayName)
	}
	builder.WriteString(`[actions]
requires = ` + tomlStringArray(actions) + `

[subprocess]
command = ` + fmt.Sprintf("%q", command) + `
`)
	if len(opts.args) > 0 {
		builder.WriteString(`args = ` + tomlStringArray(opts.args) + `
`)
	}
	if opts.shutdown > 0 {
		builder.WriteString(`shutdown_timeout = "` + opts.shutdown.String() + `"
`)
	}
	if len(opts.withEnv) > 0 {
		builder.WriteString(`
[subprocess.env]
`)
		keys := make([]string, 0, len(opts.withEnv))
		for key := range opts.withEnv {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		for _, key := range keys {
			fmt.Fprintf(&builder, "%s = %q\n", key, opts.withEnv[key])
		}
	}
	builder.WriteString(`
[security]
capabilities = ` + tomlStringArray(security) + `
`)

	return builder.String()
}

func helperCommand(t *testing.T) string {
	t.Helper()
	command, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}
	return command
}

func helperArgs() []string {
	return []string{
		"-test.run=TestExtensionManagerHelperProcess",
	}
}

func helperEnv(scenario string, markerPath string) map[string]string {
	env := map[string]string{
		extensionHelperEnvKey:      "1",
		extensionHelperScenarioKey: scenario,
	}
	if strings.TrimSpace(markerPath) != "" {
		env[extensionHelperMarkerKey] = markerPath
	}
	return env
}

func appendMarkerLine(path string, line string) (err error) {
	target := strings.TrimSpace(path)
	if target == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(target, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()
	_, err = fmt.Fprintf(file, "%s\n", strings.TrimSpace(line))
	return err
}

func markerLineCount(path string) int {
	payload, err := os.ReadFile(strings.TrimSpace(path))
	if err != nil {
		return 0
	}
	count := 0
	for line := range strings.SplitSeq(string(payload), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		count++
	}
	return count
}

func envListToMap(t testing.TB, env []string) map[string]string {
	t.Helper()

	decoded := make(map[string]string, len(env))
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			t.Fatalf("env entry %q missing '=' separator", entry)
		}
		decoded[key] = value
	}
	return decoded
}

func waitForManagerCondition(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for manager condition")
}

func testBridgeRuntimeInstance(extensionName string, instanceID string) bridgepkg.BridgeInstance {
	return bridgepkg.BridgeInstance{
		ID:            instanceID,
		Scope:         bridgepkg.ScopeGlobal,
		Platform:      "telegram",
		ExtensionName: extensionName,
		DisplayName:   "Bridge Runtime",
		Enabled:       true,
		Status:        bridgepkg.BridgeStatusReady,
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	}
}

func testScopedBridgeRuntime(
	extensionName string,
	instanceID string,
	boundSecrets []subprocess.InitializeBridgeBoundSecret,
) *subprocess.InitializeBridgeRuntime {
	instance := testBridgeRuntimeInstance(extensionName, instanceID)
	return testScopedBridgeRuntimeForInstance(instance, boundSecrets)
}

func testScopedBridgeRuntimeForInstance(
	instance bridgepkg.BridgeInstance,
	boundSecrets []subprocess.InitializeBridgeBoundSecret,
) *subprocess.InitializeBridgeRuntime {
	return &subprocess.InitializeBridgeRuntime{
		RuntimeVersion: subprocess.InitializeBridgeRuntimeVersion2,
		Purpose:        subprocess.BridgeRuntimePurposeService,
		Provider:       instance.ExtensionName,
		Platform:       instance.Platform,
		ManagedInstances: []subprocess.InitializeBridgeManagedInstance{{
			Instance:     bridgepkg.BridgeInstanceToContract(instance),
			BoundSecrets: append([]subprocess.InitializeBridgeBoundSecret(nil), boundSecrets...),
		}},
	}
}

func mustSingleManagedBridge(
	t testing.TB,
	runtime *subprocess.InitializeBridgeRuntime,
) subprocess.InitializeBridgeManagedInstance {
	t.Helper()

	if runtime == nil {
		t.Fatal("bridge runtime = nil, want non-nil")
		return subprocess.InitializeBridgeManagedInstance{}
	}
	managed, err := runtime.SingleManagedInstance()
	if err != nil {
		t.Fatalf("runtime.SingleManagedInstance() error = %v", err)
	}
	return *managed
}
