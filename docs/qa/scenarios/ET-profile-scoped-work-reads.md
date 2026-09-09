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
From All Profiles, invoke a run control and verify it targets the run owner. This adds a QA check,
not a completed browser verdict.
