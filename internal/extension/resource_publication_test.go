package extensionpkg

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"testing"

	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/testutil"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func TestResolveManifestToolResourcesMatchesDynamicSnapshotCanonicalShape(t *testing.T) {
	t.Parallel()

	t.Run("Should Match Dynamic Snapshot Canonical Shape", func(t *testing.T) {
		t.Parallel()

		manifest := &Manifest{
			Name: "linear",
			Resources: ResourcesConfig{
				Tools: map[string]ToolConfig{
					" lookup ": {
						Description: " search workspace ",
						Backend: ToolBackendConfig{
							Kind:    "extension_host",
							Handler: "lookup",
						},
						InputSchema: json.RawMessage(`{
						"properties": {"path": {"type": "string"}},
						"type": "object"
					}`),
						ReadOnly: true,
					},
				},
			},
		}

		tools, err := ResolveManifestToolResources(manifest)
		if err != nil {
			t.Fatalf("ResolveManifestToolResources() error = %v", err)
		}
		if got, want := len(tools), 1; got != want {
			t.Fatalf("len(ResolveManifestToolResources()) = %d, want %d", got, want)
		}

		codec, err := toolspkg.NewResourceCodec()
		if err != nil {
			t.Fatalf("toolspkg.NewResourceCodec() error = %v", err)
		}
		scope := resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal}

		manifestCanonical := mustCanonicalToolJSON(t, codec, scope, tools[0])
		dynamicSpec, err := codec.DecodeAndValidate(testutil.Context(t), scope, []byte(`{
		"id": "ext__linear__lookup",
		"display_title": "lookup",
		"description": "search workspace",
		"backend": {
			"kind": "extension_host",
			"extension_id": "linear",
			"handler": "lookup",
			"requires_capabilities": ["tool.provider"]
		},
		"input_schema": {
			"type": "object",
			"properties": {"path": {"type": "string"}}
		},
		"source": {
			"kind": "extension",
			"owner": "linear",
			"raw_tool_name": "lookup"
		},
		"visibility": "operator",
		"risk": "read",
		"read_only": true,
		"concurrency_safe": true
	}`))
		if err != nil {
			t.Fatalf("codec.DecodeAndValidate(dynamic) error = %v", err)
		}
		dynamicCanonical := mustCanonicalToolJSON(t, codec, scope, dynamicSpec)

		if !bytes.Equal(manifestCanonical, dynamicCanonical) {
			t.Fatalf(
				"manifest canonical tool != dynamic canonical tool\nmanifest=%s\ndynamic=%s",
				string(manifestCanonical),
				string(dynamicCanonical),
			)
		}
	})
}

func TestResolveManifestToolDescriptorsIncludesDigestAndMetadata(t *testing.T) {
	t.Parallel()

	t.Run("Should Include Digest And Metadata", func(t *testing.T) {
		t.Parallel()

		inputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {"type": "string"}
		},
		"required": ["query"]
	}`)
		outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"ok": {"type": "boolean"}
		}
	}`)
		manifest := &Manifest{
			Name: "linear",
			Resources: ResourcesConfig{
				Tools: map[string]ToolConfig{
					"lookup": {
						ID:             "ext__linear__lookup",
						DisplayTitle:   "Linear Lookup",
						FriendlyVerb:   "Looking up",
						Preview:        "arg:query",
						Description:    "Search workspace",
						Backend:        ToolBackendConfig{Kind: "extension_host", Handler: "lookup.run"},
						InputSchema:    inputSchema,
						OutputSchema:   outputSchema,
						ReadOnly:       true,
						MaxResultBytes: 4096,
						Toolsets:       []string{"ext__linear__read"},
						Tags:           []string{"search"},
						SearchHints:    []string{"issues"},
						RequiredCapabilities: []string{
							"memory.read",
						},
						Visibility: "session",
					},
				},
			},
		}

		descriptors, err := ResolveManifestToolDescriptors(manifest)
		if err != nil {
			t.Fatalf("ResolveManifestToolDescriptors() error = %v", err)
		}
		if got, want := len(descriptors), 1; got != want {
			t.Fatalf("len(ResolveManifestToolDescriptors()) = %d, want %d", got, want)
		}

		descriptor := descriptors[0]
		if got, want := descriptor.Tool.ID, toolspkg.ToolID("ext__linear__lookup"); got != want {
			t.Fatalf("Tool.ID = %q, want %q", got, want)
		}
		if got, want := descriptor.Tool.Backend.Handler, "lookup.run"; got != want {
			t.Fatalf("Tool.Backend.Handler = %q, want %q", got, want)
		}
		if got, want := descriptor.Tool.Visibility, toolspkg.VisibilitySession; got != want {
			t.Fatalf("Tool.Visibility = %q, want %q", got, want)
		}
		if got, want := descriptor.Tool.MaxResultBytes, int64(4096); got != want {
			t.Fatalf("Tool.MaxResultBytes = %d, want %d", got, want)
		}
		presentation := descriptor.Tool.Presentation()
		if got, want := presentation.FriendlyVerb, "Looking up"; got != want {
			t.Fatalf("Tool presentation friendly verb = %q, want %q", got, want)
		}
		if got, want := presentation.Preview, "arg:query"; got != want {
			t.Fatalf("Tool presentation preview = %q, want %q", got, want)
		}
		if got, want := descriptor.Tool.Toolsets, []toolspkg.ToolsetID{"ext__linear__read"}; !slices.Equal(got, want) {
			t.Fatalf("Tool.Toolsets = %#v, want %#v", got, want)
		}
		wantInputDigest, err := toolspkg.SchemaDigest(inputSchema)
		if err != nil {
			t.Fatalf("SchemaDigest(input) error = %v", err)
		}
		wantOutputDigest, err := toolspkg.SchemaDigest(outputSchema)
		if err != nil {
			t.Fatalf("SchemaDigest(output) error = %v", err)
		}
		runtime := descriptor.RuntimeDescriptor
		if got, want := runtime.InputSchemaDigest, wantInputDigest; got != want {
			t.Fatalf("RuntimeDescriptor.InputSchemaDigest = %q, want %q", got, want)
		}
		if got, want := runtime.OutputSchemaDigest, wantOutputDigest; got != want {
			t.Fatalf("RuntimeDescriptor.OutputSchemaDigest = %q, want %q", got, want)
		}
		if got, want := runtime.Handler, "lookup.run"; got != want {
			t.Fatalf("RuntimeDescriptor.Handler = %q, want %q", got, want)
		}
		if got, want := runtime.Capabilities, []string{"memory.read", "tool.provider"}; !slices.Equal(got, want) {
			t.Fatalf("RuntimeDescriptor.Capabilities = %#v, want %#v", got, want)
		}
	})
}

func TestManifestToolResourcesRemainColdUntilRuntimeHandleExists(t *testing.T) {
	t.Parallel()

	t.Run("Should Remain Cold Until Runtime Handle Exists", func(t *testing.T) {
		t.Parallel()

		manifest := &Manifest{
			Name: "linear",
			Resources: ResourcesConfig{
				Tools: map[string]ToolConfig{
					"lookup": {
						Description: "Search workspace",
						Backend:     ToolBackendConfig{Kind: "extension_host", Handler: "lookup"},
						InputSchema: json.RawMessage(`{
						"type": "object"
					}`),
						ReadOnly:   true,
						Visibility: "session",
					},
				},
			},
		}
		resolved, err := ResolveManifestToolResources(manifest)
		if err != nil {
			t.Fatalf("ResolveManifestToolResources() error = %v", err)
		}
		provider := coldManifestToolProvider{descriptor: resolved[0].Descriptor()}
		inputs := toolspkg.DefaultPolicyInputs()
		inputs.ExternalDefault = toolspkg.ExternalDefaultEnabled
		inputs.SystemPermissionMode = toolspkg.PermissionModeApproveAll
		registry, err := toolspkg.NewRegistry(
			toolspkg.WithProviders(provider),
			toolspkg.WithPolicyInputs(inputs, toolspkg.ToolsetCatalog{}),
		)
		if err != nil {
			t.Fatalf("toolspkg.NewRegistry() error = %v", err)
		}

		operatorViews, err := registry.List(testutil.Context(t), toolspkg.Scope{Operator: true})
		if err != nil {
			t.Fatalf("registry.List(operator) error = %v", err)
		}
		if got, want := len(operatorViews), 1; got != want {
			t.Fatalf("len(operatorViews) = %d, want %d", got, want)
		}
		if operatorViews[0].Availability.Executable {
			t.Fatal("operatorViews[0].Availability.Executable = true, want false")
		}
		if !slices.Contains(operatorViews[0].Availability.ReasonCodes, toolspkg.ReasonBackendNotExecutable) {
			t.Fatalf(
				"Availability.ReasonCodes = %#v, want backend_not_executable",
				operatorViews[0].Availability.ReasonCodes,
			)
		}

		sessionViews, err := registry.List(testutil.Context(t), toolspkg.Scope{})
		if err != nil {
			t.Fatalf("registry.List(session) error = %v", err)
		}
		if got := len(sessionViews); got != 0 {
			t.Fatalf("len(sessionViews) = %d, want 0", got)
		}
	})
}

func TestResolveManifestMCPServerResourcesResolvesTemplates(t *testing.T) {
	t.Parallel()

	t.Run("Should Resolve Templates", func(t *testing.T) {
		t.Parallel()

		rootDir := t.TempDir()
		manifest := &Manifest{
			Resources: ResourcesConfig{
				MCPServers: map[string]MCPServerConfig{
					"git": {
						Command: "./bin/mcp-git",
						Args:    []string{"--config", "{{config_dir}}/git.toml"},
						Env: map[string]string{
							"GIT_MODE": "{{env:GIT_MODE}}",
						},
						SecretEnv: map[string]string{
							"GIT_TOKEN": "env:GIT_TOKEN",
						},
					},
				},
			},
		}

		servers, err := ResolveManifestMCPServerResources(rootDir, manifest, func(key string) string {
			if key == "GIT_MODE" {
				return "readonly"
			}
			return ""
		})
		if err != nil {
			t.Fatalf("ResolveManifestMCPServerResources() error = %v", err)
		}
		if got, want := len(servers), 1; got != want {
			t.Fatalf("len(ResolveManifestMCPServerResources()) = %d, want %d", got, want)
		}
		if got, want := servers[0].Command, filepath.Join(rootDir, "bin", "mcp-git"); got != want {
			t.Fatalf("servers[0].Command = %q, want %q", got, want)
		}
		if got, want := servers[0].Args, []string{
			"--config",
			filepath.Join(rootDir, "git.toml"),
		}; !equalStrings(
			got,
			want,
		) {
			t.Fatalf("servers[0].Args = %#v, want %#v", got, want)
		}
		if got, want := servers[0].Env["GIT_MODE"], "readonly"; got != want {
			t.Fatalf("servers[0].Env[GIT_MODE] = %q, want %q", got, want)
		}
		if got, want := servers[0].SecretEnv["GIT_TOKEN"], "env:GIT_TOKEN"; got != want {
			t.Fatalf("servers[0].SecretEnv[GIT_TOKEN] = %q, want %q", got, want)
		}
	})
}

func mustCanonicalToolJSON(
	t *testing.T,
	codec resources.KindCodec[toolspkg.Tool],
	scope resources.ResourceScope,
	spec toolspkg.Tool,
) []byte {
	t.Helper()

	encoded, err := codec.Encode(spec)
	if err != nil {
		t.Fatalf("codec.Encode() error = %v", err)
	}
	validated, err := codec.DecodeAndValidate(testutil.Context(t), scope, encoded)
	if err != nil {
		t.Fatalf("codec.DecodeAndValidate() error = %v", err)
	}
	canonical, err := codec.Encode(validated)
	if err != nil {
		t.Fatalf("codec.Encode(validated) error = %v", err)
	}
	return canonical
}

func equalStrings(got []string, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for idx := range got {
		if got[idx] != want[idx] {
			return false
		}
	}
	return true
}

type coldManifestToolProvider struct {
	descriptor toolspkg.Descriptor
}

var _ toolspkg.Provider = (*coldManifestToolProvider)(nil)

func (p coldManifestToolProvider) ID() toolspkg.SourceRef {
	return p.descriptor.Source
}

func (p coldManifestToolProvider) List(_ context.Context, _ toolspkg.Scope) ([]toolspkg.Descriptor, error) {
	return []toolspkg.Descriptor{p.descriptor}, nil
}

func (p coldManifestToolProvider) Resolve(
	context.Context,
	toolspkg.Scope,
	toolspkg.ToolID,
) (toolspkg.Handle, bool, error) {
	return nil, false, nil
}
