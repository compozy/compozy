---
id: ET-terminal-profile-segmentation
area: ET
title: Keep terminal work isolated by profile
persona: Ada
journey: J-switch-profile-terminal-scope
expected: A terminal belongs to the profile that opened it; profile switches re-scope the list, dock badge, stream, and journal; aggregate reads label every owner; archiving closes owned terminals but preserves history; workspace deletion removes both.
entry_points: Web profile switcher; Terminal app; --profile; --all-profiles; HTTP and UDS profile selectors
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-terminal-profile-selectors; ET-profile-aggregate-owner-labels; ET-profile-stream-isolation
---

Flagged by integrated-terminal task 06. Task 10 owns the real-user walk, evidence, and verdict.

Reconciled by task 09: the owner journey moved from the general `J-scope-work-by-profile` to the
terminal-specific `J-switch-profile-terminal-scope`, whose flow is this scenario's walk. The selector
grammar and its refusals across the command line, both transports, the stream, and the agent tools
were split out to `ET-terminal-profile-selectors` rather than grown here; this file keeps the
operator-visible segmentation walk.

Walk:

1. Open a terminal under profile A, switch to B, and verify list, badge, catalog stream, and journal re-scope.
2. Switch back to A and confirm the terminal is still running.
3. Call CLI, HTTP, and UDS reads with the default scope, `--profile`/`?profile=`, and `--all-profiles`/`?all_profiles=true`; verify exact owner labels and refuse conflicting, unknown, archived, and cross-profile selectors without leaking rows.
4. Use the all-profiles read view and verify every terminal and journal row labels its owner.
5. Archive A and verify its terminals close while history stays readable; delete the workspace and verify both disappear.
