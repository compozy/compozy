---
id: RT-refuse-legacy-database
area: RT
title: Refuse a pre-Goose database without mutation
persona: Bruno
journey: J-operate-daemon-schema
expected: Startup stops before readiness, preserves the database byte-for-byte, and names the path plus remediation to stop AGH, preserve or move the complete containing AGH_HOME or workspace .agh family, and select a separate fresh home.
entry_points: agh daemon start; agh daemon start --foreground
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

The 2026-07-12 cycle passed byte preservation with the earlier single-file recovery copy. Peer-review remediation
changed that public contract to whole-family preservation, so the scenario is reset for the next targeted cycle.
