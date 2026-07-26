---
id: RT-refuse-ahead-database
area: RT
title: Refuse a database ahead of the binary
persona: Bruno
journey: J-operate-daemon-schema
expected: Startup stops before normal work, while remaining local CLI opens return exactly one parseable JSON error document with diagnostic.code schema_ahead, the surface, canonical path, and newer-binary-or-complete-family remediation; MCP auth reaches the daemon instead of opening the database locally.
entry_points: agh daemon start --foreground; agh extension list -o json; agh provider auth status <bound-secret-provider> -o json
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-refuse-legacy-database
---

The 2026-07-12 cycle passed Safety Invariant 12 with the earlier recovery copy. Peer-review remediation changed
that public contract to whole-family preservation, so the scenario is reset for the next targeted cycle; migration
history must still never be repaired in place.
