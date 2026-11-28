## 0.7.8 - 2026-11-22

### Bug Fixes

- Apply the quiet watchdog to silent ACP prompts (#1895)
- Keep session lists available when side ancestry is missing (#1901)
- Boot with unavailable workspace roots (#1905)
- Install the recorded app release while the channel feed is stale (#1904)
- Bound hosted MCP calls without caller deadlines (#1908)
- Document local sockets for stdio MCP servers (#1907)
- Accept Unicode separator schema digests (#1909)
- Exclude always-on mains from session group totals (#1911)
- Show Global always-on agents once, in the Global row (#1896)
- Show the inherited conversation in a derived child (#1910)
- Reject whitespace in pull request monitor branches (#1918)
- Keep children of deleted side chats hidden (#1919)
- Explain authorization-filtered peer lists (#1920)
- Reuse the Session window for notification jumps (#1929)
- Keep the session rail's scroll when switching sessions (#1930)
- Restore the session rail scroll once its list finishes loading (#1931)
- Return a waiting terminal window to the cap dialog when its Close is refused (#1932)
- Show usage for the current native subagent run (#1936)
- Page session list folders independently (#1935)
- Keep the loading frame's sessions rail out of reach (#1942)

### Features

- Run Google Antigravity through the official agy acp CLI (#1913)

### Performance Improvements

- Skip redundant metadata and SQL work in child-session supervision (#1893)
- Persist streamed transcript parts incrementally (#1912)
- Start desktop settings reads when the client registers (#1915)
- Bound page-cache traffic on Linux SQLite readers (#1914)
- Scope Linux busy-tick process scans (#1916)
- Drop shell-route router scroll bookkeeping and share one ancestor walk per transcript scroll (#1934)

### Refactoring

- Compose attention toasts in the components layer (#1921)

### Testing

- Leave the composer before E2E-016 presses the sidebar chord (#1894)
- Provide a profile for the peer note command (#1922)
- Assert pinned subtree session order (#1933)

### Release Notes

#### Features

##### Google Antigravity runs the official CLI

The `agy` built-in now launches the official **Antigravity CLI** as `agy acp` instead of requiring a separately installed standalone ACP server. Install the CLI with Google's instructions and confirm `agy acp --help` succeeds; an interactive `agy` sign-in is reused, so the previous server archive, its login, and its separate model catalog are no longer involved. The provider ID, display name, authentication mode, permission mode, and session records are unchanged.

Model and effort now come from the CLI's live catalog. On the verified CLI 1.5.9, seven logical models are advertised and each carries its own `thought_level` option: Hydrogen offers `low`, `medium`, `high`, and `xhigh`, while Argon offers three levels; `max` is unsupported. CompozyOS selects the model first, then applies the advertised effort and confirms the provider's acknowledgement before the first prompt. Effort is never silently lowered to a value the model does not offer. Availability depends on your Google account, CLI version, and client identity, so the observed list is not a fixed built-in catalog.

Under **Full access**, Antigravity now selects a mode its CLI actually advertises (`auto` on the verified version) instead of the previous injected `yolo`. Host permission decisions are unchanged, and no permission-bypass flag is added.

A live rewalk showed that the CLI runs its own `write_file` without asking the host even in its `plan` mode: an `approve-reads` session acknowledged `mode=plan`, `gemini-4-hydrogen`, and `xhigh`, then created a file with no permission request. Because CompozyOS cannot mediate that boundary, a native `agy acp` start is refused before any prompt under every policy below **Full access** — `deny-all`, `approve-reads`, `approve-edits`, and `auto`. The refusal never widens the policy and is never worked around by an agent raising its own level: choose **Full access** for a native Antigravity session, or keep the narrower policy on a runtime or authored route whose tool calls return through CompozyOS. An authored standalone or custom command keeps its route and its own approval behavior. For the same reason, a native `agy acp` start under **Full access** is refused when `permission_rules` contains a `deny` entry; `allow` entries are accepted because they never narrow what runs. The check recognizes the CLI under both executable names Google ships, `agy` and `antigravity`.

Antigravity sends its questions as permission requests whose options are the possible answers. CompozyOS now routes those questions to the operator as a choice prompt under every policy, including **Full access**, and cancels the request when no operator can answer instead of approving the first answer. Every Antigravity session also disables the CLI's `run_workflow` tool, which otherwise writes `skills/system/run-workflow/SKILL.md` into the workspace.

Migration notes:

- **Install and login:** install Antigravity CLI, then run `compozy provider auth login agy` and `compozy provider auth status agy --remote`. A standalone-only installation reports `missing_cli`; a standalone-server login alone reports `needs_login`. CompozyOS never copies or deletes either Google token store, and the remote probe never starts consent. For headless or SSH setup, Google's `agy acp --login` is the operator alternative and is never run as a background probe.
- **Existing overrides:** an authored provider/agent/fallback command and any explicit provider-private ACP options keep their exact bytes and precedence. An option the CLI no longer offers fails with the existing actionable validation error; CompozyOS does not silently rename or convert it. Remove an obsolete standalone command override yourself when you want the new built-in route.
- **Upgrades and continuity:** a session whose executable route changed continues the same logical session from its retained transcript through the existing replay path. This is not native-history import: Google's own history is untouched and CompozyOS does not claim provider-private context was transferred. A session without verifiable accepted-route metadata also replays rather than assuming an old native ID belongs to the CLI store. Sessions recorded by earlier releases can carry an effective provider mode without a separately recorded selection intent — for example, the old injected `yolo` — so CompozyOS cannot distinguish an automatic mode from an explicit one. It preserves that ambiguous mode instead of deleting or remapping it; if the new CLI does not offer it, the start fails with the existing unsupported-option repair guidance and keeps the session and transcript, rather than silently dropping it. To establish current intent, select the session's provider, model, and effort again with `compozy session runtime set <id> --provider agy --model <advertised-model> --reasoning-effort <offered-effort>`; that writes a complete next-prompt snapshot without the obsolete raw mode, and the same logical session then replays on a fresh native ACP session ID. `compozy session runtime clear <id>` returns to the effective/agent default and is not that confirmation. A session whose accepted route still matches an authored standalone command keeps that route; only a mismatched route replays. After the new native bind, later restarts restore that session natively when the provider supports it.
- **MCP:** the default hosted-tool transport stays stdio. If you author an HTTP MCP configuration for Antigravity, Google requires its `--process-per-session` option; CompozyOS does not enable HTTP or a different worker mode implicitly.

#### Fixes

##### The daemon boots with a missing project, stops silent turns on time, and installs updates right after a release

- **Boot with a renamed or removed project folder.** If a registered project's folder was renamed,
  removed or became unreadable, the daemon could fail to start after a restart or an update, which
  locked you out. It now starts, logs a warning naming the affected project and profile, and leaves
  that project's saved layout as it was. Other projects load normally.
- **Silent agent turns are stopped on schedule.** An ACP agent can report a failed tool call and then
  never answer the prompt. The quiet watchdog now starts measuring silence right away, so it warns at
  `session.supervision.quiet_after` (30 minutes by default) and stops the turn after
  `session.supervision.stop_grace` (10 minutes by default). Before, it waited extra watchdog intervals while the turn still
  looked active. Any new update from the agent clears the warning, and after the stop you can use
  **Resume** to retry.
- **Hosted MCP calls no longer hang forever.** A tool call made without its own deadline could wait
  indefinitely if the daemon stopped answering. Such a call now has a time budget built from the
  longest tool wait (24 hours), the longest approval wait (10 minutes) and `client.request_timeout`
  (30 seconds by default). When it runs out, the call ends with a tool error. Clarifications without
  a timeout and long extension calls keep running as long as the daemon reports progress. Calls with
  their own deadline behave as before.
- **App updates right after a release.** For a few minutes after a release was published, every
  **Update** or **Retry update** click could fail with "The app update could not be installed."
  because the update feed still named the previous version. The desktop app now installs the
  version the daemon planned, straight from its GitHub release, and still checks its digest against
  the release checksums.

##### Forked and continued sessions show the conversation they came from

A forked or continued session used to show only a divider and "Nothing said here yet", so it looked
as if its messages were lost. It now shows a collapsed, read-only **Earlier conversation** row above
the divider, with the number of messages it carried. Nothing loads until you expand it. Expanding it
shows the newest 50 messages from the source up to the fork or continue point, and **Show earlier
messages** loads older ones. The row is hidden when nothing was carried over.

These messages belong to the source. You can't edit them or fork from them, and they are not part
of the session's search, outline, message counts or find results. The expanded panel says what the
agent received, that is, how many of the carried messages it gets, or that the agent continues
from its own copy of the conversation. If the source was deleted, archived, moved, or rewound or
cleared past that point, the panel says the source is no longer available and that the agent's
carried context is unaffected.

Nothing is copied into the new session, so forking and continuing cost the same as before.
`compozy session history` and `compozy__session_history` still return only the session's own
messages.

##### Session lists load again after a parent is deleted, and the rail keeps its place

- **Workspace session lists after a deleted parent.** On v0.7.7, deleting or cleaning up a session
  whose children were still around could make every workspace-scoped session read fail with
  `session_not_found`. That broke `compozy session list --workspace`, `compozy worktree list`,
  `compozy worktree create` and `compozy session new --workspace`, and such children silently
  dropped out of the sidebar. These reads work again, and those children show in the sidebar.
- **Children of deleted side chats stay hidden.** When you delete a side chat while a child it
  started, such as a subagent, keeps running, that child no longer shows up as a top-level session
  in the session list, the catalog or `compozy__session_list`. You can still read it by ID.
- **The rail keeps its scroll when you switch sessions.** Clicking a row below the fold in a
  Session window's rail no longer sends the list back to the top. The rail also returns to where you
  were when its list finishes loading after the switch, until you scroll it yourself. While the
  picked session loads, the real rail stays beside the loading conversation instead of placeholder
  rows. A reload still starts at the top.
- **Notifications reuse your Session window.** Clicking a toast, an OS notification, or a session or
  notice row in the attention bell no longer opens a new Session tab for every session. It lands in
  your Session window by the same rule that new sessions use, and a session that already has a window
  is focused there. The rail scrolls the session into view.
- **Folders no longer push other sessions off the first page.** A folder with many newer sessions
  could take the whole first page of the list and hide the sessions outside folders. The sessions
  outside folders and each folder now page on their own, each with its own Previous and Next
  controls and its own total. Archived and Later keep their Manual order. Over HTTP and UDS, the
  sessions list accepts an optional `folder=<id>` or `folder=@root` selector, which needs one
  project and one profile; leaving it out returns the whole list as before.
- **Global always-on agents appear once.** When the session list or the Kanban board is grouped by
  project, Global always-on agents now show only in the **Global** row, not again in the **User**
  group.
- **Group totals leave out always-on main sessions.** A quiet Global always-on agent could make the
  User group read one session with no rows, and project totals counted agent main sessions too.
  Requests that send `exclude_instance_main=true`, as the List and Kanban views do, now leave those
  sessions out of the counts as well. Requests without the flag count as before.

##### Current subagent usage, terminal cap recovery and more fixes

- **Usage for resumed Claude subagents.** When Claude runs a finished background subagent again, its
  token and tool counts now show the current run instead of the highest figure from any earlier run.
  This applies to the subagent drawer, the CLI, HTTP and UDS, and the native subagent status tool.
  Existing subagents keep their recorded usage on upgrade.
- **Terminal cap dialog after a refused Close.** A Terminal window waiting behind the terminal limit
  dialog used to stay on "Opening a terminal" when the daemon refused the Close you chose there. The
  limit dialog now opens again, and dismissing it leaves the window on its empty state.
- **Empty peer lists explain themselves.** When an always-on agent's peer list is empty because
  every peer was hidden by authorization, `GET /api/agent-instances?peers=true`, the CLI and
  `compozy__agent_instance_list` now return a `peers_note` saying so, instead of looking like there
  are no peers. The note names no agents, IDs or projects.
- **Pull request monitors reject branches with spaces.** `compozy__monitor_create` with
  `kind=pull_request` now refuses a branch selector that contains whitespace anywhere, including
  leading or trailing spaces, instead of waiting for a pull request that can't exist.
- **Extension tools with line separator characters.** A TypeScript extension tool whose schema
  contained U+2028 or U+2029 in a property name or string value could fail to build or become
  unavailable. Those tools now work, and published schema digests and Loop pins are unchanged.
- **Local sockets for stdio MCP servers.** Stdio MCP servers run with their own isolated home folder,
  so a server that finds a local service's socket through your home folder can't see it. The MCP
  documentation now shows how to pass the absolute socket path through the server's own argument or
  environment variable.
