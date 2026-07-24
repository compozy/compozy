package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store/globaldb"
)

// Suite: sibling-worktree completion union
// Invariant: the union is a read-only, best-effort superset of the single-workspace
// read that never crosses a repository boundary and never regresses on failure.
// Boundary IN: real temp globaldb, real on-disk git repositories and worktrees.
// Boundary OUT: markdown projection wiring (covered by the projection subsuite).

const unionInitiative = "initiative"

// --- git + fixture helpers -------------------------------------------------

// runCoreGit runs a git subcommand pinned to dir, mirroring the daemon's
// runGitOutput. It fails the test on any non-zero exit.
func runCoreGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmdArgs := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(context.Background(), "git", cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s failed: %v\n%s", args, dir, err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output))
}

// initCoreGitRepo initializes a committed git repository at root, mirroring the
// daemon's initializeHydrationGitRepository so worktrees can be added from it.
func initCoreGitRepo(t *testing.T, root string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("union fixture\n"), 0o600); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runCoreGit(t, root, "init", "--initial-branch=main")
	runCoreGit(t, root, "config", "user.email", "union@example.com")
	runCoreGit(t, root, "config", "user.name", "Union Test")
	runCoreGit(t, root, "add", "README.md")
	runCoreGit(t, root, "commit", "-m", "initial")
}

// addWorktreeSibling creates a detached sibling worktree of primaryRoot outside
// its tree and returns the sibling path. The sibling is intentionally not
// registered here; callers register and seed it as the case requires.
func addWorktreeSibling(t *testing.T, primaryRoot string) string {
	t.Helper()
	siblingPath := filepath.Join(t.TempDir(), "sibling")
	runCoreGit(t, primaryRoot, "worktree", "add", "--detach", siblingPath)
	return siblingPath
}

// seedUnionCompleted seeds exactly the given task groups as completed for one
// workspace under the shared union initiative. Unlike seedCompletionHydrationRows
// it accepts an arbitrary ID set for scale and multi-sibling cases.
func seedUnionCompleted(t *testing.T, db *globaldb.GlobalDB, workspace globaldb.Workspace, ids []string) {
	t.Helper()
	syncedAt := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	children := make([]globaldb.WorkflowSyncInput, 0, len(ids))
	for _, id := range ids {
		children = append(children, globaldb.WorkflowSyncInput{
			WorkspaceID:        workspace.ID,
			WorkflowSlug:       unionInitiative + "/" + id,
			Kind:               globaldb.WorkflowKindTaskGroup,
			TaskGroupID:        id,
			DisplayTitle:       id,
			LifecycleCompleted: true,
			SyncedAt:           syncedAt,
			CheckpointScope:    "workflow",
		})
	}
	if _, err := db.ReconcileAggregateWorkflowSync(
		context.Background(),
		globaldb.AggregateWorkflowSyncInput{
			Parent: globaldb.WorkflowSyncInput{
				WorkspaceID:     workspace.ID,
				WorkflowSlug:    unionInitiative,
				Kind:            globaldb.WorkflowKindInitiative,
				DisplayTitle:    "Initiative",
				SyncedAt:        syncedAt,
				CheckpointScope: "workflow",
			},
			Children: children,
		},
	); err != nil {
		t.Fatalf("seed union completion: %v", err)
	}
}

// newUnionGitFixture builds a committed primary git repo registered in a fresh
// temp DB, writes its _task_groups.md plan with the given states, and seeds the
// primary's completed task groups. It returns the primary root and the DB.
func newUnionGitFixture(
	t *testing.T,
	planStates map[string]string,
	primaryCompleted []string,
) (string, *globaldb.GlobalDB) {
	t.Helper()
	primaryRoot := t.TempDir()
	initCoreGitRepo(t, primaryRoot)
	writeTaskGroupCompletionFixture(
		t,
		filepath.Join(primaryRoot, ".compozy", "tasks", unionInitiative),
		planStates,
	)
	db := openCompletionHydrationDB(t)
	workspace := registerCompletionHydrationWorkspace(t, db, primaryRoot)
	if len(primaryCompleted) > 0 {
		seedUnionCompleted(t, db, workspace, primaryCompleted)
	}
	return primaryRoot, db
}

func bothPending() map[string]string {
	return map[string]string{"TG-001": "pending", "TG-002": "pending"}
}

// registerAndSeedSibling adds a sibling worktree, registers it, and seeds the
// given completed task groups, returning the sibling path.
func registerAndSeedSibling(
	t *testing.T,
	db *globaldb.GlobalDB,
	primaryRoot string,
	completed []string,
) string {
	t.Helper()
	siblingPath := addWorktreeSibling(t, primaryRoot)
	workspace := registerCompletionHydrationWorkspace(t, db, siblingPath)
	if len(completed) > 0 {
		seedUnionCompleted(t, db, workspace, completed)
	}
	return siblingPath
}

// assertUnionIDs asserts got equals want as a set and contains no duplicates.
func assertUnionIDs(t *testing.T, got []string, want ...string) {
	t.Helper()
	counts := make(map[string]int, len(got))
	for _, id := range got {
		counts[id]++
		if counts[id] > 1 {
			t.Fatalf("completed IDs contain duplicate %q: %v", id, got)
		}
	}
	sortedGot := append([]string(nil), got...)
	sortedWant := append([]string(nil), want...)
	sort.Strings(sortedGot)
	sort.Strings(sortedWant)
	if !slices.Equal(sortedGot, sortedWant) {
		t.Fatalf("completed IDs = %v, want %v", got, want)
	}
}

func countWorkspaces(t *testing.T, db *globaldb.GlobalDB) int {
	t.Helper()
	workspaces, err := db.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	return len(workspaces)
}

// --- siblingWorktreeRoots ---------------------------------------------------

func TestSiblingWorktreeRoots(t *testing.T) {
	t.Run("UT-031 one sibling drops the primary cleaned and sorted", func(t *testing.T) {
		primaryRoot := t.TempDir()
		initCoreGitRepo(t, primaryRoot)
		siblingPath := addWorktreeSibling(t, primaryRoot)
		wantSibling, ok := canonicalExistingDir(siblingPath)
		if !ok {
			t.Fatalf("resolve sibling path %q", siblingPath)
		}
		roots, err := siblingWorktreeRoots(context.Background(), primaryRoot)
		if err != nil {
			t.Fatalf("siblingWorktreeRoots() error = %v", err)
		}
		if !slices.Equal(roots, []string{wantSibling}) {
			t.Fatalf("roots = %v, want [%s]", roots, wantSibling)
		}
	})

	t.Run("UT-032 two siblings sorted with primary excluded", func(t *testing.T) {
		primaryRoot := t.TempDir()
		initCoreGitRepo(t, primaryRoot)
		first, _ := canonicalExistingDir(addWorktreeSibling(t, primaryRoot))
		second, _ := canonicalExistingDir(addWorktreeSibling(t, primaryRoot))
		want := []string{first, second}
		sort.Strings(want)
		roots, err := siblingWorktreeRoots(context.Background(), primaryRoot)
		if err != nil {
			t.Fatalf("siblingWorktreeRoots() error = %v", err)
		}
		if !slices.Equal(roots, want) {
			t.Fatalf("roots = %v, want %v", roots, want)
		}
		primary, _ := canonicalExistingDir(primaryRoot)
		if slices.Contains(roots, primary) {
			t.Fatalf("roots %v must exclude the primary %s", roots, primary)
		}
	})

	t.Run("UT-033 bare non-git dir returns a non-nil error", func(t *testing.T) {
		roots, err := siblingWorktreeRoots(context.Background(), t.TempDir())
		if err == nil {
			t.Fatalf("siblingWorktreeRoots() error = nil, want non-nil; roots = %v", roots)
		}
	})
}

// --- completedIDsForWorkspaceRoot ------------------------------------------

func TestCompletedIDsForWorkspaceRoot(t *testing.T) {
	t.Run("UT-034 registered workspace returns id and completed ids", func(t *testing.T) {
		root := t.TempDir()
		db := openCompletionHydrationDB(t)
		workspace := registerCompletionHydrationWorkspace(t, db, root)
		seedUnionCompleted(t, db, workspace, []string{"TG-001"})
		id, ids, err := completedIDsForWorkspaceRoot(context.Background(), db, root, unionInitiative)
		if err != nil {
			t.Fatalf("completedIDsForWorkspaceRoot() error = %v", err)
		}
		if id != workspace.ID {
			t.Fatalf("workspace id = %q, want %q", id, workspace.ID)
		}
		assertUnionIDs(t, ids, "TG-001")
	})

	t.Run("UT-035 unregistered primary resolves via ResolveOrRegister fallback", func(t *testing.T) {
		root := t.TempDir()
		db := openCompletionHydrationDB(t)
		before := countWorkspaces(t, db)
		id, ids, err := completedIDsForWorkspaceRoot(context.Background(), db, root, unionInitiative)
		if err != nil {
			t.Fatalf("completedIDsForWorkspaceRoot() error = %v", err)
		}
		if id == "" {
			t.Fatalf("resolved workspace id is empty")
		}
		if len(ids) != 0 {
			t.Fatalf("ids = %v, want empty", ids)
		}
		if after := countWorkspaces(t, db); after != before+1 {
			t.Fatalf("workspace count = %d, want %d (fallback must register the primary)", after, before+1)
		}
	})

	t.Run("UT-035 primary read failure propagates a wrapped hard error", func(t *testing.T) {
		root := t.TempDir()
		db := openCompletionHydrationDB(t)
		registerCompletionHydrationWorkspace(t, db, root)
		if err := db.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
		_, _, err := completedIDsForWorkspaceRoot(context.Background(), db, root, unionInitiative)
		if err == nil {
			t.Fatalf("completedIDsForWorkspaceRoot() error = nil, want wrapped hard error")
		}
		if !strings.Contains(err.Error(), "completion hydration") {
			t.Fatalf("error = %v, want a wrapped completion-hydration read failure", err)
		}
	})
}

// --- CompletedTaskGroupIDsWithDB union orchestration ------------------------

func TestCompletedTaskGroupIDsWithDBUnion(t *testing.T) {
	t.Run("UT-036 primary and sibling completions union", func(t *testing.T) {
		primaryRoot, db := newUnionGitFixture(t, bothPending(), []string{"TG-001"})
		registerAndSeedSibling(t, db, primaryRoot, []string{"TG-002"})
		got, err := CompletedTaskGroupIDsWithDB(context.Background(), db, primaryRoot, unionInitiative)
		if err != nil {
			t.Fatalf("CompletedTaskGroupIDsWithDB() error = %v", err)
		}
		assertUnionIDs(t, got, "TG-001", "TG-002")
	})

	t.Run("UT-037 unrelated repository never leaks in either order", func(t *testing.T) {
		for _, tc := range []struct {
			name                   string
			registerUnrelatedFirst bool
		}{
			{"unrelated registered after primary", false},
			{"unrelated registered before primary", true},
		} {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				db := openCompletionHydrationDB(t)
				registerUnrelated := func() {
					unrelated := t.TempDir()
					initCoreGitRepo(t, unrelated)
					workspace := registerCompletionHydrationWorkspace(t, db, unrelated)
					seedUnionCompleted(t, db, workspace, []string{"TG-002"})
				}
				if tc.registerUnrelatedFirst {
					registerUnrelated()
				}
				primaryRoot := t.TempDir()
				initCoreGitRepo(t, primaryRoot)
				workspace := registerCompletionHydrationWorkspace(t, db, primaryRoot)
				seedUnionCompleted(t, db, workspace, []string{"TG-001"})
				if !tc.registerUnrelatedFirst {
					registerUnrelated()
				}
				got, err := CompletedTaskGroupIDsWithDB(
					context.Background(), db, primaryRoot, unionInitiative)
				if err != nil {
					t.Fatalf("CompletedTaskGroupIDsWithDB() error = %v", err)
				}
				assertUnionIDs(t, got, "TG-001")
			})
		}
	})

	t.Run("UT-038 unregistered siblings are skipped", func(t *testing.T) {
		t.Run("all siblings unregistered yields single-workspace result", func(t *testing.T) {
			primaryRoot, db := newUnionGitFixture(t, bothPending(), []string{"TG-001"})
			addWorktreeSibling(t, primaryRoot)
			addWorktreeSibling(t, primaryRoot)
			got, err := CompletedTaskGroupIDsWithDB(
				context.Background(), db, primaryRoot, unionInitiative)
			if err != nil {
				t.Fatalf("CompletedTaskGroupIDsWithDB() error = %v", err)
			}
			assertUnionIDs(t, got, "TG-001")
		})
		t.Run("a skipped sibling does not block a registered one", func(t *testing.T) {
			primaryRoot, db := newUnionGitFixture(t, bothPending(), []string{"TG-001"})
			addWorktreeSibling(t, primaryRoot) // unregistered -> skipped
			registerAndSeedSibling(t, db, primaryRoot, []string{"TG-002"})
			got, err := CompletedTaskGroupIDsWithDB(
				context.Background(), db, primaryRoot, unionInitiative)
			if err != nil {
				t.Fatalf("CompletedTaskGroupIDsWithDB() error = %v", err)
			}
			assertUnionIDs(t, got, "TG-001", "TG-002")
		})
	})

	t.Run("UT-039 stale sibling with removed dir is skipped", func(t *testing.T) {
		primaryRoot, db := newUnionGitFixture(t, bothPending(), []string{"TG-001"})
		siblingPath := registerAndSeedSibling(t, db, primaryRoot, []string{"TG-002"})
		if err := os.RemoveAll(siblingPath); err != nil {
			t.Fatalf("remove sibling worktree: %v", err)
		}
		got, err := CompletedTaskGroupIDsWithDB(context.Background(), db, primaryRoot, unionInitiative)
		if err != nil {
			t.Fatalf("CompletedTaskGroupIDsWithDB() error = %v", err)
		}
		assertUnionIDs(t, got, "TG-001")
	})

	t.Run("UT-040 non-git primary swallows enumeration failure", func(t *testing.T) {
		root := t.TempDir()
		db := openCompletionHydrationDB(t)
		workspace := registerCompletionHydrationWorkspace(t, db, root)
		seedUnionCompleted(t, db, workspace, []string{"TG-001"})
		got, err := CompletedTaskGroupIDsWithDB(context.Background(), db, root, unionInitiative)
		if err != nil {
			t.Fatalf("CompletedTaskGroupIDsWithDB() error = %v", err)
		}
		assertUnionIDs(t, got, "TG-001")
	})

	t.Run("UT-041 read never mutates the registry", func(t *testing.T) {
		t.Run("unregistered sibling adds no workspace row", func(t *testing.T) {
			primaryRoot, db := newUnionGitFixture(t, bothPending(), []string{"TG-001"})
			addWorktreeSibling(t, primaryRoot) // present but unregistered
			before := countWorkspaces(t, db)
			if _, err := CompletedTaskGroupIDsWithDB(
				context.Background(), db, primaryRoot, unionInitiative); err != nil {
				t.Fatalf("CompletedTaskGroupIDsWithDB() error = %v", err)
			}
			if after := countWorkspaces(t, db); after != before {
				t.Fatalf("workspace count = %d, want %d (read must not register a sibling)", after, before)
			}
		})
		t.Run("resolved sibling row is unchanged by the read", func(t *testing.T) {
			primaryRoot, db := newUnionGitFixture(t, bothPending(), []string{"TG-001"})
			sibling := registerAndSeedSibling(t, db, primaryRoot, []string{"TG-002"})
			resolved, _ := canonicalExistingDir(sibling)
			before, err := db.Get(context.Background(), resolved)
			if err != nil {
				t.Fatalf("Get(before) error = %v", err)
			}
			if _, err := CompletedTaskGroupIDsWithDB(
				context.Background(), db, primaryRoot, unionInitiative); err != nil {
				t.Fatalf("CompletedTaskGroupIDsWithDB() error = %v", err)
			}
			after, err := db.Get(context.Background(), resolved)
			if err != nil {
				t.Fatalf("Get(after) error = %v", err)
			}
			if before.ID != after.ID || before.RootDir != after.RootDir ||
				!before.UpdatedAt.Equal(after.UpdatedAt) || !before.CreatedAt.Equal(after.CreatedAt) {
				t.Fatalf("sibling row changed by read: before=%+v after=%+v", before, after)
			}
		})
		t.Run("concurrent reads are race-clean", func(t *testing.T) {
			primaryRoot, db := newUnionGitFixture(t, bothPending(), []string{"TG-001"})
			registerAndSeedSibling(t, db, primaryRoot, []string{"TG-002"})
			var workers sync.WaitGroup
			results := make(chan []string, 2)
			errs := make(chan error, 2)
			start := make(chan struct{})
			for range 2 {
				workers.Add(1)
				go func() {
					defer workers.Done()
					<-start
					got, err := CompletedTaskGroupIDsWithDB(
						context.Background(), db, primaryRoot, unionInitiative)
					results <- got
					errs <- err
				}()
			}
			close(start)
			workers.Wait()
			close(results)
			close(errs)
			for err := range errs {
				if err != nil {
					t.Fatalf("concurrent read error = %v", err)
				}
			}
			for got := range results {
				assertUnionIDs(t, got, "TG-001", "TG-002")
			}
		})
	})

	t.Run("UT-042 same task group in primary and sibling appears once", func(t *testing.T) {
		primaryRoot, db := newUnionGitFixture(t, bothPending(), []string{"TG-001"})
		registerAndSeedSibling(t, db, primaryRoot, []string{"TG-001"})
		got, err := CompletedTaskGroupIDsWithDB(context.Background(), db, primaryRoot, unionInitiative)
		if err != nil {
			t.Fatalf("CompletedTaskGroupIDsWithDB() error = %v", err)
		}
		assertUnionIDs(t, got, "TG-001")
	})

	t.Run("UT-043 sibling completion converges on the next read", func(t *testing.T) {
		primaryRoot, db := newUnionGitFixture(t, bothPending(), []string{"TG-001"})
		siblingPath := addWorktreeSibling(t, primaryRoot)
		siblingWorkspace := registerCompletionHydrationWorkspace(t, db, siblingPath)
		first, err := CompletedTaskGroupIDsWithDB(context.Background(), db, primaryRoot, unionInitiative)
		if err != nil {
			t.Fatalf("first read error = %v", err)
		}
		assertUnionIDs(t, first, "TG-001")
		seedUnionCompleted(t, db, siblingWorkspace, []string{"TG-002"})
		second, err := CompletedTaskGroupIDsWithDB(context.Background(), db, primaryRoot, unionInitiative)
		if err != nil {
			t.Fatalf("second read error = %v", err)
		}
		assertUnionIDs(t, second, "TG-001", "TG-002")
	})

	t.Run("UT-044 empty siblings contribute nothing", func(t *testing.T) {
		t.Run("single-worktree repo equals the single-workspace read", func(t *testing.T) {
			primaryRoot, db := newUnionGitFixture(t, bothPending(), []string{"TG-001"})
			got, err := CompletedTaskGroupIDsWithDB(
				context.Background(), db, primaryRoot, unionInitiative)
			if err != nil {
				t.Fatalf("CompletedTaskGroupIDsWithDB() error = %v", err)
			}
			assertUnionIDs(t, got, "TG-001")
		})
		t.Run("sibling with no completions contributes nothing", func(t *testing.T) {
			primaryRoot, db := newUnionGitFixture(t, bothPending(), []string{"TG-001"})
			registerAndSeedSibling(t, db, primaryRoot, nil)
			got, err := CompletedTaskGroupIDsWithDB(
				context.Background(), db, primaryRoot, unionInitiative)
			if err != nil {
				t.Fatalf("CompletedTaskGroupIDsWithDB() error = %v", err)
			}
			assertUnionIDs(t, got, "TG-001")
		})
	})

	t.Run("UT-045 cancellation marks nothing from partial data", func(t *testing.T) {
		t.Run("pre-call cancel returns a wrapped context error", func(t *testing.T) {
			primaryRoot, db := newUnionGitFixture(t, bothPending(), []string{"TG-001"})
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			got, err := CompletedTaskGroupIDsWithDB(ctx, db, primaryRoot, unionInitiative)
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("error = %v, want context.Canceled", err)
			}
			if got != nil {
				t.Fatalf("got = %v, want nil on cancellation", got)
			}
		})
		t.Run("cancellation never leaks a sibling completion", func(t *testing.T) {
			primaryRoot, db := newUnionGitFixture(t, bothPending(), []string{"TG-001"})
			registerAndSeedSibling(t, db, primaryRoot, []string{"TG-002"})
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			got, err := CompletedTaskGroupIDsWithDB(ctx, db, primaryRoot, unionInitiative)
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("error = %v, want context.Canceled", err)
			}
			if slices.Contains(got, "TG-002") {
				t.Fatalf("got = %v, must not include partial sibling data on cancellation", got)
			}
		})
	})
}

// --- HydratePlanCompletionWithDB projection --------------------------------

func TestCompletionUnionProjection(t *testing.T) {
	t.Run("UT-046 projection marks a sibling completion in the primary plan", func(t *testing.T) {
		primaryRoot, db := newUnionGitFixture(t, bothPending(), []string{"TG-001"})
		registerAndSeedSibling(t, db, primaryRoot, []string{"TG-002"})
		marked, err := HydratePlanCompletionWithDB(context.Background(), db, primaryRoot, unionInitiative)
		if err != nil {
			t.Fatalf("HydratePlanCompletionWithDB() error = %v", err)
		}
		if !slices.Contains(marked, "TG-002") {
			t.Fatalf("marked = %v, want to include TG-002", marked)
		}
		assertHydratedPlanState(t, primaryRoot, true, true)
	})

	t.Run("UT-047 re-projection is idempotent and byte-identical", func(t *testing.T) {
		primaryRoot, db := newUnionGitFixture(t, bothPending(), []string{"TG-001"})
		registerAndSeedSibling(t, db, primaryRoot, []string{"TG-002"})
		if _, err := HydratePlanCompletionWithDB(
			context.Background(), db, primaryRoot, unionInitiative); err != nil {
			t.Fatalf("first projection error = %v", err)
		}
		planPath := completionHydrationPlanPath(primaryRoot)
		afterFirst := mustReadFile(t, planPath)
		for _, pass := range []string{"second", "third"} {
			marked, err := HydratePlanCompletionWithDB(
				context.Background(), db, primaryRoot, unionInitiative)
			if err != nil {
				t.Fatalf("%s projection error = %v", pass, err)
			}
			if len(marked) != 0 {
				t.Fatalf("%s projection newly marked = %v, want 0", pass, marked)
			}
			if got := mustReadFile(t, planPath); got != afterFirst {
				t.Fatalf("%s projection changed the plan file", pass)
			}
		}
	})

	t.Run("UT-048 an existing mark is never reverted", func(t *testing.T) {
		// Plan pre-marks TG-001 [x]; the DB source reports TG-001 uncompleted and
		// only TG-002 completed. The additive projection must keep TG-001 marked.
		primaryRoot, db := newUnionGitFixture(
			t,
			map[string]string{"TG-001": "completed", "TG-002": "pending"},
			[]string{"TG-002"},
		)
		assertHydratedPlanState(t, primaryRoot, true, false)
		marked, err := HydratePlanCompletionWithDB(context.Background(), db, primaryRoot, unionInitiative)
		if err != nil {
			t.Fatalf("HydratePlanCompletionWithDB() error = %v", err)
		}
		if !slices.Contains(marked, "TG-002") {
			t.Fatalf("marked = %v, want to include TG-002", marked)
		}
		if slices.Contains(marked, "TG-001") {
			t.Fatalf("marked = %v, TG-001 must not be re-marked", marked)
		}
		assertHydratedPlanState(t, primaryRoot, true, true)
	})
}

// --- scale ------------------------------------------------------------------

func TestCompletedTaskGroupIDsWithDBScale(t *testing.T) {
	// IT-038: exactly 100 completed task groups distributed across sibling
	// worktrees of one repository are all returned by the union. Fixed fixture.
	const siblingCount = 4
	const perSibling = 25
	primaryRoot := t.TempDir()
	initCoreGitRepo(t, primaryRoot)
	db := openCompletionHydrationDB(t)
	registerCompletionHydrationWorkspace(t, db, primaryRoot)
	want := make([]string, 0, siblingCount*perSibling)
	next := 1
	for s := 0; s < siblingCount; s++ {
		ids := make([]string, 0, perSibling)
		for i := 0; i < perSibling; i++ {
			id := fmt.Sprintf("TG-%03d", next)
			ids = append(ids, id)
			want = append(want, id)
			next++
		}
		registerAndSeedSibling(t, db, primaryRoot, ids)
	}
	got, err := CompletedTaskGroupIDsWithDB(context.Background(), db, primaryRoot, unionInitiative)
	if err != nil {
		t.Fatalf("CompletedTaskGroupIDsWithDB() error = %v", err)
	}
	if len(got) != siblingCount*perSibling {
		t.Fatalf("union size = %d, want %d", len(got), siblingCount*perSibling)
	}
	assertUnionIDs(t, got, want...)
}
