# State Schema — `state.csv`

`<qa-docs-path>/state.csv` is the living scenario tracker: one flat, greppable, diffable surface answering "what does this product promise its users, and what state is each promise in?" It replaces per-round test-case trees as the cross-cycle memory.

## Contents

- Column schema
- Status enums
- Id minting
- Update rules
- Merge convention
- Anti-patterns

## Column schema

One row per **scenario** — a persona-visible behavior derived from a journey flow (Step 3 of the SKILL), not a feature, not a test case.

| # | Column | Content | Discipline |
|---|---|---|---|
| 1 | `id` | `<AREA>-NNN` stable scenario id | Never reused, never renumbered |
| 2 | `area` | Product area code (defined in `<qa-docs-path>/README.md`) | Closed set per project |
| 3 | `title` | Short scenario name (verb-first) | ≤ 80 chars |
| 4 | `persona` | Primary persona (from `<qa-docs-path>/personas.md`) | Exact persona name |
| 5 | `journey` | Journey file id (`J-NN`) | Must exist in `journeys/` |
| 6 | `expected` | The observable that proves success, in user language | One sentence |
| 7 | `entry_points` | How a user reaches it (URL, CLI verb, deep link) | Semicolon-separated |
| 8 | `qa_status` | Enum (below) | Enum only — no prose |
| 9 | `bug_ids` | Linked bugs | `BUG-NNNN` semicolon-separated, or empty |
| 10 | `fix_status` | Enum (below) | Enum only |
| 11 | `retest_status` | Enum (below) | Enum only |
| 12 | `fix_commits` | Commit SHAs of fixes | Semicolon-separated short SHAs |
| 13 | `evidence` | Paths to evidence for the latest verdict | Semicolon-separated paths |
| 14 | `last_report` | Path of the report that produced the latest verdict | One path |
| 15 | `overlaps` | Scenario ids this row overlaps, with the canonical owner first | Ids only (e.g. `SET-004;CHK-011`) |
| 16 | `notes` | Free text | The ONLY free-prose column |

**All prose lives in `notes`.** Every other column is an enum, an id, a path, or a single sentence with a fixed role. This is what keeps the tracker queryable — "all failing scenarios" must be one grep, not an interpretation exercise.

## Status enums

`qa_status` — the scenario's current verdict:

| Value | Meaning |
|---|---|
| `untested` | Planned; no session has walked it yet (or the surface changed since the last verdict) |
| `pass` | Walked by the assigned persona; expected observable confirmed with evidence |
| `fail` | Walked; expected observable not met; `bug_ids` MUST be non-empty |
| `blocked-verify` | Only completable by a human (real payment, external email, SMS, OAuth with real account) |
| `blocked-decision` | Needs a human product/design decision before it can pass (see fix-loop governor) |
| `skipped` | Deliberately out of this cycle's scope; reasoning in `notes` |

`fix_status` — only meaningful when `bug_ids` is non-empty:

| Value | Meaning |
|---|---|
| *(empty)* | No fix needed |
| `pending` | Bug filed, fix not yet applied |
| `fixed` | Fix applied; `fix_commits` MUST carry the SHA |
| `deferred` | Consciously postponed; reasoning in `notes` |

`retest_status` — only meaningful when `fix_status` is `fixed`:

| Value | Meaning |
|---|---|
| *(empty)* | No retest needed |
| `pending` | Fixed but not yet re-walked |
| `pass` | Re-walked under the same persona/journey; observable confirmed |
| `fail` | Regressed on retest; reopen the bug |

A scenario is only *done* for a cycle when `qa_status` is terminal (`pass`/`blocked-*`/`skipped`) AND any `fixed` bug has `retest_status: pass`.

## Id minting

- `id` = `<AREA>-NNN`. Area codes are 2-4 uppercase letters, defined once in `<qa-docs-path>/README.md` (e.g. `AUTH`, `CHK`, `SET`). Adding an area updates the README first.
- `NNN` is zero-padded, monotonic **per area**: next id = max existing for that area + 1. Ids of retired scenarios are never reused — mark the row `skipped` with `notes: retired — <reason>` instead of deleting it, or delete the row and never reissue the number (record deletions in the README changelog).
- When two scenarios test the same behavior from different angles, keep both rows and cross-link via `overlaps`, naming the canonical owner first. Dedup is explicit, not silent.

## Update rules

- `qa-report` creates rows (planning) and may update columns 1-7, 15, 16.
- `qa-execution` updates columns 8-14 after sessions, plus `notes`.
- Update in place. Never append a duplicate row to record a new verdict — the new verdict replaces the old one; history lives in the dated reports (`last_report` points at the latest).
- When a surface changes (new diff touches a scenario's journey), the executor or planner resets `qa_status` to `untested` — a stale `pass` is worse than no verdict.
- **Implementers flag between cycles.** Whoever lands a user-visible behavior change (UI, CLI verb, API route, config key, copy) — including agents completing non-QA tasks — applies the flag as part of task completion: new behavior → add `untested` rows; changed behavior → reset the affected rows to `untested`; pure refactors declare "no user-visible change". Flag, don't retest — the next cycle's targeted tier picks up `untested` rows as its scope. This is what keeps the tracker alive between QA cycles.

## Merge convention

The CSV is shared state; these rules keep concurrent edits mergeable:

- One row per line; no wrapped lines. Fields containing commas are double-quoted (standard CSV).
- Rows sorted by `id` — every writer re-sorts before saving so diffs are positional and conflicts stay row-local.
- Never reorder columns; never hand-align widths.
- Concurrent QA runs partition by `area` (or by journey) so no two writers touch the same rows. If a conflict happens anyway, resolve per-row by recency of `last_report`.

## Anti-patterns

- **Prose statuses** — `qa_status: "Passed after BUG-003 retest"` makes the tracker unqueryable. The enum is `pass`; the story goes in `notes` and the bug file.
- **Near-duplicate enum spellings** — `No error` vs `No product error`, `Not required` vs `Not needed`. The enum list above is closed; anything else is a schema violation to fix on sight.
- **Packed cells** — stuffing bug ids, commit SHAs, and evidence paths into one field. Columns 9, 12, 13 exist precisely to keep them separately greppable.
- **Row-per-round** — appending a new row for each cycle's verdict on the same scenario. One scenario = one row, forever.
- **Tracker as report** — the CSV answers "what state is X in?"; the *why and how* live in bug files and dated reports. Don't grow the CSV into a narrative.
