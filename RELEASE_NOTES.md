## 0.5.4 - 2026-11-07

### Bug Fixes

- Retain oversized tool results in User sessions (#1322)
- Allow profile moves with stopped subagent descendants (#1321)
- Compare SQLite commit pauses against paired storage tails (#1325)
- Accept fenced proactive triage replies (#1319)
- Preserve managed Goal pause ownership until coordinator settlement (#1329)
- Read proactive check events in heartbeat status (#1332)
- Hold MCP server form errors until a field is touched (#1338)
- Settle expired permissions and stopped session interactions (#1335)
- Keep the CompozyOS desktop window open when Settings restarts the daemon (#1339)
- Compact always-on agent cards and hide deleted agents (#1341)
- Keep dispatched children responsive after definition pauses (#1333)
- Defer oversized invoked skills to skill_view instead of failing dispatch (#1330)
- Refresh MCP tools in open sessions (#1342)
- Preserve permission recovery reports and terminal refusals (#1347)

### Features

- Route new issues through an unattended issue vetter
- Add a desktop download page and header download CTA to the site
- Add daily CompozyOS daemon log triage agent (#1353)
- Add project file explorer, file viewer and session Changes panel (#1351)

### Refactoring

- Rename the workspace always-on agents by their pipeline role

### Testing

- Observe paused child monitor settlement before receipts (#1349)

### Release Notes

#### Fixes

##### A denser Agents list

- **Compact always-on cards.** Always-on agents now use a smaller card: five per row on a wide window instead of three, with the description on one line and status and recent activity at a glance. Twenty agents fit on one screen.
- **Deleted agents are gone.** An always-on agent whose definition was deleted no longer lingers in the list or in the filter counts, and no longer opens a "not found" page.

##### Always-on agents keep working through pauses, renames and failed checks

- **Work dispatched by a paused agent keeps going.** If an always-on agent's definition was deleted or renamed, the sessions it had started stopped receiving their own monitor wakes and messages, and looked like they were just waiting. Those sessions now keep running. When input really is held, the reason and the agent holding it are shown, and the session asks for your attention instead of waiting silently.
- **Proactive checks accept fenced replies.** A check failed with "The proactive check returned an invalid response" when the model wrapped a valid answer in a Markdown code block. Fenced answers are now accepted, and a rejected answer is recorded in the failure report with its contents redacted. A failed check never drops the items it was looking at; the next check sees them again.
- **Heartbeat status opens again.** Reading an always-on agent's heartbeat status failed with `invalid wake event: invalid source "timer"` once proactive checks had run. Check runs now appear in the heartbeat history.
- **Pausing an agent with a running Goal.** Pausing an always-on agent while one of its Goals was finishing a turn could lose track of that pause, so the Goal kept running or could not be resumed. Pause and resume now hold through that window.

##### Restarting CompozyOS from Settings keeps the app open

Some settings ask you to restart CompozyOS. Clicking **Restart** used to close the whole desktop app whenever the restart took more than a few seconds, for example with busy sessions or a data migration, and you had to open it again by hand. The window now stays open. It shows that CompozyOS is restarting, reconnects to the new daemon and returns you to where you were. Runtime updates that restart the daemon get the same treatment.

##### MCP servers reach your open sessions

Adding or signing in to an MCP server could show it as Ready in Settings while a session that was already open still reported it as unavailable. MCP servers declared in your user or profile `mcp.json` could also be missing from new sessions. Open sessions now pick up an MCP server as soon as you add or authorize it, and lose access when you sign out, disable or remove it, without restarting CompozyOS or losing the conversation. Claude sessions refresh their tool list in place. Codex sessions reload at the next turn boundary, because Codex does not yet re-list MCP tools mid-session.

##### Expired permission requests no longer stay pending

A permission request that expired, or whose session was stopped, could stay **pending** for hours. The session kept showing "waiting for auth" with approval buttons that failed. Requests now close when they expire and when their session stops, is force-stopped or crashes, and the session's attention state clears. Requests left over from before a daemon restart are settled when it starts again. Answering one that already closed explains that it expired. Thanks to @zaptoscompany for the detailed report.

##### Smaller fixes

- **Large skills no longer fail the prompt.** Invoking a skill whose instructions are too large for the turn used to fail the whole prompt with "session prompt dispatch is indeterminate". The agent now gets a pointer to read the skill with `skill_view` instead. Thanks to @madtocc for the fix.
- **Large tool results in User sessions.** Oversized results such as a long `skill_view` are now saved and readable in User sessions, not just in projects. Thanks to @zaptoscompany for the report.
- **Moving a session with stopped sub-agents.** A session whose sub-agents are stopped but not archived can now move to another profile.
- **Add MCP server form.** The form no longer shows "required" errors before you type anything. Errors appear after you leave a field empty or try to save.

## 0.5.3 - 2026-11-06

### Bug Fixes

- Show a pending runtime switch on session rows (#1294)
- Allow profile moves with archived descendants (#1299)
- Wake pull request monitors when merge conflicts appear (#1289)
- Keep code_implementer working until its item is done
- Hide session chrome that doesn't apply to always-on main sessions (#1304)
- Defer fallback steering until active tools finish (#1296)
- Enable sign-in for bare OAuth MCP remotes (#1308)
- Let agents attach MCP servers (#1309)
- Align OAuth browser journeys with sign-in action (#1310)
- Keep native CLI sign-in verdicts truthful (#1278)
- Load skill resources in User sessions (#1312)
- Back off provider usage 429s, collect each account once, and re-verify native sign-in after restart (#1307)
- Keep CompozyOS usage aliases on their own account (#1280)

### Features

- Remove retired agent kind compatibility inputs (#1295)
- Add a Marketplace MCPs view with a Plugins | MCPs toggle (#1306)

### Performance Improvements

- Batch forge lookups and refresh displayed branches on demand (#1291)

### Testing

- Give the post-upgrade phase of the destructive-cut reopen test its own budget

### Release Notes

#### Breaking Changes

##### Retired agent kind inputs now report actionable errors

The release after v0.5.2 removes the agent kind input aliases retained since v0.5.1.
Use `always_on` in definitions, CLI flags, HTTP/UDS requests and native creation.

## Migration notes

Persisted `[agents.stays_on]` tables migrate on load to `[agents.always_on]`. Canonical keys
win conflicts; original values and source text are retained as inactive comments. Persisted
AGENT.md files migrate when loaded from any discovery root, including extension roots.
Writable user-owned scalar layouts update only the kind value. Extension packages are never edited. Read-only files and unsupported edit layouts
still decode as `always_on` and log guidance to update the source before v0.6.0.
This single persisted-file shim generation is scheduled for removal in v0.6.0.
CLI/native `agents.stays_on.*` paths still fail with replacement guidance; public creation,
update and duplicate inputs still reject the retired kind. Database rows already migrated in v0.5.1.

- Old kinds fail with `agent_kind_invalid`: "agent kind must be on_demand or always_on (stays_on was renamed to always_on in v0.5.1)".
- Old tables in persisted config files migrate on load; API/config mutation paths reject the retired key.
- Old config paths fail with `removed config key "agents.stays_on.max_proposals": use "agents.always_on.max_proposals"` (with the requested suffix).

No new database migration is needed: v0.5.1 already migrated persisted rows.

#### Fixes

##### Provider usage works with several accounts and profiles

If you run more than one Claude account through provider aliases (for example `claude-mm1` with its own `CLAUDE_CONFIG_DIR`), or use more than one profile, Provider usage now behaves correctly.

- **No more 429 storms.** A rate-limit reply with `Retry-After: 0` made CompozyOS retry right away and get rate-limited again. Every 429 now pauses that account for at least 5 minutes, longer if it repeats.
- **One request per account.** Each profile used to collect the same account separately. Collection is now shared per account, so two profiles no longer double the requests.
- **Aliases show their own account.** An alias whose launch command doesn't set its own account variable used to show the default account's quota. It now shows no usage instead of the wrong one.
- **Toggles and logos for aliases.** Settings → Provider usage lists every eligible alias with an on/off switch and its provider's logo, and you can edit the provider list while collection is off.

##### Provider sign-in status stays accurate

The sign-in badge for CLI providers could be wrong in a few cases. It now reports what is actually true.

- **After a restart.** Sign-in used to show as unverified until a session started or you clicked **Check sign-in**. The daemon now checks each installed CLI provider once in the background after it starts.
- **When the CLI is broken.** A saved **Ready** no longer hides a CLI that can't be found or run.
- **With a different account folder.** A session that signs in with another `HOME`, `CLAUDE_CONFIG_DIR` or `CODEX_HOME` no longer marks the default account as Ready.
- **After a timeout.** A check that timed out is no longer recorded as **Needs login**.
- **Ready looks ready.** The Ready dot in Settings now uses the success color.

##### Smaller fixes

- **Skills in User sessions.** User sessions can read a bundled skill's resource files again. Thanks to @zaptoscompany for the detailed report.
- **Pending model switch is visible.** When you switch a running session to another provider, its sidebar row and board card show `current → next` until the next message applies it.
- **Cleaner always-on sessions.** An always-on agent's main session hides controls that don't apply to it, such as the linked-work chip, Stop session and the sidebar toggles.
- **PR monitors notice merge conflicts.** A session watching a pull request now wakes up once when the PR starts to conflict with its base branch.
- **Moving sessions between profiles.** A session whose sub-sessions are all stopped and archived can now move to another profile.
- **Fewer GitHub calls.** Branch and PR lookups are batched, and branches you're looking at refresh on demand.

##### Steering no longer cancels a running tool

Some providers can't take a message in the middle of a turn. With those, sending a message while the agent was running a tool could cancel the tool right away. CompozyOS now waits until the running tools finish, then delivers your message. It waits at most 45 seconds, which you can change with `session.busy_input.steer_boundary_timeout`. Providers that can take messages mid-turn get them right away, as before.

#### Highlights

##### Sign in to MCP servers, attach them to agents, and find them in the Marketplace

MCP servers are now easier to reach and connect, wherever you start.

- **Sign in to any OAuth remote.** A plain HTTP MCP remote that reported **Needs sign-in** used to offer only Edit, and `compozy mcp login` rejected it. Remotes without custom headers now discover their authorization server and register a client automatically, so **Sign in** just works from Settings and the CLI. The Advanced editor has a working **Use OAuth** toggle. Remotes that authenticate with headers are left alone.
- **Attach MCP servers to an agent.** An agent's Tools sheet showed its MCP servers but could not add one. It now offers **Add MCP server**: pick a configured server or create a new connection. Configured servers are saved by name in the agent's `mcp.json`, so they follow later edits to that server. If the server is removed, it shows **Broken reference**. Secrets stay in your Vault.
- **MCPs in the Marketplace.** A **Plugins | MCPs** toggle next to the Marketplace search opens your MCP servers with the same status, sign-in, edit and remove actions as Settings, plus **Add MCP server**.

## 0.5.2 - 2026-11-04

### Bug Fixes

- Keep generated avatars for bundled always-on agents (#1260)
- Bump next to 16.4.5 (November 2026 security release)
- Stop CompozyOS PR display polling from exhausting the GitHub quota (#1268)
- Attach CompozyOS desktop bootstrap to a live, still-booting daemon (#1255)
- Expose CompozyOS CLI after macOS desktop installation (#1261)
- Remember cross-workspace read consent for always-on agents (#1274)
- Allow script source approval before the first check-in (#1262)
- Show CompozyOS data upgrade progress during coordinated restarts (#1256)
- Make SQLite contention acceptance measure untraced work (#1285)
- Confirm Claude triage models and report configuration rejections (#1276)

### Features

- Override bundled agent permissions without forking its prompt (#1273)

### Performance Improvements

- Bound agent activity reads and batch team pulses (#1288)

### Testing

- Fix loop editor, agent select, inspector and context menu story debt (#1277)

### Release Notes

#### Features

##### Approve a check-in script as soon as it exists

A new or edited check-in script used to ask for approval only after a check had already tried to run it. On a new or paused agent, that check never came, so the script stayed at "Not checked yet" with nothing to approve.

- **Approve right away.** The Heartbeat page shows a "needs your approval" notice for each waiting script as soon as it exists, with its size, when it last changed, and an Open script action in the desktop app.
- **Approve from the bell.** The menubar bell lists scripts waiting for approval, and you can approve them from there.
- **Paused agents.** On a paused agent, an approved script runs as soon as you resume it.

Approval is still operator-only and tied to the script's exact contents.

##### Change a bundled agent's permissions without copying it

You no longer need to copy a bundled or extension agent's whole `AGENT.md` just to run it with different permissions or a different model. Override only the fields you need in `config.toml`:

```toml
[agents.overrides.orchestrator]
permissions = "approve-all"
# provider, model, reasoning_effort and speed are optional
```

The override applies to whichever definition of that agent is in effect: workspace, global, extension or built-in. Everything you don't set, including the prompt, appearance, tools, skills and MCP servers, keeps following the original, so package updates still reach you. Agents can't change this setting themselves.

#### Fixes

##### The compozy command works after installing the macOS app

Installing CompozyOS from the DMG ran the runtime but left no `compozy` command in your terminal. Only sessions inside the app could find it.

On first launch, the macOS app now links `compozy` into `/usr/local/bin`, or into `~/.local/bin` when that folder is already on your shell's PATH. If it needs administrator rights, it asks first. If the link is missing, use **Install compozy Command…** in the CompozyOS menu to add it. Existing commands are never overwritten. App updates keep the command working without reinstalling, and `compozy --version` now works too.

##### Smaller fixes

- **Claude check-ins work again.** Hidden check-in triage on Claude always-on agents no longer fails with `provider did not confirm config option "model"`. Configuration rejections are now reported as such, instead of counting toward the automatic pause.
- **Faster Agents pages.** The Agents team and agent profiles load their activity in a few grouped reads instead of one scan per member and workspace. Pages that took 2–3 seconds now open without the CPU spike.
- **Bundled agents keep their avatars.** Bundled always-on agents such as the Orchestrator show their generated avatar again instead of a generic icon.
- **Security update.** The web UI uses Next.js 16.4.5, the November 2026 security release.

##### Long data upgrades show progress instead of looking stuck

The first start after upgrading could spend several minutes moving telemetry out of a large database. During that time the desktop app sat on "Preparing CompozyOS" with no sign of progress, and quitting it made the upgrade look failed.

- **Visible progress.** The desktop boot screen now shows "Upgrading data…" with a progress bar, and `compozy daemon bootstrap` reports "Upgrading CompozyOS data (N%)", while that one-time move runs.
- **No false failures.** If the desktop app starts while a CompozyOS runtime is still booting, it now waits for that runtime instead of trying five more launches and reporting a failure.

#### Highlights

##### PR badges no longer use up your GitHub quota

CompozyOS shows a pull request badge on every session and worktree. To keep those badges current, the daemon polled GitHub with your own `gh` login, and that polling alone could exhaust the 5,000 requests/hour that you, `gh` and your agents share. Once the quota ran out, `gh` commands and agents started failing with rate-limit errors.

- **Only live worktrees refresh.** Removed, dismissed and missing worktrees keep showing their last known PR without calling GitHub.
- **Fewer checks for quiet branches.** A branch with no PR is rechecked after 10 minutes, then less often (up to an hour) while it still has none. A merged or closed PR is rechecked every 30 minutes. Open PRs still refresh every minute.
- **No more refresh loops.** A refresh that changes nothing no longer triggers another refresh, and repository capability lookups are cached.

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

v0.5.1 upgrades in place: v0.5.0 files, config, request values, and stored rows keep working through v0.5.1. Readers of emitted `kind`, reason, and error values must update now (see below). Every alias below is removed in **the release after v0.5.2**.

- **Update the daemon together with the CLI.** The CLI doesn't check the daemon's version. A v0.5.1 CLI always sends `always_on`, which a v0.5.0 daemon rejects.
- **AGENT.md `kind: stays_on` → `kind: always_on`.** The old value loads as `always_on` through v0.5.1, and the daemon logs one warning per file. On boot, the daemon rewrites the `kind` value of every AGENT.md under `$COMPOZY_HOME/agents` and `$COMPOZY_HOME/profiles/<profile>/agents`. It changes no other byte and logs each rewrite and each skip. AGENT.md files in a repository, a workspace profile, or an extension are never rewritten, so edit those yourself before upgrading beyond v0.5.2.
- **`[agents.stays_on]` → `[agents.always_on]`.** The old table is read with one warning per process, and a key set in `[agents.always_on]` wins. `compozy config get|set|unset` and `compozy__config_*` accept `agents.stays_on.*` paths, warn, and operate on `agents.always_on.*`. `unset` through either spelling clears the key from both tables in the file it edits. Defaults are unchanged.
- **`--kind stays_on` → `--kind always_on`.** The CLI accepts the old value with a stderr warning.
- **Request enums vs. emitted values.** Request `kind` fields (create, update, duplicate, its `overrides.kind`, and `compozy__agent_create`) accept `on_demand`, `always_on`, or the deprecated `stays_on`. The generated TypeScript and Go request types widen to match. Responses, events, tool results, and CLI `-o json` emit `always_on` from this release on, so readers that compare `kind` to `stays_on` break now.
- **`definition_not_stays_on` → `definition_not_always_on`.** This changes at once in the instance API `pause_reason` field, the instance check `gate` (`compozy agent instance check` and `compozy__agent_instance_check`), the check-in history `reason`, and the `agent.instance.*` hook payload `reason`. Stored rows are migrated. Hook consumers that compare the old value break.
- **`agent_not_stays_on` → `agent_not_always_on`.** The error code changes at once. The experimental agent memory routes break with no compatibility window.
- **Startup section `stays_on_doctrine` → `always_on_doctrine`.** The tag is now `<compozy-always-on-doctrine>`. Startup-section diagnostics show the new name.
- **Permission labels.** Only the labels changed. `deny-all`, `approve-reads`, `approve-edits`, `auto`, and `approve-all` are unchanged in files, config, and the API.
- **The release after v0.5.2 removes every shim.** That covers the `stays_on` kind alias, `[agents.stays_on]`, the `agents.stays_on.*` paths, the `stays_on` request enum value, and the boot rewrite. After that, `stays_on` fails with an error that names `always_on`.

## 0.5.0 - 2026-11-03

### Bug Fixes

- Deliver late hosted MCP tools and bound projection discovery (#1135)
- Recover cold npm adapter starts before ACP initialization (#1136)
- Recover superseded extensions and interrupted installs (#1137)
- Preserve shared composer images and recover missing attachments (#1144)
- Strip label prefixes from automatic session titles (#1145)
- Keep QA provider credentials opt-in and remove lab copies on teardown (#1150)
- Run stays-on agents at their configured permission level (#1157)
- Keep allowed permission receipts with their tool (#1158)
- Coalesce monitor wakes outside the operator input queue (#1159)
- Suspend Goal turns while external work is pending (#1151)
- Resolve merge train review findings on main
- Claim subagent wakes parked during a parent resume
- Release Goal waits after wait-mode children and keep wakes in generation
- Clear the remaining main lint and integration failures
- Keep finished automation groups reachable from a grouped search
- Keep the linked work list open after an unlink leaves one item
- Keep the linked work list open when Undo is pressed in its toast
- Use CompozyOS product language and format the skill references
- Close cross-scope review gaps in approve-all session reads (#1173)
- Keep store failures out of the masked profile conflict
- Repair main CI after the stays-on merge
- Install an approved skill before recording the approval
- Use CompozyOS product language in the permissions draft
- Keep scenario-configured model discovery in mock-agent E2E daemons
- Drop the builtin reviewer mode when a provider is routed to another runtime
- Match attributed delegated child prompts in the subagent CLI fixture
- Match the attributed delegated child prompt in the update work fixture
- Keep permission requests lost to a disconnect askable after recovery

### Features

- Let archive and delete include child sessions (#1131)
- Add one View options menu with Collapse all and Kanban lane folds (#1133)
- Add stays-on agents and agent identity
- Reveal subagent model, kind and activity in the composer panel (#1147)
- Let a session monitor wait on other sessions (#1146)
- Add a pull_request session monitor (#1156)
- Simplify spec-cycle to agents and spec authoring (#1152)
- **BREAKING:** simplify spec-cycle to agents and spec authoring (#1152)
- Hide finished automation sessions from the default list (#1163)
- Link worktrees, branches and pull requests to sessions (#1161)
- Let approve-all sessions read sessions across workspaces and profiles (#1171)
- Make GitHub Copilot CLI a first-class provider (#1170)
- Make stays-on agents safe to leave running (#1174)

### Performance Improvements

- Improve responsiveness under concurrent sessions (#1141)

### Refactoring

- Extract the skill approval ledger write

### Testing

- Stop the stale-entry promotion hook from blocking its dispatch
- Align integration expectations with merged Goal and instance contracts
- Expect the authored level after instance context replacement

### Release Notes

#### Breaking Changes

##### Spec delivery moves from Loops to agents

The `spec-cycle` extension now ships 4 agents and 2 skills, with no Loops and no tools. Deliver a spec by starting `spec_conductor`, and hand small work to the stays-on `orchestrator`, which stops for your approval when it writes unless you give it a higher level. Each agent carries its own procedure, so picking the agent is enough.

- `cy-create-spec` now writes `_slices.md` and offers peer review after you approve a spec.
- A new check-in source, `work_log`, surfaces accepted work in `.compozy/work-log.md` that never started.
- Batuta leaves the catalog because it is built on the removed Loops. An installed copy keeps loading, and only `batuta-deliver` fails when run.

### Migration

This is a hard cut with no deprecation window, so the resources below are gone in this release. Upgrading needs no manual step: CompozyOS starts normally, and nothing under `~/.compozy` needs deleting. Existing sessions, automations, stored Loop overrides and `.compozy/tasks` files are kept.

| Removed                                                                             | Use instead                               |
| ----------------------------------------------------------------------------------- | ----------------------------------------- |
| Loop `implement-tasks`                                                              | Agent `spec_conductor`                    |
| Loop `review-and-fix`                                                               | Agent `reviewer`, then `code_implementer` |
| Agent `review_fixer`                                                                | Agent `code_implementer`                  |
| `ext__spec_cycle__import_tasks`                                                     | `_slices.md` read by `spec_conductor`     |
| `ext__spec_cycle__write_review_artifacts`, `ext__spec_cycle__finalize_review_round` | Agent `reviewer` findings file            |
| Skill `cy-create-tasks`                                                             | `_slices.md` in `cy-create-spec`          |
| Skills `cy-execute-task`, `cy-final-verify`, `cy-workflow-memory`                   | Agent `code_implementer`                  |
| Skill `cy-orchestrate-tasks`                                                        | Agent `spec_conductor`                    |
| Skills `cy-review-round`, `cy-fix-reviews`                                          | Agents `reviewer` and `code_implementer`  |

Anything pointing at a removed resource fails only when it is used:

- Running a removed Loop, starting a session with a removed agent, or calling a removed tool fails with a not found error. Automations that target them stay saved, and each run fails on its own.
- A run of a removed Loop that is active during the upgrade fails at its first removed step with a clear error. Its history stays, and other runs are unaffected.
- A stays-on instance of a removed agent pauses as `definition_missing`, and nothing is deleted.
- A workspace Loop with one of the removed names keeps working.

#### Features

##### Agents can wait for other sessions to finish

An agent can now hand work to several sessions, end its turn, and get one wake when they are done. Agents use it through `compozy__monitor_create` with `kind: session`, and the wake lists each session and its state. A session with an active monitor counts as having open work, so a quiet orchestrator is no longer stopped after 40 minutes.

- A monitor watches 1 to 32 sessions in the same project and profile, until `any` or `all` of them reach the states it asks for.
- A stopped, failed, archived or deleted session always counts, so it can't leave the agent waiting.
- A session waiting on its own background work, or with queued messages, doesn't count as idle yet.
- It reads each session as it is now. If the condition already holds, the monitor fires right away.
- The timeout is 2 hours by default and 6 at most, and a timeout still wakes the agent. Monitors survive a daemon restart.

This monitor has no CLI command, and it can't watch sessions in another project.

##### Agents can watch a pull request without a terminal

An agent waiting on a pull request can now end its turn and let CompozyOS watch it. `compozy__monitor_create` with `kind: pull_request` wakes the agent when checks fail, when all checks finish, when a new review or unresolved thread arrives, and when the pull request merges or closes. The monitor stays armed across wakes and survives a restart. A plain `gh pr checks --watch` is now redirected to this monitor; one that names a pull request or another repository keeps the terminal path.

CompozyOS reads the forge with its own connection and runs no agent-supplied command or URL. Forge extensions can report reviews as an optional addition; one that doesn't still supports checks, and each wake says what it couldn't observe.

Limits:

- The timeout is 6 hours by default and 24 at most, with up to 64 wakes per monitor.
- Pull requests from forks aren't visible to the monitor.
- A rerun that passes on the same commit doesn't wake the agent, and neither do late checks or reviews on a replaced commit.
- A stays-on agent's main session doesn't rotate while one of these monitors is active.
- A Goal stays **Waiting** for as long as the monitor is active.
- Each open pull request refresh costs one extra GitHub GraphQL read.
- Review wakes were tested against a simulated forge and real GitHub reads, but not yet with a review arriving live on GitHub.

##### Approve-all agents read every project and profile

An agent session in `approve-all` mode can now read sessions in any project, in the User scope and in any profile, and can read the All projects lists, the way you can. Before, the same calls failed with `workspace_access_denied` or `agent_scope_denied`.

- **Read by session id:** `compozy__session_status`, `compozy__session_history` and the other session tools accept just a `session_id`. CompozyOS finds where that session lives and checks access there, so the agent no longer has to pass `workspace`.
- **Other profiles and All projects:** `profile`, `all_profiles` and `all_workspaces=true` reads follow the permission mode. `approve-reads` still asks you first, and `deny-all` still never leaves its own project.
- **Secrets are redacted:** when an agent reads something outside its own project or profile, credential-shaped text in what comes back, such as API keys, bearer tokens, `password=` values and long random tokens, is replaced with `[REDACTED]`. This works by recognizing patterns, so a secret without a recognizable shape can still get through. Reads inside the agent's own project and profile don't change.
- **Clearer denials:** a refused call now says which setting refused it and what would allow it. It never names the target, so a refused agent can't tell whether a session it asked about exists.

This widens what agents can see. `approve-all` is the built-in default, so unless you changed `[permissions] mode` or an agent's `permissions`, your agents can now read work in every profile. Set `approve-reads` to be asked first, or `deny-all` to keep a session inside its project. Changing work in another profile, or writing through an All projects route, still needs `workspace_access: unrestricted`, which only you can set.

##### Archive and delete can include child sessions

Archiving or deleting a parent used to leave its child sessions behind, still running if they were live, without telling you they existed. Now a session with children asks first: archive offers a checked "Also archive N child sessions", and delete offers an unchecked "Also delete N child sessions". A session with no children works as before, and archive stays one click.

From the terminal, use `--with-children` on `compozy session archive` and `compozy session remove`. `compozy__session_archive` accepts `with_children`, and HTTP gains the same option plus a route that counts a session's descendants. Leaving the option out keeps today's behavior.

Limits: unarchive brings back subagents only, so restore other children one at a time from the Archive view or with `compozy session unarchive <id>`. Delete isn't atomic: if it fails partway, the sessions it didn't reach stay and the error names the child.

##### Finished automation runs leave the session list

Sessions started by an automation job or trigger now leave the session list, the board and their counts once they finish cleanly, so a weekly job no longer adds a permanent row. Nothing is archived or deleted: each run stays reachable from the Automations run view and through search, including a search in a grouped list.

A run stays on the list if it failed, if it or a child session is waiting on you or still working, or if you prompted it yourself. A failed run stays until you archive it. Opening a finished run doesn't bring it back. Loop and Task sessions are unchanged, including automation runs handed to a Loop or a Task.

Migration notes: existing runs follow the same rule after upgrading, with no action needed. `compozy session list` and `compozy__session_list` keep returning every session. The API does too, unless a client passes the new `exclude_finished_automation=true` query parameter.

##### GitHub Copilot CLI works as a full provider

You can now sign in to GitHub Copilot CLI from CompozyOS, run sessions on it, and use CompozyOS native tools inside those sessions. Before this, `compozy provider auth login copilot` failed and Copilot sessions had no `compozy__*` tools. Setup steps are on the Providers docs page under "GitHub Copilot CLI".

- `compozy provider auth login copilot` runs `copilot login`. `compozy provider auth status copilot --remote` checks the sign-in with a session that sends no prompt.
- The model picker lists the models and reasoning levels Copilot offers your account. Plans with a single model, such as Copilot Free, offer no model choice: Copilot picks the model, and asking for a specific model or level stops the session before its first prompt instead of being ignored.
- Copilot ignores stdio MCP servers, so CompozyOS serves native tools to it on a local HTTP address protected by a per-session token. The address closes when the session's Copilot process exits. Any ACP provider can opt in with the new `session_mcp_transport = "http"` setting.
- `permissions.provider_full_access` now covers Copilot by turning on its `allow_all` option. Copilot's Autopilot mode and `allow_all = on` count as unrestricted, so both need `approve-all`.
- Starting a new session while its ACP provider is signed out now fails with a sign-in diagnostic naming `compozy provider auth login <provider>`, instead of a raw protocol error. This applies to every ACP provider: API and CLI errors now carry the `provider_not_authenticated` diagnostic. Resuming a stopped session still reports the provider's own error.
- The GitHub Copilot mark appears wherever CompozyOS shows the provider.

Copilot's ACP support is a public preview from GitHub, and CompozyOS was tested with Copilot CLI 1.0.105. Copilot can't run triage sessions, because it has no read-only mode. It can't take guidance during a running turn, so a message sent then cancels the turn unless you queue it. Copilot also reports no usage limits to CompozyOS.

##### See the worktrees, branches and pull requests behind each session

A session now has **Linked work**: the worktrees, branches and pull requests that belong to it, separate from where its agent runs. A worktree a session creates with `compozy__worktree_create` shows up by itself, with no bind and no restart. Pull requests from child sessions also appear on the parent, read-only, as "From {child}".

- The session header shows the most urgent item, and `+3 PRs` or `+3 more` opens the list. List rows show a count, Kanban cards show the lead pull request plus `+N`, and the command palette finds a session by pull request number or branch.
- Paste a link or search by number, title or branch to link more by hand, or run `compozy session link <session-id> --pr-number 142`. `compozy session unlink` removes a link.
- Agents can link what they made outside the worktree tool with `compozy__session_link` and `compozy__session_unlink`, on their own session only.
- Unlinking removes the association and nothing else, and it offers **Undo**. An automatic link you unlink doesn't come back.
- The pull request details tooltip is now a checklist of what stands between the pull request and merging.

Limits: a pull request from another project is refused, and one from a fork can't be linked yet. A branch holds one pull request, so linking a second replaces the first. Pull request search needs a connected forge; without one you can still link branches.

Migration notes: one board behavior changes. A session that has settled with an open pull request linked directly to it now sits in **In review** instead of **Done**. A running session stays in **Working**, and so does one with queued input, so a session never enters **In review** while it works. Pull requests from child sessions never move the parent. With no open linked work, the column is the same as before. Existing `branch` and `pull_request` fields keep their meaning, session payloads gain an optional `linked_work`, and forge extensions need no change: without pull request search they still link branches.

##### Stays-on agents run at their configured permission level

A stays-on agent runs at the `permissions` level on its own definition, whether you or an agent created the instance.

- **No level set:** it stays at `approve-reads`, or at `[permissions].mode` when that is narrower. The bundled `orchestrator` sets no level, so it stops for your approval when it writes.
- **`approve-all`:** it acts without prompts.
- **Sessions it starts:** they don't take its level. Each uses its own agent's `permissions`, then `[permissions].mode`, so a stays-on agent at `approve-reads` can start a session that runs at `approve-all`. A `permission_mode` above that session's level is refused.

Tool, skill, MCP server and path limits still apply, and check-ins stay read-only. The level is read each time a session starts, so a session that can write the agent's `AGENT.md` or the project's `.compozy/config.toml` can change it. Treat write access to those files as authority over the level.

### Migration

Stays-on agents are new in this release, so upgrading from v0.4.8 changes nothing here. If you already run them on a pre-release build, each one moves to its configured level the next time its main session starts. A stays-on agent with no level stays at `approve-reads`. One set to `approve-all` used to be held at `approve-reads` and acts without prompts after upgrading.

To keep one asking first, set `permissions: approve-reads` on it or remove the level. This covers only the agent's own actions, so set a level on the agents it starts as well.

##### The subagent panel shows who is working, on what, and with which model

Each row in the composer's subagent panel now shows the title, model and kind, the current action and the elapsed time, so a subagent no longer reads "general" and "Not reported". Click a row to slide the panel to that subagent: status, time, tools, tokens, what it was asked to do, its result and its tool activity. Back and Escape return to the row. A bar in the header shows one segment per subagent, filling as they finish, fail, work and queue.

Limits: the provider reports a model alias, so a row reads `Sonnet`, not its full version. Token and tool counts exist only for subagents the provider runs itself. Nothing reports a percent or a per-subagent cost, so neither is shown.

Migration notes: subagent payloads and `compozy__subagent_status` gain an optional `usage` field. On provider-run subagents, `role` now holds the subagent type instead of the fixed value `general`, so update any script that matches on it.

##### One View options menu, with Collapse all and Kanban lane folds

The separate Sort and Group by buttons are now one **View options** menu on the session list and the Kanban.

- **Collapse all** and **Expand all** fold every folder, project group and child list in one step, including rows that haven't loaded yet. New items follow the choice, and the choice survives a reload.
- Kanban project lanes now fold down to their name, count and live signal. A project folded in the list is folded on the board too.
- **Show empty projects** now applies to the Kanban as well as the list.
- **Hide child sessions** moved from the Kanban header into this menu.

Collapse all is one choice per window, so using it on the board also folds the list. On the board it shows only while there are swimlanes. While a search is active, folds don't hide results.

#### Fixes

##### Allowed permissions stay with their tool in the timeline

An allowed permission no longer splits a tool group into `Used 2 tools / Allowed… / Used 2 tools`. It now sits under its tool, visible when you expand the group. Rejected, expired, pending and unmatched permissions keep their own row, and tool counts don't change.

Approvals of the native `compozy__terminal_exec` tool still show on their own row, because nothing links them to their tool call yet. In sessions that mostly run terminal commands, most allowed permissions will look the same as before.

##### Automatic session titles drop label prefixes

Automatic titles no longer keep a label the model put in front of them, such as `Título sugerido: Localizar resenhas` or `Suggested title: Checkout retry race`. CompozyOS now asks for the title alone, in the conversation's language, and strips these labels in English, Portuguese, Spanish, French, German and Italian. Titles that really contain a colon, such as `Fix title: colon handling`, stay whole.

A label in another language can still pass through, and a title that truly opens with a listed phrase, such as `Session title:`, loses it. Titles already stored are not rewritten.

##### Removing a composer image no longer breaks another one

Removing an image from the composer could delete the stored file shared with another image, including a duplicate tile or one already sent, so Send then failed with `attachment not found`. Removing a tile now only removes the tile. Retention and session deletion clean up the stored file, so an abandoned upload can remain until retention runs.

A prompt refused for a missing attachment now reads as plain guidance instead of a raw error. Your text and files return to the composer, the file uploads again, and Send retries. The guidance stays with the draft until the message is accepted or you remove the attachment or discard the draft.

A full page reload still keeps only your draft text, not unsent files, so attach them again.

##### Goals wait for external work instead of spending their turns

A Goal could use up its turn limit while a monitor or subagent it started was still running. Now, while the session has a live monitor or subagent, the Goal shows **Waiting** and starts no new work, compaction or judge check. The turn that started the wait counts once, the wake consumes no Goal turn, and the Goal carries on when the wait ends. CompozyOS checks this itself, so an agent saying it is waiting isn't enough.

- A Goal waiting on a subagent survives a daemon restart, and picks up the result when the subagent finishes during the restart.
- A subagent run in wait mode hands its result straight to the caller, so it no longer holds a Goal in **Waiting**.
- **Pause** keeps working during a wait, and so do Stop and your own messages. Wall-clock budgets keep counting.
- A wait that lasts 25 hours pauses the Goal for your attention with `goal_wait_timeout`. If the wait can't be read, it pauses with `goal_wait_unavailable`. Check the monitor or subagent before you resume.
- A deliberate daemon stop pauses the Goal and stops its session. Resume the session first, then the Goal.

Migration notes: no action is needed, and existing Goals keep their progress. `waiting` is a new Goal status in HTTP, UDS, CLI output and the Goal tools, so update any client that switches on status.

##### MCP tools that arrive late reach running agents, and tool lists load faster

An agent could finish listing its MCP tools before a server's tools were discovered, then never see them. CompozyOS now tells connected providers when the list changes, so they can pick up and call tools that arrive late. This was tested directly against the Claude adapter, not through a full session.

Listing tools also no longer rediscovers every MCP server once per tool. With two slow local servers and 114 tools, the list went from 36 seconds to under a second. Other causes of a slow list may remain, since this fixes the one we reproduced.

##### Monitor wakes no longer fill the session queue

Monitors that fire during a long turn now merge into one queue entry that reads "N monitors fired" and carries every outcome in firing order. Before, each wake took its own row, and the composer could show `10 queued · full` while your own messages were still welcome. Only your messages count toward `full` and `queue_cap` now.

On a provider that accepts input mid-turn, the merged wake goes into the running turn after `session.messaging.queue_promote_after` (2 minutes by default), behind anything you already queued. Other providers get it at the next turn boundary, as one turn. `compozy__session_inputs_list` also no longer fails on these entries.

Limits:

- Past 64 KiB, a wake keeps its outcome and drops its output tail, with a note to read the terminal.
- Past 64 merged wakes, a new one waits and retries until the entry is delivered or its retry window ends.
- After a restart, a wake whose delivery was never confirmed goes back in the queue, so the agent can see the same outcome twice. This is deliberate: repeating an outcome is safe, and losing one is not.

Migration notes: no action is needed. Entries queued before the upgrade stay separate and deliver as before. `coalesced_wakes` is a new optional field on queue entries, and `input.pre_submit` hooks now also run for wakes delivered mid-turn.

##### Slow npm adapter installs no longer fail a session start

A first `npx -y <adapter>@latest` can take longer than the 30 seconds an adapter gets to start, so CompozyOS stopped the install before the adapter ever ran, and repeat starts could interrupt npm's own recovery. This also blocked Loop agent steps. The package now installs first, with its own five-minute limit, and the 30-second limit starts only after that. This covers built-in and custom providers.

Starts that need an install take turns, and the wait counts toward the five minutes. Versions stay unpinned, nothing in your npm cache is deleted, and a failure now tells a temporary npm problem apart from a package that can't be resolved, in both the error and `compozy doctor`.

##### Extension upgrades recover from retired and half-installed packages

Upgrading no longer leaves the old `dev-cycle` extension in the way. CompozyOS moves its installs and profile enablement to `spec-cycle` in one step and keeps the old package files. If the old extension holds inputs, credentials, development links, profile markers or conflicting enablement that can't move safely, it stays registered and `compozy doctor` names its replacement and the exact removal command. Review that state before you remove it.

One broken extension also no longer fails another extension's install, update, enablement, removal or rollback. Its failure stays visible in status and in `compozy doctor`. A package left with no install can now be cleared with `compozy extension remove <name> --global`.

Migration notes: the `dev-cycle` to `spec-cycle` move runs automatically on upgrade and is scheduled for removal in v0.6.0, after the v0.5 migration window. Retired `ext__dev_cycle__*` tools have no alias and no tool replacement, because `spec-cycle` no longer ships tools in this release: see "Spec delivery moves from Loops to agents" for what to use instead. `cy-create-prd` and `cy-create-techspec` are now `cy-create-spec`, so review its inputs when you update scripts and Loop definitions.

##### Switching sessions stays responsive while others stream

With several sessions streaming, switching to another one could leave the previous conversation on screen, and still interactive, for more than five seconds. The new session or its loading state now appears as soon as you select it.

The dashboard, Loop, task and worktree views also make far fewer repeat reads when many events arrive at once, the daemon does less repeated work on session, scheduler and worktree lists, and a closed About dialog no longer polls full status. These results come from controlled tests, so your gain depends on your load.

#### Highlights

##### Stays-on agents keep watch over your projects

Agents now come in two kinds. **On demand** agents work as before: they run when something starts them. **Stays on** agents hold a place in a project, or in Global, with one main session you come back to, regular check-ins, and notes that carry over between sessions.

- **Create one:** the new create screen covers both kinds, or run `compozy agent create triage --kind stays_on --prompt "Keep issues moving."` followed by `compozy agent instance create triage --workspace <project>`. The kind is fixed at creation; duplicate an agent to choose the other one. Each profile can run 10 active instances by default.
- **See them:** avatars sit above each project's sessions and in its Kanban lane header, with anyone who needs you first.
- **Check-ins:** on a schedule, the agent reviews its project in a hidden read-only session and reports only when it has something to say. Quiet checks back off, and repeated failures pause the agent (10 by default). Reports land in the main session without starting a turn, and after an absence a **While you were away** card sums up what happened. Check-ins are on for a new instance; `compozy agent instance checks` turns them off.
- **Approvals:** requests from the agent and the sessions it starts reach you through its main session. An unanswered request is denied after 12 hours by default, and the agent records a report.
- **Memory:** edit an agent's notes in the app or with `compozy agent memory`. Agents write notes on purpose; nothing is extracted automatically. Up to 200 lines or 24 KiB load at startup. User and extension agents share notes across projects, and agents defined in a repository keep their own.
- **Identity:** a display name, description and avatar now follow an agent across sessions, tasks and automations, and the agent profile collects where it works, what it does, its Check-ins, Memory and activity.

Check-ins need a provider with a confirmed read-only mode. Claude, Codex and OpenCode qualify; other providers fail those checks with `triage_unsupported`. The agent tools for instances and memory, such as `compozy__agent_instance_create` and `compozy__agent_memory_add`, are experimental for this release and may change without the usual notice.

Migration notes: nothing changes for existing agents. They stay On demand, no `AGENT.md` is rewritten, and installing an extension never starts a stays-on agent. New settings live under `[agents.stays_on]` and `[agents.heartbeat]`, with working defaults. Older versions may reject the new `AGENT.md` keys, so keep a backup before downgrading.

## 0.4.8 - 2026-11-01

### Bug Fixes

- Keep the accent ink on the menubar Updating pill (#1089)
- Restore the usage tray before the bell and hold its slot while loading (#1090)
- Ellipsize long session titles in session message card headers (#1099)
- Remove the Skip to content link that appeared on window reactivation (#1096)
- Hide empty projects in project grouping behind a Show empty projects option (#1091)
- Authorize explicit cross-workspace session replies (#1103)
- Let operators resume and take over spawned sessions after the parent stops (#1098)
- Decode each Later reason character reference once and only when complete
- Keep shelf card edges inside the horizontal scroller (#1116)
- Move the message trail right and add sidebar tonal hierarchy (#1115)
- Clear the window when its open session is archived (#1111)
- Store only clean text in generated session titles (#1125)
- Make the spawn reaper's parent-stop reap atomic and conditional (#1123)
- Let sessions call OAuth-authenticated extension MCP servers (#1117)
- Address merge train review findings on stops, titles, and MCP lookup
- Close shell-wait guard gaps found in review
- Satisfy gocritic in the PowerShell option check
- Keep the explicit owner wording in the MCP ambiguity error
- Format the MCP ambiguity error for golines
- Keep exit-plan reads from emitting worktree.status_refreshed (#1121)

### Features

- Rich-text reason and date picker in the Move to Later dialog (#1093)
- Group session processes into trees (#1106)
- Hide child sessions on the Kanban board by default (#1124)
- Apply runtime selection changes to the live session immediately (#1108)
- Redirect foreground shell waits to CompozyOS monitors (#1127)
- Make session monitors durable across daemon restarts and trace their lifecycle (#1129)

### Refactoring

- Replace the in-repo prototype gallery with @compozy/proto (#1092)
- Align @compozy/ui with the bnui contract (#1126)

### Testing

- Cover captured-only AfterStop cleanup and lead-only Stop all for process trees

## 0.4.7 - 2026-10-31

### Bug Fixes

- Preserve clarify line breaks and make the clarification dock collapsible (#1062)
- Support User-owned session lifecycle actions in global scope (#1060)
- Verify Claude hosted MCP readiness before startup succeeds (#1066)
- Show only the title for open terminals, without a running mark (#1073)
- Flag child sessions that stop without delivering and make end-turn guidance conditional (#1069)
- Keep forked and continued sessions visible when hiding child sessions (#1077)
- Reuse stdio MCP server processes across discovery, status, and calls (#1081)
- Resolve MCP OAuth credentials from the declaring scope (#1080)
- Keep failed worktree creations visible and roll them back in order (#1082)
- Verify interrupted worktree checkouts under the lock and by branch (#1084)
- Report the Later count on the active facets read
- Keep the facets Later total optional in the API contract
- Label inherited MCP auth in listings and hold unreadable workspaces

### Features

- Archive a project from the top-bar project submenu (#1061)
- Zoom the Mermaid fullscreen viewer with Ctrl/Cmd+wheel, pinch, and keys at the cursor (#1059)
- Replace the folder browser with a directory picker input and centre onboarding step numbers (#1064)
- Open terminals in a project's managed worktree (#1067)
- Add a Waiting board column and let every Kanban column collapse (#1079)
- Park sessions in Later with an optional reason and return date (#1083)

## 0.4.6 - 2026-10-30

### Bug Fixes

- Make automatic titles lightweight and retry failed attempts
- Return conversation clear after durable reset
- Default terminals to the account login shell and open them from a launcher
- Refresh session rows in every catalog scope when the runtime changes
- Stop requests for unavailable desktop resources (#1038)
- Hand focus between project overlays before hiding their origins (#1040)
- Preserve Claude usage access across Keychain token refreshes (#1041)
- Restore project menubar focus after context menu dismissal
- Keep focused skip link out of desktop layout
- Report the enforced spawn budget to agents (#1048)
- Share terminal approvals through attention summary
- Restore workspace focus after worktree creation dialog

### Features

- Simplify the Fork session dialog and always fork into this window
- Selection-aware session context menu and project context menus (#1032)
- Group projects into folders and add session totals footer (#1047)
- Add proto.tsx prototype gallery that renders real web components (#1043)
- Open Kanban sessions in a resizable drawer over the board (#1055)
- Multi-select sessions and run bulk actions on the Kanban board (#1054)
- Inspect and control processes started by sessions (#1049)

### Testing

- Cover remaining merge train review invariants
- Synchronize detached harness session fixtures
- Record additive attention approval schema contract

## 0.4.5 - 2026-10-29

### Bug Fixes

- Preserve default profile customization across daemon startup (#1001)
- Preserve Linux update executor identity across clock drift (#1004)
- Restore terminal lifecycle under All projects and User (#1009)
- Preserve archive recovery and pause rejected automation replacements
- Unblock missing-root recovery and restore native input contracts
- Preserve picker focus and synchronize subagent navigation checks
- Preserve native tool domain errors and sanitized causes (#1014)
- Recover detached message edits after lost acknowledgment (#1018)
- Isolate missing workspace state from healthy operations (#1016)
- Register SQLite Unicode search before opening connections
- Make Goal controls interrupt work and clarify session lifecycle (#1019)
- Expose CompozyOS native tools in Pi sessions (#1021)

### Features

- Change existing session worktrees in place (#1002)
- Inspect subagents in a resizable activity drawer (#1007)
- Archive projects and add safe removal actions (#1010)

### Performance Improvements

- Accelerate session and app data loading (#1006)

### Testing

- Wait for subagent branch expansion before paging
- Account for uptime in denied settings mutation checks

## 0.4.4 - 2026-10-28

### Bug Fixes

- Preserve active sessions through desktop updates (#991)
- Show truthful subagent status across lists, cards and native tasks (#993)
- Regenerate native tool catalog and approve subagent child_activity schema
- Missing agy acp reasoning effort levels (#858)
- Address subagent status review follow-ups
- Keep the frozen elapsed time on idle subagents
- Split subagent hover preview and expect urgency-ordered branch pages
- Edit session messages directly in the composer (#996)
- Retry refused message edits and keep detached unconfirmed sends

### Features

- Sync Interface and Code fonts across clients (#990)

## 0.4.3 - 2026-10-27

### Bug Fixes

- Prevent duplicate release PRs after release merges (#986)
- Recognize Linux daemon lock owners despite clock drift (#987)
- Bound Mermaid diagrams and add fullscreen zoom (#988)

### Performance Improvements

- Reduce session latency, retained memory and shell render work (#983)

## 0.4.2 - 2026-10-26

### Bug Fixes

- Stop background Claude usage reads from prompting for the keychain (#977)

### Testing

- Allow cold PowerShell startup when reading the Windows grandchild PID

## 0.4.1 - 2026-10-25

### Bug Fixes

- Strip AppImage runtime state from child environments (#954)
- Align catalog artifacts and expose version incompatibility (#957)
- Recover Claude usage from renewed native credentials (#959)
- Load and cancel session monitors outside the default profile (#961)
- Recover half-removed worktree checkouts instead of failing with 500 (#964)
- Prevent contention in GlobalDB integration upgrades (#965)
- Keep authored line breaks and full composer links in user messages (#968)
- Preserve worktree locks and finish interrupted removal cleanup
- Preserve authored inline code spacing
- Honor app reduced motion in content transitions
- Preserve skeleton delay under reduced motion
- Synchronize ACP connection startup and align tray coverage
- Keep the settings loading rail out of the live navigation

### Features

- Add Kanban card actions, board group/sort, and per-window session order (#963)
- Add content-shaped loading states and React view transitions (#972)
- Finish loading states, window fallbacks and transition fixes for every route (#973)

### Performance Improvements

- Suspend closed palette topology updates (#960)
- Avoid palette context updates for window geometry (#971)
- Defer loading skeletons until restored frames commit

### Testing

- Align catalog fixtures and runtime readiness with current contracts
- Synchronize clarify keepalive log capture
- Retry transient desktop fixture removal failures
- Seed Kanban data before opening the browser page

## 0.4.0 - 2026-10-24

### Bug Fixes

- Validate bridge desktop smoke against its update feed
- Refresh Claude usage after account ownership changes (#931)
- Revalidate configuration before reusing workspace snapshots
- Format workspace snapshot configuration checks
- Align stable release dry-run verification
- Resume interrupted sessions after daemon restart (#942)
- Isolate window surfaces from geometry updates (#943)
- Recover stops after runtime identity release (#948)
- Reveal native subagent work from session lists (#949)
- Keep async subagent navigation outside compiler scope
- Preserve recovered stop reasons and native reveal state
- Preserve current branch and destination in session board actions
- Reveal settled native subagents by card identity
- Wrap current branch queries within source line limits
- Stop waiting on completed subagent delegations
- Collect delivery facts independently of PR display cache

### Features

- Add managed worktree teardown commands (#937)
- Link session pull requests, add waiting badge and sessions Kanban (#944)

### Performance Improvements

- Share hosted MCP projections across equivalent peers (#938)
- Make session inventory reads cheap and side-effect free (#940)
- Reduce idle skill watcher and workspace resolver scans (#936)

### Refactoring

- Group session catalog exports with board surface

### Testing

- Align restart fixtures with explicit restart admission
- Include pull request columns in CLI round-trip contract
- Attach successor approval to the named wake turn
- Separate migration verification from upgrade budget

### Release Notes

#### Highlights

##### CompozyOS is stable

v0.4.0 is the first stable CompozyOS release. It closes the v0.3.0 beta line, and every release from here on is a stable release.

- **On a v0.3.0 beta:** update in place from the app's Update action or with `compozy update`. No reinstall, and your sessions, projects, and settings carry over.
- **On v0.2:** v0.2 has reached end of life and receives no further releases. Homebrew and npm now install v0.4.0, which starts from clean runtime state. Read the [migration guide](https://compozy.com/docs/migration) before upgrading.
- **New installs:** `brew install compozy/compozy/compozy`, `npm install -g @compozy/cli`, or the verified installer at `curl -fsSL https://compozy.com/install.sh | sh`.

These notes cover the changes since the last beta. The full beta trail lives in the [GitHub releases](https://github.com/compozy/compozy/releases) and the [changelog](https://compozy.com/changelog).

## 0.3.0 - 2026-10-22

### ♻️ Refactoring

- State management with xstate-store (#268)
- Site improvements (#277)
- Replace bundles with extension kits (#291)
- Modernize Go runtime packages (#293)
- Use geist instead of inter (#334)
- Add global workspace toggle (#368)
- Unify PRD and TechSpec into a single spec pipeline (#397)
- Quiet the Context rail and sessions row for everyday use (#649)
- Retire Network, managed Sandbox, and Bridges (#681)
- Complete Modern Go adoption across module boundaries (#706)
- Remove 12 long-tail builtin providers behind a one-release compatibility shim (#778)

### ⚡ Performance Improvements

- Delegate full gates to pull request CI (#476)
- Reduce test and development feedback overhead (#556)
- Avoid rendering unchanged windows during layout updates
- Reduce SQLite contention and query costs (#723)
- Reduce desktop startup and live transcript overhead (#784)
- Cut daemon idle cost and keep the host responsive under agent load (#797)
- Reduce concurrent session query and projection overhead (#795)

### 🎉 Features

- Introducing CompozyOS beta
- **BREAKING:** introducing CompozyOS beta
- Add provider-neutral ACP speed control (#267)
- Support grouped skill directories (#270)
- Session transcript calm-surface redesign (#271)
- Add permission-aware cross-workspace access (#275)
- Extensions improvements (#278)
- Select and switch runtime per session prompt (#283)
- Adopt MCP 2026 and expand the curated catalog (#284)
- **BREAKING:** MCP catalogs now require manifest_version 2 and public
  MCP transport no longer accepts SSE.
- Add support for window tabs (#287)
- Adopt feedback semantics for durable Loops (#290)
- Add complete Loop node lifecycle (#305)
- Rewind sessions to durable conversation checkpoints (#310)
- Add session-aware slash skill commands (#311)
- Add reversible session archiving and list actions (#309)
- Replace software-delivery with implement-tasks (#325)
- **BREAKING:** remove software-delivery; use implement-tasks without gate inputs.
- Parent and child sessions (#327)
- Add secure remote gateway access (#331)
- Ship CompozyOS desktop app (#336)
- Add first-class worktree support (#388)
- Close the loops UI visual-contract parity gap (#406)
- Unify zero-inventory empty states for jobs, triggers, and tasks (#409)
- Redesign workspaces overview as command-tab switcher (#410)
- Redesign trigger detail into the When/If/Then rule page (#411)
- Session attachments — paste, drop, and picker to multimodal agents end to end (#412)
- Add Agent Plugins ingestion to extensions (#419)
- Add session attention and orchestration parity (#422)
- Replace Tauri with Electron and unify updates (#424)
- Add Batuta to marketplace (#432)
- Complete Loop graph engineering and typed inputs (#427)
- Simplify the interface for everyday users (#440)
- Deliver the command palette operating surface (#441)
- Make loop runs legible and task lists calm (#452)
- Complete production demo seed (#453)
- Profiles — the who-is-working dimension (#457)
- Combine settings update actions (#461)
- Replace environment path with worktree icon (#463)
- Group session tool calls (#466)
- Restore agent-manageable Goal orchestration (#470)
- Support conjunctive runtime routing (#475)
- Expand profile identity customization (#484)
- Absorb ecosystem skill folders and expose skills back (#488)
- Merge spec-cycle task delivery loops (#491)
- Rebuild ACP runtime catalogs (#498)
- Support child Loop config overrides (#494)
- Integrated terminal — runtime-owned shells for people and agents (#490)
- Live steer and truthful stop for sessions (#555)
- Manageable session queue, truthful live view, and legible transcripts (#557)
- Add herdr bridge to community catalog (#560)
- Bulk actions for the sessions list (#619)
- Add built-in open-design extension (#625)
- Show session context and confirmed prompt deliveries (#635)
- Unify marketplace catalog and plugin sources (#636)
- Add release video workspace and production skill
- Give rendered Markdown a readable visual hierarchy (#660)
- Add safe bulk worktree cleanup to workspace lists (#656)
- Add provider-neutral ACP full-access preference (#674)
- Calmer, plain-language web UI for everyday users (#683)
- Continue and fork sessions across agents, with fallback account routes (#684)
- Shell rail v2 with light and dark themes (#687)
- Show nested child runs and a distinct awaiting-child state (#708)
- Merge Jobs and Triggers into one Automations area (#709)
- Remove the memory feature family and adopt agent-native session compaction (#710)
- Delegate tasks to subagents with live cards, durable wakes, and operator surfaces (#711)
- Render session tool calls as verb + object rows on a work rail (#716)
- Attribute agent session messages, wake senders with replies, and isolate subagents in worktrees (#722)
- One-click CompozyOS update from the menubar (#733)
- Show the runtime provider mark on session rows (#758)
- Add session folders, manual drag order, and session context menus (#786)
- Give session titles their own line in the session list (#790)
- Adding antigravity acp (#773)
- Add app SDKs and browser application tokens (#794)
- Pick the default provider, model and reasoning from Settings → General
- Clean up missing worktrees in bulk and opt into local branch deletion
- Open new sessions in the existing Session window
- Add @ mentions for files, folders, and sessions in the session composer (#842)
- Make the session window's sessions rail resizable (#845)
- Add opencode v2 support (#811)
- Let open-design prototype with the project's component library (#867)
- Wake agent sessions natively when background work finishes (#860)
- Mark workspaces with working sessions in the switchers (#869)
- Remove discovered worktrees without adopting them (#870)
- Show and cancel session monitors from the session list and composer (#876)
- Add provider usage tray, usage app and daemon collectors (#871)
- Move selected sessions into folders by drag or menu (#881)
- Bind spawned workers to worktrees and inherit parent permissions (#885)
- Work across all projects and in a User scope without a project (#887)

### 🐛 Bug Fixes

- Align release and process test contracts
- Workspace params (#266)
- Onboarding workspace selection
- Restore Windows daemon support (PR #163 regression + new fixes) (#274)
- Untested cases from qa (#276)
- Remaining untested qa (#279)
- Durable session messaging (#288)
- Assistant-ui version
- Changelog generation (#292)
- Make busy-session inputs durable (#304)
- Restore durable ACP session continuity (#307)
- Preserve Loop template manifests across hydration (#317)
- Preserve ACP stream disconnect recovery (#319)
- Discover Cursor model catalog before sessions (#320)
- Keep managed skill loading on native seam (#323)
- Loop run bugs (#324)
- Enforce bundled agent and Loop ownership (#326)
- Session native tools and extensions details (#330)
- Judge gate on goal loops
- Restore minimum-age dependencies
- Stabilize release runtime startup
- Start absent SSH daemon
- Make loop goals converge reliably (#335)
- Desktop issues
- Ship the full desktop icon set required by Windows tauri-build
- Pause the Windows desktop lane and ship macOS + Linux only
- Adjust project copy
- Publish staged GitHub release drafts
- Repair release integration contracts
- Harden desktop startup and diagnostics (#343)
- Resolve desktop and session usability issues (#351)
- Recover canceled Loop coordinators on restart (#353)
- Resolve extension-published agents for session command catalogs (#350)
- Eliminate persistent performance bottlenecks (#354)
- Reject automation trigger events no producer emits (#358)
- Read loop watch-events with a stream-global cursor (#356)
- Prevent session timeline render loops (#361)
- Align update artifacts with release policy (#363)
- Canonicalize desktop runtime manifests (#364)
- Validate desktop bundles before release
- Make release publication atomic
- Defer npm publication safely
- Harden release recovery
- Isolate release publishers
- Make release artifacts reproducible
- Align release automation contracts
- Reject invalid agent names (#367)
- Resolve extension agent skills and missing config paths (#372)
- Include extension tools in hosted MCP bootstrap (#373)
- Use conventional release triggers
- Resolve main lint regressions
- Parse nanosecond daemon timestamps
- Validate release workflow tools by type
- Preserve awaited child loop state (#391)
- Use exact Cursor ACP model values (#392)
- Preserve dead session history (#393)
- Validate desktop release smoke wiring
- Validate public CLI version command
- Clarify missing config path errors (#401)
- Guard autonomous memory extractor writes (#396)
- Write $ENV interpolation in pi runtime models.json apiKey (#404)
- Authorize daemon-owned loop effects (#407)
- Retain terminal loop effect results in web (#408)
- Bound desktop runtime health checks (#414)
- Preserve loop goal session lineage (#420)
- Support resource-only extension development (#423)
- Daemon path resolution
- Website font style
- Harden Loop runtime and graph execution (#438)
- Use candidate version in release dry-run
- Coalesce redundant ACP tool updates (#442)
- Resolve agent runtime recovery regressions (#447)
- Preserve run-agent session lifecycle (#446)
- Use migration timeout for tail replay
- Recover ACP sessions after provider disconnects (#454)
- Close profiles regressions after PR 457 (#459)
- Make sessions dock contextual (#468)
- Classify settled spawned TTL cleanup correctly (#469)G
- Restore reliable session message copy (#460)
- Add runtime provider tooltips (#464)
- Close windows when sessions are deleted (#465)
- Route command palette entities directly (#467)
- Preserve worktree binding during reconciliation (#456)
- Harden profile-scoped runtime and web flows (#481)
- Keep inactive loop routes out of task projection (#483)
- Harden Loop recovery and expose execution truth (#492)
- Publish npm packages with trusted identity (#495)
- Accept setup-node OIDC placeholder
- Preserve web assets in release module
- Remove npm token from release recovery
- Allow extension loops to call owned tools (#503)
- Honor selected orchestrated implementer (#502)
- Externalize oversized loop action results (#510)
- Force loop cancellation (#509)
- Enable prompts for managed sessions (#517)
- Preserve sparse fan-out roster rows (#518)
- Scope Loop extension tools to worktrees (#519)
- Preserve profile-scoped extension Agent skills (#516)
- Prevent session cancel and clear races (#523)
- Restore beta 22 release notes and changelog
- Package site search catalogs
- Zoom windows without covering others and keep the layout stream alive (#525)
- Honor route skips within a planning pass (#529)
- Scope lifecycle loop actions (#531)
- Preserve profile scope in loop implementers (#527)
- Stabilize base CI integration (#536)
- Fence terminal generation outputs and stabilize e2e contracts (#547)
- Preserve ACP options in loop-managed session profiles (#542)
- Preserve unresolved review findings (#543)
- Mark skipped terminal loop nodes as not taken (#544)
- Sanitize runtime settings when changing providers (#546)
- Ship the AppImage with the static runtime (#548)
- Preserve provider failures before output validation (#545)
- Terminal operating system (#552)
- Stabilize loop recovery and simplify run inspection (#554)
- Keep pending activity visible and repair CI synchronization
- Prevent stale terminal catalog reads from replacing live state
- Honor profile in operator tool catalog (#551)
- Tolerate unmatched terms in memory recall (#569)
- Scope startup tool guidance to session roles (#572)
- Preserve migrated state during update recovery (#574)
- Unify spec-cycle task completion semantics (#575)
- Preserve provider command routing for spawned sessions (#576)
- Preserve prior session profiles and bound metadata warnings (#578)
- Preserve native model reasoning capabilities (#579)
- Make memory opt-in and preserve pressure compaction (#580)
- Resolve post-merge shutdown and verification failures
- Keep loop authoring editable during validation
- Preserve profile scope across loop run views (#583)
- Preserve profile ownership in loop and memory runtimes (#582)
- Reserve shutdown time for required cleanup (#581)
- Retain the Global desktop across workspace catalog changes
- Repair release integration contracts and lifecycle cleanup
- Stabilize release lifecycle races and integration fixtures
- Respect startup and shutdown lifecycle contexts
- Preserve interrupted session identity and bridge progress bursts
- Preserve cancellation delivery and pending activity deadlines
- Load knowledge memories without enabling automation (#590)
- Allow resizing maximized internal windows (#589)
- Delegate bounded native tools to orchestrated workers (#591)
- Emit profile scope from memory extraction (#592)
- Recover layout settings saves and expose apply results (#596)
- Confirm terminal termination when closing windows (#597)
- Reduce excessive CPU in connected sessions (#599)
- Bound session summaries and preserve complete details (#600)
- Reconcile Goal context and session lifecycle (#601)
- Preserve terminal creation scope and accessible title actions
- Preserve exact window titles and wait for reasoning controls
- Deduplicate delivered skill context and gate input-only roles (#607)
- Verify private Tailscale endpoints through tsnet (#608)
- Report effective dream role in memory health (#609)
- Group reasoning and tools in session work segments (#610)
- Add durable notification acknowledgement (#611)
- Complete notification review and main integration repairs
- Keep unread title counts current in background tabs
- Resolve notification integration Go lint findings
- Preserve escalation acknowledgement across neutral events
- Restore finished notifications and timeline disclosure behavior
- Synchronize attention delivery and durable loop state
- Recover stopped session deletion and queue polling (#614)
- Hide internal Goal prompts from the operator queue (#618)
- Recover owned task work after supervised silence stops (#621)
- Repair agent runtime and Goal workflows found in QA (#624)
- Recognize structured Claude authentication status (#632)
- Discover and bind models in provider overlays (#633)
- Preserve zsh startup directory semantics (#630)
- Preserve nested raw terminal input visibility (#631)
- Preserve unicode terminal prompt redraws (#634)
- Complete overlay admission and integration checks
- Render cursor edits in terminal quote lines
- Resolve terminal cwd from the bound session worktree (#641)
- Repair release integration and catalog compatibility
- Stabilize release integration under concurrent startup
- Goal delete session
- Reopen private Gateway files safely on Windows (#646)
- Report checkpoint role availability from compaction configuration (#645)
- Recover marketplace sources and render catalog logos under CSP (#648)
- Repair marketplace CI fixtures and evidence capture
- Cancel unbound goals before deleting sessions (#647)
- Preserve profile scope when deleting sessions (#652)
- Discover Claude reasoning options for each exact model (#658)
- Bootstrap pinned Bun with npm for Vercel builds
- Honor workspace default in session launcher (#668)
- Fixing clarify tool call timeout with keep-alive pings (#661)
- Preserve declared awaited run-loop terminal output (#672)
- Preserve whitespace in session transcripts (#670)
- Wake creator on child turn completion (#685)
- Resolve reported runtime, catalog and delivery failures (#686)
- Restore release nightly coverage and package extension runtimes
- Preserve loop results under SQLite contention (#690)
- Repair profile isolation, runtime recovery, and loop results (#691)
- Skills nested
- Upgrade dependencies and repair CI compatibility (#692)
- Repair release validation and desktop restoration (#694)
- Preserve profile recovery guidance and repair browser validation (#695)
- Preserve SQLite commit outcomes during cancellation (#696)
- Preserve profile entry across query reconciliation (#697)
- Apply the chosen runtime to continued sessions and keep continue/fork top-level (#702)
- Keep live model catalogs current without releases (#701)
- Classify teardown survivors by final process liveness (#713)
- Fold transcript entries incrementally instead of replaying each turn (#715)
- Resolve Windows runtime failures and desktop context menus (#725)
- Reduce session startup and attention overhead (#731)
- Bound scheduled claim attempts before contention backoff
- Allocate run-agent binding epochs for reviewed actions (#735)
- Keep session image previews dismissible (#744)
- Preserve goal origin identity across provider changes (#749)
- Preserve HTTPS origins through the Tailscale gateway (#743)
- Make bundled spec-cycle workflows self-contained (#741)
- Preserve daemon-owned Loop action claims (#748)
- Keep session lists stable with created and name sorts (#752)
- Attribute model catalog refresh failures to providers (#745)
- Drain queued inputs before Goal continuation (#755)
- Render session message cards for Claude and Codex sends (#757)
- Hide daemon-owned verification sessions (#759)
- Keep runtime models clickable during provider failures (#768)
- Tolerate transient session stream reconnects (#769)
- Stop false Goal attention and cut daemon idle cost (#770)
- Keep background subagents running and show them in the composer (#771)
- Keep terminals running across workspace switches (#776)
- Cut idle UI polling and attention bell CPU cost (#779)
- Preserve profile ownership in session actions and subagent rosters (#785)
- Stop warning about and stopping idle sessions (#787)
- Make the idle-session setting drive idle runtime release (#789)
- Deliver session messages to busy targets with operator-first queue, batching, hop model and explicit replies (#780)
- Address merge-train leftovers from #787 #789 #780 #786 (#793)
- Retry pending steer settlement without locking status
- Advance role fallback when the provider refuses an accepted turn (#662)
- Inherit the caller profile for agent-created sessions and keep one global profile selection (#796)
- Align catalog stream test with global profile selection
- Resolve merged PR review and CI regressions
- Align codegen boundaries and runtime test contracts
- Repair release validation and session shutdown races
- Prevent WAL maintenance from blocking shutdown history
- Group completed tool calls during live Goal turns (#807)
- Auto-enqueue dependents after direct run completion (#806)
- Keep transcripts loadable when a tool call crosses a Goal turn (#803)
- Discover releases promptly and expose manual update checks (#808)
- Allow steering active sessions with attachments (#809)
- Reject blank queued-input promotion as a client error
- Retire session windows after list deletion
- Keep first prompts with skills admitted across registry reloads and never drop a refused draft
- Keep hosted tool results schema-valid when they exceed the result budget
- Retain desktop shutdown teardown diagnostics
- Move runtime selector model list issues behind a header warning mark
- Standardize composer toolbar controls and drop the steer/queue badges
- Recover WAL owner checks and storage-failed session stops
- Allow safe cleanup of worktrees with gone upstreams
- Stabilize transcript file summaries during subagent work
- Retain session runtime metadata and aggregate profile folders
- Close the product renderer after shell cleanup
- Scope the session sidebar state to each session window (#844)
- Fit both bundled tool manuals in the skills-disabled startup budget (#849)
- Resolve review findings left open on #842, #844, and #845 (#848)
- Keep the routing coordinator within the 500-line source limit (#850)
- Render Mermaid, enable desktop notifications, and declare session env (#855)
- Stabilize live transcripts and model-aware delegation (#856)
- Avoid interpreting request identities as provider failures (#857)
- Recover releases from Intel Apple networking failures
- Confirm bulk worktree removal and recognize landed patches (#863)
- Bring runtime and attachments to the empty desktop composer (#865)
- Report actionable errors for invalid workspace config (#866)
- Consume first-message handoffs when attachments are removed
- Preserve session message formatting and editing (#874)
- Stop stale session and terminal retry loops (#875)
- Replace newer desktop runtimes that reject the app (#878)
- Keep terminal journal databases outside project directories (#877)
- Scope session monitor routes to the profile and never drop monitor refreshes
- Re-read open monitor lists when the session catalog stream reconnects
- Keep stale provider usage resolutions from restoring disabled targets
- Prevent session deletion from deadlocking the daemon (#880)
- Allow session derivation after root deletion (#884)
- Stage desktop updates before installer shutdown (#886)
- Default new automations to the host time zone with a searchable picker (#889)
- Render supervised terminal calls as compact tool rows (#893)
- Keep skill_view source a string and expose command_source for command_id reads (#892)
- Address terminal and session release review findings

### 🔧 Miscellaneous Tasks

- _(mise)_ Pin Go 1.26.6 (#433)

### 🧪 Testing

- Guard ACP initialize protocol version (#318)
- Align nightly runtime fixtures
- Align dead session recovery coverage
- Preserve loop claim tokens in daemon fixtures (#418)
- Fix cases failing
- Fix failing tests
- Stabilize Windows PTY read readiness
- Drain Windows PTY startup output
- Enable memory for the knowledge browser journey
- Keep noncooperative ACP prompts alive after cancellation
- Select the desktop workspace before awaiting its client
- Wait for connected idle Skills prompt readiness
- Supply valid origins in acknowledgement fixtures
- Label unscoped attention story notifications
- Retain explicit global scope in attention fixtures
- Create escalation fixtures through their lifecycle
- Project escalation events in acknowledgement fixtures
- Persist canonical escalation status in overview fixtures
- Await completed automation run before daemon cleanup
- Wait for terminal command output before detaching
- Refresh credential assertions context after migration
- Cover inactive loop routes with automatic approval (#703)
- Cut duplicated and misplaced tests across the pyramid (#707)
- Wait for subagent card navigation readiness (#753)
- Control popover exit animation time
- Synchronize migration and startup verification
- Apply formatter to blank promotion regression
- Assert bounded caller reads in native policy regression
- Assert specific workspacedb validation errors
- Assert desktop runtime retry before and after replacement
- Align UDS integration runtime with user scope
- Align HTTP integration runtime with user scope

### Release Notes

#### Breaking Changes

##### Extension kits replace Bundles

The Bundle surface is gone. Extensions are now the single packaging unit, and installing one is inert: it publishes no tools and no resources until you explicitly enable it. Before enabling, you can preview exactly what an extension would publish and inspect what it is publishing right now, and any extension that declares network access must have its network requirement digest confirmed by a person. (#291)

- `compozy extension preview <name>` shows what enabling would publish without changing state; `compozy extension inventory <name>` shows the live published inventory. Agents get the same reads through `compozy__extensions_preview` and `compozy__extensions_inventory`.
- `compozy extension enable|disable|update|install` accept `--confirm-network-requirement <digest>`, so a network-declaring extension cannot start publishing without an explicit confirmation recorded on the install.
- `compozy extension secrets set|bind|list|unset` manages write-only environment bindings that are scoped per workspace and stored as secret references, never as values.
- Marketplace kinds are now exactly `extension`, `mcp`, and `skill`.

Migration notes: the whole `compozy bundle` command group is removed (`catalog`, `preview`, `activate`, `list`, `get`, `deactivate`, `network-settings`), along with the `compozy__bundles_*` native tools, the `compozy__bundles` toolset, the `bundle` marketplace kind, and the Bundle API surfaces. There is no alias — rebuild bundle-shaped setups as extensions and enable them explicitly.

##### Runtime hardening and secret-safe provider login

A broad modernization pass across the Go runtime tightened lifecycle and cleanup ownership, ID allocation, task settlement, filesystem confinement, and streaming framing. Most of it is invisible, but it lands several deliberate cuts that change what operators, scripts, and agents see. (#293)

- `providers.<id>.auth_login_command` is now write-only. You can still set it through `config.toml`, `compozy config set`, or `compozy__config_set`, but no read surface returns it. `compozy config show|list|get|diff`, provider status, doctor, Settings, HTTP, and UDS return a safe `login` descriptor instead: whether it is configured, its source, the executable basename, whether that executable is present, and a recommended action.
- Every per-session event database carries an immutable owner and physical identity. A database that was copied between sessions or workspaces is refused before any migration or mutation — no adoption, no rebinding, no automatic repair. The operator recovery path is documented under "Session event store ownership".
- Markdown files under `<workspace>/knowledge/` are injected as a bounded workspace knowledge snapshot before each accepted turn, including task, task-creator, and Heartbeat wakes. It is prompt context for the turn, not durable memory.
- Installing an extension from Git now requires Git 2.37 or newer and reports `extension_git_version_unsupported` when it is older. Git sources must be HTTPS and resolve to public addresses.

Migration notes: `compozy provider auth login --print-command` is removed. The config key `memory.recall.signals.metrics_enabled` is removed with no alias. The task-notification native tools take `workspace_id` instead of `workspace`, and the old input is not an alias. Notification cursor identity and `delivery_id` are now opaque values that must be echoed back byte for byte.

##### implement-tasks replaces the software-delivery Loop

The bundled dev-cycle Loop now does one job clearly: implement authored task files in dependency order. `software-delivery` is gone and `implement-tasks` takes its place, with a five-node graph — `slug_input → load_tasks → implement → execute_task → collect` — and only three inputs. The old second control layer for review, command verification, and human approval is removed from the bundled Loop; task-level validation, self-review, tracking updates, and optional per-task commits stay inside the implementation agent's own prompt. (#325)

- Inputs are now `slug`, `implementer`, and `auto_commit`. The `review`, `verify`, and `approve` nodes and their edges are deleted, along with the verification contract, stale hash fields, and target-branch handling.
- The separate bundled `review-and-fix` Loop is unchanged, and custom Loops can still declare their own command gates — `verify_command` remains part of the generic Loop DSL.
- The catalog, Loop overview, configuration examples, migration guide, web routes, and the official CompozyOS skill all name `implement-tasks`.

Migration notes: this is a hard cut with no alias. Any config, CLI or API call, automation binding, or documentation link that says `software-delivery` must say `implement-tasks`, and the `target_branch` and `verify_command` inputs must be dropped from `[loops.inputs.*]`.

```toml
# before
[loops.inputs.software-delivery]
target_branch = "main"
verify_command = "make gate"
auto_commit = false

# after
[loops.inputs.implement-tasks]
auto_commit = false
```

##### One task-delivery Loop with two execution modes

`implement-tasks` now owns both task-delivery paths. Its default `per-task` mode keeps one isolated `code_implementer` session per task; `mode=orchestrated` uses the bundled `orchestrator` agent to spawn, prompt, verify, and stop one bounded worker per task.

- Four optional runtime inputs choose the conductor, backend workers, frontend workers, and every other worker. Task-frontmatter runtime fields still win over these run inputs.
- `compozy spawn` now accepts provider, model, reasoning-effort, and speed overrides, so orchestrated workers preserve the complete runtime choice.
- Goal output contracts now require the runtime's `complete|blocked` vocabulary, and Goal prompts receive the authored output schema.
- The standalone `orchestrate-tasks` Loop and its docs/catalog entry are removed. Operator-side `[loops.inputs.orchestrate-tasks]` config blocks are now inert and should be deleted; move any desired values under `[loops.inputs.implement-tasks]` and set `mode = "orchestrated"`.

##### Remove memory, Dream, and Knowledge; sessions use native agent compaction

CompozyOS now centers on local agent sessions, Tasks, Loops, Goals, and explicit Gateway access.
Agent memory, Dream consolidation, the Knowledge app, and the workspace `knowledge/` prompt injector
are removed from all product surfaces with no compatibility aliases. CompozyOS also stops compacting
sessions itself: the agent owns its context window, every rebuild of a session into a new agent
session is bounded, and the agent's native compaction is observed and can be requested.

### Migration

Back up `compozy.db` and the daemon state before upgrading, and export any memory database state you
need (index, decisions, Dream history) with the previous release first. Nothing needs editing by hand
after the upgrade; the daemon starts normally:

- Migration `00130_retire_memory.sql` permanently drops the memory tables and
  `goose_db_version_memory`, deletes `memory.consolidated` automation triggers (run history stays),
  and deletes legacy `dream`, `memory-extractor`, and `checkpoint-summary` sessions. Migration
  `00009_unarchive_compaction_spans.sql` restores history that the removed CompozyOS compaction had
  archived (including its transcript entries), so old compacted sessions show their full history again.
- `config.toml` is archived on load: `[memory]`, `[roles.dream]`, `[roles.checkpoint_summary]`,
  `[roles.memory_extractor]`, `[roles.memory_controller]`, `[session.compaction]`, `memory.consolidated`
  trigger entries, and the hook-matcher keys `compaction_reason` / `compaction_strategy` move, commented
  and lossless, to the end of the same file under `# Archived retired memory and compaction settings; these values are inactive.`
  The daemon logs `config.retired_keys_archived` once. If the file cannot be rewritten (it changed
  concurrently, or it is read-only), the daemon logs `config.retired_keys_archive_failed`, loads with
  the retired values inactive, and retries on the next load; the start is never blocked.
- `memory_policy` in `SOUL.md`, retired `compozy__memory*` tool and toolset IDs in tool policies, and
  extension manifest entries `memory.backend`, `memory/*`, and `memory.read|write`, extension
  automation resources triggered by `memory.consolidated`, and the hook-matcher keys
  `compaction_reason` / `compaction_strategy` in `AGENT.md` and `SKILL.md` hooks are ignored with a
  warning. Saved desktop layouts drop the Knowledge window.
- Markdown memory files, `knowledge/` directories, and `ledger.jsonl` files stay on disk and are no
  longer read. No file is deleted, and the session directories of the deleted legacy sessions are never
  recataloged. The ignore and archive rules are removed in v0.6.0.

Port scripts and extensions off `compozy memory`, `/api/memory*`, `compozy__memory_*`, the Host API
`memory/*`, and the `memory.backend` capability; no replacement memory feature is provided. New and
changed surfaces, all experimental: `compozy session compact`, `POST .../sessions/{session_id}/compact`,
`compozy__session_compact`, the Compact now action, the `compaction` transcript item, the reshaped
`session.compaction_fired` payload and usage markers, and observation-only `context.pre_compact` /
`context.post_compact` hooks with a `compaction_trigger` matcher.

There is no in-place downgrade. Restore a complete pre-upgrade backup before running an older binary.
See the [migration guide](https://compozy.com/docs/migration#memory-removal) for the state disposition
and integration changes.

##### Remove Network, Bridges, and managed Sandbox products

CompozyOS now centers on local agent sessions, Tasks, Loops, memory, and explicit Gateway access.
Network, Bridges, and managed Sandbox capabilities are removed from all product surfaces with no
compatibility aliases. HEARTBEAT, SOUL, Task autonomy, Goal nodes, session supervision, attention
notifications, task status cursors, and provider-native execution policies remain supported.

### Migration

Back up the daemon state and workspace `.compozy/` directories before upgrading. Export any retired
product history using the previous release first. Migration `00121_retire_network_bridges_sandbox.sql`
permanently drops Network state and wake runs, Bridge instances/routes/deliveries and task subscriptions,
notification presets/delivery permits, Network-actor triage state, and retired fields on retained records. It clears retained Task
references to removed Network wake runs and removes Loop channel-message events. Retained local sessions,
Tasks, ordinary runs, workspaces, memory, SOUL, and HEARTBEAT state are preserved.

Remove Network, Bridges, and managed Sandbox configuration and request fields. Port scripts and extensions
to the current CLI, routes, tools, and SDK; no replacement messaging or remote Sandbox feature is provided.
Gateway requirements move to `[gateway]` with `gateway.private` / `gateway.public` permission atoms,
`gateway_requirement_digest`, and `--confirm-gateway-requirement` / `confirm_gateway_digest` confirmation.
The previous Gateway consent tuple remains recorded. Legacy `network_participation` manifests must
be rebuilt with `[gateway]` and their new requirement digest confirmed; no legacy manifest conversion
or implicit Gateway authorization is provided.

There is no in-place downgrade. Restore a complete pre-upgrade backup before running an older binary.
See the [migration guide](https://compozy.com/docs/migration#networks-bridges-and-sandbox-removal) for the
state disposition and integration changes.

##### An agent can no longer send a session message to its own session

`compozy__session_prompt` now refuses a call whose `session_id` is the calling session. The call fails with `invalid_request` ("session_prompt cannot target the calling session.") and nothing is admitted. A self-prompt could only queue into, steer into, or interrupt the caller's own turn, so it never did useful work. Now that every session message carries its sender and can request a reply, it would also make a session reply to itself.

- Only calls made from an agent session to that same session are refused. Messages to other sessions, operator prompts from the CLI and HTTP, and `compozy session prompt` behave as before.
- A send admitted before the upgrade and retried with the same `message_id` and `idempotency_key` still returns its original result.

Migration notes: this ships without the usual one-release deprecation window. The exception is recorded in the agent-collaboration spec (Business Rule 18, ADR-002). To keep working in the same session, end the turn. To schedule work, use a subagent (`compozy__subagent_delegate`) or `compozy session prompt` from the operator.

##### Terminal history stays outside project directories

Terminal journal databases now live in `~/.compozy/workspaces/<workspace-id>/compozy.db`, beside
the workspace's retained terminal artifacts and recordings. The configured `COMPOZY_HOME` is
respected. Terminal use no longer creates SQLite databases or WAL/shared-memory sidecars in the
project's `.compozy/` directory.

## Migration notes

This is an explicitly approved storage hard cut. Existing `<project>/.compozy/compozy.db` files
are not imported, read, modified or deleted. The new journal starts empty, so old command history
and its recording/artifact index are not available through the journal after upgrading. Retain
the old files if you need that history; never remove SQLite files while an older daemon uses them.
New history persists across restarts and remains isolated by workspace and profile.

##### The desktop app is now Electron

The Tauri/Rust desktop host is replaced by an Electron shell with a narrow preload boundary, and updating became one durable daemon-owned operation exposed identically through the CLI, HTTP, UDS, Settings, and the menubar. The app provisions the bundled daemon from an empty home, or attaches to a compatible daemon that is already running without taking ownership of it, keeping single-instance focus, deep links, safe navigation boundaries, page zoom, window geometry recovery, diagnostics, logs, and the owned-versus-attached quit contract. (#424)

- Runtime and App are separate update tracks with operation progress, holder-aware blocked state, staged-next-launch state, apply and cancel actions, and truthful absence when a track is unsupported.
- A keyboard-accessible menubar indicator appears only when an update is actionable and navigates to Settings. The renderer holds no desktop-only update authority, and the SPA behaves the same in a browser and in the app.
- Desktop artifacts are planned, inventoried, and channel-checked as one release authority, with notarization and signing input checks and packaged smokes provisioned from empty isolated homes. On macOS the ZIP and on Linux the AppImage are the updater artifacts; DMG and DEB are install artifacts only.

Migration notes: this is a hard cut with no compatibility bridge. The Tauri runtime, commands, permissions, capabilities, fixtures, generated bindings, Cargo dependencies, build configuration, scripts, config keys, docs, and tests are deleted rather than deprecated. Install the app from the artifacts published with this release. The installed-app update walk from one beta to the next was not verified for this build, so the App track must be proven by a release owner across a fresh beta pair before it is treated as delivered.

##### The OS Release

CompozyOS v0.3 is a new operating system boundary for agent work. Sessions, tasks, loops, memory,
permissions, automation, the OS shell, and Compozy Network now share one daemon-owned state model.
People can start and inspect that work from the web, CLI, HTTP/SSE, or UDS. Agents can operate the
same runtime through structured tools and extension contracts.

This is a breaking beta. The command, package, environment, storage, API, and tool namespaces move
to CompozyOS, and several v0.2 surfaces have deliberate replacements or removals. Follow the
[v0.3 migration guide](https://compozy.com/runtime/migration/) before replacing an existing install.
The maintained v0.2 line and its collateral remain on `legacy/v0.2`.

Install the beta through the verified hosted installer, `@compozy/cli@beta`, or the explicit
`github.com/compozy/compozy@v0.3.0-beta.1` Go version. The beta channel may change before v0.3.0
stable; production rollouts should pin the version and review each prerelease.

The repository was already MIT licensed. v0.3 corrects stale BSL-1.1 text in distribution metadata;
it does not relicense the code.

##### Twelve long-tail providers leave the built-in registry

CompozyOS now ships only the built-in providers people actually use: `claude`, `codex`, `agy` (Google Antigravity), `gemini`, `opencode`, `kimi-cli`, `qwen-code`, `copilot`, `cursor`, and the `pi_acp` harness with `pi`, `openrouter`, `zai`, `moonshot`, `vercel-ai-gateway`, and `xai`. Twelve providers with no owner and no end-to-end validation leave the registry. With Google Antigravity added, the current catalog contains 15 built-ins. Each still runs as a custom `[providers.<id>]` entry, and existing references keep working for one release. (#761)

- Removed standalone ACP CLIs: `hermes`, `openclaw`, `blackbox`, `cline`, `goose`, `junie`, `kiro`, `openhands`, and `qoder`.
- Removed `pi_acp` API-key providers: `mistral`, `groq`, and `minimax`.
- Removed aliases: `blackbox-ai`, `blackboxai`, `cline-cli`, `goose-cli`, `hermes-agent`, `junie-cli`, `open-hands`, `openhands-cli`, `openclaw-cli`, `qoder-cli`, `minimax-ai`, `minimax-cn`, `mistralai`, and `mistral-ai`.
- Provider lists, provider inspect output, HTTP and UDS provider payloads, the model catalog, Settings, and the provider pickers no longer offer them.
- Hermes and OpenClaw lose their dedicated behavior. CompozyOS no longer treats them as native readers of `.agents/skills`, so the Agents skill source has no native readers and CompozyOS injects those skills for every provider. It also stops managing `HERMES_HOME` and `OPENCLAW_STATE_DIR` for isolated provider homes, stops running live Hermes model discovery, and stops applying the provider-managed runtime strategy to OpenClaw.
- Sessions and transcripts recorded with a removed provider stay readable: history, transcript, and inspector.

Migration notes: until v0.5.0, a reference to a removed ID or alias keeps resolving with the former built-in definition. That covers `defaults.provider` in `config.toml`, a partial `[providers.<id>]` override, an agent definition's `provider:`, Loop and automation runtime defaults, and persisted sessions. The daemon logs one `provider.retired_builtin` warning per ID with `removed_in=v0.5.0` and a `replacement` field that holds the former `[providers.<id>]` entry. The warning never echoes operator-written values. Its `action` field says to add the entry, or, when `[providers.<id>]` tables already exist, to merge it into them by adding each missing key and keeping every value already set. An existing `auth_mode` or `credential_slots` choice is respected: the replacement then adds no credentials. Log redaction masks secret references, so for `mistral`, `groq`, and `minimax` the logged entry omits the `secret_ref` line and the warning's `action` field names the value to add back (`env:<TARGET_ENV>`). Declaring `[providers.<id>]` with its own `command` (plus `harness = "pi_acp"` for `mistral`, `groq`, and `minimax`) makes it a plain custom provider and silences the warning. In v0.5.0 the shim is removed, and an undeclared reference fails like any unknown provider (provider unavailable). To keep Hermes:

```toml
[providers.hermes]
display_name = "Hermes"
command = "hermes acp"
```

To keep Mistral (Groq and MiniMax follow the same shape with `GROQ_API_KEY` / `openai/gpt-oss-120b` and `MINIMAX_API_KEY` / `MiniMax-M2.1`):

```toml
[providers.mistral]
display_name = "Mistral"
command = "npx -y pi-acp@latest"
harness = "pi_acp"
runtime_provider = "mistral"

[providers.mistral.models]
default = "devstral-medium-latest"

[[providers.mistral.credential_slots]]
name = "api_key"
target_env = "MISTRAL_API_KEY"
secret_ref = "env:MISTRAL_API_KEY"
kind = "api_key"
required = true
```

The other former commands are `openclaw acp` (with `session_mcp = false`), `blackbox --experimental-acp`, `npx -y cline@latest --acp`, `goose acp`, `junie --acp true`, `kiro-cli-chat acp`, `openhands acp`, and `npx -y @qoder-ai/qodercli@latest --acp`.

#### Features

##### Agent Plugins install as extensions

CompozyOS ingests [Agent Plugins 1.0.0](https://agent-plugins.org/) packages as extensions with no product-specific manifest. A portable plugin contributes skills plus local or remote MCP servers while keeping the existing extension lifecycle, trust, isolation, diagnostics, Marketplace, CLI, HTTP, UDS, native-tool, and Web management surfaces. (#419)

- Manifest discovery and validation are strict: fixed-location skills and MCP configuration, closed schemas, deterministic diagnostics, native-manifest precedence, and safe rejection of unsupported components.
- Portable skills and stdio or streamable-HTTP MCP servers are synthesized into the canonical extension model, with absolute `PLUGIN_ROOT` and `PLUGIN_DATA` expansion, single-token stdio commands, package-root working directories, remote-header bindings, URL policy, and secret redaction.

Migration notes: end-to-end delivery is claimed only for the provider paths proven end to end, Claude Code and Hermes. OpenClaw's current ACP bridge advertises `session_mcp=false`, so CompozyOS fails closed instead of pretending to deliver session MCP servers.

##### Agents operate commands without the UI

Everything the palette does is reachable from the CLI, HTTP, UDS, and native tools, with the same reasons and the same gates. An agent supervising CompozyOS discovers a command, checks its contract, targets a client, invokes it, and follows the approval — never depending on a browser. (#441)

- Native tools: `compozy__cmd_palette_list` reads the daemon-canonical catalog for the bound workspace, and `compozy__cmd_palette_invoke` runs one command with `id`, optional `args`, and optional `client`. Availability, targeting, single-flight, and approval rules all still apply.
- Every refusal is structured and carries the same text the UI row shows: `command_not_found`, `invalid_arguments` naming the fields, `no_attached_shell`, `multiple_clients` listing every attachment ID, and `already_running`.
- HTTP and UDS expose the catalog, clients, invocation, and stream under `/api/cmd-palette/*`, plus rank signals, usage, pins, and personalization. Approvals are read and canceled through `/api/tools/approvals/{id}`.
- Configuration parity is complete: bindings, aliases, pins, and personalization resets go through the same validated daemon paths Settings uses, and a change made by an agent reaches connected shells without a restart.
- `compozy approvals show|cancel` is a new top-level verb for the tool-approval lifecycle behind any invocation.

```bash
compozy cmd-palette invoke session.new --client <attachment-id> -o json
compozy cmd-palette invoke <destructive-id> -o json   # returns approval_pending + approval_id
compozy approvals show <approval-id> -o json          # pending → terminal
compozy approvals cancel <approval-id>
```

##### See every project at once with All projects

The topbar globe now turns on **All projects**, a real place to work. Sessions, Tasks, Loops and Goals,
Automations, Terminals, Worktrees, Agents, Home, and needs-you badges list every project plus User,
and each row names its project. Items open on the All projects desktop without switching projects,
New session asks which project (and, under All profiles, which profile) it belongs to, and turning
the globe off returns to the project or User exactly as you left it. Listing toolbars gain a
**Project** filter, and the session list gains **Group by** (None, Project, Profile).

- CLI: `--all-workspaces` on `loop runs`, `loop requests`, `loop list`, `task list`,
  `automation jobs|triggers|runs|suggestions`, `worktree list`, `terminal list`, and `agent list`, with
  a `PROJECT` column (`User` for User rows, `?` when unknown).
- HTTP/UDS: `all_workspaces=true` and repeatable `workspace_ids` (`@user` for User) on sessions,
  tasks, the task dashboard, the overview, and automation jobs, triggers, and runs, with their existing
  envelopes and app access. New operator-only aggregates that refuse app tokens: `/api/loop-runs`,
  `/api/loop-requests`, `/api/loops`, `/api/worktrees`, `/api/terminals` (local only),
  `/api/automation/suggestions`, and `/api/agents/aggregate`. Agents need the unrestricted grant to read
  them; others get `agent_scope_denied`. New errors: `workspace_breadth_invalid`, `workspace_reserved`,
  `window_binding_invalid`.
- Automation runs record the project of the job or trigger that started them (`workspace_id`,
  `attribution`); runs whose project cannot be established read **Unknown project**.
- Window, desktop, and layout tools accept `workspace: "global"` for the All projects desktop
  (operator only; every agent is refused).

Breaking: the needs-you bell's `workspace_label` for items without a project changes from `"Global"`
to `"User"`. It is a display string; match on `workspace_id` instead.

Migration notes: `shell.sessions.scope` is deprecated. Through v0.4.0 it stays stored and returned,
old clients may keep writing it, and each write warns and names `shell.sessions.group_by` and the
topbar globe. While `group_by` is unset, `scope = "all-workspaces"` groups the list by project; an
explicit `group_by` wins. In v0.5.0 the daemon turns `scope = "all-workspaces"` into
`group_by = "project"` when grouping is unset, drops the key with a logged notice, and removes it from
`config.toml` on the next apply; it never blocks startup. To migrate now, run
`compozy config unset shell.sessions.scope` and `compozy config set shell.sessions.group_by project`.
Automation run history is kept: older runs whose job or trigger still exists get its project
(`derived`), the rest stay `unknown`. Per-project desktops and saved layouts are unchanged; All
projects uses the profile's existing `global` desktop.

##### Build apps on CompozyOS with TypeScript and Go

External applications can now connect to a running CompozyOS daemon with `@compozy/client` or
`github.com/compozy/compozy/sdk/go/client`, create sessions, call typed operations, and read agent
replies and live events. The daemon continues to own execution and durable transcripts.

- `Compozy.connect()` discovers the local daemon. Generated resource methods cover JSON operations;
  `prompt()` handles streamed and accepted replies, `promptResponse()` forwards a UI message stream
  through your server, and `events()` reads SSE with bounded reconnection. The Node-only
  `@compozy/client/node` entry can start a private daemon and stops only one it owns.
- The standard-library-only Go client discovers the Unix socket first and exposes generated typed
  methods and event streams, plus the prompt stream entry point. Both clients report major/minor
  version mismatches without blocking the connection.
- `compozy app-token create|list|update|revoke` and local `/api/app-tokens` routes manage named bearer
  tokens. Creation reveals the secret once. Exact origin allowlists let browser apps call the
  explicitly allowed operations; management and privileged operations remain local. Revocation
  rejects new requests and closes the token's streams. App tokens never authenticate Gateway
  listeners and remain separate from paired devices.
- `GET /api/openapi.json` serves the running release's valid OpenAPI 3.0.3 document with named
  component schemas, ETag caching, and gzip. Prompt responses now describe their actual stream
  contract. Code generation produces both clients and the SDK samples shown in API reference pages.
- The Web app uses API types from the public package. The new **Build an app** documentation covers
  Node, a server-backed chat UI, browser access, Go, and token administration.

Experimental in this release: names and shapes may change in the next release without a deprecation window. Pin the exact
version. This applies to the client facade/resource layout, Node entry point, Go client, app-token
CLI/HTTP, and browser-access error codes. Existing daemon operations and the runtime OpenAPI endpoint
retain their normal compatibility guarantees.

Migration notes: the app-token schema migration is additive and preserves existing devices,
profiles, sessions, and credentials. Existing local callers and the same-origin CompozyOS Web app
continue to work without an app token. Aborting a client stream only detaches the reader; explicitly
cancel a session prompt to stop its turn.

##### Ask the agent when nothing matches

A query with no strong result no longer dead-ends. The palette offers one visually distinct `Ask agent: '<query>'` row; pressing Enter creates a session with the workspace's default agent and uses the query as the opening prompt. (#441)

- Nothing is sent before Enter. Typing never carries the query to a provider, and a rapid double Enter still creates exactly one session.
- A weak-but-real match keeps both the results and the fallback row; only a query below the served threshold is fallback-only.
- With no workspace default agent, Enter opens the agent picker first. If the session fails to start, the failure names its reason and the palette reopens with your query intact.
- Turn it off in Settings → Palette, or set `fallback_targets = []`. Both report the same desired state, and the row disappears immediately.

```toml
[cmd_palette]
# The current runtime accepts "agent". Use [] to disable the fallback row.
fallback_targets = ["agent"]
```

##### Attention: know which session needs you

CompozyOS has one daemon-owned attention model. Pending input, permission requests, finished-unseen sessions, operator presence, notification delivery, and cross-workspace session discovery are runtime state instead of per-surface guesses. Orchestrator agents get structured wait, spawn, stop, approve, clarify-answer, prompt-cancel, and notify controls across native tools, CLI, HTTP, and UDS, so an agent supervising other agents no longer polls a shell or depends on the web UI to act. (#422)

- The global catalog persists canonical attention revisions, pending interactions, seen and settled state, and cursor-stable attention ordering.
- Presence leases, attention summaries and events, sanitized interaction discovery, generalized waits, prompt cancellation, operator notifications, and session wake propagation are available on every transport, with deterministic CLI exit behavior.
- Desktop shortcuts moved to a daemon-owned, configurable keymap, and the command palette gained nested views including an attention-first Sessions view.

##### Batuta in the Marketplace

Batuta v0.1.0-beta.3 ships as a curated community extension, so operators discover and install it from the Marketplace instead of going through the unverified GitHub-install consent flow. (#432)

- Its agent, skill, and Loop are resource-only and vendored for review, pinned to a deterministic archive digest.
- It installs from the `community` tier and still requires explicit enablement before it publishes its resources.

##### Bundled Tailscale connectivity extension

Gateway reachability ships with a first-party provider. The `tailscale` extension runs a Tailscale
node inside the CompozyOS process through `tsnet`, against the operator's own account — nothing else
to install, and CompozyOS operates no relay, server, or account on anyone's behalf. The private tier
serves `https://compozy-gateway.<tailnet>.ts.net:8443` on the tailnet; the public tier serves the
same hostname over Tailscale Funnel on 443. (#331)

- Bind the auth key once with `compozy extension secrets set tailscale --env TS_AUTHKEY` (hidden
  input); the value never appears in output, status, or diagnostics.
- The extension declares required Live network participation for `gateway.private` and
  `gateway.public`, so enabling asks for a one-time digest confirmation — and asks again only when
  that declaration changes.
- First activation provisions the HTTPS certificate before the Funnel listener opens, verifies
  public endpoints through authenticated DNS-over-TLS (`gateway.verify.public_dns_resolver`), and
  keeps unverified listeners staged with bounded retries instead of tearing them down.
- Third-party providers implement the same `connectivity.provider` contract from the Go and
  TypeScript SDKs, gated by install-source trust and control-digest re-confirmation on every enable
  and boot.

##### Complete Loop node lifecycle

Loops now have a full declarative failure contract at the node level and precise repair controls at the operator level. Authors classify failures, declare retries with backoff, route errors, absorb them with `allow_fail`, set attempt timeouts and deadlines, emit `on_*` effects, and add durable wait nodes. Operators pause, resume, cancel, kill, or requeue individual nodes and list what is waiting, quarantined, retrying, or asking for attention — all from the CLI, HTTP, UDS, native tools, and MCP, without opening the web UI. (#305)

- `compozy loop cancel` drains a run safely and `compozy loop kill` closes it immediately; `compozy loop node pause|resume|cancel|kill|requeue` repairs a single node, and `compozy loop nodes --state waiting|quarantined|attention|retrying` inventories a run.
- Agents get the same controls through `compozy__loop_cancel`, `compozy__loop_kill`, `compozy__loop_node_pause`, `compozy__loop_node_resume`, `compozy__loop_node_cancel`, `compozy__loop_node_kill`, `compozy__loop_node_requeue`, and `compozy__loop_nodes`; `compozy__loop_status` now reports node lifecycle state.
- Repeated failure on one node quarantines that node instead of terminating the run, so independent lanes keep working, and the failure cause is classified rather than matched from a magic string.
- Long-running Loop-bound sessions are no longer killed on elapsed time. Liveness is judged from real evidence, and prolonged silence raises attention instead of ending the work.
- Defaults are tunable through `loops.defaults.delivery.*`, `loops.defaults.watch.*`, and `loops.breaker.*`, and new blocking lint rules reject invalid routes, impossible timing, malformed effects and waits, and watch sources without a stable identity.

Migration notes: `compozy loop stop` is deleted — the CLI verb, the HTTP route, and the `compozy__loop_stop` native tool. Choose `cancel` or `kill` explicitly. Extension watch sources must now declare `event_key`; a source without a stable event identity is rejected before a run starts.

##### Correct one output, repair one lane

Operators can fix a settled node output without rewriting what actually happened, and can act on a single fan-out cell without disturbing its siblings. (#427)

- **Amend output** applies to a settled output while its run, node, or cell is parked, and appears only when the node declares an output shape to validate against. It shows the recorded original read-only beside the corrected value and takes a reason.
- Amendments are append-only: the recorded output is never rewritten, the corrected value becomes what resume and downstream reads see, and both stay visible in history and in a diff. Amending does not re-run consumers — pair it with **Rerun from here**.
- Run detail returns `amendments[]` with bounded, redacted values, or a byte-size and content-hash summary for large data. No API reads an amendment's private output reference directly.
- The control is available as `compozy loop node amend`, `POST /loop-runs/:id/nodes/:node/amend`, and `compozy__loop_node_amend`.
- `--item` (or `item_index`) pauses, resumes, cancels, or kills one fan-out cell without touching the rest of the window.

```bash
compozy loop node amend --run-id <run-id> --node build --item 3 \
  --payload '{"artifact":"dist/app-1.4.2.tgz"}' --reason "wrong tag captured"
```

##### Cursor models come from your account

Cursor used to look curated in CompozyOS but was not truthful to the account that was signed in: a small hand-written list stood in for the real catalog and, worse, acted as an allowlist that rejected valid model ids before Cursor ever saw them. CompozyOS now reads the account catalog from `cursor-agent models` before a session exists, and exact provider model ids are forwarded unchanged. (#320)

- The first catalog read bootstraps Cursor discovery once and persists the outcome; later reads serve the cache, and explicit refresh is the refresh boundary. In the QA account this surfaced 193 real models, including `composer-2.5`.
- Only `id - display name` rows are parsed. Headings, tips, duplicates, and empty output can never become invented models.
- Curated data is metadata again, not membership policy. Sessions and Loops accept ids like `cursor/composer-2.5`, and an unknown _provider_ still fails with a structured `unknown_provider` error.
- `providers.<id>.models.discovery` applies live — no daemon restart. A provider outage records the failure and keeps the rows you already have; disabling discovery clears them and records `disabled`.
- In the web runtime selector, "Use an exact custom model ID…" now opens a dedicated field: empty input cannot be committed, typing turns the action into `Use "<id>"`, Enter and click both commit, and closing returns to normal catalog search.
- Cursor keeps the operator `HOME` its `native_cli` login contract expects.

Migration notes: the curated Cursor allowlist and its session preflight are deleted with no compatibility bridge. If a provider rejects an id, that provider's error is now the authority.

##### Every domain opens inside the palette

> Historical note for #441: Network and Bridges were subsequently removed. Their views and
> command IDs below are no longer available. See the
> [migration guide](https://compozy.com/docs/migration#networks-bridges-and-sandbox-removal)
> for the current product boundaries and upgrade steps.

The palette is not only a launcher — it browses. Sessions, Tasks, Loops, Jobs, Agents, Triggers, Marketplace, Bridges, Knowledge, Vault, Worktrees, Network channels, and Extensions each open as a view without leaving the overlay, and views stack so one selection can push the next. (#441)

- Four view kinds ship: **list**, **detail**, **grid**, and **form**. Lists carry domain chips with truthful counts and single-select semantics; a chip with zero matches names the filter and clears in one keystroke.
- State badges come from the shared status vocabulary and always pair a glyph with a label — never color alone.
- Selecting a row previews its metadata and sanitized text in a detail pane without stealing focus from the list, and the pane clears when the row disappears from another surface instead of showing stale content.
- Form views traverse typed fields in declared order, block an invalid submit on the first failing field, and discard values when you pop the view.
- Vault rows show names and metadata only. A secret value never enters a view, a preview, or a match highlight.
- A cold open shows a loading state, never a blank list dressed up as empty; an oversized list either scrolls virtually or states the exact `showing N of M`.
- Views stream patches, so a list already on screen updates in place as the runtime changes.

```bash
compozy cmd-palette list --source core -o json | grep palette.view.
# palette.view.sessions, .tasks, .loops, .jobs, .agents, .triggers,
# .marketplace, .bridges, .knowledge, .vault, .worktrees,
# .network-channels, .extensions
```

##### Extensions contribute commands and views

An extension can add its own commands and views to the palette from `resources.cmd_palette`, beside its tools. CompozyOS validates the contribution during `extension build`, `extension validate`, install, and development reload, and prefixes every local ID with the extension name — `capture` from the `notes` extension becomes `ext.notes.capture`. (#441)

- The action union is closed: `tool` calls a tool the same extension owns, `view` opens one of its views, `navigate` opens a CompozyOS app, and `url` opens an external link. Extensions cannot declare client operations.
- A declarative view names a **read-only** tool as its source and returns the shared `v1` view payload, which the daemon validates before rendering. A mutating, destructive, interactive, or open-world tool is rejected at validation time, so opening a view never starts an approval flow.
- A programmable view sets `program: true` and is backed by the public `view.provider` surface, with patch streaming for live updates. Start from the template with `compozy extension init notes --template view-provider-ts`.
- A command can ship a `default_shortcut`. If that chord already belongs to something else, the default stays dormant and the conflict is visible in Settings instead of silently stealing the key.
- Destructive extension commands must declare themselves and supply confirmation copy; the same approval gates apply to them as to core commands.

```ts
resources: {
  cmd_palette: {
    commands: [{
      id: "capture",
      title: "Capture note",
      section: "Notes",
      icon: "pencil",
      action: { kind: "tool", tool: "capture_note" },
      default_shortcut: "alt+shift+KeyN",
    }],
    views: [{
      id: "recent",
      title: "Recent notes",
      kind: "list",
      source: { tool: "list_recent" },
    }],
  },
},
```

##### Fan-out settles with an honest count

A fan-out can declare how it settles, and a partial result stays partial everywhere it is read instead of being rounded up to success or down to failure. (#427)

- `strategy` accepts `wait_all` (the default), `fail_fast`, `race`, and `best_effort`. `best_effort` requires both a threshold — a percentage like `66%` or a count like `{ count: 2 }` — and an explicit `missing: acceptable`.
- A collect result is `succeeded`, `partial`, or `failed`, and its output carries `total`, `succeeded`, `failed`, `canceled`, `coverage_rate`, and `partial`.
- Live counts read through `nodes.<fan-out-id>.progress.*` — `total`, `succeeded`, `failed`, `canceled`, `running`, `pending`, `settled`, `success_rate`, `failure_rate` — with the short `progress.*` form inside the fan-out body. Rates are `0` for an empty collection.
- The run page separates lanes that succeeded, lanes that failed, lanes the strategy canceled, and lanes that never materialized because the window did not open them. Partiality is a run-level fact (`completion_state`), so it reads `partial` in the outcome card, the run lists, and a diff. A wide fan-out reports aggregate counts instead of one row per lane.
- The fan-out window has no daemon-wide ceiling; logical width stays bounded by each node's positive `max_fan_out`. Write-time validation rejects a negative `fan_out_width`.

```yaml
- id: inspect_files
  class: control
  kind: fan-out
  collection: "{{ .nodes.changed.output.files }}"
  bind_as: file
  strategy:
    kind: best_effort
    threshold: 66%
    missing: acceptable
```

##### First-class Git worktrees

Git worktrees are runtime objects across the daemon, Web desktop, CLI, HTTP and UDS, native tools, extensions, configuration, generated contracts, and documentation: create, adopt, discover, inspect, reconcile, dismiss, and safely remove isolated checkouts. Sessions, Task runs, fan-out workers, and Loop environments bind to an exact worktree without losing the parent workspace's config, skills, agents, or memory. (#388, #410)

- The new worktree domain owns Git capability detection, canonical repository identity, naming and placement, per-repository mutation locking, and durable lifecycle state.
- Creation is a phased `pending → ready` operation with recorded ownership checkpoints, bootstrap copy and setup support, cancel-safe rollback, and boot recovery.
- Adoption verifies the linked checkout, the common Git directory, main-checkout exclusion, and repository identity before registering it. Discovery merges Git-known checkouts with durable records without turning a discovered row into an adopted worktree.
- Removal fences the record as removing, rechecks session activity and Git safety under the repository lock, preserves branches and history, and requires an explicit second step for dirty or unique-unpushed work.
- Finishing work runs through a truthful assisted-exit ladder for commit, push, pull request, merged evidence, and cleanup. Dismissed tombstones release their reserved name, exit actions resolve caller references to canonical record ids, and CLI mutation output keeps worktree identity.

##### Grouped skill directories

CompozyOS now discovers `SKILL.md` definitions at any depth below each skill root, so teams can organize capabilities under folders such as `marketing/content/` without changing frontmatter identity or normal precedence. `compozy skill create <name> --group <relative/path>` now scaffolds grouped workspace skills safely.

##### Keep sessions alive while observable work is still running

Session supervision now considers agent progress, running tools, active children, Loop runs, task leases, and scheduled waits. A quiet transcript alone no longer means the session is idle. If a work-signal source cannot answer, supervision reports attention and suspends automatic stopping.

Observed silence warns after 30 minutes by default and enters the normal stop ladder after another 10 minutes; fresh work cancels the pending stop. Network Live participation has no aggregate wall-time limit by default, while explicitly configured and other budgets remain effective.

Capacity starvation now escalates visibly. Blocked automation fires defer durably and resume after restart, and scheduler counters distinguish skipped wakes from successful work.

PR: [#557](https://github.com/compozy/compozy/pull/557).

##### Feedback semantics for durable Loops

A rejected Loop generation no longer restarts blind. The rejection is carried into the next attempt as context, only the producers responsible for it are re-run, and an opt-in ratchet keeps the best-scoring generation instead of losing it to a later regression. Every generation now records its origin, its parent, the gate verdict, the score, and the blocking issues inside claim-fenced transactions, so the CLI, HTTP, UDS, native tools, SSE, and the web UI all read the same durable run truth. (#290)

- Loop templates can read `previous.*` (including `previous.generation` and `previous.route_causes`) and `best.*` to steer the next attempt from what actually failed.
- Metric gates take a direction — `maximize` or `minimize` — plus a `min_delta` improvement threshold, so a regression is rejected deterministically. Invalid thresholds fail authoring with `metric_min_delta_invalid`.
- `compozy__loop_status` and `compozy__loop_runs` project score, best generation, gate verdict, and generation origin and parentage; run detail, catalog, and recent-runs views render the same fields.
- `compozy extension list` and `compozy extension status` accept `--workspace`, so agents can inspect workspace dev overlays without dropping to raw HTTP.

Migration notes: this is a greenfield hard cut that discards existing Loop run history. The migration clears Loop runs, run events, gate decisions, generation outputs, goal turns and checkpoints, session bindings, and output blobs, along with the task and automation runs that referenced them. Export anything you need before upgrading.

##### Loop inputs that know what they point at

A Loop input can declare the kind of thing it accepts, and every surface that edits inputs now validates against the real workspace catalog before anything starts. A wrong agent name, a retired skill, or an unsupported model is caught at the field that caused it instead of failing deep inside a run. (#427, #438)

- New input types: `agent` (an exact agent name), `ref` with a closed `ref.kind` of `skill`, `loop`, `worktree`, `session`, `workspace`, or `secret`, and `runtime` (`{ provider?, model?, reasoning? }`, accepting exact custom model IDs). A string-like input may still declare `enum`, and those choices take precedence over a catalog picker.
- Effective values resolve one field at a time: run input, then workspace config, then global config, then the definition default. The daemon validates the winning value immediately before a dry run or a run, including entity existence and runtime support.
- A failure starts no run, creates no task and no external action, and returns the same `input_validation` payload — `{ loop, field, kind?, value?, origin, reason }` — over HTTP, UDS, CLI, native tools, and the web form, which attaches the reason to that field.
- The same typed controls appear wherever Loop inputs are edited: the run form, scheduled automation, event-trigger mappings, fork and amend flows, and human-request answers annotated with `x-compozy-kind`. Every surface submits the exact stored identifier — a display label is never treated as a reference.
- The run form reuses the canonical runtime selector instead of free-text provider, model, and reasoning fields. From the CLI, a runtime input also accepts the compact `provider/model@reasoning` form, with `-` leaving provider or model unset.
- `compozy loop run` prompts in an interactive terminal only for supported required inputs still missing after defaults; `--no-prompt` fails instead, and structured or non-interactive input never prompts, so scripts stay deterministic.
- Secret inputs expose Vault reference names and metadata only — a secret value never enters a catalog or an error payload. Two native tools back the new pickers: `compozy__agent_list` and metadata-only `compozy__vault_list`.
- `params.runtime` binds a declared `type: runtime` input through an exact reference such as `{{ .inputs.worker_runtime }}`. Provider, model, and reasoning are validated at bind time, while literal runtime objects keep compile-time typo detection, and the resolved runtime records `input` provenance.
- If a saved reference is no longer listed, the field keeps the exact value visible so the daemon returns a field-level error instead of silently replacing it.

```yaml
inputs:
  reviewer: { type: agent, default: code_reviewer }
  release_token: { type: ref, ref: { kind: secret } }
  worker_runtime:
    { type: runtime, default: { provider: codex, model: gpt-5.5-codex, reasoning: high } }
```

##### Loops can stop and ask you a question

Two new ways for a Loop to bring a person into a run: an `ask` node parks the run until someone answers a question, and a `review` block parks an action node until someone decides on the arguments it is about to run with. Both are answerable from the web run page, the CLI, HTTP, UDS, and native tools, so an agent supervising a Loop never depends on the web UI to unblock it. (#427)

- `compozy loop requests` lists what is waiting, `compozy loop request` reads one, and `compozy loop respond` answers it. Agents get `compozy__loop_requests`, `compozy__loop_request`, and `compozy__loop_respond`.
- The run page's **Needs you** region presents one question at a time — a "Question 1 of N" header steps through several instead of stacking forms. The bounded redacted context preview and the node and generation that asked sit behind a closed **Details** disclosure.
- The answer form renders only what the daemon authorized. An ask generates its fields from the `expect` schema, with enum values as choices and booleans as Yes or No; a review shows the proposed arguments with that node's own decision allowlist, so an unauthorized decision is absent rather than a disabled button. On `edit`, the fields arrive pre-filled with the proposed arguments.
- An answer that fails the shape comes back with the failure on the field that caused it and the request stays open. A request someone else already answered — or whose run has ended — shows the recorded outcome instead of a form.
- Each fan-out lane carries its own request, named by lane and answerable independently. The waits rail counts pending requests alongside timers and events.

```yaml
# a review block on an action node
review:
  when: inputs.environment == "production"
  prompt: "Review the production release"
  decisions: [approve, edit, reject, respond]
  responders: { agents: deny }
  on_reject: { route: repair_release }
```

```bash
compozy loop respond --run-id <run-id> --node publish --decision approve
```

##### Loops take one path and tell you why

A Loop graph can now pick exactly one forward path with a `route` control node, and a gate verdict can route the same way, instead of forcing authors to encode every choice as nested branches. Each decision is recorded durably, so an operator or an agent reads why a run went the way it did rather than inferring it. (#427)

- A `route` node checks its CEL conditions in declaration order and takes the first match, falling back to a mandatory `default`. Every destination must be a unique direct forward edge.
- A broken condition fails closed with `predicate_evaluation_failed`; it never falls through to the default.
- Gate verdicts (`pass`, `fail`, `error`, `timeout`, `invalid_output`) route to `continue`, `revise`, `next_generation`, `escalate`, `halt`, or an in-body forward target written as `{ route: node_id }`. `approval` accepts only `escalate` or `halt`, so an object route cannot slip past a pending approval.
- Run status carries `generations[].route_causes` — the route node or gate, the selected forward node, the lane index, the cause, and the time. It is read from the durable `route_taken` event, so HTTP, CLI, native-tool status, and SSE replay agree.

Migration notes: the `branch` gate action is removed and is now rejected at authoring time.

```yaml
- id: classify
  class: control
  kind: route
  routes:
    - { when: nodes.score.output.value >= 0.8, to: publish }
    - { when: nodes.score.output.value >= 0.5, to: revise }
  default: reject
```

##### Loops UI and empty catalogs match their design contract

Every Loops surface adopts the approved visual contract's locked review decisions, collapse and section grammar, icon budget, and truthfulness rules, rendering only daemon-backed data and adding no helper copy. Jobs, Triggers, and Tasks share one zero-inventory empty state with the same composition, density, and icon grammar. (#406, #409)

- Automation suggestions and task templates are empty-state affordances again: they render only in the unfiltered zero-inventory state instead of sitting above a populated Jobs catalog.
- The three catalogs compose the same components and differ only in icon, title, support line, action, and panel content.

##### Manage follow-ups without losing them when a turn is interrupted

Queued follow-ups survive interruption of the active turn, including agent- and Goal-owned entries. Edit, remove, steer, or explicitly clear the queue from the session controls. Clearing writes durable actor-attributed records and is also available through `compozy session input clear` and the corresponding HTTP/UDS and native-tool surfaces.

Send identities make retries idempotent: retrying the same message returns its recorded outcome, while reusing the identity with different content is rejected. When an acknowledgment is lost, the client shows Not confirmed and retries the original identity. Daemon-generated follow-ups now use the same durable queue.

PR: [#557](https://github.com/compozy/compozy/pull/557).

##### One Automations window for schedules, events, and links

The Web UI's separate Jobs and Triggers windows are now one **Automations** window. Every automation reads as one sentence — how it starts, optional conditions, what it does — with its last-run result and an On/Off switch on the row, so a failed run is visible without opening anything. One detail page and one "New automation" dialog serve schedules, events, and links; the start you pick decides whether CompozyOS stores a job or a trigger.

- `last_run` (`id`, `status`, `started_at`, `ended_at`, `skip_reason`) is on every job and trigger from `compozy automation jobs|triggers`, `GET /api/automation/{jobs,triggers}` (HTTP and UDS), and `compozy__automation_{jobs,triggers}_{list,get}`. The CLI tables gain a **Last Run** column; `-o toon` adds `last_run_status` and `last_run_started_at`.
- The list commands, routes, and tools accept `target=agent|loop|task` (`--target` on the CLI).
- The dock has one Automations launcher; the command palette gains "New scheduled automation" and "New automation on an event"; Loop pages replace "Add trigger" and "Add schedule" with **Automate ▾**; Settings → Automation uses plainer labels and writes the same `config.toml` keys.
- CLI verbs, HTTP/UDS routes, tool ids, and `[[automation.jobs]]` / `[[automation.triggers]]` are unchanged.

```bash
compozy automation jobs --workspace checkout-api -o json | jq '.jobs[] | {name, last_run}'
compozy automation triggers --workspace checkout-api --target loop
```

Migration notes:

- Saved desktops migrate permanently (window-manager snapshot v5): every Jobs or Triggers window, tab, and recently closed entry reopens as an Automations window on the matching `/automations/…` path, in the same place. No window is closed.
- Exported layout documents and `window_layout` resources at version 4 keep loading: the daemon upgrades them to version 5 and rewrites their Jobs and Triggers windows the same way. Snapshots and layout documents are version 5 from now on.
- Command palette pins, recents, and usage for the Jobs and Triggers commands merge into "Open Automations" and the Automations view; usage counts add up and no pin is dropped.
- Old web links `/jobs`, `/jobs/<id>`, `/triggers`, and `/triggers/<id>` redirect to `/automations`, `/automations/jobs/<id>`, and `/automations/triggers/<id>`, carrying their filters. The redirects are removed in v0.5.0.
- The app ids `jobs` and `triggers` (`compozy window open --app`, window commands, `window_layout` resources) and the palette ids `app.open.jobs`, `app.open.triggers`, `palette.view.jobs`, and `palette.view.triggers` are accepted as `automations` with a deprecation warning (CLI stderr; WARN logs `windowmanager.app_id_deprecated` and `cmdpalette.command_id_deprecated`). They are removed in v0.5.0 and then fail like any unknown id. Use `automations`, `app.open.automations`, and `palette.view.automations`.
- Site docs: the Automation section is now **Automations**, with two tutorials. `/docs/automation/jobs` and `/docs/automation/triggers` moved to `/docs/automation/schedules` and `/docs/automation/events` and redirect permanently.

##### Pin, rename, and bind any command

The palette learns your workspace. Pins float the commands you always want first, ranking signals push the ones you actually use, aliases give a command your own vocabulary, and any command can take a chord — including a system-wide one on the desktop app. (#441)

- Pins and recents are workspace-scoped and shared across every attached client, so a pin made in the desktop shell shows up in the browser tab. `personalization = false` turns ranking and recents off as one desired state.
- An alias is 1–32 characters with no whitespace, unique in the workspace, and searchable alongside the command's title.
- Bindings, aliases, and pins are validated against the complete effective keymap. A conflict names the command that currently owns the chord or alias and stores nothing; `--overwrite` transfers it as one atomic change.
- The desktop shell registers global hotkeys — `meta+shift+Space` summons CompozyOS with the palette open by default — and reports per-machine truth for each one: **active**, **captured** by another app, **permission required** (with a shortcut into macOS Accessibility settings), or **unsupported**. A browser tab shows the section disabled with the reason _requires desktop shell_ rather than pretending.
- Settings → Palette exposes the agent fallback and personalization; Settings → Layouts → Shortcuts owns the keymap, aliases, and the global section.

```bash
compozy cmd-palette pin palette.view.sessions --workspace acme
compozy cmd-palette alias set session.new new --workspace acme
compozy cmd-palette bind palette.view.tasks meta+shift+KeyY --workspace acme
compozy cmd-palette bind palette.summon.global meta+shift+Space --global
compozy cmd-palette personalization show --workspace acme -o json
```

```toml
[cmd_palette]
personalization = true

[cmd_palette.aliases]
"session.new" = "new"

[window_manager.global_shortcuts]
"palette.summon.global" = "meta+shift+Space"
```

##### Reconnect and read long sessions with a truthful live view

Session streams recover from their last delivered position, including an empty catch-up acknowledgment when already current. Brief interruptions stay quiet; repeated failures show Reconnecting, then Disconnected with Try again. Sending while disconnected keeps the draft. Compaction and retention recovery preserve a consistent active turn and replace expired history windows explicitly.

Completed tool work and settled turns fold into readable summaries, interrupted work stays open, and Thinking is explicit. Reading older history or expanding work no longer competes with live scrolling. Large payloads have Show all and Download controls; stop and steering provenance remain visible on the relevant messages.

Bounded agent event ingestion and durable-history recovery prevent a slow watcher from wedging the producer or silently losing accepted output. Streaming redaction was optimized while preserving its output.

PR: [#557](https://github.com/compozy/compozy/pull/557).

##### Remote gateway: reach your daemon from anywhere

A fresh install is still reachable only from the machine it runs on — and now that is a choice
instead of a limitation. The remote gateway adds three independent, off-by-default switches: a
private overlay that serves the full product to devices you pair over your own Tailscale network, a
public delivery ingress that accepts only signed webhook and bridge callbacks, and consent-gated
public operator access for devices that cannot join the overlay. (#331)

- The daemon never binds a public address. Gateway tier listeners stay on loopback and a
  connectivity provider publishes a verified route to them: an address is advertised only after the
  daemon fetches a one-time challenge through it and gets its own nonce back.
- Reaching an address is never authentication. Devices pair with single-use, five-minute artifacts
  written to private `0600` files, credentials are stored only as hashes, and
  `compozy device revoke` cancels live streams before it returns.
- `compozy gateway status|audit`, `compozy pair`, `compozy device`, and `compozy connect` (HTTPS
  profiles plus zero-exposure SSH) operate everything, with the same state in **Settings → Gateway**
  and the `compozy__gateway` native tool.
- Public delivery verifies CompozyOS's timestamped HMAC contract on every request, with replay
  protection and per-source rate limits. There is no store-and-forward while the daemon is offline —
  senders own retries.

Setup guides live in the new Gateway docs section: https://compozy.com/docs/gateway.

##### Reversible session archiving and list actions

A stopped session can now be archived so it leaves the default catalog without deleting anything. History, events, ledger, and the saved runtime choice stay readable, and unarchiving puts the session back exactly as it was — still stopped, so a normal prompt restarts it. Both session lists gained a row menu with state-aware Stop, Archive, Unarchive, and Delete, a delete confirmation, and a separate section for archived sessions. (#309)

- `compozy session archive <id>` and `compozy session unarchive <id>`, plus `compozy session list --archived` for archived only and `--include-archived` for both. Agents get `compozy__session_archive` and `compozy__session_unarchive`, and extensions get `sessions/archive` and `sessions/unarchive` under `session.write`.
- The catalog contract takes `archive=exclude|only|include` and defaults to `exclude`, with exact filtered totals and cursor fingerprints. Archived sessions are excluded from normal metrics.
- Archiving is stopped-only and idempotent. An archived session stays readable, but prompt, attach, and resume are refused until you unarchive it. Hard delete is unchanged, and archive stays catalog metadata rather than a lifecycle state.
- Bridge providers now wait for HTTP route readiness before serving, which closes a startup race across the bundled Discord, Google Chat, GitHub, Linear, Slack, Teams, Telegram, and WhatsApp runtimes.

Migration notes: existing sessions are unarchived, so nothing changes until you archive something.

##### Rewind a session to an earlier checkpoint

You can now rewind an idle session back to one of your earlier messages instead of starting over. The selected message and everything after it leave the active transcript, the message text comes back as a composer draft, and the session continues under the same session ID with a fresh agent context rebuilt only from the part you kept. Rewind touches the conversation only — it does not undo file edits, tool effects, network activity, saved memory, or anything the provider already did outside CompozyOS — and the discarded events stay archived for audit. (#310)

- `compozy session rewind <session-id>` picks the cut point with `--message-id` and reads the current transcript fences for you; scripts retrying a known request pass `--expected-generation`, `--expected-epoch`, and `--expected-max-sequence` together with the original `--idempotency-key`. Agents get `compozy__session_rewind`.
- Retrying the same rewind with the same idempotency key returns the original result, and the response carries the `draft_text` that goes back into the composer.
- A rewind is refused with a clear conflict — and without cutting the transcript — when the fence is stale, the session is busy, input is queued, an approval is pending, or the session is daemon-managed. It is serialized against clear, delete, repair, resume, and other prompt-producing operations.
- In the web UI, the action appears on your own durable messages, requires an empty composer, confirms the side-effect boundary before it runs, and restores the draft afterward.
- Reads take an `archive` selector: `compozy session events` and `compozy session history` accept `--archive active|archived|all`, and the same selector exists on the HTTP and UDS reads.

Migration notes: `session events` and `session history` now default to `archive=active`. They previously returned archived rows alongside active ones — pass `--archive all` to keep the old behavior.

##### Runtime speed is part of the runtime

`speed` joins provider, model, and reasoning as a first-class part of a Loop runtime selection, and reports the same value everywhere it is observed. (#438)

- Speed is accepted on Loop runtime inputs, per-node runtimes, Loop defaults, and `config.toml` Loop runtime defaults, and appears in resolved provenance across CLI, HTTP, UDS, native tools, SSE, and web inspection.
- The web run form reuses the existing runtime selector's Fast control rather than introducing a parallel concept, and the run inspector shows resolved provenance read-only.
- CompozyOS reports whether speed was applied or is unsupported by the chosen provider instead of inventing support it cannot deliver.

Migration notes: the session creation profile moves to v3 as a hard cut, with no v2 branch.

```yaml
runtimes:
  worker: { provider: codex, model: gpt-5.4, reasoning: high, speed: fast }
  judge: { provider: claude, model: opus, speed: normal }
```

```bash
# the compact CLI form; "-" leaves a field unset, so speed-only intent is -/-:speed=fast
compozy loop run --name release --input worker_runtime=codex/gpt-5.4@high:speed=fast
```

##### Search the full conversation and jump between your messages

Find matches in older, unloaded conversation history and jump directly to the containing message or tool result, opening the necessary folds. Search covers text, tool names, filenames, errors, inputs, and outputs. An operator-message outline and a compact message trail make long sessions easier to navigate without losing your place as output streams.

The same reads are available through `compozy session search`, `compozy session outline`, HTTP/UDS, and the corresponding native tools. Search results carry location hints so clients can open the exact matching work row.

PR: [#557](https://github.com/compozy/compozy/pull/557).

##### Session attachments: paste, drop, or pick

The session composer accepts images (PNG, JPEG, WebP) and files (PDF, Markdown, plain text) by paste, drag-and-drop, or file picker. Attachments persist before the prompt is accepted, ride the prompt as provider-neutral references, and reach multimodal agents as protocol-conformant ACP content blocks gated by the capabilities that agent negotiated at initialization. Saving a screenshot to disk and describing its path is no longer the workaround. (#412)

- The daemon keeps the agent's prompt capabilities from the initialize handshake instead of discarding them, so unsupported content is refused in place rather than sent to an agent that never advertised it.
- Attachments render durably in the transcript across reload, live streaming, recap, and archive, and they are deleted with their session.
- The capability gate lives inside the composer's attachment strip. Steering a running prompt stays text-only.

##### Session-aware slash commands

Slash commands in the composer are now backed by a single daemon-owned catalog scoped to the exact session, and they work anywhere in a prompt rather than only at the start. Built-in and ACP control commands stay standalone, while a skill command can be dropped inline, repeated, and mixed with the text you already typed; the matched skill's full instructions are injected into that same turn. The same catalog is readable from the CLI, HTTP, UDS, and a native tool, so agents can discover what a session can actually run. (#311)

- `compozy session commands <session-id>` lists the catalog, and agents read it with `compozy__command_list` in the `compozy__catalog` toolset. `compozy__skill_view` accepts a source-qualified `command_id` from that list.
- The catalog groups Built-in, Agent, and Skills at the start of a prompt and shows only effective skills inline. A bare `/skill` resolves the effective winner across bundled, global, additional, workspace, and agent-local sources; extension skills use `/extension-id:skill` and marketplace skills use `/registry-id:skill`.
- What is effective respects global and workspace scope, agent activation and disable lists, runtime disable overlays, enabled and disabled extension resources, workspace and session ownership, and live resource revisions. A `session_commands_changed` stream frame refreshes only the affected session.
- Injection is bounded at 24 KB per skill and 64 KB per turn, and repeating the same skill activates it once. Invocations persist through admission, queueing, replay, transcript storage, and the UI, and queued ones are revalidated against the exact source before dispatch.
- Slash activation is limited to human operator input. Agent-authored prompts and `compozy__session_prompt` keep slash-shaped text literal, and hooks can remove an admitted invocation but never add one.

##### Session messaging that holds up in long orchestrations

Messages between agent sessions now reach long-running targets, stay behind the operator, and no longer exhaust a conversation after a few exchanges. (#760)

- Operator input always runs ahead of messages from other sessions, which have their own queue cap, so agents cannot fill the operator's slots. `queue_position` is the real dispatch rank.
- An agent's `mode: "interrupt"` is delivered as steer, and an agent steer never cancels another session's turn: when the target cannot take an injection, the message waits in the queue.
- `supersede: true` replaces your own older message still queued for the same target. Consecutive queued messages and reply wakes from other sessions run as one turn (up to 8), each kept as its own message; operator input is never merged.
- When a target has been in one turn longer than `session.messaging.queue_promote_after` (2 minutes by default), waiting agent messages are injected into the live turn. A failed injection falls back to after-turn delivery; nothing is dropped and nothing is interrupted.
- Hops count forwarding only: answering the session that started your turn, or replying with `in_reply_to`, keeps the hop, so a back-and-forth stays at hop 1. The flood guard is now a per sender→target rate budget; a send past it returns `message_rate_limited` (HTTP 429) with the limit and retry delay.
- `in_reply_to` links an answer to the original message. It never releases the sender's reply watch, so the completion wake still arrives.
- Reply watches now expire after 24 hours by default with a **No reply** outcome, so a forgotten watch no longer keeps its sender awake. Set `reply_watch_ttl = "0s"` to keep the previous never-expire behavior.
- `compozy session prompt` gains `--from-session auto|<id>`, `--in-reply-to`, and `--supersede`. Origin stamping is opt-in: without `--from-session`, a prompt stays operator input and a hint is printed on stderr. `--in-reply-to` and `--supersede` require a sending session.
- The operator can Steer (promote) an agent-authored queue row; editing it is still refused.
- The Web UI shows an **In reply** chip, groups consecutive messages from one sender, and says why a session needs attention.
- New `[session.messaging]` keys: `rate_limit_messages` (default 30, `-1` disables), `rate_limit_window` (default `10m`), `reply_watch_ttl` (default `24h`), and `queue_promote_after` (default `2m`, `0s` disables).

Migration notes: migration 00138 is applied automatically on daemon start and preserves all queued inputs and reply watches.

##### Spawned workers get their own worktree, the parent's grant, and a first prompt

`compozy spawn`, `compozy__session_spawn`, and `POST /api/agent/spawn` can now:

- bind the child to a worktree with `--worktree <ref>` or `--new-worktree[=name] --base --branch`.
  The checkout becomes the child's working directory and provider sandbox root.
- keep the parent's permission grant with `--inherit-permissions`, and select parent-granted tools
  by toolset with `--toolset`. Neither widens the parent.
- send the first prompt with `--prompt`. The parent's reply wake then carries the child's final
  answer.
- target another registered workspace with `--workspace` together with a worktree. The daemon
  authorizes the workspace before it resolves or creates the worktree there.

In a session, `compozy__workspace_list` now also lists the other workspaces the session's
permission mode lets it address. A refused spawn now names the field, value, and rule that
refused it, and a worktree created for it is removed again. An exhausted `max_children` budget
fails with `spawn_budget_exhausted` and the live count instead of a generic conflict; subagents no
longer count against it. The native session tools' output schemas now match the session runtime they
return, so `compozy__session_create` no longer reports a schema mismatch for a session it created.

##### Steer a working session and see when Stop has actually finished

Follow-ups now default to steering. Runtimes that advertise live steering receive the guidance inside the active turn; other runtimes report an explicit interrupt-and-replace fallback. The composer shows what happened, restores refused drafts, and Settings lets you choose queueing instead. CLI and native-tool responses expose the send outcome and steering capability.

Stop uses cooperative cancellation, forced stop, and process-group termination with process identity checks. The UI stays at Stopping until the runtime can verify the outcome; an unverifiable stop remains visible with attention. Canceling a turn keeps the session promptable, including when escalation needs to replace its process. Restart reconciliation identifies crashed agents and stale decisions instead of leaving phantom activity.

PR: [#555](https://github.com/compozy/compozy/pull/555).

##### Terminals stay with the workspace and welcome people and agents

Open real terminals inside CompozyOS, watch an agent's deliberate command run live, and reconnect after closing a window or reloading the app while the daemon keeps the process running. Visible agent terminals appear without taking focus. The same terminals are available through the Web and Desktop apps, CLI, HTTP/UDS, and native tools.

Authorized people and agents in the same workspace and profile can type, resize, answer input, signal, and close concurrently. Each submitted write stays whole and actor-attributed; there is no control handoff. Explicit read-only attachments remain available. Agent execution uses the approval policy, and hidden input is redacted from retained terminal surfaces.

A command journal records who ran what, approval, outcome, and boundary-detection confidence. Recording is opt-in with retention limits. Local macOS, Linux, and Windows support interactive terminals; remote sandboxes support command execution and journaling without interactive attachment.

PRs: [#490](https://github.com/compozy/compozy/pull/490), [#552](https://github.com/compozy/compozy/pull/552).

##### The command palette runs CompozyOS

⌘K (or ⌘⇧P) opens a palette the daemon owns. Every command — shell actions, window and desktop moves, domain views, settings, and extension contributions — is registered once in the runtime and projected to every surface, so the web app, the desktop shell, the CLI, HTTP, UDS, and native tools read the same catalog with the same availability truth. (#441)

- A row is available or it is not, and the daemon says why. A command that needs an attached shell reports `requires an attached shell` in the row itself instead of failing after you press Enter.
- Commands that need input collect it inline as typed arguments — `text`, `password`, `dropdown`, `checkbox` — before anything runs. A destructive command carries its own confirmation title and confirm verb.
- Execution is single-flight per command: a second invocation while one is still running returns `already_running` until the first reaches a terminal result, so a double Enter cannot run something twice.
- A destructive command goes through the existing tool-approval path and returns `approval_pending` with a stable approval ID. `compozy approvals show|cancel <id>` follows or ends it; approve runs exactly once, deny or cancel ends it with no effect.
- A command that acts on a shell targets one attached client. With a single attachment it auto-selects; with several it asks for an explicit client and lists every attachment ID instead of guessing.
- ⌘K on a selected row opens a filterable action panel anchored to that row: the runnable action plus Pin, Set alias, and Set shortcut. Unavailable rows expose only those meta-actions and the daemon's reason.
- The catalog is live. Installing an extension, changing a binding, or pinning from another window updates open palettes without a reload.

```bash
compozy cmd-palette list --available=false -o json   # every command with the daemon's own reason
compozy cmd-palette inspect session.new -o json      # action, arguments, execution policy, risk
compozy cmd-palette clients -o json                  # the targeting source of truth
```

##### The interface speaks plain words at a legible size

> Historical note for #440: Bridges and managed Sandbox were subsequently removed, including
> the Connections and Permissions dock entries and `compozy bridge list` example below. These
> are no longer current commands or UI labels. See the
> [migration guide](https://compozy.com/docs/migration#networks-bridges-and-sandbox-removal).

Every end-user surface moved one step up the legibility ramp and one step toward ordinary language. Body text goes from 13.5px to 15px, item titles from 15 to 16, buttons and rows get real height, the radius ladder rebases on 8, and the canvas warms up — so the interface stops asking for a magnifying glass. (#440)

- Home's first run tells the truth. Instead of seven zones filled with zeros, a fresh install shows one heading and the three starts that actually exist. A machine with an agent already running is never told nothing has happened.
- Some labels now use the word people say, while the runtime keeps its canonical noun: the dock reads **Connections** (bridges) and **Permissions** (sandbox); Settings reads **Remote access** (gateway), **Notifications** (attention), **Diagnostics** (observability), and groups them under **Personal**. The old names stay searchable.
- An alias is a label and nothing more. Code, wire payloads, CLI verbs, config keys, and generated references keep the canonical name, and the canonical noun is always one step deeper in the UI.
- "Daemon" leaves the end-user surfaces for "CompozyOS" or "this machine" across gateway, sessions, onboarding, marketplace, automation, tasks, vault, loops, and settings. Sessions get a conversation glyph instead of a terminal one.
- Small caps labels become sentence case by default, with uppercase available as an explicit variant.
- Plain language never hides the machine: install, setup, and `config.toml` still run through a terminal, and this release makes no claim of a no-terminal path.

```bash
compozy bridge list      # the dock reads "Connections"
compozy gateway status   # Settings reads "Remote access"
```

##### The workspace switcher works like Command-Tab

The fullscreen workspaces overview is rebuilt as a Command-Tab style switcher over the live shell: a glass tile strip, a frosted focus plate, an identity caption, an always-visible vertical worktree menu, a full keyboard model, and a registered `⇧⌘W` shortcut. It switches workspace identity only; window arrangements stay with the Desktops overview. (#410)

Migration notes: the previous 264 px dossier grid, with its member stacks, path footers, and "Enter →" row, is deleted rather than kept behind a flag.

##### Time travel — compare, rerun, and fork a Loop run

Durable Loop history became something you can act on. `diff` reads what changed, `rerun` opens a new generation from a settled node in the same run, and `fork` starts a linked run from a historical generation without changing its source. (#427)

- `compozy loop diff --run-id <id> --generation 1 --against-generation 2` compares two generations; `--against-run` compares two runs of the same Loop and marks different pinned definitions. Large values return their byte size and SHA-256 content hash instead of an oversized inline payload.
- `compozy loop rerun --from-node verify` re-runs the selected node and its transitive dependents while unrelated settled cells carry forward; `--item` addresses one fan-out lane. The new generation has origin `operator_rerun`.
- `compozy loop fork --generation 2` pins the source run's executed definition: generation 1 is a settled `fork_seed` baseline and generation 2 executes the body with the source inputs plus any validated overrides. Lineage is two-way — the child carries `forked_from`, the source lists `forks`.
- In the web UI, **Compare…** on an Inspect generation row opens a deep-linkable comparison page whose node rows group by the same `changed / rerun / skipped / carried / verdict` vocabulary the CLI prints, and **Fork from here** pre-fills the source run's declared inputs. Two identical generations render an explicit "nothing changed" state.
- Pass `--request-id` to retry a rerun or fork after a transport failure: the same key with identical inputs returns the committed result, and a changed request under a reused key returns `timetravel_key_reuse`.
- Agents need the `loops.timetravel` capability and get `compozy__loop_diff`, `compozy__loop_rerun`, and `compozy__loop_fork`. Diff is an ordinary workspace-scoped read. An agent cannot rerun its own executing run, but it may rerun its own terminal run.

```bash
compozy loop diff  --run-id <run-id> --generation 1 --against-generation 2
compozy loop rerun --run-id <run-id> --from-node verify --reason "retry verification"
compozy loop fork  --run-id <run-id> --generation 2 --input service=payments
```

##### Trigger detail is a rule page

A trigger now reads as the rule it is, not as a job inspector with the cron stripped out: a plain-language sentence of what the trigger does, a labeled Enable switch opposite it as the only accent on the page, one When / If / Then card, a single-open Recent-runs accordion, a four-card rail, and an Inspect sheet for runtime internals. (#411)

Migration notes: the shared automation detail panel is jobs-only again, and triggers render through their own component family.

##### Unified docs and Marketplace

`compozy.com` now serves a single `/docs` experience with reworked navigation, breadcrumbs, responsive layouts, generated CLI reference pages, and API references that include Go examples. A new `/marketplace` section lists skills, extensions, MCP entries, bridge providers, and bundled capabilities with search, install commands, and detail pages. (#277)

Migration notes: two CLI verbs were renamed and their old spellings removed — `compozy mcp authorize <server>` is now `compozy mcp auth login <server>`, and `compozy memory extractor list-pending` is now `compozy memory extractor list-failures`. The `compozy network work status` alias was removed in favor of `compozy network work lookup`, and `compozy network send --body` accepts a kind-specific JSON value rather than requiring an object.

##### Window tabs in the OS shell

The OS shell now groups windows into first-class tab frames instead of assuming one window per app. Tabs carry ordered members, an active member, per-tab navigation stacks, pinning, scoped close and reopen, and bounded history. The same topology is exposed through Web, CLI, HTTP, UDS, native tools, streams, hooks, resources, layout profiles, and the bundled CompozyOS skill, so agents operate windows with the same semantics people see. (#287)

- Run multiple instances of the same app, discover their tabs from the dock and the command palette, and drag to group, reorder, or tear out a tab.
- Move, swap, and zoom by frame instead of by single window, and adjust Window Manager behavior directly from Settings.
- `compozy config set window_manager.*` applies through the canonical Settings section endpoint, so a live apply projects only that section and unrelated restart-required drift stays pending and truthful in `compozy status`.

Migration notes: persisted window layouts move to v3 as a hard cut — v2 layout compatibility paths and singleton-window assumptions were removed from the runtime, generated contracts, and layout profiles.

##### Work outside a project in User

Sessions, Loops, Goals, terminals, tasks, and automations can now run in **User**, a scope for work
that belongs to no project. Each profile has its own User scope. It runs in your home folder (set
`[user_scope] work_dir` to change it), loads your personal configuration, and never registers a
project. Pick **User** in the workspace switcher, between **All projects** and your projects, to see
only User work on its own desktop.

- CLI: `compozy session new --user`, and `--workspace @user` on every command that takes
  `--workspace`. HTTP/UDS: `POST /api/sessions` with `{"scope":"user"}`, `@user` in workspace paths and
  selectors (`/api/workspaces/@user/...`, `workspace_id=@user`). Native tools accept
  `"workspace": "@user"`. Session payloads add `scope` and `effective_workspace_access`.
- The new AGENT.md field `workspace_access: unrestricted` lets one agent read and act on every project,
  User, and every profile, including All projects aggregates. Only the operator can set it; agents
  that try get `workspace_access_operator_only`, and marketplace or extension agents cannot ship it.
  Sessions started by other agents never get more than their creator. Tool approvals still follow
  `permissions`. Every allowed decision is logged as `workspace.access_granted` with source
  `agent_grant`, and granted sessions show an **Unrestricted** badge. This deliberately reintroduces a
  grant for this one field, superseding the earlier "no grant vocabulary" rule for cross-workspace
  access.
- New errors: `session_create_scope_conflict`, `user_scope_unsupported`,
  `user_scope_worktree_unsupported`, `user_scope_work_dir_invalid`, `workspace_name_reserved`,
  `workspace_access_operator_only`. Registering `COMPOZY_HOME` now fails like the home folder
  (`workspace_home_forbidden`). User work has no worktrees.

Migration notes: the first start moves the hidden `COMPOZY_HOME` project (and any leftover
home-folder registration) into the User scope of each profile that owns its work. Tasks, automations,
Loop judges, and managed Goals that ran there now run in User; automations that target a project Loop
keep running in that project. Session identities and creation stamps are unchanged. Terminal history
and retained recordings are copied per profile and verified before the old journal folder is renamed
with `.migrated`; keep those folders for recovery. Layouts are copied when no User layout exists yet.
The hidden project is removed only after every store finishes and no row still references it; an
interrupted upgrade resumes on the next start. A hidden project with worktrees, or targeted by a
Loop automation, stays an ordinary project, addressable by its ID or name, and the log says so.
Nothing is deleted: colliding rows and project-only data (coordination, extension bindings and dev
links, model discovery contexts) are kept in `user_scope_migration_archive` with their original JSON;
`user_scope_migrations` records progress. Projects named `user`, `global`, or starting with `@` are
renamed (leading `@` stripped, `-project` appended when needed); the old name keeps working with a
deprecation warning through v0.4.0 and is removed in v0.5.0. `@user` always means User; on window
surfaces `global` always means the All projects desktop. Daemons older than v0.3.0 reject `AGENT.md`
files that contain `workspace_access`, so upgrade every daemon that reads them before adding it.
Records saved before this release carry no grant ceiling; save them again as the operator to let a
granted agent's tasks, automations, or Loop runs keep the grant.

#### Fixes

##### A burst of tool updates no longer drops the provider

An ACP provider can emit hundreds of state-equivalent updates for a single tool call. Those duplicates filled the active prompt's bounded event channel, stalled delivery, and disconnected an otherwise healthy provider. CompozyOS now keeps one canonical projection per tool call for the duration of the prompt. (#442, fixes #439)

- Only redundant nonterminal updates are suppressed. A new title, name, kind, input, or prechecked state still comes through, and terminal results and prompt completion keep their order.
- The projection is prompt-scoped and keyed by the current `tool_call_id`; it is discarded when the prompt ends and never enters a session, workspace, or global store.
- Public event shapes are unchanged — nothing about the session transcript contract moved.
- Verified against 1,100 identical in-progress updates followed by a terminal one: both the original prompt and a follow-up completed with a single call/result pair and no disconnect.

##### A crashed agent no longer looks like a finished one

When an agent process disconnected mid-answer, the stream simply ended — and everything downstream read that silence as success. A CLI consumer reached end of file and exited zero, `compozy__session_prompt` returned a result, and the only evidence left behind was stderr with no exit code. Streams are now fail-closed: success requires an explicit completion event, and disconnect, terminal error, and process exit stay three distinct outcomes. (#315, #319)

- Chunks already received stay persisted and visible. CompozyOS never synthesizes a completion for them.
- A stream that ends after partial output without a completion event fails the CLI with a clear non-zero exit, and terminal error frames are forwarded before the error is returned so machine-readable diagnostics survive.
- `compozy__session_prompt` classifies a subprocess exit as `tool_backend_failed` with `backend_dead` instead of reporting success; the partial events remain readable in the session transcript.
- Crash evidence now carries the subprocess exit code and, where the operating system exposes it, the terminating signal.
- Fatal cleanup gives the process a bounded grace period to exit on its own before being stopped, so the real exit result is no longer lost to a race with forced teardown.
- CompozyOS does not replay a prompt automatically, because a prompt may already have caused external side effects. Sending the next prompt restarts the agent process and continues the same session and transcript.

Migration notes: crash bundles move to `compozy.session_crash_bundle.v2` with structured `exit_code` and `signal`, with no v1 branch. Any consumer that treated a closed stream as success will now correctly see a failure unless a completion event was sent.

##### A finished Loop leaves nothing running

A completed or failed Loop run could leave live descendant work behind — including a next-generation source task sitting in `ready` with no task run attached. The terminal transaction now drains every open descendant, covering ready-only tasks and `needs_attention` runs, on both normal terminal settlement and coordinator execution failure. (#438)

- The public reproduction ends with zero open tasks: the run reports failed, its next-generation task is canceled, and no descendant survives.

##### A guarded history reference no longer crashes generation 1

A Loop that referenced `previous.*` defensively still failed on its first generation, because the history namespace did not exist yet when templates were evaluated. The complete shape of `previous.*` and the generation history namespaces is now defined before evaluation, so documented guarded references validate and generation 1 runs. (#438)

- History construction is topology-aware, so a node sees the namespaces its position actually implies.
- Template and materialization failures stay inside the node lifecycle instead of escalating into an opaque coordinator failure.
- Canonical compiler, linter, namespace, coordinator, and control-flow suites cover the behavior.

```gotemplate
{{ if .previous.generation }}
The prior quality gate returned {{ .previous.verdicts.quality.outcome }}.
Blocking issues: {{ .previous.verdicts.quality.blocking_issues }}
{{ end }}
```

##### Autonomous extraction stays out of curated memory

The autonomous memory extractor could write operational chatter into curated memory, and a generated slug collision could overwrite an unrelated entry. The deterministic scanner now rejects Memory v2 operational identifiers — `memory_propose`, native `compozy__memory_*` tool names, controller event names, and scanner rule IDs — and extractor, provider, and dreaming candidates no longer update an existing memory solely because their generated slug collides. Explicit filename-collision updates from direct CLI or user writes keep working. (#396)

##### Carry the selected Profile through Loop compilation and worker sessions

Profile-scoped Agents, Skills, Loops, and Session references now resolve consistently through validation, persisted responses, and execution. Orchestrated implementers and nested daemon-issued Session commands retain the validated caller Profile while preserving workspace and identity checks.

Lifecycle extension tools are authorable only in the workspace and Profile where they are enabled. Compilation sees the same tool schema and placement as execution, with normal validation and permission policy still enforced.

Loop-managed sessions also inherit configured ACP options and speed, with explicit runtime options taking precedence. This prevents session creation-profile mismatches before the provider starts.

PRs: [#527](https://github.com/compozy/compozy/pull/527), [#531](https://github.com/compozy/compozy/pull/531), [#542](https://github.com/compozy/compozy/pull/542).

##### Updated runtime dependencies and preserved profile icons

Upgrade Go runtime dependencies and the web/desktop toolchain, including the native TypeScript
checker, Bubble Tea installation wizard and CEL expression engine. Source builds require
Go 1.27.1. Existing profile icons retain their appearance when Lucide renames their symbols.
Restored desktop window frames appear before layout-heavy contents finish mounting.

Migration notes: profile reads and writes translate `album`, `book-marked`, `building-2`,
`flip-horizontal-2`, `flip-vertical-2` and `trash-2` to their canonical Lucide names without
discarding profile state. The boundary decoder is scheduled for removal in v0.5.0 after the
persisted names have been migrated. No operator action is required for this release.

##### Dry-run proves the run you are about to submit

A Loop could validate, dry-run cleanly, and then fail at submission with `executed definition template manifest changed`. The compiler folded default values into the definition it stored, but compiled templates from the definition _before_ those defaults — so a persisted run carried more template keys than its own snapshot, and hydration rightly refused it. Compilation now uses one canonical definition throughout, and dry-run exercises the exact snapshot boundary a real submission uses. (#313, #317)

- Defaults are folded once at the start of compilation and used for linting, contracts, nodes, watch events, graph metadata, and child Loops alike — so omitted child `mode` values no longer appear out of nowhere during hydration.
- A snapshot must load through the production loader before its bytes or digest are returned; one that cannot round-trip can no longer reach storage.
- `compozy loop run --dry-run` and `compozy__loop_run` with `dry: true` run that same check, so a preview can no longer approve a definition that submission would reject.
- A mismatch now names the manifest kind, the exact key, and its source instead of reporting a generic failure.

Migration notes: no storage, API, CLI, or configuration contract changed. Integrity checks were not relaxed — inconsistent definitions are still rejected, just earlier and with a readable reason.

##### Restart a stopped session and keep its runtime

A stopped session used to be a dead end: the UI went read-only and the only way forward was creating a new one. Sending a normal prompt to a stopped session now restarts its agent process, reloads the retained provider history, and continues under the same session ID and transcript. The provider, model, reasoning effort, and speed you picked are stored on the session itself, so they survive a stop and a daemon restart instead of silently reverting to the default. (#307)

- The lifecycle gained a `starting` state, and a normal prompt is the only operation that moves a stopped session back toward execution. `session resume` stays attach-only, and queue, steer, interrupt, and attach do not restart a session.
- `compozy session runtime set <id>` takes `--provider`, `--model`, `--reasoning-effort`, and `--speed`, and `compozy session runtime clear <id>` drops the choice. Both fence on `--expected-revision` and report a conflict on a stale one. Agents get `compozy__session_runtime_set` and `compozy__session_runtime_clear`; extensions get `sessions/runtime/set` and `sessions/runtime/clear` under `session.write`.
- Session reads expose `runtime.selected`, `runtime.effective`, and `runtime.selection_revision`. A prompt resolves its runtime from an explicit snapshot first, then the stored selection, then the current effective values, and an already-queued prompt keeps the snapshot it was accepted with.
- The composer stays enabled for a stopped session, and closing a session window during a live turn no longer breaks the transcript view.

Migration notes: the `Use as Goal` action on settled assistant messages is removed. `/goal` is the single entry point for Goals.

##### Durable inputs for busy sessions

Queue, Steer, and Interrupt are now daemon-owned durable operations instead of client-side intent that could quietly disappear. An input is persisted before it is acknowledged, survives a refresh and a daemon restart, dispatches exactly once in FIFO order, and can be listed, edited, canceled, or promoted to steering by its entry ID from the CLI, HTTP, UDS, native tools, or the extension host. Disruptive changes are fenced against the turn you meant to change, so a stale client cannot interrupt a newer turn. (#304)

- `compozy session prompt` accepts `--queue`, `--interrupt`, and `--steer`; `compozy session input list|edit|steer|cancel` manages pending input by its persisted ID.
- The queue is readable and mutable over HTTP and UDS at `/api/workspaces/{workspace_id}/sessions/{session_id}/prompt/queue`, including per-entry replace, steer, and cancel.
- Agents get `compozy__session_inputs_list`, `compozy__session_input_replace`, `compozy__session_input_cancel`, and `compozy__session_input_promote`.
- The composer clears only after the daemon acknowledges, a failure keeps your draft, and a refresh reconstructs pending input from the daemon. Queued, steered, interrupted, canceled, accepted, and dropped markers no longer render as warnings, and an expected ACP cancellation no longer appears in the transcript as a provider failure.

Migration notes: the dedicated interrupt endpoint is removed — interrupt is now a prompt mode plus a fenced queue operation. The legacy ACP steer handler and the runtime steer source are removed, and the web client no longer mirrors the queue in local state.

##### Durable session messaging

Session prompts no longer duplicate, reorder, or disappear when an optimistic Web message settles, when a client reconnects, or after a cold reload. Every externally authored prompt now carries two durable identities — `message_id` for the rendered message and `idempotency_key` for the command execution — and both survive Web rendering, HTTP/UDS/CLI/native-tool ingress, queueing or steering, ACP dispatch, transcript projection, replay, and reload. (#288)

- Retrying the same prompt across supported transports is at-most-once when the original identities are reused: an exact retry returns the original result with `replayed: true` without re-running hooks or the provider.
- Divergent reuse of an identity returns a typed conflict, and uncertain post-dispatch recovery is reported as indeterminate instead of silently resending.
- Goal retries preserve the original result and the original HTTP status.
- Provider-originated ACP `user_message_chunk` echoes no longer appear as a second authored message, while locally authored steer events are preserved.
- The CLI, the Extension Host, and `compozy__session_prompt` expose the retry identities.

Migration notes: external prompt and steer inputs now require both `message_id` and `idempotency_key`, and Goal prompt responses use the standard wrapped prompt-result envelope.

##### Fan-out filters actually filter

A `filter` on a fan-out node was accepted at authoring time but never applied, so batching and `max_fan_out` still saw the whole candidate list. Each filter is now evaluated against the raw candidate before batching and branch limits. (#438)

- The candidate `item`, its original `index`, the fan-out alias, and outer fan-out aliases are all available during compilation and evaluation.
- Original order and candidate indexes survive filtering.
- Zero matches is a valid zero-branch materialization, not an error.
- A predicate failure routes through the existing `on_eval_error` policy instead of being silently ignored.

```yaml
- id: inspect_files
  class: control
  kind: fan-out
  collection: "{{ .nodes.changed.output.files }}"
  filter: "item.endsWith('.go')" # now decides what gets batched
  batch_size: 1
  max_fan_out: 8
```

##### Four runtime regressions around prompts, permissions, and settlement

A cluster of failures where the runtime reported the wrong thing about its own state. (#447)

- **Prompts stop replaying history** (fixes #399). Submitting a message sent the entire persisted chat transcript to the session prompt endpoint. Only the newest user message goes now, with its original message ID and retry idempotency preserved.
- **Permissions follow the live agent** (fixes #415). The observer carried a duplicate permission-mode resolver instead of the resource-backed agent catalog the daemon uses. Live snapshots are built from effective permissions and cached by runtime identity and revision, so a revision change can no longer leave stale permissions in place; a stopped session's fallback snapshot stays deliberately shallow.
- **A committed result survives a failed publish** (fixes #435). A pre-commit lease failure and a post-commit publication failure looked identical, so an already-settled run could be failed and settled a second time. A claimed run fails only when completion returned no committed run.
- **A crashed process is reported as crashed** (fixes #436). A non-zero exit code or a signal is now classified as a process failure and maps to the process-exited stop cause. A clean exit with code `0` after a transport failure stays on the transport path and keeps `error`, so `agent_crashed` means the subprocess actually died.

##### Idle sessions no longer warn or stop

A session whose agent had finished and was simply waiting for you used to show a large yellow "This session stops in …" notice after 30 minutes and then turn into a stopped session. Quiet supervision is now a watchdog for turns that hang, the way other agent harnesses handle it: a session waiting for you is never warned about or stopped. (fixes #782)

- `session.supervision.quiet_after` and `stop_grace` now only count silence while a turn is in flight and waiting on the agent. A turn waiting on your answer to a permission or clarify request does not count. A hung turn still warns and then stops with the `inactivity` cause, so task recovery, Loop recovery and reply wakes behave as before.
- New `session.supervision.idle_release_after` (default `30m`, `0` disables) frees the agent process of a session that has been idle that long, but only when the agent supports ACP `session/load`. The session stays active and idle in every list, and your next message reloads the same conversation. If the agent can no longer load it, CompozyOS starts a fresh agent session and replays the recent transcript.
- A released runtime records no stop and fires no `session.post_stop` hook. The process-level `agent.stopped` hook still fires, as it does when a runtime is replaced.
- The quiet-warning notice and its "Stop now" button are gone. A turn that has gone quiet now shows "quiet for 12m" on the working status line, and Stop stays where it was.

##### Keep skipped Loop branches and unresolved review findings honest

Exclusive routing no longer executes a dominated branch later in the same planning pass. Skipped downstream and fan-out steps settle as Not taken instead of appearing permanently pending after the run ends; work that ran in earlier rounds keeps its history.

Review-and-fix findings that are valid but blocked or unresolved remain pending and retain their status. Invalid findings remain invalid. Finalization no longer counts those findings as resolved simply because the review round finished.

PRs: [#529](https://github.com/compozy/compozy/pull/529), [#543](https://github.com/compozy/compozy/pull/543), [#544](https://github.com/compozy/compozy/pull/544).

##### Keep terminal output and session activity visible through lifecycle races

A short-lived terminal that exits while the browser is attaching now keeps its exited state when an older catalog response arrives late. The exit bar and retained output remain discoverable. Terminal selection controls no longer resize the process and clear the selection, hidden panes no longer publish invalid dimensions, and a completed CLI detach no longer causes an unintended reconnect.

Session stream closure drains the persisted stop marker. The pending-reply indicator also appears when a delayed React effect has already consumed its initial guard interval, instead of staying hidden indefinitely. Delayed Settings navigation and runtime-selector closing focus no longer override a newer operator action.

PRs: [#547](https://github.com/compozy/compozy/pull/547), [#557](https://github.com/compozy/compozy/pull/557). CI follow-ups: [862e138](https://github.com/compozy/compozy/commit/862e138113f438e532777421ae3b85343162360e), [52d2c4a](https://github.com/compozy/compozy/commit/52d2c4a63f8dbaa429ff61f31671f52bee8a518b).

##### Preserve terminal cleanup ownership when canceling a lane

Canceling a run-agent lane now records terminal session cleanup before generic cancellation
cleanup, preserving the exact session and binding epoch throughout settlement.

##### Live changelog and composer fixes

The changelog on `compozy.com` now reads published releases directly from GitHub at request time instead of depending on a bot pushing a generated page back into `main` after every release. Each release gets its own page with rendered Markdown, category sections, evidence, compare links, and downloadable assets, plus an RSS feed at `/changelog/feed.xml`, and releases now appear in site search, the sitemap, and the text feeds that agents read. (#292)

- Typing in the session composer no longer swallows spaces.
- A window-manager WebSocket upgrade that fails for a missing workspace now returns a proper preflight error frame, and the web client refreshes a stale workspace list when it sees that error instead of staying stuck.

Migration notes: the release workflow no longer publishes a site changelog receipt commit, and the generator scripts behind it are removed.

##### Loop runs keep their lineage, permissions, and results

Three Loop defects that broke supervision of long runs are fixed. (#420, #407, #408)

- Loop-owned Goal sessions retain the trusted provenance of the session that started the current or nearest ancestor Loop Run, so session catalogs and the Web group Goal work below its originating session. The relationship stays informational: Goal sessions remain `type=system` with no inherited TTL, auto-stop, spawn budget, or permission narrowing, provenance is derived server-side within the same workspace, and spawn limits still count only contiguous spawned ancestry. (#420)
- Daemon-owned terminal tool effects are authorized correctly. The native policy path treated the synthetic `loop-effect` audit label as an authored workspace agent and failed the lookup before the declared tool could run. The trusted daemon actor kind is now preserved through policy resolution, the label survives for attribution, and workspace policy still denies foreign targets. (#407)
- Terminal effect results stay visible in run details. The Web hook closed its event stream as soon as it received the terminal status, so a later retained or live effect-results frame never reached the run timeline, and reloading repeated the race. Successful and denied effect results now arrive in order while replacement, deactivation, navigation, and unmount keep the normal cleanup path. (#408)

##### Loop runs you can debug from the run page

A Loop run that failed used to be a dead end: every attempt died with "The agent output did not satisfy the action output schema", the node was quarantined, "Open quarantine entry" opened an empty sheet, "Open session" returned 404, the cell task sat in "Queued · attempt 1 of 10" forever, and Usage confidently reported `0 / ~$0.00`. The agent had actually answered correctly every time — the daemon joined streamed text fragments with a newline, which landed inside a JSON string and corrupted a valid reply. That joiner is fixed, and so is every surface that made the failure impossible to read. (#324)

- The agent now sees the authored `output_schema` in its prompt instead of prose that never said "JSON", extraction validates every candidate object newest-first (a quoted `package.json` no longer shadows the real answer), and the failure cause carries the underlying detail instead of one generic sentence.
- Quarantine is routed to the node that actually failed. Parked consumers collapse into a single row — "**execute task is quarantined** — collect, review, verify and approve are parked behind it until it is requeued" — with one button that opens the producer's entry, and `node_attention_flagged` is finally emitted when a run parks.
- Loop cells no longer stall in `ready` after a failed run: quarantine parks them as needs-attention and requeueing clears the park. The misleading `of 10` attempt ceiling is gone, since the Loop owns the retry budget.
- Daemon-claimed runs stop writing placeholder session ids, the real ACP session is bound to the lease under a claim token, and run detail exposes `generations[].outputs[].session_id` — so "Open session" works from the hero and from every node row that has one.
- The task list nests Loop cells under their coordinator with an escalation-first summary ("9 subtasks · 1 needs attention · 2 running") and readable identities like `g2.execute_task` instead of `loop.lo`.
- When a provider reports no tokens, Usage now reads "not reported" and "—" instead of a confident zero; the cost estimate returns only when tokens exist.

Migration notes: adds the `attention_producer_node_id` column to `loop_node_controls` through migration `00055`; run-detail payloads gain `node_controls[].attention_producer_node_id` and `generations[].outputs[].session_id`.

##### Managed run-agent workers keep their lineage and let go

Two lifecycle bugs in Loop `run-agent` actions, both reproduced against `v0.3.0-beta.18`: a managed worker lost the trail back to the session that started it, and a worker could outlive the Loop cell it belonged to. (#446, fixes #444 and #445)

- A managed worker now records the nearest originating session as informational parent lineage — parent and root are readable from the session — without borrowing or hijacking that origin session.
- When a Loop cell settles successfully, the run-owned worker binding closes and durable terminal cleanup is enqueued in the same atomic step. Cancellation and terminal failure follow the same path, and cleanup cannot run twice.
- A retryable output failure keeps the same worker session active instead of orphaning it, so a retry reuses the worker and only terminal settlement ends it.
- No public API, schema, migration, or config key changed; existing Loop and session reads simply expose corrected stored state.

##### Skills load through the native seam inside managed sessions

Managed sessions load installed skills through the native `compozy__skill_view` tool only — including skills that are not listed in the prompt catalog. The earlier attempt to give managed agents a private CLI socket is removed rather than kept as a fallback: provider code runs as the daemon user, so environment values, headers, process ancestry, and file modes cannot tell those requests apart from an operator's. (#314, #323)

- If session policy denies the native tool, the agent reports the skill unavailable instead of shelling out or reading skill files directly.
- Every `compozy skill` verb detects managed-session markers before doing any client, socket, registry, or filesystem work and points the caller at `compozy__skill_list`, `compozy__skill_search`, and `compozy__skill_view`. This is documented as a support guard, not an authorization boundary — same-user code can still clear those markers.
- Hosted-MCP bind windows now start after ACP initialization and immediately before session negotiation. A cold provider launch that takes longer than the bind window no longer expires the tool seam before the agent can use it; a bind attempted before activation still fails closed.

Migration notes: the managed CLI transport is deleted — the socket, `COMPOZY_AGENT_TRANSPORT_SOCKET`, the managed identity headers, and the managed skill API scope. Operator CLI behavior from a normal shell is unchanged.

##### Memory and dreaming require explicit opt-in

CompozyOS is a control plane, so it no longer starts persistent memory extraction or background
dreaming from omitted settings. `memory.enabled` and `roles.dream.enabled` now default to `false`.
To enable memory, set `memory.enabled = true` in `config.toml` and restart the daemon. Dreaming
requires the separate `roles.dream.enabled = true` opt-in; enabling memory alone leaves it off.

The enabled extractor now requests an explicit no-candidate result, accepts conventional empty
responses and JSONL fences, and preserves valid candidates from mixed output. Malformed lines
remain diagnosable failures instead of silently disappearing. Extraction failures and timeouts
appear in the configured DLQ and extractor failure listing. Child-stop details distinguish failed
or timed-out extraction from a parsed child response; successful inbox production owns the
`memory.extractor.completed` event.

Session pressure compaction remains independently controlled by `session.compaction.enabled`.
It reuses checkpoint coverage and may launch a summary child when an active session reaches the
pressure threshold, even with persistent memory disabled. Idle sessions and session-end memory
updates do not start that work. An explicit checkpoint-role opt-out or a failed summary leaves
uncovered events unarchived.

### Migration notes

No configuration is rewritten and no stored memory is deleted. Existing explicit `true` or `false`
values retain their meaning; omitted values use the new disabled defaults. Profile and workspace
role overrides keep their precedence under the daemon memory master switch. Existing public keys,
tools, routes, and response shapes remain available. Raw extraction failures require a new
extraction; replay remains limited to normalized candidate inbox failures. (#561)

##### One owner per Loop run, and cancellation that sticks

Loop action runs now have exactly one daemon-owned worker, cancellation survives a restart, and a session that needs CompozyOS tools fails before the provider starts instead of running without them. Fresh CompozyOS homes also start with the bundled `dev-cycle` extension already enabled, while a home that has been booted before keeps whatever you chose. (#321, #322, #326)

- Coordinators and ordinary task-role sessions can no longer activate or bootstrap a run that the dedicated `loop-action` executor already owns.
- When the effective agent or lineage policy requires concrete tools and hosted MCP cannot provide them, session startup fails closed with `ErrHostedMCPUnavailable` before the provider process is launched.
- Loop cancellation is durable: delivery state is persisted, delivery is idempotent, the run advances to draining once acknowledged, and anything still pending is retried from daemon boot and from scheduler sweeps — no restart required to converge.
- Resuming a stopped session discards the stopped ledger projection first and restores it if provider startup or the clear rolls back, so forensic projections stop conflicting and the full history is rematerialized on the next stop.
- Enablement of a bundled extension is a fresh-home default, not an override: generic local and marketplace installs stay disabled by default, and stored state survives restart and update.

##### One profile choice for every workspace, and agents keep their profile

Profiles are shared by all workspaces, and now the selected profile is too. Switching workspaces in the Web app keeps the profile you picked, and choosing another one in any workspace changes it everywhere. **All profiles** also stays selected when you switch workspaces or reload. While **All profiles** is selected, new work is created in the last profile you picked, which the destination chip shows.

Sessions an agent creates now land in that agent's profile. Before, a session created with `compozy__session_create` from an agent in another profile went to `default` and was only visible under **All profiles**. Child sessions from `compozy__session_spawn`, `compozy__subagent_delegate`, and `compozy session new` run inside a session already stayed in the caller's profile.

- `compozy profile use` and `compozy profile create` set the choice for every workspace. `compozy profile current` reports `remembered choice` without naming a workspace.
- Settings → Profiles no longer shows **Where each profile is active**, because there is now one choice.
- Choices remembered per workspace by earlier releases are deprecated. Until v0.5.0, a CLI command run inside such a workspace still uses its choice and prints a warning, and `PUT /api/profiles/selection` with `scope: "workspace"` returns a `Warning` header. Both stop applying in v0.5.0.

Migration notes: upgrading keeps every per-workspace choice and seeds the shared choice from the one you set most recently, so the Web app opens in the profile you used last. Picking a profile with `compozy profile use`, `compozy profile create`, or the Web switcher sets the shared choice and clears the per-workspace ones. Scripts that write `scope: "workspace"` selections should send `scope: "global"` before v0.5.0.

##### The idle-session setting now does what it says

Settings → General had an "End idle sessions after" control that did nothing: it wrote `session.limits.timeout`, which no session enforced, so no session ever ended. The row is now **Pause idle agents after**. It controls the idle runtime release added for #782: when a session has been idle that long, CompozyOS frees its agent process. The session stays open, and your next message picks up where it left off. Agents that can't resume a conversation keep running.

- The choices are Never, 15 minutes, 30 minutes (the default), 1 hour, 4 hours, and 1 day. A different value set in `config.toml` is shown as it is. Changing it requires a daemon restart, like other `session.*` keys.
- The general settings HTTP/UDS DTO gains `config.idle_release_after`, which maps to `session.supervision.idle_release_after`. Leave it out of an update to keep the current value.
- `session.limits.timeout` and the DTO field `config.session_timeout` are deprecated and still enforce nothing. Both stay readable and writable in this release, with the same meaning as before. A non-zero `session.limits.timeout` logs a deprecation warning when config loads, and a PATCH that sends a non-zero `session_timeout` returns a `Warning` header naming `idle_release_after`. Both are removed in v0.5.0.

Migration notes: if you set `session.limits.timeout` to limit how long sessions run, it never had an effect. Remove it, and use `session.supervision.idle_release_after` to free idle agents. Scripts that PATCH `/api/settings/general` should send `idle_release_after` before v0.5.0 removes `session_timeout`.

##### Pi providers receive the secret, not its variable name

CompozyOS wrote a Pi credential slot's target environment name, such as `ZAI_API_KEY`, straight into the session `models.json` `apiKey` field. Pi reads a bare uppercase value as a literal API key, so the provider received the variable name instead of the secret, the upstream request failed, and the session could finish without an assistant message. The Pi runtime now writes `$ZAI_API_KEY`-style references, which Pi resolves from the secret CompozyOS already injects into the provider process. (#404)

Migration notes: this covers the built-in `pi_acp` bound-secret providers — z.ai, OpenRouter, Moonshot/Kimi, xAI, MiniMax, Mistral, Groq, and Vercel AI Gateway.

##### Recover profile entry during daemon reconnection

Fixed a Web route error when daemon reconnection replaces an in-flight profile-selection read. Navigation now waits for the current workspace's profile before loading scoped work, including when the workspace changes during reconnection.

##### Keep profile recovery guidance visible during retries

Profile recovery details now remain visible while the browser retries its window-manager registration. A successful registration or an explicit context switch clears the previous failure.

##### Recover a Loop-owned task run without losing its place

A Loop worker task run parked in `needs_attention` had no honest way back. Generic subprocess-health escalation swallowed Loop-owned crashes, and `task run recover` re-enqueued a run that no longer belonged to its Loop. (#447, fixes #437)

- A confirmed agent crash inside a Loop-owned task run stays out of the generic escalation path and projects into the Loop's own node control and event model as worker attention.
- `task run recover` now fails the parked source run, creates and links a child run, and rebinds it to the exact same Loop node and item with the next attempt and epoch — all atomically. Workspace, runtime selection, designation, worktree, network, capabilities, and metadata stay attached to that cell, and attention plus death-streak state is cleared.
- Recovery diagnostics point where they should: a run that needs attention names `task run recover`, while an active run names cancellation.
- No schema, migration, or config key changed — recovery reuses the existing `wait_intervention` attention flag and the existing Loop event vocabulary.

```bash
compozy task run recover <run-id> --reason "operator recovery" -o json
```

##### Recover Loop runs without losing results or reporting an old failure as current

Carried external results with identical descriptors are deduplicated, fixing task-result reads and daemon startup during recovery while retaining corruption checks for conflicting content. Current-generation activity and blockers no longer inherit an earlier generation's failure or quarantine.

Reruns respect immutable generation lineage and allocate fresh Goal binding epochs. Completed generation outputs reject stale nonterminal overwrites, and new rounds emit one generation-start event. Built-in command judges use the daemon-matched executable and environment.

The run page leads with outcomes and action results. It previews a few outputs, keeps Details available, folds quiet control and skipped steps, and shows terminal retry guidance only when the existing planner accepts that recovery.

PRs: [#547](https://github.com/compozy/compozy/pull/547), [#554](https://github.com/compozy/compozy/pull/554).

##### Reliable session stops on slower disks

Stopping or deleting a session now completes reliably when the disk takes longer to save its shutdown history. Shutdown deadlines stay bounded, and real persistence failures remain visible.

##### Removing the suggested Home folder no longer breaks the desktop

Onboarding seeded every daemon registration into the selectable project draft, including the internal operator-home registration that Global runs on. Removing the suggested Home folder deleted that registration and left a desktop where dock apps took focus but opened nothing. (#440)

- Onboarding now partitions project workspaces from the operator home, so that row can never be seeded, added, or deleted as a project.
- The fix is covered by the canonical onboarding suite, with three cases that fail against the previous behavior.

##### Report provider failures clearly and clear stale settings when switching providers

Loop actions preserve quota, authentication, transport, timeout, and refusal failures before validating the model's output. An expired login or usage limit is no longer disguised as an invalid JSON result; genuine malformed model output still fails schema validation.

Changing an Agent's provider with `compozy agent update` clears the previous provider's model, command, reasoning effort, and ACP options unless you explicitly supply replacements. Providers that do not support reasoning configuration reject it during resolution instead of failing later at session startup.

PRs: [#545](https://github.com/compozy/compozy/pull/545), [#546](https://github.com/compozy/compozy/pull/546).

##### Resource-only extension manifests retain their declared paths

Extension manifests using the published string-array syntax for skills, agents, Loops, automation,
and layouts load again in TOML and JSON. The decoder translates each string into an unplaced path;
explicit profile placements and strict unknown-field validation remain intact. Build output uses
the current path/profile object form.

Migration notes: string-array input is accepted through v0.3.0-beta.30 and removed in
v0.3.0-beta.31. In handwritten sources, replace `agents = ["agents"]` with
`agents = [{ path = "agents" }]`, or `[[resources.agents]]` followed by `path = "agents"`.
Apply the same conversion to the other static resource families. Omit `profile` to preserve
all-profile visibility. Existing source files and installed resource state are not rewritten.

##### Resource-only extensions need no toolchain

An extension that ships only declared resources — agents, skills, Loops, automations, layouts — can now use `build`, `dev`, `reload`, and `dev --watch` without installing a Go or TypeScript toolchain. The passive build path validates and publishes those resources without running build or describe subprocesses, and active development links project them into the linked workspace while preserving deterministic generations, atomic reload, and last-good fallback. The Go and TypeScript paths are unchanged, and the resource-only path fails closed. (#423)

##### Previous-release sessions remain readable after upgrade

Sessions created by 0.3.0-beta.21 remain readable after upgrading. The persisted
creation-profile codec accepts versions 4 and 5, validates their fields, and keeps
historical profile references, policy digests, and creation digests unchanged.
New runtime profiles continue to use version 5.

Unreadable session metadata now produces a bounded scan summary with at most five
examples. An unchanged set repeats at most once every five minutes per scanner;
changed failures are reported immediately. `compozy doctor --only
runtime.session_metadata -o json` reports live counts and a bounded sample,
including missing or invalid catalog creation witnesses.

Migration notes: the version 4 to 5 change added optional ACP selections. Version
4 profiles upgrade through the persistence codec with no selections, preserving
their original bytes and hashes instead of rewriting immutable witnesses. No SQL
migration, config change, session deletion, or manual reset is required. Unknown
versions and corrupted witnesses remain rejected and unchanged. This does not
change dreaming defaults or claim to reproduce the reported CPU/disk measurements.

##### Run the Linux AppImage without installing libfuse2

Linux AppImages now embed the static runtime, allowing distributions with FUSE 3 to launch the app without installing `libfuse2`. Release, local, and update-test packaging share the same runtime selection. The extraction fallback remains available for environments without kernel FUSE support.

PR: [#548](https://github.com/compozy/compozy/pull/548).

##### Keep shutdown history moving during database maintenance

Background database maintenance now yields when a protected database open is busy, allowing queued shutdown history to be saved without waiting for maintenance to finish. Deferred maintenance retries automatically, and real write failures remain visible.

##### Preserve successful SQLite commits during cancellation

A request or persistence deadline arriving during a successful SQLite commit no longer reports that committed write as failed. This prevents persistence retries from duplicating completed tool events. Cancellation observed before commit still rolls back the write.

##### Keep palette actions open during refresh

The command palette keeps the selected row and its actions open when recent or pinned commands refresh at the same moment you open the action panel.

##### The daemon owns a managed worker's outcome

A managed Loop worker session could call complete or fail and race the daemon's own validated action result, so what a generation recorded depended on which side got there first. The daemon is now the single terminal authority for managed workers. (#438)

- A worker session may heartbeat while it holds the lease, but terminal settlement calls are denied by session lineage.
- A generation settles as succeeded only with schema-valid structured output; an invalid capture terminates as `invalid_output` instead of passing.
- The exact validated object round-trips through inline and content-addressed storage, including downstream template and CEL hydration, so a large output no longer loses required fields.

##### The desktop stays responsive with a large session catalog

Long-running sessions with a large internal session catalog put the macOS desktop into a request-and-reload feedback loop that made it unusable. The two-second liveness probe now targets a bounded `GET /api/status/identity` surface over HTTP and UDS instead of the full status aggregate, and internal sessions stop inflating the public catalog. (#414)

- Memory-extractor, auto-title, and dream sessions no longer publish wake events to the public session catalog.
- Built-in background agents, including `dreaming-curator`, resolve through effective workspace configuration instead of being reported as missing workspace-authored agents.
- The identity contract ships in OpenAPI and the generated TypeScript types.

##### Restore saved windows after layout configuration is ready

Fixed desktop restoration briefly mounting an empty composer before saved window layout configuration was ready.

##### Zoom a window without covering its tiled neighbors

Zoom now moves a window or its entire tab frame to a separate regular desktop when needed, preserving the other windows. Unzoom returns the unit through its saved layout anchors and removes an empty desktop created for zoom. Tiling another window ends zoom and gives the new neighbor its own space.

Persisted version 3 layouts migrate losslessly to the current layout shape, preserving tab order and the active tab. Layout streams now use heartbeats, stalled-stream recovery, and authoritative reconnect fences so the browser catches up after a disconnect.

Moving one window no longer forces unchanged windows to render again, reducing work during layout updates from another browser or the CLI. The existing 12-window drag, restore, and peer-convergence performance limits remain enforced.

PR: [#525](https://github.com/compozy/compozy/pull/525).

#### Highlights

##### A calmer desktop shell with light and dark themes

The CompozyOS desktop is redesigned around flat chrome and edge-to-edge windows, and it now comes in
light and dark.

- The dock moves to a slim rail on the left. Its foot holds your profile, the theme toggle, and
  Settings. Below 960px it becomes a bottom tab bar with the same controls.
- The top bar runs the full width. Desktop dots and an **All desktops** button sit in its tray, next
  to the bell and the command palette.
- Windows tile edge to edge with no gaps. A single hairline separates them: drag it, or focus it and
  use the arrow keys (hold Shift for bigger steps), to resize the panes on both sides.
- New windows open as a tab in the window you are working in instead of floating on top of it; on
  an empty desktop they fill it. Option-click a rail icon to open it split beside your window,
  Shift-click to open it on a new desktop, or right-click it for every destination, including a
  floating window. `compozy window open --floating` and the `floating` input of
  `compozy__window_open` ask for a floating window explicitly.
- **Window › Arrange** offers Main and stack, Columns, Grid, and Balance sizes, and **Window › Move
  window to** sends a window to another desktop by name. `compozy layout arrange` and the
  `compozy__layout_arrange` tool accept the new `main_stack` arrangement, and an optional
  `--keep-frames` / `keep_frames` that arranges each named tab deck whole. Without it, every named
  window is still its own participant.
- Tabs look and behave like browser tabs, and window controls are quiet icons at the end of the head.
- An empty desktop asks "What should we work on?" above the session composer. Type a prompt and press
  Enter to start a session right there, with your project's default agent already picked.
- Sessions use the full width of their window again, tool calls read as quiet rows, and the list of
  sessions is open by default.
- Choose **Light**, **Dark**, or **System** in Settings › Appearance, or flip the theme from the dock.
  Dark stays the default. The choice is saved in this browser and applied before the first frame,
  and the terminal follows it.
- The interface uses Inter for text and Geist Mono for code, with higher-contrast secondary text in
  both themes.

### Migration

Three `[window_manager]` defaults change. Your saved layouts, profiles, and any value you set
explicitly in `config.toml` are kept as they are; only unset values pick up the new defaults.

| Setting                                            | Old default                   | New default |
| -------------------------------------------------- | ----------------------------- | ----------- |
| `gaps.inner` / `top` / `right` / `bottom` / `left` | `8` / `8` / `10` / `8` / `10` | `0`         |
| `new_window_policy`                                | `floating`                    | `tab`       |
| `bindings.bottom_center`                           | `reserved`                    | `zoom`      |

`new_window_policy` now accepts `tab` (the new default) and `beside_focus` (tile new windows beside
the focused one) alongside `floating`. To keep the previous behavior, set the old values explicitly:

```toml
[window_manager]
new_window_policy = "floating"

[window_manager.gaps]
inner = 8
top = 8
right = 10
bottom = 8
left = 10

[window_manager.bindings]
bottom_center = "reserved"
```

Or apply them from the CLI, for example
`compozy config set window_manager.new_window_policy floating`. Changes apply live. The theme is a
per-browser preference, not a `config.toml` key, so there is nothing to migrate for it.

##### Gateway docs: zero to GitHub webhooks

compozy.com gains a dedicated Gateway section written for first-time operators: a ten-minute
quickstart from `gateway.enabled` to a paired phone, a step-by-step "Receive GitHub webhooks"
tutorial verified end to end — including why a native repository webhook cannot sign CompozyOS's
generic trigger contract and the GitHub Actions workflow that can — a Tailscale extension page
covering tailnet prerequisites through clean removal, a remote CLI/SSH/public-access guide, a
devices-audit-teardown runbook, and a plain-language security page. (#331)

Migration notes: `/docs/operations/remote-gateway`, `/docs/operations/gateway-threat-model`, and
`/docs/configuration/gateway` moved into `/docs/gateway/*` as a hard cut — update saved links.

##### MCP catalog, session runtime, and extension management

CompozyOS beta expands how people and agents configure the runtime across MCP, sessions, extensions, workspace boundaries, and the session UI.

- Install, authorize, repair, inspect, and remove curated MCP servers through the CLI, HTTP/UDS APIs, Web, and the official CompozyOS skill. The catalog now uses manifest version 2, the runtime uses the official MCP SDK, and public MCP transport no longer accepts SSE. (#284)
- Choose the provider, model, reasoning effort, and speed for each session prompt, switch runtime within a session, and create sessions before their first prompt. (#283)
- Create, build, validate, develop, distribute, install, and inspect extensions through the daemon, CLI, APIs, native tools, Web, and SDK contracts. Extension manifests now use version 2. (#278)
- Apply existing session permission modes to explicitly targeted cross-workspace agent access, including session-scoped consent where a native-tool prompt is available. (#275)
- Read session transcripts through a calmer timeline with clearer tool results, failures, permissions, clarifications, and goal controls. (#271)
- Run the daemon on Windows with corrected process locking, SQLite paths, detach behavior, process timestamps, and sync-directory handling. (#274)
- Automation jobs can target Loops with workspace inputs and mappings, unresolved tool calls now fail explicitly, and Loop/session recovery paths report clearer state and errors. (#276, #279)

Migration notes: update MCP catalog manifests to version 2 and replace public SSE transport; create a session before submitting its first prompt and runtime selection; update extension manifests to version 2.

##### Spend less time repeating development and verification work

Repeated development startup and verification reuse successful evidence when source, generated artifacts, toolchain, and build settings still match. Development entry points avoid unchanged generation, and Air avoids relinking an unchanged daemon while preserving atomic publication and failed-build recovery. Explicit generation and drift checks remain available.

Web tests use the existing worker bound with a threads pool; migration fixtures reuse an empty historical prefix while still exercising real upgrades. Active lint-plugin tests are included in normal verification. The full-checkptr audit now installs its Bun dependencies and uses the existing eight-way Go partitioner without reducing instrumentation or timeouts.

Marketplace MCP configuration no longer waits for unrelated model-catalog discovery. Provider and catalog changes still reconcile normally.

PRs: [#536](https://github.com/compozy/compozy/pull/536), [#556](https://github.com/compozy/compozy/pull/556).

##### Upgrade defaults and compatibility for session control

Persisted database state and layouts upgrade through their owning migrations. Review these session defaults when upgrading:

- `session.busy_input.default_mode` is now `steer`; choose `queue` to defer follow-ups. Live injection depends on runtime capability, and unsupported runtimes report the interrupt fallback.
- New supervision settings are `session.supervision.quiet_after` (30m) and `stop_grace` (10m). Existing `inactivity_warning_after` and `inactivity_timeout` settings retain their legacy timer behavior during the v0.4 compatibility window. For ordinary positive thresholds, migrate the warning threshold to `quiet_after` and the difference between stop and warning thresholds to `stop_grace`; check the documented zero semantics before changing disabled timers.
- `network.live.max_total_wall_time` defaults to `0`, disabling that aggregate wall budget. Set an explicit duration if you need a limit.
- Prefer `--expected-turn` over the deprecated `--expected-turn-id`. The old flag and the historical `interrupt` default remain accepted with warnings until their planned removal in v0.5.0; explicit per-send interruption remains supported.
- `compozy__session_prompt` defaults to `wait: false`. Request `wait: true` explicitly when you need a synchronous native-tool call.

PRs: [#525](https://github.com/compozy/compozy/pull/525), [#555](https://github.com/compozy/compozy/pull/555), [#557](https://github.com/compozy/compozy/pull/557).

## Unreleased

### Breaking & migrations

The Marketplace catalog moves to one extension catalog. Global migration `00111`
preserves existing extension catalog rows under `compozy-catalog` and removes
only the cached MCP and skill listings (ADR-001). Installed extensions, manual
MCP server configuration, and installed skill files remain intact. Provenance
receives `catalog:compozy` origin only when an existing marketplace installation
records a non-empty catalog entry ID; other installations stay unclassified.
The migration also creates storage for scoped extension inputs and MCP overrides.
Migration `00112` adds input identity and activity to extension secret bindings.
Existing bindings remain active with their references, scopes, timestamps, and
header mappings preserved. Removing an input from a manifest deactivates its
stored value; reintroducing the input can reactivate that value.
Migration `00113` includes the credential owner in both MCP OAuth token and
client-registration keys. Existing rows become `manual`; their token references,
registration metadata, and encrypted vault values remain unchanged. Extension
credentials use a separate `vault:mcp/ext/<extension>/` namespace.
Migration `00114` separates stored extension packages from their profile/workspace
installation attachments. Existing packages receive one global, all-profiles
attachment with their original installation timestamp; enablement exceptions,
package files, provenance, and credentials are preserved.
Migration `00115` removes the fixed extension discriminator from catalog rows;
source/entry identity and all catalog content and installed extension state are preserved.

The Marketplace switches directly to the extension catalog contract. Retired kind routes,
remote skill acquisition and MCP catalog installation are removed without aliases.
Browse uses `GET /api/marketplace`; detail uses `/api/marketplace/entries/{entry_id}`.
Refresh accepts no `kind` selector and reports `sources[]` outcomes.
Existing extension packages, acquisition references, installed skills and manual MCPs remain.
CLI callers remove `--kind` and the first kind argument to `marketplace info`.
`compozy mcp install` and remote `compozy skill search|install|update|remove` are removed;
local skill inspection and management remain. Native `compozy__marketplace_search`
rejects the removed `kind` argument. See [migration steps](MIGRATION_GUIDE.md#marketplace-acquisition-hard-cut).
MCP declarations embedded in previously installed ClawHub/Marketplace skills are now disabled.
The skill files and provenance, manual MCP configuration and credentials, and extension-provided
MCP servers remain intact. Existing extension removal still retires its instance bindings and
exclusive secrets while preserving shared or foreign secrets and compensating failed removals.
Retired `skills.marketplace.registry`, `skills.marketplace.base_url` and `skills.allowed_marketplace_mcp` settings
are archived as inactive comments in the same config file on load. The archive
and active configuration are written atomically, once; unrelated settings and
extension configuration are preserved. Settings and `config set` reject new
writes of these acquisition fields.

`tools.clarify.timeout` now defaults to no automatic expiration. An omitted key
or an explicit `timeout = "0s"` means a pending clarification waits until it is
answered, explicitly canceled, or stopped with its owning context; explicit
`1s`–`24h` values keep the existing deadline, fallback answer, and `timed_out`
outcome. Installations that want the previous behavior set `timeout = "5m"`
explicitly and restart the daemon (the key keeps its TOML duration shape, user
scope, and restart-required lifecycle). An unbounded pending clarification
reports `deadline: null` on every surface, and the daemon holds the agent's
blocked call open with `_compozy/clarify_ping` keepalive notifications every
30s until the request resolves. Out-of-range durations are rejected
with `tools.clarify.timeout must be between 1s and 24h, or 0s for no
expiration: <value>`; malformed durations fail at TOML parse. The last valid
policy stays in both cases.

### Features

- Client plugins with manifests under `.claude-plugin/`, `.codex-plugin/` or
  `.cursor-plugin/` load through the extension lifecycle. Supported skills and
  MCPs are adapted without rewriting packages; ignored client commands, agents
  and hooks produce diagnostics. Installed provenance records the actual layout.

- Extension MCP runtime names are allocated automatically and remain stable.
  Install requests no longer accept `--runtime-name` or `runtime_name`; failed
  installs restore prior allocations. Extension inputs accept secrets owned by
  the same extension instance, without importing manual MCP vault references.

## 0.0.9 - 2026-07-04

### ♻️ Refactoring

- Tasks orchestration (#269)

### 🐛 Bug Fixes

- Network optimizations (#263)
- Native tools (#266)
- Align release-gated network integration tests with #263 behavior

### Release Notes

#### Features

##### Network delivery policies, subscriptions, and thread-to-task promotion

CompozyOS network channels gain durable delivery control. Each channel now carries a fan-out policy — deliver to all members, route to a designated coordinator peer, or match by capability (the default) — managed with `compozy network channels create` and `compozy network channels update`. Peers and tasks can subscribe to channels: inspect subscriptions with `compozy network subscriptions` and manage per-task subscriptions with `compozy task subscribe <task-id>`. Executable thread messages can be promoted into durable workspace tasks straight from the CLI with `compozy network promote`, or by an agent through the `compozy__task_promote_from_thread` native tool, and designated sibling task runs can be fanned out as part of a workflow. Network and task views now surface task links, peer and coordination cost, and delivered size and token metrics, while mentions are normalized (trimmed, empties removed) and size and token limits are enforced as non-negative.

##### Task blocking, recovery, and needs-attention triage

Tasks are now first-class to block, triage, and recover. A task can be explicitly blocked and unblocked with `compozy task block <id>` and `compozy task unblock <id>`, and every block is kept as durable history you can inspect with `compozy task blocks <id>`. A new `needs_attention` status surfaces tasks that stalled and need a human or coordinator to step in — task details now carry the blocked reason and the wake creator so it is clear why a task is waiting and who nudged it. Clear the state with `compozy task recover <id>`, recover a specific stalled run with `compozy task run recover <run-id>`, or let an agent do it through the `compozy__task_recover` native tool and the HTTP/UDS API. Task completion now returns the IDs of any tasks it created, and stronger validation rejects invalid created-task references before they are persisted. Claim tokens are redacted across task outputs and hook payloads, and the new `needs_attention_after` setting controls how long a task may wait before it is flagged for attention.

#### Fixes

##### Clearer native tool errors and availability diagnostics

Native and hosted tool calls now fail legibly. Hosted MCP tool calls return richer, structured JSON error details — the tool ID, an error code, and any denial reasons — instead of opaque failures, and advertised tools carry improved metadata and descriptions. When runtime diagnostic data cannot be retrieved, native tool lookups report a clear "unavailable" diagnostic rather than a silent miss. Hosted and session MCP availability handling is safer overall, with clearer informational and warning logs and graceful fallbacks when a feature is disabled. Documentation and the runtime envelope now consistently direct agents to the canonical `compozy__*` tool IDs and the harness-returned tool references.

## 0.0.8 - 2026-06-22

### 🐛 Bug Fixes

- Acp update and general fixes (#256)
- Make docs markdown copy retryable

### 🧪 Testing

- Remove not needed test

## 0.0.7 - 2026-06-04

### ♻️ Refactoring

- Improvements on create modals (#252)

### 🐛 Bug Fixes

- Repair daemon-served e2e flows

## 0.0.6 - 2026-06-03

### 🎉 Features

- Dependency-driven auto-enqueue (opt-in) (#232)

### 🐛 Bug Fixes

- Wake coordinator sessions reliably (#240)
- Enable runtime evidence profiles (#242)
- React doctor fixes (#245)
- Verify marketplace skill installs (#244)
- Persist active workspace and redirect on session/workspace mismatch (#238)
- Safe workspace delete and compozy session remove command (#239)
- Unblock release CI (bootstrapRun complexity, stale gate tests) (#249)

### Release Notes

#### Features

##### Dependency-driven auto-enqueue

Tasks can now opt into dependency-driven auto-enqueue. When a task is marked `auto_enqueue_on_ready`, CompozyOS enqueues its next task run automatically as soon as a blocking dependency completes and the task reconciles to `ready` — so a dependency graph advances without a manual `compozy task enqueue` at each step. The behavior is conservative: only a successful completion satisfies a `blocks` edge, paused dependents are skipped, and the queued-run reservation keeps at most one open run per dependent under concurrent or retried completions. Enqueue happens only after the completion has durably committed and is best-effort — a failed enqueue is logged, never rolled back onto the completing run. It is off by default; enable it per task with `--auto-enqueue-on-ready` on `compozy task create` / `compozy task child create`, toggle it with `compozy task update`, or set the `auto_enqueue_on_ready` field over HTTP/UDS and the extension SDK.

##### Runtime evidence mode for task execution profiles

Task execution profiles can now drive worker startup and opt into runtime evidence mode. A task with no pool owner but a `worker.mode = "select"` profile now starts the selected agent, provider, and model and propagates the profile's required capabilities into its claim command. Setting `runtime.mode = "evidence"` boots that worker with guidance to run browser, simulator, and local-app validation and to capture runtime evidence. CompozyOS elevates the session to auto-approve permissions only when the profile also pins an explicit sandbox reference (`sandbox.mode = "ref"`); otherwise the configured permission policy stays in force, and evidence mode grants no extra task authority. The profile's new `runtime` block is surfaced through the execution-profile CLI, the HTTP/UDS endpoints, and the native task tools.

#### Fixes

##### Coordinator sessions wake reliably on new work

Coordinator sessions now wake reliably when new work arrives. When a task run is enqueued for a workspace that already has a running coordinator session, CompozyOS delivers a synthetic wake to that session — interrupting the agent's current turn if it is idle and waiting — so queued runs are picked up promptly instead of stalling. The same interrupt-if-waiting delivery now applies to harness re-entry and heartbeat wakes, force-retried and force-recovered runs re-trigger a coordinator wake, and heartbeat wakes skipped for non-transient reasons are recorded as dropped rather than silently lost. Wake delivery is de-duplicated per session and run and drained safely across daemon shutdown.

##### Marketplace skill installs are verified end-to-end

Marketplace skill installs are now verified end-to-end. After downloading and writing a skill, CompozyOS confirms the runtime can actually discover it as an enabled marketplace skill with matching provenance, instead of reporting success for an install that would never resolve. Installs that are disabled, shadowed by a higher-precedence skill of the same name, missing provenance, or resolved to a different slug now fail with a clear, terminal error and remediation guidance across `compozy skill install` and the daemon API. Use `compozy skill where <name>` to inspect the winning source before retrying.

##### Safe workspace deletion, plus compozy session remove and compozy open

Workspace deletion is now safe: CompozyOS refuses to delete a workspace while any of its sessions are still active — returning a 409 that names the blocking sessions — and cleans up the workspace's stopped session history transactionally when deletion proceeds. Two agent-manageable CLI commands ship alongside it: `compozy session remove <id>` deletes a single session and its persisted history, and `compozy open` opens the CompozyOS web UI in your default browser. The CLI reference also gains documentation for `compozy open`, `compozy session remove`, and the existing `compozy onboarding` command group.

##### Web UI remembers your active workspace

The web UI now remembers your active workspace across reloads and browser restarts, so a refresh no longer snaps you back to the first workspace. Direct and shared session links resolve a session's owning workspace from its ID and load reliably regardless of which workspace is selected, and the UI redirects to the agent page when an opened session belongs to a different workspace than the active one.

## 0.0.5 - 2026-05-29

### ♻️ Refactoring

- Optimize prompt consumption (#222)
- Orchestration improvements (#230)

### 🎉 Features

- Decouple worker concurrency from coordinator uniqueness (#229)

### 🐛 Bug Fixes

- Handle provider overlay subtables (#228)

## 0.0.4 - 2026-05-27

### 🐛 Bug Fixes

- Default workspace

## 0.0.3 - 2026-05-27

### ♻️ Refactoring

- Memory optimization (#215)

## 0.0.2 - 2026-05-27

### Other Changes

- Lessons learned

### ♻️ Refactoring

- Project structure (#7)
- Kb improvements (#12)
- Rename spaces to channels (#17)
- Add extensions gaps (#21)
- Improve tool calls ui (#22)
- Remove web app header
- Module improvements (#29)
- Memory improvements (#35)
- Storybook for web and ui (#38)
- Enable CompozyOS network by default for new installs (#57)
- Hermes adjustments (#69)
- Badges design (#84)
- Storybook scenario and logos gallery
- Migrate typescript tests (#114)
- Internal go packages (#120)
- Ui patterns (#127)
- Improve e2e tests (#130)
- Ui redesign
- Workspace isolation across runtime surfaces (#145)
- Prod ready applies (#162)
- Tool card ui (#164)
- Alpha on logo
- Prod ready features (#167)
- Thread sheet (#202)

### 🎉 Features

- Implement config foundation packages
- Implement sqlite store package
- Add ACP client package
- Add session lifecycle manager
- Implement observe package
- Add daemon composition root
- Add uds api server
- Implement cli package
- Add http api server
- Add system design
- Add foundation types, schemas, and layout shell for web client
- Add daemon health polling and agent sidebar systems for web client
- Add session system CRUD, streaming core, and session store for web client
- Add chat view, messages, and composer tests for web client
- Add tool cards and renderers for web client
- Add file-backed memory store core
- Scaffold memory session seams
- Add memory dream consolidation service
- Wire memory assembler into daemon
- Add memory api and cli
- New skills system (#1)
- Add workspace entity (#5)
- Add new skill capabilities (#8)
- Web ui v2 (#9)
- Improve hooks system (#10)
- Session resilience (#11)
- Add extensability (#13)
- Add automation (#16)
- Add channels (#14)
- Add network implementation (#15)
- Add network, bridges and automations web pages (#18)
- Ext registry (#20)
- Add core tasks (#19)
- Bridge adapters (#23)
- Add site (#26)
- Add ext refac and sandbox (#25)
- Settings ui (#37)
- Tasks ui (#36)
- Harness improvements (#44)
- Agent capabilities (#49)
- Redesign ui (#48)
- Unify capability (#53)
- Redesign network workspace (#59)
- Add task deletion and split session delete from stop (#58)
- Session provider selection (#60)
- Production grade adjustments (#66)
- Autonomous system (#75)
- Add agent session route (#80)
- Tools registry (#85)
- Agents soul (#88)
- Add network threads (#105)
- Orchestration improvements (#106)
- Memory v2 (#108)
- Agent categories (#113)
- Providers model (#118)
- Add canonical CompozyOS bundled skill (#143)
- Onboarding and improvements (#198)
- Onboarding and improvements (#201)

### 🐛 Bug Fixes

- Review round
- Review rounds
- Resolve memory extensibility review batch
- Embed web into daemon
- Defaults agents
- Acp integration (#4)
- Lint errors
- Prd folder
- Remove orphan web actions and dead surfaces (#55)
- Qa testing and fixes (#73)
- New review rounds (#82)
- Security audit (#90)
- Release qa round (#95)
- Add missing tools (#141)
- New qa round (#147)
- Advanced qa round (#149)
- Homebrew tap
- Final review round (#151)
- Daemon healthy
- Reasoning models (#158)
- Lint errors (#160)
- Review round (#168)
- Release adjustments (#171)
- Stabilize release ci fixtures
- Stabilize release integration gate
- Stabilize release verify gates
- Stabilize release integration flows
- Stabilize release verify gates
- Stabilize main verify shutdown
- Ignore stale acpmock cancel
- Marketplace search focus and filtering (#193)
- Website video
- Workspace command select

### 🧪 Testing

- Add e2e tests (#27)
- Qa rounds (#78)
- Improve test suite (#138)
- Harden daemon-served restart reloads
- Harden daemon-served readiness waits
- Stabilize dashboard focus assertion
- Stabilize release integration gates
- Stabilize release e2e markers
- Stabilize release e2e flows
- Improve suite speed

## 0.0.1 - 2026-05-26

### Other Changes

- Lessons learned

### ♻️ Refactoring

- Project structure (#7)
- Kb improvements (#12)
- Rename spaces to channels (#17)
- Add extensions gaps (#21)
- Improve tool calls ui (#22)
- Remove web app header
- Module improvements (#29)
- Memory improvements (#35)
- Storybook for web and ui (#38)
- Enable CompozyOS network by default for new installs (#57)
- Hermes adjustments (#69)
- Badges design (#84)
- Storybook scenario and logos gallery
- Migrate typescript tests (#114)
- Internal go packages (#120)
- Ui patterns (#127)
- Improve e2e tests (#130)
- Ui redesign
- Workspace isolation across runtime surfaces (#145)
- Prod ready applies (#162)
- Tool card ui (#164)
- Alpha on logo
- Prod ready features (#167)
- Thread sheet (#202)

### 🎉 Features

- Implement config foundation packages
- Implement sqlite store package
- Add ACP client package
- Add session lifecycle manager
- Implement observe package
- Add daemon composition root
- Add uds api server
- Implement cli package
- Add http api server
- Add system design
- Add foundation types, schemas, and layout shell for web client
- Add daemon health polling and agent sidebar systems for web client
- Add session system CRUD, streaming core, and session store for web client
- Add chat view, messages, and composer tests for web client
- Add tool cards and renderers for web client
- Add file-backed memory store core
- Scaffold memory session seams
- Add memory dream consolidation service
- Wire memory assembler into daemon
- Add memory api and cli
- New skills system (#1)
- Add workspace entity (#5)
- Add new skill capabilities (#8)
- Web ui v2 (#9)
- Improve hooks system (#10)
- Session resilience (#11)
- Add extensability (#13)
- Add automation (#16)
- Add channels (#14)
- Add network implementation (#15)
- Add network, bridges and automations web pages (#18)
- Ext registry (#20)
- Add core tasks (#19)
- Bridge adapters (#23)
- Add site (#26)
- Add ext refac and sandbox (#25)
- Settings ui (#37)
- Tasks ui (#36)
- Harness improvements (#44)
- Agent capabilities (#49)
- Redesign ui (#48)
- Unify capability (#53)
- Redesign network workspace (#59)
- Add task deletion and split session delete from stop (#58)
- Session provider selection (#60)
- Production grade adjustments (#66)
- Autonomous system (#75)
- Add agent session route (#80)
- Tools registry (#85)
- Agents soul (#88)
- Add network threads (#105)
- Orchestration improvements (#106)
- Memory v2 (#108)
- Agent categories (#113)
- Providers model (#118)
- Add canonical CompozyOS bundled skill (#143)
- Onboarding and improvements (#198)
- Onboarding and improvements (#201)

### 🐛 Bug Fixes

- Review round
- Review rounds
- Resolve memory extensibility review batch
- Embed web into daemon
- Defaults agents
- Acp integration (#4)
- Lint errors
- Prd folder
- Remove orphan web actions and dead surfaces (#55)
- Qa testing and fixes (#73)
- New review rounds (#82)
- Security audit (#90)
- Release qa round (#95)
- Add missing tools (#141)
- New qa round (#147)
- Advanced qa round (#149)
- Homebrew tap
- Final review round (#151)
- Daemon healthy
- Reasoning models (#158)
- Lint errors (#160)
- Review round (#168)
- Release adjustments (#171)
- Stabilize release ci fixtures
- Stabilize release integration gate
- Stabilize release verify gates
- Stabilize release integration flows
- Stabilize release verify gates
- Stabilize main verify shutdown
- Ignore stale acpmock cancel
- Marketplace search focus and filtering (#193)
- Website video
- Workspace command select

### 🧪 Testing

- Add e2e tests (#27)
- Qa rounds (#78)
- Improve test suite (#138)
- Harden daemon-served restart reloads
- Harden daemon-served readiness waits
- Stabilize dashboard focus assertion
- Stabilize release integration gates
- Stabilize release e2e markers
- Stabilize release e2e flows
