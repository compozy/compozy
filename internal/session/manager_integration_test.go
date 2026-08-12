//go:build integration

package session

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	eventspkg "github.com/compozy/compozy/internal/events"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/sandbox"
	localsandbox "github.com/compozy/compozy/internal/sandbox/local"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/store/sessiondb"
	"github.com/compozy/compozy/internal/testutil"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	worktreepkg "github.com/compozy/compozy/internal/worktree"
	skillbundled "github.com/compozy/compozy/skills"
)

func TestManagerIntegrationWorktreeBindingLifecycle(t *testing.T) {
	h := newHarness(t)
	initializeSessionIntegrationGitRepository(t, h.workspace)
	worktreesRoot := t.TempDir()
	worktreeA := filepath.Join(worktreesRoot, "worktree-a")
	worktreeB := filepath.Join(worktreesRoot, "worktree-b")
	runSessionIntegrationGit(t, h.workspace, "branch", "worktree-a", "main")
	runSessionIntegrationGit(t, h.workspace, "branch", "worktree-b", "main")
	runSessionIntegrationGit(t, h.workspace, "worktree", "add", worktreeA, "worktree-a")
	runSessionIntegrationGit(t, h.workspace, "worktree", "add", worktreeB, "worktree-b")

	missingErr := errors.New("worktree_missing")
	worktreeResolver := &fakeSessionWorktreeResolver{resolve: func(
		_ context.Context,
		_ string,
		ref string,
	) (string, string, error) {
		roots := map[string]string{"wt-a": worktreeA, "wt-b": worktreeB}
		root := roots[strings.TrimSpace(ref)]
		if root == "" {
			return "", "", errors.New("worktree_not_found")
		}
		if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
			return "", "", missingErr
		} else if err != nil {
			return "", "", err
		}
		return strings.TrimSpace(ref), root, nil
	}}
	var contextsMu sync.Mutex
	contexts := make([]StartupPromptContext, 0, 5)
	h.manager = newManagerWithHarness(
		t,
		h,
		WithWorktreeResolver(worktreeResolver),
		WithPromptAssembler(startupPromptAssemblerFunc(func(
			_ context.Context,
			startup StartupPromptContext,
			agent compozyconfig.AgentDef,
			_ *workspacepkg.ResolvedWorkspace,
		) (string, error) {
			contextsMu.Lock()
			contexts = append(contexts, startup)
			contextsMu.Unlock()
			return agent.Prompt, nil
		})),
	)

	parent, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName: "coder", Workspace: h.workspaceID, Worktree: "wt-a",
		Lineage: &store.SessionLineage{
			SpawnBudget: store.SessionSpawnBudget{MaxChildren: 2, MaxDepth: 1},
		},
	})
	if err != nil {
		t.Fatalf("Create(bound parent) error = %v", err)
	}
	sameWorktree, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName: "coder", Workspace: h.workspaceID, Worktree: "wt-a",
	})
	if err != nil {
		t.Fatalf("Create(second same-worktree session) error = %v", err)
	}
	otherWorktree, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName: "coder", Workspace: h.workspaceID, Worktree: "wt-b",
	})
	if err != nil {
		t.Fatalf("Create(other-worktree session) error = %v", err)
	}
	rootSession, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName: "coder", Workspace: h.workspaceID,
	})
	if err != nil {
		t.Fatalf("Create(root session) error = %v", err)
	}
	for _, item := range []*Session{parent, sameWorktree, otherWorktree, rootSession} {
		item := item
		t.Cleanup(func() { stopActiveIntegrationSession(t, h, item.ID) })
	}

	canonicalA := resolveIntegrationWorkspaceRoot(t, worktreeA)
	canonicalB := resolveIntegrationWorkspaceRoot(t, worktreeB)
	if got := h.driver.startCalls[0].Cwd; got != canonicalA {
		t.Fatalf("parent cmd.Dir = %q, want %q", got, canonicalA)
	}
	if got := h.driver.startCalls[1].Cwd; got != canonicalA {
		t.Fatalf("second cmd.Dir = %q, want %q", got, canonicalA)
	}
	if got := h.driver.startCalls[2].Cwd; got != canonicalB {
		t.Fatalf("other cmd.Dir = %q, want %q", got, canonicalB)
	}
	if err := os.WriteFile(filepath.Join(h.driver.startCalls[0].Cwd, "isolated.txt"), []byte("worktree-a\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(worktree A) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(h.driver.startCalls[2].Cwd, "isolated.txt"), []byte("worktree-b\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(worktree B) error = %v", err)
	}
	assertIntegrationFileContent(t, filepath.Join(worktreeA, "isolated.txt"), "worktree-a\n")
	assertIntegrationFileContent(t, filepath.Join(worktreeB, "isolated.txt"), "worktree-b\n")

	child, err := h.manager.Spawn(testutil.Context(t), SpawnOpts{
		ParentSessionID: parent.ID, AgentName: "coder", TTL: time.Minute,
	})
	if err != nil {
		t.Fatalf("Spawn(bound child) error = %v", err)
	}
	t.Cleanup(func() { stopActiveIntegrationSession(t, h, child.ID) })
	if child.Info().WorktreeID != "wt-a" || h.driver.startCalls[len(h.driver.startCalls)-1].Cwd != canonicalA {
		t.Fatalf("bound child = %#v start=%#v, want wt-a at %q", child.Info(), h.driver.startCalls, canonicalA)
	}

	contextsMu.Lock()
	contextsSnapshot := append([]StartupPromptContext(nil), contexts...)
	contextsMu.Unlock()
	if len(contextsSnapshot) < 5 {
		t.Fatalf("startup contexts = %#v, want bound, root, and child contexts", contextsSnapshot)
	}
	for _, index := range []int{0, 1, 2, 4} {
		if contextsSnapshot[index].WorkspaceID != h.workspaceID || contextsSnapshot[index].Workspace != h.workspace {
			t.Fatalf("startup context[%d] = %#v, want parent workspace", index, contextsSnapshot[index])
		}
	}
	if contextsSnapshot[0].WorktreeID != "wt-a" || contextsSnapshot[2].WorktreeID != "wt-b" ||
		contextsSnapshot[3].WorktreeID != "" {
		t.Fatalf("startup worktree contexts = %#v", contextsSnapshot)
	}

	if parent.Info().WorkspaceID != h.workspaceID || rootSession.Info().WorkspaceID != h.workspaceID {
		t.Fatalf("session workspace ids = %q/%q, want %q", parent.Info().WorkspaceID, rootSession.Info().WorkspaceID, h.workspaceID)
	}

	runSessionIntegrationGit(t, h.workspace, "worktree", "remove", "--force", worktreeA)
	startsBeforeRefusal := len(h.driver.startCalls)
	_, err = h.manager.Spawn(testutil.Context(t), SpawnOpts{
		ParentSessionID: parent.ID, AgentName: "coder", TTL: time.Minute,
	})
	if !errors.Is(err, missingErr) {
		t.Fatalf("Spawn(missing worktree) error = %v, want %v", err, missingErr)
	}
	if len(h.driver.startCalls) != startsBeforeRefusal {
		t.Fatalf("driver starts after missing spawn = %d, want %d", len(h.driver.startCalls), startsBeforeRefusal)
	}
	if err := h.manager.Stop(testutil.Context(t), parent.ID); err != nil {
		t.Fatalf("Stop(bound parent) error = %v", err)
	}
	_, err = h.manager.Resume(testutil.Context(t), parent.ID)
	if !errors.Is(err, missingErr) {
		t.Fatalf("Resume(missing worktree) error = %v, want %v", err, missingErr)
	}
}

func TestManagerIntegrationLocalSandboxContainsBoundWorktree(t *testing.T) {
	worktreeRoot := filepath.Join(t.TempDir(), "worktree")
	if err := os.MkdirAll(worktreeRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(worktree root) error = %v", err)
	}
	h := newHarness(
		t,
		WithWorktreeResolver(&fakeSessionWorktreeResolver{id: "wt-sandbox", root: worktreeRoot}),
		WithSandboxRegistry(newRegistryForProvider(t, localsandbox.NewProvider(
			localsandbox.WithPermissionMode(compozyconfig.PermissionModeApproveAll),
		))),
	)
	created, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName: "coder", Workspace: h.workspaceID, Worktree: "wt-sandbox",
	})
	if err != nil {
		t.Fatalf("Create(bound local sandbox) error = %v", err)
	}
	t.Cleanup(func() { stopActiveIntegrationSession(t, h, created.ID) })

	canonicalRoot := resolveIntegrationWorkspaceRoot(t, worktreeRoot)
	if created.Info().Sandbox == nil {
		t.Fatal("sandbox metadata = nil, want local sandbox")
	}
	runtimeRoot := resolveIntegrationWorkspaceRoot(t, created.Info().Sandbox.RuntimeRootDir)
	if runtimeRoot != canonicalRoot {
		t.Fatalf("sandbox metadata = %#v, want runtime root %q", created.Info().Sandbox, canonicalRoot)
	}
	process := created.processHandle()
	if process == nil || process.ToolHost() == nil {
		t.Fatal("bound session tool host is unavailable")
	}
	inside := filepath.Join(worktreeRoot, "nested", "inside.txt")
	if err := process.ToolHost().WriteTextFile(testutil.Context(t), inside, "inside"); err != nil {
		t.Fatalf("WriteTextFile(inside worktree) error = %v", err)
	}
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := process.ToolHost().WriteTextFile(testutil.Context(t), outside, "outside"); !errors.Is(
		err,
		acp.ErrPathOutsideWorkspace,
	) {
		t.Fatalf("WriteTextFile(outside worktree) error = %v, want %v", err, acp.ErrPathOutsideWorkspace)
	}
}

func TestManagerIntegrationWorktreeRemovalBindingRace(t *testing.T) {
	t.Run("Should let an active session binding win before removal", func(t *testing.T) {
		fixture := newSessionWorktreeRaceFixture(t, nil)

		bound, err := fixture.h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder", Workspace: fixture.h.workspaceID, Worktree: fixture.item.ID,
		})
		if err != nil {
			t.Fatalf("Create(bound winner) error = %v", err)
		}
		if got := bound.Info().WorktreeID; got != fixture.item.ID {
			t.Fatalf("bound winner worktree_id = %q, want %q", got, fixture.item.ID)
		}

		if _, err := fixture.service.Remove(
			testutil.Context(t), fixture.h.workspaceID, fixture.item.ID, false,
		); !errors.Is(err, worktreepkg.ErrSessionActive) {
			t.Fatalf("Remove(after binding) error = %v, want %v", err, worktreepkg.ErrSessionActive)
		}
		stored, err := fixture.database.Worktrees.Get(
			testutil.Context(t), fixture.h.workspaceID, fixture.item.ID,
		)
		if err != nil || stored == nil || stored.State != worktreepkg.StateReady {
			t.Fatalf("worktree after binding winner = %#v, %v, want ready", stored, err)
		}

		if err := fixture.h.manager.Stop(testutil.Context(t), bound.ID); err != nil {
			t.Fatalf("Stop(bound winner) error = %v", err)
		}
		if _, err := fixture.service.Remove(
			testutil.Context(t), fixture.h.workspaceID, fixture.item.ID, true,
		); err != nil {
			t.Fatalf("Remove(cleanup) error = %v", err)
		}
	})

	t.Run("Should let the removal fence reject a stale new session binding", func(t *testing.T) {
		hook := newBlockingSessionWorktreeRemovalHook()
		fixture := newSessionWorktreeRaceFixture(t, hook)
		removeDone := make(chan error, 1)
		go func() {
			_, err := fixture.service.Remove(
				context.Background(), fixture.h.workspaceID, fixture.item.ID, false,
			)
			removeDone <- err
		}()

		select {
		case <-hook.entered:
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for removal fence")
		}

		bound, err := fixture.h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder", Workspace: fixture.h.workspaceID, Worktree: fixture.item.ID,
		})
		if bound != nil || !errors.Is(err, worktreepkg.ErrNotReady) {
			t.Fatalf("Create(after removal fence) = (%#v, %v), want nil and %v", bound, err, worktreepkg.ErrNotReady)
		}
		if got := len(fixture.h.driver.startCalls); got != 0 {
			t.Fatalf("driver start calls after rejected binding = %d, want 0", got)
		}
		sessions, listErr := fixture.database.ListSessions(testutil.Context(t), store.SessionListQuery{
			WorkspaceID: fixture.h.workspaceID,
			WorktreeID:  fixture.item.ID,
		})
		if listErr != nil {
			t.Fatalf("ListSessions(rejected binding) error = %v", listErr)
		}
		if len(sessions) != 0 {
			t.Fatalf("sessions after rejected binding = %#v, want none", sessions)
		}

		close(hook.release)
		select {
		case removeErr := <-removeDone:
			if removeErr != nil {
				t.Fatalf("Remove(fenced winner) error = %v", removeErr)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for fenced removal")
		}
		stored, getErr := fixture.database.Worktrees.Get(
			testutil.Context(t), fixture.h.workspaceID, fixture.item.ID,
		)
		if getErr != nil || stored == nil || stored.State != worktreepkg.StateRemoved {
			t.Fatalf("worktree after removal winner = %#v, %v, want removed", stored, getErr)
		}
	})
}

type sessionWorktreeRaceFixture struct {
	h        *harness
	database *globaldb.GlobalDB
	service  *worktreepkg.Service
	item     *worktreepkg.Worktree
}

func newSessionWorktreeRaceFixture(
	t *testing.T,
	hooks worktreepkg.HookDispatcher,
) *sessionWorktreeRaceFixture {
	t.Helper()
	ctx := testutil.Context(t)
	h := newHarness(t)
	if err := h.manager.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown(initial manager) error = %v", err)
	}
	initializeSessionIntegrationGitRepository(t, h.workspace)

	database, err := globaldb.OpenGlobalDB(ctx, filepath.Join(t.TempDir(), store.GlobalDatabaseName))
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(testutil.Context(t)); err != nil {
			t.Errorf("GlobalDB.Close() error = %v", err)
		}
	})
	if err := database.InsertWorkspace(ctx, workspacepkg.Workspace{
		ID: h.workspaceID, Name: h.workspaceName, RootDir: h.workspace,
	}); err != nil {
		t.Fatalf("InsertWorkspace() error = %v", err)
	}

	runner, err := worktreepkg.NewRealGitRunner(10 * time.Second)
	if err != nil {
		t.Fatalf("NewRealGitRunner() error = %v", err)
	}
	settings := compozyconfig.DefaultWorktreesConfig()
	service := worktreepkg.NewService(
		database.Worktrees,
		runner,
		worktreepkg.WithWorkspaceResolver(sessionWorktreeWorkspaceResolver{workspace: worktreepkg.Workspace{
			ID: h.workspaceID, Name: h.workspaceName, Root: h.workspace,
		}}),
		worktreepkg.WithSessionGuard(sessionWorktreeGlobalDBGuard{database: database}),
		worktreepkg.WithConfig(settings, t.TempDir()),
		worktreepkg.WithHooks(hooks),
		worktreepkg.WithIDGenerator(func(string) (string, error) { return "wt-session-race", nil }),
	)
	item, err := service.Create(ctx, h.workspaceID, worktreepkg.CreateOptions{Name: "Session Race"})
	if err != nil {
		t.Fatalf("worktree Create() error = %v", err)
	}
	resolver := &fakeSessionWorktreeResolver{resolve: func(
		_ context.Context,
		_ string,
		_ string,
	) (string, string, error) {
		// Deliberately return the admitted path even after the SQL removal fence.
		// The catalog must still reject the stale new binding atomically.
		return item.ID, item.Path, nil
	}}
	h.manager = newManagerWithHarness(
		t,
		h,
		WithSessionCatalog(database),
		WithWorktreeResolver(resolver),
	)
	cleanupTestManager(t, h.manager)

	return &sessionWorktreeRaceFixture{h: h, database: database, service: service, item: item}
}

type sessionWorktreeWorkspaceResolver struct {
	workspace worktreepkg.Workspace
}

func (r sessionWorktreeWorkspaceResolver) ResolveWorktreeWorkspace(
	_ context.Context,
	workspaceID string,
) (worktreepkg.Workspace, error) {
	if workspaceID != r.workspace.ID {
		return worktreepkg.Workspace{}, worktreepkg.ErrNotFound
	}
	return r.workspace, nil
}

func (r sessionWorktreeWorkspaceResolver) ListWorktreeWorkspaces(
	context.Context,
) ([]worktreepkg.Workspace, error) {
	return []worktreepkg.Workspace{r.workspace}, nil
}

type sessionWorktreeGlobalDBGuard struct {
	database *globaldb.GlobalDB
}

func (g sessionWorktreeGlobalDBGuard) HasActiveSession(
	ctx context.Context,
	workspaceID string,
	worktreeID string,
) (bool, error) {
	return g.database.HasActiveWorktreeSession(ctx, workspaceID, worktreeID)
}

type blockingSessionWorktreeRemovalHook struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func newBlockingSessionWorktreeRemovalHook() *blockingSessionWorktreeRemovalHook {
	return &blockingSessionWorktreeRemovalHook{
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (h *blockingSessionWorktreeRemovalHook) DispatchWorktreeHook(
	ctx context.Context,
	request worktreepkg.HookRequest,
) (worktreepkg.HookVerdict, error) {
	if request.Event != worktreepkg.EventPreRemove {
		return worktreepkg.HookVerdict{}, nil
	}
	h.once.Do(func() { close(h.entered) })
	select {
	case <-h.release:
		return worktreepkg.HookVerdict{}, nil
	case <-ctx.Done():
		return worktreepkg.HookVerdict{}, ctx.Err()
	}
}

func initializeSessionIntegrationGitRepository(t *testing.T, root string) {
	t.Helper()
	runSessionIntegrationGit(t, root, "init", "-b", "main")
	runSessionIntegrationGit(t, root, "config", "user.name", "Compozy Integration")
	runSessionIntegrationGit(t, root, "config", "user.email", "integration@compozy.test")
	if err := os.WriteFile(filepath.Join(root, "shared.txt"), []byte("initial\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(shared.txt) error = %v", err)
	}
	runSessionIntegrationGit(t, root, "add", "shared.txt")
	runSessionIntegrationGit(t, root, "commit", "-m", "initial")
}

func runSessionIntegrationGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.CommandContext(t.Context(), "git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s error = %v; output=%s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func stopActiveIntegrationSession(t *testing.T, h *harness, sessionID string) {
	t.Helper()
	current, ok := h.manager.Get(sessionID)
	if !ok || current.Info().State == StateStopped {
		return
	}
	if err := h.manager.Stop(testutil.Context(t), sessionID); err != nil {
		t.Errorf("Stop(%q) cleanup error = %v", sessionID, err)
	}
}

func assertIntegrationFileContent(t *testing.T, path string, want string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	if string(content) != want {
		t.Fatalf("ReadFile(%q) = %q, want %q", path, content, want)
	}
}

func TestManagerIntegrationFullLifecycle(t *testing.T) {
	h := newHarness(t)

	sessionCWD := filepath.Join(h.workspace, "nested-session-cwd")
	if err := os.MkdirAll(sessionCWD, 0o755); err != nil {
		t.Fatalf("MkdirAll(session CWD) error = %v", err)
	}
	canonicalSessionCWD := resolveIntegrationWorkspaceRoot(t, sessionCWD)
	session, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName: "coder",
		Name:      "session",
		Workspace: h.workspaceID,
		CWD:       sessionCWD,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if got := h.driver.startCalls[0].Cwd; got != canonicalSessionCWD {
		t.Fatalf("Create() CWD = %q, want %q", got, canonicalSessionCWD)
	}
	firstPrompt, err := h.manager.Prompt(testutil.Context(t), session.ID, "first")
	if err != nil {
		t.Fatalf("Prompt(first) error = %v", err)
	}
	firstEvents := collectEvents(t, firstPrompt)
	if len(firstEvents) != 2 {
		t.Fatalf("first prompt events = %d, want 2", len(firstEvents))
	}
	acpSessionIDBeforeStop := session.Info().ACPSessionID
	if strings.TrimSpace(acpSessionIDBeforeStop) == "" {
		t.Fatal("ACP session ID before stop is empty")
	}

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	secondPrompt, err := h.manager.Prompt(testutil.Context(t), session.ID, "second")
	if err != nil {
		t.Fatalf("Prompt(stopped, second) error = %v", err)
	}
	secondEvents := collectEvents(t, secondPrompt)
	if len(secondEvents) != 2 {
		t.Fatalf("second prompt events = %d, want 2", len(secondEvents))
	}
	resumed, ok := h.manager.Get(session.ID)
	if !ok {
		t.Fatalf("Get(%q) found = false, want resumed session", session.ID)
	}
	h.driver.mu.Lock()
	resumeStart := h.driver.startCalls[1]
	h.driver.mu.Unlock()
	if got := resumeStart.Cwd; got != canonicalSessionCWD {
		t.Fatalf("prompt resume CWD = %q, want persisted %q", got, canonicalSessionCWD)
	}
	if got := resumeStart.ResumeSessionID; got != acpSessionIDBeforeStop {
		t.Fatalf("prompt resume ACP session ID = %q, want persisted %q", got, acpSessionIDBeforeStop)
	}

	if err := h.manager.Stop(testutil.Context(t), resumed.ID); err != nil {
		t.Fatalf("final Stop() error = %v", err)
	}

	reopened, err := sessiondb.OpenSessionDB(
		testutil.Context(t),
		testSessionDBOwner(resumed.ID, resumed.WorkspaceID),
		resumed.DBPath(),
	)
	if err != nil {
		t.Fatalf("OpenSessionDB(reopen) error = %v", err)
	}
	defer func() {
		if err := reopened.Close(testutil.Context(t)); err != nil {
			t.Fatalf("reopened.Close() error = %v", err)
		}
	}()

	events, err := reopened.Query(testutil.Context(t), store.EventQuery{})
	if err != nil {
		t.Fatalf("Query(reopen) error = %v", err)
	}
	if len(events) != 10 {
		t.Fatalf("stored events = %d, want 10", len(events))
	}
	if !containsEventType(events, acp.EventTypeAgentMessage) || !containsEventType(events, acp.EventTypeDone) {
		t.Fatalf("stored events missing expected types: %#v", events)
	}
	if got := countEventType(events, EventTypeSessionStopped); got != 2 {
		t.Fatalf("stored %q events = %d, want 2", EventTypeSessionStopped, got)
	}
	if got := countEventType(events, eventspkg.TranscriptMarkerCreated); got != 2 {
		t.Fatalf("stored %q events = %d, want 2", eventspkg.TranscriptMarkerCreated, got)
	}

	meta := readMeta(t, resumed.MetaPath())
	if meta.State != string(StateStopped) {
		t.Fatalf("meta state = %q, want %q", meta.State, StateStopped)
	}
}

func TestManagerIntegrationCapabilityAwareJoinCarriesCatalogAcrossCreateResumeAndStop(t *testing.T) {
	h := newHarness(t)
	lifecycle := newFakeNetworkPeerLifecycle()
	h.manager.SetNetworkPeerLifecycle(lifecycle)

	resolvedSandbox, err := h.cfg.ResolveSandbox(h.cfg.Defaults.Sandbox)
	if err != nil {
		t.Fatalf("ResolveSandbox() error = %v", err)
	}
	capabilityAgent := compozyconfig.AgentDef{
		Name:     "coder",
		Provider: "claude",
		Prompt:   "You are a coding assistant.",
		Capabilities: &compozyconfig.CapabilityCatalog{
			Capabilities: []compozyconfig.CapabilityDef{{
				ID:      "review-pr",
				Summary: "Review pull requests",
				Outcome: "Deliver actionable pull request feedback",
				Version: "1.0.0",
				ContextNeeded: []string{
					"Pull request diff",
					"Acceptance criteria",
				},
				ArtifactsExpected: []string{
					"Review summary",
				},
				Requirements: []string{
					"workspace-write",
					"review-guidelines",
				},
			}},
		},
	}
	if err := capabilityAgent.Validate(); err != nil {
		t.Fatalf("capabilityAgent.Validate() error = %v", err)
	}
	h.resolver.upsert(&workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID:      h.workspaceID,
			RootDir: h.workspace,
			Name:    h.workspaceName,
		},
		Config: h.cfg,
		Agents: []compozyconfig.AgentDef{
			{
				Name:     compozyconfig.DefaultAgentName,
				Provider: "claude",
				Prompt:   "You are a coding assistant.",
			},
			capabilityAgent,
		},
		Sandbox: resolvedSandbox,
	})

	session, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName:                    "coder",
		Name:                         "networked",
		Workspace:                    h.workspaceID,
		ResolvedNetworkParticipation: testLiveParticipationPtr(h.workspaceID, "builders"),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if got := lifecycle.joinCount(); got != 1 {
		t.Fatalf("join count after Create() = %d, want 1", got)
	}
	firstJoin := lifecycle.joinCall(0)
	if got, want := firstJoin.sessionID, session.ID; got != want {
		t.Fatalf("first join session_id = %q, want %q", got, want)
	}
	if got, want := firstJoin.peerID, "coder."+session.ID; got != want {
		t.Fatalf("first join peer_id = %q, want %q", got, want)
	}
	wantDigest, err := compozyconfig.CanonicalCapabilityDigest(compozyconfig.CapabilityDef{
		ID:      "review-pr",
		Summary: "Review pull requests",
		Outcome: "Deliver actionable pull request feedback",
		Version: "1.0.0",
		ContextNeeded: []string{
			"Pull request diff",
			"Acceptance criteria",
		},
		ArtifactsExpected: []string{
			"Review summary",
		},
		Requirements: []string{
			"workspace-write",
			"review-guidelines",
		},
	})
	if err != nil {
		t.Fatalf("CanonicalCapabilityDigest() error = %v", err)
	}
	wantCapabilities := []NetworkPeerCapability{{
		ID:                "review-pr",
		Summary:           "Review pull requests",
		Outcome:           "Deliver actionable pull request feedback",
		Version:           "1.0.0",
		Digest:            wantDigest,
		ContextNeeded:     []string{"Pull request diff", "Acceptance criteria"},
		ArtifactsExpected: []string{"Review summary"},
		Requirements:      []string{"review-guidelines", "workspace-write"},
	}}
	if !reflect.DeepEqual(firstJoin.capabilities, wantCapabilities) {
		t.Fatalf("first join capabilities = %#v, want %#v", firstJoin.capabilities, wantCapabilities)
	}

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if got := lifecycle.leaveCount(); got != 1 {
		t.Fatalf("leave count after Stop() = %d, want 1", got)
	}
	if got, want := lifecycle.leaveCall(0), session.ID; got != want {
		t.Fatalf("leave session_id = %q, want %q", got, want)
	}

	resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	if got := lifecycle.joinCount(); got != 2 {
		t.Fatalf("join count after Resume() = %d, want 2", got)
	}
	secondJoin := lifecycle.joinCall(1)
	if got, want := secondJoin.sessionID, resumed.ID; got != want {
		t.Fatalf("second join session_id = %q, want %q", got, want)
	}
	if got, want := secondJoin.peerID, "coder."+resumed.ID; got != want {
		t.Fatalf("second join peer_id = %q, want %q", got, want)
	}
	if !reflect.DeepEqual(secondJoin.capabilities, wantCapabilities) {
		t.Fatalf("second join capabilities = %#v, want %#v", secondJoin.capabilities, wantCapabilities)
	}

	if err := h.manager.Stop(testutil.Context(t), resumed.ID); err != nil {
		t.Fatalf("final Stop() error = %v", err)
	}
	if got := lifecycle.leaveCount(); got != 2 {
		t.Fatalf("leave count after resumed Stop() = %d, want 2", got)
	}
}

func TestManagerIntegrationCapabilityAwareJoinKeepsMissingCatalogProjectionEmpty(t *testing.T) {
	h := newHarness(t)
	lifecycle := newFakeNetworkPeerLifecycle()
	h.manager.SetNetworkPeerLifecycle(lifecycle)

	session, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName:                    "coder",
		Name:                         "networked",
		Workspace:                    h.workspaceID,
		ResolvedNetworkParticipation: testLiveParticipationPtr(h.workspaceID, "builders"),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	if got := lifecycle.joinCount(); got != 1 {
		t.Fatalf("join count after Create() = %d, want 1", got)
	}
	join := lifecycle.joinCall(0)
	if join.capabilities == nil {
		t.Fatal("join capabilities = nil, want deterministic empty projection")
	}
	if got := len(join.capabilities); got != 0 {
		t.Fatalf("join capabilities len = %d, want 0", got)
	}
}

func TestManagerIntegrationCapabilityProjectionDoesNotAliasSourceCatalog(t *testing.T) {
	h := newHarness(t)

	resolvedSandbox, err := h.cfg.ResolveSandbox(h.cfg.Defaults.Sandbox)
	if err != nil {
		t.Fatalf("ResolveSandbox() error = %v", err)
	}
	capabilityAgent := compozyconfig.AgentDef{
		Name:     "coder",
		Provider: "claude",
		Prompt:   "You are a coding assistant.",
		Capabilities: &compozyconfig.CapabilityCatalog{
			Capabilities: []compozyconfig.CapabilityDef{{
				ID:               "review-pr",
				Summary:          "Review pull requests",
				Outcome:          "Deliver actionable pull request feedback",
				Version:          "1.0.0",
				ContextNeeded:    []string{"Pull request diff", "Acceptance criteria"},
				ExecutionOutline: []string{"inspect", "comment"},
				Requirements:     []string{"workspace-write", "review-guidelines"},
			}},
		},
	}
	if err := capabilityAgent.Validate(); err != nil {
		t.Fatalf("capabilityAgent.Validate() error = %v", err)
	}
	sourceBefore := capabilityAgent.Capabilities.Clone()

	h.manager.SetNetworkPeerLifecycle(&mutatingNetworkPeerLifecycle{})
	h.resolver.upsert(&workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID:      h.workspaceID,
			RootDir: h.workspace,
			Name:    h.workspaceName,
		},
		Config: h.cfg,
		Agents: []compozyconfig.AgentDef{
			{
				Name:     compozyconfig.DefaultAgentName,
				Provider: "claude",
				Prompt:   "You are a coding assistant.",
			},
			capabilityAgent,
		},
		Sandbox: resolvedSandbox,
	})

	session, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName:                    "coder",
		Name:                         "networked",
		Workspace:                    h.workspaceID,
		ResolvedNetworkParticipation: testLiveParticipationPtr(h.workspaceID, "builders"),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	if !reflect.DeepEqual(capabilityAgent.Capabilities, sourceBefore) {
		t.Fatalf(
			"source capability catalog mutated through join projection:\nbefore=%#v\nafter=%#v",
			sourceBefore,
			capabilityAgent.Capabilities,
		)
	}
}

type mutatingNetworkPeerLifecycle struct{}

func (m *mutatingNetworkPeerLifecycle) JoinChannel(_ context.Context, join NetworkPeerJoin) error {
	if len(join.Capabilities) == 0 {
		return nil
	}

	join.Capabilities[0].Summary = "mutated summary"
	join.Capabilities[0].Digest = "sha256:mutated"
	if len(join.Capabilities[0].ContextNeeded) > 0 {
		join.Capabilities[0].ContextNeeded[0] = "mutated context"
	}
	if len(join.Capabilities[0].ExecutionOutline) > 0 {
		join.Capabilities[0].ExecutionOutline[0] = "mutated execution outline"
	}
	if len(join.Capabilities[0].Requirements) > 0 {
		join.Capabilities[0].Requirements[0] = "mutated requirement"
	}

	return nil
}

func (m *mutatingNetworkPeerLifecycle) LeaveChannel(context.Context, string) error {
	return nil
}

func TestManagerIntegrationUsesRealSQLitePerSessionDB(t *testing.T) {
	h := newHarness(t)

	session := createSession(t, h)
	eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "persist")
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	_ = collectEvents(t, eventsCh)

	recorder, ok := session.recorderHandle().(*sessiondb.SessionDB)
	if !ok {
		t.Fatalf("recorder = %T, want *sessiondb.SessionDB", session.recorderHandle())
	}
	if got, want := recorder.Path(), session.DBPath(); got != want {
		t.Fatalf("SessionDB.Path() = %q, want %q", got, want)
	}

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	reopened, err := sessiondb.OpenSessionDB(
		testutil.Context(t),
		testSessionDBOwner(session.ID, session.WorkspaceID),
		session.DBPath(),
	)
	if err != nil {
		t.Fatalf("OpenSessionDB(reopen) error = %v", err)
	}
	defer func() {
		if err := reopened.Close(testutil.Context(t)); err != nil {
			t.Fatalf("reopened.Close() error = %v", err)
		}
	}()

	events, err := reopened.Query(testutil.Context(t), store.EventQuery{})
	if err != nil {
		t.Fatalf("Query(reopen) error = %v", err)
	}
	if len(events) == 0 {
		t.Fatal("Query(reopen) returned 0 events, want persisted rows")
	}
}

func TestManagerIntegrationSyntheticPromptPersistsDedicatedEventsWithMixedHistory(t *testing.T) {
	h := newHarness(t)

	session := createLiveNetworkSession(t, h)
	userEvents, err := h.manager.Prompt(testutil.Context(t), session.ID, "user prompt")
	if err != nil {
		t.Fatalf("Prompt(user) error = %v", err)
	}
	_ = collectEvents(t, userEvents)

	networkEvents, err := h.manager.PromptNetwork(
		testutil.Context(t),
		session.ID,
		"network prompt",
		acp.PromptNetworkMeta{MessageID: "msg-1", Kind: "direct"},
	)
	if err != nil {
		t.Fatalf("PromptNetwork() error = %v", err)
	}
	_ = collectEvents(t, networkEvents)

	syntheticEvents, err := h.manager.PromptSynthetic(testutil.Context(t), session.ID, SyntheticPromptOpts{
		Message: "synthetic wake-up",
		Metadata: acp.PromptSyntheticMeta{
			TaskRunID: "run-1",
			Reason:    "task_run_completed",
			Summary:   "background task finished",
		},
	})
	if err != nil {
		t.Fatalf("PromptSynthetic() error = %v", err)
	}
	_ = collectEvents(t, syntheticEvents)

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	reopened, err := sessiondb.OpenSessionDB(
		testutil.Context(t),
		testSessionDBOwner(session.ID, session.WorkspaceID),
		session.DBPath(),
	)
	if err != nil {
		t.Fatalf("OpenSessionDB(reopen) error = %v", err)
	}
	defer func() {
		if err := reopened.Close(testutil.Context(t)); err != nil {
			t.Fatalf("reopened.Close() error = %v", err)
		}
	}()

	events, err := reopened.Query(testutil.Context(t), store.EventQuery{})
	if err != nil {
		t.Fatalf("Query(reopen) error = %v", err)
	}
	if got := countEventType(events, acp.EventTypeUserMessage); got != 2 {
		t.Fatalf("countEventType(user_message) = %d, want 2 for user+network input", got)
	}
	if got := countEventType(events, acp.EventTypeSyntheticReentry); got != 1 {
		t.Fatalf("countEventType(synthetic_reentry) = %d, want 1", got)
	}
	if !containsEventType(events, acp.EventTypeAgentMessage) || !containsEventType(events, acp.EventTypeDone) {
		t.Fatalf("mixed history missing runtime events: %#v", events)
	}
}

func TestManagerIntegrationSyntheticQueuePreservesOrderingBehindActivePrompt(t *testing.T) {
	h := newHarness(t)

	session := createSession(t, h)

	firstPromptEntered := make(chan struct{})
	releaseFirstPrompt := make(chan struct{})
	h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
		if req.TurnID == "turn-1" {
			close(firstPromptEntered)
			events := make(chan acp.AgentEvent)
			go func() {
				<-releaseFirstPrompt
				events <- acp.AgentEvent{
					Type:      acp.EventTypeDone,
					TurnID:    req.TurnID,
					Timestamp: time.Now().UTC(),
				}
				close(events)
			}()
			return events, nil
		}

		return completedSyntheticPromptEvents(req.TurnID), nil
	}

	userEvents, err := h.manager.Prompt(testutil.Context(t), session.ID, "user prompt")
	if err != nil {
		t.Fatalf("Prompt(user) error = %v", err)
	}
	<-firstPromptEntered

	syntheticEvents, err := h.manager.PromptSynthetic(testutil.Context(t), session.ID, SyntheticPromptOpts{
		Message: "synthetic wake-up",
		Metadata: acp.PromptSyntheticMeta{
			TaskRunID: "run-2",
			Reason:    "task_run_completed",
			Summary:   "queued after user turn",
		},
	})
	if err != nil {
		t.Fatalf("PromptSynthetic() error = %v", err)
	}

	close(releaseFirstPrompt)
	_ = collectEvents(t, userEvents)
	_ = collectEvents(t, syntheticEvents)

	events, err := session.recorderHandle().Query(testutil.Context(t), store.EventQuery{})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(events) < 3 {
		t.Fatalf("stored events = %d, want at least user, done, and synthetic events", len(events))
	}

	userIndex := -1
	doneIndex := -1
	syntheticIndex := -1
	for i, event := range events {
		switch event.Type {
		case acp.EventTypeUserMessage:
			if userIndex < 0 {
				userIndex = i
			}
		case acp.EventTypeDone:
			if doneIndex < 0 {
				doneIndex = i
			}
		case acp.EventTypeSyntheticReentry:
			if syntheticIndex < 0 {
				syntheticIndex = i
			}
		}
	}
	if userIndex < 0 {
		t.Fatalf("stored events missing %q: %#v", acp.EventTypeUserMessage, events)
	}
	if doneIndex < 0 {
		t.Fatalf("stored events missing %q: %#v", acp.EventTypeDone, events)
	}
	if syntheticIndex < 0 {
		t.Fatalf("stored events missing %q: %#v", acp.EventTypeSyntheticReentry, events)
	}
	if !(userIndex < doneIndex && doneIndex < syntheticIndex) {
		t.Fatalf(
			"event order user=%d done=%d synthetic=%d, want user < done < synthetic",
			userIndex,
			doneIndex,
			syntheticIndex,
		)
	}

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("cleanup Stop() error = %v", err)
	}
}

func TestManagerIntegrationRemovePurgesSyntheticState(t *testing.T) {
	tests := []struct {
		name   string
		remove func(*Manager, string)
	}{
		{
			name: "Should purge synthetic state on remove",
			remove: func(m *Manager, id string) {
				m.remove(id)
			},
		},
		{
			name: "Should purge synthetic state on removeActive",
			remove: func(m *Manager, id string) {
				m.removeActive(id)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			eventsCh := make(chan acp.AgentEvent, 1)
			finalizing := make(chan struct{})
			manager := &Manager{
				sessions: map[string]*Session{
					"sess-synth": {ID: "sess-synth"},
				},
				pending: map[string]sessionReservation{
					"sess-synth": {},
				},
				finalizing: map[string]*sessionFinalization{
					"sess-synth": {done: finalizing},
				},
				syntheticQueues: map[string][]queuedSyntheticPrompt{
					"sess-synth": {{
						request: promptRequest{target: "sess-synth", turnID: "turn-synth"},
						out:     eventsCh,
					}},
				},
				syntheticDispatching: map[string]bool{
					"sess-synth": true,
				},
			}

			tc.remove(manager, "sess-synth")

			manager.syntheticMu.Lock()
			if got := len(manager.syntheticQueues["sess-synth"]); got != 0 {
				manager.syntheticMu.Unlock()
				t.Fatalf("len(syntheticQueues[\"sess-synth\"]) = %d, want 0", got)
			}
			if manager.syntheticDispatching["sess-synth"] {
				manager.syntheticMu.Unlock()
				t.Fatal("syntheticDispatching[\"sess-synth\"] = true, want cleared")
			}
			manager.syntheticMu.Unlock()

			event, ok := <-eventsCh
			if !ok {
				t.Fatal("queued synthetic output closed without error event")
			}
			if got, want := event.Type, acp.EventTypeError; got != want {
				t.Fatalf("queued synthetic event type = %q, want %q", got, want)
			}
			if got, want := event.TurnID, "turn-synth"; got != want {
				t.Fatalf("queued synthetic event turn id = %q, want %q", got, want)
			}
			if !strings.Contains(event.Error, "synthetic prompt dropped") {
				t.Fatalf("queued synthetic error = %q, want drop summary", event.Error)
			}
			if _, ok := <-eventsCh; ok {
				t.Fatal("queued synthetic output channel left open after removal")
			}
		})
	}
}

func TestManagerIntegrationSyntheticQueueStateTransitions(t *testing.T) {
	t.Run("Should requeue a claimed synthetic prompt before clearing dispatch", func(t *testing.T) {
		t.Parallel()

		manager := &Manager{
			syntheticQueues: map[string][]queuedSyntheticPrompt{
				"sess-synth": {{
					request: promptRequest{turnID: "turn-queued"},
				}},
			},
			syntheticDispatching: map[string]bool{
				"sess-synth": true,
			},
		}
		claimed := queuedSyntheticPrompt{request: promptRequest{turnID: "turn-claimed"}}

		manager.requeueSyntheticPromptFrontAndFinishDispatch("sess-synth", claimed)

		manager.syntheticMu.Lock()
		defer manager.syntheticMu.Unlock()

		if manager.syntheticDispatching["sess-synth"] {
			t.Fatal("syntheticDispatching[\"sess-synth\"] = true, want cleared")
		}
		queue := manager.syntheticQueues["sess-synth"]
		if got, want := len(queue), 2; got != want {
			t.Fatalf("len(syntheticQueues[\"sess-synth\"]) = %d, want %d", got, want)
		}
		if got, want := queue[0].request.turnID, "turn-claimed"; got != want {
			t.Fatalf("queue[0].request.turnID = %q, want %q", got, want)
		}
		if got, want := queue[1].request.turnID, "turn-queued"; got != want {
			t.Fatalf("queue[1].request.turnID = %q, want %q", got, want)
		}
	})

	t.Run("Should drain queued synthetic prompts while clearing dispatch", func(t *testing.T) {
		t.Parallel()

		manager := &Manager{
			syntheticQueues: map[string][]queuedSyntheticPrompt{
				"sess-synth": {
					{request: promptRequest{turnID: "turn-1"}},
					{request: promptRequest{turnID: "turn-2"}},
				},
			},
			syntheticDispatching: map[string]bool{
				"sess-synth": true,
			},
		}

		drained := manager.finishQueuedSyntheticDispatchAndDrain("sess-synth")

		manager.syntheticMu.Lock()
		defer manager.syntheticMu.Unlock()

		if manager.syntheticDispatching["sess-synth"] {
			t.Fatal("syntheticDispatching[\"sess-synth\"] = true, want cleared")
		}
		if got := len(manager.syntheticQueues["sess-synth"]); got != 0 {
			t.Fatalf("len(syntheticQueues[\"sess-synth\"]) = %d, want 0", got)
		}
		if got, want := len(drained), 2; got != want {
			t.Fatalf("len(drained) = %d, want %d", got, want)
		}
		if got, want := drained[0].request.turnID, "turn-1"; got != want {
			t.Fatalf("drained[0].request.turnID = %q, want %q", got, want)
		}
		if got, want := drained[1].request.turnID, "turn-2"; got != want {
			t.Fatalf("drained[1].request.turnID = %q, want %q", got, want)
		}
	})
}

func TestResolveWorkspaceSessionAgentGuardsNilInputs(t *testing.T) {
	t.Run("Should reject a nil resolved workspace", func(t *testing.T) {
		t.Parallel()

		_, err := resolveWorkspaceSessionAgentForType("coder", "", "", nil, nil)
		if err == nil {
			t.Fatal("resolveWorkspaceSessionAgent(nil workspace) error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "resolved workspace is required") {
			t.Fatalf("resolveWorkspaceSessionAgent(nil workspace) error = %v", err)
		}
	})

	t.Run("Should allow a nil agent resolver when a workspace is provided", func(t *testing.T) {
		t.Parallel()

		homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}

		resolvedWorkspace := &workspacepkg.ResolvedWorkspace{
			Config: compozyconfig.DefaultWithHome(homePaths),
			Agents: []compozyconfig.AgentDef{{
				Name:     "coder",
				Provider: "claude",
				Prompt:   "You are a coding assistant.",
			}},
		}

		resolved, err := resolveWorkspaceSessionAgentForType("coder", "", "", resolvedWorkspace, nil)
		if err != nil {
			t.Fatalf("resolveWorkspaceSessionAgentForType(nil agent resolver) error = %v", err)
		}
		if got, want := resolved.Provider, "claude"; got != want {
			t.Fatalf("resolveWorkspaceSessionAgentForType(nil agent resolver) provider = %q, want %q", got, want)
		}
	})
}

func TestManagerIntegrationResumeWithChannelReinjectsBundledNetworkSkillBeforeACPStart(t *testing.T) {
	h := newHarness(t)
	networkSkill, err := skillbundled.LoadResource(testBundledCompozySkillName, testBundledNetworkReference)
	if err != nil {
		t.Fatalf("LoadResource(%q, %q) error = %v", testBundledCompozySkillName, testBundledNetworkReference, err)
	}
	networkSkill = strings.TrimSpace(networkSkill)

	session, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName:                    "coder",
		Name:                         "networked",
		Workspace:                    h.workspaceID,
		ResolvedNetworkParticipation: testLiveParticipationPtr(h.workspaceID, "builders"),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if got := strings.Count(h.driver.startCalls[0].SystemPrompt, networkSkill); got != 1 {
		t.Fatalf("create prompt network skill occurrences = %d, want 1", got)
	}

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	t.Cleanup(func() {
		reportSessionStop(t, h, resumed.ID)
	})

	if got := h.driver.startCalls[1].SystemPrompt; !strings.Contains(got, networkSkill) {
		t.Fatalf("resume system prompt = %q, want bundled network skill content", got)
	}
	if got := strings.Count(h.driver.startCalls[1].SystemPrompt, networkSkill); got != 1 {
		t.Fatalf("resume prompt network skill occurrences = %d, want 1", got)
	}
}

func TestManagerIntegrationFullLifecycleHooksFireInOrder(t *testing.T) {
	var (
		mu    sync.Mutex
		order []string
	)
	record := func(entry string) {
		mu.Lock()
		defer mu.Unlock()
		order = append(order, entry)
	}

	dispatcher := &spyHookDispatcher{
		dispatchSessionPreCreateFn: func(_ context.Context, payload hookspkg.SessionPreCreatePayload) (hookspkg.SessionPreCreatePayload, error) {
			record("session.pre_create")
			return payload, nil
		},
		dispatchPromptPostAssembleFn: func(_ context.Context, payload hookspkg.PromptPayload) (hookspkg.PromptPayload, error) {
			record("prompt.post_assemble")
			return payload, nil
		},
		dispatchAgentPreStartFn: func(_ context.Context, payload hookspkg.AgentPreStartPayload) (hookspkg.AgentPreStartPayload, error) {
			record("agent.pre_start")
			return payload, nil
		},
		dispatchAgentSpawnedFn: func(_ context.Context, payload hookspkg.AgentSpawnedPayload) (hookspkg.AgentSpawnedPayload, error) {
			record("agent.spawned")
			return payload, nil
		},
		dispatchSessionPostCreateFn: func(_ context.Context, payload hookspkg.SessionPostCreatePayload) (hookspkg.SessionPostCreatePayload, error) {
			record("session.post_create")
			return payload, nil
		},
		dispatchInputPreSubmitFn: func(_ context.Context, payload hookspkg.InputPreSubmitPayload) (hookspkg.InputPreSubmitPayload, error) {
			record("input.pre_submit")
			return payload, nil
		},
		dispatchTurnStartFn: func(_ context.Context, payload hookspkg.TurnStartPayload) (hookspkg.TurnStartPayload, error) {
			record("turn.start")
			return payload, nil
		},
		dispatchTurnEndFn: func(_ context.Context, payload hookspkg.TurnEndPayload) (hookspkg.TurnEndPayload, error) {
			record("turn.end")
			return payload, nil
		},
		dispatchMessageStartFn: func(_ context.Context, payload hookspkg.MessageStartPayload) (hookspkg.MessageStartPayload, error) {
			record("message.start")
			return payload, nil
		},
		dispatchMessageDeltaFn: func(_ context.Context, payload hookspkg.MessageDeltaPayload) (hookspkg.MessageDeltaPayload, error) {
			record("message.delta:" + payload.DeltaType)
			return payload, nil
		},
		dispatchMessageEndFn: func(_ context.Context, payload hookspkg.MessageEndPayload) (hookspkg.MessageEndPayload, error) {
			record("message.end")
			return payload, nil
		},
		dispatchEventPreRecordFn: func(_ context.Context, payload hookspkg.EventPreRecordPayload) (hookspkg.EventPreRecordPayload, error) {
			record("event.pre_record:" + payload.RecordType)
			return payload, nil
		},
		dispatchEventPostRecordFn: func(_ context.Context, payload hookspkg.EventPostRecordPayload) (hookspkg.EventPostRecordPayload, error) {
			record("event.post_record:" + payload.RecordType)
			return payload, nil
		},
		dispatchSessionPreStopFn: func(_ context.Context, payload hookspkg.SessionPreStopPayload) (hookspkg.SessionPreStopPayload, error) {
			record("session.pre_stop")
			return payload, nil
		},
		dispatchAgentStoppedFn: func(_ context.Context, payload hookspkg.AgentStoppedPayload) (hookspkg.AgentStoppedPayload, error) {
			record("agent.stopped")
			return payload, nil
		},
		dispatchSessionPostStopFn: func(_ context.Context, payload hookspkg.SessionPostStopPayload) (hookspkg.SessionPostStopPayload, error) {
			record("session.post_stop")
			return payload, nil
		},
	}

	h := newHarness(t, WithHookSet(fullHookSet(dispatcher)))

	session := createSession(t, h)
	eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "hello")
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	_ = collectEvents(t, eventsCh)
	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	want := []string{
		"session.pre_create",
		"prompt.post_assemble",
		"agent.pre_start",
		"agent.spawned",
		"session.post_create",
		"input.pre_submit",
		"turn.start",
		"event.pre_record:user_message",
		"event.post_record:user_message",
		"message.start",
		"message.delta:text",
		"event.pre_record:agent_message",
		"event.post_record:agent_message",
		"message.end",
		"event.pre_record:done",
		"event.post_record:done",
		"turn.end",
		"session.pre_stop",
		"event.pre_record:session_stopped",
		"event.post_record:session_stopped",
		"event.pre_record:transcript_marker.created",
		"event.post_record:transcript_marker.created",
		"agent.stopped",
		"session.post_stop",
	}

	mu.Lock()
	got := append([]string(nil), order...)
	mu.Unlock()
	if !testutil.EqualStringSlices(got, want) {
		t.Fatalf("hook order = %#v, want %#v", got, want)
	}
}

func TestManagerIntegrationSandboxNativeHooksLifecycleOrder(t *testing.T) {
	var (
		mu        sync.Mutex
		order     []string
		afterTo   = make(chan struct{})
		ready     = make(chan struct{})
		afterFrom = make(chan struct{})
	)
	record := func(event string) {
		mu.Lock()
		defer mu.Unlock()
		order = append(order, event)
	}
	waitFor := func(ctx context.Context, ch <-chan struct{}, label string) error {
		select {
		case <-ch:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
			return errors.New("timed out waiting for " + label)
		}
	}

	hooks := newNativeHookDispatcher(t,
		[]hookspkg.HookDecl{
			{
				Name:         "env-prepare",
				Event:        hookspkg.HookSandboxPrepare,
				Mode:         hookspkg.HookModeSync,
				ExecutorKind: hookspkg.HookExecutorNative,
			},
			{
				Name:         "env-sync-before",
				Event:        hookspkg.HookSandboxSyncBefore,
				Mode:         hookspkg.HookModeSync,
				ExecutorKind: hookspkg.HookExecutorNative,
			},
			{
				Name:         "env-sync-after",
				Event:        hookspkg.HookSandboxSyncAfter,
				Mode:         hookspkg.HookModeAsync,
				ExecutorKind: hookspkg.HookExecutorNative,
			},
			{
				Name:         "env-ready",
				Event:        hookspkg.HookSandboxReady,
				Mode:         hookspkg.HookModeAsync,
				ExecutorKind: hookspkg.HookExecutorNative,
			},
			{
				Name:         "env-stop",
				Event:        hookspkg.HookSandboxStop,
				Mode:         hookspkg.HookModeSync,
				ExecutorKind: hookspkg.HookExecutorNative,
			},
		},
		map[string]hookspkg.Executor{
			"env-prepare": hookspkg.NewTypedNativeExecutor(
				func(_ context.Context, _ hookspkg.RegisteredHook, payload hookspkg.SandboxPreparePayload) (hookspkg.SandboxPreparePatch, error) {
					if payload.SandboxID == "" || payload.WorkspaceID == "" {
						return hookspkg.SandboxPreparePatch{}, errors.New("sandbox.prepare missing identity fields")
					}
					record("sandbox.prepare")
					return hookspkg.SandboxPreparePatch{}, nil
				},
			),
			"env-sync-before": hookspkg.NewTypedNativeExecutor(
				func(_ context.Context, _ hookspkg.RegisteredHook, payload hookspkg.SandboxSyncBeforePayload) (hookspkg.SandboxSyncBeforePatch, error) {
					if payload.SandboxID == "" || payload.Direction == "" || payload.Reason == "" {
						return hookspkg.SandboxSyncBeforePatch{}, errors.New(
							"sandbox.sync.before missing lifecycle fields",
						)
					}
					record("sandbox.sync.before:" + payload.Direction)
					return hookspkg.SandboxSyncBeforePatch{}, nil
				},
			),
			"env-sync-after": hookspkg.NewTypedNativeExecutor(
				func(_ context.Context, _ hookspkg.RegisteredHook, payload hookspkg.SandboxSyncAfterPayload) (hookspkg.SandboxSyncAfterPatch, error) {
					if payload.SandboxID == "" || payload.Direction == "" || payload.DurationMS < 0 {
						return hookspkg.SandboxSyncAfterPatch{}, errors.New(
							"sandbox.sync.after missing lifecycle fields",
						)
					}
					record("sandbox.sync.after:" + payload.Direction)
					switch payload.Direction {
					case string(sandbox.SyncDirectionToRuntime):
						close(afterTo)
					case string(sandbox.SyncDirectionFromRuntime):
						close(afterFrom)
					default:
						return hookspkg.SandboxSyncAfterPatch{}, errors.New(
							"unexpected sync direction " + payload.Direction,
						)
					}
					return hookspkg.SandboxSyncAfterPatch{}, nil
				},
			),
			"env-ready": hookspkg.NewTypedNativeExecutor(
				func(ctx context.Context, _ hookspkg.RegisteredHook, payload hookspkg.SandboxReadyPayload) (hookspkg.SandboxReadyPatch, error) {
					if err := waitFor(ctx, afterTo, "sandbox.sync.after:to_runtime"); err != nil {
						return hookspkg.SandboxReadyPatch{}, err
					}
					if payload.SandboxID == "" || payload.RuntimeRootDir == "" {
						return hookspkg.SandboxReadyPatch{}, errors.New("sandbox.ready missing runtime fields")
					}
					record("sandbox.ready")
					close(ready)
					return hookspkg.SandboxReadyPatch{}, nil
				},
			),
			"env-stop": hookspkg.NewTypedNativeExecutor(
				func(ctx context.Context, _ hookspkg.RegisteredHook, payload hookspkg.SandboxStopPayload) (hookspkg.SandboxStopPatch, error) {
					if err := waitFor(ctx, afterFrom, "sandbox.sync.after:from_runtime"); err != nil {
						return hookspkg.SandboxStopPatch{}, err
					}
					if payload.SandboxID == "" || payload.StopReason == "" {
						return hookspkg.SandboxStopPatch{}, errors.New("sandbox.stop missing stop fields")
					}
					record("sandbox.stop")
					return hookspkg.SandboxStopPatch{}, nil
				},
			),
		},
	)

	h := newHarness(t, WithHookSet(HookSet{Sandbox: hooks}))
	session := createSession(t, h)
	if err := waitFor(testutil.Context(t), ready, "sandbox.ready"); err != nil {
		t.Fatalf("waiting for sandbox.ready: %v", err)
	}
	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	want := []string{
		"sandbox.prepare",
		"sandbox.sync.before:to_runtime",
		"sandbox.sync.after:to_runtime",
		"sandbox.ready",
		"sandbox.sync.before:from_runtime",
		"sandbox.sync.after:from_runtime",
		"sandbox.stop",
	}
	mu.Lock()
	got := append([]string(nil), order...)
	mu.Unlock()
	if !testutil.EqualStringSlices(got, want) {
		t.Fatalf("sandbox hook order = %#v, want %#v", got, want)
	}
}

func TestManagerIntegrationContextCompactionUsesPatchedParams(t *testing.T) {
	reason := "patched-reason"
	strategy := "patched-strategy"
	postSeen := make(chan hookspkg.ContextPostCompactPayload, 1)

	hooks := newNativeHookDispatcher(t,
		[]hookspkg.HookDecl{
			{
				Name:         "context-pre",
				Event:        hookspkg.HookContextPreCompact,
				Mode:         hookspkg.HookModeSync,
				ExecutorKind: hookspkg.HookExecutorNative,
			},
			{
				Name:         "context-post",
				Event:        hookspkg.HookContextPostCompact,
				Mode:         hookspkg.HookModeAsync,
				ExecutorKind: hookspkg.HookExecutorNative,
			},
		},
		map[string]hookspkg.Executor{
			"context-pre": hookspkg.NewTypedNativeExecutor(
				func(_ context.Context, _ hookspkg.RegisteredHook, payload hookspkg.ContextPreCompactPayload) (hookspkg.ContextPreCompactPatch, error) {
					return hookspkg.ContextPreCompactPatch{
						Reason:   &reason,
						Strategy: &strategy,
					}, nil
				},
			),
			"context-post": hookspkg.NewTypedNativeExecutor(
				func(_ context.Context, _ hookspkg.RegisteredHook, payload hookspkg.ContextPostCompactPayload) (hookspkg.ContextPostCompactPatch, error) {
					postSeen <- payload
					return hookspkg.ContextPostCompactPatch{}, nil
				},
			),
		},
	)

	h := newHarness(t, WithHookSet(fullHookSet(hooks)))
	session := createSession(t, h)
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	var seen hookspkg.ContextPreCompactPayload
	result, err := h.manager.runContextCompaction(
		testutil.Context(t),
		session,
		"turn-context",
		"manual",
		"noop",
		"",
		nil,
		func(_ context.Context, payload hookspkg.ContextPreCompactPayload) (hookspkg.ContextPostCompactPayload, error) {
			seen = payload
			return hookspkg.ContextPostCompactPayload{
				Summary: "after",
			}, nil
		},
	)
	if err != nil {
		t.Fatalf("runContextCompaction() error = %v", err)
	}
	if seen.Reason != reason || seen.Strategy != strategy {
		t.Fatalf("compactor saw reason/strategy = %q/%q, want %q/%q", seen.Reason, seen.Strategy, reason, strategy)
	}
	if result.Reason != reason || result.Strategy != strategy {
		t.Fatalf("result reason/strategy = %q/%q, want %q/%q", result.Reason, result.Strategy, reason, strategy)
	}
	select {
	case payload := <-postSeen:
		if payload.Reason != reason || payload.Strategy != strategy {
			t.Fatalf(
				"post hook saw reason/strategy = %q/%q, want %q/%q",
				payload.Reason,
				payload.Strategy,
				reason,
				strategy,
			)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for context.post_compact hook")
	}
}

func TestManagerIntegrationPreStopRequiredHookErrorPreventsCleanStop(t *testing.T) {
	hooks := newNativeHookDispatcher(t,
		[]hookspkg.HookDecl{{
			Name:         "required-pre-stop",
			Event:        hookspkg.HookSessionPreStop,
			Mode:         hookspkg.HookModeSync,
			Required:     true,
			ExecutorKind: hookspkg.HookExecutorNative,
		}},
		map[string]hookspkg.Executor{
			"required-pre-stop": hookspkg.NewTypedNativeExecutor(
				func(_ context.Context, _ hookspkg.RegisteredHook, _ hookspkg.SessionPreStopPayload) (hookspkg.SessionPreStopPatch, error) {
					return hookspkg.SessionPreStopPatch{}, errors.New("required hook failed")
				},
			),
		},
	)

	h := newHarness(t, WithHookSet(fullHookSet(hooks)))
	session := createSession(t, h)

	err := h.manager.Stop(testutil.Context(t), session.ID)
	if err == nil {
		t.Fatal("Stop() error = nil, want required pre-stop hook failure")
	}
	if got := session.Info().State; got != StateActive {
		t.Fatalf("session state after failed Stop() = %q, want %q", got, StateActive)
	}
	if _, ok := h.manager.Get(session.ID); !ok {
		t.Fatalf("Get(%q) = missing, want active session after failed stop", session.ID)
	}

	h.manager.hooks = HookSet{}
	if cleanupErr := h.manager.Stop(testutil.Context(t), session.ID); cleanupErr != nil {
		t.Fatalf("cleanup Stop() error = %v", cleanupErr)
	}
}
