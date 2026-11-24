# QA Run Report — 2026-11-22 — Archive stopped threads (#1924, PR #1940)

- **Scope:** "Archive stopped" for one project or All projects — CLI `session archive --stopped`
  (`--dry-run`, `-o json`, `--workspace`, `--all-workspaces`, global `--profile`), native tool
  `compozy__session_archive_stopped`, Web project menu and View options, toast copy and Undo.
- **Cadence tier:** targeted
- **Build:** `db3f92bd5` (lab binary and web bundle built from the branch) · **Environment:** isolated lab
  (`eng-qa-bootstrap` targeted profile, surfaces cli/api/web/runtime), own `COMPOZY_HOME`, HTTP 38523,
  own socket, `[automation] enabled = false`, mock ACP agents (no live provider), no live daemon state.
- **Started:** 2026-11-22T18:56Z · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Théo | operator on CLI and agent sessions | Linux terminal, local daemon | RT #1924 steps 1-5 |
| Bruno | operator in the Web session list | Chromium 1280×720, local daemon | ET #1924 steps 1-5 |

## Flows in Scope

- `J-archive-session-without-deleting` — RT-session-archive-catalog, section "Archive every stopped thread (#1924)"
- `J-14` — ET-web-session-list-bulk-actions, section "Archive stopped (issue #1924)"

Seed (project `alpha`): two stopped threads (one with a child), a thread with a real subagent (delegate
fixture), a running thread, a live thread waiting on a permission, a pinned stopped thread, a stopped thread
in Later, an agent caller thread; project `beta` with one stopped thread; archived project `gamma` with one
stopped thread; a second profile `qa-b` with one stopped thread in `alpha`.

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | RT #1924 step 1 — dry run | RT-session-archive-catalog | Théo | Feature | Pass | | |
| 2 | RT #1924 step 2 — archive, `-o json`, restore_ids, subagents | RT-session-archive-catalog | Théo | Feature | Pass | | |
| 3 | RT #1924 step 3 — cwd target, All projects, archived project | RT-session-archive-catalog | Théo | Feature | Pass | | |
| 4 | RT #1924 step 4 — usage errors | RT-session-archive-catalog | Théo | Antisocial | Pass | | |
| 4b | RT #1924 step 4 — one thread's archive fails | RT-session-archive-catalog | Théo | Antisocial | Skipped | | |
| 5 | RT #1924 step 5 — native tool | RT-session-archive-catalog | Théo | Feature | Fixed | BUG-20261122-archive-stopped-denial-code-docs | 12e4be26d |
| 6 | Global `--profile` scope | RT-session-archive-catalog | Théo | Feature | Pass | | |
| 7 | ET #1924 step 1 — project menu | ET-web-session-list-bulk-actions | Bruno | Feature | Pass | | |
| 8 | ET #1924 step 2 — Undo | ET-web-session-list-bulk-actions | Bruno | Feature | Pass | | |
| 9 | ET #1924 step 3 — nothing eligible | ET-web-session-list-bulk-actions | Bruno | Feature | Pass | | |
| 10 | ET #1924 step 4 — View options per scope | ET-web-session-list-bulk-actions | Bruno | Feature | Pass | | |
| 11 | ET #1924 step 5 — failure toast | ET-web-session-list-bulk-actions | Bruno | Antisocial | Skipped | | |

## Session Debriefs

### RT #1924 — Théo (CLI and native tool)

- **Ran:** 2026-11-22T18:59Z → 19:03Z
- **Step 1:** `session archive --stopped --workspace alpha --dry-run` printed
  `Would archive 2 threads (3 sessions) in alpha.` / `Skipped 4 threads: 2 running, 1 later, 1 pinned.`;
  nothing was archived (list read).
- **Step 2:** the real run printed the same counts with `Archived`; root and child archived together; skipped
  threads untouched. `-o json` printed the bare response DTO (`dry_run`, `archived[]` with `session_ids` and
  `restore_ids`, `skip_counts`, `skipped[]`, `failures`, `failure_count`). Unarchiving `restore_ids` in order
  restored all four sessions. A thread with a real subagent archived both sessions while `restore_ids` listed
  only the root; `session unarchive <subagent>` was refused ("Subagent sessions are archived with their parent
  session.", exit 65), and unarchiving the root brought the subagent back.
- **Step 3:** from `projA` without `--workspace` the command targeted `alpha`; `--all-workspaces` printed
  `in all projects.` and archived `alpha` and `beta`; archived project `gamma`'s stopped thread stayed
  unarchived.
- **Step 4:** `--stopped` with a session id, `--stop`, `--with-children`, `--terminate-processes`,
  `--keep-processes`, and `--dry-run` without `--stopped` each exited 2 with a named reason. A per-thread
  failure is not reachable through public surfaces; the store and session suites own it (Skipped).
- **Profile:** `compozy --profile qa-b session archive --stopped --workspace alpha` archived only
  `qa-b`'s thread; the default profile's stopped threads in the same project stayed unarchived.
- **Step 5:** from an agent session, `dry_run: true` ran without approval and skipped the caller's thread as
  `caller`; without `dry_run` the call asked for approval, then archived (caller still skipped);
  `all_workspaces: true` was denied with `tool_denied` / `workspace_access_denied`, not the documented
  `capability_denied` / `needs_grant` → BUG-20261122-archive-stopped-denial-code-docs (docs fixed).
- **Surprises:** a live session with a pending permission is skipped as `running`, not `waiting on you`: the
  spec classifies `running` first. Stopping it cancels the permission (thread becomes eligible), and a daemon
  restart did not leave an `orphaned` interaction either, so `waiting_on_you` was not reachable from a public
  seed here; the store suite owns it. Both scenarios' wording was corrected to the spec.
- **Paper cuts:** the native tool's text summary read "would archive 1 threads (1 sessions)" — fixed in
  12e4be26d with an assertion in `TestNativeSessionArchiveStoppedTool`. `--workspace` with
  `--all-workspaces` is refused by cobra's flag-group check with exit 1 rather than the usage exit 2 (not in
  the scenario; left as is).

### ET #1924 — Bruno (Web)

- **Ran:** 2026-11-22T19:06Z → 19:12Z, Web bundle served by the lab daemon.
- **Step 1:** the `alpha` group header menu lists "Archive stopped" directly above "Archive project". Choosing
  it opened no dialog; the toast read "Archived 4 threads" / "4 skipped: 2 running, 1 in Later, 1 pinned" with
  "Undo"; the group went from 7 to 2 rows; running, waiting, pinned and Later rows stayed. Archiving while one
  of the archived sessions was open moved the window to `/sessions` and still showed the toast (43 ms).
- **Step 2:** "Undo" read "Restored 4 threads"; the group returned to 7, child and subagent included (CLI list
  confirmed 11 unarchived sessions).
- **Step 3:** choosing it again right after read "No stopped threads to archive" with the skip summary and no
  Undo.
- **Step 4:** under All projects, View options ends with "Archive stopped in all projects", which archived
  both projects' threads in one toast ("Archived 5 threads"). Under `alpha` it reads "Archive stopped"; on
  the Archived shelf and in the User scope it is absent. All profiles was not walked (session list suite owns it).
- **Step 5:** a failing archive or restore is not reachable through public surfaces; the hook suite owns the
  error toasts (Skipped).
- **Surprises:** none in the feature. Opening Sessions from the dock reopens the last session and restores its
  project scope, which made a first User-scope read invalid; re-read with no session open.

## What Was Fixed

### BUG-20261122-archive-stopped-denial-code-docs: wrong denial code in the Archive stopped docs
- **Symptom:** agents were told to expect `capability_denied` / `needs_grant` for `all_workspaces`.
- **Root cause:** docs named the internal decision instead of the public `tool_denied` / `workspace_access_denied` error.
- **Fix:** 12e4be26d (skill reference and RT scenario).
- **Regression test:** none needed — `TestNativeSessionArchiveStoppedTool` already pins the wire.
- **Retested:** denial payload re-read against the corrected text.

## Paper Cuts

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
|---|---|---|---|---|
| Théo | RT step 5, native tool text | "1 threads (1 sessions)" | Low | Fixed in 12e4be26d |
| Théo | CLI flag conflict | `--workspace` + `--all-workspaces` exits 1, not 2 | Low | Left as is (outside the scenario) |

## Evidence

Lab-side (scratch, `/home/jilles/dev/qa-labs/compozy-archive-stopped-1940-20261122-185609-503208-lab/qa-artifacts/evidence/`): `01-dry-run.txt`, `02-archive-alpha.json`, `02-unarchive.txt`,
`04-archive-alpha-subagent.json`, `04-unarchive-subagent-direct.txt`, `05-all-archive.txt`, `05-gamma.txt`,
`06-usage-errors.txt`, `07-profile-flag.txt`, `08-caller-events.json`, `09-caller-events.json`,
`09-tool-all-denied.json`, `web-08-cli-after-undo.txt`.

Web screenshots (published):
[project menu](https://raw.githubusercontent.com/jillesmc/compozy/pr-evidence/pr-1940/db3f92b/web-01-project-menu.png) ·
[before](https://raw.githubusercontent.com/jillesmc/compozy/pr-evidence/pr-1940/db3f92b/web-03-before.png) ·
[archived toast](https://raw.githubusercontent.com/jillesmc/compozy/pr-evidence/pr-1940/db3f92b/web-07-archived-before-undo.png) ·
[after Undo](https://raw.githubusercontent.com/jillesmc/compozy/pr-evidence/pr-1940/db3f92b/web-08-undo-restored.png) ·
[nothing eligible](https://raw.githubusercontent.com/jillesmc/compozy/pr-evidence/pr-1940/db3f92b/web-09-nothing-eligible.png) ·
[View options, All projects](https://raw.githubusercontent.com/jillesmc/compozy/pr-evidence/pr-1940/db3f92b/web-10-view-options-all-projects.png) ·
[All projects archived](https://raw.githubusercontent.com/jillesmc/compozy/pr-evidence/pr-1940/db3f92b/web-11-all-projects-archived.png) ·
[View options, one project](https://raw.githubusercontent.com/jillesmc/compozy/pr-evidence/pr-1940/db3f92b/web-12-view-options-one-project.png) ·
[View options, User scope](https://raw.githubusercontent.com/jillesmc/compozy/pr-evidence/pr-1940/db3f92b/web-13-view-options-user-scope.png) ·
[open session archived](https://raw.githubusercontent.com/jillesmc/compozy/pr-evidence/pr-1940/db3f92b/web-06-archived-open-session.png)

Teardown: `/home/jilles/dev/qa-labs/compozy-archive-stopped-1940-20261122-185609-503208-lab/qa-artifacts/qa/teardown.json` reports `"clean": true`.

## Final Status

QA outcome for the #1924 sections: **pass** with one Friction docs bug fixed (12e4be26d) and one paper cut
fixed; two failure-injection steps skipped as not reachable through public surfaces (owned by the store and
hook suites). Totals by impact: Blocks-Completion 0, Data-Loss 0, Trust-Damage 0, Friction 1 (fixed).
PR readiness depends on CI for the pushed head.
