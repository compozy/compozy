---
id: RT-gateway-remote-cli-profile
area: RT
title: Operate a remote daemon through a paired CLI profile
persona: Ada
journey: J-operate-daemon-schema
expected: Pairing creates one secret-safe HTTPS profile whose selected target preserves structured output, local and remote command parity, live-stream reconnects, revocation isolation, and atomic removal without granting local-only authority.
entry_points: compozy pair mint; compozy connect add|list|use|remove; compozy open; remote-capable CLI commands; direct HTTPS Gateway API
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-gateway-paired-device; RT-connectivity-provider-route
---

Own the remote CLI profile journey across pairing, persistence, target selection, structured output,
SSE and WebSocket reconnect, concurrent clients, copied-identity revocation, local-only refusal, and
profile removal. Exercise reachability, authentication, revocation, interruption, duplicate-name,
and cleanup errors without exposing a credential.

QA impact 2026-08-06: added for remote-gateway Task 05. Flag only; Tasks 08–09 own the walk.
