# Bug Registry

`<qa-docs-path>/bugs/` is the global, durable bug registry. Bugs outlive the QA rounds that found them — a bug's history (found, fixed, regressed, re-found) is diagnostic knowledge the project keeps forever, keyed by one stable id. This file is the canonical severity model for both `qa-report` and `qa-execution`.

## Contents

- Id minting
- Dedup before filing
- Bug statuses
- The five user-impact tiers (canonical severity model)
- Mapping to technical severity
- Required fields
- Anti-patterns

## Id minting

- Ids are `BUG-<YYYYMMDD>-<slug>` — the date the bug was found plus 2-5 kebab-case words naming the symptom from the user's side (e.g. `BUG-20260711-cart-lost-on-refresh`). Global and stable for the life of the project; the date prefix keeps the registry chronologically sorted.
- The id is **content-addressed** — minted from the symptom, never from a counter. Nothing reads "the highest existing number", so parallel branches cannot collide on minting; two branches filing the same symptom on the same day mint the same id, and that add/add merge conflict *is* the dedup step surfacing itself — fold the two files into one.
- Ids never reset per round, per area, or per release, and are never renamed after filing — even when the symptom's understanding evolves; the title line carries the current wording.
- Adopted bugs keep unique v1 ids as-is (grandfathered — the format governs new minting only); only bugs from colliding per-round registries get fresh ids, dated by original discovery when known, with an `Origin:` field pointing at the old artifact and the old id recorded as an alias, never reused.

## Dedup before filing

Before minting, search the registry for the symptom:

1. Grep `<qa-docs-path>/bugs/` for the observable (error copy, journey step, affected element).
2. Check the affected scenario file's `bug_ids` — a scenario that failed before likely links the prior bug.
3. **Same symptom, bug not `verified`** → update the existing file (append a `## Re-found` section with date, persona, report path). Re-finding is signal about persistence, not a new bug.
4. **Same symptom, bug `verified` (fix confirmed)** → the bug regressed: reopen it (status back to `open`, append a `## Regressed` section). A regression on the same id is far more informative than a fresh id.
5. Only mint a new id when the symptom is genuinely new.

## Bug statuses

| Status | Meaning |
|---|---|
| `open` | Filed; no fix applied |
| `fixed` | Fix applied (commit SHA recorded); not yet re-verified under the original persona/journey |
| `verified` | Re-walked under the original persona/journey; observable confirmed fixed |
| `wont-fix` | Consciously declined with recorded reasoning (usually a human decision) |
| `invalid` | Not a product bug (tester error, environment artifact) — kept for the record, reasoning required |

The status lives in the bug file's frontmatter/header and is mirrored into the linked scenario files via `fix_status`/`retest_status`.

## The five user-impact tiers

Classify by what the bug does to a real person, not to the technology. A "critical" exception in an admin log can matter less than a "minor" copy bug on the checkout button. Pick exactly one tier; when in doubt between two, pick the higher.

### Blocks-Completion
- **Definition:** a user on a value-delivering journey cannot complete it — they give up or work around into incorrect state.
- **Examples:** submit does nothing on the only payment screen; login fails for valid credentials; "Save" succeeds visually but data is gone on reload.
- **Release impact:** open Blocks-Completion on a P0 journey blocks the release.

### Data-Loss
- **Definition:** user data (entered, uploaded, configured) is destroyed, corrupted, or made inaccessible without consent — often without the user noticing.
- **Examples:** form silently drops a field; upload reports success but the file is empty; settings revert on save.
- **Release impact:** always a release blocker. Silent loss is worse than visible failure — users can't recover from what they don't notice.

### Trust-Damage
- **Definition:** nothing is technically broken, but the user's confidence erodes — they wonder if the product can be relied on.
- **Examples:** confirmation email cites a different order id than the UI; error copy with no next step; "last saved" showing a future timestamp; screen reader announcing "image image image".
- **Release impact:** several Trust-Damage findings on the same journey block a release even when no single one does.

### Friction
- **Definition:** the journey completes, but with extra effort, confusion, or repetition.
- **Examples:** re-entering data already provided; hover-only controls on touch; 3-second silence before button feedback; validation only on submit.
- **Release impact:** not individually blocking. Repeated Friction in one area is a redesign signal — track the pattern.

### Cosmetic
- **Definition:** visual/wording issues that affect neither completion nor trust.
- **Examples:** tooltip typo; 2px icon misalignment; off-brand hover color.
- **Release impact:** never blocking; batch into polish work. Cosmetic on a hero/first-impression surface is at least Friction — re-classify.

## Mapping to technical severity

Keep the legacy `Severity`/`Priority` fields for tooling continuity:

| Impact tier | Default Severity | Default Priority | Override when… |
|---|---|---|---|
| Blocks-Completion | Critical | P0 | Blocked journey is low-priority → High |
| Data-Loss | Critical | P0 | Data reproducible from another source → High |
| Trust-Damage | High | P1 | Auto-corrected without user effort → Medium |
| Friction | Medium | P2 | Friction sits on a P0 journey → High |
| Cosmetic | Low | P3 | On hero/onboarding surface → Medium |

User-impact totals drive the release go/no-go conversation; severity totals drive engineering triage.

## Required fields

Every bug file (template: `<qa-docs-path>/templates/bug.md`, seed: `assets/bug-template.md`) carries:

- `Impact (user-side):` — one of the five tiers.
- `Persona Affected:` + `Journey Step:` — who is hurt, and when. These two fields let a product owner read the queue without opening the bug.
- `Reproduction:` — exact steps from the persona's entry point, with the charter id (`CH-<slug>`) and tour named.
- `Evidence:` — screenshot/report paths proving the observable.
- `Scenarios:` — the scenario ids this bug affects (kept in sync with their `bug_ids`).
- After a fix: `Fix commit:` (SHA) and `Regression test:` (the test that failed before and passes after — or the documented replay and the stated reason no automated test is meaningful).

## Anti-patterns

- **Counter ids** — `BUG-001` restarting each cycle destroys cross-round tracking, and a global monotonic counter makes parallel branches mint the same number and collide on merge. Both are the same mistake: identity from a sequence instead of from the symptom.
- **Duplicate filing** — a re-found symptom filed under a new id splits the history that makes persistent bugs visible.
- **"Critical" inflation** — reserve Critical for actual Blocks-Completion/Data-Loss, or the scale stops meaning anything.
- **Technical framing** — "API returns 500" is an observation; the bug is "a returning user loses their cart at checkout step 3". Lead with the user, cite the technical detail in reproduction.
- **Fix without regression proof** — `fixed` status with no commit SHA and no regression test/replay is a claim, not a fix.
