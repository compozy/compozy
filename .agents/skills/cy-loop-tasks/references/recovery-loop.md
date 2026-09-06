# Self-healing recovery loop

Use this procedure for failed required checks, missing required evidence, QA
bugs, or broken delegation. Load it once when needed and reuse it. Expected
search misses and worker wait expiry with a live worker are not failures.

A failure is **repairable by default**. It remains inside the current phase
action and does not create an iteration entry. `outcome=blocked` is reserved
for an external blocker proven by the test below.

## Repair loop

1. Capture the exact failing command or check, exit status, decisive output,
   and files it identifies. Keep the task/slice status and `state.yaml`
   unchanged.
2. Diagnose the root cause. Read the owning project instructions and the
   failing tool's documented remediation. When output supplies a safe,
   in-scope repair command, execute it instead of merely reporting it.
3. Apply the root-cause repair. Canonical generators, formatters, dependency
   bootstraps, and their deterministic outputs are in scope for gate closure,
   even when those outputs are outside the task's primary paths. Inspect every
   resulting diff and preserve unrelated user changes without staging them
   into this workflow's checkpoint.
4. Rerun the narrowest command that reproduces the failure. A blind rerun is
   not a repair: flaky tests, timeouts, races, and intermittent workers require
   a diagnosis or a changed precondition before retrying.
5. Once the focused failure is green, rerun invalidated scoped lanes and any
   affected QA journeys/visual rows. Assess that evidence under
   `cy-final-verify` without repeating valid checks, grounding, or the diff
   review. If another failure appears, return to step 1 within the action.
6. Update memory with the failure, root cause, repair, and final evidence.
   Only then write the phase's single final state update and summary.

Done when every failure observed in the phase action is repaired, the required
artifacts exist, and the phase's completion gate is PASS.

## Normative classifications

| Failure | Required autonomous action |
| --- | --- |
| CodegenCheck reports a stale generated file and names a generator | Run the canonical generator, inspect generated diffs, and recheck the invalidated evidence. A stale Daytona sidecar is this case, not a blocker. |
| Formatter, lint, typecheck, build, or test failure | Diagnose and fix the owning source or contract, then run the affected lane. |
| Test timeout, race, or intermittent failure | Reproduce under bounded conditions, find the production/test-infrastructure cause, fix it without weakening the test or inflating timeouts, then rerun the gates. |
| Missing local tool, generated dependency, or bootstrap state | Use the repository's canonical install/bootstrap command when it is safe and deterministic, then resume the action. |
| QA finds an in-scope completion or data-loss bug | Fix the product, rerun the affected journey, and update the existing QA report; close the action only when ready. |
| Worker launch, evidence, or artifact failure | Recover missing evidence through the existing worker first; relaunch only a confirmed broken worker. Keep the phase action open. |
| Checkpoint hook or commit failure | Fix the hook/root cause, rerun verification when source changed, and retry the normal checkpoint without bypass flags. |

## External-blocker test

Stop only when all of these are true:

1. The phase cannot reach its completion criterion without a specific missing
   credential, authorization, destructive-operation approval, external
   service, or unavailable infrastructure. A product decision parks instead
   (default it, log it under `## Open Questions`, continue); it qualifies
   here only when no safe default exists and every remaining task depends
   on it.
2. Every safe in-scope alternative has been attempted and recorded with
   evidence. Complexity, a dirty worktree, a failing gate, repeated repair,
   generated drift, or elapsed time does not satisfy this condition.
3. The missing input cannot be derived from repository truth and the agent
   cannot create, repair, restart, regenerate, or replace it within the
   authority already granted by the goal.

When the test passes, update memory with the missing external input and the
exhausted alternatives. Then record one final
`--verify-fail --blocker <text> --outcome blocked` state update and print the
blocked summary. Otherwise, return to the repair loop.

## Helper and delegation failures

- **Mode disagreement** — `init-state.py` exits 4 when `--mode` contradicts
  the filesystem. Reconcile by adding/removing `_tasks.md` before bootstrap,
  or run `update-state.py <slug> --reconcile-tasks` when the task graph was
  authored after a free-mode bootstrap.
- **Stack strategy changed** — after the user explicitly abandons the remote
  stack, run `update-state.py <slug> --disable-stacked` before the next
  checkpoint; keep `state.yaml` writer-owned.
- **Frontend strategy changed** — after the user explicitly abandons worker
  delegation, run `update-state.py <slug> --disable-frontend`; remaining
  frontend work executes locally.
- **`state.yaml` parse failure** — `detect-phase.py` exits 1 with the parse
  error on stderr. Diagnose the malformed writer or interrupted write from
  evidence and repair it without discarding unrelated worktree changes.
- **`commit-checkpoint.py` exit 1** — repair the hook or commit failure and
  retry normally. If the repair changes tracked source after the last PASS,
  refresh the invalidated evidence under `cy-final-verify` before retrying.
  `SKIP: no changes` is success.
- **`commit-checkpoint.py` exit 2 in stacked mode** — resume drift: the
  worktree is outside the stack while layer branches exist. Run
  `gh stack checkout <slug>/task-NN` then `gh stack top` and retry
  (`references/stacked-prs.md`); never re-init the stack.
- **Worker launch or delegation failure** — inspect the native status and
  screen per `herdr-orchestration`. A normal wait timeout is an observation
  interval, not a worker failure. Repair/relaunch only a confirmed broken
  launch or detection failure; retain a live worker's context for follow-ups.
- **Delegated run lacks PASS evidence or artifacts, or committed anyway** —
  keep the phase open, recover the missing evidence or rerun the lane, and do
  not advance. A worker commit is a contract breach that requires preserving
  the worker's work and repairing checkpoint ownership before continuing.
- **Invalid peer-review round** (missing or malformed review artifacts, or no
  verdict) — the round does not count; follow `deep-review` error handling
  and re-run it.
