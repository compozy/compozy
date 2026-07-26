# Self-healing recovery loop

Read this file in full whenever a phase command exits non-zero, a gate reports
FAIL, a required artifact/evidence item is absent, QA reports a completion
bug, or a delegated worker does not satisfy its completion contract.

A failure is **repairable by default**. It remains inside the current phase
action and does not create an iteration entry. `outcome=blocked` is reserved
for an external blocker proven by the test below.

## Repair loop

1. Capture the exact failing command or check, exit status, decisive output,
   and files it identifies. Keep `progress.criteria[]` and the rest of
   `state.yaml` unchanged.
2. Diagnose the root cause. Read the owning project instructions and the
   failing tool's documented remediation. When output supplies a safe,
   in-scope repair command, execute it instead of merely reporting it.
3. Apply the root-cause repair. Canonical generators, formatters, dependency
   bootstraps, and their deterministic outputs are in scope for gate closure,
   even when those outputs are outside the milestone's primary paths. Inspect
   every resulting diff and preserve unrelated user changes.
4. Rerun the narrowest command that reproduces the failure. A blind rerun is
   not a repair: flaky tests, timeouts, races, and intermittent workers require
   a diagnosis or a changed precondition before retrying.
5. Once the focused failure is green, rerun every invalidated scoped lane and
   `cy-final-verify`. If another failure appears, restart at step 1 inside the
   same phase action.
6. Update memory with the failure, root cause, repair, and final evidence.
   Only then write the phase's single final state update and summary.

Done when every failure observed in the phase action is repaired, the required
artifacts exist, and the phase's completion gate is PASS.

## Normative classifications

| Failure | Required autonomous action |
| --- | --- |
| CodegenCheck reports a stale generated file and names a generator | Run the canonical generator, inspect all generated diffs, rerun CodegenCheck, then rerun `cy-final-verify`. Stale generated sidecars are this case, not a blocker. |
| Formatter, lint, typecheck, build, or test failure | Diagnose and fix the owning source or contract, run the focused lane, then rerun `cy-final-verify`. |
| Test timeout, race, or intermittent failure | Reproduce under bounded conditions, find the production/test-infrastructure cause, fix it without weakening the test or inflating timeouts, then rerun the gates. |
| Missing local tool, generated dependency, or bootstrap state | Use the repository's canonical install/bootstrap command when it is safe and deterministic, then resume the action. |
| QA finds an in-scope completion or data-loss bug | Fix the product, rerun the affected journey and `qa-execution`, then close Phase C only when ready. |
| Worker launch, evidence, or artifact failure | Repair/relaunch the worker lane and verify its output; keep the phase action open. |
| Checkpoint hook or commit failure | Fix the hook/root cause, rerun verification when source changed, and retry the normal checkpoint without bypass flags. |
| A criterion turns out to be unprovable as written (ambiguous, split across surfaces, contradicted by the spec) | Resolve against the spec documents: record the interpretation in memory, `--add-criterion` the corrected exit condition(s), and prove those. The original stays pending only if genuinely superseded — note the supersession in `MEMORY.md`. |
| A third corrective touch in one area | Perform the repository-required structural redesign. When policy requires a new corrective TechSpec, activate its required authoring skills and let the loop decide choices bounded by the current goal/contract; validate it and continue. |

## External-blocker test

Stop only when all of these are true:

1. The phase cannot reach its completion criterion without a specific missing
   credential, authorization, destructive-operation approval, product
   decision, external service, or unavailable infrastructure.
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
