---
id: MS-always-on-check-in-sources
area: MS
title: An always-on agent wakes only when its check-in sources have something new
persona: Dora
journey: J-31
expected: Dora declares an approved script source and the bundled New GitHub issues kind in HEARTBEAT.md. Try it now shows each source's outcome without moving any cursor. Quiet hours start no model turn, even with standing guidance. A new script or an edited one waits for operator approval and never runs before it. A source that keeps failing reaches Dora as one report. A real check during a running poll says a check is already running and starts nothing. The Check-ins page lists, adds, edits, removes and approves sources, and report proposals say which source found the item. Check-ins that reach the model call the agent's read tools, native and MCP, through the gateway: unannotated MCP tools are writes and denied unless an approved operator `read_only_tools` lists them; writes run only from the approved `allow_writes` and only when the main session would not ask; caps, interaction tools and a missing main session deny with stable codes; every attempt is audited per occurrence, and a check-in that wrote always reports. Declared actions tag proposals; approved `auto_accept` actions are picked by the daemon under the daily cap and queue cap, while every other proposal waits for an operator pick (1-based CLI, once, operator credentials only). An agent-edited authority block grants nothing until the operator approves it, and `check_in_tools = "disabled"` restores tool-less check-ins.
entry_points: compozy agent heartbeat sources|validate|write|authority show|approve|revoke; compozy agent instance check [--dry-run]|show|check-ins [--occurrence]|reports|pick -o json; POST /api/agent-instances/{id}/reports/{report_id}/pick; POST|DELETE /api/agents/:name/heartbeat/authority/approve; compozy__tool_info|tool_list (access); web agent Check-ins page; main session reports
qa_status: partial
bug_ids: BUG-20261108-aocit-destructive-index, BUG-20261108-aocit-instance-mcp, BUG-20261108-aocit-occurrence-proposals, BUG-20261108-aocit-policy-actions, BUG-20261108-aocit-cli-pick-agent, BUG-20261108-aocit-selector-workspace
fix_status: fixed
retest_status: pass
fix_commits: 4da259aef, c3e755717, cab3cda15, c15a927c1, ee9ff2851, 32628460c, 2a49860ec
evidence: .compozy/tasks/always-on-check-in-tools/qa-walk/ (gitignored, aocit/qa worktree; 2026-11-08 §8 walk)
last_report: docs/qa/reports/2026-11-08-always-on-check-in-tools-qa.md
overlaps: ET-heartbeat-extension-sources, MS-web-stays-on-reports, MS-web-stays-on-placement, ET-native-tool-approval-grants
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

### Fresh executable contention

The source runner writes and closes its private verified executable before waiting for in-flight
Linux Go vforks to finish. It briefly acquires/releases the fork read lock after the close, so
inherited CLOEXEC writers have closed at exec before the source starts. Filesystem I/O never runs
under that process-wide lock. The wait observes the poll timeout and cancellation, returns
`source_timeout` when canceled, and leaves unrelated child launches free to proceed.
The Linux `TestScriptRunner` regressions run in isolated test subprocesses and coordinate the
in-flight-fork lock to verify that the
copy is written before the wait, cancellation finishes before that lock is released, and the
completed copy really executes after release. No test sleeps are used.
Run `CGO_ENABLED=1 go test -race -count=50 -run '^TestScriptRunner$' ./internal/heartbeat/source`
on Linux. Its output-limit case must still return `source_output_too_large` under parallel starts.

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

1. Open the agent's Heartbeat page. "Sources" is its own section, between "When" and "What it
   looks at", with one row per source: title (kind title or script name), parameter summary and
   status. Statuses are "Not checked yet" before the first check, "Checked N min ago · K new"
   when ok, "Couldn't check: …" in the danger tone when failing, "Scripts are turned off",
   "Extension forge-github is off", "Needs your approval" for an unapproved script, and "Can't
   run: the file isn't executable" for an unchecked script without its exec bit (approved or
   not). With scripts off, script rows read "Scripts are turned off" and nothing asks for
   approval. Rows only report: none of them carries Approve.
2. The HEARTBEAT editor keeps a file with `context.sources` in the form view and round-trips source
   order and `with` values the form does not show.
3. Try it now lists each source's outcome ("Nothing new", "1 new", or the error) above the items.
4. Add source: the dialog lists catalog kinds and scripts, renders fields from the kind's
   `params_schema`, and saves through the editor's CAS write. Add is hidden at
   `limits.max_per_agent`. The row's More menu offers Edit and Remove. Edit locks the Reads
   selection and changes the source name and settings; Remove deletes it; both save through the
   same write and survive reload.
5. Approval notice (#1258). In an editable operator view, every declared executable script the
   catalog reports unapproved gets a notice above the Sources rows. The section value reads "1 needs
   your approval". The notice title is "Approve <script> to let it run", or "Approve the new
   version of <script>" after an edit. It carries one sentence, "<n> lines · Changed <ago>", and
   Approve. On the desktop app it also has Open script, which opens the file in the configured
   editor. Approve sends the hash the catalog showed; the notice leaves and the row reads
   "Approved · runs at the next check-in". A stale hash returns `source_script_changed`: verify
   the error is shown and the catalog refreshes before reviewing and approving again. A read-only
   (extension) agent shows no notice.
6. Approve before the first check (#1258). Add a script source to a placement that has never
   checked in, with Heartbeat on or off. Without waiting for a check, the notice is already there
   and the row reads "Needs your approval", not "Not checked yet". Approve it, then Try it now or
   wait for the first check: the source runs and reads ok.
7. Paused placement (#1258). Pause the placement (`compozy agent instance pause <id>`), then edit
   the script so its hash changes. The notice asks for the new version and says it runs as soon as
   you resume. Approve while still paused; after Resume the next check runs the script without
   asking again.
8. From the bell (#1258). With a script waiting in any project, open the menubar bell. A "Scripts
   to approve" group sits above Needs you, with one line per script (script name, agent) and
   Approve, plus "Review in Heartbeat". The bell count includes it. The group lists waiting scripts
   on paused and never-checked placements too, and shows a definition placed in several projects
   once. Approve sends the same hash-checked call: the line leaves, the bell count drops, and an
   open Heartbeat page shows the script approved. A file edit after the bell read returns
   `source_script_changed` with the same toast, and the line comes back with the new version on
   the next read. "Review in Heartbeat" (or clicking the line) closes the bell and opens that
   agent's Heartbeat page in the script's project. Approving never marks notifications read.

Focused automation: `_tests.md` UT/IT ids per slice (S1 Go suites, S1w/S4 Vitest files,
`TestDaemonE2EHeartbeatSourceCatalog` in CI). Steps 7.4–7.5 match the S4 row controls and add/edit
dialog and must be walked on the integrated branch. Steps 7.5–7.8 (#1258) are covered by
`TestHeartbeatSourceRuntime/Should_project_script_approval_before_the_first_check_and_while_paused`
(daemon instance read: `sources[].awaiting_approval` and `script_sha256`),
`TestHeartbeatSourceCatalogAndApproval` (catalog `scripts[].path`), the Heartbeat cases in
`agent-editors.test.tsx`, `scriptApprovalsFromInstances` in `agent-instance.test.ts`, and the
scripts group in `attention-bell.test.tsx`.

The CLI instance create/show/checks responses use the `instance` envelope. The source-catalog
E2E decodes that envelope before checking the persisted source's `ok` status.

## 8. Check-in tools and actions (#1357, US-001–US-013)

Spec: `.compozy/tasks/always-on-check-in-tools/`. Set `[tools.policy] external_default =
"enabled"` (or grant the source): with the default `disabled`, MCP sources are `source_disabled`
for every session and a check-in sees no MCP tool. Add a fixture MCP server `fixture` to the
agent's `AGENT.md` (operator-authored, `transport: http`) with four tools: `list_items`
(`readOnlyHint: true`), `get_item` (no annotations), `add_comment` (no annotations) and
`delete_item` (`readOnlyHint: true`, `destructiveHint: true`). Use a real provider for the
check-in (Claude `plan` or Codex `read-only`). Keep the instance's main session at
`approve-edits` unless a step says otherwise. Each step that "runs a check-in" means a real
`compozy agent instance check issue-watch` with one new source item, so the gate lets it
through; read the result with `agent instance check-ins issue-watch --occurrence <id> -o json`.

1. **Reads through MCP (US-001).** Guidance: "Call fixture list_items and propose one item per
   result." Run a check-in. Its occurrence lists `mcp__fixture__list_items` with
   `access: read`, `decision: allowed`, `outcome: ok`, a redacted `args_preview` and no result
   body. The report proposes from the fixture data, and `agent instance reports -o json` shows
   `tool_calls.total ≥ 1`. `compozy__tool_info` for the tool shows `access: read` and
   `check_in_writable: false`. With no new
   item, the next check-in is `skipped` (`digest_empty`/`digest_unchanged`) and its occurrence
   has no tool calls.
2. **Unannotated and destructive tools (US-002, ADR-002).** Guidance asks for `get_item` and
   `delete_item`. Both read `access: write` (`delete_item` despite `readOnlyHint`, with
   `destructive: true`). The check-in's tool list leaves them out
   (`tool list --session <check-in>` shows `check_in_write_not_allowed`), so the model cannot call
   them and no row is written. A direct dispatch is denied `check_in_write_not_allowed`,
   `effect: none` (IT-006). The main session lists both as `approval_required`.
3. **`read_only_tools` override and its approval (US-011, US-013).** Add
   `read_only_tools: [get_item]` to the operator-authored declaration. `heartbeat authority show`
   reads `unapproved` with a new digest, and the next check-in still denies `get_item` as a
   write; the main session still treats it as a write. Approve with
   `heartbeat authority approve issue-watch`: the next check-in reads it as `read`, allowed.
   In the Web, both the pending approval notice and a changed-authority save confirmation list
   every override as server plus raw tool name under "Treated as read-only", explain that these
   tools can run without being listed under "What it can change", and never say "Nothing, reads only".
   The same override in an extension-shipped declaration is ignored with
   `mcp_read_only_override_untrusted`. An agent-definition override stays a write in
   agentless and daemon-actor tool listings when no scope approval is present; an
   operator `config.toml` override still applies. Add an invalid raw name such as
   `invalid name` to the agent override list: resolution and annotated read calls
   still succeed, the invalid entry grants no read access, and the daemon warns
   `mcp_read_only_override_invalid`.
4. **Approved write (US-002).** Write `check_ins.allow_writes: [mcp__fixture__add_comment]`
   with `heartbeat write`; it prints the block and asks to approve; answer `y`. Set the main
   session to `approve-all` (or grant `add_comment` "always allow" for the agent). A check-in
   that calls `add_comment` records it `allowed`, `outcome: ok`, `effect: done`, and the fixture
   received exactly one call. Walk this on Codex (`read-only`): Claude check-ins run in `plan` and
   usually decline approved writes (open finding F-1 in the 2026-11-08 report).
5. **Needs approval (US-002).** Put the main session back to `approve-edits` (or let it degrade
   to a narrower level). The check-in is no longer offered `add_comment`; a direct dispatch is
   denied `check_in_write_needs_approval`. No approval request appears anywhere
   (`session interactions` on main and check-in sessions are empty).
6. **Writable set and interaction tools (US-002).** List `compozy__session_prompt`,
   `compozy__terminal_exec` and `compozy__notify` in `allow_writes` and approve. A check-in that
   tries all three denies the first two `check_in_tool_not_writable` and runs `compozy__notify`.
   A call to `compozy__clarify` is denied `check_in_tool_not_writable` and opens no question.
7. **Caps (US-003).** `max_tool_calls: 2` (approved): the third attempt is denied
   `check_in_tool_budget_exhausted` and the check-in still returns a valid reply. `max_tool_calls`
   above `[agents.always_on].check_in_max_tool_calls` fails `heartbeat validate` with
   `heartbeat_check_ins_invalid`. `max_write_calls: 0` denies every write with the budget code.
8. **No main session.** Run a check-in on an instance whose `instance show -o json` has no
   `main_session_id`: every call, including `list_items`, is denied `check_in_no_main_session`.
9. **Activity report (US-009).** With the approved write from step 4, make the check-in call
   `add_comment` and answer `NOTHING_TO_REPORT`. It publishes kind `activity` ("The check-in
   made 1 change"), the occurrence is not counted as quiet, and the notification reads "Your
   agent made 1 change" (or "Your agent made N changes" for multiple completed writes) ("Your agent may have changed something" when only `maybe`). Repeat while pausing the instance mid-turn: the outcome
   stays `cancelled`, the activity report still publishes, and the source item is seen again by
   the next check-in. A write left `running` at the end shows `outcome: unknown`,
   `effect: maybe` ("May have changed").
10. **Authority unapproved (US-013).** From inside an agent session, add an action with
    `auto_accept: true` through `heartbeat write`. `instance show -o json` reads
    `authority_status: unapproved`, one notification "Check-in changes need your approval"
    fires (not again on the next check-in), check-ins run with reads only and default caps,
    actions are not offered, and every proposal waits. `authority approve` from the agent
    session and HTTP `POST /api/agents/issue-watch/heartbeat/authority/approve` with agent
    credentials fail `heartbeat_authority_operator_only`. An operator save of an unrelated
    guidance edit without confirming leaves it unapproved; `approve --sha256 <stale> --yes`
    fails `heartbeat_authority_changed`; `approve` from the operator approves it.
11. **Actions and auto-accept (US-005–US-007, US-012).** Declare `report-harness-bug`
    (`auto_accept: true`) and `open-ticket` (`auto_accept: false`) and approve. Invalid ids,
    a duplicate id and nine actions fail `heartbeat_action_invalid`,
    `heartbeat_action_duplicate_id` and `heartbeat_action_limit_exceeded`. A check-in that
    proposes one `report-harness-bug` and one untagged item publishes one report: the first is
    picked with actor `auto_accept` (`auto_accept_status: picked`), and the main session
    receives the action instructions followed by the fenced proposal and acts on it; the second
    waits. When every proposal was auto-accepted, the notification reads "Your agent acted on a
    check-in". A proposal naming an undeclared action keeps its text, drops the action and the
    check result records `check_in_action_unknown`.
12. **Auto-accept bounds (US-006).** Set `auto_accept_per_day = 1`: the second auto proposal in
    24 h waits with `auto_accept_status: limit`. Fill the main session's input queue: the next
    waits with `queue_full` and the check-in still finishes. Re-read the report after the queue
    drains: it still shows `queue_full` (recorded at publication). With an unapproved block it
    shows `unapproved`; with no main session `no_main`; paused or archived instances
    record `inactive` rather than the daily `limit`. Queue an auto pick on a busy main
    session, then pause the instance: the queued auto pick reads `cancelled`.
13. **Operator picks (US-010).** `compozy agent instance pick issue-watch <report-id> 2` picks
    the waiting proposal (1-based) and prints that it was sent to the main session. Repeating it
    fails `proposal_already_picked` (409). From an agent session the CLI and HTTP pick fail
    `proposal_pick_operator_only` (403). On a paused instance it fails `instance_not_active`.
    An untagged proposal sends its title only, as does any proposal while authority is
    unapproved. The pick never shows as `auto_accept_status: picked`.
14. **Kill switch (US-011).** Set `[agents.always_on].check_in_tools = "disabled"` and restart:
    check-ins launch with no MCP servers and no tools, the occurrence reads
    `tools_status: disabled`, and no audit rows are written. Restore it.

Focused automation: `_tests.md` UT/IT ids in the S1/S2/F1 Go suites
(`internal/tools`, `internal/store/globaldb`, `internal/daemon` `heartbeat_wake_runtime_test.go`
with acpmock and a fixture MCP server) and the S3 Vitest suites. Steps 1–14 are walked with a
real provider and the fixture MCP server; the live Linear search is an operator follow-up.

QA walk 2026-11-08 (`aocit/qa`, `88adc0207` → `2a49860ec`, Claude and Codex, fixture MCP):
steps 1, 2, 4, 5, 9, 10, 11, 13 and 14 pass, and step 12 passes for `limit`. Seven product
defects were fixed and re-walked, each with a focused regression test. Not walked: steps 3, 6, 7
and 8, step 12's `queue_full`/`no_main`/cancelled parts, and the live Linear search (blocked: needs
the operator's Linear MCP sign-in). Open: F-1, Claude `plan` check-ins decline approved writes.
Report: `docs/qa/reports/2026-11-08-always-on-check-in-tools-qa.md`.

## Corrective boundary checks

- Approve check-in authority from the Web editor: a successful 204 response clears the pending state without an error.
- Set daemon caps to 50 tool calls and 10 writes; a policy with caps 30/7 remains valid in status and for packaged agents. Configured lower caps still reject it.

- Put a FIFO in `sources/` without a writer. Discovery, approval and polling must return promptly and reject it as a script.
- Poll multiple sources across simultaneous instances, including dry runs: active source calls never exceed `max_concurrent_polls` in total.
- Start a main-session prompt or edit the policy while a timer poll is blocked. Release it and verify the batch is discarded, a skip is recorded, and the next wake uses the resolved cadence rather than the temporary polling deadline.
- A source approval notice must neither consume nor extend the ordinary session-notification rate window.

Focused regression receipt 2026-11-04: the real SQLite/ACP
`TestHeartbeatSourceRuntime/Should_persist_source_hook_cancellation` exposed a test expectation
that spelled the persisted reason as `hook_canceled`. The runtime and pre-existing source
contract retain `hook_cancelled`; restore that exact expectation rather than rename stored data.
This fixture correction preserves the assertion and the public reason contract.

CI fixture alignment 2026-11-04: IT-011 verifies the documented post-poll policy-change skip
(`heartbeat_invalid`) as a second wake event while retaining all stale-state generation and
null-cursor assertions. The catalog E2E creates an explicit always-on agent and writes its source
script beside the profile-owned definition created by the CLI, rather than the old shared agent
folder. The real browser/runtime E2E remains CI-only under the shared-host restriction.

## Release follow-up: folded proposal details

Return more source proposals than the report limit, with a distinct failure detail only in
an overflow body. The delivered report retains that detail and the source item keys, sharing
the 500-rune folded body budget across proposals even when the first body fills the limit.
Focused regression: `TestSourceDigestGateAndFolding/Should_deliver_overflow_body_details`.

## Fenced triage replies

Return a valid proposals object in one Markdown code fence, using `json`, `JSON`, or no
language tag, with surrounding whitespace and LF or CRLF. The occurrence publishes its
proposal report. The same wrappers around `NOTHING_TO_REPORT` finish quietly. Prose before,
after, or inside the fence, multiple objects or fences, and malformed JSON still produce
`triage_output_invalid`. Its failure report includes a fully redacted output placeholder
and a generated fenced/unfenced format classification in `detail`. No reply characters
survive, including punctuation-only OAuth codes; the diagnostic stays fixed in size.

Focused automated coverage: `TestParseTriageReply` and `TestTriageReplyOutcome` in
`internal/heartbeat/heartbeat_test.go`. The existing `TestHeartbeatSourceRuntime` suite owns
source dispatch and report persistence. Live-provider re-walk is separate from these checks.

CLI evidence for this journey: `instance reports` text prints each proposal's pick
state and `auto_accept_status`, followed by total tools, completed/uncertain writes
and denials. `instance check-ins` prints WHEN, OUTCOME, TOOLS and PROPOSALS; the last
column reads, for example, `1 auto-accepted, 1 waiting`. Re-read a skipped report
both after draining its queue and after 24 hours: the original reason must remain.
An unchanged approved authority digest saved through CLI asks for no new approval.
A notification publication failure must not suppress the check or mark its notice
as sent. Focused F1 backend evidence is in the slice report; integrated public
journey evidence remains owned by the conductor.

## Runtime pick golden path regression (#1385)

The canonical `TestDaemonE2ECheckInProposalPickCLI` suite exercises the real daemon,
CLI and ACP fixture process: a source-backed check produces two proposals, the
approved action is automatically picked, the operator picks the other proposal,
a repeated pick returns `proposal_already_picked`, and authority remains approved.

Keep the fixture routed by typed turn-source metadata, advertise its configured
model, and give the two proposals distinct source item keys so deduplication
preserves both independent choices. The agent created through
`agent create` must receive the registered fixture command explicitly; registering
a separate mock agent does not configure that new agent. A driver launched without
`--fixture` produces a startup failure report instead of exercising proposal picks.
This focused runtime check does not replace the live-provider and Web journeys above.

## Rejected runtime configuration recovery

1. Configure a triage model the provider rejects and allow a timer check-in to finish.
2. Confirm the check-in records `triage_config_unavailable`, keeps the instance active,
   and defers its next check to `max_interval`. Instance status/list must explain the
   rejection and name `agent_instance_check` for manual recovery.
3. Apply corrected heartbeat/provider runtime configuration. Confirm the next check is
   no later than one resolved cadence away and both backoff streaks are zero.
4. Repeat recovery through a fresh live catalog confirmation of the triage model in the
   same profile and workspace. Other profiles/workspaces must remain unchanged.
5. Confirm paused, archived, checks-disabled, and in-flight instances are unchanged;
   pending events and unattended counts survive recovery. A repeated recovery must not
   move an already earlier deadline later.

Execution evidence: CI owns validation for this change; local runtime QA is prohibited
by the parallel-delivery operator constraint.
