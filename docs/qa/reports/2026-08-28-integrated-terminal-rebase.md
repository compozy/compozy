# QA Run Report — 2026-08-28 — integrated-terminal-rebase

- **Scope:** PR #490 after rebasing onto PRs #498 and #494; re-walk the terminal stream, approval, profile-segmentation, and agent handoff contracts across Web, CLI, HTTP API, and runtime, with session-deletion, model-catalog, palette-dismissal, and profile-editor canaries for the overlapping rebase changes.
- **Cadence tier:** targeted
- **Build:** local remediation tree based on rebased head `bfbeb18da` · **Environment:** isolated labs `integrated-terminal-rebase-20260828-201516-678087` and `integrated-terminal-profile-retest-20260829-172042-776889`
- **Started:** 2026-08-28T20:17:33Z · **Status:** QA passed; delivery gates pending

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Marina | isolated terminal workspace | desktop / flaky + wifi-fast / en-US | CH-terminal-stream-flow-control, CH-terminal-lease-fencing-takeover |
| Bruno | isolated terminal workspace | desktop / wifi-fast / en-US | CH-terminal-approval-ladder |
| Théo | isolated terminal workspace | desktop / wifi-fast / en-US | CH-session-empty-and-dock-last-created |
| Sol | isolated terminal workspace | desktop / wifi-fast / en-US | CH-runtime-ui-regression-model-catalog |
| Ada | isolated two-profile terminal workspace | desktop / wifi-fast / en-US | CH-terminal-profile-fence |

## Flows in Scope

- `J-operate-integrated-terminal` — operate and recover a terminal through Web, CLI, API, and runtime (`../journeys/J-operate-integrated-terminal.md`)
- `J-supervise-agent-terminal` — supervise terminal work while preserving approval, control, and redaction boundaries (`../journeys/J-supervise-agent-terminal.md`)
- `J-15` — delete a focused session without destroying its window (`../journeys/J-15-operate-session-via-cli-api.md`)
- `J-17` — open and use the runtime selector from a cold catalog (`../journeys/J-17-session-create-unified-selector.md`)
- `J-switch-profile-terminal-scope` — switch profiles while every Terminal surface stays owner-fenced (`../journeys/J-switch-profile-terminal-scope.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-terminal-stream-flow-control | J-operate-integrated-terminal / ET-terminal-stream-resilience | Marina | Multi-Tab Tour | Pass | — | pending |
| 2 | CH-terminal-approval-ladder | J-supervise-agent-terminal / ET-terminal-approval-ladder-grants | Bruno | Feature Tour | Pass | — | pending |
| 3 | CH-terminal-lease-fencing-takeover | J-supervise-agent-terminal / ET-terminal-agent-handoff-input | Marina | Interrupt Tour | Pass | — | pending |
| 4 | CH-session-empty-and-dock-last-created | J-15 / RT-014 | Théo | Feature Tour | Pass | — | pending |
| 5 | CH-session-empty-and-dock-last-created | J-15 / RT-session-delete-keeps-empty-tab | Théo | Feature Tour | Pass | — | pending |
| 6 | CH-session-empty-and-dock-last-created | J-15 / ET-web-dock-contextual-session-launch | Théo | Feature Tour | Pass | — | pending |
| 7 | CH-runtime-ui-regression-model-catalog | J-17 / RT-model-catalog-cold-open | Sol | Feature Tour | Pass | — | pending |
| 8 | CH-terminal-profile-fence | J-switch-profile-terminal-scope / ET-terminal-profile-segmentation | Ada | Garbage Tour | Pass | BUG-20260829-terminal-journal-unlock-remount; BUG-20260829-workspace-delete-visible-terminal-deadlock | pending |
| 9 | CH-terminal-profile-fence | J-operate-desktop-shell / ET-web-command-palette-shortcuts | Bruno | Feature Tour | Pass | BUG-20260829-command-palette-portal-remains-mounted | pending |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

- Marina watched a CLI-owned terminal in Web, took control explicitly, typed `echo WEB_TYPED`, and verified the same output and exit status independently through the CLI quote surface. The multi-viewer, backpressure, replay, resize, and reconnect contracts also passed the runtime E2E lane.
- Bruno's approval ladder, typing-grant scope, revocation, and irreversible-command refusals passed the runtime E2E lane. No hidden input appeared in the visible transcript.
- Théo deleted a focused session from Web, stayed in the same empty session tab, and confirmed the independent API read returned 404. Dock/session creation behavior passed its browser E2E canaries.
- Sol opened the model selector from the isolated onboarding surface. Persisted rows appeared without a manual reload, and the focused catalog suite passed with one coalesced background refresh per source.
- Ada created terminal and journal work under default and profile B, proved default/profile/aggregate
  scope independently through Web, CLI, HTTP, and UDS, saw exact owner labels, archived profile B with
  retained history, and removed the workspace while a visible exec was active.
- Bruno opened Terminal through the current command palette bundle and verified the shared portal and
  overlay both unmounted before interacting with the focused destination.

Primary evidence:

- `/Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-rebase-20260828-201516-678087-lab/qa-artifacts/qa/journey-log.jsonl`
- `/Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-rebase-20260828-201516-678087-lab/qa-artifacts/qa/logs/web-terminal-cross-surface.md`
- `/Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-rebase-20260828-201516-678087-lab/qa-artifacts/qa/logs/test-e2e-runtime-after-fix.log`
- `/Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-rebase-20260828-201516-678087-lab/qa-artifacts/qa/screenshots/session-delete-empty-tab.png`
- `/Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-rebase-20260828-201516-678087-lab/qa-artifacts/qa/screenshots/onboarding-runtime.png`
- `/Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-profile-retest-20260829-172042-776889-lab/qa-artifacts/qa/profile-segmentation-walk.md`
- `/Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-profile-retest-20260829-172042-776889-lab/qa-artifacts/qa/qa-audit-report.md`
- `/Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-profile-retest-20260829-172042-776889-lab/qa-artifacts/qa/teardown.json`

## What Was Fixed

- Restricted the hosted-MCP active-run requirement to terminal tools; generic session tools remain available outside an active run.
- Fixed stale terminal nonces and the daemon integration buffer race.
- Stopped failed or disabled model-catalog sources from ignoring their future refresh deadline, and coalesced concurrent daemon refreshes.
- Moved extension install, enablement, and lifecycle fencing onto the global SQLite write protocol with bounded `SQLITE_BUSY` retries.
- Preserved the latest overlapping profile retarget, restored journal detail only after replay exits, and kept input-request and quote actions above the terminal canvas.
- Updated the rebased browser contracts for the hard-cut empty-session flow, current Frimousse searchbox semantics, authority-backed OS-window identity, and public response completion boundaries.
- Kept journal first-open lazy loading across OS window rematerialization in an app-wide, workspace-keyed store while preserving profile-specific TanStack Query keys.
- Corrected E2E-020 to follow the authority-backed Terminal surface instead of freezing an opaque window id, and to click the same visible log whose write lease was verified.
- Corrected E2E-002 to bind an opaque window id only after the OS authority settles each create, perform one explicit control transition, and prove final scrollback through the public quote surface instead of treating the mutable current screen as history.
- Made the shared Dialog portal follow the root open state so command-palette actions cannot leave an invisible pointer-blocking overlay; removed the obsolete exit-motion helpers and added component plus OS-shell regression coverage.
- Split terminal workspace producers into startup and registered phases so unregister can seal new work,
  archive running terminals, then wait for final producers without deadlocking.
- Pinned active journal lanes to their admitted workspace database before removal staging and allowed only
  that existing handle to finish final rows while new database admission remains sealed.

## CI Repair Follow-up — 2026-08-30

Exact-head CI run `33281491797` passed every verification lane except the combined runtime/Web E2E
job. That job completed 268 browser scenarios with 3 skips and exposed four failures:

- Bash prompt machinery replaced the operator's authenticated command marker with
  `__compozy_precmd`, so E2E-001 and E2E-020 could display the command but omit its journal row.
- At the detach timeout edge, a queued second `Ctrl-\\` could lose a `select` race to the timer and
  reach the shell as another SIGQUIT instead of completing the detach chord in E2E-011.
- A successful remote window-manager snapshot cleared an already-clear load error and published the
  full 12-window projection a second time; the redundant render produced one 52 ms peer-convergence
  task in E2E-023.

The current repair tree arms Bash command capture only between prompts, gives an already-read detach
byte priority over timeout expiry, and skips unchanged load-error publications while retaining the
manual projection required before the runtime subscription starts. Focused verification passed all
eight matching browser scenarios, including the four exact failures. Five additional performance-envelope
repetitions all reported 0 ms worst drag and peer-convergence tasks; restore from the snapshot response
ranged from 63.5 ms to 86.1 ms, versus the failed CI run's 52 ms peer task.
The canonical PTY and CLI regressions pass under `-race`, and the window-manager runtime suite passes
29/29 through the repository-root Turbo graph.

QA tracker impact: `ET-terminal-cli-public-contract`, `ET-terminal-journal-recording`, and
`ET-terminal-profile-segmentation` were re-walked against the current tree and remain passing. The
window-manager change only removes a duplicate render publication; it changes no gesture, layout,
route, or persistence contract, so it does not reset the broader gesture scenario.

### Delayed-reader follow-up

Exact-head CI run `33286027721` passed every verification and external check, plus 271 of 272 Web
scenarios with 3 skips. E2E-001, E2E-020, and E2E-023 confirmed the prior Bash journal and projection
repairs. E2E-011 alone failed because a saturated runner delayed delivery of the second `Ctrl-\`
beyond the 150 ms chord window, so the byte reached the shell instead of detaching.

The CLI now allows 500 ms for the two-byte human chord while preserving the single-key SIGQUIT path.
The canonical delayed-reader regression separates the bytes by 250 ms and passed five times under
`-race`. E2E-011 passed 10/10 focused repetitions and a fresh post-flag public-interface walk. The
`ET-terminal-cli-public-contract` scenario is passing again; the current gate and replacement exact-head
CI remain pending.

### Parallel-lane exact-head follow-up

Exact-head CI run `33296083331` validated the new parallel E2E layout: runtime, Desktop, Windows,
Go, frontend, build, and lint lanes passed, and Web completed inside its dedicated budget with 270
passes, 3 skips, and 2 failures. The Web artifacts isolated two completion races:

- Tasks loaded the target session permalink data, but issued no new window-manager command and left
  the run window focused. The run-detail response already owned both the session id and agent name,
  so the controller now opens that canonical session route directly and reserves the async ID-only
  permalink for incomplete projections.
- The terminal detach chord completed and restored the local terminal mode while the CLI was still
  fetching the final state for its detach notice. A delayed `Ctrl-\\` then reached the Linux line
  discipline as local SIGQUIT. Attached open and attach now retain raw mode until that final query
  and notice complete.

The existing controller suite proves canonical-route selection and the permalink fallback. Its three
tests pass through the repository-root Turbo graph. The CLI timing and raw-mode cases passed three
times under `-race`. The two public browser journeys then passed together in five consecutive runs
each (10/10 total). `TA-web-task-run-detail-redesign` and `ET-terminal-cli-public-contract` were
flagged before the walk and are passing again.

## Paper Cuts

- The isolated operator shell reports `No such theme: rose-pine` and fish waits ten seconds for a Primary Device Attribute query before each first prompt. Commands still execute correctly, but terminal startup is noisy and slower than necessary.

## Runtime Errors Observed

- The first canonical Web E2E run exposed three transient extension writes as `SQLITE_BUSY`; the extension registry now uses `BEGIN IMMEDIATE` with bounded retry and its canonical contention test passes under `-race`.
- The first run also exposed stale browser assumptions after PR #498: Frimousse exposes a named searchbox, session creation completes asynchronously, and a Terminal launcher can retarget to a different opaque window. The tests now follow those public contracts without forced clicks or increased timeouts.
- The post-PR #494 focused run exposed `BUG-20260829-terminal-journal-unlock-remount`: the journal could stay on its loading state after the OS rematerialized the Terminal window during an all-profiles switch. The aggregate request was absent from browser network evidence. The working-tree fix passed E2E-020 ten times in series.
- The fresh isolated browser walk exposed `BUG-20260829-command-palette-portal-remains-mounted`: the current Terminal window received focus, but the shared Dialog portal stayed mounted after the palette command and blocked the destination. The component suite, build, and real-browser retest pass on the working-tree fix; the canonical Web E2E remains pending.
- The same fresh walk exposed `BUG-20260829-workspace-delete-visible-terminal-deadlock`: unregister
  first waited on a visible exec that only Commit could close, then the final journal row could not reuse
  its staged workspace database. Both lifecycle boundaries now have race-tested regressions; the real
  removal completed in 0.15 seconds and daemon boot recovered the interrupted attempt.
- The next canonical Web E2E run passed E2E-020 but finished at 271 passed, 3 skipped, and 1 failed because E2E-002 froze a window identity before the authority settled and later confused current-screen output with retained scrollback. The corrected case passed ten focused repetitions; the canonical full rerun remains pending.
- No secret values appeared in browser artifacts, terminal journal output, or daemon logs reviewed during the walk.

## UI Technical Audit

Implementation integrity: **Pass**. The changed terminal controls preserve the product-specific lease, replay, and input-handoff model; they do not introduce a parallel visual or interaction system.

| Dimension | Score | Evidence |
|---|---:|---|
| Accessibility | 4 / 4 | Input requests remain a polite status region, use shared button semantics, and remove mutation controls from aggregate reads. |
| Performance | 4 / 4 | The request stack is bounded and scrollable; no new asset, animation, polling loop, or layout measurement was added. |
| Responsive design | 3 / 4 | Overlays keep bounded height and flex containment; this terminal window remains an intentionally desktop-first OS surface. |
| Theming | 4 / 4 | All changed surfaces use the existing semantic tokens and shared primitives. |
| Implementation integrity | 4 / 4 | The bundled detector found only the existing `border-l-2` quote marker, verified as a semantic blockquote convention rather than a new decorative side-tab. |
| **Total** | **19 / 20 — Excellent** | No P0–P3 issue attributable to this remediation. |

No systemic issue or follow-up UI command was identified. Positive controls include the explicit watcher label (`Take control & send`), focusable shared actions, bounded overlays above xterm, and replay detail that stays hidden until live journal mode resumes.

## Human Verifications Needed

None identified yet.

## Decisions for a Human

None identified yet.

## Learnings

- PR #498 (`feat: rebuild ACP runtime catalogs`) explicitly owns logical model identity, live provider catalogs, typed ACP options, shared runtime selection, first-message creation behavior, eager session acceptance, and global migrations through 00097. Those description-level contracts explain the rebase conflicts in session creation/deletion, model refresh, generated contracts, and migration numbering.
- The terminal branch keeps PR #498's hard cuts, moves its own global migration to 00098, and adds terminal behavior on top. The seven overlap canaries in this report cover the merged contract rather than preserving either branch's stale assumptions.
- PR #494 (`feat: support child Loop config overrides`) owns optional `run-loop.params.config_overrides`, literal-field rejection, typed reference preservation, and non-persistent child-run overrides. Its only rebase conflicts were shared E2E helpers: `extensions.spec.ts` kept PR #494's unmounted-on-Escape palette contract, while `profiles.spec.ts` composed its named Frimousse searchbox and forward-focus reachability with the terminal branch's real `data-selected` state and reverse keyboard navigation to color swatches.
- Current repository instructions require `make gate` before commit or push; the exact-head PR CI remains the delivery backstop.

## Final Status

- **Exit gate (full automated suite):** focused CI-failure rerun passed; `make gate` and exact-head CI pending
- **Issues by user impact:** Blocks-Completion 0 open · Data-Loss 0 · Trust-Damage 0 · Friction 0 open · Cosmetic 0
- **Coverage:** 5 / 5 journeys walked; 9 / 9 tracked rows passed; strict targeted QA audit passed
- **Teardown:** `/Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-profile-retest-20260829-172042-776889-lab/qa-artifacts/qa/teardown.json` reports `clean: true` with no survivors.
- **Verdict:** QA passed — the canonical full Web E2E rerun, `make gate`, exact-head CI, and final Claude/Fable review remain delivery gates.
