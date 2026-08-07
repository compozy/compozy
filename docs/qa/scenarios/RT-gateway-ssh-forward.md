---
id: RT-gateway-ssh-forward
area: RT
title: Reach a remote daemon through a scoped SSH forward
persona: Bruno
journey: J-connect-gateway-ssh
expected: One SSH command verifies the remote binary, reuses or starts a loopback-only daemon, exposes a local-equivalent loopback forward, preserves the selected remote home, and tears down only the resources it owns.
entry_points: compozy connect ssh <host>; system OpenSSH configuration and agent; remote compozy daemon lifecycle; loopback-forwarded CLI commands
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-remote-gateway-20260807-202655-957508-lab/qa-artifacts/qa/test-cases/39-ssh-unreachable-no-mutation.json
last_report: docs/qa/reports/2026-08-07-remote-gateway.md
overlaps: RT-gateway-remote-cli-profile; RT-gateway-local-only-boot
---

Own the SSH bootstrap and teardown journey for an absent or running remote daemon. Exercise missing
Compozy, exact-version mismatch, changed host key, unreachable host, non-loopback remote listener,
tunnel loss, remote-home propagation, and interruption after accepted work. Confirm no Gateway
surface or provider state changes and no unrelated SSH master is terminated.

QA impact 2026-08-06: added for remote-gateway Task 05. Flag only; Tasks 08–09 own the walk.

QA walk 2026-08-07: an unreachable SSH target failed deterministically before creating a profile,
credential, process, or listener. Launch/reuse, remote-home propagation, accepted work, and scoped
teardown remain blocked because no authorized SSH host was available.
