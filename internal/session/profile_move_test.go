package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/worktree"
)

const (
	profileMoveTargetID   = "01HZPROFILEMOVETARGET00000"
	profileMoveTargetName = "work"
	profileMoveBranch     = "feat/profile-move"
)

// fakeProfileMoveWorktrees stands in for the daemon worktree service at the I/O
// boundary: it resolves, binds, releases, and recreates checkouts by id.
type fakeProfileMoveWorktrees struct {
	mu        sync.Mutex
	db        *globaldb.GlobalDB
	workspace string
	base      string
	worktrees map[string]*fakeMoveWorktree
	next      int
	dirty     int
	// failCreate refuses CreateProfileMoveWorktree for these profile ids.
	failCreate map[string]error
	releases   []string
	// onInspect runs while the move plans, before the archive.
	onInspect func()
	onRelease func()
	// failAfterRelease is returned once the checkout was already removed.
	failAfterRelease error
	// failRelease is returned before anything is removed.
	failRelease error
	// inspectErrs fails inspections by call number (1-based).
	inspectErrs map[int]error
	inspects    int
	// stopAt names the step whose goroutine stops, as a daemon stop would leave it.
	stopAt  string
	creates map[string]int
}

type fakeMoveWorktree struct {
	root, profileID, branch string
	released                bool
	// stuck is a checkout that is neither ready nor gone and still holds its branch.
	stuck bool
}

func newFakeProfileMoveWorktrees(
	t *testing.T, db *globaldb.GlobalDB, workspaceID string,
) *fakeProfileMoveWorktrees {
	t.Helper()
	return &fakeProfileMoveWorktrees{
		db: db, workspace: workspaceID, base: t.TempDir(),
		worktrees: map[string]*fakeMoveWorktree{}, failCreate: map[string]error{}, creates: map[string]int{},
	}
}

// step stops the calling goroutine at the scripted step: deferred unlocks run, and
// nothing after the step does, which is the state a daemon stop leaves behind.
func (f *fakeProfileMoveWorktrees) step(name string) {
	f.mu.Lock()
	stop := f.stopAt == name
	if stop {
		f.stopAt = ""
	}
	f.mu.Unlock()
	if stop {
		runtime.Goexit()
	}
}

func (f *fakeProfileMoveWorktrees) add(t *testing.T, profileID string) string {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	id, _, err := f.addLocked(profileID, "")
	if err != nil {
		t.Fatalf("add worktree: %v", err)
	}
	return id
}

func (f *fakeProfileMoveWorktrees) addLocked(profileID, id string) (string, string, error) {
	f.next++
	if id == "" {
		id = fmt.Sprintf("wt-%d", f.next)
	}
	if _, exists := f.worktrees[id]; exists {
		return "", "", fmt.Errorf("worktree %s already exists", id)
	}
	root := filepath.Join(f.base, id)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", "", err
	}
	// Sessions reference their worktree row, so every checkout is registered.
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	if err := f.db.Worktrees.Insert(context.Background(), worktree.Worktree{
		ID: id, ProfileID: profileID, WorkspaceID: f.workspace, Name: id, Branch: profileMoveBranch,
		Path: root, State: worktree.StateReady, Origin: worktree.OriginManual, SetupState: worktree.SetupNone,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		return "", "", err
	}
	f.worktrees[id] = &fakeMoveWorktree{root: root, profileID: profileID, branch: profileMoveBranch}
	return id, root, nil
}

func (f *fakeProfileMoveWorktrees) get(id string) (fakeMoveWorktree, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	item, ok := f.worktrees[id]
	if !ok {
		return fakeMoveWorktree{}, false
	}
	return *item, true
}

func (f *fakeProfileMoveWorktrees) ResolveSessionWorktree(
	_ context.Context, _ string, ref string,
) (string, string, error) {
	item, ok := f.get(ref)
	if !ok || item.released {
		return "", "", fmt.Errorf("worktree %s is missing", ref)
	}
	return ref, item.root, nil
}

func (f *fakeProfileMoveWorktrees) AcquireSessionWorktreeBinding(
	_ context.Context, _ string, profileID, ref string,
) (string, string, func(), error) {
	item, ok := f.get(ref)
	if !ok || item.released || item.profileID != profileID {
		return "", "", nil, fmt.Errorf("worktree %s is not available to profile %s", ref, profileID)
	}
	return ref, item.root, func() {}, nil
}

func (f *fakeProfileMoveWorktrees) AcquireSessionWorktreeUnbinding(
	context.Context, string, string, string,
) (func(), error) {
	return func() {}, nil
}

func (f *fakeProfileMoveWorktrees) InspectProfileMoveWorktree(
	_ context.Context, _ string, worktreeID string,
) (ProfileMoveCheckout, error) {
	item, ok := f.get(worktreeID)
	if !ok {
		return ProfileMoveCheckout{}, fmt.Errorf("%w: %s", ErrProfileMoveWorktreeNotFound, worktreeID)
	}
	f.mu.Lock()
	onInspect := f.onInspect
	f.inspects++
	inspectErr := f.inspectErrs[f.inspects]
	f.mu.Unlock()
	if inspectErr != nil {
		return ProfileMoveCheckout{}, inspectErr
	}
	if onInspect != nil {
		onInspect()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return ProfileMoveCheckout{
		ID: worktreeID, Branch: item.branch, HeadSHA: "abc123", Ready: !item.released && !item.stuck,
		Gone:       item.released,
		DirtyFiles: f.dirty,
	}, nil
}

func (f *fakeProfileMoveWorktrees) ReleaseProfileMoveWorktree(_ context.Context, _ string, worktreeID string) error {
	f.step("release")
	f.mu.Lock()
	onRelease := f.onRelease
	f.mu.Unlock()
	if onRelease != nil {
		onRelease()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	item, ok := f.worktrees[worktreeID]
	if !ok {
		return fmt.Errorf("worktree %s not found", worktreeID)
	}
	if err := f.failRelease; err != nil {
		f.failRelease = nil
		return err
	}
	item.released = true
	f.releases = append(f.releases, worktreeID)
	if err := f.failAfterRelease; err != nil {
		f.failAfterRelease = nil
		return err
	}
	return nil
}

func (f *fakeProfileMoveWorktrees) CreateProfileMoveWorktree(
	_ context.Context, _ string, profileID, worktreeID string, checkout ProfileMoveCheckout,
) (string, string, error) {
	f.step("create:" + profileID)
	id, root, err := f.createLocked(profileID, worktreeID, checkout)
	if err == nil {
		f.step("created:" + profileID)
	}
	return id, root, err
}

func (f *fakeProfileMoveWorktrees) createLocked(
	profileID, worktreeID string, checkout ProfileMoveCheckout,
) (string, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.failCreate[profileID]; err != nil {
		return "", "", err
	}
	for _, item := range f.worktrees {
		if !item.released && item.branch == checkout.Branch {
			return "", "", fmt.Errorf("branch %s is still checked out", checkout.Branch)
		}
	}
	f.creates[profileID]++
	return f.addLocked(profileID, worktreeID)
}

func (f *fakeProfileMoveWorktrees) releaseCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.releases)
}

type profileMoveHarness struct {
	*harness
	db        *globaldb.GlobalDB
	worktrees *fakeProfileMoveWorktrees
}

// archiveFailingCatalog commits an archive and then reports a failure, and can also
// fail the archive-state reads that follow, as a store that loses its connection would.
type archiveFailingCatalog struct {
	*globaldb.GlobalDB
	archiveErr error
	readErr    error
}

func (c *archiveFailingCatalog) SetSessionArchived(
	ctx context.Context, workspaceID string, sessionID string, archived bool,
) (store.SessionInfo, error) {
	info, err := c.GlobalDB.SetSessionArchived(ctx, workspaceID, sessionID, archived)
	if err == nil && archived && c.archiveErr != nil {
		return info, c.archiveErr
	}
	return info, err
}

func (c *archiveFailingCatalog) SessionArchivedAt(
	ctx context.Context, workspaceID string, sessionID string,
) (*time.Time, error) {
	if c.readErr != nil {
		if archived, err := c.GlobalDB.SessionArchivedAt(ctx, workspaceID, sessionID); err == nil && archived != nil {
			return nil, c.readErr
		}
	}
	return c.GlobalDB.SessionArchivedAt(ctx, workspaceID, sessionID)
}

type profileMoveBindingFailingCatalog struct {
	*globaldb.GlobalDB
	failID        string
	onDescendants func(context.Context)
	cause         error
}

func (c *profileMoveBindingFailingCatalog) UpdateSessionWorktreeBinding(
	ctx context.Context,
	update store.SessionWorktreeBindingUpdate,
) error {
	if update.SessionID == c.failID {
		return c.cause
	}
	return c.GlobalDB.UpdateSessionWorktreeBinding(ctx, update)
}

func (c *profileMoveBindingFailingCatalog) RecoverSessionWorktreeBinding(
	ctx context.Context,
	update store.SessionWorktreeBindingUpdate,
) error {
	if update.SessionID == c.failID {
		return c.cause
	}
	return c.GlobalDB.RecoverSessionWorktreeBinding(ctx, update)
}

func (c *profileMoveBindingFailingCatalog) ListSessionDescendants(
	ctx context.Context,
	workspaceID, sessionID string,
) ([]store.SessionInfo, error) {
	children, err := c.GlobalDB.ListSessionDescendants(ctx, workspaceID, sessionID)
	if err == nil && c.onDescendants != nil {
		c.onDescendants(ctx)
	}
	return children, err
}

func newProfileMoveHarness(t *testing.T) *profileMoveHarness {
	t.Helper()
	return newProfileMoveHarnessWith(t, nil)
}

func newProfileMoveHarnessWith(
	t *testing.T,
	catalog func(*globaldb.GlobalDB) store.SessionCatalog,
) *profileMoveHarness {
	t.Helper()
	h := newHarness(t)
	db := openManagerInputQueueStore(t)
	registerManagerInputQueueWorkspace(t, db, h)
	if _, err := db.DB().ExecContext(testutil.Context(t),
		`INSERT INTO profiles (id, name, color, icon, state, created_at) VALUES (?, ?, 'blue', 'user', 'active', ?)`,
		profileMoveTargetID, profileMoveTargetName, "2026-11-03T00:00:00Z",
	); err != nil {
		t.Fatalf("insert target profile: %v", err)
	}
	worktrees := newFakeProfileMoveWorktrees(t, db, h.workspaceID)
	var sessionCatalog store.SessionCatalog = db
	if catalog != nil {
		sessionCatalog = catalog(db)
	}
	h.manager = newManagerWithHarness(
		t, h,
		WithSessionCatalog(sessionCatalog), WithEventLedger(db), WithSessionPromptAdmissionStore(db),
		WithWorktreeResolver(worktrees),
		WithProfileNameResolver(profileNameResolverMap{profileMoveTargetID: profileMoveTargetName}),
	)
	cleanupTestManager(t, h.manager)
	return &profileMoveHarness{harness: h, db: db, worktrees: worktrees}
}

// stoppedSource creates a session on worktreeID (none when empty), runs one
// settled turn so the copy has a conversation to carry, and stops it.
func (h *profileMoveHarness) stoppedSource(
	t *testing.T,
	worktreeID string,
	parentID string,
	spawnRole ...string,
) *Info {
	t.Helper()
	opts := CreateOpts{AgentName: "coder", Name: "Migration cleanup", Workspace: h.workspaceID, Worktree: worktreeID}
	if parentID != "" {
		opts.Lineage = &store.SessionLineage{ParentSessionID: parentID}
		if len(spawnRole) > 0 && spawnRole[0] != "" {
			opts.Lineage.SpawnRole = spawnRole[0]
			opts.Lineage.Kind = store.LineageKindSpawn
			opts.Type = SessionTypeSpawned
			parent, err := h.manager.Status(t.Context(), parentID)
			if err != nil {
				t.Fatal(err)
			}
			opts.Lineage.RootSessionID = parent.Lineage.RootSessionID
			opts.Lineage.SpawnDepth = parent.Lineage.SpawnDepth + 1
		}
	}
	created, err := h.manager.Create(testutil.Context(t), &opts)
	if err != nil {
		t.Fatalf("Create(source) error = %v", err)
	}
	events, err := h.manager.Prompt(testutil.Context(t), created.ID, "Start the migration")
	if err != nil {
		t.Fatalf("Prompt(source) error = %v", err)
	}
	collectEvents(t, events)
	if err := h.manager.Stop(testutil.Context(t), created.ID); err != nil {
		t.Fatalf("Stop(source) error = %v", err)
	}
	info, err := h.manager.Status(testutil.Context(t), created.ID)
	if err != nil {
		t.Fatalf("Status(source) error = %v", err)
	}
	return info
}

func (h *profileMoveHarness) move(ctx context.Context, sessionID string) (MoveToProfileResult, error) {
	return h.manager.MoveToProfile(ctx, MoveToProfileOpts{
		WorkspaceID: h.workspaceID, ProfileID: store.DefaultProfileID,
		SessionID: sessionID, TargetProfileID: profileMoveTargetID,
	})
}

func (h *profileMoveHarness) archivedAt(t *testing.T, sessionID string) bool {
	t.Helper()
	archived, err := h.db.SessionArchivedAt(testutil.Context(t), h.workspaceID, sessionID)
	if err != nil {
		t.Fatalf("SessionArchivedAt(%q) error = %v", sessionID, err)
	}
	return archived != nil
}

// TestManagerMoveToProfile verifies eligibility and recovery against one persistent session family.
func TestManagerMoveToProfile(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should copy the session into the target profile on a new worktree and archive the original",
		func(t *testing.T) {
			t.Parallel()

			h := newProfileMoveHarness(t)
			sourceWorktree := h.worktrees.add(t, store.DefaultProfileID)
			source := h.stoppedSource(t, sourceWorktree, "")

			result, err := h.move(testutil.Context(t), source.ID)
			if err != nil {
				t.Fatalf("MoveToProfile() error = %v", err)
			}
			copied := result.Copy
			if copied == nil || copied.ID == source.ID || copied.ProfileID != profileMoveTargetID {
				t.Fatalf("copy = %#v, want a new session owned by %s", copied, profileMoveTargetID)
			}
			if copied.Name != source.Name || copied.AgentName != source.AgentName {
				t.Fatalf(
					"copy name/agent = %q/%q, want %q/%q",
					copied.Name,
					copied.AgentName,
					source.Name,
					source.AgentName,
				)
			}
			lineage := store.NormalizeSessionLineage(copied.ID, copied.Lineage)
			if lineage == nil || lineage.Kind != store.LineageKindFork || lineage.ParentSessionID != source.ID {
				t.Fatalf("copy lineage = %#v, want a fork of %s", copied.Lineage, source.ID)
			}
			if copied.Derivation == nil || copied.Derivation.SourceSessionID != source.ID {
				t.Fatalf("copy derivation = %#v, want the source conversation carried", copied.Derivation)
			}
			copySession, ok := h.manager.Get(copied.ID)
			if !ok {
				t.Fatalf("Get(%q) did not find the copy", copied.ID)
			}
			imported := readMeta(t, copySession.MetaPath()).ImportedContext
			if imported == nil || imported.SourceSessionID != source.ID || imported.MessageCount == 0 {
				t.Fatalf("copy imported context = %+v, want messages carried from %s", imported, source.ID)
			}
			target, ok := h.worktrees.get(result.WorktreeID)
			if !ok || copied.WorktreeID != result.WorktreeID || target.profileID != profileMoveTargetID ||
				target.branch != profileMoveBranch || result.Branch != profileMoveBranch {
				t.Fatalf("copy worktree = %q (%#v, branch %q), want a target-profile checkout of %s",
					copied.WorktreeID, target, result.Branch, profileMoveBranch)
			}
			if released, _ := h.worktrees.get(sourceWorktree); !released.released {
				t.Fatalf("source worktree %s was not released", sourceWorktree)
			}
			if result.Original == nil || result.Original.ID != source.ID || !h.archivedAt(t, source.ID) {
				t.Fatalf("original = %#v, want %s archived", result.Original, source.ID)
			}
			original, err := h.manager.Status(testutil.Context(t), source.ID)
			if err != nil || original.ProfileID != store.DefaultProfileID {
				t.Fatalf("original after move = %#v, %v; want it kept in its own profile", original, err)
			}
			if _, err := h.manager.Unarchive(t.Context(), h.workspaceID, source.ID); err != nil {
				t.Fatal(err)
			}
			restored, err := h.manager.Status(t.Context(), source.ID)
			if err != nil || restored.WorktreeID != "" {
				t.Fatalf("childless source binding after unarchive = %#v, %v", restored, err)
			}
			if _, err := h.manager.Resume(t.Context(), source.ID); err != nil {
				t.Fatal(err)
			}
			if err := h.manager.Stop(t.Context(), source.ID); err != nil {
				t.Fatal(err)
			}
		},
	)

	t.Run("Should retain both a concurrent rename and completed move provenance", func(t *testing.T) {
		t.Parallel()
		h := newProfileMoveHarness(t)
		source := h.stoppedSource(t, h.worktrees.add(t, store.DefaultProfileID), "")
		if _, err := h.manager.Archive(t.Context(), h.workspaceID, source.ID); err != nil {
			t.Fatal(err)
		}
		renameRead, releaseRename := make(chan struct{}), make(chan struct{})
		release := sync.OnceFunc(func() { close(releaseRename) })
		t.Cleanup(release)
		gate := make(chan struct{}, 1)
		metaPath := store.SessionMetaFile(filepath.Join(h.manager.homePaths.SessionsDir, source.ID))
		h.manager.readSessionMeta = func(path string) (store.SessionMeta, error) {
			meta, err := store.ReadSessionMeta(path)
			if err == nil && path == metaPath {
				select {
				case gate <- struct{}{}:
					close(renameRead)
					<-releaseRename
				default:
				}
			}
			return meta, err
		}
		renameDone := make(chan error, 1)
		go func() {
			_, err := h.manager.Rename(t.Context(), h.workspaceID, source.ID, "Renamed original")
			renameDone <- err
		}()
		select {
		case <-renameRead:
		case err := <-renameDone:
			t.Fatalf("rename completed before the coordinated metadata read: %v", err)
		}
		recordDone := make(chan error, 1)
		go func() {
			recordDone <- h.manager.recordCompletedProfileMove(t.Context(), &store.SessionProfileMove{SessionID: source.ID, WorkspaceID: h.workspaceID, ProfileID: store.DefaultProfileID, SourceWorktreeID: source.WorktreeID})
		}()
		timer := time.NewTimer(time.Second)
		defer timer.Stop()
		finished := false
		select {
		case err := <-recordDone:
			if err != nil {
				t.Error(err)
			}
			finished = true
		case <-timer.C:
		}
		release()
		if err := <-renameDone; err != nil {
			t.Fatal(err)
		}
		if !finished {
			if err := <-recordDone; err != nil {
				t.Fatal(err)
			}
		}
		persisted, err := store.ReadSessionMeta(metaPath)
		if err != nil || persisted.Name != "Renamed original" || persisted.CompletedProfileMove == nil ||
			persisted.CompletedProfileMove.SourceWorktreeID != source.WorktreeID {
			t.Fatalf("concurrent metadata = %#v, %v", persisted, err)
		}
	})

	t.Run("Should move with archived descendants while preserving their profile and lineage", func(t *testing.T) {
		t.Parallel()

		h := newProfileMoveHarness(t)
		source := h.stoppedSource(t, "", "")
		child := h.stoppedSource(t, "", source.ID)
		grandchild := h.stoppedSource(t, "", child.ID)
		for _, descendant := range []*Info{grandchild, child} {
			if _, err := h.manager.Archive(t.Context(), h.workspaceID, descendant.ID); err != nil {
				t.Fatalf("Archive(descendant) error = %v", err)
			}
		}
		histories := make(map[string][]store.TurnHistory)
		for _, descendant := range []*Info{child, grandchild} {
			history, err := h.manager.History(t.Context(), descendant.ID, store.EventQuery{})
			if err != nil || len(history) == 0 {
				t.Fatalf("History(descendant) = %v, %v; want setup conversation", history, err)
			}
			histories[descendant.ID] = history
		}
		result, err := h.move(t.Context(), source.ID)
		if err != nil {
			t.Fatalf("MoveToProfile() error = %v", err)
		}
		if result.Copy.ProfileID != profileMoveTargetID || !h.archivedAt(t, source.ID) {
			t.Fatalf("move result = %#v; want target copy and archived original", result)
		}
		for _, original := range []*Info{child, grandchild} {
			history, err := h.manager.History(t.Context(), original.ID, store.EventQuery{})
			if err != nil || !reflect.DeepEqual(history, histories[original.ID]) {
				t.Fatalf("descendant history changed: %v, %v", history, err)
			}
			persisted, err := h.manager.Status(t.Context(), original.ID)
			if err != nil {
				t.Fatalf("Status(descendant) error = %v", err)
			}
			if !h.archivedAt(t, original.ID) || persisted.ProfileID != store.DefaultProfileID ||
				persisted.State != StateStopped || persisted.Lineage.ParentSessionID != original.Lineage.ParentSessionID {
				t.Fatalf("descendant changed after move: %#v", persisted)
			}
		}
	})

	for _, fail := range []bool{false, true} {
		t.Run(
			fmt.Sprintf("Should preserve the stopped subagent archive family with move failure=%t", fail),
			func(t *testing.T) {
				t.Parallel()

				h := newProfileMoveHarness(t)
				sourceWorktree := h.worktrees.add(t, store.DefaultProfileID)
				source := h.stoppedSource(t, sourceWorktree, "")
				child := h.stoppedSource(t, sourceWorktree, source.ID, store.SubagentSpawnRole)
				grandchild := h.stoppedSource(t, sourceWorktree, child.ID, store.SubagentSpawnRole)
				isolatedWorktree := h.worktrees.add(t, store.DefaultProfileID)
				h.worktrees.mu.Lock()
				h.worktrees.worktrees[isolatedWorktree].branch = "feat/isolated-subagent"
				h.worktrees.mu.Unlock()
				isolated := h.stoppedSource(t, isolatedWorktree, source.ID, store.SubagentSpawnRole)
				histories := make(map[string][]store.TurnHistory)
				for _, member := range []*Info{source, child, grandchild, isolated} {
					history, err := h.manager.History(t.Context(), member.ID, store.EventQuery{})
					if err != nil || len(history) == 0 {
						t.Fatalf("History() = %v, %v; want setup conversation", history, err)
					}
					histories[member.ID] = history
				}
				if _, err := h.manager.Archive(
					t.Context(),
					h.workspaceID,
					child.ID,
				); !errors.Is(
					err,
					store.ErrSubagentArchiveFollowsParent,
				) {
					t.Fatalf("Archive(subagent) = %v; want follows parent", err)
				}
				cause := errors.New("target checkout failed")
				if fail {
					h.worktrees.failCreate[profileMoveTargetID] = cause
				}
				result, err := h.move(t.Context(), source.ID)
				if fail {
					if !errors.Is(err, cause) {
						t.Fatalf("MoveToProfile() = %v; want checkout failure", err)
					}
				} else if err != nil || result.Copy.ProfileID != profileMoveTargetID {
					t.Fatalf("MoveToProfile() = %#v, %v; want target copy", result, err)
				}
				if !fail {
					for _, member := range []*Info{source, child, grandchild, isolated} {
						if !h.archivedAt(t, member.ID) {
							t.Fatal("successful move left a family member unarchived")
						}
					}
					if _, err := h.manager.Unarchive(
						t.Context(),
						" "+h.workspaceID+" ",
						" "+source.ID+" ",
					); err != nil {
						t.Fatal(err)
					}
				}
				root, err := h.manager.Status(t.Context(), source.ID)
				if err != nil {
					t.Fatal(err)
				}
				for _, member := range []*Info{source, child, grandchild, isolated} {
					persisted, err := h.manager.Status(t.Context(), member.ID)
					if err != nil || h.archivedAt(t, member.ID) ||
						persisted.ProfileID != store.DefaultProfileID ||
						persisted.State != StateStopped ||
						!reflect.DeepEqual(persisted.Lineage, member.Lineage) {
						t.Fatalf("family member = %#v, %v; want original profile, state and lineage", persisted, err)
					}
					wantBinding := root.WorktreeID
					if member.ID == isolated.ID {
						wantBinding = isolatedWorktree
					}
					if persisted.WorktreeID != wantBinding {
						t.Fatalf("family binding = %q, want root binding %q", persisted.WorktreeID, wantBinding)
					}
					if fail {
						if checkout, ok := h.worktrees.get(persisted.WorktreeID); !ok || checkout.released {
							t.Fatal("rollback left a removed family checkout")
						}
					} else if member.ID != isolated.ID && persisted.WorktreeID != "" {
						t.Fatal("unarchive retained removed checkout")
					}
					history, err := h.manager.History(t.Context(), member.ID, store.EventQuery{})
					if err != nil || !reflect.DeepEqual(history, histories[member.ID]) {
						t.Fatalf("family history changed: %v, %v", history, err)
					}
				}
				for _, member := range []*Info{source, child, grandchild, isolated} {
					if _, err := h.manager.Resume(t.Context(), member.ID); err != nil {
						t.Fatalf("Resume(family) = %v", err)
					}
					if err := h.manager.Stop(t.Context(), member.ID); err != nil {
						t.Fatal(err)
					}
				}
			},
		)
	}

	for _, failChild := range []bool{false, true} {
		t.Run(
			fmt.Sprintf("Should settle an open move before unarchive after child binding failure=%t", failChild),
			func(t *testing.T) {
				t.Parallel()
				catalog := &profileMoveBindingFailingCatalog{cause: errors.New("binding store unavailable")}
				h := newProfileMoveHarnessWith(
					t,
					func(db *globaldb.GlobalDB) store.SessionCatalog { catalog.GlobalDB = db; return catalog },
				)
				source := h.stoppedSource(t, h.worktrees.add(t, store.DefaultProfileID), "")
				child := h.stoppedSource(t, source.WorktreeID, source.ID, store.SubagentSpawnRole)
				catalog.failID = source.ID
				if failChild {
					catalog.failID = child.ID
				}
				h.worktrees.failCreate[profileMoveTargetID] = errors.New("target creation failed")
				if _, err := h.move(t.Context(), source.ID); !errors.Is(err, catalog.cause) {
					t.Fatalf("move error = %v", err)
				}
				_, err := h.manager.Unarchive(t.Context(), h.workspaceID, source.ID)
				refusal, ok := errors.AsType[*ProfileMoveError](err)
				if !ok || refusal.Reason != ProfileMoveRefusalRecoveryIncomplete || !errors.Is(err, catalog.cause) {
					t.Fatalf("unarchive error = %v; want recovery-incomplete wrapping %v", err, catalog.cause)
				}
				catalog.failID = ""
				if _, err := h.manager.Unarchive(t.Context(), h.workspaceID, source.ID); err != nil {
					t.Fatal(err)
				}
				if err := h.manager.RecoverProfileMoves(t.Context()); err != nil {
					t.Fatal(err)
				}
				rows, err := h.db.ListSessions(
					t.Context(),
					store.SessionListQuery{
						ReadScope:   store.ReadScope{ProfileID: store.DefaultProfileID},
						WorkspaceID: h.workspaceID,
						Archive:     store.SessionArchiveInclude,
					},
				)
				if err != nil {
					t.Fatal(err)
				}
				bindings := make(map[string]string)
				for i := range rows {
					bindings[rows[i].ID] = rows[i].WorktreeID
				}
				if bindings[source.ID] == "" || bindings[source.ID] != bindings[child.ID] {
					t.Fatalf("catalog bindings before Status repair = %v", bindings)
				}
				root, err := h.manager.Status(t.Context(), source.ID)
				if err != nil {
					t.Fatal(err)
				}
				persisted, err := h.manager.Status(t.Context(), child.ID)
				if err != nil || root.WorktreeID == "" || persisted.WorktreeID != root.WorktreeID ||
					h.archivedAt(t, child.ID) {
					t.Fatalf("family after retry = %#v, %#v, %v", root, persisted, err)
				}
				if checkout, ok := h.worktrees.get(root.WorktreeID); !ok || checkout.released {
					t.Fatal("retry lost source checkout")
				}
				if _, open, err := h.db.SessionProfileMove(t.Context(), source.ID); err != nil || open {
					t.Fatalf("journal still open: %t, %v", open, err)
				}
			},
		)
	}

	t.Run("Should allow workspace removal preparation before locking a restore family", func(t *testing.T) {
		t.Parallel()
		catalog := &profileMoveBindingFailingCatalog{}
		h := newProfileMoveHarnessWith(
			t,
			func(db *globaldb.GlobalDB) store.SessionCatalog { catalog.GlobalDB = db; return catalog },
		)
		created, err := h.manager.Create(
			t.Context(),
			&CreateOpts{DesiredSessionID: "z-root", AgentName: "coder", Workspace: h.workspaceID},
		)
		if err != nil {
			t.Fatal(err)
		}
		if err := h.manager.Stop(t.Context(), created.ID); err != nil {
			t.Fatal(err)
		}
		child := h.stoppedSource(t, "", created.ID, store.SubagentSpawnRole)
		meta, err := h.manager.readSessionMetaReadOnly(t.Context(), created.ID)
		if err != nil {
			t.Fatal(err)
		}
		called := false
		catalog.onDescendants = func(ctx context.Context) {
			called = true
			preparation, err := h.manager.PrepareWorkspaceRemoval(ctx, h.workspaceID)
			if err != nil {
				t.Fatalf("workspace removal preparation during restore = %v", err)
			}
			if err := preparation.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
		}
		if err := h.manager.restoreProfileMoveSource(t.Context(), &meta, "", "", false); err != nil {
			t.Fatal(err)
		}
		if !called {
			t.Fatal("workspace removal preparation did not run during restore")
		}
		if _, err := h.manager.Status(t.Context(), child.ID); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("Should finish a pending source unbind when completed-move unarchive is retried", func(t *testing.T) {
		t.Parallel()
		catalog := &profileMoveBindingFailingCatalog{cause: errors.New("source binding unavailable")}
		h := newProfileMoveHarnessWith(
			t,
			func(db *globaldb.GlobalDB) store.SessionCatalog { catalog.GlobalDB = db; return catalog },
		)
		source := h.stoppedSource(t, h.worktrees.add(t, store.DefaultProfileID), "")
		child := h.stoppedSource(t, source.WorktreeID, source.ID, store.SubagentSpawnRole)
		if _, err := h.move(t.Context(), source.ID); err != nil {
			t.Fatal(err)
		}
		catalog.failID = source.ID
		if _, err := h.manager.Unarchive(t.Context(), h.workspaceID, source.ID); !errors.Is(err, catalog.cause) {
			t.Fatalf("unarchive error = %v", err)
		}
		catalog.failID = ""
		if _, err := h.manager.Unarchive(t.Context(), h.workspaceID, source.ID); err != nil {
			t.Fatal(err)
		}
		rows, err := h.db.ListSessions(
			t.Context(),
			store.SessionListQuery{
				ReadScope:   store.ReadScope{ProfileID: store.DefaultProfileID},
				WorkspaceID: h.workspaceID,
				Archive:     store.SessionArchiveInclude,
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		for i := range rows {
			if (rows[i].ID == source.ID || rows[i].ID == child.ID) && rows[i].WorktreeID != "" {
				t.Fatalf("pending catalog binding survived retry: %#v", rows[i])
			}
		}
	})

	t.Run("Should reject a stale family restore before changing inherited bindings", func(t *testing.T) {
		t.Parallel()
		h := newProfileMoveHarness(t)
		source := h.stoppedSource(t, h.worktrees.add(t, store.DefaultProfileID), "")
		child := h.stoppedSource(t, source.WorktreeID, source.ID, store.SubagentSpawnRole)
		stale, err := h.manager.readSessionMetaReadOnly(t.Context(), source.ID)
		if err != nil {
			t.Fatal(err)
		}
		newWorktree := h.worktrees.add(t, store.DefaultProfileID)
		if _, err := h.manager.ChangeWorktreeBinding(
			t.Context(),
			source.ID,
			newWorktree,
			&source.WorktreeID,
		); err != nil {
			t.Fatal(err)
		}
		if err := h.manager.restoreProfileMoveSource(
			t.Context(),
			&stale,
			source.WorktreeID,
			"",
			true,
		); !errors.Is(
			err,
			store.ErrSessionWorktreeConflict,
		) {
			t.Fatalf("stale restore = %v", err)
		}
		persisted, err := h.manager.Status(t.Context(), child.ID)
		if err != nil || persisted.WorktreeID != child.WorktreeID {
			t.Fatalf("stale restore changed child: %#v, %v", persisted, err)
		}
		root, err := h.manager.Status(t.Context(), source.ID)
		if err != nil || root.WorktreeID != newWorktree {
			t.Fatalf("stale restore changed root: %#v, %v", root, err)
		}
	})

	t.Run("Should preserve ordinary unarchive bindings without inspecting Git", func(t *testing.T) {
		t.Parallel()
		h := newProfileMoveHarness(t)
		source := h.stoppedSource(t, h.worktrees.add(t, store.DefaultProfileID), "")
		child := h.stoppedSource(t, source.WorktreeID, source.ID, store.SubagentSpawnRole)
		if _, err := h.manager.Archive(t.Context(), h.workspaceID, source.ID); err != nil {
			t.Fatal(err)
		}
		h.worktrees.mu.Lock()
		h.worktrees.worktrees[source.WorktreeID].released = true
		h.worktrees.mu.Unlock()
		h.worktrees.onInspect = func() { t.Fatal("ordinary unarchive inspected Git") }
		if _, err := h.manager.Unarchive(t.Context(), h.workspaceID, source.ID); err != nil {
			t.Fatal(err)
		}
		for _, member := range []*Info{source, child} {
			persisted, err := h.manager.Status(t.Context(), member.ID)
			if err != nil || persisted.WorktreeID != source.WorktreeID || h.archivedAt(t, member.ID) {
				t.Fatalf("ordinary unarchive changed binding: %#v, %v", persisted, err)
			}
		}
	})

	for _, role := range []string{"", store.SubagentSpawnRole} {
		for _, live := range []bool{false, true} {
			t.Run(
				fmt.Sprintf(
					"Should refuse an unarchived descendant with live=%t role=%q and count only blockers",
					live,
					role,
				),
				func(t *testing.T) {
					t.Parallel()

					h := newProfileMoveHarness(t)
					source := h.stoppedSource(t, "", "")
					archived := h.stoppedSource(t, "", source.ID)
					if _, err := h.manager.Archive(t.Context(), h.workspaceID, archived.ID); err != nil {
						t.Fatalf("Archive(child) error = %v", err)
					}
					if live {
						childType := SessionTypeUser
						if role != "" {
							childType = SessionTypeSpawned
						}
						created, err := h.manager.Create(t.Context(), &CreateOpts{
							AgentName: "coder", Workspace: h.workspaceID,
							Type: childType,
							Lineage: &store.SessionLineage{ParentSessionID: source.ID, SpawnRole: role,
								RootSessionID: source.ID, SpawnDepth: 1},
						})
						if err != nil {
							t.Fatalf("Create(child) error = %v", err)
						}
						t.Cleanup(func() { reportSessionStop(t, h.harness, created.ID) })
					} else {
						h.stoppedSource(t, "", archived.ID, role)
					}
					_, err := h.move(t.Context(), source.ID)
					refusal, ok := errors.AsType[*ProfileMoveError](err)
					want := fmt.Sprintf(
						"session %s has 1 live or unarchived descendant session(s); stop and archive them first",
						source.ID,
					)
					if !ok || refusal.Reason != ProfileMoveRefusalHasChildren || refusal.Message != want {
						t.Fatalf("MoveToProfile() error = %v, want has_children with %q", err, want)
					}
					if h.archivedAt(t, source.ID) || !h.archivedAt(t, archived.ID) {
						t.Fatal("refused move changed archive state")
					}
				},
			)
		}
	}

	t.Run("Should release descendant locks when waiting for another descendant is canceled", func(t *testing.T) {
		t.Parallel()
		h := newProfileMoveHarness(t)
		source := h.stoppedSource(t, "", "")
		child := h.stoppedSource(t, "", source.ID)
		grandchild := h.stoppedSource(t, "", child.ID)
		for _, descendant := range []*Info{grandchild, child} {
			if _, err := h.manager.Archive(t.Context(), h.workspaceID, descendant.ID); err != nil {
				t.Fatalf("Archive(descendant) error = %v", err)
			}
		}
		_, unlock, err := h.manager.lockProfileMove(t.Context(), grandchild.ID)
		if err != nil {
			t.Fatalf("lockProfileMove(grandchild) error = %v", err)
		}
		defer unlock()
		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()
		if _, err := h.move(ctx, source.ID); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("MoveToProfile() error = %v, want deadline", err)
		}
		unarchiveCtx, unarchiveCancel := context.WithTimeout(t.Context(), time.Second)
		defer unarchiveCancel()
		if _, err := h.manager.Unarchive(unarchiveCtx, h.workspaceID, child.ID); err != nil {
			t.Fatalf("Unarchive(child) after canceled move error = %v; want released lock", err)
		}
	})

	for _, duringMove := range []bool{false, true} {
		t.Run(fmt.Sprintf("Should serialize descendant unarchive with duringMove=%t", duringMove), func(t *testing.T) {
			t.Parallel()
			h := newProfileMoveHarness(t)
			source := h.stoppedSource(t, h.worktrees.add(t, store.DefaultProfileID), "")
			child := h.stoppedSource(t, "", source.ID)
			grandchild := h.stoppedSource(t, "", child.ID)
			for _, descendant := range []*Info{grandchild, child} {
				if _, err := h.manager.Archive(t.Context(), h.workspaceID, descendant.ID); err != nil {
					t.Fatalf("Archive(descendant) error = %v", err)
				}
			}
			unarchive := func() {
				ctx := t.Context()
				if duringMove {
					var cancel context.CancelFunc
					ctx, cancel = context.WithTimeout(ctx, 50*time.Millisecond)
					defer cancel()
				}
				_, err := h.manager.Unarchive(ctx, h.workspaceID, grandchild.ID)
				if duringMove {
					if !errors.Is(err, context.DeadlineExceeded) {
						t.Errorf("Unarchive() during move error = %v, want deadline", err)
					}
				} else if err != nil {
					t.Errorf("Unarchive() before locking error = %v", err)
				}
			}
			if duringMove {
				h.worktrees.onRelease = unarchive
			} else {
				h.worktrees.onInspect = unarchive
			}
			_, err := h.move(t.Context(), source.ID)
			if duringMove {
				if err != nil || !h.archivedAt(t, grandchild.ID) {
					t.Fatalf("MoveToProfile() = %v; want archived descendant until commit", err)
				}
				if _, err := h.manager.Unarchive(t.Context(), h.workspaceID, grandchild.ID); err != nil {
					t.Fatalf("Unarchive() after move error = %v", err)
				}
			} else {
				refusal, ok := errors.AsType[*ProfileMoveError](err)
				if !ok || refusal.Reason != ProfileMoveRefusalHasChildren || h.archivedAt(t, source.ID) {
					t.Fatalf("MoveToProfile() = %v; want refusal before archive", err)
				}
			}
		})
	}

	t.Run("Should refuse a move that breaks a rule before changing anything", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name   string
			reason ProfileMoveRefusal
			setup  func(*testing.T, *profileMoveHarness) (context.Context, string, string)
		}{
			{
				name: "Should refuse a running session", reason: ProfileMoveRefusalRunning,
				setup: func(t *testing.T, h *profileMoveHarness) (context.Context, string, string) {
					created, err := h.manager.Create(testutil.Context(t), new(CreateOpts{
						AgentName: "coder", Name: "live", Workspace: h.workspaceID,
					}))
					if err != nil {
						t.Fatalf("Create(live) error = %v", err)
					}
					t.Cleanup(func() { reportSessionStop(t, h.harness, created.ID) })
					return testutil.Context(t), created.ID, profileMoveTargetID
				},
			},
			{
				name: "Should refuse a session with children", reason: ProfileMoveRefusalHasChildren,
				setup: func(t *testing.T, h *profileMoveHarness) (context.Context, string, string) {
					parent := h.stoppedSource(t, "", "")
					h.stoppedSource(t, "", parent.ID)
					return testutil.Context(t), parent.ID, profileMoveTargetID
				},
			},
			{
				name: "Should refuse a nested session", reason: ProfileMoveRefusalNotMovable,
				setup: func(t *testing.T, h *profileMoveHarness) (context.Context, string, string) {
					parent := h.stoppedSource(t, "", "")
					child := h.stoppedSource(t, "", parent.ID)
					return testutil.Context(t), child.ID, profileMoveTargetID
				},
			},
			{
				name: "Should refuse a worktree with uncommitted changes", reason: ProfileMoveRefusalWorktreeDirty,
				setup: func(t *testing.T, h *profileMoveHarness) (context.Context, string, string) {
					source := h.stoppedSource(t, h.worktrees.add(t, store.DefaultProfileID), "")
					h.worktrees.mu.Lock()
					h.worktrees.dirty = 2
					h.worktrees.mu.Unlock()
					return testutil.Context(t), source.ID, profileMoveTargetID
				},
			},
			{
				name: "Should refuse the calling session", reason: ProfileMoveRefusalOwnSession,
				setup: func(t *testing.T, h *profileMoveHarness) (context.Context, string, string) {
					source := h.stoppedSource(t, "", "")
					return WithActingSession(testutil.Context(t), source.ID), source.ID, profileMoveTargetID
				},
			},
			{
				name: "Should refuse the same profile", reason: ProfileMoveRefusalSameProfile,
				setup: func(t *testing.T, h *profileMoveHarness) (context.Context, string, string) {
					source := h.stoppedSource(t, "", "")
					return testutil.Context(t), source.ID, store.DefaultProfileID
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				h := newProfileMoveHarness(t)
				ctx, sessionID, targetID := tt.setup(t, h)
				_, err := h.manager.MoveToProfile(ctx, MoveToProfileOpts{
					WorkspaceID: h.workspaceID, ProfileID: store.DefaultProfileID,
					SessionID: sessionID, TargetProfileID: targetID,
				})
				refusal, ok := errors.AsType[*ProfileMoveError](err)
				if !ok || refusal.Reason != tt.reason || !errors.Is(err, ErrProfileMoveRefused) {
					t.Fatalf("MoveToProfile() error = %v, want refusal %q", err, tt.reason)
				}
				if h.archivedAt(t, sessionID) || h.worktrees.releaseCount() != 0 {
					t.Fatalf("refused move changed state: archived=%t releases=%d",
						h.archivedAt(t, sessionID), h.worktrees.releaseCount())
				}
			})
		}
	})

	t.Run(
		"Should restore the original session and its branch when the copy fails after the archive",
		func(t *testing.T) {
			t.Parallel()

			h := newProfileMoveHarness(t)
			sourceWorktree := h.worktrees.add(t, store.DefaultProfileID)
			source := h.stoppedSource(t, sourceWorktree, "")
			cause := errors.New("target checkout failed")
			h.worktrees.mu.Lock()
			h.worktrees.failCreate[profileMoveTargetID] = cause
			h.worktrees.mu.Unlock()

			_, err := h.move(testutil.Context(t), source.ID)
			if !errors.Is(err, cause) || errors.Is(err, ErrProfileMoveRefused) {
				t.Fatalf("MoveToProfile() error = %v, want the cause after a complete rollback", err)
			}
			if h.archivedAt(t, source.ID) {
				t.Fatalf("original %s is still archived after the rollback", source.ID)
			}
			original, err := h.manager.Status(testutil.Context(t), source.ID)
			if err != nil {
				t.Fatalf("Status(original) error = %v", err)
			}
			restored, ok := h.worktrees.get(original.WorktreeID)
			if !ok || original.WorktreeID == sourceWorktree || restored.released ||
				restored.profileID != store.DefaultProfileID || restored.branch != profileMoveBranch {
				t.Fatalf("original worktree = %q (%#v), want a fresh source-profile checkout of %s",
					original.WorktreeID, restored, profileMoveBranch)
			}
			rows, err := h.db.ListSessions(testutil.Context(t), store.SessionListQuery{
				ReadScope: store.ReadScope{ProfileID: profileMoveTargetID}, WorkspaceID: h.workspaceID,
			})
			if err != nil || len(rows) != 0 {
				t.Fatalf("target profile sessions = %#v, %v; want none", rows, err)
			}
		},
	)

	t.Run("Should refuse and restore the session when its worktree changes while the move plans", func(t *testing.T) {
		t.Parallel()

		h := newProfileMoveHarness(t)
		sourceWorktree := h.worktrees.add(t, store.DefaultProfileID)
		source := h.stoppedSource(t, sourceWorktree, "")
		otherWorktree := h.worktrees.add(t, store.DefaultProfileID)
		h.worktrees.mu.Lock()
		h.worktrees.onInspect = func() {
			h.worktrees.mu.Lock()
			h.worktrees.onInspect = nil
			h.worktrees.mu.Unlock()
			ctx := testutil.Context(t)
			if _, err := h.manager.ChangeWorktreeBinding(ctx, source.ID, otherWorktree, nil); err != nil {
				t.Errorf("ChangeWorktreeBinding() error = %v", err)
			}
		}
		h.worktrees.mu.Unlock()

		_, err := h.move(testutil.Context(t), source.ID)
		refusal, ok := errors.AsType[*ProfileMoveError](err)
		if !ok || refusal.Reason != ProfileMoveRefusalWorktreeChanged {
			t.Fatalf("MoveToProfile() error = %v, want worktree_changed", err)
		}
		original, err := h.manager.Status(testutil.Context(t), source.ID)
		if err != nil || original.WorktreeID != otherWorktree || h.archivedAt(t, source.ID) ||
			h.worktrees.releaseCount() != 0 {
			t.Fatalf("original after refusal = %#v, %v; archived=%t releases=%d; want it unarchived on %s",
				original, err, h.archivedAt(t, source.ID), h.worktrees.releaseCount(), otherWorktree)
		}
	})

	t.Run("Should check the branch out again when a failed release already removed the checkout", func(t *testing.T) {
		t.Parallel()

		h := newProfileMoveHarness(t)
		sourceWorktree := h.worktrees.add(t, store.DefaultProfileID)
		source := h.stoppedSource(t, sourceWorktree, "")
		cause := errors.New("tombstone write failed")
		h.worktrees.mu.Lock()
		h.worktrees.failAfterRelease = cause
		h.worktrees.mu.Unlock()

		_, err := h.move(testutil.Context(t), source.ID)
		if !errors.Is(err, cause) || errors.Is(err, ErrProfileMoveRefused) {
			t.Fatalf("MoveToProfile() error = %v, want the cause after a complete rollback", err)
		}
		original, err := h.manager.Status(testutil.Context(t), source.ID)
		if err != nil {
			t.Fatalf("Status(original) error = %v", err)
		}
		restored, ok := h.worktrees.get(original.WorktreeID)
		if !ok || original.WorktreeID == sourceWorktree || restored.released || h.archivedAt(t, source.ID) {
			t.Fatalf("original worktree = %q (%#v), archived=%t; want a fresh checkout of the branch",
				original.WorktreeID, restored, h.archivedAt(t, source.ID))
		}
	})

	t.Run("Should unarchive the source when its archive commits and then fails", func(t *testing.T) {
		t.Parallel()

		cause := errors.New("watch release failed")
		failing := &archiveFailingCatalog{}
		h := newProfileMoveHarnessWith(t, func(db *globaldb.GlobalDB) store.SessionCatalog {
			failing.GlobalDB = db
			return failing
		})
		source := h.stoppedSource(t, h.worktrees.add(t, store.DefaultProfileID), "")
		failing.archiveErr = cause

		_, err := h.move(testutil.Context(t), source.ID)
		if !errors.Is(err, cause) || errors.Is(err, ErrProfileMoveRefused) || h.archivedAt(t, source.ID) ||
			h.worktrees.releaseCount() != 0 {
			t.Fatalf("MoveToProfile() error = %v, archived=%t releases=%d; want the cause with the source restored",
				err, h.archivedAt(t, source.ID), h.worktrees.releaseCount())
		}
	})

	t.Run("Should report an unreadable archive state instead of assuming nothing was archived", func(t *testing.T) {
		t.Parallel()

		failing := &archiveFailingCatalog{}
		h := newProfileMoveHarnessWith(t, func(db *globaldb.GlobalDB) store.SessionCatalog {
			failing.GlobalDB = db
			return failing
		})
		source := h.stoppedSource(t, "", "")
		failing.archiveErr = errors.New("watch release failed")
		failing.readErr = errors.New("catalog unavailable")

		_, err := h.move(testutil.Context(t), source.ID)
		refusal, ok := errors.AsType[*ProfileMoveError](err)
		if !ok || refusal.Reason != ProfileMoveRefusalRecoveryIncomplete ||
			!strings.Contains(refusal.Message, source.ID) ||
			!strings.Contains(refusal.Message, "archive state could not be read") ||
			!strings.Contains(refusal.Message, "unarchived when the move is retried or the daemon restarts") {
			t.Fatalf("MoveToProfile() error = %v, want recovery_incomplete naming the session and its recovery", err)
		}
	})

	t.Run("Should keep the source checkout when a failed release left it in place", func(t *testing.T) {
		t.Parallel()

		h := newProfileMoveHarness(t)
		sourceWorktree := h.worktrees.add(t, store.DefaultProfileID)
		source := h.stoppedSource(t, sourceWorktree, "")
		cause := errors.New("removal hook denied")
		h.worktrees.mu.Lock()
		h.worktrees.failRelease = cause
		h.worktrees.mu.Unlock()

		_, err := h.move(testutil.Context(t), source.ID)
		if !errors.Is(err, cause) || errors.Is(err, ErrProfileMoveRefused) {
			t.Fatalf("MoveToProfile() error = %v, want the cause after a complete rollback", err)
		}
		original, err := h.manager.Status(testutil.Context(t), source.ID)
		if err != nil || original.WorktreeID != sourceWorktree || h.archivedAt(t, source.ID) {
			t.Fatalf("original = %#v, %v, archived=%t; want it unarchived on %s",
				original, err, h.archivedAt(t, source.ID), sourceWorktree)
		}
	})

	t.Run("Should never take an unreadable checkout as gone while rolling back", func(t *testing.T) {
		t.Parallel()

		h := newProfileMoveHarness(t)
		sourceWorktree := h.worktrees.add(t, store.DefaultProfileID)
		source := h.stoppedSource(t, sourceWorktree, "")
		h.worktrees.mu.Lock()
		h.worktrees.failRelease = errors.New("removal hook denied")
		// The plan reads the checkout first; the rollback's read cannot tell.
		h.worktrees.inspectErrs = map[int]error{2: errors.New("status unreadable")}
		h.worktrees.mu.Unlock()

		_, err := h.move(testutil.Context(t), source.ID)
		refusal, ok := errors.AsType[*ProfileMoveError](err)
		if !ok || refusal.Reason != ProfileMoveRefusalRecoveryIncomplete {
			t.Fatalf("MoveToProfile() error = %v, want recovery_incomplete while the checkout is unreadable", err)
		}
		if err := h.manager.RecoverProfileMoves(testutil.Context(t)); err != nil {
			t.Fatalf("RecoverProfileMoves() error = %v", err)
		}
		original, err := h.manager.Status(testutil.Context(t), source.ID)
		h.worktrees.mu.Lock()
		restores := h.worktrees.creates[store.DefaultProfileID]
		h.worktrees.mu.Unlock()
		if err != nil || original.WorktreeID != sourceWorktree || h.archivedAt(t, source.ID) || restores != 0 {
			t.Fatalf("original = %#v, %v, archived=%t, restores=%d; want it unarchived on %s",
				original, err, h.archivedAt(t, source.ID), restores, sourceWorktree)
		}
		if _, open := h.openMove(t, source.ID); open {
			t.Fatal("the move is still journaled after the recovery")
		}
	})

	t.Run("Should stop waiting for an in-flight move when the unarchive request ends", func(t *testing.T) {
		t.Parallel()

		h := newProfileMoveHarness(t)
		source := h.stoppedSource(t, "", "")
		_, unlock, err := h.manager.lockProfileMove(testutil.Context(t), source.ID)
		if err != nil {
			t.Fatalf("lockProfileMove() error = %v", err)
		}
		defer unlock()
		ctx, cancel := context.WithTimeout(testutil.Context(t), 50*time.Millisecond)
		defer cancel()
		if _, err := h.manager.Unarchive(ctx, h.workspaceID, source.ID); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Unarchive() during a move error = %v, want the request deadline", err)
		}
	})

	t.Run("Should keep the session and name the branch when the rollback cannot finish", func(t *testing.T) {
		t.Parallel()

		h := newProfileMoveHarness(t)
		source := h.stoppedSource(t, h.worktrees.add(t, store.DefaultProfileID), "")
		h.worktrees.mu.Lock()
		h.worktrees.failCreate[profileMoveTargetID] = errors.New("target checkout failed")
		h.worktrees.failCreate[store.DefaultProfileID] = errors.New("source checkout failed")
		h.worktrees.mu.Unlock()

		_, err := h.move(testutil.Context(t), source.ID)
		refusal, ok := errors.AsType[*ProfileMoveError](err)
		if !ok || refusal.Reason != ProfileMoveRefusalRecoveryIncomplete ||
			!strings.Contains(refusal.Message, profileMoveBranch) {
			t.Fatalf("MoveToProfile() error = %v, want recovery_incomplete naming %s", err, profileMoveBranch)
		}
		if _, err := h.manager.Status(testutil.Context(t), source.ID); err != nil || h.archivedAt(t, source.ID) {
			t.Fatalf("original after failed rollback: status err %v, archived %t; want kept and unarchived",
				err, h.archivedAt(t, source.ID))
		}
	})
}

// stoppingJournalCatalog stops a move right after its journal row commits, or right
// before a settled move closes the row, through the fake's scripted step.
type stoppingJournalCatalog struct {
	*globaldb.GlobalDB
	worktrees *fakeProfileMoveWorktrees
	// deleteErr fails the next journal close.
	deleteErr error
	// onReceipt runs once after the next derive receipt read.
	onReceipt func()
	// onClose runs after each journal close.
	onClose func()
}

func (c *stoppingJournalCatalog) SessionDerivationReceipt(
	ctx context.Context, workspaceID, key, profileID string,
) (store.SessionDerivationReceipt, bool, error) {
	receipt, found, err := c.GlobalDB.SessionDerivationReceipt(ctx, workspaceID, key, profileID)
	c.worktrees.mu.Lock()
	hook := c.onReceipt
	c.onReceipt = nil
	c.worktrees.mu.Unlock()
	if hook != nil {
		hook()
	}
	return receipt, found, err
}

func (c *stoppingJournalCatalog) InsertSessionProfileMove(ctx context.Context, move store.SessionProfileMove) error {
	err := c.GlobalDB.InsertSessionProfileMove(ctx, move)
	if err == nil {
		c.worktrees.step("journaled")
	}
	return err
}

func (c *stoppingJournalCatalog) DeleteSessionProfileMove(ctx context.Context, sessionID string) error {
	c.worktrees.step("close")
	c.worktrees.mu.Lock()
	err := c.deleteErr
	c.deleteErr = nil
	c.worktrees.mu.Unlock()
	if err != nil {
		return err
	}
	if err := c.GlobalDB.DeleteSessionProfileMove(ctx, sessionID); err != nil {
		return err
	}
	c.worktrees.mu.Lock()
	hook := c.onClose
	c.worktrees.mu.Unlock()
	if hook != nil {
		hook()
	}
	return nil
}

func newStoppingProfileMoveHarness(t *testing.T) *profileMoveHarness {
	t.Helper()
	h, _ := newStoppingProfileMoveHarnessWithCatalog(t)
	return h
}

func newStoppingProfileMoveHarnessWithCatalog(t *testing.T) (*profileMoveHarness, *stoppingJournalCatalog) {
	t.Helper()
	catalog := &stoppingJournalCatalog{}
	h := newProfileMoveHarnessWith(t, func(db *globaldb.GlobalDB) store.SessionCatalog {
		catalog.GlobalDB = db
		return catalog
	})
	catalog.worktrees = h.worktrees
	return h, catalog
}

func (h *profileMoveHarness) setWorktree(t *testing.T, id string, edit func(*fakeMoveWorktree)) {
	t.Helper()
	h.worktrees.mu.Lock()
	defer h.worktrees.mu.Unlock()
	item, ok := h.worktrees.worktrees[id]
	if !ok {
		t.Fatalf("worktree %s is not registered", id)
	}
	edit(item)
}

// stopAt runs work until it reaches step and stops there, as a daemon stop would.
func (h *profileMoveHarness) stopAt(t *testing.T, step string, work func()) {
	t.Helper()
	h.worktrees.mu.Lock()
	h.worktrees.stopAt = step
	h.worktrees.mu.Unlock()
	done := make(chan struct{})
	finished := false
	go func() {
		defer close(done)
		work()
		finished = true
	}()
	<-done
	if finished {
		t.Fatalf("work finished without reaching step %q", step)
	}
}

func (h *profileMoveHarness) openMove(t *testing.T, sessionID string) (store.SessionProfileMove, bool) {
	t.Helper()
	move, found, err := h.db.SessionProfileMove(testutil.Context(t), sessionID)
	if err != nil {
		t.Fatalf("SessionProfileMove(%q) error = %v", sessionID, err)
	}
	return move, found
}

func (h *profileMoveHarness) targetSessions(t *testing.T) []store.SessionInfo {
	t.Helper()
	rows, err := h.db.ListSessions(testutil.Context(t), store.SessionListQuery{
		ReadScope: store.ReadScope{ProfileID: profileMoveTargetID}, WorkspaceID: h.workspaceID,
	})
	if err != nil {
		t.Fatalf("ListSessions(target profile) error = %v", err)
	}
	return rows
}

// requireRolledBack asserts the original is unarchived on a live source-profile
// checkout of its branch, with no copy and no target checkout left.
func (h *profileMoveHarness) requireRolledBack(t *testing.T, sessionID string, move store.SessionProfileMove) {
	t.Helper()
	original, err := h.manager.Status(testutil.Context(t), sessionID)
	if err != nil {
		t.Fatalf("Status(original) error = %v", err)
	}
	bound, ok := h.worktrees.get(original.WorktreeID)
	if h.archivedAt(t, sessionID) || !ok || bound.released || bound.profileID != store.DefaultProfileID ||
		bound.branch != profileMoveBranch {
		t.Fatalf("original = archived %t on %q (%#v), want unarchived on a source-profile checkout of %s",
			h.archivedAt(t, sessionID), original.WorktreeID, bound, profileMoveBranch)
	}
	if target, ok := h.worktrees.get(move.TargetWorktreeID); ok && !target.released {
		t.Fatalf("target worktree %s is still checked out", move.TargetWorktreeID)
	}
	if rows := h.targetSessions(t); len(rows) != 0 {
		t.Fatalf("target profile sessions = %#v, want none", rows)
	}
	if _, open := h.openMove(t, sessionID); open {
		t.Fatal("the move is still journaled after it settled")
	}
}

// TestManagerRecoverProfileMoves verifies that interrupted transitions remain recoverable.
func TestManagerRecoverProfileMoves(t *testing.T) {
	t.Parallel()

	t.Run("Should settle a move a daemon stop interrupted at each step", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name      string
			step      string
			completed bool
		}{
			{name: "Should restore the original when the stop follows the journal", step: "journaled"},
			{name: "Should unarchive the original when the stop follows the archive", step: "release"},
			{
				name: "Should check the branch out again when the stop follows the release",
				step: "create:" + profileMoveTargetID,
			},
			{
				name: "Should release the target checkout when the stop follows its creation",
				step: "created:" + profileMoveTargetID,
			},
			{name: "Should complete the move when the stop follows the copy", step: "close", completed: true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				h := newStoppingProfileMoveHarness(t)
				source := h.stoppedSource(t, h.worktrees.add(t, store.DefaultProfileID), "")
				h.stopAt(t, tt.step, func() { _, _ = h.move(testutil.Context(t), source.ID) })
				move, open := h.openMove(t, source.ID)
				if !open || move.TargetWorktreeID == "" || move.RestoreWorktreeID == "" ||
					move.Branch != profileMoveBranch {
					t.Fatalf("journal after the stop = %#v, open %t; want the move's ids and branch", move, open)
				}

				if err := h.manager.RecoverProfileMoves(testutil.Context(t)); err != nil {
					t.Fatalf("RecoverProfileMoves() error = %v", err)
				}
				if !tt.completed {
					h.requireRolledBack(t, source.ID, move)
					return
				}
				rows := h.targetSessions(t)
				target, ok := h.worktrees.get(move.TargetWorktreeID)
				if !h.archivedAt(t, source.ID) || len(rows) != 1 || rows[0].WorktreeID != move.TargetWorktreeID ||
					!ok || target.released {
					t.Fatalf("completed move: archived %t, copies %#v, target %#v; want the copy on %s",
						h.archivedAt(t, source.ID), rows, target, move.TargetWorktreeID)
				}
				if _, open := h.openMove(t, source.ID); open {
					t.Fatal("the completed move is still journaled")
				}
			})
		}
	})

	t.Run("Should reuse the restored checkout when a stop interrupts the rollback", func(t *testing.T) {
		t.Parallel()

		h := newStoppingProfileMoveHarness(t)
		source := h.stoppedSource(t, h.worktrees.add(t, store.DefaultProfileID), "")
		child := h.stoppedSource(t, source.WorktreeID, source.ID, store.SubagentSpawnRole)

		h.stopAt(t, "created:"+profileMoveTargetID, func() { _, _ = h.move(testutil.Context(t), source.ID) })
		move, _ := h.openMove(t, source.ID)
		h.stopAt(t, "created:"+store.DefaultProfileID, func() {
			_ = h.manager.RecoverProfileMoves(testutil.Context(t))
		})

		if err := h.manager.RecoverProfileMoves(testutil.Context(t)); err != nil {
			t.Fatalf("RecoverProfileMoves() after the interrupted rollback error = %v", err)
		}
		h.requireRolledBack(t, source.ID, move)
		original, _ := h.manager.Status(testutil.Context(t), source.ID)
		h.worktrees.mu.Lock()
		restores := h.worktrees.creates[store.DefaultProfileID]
		h.worktrees.mu.Unlock()
		if original.WorktreeID != move.RestoreWorktreeID || restores != 1 {
			t.Fatalf("original on %q after %d restore checkout(s), want the one restore checkout %s",
				original.WorktreeID, restores, move.RestoreWorktreeID)
		}
		persisted, err := h.manager.Status(t.Context(), child.ID)
		if err != nil || persisted.WorktreeID != original.WorktreeID || h.archivedAt(t, child.ID) {
			t.Fatalf("recovered family binding = %#v, %v", persisted, err)
		}
		if _, err := h.manager.Resume(t.Context(), child.ID); err != nil {
			t.Fatal(err)
		}
		if err := h.manager.Stop(t.Context(), child.ID); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("Should check the branch out under a fresh id when the restore checkout is gone", func(t *testing.T) {
		t.Parallel()

		h := newStoppingProfileMoveHarness(t)
		source := h.stoppedSource(t, h.worktrees.add(t, store.DefaultProfileID), "")
		h.stopAt(t, "create:"+profileMoveTargetID, func() { _, _ = h.move(testutil.Context(t), source.ID) })
		move, _ := h.openMove(t, source.ID)
		h.stopAt(t, "created:"+store.DefaultProfileID, func() {
			_ = h.manager.RecoverProfileMoves(testutil.Context(t))
		})
		// The restore checkout's creation failed and rolled back.
		h.worktrees.mu.Lock()
		h.worktrees.worktrees[move.RestoreWorktreeID].released = true
		h.worktrees.mu.Unlock()

		if err := h.manager.RecoverProfileMoves(testutil.Context(t)); err != nil {
			t.Fatalf("RecoverProfileMoves() error = %v", err)
		}
		h.requireRolledBack(t, source.ID, move)
		original, _ := h.manager.Status(testutil.Context(t), source.ID)
		if original.WorktreeID == move.RestoreWorktreeID || original.WorktreeID == move.SourceWorktreeID {
			t.Fatalf("original on %q, want a fresh restore checkout", original.WorktreeID)
		}
	})

	t.Run("Should keep the move journaled until a later recovery can finish it", func(t *testing.T) {
		t.Parallel()

		h := newStoppingProfileMoveHarness(t)
		source := h.stoppedSource(t, h.worktrees.add(t, store.DefaultProfileID), "")
		h.stopAt(t, "created:"+profileMoveTargetID, func() { _, _ = h.move(testutil.Context(t), source.ID) })
		move, _ := h.openMove(t, source.ID)
		h.worktrees.mu.Lock()
		h.worktrees.failCreate[store.DefaultProfileID] = errors.New("source checkout failed")
		h.worktrees.mu.Unlock()

		if err := h.manager.RecoverProfileMoves(testutil.Context(t)); err == nil {
			t.Fatal("RecoverProfileMoves() error = nil, want the unfinished restore reported")
		}
		if _, open := h.openMove(t, source.ID); !open || h.archivedAt(t, source.ID) {
			t.Fatalf("after a failed restore: journaled %t archived %t, want journaled and unarchived",
				open, h.archivedAt(t, source.ID))
		}
		h.worktrees.mu.Lock()
		delete(h.worktrees.failCreate, store.DefaultProfileID)
		h.worktrees.mu.Unlock()
		if err := h.manager.RecoverProfileMoves(testutil.Context(t)); err != nil {
			t.Fatalf("RecoverProfileMoves() retry error = %v", err)
		}
		h.requireRolledBack(t, source.ID, move)
	})

	t.Run(
		"Should check the branch out again when the restored checkout is lost before the move closes",
		func(t *testing.T) {
			t.Parallel()

			h := newStoppingProfileMoveHarness(t)
			source := h.stoppedSource(t, h.worktrees.add(t, store.DefaultProfileID), "")
			h.stopAt(t, "created:"+profileMoveTargetID, func() { _, _ = h.move(testutil.Context(t), source.ID) })
			move, _ := h.openMove(t, source.ID)
			h.stopAt(t, "close", func() { _ = h.manager.RecoverProfileMoves(testutil.Context(t)) })
			h.setWorktree(t, move.RestoreWorktreeID, func(item *fakeMoveWorktree) { item.released = true })

			if err := h.manager.RecoverProfileMoves(testutil.Context(t)); err != nil {
				t.Fatalf("RecoverProfileMoves() error = %v", err)
			}
			h.requireRolledBack(t, source.ID, move)
			original, _ := h.manager.Status(testutil.Context(t), source.ID)
			if original.WorktreeID == move.RestoreWorktreeID {
				t.Fatalf("original still bound to the lost restore checkout %s", move.RestoreWorktreeID)
			}
		},
	)

	t.Run("Should refuse a restore checkout that another branch took over", func(t *testing.T) {
		t.Parallel()

		h := newStoppingProfileMoveHarness(t)
		source := h.stoppedSource(t, h.worktrees.add(t, store.DefaultProfileID), "")
		h.stopAt(t, "create:"+profileMoveTargetID, func() { _, _ = h.move(testutil.Context(t), source.ID) })
		move, _ := h.openMove(t, source.ID)
		h.stopAt(t, "created:"+store.DefaultProfileID, func() {
			_ = h.manager.RecoverProfileMoves(testutil.Context(t))
		})
		h.setWorktree(t, move.RestoreWorktreeID, func(item *fakeMoveWorktree) { item.branch = "someone-else" })

		err := h.manager.RecoverProfileMoves(testutil.Context(t))
		original, _ := h.manager.Status(testutil.Context(t), source.ID)
		if _, open := h.openMove(t, source.ID); err == nil || !open || original.WorktreeID == move.RestoreWorktreeID {
			t.Fatalf("recovery = %v, journaled %t, bound to %q; want it refused and kept journaled",
				err, open, original.WorktreeID)
		}
	})

	t.Run(
		"Should restore the original once a target checkout that could not be released is cleaned up",
		func(t *testing.T) {
			t.Parallel()

			h := newStoppingProfileMoveHarness(t)
			source := h.stoppedSource(t, h.worktrees.add(t, store.DefaultProfileID), "")
			h.stopAt(t, "created:"+profileMoveTargetID, func() { _, _ = h.move(testutil.Context(t), source.ID) })
			move, _ := h.openMove(t, source.ID)
			// A dismissed creation whose cleanup did not finish still holds the branch.
			h.setWorktree(t, move.TargetWorktreeID, func(item *fakeMoveWorktree) { item.stuck = true })

			if err := h.manager.RecoverProfileMoves(testutil.Context(t)); err == nil {
				t.Fatal("RecoverProfileMoves() error = nil, want the held branch reported")
			}
			if _, open := h.openMove(t, source.ID); !open || h.archivedAt(t, source.ID) {
				t.Fatalf("journaled %t archived %t, want the move kept and the original unarchived",
					open, h.archivedAt(t, source.ID))
			}
			h.setWorktree(
				t,
				move.TargetWorktreeID,
				func(item *fakeMoveWorktree) { item.stuck, item.released = false, true },
			)
			if err := h.manager.RecoverProfileMoves(testutil.Context(t)); err != nil {
				t.Fatalf("RecoverProfileMoves() after the cleanup error = %v", err)
			}
			h.requireRolledBack(t, source.ID, move)
		},
	)

	t.Run("Should report a journal that could not close instead of starting another move", func(t *testing.T) {
		t.Parallel()

		h, catalog := newStoppingProfileMoveHarnessWithCatalog(t)
		source := h.stoppedSource(t, h.worktrees.add(t, store.DefaultProfileID), "")
		h.stopAt(t, "release", func() { _, _ = h.move(testutil.Context(t), source.ID) })
		h.worktrees.mu.Lock()
		catalog.deleteErr = errors.New("journal write failed")
		h.worktrees.mu.Unlock()

		_, err := h.move(testutil.Context(t), source.ID)
		refusal, ok := errors.AsType[*ProfileMoveError](err)
		if !ok || refusal.Reason != ProfileMoveRefusalRecoveryIncomplete ||
			!strings.Contains(refusal.Message, "journal write failed") {
			t.Fatalf("MoveToProfile() error = %v, want recovery_incomplete naming the failed close", err)
		}
		if result, err := h.move(testutil.Context(t), source.ID); err != nil || result.Copy == nil {
			t.Fatalf("MoveToProfile() retry = %#v, %v; want the move to complete", result.Copy, err)
		}
	})

	t.Run("Should refuse deleting a session until its interrupted move settles", func(t *testing.T) {
		t.Parallel()

		h := newStoppingProfileMoveHarness(t)
		source := h.stoppedSource(t, h.worktrees.add(t, store.DefaultProfileID), "")
		h.stopAt(t, "created:"+profileMoveTargetID, func() { _, _ = h.move(testutil.Context(t), source.ID) })
		move, _ := h.openMove(t, source.ID)

		if err := h.manager.Delete(testutil.Context(t), source.ID); !errors.Is(err, ErrSessionProfileMoveOpen) {
			t.Fatalf("Delete() during an open move error = %v, want ErrSessionProfileMoveOpen", err)
		}
		if err := h.manager.RecoverProfileMoves(testutil.Context(t)); err != nil {
			t.Fatalf("RecoverProfileMoves() error = %v", err)
		}
		h.requireRolledBack(t, source.ID, move)
		if err := h.manager.Delete(testutil.Context(t), source.ID); err != nil {
			t.Fatalf("Delete() after the move settled error = %v", err)
		}
	})

	t.Run("Should restore the original when its copy was deleted before the move closed", func(t *testing.T) {
		t.Parallel()

		h := newStoppingProfileMoveHarness(t)
		source := h.stoppedSource(t, h.worktrees.add(t, store.DefaultProfileID), "")
		h.stopAt(t, "close", func() { _, _ = h.move(testutil.Context(t), source.ID) })
		move, _ := h.openMove(t, source.ID)
		copies := h.targetSessions(t)
		if len(copies) != 1 {
			t.Fatalf("copies after the stop = %#v, want the committed copy", copies)
		}
		if err := h.manager.Delete(testutil.Context(t), copies[0].ID); err != nil {
			t.Fatalf("Delete(copy) error = %v", err)
		}

		if err := h.manager.RecoverProfileMoves(testutil.Context(t)); err != nil {
			t.Fatalf("RecoverProfileMoves() error = %v", err)
		}
		h.requireRolledBack(t, source.ID, move)
	})

	t.Run("Should leave the remaining moves journaled when boot is canceled", func(t *testing.T) {
		t.Parallel()

		h, catalog := newStoppingProfileMoveHarnessWithCatalog(t)
		var sources []string
		for range 5 {
			source := h.stoppedSource(t, "", "")
			h.stopAt(t, "journaled", func() { _, _ = h.move(testutil.Context(t), source.ID) })
			sources = append(sources, source.ID)
		}
		ctx, cancel := context.WithCancel(testutil.Context(t))
		defer cancel()
		// Boot is canceled after the first move settles. The lock a settle takes may
		// still be won on a canceled context, so only the check before each settle
		// keeps the remaining moves untouched.
		h.worktrees.mu.Lock()
		catalog.onClose = cancel
		h.worktrees.mu.Unlock()

		if err := h.manager.RecoverProfileMoves(ctx); !errors.Is(err, context.Canceled) {
			t.Fatalf("RecoverProfileMoves(canceled) error = %v, want context.Canceled", err)
		}
		settled := 0
		for _, id := range sources {
			if _, open := h.openMove(t, id); !open {
				settled++
			}
		}
		if settled != 1 {
			t.Fatalf("settled %d move(s) after the cancel, want only the one in flight", settled)
		}
	})

	t.Run("Should roll back when the copy is deleted between the receipt read and the close", func(t *testing.T) {
		t.Parallel()

		h, catalog := newStoppingProfileMoveHarnessWithCatalog(t)
		source := h.stoppedSource(t, h.worktrees.add(t, store.DefaultProfileID), "")
		h.stopAt(t, "close", func() { _, _ = h.move(testutil.Context(t), source.ID) })
		move, _ := h.openMove(t, source.ID)
		copies := h.targetSessions(t)
		if len(copies) != 1 {
			t.Fatalf("copies after the stop = %#v, want the committed copy", copies)
		}
		h.worktrees.mu.Lock()
		catalog.onReceipt = func() {
			if err := h.manager.Delete(testutil.Context(t), copies[0].ID); err != nil {
				t.Errorf("Delete(copy) error = %v", err)
			}
		}
		h.worktrees.mu.Unlock()

		if err := h.manager.RecoverProfileMoves(testutil.Context(t)); err != nil {
			t.Fatalf("RecoverProfileMoves() error = %v", err)
		}
		h.requireRolledBack(t, source.ID, move)
	})

	t.Run("Should settle an interrupted move before a retry moves the session", func(t *testing.T) {
		t.Parallel()

		h := newStoppingProfileMoveHarness(t)
		source := h.stoppedSource(t, h.worktrees.add(t, store.DefaultProfileID), "")
		h.stopAt(t, "release", func() { _, _ = h.move(testutil.Context(t), source.ID) })

		result, err := h.move(testutil.Context(t), source.ID)
		if err != nil || result.Copy == nil || result.Copy.ProfileID != profileMoveTargetID {
			t.Fatalf("retried MoveToProfile() = %#v, %v; want the copy in the target profile", result.Copy, err)
		}
		if _, open := h.openMove(t, source.ID); open || !h.archivedAt(t, source.ID) {
			t.Fatal("the retried move left its journal open or the original unarchived")
		}
	})
}
