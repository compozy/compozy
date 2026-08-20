---
id: RT-home-zero-inventory-first-run
area: RT
title: Home offers one honest start when nothing has run
persona: Lea
journey: J-operate-home-dashboard
expected: When `GET /api/observe/overview` reports no work at all — every counter zero, `has_live_work` false, no pulse bucket, busiest and longest_session absent — Home replaces the seven zones with one heading ("No agent work yet", or "No agent work yet in <workspace>" in a project scope) and exactly three actions that really exist: Start a session (opens the session-create dialog in place), Create a task (`/tasks/new`), Browse the marketplace (`/marketplace/skills`); no zero-filled panels, no subtitle paragraph, and the seven zones return as soon as any counter, live work, or pulse bucket is non-zero.
entry_points: web `/` (dashboard OS window) on a fresh install or an empty project workspace; `GET /api/observe/overview` (HTTP+UDS)
qa_status: skipped
bug_ids: BUG-20260820-global-home-deleted-onboarding
fix_status: fixed
retest_status: pending
fix_commits: e520f3fe
evidence: docs/qa/reports/2026-08-20-ui-normies-retry.md
last_report: docs/qa/reports/2026-08-20-ui-normies-retry.md
overlaps: RT-home-dashboard-zones
---

story: As someone opening Compozy for the first time I see one thing I can do, in my own words, instead of ten panels reporting zero.

New behavior introduced by the normie-friendly UI foundation pass (2026-08-20,
`docs/prompts/normie-friendly-ui-pass.md` D3): `home-first-run.tsx` renders through
`CatalogEmptyState` and `home-overview-empty.ts` owns the predicate.

The predicate is deliberately stricter than "today's counters are zero" — `has_live_work` and the
pulse window are part of it, so a fresh install with an agent already working, or with activity
outside the today/outcomes windows, keeps the seven zones. Telling that person nothing has happened
would be untrue. Walk both sides: the honest zero state, and each of the four ways it must NOT
appear (live work, a pending approval, a run closed today, a pulse bucket outside today).

The three actions are the walk's real subject — each must land somewhere that exists and accepts
work, not a dead route. `RT-home-dashboard-zones` owns the populated read (as Cora); this file owns
the zero read and the transition between them, under Lea, because a workspace with nothing in it is
by definition a first-run surface and New User is the persona that owns first-run.

**Cycle notes — taxonomy sweep (branch cycle 2026-08-20, normie-friendly UI foundation pass).**

2026-08-20 retry: skipped by explicit user instruction after the onboarding defect was fixed and
scoped checks passed. No behavioral pass is claimed; same-persona retest remains pending.
Seven journeys in scope: J-operate-home-dashboard, J-operate-desktop-shell, J-19,
J-administer-runtime-settings, J-14, J-answer-agent-requests, plus J-start-from-empty-catalogs as
the adjacent canary. Dimensions 1–4 ride the charters listed below. Deliberate skips, recorded so
they do not read as covered:

- **Continuity (cross-device / cross-session)** — skipped on all seven. Compozy promises no
  cross-device continuity; `personas.md` already records mobile as a device lens on Marina and the
  desktop shell as desktop-only.
- **Responsiveness** — covered on J-operate-desktop-shell and J-operate-home-dashboard, the two
  layout-dense surfaces the 13.5px→15px baseline reflows. Skipped on J-14 and
  J-answer-agent-requests: full-bleed single-column transcript, no breakpoint logic touched.
- **The `daemon` → `CompozyOS` word sweep** across gateway, vault, marketplace, bridges, and network
  surfaces did not reset those scenarios. It is a COPY.md §6 surface-alias swap; the canonical noun
  is unchanged in code, wire, CLI, and API, and no scenario's `expected:` asserts the word.
- **The docs site** (`packages/site` eyebrow 11→12px, micro, icon-well radius, and the Fumadocs dark
  ramp now derived from runtime tokens) is out of this cycle's persona walks.
  `ET-site-docs-typography-opendesign` asserts site type, which is unchanged; no scenario asserts the
  ramp. `--audit-site` wired into `codegen-check` is the mechanical gate. Recorded as a follow-up.
- No `automation-backlog/` entry — this cycle changes presentation, not a journey's stability
  profile.
