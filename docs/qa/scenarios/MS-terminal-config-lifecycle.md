---
id: MS-terminal-config-lifecycle
area: MS
title: Configure terminal runtime policy safely
persona: Dora
journey: J-administer-runtime-settings
expected: Every `[terminal]` value defaults, validates, layers, and applies to the next terminal operation as documented, while a profile cannot raise the daemon-wide terminal cap.
entry_points: global, workspace, and profile config.toml [terminal]; structured configuration surfaces after public activation
qa_status: pass
bug_ids: BUG-20260826-terminal-config-set-unsupported
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-20260826-074528-452132-lab/qa-artifacts/qa/test-e2e-runtime-after-fix.log; docs/qa/reports/2026-08-26-integrated-terminal.md
last_report: docs/qa/reports/2026-08-26-integrated-terminal.md
overlaps:
---

Flagged by integrated-terminal tasks 01 and 06. Task 10 owns the real-user walk, evidence, and
verdict.

Walk:

1. Read global, workspace, and profile `[terminal]` projections and record all ten defaults plus their provenance.
2. Set each key to a valid value at every allowed scope, apply it sequentially, and open a new terminal after each change to verify next-operation and hot-apply behavior.
3. Submit one invalid value for every validation path and attempt a profile override that raises `max_per_daemon`; verify the typed refusal and unchanged effective value.
4. Read structured config and settings projections after every apply; verify precedence, restart truth, and that profile overlays cannot raise the daemon-wide cap.
