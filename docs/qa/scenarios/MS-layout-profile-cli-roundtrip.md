---
id: MS-layout-profile-cli-roundtrip
area: MS
title: Manage saved layouts from the CLI
persona: Bruno
journey: J-administer-window-manager
expected: `compozy layout-profile list|get|put|delete` round-trips a saved layout with `-o json`; a list contains global records plus the workspace's own and never another workspace's; `--expected-version 0` creates and a non-zero value replaces or deletes that exact version, failing on a concurrent write; `--scope` accepts only workspace or global; delete refuses to run without `--expected-version`.
entry_points: compozy layout-profile list|get|put|delete; Settings › Layouts › Saved layouts
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-08-01-window-tabs/agent-02-layouts-applies-now.png; docs/qa/reports/2026-08-01-window-tabs.md
last_report: docs/qa/reports/2026-08-01-window-tabs.md
overlaps: MS-configure-window-manager; ET-window-manager-public-parity
---

story: As an agent or an operator at a terminal, I can manage saved layouts without opening the web UI.

qa-impact: 2026-07-24 new surface. The routes already existed on HTTP and UDS; only the CLI verbs are new, so the scope filter and CAS semantics are the daemon's existing ones. Flag only; the next QA cycle owns live testing.

qa-impact: 2026-07-31 v2 layout profiles are rejected under the v3 tab topology contract. Reset to
verify the version boundary and current round trip.
