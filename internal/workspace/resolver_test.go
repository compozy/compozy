package workspace

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/filesnap"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/sandbox"
)

func TestResolveRoutesByIdentifierType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       func(root string, ws Workspace) string
		assertCalls func(t *testing.T, store *mockWorkspaceStore)
	}{
		{
			name: "id",
			input: func(_ string, ws Workspace) string {
				return ws.ID
			},
			assertCalls: func(t *testing.T, store *mockWorkspaceStore) {
				t.Helper()
				if got := len(store.getWorkspaceCalls); got != 1 {
					t.Fatalf("GetWorkspace() calls = %d, want 1", got)
				}
				if got := len(store.getByNameCalls); got != 0 {
					t.Fatalf("GetWorkspaceByName() calls = %d, want 0", got)
				}
				if got := len(store.getByPathCalls); got != 0 {
					t.Fatalf("GetWorkspaceByPath() calls = %d, want 0", got)
				}
			},
		},
		{
			name: "name",
			input: func(_ string, ws Workspace) string {
				return ws.Name
			},
			assertCalls: func(t *testing.T, store *mockWorkspaceStore) {
				t.Helper()
				if got := len(store.getWorkspaceCalls); got != 0 {
					t.Fatalf("GetWorkspace() calls = %d, want 0", got)
				}
				if got := len(store.getByNameCalls); got != 1 {
					t.Fatalf("GetWorkspaceByName() calls = %d, want 1", got)
				}
				if got := len(store.getByPathCalls); got != 0 {
					t.Fatalf("GetWorkspaceByPath() calls = %d, want 0", got)
				}
			},
		},
		{
			name: "absolute path",
			input: func(root string, _ Workspace) string {
				return root
			},
			assertCalls: func(t *testing.T, store *mockWorkspaceStore) {
				t.Helper()
				if got := len(store.getWorkspaceCalls); got != 0 {
					t.Fatalf("GetWorkspace() calls = %d, want 0", got)
				}
				if got := len(store.getByNameCalls); got != 0 {
					t.Fatalf("GetWorkspaceByName() calls = %d, want 0", got)
				}
				if got := len(store.getByPathCalls); got != 1 {
					t.Fatalf("GetWorkspaceByPath() calls = %d, want 1", got)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			homePaths := newTestHomePaths(t)
			root := t.TempDir()
			ws := Workspace{ID: "ws_route", RootDir: mustCanonicalRoot(t, root), Name: "repo"}

			store := newMockWorkspaceStore(ws)
			loader := &countingConfigLoader{cfg: validConfig(homePaths)}
			resolver := newTestResolver(t, store,
				WithHomePaths(homePaths),
				WithConfigLoader(loader.Load),
			)

			resolved, err := resolver.Resolve(ctx, tt.input(root, ws))
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if resolved.ID != ws.ID {
				t.Fatalf("Resolve() ID = %q, want %q", resolved.ID, ws.ID)
			}

			tt.assertCalls(t, store)
		})
	}
}

func TestResolveMatchesWorkspaceBySameFilesystemRoot(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve a registered alternate path to the same root", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := newTestHomePaths(t)
		root := t.TempDir()
		alternateRoot := symlinkWorkspaceRootForTest(t, root)
		ws := Workspace{ID: "ws_same_root", RootDir: alternateRoot, Name: "repo"}
		store := newMockWorkspaceStore(ws)
		loader := &countingConfigLoader{cfg: validConfig(homePaths)}
		resolver := newTestResolver(t, store,
			WithHomePaths(homePaths),
			WithConfigLoader(loader.Load),
		)

		resolved, err := resolver.Resolve(ctx, root)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		if got, want := resolved.ID, ws.ID; got != want {
			t.Fatalf("Resolve() ID = %q, want %q", got, want)
		}
		if store.listCalls == 0 {
			t.Fatal("ListWorkspaces() calls = 0, want same-root fallback")
		}
	})
}

func TestRegisterRejectsSameFilesystemRoot(t *testing.T) {
	t.Parallel()

	t.Run("Should reject an alternate path to an existing root", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := newTestHomePaths(t)
		root := t.TempDir()
		alternateRoot := symlinkWorkspaceRootForTest(t, root)
		existing := Workspace{ID: "ws_existing", RootDir: alternateRoot, Name: "repo"}
		store := newMockWorkspaceStore(existing)
		resolver := newTestResolver(t, store,
			WithHomePaths(homePaths),
			WithConfigLoader((&countingConfigLoader{cfg: validConfig(homePaths)}).Load),
			WithIDGenerator(func(_ string) string { return "ws_duplicate" }),
		)

		_, err := resolver.Register(ctx, RegisterOptions{RootDir: root, Name: "repo-duplicate"})
		if !errors.Is(err, ErrWorkspacePathTaken) {
			t.Fatalf("Register() error = %v, want %v", err, ErrWorkspacePathTaken)
		}
	})
}

func symlinkWorkspaceRootForTest(t *testing.T, target string) string {
	t.Helper()

	link := filepath.Join(t.TempDir(), "workspace-link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("create symlink workspace root: %v", err)
	}
	return link
}

func TestResolveWorkspaceSandboxCascade(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	homePaths := newTestHomePaths(t)
	baseConfig := validConfig(homePaths)
	baseConfig.Defaults.Sandbox = "default-env"
	baseConfig.Sandboxes["default-env"] = aghconfig.SandboxProfile{
		Backend:     "daytona",
		Persistence: "reuse",
		Daytona: aghconfig.DaytonaProfile{
			Snapshot: "snap-default",
		},
	}
	baseConfig.Sandboxes["explicit-env"] = aghconfig.SandboxProfile{
		Backend:  "daytona",
		SyncMode: "none",
		Daytona: aghconfig.DaytonaProfile{
			Snapshot: "snap-explicit",
		},
	}

	tests := []struct {
		name        string
		workspace   Workspace
		cfg         aghconfig.Config
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
			cfg: func() aghconfig.Config {
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
	cfg.Sandboxes["daytona-dev"] = aghconfig.SandboxProfile{
		Backend: "daytona",
		Daytona: aghconfig.DaytonaProfile{Snapshot: "snap-dev"},
	}
	cfg.Sandboxes["local-dev"] = aghconfig.SandboxProfile{Backend: "local"}

	store := newMockWorkspaceStore()
	resolver := newTestResolver(t, store,
		WithHomePaths(homePaths),
		WithConfigLoader((&countingConfigLoader{cfg: cfg}).Load),
		WithIDGenerator(func(string) string { return "ws_env" }),
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

func TestResolveOrRegisterExistingWorkspace(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	homePaths := newTestHomePaths(t)
	root := t.TempDir()
	ws := Workspace{ID: "ws_existing", RootDir: mustCanonicalRoot(t, root), Name: "repo"}

	store := newMockWorkspaceStore(ws)
	loader := &countingConfigLoader{cfg: validConfig(homePaths)}
	resolver := newTestResolver(t, store,
		WithHomePaths(homePaths),
		WithConfigLoader(loader.Load),
	)

	resolved, err := resolver.ResolveOrRegister(ctx, root)
	if err != nil {
		t.Fatalf("ResolveOrRegister() error = %v", err)
	}
	if resolved.ID != ws.ID {
		t.Fatalf("ResolveOrRegister() ID = %q, want %q", resolved.ID, ws.ID)
	}
	if got := len(store.insertCalls); got != 0 {
		t.Fatalf("InsertWorkspace() calls = %d, want 0", got)
	}
}

func TestResolveFallsBackToNameForWSLikeIdentifier(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	homePaths := newTestHomePaths(t)
	root := t.TempDir()
	ws := Workspace{ID: "ws_real", RootDir: mustCanonicalRoot(t, root), Name: "ws_alpha"}

	store := newMockWorkspaceStore(ws)
	loader := &countingConfigLoader{cfg: validConfig(homePaths)}
	resolver := newTestResolver(t, store,
		WithHomePaths(homePaths),
		WithConfigLoader(loader.Load),
	)

	resolved, err := resolver.Resolve(ctx, "ws_alpha")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved.ID != ws.ID {
		t.Fatalf("Resolve() ID = %q, want %q", resolved.ID, ws.ID)
	}
	if got := len(store.getWorkspaceCalls); got != 1 {
		t.Fatalf("GetWorkspace() calls = %d, want 1", got)
	}
	if got := len(store.getByNameCalls); got != 1 {
		t.Fatalf("GetWorkspaceByName() calls = %d, want 1", got)
	}
}

func TestResolveOrRegisterAutoRegisterDedupesNameAndPrefixesID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	homePaths := newTestHomePaths(t)
	root := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("MkdirAll(root) error = %v", err)
	}

	store := newMockWorkspaceStore(
		Workspace{ID: "ws_taken", RootDir: filepath.Join(t.TempDir(), "other"), Name: "repo"},
	)
	loader := &countingConfigLoader{cfg: validConfig(homePaths)}
	resolver := newTestResolver(t, store,
		WithHomePaths(homePaths),
		WithConfigLoader(loader.Load),
		WithIDGenerator(func(_ string) string { return "ws_fixed" }),
	)

	resolved, err := resolver.ResolveOrRegister(ctx, root)
	if err != nil {
		t.Fatalf("ResolveOrRegister() error = %v", err)
	}

	if resolved.ID != "ws_fixed" {
		t.Fatalf("ResolveOrRegister() ID = %q, want %q", resolved.ID, "ws_fixed")
	}
	if resolved.Name != "repo-2" {
		t.Fatalf("ResolveOrRegister() Name = %q, want %q", resolved.Name, "repo-2")
	}
	if !strings.HasPrefix(resolved.ID, "ws_") {
		t.Fatalf("ResolveOrRegister() ID = %q, want ws_ prefix", resolved.ID)
	}
	if got := len(store.insertCalls); got != 1 {
		t.Fatalf("InsertWorkspace() calls = %d, want 1", got)
	}
	if got := store.insertCalls[0].Name; got != "repo-2" {
		t.Fatalf("InsertWorkspace() Name = %q, want %q", got, "repo-2")
	}
}

func TestResolverCRUDFlow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	homePaths := newTestHomePaths(t)
	root := t.TempDir()
	additionalOne := t.TempDir()
	additionalTwo := t.TempDir()
	canonicalAdditionalOne := mustCanonicalRoot(t, additionalOne)
	canonicalAdditionalTwo := mustCanonicalRoot(t, additionalTwo)

	store := newMockWorkspaceStore()
	loader := &countingConfigLoader{cfg: validConfig(homePaths)}
	resolver := newTestResolver(t, store,
		WithHomePaths(homePaths),
		WithConfigLoader(loader.Load),
		WithIDGenerator(func(_ string) string { return "ws_manual" }),
	)

	registered, err := resolver.Register(ctx, RegisterOptions{
		RootDir:        root,
		Name:           "repo",
		AdditionalDirs: []string{additionalOne, root, additionalOne},
		DefaultAgent:   "workspace-agent",
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if registered.ID != "ws_manual" {
		t.Fatalf("Register() ID = %q, want %q", registered.ID, "ws_manual")
	}
	if got, want := registered.AdditionalDirs, []string{canonicalAdditionalOne}; !slices.Equal(got, want) {
		t.Fatalf("Register() AdditionalDirs = %#v, want %#v", got, want)
	}

	gotByName, err := resolver.Get(ctx, "repo")
	if err != nil {
		t.Fatalf("Get(name) error = %v", err)
	}
	if gotByName.ID != registered.ID {
		t.Fatalf("Get(name) ID = %q, want %q", gotByName.ID, registered.ID)
	}

	listed, err := resolver.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(listed) != 1 || listed[0].ID != registered.ID {
		t.Fatalf("List() = %#v, want one workspace %q", listed, registered.ID)
	}

	newName := "repo-renamed"
	newDefaultAgent := ""
	newAdditionalDirs := []string{additionalTwo}
	if err := resolver.Update(ctx, registered.ID, UpdateOptions{
		Name:           &newName,
		DefaultAgent:   &newDefaultAgent,
		AdditionalDirs: &newAdditionalDirs,
	}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	resolved, err := resolver.Resolve(ctx, registered.ID)
	if err != nil {
		t.Fatalf("Resolve(updated) error = %v", err)
	}
	if resolved.Name != newName {
		t.Fatalf("Resolve(updated) Name = %q, want %q", resolved.Name, newName)
	}
	if got, want := resolved.AdditionalDirs, []string{canonicalAdditionalTwo}; !slices.Equal(got, want) {
		t.Fatalf("Resolve(updated) AdditionalDirs = %#v, want %#v", got, want)
	}
	if resolved.DefaultAgent != "" {
		t.Fatalf("Resolve(updated) DefaultAgent = %q, want empty", resolved.DefaultAgent)
	}
	if resolved.Config.Defaults.Agent != aghconfig.DefaultAgentName {
		t.Fatalf(
			"Resolve(updated) Config.Defaults.Agent = %q, want %q",
			resolved.Config.Defaults.Agent,
			aghconfig.DefaultAgentName,
		)
	}

	if err := resolver.Unregister(ctx, registered.ID); err != nil {
		t.Fatalf("Unregister() error = %v", err)
	}
	if _, err := resolver.Get(ctx, registered.ID); !errors.Is(err, ErrWorkspaceNotFound) {
		t.Fatalf("Get(after unregister) error = %v, want %v", err, ErrWorkspaceNotFound)
	}
}

func TestResolveCacheHitInvalidateAndEviction(t *testing.T) {
	t.Parallel()
	t.Run("Should isolate cached config and invalidate every watched input", func(t *testing.T) {
		t.Parallel()
		runResolveCacheHitInvalidateAndEviction(t)
	})
}

func runResolveCacheHitInvalidateAndEviction(t *testing.T) {
	t.Helper()

	ctx := context.Background()
	homePaths := newTestHomePaths(t)
	root := t.TempDir()
	workspaceConfig := filepath.Join(root, aghconfig.DirName, aghconfig.ConfigName)
	agentFile := filepath.Join(root, aghconfig.DirName, aghconfig.AgentsDirName, "coder", agentDefinitionFile)
	skillsDir := filepath.Join(root, aghconfig.DirName, aghconfig.SkillsDirName)
	skillOne := filepath.Join(skillsDir, "alpha")
	skillTwo := filepath.Join(skillsDir, "beta")

	writeFile(t, workspaceConfig, "[http]\nport = 4242\n")
	writeAgentDef(t, agentFile, "coder", "v1")
	writeSkill(t, skillOne)

	ws := Workspace{ID: "ws_cache", RootDir: root, Name: "repo"}
	store := newMockWorkspaceStore(ws)
	loadedConfig := validConfig(homePaths)
	loadedConfig.WindowManager.HistoryLimit = 77
	loadedConfig.WindowManager.Snap.RepeatRatios = []float64{0.5, 0.75, 0.25}
	loadedConfig.WindowManager.Shortcuts = map[string]string{"window.focus.left": "alt+KeyH"}
	loader := &countingConfigLoader{cfg: loadedConfig}
	currentTime := time.Unix(1_700_000_000, 0).UTC()

	resolver := newTestResolver(t, store,
		WithHomePaths(homePaths),
		WithConfigLoader(loader.Load),
		withNow(func() time.Time { return currentTime }),
		WithCacheTTL(10*time.Minute),
	)

	first, err := resolver.Resolve(ctx, ws.ID)
	if err != nil {
		t.Fatalf("Resolve(first) error = %v", err)
	}
	if got := loader.Calls(); got != 1 {
		t.Fatalf("config loader calls after first resolve = %d, want 1", got)
	}
	if first.Config.WindowManager.HistoryLimit != 77 ||
		first.Config.WindowManager.Shortcuts["window.focus.left"] != "alt+KeyH" {
		t.Fatalf("Resolve(first) WindowManager = %#v, want loaded config", first.Config.WindowManager)
	}
	first.Config.WindowManager.Snap.RepeatRatios[0] = 0.4
	first.Config.WindowManager.Shortcuts["window.focus.left"] = "meta+KeyH"

	currentTime = currentTime.Add(1 * time.Minute)
	second, err := resolver.Resolve(ctx, ws.ID)
	if err != nil {
		t.Fatalf("Resolve(second) error = %v", err)
	}
	if got := loader.Calls(); got != 1 {
		t.Fatalf("config loader calls after cache hit = %d, want 1", got)
	}
	if !second.ResolvedAt.Equal(first.ResolvedAt) {
		t.Fatalf("ResolvedAt on cache hit = %v, want %v", second.ResolvedAt, first.ResolvedAt)
	}
	if got := second.Config.WindowManager.Snap.RepeatRatios; len(got) != 3 || got[0] != 0.5 {
		t.Fatalf("Resolve(cache hit) repeat ratios = %#v, want isolated loaded ratios", got)
	}
	if got := second.Config.WindowManager.Shortcuts["window.focus.left"]; got != "alt+KeyH" {
		t.Fatalf("Resolve(cache hit) shortcut = %q, want isolated alt+KeyH", got)
	}

	modTime := time.Unix(1_700_000_100, 0).UTC()
	writeFile(t, workspaceConfig, "[http]\nport = 4343\n")
	touchPath(t, workspaceConfig, modTime)
	currentTime = currentTime.Add(1 * time.Minute)
	if _, err := resolver.Resolve(ctx, ws.ID); err != nil {
		t.Fatalf("Resolve(after config change) error = %v", err)
	}
	if got := loader.Calls(); got != 2 {
		t.Fatalf("config loader calls after config invalidation = %d, want 2", got)
	}

	modTime = modTime.Add(1 * time.Minute)
	writeAgentDef(t, agentFile, "coder", "v2")
	touchPath(t, agentFile, modTime)
	currentTime = currentTime.Add(1 * time.Minute)
	afterAgent, err := resolver.Resolve(ctx, ws.ID)
	if err != nil {
		t.Fatalf("Resolve(after agent change) error = %v", err)
	}
	if got := loader.Calls(); got != 3 {
		t.Fatalf("config loader calls after agent invalidation = %d, want 3", got)
	}
	if got := agentModel(afterAgent.Agents, "coder"); got != "v2" {
		t.Fatalf("agent model after agent invalidation = %q, want %q", got, "v2")
	}

	writeSkill(t, skillTwo)
	touchPath(t, skillsDir, modTime.Add(1*time.Minute))
	currentTime = currentTime.Add(1 * time.Minute)
	afterSkill, err := resolver.Resolve(ctx, ws.ID)
	if err != nil {
		t.Fatalf("Resolve(after skill change) error = %v", err)
	}
	if got := loader.Calls(); got != 4 {
		t.Fatalf("config loader calls after skill invalidation = %d, want 4", got)
	}
	if got := skillNames(afterSkill.Skills); !slices.Equal(got, []string{"alpha", "beta"}) {
		t.Fatalf("skill names after skill invalidation = %#v, want %#v", got, []string{"alpha", "beta"})
	}

	resolver.Invalidate(ws.ID)
	currentTime = currentTime.Add(1 * time.Minute)
	if _, err := resolver.Resolve(ctx, ws.ID); err != nil {
		t.Fatalf("Resolve(after invalidate) error = %v", err)
	}
	if got := loader.Calls(); got != 5 {
		t.Fatalf("config loader calls after Invalidate = %d, want 5", got)
	}

	resolver.InvalidateAll()
	currentTime = currentTime.Add(1 * time.Minute)
	if _, err := resolver.Resolve(ctx, ws.ID); err != nil {
		t.Fatalf("Resolve(after InvalidateAll) error = %v", err)
	}
	if got := loader.Calls(); got != 6 {
		t.Fatalf("config loader calls after InvalidateAll = %d, want 6", got)
	}

	currentTime = currentTime.Add(11 * time.Minute)
	if _, err := resolver.Resolve(ctx, ws.ID); err != nil {
		t.Fatalf("Resolve(after TTL expiry) error = %v", err)
	}
	if got := loader.Calls(); got != 7 {
		t.Fatalf("config loader calls after TTL eviction = %d, want 7", got)
	}
}

func TestResolveMissingRootReturnsErrWorkspaceRootMissing(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	homePaths := newTestHomePaths(t)
	root := filepath.Join(t.TempDir(), "gone")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("MkdirAll(root) error = %v", err)
	}
	if err := os.RemoveAll(root); err != nil {
		t.Fatalf("RemoveAll(root) error = %v", err)
	}

	store := newMockWorkspaceStore(Workspace{ID: "ws_missing", RootDir: root, Name: "repo"})
	loader := &countingConfigLoader{cfg: validConfig(homePaths)}
	resolver := newTestResolver(t, store,
		WithHomePaths(homePaths),
		WithConfigLoader(loader.Load),
	)

	if _, err := resolver.Resolve(ctx, "ws_missing"); !errors.Is(err, ErrWorkspaceRootMissing) {
		t.Fatalf("Resolve() error = %v, want %v", err, ErrWorkspaceRootMissing)
	}
}

func TestResolveCreatesAndLoadsStableWorkspaceIdentity(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	homePaths := newTestHomePaths(t)
	root := t.TempDir()
	ws := Workspace{ID: "ws_identity", RootDir: root, Name: "repo"}
	store := newMockWorkspaceStore(ws)
	resolver := newTestResolver(t, store,
		WithHomePaths(homePaths),
		WithConfigLoader((&countingConfigLoader{cfg: validConfig(homePaths)}).Load),
	)

	first, err := resolver.Resolve(ctx, ws.ID)
	if err != nil {
		t.Fatalf("Resolve(first) error = %v", err)
	}
	if !IsWorkspaceID(first.WorkspaceID) {
		t.Fatalf("Resolve(first).WorkspaceID = %q, want workspace ULID", first.WorkspaceID)
	}
	identityPath := filepath.Join(first.RootDir, ".agh", "workspace.toml")
	identity, err := loadIdentityFile(identityPath)
	if err != nil {
		t.Fatalf("loadIdentityFile(%q) error = %v", identityPath, err)
	}
	if identity.WorkspaceID != first.WorkspaceID {
		t.Fatalf("identity.WorkspaceID = %q, want %q", identity.WorkspaceID, first.WorkspaceID)
	}

	restarted := newTestResolver(t, store,
		WithHomePaths(homePaths),
		WithConfigLoader((&countingConfigLoader{cfg: validConfig(homePaths)}).Load),
	)
	second, err := restarted.Resolve(ctx, ws.ID)
	if err != nil {
		t.Fatalf("Resolve(after restart) error = %v", err)
	}
	if second.WorkspaceID != first.WorkspaceID {
		t.Fatalf("Resolve(after restart).WorkspaceID = %q, want stable %q", second.WorkspaceID, first.WorkspaceID)
	}
}

func TestResolveMatchesWorkspaceByStableWorkspaceIdentity(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve a registered workspace by stable identity", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := newTestHomePaths(t)
		root := t.TempDir()
		canonical, err := canonicalRoot(root)
		if err != nil {
			t.Fatalf("canonicalRoot(%q) error = %v", root, err)
		}
		if _, err := ensureIdentity(
			ctx,
			canonical,
			func() time.Time { return time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC) },
			func() string { return testWorkspaceULID },
		); err != nil {
			t.Fatalf("ensureIdentity(%q) error = %v", canonical, err)
		}

		ws := Workspace{ID: "ws_registered", RootDir: canonical, Name: "repo"}
		store := newMockWorkspaceStore(ws)
		resolver := newTestResolver(t, store,
			WithHomePaths(homePaths),
			WithConfigLoader((&countingConfigLoader{cfg: validConfig(homePaths)}).Load),
		)

		resolved, err := resolver.Resolve(ctx, testWorkspaceULID)
		if err != nil {
			t.Fatalf("Resolve(stable identity) error = %v", err)
		}
		if resolved.ID != ws.ID {
			t.Fatalf("Resolve(stable identity).ID = %q, want %q", resolved.ID, ws.ID)
		}
		if resolved.WorkspaceID != testWorkspaceULID {
			t.Fatalf("Resolve(stable identity).WorkspaceID = %q, want %q", resolved.WorkspaceID, testWorkspaceULID)
		}
		if got := len(store.getByNameCalls); got != 0 {
			t.Fatalf("GetWorkspaceByName() calls = %d, want 0", got)
		}
		if got := store.listCalls; got != 1 {
			t.Fatalf("ListWorkspaces() calls = %d, want 1", got)
		}
	})

	t.Run("Should reject an unknown stable identity", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := newTestHomePaths(t)
		store := newMockWorkspaceStore()
		resolver := newTestResolver(t, store,
			WithHomePaths(homePaths),
			WithConfigLoader((&countingConfigLoader{cfg: validConfig(homePaths)}).Load),
		)

		_, err := resolver.Resolve(ctx, testWorkspaceULID)
		if !errors.Is(err, ErrWorkspaceNotFound) {
			t.Fatalf("Resolve(unknown stable identity) error = %v, want %v", err, ErrWorkspaceNotFound)
		}
		if got := len(store.getByNameCalls); got != 0 {
			t.Fatalf("GetWorkspaceByName() calls = %d, want 0", got)
		}
		if got := store.listCalls; got != 1 {
			t.Fatalf("ListWorkspaces() calls = %d, want 1", got)
		}
	})
}

func TestResolveFailsClosedForInvalidWorkspaceIdentity(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	homePaths := newTestHomePaths(t)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".agh", "workspace.toml"), `workspace_id = "invalid"
created_at = "2026-05-05T12:00:00Z"
realpath_at_creation = "/tmp/repo"
`)
	store := newMockWorkspaceStore(Workspace{ID: "ws_invalid_identity", RootDir: root, Name: "repo"})
	resolver := newTestResolver(t, store,
		WithHomePaths(homePaths),
		WithConfigLoader((&countingConfigLoader{cfg: validConfig(homePaths)}).Load),
	)

	_, err := resolver.Resolve(ctx, "ws_invalid_identity")
	if !errors.Is(err, ErrWorkspaceIdentityInvalid) {
		t.Fatalf("Resolve() error = %v, want %v", err, ErrWorkspaceIdentityInvalid)
	}
}

func TestResolveFailsClosedForPermissionDeniedWorkspaceIdentity(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("permission-denied identity test is not reliable as root")
	}

	ctx := context.Background()
	homePaths := newTestHomePaths(t)
	root := t.TempDir()
	identityPath := filepath.Join(root, ".agh", "workspace.toml")
	writeFile(t, identityPath, `workspace_id = "`+testWorkspaceULID+`"
created_at = "2026-05-05T12:00:00Z"
realpath_at_creation = "/tmp/repo"
`)
	if err := os.Chmod(identityPath, 0); err != nil {
		t.Fatalf("os.Chmod(identityPath) error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(identityPath, workspaceIdentityFilePerm)
	})

	store := newMockWorkspaceStore(Workspace{ID: "ws_permission_identity", RootDir: root, Name: "repo"})
	resolver := newTestResolver(t, store,
		WithHomePaths(homePaths),
		WithConfigLoader((&countingConfigLoader{cfg: validConfig(homePaths)}).Load),
	)

	_, err := resolver.Resolve(ctx, "ws_permission_identity")
	if !errors.Is(err, ErrWorkspaceIdentityPermissionDenied) {
		t.Fatalf("Resolve() error = %v, want %v", err, ErrWorkspaceIdentityPermissionDenied)
	}
}

func TestResolveSymlinkChangedUpdatesStoredRootDir(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	homePaths := newTestHomePaths(t)
	parent := t.TempDir()
	targetOne := filepath.Join(parent, "target-one")
	targetTwo := filepath.Join(parent, "target-two")
	link := filepath.Join(parent, "workspace-link")

	if err := os.MkdirAll(targetOne, 0o755); err != nil {
		t.Fatalf("MkdirAll(targetOne) error = %v", err)
	}
	if err := os.MkdirAll(targetTwo, 0o755); err != nil {
		t.Fatalf("MkdirAll(targetTwo) error = %v", err)
	}
	createSymlink(t, targetOne, link)
	createSymlink(t, targetTwo, link)

	store := newMockWorkspaceStore(Workspace{ID: "ws_symlink", RootDir: link, Name: "repo"})
	loader := &countingConfigLoader{cfg: validConfig(homePaths)}
	resolver := newTestResolver(t, store,
		WithHomePaths(homePaths),
		WithConfigLoader(loader.Load),
	)

	resolved, err := resolver.Resolve(ctx, "ws_symlink")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	canonicalTargetTwo := mustCanonicalRoot(t, targetTwo)
	if resolved.RootDir != canonicalTargetTwo {
		t.Fatalf("Resolve() RootDir = %q, want %q", resolved.RootDir, canonicalTargetTwo)
	}
	if got := loader.LastRoot(); got != canonicalTargetTwo {
		t.Fatalf("loadConfig root = %q, want %q", got, canonicalTargetTwo)
	}
	if updated := store.mustWorkspace("ws_symlink"); updated.RootDir != canonicalTargetTwo {
		t.Fatalf("updated store root = %q, want %q", updated.RootDir, canonicalTargetTwo)
	}
}

func TestResolveLocalAgentOverridesGlobalByName(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	homePaths := newTestHomePaths(t)
	root := t.TempDir()

	writeAgentDef(t, filepath.Join(homePaths.AgentsDir, "coder", agentDefinitionFile), "coder", "global")
	writeAgentDef(
		t,
		filepath.Join(root, aghconfig.DirName, aghconfig.AgentsDirName, "coder", agentDefinitionFile),
		"coder",
		"local",
	)
	writeAgentDef(t, filepath.Join(homePaths.AgentsDir, "reviewer", agentDefinitionFile), "reviewer", "review")

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
	if got := agentModel(resolved.Agents, "coder"); got != "local" {
		t.Fatalf("coder model = %q, want %q", got, "local")
	}
	if got := agentModel(resolved.Agents, "reviewer"); got != "review" {
		t.Fatalf("reviewer model = %q, want %q", got, "review")
	}
}

func TestResolveRecordsMalformedAgentDiagnostics(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	homePaths := newTestHomePaths(t)
	root := t.TempDir()

	writeAgentDef(
		t,
		filepath.Join(root, aghconfig.DirName, aghconfig.AgentsDirName, "healthy", agentDefinitionFile),
		"healthy",
		"local",
	)
	writeFile(
		t,
		filepath.Join(root, aghconfig.DirName, aghconfig.AgentsDirName, "no-fence", agentDefinitionFile),
		"plain body",
	)
	writeFile(
		t,
		filepath.Join(root, aghconfig.DirName, aghconfig.AgentsDirName, "unterminated", agentDefinitionFile),
		"---\nname: broken",
	)
	writeFile(
		t,
		filepath.Join(root, aghconfig.DirName, aghconfig.AgentsDirName, "bom", agentDefinitionFile),
		"\ufeff---\nname: broken\n---\nPrompt.",
	)
	writeFile(
		t,
		filepath.Join(root, aghconfig.DirName, aghconfig.AgentsDirName, "embedded-tab", agentDefinitionFile),
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
				aghconfig.DirName,
				aghconfig.AgentsDirName,
				aghconfig.BuiltinCoordinatorAgentName,
				agentDefinitionFile,
			),
			aghconfig.BuiltinCoordinatorAgentName,
			"shadowed-coordinator",
		)
		writeAgentDef(
			t,
			filepath.Join(
				homePaths.AgentsDir,
				aghconfig.BuiltinDreamingCuratorAgentName,
				agentDefinitionFile,
			),
			aghconfig.BuiltinDreamingCuratorAgentName,
			"shadowed-curator",
		)
		writeAgentDef(
			t,
			filepath.Join(root, aghconfig.DirName, aghconfig.AgentsDirName, "alias", agentDefinitionFile),
			aghconfig.BuiltinCoordinatorAgentName,
			"aliased-coordinator",
		)

		store := newMockWorkspaceStore(Workspace{ID: "ws_reserved_agents", RootDir: root, Name: "repo"})
		loader := &countingConfigLoader{cfg: validConfig(homePaths)}
		resolver := newTestResolver(t, store, WithHomePaths(homePaths), WithConfigLoader(loader.Load))
		resolved, err := resolver.Resolve(context.Background(), "ws_reserved_agents")
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		if got := agentModel(resolved.Agents, aghconfig.BuiltinCoordinatorAgentName); got != "" {
			t.Fatalf("reserved coordinator model = %q, want excluded", got)
		}
		if got := agentModel(resolved.Agents, aghconfig.BuiltinDreamingCuratorAgentName); got != "" {
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
			aghconfig.BuiltinCoordinatorAgentName,
			aghconfig.BuiltinCoordinatorAgentName,
			aghconfig.BuiltinDreamingCuratorAgentName,
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
	t.Setenv("AGH_HOME", homePaths.HomeDir)

	writeFile(t, homePaths.ConfigFile, "[http]\nhost = \"localhost\"\nport = 2123\n")
	writeFile(t, filepath.Join(root, aghconfig.DirName, aghconfig.ConfigName), "[http]\nport = 4242\n")
	writeFile(t, filepath.Join(additional, aghconfig.DirName, aghconfig.ConfigName), "[http]\nport = 9999\n")

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

func TestNewResolverValidatesDependenciesAndDefaults(t *testing.T) {
	t.Parallel()

	if _, err := NewResolver(nil); err == nil {
		t.Fatal("NewResolver(nil) error = nil, want non-nil")
	}

	store := newMockWorkspaceStore()
	if _, err := NewResolver(store, WithConfigLoader(nil)); err == nil {
		t.Fatal("NewResolver(..., WithConfigLoader(nil)) error = nil, want non-nil")
	}

	resolver, err := NewResolver(store,
		WithLogger(nil),
		withNow(nil),
		WithCacheTTL(0),
		WithIDGenerator(nil),
	)
	if err != nil {
		t.Fatalf("NewResolver(defaulted options) error = %v", err)
	}
	if resolver.logger == nil {
		t.Fatal("NewResolver() logger = nil, want default logger")
	}
	if resolver.now == nil {
		t.Fatal("NewResolver() now = nil, want default clock")
	}
	if resolver.cacheTTL != defaultCacheTTL {
		t.Fatalf("NewResolver() cacheTTL = %s, want %s", resolver.cacheTTL, defaultCacheTTL)
	}
	if resolver.idGenerator == nil {
		t.Fatal("NewResolver() idGenerator = nil, want default generator")
	}
}

func TestRegisterRollsBackWhenResolveFails(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	homePaths := newTestHomePaths(t)
	root := t.TempDir()
	store := newMockWorkspaceStore()

	resolver := newTestResolver(t, store,
		WithHomePaths(homePaths),
		WithConfigLoader(func(string) (aghconfig.Config, error) {
			return aghconfig.Config{}, errors.New("boom")
		}),
		WithIDGenerator(func(_ string) string { return "ws_fail" }),
	)

	if _, err := resolver.Register(ctx, RegisterOptions{RootDir: root, Name: "repo"}); err == nil {
		t.Fatal("Register() error = nil, want non-nil")
	}
	if got := len(store.deleteCalls); got != 1 {
		t.Fatalf("DeleteWorkspace() calls = %d, want 1", got)
	}
	if _, err := store.GetWorkspace(ctx, "ws_fail"); !errors.Is(err, ErrWorkspaceNotFound) {
		t.Fatalf("GetWorkspace(rolled back) error = %v, want %v", err, ErrWorkspaceNotFound)
	}
}

func TestChangeHookRunsAfterWorkspaceMutations(t *testing.T) {
	t.Parallel()

	t.Run("Should run after register update and unregister", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := newTestHomePaths(t)
		root := t.TempDir()
		store := newMockWorkspaceStore()
		loader := &countingConfigLoader{cfg: validConfig(homePaths)}
		var hookCalls int
		resolver := newTestResolver(t, store,
			WithHomePaths(homePaths),
			WithConfigLoader(loader.Load),
			WithIDGenerator(func(_ string) string { return "ws_change" }),
			WithChangeHook(func(context.Context) error {
				hookCalls++
				return nil
			}),
		)

		registered, err := resolver.Register(ctx, RegisterOptions{RootDir: root, Name: "repo"})
		if err != nil {
			t.Fatalf("Register() error = %v", err)
		}
		if hookCalls != 1 {
			t.Fatalf("change hook calls after Register() = %d, want 1", hookCalls)
		}

		renamed := "repo-renamed"
		if err := resolver.Update(ctx, registered.ID, UpdateOptions{Name: &renamed}); err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		if hookCalls != 2 {
			t.Fatalf("change hook calls after Update() = %d, want 2", hookCalls)
		}

		if err := resolver.Unregister(ctx, registered.ID); err != nil {
			t.Fatalf("Unregister() error = %v", err)
		}
		if hookCalls != 3 {
			t.Fatalf("change hook calls after Unregister() = %d, want 3", hookCalls)
		}
	})

	t.Run("Should roll back register when change hook fails", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := newTestHomePaths(t)
		root := t.TempDir()
		store := newMockWorkspaceStore()
		resolver := newTestResolver(t, store,
			WithHomePaths(homePaths),
			WithConfigLoader((&countingConfigLoader{cfg: validConfig(homePaths)}).Load),
			WithIDGenerator(func(_ string) string { return "ws_hook_fail" }),
			WithChangeHook(func(context.Context) error {
				return errors.New("sync failed")
			}),
		)

		if _, err := resolver.Register(ctx, RegisterOptions{RootDir: root, Name: "repo"}); err == nil {
			t.Fatal("Register() error = nil, want change hook failure")
		}
		if got := len(store.deleteCalls); got != 1 {
			t.Fatalf("DeleteWorkspace() calls = %d, want 1", got)
		}
		if _, err := store.GetWorkspace(ctx, "ws_hook_fail"); !errors.Is(err, ErrWorkspaceNotFound) {
			t.Fatalf("GetWorkspace(rolled back) error = %v, want %v", err, ErrWorkspaceNotFound)
		}
	})

	t.Run("Should roll back update when change hook fails", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := newTestHomePaths(t)
		root := t.TempDir()
		createdAt := time.Unix(1700, 0).UTC()
		updatedAt := time.Unix(1800, 0).UTC()
		existing := Workspace{
			ID:        "ws_update_hook_fail",
			RootDir:   mustCanonicalRoot(t, root),
			Name:      "repo",
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		}
		store := newMockWorkspaceStore(existing)
		hookErr := errors.New("sync failed")
		resolver := newTestResolver(t, store,
			WithHomePaths(homePaths),
			WithConfigLoader((&countingConfigLoader{cfg: validConfig(homePaths)}).Load),
			WithChangeHook(func(context.Context) error {
				return hookErr
			}),
		)

		renamed := "repo-renamed"
		err := resolver.Update(ctx, existing.ID, UpdateOptions{Name: &renamed})
		if !errors.Is(err, hookErr) {
			t.Fatalf("Update() error = %v, want hook error %v", err, hookErr)
		}
		if got := len(store.updateCalls); got != 2 {
			t.Fatalf("UpdateWorkspace() calls = %d, want 2", got)
		}
		if got := store.mustWorkspace(existing.ID); got.Name != existing.Name {
			t.Fatalf("persisted name after rollback = %q, want %q", got.Name, existing.Name)
		} else if !got.UpdatedAt.Equal(existing.UpdatedAt) {
			t.Fatalf("persisted UpdatedAt after rollback = %v, want %v", got.UpdatedAt, existing.UpdatedAt)
		}
	})

	t.Run("Should keep a committed unregister authoritative when derived sync fails", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := newTestHomePaths(t)
		root := t.TempDir()
		existing := Workspace{
			ID:      "ws_unregister_hook_fail",
			RootDir: mustCanonicalRoot(t, root),
			Name:    "repo",
		}
		store := newMockWorkspaceStore(existing)
		hookErr := errors.New("sync failed")
		resolver := newTestResolver(t, store,
			WithHomePaths(homePaths),
			WithConfigLoader((&countingConfigLoader{cfg: validConfig(homePaths)}).Load),
			WithChangeHook(func(context.Context) error {
				return hookErr
			}),
		)
		preparation := &recordingUnregisterPreparation{}
		order := make([]string, 0, 3)
		preparation.order = &order
		store.deleteHook = func() { order = append(order, "delete") }
		resolver.SetUnregisterPreparer(
			func(context.Context, Workspace) (UnregisterPreparation, error) {
				return preparation, nil
			},
		)

		if err := resolver.Unregister(ctx, existing.ID); err != nil {
			t.Fatalf("Unregister() error = %v, want committed deletion", err)
		}
		if got := len(store.deleteCalls); got != 1 {
			t.Fatalf("DeleteWorkspace() calls = %d, want 1", got)
		}
		if got := len(store.insertCalls); got != 0 {
			t.Fatalf("InsertWorkspace() calls = %d, want no partial rollback", got)
		}
		if preparation.commits != 1 || preparation.rollbacks != 0 {
			t.Fatalf(
				"preparation calls = (commit=%d, rollback=%d), want (1, 0)",
				preparation.commits,
				preparation.rollbacks,
			)
		}
		if want := []string{"before_delete", "delete", "commit"}; !slices.Equal(order, want) {
			t.Fatalf("unregister order = %v, want %v", order, want)
		}
		if _, err := store.GetWorkspace(ctx, existing.ID); !errors.Is(err, ErrWorkspaceNotFound) {
			t.Fatalf("GetWorkspace(committed unregister) error = %v, want %v", err, ErrWorkspaceNotFound)
		}
	})
}

type recordingUnregisterPreparation struct {
	beforeDeletes int
	commits       int
	rollbacks     int
	order         *[]string
}

func (p *recordingUnregisterPreparation) BeforeDelete(context.Context) error {
	p.beforeDeletes++
	if p.order != nil {
		*p.order = append(*p.order, "before_delete")
	}
	return nil
}

func (p *recordingUnregisterPreparation) Commit(context.Context) error {
	p.commits++
	if p.order != nil {
		*p.order = append(*p.order, "commit")
	}
	return nil
}

func (p *recordingUnregisterPreparation) Rollback(context.Context) error {
	p.rollbacks++
	if p.order != nil {
		*p.order = append(*p.order, "rollback")
	}
	return nil
}

func TestResolveOrRegisterReturnsConcurrentWinnerWhenPathTaken(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	homePaths := newTestHomePaths(t)
	root := t.TempDir()
	existing := Workspace{ID: "ws_existing", RootDir: mustCanonicalRoot(t, root), Name: "repo"}
	store := &concurrentPathStore{existing: existing}

	resolver := newTestResolver(t, store,
		WithHomePaths(homePaths),
		WithConfigLoader((&countingConfigLoader{cfg: validConfig(homePaths)}).Load),
		WithIDGenerator(func(_ string) string { return "ws_new" }),
	)

	resolved, err := resolver.ResolveOrRegister(ctx, root)
	if err != nil {
		t.Fatalf("ResolveOrRegister() error = %v", err)
	}
	if resolved.ID != existing.ID {
		t.Fatalf("ResolveOrRegister() ID = %q, want %q", resolved.ID, existing.ID)
	}
	if store.getByPathCalls != 2 {
		t.Fatalf("GetWorkspaceByPath() calls = %d, want 2", store.getByPathCalls)
	}
}

func TestCancellationRollbackUsesBoundedDetachedDeleteContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		workspaceID string
		run         func(context.Context, *Resolver, string) error
	}{
		{
			name:        "Should roll back Register with a bounded detached delete context",
			workspaceID: "ws_cancel_register",
			run: func(ctx context.Context, resolver *Resolver, root string) error {
				_, err := resolver.Register(ctx, RegisterOptions{RootDir: root, Name: "repo"})
				return err
			},
		},
		{
			name:        "Should roll back ResolveOrRegister with a bounded detached delete context",
			workspaceID: "ws_cancel_autoreg",
			run: func(ctx context.Context, resolver *Resolver, root string) error {
				_, err := resolver.ResolveOrRegister(ctx, root)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertResolveCancellationRollback(t, tt.workspaceID, tt.run)
		})
	}
}

func TestListReturnsClonedWorkspaces(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	root := t.TempDir()
	store := newMockWorkspaceStore(Workspace{
		ID:             "ws_list",
		RootDir:        mustCanonicalRoot(t, root),
		Name:           "repo",
		AdditionalDirs: []string{"one"},
	})
	resolver := newTestResolver(t, store)

	listed, err := resolver.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	listed[0].Name = "mutated"
	listed[0].AdditionalDirs[0] = "changed"

	stored := store.mustWorkspace("ws_list")
	if stored.Name != "repo" {
		t.Fatalf("store name after List() mutation = %q, want %q", stored.Name, "repo")
	}
	if stored.AdditionalDirs[0] != "one" {
		t.Fatalf("store AdditionalDirs after List() mutation = %#v, want %#v", stored.AdditionalDirs, []string{"one"})
	}
}

func TestListReconcilesWorkspaceRoots(t *testing.T) {
	t.Parallel()

	t.Run("Should durably prune missing roots and preserve healthy workspaces", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		healthyRoot := mustCanonicalRoot(t, t.TempDir())
		missingRoot := removedWorkspaceRoot(t)
		store := newMockWorkspaceStore(
			Workspace{ID: "ws_healthy", RootDir: healthyRoot, Name: "healthy"},
			Workspace{ID: "ws_missing", RootDir: missingRoot, Name: "missing"},
		)
		var hookCalls int
		resolver := newTestResolver(t, store, WithChangeHook(func(context.Context) error {
			hookCalls++
			return nil
		}))

		listed, err := resolver.List(ctx)
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(listed) != 1 || listed[0].ID != "ws_healthy" {
			t.Fatalf("List() = %#v, want only healthy workspace", listed)
		}
		if _, err := store.GetWorkspace(ctx, "ws_missing"); !errors.Is(err, ErrWorkspaceNotFound) {
			t.Fatalf("GetWorkspace(pruned) error = %v, want %v", err, ErrWorkspaceNotFound)
		}
		if got := store.mustWorkspace("ws_healthy"); got.RootDir != healthyRoot {
			t.Fatalf("healthy workspace root = %q, want %q", got.RootDir, healthyRoot)
		}
		if hookCalls != 1 {
			t.Fatalf("change hook calls = %d, want 1", hookCalls)
		}

		listed, err = resolver.List(ctx)
		if err != nil {
			t.Fatalf("List(second read) error = %v", err)
		}
		if len(listed) != 1 || listed[0].ID != "ws_healthy" {
			t.Fatalf("List(second read) = %#v, want only healthy workspace", listed)
		}
		if got := len(store.deleteCalls); got != 1 {
			t.Fatalf("DeleteWorkspace() calls = %d, want 1", got)
		}
	})

	t.Run("Should preserve registrations when root inspection fails", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		workspace := Workspace{ID: "ws_unreadable", RootDir: "\x00", Name: "unreadable"}
		store := newMockWorkspaceStore(workspace)
		resolver := newTestResolver(t, store)

		if _, err := resolver.List(ctx); err == nil {
			t.Fatal("List() error = nil, want root inspection failure")
		}
		if got := len(store.deleteCalls); got != 0 {
			t.Fatalf("DeleteWorkspace() calls = %d, want 0", got)
		}
		if got := store.mustWorkspace(workspace.ID); got.RootDir != workspace.RootDir {
			t.Fatalf("preserved workspace root = %q, want %q", got.RootDir, workspace.RootDir)
		}
	})

	t.Run("Should preserve a missing registration when durable deletion fails", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		workspace := Workspace{ID: "ws_delete_error", RootDir: removedWorkspaceRoot(t), Name: "delete-error"}
		store := newMockWorkspaceStore(workspace)
		deleteErr := errors.New("delete failed")
		store.deleteErr = deleteErr
		resolver := newTestResolver(t, store)
		preparation := &recordingUnregisterPreparation{}
		resolver.SetUnregisterPreparer(
			func(context.Context, Workspace) (UnregisterPreparation, error) {
				return preparation, nil
			},
		)

		if _, err := resolver.List(ctx); !errors.Is(err, deleteErr) {
			t.Fatalf("List() error = %v, want %v", err, deleteErr)
		}
		if preparation.commits != 0 || preparation.rollbacks != 1 {
			t.Fatalf(
				"preparation calls = (commit=%d, rollback=%d), want (0, 1)",
				preparation.commits,
				preparation.rollbacks,
			)
		}
		if got := store.mustWorkspace(workspace.ID); got.ID != workspace.ID {
			t.Fatalf("preserved workspace ID = %q, want %q", got.ID, workspace.ID)
		}
	})

	t.Run("Should converge concurrent list reconciliation", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		store := newMockWorkspaceStore(
			Workspace{ID: "ws_concurrent_healthy", RootDir: mustCanonicalRoot(t, t.TempDir()), Name: "healthy"},
			Workspace{ID: "ws_concurrent_missing", RootDir: removedWorkspaceRoot(t), Name: "missing"},
		)
		resolver := newTestResolver(t, store)

		const callers = 8
		start := make(chan struct{})
		results := make(chan error, callers)
		var callersDone sync.WaitGroup
		for range callers {
			callersDone.Go(func() {
				<-start
				listed, err := resolver.List(ctx)
				if err != nil {
					results <- err
					return
				}
				if len(listed) != 1 || listed[0].ID != "ws_concurrent_healthy" {
					results <- fmt.Errorf("List() = %#v, want only healthy workspace", listed)
					return
				}
				results <- nil
			})
		}

		close(start)
		callersDone.Wait()
		close(results)
		for err := range results {
			if err != nil {
				t.Fatalf("concurrent List() error = %v", err)
			}
		}
		if got := len(store.deleteCalls); got != 1 {
			t.Fatalf("DeleteWorkspace() calls = %d, want 1", got)
		}
	})

	t.Run("Should converge when a concurrent unregister wins durable deletion", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		store := &concurrentUnregisterStore{mockWorkspaceStore: newMockWorkspaceStore(Workspace{
			ID:      "ws_concurrent_unregister",
			RootDir: removedWorkspaceRoot(t),
			Name:    "concurrent-unregister",
		})}
		resolver := newTestResolver(t, store)

		listed, err := resolver.List(ctx)
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(listed) != 0 {
			t.Fatalf("List() = %#v, want no workspaces", listed)
		}
		listed, err = resolver.List(ctx)
		if err != nil {
			t.Fatalf("List(second read) error = %v", err)
		}
		if len(listed) != 0 {
			t.Fatalf("List(second read) = %#v, want no workspaces", listed)
		}
	})
}

func removedWorkspaceRoot(t *testing.T) string {
	t.Helper()

	root := filepath.Join(t.TempDir(), "removed-workspace")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatalf("Mkdir(%q) error = %v", root, err)
	}
	if err := os.Remove(root); err != nil {
		t.Fatalf("Remove(%q) error = %v", root, err)
	}
	return root
}

func TestCloneConfigProducesDeepCopy(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve provider model reasoning and curation metadata", func(t *testing.T) {
		t.Parallel()

		deprecated := false
		hidden := true
		featured := false
		original := aghconfig.Config{
			Providers: map[string]aghconfig.ProviderConfig{
				"claude": {
					Models: aghconfig.ProviderModelsConfig{
						Reasoning: aghconfig.ProviderReasoningConfig{
							Apply: aghconfig.ReasoningApplyNone,
						},
						Curated: []aghconfig.ProviderModelConfig{{
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
		if got, want := provider.Models.Reasoning.Apply, aghconfig.ReasoningApplyNone; got != want {
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
		original := aghconfig.Config{
			Agents: aghconfig.AgentsConfig{
				Soul: aghconfig.SoulConfig{
					Enabled:                true,
					MaxBodyBytes:           32768,
					ContextProjectionBytes: 2048,
				},
				Heartbeat: aghconfig.HeartbeatConfig{
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
			Session: aghconfig.SessionConfig{
				Limits: aghconfig.SessionLimitsConfig{
					Timeout: time.Minute,
				},
			},
			Roles: aghconfig.RolesConfig{
				Coordinator: aghconfig.CoordinatorRoleConfig{
					RoleConfig: aghconfig.RoleConfig{
						Enabled:       true,
						Agent:         "coordinator",
						Provider:      "codex",
						Model:         "gpt-4o",
						FallbackChain: []aghconfig.RoleFallback{{Provider: "claude", Model: "sonnet"}},
					},
					TTL:                           45 * time.Minute,
					MaxChildren:                   5,
					MaxActiveSessionsPerWorkspace: 5,
				},
			},
			RoleSources: aghconfig.RoleFieldSources{
				aghconfig.RoleCoordinator: {"model": aghconfig.RoleFieldSourceDefault},
			},
			Providers: map[string]aghconfig.ProviderConfig{
				"claude": {
					Command: "claude",
					Models: aghconfig.ProviderModelsConfig{
						Default: "sonnet",
					},
					CredentialSlots: []aghconfig.ProviderCredentialSlot{
						{
							Name:      "api_key",
							TargetEnv: "ANTHROPIC_API_KEY",
							SecretRef: "env:ANTHROPIC_API_KEY",
							Kind:      "api_key",
							Required:  true,
						},
					},
					MCPServers: []aghconfig.MCPServer{
						{
							Name:    "github",
							Command: "npx",
							Args:    []string{"-y"},
							Env:     map[string]string{"TOKEN": "one"},
						},
					},
				},
			},
			Skills: aghconfig.SkillsConfig{
				Enabled:        true,
				DisabledSkills: []string{"alpha"},
				PollInterval:   time.Second,
			},
			Hooks: aghconfig.HooksConfig{
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
		if got := cloned.RoleSources[aghconfig.RoleCoordinator]["model"]; got != aghconfig.RoleFieldSourceDefault {
			t.Fatalf("cloned RoleSources = %#v, want preserved coordinator model source", cloned.RoleSources)
		}
		cloned.Agents.Soul.MaxBodyBytes = 8192
		cloned.Agents.Heartbeat.MaxBodyBytes = 8192
		cloned.Session.Limits.Timeout = 2 * time.Minute
		cloned.Roles.Coordinator.Enabled = false
		cloned.Roles.Coordinator.Agent = "mutated-coordinator"
		cloned.Roles.Coordinator.TTL = 2 * time.Hour
		cloned.Roles.Coordinator.FallbackChain[0].Model = "mutated-model"
		cloned.RoleSources[aghconfig.RoleCoordinator]["model"] = "overlay"
		cloned.Providers["claude"] = aghconfig.ProviderConfig{}
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
		if got := original.RoleSources[aghconfig.RoleCoordinator]["model"]; got != aghconfig.RoleFieldSourceDefault {
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

func TestWorkspaceHelperFunctions(t *testing.T) {
	t.Parallel()

	t.Run("Should errorType", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			err  error
			want string
		}{
			{err: ErrWorkspaceNotFound, want: "workspace_not_found"},
			{err: ErrWorkspaceRootMissing, want: "workspace_root_missing"},
			{err: ErrWorkspaceNameTaken, want: "workspace_name_taken"},
			{err: ErrWorkspacePathTaken, want: "workspace_path_taken"},
			{err: context.Canceled, want: "context_canceled"},
			{err: context.DeadlineExceeded, want: "context_deadline_exceeded"},
			{err: errors.New("other"), want: "error"},
			{err: nil, want: ""},
		}

		for _, tt := range tests {
			if got := errorType(tt.err); got != tt.want {
				t.Fatalf("errorType(%v) = %q, want %q", tt.err, got, tt.want)
			}
		}
	})

	t.Run("Should checkContext", func(t *testing.T) {
		t.Parallel()

		if err := checkContext(nilTestContext()); err == nil {
			t.Fatal("checkContext(nil) error = nil, want non-nil")
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := checkContext(ctx); !errors.Is(err, context.Canceled) {
			t.Fatalf("checkContext(canceled) error = %v, want %v", err, context.Canceled)
		}

		if err := checkContext(context.Background()); err != nil {
			t.Fatalf("checkContext(background) error = %v, want nil", err)
		}
	})

	t.Run("Should canonicalRoot", func(t *testing.T) {
		t.Parallel()

		if _, err := canonicalRoot(""); err == nil {
			t.Fatal("canonicalRoot(\"\") error = nil, want non-nil")
		}
		if _, err := canonicalRoot(filepath.Join(t.TempDir(), "missing")); !errors.Is(err, ErrWorkspaceRootMissing) {
			t.Fatalf("canonicalRoot(missing) error = %v, want %v", err, ErrWorkspaceRootMissing)
		}

		filePath := filepath.Join(t.TempDir(), "file.txt")
		writeFile(t, filePath, "not-a-dir")
		if _, err := canonicalRoot(filePath); err == nil {
			t.Fatal("canonicalRoot(file) error = nil, want non-nil")
		}
	})

	t.Run("Should snapshots and overrides", func(t *testing.T) {
		t.Parallel()

		snapshots := make(map[string]filesnap.Snapshot)
		if err := addSnapshotIfExists("", snapshots); err != nil {
			t.Fatalf("addSnapshotIfExists(\"\") error = %v", err)
		}
		if err := addSnapshotIfExists(filepath.Join(t.TempDir(), "missing"), snapshots); err != nil {
			t.Fatalf("addSnapshotIfExists(missing) error = %v", err)
		}
		if len(snapshots) != 0 {
			t.Fatalf("snapshots for missing path = %#v, want empty", snapshots)
		}

		cfg := aghconfig.Config{Defaults: aghconfig.DefaultsConfig{Agent: aghconfig.DefaultAgentName}}
		applyDefaultAgentOverride(&cfg, "")
		if cfg.Defaults.Agent != aghconfig.DefaultAgentName {
			t.Fatalf(
				"Defaults.Agent after empty override = %q, want %q",
				cfg.Defaults.Agent,
				aghconfig.DefaultAgentName,
			)
		}
		applyDefaultAgentOverride(&cfg, "workspace-agent")
		if cfg.Defaults.Agent != "workspace-agent" {
			t.Fatalf("Defaults.Agent after override = %q, want %q", cfg.Defaults.Agent, "workspace-agent")
		}

		left := map[string]filesnap.Snapshot{"a": {ModTime: time.Unix(1, 0), Size: 1}}
		right := map[string]filesnap.Snapshot{"a": {ModTime: time.Unix(1, 0), Size: 1}}
		if !filesnap.Equal(left, right) {
			t.Fatal("filesnap.Equal() = false, want true")
		}
		right["a"] = filesnap.Snapshot{ModTime: time.Unix(2, 0), Size: 1}
		if filesnap.Equal(left, right) {
			t.Fatal("filesnap.Equal() = true, want false")
		}

		if got := cloneStringMap(nil); got != nil {
			t.Fatalf("cloneStringMap(nil) = %#v, want nil", got)
		}
	})

	t.Run("Should generateID", func(t *testing.T) {
		t.Parallel()

		if got := generateID("ws"); !strings.HasPrefix(got, "ws_") {
			t.Fatalf("generateID(ws) = %q, want ws_ prefix", got)
		}
		if got := generateID(""); got == "" {
			t.Fatal("generateID(\"\") = empty, want non-empty")
		}
	})
}

type mockWorkspaceStore struct {
	mu sync.Mutex

	workspaces map[string]Workspace
	deleteErr  error
	deleteHook func()

	insertCalls       []Workspace
	updateCalls       []Workspace
	deleteCalls       []string
	getWorkspaceCalls []string
	getByPathCalls    []string
	getByNameCalls    []string
	listCalls         int
}

func newMockWorkspaceStore(workspaces ...Workspace) *mockWorkspaceStore {
	store := &mockWorkspaceStore{
		workspaces: make(map[string]Workspace, len(workspaces)),
	}
	for _, ws := range workspaces {
		store.workspaces[ws.ID] = cloneWorkspace(ws)
	}
	return store
}

func (m *mockWorkspaceStore) InsertWorkspace(_ context.Context, ws Workspace) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.insertCalls = append(m.insertCalls, cloneWorkspace(ws))
	for _, existing := range m.workspaces {
		switch {
		case existing.RootDir == ws.RootDir:
			return ErrWorkspacePathTaken
		case existing.Name == ws.Name:
			return ErrWorkspaceNameTaken
		}
	}
	m.workspaces[ws.ID] = cloneWorkspace(ws)
	return nil
}

func (m *mockWorkspaceStore) UpdateWorkspace(_ context.Context, ws Workspace) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.updateCalls = append(m.updateCalls, cloneWorkspace(ws))
	if _, ok := m.workspaces[ws.ID]; !ok {
		return ErrWorkspaceNotFound
	}
	for _, existing := range m.workspaces {
		if existing.ID == ws.ID {
			continue
		}
		switch {
		case existing.RootDir == ws.RootDir:
			return ErrWorkspacePathTaken
		case existing.Name == ws.Name:
			return ErrWorkspaceNameTaken
		}
	}
	m.workspaces[ws.ID] = cloneWorkspace(ws)
	return nil
}

func (m *mockWorkspaceStore) DeleteWorkspace(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.deleteCalls = append(m.deleteCalls, id)
	if m.deleteHook != nil {
		m.deleteHook()
	}
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if _, ok := m.workspaces[id]; !ok {
		return ErrWorkspaceNotFound
	}
	delete(m.workspaces, id)
	return nil
}

func (m *mockWorkspaceStore) GetWorkspace(_ context.Context, id string) (Workspace, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.getWorkspaceCalls = append(m.getWorkspaceCalls, id)
	ws, ok := m.workspaces[id]
	if !ok {
		return Workspace{}, ErrWorkspaceNotFound
	}
	return cloneWorkspace(ws), nil
}

func (m *mockWorkspaceStore) GetWorkspaceByPath(_ context.Context, rootDir string) (Workspace, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.getByPathCalls = append(m.getByPathCalls, rootDir)
	for _, ws := range m.workspaces {
		if ws.RootDir == rootDir {
			return cloneWorkspace(ws), nil
		}
	}
	return Workspace{}, ErrWorkspaceNotFound
}

func (m *mockWorkspaceStore) GetWorkspaceByName(_ context.Context, name string) (Workspace, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.getByNameCalls = append(m.getByNameCalls, name)
	for _, ws := range m.workspaces {
		if ws.Name == name {
			return cloneWorkspace(ws), nil
		}
	}
	return Workspace{}, ErrWorkspaceNotFound
}

func (m *mockWorkspaceStore) ListWorkspaces(_ context.Context) ([]Workspace, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.listCalls++
	workspaces := make([]Workspace, 0, len(m.workspaces))
	for _, ws := range m.workspaces {
		workspaces = append(workspaces, cloneWorkspace(ws))
	}
	slices.SortFunc(workspaces, func(left, right Workspace) int {
		if compare := strings.Compare(left.Name, right.Name); compare != 0 {
			return compare
		}
		return strings.Compare(left.ID, right.ID)
	})
	return workspaces, nil
}

func (m *mockWorkspaceStore) mustWorkspace(id string) Workspace {
	m.mu.Lock()
	defer m.mu.Unlock()

	ws, ok := m.workspaces[id]
	if !ok {
		panic("workspace not found: " + id)
	}
	return cloneWorkspace(ws)
}

type countingConfigLoader struct {
	mu    sync.Mutex
	cfg   aghconfig.Config
	calls int
	roots []string
}

type concurrentPathStore struct {
	existing       Workspace
	getByPathCalls int
}

type concurrentUnregisterStore struct {
	*mockWorkspaceStore
}

func (s *concurrentUnregisterStore) DeleteWorkspace(ctx context.Context, id string) error {
	if err := s.mockWorkspaceStore.DeleteWorkspace(ctx, id); err != nil {
		return err
	}
	return ErrWorkspaceNotFound
}

type cancelOnInsertStore struct {
	*mockWorkspaceStore
	cancel context.CancelFunc

	deleteCtxErr      error
	deleteHasDeadline bool
}

func (s *concurrentPathStore) InsertWorkspace(context.Context, Workspace) error {
	return ErrWorkspacePathTaken
}

func (s *concurrentPathStore) UpdateWorkspace(context.Context, Workspace) error {
	return nil
}

func (s *concurrentPathStore) DeleteWorkspace(context.Context, string) error {
	return nil
}

func (s *concurrentPathStore) GetWorkspace(_ context.Context, id string) (Workspace, error) {
	if id != s.existing.ID {
		return Workspace{}, ErrWorkspaceNotFound
	}
	return cloneWorkspace(s.existing), nil
}

func (s *concurrentPathStore) GetWorkspaceByPath(_ context.Context, rootDir string) (Workspace, error) {
	s.getByPathCalls++
	if s.getByPathCalls == 1 {
		return Workspace{}, ErrWorkspaceNotFound
	}
	if rootDir != s.existing.RootDir {
		return Workspace{}, ErrWorkspaceNotFound
	}
	return cloneWorkspace(s.existing), nil
}

func (s *concurrentPathStore) GetWorkspaceByName(context.Context, string) (Workspace, error) {
	return Workspace{}, ErrWorkspaceNotFound
}

func (s *concurrentPathStore) ListWorkspaces(context.Context) ([]Workspace, error) {
	return nil, nil
}

func (s *cancelOnInsertStore) InsertWorkspace(ctx context.Context, ws Workspace) error {
	if err := s.mockWorkspaceStore.InsertWorkspace(ctx, ws); err != nil {
		return err
	}
	if s.cancel != nil {
		s.cancel()
	}
	return nil
}

func (s *cancelOnInsertStore) DeleteWorkspace(ctx context.Context, id string) error {
	s.deleteCtxErr = ctx.Err()
	_, s.deleteHasDeadline = ctx.Deadline()
	return s.mockWorkspaceStore.DeleteWorkspace(ctx, id)
}

func (l *countingConfigLoader) Load(root string) (aghconfig.Config, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.calls++
	l.roots = append(l.roots, root)
	return cloneConfig(&l.cfg), nil
}

func (l *countingConfigLoader) Calls() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.calls
}

func (l *countingConfigLoader) LastRoot() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.roots) == 0 {
		return ""
	}
	return l.roots[len(l.roots)-1]
}

func newTestResolver(tb testing.TB, store Store, opts ...Option) *Resolver {
	tb.Helper()

	opts = append([]Option{WithLogger(discardLogger())}, opts...)
	resolver, err := NewResolver(store, opts...)
	if err != nil {
		tb.Fatalf("NewResolver() error = %v", err)
	}
	return resolver
}

func newTestHomePaths(tb testing.TB) aghconfig.HomePaths {
	tb.Helper()

	homePaths, err := aghconfig.ResolveHomePathsFrom(filepath.Join(tb.TempDir(), "home"))
	if err != nil {
		tb.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := aghconfig.EnsureHomeLayout(homePaths); err != nil {
		tb.Fatalf("EnsureHomeLayout() error = %v", err)
	}
	return homePaths
}

func mustCanonicalRoot(t *testing.T, path string) string {
	t.Helper()

	root, err := canonicalRoot(path)
	if err != nil {
		t.Fatalf("canonicalRoot(%q) error = %v", path, err)
	}
	return root
}

func validConfig(homePaths aghconfig.HomePaths) aghconfig.Config {
	cfg := aghconfig.DefaultWithHome(homePaths)
	cfg.HTTP.Port = 2123
	return cfg
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func writeAgentDef(tb testing.TB, path string, name string, model string) {
	tb.Helper()
	writeFile(tb, path, strings.Join([]string{
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

func writeSkill(tb testing.TB, dir string) {
	tb.Helper()
	writeFile(tb, filepath.Join(dir, skillDefinitionFile), strings.Join([]string{
		"---",
		"name: " + filepath.Base(dir),
		"description: test skill",
		"---",
		"",
		"Skill body.",
		"",
	}, "\n"))
}

func writeFile(tb testing.TB, path string, contents string) {
	tb.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		tb.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		tb.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func touchPath(t *testing.T, path string, modTime time.Time) {
	t.Helper()
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("Chtimes(%q) error = %v", path, err)
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

func agentModel(agents []aghconfig.AgentDef, name string) string {
	for _, agent := range agents {
		if agent.Name == name {
			return agent.Model
		}
	}
	return ""
}

func skillNames(skills []SkillPath) []string {
	if len(skills) == 0 {
		return nil
	}

	names := make([]string, 0, len(skills))
	for _, skill := range skills {
		names = append(names, filepath.Base(skill.Dir))
	}
	return names
}

func nilTestContext() context.Context {
	return nil
}

func assertResolveCancellationRollback(
	t *testing.T,
	workspaceID string,
	run func(context.Context, *Resolver, string) error,
) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	homePaths := newTestHomePaths(t)
	root := t.TempDir()
	store := &cancelOnInsertStore{
		mockWorkspaceStore: newMockWorkspaceStore(),
		cancel:             cancel,
	}

	resolver := newTestResolver(t, store,
		WithHomePaths(homePaths),
		WithIDGenerator(func(string) string { return workspaceID }),
	)

	if err := run(ctx, resolver, root); !errors.Is(err, context.Canceled) {
		t.Fatalf("run() error = %v, want %v", err, context.Canceled)
	}
	if got := len(store.deleteCalls); got != 1 {
		t.Fatalf("DeleteWorkspace() calls = %d, want 1", got)
	}
	if store.deleteCtxErr != nil {
		t.Fatalf("DeleteWorkspace() context error = %v, want nil", store.deleteCtxErr)
	}
	if !store.deleteHasDeadline {
		t.Fatal("DeleteWorkspace() context deadline missing, want bounded rollback timeout")
	}
	if _, err := store.GetWorkspace(context.Background(), workspaceID); !errors.Is(err, ErrWorkspaceNotFound) {
		t.Fatalf("GetWorkspace(rolled back after cancellation) error = %v, want %v", err, ErrWorkspaceNotFound)
	}
}
