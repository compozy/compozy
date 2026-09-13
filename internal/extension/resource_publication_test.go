package extensionpkg

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"path/filepath"
	"slices"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/marketplace"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/testutil"
	toolspkg "github.com/compozy/compozy/internal/tools"
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
		scope := resources.ResourceScope{Kind: resources.ResourceScopeKindUser}

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

	t.Run("Should Resolve MCP Backend Without Extension Host Handler", func(t *testing.T) {
		t.Parallel()

		manifest := &Manifest{
			Name: "catalog",
			Resources: ResourcesConfig{
				Tools: map[string]ToolConfig{
					"probe": {
						Description: "Inspect the catalog",
						Backend: ToolBackendConfig{
							Kind:   "mcp",
							Server: "catalog-server",
							Tool:   "catalog_probe",
						},
						InputSchema: json.RawMessage(`{"type":"object"}`),
						ReadOnly:    true,
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
		if got, want := descriptor.Tool.Backend.Kind, toolspkg.BackendMCP; got != want {
			t.Fatalf("Tool.Backend.Kind = %q, want %q", got, want)
		}
		if got, want := descriptor.Tool.Backend.MCPServer, "catalog-server"; got != want {
			t.Fatalf("Tool.Backend.MCPServer = %q, want %q", got, want)
		}
		if got, want := descriptor.Tool.Backend.MCPTool, "catalog_probe"; got != want {
			t.Fatalf("Tool.Backend.MCPTool = %q, want %q", got, want)
		}
		if descriptor.RuntimeDescriptor.Handler != "" {
			t.Fatalf("RuntimeDescriptor.Handler = %q, want empty for MCP backend", descriptor.RuntimeDescriptor.Handler)
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

	// Invariant: publication preserves manifest OAuth policy and its install-scope default without sharing mutable slices.
	// Owner: extension resource resolution. Canonical suite: resource_publication_test.go.
	t.Run("Should carry OAuth and default scope into resolved MCP resources", func(t *testing.T) {
		t.Parallel()
		manifest := &Manifest{Resources: ResourcesConfig{MCPServers: map[string]MCPServerConfig{
			"linear": {
				Transport:    "http",
				URL:          "https://mcp.linear.app/mcp",
				DefaultScope: "workspace",
				Auth: &MCPServerAuthConfig{
					Method:       "oauth",
					Registration: "dynamic",
					IssuerURL:    "https://mcp.linear.app",
					Scopes:       []string{"read", "write"},
				},
			},
		}}}
		servers, err := ResolveManifestMCPServerResources(t.TempDir(), manifest, InputState{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(servers) != 1 {
			t.Fatalf("servers = %#v", servers)
		}
		server := servers[0]
		if server.DefaultScope != "workspace" || server.Auth.Registration != compozyconfig.MCPAuthRegistrationAuto ||
			server.Auth.IssuerURL != "https://mcp.linear.app" ||
			!slices.Equal(server.Auth.Scopes, []string{"read", "write"}) {
			t.Fatalf("server = %#v", server)
		}
		server.Auth.Scopes[0] = "altered"
		if manifest.Resources.MCPServers["linear"].Auth.Scopes[0] != "read" {
			t.Fatal("publication shares auth scope storage")
		}
	})

	t.Run("Should Resolve Templates", func(t *testing.T) {
		t.Parallel()

		rootDir := t.TempDir()
		manifest := &Manifest{
			Resources: ResourcesConfig{
				MCPServers: map[string]MCPServerConfig{
					"git": {
						Command: "./bin/mcp-git",
						CWD:     rootDir,
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

		servers, err := ResolveManifestMCPServerResources(rootDir, manifest, InputState{}, func(key string) string {
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
		canonicalRoot, err := filepath.EvalSymlinks(rootDir)
		if err != nil {
			t.Fatalf("filepath.EvalSymlinks(rootDir) error = %v", err)
		}
		if got, want := servers[0].Command, filepath.Join(canonicalRoot, "bin", "mcp-git"); got != want {
			t.Fatalf("servers[0].Command = %q, want %q", got, want)
		}
		if got, want := servers[0].CWD, rootDir; got != want {
			t.Fatalf("servers[0].CWD = %q, want %q", got, want)
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

	t.Run("Should preserve portable remote transport metadata", func(t *testing.T) {
		t.Parallel()

		manifest := &Manifest{Resources: ResourcesConfig{MCPServers: map[string]MCPServerConfig{
			"remote": {
				Transport: string(compozyconfig.MCPServerTransportHTTP),
				URL:       "https://{{env:MCP_HOST}}/mcp",
				Headers:   map[string]string{"X-Tenant": "{{env:MCP_TENANT}}"},
			},
		}}}
		servers, err := ResolveManifestMCPServerResources(t.TempDir(), manifest, InputState{}, func(key string) string {
			switch key {
			case "MCP_HOST":
				return "example.com"
			case "MCP_TENANT":
				return "acme"
			default:
				return ""
			}
		})
		if err != nil {
			t.Fatalf("ResolveManifestMCPServerResources() error = %v", err)
		}
		if got, want := len(servers), 1; got != want {
			t.Fatalf("len(ResolveManifestMCPServerResources()) = %d, want %d", got, want)
		}
		if got, want := servers[0].Transport, compozyconfig.MCPServerTransportHTTP; got != want {
			t.Fatalf("servers[0].Transport = %q, want %q", got, want)
		}
		if got, want := servers[0].URL, "https://example.com/mcp"; got != want {
			t.Fatalf("servers[0].URL = %q, want %q", got, want)
		}
		if got, want := servers[0].Headers["X-Tenant"], "acme"; got != want {
			t.Fatalf("servers[0].Headers[X-Tenant] = %q, want %q", got, want)
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

// Invariant: instance inputs bind to every declaring server, stay literal, and absent optional bindings disappear.
// Owner: extension MCP publication. Canonical suite: resource_publication_test.go.
func TestResolveManifestMCPServerInputs(t *testing.T) {
	t.Parallel()
	t.Run("Should apply scoped multi-server bindings without interpreting user templates [UT-057]", func(t *testing.T) {
		t.Parallel()
		manifest := &Manifest{Inputs: []ManifestInput{
			{
				ID:       "mode",
				Prompt:   "Mode",
				Type:     "string",
				Required: true,
				Binding:  marketplace.InputBinding{Type: "env", Name: "MODE"},
			},
			{
				ID:      "token",
				Prompt:  "Token",
				Type:    "secret",
				Binding: marketplace.InputBinding{Type: "env", Name: "TOKEN"},
			},
			{
				ID:       "workspace",
				Prompt:   "Workspace",
				Type:     "string",
				Required: true,
				Binding:  marketplace.InputBinding{Type: "url_query", Name: "ws"},
			},
			{
				ID:      "optional",
				Prompt:  "Optional",
				Type:    "string",
				Binding: marketplace.InputBinding{Type: "url_query", Name: "optional"},
			},
		}, Resources: ResourcesConfig{MCPServers: map[string]MCPServerConfig{
			"a": {
				Command:   "server-a",
				Env:       map[string]string{"MODE": "mode", "FIXED": "keep"},
				SecretEnv: map[string]string{"TOKEN": "token"},
			},
			"b": {Command: "server-b", Env: map[string]string{"MODE": "mode"}},
			"c": {Transport: "http", URL: "https://example.com/mcp?ws=&optional=placeholder&fixed=one&fixed=two"},
			"d": {Transport: "http", URL: "https://other.example.com/mcp?ws="},
			"e": {Transport: "http", URL: "https://other.example.com/mcp?fixed=unchanged"},
		}}}
		literal := "{{env:PRIVATE}} & + / ? #"
		raw, err := json.Marshal(literal)
		if err != nil {
			t.Fatal(err)
		}
		state := InputState{Values: map[string]InputValueRecord{
			"mode":      {Type: "string", Value: raw, Active: true},
			"workspace": {Type: "string", Value: raw, Active: true},
		}}
		servers, err := ResolveManifestMCPServerResources(
			t.TempDir(),
			manifest,
			state,
			func(string) string { return "" },
		)
		if err != nil {
			t.Fatal(err)
		}
		if len(servers) != 5 {
			t.Fatalf("server count = %d", len(servers))
		}
		for _, server := range servers {
			switch server.Name {
			case "a", "b":
				if server.Env["MODE"] != literal {
					t.Fatalf("%s mode was interpreted: %q", server.Name, server.Env["MODE"])
				}
				if _, exists := server.SecretEnv["TOKEN"]; exists {
					t.Fatal("absent optional secret remained bound")
				}
			case "c", "d":
				parsed, err := url.Parse(server.URL)
				if err != nil {
					t.Fatal(err)
				}
				query := parsed.Query()
				if query.Get("ws") != literal || len(query["ws"]) != 1 || query.Has("optional") {
					t.Fatalf("%s query = %#v", server.Name, query)
				}
				if server.Name == "c" && !slices.Equal(query["fixed"], []string{"one", "two"}) {
					t.Fatalf("fixed query changed: %v", query)
				}
			case "e":
				if server.URL != manifest.Resources.MCPServers["e"].URL {
					t.Fatal("input applied to a non-declaring server")
				}
			}
		}
		if manifest.Resources.MCPServers["a"].Env["MODE"] != "mode" ||
			manifest.Resources.MCPServers["a"].SecretEnv["TOKEN"] != "token" {
			t.Fatal("publication mutated its manifest")
		}
	})
	t.Run("Should retain secret references and never publish process secret plaintext [UT-021]", func(t *testing.T) {
		t.Parallel()
		// Invariant: runtime declarations retain the owner and secret references, without secret plaintext.
		// Owner: manifest publication; canonical suite: resource_publication_test.go.
		manifest := &Manifest{
			Name: "kit",
			Inputs: []ManifestInput{
				{
					ID:       "token",
					Prompt:   "Token",
					Type:     "secret",
					Required: true,
					Binding:  marketplace.InputBinding{Type: "env", Name: "TOKEN"},
				},
			},
			Resources: ResourcesConfig{MCPServers: map[string]MCPServerConfig{
				"server": {Command: "server", SecretEnv: map[string]string{"TOKEN": "token"}},
			}},
		}
		for _, state := range []InputState{{},
			{Values: map[string]InputValueRecord{"token": {Type: "secret", SecretRef: "vault:mcp/server/TOKEN", Active: true}}},
			{Values: map[string]InputValueRecord{"token": {Type: "secret", SecretRef: "vault:extensions/global/kit/profiles/default/env/TOKEN", Active: true}}},
		} {
			servers, err := ResolveManifestMCPServerResources(
				t.TempDir(),
				manifest,
				state,
				func(string) string { return "private-secret-value" },
			)
			if err != nil {
				t.Fatal(err)
			}
			want := "env:TOKEN"
			if len(state.Values) > 0 {
				want = state.Values["token"].SecretRef
			}
			if servers[0].SecretEnv["TOKEN"] != want || len(servers[0].Env) != 0 ||
				servers[0].Owner != "extension:kit" {
				t.Fatal("secret was not published by reference")
			}
			encoded, err := json.Marshal(servers)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(encoded, []byte("private-secret-value")) {
				t.Fatal("publication contains secret plaintext")
			}
		}
	})
}
