---
id: MS-inspect-background-role-routing
area: MS
title: Inspect effective background role routing
persona: Ada
journey: J-route-background-work
expected: CLI, HTTP, and UDS expose the same two-role projection (coordinator, auto_title) with truthful per-field provenance, nullable inherited values, actionable diagnostics, and no builtin identities in agent catalogs.
entry_points: compozy roles list|show -o json; GET /api/roles and GET /api/roles/{role} over HTTP; GET /api/roles and GET /api/roles/{role} over UDS; docs runtime/api-reference/roles
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-agent-roles-devtool-oss-launch-20260724-094737-758561-lab/qa-artifacts/qa/roles-cli.json; /Users/pedronauck/dev/qa-labs/compozy-agent-roles-devtool-oss-launch-20260724-094737-758561-lab/qa-artifacts/qa/roles-http.json; /Users/pedronauck/dev/qa-labs/compozy-agent-roles-devtool-oss-launch-20260724-094737-758561-lab/qa-artifacts/qa/roles-uds.json; /Users/pedronauck/dev/qa-labs/compozy-agent-roles-devtool-oss-launch-20260724-094737-758561-lab/qa-artifacts/qa/role-unknown-http.json; /Users/pedronauck/dev/qa-labs/compozy-agent-roles-devtool-oss-launch-20260724-094737-758561-lab/qa-artifacts/qa/role-dream-ghost-http.json
last_report: docs/qa/reports/2026-07-24-agent-roles.md
overlaps: MS-background-role-routing
---

QA impact 2026-07-23: the read-only roles contract and structured CLI verbs are new. Planning flag only; the next QA cycle owns the real-user parity walk.

Planning 2026-07-24 (Task 05): entry points widened to include the single-role read (`GET /api/roles/{role}` on both transports — `role_unknown` 404 is part of the contract) and the API reference docs page as the entry origin. Session charter: CH-roles-projection-truthfulness.

QA 2026-07-24: normalized list/show payloads matched across CLI, HTTP, and UDS; equal-value workspace provenance remained `workspace`; inherited fields stayed null; a ghost route returned a 200 projection with `role_agent_not_found`; and unknown roles returned the exact nonzero/404 `role_unknown` contract.

Regression walk for issue #639 (retired 2026-10-07): it covered `checkpoint_summary` availability gated by `memory.enabled` and `session.compaction.enabled`. Both consumers and the role are gone, so the walk no longer applies.

Replacement walk (2026-10-07, memory removal):

1. Start an isolated daemon. Run `roles list -o json` and human-readable `roles list`. Expect exactly two roles, `coordinator` and `auto_title`, with the same fields and provenance on `GET /api/roles` over HTTP and UDS. `compozy status -o json` carries no `memory` object (the `runtime.memory` doctor probe reports process memory and stays).
2. Run `roles show dream`, `roles show checkpoint_summary`, `roles show memory_extractor`, and `roles show memory_controller`. Each exits non-zero with `role_unknown`; `GET /api/roles/{role}` over HTTP and UDS returns the 404 `role_unknown` body for each.
3. Keep `roles.auto_title.enabled=false` in one workspace and confirm the opt-out stays isolated from a sibling workspace and from the daemon-level setting.
4. Upgrade leg: on a home whose `config.toml` still carries `[roles.dream]`, `[roles.checkpoint_summary]`, `[roles.memory_extractor]`, or `[roles.memory_controller]`, the daemon starts, the tables are archived into the commented block at the end of the file, and `roles list` still shows the two live roles. Owned by `RT-upgrade-memory-removal-home`.

This walk proves configuration reporting and public transport parity; it does not claim a live provider produced a title.
