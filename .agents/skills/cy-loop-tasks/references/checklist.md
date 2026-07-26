# Per-iteration self-audit checklist

Walk this checklist before printing the iteration summary block. Failing any
item means the iteration is **not** complete: do the missing work, then
re-check. Do not print the done-signature until every item passes on the
final iteration.

## Every iteration

- [ ] `.agents/skills/cy-loop-tasks/scripts/detect-phase.py` was run as the first action and its printed line was followed.
- [ ] The dispatched `cy-*` skills — or the herdr worker dispatch for delegated iterations — were activated **before** any code edits or reviews.
- [ ] Any iteration that flips state updated memory first via `cy-workflow-memory` (sequence: memory → checkboxes → status → master → commit); Phase E and bootstrap blockers before `state.yaml` exists are read-only exceptions.
- [ ] Every non-E iteration with an existing `state.yaml` called `.agents/skills/cy-loop-tasks/scripts/update-state.py` with the right flags; Phase E and bootstrap blockers before state creation are exceptions.
- [ ] `cy-final-verify` ran for any iteration that produced code or fixes; delegated PASS/FAIL evidence was captured and cited in the summary's `verify_evidence`.
- [ ] Every command/gate/worker/artifact failure ran the self-healing procedure in `references/recovery-loop.md`; no intermediate failure wrote final iteration state or ended the session.
- [ ] Any `outcome=blocked` satisfies all three external-blocker criteria with evidence of exhausted safe alternatives; a repairable failure never appears as a blocker.
- [ ] For any herdr dispatch: the worker launched as a TUI (banner + input box, status left `unknown`), and HEAD is unchanged (no worker commit).
- [ ] The iteration summary block (from `assets/iteration-summary.template.md`) was printed after the phase work. On completed non-E outcomes, Step 1 was re-entered immediately (**continue**); on blocked or Phase E, the session stopped (Phase E adds the done-signature as the final line).

## Phase 0 (bootstrap) only

- [ ] `_techspec.md` existence was confirmed before writing `state.yaml`.
- [ ] `mode` was decided by filesystem, not by guess (`tasks` if `_tasks.md` AND a `task_*.md` exist, else `free`).
- [ ] `goal_signature` was copied verbatim from the user's prompt (CODEX_LOOP `goal=` value or manual reason).
- [ ] `--frontend` was passed to `init-state.py` if and only if the invocation text carried it, and `state.frontend_agent` matches.

## Phase B mode=tasks only

- [ ] `task_NN.md` frontmatter `status:` was checked and trusted as source of truth (state.yaml reconciled if it disagreed).
- [ ] The lane matched detect-phase: `lane=frontend agent=<x>` printed → herdr dispatch; no suffix → local `cy-execute-task`.
- [ ] Scoped validation ran, every failure was repaired in the same phase action, then `cy-final-verify` passed (or worker PASS evidence was verified for the frontend lane) before `update-state.py`.
- [ ] No peer-review round (`deep-review`) ran in this iteration — per-task review instructions were deferred to Phase D.
- [ ] Exactly ONE task was attempted in this iteration.
- [ ] `commit-checkpoint.py <slug> --task <stem>` ran after `update-state.py` and printed a commit SHA or the literal `SKIP: no changes`, captured in the summary's checkpoint field.

## Phase B mode=free only

- [ ] The slice picked was small enough to finish in one iteration (≤ ~4 hours).
- [ ] The slice was added to `progress.checklist[]` BEFORE implementation started.
- [ ] If the slice was delegated, its owned paths were exclusively frontend surfaces per `references/herdr-delegation.md`.
- [ ] Scoped validation ran, every failure was repaired in the same phase action, then `cy-final-verify` passed (or worker PASS evidence was verified for the frontend lane) before `update-state.py`.
- [ ] If `deliverables_complete` was set true: every techspec acceptance criterion has at least one matching `progress.checklist[]` entry with `status=completed`. Self-quote each criterion → its checklist entry in the iteration summary.
- [ ] `commit-checkpoint.py <slug> --slice "<slice text>"` ran after `update-state.py` and printed a commit SHA or `SKIP: no changes`, captured in the summary's checkpoint field.

## Phase C only

- [ ] `qa_report` completed before `qa_execution` (never skip ahead).
- [ ] `qa_report` was produced by the Fable 5 herdr worker — the orchestrator only verified artifacts; `qa_execution` ran locally.
- [ ] If `bootstrap-manifest.json` was missing, a QA bootstrap skill (e.g. `agh-qa-bootstrap`) ran first — or its absence in this project was noted before falling through.
- [ ] In mode=tasks, the corresponding QA task frontmatter was flipped and `--task-completed <stem>` accompanied the QA flag so `tasks.pending` drains.
- [ ] A "not ready" report or Blocks-Completion/Data-Loss bug was repaired and retested before `--qa-execution-done`; no intermediate QA failure advanced state.

## Phase D only

- [ ] Every Phase B task or slice is complete and both QA flags are true before this round.
- [ ] Exactly ONE `deep-review` round ran, scoped to the loop's full diff with `--spec .compozy/tasks/<slug>` and `--subagent codex`.
- [ ] Every blocker and every nit from the round's findings was remediated in this same iteration (or the verdict was SHIP).
- [ ] The verification gate re-ran after remediation; any failure entered the repair loop, and every closed review round was recorded with `--verify-pass`.
- [ ] `commit-checkpoint.py <slug> --review-round <N>` ran after `update-state.py` and its result is captured in the summary's checkpoint field.

## Phase E only

- [ ] `qa.report_done=true`, `qa.execution_done=true`, AND `review.ship=true` confirmed via `state.yaml`, not memory.
- [ ] `verify.last_status` is `PASS` and the timestamp is recent (same iteration as Phase E entry).
- [ ] The done-signature from `assets/done-signature.txt` is the LAST line of the message.
