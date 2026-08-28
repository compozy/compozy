# QA Run Report — 2026-08-27 — agent-comms remediation

- **Scope:** Targeted verification of deadline-less agent calls, durable result delivery, public call contracts, Activity and Calls projections, transcript turns, and visual contracts.
- **Cadence tier:** targeted
- **Build:** `9f269e57c` plus the remediation working tree / PR #497
- **Environment:** isolated lab `agent-comms-northstar-pay-profile-fix-20260828-043732-348374`
- **Status:** partial — local runtime E2E passed; final Web E2E and repository gate delegated to PR CI by operator direction

## Results

| Surface | Result | Evidence |
|---|---|---|
| Runtime E2E | Pass | Fresh `make test-e2e-runtime`: daemon, HTTP, UDS, CLI remote, and race-enabled integration suites passed |
| Northstar Pay scenario | Partial | 12/12 assigned tasks completed across 11 agents and 10 channels; two disruption probes did not reach the intended decision-maker because of scenario topology |
| Browser smoke | Pass | Desktop and mobile catalog, Activity, PM transcript, Calls panel, and 320 px layout walked in the isolated lab |
| Visual contracts | Pass | 20/20 bundles validated with reference, implementation, side-by-side, diff, comparison metadata, and review notes |
| Web E2E | CI-owned | The local lane ended without a retained exit result; no surviving Playwright, Vite, Storybook, browser, Whisper, or QA-lab processes remained |
| Final scoped gate | CI-owned | Operator directed the branch to be pushed and validation to continue in CI |

## Scenario Matrix

| Scenario | Status | Notes |
|---|---|---|
| `RT-agent-call-cancel` | Untested | Reset for the changed behavior; CI provides automated coverage, but the public QA walk remains open |
| `RT-call-record-terminal-states` | Untested | Reset for the changed behavior; runtime E2E passed |
| `RT-call-wake-delivery-exactly-once` | Untested | Reset for the changed behavior; runtime E2E passed |
| `RT-delegation-activity-tree` | Untested | Browser smoke verified the surface, not the complete scenario |
| `RT-in-context-call-messages` | Untested | Browser smoke verified the transcript surface, not the complete scenario |
| `RT-session-calls-inspector-panel` | Untested | Browser smoke verified the panel at desktop and mobile widths, not the complete scenario |

## Findings and Fixes

- A live-network publish from a session absent from participation surfaced as a generic gateway failure. The bridge now preserves the transport cause while mapping it to the typed `call_publish_no_participation` error.
- `call_publish_no_participation` returned HTTP 409 despite the public contract requiring 422. The HTTP mapper and its canonical handler suite now use 422.
- Runtime E2E fixtures assumed that asynchronous call acceptance immediately returned a child session ID. They now wait on the durable call ID and observe the child binding when it becomes available.
- Profile-aware test resolvers did not retain the selected provider configuration. The integration resolvers now implement profile resolution explicitly.
- Hosted MCP diagnostics could be read before the session-specific server record was written. The existing E2E helper now waits for that observable record within the test context.

## Process Teardown

The isolated lab teardown completed with `"clean": true` in `qa-artifacts/qa/teardown.json`. A final process check found no surviving QA daemon, Web server, Playwright, Storybook, reference server, browser, or Whisper process owned by this run.

## Compozy Impact Audit

- **Native tools:** Call tool IDs and schemas are unchanged. Runtime E2E checked call creation, await, cancel, return, result publication, nested depth, and hosted MCP availability. The no-participation error now matches the existing public contract.
- **Extensibility and hooks:** No registry, hook, extension, capability, bridge SDK, MCP descriptor, or config key was added. The existing Network publish bridge now translates a missing live sender into the established participation error while retaining the underlying cause.
- **Workspace data isolation:** Call and message data remain workspace-scoped. Runtime E2E exercised CLI, HTTP, UDS, store, and Network paths with explicit workspace IDs; this remediation adds no global or cross-workspace state.
- **Official Compozy skill:** No tool ID, CLI path, hook event, capability, extension resource, or task/memory/network workflow changed; `skills/compozy/` requires no update.

## Verdict

The runtime remediation and visual-contract evidence are ready for CI. User-facing QA scenarios remain honestly marked `untested` until their complete public-interface walks are recorded. PR CI owns the Web E2E and final repository validation for the pushed head.
