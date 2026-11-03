package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	// failAfterRelease is returned once the checkout was already removed.
	failAfterRelease error
}

type fakeMoveWorktree struct {
	root, profileID, branch string
	released                bool
}

func newFakeProfileMoveWorktrees(
	t *testing.T, db *globaldb.GlobalDB, workspaceID string,
) *fakeProfileMoveWorktrees {
	t.Helper()
	return &fakeProfileMoveWorktrees{
		db: db, workspace: workspaceID, base: t.TempDir(),
		worktrees: map[string]*fakeMoveWorktree{}, failCreate: map[string]error{},
	}
}

func (f *fakeProfileMoveWorktrees) add(t *testing.T, profileID string) string {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	id, _, err := f.addLocked(profileID)
	if err != nil {
		t.Fatalf("add worktree: %v", err)
	}
	return id
}

func (f *fakeProfileMoveWorktrees) addLocked(profileID string) (string, string, error) {
	f.next++
	id := fmt.Sprintf("wt-%d", f.next)
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
		return ProfileMoveCheckout{}, fmt.Errorf("worktree %s not found", worktreeID)
	}
	f.mu.Lock()
	onInspect := f.onInspect
	f.mu.Unlock()
	if onInspect != nil {
		onInspect()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return ProfileMoveCheckout{
		ID: worktreeID, Branch: item.branch, HeadSHA: "abc123", Ready: !item.released, DirtyFiles: f.dirty,
	}, nil
}

func (f *fakeProfileMoveWorktrees) ReleaseProfileMoveWorktree(_ context.Context, _ string, worktreeID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	item, ok := f.worktrees[worktreeID]
	if !ok {
		return fmt.Errorf("worktree %s not found", worktreeID)
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
	_ context.Context, _ string, profileID string, checkout ProfileMoveCheckout,
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
	return f.addLocked(profileID)
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

func newProfileMoveHarness(t *testing.T) *profileMoveHarness {
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
	h.manager = newManagerWithHarness(
		t, h,
		WithSessionCatalog(db), WithEventLedger(db), WithSessionPromptAdmissionStore(db),
		WithWorktreeResolver(worktrees),
		WithProfileNameResolver(profileNameResolverMap{profileMoveTargetID: profileMoveTargetName}),
	)
	cleanupTestManager(t, h.manager)
	return &profileMoveHarness{harness: h, db: db, worktrees: worktrees}
}

// stoppedSource creates a user session on worktreeID (none when empty), runs one
// settled turn so the copy has a conversation to carry, and stops it.
func (h *profileMoveHarness) stoppedSource(t *testing.T, worktreeID string, parentID string) *Info {
	t.Helper()
	opts := CreateOpts{AgentName: "coder", Name: "Migration cleanup", Workspace: h.workspaceID, Worktree: worktreeID}
	if parentID != "" {
		opts.Lineage = &store.SessionLineage{ParentSessionID: parentID}
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
		},
	)

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
