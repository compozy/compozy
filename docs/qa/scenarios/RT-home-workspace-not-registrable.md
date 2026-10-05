---
id: RT-home-workspace-not-registrable
area: RT
title: Refuse the home directory as a workspace everywhere
persona: Dora
journey: J-scope-global-across-workspaces
expected: Registering the operator home directory is refused deterministically on CLI, HTTP, and UDS with a typed reason and creates no workspace row; the daemon no longer auto-registers it at boot; on an install that previously carried the home row that row is gone and the work it held reads back as no-workspace work rather than disappearing.
entry_points: compozy workspace add ~/; compozy workspace list; POST /api/workspaces over HTTP and UDS; GET /api/workspaces; daemon boot on a pre-existing home-workspace install
qa_status: untested
bug_ids: BUG-20261004-workspace-add-relative-path; BUG-20260906-stopped-history-schema-upgrade
fix_status: fixed
retest_status: pass
fix_commits: 52ed8d1bc
evidence:
last_report: docs/qa/reports/2026-10-02-untested.md
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

2026-10-04 structured walk: absolute/trailing/symlink and shell-expanded home spellings refuse
workspace_home_forbidden on the public surfaces and preserve the full catalog. Relative CLI
registration instead fails with an untyped absolute-path error; the linked bug owns this missing
CLI resolution. A real absolute project registers successfully and is removed after independent
HTTP/UDS readback. Baseline and current runtime are restored. Evidence: home-guard-lea-ended.json
in the current report directory. Legacy-preservation evidence is being reconciled separately.


2026-10-04 repaired relative-path replay: three relative home spellings now reach the same typed
daemon refusal. A relative project registers and reads identically through HTTP/UDS; its absolute
spelling is refused as a duplicate, preserving the same row. Cleanup restores the full baseline.
The original-persona replay and owning race-enabled suite pass. Gate/commit remain pending.
The earlier beta.19-origin session and three retained events prove lossless Global migration,
but the explicit catalog read after the second restart is not yet established; do not infer
that read from a Global owner response or the Web's hidden-home presentation.

2026-10-04 delivery: the relative CLI defect is verified at 52ed8d1bc after the exact-tree gate
passes. Fix/retest fields describe that repair; keep the full scenario untested while the remaining
legacy boot/catalog leg is executed in the new home-migration targeted lab.

2026-10-04 migration continuation: HTTP/UDS retain the Global session and three entries, but
CLI status/events/history refuse its empty workspace owner. The retained-history finding is
reopened; the session ended before repair. The relative-root fix remains verified.

2026-10-04 final functional replay: beta.19 history remains intact after the repaired startup
and two ordinary restarts. HTTP/UDS catalogs stay empty. CLI status/events/history/follow and
independent transport reads agree; the owned project canary passes and is removed. Evidence:
home-migration-final-lea-ended.json. Delivery gate, new fix SHA and targeted audit/teardown
remain before this row's final closure.
