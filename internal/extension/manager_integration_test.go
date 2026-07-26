//go:build integration

package extensionpkg

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	aghconfig "github.com/compozy/agh/internal/config"
	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/subprocess"
	"github.com/compozy/agh/internal/testutil"
)

type managerInitializeMarker struct {
	Request  subprocess.InitializeRequest  `json:"request"`
	Response subprocess.InitializeResponse `json:"response"`
}

func TestManagerIntegrationLifecycleAndHostAPICall(t *testing.T) {
	withDaemonVersion(t, "0.5.0")

	env := newRegistryTestEnv(t)
	markerPath := filepath.Join(t.TempDir(), "host-call.json")
	fixture := createManagerTestExtension(t, managerTestManifest("ext-host", managerManifestOptions{
		command:      helperCommand(t),
		args:         helperArgs(),
		withEnv:      helperEnv("host_call", markerPath),
		capabilities: []string{"memory.backend"},
		actions:      []string{"sessions/list"},
		security:     []string{"session.read"},
	}), nil)
	installManagerFixture(t, env.registry, fixture, SourceUser, true)

	manager := NewManager(
		env.registry,
		WithHostMethodHandler("sessions/list", func(_ context.Context, _ json.RawMessage) (any, error) {
			return []map[string]string{{"id": "sess-1"}}, nil
		}),
		WithHealthCheckTimeout(20*time.Millisecond),
		WithSubprocessSignalGrace(15*time.Millisecond),
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
		_, err := os.Stat(markerPath)
		return err == nil
	})

	payload, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", markerPath, err)
	}
	if !strings.Contains(string(payload), "sess-1") {
		t.Fatalf("host call payload = %s, want sess-1 response", string(payload))
	}
}

func TestManagerIntegrationRestartRecovery(t *testing.T) {
	withDaemonVersion(t, "0.5.0")

	env := newRegistryTestEnv(t)
	markerPath := filepath.Join(t.TempDir(), "starts.log")
	fixture := createManagerTestExtension(t, managerTestManifest("ext-recover", managerManifestOptions{
		command:      helperCommand(t),
		args:         helperArgs(),
		withEnv:      helperEnv("auto_exit", markerPath),
		capabilities: []string{"memory.backend"},
		actions:      []string{"sessions/list"},
		security:     []string{"session.read"},
	}), nil)
	installManagerFixture(t, env.registry, fixture, SourceUser, true)

	manager := NewManager(
		env.registry,
		WithHealthCheckTimeout(20*time.Millisecond),
		WithSubprocessSignalGrace(15*time.Millisecond),
		withRestartBackoffMax(10*time.Millisecond),
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

	waitForManagerCondition(t, 2*time.Second, func() bool {
		payload, err := os.ReadFile(markerPath)
		if err != nil {
			return false
		}
		return len(strings.Fields(string(payload))) >= 2
	})
}

func TestManagerIntegrationResourceRegistration(t *testing.T) {
	withDaemonVersion(t, "0.5.0")

	env := newRegistryTestEnv(t)
	fixture := createManagerTestExtension(t, managerTestManifest("ext-resources", managerManifestOptions{
		command:      helperCommand(t),
		args:         helperArgs(),
		withEnv:      helperEnv("default", ""),
		withSkills:   true,
		withAgents:   true,
		withHooks:    true,
		withMCP:      true,
		capabilities: []string{"memory.backend"},
		actions:      []string{"sessions/list"},
		security:     []string{"session.read"},
	}), map[string]string{
		"skills/review.md": managerSkillFile("resource-skill", "Loaded from extension"),
		"agents/agent.md":  managerAgentFile("resource-agent"),
	})
	installManagerFixture(t, env.registry, fixture, SourceUser, true)

	manager := NewManager(
		env.registry,
		WithHealthCheckTimeout(20*time.Millisecond),
		WithSubprocessSignalGrace(15*time.Millisecond),
	)

	if err := manager.Start(testutil.Context(t)); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Stop(testutil.Context(t)); err != nil {
			t.Fatalf("Stop() cleanup error = %v", err)
		}
	})

	if agents := manager.AgentDefinitions(); len(agents) != 1 || agents[0].Name != "resource-agent" {
		t.Fatalf("AgentDefinitions() = %#v, want resource-agent", agents)
	}
	loaded, err := manager.Get("ext-resources")
	if err != nil {
		t.Fatalf("Get(ext-resources) error = %v", err)
	}
	if len(loaded.Skills) != 1 || loaded.Skills[0].Meta.Name != "resource-skill" {
		t.Fatalf("Get(ext-resources).Skills = %#v, want resource-skill extension snapshot", loaded.Skills)
	}
	if decls, err := manager.HookDeclarations(testutil.Context(t)); err != nil {
		t.Fatalf("HookDeclarations() error = %v", err)
	} else if len(decls) != 1 || decls[0].Name != "ext-resources-hook" {
		t.Fatalf("HookDeclarations() = %#v, want ext-resources-hook", decls)
	}
}

func TestManagerIntegrationBridgeAdapterNegotiatesDeliveryRuntime(t *testing.T) {
	withDaemonVersion(t, "0.5.0")

	env := newRegistryTestEnv(t)
	markerPath := filepath.Join(t.TempDir(), "bridge-init.jsonl")
	fixture := createManagerTestExtension(t, managerTestManifest("ext-bridge-live", managerManifestOptions{
		command:      helperCommand(t),
		args:         helperArgs(),
		withEnv:      helperEnv("record_initialize", markerPath),
		capabilities: []string{extensionprotocol.CapabilityProvideBridgeAdapter},
		actions: []string{
			string(extensionprotocol.HostAPIMethodBridgesMessagesIngest),
			string(extensionprotocol.HostAPIMethodBridgesInstancesGet),
			string(extensionprotocol.HostAPIMethodBridgesInstancesReportState),
		},
		security: []string{"bridge.read", "bridge.write"},
	}), nil)
	installManagerFixture(t, env.registry, fixture, SourceUser, true)

	manager := NewManager(
		env.registry,
		WithBridgeRuntimeResolver(&stubBridgeRuntimeResolver{
			runtimes: map[string]*subprocess.InitializeBridgeRuntime{
				"ext-bridge-live": testScopedBridgeRuntime(
					"ext-bridge-live",
					"brg-live",
					[]subprocess.InitializeBridgeBoundSecret{
						{BindingName: "bot_token", Kind: "bot_token", Value: "token-live"},
					},
				),
			},
		}),
		WithHealthCheckTimeout(20*time.Millisecond),
		WithSubprocessSignalGrace(15*time.Millisecond),
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
		lines, err := readFileLines(markerPath)
		return err == nil && len(lines) >= 1
	})

	markers := readInitializeMarkers(t, markerPath)
	if len(markers) == 0 {
		t.Fatal("initialize markers = empty, want negotiated bridge handshake")
	}
	request := markers[0].Request
	if !slicesEqualStrings(request.Methods.ExtensionServices, []string{"bridges/deliver", "bridges/targets/snapshot"}) {
		t.Fatalf(
			"initialize extension services = %#v, want [bridges/deliver bridges/targets/snapshot]",
			request.Methods.ExtensionServices,
		)
	}
	if request.Runtime.Bridge == nil {
		t.Fatal("initialize runtime bridge = nil, want bound bridge launch payload")
	}
	managed := mustSingleManagedBridge(t, request.Runtime.Bridge)
	if got, want := managed.Instance.ID, "brg-live"; got != want {
		t.Fatalf("initialize runtime bridge instance id = %q, want %q", got, want)
	}
	if got := managed.BoundSecrets; len(got) != 1 || got[0].BindingName != "bot_token" || got[0].Value != "token-live" {
		t.Fatalf("initialize runtime bridge bound secrets = %#v, want one bound secret", got)
	}
}

func TestManagerIntegrationInvalidDeliveryResultsAreIndeterminate(t *testing.T) {
	withDaemonVersion(t, "0.5.0")

	scenarios := []string{
		"delivery_ack_missing_seq",
		"delivery_ack_null_seq",
		"delivery_ack_string_seq",
		"delivery_ack_non_object",
	}
	for _, scenario := range scenarios {
		t.Run("Should terminalize "+scenario, func(t *testing.T) {
			extensionName := "ext-" + strings.ReplaceAll(scenario, "_", "-")
			markerPath := filepath.Join(t.TempDir(), "deliveries.jsonl")
			env := newRegistryTestEnv(t)
			fixture := createManagerTestExtension(t, managerTestManifest(extensionName, managerManifestOptions{
				command:      helperCommand(t),
				args:         helperArgs(),
				withEnv:      helperEnv(scenario, markerPath),
				capabilities: []string{extensionprotocol.CapabilityProvideBridgeAdapter},
				actions: []string{
					string(extensionprotocol.HostAPIMethodBridgesMessagesIngest),
					string(extensionprotocol.HostAPIMethodBridgesInstancesGet),
					string(extensionprotocol.HostAPIMethodBridgesInstancesReportState),
				},
				security: []string{"bridge.read", "bridge.write"},
			}), nil)
			installManagerFixture(t, env.registry, fixture, SourceUser, true)

			manager := NewManager(
				env.registry,
				WithBridgeRuntimeResolver(&stubBridgeRuntimeResolver{
					runtimes: map[string]*subprocess.InitializeBridgeRuntime{
						extensionName: testScopedBridgeRuntime(extensionName, "brg-ack", nil),
					},
				}),
				WithHealthCheckTimeout(20*time.Millisecond),
				WithSubprocessSignalGrace(15*time.Millisecond),
			)
			if err := manager.Start(testutil.Context(t)); err != nil {
				t.Fatalf("Start() error = %v", err)
			}
			t.Cleanup(func() {
				if err := manager.Stop(testutil.Context(t)); err != nil {
					t.Fatalf("Stop() cleanup error = %v", err)
				}
			})

			req := bridgepkg.DeliveryRequest{Event: bridgepkg.DeliveryEvent{
				DeliveryID:       "del-zero",
				BridgeInstanceID: "brg-ack",
				RoutingKey: bridgepkg.RoutingKey{
					Scope:            bridgepkg.ScopeWorkspace,
					WorkspaceID:      "ws-ack",
					BridgeInstanceID: "brg-ack",
					PeerID:           "peer-ack",
				},
				DeliveryTarget: bridgepkg.DeliveryTarget{
					BridgeInstanceID: "brg-ack",
					PeerID:           "peer-ack",
					Mode:             bridgepkg.DeliveryModeReply,
				},
				Seq:       0,
				EventType: bridgepkg.DeliveryEventTypeStart,
				Content:   bridgepkg.MessageContent{Text: "hello"},
			}}
			ack, err := manager.DeliverBridge(testutil.Context(t), extensionName, req)
			if err != nil {
				t.Fatalf("DeliverBridge() error = %v, want semantic acknowledgement", err)
			}
			if err := ack.ValidateFor(req.Event); err != nil {
				t.Fatalf("semantic acknowledgement validation error = %v", err)
			}
			if ack.Outcome != bridgepkg.DeliveryAckOutcomeCommittedResultUnavailable ||
				ack.Error == nil || ack.Error.Message != bridgepkg.DeliveryResultUnavailableMessage {
				t.Fatalf("DeliverBridge() ack = %#v, want fixed indeterminate outcome", ack)
			}
			if markers := readDeliveryMarkers(t, markerPath); len(markers) != 1 {
				t.Fatalf("delivery calls = %d, want exactly one", len(markers))
			}
		})
	}
}

func TestManagerIntegrationWorkspaceExtensionCannotReceiveGlobalResourceScope(t *testing.T) {
	withDaemonVersion(t, "0.5.0")

	env := newRegistryTestEnv(t)
	fixture := createManagerTestExtension(t, managerTestManifest("ext-workspace-grants", managerManifestOptions{
		command:          helperCommand(t),
		args:             helperArgs(),
		withEnv:          helperEnv("default", ""),
		resourceFamilies: []string{"tools"},
		resourceMaxScope: "global",
	}), nil)
	installManagerFixture(t, env.registry, fixture, SourceWorkspace, true)

	manager := NewManager(
		env.registry,
		WithHealthCheckTimeout(20*time.Millisecond),
		WithSubprocessSignalGrace(15*time.Millisecond),
	)

	if err := manager.Start(testutil.Context(t)); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Stop(testutil.Context(t)); err != nil {
			t.Fatalf("Stop() cleanup error = %v", err)
		}
	})

	ext, err := manager.Get("ext-workspace-grants")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !slicesEqualResourceKinds(ext.GrantedResourceKinds, []resources.ResourceKind{resources.ResourceKind("tool")}) {
		t.Fatalf("GrantedResourceKinds = %#v, want [tool]", ext.GrantedResourceKinds)
	}
	if !slicesEqualResourceScopes(
		ext.GrantedResourceScopes,
		[]resources.ResourceScopeKind{resources.ResourceScopeKindWorkspace},
	) {
		t.Fatalf("GrantedResourceScopes = %#v, want [workspace]", ext.GrantedResourceScopes)
	}
}

func TestManagerIntegrationResourceGrantsComeFromDaemonPolicy(t *testing.T) {
	withDaemonVersion(t, "0.5.0")

	env := newRegistryTestEnv(t)
	checker := &CapabilityChecker{}
	checker.SetResourcePolicy(aghconfig.ExtensionsResourcesConfig{
		AllowedKinds: []resources.ResourceKind{resources.ResourceKind("tool")},
		MaxScope:     resources.ResourceScopeKindWorkspace,
	})
	fixture := createManagerTestExtension(t, managerTestManifest("ext-daemon-policy", managerManifestOptions{
		command:          helperCommand(t),
		args:             helperArgs(),
		withEnv:          helperEnv("default", ""),
		resourceFamilies: []string{"tools", "mcp_servers"},
		resourceMaxScope: "global",
	}), nil)
	installManagerFixture(t, env.registry, fixture, SourceUser, true)

	manager := NewManager(
		env.registry,
		WithCapabilityChecker(checker),
		WithHealthCheckTimeout(20*time.Millisecond),
		WithSubprocessSignalGrace(15*time.Millisecond),
	)

	if err := manager.Start(testutil.Context(t)); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Stop(testutil.Context(t)); err != nil {
			t.Fatalf("Stop() cleanup error = %v", err)
		}
	})

	ext, err := manager.Get("ext-daemon-policy")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !slicesEqualResourceKinds(ext.GrantedResourceKinds, []resources.ResourceKind{resources.ResourceKind("tool")}) {
		t.Fatalf("GrantedResourceKinds = %#v, want [tool]", ext.GrantedResourceKinds)
	}
	if !slicesEqualResourceScopes(
		ext.GrantedResourceScopes,
		[]resources.ResourceScopeKind{resources.ResourceScopeKindWorkspace},
	) {
		t.Fatalf("GrantedResourceScopes = %#v, want [workspace]", ext.GrantedResourceScopes)
	}
}

func TestManagerIntegrationInitializeIncludesSessionNonceAndResourceGrants(t *testing.T) {
	withDaemonVersion(t, "0.5.0")

	env := newRegistryTestEnv(t)
	markerPath := filepath.Join(t.TempDir(), "resource-init.jsonl")
	checker := &CapabilityChecker{}
	checker.SetResourcePolicy(aghconfig.ExtensionsResourcesConfig{
		AllowedKinds: []resources.ResourceKind{resources.ResourceKind("tool")},
		MaxScope:     resources.ResourceScopeKindWorkspace,
	})
	fixture := createManagerTestExtension(t, managerTestManifest("ext-resource-init", managerManifestOptions{
		command:          helperCommand(t),
		args:             helperArgs(),
		withEnv:          helperEnv("record_initialize", markerPath),
		resourceFamilies: []string{"tools", "mcp_servers"},
		resourceMaxScope: "global",
	}), nil)
	installManagerFixture(t, env.registry, fixture, SourceUser, true)

	resourceKernel, err := resources.NewKernel(env.registry.DB())
	if err != nil {
		t.Fatalf("resources.NewKernel() error = %v", err)
	}

	manager := NewManager(
		env.registry,
		WithCapabilityChecker(checker),
		WithSourceSessionManager(resourceKernel),
		WithHealthCheckTimeout(20*time.Millisecond),
		WithSubprocessSignalGrace(15*time.Millisecond),
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
		lines, err := readFileLines(markerPath)
		return err == nil && len(lines) >= 1
	})

	markers := readInitializeMarkers(t, markerPath)
	if len(markers) == 0 {
		t.Fatal("initialize markers = empty, want resource initialize handshake")
	}
	request := markers[0].Request
	if strings.TrimSpace(request.SessionNonce) == "" {
		t.Fatal("initialize session_nonce = empty, want daemon-issued nonce")
	}
	if !slicesEqualStrings(
		request.Capabilities.GrantedResourceKinds,
		[]string{"tool"},
	) {
		t.Fatalf(
			"initialize granted_resource_kinds = %#v, want [tool]",
			request.Capabilities.GrantedResourceKinds,
		)
	}
	if !slicesEqualStrings(
		request.Capabilities.GrantedResourceScopes,
		[]string{string(resources.ResourceScopeKindWorkspace)},
	) {
		t.Fatalf(
			"initialize granted_resource_scopes = %#v, want [workspace]",
			request.Capabilities.GrantedResourceScopes,
		)
	}
}

func TestManagerIntegrationNonBridgeExtensionStartsWithoutBridgeNegotiation(t *testing.T) {
	withDaemonVersion(t, "0.5.0")

	env := newRegistryTestEnv(t)
	markerPath := filepath.Join(t.TempDir(), "plain-init.jsonl")
	fixture := createManagerTestExtension(t, managerTestManifest("ext-plain-live", managerManifestOptions{
		command:      helperCommand(t),
		args:         helperArgs(),
		withEnv:      helperEnv("record_initialize", markerPath),
		capabilities: []string{"memory.backend"},
		actions:      []string{"sessions/list"},
		security:     []string{"session.read"},
	}), nil)
	installManagerFixture(t, env.registry, fixture, SourceUser, true)

	manager := NewManager(
		env.registry,
		WithHealthCheckTimeout(20*time.Millisecond),
		WithSubprocessSignalGrace(15*time.Millisecond),
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
		lines, err := readFileLines(markerPath)
		return err == nil && len(lines) >= 1
	})

	markers := readInitializeMarkers(t, markerPath)
	if len(markers) == 0 {
		t.Fatal("initialize markers = empty, want generic extension handshake")
	}
	request := markers[0].Request
	if slicesContainsString(request.Methods.ExtensionServices, "bridges/deliver") {
		t.Fatalf(
			"initialize extension services = %#v, want no bridges/deliver negotiation",
			request.Methods.ExtensionServices,
		)
	}
	if request.Runtime.Bridge != nil {
		t.Fatalf("initialize runtime bridge = %#v, want nil for non-bridge extension", request.Runtime.Bridge)
	}
}

func TestManagerIntegrationBridgeAdapterRestartPreservesNegotiatedSurface(t *testing.T) {
	withDaemonVersion(t, "0.5.0")

	env := newRegistryTestEnv(t)
	markerPath := filepath.Join(t.TempDir(), "bridge-restart.jsonl")
	fixture := createManagerTestExtension(t, managerTestManifest("ext-bridge-restart", managerManifestOptions{
		command:      helperCommand(t),
		args:         helperArgs(),
		withEnv:      helperEnv("auto_exit_record_initialize", markerPath),
		capabilities: []string{extensionprotocol.CapabilityProvideBridgeAdapter},
		actions: []string{
			string(extensionprotocol.HostAPIMethodBridgesMessagesIngest),
			string(extensionprotocol.HostAPIMethodBridgesInstancesGet),
			string(extensionprotocol.HostAPIMethodBridgesInstancesReportState),
		},
		security: []string{"bridge.read", "bridge.write"},
	}), nil)
	installManagerFixture(t, env.registry, fixture, SourceUser, true)

	manager := NewManager(
		env.registry,
		WithBridgeRuntimeResolver(&stubBridgeRuntimeResolver{
			runtimes: map[string]*subprocess.InitializeBridgeRuntime{
				"ext-bridge-restart": {
					RuntimeVersion: subprocess.InitializeBridgeRuntimeVersion2,
					Purpose:        subprocess.BridgeRuntimePurposeService,
					Provider:       "ext-bridge-restart",
					Platform:       "telegram",
					ManagedInstances: []subprocess.InitializeBridgeManagedInstance{
						{
							Instance: bridgepkg.BridgeInstanceToContract(
								testBridgeRuntimeInstance("ext-bridge-restart", "brg-restart-a"),
							),
							BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
								{BindingName: "bot_token", Kind: "bot_token", Value: "token-restart"},
							},
						},
						{
							Instance: bridgepkg.BridgeInstanceToContract(
								testBridgeRuntimeInstance("ext-bridge-restart", "brg-restart-b"),
							),
						},
					},
				},
			},
		}),
		WithHealthCheckTimeout(20*time.Millisecond),
		WithSubprocessSignalGrace(15*time.Millisecond),
		withRestartBackoffMax(10*time.Millisecond),
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

	waitForManagerCondition(t, 2*time.Second, func() bool {
		lines, err := readFileLines(markerPath)
		return err == nil && len(lines) >= 2
	})

	markers := readInitializeMarkers(t, markerPath)
	if len(markers) < 2 {
		t.Fatalf("initialize markers = %d, want at least 2 launches", len(markers))
	}
	for index, marker := range markers[:2] {
		if !slicesEqualStrings(
			marker.Request.Methods.ExtensionServices,
			[]string{"bridges/deliver", "bridges/targets/snapshot"},
		) {
			t.Fatalf(
				"marker %d extension services = %#v, want [bridges/deliver bridges/targets/snapshot]",
				index,
				marker.Request.Methods.ExtensionServices,
			)
		}
		if marker.Request.Runtime.Bridge == nil {
			t.Fatalf("marker %d runtime bridge = nil, want bound bridge launch payload", index)
		}
		if got, want := marker.Request.Runtime.Bridge.ManagedBridgeInstanceIDs(), []string{
			"brg-restart-a",
			"brg-restart-b",
		}; !slicesEqualStrings(
			got,
			want,
		) {
			t.Fatalf("marker %d runtime bridge managed ids = %#v, want %#v", index, got, want)
		}
	}
}

func TestManagerIntegrationBridgeAdapterDefersUntilRuntimeExists(t *testing.T) {
	withDaemonVersion(t, "0.5.0")

	env := newRegistryTestEnv(t)
	markerPath := filepath.Join(t.TempDir(), "bridge-deferred.jsonl")
	fixture := createManagerTestExtension(t, managerTestManifest("ext-bridge-deferred-live", managerManifestOptions{
		command:      helperCommand(t),
		args:         helperArgs(),
		withEnv:      helperEnv("record_initialize", markerPath),
		capabilities: []string{extensionprotocol.CapabilityProvideBridgeAdapter},
		actions: []string{
			string(extensionprotocol.HostAPIMethodBridgesMessagesIngest),
			string(extensionprotocol.HostAPIMethodBridgesInstancesGet),
			string(extensionprotocol.HostAPIMethodBridgesInstancesReportState),
		},
		security: []string{"bridge.read", "bridge.write"},
	}), nil)
	installManagerFixture(t, env.registry, fixture, SourceUser, true)

	manager := NewManager(
		env.registry,
		WithBridgeRuntimeResolver(&stubBridgeRuntimeResolver{err: ErrBridgeRuntimeDeferred}),
		WithHealthCheckTimeout(20*time.Millisecond),
		WithSubprocessSignalGrace(15*time.Millisecond),
	)

	if err := manager.Start(testutil.Context(t)); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Stop(testutil.Context(t)); err != nil {
			t.Fatalf("Stop() cleanup error = %v", err)
		}
	})

	if _, err := os.Stat(markerPath); !os.IsNotExist(err) {
		t.Fatalf("initialize marker stat error = %v, want os.ErrNotExist", err)
	}

	loaded, err := manager.Get("ext-bridge-deferred-live")
	if err != nil {
		t.Fatalf("Get(ext-bridge-deferred-live) error = %v", err)
	}
	if loaded.Status.Active {
		t.Fatal("Get(ext-bridge-deferred-live).Status.Active = true, want false")
	}
	if !loaded.Status.Registered {
		t.Fatal("Get(ext-bridge-deferred-live).Status.Registered = false, want true")
	}
	if loaded.Status.LastError != "" {
		t.Fatalf("Get(ext-bridge-deferred-live).Status.LastError = %q, want empty", loaded.Status.LastError)
	}
}

func readInitializeMarkers(t *testing.T, path string) []managerInitializeMarker {
	t.Helper()

	lines, err := readFileLines(path)
	if err != nil {
		t.Fatalf("readFileLines(%q) error = %v", path, err)
	}

	markers := make([]managerInitializeMarker, 0, len(lines))
	for _, line := range lines {
		var marker managerInitializeMarker
		if err := json.Unmarshal([]byte(line), &marker); err != nil {
			t.Fatalf("json.Unmarshal(initialize marker) error = %v; line=%q", err, line)
		}
		markers = append(markers, marker)
	}
	return markers
}

func slicesEqualResourceKinds(left []resources.ResourceKind, right []resources.ResourceKind) bool {
	return slicesEqualStrings(resourceKindsToStrings(left), resourceKindsToStrings(right))
}

func slicesEqualResourceScopes(left []resources.ResourceScopeKind, right []resources.ResourceScopeKind) bool {
	return slicesEqualStrings(resourceScopesToStrings(left), resourceScopesToStrings(right))
}

func resourceKindsToStrings(values []resources.ResourceKind) []string {
	if len(values) == 0 {
		return nil
	}
	dst := make([]string, 0, len(values))
	for _, value := range values {
		dst = append(dst, string(value))
	}
	return dst
}

func resourceScopesToStrings(values []resources.ResourceScopeKind) []string {
	if len(values) == 0 {
		return nil
	}
	dst := make([]string, 0, len(values))
	for _, value := range values {
		dst = append(dst, string(value))
	}
	return dst
}

func readFileLines(path string) ([]string, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	trimmed := strings.TrimSpace(string(payload))
	if trimmed == "" {
		return nil, nil
	}

	lines := strings.Split(trimmed, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		if candidate := strings.TrimSpace(line); candidate != "" {
			filtered = append(filtered, candidate)
		}
	}
	return filtered, nil
}

func slicesEqualStrings(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func slicesContainsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
