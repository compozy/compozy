---
id: MS-clarify-timeout-policy
area: MS
title: Configure the retained clarification timeout shim with restart truth
persona: Dora
journey: J-administer-runtime-settings
expected: Dora sets tools.clarify.timeout to omitted, 0s, a valid finite value, or an invalid value. Every valid value is accepted with a deprecation warning and reads back as the unbounded effective policy; an invalid value fails with the exact range error while the last valid value stays.
entry_points: compozy config get/set tools.clarify.timeout -o json; config.toml [tools.clarify]
qa_status: untested
bug_ids: BUG-20260917-clarify-timeout-config-set
fix_status: fixed
retest_status: untested
fix_commits: working-tree
evidence: docs/qa/reports/2026-09-17-clarify-keepalive.md
last_report: docs/qa/reports/2026-09-17-clarify-keepalive.md
overlaps: RT-session-clarification-roundtrip
---

Set the retained key through each shape and prove what the daemon applies. Omitted and explicit `0s`
both mean an unbounded wait. A valid `1s`–`24h` value (for example `5m`) is accepted but ignored, so
the wait is still unbounded. Every read agrees on the effective value: `config get -o json` reports the
unbounded default, not the stored value. The daemon logs once per process that `tools.clarify.timeout`
is deprecated, that clarification waits are unbounded until answered, and that the key will be removed
in the next release. `-5s`, `500ms`, `25h`, `soon`, and `99h` fail with the exact
`tools.clarify.timeout must be between 1s and 24h, or 0s for no expiration: <value>` error while the
last valid value stays. The pending clarification projection shows `deadline: null` for every wait.

The key keeps its `restart-required` lifecycle entry through the shim release, but a write changes no
wait: there is no expiry to re-apply.

Minted 2026-09-17 by clarify-keepalive task 04 for the config-lifecycle half of the unbounded-default
change. Superseded 2026-11-17: the finite behavior it originally covered was removed. The key is a
deprecated shim until its removal release. Runtime waits, keepalive pings, races, extension inheritance,
and isolation stay with `RT-session-clarification-roundtrip`.
