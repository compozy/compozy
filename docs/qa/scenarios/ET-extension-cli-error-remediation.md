---
id: ET-extension-cli-error-remediation
area: ET
title: Recover from actionable extension CLI failures
persona: Ada
journey: J-extension-distribution
expected: Policy-blocked install and daemon-unavailable failures print human remediation and a `try:` command identical to the diagnostic carried by JSON output.
entry_points: compozy extension install; compozy status; -o json
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-ext-improvs-final-20260729-230047-267985-lab/qa-artifacts/qa/extension-charters.json
last_report: docs/qa/reports/2026-07-29-ext-improvs.md
overlaps: ET-cli-extension-sideload-policy-block; ET-022
---

Added by ext-improvs Task 07 for the shared root execution-error renderer. Walk both failure paths
with the stamped binary and compare the authored title, message, and suggested command across output
modes.
