---
id: MS-inspect-background-role-routing
area: MS
title: Inspect effective background role routing
persona: Ada
journey: J-route-background-work
expected: CLI, HTTP, and UDS expose the same six-role projection with truthful per-field provenance, nullable inherited values, actionable diagnostics, and no builtin identities in agent catalogs.
entry_points: agh roles list|show -o json; GET /api/roles and GET /api/roles/{role} over HTTP; GET /api/roles and GET /api/roles/{role} over UDS; docs runtime/api-reference/roles
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/agh-agent-roles-devtool-oss-launch-20260724-094737-758561-lab/qa-artifacts/qa/roles-cli.json; /Users/pedronauck/dev/qa-labs/agh-agent-roles-devtool-oss-launch-20260724-094737-758561-lab/qa-artifacts/qa/roles-http.json; /Users/pedronauck/dev/qa-labs/agh-agent-roles-devtool-oss-launch-20260724-094737-758561-lab/qa-artifacts/qa/roles-uds.json; /Users/pedronauck/dev/qa-labs/agh-agent-roles-devtool-oss-launch-20260724-094737-758561-lab/qa-artifacts/qa/role-unknown-http.json; /Users/pedronauck/dev/qa-labs/agh-agent-roles-devtool-oss-launch-20260724-094737-758561-lab/qa-artifacts/qa/role-dream-ghost-http.json
last_report: docs/qa/reports/2026-07-24-agent-roles.md
overlaps: MS-background-role-routing
---

QA impact 2026-07-23: the read-only roles contract and structured CLI verbs are new. Planning flag only; the next QA cycle owns the real-user parity walk.

Planning 2026-07-24 (Task 05): entry points widened to include the single-role read (`GET /api/roles/{role}` on both transports — `role_unknown` 404 is part of the contract) and the API reference docs page as the entry origin. Session charter: CH-roles-projection-truthfulness.

QA 2026-07-24: normalized list/show payloads matched across CLI, HTTP, and UDS; equal-value workspace provenance remained `workspace`; inherited fields stayed null; a ghost route returned a 200 projection with `role_agent_not_found`; and unknown roles returned the exact nonzero/404 `role_unknown` contract.
