# QA Run Report — 2026-09-16 — Worktree bulk removal

- Scope: issue 654, menubar and workspace overview selection, missing-record cleanup, partial outcomes and ownership.
- Cadence: targeted. Build: working branch `feat/issue-654-worktree-bulk-delete`.
- Started: 2026-09-16T17:06:37Z. Status: in-progress.
- Persona: Ada, project maintainer, desktop keyboard and 390px narrow viewport, local network, English UI.
- Charter: existing `CH-worktree-destructive-recovery`; bounded extension below covers the new selection workflow.
- Lab: isolated runtime and disposable Git repositories. No operator worktrees are targets.

## Session matrix

| Journey / scenario | Tour | Status | Evidence |
| --- | --- | --- | --- |
| Menubar mixed selection, per-item refusal and failed-only retry | Money / data preservation | Pass | Real daemon Playwright journey; two successes, one refusal, then failed-only retry |
| Overview mixed selection at narrow viewport | Obvious / keyboard | Pass | 390×844 rendered journey; Enter, Space, select-all and Escape |
| Missing-only cleanup and restore affordance on both surfaces | State transition | Pass | Two missing records dismissed; replacement file and direct ID history preserved |
| API profile rejection, stale state and retained history | Data preservation | Pass | Isolated HTTP and UDS: foreign-owner 404, repeated dismissal 204, dirty removal 409, retained ID read |
| Refresh/reconnect and matching active scope cleanup | State transition | Partial | Browser reload and fresh catalogs passed; matching scope has owning store coverage; final CI pending |

## Bounded charter

Ada wants to clear completed checkouts and entries whose directories disappeared while retaining a draft, an external checkout and histories. Enter each workspace list, select the represented eligible rows, review physical-removal versus record-dismissal counts, submit, inspect individual results, repair the disposable draft and retry only the failure. Use Escape to clear selection; leave editable shortcuts intact. Confirm results through a fresh public catalog and reload, and capture the narrow viewport.

## Verification ledger

- `SHELL=/bin/sh CGO_ENABLED=1 go test -race -tags=integration ./internal/worktree ./internal/api/core -run 'TestServiceRemoveAndRecover|TestRemoveWorktreeRefusal|TestWorktreeLifecycleIntegration'` passed with real Git. An initial inherited-shell bootstrap timeout was reproduced and passed with the fixture's isolated shell; tests were not weakened.
- Focused adoption and actual remove/dismiss handler suites passed, including concurrent restore/dismissal and foreign/archived ownership refusal.
- Root Turborepo focused selection, lifecycle and scope suites passed (81 tests); subsequent expanded selection/lifecycle coverage passed (54 tests in three owning files).
- Before the user's CI-only directive, frontend typecheck and build passed. A broad frontend run passed 7,321 tests and exposed one incomplete I/O mock; the mock was repaired and its owning suite passed. This is not claimed as a full-suite pass.
- Seven real daemon/Git browser journeys passed: singular force doorway, singular dismissal, singular restore, two mixed-list batches, and two missing-only batches. The overview missing-only test initially used Escape while the path tooltip owned focus; the explicit Cancel action repaired the test interaction and the isolated re-walk passed. Selection Escape behavior is independently exercised by both mixed journeys.
- Inspected menubar and 390px screenshots. Removed duplicate check marks and fixed the ineligible explanation width; both mixed rendered journeys passed again. Screenshot artifacts are produced by `web/e2e/__tests__/worktrees.spec.ts` in the standard Playwright output.
- Direct isolated-lab HTTP/UDS requests confirmed immutable profile ownership, idempotent dismissal, replacement-file preservation, dirty refusal, and readable dismissed IDs.
- Test-shape checker passed the modified worktree suites. Its whole-file core warning points to the unchanged `TestCreateSessionWorktreeBinding` function, outside this patch.
- Per the user's updated delivery instruction, heavy gates and React Doctor run in GitHub CI. No local `make gate` or `make gate-full` is claimed. Current-head CI, CodeRabbit, Greptile and React Doctor coverage remain delivery gates.

## Session debriefs

Selection gestures stayed in the list and never adopted the discovered checkout. Confirmation separated physical removal from metadata dismissal. The dirty target stayed actionable; retry did not repeat successful removal. A directory recreated at a missing path retained its file after dismissal, and the dismissed ID stayed readable while fresh catalogs excluded it.

Direct browser inspection through the desktop connection became unavailable; the rendered evidence comes from the existing real-daemon Playwright suite. No production operator worktrees, sessions, or host runtime were modified.

## Final status

Implementation and targeted verification are in progress toward delivery. Required current-head CI and completed external reviews are still pending. Do not interpret these targeted results as final delivery approval.
