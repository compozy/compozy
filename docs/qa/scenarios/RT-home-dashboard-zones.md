---
id: RT-home-dashboard-zones
area: RT
title: Home dashboard renders seven truthful zones
persona: End user
journey:
expected: Home window (`/`) renders pagemeta → Needs you → KPI strip → Working now | Network → Pulse → Outcomes | Usage & cost → Agents | Activity → System, each backed by `GET /api/observe/overview` plus existing status/sessions/tasks/network/agents/logs reads; empty windows show honest empty states (no invented metrics); insights with no data are omitted; the head carries glyph + Live pill + one primary New session action and the body renders no H1.
entry_points: web `/` (dashboard OS window); `GET /api/observe/overview` (HTTP+UDS)
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: web/src/systems/dashboard/components/home-dashboard.tsx; internal/observe/overview.go; .compozy/tasks/dashboard-redesign/evidence/visual/home-dashboard/VC-01/review.md
last_report:
overlaps:
---

story: As an end user I open Home and see what agents did, what needs me, and what it costs — all from persisted daemon data.

New behavior shipped 2026-07-23 (dashboard redesign): zones replace the old daemon-status + 4-metric home. Verify zone order, truthful empties (no runs → axis-free empty chart; no attention → quiet "Nothing needs you right now"), pulse insights omission when the window is empty, and that magnitude charts stay neutral-ink while outcome bars alone use success/danger.
