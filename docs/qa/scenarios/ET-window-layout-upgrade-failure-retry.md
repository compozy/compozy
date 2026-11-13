---
id: ET-window-layout-upgrade-failure-retry
area: ET
title: A failed layout conversion keeps the old layout and offers Retry
persona: Théo
journey: J-organize-tabbed-work
expected: When the stored layout cannot be converted, the workspace opens on one empty desktop, the original value stays stored as the `legacy:` backup, the Web shell shows "We could not move your layout to tabs" / "Your previous layout is saved and nothing was lost. This desktop started empty so you can keep working." with Retry, and `compozy layout migration status` reports `state: "failed"` with `error.code: "migration.conversion_failed"`; Retry (Web or `compozy layout migration retry`) converts the backup when the layout has not changed since the failure; after a change the Web toasts "Your layout changed after the upgrade failed, so it was kept as it is." and the CLI answers a revision conflict, while `retry --replace` replaces the current layout; retry on a workspace that is not failed is an invalid command.
entry_points: first daemon start after upgrade; failure notice Retry; compozy layout migration status; compozy layout migration retry [--replace]; POST /api/workspaces/{id}/window-manager/migration {action:"retry", replace}
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-window-layout-upgrade-v06; ET-window-manager-layout-recovery
---

story: An operator whose old layout cannot be converted keeps working on an empty desktop and gets the layout back once the conversion can run, without losing either.

scope: The conversion failure path, the preserved backup, Retry with and without replace, and the conflict guard.

Walk: Preconditions: an isolated lab from `eng-qa-bootstrap` running this build (unique home, port and socket). No fixture produces a failed conversion today: the seeder validates its output with the same converter. Write a version 5 snapshot that decodes but fails conversion (for example a window id referenced by two panes) into the lab database with the daemon stopped; if that cannot be done without code, mark the scenario `blocked` and report the missing fixture. Start this build: one empty desktop, the failure notice, status `failed` with `failure_revision`. Click Retry before changing anything: when the stored value still fails, the notice stays; after fixing the stored backup row, Retry converts it and shows the success notice. Repeat the failure, then open a window and click Retry: the toast reports the layout changed and the current layout stays. `compozy layout migration retry --workspace <id>` answers a revision conflict; `--replace` replaces the layout with the converted one. On a migrated workspace `retry` is rejected as an invalid command.
