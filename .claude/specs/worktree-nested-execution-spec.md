# Spec: Parallel execution when Compozy is invoked from inside a git worktree

- Status: investigation + fix spec (plan-first)
- Branch: `worktree-fix` (already checked out, clean, base = main + docs commit `ca25626`)
- Related commits: `cc626ba5f247` (fix: parallel execution, #217), `6fa1d4b0bf03` (feat: worktree-backed parallel multi-run for `tasks run --multiple`, #200)
- Owning packages: `internal/daemon`, `internal/core/worktree`, `internal/core/run/parallel`, `internal/core/run/executor`, `internal/cli`

## 1. Problem statement

Commits `6fa1d4b` and `cc626ba` introduced worktree-backed parallel execution in two shapes:

1. **Multi-workflow parallel** (`compozy tasks run --multiple` with mode `parallel`): each workflow slug gets its own detached git worktree.
2. **Same-workflow parallel tasks** (`task.parallel.*` waves): each task file gets its own detached worktree, results are squash-merged into a dedicated integration worktree/branch, then fast-forwarded back onto the caller's branch.

Both shapes assume the workspace root (the directory where the user invoked compozy, registered as `workspace.RootDir`) is a **primary checkout** — a repo whose `.git` is a directory. The case where compozy is invoked **from inside a linked git worktree** (`.git` is a file containing `gitdir: <main>/.git/worktrees/<name>`) was never designed for, and — verified by grep — **no test anywhere builds a linked-worktree fixture**. All git fixtures do plain `git init` in `t.TempDir()` (e.g. `internal/cli/tasks_run_parallel_e2e_test.go:568-573`).

This matters because linked worktrees are exactly how power users run multiple agent sessions on one repo — and how compozy itself isolates child runs. Compozy running inside a worktree is the norm, not the exception:

- a user `cd`s into `~/dev/proj/_worktrees/feature-x` and runs `compozy tasks run`;
- an agent spawned by compozy inside a compozy-created child worktree invokes `compozy` again (bundled cy-\* skills call the CLI);
- two compozy sessions run concurrently in the main checkout and in a linked worktree of the same repo, sharing `$GIT_COMMON_DIR`.

## 2. Verified architecture (how it works today)

All references verified on branch `worktree-fix` at `ca25626`.

### 2.1 Worktree allocation (daemon)

`internal/daemon/task_multi_worktree.go`

- `ResolveBase()` (:130) — resolves parent branch + HEAD via `git -C <workspaceRoot> rev-parse --abbrev-ref HEAD`. **Rejects detached HEAD** (:146) with "a named branch is required for parallel multi-run".
- `Allocate()` (:167) — plans a deterministic path and runs `git -C <workspaceRoot> worktree add --detach <path> <commit>` (:188).
- `planTaskMultiWorktreePath()` (:744) — path scheme:
  `<worktreesRoot>/<sha256(workspaceRoot)[:12]>/<runid[:12]>-<sha256(runid)[:8]>/<NN-slug>`
  where `worktreesRoot = ~/.compozy/state/worktrees` (`internal/config/home.go:113`, `COMPOZY_HOME` override supported).
- `CreateIntegrationBranch()` (:326) — `git worktree add -b compozy/parallel-<runid> <...>/integration <baseRef>` (:359; branch name from `internal/daemon/task_multi.go:451`).
- `SquashMerge()` (:367), `FastForward()` (:427 — requires the workspace on the target branch and clean, then `merge --ff-only`), `DiscardIntegrationBranch()` (:475 — `worktree remove --force` + `branch -D`), `Remove()` (:637 — remove without force), `Prune()` (:714 — `git worktree prune`).
- `removeIntegrationWorktreeForPurge()` (:590) — runs **`git worktree prune --expire now`** (:608) in the user's repo after removing the integration worktree.
- `runTaskMultiWorktreeGitCommand()` (:987) — plain `exec.CommandContext(git, -C dir, ...)`. **Does NOT sanitize the environment** — inherits `GIT_DIR`, `GIT_WORK_TREE`, `GIT_INDEX_FILE`, `GIT_COMMON_DIR` from the daemon process if set.

### 2.2 Child scheduling (daemon)

`internal/daemon/task_multi.go`

- `resolveTaskMultiParallelBase()` (:1388) — base resolved once per parent run at `prepared.workspace.RootDir`.
- `startTaskMultiWorktreeChild()` (:1545) — Allocate → `mirrorTaskMultiWorkflowArtifacts()` (copies `.compozy/tasks/<slug>` into the worktree; `internal/daemon/task_multi_artifacts.go`) → emits `task.multi.*` event **before** child start (crash-safe metadata) → registers the worktree path as its own workspace row (`resolveWorkflowContext`, :1577) → `remapTaskMultiChildRuntime()` (:1719) repoints `RuntimeConfig.WorkspaceRoot` at the worktree (:1740).
- `startTaskWorktreeChild()` (:1620) — same per single task number (same-workflow parallel).
- `planParallelIntegrationPath()` (:459) — `<worktreesRoot>/<hash>/<parent>/integration`.
- Agent subprocesses inherit cwd from the runtime workspace root: `internal/core/subprocess/process.go:58` (`cmd.Dir = cfg.WorkingDir`).

### 2.3 Orchestration (same-workflow waves)

`internal/core/run/parallel/orchestrator.go` — `Run()` (:317): create integration branch (:350) → per wave, launch task worktree children → `CommitTask` (scoped to produced paths from the worktree scope artifact) → `SquashMerge` into integration (:656) → conflict resolution via resolver → `FastForward` workspace branch + `SyncTaskArtifacts` (:491-494). Scope capture: `internal/core/run/executor/review_hooks.go:115` → `worktree.BuildScope(ctx, cfg.WorkspaceRoot, preSnapshot)`.

### 2.4 Snapshot/scope

`internal/core/worktree/snapshot.go`

- `Capture()` (:145) — `os.Stat(root/.git)` (:151) works for both dir and file, so linked worktrees pass this check.
- `runGit()` (:682) — **does sanitize** env (strips `GIT_DIR`, `GIT_WORK_TREE`, `GIT_COMMON_DIR`, `GIT_INDEX_FILE`, `GIT_NAMESPACE`, :695-710). Note the asymmetry with §2.1's daemon git runner.

### 2.5 Purge / reconcile / shutdown

`internal/daemon/worktree_purge.go`

- Purge plan built from the run's own journal events (`task.multi.*` / `task.parallel.*` payloads carry `WorktreePath`, `BaseCommit`), :210-263.
- `cleanOwnedWorktreePath()` (:389) — only paths under `worktreesRoot` are ever removed (symlink-aware). The caller's own worktree (outside `~/.compozy/state/worktrees`) cannot be selected.
- `inspectTaskWorktreeForPurge()` (:311) — refuses removal when status is dirty or commits aren't retained by any branch (`git branch --contains` runs at `workspaceRoot`, :361).
- Startup/shutdown paths: `internal/daemon/reconcile.go:69`, `internal/daemon/shutdown.go:300`.

### 2.6 Workspace identity

`internal/store/globaldb/registry.go` — workspaces keyed by cleaned `RootDir` (:164, :817). CLI resolves the root from `os.Getwd()` (`internal/cli/setup.go:1353`); nothing calls `git rev-parse --show-toplevel` in production code (grep-verified), so a linked worktree registers as its own independent workspace. Compozy's own child worktrees are also registered as workspaces (:1577 above), so "workspace root = linked worktree" already happens daily for children — but only for **detached, compozy-owned** worktrees driven by the daemon itself, never for a **user-initiated top-level run**.

### 2.7 Live evidence

The machine running this spec has stale compozy worktrees at
`~/.compozy/state/worktrees/4b2e29c14cde/tasks-task-m-{1d6f0ca1,7d2f9620,9736262d}/01-multi-task-run` (all detached at `e43aa56`) — created by real runs of this feature, visible via `git worktree list` from the main checkout. A user worktree also exists at `~/Dev/compozy/_worktrees/pr-200-review` (detached).

## 3. Scenario matrix (required behavior)

| #   | Scenario                                                                                                                                       | Required behavior                                                                                                                                                                                                                                                                                                                         |
| --- | ---------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| S1  | `compozy tasks run` with parallel tasks (same workflow), CWD = **linked worktree on a named branch**                                           | Fully supported. Worktrees allocated from the linked worktree's HEAD; merge-back fast-forwards the **worktree's** branch; artifacts land in the worktree. No interference with the main checkout or sibling worktrees.                                                                                                                    |
| S2  | `compozy tasks run --multiple` mode `parallel`, CWD = linked worktree on a named branch                                                        | Fully supported, same guarantees as S1.                                                                                                                                                                                                                                                                                                   |
| S3  | Parallel/multi run, CWD = worktree with **detached HEAD** (incl. compozy-created child worktrees)                                              | Rejected **early** (CLI/wizard preflight, before any run/journal rows are created) with an actionable message naming the fix (checkout a branch). Today the rejection happens deep in the daemon (`ResolveBase`) after the parent run started — verify where the failure surfaces in the wizard/TUI and make it a first-class validation. |
| S4  | **Recursion**: compozy invoked inside a compozy-owned worktree (path under `<home>/state/worktrees/`) starts another parallel/multi run        | Explicitly detected and rejected with an actionable error (detached-HEAD rejection currently masks this by accident; make the guard intentional and the message truthful). Non-parallel commands (exec, runs, status) must keep working there.                                                                                            |
| S5  | Two compozy runs concurrent on the **same repo family** (main checkout + linked worktree, or two worktrees), both allocating/purging worktrees | No cross-run interference: run A's purge/prune must never delete or prune run B's worktrees or admin metadata (`$GIT_COMMON_DIR/worktrees/*`). Investigate the `git worktree prune --expire now` (:608) and `Prune()` (:714) blast radius — prune is repo-wide by design.                                                                 |
| S6  | Purge/reconcile when the registered workspace root (a linked worktree) **no longer exists** (user deleted it)                                  | Purge must degrade gracefully (skip with a logged reason), not error out the whole reconcile pass, and must not touch other checkouts.                                                                                                                                                                                                    |

Product decisions encoded above: linked worktrees on a named branch are first-class (S1/S2); detached HEAD and recursion are rejected with intent (S3/S4); multi-checkout concurrency is safe (S5); missing roots degrade gracefully (S6).

## 4. Suspected failure points (hypotheses — root-cause each, do not assume)

These are leads, ranked. Each must be confirmed or refuted with a reproduction (test or command trace) before any fix. Symptom-patching without a confirmed mechanism is rejected.

1. **Env leakage in the daemon git runner** — `runTaskMultiWorktreeGitCommand` (`internal/daemon/task_multi_worktree.go:987`) inherits `GIT_DIR`/`GIT_WORK_TREE`/`GIT_INDEX_FILE`/`GIT_COMMON_DIR` while `internal/core/worktree/snapshot.go:695` deliberately strips them. If the daemon (or `compozy --detach` re-exec) is started from a shell/hook/tool that exports any of these — common in worktree-heavy setups and inside git hooks — every allocator/merge/purge git call resolves against the wrong repo. The asymmetry between the two runners is itself a smell; decide one policy and apply it to all git invocations (also check `internal/daemon/review_watch_git.go`).
2. **Repo-wide `git worktree prune --expire now`** (`task_multi_worktree.go:608`) and `Prune()` (:714) — prune operates on the whole repo family. Race: run A prunes while run B's `git worktree add` is mid-flight (admin dir exists, `gitdir` file not yet final), or a user's worktree sits on a temporarily-missing path (unmounted volume). Determine git's actual guarantees (worktree locks?) and scope compozy's prune to what it owns — or lock compozy-owned worktrees (`git worktree lock`) during runs.
3. **Purge of a worktree that hosts a live nested run** — parent purge (`worktree_purge.go:108-139`) removes its child worktree once it is clean and its commits are branch-retained; a nested compozy process (agent-invoked) may still be running inside that path. Verify lifecycle ordering guarantees; consider `git worktree lock` or run-liveness checks before `Remove`.
4. **Detached-HEAD failure surfaces too late and with git jargon** (S3) — `ResolveBase` (:130) fails only after the parent run exists (journal rows, TUI states, events already emitted). Trace the failure through `internal/cli/tasks_run_wizard.go` / `internal/core/run/ui/multi_remote.go` and check what the user actually sees; add preflight validation CLI-side.
5. **`FastForward` semantics from a linked worktree** (:427-470) — requires the caller checkout on `BaseBranch` and clean. From a linked worktree: confirm `merge --ff-only` updates only the worktree's checked-out branch, that the branch is not checked out anywhere else (it can't be, git forbids it), and that failure modes (branch moved mid-run by another checkout) produce recoverable states — the integration branch must survive for manual recombination as documented.
6. **`git branch --contains` / branch listing scope in purge** (`worktree_purge.go:361`) — runs at `workspaceRoot`; branch namespace is repo-wide so this is probably correct, but verify preserved-commit detection still holds when base commit info is missing and the retaining branch lives in a different checkout.
7. **`mirrorTaskMultiWorkflowArtifacts`** (`internal/daemon/task_multi_artifacts.go`) — verify path assumptions when the source workspace is a linked worktree (artifacts under `.compozy/` are typically gitignored and live only in the checkout that ran the workflow — confirm mirror source is the caller's live filesystem, not the base commit).
8. **Workspace registry double-identity** (§2.6) — main checkout and worktree register as unrelated workspaces of the same repo. Audit daemon features keyed by workspace (review watch, recovery, reconcile, purge) for hidden one-workspace-per-repo assumptions.
9. **Snapshot/scope inside child worktrees when the child is itself nested** — `Capture` handles `.git`-as-file, but confirm `BuildScope` fingerprints behave with gitlinks/submodules and with `.git` file paths in deeper nesting (worktree-of-worktree after S4 guard should be unreachable — assert that).

## 5. Success criteria

1. **Repro first**: each S1-S6 scenario has a deterministic reproduction (failing test or documented command trace) executed **before** the fix; findings recorded in the plan/PR description. Hypotheses in §4 confirmed or explicitly refuted with evidence.
2. S1 and S2 pass end-to-end from a linked-worktree CWD: run completes, merge-back lands on the worktree's branch, `git -C <main> worktree list` shows no leaked compozy worktrees after purge, sibling checkouts untouched (verify via `git status` + `worktree list` before/after in the test).
3. S3: detached-HEAD rejection happens before parent-run creation, with a message that names the directory, states "detached HEAD", and says what to do. Covered by a CLI-level test.
4. S4: recursion guard rejects parallel/multi runs whose workspace root is inside the home worktrees root, with an intentional error message. Non-parallel commands inside such worktrees keep working (test both).
5. S5: concurrent allocate/purge across two checkouts of one repo family proven safe (integration test with two workspaces; no lost admin metadata, no failed adds caused by the sibling's prune).
6. S6: purge/reconcile with a deleted workspace root logs and skips; other runs' purges still execute.
7. Git-env policy unified: every production git invocation uses one documented environment policy (sanitized or intentionally inherited — decided at root cause, not per-call-site patched).
8. No destructive behavior toward worktrees compozy does not own; `cleanOwnedWorktreePath` guarantees preserved and extended to any new removal path.
9. `make verify` passes 100% (fmt + lint zero issues + tests with `-race` + build). No lint suppressions, no skipped tests, no `interface{}` shortcuts.
10. Existing behavior for primary checkouts unchanged (full existing suites pass unmodified — any assertion change requires a stated reason rooted in a confirmed bug, per testing rules).

## 6. User flows

### Flow A (S1) — parallel tasks from a worktree

1. User: `git -C ~/proj worktree add ../proj-wt feature-x && cd ../proj-wt`.
2. `compozy tasks run`, picks a workflow with `_tasks.md` (schema `compozy.tasks/v2`), enables parallel tasks in the wizard (`internal/cli/tasks_run_wizard.go` fields `parallelTasks`, resolver IDE/model/reasoning).
3. Daemon resolves base = `feature-x`@HEAD of the worktree; allocates per-task worktrees under `~/.compozy/state/worktrees/<hash(proj-wt)>/...`; agents run with cwd = task worktree.
4. Waves complete → squash merges into `compozy/parallel-<runid>` → fast-forward `feature-x` in `~/proj-wt` → artifacts synced → worktrees purged → integration branch deleted.
5. User sees completed run; `git log` in the worktree shows the squash commits; main checkout `~/proj` untouched.

### Flow B (S2) — multi-workflow parallel from a worktree

Same entry, `--multiple` + mode `parallel` + limit; per-slug worktrees; each slug's results preserved per ADR-005..008 (`.compozy/tasks/multi-task-run/adrs/`); worktrees preserved on non-finalized runs (status `preserved`).

### Flow C (S3/S4) — rejection paths

User in a detached worktree (or inside `~/.compozy/state/worktrees/...`) starts a parallel run → immediate CLI error naming the condition and remedy; exit non-zero; no run rows, no events, no worktrees created.

## 7. Test plan

Follow repo testing rules: table-driven, `t.Run`, `t.Parallel` where independent, `t.TempDir`, `-race`. Reuse canonical suites — extend the existing files; do not create parallel standalone suites. The goal of these tests is to catch the real bugs in §4, not to freeze current behavior.

### 7.1 New shared fixture

Add a linked-worktree fixture helper next to the existing git fixture in `internal/cli/tasks_run_parallel_e2e_test.go:568` (and a daemon-level equivalent where needed): init primary repo → commit seed → `git worktree add <dir> -b <branch>` → return both roots. This is the missing primitive every nested test needs.

### 7.2 Unit (owning layer: allocator/purge/scheduling)

Extend `internal/daemon/task_multi_worktree_test.go`:

- `ResolveBase` from a linked worktree root (named branch) → correct branch/commit; from detached worktree → the named error.
- `Allocate` with workspaceRoot = linked worktree (real git, in the style of `TestRunTaskMultiWorktreeGitCommand` :1573) → worktree created, registered in the **common** git dir, path scheme keyed by the worktree root hash.
- Git runner env policy: with `GIT_DIR`/`GIT_WORK_TREE` exported in the test process env, allocator commands still target the `-C` directory (this is the regression test for §4.1).
- `FastForward` executed against a linked worktree root.

Extend `internal/daemon/worktree_purge` coverage in `purge_test.go`:

- purge plan/removal when `workspaceRoot` is a linked worktree;
- S6: workspace root missing → skip + no error propagation;
- guard: recorded worktree path equal to the caller's own root is never removed.

Extend `internal/core/worktree/snapshot_test.go`:

- `Capture`/`BuildScope` where root's `.git` is a file (linked worktree fixture).

New guard tests (wherever the S4 guard lands, likely `internal/daemon/task_multi.go` prepare path + CLI preflight):

- workspace root under home worktrees root → parallel/multi rejected; enqueued/exec allowed.

### 7.3 Integration / e2e (owning layer: CLI e2e)

Extend `internal/cli/tasks_run_parallel_e2e_test.go`:

- S1 e2e: full parallel run with CWD = linked worktree; assert merge-back on the worktree branch, purge cleanliness (`git worktree list` before/after), main checkout untouched.
- S2 e2e: `--multiple` parallel from linked worktree.
- S3 e2e: detached worktree → early validation error, no run created.
- S5: two concurrent runs (main + worktree) — at minimum a daemon-level test driving both allocators concurrently against one repo family; assert both complete and both purges are clean.

### 7.4 Non-goals for tests

No snapshot tests of error prose beyond the key substrings; no tests freezing path-scheme internals except where the path contract is load-bearing (purge ownership).

## 8. Data & API surfaces (do not break)

- **Events** (`pkg/compozy/events/kinds/task.go`): `TaskRunMultiplePayload` / `TaskParallelPayload` carry `WorktreePath`, `BaseBranch`, `BaseCommit`, `WorktreeStatus` — purge reconstructs its plan from these; any new metadata must be additive (docs in `docs/events.md`, contract tests in `pkg/compozy/events/kinds/docs_test.go`, `payload_compat_test.go`).
- **HTTP API** (`internal/api/contract/types.go`, `openapi/compozy-daemon.json`, `internal/api/client/client_contract_test.go`): `TaskRunRequest` mode/limit fields; `TaskRunMultipleItem` worktree metadata. Additive changes only; regenerate contract fixtures if touched.
- **Config** (`internal/core/workspace/config_types.go`): `[tasks.run]` parallel limit + `ParallelTasksConfig` (max concurrency, resolver IDE/model/reasoning). There is currently **no** worktree-location or worktree-behavior config key (grep-verified) — if the fix needs one, follow existing validation patterns in `config_validate.go` + tests in `config_test.go`.
- **Run artifacts**: journal DB per run (`internal/core/run/journal`), worktree scope JSON at `RunArtifacts.JobArtifacts(<job>).WorktreeScopePath` (`internal/core/run/executor/review_hooks.go:135`), home layout `~/.compozy/state/worktrees/...` (`internal/config/home.go`).

## 9. File reference index

| Area                                  | Files                                                                                                                |
| ------------------------------------- | -------------------------------------------------------------------------------------------------------------------- |
| Allocator + git runner                | `internal/daemon/task_multi_worktree.go` (+`_test.go`)                                                               |
| Scheduling / remap / integration path | `internal/daemon/task_multi.go`, `internal/daemon/task_multi_artifacts.go` (+tests)                                  |
| Purge / ownership guards              | `internal/daemon/worktree_purge.go`, `internal/daemon/purge_test.go`                                                 |
| Reconcile / shutdown                  | `internal/daemon/reconcile.go`, `internal/daemon/shutdown.go`, `internal/daemon/run_manager.go`                      |
| Wave orchestration                    | `internal/core/run/parallel/{orchestrator,resolver,waves,fsm,events}.go` (+tests)                                    |
| Snapshot / scope                      | `internal/core/worktree/snapshot.go` (+`_test.go`)                                                                   |
| Executor hooks                        | `internal/core/run/executor/review_hooks.go`, `internal/core/run/executor/runner.go`                                 |
| Subprocess cwd/env                    | `internal/core/subprocess/process.go`, `internal/core/run/internal/acpshared/session_exec.go`                        |
| CLI wizard / e2e                      | `internal/cli/tasks_run_wizard.go`, `internal/cli/tasks_run_parallel_e2e_test.go`, `internal/cli/daemon_commands.go` |
| Workspace identity                    | `internal/store/globaldb/registry.go`, `internal/cli/setup.go:1353`                                                  |
| Home paths                            | `internal/config/home.go`                                                                                            |
| Events / API                          | `pkg/compozy/events/kinds/task.go`, `internal/api/contract/types.go`, `docs/events.md`                               |
| Design history                        | `.compozy/tasks/multi-task-run/_techspec.md`, `adrs/adr-005..008.md`                                                 |

## 10. Constraints

- Root-cause fixes only — no symptom patches, no lint suppressions, no error swallowing, no timing hacks (`no-workarounds` gates apply).
- Debugging must follow the systematic protocol: reproduce → instrument/trace → localize → confirm mechanism → then fix (`systematic-debugging`).
- Never run `git restore/checkout/reset/clean/rm` against this working tree without explicit permission; test fixtures own their temp repos and may do anything inside `t.TempDir()`.
- `go get` for any new dependency (none expected).
- Completion gate: `make verify` at 100%, full output read. Zero lint issues, `-race` clean.
- Commits: only if explicitly requested afterwards; the deliverable is the working tree on `worktree-fix` + summary of findings.

## 11. Out of scope

- Windows-specific worktree path semantics.
- Submodule-heavy repos beyond keeping existing gitlink fingerprints working.
- Redesigning the merge-back strategy (squash + ff-only stays as per ADR-005..008).
- UI/TUI redesign beyond surfacing the new validation errors correctly.
