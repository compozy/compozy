//go:build integration

package workspace_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/sandbox"
	storepkg "github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	compozyworkspace "github.com/compozy/compozy/internal/workspace"
)

func TestResolverIntegrationRegisterResolveAndMergeResources(t *testing.T) {
	ctx := context.Background()
	homePaths := newIntegrationHomePaths(t)
	t.Setenv("COMPOZY_HOME", homePaths.HomeDir)

	db := openTestGlobalDB(t, ctx)
	defer closeTestGlobalDB(t, ctx, db)

	root := t.TempDir()
	additional := t.TempDir()

	writeFile(t, homePaths.ConfigFile, "[http]\nhost = \"localhost\"\nport = 2123\n")
	writeFile(t, rootConfigPath(root), "[http]\nport = 4242\n")
	writeFile(t, rootConfigPath(additional), "[http]\nport = 9999\n")

	writeAgentDef(t, agentFilePath(homePaths.AgentsDir, "coder"), "coder", "global")
	writeAgentDef(t, agentFilePath(homePaths.AgentsDir, "reviewer"), "reviewer", "global-review")
	writeAgentDef(t, agentFilePath(agentDir(root), "coder"), "coder", "workspace")
	writeAgentDef(t, agentFilePath(agentDir(additional), "ops"), "ops", "additional")

	writeSkill(t, skillDir(homePaths.SkillsDir, "shared"))
	writeSkill(t, skillDir(homePaths.SkillsDir, "global-only"))
	writeSkill(t, skillDir(skillRoot(root), "shared"))
	emptyGroup := skillDir(skillRoot(root), "empty-group")
	mkdirAll(t, emptyGroup)
	marketingSkill := skillDir(skillRoot(root), "marketing/seo-a")
	salesSkill := skillDir(skillRoot(root), "sales/seo-b")
	shadowedSkill := skillDir(skillRoot(root), "engineering/seo-a")
	writeSkill(t, marketingSkill)
	writeSkill(t, salesSkill)
	writeSkill(t, shadowedSkill)
	writeSkill(t, skillDir(skillRoot(additional), "ops-skill"))

	resolver := newIntegrationResolver(t, db, homePaths)

	registered, err := resolver.Register(ctx, compozyworkspace.RegisterOptions{
		RootDir:        root,
		Name:           "repo",
		AdditionalDirs: []string{additional},
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	resolved, err := resolver.Resolve(ctx, registered.ID)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if got, want := resolved.Config.HTTP.Port, 4242; got != want {
		t.Fatalf("Resolve() HTTP.Port = %d, want %d", got, want)
	}
	if got := agentModel(resolved.Agents, "coder"); got != "workspace" {
		t.Fatalf("coder model = %q, want %q", got, "workspace")
	}
	if got := agentModel(resolved.Agents, "ops"); got != "additional" {
		t.Fatalf("ops model = %q, want %q", got, "additional")
	}
	if got := agentModel(resolved.Agents, "reviewer"); got != "global-review" {
		t.Fatalf("reviewer model = %q, want %q", got, "global-review")
	}

	if got := skillSourceByName(resolved.Skills, "shared"); got != "workspace" {
		t.Fatalf("shared skill source = %q, want %q", got, "workspace")
	}
	if got := skillSourceByName(resolved.Skills, "ops-skill"); got != "additional" {
		t.Fatalf("ops-skill source = %q, want %q", got, "additional")
	}
	if got := skillSourceByName(resolved.Skills, "global-only"); got != "global" {
		t.Fatalf("global-only source = %q, want %q", got, "global")
	}
	t.Run("Should project nested skill leaves without treating groups as skills", func(t *testing.T) {
		canonicalMarketingSkill, err := filepath.EvalSymlinks(marketingSkill)
		if err != nil {
			t.Fatalf("EvalSymlinks(marketing skill) error = %v", err)
		}
		canonicalSalesSkill, err := filepath.EvalSymlinks(salesSkill)
		if err != nil {
			t.Fatalf("EvalSymlinks(sales skill) error = %v", err)
		}
		canonicalMarketingGroup, err := filepath.EvalSymlinks(skillDir(skillRoot(root), "marketing"))
		if err != nil {
			t.Fatalf("EvalSymlinks(marketing group) error = %v", err)
		}
		canonicalEmptyGroup, err := filepath.EvalSymlinks(emptyGroup)
		if err != nil {
			t.Fatalf("EvalSymlinks(empty group) error = %v", err)
		}
		if got, want := len(resolved.Skills), 5; got != want {
			t.Fatalf("resolved skill count = %d, want %d", got, want)
		}
		if got := skillSourceByDir(resolved.Skills, canonicalMarketingSkill); got != "workspace" {
			t.Fatalf("marketing skill source = %q, want %q", got, "workspace")
		}
		if got := skillSourceByDir(resolved.Skills, canonicalSalesSkill); got != "workspace" {
			t.Fatalf("sales skill source = %q, want %q", got, "workspace")
		}
		canonicalShadowedSkill, err := filepath.EvalSymlinks(shadowedSkill)
		if err != nil {
			t.Fatalf("EvalSymlinks(shadowed skill) error = %v", err)
		}
		if got := skillSourceByDir(resolved.Skills, canonicalShadowedSkill); got != "" {
			t.Fatalf("shadowed skill source = %q, want empty", got)
		}
		if got := skillSourceByName(resolved.Skills, "seo-a"); got != "workspace" {
			t.Fatalf("seo-a skill source = %q, want %q", got, "workspace")
		}
		if got := skillSourceByDir(resolved.Skills, canonicalMarketingGroup); got != "" {
			t.Fatalf("marketing group source = %q, want empty", got)
		}
		if got := skillSourceByDir(resolved.Skills, canonicalEmptyGroup); got != "" {
			t.Fatalf("empty group source = %q, want empty", got)
		}
	})
}

func TestResolverIntegrationResolveUpdatesStaleSymlinkRegistration(t *testing.T) {
	ctx := context.Background()
	homePaths := newIntegrationHomePaths(t)
	t.Setenv("COMPOZY_HOME", homePaths.HomeDir)

	db := openTestGlobalDB(t, ctx)
	defer closeTestGlobalDB(t, ctx, db)

	parent := t.TempDir()
	targetOne := filepath.Join(parent, "target-one")
	targetTwo := filepath.Join(parent, "target-two")
	link := filepath.Join(parent, "workspace-link")

	mkdirAll(t, targetOne)
	mkdirAll(t, targetTwo)
	writeFile(t, rootConfigPath(targetOne), "[http]\nport = 3001\n")
	writeFile(t, rootConfigPath(targetTwo), "[http]\nport = 3002\n")
	createSymlink(t, targetOne, link)

	now := time.Now().UTC()
	if err := db.InsertWorkspace(ctx, compozyworkspace.Workspace{
		ID:        "ws_symlink",
		RootDir:   link,
		Name:      "symlinked",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("InsertWorkspace() error = %v", err)
	}

	createSymlink(t, targetTwo, link)

	resolver := newIntegrationResolver(t, db, homePaths)

	resolved, err := resolver.Resolve(ctx, "ws_symlink")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	canonicalTargetTwo, err := filepath.EvalSymlinks(targetTwo)
	if err != nil {
		t.Fatalf("EvalSymlinks(targetTwo) error = %v", err)
	}
	if resolved.RootDir != canonicalTargetTwo {
		t.Fatalf("Resolve() RootDir = %q, want %q", resolved.RootDir, canonicalTargetTwo)
	}
	if got, want := resolved.Config.HTTP.Port, 3002; got != want {
		t.Fatalf("Resolve() HTTP.Port = %d, want %d", got, want)
	}

	stored, err := db.GetWorkspace(ctx, "ws_symlink")
	if err != nil {
		t.Fatalf("GetWorkspace() error = %v", err)
	}
	if stored.RootDir != canonicalTargetTwo {
		t.Fatalf("stored RootDir = %q, want %q", stored.RootDir, canonicalTargetTwo)
	}
}

func TestResolverIntegrationListPrunesMissingWorkspaceAcrossReopen(t *testing.T) {
	t.Run(
		"Should persist pruning across reopen without deleting healthy workspace data",
		resolverIntegrationListPrunesMissingWorkspaceAcrossReopen,
	)
}

func resolverIntegrationListPrunesMissingWorkspaceAcrossReopen(t *testing.T) {
	t.Helper()

	ctx := context.Background()
	homePaths := newIntegrationHomePaths(t)
	databasePath := filepath.Join(t.TempDir(), "compozy.db")
	db, err := globaldb.OpenGlobalDB(ctx, databasePath)
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	dbClosed := false
	t.Cleanup(func() {
		if dbClosed {
			return
		}
		if err := db.Close(ctx); err != nil {
			t.Errorf("GlobalDB.Close() cleanup error = %v", err)
		}
	})

	healthyRoot := t.TempDir()
	missingRoot := filepath.Join(t.TempDir(), "removed-workspace")
	if err := os.Mkdir(missingRoot, 0o755); err != nil {
		t.Fatalf("Mkdir(%q) error = %v", missingRoot, err)
	}
	now := time.Now().UTC()
	for _, workspace := range []compozyworkspace.Workspace{
		{ID: "ws_reopen_healthy", RootDir: healthyRoot, Name: "healthy", CreatedAt: now, UpdatedAt: now},
		{ID: "ws_reopen_missing", RootDir: missingRoot, Name: "missing", CreatedAt: now, UpdatedAt: now},
	} {
		if err := db.InsertWorkspace(ctx, workspace); err != nil {
			t.Fatalf("InsertWorkspace(%q) error = %v", workspace.ID, err)
		}
	}
	for _, session := range []storepkg.SessionInfo{
		{
			ID:            "sess_reopen_healthy",
			AgentName:     "coder",
			WorkspaceID:   "ws_reopen_healthy",
			State:         "stopped",
			RuntimeStatus: storepkg.SessionRuntimeUnbound,
			CreatedAt:     now,
			UpdatedAt:     now,
		},
		{
			ID:            "sess_reopen_missing",
			AgentName:     "coder",
			WorkspaceID:   "ws_reopen_missing",
			State:         "stopped",
			RuntimeStatus: storepkg.SessionRuntimeUnbound,
			CreatedAt:     now,
			UpdatedAt:     now,
		},
	} {
		if err := db.RegisterSession(ctx, session); err != nil {
			t.Fatalf("RegisterSession(%q) error = %v", session.ID, err)
		}
	}
	if err := os.Remove(missingRoot); err != nil {
		t.Fatalf("Remove(%q) error = %v", missingRoot, err)
	}

	resolver := newIntegrationResolver(t, db, homePaths)
	listed, err := resolver.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(listed) != 1 || listed[0].ID != "ws_reopen_healthy" {
		t.Fatalf("List() = %#v, want only healthy workspace", listed)
	}

	if err := db.Close(ctx); err != nil {
		t.Fatalf("GlobalDB.Close() error = %v", err)
	}
	dbClosed = true
	reopened, err := globaldb.OpenGlobalDB(ctx, databasePath)
	if err != nil {
		t.Fatalf("OpenGlobalDB(reopen) error = %v", err)
	}
	t.Cleanup(func() {
		if err := reopened.Close(ctx); err != nil {
			t.Errorf("GlobalDB.Close(reopened) cleanup error = %v", err)
		}
	})

	restartedResolver := newIntegrationResolver(t, reopened, homePaths)
	listed, err = restartedResolver.List(ctx)
	if err != nil {
		t.Fatalf("List(after reopen) error = %v", err)
	}
	if len(listed) != 1 || listed[0].ID != "ws_reopen_healthy" {
		t.Fatalf("List(after reopen) = %#v, want only healthy workspace", listed)
	}
	if _, err := reopened.GetWorkspace(
		ctx,
		"ws_reopen_missing",
	); !errors.Is(
		err,
		compozyworkspace.ErrWorkspaceNotFound,
	) {
		t.Fatalf("GetWorkspace(pruned after reopen) error = %v, want %v", err, compozyworkspace.ErrWorkspaceNotFound)
	}
	healthySessions, err := reopened.ListSessions(ctx, storepkg.SessionListQuery{WorkspaceID: "ws_reopen_healthy"})
	if err != nil {
		t.Fatalf("ListSessions(healthy after reopen) error = %v", err)
	}
	if len(healthySessions) != 1 || healthySessions[0].ID != "sess_reopen_healthy" {
		t.Fatalf("ListSessions(healthy after reopen) = %#v, want healthy session", healthySessions)
	}
	missingSessions, err := reopened.ListSessions(ctx, storepkg.SessionListQuery{WorkspaceID: "ws_reopen_missing"})
	if err != nil {
		t.Fatalf("ListSessions(missing after reopen) error = %v", err)
	}
	if len(missingSessions) != 0 {
		t.Fatalf("ListSessions(missing after reopen) = %#v, want no sessions", missingSessions)
	}
}

func TestResolverIntegrationSandboxConfigRoundTrip(t *testing.T) {
	ctx := context.Background()
	homePaths := newIntegrationHomePaths(t)
	t.Setenv("COMPOZY_HOME", homePaths.HomeDir)

	db := openTestGlobalDB(t, ctx)
	defer closeTestGlobalDB(t, ctx, db)

	root := t.TempDir()
	writeFile(t, homePaths.ConfigFile, `
[defaults]
sandbox = "daytona-dev"

[sandboxes.daytona-dev]
backend = "daytona"
sync_mode = "session-bidirectional"
persistence = "reuse"
runtime_root = "/home/daytona/workspace"

[sandboxes.daytona-dev.env]
NODE_ENV = "development"

[sandboxes.daytona-dev.daytona]
image = "ubuntu:24.04"
snapshot = "snap-integration"
`)

	resolver := newIntegrationResolver(t, db, homePaths)
	registered, err := resolver.Register(ctx, compozyworkspace.RegisterOptions{
		RootDir:    root,
		Name:       "repo-env",
		SandboxRef: "daytona-dev",
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	stored, err := db.GetWorkspace(ctx, registered.ID)
	if err != nil {
		t.Fatalf("GetWorkspace() error = %v", err)
	}
	if got, want := stored.SandboxRef, "daytona-dev"; got != want {
		t.Fatalf("stored SandboxRef = %q, want %q", got, want)
	}

	resolved, err := resolver.Resolve(ctx, registered.ID)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved.Sandbox.Profile != "daytona-dev" ||
		resolved.Sandbox.Backend != sandbox.BackendDaytona ||
		resolved.Sandbox.Persistence != sandbox.PersistenceReuse {
		t.Fatalf("resolved Sandbox = %#v, want Daytona profile", resolved.Sandbox)
	}
	if resolved.Sandbox.Daytona == nil ||
		resolved.Sandbox.Daytona.StartupSource != sandbox.DaytonaStartupSourceSnapshot ||
		resolved.Sandbox.Daytona.StartupRef != "snap-integration" {
		t.Fatalf("resolved Daytona config = %#v, want snapshot startup", resolved.Sandbox.Daytona)
	}
	if got, want := resolved.Sandbox.Env["NODE_ENV"], "development"; got != want {
		t.Fatalf("resolved Env[NODE_ENV] = %q, want %q", got, want)
	}
}

func newIntegrationResolver(
	t *testing.T,
	store compozyworkspace.Store,
	homePaths compozyconfig.HomePaths,
) *compozyworkspace.Resolver {
	t.Helper()

	resolver, err := compozyworkspace.NewResolver(store,
		compozyworkspace.WithHomePaths(homePaths),
		compozyworkspace.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
	)
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}
	return resolver
}

func newIntegrationHomePaths(t *testing.T) compozyconfig.HomePaths {
	t.Helper()

	homePaths, err := compozyconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := compozyconfig.EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}
	return homePaths
}

func openTestGlobalDB(t *testing.T, ctx context.Context) *globaldb.GlobalDB {
	t.Helper()

	globalDB, err := globaldb.OpenGlobalDB(ctx, filepath.Join(t.TempDir(), "compozy.db"))
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	return globalDB
}

func closeTestGlobalDB(t *testing.T, ctx context.Context, globalDB *globaldb.GlobalDB) {
	t.Helper()

	if err := globalDB.Close(ctx); err != nil {
		t.Fatalf("GlobalDB.Close() error = %v", err)
	}
}

func writeAgentDef(t *testing.T, path string, name string, model string) {
	t.Helper()
	writeFile(t, path, strings.Join([]string{
		"---",
		"name: " + name,
		"provider: claude",
		"model: " + model,
		"---",
		"",
		"Prompt for " + name + ".",
		"",
	}, "\n"))
}

func writeSkill(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, "SKILL.md"), strings.Join([]string{
		"---",
		"name: " + filepath.Base(dir),
		"description: test skill",
		"---",
		"",
		"Skill body.",
		"",
	}, "\n"))
}

func writeFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func createSymlink(t *testing.T, target string, link string) {
	t.Helper()
	if err := os.Remove(link); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Remove(%q) error = %v", link, err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink(%q -> %q) error = %v", link, target, err)
	}
}

func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
}

func rootConfigPath(root string) string {
	return filepath.Join(root, compozyconfig.DirName, compozyconfig.ConfigName)
}

func agentDir(root string) string {
	return filepath.Join(root, compozyconfig.DirName, compozyconfig.AgentsDirName)
}

func skillRoot(root string) string {
	return filepath.Join(root, compozyconfig.DirName, compozyconfig.SkillsDirName)
}

func agentFilePath(root string, name string) string {
	return filepath.Join(root, name, "AGENT.md")
}

func skillDir(root string, name string) string {
	return filepath.Join(root, name)
}

func agentModel(agents []compozyconfig.AgentDef, name string) string {
	for _, agent := range agents {
		if agent.Name == name {
			return agent.Model
		}
	}
	return ""
}

func skillSourceByName(skills []compozyworkspace.SkillPath, name string) string {
	for _, skill := range skills {
		skillName := skill.Name
		if skillName == "" {
			skillName = filepath.Base(skill.Dir)
		}
		if skillName == name {
			return skill.Source
		}
	}
	return ""
}

func skillSourceByDir(skills []compozyworkspace.SkillPath, dir string) string {
	for _, skill := range skills {
		if filepath.Clean(skill.Dir) == filepath.Clean(dir) {
			return skill.Source
		}
	}
	return ""
}
