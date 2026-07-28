# state.yaml — Schema Reference

Authoritative schema for `.compozy/tasks/<slug>/state.yaml`. The file is the
orchestration layer that lets `cy-implement-spec` continue across iterations
and resume mid-flight if a session ends. The file is mutated **only** by:

- `.agents/skills/cy-implement-spec/scripts/init-state.py` (bootstrap)
- `.agents/skills/cy-implement-spec/scripts/update-state.py` (every iteration)

No other writer is permitted. Hand-editing voids resume guarantees.

## Field reference

| Field | Type | Meaning |
|-------|------|---------|
| `slug` | string | Directory name under `.compozy/tasks/`. Mirrors the value passed at bootstrap. |
| `created_at` | RFC3339 UTC | When `init-state.py` first wrote the file. |
| `last_updated` | RFC3339 UTC | When `update-state.py` last touched the file. |
| `iteration` | int ≥ 0 | Monotonic counter. `update-state.py` increments it once per call. |
| `goal_signature` | string | Verbatim text from the user's `[[CODEX_LOOP goal="..."]]` header (or manual invocation reason). Read-only after bootstrap. |
| `progress.implementation_complete` | bool | True only once every criterion is `met` on a verify-PASS tree. Set exclusively via `update-state.py --implementation-complete`, which enforces both conditions. Phase B exits when this flips to true. |
| `progress.criteria[]` | list[obj] | The spec's exit conditions, registered at bootstrap from the spec documents. Each entry: `text` (string, unique), `status` (`pending`\|`met`), `iteration` (int — the iteration that last touched it). Criteria are **exit conditions, not work items**: the agent owns the work breakdown in its own context; this list only tracks which conditions are proven. Entries are appended (`--add-criterion`, spec amendments only) or flipped to `met` (`--criterion-met`), never deleted or reverted. |
| `qa.report_done` | bool | True once the delegated `qa-report` worker's artifacts are verified. |
| `qa.execution_done` | bool | True once `qa-execution` produced its dated report. |
| `review.rounds` | int ≥ 0 | Count of closed `deep-review` rounds. Incremented by `update-state.py --review-round-done`. |
| `review.last_verdict` | `SHIP` \| `FIX_BEFORE_SHIP` \| `REWORK` \| null | Verdict of the last closed round. |
| `review.ship` | bool | True once a round closes with verdict `SHIP`. Phase E requires it. |
| `verify.last_run` | RFC3339 \| null | Last verification gate execution. |
| `verify.last_status` | `PASS` \| `FAIL` \| null | Result of the final verification observation for a completed action or proven external blocker. Intermediate repair-loop failures are not written. |
| `iterations[]` | list[obj] | Append-only log capped at the last 50 entries by `update-state.py`. Each entry: `n` (int), `timestamp` (RFC3339), `phase` (string), `action` (string), `outcome` (`completed`\|`partial`\|`blocked`), `memory_written` (list[string]), `blockers` (list[string]). `blocked` is reserved for the external-blocker test in `references/recovery-loop.md`. |

## Invariants

1. There is no top-level `current_phase`: `detect-phase.py` derives the next
   phase from `state.yaml` every run. Phase labels live only in
   `iterations[].phase` as history.
2. There is no task ledger. A slug carrying `_tasks.md` or `task_*.md` is
   refused at bootstrap (`init-state.py` exit 5) — that slug belongs to
   `cy-loop-tasks`.
3. `progress.criteria[]` is never empty after bootstrap (`init-state.py`
   requires ≥ 1 `--criterion`) and entries move only forward
   (`pending → met`). A criterion that turns out to be wrong is superseded
   by a new `--add-criterion` entry plus a memory note, never edited.
4. `progress.implementation_complete` only flips false → true, and only in
   an `update-state.py` call that also carries `--verify-pass` with zero
   pending criteria. The script refuses anything else.
5. `review.rounds` only increments; `review.ship` only flips false → true
   (via a `SHIP` round).
6. `iterations[]` is append-only; older entries get pruned by
   `update-state.py` once it exceeds 50, never edited.
7. Repair-loop failures do not append `iterations[]` entries or set
   `verify.last_status=FAIL`; only the phase's final PASS or a proven
   external blocker mutates those fields.
