# Phase B mode=tasks — execute one task

1. Take the task printed by detect-phase (`task=<stem>`). Read
   `.compozy/tasks/<slug>/<stem>.md` and confirm frontmatter `status:` is
   `pending` or `in_progress`. Frontmatter wins — on drift, reconcile with
   `update-state.py <slug> --task-completed <stem>` for already-finished
   tasks or `--reconcile-tasks` for a late-authored graph, then re-run
   detect-phase.
2. Mark the task active: flip its frontmatter `status:` to `in_progress`,
   then run `python3 .agents/skills/cy-loop-tasks/scripts/update-state.py <slug> --task-current <stem>`
   (a marker call — it records no iteration; `--task-completed` clears it
   automatically at step 8).
3. Ground the picked task once; reuse preflight and shared contract evidence.
   Check only newly encountered dependencies or unresolved inconsistencies.
4. Resolve the shared and current memory paths from
   `references/memory-protocol.md` and pass them into the lane that executes
   the work.
5. **Frontend lane** — when detect-phase printed `lane=frontend agent=<x>`:
   dispatch the task to that worker per `references/herdr-delegation.md`.
   The worker owns implementation, memory updates, focused validation, and
   `cy-final-verify` evidence. It never commits. Skip step 6.
6. **Local lane** — activate `cy-execute-task` with auto-commit disabled.
   Apply the validation ownership map to the task's outcome: run or cite
   current owning-suite/probe evidence, review the diff, and assess it with
   `cy-final-verify` in the same step. Flag affected `docs/qa/scenarios/`
   and remaining integration/visual checks for Phase C. Per-task peer review
   stays in Phase D. Do not launch full QA to close an implementation task.
7. Confirm memory is updated (written locally, or verified from the worker)
   and that `cy-final-verify` evidence is PASS before any state flip. For the
   frontend lane, verify the worker's evidence instead of re-running verify.
   Before checkpointing, satisfy `make gate` with its cached affected lanes;
   do not add another full suite. A task's focused PASS is not a gate exemption.
8. Run `python3 .agents/skills/cy-loop-tasks/scripts/update-state.py <slug> --phase B --task-completed <stem> --action "executed <stem>" --outcome completed --memory-written "memory/<stem>.md,memory/MEMORY.md" --verify-pass`.
9. Run `python3 .agents/skills/cy-loop-tasks/scripts/commit-checkpoint.py <slug> --task <stem>`.
   Stdout starts with a commit SHA or `SKIP: no changes`; copy it into the
   iteration summary (in stacked mode a `stack: submitted` line follows —
   the script owns the layer branch and PR submission, see
   `references/stacked-prs.md`). On exit 1, enter the repair loop and retry
   the normal checkpoint after its root cause is fixed. Never bypass the
   hook with `--no-verify`.

Done when: task frontmatter, memory, `state.yaml`, and the checkpoint result
all reflect the same completed task.
