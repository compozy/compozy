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
