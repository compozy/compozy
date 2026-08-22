---
id: ET-profile-stream-isolation
area: ET
title: Isolate session catalog streams by profile
persona: Ada
journey: J-operate-profiles
expected: Scoped session catalog initial state, replay, and live updates only contain sessions for the resolved profile; aggregate streams include profile_name on every session.
entry_points: session catalog SSE; HTTP/UDS session catalog stream; session create|rename|archive
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-profile-scoped-work-reads; ET-profile-aggregate-owner-labels
---

Flagged by Profiles task 06. The final QA tasks own the real-user walk, evidence, and verdict.

Walk:

1. Open scoped and aggregate catalog streams, then create sessions under two profiles.
2. Verify scoped initial state and live updates exclude the foreign profile.
3. Reconnect with a replay cursor and verify replay preserves the same boundary.
4. Verify every aggregate session row identifies its `profile_name`.

Expected evidence: timestamped initial, live, and replay frames from scoped and aggregate streams with
the corresponding session records.
