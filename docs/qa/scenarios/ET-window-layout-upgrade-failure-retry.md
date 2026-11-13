---
id: ET-window-layout-upgrade-failure-retry
area: ET
title: A failed layout conversion keeps the old layout and offers Retry
persona: Théo
journey: J-organize-tabbed-work
expected: When the stored layout cannot be converted, the workspace opens on one desktop with a New tab window, the original value stays stored unchanged as the `legacy:` backup, the Web shell shows "We could not move your layout to tabs" / "Your previous layout is saved and nothing was lost. This desktop started empty so you can keep working." with Retry and Dismiss, and `compozy layout migration status -o json` reports `state: "failed"`, `failure_revision`, and an `error` whose `code` is the converter's own topology diagnostic code when it has one, otherwise `migration.conversion_failed`; Dismiss acknowledges the notice and keeps the backup; Retry (Web or `compozy layout migration retry`) converts the backup when the layout has not changed since the failure, publishes the converted layout, shows "Your windows are now tabs", and clears `failure_revision`; a retry whose stored value still fails reopens the failure notice; after a layout change the Web toasts "Your layout changed after the upgrade failed, so it was kept as it is." and the CLI answers a revision conflict, while `compozy layout migration retry --replace` replaces the current layout; retry on a workspace that is not failed is an invalid command (422).
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

Walk: Preconditions: an isolated lab from `eng-qa-bootstrap` running this build (unique home, port and socket), one workspace registered and opened once so its layout exists.

Seed (daemon stopped): the stored value under the window-manager domain key `snapshot:<profile-id>` (profile `default` unless the lab uses another) of that workspace must be a version 5 value the converter rejects. The Go suite uses `{"version":5,"windows":{"broken":{}}}` (`internal/daemon/window_manager_repository_test.go`). The existing `web/e2e/fixtures/retired-knowledge-window-seeder` cannot write it: it builds its value from the current layout and validates it with the converter before writing. It needs a raw mode, `--raw-snapshot <file>`, that writes the file's bytes verbatim under `snapshot:<profile>` through the same client-state engine and revision guard and skips validation. Until that mode exists, record the scenario as `blocked` on the missing fixture; never edit the client-state SQLite file by hand. With the mode: `go run ./web/e2e/fixtures/retired-knowledge-window-seeder --home <home> --workspace <id> --raw-snapshot broken-v5.json`, where `broken-v5.json` holds the value above. Keep a second file holding a convertible version 5 value: run the seeder normally on a copy of the lab (`--floating <a>`), copy its stdout.

1. Start the daemon and open the workspace: one desktop with a New tab window and the failure notice with Retry and Dismiss. `compozy layout migration status --workspace <id> -o json` shows `state: "failed"`, `from_version: 5`, `failure_revision`, and `error.code`.
2. Click Retry without changing anything: the value still fails, so the notice returns (briefly "Retrying"), and status stays `failed`.
3. Stop the daemon and write the convertible value under `legacy:<profile>` (the raw mode with a target key, `--raw-key legacy`; without it, skip to step 5 and record steps 3–4 as blocked). The stored layout is now the default v6 one, so the next start does not convert again. Start and click Retry: the layout converts, "Your windows are now tabs" shows, and status is `migrated` with no `failure_revision`.
4. Repeat "Seed", start (status `failed`), stop, write the convertible `legacy:` value again, start, and open a window. Click Retry: the toast reads "Your layout changed after the upgrade failed, so it was kept as it is." and the window stays. `compozy layout migration retry --workspace <id>` answers `window_manager_revision_conflict`; `compozy layout migration retry --workspace <id> --replace` replaces the current layout with the converted one.
5. Click Dismiss on a failure notice: it closes in every connected client, status shows `acknowledged: true`, and the backup is still there (a later Retry still runs).
6. On a migrated workspace, `compozy layout migration retry --workspace <id>` is rejected as an invalid command (422 over HTTP).
