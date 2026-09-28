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

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/subprocess"
	"github.com/compozy/compozy/internal/testutil"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
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
		permissions:  []string{"sessions/list"},
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
		permissions:  []string{"sessions/list"},
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
		permissions:  []string{"sessions/list"},
	}), map[string]string{
		"skills/review/SKILL.md": managerSkillFile("resource-skill", "Loaded from extension"),
		"agents/agent/AGENT.md":  managerAgentFile("resource-agent"),
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
	if decls, err := manager.HookDeclarationsForProfiles(testutil.Context(t), []ProfileLens{{
		ID: store.DefaultProfileID, Name: "default",
	}}); err != nil {
		t.Fatalf("HookDeclarationsForProfiles() error = %v", err)
	} else if len(decls) != 1 || decls[0].Name != "ext-resources-hook" {
		t.Fatalf("HookDeclarationsForProfiles() = %#v, want ext-resources-hook", decls)
	}
}

func TestManagerIntegrationWorkspaceExtensionCannotReceiveUserResourceScope(t *testing.T) {
	withDaemonVersion(t, "0.5.0")

	env := newRegistryTestEnv(t)
	fixture := createManagerTestExtension(t, managerTestManifest("ext-workspace-grants", managerManifestOptions{
		command:          helperCommand(t),
		args:             helperArgs(),
		withEnv:          helperEnv("default", ""),
		resourceFamilies: []string{"tools"},
		resourceMaxScope: "user",
	}), nil)
	installManagerFixture(t, env.registry, fixture, SourceWorkspace, true)

	manager := NewManager(
		env.registry,
		WithWorkspaceResolver(newHostAPIFakeWorkspaceResolver(&workspacepkg.ResolvedWorkspace{
			Workspace: workspacepkg.Workspace{
				ID:      "ws-extension-grants",
				RootDir: fixture.dir,
				Name:    "extension-grants",
			},
			WorkspaceID: "ws-extension-grants",
		})),
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
		[]resources.ResourceScopeKind{
			resources.ResourceScopeKindWorkspace,
			resources.ResourceScopeKindWorkspaceProfile,
		},
	) {
		t.Fatalf("GrantedResourceScopes = %#v, want [workspace workspace_profile]", ext.GrantedResourceScopes)
	}
}

func TestManagerIntegrationResourceGrantsComeFromDaemonPolicy(t *testing.T) {
	withDaemonVersion(t, "0.5.0")

	env := newRegistryTestEnv(t)
	checker := &CapabilityChecker{}
	checker.SetResourcePolicy(compozyconfig.ExtensionsResourcesConfig{
		AllowedKinds: []resources.ResourceKind{resources.ResourceKind("tool")},
		MaxScope:     resources.ResourceScopeKindWorkspace,
	})
	fixture := createManagerTestExtension(t, managerTestManifest("ext-daemon-policy", managerManifestOptions{
		command:          helperCommand(t),
		args:             helperArgs(),
		withEnv:          helperEnv("default", ""),
		resourceFamilies: []string{"tools", "mcp_servers"},
		resourceMaxScope: "user",
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
		[]resources.ResourceScopeKind{
			resources.ResourceScopeKindWorkspace,
			resources.ResourceScopeKindWorkspaceProfile,
		},
	) {
		t.Fatalf("GrantedResourceScopes = %#v, want [workspace workspace_profile]", ext.GrantedResourceScopes)
	}
}

func TestManagerIntegrationInitializeIncludesSessionNonceAndResourceGrants(t *testing.T) {
	withDaemonVersion(t, "0.5.0")

	env := newRegistryTestEnv(t)
	markerPath := filepath.Join(t.TempDir(), "resource-init.jsonl")
	checker := &CapabilityChecker{}
	checker.SetResourcePolicy(compozyconfig.ExtensionsResourcesConfig{
		AllowedKinds: []resources.ResourceKind{resources.ResourceKind("tool")},
		MaxScope:     resources.ResourceScopeKindWorkspace,
	})
	fixture := createManagerTestExtension(t, managerTestManifest("ext-resource-init", managerManifestOptions{
		command:          helperCommand(t),
		args:             helperArgs(),
		withEnv:          helperEnv("record_initialize", markerPath),
		resourceFamilies: []string{"tools", "mcp_servers"},
		resourceMaxScope: "user",
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
		[]string{string(resources.ResourceScopeKindWorkspace), string(resources.ResourceScopeKindWorkspaceProfile)},
	) {
		t.Fatalf(
			"initialize granted_resource_scopes = %#v, want [workspace workspace_profile]",
			request.Capabilities.GrantedResourceScopes,
		)
	}
}

func TestManagerIntegrationExtensionStartsWithNegotiatedServices(t *testing.T) {
	t.Run("Should start a subprocess with negotiated service methods", func(t *testing.T) {
		withDaemonVersion(t, "0.5.0")

		env := newRegistryTestEnv(t)
		markerPath := filepath.Join(t.TempDir(), "plain-init.jsonl")
		fixture := createManagerTestExtension(t, managerTestManifest("ext-plain-live", managerManifestOptions{
			command:      helperCommand(t),
			args:         helperArgs(),
			withEnv:      helperEnv("record_initialize", markerPath),
			capabilities: []string{"memory.backend"},
			permissions:  []string{"sessions/list"},
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
		if !slicesEqualStrings(
			request.Methods.ExtensionServices,
			[]string{"memory/forget", "memory/recall", "memory/store"},
		) {
			t.Fatalf("initialize services = %#v, want memory service methods", request.Methods.ExtensionServices)
		}
	})
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
