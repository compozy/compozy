---
id: ET-ext-secrets-binding
area: ET
title: Bind extension secrets without exposing them
persona: Bruno
journey: J-extension-kit-lifecycle
expected: A declared environment key binds to an existing Vault reference or hidden input at the exact extension instance scope, while list and every transport expose only bound key names.
entry_points: /docs/extensions/secrets and CLI extension secrets reference; compozy extension secrets set --value-stdin|bind|list|unset [--workspace] -o json|jsonl|toon; GET|PUT|DELETE /api/extensions/:name/secrets[/env] over HTTP and UDS; compozy doctor --only extension
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
survival, stale declarations, unset/remove GC, deterministic 400 binding classes, and
plaintext/reference redaction across output, logs, events, SSE, and transcripts. No native tool may
write a secret.
