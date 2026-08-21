package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/filesnap"
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

func TestResolveDiscoversNearestEnclosingWorkspace(t *testing.T) {
	t.Parallel()

	t.Run("Should choose the nearest registered root for a nested directory", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := newTestHomePaths(t)
		outerRoot := t.TempDir()
		innerRoot := filepath.Join(outerRoot, "packages", "inner")
		cwd := filepath.Join(innerRoot, "src", "feature")
		if err := os.MkdirAll(cwd, 0o755); err != nil {
			t.Fatalf("MkdirAll(cwd) error = %v", err)
		}
		outer := Workspace{ID: "ws_outer", RootDir: outerRoot, Name: "outer"}
		inner := Workspace{ID: "ws_inner", RootDir: innerRoot, Name: "inner"}
		resolver := newTestResolver(
			t,
			newMockWorkspaceStore(outer, inner),
			WithHomePaths(homePaths),
			WithConfigLoader((&countingConfigLoader{cfg: validConfig(homePaths)}).Load),
		)

		resolved, err := resolver.Resolve(ctx, cwd)
		if err != nil {
			t.Fatalf("Resolve(nested cwd) error = %v", err)
		}
		if resolved.ID != inner.ID {
			t.Fatalf("Resolve(nested cwd) ID = %q, want nearest %q", resolved.ID, inner.ID)
		}
	})

	t.Run("Should choose the nearest registered root across filesystem case aliases", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := newTestHomePaths(t)
		outerRoot := t.TempDir()
		innerRoot := filepath.Join(outerRoot, "packages", "inner")
		cwd := filepath.Join(innerRoot, "src", "feature")
		if err := os.MkdirAll(cwd, 0o755); err != nil {
			t.Fatalf("MkdirAll(cwd) error = %v", err)
		}
		innerAlias, ok := existingCaseAlias(innerRoot)
		if !ok {
			t.Skip("filesystem does not expose case-insensitive path aliases")
		}
		outer := Workspace{ID: "ws_outer", RootDir: outerRoot, Name: "outer"}
		inner := Workspace{ID: "ws_inner", RootDir: innerAlias, Name: "inner"}
		resolver := newTestResolver(
			t,
			newMockWorkspaceStore(outer, inner),
			WithHomePaths(homePaths),
			WithConfigLoader((&countingConfigLoader{cfg: validConfig(homePaths)}).Load),
		)

		resolved, err := resolver.Resolve(ctx, cwd)
		if err != nil {
			t.Fatalf("Resolve(case-aliased nested cwd) error = %v", err)
		}
		if resolved.ID != inner.ID {
			t.Fatalf("Resolve(case-aliased nested cwd) ID = %q, want nearest %q", resolved.ID, inner.ID)
		}
	})

	t.Run("Should reject path-prefix collisions and not treat AdditionalDirs as CWD roots", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := newTestHomePaths(t)
		parent := t.TempDir()
		root := filepath.Join(parent, "alpha")
		prefixCollision := filepath.Join(parent, "alpha-2")
		additional := filepath.Join(parent, "shared")
		for _, path := range []string{root, prefixCollision, additional} {
			if err := os.MkdirAll(path, 0o755); err != nil {
				t.Fatalf("MkdirAll(%q) error = %v", path, err)
			}
		}
		workspace := Workspace{
			ID:             "ws_alpha",
			RootDir:        root,
			Name:           "alpha",
			AdditionalDirs: []string{additional},
		}
		resolver := newTestResolver(
			t,
			newMockWorkspaceStore(workspace),
			WithHomePaths(homePaths),
			WithConfigLoader((&countingConfigLoader{cfg: validConfig(homePaths)}).Load),
		)

		for _, path := range []string{prefixCollision, additional} {
			_, err := resolver.Resolve(ctx, path)
			if !errors.Is(err, ErrWorkspaceNotFound) {
				t.Fatalf("Resolve(%q) error = %v, want ErrWorkspaceNotFound", path, err)
			}
		}
	})

	t.Run("Should preserve symlink canonicalization for nested directories", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := newTestHomePaths(t)
		root := t.TempDir()
		nested := filepath.Join(root, "src", "nested")
		if err := os.MkdirAll(nested, 0o755); err != nil {
			t.Fatalf("MkdirAll(nested) error = %v", err)
		}
		link := symlinkWorkspaceRootForTest(t, root)
		workspace := Workspace{ID: "ws_symlink_nested", RootDir: root, Name: "symlink-nested"}
		resolver := newTestResolver(
			t,
			newMockWorkspaceStore(workspace),
			WithHomePaths(homePaths),
			WithConfigLoader((&countingConfigLoader{cfg: validConfig(homePaths)}).Load),
		)

		resolved, err := resolver.Resolve(ctx, filepath.Join(link, "src", "nested"))
		if err != nil {
			t.Fatalf("Resolve(symlink nested) error = %v", err)
		}
		if resolved.ID != workspace.ID {
			t.Fatalf("Resolve(symlink nested) ID = %q, want %q", resolved.ID, workspace.ID)
		}
	})

	t.Run("Should serialize ancestor registration with child auto-registration", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := newTestHomePaths(t)
		ancestorRoot := t.TempDir()
		childRoot := filepath.Join(ancestorRoot, "packages", "child")
		if err := os.MkdirAll(childRoot, 0o755); err != nil {
			t.Fatalf("MkdirAll(childRoot) error = %v", err)
		}
		store := newSerializedRegistrationStore()
		resolver := newTestResolver(
			t,
			store,
			WithHomePaths(homePaths),
			WithConfigLoader((&countingConfigLoader{cfg: validConfig(homePaths)}).Load),
		)

		childResult := make(chan error, 1)
		go func() {
			_, err := resolver.ResolveOrRegister(ctx, childRoot)
			childResult <- err
		}()
		<-store.firstListStarted

		ancestorResult := make(chan error, 1)
		go func() {
			_, err := resolver.Register(ctx, RegisterOptions{RootDir: ancestorRoot, Name: "ancestor"})
			ancestorResult <- err
		}()

		select {
		case inserted := <-store.inserted:
			close(store.releaseFirstList)
			t.Fatalf("InsertWorkspace(%q) interleaved before child registration decision completed", inserted)
		case <-time.After(100 * time.Millisecond):
		}
		close(store.releaseFirstList)

		for operation, result := range map[string]<-chan error{
			"ResolveOrRegister(child)": childResult,
			"Register(ancestor)":       ancestorResult,
		} {
			select {
			case err := <-result:
				if err != nil {
					t.Fatalf("%s error = %v", operation, err)
				}
			case <-time.After(5 * time.Second):
				t.Fatalf("%s did not complete", operation)
			}
		}

		firstInserted := <-store.inserted
		secondInserted := <-store.inserted
		if firstInserted != mustCanonicalRoot(t, childRoot) ||
			secondInserted != mustCanonicalRoot(t, ancestorRoot) {
			t.Fatalf(
				"registration order = [%q, %q], want child decision before ancestor insertion",
				firstInserted,
				secondInserted,
			)
		}
	})
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

		cfg := compozyconfig.Config{Defaults: compozyconfig.DefaultsConfig{Agent: compozyconfig.DefaultAgentName}}
		applyDefaultAgentOverride(&cfg, "")
		if cfg.Defaults.Agent != compozyconfig.DefaultAgentName {
			t.Fatalf(
				"Defaults.Agent after empty override = %q, want %q",
				cfg.Defaults.Agent,
				compozyconfig.DefaultAgentName,
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

	t.Run("Should generate secure IDs", func(t *testing.T) {
		t.Parallel()

		got, err := generateID("ws")
		if err != nil {
			t.Fatalf("generateID(ws) error = %v", err)
		}
		if !strings.HasPrefix(got, "ws_") {
			t.Fatalf("generateID(ws) = %q, want ws_ prefix", got)
		}
		if !IsRegistrationID(got) {
			t.Fatalf("IsRegistrationID(%q) = false, want generated workspace id accepted", got)
		}
		for _, invalid := range []string{"", "ws_route", "ws_0123456789ABCDE", "ws_0123456789abcdef0"} {
			if IsRegistrationID(invalid) {
				t.Fatalf("IsRegistrationID(%q) = true, want malformed workspace id rejected", invalid)
			}
		}
		got, err = generateID("")
		if err != nil {
			t.Fatalf("generateID(empty) error = %v", err)
		}
		if got == "" {
			t.Fatal("generateID(\"\") = empty, want non-empty")
		}
	})
}
