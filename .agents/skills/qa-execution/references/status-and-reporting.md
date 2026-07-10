# Status and Reporting

The run's bookkeeping: one status enum, one incremental report per run, and the write-back that makes the living tree remember. The principle behind all of it: **the loop never closes silently on an open question** — every unresolved thing becomes a Blocked row, a registered bug, or a report section someone else can pick up.

## Contents

- The session status enum
- Report lifecycle
- Tracker write-back
- Round-close checklist
- Anti-patterns

## The session status enum

Every matrix row (one scenario within one session) carries exactly one of six values:

| Status | Meaning | Terminal? |
|---|---|---|
| `Pending` | Not yet walked in this run | no |
| `Pass` | Walked; expected observable confirmed per the evidence standard | yes |
| `Fixed` | Failed, fix applied under the governor, retest passed | yes |
| `Skipped` | Deliberately cut from this run; reasoning recorded | yes |
| `Blocked (needs human verify)` | Only completable by a human (real payment, external email/SMS, real OAuth account); exact instructions recorded | yes — waits on a person |
| `Blocked (human decision)` | Needs a product/design decision; escalated in Decisions for a Human | yes — waits on a person |

No other values, no prose statuses, no `Pass*`-with-asterisk. Nuance goes in the row's Issue link or the report narrative. A run is closeable only when zero rows are `Pending`.

Mapping to tracker enums (the qa-report state-schema reference, routed at Step 8 of the SKILL): `Pass` → `qa_status: pass`; `Fixed` → `pass` + `fix_status: fixed` + `retest_status: pass`; `Skipped` → `skipped`; the two Blocked → `blocked-verify` / `blocked-decision`; a failure not fixed this run → `fail` with `fix_status: pending`.

## Report lifecycle

One report per run at `<qa-docs-path>/reports/<YYYY-MM-DD>-<scope>.md` (`scope` = branch slug or cycle name, lowercased, non-alphanumerics collapsed to `-`). Template: `<qa-docs-path>/templates/report.md`, falling back to `assets/report-template.md`.

- **Created the moment the session matrix exists** (Step 2) — scope, personas, flows, matrix all rows `Pending`. Not at the end.
- **Updated incrementally**: after every session (row statuses, evidence, paper cuts) and after every fix (What Was Fixed, commit SHA). The on-disk report is the source of truth for resume — a run interrupted at session 3 of 7 restarts by reading the report, not by re-running sessions 1-2.
- **Never overwritten across runs.** A new run on the same scope gets a new date-file; the old report is history.
- **Final Status written last**, after the exit gate: ready / not ready / ready-with-blocked-items, with totals by impact tier and the gate result verbatim.

## Tracker write-back

After each session (not batched at the end):

1. Settled scenarios: `qa_status` per the mapping above; `evidence` paths; `last_report` set to this report.
2. Failures: `bug_ids` linked (registry updated first — dedup before mint).
3. Fixes: `fix_status`, `fix_commits`, `retest_status` per the fix loop.
4. Charter debrief appended to the charter file with this report's path.
5. Rows sorted by id on save (merge convention).

## Round-close checklist

Run before writing Final Status; every unmet item is either fixed or disclosed in Final Status — never silently dropped:

1. **Matrix terminal** — zero `Pending` rows; every `Skipped` has reasoning; every Blocked row carries its human instructions or escalation.
2. **Coverage honest** — every in-scope journey was walked by its assigned persona, or its absence is disclosed. No coverage claims without a session behind them.
3. **Bugs consistent** — every `Fail`/`Fixed` row links a registered `BUG-NNNN`; bug statuses match reality; no orphan bugs unlinked from the tracker.
4. **Fixes proven** — every fix has commit SHA + regression test (or documented replay + backlog entry); impacted and adjacent journeys re-walked.
5. **Tracker written** — `state.csv` verdicts current, sorted, enums valid; charter debriefs appended.
6. **Evidence lean and linked** — checkpoints/failures captured, paths resolve, oversized evidence pruned per the layout policy.
7. **Exit gate run** — full automated suite result recorded verbatim.
8. **Parity disclosed** — any production-parity deviation (mocked service, missing extension set, wifi-only) stated, since it qualifies every verdict.
9. **Fidelity clean** — no evaluator framing leaked into product surfaces; any violation disclosed with the re-run that replaced the tainted verdicts.
10. **Final Status states the release readiness** in one sentence a non-reader can act on, with totals by user-impact tier.

## Anti-patterns

- **Report written at the end** — an interrupted run loses everything; the report exists before the first session or the run has no memory.
- **Prose statuses** — "passed but flaky on retry" is not a status; it's a `Pass` with an Issue link or a `Fail`, decided honestly.
- **Batched write-back** — updating the tracker "later" is how rounds end without updating it at all.
- **Green-by-omission** — an empty report section reads as "nothing found"; it must read as "not run", explicitly.
- **New report file per resume** — resuming a run continues its existing dated report; only a new run gets a new file.
