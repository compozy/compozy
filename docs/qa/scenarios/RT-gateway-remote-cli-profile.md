---
id: RT-gateway-remote-cli-profile
area: RT
title: Operate and repair a paired remote CLI profile
persona: Iris
journey: J-operate-remote-gateway-cli
expected: Pairing or passphrase-protected identity transfer creates one secret-safe HTTPS profile whose selected target preserves structured output, operation-matrix refusal, reconnects, actionable unreachable-versus-revoked errors, and atomic removal.
entry_points: compozy pair mint; compozy connect add|list|use|remove|export|import; compozy open; remote-capable CLI commands; direct HTTPS Gateway API
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-gateway-paired-device; RT-connectivity-provider-route
---

Own the remote CLI profile journey across pairing, encrypted export/import, persistence, target
selection, structured output, SSE and WebSocket reconnect, concurrent clients, copied-identity
revocation, local-only refusal, and profile removal. Exercise reachability, authentication,
revocation, interruption, wrong passphrase, duplicate-name, and cleanup errors without exposing a
credential.

QA impact 2026-08-06: added for remote-gateway Task 05. Flag only; Tasks 08–09 own the walk.
