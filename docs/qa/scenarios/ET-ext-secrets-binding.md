---
id: ET-ext-secrets-binding
area: ET
title: Bind extension secrets without exposing them
persona: Bruno
journey: J-extension-distribution
expected: A declared environment key binds to an existing Vault reference or hidden input at the exact extension instance scope, while list and every transport expose only bound key names.
entry_points: compozy extension secrets set|bind|list|unset; /api/extensions/:name/secrets over HTTP and UDS
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-019; ET-web-extension-detail
---

QA impact 2026-08-02: new secret-binding lifecycle. Walk global and workspace instances, update
survival, unset, missing declarations, and plaintext/reference redaction across output, logs, and events.
