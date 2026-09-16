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

## First review remediation

- React Doctor's compiler error and three warnings: removed the unsupported try/finally shape, parallelized independent checked targets, and extracted keyboard/row/tile presentation. The three owning Web suites passed again (54 tests); final React Doctor coverage runs in CI.
- Greptile compatibility finding: selector-free operator remove/dismiss calls infer the record owner at the API boundary. Explicit profile selectors, authenticated sessions and archived owners retain their restrictions. The owning API suite passed the legacy and refusal cases. OpenAPI and generated Web types describe the omission behavior.
- CodeRabbit minor and nitpick: singular confirmation title; composed native div props and keyboard handler on the toolbar.
- GitHub Go formatter findings: applied the pinned formatter only to changed files. The machine's unpinned shim lacked the formatter command, so the repository-pinned binary was used directly.
- A final catalog audit moved concurrent-state coverage to the owning discovery suite and verified that a dismissed winner cannot leak back into a stale listing. Focused discovery and API suites passed.

These are remediation records, not final CI/reviewer approval.

## Second review remediation

- React Doctor confirmed the compiler and serial-await fixes on `f29b16b26`; two complexity warnings remained. Menu-row rendering and the pure focus projection were further separated. The two rendered-component suites passed again (48 tests). Current-head React Doctor remains authoritative.
- The first CI race shard exposed an incomplete transport fixture: its worktree omitted the now-required persisted owner identity. The existing IT-033 fixture now carries the default owner and distinguishes public name resolution from the immutable mutation ID. HTTP/UDS success and exact refusal expectations are unchanged; the focused parity suite passed.
- Restarted only the registered disposable QA daemon. Its fresh public catalog still excluded the dismissed ID, direct inspection retained the dismissed history, and the file recreated at that path remained present. No operator daemon was restarted.

## Repository queue remediation

Greptile identified that worktree removals share one repository lock with eight waiting slots. The final batch runner chains each target after its predecessor's receipt, matching session mutation ordering and avoiding self-induced queue refusals. It retains the compiler-compatible promise finalization. A 12-target regression in the owning lifecycle hook suite simulates that queue bound and requires every eligible target to complete; all seven tests in that suite passed. New domain imports use the required `@/*` aliases. The earlier parallel-fan-out attempt is superseded by this bounded execution; final CI and reviewer coverage remain required.

## Selection-mode creation keyboard remediation

CodeRabbit found that the overview's creation footer could not activate by keyboard in selection mode. The footer now joins that mode's focus order, shows focus independently of the navigation cursor, and handles Enter/Space on the intended action. Activation does not bubble into the stale workspace/menu navigation cursor. The owning overview suite passed all 36 tests, including both keys and assertions that no workspace or worktree navigation occurred. The existing narrow-viewport mixed-batch E2E journey now opens and cancels creation from selection mode before exercising cleanup; its rendered focus screenshot and real app result require current-head CI verification.

CI uploads worktree screenshots even when the journeys pass, using the existing per-shard artifact convention and seven-day retention. The `worktree-cleanup-evidence-shard-*` artifacts contain menubar and narrow-overview selection, partial results, and overview creation focus. Final delivery evidence belongs to the PR's current-head CI and review record; this report preserves the targeted run and remediation history.
