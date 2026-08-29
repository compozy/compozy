package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"path/filepath"
	"testing"

	speccycle "github.com/compozy/compozy/extensions/spec-cycle"
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	extensionprotocol "github.com/compozy/compozy/internal/extensionprotocol"
	looppkg "github.com/compozy/compozy/internal/loop"
	looppkgdsl "github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/subprocess"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

type loopToolSchemaContextKey struct{}

func TestLoopToolSchemaSource(t *testing.T) {
	t.Parallel()

	t.Run("Should expose registry descriptor schemas as loop snapshots", func(t *testing.T) {
		t.Parallel()

		descriptor := loopToolSchemaDescriptor(t)
		source := newLoopToolSchemaSource(context.Background(), loopToolSchemaRegistry{
			views: map[toolspkg.ToolID]toolspkg.ToolView{
				descriptor.ID: {Descriptor: descriptor},
			},
		})

		snapshot, ok := source.Snapshot(descriptor.ID.String())
		if !ok {
			t.Fatalf("Snapshot(%q) ok = false, want true", descriptor.ID)
		}
		if got, want := snapshot.ToolID, descriptor.ID.String(); got != want {
			t.Fatalf("snapshot.ToolID = %q, want %q", got, want)
		}
		if !json.Valid(snapshot.InputSchema) {
			t.Fatalf("snapshot.InputSchema = %s, want valid JSON", snapshot.InputSchema)
		}
		if !json.Valid(snapshot.OutputSchema) {
			t.Fatalf("snapshot.OutputSchema = %s, want valid JSON", snapshot.OutputSchema)
		}
		if got, want := snapshot.InputSchemaDigest, descriptor.InputSchemaDigest; got != want {
			t.Fatalf("snapshot.InputSchemaDigest = %q, want %q", got, want)
		}
		if got, want := snapshot.OutputSchemaDigest, descriptor.OutputSchemaDigest; got != want {
			t.Fatalf("snapshot.OutputSchemaDigest = %q, want %q", got, want)
		}

		snapshot.InputSchema[0] = '['
		next, ok := source.Snapshot(descriptor.ID.String())
		if !ok {
			t.Fatalf("Snapshot(%q) after mutation ok = false, want true", descriptor.ID)
		}
		if got, want := next.InputSchema[0], byte('{'); got != want {
			t.Fatalf("next.InputSchema[0] = %q, want %q", got, want)
		}
	})

	t.Run("Should reject invalid or unknown tool identifiers", func(t *testing.T) {
		t.Parallel()

		source := newLoopToolSchemaSource(
			context.Background(),
			loopToolSchemaRegistry{views: map[toolspkg.ToolID]toolspkg.ToolView{}},
		)
		if _, ok := source.Snapshot("not a valid tool id"); ok {
			t.Fatal("Snapshot(invalid id) ok = true, want false")
		}
		if _, ok := source.Snapshot("ext__spec_cycle__missing"); ok {
			t.Fatal("Snapshot(unknown id) ok = true, want false")
		}
	})

	t.Run("Should project the registry once for all requested schemas", func(t *testing.T) {
		t.Parallel()

		first := loopToolSchemaDescriptor(t)
		second := first
		second.ID = toolspkg.ToolID("ext__spec_cycle__second")
		listCalls := 0
		getCalls := 0
		source := newLoopToolSchemaSource(context.Background(), loopToolSchemaRegistry{
			views: map[toolspkg.ToolID]toolspkg.ToolView{
				first.ID:  {Descriptor: first},
				second.ID: {Descriptor: second},
			},
			listCalls: &listCalls,
			getCalls:  &getCalls,
		})

		for _, id := range []toolspkg.ToolID{first.ID, second.ID, first.ID} {
			if _, ok := source.Snapshot(id.String()); !ok {
				t.Fatalf("Snapshot(%q) ok = false, want true", id)
			}
		}
		if got, want := listCalls, 1; got != want {
			t.Fatalf("registry List calls = %d, want %d", got, want)
		}
		if got, want := getCalls, 0; got != want {
			t.Fatalf("registry Get calls = %d, want %d", got, want)
		}
	})

	t.Run("Should pass caller context to registry lookups", func(t *testing.T) {
		t.Parallel()

		const marker = "schema-projection"
		var captured any
		ctx := context.WithValue(t.Context(), loopToolSchemaContextKey{}, marker)
		descriptor := loopToolSchemaDescriptor(t)
		source := newLoopToolSchemaSource(ctx, loopToolSchemaRegistry{
			views: map[toolspkg.ToolID]toolspkg.ToolView{
				descriptor.ID: {Descriptor: descriptor},
			},
			listContextValue: &captured,
		})

		if _, ok := source.Snapshot(descriptor.ID.String()); !ok {
			t.Fatalf("Snapshot(%q) ok = false, want true", descriptor.ID)
		}
		if captured != marker {
			t.Fatalf("registry List context marker = %#v, want %q", captured, marker)
		}
	})

	t.Run("Should stop before registry lookup when caller context is canceled", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		descriptor := loopToolSchemaDescriptor(t)
		listCalls := 0
		source := newLoopToolSchemaSource(ctx, loopToolSchemaRegistry{
			views: map[toolspkg.ToolID]toolspkg.ToolView{
				descriptor.ID: {Descriptor: descriptor},
			},
			listCalls: &listCalls,
		})

		if _, ok := source.Snapshot(descriptor.ID.String()); ok {
			t.Fatalf("Snapshot(%q) ok = true, want false after canceled context", descriptor.ID)
		}
		if listCalls != 0 {
			t.Fatalf("registry List calls = %d, want 0", listCalls)
		}
	})

	t.Run("Should disable schema source when registry is unavailable", func(t *testing.T) {
		t.Parallel()

		var missingContext context.Context
		if source := newLoopToolSchemaSource(missingContext, loopToolSchemaRegistry{}); source != nil {
			t.Fatalf("newLoopToolSchemaSource(nil context) = %T, want nil", source)
		}
		if source := newLoopToolSchemaSource(context.Background(), nil); source != nil {
			t.Fatalf("newLoopToolSchemaSource(nil) = %T, want nil", source)
		}
	})

	t.Run("Should compile implement-tasks through registry backed spec-cycle schemas", func(t *testing.T) {
		t.Parallel()

		source := newBundledSpecCycleLoopSchemaSource(t)
		data, err := fs.ReadFile(speccycle.FS(), "loops/implement-tasks/loop.yaml")
		if err != nil {
			t.Fatalf("ReadFile(implement-tasks loop) error = %v", err)
		}
		linter := newLoopLinterWithSchemaSource(source)
		spec, def, err := looppkg.ParseResource(data, looppkg.ResourceParseOptions{
			Source:                 looppkg.SourceMarketplace,
			Dir:                    "loops/implement-tasks",
			FilePath:               "loops/implement-tasks/loop.yaml",
			InstalledFromExtension: speccycle.Name,
			Linter:                 linter,
		})
		if err != nil {
			t.Fatalf("ParseResource(implement-tasks) error = %v", err)
		}
		if got, want := spec.Name, "implement-tasks"; got != want {
			t.Fatalf("spec.Name = %q, want %q", got, want)
		}

		resolved, err := newLoopCompilerWithSchemaSource(source).Compile(def)
		if err != nil {
			t.Fatalf("Compile(implement-tasks) error = %v", err)
		}
		if _, ok := resolved.Templates["nodes.implement.collection"]; !ok {
			t.Fatal("Compile(implement-tasks) missing implement collection template")
		}
	})

	t.Run("Should call only tools owned by the extension that installed the Loop", func(t *testing.T) {
		t.Parallel()

		const (
			owner       = "release-automation"
			ownToolID   = toolspkg.ToolID("ext__release_automation__prepare")
			otherToolID = toolspkg.ToolID("ext__artifact_store__write")
		)
		ownCalled := false
		ownProvider, err := toolspkg.NewNativeProvider(
			toolspkg.SourceRef{Kind: toolspkg.SourceExtension, Owner: owner},
			toolspkg.NativeTool{
				Descriptor: loopExtensionActionDescriptor(ownToolID, owner),
				Call: func(context.Context, toolspkg.Scope, toolspkg.CallRequest) (toolspkg.ToolResult, error) {
					ownCalled = true
					return toolspkg.ToolResult{Structured: json.RawMessage(`{"status":"prepared"}`)}, nil
				},
			},
		)
		if err != nil {
			t.Fatalf("NewNativeProvider(own extension) error = %v", err)
		}
		otherCalled := false
		otherProvider, err := toolspkg.NewNativeProvider(
			toolspkg.SourceRef{Kind: toolspkg.SourceExtension, Owner: "artifact-store"},
			toolspkg.NativeTool{
				Descriptor: loopExtensionActionDescriptor(otherToolID, "artifact-store"),
				Call: func(context.Context, toolspkg.Scope, toolspkg.CallRequest) (toolspkg.ToolResult, error) {
					otherCalled = true
					return toolspkg.ToolResult{Structured: json.RawMessage(`{"status":"written"}`)}, nil
				},
			},
		)
		if err != nil {
			t.Fatalf("NewNativeProvider(foreign extension) error = %v", err)
		}
		cfg := testConfig(t, testHomePaths(t))
		cfg.Permissions.Mode = compozyconfig.PermissionModeApproveAll
		cfg.Tools.Policy.ExternalDefault = compozyconfig.ToolsExternalDefaultDisabled
		policy, err := newNativeToolPolicyResolver(nativeToolPolicyResolverDeps{
			Config:            &cfg,
			ApprovalAvailable: true,
		})
		if err != nil {
			t.Fatalf("newNativeToolPolicyResolver() error = %v", err)
		}
		registry, err := toolspkg.NewRegistry(
			toolspkg.WithProviders(ownProvider, otherProvider),
			toolspkg.WithPolicyInputResolver(policy, toolspkg.ToolsetCatalog{}),
		)
		if err != nil {
			t.Fatalf("NewRegistry() error = %v", err)
		}
		actions, err := looppkg.NewActionRegistry(registry)
		if err != nil {
			t.Fatalf("NewActionRegistry() error = %v", err)
		}
		scope := toolspkg.Scope{ActorKind: string(workspaceaccess.ActorDaemon)}
		ctx := toolspkg.WithTrustedExtensionOwner(t.Context(), owner)
		ownerGrant := toolspkg.SourceGrant{Kind: toolspkg.SourceExtension, Owner: owner}
		inputs, err := policy.Resolve(ctx, scope)
		if err != nil {
			t.Fatalf("Resolve(loop-owned policy) error = %v", err)
		}
		if !sourceGrantExists(inputs.AllowSources, ownerGrant) {
			t.Fatalf("loop-owned allow sources = %#v, want %#v", inputs.AllowSources, ownerGrant)
		}
		if sourceGrantExists(inputs.TrustedSources, ownerGrant) {
			t.Fatalf("loop-owned trusted sources = %#v, want no permission bypass", inputs.TrustedSources)
		}

		executor, err := actions.Resolve(ctx, scope, ownToolID.String())
		if err != nil {
			t.Fatalf("Resolve(own extension tool) error = %v", err)
		}
		_, err = executor.Execute(ctx, loopActionNode(ownToolID), looppkg.ActionExecutionInput{
			ToolScope: scope,
		})
		if err != nil {
			t.Fatalf("Execute(own extension tool) error = %v", err)
		}
		if !ownCalled {
			t.Fatal("own extension tool called = false, want true")
		}

		_, err = actions.Resolve(ctx, scope, otherToolID.String())
		if !errors.Is(err, looppkg.ErrActionUnknownKind) {
			t.Fatalf("Resolve(foreign extension tool) error = %v, want ErrActionUnknownKind", err)
		}
		if otherCalled {
			t.Fatal("foreign extension tool called = true, want false")
		}

		untrustedScope := toolspkg.Scope{ActorKind: string(workspaceaccess.ActorAgentSession)}
		_, err = actions.Resolve(ctx, untrustedScope, ownToolID.String())
		if !errors.Is(err, looppkg.ErrActionUnknownKind) {
			t.Fatalf("Resolve(agent-forged owner) error = %v, want ErrActionUnknownKind", err)
		}
	})
}

func loopExtensionActionDescriptor(id toolspkg.ToolID, owner string) toolspkg.Descriptor {
	return toolspkg.Descriptor{
		ID:               id,
		ToolPresentation: toolspkg.NewToolPresentation("Prepare release", "", ""),
		Description:      "Prepare a release artifact.",
		InputSchema:      json.RawMessage(`{"type":"object","additionalProperties":false}`),
		OutputSchema: json.RawMessage(
			`{"type":"object","required":["status"],"properties":{"status":{"type":"string"}}}`,
		),
		Backend: toolspkg.BackendRef{
			Kind:       toolspkg.BackendNativeGo,
			NativeName: "prepare",
		},
		Source:     toolspkg.SourceRef{Kind: toolspkg.SourceExtension, Owner: owner},
		Visibility: toolspkg.VisibilitySession,
		Risk:       toolspkg.RiskRead,
		ReadOnly:   true,
	}
}

func loopActionNode(id toolspkg.ToolID) looppkgdsl.Node {
	return looppkgdsl.Node{
		ID:    "prepare",
		Class: looppkgdsl.NodeClassAction,
		Kind:  id.String(),
	}
}

type specCycleLoopSchemaRuntime struct {
	extension   *extensionpkg.Extension
	descriptors []toolspkg.ExtensionToolRuntimeDescriptor
}

var _ extensionpkg.ExtensionToolRuntime = (*specCycleLoopSchemaRuntime)(nil)

func (r *specCycleLoopSchemaRuntime) Get(name string) (*extensionpkg.Extension, error) {
	if name != speccycle.Name {
		return nil, extensionpkg.ErrExtensionNotFound
	}
	extension := *r.extension
	return &extension, nil
}

func (r *specCycleLoopSchemaRuntime) ProvideTools(
	context.Context,
	string,
) ([]toolspkg.ExtensionToolRuntimeDescriptor, error) {
	descriptors := make([]toolspkg.ExtensionToolRuntimeDescriptor, len(r.descriptors))
	for i := range r.descriptors {
		descriptors[i] = r.descriptors[i]
		descriptors[i].Capabilities = append([]string(nil), r.descriptors[i].Capabilities...)
	}
	return descriptors, nil
}

func (r *specCycleLoopSchemaRuntime) CallTool(
	context.Context,
	string,
	toolspkg.ExtensionToolCallRequest,
) (toolspkg.ToolResult, error) {
	return toolspkg.ToolResult{}, errors.New("unexpected spec-cycle tool call")
}

func newBundledSpecCycleLoopSchemaSource(t *testing.T) looppkg.ToolSchemaSource {
	t.Helper()

	homePaths, err := compozyconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	globalDB, err := openDaemonTestGlobalDBAtPath(t.Context(), homePaths.DatabaseFile)
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := globalDB.Close(context.Background()); err != nil {
			t.Fatalf("Close(globalDB) error = %v", err)
		}
	})

	extensionRegistry := extensionpkg.NewRegistry(globalDB.DB())
	if err := speccycle.EnsureManagedInstall(homePaths, extensionRegistry); err != nil {
		t.Fatalf("EnsureManagedInstall() error = %v", err)
	}
	if err := extensionRegistry.Enable(speccycle.Name); err != nil {
		t.Fatalf("registry.Enable(%q) error = %v", speccycle.Name, err)
	}
	runtime := newSpecCycleLoopSchemaRuntime(t, extensionRegistry)
	provider, err := extensionpkg.NewExtensionToolProvider(extensionRegistry, func() extensionpkg.ExtensionToolRuntime {
		return runtime
	})
	if err != nil {
		t.Fatalf("NewExtensionToolProvider() error = %v", err)
	}
	toolRegistry, err := toolspkg.NewRegistry(
		toolspkg.WithProviders(provider),
		toolspkg.WithPolicyInputs(extensionToolPolicyAllowAll(), toolspkg.ToolsetCatalog{}),
	)
	if err != nil {
		t.Fatalf("toolspkg.NewRegistry() error = %v", err)
	}
	return newLoopToolSchemaSource(t.Context(), toolRegistry)
}

func newSpecCycleLoopSchemaRuntime(
	t *testing.T,
	registry *extensionpkg.Registry,
) *specCycleLoopSchemaRuntime {
	t.Helper()

	info, err := registry.Get(speccycle.Name)
	if err != nil {
		t.Fatalf("registry.Get(%q) error = %v", speccycle.Name, err)
	}
	manifest, err := extensionpkg.LoadManifest(filepath.Dir(info.ManifestPath))
	if err != nil {
		t.Fatalf("LoadManifest(%q) error = %v", info.ManifestPath, err)
	}
	toolDescriptors, err := extensionpkg.ResolveManifestToolDescriptors(manifest)
	if err != nil {
		t.Fatalf("ResolveManifestToolDescriptors() error = %v", err)
	}
	runtimeDescriptors := make([]toolspkg.ExtensionToolRuntimeDescriptor, 0, len(toolDescriptors))
	for i := range toolDescriptors {
		runtimeDescriptors = append(runtimeDescriptors, toolDescriptors[i].RuntimeDescriptor)
	}

	return &specCycleLoopSchemaRuntime{
		extension: &extensionpkg.Extension{
			Info:     *info,
			Manifest: manifest,
			RootDir:  filepath.Dir(info.ManifestPath),
			InitializeResult: &subprocess.InitializeResponse{
				AcceptedCapabilities: subprocess.AcceptedCapabilities{
					Provides: []string{extensionprotocol.CapabilityToolProvider},
				},
				ImplementedMethods: []string{
					string(extensionprotocol.ExtensionServiceMethodProvideTools),
					string(extensionprotocol.ExtensionServiceMethodToolsCall),
				},
			},
			Status: extensionpkg.ExtensionStatus{
				Name:    speccycle.Name,
				Version: info.Version,
				Source:  info.Source,
				Enabled: true,
				Active:  true,
				Healthy: true,
			},
		},
		descriptors: runtimeDescriptors,
	}
}

func extensionToolPolicyAllowAll() toolspkg.PolicyInputs {
	return toolspkg.PolicyInputs{
		SystemPermissionMode: toolspkg.PermissionModeApproveAll,
		ExternalDefault:      toolspkg.ExternalDefaultEnabled,
		ApprovalAvailable:    true,
	}
}

type loopToolSchemaRegistry struct {
	views            map[toolspkg.ToolID]toolspkg.ToolView
	listCalls        *int
	getCalls         *int
	listContextValue *any
}

func (r loopToolSchemaRegistry) List(ctx context.Context, _ toolspkg.Scope) ([]toolspkg.ToolView, error) {
	if r.listCalls != nil {
		*r.listCalls++
	}
	if r.listContextValue != nil {
		*r.listContextValue = ctx.Value(loopToolSchemaContextKey{})
	}
	views := make([]toolspkg.ToolView, 0, len(r.views))
	for id := range r.views {
		views = append(views, r.views[id])
	}
	return views, nil
}

func (r loopToolSchemaRegistry) Search(
	context.Context,
	toolspkg.Scope,
	toolspkg.SearchQuery,
) ([]toolspkg.ToolView, error) {
	return r.List(context.Background(), toolspkg.Scope{})
}

func (r loopToolSchemaRegistry) Get(
	ctx context.Context,
	_ toolspkg.Scope,
	id toolspkg.ToolID,
) (toolspkg.ToolView, error) {
	if r.getCalls != nil {
		*r.getCalls++
	}
	if err := ctx.Err(); err != nil {
		return toolspkg.ToolView{}, err
	}
	view, ok := r.views[id]
	if !ok {
		return toolspkg.ToolView{}, errors.New("tool not found")
	}
	return view, nil
}

func (r loopToolSchemaRegistry) Call(
	context.Context,
	toolspkg.Scope,
	toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	return toolspkg.ToolResult{}, errors.New("unexpected tool call")
}

func loopToolSchemaDescriptor(t *testing.T) toolspkg.Descriptor {
	t.Helper()

	descriptor := toolspkg.Descriptor{
		ID:               "ext__spec_cycle__finalize_review_round",
		ToolPresentation: toolspkg.NewToolPresentation("Finalize review round", "", ""),
		Description:      "Resolve triaged review issue artifacts.",
		InputSchema: json.RawMessage(
			`{"type":"object","required":["task_name","round"],"properties":{"task_name":{"type":"string"},"round":{"type":"integer"}}}`,
		),
		OutputSchema: json.RawMessage(
			`{"type":"object","properties":{"resolved":{"type":"integer"},"invalid":{"type":"integer"},"pending":{"type":"integer"}}}`,
		),
		Backend: toolspkg.BackendRef{
			Kind:       toolspkg.BackendNativeGo,
			NativeName: "finalize_review_round",
		},
		Source: toolspkg.SourceRef{
			Kind:  toolspkg.SourceExtension,
			Owner: "spec-cycle",
		},
		Visibility:      toolspkg.VisibilityModel,
		Risk:            toolspkg.RiskMutating,
		ReadOnly:        false,
		ConcurrencySafe: false,
		MaxResultBytes:  4096,
	}
	withDigests, err := toolspkg.DescriptorWithSchemaDigests(descriptor)
	if err != nil {
		t.Fatalf("DescriptorWithSchemaDigests() error = %v", err)
	}
	return withDigests
}
