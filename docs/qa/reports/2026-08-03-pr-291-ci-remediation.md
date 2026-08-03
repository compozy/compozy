# QA Run Report — 2026-08-03 — PR 291 CI remediation

- **Scope:** Non-Apple primary-modifier route pop plus renderer/authority and steady-state performance synchronization.
- **Cadence tier:** targeted
- **Build:** working tree after `b43bfd35` · **Environment:** isolated lab `compozy-pr-291-ci-remediation-20260803-100709-872317-lab`, daemon `:58648`, current Web `:3000`
- **Started:** 2026-08-03T07:06:33-03:00 · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | fresh isolated workspace | desktop / wifi-fast / en-US | CH-window-tabs-keyboard-flow |
| Cora | fresh isolated workspace | laptop / wifi-fast / en-US | CH-window-tabs-home-canary |

## Flows in Scope

- `J-organize-tabbed-work` — preserve independent tab routes while grouping and navigating by keyboard.
- `J-operate-home-dashboard` — adjacent shell canary after tab navigation.

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-window-tabs-keyboard-flow | J-organize-tabbed-work / ET-window-tab-deck-lifecycle | Bruno | Feature Tour | Pass | Browser driver cannot encode modified BracketLeft; exact chord covered by Linux unit/E2E evidence | this remediation batch |
| 2 | CH-window-tabs-home-canary | J-operate-home-dashboard / RT-home-usage-window-persistence | Cora | Feature Tour | Pass | | not applicable |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-window-tabs-keyboard-flow — Bruno

- **Ran:** 2026-08-03T07:08:00-03:00 → 2026-08-03T07:16:00-03:00 (box respected: yes)
- **Findings:** No product failure. The browser driver accepted modified `[` commands but did not emit the `BracketLeft` event; the public Back control exercised the same route-pop command instead.
- **Bugs filed/updated:** none.
- **Scenarios settled:** ET-window-tab-deck-lifecycle → pass.
- **Paper cuts:** none.
- **Surprises:** Command-T was encoded correctly by the same driver, so this is specific to the bracket key representation.
- **Suggested next charter:** repeat the exact Control-[ leg on native Linux if a non-Playwright browser becomes available.

Bruno created `Draft release notes`, opened its detail, returned to the list through the public Back
action, and confirmed the task after reload. Command-T created a Tasks + General deck; reload kept
both tabs and the task. Evidence: `docs/qa/evidence/2026-08-03-pr-291-ci-remediation/CH-window-tabs-keyboard-flow-route-pop.png`
and `docs/qa/evidence/2026-08-03-pr-291-ci-remediation/CH-window-tabs-keyboard-flow-deck-reload.png`.

### CH-window-tabs-home-canary — Cora

- **Ran:** 2026-08-03T07:16:00-03:00 → 2026-08-03T07:18:00-03:00 (box respected: yes)
- **Findings:** none.
- **Bugs filed/updated:** none.
- **Scenarios settled:** RT-home-usage-window-persistence remained pass as an unchanged adjacent canary.
- **Paper cuts:** none.
- **Surprises:** the queued task moved from Working now to the empty state while the window remained live, without stale counts.
- **Suggested next charter:** none for this remediation.

Cora switched Home from 30d to 90d and reloaded. The summary remained truthful at `Usage · 90d`,
reported cost unavailable, and kept the live shell connected. Evidence:
`docs/qa/evidence/2026-08-03-pr-291-ci-remediation/CH-window-tabs-home-canary.png`.

## What Was Fixed

- The fixed primary navigation chord now resolves to Command on Apple platforms and Control elsewhere.
- Split-drop automation waits for the renderer to apply the authoritative tiled placement before deriving pointer coordinates.
- The 12-window peer measurement waits for all surfaces to hydrate and reach a quiet steady state before recording convergence work.

## Paper Cuts

None recorded.

## Runtime Errors Observed

No browser errors. Console output contained only React DevTools development notices. One
`Live layout disconnected` toast appeared immediately after an early reload and self-recovered;
the verdict captures were taken after the live layout reconnected.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- Modified bracket keys are not faithfully encoded by this `agent-browser` build; exact shortcut
  dispatch remains an automated platform-contract check, while dogfooding can independently walk
  the resulting public navigation path.
- The 12-window envelope passed 10/10 serial repetitions: restore `20.6–26.1 ms`, worst drag long
  task `0 ms`, and worst peer-convergence long task `0 ms`.
- The strict release-scenario auditor was run and correctly rejected this targeted Web lab because
  it has two personas, one browser channel, no live provider session, no CLI/API journey, and no
  release-level `make verify` evidence. Its report remains in the lab scratch evidence; those
  requirements do not replace this branch-level browser walk.

## Compozy Impact Audit

- **Native tools:** no impact; checked `compozy__*` IDs, toolsets, descriptors, schemas, risk flags, and capability gates. This batch changes Web keyboard dispatch and canonical E2E synchronization only.
- **Extensibility and hooks:** no impact; extensions, hooks, skills/capabilities, tools/resources, bundles, registries, bridge SDKs, MCP sidecars, and config lifecycle are unchanged.
- **Workspace data isolation:** no ownership or propagation change; the route pop remains scoped to the focused window in its existing workspace-bound manager.
- **Official Compozy skill:** no impact; public tools, CLI paths, hook events, capabilities, bundles/resources, and memory/network/task semantics are unchanged.

Web/Docs impact: `web/` keyboard behavior and its canonical unit/E2E suites changed. `packages/site` has no impact because this restores the documented portable primary-modifier contract; the QA tracker and report record the behavior change.

## Final Status

- **Exit gate (full automated suite):** owned by the workflow checkpoint after this QA report; the report is intentionally not rewritten after the content-addressed gate record is created.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 2/2 targeted journeys walked; exact non-Apple Control-[ input is covered by the Linux-platform unit regression and E2E-032 because the browser driver could not encode BracketLeft.
- **Verdict:** QA pass; code readiness remains pending until the workflow records a passing affected gate.
- **Teardown:** pass — `qa-artifacts/qa/teardown.json` reports `clean: true`, killed registered daemon PID `17522` and Web PID `17867`, and found zero survivors.
