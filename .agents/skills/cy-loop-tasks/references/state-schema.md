# state.yaml — Schema Reference

Authoritative schema for `.compozy/tasks/<slug>/state.yaml`. The file is the
orchestration layer that lets `cy-loop-tasks` continue across iterations and
resume mid-flight if a session ends. The file is mutated **only** by:

- `.agents/skills/cy-loop-tasks/scripts/init-state.py` (bootstrap)
- `.agents/skills/cy-loop-tasks/scripts/update-state.py` (every iteration)

No other writer is permitted. Hand-editing voids resume guarantees.

## Field reference

| Field | Type | Meaning |
|-------|------|---------|
| `slug` | string | Directory name under `.compozy/tasks/`. Mirrors the value passed at bootstrap. |
| `created_at` | RFC3339 UTC | When `init-state.py` first wrote the file. |
| `last_updated` | RFC3339 UTC | When `update-state.py` last touched the file. |
| `mode` | enum: `tasks` `free` | Decided at bootstrap. `tasks` if `_tasks.md` exists; `free` otherwise. May change only through `update-state.py --reconcile-tasks` when task files are authored after a free-mode bootstrap. |
| `iteration` | int ≥ 0 | Monotonic counter. `update-state.py` increments it once per call. |
| `goal_signature` | string | Verbatim text from the user's `[[CODEX_LOOP goal="..."]]` header (or manual invocation reason). Read-only after bootstrap. |
| `frontend_agent` | `claude` \| `cursor` \| null | Frontend worker selected via `init-state.py --frontend`. Non-null activates the herdr frontend lane for the whole loop. Read-only after bootstrap. |
| `tasks.total` | int | Total entries in `_tasks.md`. Populated only in `mode=tasks`. |
| `tasks.completed` | list[string] | Stems (e.g., `task_01`) of completed entries, in completion order. |
| `tasks.current` | string \| null | Stem of the task being worked on right now. Null between iterations. |
| `tasks.pending` | list[string] | Stems still to do, in execution order. |
| `progress.deliverables_complete` | bool | Set true by the LLM when it judges every techspec acceptance criterion met. Used only in `mode=free`. Phase B exits when this flips to true. |
| `progress.checklist[]` | list[obj] | Free-form checklist the LLM maintains in `mode=free`. Each entry: `text` (string), `status` (`pending`\|`in_progress`\|`completed`), `iteration` (int — the iteration that last touched it). Items only get added or status-flipped, never deleted. |
| `qa.report_done` | bool | True once the delegated `qa-report` worker's artifacts are verified. |
| `qa.execution_done` | bool | True once `qa-execution` produced its dated report. |
| `review.rounds` | int ≥ 0 | Count of closed `deep-review` rounds. Incremented by `update-state.py --review-round-done`. |
| `review.last_verdict` | `SHIP` \| `FIX_BEFORE_SHIP` \| `REWORK` \| null | Verdict of the last closed round. |
| `review.ship` | bool | True once a round closes with verdict `SHIP`. Phase E requires it. |
| `verify.last_run` | RFC3339 \| null | Last `make verify` execution. |
| `verify.last_status` | `PASS` \| `FAIL` \| null | Result of the final `make verify` observation for a completed action or proven external blocker. Intermediate repair-loop failures are not written. |
| `iterations[]` | list[obj] | Append-only log capped at the last 50 entries by `update-state.py`. Each entry: `n` (int), `timestamp` (RFC3339), `phase` (string), `action` (string), `outcome` (`completed`\|`partial`\|`blocked`), `memory_written` (list[string]), `blockers` (list[string]). `blocked` is reserved for the external-blocker test in `references/recovery-loop.md`. |

## Invariants

1. There is no top-level `current_phase`: `detect-phase.py` derives the next
   phase from `state.yaml` plus the filesystem every time. Phase labels live
   only in `iterations[].phase` as history.
2. `mode` is stable after bootstrap unless `update-state.py
   --reconcile-tasks` is used to rebuild `tasks.*` from task-file
   frontmatter after `_tasks.md` is authored later in the workflow. Do not
   hand-edit `mode`.
3. `frontend_agent` is written once at bootstrap and never mutated.
4. `progress.checklist[]` items move only forward
   (`pending → in_progress → completed`). Reverting requires a new entry.
5. `review.rounds` only increments; `review.ship` only flips false → true
   (via a `SHIP` round).
6. `iterations[]` is append-only; older entries get pruned by
   `update-state.py` once it exceeds 50, never edited.
7. Repair-loop failures do not append `iterations[]` entries or set
   `verify.last_status=FAIL`; only the phase's final PASS or a proven external
   blocker mutates those fields.
