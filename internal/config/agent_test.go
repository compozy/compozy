package config

import (
	"bytes"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/frontmatter"
	"github.com/compozy/compozy/internal/reasoning"
	"github.com/compozy/compozy/internal/resources"
	"github.com/goccy/go-yaml"
)

func TestAgentDefinitionLifecycleHelpers(t *testing.T) {
	t.Parallel()

	t.Run("Should classify global workspace additional and unknown definition roots", func(t *testing.T) {
		t.Parallel()
		home, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		workspaceRoot := t.TempDir()
		additionalRoot := t.TempDir()
		tests := []struct {
			name          string
			path          string
			wantOrigin    AgentOrigin
			wantWorkspace string
		}{
			{
				name:       "global",
				path:       filepath.Join(home.AgentsDir, "coder", AgentDefinitionFileName),
				wantOrigin: AgentOriginGlobal,
			},
			{
				name:          "workspace",
				path:          filepath.Join(workspaceRoot, DirName, AgentsDirName, "coder", AgentDefinitionFileName),
				wantOrigin:    AgentOriginWorkspace,
				wantWorkspace: workspaceRoot,
			},
			{
				name:          "additional",
				path:          filepath.Join(additionalRoot, DirName, AgentsDirName, "coder", AgentDefinitionFileName),
				wantOrigin:    AgentOriginWorkspace,
				wantWorkspace: additionalRoot,
			},
			{name: "unknown", path: filepath.Join(t.TempDir(), "coder", AgentDefinitionFileName)},
		}
		for _, tc := range tests {
			t.Run("Should classify "+tc.name, func(t *testing.T) {
				t.Parallel()
				origin, workspace := AgentOriginFor(home, tc.path)
				if origin != tc.wantOrigin || workspace != tc.wantWorkspace {
					t.Fatalf(
						"AgentOriginFor(%q) = (%q, %q), want (%q, %q)",
						tc.path,
						origin,
						workspace,
						tc.wantOrigin,
						tc.wantWorkspace,
					)
				}
			})
		}
	})

	t.Run("Should render and digest semantic definitions canonically", func(t *testing.T) {
		t.Parallel()
		draft := AgentDefinitionDraft{
			Name:      "coder",
			Provider:  "codex",
			Tools:     []string{"compozy__zeta", "compozy__alpha", "compozy__zeta"},
			Toolsets:  []string{"compozy__network", "compozy__catalog"},
			DenyTools: []string{"compozy__write_*", "compozy__delete_*"},
			Skills:    AgentSkillsConfig{Disabled: []string{"zeta", "alpha"}},
			MCPServers: []MCPServer{{
				Name:      "zeta",
				Transport: MCPServerTransportStdio,
				Command:   "zeta",
				Env:       map[string]string{"ZETA": "2", "ALPHA": "1"},
				SecretEnv: map[string]string{"TOKEN_Z": "env:TOKEN_Z", "TOKEN_A": "env:TOKEN_A"},
			}, {
				Name:      "alpha",
				Transport: MCPServerTransportHTTP,
				URL:       "https://example.test/mcp",
				Auth: MCPAuthConfig{
					Registration: MCPAuthRegistrationPreRegistered,
					IssuerURL:    "https://example.test",
					ClientID:     "client",
					Scopes:       []string{"write", "read"},
				},
			}},
			Prompt: "Operate.",
		}
		first, firstAgent, err := RenderAgentDefinition(draft)
		if err != nil {
			t.Fatalf("RenderAgentDefinition(first) error = %v", err)
		}
		second, secondAgent, err := RenderAgentDefinition(draft)
		if err != nil {
			t.Fatalf("RenderAgentDefinition(second) error = %v", err)
		}
		if !bytes.Equal(first, second) {
			t.Fatalf("repeated render differs:\nfirst=%s\nsecond=%s", first, second)
		}
		if !slices.Equal(firstAgent.Tools, []string{"compozy__alpha", "compozy__zeta"}) ||
			!slices.Equal(secondAgent.Skills.Disabled, []string{"alpha", "zeta"}) {
			t.Fatalf("canonical agent ordering = %#v", firstAgent)
		}
		firstDigest, err := AgentDefinitionDigest(firstAgent)
		if err != nil {
			t.Fatalf("AgentDefinitionDigest(first) error = %v", err)
		}
		secondDigest, err := AgentDefinitionDigest(secondAgent)
		if err != nil {
			t.Fatalf("AgentDefinitionDigest(second) error = %v", err)
		}
		if firstDigest != secondDigest {
			t.Fatalf("equal definition digests = %q and %q", firstDigest, secondDigest)
		}
		changed := CloneAgentDef(firstAgent)
		changed.Prompt = "Changed."
		changedDigest, err := AgentDefinitionDigest(changed)
		if err != nil {
			t.Fatalf("AgentDefinitionDigest(changed) error = %v", err)
		}
		if changedDigest == firstDigest {
			t.Fatalf("changed digest = %q, want different from %q", changedDigest, firstDigest)
		}
	})

	t.Run("Should remove the complete authored directory and report a repeated delete", func(t *testing.T) {
		t.Parallel()
		dir := filepath.Join(t.TempDir(), AgentsDirName, "coder")
		agentsRoot := filepath.Dir(dir)
		path := filepath.Join(dir, AgentDefinitionFileName)
		writeFile(t, path, "definition")
		writeFile(t, filepath.Join(dir, "SOUL.md"), "soul")
		writeFile(t, filepath.Join(dir, "HEARTBEAT.md"), "heartbeat")
		writeFile(t, filepath.Join(dir, "mcp.json"), "{}")
		if err := DeleteAgentDefinition(agentsRoot, path); err != nil {
			t.Fatalf("DeleteAgentDefinition() error = %v", err)
		}
		if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("os.Stat(deleted directory) error = %v, want os.ErrNotExist", err)
		}
		if err := DeleteAgentDefinition(agentsRoot, path); !errors.Is(err, ErrAgentDefinitionNotFound) ||
			!strings.Contains(err.Error(), path) {
			t.Fatalf("DeleteAgentDefinition(repeat) error = %v", err)
		}
	})

	t.Run("Should reject a shallow definition path without removing its parent", func(t *testing.T) {
		t.Parallel()
		parent := t.TempDir()
		definitionPath := filepath.Join(parent, AgentDefinitionFileName)
		sentinelPath := filepath.Join(parent, "sentinel")
		writeFile(t, definitionPath, "definition")
		writeFile(t, sentinelPath, "preserve")

		if err := DeleteAgentDefinition(filepath.Join(parent, AgentsDirName), definitionPath); err == nil ||
			!strings.Contains(err.Error(), "invalid agent definition delete target") {
			t.Fatalf("DeleteAgentDefinition(shallow path) error = %v, want invalid delete target", err)
		}
		if _, err := os.Stat(parent); err != nil {
			t.Fatalf("os.Stat(parent after rejected delete) error = %v", err)
		}
		if got, err := os.ReadFile(sentinelPath); err != nil || string(got) != "preserve" {
			t.Fatalf("sentinel after rejected delete = %q, error = %v", got, err)
		}
	})

	t.Run("Should reject symlinked deletion ancestry", func(t *testing.T) {
		t.Parallel()
		realAgentDir := filepath.Join(t.TempDir(), AgentsDirName, "coder")
		realDefinitionPath := filepath.Join(realAgentDir, AgentDefinitionFileName)
		sentinelPath := filepath.Join(realAgentDir, "sentinel")
		writeFile(t, realDefinitionPath, "definition")
		writeFile(t, sentinelPath, "preserve")

		t.Run("Should reject a symlinked agents directory", func(t *testing.T) {
			t.Parallel()
			linkedRoot := t.TempDir()
			linkedAgentsDir := filepath.Join(linkedRoot, AgentsDirName)
			if err := os.Symlink(filepath.Dir(realAgentDir), linkedAgentsDir); err != nil {
				t.Fatalf("os.Symlink(agents directory) error = %v", err)
			}
			linkedDefinitionPath := filepath.Join(linkedAgentsDir, "coder", AgentDefinitionFileName)
			if err := DeleteAgentDefinition(linkedAgentsDir, linkedDefinitionPath); err == nil ||
				!strings.Contains(err.Error(), "agents root") ||
				!strings.Contains(err.Error(), "must be a real directory") {
				t.Fatalf("DeleteAgentDefinition(symlinked agents directory) error = %v", err)
			}
			if _, err := os.Stat(sentinelPath); err != nil {
				t.Fatalf("os.Stat(sentinel after symlinked agents delete) error = %v", err)
			}
		})

		t.Run("Should reject a symlinked agent directory", func(t *testing.T) {
			t.Parallel()
			linkedAgentsDir := filepath.Join(t.TempDir(), AgentsDirName)
			if err := os.MkdirAll(linkedAgentsDir, 0o700); err != nil {
				t.Fatalf("os.MkdirAll(linked agents directory) error = %v", err)
			}
			linkedAgentDir := filepath.Join(linkedAgentsDir, "coder")
			if err := os.Symlink(realAgentDir, linkedAgentDir); err != nil {
				t.Fatalf("os.Symlink(agent directory) error = %v", err)
			}
			linkedDefinitionPath := filepath.Join(linkedAgentDir, AgentDefinitionFileName)
			if err := DeleteAgentDefinition(linkedAgentsDir, linkedDefinitionPath); err == nil ||
				!strings.Contains(err.Error(), "agent directory") ||
				!strings.Contains(err.Error(), "must be a real directory") {
				t.Fatalf("DeleteAgentDefinition(symlinked agent directory) error = %v", err)
			}
			if _, err := os.Stat(sentinelPath); err != nil {
				t.Fatalf("os.Stat(sentinel after symlinked agent delete) error = %v", err)
			}
		})
	})

	t.Run("Should reject invalid lifecycle paths before mutation", func(t *testing.T) {
		t.Parallel()
		validDraft := AgentDefinitionDraft{Name: "valid", Provider: "codex", Prompt: "Valid."}
		if _, err := CreateAgentDefFile("", validDraft, false); err == nil ||
			!strings.Contains(err.Error(), "agent definition path is required") {
			t.Fatalf("CreateAgentDefFile(empty path) error = %v, want required path", err)
		}
		invalidDraft := validDraft
		invalidDraft.Name = ""
		if _, err := CreateAgentDefFile(
			filepath.Join(t.TempDir(), AgentDefinitionFileName),
			invalidDraft,
			false,
		); !errors.Is(err, ErrInvalidAgentDefinition) {
			t.Fatalf("CreateAgentDefFile(invalid draft) error = %v, want ErrInvalidAgentDefinition", err)
		}
		if _, _, err := RenderAgentDefinition(invalidDraft); !errors.Is(err, ErrInvalidAgentDefinition) {
			t.Fatalf("RenderAgentDefinition(invalid draft) error = %v, want ErrInvalidAgentDefinition", err)
		}
		if _, err := AgentDefinitionDigest(AgentDef{}); !errors.Is(err, ErrInvalidAgentDefinition) {
			t.Fatalf("AgentDefinitionDigest(invalid definition) error = %v, want ErrInvalidAgentDefinition", err)
		}
		existingPath := filepath.Join(t.TempDir(), AgentDefinitionFileName)
		if _, err := CreateAgentDefFile(existingPath, validDraft, false); err != nil {
			t.Fatalf("CreateAgentDefFile(existing setup) error = %v", err)
		}
		if _, err := CreateAgentDefFile(existingPath, validDraft, false); !errors.Is(err, ErrAgentDefinitionExists) {
			t.Fatalf("CreateAgentDefFile(existing) error = %v, want ErrAgentDefinitionExists", err)
		}
		parentFile := filepath.Join(t.TempDir(), "parent-file")
		writeFile(t, parentFile, "not a directory")
		parentDefinitionPath := filepath.Join(parentFile, AgentDefinitionFileName)
		if _, err := CreateAgentDefFile(parentDefinitionPath, validDraft, false); err == nil ||
			!strings.Contains(err.Error(), "open config parent directory") {
			t.Fatalf("CreateAgentDefFile(non-directory parent) error = %v, want parent-open error", err)
		}
		if origin, workspace := AgentOriginFor(HomePaths{}, "not-an-agent.md"); origin != "" || workspace != "" {
			t.Fatalf("AgentOriginFor(invalid path) = (%q, %q), want empty classification", origin, workspace)
		}
		if err := DeleteAgentDefinition(filepath.Join(t.TempDir(), AgentsDirName), "not-an-agent.md"); err == nil ||
			!strings.Contains(err.Error(), "invalid agent definition source path") {
			t.Fatalf("DeleteAgentDefinition(invalid path) error = %v, want invalid source path", err)
		}
		agentsRoot := filepath.Join(t.TempDir(), AgentsDirName)
		directoryDefinition := filepath.Join(agentsRoot, "coder", AgentDefinitionFileName)
		if err := os.MkdirAll(directoryDefinition, 0o700); err != nil {
			t.Fatalf("MkdirAll(directory definition) error = %v", err)
		}
		if err := DeleteAgentDefinition(agentsRoot, directoryDefinition); err == nil ||
			!strings.Contains(err.Error(), "must be a regular file") {
			t.Fatalf("DeleteAgentDefinition(directory) error = %v, want regular-file error", err)
		}

		sourcePath := filepath.Join(t.TempDir(), "source", AgentDefinitionFileName)
		source, err := CreateAgentDefFile(sourcePath, AgentDefinitionDraft{
			Name: "source", Provider: "codex", Prompt: "Source.",
		}, false)
		if err != nil {
			t.Fatalf("CreateAgentDefFile(source) error = %v", err)
		}
		draft := AgentDefinitionDraftFromDef(source)
		draft.Name = "target"
		if _, err := DuplicateAgentDefinition(AgentDef{}, t.TempDir(), draft); err == nil ||
			!strings.Contains(err.Error(), "invalid source agent definition path") {
			t.Fatalf("DuplicateAgentDefinition(invalid source) error = %v", err)
		}
		if _, err := DuplicateAgentDefinition(source, "", draft); err == nil ||
			!strings.Contains(err.Error(), "duplicate target directory is required") {
			t.Fatalf("DuplicateAgentDefinition(empty target) error = %v", err)
		}
		symlinkPath := filepath.Join(filepath.Dir(sourcePath), "AGENT.link")
		if err := os.Symlink(AgentDefinitionFileName, symlinkPath); err != nil {
			t.Fatalf("Symlink(AGENT.link) error = %v", err)
		}
		symlinkTarget := filepath.Join(t.TempDir(), "target")
		if _, err := DuplicateAgentDefinition(source, symlinkTarget, draft); err == nil ||
			!strings.Contains(err.Error(), "must not be a symlink") {
			t.Fatalf("DuplicateAgentDefinition(symlink sibling) error = %v", err)
		}
		if _, err := os.Stat(symlinkTarget); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("os.Stat(failed symlink target) error = %v, want os.ErrNotExist", err)
		}
	})

	t.Run("Should duplicate overrides and copy siblings without modifying the source", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		sourceDir := filepath.Join(root, "source")
		sourcePath := filepath.Join(sourceDir, AgentDefinitionFileName)
		sourceDraft := AgentDefinitionDraft{Name: "source", Provider: "codex", Prompt: "Source."}
		source, err := CreateAgentDefFile(sourcePath, sourceDraft, false)
		if err != nil {
			t.Fatalf("CreateAgentDefFile(source) error = %v", err)
		}
		siblings := map[string][]byte{
			"SOUL.md":          []byte("soul\n"),
			"HEARTBEAT.md":     []byte("heartbeat\n"),
			"mcp.json":         []byte(`{"servers":{"github":{"secret_env":{"TOKEN":"secret"}}}}`),
			"notes/context.md": []byte("nested context\n"),
		}
		for name, contents := range siblings {
			writeFile(t, filepath.Join(sourceDir, name), string(contents))
		}
		sourceBefore, err := os.ReadFile(sourcePath)
		if err != nil {
			t.Fatalf("ReadFile(source before) error = %v", err)
		}
		targetDir := filepath.Join(root, "target")
		targetDraft := AgentDefinitionDraftFromDef(source)
		targetDraft.Name = "target"
		targetDraft.Model = "gpt-5"
		duplicate, err := DuplicateAgentDefinition(source, targetDir, targetDraft)
		if err != nil {
			t.Fatalf("DuplicateAgentDefinition() error = %v", err)
		}
		if duplicate.Name != "target" || duplicate.Model != "gpt-5" {
			t.Fatalf("duplicate = %#v, want target with model override", duplicate)
		}
		for name, want := range siblings {
			got, err := os.ReadFile(filepath.Join(targetDir, name))
			if err != nil {
				t.Fatalf("ReadFile(target %s) error = %v", name, err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("target %s = %q, want %q", name, got, want)
			}
		}
		sourceAfter, err := os.ReadFile(sourcePath)
		if err != nil {
			t.Fatalf("ReadFile(source after) error = %v", err)
		}
		if !bytes.Equal(sourceBefore, sourceAfter) {
			t.Fatalf("source changed:\nbefore=%s\nafter=%s", sourceBefore, sourceAfter)
		}
		if _, err := DuplicateAgentDefinition(
			source,
			targetDir,
			targetDraft,
		); !errors.Is(
			err,
			ErrAgentDefinitionExists,
		) {
			t.Fatalf("DuplicateAgentDefinition(existing) error = %v, want ErrAgentDefinitionExists", err)
		}
	})

	t.Run("Should resolve concurrent duplicate names without clobbering the winner", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		sourcePath := filepath.Join(root, "source", AgentDefinitionFileName)
		source, err := CreateAgentDefFile(sourcePath, AgentDefinitionDraft{
			Name: "source", Provider: "codex", Prompt: "Source.",
		}, false)
		if err != nil {
			t.Fatalf("CreateAgentDefFile(source) error = %v", err)
		}
		writeFile(t, filepath.Join(filepath.Dir(sourcePath), "SOUL.md"), "soul\n")

		targetDir := filepath.Join(root, "target")
		start := make(chan struct{})
		results := make(chan error, 2)
		var workers sync.WaitGroup
		for _, model := range []string{"gpt-5", "gpt-5-mini"} {
			workers.Go(func() {
				<-start
				draft := AgentDefinitionDraftFromDef(source)
				draft.Name = "target"
				draft.Model = model
				_, duplicateErr := DuplicateAgentDefinition(source, targetDir, draft)
				results <- duplicateErr
			})
		}
		close(start)
		workers.Wait()
		close(results)

		successes := 0
		conflicts := 0
		for result := range results {
			switch {
			case result == nil:
				successes++
			case errors.Is(result, ErrAgentDefinitionExists):
				conflicts++
			default:
				t.Fatalf("DuplicateAgentDefinition(concurrent) error = %v", result)
			}
		}
		if successes != 1 || conflicts != 1 {
			t.Fatalf("concurrent duplicate outcomes = (%d successes, %d conflicts), want (1, 1)", successes, conflicts)
		}
		loaded, err := LoadAgentDefFile(filepath.Join(targetDir, AgentDefinitionFileName))
		if err != nil {
			t.Fatalf("LoadAgentDefFile(target) error = %v", err)
		}
		if loaded.Model != "gpt-5" && loaded.Model != "gpt-5-mini" {
			t.Fatalf("target model = %q, want one complete winning definition", loaded.Model)
		}
		soulBody, err := os.ReadFile(filepath.Join(targetDir, "SOUL.md"))
		if err != nil {
			t.Fatalf("ReadFile(target SOUL.md) error = %v", err)
		}
		if string(soulBody) != "soul\n" {
			t.Fatalf("target SOUL.md = %q, want complete sibling copy", soulBody)
		}
	})
}

func TestSkillSourcePresetPolicy(t *testing.T) {
	t.Parallel()

	t.Run("Should expose the exact closed preset table", func(t *testing.T) {
		t.Parallel()

		got := SkillSourcePresets()
		want := []SkillSourcePreset{
			{
				Slug: SkillSourceCompozy, Label: "Compozy", AlwaysOn: true,
				WorkspaceNativeReaders: []string{}, GlobalNativeReaders: []string{},
			},
			{
				Slug: SkillSourceAgents, Label: "Agents", WorkspaceRel: ".agents/skills",
				GlobalPath: "~/.agents/skills", WorkspaceNativeReaders: []string{"openclaw", "hermes"},
				GlobalNativeReaders: []string{"openclaw"}, DefaultOn: true,
			},
			{
				Slug: SkillSourceClaude, Label: "Claude", WorkspaceRel: ".claude/skills",
				GlobalPath: "~/.claude/skills", WorkspaceNativeReaders: []string{"claude"},
				GlobalNativeReaders: []string{"claude"},
			},
		}
		if !slices.EqualFunc(got, want, func(left SkillSourcePreset, right SkillSourcePreset) bool {
			return left.Slug == right.Slug && left.Label == right.Label &&
				left.WorkspaceRel == right.WorkspaceRel && left.GlobalPath == right.GlobalPath &&
				slices.Equal(left.WorkspaceNativeReaders, right.WorkspaceNativeReaders) &&
				slices.Equal(left.GlobalNativeReaders, right.GlobalNativeReaders) &&
				left.AlwaysOn == right.AlwaysOn && left.DefaultOn == right.DefaultOn
		}) {
			t.Fatalf("SkillSourcePresets() = %#v, want %#v", got, want)
		}
	})

	t.Run("Should reject an unknown preset with valid choices and a suggestion", func(t *testing.T) {
		t.Parallel()

		err := ValidateSkillSources([]string{"agnets"})
		var validation *SkillSourceValidationError
		if !errors.As(err, &validation) {
			t.Fatalf("ValidateSkillSources() error = %v, want SkillSourceValidationError", err)
		}
		if validation.Code != "unknown_skill_source" || validation.Suggestion != SkillSourceAgents ||
			!slices.Equal(validation.Valid, []string{SkillSourceAgents, SkillSourceClaude}) {
			t.Fatalf("ValidateSkillSources() error = %#v, want unknown source metadata", validation)
		}
		for _, fragment := range []string{`unknown skill source preset "agnets"`, "valid: agents, claude", `agents`} {
			if !strings.Contains(err.Error(), fragment) {
				t.Fatalf("ValidateSkillSources() error = %q, want fragment %q", err, fragment)
			}
		}
	})

	t.Run("Should allocate sanitized custom slugs deterministically", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		first := filepath.Join(root, "a", "Team Skills")
		second := filepath.Join(root, "b", "Team Skills")
		third := filepath.Join(root, "Audit Tools")
		forward := CustomSourceSlugs([]string{first, second})
		reversed := CustomSourceSlugs([]string{second, first})
		if !maps.Equal(forward, reversed) {
			t.Fatalf("CustomSourceSlugs() order changed allocation: %#v != %#v", forward, reversed)
		}
		firstCanonical := canonicalSkillSourcePath(first)
		secondCanonical := canonicalSkillSourcePath(second)
		if forward[firstCanonical] == forward[secondCanonical] ||
			!strings.HasPrefix(forward[firstCanonical], "team-skills-") ||
			!strings.HasPrefix(forward[secondCanonical], "team-skills-") {
			t.Fatalf("CustomSourceSlugs() collision allocation = %#v", forward)
		}
		expanded := CustomSourceSlugs([]string{second, third, first})
		if expanded[firstCanonical] != forward[firstCanonical] || expanded[secondCanonical] != forward[secondCanonical] {
			t.Fatalf("CustomSourceSlugs() changed colliding slugs after unrelated addition: %#v", expanded)
		}
		if got := expanded[canonicalSkillSourcePath(third)]; got != "audit-tools" {
			t.Fatalf("CustomSourceSlugs() sanitized slug = %q, want audit-tools", got)
		}
	})

	t.Run("Should reject duplicate roots and non-workspace relative paths", func(t *testing.T) {
		t.Parallel()

		duplicate := SkillsConfig{
			Sources:       []string{SkillSourceAgents},
			CustomSources: []string{"~/.agents/skills"},
		}
		err := duplicate.ValidateForScope(WriteScopeUser)
		var validation *SkillSourceValidationError
		if !errors.As(err, &validation) || validation.Code != "duplicate_skill_source" ||
			validation.ExistingSource != SkillSourceAgents {
			t.Fatalf("ValidateForScope(duplicate) error = %#v", err)
		}

		relative := SkillsConfig{CustomSources: []string{"./rel/path"}}
		err = relative.ValidateForScope(WriteScopeUser)
		if !errors.As(err, &validation) || validation.Code != "invalid_source_path" ||
			!strings.Contains(err.Error(), "workspace-relative paths require workspace scope") {
			t.Fatalf("ValidateForScope(relative) error = %#v", err)
		}
		if err := relative.ValidateForScope(WriteScopeWorkspace); err != nil {
			t.Fatalf("ValidateForScope(workspace relative) error = %v", err)
		}
	})
}

func TestSkillRootResolutionAndIdentity(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve defaults enabled presets custom roots and compozy last", func(t *testing.T) {
		t.Parallel()

		operatorHome := t.TempDir()
		homePaths, err := ResolveHomePathsFrom(filepath.Join(operatorHome, DirName))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		workspaceRoot := t.TempDir()
		workspaceID := "workspace-stable"
		workspace := WorkspaceDiscoveryRoot{
			Dir: workspaceRoot, Source: WorkspaceDiscoverySourceWorkspace, WorkspaceID: workspaceID,
		}
		defaults := DefaultWithHome(homePaths).Skills

		global := ResolveGlobalSkillRoots(&defaults, homePaths)
		if got, want := rootSlugs(global), []string{SkillSourceAgents, SkillSourceCompozy}; !slices.Equal(got, want) {
			t.Fatalf("ResolveGlobalSkillRoots(default) slugs = %q, want %q", got, want)
		}
		if got, want := global[0].Dir, canonicalSkillSourcePath(filepath.Join(operatorHome, ".agents", "skills")); got != want {
			t.Fatalf("agents global dir = %q, want %q", got, want)
		}
		workspaceRoots := workspace.SkillsDirs(&defaults)
		if got, want := rootSlugs(workspaceRoots), []string{SkillSourceAgents, SkillSourceCompozy}; !slices.Equal(got, want) {
			t.Fatalf("SkillsDirs(default) slugs = %q, want %q", got, want)
		}
		if workspaceRoots[0].ResourceScope.Kind != resources.ResourceScopeKindWorkspace ||
			workspaceRoots[0].WorkspaceID != workspaceID {
			t.Fatalf("workspace root ownership = %#v", workspaceRoots[0])
		}

		customDir := filepath.Join(operatorHome, "Team Skills")
		configured := defaults
		configured.Sources = []string{SkillSourceAgents, SkillSourceClaude}
		configured.CustomSources = []string{customDir}
		global = ResolveGlobalSkillRoots(&configured, homePaths)
		if got, want := rootSlugs(global), []string{SkillSourceAgents, SkillSourceClaude, "team-skills", SkillSourceCompozy}; !slices.Equal(got, want) {
			t.Fatalf("ResolveGlobalSkillRoots(configured) slugs = %q, want %q", got, want)
		}
		if global[2].Kind != RootKindCustom || !filepath.IsAbs(global[2].Dir) {
			t.Fatalf("custom global root = %#v, want absolute custom root", global[2])
		}
		if global[2].ResourceScope.Kind != resources.ResourceScopeKindUser || global[2].WorkspaceID != "" {
			t.Fatalf("custom global root ownership = %#v, want user scope without workspace id", global[2])
		}
		workspaceConfigured := workspace.SkillsDirs(&configured)
		if got, want := rootSlugs(workspaceConfigured), []string{
			SkillSourceAgents, SkillSourceClaude, "team-skills", SkillSourceCompozy,
		}; !slices.Equal(got, want) {
			t.Fatalf("SkillsDirs(configured) slugs = %q, want %q", got, want)
		}

		empty := SkillsConfig{Sources: []string{}, CustomSources: []string{}}
		if got := rootSlugs(ResolveGlobalSkillRoots(&empty, homePaths)); !slices.Equal(got, []string{SkillSourceCompozy}) {
			t.Fatalf("ResolveGlobalSkillRoots(empty) slugs = %q", got)
		}
		if got := rootSlugs(workspace.SkillsDirs(&empty)); !slices.Equal(got, []string{SkillSourceCompozy}) {
			t.Fatalf("SkillsDirs(empty) slugs = %q", got)
		}
	})

	t.Run("Should keep root identity stable under ordering and display changes", func(t *testing.T) {
		t.Parallel()

		base := SkillRootSpec{
			Dir: filepath.Join(t.TempDir(), "skills"), Kind: RootKindCustom,
			ResourceScope: resources.ResourceScope{Kind: resources.ResourceScopeKindProfile, ID: "presentation-name"},
			ProfileID:     "profile-stable",
		}
		changedDisplay := base
		changedDisplay.SourceSlug = "different-display"
		changedDisplay.ResourceScope.ID = "renamed-presentation"
		if base.RootID() != changedDisplay.RootID() {
			t.Fatalf("RootID() changed with display-only fields: %q != %q", base.RootID(), changedDisplay.RootID())
		}
		restarted := SkillRootSpec{
			Dir: base.Dir, SourceSlug: base.SourceSlug, Kind: base.Kind,
			ResourceScope: base.ResourceScope, ProfileID: base.ProfileID, WorkspaceID: base.WorkspaceID,
		}
		if base.RootID() != restarted.RootID() {
			t.Fatalf("RootID() changed across equivalent reconstruction: %q != %q", base.RootID(), restarted.RootID())
		}
		otherProfile := base
		otherProfile.ProfileID = "profile-other"
		if base.RootID() == otherProfile.RootID() {
			t.Fatal("RootID() matched roots owned by different profiles")
		}
		workspaceOwned := base
		workspaceOwned.ResourceScope = resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "workspace-a"}
		workspaceOwned.ProfileID = ""
		workspaceOwned.WorkspaceID = "workspace-a"
		if base.RootID() == workspaceOwned.RootID() {
			t.Fatal("RootID() matched roots in different resource scopes")
		}

		collisionRoot := t.TempDir()
		firstPath := filepath.Join(collisionRoot, "a", "team-skills")
		secondPath := filepath.Join(collisionRoot, "b", "team-skills")
		beforeCollision := SkillRootSpec{
			Dir: firstPath, SourceSlug: "team-skills", Kind: RootKindCustom,
			ResourceScope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
		}
		reallocated := beforeCollision
		reallocated.SourceSlug = CustomSourceSlugs([]string{firstPath, secondPath})[canonicalSkillSourcePath(firstPath)]
		if beforeCollision.RootID() != reallocated.RootID() {
			t.Fatalf("RootID() changed after display slug reallocation: %q != %q", beforeCollision.RootID(), reallocated.RootID())
		}

		beforeRename := base
		afterRename := base
		afterRename.Dir = filepath.Join(filepath.Dir(base.Dir), "renamed-profile", "skills")
		if beforeRename.ProfileID != afterRename.ProfileID || beforeRename.RootID() == afterRename.RootID() {
			t.Fatalf("profile rename identity/path result = (%q, %q), want stable owner and regenerated root", beforeRename.RootID(), afterRename.RootID())
		}
	})
}

func rootSlugs(roots []SkillRootSpec) []string {
	result := make([]string, len(roots))
	for index := range roots {
		result[index] = roots[index].SourceSlug
	}
	return result
}

func TestDuplicateAgentDefinitionWritesThroughTheHeldTargetParentAfterPathReplacement(t *testing.T) {
	t.Parallel()
	t.Run("Should duplicate only below the opened target parent", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" {
			t.Skip("symlink permissions vary on Windows")
		}

		root := t.TempDir()
		sourcePath := filepath.Join(root, "source", AgentDefinitionFileName)
		source, err := CreateAgentDefFile(sourcePath, AgentDefinitionDraft{
			Name: "source", Provider: "codex", Prompt: "Source.",
		}, false)
		if err != nil {
			t.Fatalf("CreateAgentDefFile(source) error = %v", err)
		}
		writeFile(t, filepath.Join(filepath.Dir(sourcePath), "SOUL.md"), "source soul\n")

		liveParent := filepath.Join(root, "live")
		archivedParent := filepath.Join(root, "archived")
		externalParent := filepath.Join(root, "external")
		if err := os.MkdirAll(liveParent, 0o700); err != nil {
			t.Fatalf("os.MkdirAll(live parent) error = %v", err)
		}
		if err := os.MkdirAll(externalParent, 0o700); err != nil {
			t.Fatalf("os.MkdirAll(external parent) error = %v", err)
		}
		sentinelPath := filepath.Join(externalParent, "target", AgentDefinitionFileName)
		writeFile(t, sentinelPath, "external sentinel\n")

		sourceDirectory, err := fileutil.OpenDirectory(filepath.Dir(source.SourcePath))
		if err != nil {
			t.Fatalf("fileutil.OpenDirectory(source) error = %v", err)
		}
		defer func() {
			if closeErr := sourceDirectory.Close(); closeErr != nil {
				t.Errorf("source Directory.Close() error = %v", closeErr)
			}
		}()
		targetParent, err := fileutil.OpenDirectory(liveParent)
		if err != nil {
			t.Fatalf("fileutil.OpenDirectory(target parent) error = %v", err)
		}
		defer func() {
			if closeErr := targetParent.Close(); closeErr != nil {
				t.Errorf("target Directory.Close() error = %v", closeErr)
			}
		}()
		if err := os.Rename(liveParent, archivedParent); err != nil {
			t.Fatalf("os.Rename(live parent) error = %v", err)
		}
		if err := os.Symlink(externalParent, liveParent); err != nil {
			t.Fatalf("os.Symlink(external parent) error = %v", err)
		}

		draft := AgentDefinitionDraftFromDef(source)
		draft.Name = "target"
		contents, _, err := RenderAgentDefinition(draft)
		if err != nil {
			t.Fatalf("RenderAgentDefinition(target) error = %v", err)
		}
		if err := duplicateAgentDefinitionInDirectories(
			sourceDirectory,
			targetParent,
			"target",
			filepath.Join(liveParent, "target"),
			contents,
		); err != nil {
			t.Fatalf("duplicateAgentDefinitionInDirectories() error = %v", err)
		}
		assertConfigSentinelUnchanged(t, sentinelPath, "external sentinel\n")
		if _, err := os.Stat(filepath.Join(archivedParent, "target", AgentDefinitionFileName)); err != nil {
			t.Fatalf("os.Stat(archived duplicate definition) error = %v", err)
		}
		if got, err := os.ReadFile(
			filepath.Join(archivedParent, "target", "SOUL.md"),
		); err != nil ||
			string(got) != "source soul\n" {
			t.Fatalf("archived duplicate SOUL.md = %q, error = %v", got, err)
		}
	})
}

func TestDeleteAgentDefinitionRemovesThroughTheHeldRootAfterPathReplacement(t *testing.T) {
	t.Parallel()
	t.Run("Should remove only below the opened agents root", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" {
			t.Skip("symlink permissions vary on Windows")
		}

		root := t.TempDir()
		agentsRoot := filepath.Join(root, AgentsDirName)
		archivedRoot := filepath.Join(root, "archived-agents")
		externalRoot := filepath.Join(root, "external-agents")
		definitionPath := filepath.Join(agentsRoot, "coder", AgentDefinitionFileName)
		writeFile(t, definitionPath, "definition\n")
		writeFile(t, filepath.Join(agentsRoot, "coder", "SOUL.md"), "remove\n")
		externalSentinel := filepath.Join(externalRoot, "coder", "sentinel")
		writeFile(t, externalSentinel, "external sentinel\n")
		target, err := resolveAgentDeleteTarget(agentsRoot, definitionPath)
		if err != nil {
			t.Fatalf("resolveAgentDeleteTarget() error = %v", err)
		}

		directory, err := fileutil.OpenDirectory(agentsRoot)
		if err != nil {
			t.Fatalf("fileutil.OpenDirectory(agents root) error = %v", err)
		}
		defer func() {
			if closeErr := directory.Close(); closeErr != nil {
				t.Errorf("agents Directory.Close() error = %v", closeErr)
			}
		}()
		if err := os.Rename(agentsRoot, archivedRoot); err != nil {
			t.Fatalf("os.Rename(agents root) error = %v", err)
		}
		if err := os.Symlink(externalRoot, agentsRoot); err != nil {
			t.Fatalf("os.Symlink(external agents root) error = %v", err)
		}

		if err := deleteAgentDefinitionFromRoot(directory, target); err != nil {
			t.Fatalf("deleteAgentDefinitionFromRoot() error = %v", err)
		}
		if _, err := os.Stat(filepath.Join(archivedRoot, "coder")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("os.Stat(archived definition) error = %v, want %v", err, os.ErrNotExist)
		}
		assertConfigSentinelUnchanged(t, externalSentinel, "external sentinel\n")
	})
}

func TestParseAgentDefValidFrontmatterAndBody(t *testing.T) {
	t.Parallel()

	t.Run("Should parse supported frontmatter and body", func(t *testing.T) {
		t.Parallel()

		agent, err := ParseAgentDef([]byte(`---
name: coder
provider: claude
model: claude-opus
reasoning_effort: max
tools: ["compozy__skill_view", "mcp__github__*"]
toolsets: ["compozy__catalog"]
deny_tools: ["compozy__task_*"]
permissions: approve-reads
mcp_servers:
  - name: github
    command: npx
    args: ["-y", "@modelcontextprotocol/server-github"]
---

You are a senior Go engineer.
`))
		if err != nil {
			t.Fatalf("ParseAgentDef() error = %v", err)
		}

		if agent.Name != "coder" || agent.Provider != "claude" || agent.Model != "claude-opus" ||
			agent.ReasoningEffort != providerReasoningMaxKey {
			t.Fatalf("ParseAgentDef() = %#v", agent)
		}
		if len(agent.Tools) != 2 {
			t.Fatalf("ParseAgentDef() Tools = %#v", agent.Tools)
		}
		if got, want := strings.Join(agent.Toolsets, ","), "compozy__catalog"; got != want {
			t.Fatalf("ParseAgentDef() Toolsets = %#v, want %q", agent.Toolsets, want)
		}
		if got, want := strings.Join(agent.DenyTools, ","), "compozy__task_*"; got != want {
			t.Fatalf("ParseAgentDef() DenyTools = %#v, want %q", agent.DenyTools, want)
		}
		if !strings.Contains(agent.Prompt, "senior Go engineer") {
			t.Fatalf("ParseAgentDef() Prompt = %q", agent.Prompt)
		}
		if len(agent.MCPServers) != 1 || agent.MCPServers[0].Name != "github" {
			t.Fatalf("ParseAgentDef() MCPServers = %#v", agent.MCPServers)
		}
	})
}

func TestParseAgentDefRejectsInvalidToolGrammar(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name: "ShouldRejectLegacySingleToolName",
			content: `---
name: coder
tools: ["bash"]
---

Prompt.
`,
			wantErr: "agent.tools[0]",
		},
		{
			name: "ShouldRejectGlobalWildcard",
			content: `---
name: coder
tools: ["*"]
---

Prompt.
`,
			wantErr: "agent.tools[0]",
		},
		{
			name: "ShouldRejectMidSegmentWildcard",
			content: `---
name: coder
tools: ["compozy__*__view"]
---

Prompt.
`,
			wantErr: "agent.tools[0]",
		},
		{
			name: "ShouldRejectDottedToolID",
			content: `---
name: coder
tools: ["compozy.skill_view"]
---

Prompt.
`,
			wantErr: "agent.tools[0]",
		},
		{
			name: "ShouldRejectLegacySingleToolsetName",
			content: `---
name: coder
toolsets: ["core"]
---

Prompt.
`,
			wantErr: "agent.toolsets[0]",
		},
		{
			name: "ShouldRejectDottedToolsetID",
			content: `---
name: coder
toolsets: ["compozy.core"]
---

Prompt.
`,
			wantErr: "agent.toolsets[0]",
		},
		{
			name: "ShouldRejectDenyToolSuffixWildcard",
			content: `---
name: coder
deny_tools: ["*__search"]
---

Prompt.
`,
			wantErr: "agent.deny_tools[0]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := ParseAgentDef([]byte(tt.content))
			if err == nil {
				t.Fatal("ParseAgentDef() error = nil, want invalid tool grammar")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ParseAgentDef() error = %q, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestParseAgentDefAllowsDenyToolsToNarrowAllowedTools(t *testing.T) {
	t.Parallel()

	t.Run("Should allow deny tools to overlap allowed tools", func(t *testing.T) {
		t.Parallel()

		agent, err := ParseAgentDef([]byte(`---
name: coder
tools: ["compozy__skill_view"]
deny_tools: ["compozy__skill_view"]
---

Prompt.
`))
		if err != nil {
			t.Fatalf("ParseAgentDef() error = %v", err)
		}
		if got, want := strings.Join(agent.Tools, ","), "compozy__skill_view"; got != want {
			t.Fatalf("ParseAgentDef() Tools = %#v, want %q", agent.Tools, want)
		}
		if got, want := strings.Join(agent.DenyTools, ","), "compozy__skill_view"; got != want {
			t.Fatalf("ParseAgentDef() DenyTools = %#v, want %q", agent.DenyTools, want)
		}
	})
}

func TestParseAgentDefNormalizesCRLFAndPreservesConfigFrontmatterErrors(t *testing.T) {
	t.Parallel()

	agent, err := ParseAgentDef([]byte("---\r\nname: windows\r\nprovider: claude\r\n---\r\nPrompt on CRLF.\r\n"))
	if err != nil {
		t.Fatalf("ParseAgentDef() error = %v", err)
	}
	if got, want := agent.Prompt, "Prompt on CRLF."; got != want {
		t.Fatalf("ParseAgentDef() Prompt = %q, want %q", got, want)
	}

	if _, err := ParseAgentDef([]byte("plain markdown")); err == nil {
		t.Fatal("ParseAgentDef() missing frontmatter error = nil, want non-nil")
	} else if !errors.Is(err, ErrMissingAgentFrontmatter) || !errors.Is(err, frontmatter.ErrMissing) {
		t.Fatalf("ParseAgentDef() missing frontmatter error = %v, want mapped config + frontmatter sentinel", err)
	}

	if _, err := ParseAgentDef([]byte("---\nname: broken")); err == nil {
		t.Fatal("ParseAgentDef() unterminated frontmatter error = nil, want non-nil")
	} else if !errors.Is(err, ErrUnterminatedAgentFrontmatter) || !errors.Is(err, frontmatter.ErrUnterminated) {
		t.Fatalf("ParseAgentDef() unterminated frontmatter error = %v, want mapped config + frontmatter sentinel", err)
	}

	if _, err := ParseAgentDef([]byte("\ufeff---\nname: broken\n---\nPrompt.")); err == nil {
		t.Fatal("ParseAgentDef() BOM frontmatter error = nil, want non-nil")
	} else if !errors.Is(err, ErrBOMAgentFrontmatter) || !errors.Is(err, frontmatter.ErrBOM) {
		t.Fatalf("ParseAgentDef() BOM frontmatter error = %v, want mapped config + frontmatter sentinel", err)
	}

	if _, err := ParseAgentDef([]byte("---\nna\tme: broken\n---\nPrompt.")); err == nil {
		t.Fatal("ParseAgentDef() invalid key error = nil, want non-nil")
	} else if !errors.Is(err, ErrInvalidAgentFrontmatterKey) {
		t.Fatalf("ParseAgentDef() invalid key error = %v, want invalid key sentinel", err)
	}
}

func TestLoadAgentDefFromHomePath(t *testing.T) {
	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	writeFile(t, filepath.Join(homePaths.AgentsDir, "coder", agentDefName), `---
name: coder
provider: claude
---

You write reliable code.
`)

	agent, err := LoadAgentDef("coder", homePaths)
	if err != nil {
		t.Fatalf("LoadAgentDef() error = %v", err)
	}
	if agent.Name != "coder" || agent.Provider != "claude" {
		t.Fatalf("LoadAgentDef() = %#v", agent)
	}
}

func TestParseAgentDefValidationFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
	}{
		{
			name: "ShouldRejectMissingName",
			content: `---
provider: claude
---

prompt`,
		},
		{
			name: "ShouldRejectMissingPrompt",
			content: `---
name: coder
provider: claude
---`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := ParseAgentDef([]byte(tt.content)); err == nil {
				t.Fatal("ParseAgentDef() error = nil, want non-nil")
			}
		})
	}

	t.Run("Should reject a non-canonical reasoning effort", func(t *testing.T) {
		t.Parallel()

		_, err := ParseAgentDef([]byte(`---
name: coder
provider: claude
reasoning_effort: ultra
---

prompt`))

		invalid, invalidMatched := errors.AsType[*reasoning.InvalidEffortError](err)
		if !invalidMatched {
			t.Fatalf("ParseAgentDef() error = %T %v, want *reasoning.InvalidEffortError", err, err)
		}
		if invalid.Path != "agent.reasoning_effort" || invalid.Value != "ultra" {
			t.Fatalf("InvalidEffortError = %#v, want agent reasoning path and ultra value", invalid)
		}
	})
}

func TestParseAgentDefAllowsMissingProvider(t *testing.T) {
	agent, err := ParseAgentDef([]byte(`---
name: general
---

You are the default agent.
`))
	if err != nil {
		t.Fatalf("ParseAgentDef() error = %v", err)
	}
	if agent.Provider != "" {
		t.Fatalf("ParseAgentDef() Provider = %q, want empty", agent.Provider)
	}
}

func TestParseAgentDefFrontmatterErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "ShouldRejectMissingFrontmatter",
			content: "plain markdown",
		},
		{
			name: "ShouldRejectUnterminatedFrontmatter",
			content: `---
name: coder
provider: claude`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseAgentDef([]byte(tt.content)); err == nil {
				t.Fatal("ParseAgentDef() error = nil, want non-nil")
			}
		})
	}
}

func TestParseAgentDefPreservesParserErrorsInDecodeChain(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve YAML and TOML parser errors in the decode chain", func(t *testing.T) {
		t.Parallel()

		_, err := ParseAgentDef([]byte(`---
name: [
---

Prompt.
`))
		if err == nil {
			t.Fatal("ParseAgentDef() error = nil, want decode failure")
		}

		yamlErr, yamlErrMatched := errors.AsType[*yaml.SyntaxError](err)
		if !yamlErrMatched {
			t.Fatalf("ParseAgentDef() error = %v, want yaml syntax error in unwrap chain", err)
		}
		if yamlErr.Message == "" || yamlErr.Token == nil {
			t.Fatalf("YAML syntax error = %#v, want message and source token", yamlErr)
		}

		tomlErr, tomlErrMatched := errors.AsType[toml.ParseError](err)
		if !tomlErrMatched {
			t.Fatalf("ParseAgentDef() error = %v, want TOML parse error in unwrap chain", err)
		}
		if tomlErr.Message == "" || tomlErr.Position.Line == 0 {
			t.Fatalf("TOML parse error = %#v, want message and source line", tomlErr)
		}
	})
}

func TestLoadAgentDefFileMissingReturnsError(t *testing.T) {
	if _, err := LoadAgentDefFile(filepath.Join(t.TempDir(), "missing", agentDefName)); err == nil {
		t.Fatal("LoadAgentDefFile() error = nil, want non-nil")
	}
}

func TestLoadAgentDefFileMergesMCPSidecar(t *testing.T) {
	t.Parallel()

	agentDir := filepath.Join(t.TempDir(), "coder")
	agentPath := filepath.Join(agentDir, agentDefName)
	writeFile(t, agentPath, `---
name: coder
provider: claude
mcp_servers:
  - name: inline-only
    command: inline-only-command
  - name: shared
    command: inline-shared
    args: ["--inline"]
---

Prompt.
`)
	writeFile(t, filepath.Join(agentDir, MCPJSONName), `{
  "mcpServers": {
    "shared": {
      "command": "sidecar-shared"
    },
    "sidecar-only": {
      "command": "sidecar-only-command"
    }
  }
}`)

	agent, err := LoadAgentDefFile(agentPath)
	if err != nil {
		t.Fatalf("LoadAgentDefFile() error = %v", err)
	}

	if got, want := len(agent.MCPServers), 3; got != want {
		t.Fatalf("LoadAgentDefFile() MCPServers len = %d, want %d (%#v)", got, want, agent.MCPServers)
	}
	if got, want := agent.MCPServers[1].Command, "sidecar-shared"; got != want {
		t.Fatalf("LoadAgentDefFile() shared.Command = %q, want %q", got, want)
	}
	if got := len(agent.MCPServers[1].Args); got != 0 {
		t.Fatalf(
			"LoadAgentDefFile() shared.Args = %#v, want sidecar whole-object replacement",
			agent.MCPServers[1].Args,
		)
	}
	if got, want := agent.MCPServers[2].Name, "sidecar-only"; got != want {
		t.Fatalf("LoadAgentDefFile() MCPServers[2].Name = %q, want %q", got, want)
	}
}

func TestLoadAgentDefRejectsBlankAndMismatchedNames(t *testing.T) {
	t.Parallel()

	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	if _, err := LoadAgentDef("   ", homePaths); err == nil {
		t.Fatal("LoadAgentDef(blank) error = nil, want non-nil")
	}

	writeFile(t, filepath.Join(homePaths.AgentsDir, "coder", agentDefName), `---
name: reviewer
provider: claude
---

Mismatch
`)

	if _, err := LoadAgentDef("coder", homePaths); err == nil {
		t.Fatal("LoadAgentDef(mismatched name) error = nil, want non-nil")
	}
}

func TestWorkspaceDiscoveryRootsReturnsWorkspaceAdditionalGlobalOrder(t *testing.T) {
	t.Parallel()

	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}

	root := t.TempDir()
	additionalOne := t.TempDir()
	additionalTwo := t.TempDir()

	roots := WorkspaceDiscoveryRoots(root, []string{additionalOne, additionalTwo}, homePaths, "")
	if got, want := len(roots), 4; got != want {
		t.Fatalf("len(WorkspaceDiscoveryRoots()) = %d, want %d", got, want)
	}

	if got, want := roots[0].Dir, root; got != want {
		t.Fatalf("roots[0].Dir = %q, want %q", got, want)
	}
	if got, want := roots[0].Source, WorkspaceDiscoverySourceWorkspace; got != want {
		t.Fatalf("roots[0].Source = %q, want %q", got, want)
	}
	if got, want := roots[1].Dir, additionalOne; got != want {
		t.Fatalf("roots[1].Dir = %q, want %q", got, want)
	}
	if got, want := roots[1].Source, WorkspaceDiscoverySourceAdditional; got != want {
		t.Fatalf("roots[1].Source = %q, want %q", got, want)
	}
	if got, want := roots[2].Dir, additionalTwo; got != want {
		t.Fatalf("roots[2].Dir = %q, want %q", got, want)
	}
	if got, want := roots[2].Source, WorkspaceDiscoverySourceAdditional; got != want {
		t.Fatalf("roots[2].Source = %q, want %q", got, want)
	}
	if got, want := roots[3].Dir, homePaths.HomeDir; got != want {
		t.Fatalf("roots[3].Dir = %q, want %q", got, want)
	}
	if got, want := roots[3].Source, WorkspaceDiscoverySourceGlobal; got != want {
		t.Fatalf("roots[3].Source = %q, want %q", got, want)
	}

	defaultSkills := DefaultWithHome(homePaths).Skills
	workspaceSkillRoots := roots[0].SkillsDirs(&defaultSkills)
	if got, want := workspaceSkillRoots[len(workspaceSkillRoots)-1].Dir, filepath.Join(root, DirName, SkillsDirName); got != want {
		t.Fatalf("workspace compozy skill root = %q, want %q", got, want)
	}
	globalSkillRoots := roots[3].SkillsDirs(&defaultSkills)
	if got, want := globalSkillRoots[len(globalSkillRoots)-1].Dir, filepath.Join(homePaths.HomeDir, SkillsDirName); got != want {
		t.Fatalf("user compozy skill root = %q, want %q", got, want)
	}
	if got, want := roots[0].LoopsDir(), filepath.Join(root, DirName, LoopsDirName); got != want {
		t.Fatalf("roots[0].LoopsDir() = %q, want %q", got, want)
	}
	if got, want := roots[3].LoopsDir(), filepath.Join(homePaths.HomeDir, LoopsDirName); got != want {
		t.Fatalf("roots[3].LoopsDir() = %q, want %q", got, want)
	}

	profileRoots := WorkspaceDiscoveryRoots(
		root,
		[]string{additionalOne, additionalTwo},
		homePaths,
		"marketing",
	)
	if got, want := len(profileRoots), 6; got != want {
		t.Fatalf("len(profile roots) = %d, want %d", got, want)
	}
	wantSources := []WorkspaceDiscoverySource{
		WorkspaceDiscoverySourceWorkspaceProfile,
		WorkspaceDiscoverySourceWorkspace,
		WorkspaceDiscoverySourceAdditional,
		WorkspaceDiscoverySourceAdditional,
		WorkspaceDiscoverySourceProfile,
		WorkspaceDiscoverySourceGlobal,
	}
	for index, want := range wantSources {
		if got := profileRoots[index].Source; got != want {
			t.Fatalf("profileRoots[%d].Source = %q, want %q", index, got, want)
		}
	}
	if got, want := profileRoots[0].AgentsDir(), filepath.Join(
		root,
		DirName,
		ProfilesDirName,
		"marketing",
		AgentsDirName,
	); got != want {
		t.Fatalf("project profile agents dir = %q, want %q", got, want)
	}
	personalSkillRoots := profileRoots[4].SkillsDirs(&defaultSkills)
	if got, want := personalSkillRoots[len(personalSkillRoots)-1].Dir, filepath.Join(
		homePaths.ProfilesDir,
		"marketing",
		SkillsDirName,
	); got != want {
		t.Fatalf("personal profile skills dir = %q, want %q", got, want)
	}
}

func TestLoadWorkspaceAgentDefsAppliesDocumentedPrecedence(t *testing.T) {
	t.Parallel()

	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	root := t.TempDir()
	additionalOne := t.TempDir()
	additionalTwo := t.TempDir()

	writeAgentDefinition(t, filepath.Join(homePaths.AgentsDir, "coder", agentDefName), "coder", "claude", "global")
	writeAgentDefinition(
		t,
		filepath.Join(homePaths.AgentsDir, "reviewer", agentDefName),
		"reviewer",
		"claude",
		"global-review",
	)
	writeAgentDefinition(
		t,
		filepath.Join(additionalOne, DirName, AgentsDirName, "coder", agentDefName),
		"coder",
		"claude",
		"additional",
	)
	writeAgentDefinition(
		t,
		filepath.Join(additionalOne, DirName, AgentsDirName, "pairer", agentDefName),
		"pairer",
		"claude",
		"additional-pair",
	)
	writeAgentDefinition(
		t,
		filepath.Join(additionalTwo, DirName, AgentsDirName, "reviewer", agentDefName),
		"reviewer",
		"claude",
		"additional-review",
	)
	writeAgentDefinition(
		t,
		filepath.Join(root, DirName, AgentsDirName, "coder", agentDefName),
		"coder",
		"claude",
		"workspace",
	)
	writeAgentDefinition(
		t,
		filepath.Join(homePaths.ProfilesDir, "marketing", AgentsDirName, "coder", agentDefName),
		"coder",
		"claude",
		"profile",
	)
	writeAgentDefinition(
		t,
		filepath.Join(homePaths.ProfilesDir, "marketing", AgentsDirName, "personal", agentDefName),
		"personal",
		"claude",
		"profile-only",
	)
	writeAgentDefinition(
		t,
		filepath.Join(
			root,
			DirName,
			ProfilesDirName,
			"marketing",
			AgentsDirName,
			"coder",
			agentDefName,
		),
		"coder",
		"claude",
		"workspace-profile",
	)

	agents, err := LoadWorkspaceAgentDefs(root, []string{additionalOne, additionalTwo}, homePaths, "")
	if err != nil {
		t.Fatalf("LoadWorkspaceAgentDefs() error = %v", err)
	}

	if got, want := agentModel(agents, "coder"), "workspace"; got != want {
		t.Fatalf("coder model = %q, want %q", got, want)
	}
	if got, want := agentModel(agents, "pairer"), "additional-pair"; got != want {
		t.Fatalf("pairer model = %q, want %q", got, want)
	}
	if got, want := agentModel(agents, "reviewer"), "additional-review"; got != want {
		t.Fatalf("reviewer model = %q, want %q", got, want)
	}

	profileAgents, err := LoadWorkspaceAgentDefs(
		root,
		[]string{additionalOne, additionalTwo},
		homePaths,
		"marketing",
	)
	if err != nil {
		t.Fatalf("LoadWorkspaceAgentDefs(profile) error = %v", err)
	}
	if got, want := agentModel(profileAgents, "coder"), "workspace-profile"; got != want {
		t.Fatalf("profile coder model = %q, want %q", got, want)
	}
	var coder AgentDef
	for _, agent := range profileAgents {
		if agent.Name == "coder" {
			coder = agent
			break
		}
	}
	if got, want := coder.SourceLayer, "project_profile"; got != want {
		t.Fatalf("coder source layer = %q, want %q", got, want)
	}
	wantShadowLayers := []string{"project", "additional", "profile", "user"}
	shadowLayers := make([]string, 0, len(coder.ShadowedDefinitions))
	for _, shadow := range coder.ShadowedDefinitions {
		shadowLayers = append(shadowLayers, shadow.Layer)
	}
	if !slices.Equal(shadowLayers, wantShadowLayers) {
		t.Fatalf("coder shadow layers = %#v, want %#v", shadowLayers, wantShadowLayers)
	}
	if got, want := agentModel(profileAgents, "personal"), "profile-only"; got != want {
		t.Fatalf("personal model = %q, want %q", got, want)
	}
	if _, err := LoadWorkspaceAgentDefs(root, nil, homePaths, "../escape"); err == nil {
		t.Fatal("LoadWorkspaceAgentDefs(unsafe profile) error = nil, want validation error")
	} else if !strings.Contains(err.Error(), "resource profile name") {
		t.Fatalf("LoadWorkspaceAgentDefs(unsafe profile) error = %v, want resource profile validation", err)
	}
}

func writeAgentDefinition(t *testing.T, path string, name string, provider string, model string) {
	t.Helper()

	writeFile(t, path, `---
name: `+name+`
provider: `+provider+`
model: `+model+`
---

Prompt for `+name+`.
`)
}

func agentModel(agents []AgentDef, name string) string {
	for _, agent := range agents {
		if agent.Name == name {
			return agent.Model
		}
	}

	return ""
}
