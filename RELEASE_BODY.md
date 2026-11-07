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
