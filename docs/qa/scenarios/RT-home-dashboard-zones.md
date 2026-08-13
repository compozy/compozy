---
id: RT-home-dashboard-zones
area: RT
title: Home dashboard renders seven truthful zones
persona: Cora
journey: J-operate-home-dashboard
expected: Home window (`/`) renders pagemeta → Needs you → KPI strip → Working now | Network → Pulse → Outcomes | Usage & cost → Agents | Activity → System, each backed by `GET /api/observe/overview` plus existing status/sessions/tasks/network/agents/logs reads; empty windows show honest empty states (no invented metrics); insights with no data are omitted; the head carries glyph + Live pill + one primary New session action and the body renders no H1. Zone queries follow the menubar Global switch: Global (`~`) uses the home-scope filter; a project workspace uses that workspace id. Toggling Global does not invent a home row in the workspace menu.
entry_points: web `/` (dashboard OS window); `GET /api/observe/overview` (HTTP+UDS)
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: web/src/systems/dashboard/components/home-dashboard.tsx; internal/observe/overview.go; .compozy/tasks/dashboard-redesign/evidence/visual/home-dashboard/VC-01/review.md; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/037-home-dashboard
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps:
---

story: As an end user I open Home and see what agents did, what needs me, and what it costs — all from persisted daemon data.

New behavior shipped 2026-07-23 (dashboard redesign): zones replace the old daemon-status + 4-metric home. Verify zone order, truthful empties (no runs → axis-free empty chart; no attention → quiet "Nothing needs you right now"), pulse insights omission when the window is empty, and that magnitude charts stay neutral-ink while outcome bars alone use success/danger.

QA completion 2026-07-29: the isolated Home window rendered all seven zones in order from a fresh
`observe-overview/v1` read. No-attention, no-live-work, and no-outcome states were truthful; usage
showed retained tokens without inventing unavailable cost; System exposed the degraded daemon status.
The window head kept its glyph, live connection indicator, and one primary New session action, while
the dashboard body contained no H1. HTTP and UDS normalized to the same overview payload.

2026-08-12 qa-impact: Home scope follows the menubar Global switch (store `scope`), not home-dir detection. Reset to untested.

2026-08-12 walk: blocked-verify. This implementation cycle captured Storybook visual-contract evidence (`.compozy/tasks/global-workspace-menubar/evidence/visual/menubar-toggle/VC-01`–`VC-04`) and unit/typecheck coverage. An isolated QA lab with a live daemon (`COMPOZY_HOME`, production-parity web) was not started, so a persona walk through public entry points could not meet the qa-execution evidence standard.
