---
id: ET-profile-scoped-work-reads
area: ET
title: Keep ordinary work reads inside the resolved profile
persona: Ada
journey: J-scope-work-by-profile
expected: CLI, HTTP, UDS, and native reads only return work owned by the resolved or session-bound profile, and a foreign-profile detail read returns not found.
entry_points: root --profile; compozy session|task|automation|bridge|network; HTTP/UDS work routes; compozy__session_list
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-profile-aggregate-owner-labels; ET-profile-deep-link-owner; ET-profile-stream-isolation; NB-cross-profile-conversation
---

Flagged by Profiles task 06. The final QA tasks own the real-user walk, evidence, and verdict.

Walk:

1. Create equivalent work under two profiles, including sessions, tasks, automation records, bridge
   records, and network conversations.
2. Read each surface through CLI, HTTP, and UDS under one profile and prove foreign rows are absent.
3. Read from a managed session and prove native results follow the session's immutable profile.
4. Request a known foreign item through a scoped detail route and verify the not-found contract.

Expected evidence: paired structured CLI responses, HTTP and UDS payloads, native-tool results, and the
foreign-detail error body.

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
