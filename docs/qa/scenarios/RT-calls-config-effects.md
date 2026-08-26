---
id: RT-calls-config-effects
area: RT
title: Apply calls configuration through agent-manageable surfaces
persona: Bruno
journey: J-contain-and-audit-delegation
expected: Config get/set exposes every calls key and new calls honor depth, batch, child, idle, result budget, message, and overflow limits without changing in-flight snapshots.
entry_points: compozy config get calls.max_depth and compozy config set calls.max_depth 2, repeated for calls.max_batch, calls.max_children, calls.max_active_per_root, calls.idle_ttl, calls.results.default_budget, calls.results.max_budget, calls.results.overflow, calls.messages.rate_limit_per_minute, calls.messages.dedup_window, calls.messages.pending_cap, and calls.messages.max_bytes; the [calls], [calls.results], and [calls.messages] sections of config.toml; compozy__config_get and compozy__config_set with {"key":"calls.max_depth"} and {"key":"calls.max_depth","value":2}
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-delegation-depth-and-caps; RT-message-limits-typed-rejections; TA-task-result-contract
---

Change one calls limit at a time, verify the public config readback, and exercise its observable runtime boundary.

Every key must be reachable from all three agent-manageable surfaces — CLI, native tool and
`config.toml` — and resolve through the four shipped overlays (user, profile, workspace,
workspace-profile), so check that a workspace-level value actually shadows a user-level one. The
lifecycle claims to verify: `max_depth` applies to new calls only, `idle_ttl` applies at call time and
suspends while a call is in flight, there is no default deadline to configure, and
`overflow = "reject"` turns over-budget results into `call_result_over_budget` failures. Never write
config concurrently against one isolated home — run the sets sequentially.

For snapshot immutability, admit a call with `compozy call reviewer "Hold this call open"`, keep it
running, and record its admission limits. Change the applicable `calls.*` values sequentially through
the public config surface, then read the running call with `compozy call show <call-id> -o json` and
assert its original depth, child, TTL, result-budget, overflow, and message limits remain in force.
Admit a second call only after the writes complete and assert that this new record observes the new
values. Never infer snapshot behavior from config readback alone.
