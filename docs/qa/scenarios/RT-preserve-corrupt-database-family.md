---
id: RT-preserve-corrupt-database-family
area: RT
title: Preserve a corrupt database family
persona: Bruno
journey: J-operate-daemon-schema
expected: Startup or a session read refuses the corrupt database with its path while the database, WAL, and SHM bytes remain identical and no quarantine or replacement file appears.
entry_points: agh daemon start --foreground; agh session events <session-id>
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-refuse-legacy-database;RT-refuse-ahead-database
---

Flagged after the store opener stopped performing context-free automatic corruption recovery. The next targeted
cycle should hash the complete database family before and after each refusal and inspect the filesystem for
unexpected `.corrupt.*` files or fresh replacement databases. No QA session ran as part of this change.
