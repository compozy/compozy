---
name: cy-loop-tasks
description: "Run a requested Compozy spec delivery loop through task checkpoints, scoped QA, review, and PR CI."
---

# Loop Tasks Driver

Run a requested full spec delivery workflow as a **self-healing continue** loop.
Keep working through implementation, applicable QA, review, and required CI until
the delivery contract is met or a concrete external dependency blocks progress.
Ordinary edits and single tasks do not enter this workflow.

## Entry and routing

Resolve the repo root, `.compozy/tasks/<slug>/_spec.md`, and the user's goal once.
The initializer detects `tasks` mode from `_tasks.md` plus `task_*.md`; otherwise
it uses `free` mode. Optional `--frontend <claude|cursor>` delegates frontend
implementation; `--stacked` publishes task checkpoints as stacked PRs.
Read `references/stacked-prs.md` before using that option. Invocation examples
are in `references/goal-header-template.md` when needed.

Run `python3 .agents/skills/cy-loop-tasks/scripts/detect-phase.py <slug>` and load
only the matching procedure below. Reuse a loaded procedure while current;
entering another task does not require reloading every skill or shared contract.

| Detector output | Procedure to read |
| --- | --- |
| `phase=0 action=bootstrap` | [Bootstrap](references/phase-bootstrap.md) |
| `phase=B action=execute_task` | [Task checkpoint](references/phase-task.md) |
| `phase=B action=execute_free_slice` | [Free-mode slice](references/phase-slice.md) |
| `phase=C action=qa_report` or `qa_execution` | [QA](references/phase-qa.md), only the printed action |
| `phase=D action=peer_review` | [Review round](references/phase-review.md) |
| `phase=E action=await_ci` or `done` | [Delivery](references/phase-delivery.md) |

Each action completes its owned work, records meaningful evidence in memory,
advances writer-owned state, and emits `assets/iteration-summary.template.md`.
Then run detect again and continue in the same turn. The phase boundary is a
resume checkpoint, not a request for user review or a reason to end the session.

## Evidence ownership

Apply `cy-final-verify` within the existing validation step. Reuse valid evidence
for unchanged source, dependencies, build/config, and environment; a handoff,
phase transition, new message, or commit alone does not invalidate it.

| Boundary | Evidence it owns |
| --- | --- |
| Task/slice | Checks proving its outcome; explicit task-owned live/visual acceptance. Record remaining integration journeys/visual rows and their owner. |
| Integration QA | Remaining changed/cross-task journeys and required visual rows on the integrated build, reusing current task evidence. |
| Repair | Reproduction and checks/journeys/visual rows invalidated by the fix. |
| Commit/push | Repository `make gate` policy with its fingerprint cache and applicable docs-only exemption. |
| PR delivery | Requested outcomes, resolved QA/visual obligations, review SHIP, and required CI green at the current head. |

Changed user journeys need real-run evidence. Unit results alone do not close
integration QA. A UI path does not require both full E2E suites, a new lab, or a
complete screenshot bundle per task. With named visual references, inspect a
representative state early and complete required rows at their assigned boundary.
Preserve explicit acceptance requirements; route older generic per-task QA
boilerplate through the ownership map without narrowing the accepted outcome.

## State, memory, and delegation

- Use `init-state.py` and `update-state.py` under this skill's `scripts/` as the
  only state writers. Task frontmatter owns task status; `_tasks.md` owns topology.
  Keep the bootstrap `goal_signature` unchanged; later scope decisions go in memory.
- Load `references/memory-protocol.md` on entry or recovery for memory paths and
  format. Record meaningful decisions/evidence before completion status; preserve
  valid grounding and update only the current task and shared facts it changes.
- When the detector selects a frontend lane, or QA planning is explicitly
  delegated/usefully independent, read the applicable lane in
  `references/herdr-delegation.md`. The controller owns state and commits;
  workers own their assigned diff/evidence. Reuse workers for related follow-ups.
- Before `commit-checkpoint.py`, inspect staged/unstaged paths. Its `git add -A`
  contract is usable only when every dirty path belongs to this checkpoint.
  Otherwise stage and commit only owned paths with the checkpoint message.

## Failures and completion

Repair failed required checks within the current action. Do not write final iteration state
or print a completion summary for an intermediate failure. Expected search misses
and normal live-worker wait expiry are not failed checks. Use
`references/recovery-loop.md` for diagnosis, helper failures, and the external
blocker test; reuse the procedure once loaded. Routine safe fixes follow existing
user authorization. Missing permission or a product decision without a safe
default remains a real blocker; continue independent work before asking.

For unexpected detector output/state drift, consult
`references/phase-transitions.md` and `references/state-schema.md`; use
`references/checklist.md` as a targeted resume audit, not another execution gate.

Only Phase E is successful completion: QA obligations are resolved, review is
SHIP, current local evidence is PASS, and every required PR check is green at the
current head. Emit `assets/done-signature.txt` only then. Pending/red CI remains
in progress. A proven external blocker is recorded with evidence and no signature.
