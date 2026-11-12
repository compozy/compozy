## 0.6.1 - 2026-11-12

### Bug Fixes

- Point the Atlassian extension at the v2 MCP endpoint (#1551)
- Keep issue triage running when a gh search flag is rejected
- Encode triage search queries and define retryable gh errors
- Make issue intake failures reach maestro
- Preserve requested reasoning during Goal rotation (#1555)
- Align Loop ACP provenance with applied options (#1554)
- Keep daemon booting with undefined disabled providers (#1556)
- Isolate native tool state and enrich retired task results (#1558)
- Honor recurring catch-up policies after host sleep (#1569)
- Keep loaded transcript messages stable when a live entry rolls pages (#1571)
- Retire stale terminal windows and open agent terminals on demand (#1570)
- Keep skill documents inline without duplicate result content (#1575)
- Recover heartbeat cadence after runtime configuration repairs (#1584)
- Preserve exact Claude models during session recovery (#1560)
- Watch explicitly selected fork pull requests (#1578)
- Resolve current heartbeat policy for manual wakes (#1579)
- Reconcile Goal ACP provenance and prove cleanup delivery (#1574)
- Retire pre-view Loop pin translation in v0.6.1 (#1590)
- Resume agent instances after daemon listeners start (#1591)
- Accept check-in proposals with surrounding prose (#1589)
- Keep UDS and HTTP harness lifecycle parity with standing context (#1594)
- Reconnect the window-manager stream after a client-side failure (#1583)
- Keep unusable SQLite controls from rejecting compliant heads (#1592)
- Allow terminal exec to target managed worktrees (#1585)
- Group native recovery reads and expose reply watches (#1576)
- Avoid duplicate and stale child early-stop wakes (#1586)
- Validate heartbeat health requests and synchronize daemon boot (#1582)
- Return admitted automation runs before agent completion (#1587)
- Retry terminal attach attempts that stall before ATTACHED (#1609)
- Preserve exact Claude models on fresh startup (#1610)
- Prevent child reply and attention wake starvation (#1604)
- Keep boot window progress, attempts and versions accurate across retries

### Features

- Add bundled interface fonts, text size and letter spacing to CompozyOS Appearance (#1550)
- Reveal model names and page the provider filter rail (#1536)
- Bind operator rules and peer roster into always-on agent context (#1572)
- Fold worktrees under projects in the terminal picker (#1599)
- Redesign the desktop boot window with real upgrade progress (#1600)

### Miscellaneous Tasks

- Route workspace heartbeat triage to claude-haiku-5-5

### Refactoring

- Remove compatibility shims expired in v0.6.1 (#1593)
- Split the desktop boot draft and address its review notes

### Testing

- Cover the composer context toggle and pin process fixture identities (#1577)
- Expect the boot window font-src CSP in E2E-034

### Release Notes

#### Fixes

##### Agent terminals no longer leave stale windows behind

Terminals that agents open now stay in the **Terminal** list until you open one, instead of opening a desktop window each time. Before, every agent terminal created a window that outlived the process, so desktops filled with windows that restored as "This terminal isn't here" and the Terminal dock item could show a terminal as open when none was running.

When an agent's terminal exits, or any terminal is cleaned up, its windows close on every desktop, and Undo or Reopen cannot bring them back. Terminal windows left over from before a daemon restart are removed at startup. Terminal output history and recordings are kept.

Migration notes: no action needed. Open an agent's terminal from the Terminal list or from the link in the transcript.

##### The daemon starts faster with always-on agents running

Restarting CompozyOS with active always-on agents no longer stalls startup. Before, each agent's main session resumed before the daemon was listening, so its CompozyOS tools waited out a connection deadline and failed before a retry, adding that delay per agent before the desktop app was ready. Always-on agents now resume once the daemon is accepting connections, and connect on the first attempt.

`compozy app status` also no longer rejects the desktop app's state while it is still starting.

Migration notes: no action needed.

##### Always-on check-ins and heartbeats recover on their own

Several problems that could leave an always-on agent's check-ins stuck are fixed:

- A check-in reply that wraps its proposal in a sentence or two of commentary is accepted. Before, any text around the JSON rejected the reply, and repeated rejections could pause the agent. A reply with more than one candidate is still rejected, and the failure report now names the reason.
- After you fix the triage model or runtime configuration that made a check-in fail, the agent schedules its next check within its normal cadence instead of waiting up to a day. Its status explains the earlier failure and that you can run a check now.
- A manual heartbeat wake uses the `HEARTBEAT.md` currently on disk, the same policy status shows. Before, a file edited in the repository could look active in status while a wake returned `heartbeat_no_policy`.
- A heartbeat status request that asks for session health without a session ID is rejected as invalid input instead of returning `session_not_found`.

Migration notes: no action needed.

##### The Atlassian extension uses Atlassian's v2 MCP endpoint

The Atlassian extension 1.0.1 connects to `https://mcp.atlassian.com/v2/mcp`, the endpoint Atlassian recommends. On some sites, Jira, Confluence and Rovo Search calls through the old endpoint failed with `401 Unauthorized; scope does not match` even after a successful sign-in.

Migration notes: update the Atlassian extension to 1.0.1 from the Marketplace and sign in again when prompted.

##### Supervising child sessions sends one accurate wake

A parent session that watches a child with a session monitor now gets one wake when the child stops early, from that monitor. Before, it could also get a second early-stop wake that assumed the child should have opened a pull request and could arrive after the child had already moved on. The early-stop wake now asks the parent to review the child's answer, and is dropped if the child has started a newer turn.

Compact session records from native tools now include pending reply watches, so a parent can see what it is waiting on without requesting the full record.

A Claude subagent's answer is no longer lost or marked failed when the provider reuses its call ID in a later turn, and a background subagent's late summary is kept after its turn finishes.

Migration notes: no action needed.

##### Child replies reach a busy parent session during its turn

A parent session busy with a long turn now receives replies and needs-attention wakes from its child sessions during that turn. Before, monitor wakes were delivered while child replies and attention wakes waited until the turn ended. Each reply keeps its own identity, and a needs-attention wake is dropped when the child has moved on or a session monitor already covers that state. When a pending permission or question changes, the parent gets a fresh wake, so the current state is never lost.

A child that answers a follow-up from the session that created it is no longer flagged as stopped before delivering just because it opened no pull request.

Migration notes: no action needed.

##### New Claude sessions start on the exact model you selected

A new Claude session started with a full model ID such as `claude-opus-5-5` could fail at startup when Claude's model list offered only older IDs or aliases, while the same model worked on another attempt. New sessions, including input-only sessions, now register the selected model the same way resumed sessions already did, so they start on that exact model. If Claude refuses or ignores the model, the session fails once with an error that names the provider and the model. It never switches to an alias or an older model.

Migration notes: no action needed.

##### Claude sessions recover with their exact model after you sign in again

A Claude session pinned to a full model ID such as `claude-opus-5-5` now resumes on that model after a sign-in recovery. Before, a reloaded session could see only a stale or offline model list that left out the pinned model, so recovery failed with a model error even after you signed back in and **Usage** showed the account as healthy.

CompozyOS now checks Claude's local sign-in state before it selects a model, keeps the session's exact model through the reload, and reports the real cause when recovery still fails: a missing sign-in, a model the provider refuses, or the last replacement error. A parent watching a child that is waiting for you to sign in is now told so, instead of being told the child is waiting for permission.

Migration notes: no action needed.

##### The daemon starts when a disabled provider has no definition

A profile with `[providers.<id>] enabled = false` for a provider that has no built-in or configured definition no longer stops the daemon from starting. Disabled providers are no longer validated until you enable them, and provider usage skips a provider it cannot resolve and logs a warning instead of failing for every provider. An enabled provider, or the default provider, must still be fully defined.

Migration notes: no action needed.

##### Pull request monitors can watch fork pull requests

A `pull_request` monitor can now watch a pull request opened from a fork. Pass the branch as `owner:branch`, for example `alice:fix-login`. Before, the monitor registered but never saw the pull request, because only branches in the base repository were matched. A plain branch name still selects a branch in the base repository, and a malformed `owner:branch` value is rejected when you create the monitor.

A canceled monitor also no longer reports its cancellation time as `fired_at`.

Migration notes: no action needed.

##### Loop and Goal runs keep the runtime you asked for

Loop and Goal status now shows the correct source for each applied Agent Client Protocol (ACP) option, the settings CompozyOS sends to the agent when it starts: options added by the agent definition show `agent`, and options the provider could not apply no longer list a source. Before, the report could leave an agent-added option without a source or keep one for an option that was skipped.

A Goal that rotates to a fresh session after its context fills now asks for the reasoning level you requested, including the provider default. Before, it could ask for the level the provider had reported, which failed the Goal's runtime check and stopped recovery.

Migration notes: no action needed.

##### Triggering a job returns its run right away

`compozy automation jobs trigger <id>` now returns as soon as the job's agent accepts the
prompt, with the run id and `running` status, and exits 0. The run keeps going in the daemon,
which still owns retries, the final result and session cleanup. Before, the command waited for
the whole agent turn, so a turn longer than 30 seconds made the CLI report a request timeout
while the run kept going, and you never got its run id.

The HTTP/UDS trigger route and the `compozy__automation_jobs_trigger` native tool return the
running run the same way.

## Migration notes

A successful trigger now confirms that the run started, not that it finished. Scripts that
treated it as completion must ask to wait or follow the returned run id:

- CLI: `compozy automation jobs trigger <id> --wait --timeout 5m -o json`
- HTTP/UDS: `POST /api/automation/jobs/{id}/trigger?wait=true&timeout=5m`
- Native tool: `{"job_id":"<id>","wait":true,"timeout":"5m"}`

The wait defaults to one minute and accepts any positive duration up to 24 hours. If the run
finishes in time, you get its final result; a run that fails within the wait returns its error
and exits nonzero, as before. If the wait runs out, you get the running run and the CLI still
exits 0; the run is not canceled, and you can read its outcome later by run id. Invalid input,
errors before the run starts, and daemon or connection errors still exit nonzero. A timeout
without wait is rejected (`--timeout` requires `--wait`).

Task and Loop targets return their delegated receipts immediately, as before, even with
`--wait`. Extension host trigger calls still wait for completion. The `wait` and `timeout`
inputs are additive: no route, command, tool id, response field or stored data is removed or
renamed, and a retried manual run keeps its run id.

##### Compatibility shims scheduled for v0.6.1 are removed

v0.6.0 was the last release that migrated or silently ignored the retired declarations below
(#1518). v0.6.1 removes those compatibility shims (#1593). Most old declarations now fail the
normal validation for their file instead of being migrated or ignored, and CompozyOS no longer
rewrites them. The table notes the exceptions. Current declarations behave as before.

## Migration notes

**Upgrade through v0.6.0 first** if you skipped it. Start v0.6.0 once with every user, profile,
workspace and workspace-profile configuration in place so it migrates old files, resolve any
deferred-migration warning, and remove anything below that is still left over before you
upgrade; v0.6.1 refuses to load configuration that still contains it. Existing archived comments, SQL migration history, memory files, transcripts and
retained extension registrations are not deleted.

| Removed in v0.6.1                                                                                                                                                                                                                                                                                                                                                              | Use instead                                                                                                                                                                               |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| The agent kind renamed to always-on in v0.5.1, in a persisted `AGENT.md` `kind`, decoded and rewritten on load                                                                                                                                                                                                                                                                 | `kind: always_on`. Files v0.6.0 could not rewrite (read-only, or shipped in an extension) must be edited at their source; extension authors publish a new version.                        |
| `config.toml` agent tables under that renamed kind, migrated on load                                                                                                                                                                                                                                                                                                           | `[agents.always_on]`                                                                                                                                                                      |
| Archiving retired memory and compaction settings from `config.toml` on start: `[memory]` and its sub-tables, `[roles.dream]`, `[roles.checkpoint_summary]`, `[roles.memory_extractor]`, `[roles.memory_controller]`, `[session.compaction]`, `[[automation.triggers]]` with `event = "memory.consolidated"`, and hook matcher keys `compaction_reason` / `compaction_strategy` | Delete them. They now fail configuration validation.                                                                                                                                      |
| Ignoring the hook matcher keys `compaction_reason` and `compaction_strategy` in `config.toml` hooks, extension manifest hooks, `AGENT.md` and `SKILL.md`                                                                                                                                                                                                                       | `compaction_trigger` in config, extension or skill hooks. `AGENT.md` hooks cannot target context events.                                                                                  |
| Dropping `memory.backend` from an extension manifest's `capabilities.provides`, and `memory/recall`, `memory/store`, `memory/forget`, `memory.read` and `memory.write` from its `permissions.requires`                                                                                                                                                                         | Remove them from the extension source and publish a new version. Installed packages are never edited.                                                                                     |
| Skipping extension automation resources triggered by `memory.consolidated`                                                                                                                                                                                                                                                                                                     | Remove the resource.                                                                                                                                                                      |
| Ignoring `memory_policy` in `SOUL.md`                                                                                                                                                                                                                                                                                                                                          | Delete the field. It now gets the normal unsupported-field diagnostic.                                                                                                                    |
| Dropping retired memory tool IDs and the `compozy__memory` / `compozy__memory_admin` toolsets from agent, profile and session tool policies                                                                                                                                                                                                                                    | Remove them. Retired toolsets fail catalog validation. Exact retired `compozy__memory_*` IDs are still accepted but match no tool, and an explicit allow-list stays restricted.           |
| The `permissions.provider_full_access` key, a no-op with a warning in v0.6.0                                                                                                                                                                                                                                                                                                   | `permissions.mode` (`approve-all` for **Full access**). The old key is now unknown.                                                                                                       |
| Agent Client Protocol (ACP) usage aliases `cacheReadTokens` / `cacheWriteTokens`                                                                                                                                                                                                                                                                                               | ACP adapters must send `cachedReadTokens` / `cachedWriteTokens`. The aliases are no longer counted toward cache usage; nothing fails.                                                     |
| Automatic transfer of bundled `dev-cycle` extension state to `spec-cycle`                                                                                                                                                                                                                                                                                                      | Re-enter its inputs and credentials on `spec-cycle`, then run the removal command `compozy doctor` suggests for each `dev-cycle` installation. The old state stays in place until you do. |
| Treating a launch that a stop canceled before v0.4.8 as canceled instead of failed when you take over the session, so it could resume                                                                                                                                                                                                                                          | Take over those sessions while still on v0.6.0. Normal session recovery is unchanged.                                                                                                     |

Marketplace source management (`compozy marketplace sources`) is still experimental. Only the
expired "through v0.6.0" date was removed from its help text. The full upgrade steps are in the
migration guide.

##### Loop tool pins use only their selected result view

v0.6.1 removes the one-release compatibility translation for Loop tool pins created before native
tool result views shipped in v0.6.0 (#1432). Tool actions compare their pinned input and selected
output contract directly with the live tool. The automatic full-result fallback, pre-view input
digest translation, and table translating superseded output-schema digests have been removed.

## Migration notes

Unfinished Loop runs started before v0.6.0 can no longer resume actions using pre-view tool pins.
The next affected action fails with `ActionContractStale` (`action_contract_stale`). Start a new
run from the existing definition with `compozy loop run --name <name>` to pin current tool contracts.
Resuming the old run does not refresh its pins. Review definitions that read full-only fields
and add literal `result_view: full` to those nodes before starting the replacement run.

Definitions, completed runs, and their history are preserved. Runs started with current per-view
pins continue to use their selected contract, including after a daemon restart. No database,
configuration, CLI, HTTP/UDS, native tool ID, or hook shape changes in this removal.

##### Scheduled jobs follow their catch-up policy after your computer sleeps

When your computer sleeps through several runs of a recurring job, the job now follows its `catch_up_policy` when your computer wakes up. Before, a daemon that kept running through the sleep replayed every missed run regardless of policy. `coalesce` and `run_once_on_catchup` run once for the latest missed time, `replay` runs each missed time in order, and `skip_missed` checks the earliest missed run against the grace period: outside it, the backlog is recorded as one skipped run; within it, the latest missed time runs once. The job keeps its original schedule afterwards.

Migration notes: no action needed.

##### Reading a skill reference returns it once

`compozy__skill_view` now returns a skill document once, in the result's structured content, with a short summary as display text. Before, it returned the full document twice, so a large reference could exceed the result size budget and come back as an artifact to page through instead of inline text. Clients that read only text still receive the complete document.

`compozy__tool_artifact_read` now returns 32 KiB pages by default so each page fits comfortably in one response. You can still request up to 64 KiB per page.

Migration notes: no action needed.

##### Long transcripts stay steady when a turn starts

In a long session, the transcript no longer flickers or stutters when the first tool call of a turn appears. Before, once a session held more than a page of history, each new entry made the app treat the whole loaded transcript as changed and redraw every message. Messages that did not change now keep their place and are not redrawn.

Migration notes: no action needed.

##### Terminal windows no longer hang on "Connecting…"

A **Terminal** window in the Web UI could stay on "Connecting…" forever, with no prompt and no error, most often with the daemon in WSL2 and the browser on Windows. Each connection attempt now has a 10-second deadline. A stalled attempt is retried automatically and the window shows "Reconnecting…". After three stalled attempts in a row, the window says it couldn't connect to the terminal and offers **Reconnect**.

Migration notes: no action needed.

##### Terminal commands can run in a managed worktree

`compozy__terminal_exec` now accepts a managed worktree by ID or name, the same way opening a terminal does. Use the native tool's `worktree` input, the HTTP/UDS request field, or `compozy terminal exec --worktree <id-or-name>`. Before, a session not bound to a worktree could open a terminal there but could not run a command in it: exec rejected the checkout as `invalid_cwd`.

Relative working directories resolve inside the selected worktree, and the worktree cannot be removed while the command is still running.

Migration notes: no action needed.

##### Window layout saving recovers without a reload

If the app hit an error while syncing the window layout, it now reconnects on its own. Before, the menubar kept showing "Can't save window layout — retrying" with no detail, every dock item stayed disabled, and window commands did nothing until you reloaded the app. The message now shows the underlying error, the app reconnects with increasing delays, and the dock works again as soon as the layout syncs.

Migration notes: no action needed.
