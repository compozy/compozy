---
name: cy-implement-spec
description: Spec-direct execution loop for Compozy specs — self-healing checkpoint driver that ships a spec end to end straight from its documents, no task decomposition: the spec's acceptance criteria become tracked exit conditions and the model works in long-running milestones cut by provability, one atomic commit each, then QA and deep-review peer-review rounds until SHIP. Use for continuous codex-loop goal runs over a .compozy/tasks/<slug> that has _techspec.md but no task graph, or when decomposition is deliberately skipped to keep the whole spec in one model context. Do not use for one-off tasks, PRD/TechSpec authoring, or a slug carrying an authored task graph (use cy-loop-tasks).
---

# Implement Spec Driver

Drive a Compozy spec to completion as a **self-healing continue** loop that
works **directly on the spec documents** — there is no task queue and none
is ever created. Task decomposition existed to fit specs into small model
contexts; this loop instead registers the spec's own exit conditions as
**criteria** (`pending → met`) and lets the model manage the work between
checkpoints in its own context. State tracks *what is proven*, never *what
to do next*.

Each iteration detects the current phase, runs exactly one phase action,
writes memory, updates `state.yaml`, and prints the iteration summary —
then **continues** at detect unless the outcome is an evidence-backed
external blocker or Phase E. A failed command stays inside the current
phase action: diagnose, repair, and rerun it before writing iteration
state. Filesystem state still resumes cleanly if the session ends mid-loop.

The loop is a five-phase state machine:

| Phase | Action | Executor |
|-------|--------|----------|
| 0 | bootstrap: read spec docs, register criteria | orchestrator |
| B | one milestone + verify + checkpoint commit, until every criterion is met | orchestrator |
| C | `qa_report`, then `qa_execution` | Fable 5 herdr worker, then orchestrator |
| D | `deep-review` rounds until SHIP + checkpoint commit per round | orchestrator |
| E | done-signature | orchestrator |

Compatible with `~/dev/ai/codex-loop-plugin` goal mode; the plugin itself
is never modified. Prefer in-session **continue** over waiting for a
Stop→restart — restarts are a resume safety net, not the driver.

## Inputs

- `<slug>` — directory name under `.compozy/tasks/`.
- `<goal_text>` — verbatim `[[CODEX_LOOP goal="..."]]` text or the manual
  reason for the run. Captured once at bootstrap into
  `state.yaml.goal_signature`. Syntax in `references/goal-header-template.md`.
- A pre-authored `.compozy/tasks/<slug>/_techspec.md`. Without it, bootstrap
  stops with a blocker. Companion documents are read when present:
  `_prd.md`, `_tests.md`, `_user_stories.md`, `_design-spec.md`, `adrs/`.
- The slug must carry **no** `_tasks.md` / `task_*.md` — `init-state.py`
  refuses a task graph (exit 5); that slug belongs to `cy-loop-tasks`.

## Helper scripts

Bundled under `.agents/skills/cy-implement-spec/scripts/` — stdlib-only
Python 3.11+, no network, no model calls. Invoke by the explicit repo-root
paths shown in the workflow steps.

| Script | Role | Phase |
|--------|------|-------|
| `_state_io.py` | private strict state codec (imported, not invoked) | all state helpers |
| `init-state.py` | bootstrap (mutating once) | 0 |
| `detect-phase.py` | read-only | every iteration |
| `update-state.py` | mutating | every iteration |
| `commit-checkpoint.py` | mutating (git commit) | B, D |
| `test_scripts.py` | read-only self-test | skill maintenance only |

## Worker delegation

One lane dispatches work to a herdr worker TUI, for judgment independence:
the orchestrator implements the whole spec, so `qa_report` (Phase C) is
always produced by a fresh Claude Fable 5 worker with unbiased eyes —
launched direct, never plan-first. Before any dispatch, read
`references/worker-delegation.md` in full and activate the
`herdr-orchestration` skill it builds on. Every other action runs locally.

## Workflow

Each **iteration** is one detect → phase action → memory → state →
summary cycle. After a completed (non-blocked) summary that is not
Phase E, **continue** at Step 1 in the same turn — do not end the
session between rounds.

If any command, gate, worker, or artifact check fails during a phase action,
read `references/recovery-loop.md` in full immediately and run its repair loop.
Do not write final iteration state or print the summary for an intermediate
failure.

**Step 1 — Detect.**

1. Print `pwd` and confirm the working directory is the repo root (the
   directory containing `.compozy/tasks/`). On mismatch, locate that root,
   change into it, and confirm again; block only when the task tree is absent.
2. Activate `cy-workflow-memory` so its protocol is loaded for later use
   (once per session is enough; re-activate only if context was dropped).
3. Run `python3 .agents/skills/cy-implement-spec/scripts/detect-phase.py <slug>`.
   The printed line decides the whole iteration; the output catalog and
   entry/exit conditions are in `references/phase-transitions.md`.

Done when: detect-phase emitted exactly one supported phase/action line and
the matching branch below is selected.

**Step 2 — Run exactly one phase branch.**

### Phase 0 — Bootstrap

1. Confirm `.compozy/tasks/<slug>/_techspec.md` exists. Missing → scaffold
   `memory/MEMORY.md` from `references/memory-protocol.md`, record the
   blocker under `## Open Risks`, print the iteration summary with
   `outcome=blocked`, and stop (`state.yaml` does not exist yet, so there is
   no update-state call).
2. Activate `cy-spec-preflight` so the repository's playbook, lessons,
   directives, and glossary ground the whole run.
3. Read every spec document in the slug **in full** — `_techspec.md` plus
   the companions listed under Inputs. These documents are the entire
   contract; nothing else will be generated from them.
4. Extract the spec's exit conditions into a criteria list: every
   acceptance criterion and deliverable from `_techspec.md`, anchored to
   spec text — split or merge entries only so each is independently
   provable, and keep each unique. When `_tests.md` exists, add one
   standing criterion: "every test case in _tests.md is implemented in its
   owning suite". Criteria are **exit conditions, not work items** — never
   register a to-do list.
5. Run `python3 .agents/skills/cy-implement-spec/scripts/init-state.py <slug> --goal "<goal_text>" --criterion "<text>"`,
   repeating `--criterion` once per extracted criterion.
6. Scaffold `.compozy/tasks/<slug>/memory/MEMORY.md` with the canonical
   sections from `references/memory-protocol.md`.
7. Run `python3 .agents/skills/cy-implement-spec/scripts/update-state.py <slug> --phase 0 --action "bootstrap (<N> criteria registered)" --outcome completed --memory-written "memory/MEMORY.md"`.

Done when: `state.yaml` exists with every criterion registered and
`memory/MEMORY.md` has the canonical sections.

### Phase B — implement one milestone

There is no queue to pull from: re-read the spec, judge the gap between
the codebase and the pending criteria, and close part of it. One iteration
= one **milestone** — a coherent, provable increment chosen by the model
(a subsystem landed, a contract co-shipped end to end, a vertical slice
passing its tests). No size ceiling: implement as far as coherent judgment
allows before checkpointing; a milestone is cut by provability, not by
clock or file count.

1. Re-anchor from filesystem truth: `state.yaml` (which criteria are
   pending), `memory/MEMORY.md` (decisions so far), and — on the first
   Phase B iteration or after any context loss or compaction — the spec
   documents in full. Never trust a summarized recollection of the spec.
2. State the milestone in one line; this exact text becomes the
   update-state `--action` suffix and the checkpoint commit header. The
   current memory file is `memory/impl-iter-<NNN>.md`,
   `<NNN>` = `state.iteration + 1`, zero-padded to three digits.
3. Activate the repository's required domain skills for the surfaces about
   to be touched (per the repo's skill dispatch table) before writing code.
4. Implement the milestone. Record decisions, learnings, and repairs in the
   current memory file as they happen — memory is the loop's continuity
   across compaction, not an afterthought.
5. Run scoped validation for every touched lane, then `cy-final-verify`.
6. Map evidence to criteria: a criterion is met only when its proof is
   named in the memory file's `## Criteria Advanced` section (command +
   result, test id, or artifact path). Implementation without proof stays
   pending. Skip any peer-review step a spec document requests — that
   review is Phase D (see Critical Rules).
7. Run `python3 .agents/skills/cy-implement-spec/scripts/update-state.py <slug> --phase B --action "milestone: <text>" --outcome completed --criterion-met "<exact text>" --memory-written "memory/impl-iter-<NNN>.md,memory/MEMORY.md" --verify-pass`,
   repeating `--criterion-met` once per criterion proven this iteration
   (omit it for a foundation milestone that proves none — then also record
   in memory what it unblocks). When the last pending criterion is proven,
   add `--implementation-complete` to the same call — the script refuses it
   while any criterion is pending or without `--verify-pass`.
8. Run `python3 .agents/skills/cy-implement-spec/scripts/commit-checkpoint.py <slug> --milestone "<text>"`,
   passing `--coauthor` with the driving agent's identity (e.g.
   `"Codex <noreply@openai.com>"` or `"Claude Fable 5 <noreply@anthropic.com>"`).
   Stdout is a commit SHA or `SKIP: no changes`; copy it into the iteration
   summary. On exit 1, enter the repair loop and retry the normal checkpoint
   after its root cause is fixed. Never bypass the hook with `--no-verify`.

Done when: the milestone's criteria flips, memory, `state.yaml`, and the
checkpoint result all reflect the same completed milestone.

### Phase C — QA

Run only the printed action.

`qa_report` — dispatched, never authored locally:

1. When release-grade runtime scope needs a lab and no active
   `bootstrap-manifest.json` exists, activate the project's QA bootstrap
   skill first (e.g. `agh-qa-bootstrap` in AGH) when installed.
2. Dispatch the Fable 5 worker per `references/worker-delegation.md`. The
   worker activates `qa-report` with `qa-docs-path=docs/qa` and updates
   journey flows, `docs/qa/scenarios/` files, and cycle charters.
3. Verify the worker evidence (each reported artifact exists, no worker
   commit), then run `python3 .agents/skills/cy-implement-spec/scripts/update-state.py <slug> --phase C --qa-report-done --action "qa-report produced" --outcome completed --memory-written "memory/qa-report.md,memory/MEMORY.md"`.

`qa_execution` — local:

1. Activate `qa-execution` with `qa-docs-path=docs/qa`; it writes the dated
   run report at `docs/qa/reports/<YYYY-MM-DD>-<slug>.md` and updates
   scenario-file verdicts.
2. When the report is "not ready" or a Blocks-Completion/Data-Loss bug is
   open, keep the Phase C action open: repair every in-scope bug, rerun the
   affected QA, and repeat `qa-execution` through the recovery loop. Do not
   set `--qa-execution-done` on an intermediate report.
3. Once the report is ready, run `python3 .agents/skills/cy-implement-spec/scripts/update-state.py <slug> --phase C --qa-execution-done --action "qa-execution produced" --outcome completed --memory-written "memory/qa-execution.md,memory/MEMORY.md" --verify-pass`.

Done when: the printed QA artifact exists on disk and its flag is recorded in
`state.yaml`.

### Phase D — peer-review rounds until SHIP

One round per iteration; detect-phase re-emits `peer_review` until the
verdict is SHIP on a verify-PASS tree. Enter this phase only after
`implementation_complete=true` and both QA flags are true.

1. Activate `deep-review` for the round number printed by detect-phase,
   scoped to the loop's full diff: `--base` = the ref the loop started from
   when known (default `main`), `--spec .compozy/tasks/<slug>` (contract
   conformance), `--subagent codex` (cross-LLM reviewer lane — the
   implementing model never solely reviews its own work). Later rounds ride
   deep-review's incremental state; never pass `--full` mid-loop.
2. The loop is the deciding authority over the round: remediate **every
   confirmed finding and every nitpick** from the round's review.md in this
   same iteration, then re-run the project verification gate. The round's
   verdict is the SHIP/FIX_BEFORE_SHIP/REWORK value in review.md/state.json.
3. Update `memory/peer-review.md` (a `## Round <N>` section per round), then
   run `python3 .agents/skills/cy-implement-spec/scripts/update-state.py <slug> --phase D --review-round-done <SHIP|FIX_BEFORE_SHIP|REWORK> --action "peer-review round <N> (<verdict>)" --outcome completed --memory-written "memory/peer-review.md,memory/MEMORY.md" --verify-pass`.
   The call uses `--verify-pass`: a failed post-remediation gate stays inside
   the repair loop, and a SHIP verdict on a failing tree is void.
4. Run `python3 .agents/skills/cy-implement-spec/scripts/commit-checkpoint.py <slug> --review-round <N>`
   (same `--coauthor` and SKIP / exit-1 semantics as Phase B).

Done when: the round's review.md exists with a verdict, every confirmed
finding and nitpick from it is remediated (or the verdict was SHIP), and
`state.yaml` records the round.

### Phase E — done

1. Run a final `cy-final-verify` and confirm `state.verify.last_status=PASS`.
   A regression enters the repair loop and Phase E remains open until the
   fresh gate passes; skip the done-signature while repairing.
2. Walk the Phase E section of `references/checklist.md`; every box must
   pass.
3. Print the iteration summary block from
   `assets/iteration-summary.template.md` with `phase_out=E` and checkpoint
   field `n/a (phase != B/D)`.
4. Print the literal contents of `assets/done-signature.txt` on its own line
   — the codex-loop goal-check confirmation scans for it.
5. Stop — Phase E is the only successful terminal.

Done when: the Phase E checklist passes, the iteration summary is printed,
and the done-signature is the final output line.

**Step 3 — Self-audit, summarize, then continue.**

1. Walk `references/checklist.md` for the phase just executed; every box must
   pass before summarizing.
2. Print the iteration summary block from
   `assets/iteration-summary.template.md` (Phase E already printed it and
   adds only the done-signature line).
3. **Continue gate:** stop only when `phase_out=E` or the external-blocker
   criteria in `references/recovery-loop.md` are proven and
   `outcome=blocked`. Otherwise re-enter Step 1 immediately — the summary
   marks the round; it does not end the session.

Done when: the phase checklist passes, its summary is printed, and control
either returned to Step 1 or stopped at a permitted terminal.

## Memory protocol

Memory goes through the `cy-workflow-memory` skill — the exact paths per
phase are in `references/memory-protocol.md`. Update memory **before**
flipping any tracking field.

## Goal-mode integration

The canonical `[[CODEX_LOOP ...]]` header and the manual invocation text
live in `references/goal-header-template.md`.

## Critical Rules

- Criteria are **exit conditions, not work items**: `state.yaml` tracks
  what is proven; the model owns the work breakdown inside its own context.
  Never author `_tasks.md` or `task_*.md`, and never activate
  `cy-create-tasks`, `cy-execute-task`, or `cy-tasks-tail-qa-pair` inside
  this loop — a slug that needs a task graph belongs to `cy-loop-tasks`.
- One phase action per iteration; repair failures inside that action, then
  **continue** at detect until Phase E or a proven external blocker — never
  idle between rounds waiting for a restart or re-invocation.
- `state.yaml` mutates only through `init-state.py` and `update-state.py`;
  hand-edits void resume guarantees. There is no top-level `current_phase` —
  `detect-phase.py` derives it from durable state every run.
- Memory updates precede status flips. Always.
- Every `--criterion-met` is backed by proof named in memory (command +
  result, test id, or artifact path); `--implementation-complete` is
  script-guarded — zero pending criteria plus `--verify-pass` in the same
  call — and a criterion is never edited or reverted, only superseded via
  `--add-criterion` with a memory note.
- Every Phase B milestone runs scoped validation then `cy-final-verify`
  before its checkpoint commit. A FAIL opens the repair loop; only the final
  PASS closes the phase action.
- `qa_report` is always produced by the Fable 5 worker; `qa_execution` always
  runs locally. Workers never commit.
- Peer review (`deep-review`) runs only in Phase D. Per-milestone review
  instructions inside spec documents are superseded by this loop's phase
  machine — note "deferred to Phase D" in the iteration memory and move on.
- Phase D repeats in consecutive rounds until SHIP; every non-SHIP round
  remediates all blockers and nits before the next round starts.
- Checkpoint commits (Phases B and D) belong to the orchestrator and capture
  code, memory, and the advanced `state.yaml` in one atomic, restorable
  snapshot.
- Phase E requires `progress.implementation_complete=true`,
  `qa.report_done=true`, `qa.execution_done=true`, `review.ship=true`, and
  `verify.last_status=PASS`.
- Spec regeneration is out of scope: do not rewrite `_techspec.md` or
  `_prd.md` with authoring skills. The only exception is a
  repository-mandated two-touch corrective TechSpec: activate its required
  spec skills, let the loop decide choices already bounded by the current
  goal/contract, persist the corrective design, register any new exit
  conditions via `--add-criterion`, and continue.

## Error Handling

- **Any failure** — read `references/recovery-loop.md` in full and execute it
  before mutating iteration state. Failed commands are repair work, not
  blockers. Use `outcome=blocked` only after its external-blocker test passes.
- **`_techspec.md` missing at bootstrap** — record the blocker in
  `memory/MEMORY.md` `## Open Risks`, print the iteration summary with
  `outcome=blocked`, stop. No update-state call: `state.yaml` does not exist
  yet.
- **Task graph present** — `init-state.py` exits 5. Do not delete the graph
  to force this loop; hand the slug to `cy-loop-tasks` and report the
  mismatch in the summary.
- **`state.yaml` parse failure** — `detect-phase.py` exits 1 with the parse
  error on stderr. Diagnose the malformed writer or interrupted write from
  evidence and repair it without discarding unrelated worktree changes.
- **`--criterion-met` text mismatch** — `update-state.py` exits 1 naming the
  unmatched text. Re-read `state.yaml`, pass the exact stored text, and fix
  the memory note if it quoted the criterion wrong.
- **A criterion is unprovable as written** — follow the criterion
  classification row in `references/recovery-loop.md`: resolve against the
  spec, supersede via `--add-criterion`, record the supersession in memory.
- **`commit-checkpoint.py` exit 1** — repair the hook or commit failure and
  retry normally. If the repair changes tracked source after the last PASS,
  rerun `cy-final-verify` before retrying. `SKIP: no changes` is success.
- **Worker launch or delegation failure** — the pane shows raw JSON instead
  of a TUI banner, `rtk herdr agent list` stays `unknown`, or the status wait
  times out with no progress: interrupt
  (`rtk herdr pane send-keys <pane_id> ctrl+c`) and relaunch once via
  `rtk herdr agent start`; if it fails again, diagnose and repair the worker
  environment through the recovery loop.
- **Delegated run lacks artifacts or committed anyway** — keep the phase
  open, recover the missing evidence or rerun the lane, and do not advance.
  A worker commit is a contract breach that requires preserving the worker's
  work and repairing checkpoint ownership before continuing.
- **Invalid peer-review round** (missing or malformed review artifacts, or no
  verdict) — the round does not count; follow `deep-review` error handling
  and re-run it.
- **Two-touch rule** — on the third corrective touch, replace patching with
  the structural redesign required by the repository, validate it, and
  continue. It becomes a blocker only when that redesign needs an external
  product decision or authority unavailable to the loop.
- **External blocker proven** — record the evidence and exhausted alternatives
  in memory, call `update-state.py` with `--verify-fail --blocker <text> --outcome blocked`,
  print the summary, and stop without the done-signature.
