---
id: ET-terminal-limits-capabilities
area: ET
title: Explain terminal limits and platform capabilities honestly
persona: Dora
journey: J-administer-terminal-capacity
expected: Workspace and viewer caps identify the blocking limit and recovery action. Terminal capability reads report the supported interactive and execution controls truthfully, and unsupported operations fail with a structured capability error.
entry_points: Terminal app; Settings terminal section; structured terminal surfaces
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-20260826-074528-452132-lab/qa-artifacts/qa/test-e2e-runtime-after-fix.log; docs/qa/reports/2026-08-26-integrated-terminal.md
last_report: docs/qa/reports/2026-08-26-integrated-terminal.md
overlaps: MS-terminal-config-lifecycle
---

Workspace and viewer caps identify the blocking limit and recovery action. Terminal capability reads report the supported interactive and execution controls truthfully, and unsupported operations fail with a structured capability error.

Walk each listed public entry point, then reload and read the stored result independently. Exercise rejection and recovery with the same workspace and profile to confirm that unrelated state remains intact.

2026-09-27 scope update: retired product surfaces were removed from this active scenario. Historical evidence remains in the dated reports; this revised contract requires a fresh walk.
