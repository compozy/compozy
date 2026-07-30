---
id: RT-refuse-legacy-session-database
area: RT
title: Refuse incompatible session event databases on reads
persona: Bruno
journey: J-operate-daemon-schema
expected: Session event, transcript, watch, and ledger reads refuse legacy or ahead events.db files before querying or changing the database.
entry_points: compozy session events <session-id>; compozy session history <session-id>; session ledger materialization
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-rt-current-source-20260730-20260730-061631-252740-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: RT-refuse-legacy-database;RT-refuse-ahead-database
---

Implementation peer review found that read-only session consumers did not run the session migration-stream
preflight. They now share the package-owned read-only opener, which validates legacy markers, the embedded
checksum, and ahead versions without bootstrapping or mutating the database. The next targeted cycle must exercise
one public session read and the terminal ledger path against preserved incompatible `events.db` fixtures.
