---
id: ET-app-token-management
area: ET
title: Audit and revoke app credentials through local management
persona: Dora
journey: J-authorize-browser-app
expected: CLI and local HTTP/UDS expose matching token metadata, show a secret only at creation, replace origins durably, and make revocation idempotent without affecting paired devices.
entry_points: compozy app-token; /api/app-tokens over HTTP and UDS
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

Create one named token with an origin and one without. Save only redacted metadata. List active and all tokens and compare CLI with local HTTP/UDS. Replace origins and rename, then clear origins; verify fresh reads and old-origin denial. Revoke, repeat revoke, and verify changed true then false; updating a revoked token returns 409. Try unknown ID, invalid state, conflicting clear/origin flags, invalid origins, and blank name; confirm stable codes and no unintended record. Compare device list before and after.

QA impact 2026-10-17: App SDK implementation adds this user-visible journey. Authored planning only;
no live execution verdict is recorded. Execute in the assigned isolated lab and attach current-head
evidence before marking it passed.
