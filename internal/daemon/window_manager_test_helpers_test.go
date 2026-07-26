package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/agh/internal/clientstate"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/store/globaldb"
	"github.com/compozy/agh/internal/testutil"
	"github.com/compozy/agh/internal/windowmanager"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

type daemonWindowManagerFixture struct {
	database      *globaldb.GlobalDB
	resolver      *workspacepkg.Resolver
	storeResolver *windowManagerStoreWorkspaceResolver
	engine        *clientstate.Engine
	repository    *windowManagerRepository
	manager       *windowmanager.Manager
	workspace     workspacepkg.Workspace
	homePaths     aghconfig.HomePaths
	storePath     string
}

func newDaemonWindowManagerFixture(t *testing.T) daemonWindowManagerFixture {
	t.Helper()
	database := openDaemonTestGlobalDB(t)
	homePaths := testHomePaths(t)
	resolver, err := workspacepkg.NewResolver(
		database,
		workspacepkg.WithHomePaths(homePaths),
		workspacepkg.WithLogger(discardLogger()),
		workspacepkg.WithConfigLoader(func(rootDir string) (aghconfig.Config, error) {
			return aghconfig.LoadForHome(homePaths, aghconfig.WithWorkspaceRoot(rootDir))
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
	repository, err := newWindowManagerRepository(engine)
	if err != nil {
		t.Fatalf("newWindowManagerRepository() error = %v", err)
	}
	manager, err := windowmanager.NewService(
		repository,
		windowManagerWorkspaceAuthorizer{resolver: storeResolver},
		nil,
		windowmanager.DefaultConfig(),
		windowmanager.WithWorkspaceConfigResolver(windowManagerWorkspaceConfigResolver{resolver: resolver}),
	)
	if err != nil {
		t.Fatalf("windowmanager.NewService() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Close(); err != nil {
			t.Errorf("Manager.Close() error = %v", err)
		}
	})
	return daemonWindowManagerFixture{
		database: database, resolver: resolver, storeResolver: storeResolver,
		engine: engine, repository: repository, manager: manager,
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
			ID: "desktop-default", Name: name, Purpose: windowmanager.DesktopPurposeStandard,
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
