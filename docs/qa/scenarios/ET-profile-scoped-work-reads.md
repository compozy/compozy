---
id: ET-profile-scoped-work-reads
area: ET
title: Keep ordinary work reads inside the resolved profile
persona: Ada
journey: J-scope-work-by-profile
expected: CLI, HTTP, UDS, and native reads only return work owned by the resolved or session-bound profile, and a foreign-profile detail read returns not found.
entry_points: root --profile; compozy session|task|automation; HTTP/UDS work routes; compozy__session_list
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-profile-aggregate-owner-labels; ET-profile-deep-link-owner; ET-profile-stream-isolation
---

Flagged by Profiles task 06. The final QA tasks own the real-user walk, evidence, and verdict.

Walk:

1. Create equivalent work under two profiles, including sessions, tasks, and automation records.
2. Read each surface through CLI, HTTP, and UDS under one profile and prove foreign rows are absent.
3. Read from a managed session and prove native results follow the session's immutable profile.
4. Request a known foreign item through a scoped detail route and verify the not-found contract.

Expected evidence: paired structured CLI responses, HTTP and UDS payloads, native-tool results, and the
foreign-detail error body.

Session route regression #1193: in one project, create sessions in profiles X and Y. Read X's
events and owner projection with `profile=X` (200), then `profile=Y` (404). Rename X with
`profile=X` (200), then `profile=Y` (404). `all_profiles=true` permits the events read but
returns `400 profile_selection_conflict` for rename and pull-request refresh. A request carrying
Y's agent-session identity without a selector must return 404 for X's events. An operator runtime
read without a selector still follows the identified session; existing detail/lifecycle defaults
remain profile scoped. Verify HTTP/UDS parity in the owning transport suite.

2026-11-03: this regression passed 12 real API/CLI checks in a fresh two-profile lab using the
branch binary; HTTP/UDS parity passed in `TestWorkspaceSessionProfileFenceHTTPUDSTransportParity`.
See [the targeted report](../reports/2026-11-03-issue-1193-session-profile-routes.md). This evidence
covers the session regression only; the broader scenario retains its existing status.

Regression #573: open a Loop run owned by a non-default Profile and verify its detail, briefing,
roster, timeline, requests, and event stream load. Switch to another Profile and confirm the run
and cached content disappear; direct reads of the three projections must return not found.
From All Profiles, invoke a run control and verify it targets the run owner.

Loop regression verified 2026-09-09 in the daemon-served browser suite:
`web/e2e/__tests__/loop-run.spec.ts`, “Profile-scoped Loop reads and owner-bound controls survive
reload and aggregate view”. The owning Profile can read detail, briefing, roster, and timeline;
default-Profile requests return 404. The run remains readable after reload. Reopening it from
All Profiles and confirming cancellation sends `profile=marketing` and reaches the canceled state.
The paired CLI journey `TestDaemonE2ELoopRunReadCLIJourneys` also passes with race detection.
This verifies the Loop regression; the broader non-Loop scenario above retains its own QA status.

2026-09-27 scope update: retired product surfaces were removed from this active scenario. Historical evidence remains in the dated reports; this revised contract requires a fresh walk.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.

CI1 regression coverage (2026-10-21): task reads in the same project still consult the authenticated
session policy when it is configured; cross-profile denial and grant remain authoritative. Same-profile
User task fixtures preserve lease, audit, blocking, and wake invariants. Session JSON/JSONL listings
preserve profile frames without fetching display-only workspace labels. Owning race suites pass;
see [the CI1 report](../reports/2026-10-21-all-projects-ci1.md). The broader walkthrough remains outstanding.
