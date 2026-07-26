---
id: RT-compozy-global-database
area: RT
title: Create the Compozy global database without changing session storage
persona: Bruno
journey: J-operate-daemon-schema
expected: A fresh daemon creates compozy.db in the resolved home, keeps each session database named events.db, reports the global path consistently, and never creates or opens compozy.db.
entry_points: compozy daemon start; compozy status -o json; global and session database files on disk
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-refuse-legacy-database;RT-compozy-home-layout
---

QA impact 2026-07-26: the global database filename moved to `compozy.db` while the
brand-neutral per-session `events.db` contract stayed unchanged. Planning flag only;
the next QA cycle owns fresh-home inspection and structured-path parity.
