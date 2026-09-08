# Phase B mode=free — execute one slice

1. Use the grounded `_spec.md` deliverables and acceptance; compare
   against `state.progress.checklist[]`.
2. Pick a coherent slice that advances at least one
   acceptance criterion; capture its text exactly.
3. Run `python3 .agents/skills/cy-loop-tasks/scripts/update-state.py <slug> --add-progress "<slice text>" --action "slice picked" --outcome completed`.
4. Re-read `state.yaml`; the current memory file is
   `memory/free-iter-<NNN>.md`, `<NNN>` = the new checklist entry's
   `iteration`, zero-padded to three digits.
5. **Frontend lane** — when `state.frontend_agent` is set AND the slice's
   owned paths are exclusively frontend surfaces (classification in
   `references/herdr-delegation.md`): dispatch per that reference. The
   worker owns implementation, memory updates, focused validation, and
   `cy-final-verify` evidence; it never commits. Skip step 6.
6. **Local lane** — implement the slice, record decisions and evidence in
   current memory, and validate its outcome under the SKILL.md ownership map.
   Assess the focused evidence with `cy-final-verify` in that same step;
   flag remaining integration journeys/visual rows for Phase C.
7. Confirm memory is updated and `cy-final-verify` evidence is PASS. For the
   frontend lane, verify the worker's evidence instead of re-running verify.
   Satisfy the repository's cached `make gate` before checkpointing.
8. Acceptance self-check: when every implementation criterion has a completed
   checklist entry, add `--deliverables-complete` to the step 9 call.
9. Run `python3 .agents/skills/cy-loop-tasks/scripts/update-state.py <slug> --phase B --complete-progress "<slice text>" [--deliverables-complete] --action "slice <text>" --outcome completed --memory-written "memory/free-iter-<NNN>.md,memory/MEMORY.md" --verify-pass`.
10. Run `python3 .agents/skills/cy-loop-tasks/scripts/commit-checkpoint.py <slug> --slice "<slice text>"`
   with the exact step 3 text. Record the returned SHA or `SKIP: no changes`;
   diagnose a failed checkpoint through `references/recovery-loop.md`.

Done when: the slice's checklist entry is `completed` and the checkpoint
result is recorded.
