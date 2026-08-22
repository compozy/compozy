---
id: ET-profile-scoped-work-reads
area: ET
title: Keep ordinary work reads inside the resolved profile
persona: Ada
journey: J-operate-profiles
expected: CLI, HTTP, UDS, and native reads only return work owned by the resolved or session-bound profile, and a foreign-profile detail read returns not found.
entry_points: root --profile; compozy session|task|automation|bridge|network; HTTP/UDS work routes; compozy__session_list
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-profile-aggregate-owner-labels; ET-profile-deep-link-owner; ET-profile-stream-isolation
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
