---
id: MS-clarify-timeout-policy
area: MS
title: Configure clarification timeout policy with restart truth
persona: Dora
journey: J-administer-runtime-settings
expected: Dora sets tools.clarify.timeout to omitted, 0s, a finite value, or an invalid value and every surface agrees on the effective policy, the restart requirement, and the rejection with the last valid policy kept.
entry_points: compozy config get/set tools.clarify.timeout -o json; config.toml [tools.clarify]
qa_status: pass
bug_ids: BUG-20260917-clarify-timeout-config-set
fix_status: fixed
retest_status: pass
fix_commits: working-tree
evidence: docs/qa/reports/2026-09-17-clarify-keepalive.md
last_report: docs/qa/reports/2026-09-17-clarify-keepalive.md
overlaps: RT-session-clarification-roundtrip
---

Set the policy through each shape and prove the effective value after a daemon restart: omitted and
explicit `0s` both mean no automatic expiration; `1s`–`24h` (including the `5m` opt-back) keep the
existing deadline behavior; `-5s`, `500ms`, `25h`, `soon`, and `99h` fail with the exact
`tools.clarify.timeout must be between 1s and 24h, or 0s for no expiration: <value>` error while the
last valid policy stays active. Every read — CLI structured output, the config file, and the pending
clarification projection (`deadline: null` when unbounded) — agrees, and the change reports the
`restart-required` lifecycle; a policy flip never rewrites a live wait (creation-time pinning).

Minted 2026-09-17 by clarify-keepalive task 04: the config-lifecycle half of the unbounded-default
change, which the RT clarification round-trip cannot represent (that scenario owns the runtime wait,
not the admin policy lifecycle). Runtime waits, keepalive pings, races, extension inheritance, and
isolation stay with `RT-session-clarification-roundtrip`; IT-001
(`.compozy/tasks/clarify-keepalive/_tests.md`) covers loader→boot→broker propagation as automated
evidence, while task 05 walks this scenario live via `CH-clarify-policy-matrix`.
