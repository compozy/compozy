---
id: RT-refuse-legacy-cli-open
area: RT
title: Refuse legacy databases on remaining local CLI opens
persona: Ada
journey: J-operate-daemon-schema
expected: Local extension and provider-auth database opens return exactly one parseable JSON error document with diagnostic.code legacy_database, the surface, canonical path, and whole-family preserve-or-move/fresh-home remediation; MCP auth uses the daemon and does not open the database from the CLI process.
entry_points: agh extension list -o json; agh provider auth status <bound-secret-provider> -o json
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-refuse-legacy-database
---

The 2026-07-12 cycle passed all three former direct-open families with the earlier recovery copy. Peer-review
remediation changed that public contract to whole-family preservation, so the scenario was reset. Marketplace
Task 04 then moved MCP auth behind the daemon; extension and provider auth remain the direct-open coverage.
