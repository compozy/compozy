package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"path/filepath"
	"testing"

	devcycle "github.com/compozy/agh/extensions/dev-cycle"
	aghconfig "github.com/compozy/agh/internal/config"
	extensionpkg "github.com/compozy/agh/internal/extension"
	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/store/globaldb"
	"github.com/compozy/agh/internal/subprocess"
	toolspkg "github.com/compozy/agh/internal/tools"
)

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
		if _, ok := source.Snapshot("ext__dev_cycle__missing"); ok {
			t.Fatal("Snapshot(unknown id) ok = true, want false")
		}
	})

	t.Run("Should pass caller context to registry lookups", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		descriptor := loopToolSchemaDescriptor(t)
		source := newLoopToolSchemaSource(ctx, loopToolSchemaRegistry{
			views: map[toolspkg.ToolID]toolspkg.ToolView{
				descriptor.ID: {Descriptor: descriptor},
			},
		})

		if _, ok := source.Snapshot(descriptor.ID.String()); ok {
			t.Fatalf("Snapshot(%q) ok = true, want false after canceled context", descriptor.ID)
		}
	})

	t.Run("Should disable schema source when registry is unavailable", func(t *testing.T) {
		t.Parallel()

		if source := newLoopToolSchemaSource(context.Background(), nil); source != nil {
			t.Fatalf("newLoopToolSchemaSource(nil) = %T, want nil", source)
		}
	})

	t.Run("Should compile software-delivery through registry backed dev-cycle schemas", func(t *testing.T) {
		t.Parallel()

		source := newBundledDevCycleLoopSchemaSource(t)
		data, err := fs.ReadFile(devcycle.FS(), "loops/software-delivery/loop.yaml")
		if err != nil {
			t.Fatalf("ReadFile(software-delivery loop) error = %v", err)
		}
		linter := newLoopLinterWithSchemaSource(source)
		spec, def, err := looppkg.ParseResource(data, looppkg.ResourceParseOptions{
			Source:                 looppkg.SourceMarketplace,
			Dir:                    "loops/software-delivery",
			FilePath:               "loops/software-delivery/loop.yaml",
			InstalledFromExtension: devcycle.Name,
			Linter:                 linter,
		})
		if err != nil {
			t.Fatalf("ParseResource(software-delivery) error = %v", err)
		}
		if got, want := spec.Name, "software-delivery"; got != want {
			t.Fatalf("spec.Name = %q, want %q", got, want)
		}

		resolved, err := newLoopCompilerWithSchemaSource(source).Compile(def)
		if err != nil {
			t.Fatalf("Compile(software-delivery) error = %v", err)
		}
		if _, ok := resolved.Templates["nodes.implement.collection"]; !ok {
			t.Fatal("Compile(software-delivery) missing implement collection template")
		}
	})
}

type devCycleLoopSchemaRuntime struct {
	extension   *extensionpkg.Extension
	descriptors []toolspkg.ExtensionToolRuntimeDescriptor
}

var _ extensionpkg.ExtensionToolRuntime = (*devCycleLoopSchemaRuntime)(nil)

func (r *devCycleLoopSchemaRuntime) Get(name string) (*extensionpkg.Extension, error) {
	if name != devcycle.Name {
		return nil, extensionpkg.ErrExtensionNotFound
	}
	extension := *r.extension
	return &extension, nil
}

func (r *devCycleLoopSchemaRuntime) ProvideTools(
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

func (r *devCycleLoopSchemaRuntime) CallTool(
	context.Context,
	string,
	toolspkg.ExtensionToolCallRequest,
) (toolspkg.ToolResult, error) {
	return toolspkg.ToolResult{}, errors.New("unexpected dev-cycle tool call")
}

func newBundledDevCycleLoopSchemaSource(t *testing.T) looppkg.ToolSchemaSource {
	t.Helper()

	homePaths, err := aghconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	globalDB, err := globaldb.OpenGlobalDB(t.Context(), homePaths.DatabaseFile)
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := globalDB.Close(context.Background()); err != nil {
			t.Fatalf("Close(globalDB) error = %v", err)
		}
	})

	extensionRegistry := extensionpkg.NewRegistry(globalDB.DB())
	if err := devcycle.EnsureManagedInstall(homePaths, extensionRegistry); err != nil {
		t.Fatalf("EnsureManagedInstall() error = %v", err)
	}
	runtime := newDevCycleLoopSchemaRuntime(t, extensionRegistry)
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

func newDevCycleLoopSchemaRuntime(
	t *testing.T,
	registry *extensionpkg.Registry,
) *devCycleLoopSchemaRuntime {
	t.Helper()

	info, err := registry.Get(devcycle.Name)
	if err != nil {
		t.Fatalf("registry.Get(%q) error = %v", devcycle.Name, err)
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

	return &devCycleLoopSchemaRuntime{
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
				Name:    devcycle.Name,
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
	views map[toolspkg.ToolID]toolspkg.ToolView
}

func (r loopToolSchemaRegistry) List(context.Context, toolspkg.Scope) ([]toolspkg.ToolView, error) {
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
		ID:           "ext__dev_cycle__git_push",
		DisplayTitle: "Git Push",
		Description:  "Push the current branch to a remote.",
		InputSchema:  json.RawMessage(`{"type":"object","properties":{"remote":{"type":"string"}}}`),
		OutputSchema: json.RawMessage(`{"type":"object","properties":{"pushed":{"type":"boolean"}}}`),
		Backend: toolspkg.BackendRef{
			Kind:       toolspkg.BackendNativeGo,
			NativeName: "git_push",
		},
		Source: toolspkg.SourceRef{
			Kind:  toolspkg.SourceExtension,
			Owner: "dev-cycle",
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
