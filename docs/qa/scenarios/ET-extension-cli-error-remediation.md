---
id: ET-extension-cli-error-remediation
area: ET
title: Recover from actionable extension CLI failures
persona: Ada
journey: J-extension-distribution
expected: Policy-blocked install and daemon-unavailable failures print human remediation and a `try:` command identical to the diagnostic carried by JSON output.
entry_points: compozy extension install; compozy status; -o json
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-cli-extension-sideload-policy-block; ET-022
---

Added by ext-improvs Task 07 for the shared root execution-error renderer. Walk both failure paths
with the stamped binary and compare the authored title, message, and suggested command across output
modes.
