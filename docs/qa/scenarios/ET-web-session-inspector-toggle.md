---
id: ET-web-session-inspector-toggle
area: ET
title: Context sidebar opens and closes from the composer
persona: Bruno
journey: J-14
expected: A fresh session defaults to the full-width thread. The composer context button is the one control that opens and closes the Context sidebar, and it reads pressed (`aria-pressed`, held plate) while the sidebar is open; an ordinary session's head has no context toggle, except a read-only superseded session, which has no composer and keeps it. An always-on main has no composer context control, so its head keeps a Close context sidebar toggle only while the sidebar is open. Open/closed preference persists per sessionId; inline rail at 1440px and drawer below; Escape or drawer dismissal updates the shared preference. No Usage, Memory, Files or Vault tabs remain.
entry_points: web session window composer context control; SessionInspector; localStorage key compozy:session:inspector:v2
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-session-sidebar-parent-20260806-212647-734931-lab/qa-artifacts/qa/journey-log.jsonl;docs/qa/reports/2026-09-12-session-context.md
last_report: docs/qa/reports/2026-09-12-session-context.md
overlaps: RT-052; RT-024; ET-web-route-chrome-topbar
---

Added by session inspector default-closed + topbar toggle (2026-07-22). Flag only — retest in the next QA cycle.

2026-07-26 state-ownership impact flag: inspector preference moved to the XState Store persistence extension and hard-cut to the `compozy:session:inspector:v2` key while retaining per-session behavior. The scenario remains untested for the next QA cycle.
2026-08-06 session-sidebar impact flag: a PanelLeft sessions-sidebar toggle joined the topbar actions left of the goal action; inspector behavior itself is unchanged. Reset to untested for the next QA cycle.

2026-08-06 re-walked live: inspector still defaults closed and toggles from the topbar; the new sessions-sidebar toggle (dock sessions icon) sits in the same actions cluster without displacing it. Per-session persistence retained. Evidence: lab journey-log.jsonl. Verdict: pass.

2026-09-12 session-context task_03 impact: verify the tab-less Context sidebar, preserved cost provenance, and both shared-preference openers. Ledger health/status/inspect remain public API surfaces; Vault remains in its own window; the transcript changed-files roll-up remains unchanged. Reset to untested; implementation checks do not replace the final task_06 live walk.

QA 2026-09-12: session-context final feature pass; runtime, focused integration/browser and 43 visual-pair evidence are separated in the linked report. Unchanged lifecycle/roll-up behavior reuses the earlier owning evidence; this pass verifies the new Context surface and its coexistence.

2026-09-12 design-parity pass: the composer control is the board's 24px ring-only trigger (dotted stale arc, dashed unknown track, rail-open plate) and the Context rail follows `docs/design/opendesign/session-context/` (three-tier legend with square swatches, meter line with clock/threshold, chevron-first CompozyOS context head with indented rows, canvas tiles and turn card, compaction marker strip with the window figure, compact empties, drawer header matching the inline rail). Openers, preference, breakpoint, and section order are unchanged; owning unit suite and the focused browser E2E re-verified. Behavior evidence above remains valid; visual captures refreshed in `.compozy/tasks/session-context/evidence/visual/`.

QA impact 2026-11-11 (#1487): the topbar PanelRight toggle left an ordinary session's head; the composer context control now toggles the sidebar (before, a second click did nothing) and carries the pressed state. An always-on main keeps the head toggle only as the close control of a sidebar left open. Below 1440px the sidebar is a drawer over the window, dismissed by its own close button or Escape, as before.

2026-11-11 isolated-lab walk (branch daemon and web bundle, acpmock provider, temporary `COMPOZY_HOME`): at 1600px the composer control opened the inline sidebar (`aria-pressed=true`) and a second click closed it (`aria-pressed=false`); the head showed no context toggle. On the always-on main, a sidebar left open showed Close context sidebar in the head, and it closed the sidebar. At 1280px the drawer opened from the composer and closed with Escape. The same walk on the base bundle left the sidebar open after the second click. Unit coverage: `session-inspector.test.tsx` (keyboard open/close, pressed state, shared preference) and `use-session-topbar-slot.test.tsx` (no head toggle). Verdict: pass.
