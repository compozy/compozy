## 0.5.1 - 2026-11-04

### Bug Fixes

- Let the mouse wheel scroll the transcript over Mermaid diagrams (#1176)
- Apply the normal removal policy when a worktree's base ref was deleted (#1177)
- Stop reporting a lost wake-up when a session cancels its own monitor (#1190)
- Keep session_prompt output schema in sync with its receipt (#1188)
- Open a profile-scoped agent in its own profile from All profiles (#1189)
- Close another profile's session windows when switching into a profile (#1197)
- Show N linked PRs instead of one PR chip plus +N in the session header (#1207)
- Offer Add stays-on agent from the session list toolbar (#1187)
- Pick a stays-on agent's model inline on its home (#1201)
- Stop stays-on agents asking for every CompozyOS tool call (#1206)
- Allow comparison and logic builtins in CompozyOS Loop templates (#1213)
- Require approval for destructive session profile moves
- Compare Loop template numbers exactly
- Apply review follow-ups to profile-switch window sweep (#1221)
- Start the CompozyOS daemon when a profile is archived (#1214)
- Save agent avatar and color changes and never fail an agent read on appearance (#1208)
- Reject unsupported agent avatar backgrounds and clarify validation
- Resume a desk's interrupted profile-switch sweep
- Align generated clients and validation after main merges
- Cover profile-switch rollback during owner reads
- Preserve newer session attention when marking it seen
- Enforce session profile on workspace routes (#1218)
- Scope the command palette client directory to the caller's profile (#1217)
- Keep document-wide streams and queued clicks out of the HTTP/1.1 pool (#1212)
- Validate window manager bindings in the client profile
- Declutter the session list rows and give built-in agents icons (#1215)
- Preserve scoped session row identity and accessible runtime details
- Bound CI change detection to compared commits
- Match subagent navigation against accessible runtime labels
- Recover an interrupted move to another profile after a daemon restart (#1222)
- Let agent duplicate drop an unsupported saved appearance (#1225)
- Keep session row actions working after a profile rename (#1228)
- Close another profile's terminal, agent and automation windows on a profile switch (#1226)
- Sweep a desk again when an owner read failed during a profile switch (follow-up from #1221)
- Preserve provider auth recovery boundaries
- Keep an encoded colon in a file href's name (follow-up from #1223)
- Name gallery runtimes like the live row, no model for provider-only (follow-up from #1215)
- Reset the declutter draft thread segment on direction switch (follow-up from #1215)
- Draw the built-in General icon before the agent catalog loads (follow-up from #1215)
- Apply one all_profiles read contract to terminal, agent-instance and agent reads (#1238)
- Use a non-auth ACP error for rate-limit recovery fixture
- Advertise the model in the desktop open-paths fixture
- Format profile authority regression tests
- Omit HTTP-only SDK examples for UDS profile capture
- Simplify profiling test context error selector
- Select the agent runtime after its creation scope
- Preserve palette catalog writes and refresh User catalogs
- Assert always-on agent creation succeeds
- Preserve agent runtime when changing creation scope
- Exercise runtime retention across agent scope changes
- Isolate skill refresh failures and scope catalog diagnostics
- Classify agent exits during daemon shutdown as interrupted, not crashed (#1245)
- Repair CI diagnostics and scope agent page title assertions
- Satisfy scoped Go lint after catalog performance changes
- Preserve revocation errors on canceled mutation requests
- Resolve native instance creation approval in integration journey
- Keep one page heading in agent setup editors
- Assert unique headings after correcting agent editor hierarchy
- Preserve canceled turns across daemon restart races
- Address PR review feedback on the Check-ins source UI
- Harden heartbeat source polling and SDK compatibility
- Preserve optional heartbeat source fields in extension SDK
- Retain persisted heartbeat hook cancellation reason in regression
- Keep extension runtime within the source size limit
- Register safe file reads in heartbeat source boundaries
- Format source status client contract coverage
- Collect usage for provider aliases and keep live native_cli sign-in verdicts (#1250)
- Reject ambiguous usage routes and edit refreshed provider settings
- Preserve heartbeat contract names while satisfying focused lint
- Persist restart skip before acknowledging shutdown cancellation
- Align source fixtures with profile ownership and post-poll skips
- Verify heartbeat source provide conformance in SDK integration
- Align provider auth contract coverage with probe timestamps
- Keep shutdown cancellation responsive without resuming canceled turns
- Decode instance response envelopes in source catalog coverage
- Allow telemetry for registered workspace identities after deletion
- Bound checkpoint shutdown and reject reduced benchmark throughput
- Preserve overflow proposal details in check-in reports
- Read durable cancellation marker envelopes during restart recovery
- Verify telemetry ownership after workspace re-registration
- Seed historical workspace fixtures before head migrations
- Preserve workspace registration before telemetry migration
- Isolate automation history fixture from recursive stop triggers
- Preserve agent error exports in settings skills mock

### Features

- Move a session to another profile (#1205)
- Mark a session's Needs you as seen (#1210)
- Open transcript file paths in the editor from the CompozyOS desktop app (#1223)
- Wait for a lost provider sign-in and resume the turn (#1229)
- Capture daemon runtime profiles over UDS (#1240)
- Rename the Stays on agent kind to Always-on and align permission and setup labels (#1237)
- Let always-on agents check external sources on each check-in (#1247)

### Performance Improvements

- Reduce repeated skill, catalog and cache work (#1244)
- Gate stream-covered interaction polls on catalog stream status (#1246)
- Isolate telemetry and shorten SQLite write locks (#1241)

### Refactoring

- Import profile owner helpers from the profiles barrel (follow-up from #1215)

### Testing

- Synchronize reply-watch integration waits on runtime edges (#1178)
- Cover a fresh derive that names the matching source profile (follow-up from #1218)
- Assert a profile move carries messages and names its recovery (follow-up from #1205)
- Assert the 8s window-manager command deadline (follow-up from #1212)
- Cover mark-seen profile refusal and retained failure (follow-up from #1210)
- Order two JSON number operands numerically (follow-up from #1213)
- Keep a valid saved appearance in the instance projection (follow-up from #1208)
- Assert move result fields and reach the sessionless move guard (follow-up from #1205)
- Record runtime writes in the owner-profile route matrix (follow-up from #1218)
- Cover the committed move response when enrichment fails (follow-up from #1205)
- Assert the exact TOON moved header and value order (follow-up from #1205)
- Return and store the requested worktree ID on create (follow-up from #1222)
- Resume terminal catalog WebSocket streams from last_event_id (follow-up from #1212)
- Format follow-up tests for the line-length lint
- Resume the aggregate terminal stream with its selection-bound cursor (follow-up from #1212)

### Release Notes

#### Features

##### Always-on agents check external sources on each check-in

An always-on agent can now watch something outside CompozyOS, such as new GitHub issues, without paying for a model turn every time it looks. Declare **check-in sources** in `HEARTBEAT.md` under `context.sources`. On each check-in CompozyOS asks every source what is new, without a model, and starts the read-only check only when a source or a built-in fact returned something new. An empty or unchanged result never starts a model turn.

```yaml
context:
  sources:
    - id: new-issues
      kind: forge-github/issues
      with:
        repo: compozy/compozy
        labels: [bug]
```

- **Two kinds of source.** Extensions provide source kinds through the new `heartbeat.source` provide surface, and the bundled `forge-github` extension ships `forge-github/issues`. You can also put your own scripts in the agent's `sources/` folder.
- **Scripts run only after you approve them.** CompozyOS runs a script only when its exact hash matches your approval, and editing the script needs a new approval. Approve with `compozy agent heartbeat sources approve <agent> <script>`, revoke with `compozy agent heartbeat sources revoke`, or use the Check-ins page. Agents can't approve scripts.
- **Nothing is missed.** A source's position advances only after a check has seen its items, so a failed or interrupted check sees the same items again. A source that has more items wakes the agent again at `agents.heartbeat.min_interval`.
- **Failures are visible.** Each source reports a typed error and backs off on its own. After `failure_threshold` failures in a row, the check-in report says so.
- **Where to find it:** `compozy agent heartbeat sources <agent>`, `sources` on `compozy agent instance show` and `check --dry-run`, the native tool `compozy__agent_heartbeat_sources`, the Check-ins page (add, edit, remove and approve sources), and Go/TS SDK helpers with a `heartbeat-source-go` scaffold. The new guide "Watch new GitHub issues" walks through a setup.
- **Config:** `[agents.heartbeat.sources]` sets `scripts_enabled`, `timeout`, `max_concurrent_polls`, `max_per_agent`, `max_items`, `max_output_bytes`, `max_state_bytes`, `failure_threshold` and `env_allow`. It is daemon-wide and needs a restart.

Agents without sources work as before.

### Migration notes

- The daemon applies two new database migrations at startup. No existing row or file changes.
- Older daemons reject `context.sources`. Upgrade every daemon that reads a `HEARTBEAT.md` before you add sources to it.

##### Capture runtime profiles from a running daemon

When the daemon is slow or busy, you can now capture a Go runtime profile from it without restarting. Run `compozy daemon profile <kind> --file <path>`, where `<kind>` is `cpu`, `heap`, `allocs`, `goroutine`, `mutex`, `block` or `threadcreate`.

```bash
compozy daemon profile cpu --seconds 30 --file cpu.pprof
go tool pprof -tagfocus session_id=<session-id> cpu.pprof
```

- CPU, mutex and block captures sample for 1 to 120 seconds (30 by default). Mutex and block sampling is on only during the capture. Only one CPU capture runs at a time; another returns `profile_busy`.
- Profiles are labeled by session, provider, tool and route, so you can narrow one to a single session. They don't include prompts, file contents, tool arguments or query strings.
- The file is written with owner-only permissions, and an existing file is kept if the capture fails.

This command is experimental. It works only over the local daemon socket: the `GET /api/daemon/profile/{kind}` route isn't served over HTTP or the Gateway, and app tokens can't use it.

##### Mark a session's Needs you as seen

A session in **Needs you** used to stay there until something changed, even after you had looked at it, and a failed session stayed until you archived it. Now you can mark it as seen from the session row menu, the Kanban card, or the button in the session header.

- **What it clears:** a turn that ended early with no pull request and no open work, a session back from Later, and a failed session. A failed session then reads as stopped and keeps its failure record. The next failure, early stop or Later return raises Needs you again.
- **What it keeps:** a pending permission, a question waiting for your answer, a missing provider sign-in, or a stop CompozyOS couldn't confirm still need you. Marking those as seen is refused, and the refusal names the answer still owed.
- Marking a child session as seen never wakes its parent.
- **From the terminal and agents:** `compozy session mark-seen <id>`, the native tool `compozy__session_mark_seen`, and HTTP/UDS `POST /api/workspaces/{workspace_id}/sessions/{session_id}/mark-seen`. A refusal returns `attention_requires_action`. Session payloads gain `attention_dismissible`, which is true only while the current Needs you can be marked as seen.

### Migration notes

The daemon applies a new database migration at startup. Existing sessions are unchanged.

##### Move a session to another profile

A session started in the wrong profile can now move to another one. **Move to profile** copies a stopped, top-level session into another active profile and archives the original. The copy keeps the agent, title and conversation, and uses the target profile's agents, credentials and config when it starts. The original keeps its history and past usage in its own profile.

- **Where:** the session row menu (**Move to profile**), `compozy session move-to-profile <id> <profile>`, and HTTP/UDS `POST /api/workspaces/{workspace_id}/sessions/{session_id}/move-to-profile`. Agents use `compozy__session_move_to_profile`, which needs the unrestricted workspace-access grant and can't move the agent's own session.
- **Worktrees move too.** A session bound to a worktree gets a new worktree on the same branch in the target profile. Every commit, including unpushed ones, stays on the branch.
- **What is refused:** running sessions (stop them first), sessions with child sessions, worktrees with uncommitted changes, and worktrees shared with another session. Each refusal returns a `session_move_to_profile_*` error that names the rule.
- **Interrupted moves recover.** If a move fails partway, CompozyOS rolls it back. If the daemon stops in the middle, it finishes or rolls back the move at the next start. The session and its branch are never deleted.

`compozy session move` still moves a session between folders.

### Migration notes

- The daemon applies a new database migration at startup. Existing worktrees keep their behavior.
- Deleting a session now returns 409 while a move of that session is still being recovered. It succeeds once the move settles at the next daemon start or the next move of that session.

##### Open transcript file paths in your editor from the desktop app

Agents often name files in their replies, such as `internal/mcp/hosted_proxy.go:42`. In the CompozyOS desktop app, **Cmd+click** (macOS) or **Ctrl+click** (Windows and Linux) a file path in an assistant message to open it at that line in your editor. An **Open in editor** button also appears on hover or focus.

- CompozyOS detects paths in inline code and in links whose whole text is a path, with an optional `:line` or `:line:col`. Plain text isn't detected.
- Relative paths resolve against the session's working directory, then its worktree, then the project root.
- **Choose the editor** in **Settings › General › Editor command**, or run `compozy config set desktop.editor_command 'code -g {path}:{line}:{col}'`. The value is a command template with `{path}`, `{line}` and `{col}`. Left empty, the system text editor opens the file, without line support.
- Files outside the session's worktree or project, and credential folders such as `~/.ssh` and `~/.aws`, ask you to confirm first. The dialog shows the full path, and Cancel is the default.
- The browser and remote clients (LAN, phone, Gateway) show paths as before and can't start anything on the daemon's machine.

`desktop.editor_command` is set per machine; project and profile config can't set it, and agents can't change it. Single-session reads gain a read-only `working_dir` field.

##### Turns wait for a lost provider sign-in and then continue

Switching accounts in a provider CLI, such as signing out of a Claude Code or Codex account near its limit and into another, used to fail every session that sent a prompt in that moment with `Authentication required`. You then had to resume each one by hand. Now a turn that loses the provider sign-in waits for it to come back and continues by itself, for every ACP provider.

- The session shows "Waiting for the provider sign-in…" with "Retrying automatically · Attempt N of M".
- Retries start after 15 seconds and double up to one a minute, for up to 10 minutes.
- A turn that hadn't started work is sent again as it was. A turn that had already started work gets a short note that the sign-in dropped and that it should check what is done before repeating anything. Your message is never recorded twice.
- If the sign-in doesn't come back in time, the turn fails as it did before. Later turns in that session then fail right away until one completes, instead of each waiting again. Cancelling the prompt ends the wait.
- Session reads show the cause in `runtime.recovery.reason` (`provider_auth` or `runtime_exit`). The existing `session.runtime_recovery.*` hooks also fire for these waits.

Configure it in the new global-only `[session.auth_recovery]` table (`enabled`, `window`, `initial_delay`, `max_delay`). It needs a restart. Set `session.auth_recovery.enabled = false` to fail right away, as before.

#### Fixes

##### Always-on agents run unattended at Accept edits and Auto-review

An always-on agent at **Accept edits** (`approve-edits`) or **Auto-review** (`auto`) used to ask you before almost every CompozyOS tool call, including monitors, child sessions, worktrees, status reads and skill reads. The built-in Orchestrator couldn't run unattended unless you copied it at **Full access**.

- At those levels, CompozyOS tools that change things without destroying them now run without asking. Examples are creating sessions and worktrees, binding a worktree, renaming and linking sessions, task bookkeeping and layouts.
- These still ask: destructive and open-world tools, terminal commands, Loop and Automation tools, config, tool approvals, hooks, extensions, provider model changes, and creating, pausing or resuming agents.
- A call you approve at the provider prompt is no longer asked about a second time.
- CompozyOS tools now follow the session's own permission level, the same level the provider uses.
- The built-in Orchestrator now ships with `permissions: auto`.

`compozy__session_create` runs without asking at these levels and starts the child at the target agent's own level, which can be higher than the caller's.

Other always-on agent fixes:

- The **Model** row on an agent's home is now the model picker itself. A change saves at once, and **Use default** clears the agent's own choice.
- **Add always-on agent** is in the session list toolbar `+` in every view, not only in a project row.
- Changing an agent's avatar or color from its profile header saves again instead of failing with `invalid agent definition appearance`. An unsupported saved appearance no longer fails an agent read or `compozy agent duplicate`; the copy gets the default face.
- Cancelling your own monitor with `compozy__monitor_cancel` no longer reports "A wake-up didn't arrive".

##### Stopping the daemon no longer records running agents as crashed

When the daemon was stopped from outside while a session was mid-turn, for example by logging out of the OS, shutting down or quitting the desktop app, the session was recorded as failed with "ACP subprocess exited unexpectedly". That also wrote a crash bundle, fired `agent.crashed`, tried to restart the agent and sent the parent a `failed` wake.

Now an agent that exits while the daemon is shutting down is recorded as stopped by the daemon. No crash bundle or `agent.crashed` follows, the daemon doesn't restart agents while it stops, and a parent gets a `stopped` wake. After a restart, the interrupted turn still continues. An agent that exits while the daemon keeps running is still a crash.

##### Less repeated work when many sessions run

A round of fixes from a performance review of many sessions running at once.

- **Clicks no longer stall in the desktop app.** Clicks on sessions and window tabs could hang for tens of seconds and then apply all at once, with "Can't save window layout — retrying". Three long-lived streams held half of the browser's connections; they now use WebSocket. A window command still waiting after 8 seconds is dropped instead of replayed.
- **Skills are reparsed only when they change.** A failed extension refresh no longer leaves skill discovery stale, and healthy profiles and projects stay usable when one fails.
- **Fewer catalog reads and writes.** Window transcripts follow the 5-second refresh window, cached catalogs are written only when they change, and pending-answer views refresh from the live stream instead of polling every 5 or 15 seconds.
- **Model discovery stops retrying sources that can't work,** such as a missing CLI. **Refresh** still tries them, and temporary failures still retry, backing off to at most 2 hours.
- **The desktop app caps its HTTP cache at 256 MiB** and trims a larger one at launch. Your data and sign-ins are kept.
- **Cancelled requests return 499** and no longer log as server errors.

##### Many sessions no longer wait on one database lock

With around 30 active sessions, transcript updates, session health, model refreshes, process checks and a large event-summary log all waited on the same SQLite resources, so the app and agents slowed down together. CompozyOS now keeps that work apart.

- Interactive writes, such as sending a prompt or changing a session, go ahead of background work, which waits in a bounded queue.
- Global, profile and session reads have their own read pools, and transcript work is prepared before it takes the write lock.
- Session health is written in batches every 500 ms and flushed on a clean shutdown.
- Event summaries move to their own database, `$COMPOZY_HOME/compozy.db.telemetry.db`, which has its own write lock.

CLI commands, HTTP/UDS routes, native tools, Observe filters and cursors, hooks and config keys are unchanged.

### Migration notes

- On the first start after the upgrade, the daemon moves existing event summaries into `compozy.db.telemetry.db` in small batches and checks every row before it removes the old copy. On a large installation this first start takes longer and needs disk space for the new file. An interrupted move resumes at the next start.
- Back up and restore `compozy.db`, `compozy.db.telemetry.db` and their `-wal` and `-shm` files together. The two databases carry a matching identity. If the telemetry file is missing, empty or comes from another installation, the daemon refuses to start instead of replacing it; restore both files from the same backup. See **Operations › Database** for a live backup recipe.

##### Profiles stay separate across windows, routes and reads

A review of how profiles are kept apart found and closed several gaps.

- **Switching profiles closes the other profile's windows.** A session, terminal, agent or automation window opened under **All profiles** no longer stays on screen with "This session belongs to …" after you switch into one profile. It leaves the desk without stopping anything, so another profile's terminal keeps running, and you can reopen the window.
- **Agents open in their own profile.** Opening an always-on agent owned by another profile from All profiles no longer shows "Agent not found". Its window, **New session** and **Duplicate** use the agent's profile.
- **Routes check the session's profile.** Project session routes (prompt, transcript, stream, usage and the rest) now return 404 for a session owned by another profile when you select a profile. `all_profiles=true` allows reads and refuses writes with `profile_selection_conflict`. Without a selector, a route still uses the session's own profile.
- **Agent-instance writes refuse `all_profiles=true`.** Pause, resume, check, archive and the other instance writes can no longer reach another profile's instance through the aggregate.
- **One rule for `all_profiles` reads.** Terminal lists, details, streams and downloads, agent-instance lists and details, and the native tools `compozy__agent_list`, `compozy__agent_instance_get`, `compozy__agent_instance_list` and `compozy__agent_activity` accept `all_profiles` the same way, with rows labeled by `profile_name`. The instance list no longer returns 422.
- **The command palette sees only your profile's clients.** A palette call can no longer list, read or send commands to a browser tab open on another profile. `compozy cmd-palette clients --all-profiles` and `GET /api/cmd-palette/clients?all_profiles=true` still list every profile.
- **Renaming a profile keeps row actions working.** Rename, archive, Later and Move to profile on that profile's sessions no longer fail with `profile_not_found` until a reload.
- **An archived profile no longer stops the daemon from starting.** The daemon used to exit with `start provider usage: profile "<name>" is archived`, which left the desktop app on its boot error screen.

### Migration notes

Scripts that called project session routes with `profile=<other>` for a session owned by a different profile now get 404. Name the owning profile, leave the selector out, or use `all_profiles=true` for reads. `GET /api/cmd-palette/clients` now defaults to your profile; pass `all_profiles=true` for the old list.

##### Provider usage for extra accounts and verified native CLI sign-ins

Two fixes for running more than one native CLI account.

- **Usage for custom providers.** A custom provider that points at a built-in, such as a second Claude account with `runtime_provider = "claude"` and `auth_mode = "native_cli"`, used to show `unsupported` / `no_collector` in Provider usage. It now uses the matching collector and shows as its own row. Usage collection also applies an `env NAME=value … cmd` launch prefix, so each account reads its own config folder, such as `CLAUDE_CONFIG_DIR=~/.claude-mm1`, instead of mixing with your main account.
- **"Sign-in unverified" no longer sticks.** Settings › Providers and `compozy provider list` showed "Sign-in unverified" for every `native_cli` provider, even right after a sign-in check passed. The daemon now remembers the last result from `compozy provider auth status`, from **Check sign-in**, and from session starts, and shows when it checked. **Check sign-in** now checks in place instead of opening the editor. Changing the provider's config, or restarting the daemon, clears the remembered result.

##### Smaller fixes

- **Mermaid diagrams:** the mouse wheel scrolls the transcript when the pointer is over a diagram.
- **Worktrees:** `compozy worktree remove` works when the worktree's base branch was merged and deleted, instead of failing with `worktree_safety_check_failed`. It checks for commits no other branch, tag or remote keeps.
- **Agents sending prompts:** `compozy__session_prompt` no longer returns a schema error for a prompt that was queued, held or sent to another session. The prompt was always delivered.
- **Loop templates** accept `eq`, `ne`, `lt`, `le`, `gt`, `ge`, `not`, `and` and `or`, so a step can see which gate rejected the last attempt.
- **Session list:** rows are less crowded, and built-in agents have icons.
- **Session header:** a session with several linked pull requests shows one "N linked PRs" chip that opens the list, instead of one PR plus `+N`.

#### Highlights

##### Stays on is now Always-on in v0.5.1

The agent kind that keeps running on its own is now called **Always-on**, and its value is `always_on`. Every CompozyOS surface uses the new name: the app, the CLI, `config.toml`, HTTP/UDS, native tools, the SDKs, the official skill, and the docs. On-demand agents are unchanged. The app now also uses the labels other agent tools use for permission levels and agent setup:

- Permission levels: **Accept edits** (`approve-edits`, was "Edit on its own"), **Auto-review** (`auto`, was "Let a reviewer decide"), and **Full access** (`approve-all`, was "Never ask"). Wire values are unchanged.
- The agent page section is **Setup**: Instructions, Model, Heartbeat, Memory, Tools, and Permissions. A scheduled check now reads "heartbeat". Page URLs are unchanged.

### Migration notes

v0.5.1 upgrades in place: v0.5.0 files, config, request values, and stored rows keep working through v0.5.1. Readers of emitted `kind`, reason, and error values must update now (see below). Every alias below is removed in **v0.5.2**.

- **Update the daemon together with the CLI.** The CLI doesn't check the daemon's version. A v0.5.1 CLI always sends `always_on`, which a v0.5.0 daemon rejects.
- **AGENT.md `kind: stays_on` → `kind: always_on`.** The old value loads as `always_on` through v0.5.1, and the daemon logs one warning per file. On boot, the daemon rewrites the `kind` value of every AGENT.md under `$COMPOZY_HOME/agents` and `$COMPOZY_HOME/profiles/<profile>/agents`. It changes no other byte and logs each rewrite and each skip. AGENT.md files in a repository, a workspace profile, or an extension are never rewritten, so edit those yourself before v0.5.2.
- **`[agents.stays_on]` → `[agents.always_on]`.** The old table is read with one warning per process, and a key set in `[agents.always_on]` wins. `compozy config get|set|unset` and `compozy__config_*` accept `agents.stays_on.*` paths, warn, and operate on `agents.always_on.*`. `unset` through either spelling clears the key from both tables in the file it edits. Defaults are unchanged.
- **`--kind stays_on` → `--kind always_on`.** The CLI accepts the old value with a stderr warning.
- **Request enums vs. emitted values.** Request `kind` fields (create, update, duplicate, its `overrides.kind`, and `compozy__agent_create`) accept `on_demand`, `always_on`, or the deprecated `stays_on`. The generated TypeScript and Go request types widen to match. Responses, events, tool results, and CLI `-o json` emit `always_on` from this release on, so readers that compare `kind` to `stays_on` break now.
- **`definition_not_stays_on` → `definition_not_always_on`.** This changes at once in the instance API `pause_reason` field, the instance check `gate` (`compozy agent instance check` and `compozy__agent_instance_check`), the check-in history `reason`, and the `agent.instance.*` hook payload `reason`. Stored rows are migrated. Hook consumers that compare the old value break.
- **`agent_not_stays_on` → `agent_not_always_on`.** The error code changes at once. The experimental agent memory routes break with no compatibility window.
- **Startup section `stays_on_doctrine` → `always_on_doctrine`.** The tag is now `<compozy-always-on-doctrine>`. Startup-section diagnostics show the new name.
- **Permission labels.** Only the labels changed. `deny-all`, `approve-reads`, `approve-edits`, `auto`, and `approve-all` are unchanged in files, config, and the API.
- **v0.5.2 removes every shim.** That covers the `stays_on` kind alias, `[agents.stays_on]`, the `agents.stays_on.*` paths, the `stays_on` request enum value, and the boot rewrite. After that, `stays_on` fails with an error that names `always_on`.
