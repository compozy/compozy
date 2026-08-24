---
id: RT-home-workspace-not-registrable
area: RT
title: Refuse the home directory as a workspace everywhere
persona: Dora
journey: J-scope-global-across-workspaces
expected: Registering the operator home directory is refused deterministically on CLI, HTTP, and UDS with a typed reason and creates no workspace row; the daemon no longer auto-registers it at boot; on an install that previously carried the home row that row is gone and the work it held reads back as no-workspace work rather than disappearing.
entry_points: compozy workspace add ~/; compozy workspace list; POST /api/workspaces over HTTP and UDS; GET /api/workspaces; daemon boot on a pre-existing home-workspace install
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: MS-web-workspace-lists-hide-home; MS-global-scope-no-workspace-work; MS-web-workspace-add-directory-browser
---

Minted by Profiles task 12 (planning) for the task_01 QA-impact flag: phase 0 deletes the boot-time
registration of the operator home directory and adds a non-registrable guard, while the general
operator-home resolver stays for its legitimate consumers. `MS-web-workspace-lists-hide-home` owns
the web presentation of the same rule; this row owns the daemon and structured surfaces. Task 13
owns the walk, the evidence, and the verdict.

Walk:

1. On a fresh home, run `compozy workspace add ~/` and repeat the registration through HTTP and UDS.
   Every response must refuse with the same typed reason and the same action, and `workspace list`
   must show no new row.
2. Repeat with equivalent spellings of the same path — trailing slash, `$HOME`, a symlink to the
   home directory, and a relative path resolving to it — and confirm the refusal is on the resolved
   path, not on the literal string.
3. Register a real project folder immediately afterwards and confirm it succeeds, proving the guard
   is specific rather than a blanket registration failure.
4. Seed an install that already carries the legacy home workspace row with work attached to it, boot
   the daemon, and confirm no auto-registration runs, the row is gone, and every item it used to
   hold is still readable as no-workspace work with its counts and relationships intact.
5. Confirm nothing in the boot path recreates the row on a second restart.

Expected evidence: CLI, HTTP, and UDS refusal payloads side by side; the workspace listing before
and after each attempt; the successful project-folder registration; and pre-boot versus post-boot
counts for every family of work the legacy row used to hold.
