---
id: MS-layout-profile-cli-roundtrip
area: MS
title: Manage saved layouts from the CLI
persona: Bruno
journey: J-administer-window-manager
expected: `agh layout-profile list|get|put|delete` round-trips a saved layout with `-o json`; a list contains global records plus the workspace's own and never another workspace's; `--expected-version 0` creates and a non-zero value replaces or deletes that exact version, failing on a concurrent write; `--scope` accepts only workspace or global; delete refuses to run without `--expected-version`.
entry_points: agh layout-profile list|get|put|delete; Settings › Layouts › Saved layouts
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: MS-configure-window-manager; ET-window-manager-public-parity
---

story: As an agent or an operator at a terminal, I can manage saved layouts without opening the web UI.

qa-impact: 2026-07-24 new surface. The routes already existed on HTTP and UDS; only the CLI verbs are new, so the scope filter and CAS semantics are the daemon's existing ones. Flag only; the next QA cycle owns live testing.
