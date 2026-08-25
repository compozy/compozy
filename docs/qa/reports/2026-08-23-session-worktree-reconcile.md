# QA Run Report — 2026-08-23 — session-worktree-reconcile

- **Scope:** Daemon boot reconciliation preserves the worktree binding of an existing session.
- **Cadence tier:** targeted
- **Build:** `df7e80e4-dirty` · **Environment:** isolated local runtime lab
- **Started:** 2026-08-23T00:24:43-03:00 · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | isolated CLI workspace | local CLI / local network / pt-BR | CH-worktree-binding-containment |

## Flows in Scope

- `J-worktree-management` — create and resume a session bound to a ready worktree (`../journeys/J-worktree-management.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-worktree-binding-containment | J-worktree-management / RT-session-worktree-lifecycle | Ada | targeted CLI/runtime restart | Pass | [#455](https://github.com/compozy/compozy/issues/455) | uncommitted |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-worktree-binding-containment — Ada

- **Ran:** 2026-08-23T00:26:19-03:00 → 2026-08-23T00:27:52-03:00 (box respected: yes)
- **Findings:** The daemon restarted with one recovered session and no reconciliation error.
- **Bugs filed/updated:** [#455](https://github.com/compozy/compozy/issues/455)
- **Scenarios settled:** RT-session-worktree-lifecycle → pass
- **Paper cuts:** None.
- **Surprises:** The first explicit worktree path overlapped the registered workspace and was correctly rejected; the default managed path succeeded.
- **Suggested next charter:** No adjacent journey is required for this narrow reconstruction fix.

## What Was Fixed

### #455: Daemon boot fails while reconciling a worktree-bound session

- **Symptom:** The daemon refuses to start with a session participation mismatch.
- **Root cause:** Boot reconciliation rebuilt the durable session projection without its persisted worktree ID.
- **Fix:** Preserve the worktree ID when reconstructing `SessionInfo` from `meta.json`.
- **Regression test:** `internal/observe/reconcile_test.go` — failed before the fix and passes after it.
- **Retested:** A fresh isolated CLI/runtime walk stopped the bound session, restarted the daemon, then confirmed the same session and worktree ID through filtered list and independent status reads.

## Paper Cuts

None observed.

## Runtime Errors Observed

The initial explicit worktree path was rejected with `worktree_path_exists` because it overlapped the workspace root. This was expected containment behavior and the managed worktree path was used.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- Durable session metadata and the global projection must preserve immutable ownership fields through every reconstruction path.

## Final Status

- **Exit gate evidence:** The fingerprint-matched record in `.cache/gate/full.json` is authoritative. Historical failed runs are preserved at `.cache/gate/logs/full-1787455830.log` and `.cache/gate/logs/full-1787458643.log`; they identified Btrfs temporary-file semantics, an invalid external `/tmp/.git`, and stale extension rollback assertions after lifecycle ownership tokens were introduced. The changed `internal/observe` package passed in both runs.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 1/1 journeys walked; isolated CLI and runtime surfaces passed. Strict QA audit passed with zero blockers and zero warnings.
- **Verdict:** the targeted fix and runtime walk pass, and teardown evidence reports `clean: true` with no survivors. Repository merge readiness follows the current fingerprint-matched full-gate record.
