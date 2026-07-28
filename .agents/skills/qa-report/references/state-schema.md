# State Schema — Scenario Files

`<qa-docs-path>/scenarios/` is the living scenario tracker: one file per scenario, flat frontmatter, answering "what does this product promise its users, and what state is each promise in?" It replaces per-round test-case trees as the cross-cycle memory.

One file per scenario is also what keeps the tree **merge-safe** when parallel branches run QA: writers touching different scenarios never meet in a diff, and two branches settling the *same* scenario surface as a small, readable conflict in one file instead of a corrupted shared table.

## Contents

- File format
- Field schema
- Status enums
- Id minting
- Update rules
- The tracker view (`state.csv`)
- Merge behavior
- Anti-patterns

## File format

One file per **scenario** — a persona-visible behavior derived from a journey flow (Step 3 of the SKILL), not a feature, not a test case — at `<qa-docs-path>/scenarios/<id>.md` (template: `<qa-docs-path>/templates/scenario.md`, seed: `assets/scenario-template.md`):

```markdown
---
id: CHK-first-purchase-happy
area: CHK
title: Complete first purchase end-to-end
persona: New User
journey: J-first-purchase
expected: Order confirmation visible, receipt email cites the same order id, order appears in history on fresh load
entry_points: https://app.example.com/; email-invite-link
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

Free-prose notes live here and only here.
```

The discipline that keeps the directory greppable and merges clean:

- **Flat frontmatter, one field per line** — no nested YAML, no wrapped lines, fixed field order (as above). Git then auto-merges edits to *different fields* of the same scenario.
- Every field is an enum, an id, a path, or a single sentence with a fixed role; lists are semicolon-separated. **All prose lives in the body.** This is what keeps the tracker queryable — "all failing scenarios" must be one grep (`grep -l 'qa_status: fail' scenarios/`), not an interpretation exercise.

## Field schema

| # | Field | Content | Discipline |
|---|---|---|---|
| 1 | `id` | `<AREA>-<slug>` stable scenario id (= filename) | Content-addressed; never renamed |
| 2 | `area` | Product area code (defined in `<qa-docs-path>/README.md`) | Closed set per project |
| 3 | `title` | Short scenario name (verb-first) | ≤ 80 chars |
| 4 | `persona` | Primary persona (from `<qa-docs-path>/personas.md`) | Exact persona name |
| 5 | `journey` | Journey file id (`J-<slug>`) | Must exist in `journeys/` |
| 6 | `expected` | The observable that proves success, in user language | One sentence |
| 7 | `entry_points` | How a user reaches it (URL, CLI verb, deep link) | Semicolon-separated |
| 8 | `qa_status` | Enum (below) | Enum only — no prose |
| 9 | `bug_ids` | Linked bugs | Registry ids semicolon-separated, or empty |
| 10 | `fix_status` | Enum (below) | Enum only |
| 11 | `retest_status` | Enum (below) | Enum only |
| 12 | `fix_commits` | Commit SHAs of fixes | Semicolon-separated short SHAs |
| 13 | `evidence` | Paths to evidence for the latest verdict | Semicolon-separated paths |
| 14 | `last_report` | Path of the report that produced the latest verdict | One path |
| 15 | `overlaps` | Scenario ids this file overlaps, with the canonical owner first | Ids only |
| 16 | *(body)* | Free-prose notes | The ONLY free-prose location |

## Status enums

`qa_status` — the scenario's current verdict:

| Value | Meaning |
|---|---|
| `untested` | Planned; no session has walked it yet (or the surface changed since the last verdict) |
| `pass` | Walked by the assigned persona; expected observable confirmed with evidence |
| `fail` | Walked; expected observable not met; `bug_ids` MUST be non-empty |
| `blocked-verify` | Only completable by a human (real payment, external email, SMS, OAuth with real account) |
| `blocked-decision` | Needs a human product/design decision before it can pass (see fix-loop governor) |
| `skipped` | Deliberately out of this cycle's scope; reasoning in the body |

`fix_status` — only meaningful when `bug_ids` is non-empty:

| Value | Meaning |
|---|---|
| *(empty)* | No fix needed |
| `pending` | Bug filed, fix not yet applied |
| `fixed` | Fix applied; `fix_commits` MUST carry the SHA |
| `deferred` | Consciously postponed; reasoning in the body |

`retest_status` — only meaningful when `fix_status` is `fixed`:

| Value | Meaning |
|---|---|
| *(empty)* | No retest needed |
| `pending` | Fixed but not yet re-walked |
| `pass` | Re-walked under the same persona/journey; observable confirmed |
| `fail` | Regressed on retest; reopen the bug |

A scenario is only *done* for a cycle when `qa_status` is terminal (`pass`/`blocked-*`/`skipped`) AND any `fixed` bug has `retest_status: pass`.

## Id minting

- `id` = `<AREA>-<slug>`. Area codes are 2-4 uppercase letters, defined once in `<qa-docs-path>/README.md` (e.g. `AUTH`, `CHK`, `SET`). Adding an area updates the README first. Slug: 2-5 kebab-case words naming the behavior from the persona's side (`signup-empty-email`, `first-purchase-happy`).
- The id is **content-addressed** — derived from the behavior, never from a counter. Nothing reads "the highest existing number", so parallel branches cannot collide on minting; two planners describing the same behavior mint the same id, which is dedup working, not a collision.
- Ids are stable forever: never renamed, never reworded. Retiring a scenario means `qa_status: skipped` with `retired — <reason>` in the body — the file stays as memory.
- Grandfathered counter ids from an adopted v1 tree (`LP-003`-style) stay valid as-is — the content-addressed format governs new minting only; rewording an old id breaks every report, charter, and bug that cites it.
- When two scenarios test the same behavior from different angles, keep both files and cross-link via `overlaps`, naming the canonical owner first. Dedup is explicit, not silent.

## Update rules

- `qa-report` creates files and owns the **planning fields** (1-7, 15) plus the body.
- `qa-execution` owns the **verdict fields** (8-14) plus the body, updated after sessions.
- Update in place. Never fork a second file to record a new verdict — the new verdict replaces the old one; history lives in the dated reports (`last_report` points at the latest).
- When a surface changes (new diff touches a scenario's journey), the executor or planner resets `qa_status` to `untested` — a stale `pass` is worse than no verdict.
- **Implementers flag between cycles.** Whoever lands a user-visible behavior change (UI, CLI verb, API route, config key, copy) — including agents completing non-QA tasks — applies the flag as part of task completion: new behavior → add `untested` scenario files; changed behavior → reset the affected files to `untested`; pure refactors declare "no user-visible change". Flag, don't retest — the next cycle's targeted tier picks up `untested` scenarios as its scope. This is what keeps the tracker alive between QA cycles.

## The tracker view (`state.csv`)

The flat all-scenarios table is a **materialized view**, generated from the frontmatters — never edited by hand, never committed (the bootstrap gitignore block covers it; see the layout reference):

```bash
python3 <qa-report-skill-dir>/scripts/materialize_state.py <qa-docs-path>
# writes <qa-docs-path>/state.csv — 16 columns, sorted by id
```

The script mutates only the generated view; scenario files are read, never written. A file it rejects (stderr names the file and the violation) is repaired per the SKILL's error handling, then the run is repeated.

Regenerate it whenever a human wants the spreadsheet or a step needs the whole table at once; day-to-day queries are greps on `scenarios/`.

## Merge behavior

Why this format merges clean, and the one case that still needs a hand:

- **Different scenarios** → different files → no contention, ever.
- **Same scenario, different fields** (one branch sets `qa_status`, the other edits the body) → git auto-merges: one field per line.
- **Same scenario, same field** (two branches settled the same verdict) → a one-file conflict; keep the field values whose `last_report` is most recent, and keep both reports' history.
- **Same behavior, two slugs** (parallel planners worded it differently) → not a git conflict, so it must be hunted: the post-merge reconciliation in Step 1 of the SKILL folds the newer file into the older id, merges verdicts by `last_report` recency, updates references, and deletes the duplicate.

## Anti-patterns

- **Prose statuses** — `qa_status: "Passed after retest"` makes the tracker unqueryable. The enum is `pass`; the story goes in the body and the bug file.
- **Near-duplicate enum spellings** — `No error` vs `No product error`. The enum list above is closed; anything else is a schema violation to fix on sight.
- **Packed fields** — stuffing bug ids, commit SHAs, and evidence paths into one field. Fields 9, 12, 13 exist precisely to keep them separately greppable.
- **File-per-round** — a new file for each cycle's verdict on the same scenario. One scenario = one file, forever.
- **Counter ids** — `<AREA>-013`-style sequence numbers reintroduce the shared counter that makes parallel branches collide; the slug is the id.
- **Editing or committing the view** — a hand-edited or committed `state.csv` competes with the scenario files as a second source of truth. The files are the source; the view is disposable output.
- **Tracker as report** — the frontmatter answers "what state is X in?"; the *why and how* live in bug files and dated reports. Don't grow scenario bodies into narratives.
