---
id: RT-refuse-legacy-session-database
area: RT
title: Refuse incompatible session event databases on reads
persona: Bruno
journey: J-operate-daemon-schema
expected: Session event, transcript, watch, and ledger reads refuse legacy or ahead events.db files before querying or changing the database.
entry_points: agh session events <session-id>; agh session history <session-id>; session ledger materialization
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-refuse-legacy-database;RT-refuse-ahead-database
---

Implementation peer review found that read-only session consumers did not run the session migration-stream
preflight. They now share the package-owned read-only opener, which validates legacy markers, the embedded
checksum, and ahead versions without bootstrapping or mutating the database. The next targeted cycle must exercise
one public session read and the terminal ledger path against preserved incompatible `events.db` fixtures.
