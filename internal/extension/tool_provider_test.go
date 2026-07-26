package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
	"github.com/compozy/agh/internal/subprocess"
	"github.com/compozy/agh/internal/testutil"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func TestExtensionToolProviderAvailability(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		mutate func(*toolspkg.ExtensionToolRuntimeDescriptor)
		want   toolspkg.ReasonCode
	}{
		{
			name: "Should report handler mismatch from runtime descriptors",
			mutate: func(descriptor *toolspkg.ExtensionToolRuntimeDescriptor) {
				descriptor.Handler = "lookup"
			},
			want: toolspkg.ReasonExtensionRuntimeMismatch,
		},
		{
			name: "Should report input schema digest mismatch from runtime descriptors",
			mutate: func(descriptor *toolspkg.ExtensionToolRuntimeDescriptor) {
				descriptor.InputSchemaDigest = "bad-digest"
			},
			want: toolspkg.ReasonRuntimeDescriptorMismatch,
		},
		{
			name: "Should report risk mismatch from runtime descriptors",
			mutate: func(descriptor *toolspkg.ExtensionToolRuntimeDescriptor) {
				descriptor.ReadOnly = false
				descriptor.Risk = toolspkg.RiskMutating
			},
			want: toolspkg.ReasonExtensionRuntimeMismatch,
		},
		{
			name: "Should report missing runtime handler",
			mutate: func(descriptor *toolspkg.ExtensionToolRuntimeDescriptor) {
				descriptor.Handler = ""
			},
			want: toolspkg.ReasonHandlerMissing,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			env, fixture, descriptor := createExtensionToolProviderFixture(t, "ext-tool", true)
			runtimeDescriptor := descriptor.RuntimeDescriptor
			tc.mutate(&runtimeDescriptor)
			runtime := newFakeExtensionToolRuntime(
				t,
				env.registry,
				fixture.manifest.Name,
				[]toolspkg.ExtensionToolRuntimeDescriptor{
					runtimeDescriptor,
				},
			)
			registry := newExtensionToolRegistry(t, env.registry, runtime, extensionToolPolicyAllowAll())

			view, err := registry.Get(testutil.Context(t), toolspkg.Scope{Operator: true}, descriptor.Tool.ID)
			if err != nil {
				t.Fatalf("Registry.Get() error = %v", err)
			}
			if view.Availability.Executable {
				t.Fatalf("Availability.Executable = true, want false with reasons %#v", view.Availability.ReasonCodes)
			}
			if !slices.Contains(view.Availability.ReasonCodes, tc.want) {
				t.Fatalf("Availability reasons = %#v, want %q", view.Availability.ReasonCodes, tc.want)
			}
		})
	}
}

func TestExtensionToolProviderCatalog(t *testing.T) {
	t.Parallel()

	t.Run("Should remove disabled extension tools from registry projections", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		env, fixture, descriptor := createExtensionToolProviderFixture(t, "ext-tool", true)
		runtime := newFakeExtensionToolRuntime(
			t,
			env.registry,
			fixture.manifest.Name,
			[]toolspkg.ExtensionToolRuntimeDescriptor{descriptor.RuntimeDescriptor},
		)
		registry := newExtensionToolRegistry(t, env.registry, runtime, extensionToolPolicyAllowAll())

		views, err := registry.List(ctx, toolspkg.Scope{Operator: true})
		if err != nil {
			t.Fatalf("Registry.List(enabled) error = %v", err)
		}
		if !extensionToolViewsContain(views, descriptor.Tool.ID) {
			t.Fatalf("Registry.List(enabled) omitted %q", descriptor.Tool.ID)
		}

		if err := env.registry.Disable(fixture.manifest.Name); err != nil {
			t.Fatalf("Disable(%q) error = %v", fixture.manifest.Name, err)
		}
		views, err = registry.List(ctx, toolspkg.Scope{Operator: true})
		if err != nil {
			t.Fatalf("Registry.List(disabled) error = %v", err)
		}
		if extensionToolViewsContain(views, descriptor.Tool.ID) {
			t.Fatalf("Registry.List(disabled) contains %q, want removed from catalog", descriptor.Tool.ID)
		}
		searchResults, err := registry.Search(ctx, toolspkg.Scope{Operator: true}, toolspkg.SearchQuery{
			Query: descriptor.Tool.ID.String(),
		})
		if err != nil {
			t.Fatalf("Registry.Search(disabled) error = %v", err)
		}
		if len(searchResults) != 0 {
			t.Fatalf("Registry.Search(disabled) returned %#v, want no disabled extension tools", searchResults)
		}
		_, err = registry.Get(ctx, toolspkg.Scope{Operator: true}, descriptor.Tool.ID)
		if !errors.Is(err, toolspkg.ErrToolNotFound) {
			t.Fatalf("Registry.Get(disabled) error = %v, want ErrToolNotFound", err)
		}

		if err := env.registry.Enable(fixture.manifest.Name); err != nil {
			t.Fatalf("Enable(%q) error = %v", fixture.manifest.Name, err)
		}
		views, err = registry.List(ctx, toolspkg.Scope{Operator: true})
		if err != nil {
			t.Fatalf("Registry.List(restored) error = %v", err)
		}
		if !extensionToolViewsContain(views, descriptor.Tool.ID) {
			t.Fatalf("Registry.List(restored) omitted %q", descriptor.Tool.ID)
		}
	})
}

func TestExtensionToolProviderDispatch(t *testing.T) {
	t.Parallel()

	t.Run("Should call extension tool handlers through Registry.Call", func(t *testing.T) {
		t.Parallel()

		env, fixture, descriptor := createExtensionToolProviderFixture(t, "ext-tool", true)
		runtime := newFakeExtensionToolRuntime(
			t,
			env.registry,
			fixture.manifest.Name,
			[]toolspkg.ExtensionToolRuntimeDescriptor{descriptor.RuntimeDescriptor},
		)
		runtime.callResult = toolspkg.ToolResult{
			Content: []toolspkg.ToolContent{{Type: "text", Text: "ok"}},
		}
		registry := newExtensionToolRegistry(t, env.registry, runtime, extensionToolPolicyAllowAll())

		result, err := registry.Call(testutil.Context(t), toolspkg.Scope{SessionID: "session-1"}, toolspkg.CallRequest{
			ToolID: descriptor.Tool.ID,
			Input:  json.RawMessage(`{"query":"alpha"}`),
		})
		if err != nil {
			t.Fatalf("Registry.Call() error = %v", err)
		}
		if got := result.Content[0].Text; got != "ok" {
			t.Fatalf("Result content = %q, want ok", got)
		}
		if len(runtime.calls) != 1 {
			t.Fatalf("runtime calls = %d, want 1", len(runtime.calls))
		}
		if got := runtime.calls[0].Handler; got != "search" {
			t.Fatalf("Call handler = %q, want search", got)
		}
	})

	t.Run("Should gate mutating extension tools on approval policy", func(t *testing.T) {
		t.Parallel()

		env, fixture, descriptor := createExtensionToolProviderFixture(t, "ext-mutating", false)
		runtime := newFakeExtensionToolRuntime(
			t,
			env.registry,
			fixture.manifest.Name,
			[]toolspkg.ExtensionToolRuntimeDescriptor{descriptor.RuntimeDescriptor},
		)
		registry := newExtensionToolRegistry(t, env.registry, runtime, toolspkg.PolicyInputs{
			SystemPermissionMode: toolspkg.PermissionModeApproveReads,
			ExternalDefault:      toolspkg.ExternalDefaultEnabled,
			ApprovalAvailable:    true,
		})

		view, err := registry.Get(testutil.Context(t), toolspkg.Scope{Operator: true}, descriptor.Tool.ID)
		if err != nil {
			t.Fatalf("Registry.Get() error = %v", err)
		}
		if !view.Decision.ApprovalRequired {
			t.Fatalf("Decision.ApprovalRequired = false, want true")
		}
		if !slices.Contains(view.Decision.ReasonCodes, toolspkg.ReasonApprovalRequired) {
			t.Fatalf("Decision reasons = %#v, want approval_required", view.Decision.ReasonCodes)
		}
	})
}

func TestExtensionToolProviderSubprocessIntegration(t *testing.T) {
	t.Run("Should dispatch a read-only tool through a real subprocess", func(t *testing.T) {
		env, fixture, descriptor, manager := startExtensionToolSubprocess(t, "ext-tool", "tool_provider", true)
		registry := newExtensionToolRegistry(t, env.registry, manager, extensionToolPolicyAllowAll())

		result, err := registry.Call(testutil.Context(t), toolspkg.Scope{SessionID: "session-1"}, toolspkg.CallRequest{
			ToolID: descriptor.Tool.ID,
			Input:  json.RawMessage(`{"query":"alpha"}`),
		})
		if err != nil {
			t.Fatalf("Registry.Call() error = %v", err)
		}
		if len(result.Content) != 1 || result.Content[0].Type != "text" {
			t.Fatalf("Result.Content = %#v, want text content", result.Content)
		}
		if _, err := manager.Get(fixture.manifest.Name); err != nil {
			t.Fatalf("Manager.Get(%q) error = %v", fixture.manifest.Name, err)
		}
	})

	t.Run("Should mark a mutating subprocess tool as requiring approval", func(t *testing.T) {
		env, _, descriptor, manager := startExtensionToolSubprocess(t, "ext-mutating", "tool_provider", false)
		registry := newExtensionToolRegistry(t, env.registry, manager, toolspkg.PolicyInputs{
			SystemPermissionMode: toolspkg.PermissionModeApproveReads,
			ExternalDefault:      toolspkg.ExternalDefaultEnabled,
			ApprovalAvailable:    true,
		})

		view, err := registry.Get(testutil.Context(t), toolspkg.Scope{Operator: true}, descriptor.Tool.ID)
		if err != nil {
			t.Fatalf("Registry.Get() error = %v", err)
		}
		if !view.Decision.ApprovalRequired {
			t.Fatalf("Decision.ApprovalRequired = false, want true")
		}
	})

	testCases := []struct {
		name     string
		scenario string
		want     toolspkg.ReasonCode
	}{
		{
			name:     "Should surface subprocess schema mismatch availability",
			scenario: "tool_runtime_schema_mismatch",
			want:     toolspkg.ReasonRuntimeDescriptorMismatch,
		},
		{
			name:     "Should surface subprocess handler mismatch availability",
			scenario: "tool_runtime_handler_mismatch",
			want:     toolspkg.ReasonExtensionRuntimeMismatch,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			env, _, descriptor, manager := startExtensionToolSubprocess(t, "ext-tool", tc.scenario, true)
			registry := newExtensionToolRegistry(t, env.registry, manager, extensionToolPolicyAllowAll())

			view, err := registry.Get(testutil.Context(t), toolspkg.Scope{Operator: true}, descriptor.Tool.ID)
			if err != nil {
				t.Fatalf("Registry.Get() error = %v", err)
			}
			if !slices.Contains(view.Availability.ReasonCodes, tc.want) {
				t.Fatalf("Availability reasons = %#v, want %q", view.Availability.ReasonCodes, tc.want)
			}
		})
	}

	t.Run("Should surface subprocess handler errors through Registry.Call", func(t *testing.T) {
		env, _, descriptor, manager := startExtensionToolSubprocess(t, "ext-tool", "tool_call_error", true)
		registry := newExtensionToolRegistry(t, env.registry, manager, extensionToolPolicyAllowAll())

		_, err := registry.Call(testutil.Context(t), toolspkg.Scope{SessionID: "session-1"}, toolspkg.CallRequest{
			ToolID: descriptor.Tool.ID,
			Input:  json.RawMessage(`{"query":"alpha"}`),
		})
		if !errors.Is(err, toolspkg.ErrToolBackendFailed) {
			t.Fatalf("Registry.Call() error = %v, want ErrToolBackendFailed", err)
		}
	})

	t.Run("Should propagate cancellation to subprocess tool calls", func(t *testing.T) {
		env, _, descriptor, manager := startExtensionToolSubprocess(t, "ext-tool", "tool_call_slow", true)
		registry := newExtensionToolRegistry(t, env.registry, manager, extensionToolPolicyAllowAll())
		ctx, cancel := context.WithTimeout(testutil.Context(t), 20*time.Millisecond)
		defer cancel()

		_, err := registry.Call(ctx, toolspkg.Scope{SessionID: "session-1"}, toolspkg.CallRequest{
			ToolID: descriptor.Tool.ID,
			Input:  json.RawMessage(`{"query":"alpha"}`),
		})
		if !errors.Is(err, toolspkg.ErrToolTimedOut) {
			t.Fatalf("Registry.Call() error = %v, want ErrToolTimedOut", err)
		}
	})
}

func TestExtensionToolProviderGoSDKSubprocessIntegration(t *testing.T) {
	t.Run("Should dispatch a read-only Go SDK tool through Registry.Call", func(t *testing.T) {
		binary := buildGoSDKToolProviderBinary(t, "go-sdk-tool", true)
		env, fixture, descriptor, manager := startCompiledGoSDKToolSubprocess(t, "go-sdk-tool", binary, true)
		registry := newExtensionToolRegistry(t, env.registry, manager, extensionToolPolicyAllowAll())

		result, err := registry.Call(testutil.Context(t), toolspkg.Scope{SessionID: "session-1"}, toolspkg.CallRequest{
			ToolID: descriptor.Tool.ID,
			Input:  json.RawMessage(`{"query":"alpha"}`),
		})
		if err != nil {
			t.Fatalf("Registry.Call() error = %v", err)
		}
		if len(result.Content) != 1 || result.Content[0].Text != "go-sdk:alpha" {
			t.Fatalf("Result.Content = %#v, want go-sdk:alpha", result.Content)
		}
		if _, err := manager.Get(fixture.manifest.Name); err != nil {
			t.Fatalf("Manager.Get(%q) error = %v", fixture.manifest.Name, err)
		}
	})

	t.Run("Should gate a mutating Go SDK tool on approval policy", func(t *testing.T) {
		binary := buildGoSDKToolProviderBinary(t, "go-sdk-mutating", false)
		env, _, descriptor, manager := startCompiledGoSDKToolSubprocess(t, "go-sdk-mutating", binary, false)
		registry := newExtensionToolRegistry(t, env.registry, manager, toolspkg.PolicyInputs{
			SystemPermissionMode: toolspkg.PermissionModeApproveReads,
			ExternalDefault:      toolspkg.ExternalDefaultEnabled,
			ApprovalAvailable:    true,
		})

		view, err := registry.Get(testutil.Context(t), toolspkg.Scope{Operator: true}, descriptor.Tool.ID)
		if err != nil {
			t.Fatalf("Registry.Get() error = %v", err)
		}
		if !view.Decision.ApprovalRequired {
			t.Fatalf("Decision.ApprovalRequired = false, want true")
		}
	})
}

type fakeExtensionToolRuntime struct {
	extension   *Extension
	descriptors []toolspkg.ExtensionToolRuntimeDescriptor
	calls       []toolspkg.ExtensionToolCallRequest
	callResult  toolspkg.ToolResult
	callErr     error
}

var _ ExtensionToolRuntime = (*fakeExtensionToolRuntime)(nil)

func (f *fakeExtensionToolRuntime) Get(string) (*Extension, error) {
	return f.extension, nil
}

func (f *fakeExtensionToolRuntime) ProvideTools(
	context.Context,
	string,
) ([]toolspkg.ExtensionToolRuntimeDescriptor, error) {
	return cloneRuntimeToolDescriptors(f.descriptors), nil
}

func (f *fakeExtensionToolRuntime) CallTool(
	_ context.Context,
	_ string,
	req toolspkg.ExtensionToolCallRequest,
) (toolspkg.ToolResult, error) {
	f.calls = append(f.calls, req)
	if f.callErr != nil {
		return toolspkg.ToolResult{}, f.callErr
	}
	return f.callResult, nil
}

func createExtensionToolProviderFixture(
	t *testing.T,
	name string,
	readOnly bool,
) (registryTestEnv, managerFixture, ManifestToolDescriptor) {
	t.Helper()

	env := newRegistryTestEnv(t)
	fixture := createExtensionToolTestExtension(t, name, "fake-extension", nil, nil, readOnly)
	installManagerFixture(t, env.registry, fixture, SourceUser, true)
	descriptors, err := ResolveManifestToolDescriptors(fixture.manifest)
	if err != nil {
		t.Fatalf("ResolveManifestToolDescriptors() error = %v", err)
	}
	if len(descriptors) != 1 {
		t.Fatalf("manifest tool descriptors = %d, want 1", len(descriptors))
	}
	return env, fixture, descriptors[0]
}

func newFakeExtensionToolRuntime(
	t *testing.T,
	registry *Registry,
	name string,
	descriptors []toolspkg.ExtensionToolRuntimeDescriptor,
) *fakeExtensionToolRuntime {
	t.Helper()

	info, err := registry.Get(name)
	if err != nil {
		t.Fatalf("registry.Get(%q) error = %v", name, err)
	}
	return &fakeExtensionToolRuntime{
		extension: &Extension{
			Info: *info,
			InitializeResult: &subprocess.InitializeResponse{
				AcceptedCapabilities: subprocess.AcceptedCapabilities{
					Provides: []string{extensionprotocol.CapabilityToolProvider},
				},
				ImplementedMethods: []string{
					string(extensionprotocol.ExtensionServiceMethodProvideTools),
					string(extensionprotocol.ExtensionServiceMethodToolsCall),
				},
			},
			Status: ExtensionStatus{
				Name:    name,
				Enabled: true,
				Active:  true,
				Healthy: true,
			},
		},
		descriptors: descriptors,
	}
}

func newExtensionToolRegistry(
	t *testing.T,
	extensionRegistry *Registry,
	runtime ExtensionToolRuntime,
	policyInputs toolspkg.PolicyInputs,
) *toolspkg.RuntimeRegistry {
	t.Helper()

	provider, err := NewExtensionToolProvider(extensionRegistry, func() ExtensionToolRuntime {
		return runtime
	})
	if err != nil {
		t.Fatalf("NewExtensionToolProvider() error = %v", err)
	}
	registry, err := toolspkg.NewRegistry(
		toolspkg.WithProviders(provider),
		toolspkg.WithPolicyInputs(policyInputs, toolspkg.ToolsetCatalog{}),
	)
	if err != nil {
		t.Fatalf("toolspkg.NewRegistry() error = %v", err)
	}
	return registry
}

func extensionToolPolicyAllowAll() toolspkg.PolicyInputs {
	return toolspkg.PolicyInputs{
		SystemPermissionMode: toolspkg.PermissionModeApproveAll,
		ExternalDefault:      toolspkg.ExternalDefaultEnabled,
		ApprovalAvailable:    true,
	}
}

func extensionToolViewsContain(views []toolspkg.ToolView, id toolspkg.ToolID) bool {
	for i := range views {
		if views[i].Descriptor.ID == id {
			return true
		}
	}
	return false
}

func startExtensionToolSubprocess(
	t *testing.T,
	name string,
	scenario string,
	readOnly bool,
) (registryTestEnv, managerFixture, ManifestToolDescriptor, *Manager) {
	t.Helper()

	env := newRegistryTestEnv(t)
	markerPath := filepath.Join(t.TempDir(), "tool-helper.log")
	fixture := createExtensionToolTestExtension(
		t,
		name,
		helperCommand(t),
		helperArgs(),
		helperEnv(scenario, markerPath),
		readOnly,
	)
	installManagerFixture(t, env.registry, fixture, SourceUser, true)
	manager := NewManager(
		env.registry,
		WithHealthCheckTimeout(20*time.Millisecond),
		WithSubprocessSignalGrace(15*time.Millisecond),
	)
	if err := manager.Start(testutil.Context(t)); err != nil {
		t.Fatalf("Manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Stop(testutil.Context(t)); err != nil {
			t.Fatalf("Manager.Stop() cleanup error = %v", err)
		}
	})
	waitForManagerCondition(t, time.Second, func() bool {
		extension, err := manager.Get(name)
		return err == nil && extension.Status.Active && extension.Status.Healthy
	})

	descriptors, err := ResolveManifestToolDescriptors(fixture.manifest)
	if err != nil {
		t.Fatalf("ResolveManifestToolDescriptors() error = %v", err)
	}
	if len(descriptors) != 1 {
		t.Fatalf("manifest tool descriptors = %d, want 1", len(descriptors))
	}
	return env, fixture, descriptors[0], manager
}

func startCompiledGoSDKToolSubprocess(
	t *testing.T,
	name string,
	command string,
	readOnly bool,
) (registryTestEnv, managerFixture, ManifestToolDescriptor, *Manager) {
	t.Helper()

	env := newRegistryTestEnv(t)
	fixture := createExtensionToolTestExtension(t, name, command, nil, nil, readOnly)
	installManagerFixture(t, env.registry, fixture, SourceUser, true)
	manager := NewManager(
		env.registry,
		WithHealthCheckTimeout(20*time.Millisecond),
		WithSubprocessSignalGrace(15*time.Millisecond),
	)
	if err := manager.Start(testutil.Context(t)); err != nil {
		t.Fatalf("Manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Stop(testutil.Context(t)); err != nil {
			t.Fatalf("Manager.Stop() cleanup error = %v", err)
		}
	})
	waitForManagerCondition(t, time.Second, func() bool {
		extension, err := manager.Get(name)
		return err == nil && extension.Status.Active && extension.Status.Healthy
	})

	descriptors, err := ResolveManifestToolDescriptors(fixture.manifest)
	if err != nil {
		t.Fatalf("ResolveManifestToolDescriptors() error = %v", err)
	}
	if len(descriptors) != 1 {
		t.Fatalf("manifest tool descriptors = %d, want 1", len(descriptors))
	}
	return env, fixture, descriptors[0], manager
}

func buildGoSDKToolProviderBinary(t *testing.T, name string, readOnly bool) string {
	t.Helper()

	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("filepath.Abs(repo root) error = %v", err)
	}
	dir := t.TempDir()
	writeFile(
		t,
		filepath.Join(dir, "go.mod"),
		"module example.com/"+name+"\n\ngo 1.26.4\n\nrequire github.com/compozy/agh v0.0.0\n",
	)
	writeFile(t, filepath.Join(dir, "main.go"), goSDKToolProviderSource(name, readOnly))

	edit := exec.CommandContext(
		testutil.Context(t),
		"go",
		"mod",
		"edit",
		"-replace",
		"github.com/compozy/agh="+repoRoot,
	)
	edit.Dir = dir
	if output, err := edit.CombinedOutput(); err != nil {
		t.Fatalf("go mod edit -replace error = %v\n%s", err, string(output))
	}

	binary := filepath.Join(dir, "extension-bin")
	build := exec.CommandContext(testutil.Context(t), "go", "build", "-o", binary, ".")
	build.Dir = dir
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build Go SDK extension error = %v\n%s", err, string(output))
	}
	return binary
}

func goSDKToolProviderSource(name string, readOnly bool) string {
	return fmt.Sprintf(`package main

import (
	"context"
	"fmt"
	"os"

	aghsdk "github.com/compozy/agh/sdk/go"
)

type searchInput struct {
	Query string %[1]q
}

func main() {
	extension := aghsdk.NewExtension(aghsdk.ExtensionDefinition{
		Name:    %[2]q,
		Version: "0.1.0",
		Capabilities: aghsdk.CapabilitiesConfig{
			Provides: []string{"tool.provider"},
		},
	})
	if err := aghsdk.Tool[searchInput](
		extension,
		"search",
		aghsdk.ToolOptions{
			ReadOnly:    %[3]t,
			InputSchema: map[string]any{
				"type": "object",
				"required": []string{"query"},
				"properties": map[string]any{
					"query": map[string]any{"type": "string"},
				},
			},
		},
		func(_ context.Context, req aghsdk.ToolRequest[searchInput]) (aghsdk.ToolResult, error) {
			return aghsdk.TextResult("go-sdk:" + req.Input.Query), nil
		},
	); err != nil {
		fmt.Fprintf(os.Stderr, "register tool: %%v\n", err)
		os.Exit(1)
	}
	if err := extension.Run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "run extension: %%v\n", err)
		os.Exit(1)
	}
}
`, `json:"query"`, name, readOnly)
}

func createExtensionToolTestExtension(
	t *testing.T,
	name string,
	command string,
	args []string,
	env map[string]string,
	readOnly bool,
) managerFixture {
	t.Helper()

	dir := t.TempDir()
	writeFile(
		t,
		filepath.Join(dir, manifestJSONFileName),
		extensionToolManifestJSON(name, command, args, env, readOnly),
	)
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

func extensionToolManifestJSON(
	name string,
	command string,
	args []string,
	env map[string]string,
	readOnly bool,
) string {
	risk := toolspkg.RiskMutating
	if readOnly {
		risk = toolspkg.RiskRead
	}
	subprocessConfig := map[string]any{
		"command": command,
	}
	if len(args) > 0 {
		subprocessConfig["args"] = args
	}
	if len(env) > 0 {
		subprocessConfig["env"] = env
	}
	payload := map[string]any{
		"extension": map[string]any{
			"name":            name,
			"version":         "0.1.0",
			"description":     "Extension tool provider test fixture",
			"min_agh_version": "0.5.0",
		},
		"capabilities": map[string]any{
			"provides": []string{extensionprotocol.CapabilityToolProvider},
		},
		"subprocess": subprocessConfig,
		"resources": map[string]any{
			"tools": map[string]any{
				"search": map[string]any{
					"description": "Search extension data",
					"read_only":   readOnly,
					"visibility":  toolspkg.VisibilitySession,
					"risk":        risk,
					"backend": map[string]any{
						"kind":    toolspkg.BackendExtensionHost,
						"handler": "search",
					},
					"input_schema": map[string]any{
						"type":     "object",
						"required": []string{"query"},
						"properties": map[string]any{
							"query": map[string]any{"type": "string"},
						},
					},
				},
			},
		},
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		panic(fmt.Sprintf("marshal extension tool manifest fixture: %v", err))
	}
	return string(data)
}
