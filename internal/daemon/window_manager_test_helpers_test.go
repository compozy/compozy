package daemon

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/clientstate"
	compozyconfig "github.com/compozy/compozy/internal/config"
	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/windowmanager"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

const testWindowManagerProfileID = "01JQPROFILEDEFAULT00000000"

type daemonWindowManagerFixture struct {
	database      *globaldb.GlobalDB
	resolver      *workspacepkg.Resolver
	storeResolver *windowManagerStoreWorkspaceResolver
	engine        *clientstate.Engine
	registry      *windowManagerRegistry
	repository    *windowManagerRepository
	manager       *windowmanager.Manager
	workspace     workspacepkg.Workspace
	homePaths     compozyconfig.HomePaths
	storePath     string
}

type windowManagerWorkspaceStoreStub struct {
	workspacepkg.Store
	err error
}

func (s *windowManagerWorkspaceStoreStub) GetWorkspace(context.Context, string) (workspacepkg.Workspace, error) {
	return workspacepkg.Workspace{}, s.err
}

func (s *windowManagerWorkspaceStoreStub) GetWorkspaceByName(context.Context, string) (workspacepkg.Workspace, error) {
	return workspacepkg.Workspace{}, s.err
}

func newDaemonWindowManagerFixture(t *testing.T) daemonWindowManagerFixture {
	t.Helper()
	database := openDaemonTestGlobalDB(t)
	homePaths := testHomePaths(t)
	resolver, err := workspacepkg.NewResolver(
		database,
		workspacepkg.WithHomePaths(homePaths),
		workspacepkg.WithLogger(discardLogger()),
		workspacepkg.WithConfigLoader(func(rootDir string) (compozyconfig.Config, error) {
			return compozyconfig.LoadForHome(homePaths, compozyconfig.WithWorkspaceRoot(rootDir))
		}),
	)
	if err != nil {
		t.Fatalf("workspace.NewResolver() error = %v", err)
	}
	root := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", root, err)
	}
	workspace, err := resolver.Register(testutil.Context(t), workspacepkg.RegisterOptions{
		RootDir: root,
		Name:    "window-manager",
	})
	if err != nil {
		t.Fatalf("workspace.Register() error = %v", err)
	}
	storeResolver, err := newWindowManagerStoreWorkspaceResolver(resolver, discardLogger())
	if err != nil {
		t.Fatalf("newWindowManagerStoreWorkspaceResolver() error = %v", err)
	}
	storePath := clientstate.DatabasePath(t.TempDir())
	engine, err := clientstate.Open(
		testutil.Context(t),
		storePath,
		storeResolver,
		clientstate.Limits{
			MaxValueBytes:       windowManagerMaxSnapshotBytes,
			MaxKeysPerWorkspace: clientstate.DefaultLimits().MaxKeysPerWorkspace,
		},
		clientstate.WithLogger(discardLogger()),
	)
	if err != nil {
		t.Fatalf("clientstate.Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := engine.Close(); err != nil {
			t.Errorf("Engine.Close() error = %v", err)
		}
	})
	registry, err := newWindowManagerRegistry(
		engine,
		windowManagerWorkspaceAuthorizer{resolver: storeResolver},
		nil,
		resolver,
		windowmanager.DefaultConfig(),
		discardLogger(),
		windowmanager.WithWorkspaceConfigResolver(windowManagerWorkspaceConfigResolver{resolver: resolver}),
	)
	if err != nil {
		t.Fatalf("newWindowManagerRegistry() error = %v", err)
	}
	t.Cleanup(func() {
		if err := registry.Close(); err != nil {
			t.Errorf("windowManagerRegistry.Close() error = %v", err)
		}
	})
	manager, err := registry.For(testWindowManagerProfileID)
	if err != nil {
		t.Fatalf("windowManagerRegistry.For() error = %v", err)
	}
	repository, err := newWindowManagerRepository(engine, testWindowManagerProfileID)
	if err != nil {
		t.Fatalf("newWindowManagerRepository() error = %v", err)
	}
	return daemonWindowManagerFixture{
		database: database, resolver: resolver, storeResolver: storeResolver,
		engine: engine, registry: registry, repository: repository, manager: manager,
		workspace: workspace, homePaths: homePaths, storePath: storePath,
	}
}

func writeDaemonWindowManagerConfig(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", path, err)
	}
}

func daemonWindowManagerSnapshot(
	workspaceID windowmanager.WorkspaceID,
	revision windowmanager.Revision,
	name string,
) windowmanager.Snapshot {
	return windowmanager.Snapshot{
		Version: windowmanager.SnapshotVersion, WorkspaceID: workspaceID, Revision: revision,
		Desktops: []windowmanager.Desktop{{
			ID: "desktop-default", Name: name,
			Groups: []windowmanager.LayoutGroup{}, Floating: []windowmanager.WindowID{},
		}},
		Windows:   map[windowmanager.WindowID]windowmanager.Window{},
		History:   windowmanager.History{Undo: []windowmanager.HistoryEntry{}, Redo: []windowmanager.HistoryEntry{}},
		UpdatedAt: time.Date(2026, time.July, 22, 12, 0, int(revision), 0, time.UTC),
	}
}

func daemonWindowManagerCommit(
	snapshot windowmanager.Snapshot,
	expected windowmanager.Revision,
) *windowmanager.Commit {
	return &windowmanager.Commit{
		WorkspaceID: snapshot.WorkspaceID, ExpectedRevision: expected, Snapshot: snapshot,
		Event: windowmanager.Event{
			WorkspaceID: snapshot.WorkspaceID, Revision: snapshot.Revision,
			CommandID: windowmanager.CommandDesktopUpdate, Origin: "daemon-test",
			OccurredAt: snapshot.UpdatedAt,
		},
	}
}

func closeDaemonWindowManagerEngine(t *testing.T, engine *clientstate.Engine) {
	t.Helper()
	if err := engine.Close(); err != nil {
		t.Fatalf("Engine.Close() error = %v", err)
	}
}

func contextWithoutCancellation(t *testing.T) context.Context {
	t.Helper()
	return context.WithoutCancel(testutil.Context(t))
}

// staticWindowManagers serves one manager for every profile. Tests that assert the
// per-profile partition build a real registry instead.
type staticWindowManagers struct {
	manager *windowmanager.Manager
}

var _ windowManagerProvider = staticWindowManagers{}

func (s staticWindowManagers) For(string) (*windowmanager.Manager, error) {
	return s.manager, nil
}

// newDaemonTestProfileManager builds the profile domain over the fixture's catalog,
// wired to the same window managers the daemon composes.
func newDaemonTestProfileManager(
	t *testing.T,
	fixture *daemonWindowManagerFixture,
) *profilepkg.Manager {
	t.Helper()
	manager, err := profilepkg.NewManager(
		profilepkg.WithStore(fixture.database),
		profilepkg.WithHomePaths(fixture.homePaths),
		profilepkg.WithLogger(discardLogger()),
		profilepkg.WithDesktopPartitionCatalog(fixture.registry),
	)
	if err != nil {
		t.Fatalf("profile.NewManager() error = %v", err)
	}
	return manager
}

// seedInterruptedProfileDelete recreates the crash window a delete leaves behind:
// applied and committed, with its desktop purge still owed.
func seedInterruptedProfileDelete(
	ctx context.Context,
	t *testing.T,
	fixture *daemonWindowManagerFixture,
	profileID string,
) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := fixture.database.DB().ExecContext(
		ctx,
		`INSERT INTO profile_lifecycle_ops
		 (id, kind, profile_id, old_name, plan_revision, status, created_at, updated_at)
		 VALUES ('op_crashed_delete', 'delete', ?, 'doomed', 'revision', 'applied', ?, ?)`,
		profileID, now, now,
	); err != nil {
		t.Fatalf("seed lifecycle operation error = %v", err)
	}
	if _, err := fixture.database.DB().ExecContext(
		ctx,
		`INSERT INTO profile_lifecycle_op_steps (op_id, seq, action, status, updated_at)
		 VALUES ('op_crashed_delete', 0, 'remove_desktop_partitions', 'pending', ?)`,
		now,
	); err != nil {
		t.Fatalf("seed lifecycle step error = %v", err)
	}
}

// newTestWindowManagerRegistry builds a second registry over one fixture's store,
// so a test can interpose on the client-state service the profiles write through.
func newTestWindowManagerRegistry(
	t *testing.T,
	fixture *daemonWindowManagerFixture,
	service clientstate.Service,
) *windowManagerRegistry {
	t.Helper()
	return newTestWindowManagerRegistryWithAuthorizer(
		t, fixture, service, windowManagerWorkspaceAuthorizer{resolver: fixture.storeResolver},
	)
}

// newTestWindowManagerRegistryWithAuthorizer additionally interposes on workspace
// authorization, which is where a claim can be held open mid-flight.
func newTestWindowManagerRegistryWithAuthorizer(
	t *testing.T,
	fixture *daemonWindowManagerFixture,
	service clientstate.Service,
	authorizer windowmanager.WorkspaceResolver,
) *windowManagerRegistry {
	t.Helper()
	registry, err := newWindowManagerRegistry(
		service,
		authorizer,
		nil,
		fixture.resolver,
		windowmanager.DefaultConfig(),
		discardLogger(),
		windowmanager.WithWorkspaceConfigResolver(
			windowManagerWorkspaceConfigResolver{resolver: fixture.resolver},
		),
	)
	if err != nil {
		t.Fatalf("newWindowManagerRegistry() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := registry.Close(); closeErr != nil {
			t.Errorf("windowManagerRegistry.Close() error = %v", closeErr)
		}
	})
	return registry
}

func executeDaemonDesktopCreate(
	t *testing.T,
	manager *windowmanager.Manager,
	workspaceID windowmanager.WorkspaceID,
	desktopID windowmanager.DesktopID,
	name string,
) windowmanager.Result {
	t.Helper()
	snapshot, err := manager.Snapshot(testutil.Context(t), workspaceID)
	if err != nil {
		t.Fatalf("Snapshot(%s) error = %v", workspaceID, err)
	}
	result, err := manager.Execute(testutil.Context(t), windowmanager.CommandRequest{
		WorkspaceID:      workspaceID,
		ExpectedRevision: snapshot.Revision,
		Payload:          windowmanager.CreateDesktopCommand{DesktopID: desktopID, Name: name},
	})
	if err != nil {
		t.Fatalf("Execute(desktop.create %s) error = %v", desktopID, err)
	}
	return result
}

func assertDaemonDesktopNames(t *testing.T, snapshot windowmanager.Snapshot, want []string) {
	t.Helper()
	got := make([]string, 0, len(snapshot.Desktops))
	for _, desktop := range snapshot.Desktops {
		got = append(got, desktop.Name)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("desktop names = %v, want %v", got, want)
	}
}
