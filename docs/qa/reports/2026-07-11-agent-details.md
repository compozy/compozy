# QA Run Report — 2026-07-11 — agent-details

- **Scope:** Agent fleet, definition detail/settings, authored context, lifecycle parity and durability
- **Cadence tier:** targeted
- **Build:** `08788194` plus Phase C QA plan · **Environment:** fresh isolated lab; manifest below
- **Started:** 2026-07-11T19:09:57Z · **Finished:** 2026-07-11T21:38:10Z · **Status:** complete

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
| --- | --- | --- | --- |
| Bruno | Power User | desktop / wifi-fast / en-US | CH-049, CH-051 |
| Sol | Accessibility-Reliant | desktop / wifi-fast / en-US | CH-050 |
| Ada | Power User | structured CLI/API / wifi-fast / en-US | CH-052 |

## Flows in Scope

- `J-30 Scan the agent fleet` — find the correct workspace-visible definition without invented state.
- `J-31 Steward an agent definition` — inspect and edit definition-owned state with CAS and unsaved protection.
- `J-32 Manage the agent lifecycle across surfaces` — prove duplicate/delete parity, restart durability, and session survival.

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | CH-049 | J-30 / RT-074, RT-075 | Bruno | Feature | Pass | — | — |
| 2 | CH-050 | J-31 / RT-028, RT-076, RT-077, RT-082 | Sol | Accessibility | Fixed | BUG-0038 | `f7d8a96` |
| 3 | CH-051 | J-31 + J-32 / RT-069, RT-078, RT-079, RT-080 | Bruno | Scenario | Fixed | BUG-0035; BUG-0036 | `f7d8a96` |
| 4 | CH-052 | J-32 / RT-029, RT-081 | Ada | Claims | Fixed | BUG-0035 | `f7d8a96` |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Release Gates

| Gate | Verdict | Evidence |
| --- | --- | --- |
| Cross-surface parity | Pass | CLI, HTTP, UDS-backed CLI, web, and live native-tool catalog checks agree; `qa/agent-details-evidence.md`. |
| Delete durability after restart/re-sync | Pass | Three-surface un-shadowing and deleted-agent session status/inspect/history survived restart. |
| Duplicate fidelity including sidecars | Pass | MCP, Soul, Heartbeat, and opaque files were byte-identical except requested name/digest changes. |
| Truthful verbs and metrics | Pass | Non-interactive confirmation, CAS conflict, Heartbeat eligibility, partial sessions, and live/total signals matched daemon state. |
| WCAG AA floor | Pass | Semantic a11y snapshots plus neutral pill contrast 5.06:1 on canvas and 4.70:1 on soft surface. |
| Approved design parity | Pass | Desktop/920/560 captures match `docs/design/opendesign`; deterministic topbar capture at `/tmp/agh-ui-screenshot/task09-controller/topbar-actions.png`. |

## What Was Fixed

- BUG-0035 — session status/inspect now survive agent-definition deletion while omitting only unavailable Heartbeat enrichment.
- BUG-0036 — Browser Back now opens the unsaved-changes resolver and preserves the draft when the operator keeps editing.
- BUG-0037 — daemon-served web E2E always builds current assets instead of trusting an existing `web/dist/index.html`.
- BUG-0038 — topbar slots are owner-scoped; inactive parents and late cleanups cannot erase destination controls.

## Paper Cuts

- The Filters primitive exposes field/value choices as ARIA `option`; the stale E2E `menuitem` selector was corrected to the actual accessibility contract.
- The deterministic topbar PNG is 11.6 KiB because the story is mostly a flat canvas; it was inspected directly and contains the expected title, count, search, and action.

## Runtime Errors Observed

- The initial web E2E run served a stale bundle and produced 15 cross-area timeouts. BUG-0037 fixed the harness; a fresh-bundle run reduced this to current-contract failures, and the final lane passed 70/70.
- The first QA daemon binary used an invalid default fixture model. The lab was rebuilt with the advertised acpmock model before product verdicts; this was setup correction, not a product defect.

## Human Verifications Needed

None identified yet.

## Decisions for a Human

None identified yet.

## Learnings

- File existence is not a freshness proof for daemon-served frontend assets; the release gate must build the browser artifact it evaluates.
- Shared shell slots require explicit ownership because parent and child passive effects can publish in the reverse order developers intuit from the tree.
- Empty-state E2E must control the API boundary explicitly when the runtime always contributes global definitions.

## QA Bootstrap

```text
[QA_BOOTSTRAP]
manifest_path=/Users/pedronauck/dev/qa-labs/agh-agent-details-task-09-20260711-190957-738761-lab/qa-artifacts/qa/bootstrap-manifest.json
lab_root=/Users/pedronauck/dev/qa-labs/agh-agent-details-task-09-20260711-190957-738761-lab
runtime_home=/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/aghqa-3a9e6b102541/runtime
base_url=http://127.0.0.1:63173
verification_report=/Users/pedronauck/dev/qa-labs/agh-agent-details-task-09-20260711-190957-738761-lab/qa-artifacts/qa/verification-report.md
health_status=fresh
[/QA_BOOTSTRAP]
```

## Automated Verification

- `CGO_ENABLED=1 go test -race ./internal/api/core -count=1`: pass.
- Test-conventions checker for the canonical core regression: pass.
- Turbo `@agh/ui` + web typecheck/test, forced uncached: 100 UI files / 508 tests and 381 web files / 3,135 tests passed.
- React Doctor against `HEAD`: 100/100 for `@agh/ui` and `agh-web`.
- `make test-e2e-web`: 70/70 pass after fresh-build and root-cause remediation.
- `make test-e2e-runtime`: pass.
- `make verify`: pass in detached clean worktree `/tmp/a9v-1`; the worktree was removed afterward.

## Teardown

`/Users/pedronauck/dev/qa-labs/agh-agent-details-task-09-20260711-190957-738761-lab/qa-artifacts/qa/teardown.json` records `"clean": true`, no survivors, and all registered daemon/Vite/browser processes terminated. The later screenshot Storybook was also stopped and port 6006 verified free.

## Final Status

- **Exit gate (full automated suite):** Pass
- **Issues by user impact:** Blocks-Completion 3 fixed · Data-Loss 1 fixed · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 3/3 journeys walked; 12/12 scoped tracker rows terminal
- **Verdict:** pass after fix-loop — all four charters are terminal, both official E2E lanes pass, the full monorepo gate passes, and teardown is clean.
