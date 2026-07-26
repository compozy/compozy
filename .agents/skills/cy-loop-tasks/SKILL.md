---
name: cy-loop-tasks
description: Task-graph execution loop for Compozy specs — self-healing checkpoint driver that ships a techspec end to end by executing its authored task graph (_tasks.md + task_NN.md), one atomic commit per Phase B task, QA, then deep-review peer-review rounds until SHIP. Use for continuous codex-loop goal runs over a .compozy/tasks/<slug> whose work is decomposed into task files, or with --frontend to delegate frontend tasks to herdr workers. Do not use for one-off tasks, PRD/TechSpec authoring, or a slug with spec documents but no task graph (use cy-implement-spec).
---

# Loop Tasks Driver

Drive a Compozy techspec to completion as a **self-healing continue** loop: each
iteration detects the current phase, runs exactly one phase action,
writes memory, updates `state.yaml`, and prints the iteration summary —
then **continues** at detect unless the outcome is an evidence-backed external
blocker or Phase E. A failed command stays inside the current phase action:
diagnose, repair, and rerun it before writing iteration state. Filesystem state
still resumes cleanly if the session ends mid-loop.

The loop is a five-phase state machine:

| Phase | Action | Executor |
|-------|--------|----------|
| 0 | bootstrap | orchestrator |
| B | one task or slice + verify + checkpoint commit | orchestrator, or herdr frontend worker |
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
  `state.yaml.goal_signature`.
- `--frontend <claude|cursor>` — optional. Selects the frontend worker agent
  and activates the herdr frontend lane for the whole loop. Captured once at
  bootstrap into `state.yaml.frontend_agent`; when absent, every task runs
  locally. Syntax examples in `references/goal-header-template.md`.
- A pre-authored `.compozy/tasks/<slug>/_techspec.md`. Without it, bootstrap
  stops with a blocker.

## Helper scripts

Bundled under `.agents/skills/cy-loop-tasks/scripts/` — stdlib-only
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

## Herdr delegation lanes

Two lanes dispatch work to herdr worker TUIs. Before any dispatch, read
`references/herdr-delegation.md` in full and activate the
`herdr-orchestration` skill it builds on.

- **Frontend lane (Phase B)** — active only when `state.frontend_agent` is
  set. While active, every frontend task or slice is dispatched to the
  selected worker (`claude` → Claude Code Opus at xhigh effort, `cursor` →
  `cursor-agent --yolo --model grok-4.5`); the orchestrator session stays in
  orchestration mode and never implements frontend work itself.
- **QA-report lane (Phase C)** — always active. `qa_report` is produced by a
  Claude Fable 5 worker (`claude --permission-mode auto --model
  claude-fable-5`), launched direct — never plan-first. The orchestrator runs
  `qa_execution` itself.

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
3. Run `python3 .agents/skills/cy-loop-tasks/scripts/detect-phase.py <slug>`.
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
2. Run `python3 .agents/skills/cy-loop-tasks/scripts/init-state.py <slug> --goal "<goal_text>"`,
   adding `--frontend <claude|cursor>` when the invocation text carries the
   parameter. Mode auto-detects: `tasks` when `_tasks.md` plus at least one
   `task_*.md` exist, else `free`.
3. Activate `cy-spec-preflight` for the phase the next iteration enters
   (`tasks`, or `task-body` when a single concrete file is next).
4. Scaffold `.compozy/tasks/<slug>/memory/MEMORY.md` with the canonical
   sections from `references/memory-protocol.md`.
5. Run `python3 .agents/skills/cy-loop-tasks/scripts/update-state.py <slug> --phase 0 --action "bootstrap (mode=<mode>)" --outcome completed --memory-written "memory/MEMORY.md"`.

Done when: `state.yaml` exists and `memory/MEMORY.md` has the canonical
sections.

### Phase B mode=tasks — execute one task

1. Take the task printed by detect-phase (`task=<stem>`). Read
   `.compozy/tasks/<slug>/<stem>.md` and confirm frontmatter `status:` is
   `pending` or `in_progress`. Frontmatter wins — on drift, reconcile with
   `update-state.py <slug> --task-completed <stem>` for already-finished
   tasks or `--reconcile-tasks` for a late-authored graph, then re-run
   detect-phase.
2. Activate `cy-spec-preflight` in `task-body` mode for the picked file.
3. Resolve the shared and current memory paths from
   `references/memory-protocol.md` and pass them into the lane that executes
   the work.
4. **Frontend lane** — when detect-phase printed `lane=frontend agent=<x>`:
   dispatch the task to that worker per `references/herdr-delegation.md`.
   The worker owns implementation, memory updates, scoped validation, and
   `cy-final-verify` evidence. It never commits. Skip step 5.
5. **Local lane** — activate `cy-execute-task` on the picked file with
   auto-commit disabled. Run the task's scoped validation, then
   `cy-final-verify`. Skip any per-task peer-review step the task
   file requests — that review is Phase D (see Critical Rules).
6. Confirm memory is updated (written locally, or verified from the worker)
   and that `cy-final-verify` evidence is PASS before any state flip. For the
   frontend lane, verify the worker's evidence instead of re-running verify.
7. Run `python3 .agents/skills/cy-loop-tasks/scripts/update-state.py <slug> --phase B --task-completed <stem> --action "executed <stem>" --outcome completed --memory-written "memory/<stem>.md,memory/MEMORY.md" --verify-pass`.
8. Run `python3 .agents/skills/cy-loop-tasks/scripts/commit-checkpoint.py <slug> --task <stem>`.
   Stdout is a commit SHA or `SKIP: no changes`; copy it into the iteration
   summary. On exit 1, enter the repair loop and retry the normal checkpoint
   after its root cause is fixed. Never bypass the hook with `--no-verify`.

Done when: task frontmatter, memory, `state.yaml`, and the checkpoint result
all reflect the same completed task.

### Phase B mode=free — execute one slice

1. Re-read `_techspec.md` deliverables and acceptance in full; compare
   against `state.progress.checklist[]`.
2. Pick the smallest coherent slice (≤ ~4 hours) that advances at least one
   acceptance criterion; capture its text exactly.
3. Run `python3 .agents/skills/cy-loop-tasks/scripts/update-state.py <slug> --add-progress "<slice text>" --action "slice picked" --outcome completed`.
4. Re-read `state.yaml`; the current memory file is
   `memory/free-iter-<NNN>.md`, `<NNN>` = the new checklist entry's
   `iteration`, zero-padded to three digits.
5. **Frontend lane** — when `state.frontend_agent` is set AND the slice's
   owned paths are exclusively frontend surfaces (classification in
   `references/herdr-delegation.md`): dispatch per that reference. The
   worker owns implementation, memory updates, scoped validation, and
   `cy-final-verify` evidence; it never commits. Skip step 6.
6. **Local lane** — implement the slice, record decisions and learnings in
   the current memory file, run scoped validation, then `cy-final-verify`.
7. Confirm memory is updated and `cy-final-verify` evidence is PASS. For the
   frontend lane, verify the worker's evidence instead of re-running verify.
8. Acceptance self-check: when every techspec criterion has a completed
   checklist entry, add `--deliverables-complete` to the step 9 call.
9. Run `python3 .agents/skills/cy-loop-tasks/scripts/update-state.py <slug> --phase B --complete-progress "<slice text>" [--deliverables-complete] --action "slice <text>" --outcome completed --memory-written "memory/free-iter-<NNN>.md,memory/MEMORY.md" --verify-pass`.
10. Run `python3 .agents/skills/cy-loop-tasks/scripts/commit-checkpoint.py <slug> --slice "<slice text>"`
   with the exact step 3 text — same SKIP / exit-1 semantics as mode=tasks
   step 8.

Done when: the slice's checklist entry is `completed` and the checkpoint
result is recorded.

### Phase C — QA

Run only the printed action.

`qa_report` — dispatched, never authored locally:

1. When release-grade runtime scope needs a lab and no active
   `bootstrap-manifest.json` exists, activate the project's QA bootstrap
   skill first (e.g. `agh-qa-bootstrap` in AGH) when installed.
2. Dispatch the Fable 5 worker per `references/herdr-delegation.md`
   (QA-report lane). The worker activates `qa-report` with
   `qa-docs-path=docs/qa` and updates journey flows, `docs/qa/scenarios/`
   files, and cycle charters.
3. Verify the worker evidence (each reported artifact exists, no worker
   commit), then run `python3 .agents/skills/cy-loop-tasks/scripts/update-state.py <slug> --phase C --qa-report-done --action "qa-report produced" --outcome completed --memory-written "memory/qa-report.md,memory/MEMORY.md"`.

`qa_execution` — local:

1. Activate `qa-execution` with `qa-docs-path=docs/qa`; it writes the dated
   run report at `docs/qa/reports/<YYYY-MM-DD>-<slug>.md` and updates
   scenario-file verdicts.
2. When the report is "not ready" or a Blocks-Completion/Data-Loss bug is
   open, keep the Phase C action open: repair every in-scope bug, rerun the
   affected QA, and repeat `qa-execution` through the recovery loop. Do not
   set `--qa-execution-done` on an intermediate report.
3. Once the report is ready, run `python3 .agents/skills/cy-loop-tasks/scripts/update-state.py <slug> --phase C --qa-execution-done --action "qa-execution produced" --outcome completed --memory-written "memory/qa-execution.md,memory/MEMORY.md" --verify-pass`.

mode=tasks addition: when the printed QA action corresponds to the pending QA
task file, flip that task's frontmatter `status:` to `completed` and add
`--task-completed <stem>` to the same update-state call so `tasks.pending`
drains.

Done when: the printed QA artifact exists on disk and its flag is recorded in
`state.yaml`.

### Phase D — peer-review rounds until SHIP

One round per iteration; detect-phase re-emits `peer_review` until the
verdict is SHIP on a verify-PASS tree. Enter this phase only after every
Phase B task or slice is complete and both QA flags are true.

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
   run `python3 .agents/skills/cy-loop-tasks/scripts/update-state.py <slug> --phase D --review-round-done <SHIP|FIX_BEFORE_SHIP|REWORK> --action "peer-review round <N> (<verdict>)" --outcome completed --memory-written "memory/peer-review.md,memory/MEMORY.md" --verify-pass`.
   The call uses `--verify-pass`: a failed post-remediation gate stays inside
   the repair loop, and a SHIP verdict on a failing tree is void.
4. Run `python3 .agents/skills/cy-loop-tasks/scripts/commit-checkpoint.py <slug> --review-round <N>`
   — same SKIP / exit-1 semantics as Phase B.

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

The canonical `[[CODEX_LOOP ...]]` header, the manual invocation text, and
`--frontend` syntax live in `references/goal-header-template.md`.

## Critical Rules

- One phase action per iteration; repair failures inside that action, then
  **continue** at detect until Phase E or a proven external blocker — never
  idle between rounds waiting for a restart or re-invocation.
- `state.yaml` mutates only through `init-state.py` and `update-state.py`;
  hand-edits void resume guarantees. There is no top-level `current_phase` —
  `detect-phase.py` derives it from durable state and filesystem truth every
  run.
- Frontmatter `status:` on `task_NN.md` is the source of truth; reconcile
  `state.yaml` when they disagree.
- Memory updates precede status flips. Always.
- Frontend lane: `state.frontend_agent` set → herdr dispatch is the only way
  frontend work gets implemented; null → every task runs locally. Workers are
  interactive TUIs launched via `rtk herdr agent start` — a pane streaming
  raw JSON is a broken headless delegation: interrupt and relaunch per
  `references/herdr-delegation.md`.
- `qa_report` is always produced by the Fable 5 worker; `qa_execution` always
  runs locally.
- Every Phase B task or slice runs scoped validation then `cy-final-verify`
  before its checkpoint commit. A FAIL opens the repair loop; only the final
  PASS closes the phase action.
- Peer review (`deep-review`) runs only in Phase D. Per-task peer-review
  instructions inside task files or specs are superseded by this loop's phase
  machine — note "deferred to Phase D" in the task memory and move on.
- Phase D repeats in consecutive rounds until SHIP; every non-SHIP round
  remediates all blockers and nits before the next round starts.
- Checkpoint commits (Phases B and D) belong to the orchestrator:
  `cy-execute-task` runs with auto-commit disabled, and every worker packet
  forbids committing. The checkpoint captures code, memory, task frontmatter,
  the master tasks file, and the advanced `state.yaml` in one atomic,
  restorable snapshot.
- Phase E requires `qa.report_done=true`, `qa.execution_done=true`,
  `review.ship=true`, and `verify.last_status=PASS`.
- Do not regenerate the loop's input graph with `cy-create-tasks`,
  `cy-create-techspec`, `cy-tasks-tail-qa-pair`, or `cy-web-docs-impact`.
  The only exception is a repository-mandated two-touch corrective TechSpec:
  activate its required spec skills, let the loop decide choices already
  bounded by the current goal/contract, persist the corrective design, and
  continue without replacing the original task graph.

## Error Handling

- **Any failure** — read `references/recovery-loop.md` in full and execute it
  before mutating iteration state. Failed commands are repair work, not
  blockers. Use `outcome=blocked` only after its external-blocker test passes.
- **`_techspec.md` missing at bootstrap** — record the blocker in
  `memory/MEMORY.md` `## Open Risks`, print the iteration summary with
  `outcome=blocked`, stop. No update-state call: `state.yaml` does not exist
  yet.
- **Mode disagreement** — `init-state.py` exits 4 when `--mode` contradicts
  the filesystem. Reconcile by adding/removing `_tasks.md` before bootstrap,
  or run `update-state.py <slug> --reconcile-tasks` when the task graph was
  authored after a free-mode bootstrap.
- **`state.yaml` parse failure** — `detect-phase.py` exits 1 with the parse
  error on stderr. Diagnose the malformed writer or interrupted write from
  evidence and repair it without discarding unrelated worktree changes.
- **`commit-checkpoint.py` exit 1** — repair the hook or commit failure and
  retry normally. If the repair changes tracked source after the last PASS,
  rerun `cy-final-verify` before retrying. `SKIP: no changes` is success.
- **Worker launch or delegation failure** — the pane shows raw JSON instead
  of a TUI banner, `rtk herdr agent list` stays `unknown`, or the status wait
  times out with no progress: interrupt
  (`rtk herdr pane send-keys <pane_id> ctrl+c`) and relaunch once via
  `rtk herdr agent start`; if it fails again, diagnose and repair the worker
  environment through the recovery loop.
- **Delegated run lacks PASS evidence or artifacts, or committed anyway** —
  keep the phase open, recover the missing evidence or rerun the lane, and do
  not advance. A worker commit is a contract breach that requires preserving
  the worker's work and repairing checkpoint ownership before continuing.
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
