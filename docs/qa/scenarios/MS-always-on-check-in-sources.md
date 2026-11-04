---
id: MS-always-on-check-in-sources
area: MS
title: An always-on agent wakes only when its check-in sources have something new
persona: Dora
journey: J-31
expected: Dora declares an approved script source and the bundled New GitHub issues kind in HEARTBEAT.md. Try it now shows each source's outcome without moving any cursor. Quiet hours start no model turn, even with standing guidance. A new script or an edited one waits for operator approval and never runs before it. A source that keeps failing reaches Dora as one report. A real check during a running poll says a check is already running and starts nothing. The Check-ins page lists, adds, edits, removes and approves sources, and report proposals say which source found the item.
entry_points: compozy agent heartbeat sources|validate|write; compozy agent instance check [--dry-run]|show|check-ins|reports -o json; web agent Check-ins page; main session reports
qa_status: pending
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-heartbeat-extension-sources, MS-web-stays-on-reports, MS-web-stays-on-placement
---

Use an isolated lab with one project workspace whose `origin` is a disposable GitHub repository,
a GitHub sign-in for the daemon, and an always-on agent `issue-watch` with an instance in that
workspace and checks on. Spec: `.compozy/tasks/heartbeat-sources/` (US-001–US-014, E2E-001–E2E-003).
The public walkthrough is `packages/site/content/docs/guides/watch-new-github-issues.mdx`; follow
its commands verbatim, so a docs drift fails here.

Ownership: this scenario owns the operator journey on the agent (declare, approve, try, gate,
status, Web). `ET-heartbeat-extension-sources` owns extension authoring, catalog scope across
workspaces and profiles, and the `forge-github/issues` provider matrix (watermark, labels,
pagination, linked PRs, credential errors); do not repeat those steps here. Report provenance in
the transcript is checked in `MS-web-stays-on-reports`.

## 1. Script source with approval (US-001, US-014)

1. Add an executable `sources/failed-deploys` to the agent folder that returns one item, and declare
   `{id: deploys, script: failed-deploys}` under `context.sources`. Save with
   `compozy agent heartbeat write`. Verify `agent heartbeat inspect` round-trips `context.sources`.
2. `compozy agent heartbeat sources issue-watch -o json` lists the script with
   `executable: true`, `approved: false` and its `sha256`. Run a dry check: the source reads
   `unapproved` (`source_script_unapproved`) and the script did not execute (it writes a marker
   file; the marker is absent). A real check sends exactly one notification for it.
3. `compozy agent heartbeat sources approve issue-watch failed-deploys` shows the script header, hash, line count and last-modified time, asks
   `Approve this exact file to run on check-ins? [y/N]`, and prints
   `Approved failed-deploys (sha256:…)`. The next check executes the script.
4. Edit one byte of the script. The next check shows `unapproved` again and the new bytes do not
   run. `approve --sha256 <old hash> --yes` fails with `source_script_changed`.
5. From inside an agent session, `approve` and the HTTP route fail with
   `source_approval_operator_only` and nothing changes. `revoke --yes` from the operator returns the
   source to `unapproved`.
6. Set `[agents.heartbeat.sources].scripts_enabled = false` and restart: the source reads
   `disabled` and never counts as a failure. Restore it.
7. Validation: `script: ../bin/x` and an entry with both `kind` and `script` are rejected with
   `heartbeat_source_ref_invalid`; a duplicate id with `heartbeat_source_duplicate_id`. A missing or
   non-executable file is not a validation error; it shows `source_script_missing` /
   `source_script_not_executable` after the next check, while other sources still run.

## 2. Extension kind source (US-002, US-009, US-013)

1. `agent heartbeat sources` lists `forge-github/issues` titled "New GitHub issues". Declare it
   with `with.repo` and `include: []`, plus the standing guidance from the tutorial.
2. The first check baselines and reports nothing. Open an issue with no PR and no session: the
   next check publishes one report proposing "#N has no owner" with the issue link. Open a second
   issue with an open PR "Fixes #M": the check stays quiet for it.
3. Pick the #N proposal: the main session receives one ordinary prompt and follows its AGENT.md.
4. Disable `forge-github` in the workspace: the source reads `unavailable`
   (`source_kind_unavailable`), not failing. Re-enable it.

## 3. Try it now without consuming (US-012)

1. With one new issue pending, run `compozy agent instance check issue-watch --dry-run` twice.
   Both print `Would run: yes`, `Sources:` with the per-source line
   (`deploys  script failed-deploys  ok  1 item  (<ms> ms)` for a script), and `Digest:` with the same item key.
   `agent instance show -o json` keeps `last_polled_at`, `last_item_count` and `next_poll_at`
   unchanged, and a following real check still receives the item.
2. Pause the instance and dry-run again: it still polls and reports outcomes.
3. Before the first real poll, instance show lists every declared source as `pending_first_poll`
   with its kind or script. After polling it shows `ok` with the same reference. Edit `with`:
   status returns to `pending_first_poll` without changing stored cursors. Remove a declaration:
   it disappears immediately from show, before the next real poll. An unreadable or invalid
   policy falls back to stored source rows without failing the instance response.
4. Repeat instance list/show and observe a live instance stream while editing HEARTBEAT.md.
   The next read/frame reflects added, edited and removed declarations, including an edit of
   the same byte length or a replacement preserving the size and modification time. Reload an
   extension-shipped policy with changed source declarations: the next read reflects the new
   package body. Session catalog facets retain attention counts and main-session
   exclusions without loading source policies. Daemon/API regression suites own the assertion
   that list rows share one parse per definition and facets never hydrate full payloads.

## 4. Cheap gate (US-003, US-013 AC-4)

1. With no new issue, let several scheduled checks pass. `agent instance check-ins -o json` shows
   `skipped` with `digest_empty` (or `digest_unchanged` for a repeated batch); no triage session is
   created and no provider turn is recorded, although HEARTBEAT.md has standing guidance.
2. Confirm cadence: the instance's next check advances by the configured interval after each skip;
   healthy sources leave `next_poll_at` empty. Source-backed checks never stretch into quiet backoff.
   Immediately run a real manual check after a timer check and confirm it polls again.
3. Return `more: true`: after either a completed triage or an unchanged-digest skip, the next timer
   wake uses `agents.heartbeat.min_interval` plus jitter, rather than the agent cadence.
4. Cancel a scheduled source check with `agent.heartbeat.wake.before`: check-ins records
   `hook_cancelled`, the script does not run, and the next wake uses the resolved cadence.
   A hook error records `triage_failed` and schedules failure backoff (when present), never
   the short source-collection recovery deadline.
5. Use a GitHub issue with a 256-character title, a Unicode body and many labels/linked PRs.
   Confirm the bounded item reaches triage without blocking later issues; overflow attributes
   are dropped deterministically and counted in `omitted`.

## 5. Failing source after the threshold (US-006)

1. Make the script exit 1. Each check shows `failing` with `source_exit_nonzero`, the stderr tail
   in `error.detail`, and an increasing `failure_streak`. Other sources keep reporting.
2. On the third consecutive failure (`failure_threshold = 3`), one report cites
   `source:deploys:failing:source_exit_nonzero`; later identical failures do not repeat it.
3. Fix the script: the next check returns to `ok` and resumes from the last committed state,
   delivering the items the failures held back.

## 6. `source_poll_in_flight` (US-007)

1. Give the script a 20 s `sleep`. Start a real check, then immediately run a second
   `compozy agent instance check issue-watch`. The second exits 0 and prints
   `Not started: a source check is already running for issue-watch (source_poll_in_flight)` and starts
   no occurrence; on Web, "Run it for real" says "A check is already running." The first check
   finishes normally.

## 7. Web Check-ins (US-001, US-002, US-011; `_uiux.md` S1–S5)

1. Open the agent's Check-ins page. "What it looks at" shows one row per source with its title
   (kind title or script name), parameter summary and status: "Not checked yet" before the first
   check, "Checked N min ago · K new" when ok, "Couldn't check: …" in the danger tone when failing,
   "Scripts are turned off", "Extension forge-github is off", and "Waiting for your approval ·
   <short hash>" for an unapproved script.
2. The HEARTBEAT editor keeps a file with `context.sources` in the form view and round-trips source
   order and `with` values the form does not show.
3. Try it now lists each source's outcome ("Nothing new", "1 new", or the error) above the items.
4. Add source: the dialog lists catalog kinds and scripts, renders fields from the kind's
   `params_schema`, and saves through the editor's CAS write. Add is hidden at
   `limits.max_per_agent`. The row's More menu offers Edit and Remove. Edit locks the Reads
   selection and changes the source name and settings; Remove deletes it; both save through the
   same write and survive reload.
5. In an editable operator view, the Approve button on an unapproved row approves the current
   script hash from the catalog; the row then reads ok after the next check.
   A stale hash returns `source_script_changed`; verify the error is shown and the catalog refreshes
   before reviewing and approving again.

Focused automation: `_tests.md` UT/IT ids per slice (S1 Go suites, S1w/S4 Vitest files,
`TestDaemonE2EHeartbeatSourceCatalog` in CI). Steps 7.4–7.5 match the S4 row controls and add/edit
dialog and must be walked on the integrated branch.

## Corrective boundary checks

- Put a FIFO in `sources/` without a writer. Discovery, approval and polling must return promptly and reject it as a script.
- Poll multiple sources across simultaneous instances, including dry runs: active source calls never exceed `max_concurrent_polls` in total.
- Start a main-session prompt or edit the policy while a timer poll is blocked. Release it and verify the batch is discarded, a skip is recorded, and the next wake uses the resolved cadence rather than the temporary polling deadline.
- A source approval notice must neither consume nor extend the ordinary session-notification rate window.
