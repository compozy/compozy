---
id: ET-window-layout-upgrade-failure-retry
area: ET
title: A failed layout conversion keeps the old layout and offers Retry
persona: Théo
journey: J-organize-tabbed-work
expected: When the stored layout cannot be converted, the workspace opens on one desktop with a New tab window, the original value stays stored unchanged as the `legacy:` backup, the Web shell shows "We could not move your layout to tabs" / "Your previous layout is saved and nothing was lost. This desktop started empty so you can keep working." with Retry and Dismiss, and `compozy layout migration status -o json` reports `state: "failed"`, `failure_revision`, and an `error` whose `code` is the converter's own topology diagnostic code when it has one, otherwise `migration.conversion_failed`; Dismiss acknowledges the notice and keeps the backup; Retry (Web or `compozy layout migration retry`) converts the backup when the layout has not changed since the failure, publishes the converted layout, shows "Your windows are now tabs", and clears `failure_revision`; a retry whose stored value still fails reopens the failure notice; after a layout change the Web toasts "Your layout changed after the upgrade failed, so it was kept as it is." and the CLI answers a revision conflict, while `compozy layout migration retry --replace` replaces the current layout; retry on a workspace that is not failed is an invalid command (422); a re-upgrade whose stored value differs from the backup is never refused (no 503): the earlier backup is set aside unchanged under `superseded:<profile>:<sha256>`, the new value becomes `legacy:<profile>`, the status reports "<n> earlier layout backup(s) were set aside and preserved; retry uses the current legacy backup.", and both byte sets stay retrievable.
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

scope: The conversion failure path, the preserved backup, Retry with and without replace, the conflict guard, and a re-upgrade after a failed conversion.

Walk: Preconditions: an isolated lab from `eng-qa-bootstrap` running this build (unique home, port and socket), one workspace registered and opened once so its layout exists.

Seed fixtures (daemon stopped). The seeder is `web/e2e/fixtures/retired-knowledge-window-seeder`; its raw mode takes only `--home`, `--workspace`, optional `--profile` (default profile unless the lab uses another), `--raw-snapshot <file>` and `--raw-key snapshot|legacy` (default `snapshot`), and refuses any layout-building flag. It writes the file's bytes unchanged under the window-manager key `snapshot:<profile>` or `legacy:<profile>` through the client-state engine with a revision guard, without converter validation, and prints the stored value. Never edit the client-state database (`<home>/state/clientstate.db`, bbolt) by hand.

- `broken-v5.json`: `{"version":5,"windows":{"broken":{}}}`, a version 5 value the converter rejects (the value the Go suite uses in `internal/daemon/window_manager_repository_test.go`).
- `good-v5.json`: a convertible version 5 value for the same workspace. Copy the whole lab home (`cp -R <home> <home>-copy`) with the daemon stopped, run `go run ./web/e2e/fixtures/retired-knowledge-window-seeder --home <home>-copy --workspace <id> --floating <window-id>` there, and save its stdout as `good-v5.json`. The copy keeps the workspace id, which the conversion checks.

Seed: `go run ./web/e2e/fixtures/retired-knowledge-window-seeder --home <home> --workspace <id> --raw-snapshot broken-v5.json`.

1. Start the daemon and open the workspace: one desktop with a New tab window and the failure notice with Retry and Dismiss. `compozy layout migration status --workspace <id> -o json` shows `state: "failed"`, `from_version: 5`, `failure_revision`, and `error.code`.
2. Click Retry without changing anything: the value still fails, so the notice returns (briefly "Retrying"), and status stays `failed`.
3. Stop the daemon and write the convertible backup: `go run ./web/e2e/fixtures/retired-knowledge-window-seeder --home <home> --workspace <id> --raw-snapshot good-v5.json --raw-key legacy`. The stored layout is now the default v6 one, so the next start does not convert again. Start and click Retry: the layout converts, "Your windows are now tabs" shows, and status is `migrated` with no `failure_revision`.
4. Stop, run "Seed" again, start (status `failed`; the convertible backup from step 3 is set aside as `superseded:<profile>:<sha256 of good-v5.json>` and `error.message` ends with "1 earlier layout backup(s) were set aside and preserved; retry uses the current legacy backup."), stop, rerun the step 3 command, start, and open a window. Click Retry: the toast reads "Your layout changed after the upgrade failed, so it was kept as it is." and the window stays. `compozy layout migration retry --workspace <id>` answers `window_manager_revision_conflict`; `compozy layout migration retry --workspace <id> --replace` replaces the current layout with the converted one.
5. Click Dismiss on a failure notice: it closes in every connected client, status shows `acknowledged: true`, and the backup is still there (a later Retry still runs).
6. On a migrated workspace, `compozy layout migration retry --workspace <id>` is rejected as an invalid command (422 over HTTP).
7. Re-upgrade after a failed conversion. Stop, run "Seed" again and start: status `failed` (the backup is now the broken value). Stop and write the layout a v0.6 daemon would have saved in the meantime: `go run ./web/e2e/fixtures/retired-knowledge-window-seeder --home <home> --workspace <id> --raw-snapshot good-v5.json`. Start and open the workspace: it loads (no `window_manager_unavailable`, no 503) with the converted layout. `compozy layout migration status --workspace <id> -o json` shows `state: "migrated"`, no `failure_revision`, and `error` with `code: "migration.backup_superseded"`, `path` naming a `superseded:<profile>:` key, and the message "2 earlier layout backup(s) were set aside and preserved; retry uses the current legacy backup." (the count covers every set-aside backup of the profile).
8. Both byte sets are retrievable. Stop the daemon and read the bbolt store with the module's own CLI (`go run go.etcd.io/bbolt/cmd/bbolt@v1.5.0`, the version in `go.mod`): `keys <home>/state/clientstate.db ws:<id> window_manager` lists `legacy:<profile>` and `superseded:<profile>:<shasum -a 256 broken-v5.json>` (plus the step 4 key for good-v5.json). `get --format bytes <home>/state/clientstate.db ws:<id> window_manager <key> | tail -c +26` strips the 25-byte entry header: the superseded key prints exactly `broken-v5.json` and `legacy:<profile>` prints exactly `good-v5.json` (`cmp` both).
