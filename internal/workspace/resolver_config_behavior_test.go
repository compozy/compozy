package workspace

import (
	"context"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/sandbox"
)

func TestResolveWorkspaceSandboxCascade(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	homePaths := newTestHomePaths(t)
	baseConfig := validConfig(homePaths)
	baseConfig.Defaults.Sandbox = "default-env"
	baseConfig.Sandboxes["default-env"] = compozyconfig.SandboxProfile{
		Backend:     "daytona",
		Persistence: "reuse",
		Daytona: compozyconfig.DaytonaProfile{
			Snapshot: "snap-default",
		},
	}
	baseConfig.Sandboxes["explicit-env"] = compozyconfig.SandboxProfile{
		Backend:  "daytona",
		SyncMode: "none",
		Daytona: compozyconfig.DaytonaProfile{
			Snapshot: "snap-explicit",
		},
	}

	tests := []struct {
		name        string
		workspace   Workspace
		cfg         compozyconfig.Config
		wantProfile string
		wantBackend sandbox.Backend
		wantSync    sandbox.SyncMode
	}{
		{
			name: "workspace ref wins over default",
			workspace: Workspace{
				ID:         "ws_explicit",
				RootDir:    mustCanonicalRoot(t, t.TempDir()),
				Name:       "explicit",
				SandboxRef: "explicit-env",
			},
			cfg:         baseConfig,
			wantProfile: "explicit-env",
			wantBackend: sandbox.BackendDaytona,
			wantSync:    sandbox.SyncModeNone,
		},
		{
			name: "defaults sandbox applies when workspace omits ref",
			workspace: Workspace{
				ID:      "ws_default",
				RootDir: mustCanonicalRoot(t, t.TempDir()),
				Name:    "default",
			},
			cfg:         baseConfig,
			wantProfile: "default-env",
			wantBackend: sandbox.BackendDaytona,
			wantSync:    sandbox.SyncModeSessionBidirectional,
		},
		{
			name: "implicit local applies with no workspace or default ref",
			workspace: Workspace{
				ID:      "ws_local",
				RootDir: mustCanonicalRoot(t, t.TempDir()),
				Name:    "local",
			},
			cfg: func() compozyconfig.Config {
				cfg := validConfig(homePaths)
				cfg.Defaults.Sandbox = ""
				return cfg
			}(),
			wantProfile: "local",
			wantBackend: sandbox.BackendLocal,
			wantSync:    sandbox.SyncModeNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := newMockWorkspaceStore(tt.workspace)
			loader := &countingConfigLoader{cfg: tt.cfg}
			resolver := newTestResolver(t, store,
				WithHomePaths(homePaths),
				WithConfigLoader(loader.Load),
			)

			resolved, err := resolver.Resolve(ctx, tt.workspace.ID)
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if resolved.Sandbox.Profile != tt.wantProfile ||
				resolved.Sandbox.Backend != tt.wantBackend ||
				resolved.Sandbox.SyncMode != tt.wantSync {
				t.Fatalf("resolved sandbox = %#v, want profile=%q backend=%q sync=%q",
					resolved.Sandbox,
					tt.wantProfile,
					tt.wantBackend,
					tt.wantSync,
				)
			}
		})
	}
}

func TestRegisterUpdateAndLoadWorkspaceSandboxRef(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	homePaths := newTestHomePaths(t)
	cfg := validConfig(homePaths)
	cfg.Sandboxes["daytona-dev"] = compozyconfig.SandboxProfile{
		Backend: "daytona",
		Daytona: compozyconfig.DaytonaProfile{Snapshot: "snap-dev"},
	}
	cfg.Sandboxes["local-dev"] = compozyconfig.SandboxProfile{Backend: "local"}

	store := newMockWorkspaceStore()
	resolver := newTestResolver(t, store,
		WithHomePaths(homePaths),
		WithConfigLoader((&countingConfigLoader{cfg: cfg}).Load),
		WithIDGenerator(func(string) (string, error) { return "ws_env", nil }),
	)

	root := t.TempDir()
	registered, err := resolver.Register(ctx, RegisterOptions{
		RootDir:    root,
		Name:       "env-workspace",
		SandboxRef: "daytona-dev",
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if got, want := registered.SandboxRef, "daytona-dev"; got != want {
		t.Fatalf("registered SandboxRef = %q, want %q", got, want)
	}

	loaded, err := resolver.Get(ctx, registered.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got, want := loaded.SandboxRef, "daytona-dev"; got != want {
		t.Fatalf("loaded SandboxRef = %q, want %q", got, want)
	}

	nextSandbox := "local-dev"
	if err := resolver.Update(ctx, registered.ID, UpdateOptions{SandboxRef: &nextSandbox}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	updated, err := resolver.Get(ctx, registered.ID)
	if err != nil {
		t.Fatalf("Get(updated) error = %v", err)
	}
	if got, want := updated.SandboxRef, "local-dev"; got != want {
		t.Fatalf("updated SandboxRef = %q, want %q", got, want)
	}
}

func TestResolveWorkspaceLensesComposeUserResourcesAndLocalOverridesIT040(t *testing.T) {
	t.Parallel()
	t.Run("Should compose profile lenses with user resources and local overrides", func(t *testing.T) {
		t.Parallel()
		testResolveWorkspaceLensesComposeUserResourcesAndLocalOverridesIT040(t)
	})
	t.Run("Should allow personal profile resources in the global home workspace", func(t *testing.T) {
		t.Parallel()
		testResolveGlobalHomeWorkspaceProfileResources(t)
	})
}

func testResolveGlobalHomeWorkspaceProfileResources(t *testing.T) {
	t.Helper()

	homePaths := newTestHomePaths(t)
	writeSkill(t, filepath.Join(homePaths.ProfilesDir, "default", compozyconfig.SkillsDirName, "profile-skill"))
	store := newMockWorkspaceStore(Workspace{ID: "ws_global", RootDir: homePaths.HomeDir, Name: "global"})
	loader := &countingConfigLoader{cfg: validConfig(homePaths)}
	resolver := newTestResolver(t, store, WithHomePaths(homePaths), WithConfigLoader(loader.Load))

	resolved, err := resolver.ResolveForProfile(t.Context(), "ws_global", "default")
	if err != nil {
		t.Fatalf("ResolveForProfile(global home workspace) error = %v", err)
	}
	if got, want := skillPathSourceByName(resolved.Skills, "profile-skill"), "profile"; got != want {
		t.Fatalf("global home workspace profile skill source = %q, want %q", got, want)
	}
}

func testResolveWorkspaceLensesComposeUserResourcesAndLocalOverridesIT040(t *testing.T) {
	ctx := context.Background()
	homePaths := newTestHomePaths(t)
	firstRoot := t.TempDir()
	secondRoot := t.TempDir()

	writeAgentDef(t, filepath.Join(homePaths.AgentsDir, "coder", agentDefinitionFile), "coder", "global")
	writeAgentDef(
		t,
		filepath.Join(firstRoot, compozyconfig.DirName, compozyconfig.AgentsDirName, "coder", agentDefinitionFile),
		"coder",
		"local",
	)
	writeAgentDef(t, filepath.Join(homePaths.AgentsDir, "reviewer", agentDefinitionFile), "reviewer", "review")
	writeSkill(t, filepath.Join(homePaths.SkillsDir, "user-skill"))
	writeAgentDef(
		t,
		filepath.Join(homePaths.ProfilesDir, "marketing", compozyconfig.AgentsDirName, "coder", agentDefinitionFile),
		"coder",
		"profile",
	)
	writeAgentDef(
		t,
		filepath.Join(homePaths.ProfilesDir, "marketing", compozyconfig.AgentsDirName, "personal", agentDefinitionFile),
		"personal",
		"profile-only",
	)
	writeAgentDef(
		t,
		filepath.Join(
			firstRoot,
			compozyconfig.DirName,
			compozyconfig.ProfilesDirName,
			"marketing",
			compozyconfig.AgentsDirName,
			"coder",
			agentDefinitionFile,
		),
		"coder",
		"workspace-profile",
	)
	writeSkill(t, filepath.Join(homePaths.ProfilesDir, "marketing", compozyconfig.SkillsDirName, "profile-skill"))
	writeSkill(t, filepath.Join(
		firstRoot,
		compozyconfig.DirName,
		compozyconfig.ProfilesDirName,
		"marketing",
		compozyconfig.SkillsDirName,
		"project-profile-skill",
	))

	store := newMockWorkspaceStore(
		Workspace{ID: "ws_agents", RootDir: firstRoot, Name: "repo"},
		Workspace{ID: "ws_other", RootDir: secondRoot, Name: "other"},
	)
	loader := &countingConfigLoader{cfg: validConfig(homePaths)}
	resolver := newTestResolver(t, store,
		WithHomePaths(homePaths),
		WithConfigLoader(loader.Load),
	)

	first, err := resolver.Resolve(ctx, "ws_agents")
	if err != nil {
		t.Fatalf("Resolve(first workspace) error = %v", err)
	}
	if got := agentModel(first.Agents, "coder"); got != "local" {
		t.Fatalf("coder model = %q, want %q", got, "local")
	}
	if got := agentModel(first.Agents, "reviewer"); got != "review" {
		t.Fatalf("reviewer model = %q, want %q", got, "review")
	}
	canonicalUserSkill, err := filepath.EvalSymlinks(filepath.Join(homePaths.SkillsDir, "user-skill"))
	if err != nil {
		t.Fatalf("EvalSymlinks(user skill) error = %v", err)
	}
	if got, want := first.Skills, []SkillPath{
		{Name: "user-skill", Dir: canonicalUserSkill, Source: string(compozyconfig.WorkspaceDiscoverySourceGlobal)},
	}; !slices.Equal(
		got,
		want,
	) {
		t.Fatalf("first workspace user skills = %#v, want %#v", got, want)
	}

	second, err := resolver.Resolve(ctx, "ws_other")
	if err != nil {
		t.Fatalf("Resolve(second workspace) error = %v", err)
	}
	if got := agentModel(second.Agents, "coder"); got != "global" {
		t.Fatalf("second workspace coder model = %q, want global", got)
	}
	if got := agentModel(second.Agents, "reviewer"); got != "review" {
		t.Fatalf("second workspace reviewer model = %q, want review", got)
	}
	if got, want := second.Skills, first.Skills; !slices.Equal(got, want) {
		t.Fatalf("second workspace user skills = %#v, want %#v", got, want)
	}

	marketing, err := resolver.ResolveForProfile(ctx, "ws_agents", "marketing")
	if err != nil {
		t.Fatalf("ResolveForProfile(marketing) error = %v", err)
	}
	if got, want := marketing.ProfileName, "marketing"; got != want {
		t.Fatalf("resolved profile name = %q, want %q", got, want)
	}
	if got, want := agentModel(marketing.Agents, "coder"), "workspace-profile"; got != want {
		t.Fatalf("marketing coder model = %q, want %q", got, want)
	}
	if got, want := agentModel(marketing.Agents, "personal"), "profile-only"; got != want {
		t.Fatalf("marketing personal model = %q, want %q", got, want)
	}
	if got, want := skillPathSourceByName(marketing.Skills, "profile-skill"), "profile"; got != want {
		t.Fatalf("profile skill source = %q, want %q", got, want)
	}
	if got, want := skillPathSourceByName(marketing.Skills, "project-profile-skill"), "workspace_profile"; got != want {
		t.Fatalf("project profile skill source = %q, want %q", got, want)
	}

	sales, err := resolver.ResolveForProfile(ctx, "ws_agents", "sales")
	if err != nil {
		t.Fatalf("ResolveForProfile(sales) error = %v", err)
	}
	if got, want := agentModel(sales.Agents, "coder"), "local"; got != want {
		t.Fatalf("sales coder model = %q, want %q", got, want)
	}
	if got := agentModel(sales.Agents, "personal"); got != "" {
		t.Fatalf("sales personal model = %q, want absent", got)
	}
	if got := skillPathSourceByName(sales.Skills, "profile-skill"); got != "" {
		t.Fatalf("sales profile skill source = %q, want absent", got)
	}

	t.Run("Should reject a personal profile root inside the workspace", func(t *testing.T) {
		t.Parallel()

		containedHome := homePaths
		containedHome.ProfilesDir = filepath.Join(firstRoot, "personal-profiles")
		containedResolver := newTestResolver(
			t,
			store,
			WithHomePaths(containedHome),
			WithConfigLoader(loader.Load),
		)
		if _, err := containedResolver.ResolveForProfile(ctx, "ws_agents", "marketing"); err == nil {
			t.Fatal("ResolveForProfile(contained personal root) error = nil, want rejection")
		} else if !strings.Contains(err.Error(), "cannot be inside registered workspace") {
			t.Fatalf("ResolveForProfile(contained personal root) error = %v, want containment diagnostic", err)
		}
	})

	t.Run("Should reject a symlinked personal profile root inside the workspace", func(t *testing.T) {
		t.Parallel()

		outsideProfiles := t.TempDir()
		insideProfile := filepath.Join(firstRoot, "physical-profile-root")
		if err := os.MkdirAll(insideProfile, 0o755); err != nil {
			t.Fatalf("MkdirAll(inside profile) error = %v", err)
		}
		profileLink := filepath.Join(outsideProfiles, "marketing")
		if err := os.Symlink(insideProfile, profileLink); err != nil {
			t.Skipf("create profile symlink: %v", err)
		}
		symlinkHome := homePaths
		symlinkHome.ProfilesDir = outsideProfiles
		symlinkResolver := newTestResolver(t, store,
			WithHomePaths(symlinkHome),
			WithConfigLoader(loader.Load),
		)
		if _, err := symlinkResolver.ResolveForProfile(ctx, "ws_agents", "marketing"); err == nil {
			t.Fatal("ResolveForProfile(symlinked contained personal root) error = nil, want rejection")
		} else if !strings.Contains(err.Error(), "cannot be inside registered workspace") {
			t.Fatalf(
				"ResolveForProfile(symlinked contained personal root) error = %v, want containment diagnostic",
				err,
			)
		}
	})
}

func TestWorkspaceProfileDeclarationsRefreshAndBindIT041IT042(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	homePaths := newTestHomePaths(t)
	firstRoot := t.TempDir()
	secondRoot := t.TempDir()
	firstDevAgent := filepath.Join(
		firstRoot, compozyconfig.DirName, compozyconfig.ProfilesDirName, "dev",
		compozyconfig.AgentsDirName, "first-dev", agentDefinitionFile,
	)
	secondDevAgent := filepath.Join(
		secondRoot, compozyconfig.DirName, compozyconfig.ProfilesDirName, "dev",
		compozyconfig.AgentsDirName, "second-dev", agentDefinitionFile,
	)
	writeAgentDef(t, firstDevAgent, "first-dev", "first")
	writeAgentDef(t, secondDevAgent, "second-dev", "second")
	writeSkill(t, filepath.Join(
		firstRoot, compozyconfig.DirName, compozyconfig.ProfilesDirName, "marketing",
		compozyconfig.SkillsDirName, "campaigns",
	))

	store := newMockWorkspaceStore(
		Workspace{ID: "ws_first", RootDir: firstRoot, Name: "first"},
		Workspace{ID: "ws_second", RootDir: secondRoot, Name: "second"},
	)
	resolver := newTestResolver(
		t,
		store,
		WithHomePaths(homePaths),
		WithConfigLoader((&countingConfigLoader{cfg: validConfig(homePaths)}).Load),
	)

	first, err := resolver.Resolve(ctx, "ws_first")
	if err != nil {
		t.Fatalf("Resolve(first declarations) error = %v", err)
	}
	if got, want := declarationNames(
		first.ProfileDeclarations,
	), []string{
		"dev",
		"marketing",
	}; !slices.Equal(
		got,
		want,
	) {
		t.Fatalf("first declarations = %#v, want %#v", got, want)
	}
	firstBound, err := resolver.ResolveForProfile(ctx, "ws_first", "dev")
	if err != nil {
		t.Fatalf("ResolveForProfile(first dev) error = %v", err)
	}
	if got := agentModel(firstBound.Agents, "first-dev"); got != "first" {
		t.Fatalf("first repo dev agent model = %q, want first", got)
	}
	if got := agentModel(firstBound.Agents, "second-dev"); got != "" {
		t.Fatalf("first repo foreign dev agent model = %q, want absent", got)
	}
	secondBound, err := resolver.ResolveForProfile(ctx, "ws_second", "dev")
	if err != nil {
		t.Fatalf("ResolveForProfile(second dev) error = %v", err)
	}
	if got := agentModel(secondBound.Agents, "second-dev"); got != "second" {
		t.Fatalf("second repo dev agent model = %q, want second", got)
	}

	writeAgentDef(t, filepath.Join(
		firstRoot, compozyconfig.DirName, compozyconfig.ProfilesDirName, "dev",
		compozyconfig.AgentsDirName, "fresh", agentDefinitionFile,
	), "fresh", "refreshed")
	refreshed, err := resolver.ResolveForProfile(ctx, "ws_first", "dev")
	if err != nil {
		t.Fatalf("ResolveForProfile(refreshed dev) error = %v", err)
	}
	if got := agentModel(refreshed.Agents, "fresh"); got != "refreshed" {
		t.Fatalf("refreshed profile agent model = %q, want refreshed", got)
	}

	profilesRoot := filepath.Join(firstRoot, compozyconfig.DirName, compozyconfig.ProfilesDirName)
	if err := os.Rename(filepath.Join(profilesRoot, "marketing"), filepath.Join(profilesRoot, "growth")); err != nil {
		t.Fatalf("Rename(marketing profile folder) error = %v", err)
	}
	branchRefreshed, err := resolver.Resolve(ctx, "ws_first")
	if err != nil {
		t.Fatalf("Resolve(branch-refreshed declarations) error = %v", err)
	}
	if got, want := declarationNames(
		branchRefreshed.ProfileDeclarations,
	), []string{
		"dev",
		"growth",
	}; !slices.Equal(
		got,
		want,
	) {
		t.Fatalf("branch-refreshed declarations = %#v, want %#v", got, want)
	}
}

func declarationNames(values []ProfileDeclaration) []string {
	names := make([]string, 0, len(values))
	for _, value := range values {
		names = append(names, value.Name)
	}
	return names
}

func TestResolveRecordsMalformedAgentDiagnostics(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	homePaths := newTestHomePaths(t)
	root := t.TempDir()

	writeAgentDef(
		t,
		filepath.Join(root, compozyconfig.DirName, compozyconfig.AgentsDirName, "healthy", agentDefinitionFile),
		"healthy",
		"local",
	)
	writeFile(
		t,
		filepath.Join(root, compozyconfig.DirName, compozyconfig.AgentsDirName, "no-fence", agentDefinitionFile),
		"plain body",
	)
	writeFile(
		t,
		filepath.Join(root, compozyconfig.DirName, compozyconfig.AgentsDirName, "unterminated", agentDefinitionFile),
		"---\nname: broken",
	)
	writeFile(
		t,
		filepath.Join(root, compozyconfig.DirName, compozyconfig.AgentsDirName, "bom", agentDefinitionFile),
		"\ufeff---\nname: broken\n---\nPrompt.",
	)
	writeFile(
		t,
		filepath.Join(root, compozyconfig.DirName, compozyconfig.AgentsDirName, "embedded-tab", agentDefinitionFile),
		"---\nna\tme: broken\n---\nPrompt.",
	)
	store := newMockWorkspaceStore(Workspace{ID: "ws_agents", RootDir: root, Name: "repo"})
	loader := &countingConfigLoader{cfg: validConfig(homePaths)}
	resolver := newTestResolver(t, store,
		WithHomePaths(homePaths),
		WithConfigLoader(loader.Load),
	)

	resolved, err := resolver.Resolve(ctx, "ws_agents")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got := agentModel(resolved.Agents, "healthy"); got != "local" {
		t.Fatalf("healthy model = %q, want local", got)
	}
	if got, want := len(resolved.AgentDiagnostics), 4; got != want {
		t.Fatalf("len(AgentDiagnostics) = %d, want %d: %#v", got, want, resolved.AgentDiagnostics)
	}
	kinds := make(map[string]string, len(resolved.AgentDiagnostics))
	for _, diagnostic := range resolved.AgentDiagnostics {
		kinds[diagnostic.Name] = diagnostic.ErrorKind
		if diagnostic.Path == "" || diagnostic.Message == "" {
			t.Fatalf("diagnostic = %#v, want path and message", diagnostic)
		}
	}
	wantKinds := map[string]string{
		"no-fence":     "frontmatter.missing",
		"unterminated": "frontmatter.unterminated",
		"bom":          "frontmatter.bom",
		"embedded-tab": "frontmatter.invalid_key",
	}
	if !maps.Equal(kinds, wantKinds) {
		t.Fatalf("diagnostic kinds = %#v, want %#v", kinds, wantKinds)
	}
}

func TestResolveRejectsReservedAgentIdentities(t *testing.T) {
	t.Parallel()
	t.Run("Should skip reserved identities and emit named diagnostics", func(t *testing.T) {
		t.Parallel()

		homePaths := newTestHomePaths(t)
		root := t.TempDir()
		writeAgentDef(
			t,
			filepath.Join(
				root,
				compozyconfig.DirName,
				compozyconfig.AgentsDirName,
				compozyconfig.BuiltinCoordinatorAgentName,
				agentDefinitionFile,
			),
			compozyconfig.BuiltinCoordinatorAgentName,
			"shadowed-coordinator",
		)
		writeAgentDef(
			t,
			filepath.Join(
				homePaths.AgentsDir,
				compozyconfig.BuiltinDreamingCuratorAgentName,
				agentDefinitionFile,
			),
			compozyconfig.BuiltinDreamingCuratorAgentName,
			"shadowed-curator",
		)
		writeAgentDef(
			t,
			filepath.Join(root, compozyconfig.DirName, compozyconfig.AgentsDirName, "alias", agentDefinitionFile),
			compozyconfig.BuiltinCoordinatorAgentName,
			"aliased-coordinator",
		)

		store := newMockWorkspaceStore(Workspace{ID: "ws_reserved_agents", RootDir: root, Name: "repo"})
		loader := &countingConfigLoader{cfg: validConfig(homePaths)}
		resolver := newTestResolver(t, store, WithHomePaths(homePaths), WithConfigLoader(loader.Load))
		resolved, err := resolver.Resolve(context.Background(), "ws_reserved_agents")
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		if got := agentModel(resolved.Agents, compozyconfig.BuiltinCoordinatorAgentName); got != "" {
			t.Fatalf("reserved coordinator model = %q, want excluded", got)
		}
		if got := agentModel(resolved.Agents, compozyconfig.BuiltinDreamingCuratorAgentName); got != "" {
			t.Fatalf("reserved dreaming-curator model = %q, want excluded", got)
		}
		if got, want := len(resolved.AgentDiagnostics), 3; got != want {
			t.Fatalf("len(AgentDiagnostics) = %d, want %d: %#v", got, want, resolved.AgentDiagnostics)
		}
		names := make([]string, 0, len(resolved.AgentDiagnostics))
		for _, diagnostic := range resolved.AgentDiagnostics {
			if diagnostic.ErrorKind != reservedAgentDiagnosticKind {
				t.Fatalf("reserved agent diagnostic = %#v, want %q", diagnostic, reservedAgentDiagnosticKind)
			}
			names = append(names, diagnostic.Name)
		}
		slices.Sort(names)
		wantNames := []string{
			compozyconfig.BuiltinCoordinatorAgentName,
			compozyconfig.BuiltinCoordinatorAgentName,
			compozyconfig.BuiltinDreamingCuratorAgentName,
		}
		if !slices.Equal(names, wantNames) {
			t.Fatalf("diagnostic names = %#v, want %#v", names, wantNames)
		}
	})
}

func TestResolveConfigFromRootOnly(t *testing.T) {
	ctx := context.Background()
	homePaths := newTestHomePaths(t)
	root := t.TempDir()
	additional := t.TempDir()
	t.Setenv("COMPOZY_HOME", homePaths.HomeDir)

	writeFile(t, homePaths.ConfigFile, "[http]\nhost = \"localhost\"\nport = 2123\n")
	writeFile(t, filepath.Join(root, compozyconfig.DirName, compozyconfig.ConfigName), "[http]\nport = 4242\n")
	writeFile(t, filepath.Join(additional, compozyconfig.DirName, compozyconfig.ConfigName), "[http]\nport = 9999\n")

	store := newMockWorkspaceStore(Workspace{
		ID:             "ws_config",
		RootDir:        root,
		AdditionalDirs: []string{additional},
		Name:           "repo",
	})
	resolver := newTestResolver(t, store,
		WithHomePaths(homePaths),
	)

	resolved, err := resolver.Resolve(ctx, "ws_config")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got, want := resolved.Config.HTTP.Port, 4242; got != want {
		t.Fatalf("Resolve() HTTP.Port = %d, want %d", got, want)
	}
}

func TestCloneConfigProducesDeepCopy(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve provider model reasoning and curation metadata", func(t *testing.T) {
		t.Parallel()

		deprecated := false
		hidden := true
		featured := false
		original := compozyconfig.Config{
			Providers: map[string]compozyconfig.ProviderConfig{
				"claude": {
					Models: compozyconfig.ProviderModelsConfig{
						Reasoning: compozyconfig.ProviderReasoningConfig{
							Apply: compozyconfig.ReasoningApplyNone,
						},
						Curated: []compozyconfig.ProviderModelConfig{{
							ID:          "claude-sonnet-5",
							Deprecated:  &deprecated,
							Hidden:      &hidden,
							Featured:    &featured,
							ReleaseDate: "2026-07-10",
						}},
					},
				},
			},
		}

		cloned := cloneConfig(&original)
		provider := cloned.Providers["claude"]
		if got, want := provider.Models.Reasoning.Apply, compozyconfig.ReasoningApplyNone; got != want {
			t.Fatalf("cloned reasoning apply = %q, want %q", got, want)
		}
		model := provider.Models.Curated[0]
		if model.Deprecated == nil || *model.Deprecated || model.Hidden == nil || !*model.Hidden ||
			model.Featured == nil || *model.Featured || model.ReleaseDate != "2026-07-10" {
			t.Fatalf("cloned provider model metadata = %#v", model)
		}

		*model.Deprecated = true
		*model.Hidden = false
		*model.Featured = true
		originalModel := original.Providers["claude"].Models.Curated[0]
		if *originalModel.Deprecated || !*originalModel.Hidden || *originalModel.Featured {
			t.Fatalf("original provider model pointers changed through clone: %#v", originalModel)
		}
	})

	t.Run("Should produce an independent deep copy", func(t *testing.T) {
		t.Parallel()

		toolReadOnly := true
		original := compozyconfig.Config{
			Agents: compozyconfig.AgentsConfig{
				Soul: compozyconfig.SoulConfig{
					Enabled:                true,
					MaxBodyBytes:           32768,
					ContextProjectionBytes: 2048,
				},
				Heartbeat: compozyconfig.HeartbeatConfig{
					Enabled:                      true,
					MaxBodyBytes:                 32768,
					ContextProjectionBytes:       4096,
					MinInterval:                  5 * time.Minute,
					DefaultInterval:              30 * time.Minute,
					WakeCooldown:                 time.Minute,
					MaxWakesPerCycle:             25,
					ActiveSessionOnly:            true,
					AllowActiveHoursPreferences:  true,
					WakeEventRetention:           168 * time.Hour,
					SessionHealthStaleAfter:      2 * time.Minute,
					SessionHealthHookMinInterval: time.Minute,
				},
			},
			Session: compozyconfig.SessionConfig{
				Limits: compozyconfig.SessionLimitsConfig{
					Timeout: time.Minute,
				},
			},
			Roles: compozyconfig.RolesConfig{
				Coordinator: compozyconfig.CoordinatorRoleConfig{
					RoleConfig: compozyconfig.RoleConfig{
						Enabled:       true,
						Agent:         "coordinator",
						Provider:      "codex",
						Model:         "gpt-4o",
						FallbackChain: []compozyconfig.RoleFallback{{Provider: "claude", Model: "sonnet"}},
					},
					TTL:                           45 * time.Minute,
					MaxChildren:                   5,
					MaxActiveSessionsPerWorkspace: 5,
				},
			},
			RoleSources: compozyconfig.RoleFieldSources{
				compozyconfig.RoleCoordinator: {"model": compozyconfig.RoleFieldSourceDefault},
			},
			Providers: map[string]compozyconfig.ProviderConfig{
				"claude": {
					Command: "claude",
					Models: compozyconfig.ProviderModelsConfig{
						Default: "sonnet",
					},
					CredentialSlots: []compozyconfig.ProviderCredentialSlot{
						{
							Name:      "api_key",
							TargetEnv: "ANTHROPIC_API_KEY",
							SecretRef: "env:ANTHROPIC_API_KEY",
							Kind:      "api_key",
							Required:  true,
						},
					},
					MCPServers: []compozyconfig.MCPServer{
						{
							Name:    "github",
							Command: "npx",
							Args:    []string{"-y"},
							Env:     map[string]string{"TOKEN": "one"},
						},
					},
				},
			},
			Skills: compozyconfig.SkillsConfig{
				Enabled:        true,
				DisabledSkills: []string{"alpha"},
				PollInterval:   time.Second,
			},
			Hooks: compozyconfig.HooksConfig{
				Declarations: []hookspkg.HookDecl{{
					Name: "test-hook",
					Args: []string{"one"},
					Env:  map[string]string{"TOKEN": "one"},
					Metadata: map[string]string{
						"origin": "test",
					},
					Matcher: hookspkg.HookMatcher{
						ToolReadOnly: &toolReadOnly,
					},
				}},
			},
		}

		cloned := cloneConfig(&original)
		if !cloned.Roles.Coordinator.Enabled || cloned.Roles.Coordinator.Agent != "coordinator" ||
			cloned.Roles.Coordinator.TTL != 45*time.Minute ||
			!slices.Equal(cloned.Roles.Coordinator.FallbackChain, original.Roles.Coordinator.FallbackChain) {
			t.Fatalf("cloned Roles.Coordinator = %#v, want preserved role config", cloned.Roles.Coordinator)
		}
		if got := cloned.RoleSources[compozyconfig.RoleCoordinator]["model"]; got != compozyconfig.RoleFieldSourceDefault {
			t.Fatalf("cloned RoleSources = %#v, want preserved coordinator model source", cloned.RoleSources)
		}
		cloned.Agents.Soul.MaxBodyBytes = 8192
		cloned.Agents.Heartbeat.MaxBodyBytes = 8192
		cloned.Session.Limits.Timeout = 2 * time.Minute
		cloned.Roles.Coordinator.Enabled = false
		cloned.Roles.Coordinator.Agent = "mutated-coordinator"
		cloned.Roles.Coordinator.TTL = 2 * time.Hour
		cloned.Roles.Coordinator.FallbackChain[0].Model = "mutated-model"
		cloned.RoleSources[compozyconfig.RoleCoordinator]["model"] = "overlay"
		cloned.Providers["claude"] = compozyconfig.ProviderConfig{}
		cloned.Skills.DisabledSkills[0] = "beta"
		cloned.Hooks.Declarations[0].Args[0] = "two"
		cloned.Hooks.Declarations[0].Env["TOKEN"] = "two"
		cloned.Hooks.Declarations[0].Metadata["origin"] = "mutated"
		*cloned.Hooks.Declarations[0].Matcher.ToolReadOnly = false

		if got, want := original.Agents.Soul.MaxBodyBytes, int64(32768); got != want {
			t.Fatalf("original Agents.Soul.MaxBodyBytes = %d, want %d", got, want)
		}
		if got, want := original.Agents.Heartbeat.MaxBodyBytes, int64(32768); got != want {
			t.Fatalf("original Agents.Heartbeat.MaxBodyBytes = %d, want %d", got, want)
		}
		if got, want := original.Session.Limits.Timeout, time.Minute; got != want {
			t.Fatalf("original Session.Limits.Timeout = %s, want %s", got, want)
		}
		if got, want := original.Roles.Coordinator.TTL, 45*time.Minute; got != want {
			t.Fatalf("original Roles.Coordinator.TTL = %s, want %s", got, want)
		}
		if !original.Roles.Coordinator.Enabled {
			t.Fatal("original Roles.Coordinator.Enabled = false, want true")
		}
		if got, want := original.Roles.Coordinator.Agent, "coordinator"; got != want {
			t.Fatalf("original Roles.Coordinator.Agent = %q, want %q", got, want)
		}
		if got, want := original.Roles.Coordinator.FallbackChain[0].Model, "sonnet"; got != want {
			t.Fatalf("original.Roles.Coordinator.FallbackChain[0].Model = %q, want %q", got, want)
		}
		if got := original.RoleSources[compozyconfig.RoleCoordinator]["model"]; got != compozyconfig.RoleFieldSourceDefault {
			t.Fatalf("original RoleSources mutated through clone: %#v", original.RoleSources)
		}
		provider := original.Providers["claude"]
		if provider.Command != "claude" || provider.MCPServers[0].Env["TOKEN"] != "one" {
			t.Fatalf("original provider mutated: %#v", provider)
		}
		if got, want := original.Skills.DisabledSkills, []string{"alpha"}; !slices.Equal(got, want) {
			t.Fatalf("original Skills.DisabledSkills = %#v, want %#v", got, want)
		}
		hook := original.Hooks.Declarations[0]
		if hook.Args[0] != "one" || hook.Env["TOKEN"] != "one" ||
			hook.Metadata["origin"] != "test" || hook.Matcher.ToolReadOnly == nil ||
			!*hook.Matcher.ToolReadOnly {
			t.Fatalf("original hook mutated: %#v", hook)
		}
	})
}
