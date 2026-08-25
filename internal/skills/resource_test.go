package skills

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestSkillResourceCodecRejectsInvalidSpecs(t *testing.T) {
	t.Parallel()

	codec, err := NewResourceCodec()
	if err != nil {
		t.Fatalf("NewResourceCodec() error = %v", err)
	}
	scope := resources.ResourceScope{Kind: resources.ResourceScopeKindUser}

	tests := []struct {
		name    string
		spec    SkillResourceSpec
		wantErr string
	}{
		{
			name: "missing name",
			spec: SkillResourceSpec{
				Description: "desc",
				Source:      "user",
				Enabled:     true,
			},
			wantErr: "skill.name is required",
		},
		{
			name: "missing description",
			spec: SkillResourceSpec{
				Name:    "review",
				Source:  "user",
				Enabled: true,
			},
			wantErr: "skill.description is required",
		},
		{
			name: "invalid source",
			spec: SkillResourceSpec{
				Name:        "review",
				Description: "desc",
				Source:      "elsewhere",
				Enabled:     true,
			},
			wantErr: "unsupported skill source",
		},
		{
			name: "invalid mcp",
			spec: SkillResourceSpec{
				Name:        "review",
				Description: "desc",
				Source:      "user",
				Enabled:     true,
				MCPServers: []MCPServerDecl{{
					Name: "github",
				}},
			},
			wantErr: "skill.mcp_servers[0].command",
		},
		{
			name: "blank activation gate value",
			spec: SkillResourceSpec{
				Name:        "review",
				Description: "desc",
				Source:      "user",
				Enabled:     true,
				ActivationGates: ActivationGates{
					RequiresTools: []string{"compozy__skill_view", "   "},
				},
			},
			wantErr: "metadata.compozy.when.requires_tools[1] is required",
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
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("DecodeAndValidate() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestSkillResourceCodecRejectsSecretLikeLiteralMCPEnv(t *testing.T) {
	t.Parallel()

	t.Run("Should reject secret-like MCP env values at resource validation time", func(t *testing.T) {
		t.Parallel()

		codec, err := NewResourceCodec()
		if err != nil {
			t.Fatalf("NewResourceCodec() error = %v", err)
		}
		raw, err := codec.Encode(SkillResourceSpec{
			Name:        "review",
			Description: "desc",
			Source:      "user",
			Enabled:     true,
			MCPServers: []MCPServerDecl{{
				Name:    "github",
				Command: "npx",
				Env:     map[string]string{"GITHUB_TOKEN": "ghp-secret"},
			}},
		})
		if err != nil {
			t.Fatalf("Encode() error = %v", err)
		}
		_, err = codec.DecodeAndValidate(
			context.Background(),
			resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
			raw,
		)
		if err == nil {
			t.Fatal("DecodeAndValidate() error = nil, want secret-like env validation error")
		}
		if !strings.Contains(err.Error(), "must move secret-like values to secret_env") {
			t.Fatalf("DecodeAndValidate() error = %v, want secret_env validation", err)
		}
	})
}

func TestSkillResourceCodecPreservesProvenanceAndSidecarMCP(t *testing.T) {
	t.Parallel()

	skillDir := filepath.Join(t.TempDir(), "market-skill")
	skillPath := writeSkillFile(t, skillDir, skillFileName, strings.Join([]string{
		"---",
		"name: market-skill",
		"description: Installed marketplace skill",
		"version: 1.2.3",
		"metadata:",
		"  compozy:",
		"    when:",
		"      platforms: [DARWIN, linux, darwin]",
		"      requires_tools: [compozy__skill_view]",
		"---",
		"Use this skill.",
	}, "\n"))
	writeTestFile(t, filepath.Join(skillDir, "mcp.json"), `{
  "mcpServers": {
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github"],
	      "secret_env": {"GITHUB_TOKEN": "env:GITHUB_TOKEN"}
    }
  }
}`)
	if err := WriteSidecar(skillDir, Provenance{
		Hash:        skillDirectoryHash(t, skillDir),
		Registry:    "clawhub",
		Slug:        "@author/market-skill",
		Version:     "1.2.3",
		InstalledAt: time.Date(2026, 4, 16, 1, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("WriteSidecar() error = %v", err)
	}

	registry := NewRegistry(RegistryConfig{GlobalSkillRoots: testGlobalSkillRoots(filepath.Dir(skillDir))})
	discovered, _, err := registry.DiscoverGlobal(context.Background())
	if err != nil {
		t.Fatalf("DiscoverGlobal() error = %v", err)
	}
	if got, want := len(discovered), 1; got != want {
		t.Fatalf("len(DiscoverGlobal()) = %d, want %d", got, want)
	}
	if discovered[0].FilePath != skillPath {
		t.Fatalf("FilePath = %q, want %q", discovered[0].FilePath, skillPath)
	}

	codec, err := NewResourceCodec()
	if err != nil {
		t.Fatalf("NewResourceCodec() error = %v", err)
	}
	raw, err := codec.Encode(SkillToResourceSpec(discovered[0]))
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	decoded, err := codec.DecodeAndValidate(
		context.Background(),
		resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
		raw,
	)
	if err != nil {
		t.Fatalf("DecodeAndValidate() error = %v", err)
	}
	projected, err := SkillFromResourceSpec(decoded)
	if err != nil {
		t.Fatalf("SkillFromResourceSpec() error = %v", err)
	}
	if projected.Provenance == nil || projected.Provenance.Slug != "@author/market-skill" {
		t.Fatalf("Provenance = %#v, want marketplace sidecar provenance", projected.Provenance)
	}
	if got, want := projected.Source, SourceMarketplace; got != want {
		t.Fatalf("Source = %v, want %v", got, want)
	}
	if got, want := len(projected.MCPServers), 1; got != want {
		t.Fatalf("len(MCPServers) = %d, want %d", got, want)
	}
	if got, want := projected.MCPServers[0].Command, "npx"; got != want {
		t.Fatalf("MCP command = %q, want %q", got, want)
	}
	if got, want := projected.MCPServers[0].SecretEnv["GITHUB_TOKEN"], "env:GITHUB_TOKEN"; got != want {
		t.Fatalf("MCP secret env = %q, want %q", got, want)
	}
	if got, want := projected.ActivationGates.Platforms, []string{"darwin", "linux"}; !slices.Equal(got, want) {
		t.Fatalf("ActivationGates.Platforms = %#v, want %#v", got, want)
	}
	if got, want := projected.ActivationGates.RequiresTools, []string{"compozy__skill_view"}; !slices.Equal(got, want) {
		t.Fatalf("ActivationGates.RequiresTools = %#v, want %#v", got, want)
	}
}

func TestDiscoverGlobalPreservesPhysicalHomonymsForPublication(t *testing.T) {
	t.Parallel()
	t.Run("Should publish both physical homonyms with distinct root identities", func(t *testing.T) {
		t.Parallel()

		agentsRoot := filepath.Join(t.TempDir(), "agents")
		claudeRoot := filepath.Join(t.TempDir(), "claude")
		writeSkillFile(
			t,
			filepath.Join(agentsRoot, "review"),
			skillFileName,
			skillWithBody("review", "Agents review", "Agents body."),
		)
		writeSkillFile(
			t,
			filepath.Join(claudeRoot, "review"),
			skillFileName,
			skillWithBody("review", "Claude review", "Claude body."),
		)
		scope := resources.ResourceScope{Kind: resources.ResourceScopeKindUser}
		registry := NewRegistry(RegistryConfig{GlobalSkillRoots: []compozyconfig.SkillRootSpec{
			{
				Dir:           agentsRoot,
				SourceSlug:    compozyconfig.SkillSourceAgents,
				Kind:          compozyconfig.RootKindPreset,
				ResourceScope: scope,
			},
			{
				Dir:           claudeRoot,
				SourceSlug:    compozyconfig.SkillSourceClaude,
				Kind:          compozyconfig.RootKindPreset,
				ResourceScope: scope,
			},
		}})
		discovered, _, err := registry.DiscoverGlobal(t.Context())
		if err != nil {
			t.Fatalf("DiscoverGlobal() error = %v", err)
		}
		if len(discovered) != 2 {
			t.Fatalf("DiscoverGlobal() = %#v, want both physical homonyms", discovered)
		}
		rootIDs := []string{discovered[0].RootID, discovered[1].RootID}
		if rootIDs[0] == "" || rootIDs[1] == "" || rootIDs[0] == rootIDs[1] {
			t.Fatalf("DiscoverGlobal() RootIDs = %#v, want two non-empty identities", rootIDs)
		}
	})
}

func TestParseSkillFileWithSourceMergesMCPSidecar(t *testing.T) {
	t.Parallel()

	skillDir := filepath.Join(t.TempDir(), "extension-skill")
	skillPath := writeSkillFile(t, skillDir, skillFileName, strings.Join([]string{
		"---",
		"name: extension-skill",
		"description: Extension skill",
		"---",
		"Use this skill.",
	}, "\n"))
	writeTestFile(t, filepath.Join(skillDir, "mcp.json"), `{
  "mcpServers": {
    "extension-mcp": {
      "command": "extension-command"
    }
  }
}`)

	skill, err := ParseSkillFileWithSource(skillPath, SourceWorkspace)
	if err != nil {
		t.Fatalf("ParseSkillFileWithSource() error = %v", err)
	}
	if got, want := skill.Source, SourceWorkspace; got != want {
		t.Fatalf("Source = %v, want %v", got, want)
	}
	if got, want := len(skill.MCPServers), 1; got != want {
		t.Fatalf("len(MCPServers) = %d, want %d", got, want)
	}
	if got, want := skill.MCPServers[0].Name, "extension-mcp"; got != want {
		t.Fatalf("MCPServers[0].Name = %q, want %q", got, want)
	}
}

func TestSkillResourceCodecCanonicalizesHookMetadata(t *testing.T) {
	t.Parallel()

	toolReadOnly := true
	codec, err := NewResourceCodec()
	if err != nil {
		t.Fatalf("NewResourceCodec() error = %v", err)
	}
	raw, err := codec.Encode(SkillResourceSpec{
		Name:        "hooked-skill",
		Description: "Skill with hooks",
		Source:      skillSourceName(SourceWorkspace),
		Enabled:     true,
		Hooks: []hookspkg.HookDecl{{
			Name:    "hooked",
			Event:   hookspkg.HookToolPreCall,
			Source:  hookspkg.HookSourceSkill,
			Mode:    hookspkg.HookModeSync,
			Command: "echo",
			Args:    []string{"ok"},
			Env:     map[string]string{"ONE": "1"},
			Matcher: hookspkg.HookMatcher{ToolReadOnly: &toolReadOnly},
			Metadata: map[string]string{
				"skill": "hooked-skill",
			},
		}},
	})
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	decoded, err := codec.DecodeAndValidate(
		context.Background(),
		resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "ws-hooks"},
		raw,
	)
	if err != nil {
		t.Fatalf("DecodeAndValidate() error = %v", err)
	}
	toolReadOnly = false
	if got, want := len(decoded.Hooks), 1; got != want {
		t.Fatalf("len(Hooks) = %d, want %d", got, want)
	}
	hook := decoded.Hooks[0]
	if hook.Matcher.ToolReadOnly == nil || !*hook.Matcher.ToolReadOnly {
		t.Fatalf("Hook matcher ToolReadOnly = %#v, want cloned true pointer", hook.Matcher.ToolReadOnly)
	}
	if hook.Args[0] != "ok" || hook.Env["ONE"] != "1" || hook.Metadata["skill"] != "hooked-skill" {
		t.Fatalf("Hook clone = %#v, want args/env/metadata preserved", hook)
	}
}

func TestResourceAuthorityKeepsFilesystemDiscoveryNonAuthoritative(t *testing.T) {
	t.Parallel()

	userDir := t.TempDir()
	writeSkillFile(t, userDir, filepath.Join("legacy-skill", skillFileName), strings.Join([]string{
		"---",
		"name: legacy-skill",
		"description: Filesystem skill",
		"---",
		"Loaded only before resource authority exists.",
	}, "\n"))

	registry := NewRegistry(RegistryConfig{GlobalSkillRoots: testGlobalSkillRoots(userDir)})
	records := []resources.Record[SkillResourceSpec]{{
		Kind: SkillResourceKind,
		ID:   "global:resource-backed",
		Scope: resources.ResourceScope{
			Kind: resources.ResourceScopeKindUser,
		},
		Spec: SkillResourceSpec{
			Name:        "resource-backed",
			Description: "Canonical resource skill",
			Source:      skillSourceName(SourceUser),
			Enabled:     true,
		},
	}}
	var nilCtx context.Context
	if err := registry.ApplyResourceRecords(nilCtx, 1, records); err == nil {
		t.Fatal("ApplyResourceRecords(nil) error = nil, want context validation failure")
	}
	if _, ok := registry.Get("resource-backed"); ok {
		t.Fatal("Get(\"resource-backed\") ok = true after rejected projection")
	}
	if err := registry.ApplyResourceRecords(context.Background(), 1, records); err != nil {
		t.Fatalf("ApplyResourceRecords() error = %v", err)
	}
	if err := registry.LoadAll(context.Background()); err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}

	if _, ok := registry.Get("legacy-skill"); ok {
		t.Fatal("Get(\"legacy-skill\") ok = true, want filesystem discovery non-authoritative after resource cutover")
	}
	if _, ok := registry.Get("resource-backed"); !ok {
		t.Fatal("Get(\"resource-backed\") ok = false, want canonical resource skill")
	}
}

func TestResourceAuthorityProjectsWorkspaceSkills(t *testing.T) {
	t.Parallel()

	registry := NewRegistry(RegistryConfig{})
	records := []resources.Record[SkillResourceSpec]{
		{
			Kind: SkillResourceKind,
			ID:   "global:global-skill",
			Scope: resources.ResourceScope{
				Kind: resources.ResourceScopeKindUser,
			},
			Spec: SkillResourceSpec{
				Name:        "global-skill",
				Description: "Global resource skill",
				Source:      skillSourceName(SourceUser),
				Enabled:     true,
			},
		},
		{
			Kind: SkillResourceKind,
			ID:   "workspace:workspace-skill",
			Scope: resources.ResourceScope{
				Kind: resources.ResourceScopeKindWorkspace,
				ID:   "/workspace/project",
			},
			Spec: SkillResourceSpec{
				Name:        "workspace-skill",
				Description: "Workspace resource skill",
				Source:      skillSourceName(SourceWorkspace),
				Enabled:     true,
			},
		},
		{
			Kind: SkillResourceKind,
			ID:   "profile:profile-skill",
			Scope: resources.ResourceScope{
				Kind: resources.ResourceScopeKindProfile,
				ID:   store.DefaultProfileID,
			},
			Spec: SkillResourceSpec{
				Name: "profile-skill", Description: "Profile resource skill",
				Source: skillSourceName(SourceProfile), Enabled: true,
			},
		},
		{
			Kind: SkillResourceKind,
			ID:   "workspace-profile:workspace-profile-skill",
			Scope: resources.ResourceScope{
				Kind: resources.ResourceScopeKindWorkspaceProfile,
				ID:   "/workspace/project@pf:default",
			},
			Spec: SkillResourceSpec{
				Name: "workspace-profile-skill", Description: "Workspace-profile resource skill",
				Source: skillSourceName(SourceWorkspaceProfile), Enabled: true,
			},
		},
	}
	if err := registry.ApplyResourceRecords(context.Background(), 2, records); err != nil {
		t.Fatalf("ApplyResourceRecords() error = %v", err)
	}

	skills, err := registry.ForWorkspace(context.Background(), &workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{ID: "/workspace/project"},
		ProfileID: store.DefaultProfileID, ProfileName: "default",
	})
	if err != nil {
		t.Fatalf("ForWorkspace() error = %v", err)
	}
	if !hasSkillNamed(skills, "global-skill") {
		t.Fatal("ForWorkspace() missing global-skill")
	}
	if !hasSkillNamed(skills, "workspace-skill") {
		t.Fatal("ForWorkspace() missing workspace-skill")
	}
	if !hasSkillNamed(skills, "profile-skill") {
		t.Fatal("ForWorkspace() missing profile-skill")
	}
	if !hasSkillNamed(skills, "workspace-profile-skill") {
		t.Fatal("ForWorkspace() missing workspace-profile-skill")
	}
	defaultSkills, err := registry.ForWorkspace(context.Background(), &workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{ID: "/workspace/project"},
	})
	if err != nil {
		t.Fatalf("ForWorkspace(default profile) error = %v", err)
	}
	if !hasSkillNamed(defaultSkills, "profile-skill") ||
		!hasSkillNamed(defaultSkills, "workspace-profile-skill") {
		t.Fatalf("ForWorkspace(default profile) skills = %#v, want profile layers", defaultSkills)
	}

	other, err := registry.ForWorkspace(context.Background(), &workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{ID: "/workspace/other"},
		ProfileID: store.DefaultProfileID, ProfileName: "default",
	})
	if err != nil {
		t.Fatalf("ForWorkspace(other) error = %v", err)
	}
	if hasSkillNamed(other, "workspace-skill") {
		t.Fatal("ForWorkspace(other) includes workspace-skill, want workspace scope isolation")
	}
	if hasSkillNamed(other, "workspace-profile-skill") {
		t.Fatal("ForWorkspace(other) includes workspace-profile-skill, want combined scope isolation")
	}

	if err := registry.SetEnabled(
		"workspace-skill",
		&workspacepkg.ResolvedWorkspace{Workspace: workspacepkg.Workspace{ID: "/workspace/project"}},
		false,
	); err != nil {
		t.Fatalf("SetEnabled(workspace-skill) error = %v", err)
	}
	updated, err := registry.ForWorkspace(context.Background(), &workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{ID: "/workspace/project"},
	})
	if err != nil {
		t.Fatalf("ForWorkspace(updated) error = %v", err)
	}
	workspaceSkill := findSkillByName(updated, "workspace-skill")
	if workspaceSkill == nil || workspaceSkill.Enabled {
		t.Fatalf("workspace-skill after SetEnabled = %#v, want disabled resource projection", workspaceSkill)
	}
}

func TestDiscoverWorkspaceLoadsDefinitionsForPublication(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	skillDir := filepath.Join(root, ".compozy", "skills", "workspace-review")
	writeSkillFile(t, skillDir, skillFileName, strings.Join([]string{
		"---",
		"name: workspace-review",
		"description: Workspace review",
		"---",
		"Review workspace changes.",
	}, "\n"))

	registry := NewRegistry(RegistryConfig{})
	discovered, snapshots, err := registry.DiscoverWorkspace(
		context.Background(),
		&workspacepkg.ResolvedWorkspace{
			Workspace:   workspacepkg.Workspace{ID: "ws-discover", RootDir: root},
			WorkspaceID: "ws-discover",
		},
	)
	if err != nil {
		t.Fatalf("DiscoverWorkspace() error = %v", err)
	}
	if got, want := len(discovered), 1; got != want {
		t.Fatalf("len(DiscoverWorkspace()) = %d, want %d", got, want)
	}
	if discovered[0].Meta.Name != "workspace-review" || discovered[0].Source != SourceWorkspace {
		t.Fatalf("DiscoverWorkspace()[0] = %#v, want workspace-review from workspace source", discovered[0])
	}
	if len(snapshots) == 0 {
		t.Fatal("DiscoverWorkspace() snapshots = empty, want publication change tracking snapshots")
	}
}

func findSkillByName(skills []*Skill, name string) *Skill {
	for _, skill := range skills {
		if skill != nil && skill.Meta.Name == name {
			return skill
		}
	}
	return nil
}

func hasSkillNamed(skills []*Skill, name string) bool {
	for _, skill := range skills {
		if skill != nil && skill.Meta.Name == name {
			return true
		}
	}
	return false
}

func skillDirectoryHash(t *testing.T, skillDir string) string {
	t.Helper()

	hash, err := ComputeDirectoryHash(skillDir)
	if err != nil {
		t.Fatalf("ComputeDirectoryHash(%q) error = %v", skillDir, err)
	}
	return hash
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
