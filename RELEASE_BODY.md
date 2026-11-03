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
