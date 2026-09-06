# Phase 0 — Bootstrap

1. Confirm `.compozy/tasks/<slug>/_spec.md` exists. Missing → scaffold
   `memory/MEMORY.md` from `references/memory-protocol.md`, record the
   blocker under `## Open Risks`, print the iteration summary with
   `outcome=blocked`, and stop (`state.yaml` does not exist yet, so there is
   no update-state call).
2. Run `python3 .agents/skills/cy-loop-tasks/scripts/init-state.py <slug> --goal "<goal_text>"`,
   adding `--frontend <claude|cursor>` and/or `--stacked` when the invocation
   text carries the parameter (for `--stacked`, first read
   `references/stacked-prs.md` in full and verify its prerequisites). Mode
   auto-detects: `tasks` when `_tasks.md` plus at least one `task_*.md`
   exist, else `free`.
3. Reuse the authored graph's preflight evidence. Apply `cy-spec-preflight`
   only to missing or changed task/contract facts; bootstrap does not repeat
   spec research or task authoring.
4. Scaffold `.compozy/tasks/<slug>/memory/MEMORY.md` with the canonical
   sections from `references/memory-protocol.md`.
5. Run `python3 .agents/skills/cy-loop-tasks/scripts/update-state.py <slug> --phase 0 --action "bootstrap (mode=<mode>)" --outcome completed --memory-written "memory/MEMORY.md"`.

Done when: `state.yaml` exists and `memory/MEMORY.md` has the canonical
sections.
