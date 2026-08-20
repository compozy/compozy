# QA Run Report — 2026-08-20 — command-palette

- **Scope:** Command palette tasks 01–10: daemon registry, structured surfaces, ranking and personalization, execution UX, keymap/settings, domain and extension views, programmable view runtime, desktop global hotkeys, and agent fallback.
- **Cadence tier:** full
- **Build:** `3e42a7e988305847477029e9944e858b9328108d` · **Environment:** isolated lab `command-palette-20260820-072509-978555`, daemon `http://127.0.0.1:56298`, browser-use; local production-like build, not a deployed release artifact.
- **Started:** 2026-08-20T07:25:46Z · **Status:** deferred by operator
- **Bootstrap manifest:** `/Users/pedronauck/dev/qa-labs/compozy-command-palette-20260820-072509-978555-lab/qa-artifacts/qa/bootstrap-manifest.json`
- **Real-scenario playbook:** `devtool-oss-launch` (Mateo Rivera, one kickoff; runtime observation and strict audit pending)

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | Agent/operator | desktop / wifi-fast / en-US | CH-palette-approval-exactly-once |
| Bruno | Keyboard-first operator | desktop / wifi-fast / en-US | CH-palette-view-session-isolation, CH-palette-membership-vs-health, CH-palette-keymap-conflict-truth, CH-palette-catalog-revision-truth, CH-palette-make-it-mine, CH-palette-global-summon-truth, CH-palette-sessions-landing-canary |
| Sol | Keyboard and screen-reader operator | desktop / wifi-fast / en-US | CH-palette-domain-views-grammar |
| Mateo Rivera | Helix CLI founder/operator | desktop / wifi-fast / en-US | devtool-oss-launch runtime scenario |

## Flows in Scope

- `J-operate-command-palette` — discover, invoke, approve, and configure through structured surfaces (`../journeys/J-operate-command-palette.md`).
- `J-command-os-from-palette` — command the OS, browse views, personalize, and fall back to an agent (`../journeys/J-command-os-from-palette.md`).
- `J-extension-dev-lifecycle` — validate extension contribution membership, health, reload, and recovery (`../journeys/J-extension-dev-lifecycle.md`).
- `J-operate-desktop-shell` — preserve keyboard, global summon, and session-landing behavior (`../journeys/J-operate-desktop-shell.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-palette-approval-exactly-once | J-operate-command-palette / ET-agent-command-invoke; ET-agent-palette-config-parity | Ada | Interrupt Tour | Pending | | |
| 2 | CH-palette-view-session-isolation | J-command-os-from-palette / ET-extension-palette-contributions; ET-palette-nested-views | Bruno | Multi-Tab Tour | Pending | | |
| 3 | CH-palette-membership-vs-health | J-extension-dev-lifecycle / ET-extension-palette-contributions | Bruno | Interrupt Tour | Pending | | |
| 4 | CH-palette-keymap-conflict-truth | J-operate-desktop-shell / ET-web-command-palette-shortcuts | Bruno | Garbage Tour | Pending | | |
| 5 | CH-palette-catalog-revision-truth | J-command-os-from-palette / ET-palette-registry-driven-root; ET-window-tab-palette-search; ET-palette-agent-fallback | Bruno | Network Tour | Pending | | |
| 6 | CH-palette-make-it-mine | J-command-os-from-palette / ET-palette-action-panel; ET-palette-inline-args-confirmation; ET-palette-personalization-lifecycle | Bruno | Garbage Tour | Pending | | |
| 7 | CH-palette-domain-views-grammar | J-command-os-from-palette / ET-palette-domain-views | Sol | Feature Tour | Pending | | |
| 8 | CH-palette-global-summon-truth | J-command-os-from-palette / ET-desktop-global-summon | Bruno | Feature Tour | Pending | | |
| 9 | CH-palette-sessions-landing-canary | J-operate-desktop-shell / ET-palette-sessions-view-switch | Bruno | Feature Tour | Pending | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Release-Grade Runtime Scenario

- **Playbook:** `devtool-oss-launch`
- **Operator kickoff:** Completed once; transcript captured in the isolated lab
- **Task activation:** Released for all 11 deterministic tasks
- **Observation:** Stalled after 300 seconds without runtime-owned journey-log progress
- **Strict audit:** Deferred to the follow-up QA round
- **Known open bugs to deduplicate if reproduced:** `BUG-0028`, `BUG-20260719-autonomous-progress-unobservable`, `BUG-20260816-daemon-stop-timeout`

## Session Debriefs

Pending. Each charter debrief is written here immediately after its session.

## What Was Fixed

- `BUG-20260813-desktop-shell-context-order` — moved the palette registry consumer below the shell
  provider (`531b9f5`); the daemon-served desktop now mounts instead of entering the root boundary.
- `BUG-0017` — encoded every required empty command-palette and client collection as `[]`
  (`c3c50b6`); focused contract tests pass under `-race`.
- `BUG-20260729-session-window-cross-tab-focus` — read the workspace/client-scoped settings envelope
  from the Query cache (`538777e`); 27 runtime cases and the failed daemon-served Agents journey pass.

## Paper Cuts

None recorded yet.

## Runtime Errors Observed

- Desktop bootstrap initially failed with `useOsShell requires an <OsShellContext.Provider> above`.
- After provider repair, the catalog failed while flattening a required collection encoded as
  `null`, followed by client-registration retries on `global_shortcuts: null`.
- After contract repair, `/agents` and the daemon snapshot advanced while no window rendered because
  the runtime read a global bare config instead of the scoped settings envelope.

## Human Verifications Needed

None. The nine charter walks and the runtime scenario remain Pending in the session matrix; those are unwalked sessions, not human-only blockers.

## Decisions for a Human

None recorded yet.

## Learnings

Pending.

## Final Status

- **Exit gate (full automated suite):** Deferred by operator; no completion gate was run
- **Issues by user impact:** Not assessed in this round
- **Coverage:** 0/9 charters walked; runtime scenario stopped after the observation stall
- **Verdict:** Deferred — implementation is being opened for review before QA and gates; a
  separate follow-up round owns the nine charter walks, strict runtime audit, and final gates.
