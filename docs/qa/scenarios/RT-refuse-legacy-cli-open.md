---
id: RT-refuse-legacy-cli-open
area: RT
title: Refuse legacy databases on remaining local CLI opens
persona: Ada
journey: J-operate-daemon-schema
expected: Local extension and provider-auth database opens return exactly one parseable JSON error document with diagnostic.code legacy_database, the surface, canonical path, and whole-family preservation and missing-upgrade guidance; MCP auth uses the daemon and does not open the database from the CLI process.
entry_points: compozy extension list -o json; compozy provider auth status <bound-secret-provider> -o json
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-rt-current-source-20260730-20260730-061631-252740-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: RT-refuse-legacy-database
---

The 2026-07-12 cycle passed all three former direct-open families with the earlier recovery copy. Peer-review
remediation changed that public contract to whole-family preservation, so the scenario was reset. Marketplace
Task 04 then moved MCP auth behind the daemon; extension and provider auth remain the direct-open coverage.

Policy revalidation (2026-09-04): SD-013 requires lossless user-state upgrades. Refusal with byte preservation remains necessary when no supported converter exists; selecting a fresh home does not satisfy migration. The current diagnostic/recovery path must be reconciled with this policy and re-walked before passing this scenario. This editorial audit does not claim that a legacy converter was implemented.
