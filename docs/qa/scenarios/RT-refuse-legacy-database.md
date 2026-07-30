---
id: RT-refuse-legacy-database
area: RT
title: Refuse a pre-Goose database without mutation
persona: Bruno
journey: J-operate-daemon-schema
expected: Startup stops before readiness, preserves the database byte-for-byte, and names the path plus remediation to stop Compozy, preserve or move the complete containing COMPOZY_HOME or workspace .compozy family, and select a separate fresh home.
entry_points: compozy daemon start; compozy daemon start --foreground
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-rt-current-source-20260730-20260730-061631-252740-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps:
---

The 2026-07-12 cycle passed byte preservation with the earlier single-file recovery copy. Peer-review remediation
changed that public contract to whole-family preservation, so the scenario is reset for the next targeted cycle.
