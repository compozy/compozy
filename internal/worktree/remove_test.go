package worktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/config"
)

// Canonical suite: removal fencing, authoritative safety checks, and boot recovery.
func TestServiceRemoveAndRecover(t *testing.T) {
	t.Parallel()

	t.Run("Should keep removal idempotent and workspace scoped across terminal states", func(t *testing.T) {
		t.Parallel()
		crossWorkspace := newRemovalTestFixture(t)
		if result, err := crossWorkspace.service.Remove(
			context.Background(), "ws-other", crossWorkspace.item.ID, false,
		); !errors.Is(err, ErrNotFound) || result != nil {
			t.Fatalf("Remove(cross-workspace) = (%#v, %v), want ErrNotFound", result, err)
		}

		removed := newRemovalTestFixture(t)
		if err := removed.store.SetState(
			context.Background(), removed.workspace.ID, removed.item.ID, StateRemoved, removed.item.UpdatedAt,
		); err != nil {
			t.Fatalf("seed removed state: %v", err)
		}
		if result, err := removed.service.Remove(
			context.Background(), removed.workspace.ID, removed.item.ID, false,
		); err != nil || result != nil || len(removed.runner.invocations()) != 0 {
			t.Fatalf("Remove(removed) = (%#v, %v), calls=%#v, want idempotent", result, err, removed.runner.invocations())
		}

		missing := newRemovalTestFixture(t)
		if err := missing.store.SetState(
			context.Background(), missing.workspace.ID, missing.item.ID, StateMissing, missing.item.UpdatedAt,
		); err != nil {
			t.Fatalf("seed missing state: %v", err)
		}
		if result, err := missing.service.Remove(
			context.Background(), missing.workspace.ID, missing.item.ID, false,
		); !errors.Is(err, ErrNotReady) || result != nil || len(missing.runner.invocations()) != 0 {
			t.Fatalf("Remove(missing) = (%#v, %v), calls=%#v, want ErrNotReady", result, err, missing.runner.invocations())
		}
	})

	t.Run("Should refuse removal and dismissal when another actor wins the state fence", func(t *testing.T) {
		t.Parallel()
		removal := newRemovalTestFixture(t)
		removal.service.store = &lostStateFenceStore{memoryWorktreeStore: removal.store}
		if result, err := removal.service.Remove(
			context.Background(), removal.workspace.ID, removal.item.ID, false,
		); !errors.Is(err, ErrNotReady) || result != nil || len(removal.runner.invocations()) != 0 {
			t.Fatalf("Remove(lost fence) = (%#v, %v), calls=%#v, want ErrNotReady", result, err, removal.runner.invocations())
		}

		dismissal := newRemovalTestFixture(t)
		if err := dismissal.store.SetState(
			context.Background(), dismissal.workspace.ID, dismissal.item.ID, StateMissing, dismissal.item.UpdatedAt,
		); err != nil {
			t.Fatalf("seed dismissal target: %v", err)
		}
		dismissal.service.store = &lostStateFenceStore{memoryWorktreeStore: dismissal.store}
		if err := dismissal.service.Dismiss(context.Background(), dismissal.workspace.ID, dismissal.item.ID); !errors.Is(err, ErrNotReady) {
			t.Fatalf("Dismiss(lost fence) error = %v, want ErrNotReady", err)
		}
	})

	t.Run("Should remove a clean worktree and leave its branch untouched", func(t *testing.T) {
		t.Parallel()
		fixture := newRemovalTestFixture(t)
		refusalResult, err := fixture.service.Remove(
			context.Background(), fixture.workspace.ID, fixture.item.ID, false,
		)
		if err != nil || refusalResult != nil {
			t.Fatalf("Remove() = %#v, %v, want clean success", refusalResult, err)
		}
		stored, err := fixture.store.Get(context.Background(), fixture.workspace.ID, fixture.item.ID)
		if err != nil || stored.State != StateRemoved {
			t.Fatalf("stored removal = %#v, %v, want removed tombstone", stored, err)
		}
		commands := invocationCommands(fixture.runner.invocations())
		if !containsCommandPrefix(commands, "--git-dir "+fixture.commonDir+" worktree remove ") {
			t.Fatalf("Git calls = %#v, want non-force worktree removal", commands)
		}
		if containsCommandPrefix(commands, "update-ref -d ") {
			t.Fatalf("Git calls = %#v, user branch must survive", commands)
		}
		if got := fixture.events.count(EventRemoved); got != 1 {
			t.Fatalf("removed events = %d, want exactly one", got)
		}
	})

	t.Run("Should refuse dirty state with an inventory and restore the ready fence", func(t *testing.T) {
		t.Parallel()
		fixture := newRemovalTestFixture(t)
		fixture.dirty.Store(true)
		refusalResult, err := fixture.service.Remove(
			context.Background(), fixture.workspace.ID, fixture.item.ID, false,
		)
		if !errors.Is(err, ErrDirtyRequiresForce) || refusalResult == nil ||
			refusalResult.Risk.ChangedFiles != 1 || refusalResult.Risk.Insertions != 3 ||
			refusalResult.Risk.Deletions != 2 {
			t.Fatalf("Remove(dirty) = %#v, %v, want dirty inventory", refusalResult, err)
		}
		stored, getErr := fixture.store.Get(context.Background(), fixture.workspace.ID, fixture.item.ID)
		if getErr != nil || stored.State != StateReady {
			t.Fatalf("stored refusal = %#v, %v, want ready", stored, getErr)
		}
		if containsCommandPrefix(invocationCommands(fixture.runner.invocations()), "--git-dir "+fixture.commonDir+" worktree remove ") {
			t.Fatal("dirty refusal invoked destructive Git removal")
		}
	})

	t.Run("Should re-read safety after the hook and refuse a new dirty change", func(t *testing.T) {
		t.Parallel()
		fixture := newRemovalTestFixture(t)
		fixture.service.hooks = removalHookDispatcher{dispatch: func(request HookRequest) (HookVerdict, error) {
			if request.Event == EventPreRemove {
				fixture.dirty.Store(true)
			}
			return HookVerdict{}, nil
		}}
		refusalResult, err := fixture.service.Remove(
			context.Background(), fixture.workspace.ID, fixture.item.ID, false,
		)
		if !errors.Is(err, ErrDirtyRequiresForce) || refusalResult == nil {
			t.Fatalf("Remove(TOCTOU) = %#v, %v, want dirty refusal", refusalResult, err)
		}
		if containsCommandPrefix(invocationCommands(fixture.runner.invocations()), "--git-dir "+fixture.commonDir+" worktree remove ") {
			t.Fatal("post-hook dirty state was removed")
		}
	})

	t.Run("Should block an active bound session before mutation", func(t *testing.T) {
		t.Parallel()
		fixture := newRemovalTestFixture(t)
		WithSessionGuard(fixedSessionGuard{active: true})(fixture.service)
		if _, err := fixture.service.Remove(
			context.Background(), fixture.workspace.ID, fixture.item.ID, true,
		); !errors.Is(err, ErrSessionActive) {
			t.Fatalf("Remove(active session) error = %v, want ErrSessionActive", err)
		}
		stored, err := fixture.store.Get(context.Background(), fixture.workspace.ID, fixture.item.ID)
		if err != nil || stored.State != StateReady {
			t.Fatalf("stored active-session refusal = %#v, %v, want ready", stored, err)
		}
	})

	t.Run("Should preserve the fence when session and status safety checks fail", func(t *testing.T) {
		t.Parallel()
		sessionFailure := newRemovalTestFixture(t)
		WithSessionGuard(fixedSessionGuard{err: errors.New("session store unavailable")})(sessionFailure.service)
		if _, err := sessionFailure.service.Remove(
			context.Background(), sessionFailure.workspace.ID, sessionFailure.item.ID, false,
		); err == nil || !strings.Contains(err.Error(), "inspect active sessions") {
			t.Fatalf("Remove(session check failure) error = %v, want safety failure", err)
		}
		if got := sessionFailure.mustItem(t).State; got != StateReady {
			t.Fatalf("session check failure state = %q, want ready", got)
		}

		statusFailure := newRemovalTestFixture(t)
		baseRun := statusFailure.runner.run
		statusFailure.runner.run = func(call gitInvocation) gitResponse {
			if strings.Join(call.args, " ") == "status --porcelain=v2 --branch -z" {
				return gitResponse{stderr: []byte("cannot read index"), err: errors.New("exit 128")}
			}
			return baseRun(call)
		}
		if _, err := statusFailure.service.Remove(
			context.Background(), statusFailure.workspace.ID, statusFailure.item.ID, false,
		); !errors.Is(err, ErrSafetyCheckFailed) {
			t.Fatalf("Remove(status failure) error = %v, want ErrSafetyCheckFailed", err)
		}
		if got := statusFailure.mustItem(t).State; got != StateReady {
			t.Fatalf("status failure state = %q, want ready", got)
		}
	})

	t.Run("Should refuse unpushed commits unless a remote contains the head", func(t *testing.T) {
		t.Parallel()
		refused := newRemovalTestFixture(t)
		refused.uniqueCommits.Store(2)
		refusalResult, err := refused.service.Remove(
			context.Background(), refused.workspace.ID, refused.item.ID, false,
		)
		if !errors.Is(err, ErrUnpushedRequiresForce) || refusalResult == nil ||
			refusalResult.Risk.UnpushedCommits != 2 {
			t.Fatalf("Remove(unpushed) = (%#v, %v), want unpushed refusal", refusalResult, err)
		}

		contained := newRemovalTestFixture(t)
		contained.uniqueCommits.Store(2)
		contained.remoteContains.Store(true)
		refusalResult, err = contained.service.Remove(
			context.Background(), contained.workspace.ID, contained.item.ID, false,
		)
		if err != nil || refusalResult != nil {
			t.Fatalf("Remove(remote-contained) = (%#v, %v), want success", refusalResult, err)
		}
	})

	t.Run("Should use force only when explicit and restore ready after Git failure", func(t *testing.T) {
		t.Parallel()
		forced := newRemovalTestFixture(t)
		forced.dirty.Store(true)
		if refusalResult, err := forced.service.Remove(
			context.Background(), forced.workspace.ID, forced.item.ID, true,
		); err != nil || refusalResult != nil {
			t.Fatalf("Remove(force) = (%#v, %v), want success", refusalResult, err)
		}
		if !containsCommandPrefix(
			invocationCommands(forced.runner.invocations()),
			"--git-dir "+forced.commonDir+" worktree remove --force ",
		) {
			t.Fatal("forced removal omitted the explicit --force flag")
		}

		failed := newRemovalTestFixture(t)
		failed.removeFails.Store(true)
		if _, err := failed.service.Remove(
			context.Background(), failed.workspace.ID, failed.item.ID, false,
		); !errors.Is(err, ErrRemovalFailed) {
			t.Fatalf("Remove(Git failure) error = %v, want ErrRemovalFailed", err)
		}
		stored, err := failed.store.Get(context.Background(), failed.workspace.ID, failed.item.ID)
		if err != nil || stored.State != StateReady {
			t.Fatalf("stored Git failure = %#v, %v, want retriable ready", stored, err)
		}
	})

	t.Run("Should reclaim only an unchanged runtime-owned branch", func(t *testing.T) {
		t.Parallel()
		fixture := newRemovalTestFixture(t)
		fixture.item.Branch = "run/remove"
		fixture.item.CreatedBranch = true
		fixture.item.RunNamespace = "run/"
		fixture.item.CreatedHead = "base-head"
		fixture.store.mu.Lock()
		fixture.store.items[worktreeStoreKey(fixture.workspace.ID, fixture.item.ID)] = fixture.item
		fixture.store.mu.Unlock()
		if refusalResult, err := fixture.service.Remove(
			context.Background(), fixture.workspace.ID, fixture.item.ID, false,
		); err != nil || refusalResult != nil {
			t.Fatalf("Remove(runtime-owned) = (%#v, %v), want success", refusalResult, err)
		}
		if !containsCommandPrefix(
			invocationCommands(fixture.runner.invocations()),
			"update-ref -d refs/heads/run/remove base-head",
		) {
			t.Fatal("runtime-owned branch was not compare-deleted")
		}
		if got := fixture.events.count(EventBranchReclaimed); got != 1 {
			t.Fatalf("branch reclaimed events = %d, want one", got)
		}
	})

	t.Run("Should recover pending phases from durable ownership checkpoints", func(t *testing.T) {
		t.Parallel()
		fixture := newRemovalTestFixture(t)
		branchPending := fixture.item
		branchPending.ID, branchPending.Name = "wt-branch", "branch-recovery"
		branchPending.Path = t.TempDir()
		branchPending.State, branchPending.PendingPhase = StatePending, PhaseBranch
		branchPending.CreatedBranch, branchPending.CreatedHead = true, "base-head"
		branchPending.Branch = "run/recover"
		copyPending := fixture.item
		copyPending.ID, copyPending.Name = "wt-copy", "copy-recovery"
		copyPending.Path = t.TempDir()
		copyPending.State, copyPending.PendingPhase = StatePending, PhaseCopy
		setupPending := fixture.item
		setupPending.ID, setupPending.Name = "wt-setup", "setup-recovery"
		setupPending.Path = t.TempDir()
		setupPending.State, setupPending.PendingPhase = StatePending, PhaseSetup
		if err := fixture.store.Insert(context.Background(), branchPending); err != nil {
			t.Fatalf("seed branch recovery: %v", err)
		}
		if err := fixture.store.Insert(context.Background(), copyPending); err != nil {
			t.Fatalf("seed copy recovery: %v", err)
		}
		if err := fixture.store.Insert(context.Background(), setupPending); err != nil {
			t.Fatalf("seed setup recovery: %v", err)
		}
		if err := fixture.service.RecoverCreations(context.Background()); err != nil {
			t.Fatalf("RecoverCreations() error = %v", err)
		}
		branchStored, branchErr := fixture.store.Get(context.Background(), fixture.workspace.ID, branchPending.ID)
		copyStored, copyErr := fixture.store.Get(context.Background(), fixture.workspace.ID, copyPending.ID)
		setupStored, setupErr := fixture.store.Get(context.Background(), fixture.workspace.ID, setupPending.ID)
		if branchErr != nil || branchStored.State != StateFailed {
			t.Fatalf("branch recovery = %#v, %v, want failed", branchStored, branchErr)
		}
		if copyErr != nil || copyStored.State != StateReady || copyStored.SetupState != SetupFailed ||
			!strings.Contains(copyStored.SetupError, "daemon restart") {
			t.Fatalf("copy recovery = %#v, %v, want usable setup failure", copyStored, copyErr)
		}
		if setupErr != nil || setupStored.State != StateReady || setupStored.SetupState != SetupFailed ||
			!strings.Contains(setupStored.SetupError, "daemon restart") {
			t.Fatalf("setup recovery = %#v, %v, want usable setup failure", setupStored, setupErr)
		}
		if !containsCommandPrefix(invocationCommands(fixture.runner.invocations()), "update-ref -d refs/heads/run/recover base-head") {
			t.Fatal("branch-phase recovery did not compare-delete the recorded branch")
		}
		if got := fixture.events.count(EventSetupFailed); got != 2 {
			t.Fatalf("setup failed events = %d, want two", got)
		}
	})

	t.Run("Should resume a stale removal and fail interrupted exit operations", func(t *testing.T) {
		t.Parallel()
		fixture := newRemovalTestFixture(t)
		if err := fixture.store.SetState(
			context.Background(), fixture.workspace.ID, fixture.item.ID, StateRemoving, fixture.item.UpdatedAt,
		); err != nil {
			t.Fatalf("seed removing state: %v", err)
		}
		op := ExitOperation{
			ID: "op-interrupted", WorkspaceID: fixture.workspace.ID, WorktreeID: fixture.item.ID,
			Action: "push", State: "running", StartedAt: fixture.item.CreatedAt,
		}
		if err := fixture.store.InsertExitOperation(context.Background(), op); err != nil {
			t.Fatalf("seed running exit operation: %v", err)
		}
		if err := fixture.service.RecoverCreations(context.Background()); err != nil {
			t.Fatalf("RecoverCreations() error = %v", err)
		}
		if got := fixture.mustItem(t).State; got != StateRemoved {
			t.Fatalf("stale removing state = %q, want removed", got)
		}
		fixture.store.mu.Lock()
		recoveredOp := fixture.store.exitOps[op.ID]
		fixture.store.mu.Unlock()
		if recoveredOp.State != "failed" || recoveredOp.FinishedAt == nil {
			t.Fatalf("interrupted exit operation = %#v, want failed with finished_at", recoveredOp)
		}
	})

	t.Run("Should leave worktree rows untouched when the parent workspace root vanished", func(t *testing.T) {
		t.Parallel()
		fixture := newRemovalTestFixture(t)
		if err := fixture.store.SetState(
			context.Background(), fixture.workspace.ID, fixture.item.ID, StateMissing, fixture.item.UpdatedAt,
		); err != nil {
			t.Fatalf("seed missing row: %v", err)
		}
		vanishedRoot := filepath.Join(t.TempDir(), "vanished")
		fixture.workspace.Root = vanishedRoot
		fixture.service.resolver = staticWorkspaceResolver{workspaces: map[string]Workspace{
			fixture.workspace.ID: fixture.workspace,
		}}
		if err := fixture.service.RecoverCreations(context.Background()); err != nil {
			t.Fatalf("RecoverCreations(vanished workspace) error = %v", err)
		}
		if got := fixture.mustItem(t).State; got != StateMissing {
			t.Fatalf("vanished-workspace row state = %q, want unchanged missing", got)
		}
	})

	t.Run("Should restore only matching missing identities and dismiss tombstones", func(t *testing.T) {
		t.Parallel()
		matching := newRemovalTestFixture(t)
		if err := matching.store.SetState(
			context.Background(), matching.workspace.ID, matching.item.ID, StateMissing, matching.item.UpdatedAt,
		); err != nil {
			t.Fatalf("SetState(missing) error = %v", err)
		}
		if err := matching.service.RecoverCreations(context.Background()); err != nil {
			t.Fatalf("RecoverCreations(matching missing) error = %v", err)
		}
		if got := matching.mustItem(t).State; got != StateReady {
			t.Fatalf("matching missing state = %q, want ready", got)
		}

		dismissed := newRemovalTestFixture(t)
		known := 3
		if err := dismissed.store.SaveStatus(context.Background(), dismissed.workspace.ID, dismissed.item.ID, Status{
			WorktreeID: dismissed.item.ID, DirtyFiles: &known,
		}); err != nil {
			t.Fatalf("seed status history: %v", err)
		}
		op := ExitOperation{
			ID: "op-dismiss-history", WorkspaceID: dismissed.workspace.ID, WorktreeID: dismissed.item.ID,
			Action: "push", State: "completed", StartedAt: dismissed.item.CreatedAt,
		}
		if err := dismissed.store.InsertExitOperation(context.Background(), op); err != nil {
			t.Fatalf("seed operation history: %v", err)
		}
		if err := dismissed.store.SetState(
			context.Background(), dismissed.workspace.ID, dismissed.item.ID, StateMissing, dismissed.item.UpdatedAt,
		); err != nil {
			t.Fatalf("SetState(dismiss target) error = %v", err)
		}
		if err := dismissed.service.Dismiss(
			context.Background(), dismissed.workspace.ID, dismissed.item.ID,
		); err != nil {
			t.Fatalf("Dismiss() error = %v", err)
		}
		if got := dismissed.mustItem(t).State; got != StateDismissed {
			t.Fatalf("dismissed state = %q, want dismissed", got)
		}
		if got := dismissed.events.count(EventDismissed); got != 1 {
			t.Fatalf("dismissed events = %d, want one", got)
		}
		status, statusErr := dismissed.store.GetStatus(context.Background(), dismissed.workspace.ID, dismissed.item.ID)
		if statusErr != nil || status == nil || status.DirtyFiles == nil || *status.DirtyFiles != known {
			t.Fatalf("dismissed status history = %#v, %v, want preserved", status, statusErr)
		}
		dismissed.store.mu.Lock()
		preservedOp, ok := dismissed.store.exitOps[op.ID]
		dismissed.store.mu.Unlock()
		if !ok || preservedOp.WorktreeID != dismissed.item.ID {
			t.Fatalf("dismissed operation history = %#v, want preserved", preservedOp)
		}
	})

	t.Run("Should dismiss only terminal tombstones inside the owning workspace", func(t *testing.T) {
		t.Parallel()
		for _, state := range []State{StateFailed, StateRemoved} {
			state := state
			t.Run("Should dismiss "+string(state), func(t *testing.T) {
				t.Parallel()
				fixture := newRemovalTestFixture(t)
				if err := fixture.store.SetState(
					context.Background(), fixture.workspace.ID, fixture.item.ID, state, fixture.item.UpdatedAt,
				); err != nil {
					t.Fatalf("seed %s state: %v", state, err)
				}
				if err := fixture.service.Dismiss(context.Background(), fixture.workspace.ID, fixture.item.ID); err != nil {
					t.Fatalf("Dismiss(%s) error = %v", state, err)
				}
				if got := fixture.mustItem(t).State; got != StateDismissed {
					t.Fatalf("Dismiss(%s) state = %q, want dismissed", state, got)
				}
			})
		}

		ready := newRemovalTestFixture(t)
		if err := ready.service.Dismiss(context.Background(), ready.workspace.ID, ready.item.ID); !errors.Is(err, ErrNotReady) {
			t.Fatalf("Dismiss(ready) error = %v, want ErrNotReady", err)
		}
		if err := ready.service.Dismiss(context.Background(), "ws-other", ready.item.ID); !errors.Is(err, ErrNotFound) {
			t.Fatalf("Dismiss(cross-workspace) error = %v, want ErrNotFound", err)
		}
	})
}

type removalTestFixture struct {
	service        *Service
	store          *memoryWorktreeStore
	runner         *recordingGitRunner
	events         *recordingEventSink
	workspace      Workspace
	item           Worktree
	commonDir      string
	dirty          atomicBool
	uniqueCommits  atomicInt
	remoteContains atomicBool
	removeFails    atomicBool
}

type lostStateFenceStore struct {
	*memoryWorktreeStore
}

func (s *lostStateFenceStore) CompareAndSwapState(
	context.Context,
	string,
	string,
	State,
	State,
	time.Time,
) (bool, error) {
	return false, nil
}

type atomicBool struct {
	mu    sync.Mutex
	value bool
}

func (b *atomicBool) Store(value bool) {
	b.mu.Lock()
	b.value = value
	b.mu.Unlock()
}

func (b *atomicBool) Load() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.value
}

type atomicInt struct {
	mu    sync.Mutex
	value int
}

func (v *atomicInt) Store(value int) {
	v.mu.Lock()
	v.value = value
	v.mu.Unlock()
}

func (v *atomicInt) Load() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.value
}

func newRemovalTestFixture(t *testing.T) *removalTestFixture {
	t.Helper()
	workspaceRoot := t.TempDir()
	canonicalRoot, err := filepath.EvalSymlinks(workspaceRoot)
	if err != nil {
		t.Fatalf("canonicalize workspace root: %v", err)
	}
	commonDir := filepath.Join(canonicalRoot, ".git")
	pathRaw := t.TempDir()
	path, err := filepath.EvalSymlinks(pathRaw)
	if err != nil {
		t.Fatalf("canonicalize worktree path: %v", err)
	}
	adminGitDir := filepath.Join(commonDir, "worktrees", "removal")
	if err := os.MkdirAll(adminGitDir, 0o700); err != nil {
		t.Fatalf("create worktree admin dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(path, ".git"), []byte("gitdir: "+adminGitDir+"\n"), 0o600); err != nil {
		t.Fatalf("write worktree pointer: %v", err)
	}
	if err := os.WriteFile(filepath.Join(adminGitDir, "commondir"), []byte("../..\n"), 0o600); err != nil {
		t.Fatalf("write common pointer: %v", err)
	}
	if err := os.WriteFile(filepath.Join(adminGitDir, "gitdir"), []byte(filepath.Join(path, ".git")+"\n"), 0o600); err != nil {
		t.Fatalf("write worktree backlink: %v", err)
	}
	workspace := Workspace{ID: "ws-remove", Name: "Remove", Root: canonicalRoot}
	now := time.Date(2026, 8, 12, 16, 0, 0, 0, time.UTC)
	item := Worktree{
		ID: "wt-remove", WorkspaceID: workspace.ID, Name: "remove", Branch: "feature/remove",
		Path: path, GitDir: adminGitDir, State: StateReady, Origin: OriginManual,
		SetupState: SetupNone, BaseRef: "main", CreatedAt: now, UpdatedAt: now,
	}
	fixture := &removalTestFixture{workspace: workspace, item: item, commonDir: commonDir}
	runner := &recordingGitRunner{run: func(call gitInvocation) gitResponse {
		command := strings.Join(call.args, " ")
		switch {
		case command == "rev-parse --path-format=absolute --git-common-dir":
			return gitResponse{stdout: []byte(commonDir + "\n")}
		case command == "status --porcelain=v2 --branch -z":
			status := "# branch.oid head\x00# branch.head " + fixture.item.Branch + "\x00"
			if fixture.dirty.Load() {
				status += "? changed.txt\x00"
			}
			return gitResponse{stdout: []byte(status)}
		case command == "diff --numstat -z":
			if fixture.dirty.Load() {
				return gitResponse{stdout: []byte("3\t2\tchanged.txt\x00")}
			}
			return gitResponse{}
		case command == "diff --cached --numstat -z":
			return gitResponse{}
		case command == "rev-list --count main..HEAD":
			return gitResponse{stdout: []byte(strconv.Itoa(fixture.uniqueCommits.Load()) + "\n")}
		case command == "branch -r --contains HEAD":
			if fixture.remoteContains.Load() {
				return gitResponse{stdout: []byte("  origin/feature/remove\n")}
			}
			return gitResponse{}
		case strings.HasPrefix(command, "--git-dir "+commonDir+" worktree remove "):
			if fixture.removeFails.Load() {
				return gitResponse{stderr: []byte("resource busy"), err: errors.New("exit 1")}
			}
			if err := os.RemoveAll(path); err != nil {
				return gitResponse{err: err}
			}
			if err := os.RemoveAll(adminGitDir); err != nil {
				return gitResponse{err: err}
			}
			return gitResponse{}
		case strings.HasPrefix(command, "update-ref -d "):
			return gitResponse{}
		default:
			return gitResponse{err: errors.New("unexpected removal Git command: " + command)}
		}
	}}
	store := newMemoryWorktreeStore()
	if err := store.Insert(context.Background(), item); err != nil {
		t.Fatalf("seed removal worktree: %v", err)
	}
	events := &recordingEventSink{}
	service := NewService(
		store, runner, WithCapabilityGate(readyCapabilityGate()),
		WithWorkspaceResolver(staticWorkspaceResolver{workspaces: map[string]Workspace{workspace.ID: workspace}}),
		WithConfig(config.DefaultWorktreesConfig(), t.TempDir()), WithEvents(events),
		WithClock(func() time.Time { return now }),
	)
	fixture.service, fixture.store, fixture.runner, fixture.events = service, store, runner, events
	return fixture
}

func (f *removalTestFixture) mustItem(t *testing.T) Worktree {
	t.Helper()
	item, err := f.store.Get(context.Background(), f.workspace.ID, f.item.ID)
	if err != nil || item == nil {
		t.Fatalf("store.Get() = (%#v, %v)", item, err)
	}
	return *item
}

type removalHookDispatcher struct {
	dispatch func(HookRequest) (HookVerdict, error)
}

func (d removalHookDispatcher) DispatchWorktreeHook(
	_ context.Context,
	request HookRequest,
) (HookVerdict, error) {
	return d.dispatch(request)
}

type fixedSessionGuard struct {
	active bool
	err    error
}

func (g fixedSessionGuard) HasActiveSession(context.Context, string, string) (bool, error) {
	return g.active, g.err
}

type recordingEventSink struct {
	mu     sync.Mutex
	events []LifecycleEvent
}

func (s *recordingEventSink) PublishWorktreeEvent(_ context.Context, event LifecycleEvent) error {
	s.mu.Lock()
	s.events = append(s.events, event)
	s.mu.Unlock()
	return nil
}

func (s *recordingEventSink) count(name string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, event := range s.events {
		if event.Name == name {
			count++
		}
	}
	return count
}
