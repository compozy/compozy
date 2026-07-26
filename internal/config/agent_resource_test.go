package config

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/resources"
)

func TestAgentResourceCodecRejectsInvalidSpecs(t *testing.T) {
	t.Parallel()

	codec, err := NewAgentResourceCodec()
	if err != nil {
		t.Fatalf("NewAgentResourceCodec() error = %v", err)
	}
	scope := resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal}

	tests := []struct {
		name    string
		spec    AgentDef
		wantErr string
	}{
		{
			name: "ShouldRejectMissingName",
			spec: AgentDef{
				Prompt: "You are helpful.",
			},
			wantErr: "agent name is required",
		},
		{
			name: "ShouldRejectMissingPrompt",
			spec: AgentDef{
				Name: "coder",
			},
			wantErr: "agent prompt is required",
		},
		{
			name: "ShouldRejectInvalidPermissions",
			spec: AgentDef{
				Name:        "coder",
				Prompt:      "You are helpful.",
				Permissions: "invalid",
			},
			wantErr: "agent.permissions",
		},
		{
			name: "ShouldRejectInvalidMCPServer",
			spec: AgentDef{
				Name:   "coder",
				Prompt: "You are helpful.",
				MCPServers: []MCPServer{{
					Name: "github",
				}},
			},
			wantErr: "agent.mcp_servers[0]",
		},
		{
			name: "ShouldRejectInvalidToolPattern",
			spec: AgentDef{
				Name:   "coder",
				Prompt: "You are helpful.",
				Tools:  []string{"github.search"},
			},
			wantErr: "agent.tools[0]",
		},
		{
			name: "ShouldRejectInvalidToolsetID",
			spec: AgentDef{
				Name:     "coder",
				Prompt:   "You are helpful.",
				Toolsets: []string{"core"},
			},
			wantErr: "agent.toolsets[0]",
		},
		{
			name: "ShouldRejectInvalidCapabilityCatalog",
			spec: AgentDef{
				Name:   "coder",
				Prompt: "You are helpful.",
				Capabilities: &CapabilityCatalog{
					Capabilities: []CapabilityDef{{
						ID: "build-site",
					}},
				},
			},
			wantErr: "agent.capabilities",
		},
		{
			name: "ShouldRejectInvalidCategoryPath",
			spec: AgentDef{
				Name:         "coder",
				Prompt:       "You are helpful.",
				CategoryPath: []string{"Marketing/Sales"},
			},
			wantErr: "agent.category_path[0]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			raw, err := codec.Encode(tt.spec)
			if err != nil {
				t.Fatalf("Encode() error = %v", err)
			}
			_, err = codec.DecodeAndValidate(context.Background(), scope, raw)
			if err == nil {
				t.Fatal("DecodeAndValidate() error = nil, want validation error")
			}
			if !errors.Is(err, resources.ErrValidation) {
				t.Fatalf("DecodeAndValidate() error = %v, want resources.ErrValidation", err)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("DecodeAndValidate() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestAgentResourceCodecCanonicalizesTypedRecordSpec(t *testing.T) {
	t.Parallel()

	t.Run("Should canonicalize typed record spec", func(t *testing.T) {
		t.Parallel()

		codec, err := NewAgentResourceCodec()
		if err != nil {
			t.Fatalf("NewAgentResourceCodec() error = %v", err)
		}
		stdioServer := MCPServer{
			Name:      " github ",
			Transport: " stdio ",
			Command:   " npx ",
			Args:      []string{" -y ", " @modelcontextprotocol/server-github "},
			Env: map[string]string{
				" NODE_ENV ": " production ",
			},
			SecretEnv: map[string]string{
				" GITHUB_TOKEN ": " env:GITHUB_TOKEN ",
			},
		}
		remoteServer := MCPServer{
			Name:      " linear ",
			Transport: " sse ",
			URL:       " https://mcp.example/sse ",
			Auth: MCPAuthConfig{
				Type:             " oauth2_pkce ",
				AuthorizationURL: " https://auth.example/authorize ",
				TokenURL:         " https://auth.example/token ",
				ClientID:         " client-id ",
				Scopes:           []string{" read ", " write "},
			},
		}
		raw, err := codec.Encode(AgentDef{
			Name:   " coder ",
			Prompt: " Build things. ",
			Tools:  []string{" mcp__github__search ", "mcp__github__search", " agh__skill_* "},
			Toolsets: []string{
				" agh__catalog ",
				"agh__catalog",
			},
			DenyTools: []string{
				" agh__task_* ",
				"agh__task_*",
			},
			CategoryPath: []string{" Marketing ", " Sales "},
			Capabilities: &CapabilityCatalog{
				Capabilities: []CapabilityDef{{
					ID:                " build-site ",
					Summary:           " Build the landing page. ",
					Outcome:           " A finished landing page. ",
					ContextNeeded:     []string{" repo ", "", " brand brief "},
					ExecutionOutline:  []string{" inspect ", " build "},
					ArtifactsExpected: []string{" final page "},
				}},
			},
			MCPServers: []MCPServer{stdioServer, remoteServer},
		})
		if err != nil {
			t.Fatalf("Encode() error = %v", err)
		}

		got, err := codec.DecodeAndValidate(
			context.Background(),
			resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "ws_1"},
			raw,
		)
		if err != nil {
			t.Fatalf("DecodeAndValidate() error = %v", err)
		}
		if got.Name != "coder" || got.Prompt != "Build things." {
			t.Fatalf("decoded agent = %#v, want trimmed name and prompt", got)
		}
		if want := []string{
			"mcp__github__search",
			"agh__skill_*",
		}; strings.Join(
			got.Tools,
			",",
		) != strings.Join(
			want,
			",",
		) {
			t.Fatalf("Tools = %#v, want %#v", got.Tools, want)
		}
		if want := []string{"agh__catalog"}; strings.Join(got.Toolsets, ",") != strings.Join(want, ",") {
			t.Fatalf("Toolsets = %#v, want %#v", got.Toolsets, want)
		}
		if want := []string{"agh__task_*"}; strings.Join(got.DenyTools, ",") != strings.Join(want, ",") {
			t.Fatalf("DenyTools = %#v, want %#v", got.DenyTools, want)
		}
		if want := []string{"Marketing", "Sales"}; strings.Join(got.CategoryPath, ",") != strings.Join(want, ",") {
			t.Fatalf("CategoryPath = %#v, want %#v", got.CategoryPath, want)
		}
		if gotCount, wantCount := len(got.MCPServers), 2; gotCount != wantCount {
			t.Fatalf("len(MCPServers) = %d, want %d", gotCount, wantCount)
		}
		for idx, server := range []MCPServer{stdioServer, remoteServer} {
			want := canonicalMCPServerResourceSpecForAgentTest(t, server)
			if !reflect.DeepEqual(got.MCPServers[idx], want) {
				t.Fatalf("MCPServers[%d] = %#v, want standalone canonical spec %#v", idx, got.MCPServers[idx], want)
			}
		}
		if got.Capabilities == nil || len(got.Capabilities.Capabilities) != 1 {
			t.Fatalf("Capabilities = %#v, want one normalized capability", got.Capabilities)
		}
		if got.Capabilities.Capabilities[0].ID != "build-site" {
			t.Fatalf("Capabilities[0].ID = %q, want build-site", got.Capabilities.Capabilities[0].ID)
		}
		if want := []string{
			"repo",
			"brand brief",
		}; strings.Join(
			got.Capabilities.Capabilities[0].ContextNeeded,
			",",
		) != strings.Join(
			want,
			",",
		) {
			t.Fatalf("ContextNeeded = %#v, want %#v", got.Capabilities.Capabilities[0].ContextNeeded, want)
		}
	})

	t.Run("Should decode raw JSON snake case resource fields", func(t *testing.T) {
		t.Parallel()

		codec, err := NewAgentResourceCodec()
		if err != nil {
			t.Fatalf("NewAgentResourceCodec() error = %v", err)
		}

		raw := []byte(`{
			"name": " coder ",
			"provider": " openai ",
			"prompt": " Build things. ",
			"tools": [" mcp__github__search "],
			"toolsets": [" agh__catalog "],
			"deny_tools": [" agh__task_* ", "agh__task_*"],
			"permissions": " approve-reads ",
			"skills": {
				"disabled": [" legacy-skill ", "legacy-skill"]
			},
			"category_path": [" Engineering ", " Backend "],
			"mcp_servers": [
				{
					"name": " github ",
					"transport": "stdio",
					"command": " npx ",
					"args": ["-y", "@modelcontextprotocol/server-github"]
				}
			]
		}`)

		got, err := codec.DecodeAndValidate(
			context.Background(),
			resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "ws_1"},
			raw,
		)
		if err != nil {
			t.Fatalf("DecodeAndValidate() error = %v", err)
		}
		if got.Name != "coder" || got.Provider != "openai" || got.Prompt != "Build things." {
			t.Fatalf("decoded agent = %#v, want trimmed scalar fields", got)
		}
		if want := []string{"agh__task_*"}; strings.Join(got.DenyTools, ",") != strings.Join(want, ",") {
			t.Fatalf("DenyTools = %#v, want %#v", got.DenyTools, want)
		}
		if want := []string{"legacy-skill"}; strings.Join(got.Skills.Disabled, ",") != strings.Join(want, ",") {
			t.Fatalf("Skills.Disabled = %#v, want %#v", got.Skills.Disabled, want)
		}
		if want := []string{"Engineering", "Backend"}; strings.Join(got.CategoryPath, ",") != strings.Join(want, ",") {
			t.Fatalf("CategoryPath = %#v, want %#v", got.CategoryPath, want)
		}
		if gotCount, wantCount := len(got.MCPServers), 1; gotCount != wantCount {
			t.Fatalf("len(MCPServers) = %d, want %d", gotCount, wantCount)
		}
		if got.MCPServers[0].Name != "github" || got.MCPServers[0].Command != "npx" {
			t.Fatalf("MCPServers = %#v, want trimmed name/command", got.MCPServers)
		}
	})
}

func canonicalMCPServerResourceSpecForAgentTest(t *testing.T, spec MCPServer) MCPServer {
	t.Helper()

	codec, err := NewMCPServerResourceCodec()
	if err != nil {
		t.Fatalf("NewMCPServerResourceCodec() error = %v", err)
	}
	raw, err := codec.Encode(spec)
	if err != nil {
		t.Fatalf("MCP codec Encode() error = %v", err)
	}
	got, err := codec.DecodeAndValidate(
		context.Background(),
		resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "ws_1"},
		raw,
	)
	if err != nil {
		t.Fatalf("MCP codec DecodeAndValidate() error = %v", err)
	}
	return got
}
