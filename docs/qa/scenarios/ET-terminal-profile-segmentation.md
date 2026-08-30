---
id: ET-terminal-profile-segmentation
area: ET
title: Keep terminal work isolated by profile
persona: Ada
journey: J-switch-profile-terminal-scope
expected: A terminal belongs to the profile that opened it; profile switches re-scope the list, dock badge, stream, and journal; aggregate reads label every owner; archiving closes owned terminals but preserves history; workspace deletion removes both.
entry_points: Web profile switcher; Terminal app; --profile; --all-profiles; HTTP and UDS profile selectors
qa_status: pass
bug_ids: BUG-20260826-terminal-attach-profile-scope; BUG-20260829-terminal-journal-unlock-remount; BUG-20260829-workspace-delete-visible-terminal-deadlock
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-profile-retest-20260829-172042-776889-lab/qa-artifacts/qa/profile-segmentation-walk.md; /Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-profile-retest-20260829-172042-776889-lab/qa-artifacts/qa/qa-audit-report.md
last_report: docs/qa/reports/2026-08-28-integrated-terminal-rebase.md
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

2026-08-29 re-walk: passed. The isolated Web, CLI, HTTP, and UDS walk proved profile-local and
aggregate lists/journals, exact owner labels, selector refusals, archive cleanup with retained history,
and workspace deletion. Two production faults found by the walk were fixed and independently retested;
the strict evidence audit passed. The lab ended with `teardown.json` reporting `clean: true`.

2026-08-30 CI repair re-walk: passed. Current-tree E2E-020 switched from default to profile B and
All profiles, then rendered both authenticated command rows with their exact owner labels. The focused
eight-scenario browser run passed without a missing journal row or stale scope.
