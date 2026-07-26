# Per-iteration self-audit checklist

Walk this checklist before printing the iteration summary block. Failing any
item means the iteration is **not** complete: do the missing work, then
re-check. Do not print the done-signature until every item passes on the
final iteration.

## Every iteration

- [ ] `.agents/skills/cy-implement-spec/scripts/detect-phase.py` was run as the first action and its printed line was followed.
- [ ] The dispatched skills — or the herdr worker dispatch for the delegated `qa_report` iteration — were activated **before** any code edits or reviews.
- [ ] Any iteration that flips state updated memory first via `cy-workflow-memory` (sequence: memory → criteria → state → commit); Phase E and bootstrap blockers before `state.yaml` exists are read-only exceptions.
- [ ] Every non-E iteration with an existing `state.yaml` called `.agents/skills/cy-implement-spec/scripts/update-state.py` with the right flags; Phase E and bootstrap blockers before state creation are exceptions.
- [ ] `cy-final-verify` ran for any iteration that produced code or fixes; PASS/FAIL evidence was captured and cited in the summary's `verify_evidence`.
- [ ] Every command/gate/worker/artifact failure ran the self-healing procedure in `references/recovery-loop.md`; no intermediate failure wrote final iteration state or ended the session.
- [ ] Any `outcome=blocked` satisfies all three external-blocker criteria with evidence of exhausted safe alternatives; a repairable failure never appears as a blocker.
- [ ] No `_tasks.md`, `task_*.md`, or task-graph tooling (`cy-create-tasks`, `cy-execute-task`, `cy-tasks-tail-qa-pair`) was produced or activated by this loop.
- [ ] For any herdr dispatch: the worker launched as a TUI (banner + input box, status left `unknown`), and HEAD is unchanged (no worker commit).
- [ ] The iteration summary block (from `assets/iteration-summary.template.md`) was printed after the phase work. On completed non-E outcomes, Step 1 was re-entered immediately (**continue**); on blocked or Phase E, the session stopped (Phase E adds the done-signature as the final line).

## Phase 0 (bootstrap) only

- [ ] `_techspec.md` existence was confirmed before writing `state.yaml`.
- [ ] Every spec document present in the slug was read **in full** before extracting criteria (`_techspec.md`, and when present `_prd.md`, `_tests.md`, `_user_stories.md`, `_design-spec.md`, `adrs/`).
- [ ] Each registered criterion is an exit condition anchored to spec text, independently provable, and unique; together they cover every techspec acceptance criterion and deliverable (self-quote the mapping in the summary).
- [ ] When `_tests.md` exists, a standing criterion covering its test contract was registered.
- [ ] `goal_signature` was copied verbatim from the user's prompt (CODEX_LOOP `goal=` value or manual reason).

## Phase B only

- [ ] The milestone was stated in one line BEFORE implementation started, and that exact text went to both `update-state.py --action` and `commit-checkpoint.py --milestone`.
- [ ] The milestone is a coherent, provable increment cut by provability — not a timer tick or an arbitrary file batch. A milestone that flips no criterion names what it unblocks in memory.
- [ ] The repository's required domain skills for the touched surfaces were activated before code was written.
- [ ] Scoped validation ran for every touched lane, every failure was repaired in the same phase action, then `cy-final-verify` passed before `update-state.py`.
- [ ] Every `--criterion-met` flag is backed by proof named in `## Criteria Advanced` of the iteration's memory file (command + result, test id, or artifact path) — implementation without proof stays pending.
- [ ] If `--implementation-complete` was passed: every criterion has status `met`, and the summary self-quotes each criterion → its proof.
- [ ] Exactly ONE milestone was closed in this iteration.
- [ ] No peer-review round (`deep-review`) ran in this iteration — review instructions inside spec documents are deferred to Phase D.
- [ ] `commit-checkpoint.py <slug> --milestone "<text>"` ran after `update-state.py` and printed a commit SHA or the literal `SKIP: no changes`, captured in the summary's checkpoint field.

## Phase C only

- [ ] `qa_report` completed before `qa_execution` (never skip ahead).
- [ ] `qa_report` was produced by the Fable 5 herdr worker — the orchestrator only verified artifacts; `qa_execution` ran locally.
- [ ] If `bootstrap-manifest.json` was missing, a QA bootstrap skill (e.g. `agh-qa-bootstrap`) ran first — or its absence in this project was noted before falling through.
- [ ] A "not ready" report or Blocks-Completion/Data-Loss bug was repaired and retested before `--qa-execution-done`; no intermediate QA failure advanced state.

## Phase D only

- [ ] `implementation_complete=true` and both QA flags are true before this round.
- [ ] Exactly ONE `deep-review` round ran, scoped to the loop's full diff with `--spec .compozy/tasks/<slug>` and `--subagent codex`.
- [ ] Every blocker and every nit from the round's findings was remediated in this same iteration (or the verdict was SHIP).
- [ ] The verification gate re-ran after remediation; any failure entered the repair loop, and every closed review round was recorded with `--verify-pass`.
- [ ] `commit-checkpoint.py <slug> --review-round <N>` ran after `update-state.py` and its result is captured in the summary's checkpoint field.

## Phase E only

- [ ] `progress.implementation_complete=true`, `qa.report_done=true`, `qa.execution_done=true`, AND `review.ship=true` confirmed via `state.yaml`, not memory.
- [ ] `verify.last_status` is `PASS` and the timestamp is recent (same iteration as Phase E entry).
- [ ] The done-signature from `assets/done-signature.txt` is the LAST line of the message.
