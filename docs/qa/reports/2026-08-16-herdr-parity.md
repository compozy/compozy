# QA Run Report — 2026-08-16 — Herdr parity

- **Scope:** Herdr parity attention state, agent-manageable session controls, operator notification delivery, daemon-owned shortcuts, and nested Sessions palette
- **Cadence tier:** full
- **Build:** Task 08 working tree after `a8585741a2a1b267aae44e870a878cd8e07126c9`
- **Environment:** isolated `northstar-pay-20260816-141901-835450` lab; runtime API `http://127.0.0.1:59798`
- **Started:** 2026-08-16T14:21:41Z
- **Core scenario status:** pass
- **Lab teardown:** clean at 2026-08-17T00:05:29Z

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Cora | Herdr parity attention operator | laptop / fast Wi-Fi / en-US | CH-herdr-attention-signals, CH-herdr-task-approval-canary |
| Sol | Keyboard and screen-reader operator | desktop / fast Wi-Fi / en-US | CH-herdr-attention-accessibility |
| Théo | Multi-client session operator | desktop / fast Wi-Fi / en-US | CH-herdr-done-presence, CH-herdr-interaction-recovery |
| Ada | Agent/runtime operator | desktop / fast Wi-Fi / en-US | CH-herdr-session-orchestration, CH-herdr-attention-hook |
| Dora | Runtime settings administrator | desktop / fast Wi-Fi / en-US | CH-herdr-attention-settings |
| Bruno | Keyboard-first desktop operator | desktop / fast Wi-Fi / en-US | CH-herdr-keymap-hard-cut, CH-herdr-keyboard-navigation |
| Sofia Mendes | Northstar Pay Founder/PM | desktop / fast Wi-Fi / en-US | northstar-pay companion real scenario |

## Flows in Scope

- `J-respond-to-agent-attention` — notice, enter, resolve, and clear cross-workspace attention
- `J-11` — return to a running session and preserve finished-unseen truth
- `J-answer-agent-requests` — discover and resolve live or restart-orphaned requests
- `J-15` — operate sessions through structured CLI, API, UDS, and native surfaces
- `J-administer-runtime-settings` — keep attention policy identical across public settings surfaces
- `J-administer-window-manager` — edit and verify the daemon-owned keymap
- `J-operate-desktop-shell` — reach active work through shortcuts and palette views
- `J-agent-marketplace-parity` — discover and observe the attention hook
- `J-operate-home-dashboard` — preserve the task-approval attention canary

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix batch |
|---|---|---|---|---|---|---|---|
| 1 | CH-herdr-attention-signals | J-respond-to-agent-attention / MS-web-attention-channel-states | Cora | Feature Tour | Pass | | |
| 2 | CH-herdr-attention-signals | J-respond-to-agent-attention / RT-session-spawn-wake | Cora | Feature Tour | Pass | | |
| 3 | CH-herdr-attention-signals | J-respond-to-agent-attention / RT-web-attention-bell-jump | Cora | Feature Tour | Fixed | BUG-20260729-session-window-cross-tab-focus | Task 08 QA |
| 4 | CH-herdr-attention-signals | J-respond-to-agent-attention / RT-web-attention-title-count | Cora | Feature Tour | Pass | | |
| 5 | CH-herdr-attention-signals | J-respond-to-agent-attention / RT-web-attention-toast-delivery | Cora | Feature Tour | Pass | | |
| 6 | CH-herdr-attention-signals | J-respond-to-agent-attention / RT-web-session-all-workspaces | Cora | Feature Tour | Pass | | |
| 7 | CH-herdr-attention-signals | J-respond-to-agent-attention / RT-web-session-attention-sort | Cora | Feature Tour | Pass | | |
| 8 | CH-herdr-attention-accessibility | J-respond-to-agent-attention / MS-web-attention-channel-states | Sol | Back-Button Tour | Pass | | |
| 9 | CH-herdr-attention-accessibility | J-respond-to-agent-attention / RT-web-attention-bell-jump | Sol | Back-Button Tour | Fixed | BUG-20260729-session-window-cross-tab-focus | Task 08 QA |
| 10 | CH-herdr-attention-accessibility | J-respond-to-agent-attention / RT-web-session-all-workspaces | Sol | Back-Button Tour | Pass | | |
| 11 | CH-herdr-attention-accessibility | J-respond-to-agent-attention / RT-web-session-attention-sort | Sol | Back-Button Tour | Pass | | |
| 12 | CH-herdr-done-presence | J-11 / RT-session-done-presence | Théo | Multi-Tab Tour | Pass | | |
| 13 | CH-herdr-interaction-recovery | J-answer-agent-requests / RT-021 | Théo | Interrupt Tour | Pass | | |
| 14 | CH-herdr-interaction-recovery | J-answer-agent-requests / RT-session-clarification-roundtrip | Théo | Interrupt Tour | Pass | | |
| 15 | CH-herdr-interaction-recovery | J-answer-agent-requests / RT-session-native-interaction-resolution | Théo | Interrupt Tour | Pass | | |
| 16 | CH-herdr-session-orchestration | J-15 / RT-session-attention-catalog | Ada | Interrupt Tour | Pass | | |
| 17 | CH-herdr-session-orchestration | J-15 / RT-operator-notification-delivery | Ada | Interrupt Tour | Pass | | |
| 18 | CH-herdr-session-orchestration | J-15 / RT-session-wait-state | Ada | Interrupt Tour | Pass | | |
| 19 | CH-herdr-session-orchestration | J-15 / RT-session-prompt-cancel | Ada | Interrupt Tour | Pass | | |
| 20 | CH-herdr-session-orchestration | J-15 / RT-session-native-stop | Ada | Interrupt Tour | Pass | | |
| 21 | CH-herdr-attention-settings | J-administer-runtime-settings / MS-attention-settings-roundtrip | Dora | Multi-Tab Tour | Pass | | |
| 22 | CH-herdr-keymap-hard-cut | J-administer-window-manager / ET-layout-editor-shortcut-recorder | Bruno | Multi-Tab Tour | Pass | | |
| 23 | CH-herdr-keymap-hard-cut | J-administer-window-manager / MS-configure-window-manager | Bruno | Multi-Tab Tour | Pass | | |
| 24 | CH-herdr-keymap-hard-cut | J-administer-window-manager / MS-terminal-shortcut-preset | Bruno | Multi-Tab Tour | Pass | | |
| 25 | CH-herdr-keymap-hard-cut | J-administer-window-manager / MS-window-shortcut-arrays-ranges | Bruno | Multi-Tab Tour | Pass | | |
| 26 | CH-herdr-keyboard-navigation | J-operate-desktop-shell / ET-editable-shell-shortcuts | Bruno | Feature Tour | Pass | | |
| 27 | CH-herdr-keyboard-navigation | J-operate-desktop-shell / ET-keyboard-navigation-actions | Bruno | Feature Tour | Pass | | |
| 28 | CH-herdr-keyboard-navigation | J-operate-desktop-shell / ET-live-shortcut-cheatsheet | Bruno | Feature Tour | Pass | | |
| 29 | CH-herdr-keyboard-navigation | J-operate-desktop-shell / ET-web-command-palette-shortcuts | Bruno | Feature Tour | Pass | | |
| 30 | CH-herdr-keyboard-navigation | J-operate-desktop-shell / ET-web-shell-shortcuts-about-dialogs | Bruno | Feature Tour | Pass | | |
| 31 | CH-herdr-keyboard-navigation | J-operate-desktop-shell / ET-palette-nested-views | Bruno | Feature Tour | Pass | | |
| 32 | CH-herdr-keyboard-navigation | J-operate-desktop-shell / ET-palette-sessions-view-switch | Bruno | Feature Tour | Pass | | |
| 33 | CH-herdr-attention-hook | J-agent-marketplace-parity / ET-042 | Ada | Feature Tour | Pass | | |
| 34 | CH-herdr-task-approval-canary | J-operate-home-dashboard / RT-home-approve-from-dashboard | Cora | Feature Tour | Pass | | |

Status legend: `Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

- **Attention signals/accessibility:** the cross-workspace jump first exposed stale Window Manager topology. After the shared hook waited for the target workspace projection, the same browser flow opened the named permission session, resolved it, opened the Finished session, and reached `needs_you=0`, `finished=0` with an `All quiet` bell.
- **Done presence/interactions:** restart-orphaned interactions, clarification, permission resolution, done-presence clearing, prompt cancellation, and native stop all converged on one durable state under the runtime E2E race suite.
- **Session orchestration:** CLI, HTTP, UDS, and native tools returned the same structured catalog, wait, notify, cancel, and stop outcomes, including stable scope and capacity errors.
- **Attention settings:** complete-section writes preserved non-null arrays and public workspace registration IDs across settings transports. Muted rows remained visible while delivery stopped.
- **Keymap/keyboard/palette:** arrays, compact ranges, partial overrides, conflict ownership, Terminal preset rollback, editable-context shortcuts, nested view stack, and zero-match clear behavior passed Web E2E and their visual contracts.

## What Was Fixed

- Foreign-workspace attention activation now waits until the live Window Manager snapshot belongs to the target workspace before opening the session. The governed regression and browser retest are recorded in `BUG-20260729-session-window-cross-tab-focus`.
- Settings boundaries now emit non-null attention and shortcut arrays, use public workspace registration IDs for mutes, and retain every window-manager navigation limit.
- Attention-first catalog ordering now stays durable across cursor pages; widened workspace listings are loaded only when their surface needs them.
- Prompt cancellation is atomic; boot repair, delegation tool budgets, memory extraction, and ACP prompt fixtures now preserve their public contracts under restart and nested-agent paths.
- Loop environment persistence, missing-worktree detection, dialog overflow, toast hit-testing, and E2E cleanup races were corrected while closing the full browser matrix.

## Final Deep Review

- Exactly one full-diff Codex reviewer round covered 627 selected files and 1,944 selected hunks. The initial verdict was `FIX_BEFORE_SHIP` with two defects and two advisories.
- Browser blur now releases session presence even while the internal shell window remains focused, preserving truthful `done` behavior.
- Daemon-backed attention settings serialize an empty `muted_workspaces` value as `[]`, matching the non-null public contract.
- Unchanged presence renewals no longer emit catalog invalidations, while same-badge revision changes still do.
- Renewal now distinguishes an invalid lease from retryable transport/server failures, retaining the existing lease across transient failures.
- Remediation evidence: 19 focused Web tests passed; the canonical CLI and session catalog Go regressions passed with the race detector; Web lint/typecheck and Go lint passed with zero warnings.
- Review source and disposition: `.deep-review/herdr-parity-final/reviewer-findings.json` and `.deep-review/herdr-parity-final/remediation.json`.

## Paper Cuts

- No unresolved Herdr parity paper cut remained after the visual and browser retests.

## Runtime Errors Observed

- The broad Northstar companion observer stopped growing after fourteen controller rows and detected a five-minute stall. This re-found `BUG-20260719-autonomous-progress-unobservable`; no provider-backed session was fabricated to hide the failure.
- The isolated daemon required an escalated signal during teardown after its graceful stop window, but `teardown.json` records no survivor.

## Human Verifications Needed

- None for the Herdr parity contract. The lab's real browser capability produced the unavailable notification state; deterministic browser coverage exercised the granted and denied UI branches. Native OS notification presentation remains outside this browser-run evidence.

## Decisions for a Human

- `BUG-20260719-autonomous-progress-unobservable` remains a product-wide, pre-existing decision outside the Herdr parity remediation governor. The broad Northstar companion scenario cannot be called a provider-backed pass until that bug is scheduled and fixed.

## Experiential Lens Pass

- The attention journey preserved one clear pull signal, exact workspace/session landing, truthful stale/quiet states, and keyboard reachability.
- The palette journey preserved neutral selection, stable focus, one-level back navigation, and visible recovery for empty and zero-match states.

## Visual Contract Evidence

- Task 03 VC-01..VC-23: 23/23 `PASS`, zero blocking divergences.
- Task 05 VC-01..VC-08: 8/8 `PASS`, zero blocking divergences.
- Task 06 VC-01..VC-05: 5/5 `PASS`, zero blocking divergences.
- Durable bundles: `.compozy/tasks/herdr-parity/evidence/visual/`.

## Browser Journey Evidence

- Needs-you cross-workspace landing: `qa/screenshots/herdr-cross-workspace-needs-you-fixed.png`.
- Finished presence clearing: `qa/screenshots/herdr-finished-presence-cleared.png`.
- Final quiet state: `qa/screenshots/herdr-attention-all-quiet-cleared.png`.
- Evidence root: `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/`.

## Northstar Pay Companion Scenario

- Exactly one Sofia Mendes kickoff and twelve task-run starts were recorded.
- The observer reported `stall_detected=true`; all twelve runs remained incomplete and all twelve declared deliverables remained unstarted.
- `provider-attempt.json` contains no provider-backed proof. The strict companion audit therefore did not pass.
- Evidence: `observation-summary.json`, `journey-log.jsonl`, and `provider-attempt.json` under the lab evidence root.
- Disposition: deduplicated against `BUG-20260719-autonomous-progress-unobservable`; not counted as a Herdr parity scenario pass and not broadened into an unrelated architecture fix.

## Automated Evidence

- Focused attention browser lane: 19 passed.
- `make test-e2e-runtime`: pass.
- `make test-e2e-web`: 180 passed, 3 skipped, 0 failed.
- Visual-contract validator: 36 passed, 0 failed.
- `make codegen-check`: pass.
- Deep-review remediation regressions: 19 Web tests passed; focused CLI and session catalog Go tests passed with `-race`.
- Workstream close: `make gate-full` pass; `make gate-status` reports a current successful full-gate record for the final tree.

## Production-Parity Deviations

- The broad Northstar companion never reached a provider-backed session because the registered autonomous-progress bug stalled public progress first.
- Native OS notification presentation was not asserted from the browser lab; the real unavailable state plus deterministic granted/denied browser branches cover the product mapping without claiming OS delivery.
- No Herdr parity CLI, HTTP, UDS, native-tool, Web, persistence, or workspace-isolation deviation remains open.

## Teardown Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/teardown.json` records `"clean": true` and `"survivors": []`.
- Registered observer, Web dev server, and daemon PIDs were terminated. Visual-contract Storybook/static servers on ports 6006, 6007, and 6010 were also stopped with no listeners remaining.

## Learnings

- A live Window Manager client is not sufficient proof of routing readiness after a workspace switch; commands must bind to the target workspace projection.
- Serialized list fields must be made non-null at the API boundary even when Go treats nil and empty slices similarly.
- Visual state fixtures must reach state through the same interaction path when session-local affordances, such as Revert, depend on history.

## Final Status

- **Core issues by user impact:** Blocks-Completion 1 fixed · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 34/34 matrix rows settled; 30/30 unique Herdr parity scenarios passed
- **Companion broad-scenario issue:** 1 registered pre-existing blocker remains outside this workstream
- **Verdict:** Herdr parity QA and workstream closure pass. The single deep-review round is fully remediated, the full gate is current, and teardown is clean.
