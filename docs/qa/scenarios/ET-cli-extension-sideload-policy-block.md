---
id: ET-cli-extension-sideload-policy-block
area: ET
title: Block unverified extension side-loads through structured CLI output
persona: Ada
journey: J-extension-policy-admin
expected: With `extensions.trust.allow_unverified=false`, local side-loads fail before mutation and both human and JSON output carry the stable Settings › Extensions remediation and config command; enabling policy still requires request consent.
entry_points: compozy extension install; compozy config set extensions.trust.allow_unverified; Settings › Extensions
qa_status: blocked-verify
bug_ids: BUG-20260715-extension-cli-slow-boot-offline
fix_status: fixed
retest_status: pass
fix_commits: 8eeb8a38
evidence: /Users/pedronauck/dev/qa-labs/compozy-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/extension-cli-slow-boot-reachability.json; docs/qa/reports/2026-07-15-marketplace.md;/Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: ET-017; ET-018; ET-023; ET-049
---

Added by marketplace Task 05. QA should prove both consent levels, live apply without a daemon restart, deterministic JSON diagnostics over the daemon transport, and absence of install files or registry rows after a policy block.

QA impact 2026-07-29: human stderr now preserves the same authored remediation and suggested
command as JSON output, and the legacy config key was hard-cut; reset for the next QA cycle.
