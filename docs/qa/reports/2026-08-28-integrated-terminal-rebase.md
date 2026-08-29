# QA Run Report — 2026-08-28 — integrated-terminal-rebase

- **Scope:** PR #490 after rebasing onto PR #498; re-walk the terminal stream, approval, and agent handoff contracts across Web, CLI, HTTP API, and runtime, with session-deletion and model-catalog canaries for the overlapping rebase changes.
- **Cadence tier:** targeted
- **Build:** local remediation tree based on `b1842a48a` · **Environment:** isolated local lab `integrated-terminal-rebase-20260828-201516-678087`, daemon `127.0.0.1:61075`, isolated UDS and provider home
- **Started:** 2026-08-28T20:17:33Z · **Status:** in-progress

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Marina | isolated terminal workspace | desktop / flaky + wifi-fast / en-US | CH-terminal-stream-flow-control, CH-terminal-lease-fencing-takeover |
| Bruno | isolated terminal workspace | desktop / wifi-fast / en-US | CH-terminal-approval-ladder |
| Théo | isolated terminal workspace | desktop / wifi-fast / en-US | CH-session-empty-and-dock-last-created |
| Sol | isolated terminal workspace | desktop / wifi-fast / en-US | CH-runtime-ui-regression-model-catalog |

## Flows in Scope

- `J-operate-integrated-terminal` — operate and recover a terminal through Web, CLI, API, and runtime (`../journeys/J-operate-integrated-terminal.md`)
- `J-supervise-agent-terminal` — supervise terminal work while preserving approval, control, and redaction boundaries (`../journeys/J-supervise-agent-terminal.md`)
- `J-15` — delete a focused session without destroying its window (`../journeys/J-15-operate-session-via-cli-api.md`)
- `J-17` — open and use the runtime selector from a cold catalog (`../journeys/J-17-session-create-unified-selector.md`)

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

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

- Marina watched a CLI-owned terminal in Web, took control explicitly, typed `echo WEB_TYPED`, and verified the same output and exit status independently through the CLI quote surface. The multi-viewer, backpressure, replay, resize, and reconnect contracts also passed the runtime E2E lane.
- Bruno's approval ladder, typing-grant scope, revocation, and irreversible-command refusals passed the runtime E2E lane. No hidden input appeared in the visible transcript.
- Théo deleted a focused session from Web, stayed in the same empty session tab, and confirmed the independent API read returned 404. Dock/session creation behavior passed its browser E2E canaries.
- Sol opened the model selector from the isolated onboarding surface. Persisted rows appeared without a manual reload, and the focused catalog suite passed with one coalesced background refresh per source.

Primary evidence:

- `/Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-rebase-20260828-201516-678087-lab/qa-artifacts/qa/journey-log.jsonl`
- `/Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-rebase-20260828-201516-678087-lab/qa-artifacts/qa/logs/web-terminal-cross-surface.md`
- `/Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-rebase-20260828-201516-678087-lab/qa-artifacts/qa/logs/test-e2e-runtime-after-fix.log`
- `/Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-rebase-20260828-201516-678087-lab/qa-artifacts/qa/screenshots/session-delete-empty-tab.png`
- `/Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-rebase-20260828-201516-678087-lab/qa-artifacts/qa/screenshots/onboarding-runtime.png`

## What Was Fixed

- Restricted the hosted-MCP active-run requirement to terminal tools; generic session tools remain available outside an active run.
- Fixed stale terminal nonces and the daemon integration buffer race.
- Stopped failed or disabled model-catalog sources from ignoring their future refresh deadline, and coalesced concurrent daemon refreshes.
- Moved extension install, enablement, and lifecycle fencing onto the global SQLite write protocol with bounded `SQLITE_BUSY` retries.
- Preserved the latest overlapping profile retarget, restored journal detail only after replay exits, and kept input-request and quote actions above the terminal canvas.
- Updated the rebased browser contracts for the hard-cut empty-session flow, current Frimousse grid semantics, stable OS-window identity, and public response completion boundaries.

## Paper Cuts

- The isolated operator shell reports `No such theme: rose-pine` and fish waits ten seconds for a Primary Device Attribute query before each first prompt. Commands still execute correctly, but terminal startup is noisy and slower than necessary.

## Runtime Errors Observed

- The first canonical Web E2E run exposed three transient extension writes as `SQLITE_BUSY`; the extension registry now uses `BEGIN IMMEDIATE` with bounded retry and its canonical contention test passes under `-race`.
- The first run also exposed stale browser assumptions after PR #498: Frimousse emojis are a `grid`, session creation completes asynchronously, and a Terminal launcher can retarget to a different opaque window. The tests now follow those public contracts without forced clicks or increased timeouts.
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
- Per the operator's instruction, this run will not execute `make gate`; scoped QA and exact-head PR CI are the delivery evidence.

## Final Status

- **Exit gate (full automated suite):** rerun in progress — exact-head PR CI will replace local `make gate` by operator instruction
- **Issues by user impact:** Blocks-Completion 0 open · Data-Loss 0 · Trust-Damage 0 · Friction 1 open · Cosmetic 0
- **Coverage:** 4 / 4 journeys walked; 7 / 7 tracked scenarios passed
- **Verdict:** in progress — the full Web E2E rerun, teardown, exact-head CI, and final Claude/Fable review remain pending.
