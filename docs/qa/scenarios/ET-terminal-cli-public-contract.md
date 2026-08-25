---
id: ET-terminal-cli-public-contract
area: ET
title: Manage terminals through the complete CLI contract
persona: Ada
journey: J-operate-terminal-by-cli
expected: All twelve terminal verbs expose structured success and error output, list/get/journal agree with HTTP, selectors obey profile rules, and attach preserves watch, control, passthrough, and detach behavior.
entry_points: compozy terminal; HTTP terminal routes; UDS terminal routes
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-profile-selection-precedence
---

Flagged by integrated-terminal task 06. Task 10 owns the real-user walk, evidence, and verdict.

Walk:

1. Open, run, detach, list, inspect, signal, record, quote, respond, and close through the CLI.
2. Compare structured list, get, and journal output with HTTP and UDS responses.
3. Exercise the documented flag, selector, terminal-state, and capability failures and compare exact codes.
4. Attach in watch and control modes; verify the watch banner, detach chord, single-key passthrough, and exited-terminal refusal.

