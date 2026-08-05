package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func TestResolveOrRegisterDiscoversBeforeRegistration(t *testing.T) {
	t.Parallel()

	t.Run("Should bind a subdirectory to its enclosing workspace without registering it", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := newTestHomePaths(t)
		root := t.TempDir()
		nested := filepath.Join(root, "src", "feature")
		if err := os.MkdirAll(nested, 0o755); err != nil {
			t.Fatalf("MkdirAll(nested) error = %v", err)
		}
		workspace := Workspace{ID: "ws_existing_nested", RootDir: root, Name: "existing"}
		store := newMockWorkspaceStore(workspace)
		resolver := newTestResolver(
			t,
			store,
			WithHomePaths(homePaths),
			WithConfigLoader((&countingConfigLoader{cfg: validConfig(homePaths)}).Load),
		)

		resolved, err := resolver.ResolveOrRegister(ctx, nested)
		if err != nil {
			t.Fatalf("ResolveOrRegister(nested) error = %v", err)
		}
		if resolved.ID != workspace.ID {
			t.Fatalf("ResolveOrRegister(nested) ID = %q, want %q", resolved.ID, workspace.ID)
		}
		if len(store.insertCalls) != 0 {
			t.Fatalf("InsertWorkspace() calls = %d, want 0", len(store.insertCalls))
		}
	})
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
		WithIDGenerator(func(_ string) (string, error) { return "ws_fixed", nil }),
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
		WithIDGenerator(func(_ string) (string, error) { return "ws_manual", nil }),
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
	if resolved.Config.Defaults.Agent != compozyconfig.DefaultAgentName {
		t.Fatalf(
			"Resolve(updated) Config.Defaults.Agent = %q, want %q",
			resolved.Config.Defaults.Agent,
			compozyconfig.DefaultAgentName,
		)
	}

	if err := resolver.Unregister(ctx, registered.ID); err != nil {
		t.Fatalf("Unregister() error = %v", err)
	}
	if _, err := resolver.Get(ctx, registered.ID); !errors.Is(err, ErrWorkspaceNotFound) {
		t.Fatalf("Get(after unregister) error = %v, want %v", err, ErrWorkspaceNotFound)
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
		WithConfigLoader(func(string) (compozyconfig.Config, error) {
			return compozyconfig.Config{}, errors.New("boom")
		}),
		WithIDGenerator(func(_ string) (string, error) { return "ws_fail", nil }),
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

func TestRegisterIDGeneration(t *testing.T) {
	t.Parallel()

	t.Run("Should stop before publication when initial id generation fails", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		homePaths := newTestHomePaths(t)
		root := t.TempDir()
		store := newMockWorkspaceStore()
		entropyErr := errors.New("entropy unavailable")
		hookCalls := 0
		resolver := newTestResolver(t, store,
			WithHomePaths(homePaths),
			WithConfigLoader((&countingConfigLoader{cfg: validConfig(homePaths)}).Load),
			WithIDGenerator(func(string) (string, error) {
				return "", entropyErr
			}),
			WithChangeHook(func(context.Context) error {
				hookCalls++
				return nil
			}),
		)

		_, err := resolver.Register(ctx, RegisterOptions{RootDir: root, Name: "repo"})
		if !errors.Is(err, entropyErr) {
			t.Fatalf("Register() error = %v, want %v", err, entropyErr)
		}
		if got := len(store.insertCalls); got != 0 {
			t.Fatalf("InsertWorkspace() calls = %d, want 0", got)
		}
		if hookCalls != 0 {
			t.Fatalf("change hook calls = %d, want 0", hookCalls)
		}
		resolver.mu.RLock()
		cached := len(resolver.cache)
		resolver.mu.RUnlock()
		if cached != 0 {
			t.Fatalf("resolver cache entries = %d, want 0", cached)
		}
	})

	t.Run("Should stop retry when id generation fails after an automatic name collision", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		homePaths := newTestHomePaths(t)
		root := t.TempDir()
		store := &nameCollisionOnceStore{
			mockWorkspaceStore: newMockWorkspaceStore(),
			collisionPending:   true,
		}
		entropyErr := errors.New("entropy unavailable")
		generationCalls := 0
		hookCalls := 0
		resolver := newTestResolver(t, store,
			WithHomePaths(homePaths),
			WithConfigLoader((&countingConfigLoader{cfg: validConfig(homePaths)}).Load),
			WithIDGenerator(func(string) (string, error) {
				generationCalls++
				if generationCalls == 1 {
					return "ws_initial", nil
				}
				return "", entropyErr
			}),
			WithChangeHook(func(context.Context) error {
				hookCalls++
				return nil
			}),
		)

		_, err := resolver.Register(ctx, RegisterOptions{RootDir: root})
		if !errors.Is(err, entropyErr) {
			t.Fatalf("Register() error = %v, want %v", err, entropyErr)
		}
		if generationCalls != 2 {
			t.Fatalf("ID generation calls = %d, want 2", generationCalls)
		}
		if got := len(store.insertCalls); got != 1 {
			t.Fatalf("InsertWorkspace() calls = %d, want 1", got)
		}
		if got := store.insertCalls[0].ID; got != "ws_initial" {
			t.Fatalf("first InsertWorkspace() ID = %q, want %q", got, "ws_initial")
		}
		if hookCalls != 0 {
			t.Fatalf("change hook calls = %d, want 0", hookCalls)
		}
		resolver.mu.RLock()
		cached := len(resolver.cache)
		resolver.mu.RUnlock()
		if cached != 0 {
			t.Fatalf("resolver cache entries = %d, want 0", cached)
		}
		workspaces, listErr := store.ListWorkspaces(ctx)
		if listErr != nil {
			t.Fatalf("ListWorkspaces() error = %v", listErr)
		}
		if len(workspaces) != 1 || workspaces[0].ID != "ws_competitor" {
			t.Fatalf("ListWorkspaces() = %#v, want only the concurrent workspace", workspaces)
		}
	})

	t.Run("Should use a fresh id after an automatic name collision", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		homePaths := newTestHomePaths(t)
		root := t.TempDir()
		store := &nameCollisionOnceStore{
			mockWorkspaceStore: newMockWorkspaceStore(),
			collisionPending:   true,
		}
		ids := []string{"ws_initial", "ws_retry"}
		generationCalls := 0
		resolver := newTestResolver(t, store,
			WithHomePaths(homePaths),
			WithConfigLoader((&countingConfigLoader{cfg: validConfig(homePaths)}).Load),
			WithIDGenerator(func(string) (string, error) {
				id := ids[generationCalls]
				generationCalls++
				return id, nil
			}),
		)

		registered, err := resolver.Register(ctx, RegisterOptions{RootDir: root})
		if err != nil {
			t.Fatalf("Register() error = %v", err)
		}
		if registered.ID != "ws_retry" {
			t.Fatalf("Register() ID = %q, want %q", registered.ID, "ws_retry")
		}
		if generationCalls != len(ids) {
			t.Fatalf("ID generation calls = %d, want %d", generationCalls, len(ids))
		}
		if got := len(store.insertCalls); got != 2 {
			t.Fatalf("InsertWorkspace() calls = %d, want 2", got)
		}
		if got := store.insertCalls[0].ID; got != "ws_initial" {
			t.Fatalf("first InsertWorkspace() ID = %q, want %q", got, "ws_initial")
		}
		if got := store.insertCalls[1].ID; got != "ws_retry" {
			t.Fatalf("retry InsertWorkspace() ID = %q, want %q", got, "ws_retry")
		}
	})

	t.Run("Should register with the default id generator", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		homePaths := newTestHomePaths(t)
		resolver := newTestResolver(t, newMockWorkspaceStore(),
			WithHomePaths(homePaths),
			WithConfigLoader((&countingConfigLoader{cfg: validConfig(homePaths)}).Load),
		)

		registered, err := resolver.Register(ctx, RegisterOptions{RootDir: t.TempDir(), Name: "repo"})
		if err != nil {
			t.Fatalf("Register() error = %v", err)
		}
		if !strings.HasPrefix(registered.ID, "ws_") {
			t.Fatalf("Register() ID = %q, want ws_ prefix", registered.ID)
		}
		if got, want := len(registered.ID), len("ws_")+16; got != want {
			t.Fatalf("Register() ID length = %d, want %d", got, want)
		}
	})
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
			WithIDGenerator(func(_ string) (string, error) { return "ws_change", nil }),
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
			WithIDGenerator(func(_ string) (string, error) { return "ws_hook_fail", nil }),
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
		WithIDGenerator(func(_ string) (string, error) { return "ws_new", nil }),
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
