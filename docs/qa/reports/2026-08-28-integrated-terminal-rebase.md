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

- **Exit gate (full automated suite):** full Web E2E rerun in progress; `make gate` pending
- **Issues by user impact:** Blocks-Completion 0 open · Data-Loss 0 · Trust-Damage 0 · Friction 0 open · Cosmetic 0
- **Coverage:** 5 / 5 journeys walked; 9 / 9 tracked rows passed; strict targeted QA audit passed
- **Teardown:** `/Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-profile-retest-20260829-172042-776889-lab/qa-artifacts/qa/teardown.json` reports `clean: true` with no survivors.
- **Verdict:** QA passed — the canonical full Web E2E rerun, `make gate`, exact-head CI, and final Claude/Fable review remain delivery gates.
