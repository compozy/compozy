# QA Run Report — 2026-10-28 — Subagent drawer

- **Scope:** issue #1003; inspect delegated and provider-native subagents while preserving the parent conversation.
- **Cadence tier:** targeted.
- **Build:** `feat/issue-1003-subagent-drawer`; exact head and hosted evidence are recorded in the PR.
- **Environment:** isolated daemon home, socket and ports; real Web application; official acpmock delegation fixtures and pinned Claude Agent adapter frames.
- **Started:** 2026-10-28T21:04:00Z · **Status:** targeted walks passed; PR CI/review pending.

## Personas

| Persona | Device / Network / Locale | Sessions |
| --- | --- | --- |
| Bruno | Desktop / local / en-US | Session sidebar navigation |
| Rafa | Desktop and narrow viewport / local / en-US | Native activity inspection |

## Flows in Scope

- J-13: follow live subagent work without replacing the parent conversation.
- J-14: inspect retained delegated and native work, then dismiss or explicitly open a child session.
- Existing sidebar charter `CH-session-sidebar-navigation`, bounded to the subagent branch.
- Native/transcript walk follows the changed steps in `ET-web-native-subagent` and `ET-web-subagent-card`.

## Session Matrix & Results

| Session | Scenario | Persona | Tour | Status |
| --- | --- | --- | --- | --- |
| Sidebar and inspector | ET-web-session-sidebar-threads | Bruno | Feature Tour | Passed |
| Transcript and composer | ET-web-subagent-card | Bruno | Feature Tour | Passed |
| Native activity | ET-web-native-subagent | Rafa | Feature Tour | Passed |

## Implementation Verification

The first integrated run exposed duplicate assistant-ui toolkit registration in nested runtimes.
The drawer now creates an isolated assistant-ui root. Screenshot inspection also found the inline
portal could initially attach outside its panel; its portal now waits for the actual host element.
The resize separator lives within the dialog focus boundary. Existing tests own these invariants.

The 1000px walk exposed an existing responsive feedback loop: measuring the dock-reduced work area
switched the shell to compact mode, which moved the dock below the desktop and widened the work
area enough to switch it back. The shell now records viewport width separately and uses it for the
presentation policy. A pure projection regression covers both dock geometries.

Full local gates, race lanes and broad E2E suites are intentionally excluded by the operator's
standing rule. CI owns the full delivery matrix. Only changed-package checks and the two affected
Playwright specs run locally.

## Evidence and Remaining Work

- Web: 143 focused tests passed across the five owning suites.
- Shared UI: 3 drawer tests passed (state preservation, focus return, keyboard width limits).
- Both affected Playwright specs: all eight journeys passed across the final scoped runs. The final
  two-journey rerun covered pointer resize, reopening width, unsent draft preservation, all delegated
  entry surfaces, narrow overlay, explicit session navigation and modifier split.
- Package-scoped Turborepo typecheck/lint passed with zero warnings/errors.
- Light/dark component matrices cover queued, working, waiting, done, idle, failed, canceled,
  interrupted and native states. Real Web captures cover all four entry surfaces and native activity.
- The separate manual lab was torn down with the bootstrap helper (`TEARDOWN_ALL_CLEAN=true`).
  Browser journeys use the fixture's independent daemon homes and teardown.
- The PR gallery hosts the inspected final rendered evidence. CI and reviewer status are reported
  there; this report does not substitute local checks for current-head CI.
Pinned Claude adapter frames exercise native attribution and rendering; they do not claim a live
Claude model invocation or re-certify every background lifecycle branch of the broader scenario.
