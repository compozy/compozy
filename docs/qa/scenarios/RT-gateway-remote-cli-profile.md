---
id: RT-gateway-remote-cli-profile
area: RT
title: Operate and repair a paired remote CLI profile
persona: Iris
journey: J-operate-remote-gateway-cli
expected: Pairing or passphrase-protected identity transfer creates one secret-safe HTTPS profile whose selected target preserves structured output, operation-matrix refusal, reconnects, actionable unreachable-versus-revoked errors, and atomic removal.
entry_points: compozy pair mint; compozy connect add|list|use|remove|export|import; compozy open; remote-capable CLI commands; direct HTTPS Gateway API
qa_status: blocked-verify
bug_ids: BUG-20260807-gateway-profile-recovery
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-remote-gateway-review-remediation-20260808-035328-688540-lab/qa-artifacts/qa/test-cases/42-pairing-artifact-handoff.json
last_report: docs/qa/reports/2026-08-08-remote-gateway-review-remediation.md
overlaps: RT-gateway-paired-device; RT-connectivity-provider-route
---

Own the remote CLI profile journey across pairing, encrypted export/import, persistence, target
selection, structured output, SSE and WebSocket reconnect, concurrent clients, copied-identity
revocation, local-only refusal, and profile removal. Exercise reachability, authentication,
revocation, interruption, wrong passphrase, duplicate-name, and cleanup errors without exposing a
credential.

QA impact 2026-08-06: added for remote-gateway Task 05. Flag only; Tasks 08–09 own the walk.

QA walk 2026-08-07: a TCP dial refusal returned the stable reachability code and, after the fix,
left no credential, journal, or profile while `connect list` remained usable. Real HTTPS pairing,
remote work, reconnect, export/import, and local-only refusal remain blocked without a provider address.

QA impact 2026-08-07: review remediation changed `compozy pair mint` to emit only a private
`artifact_ref`; reset for a focused handoff-output walk before workstream close.

QA walk 2026-08-08: the current CLI minted distinct private handoff files in human, JSON,
JSONL, and TOON modes. Every file used mode `0600`, every structured payload exposed only
`artifact_ref` plus `expires_at`, and no raw pairing bytes appeared in stdout or stderr. The
full remote-profile journey remains blocked until an authorized remote provider is available.
