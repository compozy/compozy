//go:build integration

package worktree

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/config"
)

// Canonical integration suite: the domain lifecycle against a real Git repository.
func TestWorktreeLifecycleIntegration(t *testing.T) {
	t.Parallel()

	fixture := newRealGitFixture(t)

	t.Run("Should materialize bootstrap inspect and safely remove a real linked worktree", func(t *testing.T) {
		item, err := fixture.service.Create(
			context.Background(), fixture.workspace.ID, CreateOptions{Name: "Feature A"},
		)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if item.State != StateReady || item.CreatedHead == "" || item.BaseRef != "main" {
			t.Fatalf("created item = %#v, want ready item with recorded main head", item)
		}
		wantPath, err := canonicalFuturePath(filepath.Join(fixture.worktreesRoot, "real-workspace", "feature-a"))
		if err != nil {
			t.Fatalf("canonicalFuturePath() error = %v", err)
		}
		if got, want := item.Path, wantPath; got != want {
			t.Fatalf("item.Path = %q, want %q", got, want)
		}
		assertFileContent(t, filepath.Join(item.Path, ".env"), "local=true\n")
		assertFileContent(t, filepath.Join(item.Path, ".setup-marker"), item.ID+"\n")
		if output := fixture.git(item.Path, "status", "--porcelain", "--ignored"); !strings.Contains(output, "!! .env") {
			t.Fatalf("ignored status = %q, want copied .env", output)
		}
		if got := fixture.git(fixture.workspace.Root, "config", "--get", "branch.feature-a.gh-merge-base"); got != "main" {
			t.Fatalf("gh-merge-base = %q, want main", got)
		}
		if got := fixture.git(fixture.workspace.Root, "rev-parse", "feature-a"); got != item.CreatedHead {
			t.Fatalf("branch head = %q, want recorded %q", got, item.CreatedHead)
		}

		if err := os.WriteFile(filepath.Join(item.Path, "README.md"), []byte("changed\n"), 0o600); err != nil {
			t.Fatalf("WriteFile(README.md) error = %v", err)
		}
		status, err := fixture.service.Status(context.Background(), fixture.workspace.ID, item.ID, true)
		if err != nil {
			t.Fatalf("Status() error = %v", err)
		}
		if status.DirtyFiles == nil || *status.DirtyFiles != 1 || status.Insertions == nil || *status.Insertions != 1 {
			t.Fatalf("dirty status = %#v, want one changed file and one insertion", status)
		}
		refusalResult, err := fixture.service.Remove(context.Background(), fixture.workspace.ID, item.ID, false)
		if !errors.Is(err, ErrDirtyRequiresForce) || refusalResult == nil || refusalResult.Risk.ChangedFiles != 1 {
			t.Fatalf("Remove(dirty) = (%#v, %v), want dirty refusal", refusalResult, err)
		}
		persisted := fixture.item(item.ID)
		if persisted.State != StateReady {
			t.Fatalf("state after refusal = %q, want ready", persisted.State)
		}

		if err := os.WriteFile(filepath.Join(item.Path, "README.md"), []byte("initial\n"), 0o600); err != nil {
			t.Fatalf("WriteFile(README.md restore) error = %v", err)
		}
		refusalResult, err = fixture.service.Remove(context.Background(), fixture.workspace.ID, item.ID, false)
		if err != nil || refusalResult != nil {
			t.Fatalf("Remove(clean) = (%#v, %v), want success", refusalResult, err)
		}
		if _, err := os.Stat(item.Path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Stat(removed path) error = %v, want not-exist", err)
		}
		if got := fixture.git(fixture.workspace.Root, "rev-parse", "--verify", "refs/heads/feature-a"); got == "" {
			t.Fatal("manual branch was removed, want it preserved")
		}
		if got := fixture.item(item.ID).State; got != StateRemoved {
			t.Fatalf("registry state = %q, want removed", got)
		}
	})

	t.Run("Should discover and strictly adopt a real external linked worktree", func(t *testing.T) {
		fixture.git(fixture.workspace.Root, "branch", "external", "main")
		externalPath := filepath.Join(t.TempDir(), "external")
		fixture.git(fixture.workspace.Root, "worktree", "add", externalPath, "external")

		listing, err := fixture.service.List(context.Background(), fixture.workspace.ID, true)
		if err != nil {
			t.Fatalf("List(refresh) error = %v", err)
		}
		if len(listing.Discovered) != 1 ||
			canonicalComparablePath(listing.Discovered[0].Path) != canonicalComparablePath(externalPath) ||
			!listing.Discovered[0].Selectable {
			t.Fatalf("discovered = %#v, want selectable external worktree", listing.Discovered)
		}
		item, err := fixture.service.Adopt(context.Background(), fixture.workspace.ID, externalPath)
		if err != nil {
			t.Fatalf("Adopt() error = %v", err)
		}
		if item.Origin != OriginAdopted || item.GitDir == "" || item.State != StateReady {
			t.Fatalf("adopted item = %#v, want ready identity", item)
		}
		again, err := fixture.service.Adopt(context.Background(), fixture.workspace.ID, externalPath)
		if err != nil || again.ID != item.ID {
			t.Fatalf("Adopt(retry) = (%#v, %v), want idempotent %q", again, err, item.ID)
		}
		fixture.git(fixture.workspace.Root, "branch", "discovered-alongside-adopted", "main")
		discoveredPath := filepath.Join(t.TempDir(), "discovered-alongside-adopted")
		fixture.git(
			fixture.workspace.Root,
			"worktree",
			"add",
			discoveredPath,
			"discovered-alongside-adopted",
		)
		merged, err := fixture.service.List(context.Background(), fixture.workspace.ID, true)
		adoptedFound := false
		if err == nil {
			for _, registered := range merged.Worktrees {
				if registered.ID == item.ID && registered.State == StateReady {
					adoptedFound = true
					break
				}
			}
		}
		if err != nil || !adoptedFound || len(merged.Discovered) != 1 ||
			canonicalComparablePath(merged.Discovered[0].Path) != canonicalComparablePath(discoveredPath) {
			t.Fatalf("List(adopted plus discovered) = %#v, %v, want one registry row and one Git-only row", merged, err)
		}
		if _, err := fixture.service.Adopt(context.Background(), fixture.workspace.ID, fixture.workspace.Root); !errors.Is(
			err, ErrAdoptionMainCheckout,
		) {
			t.Fatalf("Adopt(main) error = %v, want ErrAdoptionMainCheckout", err)
		}
		if refusalResult, err := fixture.service.Remove(
			context.Background(), fixture.workspace.ID, item.ID, false,
		); err != nil || refusalResult != nil {
			t.Fatalf("Remove(adopted) = (%#v, %v), want success", refusalResult, err)
		}
	})

	t.Run("Should recover a checkout-phase crash from persisted ownership", func(t *testing.T) {
		branch := "recovery"
		path := filepath.Join(fixture.worktreesRoot, "real-workspace", branch)
		fixture.git(fixture.workspace.Root, "branch", branch, "main")
		fixture.git(fixture.workspace.Root, "worktree", "add", path, branch)
		gitDir := fixture.git(path, "rev-parse", "--path-format=absolute", "--git-dir")
		head := fixture.git(fixture.workspace.Root, "rev-parse", branch)
		now := time.Date(2026, 8, 12, 20, 0, 0, 0, time.UTC)
		item := Worktree{
			ID: "wt-recovery", WorkspaceID: fixture.workspace.ID, Name: branch, Branch: branch,
			Path: path, GitDir: gitDir, State: StatePending, PendingPhase: PhaseCheckout,
			Origin: OriginManual, SetupState: SetupNone, BaseRef: "main", CreatedBranch: true,
			CreatedHead: head, CreatedAt: now, UpdatedAt: now,
		}
		if err := fixture.store.Insert(context.Background(), item); err != nil {
			t.Fatalf("Insert(recovery row) error = %v", err)
		}
		if err := fixture.service.RecoverCreations(context.Background()); err != nil {
			t.Fatalf("RecoverCreations() error = %v", err)
		}
		if got := fixture.item(item.ID).State; got != StateFailed {
			t.Fatalf("recovered state = %q, want failed", got)
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Stat(recovered checkout) error = %v, want not-exist", err)
		}
		if _, stderr, err := fixture.runner.Run(
			context.Background(), fixture.workspace.Root, "rev-parse", "--verify", "refs/heads/"+branch,
		); err == nil {
			t.Fatalf("recovery branch still exists; stderr=%q", strings.TrimSpace(string(stderr)))
		}

		usableBranch := "recovery-usable"
		usablePath := filepath.Join(fixture.worktreesRoot, "real-workspace", usableBranch)
		fixture.git(fixture.workspace.Root, "branch", usableBranch, "main")
		fixture.git(fixture.workspace.Root, "worktree", "add", usablePath, usableBranch)
		usableGitDir := fixture.git(usablePath, "rev-parse", "--path-format=absolute", "--git-dir")
		usable := Worktree{
			ID: "wt-recovery-usable", WorkspaceID: fixture.workspace.ID, Name: usableBranch, Branch: usableBranch,
			Path: usablePath, GitDir: usableGitDir, State: StatePending, PendingPhase: PhaseCopy,
			Origin: OriginManual, SetupState: SetupNone, BaseRef: "main", CreatedBranch: true,
			CreatedHead: head, CreatedAt: now, UpdatedAt: now,
		}
		if err := fixture.store.Insert(context.Background(), usable); err != nil {
			t.Fatalf("Insert(usable recovery row) error = %v", err)
		}
		if err := fixture.service.RecoverCreations(context.Background()); err != nil {
			t.Fatalf("RecoverCreations(usable) error = %v", err)
		}
		usable = fixture.item(usable.ID)
		if usable.State != StateReady || usable.SetupState != SetupFailed ||
			!strings.Contains(usable.SetupError, "daemon restart") {
			t.Fatalf("usable recovery = %#v, want ready checkout with interrupted setup", usable)
		}
		if _, err := os.Stat(usablePath); err != nil {
			t.Fatalf("Stat(usable recovery path) error = %v", err)
		}

		absentBranch := "recovery-absent"
		absentPath := filepath.Join(fixture.worktreesRoot, "real-workspace", absentBranch)
		fixture.git(fixture.workspace.Root, "branch", absentBranch, "main")
		absent := Worktree{
			ID: "wt-recovery-absent", WorkspaceID: fixture.workspace.ID, Name: absentBranch, Branch: absentBranch,
			Path: absentPath, State: StatePending, PendingPhase: PhaseCheckout,
			Origin: OriginManual, SetupState: SetupNone, BaseRef: "main", CreatedBranch: true,
			CreatedHead: head, CreatedAt: now, UpdatedAt: now,
		}
		if err := fixture.store.Insert(context.Background(), absent); err != nil {
			t.Fatalf("Insert(absent recovery row) error = %v", err)
		}
		if err := fixture.service.RecoverCreations(context.Background()); err != nil {
			t.Fatalf("RecoverCreations(absent) error = %v", err)
		}
		if got := fixture.item(absent.ID).State; got != StateFailed {
			t.Fatalf("absent recovery state = %q, want failed", got)
		}
		if _, err := os.Stat(absentPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Stat(absent recovery path) error = %v, want not-exist", err)
		}
	})

	t.Run("Should classify existing held root and unborn branch cases with real Git", func(t *testing.T) {
		branchFixture := newRealGitFixture(t)
		branchFixture.git(branchFixture.workspace.Root, "branch", "feature/existing", "main")
		existing, err := branchFixture.service.Create(
			context.Background(),
			branchFixture.workspace.ID,
			CreateOptions{Name: "Existing Real", ExistingBranch: "feature/existing"},
		)
		if err != nil || existing.CreatedBranch {
			t.Fatalf("Create(existing) = (%#v, %v), want non-created branch", existing, err)
		}
		if _, err := branchFixture.service.Create(
			context.Background(), branchFixture.workspace.ID, CreateOptions{Name: "Root Held", Branch: "main"},
		); !errors.Is(err, ErrBranchCheckedOutAtRoot) {
			t.Fatalf("Create(root-held) error = %v, want ErrBranchCheckedOutAtRoot", err)
		}
		if _, err := branchFixture.service.Create(
			context.Background(),
			branchFixture.workspace.ID,
			CreateOptions{Name: "Linked Held", Branch: "feature/existing"},
		); !errors.Is(err, ErrBranchHeld) || !strings.Contains(err.Error(), existing.Path) {
			t.Fatalf("Create(linked-held) error = %v, want holder path", err)
		}

		unborn := newRealGitFixtureWithCommit(t, false)
		if _, err := unborn.service.Create(
			context.Background(), unborn.workspace.ID, CreateOptions{Name: "Unborn"},
		); !errors.Is(err, ErrRepoHasNoCommits) {
			t.Fatalf("Create(unborn) error = %v, want ErrRepoHasNoCommits", err)
		}
	})

	t.Run("Should fully unwind a real checkout failure and allow retry", func(t *testing.T) {
		rollback := newRealGitFixture(t)
		injected := &interceptingGitRunner{
			inner: rollback.runner,
			intercept: func(_ string, args []string) ([]byte, []byte, error, bool) {
				if len(args) >= 2 && args[0] == "worktree" && args[1] == "add" {
					return nil, []byte("injected add failure"), errors.New("injected add failure"), true
				}
				return nil, nil, nil, false
			},
		}
		service := rollback.newService(injected)
		_, err := service.Create(
			context.Background(), rollback.workspace.ID, CreateOptions{Name: "Retry Real"},
		)
		if err == nil || !strings.Contains(err.Error(), "injected add failure") {
			t.Fatalf("Create(injected failure) error = %v", err)
		}
		if item, getErr := rollback.store.Get(
			context.Background(), rollback.workspace.ID, "retry-real",
		); !errors.Is(getErr, ErrNotFound) || item != nil {
			t.Fatalf("failed registry row = (%#v, %v), want absent", item, getErr)
		}
		if _, stderr, refErr := rollback.runner.Run(
			context.Background(), rollback.workspace.Root, "show-ref", "--verify", "refs/heads/retry-real",
		); refErr == nil {
			t.Fatalf("failed branch still exists; stderr=%q", strings.TrimSpace(string(stderr)))
		}
		item, err := rollback.service.Create(
			context.Background(), rollback.workspace.ID, CreateOptions{Name: "Retry Real"},
		)
		if err != nil || item.State != StateReady {
			t.Fatalf("Create(retry) = (%#v, %v), want ready", item, err)
		}
	})

	t.Run("Should let one real concurrent creation and adoption own each intent", func(t *testing.T) {
		concurrent := newRealGitFixture(t)
		createResults := make(chan createIntegrationResult, 2)
		start := make(chan struct{})
		for range 2 {
			go func() {
				<-start
				item, err := concurrent.service.Create(
					context.Background(), concurrent.workspace.ID, CreateOptions{Name: "Concurrent Real"},
				)
				createResults <- createIntegrationResult{item: item, err: err}
			}()
		}
		close(start)
		created, conflicts := 0, 0
		for range 2 {
			result := <-createResults
			switch {
			case result.err == nil && result.item != nil && result.item.State == StateReady:
				created++
			case errors.Is(result.err, ErrNameTaken):
				conflicts++
			default:
				t.Fatalf("concurrent Create() = (%#v, %v)", result.item, result.err)
			}
		}
		if created != 1 || conflicts != 1 {
			t.Fatalf("concurrent create = %d ready, %d conflict, want one each", created, conflicts)
		}

		concurrent.git(concurrent.workspace.Root, "branch", "adopt-race", "main")
		adoptPath := filepath.Join(t.TempDir(), "adopt-race")
		concurrent.git(concurrent.workspace.Root, "worktree", "add", adoptPath, "adopt-race")
		adoptResults := make(chan createIntegrationResult, 2)
		start = make(chan struct{})
		for range 2 {
			go func() {
				<-start
				item, err := concurrent.service.Adopt(context.Background(), concurrent.workspace.ID, adoptPath)
				adoptResults <- createIntegrationResult{item: item, err: err}
			}()
		}
		close(start)
		first := <-adoptResults
		second := <-adoptResults
		if first.err != nil || second.err != nil || first.item == nil || second.item == nil || first.item.ID != second.item.ID {
			t.Fatalf("concurrent Adopt() = (%#v, %v) / (%#v, %v), want one identity", first.item, first.err, second.item, second.err)
		}
	})

	t.Run("Should preserve foreign adoption targets byte for byte", func(t *testing.T) {
		owner := newRealGitFixture(t)
		foreign := newRealGitFixture(t)
		foreign.git(foreign.workspace.Root, "branch", "foreign", "main")
		foreignPath := filepath.Join(t.TempDir(), "foreign")
		foreign.git(foreign.workspace.Root, "worktree", "add", foreignPath, "foreign")
		sentinel := filepath.Join(foreignPath, "sentinel.txt")
		if err := os.WriteFile(sentinel, []byte("untouched\n"), 0o600); err != nil {
			t.Fatalf("WriteFile(sentinel) error = %v", err)
		}
		before, err := os.Stat(sentinel)
		if err != nil {
			t.Fatalf("Stat(sentinel before) error = %v", err)
		}
		if _, err := owner.service.Adopt(
			context.Background(), owner.workspace.ID, foreignPath,
		); !errors.Is(err, ErrAdoptionForeignRepo) {
			t.Fatalf("Adopt(foreign) error = %v, want ErrAdoptionForeignRepo", err)
		}
		after, err := os.Stat(sentinel)
		if err != nil {
			t.Fatalf("Stat(sentinel after) error = %v", err)
		}
		assertFileContent(t, sentinel, "untouched\n")
		if !after.ModTime().Equal(before.ModTime()) || after.Size() != before.Size() {
			t.Fatalf("foreign sentinel changed: before=%#v after=%#v", before, after)
		}
	})

	t.Run("Should refuse dirty and unpushed real loss then honor explicit evidence", func(t *testing.T) {
		risk := newRealGitFixture(t)
		dirty, err := risk.service.Create(
			context.Background(), risk.workspace.ID, CreateOptions{Name: "Dirty Force"},
		)
		if err != nil {
			t.Fatalf("Create(dirty force) error = %v", err)
		}
		lostPath := filepath.Join(dirty.Path, "lost.txt")
		if err := os.WriteFile(lostPath, []byte("lost\n"), 0o600); err != nil {
			t.Fatalf("WriteFile(lost) error = %v", err)
		}
		refusalResult, err := risk.service.Remove(context.Background(), risk.workspace.ID, dirty.ID, false)
		if !errors.Is(err, ErrDirtyRequiresForce) || refusalResult == nil || refusalResult.Risk.ChangedFiles != 1 {
			t.Fatalf("Remove(dirty real) = (%#v, %v), want one-file refusal", refusalResult, err)
		}
		if refusalResult, err = risk.service.Remove(
			context.Background(), risk.workspace.ID, dirty.ID, true,
		); err != nil || refusalResult != nil {
			t.Fatalf("Remove(force real) = (%#v, %v), want success", refusalResult, err)
		}
		if _, err := os.Stat(lostPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Stat(lost after force) error = %v, want not-exist", err)
		}

		bare := filepath.Join(t.TempDir(), "origin.git")
		if err := os.MkdirAll(bare, 0o700); err != nil {
			t.Fatalf("MkdirAll(bare) error = %v", err)
		}
		risk.git(bare, "init", "--bare")
		risk.git(risk.workspace.Root, "remote", "add", "origin", bare)
		risk.git(risk.workspace.Root, "push", "-u", "origin", "main")
		unpushed, err := risk.service.Create(
			context.Background(), risk.workspace.ID, CreateOptions{Name: "Unpushed Real"},
		)
		if err != nil {
			t.Fatalf("Create(unpushed) error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(unpushed.Path, "commit.txt"), []byte("commit\n"), 0o600); err != nil {
			t.Fatalf("WriteFile(commit) error = %v", err)
		}
		risk.git(unpushed.Path, "add", "commit.txt")
		risk.git(unpushed.Path, "commit", "-m", "unique")
		refusalResult, err = risk.service.Remove(context.Background(), risk.workspace.ID, unpushed.ID, false)
		if !errors.Is(err, ErrUnpushedRequiresForce) || refusalResult == nil || refusalResult.Risk.UnpushedCommits != 1 {
			t.Fatalf("Remove(unpushed real) = (%#v, %v), want one-commit refusal", refusalResult, err)
		}
		risk.git(unpushed.Path, "push", "-u", "origin", unpushed.Branch)
		refusalResult, err = risk.service.Remove(context.Background(), risk.workspace.ID, unpushed.ID, false)
		if err != nil || refusalResult != nil {
			t.Fatalf("Remove(pushed real) = (%#v, %v), want informational success", refusalResult, err)
		}
	})

	t.Run("Should reclaim only unchanged real runtime branches", func(t *testing.T) {
		reclaim := newRealGitFixture(t)
		unchanged, err := reclaim.service.Create(context.Background(), reclaim.workspace.ID, CreateOptions{
			Name: "Run Clean", Branch: "run/clean", Origin: OriginPerRun, RunNamespace: "run/",
		})
		if err != nil {
			t.Fatalf("Create(run clean) error = %v", err)
		}
		if refusalResult, err := reclaim.service.Remove(
			context.Background(), reclaim.workspace.ID, unchanged.ID, false,
		); err != nil || refusalResult != nil {
			t.Fatalf("Remove(run clean) = (%#v, %v), want success", refusalResult, err)
		}
		if _, stderr, err := reclaim.runner.Run(
			context.Background(), reclaim.workspace.Root, "show-ref", "--verify", "refs/heads/run/clean",
		); err == nil {
			t.Fatalf("unchanged runtime branch survived; stderr=%q", strings.TrimSpace(string(stderr)))
		}

		changed, err := reclaim.service.Create(context.Background(), reclaim.workspace.ID, CreateOptions{
			Name: "Run Changed", Branch: "run/changed", Origin: OriginPerRun, RunNamespace: "run/",
		})
		if err != nil {
			t.Fatalf("Create(run changed) error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(changed.Path, "changed.txt"), []byte("changed\n"), 0o600); err != nil {
			t.Fatalf("WriteFile(changed) error = %v", err)
		}
		reclaim.git(changed.Path, "add", "changed.txt")
		reclaim.git(changed.Path, "commit", "-m", "advance branch")
		if refusalResult, err := reclaim.service.Remove(
			context.Background(), reclaim.workspace.ID, changed.ID, true,
		); err != nil || refusalResult != nil {
			t.Fatalf("Remove(run changed force) = (%#v, %v), want success", refusalResult, err)
		}
		if got := reclaim.git(reclaim.workspace.Root, "show-ref", "--verify", "refs/heads/run/changed"); got == "" {
			t.Fatal("advanced runtime branch was deleted")
		}
	})

	t.Run("Should record real status corruption and recover truthful reads", func(t *testing.T) {
		statusFixture := newRealGitFixture(t)
		item, err := statusFixture.service.Create(
			context.Background(), statusFixture.workspace.ID, CreateOptions{Name: "Status Corrupt"},
		)
		if err != nil {
			t.Fatalf("Create(status corrupt) error = %v", err)
		}
		headPath := filepath.Join(item.GitDir, "HEAD")
		head, err := os.ReadFile(headPath)
		if err != nil {
			t.Fatalf("ReadFile(HEAD) error = %v", err)
		}
		if err := os.WriteFile(headPath, []byte("invalid head\n"), 0o600); err != nil {
			t.Fatalf("WriteFile(corrupt HEAD) error = %v", err)
		}
		status, err := statusFixture.service.Status(context.Background(), statusFixture.workspace.ID, item.ID, true)
		if err != nil || status.ReadError == "" {
			t.Fatalf("Status(corrupt) = (%#v, %v), want persisted read_error", status, err)
		}
		if err := os.WriteFile(headPath, head, 0o600); err != nil {
			t.Fatalf("WriteFile(repair HEAD) error = %v", err)
		}
		status, err = statusFixture.service.Status(context.Background(), statusFixture.workspace.ID, item.ID, true)
		if err != nil || status.ReadError != "" || status.HeadSHA == nil {
			t.Fatalf("Status(repaired) = (%#v, %v), want truthful status", status, err)
		}
		statusFixture.git(item.Path, "checkout", "--detach")
		status, err = statusFixture.service.Status(context.Background(), statusFixture.workspace.ID, item.ID, true)
		if err != nil || status.Detached == nil || !*status.Detached || status.Branch == nil || *status.Branch != "" {
			t.Fatalf("Status(detached) = (%#v, %v), want detached truth", status, err)
		}
	})

	t.Run("Should report real upstream divergence exactly", func(t *testing.T) {
		statusFixture := newRealGitFixture(t)
		bare := filepath.Join(t.TempDir(), "status-origin.git")
		if err := os.MkdirAll(bare, 0o700); err != nil {
			t.Fatalf("MkdirAll(status bare) error = %v", err)
		}
		statusFixture.git(bare, "init", "--bare")
		statusFixture.git(statusFixture.workspace.Root, "remote", "add", "origin", bare)
		statusFixture.git(statusFixture.workspace.Root, "push", "-u", "origin", "main")
		item, err := statusFixture.service.Create(
			context.Background(), statusFixture.workspace.ID, CreateOptions{Name: "Status Matrix"},
		)
		if err != nil {
			t.Fatalf("Create(status matrix) error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(item.Path, "base.txt"), []byte("base\n"), 0o600); err != nil {
			t.Fatalf("WriteFile(status base) error = %v", err)
		}
		statusFixture.git(item.Path, "add", "base.txt")
		statusFixture.git(item.Path, "commit", "-m", "status base")
		status, err := statusFixture.service.Status(
			context.Background(), statusFixture.workspace.ID, item.ID, true,
		)
		if err != nil || status.HasUpstream == nil || *status.HasUpstream || status.Ahead != nil ||
			status.Behind != nil || status.AheadOfBase == nil || *status.AheadOfBase != 1 {
			t.Fatalf("Status(no upstream) = (%#v, %v), want ahead-of-base 1 with unknown remote counts", status, err)
		}
		statusFixture.git(item.Path, "push", "-u", "origin", item.Branch)

		cloneParent := t.TempDir()
		clonePath := filepath.Join(cloneParent, "clone")
		statusFixture.git(cloneParent, "clone", bare, clonePath)
		statusFixture.git(clonePath, "config", "user.name", "Remote Integration")
		statusFixture.git(clonePath, "config", "user.email", "remote@compozy.test")
		statusFixture.git(clonePath, "checkout", "-b", item.Branch, "origin/"+item.Branch)
		if err := os.WriteFile(filepath.Join(clonePath, "remote.txt"), []byte("remote\n"), 0o600); err != nil {
			t.Fatalf("WriteFile(remote advance) error = %v", err)
		}
		statusFixture.git(clonePath, "add", "remote.txt")
		statusFixture.git(clonePath, "commit", "-m", "remote advance")
		statusFixture.git(clonePath, "push", "origin", item.Branch)

		if err := os.WriteFile(filepath.Join(item.Path, "local.txt"), []byte("local\n"), 0o600); err != nil {
			t.Fatalf("WriteFile(local advance) error = %v", err)
		}
		statusFixture.git(item.Path, "add", "local.txt")
		statusFixture.git(item.Path, "commit", "-m", "local advance")
		statusFixture.git(item.Path, "fetch", "origin")
		status, err = statusFixture.service.Status(
			context.Background(), statusFixture.workspace.ID, item.ID, true,
		)
		if err != nil || status.Ahead == nil || *status.Ahead != 1 || status.Behind == nil || *status.Behind != 1 ||
			status.HasUpstream == nil || !*status.HasUpstream {
			t.Fatalf("Status(diverged) = (%#v, %v), want upstream +1 -1", status, err)
		}
	})

	t.Run("Should discover stale worktrees without mutating Git and honor the cache", func(t *testing.T) {
		discovery := newRealGitFixture(t)
		discovery.git(discovery.workspace.Root, "branch", "stale-external", "main")
		stalePath := filepath.Join(t.TempDir(), "stale-external")
		discovery.git(discovery.workspace.Root, "worktree", "add", stalePath, "stale-external")
		if err := os.RemoveAll(stalePath); err != nil {
			t.Fatalf("RemoveAll(stale worktree) error = %v", err)
		}
		before, stderr, err := discovery.runner.Run(
			context.Background(), discovery.workspace.Root, "worktree", "list", "--porcelain", "-z",
		)
		if err != nil {
			t.Fatalf("git worktree list before error = %v; stderr=%q", err, strings.TrimSpace(string(stderr)))
		}
		listing, err := discovery.service.List(context.Background(), discovery.workspace.ID, true)
		if err != nil {
			t.Fatalf("List(stale) error = %v", err)
		}
		if len(listing.Discovered) != 1 || !listing.Discovered[0].Stale || listing.Discovered[0].Selectable {
			t.Fatalf("List(stale) discovered = %#v, want stale non-selectable", listing.Discovered)
		}
		after, stderr, err := discovery.runner.Run(
			context.Background(), discovery.workspace.Root, "worktree", "list", "--porcelain", "-z",
		)
		if err != nil {
			t.Fatalf("git worktree list after error = %v; stderr=%q", err, strings.TrimSpace(string(stderr)))
		}
		if string(after) != string(before) {
			t.Fatalf("discovery mutated worktree metadata:\nbefore=%q\nafter=%q", before, after)
		}
		if _, err := os.Stat(filepath.Join(discovery.workspace.Root, ".git", "FETCH_HEAD")); !errors.Is(
			err, os.ErrNotExist,
		) {
			t.Fatalf("FETCH_HEAD stat error = %v, want discovery to avoid fetch", err)
		}

		cached := newRealGitFixture(t)
		var listCalls atomic.Int64
		counting := &interceptingGitRunner{
			inner: cached.runner,
			intercept: func(_ string, args []string) ([]byte, []byte, error, bool) {
				if strings.Contains(strings.Join(args, " "), "worktree list --porcelain -z") {
					listCalls.Add(1)
				}
				return nil, nil, nil, false
			},
		}
		service := cached.newService(counting)
		if _, err := service.List(context.Background(), cached.workspace.ID, false); err != nil {
			t.Fatalf("List(cache first) error = %v", err)
		}
		if _, err := service.List(context.Background(), cached.workspace.ID, false); err != nil {
			t.Fatalf("List(cache second) error = %v", err)
		}
		if got := listCalls.Load(); got != 1 {
			t.Fatalf("cached worktree list calls = %d, want one", got)
		}
		if _, err := service.List(context.Background(), cached.workspace.ID, true); err != nil {
			t.Fatalf("List(cache refresh) error = %v", err)
		}
		if got := listCalls.Load(); got != 2 {
			t.Fatalf("refreshed worktree list calls = %d, want two", got)
		}
		if _, err := service.Create(
			context.Background(), cached.workspace.ID, CreateOptions{Name: "Cache Invalidate"},
		); err != nil {
			t.Fatalf("Create(cache invalidate) error = %v", err)
		}
		beforePostCreate := listCalls.Load()
		if _, err := service.List(context.Background(), cached.workspace.ID, false); err != nil {
			t.Fatalf("List(after create) error = %v", err)
		}
		if got := listCalls.Load(); got != beforePostCreate+1 {
			t.Fatalf("post-create worktree list calls = %d, want invalidated cache after %d", got, beforePostCreate)
		}
	})

	t.Run("Should restore dismiss and preserve unrelated replacements for missing rows", func(t *testing.T) {
		reconcile := newRealGitFixture(t)
		item, err := reconcile.service.Create(
			context.Background(), reconcile.workspace.ID, CreateOptions{Name: "Missing Restore"},
		)
		if err != nil {
			t.Fatalf("Create(missing restore) error = %v", err)
		}
		reconcile.git(reconcile.workspace.Root, "worktree", "remove", "--force", item.Path)
		listing, err := reconcile.service.List(context.Background(), reconcile.workspace.ID, true)
		if err != nil || len(listing.Worktrees) != 1 || listing.Worktrees[0].State != StateMissing {
			t.Fatalf("List(out-of-band removal) = (%#v, %v), want missing", listing, err)
		}
		reconcile.git(reconcile.workspace.Root, "worktree", "add", item.Path, item.Branch)
		if err := reconcile.service.RecoverCreations(context.Background()); err != nil {
			t.Fatalf("RecoverCreations(restored) error = %v", err)
		}
		if got := reconcile.item(item.ID).State; got != StateReady {
			t.Fatalf("restored state = %q, want ready", got)
		}
		reconcile.git(reconcile.workspace.Root, "worktree", "remove", "--force", item.Path)
		if _, err := reconcile.service.List(context.Background(), reconcile.workspace.ID, true); err != nil {
			t.Fatalf("List(second missing) error = %v", err)
		}
		if err := reconcile.service.Dismiss(context.Background(), reconcile.workspace.ID, item.ID); err != nil {
			t.Fatalf("Dismiss(missing) error = %v", err)
		}
		if got := reconcile.item(item.ID).State; got != StateDismissed {
			t.Fatalf("dismissed state = %q, want dismissed", got)
		}

		unrelated, err := reconcile.service.Create(
			context.Background(), reconcile.workspace.ID, CreateOptions{Name: "Unrelated Replacement"},
		)
		if err != nil {
			t.Fatalf("Create(unrelated replacement) error = %v", err)
		}
		reconcile.git(reconcile.workspace.Root, "worktree", "remove", "--force", unrelated.Path)
		if _, err := reconcile.service.List(context.Background(), reconcile.workspace.ID, true); err != nil {
			t.Fatalf("List(unrelated missing) error = %v", err)
		}
		if err := os.MkdirAll(unrelated.Path, 0o700); err != nil {
			t.Fatalf("MkdirAll(unrelated path) error = %v", err)
		}
		reconcile.git(unrelated.Path, "init", "-b", "main")
		if err := reconcile.service.RecoverCreations(context.Background()); err != nil {
			t.Fatalf("RecoverCreations(unrelated) error = %v", err)
		}
		if got := reconcile.item(unrelated.ID).State; got != StateMissing {
			t.Fatalf("unrelated replacement state = %q, want missing", got)
		}
	})

	t.Run("Should clean only a Git-identity-matching forced leftover", func(t *testing.T) {
		leftover := newRealGitFixture(t)
		item, err := leftover.service.Create(
			context.Background(), leftover.workspace.ID, CreateOptions{Name: "Matching Leftover"},
		)
		if err != nil {
			t.Fatalf("Create(matching leftover) error = %v", err)
		}
		failingRunner := &interceptingGitRunner{
			inner: leftover.runner,
			intercept: func(_ string, args []string) ([]byte, []byte, error, bool) {
				if strings.Contains(strings.Join(args, " "), "worktree remove --force") {
					return nil, []byte("resource busy"), errors.New("resource busy"), true
				}
				return nil, nil, nil, false
			},
		}
		service := leftover.newService(failingRunner)
		if refusalResult, err := service.Remove(
			context.Background(), leftover.workspace.ID, item.ID, true,
		); err != nil || refusalResult != nil {
			t.Fatalf("Remove(matching leftover) = (%#v, %v), want verified cleanup", refusalResult, err)
		}
		if _, err := os.Stat(item.Path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Stat(matching leftover) error = %v, want not-exist", err)
		}

		foreign := newRealGitFixture(t)
		foreignItem, err := foreign.service.Create(
			context.Background(), foreign.workspace.ID, CreateOptions{Name: "Foreign Leftover"},
		)
		if err != nil {
			t.Fatalf("Create(foreign leftover) error = %v", err)
		}
		foreignAdmin := t.TempDir()
		if err := os.WriteFile(
			filepath.Join(foreignItem.Path, ".git"), []byte("gitdir: "+foreignAdmin+"\n"), 0o600,
		); err != nil {
			t.Fatalf("WriteFile(foreign pointer) error = %v", err)
		}
		if _, err := foreign.service.Remove(
			context.Background(), foreign.workspace.ID, foreignItem.ID, true,
		); !errors.Is(err, ErrRemovalFailed) {
			t.Fatalf("Remove(foreign leftover) error = %v, want ErrRemovalFailed", err)
		}
		if _, err := os.Stat(foreignItem.Path); err != nil {
			t.Fatalf("Stat(foreign leftover) error = %v, want preserved", err)
		}
	})

	t.Run("Should keep a timed-out setup visible as a usable failed setup", func(t *testing.T) {
		settings := config.DefaultWorktreesConfig()
		settings.SetupCommand = "sleep 5"
		settings.SetupTimeout = 50 * time.Millisecond
		fixture.service.Reconfigure(settings, fixture.worktreesRoot)
		item, err := fixture.service.Create(
			context.Background(), fixture.workspace.ID, CreateOptions{Name: "Timeout Setup"},
		)
		if err != nil {
			t.Fatalf("Create(timeout setup) error = %v", err)
		}
		if item.State != StateReady || item.SetupState != SetupFailed || !strings.Contains(item.SetupError, "deadline") {
			t.Fatalf("timeout item = %#v, want ready with readable setup failure", item)
		}
		if refusalResult, err := fixture.service.Remove(
			context.Background(), fixture.workspace.ID, item.ID, false,
		); err != nil || refusalResult != nil {
			t.Fatalf("Remove(timeout setup) = (%#v, %v), want success", refusalResult, err)
		}
	})

	t.Run("Should serialize eight mixed operations and isolate repository contention", func(t *testing.T) {
		concurrent := newRealGitFixture(t)
		first, err := concurrent.service.Create(
			context.Background(), concurrent.workspace.ID, CreateOptions{Name: "Mixed Remove One"},
		)
		if err != nil {
			t.Fatalf("Create(first removal target) error = %v", err)
		}
		second, err := concurrent.service.Create(
			context.Background(), concurrent.workspace.ID, CreateOptions{Name: "Mixed Remove Two"},
		)
		if err != nil {
			t.Fatalf("Create(second removal target) error = %v", err)
		}
		start := make(chan struct{})
		results := make(chan error, 8)
		var group sync.WaitGroup
		operations := []func() error{
			func() error {
				_, createErr := concurrent.service.Create(
					context.Background(), concurrent.workspace.ID, CreateOptions{Name: "Mixed Create A"},
				)
				return createErr
			},
			func() error {
				_, createErr := concurrent.service.Create(
					context.Background(), concurrent.workspace.ID, CreateOptions{Name: "Mixed Create B"},
				)
				return createErr
			},
			func() error {
				_, createErr := concurrent.service.Create(
					context.Background(), concurrent.workspace.ID, CreateOptions{Name: "Mixed Create C"},
				)
				return createErr
			},
			func() error {
				_, removeErr := concurrent.service.Remove(context.Background(), concurrent.workspace.ID, first.ID, false)
				return removeErr
			},
			func() error {
				_, removeErr := concurrent.service.Remove(context.Background(), concurrent.workspace.ID, second.ID, false)
				return removeErr
			},
			func() error {
				_, statusErr := concurrent.service.Status(context.Background(), concurrent.workspace.ID, first.ID, true)
				return statusErr
			},
			func() error {
				_, statusErr := concurrent.service.Status(context.Background(), concurrent.workspace.ID, second.ID, true)
				return statusErr
			},
			func() error {
				_, statusErr := concurrent.service.Status(context.Background(), concurrent.workspace.ID, first.ID, true)
				return statusErr
			},
		}
		for _, operation := range operations {
			operation := operation
			group.Add(1)
			go func() {
				defer group.Done()
				<-start
				results <- operation()
			}()
		}
		close(start)
		group.Wait()
		close(results)
		for operationErr := range results {
			if operationErr != nil {
				if strings.Contains(strings.ToLower(operationErr.Error()), "index.lock") {
					t.Fatalf("mixed operation leaked Git lock failure: %v", operationErr)
				}
				t.Fatalf("mixed operation error = %v", operationErr)
			}
		}

		holder := newRealGitFixture(t)
		entered, release := make(chan struct{}), make(chan struct{})
		var blockOnce sync.Once
		blockingRunner := &interceptingGitRunner{
			inner: holder.runner,
			intercept: func(_ string, args []string) ([]byte, []byte, error, bool) {
				if strings.HasPrefix(strings.Join(args, " "), "branch lock-holder ") {
					blockOnce.Do(func() { close(entered) })
					<-release
				}
				return nil, nil, nil, false
			},
		}
		holderService := holder.newService(blockingRunner)
		holderService.locks = NewRepositoryLocks(0)
		holderDone := make(chan error, 1)
		go func() {
			_, createErr := holderService.Create(
				context.Background(), holder.workspace.ID, CreateOptions{Name: "Lock Holder"},
			)
			holderDone <- createErr
		}()
		<-entered
		if _, err := holderService.Create(
			context.Background(), holder.workspace.ID, CreateOptions{Name: "Overflow"},
		); !errors.Is(err, ErrOperationInProgress) {
			t.Fatalf("Create(queue overflow) error = %v, want ErrOperationInProgress", err)
		}
		otherRepo := newRealGitFixture(t)
		otherDone := make(chan error, 1)
		go func() {
			_, createErr := otherRepo.service.Create(
				context.Background(), otherRepo.workspace.ID, CreateOptions{Name: "Independent Repo"},
			)
			otherDone <- createErr
		}()
		select {
		case otherErr := <-otherDone:
			if otherErr != nil {
				t.Fatalf("Create(second repository) error = %v", otherErr)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("second repository was blocked by unrelated repository contention")
		}
		close(release)
		if err := <-holderDone; err != nil {
			t.Fatalf("Create(lock holder) error = %v", err)
		}
	})

	t.Run("Should make racing removal and same-branch creation deterministic", func(t *testing.T) {
		racing := newRealGitFixture(t)
		item, err := racing.service.Create(
			context.Background(), racing.workspace.ID, CreateOptions{Name: "Racing Removal"},
		)
		if err != nil {
			t.Fatalf("Create(racing removal) error = %v", err)
		}
		entered, release := make(chan struct{}), make(chan struct{})
		var blockOnce sync.Once
		blockingRunner := &interceptingGitRunner{
			inner: racing.runner,
			intercept: func(_ string, args []string) ([]byte, []byte, error, bool) {
				if strings.Contains(strings.Join(args, " "), "worktree remove ") {
					blockOnce.Do(func() { close(entered) })
					<-release
				}
				return nil, nil, nil, false
			},
		}
		service := racing.newService(blockingRunner)
		winner := make(chan error, 1)
		go func() {
			_, removeErr := service.Remove(context.Background(), racing.workspace.ID, item.ID, false)
			winner <- removeErr
		}()
		<-entered
		if _, err := service.Remove(
			context.Background(), racing.workspace.ID, item.ID, false,
		); !errors.Is(err, ErrNotReady) {
			t.Fatalf("Remove(racing loser) error = %v, want ErrNotReady", err)
		}
		if _, err := service.Create(context.Background(), racing.workspace.ID, CreateOptions{
			Name: "Same Branch Race", ExistingBranch: item.Branch,
		}); !errors.Is(err, ErrBranchHeld) {
			t.Fatalf("Create(same branch while removing) error = %v, want ErrBranchHeld", err)
		}
		close(release)
		if err := <-winner; err != nil {
			t.Fatalf("Remove(racing winner) error = %v", err)
		}
		if result, err := service.Remove(
			context.Background(), racing.workspace.ID, item.ID, false,
		); err != nil || result != nil {
			t.Fatalf("Remove(after racing winner) = (%#v, %v), want deterministic removed tombstone", result, err)
		}
		if got := racing.item(item.ID).State; got != StateRemoved {
			t.Fatalf("racing removal state = %q, want removed", got)
		}
	})

	t.Run("Should re-read real Git evidence after an external commit during removal", func(t *testing.T) {
		racing := newRealGitFixture(t)
		item, err := racing.service.Create(
			context.Background(), racing.workspace.ID, CreateOptions{Name: "External Commit Race"},
		)
		if err != nil {
			t.Fatalf("Create(external commit race) error = %v", err)
		}
		service := racing.newService(racing.runner)
		service.hooks = removalHookDispatcher{dispatch: func(request HookRequest) (HookVerdict, error) {
			if request.Event != EventPreRemove {
				return HookVerdict{}, nil
			}
			path := filepath.Join(item.Path, "race.txt")
			if err := os.WriteFile(path, []byte("committed\n"), 0o600); err != nil {
				t.Fatalf("WriteFile(commit race) error = %v", err)
			}
			racing.git(item.Path, "add", "race.txt")
			racing.git(item.Path, "commit", "-m", "external racing commit")
			if err := os.WriteFile(path, []byte("dirty after commit\n"), 0o600); err != nil {
				t.Fatalf("WriteFile(post-commit dirty) error = %v", err)
			}
			return HookVerdict{}, nil
		}}
		refusalResult, err := service.Remove(context.Background(), racing.workspace.ID, item.ID, false)
		if !errors.Is(err, ErrDirtyRequiresForce) || refusalResult == nil || refusalResult.Risk.ChangedFiles == 0 {
			t.Fatalf("Remove(external commit race) = (%#v, %v), want fresh dirty refusal", refusalResult, err)
		}
		if got := racing.item(item.ID).State; got != StateReady {
			t.Fatalf("external commit refusal state = %q, want ready", got)
		}
	})

	t.Run("Should emit every core lifecycle event with canonical correlation", func(t *testing.T) {
		eventFixture := newRealGitFixture(t)
		created, err := eventFixture.service.Create(
			context.Background(), eventFixture.workspace.ID, CreateOptions{Name: "Event Created"},
		)
		if err != nil {
			t.Fatalf("Create(event matrix) error = %v", err)
		}
		if _, err := eventFixture.service.Status(
			context.Background(), eventFixture.workspace.ID, created.ID, true,
		); err != nil {
			t.Fatalf("Status(event matrix) error = %v", err)
		}

		eventFixture.git(eventFixture.workspace.Root, "branch", "event-adopted", "main")
		adoptPath := filepath.Join(t.TempDir(), "event-adopted")
		eventFixture.git(eventFixture.workspace.Root, "worktree", "add", adoptPath, "event-adopted")
		if _, err := eventFixture.service.Adopt(
			context.Background(), eventFixture.workspace.ID, adoptPath,
		); err != nil {
			t.Fatalf("Adopt(event matrix) error = %v", err)
		}

		missing, err := eventFixture.service.Create(
			context.Background(), eventFixture.workspace.ID, CreateOptions{Name: "Event Missing"},
		)
		if err != nil {
			t.Fatalf("Create(missing event) error = %v", err)
		}
		eventFixture.git(eventFixture.workspace.Root, "worktree", "remove", "--force", missing.Path)
		if _, err := eventFixture.service.List(context.Background(), eventFixture.workspace.ID, true); err != nil {
			t.Fatalf("List(missing event) error = %v", err)
		}
		if err := eventFixture.service.Dismiss(
			context.Background(), eventFixture.workspace.ID, missing.ID,
		); err != nil {
			t.Fatalf("Dismiss(event matrix) error = %v", err)
		}

		now := time.Date(2026, 8, 12, 23, 0, 0, 0, time.UTC)
		pending := Worktree{
			ID: "wt-event-cancel", WorkspaceID: eventFixture.workspace.ID, Name: "event-cancel",
			Branch: "event-cancel", Path: filepath.Join(eventFixture.worktreesRoot, "event-cancel"),
			State: StatePending, Origin: OriginManual, SetupState: SetupNone, CreatedAt: now, UpdatedAt: now,
		}
		if err := eventFixture.store.Insert(context.Background(), pending); err != nil {
			t.Fatalf("seed cancel event: %v", err)
		}
		if err := eventFixture.service.CancelCreate(
			context.Background(), eventFixture.workspace.ID, pending.ID,
		); err != nil {
			t.Fatalf("CancelCreate(event matrix) error = %v", err)
		}

		failedSettings := eventFixture.settings
		failedSettings.SetupCommand = "exit 9"
		eventFixture.service.Reconfigure(failedSettings, eventFixture.worktreesRoot)
		if _, err := eventFixture.service.Create(
			context.Background(), eventFixture.workspace.ID, CreateOptions{Name: "Event Setup Failed"},
		); err != nil {
			t.Fatalf("Create(setup-failed event) error = %v", err)
		}
		eventFixture.service.Reconfigure(eventFixture.settings, eventFixture.worktreesRoot)

		runItem, err := eventFixture.service.MaterializeForRun(
			context.Background(), eventFixture.workspace.ID,
			RunWorktreeRequest{TaskSlug: "Event Reclaim", RunID: "run-event-reclaim"},
		)
		if err != nil {
			t.Fatalf("MaterializeForRun(event matrix) error = %v", err)
		}
		if _, err := eventFixture.service.Remove(
			context.Background(), eventFixture.workspace.ID, runItem.ID, false,
		); err != nil {
			t.Fatalf("Remove(event matrix) error = %v", err)
		}

		required := map[string]bool{
			EventCreated: false, EventAdopted: false, EventRemoved: false,
			EventMissing: false, EventDismissed: false, EventCreationCanceled: false,
			EventSetupFailed: false, EventStatusRefreshed: false, EventBranchReclaimed: false,
		}
		eventFixture.events.mu.Lock()
		captured := append([]LifecycleEvent(nil), eventFixture.events.events...)
		eventFixture.events.mu.Unlock()
		for _, event := range captured {
			if _, tracked := required[event.Name]; !tracked {
				continue
			}
			required[event.Name] = true
			if event.WorkspaceID != eventFixture.workspace.ID || event.WorktreeID == "" {
				t.Fatalf("event correlation = %#v, want owning workspace/worktree", event)
			}
			var payload HookWorktree
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				t.Fatalf("decode %s payload: %v", event.Name, err)
			}
			if payload.WorkspaceID != event.WorkspaceID || payload.WorktreeID != event.WorktreeID {
				t.Fatalf("event %s payload correlation = %#v, envelope=%#v", event.Name, payload, event)
			}
		}
		for name, observed := range required {
			if !observed {
				t.Fatalf("core lifecycle event %q was not emitted; captured=%#v", name, captured)
			}
		}
	})
}

type realGitFixture struct {
	t              *testing.T
	runner         *RealGitRunner
	store          *memoryWorktreeStore
	service        *Service
	events         *recordingEventSink
	workspace      Workspace
	worktreesRoot  string
	settings       config.WorktreesConfig
	identifierSeed atomic.Int64
}

type createIntegrationResult struct {
	item *Worktree
	err  error
}

type interceptingGitRunner struct {
	inner     GitRunner
	intercept func(string, []string) ([]byte, []byte, error, bool)
}

func (r *interceptingGitRunner) Run(
	ctx context.Context,
	dir string,
	args ...string,
) ([]byte, []byte, error) {
	if r.intercept != nil {
		stdout, stderr, err, handled := r.intercept(dir, append([]string(nil), args...))
		if handled {
			return stdout, stderr, err
		}
	}
	return r.inner.Run(ctx, dir, args...)
}

func newRealGitFixture(t *testing.T) *realGitFixture {
	return newRealGitFixtureWithCommit(t, true)
}

func newRealGitFixtureWithCommit(t *testing.T, withCommit bool) *realGitFixture {
	t.Helper()
	runner, err := NewRealGitRunner(10 * time.Second)
	if err != nil {
		t.Fatalf("NewRealGitRunner() error = %v", err)
	}
	repo := filepath.Join(t.TempDir(), "repository")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatalf("MkdirAll(repository) error = %v", err)
	}
	fixture := &realGitFixture{
		t: t, runner: runner, store: newMemoryWorktreeStore(), events: &recordingEventSink{},
		workspace:     Workspace{ID: "ws-real", Name: "Real Workspace", Root: repo},
		worktreesRoot: filepath.Join(t.TempDir(), "worktrees"),
	}
	fixture.git(repo, "init", "-b", "main")
	fixture.git(repo, "config", "user.name", "Compozy Integration")
	fixture.git(repo, "config", "user.email", "integration@compozy.test")
	if withCommit {
		if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("initial\n"), 0o600); err != nil {
			t.Fatalf("WriteFile(README.md) error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte(".env\n.setup-marker\n"), 0o600); err != nil {
			t.Fatalf("WriteFile(.gitignore) error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(repo, ".env"), []byte("local=true\n"), 0o600); err != nil {
			t.Fatalf("WriteFile(.env) error = %v", err)
		}
		fixture.git(repo, "add", "README.md", ".gitignore")
		fixture.git(repo, "commit", "-m", "initial")
	}
	settings := config.DefaultWorktreesConfig()
	settings.CopyList = []string{".env"}
	settings.SetupCommand = `printf '%s\n' "$COMPOZY_WORKTREE_ID" > .setup-marker`
	settings.SetupTimeout = 2 * time.Second
	fixture.settings = settings
	fixture.service = fixture.newService(runner)
	return fixture
}

func (f *realGitFixture) newService(runner GitRunner) *Service {
	return NewService(
		f.store,
		runner,
		WithWorkspaceResolver(staticWorkspaceResolver{workspaces: map[string]Workspace{
			f.workspace.ID: f.workspace,
		}}),
		WithConfig(f.settings, f.worktreesRoot),
		WithEvents(f.events),
		WithIDGenerator(func(string) (string, error) {
			return "wt-real-" + time.Unix(f.identifierSeed.Add(1), 0).UTC().Format("150405"), nil
		}),
	)
}

func (f *realGitFixture) git(dir string, args ...string) string {
	f.t.Helper()
	stdout, stderr, err := f.runner.Run(context.Background(), dir, args...)
	if err != nil {
		f.t.Fatalf("git %s error = %v; stderr=%q", strings.Join(args, " "), err, strings.TrimSpace(string(stderr)))
	}
	return strings.TrimSpace(string(stdout))
}

func (f *realGitFixture) item(ref string) Worktree {
	f.t.Helper()
	item, err := f.store.Get(context.Background(), f.workspace.ID, ref)
	if err != nil || item == nil {
		f.t.Fatalf("store.Get(%q) = (%#v, %v)", ref, item, err)
	}
	return *item
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	if got := string(contents); got != want {
		t.Fatalf("ReadFile(%q) = %q, want %q", path, got, want)
	}
}
